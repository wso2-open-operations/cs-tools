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

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

const (
	// UploadChunkBytes is the size of every upload chunk but the last. The browser
	// sends 8 MiB at a time, which stays well under the gateway's 50 MB request limit.
	UploadChunkBytes = 8 << 20
	// relocateBlockBytes is the size of each block written while moving an image.
	relocateBlockBytes = 8 << 20
	// relocateDeadline bounds one move; it runs detached from the browser's request
	// so a closed tab does not leave it half done.
	relocateDeadline = 2 * time.Minute
	// removedAttachmentMessage is sent with the 410 for a deleted attachment.
	removedAttachmentMessage = "This image was removed."
	// defaultMaxTransfers is the number of chunks and downloads moved at once when
	// the configuration gives no limit.
	defaultMaxTransfers = 32
)

// directLinkTTL is how long the links of a direct transfer of a file of sizeBytes
// last. Azure checks a link when a request starts, so for an upload it must cover
// the start of the last chunk (a file is sent a few chunks at a time); a bigger
// file takes longer. A link that expires early is no loss: the browser asks for
// new ones and carries on, and a download that already started finishes anyway.
func directLinkTTL(sizeBytes int64) time.Duration {
	const mib = 1 << 20
	switch {
	case sizeBytes <= 50*mib:
		return 5 * time.Minute
	case sizeBytes <= 100*mib:
		return 7 * time.Minute
	case sizeBytes <= 250*mib:
		return 10 * time.Minute
	case sizeBytes <= 500*mib:
		return 15 * time.Minute
	default:
		return 20 * time.Minute
	}
}

// FileStore is the part of azblobstore.Store the file routes use. It is nil when no
// storage is configured, and every file operation then answers 503.
type FileStore interface {
	PutBlock(ctx context.Context, key string, index int, body io.Reader, size int64) error
	UncommittedBlocks(ctx context.Context, key string) ([]azblobstore.Block, error)
	Commit(ctx context.Context, key string, blockCount int, contentType string) error
	Stat(ctx context.Context, key string) (azblobstore.Info, error)
	Open(ctx context.Context, key string, rng *azblobstore.Range) (*azblobstore.Object, error)
	Delete(ctx context.Context, key string) error
	Copy(ctx context.Context, srcKey, dstKey string) error
}

// LinkStore is the part of azblobstore.Store that signs the short-lived links of a
// direct transfer. Only a store signed in with an account key can.
type LinkStore interface {
	CanSignLinks() bool
	UploadLinks(key string, count int, ttl time.Duration) ([]string, time.Time, error)
	DownloadLink(key, fileName, contentType string, ttl time.Duration) (string, time.Time, error)
}

// FileDownload is a file opened for reading. The caller must close Object.Body.
type FileDownload struct {
	// Name is the file's own name, for the download's file name.
	Name string
	// ContentType is the type the stored row declares. An inline image is only
	// ever shown as one of four image types; the handler neutralises the rest.
	ContentType string
	// Object is the open file; Partial and ContentRange describe a ranged read.
	Object *azblobstore.Object
}

// RangeNotSatisfiableError is a Range header that asks for bytes the file does
// not have; Size is the file's size, for the Content-Range of the 416.
type RangeNotSatisfiableError struct{ Size int64 }

// Error implements the error interface.
func (e *RangeNotSatisfiableError) Error() string { return "range not satisfiable" }

