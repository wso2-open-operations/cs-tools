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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery turns a handler panic into a logged 500 with the standard error
// envelope instead of a dropped connection. It sits inside Logger, so the 500 is
// recorded in the access log, and outside the mux. http.ErrAbortHandler is
// re-panicked: it is net/http's own signal to abort a response and must not be
// swallowed.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			slog.ErrorContext(r.Context(), "panic recovered",
				"method", r.Method,
				"path", r.URL.Path,
				"panic", rec,
				"stack", string(debug.Stack()),
			)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"An internal server error occurred. Please try again later."}` + "\n"))
		}()
		next.ServeHTTP(w, r)
	})
}
