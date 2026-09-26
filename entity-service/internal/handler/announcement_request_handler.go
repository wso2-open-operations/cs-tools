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
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// autoPublishHandlerTimeout bounds how long AutoPublishAnnouncementRequest
// waits for the whole per-project fan-out (see AnnouncementRequestService.
// AutoPublish's own doc comment) -- deliberately well under that service's
// own autoPublishClaimStaleAfter (5 minutes), so a still-legitimately-running
// attempt is never mistaken by a later claim attempt for a dead one.
// Deliberately NOT the global 30s request timeout (middleware.Timeout,
// routes.go): a real announcement can target upward of a thousand projects,
// each a real ServiceNow round trip, so 30s is nowhere near enough for one
// call to make meaningful progress -- see autoPublishWriteDeadlineBuffer's
// own comment for why extending only this handler's deadlines (not the
// global default) is safe.
// Kept safely under AnnouncementRequestService's own autoPublishClaimStaleAfter
// (13 minutes) -- see that constant's own comment for the margin reasoning.
const autoPublishHandlerTimeout = 11 * time.Minute

// autoPublishWriteDeadlineBuffer pads the extended write deadline past
// autoPublishHandlerTimeout, so the connection's own deadline can never
// expire before the context timeout does (which would otherwise fail the
// final response write even on a successful, on-time completion).
const autoPublishWriteDeadlineBuffer = 30 * time.Second

// AnnouncementRequestHandler handles HTTP requests for the
// announcement_requests resource — see domain.AnnouncementRequest's doc
// comment for what it's for.
type AnnouncementRequestHandler struct {
	svc service.AnnouncementRequestService
}

// NewAnnouncementRequestHandler constructs an AnnouncementRequestHandler
// with the given service.
func NewAnnouncementRequestHandler(svc service.AnnouncementRequestService) *AnnouncementRequestHandler {
	return &AnnouncementRequestHandler{svc: svc}
}

func writeAnnouncementRequestJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// CreateAnnouncementRequest handles POST /announcement-requests.
func (h *AnnouncementRequestHandler) CreateAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateAnnouncementRequestRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CreateDraft(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusCreated, resp)
}

// GetAnnouncementRequest handles GET /announcement-requests/{id}.
func (h *AnnouncementRequestHandler) GetAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// SearchAnnouncementRequests handles POST /announcement-requests/search.
func (h *AnnouncementRequestHandler) SearchAnnouncementRequests(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchAnnouncementRequestsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// UpdateAnnouncementRequest handles PATCH /announcement-requests/{id}.
func (h *AnnouncementRequestHandler) UpdateAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateAnnouncementRequestRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Update(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// RecordAnnouncementRequestDryRun handles
// POST /announcement-requests/{id}/dry-run.
func (h *AnnouncementRequestHandler) RecordAnnouncementRequestDryRun(w http.ResponseWriter, r *http.Request) {
	var req domain.RecordAnnouncementDryRunRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.RecordDryRun(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// SubmitAnnouncementRequest handles
// POST /announcement-requests/{id}/submit.
func (h *AnnouncementRequestHandler) SubmitAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	var req domain.SubmitAnnouncementRequestRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Submit(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// ApproveAnnouncementRequest handles
// POST /announcement-requests/{id}/approve.
func (h *AnnouncementRequestHandler) ApproveAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	var req domain.AnnouncementRequestActorRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Approve(r.Context(), r.PathValue("id"), req.ActorID, req.ActorEmail)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// ScheduleAnnouncementRequest handles
// POST /announcement-requests/{id}/schedule.
func (h *AnnouncementRequestHandler) ScheduleAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	var req domain.ScheduleAnnouncementRequestRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Schedule(r.Context(), r.PathValue("id"), req.ActorID, req.ActorEmail, req.ScheduledFor)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// AutoPublishAnnouncementRequest handles
// POST /announcement-requests/{id}/auto-publish. Internal-caller-only (see
// AnnouncementRequestService.AutoPublish's own doc comment) — takes no
// body at all, unlike every other action here: there is no actor to
// authenticate, since this is never called by a browser.
//
// Escapes the global 30s request timeout (middleware.Timeout, routes.go)
// deliberately, for this route only: AutoPublish's own fan-out is one real
// ServiceNow round trip per target project, sequentially, and a large
// announcement can target well over a thousand of them -- 30s is nowhere
// near enough for one call to make useful progress. context.WithoutCancel
// detaches from the request's own (30s-bounded) context before applying a
// fresh, longer deadline, and http.NewResponseController extends the
// underlying connection's write deadline to match -- otherwise the
// stdlib server's own WriteTimeout (server.go) would still fail the
// response write once this handler finally returns, even on success.
func (h *AnnouncementRequestHandler) AutoPublishAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(autoPublishHandlerTimeout + autoPublishWriteDeadlineBuffer)); err != nil {
		writeServiceError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), autoPublishHandlerTimeout)
	defer cancel()
	resp, err := h.svc.AutoPublish(ctx, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// PublishAnnouncementRequest handles
// POST /announcement-requests/{id}/publish.
func (h *AnnouncementRequestHandler) PublishAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	var req domain.PublishAnnouncementRequestRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.MarkPublished(r.Context(), r.PathValue("id"), req.ActorID, req.ActorEmail, req.CaseIDs)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// CreateAnnouncementRequestUpdate handles
// POST /announcement-requests/{id}/updates.
func (h *AnnouncementRequestHandler) CreateAnnouncementRequestUpdate(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateAnnouncementRequestUpdateRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.AddUpdate(r.Context(), r.PathValue("id"), req.ActorID, req.ActorEmail, req.Content)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusCreated, resp)
}

// ListAnnouncementRequestUpdates handles
// GET /announcement-requests/{id}/updates.
func (h *AnnouncementRequestHandler) ListAnnouncementRequestUpdates(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListUpdates(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// RecordAnnouncementRequestDeliveries handles
// POST /announcement-requests/{id}/deliveries.
func (h *AnnouncementRequestHandler) RecordAnnouncementRequestDeliveries(w http.ResponseWriter, r *http.Request) {
	var req domain.RecordAnnouncementRequestDeliveriesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.RecordDeliveries(r.Context(), r.PathValue("id"), req.ActorID, req.Deliveries)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// ListAnnouncementRequestDeliveries handles
// GET /announcement-requests/{id}/deliveries.
func (h *AnnouncementRequestHandler) ListAnnouncementRequestDeliveries(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListDeliveries(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}
