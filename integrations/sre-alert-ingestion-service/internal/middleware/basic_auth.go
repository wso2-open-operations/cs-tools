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
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// basicAuthRealm is advertised in the WWW-Authenticate header on every 401
// this middleware produces.
const basicAuthRealm = "sre-alert-ingestion-service"

// dummyAuthPassword is bcrypt-compared against on every unknown username, so
// an unrecognized username costs the same wall-clock time as a recognized
// one with a wrong password. Without this, an unknown username would return
// 401 near-instantly (no bcrypt work done) while a known username takes
// bcrypt's deliberately-slow compare to fail, and that timing gap leaks
// which usernames are valid to anyone who can time the response. The value
// itself is arbitrary and not a secret — it is never a real credential and
// is only ever compared against itself's own hash, which always matches, so
// this never grants access.
const dummyAuthPassword = "sre-alert-ingestion-service-dummy-password-for-timing-parity" // #nosec G101 -- not a credential, see doc comment above

// authenticatedUsernameKey is the unexported context key BasicAuth stores
// the successfully-authenticated username under, following this package's
// existing context-key convention (see correlation.go's correlationIDKey) —
// an unexported empty-struct type so no other package can accidentally
// collide with or forge this key.
type authenticatedUsernameKey struct{}

// AuthenticatedUsernameFromContext returns the username BasicAuth
// authenticated this request as, and true, or ("", false) if BasicAuth never
// ran against ctx (or the request was rejected before reaching the wrapped
// handler). Handlers behind this middleware use this to cross-check the
// identity they authenticated as against a claimed value in the request
// itself (e.g. AlertRequest.Source) — see internal/handler.AlertHandler's
// requireAuthenticatedSource for why that check exists: HTTP Basic Auth on
// its own only proves *who* is calling, not that the caller is entitled to
// claim any particular Source.
func AuthenticatedUsernameFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(authenticatedUsernameKey{}).(string)
	return v, ok
}

// WithAuthenticatedUsername returns a copy of ctx carrying username as the
// authenticated identity, retrievable via AuthenticatedUsernameFromContext —
// the same context key BasicAuth itself sets on a successful auth. This
// exists for handler-level unit tests (see internal/handler's alerts_test.go
// and adapter_*_test.go) that construct *http.Request values directly rather
// than routing them through the real BasicAuth middleware, mirroring
// WithCorrelationID's identical role for CorrelationID in this same package.
func WithAuthenticatedUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, authenticatedUsernameKey{}, username)
}

// BasicAuthUsers is the parsed, ready-to-check form of SRE_ALERT_AUTH_USERS:
// username -> bcrypt hash of that username's password.
type BasicAuthUsers map[string]string

