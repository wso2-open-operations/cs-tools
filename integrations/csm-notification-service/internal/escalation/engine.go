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

package escalation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// callPlacer abstracts notifications.TwilioClient's two call methods.
type callPlacer interface {
	MakeSSMLCall(ctx context.Context, to string, speech notifications.Speech) (notifications.Call, error)
	MakeCall(ctx context.Context, to, message string) (notifications.Call, error)
}

// ladderStore abstracts Store for testability.
type ladderStore interface {
	Create(ctx context.Context, incidentID string, st LadderState) (bool, error)
	Save(ctx context.Context, incidentID string, st LadderState) error
	Get(ctx context.Context, incidentID string) (LadderState, bool, error)
	Delete(ctx context.Context, incidentID string) error
	AddWake(ctx context.Context, member string, at time.Time) error
	RemoveWakes(ctx context.Context, members ...string) error
	DueMembers(ctx context.Context, now time.Time) ([]string, error)
	// MarkCalled records who was reached, for the evening pairing's
	// round-robin. Best-effort at every call site.
	MarkCalled(ctx context.Context, email string, at time.Time) error
}

// incidentNotes abstracts the entity-service client that writes the execution
// summary back onto the incident (section 11.0).
type incidentNotes interface {
	AppendWorkNote(ctx context.Context, incidentID, note string) error
}

// EngineConfig holds the engine's operational switches.
type EngineConfig struct {
	// CallSendingEnabled is the killswitch, mirroring
	// dispatch.Dispatcher.callSendingEnabled (CALL_SENDING_ENABLED). When
	// false the ladder still runs, still records, and still writes its work
	// note — it just logs each call instead of dialling, so a deployment can
	// watch a real ladder end to end without paging anyone.
	CallSendingEnabled bool
	// UseSSML picks Trigger.VoiceSpeech over Trigger.VoiceMessagePlain.
	UseSSML bool
	// Channel selects how a rung reaches people: the specification's phone
	// call, a card in the incident's Google Chat space, or both. Defaults to
	// calls — see ParseChannel for why silently downgrading a pager would be
	// the wrong default.
	Channel Channel
	// Ladder is this ladder's section of the configuration file: which
	// incidents get a ladder at all, and what one ladder may spend. The zero
	// value has no opinion on either, which is what a deployment with no
	// configuration file gets.
	Ladder LadderConfig
}

// Engine runs the incident call-escalation ladder.
//
// Two halves, the same split internal/slaengine uses: Handle is the consumer
// side (its own consumer group — see cmd/server/main.go), reacting to the three
// signals that start and stop a ladder; Tick is the scheduler side, placing
// whatever calls have come due. Neither knows the timing rules, which live
// entirely in policy.go and plan.go.
type Engine struct {
	policies map[string]PriorityPolicy
	resolver Resolver
	// notifiers is how a rung reaches people, one per selected channel. The
	// ladder's rules know nothing about which are in play.
	notifiers []notifier
	// missingChannels names every channel cfg.Channel selected that has no
	// client to serve it. Recorded rather than rejected: a deployment asking
	// for both channels with only one client configured should still page
	// people over the one it has, and refusing to construct would take the
	// working half down with the broken one. But it must not be silent --
	// a ladder that posts no chat card while an operator believes it does is
	// precisely the failure this exists to surface.
	missingChannels []Channel
	store           ladderStore
	notes           incidentNotes
	cfg             EngineConfig
	// clock is time.Now unless a test substitutes one; the staleness check
	// in start is the only thing that reads it, and it has to be testable
	// against a trigger that is genuinely old.
	clock func() time.Time
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock()
	}
	return time.Now()
}

