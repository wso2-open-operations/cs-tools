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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// validChangeRequestState/validChangeRequestImpact (shared with
// sn_change_request_service.go) restrict to change_request_state_enum/
// change_request_impact_enum's real labels (migration 000047) -- both
// domain enums happen to match their real column 1:1 (case-folded), so
// there's nothing Postgres-specific to add for either. change_request also
// has a risk column (change_request_risk_enum) and a narrower category
// column (change_request_category_enum has only 9 labels, not
// domain.ChangeRequestCategory's full 13 -- "regular_release_cloud",
// "hotfix_release_cloud", "devops", and "cloud_computing" have no real
// column value), neither validated here since CreateChangeRequest (the only
// method that would accept either) has no Postgres implementation yet -- see
// its own doc comment below.

type changeRequestService struct {
	repo repository.ChangeRequestRepository
	// userRepo resolves the calling user's UUID from their x-user-id-token
	// for DecideChangeRequestApproval -- approval_stage_approver.
	// approver_user_id is a "user".id FK, not an email, so the JWT's email
	// alone isn't enough to match a row. See currentUser's own doc comment.
	userRepo repository.UserRepository
	// snMirror is nil in every mode except DATA_SOURCE=postgres-servicenow-dual-write
	// (config.DataSourcePostgresServiceNowDualWrite) -- see
	// NewChangeRequestServiceWithSNMirror's own doc comment. When set,
	// CreateChangeRequest delegates to createChangeRequestSNFirst instead of
	// the plain Postgres path's ServiceUnavailableError below, mirroring
	// incidentService's identical snMirror-gated branch for CreateIncident.
	//
	// snWriteback is additionally set (via NewChangeRequestServiceWithSNWriteback)
	// for PatchChangeRequest's best-effort, asynchronous ServiceNow mirror
	// write -- see that method's own doc comment. Safe to mirror by id here,
	// unlike call_request/time_card's writeback: change request CREATE is
	// ServiceNow-first under this data source (createChangeRequestSNFirst),
	// so a change request's Postgres id IS the real ServiceNow sys_id
	// round-tripped through sysidToUUID -- uuidToSysid(id) always resolves to
	// the correct ServiceNow record.
	snMirror    ChangeRequestService
	snWriteback *SNWritebackDispatcher
}

// NewChangeRequestService constructs a ChangeRequestService backed by
// Postgres. CreateChangeRequest always returns a ServiceUnavailableError --
// see ChangeRequestRepository's own package doc comment for exactly why (no
// number-generation sequence). GetChangeRequestApprovals/
// DecideChangeRequestApproval are fully implemented against Postgres in
// every mode -- see their own doc comments below.
func NewChangeRequestService(repo repository.ChangeRequestRepository, userRepo repository.UserRepository) ChangeRequestService {
	return &changeRequestService{repo: repo, userRepo: userRepo}
}

// NewChangeRequestServiceWithSNMirror is NewChangeRequestService plus the
// wiring DATA_SOURCE=postgres-servicenow-dual-write needs for change request
// CREATE: a synchronous, ServiceNow-first creation path -- see
// createChangeRequestSNFirst's own doc comment for the full reasoning
// (identical to incidentService.createIncidentSNFirst's: a Postgres-first
// async create could leave a permanent orphan). This mode has no change
// request UPDATE mirror -- PatchChangeRequest stays exactly as it is in
// every other mode; only CREATE is in scope for this pilot extension.
//
// mirror is the ServiceNow-backed ChangeRequestService (from
// NewServiceNowChangeRequestService) whose CreateChangeRequest performs the
// real ServiceNow POST. It is never made the active ChangeRequestService
// here -- reads always stay on Postgres in this mode.
func NewChangeRequestServiceWithSNMirror(repo repository.ChangeRequestRepository, userRepo repository.UserRepository, mirror ChangeRequestService) ChangeRequestService {
	return &changeRequestService{repo: repo, userRepo: userRepo, snMirror: mirror}
}