// StoredAttachmentFileService moves attachment files between the portals and Azure
// Blob Storage. It is the only code in the platform that does: the portal backends
// forward a browser's bytes to it and stream its answer back, and the notification
// service reads an email's images from it. Every method runs the access check as
// the caller (row-level security, through StoredAttachmentService) before a byte
// moves, and a storage key never leaves this process.
type StoredAttachmentFileService interface {
	// PutChunk stores one chunk of a started upload. The caller must own the
	// upload, and it must still be PENDING and its parent open. A chunk may be sent
	// again (a retry overwrites the same block), in any order, and a few at a time.
	// Nothing is visible until the upload is confirmed.
	PutChunk(ctx context.Context, kind domain.StoredAttachmentKind, id string, index int, size int64, body io.Reader) error
	// ListChunks returns the chunks already stored, so an interrupted upload can
	// carry on where it stopped.
	ListChunks(ctx context.Context, kind domain.StoredAttachmentKind, id string) (domain.UploadChunkList, error)
	// CompleteUpload assembles the chunks into the file, reads what was stored and
	// runs the final checks. A file the checks refuse is removed again.
	CompleteUpload(ctx context.Context, req domain.CompleteAttachmentUploadRequest) (domain.ConfirmAttachmentUploadResponse, error)
	// OpenAttachment opens an attachment's file for a caller who may see it.
	// rangeHeader is the request's Range header (a single range is honoured).
	OpenAttachment(ctx context.Context, kind domain.StoredAttachmentKind, id, rangeHeader string) (*FileDownload, error)
	// OpenInlineImage opens a comment's inline image. workItemID, when not empty,
	// is the record an email is about: the image is only returned when it is linked
	// to that record, never when it is another record's or a draft (an internal
	// caller is not filtered by any access rule, so without this check a reference
	// to someone else's image would be read and sent).
	OpenInlineImage(ctx context.Context, id, workItemID string) (*FileDownload, error)
	// DirectUploadAllowed reports whether a file of sizeBytes may be sent straight
	// to Azure through DirectUploadLinks.
	DirectUploadAllowed(sizeBytes int64) bool
	// DirectUploadLinks returns one signed link per chunk of a started upload, for
	// the client to send its chunks to Azure itself. The access check, the open
	// parent and the size rules are those of PutChunk, made once when the links are
	// issued. The upload is then confirmed with CompleteUpload, which commits the
	// chunks and checks the real size.
	DirectUploadLinks(ctx context.Context, kind domain.StoredAttachmentKind, id string) (domain.DirectUploadLinks, error)
	// DirectDownloadLink returns a signed link to read an attachment from, for a
	// caller who may see it, as a download of the file's own name.
	DirectDownloadLink(ctx context.Context, kind domain.StoredAttachmentKind, id string) (domain.DirectDownloadLink, error)
	// DeleteAttachment deletes an attachment (the row first, as StoredAttachmentService
	// does, then the file, best effort: a file that cannot be removed now is left to
	// the orphan cleanup).
	DeleteAttachment(ctx context.Context, req domain.DeleteStoredAttachmentRequest) error
}

type storedAttachmentFileService struct {
	attachments StoredAttachmentService
	store       FileStore
	// slots bounds the transfers in flight; a transfer that finds it full is refused.
	slots chan struct{}
	// links signs direct-transfer links; nil keeps every file going through this
	// service. directMinBytes is the size above which a file goes direct.
	links          LinkStore
	directMinBytes int64
}

// FileServiceOption changes how a StoredAttachmentFileService is built.
type FileServiceOption func(*storedAttachmentFileService)

// WithDirectTransfers lets a file larger than minBytes go between the browser and
// Azure through links links signs. A nil links, or one that cannot sign, leaves
// every file going through this service.
func WithDirectTransfers(links LinkStore, minBytes int64) FileServiceOption {
	return func(s *storedAttachmentFileService) {
		if links != nil && links.CanSignLinks() {
			s.links, s.directMinBytes = links, minBytes
		}
	}
}

