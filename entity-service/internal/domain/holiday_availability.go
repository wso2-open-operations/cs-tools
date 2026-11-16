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

// HolidayTagKindCode is the Team Schedule tag placed on a CRE member for a
// public holiday of their country (migration 0283).
const HolidayTagKindCode = "PH"

// HolidayTagActor is who the Team Schedule records as placing (and, when a
// holiday is cancelled, removing) a public holiday tag.
const HolidayTagActor = "team-schedule-holidays"

// Holiday availability statuses, per team and holiday.
const (
	// HolidayAvailabilityNeedsMarking: a rota turn the team holds that day
	// has nobody available, and nobody has confirmed.
	HolidayAvailabilityNeedsMarking = "NEEDS_MARKING"
	// HolidayAvailabilityAwaitingConfirmation: every turn has somebody
	// available; the lead (or a rota manager) has not confirmed yet.
	HolidayAvailabilityAwaitingConfirmation = "AWAITING_CONFIRMATION"
	// HolidayAvailabilityConfirmed: every turn covered and confirmed: fully
	// marked, no more reminders.
	HolidayAvailabilityConfirmed = "CONFIRMED"
	// HolidayAvailabilityNeedsAttention: confirmed, but a turn has since
	// lost its person (re-tagged, a swap undone). Reminders resume.
	HolidayAvailabilityNeedsAttention = "NEEDS_ATTENTION"
)

// Member statuses on a holiday.
const (
	// HolidayMemberTagged: the Public holiday tag is on them; they are off.
	HolidayMemberTagged = "TAGGED"
	// HolidayMemberAvailable: the tag was removed; they work that day.
	HolidayMemberAvailable = "AVAILABLE"
	// HolidayMemberAway: another tag (leave, say) is on them that day.
	HolidayMemberAway = "AWAY"
)

// Holiday availability reminder stages, in the order they fall due.
const (
	HolidayReminderNotice  = "notice"   // 14 days before, always
	HolidayReminderWeek    = "week"     // 7 days before, while not fully marked
	HolidayReminderTwoDays = "two_days" // 2 working days before, ditto
	HolidayReminderOneDay  = "one_day"  // 1 working day before, ditto
)

// HolidayTagPlacement is one tag the daily run wants to place.
type HolidayTagPlacement struct {
	UserID  string
	TeamKey string
	Date    string
	Country string
	Title   string
}

// HolidayTagPlacementResult reports one placement run.
type HolidayTagPlacementResult struct {
	Placed int `json:"placed"`
	// Skipped: the person already had a tag that day (leave wins), or a
	// tag was placed for them before.
	Skipped int `json:"skipped"`
	// Removed: tags of holidays the calendar no longer has, still untouched.
	Removed int `json:"removed"`
}

// SearchHolidayAvailabilityRequest is the body of
// POST /team-schedule/holiday-availability/search.
type SearchHolidayAvailabilityRequest struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	TeamKeys []string `json:"teamKeys,omitempty"`
}

// HolidayAvailabilityMember is one member a holiday tag was placed for.
type HolidayAvailabilityMember struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email,omitempty"`
	// Status is TAGGED, AVAILABLE or AWAY.
	Status string `json:"status"`
	// AbsenceID is the Public holiday tag while it is on them (TAGGED).
	AbsenceID string `json:"absenceId,omitempty"`
	// AwayKind is the other tag's code when AWAY.
	AwayKind string `json:"awayKind,omitempty"`
}

// HolidayAvailabilityTurn is a rota turn of the team that day with nobody
// available on it.
type HolidayAvailabilityTurn struct {
	ShiftCode  string `json:"shiftCode"`
	ShiftLabel string `json:"shiftLabel"`
	UserID     string `json:"userId"`
	Name       string `json:"name"`
}

// HolidayAvailability is one team's availability on one holiday.
type HolidayAvailability struct {
	TeamKey   string   `json:"teamKey"`
	TeamName  string   `json:"teamName"`
	Date      string   `json:"date"`
	Titles    []string `json:"titles"`
	Countries []string `json:"countries"`
	Status    string   `json:"status"`
	// FullyMarked is Status == CONFIRMED: the reminders stop.
	FullyMarked bool                        `json:"fullyMarked"`
	Members     []HolidayAvailabilityMember `json:"members"`
	Turns       int                         `json:"turns"`
	// Uncovered is every turn whose person is not available.
	Uncovered   []HolidayAvailabilityTurn `json:"uncovered"`
	ConfirmedBy string                    `json:"confirmedBy,omitempty"`
	ConfirmedOn *time.Time                `json:"confirmedOn,omitempty"`
	// CanConfirm is whether the caller may confirm or withdraw it.
	CanConfirm bool `json:"canConfirm"`
}

// HolidayAvailabilityResponse answers the search.
type HolidayAvailabilityResponse struct {
	Holidays []HolidayAvailability `json:"holidays"`
	Count    int                   `json:"count"`
}

// HolidayReminderRecipient is somebody a team's holiday reminder goes to.
type HolidayReminderRecipient struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	// Role is "lead" or "rota_manager".
	Role string `json:"role"`
}

// DueHolidayReminder is one reminder the daily task should send.
type DueHolidayReminder struct {
	Stage        string                     `json:"stage"`
	DaysBefore   int                        `json:"daysBefore"`
	Availability HolidayAvailability        `json:"availability"`
	Recipients   []HolidayReminderRecipient `json:"recipients"`
}

// DueHolidayRemindersRequest is the body of
// POST /team-schedule/holiday-availability/reminders/due.
type DueHolidayRemindersRequest struct {
	// Today is the day the reminders are for (YYYY-MM-DD, the rota's own
	// calendar), so a retry the next morning still sends that morning's.
	Today string `json:"today"`
}

// DueHolidayRemindersResponse lists the reminders due.
type DueHolidayRemindersResponse struct {
	Reminders []DueHolidayReminder `json:"reminders"`
}

// RecordHolidayReminderRequest is the body of
// PUT /team-schedule/holiday-availability/reminders/{teamKey}/{date}/{stage}.
type RecordHolidayReminderRequest struct {
	SentBy string `json:"sentBy,omitempty"`
}

// HolidayTurn is a rota turn as the availability read needs it.
type HolidayTurn struct {
	TeamKey    string
	Date       string
	ShiftCode  string
	ShiftLabel string
	UserID     string
	Name       string
	// Away is whether a tag takes the holder off that day.
	Away bool
}

// HolidayPlacedRow is a placed tag as the availability read sees it now.
type HolidayPlacedRow struct {
	UserID   string
	Name     string
	Email    string
	TeamKey  string
	TeamName string
	Date     string
	Country  string
	Title    string
	// AbsenceID is set while the placed tag still stands.
	AbsenceID string
	// OtherKind is another tag's code on that day, "" when none.
	OtherKind string
}

// HolidayConfirmationRow is a stored confirmation.
type HolidayConfirmationRow struct {
	TeamKey     string
	Date        string
	ConfirmedBy string
	ConfirmedOn time.Time
}
