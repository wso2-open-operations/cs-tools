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

// Package slacalendar is the one place that decides which schedule an SLA
// clock runs on and how much time has elapsed on it. Every caller that needs
// "elapsed time between two instants for this sla_policy.schedule" asks for a
// Calendar here instead of comparing schedule names itself.
//
// Known schedules: "" and "24 x 7" (flat wall-clock time) and "WSO2 Business
// days" (internal/businessdays.Elapsed), plus the non-flat "CS 12x5 IST" and
// "CS 24x7". Any other schedule name is unknown;
// callers keep treating it as flat wall-clock time, exactly as before this
// package existed.
package slacalendar

import (
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/businessdays"
)

// Schedule names as stored in sla_policy.schedule.
const (
	// Flat24x7 is the explicit flat schedule name ("" is flat too).
	Flat24x7 = "24 x 7"
	// WSO2BusinessDays is Monday-Friday, America/New_York.
	WSO2BusinessDays = "WSO2 Business days"
	// CS12x5IST is Monday-Friday 09:00-21:00 Asia/Kolkata, no holidays.
	CS12x5IST = "CS 12x5 IST"
	// CS24x7 is wall-clock time, registered as non-flat on purpose: non-flat
	// clocks are computed in Go and can be given an explicit event time,
	// while the flat SQL paths use the database NOW().
	CS24x7 = "CS 24x7"
)

// Calendar measures how much of the interval [start, end) counts on a
// schedule.
type Calendar interface {
	Elapsed(start, end time.Time) time.Duration
}

type flatCalendar struct{}

func (flatCalendar) Elapsed(start, end time.Time) time.Duration { return end.Sub(start) }

type wso2BusinessDaysCalendar struct{}

func (wso2BusinessDaysCalendar) Elapsed(start, end time.Time) time.Duration {
	return businessdays.Elapsed(start, end)
}

// istZone is Asia/Kolkata: a fixed +05:30 offset, India has no DST, so no
// tzdata lookup is needed.
var istZone = time.FixedZone("Asia/Kolkata", 5*3600+30*60)

type cs12x5ISTCalendar struct{}

// Elapsed is the part of [start, end) inside Monday-Friday 09:00-21:00 IST.
func (cs12x5ISTCalendar) Elapsed(start, end time.Time) time.Duration {
	if !end.After(start) {
		return 0
	}
	s, e := start.In(istZone), end.In(istZone)
	day := time.Date(s.Year(), s.Month(), s.Day(), 0, 0, 0, 0, istZone)
	var total time.Duration
	for ; day.Before(e); day = time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, istZone) {
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		open := day.Add(9 * time.Hour)
		closeAt := day.Add(21 * time.Hour)
		lo, hi := open, closeAt
		if s.After(lo) {
			lo = s
		}
		if e.Before(hi) {
			hi = e
		}
		if hi.After(lo) {
			total += hi.Sub(lo)
		}
	}
	return total
}

type cs24x7Calendar struct{}

// Elapsed is end-start floored at zero, like the flat calendar.
func (cs24x7Calendar) Elapsed(start, end time.Time) time.Duration {
	if d := end.Sub(start); d > 0 {
		return d
	}
	return 0
}

type entry struct {
	cal Calendar
	// flat reports that the schedule is plain wall-clock time, which the
	// repository computes in SQL rather than in Go.
	flat bool
}

var registry = map[string]entry{
	"":               {cal: flatCalendar{}, flat: true},
	Flat24x7:         {cal: flatCalendar{}, flat: true},
	WSO2BusinessDays: {cal: wso2BusinessDaysCalendar{}},
	CS12x5IST:        {cal: cs12x5ISTCalendar{}},
	CS24x7:           {cal: cs24x7Calendar{}},
}

// nonFlat lists, in a stable order, the schedule names whose calendar is not
// flat.
var nonFlat = []string{WSO2BusinessDays, CS12x5IST, CS24x7}

// Flat is the flat wall-clock calendar.
func Flat() Calendar { return flatCalendar{} }

// For returns the calendar for a schedule name. known is false for a schedule
// this package does not define; the returned calendar is then the flat one,
// which is how callers must treat it.
func For(schedule string) (cal Calendar, known bool) {
	e, ok := registry[schedule]
	if !ok {
		return flatCalendar{}, false
	}
	return e.cal, true
}

// NonFlat returns the calendar for a schedule that is computed in Go rather
// than as flat wall-clock time. ok is false for flat and unknown schedules.
func NonFlat(schedule string) (cal Calendar, ok bool) {
	e, found := registry[schedule]
	if !found || e.flat {
		return nil, false
	}
	return e.cal, true
}

// NonFlatSchedules returns the schedule names that use a non-flat calendar,
// for routing rows in SQL. The caller may keep the slice but must not modify
// it.
func NonFlatSchedules() []string {
	return append([]string(nil), nonFlat...)
}
