// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

// MinWakeTokenLen rejects a token too short to resist guessing; `openssl rand -hex 32` gives 64.
const MinWakeTokenLen = 32

const bearerPrefix = "bearer "

// RequireWakeToken guards /alertz with the shared ALERT_CORE_WAKE_TOKEN, sent as Authorization: Bearer <token>; an empty token rejects every call, so a missing secret never leaves the route open.
func RequireWakeToken(token string, logger *slog.Logger) func(http.Handler) http.Handler {
	return RequireToken(token, "wake token", logger)
}

// RequireToken guards a route with a shared secret sent as Authorization: Bearer <token>; name labels it in logs. An
// empty token rejects every call, so a missing secret never leaves the route open.
func RequireToken(token, name string, logger *slog.Logger) func(http.Handler) http.Handler {
	// Comparing digests keeps the comparison constant-time even when the lengths differ.
	want := sha256.Sum256([]byte(token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, ok := bearerToken(r)
			if !ok || token == "" {
				logger.Warn("auth: missing or malformed "+name, "path", r.URL.Path)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			digest := sha256.Sum256([]byte(got))
			if subtle.ConstantTimeCompare(digest[:], want[:]) != 1 {
				logger.Warn("auth: wrong "+name, "path", r.URL.Path)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bearerToken returns the token from Authorization: Bearer <token>; the scheme is case-insensitive (RFC 7235).
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if len(h) <= len(bearerPrefix) || !strings.EqualFold(h[:len(bearerPrefix)], bearerPrefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(bearerPrefix):])
	return token, token != ""
}
