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
	// snWriteback/snMirror back CreateCallRequest's best-effort, asynchronous
	// ServiceNow mirror write under DATA_SOURCE=postgres-servicenow-dual-write
	// -- both nil in every other mode. Set only via
	// NewCallRequestServiceWithSNWriteback.
	//
	// UpdateCallRequest is deliberately NOT mirrored here (see that method's
	// own doc comment): unlike case/change_request/incident (whose CREATE is
	// ServiceNow-first under this data source, so their Postgres id IS the
	// real ServiceNow sys_id round-tripped through sysidToUUID),
	// CreateCallRequest is Postgres-first -- customer_call.id is a plain
	// Postgres-generated UUID with no ServiceNow counterpart recorded
	// anywhere (no column on customer_call holds one -- see migration
	// 000072). uuidToSysid(that id) would not resolve to any real ServiceNow
	// record, so a mirrored UpdateCallRequest would either 404 against
	// ServiceNow every single time (if the CREATE mirror never landed, or
	// landed under a different, SN-generated id) or -- far worse -- collide
	// with an unrelated ServiceNow record if the fabricated sys_id happened
	// to match one. Neither outcome is acceptable, and there is no id
	// mapping to close this gap with today.
	snWriteback *SNWritebackDispatcher
	snMirror    CallRequestService
}

// NewCallRequestService constructs a CallRequestService backed by Postgres
// (customer_call, migration 000072).
func NewCallRequestService(repo repository.CallRequestRepository, userRepo repository.UserRepository) CallRequestService {
	return &callRequestService{repo: repo, userRepo: userRepo}
}

// NewCallRequestServiceWithSNWriteback is NewCallRequestService plus the
// wiring DATA_SOURCE=postgres-servicenow-dual-write needs: CreateCallRequest
// dispatches a best-effort, asynchronous ServiceNow mirror write onto mirror
// after the Postgres write commits -- see CreateCallRequest's own doc
// comment, and callRequestService's own doc comment on why UpdateCallRequest
// is not mirrored. A separate constructor rather than extending
// NewCallRequestService's own signature, same reasoning as
// NewCaseServiceWithSNWriteback's own doc comment.
func NewCallRequestServiceWithSNWriteback(repo repository.CallRequestRepository, userRepo repository.UserRepository, dispatcher *SNWritebackDispatcher, mirror CallRequestService) CallRequestService {
	return &callRequestService{repo: repo, userRepo: userRepo, snWriteback: dispatcher, snMirror: mirror}
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
	resp, err := s.repo.CreateCallRequest(ctx, req, user.ID, email)
	if err != nil {
		return domain.CreateCallRequestResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback/snMirror are both nil otherwise -- see
	// callRequestService's own doc comment). Postgres has already committed
	// by this point; the ServiceNow-side id this mirror creates is
	// deliberately discarded (never written back onto the Postgres row) --
	// see callRequestService's own doc comment for why UpdateCallRequest
	// cannot use it later anyway.
	if s.snWriteback != nil {
		mirrorReq := req
		s.snWriteback.Dispatch(ctx, "call_request", resp.CallRequest.ID, "create",
			map[string]any{"caseId": req.CaseID, "reason": req.Reason, "utcTimes": req.UTCTimes, "durationInMinutes": req.DurationMinutes},
			func(writeCtx context.Context) error {
				_, err := s.snMirror.CreateCallRequest(writeCtx, mirrorReq)
				return err
			},
		)
	}

	return resp, nil
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
