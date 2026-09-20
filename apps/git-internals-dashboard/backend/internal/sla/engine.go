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

package sla

import (
	"sort"
	"time"
)

const (
	msPerHour = 3_600_000
	msPerDay  = 86_400_000
)

// IST (Asia/Kolkata) is a fixed UTC+5:30 offset with no daylight saving, so a
// constant shift converts UTC <-> IST wall-clock exactly. Adding the offset
// and reading the result as UTC yields the IST calendar day/hour.
const istOffsetMs = 5*msPerHour + 30*60_000

const (
	coverageStartHour = 9  // 09:00 IST
	coverageEndHour   = 21 // 21:00 IST (9pm)
)

// floorDiv is integer division that floors toward negative infinity (like
// JS's Math.floor(a/b)), unlike Go's truncating "/" — matters only for
// instants before 1970, but kept exact rather than assuming all inputs are
// positive.
func floorDiv(a, b int64) int64 {
	q := a / b
	if r := a % b; r != 0 && (r < 0) != (b < 0) {
		q--
	}
	return q
}

// HolidayDayIndex returns the IST-calendar-day index date represents — the
// same index covered12x5IstMs/within12x5Ist key their holidays map by.
// date is expected to be a bare calendar date (e.g. parsed from "2026-01-26"
// via time.Parse("2006-01-02", ...), which yields UTC midnight): its
// UnixMilli, divided exactly by msPerDay, is the day-count since epoch that
// the coverage walk's own `day` variable uses.
func HolidayDayIndex(date time.Time) int64 {
	return date.UnixMilli() / msPerDay
}

// within12x5Ist reports whether instant ms falls inside the 12x5 window
// (Mon-Fri, 09:00-21:00 IST, minus any IST calendar day in holidays).
// holidays may be nil (no holidays configured).
func within12x5Ist(ms int64, holidays map[int64]bool) bool {
	ist := time.UnixMilli(ms + istOffsetMs).UTC()
	dow := ist.Weekday() // Sunday=0 ... Saturday=6, in IST wall-clock terms
	if dow == time.Sunday || dow == time.Saturday {
		return false
	}
	if holidays[floorDiv(ms+istOffsetMs, msPerDay)] {
		return false
	}
	hour := ist.Hour()
	return hour >= coverageStartHour && hour < coverageEndHour
}

// covered12x5IstMs returns the milliseconds of [startMs, endMs) that land
// inside the 12x5 IST window, walking day by day and clamping each day's
// coverage interval to [startMs, endMs). holidays may be nil (no holidays
// configured).
func covered12x5IstMs(startMs, endMs int64, holidays map[int64]bool) int64 {
	if endMs <= startMs {
		return 0
	}
	var total int64
	firstDay := floorDiv(startMs+istOffsetMs, msPerDay)
	lastDay := floorDiv(endMs+istOffsetMs, msPerDay)
	for day := firstDay; day <= lastDay; day++ {
		dow := time.UnixMilli(day * msPerDay).UTC().Weekday()
		if dow == time.Sunday || dow == time.Saturday {
			continue // weekend: no coverage
		}
		if holidays[day] {
			continue // holiday: no coverage
		}
		midnightUtc := day*msPerDay - istOffsetMs // 00:00 IST for this day, in UTC ms
		lo := max(startMs, midnightUtc+coverageStartHour*msPerHour)
		hi := min(endMs, midnightUtc+coverageEndHour*msPerHour)
		if hi > lo {
			total += hi - lo
		}
	}
	return total
}

// coveredMs returns the milliseconds of [startMs, endMs) during which the
// SLA clock ticks under coverage. holidays applies only to Coverage12x5Ist —
// a 24x7 budget already ignores weekends, so a holiday has nothing to remove.
func coveredMs(startMs, endMs int64, coverage Coverage, holidays map[int64]bool) int64 {
	if coverage == Coverage12x5Ist {
		return covered12x5IstMs(startMs, endMs, holidays)
	}
	return max(0, endMs-startMs) // 24x7
}

// isWithinCoverage reports whether the SLA clock is currently ticking at
// instant ms under coverage.
func isWithinCoverage(ms int64, coverage Coverage, holidays map[int64]bool) bool {
	if coverage == Coverage12x5Ist {
		return within12x5Ist(ms, holidays)
	}
	return true
}

