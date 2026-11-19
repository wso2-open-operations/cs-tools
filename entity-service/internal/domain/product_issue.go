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
	// SLA is the issue's resolution clock standing, computed at read time. Always an
	// object; State is "NONE" when the issue has no clock.
	SLA ProductIssueSLA `json:"sla"`
	// NonStandardColumn is true when the issue sits in a project board column the
	// portal does not map (BoardStatus OTHER); BoardStatusRaw is that column's name.
	NonStandardColumn bool `json:"nonStandardColumn"`
	// Clock is the newest resolution clock row the repository loaded; the service
	// turns it into SLA and clears it. Never serialised.
	Clock *ProductIssueClock `json:"-"`
}

// Product issue SLA standing states.
const (
	ProductIssueSLANone     = "NONE"
	ProductIssueSLAOK       = "OK"
	ProductIssueSLAAtRisk   = "AT_RISK"
	ProductIssueSLAPaused   = "PAUSED"
	ProductIssueSLAMet      = "MET"
	ProductIssueSLABreached = "BREACHED"
)

// ProductIssueClock is the newest RESOLUTION sla row of a product issue, as stored.
type ProductIssueClock struct {
	PolicyName           string
	Schedule             string
	Stage                string
	StartOn              time.Time
	PausedOn             *time.Time
	EndOn                *time.Time
	BudgetSeconds        float64
	BusinessPauseSeconds float64
	HasBreached          bool
	StoredPercentage     float64
}

// ProductIssueSLA is a product issue's SLA standing. Seconds are business seconds on
// the clock's schedule. PausedSince is set only while State is PAUSED (or BREACHED
// on a paused clock). The zero-clock answer is State NONE with every number 0.
type ProductIssueSLA struct {
	State            string     `json:"state"`
	BudgetSeconds    int64      `json:"budgetSeconds"`
	ConsumedSeconds  int64      `json:"consumedSeconds"`
	RemainingSeconds int64      `json:"remainingSeconds"`
	Percentage       float64    `json:"percentage"`
	Running          bool       `json:"running"`
	PausedSince      *time.Time `json:"pausedSince"`
	Priority         *string    `json:"priority"`
	PolicyName       *string    `json:"policyName"`
	Schedule         *string    `json:"schedule"`
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
	// Mine selects issues the caller opened (work_item.created_by or
	// product_issue.opened_by_email) or whose linked case is assigned to the caller.
	// The caller is the signed-in user of the request.
	Mine bool `json:"mine"`
	// Sort is "created" (default; newest first), "pausedSince" (oldest first; the
	// default when Mine is true) or "slaPercentage" (highest stored percentage first).
	Sort string `json:"sort"`
	// MineEmail is the resolved caller; set by the service, never read from the body.
	MineEmail string `json:"-"`
}

// Search sort orders.
const (
	ProductIssueSortCreated       = "created"
	ProductIssueSortPausedSince   = "pausedSince"
	ProductIssueSortSLAPercentage = "slaPercentage"
)

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

// ProductIssueReconcileRequest is the body of POST /product-work-items/reconcile,
// the daily pass that brings every linked product issue up to date with GitHub.
// Cursor is the previous answer's NextCursor ("" starts a run); Limit is the most
// issues one call works on (default 100, at most 500).
type ProductIssueReconcileRequest struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// ProductIssueReconcileResponse reports one call of the reconciliation. Boards
// counts the tracking boards the call visited; ItemsChecked the board items that
// matched an active link; LinksUpdated the links whose issue fields or board
// status changed; StatusEventsWritten the board events recorded; NotOnBoard the
// linked issues of a board's repositories missing from its listing (refreshed from
// the issue itself); Failed the issues or boards that could not be processed (the
// cause is logged). More is true while work is left: call again with NextCursor.
// RetryAfterSeconds is set when the engineering entity answered 429: the call
// stopped, and the caller should wait that long (or give up for the day).
type ProductIssueReconcileResponse struct {
	Boards              int    `json:"boards"`
	ItemsChecked        int    `json:"itemsChecked"`
	LinksUpdated        int    `json:"linksUpdated"`
	StatusEventsWritten int    `json:"statusEventsWritten"`
	NotOnBoard          int    `json:"notOnBoard"`
	Failed              int    `json:"failed"`
	NextCursor          string `json:"nextCursor"`
	More                bool   `json:"more"`
	RetryAfterSeconds   int    `json:"retryAfterSeconds,omitempty"`
}

// ProductIssueBackfillRequest is the body of POST /product-work-items/backfill:
// register the Origin/CS issues of one mapped repository that have no work item.
// Owner and Repository must be a product_repo_mapping row. SinceDays (default
// 180) is how far back closed issues count; every open one does. MaxIssues is the
// most issues one call considers (default 25, at most 100). DryRun writes nothing.
type ProductIssueBackfillRequest struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
	DryRun     bool   `json:"dryRun"`
	Cursor     string `json:"cursor,omitempty"`
	MaxIssues  int    `json:"maxIssues,omitempty"`
	SinceDays  int    `json:"sinceDays,omitempty"`
}

// ProductIssueBackfillResponse reports one call of the backfill. Considered is
// every Origin/CS issue looked at (patches and pull requests included). A dry run
// fills the WouldCreate* counts and writes nothing; a real run fills the Created*
// counts. AlreadyKnown issues had a link; only their missing comments and board
// events are filled in. Unmatched issues are registered with no case link: they
// are the unmatched list. More is true while issues are left: call again with
// NextCursor. RetryAfterSeconds is set when the engineering entity answered 429.
type ProductIssueBackfillResponse struct {
	DryRun               bool   `json:"dryRun"`
	Considered           int    `json:"considered"`
	WouldCreateMatched   int    `json:"wouldCreateMatched"`
	WouldCreateUnmatched int    `json:"wouldCreateUnmatched"`
	CreatedMatched       int    `json:"createdMatched"`
	CreatedUnmatched     int    `json:"createdUnmatched"`
	AlreadyKnown         int    `json:"alreadyKnown"`
	SkippedPatch         int    `json:"skippedPatch"`
	SkippedPullRequest   int    `json:"skippedPullRequest"`
	CommentsWritten      int    `json:"commentsWritten"`
	StatusEventsWritten  int    `json:"statusEventsWritten"`
	Failed               int    `json:"failed"`
	FailedIssueNumbers   []int  `json:"failedIssueNumbers"`
	NextCursor           string `json:"nextCursor"`
	More                 bool   `json:"more"`
	RetryAfterSeconds    int    `json:"retryAfterSeconds,omitempty"`
}
