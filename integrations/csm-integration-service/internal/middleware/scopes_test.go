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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/middleware"
)

// unsignedJWT builds a compact JWT with the given payload claims and an empty
// signature, which is all the scope guard reads (it never verifies signatures).
func unsignedJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + "."
}

func serveWithScope(t *testing.T, guard *middleware.ScopeGuard, scope, token string) *httptest.ResponseRecorder {
	t.Helper()
	var nextCalled bool
	h := guard.Require(scope, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if token != "" {
		r.Header.Set("x-jwt-assertion", token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code == http.StatusNoContent && !nextCalled {
		t.Fatal("204 without the wrapped handler running")
	}
	if w.Code != http.StatusNoContent && nextCalled {
		t.Fatalf("wrapped handler ran but status = %d", w.Code)
	}
	return w
}

func assertAuthError(t *testing.T, w *httptest.ResponseRecorder, wantCode int, wantMsg string) {
	t.Helper()
	if w.Code != wantCode {
		t.Fatalf("status = %d, want %d; body %s", w.Code, wantCode, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Message != wantMsg {
		t.Errorf("message = %q, want %q", body.Message, wantMsg)
	}
}

const (
	wantUnauthorized = "You are not authorized to perform this action. Please try again."
	wantForbidden    = "Access to the requested resource is forbidden!"
)

func TestScopeGuard_DisabledPassesEverything(t *testing.T) {
	t.Parallel()
	guard := middleware.NewScopeGuard(false)
	if guard.Enforcing() {
		t.Fatal("Enforcing() = true for a disabled guard")
	}
	if w := serveWithScope(t, guard, "cases:read", ""); w.Code != http.StatusNoContent {
		t.Errorf("disabled guard, no token: status = %d, want 204", w.Code)
	}
}

func TestScopeGuard_MissingTokenIsUnauthorized(t *testing.T) {
	t.Parallel()
	guard := middleware.NewScopeGuard(true)
	assertAuthError(t, serveWithScope(t, guard, "cases:read", ""), http.StatusUnauthorized, wantUnauthorized)
	assertAuthError(t, serveWithScope(t, guard, "cases:read", "   "), http.StatusUnauthorized, wantUnauthorized)
}

func TestScopeGuard_MalformedTokenIsUnauthorized(t *testing.T) {
	t.Parallel()
	guard := middleware.NewScopeGuard(true)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	cases := map[string]string{
		"not a JWT":        "opaque-token",
		"two segments":     header + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"scope":"cases:read"}`)),
		"four segments":    header + ".a.b.c",
		"payload not b64":  header + ".!!!.",
		"payload not JSON": header + "." + base64.RawURLEncoding.EncodeToString([]byte(`scope=cases:read`)) + ".",
		"oversized":        header + "." + strings.Repeat("A", 17<<10) + ".",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			assertAuthError(t, serveWithScope(t, guard, "cases:read", token), http.StatusUnauthorized, wantUnauthorized)
		})
	}
}

func TestScopeGuard_NoScopeClaimIsForbidden(t *testing.T) {
	t.Parallel()
	guard := middleware.NewScopeGuard(true)
	token := unsignedJWT(t, map[string]any{"sub": "client", "client_id": "abc"})
	assertAuthError(t, serveWithScope(t, guard, "cases:read", token), http.StatusForbidden, wantForbidden)
}

func TestScopeGuard_WrongScopeIsForbidden(t *testing.T) {
	t.Parallel()
	guard := middleware.NewScopeGuard(true)
	token := unsignedJWT(t, map[string]any{"scope": "cases:read accounts:read"})
	assertAuthError(t, serveWithScope(t, guard, "cases:write", token), http.StatusForbidden, wantForbidden)
	// A scope is matched whole, never by prefix.
	assertAuthError(t, serveWithScope(t, guard, "cases", token), http.StatusForbidden, wantForbidden)
}

func TestScopeGuard_GrantedScopePasses(t *testing.T) {
	t.Parallel()
	guard := middleware.NewScopeGuard(true)
	cases := map[string]map[string]any{
		"scope string":            {"scope": "accounts:read cases:read"},
		"scope string single":     {"scope": "cases:read"},
		"scope array":             {"scope": []string{"accounts:read", "cases:read"}},
		"scp string fallback":     {"scp": "cases:read"},
		"scp array fallback":      {"scp": []string{"cases:read"}},
		"padded token tolerated":  {"scope": "cases:read", "pad": "xx"},
		"empty scope then scp":    {"scope": "", "scp": "cases:read"},
		"extra whitespace in str": {"scope": "  accounts:read   cases:read "},
	}
	for name, claims := range cases {
		t.Run(name, func(t *testing.T) {
			token := unsignedJWT(t, claims)
			if name == "padded token tolerated" {
				parts := strings.Split(token, ".")
				parts[1] = base64.URLEncoding.EncodeToString(mustDecode(t, parts[1]))
				token = strings.Join(parts, ".")
			}
			if w := serveWithScope(t, guard, "cases:read", token); w.Code != http.StatusNoContent {
				t.Errorf("status = %d, want 204; body %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestScopeGuard_ScopeClaimWinsOverScp(t *testing.T) {
	t.Parallel()
	guard := middleware.NewScopeGuard(true)
	token := unsignedJWT(t, map[string]any{"scope": "accounts:read", "scp": "cases:read"})
	assertAuthError(t, serveWithScope(t, guard, "cases:read", token), http.StatusForbidden, wantForbidden)
}

func mustDecode(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return b
}
