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
// days" (internal/businessdays.Elapsed). Any other schedule name is unknown;
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
}

// nonFlat lists, in a stable order, the schedule names whose calendar is not
// flat.
var nonFlat = []string{WSO2BusinessDays}

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
