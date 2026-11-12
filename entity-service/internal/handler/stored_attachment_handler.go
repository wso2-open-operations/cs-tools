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
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const (
	// attachmentChunkDeadline is how long one chunk may take to arrive and be
	// stored, beyond the server's global read and write timeouts: 8 MiB over a slow
	// link.
	attachmentChunkDeadline = 10 * time.Minute
	// attachmentStreamDeadline is how long a file download may take to write,
	// beyond the server's global WriteTimeout: 100 MiB over a slow link.
	attachmentStreamDeadline = 30 * time.Minute
	// attachmentImageDeadline is the same for an inline image, which is small but
	// may be moved to its work item's folder on the way.
	attachmentImageDeadline = 5 * time.Minute
)

// storedInlineImageTypes is the only set of types a streamed inline image is
// sent as; anything else is neutralised to application/octet-stream.
var storedInlineImageTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
}

// StoredAttachmentHandler handles HTTP requests that record and authorize the
// attachment files the portal backends hold in Azure Blob Storage.
type StoredAttachmentHandler struct {
	svc     service.StoredAttachmentService
	files   service.StoredAttachmentFileService
	uploads service.AttachmentUploadCleanupService
}

// WithFiles attaches the file service, which backs every route that moves a file
// (the chunks, confirm, download, inline images and delete).
func (h *StoredAttachmentHandler) WithFiles(f service.StoredAttachmentFileService) *StoredAttachmentHandler {
	h.files = f
	return h
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

// Limits handles GET /attachments/limits: the attachment sizes in force. They come
// from configuration, so the answer may be cached for a few minutes.
func (h *StoredAttachmentHandler) Limits(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(h.svc.Limits())
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

// ConfirmUpload handles POST /attachments/upload/{id}/confirm. The body is
// {"type": ..., "chunks": N}: this service assembles the N stored chunks into the
// file, reads what was stored and runs the final checks.
func (h *StoredAttachmentHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	if h.files == nil {
		http.NotFound(w, r)
		return
	}
	var req domain.CompleteAttachmentUploadRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("id")
	resp, err := h.files.CompleteUpload(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, resp)
}

// DeleteAttachment handles POST /attachments/delete: the attachment is removed
// if the caller may (whoever uploaded it, or a portal admin), then its file goes.
func (h *StoredAttachmentHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	var req domain.DeleteStoredAttachmentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	var err error
	if h.files != nil {
		err = h.files.DeleteAttachment(r.Context(), req)
	} else {
		_, err = h.svc.DeleteAttachment(r.Context(), req)
	}
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// kindFromQuery reads the ?type= of a route whose body or answer is a file.
func kindFromQuery(r *http.Request) domain.StoredAttachmentKind {
	return domain.StoredAttachmentKind(r.URL.Query().Get("type"))
}

// PutChunk handles PUT /attachments/upload/{id}/chunks/{index}?type=: one chunk
// of up to 8 MiB, as the request body. Chunks may be sent again, in any order,
// and a few at a time. Nothing is visible until the upload is confirmed.
func (h *StoredAttachmentHandler) PutChunk(w http.ResponseWriter, r *http.Request) {
	if h.files == nil {
		http.NotFound(w, r)
		return
	}
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		writeServiceError(w, r, &apierror.ValidationError{Msg: "Invalid chunk index."})
		return
	}
	// A slow link can need longer than the server's global timeouts for one chunk.
	// Best effort: a ResponseWriter that cannot extend them keeps them.
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(attachmentChunkDeadline)
	_ = rc.SetReadDeadline(deadline)
	_ = rc.SetWriteDeadline(deadline)
	body := http.MaxBytesReader(w, r.Body, service.UploadChunkBytes)
	if err := h.files.PutChunk(r.Context(), kindFromQuery(r), r.PathValue("id"), index, r.ContentLength, body); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// ListChunks handles GET /attachments/upload/{id}/chunks?type=: the chunks
// already stored, so an interrupted upload can carry on where it stopped.
func (h *StoredAttachmentHandler) ListChunks(w http.ResponseWriter, r *http.Request) {
	if h.files == nil {
		http.NotFound(w, r)
		return
	}
	resp, err := h.files.ListChunks(r.Context(), kindFromQuery(r), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStoredAttachmentJSON(w, http.StatusOK, resp)
}

// DirectUploadLinks handles POST /attachments/{id}/upload-links?type=: one signed
// link per chunk, so a large file goes from the browser straight to Azure. The
// links are credentials, so the answer is never cached.
func (h *StoredAttachmentHandler) DirectUploadLinks(w http.ResponseWriter, r *http.Request) {
	if h.files == nil {
		http.NotFound(w, r)
		return
	}
	resp, err := h.files.DirectUploadLinks(r.Context(), kindFromQuery(r), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeStoredAttachmentJSON(w, http.StatusOK, resp)
}

// DirectDownloadLink handles POST /attachments/{id}/download-links?type=: a signed
// link the browser downloads the file from, straight from Azure. The answer is
// never cached.
func (h *StoredAttachmentHandler) DirectDownloadLink(w http.ResponseWriter, r *http.Request) {
	if h.files == nil {
		http.NotFound(w, r)
		return
	}
	resp, err := h.files.DirectDownloadLink(r.Context(), kindFromQuery(r), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeStoredAttachmentJSON(w, http.StatusOK, resp)
}

// DownloadAttachment handles GET /attachments/download/{id}/content?type=: the
// file as a download, honouring a single Range so a break can resume.
func (h *StoredAttachmentHandler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	if h.files == nil {
		http.NotFound(w, r)
		return
	}
	f, err := h.files.OpenAttachment(r.Context(), kindFromQuery(r), r.PathValue("id"), r.Header.Get("Range"))
	if err != nil {
		h.writeFileError(w, r, err)
		return
	}
	defer f.Object.Body.Close()
	h.stream(w, f, false, attachmentStreamDeadline)
}

// InlineImageContent handles GET /attachments/inline/{id}/content: a comment's
// inline image. With ?workItemId= the image is only returned when it is linked to
// that record (the notification service reads the images of an email this way).
func (h *StoredAttachmentHandler) InlineImageContent(w http.ResponseWriter, r *http.Request) {
	if h.files == nil {
		http.NotFound(w, r)
		return
	}
	f, err := h.files.OpenInlineImage(r.Context(), r.PathValue("id"), r.URL.Query().Get("workItemId"))
	if err != nil {
		h.writeFileError(w, r, err)
		return
	}
	defer f.Object.Body.Close()
	h.stream(w, f, true, attachmentImageDeadline)
}

// writeFileError answers a file read that failed: a 416 for a range the file does
// not have, otherwise the ordinary mapping.
func (h *StoredAttachmentHandler) writeFileError(w http.ResponseWriter, r *http.Request, err error) {
	var rng *service.RangeNotSatisfiableError
	if errors.As(err, &rng) {
		if rng.Size > 0 {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", rng.Size))
		}
		http.Error(w, "Range not satisfiable.", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	writeServiceError(w, r, err)
}

// stream copies the file to the response, never holding it in memory. A download
// is always sent as an attachment of an opaque type; only an inline comment image
// is shown in the page, and then only as one of four image types.
func (h *StoredAttachmentHandler) stream(w http.ResponseWriter, f *service.FileDownload, inline bool, writeDeadline time.Duration) {
	obj := f.Object
	hd := w.Header()
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	if inline {
		ct := strings.ToLower(strings.TrimSpace(strings.SplitN(f.ContentType, ";", 2)[0]))
		if !storedInlineImageTypes[ct] {
			ct = "application/octet-stream"
		}
		hd.Set("Content-Type", ct)
		hd.Set("Content-Disposition", "inline")
		hd.Set("Cache-Control", "private, max-age=300")
	} else {
		hd.Set("Content-Type", "application/octet-stream")
		hd.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": downloadName(f.Name)}))
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
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(writeDeadline))
	w.WriteHeader(status)
	_, _ = io.Copy(w, obj.Body) // #nosec G705 -- opaque type or one of four image types, with nosniff and a sandboxing CSP
}

// downloadName is the name a file is saved under: no path, and nothing that could
// break out of the header.
func downloadName(name string) string {
	name = strings.NewReplacer("\r", "", "\n", "", "\"", "'", "/", "_", "\\", "_").Replace(name)
	if strings.TrimSpace(name) == "" {
		return "attachment"
	}
	return name
}

// writeStoredAttachmentJSON writes a response that can carry an internal storage
// key, so it must never be cached by a browser or an intermediary.
func writeStoredAttachmentJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
