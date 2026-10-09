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
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// cloudStatusMaxAttempts caps how many times one webhook is retried before it
// stops being handed out as pending.
//
// ServiceNow set its REST step's retry policy to NONE: one attempt, and a
// failed post was lost silently. Retrying is a deliberate improvement, but a
// bounded one -- an endpoint that is refusing everything should not have the
// whole backlog thrown at it once a minute forever. Exhausted rows stay in the
// table, undelivered, which is what makes the failure visible instead of
// merely absent.
const cloudStatusMaxAttempts = 5

// cloudStatusPendingLimit caps one delivery batch.
const cloudStatusPendingLimit = 50

type cloudStatusService struct {
	repo repository.CloudStatusRepository
	// parentServiceIDs is the trigger's 14 service ids, from configuration.
	//
	// NOT hardcoded, despite being stable: ServiceNow kept them in the flow
	// definition where changing the list was a flow edit. Config is the
	// nearest equivalent that does not require a deploy, and it is the list
	// most likely to change -- adding a cloud to the status page is a
	// business decision, not a code change.
	parentServiceIDs []string
	// publisher puts each transition on the operations topic (sre-events) as
	// outage.status_page_due the moment it is recorded, for
	// csm-notification-service to post. Nil leaves every post to
	// csm-scheduled-tasks' tick, as before. See WithCloudStatusPublisher.
	publisher EventPublisherService
}

// cloudStatusDeliveryLease protects a reservation (published, not yet
// attempted) and an attempt (a sender is posting). A reservation that runs
// out goes to the scheduled task -- nothing was sent. An attempt that runs out
// without an outcome is an unknown outcome and is never re-sent. Long enough
// for the scheduled task to post a whole batch (50 x 15s).
const cloudStatusDeliveryLease = 15 * time.Minute

// reservedWebhook is a row recorded and reserved for publishing.
type reservedWebhook struct{ id, token string }

// WithCloudStatusPublisher makes the record-triggered path (HandleOutages,
// called straight after an outage is written) publish each transition it
// records as outage.status_page_due, instead of leaving it for
// csm-scheduled-tasks' next tick -- up to five minutes, plus however often
// Choreo fires that component. csm-notification-service posts it and reports
// the outcome; the scheduled task stays the retry path for a failed post, and
// its sweep still records anything never published.
//
// *** THE SAME DOUBLE-FIRE GUARD AS CLOUD_STATUS_ENABLED. *** While
// ServiceNow's "Cloud Status Event Notification Flow" (and "- Affected CI")
// is active, ServiceNow posts these too.
//
// svc is returned unchanged when it is not the service this package builds.
func WithCloudStatusPublisher(svc CloudStatusService, publisher EventPublisherService) CloudStatusService {
	if s, ok := svc.(*cloudStatusService); ok && publisher != nil {
		s.publisher = publisher
	}
	return svc
}

// NewCloudStatusService constructs the cloud status webhook decision service.
func NewCloudStatusService(repo repository.CloudStatusRepository, parentServiceIDs []string) CloudStatusService {
	return &cloudStatusService{repo: repo, parentServiceIDs: parentServiceIDs}
}

// Sweep decides which in-scope outages now owe the status dashboard a webhook
// and records them. It sends nothing itself.
//
// WHY A SWEEP AND NOT AN EVENT -- AND WHY THAT IS TEMPORARY.
//
// ServiceNow triggered on the outage record being updated. Today `outage` is
// written by csm-sync-service and by nothing in this service, so there is no
// write here to hang a trigger or an outbox row off, and a sweep over current
// state is the only mechanism available.
//
// *** THAT CHANGES AT CUTOVER. *** csm-sync-service is a migration aid, not a
// permanent component: once ServiceNow is decommissioned it stops running and
// `outage` becomes a natively owned table like any other. At that point the
// honest design is the one `change_request` already uses -- an AFTER-change
// trigger writing to event_outbox, drained in process -- and this sweep
// should be replaced by it rather than kept out of habit.
//
// Two things to carry across when that happens, both of which the sweep gets
// for free and a trigger does not:
//
//   - SELF-HEALING. A sweep missed for any reason is made up by the next one,
//     because the decision is derived from the outage's state rather than
//     from having observed the change. A missed trigger is gone. Whatever
//     replaces this wants a periodic reconciliation pass behind it.
//   - THE EVENTS TABLE STAYS EITHER WAY. cloud_status_events is what makes a
//     webhook send once; it is not an artifact of sweeping.
//
// And one property the sweep LOSES, which a trigger would recover: an outage
// that begins and ends inside one sweep interval is only ever seen in its
// final state, so it produces an end event and no begin event. Outages
// shorter than the interval never appear on the public status page at all.
func (s *cloudStatusService) Sweep(ctx context.Context) (domain.CloudStatusSweepResponse, error) {
	if len(s.parentServiceIDs) == 0 {
		// Not an error that should fail the sweep loudly on every tick, but
		// also not something to pass over in silence: with no scope the sweep
		// is a no-op, and an operator who set the config wrongly would
		// otherwise see only a healthy-looking zero.
		slog.WarnContext(ctx, "cloud status sweep has no in-scope services configured; nothing will ever be sent")
		return domain.CloudStatusSweepResponse{}, nil
	}

	candidates, err := s.repo.Candidates(ctx, s.parentServiceIDs)
	if err != nil {
		return domain.CloudStatusSweepResponse{}, err
	}

	resp := domain.CloudStatusSweepResponse{Scanned: len(candidates)}
	// nil: the sweep only records. It is csm-scheduled-tasks that calls it,
	// and that component posts the pending rows in the same run.
	if err := s.process(ctx, candidates, &resp, nil); err != nil {
		return domain.CloudStatusSweepResponse{}, err
	}
	return resp, nil
}

