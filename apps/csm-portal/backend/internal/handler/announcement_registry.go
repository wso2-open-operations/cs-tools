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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityAnnouncementRegistryClient abstracts the entity service operations
// AnnouncementRegistryHandler needs — the same SearchCases method
// CaseHandler already uses, plus the same SearchAnnouncementRequests
// AnnouncementRequestHandler already uses, on the same underlying client.
type entityAnnouncementRegistryClient interface {
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	SearchAnnouncementRequests(ctx context.Context, body []byte) ([]byte, error)
}

// AnnouncementRegistryHandler backs the Announcements tab's registry list —
// see SearchAnnouncementRegistry's own doc comment for what it actually does
// and why it needs its own handler rather than reusing CaseHandler.SearchCases
// or AnnouncementRequestHandler.SearchAnnouncementRequests directly.
type AnnouncementRegistryHandler struct {
	entity entityAnnouncementRegistryClient
}

// NewAnnouncementRegistryHandler creates an AnnouncementRegistryHandler
// backed by the given entity client.
func NewAnnouncementRegistryHandler(entity entityAnnouncementRegistryClient) *AnnouncementRegistryHandler {
	return &AnnouncementRegistryHandler{entity: entity}
}

// registryPageLimit/maxRegistryPages bound this handler's two "fetch
// everything matching the current filters" loops (cases, then published
// announcement requests) — same fail-loudly-rather-than-truncate safety-bound
// convention as announcementAudiencePageLimit/maxAnnouncementAudiencePages in
// announcement_requests.go, and for the same reason: a registry row silently
// missing because the underlying fetch gave up partway through is worse than
// a clear error telling the caller to narrow their search.
const (
	registryPageLimit = 50
	maxRegistryPages  = 200
)

