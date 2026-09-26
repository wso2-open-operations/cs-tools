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

import (
	"fmt"
	"time"
)

// Query-hour consumption states, mirroring ServiceNow's u_query_hour_state
// so a cutover comparison is a straight equality check.
const (
	QueryHourStateNormal   = 0 // under 75% consumed
	QueryHourStateWarning  = 1 // >= 75%
	QueryHourStateCritical = 2 // >= 90%
	QueryHourStateExceeded = 3 // >= 100%
)

// QueryHourThresholds are the percent-consumed boundaries, highest first so a
// linear scan returns the most severe match. Values come from ServiceNow's
// `Set Project Query Hour State` business rule.
var QueryHourThresholds = []struct {
	MinPercent float64
	State      int
}{
	{100, QueryHourStateExceeded},
	{90, QueryHourStateCritical},
	{75, QueryHourStateWarning},
}

// ProjectQueryHours is one project's computed query-hour position.
//
// Every duration is minutes, not an INTERVAL or ServiceNow's "100h 0m"
// string. ServiceNow carried these as strings and grew two rival parsers for
// them (QueryHourUtils.getInMinuites and SN_Utils.convertToMinutes) that
// disagree on a value with no minutes part and on a negative value; the
// Postgres columns are typed, so neither parser is ported.
type ProjectQueryHours struct {
	ProjectID string `json:"projectId"`
	// ProjectKey and ProjectSFID are read-through from the synced project row
	// for callers' convenience; neither is stored on project_query_hours.
	ProjectKey  string `json:"projectKey,omitempty"`
	ProjectSFID string `json:"projectSfId,omitempty"`

	EntitlementMinutes int `json:"entitlementMinutes"`
	ConsumedMinutes    int `json:"consumedMinutes"`
	BillableMinutes    int `json:"billableMinutes"`
	NonBillableMinutes int `json:"nonBillableMinutes"`

	// RemainingMinutes may be negative — a project can overrun its
	// entitlement, and ServiceNow records that rather than clamping.
	RemainingMinutes int `json:"remainingMinutes"`
	// PercentConsumed is 0 when there is no entitlement to divide by.
	PercentConsumed float64 `json:"percentConsumed"`
	QueryHourState  int     `json:"queryHourState"`

	// EntitlementSource is "opportunity_lines" or "servicenow_synced".
	EntitlementSource string `json:"entitlementSource,omitempty"`
	// SyncedEntitlementMinutes is ServiceNow's own figure, for comparison.
	SyncedEntitlementMinutes int `json:"syncedEntitlementMinutes,omitempty"`
	// UnmatchedLineCount > 0 means some active product line carried a name the
	// entitlement rule does not know and contributed nothing.
	UnmatchedLineCount int `json:"unmatchedLineCount,omitempty"`

	LastPushedState *int       `json:"lastPushedState,omitempty"`
	LastPushedAt    *time.Time `json:"lastPushedAt,omitempty"`
	ComputedAt      time.Time  `json:"computedAt"`
}

// Where a project's entitlement figure came from. Reported so the parallel
// run before cutover can tell a derived number from ServiceNow's own.
const (
	// EntitlementSourceOpportunityLines means it was computed here, from the
	// Salesforce opportunity product lines funding the project.
	EntitlementSourceOpportunityLines = "opportunity_lines"
	// EntitlementSourceServiceNow means it was read from
	// project.total_query_duration, which csm-sync-service mirrors from
	// ServiceNow. Used when no opportunity line covers today, or when the
	// opportunity tables are not present yet.
	EntitlementSourceServiceNow = "servicenow_synced"
)

