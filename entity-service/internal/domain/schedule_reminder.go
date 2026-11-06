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

// ScheduleReminderSent is what csm-scheduled-tasks last sent for one rota
// period: when, how often, and which calendar invites to whom. A run reads it
// to tell "already sent" from "not yet", and to cancel an invite for somebody
// taken off the rota since.
type ScheduleReminderSent struct {
	Family string `json:"family"`
	// Kind is the reminder, e.g. "weekend_notice".
	Kind string `json:"kind"`
	// PeriodStart is the first rota day the reminder covers (YYYY-MM-DD): a
	// weekend's Saturday.
	PeriodStart string                   `json:"periodStart"`
	FirstSentOn time.Time                `json:"firstSentOn"`
	LastSentOn  time.Time                `json:"lastSentOn"`
	SendCount   int                      `json:"sendCount"`
	Invites     []ScheduleReminderInvite `json:"invites"`
	// SentBy is who sent it: the task's name, or a person's address.
	SentBy string `json:"sentBy"`
}

// ScheduleReminderInvite is one calendar event a reminder sent.
type ScheduleReminderInvite struct {
	UID       string   `json:"uid"`
	Sequence  int      `json:"sequence"`
	Attendees []string `json:"attendees"`
}

// RecordScheduleReminderRequest is the body of PUT
// /team-schedule/reminders/{family}/{kind}/{periodStart}: the invites just
// sent, which replace the stored ones.
type RecordScheduleReminderRequest struct {
	Invites []ScheduleReminderInvite `json:"invites"`
	SentBy  string                   `json:"sentBy"`
}

// RotaAdminContact is one rota admin of a family, to be emailed.
type RotaAdminContact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ReminderWindow is a rotation's hours on its own clock: "08:00" to "18:00"
// in America/Los_Angeles. An end not after the start is on the next day.
type ReminderWindow struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	TimeZone string `json:"timeZone"`
}

// RotaReminderSettings is a family's rota reminder settings
// (team_schedule_reminder_setting), saved on the Rota Access tab.
type RotaReminderSettings struct {
	Family          string                    `json:"family"`
	WeekdayEnabled  bool                      `json:"weekdayEnabled"`
	WeekendEnabled  bool                      `json:"weekendEnabled"`
	InvitesEnabled  bool                      `json:"invitesEnabled"`
	WeekendRequired map[string]int            `json:"weekendRequired"`
	Windows         map[string]ReminderWindow `json:"windows"`
	// InviteRotations is, per rotation (the shift code of its window), whether
	// its reminders carry a calendar invitation. A rotation not named does.
	// InvitesEnabled off turns every rotation's off.
	InviteRotations map[string]bool `json:"inviteRotations"`
	CopyTo          []string        `json:"copyTo"`
	// WeekendCopyTo is copied (Cc) on the weekend summary email only, beside
	// CopyTo, which is copied on every reminder.
	WeekendCopyTo []string  `json:"weekendCopyTo"`
	UpdatedBy     string    `json:"updatedBy"`
	UpdatedOn     time.Time `json:"updatedOn"`
}

// SaveRotaReminderSettingsRequest replaces a family's settings. Every field
// is required, the card always sending the whole form, except
// InviteRotations and WeekendCopyTo: a card from before they existed sends
// neither, and its save keeps what is stored rather than turning every
// rotation's invitations back on or dropping the weekend copies.
type SaveRotaReminderSettingsRequest struct {
	WeekdayEnabled  *bool                     `json:"weekdayEnabled"`
	WeekendEnabled  *bool                     `json:"weekendEnabled"`
	InvitesEnabled  *bool                     `json:"invitesEnabled"`
	WeekendRequired map[string]int            `json:"weekendRequired"`
	Windows         map[string]ReminderWindow `json:"windows"`
	InviteRotations map[string]bool           `json:"inviteRotations"`
	CopyTo          []string                  `json:"copyTo"`
	WeekendCopyTo   []string                  `json:"weekendCopyTo"`
}

// RotaReminderRequest is a rota reminder asked for by hand ("Send now"),
// queued for csm-scheduled-tasks (team_schedule_reminder_request).
type RotaReminderRequest struct {
	ID          string     `json:"id"`
	Family      string     `json:"family"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	Test        bool       `json:"test"`
	RequestedBy string     `json:"requestedBy"`
	RequestedOn time.Time  `json:"requestedOn"`
	ClaimedOn   *time.Time `json:"claimedOn,omitempty"`
	HandledOn   *time.Time `json:"handledOn,omitempty"`
	Result      string     `json:"result,omitempty"`
}

// CreateRotaReminderRequest asks for a rota reminder to be sent now.
type CreateRotaReminderRequest struct {
	Kind string `json:"kind"`
	// Test sends it only to the person asking, as a test copy.
	Test bool `json:"test"`
}

// ReportRotaReminderRequest is how a claimed request went: SENT or FAILED,
// with a short result for the card.
type ReportRotaReminderRequest struct {
	Status string `json:"status"`
	Result string `json:"result"`
}
