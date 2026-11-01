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

// AutoclosureStage names one step of the auto-closure workflow.
type AutoclosureStage string

const (
	// AutoclosureStageReleaseHold returns a ticket whose customer-requested
	// hold has run out to the start of the workflow (step DEFAULT, clock reset).
	AutoclosureStageReleaseHold AutoclosureStage = "release_hold"
	// AutoclosureStageFirstReminder posts the first reminder comment.
	AutoclosureStageFirstReminder AutoclosureStage = "first_reminder"
	// AutoclosureStageSecondReminder posts the second reminder comment.
	AutoclosureStageSecondReminder AutoclosureStage = "second_reminder"
	// AutoclosureStageClose posts the closure comment and closes the ticket.
	AutoclosureStageClose AutoclosureStage = "close"
)

// Workflow positions stored in the case-like tables' autoclosure_step column.
// The column is free text and the sync still writes it, so a value outside
// this set is never an error, it just matches no stage.
const (
	AutoclosureStepDefault       = "DEFAULT"
	AutoclosureStepFirstComment  = "FIRST_COMMENT"
	AutoclosureStepSecondComment = "SECOND_COMMENT"
	AutoclosureStepOnHold        = "ON_HOLD"
)

// AutoclosureDefaultActor is who the auto-closure workflow acts as, and what the
// customer and staff see (comment author, "Added by", the activity entry), when
// no AUTOCLOSURE_SYSTEM_USER_EMAIL is configured: the system itself, not a person.
// It is stored where an email would be, matches no "user" row, and so leaves
// closed_by_user_id empty.
const AutoclosureDefaultActor = "System user"

// AutoclosureCandidatesRequest asks which tickets are due for a stage.
type AutoclosureCandidatesRequest struct {
	Stage AutoclosureStage `json:"stage"`
	// IntervalDays is how long a ticket must have waited before the stage is
	// due, counted in whole calendar days from the stage's reference time
	// (the workflow runs 24/7/365, so business days are not used). Ignored for
	// release_hold. Default 7.
	IntervalDays int `json:"intervalDays,omitempty"`
	// Limit caps how many are returned, oldest reference time first, so a
	// backlog larger than one run's budget drains in order. Default 500, max 5000.
	Limit int `json:"limit,omitempty"`
	// DailyLimit, when above zero, caps the notifications and closures the
	// workflow performs in one UTC day across all three stages: what has
	// already been advanced today (the audit table) is subtracted from it, so
	// a retried run, or three stages in a row, can never exceed it. Hold
	// releases send nothing and are not counted. Zero means no daily cap.
	DailyLimit int `json:"dailyLimit,omitempty"`
	// States optionally narrows the eligible states (awaiting_info,
	// solution_proposed); empty means both. It is how the Solution Proposed
	// flow is switched on before the Awaiting Info one.
	States []CaseState `json:"states,omitempty"`
}

// AutoclosureCandidate is one ticket due for a stage.
type AutoclosureCandidate struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	// State is the ticket's state (awaiting_info or solution_proposed).
	State CaseState `json:"state"`
	// Step is the workflow position now, nil when none was ever set.
	Step *string `json:"step,omitempty"`
	// ReferenceTime is the instant the stage's interval is measured from.
	ReferenceTime time.Time `json:"referenceTime"`
}

// AutoclosureCandidatesResponse lists the tickets due for a stage.
type AutoclosureCandidatesResponse struct {
	Candidates []AutoclosureCandidate `json:"candidates"`
	// Quarantined is how many tickets this stage left out because they failed it
	// repeatedly (3 FAILED events in 7 days). They are due but will not be tried
	// again until those failures age out, so someone should look at them.
	Quarantined int `json:"quarantined"`
}

// AutoclosureAdvanceRequest performs one stage on one ticket. The caller does
// not say what the ticket looks like: the service decides again, under a lock,
// whether the ticket is still due, so a stale candidate list is harmless.
type AutoclosureAdvanceRequest struct {
	Stage        AutoclosureStage `json:"stage"`
	IntervalDays int              `json:"intervalDays,omitempty"`
	States       []CaseState      `json:"states,omitempty"`
	// DailyLimit is the same per-UTC-day budget the listing applies, checked
	// again here, right before the ticket is claimed: two overlapping runs can
	// both read the same count when they list, and this is what stops them
	// advancing more than the budget between them (up to one in-flight ticket
	// per concurrent run can still slip past it). Zero means no cap.
	DailyLimit int `json:"dailyLimit,omitempty"`
	// DryRun runs the whole decision (still due? within the daily limit?) and
	// stops there: it posts no comment, claims no step, closes nothing and
	// queues nothing. It records an audit row with outcome DRY_RUN saying what
	// it would have done, and that row is not counted against the daily limit.
	// It is how the selection rules can be checked against real data before
	// the first real reminder goes out.
	DryRun bool `json:"dryRun,omitempty"`
	// Templates overrides comment text by key, "<state>.<stage>", for example
	// "awaiting_info.first_reminder". A key not listed keeps its default.
	Templates map[string]string `json:"templates,omitempty"`
}

// AutoclosureOutcome is the result of an advance.
type AutoclosureOutcome string

const (
	// AutoclosureOutcomeAdvanced means the stage ran.
	AutoclosureOutcomeAdvanced AutoclosureOutcome = "advanced"
	// AutoclosureOutcomeDryRun means a dry run found the ticket due and changed
	// nothing; only an audit row (outcome DRY_RUN) says what would have happened.
	AutoclosureOutcomeDryRun AutoclosureOutcome = "dry_run"
	// AutoclosureOutcomeSkipped means nothing was written: the ticket is no
	// longer due (it moved, got a hold, gained an open child, was handled by
	// another run). Not an error.
	AutoclosureOutcomeSkipped AutoclosureOutcome = "skipped"
)

// AutoclosureAdvanceResponse reports what an advance did.
type AutoclosureAdvanceResponse struct {
	CaseID   string             `json:"caseId"`
	Stage    AutoclosureStage   `json:"stage"`
	Outcome  AutoclosureOutcome `json:"outcome"`
	Reason   string             `json:"reason,omitempty"`
	FromStep *string            `json:"fromStep,omitempty"`
	ToStep   *string            `json:"toStep,omitempty"`
}
