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

// corsAllowedHeaders lists every request header a browser client may need to
// send to this service. Authorization is what the frontend actually sets
// (see useAuthApiClient.ts) — Choreo's gateway is
// what translates it into x-jwt-assertion before this backend ever sees it in
// a real deployment (confirmed: Choreo's own CORS response for :8080 already
// allow-lists "authorization", not "x-jwt-assertion") — but x-jwt-assertion is
// allow-listed too, both for local testing that bypasses the gateway and as
// defense-in-depth. Content-Type is required for JSON request bodies:
// application/json is not a CORS-safelisted value, so a POST/PATCH preflight
// fails without it.
const corsAllowedHeaders = "Content-Type, Authorization, x-jwt-assertion, x-user-id-token, X-CSM-Correlation-ID"

// corsAllowedMethods is the REST API's verb set. Advertising a method a given
// path has no route for is harmless — the route simply 404s if called.
const corsAllowedMethods = "GET, POST, PATCH, DELETE, OPTIONS"

// CORS returns an HTTP middleware handling cross-origin browser requests. It
// wraps the server's handler (see cmd/server/main.go). In a real deployment
// Choreo's API gateway supplies these headers itself, making this a no-op
// there; it matters when the gateway isn't in the path — local development,
// where the browser calls the service directly.
//
// MUST wrap Auth, not be wrapped by it: a CORS preflight is an OPTIONS
// request with no x-jwt-assertion header at all, so if Auth ran first it
// would reject every preflight with 401 before the browser ever saw a CORS
// header — which the browser reports as "blocked by CORS policy", masking
// the real cause. Worse, Auth sits *before* Logger in this backend's chain,
// so such a rejection isn't even logged, making it invisible server-side.
// SecurityHeaders, however, wraps *this* — see cmd/server/main.go's
// Handler chain — so its headers are present on every response, including
// a preflight, which Auth-then-CORS ordering would otherwise skip since a
// preflight never reaches the handlers CORS itself wraps. See
// apps/customer-portal/backend-v2's identically-named middleware, whose doc
// comment this mirrors.
//
// allowedOrigins is an allow-list of browser Origins; fail-closed by
// default — an empty list allows *no* cross-origin browser request through
// (Access-Control-Allow-Origin is never set, so the browser blocks it),
// rather than reflecting any Origin back. Deliberately stricter than
// apps/customer-portal/backend-v2's identically-named middleware, which
// does default open, safe there only for the same reason it'd be safe
// here too: this backend authenticates via a caller-supplied
// x-jwt-assertion header, never cookies, so there is no session credential
// for a browser to attach automatically, and Access-Control-Allow-Credentials
// is deliberately never set below — an open default wouldn't let a
// malicious page forge a request with a victim's identity today. It's
// still fail-closed rather than matching backend-v2, because that
// precondition is easy to silently invalidate later (e.g. a future change
// that exposes the token to page-readable JS) and CORS should not be the
// only thing standing between that mistake and real impact. Local
// development sets this explicitly (see .env.example) rather than relying
// on an implicit any-origin fallback.
//
// If this backend ever adds cookie-based auth, allowedOrigins MUST remain
// a real non-empty allow-list — an unrestricted origin plus credentials
// lets any site read authenticated responses on the victim's behalf.
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
