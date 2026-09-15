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

// PatchCase calls PATCH /cases/{id} on the entity service to update a case.
// Unlike UpdateProject, this entity-service operation does NOT unconditionally
// require a forwarded end-user identity token — whether it does depends on
// entity-service's own DATA_SOURCE:
//   - On a Postgres data source, a state/severity/workState update (optionally
//     combined with resolutionCode/cause/closeNotes when state is closed or
//     solution_proposed) succeeds for a pure M2M caller — no token check on
//     this path. Every other field this request shape can carry (watchList,
//     assigneeEmail, type and its companions, parentId, relatedCaseId,
//     autocloseHoldUntil, subject, description, deploymentId,
//     deployedProductId, the fix-ETA group, acknowledge, workaroundProvided)
//     is ServiceNow-only and is rejected with 400 on this data source, not
//     proxied through to any token check.
//   - On a ServiceNow data source, every field this operation accepts —
//     including a bare state/severity/workState update — requires a forwarded
//     end-user identity token, so every call here is expected to 401 the same
//     way UpdateProject always does, with no field combination that succeeds.
//
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) PatchCase(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/cases/%s", url.PathEscape(id)), body)
}

// CreateCaseComment calls POST /cases/{id}/comments on the entity service.
// Unlike PatchCase, this entity-service operation requires a forwarded
// end-user identity token unconditionally, on both data sources (the comment's
// author is resolved from that token). This service is strictly M2M with no
// mechanism to carry one, so this call is expected to always receive a mapped
// 401 — kept for API-shape completeness, not because it currently succeeds.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/comments", url.PathEscape(caseID)), body)
}

// SearchOpportunities calls POST /opportunities/search on the entity service.
// ServiceNow data source only; a pure M2M call succeeds here, unlike UpdateProject —
// this operation does not require a forwarded end-user identity token. Response is
// returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchOpportunities(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/opportunities/search", body)
}

// GetOpportunity calls GET /opportunities/{id} on the entity service.
// ServiceNow data source only; a pure M2M call succeeds here (read-only, no
// forwarded identity required). Response is returned as raw JSON; typed response
// structs are deferred.
func (c *Client) GetOpportunity(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/opportunities/%s", url.PathEscape(id)), nil)
}

// SearchInvoices calls POST /invoices/search on the entity service.
// ServiceNow data source only; a pure M2M call succeeds here (read-only, no
// forwarded identity required). Response is returned as raw JSON; typed response
// structs are deferred.
func (c *Client) SearchInvoices(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/invoices/search", body)
}

// GetInvoice calls GET /invoices/{id} on the entity service.
// ServiceNow data source only; a pure M2M call succeeds here (read-only, no
// forwarded identity required). Response is returned as raw JSON; typed response
// structs are deferred.
func (c *Client) GetInvoice(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/invoices/%s", url.PathEscape(id)), nil)
}

// SearchProjectOpportunityLinks calls POST /project-opportunity-links/search on the
// entity service. ServiceNow data source only; a pure M2M call succeeds here
// (read-only, no forwarded identity required). There is no by-id fetch for this
// resource — the underlying ServiceNow data has no single-record endpoint. Response
// is returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchProjectOpportunityLinks(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/project-opportunity-links/search", body)
}

// SyncProductVulnerabilities calls POST /products/vulnerabilities/sync on the entity
// service. This is a full-replace sync: the caller must submit the complete current set
// of product-vulnerability records on every call, not an incremental delta — the
// downstream ServiceNow-backed operation deletes any existing record not present in the
// submitted set. Unlike UpdateProject, this entity-service operation accepts pure M2M
// calls with no forwarded end-user token, so this call is expected to succeed.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) SyncProductVulnerabilities(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/products/vulnerabilities/sync", body)
}