// statusEqual reports whether a and b are both nil or point to equal
// strings.
func statusEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// ComputeSla accumulates the time an issue spent in a product-side status,
// never resetting on pause/reopen. Pass the FULL event list; now bounds the
// open interval (pass a past instant to reconstruct a historical snapshot).
// Events after now are ignored.
func ComputeSla(priority *string, events []StatusEvent, currentStatus *string, cfg Config, now time.Time) Result {
	var budget float64
	var hasBudget bool
	if priority != nil {
		budget, hasBudget = cfg.Budgets[*priority]
	}
	if !hasBudget {
		return Result{SlaState: NoSla, SlaRunning: false}
	}

	coverage := Coverage24x7
	if priority != nil {
		if c, ok := cfg.Coverage[*priority]; ok {
			coverage = c
		}
	}

	nowMs := now.UnixMilli()
	sorted := make([]StatusEvent, 0, len(events))
	for _, e := range events {
		if e.OccurredAt.UnixMilli() <= nowMs {
			sorted = append(sorted, e)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].OccurredAt.UnixMilli() < sorted[j].OccurredAt.UnixMilli()
	})

	var consumedMs int64
	for i, e := range sorted {
		start := e.OccurredAt.UnixMilli()
		end := nowMs
		if i+1 < len(sorted) {
			end = sorted[i+1].OccurredAt.UnixMilli()
		}
		// Only time inside the priority's coverage window burns SLA budget.
		if cfg.Accrues(e.Status) {
			consumedMs += coveredMs(start, end, coverage, cfg.Holidays)
		}
	}

	consumedHours := float64(consumedMs) / msPerHour
	var pct float64
	if budget > 0 {
		pct = consumedHours / budget
	}

	var state SlaState
	switch {
	case cfg.IsTerminal(currentStatus):
		state = Terminal
	case pct >= 1.0:
		state = Violated
	case pct >= cfg.AtRiskThreshold:
		state = AtRisk
	default:
		state = Ok
	}

	remaining := budget - consumedHours
	return Result{
		BudgetHours:    &budget,
		ConsumedHours:  consumedHours,
		RemainingHours: &remaining,
		PctConsumed:    &pct,
		SlaState:       state,
		// The clock is "running" only when the status accrues AND we are
		// inside the coverage window (a P2/P3 issue mid-transition at 2am
		// IST is paused). Gated on state != Terminal too: an AdjustForClosure
		// override makes IsTerminal report true for a closed issue whose
		// board status still accrues, and the clock must read "not running"
		// the instant it goes terminal, not just when board status changes.
		SlaRunning: state != Terminal && cfg.Accrues(currentStatus) && isWithinCoverage(nowMs, coverage, cfg.Holidays),
		// Independent of state: computed the same way whether or not
		// currentStatus is terminal, so a caller can OR it against a
		// persisted value and never lose a real breach to a later
		// resolution/downgrade. See the field doc on Result.
		BreachedEver: pct >= 1.0,
	}
}

// AdjustForClosure adapts cfg and now so a GitHub-closed issue stops
// accruing SLA budget at the moment of closure and reports TERMINAL from
// then on, regardless of board status — GitHub closure is authoritative
// over the board, which a process guarantee (a workflow or a human) is
// otherwise relied on to keep in sync.
//
// The override is relative to the instant being computed, not just to
// whether the issue is closed today: a caller replaying history one day at
// a time (the seed's snapshot writer) calls this once per day with that
// day's own `now`, and a day that precedes closedAt must compute exactly as
// it would have before the issue closed — otherwise every pre-closure day
// gets rewritten to TERMINAL too, erasing real VIOLATED/AT_RISK history from
// the trend. So: no override at all for an instant before closedAt: cfg/now
// pass through unchanged. From closedAt onward, IsTerminal reports true and
// now is capped at closedAt, freezing consumption at the moment of closure.
// A nil closedAt (unreachable for real GitHub data — a CLOSED issue always
// carries one) is treated as "already closed": terminal, clock uncapped.
func AdjustForClosure(cfg Config, now time.Time, closed bool, closedAt *time.Time) (Config, time.Time) {
	if !closed || (closedAt != nil && closedAt.After(now)) {
		return cfg, now
	}
	adjusted := cfg
	adjusted.IsTerminal = func(*string) bool { return true }
	if closedAt != nil && closedAt.Before(now) {
		now = *closedAt
	}
	return adjusted, now
}

// StatusAsOf returns the status active at instant at. Assumes events sorted
// ascending.
func StatusAsOf(events []StatusEvent, at time.Time) *string {
	atMs := at.UnixMilli()
	var s *string
	for _, e := range events {
		if e.OccurredAt.UnixMilli() <= atMs {
			s = e.Status
		} else {
			break
		}
	}
	return s
}

// WithCurrentStatusBoundary reconciles the event walk with the project-scoped
// current status (the authoritative source for the final open interval). It
// appends an in-memory boundary event when the
// last event's status disagrees with currentStatus, splitting the final
// interval at currentStatusAt. Never persisted: derived from
// Issue.currentStatus/currentStatusAt on demand.
func WithCurrentStatusBoundary(events []StatusEvent, currentStatus *string, currentStatusAt *time.Time, now time.Time) []StatusEvent {
	nowMs := now.UnixMilli()
	sorted := make([]StatusEvent, 0, len(events))
	for _, e := range events {
		// Mirror ComputeSla's own filter: a future-dated event (clock skew,
		// contradictory data) must never anchor the boundary below, or the
		// clamp floors on it instead of on currentStatusAt.
		if e.OccurredAt.UnixMilli() <= nowMs {
			sorted = append(sorted, e)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].OccurredAt.UnixMilli() < sorted[j].OccurredAt.UnixMilli()
	})

	if len(sorted) == 0 {
		// Empty timeline: the clock starts when the current status was set
		// (exact — currentStatusAt is the field's updatedAt), or contributes
		// nothing if unknown. Clamp to now: a future-dated currentStatusAt
		// must not produce an event ComputeSla then filters out entirely,
		// which would otherwise report SlaRunning=true while consuming 0h.
		if currentStatus == nil {
			return []StatusEvent{}
		}
		occurredAt := now
		if currentStatusAt != nil && currentStatusAt.Before(now) {
			occurredAt = *currentStatusAt
		}
		return []StatusEvent{{Status: currentStatus, OccurredAt: occurredAt}}
	}

	last := sorted[len(sorted)-1]
	if statusEqual(last.Status, currentStatus) {
		return sorted
	}

	// Divergence: clamp the boundary into [last.occurredAt, now] so
	// contradictory timestamps (currentStatusAt older than the last event)
	// can't reorder the walk.
	candidate := now
	if currentStatusAt != nil {
		candidate = *currentStatusAt
	}
	atMs := min(max(candidate.UnixMilli(), last.OccurredAt.UnixMilli()), now.UnixMilli())
	return append(sorted, StatusEvent{Status: currentStatus, OccurredAt: time.UnixMilli(atMs).UTC()})
}
