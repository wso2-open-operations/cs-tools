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
)

const (
	// org is fixed to "internal" — the CSM portal is exclusively for WSO2 employees.
	org = "internal"

	domainDefault    = "DEFAULT"
	attrPhoneNumbers = "phoneNumbers"
	attrUserName     = "userName"
	attrSchema       = "urn:scim:wso2:schema"

	mobilePhoneType = "mobile"
)

// SearchUser fetches SCIM data for the given email and returns the extracted
// phone number and last password update time, mirroring searchUsers + processPhoneNumber
// + processLastPasswordUpdateTime in the Ballerina SCIM module.
// Returns nil if no matching user is found in the SCIM service.
func (c *Client) SearchUser(ctx context.Context, email string) (*UserInfo, error) {
	startIndex := 1
	var found *scimUser

	for {
		reqBody, err := json.Marshal(scimSearchRequest{
			Domain:     domainDefault,
			Attributes: []string{attrPhoneNumbers, attrUserName, attrSchema},
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
		PhoneNumber:            extractMobilePhone(*found),
		LastPasswordUpdateTime: extractLastPasswordUpdateTime(*found),
	}, nil
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