// registryCaseView is the minimal subset of entity-service's
// domain.SearchCaseView this handler needs — deliberately narrow, not a
// full mirror, since every other case field is irrelevant to grouping.
type registryCaseView struct {
	ID         string  `json:"id"`
	Number     string  `json:"number"`
	InternalID string  `json:"internalId"`
	Subject    *string `json:"subject"`
	State      *string `json:"state"`
	CreatedOn  string  `json:"createdOn"`
	UpdatedOn  string  `json:"updatedOn"`
	CreatedBy  *struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"createdBy"`
	Project *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
}

type registryCaseSearchResponse struct {
	Cases  []registryCaseView `json:"cases"`
	Total  int                `json:"total"`
	Offset int                `json:"offset"`
	Limit  int                `json:"limit"`
}

// registryAnnouncementRequestView is the minimal subset of entity-service's
// domain.AnnouncementRequest this handler needs.
type registryAnnouncementRequestView struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	// CreatedBy is the raw IdP account id -- opaque, not human-readable (see
	// CreatedByEmail's own doc comment). Never shown to the caller directly;
	// resolveBatchCreatedBy prefers CreatedByEmail whenever it's set.
	CreatedBy string `json:"createdBy"`
	// CreatedByEmail is a display-only companion to CreatedBy, resolved at
	// the moment the request was created (see entity-service's
	// AnnouncementRequest.CreatedByEmail doc comment). Nil for a row written
	// before this field existed.
	CreatedByEmail         *string  `json:"createdByEmail"`
	CreatedAt              string   `json:"createdAt"`
	UpdatedAt              string   `json:"updatedAt"`
	IsSecurityAnnouncement bool     `json:"isSecurityAnnouncement"`
	ResolvedProjectCount   *int     `json:"resolvedProjectCount"`
	PublishedCaseIDs       []string `json:"publishedCaseIds"`
}

// resolveBatchCreatedBy prefers the request's own resolved email over its
// raw IdP account id -- the same "prefer the display-only email" precedent
// the Pending tab's own row already follows (r.createdByEmail || r.createdBy),
// just applied here too: a batch row's own CreatedBy used to always be the
// raw id, unlike a "case" row (built from the case's own already-resolved
// CreatedBy.Name/Email pair below), which is what made the mismatch visible
// live -- a grouped batch showed a raw UUID right next to case rows showing
// real names.
func resolveBatchCreatedBy(reqView registryAnnouncementRequestView) string {
	if reqView.CreatedByEmail != nil && *reqView.CreatedByEmail != "" {
		return *reqView.CreatedByEmail
	}
	return reqView.CreatedBy
}

type registryAnnouncementRequestSearchResponse struct {
	Requests []registryAnnouncementRequestView `json:"requests"`
	Total    int                               `json:"total"`
	Offset   int                               `json:"offset"`
	Limit    int                               `json:"limit"`
}

// registryCaseMember is one project's case within a "batch" row. Grouping
// already has to inspect every matching case (see fetchAllMatchingCases), so
// surfacing per-project detail is usually free — there is no per-case lookup
// by id to fall back on either, since case search has no "id in [...]"
// filter. The one case that isn't free is a filter that hides a displayed
// batch's own members; registryCaseLookup covers that with a second,
// unfiltered pass, and only then.
type registryCaseMember struct {
	CaseID      string `json:"caseId"`
	CaseNumber  string `json:"caseNumber"`
	WSO2CaseID  string `json:"wso2CaseId"`
	ProjectName string `json:"projectName"`
}

// registryRow is one row of the grouped registry — either a "batch" (a
// published announcement_request, representing every case in its own
// PublishedCaseIDs as one row) or a "case" (an announcement-type case with
// no known owning request — anything sent before this workflow existed, or
// via any other path). Kind tells the caller which set of fields is
// populated; the two are deliberately not split into separate response
// arrays so ordering (by recency) stays a single, simple sort.
type registryRow struct {
	Kind      string `json:"kind"`
	Subject   string `json:"subject"`
	CreatedBy string `json:"createdBy,omitempty"`
	CreatedOn string `json:"createdOn,omitempty"`
	UpdatedOn string `json:"updatedOn,omitempty"`

	// Batch-only.
	AnnouncementRequestID string               `json:"announcementRequestId,omitempty"`
	ProjectCount          int                  `json:"projectCount,omitempty"`
	Cases                 []registryCaseMember `json:"cases,omitempty"`
	// IsSecurityAnnouncement is only meaningful for kind="batch": a "case"
	// row has no owning request to read the flag from, and this handler has
	// no cheap way to know whether a bare legacy case carries the security
	// tag itself (SearchCases doesn't return a case's tags at all, only
	// GetCaseByID does — see CaseView.Tags' own doc comment; doing that
	// per-row would mean one extra upstream call per legacy case on every
	// page). Left false for "case" rows rather than guessed at.
	IsSecurityAnnouncement bool `json:"isSecurityAnnouncement,omitempty"`

	// Case-only.
	CaseID      string `json:"caseId,omitempty"`
	CaseNumber  string `json:"caseNumber,omitempty"`
	WSO2CaseID  string `json:"wso2CaseId,omitempty"`
	State       string `json:"state,omitempty"`
	ProjectName string `json:"projectName,omitempty"`
}

type registrySearchResponse struct {
	Rows    []registryRow `json:"rows"`
	Total   int           `json:"total"`
	Limit   int           `json:"limit"`
	Offset  int           `json:"offset"`
	HasMore bool          `json:"hasMore"`
}

// registrySearchRequest is this endpoint's own request shape — the same
// search/states/projectIds filters the Announcements tab already collects,
// plus pagination applied to the *grouped* result, not the raw case list
// (see SearchAnnouncementRegistry's own doc comment for why that distinction
// matters).
type registrySearchRequest struct {
	Search     string   `json:"search"`
	States     []string `json:"states"`
	ProjectIDs []string `json:"projectIds"`
	Pagination struct {
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	} `json:"pagination"`
}

// fetchAllMatchingCases pages through every announcement-type case matching
// req's own search/states/projectIds filters, newest-updated first —
// ignoring req.Pagination, which this handler applies to the *grouped*
// result afterward, not this raw fetch.
func (h *AnnouncementRegistryHandler) fetchAllMatchingCases(ctx context.Context, req registrySearchRequest) ([]registryCaseView, error) {
	fieldFilters := []map[string]any{
		{"field": "type", "op": "in", "values": []string{"announcement"}},
	}
	if len(req.States) > 0 {
		fieldFilters = append(fieldFilters, map[string]any{"field": "state", "op": "in", "values": req.States})
	}
	if len(req.ProjectIDs) > 0 {
		fieldFilters = append(fieldFilters, map[string]any{"field": "projectId", "op": "in", "values": req.ProjectIDs})
	}

	var cases []registryCaseView
	offset := 0
	for page := 0; page < maxRegistryPages; page++ {
		payload := map[string]any{
			"pagination": map[string]int{"offset": offset, "limit": registryPageLimit},
			"sortBy":     map[string]string{"field": "updatedOn", "order": "desc"},
			"filters": map[string]any{
				"filters": fieldFilters,
			},
		}
		if req.Search != "" {
			filters := payload["filters"].(map[string]any)
			filters["searchQuery"] = req.Search
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal case search request: %w", err)
		}

		raw, err := h.entity.SearchCases(ctx, body)
		if err != nil {
			return nil, err
		}
		var resp registryCaseSearchResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("decode case search response: %w", err)
		}
		cases = append(cases, resp.Cases...)
		offset += len(resp.Cases)
		if offset >= resp.Total || len(resp.Cases) == 0 {
			return cases, nil
		}
	}
	return nil, fmt.Errorf("too many matching cases to build the registry safely (exceeded %d pages of %d)", maxRegistryPages, registryPageLimit)
}

// fetchAllPublishedRequests pages through every published announcement
// request, used to build the caseId -> owning-request map. Independent of
// req's own case-level filters — a request is a candidate for grouping
// purely by having a published case in the matching set, decided by the
// caller once both fetches are in hand.
func (h *AnnouncementRegistryHandler) fetchAllPublishedRequests(ctx context.Context) ([]registryAnnouncementRequestView, error) {
	var requests []registryAnnouncementRequestView
	offset := 0
	for page := 0; page < maxRegistryPages; page++ {
		body, err := json.Marshal(map[string]any{
			"state":      "published",
			"pagination": map[string]int{"offset": offset, "limit": registryPageLimit},
		})
		if err != nil {
			return nil, fmt.Errorf("marshal announcement request search: %w", err)
		}
		raw, err := h.entity.SearchAnnouncementRequests(ctx, body)
		if err != nil {
			return nil, err
		}
		var resp registryAnnouncementRequestSearchResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("decode announcement request search response: %w", err)
		}
		requests = append(requests, resp.Requests...)
		offset += len(resp.Requests)
		if offset >= resp.Total || len(resp.Requests) == 0 {
			return requests, nil
		}
	}
	return nil, fmt.Errorf("too many published announcement requests to build the registry safely (exceeded %d pages of %d)", maxRegistryPages, registryPageLimit)
}

// registryCaseLookup returns a caseId -> case map that resolves every id in
// needed, which the caller supplies as the member ids of the batch rows on
// the page it is about to return.
//
// filtered is the caller's own filtered case set, keyed by id. Usually it
// already covers needed and is returned untouched — that holds for every
// unfiltered request, and also for a filtered one whose filter didn't
// happen to split a batch (a search term matching the shared subject, or a
// state every member shares, keeps all of them in the matching set). Only
// when the filter really did hide a member of a displayed batch is a second
// fetch worth its cost, and only then is one issued: every filter cleared
// (type=announcement only, same as fetchAllMatchingCases' own hardcoded
// filter) so every member becomes resolvable. Scoping the check to the
// page's own members, rather than to "did the caller pass any filter at
// all", is what keeps the common interactive search down to a single pass
// over the case list.
func (h *AnnouncementRegistryHandler) registryCaseLookup(ctx context.Context, filtered map[string]registryCaseView, needed []string) (map[string]registryCaseView, error) {
	complete := true
	for _, id := range needed {
		if _, ok := filtered[id]; !ok {
			complete = false
			break
		}
	}
	if complete {
		return filtered, nil
	}
	all, err := h.fetchAllMatchingCases(ctx, registrySearchRequest{})
	if err != nil {
		return nil, err
	}
	lookup := make(map[string]registryCaseView, len(all))
	for _, c := range all {
		lookup[c.ID] = c
	}
	return lookup, nil
}

// SearchAnnouncementRegistry handles POST /announcements/registry/search —
// the Announcements tab's own list, grouped by announcement instead of
// repeating the same subject once per project (Phase 3's "registry groups
// by batch" backlog item).
//
// There is no column linking a case back to the announcement_request that
// created it (only the reverse: announcement_requests.publishedCaseIds
// points forward to its own cases), and not every announcement case has a
// backing request at all — anything sent before this workflow existed, or
// via any other path, is just a bare case with no batch information. So
// this fetches the FULL matching case set and the FULL published-request
// set (each via its own bounded "fetch everything" loop above — the
// grouping can't be pushed down as a database-level JOIN across two
// completely different queries), builds a caseId -> request map from the
// requests' own PublishedCaseIDs, then walks the case list in its existing
// sort order collapsing every case that belongs to an already-seen request
// into that request's one row. Pagination (req.Pagination) is applied to
// this *grouped* row list, not the raw per-case fetch above — a batch of 50
// cases becoming 1 row means raw case offset/limit can't correspond to
// grouped-row offset/limit at all.
func (h *AnnouncementRegistryHandler) SearchAnnouncementRegistry(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req registrySearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Pagination.Limit <= 0 {
		req.Pagination.Limit = 20
	}
	if req.Pagination.Offset < 0 {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	cases, err := h.fetchAllMatchingCases(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "fetch all matching cases for registry failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search announcements.")
		return
	}
	requests, err := h.fetchAllPublishedRequests(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "fetch all published announcement requests for registry failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search announcements.")
		return
	}
	filteredByID := make(map[string]registryCaseView, len(cases))
	for _, c := range cases {
		filteredByID[c.ID] = c
	}

	caseToRequest := make(map[string]*registryAnnouncementRequestView, len(requests))
	for i := range requests {
		for _, caseID := range requests[i].PublishedCaseIDs {
			caseToRequest[caseID] = &requests[i]
		}
	}

	// Member lists are deliberately left empty here and filled in after
	// pagination below: a row that this request won't return doesn't need
	// its members resolved, and whether resolving them costs a second fetch
	// at all depends on which members the returned page actually asks for.
	var rows []registryRow
	memberIDsByRow := make(map[int][]string)
	rowAddedForRequestID := make(map[string]bool, len(requests))
	for _, c := range cases {
		if reqView, ok := caseToRequest[c.ID]; ok {
			if rowAddedForRequestID[reqView.ID] {
				continue
			}
			projectCount := len(reqView.PublishedCaseIDs)
			if reqView.ResolvedProjectCount != nil {
				projectCount = *reqView.ResolvedProjectCount
			}
			rowAddedForRequestID[reqView.ID] = true
			memberIDsByRow[len(rows)] = reqView.PublishedCaseIDs
			rows = append(rows, registryRow{
				Kind:                   "batch",
				Subject:                reqView.Subject,
				CreatedBy:              resolveBatchCreatedBy(*reqView),
				CreatedOn:              reqView.CreatedAt,
				UpdatedOn:              reqView.UpdatedAt,
				AnnouncementRequestID:  reqView.ID,
				ProjectCount:           projectCount,
				IsSecurityAnnouncement: reqView.IsSecurityAnnouncement,
			})
			continue
		}

		subject := ""
		if c.Subject != nil {
			subject = *c.Subject
		}
		state := ""
		if c.State != nil {
			state = *c.State
		}
		createdBy := ""
		if c.CreatedBy != nil {
			if c.CreatedBy.Name != "" {
				createdBy = c.CreatedBy.Name
			} else {
				createdBy = c.CreatedBy.Email
			}
		}
		projectName := ""
		if c.Project != nil {
			projectName = c.Project.Name
		}
		rows = append(rows, registryRow{
			Kind:        "case",
			Subject:     subject,
			CreatedBy:   createdBy,
			CreatedOn:   c.CreatedOn,
			UpdatedOn:   c.UpdatedOn,
			CaseID:      c.ID,
			CaseNumber:  c.Number,
			WSO2CaseID:  c.InternalID,
			State:       state,
			ProjectName: projectName,
		})
	}

	total := len(rows)
	start := req.Pagination.Offset
	if start > total {
		start = total
	}
	end := start + req.Pagination.Limit
	if end > total {
		end = total
	}
	var needed []string
	for i := start; i < end; i++ {
		needed = append(needed, memberIDsByRow[i]...)
	}
	// A batch's member list must be resolved independently of req's own
	// search/states/projectIds filters: those filters correctly decide which
	// rows appear at all (a batch shows up if just one of its cases matches),
	// but every member case still needs to be listed once it does — dropping
	// the members that didn't happen to match would understate "Delivered to
	// N projects" and omit their CS numbers entirely.
	memberLookup, err := h.registryCaseLookup(r.Context(), filteredByID, needed)
	if err != nil {
		slog.ErrorContext(r.Context(), "fetch unfiltered cases for registry batch members failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search announcements.")
		return
	}
	for i := start; i < end; i++ {
		ids := memberIDsByRow[i]
		if len(ids) == 0 {
			continue
		}
		members := make([]registryCaseMember, 0, len(ids))
		for _, cid := range ids {
			cv, ok := memberLookup[cid]
			if !ok {
				continue
			}
			projectName := ""
			if cv.Project != nil {
				projectName = cv.Project.Name
			}
			members = append(members, registryCaseMember{
				CaseID:      cv.ID,
				CaseNumber:  cv.Number,
				WSO2CaseID:  cv.InternalID,
				ProjectName: projectName,
			})
		}
		rows[i].Cases = members
	}

	page := rows[start:end]
	if page == nil {
		page = []registryRow{}
	}

	writeJSONValue(w, http.StatusOK, registrySearchResponse{
		Rows:    page,
		Total:   total,
		Limit:   req.Pagination.Limit,
		Offset:  req.Pagination.Offset,
		HasMore: end < total,
	})
}
