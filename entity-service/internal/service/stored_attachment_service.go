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
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/errgroup"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/filestore"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

const (
	// maxDownloadLinkIDs is how many attachments one POST /attachments/download
	// call may ask for: one file per request. The ids array is kept so the
	// request shape does not change if the limit is ever raised.
	maxDownloadLinkIDs = 1
	// maxAttachmentLinkIDs bounds how many inline images of one comment are
	// linked to it.
	maxAttachmentLinkIDs = 20
	// attachmentLinkConcurrency bounds how many shares one call creates at once.
	attachmentLinkConcurrency = 8
	// maxSharesPerCallerPerMinute is how many shares (download or upload links)
	// one caller may have created per minute; beyond it the call is a 429.
	maxSharesPerCallerPerMinute = 60
	// maxAttachmentNameLen is the width of the name column (VARCHAR(255)).
	maxAttachmentNameLen = 255
	// downloadLinkMaxUses and previewLinkMaxUses are how many requests a link
	// serves before SFTPGo refuses it. A link is a bearer credential, so it is
	// spent on the first open: a download is one request; a preview gets one
	// spare for a browser that fetches an image twice. The webapp asks again if
	// a link has gone.
	downloadLinkMaxUses = 1
	previewLinkMaxUses  = 2
	// sniffBytes is how much of an upload is read to check its real type.
	sniffBytes = 512
)

// inlineImageTypes are the only types a comment image may have. SVG is left
// out on purpose: it can carry script.
var inlineImageTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

// fileStore is the part of the SFTPGo client this service uses, so tests can
// stand in for it.
type fileStore interface {
	CreateShare(ctx context.Context, spec filestore.ShareSpec) (string, error)
	MkdirAll(ctx context.Context, dir string) error
	ListDir(ctx context.Context, dir string) ([]filestore.Entry, error)
	Rename(ctx context.Context, from, to string) error
	RemoveFile(ctx context.Context, path string) error
	RemoveDir(ctx context.Context, dir string) error
	ReadHead(ctx context.Context, path string, n int) ([]byte, error)
	Open(ctx context.Context, path string) (io.ReadCloser, int64, error)
	DownloadURL(shareID string) string
	UploadURL(shareID string) string
}

// StoredAttachmentLimits are the settings the service runs under.
type StoredAttachmentLimits struct {
	// MaxBytes caps one file; InlineMaxBytes caps one comment image.
	MaxBytes       int64
	InlineMaxBytes int64
	// ReadLinkTTL and UploadLinkTTL are how long a download and an upload link
	// stay valid.
	ReadLinkTTL   time.Duration
	UploadLinkTTL time.Duration
}

type storedAttachmentService struct {
	repo   repository.StoredAttachmentRepository
	store  fileStore
	limits StoredAttachmentLimits
	now    func() time.Time
	shares *shareLimiter
}

// NewStoredAttachmentService constructs a StoredAttachmentService.
//
// There is no AccessService here on purpose. Who may see an attachment, or add
// one, is decided by row-level security on the attachment tables (migration
// 100026) plus the parent lookup, both run as the caller: an external caller
// needs to be a REGISTERED member of the parent's project, an internal one is
// unrestricted, and a parent with no project is internal-only.
func NewStoredAttachmentService(repo repository.StoredAttachmentRepository, store fileStore, limits StoredAttachmentLimits) StoredAttachmentService {
	return &storedAttachmentService{repo: repo, store: store, limits: limits, now: time.Now, shares: newShareLimiter(maxSharesPerCallerPerMinute, time.Minute)}
}

// resolveActorEmail resolves the caller's email from x-user-id-token. It is
// stored as created_by on a new attachment, and the row-level-security policy
// for an unlinked comment image compares it with the caller's own email.
func (s *storedAttachmentService) resolveActorEmail(ctx context.Context) (string, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	return email, nil
}

// limiterKey is who a share is charged to: the user's UUID, else the email in
// the user's own (already verified) x-user-id-token, else the client id. The
// client id is shared by every user of one portal backend, so it is only the key
// for a caller with no user at all (a machine caller); the email keeps one user
// without a userid claim from using up everyone else's allowance. It is a map
// key only and is never logged (auditCaller stays the audit identifier).
func limiterKey(ctx context.Context) string {
	id := auth.IdentityFromContext(ctx)
	if id.UserID != "" {
		return id.UserID
	}
	if token := middleware.UserIDTokenFromContext(ctx); token != "" {
		if email, err := emailFromJWT(token); err == nil && email != "" {
			return "email:" + strings.ToLower(email)
		}
	}
	return auditCaller(ctx)
}

