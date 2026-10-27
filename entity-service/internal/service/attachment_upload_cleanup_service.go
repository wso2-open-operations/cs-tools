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
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/filestore"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	// abandonedUploadAge is how old a PENDING upload must be to count as
	// abandoned. A confirm that failed with a storage error leaves the row
	// PENDING and the file staged on purpose so it can be retried, so this is
	// long next to the 15 minute upload link.
	abandonedUploadAge = 24 * time.Hour
	// uploadCleanupMaxPerRun bounds one run; a larger backlog is worked off over
	// several runs.
	uploadCleanupMaxPerRun = 200
)

// unlinkedImageAge is how long a stored comment image may stay unlinked before it
// is removed: the time to write a comment, with room to spare.
const unlinkedImageAge = 24 * time.Hour

// uploadStore is the part of the SFTPGo client the upload cleanup uses.
type uploadStore interface {
	RemoveDir(ctx context.Context, dir string) error
	RemoveFile(ctx context.Context, path string) error
}

type attachmentUploadCleanupService struct {
	repo   repository.StoredAttachmentRepository
	store  uploadStore
	access AccessService
	now    func() time.Time
}

// NewAttachmentUploadCleanupService constructs an AttachmentUploadCleanupService.
func NewAttachmentUploadCleanupService(repo repository.StoredAttachmentRepository, store uploadStore, access AccessService) AttachmentUploadCleanupService {
	return &attachmentUploadCleanupService{repo: repo, store: store, access: access, now: time.Now}
}

// CleanupUploads implements AttachmentUploadCleanupService.
//
// An upload that was started and never confirmed leaves a PENDING row and a
// staging folder under csm/staging. This removes the folder and marks the row
// FAILED (which every list already hides and a later confirm refuses). A folder
// that cannot be removed leaves the row PENDING, so the next run tries it again.
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
		if err := s.store.RemoveDir(ctx, filestore.StagingDir(u.ID)); err != nil && !errors.Is(err, filestore.ErrNotFound) {
			res.Failed++
			slog.WarnContext(ctx, "could not remove an abandoned upload folder", "attachmentId", u.ID, "error", err)
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
		if img.StorageKey != nil && *img.StorageKey != "" {
			if err := s.store.RemoveFile(ctx, strings.TrimLeft(*img.StorageKey, "/")); err != nil && !errors.Is(err, filestore.ErrNotFound) {
				res.ImagesFailed++
				slog.WarnContext(ctx, "could not remove an unlinked comment image", "attachmentId", img.ID, "error", err)
				s.deferRetry(ctx, sys, domain.StoredAttachmentComment, img.ID)
				continue
			}
		}
		if _, err := s.repo.MarkDeleted(sys, domain.StoredAttachmentComment, img.ID, "inline-image-cleanup"); err != nil {
			res.ImagesFailed++
			slog.WarnContext(ctx, "could not mark an unlinked comment image deleted", "attachmentId", img.ID, "error", err)
			s.deferRetry(ctx, sys, domain.StoredAttachmentComment, img.ID)
			continue
		}
		res.ImagesRemoved++
	}
	slog.InfoContext(ctx, "attachment upload cleanup",
		"audit", true, "action", "upload-cleanup", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx),
		"found", res.Found, "cleaned", res.Cleaned, "failed", res.Failed,
		"imagesRemoved", res.ImagesRemoved, "imagesFailed", res.ImagesFailed)
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
