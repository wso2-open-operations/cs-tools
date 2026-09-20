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

package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/apierror"
)

func TestNewGoogleChatClient_NeverFailsOnZeroValueConfig(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{})
	if c == nil {
		t.Fatal("NewGoogleChatClient returned nil for zero-value GoogleChatConfig")
	}
	if err := c.SendMessage(context.Background(), "hello"); err == nil {
		t.Fatal("expected error for an unconfigured client, got nil")
	}
}

func TestGoogleChatClient_SendMessage_ValidatesArguments(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newGoogleChatClient(GoogleChatConfig{WebhookURL: srv.URL}, true)
	if err := c.SendMessage(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty text, got nil")
	}
	if called {
		t.Error("upstream should not have been called for invalid arguments")
	}
}

// TestGoogleChatClient_SendMessage_DoesNotFollowRedirects guards against the
// webhook URL's secret key/token query parameters leaking to a redirect
// target: a 307 would otherwise resubmit the original POST, URL included, to
// whatever host the response names.
func TestGoogleChatClient_SendMessage_DoesNotFollowRedirects(t *testing.T) {
	redirectTargetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()

	c := newGoogleChatClient(GoogleChatConfig{WebhookURL: srv.URL}, true)
	err := c.SendMessage(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected an error for an unfollowed redirect, got nil")
	}
	if redirectTargetCalled {
		t.Error("client followed the redirect; the webhook URL's secret query params were resubmitted to the redirect target")
	}
}

func TestGoogleChatClient_SendMessage_SendsExpectedRequest(t *testing.T) {
	var gotBody chatMessage
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newGoogleChatClient(GoogleChatConfig{WebhookURL: srv.URL}, true)
	if err := c.SendMessage(context.Background(), "escalation message"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody.Text != "escalation message" {
		t.Errorf("Text = %q, want %q", gotBody.Text, "escalation message")
	}
}

func TestGoogleChatClient_SendMessage_MapsUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid webhook payload"}}`))
	}))
	defer srv.Close()

	c := newGoogleChatClient(GoogleChatConfig{WebhookURL: srv.URL}, true)
	err := c.SendMessage(context.Background(), "escalation message")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *apierror.Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
	}
}
