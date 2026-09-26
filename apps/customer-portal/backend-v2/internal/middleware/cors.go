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

import "net/http"

// corsAllowedHeaders lists every request header the frontend may need to send
// on a cross-origin call: the bearer token, the JWT assertion and
// impersonation headers Auth reads, the correlation ID header, and the
// standard content-type/upload headers used by JSON and binary (zip upload)
// request bodies.
const corsAllowedHeaders = "Content-Type, Authorization, x-jwt-assertion, x-user-id-token, X-CSM-Correlation-ID"

// CORS returns an HTTP middleware that handles cross-origin requests from the
// browser-based frontend. It MUST be the outermost middleware in the chain
// (wrapping Auth, not wrapped by it): a CORS preflight is an OPTIONS request
// with no Authorization/JWT header at all, so if Auth ran first it would
// reject every preflight with 401 before the browser ever saw a CORS header
// — which is exactly what a browser reports as a "blocked by CORS policy"
// error, even though the real cause is auth rejecting the preflight, not a
// CORS misconfiguration.
//
// allowedOrigins is a comma-separated-then-split allow-list of browser
// Origins (see splitComma in cmd/server/main.go); an empty list allows any
// origin, matching this backend's other Origin-gated feature
// (handler.NewWebSocketHandler) and is intended for local development only.
//
// Deliberately never sets Access-Control-Allow-Credentials: this backend
// authenticates via a caller-supplied x-jwt-assertion header (see
// middleware.Auth), never cookies, so there is no session credential for a
// browser to attach automatically — reflecting an arbitrary Origin back is
// safe only as long as that stays true. If this backend ever adds
// cookie-based auth, allowedOrigins MUST become a real non-empty allow-list
// before Access-Control-Allow-Credentials could safely be added, since the
// two together (any origin + credentials) let any site read authenticated
// responses on the victim's behalf.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (len(allowed) == 0 || allowed[origin]) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
				w.Header().Set("Access-Control-Max-Age", "3600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
