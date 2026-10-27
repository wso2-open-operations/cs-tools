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
	"io"
	"net/http"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// StoredAttachmentHandler handles HTTP requests for the links a browser uses to
// download and upload attachment files held in SFTPGo.
type StoredAttachmentHandler struct {
	svc service.StoredAttachmentService
}

// NewStoredAttachmentHandler constructs a StoredAttachmentHandler.
func NewStoredAttachmentHandler(svc service.StoredAttachmentService) *StoredAttachmentHandler {
	return &StoredAttachmentHandler{svc: svc}
}

// ResolveLinks handles POST /attachments/download.
func (h *StoredAttachmentHandler) ResolveLinks(w http.ResponseWriter, r *http.Request) {
	var req domain.AttachmentLinksRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.ResolveLinks(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, resp)
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

// InlineImageContent handles GET /attachments/inline/{id}/content: it streams a
// comment's inline image after the service has checked the caller may see it.
func (h *StoredAttachmentHandler) InlineImageContent(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.OpenInlineImage(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	defer c.Body.Close()
	hd := w.Header()
	hd.Set("Content-Type", c.ContentType)
	if c.Size >= 0 {
		hd.Set("Content-Length", strconv.FormatInt(c.Size, 10))
	}
	// Shown in the page, never run: the type is one of the four image types, and
	// nosniff plus a sandboxing policy stop anything else being treated as a page.
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hd.Set("Content-Disposition", "inline")
	// The image is private to this caller; a short browser cache keeps an activity
	// feed from fetching it again on every render.
	hd.Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, c.Body)
}

// writeStoredAttachmentJSON writes a response that carries short-lived links, so
// it must never be cached by a browser or an intermediary.
func writeStoredAttachmentJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
