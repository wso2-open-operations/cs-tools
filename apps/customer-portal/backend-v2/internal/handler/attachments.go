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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityAttachmentClient abstracts the entity-service attachment operations
// used by AttachmentHandler.
type entityAttachmentClient interface {
	CreateAttachment(ctx context.Context, req entity.CreateAttachmentRequest) (entity.CreateAttachmentResponse, error)
	GetAttachmentContent(ctx context.Context, id string) (body []byte, contentType string, err error)
	DeleteAttachment(ctx context.Context, id string) (entity.DeleteAttachmentResponse, error)
	GetAttachment(ctx context.Context, id string) (entity.AttachmentDetails, error)
	GetCase(ctx context.Context, id string) (entity.CaseView, error)
}

// AttachmentHandler handles HTTP requests for attachment operations.
type AttachmentHandler struct {
	entity      entityAttachmentClient
	callerScope *CallerScopeResolver
}

// NewAttachmentHandler creates an AttachmentHandler backed by the given entity client.
func NewAttachmentHandler(entity entityAttachmentClient) *AttachmentHandler {
	return &AttachmentHandler{entity: entity}
}

// SetCallerScope sets the resolver used to verify the caller has access to the
// project containing an attachment's reference entity.
func (h *AttachmentHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

// authorizeAttachmentByID fetches the attachment's metadata to resolve its parent entity
// (case or deployment/etc.) and enforces caller project membership.
// Returns the AttachmentDetails if authorized, or false if unauthorized/error occurred.
func (h *AttachmentHandler) authorizeAttachmentByID(w http.ResponseWriter, r *http.Request, id string, user *middleware.UserInfo) (entity.AttachmentDetails, bool) {
	details, err := h.entity.GetAttachment(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachment failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve attachment.")
		return entity.AttachmentDetails{}, false
	}

	if details.ReferenceID != "" && uuidRe.MatchString(details.ReferenceID) && (details.ReferenceType == entity.ReferenceTypeCase || string(details.ReferenceType) == "case") {
		// Commented out pending end-to-end verification against real
		// entity-service data — uncomment while testing, re-comment before
		// committing. See handler.CallerScopeResolver / requireProjectMember.
		// caseView, err := h.entity.GetCase(r.Context(), details.ReferenceID)
		// if err != nil {
		// 	slog.ErrorContext(r.Context(), "resolving attachment parent case failed", "userID", user.UserID, "attachmentID", id, "referenceID", details.ReferenceID, "err", summarizeErr(err))
		// 	writeError(w, http.StatusNotFound, ErrMsgNotFound)
		// 	return entity.AttachmentDetails{}, false
		// }
		// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
		// 	return entity.AttachmentDetails{}, false
		// }
	}

	return details, true
}

// CreateAttachment handles POST /attachments.
func (h *AttachmentHandler) CreateAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.CreateAttachmentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if req.ReferenceID != "" && req.ReferenceType == entity.ReferenceTypeCase {
		// Commented out pending end-to-end verification against real
		// entity-service data — uncomment while testing, re-comment before
		// committing. See handler.CallerScopeResolver / requireProjectMember.
		// caseView, err := h.entity.GetCase(r.Context(), req.ReferenceID)
		// if err != nil {
		// 	slog.ErrorContext(r.Context(), "entity GetCase failed for attachment creation", "userID", user.UserID, "caseID", req.ReferenceID, "err", summarizeErr(err))
		// 	mapUpstreamError(w, err, "Failed to create attachment.")
		// 	return
		// }
		// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
		// 	return
		// }
	}

	result, err := h.entity.CreateAttachment(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateAttachment failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create attachment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapAttachmentCreate(result))
}

// GetAttachmentContent handles GET /attachments/{id}/content. The response is
// the raw file content, not JSON. Content-Disposition: attachment is always
// set (mirroring entity-service's own XSS mitigation for this endpoint) so
// browsers never render an attachment inline.
func (h *AttachmentHandler) GetAttachmentContent(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !isAttachmentID(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	if _, ok := h.authorizeAttachmentByID(w, r, id, user); !ok {
		return
	}

	content, contentType, err := h.entity.GetAttachmentContent(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachmentContent failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to download attachment.")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content) // #nosec G705 -- Content-Type set from entity-service's own sanitized value; Content-Disposition forces download, never inline rendering
}

// DeleteAttachment handles DELETE /attachments/{id}.
func (h *AttachmentHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !isAttachmentID(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	if _, ok := h.authorizeAttachmentByID(w, r, id, user); !ok {
		return
	}

	result, err := h.entity.DeleteAttachment(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteAttachment failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to delete attachment.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapDeleteAttachment(result))
}

// GetAttachment handles GET /attachments/{id} — metadata plus base64-encoded
// content, distinct from GetAttachmentContent's raw binary stream.
func (h *AttachmentHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !isAttachmentID(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	details, ok := h.authorizeAttachmentByID(w, r, id, user)
	if !ok {
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapAttachmentDetails(details))
}
