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
)

// SLAEngineService is the CSM-native SLA clock engine: it registers,
// completes, pauses and resumes source='CSM' "sla" rows (migration 0134)
// in reaction to case-lifecycle events, sourcing real durations from the
// deterministic, severity-keyed sla_policy rows migration 0203 seeds (via
// slaPolicyResolver) -- not the real, ServiceNow-synced sla_policy rows
// (those are looked up by name/pattern-guessing nothing here does anymore;
// see slaPolicyResolver's own doc comment for why), and not the old,
// deleted sla_clocks design's hardcoded severity->duration map either (see
// git history for internal/service/sla_policy.go before commit
// 116d43522). Every method here is called directly, in-process, from
// snCaseService's own case-lifecycle hooks (sn_case_service.go) -- never
// over HTTP, and never gated on Event Hub/publisher being configured, same
// reasoning the old design's applyResponseSLAOnComment/
// applyCaseStateSLAEffects gave for being independent of s.publisher: a
// deployment with no Event Hub configured must not lose SLA tracking as a
// side effect.
//
// Every method is best-effort: a failure here is logged and never returned
// to the caller as an error, because none of this may ever fail case
// creation, comment creation, or a state-changing PATCH -- the case/comment
// mutation itself has already succeeded (in ServiceNow) by the time any of
// these run, and treating an SLA-tracking hiccup as a failed case mutation
// would be strictly worse than tracking nothing.
type SLAEngineService interface {
	// RegisterCaseClocks resolves and registers a new source='CSM' "sla"
	// row for every clock type severity applies to (see
	// slaApplicableClockTypes) -- called once, from CreateCase, right after
	// a case is created. projectID is accepted for call-site compatibility
	// but no longer used: policy resolution is keyed on severity alone (see
	// slaPolicyResolver's own doc comment). severity may be nil (a real, if
	// rare, production state
	// -- see domain.CaseView.Severity's own doc comment on confirmed null
	// rates), in which case this logs and returns without registering
	// anything, same as the old design's "severity not in map" handling.
	RegisterCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, projectID string)
	// ReviseCaseClocks cancels every existing active clock for the case and
	// registers an entirely fresh set for its NEW severity, atomically (see
	// repository.SLAEngineRepository.ReviseClocks) -- called from UpdateCase
	// when an EXISTING case's severity changes (see RegisterCaseClocks's own
	// doc comment: registration only ever runs once, from CreateCase, so a
	// case re-severitized after creation would otherwise keep running its
	// already-registered clocks against the original severity's durations
	// forever).
	//
	// Per explicit product direction, the new clocks have NO relation to the
	// old ones: no revised-in-place policy/duration, no carried-over
	// start_on or elapsed time. The old severity's clocks run into a
	// terminal CANCELLED state and the new ones start from zero, exactly as
	// if the case had just been created at the new severity. This also
	// means a clock type no longer applicable after a severity DOWNGRADE
	// (e.g. Catastrophic -> Low losing "workaround"/"resolution") is
	// cancelled along with every other clock, not left running -- unlike
	// RegisterCaseClocks alone, the cancellation touches every clock type on
	// the case, not just the ones the new severity resolves.
	//
	// A WORKAROUND/RESOLUTION clock that had merely BREACHED under the old
	// severity (ran out the wall clock without ever being satisfied) IS
	// cancelled and replaced by a fresh one here, same as a still-running
	// IN_PROGRESS/PAUSED clock -- a real, reported bug had this treated the
	// same as a genuine completion, leaving a case's workaround/resolution
	// tracking permanently stuck on a stale, timed-out clock from the OLD
	// severity instead of starting over under the new one. RESPONSE is the
	// one deliberate exception, per explicit product direction: "did a
	// support engineer reply at all" is a fact about the past a severity
	// change cannot undo either way, so a RESPONSE clock already BREACHED
	// (the first-reply window closed unanswered) is treated the same as one
	// already ACHIEVED (a reply came in) -- neither is cancelled or
	// resurrected by a later severity change (see repository.
	// SLAEngineRepository's own slaEngineRevisionBlockStages doc comment for
	// the exact per-target rule). A clock already in a GENUINE completion
	// outcome (e.g. a workaround CompleteWorkaroundClock already provided)
	// is likewise always left untouched -- that outcome already happened and
	// a later severity change must not undo it.
	//
	// Because cancellation and registration run in one transaction, a
	// failure partway through never leaves the case with its old clocks
	// cancelled and no replacement -- either the whole revision applies, or
	// none of it does and the case's prior clocks are untouched.
	//
	// severity/projectID have the exact same handling as RegisterCaseClocks
	// (see its own doc comment) -- projectID is likewise unused.
	ReviseCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, projectID string)
	// CompleteResponseClock marks the case's CSM-authored "response" clock
	// ACHIEVED -- called from CreateCaseComment when the new comment
	// qualifies as the case's first substantive support-engineer reply (see
	// sn_case_service.go's applyResponseSLAOnComment-equivalent hook for the
	// exact qualification check, ported from the old design).
	CompleteResponseClock(ctx context.Context, caseID string)
	// CompleteWorkaroundClock marks the case's CSM-authored "workaround"
	// clock ACHIEVED -- called when a case's WorkaroundProvided is set to
	// true (the "Provide Workaround" action), the one genuine "workaround
	// was provided" signal that exists anywhere in the domain model. Closes
	// a real, previously-accepted gap: ApplyCaseStateEffects below only
	// ever paused this clock, on any state including Closed, since it had
	// no signal of its own to complete it on.
	CompleteWorkaroundClock(ctx context.Context, caseID string)
	// CompleteFixEtaSharedClocks marks the case's CSM-authored "workaround"
	// AND "resolution" clocks ACHIEVED -- called when a PATCH shares a fix
	// ETA with the customer (req.AddPublicComment true alongside a fix-ETA
	// date, the webapp's "Share fix ETA with customer" action; ServiceNow
	// data source only -- see case_service.go's own UpdateCase rejection
	// list, AddPublicComment has no Postgres equivalent). Once WSO2 has
	// committed a fix timeline to the customer, neither clock has anything
	// further to track: "complete" here means the same real, uncapped
	// elapsed-time-at-this-moment semantics CompleteWorkaroundClock/
	// ApplyCaseStateEffects already use (see repository.
	// SLAEngineRepository.CompleteClock's own doc comment) -- not an
	// unconditional 100%, and not a no-op if one or both already happen to
	// be BREACHED. A clock already ACHIEVED/CANCELLED/COMPLETED (an earlier
	// workaround/close) is simply unaffected, so calling this alongside
	// CompleteWorkaroundClock on the same PATCH (a caller can set
	// workaroundProvided and addPublicComment together) is safe -- whichever
	// one runs first wins, the second is a no-op for that clock.
	CompleteFixEtaSharedClocks(ctx context.Context, caseID string)
	// ApplyCaseStateEffects pauses/resumes the case's CSM-authored
	// "workaround"/"resolution" clocks in reaction to a state-changing PATCH
	// -- see the old design's applyCaseStateSLAEffects for the exact
	// per-state behavior this ports -- and, on CaseStateClosed, completes
	// all three clock types ("response"/"workaround"/"resolution"), not
	// just "resolution": a closed case has nothing left to track on any of
	// them, so whichever haven't already reached a genuine completion
	// (CompleteResponseClock/CompleteWorkaroundClock, or an earlier close)
	// are finalized here with their real elapsed time, however far past
	// 100% a still-BREACHED one has climbed (see repository.
	// SLAEngineRepository.CompleteClock's own doc comment for why BREACHED
	// is completable at all) -- rather than left running, or merely paused
	// with no real disposition, forever. A clock already ACHIEVED/CANCELLED/
	// COMPLETED is simply unaffected (CompleteClock's own stage filter
	// matches nothing for it), so this is safe to call unconditionally for
	// all three on every close, not just the ones that happen to still need
	// it.
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

