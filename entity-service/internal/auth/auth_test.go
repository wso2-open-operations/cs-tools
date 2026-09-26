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

package auth

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
	testIssuer = "https://api.asgardeo.io/t/example/oauth2/token"
	testSPA    = "spa-client-id"
	testM2M    = "integration-client-id"
)

func newKey(t *testing.T) *rsa.PrivateKey {
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
	tok.Header["kid"] = "k1"
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func testConfig() Config {
	return Config{Issuer: testIssuer, UserTokenAudiences: []string{testSPA}, ClockSkew: 5 * time.Second}
}

func staticValidator(key *rsa.PrivateKey) *Validator {
	return NewValidatorWithKeyfunc(testConfig(), func(*jwt.Token) (any, error) { return &key.PublicKey, nil })
}

func userClaims(mod func(jwt.MapClaims)) jwt.MapClaims {
	c := jwt.MapClaims{"iss": testIssuer, "aud": []string{testSPA}, "sub": "user-1", "userid": "asgardeo-uuid-1", "email": "Jane@Example.com", "exp": time.Now().Add(time.Hour).Unix()}
	if mod != nil {
		mod(c)
	}
	return c
}

func clientClaims(mod func(jwt.MapClaims)) jwt.MapClaims {
	c := jwt.MapClaims{"iss": testIssuer, "aud": []string{testM2M}, "sub": "m2m-sub", "client_id": testM2M, "exp": time.Now().Add(time.Hour).Unix()}
	if mod != nil {
		mod(c)
	}
	return c
}

func TestValidateUserToken(t *testing.T) {
	key, other := newKey(t), newKey(t)
	v := staticValidator(key)

	uc, err := v.ValidateUserToken(sign(t, key, userClaims(nil)))
	if err != nil || uc.Email != "Jane@Example.com" || uc.Subject != "user-1" || uc.UserID != "asgardeo-uuid-1" {
		t.Fatalf("valid token: %+v, %v", uc, err)
	}

	bad := map[string]string{
		"wrong issuer":        sign(t, key, userClaims(func(c jwt.MapClaims) { c["iss"] = "https://evil.example/token" })),
		"audience not listed": sign(t, key, userClaims(func(c jwt.MapClaims) { c["aud"] = []string{"someone-else"} })),
		"expired":             sign(t, key, userClaims(func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() })),
		"no exp":              sign(t, key, userClaims(func(c jwt.MapClaims) { delete(c, "exp") })),
		"no email":            sign(t, key, userClaims(func(c jwt.MapClaims) { delete(c, "email") })),
		"blank email":         sign(t, key, userClaims(func(c jwt.MapClaims) { c["email"] = "  " })),
		"signed by other key": sign(t, other, userClaims(nil)),
		"not a jwt":           "garbage",
	}
	for name, tok := range bad {
		if _, err := v.ValidateUserToken(tok); err == nil {
			t.Errorf("%s: token was accepted", name)
		}
	}
}

// A token whose header says HS256 and is MACed with the (public) key bytes
// must never validate: accepted algorithms are asymmetric only.
func TestValidateUserToken_RejectsAlgorithmConfusion(t *testing.T) {
	key := newKey(t)
	v := NewValidatorWithKeyfunc(testConfig(), func(*jwt.Token) (any, error) { return []byte("public-key-bytes"), nil })
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, userClaims(nil))
	s, _ := tok.SignedString([]byte("public-key-bytes"))
	if _, err := v.ValidateUserToken(s); err == nil {
		t.Fatal("HS256 token was accepted")
	}
	_ = key
}