// NewStoredAttachmentFileService constructs a StoredAttachmentFileService.
// store may be nil (no storage configured). maxTransfers bounds the chunks and
// downloads in flight (0 means the default).
func NewStoredAttachmentFileService(attachments StoredAttachmentService, store FileStore, maxTransfers int, opts ...FileServiceOption) StoredAttachmentFileService {
	if maxTransfers <= 0 {
		maxTransfers = defaultMaxTransfers
	}
	s := &storedAttachmentFileService{attachments: attachments, store: store, slots: make(chan struct{}, maxTransfers)}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// acquire takes a transfer slot, or refuses with a 429 the portal retries.
func (s *storedAttachmentFileService) acquire() (release func(), err error) {
	select {
	case s.slots <- struct{}{}:
		return func() { <-s.slots }, nil
	default:
		return nil, &apierror.TooManyRequestsError{Msg: "too many file transfers are in progress right now, try again in a moment"}
	}
}

// requireStore refuses when no storage is configured.
func (s *storedAttachmentFileService) requireStore() error {
	if s.store == nil {
		return &apierror.ServiceUnavailableError{Msg: "attachment storage is not configured"}
	}
	return nil
}

// storeError turns a storage failure into an error that says nothing about where
// the file is: the detail is logged here and never sent.
func storeError(ctx context.Context, op, id string, err error, msg string) error {
	switch {
	case errors.Is(err, azblobstore.ErrNotFound):
		return &apierror.NotFoundError{Msg: "attachment not found"}
	case errors.Is(err, azblobstore.ErrBlockSize), errors.Is(err, azblobstore.ErrBlockIndex), errors.Is(err, azblobstore.ErrBlockCount):
		return &apierror.ValidationError{Msg: "Invalid chunk."}
	case errors.Is(err, azblobstore.ErrRangeNotSatisfiable):
		return &apierror.ValidationError{Msg: "Range not satisfiable."}
	}
	slog.ErrorContext(ctx, "blob storage failed", "op", op, "attachmentId", id,
		"correlationId", middleware.CorrelationIDFromContext(ctx), "error", err)
	return &apierror.ServiceUnavailableError{Msg: msg}
}

// maxChunks is how many chunks a file of at most maxBytes can have.
func maxChunks(maxBytes int64) int {
	if maxBytes <= 0 {
		return 1
	}
	return int((maxBytes + UploadChunkBytes - 1) / UploadChunkBytes)
}

// stagingSuffix is added to a file's key for the place its chunks are first put.
const stagingSuffix = ".upload"

// stagedUpload reports whether an upload of this declared size is put together
// under a staging key and copied to its final key at confirm. That is every upload
// that may be sent direct: its links write to the staging key only, so a link that
// is still valid after the confirm can change a file nothing reads, never the one
// that was confirmed. A smaller upload has no links and goes straight to its key.
func (s *storedAttachmentFileService) stagedUpload(target domain.AttachmentUploadTarget) bool {
	return s.DirectUploadAllowed(target.SizeBytes)
}

// uploadKey is where the chunks of this upload are put: the staging key of a
// staged upload, else the file's own key.
func (s *storedAttachmentFileService) uploadKey(target domain.AttachmentUploadTarget) string {
	if s.stagedUpload(target) {
		return target.StorageKey + stagingSuffix
	}
	return target.StorageKey
}

// PutChunk implements StoredAttachmentFileService.
func (s *storedAttachmentFileService) PutChunk(ctx context.Context, kind domain.StoredAttachmentKind, id string, index int, size int64, body io.Reader) error {
	if err := s.requireStore(); err != nil {
		return err
	}
	if index < 0 {
		return &apierror.ValidationError{Msg: "Invalid chunk index."}
	}
	target, err := s.attachments.AuthorizeUpload(ctx, domain.AuthorizeAttachmentUploadRequest{Kind: kind, ID: id})
	if err != nil {
		return err
	}
	if index >= maxChunks(target.MaxBytes) {
		return &apierror.ValidationError{Msg: "Chunk index is beyond the largest file allowed."}
	}
	if size <= 0 {
		return &apierror.ValidationError{Msg: "A chunk must declare its length."}
	}
	if size > UploadChunkBytes {
		return &apierror.ValidationError{Msg: fmt.Sprintf("A chunk may be at most %d bytes.", UploadChunkBytes)}
	}
	// A chunk can never reach past the largest file allowed.
	if int64(index)*UploadChunkBytes+size > target.MaxBytes {
		return &apierror.ValidationError{Msg: "The upload is larger than allowed."}
	}
	release, err := s.acquire()
	if err != nil {
		return err
	}
	defer release()
	if err := s.store.PutBlock(ctx, s.uploadKey(target), index, body, size); err != nil {
		return storeError(ctx, "PutBlock", id, err, "Failed to store the chunk.")
	}
	return nil
}

// ListChunks implements StoredAttachmentFileService.
func (s *storedAttachmentFileService) ListChunks(ctx context.Context, kind domain.StoredAttachmentKind, id string) (domain.UploadChunkList, error) {
	if err := s.requireStore(); err != nil {
		return domain.UploadChunkList{}, err
	}
	target, err := s.attachments.AuthorizeUpload(ctx, domain.AuthorizeAttachmentUploadRequest{Kind: kind, ID: id})
	if err != nil {
		return domain.UploadChunkList{}, err
	}
	blocks, err := s.store.UncommittedBlocks(ctx, s.uploadKey(target))
	if err != nil {
		return domain.UploadChunkList{}, storeError(ctx, "UncommittedBlocks", id, err, "Failed to list the stored chunks.")
	}
	out := domain.UploadChunkList{ChunkBytes: UploadChunkBytes, Chunks: make([]domain.UploadChunk, 0, len(blocks))}
	for _, b := range blocks {
		out.Chunks = append(out.Chunks, domain.UploadChunk{Index: b.Index, Size: b.Size})
	}
	return out, nil
}

// DirectUploadAllowed implements StoredAttachmentFileService.
func (s *storedAttachmentFileService) DirectUploadAllowed(sizeBytes int64) bool {
	return s.store != nil && s.links != nil && sizeBytes > s.directMinBytes
}

// DirectUploadLinks implements StoredAttachmentFileService.
func (s *storedAttachmentFileService) DirectUploadLinks(ctx context.Context, kind domain.StoredAttachmentKind, id string) (domain.DirectUploadLinks, error) {
	if err := s.requireStore(); err != nil {
		return domain.DirectUploadLinks{}, err
	}
	target, err := s.attachments.AuthorizeUpload(ctx, domain.AuthorizeAttachmentUploadRequest{Kind: kind, ID: id})
	if err != nil {
		return domain.DirectUploadLinks{}, err
	}
	// Only the files that are big enough to be worth it: the rest keep going
	// through this service, where every chunk is checked.
	if !s.DirectUploadAllowed(target.SizeBytes) {
		return domain.DirectUploadLinks{}, &apierror.ValidationError{Msg: "This upload cannot be sent directly; send it in chunks."}
	}
	// One link for each chunk the declared size needs. The real size is checked
	// when the upload is confirmed.
	chunks := int((target.SizeBytes + UploadChunkBytes - 1) / UploadChunkBytes)
	if chunks > maxChunks(target.MaxBytes) {
		return domain.DirectUploadLinks{}, &apierror.ValidationError{Msg: "The upload is larger than allowed."}
	}
	links, expires, err := s.links.UploadLinks(s.uploadKey(target), chunks, directLinkTTL(target.SizeBytes))
	if err != nil {
		return domain.DirectUploadLinks{}, storeError(ctx, "UploadLinks", id, err, "Failed to prepare the upload.")
	}
	return domain.DirectUploadLinks{Links: links, ChunkBytes: UploadChunkBytes, ExpiresAt: expires}, nil
}

// DirectDownloadLink implements StoredAttachmentFileService.
func (s *storedAttachmentFileService) DirectDownloadLink(ctx context.Context, kind domain.StoredAttachmentKind, id string) (domain.DirectDownloadLink, error) {
	if err := s.requireStore(); err != nil {
		return domain.DirectDownloadLink{}, err
	}
	if s.links == nil {
		// Not an error: the portal asks before every large download and falls
		// back to the streamed route on this answer, so it is a plain 404 that the
		// backends do not log.
		return domain.DirectDownloadLink{}, &apierror.NotFoundError{Msg: "Direct downloads are not available."}
	}
	loc, err := s.attachments.LocateAttachment(ctx, domain.LocateAttachmentRequest{Kind: kind, ID: id})
	if err != nil {
		return domain.DirectDownloadLink{}, err
	}
	if err := unavailable(loc); err != nil {
		return domain.DirectDownloadLink{}, err
	}
	// An attachment is always an opaque download, whatever type it declares: the
	// link is signed to answer as one.
	link, expires, err := s.links.DownloadLink(loc.StorageKey, loc.Name, "application/octet-stream", directLinkTTL(sizeOrZero(loc.SizeBytes)))
	if err != nil {
		return domain.DirectDownloadLink{}, storeError(ctx, "DownloadLink", id, err, "Failed to prepare the download.")
	}
	return domain.DirectDownloadLink{URL: link, Name: loc.Name, ExpiresAt: expires}, nil
}

// discard removes a file that must not stay, best effort: one that cannot be
// removed now is left to the orphan cleanup.
func (s *storedAttachmentFileService) discard(ctx context.Context, id, key string) {
	if err := s.store.Delete(ctx, key); err != nil {
		slog.WarnContext(ctx, "could not remove a refused upload", "attachmentId", id,
			"correlationId", middleware.CorrelationIDFromContext(ctx), "error", err)
	}
}

// refusalIsFinal reports whether an error from the final checks means the file is
// no use to anyone, so it goes. A failure that may pass (a conflict with another
// confirm, a rate limit, an expired token, a storage hiccup) leaves the file for
// the browser to confirm again.
func refusalIsFinal(err error) bool {
	var (
		ve *apierror.ValidationError
		fe *apierror.ForbiddenError
		ne *apierror.NotFoundError
		ge *apierror.GoneError
		ce *apierror.ConflictError
	)
	switch {
	case errors.As(err, &ve), errors.As(err, &fe), errors.As(err, &ne), errors.As(err, &ge):
		return true
	case errors.As(err, &ce):
		// The one conflict that is final: the parent was closed, so the row is now FAILED.
		return ce.Msg == closedParentMessage
	}
	return false
}

// CompleteUpload implements StoredAttachmentFileService.
func (s *storedAttachmentFileService) CompleteUpload(ctx context.Context, req domain.CompleteAttachmentUploadRequest) (domain.ConfirmAttachmentUploadResponse, error) {
	if err := s.requireStore(); err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, err
	}
	target, err := s.attachments.AuthorizeUpload(ctx, domain.AuthorizeAttachmentUploadRequest{Kind: req.Kind, ID: req.ID})
	if err != nil {
		// A confirm repeated after its answer was lost (a timeout, a dropped
		// connection) gets the same answer again; no chunk is committed twice.
		var conflict *apierror.ConflictError
		if errors.As(err, &conflict) && conflict.Msg == alreadyConfirmedMessage {
			return s.attachments.ConfirmUpload(ctx, domain.ConfirmAttachmentUploadRequest{Kind: req.Kind, ID: req.ID})
		}
		return domain.ConfirmAttachmentUploadResponse{}, err
	}
	if req.Chunks < 1 || req.Chunks > maxChunks(target.MaxBytes) {
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "chunks is out of range."}
	}
	// The chunks are where uploadKey put them: for a staged upload, a key that no
	// reader uses. The checks below are made there; the file reaches its own key
	// only when they pass, by a copy inside Azure that no upload link can reach.
	putKey := s.uploadKey(target)
	staged := putKey != target.StorageKey
	if err := s.store.Commit(ctx, putKey, req.Chunks, target.ContentType); err != nil {
		if errors.Is(err, azblobstore.ErrMissingBlocks) {
			return domain.ConfirmAttachmentUploadResponse{}, &apierror.ConflictError{Msg: "Some chunks have not been uploaded."}
		}
		return domain.ConfirmAttachmentUploadResponse{}, storeError(ctx, "Commit", req.ID, err, "Failed to finish the upload.")
	}
	info, err := s.store.Stat(ctx, putKey)
	if err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, storeError(ctx, "Stat", req.ID, err, "Failed to finish the upload.")
	}
	if info.Size > target.MaxBytes {
		s.discardUpload(ctx, req.ID, target.StorageKey, staged)
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "The upload is larger than allowed."}
	}
	if staged {
		if err := s.store.Copy(ctx, putKey, target.StorageKey); err != nil {
			return domain.ConfirmAttachmentUploadResponse{}, storeError(ctx, "Copy", req.ID, err, "Failed to finish the upload.")
		}
	}
	size := info.Size
	resp, err := s.attachments.ConfirmUpload(ctx, domain.ConfirmAttachmentUploadRequest{
		Kind: req.Kind, ID: req.ID, SizeBytes: &size, ContentType: target.ContentType,
	})
	if err != nil {
		if refusalIsFinal(err) {
			s.discardUpload(ctx, req.ID, target.StorageKey, staged)
		}
		return domain.ConfirmAttachmentUploadResponse{}, err
	}
	if staged {
		// The staging file is no use now; one that cannot be removed is harmless.
		s.discard(ctx, req.ID, putKey)
	}
	return resp, nil
}

