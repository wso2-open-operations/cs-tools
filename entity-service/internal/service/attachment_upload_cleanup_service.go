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
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

const (
	// abandonedUploadAge is how old a PENDING upload must be to count as
	// abandoned. A chunked upload of a large file can run for hours on a slow
	// link, so this is long.
	abandonedUploadAge = 24 * time.Hour
	// uploadCleanupMaxPerRun bounds one run; a larger backlog is worked off over
	// several runs.
	uploadCleanupMaxPerRun = 200
)

// unlinkedImageAge is how long a stored comment image may stay unlinked before it
// is removed: the time to write a comment, with room to spare.
const unlinkedImageAge = 24 * time.Hour

const (
	// reportDraftUsableAge is how old a security report's draft file may be and
	// still be used to create a case.
	reportDraftUsableAge = 24 * time.Hour
	// reportDraftCleanupAge is when an unlinked draft is handed over for removal.
	// It is an hour past reportDraftUsableAge, so a draft is never linked to a
	// case after its file was handed over to be deleted.
	reportDraftCleanupAge = reportDraftUsableAge + time.Hour
)

type attachmentUploadCleanupService struct {
	repo   repository.StoredAttachmentRepository
	access AccessService
	now    func() time.Time
	// store removes a file this service owns. Nil (no storage configured) means an
	// unlinked file is handed to the caller in Orphans instead.
	store FileStore
}

// WithFileStore lets the cleanup remove an unlinked file from storage itself and mark
// the row DELETED in the same run, so no caller has to delete files and acknowledge
// them. A file that cannot be removed now keeps its row COMPLETE and goes to the back
// of the queue, so the next run tries it again. Without a store the cleanup hands the
// file over in Orphans, as before.
func WithFileStore(c AttachmentUploadCleanupService, store FileStore) AttachmentUploadCleanupService {
	if s, ok := c.(*attachmentUploadCleanupService); ok && store != nil {
		s.store = store
	}
	return c
}

// removeFile deletes an unlinked file from storage and marks its row DELETED. It
// reports whether the row is done: false means the file could not be removed now
// (the row stays COMPLETE and is tried again on a later run).
func (s *attachmentUploadCleanupService) removeFile(ctx, sys context.Context, kind domain.StoredAttachmentKind, id, key, by string) bool {
	if err := s.store.Delete(ctx, key); err != nil && !errors.Is(err, azblobstore.ErrNotFound) {
		slog.WarnContext(ctx, "could not remove an unlinked file from storage", "attachmentId", id, "error", err)
		s.deferRetry(ctx, sys, kind, id)
		return false
	}
	// A file that is already gone counts as removed: the row is then simply marked.
	if _, err := s.repo.MarkDeleted(sys, kind, id, by); err != nil {
		slog.WarnContext(ctx, "could not mark a removed file's row deleted", "attachmentId", id, "error", err)
		s.deferRetry(ctx, sys, kind, id)
		return false
	}
	return true
}

// NewAttachmentUploadCleanupService constructs an AttachmentUploadCleanupService.
// This service holds no files, so it marks abandoned rows FAILED and hands the
// unlinked comment images to the caller to remove
// (domain.UploadCleanupResult.Orphans), marking them DELETED only when the caller
// acknowledges them (AcknowledgeOrphans). An upload that was never confirmed
// leaves nothing in storage to remove (uncommitted blocks expire by themselves).
func NewAttachmentUploadCleanupService(repo repository.StoredAttachmentRepository, access AccessService) AttachmentUploadCleanupService {
	return &attachmentUploadCleanupService{repo: repo, access: access, now: time.Now}
}

