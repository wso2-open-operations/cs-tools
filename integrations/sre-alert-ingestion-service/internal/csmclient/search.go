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
	"strings"
	"time"
)

// TagDelimiterChars are the characters DedupTag/GroupTag use to structure a
// tag: "[", "]", ":". A field embedded unescaped inside one of those tags
// must not contain them — otherwise a crafted Source/UniqueIdentifier could
// forge a tag string that collides with a different alert's group
// (attacker-controlled grouping/dedup conflation) or inject unexpected
// structure into the free-text search query sent to csm-integration-service.
// internal/handler.AlertRequest.validate rejects these on ingress; anything
// consuming Source/UniqueIdentifier out of a persisted row (e.g.
// internal/worker.Worker.tryGroup) must re-check them too, since a legacy
// row buffered before that ingress check existed could still carry one.
const TagDelimiterChars = "[]:"

// DedupTag returns the exact, stable tag internal/handler.MapToIncident
// embeds in every CreateIncidentRequest.Subject it builds, keyed off the
// buffered alert row's own human-readable alert number (this service's own
// Postgres sequence — see internal/store.Store.Enqueue's doc comment and
// migrations/0001_create_alert_buffer.up.sql). The row's internal UUID primary
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

// GroupTag returns the tag internal/handler.buildSubject embeds in a new
// incident's Subject whenever the triggering alert carries a vendor-supplied
// UniqueIdentifier — the cross-alert grouping key a later, different alert
// reporting the same underlying condition (e.g. that condition's "resolved"
// event) searches for via SearchOpenIncidentByGroupTag, so it attaches to
// the same incident instead of creating a new one.
//
// Format: "[group:<source>:<uniqueIdentifier>]". Unlike DedupTag (unique per
// buffered alert, for this service's own retry-safety), this value is
// deliberately the *same* across every alert reporting the same condition —
// that's what makes it a grouping key rather than a per-row identifier.
//
// This mirrors, in spirit, an alert_hash + time-window dedup design found
// (2026-09) in a ServiceNow prod flow ("Create Incident from Alert") built
// for a different, not-yet-live alert pipeline — but is not a port of it:
// that flow's referenced alert_hash column does not actually exist on any
// SN table today (confirmed by direct schema read), so there was no live
// field-level mechanism to copy. The 15-minute window
// (Worker.Config.GroupWindow's default) is the one concrete, prod-confirmed
// parameter carried over from that design; the tag itself is this service's
// own.
func GroupTag(source, uniqueIdentifier string) string {
	return "[group:" + source + ":" + uniqueIdentifier + "]"
}

