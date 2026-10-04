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

package server

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// testInternalClientID is the one client id withTestAuth allow-lists as an
// internal caller. A request stamped by asInternalClient resolves as
// Unrestricted and so passes the caller-identity gate and every internalOnly
// route; a request carrying no token at all is refused with 401 before any
// handler runs (see callerIdentityMiddleware).
const testInternalClientID = "test-internal-client"

// testAuthConfig starts a throwaway local JWKS server and returns the three
// Auth* fields needed to satisfy config.Config.Validate/NewRouter -- token
// validation always runs now (there's no flag to turn it off), so every test
// in this package that builds a real Config for NewRouter needs a real,
// reachable JWKS. No test in this package sends a user token, so the key
// itself is never used to sign one -- only its presence matters, to let
// auth.NewValidator load at least one key and not panic. Tests that need to
// reach a handler stamp the request with asInternalClient instead.
func testAuthConfig(t *testing.T) (issuer, jwksURL string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{
			"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
		}}})
	}))
	t.Cleanup(srv.Close)
	return "https://test-issuer.invalid/oauth2/token", srv.URL
}

// withTestAuth fills in cfg's Auth* fields in place via testAuthConfig and
// allow-lists testInternalClientID as an internal caller.
func withTestAuth(t *testing.T, cfg *config.Config) {
	t.Helper()
	cfg.AuthIssuer, cfg.AuthJWKSURL = testAuthConfig(t)
	cfg.AuthUserTokenAudiences = []string{"test-spa"}
	cfg.AuthInternalClientIDs = map[string]bool{testInternalClientID: true}
}

var (
	assertionKeyOnce sync.Once
	assertionKey     *rsa.PrivateKey
)

// asInternalClient stamps req with an x-jwt-assertion naming
// testInternalClientID and returns it, so a router built with withTestAuth
// treats the request as coming from an allow-listed internal service. The
// assertion is signed (RS256) with a key generated once per test binary; the
// validator only decodes this header (see auth.Validator.ExtractClientID), so
// no JWKS needs to publish the key.
func asInternalClient(t *testing.T, req *http.Request) *http.Request {
	t.Helper()
	assertionKeyOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate assertion key: %v", err)
		}
		assertionKey = key
	})
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":       "https://test-issuer.invalid/oauth2/token",
		"client_id": testInternalClientID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = "k1"
	signed, err := tok.SignedString(assertionKey)
	if err != nil {
		t.Fatalf("sign assertion: %v", err)
	}
	req.Header.Set("x-jwt-assertion", signed)
	return req
}
