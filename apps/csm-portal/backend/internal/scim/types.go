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

// ---- upstream (SCIM wire) types ----
// These mirror the record types in modules/scim/types.bal.

type scimSearchRequest struct {
	Domain     string   `json:"domain"`
	Attributes []string `json:"attributes"`
	Filter     string   `json:"filter"`
	StartIndex int      `json:"startIndex"`
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
}

type scimPhone struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// scimSchema holds the WSO2-specific SCIM extension fields.
type scimSchema struct {
	LastPasswordUpdateTime *string `json:"lastPasswordUpdateTime,omitempty"`
}

type scimUpdateRequest struct {
	PhoneNumber *scimPhonePayload `json:"phoneNumber,omitempty"`
}

type scimPhonePayload struct {
	Mobile string `json:"mobile"`
}

// ---- public types ----

// UserInfo holds the SCIM-derived fields for a user, extracted from the raw SCIM response.
type UserInfo struct {
	PhoneNumber            *string
	LastPasswordUpdateTime *string
}