// reserveShares charges n new shares to the caller, or refuses with a 429.
func (s *storedAttachmentService) reserveShares(ctx context.Context, n int) error {
	if n == 0 || s.shares.allow(limiterKey(ctx), n) {
		return nil
	}
	return &apierror.TooManyRequestsError{Msg: "too many attachment links requested; try again in a minute"}
}

func validateStoredAttachmentKind(kind domain.StoredAttachmentKind) error {
	switch kind {
	case domain.StoredAttachmentWorkItem, domain.StoredAttachmentComment, domain.StoredAttachmentDeployment:
		return nil
	}
	return &apierror.ValidationError{Msg: "kind must be one of work_item, comment, deployment"}
}

// storageUnavailable is what the caller sees when SFTPGo cannot be reached.
// The cause is logged, never returned: it can name internal hosts.
func storageUnavailable(ctx context.Context, what string, err error) error {
	slog.ErrorContext(ctx, "attachment storage call failed", "operation", what, "error", err)
	return &apierror.ServiceUnavailableError{Msg: "file storage is unavailable, please try again"}
}

// auditCaller identifies the caller on an audit line the way the access log does
// (middleware.Logger): the user's UUID (the userid claim), else the client id of a
// machine caller, else "-". It is not the email, which is personal data and is
// kept out of logs.
func auditCaller(ctx context.Context) string {
	id := auth.IdentityFromContext(ctx)
	switch {
	case id.UserID != "":
		return id.UserID
	case id.ClientID != "":
		return id.ClientID
	}
	return "-"
}

// auditLink records who was given a link to which attachment. SFTPGo only sees
// the one service user for every transfer, so this line is how a download or an
// upload is tied to a person: SFTPGo's transfer log carries the file path (for an
// upload, the staging folder named after the attachment id), which this line
// carries too, so the two match on path and time. The person is the callerId the
// access log shows for the same request (see auditCaller), and correlationId
// finds that request's other lines; neither is personal data.
//
// The share id is NOT logged in full. It is the link's secret (the link is
// .../api/v2/shares/<id>), so anyone who could read it from a log could use the
// link until it expires. Only a short prefix is kept, enough to recognise one in
// a proxy log without being able to use it.
func auditLink(ctx context.Context, action string, kind domain.StoredAttachmentKind, attachmentID, path, shareID string, expires time.Time) {
	slog.InfoContext(ctx, "attachment link created",
		"audit", true, "action", action, "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx), "kind", string(kind),
		"attachmentId", attachmentID, "path", path, "shareRef", shareRef(shareID),
		"expiresAt", expires)
}

// shareRef is the first few characters of a share id, never the whole of it.
func shareRef(id string) string {
	n := len(id) / 4
	if n > 6 {
		n = 6
	}
	return id[:n] + "..."
}

