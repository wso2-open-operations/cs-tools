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
	"time"

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
	"engagement":               true,
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
		// Attachments are optional here (not backend-enforced by ServiceNow either):
		// the FE creates the case first, then uploads attachments in a separate
		// request per file, so a failed attachment upload never masks a
		// successful case creation.
		for i, a := range req.Attachments {
			if a.Name == "" {
				return &apierror.ValidationError{Msg: fmt.Sprintf("attachments[%d].name is required", i)}
			}
			if a.File == "" {
				return &apierror.ValidationError{Msg: fmt.Sprintf("attachments[%d].file is required", i)}
			}
		}
	case "engagement":
		if req.Subject == "" {
			return &apierror.ValidationError{Msg: "subject is required for engagement"}
		}
		if req.Description == "" {
			return &apierror.ValidationError{Msg: "description is required for engagement"}
		}
		if !validEngagementType[req.EngagementType] {
			return &apierror.ValidationError{Msg: "engagementType contains invalid value: " + string(req.EngagementType)}
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
	if len(req.WatchList) > 0 || req.AssigneeEmail != nil ||
		req.RelatedCaseID != nil || req.ParentID != nil || req.AutocloseHoldUntil != nil ||
		req.Subject != nil || req.Description != nil || req.DeploymentID != nil || req.DeployedProductID != nil ||
		req.BestCaseFixEta != nil || req.MostLikelyFixEta != nil || req.WorstCaseFixEta != nil {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "watchList, assigneeEmail, relatedCaseId, parentId, autocloseHoldUntil, subject, description, deploymentId, deployedProductId, bestCaseFixEta, mostLikelyFixEta, and worstCaseFixEta are only supported for the ServiceNow data source"}
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
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	callerEmail, callerEmailErr := resolveCaseFilterCallerEmail(token)
	parsed, err := ParseCaseFieldFilters(req.Filters.Filters, callerEmail, callerEmailErr, time.Now().UTC())
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}

	if err := validateUUIDs("projectId", parsed.ProjectIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateUUIDs("deploymentId", parsed.DeploymentIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	for _, t := range parsed.Types {
		if !validCaseType[t] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + t}
		}
	}
	for _, st := range parsed.States {
		if !validCaseState[st] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(st)}
		}
	}
	for _, sv := range parsed.Severities {
		if !validCaseSeverity[sv] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "severity contains invalid value: " + string(sv)}
		}
	}
	for _, it := range parsed.IssueTypes {
		if !validCaseIssueType[it] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "issueType contains invalid value: " + string(it)}
		}
	}
	for _, et := range parsed.EngagementTypes {
		if !validEngagementType[et] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "engagementType contains invalid value: " + string(et)}
		}
	}
	for _, ws := range parsed.WorkStates {
		if !validCaseWorkState[ws] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "workState contains invalid value: " + string(ws)}
		}
	}
	if err := validateUUIDs("assignedUserId", parsed.AssignedUserIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	if parsed.CreatedByMe {
		parsed.CreatedBy = append(parsed.CreatedBy, callerEmail)
	}

	if parsed.ClosedEndDate != nil && parsed.ClosedStartDate != nil &&
		parsed.ClosedEndDate.Before(*parsed.ClosedStartDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "closedOn: lte value must not be before gte value"}
	}
	if parsed.EndCreatedDate != nil && parsed.StartCreatedDate != nil &&
		parsed.EndCreatedDate.Before(*parsed.StartCreatedDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "createdOn: lte value must not be before gte value"}
	}
	if parsed.EndUpdatedDate != nil && parsed.StartUpdatedDate != nil &&
		parsed.EndUpdatedDate.Before(*parsed.StartUpdatedDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "updatedOn: lte value must not be before gte value"}
	}

	// These fields dot-walk into ServiceNow-specific concepts (tags,
	// project-onboarding-status, integration-CS-team, etc.) that have no
	// equivalent in the Postgres schema and no repository query support today.
	// Reject rather than silently drop the predicate and widen the result set.
	if len(parsed.Tags) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "tag" is not supported by this data source`}
	}
	if len(parsed.ExcludeTags) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "tag" (notIn) is not supported by this data source`}
	}
	if parsed.ParentID != nil {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "parentId" is not supported by this data source`}
	}
	if len(parsed.ProductNames) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "product" is not supported by this data source`}
	}
	if len(parsed.ProjectOnboardingStatuses) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "projectOnboardingStatus" is not supported by this data source`}
	}
	if len(parsed.ProjectTypeNames) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "projectType" is not supported by this data source`}
	}
	if len(parsed.IntegrationCsTeamIDs) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "integrationCsTeam" is not supported by this data source`}
	}
	if parsed.Unassigned {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "assignedUserId" (isEmpty) is not supported by this data source`}
	}
	if parsed.ResolutionNotesEmpty {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "resolutionNotes" is not supported by this data source`}
	}

	// Task-SLA and escalation predicates, OR groups, and grouped counts are
	// implemented only in the ServiceNow case service (snCaseService.SearchCases);
	// caseRepo.SearchCases models none of them. ParseCaseFieldFilters accepts them
	// because it is shared by both data sources, so without these guards a
	// Postgres deployment would drop the predicate and answer 200 with a wider
	// result set than the caller asked for. These stay ServiceNow-only by design:
	// reject loudly rather than implement them here.
	if parsed.TaskSLAFilter != nil {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "taskSLABusinessElapsedPercent" is not supported by this data source`}
	}
	if len(parsed.EscalationLevels) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "escalationLevel" is not supported by this data source`}
	}
	if parsed.HasActiveEscalation != nil {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "escalation" is not supported by this data source`}
	}
	if len(req.Filters.AnyOf) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "anyOf is not supported by this data source"}
	}
	if req.GroupBy != "" {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "groupBy is not supported by this data source"}
	}

	req.Parsed = parsed

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
		Cases:  cases,
		Total:  total,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}