// NewChangeRequestServiceWithSNWriteback is NewChangeRequestServiceWithSNMirror
// plus PatchChangeRequest's best-effort, asynchronous ServiceNow mirror write
// -- see PatchChangeRequest's own doc comment, and changeRequestService's own
// doc comment on snWriteback for why this is safe to do by id (unlike
// call_request/time_card). A separate constructor rather than extending
// NewChangeRequestServiceWithSNMirror's own signature: several existing
// tests construct that one directly with dispatcher/writeback out of scope,
// and keeping it as-is means they keep working unchanged.
func NewChangeRequestServiceWithSNWriteback(repo repository.ChangeRequestRepository, userRepo repository.UserRepository, mirror ChangeRequestService, dispatcher *SNWritebackDispatcher) ChangeRequestService {
	return &changeRequestService{repo: repo, userRepo: userRepo, snMirror: mirror, snWriteback: dispatcher}
}

// currentUser resolves the caller's full user record from their
// x-user-id-token -- same mechanism as timeCardService.currentUserID
// (case_service.go's CreateCaseComment originates it), except this returns
// the whole domain.User rather than just the id: DecideChangeRequestApproval
// needs both the id (to match approval_stage_approver.approver_user_id) and
// the email (to stamp updated_by, matching PatchChangeRequest's own
// actorEmail convention) from a single lookup.
func (s *changeRequestService) currentUser(ctx context.Context) (domain.User, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.User{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return domain.User{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	return s.userRepo.GetUserByEmail(ctx, email)
}

func validateChangeRequestFilters(f domain.SearchChangeRequestsFilters) error {
	if err := validateUUIDs("filters.projectIds", f.ProjectIDs); err != nil {
		return err
	}
	for _, s := range f.States {
		if !validChangeRequestState[s] {
			return &apierror.ValidationError{Msg: "filters.states contains invalid value: " + string(s)}
		}
	}
	for _, i := range f.Impacts {
		if !validChangeRequestImpact[i] {
			return &apierror.ValidationError{Msg: "filters.impacts contains invalid value: " + string(i)}
		}
	}
	if err := validateSearchQuery(f.SearchQuery); err != nil {
		return err
	}
	return nil
}

// SearchChangeRequests implements ChangeRequestService.
func (s *changeRequestService) SearchChangeRequests(ctx context.Context, req domain.SearchChangeRequestsRequest) (domain.SearchChangeRequestsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchChangeRequestsResponse{}, err
	}
	if err := validateChangeRequestFilters(req.Filters); err != nil {
		return domain.SearchChangeRequestsResponse{}, err
	}
	// Reuses validChangeRequestSortField/validChangeRequestSortOrder (defined
	// in sn_change_request_service.go) so an invalid sortBy is a 400 on both
	// data sources instead of silently falling back to created_on DESC here.
	if req.SortBy.Field != "" && !validChangeRequestSortField[req.SortBy.Field] {
		return domain.SearchChangeRequestsResponse{}, &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(req.SortBy.Field)}
	}
	if req.SortBy.Order != "" && !validChangeRequestSortOrder[req.SortBy.Order] {
		return domain.SearchChangeRequestsResponse{}, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(req.SortBy.Order)}
	}
	parsed, err := ParseChangeRequestFieldFilters(req.Filters.Filters, time.Now().UTC())
	if err != nil {
		return domain.SearchChangeRequestsResponse{}, err
	}
	if parsed.CreatedStartDate != nil && parsed.CreatedEndDate != nil && parsed.CreatedEndDate.Before(*parsed.CreatedStartDate) {
		return domain.SearchChangeRequestsResponse{}, &apierror.ValidationError{Msg: "filters: createdOn lte bound must not be before its gte bound"}
	}

	views, total, err := s.repo.SearchChangeRequests(ctx, req, parsed.CreatedStartDate, parsed.CreatedEndDate, parsed.Approval, parsed.AssignmentGroupIDs)
	if err != nil {
		return domain.SearchChangeRequestsResponse{}, err
	}

	return domain.SearchChangeRequestsResponse{
		ChangeRequests: views,
		Total:          total,
		Limit:          req.Pagination.Limit,
		Offset:         req.Pagination.Offset,
	}, nil
}

