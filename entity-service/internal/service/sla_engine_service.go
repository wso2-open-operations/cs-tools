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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

// SLAEngineService is this service's own in-process SLA clock engine: it
// registers, completes, pauses, resumes and cancels source='CSM' "sla"
// rows (migration 0134's own original enum intent -- see
// repository.SLAEngineRepository's own doc comment for why this engine's
// own clocks must never carry the 'SERVICENOW' tag, which belongs to a
// real, independent, continuously-running external sync job) in reaction
// to case-lifecycle events, sourcing real durations AND
// real applicability from the actual, previous-system-synced, source='SERVICENOW'
// sla_policy rows (decoded by migrations 0250-0258) -- evaluated via
// internal/slacondition against the case's own current field values
// (slaPolicyResolver, repository.CaseFacts), not a hardcoded severity map
// and not a name-keyed lookup against CSM-only rows (that mechanism, and the
// 16 rows backing it, are gone -- migration 0259; see git history before
// that migration, and sla_policy_resolver.go's own doc comment, for what
// this replaces).
//
// Every method is called directly, in-process, from snCaseService's own
// case-lifecycle hooks (sn_case_service.go) -- never over HTTP, and never
// gated on Event Hub/publisher being configured, same reasoning the old
// design gave for being independent of s.publisher: a deployment with no
// Event Hub configured must not lose SLA tracking as a side effect.
//
// Every method is best-effort: a failure here is logged and never returned
// to the caller as an error, because none of this may ever fail case
// creation, comment creation, or a state-changing PATCH -- the case/comment
// mutation itself has already succeeded (in the previous system) by the time any of
// these run, and treating an SLA-tracking hiccup as a failed case mutation
// would be strictly worse than tracking nothing.
type SLAEngineService interface {
	// RegisterCaseClocks resolves and registers a new source='CSM'
	// "sla" row for every clock target (RESPONSE/WORKAROUND/RESOLUTION) whose
	// current real sla_policy start_condition matches the case -- called
	// once, from CreateCase, right after a case is created. projectID is
	// accepted for call-site compatibility but not used directly: it's
	// folded into the case's own CaseFacts (project type) by a fresh
	// database read, same as every other fact. severity may be nil (a
	// real, if rare, production state -- see domain.CaseView.Severity's
	// own doc comment on confirmed null rates); when given, it overrides
	// whatever a fresh read of the case's own severity would show (see
	// loadCaseFacts' own doc comment for why an override is needed at
	// all) -- several real policies (ServiceRequest SLA, the Query/
	// SecReport Analysis family) have no severity condition at all, so a
	// nil severity no longer means "register nothing" the way it used to.
	// A target can resolve to more than one applicable policy at once (see
	// slaPolicyResolver.resolve's own doc comment) -- every one of them gets
	// its own, independent clock, not just the first. A policy whose
	// sla_policy.retroactive is set starts its clock backdated to the
	// case's own creation instant (facts.CreatedOn) instead of the instant
	// this method happens to run at -- real policies set this on every
	// P1-P3 response definition, where the source system's own semantics
	// are "this clock has always been running since the case was opened,"
	// not "this clock starts now."
	RegisterCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, projectID string)
	// ReviseCaseClocks cancels every existing active clock for the case and
	// registers an entirely fresh set matching its NEW severity, atomically
	// (see repository.SLAEngineRepository.ReviseClocks) -- called from
	// UpdateCase when an EXISTING case's severity changes (see
	// RegisterCaseClocks's own doc comment: registration only ever runs
	// once, from CreateCase, so a case re-severitized after creation would
	// otherwise keep running its already-registered clocks against the
	// original severity's resolved policies forever).
	//
	// Per explicit product direction, the new clocks have NO relation to the
	// old ones: no revised-in-place policy/duration, no carried-over
	// start_on or elapsed time. The old clocks run into a terminal CANCELLED
	// state (per-target, see repository.SLAEngineRepository.ReviseClocks'
	// own doc comment) and the new ones start from zero, exactly as if the
	// case had just been created at the new severity/facts.
	ReviseCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, projectID string)
	// CompleteResponseClock marks the case's CSM-authored "response" clock
	// ACHIEVED -- called from CreateCaseComment when the new comment
	// qualifies as the case's first substantive support-engineer reply (see
	// sn_case_service.go's applyResponseSLAOnComment-equivalent hook for the
	// exact qualification check, ported from the old design). This is also
	// the one existing signal repository.CaseFacts.CSEngineerResponseComment
	// is derived from -- see that field's own doc comment.
	CompleteResponseClock(ctx context.Context, caseID string)
	// CompleteWorkaroundClock marks the case's CSM-authored "workaround"
	// clock ACHIEVED -- called when a case's WorkaroundProvided is set to
	// true (the "Provide Workaround" action), the one genuine "workaround
	// was provided" signal that exists anywhere in the domain model.
	CompleteWorkaroundClock(ctx context.Context, caseID string)
	// CompleteFixEtaSharedClocks marks the case's CSM-authored "workaround"
	// AND "resolution" clocks ACHIEVED -- called when a PATCH shares a fix
	// ETA with the customer (req.AddPublicComment true alongside a fix-ETA
	// date, the webapp's "Share fix ETA with customer" action; ServiceNow
	// data source only -- see case_service.go's own UpdateCase rejection
	// list, AddPublicComment has no Postgres equivalent). Once WSO2 has
	// committed a fix timeline to the customer, neither clock has anything
	// further to track.
	CompleteFixEtaSharedClocks(ctx context.Context, caseID string)
	// ApplyCaseStateEffects evaluates the case's CURRENTLY ACTIVE clocks
	// (RESPONSE/WORKAROUND/RESOLUTION) against their OWN bound sla_policy's
	// stop_condition/cancel_condition/pause_condition/resume_condition --
	// the real, decoded conditions (migrations 0250-0258), not a hardcoded
	// switch on state. Replaces the old design's applyCaseStateSLAEffects
	// port (a fixed per-state pause/resume/complete-on-CLOSED switch): most
	// real policies' own stop_condition already includes "stateINCLOSED" (or
	// similar), so closing a case naturally completes its clocks through
	// this same generic evaluation, with no special-cased CLOSED branch
	// needed here any more.
	//
	// For each clock target with a currently open (non-final-stage) clock:
	//  1. stop_condition true -> CompleteClock (a genuine completion, e.g.
	//     a reply went out, or the case closed).
	//  2. else cancel_condition true -> CancelClock (the policy that was
	//     backing this clock no longer applies at all, e.g. the case's type
	//     changed away from what it required).
	//  3. else pause_condition true -> SetPaused(true).
	//  4. else resume_condition true -> SetPaused(false).
	//  5. else: left exactly as it is. A policy with no pause_condition at
	//     all (every real RESPONSE policy) is simply never paused by this
	//     method, matching the old design's "RESPONSE is never paused"
	//     behavior for free, as a consequence of the real data rather than
	//     a special case written here.
	// A clock already in a genuine completion/cancellation stage is not
	// matched by ActiveClockPolicy's own open-stage filter at all, so
	// calling this unconditionally on every state-changing PATCH is safe.
	ApplyCaseStateEffects(ctx context.Context, caseID string, state domain.CaseState)
}