// NewEngine constructs an Engine. notes may be nil, in which case the
// execution summary is logged rather than written back to the incident (see
// writeNote) — a deployment without entity-service access still runs a real
// ladder.
//
// The nil check is deliberate and must stay: assigning a nil *EntityClient
// straight into the incidentNotes interface field would store a non-nil
// interface holding a nil pointer, so writeNote's `e.notes == nil` would be
// false and it would call AppendWorkNote on a nil receiver.
func NewEngine(policies map[string]PriorityPolicy, resolver Resolver, calls *notifications.TwilioClient, chat *notifications.GoogleChatClient, links PortalLinks, store *Store, notes *EntityClient, defaultChatProduct string, cfg EngineConfig) *Engine {
	e := &Engine{policies: policies, resolver: resolver, store: store, cfg: cfg}
	if notes != nil {
		e.notes = notes
	}
	// ChannelLog needs no client and so can never be half-configured — there
	// is no missingChannels case for it. It is also exclusive: Uses() reports
	// false for both real channels here, so a ladder set to log wires this
	// notifier and nothing else, which is the point. A log ladder that also
	// dialled would be the worst of both.
	if cfg.Channel == ChannelLog {
		e.notifiers = append(e.notifiers, logNotifier{})
	}
	if cfg.Channel.Uses(ChannelCall) {
		if calls == nil {
			e.missingChannels = append(e.missingChannels, ChannelCall)
		} else {
			e.notifiers = append(e.notifiers, voiceNotifier{calls: calls, useSSML: cfg.UseSSML})
		}
	}
	if cfg.Channel.Uses(ChannelChat) {
		if chat == nil {
			e.missingChannels = append(e.missingChannels, ChannelChat)
		} else {
			// links goes in only when it is really there, for the same reason
			// links is a value type on purpose. It used to be a
			// *recipientlinks.Resolver, and assigning a nil one straight into
			// the incidentLinker field stored a non-nil interface holding a
			// nil pointer, so the nil check inside Deliver passed and
			// IncidentLink was called on a nil receiver — a panic in the
			// middle of paging somebody, made twice in this one constructor
			// before a real run caught it. A PortalLinks with no base URL
			// simply returns an empty link, which the card renders as no link,
			// so there is no nil to get wrong any more.
			n := chatNotifier{chat: chat, defaultProduct: defaultChatProduct, links: links}
			e.notifiers = append(e.notifiers, n)
		}
	}
	// Logged here rather than per attempt: this is a deployment mistake, it
	// cannot change while the process runs, and one line at startup is
	// actionable where one line per rung is noise. place() still reports the
	// case where nothing at all is configured, because that one reaches
	// nobody.
	for _, ch := range e.missingChannels {
		slog.Error("escalation: configured for a channel with no client; nothing will be sent over it",
			"missingChannel", string(ch), "configuredChannel", string(cfg.Channel))
	}
	return e
}

// Handle implements eventbus.Handle for this engine's own consumer group.
//
// It shares a topic with every other event in this service, so anything that
// is not one of its three signals is a silent no-op rather than an error —
// same reasoning as slaengine.Engine.Handle and dispatch.Handle's own no-op
// cases. Erroring would burn this consumer's retries and dead-letter a record
// that was never broken.
func (e *Engine) Handle(ctx context.Context, record eventbus.Record) error {
	var env events.Envelope
	if err := json.Unmarshal(record.Value, &env); err != nil {
		return fmt.Errorf("escalation: decode envelope: %w", err)
	}
	switch env.Type {
	case events.TypeIncidentCreated, events.TypeIncidentPriorityElevated,
		events.TypeIncidentAcknowledged, events.TypeIncidentCommentAdded:
	default:
		return nil
	}
	if err := events.Validate(env.EntityID, env.Type, env.Payload); err != nil {
		return fmt.Errorf("escalation: invalid payload: %w", err)
	}

	switch env.Type {
	case events.TypeIncidentCreated:
		var p events.IncidentCreatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("escalation: decode incident.created payload: %w", err)
		}
		return e.start(ctx, triggerFromCreated(env.EntityID, p), false)
	case events.TypeIncidentPriorityElevated:
		var p events.IncidentPriorityElevatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("escalation: decode incident.priority_elevated payload: %w", err)
		}
		return e.start(ctx, triggerFromElevated(env.EntityID, p), true)
	case events.TypeIncidentCommentAdded:
		var p events.IncidentCommentAddedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("escalation: decode incident.comment_added payload: %w", err)
		}
		if !p.IsPublic {
			// A work note is an internal jotting, not section 3.0's
			// acknowledgement gesture. Somebody writing one while triaging
			// must not silence their own pager.
			return nil
		}
		return e.cancelBy(ctx, env.EntityID, cancelPublicComment)
	default:
		return e.cancelBy(ctx, env.EntityID, cancelStateChange)
	}
}

// cancelReason records which of section 3.0's two acknowledgement gestures
// stopped a ladder, so the execution summary says which one it was.
type cancelReason string

const (
	// cancelStateChange is the gesture for a newly reported incident: moving
	// it out of NEW, which is what section 10.0's voice message instructs.
	cancelStateChange cancelReason = "Acknowledged"
	// cancelPublicComment is the gesture for a priority elevation, and the
	// only stop signal such a ladder has.
	cancelPublicComment cancelReason = "Public comment added"
	// cancelAcknowledged is both gestures together, which is what
	// acknowledgement actually means: the incident moved out of NEW and
	// somebody said so where the customer can see it.
	cancelAcknowledged cancelReason = "Acknowledged (status and public comment)"
)

