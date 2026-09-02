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

package csmclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DedupTag returns the exact, stable tag internal/handler.MapToIncident
// embeds in every CreateIncidentRequest.Subject it builds, keyed off the
// buffered alert row's own human-readable alert number (this service's own
// Postgres sequence — see internal/store.Store.Enqueue's doc comment and
// migrations/0002_add_alert_number.up.sql). The row's internal UUID primary
// key (internal/idgen) is unaffected by this and remains what every
// Store method keys its UPDATE/WHERE off of — this tag is purely the
// externally-facing identifier.
//
// Format: "[alert:<alert-number>]", e.g. "[alert:ALT0000123]". This exact
// string is what internal/worker's pre-retry dedup check
// (SearchIncidentByTag) later searches for via SearchIncidentsFilters.SearchQuery
// to find an incident a previous, lost-response attempt may already have
// created. Changing this format is a breaking change for any row already
// buffered with the old tag baked into its persisted payload — don't change
// it without a migration plan for in-flight rows.
func DedupTag(alertNumber string) string {
	return "[alert:" + alertNumber + "]"
}

// SearchIncidentsFilters is the filter subset of entity-service's own
// SearchIncidentsFilters (internal/domain/entity.go) this service actually
// sends. The real upstream type carries more optional fields (Priorities,
// ParentIDs, a fuller Filters array) — this service only ever needs
// free-text SearchQuery for the pre-retry dedup check, plus Number and a
// state Filters entry for the incident-grouping open-state confirmation
// (see SearchOpenIncidentByNumber), so the rest are left zero-valued/omitted
// rather than modeled here.
type SearchIncidentsFilters struct {
	SearchQuery string `json:"searchQuery"`
	// Number filters to the incident whose human-readable number (e.g.
	// "INC0010001") exactly matches. Only set by SearchOpenIncidentByNumber.
	Number *string `json:"number,omitempty"`
	// Filters is entity-service's generic field/op/values filter array. This
	// service only ever sends a single "state" (op "in") entry — see
	// openIncidentStates and SearchOpenIncidentByNumber.
	Filters []IncidentFieldFilter `json:"filters,omitempty"`
}

// IncidentFieldFilter is a single predicate in entity-service's generic
// incident-search filter array: "field op values". Mirrors entity-service's
// own IncidentFieldFilter (internal/domain/entity.go).
type IncidentFieldFilter struct {
	Field  string   `json:"field"`
	Op     string   `json:"op"`
	Values []string `json:"values,omitempty"`
}

// openIncidentStates are entity-service's domain.IncidentState values (see
// entity-service/internal/domain/entity.go — a read-only reference this
// service does not import, being a separate Go module, so these literals
// are copied and must be kept in sync by hand) that represent an incident
// still being worked: everything except RESOLVED, CLOSED, and CANCELLED.
var openIncidentStates = []string{"NEW", "IN_PROGRESS", "ON_HOLD"}

// IncidentSort mirrors entity-service's IncidentSort. Left zero-valued by
// SearchIncidentByTag — sort order doesn't matter when Pagination.Limit is 1
// and any match at all is treated the same way.
type IncidentSort struct {
	Field string `json:"field,omitempty"`
	Order string `json:"order,omitempty"`
}

// Pagination mirrors entity-service's Pagination.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// SearchIncidentsRequest is the request body for csm-integration-service's
// POST /incidents/search — a thin proxy of entity-service's own
// SearchIncidentsRequest, proxied as-is (field names/JSON tags copied
// verbatim), matching the same convention CreateIncidentRequest already
// follows in this package.
type SearchIncidentsRequest struct {
	Filters    SearchIncidentsFilters `json:"filters"`
	SortBy     IncidentSort           `json:"sortBy"`
	Pagination Pagination             `json:"pagination"`
}

// searchIncidentView is the subset of entity-service's SearchIncidentView
// this service actually reads out of a search hit — enough to record
// against the buffered alert row exactly like a fresh CreateIncident result
// would be (see CreateIncidentResult).
type searchIncidentView struct {
	ID     *string `json:"id"`
	Number *string `json:"number"`
}

