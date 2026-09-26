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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// testAuthConfig starts a throwaway local JWKS server and returns the three
// Auth* fields needed to satisfy config.Config.Validate/NewRouter -- token
// validation always runs now (there's no flag to turn it off), so every test
// in this package that builds a real Config for NewRouter needs a real,
// reachable JWKS. None of the tests using this actually send a token (they
// all rely on "no tokens at all passes through unrejected"), so the key
// itself is never used to sign anything -- only its presence matters, to let
// auth.NewValidator load at least one key and not panic.
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

// withTestAuth fills in cfg's Auth* fields in place via testAuthConfig.
func withTestAuth(t *testing.T, cfg *config.Config) {
	t.Helper()
	cfg.AuthIssuer, cfg.AuthJWKSURL = testAuthConfig(t)
	cfg.AuthUserTokenAudiences = []string{"test-spa"}
}