const defaultChangeRequestMaxGroups = 10

// AggregateChangeRequests implements ChangeRequestService.
func (s *changeRequestService) AggregateChangeRequests(ctx context.Context, req domain.AggregateChangeRequestsRequest) (domain.AggregateResponse, error) {
	if err := validateChangeRequestFilters(req.Filters); err != nil {
		return domain.AggregateResponse{}, err
	}
	if req.GroupBy == "" {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy is required"}
	}
	maxGroups := req.MaxGroups
	if maxGroups <= 0 {
		maxGroups = defaultChangeRequestMaxGroups
	}
	parsed, err := ParseChangeRequestFieldFilters(req.Filters.Filters, time.Now().UTC())
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	return s.repo.AggregateChangeRequests(ctx, req, req.GroupBy, maxGroups, parsed.CreatedStartDate, parsed.CreatedEndDate, parsed.Approval)
}

// GetChangeRequest implements ChangeRequestService.
func (s *changeRequestService) GetChangeRequest(ctx context.Context, id string) (domain.ChangeRequest, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ChangeRequest{}, err
	}
	return s.repo.GetChangeRequestByID(ctx, id)
}

// PatchChangeRequest implements ChangeRequestService.
func (s *changeRequestService) PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.PatchChangeRequestResponse{}, err
	}
	ids := []string{}
	for _, ptr := range []*string{req.ProjectID, req.CaseID, req.DeploymentID, req.DeployedProductID, req.ServiceID, req.ServiceOfferingID, req.AssignedEngineerID} {
		if ptr != nil {
			ids = append(ids, *ptr)
		}
	}
	if err := validateUUIDs("id", ids); err != nil {
		return domain.PatchChangeRequestResponse{}, err
	}
	if req.Impact != nil && !validChangeRequestImpact[*req.Impact] {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "impact contains invalid value: " + string(*req.Impact)}
	}
	if req.State != nil && !validChangeRequestState[*req.State] {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(*req.State)}
	}
	if req.Title == nil && req.Description == nil && req.ProjectID == nil && req.CaseID == nil &&
		req.DeploymentID == nil && req.DeployedProductID == nil && req.AssignedEngineerID == nil &&
		req.AssignedTeamID == nil && req.PlannedStartOn == nil && req.PlannedEndOn == nil &&
		req.Impact == nil && req.State == nil && req.Type == nil && req.Justification == nil &&
		req.ImpactDescription == nil && req.ServiceOutage == nil && req.CommunicationPlan == nil &&
		req.RollbackPlan == nil && req.TestPlan == nil && req.IsCustomerApproved == nil &&
		req.IsCustomerReviewed == nil && req.RequestApproval == nil {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.PatchChangeRequestResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}

	cr, err := s.repo.PatchChangeRequest(ctx, id, req, email)
	if err != nil {
		return domain.PatchChangeRequestResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback is nil otherwise -- see changeRequestService's own
	// doc comment on snWriteback). Postgres has already committed by this
	// point; this fires after, asynchronously, and never affects this
	// response. Safe to mirror by id -- see that same doc comment for why.
	// snMirror.PatchChangeRequest (sn_change_request_service.go) is already
	// a bare PATCH with no GET-before-write or notification side effects, so
	// it's called directly here rather than through a narrower interface
	// (unlike case's UpdateCase, which needed patchCaseFields specifically
	// to avoid snCaseService.UpdateCase's own read-before-write behavior).
	if s.snWriteback != nil {
		mirrorID, mirrorReq := id, req
		s.snWriteback.Dispatch(ctx, "change_request", id, "patch", req,
			func(writeCtx context.Context) error {
				_, err := s.snMirror.PatchChangeRequest(writeCtx, mirrorID, mirrorReq)
				return err
			},
		)
	}

	return domain.PatchChangeRequestResponse{
		Message:       "Change request updated successfully",
		ChangeRequest: cr,
	}, nil
}

