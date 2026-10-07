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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type callRequestService struct {
	repo     repository.CallRequestRepository
	userRepo repository.UserRepository
	// snWriteback/snMirror back CreateCallRequest's and UpdateCallRequest's
	// ServiceNow mirror writes under DATA_SOURCE=postgres-servicenow-dual-write
	// -- both nil in every other mode. Set only via
	// NewCallRequestServiceWithSNWriteback.
	//
	// CreateCallRequest is ServiceNow-FIRST and SYNCHRONOUS under this data
	// source -- see createCallRequestSNFirst's own doc comment -- exactly
	// like case/deployment/deployed_product/incident/problem/change_request's
	// own CREATE paths, for the same orphan-avoidance reason. customer_call.id
	// is therefore the real ServiceNow sys_id (round-tripped through
	// sysidToUUID) from the moment the Postgres row exists, recoverable at any
	// time via uuidToSysid(id) -- no separate id-mapping column is needed for
	// a freshly created row.
	//
	// customer_call.sn_sys_id (migration 0135) and
	// Set/GetCallRequestSNSysID still exist and are still read by
	// UpdateCallRequest's mirror below, purely for backward compatibility
	// with rows created before this change under the old Postgres-first
	// behavior: those rows' id is a plain Postgres-generated UUID with no
	// ServiceNow counterpart, and sn_sys_id is the only record of the
	// mapping CreateCallRequest's old async mirror once persisted for them.
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
// becomes ServiceNow-first/synchronous (createCallRequestSNFirst) and
// UpdateCallRequest dispatches a best-effort, asynchronous ServiceNow mirror
// write onto mirror after the Postgres write commits -- see each method's own
// doc comment. A separate constructor rather than extending
// NewCallRequestService's own signature, same reasoning as
// NewCaseServiceWithSNWriteback's own doc comment.
func NewCallRequestServiceWithSNWriteback(repo repository.CallRequestRepository, userRepo repository.UserRepository, dispatcher *SNWritebackDispatcher, mirror CallRequestService) CallRequestService {
	return &callRequestService{repo: repo, userRepo: userRepo, snWriteback: dispatcher, snMirror: mirror}
}

// callRequestSNCreator is implemented by *snCallRequestService
// (createCallRequestSNFirstDetails, sn_call_request_service.go). A narrow
// interface -- rather than adding this to the full CallRequestService
// interface, which every implementer (including the plain, Postgres-backed
// callRequestService itself) would then have to satisfy -- named for exactly
// what createCallRequestSNFirst needs: the raw id/createdBy/createdOn
// ServiceNow assigned. Unlike deploymentSNCreator there is no number field:
// the SN integration service's call-request create response carries no
// number at all (confirmed against its response shape), so customer_call.number
// legitimately stays NULL for an SN-first-created row too, same as it already
// is today -- this is a pre-existing, separately tracked gap, not something
// this change fixes or regresses.
type callRequestSNCreator interface {
	createCallRequestSNFirstDetails(ctx context.Context, req domain.CreateCallRequestRequest) (id, createdBy string, createdOn time.Time, err error)
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

	if s.snMirror != nil {
		return s.createCallRequestSNFirst(ctx, req, user.ID, email)
	}

	return s.repo.CreateCallRequest(ctx, req, user.ID, email)
}

// createCallRequestSNFirst implements CreateCallRequest's
// DATA_SOURCE=postgres-servicenow-dual-write path: ServiceNow-FIRST and
// SYNCHRONOUS, exactly like deploymentService.createDeploymentSNFirst (and
// case/deployed_product/incident/problem/change_request's own CREATE paths)
// and for the same reason -- a Postgres-first create could leave a Postgres
// row with no ServiceNow counterpart if an async mirror write then failed, a
// PERMANENT orphan. Calling ServiceNow first, and only writing to Postgres
// once that succeeds, makes that orphan impossible.
//
// On success, id/createdBy/createdOn come from ServiceNow's own response and
// are used AS-IS for the Postgres insert
// (CallRequestRepository.CreateCallRequestFromServiceNow) rather than
// generated -- callerID/callerEmail are still the resolved caller's own
// identity (used for customer_call.opened_by_id/created_by, matching the
// plain-Postgres insert's own attribution), not anything ServiceNow returns.
func (s *callRequestService) createCallRequestSNFirst(ctx context.Context, req domain.CreateCallRequestRequest, callerID, callerEmail string) (domain.CreateCallRequestResponse, error) {
	creator, ok := s.snMirror.(callRequestSNCreator)
	if !ok {
		// Cannot happen with the real constructor (routes.go always passes a
		// *snCallRequestService as mirror), but a fake mirror in a test that
		// doesn't implement this would otherwise panic on the type assertion
		// below instead of failing cleanly.
		return domain.CreateCallRequestResponse{}, fmt.Errorf("call request SN mirror does not support create")
	}

	id, createdBy, createdOn, err := creator.createCallRequestSNFirstDetails(ctx, req)
	if err != nil {
		// ServiceNow never accepted the call request -- nothing is written to
		// Postgres at all, by construction (s.repo.CreateCallRequestFromServiceNow
		// is simply never called on this path). No orphan gets created.
		return domain.CreateCallRequestResponse{}, err
	}

	resp, err := s.repo.CreateCallRequestFromServiceNow(ctx, req, id, createdBy, createdOn, callerID)
	if err != nil {
		// ServiceNow already has the call request at this point -- this is
		// now real drift (ServiceNow has it, Postgres doesn't) needing
		// operator attention, not a safely-rejected request. Logged loudly
		// rather than only returned, same as createDeploymentSNFirst's
		// equivalent Postgres-insert-failure path.
		slog.ErrorContext(ctx, "sn create call request: ServiceNow call request created but the Postgres insert failed",
			"callRequestId", id, "caseId", req.CaseID, "error", err)
		return domain.CreateCallRequestResponse{}, err
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
	// The parent case's team is its account's CRE team on this data source; the
	// repository binds these as uuid[], so a malformed id is rejected here
	// (a 400 naming the field) rather than surfacing as a database cast error.
	if err := validateUUIDs("filters.assignmentTeamIds", req.Filters.AssignmentTeamIDs); err != nil {
		return domain.SearchCallRequestsResponse{}, err
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
	// resolution happens inside the dispatched closure (not here), since a
	// stored sn_sys_id lookup must run after any earlier queued job for this
	// same call request (e.g. a pre-this-change CREATE mirror that persists
	// it) has already applied -- the dispatcher serializes jobs per entity
	// key precisely so this ordering holds.
	if s.snWriteback != nil {
		mirrorReq := req
		s.snWriteback.Dispatch(ctx, "call_request", req.ID, "update",
			map[string]any{"id": req.ID, "state": req.State},
			func(writeCtx context.Context) error {
				// sn_sys_id first: authoritative for a row CREATE originally
				// wrote it for (the old Postgres-first behavior -- see
				// callRequestService's own doc comment). Falling back to
				// deriving it from req.ID via uuidToSysid covers every row
				// CreateCallRequest creates now (SN-first: req.ID already IS
				// the real sys_id round-tripped through sysidToUUID, so no
				// sn_sys_id column value is ever stored for it) -- without
				// this fallback, a freshly created row's update mirror would
				// skip forever, since GetCallRequestSNSysID never has
				// anything to return for it.
				snSysID, lookupErr := s.repo.GetCallRequestSNSysID(writeCtx, req.ID)
				if lookupErr != nil {
					return lookupErr
				}
				var resolvedSysID string
				if snSysID != nil && *snSysID != "" {
					resolvedSysID = *snSysID
				} else {
					resolvedSysID = uuidToSysid(req.ID)
				}
				// mirrorReq.ID is replaced with the mapped ServiceNow sys_id
				// (as a UUID, since snMirror.UpdateCallRequest converts it
				// back via uuidToSysid). CaseID is cleared: when set, the SN
				// mirror does an extra GET-before-write
				// (verifyCallRequestBelongsToCase, paging through ServiceNow
				// search results) that this background mirror does not need
				// -- the case linkage was already established at CREATE
				// time and never changes on update.
				mirrorReq.ID = sysidToUUID(resolvedSysID)
				mirrorReq.CaseID = ""
				_, err := s.snMirror.UpdateCallRequest(writeCtx, mirrorReq)
				return err
			},
		)
	}

	return resp, nil
}
