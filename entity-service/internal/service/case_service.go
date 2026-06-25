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
	"fmt"

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
	domain.CaseSortFieldCreatedOn: true,
	domain.CaseSortFieldUpdatedOn: true,
	domain.CaseSortFieldSeverity:  true,
	domain.CaseSortFieldState:     true,
}

var validCaseType = map[string]bool{
	"case":                     true,
	"service_request":          true,
	"security_report_analysis": true,
}

var validEngagementType = map[domain.EngagementType]bool{
	domain.EngagementTypeMigration:             true,
	domain.EngagementTypeConsultancy:           true,
	domain.EngagementTypeNewFeatureImprovement: true,
	domain.EngagementTypeFollowUp:              true,
	domain.EngagementTypeOnboarding:            true,
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
	domain.CaseStateReopened:         true,
	domain.CaseStateSolutionProposed: true,
	domain.CaseStateClosed:           true,
}

var validCaseSeverity = map[domain.CaseSeverity]bool{
	domain.CaseSeverityCatastrophic: true,
	domain.CaseSeverityCritical:     true,
	domain.CaseSeverityHigh:         true,
	domain.CaseSeverityMedium:       true,
	domain.CaseSeverityLow:          true,
}

var validCaseIssueType = map[domain.CaseIssueType]bool{
	domain.CaseIssueTypeError:                  true,
	domain.CaseIssueTypePartialOutage:          true,
	domain.CaseIssueTypePerformanceDegradation: true,
	domain.CaseIssueTypeQuestion:               true,
	domain.CaseIssueTypeSecurityOrCompliance:   true,
	domain.CaseIssueTypeTotalOutage:            true,
}

var validCaseWorkState = map[domain.CaseWorkState]bool{
	domain.CaseWorkStateOngoing: true,
	domain.CaseWorkStatePaused:  true,
}

// validateCreateCaseRequest validates fields common to all CreateCase data sources.
// UUID format of ID fields is not checked here — postgres IDs are UUIDs but
// ServiceNow IDs are opaque hex strings; callers add format checks as needed.
func validateCreateCaseRequest(req domain.CreateCaseRequest) error {
	if req.Type == "" {
		return &apierror.ValidationError{Msg: "type is required"}
	}
	if !validCaseType[req.Type] {
		return &apierror.ValidationError{Msg: "type contains invalid value: " + req.Type}
	}
	if req.ProjectID == "" {
		return &apierror.ValidationError{Msg: "projectId is required"}
	}
	if req.DeploymentID == "" {
		return &apierror.ValidationError{Msg: "deploymentId is required"}
	}
	if req.DeployedProductID == "" {
		return &apierror.ValidationError{Msg: "deployedProductId is required"}
	}

	switch req.Type {
	case "case":
		if req.Subject == "" {
			return &apierror.ValidationError{Msg: "subject is required"}
		}
		if req.Description == "" {
			return &apierror.ValidationError{Msg: "description is required"}
		}
		if !validCaseSeverity[req.Severity] {
			return &apierror.ValidationError{Msg: "severity contains invalid value: " + string(req.Severity)}
		}
		if !validCaseIssueType[req.IssueType] {
			return &apierror.ValidationError{Msg: "issueType contains invalid value: " + string(req.IssueType)}
		}
	case "service_request":
		if req.CatalogID == "" {
			return &apierror.ValidationError{Msg: "catalogId is required for service_request"}
		}
		if req.CatalogItemID == "" {
			return &apierror.ValidationError{Msg: "catalogItemId is required for service_request"}
		}
		if len(req.Variables) == 0 {
			return &apierror.ValidationError{Msg: "variables are required for service_request"}
		}
	case "security_report_analysis":
		if req.Subject == "" {
			return &apierror.ValidationError{Msg: "subject is required for security_report_analysis"}
		}
		if req.Description == "" {
			return &apierror.ValidationError{Msg: "description is required for security_report_analysis"}
		}
		if len(req.Attachments) == 0 {
			return &apierror.ValidationError{Msg: "at least one attachment is required for security_report_analysis"}
		}
		for i, a := range req.Attachments {
			if a.Name == "" {
				return &apierror.ValidationError{Msg: fmt.Sprintf("attachments[%d].name is required", i)}
			}
			if a.File == "" {
				return &apierror.ValidationError{Msg: fmt.Sprintf("attachments[%d].file is required", i)}
			}
		}
	}

	return nil
}

