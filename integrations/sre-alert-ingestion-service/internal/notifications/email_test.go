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

func emailTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"bearer","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNewEmailClient_NeverFailsOnZeroValueConfig(t *testing.T) {
	c := NewEmailClient(EmailConfig{})
	if c == nil {
		t.Fatal("NewEmailClient returned nil for zero-value EmailConfig")
	}
	if err := c.SendEscalation(context.Background(), "subject", "body"); err == nil {
		t.Fatal("expected error for an unconfigured client (no recipients), got nil")
	}
}

// TestEmailClient_SendEscalation_DoesNotFollowRedirects guards against the
// bearer token leaking to a redirect target: oauth2.Transport reattaches the
// Authorization header to every request it processes, including a followed
// redirect, so a 307 from the email service must not be followed.
func TestEmailClient_SendEscalation_DoesNotFollowRedirects(t *testing.T) {
	redirectTargetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()
	tokenSrv := emailTokenServer(t)

	c := newEmailClient(EmailConfig{
		BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret",
		ToAddresses: []string{"sre@example.com"},
	}, true)

	err := c.SendEscalation(context.Background(), "subject", "body")
	if err == nil {
		t.Fatal("expected an error for an unfollowed redirect, got nil")
	}
	if redirectTargetCalled {
		t.Error("client followed the redirect; the bearer token was resubmitted to the redirect target")
	}
}

func TestEmailClient_SendEscalation_ValidatesArguments(t *testing.T) {
	called := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	tokenSrv := emailTokenServer(t)

	c := newEmailClient(EmailConfig{
		BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret",
		ToAddresses: []string{"sre@example.com"},
	}, true)

	if err := c.SendEscalation(context.Background(), "", "body"); err == nil {
		t.Fatal("expected error for empty subject, got nil")
	}
	if called {
		t.Error("upstream should not have been called for invalid arguments")
	}
}

func TestEmailClient_SendEscalation_SendsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody sendEmailRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	tokenSrv := emailTokenServer(t)

	c := newEmailClient(EmailConfig{
		BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret",
		FromAddress: "sre-alerts@wso2.com", ToAddresses: []string{"sre@example.com", "oncall@example.com"},
	}, true)

	if err := c.SendEscalation(context.Background(), "escalation subject", "<p>body</p>"); err != nil {
		t.Fatalf("SendEscalation returned error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/send-email" {
		t.Errorf("request = %s %s, want POST /send-email", gotMethod, gotPath)
	}
	if gotBody.From != "sre-alerts@wso2.com" {
		t.Errorf("From = %q, want %q", gotBody.From, "sre-alerts@wso2.com")
	}
	if len(gotBody.To) != 2 || gotBody.To[0] != "sre@example.com" || gotBody.To[1] != "oncall@example.com" {
		t.Errorf("To = %v, want [sre@example.com oncall@example.com]", gotBody.To)
	}
	if gotBody.Subject != "escalation subject" {
		t.Errorf("Subject = %q, want %q", gotBody.Subject, "escalation subject")
	}
	if string(gotBody.Template) != "<p>body</p>" {
		t.Errorf("Template = %q, want %q", gotBody.Template, "<p>body</p>")
	}
}

func TestEmailClient_SendEscalation_MapsUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	}))
	defer upstream.Close()
	tokenSrv := emailTokenServer(t)

	c := newEmailClient(EmailConfig{
		BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret",
		ToAddresses: []string{"sre@example.com"},
	}, true)

	err := c.SendEscalation(context.Background(), "subject", "body")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("error = %v, want *apierror.Error{StatusCode: 401}", err)
	}
}