// discardUpload removes a refused upload's file and, for a staged upload, the
// staging file too (the final key has a file only once the copy was made).
func (s *storedAttachmentFileService) discardUpload(ctx context.Context, id, key string, staged bool) {
	s.discard(ctx, id, key)
	if staged {
		s.discard(ctx, id, key+stagingSuffix)
	}
}

// unavailable is the refusal for an attachment whose file cannot be read.
func unavailable(loc domain.AttachmentLocation) error {
	if loc.Available && loc.StorageKey != "" {
		return nil
	}
	if loc.Reason == domain.AttachmentUnavailableDeleted {
		return &apierror.GoneError{Msg: removedAttachmentMessage, Code: apierror.CodeAttachmentRemoved}
	}
	return &apierror.NotFoundError{Msg: "attachment not found"}
}

// parseRange reads a single "bytes=a-b", "bytes=a-" or "bytes=-n" range against a
// file of size bytes. ok is false for a header that is not a range this service
// serves (it then sends the whole file); unsatisfiable means a 416.
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

// open opens the file at key, honouring a single range when one is asked for.
func (s *storedAttachmentFileService) open(ctx context.Context, id, key, rangeHeader string) (*azblobstore.Object, error) {
	var rng *azblobstore.Range
	if rangeHeader != "" {
		info, err := s.store.Stat(ctx, key)
		if err != nil {
			return nil, storeError(ctx, "Stat", id, err, "Failed to read the attachment.")
		}
		parsed, ok, bad := parseRange(rangeHeader, info.Size)
		if bad {
			return nil, &RangeNotSatisfiableError{Size: info.Size}
		}
		if ok {
			rng = &parsed
		}
	}
	obj, err := s.store.Open(ctx, key, rng)
	if err != nil {
		if errors.Is(err, azblobstore.ErrRangeNotSatisfiable) {
			return nil, &RangeNotSatisfiableError{}
		}
		return nil, storeError(ctx, "Open", id, err, "Failed to read the attachment.")
	}
	return obj, nil
}

