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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// StoredAttachmentHandler handles HTTP requests that record and authorize the
// attachment files the portal backends hold in Azure Blob Storage.
type StoredAttachmentHandler struct {
	svc     service.StoredAttachmentService
	uploads service.AttachmentUploadCleanupService
}

// NewStoredAttachmentHandler constructs a StoredAttachmentHandler.
func NewStoredAttachmentHandler(svc service.StoredAttachmentService) *StoredAttachmentHandler {
	return &StoredAttachmentHandler{svc: svc}
}

// WithUploadCleanup attaches the abandoned-upload cleanup, which backs POST /attachments/uploads/cleanup.
func (h *StoredAttachmentHandler) WithUploadCleanup(c service.AttachmentUploadCleanupService) *StoredAttachmentHandler {
	h.uploads = c
	return h
}

// CleanupUploads handles POST /attachments/uploads/cleanup.
func (h *StoredAttachmentHandler) CleanupUploads(w http.ResponseWriter, r *http.Request) {
	if h.uploads == nil {
		http.NotFound(w, r)
		return
	}
	res, err := h.uploads.CleanupUploads(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, res)
}

// AcknowledgeOrphans handles POST /attachments/uploads/cleanup/acknowledge.
func (h *StoredAttachmentHandler) AcknowledgeOrphans(w http.ResponseWriter, r *http.Request) {
	if h.uploads == nil {
		http.NotFound(w, r)
		return
	}
	var req domain.AcknowledgeOrphansRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	res, err := h.uploads.AcknowledgeOrphans(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, res)
}

// InitiateUpload handles POST /attachments/upload.
func (h *StoredAttachmentHandler) InitiateUpload(w http.ResponseWriter, r *http.Request) {
	var req domain.InitiateAttachmentUploadRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.InitiateUpload(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStoredAttachmentJSON(w, http.StatusCreated, resp)
}

// ConfirmUpload handles POST /attachments/upload/{id}/confirm.
func (h *StoredAttachmentHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	var req domain.ConfirmAttachmentUploadRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("id")
	resp, err := h.svc.ConfirmUpload(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, resp)
}

// DeleteAttachment handles POST /attachments/delete.
func (h *StoredAttachmentHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	var req domain.DeleteStoredAttachmentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if err := h.svc.DeleteAttachment(r.Context(), req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// AuthorizeUpload handles POST /attachments/upload/{id}/authorize. The response
// carries an internal storage key for the calling portal backend.
func (h *StoredAttachmentHandler) AuthorizeUpload(w http.ResponseWriter, r *http.Request) {
	var req domain.AuthorizeAttachmentUploadRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("id")
	resp, err := h.svc.AuthorizeUpload(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, resp)
}

// LocateAttachment handles POST /attachments/locate. The response carries an
// internal storage key for the calling portal backend.
func (h *StoredAttachmentHandler) LocateAttachment(w http.ResponseWriter, r *http.Request) {
	var req domain.LocateAttachmentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.LocateAttachment(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, resp)
}

// InlineImageLocation handles GET /attachments/inline/{id}/location: where a
// comment's inline image is stored, after the service has checked the caller may
// see it.
func (h *StoredAttachmentHandler) InlineImageLocation(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.LocateInlineImage(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, resp)
}

// InlineImageRelocated handles POST /attachments/inline/{id}/relocated: the portal
// backend moved the file of a description image to the key LocateInlineImage named.
func (h *StoredAttachmentHandler) InlineImageRelocated(w http.ResponseWriter, r *http.Request) {
	var req domain.InlineImageRelocatedRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if err := h.svc.InlineImageRelocated(r.Context(), r.PathValue("id"), req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// writeStoredAttachmentJSON writes a response that can carry an internal storage
// key, so it must never be cached by a browser or an intermediary.
func writeStoredAttachmentJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
