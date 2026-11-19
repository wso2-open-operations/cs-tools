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
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// JWTAssertionHeader carries the calling application's identity, set by the Choreo gateway after it has checked the
// caller's OAuth2 access token (client credentials, the app subscribed to the API).
const JWTAssertionHeader = "x-jwt-assertion"

// RequireClientID guards the read API: the request must carry a gateway assertion whose client_id (or azp) is in
// allowed. The gateway has already checked the access token, so the assertion is decoded, not verified, as
// entity-service and csm-integration-service do. An empty allowed list rejects every call, so a missing setting
// never leaves the routes open. A missing or unreadable assertion is 401, a client that is not allowed 403.
func RequireClientID(allowed []string, logger *slog.Logger) func(http.Handler) http.Handler {
	set := map[string]struct{}{}
	for _, id := range allowed {
		if id = strings.TrimSpace(id); id != "" {
			set[id] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := ClientIDFromAssertion(r.Header.Get(JWTAssertionHeader))
			if id == "" {
				logger.Warn("auth: missing or unreadable gateway assertion", "path", r.URL.Path)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if _, ok := set[id]; !ok {
				logger.Warn("auth: client not allowed on the read API", "path", r.URL.Path, "client_id", id)
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIDFromAssertion returns the client_id claim of a JWT (azp when there is none), without verifying it, or ""
// when the value is not a readable JWT or names no client.
func ClientIDFromAssertion(raw string) string {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return ""
	}
	var c struct {
		ClientID string `json:"client_id"`
		AZP      string `json:"azp"`
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return ""
	}
	if id := strings.TrimSpace(c.ClientID); id != "" {
		return id
	}
	return strings.TrimSpace(c.AZP)
}

// SplitClientIDs parses a comma-separated list of client ids, dropping blanks.
func SplitClientIDs(raw string) []string {
	var ids []string
	for _, id := range strings.Split(raw, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
