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

// Product issue dashboard (phase 4). One set of definitions (service package,
// BuildProductIssueDashboard) feeds GET /product-work-items/dashboard and
// GET /reports/product-issues.

// ProductIssueDashboardMapping is the part of a product_repo_mapping row the
// dashboard resolves an issue's product with. lead_emails is deliberately absent.
type ProductIssueDashboardMapping struct {
	ProductName        string
	Owner              string
	Repository         string
	GithubLabel        string
	IsActive           bool
	BoardProjectNumber *int
}

// ProductIssueDashboardCase is the case an issue AFFECTS through its active link.
// Open is true when that work item is case-like and not closed.
type ProductIssueDashboardCase struct {
	ID      string
	Number  string
	Subject string
	Open    bool
}

// ProductIssueDashboardRow is one PRODUCT_ISSUE work item with a github_issue_link
// that is not DELETED, with every fact the dashboard definitions read. Never serialised.
type ProductIssueDashboardRow struct {
	ID                 string
	Number             string
	Title              string
	Owner              string
	Repository         string
	IssueNumber        int
	HTMLURL            *string
	GitHubState        string
	BoardStatus        *string
	BoardProjectNumber *int
	Labels             []string
	// OpenedOn is github_created_on, else the work item's created_on.
	OpenedOn       time.Time
	GitHubClosedOn *time.Time
	LastSyncedOn   *time.Time
	Priority       *string
	OpenedByEmail  *string
	// BoardWaitsOn is waits_on of the newest board move into the current board status.
	BoardWaitsOn string
	// The occurred_at of the newest status event of each kind (nil: none).
	LatestBoardEventOn    *time.Time
	LatestReopenEventOn   *time.Time
	LatestWOCEventOn      *time.Time
	LatestResolvedEventOn *time.Time
	// LatestClosedEventOn is the newest board move (raw status set) into CLOSED.
	LatestClosedEventOn *time.Time
	// Case is nil when the issue has no active AFFECTS link (unmatched).
	Case  *ProductIssueDashboardCase
	Clock *ProductIssueClock
}

// ProductIssueLead is one lead email of an active product_repo_mapping row.
type ProductIssueLead struct {
	ProductName string
	Email       string
}

// ProductIssueRecipient is a distinct lead email with the products that list it.
type ProductIssueRecipient struct {
	Email        string   `json:"email"`
	ProductNames []string `json:"productNames"`
}

// ProductIssueRecipients is the answer of GET /reports/product-issues/recipients.
type ProductIssueRecipients struct {
	Leads []ProductIssueRecipient `json:"leads"`
}

// ProductIssueDashboardSLACounts counts open issues by SLA state. None also
// holds an open issue whose newest clock is already finished (MET).
type ProductIssueDashboardSLACounts struct {
	OK       int `json:"ok"`
	AtRisk   int `json:"atRisk"`
	Breached int `json:"breached"`
	Paused   int `json:"paused"`
	None     int `json:"none"`
}

// ProductIssueDashboardTotals are the headline numbers (filtered products).
type ProductIssueDashboardTotals struct {
	Open                  int `json:"open"`
	OpenedInPeriod        int `json:"openedInPeriod"`
	ResolvedInPeriod      int `json:"resolvedInPeriod"`
	WaitingOnCs           int `json:"waitingOnCs"`
	WaitingOnProductTeam  int `json:"waitingOnProductTeam"`
	UnmatchedOpen         int `json:"unmatchedOpen"`
	NonStandardColumnOpen int `json:"nonStandardColumnOpen"`
	ProductIssueDashboardSLAFigures
}

// ProductIssueDashboardSLAFigures is the SLA part shared by the totals and a product row.
type ProductIssueDashboardSLAFigures struct {
	OpenBySLA            ProductIssueDashboardSLACounts `json:"openBySla"`
	SLAMet               int                            `json:"slaMet"`
	SLABreached          int                            `json:"slaBreached"`
	CompliancePercentage *float64                       `json:"compliancePercentage"`
}

// ProductIssueDashboardTrendPoint is one bucket of the trend.
type ProductIssueDashboardTrendPoint struct {
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Opened   int       `json:"opened"`
	Resolved int       `json:"resolved"`
}