func TestExtractClientID(t *testing.T) {
	key, other := newKey(t), newKey(t)
	v := staticValidator(key)

	cc, err := v.ExtractClientID(sign(t, key, clientClaims(nil)))
	if err != nil || cc.ClientID != testM2M {
		t.Fatalf("client_id claim: %+v, %v", cc, err)
	}
	cc, err = v.ExtractClientID(sign(t, key, clientClaims(func(c jwt.MapClaims) { delete(c, "client_id"); c["azp"] = "from-azp" })))
	if err != nil || cc.ClientID != "from-azp" {
		t.Fatalf("azp fallback: %+v, %v", cc, err)
	}

	// Deliberately unverified -- see ExtractClientID's own doc comment for
	// why: the client id is still read out of a wrong-issuer, expired, or
	// wrong-key-signed token, since none of that is checked.
	unverified := map[string]string{
		"wrong issuer":        sign(t, key, clientClaims(func(c jwt.MapClaims) { c["iss"] = "https://evil.example/token" })),
		"expired":             sign(t, key, clientClaims(func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() })),
		"no exp claim at all": sign(t, key, clientClaims(func(c jwt.MapClaims) { delete(c, "exp") })),
		"signed by other key": sign(t, other, clientClaims(nil)),
	}
	for name, tok := range unverified {
		if cc, err := v.ExtractClientID(tok); err != nil || cc.ClientID != testM2M {
			t.Errorf("%s: expected the client id to still be extracted, got %+v, %v", name, cc, err)
		}
	}

	bad := map[string]string{
		"no client id, no azp": sign(t, key, clientClaims(func(c jwt.MapClaims) { delete(c, "client_id") })),
		"not a jwt":            "garbage",
	}
	for name, tok := range bad {
		if _, err := v.ExtractClientID(tok); err == nil {
			t.Errorf("%s: token was accepted", name)
		}
	}
}

func run(t *testing.T, v *Validator, headers map[string]string) (code int, id Identity, called bool) {
	t.Helper()
	_, code, id, called = runWithHolder(t, v, headers)
	return code, id, called
}

// runWithHolder is run's superset, also returning the *IdentityHolder --
// installed into context the same way middleware.Logger installs one in
// production, before Middleware runs -- so a test can assert what an outer
// access logger would see, including on a rejected request.
func runWithHolder(t *testing.T, v *Validator, headers map[string]string) (holder *IdentityHolder, code int, id Identity, called bool) {
	t.Helper()
	h := Middleware(v)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		id = IdentityFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodPost, "/search", nil)
	for k, val := range headers {
		req.Header.Set(k, val)
	}
	ctx, holderRef := WithIdentityHolder(req.Context())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return holderRef, rec.Code, id, called
}

// TestMiddleware_NilValidatorIsADefensiveFallbackNotADeploymentMode: routes.go
// always supplies a real Validator, so a nil one here is only reachable if
// some caller wires this middleware incorrectly -- it must still fail safe
// (pass through, never trust the token) rather than panic.
func TestMiddleware_NilValidatorIsADefensiveFallbackNotADeploymentMode(t *testing.T) {
	code, id, called := run(t, nil, map[string]string{"x-user-id-token": "garbage", "x-jwt-assertion": "garbage"})
	if code != http.StatusOK || !called {
		t.Fatalf("a nil validator must pass through, got %d called=%v", code, called)
	}
	if id != (Identity{}) {
		t.Fatalf("unvalidated identity must be empty, got %+v", id)
	}
}

func TestMiddleware_Enabled(t *testing.T) {
	key := newKey(t)
	v := staticValidator(key)
	user := sign(t, key, userClaims(nil))
	clientAssertion := sign(t, key, clientClaims(nil))

	t.Run("no tokens passes as validated-but-anonymous", func(t *testing.T) {
		code, id, called := run(t, v, nil)
		if code != 200 || !called || !id.Validated || id.UserEmail != "" || id.ClientID != "" {
			t.Fatalf("got %d %+v called=%v", code, id, called)
		}
	})
	t.Run("m2m: client assertion only", func(t *testing.T) {
		_, id, _ := run(t, v, map[string]string{"x-jwt-assertion": clientAssertion})
		if !id.Validated || id.ClientID != testM2M || id.UserEmail != "" {
			t.Fatalf("got %+v", id)
		}
	})
	t.Run("an expired or wrong-key-signed client assertion still resolves the client id", func(t *testing.T) {
		other := newKey(t)
		expired := sign(t, key, clientClaims(func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }))
		wrongKey := sign(t, other, clientClaims(nil))
		for _, tok := range []string{expired, wrongKey} {
			code, id, _ := run(t, v, map[string]string{"x-jwt-assertion": tok})
			if code != http.StatusOK || id.ClientID != testM2M {
				t.Fatalf("x-jwt-assertion is decoded, not verified -- got %d %+v", code, id)
			}
		}
	})
	t.Run("on behalf of a user: client assertion + user token", func(t *testing.T) {
		_, id, _ := run(t, v, map[string]string{"x-jwt-assertion": clientAssertion, "x-user-id-token": user})
		if id.ClientID != testM2M || id.UserEmail != "Jane@Example.com" || id.UserSubject != "user-1" || id.UserID != "asgardeo-uuid-1" {
			t.Fatalf("got %+v", id)
		}
	})
	t.Run("invalid user token is rejected, not downgraded to anonymous", func(t *testing.T) {
		code, _, called := run(t, v, map[string]string{"x-jwt-assertion": clientAssertion, "x-user-id-token": "garbage"})
		if code != http.StatusUnauthorized || called {
			t.Fatalf("got %d called=%v", code, called)
		}
	})
	t.Run("invalid client assertion is rejected", func(t *testing.T) {
		code, _, called := run(t, v, map[string]string{"x-jwt-assertion": "garbage"})
		if code != http.StatusUnauthorized || called {
			t.Fatalf("got %d called=%v", code, called)
		}
	})
	t.Run("a client-credentials token is not accepted as a user token", func(t *testing.T) {
		code, _, called := run(t, v, map[string]string{"x-user-id-token": clientAssertion})
		if code != http.StatusUnauthorized || called {
			t.Fatalf("got %d called=%v", code, called)
		}
	})
}

