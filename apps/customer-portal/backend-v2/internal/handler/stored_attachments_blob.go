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
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// ChunkBytes is the size of every upload chunk but the last. The browser sends
// 8 MiB at a time, which stays well under the gateway's 50 MB request limit.
const ChunkBytes = 8 << 20

// attachmentStreamWriteDeadline is how long a file download may take to write,
// beyond the server's global WriteTimeout: 100 MiB over a slow link.
const attachmentStreamWriteDeadline = 30 * time.Minute

// attachmentChunkDeadline is how long one chunk may take to arrive and be stored,
// beyond the server's global read and write timeouts: 8 MiB over a slow link.
const attachmentChunkDeadline = 10 * time.Minute

// blobEntityClient is the entity-service surface of the file routes. Entity-service
// is the only service that talks to Azure Blob Storage: it checks the caller's
// access and moves the bytes, so this handler only forwards them.
type blobEntityClient interface {
	PutStoredAttachmentChunk(ctx context.Context, id, kind string, index int, size int64, body io.Reader) error
	ListStoredAttachmentChunks(ctx context.Context, id, kind string) ([]byte, error)
	DownloadStoredAttachment(ctx context.Context, kind, id, rangeHeader string) (*entity.FileResponse, error)
	InlineImageContent(ctx context.Context, id string) (*entity.FileResponse, error)
}

// BlobAttachmentHandler serves attachment files. It holds no storage credential and
// no storage key: the browser's bytes are forwarded to entity-service, which checks
// who may upload or read (row-level security) and keeps the files in Azure Blob
// Storage, and its answer is streamed back. There is no link to leak: every read
// goes through here with the caller's own token.
type BlobAttachmentHandler struct {
	stored *StoredAttachmentHandler
	entity blobEntityClient
}

// NewBlobAttachmentHandler creates a BlobAttachmentHandler. stored supplies the
// permission check and the relayed routes (confirm, delete).
func NewBlobAttachmentHandler(stored *StoredAttachmentHandler, entity blobEntityClient) *BlobAttachmentHandler {
	return &BlobAttachmentHandler{stored: stored, entity: entity}
}

// typeFromQuery reads and checks the ?type= of a route whose body is a file.
func (h *BlobAttachmentHandler) typeFromQuery(w http.ResponseWriter, r *http.Request, action middleware.Action) (string, bool) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return "", false
	}
	t := r.URL.Query().Get("type")
	module, ok := moduleForStoredAttachmentType(t)
	if !ok {
		writeError(w, http.StatusBadRequest, "type must be one of work_item, comment, deployment")
		return "", false
	}
	if !h.stored.permit(w, r, module, action) {
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
	kind, ok := h.typeFromQuery(w, r, middleware.ActionCreate)
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
	kind, ok := h.typeFromQuery(w, r, middleware.ActionCreate)
	if !ok {
		return
	}
	out, err := h.entity.ListStoredAttachmentChunks(r.Context(), id, kind)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListStoredAttachmentChunks failed", "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to list the stored chunks.")
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, out)
}

// ConfirmUpload handles POST /attachments/upload/{id}/confirm. The body is
// {"type": ..., "chunks": N}: entity-service assembles the N stored chunks into
// the file, checks what was stored against what the upload declared, and removes
// a file it refuses.
func (h *BlobAttachmentHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	raw, ok := readJSONBodyWithLimit(w, r, maxStoredAttachmentBodyBytes)
	if !ok {
		return
	}
	var in struct {
		Type   string `json:"type"`
		Chunks int    `json:"chunks"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	module, ok := moduleForStoredAttachmentType(in.Type)
	if !ok {
		writeError(w, http.StatusBadRequest, "type must be one of work_item, comment, deployment")
		return
	}
	if !h.stored.permit(w, r, module, middleware.ActionCreate) {
		return
	}
	// Only the two fields this handler checked are forwarded.
	body, _ := json.Marshal(map[string]any{"type": in.Type, "chunks": in.Chunks})
	out, err := h.stored.entity.ConfirmStoredAttachmentUpload(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ConfirmStoredAttachmentUpload failed", "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to confirm the upload.")
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, out)
}

// Download handles GET /attachments/download/{id}/content?type=: it streams the
// attachment as a download, honouring a single Range so a break can resume.
func (h *BlobAttachmentHandler) Download(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	kind, ok := h.typeFromQuery(w, r, middleware.ActionRead)
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
	if !h.stored.permit(w, r, middleware.ModuleCases, middleware.ActionRead) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), attachmentStreamWriteDeadline)
	defer cancel()
	f, err := h.entity.InlineImageContent(ctx, id)
	if err != nil {
		h.fileError(w, r, "InlineImageContent", id, err, "Failed to load the image.")
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
	// A large file takes longer than the server's global write timeout; extend it
	// as ProcessLicenseDownload does. Best effort: a ResponseWriter that cannot
	// keeps the server default.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(attachmentStreamWriteDeadline))
	w.WriteHeader(f.StatusCode)
	_, _ = io.Copy(w, f.Body) // #nosec G705 -- entity-service sends an opaque type or one of four image types; nosniff and a sandboxing CSP are always set
}

// DeleteAttachment handles POST /attachments/delete: entity-service removes the
// attachment if the caller may (only whoever uploaded it) and then its file.
func (h *BlobAttachmentHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	h.stored.DeleteAttachment(w, r)
}
