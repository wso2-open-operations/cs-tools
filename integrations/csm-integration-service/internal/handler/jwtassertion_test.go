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

package handler

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// testJWTAssertion builds an unsigned, decode-only JWT-shaped string carrying
// clientID as its client_id claim -- everything clientIDFromJWTAssertion
// ever reads. Good enough for tests: that function never verifies a
// signature, same as the gateway-trust assumption it documents.
func testJWTAssertion(t *testing.T, clientID string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, err := json.Marshal(jwtAssertionClaims{ClientID: clientID})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + "."
}

func TestClientIDFromJWTAssertion(t *testing.T) {
	t.Run("reads client_id claim", func(t *testing.T) {
		got := clientIDFromJWTAssertion(testJWTAssertion(t, "client-abc"))
		if got != "client-abc" {
			t.Errorf("clientID = %q, want %q", got, "client-abc")
		}
	})

	t.Run("falls back to azp when client_id is absent", func(t *testing.T) {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"azp":"client-xyz"}`))
		got := clientIDFromJWTAssertion(header + "." + payload + ".")
		if got != "client-xyz" {
			t.Errorf("clientID = %q, want %q", got, "client-xyz")
		}
	})

	t.Run("empty header returns empty", func(t *testing.T) {
		if got := clientIDFromJWTAssertion(""); got != "" {
			t.Errorf("clientID = %q, want empty", got)
		}
	})

	t.Run("malformed token (not three segments) returns empty", func(t *testing.T) {
		if got := clientIDFromJWTAssertion("not-a-jwt"); got != "" {
			t.Errorf("clientID = %q, want empty", got)
		}
	})

	t.Run("unparseable payload segment returns empty", func(t *testing.T) {
		if got := clientIDFromJWTAssertion("a.!!!not-base64!!!.c"); got != "" {
			t.Errorf("clientID = %q, want empty", got)
		}
	})

	t.Run("payload with neither claim returns empty", func(t *testing.T) {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{}`))
		got := clientIDFromJWTAssertion(header + "." + payload + ".")
		if got != "" {
			t.Errorf("clientID = %q, want empty", got)
		}
	})
}
