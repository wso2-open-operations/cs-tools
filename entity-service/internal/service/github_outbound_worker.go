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

package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const outboundBatchSize = 20

// GithubOutboundWorker drains github_outbound_queue and pushes to GitHub.
//
// Separate from the notification drainer because the failure model is
// different. That one claims a row and never retries -- a lost notification
// beats a duplicate one. This calls a third party that returns 502s and rate
// limits which succeed on the next attempt, so a failure is rescheduled with
// exponential backoff and only abandoned after a bounded number of tries.
type GithubOutboundWorker struct {
	Repo     repository.GithubOutboundRepository
	Outbound GithubOutboundService
	Interval time.Duration
}

// NewGithubOutboundWorker constructs the worker.
func NewGithubOutboundWorker(repo repository.GithubOutboundRepository, outbound GithubOutboundService, interval time.Duration) *GithubOutboundWorker {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &GithubOutboundWorker{Repo: repo, Outbound: outbound, Interval: interval}
}

// Run drains until ctx is cancelled.
func (w *GithubOutboundWorker) Run(ctx context.Context) {
	slog.InfoContext(ctx, "github outbound: worker started", "interval", w.Interval)
	for {
		n, err := w.drainOnce(ctx)
		if ctx.Err() != nil {
			slog.InfoContext(ctx, "github outbound: worker stopped")
			return
		}
		if err != nil {
			// Keep going. A transient database error must not take the worker
			// down for the life of the process.
			slog.ErrorContext(ctx, "github outbound: drain failed", "err", err)
		}
		// A full batch means there is more waiting; do not sleep on a backlog.
		if n == outboundBatchSize {
			continue
		}
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "github outbound: worker stopped")
			return
		case <-time.After(w.Interval):
		}
	}
}

func (w *GithubOutboundWorker) drainOnce(ctx context.Context) (int, error) {
	items, err := w.Repo.ClaimDue(ctx, outboundBatchSize)
	if err != nil {
		return 0, err
	}
	for _, item := range items {
		w.deliver(ctx, item)
	}
	return len(items), nil
}

// deliver pushes one item and records the outcome. One item's failure never
// abandons the rest of the batch: they are independent, and a rate limit on
// one issue says nothing about another.
func (w *GithubOutboundWorker) deliver(ctx context.Context, item repository.OutboundItem) {
	err := w.Outbound.Deliver(ctx, item)
	if err == nil {
		if err := w.Repo.MarkDelivered(ctx, item.ID); err != nil {
			slog.ErrorContext(ctx, "github outbound: could not mark delivered", "id", item.ID, "err", err)
		}
		return
	}

	attempts := item.Attempts + 1

	// A permanent failure is failed immediately rather than retried six times
	// against something that will never succeed.
	if OutboundPermanent(err) {
		if rErr := w.Repo.Reschedule(ctx, item.ID, outboundMaxAttempts, outboundMaxAttempts, 0, errClass(err)); rErr != nil {
			slog.ErrorContext(ctx, "github outbound: could not record permanent failure", "id", item.ID, "err", rErr)
		}
		slog.WarnContext(ctx, "github outbound: permanent failure, not retrying",
			"id", item.ID, "event", item.Event, "reason", errClass(err))
		return
	}

	// GitHub's own Retry-After wins over our backoff: it knows when the limit
	// resets and we are guessing.
	backoff := OutboundBackoff(attempts)
	if after, ok := OutboundRetryAfter(err); ok {
		backoff = after
	}

	if rErr := w.Repo.Reschedule(ctx, item.ID, attempts, outboundMaxAttempts, backoff, errClass(err)); rErr != nil {
		slog.ErrorContext(ctx, "github outbound: could not reschedule", "id", item.ID, "err", rErr)
		return
	}
	slog.WarnContext(ctx, "github outbound: delivery failed, rescheduled",
		"id", item.ID, "event", item.Event, "attempts", attempts,
		"retryIn", backoff.String(), "reason", errClass(err))
}

// errClass describes a failure without reproducing it. A GitHub error can
// carry response content this service is not allowed to store or log, so only
// the classification and status are kept.
func errClass(err error) string {
	if err == nil {
		return ""
	}
	if after, ok := OutboundRetryAfter(err); ok {
		return "rate limited, retry after " + after.String()
	}
	if OutboundPermanent(err) {
		return "permanent: " + shortErr(err)
	}
	return shortErr(err)
}

func shortErr(err error) string {
	s := err.Error()
	const max = 200
	if len(s) > max {
		return s[:max]
	}
	return s
}
