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

// The CS Worklogs reports: a daily and a weekly roll-up of logged support
// time, grouped by team and by work-item type, mailed to CS leads.
//
// These are NOT a ServiceNow port. Scripts 85-91 established that no
// ServiceNow artefact produces either mail under any name, and nothing in
// cs-tools, digiops-cs or digiops-infra does either. The specification is
// two delivered mails (weekly 2026-10-05..10-11, daily 2026-10-18), whose
// own arithmetic was checked line by line before any of this was written.
//
// Everything is read from tables that already exist -- time_card,
// work_item, "user", team, team_member, team_schedule_absence. No migration
// accompanies this feature.

// WorklogReportCadence distinguishes the two reports. They share a query and
// differ in presentation: the daily one carries a Leave column and a
// disapproved-entry count, the weekly one carries worked days and a
// per-day average.
type WorklogReportCadence string

const (
	WorklogReportCadenceDaily  WorklogReportCadence = "daily"
	WorklogReportCadenceWeekly WorklogReportCadence = "weekly"
)

// WorklogReportRequest asks for one report. From and To are inclusive dates
// in the report's own timezone; for the daily report they are equal.
type WorklogReportRequest struct {
	Cadence WorklogReportCadence `json:"cadence"`
	From    time.Time            `json:"from"`
	To      time.Time            `json:"to"`
}

// WorklogReport is one rendered report.
type WorklogReport struct {
	Cadence WorklogReportCadence `json:"cadence"`
	From    string               `json:"from"`
	To      string               `json:"to"`
	Totals  WorklogTotals        `json:"totals"`
	// ByType is the work-item type breakdown: issue counts and hours for
	// Query (= a CASE of severity S4), Incident (= a CASE of severity
	// S0-S3, or an incident work item), Service Request, Security Report
	// Analysis and Engagement.
	ByType []WorklogTypeBucket `json:"byType"`
	Teams  []WorklogTeamReport `json:"teams"`
	// Recipients is who the data says should receive this report. It is
	// derived, never stored: the To line is every team lead in scope, the
	// Cc line every team's group address. The sending task unions it with
	// whatever its configuration adds, because the delivered weekly mail
	// also reaches five managers who hold no team membership at all and
	// so cannot be derived from anything here.
	Recipients WorklogRecipients `json:"recipients"`
}

// WorklogRecipients is the derived audience for a report.
type WorklogRecipients struct {
	// To is the email of every team_member with role 'lead', in the teams
	// this report covers.
	To []string `json:"to"`
	// Cc is each covered team's group_email (migration 0117), where set.
	Cc []string `json:"cc"`
}

// WorklogTotals is the report's summary block.
//
// TotalHours = ApprovedHours + PendingHours. Rejected time is excluded
// entirely, which the delivered weekly mail confirms: its 31.38 approved
// plus 943.02 pending is exactly its 974.40 total.
type WorklogTotals struct {
	Members       int     `json:"members"`
	Issues        int     `json:"issues"`
	TotalHours    float64 `json:"totalHours"`
	ApprovedHours float64 `json:"approvedHours"`
	PendingHours  float64 `json:"pendingHours"`
	// AvgHoursPerPerson and friends divide by Members. The delivered mail
	// agrees at the top level (974.40/79 = 12.33) but NOT per team -- see
	// WorklogTeamReport.
	AvgHoursPerPerson         float64 `json:"avgHoursPerPerson"`
	AvgApprovedHoursPerPerson float64 `json:"avgApprovedHoursPerPerson"`
	AvgPendingHoursPerPerson  float64 `json:"avgPendingHoursPerPerson"`
	// AveragePeoplePerDay is weekly-only: the mean number of people who
	// logged time on each day in range that anyone logged time at all.
	AveragePeoplePerDay float64 `json:"averagePeoplePerDay,omitempty"`
	// OnLeaveMembers is daily-only. The mail calls its complement
	// "Non Leave Members".
	OnLeaveMembers int `json:"onLeaveMembers"`
	// NonLeaveMembers is fractional: summerizeTimeReport counts a
	// half-day absence as 0.5 of a present person, not as absent.
	NonLeaveMembers float64 `json:"nonLeaveMembers"`
	QueryIssues     int     `json:"queryIssues"`
	IncidentIssues  int     `json:"incidentIssues"`
}

