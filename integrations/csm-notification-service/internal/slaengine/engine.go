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

package slaengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
)

// statusLister abstracts EntityClient's FetchAllActiveSLAStatuses for
// testability.
type statusLister interface {
	FetchAllActiveSLAStatuses(ctx context.Context) ([]SLAStatus, error)
}

// tierStore abstracts TierStore for testability.
type tierStore interface {
	GetTier(ctx context.Context, caseID, clockType string) (tier int, found bool, err error)
	SetTier(ctx context.Context, caseID, clockType string, tier int) error
	ClaimTier(ctx context.Context, caseID, clockType string, tier int) (claimed bool, err error)
	ReleaseTier(ctx context.Context, caseID, clockType string, tier int) error
	ClaimEmail(ctx context.Context, caseID, clockType string, tier int) (claimed bool, err error)
}

// eventPublisher abstracts eventbus.Producer for testability.
type eventPublisher interface {
	Publish(ctx context.Context, key, value []byte) error
}

// chatSender abstracts notifications.GoogleChatClient's SendSLABreachAlert/
// HasAudienceSpace for testability. SendSLABreachAlert's first parameter is
// a Chat audience key (see chataudience.Resolve), not a product — SLA
// breach alerts route by team/standing-audience, per explicit product
// direction (see that function's own doc comment for why, and which other
// event types don't). The bulk /sla-status response carries every field
// this alert needs directly, so there's no second per-clock lookup to
// build it from.
type chatSender interface {
	SendSLABreachAlert(ctx context.Context, audience, clockType, tier, caseNumber, wso2CaseID, caseTitle, caseType, productName, team, teamLeadName, severity, state, openedAt, caseLink string) error
	HasAudienceSpace(audience string) bool
}

// linkResolver abstracts recipientlinks.Resolver's CSMLink for testability
// — the only method this engine needs from it (unlike
// internal/dispatch's own, larger linkResolver interface).
type linkResolver interface {
	CSMLink(caseID string) string
}

