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

// ScheduleZone is a block of the day a rota is worked in: SaaS SRE's time
// zones TZ1-TZ3, or a rotation's Day and Night. WeekendZoneCode names the zone
// that absorbs this one at the weekend, when three weekday zones collapse into
// two -- without it, a Saturday's small hours (which belong to Friday's TZ3
// crew) cannot be attributed to any weekend zone. RotaCode is the rota the
// zone belongs to; absent for a zone no rota claims, which reads as before.
type ScheduleZone struct {
	ID              string  `json:"id"`
	Code            string  `json:"code"`
	Label           string  `json:"label"`
	WeekendZoneCode *string `json:"weekendZoneCode,omitempty"`
	SortOrder       int     `json:"sortOrder"`
	RotaCode        *string `json:"rotaCode,omitempty"`
}

// ScheduleRota is a named rotation inside a family -- SRE runs SaaS and IaaS,
// SME one per product -- with the rules its own sheet states. A team belongs to
// the rota whose TeamType matches team.type, the same convention family follows.
// EscalationMinutes is informational (nil where the source does not say): the
// escalation ladder keeps its own timing.
type ScheduleRota struct {
	Code              string  `json:"code"`
	Label             string  `json:"label"`
	Family            string  `json:"family"`
	Rotates           string  `json:"rotates"`
	EscalationMinutes *int16  `json:"escalationMinutes,omitempty"`
	SourceSheet       *string `json:"sourceSheet,omitempty"`
	SortOrder         int     `json:"sortOrder"`
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
	IsRotation bool `json:"isRotation"`
	// RequiredHeadcount is the target for this window. Nil means no target is
	// defined, which is not the same as zero -- a view has to render it as
	// "cannot say" rather than "nobody needed".
	RequiredHeadcount *int16 `json:"requiredHeadcount,omitempty"`
	CrossesMidnight   bool   `json:"crossesMidnight"`
	ColourToken       string `json:"colourToken"`
	SortOrder         int    `json:"sortOrder"`
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
	// Custom is true for a kind a lead added from the portal, which a lead may
	// also delete. The catalogue's own kinds, seeded by migration, are not.
	Custom bool `json:"custom"`
	// Family is the rota the kind is offered on, CRE, SRE or SME; absent for a
	// kind every rota uses, which is every kind of leave.
	Family *string `json:"family,omitempty"`
	// Retired is true for a kind no longer offered. It is still served so the
	// days already marked with it keep their label, but nothing should offer it.
	Retired bool `json:"retired,omitempty"`
	// MovesToTeamKey is the team a span of this kind is spent working for --
	// the Brazil rotation moves someone to the Americas team, Migration to
	// the Migration team. Such a span is filed under that team for its dates,
	// so the roster shows the person there and that team's lead may roster
	// them; their own team membership is unchanged.
	MovesToTeamKey *string `json:"movesToTeamKey,omitempty"`
	// WorksRotaThere is true when that stint is rota work on the other team
	// (the Brazil rotation works the Americas rota), so the person is not
	// listed as off the rota on those days.
	WorksRotaThere bool `json:"worksRotaThere,omitempty"`
	// ShowsAsShiftCode is the standing window a span of this kind is drawn as
	// on the roster -- on the team it moves someone to, they work its normal
	// hours. The span is still what the day holds.
	ShowsAsShiftCode *string `json:"showsAsShiftCode,omitempty"`
}

// ScheduleCatalogue is everything the UI needs before it can draw a rota:
// the windows that exist, the zones they run in, and why someone might be
// out. Served as one payload because a client needs all three to render a
// single day and would otherwise make three round trips.
type ScheduleCatalogue struct {
	Zones        []ScheduleZone        `json:"zones"`
	Shifts       []ScheduleShift       `json:"shifts"`
	AbsenceKinds []ScheduleAbsenceKind `json:"absenceKinds"`
	Teams        []ScheduleTeam        `json:"teams"`
	Rotas        []ScheduleRota        `json:"rotas"`
}

