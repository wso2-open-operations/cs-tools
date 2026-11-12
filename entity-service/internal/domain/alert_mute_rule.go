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

// AlertMuteRuleState is an alert mute rule's state, derived from its window and whether it was ended early; never stored.
type AlertMuteRuleState string

const (
	// AlertMuteRuleStateScheduled: the window has not started.
	AlertMuteRuleStateScheduled AlertMuteRuleState = "scheduled"
	// AlertMuteRuleStateActive: matching alerts are muted now.
	AlertMuteRuleStateActive AlertMuteRuleState = "active"
	// AlertMuteRuleStateExpired: the window closed. As a search filter it also takes the rules ended early.
	AlertMuteRuleStateExpired AlertMuteRuleState = "expired"
	// AlertMuteRuleStateEnded: ended early by a person (endedOn is set).
	AlertMuteRuleStateEnded AlertMuteRuleState = "ended"
)

// AlertMuteRuleCondition is one condition of an alert mute rule: the alert's canonical field and the value it must have.
type AlertMuteRuleCondition struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// AlertMuteRule is a one-time window during which SRE alerts matching every condition are muted (migration 0278).
type AlertMuteRule struct {
	ID string `json:"id"`
	// Number is the rule's own number (AMR0000001), given when it is created and never changed.
	Number      string `json:"number"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// TimeZone is the IANA zone the times were entered in; StartOn and EndOn are UTC.
	TimeZone   string                   `json:"timeZone"`
	StartOn    time.Time                `json:"startOn"`
	EndOn      time.Time                `json:"endOn"`
	Conditions []AlertMuteRuleCondition `json:"conditions"`
	CreatedBy  string                   `json:"createdBy"`
	CreatedOn  time.Time                `json:"createdOn"`
	UpdatedBy  string                   `json:"updatedBy"`
	UpdatedOn  time.Time                `json:"updatedOn"`
	EndedBy    *string                  `json:"endedBy"`
	EndedOn    *time.Time               `json:"endedOn"`
}

// StateAt derives the rule's state at now: ended if ended early, else scheduled before startOn, active until endOn, then expired.
func (b AlertMuteRule) StateAt(now time.Time) AlertMuteRuleState {
	switch {
	case b.EndedOn != nil:
		return AlertMuteRuleStateEnded
	case now.Before(b.StartOn):
		return AlertMuteRuleStateScheduled
	case now.Before(b.EndOn):
		return AlertMuteRuleStateActive
	default:
		return AlertMuteRuleStateExpired
	}
}

// SaveAlertMuteRuleRequest is the body of POST /alert-mute-rules: every field of a new rule.
// StartOn and EndOn are RFC 3339 timestamps.
type SaveAlertMuteRuleRequest struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	TimeZone    string                   `json:"timeZone"`
	StartOn     string                   `json:"startOn"`
	EndOn       string                   `json:"endOn"`
	Conditions  []AlertMuteRuleCondition `json:"conditions"`
}

// PatchAlertMuteRuleRequest is the body of PATCH /alert-mute-rules/{id}: only the fields present change, the rest keep
// the stored rule's values (like PatchOutageRequest). Conditions, when present, replace the whole list. The merged rule
// is validated as a create would be.
type PatchAlertMuteRuleRequest struct {
	Name        *string                   `json:"name"`
	Description *string                   `json:"description"`
	TimeZone    *string                   `json:"timeZone"`
	StartOn     *string                   `json:"startOn"`
	EndOn       *string                   `json:"endOn"`
	Conditions  *[]AlertMuteRuleCondition `json:"conditions"`
}

// AlertMuteRuleWrite is a validated SaveAlertMuteRuleRequest, as the repository stores it.
type AlertMuteRuleWrite struct {
	Name        string
	Description string
	TimeZone    string
	StartOn     time.Time
	EndOn       time.Time
	Conditions  []AlertMuteRuleCondition
}

// ExtendAlertMuteRuleRequest is the body of POST /alert-mute-rules/{id}/extend.
type ExtendAlertMuteRuleRequest struct {
	EndOn string `json:"endOn"`
}

// SearchAlertMuteRulesFilters are the filters of POST /alert-mute-rules/search; none set means every rule.
type SearchAlertMuteRulesFilters struct {
	States      []AlertMuteRuleState `json:"states"`
	SearchQuery string               `json:"searchQuery"`
}

// SearchAlertMuteRulesRequest is the body of POST /alert-mute-rules/search.
type SearchAlertMuteRulesRequest struct {
	Filters    SearchAlertMuteRulesFilters `json:"filters"`
	Pagination Pagination                  `json:"pagination"`
}

// SearchAlertMuteRulesResponse is a page of alert mute rules, newest created first.
type SearchAlertMuteRulesResponse struct {
	AlertMuteRules []AlertMuteRule `json:"alertMuteRules"`
	Total          int             `json:"total"`
	Offset         int             `json:"offset"`
	Limit          int             `json:"limit"`
}

// ActiveAlertMuteRulesResponse is the body of GET /alert-mute-rules/active: every rule muting alerts now.
type ActiveAlertMuteRulesResponse struct {
	AlertMuteRules []AlertMuteRule `json:"alertMuteRules"`
}