// emailSender abstracts notifications.EmailClient's SendEmail for
// testability — see sendBreachEmails.
type emailSender interface {
	SendEmail(ctx context.Context, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error
}

// tierSequence is the fixed set of elapsed-percentage checkpoints this
// engine alerts on, in ascending order — not configurable, same as the
// design this replaced.
var tierSequence = []int{50, 75, 100}

func tierLabel(tier int) string {
	return fmt.Sprintf("%d", tier)
}

// tierForStatus derives the highest checkpoint s currently satisfies from
// its live businessElapsedPercent — HasBreached is trusted directly for the
// 100% case rather than re-derived from the percentage alone, matching
// entity-service's own SLAStatus doc comment on why HasBreached is trusted
// as-is (the backing data source's own SLA engine sets it, and it agrees with
// BusinessElapsedPercent >= 100 in every case checked live).
func tierForStatus(s SLAStatus) int {
	switch {
	case s.HasBreached || s.BusinessElapsedPercent >= 100:
		return 100
	case s.BusinessElapsedPercent >= 75:
		return 75
	case s.BusinessElapsedPercent >= 50:
		return 50
	default:
		return 0
	}
}

// Engine is the SLA breach-alerting engine: RunTicker/Tick periodically poll
// entity-service's GET /sla-status (see client.go), diff each clock's
// current tier against the last tier this engine has seen for it (stored in
// TierStore — see redis.go), and — on a genuine new crossing — send a
// Google Chat alert directly (see sendBreachAlert; not routed through
// internal/dispatch) and publish events.TypeSLATierReached.
type Engine struct {
	entity statusLister
	store  tierStore
	pub    eventPublisher
	chat   chatSender
	links  linkResolver
	email  emailSender

	// emailSendingEnabled/emailDebugMode/emailDebugRecipients mirror
	// dispatch.Dispatcher's own three email-specific controls exactly
	// (EMAIL_SENDING_ENABLED/EMAIL_DEBUG_MODE/EMAIL_DEBUG_RECIPIENTS, the
	// same env vars, read once in cmd/server/main.go and passed to both
	// this Engine and dispatch.NewDispatcher) — see sendBreachEmails for
	// where they're applied. This engine deliberately does NOT have its
	// own defaultCSMEmailCC equivalent: the breach emails below are
	// addressed directly (assignee, team) rather than split into a
	// customer/CSM portal-link group the way dispatch.sendPerGroup's
	// case.* emails are, so there is no "CSM-portal group" to CC in the
	// first place.
	emailSendingEnabled  bool
	emailDebugMode       bool
	emailDebugRecipients []string

	// now is the clock chataudience.Resolve's time-of-day/weekend
	// audiences are decided against; nil means time.Now. Tests pin it so
	// an alert's audience set does not depend on when the suite runs.
	now func() time.Time
}

// clock returns the engine's current time (see Engine.now).
func (e *Engine) clock() time.Time {
	if e.now != nil {
		return e.now()
	}
	return time.Now()
}

// NewEngine constructs an Engine. emailSendingEnabled/emailDebugMode/
// emailDebugRecipients gate sendBreachEmails exactly the way the same
// three values gate every email dispatch.Dispatcher sends — see Engine's
// own doc comment on those fields.
func NewEngine(entity *EntityClient, store *TierStore, pub *eventbus.Producer, chat *notifications.GoogleChatClient, links *recipientlinks.Resolver, email *notifications.EmailClient, emailSendingEnabled, emailDebugMode bool, emailDebugRecipients []string) *Engine {
	return &Engine{
		entity:               entity,
		store:                store,
		pub:                  pub,
		chat:                 chat,
		links:                links,
		email:                email,
		emailSendingEnabled:  emailSendingEnabled,
		emailDebugMode:       emailDebugMode,
		emailDebugRecipients: emailDebugRecipients,
	}
}

// Tick polls every currently-active SLA clock and processes each — a failed
// clock doesn't stop the others; every error is joined and returned so
// RunTicker can log the whole batch's outcome in one line. Takes no explicit
// "now": every tier decision comes from each clock's own live
// businessElapsedPercent (see processStatus), not a comparison against a
// point in time the way the wake-index design this replaced needed.
func (e *Engine) Tick(ctx context.Context) error {
	statuses, err := e.entity.FetchAllActiveSLAStatuses(ctx)
	if err != nil {
		return fmt.Errorf("slaengine: fetch active sla statuses: %w", err)
	}

	var errs []error
	for _, s := range statuses {
		if err := e.processStatus(ctx, s); err != nil {
			errs = append(errs, fmt.Errorf("%s/%s: %w", s.CaseID, s.ClockType, err))
		}
	}
	return errors.Join(errs...)
}

// processStatus diffs one clock's current tier against TierStore's recorded
// cursor for it and reacts:
//
//   - No cursor yet (first time this engine has ever seen this exact
//     (caseID, clockType) pair, or its cursor expired — see tierTTL):
//     seed the cursor at the clock's CURRENT tier WITHOUT alerting. This is
//     deliberate, not an oversight: entity-service's "sla" table already
//     holds around 120,000 pre-existing in-progress rows the first time
//     this engine polls after this redesign deploys, most of them already
//     well past 50%/75% elapsed — alerting for every one of those on first
//     sight would flood Chat with alerts for SLAs that have been sitting at
//     that percentage for a long time, not ones that just crossed it. Only
//     a tier crossed on a SUBSEQUENT poll, relative to this seeded
//     baseline, is a genuine new crossing worth alerting on.
//   - Current tier is BELOW the stored cursor: the clock's percentage has
//     gone backwards since the last poll — entity-service resets an SLA
//     policy (about 0.5% of clocks have had this happen at least once, per
//     a live check) or a genuinely new tracking cycle started under the
//     same case/clock-type. Reseed the cursor at the new, lower tier
//     without alerting, same reasoning as the no-cursor case: this is a new
//     cycle, not a regression to warn about.
//   - Current tier is AT the stored cursor: nothing crossed since the last
//     poll — no-op.
//   - Current tier is ABOVE the stored cursor: for every checkpoint
//     strictly between the stored cursor and the current tier, in
//     ascending order, atomically claim that tier via TierStore.ClaimTier
//     (a Redis SETNX) before alerting for it — see ClaimTier's own doc
//     comment for why a plain read-then-write on the cursor alone isn't
//     enough: if this service is ever deployed with more than one replica,
//     two replicas can both read the same stale cursor and both decide to
//     alert for the same tier at once. Only the replica whose ClaimTier
//     call actually wins alerts; a losing call just moves on to the next
//     tier. A failure to alert after winning the claim releases it (so a
//     later tick — this replica or another — can retry that exact tier
//     rather than losing it for good), and advances the cursor only after
//     a tier's alert actually succeeds, so a failure partway through a
//     multi-tier crossing still keeps whatever alerted successfully and
//     retries only the remainder on the next Tick.
//
// A paused clock (s.IsPaused) is skipped outright: the backing data source freezes
// businessElapsedPercent while paused, so there is nothing to cross either
// way, and skipping avoids a pointless Redis round trip for every paused
// clock on every poll.
func (e *Engine) processStatus(ctx context.Context, s SLAStatus) error {
	if s.IsPaused {
		return nil
	}

	current := tierForStatus(s)
	last, found, err := e.store.GetTier(ctx, s.CaseID, s.ClockType)
	if err != nil {
		return fmt.Errorf("get tier cursor: %w", err)
	}

	if !found {
		if err := e.store.SetTier(ctx, s.CaseID, s.ClockType, current); err != nil {
			return fmt.Errorf("seed tier cursor: %w", err)
		}
		slog.InfoContext(ctx, "slaengine: seeded sla tier baseline, no alert on first sight", "caseId", s.CaseID, "clockType", s.ClockType, "tier", current)
		return nil
	}

	if current < last {
		// A regression invalidates any claim this engine made for a tier
		// above the new, lower one — those belonged to a now-superseded
		// cycle (see the doc comment above). Release them so a later
		// re-crossing under the new cycle can claim and alert again;
		// without this, a stale claim from the old cycle would silently
		// swallow the new cycle's own genuine crossing.
		for _, tier := range tierSequence {
			if tier <= current {
				continue
			}
			if err := e.store.ReleaseTier(ctx, s.CaseID, s.ClockType, tier); err != nil {
				return fmt.Errorf("release tier claim %d after regression: %w", tier, err)
			}
		}
		if err := e.store.SetTier(ctx, s.CaseID, s.ClockType, current); err != nil {
			return fmt.Errorf("reseed tier cursor after regression: %w", err)
		}
		slog.InfoContext(ctx, "slaengine: sla clock tier regressed (policy reset or new cycle), rebaselined without alerting", "caseId", s.CaseID, "clockType", s.ClockType, "from", last, "to", current)
		return nil
	}

	for _, tier := range tierSequence {
		if tier <= last || tier > current {
			continue
		}
		claimed, err := e.store.ClaimTier(ctx, s.CaseID, s.ClockType, tier)
		if err != nil {
			return fmt.Errorf("claim tier %d: %w", tier, err)
		}
		if !claimed {
			// Some other call already owns this tier — a concurrent
			// replica, or an earlier attempt that's already alerted for
			// it. Either way, this call must not alert again; move on to
			// whatever tier comes next. The cursor is intentionally left
			// untouched here — the call that actually won the claim
			// advances it once its own alert succeeds, and this replica
			// picks up the advanced value on its own next poll.
			continue
		}
		if err := e.alertTier(ctx, s, tier); err != nil {
			if releaseErr := e.store.ReleaseTier(ctx, s.CaseID, s.ClockType, tier); releaseErr != nil {
				slog.ErrorContext(ctx, "slaengine: failed to release tier claim after a failed alert, tier may be stuck until it expires", "caseId", s.CaseID, "clockType", s.ClockType, "tier", tier, "err", apierror.Summary(releaseErr))
			}
			return fmt.Errorf("alert tier %d: %w", tier, err)
		}
		if err := e.store.SetTier(ctx, s.CaseID, s.ClockType, tier); err != nil {
			return fmt.Errorf("advance tier cursor to %d: %w", tier, err)
		}
	}
	return nil
}

// alertTier publishes events.TypeSLATierReached and sends the Google Chat
// breach card for one newly-crossed tier — only ever called after
// processStatus has already won that tier's Redis claim (see ClaimTier),
// so this function itself needs no additional idempotency of its own.
func (e *Engine) alertTier(ctx context.Context, s SLAStatus, tier int) error {
	envelope := events.Envelope{Type: events.TypeSLATierReached, EntityID: s.CaseID}
	payload, err := json.Marshal(events.SLATierReachedPayload{CaseID: s.CaseID, ClockType: s.ClockType, Tier: tierLabel(tier)})
	if err != nil {
		return fmt.Errorf("encode sla.tier_reached payload: %w", err)
	}
	envelope.Payload = payload
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode sla.tier_reached envelope: %w", err)
	}
	if err := e.pub.Publish(ctx, []byte(s.CaseID), body); err != nil {
		return fmt.Errorf("publish sla.tier_reached: %w", err)
	}

	chatErr := e.sendBreachAlert(ctx, s, tier)
	// Breach emails are attempted regardless of the Chat alert's own
	// outcome: a Chat space outage must not also suppress email, which
	// would otherwise happen silently if the clock completes (and so drops
	// out of the active /sla-status list) before Chat recovers and this
	// tier gets a retry. Best-effort, deliberately not folded into this
	// function's own error return: a transient email failure must never
	// cause processStatus to release this tier's claim and retry the WHOLE
	// tier — see dispatch.go's beginRecord/endRecord history for the class
	// of duplicate-send bug that would reintroduce for the Chat alert
	// above. e.store.ClaimEmail (checked inside sendBreachEmails) ensures a
	// retry caused solely by the Chat error below doesn't re-attempt an
	// already-attempted email.
	e.sendBreachEmails(ctx, s, tier)
	if chatErr != nil {
		return fmt.Errorf("send sla breach alert: %w", chatErr)
	}
	slog.InfoContext(ctx, "slaengine: sla tier reached", "caseId", s.CaseID, "clockType", s.ClockType, "tier", tier)
	return nil
}

