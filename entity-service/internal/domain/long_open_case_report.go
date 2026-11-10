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

// LongOpenCaseReportVariantOpenBeforeYesterday selects the report of cases
// still in the open state that were created before the start of yesterday.
const LongOpenCaseReportVariantOpenBeforeYesterday = "open-before-yesterday"

// LongOpenCaseReportRequest asks for the long-open cases report: the cases
// still being worked on after OlderThanDays.
type LongOpenCaseReportRequest struct {
	// Variant selects the report. Empty is the long-open cases report;
	// LongOpenCaseReportVariantOpenBeforeYesterday is the report of cases
	// still in the open state that were created before yesterday. That
	// variant takes no OlderThanDays.
	Variant string
	// OlderThanDays is the age a case must have reached to be listed. The
	// cut-off is the start of the day that many days before Now, in the
	// report's own timezone, and a case is listed only when it was created
	// at or before that instant.
	OlderThanDays int
	// Now is the instant the report is built for. A caller that retries
	// passes its original firing time so the retry lists the same cases.
	Now time.Time
}

// LongOpenCaseReport is the long-open cases report: two tables of cases
// that are still being worked on, split by the onboarding status of the
// project each case belongs to. Each table is capped, so a defect can never
// produce an unbounded response; the matching Total says how many cases
// the table would hold without the cap.
type LongOpenCaseReport struct {
	GeneratedAt time.Time `json:"generatedAt"`
	// Variant echoes the requested variant; it is left out of the default
	// report.
	Variant       string `json:"variant,omitempty"`
	OlderThanDays int    `json:"olderThanDays"`
	// CreatedBefore is the cut-off every listed case was created on or
	// before. In the open-before-yesterday variant it is the start of
	// yesterday and every listed case was created strictly before it.
	CreatedBefore time.Time `json:"createdBefore"`
	// Cases are the long-open cases whose project is not being onboarded.
	Cases []LongOpenCaseReportRow `json:"cases"`
	// OnboardingCases are the long-open cases whose project onboarding is
	// in progress or not yet started.
	OnboardingCases []LongOpenCaseReportRow `json:"onboardingCases"`
	// CasesTotal and OnboardingCasesTotal are the full sizes of the two
	// tables. A table holds fewer rows than its total only when it was cut
	// at the cap.
	CasesTotal               int  `json:"casesTotal"`
	OnboardingCasesTotal     int  `json:"onboardingCasesTotal"`
	CasesTruncated           bool `json:"casesTruncated"`
	OnboardingCasesTruncated bool `json:"onboardingCasesTruncated"`
}

// LongOpenCaseReportRow is one long-open case.
type LongOpenCaseReportRow struct {
	// ID is the case's UUID, the identifier the case page and the case
	// detail endpoint route on. It is not InternalID.
	ID     string `json:"id"`
	Number string `json:"number"`
	// InternalID is the case's internal identifier (a display reference, not
	// the routing id); empty when it has none.
	InternalID string    `json:"internalId"`
	CreatedOn  time.Time `json:"createdOn"`
	UpdatedOn  time.Time `json:"updatedOn"`
	State      string    `json:"state"`
	// Severity is the case's severity in the API's vocabulary (critical,
	// high, ...); empty when none is set.
	Severity string `json:"severity"`
	// AssignedEngineer is the display name of the assignee, or empty.
	AssignedEngineer string `json:"assignedEngineer"`
	// CsTeam is the name of the CS team that owns the case's account.
	CsTeam string `json:"csTeam"`
	// AccountName is the name of the case's account. It is filled only by
	// the open-before-yesterday variant and left out of the default report.
	AccountName string `json:"accountName,omitempty"`
}

// LongOpenCaseReportData is what the repository returns for the report: the
// two tables and their full sizes.
type LongOpenCaseReportData struct {
	Cases                []LongOpenCaseReportRow
	OnboardingCases      []LongOpenCaseReportRow
	CasesTotal           int
	OnboardingCasesTotal int
}