// start expands a trigger into a ladder and schedules it.
//
// replace distinguishes the two triggers. A redelivered incident.created must
// never restart a ladder that is already part-way through — Kafka is
// at-least-once, and restarting would re-dial from LEVEL_0 — so it claims the
// incident with a create-if-absent write and does nothing if one is already
// running. A priority elevation is the opposite: it is a fresh trigger with
// its own, faster timings (section 7.0 keys everything on priority), so it
// deliberately replaces whatever is running, retiring the old ladder's
// outstanding calls first.
func (e *Engine) start(ctx context.Context, t Trigger, replace bool) error {
	policy, ok := Lookup(e.policies, t.Priority)
	if !ok {
		// Not an error: section 7.0 has no row below P4, so a
		// planning-priority incident legitimately has no ladder. Erroring
		// would dead-letter a valid event.
		slog.InfoContext(ctx, "escalation: no ladder for this priority; skipping",
			"incidentId", t.IncidentID, "priority", t.Priority)
		return nil
	}

	// The configuration file's own gate, checked before anything is planned or
	// stored. Skipping is not an error: a deployment that has narrowed which
	// incidents it escalates has said so deliberately, and erroring would
	// dead-letter events it simply does not want to act on.
	if ok, why := e.cfg.Ladder.Allows(t.Priority, t.Routing.AssignedCRETeam, string(t.Routing.Shift)); !ok {
		slog.InfoContext(ctx, "escalation: configuration does not escalate this incident; skipping",
			"incidentId", t.IncidentID, "priority", t.Priority,
			"team", t.Routing.AssignedCRETeam, "shift", string(t.Routing.Shift),
			"reason", why)
		return nil
	}

	// USA_WEEKEND is the one shift whose LEVEL_0 depends on ABT eligibility
	// (R10 has none, R12/R14 do — see RoutingContext.HasNotificationLevel).
	// No publisher populates ABTEligible today: entity-service has no
	// product-to-ABT mapping, so the field arrives false and this shift always
	// takes the R12 branch. A bool cannot distinguish "not eligible" from
	// "nobody told us", so the mis-branch is announced rather than hidden.
	if !t.Routing.abtKnown() {
		// Not a detail: eligibility selects which half of section 5.0's table
		// an incident routes by, so without it the rule is unnamed and the
		// recipients are whatever the roster's fallback tier happens to hold.
		// On USA_WEEKEND it also decides whether LEVEL_0 exists at all.
		slog.WarnContext(ctx, "escalation: no ABT eligibility on this incident; "+
			"the section 5.0 rule cannot be named and routing falls back",
			"incidentId", t.IncidentID, "product", t.Routing.Product,
			"shift", string(t.Routing.Shift),
			"level0Included", t.Routing.HasNotificationLevel())
	}

	plan, err := BuildPlan(ctx, t, e.policies, e.resolver, e.cfg.Channel)
	if err != nil {
		return fmt.Errorf("escalation: build plan for %s: %w", t.IncidentID, err)
	}
	// BuildPlan takes the trigger by value and stamps the matched rule onto
	// its own copy, so the local one here still reports the old fourteen-row
	// derivation -- an LK incident logged rule=R1, a row that shift cannot
	// reach. Adopt the stamped copy so every line below names the row the
	// recipients actually came from.
	t = plan.Trigger
	e.applySafety(ctx, &plan)
	for _, issue := range plan.Issues {
		// Logged without the recipient's email — plan issues carry one in
		// Detail for NO_NUMBER, and this repo does not log recipient
		// addresses (see internal/entity's do() doc comment). The work note
		// is where the per-person detail belongs.
		slog.WarnContext(ctx, "escalation: level cannot be called",
			"incidentId", t.IncidentID, "rule", t.Routing.Rule(),
			"level", issue.Level.String(), "reason", issue.Reason)
	}
	if len(plan.Calls) == 0 {
		slog.WarnContext(ctx, "escalation: plan has no reachable recipients; nothing scheduled",
			"incidentId", t.IncidentID, "priority", t.Priority, "rule", t.Routing.Rule(),
			"shift", string(t.Routing.Shift), "product", t.Routing.Product, "team", t.Routing.AssignedCRETeam)
		e.writeNote(ctx, plan, nil, nil, nil, "")
		return nil
	}

	// A ladder whose every call is already in the past has nothing left to
	// do, and scheduling it anyway would burst-dial the whole thing on the
	// next tick. That is not hypothetical: this engine's consumer group reads
	// the topic from its beginning the first time it exists (eventbus sets
	// StartOffset to FirstOffset), so the first deployment replays every
	// trigger still in retention — and a redelivery, a DLQ retry or a long
	// consumer outage can all hand it an old trigger later. Every call is an
	// offset from the trigger time on purpose, so a *short* backlog still
	// catches up correctly (the due calls go out on the next tick, at the
	// rung the ladder should be on by now); it is the ladder that has run
	// its whole course before we heard about it that must be dropped.
	if last := plan.Calls[len(plan.Calls)-1]; last.At.Before(e.now()) {
		slog.WarnContext(ctx, "escalation: trigger is older than its whole ladder; not scheduling",
			"incidentId", t.IncidentID, "priority", t.Priority, "trigger", string(t.Kind),
			"triggeredAt", t.At.Format(time.RFC3339), "lastCallAt", last.At.Format(time.RFC3339))
		return nil
	}

	st := LadderState{Plan: plan, Placed: make([]bool, len(plan.Calls))}
	if replace {
		if err := e.retireRunning(ctx, t.IncidentID); err != nil {
			return err
		}
		if err := e.store.Save(ctx, t.IncidentID, st); err != nil {
			return fmt.Errorf("escalation: replace ladder for %s: %w", t.IncidentID, err)
		}
	} else {
		created, err := e.store.Create(ctx, t.IncidentID, st)
		if err != nil {
			return fmt.Errorf("escalation: claim ladder for %s: %w", t.IncidentID, err)
		}
		if !created {
			slog.InfoContext(ctx, "escalation: ladder already running; ignoring duplicate trigger",
				"incidentId", t.IncidentID)
			return nil
		}
	}

	// Seed every call's wake entry. A failure part-way leaves the ladder
	// partially scheduled; the record is retried, and because the state write
	// above already happened, the retry re-seeds the same members — ZADD is
	// idempotent for an unchanged score, so re-seeding is harmless.
	for i, c := range plan.Calls {
		if err := e.store.AddWake(ctx, wakeMember(t.IncidentID, i), c.At); err != nil {
			return fmt.Errorf("escalation: schedule call %d for %s: %w", i, t.IncidentID, err)
		}
	}
	// One line that answers "which ladder, and by which path" without anyone
	// re-deriving section 5.0's table from four fields by hand. levels is the
	// rungs this incident will actually climb, in order, which is the part
	// that differs between rules — a USA_WEEKEND ABT incident has no LEVEL_0
	// (R10) while its IAM counterpart does (R12).
	slog.InfoContext(ctx, "escalation: ladder scheduled",
		"incidentId", t.IncidentID, "priority", t.Priority, "trigger", string(t.Kind),
		"rule", t.Routing.Rule(), "shift", string(t.Routing.Shift),
		"product", t.Routing.Product, "team", t.Routing.AssignedCRETeam,
		"abtEligible", t.Routing.ABTEligibility(),
		"levels", plan.LevelsClimbed(), "calls", len(plan.Calls),
		"finalLevelAt", t.At.Add(TimeToFinalLevel(policy, t.Routing.HasNotificationLevel())).Format(time.RFC3339))
	return nil
}