// QueryHourConsumption is the raw aggregate the repository returns for one
// project, before thresholds are applied.
// NOTE ON THE NAME: this was ProjectConsumption until dev-app-csm-portal grew
// a type of that name for something entirely unrelated — a project's Choreo
// provisioning state (application id, consumer key/secret). Two unrelated
// meanings cannot share one name in one package, and the provisioning type was
// there first on the target branch, so the query-hour one is the one that
// moved. The name is also simply more accurate: this is a query-hour figure,
// not a general notion of what a project consumes.
type QueryHourConsumption struct {
	ProjectID   string
	ProjectKey  string
	ProjectSFID string

	// EntitlementMinutes is the figure actually used, from EntitlementSource.
	EntitlementMinutes int
	EntitlementSource  string
	// SyncedEntitlementMinutes is always ServiceNow's own figure, kept even
	// when the derived one is used so the two can be compared at cutover.
	SyncedEntitlementMinutes int

	BillableMinutes    int
	NonBillableMinutes int

	// ActiveLineCount is how many opportunity product lines cover today.
	ActiveLineCount int
	// UnmatchedLineCount is how many of those carry a product name outside the
	// six the rule knows, and therefore contributed ZERO hours. A non-zero
	// value here is the signal that a pack has been renamed or added and the
	// entitlement is quietly too low.
	UnmatchedLineCount int
	// DevSupportHours is the sum of the mirrored development_support_hours
	// column, which ServiceNow ignores. Reported for comparison only; see
	// entitlementHoursSQL.
	DevSupportHours float64
}

// ConsumedMinutes is billable plus non-billable. ServiceNow summed
// time_card.total and split it by u_is_billable; the Postgres time_card has
// no `total` column, so the split is summed from the five per-activity
// minute columns instead and the total is their sum.
func (c QueryHourConsumption) ConsumedMinutes() int {
	return c.BillableMinutes + c.NonBillableMinutes
}

// RecomputeQueryHoursRequest asks for one project's position to be recomputed.
type RecomputeQueryHoursRequest struct {
	ProjectID string `json:"projectId"`
}

// RecomputeQueryHoursResponse reports the result of a recompute, including
// whether the outbound Choreo push was attempted and whether it succeeded.
type RecomputeQueryHoursResponse struct {
	ProjectQueryHours
	// StateChanged is true when this recompute moved query_hour_state.
	StateChanged bool `json:"stateChanged"`
	// Pushed is true when the Choreo subscription-closure call succeeded on
	// this recompute. False covers "not needed", "disabled" and "failed" —
	// PushError distinguishes the last.
	Pushed    bool   `json:"pushed"`
	PushError string `json:"pushError,omitempty"`
}

// RecomputeQueryHoursBatchResponse reports a sweep over many projects.
type RecomputeQueryHoursBatchResponse struct {
	Requested int                           `json:"requested"`
	Succeeded int                           `json:"succeeded"`
	Failed    int                           `json:"failed"`
	Results   []RecomputeQueryHoursResponse `json:"results"`
	// Errors carries one entry per failed project, keyed by project id, so a
	// partial sweep still reports which projects were missed and why.
	Errors map[string]string `json:"errors,omitempty"`
}

// SubscriptionClosureUpdate is the payload pushed to Choreo Sales Operations
// when a project's consumption moves. Field names match the JSON ServiceNow's
// `Consumed Query Hour Update` business rule sends, so the receiving Choreo
// service needs no change at cutover.
type SubscriptionClosureUpdate struct {
	ConsumedQueryTime int `json:"consumedQueryTime"`
	TotalQueryTime    int `json:"totalQueryTime"`
}

// QueryHourNotificationContext is the human-readable context the threshold
// email needs, resolved from the project's account.
type QueryHourNotificationContext struct {
	AccountName         string
	ProjectName         string
	AccountManagerEmail string
	TechnicalOwnerEmail string
	// AccountManagerName is the greeting's subject. Empty when the account has
	// no owner on file, in which case the email falls back to a generic
	// salutation rather than omitting it.
	AccountManagerName string
}

// FormatHoursMinutes renders a minute count as ServiceNow rendered it —
// "100h 0m" — because that is the form the recipients of this email have been
// reading for years.
//
// Negative input (an overrun's remaining time) keeps its sign on the hours and
// reports the minutes as a magnitude: -90 becomes "-1h 30m", not "-1h -30m".
// ServiceNow stored these as strings and had two rival parsers that disagreed
// on exactly this case; here the number is authoritative and the string is
// only a rendering, so the ambiguity cannot propagate.
func FormatHoursMinutes(minutes int) string {
	sign := ""
	if minutes < 0 {
		sign = "-"
		minutes = -minutes
	}
	return fmt.Sprintf("%s%dh %dm", sign, minutes/60, minutes%60)
}