// ScheduleTeam is one team the rota is run for.
//
// Served so no client has to hold the list. Team names are organisation
// vocabulary, and a frontend that hardcodes them is coupled to a deploy it
// cannot see -- which is what CSM_TEAM_REGISTRY was built to avoid, and what
// the page and its colour table were doing anyway. SortOrder is the order to
// show them in, and is also what gives each team a stable colour without
// naming any of them in committed source.
type ScheduleTeam struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Family    string `json:"family"`
	SortOrder int    `json:"sortOrder"`
	// RotaCode is the rota this team's type belongs to; absent for a team on
	// no named rota (CRE's, and Americas).
	RotaCode *string `json:"rotaCode,omitempty"`
	// Members are everyone on the team, with their role, so a roster shows
	// each of them on a month they hold no window -- a lead is rarely on the
	// rota, and somebody whose only entry was an allocation vanished from a
	// grid built from entries the moment it was cleared, taking with them the
	// row a lead marks their leave on.
	Members []ScheduleTeamMember `json:"members"`
	// DefaultShiftCode is the standing window a member's ordinary weekday is:
	// Americas cover for the Americas team. Absent means Regular hours.
	DefaultShiftCode *string `json:"defaultShiftCode,omitempty"`
}

// ScheduleTeamMember is one member of a rota team. Role is the team_member
// role as stored: engineer (or member), lead, americas_team_lead, and so on.
type ScheduleTeamMember struct {
	ScheduleEngineer
	Role string `json:"role"`
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
// CreateScheduleAssignmentRequest puts somebody on a window for a day.
//
// StartsAt and EndsAt are deliberately not accepted from the caller. They are
// resolved from the shift's own window and authoring zone, so a hand-placed
// cover cannot drift from the window it claims to be.
type CreateScheduleAssignmentRequest struct {
	UserID    string  `json:"userId"`
	TeamKey   string  `json:"teamKey"`
	ShiftCode string  `json:"shiftCode"`
	RotaDate  string  `json:"rotaDate"`
	Tier      *string `json:"tier,omitempty"`
	IsOnCall  *bool   `json:"isOnCall,omitempty"`
	Note      *string `json:"note,omitempty"`
}

// UpdateScheduleAssignmentRequest changes who holds a slot, or its detail.
// Every field is optional; a nil field is left as it was.
type UpdateScheduleAssignmentRequest struct {
	UserID   *string `json:"userId,omitempty"`
	Tier     *string `json:"tier,omitempty"`
	IsOnCall *bool   `json:"isOnCall,omitempty"`
	Note     *string `json:"note,omitempty"`
}

// ApplyScheduleRangeRequest sets one engineer to one window across a span of
// days -- the shape the roster's cell picker works in.
//
// A day the shift is not valid on is skipped rather than refused: picking a
// weekday rotation across a week that contains a Saturday should set the five
// weekdays, not fail because of the Saturday.
type ApplyScheduleRangeRequest struct {
	UserID  string `json:"userId"`
	TeamKey string `json:"teamKey"`
	// ShiftCode empty means take them off the rota over the span -- the
	// picker's own clear option.
	ShiftCode string  `json:"shiftCode"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Note      *string `json:"note,omitempty"`
	// Tier is L1, L2 or L3, for an escalation window that leaves the tier to
	// the person (SRE_TZ1, SRE_TZ3, ...). Absent takes the window's own tier.
	// A window that fixes a tier accepts only that one.
	Tier *string `json:"tier,omitempty"`
	// ZoneCode narrows a clear (ShiftCode empty) to that zone's turn, leaving
	// the rest of the person's day. Only meaningful when clearing.
	ZoneCode *string `json:"zoneCode,omitempty"`
}

// ApplyScheduleRangeResponse says what actually happened, because it is
// routinely less than what was asked for.
type ApplyScheduleRangeResponse struct {
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
	// SkippedDates are the days the window is not worked on, so the caller can
	// say which rather than only how many.
	SkippedDates []string `json:"skippedDates"`
}

// ApplyScheduleAbsenceRequest marks one engineer away across a span, or --
// with an empty KindCode -- brings them back over it.
//
// Unlike a rota window, this is not resolved day by day. An absence is a span
// in the model and stays one here: somebody away from Wednesday to the
// following Tuesday is one fact, and storing it as five weekday rows would
// lose the weekend in the middle that they are also away for.
type ApplyScheduleAbsenceRequest struct {
	UserID  string `json:"userId"`
	TeamKey string `json:"teamKey"`
	// KindCode empty clears the span instead of marking it.
	KindCode string  `json:"kindCode"`
	From     string  `json:"from"`
	To       string  `json:"to"`
	Note     *string `json:"note,omitempty"`
	// AllocatedTo is who an allocation is for: the customer, or the product
	// team for RnD. Kept only when KindCode is an ALLOCATION kind -- leave is
	// not "for" anybody -- and dropped when blank.
	AllocatedTo *string `json:"allocatedTo,omitempty"`

	// HomeTeamKey is the team the person belongs to, set by the service --
	// never read from the request body. A kind that moves people to another
	// team files the span under that team and keeps this beside it, so the
	// home team's lead still owns the span.
	HomeTeamKey string `json:"-"`
}

// CreateScheduleAbsenceKindRequest is a lead adding a kind of time away the
// catalogue does not have yet. The kind is shared: once created it is offered
// to every team. The code is derived from the label, not taken from the
// caller, so two leads naming the same thing arrive at the same code.
type CreateScheduleAbsenceKindRequest struct {
	ShortCode   string `json:"shortCode"`
	Label       string `json:"label"`
	Bucket      string `json:"bucket"`
	ColourToken string `json:"colourToken"`
}

// ApplyScheduleAbsenceResponse says what the span did to what was already
// there, which is rarely just "one row added".
type ApplyScheduleAbsenceResponse struct {
	// Created is 1 when a span was marked, 0 when it was cleared.
	Created int `json:"created"`
	// Removed is absences that fell entirely inside the span and went.
	Removed int `json:"removed"`
	// Trimmed is absences that overlapped one end and were shortened rather
	// than deleted -- a fortnight of leave with three days cleared out of the
	// middle is still a fortnight of leave either side.
	Trimmed int `json:"trimmed"`
}

// ScheduleEditMarker says that a cell on the roster was changed by a person,
// and by whom.
//
// Deliberately not the change itself. The roster is a hundred-odd engineers
// wide by three months, and shipping every field that moved for every cell
// would cost far more than the marks are worth -- the page only needs to know
// which cells to mark and what to say when one is pointed at. The full history
// of a cell is a second request, made when somebody asks for it.
type ScheduleEditMarker struct {
	UserID    string    `json:"userId"`
	RotaDate  string    `json:"rotaDate"`
	Actor     string    `json:"actor"`
	ChangedAt time.Time `json:"changedAt"`
	Action    string    `json:"action"`
}

// ScheduleEditMarkersResponse is every marked cell in a window.
type ScheduleEditMarkersResponse struct {
	Markers []ScheduleEditMarker `json:"markers"`
	Count   int                  `json:"count"`
}

// ScheduleAssignmentActivity is one recorded change to the rota -- a window
// set or cleared, or a span of leave marked or cleared.
type ScheduleAssignmentActivity struct {
	ID           string `json:"id"`
	AssignmentID string `json:"assignmentId"`
	UserID       string `json:"userId"`
	TeamKey      string `json:"teamKey"`
	RotaDate     string `json:"rotaDate"`
	// Subject is "rota" for a change to a window and "leave" for a change to an
	// absence. For leave, ShiftCode carries the absence kind's code, RotaDate
	// the first day and EndsOn the last (absent for an open-ended span).
	Subject    string    `json:"subject"`
	EndsOn     *string   `json:"endsOn,omitempty"`
	ShiftCode  string    `json:"shiftCode"`
	Action     string    `json:"action"`
	FieldName  *string   `json:"fieldName,omitempty"`
	OldValue   *string   `json:"oldValue,omitempty"`
	NewValue   *string   `json:"newValue,omitempty"`
	ActorEmail string    `json:"actorEmail"`
	Note       *string   `json:"note,omitempty"`
	CreatedOn  time.Time `json:"createdOn"`
}

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
	// AllocatedTo is who an allocation is for: the customer, for a customer
	// allocation; the product team, for RnD. The kind says what sort of time it
	// is, this says for whom -- so a new customer is a value, not a new kind.
	AllocatedTo *string `json:"allocatedTo,omitempty"`
	// HomeTeamKey is the person's own team, on a span filed under the team a
	// kind moved them to (TeamKey); absent on every other span.
	HomeTeamKey *string `json:"homeTeamKey,omitempty"`
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
