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
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
)

// ReadOnly marks the request context so the db.Router serves the handler's
// queries from the read pool. A route opts in explicitly by wrapping its
// handler; nothing is marked by default, and a handler that writes must never
// be wrapped (the read pool's sessions are read-only and reject writes).
func ReadOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(db.WithReadOnly(r.Context())))
	})
}

// ReadOnlyFunc is ReadOnly for routes registered with mux.HandleFunc.
func ReadOnlyFunc(next http.HandlerFunc) http.HandlerFunc {
	return ReadOnly(next).ServeHTTP
}
