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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/filestore"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
)

const (
	// shareCleanupPageSize is how many shares are read from SFTPGo at a time.
	shareCleanupPageSize = 100
	// shareCleanupMaxDeletes bounds one run, so a large backlog is worked off over
	// several runs instead of holding one request open.
	shareCleanupMaxDeletes = 500
	// shareCleanupGrace is how long past its expiry a share is kept before it is
	// removed, so a clock difference with SFTPGo never deletes a live one.
	shareCleanupGrace = 5 * time.Minute
)

// attachmentShareNames are the names entity-service gives the shares it creates
// ("dl" download, "up" upload). Only these are ever touched: the service user may
// hold shares that something else made.
var attachmentShareNames = map[string]bool{"dl": true, "up": true}

// shareStore is the part of the SFTPGo client the cleanup uses.
type shareStore interface {
	ListShares(ctx context.Context, limit, offset int) ([]filestore.Share, error)
	DeleteShare(ctx context.Context, id string) error
}

type attachmentShareCleanupService struct {
	store  shareStore
	access AccessService
	now    func() time.Time
}

// NewAttachmentShareCleanupService constructs an AttachmentShareCleanupService.
func NewAttachmentShareCleanupService(store shareStore, access AccessService) AttachmentShareCleanupService {
	return &attachmentShareCleanupService{store: store, access: access, now: time.Now}
}

// CleanupShares implements AttachmentShareCleanupService.
//
// Every download and upload link is a stored record in SFTPGo, and a record that
// has expired or been used up serves nothing but is not necessarily removed by
// SFTPGo itself. This deletes entity-service's own shares (by name) that expired
// more than shareCleanupGrace ago, or whose uses are all spent. A share that
// cannot be deleted is skipped and counted; the next run tries it again.
func (s *attachmentShareCleanupService) CleanupShares(ctx context.Context) (domain.ShareCleanupResult, error) {
	if err := RequireInternalCaller(ctx, s.access, "share cleanup is only available to internal services"); err != nil {
		return domain.ShareCleanupResult{}, err
	}
	cutoff := s.now().Add(-shareCleanupGrace).UnixMilli()

	var res domain.ShareCleanupResult
	var doomed []string
	for offset := 0; len(doomed) < shareCleanupMaxDeletes; offset += shareCleanupPageSize {
		page, err := s.store.ListShares(ctx, shareCleanupPageSize, offset)
		if err != nil {
			return res, storageUnavailable(ctx, "list shares", err)
		}
		for _, sh := range page {
			res.Scanned++
			if !attachmentShareNames[sh.Name] {
				continue
			}
			expired := sh.ExpiresAt > 0 && sh.ExpiresAt < cutoff
			spent := sh.MaxTokens > 0 && sh.UsedTokens >= sh.MaxTokens
			if (expired || spent) && len(doomed) < shareCleanupMaxDeletes {
				doomed = append(doomed, sh.ID)
			}
		}
		if len(page) < shareCleanupPageSize {
			break
		}
	}
	for _, id := range doomed {
		switch err := s.store.DeleteShare(ctx, id); {
		case err == nil || errors.Is(err, filestore.ErrNotFound):
			res.Deleted++
		default:
			res.Failed++
			slog.WarnContext(ctx, "could not delete share", "shareRef", shareRef(id), "error", err)
		}
	}
	slog.InfoContext(ctx, "attachment share cleanup",
		"audit", true, "action", "share-cleanup", "callerId", auditCaller(ctx),
		"correlationId", middleware.CorrelationIDFromContext(ctx),
		"scanned", res.Scanned, "deleted", res.Deleted, "failed", res.Failed)
	return res, nil
}
