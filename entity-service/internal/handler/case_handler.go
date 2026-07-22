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

// Package handler is declared in user_handler.go.
package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// safeAttachmentTypes is the allowlist of Content-Type values that may be
// forwarded as-is. Anything not in this set is coerced to application/octet-stream
// to prevent a stored-XSS attack via a crafted upstream Content-Type (e.g. text/html).
// All responses also carry Content-Disposition: attachment.
var safeAttachmentTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"application/pdf":          true,
	"text/plain":               true,
	"application/zip":          true,
	"application/x-zip-compressed": true,
	"application/msword":       true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
}

// CaseHandler handles HTTP requests for the case resource.
type CaseHandler struct {
	svc service.CaseService
}

// NewCaseHandler constructs a CaseHandler with the given service.
func NewCaseHandler(svc service.CaseService) *CaseHandler {
	return &CaseHandler{svc: svc}
}

// GetCase handles GET /cases/{id}.
func (h *CaseHandler) GetCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.svc.GetCaseByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

// CreateCase handles POST /cases.
func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateCaseRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	c, err := h.svc.CreateCase(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(c)
}

// PatchCase handles PATCH /cases/{id}.
// Accepts stateKey, severityKey, workStateKey (both data sources), or watchList, assigneeEmail (ServiceNow only).
// Exactly one field must be provided per request.
func (h *CaseHandler) PatchCase(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateCaseRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("id")
	resp, err := h.svc.UpdateCase(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateCaseComment handles POST /cases/{id}/comments.
func (h *CaseHandler) CreateCaseComment(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateCaseCommentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = r.PathValue("id")
	resp, err := h.svc.CreateCaseComment(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCaseComments handles POST /cases/{id}/comments/search.
func (h *CaseHandler) SearchCaseComments(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCaseCommentsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = r.PathValue("id")
	resp, err := h.svc.SearchCaseComments(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCaseActivities handles POST /cases/{id}/activities/search.
func (h *CaseHandler) SearchCaseActivities(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCaseActivitiesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = r.PathValue("id")
	resp, err := h.svc.SearchCaseActivities(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCases handles POST /cases/search.
func (h *CaseHandler) SearchCases(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCasesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchCases(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateCaseAttachment handles POST /attachments.
func (h *CaseHandler) CreateCaseAttachment(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateAttachmentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CreateCaseAttachment(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCaseAttachments handles POST /attachments/search.
func (h *CaseHandler) SearchCaseAttachments(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchAttachmentsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchCaseAttachments(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetCaseAttachmentContent handles GET /attachments/{id}/content.
func (h *CaseHandler) GetCaseAttachmentContent(w http.ResponseWriter, r *http.Request) {
	attachmentID := r.PathValue("id")
	content, contentType, err := h.svc.GetCaseAttachmentContent(r.Context(), attachmentID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	// Strip Content-Type parameters (e.g. charset) before the allowlist check.
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if !safeAttachmentTypes[ct] {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "attachment")
	_, _ = w.Write(content) // #nosec G705 -- Content-Type is allowlisted above; Content-Disposition: attachment prevents inline rendering
}

// DeleteCaseAttachment handles DELETE /attachments/{id}.
func (h *CaseHandler) DeleteCaseAttachment(w http.ResponseWriter, r *http.Request) {
	req := domain.DeleteAttachmentRequest{
		AttachmentID: r.PathValue("id"),
	}
	resp, err := h.svc.DeleteCaseAttachment(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
