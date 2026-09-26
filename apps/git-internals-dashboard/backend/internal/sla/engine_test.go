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

// Test vectors for ComputeSla, StatusAsOf, and WithCurrentStatusBoundary,
// covering budget accrual, coverage windows, terminal/reopen handling, and
// the current-status boundary event.
package sla

import (
	"math"
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// at returns the instant h hours after the fixed test epoch t0.
func at(h int) time.Time {
	return t0.Add(time.Duration(h) * time.Hour)
}

// strp returns a pointer to s, for building literal *string fixture values.
func strp(s string) *string { return &s }

// timep returns a pointer to t.
func timep(t time.Time) *time.Time { return &t }

// ev builds a StatusEvent for status occurring at at(h).
func ev(status string, h int) StatusEvent {
	return StatusEvent{Status: strp(status), OccurredAt: at(h)}
}

var testCfg = Config{
	Budgets: map[string]float64{"High(P2)": 24},
	Accrues: func(s *string) bool {
		return s != nil && (*s == "Open" || *s == "In Progress" || *s == "WOW" || *s == "Reopened")
	},
	IsTerminal: func(s *string) bool {
		return s != nil && (*s == "Resolved" || *s == "Duplicate")
	},
	AtRiskThreshold: 0.75,
}

// closeTo fails the test with label unless got is within 1e-5 of want.
func closeTo(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-5 {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

// TestComputeSlaReturnsNoSlaWhenPriorityHasNoBudget verifies a priority
// absent from Config.Budgets yields NO_SLA with a nil budget.
func TestComputeSlaReturnsNoSlaWhenPriorityHasNoBudget(t *testing.T) {
	r := ComputeSla(nil, []StatusEvent{ev("Open", 0)}, strp("Open"), testCfg, at(10))
	if r.SlaState != NoSla {
		t.Errorf("expected NO_SLA, got %s", r.SlaState)
	}
	if r.BudgetHours != nil {
		t.Errorf("expected nil budgetHours, got %v", *r.BudgetHours)
	}
}

// TestComputeSlaAccruesOnlyProductSideIntervalsAndPausesOnCsSide verifies
// only product-side intervals accrue budget; a CS-side interval pauses it.
func TestComputeSlaAccruesOnlyProductSideIntervalsAndPausesOnCsSide(t *testing.T) {
	// Open 0-4h (accrues), WOC 4-10h (pauses), In Progress 10-14h (accrues) = 8h
	events := []StatusEvent{ev("Open", 0), ev("WOC", 4), ev("In Progress", 10)}
	r := ComputeSla(strp("High(P2)"), events, strp("In Progress"), testCfg, at(14))
	closeTo(t, "consumedHours", r.ConsumedHours, 8)
	if r.SlaState != Ok {
		t.Errorf("expected OK, got %s", r.SlaState)
	}
	if !r.SlaRunning {
		t.Errorf("expected slaRunning=true")
	}
}

// TestComputeSlaOpenIntervalAccruesToNowOnlyWhenCurrentStatusIsProductSide
// verifies the open (still-running) interval stops accruing once the
// current status is CS-side.
func TestComputeSlaOpenIntervalAccruesToNowOnlyWhenCurrentStatusIsProductSide(t *testing.T) {
	paused := ComputeSla(strp("High(P2)"), []StatusEvent{ev("Open", 0), ev("WOC", 5)}, strp("WOC"), testCfg, at(100))
	closeTo(t, "consumedHours", paused.ConsumedHours, 5)
	if paused.SlaRunning {
		t.Errorf("expected slaRunning=false")
	}
}

// TestComputeSlaCrossesAtRiskAt75AndViolatedAt100 verifies the SLA state
// crosses to AT_RISK at the configured threshold and VIOLATED at 100%.
func TestComputeSlaCrossesAtRiskAt75AndViolatedAt100(t *testing.T) {
	atRisk := ComputeSla(strp("High(P2)"), []StatusEvent{ev("Open", 0)}, strp("Open"), testCfg, at(18)) // 18/24
	if atRisk.SlaState != AtRisk {
		t.Errorf("expected AT_RISK, got %s", atRisk.SlaState)
	}
	violated := ComputeSla(strp("High(P2)"), []StatusEvent{ev("Open", 0)}, strp("Open"), testCfg, at(25))
	if violated.SlaState != Violated {
		t.Errorf("expected VIOLATED, got %s", violated.SlaState)
	}
}

// TestComputeSlaNeverResetsOnReopen verifies consumed time keeps
// accumulating across a reopen rather than resetting to zero.
func TestComputeSlaNeverResetsOnReopen(t *testing.T) {
	// Open 0-4h, WOC 4-28h, Reopened 28h..52h => 4 + 24 = 28h > 24h budget
	events := []StatusEvent{ev("Open", 0), ev("WOC", 4), ev("Reopened", 28)}
	r := ComputeSla(strp("High(P2)"), events, strp("Reopened"), testCfg, at(52))
	closeTo(t, "consumedHours", r.ConsumedHours, 28)
	if r.SlaState != Violated {
		t.Errorf("expected VIOLATED, got %s", r.SlaState)
	}
}

// TestComputeSlaTerminalStatusWinsRegardlessOfPctConsumed verifies a
// terminal current status forces TERMINAL and slaRunning=false regardless of
// how much budget was consumed.
func TestComputeSlaTerminalStatusWinsRegardlessOfPctConsumed(t *testing.T) {
	r := ComputeSla(strp("High(P2)"), []StatusEvent{ev("Open", 0), ev("Resolved", 30)}, strp("Resolved"), testCfg, at(40))
	if r.SlaState != Terminal {
		t.Errorf("expected TERMINAL, got %s", r.SlaState)
	}
	if r.SlaRunning {
		t.Errorf("expected slaRunning=false")
	}
	// BreachedEver must still report the real breach (pct=1.25) even though
	// SlaState reports TERMINAL, not VIOLATED.
	if !r.BreachedEver {
		t.Errorf("expected BreachedEver=true despite SlaState=TERMINAL")
	}
}

// TestComputeSlaBreachedEverFalseWhenUnderBudget verifies BreachedEver is
// false while pct never reached 1.0.
func TestComputeSlaBreachedEverFalseWhenUnderBudget(t *testing.T) {
	r := ComputeSla(strp("High(P2)"), []StatusEvent{ev("Open", 0)}, strp("Open"), testCfg, at(10))
	if r.BreachedEver {
		t.Errorf("expected BreachedEver=false, pct never reached 1.0")
	}
}

// TestComputeSlaIgnoresEventsAfterNow verifies an event later than `now`
// (a historical snapshot replay) is excluded from accrual.
func TestComputeSlaIgnoresEventsAfterNow(t *testing.T) {
	events := []StatusEvent{ev("Open", 0), ev("Resolved", 50)}
	r := ComputeSla(strp("High(P2)"), events, StatusAsOf(events, at(10)), testCfg, at(10))
	closeTo(t, "consumedHours", r.ConsumedHours, 10)
	if r.SlaState != Ok {
		t.Errorf("expected OK, got %s", r.SlaState)
	}
}

// --- computeSla — 12x5 IST coverage ---
// IST = UTC+5:30. 09:00 IST = 03:30 UTC; 21:00 IST = 15:30 UTC.
// 2026-01-05 is a Monday, 2026-01-09 a Friday, 2026-01-12 a Monday.

var testCfg12x5 = Config{
	Budgets:         testCfg.Budgets,
	Coverage:        map[string]Coverage{"High(P2)": Coverage12x5Ist},
	Accrues:         testCfg.Accrues,
	IsTerminal:      testCfg.IsTerminal,
	AtRiskThreshold: testCfg.AtRiskThreshold,
}

var mon0105_09ist = time.Date(2026, 1, 5, 3, 30, 0, 0, time.UTC) // Mon 09:00 IST

// TestComputeSla12x5AccruesOnlyInsideWindow verifies a 12x5 IST-covered
// priority only accrues the 09:00-21:00 IST business-hours portion of a
// 24-hour span.
func TestComputeSla12x5AccruesOnlyInsideWindow(t *testing.T) {
	// Mon 09:00 IST -> Tue 09:00 IST: only Mon 09-21 counts = 12h.
	open := StatusEvent{Status: strp("Open"), OccurredAt: mon0105_09ist}
	r := ComputeSla(strp("High(P2)"), []StatusEvent{open}, strp("Open"), testCfg12x5, mon0105_09ist.Add(24*time.Hour))
	closeTo(t, "consumedHours", r.ConsumedHours, 12)
}

// TestComputeSla12x5BurnsZeroBudgetAcrossWeekend verifies the weekend
// between Friday's window close and Monday's window open accrues nothing.
func TestComputeSla12x5BurnsZeroBudgetAcrossWeekend(t *testing.T) {
	fri21 := time.Date(2026, 1, 9, 15, 30, 0, 0, time.UTC) // Fri 21:00 IST (window just closed)
	mon08 := time.Date(2026, 1, 12, 2, 30, 0, 0, time.UTC) // Mon 08:00 IST (before window opens)
	r := ComputeSla(strp("High(P2)"), []StatusEvent{{Status: strp("Open"), OccurredAt: fri21}}, strp("Open"), testCfg12x5, mon08)
	closeTo(t, "consumedHours", r.ConsumedHours, 0)
}

// TestComputeSla12x5ClipsPartialWindow verifies an interval straddling the
// business-hours boundary only counts its portion inside the window.
func TestComputeSla12x5ClipsPartialWindow(t *testing.T) {
	mon20 := time.Date(2026, 1, 5, 14, 30, 0, 0, time.UTC) // Mon 20:00 IST
	mon22 := time.Date(2026, 1, 5, 16, 30, 0, 0, time.UTC) // Mon 22:00 IST
	// Only 20:00-21:00 IST counts = 1h.
	r := ComputeSla(strp("High(P2)"), []StatusEvent{{Status: strp("Open"), OccurredAt: mon20}}, strp("Open"), testCfg12x5, mon22)
	closeTo(t, "consumedHours", r.ConsumedHours, 1)
}

// TestComputeSla12x5SlaRunningFalseOutsideWindow verifies slaRunning is
// false when `now` falls outside the coverage window, even mid-accrual.
func TestComputeSla12x5SlaRunningFalseOutsideWindow(t *testing.T) {
	sat := time.Date(2026, 1, 10, 6, 0, 0, 0, time.UTC) // Saturday
	r := ComputeSla(strp("High(P2)"), []StatusEvent{{Status: strp("Open"), OccurredAt: mon0105_09ist}}, strp("Open"), testCfg12x5, sat)
	if r.SlaRunning {
		t.Errorf("expected slaRunning=false")
	}
}

// TestComputeSlaFallsBackTo24x7WhenNoCoverageEntry verifies a priority with
// no Coverage map entry accrues wall-clock time with no business-hours mask.
func TestComputeSlaFallsBackTo24x7WhenNoCoverageEntry(t *testing.T) {
	// testCfg has no coverage map at all for High(P2).
	open := StatusEvent{Status: strp("Open"), OccurredAt: mon0105_09ist}
	r := ComputeSla(strp("High(P2)"), []StatusEvent{open}, strp("Open"), testCfg, mon0105_09ist.Add(24*time.Hour))
	closeTo(t, "consumedHours", r.ConsumedHours, 24) // full wall-clock — no business-hours mask
}

// --- 12x5 with holidays ---

// mon0105Holiday is the IST-calendar-day index of Mon 2026-01-05, the day
// mon0105_09ist falls on.
var mon0105Holiday = func() int64 {
	d, err := time.Parse("2006-01-02", "2026-01-05")
	if err != nil {
		panic(err)
	}
	return HolidayDayIndex(d)
}()

// TestComputeSla12x5SkipsHolidayEntirely verifies a configured holiday
// removes that whole day's window from accrual, even though it's an
// ordinary weekday.
func TestComputeSla12x5SkipsHolidayEntirely(t *testing.T) {
	cfg := testCfg12x5
	cfg.Holidays = map[int64]bool{mon0105Holiday: true}
	open := StatusEvent{Status: strp("Open"), OccurredAt: mon0105_09ist}
	r := ComputeSla(strp("High(P2)"), []StatusEvent{open}, strp("Open"), cfg, mon0105_09ist.Add(24*time.Hour))
	closeTo(t, "consumedHours", r.ConsumedHours, 0)
}

// TestComputeSla12x5SlaRunningFalseDuringHoliday verifies slaRunning is
// false during a configured holiday, even mid-business-hours.
func TestComputeSla12x5SlaRunningFalseDuringHoliday(t *testing.T) {
	cfg := testCfg12x5
	cfg.Holidays = map[int64]bool{mon0105Holiday: true}
	r := ComputeSla(strp("High(P2)"), []StatusEvent{{Status: strp("Open"), OccurredAt: mon0105_09ist}}, strp("Open"), cfg, mon0105_09ist)
	if r.SlaRunning {
		t.Errorf("expected slaRunning=false during a configured holiday")
	}
}

// TestComputeSla12x5HolidayDoesNotAffect24x7 verifies a holiday only removes
// hours from Coverage12x5Ist — a 24x7 budget is unaffected.
func TestComputeSla12x5HolidayDoesNotAffect24x7(t *testing.T) {
	cfg := testCfg // High(P2) here has no Coverage entry -> falls back to 24x7
	cfg.Holidays = map[int64]bool{mon0105Holiday: true}
	open := StatusEvent{Status: strp("Open"), OccurredAt: mon0105_09ist}
	r := ComputeSla(strp("High(P2)"), []StatusEvent{open}, strp("Open"), cfg, mon0105_09ist.Add(24*time.Hour))
	closeTo(t, "consumedHours", r.ConsumedHours, 24)
}

// TestComputeSla12x5HolidayMatchesMinuteByMinuteOracle differentially
// verifies covered12x5IstMs/within12x5Ist agree with holidays involved: a
// full week (one weekday marked a holiday), checked minute by minute.
func TestComputeSla12x5HolidayMatchesMinuteByMinuteOracle(t *testing.T) {
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC) // Monday 00:00 UTC
	end := start.AddDate(0, 0, 7)

	var holidayDate time.Time
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		ist := time.UnixMilli(d.UnixMilli() + istOffsetMs).UTC()
		if ist.Weekday() == time.Wednesday {
			holidayDate = time.Date(ist.Year(), ist.Month(), ist.Day(), 0, 0, 0, 0, time.UTC)
			break
		}
	}
	holidays := map[int64]bool{HolidayDayIndex(holidayDate): true}

	startMs, endMs := start.UnixMilli(), end.UnixMilli()
	got := covered12x5IstMs(startMs, endMs, holidays)

	var want int64
	for ms := startMs; ms < endMs; ms += 60_000 {
		if within12x5Ist(ms, holidays) {
			want += 60_000
		}
	}
	if got != want {
		t.Errorf("covered12x5IstMs=%d, minute-by-minute oracle=%d", got, want)
	}
	if want == 0 {
		t.Fatalf("oracle computed zero covered ms — test isn't exercising the window")
	}
}

// --- statusAsOf ---

// TestStatusAsOfReturnsStatusActiveAtInstant verifies StatusAsOf returns nil
// before the first event and each event's status thereafter, holding until
// the next transition.
func TestStatusAsOfReturnsStatusActiveAtInstant(t *testing.T) {
	events := []StatusEvent{ev("Open", 5), ev("WOC", 10)}
	if got := StatusAsOf(events, at(1)); got != nil {
		t.Errorf("expected nil before first event, got %v", *got)
	}
	if got := StatusAsOf(events, at(7)); got == nil || *got != "Open" {
		t.Errorf("expected Open at h=7, got %v", got)
	}
	if got := StatusAsOf(events, at(10)); got == nil || *got != "WOC" {
		t.Errorf("expected WOC at h=10, got %v", got)
	}
	if got := StatusAsOf(events, at(99)); got == nil || *got != "WOC" {
		t.Errorf("expected WOC at h=99, got %v", got)
	}
}

// --- withCurrentStatusBoundary ---

// TestWithCurrentStatusBoundaryDivergenceSplitsFinalInterval verifies a
// current status that diverges from the last event appends a boundary event
// at currentStatusAt, splitting the final interval correctly for accrual.
func TestWithCurrentStatusBoundaryDivergenceSplitsFinalInterval(t *testing.T) {
	events := []StatusEvent{ev("Open", 0), ev("WOC", 4)} // last event WOC@T4
	out := WithCurrentStatusBoundary(events, strp("In Progress"), timep(at(10)), at(14))
	if len(out) != 3 {
		t.Fatalf("expected 3 events, got %d", len(out))
	}
	if out[2].Status == nil || *out[2].Status != "In Progress" || !out[2].OccurredAt.Equal(at(10)) {
		t.Errorf("expected boundary event {In Progress, at(10)}, got %+v", out[2])
	}

	r := ComputeSla(strp("High(P2)"), out, strp("In Progress"), testCfg, at(14))
	// Open 0-4 (accrues) + In Progress 10-14 (accrues) = 8h; WOC 4-10 pauses.
	closeTo(t, "consumedHours", r.ConsumedHours, 8)
	if !r.SlaRunning {
		t.Errorf("expected slaRunning=true")
	}
}

// TestWithCurrentStatusBoundaryEmptyTimelineAccruesFromCurrentStatusAt
// verifies an empty timeline still produces one boundary event at
// currentStatusAt, so accrual starts from there.
func TestWithCurrentStatusBoundaryEmptyTimelineAccruesFromCurrentStatusAt(t *testing.T) {
	out := WithCurrentStatusBoundary([]StatusEvent{}, strp("Open"), timep(at(0)), at(5))
	if len(out) != 1 || out[0].Status == nil || *out[0].Status != "Open" || !out[0].OccurredAt.Equal(at(0)) {
		t.Fatalf("expected [{Open, at(0)}], got %+v", out)
	}
	r := ComputeSla(strp("High(P2)"), out, strp("Open"), testCfg, at(5))
	closeTo(t, "consumedHours", r.ConsumedHours, 5)
}

// TestWithCurrentStatusBoundaryClampsToLastEventWhenCurrentStatusAtIsEarlier
// verifies a currentStatusAt earlier than the last event clamps the boundary
// to the last event's time instead of going backwards.
func TestWithCurrentStatusBoundaryClampsToLastEventWhenCurrentStatusAtIsEarlier(t *testing.T) {
	events := []StatusEvent{ev("Open", 0), ev("WOC", 10)}
	out := WithCurrentStatusBoundary(events, strp("In Progress"), timep(at(3)), at(20))
	if len(out) != 3 {
		t.Fatalf("expected 3 events, got %d", len(out))
	}
	if out[2].Status == nil || *out[2].Status != "In Progress" || !out[2].OccurredAt.Equal(at(10)) {
		t.Errorf("expected boundary clamped to last.occurredAt=at(10), got %+v", out[2])
	}
	// Walk order preserved (ascending).
	for i := 1; i < len(out); i++ {
		if out[i].OccurredAt.Before(out[i-1].OccurredAt) {
			t.Errorf("expected ascending order, index %d (%v) before %d (%v)", i, out[i].OccurredAt, i-1, out[i-1].OccurredAt)
		}
	}
}

// TestWithCurrentStatusBoundaryAgreementIsNoOp verifies a current status
// that already agrees with the last event appends nothing.
func TestWithCurrentStatusBoundaryAgreementIsNoOp(t *testing.T) {
	events := []StatusEvent{ev("Open", 0), ev("In Progress", 5)}
	out := WithCurrentStatusBoundary(events, strp("In Progress"), timep(at(5)), at(20))
	if len(out) != len(events) {
		t.Fatalf("expected no-op (same length), got %d vs %d", len(out), len(events))
	}
	for i := range events {
		if !statusEqual(out[i].Status, events[i].Status) || !out[i].OccurredAt.Equal(events[i].OccurredAt) {
			t.Errorf("expected out[%d]=%+v to equal events[%d]=%+v", i, out[i], i, events[i])
		}
	}
}

// TestWithCurrentStatusBoundaryIgnoresFutureDatedLastEvent reproduces a
// skewed/contradictory last event stamped after now: it must not anchor the
// divergence clamp — the paused hour (9->10, board says WOC since h=9) must
// not accrue.
func TestWithCurrentStatusBoundaryIgnoresFutureDatedLastEvent(t *testing.T) {
	now := at(10)
	events := []StatusEvent{
		ev("Open", 0),
		{Status: strp("In Progress"), OccurredAt: now.Add(2 * time.Minute)}, // skewed, future-dated
	}
	out := WithCurrentStatusBoundary(events, strp("WOC"), timep(at(9)), now)
	r := ComputeSla(strp("High(P2)"), out, strp("WOC"), testCfg, now)
	closeTo(t, "consumedHours", r.ConsumedHours, 9)
}

// TestWithCurrentStatusBoundaryFutureLastEventAgreeingWithCurrentStatusStillGetsBoundary
// covers the agreement-case variant: the (unfiltered) last event is
// future-dated and its status equals currentStatus. Matching on that
// unfiltered last event would short-circuit to "no boundary needed",
// silently losing the pause at currentStatusAt once ComputeSla drops the
// future event on its own filter, so the boundary must instead be derived
// from currentStatusAt.
func TestWithCurrentStatusBoundaryFutureLastEventAgreeingWithCurrentStatusStillGetsBoundary(t *testing.T) {
	now := at(10)
	events := []StatusEvent{
		ev("Open", 0),
		{Status: strp("WOC"), OccurredAt: now.Add(2 * time.Minute)}, // skewed, agrees with currentStatus
	}
	out := WithCurrentStatusBoundary(events, strp("WOC"), timep(at(9)), now)
	if len(out) != 2 {
		t.Fatalf("expected boundary event to be appended (not suppressed), got %d events: %+v", len(out), out)
	}
	last := out[len(out)-1]
	if last.Status == nil || *last.Status != "WOC" || !last.OccurredAt.Equal(at(9)) {
		t.Errorf("expected boundary {WOC, at(9)}, got %+v", last)
	}
	r := ComputeSla(strp("High(P2)"), out, strp("WOC"), testCfg, now)
	closeTo(t, "consumedHours", r.ConsumedHours, 9)
}

// TestWithCurrentStatusBoundaryEmptyTimelineClampsFutureCurrentStatusAt
// verifies a future-dated currentStatusAt clamps to now rather than
// producing an event ComputeSla then filters out entirely.
func TestWithCurrentStatusBoundaryEmptyTimelineClampsFutureCurrentStatusAt(t *testing.T) {
	out := WithCurrentStatusBoundary([]StatusEvent{}, strp("Open"), timep(at(15)), at(10))
	if len(out) != 1 || out[0].Status == nil || *out[0].Status != "Open" || !out[0].OccurredAt.Equal(at(10)) {
		t.Fatalf("expected boundary clamped to now=at(10), got %+v", out)
	}
}

// TestAdjustForClosureCapsClockAndForcesTerminal verifies a closed issue's
// clock freezes at closedAt and its state reports TERMINAL regardless of
// board status.
func TestAdjustForClosureCapsClockAndForcesTerminal(t *testing.T) {
	closedAt := at(5)
	cfg, now := AdjustForClosure(testCfg, at(10), true, &closedAt)
	if !now.Equal(at(5)) {
		t.Errorf("expected now capped to closedAt=at(5), got %v", now)
	}
	if !cfg.IsTerminal(strp("In Progress")) {
		t.Errorf("expected closed cfg to report terminal regardless of board status")
	}
}

// TestAdjustForClosureNoopWhenNotClosed verifies an open issue's cfg/now
// pass through unchanged.
func TestAdjustForClosureNoopWhenNotClosed(t *testing.T) {
	cfg, now := AdjustForClosure(testCfg, at(10), false, nil)
	if !now.Equal(at(10)) {
		t.Errorf("expected now unchanged, got %v", now)
	}
	if cfg.IsTerminal(strp("In Progress")) {
		t.Errorf("expected original IsTerminal to still apply (In Progress isn't terminal)")
	}
}

// TestAdjustForClosureTerminalEvenWithoutClosedAt verifies a closed issue
// with an unknown closedAt still reports terminal, just without capping the
// clock (nothing sane to cap it to).
func TestAdjustForClosureTerminalEvenWithoutClosedAt(t *testing.T) {
	cfg, now := AdjustForClosure(testCfg, at(10), true, nil)
	if !now.Equal(at(10)) {
		t.Errorf("expected now unchanged when closedAt is unknown, got %v", now)
	}
	if !cfg.IsTerminal(strp("In Progress")) {
		t.Errorf("expected terminal regardless of missing closedAt")
	}
}

// TestAdjustForClosureNoOverrideBeforeClosedAt is the regression for the
// seed's per-day replay: a caller re-derives (cfg, now) once per historical
// instant, and an instant that precedes closedAt must compute exactly as if
// the issue were still open — otherwise every pre-closure day gets rewritten
// to TERMINAL too, erasing real VIOLATED/AT_RISK history from the trend.
func TestAdjustForClosureNoOverrideBeforeClosedAt(t *testing.T) {
	closedAt := at(10)
	cfg, now := AdjustForClosure(testCfg, at(5), true, &closedAt) // "now" precedes closedAt
	if !now.Equal(at(5)) {
		t.Errorf("expected now unchanged for an instant before closedAt, got %v", now)
	}
	if cfg.IsTerminal(strp("In Progress")) {
		t.Errorf("expected no terminal override for an instant before closedAt")
	}
	if !cfg.IsTerminal(strp("Resolved")) {
		t.Errorf("expected the original IsTerminal to still apply for a genuinely terminal status")
	}
}

// TestComputeSlaClosureCapsConsumptionAndReportsTerminal is an engine-level
// regression: closing the GitHub issue while its board status still accrues
// must freeze consumption at closure and stop the clock (SlaRunning=false),
// not keep accruing indefinitely.
func TestComputeSlaClosureCapsConsumptionAndReportsTerminal(t *testing.T) {
	events := []StatusEvent{ev("Open", 0)}
	closedAt := at(5)
	cfg, now := AdjustForClosure(testCfg, at(10), true, &closedAt)
	r := ComputeSla(strp("High(P2)"), events, strp("In Progress"), cfg, now)
	closeTo(t, "consumedHours", r.ConsumedHours, 5)
	if r.SlaState != Terminal {
		t.Errorf("expected TERMINAL, got %s", r.SlaState)
	}
	if r.SlaRunning {
		t.Errorf("expected SlaRunning=false once terminal")
	}
}
