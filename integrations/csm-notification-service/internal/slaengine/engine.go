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
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/businessdays"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
)

// Clock types — entity-service's own sla_duration_policy.clock_type
// vocabulary (lowercase), matching sla_policy.target's existing lower-cased
// convention this package already used on the wire (see the removed poll
// design's own SLAStatus.ClockType).
const (
	ClockResponse   = "response"
	ClockWorkaround = "workaround"
	ClockResolution = "resolution"
)

// clockTypesInOrder is every clock type RegisterClocks considers, in a
// fixed order purely for deterministic logging — registration itself
// doesn't depend on the order, since each is independently looked up in
// the duration policy map.
var clockTypesInOrder = []string{ClockResponse, ClockWorkaround, ClockResolution}

// tierSequence is the fixed set of elapsed-duration checkpoints this engine
// tracks per clock — 50%, 75%, 100% of the way from a clock's own start to
// its due date. Not configurable, ported as-is from the pre-poll design.
var tierSequence = []int{50, 75, 100}

func tierLabel(tier int) string {
	return strconv.Itoa(tier)
}

// redisRetryAttempts/redisRetryDelay bound the retry this engine applies to
// its own Redis calls on the hot tick/check path — a short, transient blip
// (a brief network hiccup, a Redis failover) should not immediately fall
// back to the entity-service degraded path (processDueMemberFallback),
// which can only ever reconstruct a best-effort, metadata-poor alert.
const (
	redisRetryAttempts = 3
	redisRetryDelay    = 150 * time.Millisecond
)

// withRedisRetry runs fn up to redisRetryAttempts times, pausing
// redisRetryDelay between attempts, and returns the last error if every
// attempt fails. ctx cancellation aborts immediately between attempts.
func withRedisRetry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 1; attempt <= redisRetryAttempts; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if attempt == redisRetryAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(redisRetryDelay):
		}
	}
	return err
}

// store abstracts Store for testability.
type store interface {
	AddWake(ctx context.Context, member string, at time.Time) error
	RemoveWake(ctx context.Context, member string) error
	DueMembers(ctx context.Context, now time.Time) ([]string, error)
	SetClock(ctx context.Context, caseID, clockType string, meta ClockMeta) error
	GetClock(ctx context.Context, caseID, clockType string) (ClockMeta, bool, error)
	SetPaused(ctx context.Context, caseID, clockType string, paused bool) error
	ShiftWake(ctx context.Context, caseID, clockType string, deltaSeconds, newBaselineSeconds int64) error
	SetWakeAbsolute(ctx context.Context, caseID, clockType string, wakes map[int]time.Time, newBaselineSeconds int64) error
	SetShiftedPauseSeconds(ctx context.Context, caseID, clockType string, seconds int64) error
	SetState(ctx context.Context, caseID, clockType, state string) error
	AdvanceAlertedTier(ctx context.Context, caseID, clockType string, tier int, incarnation time.Time) (applied bool, err error)
	ClaimTier(ctx context.Context, caseID, clockType string, tier int, startedAt time.Time) (claimed bool, err error)
	ReleaseTier(ctx context.Context, caseID, clockType string, tier int, startedAt time.Time) error
}

// eventPublisher abstracts eventbus.Producer for testability.
type eventPublisher interface {
	Publish(ctx context.Context, key, value []byte) error
}

// chatSender abstracts notifications.GoogleChatClient's SendSLABreachAlert/
// HasAudienceSpace for testability. teamLeadName is always sent "" — see
// Engine.sendBreachAlert's own doc comment for why this engine has no way
// to resolve it any more.
type chatSender interface {
	SendSLABreachAlert(ctx context.Context, audience, clockType, tier, caseNumber, wso2CaseID, caseTitle, caseType, productName, team, teamLeadName, severity, state, openedAt, caseLink string) error
	HasAudienceSpace(audience string) bool
}

// linkResolver abstracts recipientlinks.Resolver's CSMLink for testability.
type linkResolver interface {
	CSMLink(caseID string) string
}

// entityClockStatusClient abstracts EntityClient.GetClockState for
// testability — the final, authoritative check against entity-service's own
// durable record, done right before a breach alert is actually sent (see
// that method's own doc comment for why). A nil value (e.g. every existing
// test's own Engine literal, which doesn't set this field) skips the check
// entirely and behaves exactly as this engine always has — this is a
// deliberate, additive safety net, not something every caller must now wire
// up.
type entityClockStatusClient interface {
	GetClockState(ctx context.Context, caseID, clockType string) (clockState, error)
}

// entityClockStateRetryAttempts/entityClockStateRetryDelay bound a short
// retry around entity-service's own GetClockState when it reports a clock as
// not yet found — entity-service's SLA engine registers/applies a case's
// real clocks synchronously, inside the same request that publishes
// case.created/case.status_changed, but it does that AFTER publishing, not
// before (an ordering this package doesn't control and can't safely
// reorder — see entity-service's own sn_case_service.go, CreateCase/
// UpdateCase). A Kafka consumer reacting to that same event can therefore
// race ahead of entity-service's own registration/pause-application and see
// "not found" on its very first read, even though entity-service is a
// moment away from writing the row this read is asking about. This retries
// a couple of times, briefly, rather than accepting that first "not found"
// as final — both RegisterClocks (right after case.created) and
// entityPauseStatus (right after case.status_changed) call
// getClockStateWithRetry for exactly this reason. Kept short on purpose:
// "not found" is also the ordinary, permanent answer for a clock type a
// case's policy genuinely never registers (e.g. a LOW/S4 case with no
// workaround/resolution policy at all), which is the overwhelmingly more
// common case on every status change after the first — a long or
// many-attempt retry would pay that cost needlessly on every single one.
const (
	entityClockStateRetryAttempts = 2
	entityClockStateRetryDelay    = 300 * time.Millisecond
)

// getClockStateWithRetry calls entity.GetClockState, retrying briefly when
// the lookup itself succeeds but reports the clock as not yet found — see
// entityClockStateRetryAttempts's own doc comment for why. A transport/
// lookup error is returned immediately, unretried here: that's each
// caller's own existing, separate concern (already logged and handled by
// treating the clock as absent/not-paused).
func getClockStateWithRetry(ctx context.Context, entity entityClockStatusClient, caseID, clockType string) (clockState, error) {
	var state clockState
	var err error
	for attempt := 1; attempt <= entityClockStateRetryAttempts; attempt++ {
		state, err = entity.GetClockState(ctx, caseID, clockType)
		if err != nil || state.Found {
			return state, err
		}
		if attempt == entityClockStateRetryAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return clockState{}, ctx.Err()
		case <-time.After(entityClockStateRetryDelay):
		}
	}
	return state, err
}

// clockIncarnationTolerance is how far apart entity-service's own
// sla.start_on and this engine's own Redis-held ClockMeta.StartedAt may be
// while still being treated as the SAME clock incarnation — the two are set
// independently, by two different services, reacting to (normally) the same
// case.created moment a few milliseconds apart, so exact equality is too
// strict. A genuinely NEW incarnation (a severity revision cancels the old
// clock and registers a fresh one) starts at a point at least as far apart
// as however long it took a human to act — realistically minutes to days,
// never mere seconds — so this stays comfortably below that while easily
// covering ordinary processing lag. See isSameClockIncarnation's own call
// site (sendBreachAlert) for why this distinction matters at all.
const clockIncarnationTolerance = 5 * time.Minute

