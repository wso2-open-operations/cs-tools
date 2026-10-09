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
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// validChangeRequestState/validChangeRequestImpact (shared with
// sn_change_request_service.go) restrict to change_request_state_enum/
// change_request_impact_enum's real labels (migration 0043) -- both
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
	// snUsers is optional and only used by the dual-write create path, to check
	// that the people a change is assigned to exist in ServiceNow before it is
	// called -- see resolveServiceNowPeople. Set by WithChangeRequestSNUserLookup.
	snUsers snUserSearcher
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
	// customerGroupId / environmentIds are no longer accepted (the Customer
	// Group is derived from the project's registered contacts; a deployment
	// carries its environment): refused before anything else is looked at.
	if err := repository.RejectRemovedPatchFields(req); err != nil {
		return domain.PatchChangeRequestResponse{}, err
	}
	ids := []string{}
	for _, pp := range []**string{req.RequestedByID} {
		if pp != nil && *pp != nil {
			ids = append(ids, **pp)
		}
	}
	for _, ptr := range []*string{req.ProjectID, req.CaseID, req.DeploymentID, req.DeployedProductID, req.ServiceID, req.ServiceOfferingID, req.AssignedEngineerID} {
		if ptr != nil {
			ids = append(ids, *ptr)
		}
	}
	if err := validateUUIDs("id", ids); err != nil {
		return domain.PatchChangeRequestResponse{}, err
	}
	if err := validateChangeRequestScopeLists(derefStrings(req.DeploymentIDs), derefStrings(req.DeploymentProductIDs)); err != nil {
		return domain.PatchChangeRequestResponse{}, err
	}
	if req.Comment != nil && strings.TrimSpace(*req.Comment) == "" {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "comment must not be empty"}
	}
	if req.WorkNote != nil && strings.TrimSpace(*req.WorkNote) == "" {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "workNote must not be empty"}
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
		req.IsCustomerReviewed == nil && req.RequestApproval == nil &&
		req.IsPlanningVisibleToCustomers == nil &&
		req.ImplementationPlan == nil && req.Priority == nil && req.Category == nil &&
		req.RequestedByID == nil && req.AffectedServicesText == nil && req.AffectedComponentsText == nil &&
		req.RollbackDurationText == nil &&
		req.OnHold == nil && req.OnHoldReason == nil &&
		req.CustomerApprovalRequired == nil && req.CustomerReviewRequired == nil &&
		req.ConfirmCustomerUpdatedDate == nil &&
		req.DeploymentIDs == nil && req.DeploymentProductIDs == nil &&
		req.Comment == nil && req.WorkNote == nil && req.DurationInput == nil {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}
	// Accepted by the contract (and mirrored) but with no Postgres column
	// behind it: reject rather than silently drop it.
	if req.DurationInput != nil {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "durationInput is not supported on this data source"}
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
	//
	// customerApprovalRequired / customerReviewRequired are stripped first:
	// the creation form's two checkboxes have no field in ServiceNow's change
	// request API that this service can name (the scripted API only exposes
	// isCustomerApproved / isCustomerReviewed, the customer's OUTCOME, which
	// are a different thing), so they stay Postgres-only. A PATCH that carried
	// nothing else has nothing to mirror.
	//
	// deploymentIds / deploymentProductIds are stripped too: ServiceNow's change
	// request API carries a single deployment and a single deployed product on
	// a PATCH, and its deployment products are ServiceNow records whose ids are
	// not the ones PostgreSQL derives -- the field names and reference tables
	// behind them are not discoverable here, so rather than guess they stay
	// Postgres-only. projectId, category, comment and workNote are forwarded as
	// before. customerGroupId and environmentIds are no longer accepted at all
	// (refused above), so there is nothing of them to forward: the Customer
	// Group is derived from the project's registered contacts in PostgreSQL.
	mirrorReq := req
	mirrorReq.CustomerApprovalRequired, mirrorReq.CustomerReviewRequired = nil, nil
	mirrorReq.DeploymentIDs, mirrorReq.DeploymentProductIDs = nil, nil
	// The conversation about a time the customer proposed (confirmCustomerUpdatedDate,
	// expectedCustomerUpdatedOn) has no field in the previous system's change request API: it stays
	// PostgreSQL-only. What of it changes the change request itself is mirrored from what
	// PostgreSQL COMMITTED, and whether a window is a customer's PROPOSAL (never mirrored) is
	// decided from the request's CALLER (mirrorOfTheTimeConversation), never from a read of the
	// committed row alone: that read runs after the commit, in another transaction, and its
	// failure is logged and swallowed.
	mirrorReq.ConfirmCustomerUpdatedDate, mirrorReq.ExpectedCustomerUpdatedOn = nil, nil
	mirrorReq = mirrorOfTheTimeConversation(req, mirrorReq, cr, repository.IsExternalCaller(ctx))
	// PostgreSQL has accepted the window, in either of the layouts it takes (RFC
	// 3339, or "YYYY-MM-DD HH:MM:SS" in UTC); ServiceNow's API takes only the
	// second, so the mirror gets it in that one (what was sent in it is unchanged).
	if mirrorReq.PlannedStartOn != nil {
		v := repository.PlannedTimestampForServiceNow(*mirrorReq.PlannedStartOn)
		mirrorReq.PlannedStartOn = &v
	}
	if mirrorReq.PlannedEndOn != nil {
		v := repository.PlannedTimestampForServiceNow(*mirrorReq.PlannedEndOn)
		mirrorReq.PlannedEndOn = &v
	}
	// The window a customer's answer was given for is a precondition checked
	// against PostgreSQL only; there is nothing of it to mirror.
	mirrorReq.ExpectedPlannedStartOn, mirrorReq.ExpectedPlannedEndOn = nil, nil
	if s.snWriteback != nil && !reflect.DeepEqual(mirrorReq, domain.PatchChangeRequestRequest{}) {
		mirrorID := id
		s.snWriteback.Dispatch(ctx, "change_request", id, "patch", mirrorReq,
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

// mirrorOfTheTimeConversation adjusts the best-effort mirror of a PATCH to the previous system for the
// acts of the customer's-proposed-time conversation, and ONLY for them: every other PATCH
// mirrors exactly what it always did (mirror comes back unchanged).
//
// externalCaller is whether the PATCH came from a customer (repository.IsExternalCaller: the
// very test the repository used to decide what the request WAS), so what a window is, is
// decided by who sent it and not by anything read back afterwards.
//
//   - A PATCH from an external caller never mirrors its window. The repository accepts exactly
//     two things from a customer (classifyExternalPatch): their answer (isCustomerApproved /
//     isCustomerReviewed, mirrored as before) and a proposed window (plannedStartOn /
//     plannedEndOn), which PostgreSQL did NOT apply as the plan: it waits for WSO2 in
//     customer_updated_on, and the previous system has no field for it. Everything else is refused (403)
//     before this runs. This does not depend on the committed read model: the detail read
//     (GetChangeRequestByID -> fillCustomerProposal) runs after the commit, in a separate
//     transaction, logs and swallows its errors (CustomerProposal then stays nil) and can see
//     a conversation that has moved on (WSO2 answered in between), and a customer's proposed
//     time must never reach the previous system as the plan because of either.
//   - Accept proposed time (confirmCustomerUpdatedDate): the previous system has no field for the
//     answer, but the change moved to Scheduled with a new planned window, so that is what is
//     mirrored, read from what PostgreSQL committed. UNVERIFIED that the previous system accepts a
//     manual Scheduled out of Customer Approval: if it refuses, PostgreSQL stays committed and the
//     refused payload lands in the write-back failure record.
//   - A Re-schedule / counter-proposal / decline names {state: "authorize"} but the change STAYS in
//     Customer Approval: forwarding the state would put the previous system in Authorize while
//     PostgreSQL is not, so the state is dropped (the window, when there is one, is mirrored as
//     always).
//   - Second guard, for any caller: a window equal to the proposal the committed row still shows as
//     pending, that is not the committed plan, is the proposal and is not mirrored either. It can
//     only ever ADD to what the caller rule keeps out (it needs the read model to be there).
func mirrorOfTheTimeConversation(req, mirror domain.PatchChangeRequestRequest, committed domain.ChangeRequest, externalCaller bool) domain.PatchChangeRequestRequest {
	if externalCaller {
		mirror.PlannedStartOn, mirror.PlannedEndOn = nil, nil
		return mirror
	}
	if req.ConfirmCustomerUpdatedDate != nil {
		scheduled := domain.ChangeRequestStateScheduled
		return domain.PatchChangeRequestRequest{State: &scheduled, PlannedStartOn: committed.PlannedStartOn, PlannedEndOn: committed.PlannedEndOn}
	}
	if mirror.State != nil && strings.EqualFold(string(*mirror.State), string(domain.ChangeRequestStateAuthorize)) &&
		committed.State != nil && strings.EqualFold(*committed.State, string(domain.ChangeRequestStateCustomerApproval)) {
		mirror.State = nil
	}
	if mirror.State == nil && mirror.PlannedStartOn != nil && committed.CustomerProposal != nil && committed.CustomerProposal.Answer == "pending" &&
		sameMirroredInstant(*mirror.PlannedStartOn, committed.CustomerProposal.StartOn) &&
		(committed.PlannedStartOn == nil || !sameMirroredInstant(*mirror.PlannedStartOn, *committed.PlannedStartOn)) {
		mirror.PlannedStartOn, mirror.PlannedEndOn = nil, nil
	}
	return mirror
}

// sameMirroredInstant compares two planned timestamps as the mirror writes them (the previous
// system's layout, UTC, whole seconds).
func sameMirroredInstant(a, b string) bool {
	return repository.PlannedTimestampForServiceNow(a) == repository.PlannedTimestampForServiceNow(b)
}

// CreateChangeRequest implements ChangeRequestService.
//
// Under DATA_SOURCE=postgres-servicenow-dual-write (snMirror != nil), this
// delegates to createChangeRequestSNFirst; otherwise createChangeRequestPortal
// -- see each method's own doc comment.
func (s *changeRequestService) CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
	// customerGroupId / environmentIds are no longer accepted (see
	// repository.RejectRemovedCreateFields); refused before anything else,
	// including before ServiceNow is called.
	if err := repository.RejectRemovedCreateFields(req); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	if s.snMirror != nil {
		return s.createChangeRequestSNFirst(ctx, req)
	}
	return s.createChangeRequestPortal(ctx, req)
}

// createChangeRequestPortal implements CreateChangeRequest's plain-Postgres
// path (s.snMirror == nil, no ServiceNow at all) -- unblocked by migration
// 0140's next_portal_work_item_number(), the same product decision that used
// to defer this (see CLAUDE.md, "CreateCase and case numbers", and
// ChangeRequestRepository's own doc comment on CreateChangeRequestFromServiceNow
// for why a change request specifically needs no wso2ID the way case/incident
// do). createdBy is resolved from the caller's own JWT email claim -- the
// same middleware.UserIDTokenFromContext + emailFromJWT chain
// problemService.createProblemSNFirst already uses -- since there is no
// ServiceNow response to take it from on this path.
func (s *changeRequestService) createChangeRequestPortal(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CreateChangeRequestResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	createdBy, err := emailFromJWT(token)
	if err != nil {
		return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	// Same type check createChangeRequestSNFirst runs before calling
	// ServiceNow -- deterministic, no I/O, so there's no reason to defer it
	// to the repository's own identical check.
	// The type is mandatory and must be standard/normal/emergency: it decides
	// the whole approval flow (see repository.ValidateCreateChangeRequestType).
	if err := repository.ValidateCreateChangeRequestType(req.Type); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	if !repository.ChangeRequestTypeSupported(*req.Type) {
		return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("type %q is not supported on the PostgreSQL data source", *req.Type)}
	}
	// An Emergency change takes no customer step: refused here, before the previous
	// system is called on the dual-write path, where a refusal after the fact would
	// strand a record there with no PostgreSQL row.
	if err := repository.ValidateCreateChangeRequestCustomerGates(req.Type, req.CustomerApprovalRequired, req.CustomerReviewRequired); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	if err := validateChangeRequestCreateScope(req); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	return s.repo.CreateChangeRequest(ctx, req, createdBy)
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
	// The type is mandatory and must be standard/normal/emergency: it decides
	// the whole approval flow (see repository.ValidateCreateChangeRequestType).
	if err := repository.ValidateCreateChangeRequestType(req.Type); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	if !repository.ChangeRequestTypeSupported(*req.Type) {
		return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("type %q is not supported on the PostgreSQL data source", *req.Type)}
	}
	// An Emergency change takes no customer step: refused here, before the previous
	// system is called on the dual-write path, where a refusal after the fact would
	// strand a record there with no PostgreSQL row.
	if err := repository.ValidateCreateChangeRequestCustomerGates(req.Type, req.CustomerApprovalRequired, req.CustomerReviewRequired); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	if err := validateChangeRequestCreateScope(req); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	// The planned window is validated BEFORE the previous system is called, exactly as the
	// plain-Postgres create does it (repository.NormalizeCreatePlannedWindow): the
	// raw text used to reach both the previous system and PostgreSQL's own date parser
	// ('tomorrow', 'infinity', a year in the thousands). PostgreSQL then gets the
	// request as sent (its create normalises the window again, to an instant) and the
	// previous system the window in the layout ITS service takes, as the PATCH mirror
	// does it (the mirror's timestamp conversion: RFC 3339 becomes "YYYY-MM-DD HH:MM:SS"
	// in UTC, whole seconds). The service in front
	// of it converts RFC 3339 itself too (snPlannedTimestamp), but it refuses a
	// zoneless value with a fractional second, which PostgreSQL accepts, so what
	// PostgreSQL accepted is converted here and never reaches it as typed.
	if _, err := repository.NormalizeCreatePlannedWindow(req); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	// The project / deployments / deployment products are validated BEFORE
	// ServiceNow is called: once ServiceNow has created the change request, a
	// refusal on the Postgres side would strand it there.
	// The assignment group is checked here too: a group that is not one of
	// "group" (a hand-made team the picker used to list) is not a ServiceNow group
	// either, and ServiceNow answers it with a bare 404.
	if _, err := s.repo.ValidateChangeRequestLinks(ctx, domain.ChangeRequestLinkSelection{
		ProjectID: req.ProjectID, AssignmentGroupID: req.GroupID,
		DeploymentIDs: req.DeploymentIDs, DeploymentProductIDs: req.DeploymentProductIDs,
	}); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	// ServiceNow gets the request without the scope fields only PostgreSQL
	// models: projectId and deploymentIds have no field on ServiceNow's create
	// payload, and deployment products are PostgreSQL-derived ids that are not
	// ServiceNow records (see PatchChangeRequest's mirror comment). category,
	// comment and workNote are forwarded; customerGroupId / environmentIds are
	// refused up front (nothing to forward: the Customer Group is derived from
	// the project's registered contacts).
	mirrorReq := req
	mirrorReq.ProjectID, mirrorReq.DeploymentIDs, mirrorReq.DeploymentProductIDs = nil, nil, nil
	if mirrorReq.PlannedStartDate != nil {
		v := repository.PlannedTimestampForServiceNow(*mirrorReq.PlannedStartDate)
		mirrorReq.PlannedStartDate = &v
	}
	if mirrorReq.PlannedEndDate != nil {
		v := repository.PlannedTimestampForServiceNow(*mirrorReq.PlannedEndDate)
		mirrorReq.PlannedEndDate = &v
	}
	// The people named are made ServiceNow ones (or refused in words) before
	// ServiceNow is called: its answer to an unknown user is a bare 404.
	if err := s.resolveServiceNowPeople(ctx, &mirrorReq); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	snResp, err := s.snMirror.CreateChangeRequest(ctx, mirrorReq)
	if err != nil {
		// ServiceNow never accepted the change request -- nothing is
		// written to Postgres at all, by construction
		// (s.repo.CreateChangeRequestFromServiceNow is simply never called
		// on this path). No orphan gets created.
		//
		// A "not found" here cannot be about the change request (it does not
		// exist yet): ServiceNow does not know a record it was handed. The portal
		// shows a 404 as "The requested resource was not found!", which says
		// nothing, so it is reported as the 400 it is.
		var notFound *apierror.NotFoundError
		if errors.As(err, &notFound) {
			slog.WarnContext(ctx, "sn create change request: ServiceNow did not recognise a record the change request refers to", "error", err)
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: serviceNowUnknownRecordMsg}
		}
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

// serviceNowUnknownRecordMsg is what a create is refused with when ServiceNow answers
// "not found": one of the records the form refers to is not one it knows, and it does
// not say which. The people and the assignment group are checked before ServiceNow is
// called, so what is left is mostly the service, the service offering and the
// configuration item.
const serviceNowUnknownRecordMsg = "The change request was not created: ServiceNow did not recognise one of the records it refers to " +
	"(the assignment group, the person it is assigned to, the requester, the service, the service offering or the configuration item). " +
	"Change one of those fields and try again."

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
	return s.repo.GetChangeRequestApprovals(s.withApprovalViewer(ctx), id)
}

