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
}

// eventPublisher abstracts eventbus.Producer for testability.
type eventPublisher interface {
	Publish(ctx context.Context, key, value []byte) error
}

// chatSender abstracts notifications.GoogleChatClient's SendSLABreachAlert
// for testability. Signature unchanged from the previous design — the
// bulk /sla-status response carries every field this alert needs directly,
// so there's no second per-clock lookup to build it from anymore.
type chatSender interface {
	SendSLABreachAlert(ctx context.Context, product, clockType, tier, caseNumber, wso2CaseID, caseTitle, caseType, productName, team, severity, state, openedAt, caseLink string) error
}

// linkResolver abstracts recipientlinks.Resolver's CSMLink for testability
// — the only method this engine needs from it (unlike
// internal/dispatch's own, larger linkResolver interface).
type linkResolver interface {
	CSMLink(caseID string) string
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
// as-is (ServiceNow's own SLA engine sets it, and it agrees with
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
	// defaultChatProduct is sendBreachAlert's fallback when a clock's own
	// Product (as returned by /sla-status) is empty — same "publisher
	// didn't say" fallback reasoning as dispatch.Dispatcher.
	// defaultChatProduct, reusing the same configured DEFAULT_CHAT_PRODUCT
	// value (see cmd/server/main.go).
	defaultChatProduct string
}

// NewEngine constructs an Engine.
func NewEngine(entity *EntityClient, store *TierStore, pub *eventbus.Producer, chat *notifications.GoogleChatClient, links *recipientlinks.Resolver, defaultChatProduct string) *Engine {
	return &Engine{entity: entity, store: store, pub: pub, chat: chat, links: links, defaultChatProduct: defaultChatProduct}
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
// A paused clock (s.IsPaused) is skipped outright: ServiceNow freezes
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
				slog.ErrorContext(ctx, "slaengine: failed to release tier claim after a failed alert, tier may be stuck until it expires", "caseId", s.CaseID, "clockType", s.ClockType, "tier", tier, "err", releaseErr)
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

	if err := e.sendBreachAlert(ctx, s, tier); err != nil {
		return fmt.Errorf("send sla breach alert: %w", err)
	}
	slog.InfoContext(ctx, "slaengine: sla tier reached", "caseId", s.CaseID, "clockType", s.ClockType, "tier", tier)
	return nil
}

// sendBreachAlert builds and sends the Google Chat breach card for one tier
// crossing, using s's own display fields — the bulk /sla-status response
// already carries all eight, so no second lookup is needed here (unlike the
// old per-clock GetClock design). product falls back to
// e.defaultChatProduct when s's own Product is empty, same reasoning as
// dispatch.Dispatcher's own Product fallback.
func (e *Engine) sendBreachAlert(ctx context.Context, s SLAStatus, tier int) error {
	product := s.Product
	if product == "" {
		product = e.defaultChatProduct
	}
	if product == "" {
		slog.WarnContext(ctx, "slaengine: sla breach alert not sent, no Google Chat product configured", "caseId", s.CaseID, "clockType", s.ClockType)
		return nil
	}
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
	return e.chat.SendSLABreachAlert(ctx, product, s.ClockType, tierLabel(tier), caseNumber, s.WSO2CaseID, s.CaseTitle, s.CaseType, s.Product, s.Team, s.Priority, s.State, openedAt, e.links.CSMLink(s.CaseID))
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
				slog.ErrorContext(ctx, "slaengine: tick failed", "err", err)
			}
		}
	}
}