// SearchIncidentsFilters is the filter subset of entity-service's own
// SearchIncidentsFilters (internal/domain/entity.go) this service actually
// sends. The real upstream type carries more optional fields (Priorities,
// ParentIDs, Number) — this service only ever needs free-text SearchQuery
// (the dedup/group tag) plus a generic Filters array (state, createdOn), so
// the rest are left zero-valued/omitted rather than modeled here.
type SearchIncidentsFilters struct {
	SearchQuery string `json:"searchQuery"`
	// Filters is entity-service's generic field/op/values filter array. This
	// service sends "state" (op "in", see openIncidentStates) and,  for the
	// incident-grouping search only, "createdOn" (op "gte") to bound the
	// match to Worker.Config.GroupWindow — see SearchOpenIncidentByGroupTag.
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

// SearchOpenIncidentByGroupTag calls POST /incidents/search filtered to
// GroupTag(source, uniqueIdentifier) AND a state filter restricted to
// openIncidentStates (i.e. not Resolved/Closed/Cancelled) AND createdOn >=
// since, and reports whether such a still-open, still-within-window
// incident exists.
//
// This backs internal/worker's incident-grouping check: before creating a
// new incident for an alert that carries a UniqueIdentifier, this looks for
// an earlier alert reporting the same (source, uniqueIdentifier) condition
// whose incident is both still open and was created within the configured
// GroupWindow — see GroupTag's doc comment for why this is not a strict
// port of any single existing mechanism. since is the caller's
// responsibility to compute (Worker.now().Add(-GroupWindow)), not this
// method's — keeps this package free of a "what time is it" dependency.
//
// Known limitation: like SearchIncidentByTag and CreateIncident, this
// endpoint is ServiceNow-backed and requires a forwarded end-user identity
// token this stack cannot currently supply, so it also 401s on every call
// today (see this package's doc comment and this service's README/CLAUDE.md).
// internal/worker fails open on any error here — "we couldn't confirm a
// groupable incident exists" is treated the same as "not groupable, proceed
// as before," never as "assume one exists." This method is structurally
// correct and ready for when that infrastructure gap is closed; it does not
// itself work around it.
func (c *Client) SearchOpenIncidentByGroupTag(ctx context.Context, tag string, since time.Time) (*CreateIncidentResult, bool, error) {
	req := SearchIncidentsRequest{
		Filters: SearchIncidentsFilters{
			SearchQuery: tag,
			Filters: []IncidentFieldFilter{
				{Field: "state", Op: "in", Values: openIncidentStates},
				{Field: "createdOn", Op: "gte", Values: []string{since.UTC().Format(time.RFC3339)}},
			},
		},
		Pagination: Pagination{Limit: 1, Offset: 0},
	}
	return c.searchFirstIncident(ctx, req)
}

// searchFirstIncident is the shared POST /incidents/search call + decode
// path behind SearchIncidentByTag and SearchOpenIncidentByGroupTag — both only
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
	if hit.ID == nil || *hit.ID == "" {
		// A "match" with no incident ID is malformed, not a real hit —
		// treat it as no-match (fail open to the caller's own fallback,
		// same posture as a failed search call) rather than handing back a
		// result callers would treat as a confirmed existing incident.
		return nil, false, fmt.Errorf("csmclient: incident search hit missing incident id")
	}

	result := &CreateIncidentResult{IncidentID: *hit.ID}
	if hit.Number != nil {
		result.IncidentNumber = *hit.Number
	}
	return result, true, nil
}

// SearchITServicesFilters is the filter subset of entity-service's own
// SearchITServicesFilters this service sends: just free-text SearchQuery —
// the alert's raw, human-readable Service label (see
// internal/handler.AlertRequest.Service and internal/worker.resolveServiceID).
type SearchITServicesFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

// SearchITServicesRequest is the request body for csm-integration-service's
// POST /services/search — a thin proxy of entity-service's own
// SearchITServicesRequest, proxied as-is (field names/JSON tags copied
// verbatim), matching SearchIncidentsRequest's convention above.
type SearchITServicesRequest struct {
	Filters    *SearchITServicesFilters `json:"filters,omitempty"`
	Pagination Pagination               `json:"pagination"`
}

// ITService is the subset of entity-service's own ITService this service
// actually reads out of a search hit — enough to resolve a raw Service label
// to a CMDB service UUID. Name is kept for logging/debugging only.
type ITService struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// searchITServicesResponse is the response body for POST /services/search,
// decoded tolerantly (unknown fields ignored), matching
// searchIncidentsResponse's convention above.
type searchITServicesResponse struct {
	Services []ITService `json:"services"`
	Total    int         `json:"total"`
	Offset   int         `json:"offset"`
	Limit    int         `json:"limit"`
}

