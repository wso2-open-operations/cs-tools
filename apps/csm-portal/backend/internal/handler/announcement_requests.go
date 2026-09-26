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
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityAnnouncementRequestClient abstracts the entity service operations
// used by AnnouncementRequestHandler.
type entityAnnouncementRequestClient interface {
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	SearchProjectsByProductVersion(ctx context.Context, body []byte) ([]byte, error)
	CreateAnnouncementRequest(ctx context.Context, body []byte) ([]byte, error)
	GetAnnouncementRequest(ctx context.Context, id string) ([]byte, error)
	SearchAnnouncementRequests(ctx context.Context, body []byte) ([]byte, error)
	UpdateAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error)
	RecordAnnouncementRequestDryRun(ctx context.Context, id string, body []byte) ([]byte, error)
	SubmitAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error)
	ApproveAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error)
	ScheduleAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error)
	PublishAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error)
	CreateAnnouncementRequestUpdate(ctx context.Context, id string, body []byte) ([]byte, error)
	ListAnnouncementRequestUpdates(ctx context.Context, id string) ([]byte, error)
	RecordAnnouncementRequestDeliveries(ctx context.Context, id string, body []byte) ([]byte, error)
	ListAnnouncementRequestDeliveries(ctx context.Context, id string) ([]byte, error)
}

// AnnouncementRequestHandler handles HTTP requests for the Phase 2
// draft/pending_approval/approved/published announcement workflow —
// everything before an announcement request's own Publish action fans out
// into the real per-project cases that CreateCustomerAnnouncementForm /
// CreateEolAnnouncementForm already create today (that fan-out is
// unchanged, and does not go through this handler at all).
type AnnouncementRequestHandler struct {
	entity              entityAnnouncementRequestClient
	excludedProjectKeys []string
}

// NewAnnouncementRequestHandler creates an AnnouncementRequestHandler backed
// by the given entity client and the same mandatory excluded-project-key
// list AnnouncementHandler uses (see loadAnnouncementExcludedProjectKeys in
// main.go) — Submit's own audience resolution must apply the identical
// exclusion policy a live "All customer projects" search already does, or
// the frozen snapshot it produces could include a project the deployment
// has configured as permanently excluded.
func NewAnnouncementRequestHandler(entity entityAnnouncementRequestClient, excludedProjectKeys []string) *AnnouncementRequestHandler {
	return &AnnouncementRequestHandler{entity: entity, excludedProjectKeys: excludedProjectKeys}
}

// announcementAudiencePageLimit/maxAnnouncementAudiencePages bound Submit's
// own paginated audience resolution the same way the webapp's
// useResolveAnnouncementAudience/useResolveProductVersionAudience hooks
// bound theirs (see those hooks' own MAX_AUDIENCE_PAGES) — resolving "all
// customer projects" server-side here needs the identical fail-loudly
// safety net: silently truncating an over-large audience and freezing that
// truncated list as the Submit snapshot would be far worse than refusing
// to submit at all.
const (
	announcementAudiencePageLimit = 50
	maxAnnouncementAudiencePages  = 200
)

func readJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}

