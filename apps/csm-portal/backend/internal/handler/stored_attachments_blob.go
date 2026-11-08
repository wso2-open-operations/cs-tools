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
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

// ChunkBytes is the size of every upload chunk but the last. The browser sends
// 8 MiB at a time, which stays well under the gateway's 50 MB request limit.
const ChunkBytes = 8 << 20

// maxChunkBodyBytes is the most one chunk request may carry.
const maxChunkBodyBytes = ChunkBytes

// attachmentChunkDeadline is how long one chunk may take to arrive and be stored,
// beyond the server's global read and write timeouts: 8 MiB over a slow link.
const attachmentChunkDeadline = 10 * time.Minute

// attachmentStreamWriteDeadline is how long a file download may take to write,
// beyond the server's global WriteTimeout: 100 MiB over a slow link.
const attachmentStreamWriteDeadline = 30 * time.Minute

// attachmentBlobStore is the part of azblobstore.Store the attachment routes use.
type attachmentBlobStore interface {
	PutBlock(ctx context.Context, key string, index int, body io.Reader, size int64) error
	UncommittedBlocks(ctx context.Context, key string) ([]azblobstore.Block, error)
	Commit(ctx context.Context, key string, blockCount int, contentType string) error
	Stat(ctx context.Context, key string) (azblobstore.Info, error)
	Open(ctx context.Context, key string, rng *azblobstore.Range) (*azblobstore.Object, error)
	Delete(ctx context.Context, key string) error
}

// blobEntityClient is the entity-service surface of the backend-held attachment
// routes: the access check and the storage key, never a file.
type blobEntityClient interface {
	AuthorizeStoredAttachmentUpload(ctx context.Context, id string, body []byte) ([]byte, error)
	LocateStoredAttachment(ctx context.Context, body []byte) ([]byte, error)
	LocateInlineImage(ctx context.Context, id string) ([]byte, error)
	InlineImageRelocated(ctx context.Context, id string, body []byte) error
	ConfirmStoredAttachmentUpload(ctx context.Context, id string, body []byte) ([]byte, error)
	DeleteStoredAttachment(ctx context.Context, body []byte) error
}

// uploadTarget mirrors entity-service's AttachmentUploadTarget. StorageKey stays
// in this process.
type uploadTarget struct {
	AttachmentID string `json:"attachmentId"`
	Name         string `json:"name"`
	ContentType  string `json:"contentType"`
	SizeBytes    int64  `json:"sizeBytes"`
	MaxBytes     int64  `json:"maxBytes"`
	StorageKey   string `json:"storageKey"`
}

// attachmentLocation mirrors entity-service's AttachmentLocation.
type attachmentLocation struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	ContentType *string `json:"contentType"`
	SizeBytes   *int64  `json:"sizeBytes"`
	Available   bool    `json:"available"`
	Reason      string  `json:"reason"`
	StorageKey  string  `json:"storageKey"`
	// RelocateTo is set for a description image still in the drafts folder.
	RelocateTo string `json:"relocateTo"`
}

// BlobAttachmentHandler serves attachment files held in Azure Blob Storage.
// Entity-service decides who may upload or read (row-level security) and says
// which key to use; this handler moves the bytes. The route's own permission is
// applied at registration (PermWrite to upload, PermDownloadAttachment to read).
// A key or a storage error is never sent to the browser, and there is no link to
// leak: every read goes through here with the caller's own token.
type BlobAttachmentHandler struct {
	entity blobEntityClient
	store  attachmentBlobStore
}