// isSameClockIncarnation reports whether entityStartedOn (entity-service's
// own record) and redisStartedAt (this engine's own ClockMeta.StartedAt)
// are close enough to be the same registration rather than two different
// ones for the same (case, clockType) pair. A zero redisStartedAt (should
// never happen for a real wake entry, but defensively handled) can't be
// compared against anything, so this reports true rather than refusing to
// ever trust entity-service's answer for such a clock.
func isSameClockIncarnation(entityStartedOn *time.Time, redisStartedAt time.Time) bool {
	if entityStartedOn == nil || redisStartedAt.IsZero() {
		return true
	}
	diff := entityStartedOn.Sub(redisStartedAt)
	if diff < 0 {
		diff = -diff
	}
	return diff <= clockIncarnationTolerance
}

// isTerminalSLAStage reports whether stage is one of this engine's
// terminal, genuinely-finished sla.stage values — mirrors entity-service's
// own slaEngineOpenStageFilter (sla_engine_repo.go): ACHIEVED/CANCELLED/
// COMPLETED are real completions, BREACHED deliberately is not (a clock
// whose wall-clock duration ran out without yet being satisfied is still
// live, still completable by its own later real event).
func isTerminalSLAStage(stage string) bool {
	switch strings.ToUpper(stage) {
	case "ACHIEVED", "CANCELLED", "COMPLETED":
		return true
	default:
		return false
	}
}

// Engine is the SLA breach-alerting engine — see this package's own doc
// comment (client.go) for the full design and what it replaced.
type Engine struct {
	store store
	pub   eventPublisher
	chat  chatSender
	links linkResolver
	// entity is this engine's one live link to entity-service's own durable
	// record — RegisterClocks/reconcileClock read a clock's real, resolved
	// policy duration from it (entity-service's own GET /sla-duration-policy
	// and this engine's static severity-keyed duration table are both gone
	// — see RegisterClocks' own doc comment), ApplyStateEffects reads a
	// clock's real pause status from it, and sendBreachAlert/
	// processDueMemberFallback use it as a last, authoritative check/
	// fallback — see each one's own doc comment.
	entity entityClockStatusClient
	// slaEmailSendingEnabled is the SLA notification killswitch
	// (SLA_EMAIL_SENDING_ENABLED, default true; disabled only by the literal
	// "false", same disable-entirely convention as
	// EMAIL_SENDING_ENABLED/CALL_SENDING_ENABLED in cmd/server/main.go). The
	// name is kept for config compatibility, but when false it suppresses
	// every SLA-triggered outbound message: the Google Chat breach card sent
	// by sendBreachAlert (the only SLA Chat send in this service) and any SLA
	// breach email should that reaction be rebuilt. Non-SLA alerts are
	// unaffected.
	slaEmailSendingEnabled bool
	// now is the clock chataudience.Resolve is given, so a test can pin
	// the time of day the Americas/weekend audiences depend on. nil means
	// time.Now; see clock.
	now func() time.Time
}

// clock returns e.now(), or time.Now() when no clock is set.
func (e *Engine) clock() time.Time {
	if e.now != nil {
		return e.now()
	}
	return time.Now()
}

// NewEngine constructs an Engine. entity is the same *EntityClient the
// caller already constructs for Reconcile — see entityClockStatusClient's
// own doc comment for what it's used for here. slaEmailSendingEnabled gates
// all SLA-triggered messages (Chat card and email) — see that field's own
// doc comment.
func NewEngine(store *Store, pub *eventbus.Producer, chat chatSender, links *recipientlinks.Resolver, entity entityClockStatusClient, slaEmailSendingEnabled bool) *Engine {
	return &Engine{store: store, pub: pub, chat: chat, links: links, entity: entity, slaEmailSendingEnabled: slaEmailSendingEnabled}
}

// avoidWeekend rolls due forward to the next Monday, same time-of-day, if
// it falls on a Saturday or Sunday — approximates MEDIUM/S3's resolution
// duration being published as "1 Business Week" rather than a flat 7×24h.
// Ported from the pre-poll design's own helper.
func avoidWeekend(due time.Time) time.Time {
	switch due.Weekday() {
	case time.Saturday:
		return due.AddDate(0, 0, 2)
	case time.Sunday:
		return due.AddDate(0, 0, 1)
	default:
		return due
	}
}

// slaScheduleWSO2BusinessDays mirrors entity-service's own
// repository.slaScheduleWSO2BusinessDays (unexported there, kept as a
// separate literal here, same as sla_policy_resolver.go's own
// slaScheduleWSO2BusinessDaysLabel does in that repo) — the one schedule
// value this engine treats as business-day-aware; every other value
// (including "", "24 x 7", or an unsupported one like "8-5 weekdays") is
// plain flat wall-clock time, matching entity-service's own posture.
const slaScheduleWSO2BusinessDays = "WSO2 Business days"

// dueInstant returns the instant reached by adding duration d to startedAt,
// using business-day-aware math (businessdays.AddBusinessDuration) when
// schedule is 'WSO2 Business days', flat wall-clock Add(d) for every other
// schedule — the same choice entity-service's own SLA engine makes when
// computing a live clock's remaining-time/percentage figures
// (internal/repository/sla_engine_repo.go's businessDayElapsedSnapshot),
// now mirrored on this side of the wire too. Before this, RegisterClocks/
// reconcileClock always used flat Add(duration) regardless of schedule — a
// real bug found in code review: a 240h business-day RESOLUTION clock
// starting on a Friday was woken at Friday+240h calendar time (ten
// calendar days later) instead of the real business-day-adjusted instant
// (which skips two weekends along the way), firing premature breach
// alerts well before the clock was actually due by entity-service's own
// reckoning.
func dueInstant(startedAt time.Time, d time.Duration, schedule string) time.Time {
	if schedule == slaScheduleWSO2BusinessDays {
		return businessdays.AddBusinessDuration(startedAt, d)
	}
	return startedAt.Add(d)
}

// tierFraction returns tier's share of d (50% -> d/2, 75% -> d*3/4, 100% ->
// d) — split out from tierTime so Engine.resumeShift's business-day branch
// can compute the same fraction without going through dueInstant, which
// anchors at a start instant this call site doesn't have (it adds the
// fraction to the clock's already-known duration-plus-pause total instead).
func tierFraction(d time.Duration, tier int) time.Duration {
	switch tier {
	case 50:
		return d / 2
	case 75:
		return d * 3 / 4
	default: // 100
		return d
	}
}

// tierTime returns the instant tier is reached, given a clock that started
// at startedAt and runs for duration d under the given schedule — ported
// from the pre-poll design's own helper, now schedule-aware via dueInstant.
func tierTime(startedAt time.Time, d time.Duration, tier int, schedule string) time.Time {
	return dueInstant(startedAt, tierFraction(d, tier), schedule)
}

