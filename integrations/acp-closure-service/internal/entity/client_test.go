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

package entity

import (
	"bytes"
	"context"
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

// mustNewClient builds a Client for tests whose subject isn't NewClient's
// own URL validation, failing the test immediately if construction errors.
func mustNewClient(t *testing.T, cfg Config) *Client {
	t.Helper()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	return client
}

// TestNewClient_RejectsInsecureTokenURL verifies NewClient refuses to
// construct a Client whose TokenURL isn't https:// — the token request
// carries the real ClientSecret, and an http:// endpoint would send it in
// cleartext. Same check internal/emailservice.NewClient already has.
func TestNewClient_RejectsInsecureTokenURL(t *testing.T) {
	_, err := NewClient(Config{
		BaseURL:      "https://csm-integration.example",
		TokenURL:     "http://csm-integration.example/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})
	if err == nil {
		t.Fatal("NewClient() error = nil, want non-nil for an http:// TokenURL")
	}
}

// TestNewClient_RejectsInsecureBaseURL mirrors the same check for BaseURL —
// the bearer token and real customer data flow over every request against
// this address.
func TestNewClient_RejectsInsecureBaseURL(t *testing.T) {
	_, err := NewClient(Config{
		BaseURL:      "http://csm-integration.example",
		TokenURL:     "https://csm-integration.example/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})
	if err == nil {
		t.Fatal("NewClient() error = nil, want non-nil for an http:// BaseURL")
	}
}

// TestNewClient_AcceptsHTTPSURLs is the green counterpart — confirms the
// validation doesn't false-positive on the correct, real configuration.
func TestNewClient_AcceptsHTTPSURLs(t *testing.T) {
	_, err := NewClient(Config{
		BaseURL:      "https://csm-integration.example",
		TokenURL:     "https://csm-integration.example/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil for valid https:// URLs", err)
	}
}

// TestTokenFetchRejectsRedirects verifies the token request itself — which
// POSTs the real client secret — never follows a redirect. TestDoRejectsRedirects
// covers only the client used for regular API calls; the token fetch runs on
// a separate client (the one embedded in tokenCtx) that needs its own guard.
func TestTokenFetchRejectsRedirects(t *testing.T) {
	var redirectTargetHit bool
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetHit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"bearer","expires_in":3600}`))
	}))
	defer redirectTarget.Close()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusTemporaryRedirect)
	}))
	defer tokenSrv.Close()

	client := mustNewClient(t, Config{
		BaseURL:      "https://unused.invalid",
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})

	if _, err := client.SearchProjects(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("expected error for a redirected token request, got nil")
	}
	if redirectTargetHit {
		t.Error("redirect target was contacted; want the token fetch to never follow the redirect")
	}
}

// TestTokenFetchTimeout verifies that a stalled token endpoint fails requests
// within the configured timeout rather than blocking indefinitely.
func TestTokenFetchTimeout(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer tokenSrv.Close()

	tokenFetchTimeout = 100 * time.Millisecond
	t.Cleanup(func() { tokenFetchTimeout = 10 * time.Second })

	client := mustNewClient(t, Config{
		BaseURL:      tokenSrv.URL,
		TokenURL:     tokenSrv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})

	start := time.Now()
	_, err := client.GetAccount(context.Background(), "11111111-1111-1111-1111-111111111111")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from hung token server, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("token fetch took %v; expected <2s with 100ms timeout", elapsed)
	}
}

// TestDoSuccess verifies a 2xx upstream response is returned as-is.
func TestDoSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/search" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"projects":[],"total":0}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := mustNewClient(t, Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})

	body, err := client.SearchProjects(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("SearchProjects() error = %v, want nil", err)
	}
	const want = `{"projects":[],"total":0}`
	if string(body) != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

// TestDoRejectsOversizedSuccessBody: a successful (2xx) response larger than
// the client's limit is rejected with an error instead of being read into
// memory. Error bodies were already capped; success bodies weren't, so a
// buggy or compromised upstream could exhaust the task's memory (security
// assessment 2026-10-08, CWE-400).
func TestDoRejectsOversizedSuccessBody(t *testing.T) {
	const limit = 10 << 20 // 10 MiB
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("x"), limit+1))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := mustNewClient(t, Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})

	body, err := client.SearchProjects(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatalf("SearchProjects() returned %d bytes and no error, want an error for a body over %d bytes", len(body), limit)
	}
}

// TestDoUpstreamError verifies a non-2xx upstream response is converted to an
// *apierror.Error carrying the status code and a truncated body excerpt.
func TestDoUpstreamError(t *testing.T) {
	longBody := make([]byte, 512)
	for i := range longBody {
		longBody[i] = 'x'
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(longBody)
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := mustNewClient(t, Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})

	_, err := client.GetAccount(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *apierror.Error", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
	const maxErrBody = 256
	if len(apiErr.Body) != maxErrBody {
		t.Errorf("Body length = %d, want truncated to %d", len(apiErr.Body), maxErrBody)
	}
}

// TestDoForwardsCorrelationID verifies the correlation ID stored in the request
// context is forwarded as X-CSM-Correlation-ID on the outgoing request.
func TestDoForwardsCorrelationID(t *testing.T) {
	const wantID = "test-correlation-id"
	var gotID string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Header.Get("X-CSM-Correlation-ID")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := mustNewClient(t, Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})

	ctx := WithCorrelationID(context.Background(), wantID)
	if _, err := client.SearchProjects(ctx, []byte(`{}`)); err != nil {
		t.Fatalf("SearchProjects() error = %v, want nil", err)
	}
	if gotID != wantID {
		t.Errorf("X-CSM-Correlation-ID = %q, want %q", gotID, wantID)
	}
}

// TestDoWithoutCorrelationIDOmitsHeader verifies no correlation header is sent
// when the context carries none, rather than an empty header value.
func TestDoWithoutCorrelationIDOmitsHeader(t *testing.T) {
	var sawHeader bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawHeader = r.Header["X-Csm-Correlation-Id"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := mustNewClient(t, Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})

	if _, err := client.SearchProjects(context.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("SearchProjects() error = %v, want nil", err)
	}
	if sawHeader {
		t.Error("X-CSM-Correlation-ID header sent with no correlation ID in context, want omitted")
	}
}

// TestDoRejectsRedirects verifies the client refuses to follow a 3xx
// response rather than transparently following it. oauth2.Transport
// reattaches the Authorization bearer token to every request it processes,
// including a followed redirect — so silently following one would leak the
// M2M token to whatever host the upstream says to redirect to.
func TestDoRejectsRedirects(t *testing.T) {
	var redirectTargetHit bool
	var gotAuthAtTarget string
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetHit = true
		gotAuthAtTarget = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer redirectTarget.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := mustNewClient(t, Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})

	_, err := client.SearchProjects(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for a 3xx upstream response, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *apierror.Error", err)
	}
	if apiErr.StatusCode != http.StatusFound {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusFound)
	}
	if redirectTargetHit {
		t.Errorf("redirect target was contacted; want the client to never follow the redirect")
	}
	if gotAuthAtTarget != "" {
		t.Errorf("Authorization header leaked to redirect target: %q", gotAuthAtTarget)
	}
}

// TestNewClientRequestsConfiguredScopes verifies the scope string is actually
// sent on the token request — csm-integration-service's token endpoint
// requires an explicit scope (confirmed via Postman); a bare
// grant_type=client_credentials with no scope is not sufficient.
func TestNewClientRequestsConfiguredScopes(t *testing.T) {
	var gotScope string
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse token request form: %v", err)
		}
		gotScope = r.PostForm.Get("scope")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	client := mustNewClient(t, Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       RequiredScopes,
	})

	if _, err := client.SearchProjects(context.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("SearchProjects() error = %v, want nil", err)
	}

	const wantScope = "csm_integration:project:read csm_integration:account:read " +
		"csm_integration:accounts:read csm_integration:project:update " +
		"csm_integration:accounts:contacts:read csm_integration:projects:contacts:read " +
		"csm_integration:projects:read"
	if gotScope != wantScope {
		t.Errorf("token request scope = %q, want %q", gotScope, wantScope)
	}
}