type slaEngineService struct {
	resolver *slaPolicyResolver
	repo     repository.SLAEngineRepository
}

// NewSLAEngineService constructs an SLAEngineService backed by the given
// repository.
func NewSLAEngineService(repo repository.SLAEngineRepository) SLAEngineService {
	return &slaEngineService{resolver: newSLAPolicyResolver(repo), repo: repo}
}

// loadCaseFacts reads caseID's current CaseFacts fresh from Postgres and
// applies the two optional overrides callers may already know more
// authoritatively than a fresh read would: severityOverride (an in-flight
// severity a dual-write case's own Postgres mirror may not have
// caught up to yet) and stateOverride (ApplyCaseStateEffects is always
// handed the case's brand new state directly from the PATCH that triggered
// it, for the exact same reason). Every other fact -- case type, project
// type, product, workaround/fix-ETA/account presence, creation date, and
// whether a CS engineer has already replied -- comes from the fresh read:
// none of those are ever passed in by a caller today.
func (s *slaEngineService) loadCaseFacts(ctx context.Context, caseID string, severityOverride *domain.CaseSeverity, stateOverride *domain.CaseState) (repository.CaseFacts, error) {
	facts, err := s.repo.GetCaseFacts(ctx, caseID)
	if err != nil {
		return repository.CaseFacts{}, err
	}
	if severityOverride != nil {
		if label, ok := slaSeverityLabel[*severityOverride]; ok {
			facts.Severity = label
		}
	}
	if stateOverride != nil {
		if label, ok := slaStateLabel[*stateOverride]; ok {
			facts.State = label
		}
	}
	return facts, nil
}