// dueAtAndTierDuration computes a clock's overall due instant and the
// duration tierTime should treat each 50/75/100% checkpoint as a fraction
// of — shared by RegisterClocks and reconcileClock so the business-day fix
// lives in exactly one place.
//
// For a flat (non-business-day) schedule this reproduces the pre-existing
// behavior byte for byte: dueAt is startedAt.Add(duration), optionally
// rolled by avoidWeekend when resolutionAvoidWeekend is set (MEDIUM/S3
// resolution's "1 Business Week" approximation), and tierDuration is the
// resulting, possibly-rolled span — so a MEDIUM resolution clock's 50%/75%
// checkpoints shift along with the rolled due date, exactly as the old,
// single-path tierTime(startedAt, actualDuration, tier) call always did.
//
// For a 'WSO2 Business days' schedule, tierDuration is the real, pure
// policy duration instead: dueInstant's own business-day math already skips
// weekends correctly for each fractional checkpoint on its own (a 50%
// checkpoint computed as half the real duration, business-day-adjusted, is
// not the same instant as half of a wall-clock span that itself already had
// two weekends baked into it by Sub) — and avoidWeekend is a guaranteed
// no-op here regardless, since businessdays.AddBusinessDuration never
// returns a weekend instant in the first place (see that function's own
// doc comment).
func dueAtAndTierDuration(startedAt time.Time, duration time.Duration, schedule string, resolutionAvoidWeekend bool) (dueAt time.Time, tierDuration time.Duration) {
	dueAt = dueInstant(startedAt, duration, schedule)
	if resolutionAvoidWeekend {
		dueAt = avoidWeekend(dueAt)
	}
	if schedule == slaScheduleWSO2BusinessDays {
		return dueAt, duration
	}
	return dueAt, dueAt.Sub(startedAt)
}

func wakeMember(caseID, clockType string, tier int) string {
	return caseID + "|" + clockType + "|" + tierLabel(tier)
}

func parseWakeMember(member string) (caseID, clockType string, tier int, ok bool) {
	parts := strings.Split(member, "|")
	if len(parts) != 3 {
		return "", "", 0, false
	}
	tier, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", "", 0, false
	}
	return parts[0], parts[1], tier, true
}

// RegisterClocks computes and schedules every clock dispatch.handleCaseCreated's
// own case.created payload implies — called once, from that handler,
// alongside its existing email/Chat reactions.
//
// Duration comes from entity-service's own real, resolved sla_policy —
// fetched live, per clock type, via GetClockState — not a static,
// severity-keyed table any more (GET /sla-duration-policy and this engine's
// own in-memory durations map are both gone: entity-service's condition-
// driven SLA engine resolves and registers the case's real clocks
// synchronously, as part of the same request that creates the case). A
// clock type entity-service has no applicable policy for (e.g. LOW/S4
// "case" has no workaround/resolution policy at all in the real table
// today) comes back Found=false and is simply not registered here either —
// not an error, same "no fallback duration" posture the old static table
// had for an absent entry. A lookup failure (entity-service unreachable) is
// logged and that one clock type is skipped; the other two are still
// attempted independently.
//
// entity-service PUBLISHES case.created BEFORE it registers the clock, not
// after — a pre-existing ordering in that service's own sn_case_service.go
// (CreateCase) this package doesn't control and can't safely reorder
// (confirmed: unmodified by this engine's own SLA work). So this handler
// can genuinely race ahead of entity-service's own registration and see
// "not found" on its very first GetClockState read — see
// getClockStateWithRetry's own doc comment for the short, bounded retry
// that closes that race, rather than this method giving up on the first try.
//
// KNOWN GAP: there is no backfill. A case that already existed before this
// engine's first deployment never gets a RegisterClocks call — this only
// ever runs from case.created — so it simply never gets SLA tracking. A
// one-off script reading existing cases and calling this per case would
// close that, not built here.
func (e *Engine) RegisterClocks(ctx context.Context, caseID, priority string, createdAt time.Time, caseNumber, wso2CaseID, caseTitle, caseType, product, team string) {
	if e.entity == nil {
		slog.WarnContext(ctx, "slaengine: no entity-service client configured, sla clocks not registered", "caseId", caseID)
		return
	}

	for _, clockType := range clockTypesInOrder {
		state, err := getClockStateWithRetry(ctx, e.entity, caseID, clockType)
		if err != nil {
			slog.ErrorContext(ctx, "slaengine: failed to read clock state from entity-service, not registering this clock type", "caseId", caseID, "clockType", clockType, "err", err)
			continue
		}
		if !state.Found || state.DurationSeconds <= 0 {
			continue
		}

		startedAt := createdAt
		if state.StartedOn != nil {
			startedAt = *state.StartedOn
		}
		duration := time.Duration(state.DurationSeconds) * time.Second
		resolutionAvoidWeekend := clockType == ClockResolution && strings.EqualFold(priority, "MEDIUM")
		dueAt, tierDuration := dueAtAndTierDuration(startedAt, duration, state.Schedule, resolutionAvoidWeekend)

		meta := ClockMeta{
			CaseNumber: caseNumber,
			WSO2CaseID: wso2CaseID,
			CaseTitle:  caseTitle,
			CaseType:   caseType,
			Product:    product,
			Team:       team,
			Priority:   priority,
			StartedAt:  startedAt,
			Schedule:   state.Schedule,
		}
		if err := e.store.SetClock(ctx, caseID, clockType, meta); err != nil {
			slog.ErrorContext(ctx, "slaengine: failed to register sla clock", "caseId", caseID, "clockType", clockType, "err", err)
			continue
		}
		for _, tier := range tierSequence {
			at := tierTime(startedAt, tierDuration, tier, state.Schedule)
			if err := e.store.AddWake(ctx, wakeMember(caseID, clockType, tier), at); err != nil {
				slog.ErrorContext(ctx, "slaengine: failed to schedule sla wake entry", "caseId", caseID, "clockType", clockType, "tier", tier, "err", err)
			}
		}
		slog.InfoContext(ctx, "slaengine: registered sla clock", "caseId", caseID, "clockType", clockType, "dueAt", dueAt.Format(time.RFC3339), "schedule", state.Schedule)
	}
}

// ApplyStateEffects applies this case's new status to its workaround/
// resolution clocks — called from dispatch.handleStatusChanged alongside
// its existing email reaction, unconditionally (every case.status_changed,
// not just a genuine transition — entity-service's own publishers already
// gate on a real change, and every effect here is idempotent, so a resent
// status change just harmlessly re-applies the same effect).
//
// "Closed" is still matched directly against entity-service's own human
// display label — that is a genuine lifecycle terminus, not a guess about
// whether a particular status happens to pause a particular policy, so it
// stays a plain string check: resolution and response are genuinely done
// — force-complete both (AdvanceAlertedTier to 100, same as an early
// completion). workaround is only paused, not completed, here — a real
// "workaround provided" signal now exists (case.workaround_provided, see
// events.WorkaroundProvidedPayload's own doc comment), but it's consumed by
// its own handler (dispatch.handleWorkaroundProvided →
// CompleteWorkaroundClock), independently of a status change, not by this
// function. A case closed without ever having workaroundProvided set true
// simply never completes this clock — functionally safe regardless, since
// processDueMember drops a paused clock's due wake entry without alerting,
// same as a completed one would.
//
// For every OTHER status, whether workaround/resolution should be paused or
// resumed is no longer guessed from a hardcoded list of case states
// ("Awaiting Info"/"Solution Proposed" used to always mean pause, anything
// else always meant resume) — that list disagreed with the real,
// per-policy pause_condition/resume_condition the condition-driven SLA
// engine (entity-service's internal/slacondition) now evaluates: not every
// policy pauses on the identical set of states (a workaround policy's own
// pause_condition can differ from its resolution sibling's, and some
// targets have no pause_condition at all). entityPauseStatus asks
// entity-service's own authoritative record instead, per clock type, and
// this just applies whatever it answers.
//
// Closing is the one place besides a qualifying comment that force-completes
// the response clock (reusing CompleteResponseClock itself, so there's only
// one place that knows how). Without this, a case that closes without ever
// getting a qualifying support-engineer reply (CompleteResponseClock never
// called — reported live: a work note doesn't qualify, and neither does a
// comment whose author didn't resolve as a recognized CS-engineer role)
// left the response clock neither paused nor completed — its wake entries
// stayed live in Redis, so a breach alert fired into Chat well after the
// case had already closed.
func (e *Engine) ApplyStateEffects(ctx context.Context, caseID, newStatus string) {
	if err := e.store.SetState(ctx, caseID, ClockResponse, newStatus); err != nil {
		slog.ErrorContext(ctx, "slaengine: failed to update clock state", "caseId", caseID, "clockType", ClockResponse, "err", err)
	}
	if err := e.store.SetState(ctx, caseID, ClockWorkaround, newStatus); err != nil {
		slog.ErrorContext(ctx, "slaengine: failed to update clock state", "caseId", caseID, "clockType", ClockWorkaround, "err", err)
	}
	if err := e.store.SetState(ctx, caseID, ClockResolution, newStatus); err != nil {
		slog.ErrorContext(ctx, "slaengine: failed to update clock state", "caseId", caseID, "clockType", ClockResolution, "err", err)
	}

	if strings.EqualFold(strings.TrimSpace(newStatus), "closed") {
		e.CompleteResponseClock(ctx, caseID)
		if _, err := e.store.AdvanceAlertedTier(ctx, caseID, ClockResolution, 100, time.Time{}); err != nil {
			slog.ErrorContext(ctx, "slaengine: failed to complete resolution clock on case close", "caseId", caseID, "err", err)
		}
		e.setPaused(ctx, caseID, ClockWorkaround, true)
		return
	}

	for _, clockType := range []string{ClockWorkaround, ClockResolution} {
		paused := e.entityPauseStatus(ctx, caseID, clockType)
		e.setPaused(ctx, caseID, clockType, paused)
		if !paused {
			e.resumeShift(ctx, caseID, clockType)
		}
	}
}

