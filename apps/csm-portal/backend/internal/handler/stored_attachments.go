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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// storedAttachmentEntityClient is the entity-service surface the stored
// attachment routes need. The entity service owns every access decision (the
// caller's own row-level security), so this handler adds no case lookups.
type storedAttachmentEntityClient interface {
	StoredAttachmentLinks(ctx context.Context, body []byte) ([]byte, error)
	InitiateStoredAttachmentUpload(ctx context.Context, body []byte) ([]byte, error)
	ConfirmStoredAttachmentUpload(ctx context.Context, id string, body []byte) ([]byte, error)
	DeleteStoredAttachment(ctx context.Context, body []byte) error
	OpenInlineImage(ctx context.Context, id string) (*entity.InlineImageStream, error)
}

// StoredAttachmentHandler forwards the entity service's stored-attachment routes
// (files held in SFTPGo). Uploads and downloads hand the browser a short-lived
// link; a comment's inline image is streamed through this backend instead, so
// the browser never talks to SFTPGo for a preview.
type StoredAttachmentHandler struct {
	entity storedAttachmentEntityClient
}

// NewStoredAttachmentHandler constructs a StoredAttachmentHandler.
func NewStoredAttachmentHandler(e storedAttachmentEntityClient) *StoredAttachmentHandler {
	return &StoredAttachmentHandler{entity: e}
}

// storedInlineImageTypes is the only set of types a streamed inline image is
// sent as; anything else is neutralised to application/octet-stream.
var storedInlineImageTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
}

// mapStoredAttachmentError maps an entity-service failure for these routes. The
// entity service's refusals here are written for the person using the portal
// ("This case is closed, so attachments can't be added", "Only the person who
// uploaded an attachment can delete it", "too many attachment links requested"),
// so 400, 403, 409 and 429 carry its message; anything internal is generic.
func mapStoredAttachmentError(w http.ResponseWriter, err error, fallback string) {
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		writeError(w, http.StatusInternalServerError, fallback)
		return
	}
	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
	case http.StatusNotFound:
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
	case http.StatusBadRequest, http.StatusForbidden, http.StatusConflict, http.StatusTooManyRequests:
		writeError(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallback))
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		writeError(w, http.StatusServiceUnavailable, fallback)
	default:
		writeError(w, http.StatusInternalServerError, fallback)
	}
}

// readStoredAttachmentBody authenticates the caller and reads the capped body.
func readStoredAttachmentBody(w http.ResponseWriter, r *http.Request) (*middleware.UserInfo, []byte, bool) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
		} else {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		}
		return nil, nil, false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, nil, false
	}
	return user, body, true
}

// ResolveLinks handles POST /attachments/download.
func (h *StoredAttachmentHandler) ResolveLinks(w http.ResponseWriter, r *http.Request) {
	user, body, ok := readStoredAttachmentBody(w, r)
	if !ok {
		return
	}
	out, err := h.entity.StoredAttachmentLinks(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity StoredAttachmentLinks failed", "userID", user.UserID, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to get attachment links.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// InitiateUpload handles POST /attachments/upload.
func (h *StoredAttachmentHandler) InitiateUpload(w http.ResponseWriter, r *http.Request) {
	user, body, ok := readStoredAttachmentBody(w, r)
	if !ok {
		return
	}
	out, err := h.entity.InitiateStoredAttachmentUpload(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity InitiateStoredAttachmentUpload failed", "userID", user.UserID, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to start the attachment upload.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, out)
}

// ConfirmUpload handles POST /attachments/upload/{id}/confirm.
func (h *StoredAttachmentHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		if middleware.UserInfoFromContext(r.Context()) == nil {
			writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
			return
		}
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	user, body, ok := readStoredAttachmentBody(w, r)
	if !ok {
		return
	}
	out, err := h.entity.ConfirmStoredAttachmentUpload(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ConfirmStoredAttachmentUpload failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to confirm the attachment upload.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// DeleteAttachment handles POST /attachments/delete.
func (h *StoredAttachmentHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	user, body, ok := readStoredAttachmentBody(w, r)
	if !ok {
		return
	}
	if err := h.entity.DeleteStoredAttachment(r.Context(), body); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteStoredAttachment failed", "userID", user.UserID, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to delete the attachment.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// InlineImageContent handles GET /attachments/inline/{id}/content: it streams a
// comment's inline image from the entity service, which has already checked that
// the caller may see it. Nothing is buffered, and no link that could be shared
// with someone without access ever exists.
func (h *StoredAttachmentHandler) InlineImageContent(w http.ResponseWriter, r *http.Request) {
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
	img, err := h.entity.OpenInlineImage(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity OpenInlineImage failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to retrieve the image.")
		return
	}
	defer img.Body.Close()

	// Strip parameters (e.g. charset) before the allowlist check.
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(img.ContentType, ";", 2)[0]))
	if !storedInlineImageTypes[ct] {
		ct = "application/octet-stream"
	}
	hd := w.Header()
	hd.Set("Content-Type", ct)
	if img.Length >= 0 {
		hd.Set("Content-Length", strconv.FormatInt(img.Length, 10))
	}
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hd.Set("Content-Disposition", "inline")
	hd.Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, img.Body) // #nosec G705 -- type is allowlisted above; nosniff and a sandboxing CSP are set
}