// resolveApplicablePolicies resolves, for every clock target (slaTargetOrder:
// RESPONSE, WORKAROUND, RESOLUTION), the one real sla_policy whose
// start_condition currently matches facts -- shared by RegisterCaseClocks
// and ReviseCaseClocks so both derive the same "what should this case's
// clocks look like" answer the same way.
//
// The second return, lookupFailed, distinguishes two very different reasons
// a clock target can be missing from the returned slice: slaPolicyResolver.
// resolve's own doc comment explains why a genuinely-unmatched target (no
// error) and a failed lookup (an error, e.g. a database blip) both drop
// that target from the slice the same way, but callers must NOT treat them
// the same. RegisterCaseClocks (case creation) safely ignores lookupFailed
// -- nothing existing is at risk, and a missing clock target there is
// retried the next time this case is touched. ReviseCaseClocks must check
// it: proceeding to ReviseClocks with a policy list that's incomplete
// because of a lookup failure (not because no policy genuinely applies)
// would cancel the case's existing clocks and commit no replacement for the
// target that failed to resolve.
func (s *slaEngineService) resolveApplicablePolicies(ctx context.Context, facts repository.CaseFacts) (policies []repository.SLAPolicyRef, lookupFailed bool) {
	f := caseConditionFacts{c: facts}
	policies = make([]repository.SLAPolicyRef, 0, len(slaTargetOrder))
	for _, clockType := range slaTargetOrder {
		target := slaClockTypeTarget[clockType]
		matches, err := s.resolver.resolve(ctx, target, facts)
		if err != nil {
			// resolve already logged why -- this target's policy couldn't
			// be determined right now, not that none applies.
			lookupFailed = true
			continue
		}
		// matches may hold more than one policy for this one target -- see
		// slaPolicyResolver.resolve's own doc comment (a Hosting case's
		// RESOLUTION target legitimately matches two policies at once).
		// Every match is appended; RegisterClock's own per-policy
		// uniqueness guard is what makes registering all of them safe.
		for _, m := range matches {
			// resolve() only ever evaluates start_condition -- a policy
			// whose stop_condition ALSO already holds against these same
			// facts must never be registered at all: upstream only attaches
			// a definition when start holds AND stop doesn't. Without this,
			// a case that already satisfies a policy's own completion
			// condition (e.g. a fix ETA already shared) gets a fresh clock
			// registered on every call that re-resolves applicable policies
			// (case creation, and ApplyCaseStateEffects' own re-registration
			// pass), which the very next state-effects pass then completes
			// at ~0% elapsed -- a junk ACHIEVED row with no real tracking
			// value, skewing compliance numbers. Checked once here so both
			// callers (RegisterCaseClocks and ApplyCaseStateEffects) get it
			// for free.
			if m.StopCondition != "" && slacondition.Parse(slaPolicyConditionText(m, m.StopCondition)).Evaluate(f) {
				continue
			}
			policies = append(policies, m)
		}
	}
	return policies, lookupFailed
}

