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
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// ChunkBytes is the size of every upload chunk but the last. The browser sends
// 8 MiB at a time, which stays well under the gateway's 50 MB request limit.
const ChunkBytes = 8 << 20

// attachmentChunkDeadline is how long one chunk may take to arrive and be stored,
// beyond the server's global read and write timeouts: 8 MiB over a slow link.
const attachmentChunkDeadline = 10 * time.Minute

// attachmentStreamWriteDeadline is how long a file download may take to write,
// beyond the server's global WriteTimeout: 100 MiB over a slow link.
const attachmentStreamWriteDeadline = 30 * time.Minute

// blobEntityClient is the entity-service surface of the file routes. Entity-service
// is the only service that talks to Azure Blob Storage: it checks the caller's
// access and moves the bytes, so this handler only forwards them.
type blobEntityClient interface {
	PutStoredAttachmentChunk(ctx context.Context, id, kind string, index int, size int64, body io.Reader) error
	ListStoredAttachmentChunks(ctx context.Context, id, kind string) ([]byte, error)
	StoredAttachmentLimits(ctx context.Context) ([]byte, error)
	DirectStoredAttachmentUploadLinks(ctx context.Context, id, kind string) ([]byte, error)
	DirectStoredAttachmentDownloadLink(ctx context.Context, id, kind string) ([]byte, error)
	ConfirmStoredAttachmentUpload(ctx context.Context, id string, body []byte) ([]byte, error)
	DownloadStoredAttachment(ctx context.Context, kind, id, rangeHeader string) (*entity.FileResponse, error)
	InlineImageContent(ctx context.Context, id string) (*entity.FileResponse, error)
	DeleteStoredAttachment(ctx context.Context, body []byte) error
}

// BlobAttachmentHandler serves attachment files. It holds no storage credential and
// no storage key: the browser's bytes are forwarded to entity-service, which checks
// who may upload or read (row-level security) and keeps the files in Azure Blob
// Storage, and its answer is streamed back. The route's own permission is applied
// at registration (PermWrite to upload, PermDownloadAttachment to read). There is no
// link to leak: every read goes through here with the caller's own token.
type BlobAttachmentHandler struct {
	entity blobEntityClient
	// access decides whether the caller is an admin, who may delete an
	// attachment someone else uploaded. Nil means nobody is.
	access *AccessGuard
}

// WithAccessGuard lets a portal admin delete an attachment they did not upload.
func (h *BlobAttachmentHandler) WithAccessGuard(g *AccessGuard) *BlobAttachmentHandler {
	h.access = g
	return h
}

// NewBlobAttachmentHandler creates a BlobAttachmentHandler.
func NewBlobAttachmentHandler(entity blobEntityClient) *BlobAttachmentHandler {
	return &BlobAttachmentHandler{entity: entity}
}

// validStoredAttachmentType reports whether t is a kind entity-service stores.
func validStoredAttachmentType(t string) bool {
	return t == "work_item" || t == "comment" || t == "deployment"
}

// typeFromQuery authenticates the caller and reads the ?type= of a route whose
// body is a file.
func (h *BlobAttachmentHandler) typeFromQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return "", false
	}
	t := r.URL.Query().Get("type")
	if !validStoredAttachmentType(t) {
		writeError(w, http.StatusBadRequest, "type must be one of work_item, comment, deployment")
		return "", false
	}
	return t, true
}

