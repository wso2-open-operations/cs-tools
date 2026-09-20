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

	"golang.org/x/crypto/bcrypt"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/middleware"
)

// mustHash hashes at bcrypt.DefaultCost — the only cost ParseBasicAuthUsers
// accepts (see its own doc comment on why a mismatched cost reopens the
// username-enumeration timing side channel BasicAuth's dummy-hash compare
// exists to close).
func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}
	return string(hash)
}

func TestBasicAuth_ValidCredentials_CallsWrappedHandler(t *testing.T) {
	t.Parallel()

	users := middleware.BasicAuthUsers{"datadog": mustHash(t, "s3cret")}
	called := false
	handler := middleware.BasicAuth(users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodPost, "/alerts", nil)
	r.SetBasicAuth("datadog", "s3cret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("wrapped handler was not called for valid credentials")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestBasicAuth_RejectsAndNeverCallsWrappedHandler(t *testing.T) {
	t.Parallel()

	users := middleware.BasicAuthUsers{"datadog": mustHash(t, "s3cret")}

	cases := []struct {
		name      string
		configure func(r *http.Request)
	}{
		{
			name:      "missing Authorization header",
			configure: func(r *http.Request) {},
		},
		{
			name: "malformed Authorization header",
			configure: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer not-basic-auth")
			},
		},
		{
			name: "unknown username",
			configure: func(r *http.Request) {
				r.SetBasicAuth("grafana", "whatever")
			},
		},
		{
			name: "known username, wrong password",
			configure: func(r *http.Request) {
				r.SetBasicAuth("datadog", "wrong-password")
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			called := false
			handler := middleware.BasicAuth(users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			r := httptest.NewRequest(http.MethodPost, "/alerts", nil)
			tc.configure(r)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			if called {
				t.Error("wrapped handler was called on a rejected request")
			}
			if w.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
			if got := w.Header().Get("WWW-Authenticate"); got == "" {
				t.Error("WWW-Authenticate header was not set")
			} else if want := `Basic realm="sre-alert-ingestion-service"`; got != want {
				t.Errorf("WWW-Authenticate = %q, want %q", got, want)
			}
		})
	}
}

func TestParseBasicAuthUsers_Valid(t *testing.T) {
	t.Parallel()

	datadogHash := mustHash(t, "dd-secret")
	grafanaHash := mustHash(t, "gf-secret")
	raw := "datadog:" + datadogHash + ",grafana:" + grafanaHash

	users, err := middleware.ParseBasicAuthUsers(raw)
	if err != nil {
		t.Fatalf("ParseBasicAuthUsers: unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("len(users) = %d, want 2", len(users))
	}
	if users["datadog"] != datadogHash {
		t.Errorf("users[datadog] = %q, want %q", users["datadog"], datadogHash)
	}
	if users["grafana"] != grafanaHash {
		t.Errorf("users[grafana] = %q, want %q", users["grafana"], grafanaHash)
	}
}

func TestParseBasicAuthUsers_Malformed(t *testing.T) {
	t.Parallel()

	validHash := mustHash(t, "s3cret")
	wrongCostHash, err := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}

	cases := []struct {
		name string
		raw  string
	}{
		{"empty string", ""},
		{"missing colon", "datadognohash"},
		{"empty username", ":" + validHash},
		{"extra colon in value makes it not look like a bcrypt hash", "data:dog:" + validHash},
		{"comma splits into a separate malformed pair", "data,dog:" + validHash},
		{"non-bcrypt-looking hash", "datadog:plaintext-password"},
		{"empty entry between commas", "datadog:" + validHash + ",,grafana:" + validHash},
		{"duplicate username", "datadog:" + validHash + ",datadog:" + validHash},
		{"hash at a non-default cost breaks timing parity with BasicAuth's dummy hash", "datadog:" + string(wrongCostHash)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := middleware.ParseBasicAuthUsers(tc.raw); err == nil {
				t.Errorf("ParseBasicAuthUsers(%q): expected error, got nil", tc.raw)
			}
		})
	}
}
