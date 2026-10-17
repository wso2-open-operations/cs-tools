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

// Package viewer holds the wire types shared by the viewer-audience
// endpoints that were ported from SupportPortalLite. The data behind them
// now comes from entity-service (Postgres); the response shapes are kept
// so the ported frontend keeps working unchanged.
package viewer

import "errors"

// ErrAccountNotFound is returned when no account matches the given id.
var ErrAccountNotFound = errors.New("viewer: no account found for that id")

// ErrProjectNotFound is returned when no project matches the given id.
var ErrProjectNotFound = errors.New("viewer: no project found for the given id")

// GoLiveStatus is a project's go-live status badge.
type GoLiveStatus struct {
	Status string `json:"status"`
	IsRisk bool   `json:"isRisk"`
	State  string `json:"state"`
}

// CaseInfo is a minimal case reference used inside health-detail dropdowns.
type CaseInfo struct {
	SysID  string `json:"sysId"`
	Number string `json:"number"`
}

// CaseGroup groups cases by priority for the health-detail dropdowns.
type CaseGroup struct {
	Priority string     `json:"priority"`
	Count    int        `json:"count"`
	Cases    []CaseInfo `json:"cases,omitempty"`
}

// Deployment is a minimal deployment reference used inside health-detail
// dropdowns.
type Deployment struct {
	SysID string `json:"sysId"`
	Name  string `json:"name"`
}

// EolProduct is an end-of-life product and the deployments using it.
type EolProduct struct {
	Name        string       `json:"name"`
	EolDate     string       `json:"eolDate"`
	Deployments []Deployment `json:"deployments"`
}

// CaseLink is a case/escalation reference used inside health-detail
// dropdowns.
type CaseLink struct {
	SysID            string `json:"sysId,omitempty"`
	Number           string `json:"number,omitempty"`
	EscalationSysID  string `json:"escalationSysId,omitempty"`
	EscalationNumber string `json:"escalationNumber,omitempty"`
}

// ProjectDetail is a single project's detailed risk breakdown within an
// account's customer-health detail view.
type ProjectDetail struct {
	SysID                   string       `json:"sysId"`
	Name                    string       `json:"name"`
	HasRecentCases          bool         `json:"hasRecentCases"`
	DetailedRecentCases     []CaseGroup  `json:"detailedRecentCases"`
	TotalRecentCases        int          `json:"totalRecentCases"`
	HasAbandonedCases       bool         `json:"hasAbandonedCases"`
	DetailedAbandonedCases  []CaseLink   `json:"detailedAbandonedCases"`
	IsUsingEolProduct       bool         `json:"isUsingEolProduct"`
	SoftwareModel           []EolProduct `json:"softwareModel"`
	Deployments             []Deployment `json:"deployments"`
	HasMigrationDelays      bool         `json:"hasMigrationDelays"`
	DetailedMigrationDelays []CaseLink   `json:"detailedMigrationDelays"`
	HasEscalatedCases       bool         `json:"hasEscalatedCases"`
	DetailedEscalatedCases  []CaseLink   `json:"detailedEscalatedCases"`
	GoLiveStatus            GoLiveStatus `json:"goLiveStatus"`
}

// AccountDetail is the full customer-health detail view for one account.
type AccountDetail struct {
	AccountName      string          `json:"accountName"`
	CustomerProjects []ProjectDetail `json:"customerProjects"`
}

// EscalationDetail is one row of an account's escalation list.
type EscalationDetail struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	State       string `json:"state"`
	EscalatedOn string `json:"escalatedOn"`
}

// TimeLogBreakdownDetails is the time-log breakdown report for a project.
type TimeLogBreakdownDetails struct {
	ProjectName         string                 `json:"projectName"`
	ProjectKey          string                 `json:"projectKey"`
	ProjectType         string                 `json:"projectType"`
	RemainingQueryHours string                 `json:"remainingQueryHours"`
	TotalQueryHours     string                 `json:"totalQueryHours"`
	Cases               []TimeLogBreakdownCase `json:"cases"`
}

// TimeLogBreakdownCase is one case's row in the time-log breakdown report.
type TimeLogBreakdownCase struct {
	CaseSysID          string            `json:"caseSysId"`
	CaseNumber         string            `json:"caseNumber"`
	CaseID             string            `json:"caseId"`
	CaseType           string            `json:"caseType"`
	ShortDescription   string            `json:"shortDescription"`
	Priority           string            `json:"priority"`
	State              string            `json:"state"`
	TotalHours         string            `json:"totalHours"`
	ConsumedQueryHours string            `json:"consumedQueryHours"`
	TimeCards          []TimeCardDetails `json:"timeCards"`
}