// withApprovalViewer makes sure the caller identity on ctx names the person
// reading the approvals, so the repository can compute each approver row's
// CanDecide for them.
//
// AccessService.ResolveScope only fills SearchScope.ViewerEmail on some
// branches (a customer-scoped user, or the CSM portal backend client with a
// matching-domain user). An internal user resolved from the user token alone
// (scopeForUser's internal branch), or any caller behind an M2M client id,
// comes back Unrestricted with an EMPTY ViewerEmail -- and the repository's
// markCanDecide treats an empty ViewerEmail as "viewer unknown" and leaves
// every canDecide false, so the portal rendered Approve/Reject disabled for
// the very approver the row belongs to. DecideChangeRequestApproval
// identifies its caller from the x-user-id-token (currentUser), so the same
// source is used here, keeping "may decide" and "decided" consistent.
//
// Only fills a missing email and never invents an identity: with none on ctx
// the repository still fails closed. ViewerEmail on an Unrestricted scope has
// no effect on row visibility.
func (s *changeRequestService) withApprovalViewer(ctx context.Context) context.Context {
	scope, ok := repository.CallerIdentityFromContext(ctx)
	if !ok || strings.TrimSpace(scope.ViewerEmail) != "" {
		return ctx
	}
	email := auth.IdentityFromContext(ctx).UserEmail
	if email == "" {
		if token := middleware.UserIDTokenFromContext(ctx); token != "" {
			email, _ = emailFromJWT(token)
		}
	}
	if strings.TrimSpace(email) == "" {
		return ctx
	}
	scope.ViewerEmail = email
	return repository.WithCallerIdentity(ctx, scope)
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
	// and approval_stage_approver.state (renamed from status by migration
	// 0138, values UPPER_SNAKE_CASE) stores them uppercased -- the repository
	// does that one conversion (strings.ToUpper at the UPDATE) -- so there is
	// no separate translation table to keep in lockstep here.
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

// GetChangeRequestLinkOptions implements ChangeRequestService.
func (s *changeRequestService) GetChangeRequestLinkOptions(ctx context.Context, req domain.ChangeRequestLinkOptionsRequest) (domain.ChangeRequestLinkOptionsResponse, error) {
	if strings.TrimSpace(req.ProjectID) == "" {
		return domain.ChangeRequestLinkOptionsResponse{}, &apierror.ValidationError{Msg: "projectId is required"}
	}
	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.ChangeRequestLinkOptionsResponse{}, err
	}
	if err := validateChangeRequestScopeLists(req.DeploymentIDs, nil); err != nil {
		return domain.ChangeRequestLinkOptionsResponse{}, err
	}
	return s.repo.GetChangeRequestLinkOptions(ctx, req)
}

