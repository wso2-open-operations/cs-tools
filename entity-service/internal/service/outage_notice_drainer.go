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
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// outageChangeListener wakes the drainer when an outage changes; see
// repository.OutageChangeListener.
type outageChangeListener interface {
	Listen(ctx context.Context) error
	Wait(ctx context.Context, timeout time.Duration) (notified bool, err error)
	Close()
}

const (
	// defaultOutageNoticeUnlistenedInterval is the poll while the drainer
	// cannot listen (no listener, or its connection is down): the delay it
	// had before it could be woken, so losing the listener only ever makes
	// the email slower, never missing.
	defaultOutageNoticeUnlistenedInterval = 10 * time.Second
	// defaultOutageNoticeSettle lets a burst of edits -- a portal save that
	// writes the outage and then its affected CIs, a sync batch -- arrive
	// before the pass that reads them, so one pass handles them all.
	defaultOutageNoticeSettle = 300 * time.Millisecond
	// outageNoticeMaxUnlistenedBackoff caps how far a sustained run of
	// consecutive Sweep failures (e.g. the database being down) stretches the
	// unlistened fallback poll -- without this, a failure every 10s forever
	// would keep hitting a broken dependency at that same fixed cadence
	// indefinitely.
	outageNoticeMaxUnlistenedBackoff = 10 * time.Minute
)

// outageNoticePublisher is the slice of EventPublisherService the drainer needs.
type outageNoticePublisher interface {
	Publish(ctx context.Context, eventType events.Type, entityID string, payload json.RawMessage) error
}

// OutageNoticeDrainer sends the two outage emails within seconds of the change
// that makes them due, as ServiceNow's record-triggered flows do.
//
// *** IT RUNS THE SAME DECISIONS THE SCHEDULED TASKS USED TO. *** Both emails
// are state-based -- "this outage is opted in and has ended, and no resolution
// has been sent" -- so the existing sweeps already say exactly what is owed and
// record it. What changes is cadence and delivery: every Interval instead of
// every scheduler tick, and an event on the outage topic for
// csm-notification-service instead of an email sent by the caller. No outbox
// row is claimed, so the cloud status drainer's use of event_outbox for the
// same outages is untouched.
//
// *** A FLOW WITH NO RECIPIENTS IS NOT SWEPT. *** A sweep records what it
// decides, so sweeping with nobody to send to would mark emails sent that no
// one received. Leaving a recipient list empty is how a deployment keeps that
// email off.
type OutageNoticeDrainer struct {
	Notifications           OutageNotificationService
	Communications          OutageCommunicationService
	Publisher               outageNoticePublisher
	NotificationRecipients  []string
	CommunicationRecipients []string
	// Listener, when set, wakes the drainer within a second of an outage
	// change, and Interval is then only the fallback poll for a notification
	// missed while it was not listening.
	Listener outageChangeListener
	Interval time.Duration
	// UnlistenedInterval and Settle default to the constants above; tests set
	// them shorter.
	UnlistenedInterval time.Duration
	Settle             time.Duration
}

