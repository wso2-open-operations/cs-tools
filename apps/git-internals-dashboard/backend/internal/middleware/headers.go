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

// SecurityHeaders returns an HTTP middleware that sets every entry in
// headers on every response before calling next. The header set is plain
// data (app-config.yaml's securityHeaders map, see internal/appconfig), so
// adding, removing, or changing a header is a config edit and restart, never
// a code change.
//
// MUST run outside CORS in the handler chain (cmd/server/main.go) even
// though CORS's OPTIONS preflight branch short-circuits before reaching
// next: headers are set here unconditionally before next.ServeHTTP is
// called, so they are already present on the ResponseWriter by the time any
// downstream handler (including CORS's preflight branch, a 404 from the
// mux, or apierror's error envelope) calls WriteHeader.
func SecurityHeaders(headers map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for name, value := range headers {
				w.Header().Set(name, value)
			}
			next.ServeHTTP(w, r)
		})
	}
}