// CleanupUploads implements AttachmentUploadCleanupService.
//
// An upload that was started and never confirmed leaves a PENDING row. This marks
// it FAILED (which every list already hides and a later confirm refuses). A row
// that cannot be marked stays PENDING, so the next run tries it again.
func (s *attachmentUploadCleanupService) CleanupUploads(ctx context.Context) (domain.UploadCleanupResult, error) {
	if err := RequireInternalCaller(ctx, s.access, "upload cleanup is only available to internal services"); err != nil {
		return domain.UploadCleanupResult{}, err
	}
	sys := repository.WithSystemIdentity(ctx)
	abandoned, err := s.repo.ListAbandoned(sys, s.now().Add(-abandonedUploadAge), uploadCleanupMaxPerRun)
	if err != nil {
		return domain.UploadCleanupResult{}, err
	}
	res := domain.UploadCleanupResult{Found: len(abandoned)}
	for _, u := range abandoned {
		if !s.discardAbandonedFiles(ctx, sys, u) {
			res.Failed++
			s.deferRetry(ctx, sys, u.Kind, u.ID)
			continue
		}
		if err := s.repo.MarkFailed(sys, u.Kind, u.ID, "upload abandoned: never confirmed"); err != nil {
			res.Failed++
			slog.WarnContext(ctx, "could not mark an abandoned upload failed", "attachmentId", u.ID, "error", err)
			s.deferRetry(ctx, sys, u.Kind, u.ID)
			continue
		}
		res.Cleaned++
	}
	// Comment images that were uploaded and stored but never linked to a comment.
	unlinked, err := s.repo.ListUnlinkedImages(sys, s.now().Add(-unlinkedImageAge), uploadCleanupMaxPerRun)
	if err != nil {
		return res, err
	}
	for _, img := range unlinked {
		hasFile := img.StorageKey != nil && *img.StorageKey != ""
		if hasFile && s.store != nil {
			if s.removeFile(ctx, sys, domain.StoredAttachmentComment, img.ID, *img.StorageKey, "inline-image-cleanup") {
				res.ImagesRemoved++
			} else {
				res.ImagesFailed++
			}
			continue
		}
		if hasFile {
			// Not marked DELETED here: this service cannot remove the file, and a
			// DELETED row is never listed again, so a key in a lost response would be
			// lost with it. The row stays COMPLETE and unlinked, so every run lists it
			// again until the caller acknowledges it.
			res.Orphans = append(res.Orphans, domain.OrphanedImage{ID: img.ID, Type: domain.StoredAttachmentComment, StorageKey: *img.StorageKey})
			continue
		}
		if _, err := s.repo.MarkDeleted(sys, domain.StoredAttachmentComment, img.ID, "inline-image-cleanup"); err != nil {
			res.ImagesFailed++
			slog.WarnContext(ctx, "could not mark an unlinked comment image deleted", "attachmentId", img.ID, "error", err)
			s.deferRetry(ctx, sys, domain.StoredAttachmentComment, img.ID)
			continue
		}
		res.ImagesRemoved++
	}
	// A security report's files that were uploaded and never linked to a case: the
	// form was abandoned, or the create was refused. Handled like the images above.
	drafts, err := s.repo.ListUnlinkedReportDrafts(sys, s.now().Add(-reportDraftCleanupAge), uploadCleanupMaxPerRun)
	if err != nil {
		return res, err
	}
	for _, d := range drafts {
		if d.StorageKey != nil && *d.StorageKey != "" && s.store != nil {
			if s.removeFile(ctx, sys, domain.StoredAttachmentWorkItem, d.ID, *d.StorageKey, "report-draft-cleanup") {
				res.ReportDraftsRemoved++
			} else {
				res.ReportDraftsFailed++
			}
			continue
		}
		if d.StorageKey != nil && *d.StorageKey != "" {
			res.Orphans = append(res.Orphans, domain.OrphanedImage{ID: d.ID, Type: domain.StoredAttachmentWorkItem, StorageKey: *d.StorageKey})
			continue
		}
		if _, err := s.repo.MarkDeleted(sys, domain.StoredAttachmentWorkItem, d.ID, "report-draft-cleanup"); err != nil {
			res.ReportDraftsFailed++
			slog.WarnContext(ctx, "could not mark an unlinked report draft deleted", "attachmentId", d.ID, "error", err)
			s.deferRetry(ctx, sys, domain.StoredAttachmentWorkItem, d.ID)
			continue
		}
		res.ReportDraftsRemoved++
	}
	slog.InfoContext(ctx, "attachment upload cleanup",
		"audit", true, "action", "upload-cleanup", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx),
		"found", res.Found, "cleaned", res.Cleaned, "failed", res.Failed,
		"imagesRemoved", res.ImagesRemoved, "imagesFailed", res.ImagesFailed,
		"reportDraftsRemoved", res.ReportDraftsRemoved, "reportDraftsFailed", res.ReportDraftsFailed,
		"orphans", len(res.Orphans))
	return res, nil
}

// discardAbandonedFiles removes what an abandoned upload left in storage: the
// chunks put under its staging key, and anything at its own key (a copy made by a
// confirm that was never recorded). It reports whether the upload may be marked
// FAILED: false means the files could not be removed now, and the upload is tried
// again on a later run. Without a store there is nothing this service can remove,
// and a key it cannot work out is logged and left to expire on its own.
func (s *attachmentUploadCleanupService) discardAbandonedFiles(ctx, sys context.Context, u domain.AbandonedUpload) bool {
	if s.store == nil {
		return true
	}
	key, err := s.abandonedKey(sys, u)
	if err != nil {
		slog.WarnContext(ctx, "could not work out where an abandoned upload's files are", "attachmentId", u.ID, "error", err)
		return true
	}
	for _, k := range []string{key + stagingSuffix, key} {
		if err := s.store.Discard(ctx, k); err != nil {
			slog.WarnContext(ctx, "could not remove an abandoned upload's files", "attachmentId", u.ID, "error", err)
			return false
		}
	}
	return true
}

