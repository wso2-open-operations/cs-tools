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
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// maxStoredAttachmentBodyBytes bounds the JSON bodies of the stored-attachment
// routes: they carry ids and metadata, never file content (the file goes from
// the browser straight to SFTPGo).
const maxStoredAttachmentBodyBytes = 64 << 10

// storedAttachmentEntityClient is the entity-service surface of the SFTPGo-backed
// attachment routes. Bodies are relayed as-is; entity-service validates them
// and decides who may see what.
type storedAttachmentEntityClient interface {
	ResolveStoredAttachmentLinks(ctx context.Context, body json.RawMessage) ([]byte, error)
	InitiateStoredAttachmentUpload(ctx context.Context, body json.RawMessage) ([]byte, error)
	ConfirmStoredAttachmentUpload(ctx context.Context, id string, body json.RawMessage) ([]byte, error)
	DeleteStoredAttachment(ctx context.Context, body json.RawMessage) error
	StreamInlineImage(ctx context.Context, id string) (body io.ReadCloser, contentType string, size int64, err error)
}

// StoredAttachmentPermit checks the caller's permission for a module and action,
// writing the refusal itself and returning false when it is denied.
type StoredAttachmentPermit func(w http.ResponseWriter, r *http.Request, module middleware.Module, action middleware.Action) bool

// StoredAttachmentHandler relays entity-service's SFTPGo-backed attachment
// routes. Access to the attachment itself is entity-service's (row-level
// security); this handler only applies the portal's module permission, which
// depends on the attachment type, and shapes errors the way this backend does.
type StoredAttachmentHandler struct {
	entity storedAttachmentEntityClient
	permit StoredAttachmentPermit
}

// NewStoredAttachmentHandler creates a StoredAttachmentHandler. permit is
// normally built on middleware.CheckPermission.
func NewStoredAttachmentHandler(entity storedAttachmentEntityClient, permit StoredAttachmentPermit) *StoredAttachmentHandler {
	return &StoredAttachmentHandler{entity: entity, permit: permit}
}

// moduleForStoredAttachmentType maps the request's attachment type to the
// permission module that governs it: a deployment's attachments follow the
// deployments module, a work item's and a comment's follow cases.
func moduleForStoredAttachmentType(t string) (middleware.Module, bool) {
	switch t {
	case "work_item", "comment":
		return middleware.ModuleCases, true
	case "deployment":
		return middleware.ModuleDeployments, true
	}
	return "", false
}

// mapStoredAttachmentError is mapUpstreamError plus the one status this API
// adds: entity-service answers 429 when a caller asks for too many links, and it
// is meant to reach the caller.
func mapStoredAttachmentError(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
		msg := apiErr.Body
		if msg == "" {
			msg = "Too many requests. Please try again shortly."
		}
		writeErrorCode(w, http.StatusTooManyRequests, msg, apiErr.Code)
		return
	}
	mapUpstreamError(w, err, fallbackMsg)
}

// relay reads the JSON body, checks the module permission for its type and
// returns the raw body for forwarding. ok is false when a response was written.
func (h *StoredAttachmentHandler) relay(w http.ResponseWriter, r *http.Request, action middleware.Action) (json.RawMessage, bool) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, false
	}
	body, ok := readJSONBodyWithLimit(w, r, maxStoredAttachmentBodyBytes)
	if !ok {
		return nil, false
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	module, ok := moduleForStoredAttachmentType(probe.Type)
	if !ok {
		writeError(w, http.StatusBadRequest, "type must be one of work_item, comment, deployment")
		return nil, false
	}
	if !h.permit(w, r, module, action) {
		return nil, false
	}
	return body, true
}

// ResolveLinks handles POST /attachments/download.
func (h *StoredAttachmentHandler) ResolveLinks(w http.ResponseWriter, r *http.Request) {
	body, ok := h.relay(w, r, middleware.ActionRead)
	if !ok {
		return
	}
	out, err := h.entity.ResolveStoredAttachmentLinks(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ResolveStoredAttachmentLinks failed", "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to get attachment links.")
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, out)
}

// InitiateUpload handles POST /attachments/upload.
func (h *StoredAttachmentHandler) InitiateUpload(w http.ResponseWriter, r *http.Request) {
	body, ok := h.relay(w, r, middleware.ActionCreate)
	if !ok {
		return
	}
	out, err := h.entity.InitiateStoredAttachmentUpload(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity InitiateStoredAttachmentUpload failed", "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to start the upload.")
		return
	}
	writeStoredAttachmentJSON(w, http.StatusCreated, out)
}

// ConfirmUpload handles POST /attachments/upload/{id}/confirm.
func (h *StoredAttachmentHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	body, ok := h.relay(w, r, middleware.ActionCreate)
	if !ok {
		return
	}
	out, err := h.entity.ConfirmStoredAttachmentUpload(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ConfirmStoredAttachmentUpload failed", "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to confirm the upload.")
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, out)
}

// DeleteAttachment handles POST /attachments/delete.
func (h *StoredAttachmentHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	body, ok := h.relay(w, r, middleware.ActionDelete)
	if !ok {
		return
	}
	if err := h.entity.DeleteStoredAttachment(r.Context(), body); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteStoredAttachment failed", "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to delete attachment.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// inlineImageTypes are the only types an inline comment image is ever sent as.
var inlineImageTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
}

// GetInlineImage handles GET /attachments/inline/{id}/content: it streams a
// comment's inline image from entity-service, which checks access on every
// request, so the browser never reaches SFTPGo and no link exists to forward.
// The body is copied through, never buffered.
func (h *StoredAttachmentHandler) GetInlineImage(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	if !h.permit(w, r, middleware.ModuleCases, middleware.ActionRead) {
		return
	}
	body, contentType, size, err := h.entity.StreamInlineImage(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity StreamInlineImage failed", "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to load the image.")
		return
	}
	defer body.Close()

	// Entity-service sends the stored image type; anything else is neutralised
	// rather than echoed, so a response can never be treated as a page.
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if !inlineImageTypes[ct] {
		ct = "application/octet-stream"
	}
	hd := w.Header()
	hd.Set("Content-Type", ct)
	if size >= 0 {
		hd.Set("Content-Length", strconv.FormatInt(size, 10))
	}
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hd.Set("Content-Disposition", "inline")
	hd.Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body) // #nosec G705 -- type is one of four image types or octet-stream, with nosniff and a sandboxing CSP
}

// writeStoredAttachmentJSON relays entity-service's JSON, which carries
// short-lived links, so it must never be cached.
func writeStoredAttachmentJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body) // #nosec G705 -- Content-Type: application/json set; SecurityHeaders adds nosniff
}