// ResolveLinks implements StoredAttachmentService.
func (s *storedAttachmentService) ResolveLinks(ctx context.Context, req domain.AttachmentLinksRequest) (domain.AttachmentLinksResponse, error) {
	if err := validateStoredAttachmentKind(req.Kind); err != nil {
		return domain.AttachmentLinksResponse{}, err
	}
	if len(req.IDs) == 0 {
		return domain.AttachmentLinksResponse{}, &apierror.ValidationError{Msg: "ids is required"}
	}
	if len(req.IDs) > maxDownloadLinkIDs {
		return domain.AttachmentLinksResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("ids cannot exceed %d entry: request one file at a time", maxDownloadLinkIDs)}
	}
	if err := validateUUIDs("ids", req.IDs); err != nil {
		return domain.AttachmentLinksResponse{}, err
	}
	uses := downloadLinkMaxUses
	switch req.Purpose {
	case "", domain.AttachmentPurposeDownload:
	case domain.AttachmentPurposePreview:
		uses = previewLinkMaxUses
	default:
		return domain.AttachmentLinksResponse{}, &apierror.ValidationError{Msg: "purpose must be download or preview"}
	}

	// Only rows the caller may see come back; the rest are treated as absent.
	rows, err := s.repo.GetByIDs(ctx, req.Kind, req.IDs)
	if err != nil {
		return domain.AttachmentLinksResponse{}, err
	}
	byID := make(map[string]domain.StoredAttachmentRow, len(rows))
	for _, row := range rows {
		byID[strings.ToLower(row.ID)] = row
	}

	if err := s.reserveShares(ctx, len(rows)); err != nil {
		return domain.AttachmentLinksResponse{}, err
	}
	links := make(map[string]domain.AttachmentLink, len(rows))
	var g errgroup.Group
	g.SetLimit(attachmentLinkConcurrency)
	results := make(chan domain.AttachmentLink, len(rows))
	for _, row := range rows {
		g.Go(func() error {
			results <- s.linkFor(ctx, req.Kind, row, uses)
			return nil
		})
	}
	_ = g.Wait()
	close(results)
	for l := range results {
		links[strings.ToLower(l.ID)] = l
	}

	// Answer in the order asked, once per id.
	out := make([]domain.AttachmentLink, 0, len(links))
	seen := make(map[string]bool, len(req.IDs))
	for _, id := range req.IDs {
		key := strings.ToLower(id)
		if seen[key] {
			continue
		}
		seen[key] = true
		if l, ok := links[key]; ok {
			out = append(out, l)
		}
	}
	return domain.AttachmentLinksResponse{Links: out}, nil
}

// linkFor builds one attachment's entry, creating its download share when the
// file is stored. A storage failure makes only that entry unavailable.
func (s *storedAttachmentService) linkFor(ctx context.Context, kind domain.StoredAttachmentKind, row domain.StoredAttachmentRow, uses int) domain.AttachmentLink {
	l := domain.AttachmentLink{
		ID: row.ID, Number: row.Number, Name: row.Name,
		ContentType: row.ContentType, SizeBytes: row.SizeBytes,
	}
	switch {
	case row.Status == "DELETED":
		l.Reason = domain.AttachmentUnavailableDeleted
		return l
	case row.Status == "FAILED":
		l.Reason = domain.AttachmentUnavailableFailed
		return l
	case row.Status != "COMPLETE" || row.StorageKey == nil || *row.StorageKey == "":
		l.Reason = domain.AttachmentUnavailableNotStored
		return l
	}
	expires := s.now().Add(s.limits.ReadLinkTTL)
	shareID, err := s.store.CreateShare(ctx, filestore.ShareSpec{
		Name: "dl", Scope: filestore.ShareRead, Path: *row.StorageKey,
		MaxUses: uses, ExpiresAt: expires,
	})
	if err != nil {
		slog.ErrorContext(ctx, "attachment storage call failed", "operation", "create read share", "attachmentId", row.ID, "error", err)
		l.Reason = domain.AttachmentUnavailableStorage
		return l
	}
	auditLink(ctx, "download", kind, row.ID, *row.StorageKey, shareID, expires)
	l.Available = true
	l.URL = s.store.DownloadURL(shareID)
	l.ExpiresAt = &expires
	return l
}