// CreateCase implements CaseService.
func (s *caseService) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
	if err := validateCreateCaseRequest(req); err != nil {
		return domain.CreateCaseResponse{}, err
	}
	if req.Type != "case" {
		return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "only type \"case\" is supported for the Postgres data source"}
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
			CreatedOn:  c.CreatedOn,
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
			CreatedOn: c.CreatedOn,
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
	fieldCount := 0
	if req.State != nil {
		fieldCount++
	}
	if req.Severity != nil {
		fieldCount++
	}
	if req.WorkState != nil {
		fieldCount++
	}
	if fieldCount == 0 {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "exactly one of state, severity, or workState must be provided"}
	}
	if fieldCount > 1 {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "only one of state, severity, or workState may be provided per request"}
	}
	if req.State != nil && !validCaseState[*req.State] {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(*req.State)}
	}
	if req.Severity != nil && !validCaseSeverity[*req.Severity] {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity contains invalid value: " + string(*req.Severity)}
	}
	if req.WorkState != nil && !validCaseWorkState[*req.WorkState] {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "workState contains invalid value: " + string(*req.WorkState)}
	}
	c, err := s.repo.UpdateCase(ctx, req)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	return domain.UpdateCaseResponse{
		Message: "Case updated successfully",
		Case: domain.UpdatedCase{
			ID:        c.ID,
			UpdatedOn: c.UpdatedOn,
			State:     c.State,
			Severity:  c.Severity,
			WorkState: c.WorkState,
		},
	}, nil
}

// SearchCases implements CaseService.
func (s *caseService) SearchCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if req.Pagination.Limit > 50 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "limit cannot exceed 50"}
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

	for _, t := range req.Filters.Types {
		if !validCaseType[t] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "types contains invalid value: " + t}
		}
	}
	for _, s := range req.Filters.States {
		if !validCaseState[s] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "states contains invalid value: " + string(s)}
		}
	}
	for _, s := range req.Filters.Severities {
		if !validCaseSeverity[s] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "severities contains invalid value: " + string(s)}
		}
	}
	for _, it := range req.Filters.IssueTypes {
		if !validCaseIssueType[it] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "issueTypes contains invalid value: " + string(it)}
		}
	}
	for _, et := range req.Filters.EngagementTypes {
		if !validEngagementType[et] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "engagementTypes contains invalid value: " + string(et)}
		}
	}

	if req.Filters.CreatedByMe {
		token := middleware.UserIDTokenFromContext(ctx)
		if token == "" {
			return domain.SearchCasesResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required for createdByMe filter"}
		}
		email, err := emailFromJWT(token)
		if err != nil {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
		}
		req.Filters.CreatedBy = append(req.Filters.CreatedBy, email)
	}

	if req.Filters.ClosedEndDate != nil && req.Filters.ClosedStartDate != nil &&
		req.Filters.ClosedEndDate.Before(*req.Filters.ClosedStartDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "closedEndDate must not be before closedStartDate"}
	}
	if req.Filters.EndCreatedDate != nil && req.Filters.StartCreatedDate != nil &&
		req.Filters.EndCreatedDate.Before(*req.Filters.StartCreatedDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "endCreatedDate must not be before startCreatedDate"}
	}
	if req.Filters.EndUpdatedDate != nil && req.Filters.StartUpdatedDate != nil &&
		req.Filters.EndUpdatedDate.Before(*req.Filters.StartUpdatedDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "endUpdatedDate must not be before startUpdatedDate"}
	}

	if req.SortBy.Field == "" {
		req.SortBy.Field = domain.CaseSortFieldCreatedOn
	} else if !validCaseSortField[req.SortBy.Field] {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "sortBy.field must be one of: createdOn, updatedOn, severity, state"}
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
		Cases:        cases,
		Total: total,
		Limit:        req.Pagination.Limit,
		Offset:       req.Pagination.Offset,
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
