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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	// org is fixed to "internal" — the CSM portal is exclusively for WSO2 employees.
	org = "internal"

	// orgExternal is the SCIM org that holds customer/partner contacts, used
	// to check whether an external contact's account exists and is locked.
	orgExternal = "external"

	domainDefault    = "DEFAULT"
	attrID           = "id"
	attrPhoneNumbers = "phoneNumbers"
	attrUserName     = "userName"
	attrSchema       = "urn:scim:wso2:schema"
	attrRoles        = "roles"

	mobilePhoneType = "mobile"

	accountStateLocked = "LOCKED"
)

// SearchUser fetches SCIM data for the given email and returns the extracted
// phone number, last password update time, role assignment, and the user's
// own Asgardeo ID (for a subsequent GetUserByID call), mirroring searchUsers
// + processPhoneNumber + processLastPasswordUpdateTime in the Ballerina SCIM
// module (roles and id have no Ballerina-side precedent -- id backs
// GetUserByID, added later than that mirrored logic). A search response only
// returns whichever attributes are explicitly requested -- id is not
// returned for free, the same convention the SCIM operations service's own
// getUserByUserName follows -- so it must be requested here even though
// nothing SearchUser itself computes needs it.
// Returns nil if no matching user is found in the SCIM service.
func (c *Client) SearchUser(ctx context.Context, email string) (*UserInfo, error) {
	startIndex := 1
	var found *scimUser

	for {
		reqBody, err := json.Marshal(scimSearchRequest{
			Domain:     domainDefault,
			Attributes: []string{attrID, attrPhoneNumbers, attrUserName, attrSchema, attrRoles},
			Filter:     fmt.Sprintf("userName eq %s", email),
			StartIndex: startIndex,
		})
		if err != nil {
			return nil, fmt.Errorf("scim: encode search request: %w", err)
		}

		raw, err := c.do(ctx, http.MethodPost, "/organizations/"+org+"/users/search", reqBody)
		if err != nil {
			return nil, err
		}

		var result scimSearchResponse
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("scim: decode search response: %w", err)
		}

		if len(result.Resources) > 0 && found == nil {
			u := result.Resources[0]
			found = &u
		}

		// Pagination mirrors the Ballerina while-loop condition:
		// moreUsersExist = (startIndex + itemsPerPage - 1) < totalResults
		if result.ItemsPerPage == 0 || (startIndex+result.ItemsPerPage-1) >= result.TotalResults {
			break
		}
		startIndex += result.ItemsPerPage
	}

	if found == nil {
		return nil, nil
	}

	return &UserInfo{
		ID:                     found.ID,
		PhoneNumber:            extractMobilePhone(*found),
		LastPasswordUpdateTime: extractLastPasswordUpdateTime(*found),
		Roles:                  []string(found.Roles),
	}, nil
}

// SearchExternalUser checks whether the given email exists in the SCIM
// "external" org and, if so, whether the account is locked, mirroring
// asgardeo-user-check's searchUser. A single itemsPerPage=1 lookup is enough
// to answer an existence check, so unlike SearchUser this does not paginate.
func (c *Client) SearchExternalUser(ctx context.Context, email string) (*ExternalUserInfo, error) {
	reqBody, err := json.Marshal(scimExternalSearchRequest{
		Attributes:   []string{attrUserName, attrSchema},
		Filter:       fmt.Sprintf("userName eq %q", email),
		ItemsPerPage: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("scim: encode external search request: %w", err)
	}

	raw, err := c.do(ctx, http.MethodPost, "/organizations/"+orgExternal+"/users/search", reqBody)
	if err != nil {
		return nil, err
	}

	var result scimSearchResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("scim: decode external search response: %w", err)
	}

	if result.TotalResults == 0 || len(result.Resources) == 0 {
		return &ExternalUserInfo{Exists: false}, nil
	}

	return &ExternalUserInfo{
		Exists: true,
		Locked: extractAccountLocked(result.Resources[0]),
	}, nil
}

