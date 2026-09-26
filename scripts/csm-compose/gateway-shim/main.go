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
//
// gateway-shim is a minimal reverse proxy for the docker-compose local dev
// stack described in apps/csm-portal/README.md.
//
// LOCAL DEVELOPMENT ONLY. It stands in for the one job Choreo's real API
// gateway does in front of every backend in production: validate the
// browser's `Authorization: Bearer <token>` and hand the backend a
// `x-jwt-assertion` header instead. This compose stack has no gateway in
// the request path at all, so without this shim both BFFs
// (apps/csm-portal/backend and apps/customer-portal/backend-v2) 401 every
// request forever -- their auth middleware
// (internal/middleware/auth.go) only ever reads `x-jwt-assertion`, and
// neither webapp ever sends that header itself.
//
// In production, Choreo validates the Authorization bearer and then
// SYNTHESIZES its own x-jwt-assertion from claims Choreo's directory holds,
// rather than forwarding the access token verbatim -- see
// scripts/csm-debug/gateway-shim.py in the planning repo for that fuller
// version, built for a topology where the access token and the claims the
// BFF needs come from two different places. This compose stack's mock OIDC
// provider (scripts/csm-compose/mock-oidc) is simpler than that: it signs
// the access_token and id_token from the identical claims map (email,
// userid, groups, roles), and the access token's `aud` is already the
// OAuth client_id used at login, which is exactly the AUTH_AUDIENCE each
// BFF is configured with in docker-compose.yml. That was verified
// empirically (not assumed) by calling each BFF directly with the mock
// provider's access token copied verbatim into x-jwt-assertion -- both
// returned 200 with real seeded user data. So this shim's job reduces to a
// pure header rename: strip the "Bearer " prefix and set x-jwt-assertion
// to whatever is left. No claim synthesis, decoding, or re-signing needed
// for this stack.
//
// This is NOT a real gateway: it does no token validation of its own (the
// BFF behind it still does, against mock-oidc's JWKS) and must never be
// pointed at anything other than this local compose stack.
package main

import (
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

const jwtAssertionHeader = "x-jwt-assertion"

func main() {
	port := envOrDefault("PORT", "8888")
	upstreamURL := os.Getenv("UPSTREAM_URL")
	if upstreamURL == "" {
		log.Fatal("gateway-shim: UPSTREAM_URL must be set")
	}

	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		// %q quotes the string using Go syntax, escaping control characters
		// (including newlines) rather than emitting them raw, so this can't
		// be used to forge extra log lines -- the usual log-injection
		// concern the linter is flagging. Not user input either:
		// upstreamURL is the UPSTREAM_URL env var (see below).
		log.Fatalf("gateway-shim: invalid UPSTREAM_URL %q: %v", upstreamURL, err) // #nosec G706 -- %q escapes control chars/newlines, so no forged log lines are possible
	}

	// UPSTREAM_URL is fixed, operator-controlled container config (set once
	// in docker-compose.yml, e.g. http://csm-portal-backend:8080) -- never
	// derived from a request, header, query param, or other caller-supplied
	// input. Same shape as the registry.go G304 precedent (fixed deployment
	// config, not user input).
	proxy := httputil.NewSingleHostReverseProxy(upstream) // #nosec G704 -- UPSTREAM_URL is fixed local-dev compose config (docker-compose.yml env var), never derived from a request, not user-controlled input

	// httputil.ReverseProxy already strips hop-by-hop headers and rewrites
	// the request's scheme/host/path to the upstream -- wrap its default
	// Director rather than replacing it, so that behaviour is kept and the
	// only thing added here is the header translation.
	defaultDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		defaultDirector(r)
		addJWTAssertion(r)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("gateway-shim: proxy error", "method", r.Method, "path", r.URL.Path, "err", err)
		http.Error(w, `{"message":"gateway-shim: upstream unreachable"}`, http.StatusBadGateway)
	}

	slog.Info("gateway-shim: listening", "port", port, "upstream", upstreamURL)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           logRequests(proxy),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

// addJWTAssertion is this shim's one job: turn the browser's
// `Authorization: Bearer <token>` into the `x-jwt-assertion` header the BFF
// requires, mirroring what Choreo's gateway does in production. If the
// caller already set x-jwt-assertion itself (e.g. a curl example from
// apps/csm-portal/README.md that sets it by hand), that is left untouched
// -- this only fills it in when missing.
func addJWTAssertion(r *http.Request) {
	if r.Header.Get(jwtAssertionHeader) != "" {
		return
	}
	auth := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok || token == "" {
		return
	}
	r.Header.Set(jwtAssertionHeader, token)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("gateway-shim: request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
