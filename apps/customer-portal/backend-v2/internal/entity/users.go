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
	"net/http"
)

// GetMe calls GET /users/me.
//
// NOTE: this route is only registered by entity-service when it is deployed
// with DATA_SOURCE=servicenow (see cs-tools/entity-service/internal/server/routes.go).
// A Postgres-mode deployment will 404 on this call.
func (c *Client) GetMe(ctx context.Context) (GetUserMeResponse, error) {
	var out GetUserMeResponse
	err := c.getJSON(ctx, "/users/me", &out)
	return out, err
}

// RegisterInvitedMemberships calls POST /users/me/memberships/register: for
// each of the caller's memberships still in state INVITED, entity-service
// clears the contact's Salesforce lockout flag, sets the membership to
// REGISTERED, and refreshes the database. It returns 204 with no body, and
// is a no-op for a caller with nothing INVITED, which is every caller after
// their first sign-in.
//
// NOTE: entity-service only registers this route when it is deployed with
// CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED=true (and its membership
// ingest on). Any other deployment 404s, which is why the only caller treats
// a failure as nothing to act on.
func (c *Client) RegisterInvitedMemberships(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodPost, "/users/me/memberships/register", nil)
	return err
}

// PatchMe calls PATCH /users/me to update the caller's timezone.
//
// NOTE: this route is only registered by entity-service when it is deployed
// with DATA_SOURCE=servicenow (see cs-tools/entity-service/internal/server/routes.go).
// A Postgres-mode deployment will 404 on this call.
func (c *Client) PatchMe(ctx context.Context, req PatchUserMeRequest) (PatchUserMeResponse, error) {
	var out PatchUserMeResponse
	err := c.patchJSON(ctx, "/users/me", req, &out)
	return out, err
}
