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

// Port of v3's src/server/db/sla.test.ts — every vector there has a direct
// counterpart here, read line by line rather than sampled.
package sla

import (
	"math"
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func at(h int) time.Time {
	return t0.Add(time.Duration(h) * time.Hour)
}

func strp(s string) *string { return &s }

func timep(t time.Time) *time.Time { return &t }

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
	PossibleThreshold: 0.75,
}

func closeTo(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-5 {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

func TestComputeSlaReturnsNoSlaWhenPriorityHasNoBudget(t *testing.T) {
	r := ComputeSla(nil, []StatusEvent{ev("Open", 0)}, strp("Open"), testCfg, at(10))
	if r.SlaState != NoSla {
		t.Errorf("expected NO_SLA, got %s", r.SlaState)
	}
	if r.BudgetHours != nil {
		t.Errorf("expected nil budgetHours, got %v", *r.BudgetHours)
	}
}

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

func TestComputeSlaOpenIntervalAccruesToNowOnlyWhenCurrentStatusIsProductSide(t *testing.T) {
	paused := ComputeSla(strp("High(P2)"), []StatusEvent{ev("Open", 0), ev("WOC", 5)}, strp("WOC"), testCfg, at(100))
	closeTo(t, "consumedHours", paused.ConsumedHours, 5)
	if paused.SlaRunning {
		t.Errorf("expected slaRunning=false")
	}
}

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

func TestComputeSlaNeverResetsOnReopen(t *testing.T) {
	// Open 0-4h, WOC 4-28h, Reopened 28h..52h => 4 + 24 = 28h > 24h budget
	events := []StatusEvent{ev("Open", 0), ev("WOC", 4), ev("Reopened", 28)}
	r := ComputeSla(strp("High(P2)"), events, strp("Reopened"), testCfg, at(52))
	closeTo(t, "consumedHours", r.ConsumedHours, 28)
	if r.SlaState != Violated {
		t.Errorf("expected VIOLATED, got %s", r.SlaState)
	}
}

func TestComputeSlaTerminalStatusWinsRegardlessOfPctConsumed(t *testing.T) {
	r := ComputeSla(strp("High(P2)"), []StatusEvent{ev("Open", 0), ev("Resolved", 30)}, strp("Resolved"), testCfg, at(40))
	if r.SlaState != Terminal {
		t.Errorf("expected TERMINAL, got %s", r.SlaState)
	}
	if r.SlaRunning {
		t.Errorf("expected slaRunning=false")
	}
}

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
	Budgets:           testCfg.Budgets,
	Coverage:          map[string]Coverage{"High(P2)": Coverage12x5Ist},
	Accrues:           testCfg.Accrues,
	IsTerminal:        testCfg.IsTerminal,
	PossibleThreshold: testCfg.PossibleThreshold,
}

var mon0105_09ist = time.Date(2026, 1, 5, 3, 30, 0, 0, time.UTC) // Mon 09:00 IST

func TestComputeSla12x5AccruesOnlyInsideWindow(t *testing.T) {
	// Mon 09:00 IST -> Tue 09:00 IST: only Mon 09-21 counts = 12h.
	open := StatusEvent{Status: strp("Open"), OccurredAt: mon0105_09ist}
	r := ComputeSla(strp("High(P2)"), []StatusEvent{open}, strp("Open"), testCfg12x5, mon0105_09ist.Add(24*time.Hour))
	closeTo(t, "consumedHours", r.ConsumedHours, 12)
}

func TestComputeSla12x5BurnsZeroBudgetAcrossWeekend(t *testing.T) {
	fri21 := time.Date(2026, 1, 9, 15, 30, 0, 0, time.UTC) // Fri 21:00 IST (window just closed)
	mon08 := time.Date(2026, 1, 12, 2, 30, 0, 0, time.UTC) // Mon 08:00 IST (before window opens)
	r := ComputeSla(strp("High(P2)"), []StatusEvent{{Status: strp("Open"), OccurredAt: fri21}}, strp("Open"), testCfg12x5, mon08)
	closeTo(t, "consumedHours", r.ConsumedHours, 0)
}

func TestComputeSla12x5ClipsPartialWindow(t *testing.T) {
	mon20 := time.Date(2026, 1, 5, 14, 30, 0, 0, time.UTC) // Mon 20:00 IST
	mon22 := time.Date(2026, 1, 5, 16, 30, 0, 0, time.UTC) // Mon 22:00 IST
	// Only 20:00-21:00 IST counts = 1h.
	r := ComputeSla(strp("High(P2)"), []StatusEvent{{Status: strp("Open"), OccurredAt: mon20}}, strp("Open"), testCfg12x5, mon22)
	closeTo(t, "consumedHours", r.ConsumedHours, 1)
}

func TestComputeSla12x5SlaRunningFalseOutsideWindow(t *testing.T) {
	sat := time.Date(2026, 1, 10, 6, 0, 0, 0, time.UTC) // Saturday
	r := ComputeSla(strp("High(P2)"), []StatusEvent{{Status: strp("Open"), OccurredAt: mon0105_09ist}}, strp("Open"), testCfg12x5, sat)
	if r.SlaRunning {
		t.Errorf("expected slaRunning=false")
	}
}

func TestComputeSlaFallsBackTo24x7WhenNoCoverageEntry(t *testing.T) {
	// testCfg has no coverage map at all for High(P2).
	open := StatusEvent{Status: strp("Open"), OccurredAt: mon0105_09ist}
	r := ComputeSla(strp("High(P2)"), []StatusEvent{open}, strp("Open"), testCfg, mon0105_09ist.Add(24*time.Hour))
	closeTo(t, "consumedHours", r.ConsumedHours, 24) // full wall-clock — no business-hours mask
}

// --- statusAsOf ---

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

func TestWithCurrentStatusBoundaryEmptyTimelineAccruesFromCurrentStatusAt(t *testing.T) {
	out := WithCurrentStatusBoundary([]StatusEvent{}, strp("Open"), timep(at(0)), at(5))
	if len(out) != 1 || out[0].Status == nil || *out[0].Status != "Open" || !out[0].OccurredAt.Equal(at(0)) {
		t.Fatalf("expected [{Open, at(0)}], got %+v", out)
	}
	r := ComputeSla(strp("High(P2)"), out, strp("Open"), testCfg, at(5))
	closeTo(t, "consumedHours", r.ConsumedHours, 5)
}

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
