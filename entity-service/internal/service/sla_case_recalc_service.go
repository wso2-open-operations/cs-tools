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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/businessdays"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

// CaseClockPauseRecalcResult is one clock's own outcome from
// SLAEngineService.RecalculateCasePauseTotals.
type CaseClockPauseRecalcResult struct {
	Target   string
	Schedule string
	Skipped  bool
	// SkipReason is "" when Skipped is false.
	SkipReason      string
	OldPauseSeconds int64
	NewPauseSeconds int64
	// OldBusinessPauseSeconds/NewBusinessPauseSeconds are both zero and not
	// meaningful when Schedule isn't 'WSO2 Business days' -- that column is
	// never read for any other schedule (sla_live's own CASE branches), so
	// there is nothing to recompute or report for it.
	OldBusinessPauseSeconds int64
	NewBusinessPauseSeconds int64
	Updated                 bool
}

// CaseSLAPauseRecalcReport is RecalculateCasePauseTotals' own return value.
// HasHistory is false (Clocks always empty) when the case has no state
// activity history at all to replay -- the common case for a real case
// handled through the ServiceNow-backed write path today, not a rare edge
// case (see that method's own doc comment) -- and existing clocks are left
// exactly as they are rather than guessed at.
type CaseSLAPauseRecalcReport struct {
	HasHistory bool
	Clocks     []CaseClockPauseRecalcResult
}

// recalcPauseThresholdSeconds is how far a recomputed total can differ from
// what's stored before this bothers writing a correction -- a replay
// re-deriving the same total the live engine already has isn't unusual, and
// writing back a value only off by clock-skew-sized rounding would just add
// noise (an updated_on bump, an audit trail entry) for no real correction.
const recalcPauseThresholdSeconds = 2

// recalcScheduleWSO2BusinessDays must match repository's own (unexported)
// slaScheduleWSO2BusinessDays exactly -- duplicated here because this
// package has no existing copy of that constant to import.
const recalcScheduleWSO2BusinessDays = "WSO2 Business days"