// registerCaseSLAClockStartOn chooses the start_on instant RegisterClock
// should use for policy -- caseCreatedOn (backdated to the case's own
// creation instant) when policy.Retroactive is set, a zero time.Time
// (RegisterClock's own "start now" default) otherwise. See
// SLAEngineService.RegisterCaseClocks' own doc comment for which real
// policies this affects and why.
//
// Known, accepted simplification: a retroactive clock is ALWAYS backdated
// to case CREATION here, regardless of which call site registers it. That
// is correct for the P1/managed-services policies this engine's Retroactive
// definitions mean to cover when registered at case creation (RegisterCaseClocks) --
// both anchors (creation vs. "the update that first met the start
// condition") coincide at that moment. It is only an approximation for a
// P2/P3-style policy that becomes newly applicable LATER (e.g. a severity
// change into P2 via ApplyCaseStateEffects' own re-registration pass, or
// ReviseClocks' replacement path) -- upstream anchors those to the
// triggering update, not creation, but sla_policy carries no column
// distinguishing the two anchor semantics, so this always uses creation,
// which can backdate a late P2/P3 registration earlier than the real
// qualifying event. Not fixed here: it would need a new sla_policy column
// recording which anchor a given definition actually uses.
func registerCaseSLAClockStartOn(policy repository.SLAPolicyRef, caseCreatedOn time.Time) time.Time {
	if policy.Retroactive {
		return caseCreatedOn
	}
	return time.Time{}
}

// RegisterCaseClocks implements SLAEngineService.
func (s *slaEngineService) RegisterCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, _ string) {
	facts, err := s.loadCaseFacts(ctx, caseID, severity, nil)
	if err != nil {
		slog.ErrorContext(ctx, "sla engine: register clocks: load case facts failed", "caseId", caseID, "err", err)
		return
	}
	// lookupFailed deliberately ignored here -- see resolveApplicablePolicies'
	// own doc comment: nothing existing is at risk at case-creation time, a
	// missing clock target just means one target doesn't get registered
	// this time.
	policies, _ := s.resolveApplicablePolicies(ctx, facts)
	if len(policies) == 0 {
		slog.InfoContext(ctx, "sla engine: not registering clocks, no applicable policy matched", "caseId", caseID)
		return
	}
	for _, policy := range policies {
		startOn := registerCaseSLAClockStartOn(policy, facts.CreatedOn)
		registered, err := s.repo.RegisterClock(ctx, caseID, policy, startOn)
		if err != nil {
			slog.ErrorContext(ctx, "sla engine: register clock failed", "caseId", caseID, "clockType", policy.Target, "policy", policy.Name, "err", err)
			continue
		}
		if !registered {
			slog.InfoContext(ctx, "sla engine: clock already registered, skipped", "caseId", caseID, "clockType", policy.Target, "policy", policy.Name)
		}
	}
}

// ReviseCaseClocks implements SLAEngineService.
func (s *slaEngineService) ReviseCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, _ string) {
	facts, err := s.loadCaseFacts(ctx, caseID, severity, nil)
	if err != nil {
		slog.ErrorContext(ctx, "sla engine: revise clocks: load case facts failed", "caseId", caseID, "err", err)
		return
	}
	policies, lookupFailed := s.resolveApplicablePolicies(ctx, facts)
	if lookupFailed {
		// Do NOT proceed to ReviseClocks with an incomplete policy list --
		// see resolveApplicablePolicies' own doc comment. Leaving the case's
		// existing clocks completely untouched (stale, but intact) is safer
		// than cancelling them and committing no replacement for the target
		// whose policy lookup failed.
		slog.ErrorContext(ctx, "sla engine: revise clocks skipped, a policy lookup failed", "caseId", caseID)
		return
	}
	// Bounded retry (2 attempts, short pause between): ReviseClocks is one
	// atomic transaction (see its own doc comment), so retrying it is safe
	// -- a second attempt after a failed first one just repeats the same
	// cancel-then-register-fresh transaction, it never duplicates rows.
	var cancelled int
	for attempt := 1; attempt <= 2; attempt++ {
		cancelled, err = s.repo.ReviseClocks(ctx, caseID, policies)
		if err == nil || attempt == 2 {
			break
		}
		slog.WarnContext(ctx, "sla engine: revise clocks failed, retrying once", "caseId", caseID, "err", err)
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		// Atomic: a failure here means NOTHING changed -- the case's prior
		// clocks are exactly as they were.
		slog.ErrorContext(ctx, "sla engine: revise clocks for severity change failed after retry", "caseId", caseID, "err", err)
		return
	}
	if cancelled > 0 {
		slog.InfoContext(ctx, "sla engine: cancelled active clocks for severity change", "caseId", caseID, "count", cancelled)
	}
}