// CreateAnnouncementRequest handles POST /announcement-requests. createdBy
// is always the authenticated caller — never a client-supplied value. The
// inbound struct below has no field for it at all (not even a `json:"-"`
// one — that tag suppresses marshaling too, which would silently drop the
// real value again when re-encoding for upstream, not just block it from
// being read on the way in), so nothing in the request body could ever
// populate one; the outbound struct is a separate type that adds it back
// in with the authenticated value.
func (h *AnnouncementRequestHandler) CreateAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req struct {
		Kind                   string          `json:"kind"`
		Subject                string          `json:"subject"`
		Description            string          `json:"description"`
		IsSecurityAnnouncement bool            `json:"isSecurityAnnouncement"`
		AudienceDefinition     json.RawMessage `json:"audienceDefinition"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	upstreamBody, err := json.Marshal(struct {
		Kind                   string          `json:"kind"`
		Subject                string          `json:"subject"`
		Description            string          `json:"description"`
		IsSecurityAnnouncement bool            `json:"isSecurityAnnouncement"`
		AudienceDefinition     json.RawMessage `json:"audienceDefinition,omitempty"`
		CreatedBy              string          `json:"createdBy"`
		CreatedByEmail         string          `json:"createdByEmail,omitempty"`
	}{
		Kind:                   req.Kind,
		Subject:                req.Subject,
		Description:            req.Description,
		IsSecurityAnnouncement: req.IsSecurityAnnouncement,
		AudienceDefinition:     req.AudienceDefinition,
		CreatedBy:              user.UserID,
		CreatedByEmail:         user.Email,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.CreateAnnouncementRequest(r.Context(), upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateAnnouncementRequest failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create the announcement request.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// GetAnnouncementRequest handles GET /announcement-requests/{id}.
func (h *AnnouncementRequestHandler) GetAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetAnnouncementRequest(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAnnouncementRequest failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve the announcement request.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchAnnouncementRequests handles POST /announcement-requests/search —
// a plain passthrough, no actor injection needed for a read.
func (h *AnnouncementRequestHandler) SearchAnnouncementRequests(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchAnnouncementRequests(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAnnouncementRequests failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search announcement requests.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// UpdateAnnouncementRequest handles PATCH /announcement-requests/{id}. Uses
// mapUpstreamError (not the Generic variant) per this backend's own PATCH
// convention: the payload just submitted is what's being rejected (e.g. an
// audience change on an approved request), so the upstream reason is
// caller-actionable and worth surfacing.
func (h *AnnouncementRequestHandler) UpdateAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req struct {
		Subject                *string         `json:"subject,omitempty"`
		Description            *string         `json:"description,omitempty"`
		IsSecurityAnnouncement *bool           `json:"isSecurityAnnouncement,omitempty"`
		AudienceDefinition     json.RawMessage `json:"audienceDefinition,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	upstreamBody, err := json.Marshal(struct {
		Subject                *string         `json:"subject,omitempty"`
		Description            *string         `json:"description,omitempty"`
		IsSecurityAnnouncement *bool           `json:"isSecurityAnnouncement,omitempty"`
		AudienceDefinition     json.RawMessage `json:"audienceDefinition,omitempty"`
		ActorID                string          `json:"actorId"`
	}{
		Subject:                req.Subject,
		Description:            req.Description,
		IsSecurityAnnouncement: req.IsSecurityAnnouncement,
		AudienceDefinition:     req.AudienceDefinition,
		ActorID:                user.UserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.UpdateAnnouncementRequest(r.Context(), id, upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateAnnouncementRequest failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to update the announcement request.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// RecordAnnouncementRequestDryRun handles
// POST /announcement-requests/{id}/dry-run. caseId is the real case id the
// caller's own dry-run mechanism (useAnnouncementDryRun.ts) already
// created — this handler does not create it and does not verify it exists.
func (h *AnnouncementRequestHandler) RecordAnnouncementRequestDryRun(w http.ResponseWriter, r *http.Request) {
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req struct {
		CaseID string `json:"caseId"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.CaseID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	upstreamBody, err := json.Marshal(struct {
		CaseID  string `json:"caseId"`
		ActorID string `json:"actorId"`
	}{CaseID: req.CaseID, ActorID: user.UserID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.RecordAnnouncementRequestDryRun(r.Context(), id, upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity RecordAnnouncementRequestDryRun failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to record the dry run.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ApproveAnnouncementRequest handles
// POST /announcement-requests/{id}/approve. Takes no request body — the
// only input this transition needs is who's performing it, and that's
// always the authenticated caller, never anything read from the request.
// There is deliberately no approver-role check here (see
// entity-service's own AnnouncementRequestService.Approve doc comment):
// the real approval decision already happened over email, outside this
// platform entirely.
func (h *AnnouncementRequestHandler) ApproveAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	h.actorOnlyTransition(w, r, "approve", h.entity.ApproveAnnouncementRequest, "Failed to approve the announcement request.")
}

// ScheduleAnnouncementRequest handles
// POST /announcement-requests/{id}/schedule. Sets or clears (a null/omitted
// scheduledFor clears it) an approved request's automatic-publish time —
// once set, operations/csm-scheduled-tasks' publish_scheduled_announcements
// sub-cron publishes it automatically once that time arrives, the same way
// the manual Publish button does today. actorId is always the authenticated
// caller, never read from the request body — same restriction as Publish
// (scheduling is choosing when Publish happens).
func (h *AnnouncementRequestHandler) ScheduleAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req struct {
		ScheduledFor *string `json:"scheduledFor"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	upstreamBody, err := json.Marshal(struct {
		ActorID      string  `json:"actorId"`
		ScheduledFor *string `json:"scheduledFor"`
		ActorEmail   string  `json:"actorEmail,omitempty"`
	}{ActorID: user.UserID, ScheduledFor: req.ScheduledFor, ActorEmail: user.Email})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.ScheduleAnnouncementRequest(r.Context(), id, upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ScheduleAnnouncementRequest failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to schedule the announcement request.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PublishAnnouncementRequest handles
// POST /announcement-requests/{id}/publish. Unlike Approve, this does take
// a body: caseIds, the real case id created for each project in the
// request's own resolvedProjectIds, from the webapp's own fan-out
// (unchanged from today, and still what actually sends the announcement —
// this call only records that it happened and which cases resulted, so a
// later update can target them). actorId is still always the authenticated
// caller, never read from the request body.
func (h *AnnouncementRequestHandler) PublishAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req struct {
		CaseIDs []string `json:"caseIds"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if len(req.CaseIDs) == 0 {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	upstreamBody, err := json.Marshal(struct {
		ActorID    string   `json:"actorId"`
		CaseIDs    []string `json:"caseIds"`
		ActorEmail string   `json:"actorEmail,omitempty"`
	}{ActorID: user.UserID, CaseIDs: req.CaseIDs, ActorEmail: user.Email})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.PublishAnnouncementRequest(r.Context(), id, upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PublishAnnouncementRequest failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to publish the announcement request.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// CreateAnnouncementRequestUpdate handles
// POST /announcement-requests/{id}/updates. content is the follow-up text;
// actorId is always the authenticated caller. Does not itself apply content
// as a comment anywhere — the webapp's own fan-out (POST
// /cases/{id}/comments per id in publishedCaseIds) does that, separately,
// after this call succeeds.
func (h *AnnouncementRequestHandler) CreateAnnouncementRequestUpdate(w http.ResponseWriter, r *http.Request) {
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	upstreamBody, err := json.Marshal(struct {
		Content    string `json:"content"`
		ActorID    string `json:"actorId"`
		ActorEmail string `json:"actorEmail,omitempty"`
	}{Content: req.Content, ActorID: user.UserID, ActorEmail: user.Email})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.CreateAnnouncementRequestUpdate(r.Context(), id, upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateAnnouncementRequestUpdate failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to post the update.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// ListAnnouncementRequestUpdates handles
// GET /announcement-requests/{id}/updates — a plain passthrough, no actor
// injection needed for a read.
func (h *AnnouncementRequestHandler) ListAnnouncementRequestUpdates(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.ListAnnouncementRequestUpdates(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListAnnouncementRequestUpdates failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to list updates for the announcement request.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// announcementRequestDeliveryInput mirrors entity-service's
// RecordAnnouncementRequestDeliveryInput — decoded here only to validate
// shape before forwarding (projectId/status required), not reshaped.
type announcementRequestDeliveryInput struct {
	ProjectID    string  `json:"projectId"`
	CaseID       *string `json:"caseId,omitempty"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"errorMessage,omitempty"`
}

// RecordAnnouncementRequestDeliveries handles
// POST /announcement-requests/{id}/deliveries. deliveries is the caller's
// own Publish fan-out pass result, one entry per project attempted in that
// pass; actorId is always the authenticated caller, never client-supplied —
// same restriction as publish, since this is bookkeeping for the same real
// send only the request's own creator can drive.
func (h *AnnouncementRequestHandler) RecordAnnouncementRequestDeliveries(w http.ResponseWriter, r *http.Request) {
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req struct {
		Deliveries []announcementRequestDeliveryInput `json:"deliveries"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if len(req.Deliveries) == 0 {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	for _, d := range req.Deliveries {
		if d.ProjectID == "" || d.Status == "" {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
	}

	upstreamBody, err := json.Marshal(struct {
		ActorID    string                             `json:"actorId"`
		Deliveries []announcementRequestDeliveryInput `json:"deliveries"`
	}{ActorID: user.UserID, Deliveries: req.Deliveries})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.RecordAnnouncementRequestDeliveries(r.Context(), id, upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity RecordAnnouncementRequestDeliveries failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to record the delivery status.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ListAnnouncementRequestDeliveries handles
// GET /announcement-requests/{id}/deliveries — a plain passthrough, no
// actor injection needed for a read.
func (h *AnnouncementRequestHandler) ListAnnouncementRequestDeliveries(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.ListAnnouncementRequestDeliveries(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListAnnouncementRequestDeliveries failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to list deliveries for the announcement request.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AnnouncementRequestHandler) actorOnlyTransition(
	w http.ResponseWriter,
	r *http.Request,
	action string,
	call func(ctx context.Context, id string, body []byte) ([]byte, error),
	fallbackMsg string,
) {
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

	body, err := json.Marshal(struct {
		ActorID    string `json:"actorId"`
		ActorEmail string `json:"actorEmail,omitempty"`
	}{ActorID: user.UserID, ActorEmail: user.Email})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := call(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity announcement request "+action+" failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, fallbackMsg)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// projectSearchPage is the minimal shape this handler needs from either
// /projects/search's or /deployed-products/projects/search's response —
// both entity service endpoints share the same
// {projects, total, limit, offset, hasMore} envelope.
type projectSearchPage struct {
	Projects []struct {
		ID string `json:"id"`
	} `json:"projects"`
	HasMore bool `json:"hasMore"`
}

// projectSearchPagination is the outgoing request-body shape both
// /projects/search and /deployed-products/projects/search expect for
// pagination (see entity-service's own domain.Pagination) — shared here so
// resolveCustomerAudience/resolveEOLAudience can't drift into their own,
// differently-tagged copies.
type projectSearchPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// resolveAllProjectIDs pages through fetchPage (offset, limit) -> raw JSON,
// accumulating every project id across all pages. Bounded by
// maxAnnouncementAudiencePages so a wrong/always-true hasMore can't loop
// forever — the same safety-bound convention the webapp's own paged
// audience-resolution hooks use, applied here because Submit's own
// resolution has no equivalent client-side loop to fall back on.
func resolveAllProjectIDs(fetchPage func(offset, limit int) ([]byte, error)) ([]string, error) {
	var ids []string
	offset := 0
	for pageNum := 0; ; pageNum++ {
		if pageNum == maxAnnouncementAudiencePages {
			return nil, fmt.Errorf("too many matching projects to resolve safely (exceeded %d pages of %d)", maxAnnouncementAudiencePages, announcementAudiencePageLimit)
		}
		raw, err := fetchPage(offset, announcementAudiencePageLimit)
		if err != nil {
			return nil, err
		}
		var page projectSearchPage
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("decode project search response: %w", err)
		}
		for _, p := range page.Projects {
			ids = append(ids, p.ID)
		}
		offset += len(page.Projects)
		if !page.HasMore || len(page.Projects) == 0 {
			break
		}
	}
	return ids, nil
}

// customerAudienceDefinition is this handler's own contract for what a
// "customer" kind announcement_requests row's audienceDefinition JSON
// blob contains — deliberately the exact same shape
// CreateCustomerAnnouncementForm.tsx already builds for a live
// /announcements/audience/search resolution (excludeClosureStates/
// excludeSubscriptionTypes as the actual value lists, not booleans), so
// Submit can forward them straight through per page with no translation.
type customerAudienceDefinition struct {
	Scope                    string   `json:"scope"`
	ProjectIDs               []string `json:"projectIds,omitempty"`
	ExcludeClosureStates     []string `json:"excludeClosureStates,omitempty"`
	ExcludeSubscriptionTypes []string `json:"excludeSubscriptionTypes,omitempty"`
}

// eolAudienceDefinition is the "eol" kind's own audienceDefinition shape.
type eolAudienceDefinition struct {
	ProductID        string `json:"productId"`
	ProductVersionID string `json:"productVersionId"`
}

// resolveCustomerAudience resolves a "customer" kind request's frozen
// project-id snapshot. "specific" scope needs no search at all — the
// audience already *is* whichever projects were hand-picked. "all" scope
// pages through the entity service's project search with the mandatory
// excluded-project-key denylist injected, exactly like a live
// SearchCustomerAnnouncementAudience resolution — reusing
// injectExcludeProjectKeys rather than re-deriving that policy here.
func (h *AnnouncementRequestHandler) resolveCustomerAudience(ctx context.Context, raw json.RawMessage) ([]string, error) {
	var def customerAudienceDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, fmt.Errorf("decode customer audienceDefinition: %w", err)
	}

	switch def.Scope {
	case "specific":
		return def.ProjectIDs, nil
	case "all":
		// falls through to the resolution below
	default:
		// Any other value — including empty, a typo, or malformed stored
		// data — must fail loudly. Silently falling through to "all" here
		// would mean a broken/unexpected scope value resolves to every
		// customer project instead of the hand-picked list it was
		// probably meant to be, turning a data bug into an announcement
		// sent to the wrong audience entirely.
		return nil, fmt.Errorf("unknown customer audienceDefinition scope: %q", def.Scope)
	}

	return resolveAllProjectIDs(func(offset, limit int) ([]byte, error) {
		body, err := json.Marshal(struct {
			Pagination               projectSearchPagination `json:"pagination"`
			ExcludeClosureStates     []string                `json:"excludeClosureStates,omitempty"`
			ExcludeSubscriptionTypes []string                `json:"excludeSubscriptionTypes,omitempty"`
		}{
			Pagination:               projectSearchPagination{Limit: limit, Offset: offset},
			ExcludeClosureStates:     def.ExcludeClosureStates,
			ExcludeSubscriptionTypes: def.ExcludeSubscriptionTypes,
		})
		if err != nil {
			return nil, err
		}
		body, err = injectExcludeProjectKeys(body, h.excludedProjectKeys)
		if err != nil {
			return nil, err
		}
		return h.entity.SearchProjects(ctx, body)
	})
}

// resolveEOLAudience resolves an "eol" kind request's frozen project-id
// snapshot by paging through the entity service's product-version project
// search. The mandatory closure-state/subscription-type exclusions for
// this flow are applied unconditionally by the entity service itself (see
// useResolveProductVersionAudience.ts's own doc comment) — no exclusion
// injection needed here, unlike the customer flow's "all" scope.
func (h *AnnouncementRequestHandler) resolveEOLAudience(ctx context.Context, raw json.RawMessage) ([]string, error) {
	var def eolAudienceDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, fmt.Errorf("decode eol audienceDefinition: %w", err)
	}
	if def.ProductID == "" || def.ProductVersionID == "" {
		return nil, fmt.Errorf("eol audienceDefinition missing productId/productVersionId")
	}

	return resolveAllProjectIDs(func(offset, limit int) ([]byte, error) {
		body, err := json.Marshal(struct {
			ProductID        string                  `json:"productId"`
			ProductVersionID string                  `json:"productVersionId"`
			Pagination       projectSearchPagination `json:"pagination"`
		}{
			ProductID:        def.ProductID,
			ProductVersionID: def.ProductVersionID,
			Pagination:       projectSearchPagination{Limit: limit, Offset: offset},
		})
		if err != nil {
			return nil, err
		}
		return h.entity.SearchProjectsByProductVersion(ctx, body)
	})
}

// SubmitAnnouncementRequest handles
// POST /announcement-requests/{id}/submit. This is the one endpoint on
// this handler with real logic beyond auth + actor injection + passthrough:
// it loads the request's own stored kind/audienceDefinition, resolves the
// authoritative (exclusion-applied) project-id snapshot itself — never
// trusting a client-supplied list, the same threat model
// injectExcludeProjectKeys already exists to close — and only then forwards
// to the entity service's own Submit, which separately enforces the
// dry-run-recorded precondition.
func (h *AnnouncementRequestHandler) SubmitAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
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

	currentRaw, err := h.entity.GetAnnouncementRequest(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAnnouncementRequest failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load the announcement request.")
		return
	}

	var current struct {
		Kind               string          `json:"kind"`
		AudienceDefinition json.RawMessage `json:"audienceDefinition"`
	}
	if err := json.Unmarshal(currentRaw, &current); err != nil {
		slog.ErrorContext(r.Context(), "decode announcement request failed", "userID", user.UserID, "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	var projectIDs []string
	switch current.Kind {
	case "customer":
		projectIDs, err = h.resolveCustomerAudience(r.Context(), current.AudienceDefinition)
	case "eol":
		projectIDs, err = h.resolveEOLAudience(r.Context(), current.AudienceDefinition)
	default:
		writeError(w, http.StatusBadRequest, "Unknown announcement request kind.")
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "resolve announcement request audience failed", "userID", user.UserID, "id", id, "err", err)
		// An *apierror.Error means SearchProjects/SearchProjectsByProductVersion
		// itself failed (a real upstream problem — could be transient, e.g. a
		// 503) — map it through the normal upstream-error path instead of
		// flattening it to a 400, which would misreport a retriable failure
		// as a permanent client-side one. Anything else here is this
		// handler's own local validation (malformed stored audienceDefinition,
		// an unrecognized scope, the page-count safety bound) — a real 400,
		// since retrying without first fixing the stored data would fail the
		// same way every time.
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) {
			mapUpstreamErrorGeneric(w, err, "Failed to resolve the announcement audience.")
			return
		}
		writeError(w, http.StatusBadRequest, "Failed to resolve the announcement audience.")
		return
	}
	if len(projectIDs) == 0 {
		writeError(w, http.StatusBadRequest, "The resolved audience is empty — nothing to send to.")
		return
	}

	body, err := json.Marshal(struct {
		ResolvedProjectIDs []string `json:"resolvedProjectIds"`
		ActorID            string   `json:"actorId"`
		ActorEmail         string   `json:"actorEmail,omitempty"`
	}{ResolvedProjectIDs: projectIDs, ActorID: user.UserID, ActorEmail: user.Email})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.SubmitAnnouncementRequest(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SubmitAnnouncementRequest failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to submit the announcement request for approval.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
