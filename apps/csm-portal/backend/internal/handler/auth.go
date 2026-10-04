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

package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// requireViewerAccess is the first call in every SupportPortalLite handler: it
// replaces the plain UserInfoFromContext nil-check every other handler in this
// package starts with, additionally enforcing SupportPortalLite's blanket
// PermViewerAccess audience gate (mirrors Ballerina authJWT.imposeGlobalRules,
// which ran before every SupportPortalLite request, and previously
// SPL_ALLOWED_GROUPS's raw-Asgardeo-groups check before the SPL roles
// migration -- see PermViewerAccess's own doc comment). Returns the
// authenticated user and true on success; on failure it has already written
// the HTTP response (401 if unauthenticated, 403 if authenticated but
// holding no role granting PermViewerAccess) and the caller must return
// immediately.
func requireViewerAccess(w http.ResponseWriter, r *http.Request, guard *AccessGuard) (*middleware.UserInfo, bool) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, false
	}
	if !guard.Permits(PermViewerAccess, user.Roles) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return nil, false
	}
	return user, true
}

// requireViewerWriteAccess is requireViewerAccess for a SupportPortalLite route
// that changes state (every non-GET /customer-health/* route except the
// POST-as-query summary, and POST /scan-user): on top of the PermViewerAccess
// audience gate it requires PermWrite, the floor for a state change
// everywhere else in this backend. Same contract as requireViewerAccess: on
// false the response (401/403) has already been written.
func requireViewerWriteAccess(w http.ResponseWriter, r *http.Request, guard *AccessGuard) (*middleware.UserInfo, bool) {
	user, ok := requireViewerAccess(w, r, guard)
	if !ok {
		return nil, false
	}
	if !requireViewerPermission(w, user, guard, PermWrite) {
		return nil, false
	}
	return user, true
}

// requireViewerPermission performs one of SupportPortalLite's additional,
// narrower permission checks (PermEscalate, PermDownloadAttachment,
// PermUsageMetricsViewer) layered on top of the blanket PermViewerAccess gate
// requireViewerAccess already enforced. Call this after requireViewerAccess, only
// for the handful of endpoints Ballerina's operations.bal gated a second
// time (formerly addEscalationGroups/downloadAttachmentGroups/
// usageMetricsGroups). Returns false (and has already written a 403) when
// user holds no role granting perm.
func requireViewerPermission(w http.ResponseWriter, user *middleware.UserInfo, guard *AccessGuard, perm Permission) bool {
	if !guard.Permits(perm, user.Roles) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return false
	}
	return true
}
