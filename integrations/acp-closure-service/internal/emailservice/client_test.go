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

package emailservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/apierror"
)

// tokenServer returns an httptest.Server that always issues a client-credentials
// access token, so tests can exercise Client without a real OAuth2 provider.
func tokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"bearer","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestNewClient_RejectsInsecureTokenURL verifies NewClient refuses to
// construct a Client whose TokenURL isn't https:// — the token request
// carries the real ClientSecret, and an http:// endpoint would send it in
// cleartext (CodeRabbit, PR #1657).
func TestNewClient_RejectsInsecureTokenURL(t *testing.T) {
	_, err := NewClient(Config{
		BaseURL:      "https://email.example",
		TokenURL:     "http://email.example/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		FromAddress:  "no-reply@wso2.com",
	})
	if err == nil {
		t.Fatal("NewClient() error = nil, want non-nil for an http:// TokenURL")
	}
}

// TestNewClient_RejectsInsecureBaseURL mirrors the same check for BaseURL —
// real notice content (project names, account names, customer emails)
// flows through every request against this address.
func TestNewClient_RejectsInsecureBaseURL(t *testing.T) {
	_, err := NewClient(Config{
		BaseURL:      "http://email.example",
		TokenURL:     "https://email.example/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		FromAddress:  "no-reply@wso2.com",
	})
	if err == nil {
		t.Fatal("NewClient() error = nil, want non-nil for an http:// BaseURL")
	}
}

// TestNewClient_AcceptsHTTPSURLs is the green counterpart — confirms the
// validation doesn't false-positive on the correct, real configuration.
func TestNewClient_AcceptsHTTPSURLs(t *testing.T) {
	_, err := NewClient(Config{
		BaseURL:      "https://email.example",
		TokenURL:     "https://email.example/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		FromAddress:  "no-reply@wso2.com",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil for valid https:// URLs", err)
	}
}

// TestTokenFetchTimeout verifies that a stalled token endpoint fails requests
// within the configured timeout rather than blocking indefinitely — mirrors
// internal/entity's identical test, same underlying OAuth2 client-credentials
// setup.
func TestTokenFetchTimeout(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer tokenSrv.Close()

	tokenFetchTimeout = 100 * time.Millisecond
	t.Cleanup(func() { tokenFetchTimeout = 10 * time.Second })

	client, err := NewClient(Config{
		BaseURL:      tokenSrv.URL,
		TokenURL:     tokenSrv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		FromAddress:  "no-reply@wso2.com",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	start := time.Now()
	err = client.SendEmail(context.Background(), []string{"to@wso2.example"}, nil, "subject", "<p>body</p>")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from hung token server, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("token fetch took %v; expected <2s with 100ms timeout", elapsed)
	}
}

// TestSendEmail_Success verifies the request is built correctly and a 2xx
// response is treated as success. Confirms the wire shape against Rashmika's
// real csm-notification-service client (integrations/csm-notification-service/
// internal/notifications/email.go, dev-app-csm-portal branch): POST
// /send-email with {to, cc, from, subject, template}.
func TestSendEmail_Success(t *testing.T) {
	var gotBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/send-email" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client, err := NewClient(Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		FromAddress:  "no-reply@wso2.com",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	err = client.SendEmail(context.Background(),
		[]string{"to@wso2.example"}, []string{"cc@wso2.example"},
		"Test Subject", "<p>Test body</p>")
	if err != nil {
		t.Fatalf("SendEmail() error = %v, want nil", err)
	}

	if got := gotBody["from"]; got != "no-reply@wso2.com" {
		t.Errorf("from = %v, want %q", got, "no-reply@wso2.com")
	}
	if got := gotBody["subject"]; got != "Test Subject" {
		t.Errorf("subject = %v, want %q", got, "Test Subject")
	}
	// template must be base64-encoded on the wire — matches the real
	// service's own request struct, which types this field []byte
	// (encoding/json base64-encodes []byte automatically). Sending it as
	// a plain string instead would not match what the server decodes.
	const wantTemplateBase64 = "PHA+VGVzdCBib2R5PC9wPg==" // base64("<p>Test body</p>")
	if got := gotBody["template"]; got != wantTemplateBase64 {
		t.Errorf("template = %v, want base64-encoded %q", got, wantTemplateBase64)
	}
	to, _ := gotBody["to"].([]any)
	if len(to) != 1 || to[0] != "to@wso2.example" {
		t.Errorf("to = %v, want [to@wso2.example]", gotBody["to"])
	}
	cc, _ := gotBody["cc"].([]any)
	if len(cc) != 1 || cc[0] != "cc@wso2.example" {
		t.Errorf("cc = %v, want [cc@wso2.example]", gotBody["cc"])
	}
}