// InitiateUpload implements StoredAttachmentService.
func (s *storedAttachmentService) InitiateUpload(ctx context.Context, req domain.InitiateAttachmentUploadRequest) (domain.InitiateAttachmentUploadResponse, error) {
	if err := validateStoredAttachmentKind(req.Kind); err != nil {
		return domain.InitiateAttachmentUploadResponse{}, err
	}
	if !validate.IsUUID(req.ParentID) {
		return domain.InitiateAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "parentId must be a valid UUID"}
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return domain.InitiateAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "name is required"}
	}
	if utf8.RuneCountInString(req.Name) > maxAttachmentNameLen {
		return domain.InitiateAttachmentUploadResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("name cannot exceed %d characters", maxAttachmentNameLen)}
	}
	req.ContentType = strings.ToLower(strings.TrimSpace(req.ContentType))
	if req.Kind == domain.StoredAttachmentComment {
		if !inlineImageTypes[req.ContentType] {
			return domain.InitiateAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "only PNG, JPEG, GIF and WebP images can be added to a comment"}
		}
	} else if req.ContentType == "" {
		req.ContentType = "application/octet-stream"
	} else if !strings.Contains(req.ContentType, "/") || len(req.ContentType) > 255 {
		return domain.InitiateAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "contentType must look like type/subtype"}
	}
	limit := s.limitFor(req.Kind)
	if req.SizeBytes <= 0 {
		return domain.InitiateAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "sizeBytes must be greater than 0"}
	}
	if req.SizeBytes > limit {
		return domain.InitiateAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "the file is larger than the " + formatByteLimit(limit) + " limit"}
	}

	email, err := s.resolveActorEmail(ctx)
	if err != nil {
		return domain.InitiateAttachmentUploadResponse{}, err
	}

	// Run as the caller: a parent they cannot see is not found.
	parent, err := s.repo.GetParent(ctx, req.Kind, req.ParentID)
	if err != nil {
		return domain.InitiateAttachmentUploadResponse{}, err
	}
	if parent.Closed {
		return domain.InitiateAttachmentUploadResponse{}, &apierror.ConflictError{Msg: closedParentMessage}
	}

	// Charged before the row is written, so a refused call leaves nothing behind.
	if err := s.reserveShares(ctx, 1); err != nil {
		return domain.InitiateAttachmentUploadResponse{}, err
	}
	row, err := s.repo.InsertPending(ctx, req.Kind, domain.StoredAttachmentInsert{
		ParentID: req.ParentID, Name: req.Name, ContentType: req.ContentType,
		SizeBytes: req.SizeBytes, CreatedBy: email,
	})
	if err != nil {
		return domain.InitiateAttachmentUploadResponse{}, err
	}

	// One folder per upload, and a share on just that folder that accepts one
	// request: the browser can put one file there and reach nothing else.
	staging := filestore.StagingDir(row.ID)
	expires := s.now().Add(s.limits.UploadLinkTTL)
	shareID, err := s.openUploadShare(ctx, staging, expires)
	if err != nil {
		// The row would sit PENDING forever; record that it never got a link.
		s.failQuietly(ctx, req.Kind, row.ID, "upload link could not be created")
		return domain.InitiateAttachmentUploadResponse{}, storageUnavailable(ctx, "open upload share", err)
	}
	auditLink(ctx, "upload", req.Kind, row.ID, staging, shareID, expires)
	return domain.InitiateAttachmentUploadResponse{
		AttachmentID: row.ID,
		Number:       row.Number,
		UploadURL:    s.store.UploadURL(shareID),
		FieldName:    "filenames",
		ExpiresAt:    expires,
		MaxBytes:     limit,
	}, nil
}

func (s *storedAttachmentService) openUploadShare(ctx context.Context, staging string, expires time.Time) (string, error) {
	if err := s.store.MkdirAll(ctx, staging); err != nil {
		return "", err
	}
	return s.store.CreateShare(ctx, filestore.ShareSpec{
		Name: "up", Scope: filestore.ShareWrite, Path: staging, MaxUses: 1, ExpiresAt: expires,
	})
}

