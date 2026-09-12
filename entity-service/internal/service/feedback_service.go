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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// validFeedbackBuckets is the closed set of bucket granularities the backing
// data source accepts for POST /cases/feedback/aggregate.
var validFeedbackBuckets = map[domain.FeedbackBucket]bool{
	domain.FeedbackBucketDay:                     true,
	domain.FeedbackBucketWeek:                    true,
	domain.FeedbackBucketMonth:                   true,
	domain.FeedbackBucketRating:                  true,
	domain.FeedbackBucketReasonsVeryDissatisfied: true,
	domain.FeedbackBucketReasonsDissatisfied:     true,
	domain.FeedbackBucketReasonsNeutral:          true,
	domain.FeedbackBucketReasonsSatisfied:        true,
	domain.FeedbackBucketReasonsVerySatisfied:    true,
}

// snFeedbackFilters mirrors the "filters" object accepted by the Choreo
// POST /cases/feedback/search and POST /cases/feedback/aggregate endpoints.
// The aggregate endpoint's closed-record binding rejects a caseId field, so
// this struct -- shared by both outbound payloads -- deliberately never sets
// CaseID for an aggregate call (see snAggregateFeedbackPayload).
type snFeedbackFilters struct {
	CaseID     string   `json:"caseId,omitempty"`
	AccountIDs []string `json:"accountIds,omitempty"`
	Rating     *int     `json:"rating,omitempty"`
	DateFrom   string   `json:"dateFrom,omitempty"`
	DateTo     string   `json:"dateTo,omitempty"`
}

// snSearchFeedbackPayload is the Choreo POST /cases/feedback/search request body.
type snSearchFeedbackPayload struct {
	Filters  snFeedbackFilters `json:"filters"`
	Page     int               `json:"page,omitempty"`
	PageSize int               `json:"pageSize,omitempty"`
}

// snFeedbackRow mirrors a single row in the Choreo feedback-search response.
type snFeedbackRow struct {
	InstanceID     string  `json:"instanceId"`
	CaseID         string  `json:"caseId"`
	CaseNumber     *string `json:"caseNumber"`
	CaseInternalID *string `json:"caseInternalId"`
	Rating         int     `json:"rating"`
	RatingLabel    string  `json:"ratingLabel"`
	Comment        *string `json:"comment"`
	SubmittedAt    string  `json:"submittedAt"`
	SubmitterName  *string `json:"submitterName"`
	SubmitterEmail *string `json:"submitterEmail"`
}

// snSearchFeedbackResponse mirrors the Choreo POST /cases/feedback/search response.
type snSearchFeedbackResponse struct {
	Results      []snFeedbackRow `json:"results"`
	TotalRecords int             `json:"totalRecords"`
}

// snAggregateFeedbackPayload is the Choreo POST /cases/feedback/aggregate request
// body. Filters never carries CaseID: the aggregate endpoint's closed-record
// binding rejects it with a 400 (it is the many-cases trend endpoint, not
// scoped to one case).
type snAggregateFeedbackPayload struct {
	Filters snFeedbackFilters     `json:"filters"`
	Bucket  domain.FeedbackBucket `json:"bucket"`
}

// snFeedbackBucketRow mirrors a single bucket in the Choreo feedback-aggregate response.
type snFeedbackBucketRow struct {
	BucketStart string  `json:"bucketStart"`
	AvgRating   float64 `json:"avgRating"`
	Count       int     `json:"count"`
}

// snAggregateFeedbackResponse mirrors the Choreo POST /cases/feedback/aggregate response.
type snAggregateFeedbackResponse struct {
	Buckets      []snFeedbackBucketRow `json:"buckets"`
	TotalRecords int                   `json:"totalRecords"`
}

type snFeedbackService struct {
	client *integrationservice.Client
}

// NewServiceNowFeedbackService constructs a FeedbackService backed by the Choreo API.
func NewServiceNowFeedbackService(client *integrationservice.Client) FeedbackService {
	return &snFeedbackService{client: client}
}