// ProductIssueDashboardProduct is the figures of one product (or one unmapped repository).
type ProductIssueDashboardProduct struct {
	ProductName          *string `json:"productName"`
	Owner                string  `json:"owner"`
	Repository           string  `json:"repository"`
	Open                 int     `json:"open"`
	OpenedInPeriod       int     `json:"openedInPeriod"`
	ResolvedInPeriod     int     `json:"resolvedInPeriod"`
	WaitingOnCs          int     `json:"waitingOnCs"`
	WaitingOnProductTeam int     `json:"waitingOnProductTeam"`
	ProductIssueDashboardSLAFigures
}

// ProductIssueDashboardItem is one issue in a list of the dashboard.
type ProductIssueDashboardItem struct {
	ID            string               `json:"id"`
	Number        string               `json:"number"`
	Title         string               `json:"title"`
	ProductName   *string              `json:"productName"`
	Owner         string               `json:"owner"`
	Repository    string               `json:"repository"`
	IssueNumber   int                  `json:"issueNumber"`
	HTMLURL       *string              `json:"htmlUrl"`
	BoardStatus   *string              `json:"boardStatus"`
	Priority      *string              `json:"priority"`
	OpenedOn      time.Time            `json:"openedOn"`
	WocSince      *time.Time           `json:"wocSince"`
	Case          *ProductIssueCaseRef `json:"case"`
	OpenedByEmail *string              `json:"openedByEmail"`
	SLA           ProductIssueSLA      `json:"sla"`
}

// ProductIssueDashboardAgeBucket is one band of the waiting-on-CS ageing.
type ProductIssueDashboardAgeBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// ProductIssueDashboardAgeing is the waiting-on-CS ageing.
type ProductIssueDashboardAgeing struct {
	Buckets []ProductIssueDashboardAgeBucket `json:"buckets"`
	Oldest  []ProductIssueDashboardItem      `json:"oldest"`
}

// ProductIssueDashboardBlockingIssue is an open product-team issue that blocks a case.
type ProductIssueDashboardBlockingIssue struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	Title       string    `json:"title"`
	ProductName *string   `json:"productName"`
	BoardStatus *string   `json:"boardStatus"`
	OpenedOn    time.Time `json:"openedOn"`
}

// ProductIssueDashboardBlockedCase is a case blocked by product-team work.
type ProductIssueDashboardBlockedCase struct {
	CaseID         string                               `json:"caseId"`
	CaseNumber     string                               `json:"caseNumber"`
	Subject        string                               `json:"subject"`
	Issues         []ProductIssueDashboardBlockingIssue `json:"issues"`
	OldestOpenedOn time.Time                            `json:"oldestOpenedOn"`
}

// ProductIssueDashboardBlockedProduct counts the blocked cases of one product.
type ProductIssueDashboardBlockedProduct struct {
	ProductName *string `json:"productName"`
	Owner       string  `json:"owner"`
	Repository  string  `json:"repository"`
	Cases       int     `json:"cases"`
}

// ProductIssueDashboardBlocked is the cases blocked by product teams.
type ProductIssueDashboardBlocked struct {
	Total     int                                   `json:"total"`
	ByProduct []ProductIssueDashboardBlockedProduct `json:"byProduct"`
	Cases     []ProductIssueDashboardBlockedCase    `json:"cases"`
}

// ProductIssueDashboard is the answer of GET /product-work-items/dashboard and the
// data of GET /reports/product-issues.
type ProductIssueDashboard struct {
	PeriodStart  time.Time                         `json:"periodStart"`
	PeriodEnd    time.Time                         `json:"periodEnd"`
	GeneratedOn  time.Time                         `json:"generatedOn"`
	Bucket       string                            `json:"bucket"`
	Products     []string                          `json:"products"`
	Totals       ProductIssueDashboardTotals       `json:"totals"`
	Trend        []ProductIssueDashboardTrendPoint `json:"trend"`
	ByProduct    []ProductIssueDashboardProduct    `json:"byProduct"`
	WocAgeing    ProductIssueDashboardAgeing       `json:"wocAgeing"`
	BlockedCases ProductIssueDashboardBlocked      `json:"blockedCases"`
	BreachedOpen []ProductIssueDashboardItem       `json:"breachedOpen"`
}