// RecalculateCasePauseTotals reconstructs one case's own currently-active,
// IN_PROGRESS clocks' pause history from the case's real state-change
// history (work_item_activity) and corrects their stored pause_duration/
// business_pause_duration when they disagree.
//
// Why state history is enough on its own: every real pause_condition/
// resume_condition in this system only ever references the case's state --
// nothing else. That isn't a simplifying assumption of convenience, it's
// structural: anything that WOULD make a different fact change mid-clock
// (a severity revision, a case-type transfer) already cancels the clock
// outright and registers a fresh one (ReviseCaseClocks) rather than merely
// pausing the old one -- so within one clock's own lifetime, state is the
// only fact that can ever matter to its own pause/resume decision. This
// replay reuses applyCaseStateEffectsForPolicy's own exact decision order
// (decidePauseTransition below), not a reimplementation, so it can never
// disagree with what the live engine would have done for the same history.
//
// Why only Postgres-native cases: work_item_activity (migration 0055) is
// only ever written by caseService's own write path -- snCaseService (the
// ServiceNow-backed path this deployment's DATA_SOURCE actually uses for
// real cases) writes none of it. A case with zero state-activity rows has
// no history to replay at all, so HasHistory is false and nothing is
// touched -- never guessed at from an empty log.
//
// Why a later severity/case-type change disqualifies a clock: if one
// happened after this clock's own start_on, the real engine would already
// have cancelled this clock and registered a different one in its place --
// finding one means this row is either stale or something else is
// inconsistent, so it's reported and skipped, never corrected.
//
// IN_PROGRESS and PAUSED clocks are both corrected; a terminal clock
// (COMPLETED/ACHIEVED/CANCELLED/BREACHED) is not -- its own figures are
// final, not something "elapsed time so far" even means for it. A PAUSED
// clock's own frozen business_elapsed_percentage/business_duration/
// remaining_business_duration were computed using its CURRENT pause total
// at the moment it was written (setPausedFlat/setPausedBusinessDays) --
// correcting pause_duration underneath it without also recomputing that
// snapshot would leave it internally inconsistent, so this recomputes and
// overwrites all of them together (UpdateClockPausedSnapshot), using the
// exact same freeze formula those two functions already use, anchored on
// this clock's own paused_on instant instead of NOW().
func (s *slaEngineService) RecalculateCasePauseTotals(ctx context.Context, caseID string) (CaseSLAPauseRecalcReport, error) {
	events, err := s.repo.ListCaseActivityEvents(ctx, caseID, []string{"state", "severity", "type"})
	if err != nil {
		return CaseSLAPauseRecalcReport{}, err
	}
	var stateEvents, lifetimeBoundaryEvents []repository.CaseActivityEvent
	for _, e := range events {
		if e.FieldName == "state" {
			stateEvents = append(stateEvents, e)
		} else {
			lifetimeBoundaryEvents = append(lifetimeBoundaryEvents, e)
		}
	}
	if len(stateEvents) == 0 {
		return CaseSLAPauseRecalcReport{HasHistory: false}, nil
	}

	clocks, err := s.repo.ListActiveClocksForRecalc(ctx, caseID)
	if err != nil {
		return CaseSLAPauseRecalcReport{}, err
	}
	facts, err := s.repo.GetCaseFacts(ctx, caseID)
	if err != nil {
		return CaseSLAPauseRecalcReport{}, err
	}
	now := time.Now()

	report := CaseSLAPauseRecalcReport{HasHistory: true}
	for _, clock := range clocks {
		result := CaseClockPauseRecalcResult{
			Target:                  clock.Target,
			Schedule:                clock.Schedule,
			OldPauseSeconds:         clock.PauseSeconds,
			OldBusinessPauseSeconds: clock.BusinessPauseSeconds,
		}

		if clock.Stage != "IN_PROGRESS" && clock.Stage != "PAUSED" {
			result.Skipped = true
			result.SkipReason = "clock is not currently open (stage=" + clock.Stage + "); nothing to recalculate"
			report.Clocks = append(report.Clocks, result)
			continue
		}

		if reason := laterLifetimeBoundary(lifetimeBoundaryEvents, clock.StartOn); reason != "" {
			result.Skipped = true
			result.SkipReason = reason
			report.Clocks = append(report.Clocks, result)
			continue
		}

		isPaused := clock.Stage == "PAUSED"
		pauseSeconds, businessPauseSeconds, ok, reason := replayClockPauseHistory(clock, stateEvents, facts, now, isPaused)
		if !ok {
			result.Skipped = true
			result.SkipReason = reason
			report.Clocks = append(report.Clocks, result)
			continue
		}

		result.NewPauseSeconds = pauseSeconds
		businessDays := clock.Schedule == recalcScheduleWSO2BusinessDays
		needsUpdate := abs64(pauseSeconds-clock.PauseSeconds) > recalcPauseThresholdSeconds

		var businessPauseArg *int64
		if businessDays {
			result.NewBusinessPauseSeconds = businessPauseSeconds
			if abs64(businessPauseSeconds-clock.BusinessPauseSeconds) > recalcPauseThresholdSeconds {
				needsUpdate = true
			}
			businessPauseArg = &businessPauseSeconds
		}

		if needsUpdate {
			var writeErr error
			if isPaused {
				elapsedPercent, elapsedSeconds, remainingSeconds := frozenPausedSnapshot(clock, pauseSeconds, businessPauseSeconds, businessDays)
				writeErr = s.repo.UpdateClockPausedSnapshot(ctx, caseID, clock.PolicyID, pauseSeconds, businessPauseArg, elapsedPercent, elapsedSeconds, remainingSeconds)
			} else {
				writeErr = s.repo.UpdateClockPauseTotals(ctx, caseID, clock.PolicyID, pauseSeconds, businessPauseArg)
			}
			if writeErr != nil {
				slog.ErrorContext(ctx, "sla engine: recalculate case pause totals: update failed", "caseId", caseID, "target", clock.Target, "err", writeErr)
				result.Skipped = true
				result.SkipReason = "recomputed a correction but failed to write it: " + writeErr.Error()
				report.Clocks = append(report.Clocks, result)
				continue
			}
			result.Updated = true
		}
		report.Clocks = append(report.Clocks, result)
	}
	return report, nil
}