func (s *caseService) CreateCaseAttachment(_ context.Context, _ domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
	return domain.CreateAttachmentResponse{}, &apierror.ServiceUnavailableError{Msg: "attachments are only supported for the ServiceNow data source"}
}

func (s *caseService) SearchCaseAttachments(_ context.Context, _ domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error) {
	return domain.SearchAttachmentsResponse{}, &apierror.ServiceUnavailableError{Msg: "attachments are only supported for the ServiceNow data source"}
}

func (s *caseService) SearchCaseActivities(_ context.Context, _ domain.SearchCaseActivitiesRequest) (domain.SearchCaseActivitiesResponse, error) {
	return domain.SearchCaseActivitiesResponse{}, &apierror.ServiceUnavailableError{Msg: "case activities are only supported for the ServiceNow data source"}
}

func (s *caseService) GetCaseAttachmentContent(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", &apierror.ServiceUnavailableError{Msg: "attachments are only supported for the ServiceNow data source"}
}

func (s *caseService) DeleteCaseAttachment(_ context.Context, _ domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error) {
	return domain.DeleteAttachmentResponse{}, &apierror.ServiceUnavailableError{Msg: "attachments are only supported for the ServiceNow data source"}
}

func (s *caseService) AddCaseTag(_ context.Context, _, _ string) (domain.Tag, error) {
	return domain.Tag{}, &apierror.ServiceUnavailableError{Msg: "case tags are only supported for the ServiceNow data source"}
}

func (s *caseService) RemoveCaseTag(_ context.Context, _, _ string) error {
	return &apierror.ServiceUnavailableError{Msg: "case tags are only supported for the ServiceNow data source"}
}

func (s *caseService) SearchTags(_ context.Context, _ string, _ int) ([]domain.Tag, error) {
	return nil, &apierror.ServiceUnavailableError{Msg: "case tags are only supported for the ServiceNow data source"}
}
