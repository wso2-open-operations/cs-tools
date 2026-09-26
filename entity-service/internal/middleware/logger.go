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

// Package middleware provides composable HTTP middleware for the entity service.
package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
)

// responseWriter wraps http.ResponseWriter to capture the status code written
// by the downstream handler so it can be included in the access log.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.NewResponseController see through this wrapper to the
// underlying ResponseWriter -- required for SetWriteDeadline/SetReadDeadline
// (used by handlers whose own work can legitimately exceed the server's
// global WriteTimeout, see AutoPublishAnnouncementRequest) to reach the real,
// deadline-capable writer several middleware layers down (this wrapper is
// itself nested inside recoveryWriter, see recovery.go's own Unwrap).
// ResponseController checks for a direct SetWriteDeadline implementation
// first and only falls back to Unwrap when that's absent, so a wrapper-local
// forwarding method (the previous approach here) is unnecessary and, worse,
// actively wrong: it would be found and called before Unwrap is ever
// consulted, while itself only being able to see one layer down.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// sanitizePath strips newline characters from a URL path to prevent log injection.
var sanitizePath = strings.NewReplacer("\n", `\n`, "\r", `\r`).Replace

// Logger is an HTTP middleware that logs each request's method, path, caller
// id, response status code, and elapsed time.
//
// callerId is the same Asgardeo user UUID csm-portal-backend/customer-portal
// backend-v2 already log for the request that reached them, when auth.
// Middleware (further inside this chain) validated an x-user-id-token --
// letting a request be traced across services by that one value. For a pure
// machine-to-machine caller (only a client-credentials x-jwt-assertion
// token, no end user in the loop) it falls back to that token's client id
// instead; "-" when neither validated (no tokens presented, or a token that
// failed validation, whose claims are unproven and never logged as if they
// were real -- see auth.Middleware's own comment on this). See
// auth.IdentityHolder's doc comment for why this needs its own holder rather
// than reading auth.IdentityFromContext(r.Context()) directly: Middleware
// returns early on a 401 without ever handing a mutated request back up to
// this (outer) middleware the normal way, and a rejected request must still
// appear in this access log.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		ctx, holder := auth.WithIdentityHolder(r.Context())
		next.ServeHTTP(rw, r.WithContext(ctx))
		log.Printf("%s %s correlationID=%s callerId=%s status=%d elapsed=%s", r.Method, sanitizePath(r.URL.Path), CorrelationIDFromContext(r.Context()), callerID(holder), rw.status, time.Since(start)) // #nosec G706 -- path sanitized above
	})
}

// callerID picks the value an access log line should attribute a request to:
// the caller's user UUID when present, else their client id, else "-".
func callerID(h *auth.IdentityHolder) string {
	if h == nil {
		return "-"
	}
	if h.UserID != "" {
		return h.UserID
	}
	if h.ClientID != "" {
		return h.ClientID
	}
	return "-"
}
