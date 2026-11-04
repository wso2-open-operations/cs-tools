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

package server

import (
	"errors"
	"log"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// internalOnly wraps a handler so only internal callers (an internal client
// credential, or a user token whose active user rows are all INTERNAL) reach
// it; everyone else gets 403 (or 401/503 when the identity itself cannot be
// resolved, exactly as AccessService.ResolveScope reports it).
//
// This is the authorization boundary for the resources that have no
// row-level security: sla, incident, incident_task and problem (migration
// 0153). None of them is used by the customer portal, and none has a
// per-project concept a policy could filter on, so "internal or nothing" is
// enforced here, once, on the route, independent of which service
// implementation (Postgres or ServiceNow) backs the handler. The check itself
// is service.RequireInternalCaller, shared with the per-service guards.
func internalOnly(access service.AccessService, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := service.RequireInternalCaller(r.Context(), access, "this resource is only available to internal callers")
		if err != nil {
			var ue *apierror.UnauthorizedError
			var sue *apierror.ServiceUnavailableError
			var fe *apierror.ForbiddenError
			switch {
			case errors.As(err, &fe):
				apierror.WriteJSON(w, http.StatusForbidden, fe.Msg)
			case errors.As(err, &ue):
				apierror.WriteJSON(w, http.StatusUnauthorized, ue.Msg)
			case errors.As(err, &sue):
				apierror.WriteJSON(w, http.StatusServiceUnavailable, sue.Msg)
			default:
				log.Printf("internalOnly: resolve scope for %s: %v", r.Method, err)
				apierror.WriteJSON(w, http.StatusInternalServerError, "internal server error")
			}
			return
		}
		next(w, r)
	}
}

// serviceOnly is internalOnly for a route only a service may call: an
// internal service credential passes, a user token is refused with 403 even
// when the user is internal (the check requireService applies to the team
// schedule reminders).
func serviceOnly(access service.AccessService, next http.HandlerFunc) http.HandlerFunc {
	return internalOnly(access, func(w http.ResponseWriter, r *http.Request) {
		if auth.IdentityFromContext(r.Context()).UserEmail != "" {
			apierror.WriteJSON(w, http.StatusForbidden, "this resource is only available to a service credential, not a user")
			return
		}
		next(w, r)
	})
}