// TestSendEmail_RequiresAtLeastOneRecipient mirrors the real
// csm-notification-service client's own validation (email.go: "at least one
// recipient (to) is required") — checked client-side before ever making the
// request, so a caller gets an immediate, specific error instead of a vague
// upstream rejection.
func TestSendEmail_RequiresAtLeastOneRecipient(t *testing.T) {
	client, err := NewClient(Config{
		BaseURL:      "https://unused.invalid",
		TokenURL:     "https://unused.invalid",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		FromAddress:  "no-reply@wso2.com",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	err = client.SendEmail(context.Background(), nil, nil, "subject", "<p>body</p>")
	if err == nil {
		t.Fatal("SendEmail() error = nil, want non-nil for zero recipients")
	}
}

// TestSendEmail_RequiresSubject mirrors the same client-side validation for
// an empty subject.
func TestSendEmail_RequiresSubject(t *testing.T) {
	client, err := NewClient(Config{
		BaseURL:      "https://unused.invalid",
		TokenURL:     "https://unused.invalid",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		FromAddress:  "no-reply@wso2.com",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	err = client.SendEmail(context.Background(), []string{"to@wso2.example"}, nil, "", "<p>body</p>")
	if err == nil {
		t.Fatal("SendEmail() error = nil, want non-nil for empty subject")
	}
}

// TestDoUpstreamError verifies a non-2xx upstream response is converted to an
// *apierror.Error carrying the status code and body excerpt — same shared
// error type internal/entity's client uses.
func TestDoUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid request"}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client, err := NewClient(Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		FromAddress:  "no-reply@wso2.com",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	err = client.SendEmail(context.Background(), []string{"to@wso2.example"}, nil, "subject", "<p>body</p>")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *apierror.Error", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
	}
}

// TestTokenFetchRejectsRedirects verifies the redirect protection also
// covers the token-fetch request itself, not just regular API calls. The
// OAuth2 client-credentials flow POSTs the real client secret to TokenURL;
// if that request follows a redirect, the secret could reach an
// unintended host. A prior version of this fix applied CheckRedirect only
// to the client returned for regular API calls, leaving the separate
// client embedded in tokenCtx (the one that actually performs the
// token-fetch request) unprotected.
func TestTokenFetchRejectsRedirects(t *testing.T) {
	var redirectTargetHit bool
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetHit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"bearer","expires_in":3600}`))
	}))
	defer redirectTarget.Close()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer tokenSrv.Close()

	client, err := NewClient(Config{
		BaseURL:      "https://unused.invalid",
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		FromAddress:  "no-reply@wso2.com",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	err = client.SendEmail(context.Background(), []string{"to@wso2.example"}, nil, "subject", "<p>body</p>")
	if err == nil {
		t.Fatal("expected error for a redirected token request, got nil")
	}
	if redirectTargetHit {
		t.Error("redirect target was contacted; want the token fetch to never follow the redirect")
	}
}

// TestDoRejectsRedirects mirrors internal/entity's identical protection:
// oauth2.Transport reattaches the Authorization bearer token to every
// request it processes, including a followed redirect — so silently
// following one would leak the M2M token to whatever host the upstream says
// to redirect to.
func TestDoRejectsRedirects(t *testing.T) {
	var redirectTargetHit bool
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer redirectTarget.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client, err := NewClient(Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		FromAddress:  "no-reply@wso2.com",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	err = client.SendEmail(context.Background(), []string{"to@wso2.example"}, nil, "subject", "<p>body</p>")
	if err == nil {
		t.Fatal("expected error for a 3xx upstream response, got nil")
	}
	if redirectTargetHit {
		t.Error("redirect target was contacted; want the client to never follow the redirect")
	}
}