// process records and applies every candidate's current transition.
//
// THE SWEEP AND THE TRIGGER PATH BOTH END HERE, and that is the point. One
// notices changes quickly and the other notices them eventually; neither is
// allowed its own opinion about what a change MEANS. If they diverged, a
// transition handled by the trigger would behave differently from the same
// transition handled by reconciliation, and the two are indistinguishable
// after the fact.
//
// due, when non-nil, collects the rows recorded for instant delivery: each is
// recorded with a reservation (RecordAndReserve), and the caller publishes it
// after this returns.
func (s *cloudStatusService) process(ctx context.Context, candidates []repository.CloudStatusCandidate, resp *domain.CloudStatusSweepResponse, due *[]reservedWebhook) error {
	for _, c := range candidates {
		clouds, err := s.cloudsFor(ctx, c)
		if err != nil {
			return err
		}
		if len(clouds) == 0 {
			// No cloud monitor on the outage's configuration item, or an
			// offering this service does not know how to address. Either way
			// there is nowhere to post.
			//
			// ServiceNow's Look Up Record step failed the whole flow execution
			// here, which meant one unroutable outage stopped every later step
			// for that record. Skipping and continuing is the right trade for
			// a sweep that handles many outages per run -- but it must be
			// counted, or an outage that never reaches the dashboard looks
			// exactly like an outage with nothing to report.
			resp.SkippedNoCloud++
			slog.WarnContext(ctx, "in-scope outage has no routable cloud offering; skipping",
				"outageId", c.OutageID, "number", c.Number, "cloudOffering", c.Cloud)
			continue
		}
		for _, cloud := range clouds {
			rec := c
			rec.Cloud = cloud
			var recorded bool
			var err error
			if due != nil && s.publisher != nil {
				var id, token string
				id, token, recorded, err = s.repo.RecordAndReserve(ctx, rec, cloudStatusDeliveryLease)
				if recorded {
					*due = append(*due, reservedWebhook{id: id, token: token})
				}
			} else {
				recorded, err = s.repo.Record(ctx, rec)
			}
			if err != nil {
				return err
			}
			if recorded {
				resp.Recorded++
				slog.InfoContext(ctx, "cloud status transition recorded",
					"outageId", c.OutageID, "number", c.Number,
					"event", string(c.Event), "cloud", cloud)
			}
		}

		// The status write runs on EVERY sweep, not only when the transition
		// was newly recorded.
		//
		// Recording is once-only because a webhook must not be re-sent; the
		// status is the opposite kind of thing. It is a desired end state, and
		// re-asserting it is how the port self-heals -- if the sync overwrites
		// a monitor, or a row was missed, the next sweep puts it right. The
		// repository skips monitors already showing the target value, so the
		// steady state costs a read and no writes.
		changed, unknownType, err := s.applyMonitorStatus(ctx, c)
		if err != nil {
			return err
		}
		resp.MonitorsUpdated += changed
		if unknownType {
			resp.UnknownOutageType++
		}
	}
	return nil
}