// resolveApplicablePolicies resolves the real sla_policy row for every clock
// type severity applies to (slaApplicableClockTypes) -- shared by
// RegisterCaseClocks and ReviseCaseClocks so both derive the same "what
// should this case's clocks look like" answer the same way. Returns nil,
// false (not an error) for a nil severity or one with no applicable clock
// types.
//
// The second return, lookupFailed, distinguishes two very different reasons
// a clock type can be missing from the returned slice: slaPolicyResolver.
// resolve's own doc comment explains why a genuinely-absent policy (no error)
// and a failed lookup (an error, e.g. a database blip) both drop that clock
// type from the slice the same way, but callers must NOT treat them the
// same. RegisterCaseClocks (case creation) safely ignores lookupFailed --
// nothing existing is at risk, and a missing clock type there is retried the
// next time this case is touched. ReviseCaseClocks must check it: proceeding
// to ReviseClocks with a policy list that's incomplete because of a lookup
// failure (not because the policy is genuinely unconfigured) would cancel
// the case's existing clocks and commit no replacement for the one that
// failed to resolve.
func (s *slaEngineService) resolveApplicablePolicies(ctx context.Context, caseID string, severity *domain.CaseSeverity, _ string) (policies []repository.SLAPolicyRef, lookupFailed bool) {
	if severity == nil {
		return nil, false
	}
	clockTypes, ok := slaApplicableClockTypes[*severity]
	if !ok || len(clockTypes) == 0 {
		return nil, false
	}

	policies = make([]repository.SLAPolicyRef, 0, len(clockTypes))
	for _, clockType := range clockTypes {
		policy, ok, err := s.resolver.resolve(ctx, *severity, clockType)
		if err != nil {
			// resolve already logged why -- this clock type's policy
			// couldn't be determined right now, not that it doesn't exist.
			lookupFailed = true
			continue
		}
		if !ok {
			// resolve already logged why -- skipping this clock type is the
			// same "no fallback duration" behavior the old slaDurations
			// map's absent map entries had.
			continue
		}
		policies = append(policies, policy)
	}
	return policies, lookupFailed
}