// Run drains until ctx is cancelled. ctx must carry the system identity: both
// sweeps are for internal callers only.
//
// With a Listener it drains once, then sleeps until an outage change is
// notified or Interval passes, whichever is first. Without one -- or whenever
// listening fails -- it polls every UnlistenedInterval, retrying the listener
// each time round.
func (d *OutageNoticeDrainer) Run(ctx context.Context) {
	unlistened := d.UnlistenedInterval
	if unlistened <= 0 {
		unlistened = defaultOutageNoticeUnlistenedInterval
	}
	settle := d.Settle
	if settle <= 0 {
		settle = defaultOutageNoticeSettle
	}
	slog.InfoContext(ctx, "outagenotice: drainer started", "fallbackInterval", d.Interval,
		"listener", d.Listener != nil,
		"notificationRecipients", len(d.NotificationRecipients),
		"communicationRecipients", len(d.CommunicationRecipients))
	defer func() {
		if d.Listener != nil {
			d.Listener.Close()
		}
	}()

	listening := false
	lastListenErr := ""
	consecutiveFailures := 0
	for {
		if d.Listener != nil && !listening {
			if err := d.Listener.Listen(ctx); err != nil {
				// Logged once per distinct failure, not once per poll.
				if msg := err.Error(); msg != lastListenErr {
					slog.WarnContext(ctx, "outagenotice: cannot listen for outage changes; polling instead",
						"every", unlistened, "err", err)
					lastListenErr = msg
				}
			} else {
				listening, lastListenErr = true, ""
				slog.InfoContext(ctx, "outagenotice: listening for outage changes")
			}
		}

		_, failed := d.drainOnce(ctx)

		if !listening {
			// Back off the fixed unlistened poll once failures are sustained
			// (e.g. the database is down) rather than retrying at the same
			// fixed cadence forever; a single clean pass resets it to the
			// plain unlistened interval immediately, not after one more
			// inflated wait left over from before it recovered.
			wait := unlistened
			if failed {
				wait = min(unlistened*time.Duration(1<<min(consecutiveFailures, 16)), outageNoticeMaxUnlistenedBackoff)
				consecutiveFailures++
			} else {
				consecutiveFailures = 0
			}
			if !sleepCtx(ctx, wait) {
				slog.InfoContext(ctx, "outagenotice: drainer stopped")
				return
			}
			continue
		}
		notified, err := d.Listener.Wait(ctx, d.Interval)
		if ctx.Err() != nil {
			slog.InfoContext(ctx, "outagenotice: drainer stopped")
			return
		}
		if err != nil {
			slog.WarnContext(ctx, "outagenotice: lost the outage change listener; reconnecting", "err", err)
			d.Listener.Close()
			listening = false
			continue
		}
		if notified && !sleepCtx(ctx, settle) {
			slog.InfoContext(ctx, "outagenotice: drainer stopped")
			return
		}
	}
}

// sleepCtx waits d, returning false if ctx ended first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// drainOnce runs each enabled flow once and publishes what it decided.
// failed is true when a Sweep call itself errored (a publish failure is kept
// for replay separately and does not count here) -- used only to back off
// the unlistened fallback poll on a sustained failure, see Run.
//
// One flow failing never stops the other, and one publish failing never stops
// the rest: each decision is already recorded, so returning early would drop
// the remainder silently. A failed publish is kept by EventPublisherService's
// failure store for replay, and logged here with the outage it belongs to.
func (d *OutageNoticeDrainer) drainOnce(ctx context.Context) (published int, failed bool) {
	if len(d.NotificationRecipients) > 0 {
		res, err := d.Notifications.Sweep(ctx, 0)
		if err != nil {
			slog.ErrorContext(ctx, "outagenotice: internal-notification sweep failed", "err", err)
			failed = true
		} else {
			for _, dec := range res.Decisions {
				if d.publish(ctx, events.TypeOutageNotificationDue, events.OutageNoticePayload{
					OutageID: dec.OutageID, Number: dec.Number, Kind: string(dec.Kind),
					Subject: dec.Subject, Body: dec.Body, Recipients: d.NotificationRecipients,
				}) {
					published++
				}
			}
			for id, msg := range res.Errors {
				slog.ErrorContext(ctx, "outagenotice: internal notification could not be recorded; not sent",
					"outageId", id, "err", msg)
			}
		}
	}
	if len(d.CommunicationRecipients) > 0 {
		res, err := d.Communications.Sweep(ctx, 0)
		if err != nil {
			slog.ErrorContext(ctx, "outagenotice: outage-communication sweep failed", "err", err)
			failed = true
		} else {
			for _, dec := range res.Decisions {
				if d.publish(ctx, events.TypeOutageCommunicationDue, events.OutageNoticePayload{
					OutageID: dec.OutageID, Number: dec.Number, Kind: string(dec.Kind),
					Subject: dec.Subject, Body: dec.Body, Recipients: d.CommunicationRecipients,
				}) {
					published++
				}
			}
		}
	}
	return published, failed
}

func (d *OutageNoticeDrainer) publish(ctx context.Context, t events.Type, p events.OutageNoticePayload) bool {
	raw, err := json.Marshal(p)
	if err == nil {
		err = d.Publisher.Publish(ctx, t, p.OutageID, raw)
	}
	if err != nil {
		slog.ErrorContext(ctx, "outagenotice: publish failed; email recorded as sent but not delivered",
			"type", string(t), "outageId", p.OutageID, "number", p.Number, "kind", p.Kind,
			"err", fmt.Errorf("publish %s: %w", t, err))
		return false
	}
	slog.InfoContext(ctx, "outagenotice: published", "type", string(t),
		"outageId", p.OutageID, "number", p.Number, "kind", p.Kind)
	return true
}
