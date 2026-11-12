/*
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package domain

import "time"

// Who last wrote a run task's value (plg_task_value_source_enum).
const (
	ValueSourceManual     = "MANUAL"
	ValueSourceValidation = "VALIDATION"
)

// TaskCodeEmailClassification is the registration task the queue reports back,
// so the job can skip the Directory and model calls once it is answered.
const TaskCodeEmailClassification = "REGISTERED_EMAIL_CLASSIFICATION"

// ValidationQueueRequest is the body of POST /plg/registrations/validation-queue.
type ValidationQueueRequest struct {
	ProductCode     string    `json:"productCode"`
	RegisteredAfter time.Time `json:"registeredAfter"`
	Limit           int       `json:"limit"`
}

// ValidationQueueItem is one pairing for the job to validate. Ids only.
type ValidationQueueItem struct {
	OrgPlatformID string    `json:"orgPlatformId"`
	CompanyID     string    `json:"companyId"`
	OwnerUserID   *string   `json:"ownerUserId"`
	RegisteredOn  time.Time `json:"registeredOn"`
	// The stored classification code, whoever set it; nil when unanswered.
	EmailClassification *string `json:"emailClassification"`
	// When the job last wrote this pairing's summary; nil when never.
	LastValidatedOn *time.Time `json:"lastValidatedOn"`
}

// ValidationQueueResult answers POST /plg/registrations/validation-queue.
type ValidationQueueResult struct {
	Items []ValidationQueueItem `json:"items"`
}

// ActivitySummary is what the job derived from one pairing's events.
type ActivitySummary struct {
	WindowStart  time.Time      `json:"windowStart"`
	WindowEnd    time.Time      `json:"windowEnd"`
	FirstEventOn *time.Time     `json:"firstEventOn"`
	LastEventOn  *time.Time     `json:"lastEventOn"`
	EventCount   int            `json:"eventCount"`
	ActionCounts map[string]int `json:"actionCounts"`
	FurthestStep *string        `json:"furthestStep"`
}

// ValidationValue sets one task by code: the one value field its type takes,
// or none to clear it.
type ValidationValue struct {
	Code         string    `json:"code"`
	BoolValue    *bool     `json:"boolValue,omitempty"`
	NumberValue  *float64  `json:"numberValue,omitempty"`
	TextValue    *string   `json:"textValue,omitempty"`
	CheckedCodes *[]string `json:"checkedCodes,omitempty"`
}

// ApplyValidationRequest is the body of
// PUT /plg/pairings/{orgPlatformId}/registration-validation.
type ApplyValidationRequest struct {
	OrgPlatformID string            `json:"-"`
	Summary       ActivitySummary   `json:"summary"`
	Values        []ValidationValue `json:"values"`
	// Abandon closes the run and moves the pairing to ABANDONED, so an
	// excluded registration leaves the engineers' lists. Needs
	// ELIGIBILITY_ASSESSMENT_OUTCOME = EXCLUDED among the values.
	Abandon bool `json:"abandon"`
}

// Eligibility outcome the job may abandon on.
const (
	TaskCodeEligibilityOutcome = "ELIGIBILITY_ASSESSMENT_OUTCOME"
	TaskCodeEligibilityReason  = "ELIGIBILITY_ASSESSMENT_REASON"
	OutcomeExcluded            = "EXCLUDED"
)

// Why a value was not written.
const (
	SkipManual  = "MANUAL"  // an engineer set it
	SkipMissing = "MISSING" // the run has no task with that code
	SkipClosed  = "CLOSED"  // the run is closed
)

// SkippedValue is a value the write left alone.
type SkippedValue struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

// ApplyValidationResult answers the write.
type ApplyValidationResult struct {
	PlaybookRunID string         `json:"playbookRunId"`
	Attached      bool           `json:"attached"`
	Written       []string       `json:"written"`
	Skipped       []SkippedValue `json:"skipped"`
	// Abandoned is true when this call closed the run and moved the pairing to ABANDONED.
	Abandoned bool `json:"abandoned"`
}
