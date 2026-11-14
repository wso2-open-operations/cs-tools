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
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityTimeCardClient abstracts the entity service time-card operations.
type entityTimeCardClient interface {
	SearchTimeCards(ctx context.Context, body []byte) ([]byte, error)
	AggregateTimeCards(ctx context.Context, body []byte) ([]byte, error)
	CreateTimeCard(ctx context.Context, body []byte) ([]byte, error)
	UpdateTimeCard(ctx context.Context, id string, body []byte) ([]byte, error)
	DeleteTimeCard(ctx context.Context, id string) ([]byte, error)
}

// TimeCardHandler handles HTTP requests for time-card operations.
type TimeCardHandler struct {
	entity entityTimeCardClient
	// access backs the approve/reject check in UpdateTimeCard -- see
	// WithAccessGuard. nil fails that check closed (denied), never open,
	// mirroring CaseHandler's own field of the same name/reasoning.
	access *AccessGuard
}

// NewTimeCardHandler creates a TimeCardHandler backed by the given entity client.
func NewTimeCardHandler(entity entityTimeCardClient) *TimeCardHandler {
	return &TimeCardHandler{entity: entity}
}

// WithAccessGuard wires the same guard that authorises every route into this
// handler, so UpdateTimeCard can additionally require PermApproveTimeCard for
// a state-transition (approve/reject) request -- a restriction PermView*/
// PermTimeCardsAndUpdates alone (the route-level permission this route already
// carries, shared with ordinary field edits) cannot express, the same
// shared-route problem CaseHandler.WithAccessGuard solves for Security Center
// -- see that method's own doc comment. Returns h for chaining at the
// construction site.
func (h *TimeCardHandler) WithAccessGuard(g *AccessGuard) *TimeCardHandler {
	h.access = g
	return h
}

// SearchTimeCards handles POST /time-cards/search.
func (h *TimeCardHandler) SearchTimeCards(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchTimeCards(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchTimeCards failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search time cards.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// AggregateTimeCards handles POST /time-cards/aggregate: time card counts and
// summed minutes per state, user, project or day, for dashboard charts. The
// groupBy / measure / day-window rules are entity-service's; this layer
// forwards the body and passes the response through. The route needs
// PermViewTimeCardTotals (CS engineer and admin), narrower than the search's
// PermTimeCardsAndUpdates; see that permission's doc comment.
func (h *TimeCardHandler) AggregateTimeCards(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readTimeCardBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.AggregateTimeCards(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AggregateTimeCards failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to aggregate time cards.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// readTimeCardBody applies the 1 MiB cap and JSON-validity guard, returning the
// body and true on success; on failure it has already written the error response.
func readTimeCardBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}

// CreateTimeCard handles POST /time-cards.
func (h *TimeCardHandler) CreateTimeCard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readTimeCardBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.CreateTimeCard(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateTimeCard failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create time card.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// timeCardUpdateTargetsStateTransition reports whether body is an
// approve/reject request rather than a plain field edit -- entity-service's
// UpdateTimeCardRequest (openapi.yaml) carries EITHER editable fields OR a
// state transition (`state: "approved"`/`"rejected"`) on this same PATCH
// route, so the two can only be told apart by inspecting the body itself,
// the same best-effort JSON-inspection approach
// caseSearchTargetsSecurityReports uses for its own shared-route problem. A
// body this can't make sense of is treated as not a transition, since a
// genuinely malformed request is rejected by entity-service's own validation
// regardless of what this check decides.
func timeCardUpdateTargetsStateTransition(body []byte) bool {
	var req struct {
		State *string `json:"state"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	return req.State != nil
}

// timeCardUpdateIsBillableOnly reports whether body is, as entity-service
// itself decodes it, a request carrying isBillable and no other field -- the
// shape it treats as an override of an approved card's billable flag. It
// decodes into a struct mirroring entity-service's UpdateTimeCardRequest
// field set so both sides agree exactly on case-insensitive keys, ignored
// unknown keys and null values (null decodes as absent); pointer and slice
// types make nil mean "absent", and an empty approverIds array is non-nil, as
// downstream. Keep this field list in step with that request type. A body
// that does not decode is reported as false so upstream validation behaves as
// before.
func timeCardUpdateIsBillableOnly(body []byte) bool {
	var req struct {
		State                    *string  `json:"state"`
		LeadComment              *string  `json:"leadComment"`
		Date                     *string  `json:"date"`
		ApproverIDs              []string `json:"approverIds"`
		IsBillable               *bool    `json:"isBillable"`
		IssueComplexity          *string  `json:"issueComplexity"`
		WorkLogComment           *string  `json:"workLogComment"`
		TimeAnalyzing            *int     `json:"timeAnalyzing"`
		TimeSettingUp            *int     `json:"timeSettingUp"`
		TimeReproducingDebugging *int     `json:"timeReproducingDebugging"`
		TimeProvidingSolution    *int     `json:"timeProvidingSolution"`
		TimePatching             *int     `json:"timePatching"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	return req.IsBillable != nil && req.State == nil && req.LeadComment == nil && req.Date == nil &&
		req.ApproverIDs == nil && req.IssueComplexity == nil && req.WorkLogComment == nil &&
		req.TimeAnalyzing == nil && req.TimeSettingUp == nil && req.TimeReproducingDebugging == nil &&
		req.TimeProvidingSolution == nil && req.TimePatching == nil
}

// UpdateTimeCard handles PATCH /time-cards/{id}.
func (h *TimeCardHandler) UpdateTimeCard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readTimeCardBody(w, r)
	if !ok {
		return
	}

	if timeCardUpdateTargetsStateTransition(body) && !(h.access != nil && h.access.Permits(PermApproveTimeCard, user.Roles)) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return
	}

	// An isBillable-only body can override an approved card, and upstream does
	// not check the caller's role for that, so it is admin-only here.
	if timeCardUpdateIsBillableOnly(body) && !(h.access != nil && h.access.Permits(PermOverrideApprovedTimeCardBillable, user.Roles)) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return
	}

	result, err := h.entity.UpdateTimeCard(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateTimeCard failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to update time card.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteTimeCard handles DELETE /time-cards/{id}. Carries no body for
// upstream to reject with a caller-fixable reason, so — unlike UpdateTimeCard
// above — this uses mapUpstreamErrorGeneric, matching every other
// non-PATCH-with-a-body handler (see backend CLAUDE.md's Handler conventions).
func (h *TimeCardHandler) DeleteTimeCard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.DeleteTimeCard(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteTimeCard failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to delete time card.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
