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

// UpdateProject calls PATCH /projects/{id} on the entity service. This targets an
// external-data-source-only operation (used by the Account Closure Process
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
//     is external-data-source-only and is rejected with 400 on this data source, not
//     proxied through to any token check.
//   - On the external data source, every field this operation accepts —
//     including a bare state/severity/workState update — requires a forwarded
//     end-user identity token, so every call here is expected to 401 the same
//     way UpdateProject always does, with no field combination that succeeds.
//
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) PatchCase(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/cases/%s", url.PathEscape(id)), body)
}

// SearchCases calls POST /cases/search on the entity service, mirroring
// SearchAccounts's shape. A generic passthrough — callers build whatever
// filter/pagination shape they need (e.g. an exact-match filter on "number"
// to resolve a case number to this platform's own case UUID, never a
// backing-system record id, which the entity service's case model never exposes).
// Postgres-backed; a pure M2M call succeeds here, no forwarded identity
// required. Response is returned as raw JSON; typed response structs are
// deferred.
func (c *Client) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/cases/search", body)
}

// AddCaseTag calls POST /cases/{id}/tags on the entity service. Postgres-backed;
// unlike PatchCase's general field set, tagging supports an M2M caller
// supplying an actorEmail in the request body when no end-user identity token
// is forwarded, provided that email is on entity-service's configured
// M2M_TRUSTED_ACTOR_EMAILS allowlist (otherwise entity-service returns 403).
// This client method forwards the body verbatim; the caller is responsible
// for populating actorEmail. Response is returned as raw JSON; typed response
// structs are deferred.
func (c *Client) AddCaseTag(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/tags", url.PathEscape(caseID)), body)
}

// CreateCaseComment calls POST /cases/{id}/comments on the entity service.
// Mirrors AddCaseTag: on DATA_SOURCE=postgres, entity-service supports an
// M2M caller supplying an actorEmail in the request body when no end-user
// identity token is forwarded, provided that email is on entity-service's
// configured M2M_TRUSTED_ACTOR_EMAILS allowlist (otherwise entity-service
// returns 403). On the external data source, entity-service's path
// still requires a forwarded end-user identity token unconditionally and
// ignores actorEmail, so this service (strictly M2M, no mechanism to carry
// one) still gets a mapped 401 there. This client method forwards the body
// verbatim; the caller is responsible for populating actorEmail. Response is
// returned as raw JSON; typed response structs are deferred.
func (c *Client) CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/comments", url.PathEscape(caseID)), body)
}

// SearchOpportunities calls POST /opportunities/search on the entity service.
// M2M-safe on both data sources (Postgres reads sf_opportunity). Raw JSON passthrough.
func (c *Client) SearchOpportunities(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/opportunities/search", body)
}

// GetOpportunity calls GET /opportunities/{id} on the entity service.
// M2M-safe on both data sources (Postgres reads sf_opportunity). Raw JSON passthrough.
func (c *Client) GetOpportunity(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/opportunities/%s", url.PathEscape(id)), nil)
}

// SearchInvoices calls POST /invoices/search on the entity service.
// M2M-safe on both data sources (Postgres reads sf_invoice). Raw JSON passthrough.
func (c *Client) SearchInvoices(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/invoices/search", body)
}

// GetInvoice calls GET /invoices/{id} on the entity service.
// M2M-safe on both data sources (Postgres reads sf_invoice). Raw JSON passthrough.
func (c *Client) GetInvoice(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/invoices/%s", url.PathEscape(id)), nil)
}

// SearchProjectOpportunityLinks calls POST /project-opportunity-links/search on the
// entity service. M2M-safe on both data sources (Postgres reads sf_opportunity_link).
// Search only, no by-id fetch. Raw JSON passthrough.
func (c *Client) SearchProjectOpportunityLinks(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/project-opportunity-links/search", body)
}

// SyncProductVulnerabilities calls POST /products/vulnerabilities/sync on the entity
// service. This is a full-replace sync: the caller must submit the complete current set
// of product-vulnerability records on every call, not an incremental delta — the
// downstream externally backed operation deletes any existing record not present in the
// submitted set. Unlike UpdateProject, this entity-service operation accepts pure M2M
// calls with no forwarded end-user token, so this call is expected to succeed.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) SyncProductVulnerabilities(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/products/vulnerabilities/sync", body)
}

// CreateIncident calls POST /incidents on the entity service. The backing
// operation does not strictly require a forwarded end-user identity: when
// none is present it falls back to a separately configured machine
// credential for the backing data source, so this M2M-only call succeeds
// wherever that credential is configured and is answered 401 only where it
// is not. Unlike UpdateProject, a 401 here is therefore environment-dependent,
// not unconditional. Response is returned as raw JSON; typed response structs
// are deferred.
func (c *Client) CreateIncident(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/incidents", body)
}