// CreateChangeRequest implements ChangeRequestService.
//
// Under DATA_SOURCE=postgres-servicenow-dual-write (snMirror != nil), this
// delegates to createChangeRequestSNFirst instead of the plain Postgres
// path's ServiceUnavailableError below -- see that method's own doc comment.
func (s *changeRequestService) CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
	if s.snMirror != nil {
		return s.createChangeRequestSNFirst(ctx, req)
	}
	// CreateChangeRequest is not supported for the plain PostgreSQL data
	// source: like CaseRepository.CreateCase, work_item.number has no DB
	// default and no backing sequence anywhere in migrations/. Generating it
	// requires a product decision (a new migration adding a sequence, vs.
	// Go-side generation, and the exact number format) this change does not
	// make unilaterally.
	return domain.CreateChangeRequestResponse{}, &apierror.ServiceUnavailableError{Msg: "creating change requests is not yet supported on the Postgres data source (no number-generation sequence)"}
}

// createChangeRequestSNFirst implements CreateChangeRequest's
// DATA_SOURCE=postgres-servicenow-dual-write path: ServiceNow-FIRST and
// SYNCHRONOUS, exactly mirroring incidentService.createIncidentSNFirst's
// reasoning -- see that method's own doc comment for why CREATE must be
// ServiceNow-first rather than Postgres-first-and-async: a Postgres row with
// no ServiceNow counterpart would be a PERMANENT orphan (ServiceNow is still
// the real backing store this platform proxies most writes onto), while an
// async-after-commit UPDATE has no equivalent failure mode.
//
// The ServiceNow call is made exactly once, with no internal retry: retrying
// here risks creating a second, duplicate ServiceNow record if ServiceNow's
// create actually succeeded but the HTTP response back to entity-service was
// lost (timeout/network blip) -- entity-service has no way to distinguish
// that from a real failure, and retry policy for that case belongs to the
// caller, not this layer.
//
// On success, id/number/createdBy come from ServiceNow's own response and
// are used AS-IS for the Postgres insert
// (ChangeRequestRepository.CreateChangeRequestFromServiceNow) rather than
// generated -- see that method's own doc comment for why there is no wso2ID
// parameter here, unlike case's equivalent.
func (s *changeRequestService) createChangeRequestSNFirst(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
	// Reject an unsupported type before ServiceNow ever sees the request --
	// this check is deterministic and needs no I/O, so there's no reason to
	// defer it to CreateChangeRequestFromServiceNow's own check (which runs
	// only after ServiceNow already accepted the create, at which point
	// ServiceNow would keep an orphan with no Postgres row).
	if req.Type != nil && !repository.ChangeRequestTypeSupported(*req.Type) {
		return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("type %q is not supported on the PostgreSQL data source", *req.Type)}
	}
	snResp, err := s.snMirror.CreateChangeRequest(ctx, req)
	if err != nil {
		// ServiceNow never accepted the change request -- nothing is
		// written to Postgres at all, by construction
		// (s.repo.CreateChangeRequestFromServiceNow is simply never called
		// on this path). No orphan gets created.
		return domain.CreateChangeRequestResponse{}, err
	}

	resp, err := s.repo.CreateChangeRequestFromServiceNow(ctx, req, snResp.ChangeRequest.ID, snResp.ChangeRequest.Number, snResp.ChangeRequest.CreatedBy)
	if err != nil {
		// ServiceNow already has the change request at this point -- this
		// is now real drift (ServiceNow has it, Postgres doesn't) needing
		// operator attention, not a safely-rejected request. Logged loudly
		// rather than only returned, same convention as
		// incidentService.createIncidentSNFirst's identical failure shape.
		slog.ErrorContext(ctx, "sn create change request: ServiceNow change request created but the Postgres insert failed",
			"changeRequestId", snResp.ChangeRequest.ID, "snNumber", snResp.ChangeRequest.Number, "error", err)
		return domain.CreateChangeRequestResponse{}, err
	}
	return resp, nil
}

