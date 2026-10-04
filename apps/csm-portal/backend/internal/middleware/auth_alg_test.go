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
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestExtractUserInfo_AcceptsOnlyRS256 checks the validating path pins the
// signing algorithm: the same RSA key signing with RS512 is refused.
func TestExtractUserInfo_AcceptsOnlyRS256(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{TokenValidatorEnabled: true, Issuer: "https://issuer.example.com", Audiences: []string{"api.example.com"}, ClockSkew: 5 * time.Second}
	keyFunc := func(*jwt.Token) (any, error) { return &key.PublicKey, nil }
	sign := func(m jwt.SigningMethod, k any) string {
		claims := jwt.MapClaims{
			"iss":    cfg.Issuer,
			"aud":    "api.example.com",
			"exp":    time.Now().Add(time.Hour).Unix(),
			"email":  "jane.doe@example.com",
			"userid": "00000000-0000-0000-0000-000000000000",
		}
		s, err := jwt.NewWithClaims(m, claims).SignedString(k)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	if _, err := extractUserInfo(sign(jwt.SigningMethodRS256, key), cfg, keyFunc); err != nil {
		t.Fatalf("RS256 token rejected: %v", err)
	}
	if _, err := extractUserInfo(sign(jwt.SigningMethodRS512, key), cfg, keyFunc); err == nil {
		t.Fatal("RS512 token accepted, want only RS256")
	}
	if _, err := extractUserInfo(sign(jwt.SigningMethodHS256, []byte("shared-secret")), cfg, keyFunc); err == nil {
		t.Fatal("HS256 token accepted, want only RS256")
	}
}
