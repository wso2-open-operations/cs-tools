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
}

// NewChangeRequestService constructs a ChangeRequestService backed by
// Postgres. CreateChangeRequest, GetChangeRequestApprovals, and
// DecideChangeRequestApproval always return a ServiceUnavailableError --
// see ChangeRequestRepository's own package doc comment for exactly why
// (no number-generation sequence; no approval-stage/approver tables).
func NewChangeRequestService(repo repository.ChangeRequestRepository) ChangeRequestService {
	return &changeRequestService{repo: repo}
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

	return domain.PatchChangeRequestResponse{
		Message:       "Change request updated successfully",
		ChangeRequest: cr,
	}, nil
}

// CreateChangeRequest implements ChangeRequestService. Not supported by the
// Postgres data source: work_item.number has no DB default and no backing
// sequence anywhere in migrations/, the same blocker
// CaseRepository.CreateCase has -- see that method's own doc comment for
// the full reasoning. Generating it requires a product decision (a new
// migration adding a sequence, vs. Go-side generation, and the exact number
// format) this change does not make unilaterally.
func (s *changeRequestService) CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
	return domain.CreateChangeRequestResponse{}, &apierror.ServiceUnavailableError{Msg: "creating change requests is not yet supported on the Postgres data source (no number-generation sequence)"}
}

// GetChangeRequestApprovals implements ChangeRequestService. Not supported
// by the Postgres data source: ChangeRequestApprovals models multiple
// approval stages, each with multiple approvers and per-approver status,
// and this schema has only one summary change_request.approval column --
// there is no approval-stage or approver table to serve this from.
func (s *changeRequestService) GetChangeRequestApprovals(ctx context.Context, id string) (domain.ChangeRequestApprovals, error) {
	return domain.ChangeRequestApprovals{}, &apierror.ServiceUnavailableError{Msg: "change request approval stages are only supported for the ServiceNow data source"}
}

// DecideChangeRequestApproval implements ChangeRequestService. Same
// limitation as GetChangeRequestApprovals: there is no per-approver
// approval record to decide on in this schema.
func (s *changeRequestService) DecideChangeRequestApproval(ctx context.Context, id, decision string) (domain.ChangeRequestApprovalDecisionResponse, error) {
	return domain.ChangeRequestApprovalDecisionResponse{}, &apierror.ServiceUnavailableError{Msg: "change request approval decisions are only supported for the ServiceNow data source"}
}
