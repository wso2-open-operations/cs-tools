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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

const (
	// minRetentionDays is the shortest time a closed work item's files may be kept
	// before removal, so a mistyped value cannot remove recent files.
	minRetentionDays = 30
	// defaultRetentionLimit and maxRetentionLimit bound one call.
	defaultRetentionLimit = 100
	maxRetentionLimit     = 500
)

type attachmentRetentionService struct {
	repo   repository.StoredAttachmentRepository
	access AccessService
	store  FileStore
	now    func() time.Time
}

// NewAttachmentRetentionService constructs an AttachmentRetentionService. store is
// nil when no storage is configured, which makes every call fail.
func NewAttachmentRetentionService(repo repository.StoredAttachmentRepository, access AccessService, store FileStore) AttachmentRetentionService {
	return &attachmentRetentionService{repo: repo, access: access, store: store, now: time.Now}
}

// ExpireClosedWorkItemFiles implements AttachmentRetentionService.
func (s *attachmentRetentionService) ExpireClosedWorkItemFiles(ctx context.Context, req domain.ExpireAttachmentsRequest) (domain.ExpireAttachmentsResult, error) {
	if err := RequireInternalCaller(ctx, s.access, "attachment retention is only available to internal services"); err != nil {
		return domain.ExpireAttachmentsResult{}, err
	}
	if req.ClosedForDays < minRetentionDays {
		return domain.ExpireAttachmentsResult{}, &apierror.ValidationError{Msg: "closedForDays must be at least 30"}
	}
	if s.store == nil {
		return domain.ExpireAttachmentsResult{}, &apierror.ServiceUnavailableError{Msg: "File storage is unavailable or not configured."}
	}
	limit := req.Limit
	switch {
	case limit <= 0:
		limit = defaultRetentionLimit
	case limit > maxRetentionLimit:
		limit = maxRetentionLimit
	}

	sys := repository.WithSystemIdentity(ctx)
	closedBefore := s.now().AddDate(0, 0, -req.ClosedForDays)
	rows, err := s.repo.ListClosedWorkItemFiles(sys, closedBefore, limit)
	if err != nil {
		return domain.ExpireAttachmentsResult{}, err
	}
	res := domain.ExpireAttachmentsResult{DryRun: req.DryRun, Found: len(rows), More: len(rows) == limit}
	for _, row := range rows {
		if !req.DryRun && !s.expire(ctx, sys, row) {
			res.Failed++
			continue
		}
		res.Expired++
		res.Attachments = append(res.Attachments, row.Number)
	}
	return res, nil
}

// expire removes one file, then records it. A file that cannot be removed keeps its
// row COMPLETE, so the next run tries it again.
func (s *attachmentRetentionService) expire(ctx, sys context.Context, row domain.StoredAttachmentRow) bool {
	if row.StorageKey == nil {
		return false
	}
	// A file that is already gone counts as removed.
	if err := s.store.Delete(ctx, *row.StorageKey); err != nil && !errors.Is(err, azblobstore.ErrNotFound) {
		slog.WarnContext(ctx, "could not remove a closed work item's attachment", "attachmentId", row.ID, "error", err)
		return false
	}
	if _, err := s.repo.MarkExpired(sys, row.ID); err != nil {
		slog.WarnContext(ctx, "could not mark a removed attachment expired", "attachmentId", row.ID, "error", err)
		return false
	}
	slog.InfoContext(ctx, "attachment expired",
		"audit", true, "action", "expire", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx), "attachmentId", row.ID, "attachmentNumber", row.Number)
	return true
}
