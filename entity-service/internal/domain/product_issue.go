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

// Product issue (work item type PRODUCT_ISSUE) read and link models. Everything
// here is internal-only: no customer-facing route returns these shapes.

// Board and GitHub states a product issue filter accepts.
const (
	ProductIssueStateOpen   = "OPEN"
	ProductIssueStateClosed = "CLOSED"
)

// ProductIssueBoardStatuses are the normalised board statuses (the CHECK list on
// github_issue_link.board_status).
var ProductIssueBoardStatuses = []string{
	"OPEN", "IN_PROGRESS", "WAITING_ON_PRODUCT_TEAM", "WAITING_ON_CS", "REOPENED", "RESOLVED", "CLOSED", "OTHER",
}

// ProductIssueGitHub is the GitHub side of a product issue (github_issue_link).
type ProductIssueGitHub struct {
	Owner       string   `json:"owner"`
	Repository  string   `json:"repository"`
	IssueNumber int      `json:"issueNumber"`
	HTMLURL     *string  `json:"htmlUrl"`
	State       string   `json:"state"`
	StateReason *string  `json:"stateReason"`
	Labels      []string `json:"labels"`
}

// ProductIssueCaseRef names the case a product issue affects.
type ProductIssueCaseRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

// ProductIssueSummary is one product issue as the case list, the search and the
// link answers show it. Case and the link fields are null for an issue with no
// active AFFECTS link (unmatched).
type ProductIssueSummary struct {
	ID             string               `json:"id"`
	Number         string               `json:"number"`
	Title          string               `json:"title"`
	GitHub         ProductIssueGitHub   `json:"github"`
	BoardStatus    *string              `json:"boardStatus"`
	BoardStatusRaw *string              `json:"boardStatusRaw"`
	Priority       *string              `json:"priority"`
	Category       *string              `json:"category"`
	Case           *ProductIssueCaseRef `json:"case"`
	LinkSource     *string              `json:"linkSource"`
	LinkedOn       *time.Time           `json:"linkedOn"`
	LastActivityOn *time.Time           `json:"lastActivityOn"`
	LinkState      string               `json:"linkState"`
}

// ProductIssueComment is one comment of a product issue's conversation.
type ProductIssueComment struct {
	ID        string    `json:"id"`
	Type      *string   `json:"type"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedOn time.Time `json:"createdOn"`
	// Origin is PORTAL or GITHUB, null for a comment with no comment_github_ref.
	Origin          *string `json:"origin"`
	GitHubCommentID *int64  `json:"githubCommentId"`
	AuthorLogin     *string `json:"authorLogin"`
}

// ProductIssueStatusEvent is one row of the issue's board status history.
type ProductIssueStatusEvent struct {
	ID         string    `json:"id"`
	FromStatus *string   `json:"fromStatus"`
	ToStatus   string    `json:"toStatus"`
	WaitsOn    string    `json:"waitsOn"`
	OccurredOn time.Time `json:"occurredOn"`
	Actor      *string   `json:"actor"`
	Source     string    `json:"source"`
	RawStatus  *string   `json:"rawStatus"`
}

// ProductIssueDetail is GET /product-work-items/{id}.
type ProductIssueDetail struct {
	ProductIssueSummary
	OpenedByEmail         *string                   `json:"openedByEmail"`
	ABTTeam               *string                   `json:"abtTeam"`
	PublicIssueURL        *string                   `json:"publicIssueUrl"`
	CreatedOn             time.Time                 `json:"createdOn"`
	LastSyncedOn          *time.Time                `json:"lastSyncedOn"`
	Conversation          []ProductIssueComment     `json:"conversation"`
	ConversationTruncated bool                      `json:"conversationTruncated"`
	StatusHistory         []ProductIssueStatusEvent `json:"statusHistory"`
}

// ProductIssueListResponse is GET /cases/{id}/product-work-items.
type ProductIssueListResponse struct {
	Items []ProductIssueSummary `json:"items"`
}

// SearchProductIssuesFilters are the filters of POST /product-work-items/search.
type SearchProductIssuesFilters struct {
	CaseID        string   `json:"caseId"`
	GitHubState   string   `json:"githubState"`
	BoardStatuses []string `json:"boardStatuses"`
	Owner         string   `json:"owner"`
	Repository    string   `json:"repository"`
	// Unmatched selects product issues with no active AFFECTS link.
	Unmatched bool `json:"unmatched"`
	// Query is a substring of the PI number or title (a plain number also matches
	// the GitHub issue number).
	Query string `json:"query"`
}

// SearchProductIssuesRequest is the body of POST /product-work-items/search.
type SearchProductIssuesRequest struct {
	Filters    SearchProductIssuesFilters `json:"filters"`
	Pagination Pagination                 `json:"pagination"`
}

// SearchProductIssuesResponse is the answer of POST /product-work-items/search.
type SearchProductIssuesResponse struct {
	Items  []ProductIssueSummary `json:"items"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

// LinkProductIssueRequest is the body of POST /cases/{id}/product-work-items/links:
// exactly one of WorkItemID or the Owner/Repository/IssueNumber triple.
type LinkProductIssueRequest struct {
	CaseID      string `json:"-"`
	WorkItemID  string `json:"workItemId"`
	Owner       string `json:"owner"`
	Repository  string `json:"repository"`
	IssueNumber int    `json:"issueNumber"`
}
