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

// Package queryhours is the weekly query-support consumption report and
// the per-project usage notifications, ported from ServiceNow's
// QueryHourUtils and its two flows.
package queryhours

// Report is entity-service's weekly query-hour report.
type Report struct {
	GeneratedOn   string    `json:"generatedOn"`
	Exceeded      []Account `json:"exceeded"`
	CloseToExceed []Account `json:"closeToExceed"`
	// Recipients is the audience the report's own accounts imply: the
	// account manager and technical owner of every account in EITHER
	// table, internal addresses only. Unlike the CS Worklogs reports,
	// this one is genuinely derived -- it is a property of the accounts
	// being reported on, not a hand-kept list.
	Recipients []string `json:"recipients"`
}

// Account is one account's section of the report.
type Account struct {
	AccountID           string        `json:"accountId"`
	AccountName         string        `json:"accountName"`
	SalesforceID        string        `json:"salesforceId"`
	AccountManagerEmail string        `json:"accountManagerEmail"`
	TechnicalOwnerEmail string        `json:"technicalOwnerEmail"`
	IsExceeded          bool          `json:"isExceeded"`
	IsCloseToExceed     bool          `json:"isCloseToExceed"`
	Opportunities       []Opportunity `json:"opportunities"`
}

// Opportunity is one allowance and what has been drawn against it.
type Opportunity struct {
	OpportunityID    string    `json:"opportunityId"`
	SalesforceID     string    `json:"salesforceId"`
	Name             string    `json:"name"`
	TotalMinutes     int       `json:"totalMinutes"`
	ConsumedMinutes  int       `json:"consumedMinutes"`
	RemainingMinutes int       `json:"remainingMinutes"`
	UsagePercent     float64   `json:"usagePercent"`
	State            *string   `json:"state"`
	IsExceeded       bool      `json:"isExceeded"`
	IsCloseToExceed  bool      `json:"isCloseToExceed"`
	UnmatchedLines   int       `json:"unmatchedLines"`
	Projects         []Project `json:"projects"`
}

// Project is one project's draw on an opportunity.
type Project struct {
	ProjectID          string `json:"projectId"`
	ProjectKey         string `json:"projectKey"`
	ProjectName        string `json:"projectName"`
	SalesforceID       string `json:"salesforceId"`
	ConsumedMinutes    int    `json:"consumedMinutes"`
	NonBillableMinutes int    `json:"nonBillableMinutes"`
	OnboardingMinutes  int    `json:"onboardingMinutes"`
}

// Transition is a project crossing a threshold, which is what the
// per-project usage notification reacts to.
type Transition struct {
	ProjectID           string  `json:"projectId"`
	ProjectKey          string  `json:"projectKey"`
	ProjectName         string  `json:"projectName"`
	AccountID           string  `json:"accountId"`
	AccountName         string  `json:"accountName"`
	From                *string `json:"from"`
	To                  string  `json:"to"`
	TotalText           string  `json:"totalText"`
	ConsumedText        string  `json:"consumedText"`
	RemainingText       string  `json:"remainingText"`
	AccountManagerEmail string  `json:"accountManagerEmail"`
	AccountManagerName  string  `json:"accountManagerName"`
	TechnicalOwnerEmail string  `json:"technicalOwnerEmail"`
}

// RecomputeResponse is what POST /query-hours/recompute returns.
type RecomputeResponse struct {
	Transitions []Transition `json:"transitions"`
}

// The three states, as the enum stores them.
const (
	StateNotified75 = "NOTIFIED_75"
	StateNotified90 = "NOTIFIED_90"
	StateClosure100 = "CLOSURE_NOTICE_100"
)
