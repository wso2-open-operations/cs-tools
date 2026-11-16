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

// SLABreachReportRequest asks for one week's SLA breaches under one set
// of filters.
//
// The ServiceNow originals are TWO separate report definitions --
// "Weekly SLA Breach Report" and "Weekly SLA Breach Report (Onboarding)"
// -- which share their ten SLA definitions but differ in BOTH their
// stage filter and their onboarding-status filter. So this is one
// parameterised query run once per report, never one query split in two.
//
// Their status sets are not complements: the onboarding report takes
// IN_PROGRESS only, while the other excludes IN_PROGRESS and NOT_STARTED,
// so a NOT_STARTED case appears in neither. Reproduced as configured
// rather than "fixed" here.
type SLABreachReportRequest struct {
	// Week, when set, is any instant inside the week to report on. Zero
	// means "the week before Now".
	Week time.Time
	// Now is the clock the week is resolved against; zero means time.Now.
	Now time.Time
	// Definitions are the sla_policy names the report covers, matched
	// exactly. Required: an empty list would silently report every policy.
	Definitions []string
	// Stages are the sla.stage values the report covers, matched exactly
	// and upper-cased by the caller. Empty means every stage.
	Stages []string
	// OnboardingStatuses keeps only cases whose project carries one of
	// these onboarding statuses -- ServiceNow's "Onboarding Status is one
	// of". Empty applies no such restriction.
	OnboardingStatuses []string
	// ExcludeOnboardingStatuses drops cases whose project carries one of
	// these -- ServiceNow's "is not one of". A case with no project, and
	// so no status at all, is KEPT: ServiceNow's "is not one of" does not
	// discard an empty value. Empty applies no exclusion.
	ExcludeOnboardingStatuses []string
}

// SLABreachRow is one breached SLA clock, as the report lists it.
type SLABreachRow struct {
	// Team is the account's integration CS team -- ServiceNow's
	// account.u_integration_cs_team, which Postgres carries as the
	// account's CRE group. The report groups on it.
	Team string `json:"team"`
	// CaseNumber is ServiceNow's own case number (e.g. "CS0023001").
	CaseNumber string `json:"caseNumber"`
	// WSO2CaseID is the portal's own reference (e.g. "WSO2-1000").
	WSO2CaseID string `json:"wso2CaseId"`
	// CaseID is the raw work item id, used to build the case link.
	CaseID string `json:"caseId"`
	// Subject is the case title.
	Subject string `json:"subject"`
	// Account is the customer account's name.
	Account string `json:"account"`
	// Severity is the case severity (e.g. "CRITICAL").
	Severity string `json:"severity"`
	// Product is the deployed product and version, when resolvable.
	Product string `json:"product"`
	// Definition is the sla_policy name -- the report's "SLA definition",
	// and its sort key within a team.
	Definition string `json:"definition"`
	// BreachedOn is sla.original_breached_on, the clock's real breach time.
	BreachedOn time.Time `json:"breachedOn"`
	// Stage is the clock's stage at report time.
	Stage string `json:"stage"`
	// AssigneeName is who the case is assigned to, when assigned.
	AssigneeName string `json:"assigneeName"`
	// OnboardingStatus is the case's project onboarding status, the field
	// that decides which of the two sections a row belongs to. Carried on
	// the row so the split stays visible to a reader of the response.
	OnboardingStatus string `json:"onboardingStatus"`
}

// SLABreachReport is one week's breaches under one set of filters.
type SLABreachReport struct {
	// PeriodStart is inclusive, PeriodEnd exclusive.
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
	// Rows are the matching breaches, ordered by the account's
	// integration CS team, then SLA definition, then breach time.
	Rows []SLABreachRow `json:"rows"`
}