// SearchIncidents calls POST /incidents/search on the entity service. Same
// machine-credential fallback as CreateIncident above: it succeeds over M2M
// where that credential is configured and 401s only where it is not.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *Client) SearchIncidents(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/incidents/search", body)
}

// CreateAlertIncidentMapping calls POST /alert-incident-mappings on the entity
// service. Unlike CreateIncident/SearchIncidents above, this targets a
// Postgres-only entity-service operation with no external-data-source dependency and no
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

// UpdateIncident calls PATCH /incidents/{id} on the entity service. Unlike
// PatchCase, this operation has no Postgres-data-source path at all: on
// DATA_SOURCE=postgres, entity-service's incidentService.UpdateIncident
// unconditionally returns a 503 (not supported on this data source yet, no
// field combination succeeds — several fields have no backing Postgres
// column, and others would need comment-table side effects not implemented
// there); on the external data source, it goes through the same M2M-fallback
// mechanism as CreateIncident/SearchIncidents/SearchITServices above (a
// separately-configured M2M credential for the external data source is used when no end-user
// identity token is forwarded, and only 401s if that fallback credential is
// itself unconfigured in the target environment). So this call is
// unconditionally backed by the external data source with no Postgres fallback path: whether
// it succeeds depends entirely on the target environment's data source and,
// on the external data source, its M2M credential configuration — not on which fields are
// sent, unlike PatchCase's field-dependent behavior. Response is returned as
// raw JSON; typed response structs are deferred.
func (c *Client) UpdateIncident(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/incidents/%s", url.PathEscape(id)), body)
}

// SearchITServices calls POST /services/search on the entity service. This
// targets an externally backed operation with the same M2M-fallback
// mechanism as CreateIncident/SearchIncidents above: when no end-user
// identity token is forwarded, it uses a separately-configured M2M
// credential for the external data source instead of erroring, and only
// 401s if that fallback
// credential is itself unconfigured in the target environment. This service
// carries no forwarded end-user identity by design (see this file's own
// CreateIncident doc comment), so whether this 401s depends on the target
// environment's M2M credential configuration, not on this service's M2M-only
// design per se. Response is returned as raw JSON; typed response structs
// are deferred.
func (c *Client) SearchITServices(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/services/search", body)
}

// GetCloudStatusMonitors calls GET /cloud-status/monitors on the entity
// service. Response is returned as raw JSON; typed response structs are
// deferred, as elsewhere here.
//
// The cloud is passed through as a query parameter rather than validated: the
// entity service owns the list of valid clouds and returns a 400 for anything
// else, and duplicating that list here would give two places to update when a
// cloud is added.
func (c *Client) GetCloudStatusMonitors(ctx context.Context, cloud string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/cloud-status/monitors?cloud="+url.QueryEscape(cloud), nil)
}

// GetCloudStatusIncidents calls GET /cloud-status/incidents on the entity
// service.
func (c *Client) GetCloudStatusIncidents(ctx context.Context, cloud string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/cloud-status/incidents?cloud="+url.QueryEscape(cloud), nil)
}

// GetCloudStatusAvailabilities calls GET /cloud-status/availabilities on the
// entity service -- the weighted uptime figures the dashboard prints beside
// each region.
func (c *Client) GetCloudStatusAvailabilities(ctx context.Context, cloud string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/cloud-status/availabilities?cloud="+url.QueryEscape(cloud), nil)
}

// GetCloudStatusIncidentDetail calls GET /cloud-status/incidents/{id} on the
// entity service -- one outage's public detail view.
//
// The id is forwarded as given. The entity service accepts both the dashed
// uuid and the 32-hex legacy id form, so a link created before the cutover
// still resolves after it; normalising here would put that rule in two
// places.
func (c *Client) GetCloudStatusIncidentDetail(ctx context.Context, id, cloud string) ([]byte, error) {
	return c.do(ctx, http.MethodGet,
		"/cloud-status/incidents/"+url.PathEscape(id)+"?cloud="+url.QueryEscape(cloud), nil)
}

// GetCloudStatusAvailabilityHistory calls GET
// /cloud-status/availability-history on the entity service -- the 90-day
// daily chart.
func (c *Client) GetCloudStatusAvailabilityHistory(ctx context.Context, cloud string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/cloud-status/availability-history?cloud="+url.QueryEscape(cloud), nil)
}