// extractAccountLocked mirrors asgardeo-user-check's extractLocked: Asgardeo
// returns accountLocked as a quoted string ("true"/"false"), not a JSON
// boolean, so it must be parsed rather than decoded directly. Falls back to
// accountState ("LOCKED"/"UNLOCKED") when accountLocked is absent or
// unparseable, and returns nil if neither yields a determinate answer.
func extractAccountLocked(u scimUser) *bool {
	if u.SchemaScope == nil {
		return nil
	}
	if u.SchemaScope.AccountLocked != nil {
		if v, err := strconv.ParseBool(*u.SchemaScope.AccountLocked); err == nil {
			return &v
		}
	}
	if u.SchemaScope.AccountState != nil {
		v := strings.EqualFold(*u.SchemaScope.AccountState, accountStateLocked)
		return &v
	}
	return nil
}

// UpdateUserPhone updates the mobile phone number for the given user via a SCIM
// PATCH and returns the updated phone number extracted from the response,
// mirroring updateUser in the Ballerina SCIM module.
func (c *Client) UpdateUserPhone(ctx context.Context, userID, mobile string) (*string, error) {
	reqBody, err := json.Marshal(scimUpdateRequest{
		PhoneNumber: &scimPhonePayload{Mobile: mobile},
	})
	if err != nil {
		return nil, fmt.Errorf("scim: encode update request: %w", err)
	}

	path := "/organizations/internal/users/" + url.PathEscape(userID)
	raw, err := c.do(ctx, http.MethodPatch, path, reqBody)
	if err != nil {
		return nil, err
	}

	var updatedUser scimUser
	if err := json.Unmarshal(raw, &updatedUser); err != nil {
		return nil, fmt.Errorf("scim: decode update response: %w", err)
	}

	return extractMobilePhone(updatedUser), nil
}

// GetUserByID fetches the full SCIM user record for the given Asgardeo user
// ID (see UserInfo.ID, from a prior SearchUser call -- there is no lookup by
// email alone) and returns both the role assignments and the group
// memberships from it, each carrying both the resource's own Asgardeo ID and
// its display name.
//
// This is the canonical resource read, used instead of trusting the "roles"
// field already present on a /Users/.search result (scimUser.Roles /
// SearchUser's own UserInfo.Roles): a search response only carries whichever
// attributes the caller requested, while a direct GET of /Users/{id} always
// returns the complete representation. The practical reason to prefer it
// here specifically: matching a held role against ASGARDEO_ROLE_IDS needs
// each role's ID, not just its display name (two distinct roles can share a
// name), and getting a reliable ID for every entry is worth the extra round
// trip. Group membership (for ASGARDEO_ROLE_GROUP_NAMES, see
// handler.AccessGuard.WithGroupNames) rides along on this same response at
// no further cost -- there is no separate "get a user's groups" call.
func (c *Client) GetUserByID(ctx context.Context, userID string) (UserAssignments, error) {
	path := "/organizations/" + org + "/users/" + url.PathEscape(userID)
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return UserAssignments{}, err
	}

	var user scimUserByID
	if err := json.Unmarshal(raw, &user); err != nil {
		return UserAssignments{}, fmt.Errorf("scim: decode user response: %w", err)
	}
	roles := make([]RoleAssignment, 0, len(user.Roles))
	roles = append(roles, user.Roles...)
	groups := make([]GroupAssignment, 0, len(user.Groups))
	groups = append(groups, user.Groups...)
	return UserAssignments{Roles: roles, Groups: groups}, nil
}

