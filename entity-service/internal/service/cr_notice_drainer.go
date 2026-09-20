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

// crNoticeBatchSize is how many outbox rows one pass claims. Large enough that
// a backlog drains in few round trips, small enough that one slow batch cannot
// hold rows claimed-but-unprocessed for long.
const crNoticeBatchSize = 100

// CRNoticeDrainer polls event_outbox and hands each row to CRNoticeService.
//
// WHY POLL A TABLE RATHER THAN CONSUME THE BUS. The notices are
// record-triggered -- "state changes to ASSESS" -- which needs a before/after
// diff, and no writer here can produce one: csm-sync upserts blindly from
// ServiceNow, knowing the new row but never the old. An AFTER UPDATE trigger
// is the only place both versions exist at once, so the diff is written to
// event_outbox and read back here.
//
// This is expected to be temporary. Once every writer of change_request is an
// application path that knows what it changed, the trigger and this drainer
// both retire and the same CRNoticeService is fed from that path directly --
// which is why the policy takes a repository.OutboxChange and knows nothing
// about how it arrived.
type CRNoticeDrainer struct {
	Repo     repository.CRNoticeRepository
	Notices  CRNoticeService
	Interval time.Duration
	// EntityTypes limits what is drained. The outbox is shared, and another
	// consumer's rows are none of this drainer's business.
	EntityTypes []string
}

// NewCRNoticeDrainer constructs the poller.
func NewCRNoticeDrainer(repo repository.CRNoticeRepository, notices CRNoticeService, interval time.Duration) *CRNoticeDrainer {
	return &CRNoticeDrainer{
		Repo:        repo,
		Notices:     notices,
		Interval:    interval,
		EntityTypes: []string{CREntityType},
	}
}

// Run drains until ctx is cancelled.
//
// A pass that found a full batch does not wait: a backlog drains at full speed
// and the interval governs only the idle case, trading notice latency against
// query volume.
func (d *CRNoticeDrainer) Run(ctx context.Context) {
	slog.InfoContext(ctx, "crnotice: drainer started", "interval", d.Interval, "entityTypes", d.EntityTypes)
	for {
		n, err := d.drainOnce(ctx)
		if ctx.Err() != nil {
			slog.InfoContext(ctx, "crnotice: drainer stopped")
			return
		}
		if err != nil {
			// Keep polling. A transient database error must not take the
			// drainer down for the life of the process.
			slog.ErrorContext(ctx, "crnotice: drain failed", "err", err)
		}
		if n == crNoticeBatchSize {
			continue
		}
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "crnotice: drainer stopped")
			return
		case <-time.After(d.Interval):
		}
	}
}

// drainOnce claims one batch and processes it, returning how many rows it saw.
func (d *CRNoticeDrainer) drainOnce(ctx context.Context) (int, error) {
	changes, err := d.Repo.ClaimChanges(ctx, d.EntityTypes, crNoticeBatchSize)
	if err != nil {
		return 0, err
	}
	for _, c := range changes {
		// One row's failure must not abandon the rest of the batch: the rows
		// are independent, and they are already claimed, so returning here
		// would drop the remainder silently.
		if err := d.Notices.HandleChange(ctx, c); err != nil {
			slog.ErrorContext(ctx, "crnotice: handle change failed",
				"outboxId", c.ID, "entityType", c.EntityType, "entityId", c.EntityID, "err", err)
		}
	}
	return len(changes), nil
}
