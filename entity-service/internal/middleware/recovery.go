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
	"encoding/json"
	"log"
	"net/http"
)

// recoveryWriter wraps http.ResponseWriter to track whether WriteHeader has been called,
// so the panic handler can skip writing a body if the response has already started.
type recoveryWriter struct {
	http.ResponseWriter
	headerWritten bool
}

func (rw *recoveryWriter) WriteHeader(code int) {
	rw.headerWritten = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *recoveryWriter) Write(b []byte) (int, error) {
	rw.headerWritten = true
	return rw.ResponseWriter.Write(b)
}

// Recovery is an HTTP middleware that catches any panic in a downstream handler,
// logs it, and writes a JSON 500 response so the server goroutine keeps running.
// If the handler already started writing a response before panicking, the error
// body is skipped to avoid corrupting a partially-written response.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &recoveryWriter{ResponseWriter: w}
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("Panic recovered (type: %T)", rec)
				if !rw.headerWritten {
					rw.ResponseWriter.Header().Set("Content-Type", "application/json")
					rw.ResponseWriter.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(rw.ResponseWriter).Encode(struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
					}{Code: http.StatusInternalServerError, Message: "internal server error"})
				}
			}
		}()
		next.ServeHTTP(rw, r)
	})
}