// CSReportDetails is the customer-success report for one project over a date range.
// SLA clocks stay empty: Postgres does not store them yet.
type CSReportDetails struct {
	SubscriptionDetails SubscriptionDetail  `json:"subscriptionDetails"`
	CasesRecords        []CaseRecordDetail  `json:"casesRecords"`
	SlaDetails          SlaStats            `json:"slaDetails"`
	ProjectDeployments  []ProjectDeployment `json:"projectDeployments"`
	MonthlyCounts       []MonthlyCount      `json:"monthlyCounts"`
}

// SubscriptionDetail is the project header on the CS report.
type SubscriptionDetail struct {
	ProjectName        string `json:"projectName"`
	ProjectKey         string `json:"projectKey"`
	ProjectType        string `json:"projectType"`
	AccountName        string `json:"accountName"`
	StartDate          string `json:"startDate"`
	EndDate            string `json:"endDate"`
	SupportTier        string `json:"supportTier"`
	Subscription       string `json:"subscription"`
	TotalQueryHours    string `json:"totalQueryHours"`
	ConsumedQueryHours string `json:"consumedQueryHours"`
}

// CaseRecordDetail is one case row on the CS report.
type CaseRecordDetail struct {
	CaseSysID      string `json:"caseSysId"`
	CaseNumber     string `json:"caseNumber"`
	EngagementType string `json:"engagementType"`
	CaseType       string `json:"caseType"`
	CasePriority   string `json:"casePriority"`
	CaseState      string `json:"caseState"`
	Opened         string `json:"opened"`
	Description    string `json:"description"`
	Updated        string `json:"updated"`
	Deployment     string `json:"deployment"`
	ProductName    string `json:"productName"`
}

// SlaStats is the SLA block. Records stay empty until SLA clocks live in Postgres.
// An empty performance object is omitted so the page shows that nothing is available.
type SlaStats struct {
	SlaRecords          []SlaRecordDetail   `json:"slaRecords"`
	SlaPerformanceStats SlaPerformanceStats `json:"slaPerformanceStats,omitzero"`
}

// SlaRecordDetail is one SLA task row.
type SlaRecordDetail struct {
	Task                      string `json:"task"`
	SlaDefinition             string `json:"slaDefinition"`
	BusinessElapsedPercentage string `json:"businessElapsedPercentage"`
}

// SlaStatsDetail is one percentile cell in the SLA summary.
type SlaStatsDetail struct {
	Fraction   string `json:"fraction"`
	Percentage string `json:"percentage"`
}

// SlaPerformanceStats keeps the capitalized keys the CS report page already reads.
type SlaPerformanceStats struct {
	Workaround SlaStatsDetail `json:"Workaround"`
	Resolution SlaStatsDetail `json:"Resolution"`
	Response   SlaStatsDetail `json:"Response"`
}

// ProjectDeployment is one environment and the products deployed in it.
type ProjectDeployment struct {
	Name     string    `json:"name"`
	Products []Product `json:"products"`
}

// Product is one deployed product on the CS report.
type Product struct {
	Name                           string `json:"name"`
	Version                        string `json:"version"`
	SupportStatus                  string `json:"supportStatus"`
	EOLDate                        string `json:"eolDate"`
	Cores                          string `json:"cores"`
	TPS                            string `json:"tps"`
	UpdateLevelInfo                *int   `json:"updateLevelInfo,omitempty"`
	EarliestPossibleSupportEOLDate string `json:"earliestPossibleSupportEOLDate"`
}

// MonthlyCount is one month of incident and query counts.
type MonthlyCount struct {
	YearAndMonth string `json:"yearAndMonth"`
	Counts       Count  `json:"counts"`
}

// Count is the incident and query totals for one month.
type Count struct {
	IncidentCount int `json:"incidentCount"`
	QueryCount    int `json:"queryCount"`
}

// TimeCardDetails is one time card under a TimeLogBreakdownCase.
type TimeCardDetails struct {
	Total      string `json:"total"`
	CreatedOn  string `json:"createdOn"`
	CreatedBy  string `json:"createdBy"`
	UpdatedOn  string `json:"updatedOn"`
	UpdatedBy  string `json:"updatedBy"`
	IsBillable string `json:"isBillable"`
	State      string `json:"state"`
}