// PutChunk handles PUT /attachments/upload/{id}/chunks/{index}?type=.
//
// Chunks may be sent again (a retried request overwrites the same block), in any
// order, and a few at a time. Nothing is visible until the upload is confirmed.
func (h *BlobAttachmentHandler) PutChunk(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		writeError(w, http.StatusBadRequest, "Invalid chunk index.")
		return
	}
	kind, ok := h.typeFromQuery(w, r)
	if !ok {
		return
	}
	size := r.ContentLength
	if size <= 0 {
		writeError(w, http.StatusLengthRequired, "A chunk must declare its length.")
		return
	}
	if size > ChunkBytes {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("A chunk may be at most %d bytes.", ChunkBytes))
		return
	}
	// A slow link can need longer than the server's global timeouts for one
	// chunk. Best effort: a ResponseWriter that cannot extend them keeps them.
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(attachmentChunkDeadline)
	_ = rc.SetReadDeadline(deadline)
	_ = rc.SetWriteDeadline(deadline)
	ctx, cancel := context.WithTimeout(r.Context(), attachmentChunkDeadline)
	defer cancel()
	body := http.MaxBytesReader(w, r.Body, ChunkBytes)
	if err := h.entity.PutStoredAttachmentChunk(ctx, id, kind, index, size, body); err != nil {
		slog.ErrorContext(ctx, "entity PutStoredAttachmentChunk failed", "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to store the chunk.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// ListChunks handles GET /attachments/upload/{id}/chunks?type=: the chunks
// already stored, so an interrupted upload can carry on where it stopped.
func (h *BlobAttachmentHandler) ListChunks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	kind, ok := h.typeFromQuery(w, r)
	if !ok {
		return
	}
	out, err := h.entity.ListStoredAttachmentChunks(r.Context(), id, kind)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListStoredAttachmentChunks failed", "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to list the stored chunks.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// Limits handles GET /attachments/limits: the attachment size limits the upload
// dialogs show. They are the same for everyone, so the browser may keep them for
// the five minutes entity-service allows.
func (h *BlobAttachmentHandler) Limits(w http.ResponseWriter, r *http.Request) {
	out, err := h.entity.StoredAttachmentLimits(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity StoredAttachmentLimits failed", "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to read the attachment limits.")
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	writeJSON(w, http.StatusOK, out)
}

// DirectUploadLinks handles POST /attachments/{id}/upload-links?type=: one signed
// link per chunk, so a large file goes from the browser straight to Azure. The
// links are credentials, so the answer is never cached or logged.
func (h *BlobAttachmentHandler) DirectUploadLinks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	kind, ok := h.typeFromQuery(w, r)
	if !ok {
		return
	}
	out, err := h.entity.DirectStoredAttachmentUploadLinks(r.Context(), id, kind)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DirectStoredAttachmentUploadLinks failed", "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to prepare the upload.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// DirectDownloadLink handles POST /attachments/{id}/download-links?type=: a signed
// link the browser downloads the file from, straight from Azure. The link is a
// credential, so the answer is never cached or logged.
func (h *BlobAttachmentHandler) DirectDownloadLink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	kind, ok := h.typeFromQuery(w, r)
	if !ok {
		return
	}
	out, err := h.entity.DirectStoredAttachmentDownloadLink(r.Context(), id, kind)
	if err != nil {
		// A 404 is the expected answer while direct transfers are off (the webapp asks
		// before every large download and then streams it): not an error to log.
		var apiErr *apierror.Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			slog.ErrorContext(r.Context(), "entity DirectStoredAttachmentDownloadLink failed", "attachmentID", id, "err", summarizeErr(err))
		}
		mapStoredAttachmentError(w, err, "Failed to prepare the download.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// ConfirmUpload handles POST /attachments/upload/{id}/confirm. The body is
// {"type": ..., "chunks": N}: entity-service assembles the N stored chunks into
// the file, checks what was stored against what the upload declared, and removes
// a file it refuses.
func (h *BlobAttachmentHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		if middleware.UserInfoFromContext(r.Context()) == nil {
			writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
			return
		}
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	user, raw, ok := readStoredAttachmentBody(w, r)
	if !ok {
		return
	}
	var in struct {
		Type   string `json:"type"`
		Chunks int    `json:"chunks"`
	}
	if err := json.Unmarshal(raw, &in); err != nil || !validStoredAttachmentType(in.Type) {
		writeError(w, http.StatusBadRequest, "type must be one of work_item, comment, deployment")
		return
	}
	// Only the two fields this handler checked are forwarded.
	body, _ := json.Marshal(map[string]any{"type": in.Type, "chunks": in.Chunks})
	out, err := h.entity.ConfirmStoredAttachmentUpload(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ConfirmStoredAttachmentUpload failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to confirm the attachment upload.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// Download handles GET /attachments/download/{id}/content?type=: it streams the
// attachment as a download, honouring a single Range so a break can resume.
func (h *BlobAttachmentHandler) Download(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	kind, ok := h.typeFromQuery(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), attachmentStreamWriteDeadline)
	defer cancel()
	f, err := h.entity.DownloadStoredAttachment(ctx, kind, id, r.Header.Get("Range"))
	if err != nil {
		h.fileError(w, r, "DownloadStoredAttachment", id, err, "Failed to get the attachment.")
		return
	}
	defer f.Body.Close()
	h.stream(w, f)
}

// InlineImage handles GET /attachments/inline/{id}/content for a comment image.
func (h *BlobAttachmentHandler) InlineImage(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), attachmentStreamWriteDeadline)
	defer cancel()
	f, err := h.entity.InlineImageContent(ctx, id)
	if err != nil {
		h.fileError(w, r, "InlineImageContent", id, err, "Failed to retrieve the image.")
		return
	}
	defer f.Body.Close()
	h.stream(w, f)
}

// fileError answers a file read that failed: a 416 carrying the file's size for a
// range it does not have, otherwise the ordinary mapping.
func (h *BlobAttachmentHandler) fileError(w http.ResponseWriter, r *http.Request, op, id string, err error, fallback string) {
	var rng *entity.RangeNotSatisfiableError
	if errors.As(err, &rng) {
		if rng.ContentRange != "" {
			w.Header().Set("Content-Range", rng.ContentRange)
		}
		writeError(w, http.StatusRequestedRangeNotSatisfiable, "Range not satisfiable.")
		return
	}
	slog.ErrorContext(r.Context(), "entity "+op+" failed", "attachmentID", id, "err", summarizeErr(err))
	mapStoredAttachmentError(w, err, fallback)
}

// streamedHeaders are the response headers of a file that are passed on. Entity-service
// decides how a file is presented (an opaque attachment, or one of four image types);
// nothing else it sends reaches the browser.
var streamedHeaders = []string{
	"Content-Type", "Content-Disposition", "Content-Length", "Content-Range",
	"Accept-Ranges", "Cache-Control", "X-Content-Type-Options", "Content-Security-Policy",
}

// stream copies the file to the response, never holding it in memory.
func (h *BlobAttachmentHandler) stream(w http.ResponseWriter, f *entity.FileResponse) {
	hd := w.Header()
	for _, k := range streamedHeaders {
		if v := f.Header.Get(k); v != "" {
			hd.Set(k, v)
		}
	}
	// Whatever entity-service sent, a file is never sniffed into something else.
	hd.Set("X-Content-Type-Options", "nosniff")
	if hd.Get("Content-Security-Policy") == "" {
		hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	}
	// A large file takes longer than the server's global write timeout. Best
	// effort: a ResponseWriter that cannot extend it keeps the server default.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(attachmentStreamWriteDeadline))
	w.WriteHeader(f.StatusCode)
	_, _ = io.Copy(w, f.Body) // #nosec G705 -- entity-service sends an opaque type or one of four image types; nosniff and a sandboxing CSP are always set
}

// withAdminDelete returns body with entity-service's `asAdmin` set to whether
// the caller holds PermAdmin, taken from their token roles. Whatever the browser
// sent under any spelling of that key is dropped first (entity-service's decoder
// matches keys without regard to case), so only this decision reaches it, and
// entity-service honours it only from this backend.
func (h *BlobAttachmentHandler) withAdminDelete(body []byte, roles []string) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	for key := range fields {
		if strings.EqualFold(key, "asAdmin") {
			delete(fields, key)
		}
	}
	if h.access != nil && h.access.Permits(PermAdmin, roles) {
		fields["asAdmin"] = json.RawMessage("true")
	}
	return json.Marshal(fields)
}

// DeleteAttachment handles POST /attachments/delete: entity-service removes the
// attachment if the caller may (whoever uploaded it, or a portal admin), and then
// its file.
func (h *BlobAttachmentHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	user, body, ok := readStoredAttachmentBody(w, r)
	if !ok {
		return
	}
	deleteBody, err := h.withAdminDelete(body, user.Roles)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if err := h.entity.DeleteStoredAttachment(r.Context(), deleteBody); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteStoredAttachment failed", "userID", user.UserID, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to delete the attachment.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
