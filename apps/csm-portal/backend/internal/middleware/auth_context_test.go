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

// TestAuthWithContext_ValidatesAgainstJWKS builds the validating middleware
// with a caller-supplied context against a local JWKS endpoint, checks a
// signed token is accepted, and cancels the context afterwards (the server
// does the same on shutdown).
func TestAuthWithContext_ValidatesAgainstJWKS(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "test-key"
	jwks := map[string]any{"keys": []map[string]any{{
		"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := Config{JWKSEndpoint: srv.URL, Issuer: "https://issuer.example.com", Audiences: []string{"api.example.com"}, ClockSkew: 5 * time.Second, TokenValidatorEnabled: true}
	mw := AuthWithContext(ctx, cfg)

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": cfg.Issuer, "aud": "api.example.com", "exp": time.Now().Add(time.Hour).Unix(),
		"email": "jane.doe@example.com", "userid": "00000000-0000-0000-0000-000000000000",
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	var seen *UserInfo
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = UserInfoFromContext(r.Context()) }))
	r := httptest.NewRequest(http.MethodGet, "/cases", nil)
	r.Header.Set(jwtAssertionHeader, signed)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if seen == nil || seen.Email != "jane.doe@example.com" {
		t.Fatalf("signed token not accepted: status %d", w.Code)
	}
	cancel()
}
