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

// StandardChangeSuggestionCriteria are the thresholds a cluster of completed
// change requests is judged by: fixed configuration
// (STANDARD_CHANGE_SUGGESTION_*), applied when the snapshot is computed.
type StandardChangeSuggestionCriteria struct {
	// WindowMonths: only change requests completed within this many months count.
	WindowMonths int `json:"windowMonths"`
	// MinClosed: at least this many members closed (not rolled back) in the window.
	MinClosed int `json:"minClosed"`
	// MinSuccessPercent: closed / (closed + rolled back), at least this, in percent.
	MinSuccessPercent int `json:"minSuccessPercent"`
	// PlanSimilarityPercent: two members with the same normalised subject are the
	// same change only when their implementation plans are at least this similar
	// (word-set Jaccard, percent).
	PlanSimilarityPercent int `json:"planSimilarityPercent"`
}

// StandardChangeSuggestionsRequest is GET /standard-change-suggestions' query,
// applied over the stored snapshot.
type StandardChangeSuggestionsRequest struct {
	// AccountID limits the suggestions to one customer; empty is every customer.
	AccountID string
	// IncludeIneligible also returns clusters of two or more that miss a
	// criterion, each naming what it misses (for measuring the thresholds).
	IncludeIneligible bool
	// IncludeDismissed also returns dismissed suggestions, with their dismissal.
	IncludeDismissed bool
}

// StandardChangeSuggestionsResponse is GET /standard-change-suggestions' answer:
// the last published snapshot. ComputedOn is when it was published; nil (with no
// suggestions) when none has been computed yet.
type StandardChangeSuggestionsResponse struct {
	Suggestions []StandardChangeSuggestion       `json:"suggestions"`
	Criteria    StandardChangeSuggestionCriteria `json:"criteria"`
	ComputedOn  *time.Time                       `json:"computedOn"`
	// Scanned: completed change requests the snapshot considered (not raised from
	// a template, of a customer, completed in the window) and the clusters they
	// formed (two or more members); Dismissed is counted when read.
	Scanned StandardChangeSuggestionScan `json:"scanned"`
	// SkippedCustomers: customers (the one asked for, or all) whose suggestions
	// this snapshot does not hold, because the refresh could not compute them in
	// time: "not computed in the latest refresh", not "no suggestions".
	SkippedCustomers []StandardChangeSuggestionSkipped `json:"skippedCustomers"`
}

