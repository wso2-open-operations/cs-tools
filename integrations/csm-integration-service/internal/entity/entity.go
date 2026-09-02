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
	"net/http"
	"net/url"
)

// GetAccount calls GET /accounts/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) GetAccount(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/accounts/%s", url.PathEscape(id)), nil)
}

// SearchAccounts calls POST /accounts/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchAccounts(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/accounts/search", body)
}

// SearchAccountContacts calls POST /accounts/{id}/contacts/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/accounts/%s/contacts/search", url.PathEscape(accountID)), body)
}

// GetProject calls GET /projects/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) GetProject(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%s", url.PathEscape(id)), nil)
}

// SearchProjects calls POST /projects/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/projects/search", body)
}

// SearchProjectContacts calls POST /projects/{id}/contacts/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%s/contacts/search", url.PathEscape(projectID)), body)
}

// UpdateProject calls PATCH /projects/{id} on the entity service. This targets a
// ServiceNow-data-source-only operation (used by the Account Closure Process
// automation to write closure-state fields on a project) that requires a
// forwarded end-user identity token. This service is strictly M2M with no
// mechanism to carry one, so entity-service is expected to reject this call
// with 401 — kept for API-shape completeness and so ACP's caller-side wiring
// has somewhere real to point at, not because it currently succeeds. This
// method returns c.do()'s raw result unchanged; status mapping happens in the
// handler via mapUpstreamError, which may produce a different status (e.g. a
// generic 500) for a transport-level failure that never reaches entity-service
// at all. Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/projects/%s", url.PathEscape(id)), body)
}

// CreateIncident calls POST /incidents on the entity service. This targets a
// ServiceNow-backed operation that requires a forwarded end-user identity
// token. This service is strictly M2M with no mechanism to carry one, so
// entity-service is expected to reject this call with 401 — kept for
// API-shape completeness so a real caller has somewhere stable to point at,
// not because it currently succeeds. See UpdateProject's doc comment above
// for the same situation. Response is returned as raw JSON; typed response
// structs are deferred.
func (c *Client) CreateIncident(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/incidents", body)
}

// SearchIncidents calls POST /incidents/search on the entity service. This
// targets a ServiceNow-backed operation that requires a forwarded end-user
// identity token, same as CreateIncident above — this service cannot supply
// one, so entity-service is expected to reject this call with 401. Kept for
// API-shape completeness, not because it currently succeeds. Response is
// returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchIncidents(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/incidents/search", body)
}

// CreateAlertIncidentMapping calls POST /alert-incident-mappings on the entity
// service. Unlike CreateIncident/SearchIncidents above, this targets a
// Postgres-only entity-service operation with no ServiceNow dependency and no
// requirement for a forwarded end-user identity token — this service's M2M
// identity to entity-service is sufficient, so this call is expected to
// actually succeed today. Response is returned as raw JSON; typed response
// structs are deferred.
func (c *Client) CreateAlertIncidentMapping(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/alert-incident-mappings", body)
}

// LookupAlertIncidentMappings calls POST /alert-incident-mappings/lookup on
// the entity service. Same Postgres-only, M2M-friendly situation as
// CreateAlertIncidentMapping above — no forwarded end-user identity is
// required, so this call is expected to actually succeed today. Response is
// returned as raw JSON; typed response structs are deferred.
func (c *Client) LookupAlertIncidentMappings(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/alert-incident-mappings/lookup", body)
}
