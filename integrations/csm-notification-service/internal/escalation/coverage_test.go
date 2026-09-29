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
	"fmt"
	"strings"
	"testing"
	"time"
)

// Does an incident reported at ANY hour of the week get a ladder?
//
// The specification splits the clock two ways at once, and the seams between
// them are where coverage goes missing. Section 6.0 divides the working day
// into LK hours (06:00–21:00 IST) and Americas hours (21:00–06:00), then
// subdivides LK hours again into a morning rotation, business hours and an
// evening rotation — and the Americas window crosses midnight, so the calendar
// date alone cannot say which day's shift an early-morning incident belongs
// to. A gap at any of those boundaries is an hour of the week in which an
// unattended incident silently gets no ladder at all.
//
// So this sweeps all 168 hours: every hour of every day is asserted to land in
// exactly one shift, on the correct side of the LK/Americas split, resolving
// to a real section 5.0 rule and a schedulable ladder.

// lkHours is the LK working window per section 6.0: 06:00 up to but not
// including 21:00. Everything else is the Americas window.
func lkHours(hour int) bool { return hour >= 6 && hour < 21 }

func TestCoverage_EveryHourOfTheWeekGetsALadder(t *testing.T) {
	// A full week starting Monday 2026-09-07, so every weekday and both
	// weekend days are walked.
	weekStart := time.Date(2026, 9, 7, 0, 0, 0, 0, IST)

	type row struct{ day, hours, shift, rule string }
	var rows []row
	var current row

	for day := 0; day < 7; day++ {
		for hour := 0; hour < 24; hour++ {
			at := weekStart.AddDate(0, 0, day).Add(time.Duration(hour) * time.Hour)
			shift := ShiftAt(at)

			// 1. Every hour resolves to a shift the specification defines.
			if !knownShift(shift) {
				t.Fatalf("%s %02d:00 IST resolved to %q, which section 6.0 does not define",
					at.Weekday(), hour, shift)
			}

			// 2. On the correct side of the LK / Americas split. The
			// Americas shift runs 21:00-06:00, so an hour before 06:00
			// belongs to the night that opened the evening before — that is
			// what makes it an Americas hour rather than an LK one.
			wantLK := lkHours(hour)
			if gotLK := isLKShift(shift); gotLK != wantLK {
				window := "Americas hours (21:00-06:00)"
				if wantLK {
					window = "LK hours (06:00-21:00)"
				}
				t.Errorf("%s %02d:00 IST resolved to %s, but that hour is %s",
					at.Weekday(), hour, shift, window)
			}

			// 3. The shift yields a real routing rule, not a fallthrough.
			rc := RoutingContext{Product: "WSO2 API Manager", ABTEligible: abtYes(),
				AssignedCRETeam: "Atlas", Shift: shift}
			rule := rc.Rule()
			if rule == "UNKNOWN" {
				t.Errorf("%s %02d:00 IST (%s) routes to no rule in section 5.0",
					at.Weekday(), hour, shift)
			}

			// 4. And an incident reported then actually gets a ladder with
			// calls in it — the whole point of the hour being covered.
			tr := testTrigger("P1", shift)
			tr.At = at
			tr.Routing = rc
			plan, err := BuildPlan(context.Background(), tr, DefaultPolicy, fullResolver())
			if err != nil {
				t.Errorf("%s %02d:00 IST (%s): no ladder: %v", at.Weekday(), hour, shift, err)
				continue
			}
			if len(plan.Calls) == 0 {
				t.Errorf("%s %02d:00 IST (%s): ladder has no calls", at.Weekday(), hour, shift)
			}

			// Collect contiguous runs of the same shift for the report.
			label := fmt.Sprintf("%.3s", at.Weekday().String())
			if current.day == label && current.shift == string(shift) {
				current.hours = strings.Split(current.hours, "-")[0] + fmt.Sprintf("-%02d:59", hour)
			} else {
				if current.day != "" {
					rows = append(rows, current)
				}
				current = row{day: label, hours: fmt.Sprintf("%02d:00-%02d:59", hour, hour),
					shift: string(shift), rule: rule}
			}
		}
	}
	rows = append(rows, current)

	if testing.Verbose() {
		var b strings.Builder
		b.WriteString("\nday  hours IST     shift         rule  window\n")
		for _, r := range rows {
			window := "Americas"
			if strings.HasPrefix(r.shift, "LK") {
				window = "LK"
			}
			b.WriteString(fmt.Sprintf("%-4s %-13s %-13s %-5s %s\n", r.day, r.hours, r.shift, r.rule, window))
		}
		t.Log(b.String())
	}
}