// CompleteResponseClock implements SLAEngineService.
//
// Does not separately stamp an "already responded" fact any more --
// repository.CaseFacts.CSEngineerResponseComment is now derived live from
// the comment table on every GetCaseFacts read, not from a durable column
// this method used to write. See that field's own doc comment.
func (s *slaEngineService) CompleteResponseClock(ctx context.Context, caseID string) {
	if _, err := s.repo.CompleteClock(ctx, caseID, slaClockTypeTarget[slaClockTypeResponse]); err != nil {
		slog.ErrorContext(ctx, "sla engine: complete response clock failed", "caseId", caseID, "err", err)
	}
}

// CompleteWorkaroundClock implements SLAEngineService.
func (s *slaEngineService) CompleteWorkaroundClock(ctx context.Context, caseID string) {
	if _, err := s.repo.CompleteClock(ctx, caseID, slaClockTypeTarget[slaClockTypeWorkaround]); err != nil {
		slog.ErrorContext(ctx, "sla engine: complete workaround clock failed", "caseId", caseID, "err", err)
	}
}

// CompleteFixEtaSharedClocks implements SLAEngineService.
func (s *slaEngineService) CompleteFixEtaSharedClocks(ctx context.Context, caseID string) {
	if _, err := s.repo.CompleteClock(ctx, caseID, slaClockTypeTarget[slaClockTypeWorkaround]); err != nil {
		slog.ErrorContext(ctx, "sla engine: complete workaround clock on fix eta shared failed", "caseId", caseID, "err", err)
	}
	if _, err := s.repo.CompleteClock(ctx, caseID, slaClockTypeTarget[slaClockTypeResolution]); err != nil {
		slog.ErrorContext(ctx, "sla engine: complete resolution clock on fix eta shared failed", "caseId", caseID, "err", err)
	}
}

