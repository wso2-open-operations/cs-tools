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
	"errors"
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
	attrPhoneNumbers = "phoneNumbers"
	attrUserName     = "userName"
	attrSchema       = "urn:scim:wso2:schema"
	attrRoles        = "roles"

	mobilePhoneType = "mobile"

	accountStateLocked = "LOCKED"
)

// SearchUser fetches SCIM data for the given email and returns the extracted
// phone number, last password update time, and role assignment, mirroring
// searchUsers + processPhoneNumber + processLastPasswordUpdateTime in the
// Ballerina SCIM module (roles has no Ballerina-side precedent -- it backs the
// profile page's own role display, added later than that mirrored logic).
// Returns nil if no matching user is found in the SCIM service.
func (c *Client) SearchUser(ctx context.Context, email string) (*UserInfo, error) {
	if err := validateFilterEmail(email); err != nil {
		return nil, err
	}
	startIndex := 1
	var found *scimUser

	for {
		reqBody, err := json.Marshal(scimSearchRequest{
			Domain:     domainDefault,
			Attributes: []string{attrPhoneNumbers, attrUserName, attrSchema, attrRoles},
			Filter:     userNameEqFilter(email),
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
	if err := validateFilterEmail(email); err != nil {
		return nil, err
	}
	reqBody, err := json.Marshal(scimExternalSearchRequest{
		Attributes:   []string{attrUserName, attrSchema},
		Filter:       userNameEqFilter(email),
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

// emailFromDisplay strips a SCIM role member's "<domain>/" prefix (e.g.
// "DEFAULT/jane@wso2.com" -> "jane@wso2.com"). Falls back to the raw value
// when it carries no "/", rather than returning an empty string.
func emailFromDisplay(display string) string {
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

// userNameEqFilter builds a `userName eq "<value>"` SCIM filter. RFC 7644
// filter values are JSON string literals, so the value is JSON-encoded:
// quotes, backslashes and control characters are escaped and the value can
// never extend the filter expression.
func userNameEqFilter(value string) string {
	lit, _ := json.Marshal(value)
	return "userName eq " + string(lit)
}

// validateFilterEmail rejects a value that cannot be an e-mail address before
// it is used in a filter: empty, longer than 254 bytes, containing whitespace,
// a control character, a quote or a backslash, or not of the form local@domain.
func validateFilterEmail(email string) error {
	if email == "" || len(email) > 254 {
		return errInvalidFilterEmail
	}
	for _, r := range email {
		if r <= ' ' || r == 0x7f || r == '"' || r == '\\' {
			return errInvalidFilterEmail
		}
	}
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at == len(email)-1 || strings.Count(email, "@") != 1 {
		return errInvalidFilterEmail
	}
	return nil
}

// errInvalidFilterEmail is returned by SearchUser/SearchExternalUser for a
// value that is not a plausible e-mail address.
var errInvalidFilterEmail = errors.New("scim: not a valid e-mail address")
