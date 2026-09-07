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

// corsAllowedHeaders lists every request header the webapp may send.
// Authorization carries the Asgardeo bearer access token the frontend
// attaches to every request (see webapp's api/client.ts) — in a real
// deployment this is validated by the Choreo gateway before the request ever
// reaches this backend (D7: the backend itself parses no auth at all), but
// the header still has to clear the browser's CORS preflight to get there.
// Content-Type is required for JSON request bodies: application/json is not
// a CORS-safelisted value, so a POST preflight fails without it.
const corsAllowedHeaders = "Content-Type, Authorization"

// corsAllowedMethods covers every verb this API's routes use.
const corsAllowedMethods = "GET, POST, OPTIONS"

// CORS returns an HTTP middleware handling cross-origin browser requests. In
// a real deployment Choreo's API gateway supplies these headers itself,
// making this middleware a no-op there (D14); it matters only for local
// development, where the webapp (Vite on :5173) calls this backend (:8080)
// directly with no gateway in front of it.
//
// MUST wrap the rest of the chain and run ahead of anything that could
// reject a request: a CORS preflight is an OPTIONS request that must always
// receive CORS headers so the browser can decide whether to send the real
// request at all; if the preflight itself were ever rejected the browser
// reports "blocked by CORS policy" no matter what the real cause was.
//
// allowedOrigins is an allow-list of browser Origins; fail-closed by
// default — an empty list allows *no* cross-origin browser request through
// (Access-Control-Allow-Origin is never set, so the browser blocks it),
// rather than reflecting any Origin back. This backend is authless (D7) and
// has no cookie-based session for a browser to attach automatically, and
// Access-Control-Allow-Credentials is deliberately never set below, but
// fail-closed is kept anyway as defense-in-depth rather than relying on that
// precondition never changing. Local development sets this explicitly (see
// .env.example) rather than relying on an implicit any-origin fallback.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				// Set regardless of whether origin is actually allowed: a
				// denied-origin response varies by Origin exactly as much as
				// an allowed one does (they produce different
				// Access-Control-Allow-Origin values), so a cache that
				// doesn't see Vary here could reuse a denied response for a
				// later, legitimately allowed origin's request.
				w.Header().Add("Vary", "Origin")
				if allowed[origin] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
				w.Header().Set("Access-Control-Max-Age", "3600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
