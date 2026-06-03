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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type caseService struct {
	repo repository.CaseRepository
}

// NewCaseService constructs a CaseService backed by the given repository.
func NewCaseService(repo repository.CaseRepository) CaseService {
	return &caseService{repo: repo}
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
	domain.CaseStateReopened:         true,
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

// CreateCase implements CaseService.
func (s *caseService) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
	if err := validateUUIDs("createdBy", []string{req.CreatedBy}); err != nil {
		return domain.Case{}, err
	}
	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.Case{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.Case{}, err
	}
	if err := validateUUIDs("deployedProductId", []string{req.DeployedProductID}); err != nil {
		return domain.Case{}, err
	}
	if req.Subject == "" {
		return domain.Case{}, &apierror.ValidationError{Msg: "subject is required"}
	}
	if req.Description == "" {
		return domain.Case{}, &apierror.ValidationError{Msg: "description is required"}
	}
	if !validCasePriority[req.Priority] {
		return domain.Case{}, &apierror.ValidationError{Msg: "priority contains invalid value: " + string(req.Priority)}
	}
	if !validCaseIssueType[req.IssueType] {
		return domain.Case{}, &apierror.ValidationError{Msg: "issueType contains invalid value: " + string(req.IssueType)}
	}
	return s.repo.CreateCase(ctx, req)
}

var validCommentType = map[domain.CommentType]bool{
	domain.CommentTypeWorkNote: true,
	domain.CommentTypeComment:  true,
	domain.CommentTypeActivity: true,
}

// CreateCaseComment implements CaseService.
func (s *caseService) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CaseComment, error) {
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.CaseComment{}, err
	}
	if err := validateUUIDs("createdBy", []string{req.CreatedBy}); err != nil {
		return domain.CaseComment{}, err
	}
	if !validCommentType[req.CommentType] {
		return domain.CaseComment{}, &apierror.ValidationError{Msg: "commentType contains invalid value: " + string(req.CommentType)}
	}
	if req.Body == "" {
		return domain.CaseComment{}, &apierror.ValidationError{Msg: "body is required"}
	}
	return s.repo.CreateCaseComment(ctx, req)
}

// SearchCaseComments implements CaseService.
func (s *caseService) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error) {
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseCommentsResponse{}, err
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

// SearchCases implements CaseService.
func (s *caseService) SearchCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateUUIDs("projectIds", req.ProjectIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateUUIDs("deploymentIds", req.DeploymentIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateUUIDs("deployedProductIds", req.DeployedProductIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	for _, s := range req.StateKeys {
		if !validCaseState[s] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "stateKeys contains invalid value: " + string(s)}
		}
	}
	for _, p := range req.PriorityKeys {
		if !validCasePriority[p] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "priorityKeys contains invalid value: " + string(p)}
		}
	}
	for _, it := range req.IssueTypeKeys {
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