// abandonedKey is the key the file of an abandoned upload would have had, worked
// out the way the upload did.
func (s *attachmentUploadCleanupService) abandonedKey(sys context.Context, u domain.AbandonedUpload) (string, error) {
	rows, err := s.repo.GetByIDs(sys, u.Kind, []string{u.ID})
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("the upload row is gone")
	}
	row := rows[0]
	var parent domain.StoredAttachmentParent
	if row.Draft {
		parent, err = s.repo.GetProject(sys, row.ParentID)
	} else {
		parent, err = s.repo.GetParent(sys, u.Kind, row.ParentID)
	}
	if err != nil {
		return "", err
	}
	return storageKeyFor(u.Kind, row, parent), nil
}

// AcknowledgeOrphans implements AttachmentUploadCleanupService.
//
// Only an image or report file that is still an orphan is marked: the unlinked ones are read
// again and the request is intersected with them, so an id that was linked to a
// comment since, was already acknowledged, or never was an orphan is skipped
// instead of deleted. The caller cannot use this to delete an arbitrary image.
func (s *attachmentUploadCleanupService) AcknowledgeOrphans(ctx context.Context, req domain.AcknowledgeOrphansRequest) (domain.AcknowledgeOrphansResult, error) {
	if err := RequireInternalCaller(ctx, s.access, "upload cleanup is only available to internal services"); err != nil {
		return domain.AcknowledgeOrphansResult{}, err
	}
	if len(req.IDs) == 0 {
		return domain.AcknowledgeOrphansResult{}, &apierror.ValidationError{Msg: "ids is required"}
	}
	if len(req.IDs) > uploadCleanupMaxPerRun {
		return domain.AcknowledgeOrphansResult{}, &apierror.ValidationError{Msg: fmt.Sprintf("ids cannot exceed %d entries", uploadCleanupMaxPerRun)}
	}
	if err := validateUUIDs("ids", req.IDs); err != nil {
		return domain.AcknowledgeOrphansResult{}, err
	}

	sys := repository.WithSystemIdentity(ctx)
	unlinked, err := s.repo.ListUnlinkedImages(sys, s.now().Add(-unlinkedImageAge), uploadCleanupMaxPerRun)
	if err != nil {
		return domain.AcknowledgeOrphansResult{}, err
	}
	drafts, err := s.repo.ListUnlinkedReportDrafts(sys, s.now().Add(-reportDraftCleanupAge), uploadCleanupMaxPerRun)
	if err != nil {
		return domain.AcknowledgeOrphansResult{}, err
	}
	orphan := make(map[string]domain.StoredAttachmentKind, len(unlinked)+len(drafts))
	for _, img := range unlinked {
		orphan[strings.ToLower(img.ID)] = domain.StoredAttachmentComment
	}
	for _, d := range drafts {
		orphan[strings.ToLower(d.ID)] = domain.StoredAttachmentWorkItem
	}

	var res domain.AcknowledgeOrphansResult
	seen := make(map[string]bool, len(req.IDs))
	for _, id := range req.IDs {
		key := strings.ToLower(id)
		if seen[key] {
			continue
		}
		seen[key] = true
		kind, ok := orphan[key]
		if !ok {
			res.Skipped++
			continue
		}
		by := "inline-image-cleanup"
		if kind == domain.StoredAttachmentWorkItem {
			by = "report-draft-cleanup"
		}
		if _, err := s.repo.MarkDeleted(sys, kind, id, by); err != nil {
			return res, err
		}
		res.Acknowledged++
	}
	slog.InfoContext(ctx, "orphaned comment images acknowledged",
		"audit", true, "action", "upload-cleanup-ack", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx),
		"acknowledged", res.Acknowledged, "skipped", res.Skipped)
	return res, nil
}

// deferRetry sends a row that could not be cleaned to the back of the queue
// (best effort), so a handful of rows that keep failing cannot take every run's
// limited batch and keep the rows behind them from ever being reached.
func (s *attachmentUploadCleanupService) deferRetry(ctx context.Context, sys context.Context, kind domain.StoredAttachmentKind, id string) {
	if err := s.repo.RecordCleanupAttempt(sys, kind, id); err != nil {
		slog.WarnContext(ctx, "could not defer a failed cleanup", "attachmentId", id, "error", err)
	}
}
