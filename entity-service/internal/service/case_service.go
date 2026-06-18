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

// Package service is declared in interfaces.go.
package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type caseService struct {
	repo     repository.CaseRepository
	userRepo repository.UserRepository
}

// NewCaseService constructs a CaseService backed by the given repositories.
func NewCaseService(repo repository.CaseRepository, userRepo repository.UserRepository) CaseService {
	return &caseService{repo: repo, userRepo: userRepo}
}

var validCaseSortField = map[domain.CaseSortField]bool{
	domain.CaseSortFieldCreatedAt: true,
	domain.CaseSortFieldUpdatedAt: true,
	domain.CaseSortFieldClosedAt:  true,
}

var validCaseSortOrder = map[domain.CaseSortOrder]bool{
	domain.CaseSortOrderAsc:  true,
	domain.CaseSortOrderDesc: true,
}

var validCaseState = map[domain.CaseState]bool{
	domain.CaseStateOpen:             true,
	domain.CaseStateWorkInProgress:   true,
	domain.CaseStateWaitingOnWSO2:    true,
	domain.CaseStateAwaitingInfo:     true,
	domain.CaseStateSolutionProposed: true,
	domain.CaseStateClosed:           true,
}

var validCasePriority = map[domain.CasePriority]bool{
	domain.CasePriorityCatastrophic: true,
	domain.CasePriorityCritical:     true,
	domain.CasePriorityHigh:         true,
	domain.CasePriorityMedium:       true,
	domain.CasePriorityLow:          true,
}

var validCaseIssueType = map[domain.CaseIssueType]bool{
	domain.CaseIssueTypeError:                  true,
	domain.CaseIssueTypePartialOutage:          true,
	domain.CaseIssueTypePerformanceDegradation: true,
	domain.CaseIssueTypeQuestion:               true,
	domain.CaseIssueTypeSecurityOrCompliance:   true,
	domain.CaseIssueTypeTotalOutage:            true,
}

// validateCreateCaseRequest validates fields common to all CreateCase data sources.
// UUID format of ID fields is not checked here — postgres IDs are UUIDs but
// ServiceNow IDs are opaque hex strings; callers add format checks as needed.
func validateCreateCaseRequest(req domain.CreateCaseRequest) error {
	if req.ProjectID == "" {
		return &apierror.ValidationError{Msg: "projectId is required"}
	}
	if req.DeploymentID == "" {
		return &apierror.ValidationError{Msg: "deploymentId is required"}
	}
	if req.DeployedProductID == "" {
		return &apierror.ValidationError{Msg: "deployedProductId is required"}
	}
	if req.Subject == "" {
		return &apierror.ValidationError{Msg: "subject is required"}
	}
	if req.Description == "" {
		return &apierror.ValidationError{Msg: "description is required"}
	}
	if !validCasePriority[req.Priority] {
		return &apierror.ValidationError{Msg: "priority contains invalid value: " + string(req.Priority)}
	}
	if !validCaseIssueType[req.IssueType] {
		return &apierror.ValidationError{Msg: "issueType contains invalid value: " + string(req.IssueType)}
	}
	return nil
}

// CreateCase implements CaseService.
func (s *caseService) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
	if err := validateCreateCaseRequest(req); err != nil {
		return domain.CreateCaseResponse{}, err
	}
	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.CreateCaseResponse{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.CreateCaseResponse{}, err
	}
	if err := validateUUIDs("deployedProductId", []string{req.DeployedProductID}); err != nil {
		return domain.CreateCaseResponse{}, err
	}
	if req.CreatedBy == "" {
		token := middleware.UserIDTokenFromContext(ctx)
		if token == "" {
			return domain.CreateCaseResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
		}
		email, err := emailFromJWT(token)
		if err != nil {
			return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
		}
		user, err := s.userRepo.GetUserByEmail(ctx, email)
		if err != nil {
			return domain.CreateCaseResponse{}, err
		}
		req.CreatedBy = user.ID
	}
	c, err := s.repo.CreateCase(ctx, req)
	if err != nil {
		return domain.CreateCaseResponse{}, err
	}
	return domain.CreateCaseResponse{
		Message: "Case created successfully.",
		Case: domain.CreateCaseDetails{
			ID:         c.ID,
			InternalID: c.InternalID,
			Number:     c.Number,
			CreatedBy:  c.CreatedBy,
			CreatedOn:  c.CreatedAt,
			State:      string(c.State),
		},
	}, nil
}

// GetCaseByID implements CaseService.
func (s *caseService) GetCaseByID(ctx context.Context, id string) (domain.CaseView, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.CaseView{}, err
	}
	return s.repo.GetCaseByID(ctx, id)
}