// SearchServices calls POST /services/search on csm-integration-service with
// searchQuery=label and Pagination{Limit: 1, Offset: 0}.
//
// This is the live half of internal/handler.MapToIncident's hybrid
// service-UUID resolution: the static SRE_ALERT_SERVICE_MAP lookup runs
// synchronously in the request path; when that has no entry for a label,
// internal/worker calls this method instead, at delivery-attempt time —
// never inline before the 202 response (see MapToIncident's doc comment for
// why that split exists, and internal/worker.resolveServiceID for the
// caching/fallback logic built on top of this method).
//
// entity-service's own SearchITServices does NOT do an exact match on
// name — its Postgres-backed implementation is `name ILIKE '%<query>%'`, a
// case-insensitive substring match, ordered by created_on, not by match
// quality (see entity-service/internal/repository/it_service_repo.go). A
// bare "first result" read (the original, incorrect version of this method)
// could therefore return an unrelated service whose name merely contains
// label as a substring, and — worse — since results are ordered by creation
// time rather than relevance, a real exact match is not guaranteed to be
// the first page's first row at all. So this method pages through every
// result itself and only ever returns a service whose Name matches label
// case-insensitively (folded via strings.EqualFold) and whose ID is
// non-empty — the one true "the same match a human typing this label into
// CMDB search and picking the exact-name result would have gotten",
// independent of entity-service's own ordering. servicesSearchPageSize
// bounds each page; servicesSearchMaxPages bounds the total pages walked
// (a defensive cap — a legitimately large CMDB shouldn't need anywhere
// near this many exact-name collisions on one label, and this must never
// become an unbounded loop against a live service).
//
// Returns the (possibly empty) slice of matches and a nil error on a normal
// 2xx response — an empty slice means either a confirmed zero-result search
// or a confirmed zero-*exact-match* search (both are the same "no match"
// outcome from the caller's point of view), not an error; the caller
// (internal/worker.resolveServiceID) decides what "no match" means (its
// unknown-service fallback). A non-nil error here is always a
// transient/transport-level failure (a non-2xx response, or the request
// never completing) — the caller folds that into the exact same
// retryable-delivery-failure path a CreateIncident error takes, never
// translating it into a "no match" outcome itself.
//
// Like CreateIncident and the searches above, this endpoint's underlying
// ServiceNow operation has a documented M2M-credential fallback on the
// csm-integration-service side (see that service's CLAUDE.md), so a 401 is
// possible but not unconditional — treated as retryable regardless, same as
// every other error from this call.
func (c *Client) SearchServices(ctx context.Context, label string) ([]ITService, error) {
	offset := 0
	for page := 0; page < servicesSearchMaxPages; page++ {
		req := SearchITServicesRequest{
			Filters:    &SearchITServicesFilters{SearchQuery: label},
			Pagination: Pagination{Limit: servicesSearchPageSize, Offset: offset},
		}

		body, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("csmclient: marshal SearchITServicesRequest: %w", err)
		}

		respBody, err := c.do(ctx, http.MethodPost, "/services/search", body)
		if err != nil {
			return nil, err
		}

		var resp searchITServicesResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("csmclient: decode SearchITServices response: %w", err)
		}

		for _, svc := range resp.Services {
			if svc.ID != "" && strings.EqualFold(svc.Name, label) {
				return []ITService{svc}, nil
			}
		}

		offset += len(resp.Services)
		// Stop once offset has caught up with the server's own reported
		// Total — deliberately not also keying off "this page came back
		// short:" Postgres LIMIT/OFFSET (entity-service's own backing
		// query) always returns a full page unless it's genuinely the last
		// one, but trusting that as a second, independent stop condition
		// only adds a way for the two signals to disagree; Total alone is
		// the authoritative one. An empty page with offset still short of
		// Total (a buggy/inconsistent server response) does not infinite
		// loop — it just stops making progress, and servicesSearchMaxPages
		// is the backstop that ends the loop regardless.
		if offset >= resp.Total {
			return nil, nil
		}
	}

	// servicesSearchMaxPages exhausted without a short/complete page ever
	// being seen — treat as no match rather than looping further; see this
	// const's own doc comment for why this is a defensive cap, not an
	// expected outcome.
	return nil, nil
}

// servicesSearchPageSize is the page size SearchServices requests per call
// to POST /services/search while walking for an exact-name match.
const servicesSearchPageSize = 50

// servicesSearchMaxPages bounds how many pages SearchServices will walk
// before giving up and treating the search as a no-match — a defensive cap
// against ever looping unbounded against a live service, not a value this
// service expects to actually hit in practice (see SearchServices' own doc
// comment).
const servicesSearchMaxPages = 20