// entityPauseStatus asks entity-service's own authoritative record whether
// this clock is currently paused — see ApplyStateEffects' own doc comment
// for why this replaced a hardcoded match against case-state labels. e.entity
// nil (every pre-existing test's own bare Engine{}) or a lookup failure both
// default to "not paused" — the same fail-open-toward-sending posture
// sendBreachAlert's own entity-service check already uses: wrongly leaving a
// clock paused forever would silently suppress a real breach alert, which is
// worse than the occasional premature one from wrongly resuming it.
func (e *Engine) entityPauseStatus(ctx context.Context, caseID, clockType string) bool {
	if e.entity == nil {
		return false
	}
	state, err := getClockStateWithRetry(ctx, e.entity, caseID, clockType)
	if err != nil {
		slog.WarnContext(ctx, "slaengine: failed to read pause status from entity-service, treating clock as not paused", "caseId", caseID, "clockType", clockType, "err", err)
		return false
	}
	if !state.Found {
		return false
	}
	return state.IsPaused
}

// setPaused is the plain Redis flag write only — see resumeShift for the
// wake-index rescheduling a genuine resume additionally needs. Kept this
// narrow deliberately: Engine.reconcileClock also calls this (to apply a
// durable row's own IsPaused flag), but it computes and bakes a correct,
// pause-aware wake schedule itself from scratch — routing its own call
// through resumeShift too would apply the pause offset a second time.
func (e *Engine) setPaused(ctx context.Context, caseID, clockType string, paused bool) {
	if err := e.store.SetPaused(ctx, caseID, clockType, paused); err != nil {
		slog.ErrorContext(ctx, "slaengine: failed to update clock paused state", "caseId", caseID, "clockType", clockType, "paused", paused, "err", err)
	}
}

// resumeShift is the other half of setPaused(..., false): once a clock
// actually resumes, the wake-index entries scheduled back at RegisterClocks/
// the last reconciliation have due times that take no account of the pause
// that just ended, so every tier not yet alerted needs pushing forward by
// however long this pause segment actually lasted — see this package's own
// CLAUDE.md, "SLA breach-alerting engine", for the incident this closes (a
// clock paused across one of its own tiers previously lost that tier's
// alert forever).
//
// The pause segment's own length is read from entity-service, not tracked a
// second time in Redis: entity-service's own CSM-native engine
// (SLAEngineRepository.SetPaused) already accumulates the exact same
// wall-clock pause interval into its "sla" table's pause_duration column,
// synchronously, as part of the very same case-status-changing request that
// published the case.status_changed event this function is reacting to — by
// the time this Kafka consumer sees that event, entity-service's own row
// already reflects the just-completed segment. Diffing that fresh total
// against ClockMeta.ShiftedPauseSeconds (the cumulative amount this
// clock's own wake schedule has already been shifted by, as of the last
// resume or reconciliation) gives exactly the new segment's length, so a
// clock paused and resumed several times only ever gets shifted by each
// new increment, never the whole running total again.
//
// e.entity nil (every existing test's own Engine literal) or a lookup
// failure/empty result both skip the shift entirely and log — fails safe
// (no shift) rather than guessing: an unshifted tier at worst alerts
// slightly early relative to actual working time, which Tick's own
// best-effort posture already tolerates elsewhere, where an incorrect
// shift could push a tier's due time arbitrarily far into the future.
func (e *Engine) resumeShift(ctx context.Context, caseID, clockType string) {
	if e.entity == nil {
		return
	}
	meta, found, err := e.store.GetClock(ctx, caseID, clockType)
	if err != nil {
		slog.ErrorContext(ctx, "slaengine: failed to read clock before computing resume shift", "caseId", caseID, "clockType", clockType, "err", err)
		return
	}
	if !found {
		return
	}
	state, err := e.entity.GetClockState(ctx, caseID, clockType)
	if err != nil {
		slog.WarnContext(ctx, "slaengine: failed to fetch pause duration from entity-service, wake schedule not shifted for this resume", "caseId", caseID, "clockType", clockType, "err", err)
		return
	}
	if !state.Found {
		return
	}
	if meta.Schedule == slaScheduleWSO2BusinessDays {
		e.resumeShiftBusinessDays(ctx, caseID, clockType, meta, state)
		return
	}
	delta := state.PauseDurationSeconds - meta.ShiftedPauseSeconds
	if delta <= 0 {
		return
	}
	// One atomic call: shifting the wake entries and recording the new
	// baseline must not be allowed to happen only one at a time — see
	// Store.ShiftWake's own doc comment for the double-shift this prevents.
	if err := e.store.ShiftWake(ctx, caseID, clockType, delta, state.PauseDurationSeconds); err != nil {
		slog.ErrorContext(ctx, "slaengine: failed to shift wake entries after resume", "caseId", caseID, "clockType", clockType, "deltaSeconds", delta, "err", err)
	}
}

