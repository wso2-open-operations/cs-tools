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

// Query-hour utilisation: how much of an account's purchased query-support
// time has been used, and how close it is to running out.
//
// Ported from ServiceNow's QueryHourUtils. The entitlement comes from
// Salesforce opportunity product lines, the consumption from approved
// time cards, and the two produce a state that drives both a weekly
// report and a per-project notification.
//
// NOTHING HERE ADDS A COLUMN. The state enum, the opportunity state
// column and the three project duration columns all already exist; this
// computes what goes in them.

// QueryHourState is the escalation ladder, matching the existing
// query_hour_state_enum and ServiceNow's 1/2/3. Below 75% there is no
// state at all -- ServiceNow clears the field rather than storing a
// fourth value, and the column is nullable for the same reason.
type QueryHourState string

const (
	QueryHourStateNotified75 QueryHourState = "NOTIFIED_75"
	QueryHourStateNotified90 QueryHourState = "NOTIFIED_90"
	QueryHourStateClosure100 QueryHourState = "CLOSURE_NOTICE_100"
)

// QueryHourStateFor returns the state a usage percentage earns, and
// whether there is one at all. The thresholds are ServiceNow's:
//
//	usage >= 100  ->  CLOSURE_NOTICE_100
//	usage >=  90  ->  NOTIFIED_90
//	usage >=  75  ->  NOTIFIED_75
//	below         ->  no state
func QueryHourStateFor(usagePercent float64) (QueryHourState, bool) {
	switch {
	case usagePercent >= 100:
		return QueryHourStateClosure100, true
	case usagePercent >= 90:
		return QueryHourStateNotified90, true
	case usagePercent >= 75:
		return QueryHourStateNotified75, true
	default:
		return "", false
	}
}

// QueryHourCloseToExceedMinutes is how little may remain before an
// opportunity counts as close to exceeding: ServiceNow's
// `remaining_query_hour < 600`, in minutes, so ten hours.
const QueryHourCloseToExceedMinutes = 600

// QueryHourProductHours maps a Salesforce product line to the hours one
// unit of it grants. A line whose product is absent grants nothing and
// is counted as unmatched -- worth surfacing, because a renamed SKU
// would otherwise silently reduce an entitlement to zero rather than
// failing.
var QueryHourProductHours = map[string]int{
	"Development Support - 200 hours":                200,
	"Development Support - 100 hours":                100,
	"Development Support - 50 hours":                 50,
	"Development Support - 25 hours":                 25,
	"Development Support - 10 hours":                 10,
	"Query support limit (included in subscription)": 1,
}

// QueryHourProject is one project's slice of an opportunity's allowance.
type QueryHourProject struct {
	ProjectID    string `json:"projectId"`
	ProjectKey   string `json:"projectKey"`
	ProjectName  string `json:"projectName"`
	SalesforceID string `json:"salesforceId,omitempty"`
	// ConsumedMinutes is BILLABLE approved time only. ServiceNow adds
	// `summary.total_billable` to the running total and never the
	// non-billable figure, which it computes purely to display.
	ConsumedMinutes int `json:"consumedMinutes"`
	// NonBillableMinutes is carried for the report, not for the sum.
	NonBillableMinutes int `json:"nonBillableMinutes"`
	// OnboardingMinutes is deducted from consumption -- onboarding time
	// is not query support, and ServiceNow subtracts it explicitly.
	OnboardingMinutes int `json:"onboardingMinutes"`
}

// QueryHourOpportunity is one opportunity's allowance and what has been
// drawn against it.
type QueryHourOpportunity struct {
	OpportunityID   string `json:"opportunityId"`
	SalesforceID    string `json:"salesforceId"`
	Name            string `json:"name"`
	AccountID       string `json:"accountId"`
	AccountName     string `json:"accountName"`
	TotalMinutes    int    `json:"totalMinutes"`
	ConsumedMinutes int    `json:"consumedMinutes"`
	// RemainingMinutes goes negative once the allowance is overdrawn,
	// which is exactly how "exceeded" is decided.
	RemainingMinutes int             `json:"remainingMinutes"`
	UsagePercent     float64         `json:"usagePercent"`
	State            *QueryHourState `json:"state"`
	IsExceeded       bool            `json:"isExceeded"`
	IsCloseToExceed  bool            `json:"isCloseToExceed"`
	// UnmatchedLines counts active product lines whose name is not in
	// QueryHourProductHours -- a renamed SKU, most likely.
	UnmatchedLines int `json:"unmatchedLines"`
	// ServiceStart and ServiceEnd bound the window consumption is
	// counted over: the earliest start and latest end across the
	// opportunity's active lines.
	ServiceStart *time.Time         `json:"serviceStart,omitempty"`
	ServiceEnd   *time.Time         `json:"serviceEnd,omitempty"`
	Projects     []QueryHourProject `json:"projects"`
}

// QueryHourAccount groups an account's opportunities, and is the unit the
// weekly report lists and the unit whose owners it mails.
type QueryHourAccount struct {
	AccountID    string `json:"accountId"`
	AccountName  string `json:"accountName"`
	SalesforceID string `json:"salesforceId,omitempty"`
	// AccountManagerEmail and TechnicalOwnerEmail are who ServiceNow
	// mails about this account. Both may be empty.
	AccountManagerEmail string                 `json:"accountManagerEmail,omitempty"`
	TechnicalOwnerEmail string                 `json:"technicalOwnerEmail,omitempty"`
	IsExceeded          bool                   `json:"isExceeded"`
	IsCloseToExceed     bool                   `json:"isCloseToExceed"`
	Opportunities       []QueryHourOpportunity `json:"opportunities"`
}

// QueryHourReport is the weekly consumption report.
type QueryHourReport struct {
	GeneratedOn string `json:"generatedOn"`
	// Exceeded and CloseToExceed are the report's two tables, in that
	// order, and together they decide the audience: ServiceNow mails the
	// owners of every account in EITHER.
	Exceeded      []QueryHourAccount `json:"exceeded"`
	CloseToExceed []QueryHourAccount `json:"closeToExceed"`
	// Recipients are those owners, deduped and restricted to internal
	// addresses. Unlike the CS Worklogs reports this audience IS
	// derivable, because it is a property of the accounts in the report.
	Recipients []string `json:"recipients"`
}

// QueryHourTransition is a project crossing a threshold, which is what
// the per-project usage notification reacts to.
type QueryHourTransition struct {
	ProjectID   string          `json:"projectId"`
	ProjectKey  string          `json:"projectKey"`
	ProjectName string          `json:"projectName"`
	AccountID   string          `json:"accountId"`
	AccountName string          `json:"accountName"`
	From        *QueryHourState `json:"from"`
	To          QueryHourState  `json:"to"`
	// The three figures the notification prints, as text in the same
	// "Xh Ym" shape ServiceNow renders.
	TotalText           string `json:"totalText"`
	ConsumedText        string `json:"consumedText"`
	RemainingText       string `json:"remainingText"`
	AccountManagerEmail string `json:"accountManagerEmail,omitempty"`
	AccountManagerName  string `json:"accountManagerName,omitempty"`
	TechnicalOwnerEmail string `json:"technicalOwnerEmail,omitempty"`
}
