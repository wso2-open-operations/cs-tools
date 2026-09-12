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
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// maxAttachmentBodySize caps the CreateCaseAttachment JSON body at 15 MiB,
// matching the csm-portal backend's own maxAttachmentBodyBytes ceiling: a 10
// MB file becomes ~13.3 MB once base64-encoded, plus JSON envelope overhead.
// The generic maxRequestBodySize (1 MiB) is far too small for this endpoint
// and rejects legitimate small attachments (e.g. a 2 MB file).
const maxAttachmentBodySize = int64(15 << 20)

// maxTagSearchLimit caps POST /tags/search results. Tag search backs a
// type-ahead, not a paged browse, so the ceiling is deliberately low.
const maxTagSearchLimit = 100

// attachmentTooLargeMsg is deliberately expressed as the file-size limit (10
// MB) a caller actually cares about, not the raw request-body cap above — the
// extra headroom in maxAttachmentBodySize exists only to absorb base64/JSON
// overhead the caller never sees.
const attachmentTooLargeMsg = "attachment exceeds the maximum allowed size of 10 MB"

// safeAttachmentTypes is the allowlist of Content-Type values that may be
// forwarded as-is. Anything not in this set is coerced to application/octet-stream
// to prevent a stored-XSS attack via a crafted upstream Content-Type (e.g. text/html).
// All responses also carry Content-Disposition: attachment.
var safeAttachmentTypes = map[string]bool{
	"image/png":                    true,
	"image/jpeg":                   true,
	"image/gif":                    true,
	"image/webp":                   true,
	"application/pdf":              true,
	"text/plain":                   true,
	"application/zip":              true,
	"application/x-zip-compressed": true,
	"application/msword":           true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
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

// AggregateCases handles POST /cases/aggregate.
func (h *CaseHandler) AggregateCases(w http.ResponseWriter, r *http.Request) {
	var req domain.AggregateCasesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.AggregateCases(r.Context(), req)
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
	if !decodeRequestWithLimit(w, r, &req, maxAttachmentBodySize, attachmentTooLargeMsg) {
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

// ConfirmCaseAttachment handles POST /attachments/{id}/confirm.
func (h *CaseHandler) ConfirmCaseAttachment(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ConfirmCaseAttachment(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
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

// GetAttachmentByID handles GET /attachments/{id}.
func (h *CaseHandler) GetAttachmentByID(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetAttachmentByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// UpdateAttachment handles PATCH /attachments/{id}.
func (h *CaseHandler) UpdateAttachment(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateAttachmentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.AttachmentID = r.PathValue("id")
	resp, err := h.svc.UpdateAttachment(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetCaseFeedback handles GET /cases/{id}/feedback.
func (h *CaseHandler) GetCaseFeedback(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetCaseFeedback(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SubmitCaseFeedback handles POST /cases/{id}/feedback.
func (h *CaseHandler) SubmitCaseFeedback(w http.ResponseWriter, r *http.Request) {
	var req domain.SubmitCaseFeedbackRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SubmitCaseFeedback(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// AddCaseTag handles POST /cases/{id}/tags.
func (h *CaseHandler) AddCaseTag(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")

	var req domain.AddCaseTagRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = caseID

	tag, err := h.svc.AddCaseTag(r.Context(), caseID, req.Label)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(tag)
}

// RemoveCaseTag handles DELETE /cases/{id}/tags/{tagId}.
func (h *CaseHandler) RemoveCaseTag(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")
	tagID := r.PathValue("tagId")

	if err := h.svc.RemoveCaseTag(r.Context(), caseID, tagID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SearchTags handles POST /tags/search. Not scoped to a single case; used
// for FE autocomplete when attaching a tag.
func (h *CaseHandler) SearchTags(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchTagsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	h.searchTags(w, r, req)
}

// SearchTagsQuery handles GET /tags/search?q={query}&limit={limit}, the
// query-parameter form tag search used before it moved to a JSON body.
//
// Deprecated: use POST /tags/search instead. This alias exists only so the
// services in front of this one can be rolled out over the following release
// instead of having to deploy in lockstep with it; a caller still on the GET
// would otherwise get a 405. Delete it once that release has shipped.
func (h *CaseHandler) SearchTagsQuery(w http.ResponseWriter, r *http.Request) {
	req := domain.SearchTagsRequest{
		Filters: domain.SearchTagsFilters{SearchQuery: r.URL.Query().Get("q")},
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeServiceError(w, r, &apierror.ValidationError{Msg: "limit must be a non-negative integer"})
			return
		}
		req.Limit = parsed
	}
	h.searchTags(w, r, req)
}

// searchTags is the single path both /tags/search forms run through: validation,
// the service call, and the response envelope all live here so the deprecated
// GET alias cannot drift from the POST.
func (h *CaseHandler) searchTags(w http.ResponseWriter, r *http.Request, req domain.SearchTagsRequest) {
	if req.Limit < 0 {
		writeServiceError(w, r, &apierror.ValidationError{Msg: "limit must be a non-negative integer"})
		return
	}
	if req.Limit > maxTagSearchLimit {
		writeServiceError(w, r, &apierror.ValidationError{Msg: "limit must not exceed 100"})
		return
	}

	tags, err := h.svc.SearchTags(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string][]domain.Tag{"tags": tags})
}