// ConfirmUpload implements StoredAttachmentService.
func (s *storedAttachmentService) ConfirmUpload(ctx context.Context, req domain.ConfirmAttachmentUploadRequest) (domain.ConfirmAttachmentUploadResponse, error) {
	if err := validateStoredAttachmentKind(req.Kind); err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, err
	}
	if !validate.IsUUID(req.ID) {
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "id must be a valid UUID"}
	}
	email, err := s.resolveActorEmail(ctx)
	if err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, err
	}

	rows, err := s.repo.GetByIDs(ctx, req.Kind, []string{req.ID})
	if err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, err
	}
	// Not visible to the caller, or not theirs: not found, never forbidden.
	if len(rows) != 1 || !strings.EqualFold(rows[0].CreatedBy, email) {
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.NotFoundError{Msg: "attachment not found"}
	}
	row := rows[0]

	switch row.Status {
	case "COMPLETE":
		// A confirm that is repeated (a retry after a timeout) answers the same.
		return confirmResponse(row, sizeOrZero(row.SizeBytes)), nil
	case "PENDING":
	default:
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.ConflictError{Msg: "this upload was rejected; start a new upload"}
	}

	staging := filestore.StagingDir(row.ID)

	parent, err := s.repo.GetParent(ctx, req.Kind, row.ParentID)
	if err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, err
	}
	// The item may have been closed since the upload began.
	if parent.Closed {
		return domain.ConfirmAttachmentUploadResponse{}, s.reject(ctx, req.Kind, row.ID, staging, closedParentMessage, &apierror.ConflictError{Msg: closedParentMessage})
	}

	key := filestore.Key(filestore.KeyParts{
		ProjectNumber:    parent.ProjectNumber,
		WorkItemNumber:   workItemNumberFor(req.Kind, parent),
		DeploymentNumber: deploymentNumberFor(req.Kind, parent),
		Inline:           req.Kind == domain.StoredAttachmentComment,
		AttachmentNumber: row.Number,
		Name:             row.Name,
	})

	file, err := s.receivedFile(ctx, staging)
	if err != nil {
		var verr *apierror.ValidationError
		if errors.As(err, &verr) {
			// A confirm that moved the file but failed to mark the row leaves staging
			// empty and the file at its final key: finish that job instead of
			// rejecting an upload that did arrive.
			if resp, done, ferr := s.completeMoved(ctx, req.Kind, row, key, staging); done {
				return resp, ferr
			}
			return domain.ConfirmAttachmentUploadResponse{}, s.reject(ctx, req.Kind, row.ID, staging, verr.Msg, verr)
		}
		return domain.ConfirmAttachmentUploadResponse{}, storageUnavailable(ctx, "list upload folder", err)
	}

	limit := s.limitFor(req.Kind)
	if file.Size > limit {
		msg := "the file is larger than the " + formatByteLimit(limit) + " limit"
		return domain.ConfirmAttachmentUploadResponse{}, s.reject(ctx, req.Kind, row.ID, staging, msg, &apierror.ValidationError{Msg: msg})
	}
	if row.SizeBytes != nil && file.Size != *row.SizeBytes {
		msg := "the uploaded file's size does not match the size declared"
		return domain.ConfirmAttachmentUploadResponse{}, s.reject(ctx, req.Kind, row.ID, staging, msg, &apierror.ValidationError{Msg: msg})
	}

	stagedPath := path.Join(staging, file.Name)
	if req.Kind == domain.StoredAttachmentComment {
		// The type the browser declared is only a claim; look at the bytes.
		head, err := s.store.ReadHead(ctx, stagedPath, sniffBytes)
		if err != nil {
			return domain.ConfirmAttachmentUploadResponse{}, storageUnavailable(ctx, "read upload", err)
		}
		if !inlineImageTypes[strings.ToLower(strings.SplitN(http.DetectContentType(head), ";", 2)[0])] {
			msg := "the file is not a PNG, JPEG, GIF or WebP image"
			return domain.ConfirmAttachmentUploadResponse{}, s.reject(ctx, req.Kind, row.ID, staging, msg, &apierror.ValidationError{Msg: msg})
		}
	}

	// The folder must exist before the file can be moved into it. A failure from
	// here on leaves the row PENDING and the file staged, so confirm can be retried.
	if err := s.store.MkdirAll(ctx, path.Dir(key)); err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, storageUnavailable(ctx, "create final folder", err)
	}
	if err := s.store.Rename(ctx, stagedPath, key); err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, storageUnavailable(ctx, "move upload into place", err)
	}

	// UPDATE on these tables is internal-only, so the write runs as the system
	// after the checks above (the caller owns this row, in a parent they can see).
	ok, err := s.repo.MarkComplete(repository.WithSystemIdentity(ctx), req.Kind, row.ID, key, file.Size)
	if err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, err
	}
	if !ok {
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.ConflictError{Msg: "this upload was already confirmed"}
	}
	s.removeStaging(ctx, staging)

	row.Status = "COMPLETE"
	return confirmResponse(row, file.Size), nil
}