// NewBlobAttachmentHandler creates a BlobAttachmentHandler.
func NewBlobAttachmentHandler(entity blobEntityClient, store attachmentBlobStore) *BlobAttachmentHandler {
	return &BlobAttachmentHandler{entity: entity, store: store}
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

// authorize asks entity-service whether the caller may upload to the pending
// attachment and where.
func (h *BlobAttachmentHandler) authorize(w http.ResponseWriter, r *http.Request, id, kind string) (uploadTarget, bool) {
	body, _ := json.Marshal(map[string]string{"type": kind})
	out, err := h.entity.AuthorizeStoredAttachmentUpload(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AuthorizeStoredAttachmentUpload failed", "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to authorize the upload.")
		return uploadTarget{}, false
	}
	var t uploadTarget
	if err := json.Unmarshal(out, &t); err != nil || t.StorageKey == "" {
		slog.ErrorContext(r.Context(), "entity returned an unusable upload target", "attachmentID", id)
		writeError(w, http.StatusServiceUnavailable, "Failed to authorize the upload.")
		return uploadTarget{}, false
	}
	return t, true
}

// maxChunks is how many chunks a file of at most maxBytes can have.
func maxChunks(maxBytes int64) int {
	if maxBytes <= 0 {
		return 1
	}
	return int((maxBytes + ChunkBytes - 1) / ChunkBytes)
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
	target, ok := h.authorize(w, r, id, kind)
	if !ok {
		return
	}
	if index >= maxChunks(target.MaxBytes) {
		writeError(w, http.StatusBadRequest, "Chunk index is beyond the largest file allowed.")
		return
	}
	size := r.ContentLength
	if size <= 0 {
		writeError(w, http.StatusLengthRequired, "A chunk must declare its length.")
		return
	}
	if size > maxChunkBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("A chunk may be at most %d bytes.", maxChunkBodyBytes))
		return
	}
	// A chunk can never reach past the largest file allowed.
	if int64(index)*ChunkBytes+size > target.MaxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "The upload is larger than allowed.")
		return
	}
	// A slow link can need longer than the server's global timeouts for one
	// chunk. Best effort: a ResponseWriter that cannot extend them keeps them.
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(attachmentChunkDeadline)
	_ = rc.SetReadDeadline(deadline)
	_ = rc.SetWriteDeadline(deadline)
	body := http.MaxBytesReader(w, r.Body, maxChunkBodyBytes)
	if err := h.store.PutBlock(r.Context(), target.StorageKey, index, body, size); err != nil {
		h.storeError(w, r, "PutBlock", id, err, "Failed to store the chunk.")
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
	target, ok := h.authorize(w, r, id, kind)
	if !ok {
		return
	}
	blocks, err := h.store.UncommittedBlocks(r.Context(), target.StorageKey)
	if err != nil {
		h.storeError(w, r, "UncommittedBlocks", id, err, "Failed to list the stored chunks.")
		return
	}
	type chunk struct {
		Index int   `json:"index"`
		Size  int64 `json:"size"`
	}
	chunks := make([]chunk, 0, len(blocks))
	for _, b := range blocks {
		chunks = append(chunks, chunk{Index: b.Index, Size: b.Size})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSONValue(w, http.StatusOK, map[string]any{"chunkBytes": ChunkBytes, "chunks": chunks})
}

// ConfirmUpload handles POST /attachments/upload/{id}/confirm. The body is
// {"type": ..., "chunks": N}: it assembles the N stored chunks into the file,
// then hands entity-service the size and type actually stored for the final
// check. A file entity-service refuses is deleted again.
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
	target, ok := h.authorize(w, r, id, in.Type)
	if !ok {
		return
	}
	if in.Chunks < 1 || in.Chunks > maxChunks(target.MaxBytes) {
		writeError(w, http.StatusBadRequest, "chunks is out of range.")
		return
	}
	ctx := r.Context()
	if err := h.store.Commit(ctx, target.StorageKey, in.Chunks, target.ContentType); err != nil {
		if errors.Is(err, azblobstore.ErrMissingBlocks) {
			writeError(w, http.StatusConflict, "Some chunks have not been uploaded.")
			return
		}
		h.storeError(w, r, "Commit", id, err, "Failed to finish the upload.")
		return
	}
	info, err := h.store.Stat(ctx, target.StorageKey)
	if err != nil {
		h.storeError(w, r, "Stat", id, err, "Failed to finish the upload.")
		return
	}
	if info.Size > target.MaxBytes {
		h.discard(ctx, id, target.StorageKey)
		writeError(w, http.StatusRequestEntityTooLarge, "The upload is larger than allowed.")
		return
	}
	confirmBody, _ := json.Marshal(map[string]any{
		"type": in.Type, "sizeBytes": info.Size, "contentType": target.ContentType,
	})
	out, err := h.entity.ConfirmStoredAttachmentUpload(ctx, id, confirmBody)
	if err != nil {
		slog.ErrorContext(ctx, "entity ConfirmStoredAttachmentUpload failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		// A refusal (the file does not match what was declared, the upload is no
		// longer pending) is final: the file is no use to anyone, so it goes. A
		// failure that may pass (a 401, 408, 409, 429 or a 5xx) leaves the file for
		// the browser to confirm again.
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 &&
			!retryableConfirmStatus(apiErr.StatusCode) {
			h.discard(ctx, id, target.StorageKey)
		}
		mapStoredAttachmentError(w, err, "Failed to confirm the attachment upload.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// retryableConfirmStatus reports a 4xx from entity-service that may pass on a
// second try (an expired token, a timeout, a conflict, a rate limit), so the
// committed file stays for the browser to confirm again.
func retryableConfirmStatus(code int) bool {
	switch code {
	case http.StatusUnauthorized, http.StatusRequestTimeout, http.StatusConflict, http.StatusTooManyRequests:
		return true
	}
	return false
}

// discard removes a file that must not stay, best effort: one that cannot be
// removed now is left to the orphan cleanup.
func (h *BlobAttachmentHandler) discard(ctx context.Context, id, key string) {
	if err := h.store.Delete(ctx, key); err != nil {
		slog.WarnContext(ctx, "could not remove a refused upload", "attachmentID", id, "err", summarizeErr(err))
	}
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
	body, _ := json.Marshal(map[string]string{"type": kind, "id": id})
	out, err := h.entity.LocateStoredAttachment(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity LocateStoredAttachment failed", "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to get the attachment.")
		return
	}
	var loc attachmentLocation
	if err := json.Unmarshal(out, &loc); err != nil {
		writeError(w, http.StatusServiceUnavailable, "Failed to get the attachment.")
		return
	}
	if !h.available(w, loc) {
		return
	}
	h.stream(w, r, loc, false)
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
	out, err := h.entity.LocateInlineImage(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity LocateInlineImage failed", "attachmentID", id, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to retrieve the image.")
		return
	}
	var loc attachmentLocation
	if err := json.Unmarshal(out, &loc); err != nil {
		writeError(w, http.StatusServiceUnavailable, "Failed to retrieve the image.")
		return
	}
	if !h.available(w, loc) {
		return
	}
	// A description image is uploaded before its case exists, so its file starts
	// in a drafts folder. The first read after it is linked moves it to the work
	// item's own folder; if that fails the old file is read, so nothing breaks.
	if loc.RelocateTo != "" && h.relocateInlineImage(r.Context(), id, loc) {
		loc.StorageKey = loc.RelocateTo
	}
	h.stream(w, r, loc, true)
}

// available writes the refusal for an attachment whose file cannot be read and
// reports whether it can.
func (h *BlobAttachmentHandler) available(w http.ResponseWriter, loc attachmentLocation) bool {
	if loc.Available && loc.StorageKey != "" {
		return true
	}
	switch loc.Reason {
	case "DELETED":
		writeErrorCode(w, http.StatusGone, ErrMsgAttachmentRemoved, errCodeAttachmentRemoved)
	case "STORAGE_UNAVAILABLE":
		writeError(w, http.StatusServiceUnavailable, "The attachment is temporarily unavailable.")
	default:
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
	}
	return false
}

// parseRange reads a single "bytes=a-b", "bytes=a-" or "bytes=-n" range against
// a file of size bytes. ok is false for a header that is not a range this
// handler serves (it then sends the whole file); unsatisfiable means a 416.
func parseRange(header string, size int64) (rng azblobstore.Range, ok, unsatisfiable bool) {
	const prefix = "bytes="
	if !strings.HasPrefix(header, prefix) || strings.Contains(header, ",") {
		return azblobstore.Range{}, false, false
	}
	spec := strings.TrimSpace(header[len(prefix):])
	startStr, endStr, found := strings.Cut(spec, "-")
	if !found {
		return azblobstore.Range{}, false, false
	}
	if startStr == "" {
		n, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || n <= 0 {
			return azblobstore.Range{}, false, err == nil
		}
		if n > size {
			n = size
		}
		return azblobstore.Range{Offset: size - n, Length: n}, true, false
	}
	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 {
		return azblobstore.Range{}, false, false
	}
	if start >= size {
		return azblobstore.Range{}, false, true
	}
	end := size - 1
	if endStr != "" {
		e, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || e < start {
			return azblobstore.Range{}, false, false
		}
		if e < end {
			end = e
		}
	}
	return azblobstore.Range{Offset: start, Length: end - start + 1}, true, false
}

// stream copies the file to the response, never holding it in memory. A
// download is always sent as an attachment of an opaque type; only an inline
// comment image is shown in the page, and then only as one of four image types.
func (h *BlobAttachmentHandler) stream(w http.ResponseWriter, r *http.Request, loc attachmentLocation, inline bool) {
	ctx := r.Context()
	var rng *azblobstore.Range
	if rh := r.Header.Get("Range"); rh != "" && !inline {
		info, err := h.store.Stat(ctx, loc.StorageKey)
		if err != nil {
			h.storeError(w, r, "Stat", loc.ID, err, "Failed to read the attachment.")
			return
		}
		parsed, ok, bad := parseRange(rh, info.Size)
		if bad {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size))
			writeError(w, http.StatusRequestedRangeNotSatisfiable, "Range not satisfiable.")
			return
		}
		if ok {
			rng = &parsed
		}
	}
	obj, err := h.store.Open(ctx, loc.StorageKey, rng)
	if err != nil {
		if errors.Is(err, azblobstore.ErrRangeNotSatisfiable) {
			writeError(w, http.StatusRequestedRangeNotSatisfiable, "Range not satisfiable.")
			return
		}
		h.storeError(w, r, "Open", loc.ID, err, "Failed to read the attachment.")
		return
	}
	defer obj.Body.Close()

	hd := w.Header()
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	if inline {
		ct := ""
		if loc.ContentType != nil {
			ct = strings.ToLower(strings.TrimSpace(strings.SplitN(*loc.ContentType, ";", 2)[0]))
		}
		if !storedInlineImageTypes[ct] {
			ct = "application/octet-stream"
		}
		hd.Set("Content-Type", ct)
		hd.Set("Content-Disposition", "inline")
		hd.Set("Cache-Control", "private, max-age=300")
	} else {
		hd.Set("Content-Type", "application/octet-stream")
		hd.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": downloadName(loc.Name)}))
		hd.Set("Accept-Ranges", "bytes")
		hd.Set("Cache-Control", "private, no-store")
	}
	hd.Set("Content-Length", strconv.FormatInt(obj.ContentLength, 10))
	status := http.StatusOK
	if obj.Partial {
		hd.Set("Content-Range", obj.ContentRange)
		status = http.StatusPartialContent
	}
	// A large file takes longer than the server's global write timeout. Best
	// effort: a ResponseWriter that cannot extend it keeps the server default.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(attachmentStreamWriteDeadline))
	w.WriteHeader(status)
	_, _ = io.Copy(w, obj.Body) // #nosec G705 -- opaque type or one of four image types, with nosniff and a sandboxing CSP
}

// downloadName is the name a file is saved under: no path, and nothing that
// could break out of the header.
func downloadName(name string) string {
	name = strings.NewReplacer("\r", "", "\n", "", "\"", "'", "/", "_", "\\", "_").Replace(name)
	if strings.TrimSpace(name) == "" {
		return "attachment"
	}
	return name
}

// DeleteAttachment handles POST /attachments/delete: entity-service removes the
// attachment if the caller may (only whoever uploaded it), then its file goes.
func (h *BlobAttachmentHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	user, body, ok := readStoredAttachmentBody(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	// Where the file is has to be read before the row is deleted.
	var key string
	if out, err := h.entity.LocateStoredAttachment(ctx, body); err == nil {
		var loc attachmentLocation
		if json.Unmarshal(out, &loc) == nil {
			key = loc.StorageKey
		}
	}
	if err := h.entity.DeleteStoredAttachment(ctx, body); err != nil {
		slog.ErrorContext(ctx, "entity DeleteStoredAttachment failed", "userID", user.UserID, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to delete the attachment.")
		return
	}
	if key != "" {
		if err := h.store.Delete(ctx, key); err != nil {
			slog.WarnContext(ctx, "could not remove a deleted attachment's file", "err", summarizeErr(err))
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// storeError logs a storage failure and answers with a message that says
// nothing about where the file is.
func (h *BlobAttachmentHandler) storeError(w http.ResponseWriter, r *http.Request, op, id string, err error, msg string) {
	switch {
	case errors.Is(err, azblobstore.ErrNotFound):
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
	case errors.Is(err, azblobstore.ErrBlockSize), errors.Is(err, azblobstore.ErrBlockIndex), errors.Is(err, azblobstore.ErrBlockCount):
		writeError(w, http.StatusBadRequest, "Invalid chunk.")
	default:
		slog.ErrorContext(r.Context(), "blob storage failed", "op", op, "attachmentID", id, "err", summarizeErr(err))
		writeError(w, http.StatusServiceUnavailable, msg)
	}
}