// RegisterCaseClocks implements SLAEngineService.
func (s *slaEngineService) RegisterCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, projectID string) {
	if severity == nil {
		slog.InfoContext(ctx, "sla engine: not registering clocks, case has no severity", "caseId", caseID)
		return
	}
	// lookupFailed deliberately ignored here -- see resolveApplicablePolicies'
	// own doc comment: nothing existing is at risk at case-creation time, a
	// missing clock type just means one clock type doesn't get registered
	// this time.
	policies, _ := s.resolveApplicablePolicies(ctx, caseID, severity, projectID)
	if len(policies) == 0 {
		slog.WarnContext(ctx, "sla engine: not registering clocks, no applicable policies for severity", "caseId", caseID, "severity", *severity)
		return
	}
	for _, policy := range policies {
		registered, err := s.repo.RegisterClock(ctx, caseID, policy)
		if err != nil {
			slog.ErrorContext(ctx, "sla engine: register clock failed", "caseId", caseID, "clockType", policy.Target, "err", err)
			continue
		}
		if !registered {
			slog.InfoContext(ctx, "sla engine: clock already registered, skipped", "caseId", caseID, "clockType", policy.Target)
		}
	}
}

// ReviseCaseClocks implements SLAEngineService.
func (s *slaEngineService) ReviseCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, projectID string) {
	policies, lookupFailed := s.resolveApplicablePolicies(ctx, caseID, severity, projectID)
	if lookupFailed {
		// Do NOT proceed to ReviseClocks with an incomplete policy list --
		// see resolveApplicablePolicies' own doc comment. Leaving the case's
		// existing clocks completely untouched (stale, but intact) is safer
		// than cancelling them and committing no replacement for the clock
		// type whose policy lookup failed.
		slog.ErrorContext(ctx, "sla engine: revise clocks skipped, a policy lookup failed", "caseId", caseID)
		return
	}
	// Bounded retry (2 attempts, short pause between): ReviseClocks is one
	// atomic transaction (see its own doc comment), so retrying it is safe
	// -- a second attempt after a failed first one just repeats the same
	// cancel-then-register-fresh transaction, it never duplicates rows.
	// This narrows, but does not eliminate, the case-vs-clock-state
	// mismatch a transient DB failure here would otherwise leave behind
	// (the severity PATCH itself has already succeeded by the time this
	// runs) -- a sustained outage still falls through to the same accepted
	// best-effort logging below.
	var cancelled int
	var err error
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
		// clocks are exactly as they were (see SLAEngineRepository.
		// ReviseClocks' own doc comment on why this is one transaction, not
		// two independent calls). Never partially cancelled with no
		// replacement.
		slog.ErrorContext(ctx, "sla engine: revise clocks for severity change failed after retry", "caseId", caseID, "err", err)
		return
	}
	if cancelled > 0 {
		slog.InfoContext(ctx, "sla engine: cancelled active clocks for severity change", "caseId", caseID, "count", cancelled)
	}
}