// OpenInlineImage implements StoredAttachmentService. It is how a comment's
// image is shown: entity-service reads it from SFTPGo and streams it, so access is
// checked on every request and no link that could be forwarded ever exists.
//
// An image pasted into a comment here is a comment_attachment row. One migrated
// from the previous system is a work_item_attachment row (its comment images were
// stored as the work item's own attachments), so when there is no comment row the
// work item's own is tried, and streamed only when it really is an image.
func (s *storedAttachmentService) OpenInlineImage(ctx context.Context, id string) (domain.StoredAttachmentContent, error) {
	if !validate.IsUUID(id) {
		return domain.StoredAttachmentContent{}, &apierror.ValidationError{Msg: "id must be a valid UUID"}
	}
	// Only a row the caller may see comes back; anything else is not found.
	kind := domain.StoredAttachmentComment
	rows, err := s.repo.GetByIDs(ctx, kind, []string{id})
	if err != nil {
		return domain.StoredAttachmentContent{}, err
	}
	if len(rows) == 0 {
		kind = domain.StoredAttachmentWorkItem
		rows, err = s.repo.GetByIDs(ctx, kind, []string{id})
		if err != nil {
			return domain.StoredAttachmentContent{}, err
		}
	}
	notFound := &apierror.NotFoundError{Msg: "attachment not found"}
	if len(rows) != 1 {
		return domain.StoredAttachmentContent{}, notFound
	}
	row := rows[0]
	if row.Status != "COMPLETE" || row.StorageKey == nil || *row.StorageKey == "" {
		return domain.StoredAttachmentContent{}, notFound
	}
	// A migrated type may carry parameters ("image/png; charset=binary"); only the
	// media type decides, and only it is sent back.
	mediaType := ""
	if row.ContentType != nil {
		mediaType = strings.ToLower(strings.TrimSpace(strings.SplitN(*row.ContentType, ";", 2)[0]))
	}
	imageType := inlineImageTypes[mediaType]
	if kind == domain.StoredAttachmentWorkItem && !imageType {
		// Files of a work item are downloaded through a link, never shown here.
		return domain.StoredAttachmentContent{}, notFound
	}
	body, size, err := s.store.Open(ctx, *row.StorageKey)
	if err != nil {
		if errors.Is(err, filestore.ErrNotFound) {
			return domain.StoredAttachmentContent{}, notFound
		}
		return domain.StoredAttachmentContent{}, storageUnavailable(ctx, "open inline image", err)
	}
	slog.InfoContext(ctx, "attachment content served",
		"audit", true, "action", "view", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx), "kind", string(kind),
		"attachmentId", row.ID, "path", *row.StorageKey)
	contentType := "application/octet-stream"
	if imageType {
		contentType = mediaType
	}
	return domain.StoredAttachmentContent{Body: body, ContentType: contentType, Size: size}, nil
}

// inlineImageRef finds the comment images a comment's HTML shows: its <img src>
// ends in .../attachments/inline/<attachment id>/content.
var inlineImageRef = regexp.MustCompile(`attachments/inline/([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})/content`)

// inlineImageIDs returns the distinct attachment ids a comment's content refers
// to, at most maxAttachmentLinkIDs.
func inlineImageIDs(content string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, m := range inlineImageRef.FindAllStringSubmatch(content, -1) {
		id := strings.ToLower(m[1])
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
		if len(ids) == maxAttachmentLinkIDs {
			break
		}
	}
	return ids
}

// LinkFromContent implements InlineImageLinker.
//
// A pasted image is uploaded before its comment exists, so it starts unlinked
// (comment_id NULL, visible only to its uploader). When the comment is saved
// this finds the images its HTML shows and attaches them to it, so everyone who
// may read the comment can see them. Only the author's own stored images on the
// comment's own work item are linked, so a comment cannot claim someone else's
// image or one from another case. It is best effort: a failure is logged and the
// comment is not affected (the image then stays visible to its uploader only, and
// is removed by the upload cleanup after 24 hours).
func (s *storedAttachmentService) LinkFromContent(ctx context.Context, commentID, workItemID, authorEmail, content string) {
	ids := inlineImageIDs(content)
	if len(ids) == 0 || authorEmail == "" {
		return
	}
	linked, err := s.repo.LinkInlineImages(repository.WithSystemIdentity(ctx), commentID, workItemID, authorEmail, ids)
	if err != nil {
		slog.ErrorContext(ctx, "could not link comment images", "commentId", commentID, "error", err)
		return
	}
	slog.InfoContext(ctx, "comment images linked",
		"audit", true, "action", "link", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx),
		"commentId", commentID, "referenced", len(ids), "linked", len(linked))
}

