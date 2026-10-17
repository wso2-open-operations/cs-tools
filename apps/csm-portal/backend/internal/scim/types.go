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
	Value   string `json:"value"`
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

// scimUserByID mirrors the SCIM User record returned by GET
// /organizations/internal/users/{id} -- the canonical resource, which always
// carries the complete "roles" and "groups" arrays, unlike a /Users/.search
// response (scimUser above), which only carries whichever attributes the
// caller asked for. Only the fields this client actually needs from it are
// modeled.
type scimUserByID struct {
	Roles  scimRoleAssignments  `json:"roles,omitempty"`
	Groups scimGroupAssignments `json:"groups,omitempty"`
}

// scimRoleAssignments decodes the SCIM "roles" attribute the same shape
// scimRoles does (every observed variant: bare string, array of strings,
// bare {value,display} object, array of {value,display} objects -- see
// scimRoles' own doc comment for why all four are accepted), but keeps each
// entry's Value (the role's own unique Asgardeo ID) alongside its Display,
// instead of collapsing to Display alone: a caller matching a held role
// against ASGARDEO_ROLE_IDS needs the ID, since two distinct roles can share
// a display name. A shape with no id available (a bare string, or an object
// with no "value") decodes with an empty ID -- RoleAssignment.ID is not
// guaranteed non-empty.
type scimRoleAssignments []RoleAssignment

func (r *scimRoleAssignments) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if s == "" {
			*r = nil
		} else {
			*r = []RoleAssignment{{Name: s}}
		}
		return nil
	}

	var oneEntry scimRoleEntry
	if err := json.Unmarshal(b, &oneEntry); err == nil {
		*r = []RoleAssignment{{ID: oneEntry.Value, Name: oneEntry.Display}}
		return nil
	}

	var strs []string
	if err := json.Unmarshal(b, &strs); err == nil {
		values := make([]RoleAssignment, len(strs))
		for i, s := range strs {
			values[i] = RoleAssignment{Name: s}
		}
		*r = values
		return nil
	}

	var entries []scimRoleEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return fmt.Errorf("roles must be a string, an object, or an array of either: %w", err)
	}
	values := make([]RoleAssignment, len(entries))
	for i, e := range entries {
		values[i] = RoleAssignment{ID: e.Value, Name: e.Display}
	}
	*r = values
	return nil
}

// scimGroupAssignments decodes the SCIM "groups" attribute -- the core SCIM
// schema's own multi-valued complex attribute, observed (and accepted here)
// in the same shapes scimRoleAssignments already handles for "roles" (see
// that type's own doc comment for why every variant is accepted rather than
// assuming a single one): a bare string, an array of strings, a bare
// {value,display} object, or an array of {value,display} objects. Value
// (the group's own unique Asgardeo ID) is still kept on GroupAssignment for
// completeness, but nothing in this backend currently matches against it.
//
// Name is stripped of its leading "<domain>/" prefix via
// stripSCIMDomainPrefix (scim.go) -- confirmed live against a real
// deployment that a group's own SCIM display comes back "DEFAULT/<group
// name>", not the bare group name ASGARDEO_ROLE_GROUP_NAMES is configured
// with, the identical shape emailFromDisplay already strips for role
// members' emails. Applied in
// every decoded shape below (including a bare string/array of strings,
// since groups can arrive that way too) so GroupAssignment.Name is always
// already the clean, directly-comparable name -- callers never need to know
// this prefix exists at all.
type scimGroupAssignments []GroupAssignment

func (r *scimGroupAssignments) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if s == "" {
			*r = nil
		} else {
			*r = []GroupAssignment{{Name: stripSCIMDomainPrefix(s)}}
		}
		return nil
	}

	var oneEntry scimRoleEntry
	if err := json.Unmarshal(b, &oneEntry); err == nil {
		*r = []GroupAssignment{{ID: oneEntry.Value, Name: stripSCIMDomainPrefix(oneEntry.Display)}}
		return nil
	}

	var strs []string
	if err := json.Unmarshal(b, &strs); err == nil {
		values := make([]GroupAssignment, len(strs))
		for i, s := range strs {
			values[i] = GroupAssignment{Name: stripSCIMDomainPrefix(s)}
		}
		*r = values
		return nil
	}

	var entries []scimRoleEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return fmt.Errorf("groups must be a string, an object, or an array of either: %w", err)
	}
	values := make([]GroupAssignment, len(entries))
	for i, e := range entries {
		values[i] = GroupAssignment{ID: e.Value, Name: stripSCIMDomainPrefix(e.Display)}
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
	// ID is the user's own Asgardeo UUID -- needed to then call
	// GetUserByID, since /Users/.search's own "roles" attribute does not
	// reliably carry each role's ID (see GetUserByID's own doc comment).
	// Empty if the search response carried none (not observed in practice).
	ID                     string
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

// RoleAssignment is one role a user holds, as returned by GetUserByID. ID is
// the role's own Asgardeo UUID -- the thing to match against
// ASGARDEO_ROLE_IDS, since Name (the role's display name) can be shared by
// more than one distinct role.
type RoleAssignment struct {
	ID   string
	Name string
}

// GroupAssignment is one Asgardeo group a user belongs to, as returned by
// GetUserByID. Name is the group's display name -- what
// ASGARDEO_ROLE_GROUP_NAMES configures per portal role (a deployment choice:
// a group is named directly, the same way a real role is named for
// AUTH_<ROLE>_ROLES, rather than requiring its opaque id to be looked up).
// ID is kept alongside it, mirroring RoleAssignment's own ID/Name split, but
// is not currently matched against anything.
type GroupAssignment struct {
	ID   string
	Name string
}

// UserAssignments is GetUserByID's full result: both the role assignments it
// always returned, and the group memberships a deployment can also grant a
// portal role from (see handler.AccessGuard.WithGroupIDs). Both come off the
// same canonical GET /Users/{id} response, so reading group membership costs
// no extra round trip beyond the one GetUserByID already makes.
type UserAssignments struct {
	Roles  []RoleAssignment
	Groups []GroupAssignment
}

// ExternalUserInfo holds the SCIM "external" org existence/lock status for a
// user, mirroring the asgardeo-user-check service's {exists, locked} contract.
// Locked is nil when the account doesn't exist or its lock state can't be
// determined from the extension schema.
type ExternalUserInfo struct {
	Exists bool
	Locked *bool
}
