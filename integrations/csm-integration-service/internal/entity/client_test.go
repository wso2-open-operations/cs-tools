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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/apierror"
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
