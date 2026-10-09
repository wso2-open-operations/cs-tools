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

package scim

import (
	"encoding/json"
	"fmt"
)

// ---- upstream (SCIM wire) types ----
// These mirror the upstream service's SCIM record types.

type scimSearchRequest struct {
	Domain     string   `json:"domain"`
	Attributes []string `json:"attributes"`
	Filter     string   `json:"filter"`
	StartIndex int      `json:"startIndex"`
}

// scimExternalSearchRequest is the request body for the "external" org search,
// which has no domain concept and needs no pagination: itemsPerPage=1 is
// enough to answer an existence check. Mirrors asgardeo-user-check's
// searchRequest.
type scimExternalSearchRequest struct {
	Attributes   []string `json:"attributes"`
	Filter       string   `json:"filter"`
	ItemsPerPage int      `json:"itemsPerPage"`
}

type scimSearchResponse struct {
	TotalResults int        `json:"totalResults"`
	StartIndex   int        `json:"startIndex"`
	ItemsPerPage int        `json:"itemsPerPage"`
	Resources    []scimUser `json:"Resources"`
}

// scimUser mirrors the SCIM User record. The WSO2 schema field uses the
// literal key "urn:scim:wso2:schema" which Go handles with a JSON struct tag.
type scimUser struct {
	ID           string      `json:"id"`
	PhoneNumbers []scimPhone `json:"phoneNumbers,omitempty"`
	SchemaScope  *scimSchema `json:"urn:scim:wso2:schema,omitempty"`
	Roles        scimRoles   `json:"roles,omitempty"`
}

// scimRoleEntry is one element of the SCIM "roles" attribute in its full
// complex-object shape, as actually observed against a real SCIM service:
//
//	{"value": "<opaque role id>", "display": "<role name>",
//	 "audienceType": "application", "audienceDisplay": "...", ...}
//
// "value" is the role resource's own opaque id (a UUID) -- NOT the role
// name -- confirmed live: an earlier attempt at matching a role's
// application by a hardcoded prefix against Value silently matched nothing,
// since a role id is a UUID, never a readable name. "display" is the actual
// role name, in whatever convention this deployment's Asgardeo org actually
// uses -- this package makes no assumption about its shape and leaves it to
// the caller (AccessGuard.RolesFor's own AUTH_<ROLE>_ROLES configuration) to
// recognize.
type scimRoleEntry struct {
	Display string `json:"display"`
}

// scimRoles decodes the SCIM "roles" attribute. It was initially assumed to
// follow the same "bare when one, array when several" convention Asgardeo's
// JWT "roles" claim uses (see middleware's own stringList), but a real SCIM
// service returns an array of {value, display, ...} objects instead --
// confirmed live ("json: cannot unmarshal object into Go value of type
// string" against the naive string-array assumption). Every shape below is
// accepted, since Asgardeo's own singular/plural convention elsewhere means a
// bare single value (string or object) isn't safe to rule out either:
//   - a bare string:                 "roles": "app-csm-example-role"
//   - an array of strings:           "roles": ["app-csm-example-role", ...]
//   - a bare {display} object:       "roles": {"display": "app-csm-example-role", ...}
//   - an array of {display} objects: "roles": [{"display": "app-csm-example-role", ...}, ...]
type scimRoles []string

func (r *scimRoles) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if s == "" {
			*r = nil
		} else {
			*r = []string{s}
		}
		return nil
	}

	var oneEntry scimRoleEntry
	if err := json.Unmarshal(b, &oneEntry); err == nil {
		*r = []string{oneEntry.Display}
		return nil
	}

	var strs []string
	if err := json.Unmarshal(b, &strs); err == nil {
		*r = strs
		return nil
	}

	var entries []scimRoleEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return fmt.Errorf("roles must be a string, an object, or an array of either: %w", err)
	}
	values := make([]string, len(entries))
	for i, e := range entries {
		values[i] = e.Display
	}
	*r = values
	return nil
}