// TestMiddleware_IdentityHolder covers what middleware.Logger actually reads
// to build its access-log line: the *IdentityHolder installed into context
// before Middleware runs, not IdentityFromContext (see IdentityHolder's own
// doc comment for why the two differ on a rejected request).
func TestMiddleware_IdentityHolder(t *testing.T) {
	key := newKey(t)
	v := staticValidator(key)
	user := sign(t, key, userClaims(nil))
	clientAssertion := sign(t, key, clientClaims(nil))

	t.Run("no tokens: holder stays empty", func(t *testing.T) {
		holder, _, _, _ := runWithHolder(t, v, nil)
		if holder.UserID != "" || holder.ClientID != "" {
			t.Fatalf("got %+v", holder)
		}
	})
	t.Run("m2m: holder carries the client id", func(t *testing.T) {
		holder, _, _, _ := runWithHolder(t, v, map[string]string{"x-jwt-assertion": clientAssertion})
		if holder.ClientID != testM2M || holder.UserID != "" {
			t.Fatalf("got %+v", holder)
		}
	})
	t.Run("on behalf of a user: holder carries the user UUID, not sub", func(t *testing.T) {
		holder, _, _, _ := runWithHolder(t, v, map[string]string{"x-jwt-assertion": clientAssertion, "x-user-id-token": user})
		if holder.UserID != "asgardeo-uuid-1" || holder.ClientID != testM2M {
			t.Fatalf("got %+v", holder)
		}
	})
	t.Run("invalid user token: holder stays empty, even though the client assertion alone would have validated", func(t *testing.T) {
		holder, code, _, called := runWithHolder(t, v, map[string]string{"x-jwt-assertion": clientAssertion, "x-user-id-token": "garbage"})
		if code != http.StatusUnauthorized || called {
			t.Fatalf("got %d called=%v", code, called)
		}
		if holder.UserID != "" || holder.ClientID != "" {
			t.Fatalf("a rejected request must never attribute the access log to an unproven claim, got %+v", holder)
		}
	})
	t.Run("invalid client assertion: holder stays empty", func(t *testing.T) {
		holder, code, _, called := runWithHolder(t, v, map[string]string{"x-jwt-assertion": "garbage"})
		if code != http.StatusUnauthorized || called {
			t.Fatalf("got %d called=%v", code, called)
		}
		if holder.UserID != "" || holder.ClientID != "" {
			t.Fatalf("got %+v", holder)
		}
	})
}

// Asgardeo publishes x5c certs Go's x509 parser rejects; NewValidator must
// still load the JWKS (the transport strips x5c) and validate real tokens.
func TestNewValidator_LoadsJWKSDespiteUnparseableX5C(t *testing.T) {
	key := newKey(t)
	jwk := map[string]any{
		"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
		"x5c": []string{"bm90LWEtY2VydA=="},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk}})
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.JWKSURL = srv.URL
	v, err := NewValidator(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	if _, err := v.ValidateUserToken(sign(t, key, userClaims(nil))); err != nil {
		t.Fatalf("token signed by the published key was rejected: %v", err)
	}
}

func TestNewValidator_FailsWhenJWKSUnreachable(t *testing.T) {
	cfg := testConfig()
	cfg.JWKSURL = "http://127.0.0.1:1/jwks"
	if _, err := NewValidator(context.Background(), cfg); err == nil {
		t.Fatal("want an error at startup when the JWKS cannot be fetched")
	}
}
