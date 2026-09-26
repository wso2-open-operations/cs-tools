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

package domain

import "time"

// Team Schedule: who from CRE and SRE is working, when, and in which
// escalation tier. Portal-native data with no ServiceNow equivalent, so these
// types describe the system of record rather than a mirror of one.

// ScheduleZone is an SRE time zone. WeekendZoneCode names the zone that
// absorbs this one at the weekend, when three weekday zones collapse into
// two -- without it, a Saturday's small hours (which belong to Friday's TZ3
// crew) cannot be attributed to any weekend zone.
type ScheduleZone struct {
	ID              string  `json:"id"`
	Code            string  `json:"code"`
	Label           string  `json:"label"`
	WeekendZoneCode *string `json:"weekendZoneCode,omitempty"`
	SortOrder       int     `json:"sortOrder"`
}

// ScheduleShift is a named window of the working day. StartMinute and
// EndMinute are counted from midnight in AuthoringTimeZone; an end past 1440
// runs into the next day, so the night block 21:00-06:00 is one window
// (1260 -> 1800) rather than two a reader has to stitch together.
type ScheduleShift struct {
	ID                string  `json:"id"`
	Code              string  `json:"code"`
	ShortCode         string  `json:"shortCode"`
	Label             string  `json:"label"`
	Family            string  `json:"family"`
	ZoneCode          *string `json:"zoneCode,omitempty"`
	Tier              *string `json:"tier,omitempty"`
	DayScope          string  `json:"dayScope"`
	StartMinute       int     `json:"startMinute"`
	EndMinute         int     `json:"endMinute"`
	AuthoringTimeZone string  `json:"authoringTimeZone"`
	IsOnCall          bool    `json:"isOnCall"`
	IsEscalation      bool    `json:"isEscalation"`
	// IsRotation is false for a window that is simply when a team works --
	// regular hours, or the Americas night -- rather than a turn on the rota.
	IsRotation      bool   `json:"isRotation"`
	CrossesMidnight bool   `json:"crossesMidnight"`
	ColourToken     string `json:"colourToken"`
	SortOrder       int    `json:"sortOrder"`
}

// ScheduleAbsenceKind is a reason someone is out of the rota. A table rather
// than an enum so a new category is a row, not a migration.
type ScheduleAbsenceKind struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	ShortCode   string `json:"shortCode"`
	Label       string `json:"label"`
	Bucket      string `json:"bucket"`
	ColourToken string `json:"colourToken"`
	SortOrder   int    `json:"sortOrder"`
}

// ScheduleCatalogue is everything the UI needs before it can draw a rota:
// the windows that exist, the zones they run in, and why someone might be
// out. Served as one payload because a client needs all three to render a
// single day and would otherwise make three round trips.
type ScheduleCatalogue struct {
	Zones        []ScheduleZone        `json:"zones"`
	Shifts       []ScheduleShift       `json:"shifts"`
	AbsenceKinds []ScheduleAbsenceKind `json:"absenceKinds"`
}

// ScheduleEngineer is who is working, flattened onto the assignment so a day
// view renders without a second lookup per person.
type ScheduleEngineer struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	IsLead bool   `json:"isLead"`
}

// ScheduleAssignment is one engineer, one rota day, one window.
//
// RotaDate is the day the CREW is rostered for, not the calendar date of
// every hour worked: a Monday 21:00-06:00 block carries the Monday even
// though six of its hours fall on Tuesday. StartsAt/EndsAt are the resolved
// absolute instants, so a point-in-time lookup needs nothing else and stays
// correct across DST.
type ScheduleAssignment struct {
	ID        string           `json:"id"`
	Engineer  ScheduleEngineer `json:"engineer"`
	TeamKey   string           `json:"teamKey"`
	ShiftCode string           `json:"shiftCode"`
	ZoneCode  *string          `json:"zoneCode,omitempty"`
	Tier      *string          `json:"tier,omitempty"`
	RotaDate  string           `json:"rotaDate"`
	StartsAt  time.Time        `json:"startsAt"`
	EndsAt    time.Time        `json:"endsAt"`
	IsOnCall  bool             `json:"isOnCall"`
	Source    string           `json:"source"`
	Note      *string          `json:"note,omitempty"`
}

// ScheduleAbsence is whole days an engineer is unavailable to the rota.
// EndsOn is nil for a standing allocation that runs until further notice.
type ScheduleAbsence struct {
	ID       string           `json:"id"`
	Engineer ScheduleEngineer `json:"engineer"`
	TeamKey  string           `json:"teamKey"`
	KindCode string           `json:"kindCode"`
	StartsOn string           `json:"startsOn"`
	EndsOn   *string          `json:"endsOn,omitempty"`
	Note     *string          `json:"note,omitempty"`
}

// SearchScheduleAssignmentsRequest bounds a rota read by date and, optionally,
// by team or family.
//
// IncludeOvernight widens the window to catch a block that began on the day
// before From and is still running into it -- the night crew a day view must
// show at the top even though their rota_date is yesterday.
type SearchScheduleAssignmentsRequest struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	TeamKeys []string `json:"teamKeys,omitempty"`
	Family   string   `json:"family,omitempty"`
	UserID   string   `json:"userId,omitempty"`
	// UserEmail finds one engineer's own rota. The portal knows its users by
	// email, not by this service's ids, so resolving that here saves every
	// client a lookup -- and saves them all implementing it differently.
	UserEmail        string `json:"userEmail,omitempty"`
	IncludeOvernight bool   `json:"includeOvernight,omitempty"`
}

// SearchScheduleAbsencesRequest finds absences overlapping a date range.
type SearchScheduleAbsencesRequest struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	TeamKeys  []string `json:"teamKeys,omitempty"`
	UserID    string   `json:"userId,omitempty"`
	UserEmail string   `json:"userEmail,omitempty"`
}

// ScheduleAssignmentsResponse is the rota for the window asked for.
type ScheduleAssignmentsResponse struct {
	Assignments []ScheduleAssignment `json:"assignments"`
	Count       int                  `json:"count"`
}

// ScheduleAbsencesResponse is who is out over the window asked for.
type ScheduleAbsencesResponse struct {
	Absences []ScheduleAbsence `json:"absences"`
	Count    int               `json:"count"`
}
