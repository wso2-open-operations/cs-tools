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
	"encoding/json"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// cloudStatusBatchSize is how many outbox rows one pass claims.
const cloudStatusBatchSize = 100

// cloudStatusEntityTypes are the outbox producers this drainer owns. The
// table is shared; another consumer's rows are none of its business.
//
// Both tables appear because both flows this ports have their own trigger:
// the outage-triggered one on `outage`, the affected-CI one on
// `outage_affected_ci`. A row on either means "this outage may now owe the
// dashboard something".
var cloudStatusEntityTypes = []string{"outage", "outage_affected_ci"}

// CloudStatusDrainer polls event_outbox and re-derives the cloud status
// transitions for whichever outages were touched.
//
// THE FAST PATH, NOT THE ONLY PATH. The reconciliation sweep still runs and
// still reaches the same conclusions; this exists because a sweep reads
// current state and therefore cannot see anything that starts and finishes
// between two runs. An outage lasting less than the sweep interval was only
// ever observed finished, so it produced an end event, no begin event, and
// never appeared on the public status page at all. The trigger sees both
// writes.
//
// WHY A TRIGGER RATHER THAN THE WRITER TELLING US. Today nothing in this
// service writes `outage` -- csm-sync-service does, upserting blindly from
// ServiceNow, so it knows the new row but never the old. An AFTER-change
// trigger is the only place both versions exist at once. This is the same
// reasoning 0051 gives for change_request, and the same conclusion.
//
// It is also expected to be temporary for the same reason. Once outages are
// created and resolved through this service's own API, that path knows what
// it changed and can record the transition directly, in the same transaction
// -- at which point the triggers and this drainer retire together. The
// decision logic is untouched by that change, because it takes candidates and
// knows nothing about how they were noticed.
type CloudStatusDrainer struct {
	Repo     repository.CloudStatusRepository
	Service  CloudStatusService
	Interval time.Duration
}

// NewCloudStatusDrainer constructs the poller.
func NewCloudStatusDrainer(repo repository.CloudStatusRepository, svc CloudStatusService, interval time.Duration) *CloudStatusDrainer {
	return &CloudStatusDrainer{Repo: repo, Service: svc, Interval: interval}
}

// Run drains until ctx is cancelled.
func (d *CloudStatusDrainer) Run(ctx context.Context) {
	slog.InfoContext(ctx, "cloudstatus: drainer started", "interval", d.Interval.String())
	for {
		n, err := d.drainOnce(ctx)
		if err != nil {
			// Logged, not fatal. A failed pass leaves the rows unclaimed for
			// the next one, and the sweep would catch the transition anyway.
			slog.ErrorContext(ctx, "cloudstatus: drain pass failed", "err", err)
		} else if n > 0 {
			slog.InfoContext(ctx, "cloudstatus: drained outbox rows", "count", n)
		}
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "cloudstatus: drainer stopped")
			return
		case <-time.After(d.Interval):
		}
	}
}