// WorklogTypeBucket is one row of the by-type breakdown.
type WorklogTypeBucket struct {
	// Type is the work_item_type_enum value; Label is what the mail prints
	// for it. They differ for CASE, which the reports call "Query".
	Type          string  `json:"type"`
	Label         string  `json:"label"`
	Issues        int     `json:"issues"`
	TotalHours    float64 `json:"totalHours"`
	ApprovedHours float64 `json:"approvedHours"`
	PendingHours  float64 `json:"pendingHours"`
}

// WorklogTeamReport is one team's section.
type WorklogTeamReport struct {
	TeamKey  string `json:"teamKey"`
	TeamName string `json:"teamName"`
	TeamType string `json:"teamType"`
	Members  int    `json:"members"`
	// Issues is the sum of its developers' issue counts, so an issue two
	// people worked counts twice. The delivered mail does the same: its
	// team totals (Americas 120, Atlas 34, CS Migrations 76, Castor 53)
	// are each exactly the sum of that team's developer rows.
	Issues        int     `json:"issues"`
	ApprovedHours float64 `json:"approvedHours"`
	PendingHours  float64 `json:"pendingHours"`
	TotalHours    float64 `json:"totalHours"`
	// MembersForAverage is the single divisor both averages below use.
	//
	// WEEKLY: the team's rostered members. Leads are not on the roster, so
	// nothing is subtracted. The previous system divides by members less
	// one and prints 0.00 for average approved hours; both are defects and
	// are intentionally not reproduced.
	//
	// DAILY: present members (1 each, 0.5 for a half day, 0 for a full
	// day), with the numerators taken over the same people.
	//
	// A team with no one counting yields 0, never NaN or Infinity.
	MembersForAverage         float64            `json:"membersForAverage"`
	AvgApprovedHoursPerPerson float64            `json:"avgApprovedHoursPerPerson"`
	AvgTotalHoursPerPerson    float64            `json:"avgTotalHoursPerPerson"`
	Developers                []WorklogDeveloper `json:"developers"`
}

// WorklogIssueRef is one work item as a report prints it.
type WorklogIssueRef struct {
	ID string `json:"id"`
	// Key is work_item.wso2_id, falling back to number when unset.
	Key string `json:"key"`
}

// WorklogDeveloper is one person's row inside a team section.
type WorklogDeveloper struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	// WorkedDays is weekly-only, and is NOT derived from time cards: it is
	// the weekdays in range on which the person has a team, meaning not
	// covered by leave (a half day removes the whole day), an exclusion or
	// a confirmed allocation, each day counted once. The mail proves the
	// distinction -- vikt@ shows 5 worked days against 0 issues and 0.00
	// hours, which no time-card-derived count could produce. Public holidays are not modelled
	// anywhere in the schema, so they are not deducted.
	WorkedDays float64 `json:"workedDays"`
	// Leave is daily-only: the absence kind's short code if the person is
	// away that day, otherwise empty.
	Leave          string  `json:"leave,omitempty"`
	ApprovedIssues int     `json:"approvedIssues"`
	ApprovedHours  float64 `json:"approvedHours"`
	PendingIssues  int     `json:"pendingIssues"`
	PendingHours   float64 `json:"pendingHours"`
	// Issues are the work items this person logged time against, in the
	// order the mail prints them (most recently worked first). The id
	// travels with the key so the mail can link each one; ServiceNow
	// linked them into its agent workspace, a URL that stops existing at
	// cutover.
	Issues []WorklogIssueRef `json:"issues"`
	// DisapprovedEntries is daily-only: time cards rejected that day. They
	// contribute no hours anywhere.
	DisapprovedEntries int `json:"disapprovedEntries"`
	// AvgTimePerDay is weekly-only: TotalHours / WorkedDays. Verified
	// against the mail (asha@ 25.08 over 5 days = 5.02).
	AvgTimePerDay float64 `json:"avgTimePerDay"`
	TotalHours    float64 `json:"totalHours"`
}
