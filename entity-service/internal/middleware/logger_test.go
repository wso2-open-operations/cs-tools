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

package middleware

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
)

const testIssuer = "https://api.asgardeo.io/t/example/oauth2/token"

func newTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func sign(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// captureLog redirects the standard logger's output for the duration of fn,
// restoring it afterward -- Logger and auth.Middleware's reject() both write
// through log.Printf/slog respectively; this only needs the former.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)
	fn()
	return buf.String()
}

// TestLogger_CallerID covers Logger wrapping auth.Middleware end to end --
// the actual composition entity-service's own middleware chain uses (Logger
// wraps UserIDToken wraps auth.Middleware) -- since the two packages'
// respective unit tests each cover only their own half of the IdentityHolder
// handoff.
func TestLogger_CallerID(t *testing.T) {
	key := newTestKey(t)
	v := auth.NewValidatorWithKeyfunc(
		auth.Config{Issuer: testIssuer, UserTokenAudiences: []string{"spa-client-id"}, ClockSkew: 5 * time.Second},
		func(*jwt.Token) (any, error) { return &key.PublicKey, nil },
	)
	userToken := sign(t, key, jwt.MapClaims{
		"iss": testIssuer, "aud": []string{"spa-client-id"}, "sub": "session-scoped-id",
		"userid": "asgardeo-uuid-1", "email": "jane@example.com", "exp": time.Now().Add(time.Hour).Unix(),
	})
	clientAssertion := sign(t, key, jwt.MapClaims{
		"iss": testIssuer, "aud": []string{"m2m"}, "sub": "m2m-sub", "client_id": "integration-client-id",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	chain := func(w http.ResponseWriter, r *http.Request, headers map[string]string) string {
		for k, val := range headers {
			r.Header.Set(k, val)
		}
		handler := Logger(auth.Middleware(v)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})))
		var out string
		out = captureLog(t, func() { handler.ServeHTTP(w, r) })
		return out
	}

	t.Run("a human caller logs the stable userid claim, not sub", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/search", nil)
		out := chain(httptest.NewRecorder(), req, map[string]string{
			"x-jwt-assertion": clientAssertion, "x-user-id-token": userToken,
		})
		if !strings.Contains(out, "callerId=asgardeo-uuid-1") {
			t.Fatalf("access log missing the user's stable id, got: %s", out)
		}
		if strings.Contains(out, "callerId=session-scoped-id") {
			t.Fatalf("access log used the per-session sub claim instead of userid: %s", out)
		}
	})

	t.Run("a pure M2M caller logs the client id", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/search", nil)
		out := chain(httptest.NewRecorder(), req, map[string]string{"x-jwt-assertion": clientAssertion})
		if !strings.Contains(out, "callerId=integration-client-id") {
			t.Fatalf("access log missing the M2M caller's client id, got: %s", out)
		}
	})

	t.Run("a request with no tokens logs the placeholder, not an empty field", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/search", nil)
		out := chain(httptest.NewRecorder(), req, nil)
		if !strings.Contains(out, "callerId=- ") {
			t.Fatalf("expected the '-' placeholder, got: %s", out)
		}
	})

	t.Run("a rejected (invalid token) request still appears in the access log, with no caller id attributed", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/search", nil)
		out := chain(httptest.NewRecorder(), req, map[string]string{"x-user-id-token": "garbage"})
		if !strings.Contains(out, "callerId=- ") {
			t.Fatalf("a rejected request must still be logged, with no unproven caller id, got: %s", out)
		}
		if !strings.Contains(out, "status=401") {
			t.Fatalf("expected the 401 to be logged, got: %s", out)
		}
	})
}