// GetRole fetches the role with the given Asgardeo role ID and returns its
// member users. roleID is deployment configuration set once, out of band
// (e.g. TIMECARD_APPROVER_ASGARDEO_ROLE_ID) -- not looked up by name on every
// call, since the SCIM operations service's role endpoint is a plain
// get-by-id, not a search.
func (c *Client) GetRole(ctx context.Context, roleID string) ([]RoleMember, error) {
	path := "/organizations/" + org + "/roles/" + url.PathEscape(roleID)
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var role scimRole
	if err := json.Unmarshal(raw, &role); err != nil {
		return nil, fmt.Errorf("scim: decode role response: %w", err)
	}

	members := make([]RoleMember, 0, len(role.Users))
	for _, u := range role.Users {
		members = append(members, RoleMember{ID: u.Value, Email: emailFromDisplay(u.Display)})
	}
	return members, nil
}

// AddRoleMembers grants the given role to one or more users, by email, via
// the SCIM operations service's POST /organizations/internal/roles/{id}/users
// (see digiops-infra's scim-operations-service, modules/scim/scim.bal's
// addRoleMembers) -- the service resolves each email to its own user ID on
// the identity provider and performs the real role-membership update, so
// this client never handles user IDs directly. roleID is deployment
// configuration, resolved once at startup (see
// directory.ParseRoleIDs/handler.ResolveGrantableRoles), the same "already
// know the ID, never looked up by name" posture GetRole documents.
//
// An email the SCIM service couldn't resolve to a real user comes back in
// the response's own failedUsers list rather than as a transport error --
// surfaced here as an error naming the failed email(s), so a caller
// (CreateUser) can log a role grant that silently didn't take.
func (c *Client) AddRoleMembers(ctx context.Context, roleID string, emails []string) error {
	reqBody, err := json.Marshal(scimAddRoleMembersRequest{Emails: emails})
	if err != nil {
		return fmt.Errorf("scim: encode add-role-members request: %w", err)
	}

	path := "/organizations/" + org + "/roles/" + url.PathEscape(roleID) + "/users"
	raw, err := c.do(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return err
	}

	var result scimAddRoleMembersResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("scim: decode add-role-members response: %w", err)
	}
	if len(result.FailedUsers) > 0 {
		return fmt.Errorf("scim: role %s did not accept these emails: %s", roleID, strings.Join(result.FailedUsers, ", "))
	}
	return nil
}

// emailFromDisplay strips a SCIM role member's "<domain>/" prefix (e.g.
// "DEFAULT/<email>" -> "<email>").
func emailFromDisplay(display string) string {
	return stripSCIMDomainPrefix(display)
}

// stripSCIMDomainPrefix strips a SCIM display value's leading "<domain>/"
// prefix. Shared by emailFromDisplay (role members) and scimGroupAssignments
// (group names, in types.go): confirmed live that a group's own SCIM
// "display" comes back the identical "<domain>/<name>" shape role members'
// emails do (e.g. "DEFAULT/<group name>", not the bare group name
// ASGARDEO_ROLE_GROUP_NAMES is configured with) -- without stripping it,
// every group-name match silently failed. Falls back to the raw value when
// it carries no "/", rather than returning an empty string.
func stripSCIMDomainPrefix(display string) string {
	if idx := strings.Index(display, "/"); idx >= 0 && idx+1 < len(display) {
		return display[idx+1:]
	}
	return display
}

// extractMobilePhone returns the first phone number of type "mobile", or nil.
// Mirrors processPhoneNumber in the Ballerina SCIM utils.
func extractMobilePhone(u scimUser) *string {
	for _, p := range u.PhoneNumbers {
		if p.Type == mobilePhoneType {
			v := p.Value
			return &v
		}
	}
	return nil
}

// extractLastPasswordUpdateTime returns the lastPasswordUpdateTime from the
// WSO2 SCIM extension schema, or nil. Mirrors processLastPasswordUpdateTime.
func extractLastPasswordUpdateTime(u scimUser) *string {
	if u.SchemaScope != nil {
		return u.SchemaScope.LastPasswordUpdateTime
	}
	return nil
}
