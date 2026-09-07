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

// Recovery is an HTTP middleware that recovers from a panic in any downstream
// handler, logs it, and writes the standard 500 error envelope instead of
// letting net/http's own recovery kill the connection with no body.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// net/http itself uses panic(http.ErrAbortHandler) as the
				// sanctioned "abort this response, no logging" signal (its
				// own recovery suppresses the stack trace for it); recovering
				// it here would instead turn a deliberate abort into a
				// logged "panic recovered" plus an attempted write on a
				// connection net/http already intends to drop
				// (AUDIT-FINDINGS A6) — including a "superfluous
				// WriteHeader" log if the downstream handler had already
				// written headers before panicking, which is an acceptable
				// side effect of re-panicking rather than something to
				// guard against here.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				slog.ErrorContext(r.Context(), "panic recovered", "panic", rec, "path", r.URL.Path)
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternal, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