// drainOnce claims one batch and processes the outages it names.
func (d *CloudStatusDrainer) drainOnce(ctx context.Context) (int, error) {
	changes, err := d.Repo.ClaimChanges(ctx, cloudStatusEntityTypes, cloudStatusBatchSize)
	if err != nil {
		return 0, err
	}
	if len(changes) == 0 {
		return 0, nil
	}

	// Collapse to distinct outage ids before doing any work. A batch routinely
	// holds several rows for one outage -- an update plus two affected CIs --
	// and they all lead to the same question. Asking it once per outage rather
	// than once per row is the difference between one query and a dozen.
	seen := map[string]bool{}
	ids := make([]string, 0, len(changes))
	for _, c := range changes {
		id := outageIDOf(c)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	if err := d.Service.HandleOutages(ctx, ids); err != nil {
		return len(changes), err
	}
	return len(changes), nil
}

// outageIDOf finds the outage an outbox row is about.
//
// For an `outage` row the entity id IS the outage. For an `outage_affected_ci`
// row it is the join row's own id, and the outage is a column on it -- read
// from the snapshot, which is why the trigger records one.
func outageIDOf(c repository.OutboxChange) string {
	if c.EntityType == "outage" {
		return c.EntityID
	}
	if v, ok := c.Snapshot["outage_id"].(string); ok {
		return v
	}
	// A join row with no outage is the orphan the affected-CI mirror already
	// documents one of. Nothing to decide about it.
	return ""
}

// publishDue publishes rows just recorded and reserved as
// outage.status_page_due, each carrying its claim token: the consumer must win
// that claim (POST /internal/cloud-status/{id}/claim) before it posts, so a
// redelivered or late event can never post twice. A row that cannot be
// published has its reservation released at once -- nothing was sent -- so the
// scheduled task posts it on its next tick.
func (s *cloudStatusService) publishDue(ctx context.Context, due []reservedWebhook) {
	for _, r := range due {
		w, err := s.repo.PendingByID(ctx, r.id)
		if err != nil {
			// The reservation runs out and the scheduled task picks it up.
			slog.ErrorContext(ctx, "cloudstatus: read webhook to publish failed", "webhookId", r.id, "err", err)
			continue
		}
		if w == nil {
			continue
		}
		slug, wire := domain.CloudOfferingSlug(w.Cloud), w.Event.WireValue()
		if slug == "" || wire == "" {
			// Cannot happen for a row recorded through process, which checks
			// the slug. Released, so the scheduled task closes it visibly.
			s.releaseReservation(ctx, r)
			continue
		}
		raw, err := json.Marshal(events.OutageStatusPageDuePayload{
			WebhookID: w.ID, ClaimToken: r.token, OutageID: w.OutageID, Number: w.Number,
			Cloud: slug, Event: wire, Timestamp: w.Timestamp,
		})
		if err == nil {
			// Keyed by outage: its begin and end land on one partition, in order.
			err = s.publisher.Publish(ctx, events.TypeOutageStatusPageDue, w.OutageID, raw)
		}
		if err != nil {
			// Not logging err: it can carry broker details, and Publish has
			// already recorded it in event_publish_failures.
			slog.ErrorContext(ctx, "cloudstatus: publish outage.status_page_due failed; left for the scheduled task",
				"webhookId", r.id, "number", w.Number, "cloud", slug, "event", wire)
			s.releaseReservation(ctx, r)
			continue
		}
		slog.InfoContext(ctx, "cloudstatus: published outage.status_page_due",
			"webhookId", r.id, "number", w.Number, "cloud", slug, "event", wire)
	}
}

func (s *cloudStatusService) releaseReservation(ctx context.Context, r reservedWebhook) {
	if err := s.repo.ReleaseReservation(ctx, r.id, r.token); err != nil {
		slog.ErrorContext(ctx, "cloudstatus: release reservation failed", "webhookId", r.id, "err", err)
	}
}

// HandleOutages re-derives and records the current transition for the named
// outages. It is the record-triggered counterpart to Sweep, and deliberately
// shares its decision path.
func (s *cloudStatusService) HandleOutages(ctx context.Context, outageIDs []string) error {
	if len(outageIDs) == 0 {
		return nil
	}
	if len(s.parentServiceIDs) == 0 {
		return nil
	}
	candidates, err := s.repo.CandidatesByOutage(ctx, s.parentServiceIDs, outageIDs)
	if err != nil {
		return err
	}
	var resp domain.CloudStatusSweepResponse
	var due []reservedWebhook
	var dueRef *[]reservedWebhook
	if s.publisher != nil {
		dueRef = &due
	}
	err = s.process(ctx, candidates, &resp, dueRef)
	// Publish what was recorded even when a later candidate failed: those
	// rows are leased, and leaving them would delay them by the whole lease.
	s.publishDue(ctx, due)
	if err != nil {
		return err
	}
	if resp.Recorded > 0 || resp.MonitorsUpdated > 0 {
		slog.InfoContext(ctx, "cloudstatus: handled outage changes",
			"outages", len(outageIDs), "recorded", resp.Recorded,
			"monitorsUpdated", resp.MonitorsUpdated)
	}
	return nil
}