// cloudsFor returns every cloud that must be told about this outage.
//
// TWO FLOWS, TWO ANSWERS, AND THEY DIFFER BY ARM. This port merges a pair of
// ServiceNow flows that do the same work off different triggers:
//
//	Cloud Status Event Notification Flow                fires on the outage
//	Cloud Status Event Notification Flow - Affected CI  fires on each affected CI
//
// The first posts for the outage's own cloud on BOTH arms. The second posts
// for the affected CI's own cloud and has only ONE arm -- its condition is
// "Outage is not Completed", so it never runs once the outage has ended.
//
//	ongoing    own cloud  +  every in-scope affected cloud
//	completed  own cloud only
//
// Fanning out on completion too would look like a tidy symmetry and would be
// wrong: it posts resolution webhooks to dashboards ServiceNow never tells,
// and on a public status page an unexpected all-clear is the worst direction
// to be wrong in.
//
// Unroutable clouds are dropped here rather than at the point of sending, so
// a cloud this build cannot address never reaches the events table and cannot
// become a webhook nobody can deliver.
func (s *cloudStatusService) cloudsFor(ctx context.Context, c repository.CloudStatusCandidate) ([]string, error) {
	seen := map[string]bool{}
	var out []string

	add := func(cloud string) {
		if cloud == "" || seen[cloud] || domain.CloudOfferingSlug(cloud) == "" {
			return
		}
		seen[cloud] = true
		out = append(out, cloud)
	}
	add(c.Cloud)

	// The completed arm is the outage-triggered flow alone.
	if c.Event == domain.CloudStatusEventOutageEnd {
		return out, nil
	}

	affected, err := s.repo.AffectedClouds(ctx, c.OutageID, s.parentServiceIDs)
	if err != nil {
		return nil, err
	}
	for _, cloud := range affected {
		if domain.CloudOfferingSlug(cloud) == "" {
			slog.WarnContext(ctx, "affected CI sits on a cloud this build cannot address; no webhook for it",
				"outageId", c.OutageID, "number", c.Number, "cloudOffering", cloud)
			continue
		}
		add(cloud)
	}
	return out, nil
}

// applyMonitorStatus writes the status every monitor affected by this outage
// should currently show. It reports how many rows changed and whether the
// outage's type had to be guessed at.
//
// This is steps 3-6 and 10-13 of the flow. The two arms differ only in the
// status they write: a completed outage returns its monitors to OPERATIONAL,
// an ongoing one sets the severity its type implies.
func (s *cloudStatusService) applyMonitorStatus(ctx context.Context, c repository.CloudStatusCandidate) (int64, bool, error) {
	var status domain.CloudMonitorStatus
	var unknownType bool

	if c.Event == domain.CloudStatusEventOutageEnd {
		// The completed arm wrote a literal 0. Note what it did NOT do: it did
		// not check whether some OTHER ongoing outage also affects these
		// monitors. Two overlapping outages on one component mean the first to
		// end clears the second's status, and the page shows Operational while
		// an incident is still running.
		//
		// That is faithfully reproduced here rather than fixed, because fixing
		// it changes what the public page says and needs a decision from
		// whoever owns it -- see this port's own notes. It is recorded so the
		// next person does not have to rediscover it from behaviour.
		status = domain.CloudMonitorStatusOperational
	} else {
		mapped, ok := domain.StatusForOngoingOutage(c.Type)
		if !ok {
			status = domain.CloudMonitorStatusUnknownType
			unknownType = true
			slog.ErrorContext(ctx, "ongoing outage has no usable type; falling back rather than leaving the status page claiming all is well",
				"outageId", c.OutageID, "number", c.Number,
				"outageType", c.Type, "fallback", string(status))
		} else {
			status = mapped
		}
	}

	monitors, err := s.repo.AffectedMonitors(ctx, c.OutageID)
	if err != nil {
		return 0, unknownType, err
	}
	if len(monitors) == 0 {
		// Common and not an error: most outages name no affected CIs at all,
		// and a CI that is not a service offering has no monitor. The webhook
		// still goes out -- it is routed from the outage's own configuration
		// item, not from this list.
		return 0, unknownType, nil
	}

	changed, err := s.repo.SetMonitorStatus(ctx, monitors, status)
	if err != nil {
		return 0, unknownType, err
	}
	if changed > 0 {
		slog.InfoContext(ctx, "cloud monitor status updated",
			"outageId", c.OutageID, "number", c.Number,
			"status", string(status), "monitorsChanged", changed)
	}
	return changed, unknownType, nil
}

