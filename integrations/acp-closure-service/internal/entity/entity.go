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

// GetAccount calls GET /accounts/{id}. Response is returned as raw JSON;
// typed response structs are deferred to the caller.
func (c *Client) GetAccount(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/accounts/%s", url.PathEscape(id)), nil)
}

// GetProject calls GET /projects/{id}. Used to scope a run to a single
// project (e.g. TEST_PROJECT_ID) instead of the broad /projects/search
// sweep. Returns the same Project shape as each item in SearchProjects's
// response — csm-integration-service's openapi.yaml references the same
// Project schema for both. Response is returned as raw JSON; typed response
// structs are deferred to the caller.
func (c *Client) GetProject(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%s", url.PathEscape(id)), nil)
}

// SearchProjects calls POST /projects/search. Response is returned as raw
// JSON; typed response structs are deferred to the caller.
//
// closureStatus/endDateFrom/endDateTo/sortBy/sortOrder are undocumented in
// csm-integration-service's openapi.yaml (which only lists
// pagination/searchQuery) but are confirmed working via direct Postman
// testing against staging — the handler forwards the request body verbatim
// to entity-service, which does honor them. The response body includes
// closureState, endDateClosureState, and endDate even though those fields
// are similarly undocumented — also confirmed via Postman.
//
// If body sets an explicit empty-string searchQuery, expect a 400 — omit
// the field entirely instead (confirmed quirk, same as calling
// entity-service directly: an empty JSON object is accepted, an explicit
// empty searchQuery is not).
func (c *Client) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/projects/search", body)
}

// SearchAccountContacts calls POST /accounts/{id}/contacts/search. Response
// is returned as raw JSON; typed response structs are deferred to the caller.
// See SearchProjects's doc comment for the empty-searchQuery quirk.
func (c *Client) SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/accounts/%s/contacts/search", url.PathEscape(accountID)), body)
}

// SearchProjectContacts calls POST /projects/{id}/contacts/search. Response
// is returned as raw JSON; typed response structs are deferred to the caller.
// See SearchProjects's doc comment for the empty-searchQuery quirk.
func (c *Client) SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%s/contacts/search", url.PathEscape(projectID)), body)
}

// UpdateProject calls PATCH /projects/{id}. This is a
// ServiceNow-data-source-only operation. M2M-only auth (no forwarded
// x-user-id-token) is confirmed sufficient for this call — verified via a
// real write against the dedicated test project
// (e3e87599-1bc7-6650-182c-0dc5604bcb68), not inferred. Response is returned
// as raw JSON; typed response structs are deferred to the caller.
func (c *Client) UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/projects/%s", url.PathEscape(id)), body)
}

// SearchOpportunities calls POST /opportunities/search — Phase 2
// (invoice-based closure). Confirmed read-only, no forwarded end-user
// identity required, so this M2M-only client can call it directly (unlike
// e.g. UpdateProject on the entity-service side; csm-integration-service's
// own CLAUDE.md documents this distinction). Response is returned as raw
// JSON; typed response structs are deferred to the caller.
func (c *Client) SearchOpportunities(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/opportunities/search", body)
}

// GetOpportunity calls GET /opportunities/{id}. Response is returned as raw
// JSON; typed response structs are deferred to the caller.
func (c *Client) GetOpportunity(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/opportunities/%s", url.PathEscape(id)), nil)
}

// SearchInvoices calls POST /invoices/search. Response is returned as raw
// JSON; typed response structs are deferred to the caller.
func (c *Client) SearchInvoices(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/invoices/search", body)
}

// GetInvoice calls GET /invoices/{id}. Response is returned as raw JSON;
// typed response structs are deferred to the caller.
func (c *Client) GetInvoice(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/invoices/%s", url.PathEscape(id)), nil)
}

// SearchProjectOpportunityLinks calls POST /project-opportunity-links/search.
// No by-id fetch exists for this resource — a project can link to more than
// one opportunity, so the underlying data has no single-record shape.
// Response is returned as raw JSON; typed response structs are deferred to
// the caller.
func (c *Client) SearchProjectOpportunityLinks(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/project-opportunity-links/search", body)
}