// DeleteAttachment implements StoredAttachmentService.
//
// Who may delete is decided here, not by row-level security: UPDATE on these
// tables is internal-only, so the soft delete runs as the system after the checks
// below. For now only the person who uploaded an attachment may delete it, and
// never once the work item is closed. The file is removed from SFTPGo first and
// the row marked afterwards, so a failure in between leaves a COMPLETE row whose
// file is gone, and repeating the call finishes the job.
func (s *storedAttachmentService) DeleteAttachment(ctx context.Context, req domain.DeleteStoredAttachmentRequest) error {
	if err := validateStoredAttachmentKind(req.Kind); err != nil {
		return err
	}
	if req.Kind == domain.StoredAttachmentComment {
		return &apierror.ValidationError{Msg: "an image in a comment cannot be deleted on its own"}
	}
	if !validate.IsUUID(req.ID) {
		return &apierror.ValidationError{Msg: "id must be a valid UUID"}
	}
	email, err := s.resolveActorEmail(ctx)
	if err != nil {
		return err
	}

	// As the caller: an attachment they cannot see is not found.
	rows, err := s.repo.GetByIDs(ctx, req.Kind, []string{req.ID})
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return &apierror.NotFoundError{Msg: "attachment not found"}
	}
	row := rows[0]
	if !strings.EqualFold(row.CreatedBy, email) {
		return &apierror.ForbiddenError{Msg: "Only the person who uploaded an attachment can delete it."}
	}

	switch row.Status {
	case "DELETED":
		// A delete that is repeated (a retry after a timeout) answers the same.
		return nil
	case "COMPLETE":
	default:
		return &apierror.ConflictError{Msg: "this attachment is not stored, so it cannot be deleted"}
	}

	parent, err := s.repo.GetParent(ctx, req.Kind, row.ParentID)
	if err != nil {
		return err
	}
	if parent.Closed {
		return &apierror.ConflictError{Msg: closedParentDeleteMessage}
	}

	if row.StorageKey != nil && *row.StorageKey != "" {
		if err := s.store.RemoveFile(ctx, strings.TrimLeft(*row.StorageKey, "/")); err != nil && !errors.Is(err, filestore.ErrNotFound) {
			return storageUnavailable(ctx, "delete attachment file", err)
		}
	}
	ok, err := s.repo.MarkDeleted(repository.WithSystemIdentity(ctx), req.Kind, row.ID, email)
	if err != nil {
		return err
	}
	if !ok {
		// Another request deleted it first: the outcome the caller wanted.
		return nil
	}
	path := ""
	if row.StorageKey != nil {
		path = *row.StorageKey
	}
	slog.InfoContext(ctx, "attachment deleted",
		"audit", true, "action", "delete", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx), "kind", string(req.Kind),
		"attachmentId", row.ID, "path", path)
	return nil
}

// formatByteLimit renders a size limit for a message: "100 MB", or "512 KB"
// for a limit under one megabyte.
func formatByteLimit(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%d MB", n>>20)
	}
	return fmt.Sprintf("%d KB", n>>10)
}

// closedParentMessage is shown when a file is added to a closed work item.
const closedParentMessage = "This case is closed, so new attachments can't be added."

// closedParentDeleteMessage is shown when a file is deleted from a closed work item.
const closedParentDeleteMessage = "This case is closed, so attachments can't be deleted."

func (s *storedAttachmentService) limitFor(kind domain.StoredAttachmentKind) int64 {
	if kind == domain.StoredAttachmentComment {
		return s.limits.InlineMaxBytes
	}
	return s.limits.MaxBytes
}

// receivedFile finds the one file the browser uploaded into a staging folder.
// A *apierror.ValidationError means the folder holds no file, or more than one.
func (s *storedAttachmentService) receivedFile(ctx context.Context, staging string) (filestore.Entry, error) {
	entries, err := s.store.ListDir(ctx, staging)
	if err != nil {
		if errors.Is(err, filestore.ErrNotFound) {
			return filestore.Entry{}, &apierror.ValidationError{Msg: "no file was uploaded"}
		}
		return filestore.Entry{}, err
	}
	var files []filestore.Entry
	for _, e := range entries {
		// A dot-file is not the browser's: nothing it can upload keeps its name.
		if e.IsDir() || strings.HasPrefix(e.Name, ".") {
			continue
		}
		files = append(files, e)
	}
	switch len(files) {
	case 0:
		return filestore.Entry{}, &apierror.ValidationError{Msg: "no file was uploaded"}
	case 1:
		return files[0], nil
	}
	return filestore.Entry{}, &apierror.ValidationError{Msg: "more than one file was uploaded"}
}