// sendBreachAlert builds and sends the Google Chat breach card for one tier
// crossing, once per resolved Chat audience — team/standing-audience
// routing (see chataudience.Resolve and chatSender's own doc comment for
// why, and which other event types don't route this way), not
// product-based: unlike the case.*/incident.* cards, there is no "no
// audience configured" skip here — Resolve always returns at least the
// "Incident Monitor" fallback, so this alert is never silently dropped for
// lack of a routing value. s's own display fields (the bulk /sla-status
// response already carries all of them, so no second lookup is needed
// here, unlike the old per-clock GetClock design) are shared unchanged
// across every audience's own card. A failure on any one audience fails
// the whole call (errors.Join) — alertTier's own caller already retries
// the entire tier (including the Kafka publish) on any sendBreachAlert
// error, so this doesn't weaken that existing retry contract, just applies
// it across however many audiences resolved instead of one.
func (e *Engine) sendBreachAlert(ctx context.Context, s SLAStatus, tier int) error {
	caseNumber := s.CaseNumber
	if caseNumber == "" {
		// s.CaseNumber can be empty for a work item entity-service's own
		// case-like joins don't cover — fall back to the raw case id so the
		// card still has something to show rather than a blank "Case ID :"
		// line.
		caseNumber = s.CaseID
	}
	var openedAt string
	if s.StartedOn != nil && !s.StartedOn.IsZero() {
		openedAt = s.StartedOn.UTC().Format("2006-01-02 15:04:05") + " (UTC)"
	}

	audiences := chataudience.Resolve(s.Team, s.IsEvaluationAccount, s.ProjectOnboardingStatus, e.clock(), e.chat.HasAudienceSpace)
	caseLink := e.links.CSMLink(s.CaseID)
	var errs []error
	for _, audience := range audiences {
		if err := e.chat.SendSLABreachAlert(ctx, audience, s.ClockType, tierLabel(tier), caseNumber, s.WSO2CaseID, s.CaseTitle, s.CaseType, s.Product, s.Team, s.TeamLeadName, s.Priority, s.State, openedAt, caseLink); err != nil {
			errs = append(errs, fmt.Errorf("audience %q: %w", audience, err))
		}
	}
	return errors.Join(errs...)
}