// ParseBasicAuthUsers parses the SRE_ALERT_AUTH_USERS environment variable
// format: comma-separated "username:bcryptHash" pairs, e.g.
// "datadog:$2a$10$...,grafana:$2a$10$...".
//
// Splitting is unambiguous: pairs are split on "," first, then each pair is
// split on its FIRST ":" — so a username can never itself contain "," (it
// would already have been split into a separate, malformed pair) or ":"
// (splitting on the first one always takes the shortest possible username).
// A bcrypt hash never contains either character, so there is exactly one
// valid parse of a well-formed entry.
//
// Any malformed entry — wrong shape, empty username, or a value that
// doesn't look like a bcrypt hash — is an error. Callers must fail fast on
// that error rather than skip the bad entry: a silently-dropped malformed
// entry is a silently-unauthenticated source.
func ParseBasicAuthUsers(raw string) (BasicAuthUsers, error) {
	users := make(BasicAuthUsers)
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			return nil, fmt.Errorf("empty entry in SRE_ALERT_AUTH_USERS")
		}
		username, hash, ok := strings.Cut(pair, ":")
		if !ok {
			return nil, fmt.Errorf("malformed entry %q in SRE_ALERT_AUTH_USERS: expected username:bcryptHash", pair)
		}
		if username == "" {
			return nil, fmt.Errorf("malformed entry %q in SRE_ALERT_AUTH_USERS: empty username", pair)
		}
		if !looksLikeBcryptHash(hash) {
			return nil, fmt.Errorf("malformed entry for username %q in SRE_ALERT_AUTH_USERS: value is not a bcrypt hash", username)
		}
		// BasicAuth's username-enumeration defense compares every unknown
		// username against a fixed dummyHash at bcrypt.DefaultCost, so that
		// path takes the same wall-clock time as a known-username wrong-
		// password compare. That only holds if every real hash here was
		// also generated at DefaultCost — a hash at a different cost would
		// make its own username distinguishable by timing alone, regardless
		// of password correctness. Reject rather than silently accept a
		// mismatched cost.
		if cost, cerr := bcrypt.Cost([]byte(hash)); cerr != nil || cost != bcrypt.DefaultCost {
			return nil, fmt.Errorf("malformed entry for username %q in SRE_ALERT_AUTH_USERS: hash must be bcrypt.DefaultCost (%d)", username, bcrypt.DefaultCost)
		}
		if _, dup := users[username]; dup {
			return nil, fmt.Errorf("duplicate username %q in SRE_ALERT_AUTH_USERS", username)
		}
		users[username] = hash
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("SRE_ALERT_AUTH_USERS parsed to zero users")
	}
	return users, nil
}

// looksLikeBcryptHash reports whether hash has bcrypt's recognizable shape
// ($2a$/$2b$/$2y$ prefix, cost, and a fixed-length base64-ish body) without
// doing a full compare — a real shape-and-cost check is bcrypt's own job at
// verification time, this just catches an obviously-wrong value (e.g. a
// plaintext password) at startup instead of at first request.
func looksLikeBcryptHash(hash string) bool {
	if len(hash) != 60 {
		return false
	}
	return strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") || strings.HasPrefix(hash, "$2y$")
}

// BasicAuth is an HTTP middleware enforcing HTTP Basic Auth against the
// given set of users. It is this service's sole inbound-authentication
// enforcement point on AKS, where there is no gateway in front of it to
// trust — see cmd/server/main.go's wiring comment for the full rationale.
//
// Missing/malformed Authorization header, unknown username, and a real
// username/password mismatch all produce the same generic 401 response
// (correct HTTP semantics for Basic Auth, and doesn't leak which check
// failed). Rejected attempts are logged at Warn with the attempted username
// and remote address — never the password or any hash.
func BasicAuth(users BasicAuthUsers) func(http.Handler) http.Handler {
	// Precomputed once so every rejected/unknown-username request pays the
	// same bcrypt cost as a real, known-username mismatch. bcrypt.GenerateFromPassword
	// with the default cost cannot fail for a password of this length.
	dummyHash, err := bcrypt.GenerateFromPassword([]byte(dummyAuthPassword), bcrypt.DefaultCost)
	if err != nil {
		panic("middleware: failed to precompute dummy bcrypt hash: " + err.Error())
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok {
				reject(w, r, "")
				return
			}

			hash, known := users[username]
			if !known {
				// Compare against the dummy hash anyway — always a
				// mismatch, but it burns the same wall-clock time as a
				// known-username wrong-password compare below, so an
				// observer timing responses cannot distinguish "unknown
				// username" from "known username, wrong password".
				_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
				reject(w, r, username)
				return
			}

			if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
				reject(w, r, username)
				return
			}

			ctx := context.WithValue(r.Context(), authenticatedUsernameKey{}, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func reject(w http.ResponseWriter, r *http.Request, attemptedUsername string) {
	slog.WarnContext(r.Context(), "rejected basic auth attempt",
		"attemptedUsername", attemptedUsername, "remoteAddr", r.RemoteAddr)
	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", basicAuthRealm))
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}