// retireRunning drops any outstanding wake entries for an incident, used when
// an elevation replaces a running ladder. The old ladder's state is not
// deleted here — Save overwrites it immediately after.
func (e *Engine) retireRunning(ctx context.Context, incidentID string) error {
	st, found, err := e.store.Get(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: load running ladder for %s: %w", incidentID, err)
	}
	if !found {
		return nil
	}
	if err := e.store.RemoveWakes(ctx, pendingMembers(incidentID, st)...); err != nil {
		return fmt.Errorf("escalation: retire running ladder for %s: %w", incidentID, err)
	}
	return nil
}

// cancelBy stops a running ladder because the incident was acknowledged, by
// whichever of section 3.0's two gestures reason names.
//
// Both gestures stop ANY running ladder, not only the one their own trigger
// started. Section 3.0 pairs a gesture with each trigger — status change for a
// new incident, public comment for an elevation — but treating them as
// mutually exclusive would mean a responder who commented on a newly reported
// incident, rather than moving it to Work In Progress, keeps being called
// while visibly working on it. Both are unambiguous evidence the incident is
// being attended, which is what the ladder exists to provoke, so either one
// ends it. This is deliberately slightly broader than the document's literal
// pairing.
//
// Order matters. The remaining wake entries are dropped first — stopping the
// calls is the whole point, and it is idempotent — then the work note is
// written, then the state is deleted. A failed work note therefore retries
// with the state still present (and Cancelled already set, so the drop is not
// repeated), rather than losing the summary.
func (e *Engine) cancelBy(ctx context.Context, incidentID string, reason cancelReason) error {
	st, found, err := e.store.Get(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: load ladder for %s: %w", incidentID, err)
	}
	if !found {
		// The common case: an incident acknowledged without a ladder ever
		// having run, or one already finished.
		return nil
	}

	// Acknowledgement is BOTH gestures, not either: the incident moved out of
	// NEW and a public comment written. Moving the status alone is what a
	// dispatcher does while triaging a queue, and it would otherwise silence
	// the pager for an incident nobody had actually picked up -- which is the
	// case this rule exists for. The card and the voice message have always
	// asked for both; only the engine disagreed.
	//
	// The two arrive as separate events in either order, so each is recorded
	// and the ladder keeps climbing until both are in.
	if st.Cancelled == nil && e.cfg.Ladder.RequireBothGestures() {
		switch reason {
		case cancelStateChange:
			st.SawStateChange = true
		case cancelPublicComment:
			st.SawPublicComment = true
		}
		if !(st.SawStateChange && st.SawPublicComment) {
			missing := "a public comment"
			if !st.SawStateChange {
				missing = "a move out of NEW"
			}
			if err := e.store.Save(ctx, incidentID, st); err != nil {
				return fmt.Errorf("escalation: record acknowledgement for %s: %w", incidentID, err)
			}
			slog.InfoContext(ctx, "escalation: half acknowledged; the ladder keeps climbing",
				"incidentId", incidentID, "saw", string(reason), "stillNeeds", missing,
				"reachedLevel", st.ReachedLevel())
			return nil
		}
		reason = cancelAcknowledged
	}

	if st.Cancelled == nil {
		now := time.Now()
		st.Cancelled = &now
		st.CancelReason = string(reason)
		if err := e.store.Save(ctx, incidentID, st); err != nil {
			return fmt.Errorf("escalation: mark ladder cancelled for %s: %w", incidentID, err)
		}
		pending := pendingMembers(incidentID, st)
		if err := e.store.RemoveWakes(ctx, pending...); err != nil {
			return fmt.Errorf("escalation: stop ladder for %s: %w", incidentID, err)
		}
		slog.InfoContext(ctx, "escalation: ladder cancelled",
			"incidentId", incidentID, "reason", string(reason),
			"rule", st.Plan.Trigger.Routing.Rule(), "priority", st.Plan.Trigger.Priority,
			"reachedLevel", st.ReachedLevel(), "placedCalls", st.PlacedCount(),
			"cancelledCalls", len(pending))
	}

	e.writeNote(ctx, st.Plan, st.Placed, st.Failed, st.Cancelled, st.CancelReason)
	return e.store.Delete(ctx, incidentID)
}