// ApplyCaseStateEffects implements SLAEngineService.
//
// A case transitioning to CLOSED always force-completes every clock still
// open on it, regardless of what that clock's own policy's stop_condition
// says -- a safety net on TOP of (never instead of) the condition-based
// completion below, not a replacement for it. Before the condition-driven
// engine, this method unconditionally completed every clock on a CLOSED
// transition; that blanket behavior is gone now that completion is
// per-policy-condition, and while every real case policy's stop_condition
// this codebase has been able to confirm does reach a CLOSED-matching clause,
// a future synced policy whose stop_condition simply doesn't name CLOSED
// would otherwise leave its clock running forever on an already-closed case
// with nobody able to notice -- a stuck clock is a worse failure mode than a
// redundant CompleteClock call, so this net applies unconditionally rather
// than only after confirming every policy's text by hand. CompleteClock/
// CancelClock are themselves idempotent against a clock that isn't open
// (their own slaEngineOpenStageFilter scope), so calling CompleteClock here
// for a clock this same pass already completed or cancelled is a harmless
// no-op -- still skipped via `settled` purely to avoid a pointless second
// query.
//
// Every currently open clock of every target is evaluated independently,
// against its OWN bound policy's own conditions (ActiveClockPolicies can
// return more than one policy per target -- see that method's own doc
// comment on why: a Hosting case can have an open-source-track and a
// managed-services-track RESOLUTION clock running side by side, and a
// severity change to one must never be decided by the other's conditions).
//
// Cancellation and resumption are driven by each policy's own declared
// semantics, not a single hardcoded rule for every policy:
//   - WhenToCancel == "NEVER": this clock is never cancelled by this method
//     at all (only CompleteClock -- a genuine stop_condition match, or the
//     CLOSED safety net -- can end it).
//   - WhenToCancel == "START_CONDITIONS_ARE_NOT_MET" (used by every policy
//     whose own cancel_condition is empty): cancel once this policy's own
//     StartCondition no longer matches -- i.e. the fact that qualified this
//     case for the policy in the first place has changed (a case_type/
//     project_type change, most commonly).
//   - WhenToCancel == "CANCEL_CONDITIONS_ARE_MET" (the default): cancel when
//     CancelCondition itself evaluates true -- the original, condition-only
//     behavior.
//   - WhenToResume == "PAUSE_CONDITIONS_ARE_NOT_MET": a clock already PAUSED
//     resumes once this policy's own PauseCondition stops matching, instead
//     of waiting on a separate ResumeCondition (several real policies carry
//     no resume_condition at all under this mode -- there is nothing else
//     that could ever resume them).
//   - WhenToResume == "RESUME_CONDITIONS_ARE_MET" (the default): resume when
//     ResumeCondition itself evaluates true -- the original behavior.
//
// After every existing open clock has been evaluated, this method also
// re-resolves every policy applicable to the case's CURRENT facts and
// attempts to register each one, exactly as RegisterCaseClocks does at case
// creation -- so a start_condition that only becomes true later (e.g. a
// policy gated on a state the case wasn't in at creation) is picked up on
// the very next state-changing update, not left to a severity change that
// may never come. RegisterClock's own per-(work item, policy) uniqueness
// guard makes this a no-op for every policy that already has a clock,
// registered or cancelled-and-superseded, so this adds no duplicate
// registrations -- only genuinely new ones.
func (s *slaEngineService) ApplyCaseStateEffects(ctx context.Context, caseID string, state domain.CaseState) {
	facts, err := s.loadCaseFacts(ctx, caseID, nil, &state)
	if err != nil {
		slog.ErrorContext(ctx, "sla engine: apply case state effects: load case facts failed", "caseId", caseID, "state", state, "err", err)
		return
	}
	f := caseConditionFacts{c: facts}

	for _, clockType := range slaTargetOrder {
		target := slaClockTypeTarget[clockType]
		policies, err := s.repo.ActiveClockPolicies(ctx, caseID, target)
		if err != nil {
			slog.ErrorContext(ctx, "sla engine: apply case state effects: active clock policies lookup failed", "caseId", caseID, "clockType", target, "err", err)
			continue
		}
		for _, policy := range policies {
			s.applyCaseStateEffectsForPolicy(ctx, caseID, target, policy, f, state)
		}
	}

	// Re-evaluate start conditions against the case's current facts, same as
	// case creation -- see this method's own doc comment. A policy whose
	// cancel condition is ALSO true for these same facts is skipped here:
	// without this check, a policy that both starts and cancels under the
	// current facts would be cancelled by the loop above and immediately
	// re-registered by this one, resetting on every subsequent update and
	// never reaching a real stop.
	policies, _ := s.resolveApplicablePolicies(ctx, facts)
	for _, policy := range policies {
		if slaPolicyShouldCancel(policy, f) {
			continue
		}
		startOn := registerCaseSLAClockStartOn(policy, facts.CreatedOn)
		if _, err := s.repo.RegisterClock(ctx, caseID, policy, startOn); err != nil {
			slog.ErrorContext(ctx, "sla engine: apply case state effects: register newly-applicable clock failed", "caseId", caseID, "clockType", policy.Target, "policy", policy.Name, "err", err)
		}
	}
}

