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
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/errgroup"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/filestore"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

const (
	// maxAttachmentLinkIDs bounds one POST /attachments/download call.
	maxAttachmentLinkIDs = 100
	// attachmentLinkConcurrency bounds how many shares one call creates at once.
	attachmentLinkConcurrency = 8
	// maxAttachmentNameLen is the width of the name column (VARCHAR(255)).
	maxAttachmentNameLen = 255
	// readLinkMaxUses is how many requests a download link serves. More than
	// one on purpose: a browser can fetch one image or PDF preview in several
	// requests (a retry, a range request), and a link with a single use would
	// break it. The short expiry is what bounds a leaked link.
	readLinkMaxUses = 10
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
	RemoveDir(ctx context.Context, dir string) error
	ReadHead(ctx context.Context, path string, n int) ([]byte, error)
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
}

// NewStoredAttachmentService constructs a StoredAttachmentService.
//
// There is no AccessService here on purpose. Who may see an attachment, or add
// one, is decided by row-level security on the attachment tables (migration
// 100026) plus the parent lookup, both run as the caller: an external caller
// needs to be a REGISTERED member of the parent's project, an internal one is
// unrestricted, and a parent with no project is internal-only.
func NewStoredAttachmentService(repo repository.StoredAttachmentRepository, store fileStore, limits StoredAttachmentLimits) StoredAttachmentService {
	return &storedAttachmentService{repo: repo, store: store, limits: limits, now: time.Now}
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

// auditActorEmail is the caller's email for the audit log, or "" when it cannot be
// read. It never fails a request: the access decision does not depend on it.
func auditActorEmail(ctx context.Context) string {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return ""
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return ""
	}
	return email
}

// auditLink records who was given a link to which attachment. SFTPGo only sees
// the one service user for every transfer, so this line (with the share id,
// which SFTPGo's own log also carries) is how a download or an upload is tied
// to a person. The access log's correlation id and callerId sit beside it.
func auditLink(ctx context.Context, action, actor string, kind domain.StoredAttachmentKind, attachmentID, shareID string, expires time.Time) {
	slog.InfoContext(ctx, "attachment link created",
		"audit", true, "action", action, "actor", actor, "kind", string(kind),
		"attachmentId", attachmentID, "shareId", shareID, "expiresAt", expires)
}

// ResolveLinks implements StoredAttachmentService.
func (s *storedAttachmentService) ResolveLinks(ctx context.Context, req domain.AttachmentLinksRequest) (domain.AttachmentLinksResponse, error) {
	if err := validateStoredAttachmentKind(req.Kind); err != nil {
		return domain.AttachmentLinksResponse{}, err
	}
	if len(req.IDs) == 0 {
		return domain.AttachmentLinksResponse{}, &apierror.ValidationError{Msg: "ids is required"}
	}
	if len(req.IDs) > maxAttachmentLinkIDs {
		return domain.AttachmentLinksResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("ids cannot exceed %d entries", maxAttachmentLinkIDs)}
	}
	if err := validateUUIDs("ids", req.IDs); err != nil {
		return domain.AttachmentLinksResponse{}, err
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

	actor := auditActorEmail(ctx)
	links := make(map[string]domain.AttachmentLink, len(rows))
	var g errgroup.Group
	g.SetLimit(attachmentLinkConcurrency)
	results := make(chan domain.AttachmentLink, len(rows))
	for _, row := range rows {
		g.Go(func() error {
			results <- s.linkFor(ctx, actor, req.Kind, row)
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
func (s *storedAttachmentService) linkFor(ctx context.Context, actor string, kind domain.StoredAttachmentKind, row domain.StoredAttachmentRow) domain.AttachmentLink {
	l := domain.AttachmentLink{
		ID: row.ID, Number: row.Number, Name: row.Name,
		ContentType: row.ContentType, SizeBytes: row.SizeBytes,
	}
	switch {
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
		MaxUses: readLinkMaxUses, ExpiresAt: expires,
	})
	if err != nil {
		slog.ErrorContext(ctx, "attachment storage call failed", "operation", "create read share", "attachmentId", row.ID, "error", err)
		l.Reason = domain.AttachmentUnavailableStorage
		return l
	}
	auditLink(ctx, "download", actor, kind, row.ID, shareID, expires)
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
	auditLink(ctx, "upload", email, req.Kind, row.ID, shareID, expires)
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

	file, err := s.receivedFile(ctx, staging)
	if err != nil {
		var verr *apierror.ValidationError
		if errors.As(err, &verr) {
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

	key := filestore.Key(filestore.KeyParts{
		ProjectNumber:    parent.ProjectNumber,
		WorkItemNumber:   workItemNumberFor(req.Kind, parent),
		DeploymentNumber: deploymentNumberFor(req.Kind, parent),
		Inline:           req.Kind == domain.StoredAttachmentComment,
		AttachmentNumber: row.Number,
		Name:             row.Name,
	})
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