// Tick places every call that has come due. Mirrors slaengine.Engine.Tick: one
// scan per tick, each due entry handled independently so one failure does not
// block the rest.
func (e *Engine) Tick(ctx context.Context, now time.Time) error {
	members, err := e.store.DueMembers(ctx, now)
	if err != nil {
		return fmt.Errorf("escalation: scan due calls: %w", err)
	}
	var errs []error
	for _, member := range members {
		if err := e.processDue(ctx, member); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// processDue handles one due call.
//
// The call is placed, then recorded, then its wake entry dropped — in that
// order, so a crash or a Redis failure after dialling leaves the entry in
// place and the call is repeated next tick. That is the deliberate direction
// to fail in: this is a paging system, and a duplicate call to the same
// on-call engineer costs far less than a page that never happens. It is the
// same trade-off slaengine.processDueMember documents for its Chat send.
func (e *Engine) processDue(ctx context.Context, member string) error {
	incidentID, index, ok := parseWakeMember(member)
	if !ok {
		slog.ErrorContext(ctx, "escalation: malformed wake member, dropping", "member", member)
		return e.store.RemoveWakes(ctx, member)
	}

	st, found, err := e.store.Get(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: load ladder for %s: %w", incidentID, err)
	}
	if !found || index >= len(st.Placed) {
		// The ladder was cancelled, completed or replaced and this entry is a
		// leftover. Dropping it is the correct outcome.
		return e.store.RemoveWakes(ctx, member)
	}
	if st.Cancelled != nil || st.Placed[index] || st.failure(index) != "" {
		return e.store.RemoveWakes(ctx, member)
	}

	call := st.Plan.Calls[index]
	var failure string
	if err := e.place(ctx, st.Plan, call); err != nil {
		if !isPermanent(err) {
			// Transient — leave the wake entry, the next tick retries.
			return fmt.Errorf("escalation: place %s call for %s: %w", call.Level, incidentID, err)
		}
		// The provider rejected the request itself; trying again with the
		// same number and the same document cannot succeed. Record why, so
		// the work note says this person was not reached and the ladder can
		// still complete, then treat it like a placed call for scheduling.
		slog.ErrorContext(ctx, "escalation: call rejected by the provider; not retrying",
			"incidentId", incidentID, "rule", st.Plan.Trigger.Routing.Rule(),
			"level", call.Level.String(), "attempt", call.Ordinal,
			"to", maskPhone(call.Recipient.Phone), "reason", permanentReason(err))
		failure = permanentReason(err)
	}

	// Re-read before writing, and carry over only what this goroutine owns.
	//
	// e.place above is an HTTP call that can take seconds, and the consumer
	// goroutine is live throughout: an acknowledgement can arrive and write
	// SawStateChange, or Cancelled, or delete the ladder outright. Saving the
	// copy read before the call would put all of that back -- losing a half
	// acknowledgement, so the ladder demands a gesture that already arrived,
	// or resurrecting a ladder somebody has already stopped and calling the
	// next rung.
	//
	// This narrows the window to the gap between this read and the save
	// rather than closing it: there is no compare-and-set here, and a proper
	// fix is a Lua script that does both in one round trip. The remaining gap
	// is microseconds against seconds, and it fails in the recoverable
	// direction -- a placed call is not recorded, so it is placed again.
	fresh, stillRunning, err := e.store.Get(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: reload ladder for %s: %w", incidentID, err)
	}
	if !stillRunning || fresh.Cancelled != nil || index >= len(fresh.Placed) {
		// Acknowledged, replaced or completed while the call was in flight.
		// The call did go out, but there is nothing left to record it on.
		return e.store.RemoveWakes(ctx, member)
	}
	if failure != "" {
		fresh.setFailure(index, failure)
	} else {
		fresh.Placed[index] = true
	}
	st = fresh

	if err := e.store.Save(ctx, incidentID, st); err != nil {
		return fmt.Errorf("escalation: record placed call for %s: %w", incidentID, err)
	}
	if err := e.store.RemoveWakes(ctx, member); err != nil {
		return fmt.Errorf("escalation: clear placed call for %s: %w", incidentID, err)
	}

	if st.AllSettled() {
		// The ladder ran to its end without anyone acknowledging. Record what
		// happened and stop tracking it.
		e.writeNote(ctx, st.Plan, st.Placed, st.Failed, nil, "")
		slog.WarnContext(ctx, "escalation: ladder exhausted without acknowledgement",
			"incidentId", incidentID, "priority", st.Plan.Trigger.Priority,
			"rule", st.Plan.Trigger.Routing.Rule(), "reachedLevel", st.ReachedLevel(),
			"placedCalls", st.PlacedCount())
		return e.store.Delete(ctx, incidentID)
	}
	return nil
}

// place notifies one recipient over every configured channel, or logs what it
// would have done when sending is disabled.
//
// Two log lines, not one, and both matter when something goes wrong at 3am.
// The first is written BEFORE the attempt, so one that hangs or crashes the
// process still leaves a record that this rung was about to page someone. The
// second is written after a channel accepts it, and carries the provider's own
// handle where there is one — a Twilio call sid is what an operator searches
// the console by to find out whether it actually rang.
//
// Every channel is attempted even if an earlier one fails, and the errors are
// joined: a chat webhook being down must not stop the phone ringing, and a
// phone failing must not cost the room its sight of the escalation.
func (e *Engine) place(ctx context.Context, plan Plan, call PlannedCall) error {
	t := plan.Trigger
	if !e.cfg.CallSendingEnabled {
		slog.InfoContext(ctx, "escalation: sending disabled (CALL_SENDING_ENABLED=false); not notifying",
			"incidentId", t.IncidentID, "rule", t.Routing.Rule(), "priority", t.Priority,
			"level", call.Level.String(), "attempt", call.Ordinal,
			"channel", string(e.cfg.Channel), "to", maskPhone(call.Recipient.Phone))
		return nil
	}
	if len(e.notifiers) == 0 {
		// Configured for a channel whose client was never constructed. Not an
		// error to retry — no tick will fix it — but never silent either.
		slog.ErrorContext(ctx, "escalation: no notifier configured for this channel; nobody was contacted",
			"incidentId", t.IncidentID, "channel", string(e.cfg.Channel),
			"level", call.Level.String(), "attempt", call.Ordinal)
		return nil
	}

	slog.InfoContext(ctx, "escalation: notifying",
		"incidentId", t.IncidentID, "rule", t.Routing.Rule(), "priority", t.Priority,
		"level", call.Level.String(), "attempt", call.Ordinal,
		"channel", string(e.cfg.Channel), "to", maskPhone(call.Recipient.Phone),
		"message", messageKind(e.cfg.UseSSML))

	var errs []error
	for _, n := range e.notifiers {
		delivered, err := n.Deliver(ctx, plan, call)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", n.Channel(), err))
			continue
		}
		slog.InfoContext(ctx, "escalation: ALERT TRIGGERED",
			"incidentId", t.IncidentID, "rule", t.Routing.Rule(), "priority", t.Priority,
			"level", call.Level.String(), "attempt", call.Ordinal,
			"recipient", call.Recipient.Name, "to", maskPhone(call.Recipient.Phone),
			"channel", string(delivered.Channel), "ref", delivered.Ref,
			"status", delivered.Status)
	}

	// Record the call for the evening pairing's fairness rule, whatever the
	// channel was -- somebody reached by a card has been reached. Best-effort
	// on purpose: an unrecorded call makes the next pairing slightly less
	// fair, which is not worth failing a page over, and the error is a log
	// line rather than an entry in errs so it can never cause a retry that
	// dials somebody a second time.
	if e.store != nil && call.Recipient.Email != "" {
		if err := e.store.MarkCalled(ctx, call.Recipient.Email, time.Now()); err != nil {
			slog.WarnContext(ctx, "escalation: could not record who was called; the next pairing may repeat somebody",
				"incidentId", t.IncidentID, "err", err)
		}
	}
	return errors.Join(errs...)
}

// messageKind names which voice document a call carried, so a log line says
// whether the SSML path or the plain one was exercised.
func messageKind(ssml bool) string {
	if ssml {
		return "ssml"
	}
	return "plain"
}

// writeNote PATCHes the execution summary onto the incident (section 11.0).
// With no entity-service client configured the summary is logged instead, so a
// deployment without one still runs the ladder rather than failing every
// record.
func (e *Engine) writeNote(ctx context.Context, plan Plan, placed []bool, failed []string, cancelledAt *time.Time, reason string) {
	note := plan.WorkNote(placed, failed, cancelledAt, reason)
	if e.notes == nil {
		slog.InfoContext(ctx, "escalation: no incident-notes client configured; execution summary not written back",
			"incidentId", plan.Trigger.IncidentID)
		return
	}
	if err := e.notes.AppendWorkNote(ctx, plan.Trigger.IncidentID, note); err != nil {
		// Logged, never returned. This used to propagate, and a record whose
		// summary could not be written was retried three times and then
		// dead-lettered -- an incident that had genuinely been handled,
		// abandoned because the note about it would not save. PATCH
		// /incidents is 503 on a Postgres deployment, so that was every
		// record, including ones with nobody to call, which is how it was
		// found.
		//
		// The summary is a record of work already done: the calls are placed,
		// the cards are posted, the state is settled. Losing the record is a
		// loss; discarding the event is worse, and retrying cannot help,
		// because whatever refused the write will refuse it again.
		slog.ErrorContext(ctx, "escalation: could not write the execution summary; it is lost for this incident",
			"incidentId", plan.Trigger.IncidentID, "error", err)
	}
}

// RunTicker calls Tick every interval until ctx is done, from its own
// goroutine (see cmd/server/main.go). A failed tick is logged, not fatal — the
// next tick retries whatever was due.
func (e *Engine) RunTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.Tick(ctx, time.Now()); err != nil {
				slog.ErrorContext(ctx, "escalation: tick failed", "err", err)
			}
		}
	}
}

