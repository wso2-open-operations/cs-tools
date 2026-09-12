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

// CaseFeedback is a single case-feedback (satisfaction rating) record.
type CaseFeedback struct {
	InstanceID string `json:"instanceId"`
	CaseID     string `json:"caseId"`
	// CaseNumber is the human-readable case number (e.g. "CS0441331").
	// Nullable -- absent if the case record could not be resolved.
	CaseNumber *string `json:"caseNumber"`
	// CaseInternalID is the WSO2 internal case id (e.g. "SCTPSUB-88") for CaseID.
	// Nullable -- absent if the case record could not be resolved upstream or has
	// no internal id assigned.
	CaseInternalID *string `json:"caseInternalId"`
	Rating         int     `json:"rating"`
	// RatingLabel is the human-readable label for Rating (e.g. "Satisfied"),
	// as supplied by the backing data source.
	RatingLabel string `json:"ratingLabel"`
	// Comment is nullable: feedback submitted without free-text comment
	// carries a null comment upstream, not an empty string.
	Comment     *string `json:"comment"`
	SubmittedAt string  `json:"submittedAt"`
	// SubmitterName is the display name of the customer contact who
	// submitted the feedback. Nullable -- absent if the submitting user
	// record could not be resolved.
	SubmitterName *string `json:"submitterName"`
	// SubmitterEmail is the email of the submitting contact. Nullable, same
	// as SubmitterName.
	SubmitterEmail *string `json:"submitterEmail"`
}

// SearchFeedbackFilters holds the optional filter criteria for a case-feedback search.
type SearchFeedbackFilters struct {
	CaseID     string   `json:"caseId,omitempty"`
	AccountIDs []string `json:"accountIds,omitempty"`
	// Rating is an exact-match filter (1-5). Nil means unset; the valid
	// range never includes 0, so a plain *int (not 0-as-sentinel) is used.
	Rating   *int   `json:"rating,omitempty"`
	DateFrom string `json:"dateFrom,omitempty"`
	DateTo   string `json:"dateTo,omitempty"`
}

// SearchFeedbackRequest is the input for POST /cases/feedback/search.
// Page and PageSize are 1-based/optional; omitted values let the backing
// data source apply its own defaults.
type SearchFeedbackRequest struct {
	Filters  SearchFeedbackFilters `json:"filters"`
	Page     int                   `json:"page,omitempty"`
	PageSize int                   `json:"pageSize,omitempty"`
}

// SearchFeedbackResponse is the result of a case-feedback search.
type SearchFeedbackResponse struct {
	Results      []CaseFeedback `json:"results"`
	TotalRecords int            `json:"totalRecords"`
}

// FeedbackBucket is the enum of supported aggregation granularities for
// POST /cases/feedback/aggregate: "day"/"week"/"month" bucket by submission
// date, "rating" groups by rating value instead of time (each bucket's
// BucketStart then carries the rating's own label, e.g. "Very Satisfied",
// rather than a date), and the 5 "reasons_*" values group by which free-text
// reason was selected for that specific rating level (each bucket's
// BucketStart then carries the reason's own label, e.g. "Unhelpful support").
type FeedbackBucket string

const (
	FeedbackBucketDay                     FeedbackBucket = "day"
	FeedbackBucketWeek                    FeedbackBucket = "week"
	FeedbackBucketMonth                   FeedbackBucket = "month"
	FeedbackBucketRating                  FeedbackBucket = "rating"
	FeedbackBucketReasonsVeryDissatisfied FeedbackBucket = "reasons_very_dissatisfied"
	FeedbackBucketReasonsDissatisfied     FeedbackBucket = "reasons_dissatisfied"
	FeedbackBucketReasonsNeutral          FeedbackBucket = "reasons_neutral"
	FeedbackBucketReasonsSatisfied        FeedbackBucket = "reasons_satisfied"
	FeedbackBucketReasonsVerySatisfied    FeedbackBucket = "reasons_very_satisfied"
)

// AggregateFeedbackFilters holds the optional filter criteria for a
// date-bucketed feedback aggregation. Unlike SearchFeedbackFilters, this has
// no CaseID: aggregation is the many-cases trend endpoint, not scoped to one case.
type AggregateFeedbackFilters struct {
	AccountIDs []string `json:"accountIds,omitempty"`
	DateFrom   string   `json:"dateFrom,omitempty"`
	DateTo     string   `json:"dateTo,omitempty"`
}

// AggregateFeedbackRequest is the input for POST /cases/feedback/aggregate.
type AggregateFeedbackRequest struct {
	Filters AggregateFeedbackFilters `json:"filters"`
	// Bucket selects the aggregation granularity. Required; see FeedbackBucket.
	Bucket FeedbackBucket `json:"bucket"`
}

// FeedbackBucketResult is one date bucket in an aggregated feedback result.
type FeedbackBucketResult struct {
	BucketStart string  `json:"bucketStart"`
	AvgRating   float64 `json:"avgRating"`
	Count       int     `json:"count"`
}

// AggregateFeedbackResponse is the result of a date-bucketed feedback aggregation.
type AggregateFeedbackResponse struct {
	Buckets      []FeedbackBucketResult `json:"buckets"`
	TotalRecords int                    `json:"totalRecords"`
}
