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
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// noopHandler writes 200 OK and is used as the inner handler in middleware tests.
var noopHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// testConfig returns an auth config with signature validation disabled (safe for tests).
func testConfig() middleware.Config {
	return middleware.Config{
		JWKSEndpoint:          "http://unused.example.com",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		TokenValidatorEnabled: false,
	}
}

// makeTestJWT builds a minimal unsigned JWT with the given claims.
// The token is accepted when TokenValidatorEnabled is false.
func makeTestJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claimsJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	sig := base64.RawURLEncoding.EncodeToString([]byte("test-sig"))
	return header + "." + payload + "." + sig
}

func validToken() string {
	return makeTestJWT(map[string]any{
		"email":  "user@example.com",
		"userid": "uid-123",
	})
}

// serve is a convenience wrapper that runs the auth middleware around noopHandler.
func serve(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	middleware.Auth(testConfig())(noopHandler).ServeHTTP(w, r)
	return w
}

// ----- health check -----

func TestAuth_HealthCheck(t *testing.T) {
	t.Run("skips auth and returns 200", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := serve(r)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})
}

// ----- token validation -----

func TestAuth_TokenValidation(t *testing.T) {
	tests := []struct {
		name      string
		token     string // empty string means no header set
		wantCode  int
	}{
		{"missing token header", "", http.StatusUnauthorized},
		{"too few segments", "only-one-segment", http.StatusUnauthorized},
		{"too many segments", "a.b.c.d", http.StatusUnauthorized},
		{"valid token passes", validToken(), http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/cases", nil)
			if tc.token != "" {
				r.Header.Set("x-jwt-assertion", tc.token)
			}
			w := serve(r)
			if w.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}

// ----- required claims -----

func TestAuth_RequiredClaims(t *testing.T) {
	tests := []struct {
		name   string
		claims map[string]any
	}{
		{"missing email", map[string]any{"userid": "uid-123"}},
		{"missing userid", map[string]any{"email": "user@example.com"}},
		{"both claims missing", map[string]any{"sub": "someone"}},
	}
	for _, tc := range tests {
		t.Run(tc.name+" returns 401", func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/cases", nil)
			r.Header.Set("x-jwt-assertion", makeTestJWT(tc.claims))
			w := serve(r)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", w.Code)
			}
		})
	}
}

// ----- user info injection -----

func TestAuth_UserInfoInjection(t *testing.T) {
	t.Run("injects email, userid and groups from token into context", func(t *testing.T) {
		var captured *middleware.UserInfo
		capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			captured = middleware.UserInfoFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		r := httptest.NewRequest(http.MethodGet, "/cases", nil)
		r.Header.Set("x-jwt-assertion", makeTestJWT(map[string]any{
			"email":  "agent@wso2.com",
			"userid": "uid-456",
			"groups": []string{"csm-agents", "csm-admins"},
		}))
		w := httptest.NewRecorder()
		middleware.Auth(testConfig())(capture).ServeHTTP(w, r)

		if captured == nil {
			t.Fatal("UserInfo not set in context")
		}
		if captured.Email != "agent@wso2.com" {
			t.Errorf("email = %q, want agent@wso2.com", captured.Email)
		}
		if captured.UserID != "uid-456" {
			t.Errorf("userID = %q, want uid-456", captured.UserID)
		}
		if len(captured.Groups) != 2 {
			t.Errorf("groups = %v, want 2 entries", captured.Groups)
		}
	})
}

// ----- security headers -----

func TestAuth_SecurityHeaders(t *testing.T) {
	wantHeaders := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"Content-Security-Policy":   "upgrade-insecure-requests",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	}

	cases := []struct {
		name  string
		setup func(*http.Request)
	}{
		{"authenticated request", func(r *http.Request) { r.Header.Set("x-jwt-assertion", validToken()) }},
		{"unauthenticated request", func(_ *http.Request) {}},
		{"health check", func(r *http.Request) { /* no token needed */ }},
	}
	for _, tc := range cases {
		t.Run(tc.name+" has security headers", func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/cases", nil)
			if tc.name == "health check" {
				r = httptest.NewRequest(http.MethodGet, "/health", nil)
			}
			tc.setup(r)
			w := serve(r)
			for name, want := range wantHeaders {
				if got := w.Header().Get(name); got != want {
					t.Errorf("header %s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

// ----- error response shape -----

func TestAuth_ErrorResponse(t *testing.T) {
	t.Run("is JSON with a message field", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/cases", nil) // no token
		w := serve(r)

		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var body struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatalf("decode error body: %v", err)
		}
		if body.Message == "" {
			t.Error("expected non-empty message in error response")
		}
	})
}
