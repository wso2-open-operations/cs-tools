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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// newTestClient wires an EmailClient at a fake token endpoint and a fake
// notification service endpoint, both backed by httptest servers that the
// caller controls.
func newTestClient(t *testing.T, tokenSrv, apiSrv *httptest.Server, fromAddress string) *EmailClient {
	t.Helper()
	return NewEmailClient(EmailConfig{
		BaseURL:      apiSrv.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Scopes:       []string{"email"},
		FromAddress:  fromAddress,
	})
}

func newTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
}

func TestSendEmail_ValidatesArgumentsBeforeCallingUpstream(t *testing.T) {
	called := false
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()
	tokenSrv := newTokenServer(t)
	defer tokenSrv.Close()

	c := newTestClient(t, tokenSrv, apiSrv, "noreply@example.com")

	t.Run("rejects empty recipients", func(t *testing.T) {
		if err := c.SendEmail(context.Background(), nil, nil, nil, nil, "subject", "<p>body</p>", nil); err == nil {
			t.Fatal("expected error for empty recipients, got nil")
		}
	})

	t.Run("rejects empty subject", func(t *testing.T) {
		if err := c.SendEmail(context.Background(), []string{"a@example.com"}, nil, nil, nil, "", "<p>body</p>", nil); err == nil {
			t.Fatal("expected error for empty subject, got nil")
		}
	})

	if called {
		t.Error("upstream should not have been called for invalid arguments")
	}
}

func TestSendEmail_SendsExpectedRequest(t *testing.T) {
	var capturedAuth string
	var capturedBody sendEmailRequest
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/send-email" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		capturedAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "email sent successfully"})
	}))
	defer apiSrv.Close()
	tokenSrv := newTokenServer(t)
	defer tokenSrv.Close()

	c := newTestClient(t, tokenSrv, apiSrv, "noreply@example.com")

	attachments := []EmailAttachment{
		{ContentName: "notes.txt", ContentType: "text/plain", Attachment: []byte("hello")},
	}
	err := c.SendEmail(context.Background(),
		[]string{"customer@example.com"},
		[]string{"cc@example.com"},
		nil,
		[]string{"support@example.com"},
		"Your case has been updated",
		"<p>Hello there</p>",
		attachments,
	)
	if err != nil {
		t.Fatalf("SendEmail returned error: %v", err)
	}

	if capturedAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", capturedAuth, "Bearer test-token")
	}
	if capturedBody.From != "noreply@example.com" {
		t.Errorf("From = %q, want configured FromAddress", capturedBody.From)
	}
	if len(capturedBody.To) != 1 || capturedBody.To[0] != "customer@example.com" {
		t.Errorf("To = %v, want [customer@example.com]", capturedBody.To)
	}
	if len(capturedBody.CC) != 1 || capturedBody.CC[0] != "cc@example.com" {
		t.Errorf("CC = %v, want [cc@example.com]", capturedBody.CC)
	}
	if capturedBody.Subject != "Your case has been updated" {
		t.Errorf("Subject = %q, want %q", capturedBody.Subject, "Your case has been updated")
	}
	if string(capturedBody.Template) != "<p>Hello there</p>" {
		t.Errorf("Template = %q, want %q", capturedBody.Template, "<p>Hello there</p>")
	}
	if len(capturedBody.Attachments) != 1 || capturedBody.Attachments[0].ContentName != "notes.txt" {
		t.Errorf("Attachments = %+v, want one attachment named notes.txt", capturedBody.Attachments)
	}
}

func TestSendEmail_MapsUpstreamError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "missing required field: subject"})
	}))
	defer apiSrv.Close()
	tokenSrv := newTokenServer(t)
	defer tokenSrv.Close()

	c := newTestClient(t, tokenSrv, apiSrv, "noreply@example.com")

	err := c.SendEmail(context.Background(), []string{"a@example.com"}, nil, nil, nil, "subject", "body", nil)
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

func TestSendEmail_ConstructsWithZeroValueConfig(t *testing.T) {
	// NewEmailClient must never fail or panic even when the email channel has
	// not been configured for a given deployment.
	c := NewEmailClient(EmailConfig{})
	if c == nil {
		t.Fatal("NewEmailClient returned nil for zero-value EmailConfig")
	}
}