// RefreshStandardChangeSuggestionsRequest is the body of
// POST /standard-change-suggestions/refresh. The first call names no run and
// starts one; each later call continues it from the cursor the previous call
// returned, until one answers more = false, which publishes it.
type RefreshStandardChangeSuggestionsRequest struct {
	RunID  string `json:"runId,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	// Limit is the most customers this call computes (default 50, at most 500); a
	// call short of time stops earlier, after at least one.
	Limit int `json:"limit,omitempty"`
}

// RefreshStandardChangeSuggestionsResponse is one refresh call's answer.
type RefreshStandardChangeSuggestionsResponse struct {
	RunID string `json:"runId"`
	// What this call computed.
	Customers      int `json:"customers"`
	ChangeRequests int `json:"changeRequests"`
	Suggestions    int `json:"suggestions"`
	// Skipped: customers this call could not compute within their deadline.
	Skipped int `json:"skipped"`
	// More: call again with runId and nextCursor. When false the run was
	// published (ComputedOn) and Totals are the whole run's.
	More       bool                          `json:"more"`
	NextCursor string                        `json:"nextCursor,omitempty"`
	ComputedOn *time.Time                    `json:"computedOn,omitempty"`
	Totals     *StandardChangeSuggestionScan `json:"totals,omitempty"`
}

// StandardChangeSuggestionScan counts what one computation looked at.
type StandardChangeSuggestionScan struct {
	Customers      int `json:"customers"`
	ChangeRequests int `json:"changeRequests"`
	Clusters       int `json:"clusters"`
	Eligible       int `json:"eligible"`
	Dismissed      int `json:"dismissed"`
	// Skipped: customers the refresh could not compute in time (see
	// StandardChangeSuggestionsResponse.SkippedCustomers).
	Skipped int `json:"skipped"`
}

// StandardChangeSuggestionSkipped is a customer whose suggestions the snapshot
// does not hold: the refresh could not compute them in time.
type StandardChangeSuggestionSkipped struct {
	Customer EntityRef `json:"customer"`
	Reason   string    `json:"reason"`
}

// StandardChangeSuggestion is one cluster of a customer's completed change
// requests that look like the same change done again and again.
type StandardChangeSuggestion struct {
	// Key is the cluster's own key, unique within its customer and kept from one
	// refresh to the next: what it is dismissed by. It is SubjectKey, or
	// SubjectKey, '#' and 12 hex digits when the subject split into more than
	// one cluster.
	Key string `json:"key"`
	// SubjectKey is the normalised subject the cluster's members share.
	SubjectKey string    `json:"subjectKey"`
	Customer   EntityRef `json:"customer"`
	// Subject is the newest member's subject, as written.
	Subject       string  `json:"subject"`
	MemberCount   int     `json:"memberCount"`
	ClosedCount   int     `json:"closedCount"`
	RollbackCount int     `json:"rollbackCount"`
	SuccessRate   float64 `json:"successRate"`
	// Incidents caused by a member (incident.caused_by_id or change_request_id).
	Incidents []EntityRef                       `json:"incidents"`
	Risk      StandardChangeSuggestionBreakdown `json:"risk"`
	Impact    StandardChangeSuggestionBreakdown `json:"impact"`
	// FirstCompletedOn / LastCompletedOn bound the members' completions.
	FirstCompletedOn time.Time `json:"firstCompletedOn"`
	LastCompletedOn  time.Time `json:"lastCompletedOn"`
	// Eligible: every criterion is met; Unmet names those that are not.
	Eligible bool     `json:"eligible"`
	Unmet    []string `json:"unmet"`
	// ProposeFrom is the newest clean member (closed, no deviation from its
	// implementation plan, and never rejected by the CAB; failing that, the
	// newest closed one with no deviation): what a proposal starts from. Evidence is every other
	// closed member, newest first, so that ProposeFrom plus Evidence is at most
	// the 100 a proposal can cite.
	ProposeFrom *EntityRef  `json:"proposeFrom"`
	Evidence    []EntityRef `json:"evidence"`
	// OpenProposals already started from, or citing, a member (draft or under
	// review): proposing again would duplicate them.
	OpenProposals []EntityRef `json:"openProposals"`
	// Sample is set when this subject had more completed change requests than
	// are compared: the cluster was built from the newest Compared of Total (the
	// older ones are not in it, and not counted above).
	Sample *StandardChangeSuggestionSample `json:"sample,omitempty"`
	// Members, newest completion first.
	Members   []StandardChangeSuggestionMember   `json:"members"`
	Dismissal *StandardChangeSuggestionDismissal `json:"dismissal"`
	// DeviationCount: members that deviated from their implementation plan (all
	// of them, not only those listed).
	DeviationCount int `json:"deviationCount"`
	// The CAB's pushback on this change: its rejections of members and of
	// look-alikes, and its comments (StandardChangeCABHistory). Shown, never a
	// criterion. A snapshot computed before these existed reads them as zero.
	StandardChangeCABHistory
}

// StandardChangeCABHistory is what the CAB said about a set of change requests:
// a suggestion's members and the look-alikes attached to it, or a proposal's
// source and evidence and their look-alikes. A look-alike is one of the same
// customer's change requests that the CAB rejected (so it never completed and
// is no candidate) whose normalised subject is the suggestion's.
type StandardChangeCABHistory struct {
	// CABRejectionCount: change requests the CAB rejected (each once).
	// CABRejections lists them, newest rejection first, at most 20.
	CABRejectionCount int                          `json:"cabRejectionCount"`
	CABRejections     []StandardChangeCABRejection `json:"cabRejections"`
	// CABCommentCount: comments CAB approvers wrote on these change requests,
	// with any decision (an approval that asks for more is pushback too).
	// CABComments lists them, newest first, at most 20.
	CABCommentCount int                        `json:"cabCommentCount"`
	CABComments     []StandardChangeCABComment `json:"cabComments"`
}

// StandardChangeCABChangeRequest is the change request a CAB decision is on.
type StandardChangeCABChangeRequest struct {
	ID      string `json:"id"`
	Number  string `json:"number"`
	Subject string `json:"subject"`
	// State is its state now, lower case ("canceled", "closed", ...).
	State string `json:"state"`
}

// StandardChangeCABRejection is a change request the CAB rejected: its latest
// CAB rejection.
type StandardChangeCABRejection struct {
	ChangeRequest StandardChangeCABChangeRequest `json:"changeRequest"`
	// Member: a member of the suggestion (a proposal's source or evidence); false
	// for a look-alike.
	Member     bool      `json:"member"`
	RejectedOn time.Time `json:"rejectedOn"`
	// RejectedBy is the CAB member's name ("" when unknown).
	RejectedBy string `json:"rejectedBy"`
	// Comment is the rejection's comment as plain text ("" when none).
	Comment string `json:"comment"`
}

// StandardChangeCABComment is one comment a CAB approver wrote on a change request.
type StandardChangeCABComment struct {
	ChangeRequest StandardChangeCABChangeRequest `json:"changeRequest"`
	Member        bool                           `json:"member"`
	ApproverName  string                         `json:"approverName"`
	// State is the approver's decision: APPROVED, REJECTED, REQUESTED, CANCELLED, ...
	State string `json:"state"`
	// Comment as plain text (markup removed), at most 2000 characters.
	Comment   string    `json:"comment"`
	DecidedOn time.Time `json:"decidedOn"`
}

// StandardChangeSuggestionSample says a suggestion was computed from the newest
// Compared of the Total completed change requests with its subject.
type StandardChangeSuggestionSample struct {
	Compared int `json:"compared"`
	Total    int `json:"total"`
}

// StandardChangeSuggestionBreakdown counts members by level; Unknown is a
// member with none recorded.
type StandardChangeSuggestionBreakdown struct {
	Low      int `json:"low"`
	Moderate int `json:"moderate"`
	High     int `json:"high"`
	Unknown  int `json:"unknown"`
}

// StandardChangeSuggestionMember is one completed change request of a cluster.
type StandardChangeSuggestionMember struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	Subject     string    `json:"subject"`
	State       string    `json:"state"`
	CompletedOn time.Time `json:"completedOn"`
	Risk        *string   `json:"risk"`
	Impact      *string   `json:"impact"`
	// ClonedFrom is the member's clone source, when it was recorded.
	ClonedFrom   *string `json:"clonedFromId"`
	HasDeviation bool    `json:"hasDeviation"`
	// CABRejected: the CAB rejected it at least once (it was then resubmitted
	// and completed); CABCommentCount: the comments CAB approvers wrote on it.
	CABRejected     bool `json:"cabRejected"`
	CABCommentCount int  `json:"cabCommentCount"`
	// Why the member is in the cluster: "subject" (same normalised subject and a
	// similar or no implementation plan) or "clone" (cloned from a member).
	JoinedBy string `json:"joinedBy"`
}

// StandardChangeSuggestionDismissal records that a suggestion was dismissed.
type StandardChangeSuggestionDismissal struct {
	ID          string     `json:"id"`
	Reason      string     `json:"reason"`
	DismissedBy *EntityRef `json:"dismissedBy"`
	DismissedOn time.Time  `json:"dismissedOn"`
}

// DismissStandardChangeSuggestionRequest is the body of
// POST /standard-change-suggestions/dismissals.
type DismissStandardChangeSuggestionRequest struct {
	AccountID string `json:"accountId"`
	Key       string `json:"key"`
	Reason    string `json:"reason"`
}
