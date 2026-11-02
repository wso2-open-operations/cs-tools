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
	"encoding/json"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// ProjectReactivationMaxAttempts is how many times a "Back to Open state"
// notice is tried before it is parked: with the outbox's backoff (30s,
// doubling, at most an hour) that is about 14 hours.
const ProjectReactivationMaxAttempts = 20

// projectReactivationBatch is how many queued notices one pass takes.
const projectReactivationBatch = 50

// ProjectReactivationDrainer publishes the "Back to Open state" notices the
// ACP reopen queued (repository.ACPReactivationOutbox) as project.reactivated
// on the project topic. A notice is marked done only once Event Hub took it;
// a failure is retried with backoff, and parked (logged at ERROR, kept in
// event_outbox with its last error) after maxAttempts. Delivery is at least
// once: a publish that succeeded just before its row could be marked done is
// published again.
type ProjectReactivationDrainer struct {
	outbox      repository.ACPReactivationOutbox
	publisher   EventPublisherService
	interval    time.Duration
	maxAttempts int
}

// projectReactivationDefaultInterval is the poll interval used when the one
// given is not positive (time.NewTicker panics on it).
const projectReactivationDefaultInterval = 5 * time.Second

// NewProjectReactivationDrainer constructs the drainer. A non-positive
// interval is replaced by projectReactivationDefaultInterval.
func NewProjectReactivationDrainer(outbox repository.ACPReactivationOutbox, publisher EventPublisherService, interval time.Duration, maxAttempts int) *ProjectReactivationDrainer {
	if interval <= 0 {
		interval = projectReactivationDefaultInterval
	}
	return &ProjectReactivationDrainer{outbox: outbox, publisher: publisher, interval: interval, maxAttempts: maxAttempts}
}

// Run drains the queue every interval until ctx is cancelled.
func (d *ProjectReactivationDrainer) Run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		d.DrainOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// DrainOnce publishes every notice that is due, once.
func (d *ProjectReactivationDrainer) DrainOnce(ctx context.Context) {
	ids, err := d.outbox.Pending(ctx, projectReactivationBatch)
	if err != nil {
		if ctx.Err() == nil {
			slog.ErrorContext(ctx, "acp reactivation notices: list pending failed", "err", err)
		}
		return
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		_, err := d.outbox.Process(ctx, id, func(ctx context.Context, projectID string, payload json.RawMessage) error {
			return d.publisher.Publish(ctx, events.TypeProjectReactivated, projectID, payload)
		})
		if err == nil {
			continue
		}
		if ctx.Err() != nil {
			// Shutting down: the notice stays queued, untouched, for the
			// next run; recording a failure here would only fail too.
			return
		}
		parked, recErr := d.outbox.RecordFailure(ctx, id, err.Error(), d.maxAttempts)
		switch {
		case recErr != nil:
			slog.ErrorContext(ctx, "acp reactivation notices: record failure failed", "outboxId", id, "err", recErr, "cause", err)
		case parked:
			slog.ErrorContext(ctx, "acp reactivation notices: giving up on a notice after repeated failures", "outboxId", id, "err", err)
		default:
			slog.WarnContext(ctx, "acp reactivation notices: publish failed, will retry", "outboxId", id, "err", err)
		}
	}
}
