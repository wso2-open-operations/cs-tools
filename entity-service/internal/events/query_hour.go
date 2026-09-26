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

package events

// TypeQueryHourThresholdReached fires when a project's query-hour consumption
// crosses 75%, 90% or 100% of its entitlement.
//
// Port of ServiceNow's `[WSO2][Query Hour] Usage Notifications - Project`
// flow (sys_id 3565fd3b3b0787103e1e088aa4e45afc). Three differences from the
// original, all deliberate:
//
//   - SN triggered on EVERY update of customer_project with no condition at
//     all, then decided inside a script whether there was anything to say.
//     This fires only on an actual upward crossing.
//   - SN's script read u_query_hour_state, which its own business rule only
//     ever RAISED. Because this port recomputes downwards too, a project can
//     re-cross a threshold after dropping below it, and will notify again.
//     That is intended: the second crossing is real news.
//   - SN's script never declared `internal_message` (an implicit global) and
//     built the email body regardless of state, so a project at state 0
//     produced an email containing the literal string "undefined". There is
//     no state-0 path here at all.
const TypeQueryHourThresholdReached Type = "project.query_hour_threshold_reached"

// QueryHourThresholdReachedPayload is TypeQueryHourThresholdReached's payload.
//
// The publisher resolves everything: recipients are already looked up,
// filtered to @wso2.com and de-duplicated, and Subject is already rendered.
// csm-notification-service formats and sends, and decides nothing — the same
// division as every other notification in this service.
type QueryHourThresholdReachedPayload struct {
	ProjectID   string `json:"projectId"`
	ProjectKey  string `json:"projectKey,omitempty"`
	ProjectName string `json:"projectName,omitempty"`
	AccountName string `json:"accountName,omitempty"`

	// State is 1 (>=75%), 2 (>=90%) or 3 (>=100%), matching ServiceNow's
	// u_query_hour_state values so a cutover comparison is a straight equality
	// check. State 0 is never published.
	State int `json:"state"`
	// PreviousState is what it was before this crossing; -1 when the project
	// had no recorded position yet.
	PreviousState int `json:"previousState"`

	// Durations are pre-formatted as "100h 0m" — the form the ServiceNow email
	// showed, and the form the recipients already recognise. The raw minute
	// counts travel alongside so a future consumer need not re-parse them.
	TotalQueryHours string `json:"totalQueryHours"`
	ConsumedHours   string `json:"consumedHours"`
	RemainingHours  string `json:"remainingHours"`

	EntitlementMinutes int     `json:"entitlementMinutes"`
	ConsumedMinutes    int     `json:"consumedMinutes"`
	RemainingMinutes   int     `json:"remainingMinutes"`
	PercentConsumed    float64 `json:"percentConsumed"`

	// Subject is fully rendered by the publisher. ServiceNow's three subjects,
	// verbatim: state 1 and 2 name the percentage, state 3 names the account.
	Subject string `json:"subject"`

	// OwnerName is the account manager's display name, used for the greeting.
	// ServiceNow read it from account.u_owner and addressed the mail "Hi
	// <name>,"; without it the email opens "Hi Account Manager," which is
	// how the first cut of this port read.
	OwnerName string `json:"ownerName,omitempty"`

	// Recipients is the account manager and technical owner. CcRecipients is
	// the three standing internal groups, plus one extra address at state 3 —
	// exactly the routing the ServiceNow script performed inline.
	//
	// No customer ever appears in either list. SN's flow had a whole
	// customer-facing branch, and every line of it is commented out; this port
	// does not reinstate it.
	Recipients   []string `json:"recipients"`
	CcRecipients []string `json:"ccRecipients,omitempty"`
}