// GetChangeRequestApprovals implements ChangeRequestService. Reads always
// stay on Postgres regardless of data source -- config.go's own doc comment
// on config.DataSourcePostgresServiceNowDualWrite says "ServiceNow is never
// read from in this mode", and changeRequestService's other read path
// (GetChangeRequest) already follows that same rule -- so there is no
// snMirror branching here, unlike CreateChangeRequest.
func (s *changeRequestService) GetChangeRequestApprovals(ctx context.Context, id string) (domain.ChangeRequestApprovals, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ChangeRequestApprovals{}, err
	}
	return s.repo.GetChangeRequestApprovals(ctx, id)
}

// DecideChangeRequestApproval implements ChangeRequestService.
//
// Postgres-FIRST, with a best-effort ASYNCHRONOUS ServiceNow mirror under
// DATA_SOURCE=postgres-servicenow-dual-write (snWriteback != nil) --
// deliberately the opposite ordering from CreateChangeRequest's
// createChangeRequestSNFirst. A synchronous SN-first approach was
// considered (matching CREATE's reasoning that an orphan is worse than a
// stale mirror) and rejected: unlike CREATE, ServiceNow's own decideApproval
// has cascade side effects on the change request's overall state (its
// business rule may move the change request out of "assess"/"authorize"
// entirely once the last approver in a stage responds), and this platform
// has no visibility into that cascade from here -- a synchronous call would
// have to either duplicate ServiceNow's own state machine to know what
// changed, or block the response on a round trip whose result this method
// can't fully act on anyway. Postgres-first accepts the same drift window
// PatchChangeRequest already accepts (a mirror failure leaves ServiceNow's
// row stale until sn_writeback_failures is backfilled) rather than take on
// that larger, harder-to-scope risk.
func (s *changeRequestService) DecideChangeRequestApproval(ctx context.Context, id, decision string) (domain.ChangeRequestApprovalDecisionResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ChangeRequestApprovalDecisionResponse{}, err
	}
	// changeRequestApprovalDecisions (sn_change_request_service.go) is
	// reused directly rather than redeclared: both data sources accept
	// exactly the same two request-level values ("approved"/"rejected"),
	// and approval_stage_approver.status stores those same raw strings
	// verbatim (migration 000087's own comment), so there is no separate
	// translation table to keep in lockstep here.
	if !changeRequestApprovalDecisions[decision] {
		return domain.ChangeRequestApprovalDecisionResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid decision %q", decision)}
	}

	user, err := s.currentUser(ctx)
	if err != nil {
		return domain.ChangeRequestApprovalDecisionResponse{}, err
	}

	approvalID, err := s.repo.DecideChangeRequestApproval(ctx, id, user.ID, decision, user.Email)
	if err != nil {
		return domain.ChangeRequestApprovalDecisionResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback is nil otherwise) -- exact same shape as
	// PatchChangeRequest's own dispatch above: Postgres has already
	// committed by this point, this fires after, asynchronously, and never
	// affects this response. Safe to mirror by id for the same reason
	// PatchChangeRequest's dispatch is (see changeRequestService's own doc
	// comment on snWriteback): change request CREATE is ServiceNow-first
	// under this data source, so id round-trips to the real ServiceNow
	// sys_id via uuidToSysid.
	if s.snWriteback != nil {
		mirrorID, mirrorDecision := id, decision
		s.snWriteback.Dispatch(ctx, "change_request", id, "approval_decision", decision,
			func(writeCtx context.Context) error {
				_, err := s.snMirror.DecideChangeRequestApproval(writeCtx, mirrorID, mirrorDecision)
				return err
			},
		)
	}

	return domain.ChangeRequestApprovalDecisionResponse{
		ID:    approvalID,
		State: decision,
	}, nil
}
