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
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

const (
	// maxAttachmentLinkIDs bounds how many inline images of one comment are
	// linked to it.
	maxAttachmentLinkIDs = 20
	// maxSharesPerCallerPerMinute is how many uploads and locates one caller may
	// start per minute; beyond it the call is a 429.
	maxSharesPerCallerPerMinute = 60
	// maxAttachmentNameLen is the width of the name column (VARCHAR(255)).
	maxAttachmentNameLen = 255
)

// inlineImageTypes are the only types a comment image may have. SVG is left
// out on purpose: it can carry script.
var inlineImageTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

// newInlineImageTypes are the only types an image added to a comment may have.
// SVG is left out on purpose (it can carry script), and so is GIF. Images already
// stored with another type are still shown (inlineImageTypes).
var newInlineImageTypes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

// StoredAttachmentLimits are the settings the service runs under.
type StoredAttachmentLimits struct {
	// MaxBytes caps one file; InlineMaxBytes caps one comment image.
	MaxBytes       int64
	InlineMaxBytes int64
}

// storedAttachmentService never touches a file: the portal backends hold the
// files in Azure Blob Storage. It checks access, records the attachment rows and
// hands each backend the storage key to use.
type storedAttachmentService struct {
	repo   repository.StoredAttachmentRepository
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
func NewStoredAttachmentService(repo repository.StoredAttachmentRepository, limits StoredAttachmentLimits) StoredAttachmentService {
	return &storedAttachmentService{repo: repo, limits: limits, now: time.Now, shares: newShareLimiter(maxSharesPerCallerPerMinute, time.Minute)}
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
		if _, ok := newInlineImageTypes[req.ContentType]; !ok {
			return domain.InitiateAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "only PNG, JPEG and WebP images can be added to a comment"}
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

	// The portal backend holds the file: nothing to open here. It asks to
	// authorize each chunk, so the row is all that is recorded.
	slog.InfoContext(ctx, "attachment upload started",
		"audit", true, "action", "upload", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx), "kind", string(req.Kind),
		"attachmentId", row.ID)
	return domain.InitiateAttachmentUploadResponse{
		AttachmentID: row.ID,
		Number:       row.Number,
		ExpiresAt:    s.now().Add(abandonedUploadAge),
		MaxBytes:     limit,
	}, nil
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
	return s.confirmFromBackend(ctx, req, row)
}

// inlineImage is the stored row an inline image id names, with the type it is
// served as.
type inlineImage struct {
	kind      domain.StoredAttachmentKind
	row       domain.StoredAttachmentRow
	mediaType string
	isImage   bool
}

// findInlineImage resolves the id of a comment's inline image to a stored row the
// caller may see, for OpenInlineImage and LocateInlineImage alike.
//
// An image pasted into a comment here is a comment_attachment row. One migrated
// from the previous system is a work_item_attachment row (its comment images were
// stored as the work item's own attachments), so when there is no comment row the
// work item's own is tried, and used only when it really is an image.
func (s *storedAttachmentService) findInlineImage(ctx context.Context, id string) (inlineImage, error) {
	if !validate.IsUUID(id) {
		return inlineImage{}, &apierror.ValidationError{Msg: "id must be a valid UUID"}
	}
	// Only a row the caller may see comes back; anything else is not found.
	kind := domain.StoredAttachmentComment
	rows, err := s.repo.GetByIDs(ctx, kind, []string{id})
	if err != nil {
		return inlineImage{}, err
	}
	if len(rows) == 0 {
		kind = domain.StoredAttachmentWorkItem
		rows, err = s.repo.GetByIDs(ctx, kind, []string{id})
		if err != nil {
			return inlineImage{}, err
		}
	}
	notFound := &apierror.NotFoundError{Msg: "attachment not found"}
	if len(rows) != 1 {
		return inlineImage{}, notFound
	}
	row := rows[0]
	if row.Status == "DELETED" {
		// The caller could see this row, so saying it was removed reveals nothing.
		return inlineImage{}, &apierror.GoneError{Msg: "this image was removed", Code: apierror.CodeAttachmentRemoved}
	}
	if row.Status != "COMPLETE" || row.StorageKey == nil || *row.StorageKey == "" {
		return inlineImage{}, notFound
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
		return inlineImage{}, notFound
	}
	return inlineImage{kind: kind, row: row, mediaType: mediaType, isImage: imageType}, nil
}

// inlineImageRef finds the comment images a comment's HTML shows: its <img src>
// ends in .../attachments/inline/<attachment id>/content, or is the bare
// reference src="/<attachment id>" that ExtractInlineImages writes (and that
// comments migrated from the previous system use; the .iix suffix is tolerated).
var inlineImageRef = regexp.MustCompile(`(?i)(?:attachments/inline/|\bsrc\s*=\s*["']/?)([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:/content|\.iix|["'])`)

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
// never once the work item is closed. Only the row is marked here; the portal
// backend removes the file afterwards.
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

	// The file is not here to remove: the portal backend removes it from storage
	// after this succeeds.
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

// failQuietly marks a row FAILED as the system. A failure to do so is logged
// only: the caller is already being given a more useful error.
func (s *storedAttachmentService) failQuietly(ctx context.Context, kind domain.StoredAttachmentKind, id, reason string) {
	if err := s.repo.MarkFailed(repository.WithSystemIdentity(ctx), kind, id, reason); err != nil {
		slog.ErrorContext(ctx, "could not mark attachment failed", "attachmentId", id, "error", err)
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