// pendingMembers lists the wake members for calls that have not been placed.
func pendingMembers(incidentID string, st LadderState) []string {
	var out []string
	for i, done := range st.Placed {
		if !done && st.failure(i) == "" {
			out = append(out, wakeMember(incidentID, i))
		}
	}
	return out
}

// isPermanent reports whether a call error is one that retrying cannot fix:
// the provider accepted the request and rejected its content. Anything else —
// a network failure, a 5xx, a timeout — is transient and stays scheduled.
func isPermanent(err error) bool {
	var upstream *apierror.Error
	if errors.As(err, &upstream) {
		return upstream.StatusCode >= 400 && upstream.StatusCode < 500
	}
	return false
}

// twilioCodePattern finds the provider's own error code in a rejection body.
//
// Scanned for rather than JSON-decoded on purpose: the body is truncated to a
// fixed budget before it reaches here (see the call client's maxErrBody), so a
// long message — and Twilio's are long, they name the offending number and
// link its documentation — leaves the JSON unparsable and took the code down
// with it. A real rejection reported only as "REJECTED_400" sent this
// investigation to the wrong error entirely: 21219 (destination unverified)
// and 21210 (source number not on the account) are the same status and
// completely different fixes.
var twilioCodePattern = regexp.MustCompile(`"code"\s*:\s*(\d+)`)