// CompleteResponseClock implements SLAEngineService.
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
//   - CaseStateAwaitingInfo/CaseStateSolutionProposed: pause both
//     workaround and resolution -- the case is waiting on the customer, not
//     actively being worked.
//   - CaseStateClosed: resume then complete all three clock types --
//     resolution, workaround, and response. Resolution and workaround are
//     each resumed first (in case either was paused) then completed, same
//     "claim its real elapsed percentage at completion time" behavior
//     CompleteResponseClock/CompleteWorkaroundClock already give their own
//     triggers -- see SLAEngineRepository.CompleteClock's own doc comment,
//     including why a clock already BREACHED is still completable here, not
//     left stuck. Response is never paused (this engine never pauses it at
//     any state), so it's completed directly. A clock that already reached
//     a genuine completion (an engineer's reply, an earlier close) is
//     simply untouched -- CompleteClock's own stage filter matches nothing
//     for it -- so completing all three here unconditionally is safe
//     regardless of which ones still needed it. This closes a real,
//     previously-accepted gap: a case closed before response/workaround
//     ever completed on their own used to leave the response clock
//     untouched entirely and the workaround clock merely paused forever,
//     neither with any real final disposition.
//   - Anything else: resume both -- the case is active again.
func (s *slaEngineService) ApplyCaseStateEffects(ctx context.Context, caseID string, state domain.CaseState) {
	workaroundTarget := slaClockTypeTarget[slaClockTypeWorkaround]
	resolutionTarget := slaClockTypeTarget[slaClockTypeResolution]
	responseTarget := slaClockTypeTarget[slaClockTypeResponse]

	switch state {
	case domain.CaseStateAwaitingInfo, domain.CaseStateSolutionProposed:
		if _, err := s.repo.SetPaused(ctx, caseID, workaroundTarget, true); err != nil {
			slog.ErrorContext(ctx, "sla engine: pause workaround clock failed", "caseId", caseID, "err", err)
		}
		if _, err := s.repo.SetPaused(ctx, caseID, resolutionTarget, true); err != nil {
			slog.ErrorContext(ctx, "sla engine: pause resolution clock failed", "caseId", caseID, "err", err)
		}
	case domain.CaseStateClosed:
		if _, err := s.repo.SetPaused(ctx, caseID, resolutionTarget, false); err != nil {
			slog.ErrorContext(ctx, "sla engine: resume resolution clock failed", "caseId", caseID, "err", err)
		}
		if _, err := s.repo.CompleteClock(ctx, caseID, resolutionTarget); err != nil {
			slog.ErrorContext(ctx, "sla engine: complete resolution clock failed", "caseId", caseID, "err", err)
		}
		if _, err := s.repo.SetPaused(ctx, caseID, workaroundTarget, false); err != nil {
			slog.ErrorContext(ctx, "sla engine: resume workaround clock failed", "caseId", caseID, "err", err)
		}
		if _, err := s.repo.CompleteClock(ctx, caseID, workaroundTarget); err != nil {
			slog.ErrorContext(ctx, "sla engine: complete workaround clock failed", "caseId", caseID, "err", err)
		}
		if _, err := s.repo.CompleteClock(ctx, caseID, responseTarget); err != nil {
			slog.ErrorContext(ctx, "sla engine: complete response clock failed", "caseId", caseID, "err", err)
		}
	default:
		if _, err := s.repo.SetPaused(ctx, caseID, workaroundTarget, false); err != nil {
			slog.ErrorContext(ctx, "sla engine: resume workaround clock failed", "caseId", caseID, "err", err)
		}
		if _, err := s.repo.SetPaused(ctx, caseID, resolutionTarget, false); err != nil {
			slog.ErrorContext(ctx, "sla engine: resume resolution clock failed", "caseId", caseID, "err", err)
		}
	}
}
