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

package statuspage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The request is csm-scheduled-tasks': path, header and a three-field body.
func TestPost_SendsTheScheduledTasksRequest(t *testing.T) {
	var path, sig, ctype string
	var body map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, sig, ctype = r.URL.Path, r.Header.Get("X-Webhook-Signature"), r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
	}))
	defer srv.Close()

	w, err := New(map[string]string{"choreo": srv.URL + "/"}, map[string]string{"default": "Secret tok"})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Post(context.Background(), "choreo", "outage_begin", "2026-10-09T06:54:00.000Z"); err != nil {
		t.Fatal(err)
	}
	if path != "/api/v1/webhook" || sig != "Secret tok" || ctype != "application/json" {
		t.Errorf("path=%q sig=%q content-type=%q", path, sig, ctype)
	}
	if len(body) != 3 || body["event"] != "outage_begin" || body["timestamp"] != "2026-10-09T06:54:00.000Z" || body["cloud"] != "choreo" {
		t.Errorf("body = %v", body)
	}
}

func TestPost_CloudSecretWinsOverDefault(t *testing.T) {
	var sig string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { sig = r.Header.Get("X-Webhook-Signature") }))
	defer srv.Close()
	w, _ := New(map[string]string{"asgardeo": srv.URL}, map[string]string{"default": "Secret d", "asgardeo": "Secret a"})
	_ = w.Post(context.Background(), "asgardeo", "outage_end", "x")
	if sig != "Secret a" {
		t.Errorf("sig = %q, want the cloud's own secret", sig)
	}
}

func TestPost_Non2xxCarriesStatusNotBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("token=abc"))
	}))
	defer srv.Close()
	w, _ := New(map[string]string{"choreo": srv.URL}, map[string]string{"default": "Secret tok"})
	err := w.Post(context.Background(), "choreo", "outage_end", "x")
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "token=abc") {
		t.Errorf("err = %v", err)
	}
	if err := w.Post(context.Background(), "devant", "outage_end", "x"); err == nil {
		t.Error("an unconfigured cloud must be an error")
	}
}

func TestNew_RejectsBadConfig(t *testing.T) {
	sec := map[string]string{"default": "Secret tok"}
	for name, tc := range map[string]struct{ urls, secrets map[string]string }{
		"plain http to a real host": {map[string]string{"choreo": "http://status.example.com"}, sec},
		"not a URL":                 {map[string]string{"choreo": "::"}, sec},
		"no URLs":                   {map[string]string{}, sec},
		"no secrets":                {map[string]string{"choreo": "https://status.example.com"}, nil},
	} {
		if _, err := New(tc.urls, tc.secrets); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if _, err := New(map[string]string{"choreo": "http://127.0.0.1:9400"}, sec); err != nil {
		t.Errorf("loopback http is allowed for local development: %v", err)
	}
}

// A request that reached the dashboard but got no answer is an unknown
// outcome; one that never left (nothing listening) is a definite failure.
func TestPost_ClassifiesUnknownOutcome(t *testing.T) {
	hang := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// Read the request (so the server notices the client hanging up),
		// then never answer within the client's timeout.
		_, _ = io.ReadAll(r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer hang.Close()
	w, _ := New(map[string]string{"choreo": hang.URL, "devant": "http://127.0.0.1:1"}, map[string]string{"default": "Secret tok"})
	w.http.Timeout = 300 * time.Millisecond

	if err := w.Post(context.Background(), "choreo", "outage_begin", "x"); !IsUnknownOutcome(err) {
		t.Errorf("a timeout after sending must be unknown, got %v", err)
	}
	if err := w.Post(context.Background(), "devant", "outage_begin", "x"); err == nil || IsUnknownOutcome(err) {
		t.Errorf("a refused connection must be a definite failure, got %v", err)
	}
}