// applyCaseStateEffectsForPolicy evaluates one open clock's own bound
// policy conditions against f -- the per-policy body of
// SLAEngineService.ApplyCaseStateEffects, split out so that method's own
// loop can call it once per policy returned by ActiveClockPolicies.
func (s *slaEngineService) applyCaseStateEffectsForPolicy(ctx context.Context, caseID, target string, policy repository.SLAPolicyRef, f caseConditionFacts, state domain.CaseState) {
	settled := false
	switch {
	case policy.StopCondition != "" && slacondition.Parse(slaPolicyConditionText(policy, policy.StopCondition)).Evaluate(f):
		if _, err := s.repo.CompleteClockByPolicy(ctx, caseID, target, policy.ID); err != nil {
			slog.ErrorContext(ctx, "sla engine: apply case state effects: complete clock failed", "caseId", caseID, "clockType", target, "policy", policy.Name, "err", err)
		}
		settled = true
	case slaPolicyShouldCancel(policy, f):
		if _, err := s.repo.CancelClockByPolicy(ctx, caseID, target, policy.ID); err != nil {
			slog.ErrorContext(ctx, "sla engine: apply case state effects: cancel clock failed", "caseId", caseID, "clockType", target, "policy", policy.Name, "err", err)
		}
		settled = true
	case policy.PauseCondition != "" && slacondition.Parse(slaPolicyConditionText(policy, policy.PauseCondition)).Evaluate(f):
		if _, err := s.repo.SetPausedByPolicy(ctx, caseID, target, policy.ID, true); err != nil {
			slog.ErrorContext(ctx, "sla engine: apply case state effects: pause clock failed", "caseId", caseID, "clockType", target, "policy", policy.Name, "err", err)
		}
	case slaPolicyShouldResume(policy, f):
		if _, err := s.repo.SetPausedByPolicy(ctx, caseID, target, policy.ID, false); err != nil {
			slog.ErrorContext(ctx, "sla engine: apply case state effects: resume clock failed", "caseId", caseID, "clockType", target, "policy", policy.Name, "err", err)
		}
	default:
		// Neither pause nor resume condition is configured/matches: left
		// exactly as it is -- see this type's own ApplyCaseStateEffects doc
		// comment.
	}

	if !settled && state == domain.CaseStateClosed {
		if _, err := s.repo.CompleteClockByPolicy(ctx, caseID, target, policy.ID); err != nil {
			slog.ErrorContext(ctx, "sla engine: apply case state effects: safety-net complete clock on CLOSED failed", "caseId", caseID, "clockType", target, "policy", policy.Name, "err", err)
		}
	}
}

// slaPolicyShouldCancel decides whether policy's open clock should be
// cancelled right now, per policy.WhenToCancel -- see
// SLAEngineService.ApplyCaseStateEffects' own doc comment for what each
// value means.
func slaPolicyShouldCancel(policy repository.SLAPolicyRef, f caseConditionFacts) bool {
	switch policy.WhenToCancel {
	case "NEVER":
		return false
	case "START_CONDITIONS_ARE_NOT_MET":
		// No StartCondition at all is never treated as "always cancel" --
		// an empty condition means this package's own Condition.Evaluate
		// already returns false for it (see slacondition's own doc
		// comment), which would otherwise cancel every such clock on its
		// very next evaluation.
		if policy.StartCondition == "" {
			return false
		}
		return !slacondition.Parse(slaPolicyConditionText(policy, policy.StartCondition)).Evaluate(f)
	default: // "CANCEL_CONDITIONS_ARE_MET", or unset/unrecognized
		return policy.CancelCondition != "" && slacondition.Parse(slaPolicyConditionText(policy, policy.CancelCondition)).Evaluate(f)
	}
}

// slaPolicyShouldResume decides whether policy's open (and, by the time this
// is reached in applyCaseStateEffectsForPolicy's switch, still-paused)
// clock should resume right now, per policy.WhenToResume -- see
// SLAEngineService.ApplyCaseStateEffects' own doc comment for what each
// value means. Evaluated regardless of whether the clock is ACTUALLY paused
// right now (SetPausedByPolicy's own UPDATE only ever matches an open row
// and sets exactly the requested paused value either way, so calling it on
// an already-running clock is a harmless, idempotent no-op) -- same posture
// the original ResumeCondition-only check always had.
func slaPolicyShouldResume(policy repository.SLAPolicyRef, f caseConditionFacts) bool {
	switch policy.WhenToResume {
	case "PAUSE_CONDITIONS_ARE_NOT_MET":
		if policy.PauseCondition == "" {
			return false
		}
		return !slacondition.Parse(slaPolicyConditionText(policy, policy.PauseCondition)).Evaluate(f)
	default: // "RESUME_CONDITIONS_ARE_MET", or unset/unrecognized
		return policy.ResumeCondition != "" && slacondition.Parse(slaPolicyConditionText(policy, policy.ResumeCondition)).Evaluate(f)
	}
}