// permanentReason renders a rejection for the log line and the work note:
// the provider's own status and, where it gives one, its error code — never
// the message, which echoes the phone number back into places this service
// keeps numbers out of.
func permanentReason(err error) string {
	var upstream *apierror.Error
	if !errors.As(err, &upstream) {
		return "REJECTED"
	}
	if m := twilioCodePattern.FindStringSubmatch(upstream.Body); m != nil {
		return fmt.Sprintf("REJECTED_%d_%s", upstream.StatusCode, m[1])
	}
	return fmt.Sprintf("REJECTED_%d", upstream.StatusCode)
}

// maskPhone keeps only the last four digits, matching dispatch.maskPhone's own
// shape (unexported there, and dispatch importing this package would be the
// wrong direction).
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "****"
	}
	return "********" + phone[len(phone)-4:]
}

// triggerFromCreated builds a new-incident trigger from its event payload.
//
// The effective shift is derived from the report time rather than carried in
// the payload: section 5.0 takes it from ServiceNow's on-call schedule, which
// entity-service cannot read either, and deriving it here keeps one definition
// of the boundaries (see ShiftAt).
func triggerFromCreated(incidentID string, p events.IncidentCreatedPayload) Trigger {
	// Resolved once, not once per use: when the payload carries no timestamp
	// both uses fall back to time.Now(), and two separate calls would read
	// two different instants — which across a shift boundary would schedule
	// the ladder against one shift and route it by another.
	at := reportedAt(p.ReportedAt)
	return Trigger{
		IncidentID: incidentID,
		Number:     p.Number,
		WSO2CaseID: p.WSO2CaseID,
		Priority:   p.Priority,
		Title:      p.Title,
		Account:    p.Account,
		Team:       p.Team,
		Kind:       TriggerNewIncident,
		At:         at,
		Routing: RoutingContext{
			Product:         p.Product,
			ABTEligible:     p.ABTEligible,
			AssignedCRETeam: p.Team,
			Shift:           ShiftAt(at),
			At:              at,
		},
	}
}