// searchIncidentsResponse is the response body for POST /incidents/search,
// decoded tolerantly (unknown fields ignored), matching
// createIncidentResponse's convention in incidents.go.
type searchIncidentsResponse struct {
	Incidents []searchIncidentView `json:"incidents"`
	Total     int                  `json:"total"`
	Offset    int                  `json:"offset"`
	Limit     int                  `json:"limit"`
}

// SearchIncidentByTag calls POST /incidents/search on csm-integration-service
// with searchQuery=tag and Pagination{Limit: 1}, and reports whether a
// matching incident already exists.
//
// This is the pre-retry dedup check internal/worker runs before ever
// retrying a delivery that already failed once: a failed POST /incidents
// call does not prove the incident wasn't actually created on the far side
// (the response could have been lost to a timeout or connection reset), so
// blindly retrying risks creating a duplicate. See internal/worker.attempt's
// doc comment for the full call site and the fail-open behavior on a search
// error.
//
// Known limitation: like CreateIncident, this endpoint is ServiceNow-backed
// and requires a forwarded end-user identity token this stack cannot
// currently supply, so it also 401s on every call today (see this package's
// doc comment and this service's README/CLAUDE.md). That means the dedup
// check this method exists to support is not yet actually effective in
// production — every retry today will get a search error here and proceed
// to attempt creation anyway (fail-open, by design — see internal/worker).
// This method is structurally correct and ready for when that
// infrastructure gap is closed; it does not itself work around it.
func (c *Client) SearchIncidentByTag(ctx context.Context, tag string) (*CreateIncidentResult, bool, error) {
	req := SearchIncidentsRequest{
		Filters:    SearchIncidentsFilters{SearchQuery: tag},
		Pagination: Pagination{Limit: 1, Offset: 0},
	}
	return c.searchFirstIncident(ctx, req)
}

// SearchOpenIncidentByNumber calls POST /incidents/search filtered to the
// incident whose human-readable number exactly matches number, AND a state
// filter restricted to openIncidentStates (i.e. not Resolved/Closed/
// Cancelled), and reports whether such a still-open incident exists.
//
// This backs internal/worker's incident-grouping check: before attaching a
// new alert to an earlier alert's already-known incident (found via a
// recorded alert-incident-mapping row), this confirms that incident hasn't
// since been resolved or closed — grouping a new, currently-firing alert
// onto a closed incident would silently bury it instead of surfacing it.
//
// Known limitation: like SearchIncidentByTag and CreateIncident, this
// endpoint is ServiceNow-backed and requires a forwarded end-user identity
// token this stack cannot currently supply, so it also 401s on every call
// today (see this package's doc comment and this service's README/CLAUDE.md).
// internal/worker fails open on any error here — "we couldn't confirm the
// incident is still open" is treated the same as "not groupable, proceed as
// before," never as "assume it's open." This method is structurally correct
// and ready for when that infrastructure gap is closed; it does not itself
// work around it.
func (c *Client) SearchOpenIncidentByNumber(ctx context.Context, number string) (*CreateIncidentResult, bool, error) {
	req := SearchIncidentsRequest{
		Filters: SearchIncidentsFilters{
			Number:  &number,
			Filters: []IncidentFieldFilter{{Field: "state", Op: "in", Values: openIncidentStates}},
		},
		Pagination: Pagination{Limit: 1, Offset: 0},
	}
	return c.searchFirstIncident(ctx, req)
}

// searchFirstIncident is the shared POST /incidents/search call + decode
// path behind SearchIncidentByTag and SearchOpenIncidentByNumber — both only
// ever care about "does at least one incident match, and if so what's its
// id/number," differing only in which filters they send.
func (c *Client) searchFirstIncident(ctx context.Context, req SearchIncidentsRequest) (*CreateIncidentResult, bool, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, false, fmt.Errorf("csmclient: marshal SearchIncidentsRequest: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/incidents/search", body)
	if err != nil {
		return nil, false, err
	}

	var resp searchIncidentsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, false, fmt.Errorf("csmclient: decode incident search response: %w", err)
	}

	if len(resp.Incidents) == 0 {
		return nil, false, nil
	}

	hit := resp.Incidents[0]
	result := &CreateIncidentResult{}
	if hit.ID != nil {
		result.IncidentID = *hit.ID
	}
	if hit.Number != nil {
		result.IncidentNumber = *hit.Number
	}
	return result, true, nil
}