// resumeShiftBusinessDays is resumeShift's 'WSO2 Business days' branch. A
// flat ZINCRBY shift (the 24x7 path above) is wrong for a business-day
// clock: entity-service computes a business-day clock's real due instant as
// AddBusinessDuration(start, duration + businessPause), so incrementing an
// already-computed wake score by a flat wall-clock pause delta can land it
// anywhere from days early to days late depending on how much of the pause
// segment fell on a weekend — a real gap a PR review caught (a confirmed
// example: a 116h business-day resolution clock's 100% wake landed a full
// two days before entity-service's own real due instant after one overnight
// pause). Instead, every tier not yet alerted (meta.AlertedTier) is
// recomputed from scratch — businessdays.AddBusinessDuration(startedAt,
// tierFraction(duration, tier) + businessPause) — the exact formula
// entity-service's own businessDayElapsedSnapshot inverts for breach
// detection, so this engine's wake schedule and entity-service's own
// verdict agree. SetWakeAbsolute only touches a tier whose wake entry still
// exists, so an already-alerted-and-removed tier is never resurrected.
func (e *Engine) resumeShiftBusinessDays(ctx context.Context, caseID, clockType string, meta ClockMeta, state clockState) {
	if state.DurationSeconds <= 0 {
		return
	}
	duration := time.Duration(state.DurationSeconds) * time.Second
	businessPause := time.Duration(state.BusinessPauseDurationSeconds) * time.Second

	wakes := make(map[int]time.Time, len(tierSequence))
	for _, tier := range tierSequence {
		if tier <= meta.AlertedTier {
			continue
		}
		wakes[tier] = businessdays.AddBusinessDuration(meta.StartedAt, tierFraction(duration, tier)+businessPause)
	}
	if len(wakes) == 0 {
		return
	}
	// One atomic call, same reasoning as ShiftWake's own doc comment: the
	// wake entries and the new baseline must not be allowed to land only
	// one at a time.
	if err := e.store.SetWakeAbsolute(ctx, caseID, clockType, wakes, state.BusinessPauseDurationSeconds); err != nil {
		slog.ErrorContext(ctx, "slaengine: failed to recompute business-day wake entries after resume", "caseId", caseID, "clockType", clockType, "err", err)
	}
}

// CompleteResponseClock force-completes the response clock — called from
// dispatch.handleCommentAdded when events.CommentAddedPayload.IsSupportEngineerResponse
// is true (entity-service has already confirmed the comment is a qualifying
// support-engineer reply; this engine does no role/identity resolution of
// its own). Idempotent: AdvanceAlertedTier never moves the cursor backward,
// so a redelivered comment-added event is harmless.
func (e *Engine) CompleteResponseClock(ctx context.Context, caseID string) {
	if _, err := e.store.AdvanceAlertedTier(ctx, caseID, ClockResponse, 100, time.Time{}); err != nil {
		slog.ErrorContext(ctx, "slaengine: failed to complete response clock", "caseId", caseID, "err", err)
	}
}

// CompleteWorkaroundClock force-completes the workaround clock — called
// from dispatch.handleWorkaroundProvided when entity-service publishes
// events.TypeWorkaroundProvided (a case's workaroundProvided field was set
// to true via PATCH; entity-service has no equivalent "recall" event, so
// there's no opposite operation here either — same accepted gap
// CompleteWorkaroundClock's entity-service namesake documents). Before this
// existed, ApplyStateEffects only ever paused/resumed this clock, never
// completed it — a documented gap this closes. Idempotent, same reasoning
// as CompleteResponseClock.
//
// Unlike every other trigger in this file, this one DOES return its store
// error rather than just logging it (a CodeRabbit-caught gap): this is a
// one-shot signal with no later reconciliation pass to re-derive it from —
// RegisterClocks/ApplyStateEffects/CompleteResponseClock all react to
// events whose effect either repeats (a resent status change) or is
// re-established by a later event in the same case's lifecycle, and
// Reconcile's own startup sweep can rebuild a clock's state from
// entity-service's durable row regardless. A workaround-provided signal
// lost here (e.g. a transient Redis outage) has no such second chance:
// nothing else ever calls this again for the same PATCH. Returning the
// error lets dispatch.handleWorkaroundProvided fail the record, so
// eventbus.Consumer retries it instead of silently acknowledging a clock
// that was never actually completed.
func (e *Engine) CompleteWorkaroundClock(ctx context.Context, caseID string) error {
	if _, err := e.store.AdvanceAlertedTier(ctx, caseID, ClockWorkaround, 100, time.Time{}); err != nil {
		slog.ErrorContext(ctx, "slaengine: failed to complete workaround clock", "caseId", caseID, "err", err)
		return fmt.Errorf("slaengine: complete workaround clock for %s: %w", caseID, err)
	}
	return nil
}

// Tick scans the Redis wake-index for every member due at or before now and
// processes each — a failed member doesn't stop the others; every error is
// joined and returned so RunTicker can log the whole batch's outcome in one
// line. The scan itself is retried (withRedisRetry) before giving up for
// this tick entirely — a full scan failure has no narrower fallback than
// "try again next tick" (there's no member list to fall back to a case-by-
// case entity-service check for), unlike a single member's own Redis calls
// inside processDueMember, which do have one.
func (e *Engine) Tick(ctx context.Context, now time.Time) error {
	var members []string
	err := withRedisRetry(ctx, func() error {
		var err error
		members, err = e.store.DueMembers(ctx, now)
		return err
	})
	if err != nil {
		return fmt.Errorf("slaengine: scan due members: %w", err)
	}

	var errs []error
	for _, member := range members {
		if err := e.processDueMember(ctx, member); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", member, err))
		}
	}
	return errors.Join(errs...)
}