// completeMoved finishes a confirm whose file was already moved to its final key
// (the earlier attempt failed between the move and marking the row). done is
// false when no file is there, so the caller rejects as before.
func (s *storedAttachmentService) completeMoved(ctx context.Context, kind domain.StoredAttachmentKind, row domain.StoredAttachmentRow, key, staging string) (domain.ConfirmAttachmentUploadResponse, bool, error) {
	entries, err := s.store.ListDir(ctx, path.Dir(key))
	if err != nil {
		if errors.Is(err, filestore.ErrNotFound) {
			return domain.ConfirmAttachmentUploadResponse{}, false, nil
		}
		// Not knowing is not the same as absent: a transient failure must not
		// reject an upload that may have arrived. The row stays PENDING for a retry.
		return domain.ConfirmAttachmentUploadResponse{}, true, storageUnavailable(ctx, "list final folder", err)
	}
	for _, e := range entries {
		if e.IsDir() || e.Name != path.Base(key) {
			continue
		}
		ok, err := s.repo.MarkComplete(repository.WithSystemIdentity(ctx), kind, row.ID, key, e.Size)
		if err != nil {
			return domain.ConfirmAttachmentUploadResponse{}, true, err
		}
		if !ok {
			return domain.ConfirmAttachmentUploadResponse{}, true, &apierror.ConflictError{Msg: "this upload was already confirmed"}
		}
		s.removeStaging(ctx, staging)
		row.Status = "COMPLETE"
		return confirmResponse(row, e.Size), true, nil
	}
	return domain.ConfirmAttachmentUploadResponse{}, false, nil
}

// reject throws an upload away: the staged file is removed and the row is
// recorded as failed, so it cannot be confirmed later. It returns cause for the
// caller to hand back.
func (s *storedAttachmentService) reject(ctx context.Context, kind domain.StoredAttachmentKind, id, staging, reason string, cause error) error {
	s.removeStaging(ctx, staging)
	s.failQuietly(ctx, kind, id, reason)
	return cause
}

// failQuietly marks a row FAILED as the system. A failure to do so is logged
// only: the caller is already being given a more useful error.
func (s *storedAttachmentService) failQuietly(ctx context.Context, kind domain.StoredAttachmentKind, id, reason string) {
	if err := s.repo.MarkFailed(repository.WithSystemIdentity(ctx), kind, id, reason); err != nil {
		slog.ErrorContext(ctx, "could not mark attachment failed", "attachmentId", id, "error", err)
	}
}

// removeStaging deletes an upload folder, best effort: the file is either
// already moved or being thrown away, and a leftover folder is harmless.
func (s *storedAttachmentService) removeStaging(ctx context.Context, staging string) {
	if err := s.store.RemoveDir(ctx, staging); err != nil && !errors.Is(err, filestore.ErrNotFound) {
		slog.WarnContext(ctx, "could not remove upload folder", "folder", staging, "error", err)
	}
}

func workItemNumberFor(kind domain.StoredAttachmentKind, p domain.StoredAttachmentParent) string {
	if kind == domain.StoredAttachmentDeployment {
		return ""
	}
	return p.Number
}

func deploymentNumberFor(kind domain.StoredAttachmentKind, p domain.StoredAttachmentParent) string {
	if kind == domain.StoredAttachmentDeployment {
		return p.Number
	}
	return ""
}

func sizeOrZero(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}

func confirmResponse(row domain.StoredAttachmentRow, size int64) domain.ConfirmAttachmentUploadResponse {
	return domain.ConfirmAttachmentUploadResponse{
		ID: row.ID, Number: row.Number, Name: row.Name, ContentType: row.ContentType,
		SizeBytes: size, Status: row.Status,
	}
}
