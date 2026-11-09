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
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/filestore"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// storageKeyFor is the key an attachment's file has once it is stored: the
// migrator's layout, so new files sit beside migrated ones.
//
// A draft description image has no work item yet, so a "drafts" folder stands in
// for the work item number; the key stays as it is when the image is linked.
func storageKeyFor(kind domain.StoredAttachmentKind, row domain.StoredAttachmentRow, parent domain.StoredAttachmentParent) string {
	workItem := workItemNumberFor(kind, parent)
	if row.Draft {
		workItem = draftFolder
	}
	return filestore.Key(filestore.KeyParts{
		ProjectNumber:    parent.ProjectNumber,
		WorkItemNumber:   workItem,
		DeploymentNumber: deploymentNumberFor(kind, parent),
		Inline:           kind == domain.StoredAttachmentComment,
		AttachmentNumber: row.Number,
		Name:             row.Name,
	})
}

// parentOf reads what a row hangs from, as the caller: its work item or
// deployment, or for a draft description image its project.
func (s *storedAttachmentService) parentOf(ctx context.Context, kind domain.StoredAttachmentKind, row domain.StoredAttachmentRow) (domain.StoredAttachmentParent, error) {
	if row.Draft {
		return s.repo.GetProject(ctx, row.ParentID)
	}
	return s.repo.GetParent(ctx, kind, row.ParentID)
}

// AuthorizeUpload implements StoredAttachmentService.
//
// The portal backend calls it for every chunk of an upload it is receiving, so the
// caller's access is checked again each time: someone whose access was removed
// part-way stops being able to add to the upload. It is cheap (one row read and
// one parent read) and charges nothing against the per-caller link budget.
func (s *storedAttachmentService) AuthorizeUpload(ctx context.Context, req domain.AuthorizeAttachmentUploadRequest) (domain.AttachmentUploadTarget, error) {
	if err := validateStoredAttachmentKind(req.Kind); err != nil {
		return domain.AttachmentUploadTarget{}, err
	}
	if !validate.IsUUID(req.ID) {
		return domain.AttachmentUploadTarget{}, &apierror.ValidationError{Msg: "id must be a valid UUID"}
	}
	email, err := s.resolveActorEmail(ctx)
	if err != nil {
		return domain.AttachmentUploadTarget{}, err
	}

	rows, err := s.repo.GetByIDs(ctx, req.Kind, []string{req.ID})
	if err != nil {
		return domain.AttachmentUploadTarget{}, err
	}
	// Not visible to the caller, or not theirs: not found, never forbidden.
	if len(rows) != 1 || !strings.EqualFold(rows[0].CreatedBy, email) {
		return domain.AttachmentUploadTarget{}, &apierror.NotFoundError{Msg: "attachment not found"}
	}
	row := rows[0]
	switch row.Status {
	case "PENDING":
	case "COMPLETE":
		return domain.AttachmentUploadTarget{}, &apierror.ConflictError{Msg: alreadyConfirmedMessage}
	default:
		return domain.AttachmentUploadTarget{}, &apierror.ConflictError{Msg: "this upload was rejected; start a new upload"}
	}

	parent, err := s.parentOf(ctx, req.Kind, row)
	if err != nil {
		return domain.AttachmentUploadTarget{}, err
	}
	// The item may have been closed since the upload began. Confirm rejects it;
	// here the backend just stops receiving.
	if parent.Closed {
		return domain.AttachmentUploadTarget{}, &apierror.ConflictError{Msg: closedParentMessage}
	}

	contentType := ""
	if row.ContentType != nil {
		contentType = *row.ContentType
	}
	return domain.AttachmentUploadTarget{
		AttachmentID: row.ID,
		Number:       row.Number,
		Name:         row.Name,
		ContentType:  contentType,
		SizeBytes:    sizeOrZero(row.SizeBytes),
		MaxBytes:     s.limitFor(req.Kind),
		StorageKey:   storageKeyFor(req.Kind, row, parent),
	}, nil
}

