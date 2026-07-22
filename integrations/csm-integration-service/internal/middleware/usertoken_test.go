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

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/middleware"
)

// tokenServer returns an httptest.Server that always issues a client-credentials
// access token, so tests can build a real entity.Client without an OAuth2 provider.
func tokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"bearer","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestUserIDToken_ForwardsToEntityClient verifies that a caller-supplied
// x-user-id-token header rides through the middleware's context and is forwarded
// by the entity client on its outgoing request.
func TestUserIDToken_ForwardsToEntityClient(t *testing.T) {
	t.Parallel()

	const wantToken = "test-user-id-token"
	var gotToken string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("x-user-id-token")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	client := entity.NewClient(entity.Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenServer(t).URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	handler := middleware.UserIDToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = client.SearchAccounts(r.Context(), []byte(`{}`))
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodPost, "/accounts/search", nil)
	r.Header.Set("x-user-id-token", wantToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if gotToken != wantToken {
		t.Errorf("x-user-id-token forwarded = %q, want %q", gotToken, wantToken)
	}
}

// TestUserIDToken_OmitsWhenAbsent verifies the common case: no x-user-id-token
// header on the inbound request means none is forwarded downstream either — pure
// M2M, no end-user identity involved.
func TestUserIDToken_OmitsWhenAbsent(t *testing.T) {
	t.Parallel()

	var sawHeader bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawHeader = r.Header["X-User-Id-Token"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	client := entity.NewClient(entity.Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenServer(t).URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	handler := middleware.UserIDToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = client.SearchAccounts(r.Context(), []byte(`{}`))
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodPost, "/accounts/search", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if sawHeader {
		t.Error("x-user-id-token header forwarded with none on the inbound request, want omitted")
	}
}
