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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testIssuer = "https://idp.example.com/oauth2/token"
	testKID    = "test-key"
)

// newTestKeys starts a JWKS endpoint publishing key's public half. Its "x5c"
// is deliberately not a parseable certificate: without the transport
// stripping it the key is dropped, and no token would verify.
func newTestKeys(t *testing.T, key *rsa.PrivateKey) jwt.Keyfunc {
	t.Helper()
	b64 := base64.RawURLEncoding.EncodeToString
	set := map[string]any{"keys": []map[string]any{{
		"kty": "RSA", "kid": testKID, "use": "sig", "alg": "RS256",
		"n":   b64(key.N.Bytes()),
		"e":   b64(big.NewInt(int64(key.E)).Bytes()),
		"x5c": []string{"bm90IGEgY2VydGlmaWNhdGU="},
	}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	kf, err := NewKeyfunc(ctx, srv.URL)
	if err != nil {
		t.Fatalf("NewKeyfunc: %v", err)
	}
	return kf
}

func newRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func baseClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":         testIssuer,
		"sub":         "user-123",
		"exp":         time.Now().Add(time.Hour).Unix(),
		"email":       "jane@example.com",
		"given_name":  "Jane",
		"family_name": "Doe",
		"aud":         "webapp-client",
		"client_id":   "webapp-client",
	}
}

func sign(t *testing.T, key *rsa.PrivateKey, c jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	tok.Header["kid"] = testKID
	tok.Header["typ"] = "at+jwt" // as Asgardeo issues access tokens
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// call runs a request with token through Auth and returns the status and the
// user the handler saw.
func call(t *testing.T, cfg AuthConfig, kf jwt.Keyfunc, token string) (int, *User) {
	t.Helper()
	var seen *User
	h := Auth(cfg, kf)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/submissions", nil)
	if token != "" {
		req.Header.Set(jwtAssertionHeader, token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code == http.StatusUnauthorized {
		var body map[string]string
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil || body["message"] == "" {
			t.Fatalf("401 body = %v (%v), want {\"message\"}", body, err)
		}
	}
	return w.Code, seen
}

func TestAuthAcceptsAValidToken(t *testing.T) {
	key := newRSAKey(t)
	kf := newTestKeys(t, key)
	cfg := AuthConfig{Issuer: testIssuer, ModeratorGroups: []string{"til-moderators"}}

	status, user := call(t, cfg, kf, sign(t, key, baseClaims()))
	if status != http.StatusOK || user == nil {
		t.Fatalf("status = %d, user = %v", status, user)
	}
	if user.Email != "jane@example.com" || user.DisplayName != "Jane Doe" || user.Subject != "user-123" || user.IsModerator {
		t.Fatalf("user = %+v", user)
	}
	if !user.IssuedTo("webapp-client") || user.IssuedTo("other-client") || user.IssuedTo("") {
		t.Fatalf("IssuedTo wrong for %v", user.ClientIDs)
	}
}

func TestAuthRejects(t *testing.T) {
	key := newRSAKey(t)
	kf := newTestKeys(t, key)
	cfg := AuthConfig{Issuer: testIssuer}

	with := func(change func(jwt.MapClaims)) string {
		c := baseClaims()
		change(c)
		return sign(t, key, c)
	}
	hs256, err := jwt.NewWithClaims(jwt.SigningMethodHS256, baseClaims()).SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, baseClaims()).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"no token":            "",
		"not a token":         "abc.def",
		"wrong issuer":        with(func(c jwt.MapClaims) { c["iss"] = "https://evil.example.com" }),
		"expired":             with(func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }),
		"no expiry":           with(func(c jwt.MapClaims) { delete(c, "exp") }),
		"no email":            with(func(c jwt.MapClaims) { delete(c, "email") }),
		"no subject":          with(func(c jwt.MapClaims) { delete(c, "sub") }),
		"signed by another":   sign(t, newRSAKey(t), baseClaims()),
		"HS256":               hs256,
		"unsigned (alg none)": none,
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if status, _ := call(t, cfg, kf, token); status != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", status)
			}
		})
	}
}

func TestAuthAudienceList(t *testing.T) {
	key := newRSAKey(t)
	kf := newTestKeys(t, key)

	c := baseClaims()
	c["aud"] = "something-else"
	c["client_id"] = "webapp-client"
	token := sign(t, key, c)

	if status, _ := call(t, AuthConfig{Issuer: testIssuer, Audiences: []string{"webapp-client"}}, kf, token); status != http.StatusOK {
		t.Fatalf("client_id in the list: status = %d", status)
	}
	if status, _ := call(t, AuthConfig{Issuer: testIssuer, Audiences: []string{"another-app"}}, kf, token); status != http.StatusUnauthorized {
		t.Fatalf("client_id not in the list: status = %d", status)
	}

	delete(c, "client_id")
	c["azp"] = "webapp-client"
	if status, user := call(t, AuthConfig{Issuer: testIssuer}, kf, sign(t, key, c)); status != http.StatusOK || !user.IssuedTo("webapp-client") {
		t.Fatalf("azp: status = %d, user = %+v", status, user)
	}
}

func TestAuthModeratorGroups(t *testing.T) {
	key := newRSAKey(t)
	kf := newTestKeys(t, key)
	cfg := AuthConfig{Issuer: testIssuer, ModeratorGroups: []string{"til-moderators"}}

	for name, groups := range map[string]any{
		"array":                  []string{"everyone", "til-moderators"},
		"single string":          "til-moderators",
		"comma-separated string": "everyone, til-moderators",
	} {
		t.Run(name, func(t *testing.T) {
			c := baseClaims()
			c["groups"] = groups
			if _, user := call(t, cfg, kf, sign(t, key, c)); user == nil || !user.IsModerator {
				t.Fatalf("user = %+v, want a moderator", user)
			}
		})
	}

	c := baseClaims()
	c["groups"] = []string{"TIL-Moderators"}
	if _, user := call(t, cfg, kf, sign(t, key, c)); user == nil || user.IsModerator {
		t.Fatal("group match must be exact")
	}
}

func TestAuthDisplayNameFallback(t *testing.T) {
	key := newRSAKey(t)
	kf := newTestKeys(t, key)
	cfg := AuthConfig{Issuer: testIssuer}

	c := baseClaims()
	delete(c, "given_name")
	delete(c, "family_name")
	c["name"] = "Jane D."
	if _, user := call(t, cfg, kf, sign(t, key, c)); user.DisplayName != "Jane D." {
		t.Fatalf("name claim: %q", user.DisplayName)
	}
	delete(c, "name")
	if _, user := call(t, cfg, kf, sign(t, key, c)); user.DisplayName != "jane@example.com" {
		t.Fatalf("email fallback: %q", user.DisplayName)
	}
}

func TestAuthSkipsHealth(t *testing.T) {
	h := Auth(AuthConfig{Issuer: testIssuer}, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /health = %d", w.Code)
	}
}
