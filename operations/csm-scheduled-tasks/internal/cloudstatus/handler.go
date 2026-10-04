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

package cloudstatus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// decisionClient is the subset of *Client this handler depends on, declared
// here so a test can substitute a fake -- the same convention
// internal/announcementpublish.dueClient uses.
type decisionClient interface {
	Sweep(ctx context.Context) (SweepResult, error)
	Pending(ctx context.Context) ([]PendingWebhook, error)
	RecordDelivery(ctx context.Context, id string, delivered bool, errMsg string) error
}

// poster is the subset of *Webhook this handler depends on.
type poster interface {
	Post(ctx context.Context, cloud, event, timestamp string) error
	Knows(cloud string) bool
}

// DeliverDue returns a registry.Task.Handler that re-derives which cloud
// status transitions are owed, then posts each one and reports the outcome.
//
// SWEEP THEN DELIVER, IN ONE TICK. The two could be separate tasks, and are
// deliberately not: the sweep is cheap, and running it immediately before
// delivering means a transition is never waiting a full extra period between
// being noticed and being sent. A public status page is the one audience where
// that latency is the whole point of the feature.
//
// A SWEEP FAILURE DOES NOT STOP DELIVERY. If entity-service cannot re-derive
// the scope -- a bad query, a database hiccup -- the webhooks already recorded
// are still owed and still deliverable. Abandoning the tick would hold back
// events that have nothing to do with the failure. The sweep error is kept and
// joined into the summary so it still reaches the alert.
//
// EVERY WEBHOOK IS ATTEMPTED INDEPENDENTLY, for the reason the legacy
// workflow could not: there, one action failure stopped the whole execution
// for that record.
// Here one dashboard being down must not stop another dashboard being told.
//
// Idempotent per period, as registry.Task.Handler requires. The sweep records
// nothing it has recorded before, and a delivered webhook stops appearing in
// the pending set -- so a tick with nothing new does nothing at all.
func DeliverDue(client decisionClient, hook poster) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var errs []error

		if res, err := client.Sweep(ctx); err != nil {
			slog.ErrorContext(ctx, "cloudstatus: sweep failed; delivering what is already recorded", "err", err)
			errs = append(errs, fmt.Errorf("sweep: %w", err))
		} else {
			slog.InfoContext(ctx, "cloudstatus: sweep complete",
				"scanned", res.Scanned, "recorded", res.Recorded, "skippedNoCloud", res.SkippedNoCloud)
			if res.SkippedNoCloud > 0 {
				// Not joined into errs: an outage with no cloud monitor is a
				// data problem in the source record, not a failure of this tick, and
				// alerting on every tick until someone fixes the record would
				// train the audience to ignore the alert.
				slog.WarnContext(ctx, "cloudstatus: in-scope outages could not be routed to any cloud",
					"count", res.SkippedNoCloud)
			}
		}

		pending, err := client.Pending(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("read pending: %w", err))
			return errors.Join(errs...)
		}
		if len(pending) == 0 {
			return errors.Join(errs...)
		}

		for _, w := range pending {
			if w.WireEvent == "" {
				// entity-service withholds the wire value when it cannot map
				// the event, so an empty one here means a version skew
				// between the two services rather than bad data. Posting an
				// empty event would be accepted and silently ignored.
				msg := "entity-service sent no wire value for event " + w.Event
				slog.ErrorContext(ctx, "cloudstatus: "+msg, "webhookId", w.ID)
				errs = append(errs, fmt.Errorf("webhook %s: %s", w.ID, msg))
				continue
			}
			if !hook.Knows(w.Cloud) {
				// A recorded event for a dashboard this deployment has no URL
				// for. Reported as a failed attempt rather than skipped, so it
				// counts against the attempt limit and stops being retried
				// forever -- and so last_error says exactly what is missing.
				msg := "no dashboard URL configured for cloud " + w.Cloud
				slog.ErrorContext(ctx, "cloudstatus: "+msg, "webhookId", w.ID, "number", w.Number)
				if rerr := client.RecordDelivery(ctx, w.ID, false, msg); rerr != nil {
					errs = append(errs, fmt.Errorf("webhook %s: report unroutable: %w", w.ID, rerr))
				}
				errs = append(errs, fmt.Errorf("webhook %s: %s", w.ID, msg))
				continue
			}

			postErr := hook.Post(ctx, w.Cloud, w.WireEvent, w.Timestamp)
			if postErr != nil {
				slog.WarnContext(ctx, "cloudstatus: webhook post failed",
					"webhookId", w.ID, "number", w.Number, "cloud", w.Cloud,
					"attempt", w.AttemptCount+1, "err", postErr)
				if rerr := client.RecordDelivery(ctx, w.ID, false, postErr.Error()); rerr != nil {
					errs = append(errs, fmt.Errorf("webhook %s: report failure: %w", w.ID, rerr))
				}
				errs = append(errs, fmt.Errorf("webhook %s (%s): %w", w.ID, w.Cloud, postErr))
				continue
			}

			// The post succeeded and the report of it can still fail. That
			// leaves the row undelivered and it will be posted again next
			// tick -- a duplicate event to the dashboard, which is the right
			// way to lose this race: telling a status page twice that an
			// outage began is harmless, never telling it is not.
			if rerr := client.RecordDelivery(ctx, w.ID, true, ""); rerr != nil {
				slog.ErrorContext(ctx, "cloudstatus: webhook delivered but the outcome could not be recorded; it will be re-sent",
					"webhookId", w.ID, "number", w.Number, "err", rerr)
				errs = append(errs, fmt.Errorf("webhook %s: report success: %w", w.ID, rerr))
				continue
			}
			slog.InfoContext(ctx, "cloudstatus: webhook delivered",
				"webhookId", w.ID, "number", w.Number, "cloud", w.Cloud, "event", w.Event)
		}
		return errors.Join(errs...)
	}
}
