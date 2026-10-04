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
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
)

// withPortalRoles replaces GET /users/{id}'s roles field, for an internal
// (WSO2 staff) target only, with the same portal-role vocabulary GET
// /users/me reports (viewer, escalator, cs_engineer, admin, ...) rather than
// entity-service's own role vocabulary (admin, commenter, customer, ...).
// The two are unrelated: entity-service's vocabulary is what an external
// contact's project access is actually modeled on, but a portal user's own
// access is modeled on their Asgardeo role assignment instead, and showing
// the wrong one under "Platform roles" reads as though most of a staff
// member's real access were missing.
//
// GET /users/me computes this from the CALLER's own JWT "roles" claim, which
// only ever describes the caller -- there is no way to read another user's
// token from here. SCIM's own user search returns the same role assignment
// for ANY user by email, so this reaches the same portal-role vocabulary for
// the user being VIEWED via SCIM instead. SCIM's roles span every Asgardeo
// application the person holds a role in, not just this portal, so they are
// filtered to the CSM app's own prefix (scim.CSMAppRolePrefix) first --
// matching what the JWT's own "roles" claim already narrows to at
// token-issuance time.
//
// Best-effort like every other enrichment on this profile: any failure
// (decode, lookup, re-encode) is logged and the response returned unchanged.
func (h *UsersHandler) withPortalRoles(ctx context.Context, raw []byte, callerID string) []byte {
	if h.access == nil {
		return raw
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		slog.WarnContext(ctx, "withPortalRoles: decode user profile failed", "userID", callerID, "err", summarizeErr(err))
		return raw
	}

	var identity entityUserIdentity
	if rawEmail, ok := envelope["email"]; ok {
		_ = json.Unmarshal(rawEmail, &identity.Email)
	}
	if rawType, ok := envelope["userType"]; ok {
		_ = json.Unmarshal(rawType, &identity.UserType)
	}

	if identity.Email == "" || identity.UserType != "internal" {
		return raw
	}

	info, err := h.scim.SearchUser(ctx, identity.Email)
	if err != nil {
		slog.WarnContext(ctx, "withPortalRoles: scim SearchUser failed", "userID", callerID, "err", summarizeErr(err))
		return raw
	}
	if info == nil {
		slog.WarnContext(ctx, "withPortalRoles: scim SearchUser: no result", "userID", callerID)
		return raw
	}

	var csmRoles []string
	for _, role := range info.Roles {
		if strings.HasPrefix(role, scim.CSMAppRolePrefix) {
			csmRoles = append(csmRoles, role)
		}
	}

	encoded, err := json.Marshal(h.access.RolesFor(csmRoles))
	if err != nil {
		slog.WarnContext(ctx, "withPortalRoles: encode roles failed", "userID", callerID, "err", summarizeErr(err))
		return raw
	}
	envelope["roles"] = encoded

	out, err := json.Marshal(envelope)
	if err != nil {
		slog.WarnContext(ctx, "withPortalRoles: encode user profile failed", "userID", callerID, "err", summarizeErr(err))
		return raw
	}
	return out
}