// maxChangeRequestScopeIDs caps each id list of the customer-scope fields.
const maxChangeRequestScopeIDs = 100

func derefStrings(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}

// validateChangeRequestScopeLists checks the shape of the deployment,
// and deployment-product id lists: UUIDs, bounded. Whether they
// fit together is the repository's to judge.
func validateChangeRequestScopeLists(deploymentIDs, deploymentProductIDs []string) error {
	for _, l := range []struct {
		field string
		ids   []string
	}{{"deploymentIds", deploymentIDs}, {"deploymentProductIds", deploymentProductIDs}} {
		if len(l.ids) > maxChangeRequestScopeIDs {
			return &apierror.ValidationError{Msg: fmt.Sprintf("%s must contain at most %d entries", l.field, maxChangeRequestScopeIDs)}
		}
		if err := validateUUIDs(l.field, l.ids); err != nil {
			return err
		}
	}
	return nil
}

// validateChangeRequestCreateScope checks the shape of the create request's
// customer-scope fields (the repository judges how they fit together).
func validateChangeRequestCreateScope(req domain.CreateChangeRequestRequest) error {
	if req.ProjectID != nil {
		if err := validateUUIDs("projectId", []string{*req.ProjectID}); err != nil {
			return err
		}
	}
	return validateChangeRequestScopeLists(req.DeploymentIDs, req.DeploymentProductIDs)
}