// OpenAttachment implements StoredAttachmentFileService.
func (s *storedAttachmentFileService) OpenAttachment(ctx context.Context, kind domain.StoredAttachmentKind, id, rangeHeader string) (*FileDownload, error) {
	if err := s.requireStore(); err != nil {
		return nil, err
	}
	loc, err := s.attachments.LocateAttachment(ctx, domain.LocateAttachmentRequest{Kind: kind, ID: id})
	if err != nil {
		return nil, err
	}
	if err := unavailable(loc); err != nil {
		return nil, err
	}
	return s.download(ctx, id, loc, loc.StorageKey, rangeHeader)
}

// download opens the file at key for a transfer. It holds a transfer slot until
// the caller closes the file.
func (s *storedAttachmentFileService) download(ctx context.Context, id string, loc domain.AttachmentLocation, key, rangeHeader string) (*FileDownload, error) {
	release, err := s.acquire()
	if err != nil {
		return nil, err
	}
	obj, err := s.open(ctx, id, key, rangeHeader)
	if err != nil {
		release()
		return nil, err
	}
	obj.Body = &releasingBody{ReadCloser: obj.Body, release: release}
	return &FileDownload{Name: loc.Name, ContentType: derefString(loc.ContentType), Object: obj}, nil
}

// releasingBody gives its transfer slot back when the file is closed.
type releasingBody struct {
	io.ReadCloser
	release func()
}

