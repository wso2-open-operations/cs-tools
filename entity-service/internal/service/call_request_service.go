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

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type callRequestService struct {
	repo     repository.CallRequestRepository
	userRepo repository.UserRepository
}

// NewCallRequestService constructs a CallRequestService backed by Postgres
// (customer_call, migration 000072).
func NewCallRequestService(repo repository.CallRequestRepository, userRepo repository.UserRepository) CallRequestService {
	return &callRequestService{repo: repo, userRepo: userRepo}
}

// callerEmail resolves the caller's email from their x-user-id-token -- the
// same mechanism every other Postgres write path uses (there is no other
// notion of who is calling on this data source). customer_call.created_by/
// updated_by are plain VARCHAR audit strings holding the email, matching
// comment.created_by and the rest of this schema.
func callerEmail(ctx context.Context) (string, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	return email, nil
}

// CreateCallRequest implements CallRequestService.
func (s *callRequestService) CreateCallRequest(ctx context.Context, req domain.CreateCallRequestRequest) (domain.CreateCallRequestResponse, error) {
	if req.CaseID == "" {
		return domain.CreateCallRequestResponse{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.CreateCallRequestResponse{}, err
	}
	if req.Reason == "" {
		return domain.CreateCallRequestResponse{}, &apierror.ValidationError{Msg: "reason is required"}
	}
	if len(req.UTCTimes) == 0 {
		return domain.CreateCallRequestResponse{}, &apierror.ValidationError{Msg: "utcTimes must not be empty"}
	}
	if req.DurationMinutes <= 0 {
		return domain.CreateCallRequestResponse{}, &apierror.ValidationError{Msg: "durationInMinutes must be positive"}
	}

	email, err := callerEmail(ctx)
	if err != nil {
		return domain.CreateCallRequestResponse{}, err
	}
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.CreateCallRequestResponse{}, err
	}
	return s.repo.CreateCallRequest(ctx, req, user.ID, email)
}

// SearchCallRequests implements CallRequestService.
func (s *callRequestService) SearchCallRequests(ctx context.Context, req domain.SearchCallRequestsRequest) (domain.SearchCallRequestsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}
	if req.CaseID == "" {
		return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}
	var states []domain.CallRequestStateType
	if req.Filters != nil {
		for _, st := range req.Filters.States {
			if _, ok := validCallRequestStates[st]; !ok {
				return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid state %q", st)}
			}
		}
		states = req.Filters.States
	}

	views, total, err := s.repo.SearchCallRequests(ctx, req.CaseID, states, req.Pagination)
	if err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}
	return domain.SearchCallRequestsResponse{
		CallRequests: views,
		Total:        total,
		Limit:        req.Pagination.Limit,
		Offset:       req.Pagination.Offset,
	}, nil
}

// SearchAllCallRequests implements CallRequestService.
func (s *callRequestService) SearchAllCallRequests(ctx context.Context, req domain.SearchAllCallRequestsRequest) (domain.SearchCallRequestsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}
	if err := validateUUIDs("filters.assignedUserIds", req.Filters.AssignedUserIDs); err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}
	// The parent case's assignment team has no column on this data source
	// (work_item carries only assigned_to_id, and customer_call's own
	// assignment_group was deliberately skipped in migration 000072), so this
	// filter cannot be honored. Reject it rather than silently ignore it: an
	// ignored filter would widen the result set.
	if len(req.Filters.AssignmentTeamIDs) > 0 {
		return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "filters.assignmentTeamIds is only supported for the ServiceNow data source"}
	}
	for _, st := range req.Filters.States {
		if _, ok := validCallRequestStates[st]; !ok {
			return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid state %q", st)}
		}
	}
	for _, st := range req.Filters.CaseStates {
		if !validCaseState[st] {
			return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "filters.caseStates contains invalid value: " + string(st)}
		}
	}
	for _, st := range req.Filters.ExcludeCaseStates {
		if !validCaseState[st] {
			return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "filters.excludeCaseStates contains invalid value: " + string(st)}
		}
	}

	if req.SortBy.Field == "" {
		req.SortBy.Field = domain.CallRequestSortFieldUpdatedOn
	} else if !validCallRequestSortField[req.SortBy.Field] {
		return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "sortBy.field must be one of: createdOn, updatedOn, scheduleTime"}
	}
	if req.SortBy.Order == "" {
		req.SortBy.Order = domain.CallRequestSortOrderDesc
	} else if !validCallRequestSortOrder[req.SortBy.Order] {
		return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "sortBy.order must be one of: asc, desc"}
	}

	views, total, err := s.repo.SearchAllCallRequests(ctx, req.Filters, req.SortBy, req.Pagination)
	if err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}
	return domain.SearchCallRequestsResponse{
		CallRequests: views,
		Total:        total,
		Offset:       req.Pagination.Offset,
		Limit:        req.Pagination.Limit,
	}, nil
}

// UpdateCallRequest implements CallRequestService. The input rules mirror the
// ServiceNow implementation's (per-state required fields); req.Assignee is
// interpreted as the assignee's email, resolved to a user id. CancellationReason
// is rejected: customer_call has no column to store it, and accepting it would
// report success while silently discarding what the caller sent.
func (s *callRequestService) UpdateCallRequest(ctx context.Context, req domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error) {
	if req.CancellationReason != nil {
		return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "cancellationReason is not supported for the Postgres data source"}
	}
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateCallRequestResponse{}, err
	}
	if _, ok := validCallRequestStates[req.State]; !ok {
		return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid state %q", req.State)}
	}
	if req.DurationMinutes != nil && *req.DurationMinutes <= 0 {
		return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "durationInMinutes must be positive"}
	}
	if req.UTCTimes != nil && len(req.UTCTimes) == 0 {
		return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "utcTimes must not be empty when provided"}
	}
	if req.MeetingDate != nil {
		if _, err := time.Parse(time.RFC3339, *req.MeetingDate); err != nil {
			return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "meetingDate must be a valid RFC3339 timestamp"}
		}
	}
	if req.ActualDurationMin != nil && *req.ActualDurationMin <= 0 {
		return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "actualDurationMin must be positive"}
	}
	switch req.State {
	case domain.CallRequestStateScheduled:
		if req.MeetingDate == nil || strings.TrimSpace(*req.MeetingDate) == "" {
			return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "meetingDate is required when state is scheduled"}
		}
		if req.DurationMinutes == nil {
			return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "durationInMinutes is required when state is scheduled"}
		}
	case domain.CallRequestStateConcluded:
		if req.Notes == nil || strings.TrimSpace(*req.Notes) == "" {
			return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "notes is required when state is concluded"}
		}
	}
	if req.CaseID != "" {
		if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
			return domain.UpdateCallRequestResponse{}, err
		}
	}

	email, err := callerEmail(ctx)
	if err != nil {
		return domain.UpdateCallRequestResponse{}, err
	}

	var assigneeID *string
	if req.Assignee != nil && strings.TrimSpace(*req.Assignee) != "" {
		user, err := s.userRepo.GetUserByEmail(ctx, strings.TrimSpace(*req.Assignee))
		if err != nil {
			return domain.UpdateCallRequestResponse{}, err
		}
		assigneeID = &user.ID
	}

	return s.repo.UpdateCallRequest(ctx, req, assigneeID, email)
}
