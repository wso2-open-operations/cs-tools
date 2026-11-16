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

// SLABreachReportRequest asks for one week's SLA breaches, split into the
// two audiences the ServiceNow "Weekly SLA Breach Report" mailed as two
// attachments: cases whose project is still onboarding, and everything
// else (the CRE team's own cases).
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

// SLABreachReport is one week's breaches, already split in two.
type SLABreachReport struct {
	// PeriodStart is inclusive, PeriodEnd exclusive.
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
	// Onboarding holds rows whose project is still onboarding.
	Onboarding []SLABreachRow `json:"onboarding"`
	// CRE holds every other row.
	CRE []SLABreachRow `json:"cre"`
}