// processDueMember handles one due wake-index member. A member is removed
// from the index once it's been actually handled — alerted, or already
// covered by AlertedTier — or once it's malformed, since a member whose due
// time has passed stays "due" forever once reached and would otherwise have
// Tick re-examine it on every future poll indefinitely.
//
// A PAUSED clock's own due member is the one case left IN the index rather
// than removed — this replaces an earlier, carried-forward design flaw
// where a tier due while paused was dropped outright, permanently losing
// that tier's alert even once the clock resumed. Leaving it in place is
// cheap (it just keeps showing up as "due" on every tick, a single
// GetClock round trip, until resumed) and is exactly what lets
// Engine.setPaused's own resume-time wake shift correctly push the
// member's score forward past "now" once the pause segment's true length
// is known — removing it here would leave nothing for that shift to act
// on.
//
// AlertedTier is checked BEFORE Paused (a CodeRabbit-caught ordering bug):
// CompleteResponseClock/CompleteWorkaroundClock/ApplyStateEffects' CLOSED
// branch all advance AlertedTier to 100 without removing the clock's own
// still-pending wake members — Reconcile's "don't resurrect an
// already-handled tier" posture relies on this same check cleaning them up
// the next time they come due. A clock force-completed and THEN paused
// (e.g. closed, then reopened and immediately put on hold) would otherwise
// return on the Paused check first, every tick, forever — a harmless but
// needless GetClock round trip on every poll until the hash's 90-day TTL
// expires or the clock resumes, since checking AlertedTier reaches the same
// RemoveWake outcome immediately and doesn't depend on pause state at all.
func (e *Engine) processDueMember(ctx context.Context, member string) error {
	caseID, clockType, tier, ok := parseWakeMember(member)
	if !ok {
		slog.ErrorContext(ctx, "slaengine: malformed wake member, dropping", "member", member)
		return e.store.RemoveWake(ctx, member)
	}

	var meta ClockMeta
	var found bool
	err := withRedisRetry(ctx, func() error {
		var err error
		meta, found, err = e.store.GetClock(ctx, caseID, clockType)
		return err
	})
	if err != nil {
		// Redis is unreachable for this member even after retrying -- see
		// processDueMemberFallback's own doc comment for why this no longer
		// attempts a degraded send.
		return e.processDueMemberFallback(ctx, caseID, clockType, member)
	}
	if !found {
		return e.store.RemoveWake(ctx, member)
	}
	if meta.AlertedTier >= tier {
		return e.store.RemoveWake(ctx, member)
	}
	if meta.Paused {
		return nil
	}

	var claimed bool
	err = withRedisRetry(ctx, func() error {
		var err error
		claimed, err = e.store.ClaimTier(ctx, caseID, clockType, tier, meta.StartedAt)
		return err
	})
	if err != nil {
		return fmt.Errorf("claim tier %d for %s/%s: %w", tier, caseID, clockType, err)
	}
	if !claimed {
		return e.store.RemoveWake(ctx, member)
	}

	if err := e.alertTier(ctx, meta, caseID, clockType, tier); err != nil {
		if releaseErr := e.store.ReleaseTier(ctx, caseID, clockType, tier, meta.StartedAt); releaseErr != nil {
			slog.ErrorContext(ctx, "slaengine: failed to release tier claim after a failed publish, tier may be stuck until it expires", "caseId", caseID, "clockType", clockType, "tier", tier, "err", releaseErr)
		}
		return fmt.Errorf("alert tier %d for %s/%s: %w", tier, caseID, clockType, err)
	}
	applied, err := e.store.AdvanceAlertedTier(ctx, caseID, clockType, tier, meta.StartedAt)
	if err != nil {
		// Logged, not returned/retried: the alert (publish + Chat) has
		// already gone out, and the tier claim above already prevents a
		// future tick from re-alerting this exact tier regardless of
		// whether this cursor update lands — see ClaimTier's own doc
		// comment. Leaving the wake member in place here would only cause
		// it to be re-examined (and immediately re-dropped by the
		// claimed=false branch above) forever, so RemoveWake below still
		// runs on this path.
		slog.ErrorContext(ctx, "slaengine: failed to advance alerted-tier cursor after a successful alert", "caseId", caseID, "clockType", clockType, "tier", tier, "err", err)
	} else if !applied {
		// The clock has moved on to a new incarnation since this tick read
		// meta (see advanceAlertedTierScript's own doc comment) — the alert
		// already sent reflects the OLD incarnation's data, which can't be
		// undone, but this tick's own work for THIS wake member ends here:
		// the wake-index member for (caseID, clockType, tier) may already
		// belong to the new incarnation's own, independently-scheduled
		// entry (AddWake overwrites the same key), so removing it now would
		// delete a still-valid future alert out from under the new clock.
		// Its cursor was never touched either, so nothing here needs
		// correcting on the new incarnation's behalf.
		slog.InfoContext(ctx, "slaengine: tier alerted under a clock incarnation that has since been replaced, leaving the new incarnation's own state untouched", "caseId", caseID, "clockType", clockType, "tier", tier)
		return nil
	}
	if err := e.store.RemoveWake(ctx, member); err != nil {
		return fmt.Errorf("remove wake entry after alerting tier %d for %s/%s: %w", tier, caseID, clockType, err)
	}
	slog.InfoContext(ctx, "slaengine: sla tier reached", "caseId", caseID, "clockType", clockType, "tier", tier)
	return nil
}

// processDueMemberFallback handles one due wake-index member when this
// engine's own Redis-held clock metadata is unreachable even after
// withRedisRetry's bounded retries.
//
// This no longer sends a degraded alert of its own. An earlier version asked
// entity-service directly for the clock's live state and sent from that --
// but recording that a degraded send happened (AdvanceAlertedTier,
// RemoveWake) needs exactly the same Redis commands that just failed and
// triggered this fallback in the first place, so a Redis outage spanning
// more than one tick could send the same degraded alert again on every tick
// it stayed down, and a recovered normal Tick could then claim the tier
// fresh and send yet again -- two real, code-review-found duplicate-alert
// shapes from one design. Comparing a live percentage against a single wake
// member's own tier (an earlier fix attempt) only closed one of the two: it
// could still silently drop a tier that was due by schedule but not yet
// reflected in entity-service's own live percentage, and the underlying
// double-send risk from an outage spanning several ticks remained.
//
// Instead: log the degraded state and leave the member in the wake index
// untouched. The next tick re-examines it through the normal,
// Redis-backed processDueMember path (ClaimTier's own idempotency), which
// sends exactly once, as soon as Redis recovers. A Redis outage therefore
// delays a breach alert rather than ever risking a duplicate or a lost
// one -- the safer failure mode for a notification.
func (e *Engine) processDueMemberFallback(ctx context.Context, caseID, clockType, member string) error {
	slog.WarnContext(ctx, "slaengine: redis unavailable for a due wake member, deferring to the next tick once it recovers", "caseId", caseID, "clockType", clockType, "member", member)
	return nil
}

// alertTier publishes events.TypeSLATierReached and sends the Google Chat
// breach card for one newly-crossed tier — only ever called after
// processDueMember has already won that tier's Redis claim, so this
// function itself needs no additional idempotency of its own. The Kafka
// publish failing is a real, retried error (processDueMember releases the
// claim and leaves the wake entry for the next tick); a Chat-send failure
// is logged only and never propagated — see sendBreachAlert's own doc
// comment for why, same explicit product decision the removed poll design
// already made for this exact split.
func (e *Engine) alertTier(ctx context.Context, meta ClockMeta, caseID, clockType string, tier int) error {
	envelope := events.Envelope{Type: events.TypeSLATierReached, EntityID: caseID}
	payload, err := json.Marshal(events.SLATierReachedPayload{CaseID: caseID, ClockType: clockType, Tier: tierLabel(tier)})
	if err != nil {
		return fmt.Errorf("encode sla.tier_reached payload: %w", err)
	}
	envelope.Payload = payload
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode sla.tier_reached envelope: %w", err)
	}
	if err := e.pub.Publish(ctx, []byte(caseID), body); err != nil {
		return fmt.Errorf("publish sla.tier_reached: %w", err)
	}

	e.sendBreachAlert(ctx, meta, caseID, clockType, tier)
	return nil
}

