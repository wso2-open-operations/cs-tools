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

import "time"

// IST is the timezone every shift boundary in section 6.0 is expressed in.
// Fixed offset rather than a tzdata lookup: India has no DST and no historical
// transition this service could encounter, and a fixed zone cannot fail to
// load on a container without tzdata installed.
var IST = time.FixedZone("IST", 5*60*60+30*60)

// Shift boundaries from section 6.0's matrix, as hours in IST.
const (
	shiftDayStartHour = 6  // LK_MORNING / LK_WEEKEND open
	lkStartHour       = 9  // LK opens, LK_MORNING closes
	lkEndHour         = 18 // LK closes, LK_EVENING opens
	nightStartHour    = 21 // USA / USA_WEEKEND open, evening shifts close
)

// ShiftAt derives the effective shift for an incident reported at t.
//
// Section 5.0 takes the effective shift from ServiceNow's on-call escalation
// path definition. This service has no access to that module (see Resolver),
// so it reconstructs the same boundaries from section 6.0's matrix directly:
//
//	06:00–09:00 IST   LK_MORNING   (weekend: LK_WEEKEND)
//	09:00–18:00 IST   LK           (weekend: LK_WEEKEND)
//	18:00–21:00 IST   LK_EVENING   (weekend: LK_WEEKEND)
//	21:00–06:00 IST   USA          (weekend: USA_WEEKEND)
//
// The night shift runs 21:00 to 06:00, so it straddles midnight and the
// calendar date alone cannot say which shift-day an early-morning hour belongs
// to. Hours before 06:00 are therefore attributed to the previous day: 02:00 on
// a Monday is still Sunday night's USA_WEEKEND shift, not Monday's USA shift.
// Getting this wrong would route a weekend-night incident to the weekday USA
// rotation, which section 6.0 staffs differently.
//
// KNOWN GAP: public holidays are not modelled. Section 8.0 treats them like
// rotations ("except for special occasions (e.g., public holidays) and some
// rotations"), but the holiday calendar lives in the same ServiceNow schedule
// this function exists to work around. A holiday currently resolves to its
// weekday shift.
func ShiftAt(t time.Time) Shift {
	local := t.In(IST)
	hour := local.Hour()

	// Attribute the pre-dawn hours to the shift-day that started the night
	// before, so weekend-ness is judged against the day the shift opened.
	shiftDay := local
	if hour < shiftDayStartHour {
		shiftDay = local.AddDate(0, 0, -1)
	}
	weekend := isWeekend(shiftDay.Weekday())

	night := hour >= nightStartHour || hour < shiftDayStartHour
	if night {
		if weekend {
			return ShiftUSAWeekend
		}
		return ShiftUSA
	}
	if weekend {
		return ShiftLKWeekend
	}
	switch {
	case hour < lkStartHour:
		return ShiftLKMorning
	case hour < lkEndHour:
		return ShiftLK
	default:
		return ShiftLKEvening
	}
}

func isWeekend(d time.Weekday) bool {
	return d == time.Saturday || d == time.Sunday
}
