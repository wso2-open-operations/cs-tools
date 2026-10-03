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
	"testing"
	"time"
)

// ist builds an IST instant, which is the timezone every section 6.0 boundary
// is stated in.
func ist(year int, month time.Month, day, hour, min int) time.Time {
	return time.Date(year, month, day, hour, min, 0, 0, IST)
}

func TestShiftAt_Section6Boundaries(t *testing.T) {
	// 2026-09-09 is a Wednesday; 2026-09-12 a Saturday; 2026-09-13 a Sunday.
	tests := []struct {
		name string
		at   time.Time
		want Shift
	}{
		{"weekday 05:59 is still the previous night's USA shift", ist(2026, 9, 9, 5, 59), ShiftUSA},
		{"weekday 06:00 opens the morning rotation", ist(2026, 9, 9, 6, 0), ShiftLKMorning},
		{"weekday 08:59 is still the morning rotation", ist(2026, 9, 9, 8, 59), ShiftLKMorning},
		{"weekday 09:00 opens business hours", ist(2026, 9, 9, 9, 0), ShiftLK},
		{"weekday 17:59 is still business hours", ist(2026, 9, 9, 17, 59), ShiftLK},
		{"weekday 18:00 opens the evening rotation", ist(2026, 9, 9, 18, 0), ShiftLKEvening},
		{"weekday 20:59 is still the evening rotation", ist(2026, 9, 9, 20, 59), ShiftLKEvening},
		{"weekday 21:00 opens the USA shift", ist(2026, 9, 9, 21, 0), ShiftUSA},

		{"saturday 06:00 is the weekend day shift", ist(2026, 9, 12, 6, 0), ShiftLKWeekend},
		{"saturday 14:00 is the weekend day shift", ist(2026, 9, 12, 14, 0), ShiftLKWeekend},
		{"saturday 20:59 is still the weekend day shift", ist(2026, 9, 12, 20, 59), ShiftLKWeekend},
		{"saturday 21:00 opens the USA weekend shift", ist(2026, 9, 12, 21, 0), ShiftUSAWeekend},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShiftAt(tc.at); got != tc.want {
				t.Errorf("ShiftAt(%s) = %s, want %s", tc.at.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

// The night shift runs 21:00-06:00, so it straddles midnight and the calendar
// date alone cannot say which shift-day an early-morning hour belongs to.
// Getting this wrong routes a weekend-night incident to the weekday USA
// rotation, which section 6.0 staffs differently.
func TestShiftAt_NightShiftBelongsToTheDayItOpened(t *testing.T) {
	// 02:00 Sunday is Saturday night's shift — still the weekend.
	if got := ShiftAt(ist(2026, 9, 13, 2, 0)); got != ShiftUSAWeekend {
		t.Errorf("Sunday 02:00 = %s, want %s (Saturday night's shift)", got, ShiftUSAWeekend)
	}
	// 02:00 Monday is Sunday night's shift — also still the weekend.
	if got := ShiftAt(ist(2026, 9, 14, 2, 0)); got != ShiftUSAWeekend {
		t.Errorf("Monday 02:00 = %s, want %s (Sunday night's shift)", got, ShiftUSAWeekend)
	}
	// 02:00 Saturday is Friday night's shift — a weekday night.
	if got := ShiftAt(ist(2026, 9, 12, 2, 0)); got != ShiftUSA {
		t.Errorf("Saturday 02:00 = %s, want %s (Friday night's shift)", got, ShiftUSA)
	}
}

// The shift is derived in IST regardless of the incoming timestamp's own zone.
func TestShiftAt_ConvertsToIST(t *testing.T) {
	// 04:00 UTC on a Wednesday is 09:30 IST — business hours, not the USA
	// night shift a naive UTC reading would give.
	utc := time.Date(2026, 9, 9, 4, 0, 0, 0, time.UTC)
	if got := ShiftAt(utc); got != ShiftLK {
		t.Errorf("ShiftAt(%s) = %s, want %s", utc.Format(time.RFC3339), got, ShiftLK)
	}
}