// PendingWebhooks claims and returns the webhooks still owed to the
// dashboard, for the scheduled task to post.
//
// THE READ IS A CLAIM. Each returned row has an attempt started under its
// ClaimToken (repository.ClaimPending), so it cannot be posted by anyone else
// while the caller posts it -- in particular not by csm-notification-service
// handling outage.status_page_due for the same row. csm-scheduled-tasks needs
// no change for this: it already reports every row it reads.
//
// The cloud is translated to its wire slug here rather than in SQL so that the
// mapping lives in one place next to the reason it exists.
func (s *cloudStatusService) PendingWebhooks(ctx context.Context) (domain.PendingCloudStatusWebhooksResponse, error) {
	rows, err := s.repo.ClaimPending(ctx, cloudStatusPendingLimit, cloudStatusMaxAttempts, cloudStatusDeliveryLease)
	if err != nil {
		return domain.PendingCloudStatusWebhooksResponse{}, err
	}

	out := make([]domain.PendingCloudStatusWebhook, 0, len(rows))
	for _, w := range rows {
		slug := domain.CloudOfferingSlug(w.Cloud)
		wire := w.Event.WireValue()
		if slug == "" || wire == "" {
			// Recorded under an offering or an event this build cannot put on
			// the wire -- only reachable through version skew. Better held
			// back visibly than posted as something the dashboard ignores.
			// Claimed above, so its attempt is closed here as a definite
			// failure (nothing was sent); left open it would read as an
			// unknown outcome.
			slog.ErrorContext(ctx, "pending cloud status webhook cannot be put on the wire; not dispatching",
				"webhookId", w.ID, "cloudOffering", w.Cloud, "event", string(w.Event))
			if _, err := s.repo.RecordDelivery(ctx, w.ID, repository.DeliveryOutcome{
				Error: "no wire value for cloud " + w.Cloud + " / event " + string(w.Event), ClaimToken: w.ClaimToken,
			}); err != nil {
				slog.ErrorContext(ctx, "cloudstatus: closing an unpostable webhook failed", "webhookId", w.ID, "err", err)
			}
			continue
		}
		w.Cloud = slug
		w.WireEvent = wire
		out = append(out, w)
	}

	unknown, err := s.repo.UnknownOutcomes(ctx, cloudStatusPendingLimit)
	if err != nil {
		// Reporting only: the claimed rows above must still go out.
		slog.ErrorContext(ctx, "cloudstatus: reading webhooks with an unknown outcome failed", "err", err)
	}
	for _, w := range unknown {
		// Logged on every tick until someone settles it: this is the one
		// state the system cannot resolve by itself without risking a
		// duplicate on the public page.
		slog.ErrorContext(ctx, "cloudstatus: webhook outcome unknown -- check the status page; it will not be re-sent automatically",
			"webhookId", w.ID, "number", w.Number, "cloud", w.Cloud, "event", string(w.Event), "lastError", w.LastError)
	}
	return domain.PendingCloudStatusWebhooksResponse{Count: len(out), Webhooks: out, UnknownOutcome: len(unknown)}, nil
}

// RecordDelivery stamps the outcome of one webhook attempt. A ConflictError
// means there was no open attempt to record it against: the webhook is
// already delivered, or the report is for an attempt that is not current.
func (s *cloudStatusService) RecordDelivery(ctx context.Context, req domain.RecordCloudStatusDeliveryRequest) error {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return err
	}
	if req.ClaimToken != "" {
		if err := validateUUIDs("claimToken", []string{req.ClaimToken}); err != nil {
			return err
		}
	}
	if req.Delivered && req.Unknown {
		return &apierror.ValidationError{Msg: "unknown cannot be true when delivered is true"}
	}
	if !req.Delivered && strings.TrimSpace(req.Error) == "" {
		return &apierror.ValidationError{Msg: "error is required when delivered is false"}
	}
	recorded, err := s.repo.RecordDelivery(ctx, req.ID, repository.DeliveryOutcome{
		Delivered: req.Delivered, Unknown: req.Unknown, Error: req.Error, ClaimToken: req.ClaimToken,
	})
	if err != nil {
		return err
	}
	if !recorded {
		return &apierror.ConflictError{Msg: "delivery not recorded: the webhook is already delivered, or has no open attempt under this claim"}
	}
	return nil
}

// ClaimWebhook starts the attempt to post a webhook published as
// outage.status_page_due, under the claim token the event carried. A
// ConflictError means the caller must NOT post: the webhook is already
// delivered, or its reservation ran out and the scheduled task has it.
func (s *cloudStatusService) ClaimWebhook(ctx context.Context, req domain.ClaimCloudStatusWebhookRequest) error {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return err
	}
	if err := validateUUIDs("claimToken", []string{req.ClaimToken}); err != nil {
		return err
	}
	started, err := s.repo.StartAttempt(ctx, req.ID, req.ClaimToken, cloudStatusDeliveryLease)
	if err != nil {
		return err
	}
	if !started {
		return &apierror.ConflictError{Msg: "webhook not claimable: already delivered, or no longer reserved under this claim"}
	}
	return nil
}