// sendBreachEmails sends the same tier crossing as two separate emails —
// one to the case's assigned engineer, one to the case's team email group
// — on top of (not instead of) the Chat alert above. Best-effort by
// design: see alertTier's own call site for why a failure here must never
// propagate and trigger a retry of the whole tier. Either recipient is
// skipped silently (logged at INFO, not WARN/ERROR) when entity-service
// couldn't resolve it (s.AssigneeEmail/s.TeamEmail empty) — a case with no
// assignee, or a team with no configured group_email, is a normal state,
// not a misconfiguration this engine can fix; production has no real
// address to send to in that case regardless of recipient.
//
// The team recipient is the one exception, and only while emailDebugMode
// is on: when s.Team (the team's name) is known but s.TeamEmail isn't (the
// team itself resolved, its group just has no configured group_email yet —
// common while entity-service's team data is still being backfilled), the
// email is still sent to e.emailDebugRecipients, with the body's "Sent to"
// line naming the team directly — so confirming the routing itself works
// doesn't depend on every test environment also having every team's
// group_email filled in. An unassigned case's assignee has no equivalent
// name worth surfacing this way, so that recipient is skipped the same in
// debug mode as in production — see the send closure's own doc comment.
//
// Claims (caseID, clockType, tier) via e.store.ClaimEmail before sending
// anything: alertTier now attempts this regardless of whether the Chat
// alert itself succeeded, so a tier retried solely because Chat failed
// must not re-send an already-attempted email — see emailClaimKeyPrefix's
// own doc comment. A claim failure (Redis error) is logged and treated the
// same as "already claimed" — skip rather than risk a duplicate send on an
// indeterminate claim result.
func (e *Engine) sendBreachEmails(ctx context.Context, s SLAStatus, tier int) {
	if e.email == nil || !e.emailSendingEnabled {
		return
	}
	claimed, err := e.store.ClaimEmail(ctx, s.CaseID, s.ClockType, tier)
	if err != nil {
		slog.ErrorContext(ctx, "slaengine: failed to claim sla breach email, skipping to avoid a duplicate send", "caseId", s.CaseID, "clockType", s.ClockType, "tier", tier, "err", apierror.Summary(err))
		return
	}
	if !claimed {
		return
	}
	caseLink := e.links.CSMLink(s.CaseID)
	// s.CaseNumber can be empty for a work item entity-service's own case-like
	// joins don't cover — fall back to the raw case id, same as
	// sendBreachAlert's own Chat card above, so the email's case reference is
	// never blank.
	caseNumber := s.CaseNumber
	if caseNumber == "" {
		caseNumber = s.CaseID
	}
	data := notifications.SLABreachEmailData{
		ClockType:    s.ClockType,
		Tier:         tierLabel(tier),
		Severity:     s.Priority,
		CaseNumber:   caseNumber,
		WSO2CaseID:   s.WSO2CaseID,
		CaseTitle:    s.CaseTitle,
		CaseType:     s.CaseType,
		Product:      s.Product,
		Team:         s.Team,
		TeamLeadName: s.TeamLeadName,
		State:        s.State,
		CaseLink:     caseLink,
	}
	if s.StartedOn != nil && !s.StartedOn.IsZero() {
		data.OpenedAt = s.StartedOn.UTC().Format("2006-01-02 15:04:05") + " (UTC)"
	}
	subject := notifications.SLABreachEmailSubject(s.ClockType, data.Tier, s.Priority, caseNumber, s.WSO2CaseID)

	// recipientRole ("assignee"/"team") is logged on every outcome below
	// specifically so a skip/failure is attributable to one recipient or
	// the other — without it, both calls log an identical line (same
	// caseId/clockType/tier), making "which recipient was this about"
	// unanswerable from the log alone, a real gap hit while diagnosing a
	// missing breach email live.
	//
	// unresolvedLabel, when non-empty, names who this recipient WOULD have
	// been (e.g. the team name) even though real is empty — real
	// production has nowhere to send regardless, but a debug deployment
	// can still show it: sent to the configured debug recipients, with the
	// body's "Sent to" line naming unresolvedLabel directly, so testing
	// whether team routing itself resolved correctly doesn't depend on
	// this environment also having that team's group_email configured.
	// Assignee passes "" here — an unassigned case has no name worth
	// surfacing this way, see sendBreachEmails' own doc comment.
	send := func(recipientRole, real, unresolvedLabel string, render func(intendedFor string) string) {
		if real == "" {
			if !e.emailDebugMode || unresolvedLabel == "" {
				slog.InfoContext(ctx, "slaengine: sla breach email recipient not resolved, skipping", "caseId", s.CaseID, "clockType", s.ClockType, "tier", tier, "recipient", recipientRole)
				return
			}
			if len(e.emailDebugRecipients) == 0 {
				slog.WarnContext(ctx, "slaengine: EMAIL_DEBUG_MODE=true but EMAIL_DEBUG_RECIPIENTS is empty; not sending breach email",
					"caseId", s.CaseID, "clockType", s.ClockType, "tier", tier, "recipient", recipientRole)
				return
			}
			body := render(unresolvedLabel + " (no email on file)")
			if err := e.email.SendEmail(ctx, e.emailDebugRecipients, nil, nil, nil, subject, body, nil); err != nil {
				slog.ErrorContext(ctx, "slaengine: failed to send sla breach email", "caseId", s.CaseID, "clockType", s.ClockType, "tier", tier, "recipient", recipientRole, "err", apierror.Summary(err))
				return
			}
			slog.InfoContext(ctx, "slaengine: sla breach email sent", "caseId", s.CaseID, "clockType", s.ClockType, "tier", tier, "recipient", recipientRole, "unresolvedLabel", unresolvedLabel)
			return
		}
		to := []string{real}
		var intendedFor string
		if e.emailDebugMode {
			if len(e.emailDebugRecipients) == 0 {
				slog.WarnContext(ctx, "slaengine: EMAIL_DEBUG_MODE=true but EMAIL_DEBUG_RECIPIENTS is empty; not sending breach email",
					"caseId", s.CaseID, "clockType", s.ClockType, "tier", tier, "recipient", recipientRole)
				return
			}
			intendedFor = real
			to = e.emailDebugRecipients
		}
		body := render(intendedFor)
		if err := e.email.SendEmail(ctx, to, nil, nil, nil, subject, body, nil); err != nil {
			slog.ErrorContext(ctx, "slaengine: failed to send sla breach email", "caseId", s.CaseID, "clockType", s.ClockType, "tier", tier, "recipient", recipientRole, "err", apierror.Summary(err))
			return
		}
		slog.InfoContext(ctx, "slaengine: sla breach email sent", "caseId", s.CaseID, "clockType", s.ClockType, "tier", tier, "recipient", recipientRole)
	}

	send("assignee", s.AssigneeEmail, "", func(intendedFor string) string {
		d := data
		d.IntendedFor = intendedFor
		return notifications.RenderSLABreachAssigneeEmail(s.AssigneeName, d)
	})
	send("team", s.TeamEmail, s.Team, func(intendedFor string) string {
		d := data
		d.IntendedFor = intendedFor
		return notifications.RenderSLABreachTeamEmail(s.TeamLeadName, d)
	})
}

// RunTicker calls Tick every interval until ctx is done. Run from its own
// goroutine (see cmd/server/main.go); a failed Tick is logged, not fatal —
// the next tick gets another chance at whatever needs (re)checking.
func (e *Engine) RunTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.Tick(ctx); err != nil {
				slog.ErrorContext(ctx, "slaengine: tick failed", "err", apierror.Summary(err))
			}
		}
	}
}
