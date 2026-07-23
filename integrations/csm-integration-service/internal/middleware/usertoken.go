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

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/entity"
)

// UserIDToken is an HTTP middleware that optionally forwards an end-user identity
// token supplied by the caller. This service is M2M-only and does not require an
// x-user-id-token header — most requests will not carry one, and that's fine. If a
// caller does supply one, it is stored in the entity client context so it rides
// along on the outgoing entity-service request, for callers that need
// entity-service's ServiceNow-backed operations (which require a forwarded
// end-user identity and will reject an M2M-only request with no such token).
func UserIDToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.Header.Get("x-user-id-token"); token != "" {
			r = r.WithContext(entity.WithUserIDToken(r.Context(), token))
		}
		next.ServeHTTP(w, r)
	})
}