// Close closes the file and gives its transfer slot back.
func (b *releasingBody) Close() error {
	defer b.release()
	return b.ReadCloser.Close()
}

// OpenInlineImage implements StoredAttachmentFileService.
func (s *storedAttachmentFileService) OpenInlineImage(ctx context.Context, id, workItemID string) (*FileDownload, error) {
	if err := s.requireStore(); err != nil {
		return nil, err
	}
	loc, err := s.attachments.LocateInlineImage(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := unavailable(loc); err != nil {
		return nil, err
	}
	if workItemID != "" && (loc.Draft || loc.ParentID == "" || !strings.EqualFold(loc.ParentID, workItemID)) {
		// Not the record the caller named: indistinguishable from an image that is not there.
		return nil, &apierror.NotFoundError{Msg: "attachment not found"}
	}
	key := loc.StorageKey
	// A description image is uploaded before its case exists, so its file starts in
	// a drafts folder. The first read after it is linked moves it to the work item's
	// own folder; if that fails the old file is read, so nothing breaks.
	if loc.RelocateTo != "" && s.relocateInlineImage(ctx, id, key, loc.RelocateTo) {
		key = loc.RelocateTo
	}
	return s.download(ctx, id, loc, key, "")
}

// relocateInlineImage moves a description image's file from the project's drafts
// folder to the work item's inline_attachments folder (to), then records the new
// key. It returns true only when everything succeeded, so the caller then reads
// from the new key; on any failure the old file is left as it was (and removed
// only after the new key was recorded), the failure is logged, and the caller
// reads the old key. The image is never unavailable because a move failed. It is
// not repeated for an image already moved: the row stops asking once it has the
// new key.
func (s *storedAttachmentFileService) relocateInlineImage(ctx context.Context, id, from, to string) bool {
	if to == "" || from == "" || from == to {
		return false
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), relocateDeadline)
	defer cancel()

	src, err := s.store.Open(ctx, from, nil)
	if err != nil {
		slog.WarnContext(ctx, "inline image move: cannot read the file", "attachmentId", id, "error", err)
		return false
	}
	defer src.Body.Close()
	size := src.ContentLength
	if size <= 0 {
		return false
	}
	blocks := 0
	for offset := int64(0); offset < size; blocks++ {
		n := min(int64(relocateBlockBytes), size-offset)
		if err := s.store.PutBlock(ctx, to, blocks, io.LimitReader(src.Body, n), n); err != nil {
			slog.WarnContext(ctx, "inline image move: cannot write the copy", "attachmentId", id, "error", err)
			return false
		}
		offset += n
	}
	if err := s.store.Commit(ctx, to, blocks, src.Info.ContentType); err != nil {
		slog.WarnContext(ctx, "inline image move: cannot commit the copy", "attachmentId", id, "error", err)
		return false
	}
	if got, err := s.store.Stat(ctx, to); err != nil || got.Size != size {
		slog.WarnContext(ctx, "inline image move: the copy is not the same size", "attachmentId", id, "want", size, "error", err)
		_ = s.store.Delete(ctx, to)
		return false
	}
	if err := s.attachments.InlineImageRelocated(ctx, id, domain.InlineImageRelocatedRequest{StorageKey: to}); err != nil {
		// The copy stays: the next read copies the same bytes over it.
		slog.WarnContext(ctx, "inline image move: the new key was not accepted", "attachmentId", id, "error", err)
		return false
	}
	if err := s.store.Delete(ctx, from); err != nil {
		slog.WarnContext(ctx, "inline image move: cannot remove the old file", "attachmentId", id, "error", err)
	}
	slog.InfoContext(ctx, "inline image moved to its work item folder", "attachmentId", id)
	return true
}

// DeleteAttachment implements StoredAttachmentFileService.
func (s *storedAttachmentFileService) DeleteAttachment(ctx context.Context, req domain.DeleteStoredAttachmentRequest) error {
	// Where the file is has to be read before the row is deleted.
	var key string
	if s.store != nil {
		if loc, err := s.attachments.LocateAttachment(ctx, domain.LocateAttachmentRequest{Kind: req.Kind, ID: req.ID}); err == nil {
			key = loc.StorageKey
		}
	}
	if err := s.attachments.DeleteAttachment(ctx, req); err != nil {
		return err
	}
	if key != "" {
		if err := s.store.Delete(ctx, key); err != nil {
			slog.WarnContext(ctx, "could not remove a deleted attachment's file",
				"correlationId", middleware.CorrelationIDFromContext(ctx), "error", err)
		}
	}
	return nil
}
