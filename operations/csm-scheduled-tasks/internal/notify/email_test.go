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

package notify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

func mailServer(t *testing.T, status int, respBody string, got *sendEmailRequest, calls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600}`))
			return
		}
		*calls++
		if r.URL.Path != "/send-email" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(got)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
}

func TestSendEmail_WireShape(t *testing.T) {
	var got sendEmailRequest
	calls := 0
	srv := mailServer(t, 200, `{}`, &got, &calls)
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", FromAddress: "noreply@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SendEmail(context.Background(), []string{"a@example.com"}, []string{"b@example.com"}, "subj", "<p>hi</p>"); err != nil {
		t.Fatal(err)
	}
	if got.From != "noreply@example.com" || got.Subject != "subj" || string(got.Template) != "<p>hi</p>" ||
		got.To[0] != "a@example.com" || got.CC[0] != "b@example.com" {
		t.Fatalf("unexpected request %+v", got)
	}
}

func TestSendEmail_NoRecipientsIsANoOp(t *testing.T) {
	var got sendEmailRequest
	calls := 0
	srv := mailServer(t, 200, `{}`, &got, &calls)
	defer srv.Close()

	c, _ := NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token"})
	if err := c.SendEmail(context.Background(), nil, nil, "subj", "body"); err != nil || calls != 0 {
		t.Fatalf("no recipients must send nothing, calls=%d err=%v", calls, err)
	}
}

func TestSendEmail_ErrorTruncatesBodyAndKeepsItOutOfTheMessage(t *testing.T) {
	var got sendEmailRequest
	calls := 0
	srv := mailServer(t, 502, strings.Repeat("x", 1000), &got, &calls)
	defer srv.Close()

	c, _ := NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token"})
	err := c.SendEmail(context.Background(), []string{"a@example.com"}, nil, "subj", "body")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 502 {
		t.Fatalf("want *apierror.Error 502, got %v", err)
	}
	if len(apiErr.Body) != 256 {
		t.Errorf("the kept body excerpt must be bounded to 256 bytes, got %d", len(apiErr.Body))
	}
	if strings.Contains(err.Error(), "xxx") {
		t.Errorf("the response body must not appear in the error text")
	}
}

func TestNewClient_EmptyBaseURLIsAllowed(t *testing.T) {
	if _, err := NewClient(Config{TokenURL: "https://example.invalid/token"}); err != nil {
		t.Fatalf("e-mail is optional per deployment; an empty base URL must construct, got %v", err)
	}
	if _, err := NewClient(Config{TokenURL: "https://example.invalid/token", BaseURL: "http://example.invalid"}); err == nil {
		t.Fatal("a plaintext non-loopback base URL must be rejected")
	}
}
