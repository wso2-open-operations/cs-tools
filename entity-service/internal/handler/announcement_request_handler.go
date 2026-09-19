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
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

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
	resp, err := h.svc.Approve(r.Context(), r.PathValue("id"), req.ActorID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}

// PublishAnnouncementRequest handles
// POST /announcement-requests/{id}/publish.
func (h *AnnouncementRequestHandler) PublishAnnouncementRequest(w http.ResponseWriter, r *http.Request) {
	var req domain.AnnouncementRequestActorRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.MarkPublished(r.Context(), r.PathValue("id"), req.ActorID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAnnouncementRequestJSON(w, http.StatusOK, resp)
}