// confirmFromBackend finishes an upload in backend mode. The portal backend has
// already committed the file to storage and reports what it stored; this service
// cannot look, so it checks the report against what the upload declared and the
// limits, and marks the row COMPLETE. When it refuses, the row is recorded FAILED
// and the backend removes the file it committed.
func (s *storedAttachmentService) confirmFromBackend(ctx context.Context, req domain.ConfirmAttachmentUploadRequest, row domain.StoredAttachmentRow) (domain.ConfirmAttachmentUploadResponse, error) {
	parent, err := s.parentOf(ctx, req.Kind, row)
	if err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, err
	}
	// The item may have been closed since the upload began.
	if parent.Closed {
		s.failQuietly(ctx, req.Kind, row.ID, closedParentMessage)
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.ConflictError{Msg: closedParentMessage}
	}

	if req.SizeBytes == nil || *req.SizeBytes <= 0 {
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.ValidationError{Msg: "sizeBytes is required"}
	}
	size := *req.SizeBytes
	if limit := s.limitFor(req.Kind); size > limit {
		msg := "the file is larger than the " + formatByteLimit(limit) + " limit"
		s.failQuietly(ctx, req.Kind, row.ID, msg)
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.ValidationError{Msg: msg}
	}
	if row.SizeBytes != nil && size != *row.SizeBytes {
		msg := "the uploaded file's size does not match the size declared"
		s.failQuietly(ctx, req.Kind, row.ID, msg)
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.ValidationError{Msg: msg}
	}
	if req.Kind == domain.StoredAttachmentComment {
		// The type the browser declared is only a claim; the backend read the bytes.
		reported := strings.ToLower(strings.TrimSpace(strings.SplitN(req.ContentType, ";", 2)[0]))
		if _, ok := newInlineImageTypes[reported]; !ok {
			msg := "the file is not a PNG, JPEG or WebP image"
			s.failQuietly(ctx, req.Kind, row.ID, msg)
			return domain.ConfirmAttachmentUploadResponse{}, &apierror.ValidationError{Msg: msg}
		}
	}

	key := storageKeyFor(req.Kind, row, parent)
	// UPDATE on these tables is internal-only, so the write runs as the system
	// after the checks above (the caller owns this row, in a parent they can see).
	ok, err := s.repo.MarkComplete(repository.WithSystemIdentity(ctx), req.Kind, row.ID, key, size)
	if err != nil {
		return domain.ConfirmAttachmentUploadResponse{}, err
	}
	if !ok {
		return domain.ConfirmAttachmentUploadResponse{}, &apierror.ConflictError{Msg: alreadyConfirmedMessage}
	}
	slog.InfoContext(ctx, "attachment upload confirmed",
		"audit", true, "action", "confirm", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx), "kind", string(req.Kind),
		"attachmentId", row.ID, "path", key, "sizeBytes", size)

	// Only here, on the confirm that completed the upload: a repeated confirm
	// ("already COMPLETE") returns earlier and must not replay the move, which
	// could undo a later staff move back to Awaiting Info. Best-effort: the
	// attachment is committed, so a failure is logged by the rule, never returned.
	if req.Kind == domain.StoredAttachmentWorkItem && s.caseStates != nil {
		s.caseStates.ApplyCustomerAttachmentStateTransition(ctx, row.ParentID)
	}

	row.Status = "COMPLETE"
	return confirmResponse(row, size), nil
}

// LocateAttachment implements StoredAttachmentService.
func (s *storedAttachmentService) LocateAttachment(ctx context.Context, req domain.LocateAttachmentRequest) (domain.AttachmentLocation, error) {
	if err := validateStoredAttachmentKind(req.Kind); err != nil {
		return domain.AttachmentLocation{}, err
	}
	if !validate.IsUUID(req.ID) {
		return domain.AttachmentLocation{}, &apierror.ValidationError{Msg: "id must be a valid UUID"}
	}

	// Only a row the caller may see comes back; anything else is not found.
	rows, err := s.repo.GetByIDs(ctx, req.Kind, []string{req.ID})
	if err != nil {
		return domain.AttachmentLocation{}, err
	}
	if len(rows) != 1 {
		return domain.AttachmentLocation{}, &apierror.NotFoundError{Msg: "attachment not found"}
	}
	row := rows[0]

	// A read is a request to storage, so it is charged like a link used to be.
	if err := s.reserveShares(ctx, 1); err != nil {
		return domain.AttachmentLocation{}, err
	}

	loc := domain.AttachmentLocation{
		ID: row.ID, Number: row.Number, Name: row.Name,
		ContentType: row.ContentType, SizeBytes: row.SizeBytes,
	}
	switch {
	case row.Status == "DELETED":
		loc.Reason = domain.AttachmentUnavailableDeleted
		return loc, nil
	case row.Status == "FAILED":
		loc.Reason = domain.AttachmentUnavailableFailed
		return loc, nil
	case row.Status != "COMPLETE" || row.StorageKey == nil || *row.StorageKey == "":
		loc.Reason = domain.AttachmentUnavailableNotStored
		return loc, nil
	}
	loc.Available = true
	loc.StorageKey = *row.StorageKey
	slog.InfoContext(ctx, "attachment located",
		"audit", true, "action", "download", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx), "kind", string(req.Kind),
		"attachmentId", row.ID, "path", *row.StorageKey)
	return loc, nil
}

