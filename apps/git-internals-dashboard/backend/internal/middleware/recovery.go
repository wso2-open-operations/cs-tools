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
	"log/slog"
	"net/http"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/apierror"
)

// committedResponseWriter tracks whether the downstream handler has already
// started writing a response (WriteHeader or Write), so Recovery knows
// whether it's still safe to write the 500 envelope.
type committedResponseWriter struct {
	http.ResponseWriter
	committed bool
}

// WriteHeader marks the response committed before delegating.
func (w *committedResponseWriter) WriteHeader(code int) {
	w.committed = true
	w.ResponseWriter.WriteHeader(code)
}

// Write marks the response committed before delegating.
func (w *committedResponseWriter) Write(b []byte) (int, error) {
	w.committed = true
	return w.ResponseWriter.Write(b)
}

// Unwrap exposes the underlying ResponseWriter so http.NewResponseController
// (used by PostSyncRuns to extend its write deadline) can see through this
// wrapper — without it, SetWriteDeadline/SetReadDeadline would silently
// no-op for every route, since Recovery is outermost in the handler chain
// (cmd/server/main.go).
func (w *committedResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Recovery is an HTTP middleware that recovers from a panic in any downstream
// handler, logs it, and writes the standard 500 error envelope instead of
// letting net/http's own recovery kill the connection with no body.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cw := &committedResponseWriter{ResponseWriter: w}
		defer func() {
			if rec := recover(); rec != nil {
				// net/http itself uses panic(http.ErrAbortHandler) as the
				// sanctioned "abort this response, no logging" signal (its
				// own recovery suppresses the stack trace for it); recovering
				// it here would instead turn a deliberate abort into a
				// logged "panic recovered" plus an attempted write on a
				// connection net/http already intends to drop — including a
				// "superfluous WriteHeader" log if the downstream handler had
				// already written headers before panicking, which is an
				// acceptable side effect of re-panicking rather than
				// something to guard against here.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				slog.ErrorContext(r.Context(), "panic recovered", "panic", rec, "path", r.URL.Path)
				if cw.committed {
					// The response already started: net/http keeps whatever
					// status/headers/body the handler already wrote
					// regardless of anything written here, so apierror.Write
					// could only append a JSON error envelope onto a partial
					// (possibly 200 OK) response. Log-and-terminate instead.
					return
				}
				apierror.Write(cw, http.StatusInternalServerError, apierror.CodeInternal, "internal server error")
			}
		}()
		next.ServeHTTP(cw, r)
	})
}