// Both windows must be reachable on a weekend as well as a weekday, and the
// weekend variants are the ones the notification level hangs off.
func TestCoverage_BothWindowsOnWeekdaysAndWeekends(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want Shift
	}{
		{"weekday LK hours, early", ist(2026, 9, 9, 6, 0), ShiftLKMorning},
		{"weekday LK hours, business", ist(2026, 9, 9, 12, 0), ShiftLK},
		{"weekday LK hours, late", ist(2026, 9, 9, 20, 59), ShiftLKEvening},
		{"weekday Americas hours, evening side", ist(2026, 9, 9, 21, 0), ShiftUSA},
		{"weekday Americas hours, morning side", ist(2026, 9, 10, 5, 59), ShiftUSA},
		{"weekend LK hours", ist(2026, 9, 12, 12, 0), ShiftLKWeekend},
		{"weekend Americas hours, evening side", ist(2026, 9, 12, 21, 0), ShiftUSAWeekend},
		{"weekend Americas hours, morning side", ist(2026, 9, 13, 5, 59), ShiftUSAWeekend},
		// Friday night is a weekday night: the shift opened before the
		// weekend began.
		{"friday night is a weekday shift", ist(2026, 9, 11, 23, 0), ShiftUSA},
		{"saturday pre-dawn belongs to friday night", ist(2026, 9, 12, 2, 0), ShiftUSA},
		// Monday pre-dawn belongs to Sunday night, which is the weekend.
		{"monday pre-dawn belongs to sunday night", ist(2026, 9, 14, 2, 0), ShiftUSAWeekend},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShiftAt(tc.at); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.at.Format("Mon 15:04 MST"), got, tc.want)
			}
		})
	}
}

// The two windows staff their ladders differently, and that difference has to
// reach the plan: an LK rotation has a notification level, the Americas
// weekday shift does not.
func TestCoverage_WindowsProduceTheLaddersTheyShould(t *testing.T) {
	cases := []struct {
		name       string
		at         time.Time
		abt        bool
		wantRule   string
		wantLevel0 bool
	}{
		{"LK morning rotation", ist(2026, 9, 9, 7, 0), true, "R2", true},
		{"LK business hours", ist(2026, 9, 9, 12, 0), true, "R1", false},
		{"LK evening rotation", ist(2026, 9, 9, 19, 0), true, "R2", true},
		{"LK weekend rotation", ist(2026, 9, 12, 12, 0), true, "R2", true},
		{"Americas weekday", ist(2026, 9, 9, 22, 0), true, "R9", false},
		{"Americas weekend, ABT", ist(2026, 9, 12, 22, 0), true, "R10", false},
		{"Americas weekend, sub team", ist(2026, 9, 12, 22, 0), false, "R12", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rc := RoutingContext{Product: "WSO2 API Manager", ABTEligible: abtFor(tc.abt),
				AssignedCRETeam: "Atlas", Shift: ShiftAt(tc.at)}
			if got := rc.Rule(); got != tc.wantRule {
				t.Errorf("rule = %s, want %s (shift %s)", got, tc.wantRule, rc.Shift)
			}
			if got := rc.HasNotificationLevel(); got != tc.wantLevel0 {
				t.Errorf("LEVEL_0 present = %v, want %v (shift %s)", got, tc.wantLevel0, rc.Shift)
			}

			tr := testTrigger("P1", rc.Shift)
			tr.At = tc.at
			tr.Routing = rc
			plan, err := BuildPlan(context.Background(), tr, DefaultPolicy, fullResolver())
			if err != nil {
				t.Fatal(err)
			}
			startsAtLevel0 := plan.Calls[0].Level == Level0
			if startsAtLevel0 != tc.wantLevel0 {
				t.Errorf("ladder starts at %s; LEVEL_0 expected = %v", plan.Calls[0].Level, tc.wantLevel0)
			}
		})
	}
}

// knownShift reports whether a shift is one section 6.0 defines.
func knownShift(s Shift) bool {
	switch s {
	case ShiftLK, ShiftLKMorning, ShiftLKEvening, ShiftLKWeekend, ShiftUSA, ShiftUSAWeekend:
		return true
	}
	return false
}

// isLKShift reports whether a shift belongs to the LK window rather than the
// Americas one.
func isLKShift(s Shift) bool {
	return s == ShiftLK || s == ShiftLKMorning || s == ShiftLKEvening || s == ShiftLKWeekend
}