// triggerFromElevated builds a priority-elevation trigger from its event
// payload.
func triggerFromElevated(incidentID string, p events.IncidentPriorityElevatedPayload) Trigger {
	at := reportedAt(p.ElevatedAt)
	return Trigger{
		IncidentID: incidentID,
		Number:     p.Number,
		WSO2CaseID: p.WSO2CaseID,
		Priority:   p.NewPriority,
		Title:      p.Title,
		Account:    p.Account,
		Team:       p.Team,
		Kind:       TriggerPriorityElevated,
		At:         at,
		Routing: RoutingContext{
			Product:         p.Product,
			ABTEligible:     p.ABTEligible,
			AssignedCRETeam: p.Team,
			Shift:           ShiftAt(at),
			At:              at,
		},
	}
}

// reportedAt parses the trigger time a payload carries, falling back to now.
//
// The fallback matters for the ladder's honesty: every call is an offset from
// the trigger, so consuming a backlogged record must not shift the whole
// ladder later than section 7.0 intends. When the publisher tells us when the
// incident was actually reported, that is what the offsets are measured from.
func reportedAt(raw string) time.Time {
	if raw == "" {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Now()
	}
	return t
}

// applySafety trims a freshly built plan to what the configuration file says
// one ladder may spend. It runs before anything is stored, so a capped ladder
// is capped for its whole life rather than only until the next restart.
//
// Each cap records a PlanIssue rather than silently shrinking the plan. A
// ladder that reaches fewer people than the rules say it should is a fact
// somebody needs on the work note, not a quiet saving.
func (e *Engine) applySafety(ctx context.Context, plan *Plan) {
	s := e.cfg.Ladder.Safety

	if top, capped := e.cfg.Ladder.CapLevel(); capped {
		kept := plan.Calls[:0]
		var dropped int
		for _, c := range plan.Calls {
			if c.Level <= top {
				kept = append(kept, c)
				continue
			}
			dropped++
		}
		if dropped > 0 {
			plan.Calls = kept
			plan.Issues = append(plan.Issues, PlanIssue{
				Level:  top,
				Reason: "LEVEL_CAPPED",
			})
			slog.WarnContext(ctx, "escalation: ladder capped by configuration; the rungs above it will never be reached",
				"incidentId", plan.Trigger.IncidentID, "maxLevel", top.String(), "callsDropped", dropped)
		}
	}

	// Numbers are filtered rather than the calls being left to fail: an
	// unlisted recipient is not an error, it is a deployment saying "not this
	// person". The number itself is never logged.
	if len(s.AllowedNumbers) > 0 {
		kept := plan.Calls[:0]
		var dropped int
		for _, c := range plan.Calls {
			if e.cfg.Ladder.Dialable(c.Recipient.Phone) {
				kept = append(kept, c)
				continue
			}
			dropped++
		}
		if dropped > 0 {
			plan.Calls = kept
			plan.Issues = append(plan.Issues, PlanIssue{Reason: "NUMBER_NOT_ALLOWED"})
			slog.WarnContext(ctx, "escalation: recipients dropped by safety.allowedNumbers",
				"incidentId", plan.Trigger.IncidentID, "callsDropped", dropped)
		}
	}

	// The call cap is deliberately last and deliberately truncating rather
	// than refusing: by this point the earlier rungs are the ones worth
	// keeping, and a ladder that reaches its first responders is better than
	// one that reaches nobody because its top rung resolved to forty people.
	if s.MaxCallsPerLadder > 0 && len(plan.Calls) > s.MaxCallsPerLadder {
		dropped := len(plan.Calls) - s.MaxCallsPerLadder
		plan.Calls = plan.Calls[:s.MaxCallsPerLadder]
		plan.Issues = append(plan.Issues, PlanIssue{Reason: "CALL_CAP_REACHED"})
		slog.WarnContext(ctx, "escalation: plan truncated by safety.maxCallsPerLadder; "+
			"the rota may have grown, or the cap may be too low",
			"incidentId", plan.Trigger.IncidentID,
			"cap", s.MaxCallsPerLadder, "callsDropped", dropped)
	}
}
