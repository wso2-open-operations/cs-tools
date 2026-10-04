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
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// internalErrorBody is the standard 500 envelope, byte-identical to what the
// handler package's writeError(w, 500, ErrMsgInternal) produces. It is
// duplicated here rather than imported because handler depends on this
// package, not the other way round.
const internalErrorBody = `{"message":"An internal server error occurred. Please try again later."}` + "\n"

// Recover converts a panic in any downstream handler into a logged 500 with
// the standard error envelope. Without it net/http's own per-connection
// recovery logs the stack and closes the socket, so the client sees a dropped
// connection rather than an HTTP response and the Logger middleware never
// records the request.
//
// It sits directly inside SecurityHeaders and outside CorrelationID, so it
// cannot read the correlation id from the request context; it reads the
// response header CorrelationID has already set instead, which is the same
// value and is available regardless of where in the chain the panic happened.
//
// http.ErrAbortHandler is re-panicked, as net/http documents: it is the
// sentinel a handler uses to abort a response deliberately without a log line.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &recoverWriter{ResponseWriter: w}
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			slog.ErrorContext(r.Context(), "handler panic recovered",
				"method", r.Method,
				"path", r.URL.Path,
				"correlationID", w.Header().Get(correlationIDHeader),
				"panic", fmt.Sprint(rec),
				"stack", string(debug.Stack()),
			)
			if rw.wroteHeader {
				// The status line is already on the wire; nothing coherent
				// can be sent after it. The log line above is the record.
				return
			}
			h := w.Header()
			h.Del("Content-Length")
			h.Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(internalErrorBody))
		}()
		next.ServeHTTP(rw, r)
	})
}

// recoverWriter records whether a status line has been committed, so the
// deferred recovery knows whether a 500 can still be written.
type recoverWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (rw *recoverWriter) WriteHeader(code int) {
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *recoverWriter) Write(b []byte) (int, error) {
	rw.wroteHeader = true
	return rw.ResponseWriter.Write(b)
}

// Flush keeps http.Flusher reachable through the wrapper (see the identical
// note on Logger's responseWriter).
func (rw *recoverWriter) Flush() {
	rw.wroteHeader = true
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (rw *recoverWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}