// sendBreachAlert builds and sends the Google Chat breach card for one tier
// crossing, once per resolved Chat audience — team/standing-audience
// routing (chataudience.Resolve), same as the design this replaces. Unlike
// that design, isEvaluationAccount/onboardingStatus are always
// false/"" here: case.created carries neither any more (see
// events.CaseCreatedPayload's own doc comment — a since-reverted feature
// briefly needed them and entity-service stopped populating them), so this
// alert routes on team alone, falling back to the fixed "Incident Monitor"
// audience — a known, accepted reduction from the removed poll design's
// own live entity-service lookup, not a bug. teamLeadName is always sent
// "" for the same reason: no event this engine consumes carries it, and
// there is no live entity-service lookup left to resolve it from.
//
// A failed send to one audience is logged (naming that Chat space) and NOT
// retried — same explicit product decision, and same reasoning, as the
// removed poll design's own sendBreachAlert: retrying here would mean
// re-claiming the tier (processDueMember's own ClaimTier is the only thing
// stopping a retry), and a single persistently broken Chat space would
// otherwise turn into an unbounded stream of duplicate alerts to every
// OTHER space that worked fine, repeating every tick, forever.
//
// Before building or sending anything, re-verifies this exact clock against
// entity-service's own durable record (e.entity.GetClockState) — a real,
// reproduced incident showed this engine's own Redis copy of a clock's
// completion can silently go stale (see entityClockStatusClient's own doc
// comment), leaving a wake entry that fires here long after the clock had,
// in truth, already finished cleanly. The alert is suppressed — logged and
// dropped with no Chat send and no Kafka publish — only when ALL of the
// following hold, each closing a gap a CodeRabbit review caught in an
// earlier, simpler ("is any row of this clock type still active") version
// of this check:
//   - the lookup itself succeeded (a network error/timeout is unconfirmed,
//     not evidence of anything — see the fail-open reasoning below);
//   - entity-service actually has a row for this clock at all (Found) —
//     entity-service's own CSM clock registration is itself best-effort and
//     runs on a different trigger than this engine's own Redis
//     registration, so "no row" can just as easily mean "never registered
//     there" as "resolved", and must not be read as the latter;
//   - that row is the SAME incarnation of the clock this wake entry is
//     about (isSameClockIncarnation, comparing StartedOn) — a severity
//     revision cancels the old clock and registers a fresh one for the same
//     (case, clockType) pair, and a stale wake entry for the OLD
//     incarnation must not be waved through just because entity-service now
//     has an unrelated, currently-active NEW one;
//   - that row's stage is genuinely terminal (isTerminalSLAStage) — a clock
//     entity-service still considers IN_PROGRESS/BREACHED/PAUSED is still
//     live, exactly the ordinary case this check must never interfere with;
//   - and it resolved WITHOUT a real breach ever being recorded
//     (!HasBreached) — a clock that did genuinely breach before it was
//     later completed is a real violation worth reporting, not a false
//     alarm, so this check only suppresses the specific "completed cleanly,
//     the alert is simply stale" case this whole feature exists for.
//
// This is deliberately checked here, inside sendBreachAlert, rather than
// earlier in processDueMember: every other bookkeeping step there (the tier
// claim, AdvanceAlertedTier, RemoveWake) still runs exactly as if the alert
// had been sent, so a clock entity-service confirms is done also gets
// marked done here, same as a real alert would, and never re-checked on a
// later tier. Any failure to positively confirm suppression — a lookup
// error, no row found, a mismatched incarnation — fails open (sends
// anyway, logged): silently losing a genuine breach alert over an
// inconclusive double-check is worse than the occasional false positive
// this whole feature exists to reduce. e.entity is nil for every caller
// that hasn't wired one up (every existing test), which also fails open
// the same way, preserving this function's exact prior behavior for them.
//
// When e.slaEmailSendingEnabled is false (SLA_EMAIL_SENDING_ENABLED=false)
// this returns immediately, before the entity-service lookup and before any
// Chat send. Only the send is skipped: processDueMember has already claimed
// the tier and published sla.tier_reached before calling here, and still
// advances the alerted-tier cursor and removes the wake entry afterwards. So
// a gated-off tier is deliberately treated as alerted, which means
// re-enabling the flag does not flood Chat with a backlog of stale breach
// cards for tiers that were crossed while it was off.
func (e *Engine) sendBreachAlert(ctx context.Context, meta ClockMeta, caseID, clockType string, tier int) {
	if !e.slaEmailSendingEnabled {
		slog.InfoContext(ctx, "slaengine: SLA_EMAIL_SENDING_ENABLED=false, skipping sla breach chat alert", "caseId", caseID, "clockType", clockType, "tier", tier)
		return
	}

	if e.entity != nil {
		state, err := e.entity.GetClockState(ctx, caseID, clockType)
		switch {
		case err != nil:
			slog.WarnContext(ctx, "slaengine: failed to verify clock state before sending breach alert, sending anyway", "caseId", caseID, "clockType", clockType, "tier", tier, "err", err)
		case !state.Found:
			slog.WarnContext(ctx, "slaengine: entity-service has no record of this clock, sending anyway", "caseId", caseID, "clockType", clockType, "tier", tier)
		case !isSameClockIncarnation(state.StartedOn, meta.StartedAt):
			slog.WarnContext(ctx, "slaengine: entity-service is tracking a different clock incarnation than this wake entry, sending anyway", "caseId", caseID, "clockType", clockType, "tier", tier)
		case isTerminalSLAStage(state.Stage) && !state.HasBreached:
			slog.InfoContext(ctx, "slaengine: clock already resolved in entity-service's own record with no breach, suppressing stale breach alert", "caseId", caseID, "clockType", clockType, "tier", tier, "stage", state.Stage)
			return
		}
	}

	caseNumber := meta.CaseNumber
	if caseNumber == "" {
		caseNumber = caseID
	}
	var openedAt string
	if !meta.StartedAt.IsZero() {
		openedAt = meta.StartedAt.UTC().Format("2006-01-02 15:04:05") + " (UTC)"
	}

	audiences := chataudience.Resolve(meta.Team, false, "", e.clock(), e.chat.HasAudienceSpace)
	caseLink := e.links.CSMLink(caseID)
	for _, audience := range audiences {
		if err := e.chat.SendSLABreachAlert(ctx, audience, clockType, tierLabel(tier), caseNumber, meta.WSO2CaseID, meta.CaseTitle, meta.CaseType, meta.Product, meta.Team, "", meta.Priority, meta.State, openedAt, caseLink); err != nil {
			slog.ErrorContext(ctx, "slaengine: failed to send sla breach alert to chat space, not retrying", "caseId", caseID, "clockType", clockType, "tier", tier, "chatSpace", audience, "err", err)
		}
	}
}

// entityActiveClocksClient abstracts EntityClient.GetActiveCSMSLAClocks for
// testability.
type entityActiveClocksClient interface {
	GetActiveCSMSLAClocks(ctx context.Context) ([]activeSLAClock, error)
}

// Reconcile rebuilds this engine's entire Redis-held state from
// entity-service's own durable record (GET /sla-status?source=csm) — the one
// thing Redis itself has no durability for (see redis.go's own package doc
// comment: every clock's state lives in Redis, nowhere else). Entity-service
// already writes the matching source='CSM' "sla" row synchronously, in the
// same request, for every lifecycle event this engine also reacts to (see
// that repo's own CLAUDE.md, "CSM-native SLA clock engine") — so if this
// engine's own Redis instance is ever flushed or replaced, this is what lets
// it resume tracking every still-open case's clocks from that record instead
// of silently losing them all with no way to recover, which is the normal,
// silent outcome without this.
//
// Intended to run once, at process startup, before RunTicker starts (see
// cmd/server/main.go) — not a recurring poll, and not gated on "only run if
// Redis looks empty": calling it against a Redis instance that already holds
// everything correctly is a safe, cheap no-op in effect, since every write
// it makes (SetClock/AddWake/AdvanceAlertedTier) is already idempotent (see
// each one's own doc comment in redis.go). As a side effect, this also
// closes RegisterClocks' own documented "no backfill" gap for any case that
// already had an entity-service "sla" row before this engine's Redis ever
// saw it — not its primary purpose, but a natural consequence of rebuilding
// from the same durable source.
//
// A fetch failure is logged and returned (so main.go can decide whether to
// still start RunTicker) rather than treated as fatal — same "nice-to-have
// engine, not core delivery" posture as GetDurationPolicy's own startup
// call. A per-clock write failure is logged and the reconciliation
// continues with the next clock — one bad row must not abandon every other
// case's recovery.
func (e *Engine) Reconcile(ctx context.Context, client entityActiveClocksClient) error {
	clocks, err := client.GetActiveCSMSLAClocks(ctx)
	if err != nil {
		return fmt.Errorf("slaengine: reconcile: fetch active clocks: %w", err)
	}

	now := time.Now()
	for _, c := range clocks {
		e.reconcileClock(ctx, c, now)
	}
	slog.InfoContext(ctx, "slaengine: reconciled sla clocks from entity-service", "count", len(clocks))
	return nil
}

