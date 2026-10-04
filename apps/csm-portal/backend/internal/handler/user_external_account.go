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
)

// wso2EmailDomain is WSO2's own corporate domain. The SCIM "external" org can
// never contain such an account -- it's reserved for WSO2 staff -- so a
// wso2.com email skips the lookup even when the backing system tags the row with a
// non-"internal" userType/role (e.g. a wso2.com contact recorded under a
// customer-facing role like snc_external for testing).
const wso2EmailDomain = "@wso2.com"

func isWso2Email(email string) bool {
	return strings.HasSuffix(strings.ToLower(email), wso2EmailDomain)
}

// internalUserTypeRoles are the role names entity-service's recompute_user_type
// trigger (migration 0011_users_add_user_type.sql) resolves to user_type =
// INTERNAL -- mirrors the identically-named list in entity-service's own
// user_service.go, kept in sync by hand the same way wso2EmailDomain is.
var internalUserTypeRoles = []string{"admin", "internal"}

// requestsInternalUserType reports whether granting roles at user creation
// would resolve the new user's user_type to INTERNAL. Checked here so POST
// /users can reject a non-wso2.com email with a friendly 400 before ever
// reaching entity-service, which enforces the same rule as the real,
// authoritative check (see that service's own CreateUser).
func requestsInternalUserType(roles []string) bool {
	for _, role := range roles {
		for _, internal := range internalUserTypeRoles {
			if strings.EqualFold(role, internal) {
				return true
			}
		}
	}
	return false
}

// externalUserTypeRoles mirrors the identically-named list in entity-service's
// own user_service.go -- the role names its recompute_user_type trigger
// resolves to user_type = EXTERNAL. Creating an EXTERNAL-type user is
// temporarily disabled; see requestsExternalUserType.
var externalUserTypeRoles = []string{"external", "partner", "customer", "partner_admin", "customer_admin"}

// requestsExternalUserType reports whether granting roles at user creation
// would resolve the new user's user_type to EXTERNAL. Checked here so POST
// /users can reject it with a friendly 400 before ever reaching entity-service,
// which enforces the same rule as the real, authoritative check.
func requestsExternalUserType(roles []string) bool {
	for _, role := range roles {
		for _, external := range externalUserTypeRoles {
			if strings.EqualFold(role, external) {
				return true
			}
		}
	}
	return false
}

// externalAccountStatus is the SCIM "external" org lock/existence status
// appended to GET /users/{id} for external contacts, mirroring the
// asgardeo-user-check service's {exists, locked} contract. Locked is null
// (not omitted) whenever it cannot be determined, matching that contract.
type externalAccountStatus struct {
	Exists bool  `json:"exists"`
	Locked *bool `json:"locked"`
}

// entityUserIdentity is the subset of a GET /users/{id} response this needs
// to decide whether -- and by which email -- to run the SCIM external lookup.
type entityUserIdentity struct {
	Email    string `json:"email"`
	UserType string `json:"userType"`
}

// withExternalAccountStatus appends externalAccount (SCIM exists/locked) to a
// GET /users/{id} response, for external contacts only -- WSO2 staff live in
// the SCIM "internal" org, which this lookup does not query. Best-effort like
// every other enrichment on this profile: any failure (decode, lookup,
// re-encode) is logged and the response returned unchanged, never failing
// the request.
func (h *UsersHandler) withExternalAccountStatus(ctx context.Context, raw []byte, callerID string) []byte {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		slog.WarnContext(ctx, "scim SearchExternalUser: decode user profile failed", "userID", callerID, "err", summarizeErr(err))
		return raw
	}

	var identity entityUserIdentity
	if rawEmail, ok := envelope["email"]; ok {
		_ = json.Unmarshal(rawEmail, &identity.Email)
	}
	if rawType, ok := envelope["userType"]; ok {
		_ = json.Unmarshal(rawType, &identity.UserType)
	}

	if identity.Email == "" || identity.UserType == "internal" || isWso2Email(identity.Email) {
		return raw
	}

	info, err := h.scim.SearchExternalUser(ctx, identity.Email)
	if err != nil {
		slog.WarnContext(ctx, "scim SearchExternalUser failed", "userID", callerID, "err", summarizeErr(err))
		return raw
	}
	if info == nil {
		slog.WarnContext(ctx, "scim SearchExternalUser: no result", "userID", callerID)
		return raw
	}

	encoded, err := json.Marshal(externalAccountStatus{Exists: info.Exists, Locked: info.Locked})
	if err != nil {
		slog.WarnContext(ctx, "scim SearchExternalUser: encode status failed", "userID", callerID, "err", summarizeErr(err))
		return raw
	}
	envelope["externalAccount"] = encoded

	out, err := json.Marshal(envelope)
	if err != nil {
		slog.WarnContext(ctx, "scim SearchExternalUser: encode user profile failed", "userID", callerID, "err", summarizeErr(err))
		return raw
	}
	return out
}
