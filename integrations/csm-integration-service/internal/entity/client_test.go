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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/apierror"
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

// TestTokenFetchTimeout verifies that a stalled token endpoint fails requests
// within the configured timeout rather than blocking indefinitely.
func TestTokenFetchTimeout(t *testing.T) {
	// Token server that stalls longer than tokenFetchTimeout but returns eventually
	// so httptest.Server.Close() can drain cleanly.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer tokenSrv.Close()

	// Use a short timeout so the test runs quickly.
	tokenFetchTimeout = 100 * time.Millisecond
	t.Cleanup(func() { tokenFetchTimeout = 10 * time.Second })

	client := NewClient(Config{
		BaseURL:      tokenSrv.URL,
		TokenURL:     tokenSrv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	start := time.Now()
	_, err := client.GetAccount(context.Background(), "11111111-1111-1111-1111-111111111111")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from hung token server, got nil")
	}
	// Should fail well within 1 s (configured at 100 ms); allow 2 s for CI jitter.
	if elapsed > 2*time.Second {
		t.Errorf("token fetch took %v; expected <2s with 100ms timeout", elapsed)
	}
}

// TestDoSuccess verifies a 2xx upstream response is returned as-is.
func TestDoSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/accounts/search" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"accounts":[],"total":0}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	body, err := client.SearchAccounts(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("SearchAccounts() error = %v, want nil", err)
	}
	const want = `{"accounts":[],"total":0}`
	if string(body) != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

// TestDoUpstreamError verifies a non-2xx upstream response is converted to an
// *apierror.Error carrying the status code and a truncated body excerpt.
func TestDoUpstreamError(t *testing.T) {
	longBody := make([]byte, 8192)
	for i := range longBody {
		longBody[i] = 'x'
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(longBody)
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
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
	const maxErrBody = 4096
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
	client := NewClient(Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	ctx := WithCorrelationID(context.Background(), wantID)
	if _, err := client.SearchAccounts(ctx, []byte(`{}`)); err != nil {
		t.Fatalf("SearchAccounts() error = %v, want nil", err)
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
	client := NewClient(Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	if _, err := client.SearchAccounts(context.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("SearchAccounts() error = %v, want nil", err)
	}
	if sawHeader {
		t.Error("X-CSM-Correlation-ID header sent with no correlation ID in context, want omitted")
	}
}

// TestHealthProbe verifies the reachability probe hits GET /health without a
// token, succeeds on 2xx and fails on anything else.
func TestHealthProbe(t *testing.T) {
	var status int
	var sawAuth bool
	var tokenCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			tokenCalls++
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/health" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		sawAuth = r.Header.Get("Authorization") != ""
		w.WriteHeader(status)
	}))
	defer srv.Close()
	client := NewClient(Config{BaseURL: srv.URL + "/", TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s"})

	status = http.StatusOK
	if err := client.Health(context.Background()); err != nil {
		t.Errorf("200: err = %v, want nil", err)
	}
	if sawAuth || tokenCalls != 0 {
		t.Errorf("probe sent credentials (auth header %v, token calls %d)", sawAuth, tokenCalls)
	}

	status = http.StatusServiceUnavailable
	err := client.Health(context.Background())
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("503: err = %v, want *apierror.Error with 503", err)
	}

	srv.Close()
	if err := client.Health(context.Background()); err == nil {
		t.Error("closed server: err = nil, want an error")
	}
}

// TestDoCapturesRetryAfter verifies an upstream Retry-After header travels on
// the typed error so handlers can pass it through.
func TestDoCapturesRetryAfter(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"slow down"}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	_, err := client.SearchAccounts(context.Background(), []byte(`{}`))
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *apierror.Error", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want 429", apiErr.StatusCode)
	}
	if apiErr.RetryAfter != "30" {
		t.Errorf("RetryAfter = %q, want %q", apiErr.RetryAfter, "30")
	}
}

// TestDoRejectsOversizedSuccessBody verifies a 2xx body over the cap is an
// error rather than a truncated payload.
func TestDoRejectsOversizedSuccessBody(t *testing.T) {
	maxResponseBodyBytes = 1024
	t.Cleanup(func() { maxResponseBodyBytes = 32 << 20 })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pad":"` + strings.Repeat("x", 2048) + `"}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	body, err := client.SearchAccounts(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatalf("expected an error for a %d-byte body over a %d-byte cap, got body of %d bytes", 2048, 1024, len(body))
	}
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		t.Errorf("oversized body produced a typed upstream error %v; want a plain error", apiErr)
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error = %q, want it to say the body exceeds the cap", err)
	}

	// A body exactly at the cap is still accepted.
	exact := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
	}))
	defer exact.Close()
	client = NewClient(Config{BaseURL: exact.URL, TokenURL: tokenSrv.URL, ClientID: "c", ClientSecret: "s"})
	body, err = client.SearchAccounts(context.Background(), []byte(`{}`))
	if err != nil || len(body) != 1024 {
		t.Errorf("body at the cap: len=%d err=%v, want 1024 and nil", len(body), err)
	}
}