// reconcileClock rebuilds one (caseID, clockType) clock's Redis state from
// its durable entity-service row — the same due-date arithmetic
// RegisterClocks uses (tierTime/avoidWeekend), just anchored on that row's
// own StartedOn rather than a fresh case.created payload's CreatedAt, since
// a case's three clock types aren't guaranteed to share one under every
// historical code path (a severity revision, for one — see entity-service's
// own CLAUDE.md, "ReviseCaseClocks").
//
// Unlike a fresh RegisterClocks call, this does NOT blindly schedule a wake
// entry for every tier: a tier whose due time has already passed is instead
// pre-marked via AdvanceAlertedTier (which never moves the cursor backward,
// so this can never un-complete a tier Redis already knows about) — without
// this, every already-overdue tier would be immediately picked up by the
// very next Tick and alerted on all at once, flooding Chat with stale
// "breach" alerts for history that's probably already been alerted on
// before whatever wiped Redis. Only a tier still in the future gets a real
// wake entry, so normal ticking resumes seamlessly from here.
func (e *Engine) reconcileClock(ctx context.Context, c activeSLAClock, now time.Time) {
	if c.DurationSeconds <= 0 {
		// entity-service itself has no resolved duration recorded for this
		// row -- same "no fallback duration" posture RegisterClocks has for
		// a clock type with no applicable policy. Not expected in practice
		// (a row GetActiveCSMSLAClocks returned was, by definition, already
		// registered against some real policy), but defended regardless.
		slog.WarnContext(ctx, "slaengine: reconcile: no duration recorded for this clock, skipping", "caseId", c.CaseID, "clockType", c.ClockType, "priority", c.Priority)
		return
	}
	duration := time.Duration(c.DurationSeconds) * time.Second
	if c.StartedOn == nil {
		slog.WarnContext(ctx, "slaengine: reconcile: no start time, skipping clock", "caseId", c.CaseID, "clockType", c.ClockType)
		return
	}
	startedAt := *c.StartedOn

	resolutionAvoidWeekend := c.ClockType == ClockResolution && strings.EqualFold(c.Priority, "MEDIUM")
	_, tierDuration := dueAtAndTierDuration(startedAt, duration, c.Schedule, resolutionAvoidWeekend)

	// pauseOffset is every bit of time this clock has spent paused, start to
	// now -- both already-completed segments (c.PauseDurationSeconds,
	// entity-service's own accumulated total) and, if the clock is still
	// paused at this exact reconciliation pass, the open segment running
	// since c.PausedOn. Folded into each tier's own due time below so a
	// freshly-rebuilt Redis doesn't schedule a wake entry as if the clock
	// had never paused at all -- the same correction resumeShift applies
	// during ordinary operation, just computed from scratch here instead of
	// as a diff against a prior baseline (there IS no prior baseline right
	// after a Redis wipe).
	pauseOffset := time.Duration(c.PauseDurationSeconds) * time.Second
	if c.IsPaused && c.PausedOn != nil {
		if open := now.Sub(*c.PausedOn); open > 0 {
			pauseOffset += open
		}
	}

	meta := ClockMeta{
		CaseNumber: c.CaseNumber,
		WSO2CaseID: c.WSO2CaseID,
		CaseTitle:  c.CaseTitle,
		CaseType:   c.CaseType,
		Product:    c.Product,
		Team:       c.Team,
		Priority:   c.Priority,
		State:      c.State,
		StartedAt:  startedAt,
		Schedule:   c.Schedule,
	}
	if err := e.store.SetClock(ctx, c.CaseID, c.ClockType, meta); err != nil {
		slog.ErrorContext(ctx, "slaengine: reconcile: failed to rebuild clock metadata", "caseId", c.CaseID, "clockType", c.ClockType, "err", err)
		return
	}

	highestPastTier := 0
	for _, tier := range tierSequence {
		at := tierTime(startedAt, tierDuration, tier, c.Schedule).Add(pauseOffset)
		if at.After(now) {
			if err := e.store.AddWake(ctx, wakeMember(c.CaseID, c.ClockType, tier), at); err != nil {
				slog.ErrorContext(ctx, "slaengine: reconcile: failed to schedule sla wake entry", "caseId", c.CaseID, "clockType", c.ClockType, "tier", tier, "err", err)
			}
			continue
		}
		highestPastTier = tier
	}
	if highestPastTier > 0 {
		// Scoped to the incarnation this call itself just wrote via SetClock
		// above: a concurrent reconciliation pass (another replica, or an
		// overlapping run) for the SAME case racing a different severity
		// could otherwise clobber whichever one writes last, the same class
		// of race AdvanceAlertedTier's own doc comment describes for Tick.
		if _, err := e.store.AdvanceAlertedTier(ctx, c.CaseID, c.ClockType, highestPastTier, startedAt); err != nil {
			slog.ErrorContext(ctx, "slaengine: reconcile: failed to pre-claim already-past tiers", "caseId", c.CaseID, "clockType", c.ClockType, "tier", highestPastTier, "err", err)
		}
	}
	// The shifted-pause baseline must equal exactly what this pass already
	// baked into the tier times above -- the whole of pauseOffset, open
	// segment included, not just c.PauseDurationSeconds (a CodeRabbit-caught
	// double-count: an earlier version recorded the completed total alone,
	// reasoning that the open segment "hasn't completed yet" so it shouldn't
	// be counted as already shifted -- but the open segment IS already baked
	// into the due times via pauseOffset above, so excluding it from the
	// baseline just means resumeShift's own diff double-applies it on the
	// real resume. Concretely: if this pass observes an open segment of
	// length o, every tier's due time is already pushed out by o. On the
	// real resume, entity-service reports completed+L (L = this segment's
	// true, now-finished length, L >= o); diffing against a baseline of only
	// "completed" gives a resume delta of L, which shifts the (already
	// o-shifted) entries by L more -- a total of o+L instead of the correct
	// L, making every tier L-o too late. Diffing against completed+o instead
	// gives a resume delta of L-o -- exactly the portion of the segment this
	// reconciliation pass hadn't yet observed -- for a correct total shift of
	// o + (L-o) = L.
	baselineSeconds := c.PauseDurationSeconds
	if c.IsPaused && c.PausedOn != nil {
		if open := now.Sub(*c.PausedOn); open > 0 {
			baselineSeconds += int64(open / time.Second)
		}
	}
	if err := e.store.SetShiftedPauseSeconds(ctx, c.CaseID, c.ClockType, baselineSeconds); err != nil {
		slog.ErrorContext(ctx, "slaengine: reconcile: failed to record the shifted-pause baseline", "caseId", c.CaseID, "clockType", c.ClockType, "err", err)
	}
	// Always applied, not just when true: the durable row is the source of
	// truth for this flag during reconciliation, and Redis may currently
	// hold the opposite value (e.g. paused while Redis was down, then
	// resumed before this reconciliation pass runs) -- only ever setting
	// true here would leave a stale paused=true in Redis forever, which
	// Tick's own "drop if paused" check (processDueMember) would then
	// silently swallow every future tier crossing for, never alerting.
	e.setPaused(ctx, c.CaseID, c.ClockType, c.IsPaused)
}

// RunTicker calls Tick every interval until ctx is done. Run from its own
// goroutine (see cmd/server/main.go); a failed Tick is logged, not fatal —
// the next tick gets another chance at whatever was due.
func (e *Engine) RunTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.Tick(ctx, time.Now()); err != nil {
				slog.ErrorContext(ctx, "slaengine: tick failed", "err", err)
			}
		}
	}
}