var validCommentType = map[domain.CommentType]bool{
	domain.CommentTypeWorkNote: true,
	domain.CommentTypeComment:  true,
	domain.CommentTypeActivity: true,
}

// CreateCaseComment implements CaseService.
func (s *caseService) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CreateCaseCommentResponse, error) {
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.CreateCaseCommentResponse{}, err
	}
	if !validCommentType[req.Type] {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(req.Type)}
	}
	if req.Content == "" {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "content is required"}
	}
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CreateCaseCommentResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.CreateCaseCommentResponse{}, err
	}
	req.CreatedBy = user.ID
	c, err := s.repo.CreateCaseComment(ctx, req)
	if err != nil {
		return domain.CreateCaseCommentResponse{}, err
	}
	return domain.CreateCaseCommentResponse{
		Message: "Comment created successfully",
		Comment: domain.CaseCommentDetail{
			ID:        c.ID,
			CreatedOn: c.CreatedAt,
			CreatedBy: user.Email,
		},
	}, nil
}

// SearchCaseComments implements CaseService.
func (s *caseService) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error) {
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}
	if req.Filters != nil && req.Filters.Type != nil && !validCommentType[*req.Filters.Type] {
		return domain.SearchCaseCommentsResponse{}, &apierror.ValidationError{Msg: "filters.type contains invalid value: " + string(*req.Filters.Type)}
	}
	comments, total, err := s.repo.SearchCaseComments(ctx, req)
	if err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}
	return domain.SearchCaseCommentsResponse{
		Comments: comments,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(comments) < total,
	}, nil
}

// UpdateCase implements CaseService.
func (s *caseService) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	if len(req.WatchList) > 0 || req.AssigneeEmail != nil {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "watchList and assigneeEmail are only supported for the ServiceNow data source"}
	}
	if req.State == nil && req.Priority == nil {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "at least one of state or priority must be provided"}
	}
	if req.State != nil && !validCaseState[*req.State] {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(*req.State)}
	}
	if req.Priority != nil && !validCasePriority[*req.Priority] {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "priority contains invalid value: " + string(*req.Priority)}
	}
	c, err := s.repo.UpdateCase(ctx, req)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	return domain.UpdateCaseResponse{
		Message: "Case updated successfully",
		Case: domain.UpdatedCase{
			ID:        c.ID,
			UpdatedOn: c.UpdatedAt,
			State:     c.State,
			Priority:  c.Priority,
		},
	}, nil
}

// SearchCases implements CaseService.
func (s *caseService) SearchCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateUUIDs("projectIds", req.Filters.ProjectIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateUUIDs("deploymentIds", req.Filters.DeploymentIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateUUIDs("deployedProductIds", req.Filters.DeployedProductIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	for _, s := range req.Filters.StateKeys {
		if !validCaseState[s] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "stateKeys contains invalid value: " + string(s)}
		}
	}
	for _, p := range req.Filters.PriorityKeys {
		if !validCasePriority[p] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "priorityKeys contains invalid value: " + string(p)}
		}
	}
	for _, it := range req.Filters.IssueTypeKeys {
		if !validCaseIssueType[it] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "issueTypeKeys contains invalid value: " + string(it)}
		}
	}

	if req.SortBy.Field == "" {
		req.SortBy.Field = domain.CaseSortFieldCreatedAt
	} else if !validCaseSortField[req.SortBy.Field] {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "sortBy.field must be one of: created_at, updated_at, closed_at"}
	}
	if req.SortBy.Order == "" {
		req.SortBy.Order = domain.CaseSortOrderDesc
	} else if !validCaseSortOrder[req.SortBy.Order] {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "sortBy.order must be one of: asc, desc"}
	}

	cases, total, err := s.repo.SearchCases(ctx, req)
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}

	return domain.SearchCasesResponse{
		Cases:   cases,
		Total:   total,
		Limit:   req.Pagination.Limit,
		Offset:  req.Pagination.Offset,
		HasMore: req.Pagination.Offset+len(cases) < total,
	}, nil
}

func (s *caseService) CreateCaseAttachment(_ context.Context, _ domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
	return domain.CreateAttachmentResponse{}, &apierror.ServiceUnavailableError{Msg: "attachments are only supported for the ServiceNow data source"}
}

func (s *caseService) SearchCaseAttachments(_ context.Context, _ domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error) {
	return domain.SearchAttachmentsResponse{}, &apierror.ServiceUnavailableError{Msg: "attachments are only supported for the ServiceNow data source"}
}

func (s *caseService) GetCaseAttachmentContent(_ context.Context, _, _ string) ([]byte, string, error) {
	return nil, "", &apierror.ServiceUnavailableError{Msg: "attachments are only supported for the ServiceNow data source"}
}
