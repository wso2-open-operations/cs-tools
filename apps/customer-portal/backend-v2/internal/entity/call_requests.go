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

package entity

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// toDashedID converts an identifier (either a dashed UUID or a 32-hex sysid)
// to a canonical lowercase 8-4-4-4-12 dashed UUID string expected by entity-service.
func toDashedID(id string) string {
	clean := strings.ToLower(strings.ReplaceAll(id, "-", ""))
	if len(clean) == 32 {
		return clean[0:8] + "-" + clean[8:12] + "-" + clean[12:16] + "-" + clean[16:20] + "-" + clean[20:32]
	}
	return strings.ToLower(id)
}

// CreateCallRequest calls POST /call-requests.
//
// NOTE: entity-service only supports call requests on its ServiceNow data
// source — a Postgres-mode deployment 404s on every route in this file.
func (c *Client) CreateCallRequest(ctx context.Context, req CreateCallRequestRequest) (CreateCallRequestResponse, error) {
	req.CaseID = toDashedID(req.CaseID)
	var out CreateCallRequestResponse
	err := c.postJSON(ctx, "/call-requests", req, &out)
	return out, err
}

// SearchCallRequests calls POST /call-requests/search.
func (c *Client) SearchCallRequests(ctx context.Context, req SearchCallRequestsRequest) (SearchCallRequestsResponse, error) {
	req.CaseID = toDashedID(req.CaseID)
	var out SearchCallRequestsResponse
	err := c.postJSON(ctx, "/call-requests/search", req, &out)
	return out, err
}

// UpdateCallRequest calls PATCH /call-requests/{id}.
func (c *Client) UpdateCallRequest(ctx context.Context, id string, req UpdateCallRequestRequest) (UpdateCallRequestResponse, error) {
	id = toDashedID(id)
	req.ID = id // never serialized (json:"-"); set for consistency with the struct's doc comment
	var out UpdateCallRequestResponse
	err := c.patchJSON(ctx, fmt.Sprintf("/call-requests/%s", url.PathEscape(id)), req, &out)
	return out, err
}
