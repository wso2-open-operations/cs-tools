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
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Package housekeeping is this component's own first real sub-cron: it
// deletes old resolved rows from entity-service's scheduled_task_run
// table, so that table doesn't grow forever now that something is finally
// registered to call DELETE /scheduled-tasks/attempts (see both
// components' CLAUDE.md, "Housekeeping" / "Scheduled task runs" — this
// endpoint existed from the start but nothing called it until this
// package).
package housekeeping

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
)

// Client is the subset of *ledger.Client this package depends on —
// declared here so a test can substitute a fake without importing the
// real HTTP client.
type Client interface {
	DeleteResolvedBefore(ctx context.Context, cutoff time.Time) (int, error)
}

// CleanupResolvedRuns returns a registry.Task.Handler that deletes every
// scheduled_task_run row that succeeded or was superseded ("fully omitted
// after retrying") more than retention ago, by its own resolution time —
// a row still open/failed is never touched, regardless of age.
func CleanupResolvedRuns(client Client, retention time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		cutoff := time.Now().Add(-retention)
		deleted, err := client.DeleteResolvedBefore(ctx, cutoff)
		if err != nil {
			return fmt.Errorf("housekeeping: delete resolved runs before %s: %w", cutoff.Format(time.RFC3339), err)
		}
		slog.InfoContext(ctx, "housekeeping: deleted resolved scheduled_task_run rows", "cutoff", cutoff.Format(time.RFC3339), "deleted", deleted)
		return nil
	}
}

// AttachmentCleaner asks entity-service to clean up abandoned attachment uploads.
type AttachmentCleaner interface {
	CleanupAttachmentUploads(ctx context.Context) (ledger.AttachmentCleanup, error)
}

// CleanupAttachmentUploads returns a registry.Task.Handler that runs entity-service's
// attachment upload cleanup. Entity-service does the work; this only triggers it.
// A file that could not be removed is tried again on the next run, so it is logged
// as a warning, not failed.
func CleanupAttachmentUploads(client AttachmentCleaner) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		result, err := client.CleanupAttachmentUploads(ctx)
		if err != nil {
			return fmt.Errorf("housekeeping: attachment upload cleanup: %w", err)
		}
		slog.InfoContext(ctx, "housekeeping: attachment upload cleanup done",
			"uploadsCleaned", result.Cleaned, "imagesRemoved", result.ImagesRemoved, "reportDraftsRemoved", result.ReportDraftsRemoved)
		failed := result.Failed + result.ImagesFailed + result.ReportDraftsFailed
		if failed > 0 || len(result.Orphans) > 0 {
			slog.WarnContext(ctx, "housekeeping: some attachment files were not removed and will be tried again",
				"failed", failed, "orphans", len(result.Orphans))
		}
		return nil
	}
}

// AttachmentRetentionConfig is how the retention job runs.
type AttachmentRetentionConfig struct {
	// ClosedForDays is how long a work item must have been closed before its files go.
	ClosedForDays int
	// BatchSize is how many attachments one call handles.
	BatchSize int
	// DryRun counts what would be removed and removes nothing.
	DryRun bool
}

// maxRetentionPages bounds one run; a larger backlog is worked off over several runs.
const maxRetentionPages = 20

// AttachmentRetentionClient asks entity-service to remove the files of closed work items.
type AttachmentRetentionClient interface {
	ExpireClosedAttachments(ctx context.Context, closedForDays, limit int, dryRun bool) (ledger.AttachmentRetention, error)
}

// ExpireClosedAttachments returns a registry.Task.Handler that removes the files of
// work items that have been closed for cfg.ClosedForDays. Entity-service decides what
// is due and does the removal; this asks for one page at a time until it is done.
// A file that could not be removed is tried again next run, so it only warns.
func ExpireClosedAttachments(client AttachmentRetentionClient, cfg AttachmentRetentionConfig) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var expired, failed int
		for page := 0; page < maxRetentionPages; page++ {
			result, err := client.ExpireClosedAttachments(ctx, cfg.ClosedForDays, cfg.BatchSize, cfg.DryRun)
			if err != nil {
				return fmt.Errorf("housekeeping: attachment retention: %w", err)
			}
			expired += result.Expired
			failed += result.Failed
			// A dry run changes nothing, so another page would show the same rows.
			if !result.More || cfg.DryRun {
				break
			}
		}
		slog.InfoContext(ctx, "housekeeping: attachment retention done",
			"closedForDays", cfg.ClosedForDays, "dryRun", cfg.DryRun, "expired", expired, "failed", failed)
		if failed > 0 {
			slog.WarnContext(ctx, "housekeeping: some attachment files were not removed and will be tried again", "failed", failed)
		}
		return nil
	}
}