// LocateInlineImage implements StoredAttachmentService. It finds a comment's
// image: the same lookup and refusals as any attachment, and the storage key for
// the backend to stream from. It is not charged, like streaming an image never was.
func (s *storedAttachmentService) LocateInlineImage(ctx context.Context, id string) (domain.AttachmentLocation, error) {
	img, err := s.findInlineImage(ctx, id)
	if err != nil {
		return domain.AttachmentLocation{}, err
	}
	row := img.row
	// Only the four image types are ever sent as an image; anything else is
	// octet-stream, as in OpenInlineImage.
	contentType := "application/octet-stream"
	if img.isImage {
		contentType = img.mediaType
	}
	slog.InfoContext(ctx, "attachment located",
		"audit", true, "action", "view", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx), "kind", string(img.kind),
		"attachmentId", row.ID, "path", *row.StorageKey)
	// Moving the file is an extra: a failed lookup of where it belongs must not
	// stop the image being shown from where it is, so it is logged and left out.
	relocateTo, err := s.relocationFor(ctx, img)
	if err != nil {
		slog.WarnContext(ctx, "could not work out where an inline image should move to; serving it where it is",
			"attachmentId", row.ID, "correlationId", middleware.CorrelationIDFromContext(ctx), "error", err)
		relocateTo = ""
	}
	return domain.AttachmentLocation{
		ID: row.ID, Number: row.Number, Name: row.Name,
		ContentType: &contentType, SizeBytes: row.SizeBytes,
		Available: true, StorageKey: *row.StorageKey, RelocateTo: relocateTo,
		ParentID: row.ParentID, Draft: row.Draft,
	}, nil
}

// draftKeyMarker is what a description image's key contains while its file is
// still where it was uploaded, before its record existed.
var draftKeyMarker = "/" + "workitems" + "/" + draftFolder + "/"

// relocationFor returns the key a linked description image's file belongs under,
// or "" when it is already there (or is not such an image). A description image is
// uploaded to the project's drafts folder because its record has no number yet;
// once it is linked to a work item it belongs with that work item's files.
func (s *storedAttachmentService) relocationFor(ctx context.Context, img inlineImage) (string, error) {
	row := img.row
	if img.kind != domain.StoredAttachmentComment || row.Draft || row.StorageKey == nil ||
		!strings.Contains(*row.StorageKey, draftKeyMarker) {
		return "", nil
	}
	parent, err := s.repo.GetParent(ctx, img.kind, row.ParentID)
	if err != nil {
		return "", err
	}
	key := storageKeyFor(img.kind, row, parent)
	if key == *row.StorageKey {
		return "", nil
	}
	return key, nil
}

// InlineImageRelocated implements StoredAttachmentService.
//
// The backend has copied the file to the key LocateInlineImage named and removed
// the old one; this makes the row point at it. Only the key this service itself
// computes is accepted, so a caller cannot point an image at some other file. It
// runs as the system after the checks (UPDATE is internal-only), and a repeated
// call answers the same.
func (s *storedAttachmentService) InlineImageRelocated(ctx context.Context, id string, req domain.InlineImageRelocatedRequest) error {
	img, err := s.findInlineImage(ctx, id)
	if err != nil {
		return err
	}
	want, err := s.relocationFor(ctx, img)
	if err != nil {
		return err
	}
	if want == "" {
		// Already moved (a repeated call), or not an image that moves.
		if img.row.StorageKey != nil && *img.row.StorageKey == req.StorageKey {
			return nil
		}
		return &apierror.ConflictError{Msg: "this image does not need to be moved"}
	}
	if req.StorageKey != want {
		return &apierror.ValidationError{Msg: "storageKey is not where this image belongs"}
	}
	from := *img.row.StorageKey
	ok, err := s.repo.UpdateStorageKey(repository.WithSystemIdentity(ctx), img.kind, img.row.ID, from, want)
	if err != nil {
		return err
	}
	if ok {
		slog.InfoContext(ctx, "inline image relocated",
			"audit", true, "action", "relocate", "callerId", auditCaller(ctx),
			"correlationId", middleware.CorrelationIDFromContext(ctx),
			"attachmentId", img.row.ID, "from", from, "path", want)
	}
	return nil
}