// laterLifetimeBoundary returns a non-empty skip reason when any severity/
// case-type-change activity event happened after startOn -- see
// RecalculateCasePauseTotals' own doc comment for why that disqualifies a
// clock from correction entirely.
func laterLifetimeBoundary(lifetimeBoundaryEvents []repository.CaseActivityEvent, startOn time.Time) string {
	for _, e := range lifetimeBoundaryEvents {
		if e.CreatedOn.After(startOn) {
			return "a " + e.FieldName + " change happened after this clock started -- this clock's own row may be stale"
		}
	}
	return ""
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// frozenPausedSnapshot recomputes a PAUSED clock's own frozen
// business_elapsed_percentage/business_duration/remaining_business_duration
// using exactly setPausedFlat/setPausedBusinessDays' own freeze formula
// (see this file's package doc comment), anchored at clock.PausedOn rather
// than NOW() -- the newly-replayed pause totals substituted in place of the
// stored ones those two functions would otherwise have trusted. clock.PausedOn
// is only nil if the DB row itself is inconsistent (Stage == "PAUSED" with no
// paused_on); that can't happen through the live engine's own write paths, so
// this falls back to clock.StartOn rather than panicking.
func frozenPausedSnapshot(clock repository.ActiveClockForRecalc, pauseSeconds, businessPauseSeconds int64, businessDays bool) (elapsedPercent float64, elapsedSeconds, remainingSeconds int64) {
	pausedOn := clock.StartOn
	if clock.PausedOn != nil {
		pausedOn = *clock.PausedOn
	}

	var elapsed time.Duration
	if businessDays {
		elapsed = businessdays.Elapsed(clock.StartOn, pausedOn) - time.Duration(businessPauseSeconds)*time.Second
	} else {
		elapsed = pausedOn.Sub(clock.StartOn) - time.Duration(pauseSeconds)*time.Second
	}
	if elapsed < 0 {
		elapsed = 0
	}

	duration := time.Duration(clock.DurationSeconds) * time.Second
	remaining := duration - elapsed
	if remaining < 0 {
		remaining = 0
	}
	if clock.DurationSeconds > 0 {
		elapsedPercent = elapsed.Seconds() / float64(clock.DurationSeconds) * 100
	}
	return elapsedPercent, int64(elapsed.Seconds()), int64(remaining.Seconds())
}

// replayClockPauseHistory walks clock's real state-change history from its
// own start to now, applying decidePauseTransition's own stop/cancel/pause/
// resume decision order against the state in effect at each point in time,
// pinning every OTHER fact (severity, case type, product, etc.) to
// currentFacts -- see RecalculateCasePauseTotals' own doc comment for why
// that's exact, not approximate, for every policy that exists today.
//
// ok is false (nothing to apply) when the replay concludes the clock
// should have stopped or been cancelled partway through its own history, or
// that its paused/running state at the end of the replay disagrees with
// expectPaused (clock.Stage == "PAUSED") -- both a genuine disagreement
// with the live engine's own tracked state that needs a person to look at,
// never a guess.
//
// When expectPaused is true, the trailing segment from the last known event
// up to now is never folded into the returned total: that segment is the
// clock's own still-open pause, not a completed one, so including it would
// double count once the clock eventually resumes and the live engine's own
// resume handling adds its own elapsed-since-paused_on span on top.
func replayClockPauseHistory(clock repository.ActiveClockForRecalc, stateEvents []repository.CaseActivityEvent, currentFacts repository.CaseFacts, now time.Time, expectPaused bool) (pauseSeconds, businessPauseSeconds int64, ok bool, reason string) {
	policy := repository.SLAPolicyRef{
		ID:              clock.PolicyID,
		Target:          clock.Target,
		Schedule:        clock.Schedule,
		Source:          clock.Source,
		StopCondition:   clock.StopCondition,
		PauseCondition:  clock.PauseCondition,
		ResumeCondition: clock.ResumeCondition,
		CancelCondition: clock.CancelCondition,
		WhenToCancel:    clock.WhenToCancel,
		WhenToResume:    clock.WhenToResume,
	}

	// The state in effect at clock.StartOn: the newest state-event at or
	// before start_on tells us the state it changed TO; absent that, the
	// earliest known event's own "old" value tells us the state before
	// anything in this log ever changed -- nothing changed between then and
	// start_on either, so it was already that value.
	currentState := stateEvents[0].OldValue
	for _, e := range stateEvents {
		if e.CreatedOn.After(clock.StartOn) {
			break
		}
		currentState = e.NewValue
	}

	f := currentFacts
	f.State = currentState
	paused, settled := decidePauseTransition(policy, caseConditionFacts{c: f}, false)
	if settled {
		return 0, 0, false, "history shows this clock should already have stopped or been cancelled at its own start -- needs manual review"
	}

	businessDays := clock.Schedule == recalcScheduleWSO2BusinessDays
	var flatTotal, businessTotal time.Duration
	lastEventTime := clock.StartOn

	accumulate := func(from, to time.Time) {
		if !paused || !to.After(from) {
			return
		}
		flatTotal += to.Sub(from)
		if businessDays {
			businessTotal += businessdays.Elapsed(from, to)
		}
	}

	for _, e := range stateEvents {
		if !e.CreatedOn.After(clock.StartOn) || e.CreatedOn.After(now) {
			continue
		}
		accumulate(lastEventTime, e.CreatedOn)
		lastEventTime = e.CreatedOn

		f.State = e.NewValue
		newPaused, settled := decidePauseTransition(policy, caseConditionFacts{c: f}, paused)
		if settled {
			return 0, 0, false, "history shows this clock should have stopped or been cancelled partway through -- the live engine still treats it as open; needs manual review"
		}
		paused = newPaused
	}

	if paused != expectPaused {
		if paused {
			return 0, 0, false, "history concludes this clock should currently be paused, but it is tracked as IN_PROGRESS -- needs manual review"
		}
		return 0, 0, false, "history concludes this clock should currently be running, but it is tracked as PAUSED -- needs manual review"
	}
	if !expectPaused {
		accumulate(lastEventTime, now)
	}

	return int64(flatTotal.Seconds()), int64(businessTotal.Seconds()), true, ""
}

// decidePauseTransition applies applyCaseStateEffectsForPolicy's own
// stop/cancel/pause/resume decision order, read-only, at one point in a
// replayed history: settled is true for stop or cancel (the clock's own
// real lifetime appears to have already ended per this history), in which
// case newPaused is meaningless and the caller stops the replay rather than
// guessing further. currentlyPaused carries forward unchanged for the
// "neither condition matches" case, same as the live engine's own "left
// exactly as it is" rule (see ApplyCaseStateEffects' own doc comment).
func decidePauseTransition(policy repository.SLAPolicyRef, f caseConditionFacts, currentlyPaused bool) (newPaused, settled bool) {
	switch {
	case policy.StopCondition != "" && slacondition.Parse(slaPolicyConditionText(policy, policy.StopCondition)).Evaluate(f):
		return currentlyPaused, true
	case slaPolicyShouldCancel(policy, f):
		return currentlyPaused, true
	case policy.PauseCondition != "" && slacondition.Parse(slaPolicyConditionText(policy, policy.PauseCondition)).Evaluate(f):
		return true, false
	case slaPolicyShouldResume(policy, f):
		return false, false
	default:
		return currentlyPaused, false
	}
}