type scimPhone struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// scimSchema holds the WSO2-specific SCIM extension fields.
type scimSchema struct {
	LastPasswordUpdateTime *string `json:"lastPasswordUpdateTime,omitempty"`
	// AccountLocked and AccountState are only populated on "external" org
	// lookups. Asgardeo returns accountLocked as a quoted string ("true"/
	// "false"), not a JSON boolean, hence the string type here too.
	AccountLocked *string `json:"accountLocked,omitempty"`
	AccountState  *string `json:"accountState,omitempty"`
}

type scimUpdateRequest struct {
	PhoneNumber *scimPhonePayload `json:"phoneNumber,omitempty"`
}

// scimRole mirrors the SCIM Role record returned by GET
// /organizations/internal/roles/{id} (Asgardeo Roles Management API v2).
type scimRole struct {
	ID          string         `json:"id"`
	DisplayName string         `json:"displayName"`
	Users       []scimRoleUser `json:"users"`
}

// scimRoleUser is one element of a role's "users" array. Display is observed
// as "<domain>/<email>" (e.g. "DEFAULT/jane@wso2.com") for a provisioned
// user -- see RoleMember.Email's own doc comment.
type scimRoleUser struct {
	Display string `json:"display"`
	Value   string `json:"value"`
}

type scimPhonePayload struct {
	Mobile string `json:"mobile"`
}

// scimAddRoleMembersRequest is the request body for PATCH
// /organizations/internal/roles/{id} (the SCIM operations service's own
// add-role-members endpoint, not a direct call to the identity provider) --
// emails only; the SCIM operations service resolves each to its own user ID
// and performs the actual role-membership update.
type scimAddRoleMembersRequest struct {
	Emails []string `json:"emails"`
}

// scimAddRoleMembersResponse mirrors the SCIM operations service's
// {addedUsers, failedUsers, addedGroups, failedGroups} response. This client
// only ever sends emails, never groups, so AddedGroups/FailedGroups are read
// but unused here.
type scimAddRoleMembersResponse struct {
	AddedUsers   []string `json:"addedUsers"`
	FailedUsers  []string `json:"failedUsers"`
	AddedGroups  []string `json:"addedGroups"`
	FailedGroups []string `json:"failedGroups"`
}

// ---- public types ----

// UserInfo holds the SCIM-derived fields for a user, extracted from the raw SCIM response.
type UserInfo struct {
	PhoneNumber            *string
	LastPasswordUpdateTime *string
	// Roles is the user's full Asgardeo role assignment, spanning every
	// application they hold a role in -- not just the CSM portal, and
	// deliberately left unfiltered: a caller wanting just this portal's roles
	// (e.g. handler.UsersHandler.withPortalRoles, to run through
	// AccessGuard.RolesFor for a user other than the caller) should hand the
	// whole list to RolesFor rather than pre-filtering it by a guessed naming
	// convention -- RolesFor already only matches what AUTH_<ROLE>_ROLES
	// actually configures, so a role belonging to another application simply
	// never matches anything. An earlier version of this package exported a
	// CSMAppRolePrefix constant for exactly that pre-filtering, on a guessed
	// naming convention -- removed after it was confirmed live not to match
	// this deployment's real one, silently dropping every role and never
	// resolving anyone's CSM Platform roles regardless of what they actually
	// held in Asgardeo.
	Roles []string
}

// RoleMember is one user holding a role, as returned by GetRole.
type RoleMember struct {
	// ID is the user's Asgardeo UUID.
	ID string
	// Email is extracted from the SCIM "display" field (observed as
	// "<domain>/<email>", e.g. "DEFAULT/jane@wso2.com") with the domain
	// prefix stripped. Falls back to the raw display value if it carries no
	// "/" (defensive -- not observed in practice).
	Email string
}

// ExternalUserInfo holds the SCIM "external" org existence/lock status for a
// user, mirroring the asgardeo-user-check service's {exists, locked} contract.
// Locked is nil when the account doesn't exist or its lock state can't be
// determined from the extension schema.
type ExternalUserInfo struct {
	Exists bool
	Locked *bool
}