// SearchFeedback implements FeedbackService by calling the Choreo
// POST /cases/feedback/search endpoint.
func (s *snFeedbackService) SearchFeedback(ctx context.Context, req domain.SearchFeedbackRequest) (domain.SearchFeedbackResponse, error) {
	if req.Page < 0 {
		return domain.SearchFeedbackResponse{}, &apierror.ValidationError{Msg: "page must not be negative"}
	}
	if req.PageSize < 0 {
		return domain.SearchFeedbackResponse{}, &apierror.ValidationError{Msg: "pageSize must not be negative"}
	}
	if err := validateUUIDs("accountIds", req.Filters.AccountIDs); err != nil {
		return domain.SearchFeedbackResponse{}, err
	}
	if req.Filters.CaseID != "" {
		if err := validateUUIDs("caseId", []string{req.Filters.CaseID}); err != nil {
			return domain.SearchFeedbackResponse{}, err
		}
	}
	if req.Filters.Rating != nil && (*req.Filters.Rating < 1 || *req.Filters.Rating > 5) {
		return domain.SearchFeedbackResponse{}, &apierror.ValidationError{Msg: "rating must be between 1 and 5"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	// AccountIDs/CaseID are UUIDs at this service's own public boundary, like
	// every other ID this entity-service exposes; converted to ServiceNow
	// sysids here to match the sysid<->UUID boundary convention used by every
	// other SN-backed filter in this package (see sn_id.go). Both conversion
	// helpers are safe no-ops on input that isn't already sysid/UUID shaped.
	payload := snSearchFeedbackPayload{
		Filters: snFeedbackFilters{
			CaseID:     uuidToSysid(req.Filters.CaseID),
			AccountIDs: uuidsToSysids(req.Filters.AccountIDs),
			Rating:     req.Filters.Rating,
			DateFrom:   req.Filters.DateFrom,
			DateTo:     req.Filters.DateTo,
		},
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	raw, err := s.client.Post(ctx, "/cases/feedback/search", token, payload)
	if err != nil {
		return domain.SearchFeedbackResponse{}, err
	}

	var snResp snSearchFeedbackResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchFeedbackResponse{}, fmt.Errorf("sn search feedback: parse response: %w", err)
	}

	results := make([]domain.CaseFeedback, 0, len(snResp.Results))
	for _, row := range snResp.Results {
		results = append(results, domain.CaseFeedback{
			InstanceID:     sysidToUUID(row.InstanceID),
			CaseID:         sysidToUUID(row.CaseID),
			CaseNumber:     row.CaseNumber,
			CaseInternalID: row.CaseInternalID,
			Rating:         row.Rating,
			RatingLabel:    row.RatingLabel,
			Comment:        row.Comment,
			SubmittedAt:    row.SubmittedAt,
			SubmitterName:  row.SubmitterName,
			SubmitterEmail: row.SubmitterEmail,
		})
	}

	return domain.SearchFeedbackResponse{
		Results:      results,
		TotalRecords: snResp.TotalRecords,
	}, nil
}

// AggregateFeedback implements FeedbackService by calling the Choreo
// POST /cases/feedback/aggregate endpoint.
func (s *snFeedbackService) AggregateFeedback(ctx context.Context, req domain.AggregateFeedbackRequest) (domain.AggregateFeedbackResponse, error) {
	if !validFeedbackBuckets[req.Bucket] {
		return domain.AggregateFeedbackResponse{}, &apierror.ValidationError{Msg: "bucket must be one of: day, week, month, rating, reasons_very_dissatisfied, reasons_dissatisfied, reasons_neutral, reasons_satisfied, reasons_very_satisfied"}
	}
	if err := validateUUIDs("accountIds", req.Filters.AccountIDs); err != nil {
		return domain.AggregateFeedbackResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snAggregateFeedbackPayload{
		Filters: snFeedbackFilters{
			AccountIDs: uuidsToSysids(req.Filters.AccountIDs),
			DateFrom:   req.Filters.DateFrom,
			DateTo:     req.Filters.DateTo,
		},
		Bucket: req.Bucket,
	}

	raw, err := s.client.Post(ctx, "/cases/feedback/aggregate", token, payload)
	if err != nil {
		return domain.AggregateFeedbackResponse{}, err
	}

	var snResp snAggregateFeedbackResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.AggregateFeedbackResponse{}, fmt.Errorf("sn aggregate feedback: parse response: %w", err)
	}

	buckets := make([]domain.FeedbackBucketResult, 0, len(snResp.Buckets))
	for _, b := range snResp.Buckets {
		buckets = append(buckets, domain.FeedbackBucketResult{
			BucketStart: b.BucketStart,
			AvgRating:   b.AvgRating,
			Count:       b.Count,
		})
	}

	return domain.AggregateFeedbackResponse{
		Buckets:      buckets,
		TotalRecords: snResp.TotalRecords,
	}, nil
}
