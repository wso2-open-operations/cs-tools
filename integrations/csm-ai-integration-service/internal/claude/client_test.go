// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package claude

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

const fakeMessage = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5-5",
"content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","stop_sequence":null,
"usage":{"input_tokens":1,"output_tokens":1}}`

func sendOne(t *testing.T, client anthropic.Client) (*anthropic.Message, error) {
	t.Helper()
	return client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     "claude-sonnet-5-5",
		MaxTokens: 20,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("Say hi"))},
	})
}

func TestNewClient_CallsTheGatewayWithOnlyTheKey(t *testing.T) {
	// Variables the SDK would read by default. None may reach the request.
	t.Setenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("ANTHROPIC_API_KEY", "env-key")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "env-token")
	t.Setenv("ANTHROPIC_CUSTOM_HEADERS", "X-Injected: yes")

	var got *http.Request
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(context.Background())
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, fakeMessage)
	}))
	defer srv.Close()

	client := NewClient(Config{APIKey: "test-key", BaseURL: srv.URL + "/team/proxy", Timeout: 5 * time.Second})
	msg, err := sendOne(t, client)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.URL.Path != "/team/proxy/v1/messages" {
		t.Errorf("path = %q, want the base URL's path + /v1/messages", got.URL.Path)
	}
	if k := got.Header.Get("X-Api-Key"); k != "test-key" {
		t.Errorf("x-api-key = %q, want the configured key", k)
	}
	if a := got.Header.Get("Authorization"); a != "" {
		t.Errorf("Authorization = %q, want none (ANTHROPIC_AUTH_TOKEN must be ignored)", a)
	}
	if got.Header.Get("X-Injected") != "" {
		t.Error("ANTHROPIC_CUSTOM_HEADERS reached the request")
	}
	if got.Header.Get("Anthropic-Version") == "" {
		t.Error("no anthropic-version header")
	}
	if !strings.Contains(body, `"model":"claude-sonnet-5-5"`) {
		t.Errorf("body = %s, want the model", body)
	}
	if len(msg.Content) != 1 || msg.Content[0].Text != "hi" {
		t.Errorf("content = %+v, want the fake reply", msg.Content)
	}
}

func TestNewClient_RetriesOnceThenGivesUp(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After-Ms", "1")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"overloaded_error","message":"busy"}}`)
	}))
	defer srv.Close()

	_, err := sendOne(t, NewClient(Config{APIKey: "k", BaseURL: srv.URL, Timeout: 5 * time.Second}))
	if err == nil {
		t.Fatal("want an error from a gateway that is always unavailable")
	}
	if n := calls.Load(); n != 1+maxRetries {
		t.Errorf("calls = %d, want %d (one attempt, %d retry)", n, 1+maxRetries, maxRetries)
	}
}

func TestNewClient_TimeoutBoundsTheCall(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	start := time.Now()
	_, err := sendOne(t, NewClient(Config{APIKey: "k", BaseURL: srv.URL, Timeout: 100 * time.Millisecond}))
	if err == nil {
		t.Fatal("want an error from a gateway that never answers")
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("took %v, want about (1+%d) x 100ms", took, maxRetries)
	}
}
