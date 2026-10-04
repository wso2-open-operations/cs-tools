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
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type callRequestService struct {
	repo     repository.CallRequestRepository
	userRepo repository.UserRepository
	// snWriteback/snMirror back CreateCallRequest's and UpdateCallRequest's
	// best-effort, asynchronous ServiceNow mirror writes under
	// DATA_SOURCE=postgres-servicenow-dual-write -- both nil in every other
	// mode. Set only via NewCallRequestServiceWithSNWriteback.
	//
	// UpdateCallRequest's mirror needs an id mapping CreateCallRequest didn't
	// used to record: unlike case/change_request/incident (whose CREATE is
	// ServiceNow-first under this data source, so their Postgres id IS the
	// real ServiceNow sys_id round-tripped through sysidToUUID),
	// CreateCallRequest is Postgres-first -- customer_call.id is a plain
	// Postgres-generated UUID with no ServiceNow counterpart. uuidToSysid(that
	// id) would not resolve to any real ServiceNow record. Migration 000088
	// adds customer_call.sn_sys_id for exactly this: CreateCallRequest's own
	// mirror success path now persists it (best-effort, asynchronously --
	// see that method below), and UpdateCallRequest's mirror looks it up
	// before dispatching, skipping silently (not erroring) when it is still
	// NULL -- the CREATE mirror hasn't landed yet, or never will have.
	snWriteback *SNWritebackDispatcher
	snMirror    CallRequestService
}

// NewCallRequestService constructs a CallRequestService backed by Postgres
// (customer_call, migration 0073).
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
	// by this point. On success, the ServiceNow-side id this mirror creates
	// is persisted back onto the Postgres row (migration 0135's
	// customer_call.sn_sys_id) -- itself a second best-effort, asynchronous
	// write: if it fails, the row simply has no id yet, the same "not yet
	// mirrorable" state UpdateCallRequest's mirror already tolerates. This
	// follow-up write never turns a successful mirror into a recorded
	// sn_writeback_failures row.
	if s.snWriteback != nil {
		mirrorReq := req
		pgID := resp.CallRequest.ID
		s.snWriteback.Dispatch(ctx, "call_request", pgID, "create",
			map[string]any{"caseId": req.CaseID, "reason": req.Reason, "utcTimes": req.UTCTimes, "durationInMinutes": req.DurationMinutes},
			func(writeCtx context.Context) error {
				snResp, err := s.snMirror.CreateCallRequest(writeCtx, mirrorReq)
				if err != nil {
					return err
				}
				snSysID := uuidToSysid(snResp.CallRequest.ID)
				if setErr := s.repo.SetCallRequestSNSysID(writeCtx, pgID, snSysID); setErr != nil {
					slog.WarnContext(writeCtx, "sn writeback: call request created in ServiceNow but persisting its sys_id back onto Postgres failed -- update mirrors for this row will keep skipping until this is fixed",
						"callRequestId", pgID, "error", setErr)
				}
				return nil
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
	// assignment_group was deliberately skipped in migration 0073), so this
	// filter cannot be honored. Reject it rather than silently ignore it: an
	// ignored filter would widen the result set.
	if len(req.Filters.AssignmentTeamIDs) > 0 {
		return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "filters.assignmentTeamIds is not supported by this data source"}
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

	resp, err := s.repo.UpdateCallRequest(ctx, req, assigneeID, email)
	if err != nil {
		return domain.UpdateCallRequestResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only. Postgres has already committed by this point. The SN sys_id
	// lookup happens inside the dispatched closure (not here), since it must
	// run after any earlier queued job for this same call request (e.g. the
	// CREATE mirror that persists it) has already applied -- the dispatcher
	// serializes jobs per entity key precisely so this ordering holds.
	if s.snWriteback != nil {
		mirrorReq := req
		s.snWriteback.Dispatch(ctx, "call_request", req.ID, "update",
			map[string]any{"id": req.ID, "state": req.State},
			func(writeCtx context.Context) error {
				snSysID, lookupErr := s.repo.GetCallRequestSNSysID(writeCtx, req.ID)
				if lookupErr != nil {
					return lookupErr
				}
				if snSysID == nil || *snSysID == "" {
					slog.InfoContext(writeCtx, "sn writeback: call request update mirror skipped, no ServiceNow mapping stored yet",
						"callRequestId", req.ID)
					return nil
				}
				// mirrorReq.ID is replaced with the mapped ServiceNow sys_id
				// (as a UUID, since snMirror.UpdateCallRequest converts it
				// back via uuidToSysid). CaseID is cleared: when set, the SN
				// mirror does an extra GET-before-write
				// (verifyCallRequestBelongsToCase, paging through ServiceNow
				// search results) that this background mirror does not need
				// -- the case linkage was already established at CREATE
				// time and never changes on update.
				mirrorReq.ID = sysidToUUID(*snSysID)
				mirrorReq.CaseID = ""
				_, err := s.snMirror.UpdateCallRequest(writeCtx, mirrorReq)
				return err
			},
		)
	}

	return resp, nil
}
