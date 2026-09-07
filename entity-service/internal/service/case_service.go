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
	"log/slog"
	"strings"
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
	"announcement":             true,
	"engagement":               true,
}

// caseTypeAliases maps caller-supplied case type values this API does not
// consider canonical to the value it actually recognises. "default_case" is
// the real, currently-in-production customer-portal frontend's value for
// this type (it's ServiceNow's own raw caseType wire value, which the
// frontend was built directly against before this service's Postgres-backed
// "case" enum existed — see migrations/000008_create_cases.up.sql's
// case_type_enum) and must keep working indefinitely, not just during a
// migration window. Applying this alias is the FIRST thing that happens to
// any caller-supplied case type value, before validCaseType or any
// data-source-specific mapping (snCaseTypeMap, the Postgres repo's enum
// cast) ever sees it, so every downstream consumer only ever has to know
// about the canonical value "case".
var caseTypeAliases = map[string]string{
	"default_case": "case",
}

// normalizeCaseType resolves a caller-supplied case type value to its
// canonical form via caseTypeAliases, or returns it unchanged if it isn't an
// alias (including if it's already canonical, or altogether invalid --
// validCaseType is what rejects the latter).
func normalizeCaseType(t string) string {
	if canonical, ok := caseTypeAliases[t]; ok {
		return canonical
	}
	return t
}

var validEngagementType = map[domain.EngagementType]bool{
	domain.EngagementTypeMigration:             true,
	domain.EngagementTypeConsultancy:           true,
	domain.EngagementTypeNewFeatureImprovement: true,
	domain.EngagementTypeFollowUp:              true,
	domain.EngagementTypeOnboarding:            true,
}

var validEngagementPaymentType = map[domain.EngagementPaymentType]bool{
	domain.EngagementPaymentTypePaid: true,
	domain.EngagementPaymentTypeFOC:  true,
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

// validCaseAggregateField is the allow-list for AggregateCasesRequest.GroupBy,
// matching openapi.yaml's AggregateCasesRequest.groupBy enum exactly.
var validCaseAggregateField = map[string]bool{
	"account":  true,
	"state":    true,
	"severity": true,
	"type":     true,
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

// validateCreateCaseRequest validates fields common to all CreateCase data
// sources. Normalizes req.Type via normalizeCaseType FIRST (req is a
// pointer specifically so this mutation is visible to the caller's own
// switch on req.Type and its SN/repo payload-building afterwards) — every
// other check in this function, and everything downstream, only ever sees
// the canonical value.
// UUID format of ID fields is not checked here — postgres IDs are UUIDs but
// ServiceNow IDs are opaque hex strings; callers add format checks as needed.
func validateCreateCaseRequest(req *domain.CreateCaseRequest) error {
	req.Type = normalizeCaseType(req.Type)
	if req.Type == "" {
		return &apierror.ValidationError{Msg: "type is required"}
	}
	if !validCaseType[req.Type] {
		return &apierror.ValidationError{Msg: "type contains invalid value: " + req.Type}
	}
	if req.ProjectID == "" {
		return &apierror.ValidationError{Msg: "projectId is required"}
	}
	// Announcements have no deployment/deployed-product concept: these fields
	// are deliberately omitted at the ServiceNow layer, not just optional.
	if req.Type != "announcement" {
		if req.DeploymentID == "" {
			return &apierror.ValidationError{Msg: "deploymentId is required"}
		}
		if req.DeployedProductID == "" {
			return &apierror.ValidationError{Msg: "deployedProductId is required"}
		}
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
		if !validEngagementPaymentType[req.EngagementPaymentType] {
			return &apierror.ValidationError{Msg: "engagementPaymentType contains invalid value: " + string(req.EngagementPaymentType)}
		}
	case "announcement":
		if req.Subject == "" {
			return &apierror.ValidationError{Msg: "subject is required for announcement"}
		}
		if req.Description == "" {
			return &apierror.ValidationError{Msg: "description is required for announcement"}
		}
	}

	return nil
}

// CreateCase implements CaseService.
func (s *caseService) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
	if err := validateCreateCaseRequest(&req); err != nil {
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
	if req.WatchList != nil || req.AssigneeEmail != nil ||
		req.RelatedCaseID != nil || req.AutocloseHoldUntil != nil ||
		req.Subject != nil || req.Description != nil || req.DeploymentID != nil || req.DeployedProductID != nil ||
		req.BestCaseFixEta != nil || req.MostLikelyFixEta != nil || req.WorstCaseFixEta != nil ||
		req.Type != nil || req.EngagementType != nil || req.CatalogID != nil ||
		req.CatalogItemID != nil || len(req.Variables) > 0 {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "watchList, assigneeEmail, relatedCaseId, autocloseHoldUntil, subject, description, deploymentId, deployedProductId, bestCaseFixEta, mostLikelyFixEta, worstCaseFixEta, type, engagementType, catalogId, catalogItemId, and variables are only supported for the ServiceNow data source"}
	}
	// Native parent-linking: parentId is a standalone update on the Postgres data
	// source -- it links this case to a parent, or clears the link when empty --
	// and is mutually exclusive with every other field, as on the ServiceNow path.
	// This is the write counterpart to the Prevent Closure Of Parent guard below:
	// it is how a parent link comes to exist natively in the first place.
	if req.ParentID != nil {
		if req.State != nil || req.Severity != nil || req.WorkState != nil {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "parentId cannot be combined with state, severity, or workState"}
		}
		return s.updateCaseParent(ctx, req.ID, *req.ParentID)
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
	// Prevent Closure Of Parent (ported from the ServiceNow `before` business
	// rule of the same name, which called current.setAbortAction(true)): a case
	// may not be closed while any of its child cases is still open. Only the
	// transition to closed is gated -- every other state change, and severity or
	// work-state edits, are unaffected. Enforced here in the service layer,
	// matching the other case invariants, rather than in the UPDATE statement.
	//
	// The count and the close are not one transaction, so a child could be linked
	// to this parent (via the parentId branch above) between them. That is not a
	// correctness hole: as in ServiceNow, linking an open child under an
	// already-closed parent is itself allowed, so "closed parent + open child" is a
	// legitimately reachable state, not a strict invariant this guard promises to
	// hold. The guard blocks the one path ServiceNow blocked -- closing a parent
	// that currently has open children -- and nothing more. If the product later
	// decides that state must never exist, gate the parentId branch too and make
	// both atomic (an advisory lock keyed on the parent id, as in
	// scheduled_task_run_repo.Attempt).
	if req.State != nil && *req.State == domain.CaseStateClosed {
		openChildren, err := s.repo.CountNonClosedChildCases(ctx, req.ID)
		if err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		if openChildren > 0 {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "cannot close this case while it has open child cases"}
		}
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

// updateCaseParent handles the parentId branch of UpdateCase: it validates the
// requested parent link and applies it. An empty parentId clears the link; a
// non-empty one must be a UUID other than the case's own id -- a case cannot be
// its own parent. The parent's existence is enforced by the foreign key in the
// repository, which maps a dangling reference to a ValidationError.
//
// Faithful to the ServiceNow "Prevent Closure Of Parent" rule, only closing a
// parent is gated (see UpdateCase); linking an open child under an already-closed
// parent is not rejected here, matching the source system. Closing that asymmetry
// would be a deliberate product decision, not a bug fix.
func (s *caseService) updateCaseParent(ctx context.Context, caseID, parentID string) (domain.UpdateCaseResponse, error) {
	var parent *string
	if parentID != "" {
		if err := validateUUIDs("parentId", []string{parentID}); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		if parentID == caseID {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "a case cannot be its own parent"}
		}
		parent = &parentID
	}
	c, err := s.repo.UpdateCaseParent(ctx, caseID, parent)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	// Add Work notes on Parent addition (ported from the ServiceNow business rule
	// of the same name): when a parent is linked, record it on the case timeline
	// as a work note. Fires only on link, not on clear -- matching the rule ("on
	// addition"). ServiceNow ran this inside the write transaction; here the link
	// is the committed result and the note is a best-effort follow-on write, so a
	// failure to note it (a caller who isn't the internal user the work_note
	// trigger requires, or who carries no identity token) is logged, never
	// surfaced, and never undoes the link.
	if parent != nil {
		s.addParentLinkedWorkNote(ctx, caseID, parentID)
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

// addParentLinkedWorkNote posts the "parent linked" work note for updateCaseParent.
// It is deliberately side-effect-only (no error return): the parent link has already
// committed, and per the rule's port the note must not be able to roll it back. It
// resolves the acting user (the work_note DB trigger requires an internal author),
// resolves the parent's human-readable number for a readable note, and logs — rather
// than fails — if either step or the insert doesn't succeed.
func (s *caseService) addParentLinkedWorkNote(ctx context.Context, caseID, parentID string) {
	actor, err := s.resolveActor(ctx)
	if err != nil {
		slog.WarnContext(ctx, "parent-linked work note skipped: cannot resolve actor", "caseId", caseID, "parentId", parentID, "error", err)
		return
	}
	parentRef := parentID
	if pv, err := s.repo.GetCaseByID(ctx, parentID); err == nil && pv.Number != "" {
		parentRef = pv.Number
	}
	if _, err := s.repo.CreateCaseComment(ctx, domain.CreateCaseCommentRequest{
		CaseID:    caseID,
		CreatedBy: actor.ID,
		Type:      domain.CommentTypeWorkNote,
		Content:   "Linked to parent case " + parentRef + ".",
	}); err != nil {
		slog.WarnContext(ctx, "parent-linked work note failed", "caseId", caseID, "parentId", parentID, "error", err)
	}
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
	// resolvedOn has no backing column in the relational schema and
	// caseRepo.SearchCases models no predicate for it, so accepting it here
	// would drop the bound silently and answer 200 with every case rather
	// than the resolved-in-range ones asked for. Reject, same as every other
	// predicate this data source cannot express.
	if parsed.ResolvedStartDate != nil || parsed.ResolvedEndDate != nil {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "resolvedOn" is not supported by this data source`}
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
	// state+in is supported here; state+notIn has no repository query support,
	// and dropping an exclusion silently would widen the result set.
	if len(parsed.ExcludeStates) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "state" (notIn) is not supported by this data source`}
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
	if len(parsed.CreTeamIDs) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "creTeam" is not supported by this data source`}
	}
	if len(parsed.SreTeamIDs) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "sreTeam" is not supported by this data source`}
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
	if parsed.HasBreachedSLA != nil {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "slaBreached" is not supported by this data source`}
	}
	if parsed.HasActiveAccountEscalation != nil {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "accountEscalationActive" is not supported by this data source`}
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

func (s *caseService) AggregateCases(_ context.Context, _ domain.AggregateCasesRequest) (domain.AggregateResponse, error) {
	return domain.AggregateResponse{}, &apierror.ServiceUnavailableError{Msg: "groupBy is only supported for the ServiceNow data source"}
}

// resolveActor authenticates the caller from the x-user-id-token header
// carried on ctx and resolves it to a platform user record. It is the same
// authentication step CreateCaseComment above already performs -- there is no
// finer-grained per-case ACL check in this data source's case service beyond
// "the caller must be a known, authenticated user," so attachment mutations
// reuse it verbatim rather than inventing a new authorization pattern.
func (s *caseService) resolveActor(ctx context.Context) (domain.User, error) {
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

// CreateCaseAttachment implements CaseService for the CSM-native (Postgres)
// data source. Unlike ServiceNow, this data source never receives file bytes
// directly: the caller must have already uploaded the file to SFTPGo and
// supplies its storage_key plus the size/name/type metadata. Only
// ReferenceTypeCase is supported -- the other ReferenceType values
// (conversation, change_request, deployment, incident) have no Postgres
// schema backing on this data source.
func (s *caseService) CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.CreateAttachmentResponse{}, err
	}
	if req.ReferenceType != domain.ReferenceTypeCase {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "referenceType must be 'case' for this data source"}
	}
	if req.Name == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "name is required"}
	}
	if req.Type == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "type is required"}
	}
	if req.StorageKey == nil || *req.StorageKey == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "storageKey is required: this data source has no base64 payload alternative, the file must already be uploaded to SFTPGo"}
	}
	if req.SizeBytes <= 0 {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "sizeBytes must be greater than zero"}
	}
	// Empty defaults to complete: every caller before this change (and every
	// existing Postgres-path caller that doesn't know about the pending
	// state) gets exactly today's behavior. Only a caller that explicitly
	// wants the two-step upload flow passes "pending".
	switch req.Status {
	case "":
		req.Status = domain.AttachmentStatusComplete
	case domain.AttachmentStatusPending, domain.AttachmentStatusComplete:
		// valid
	default:
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid status %q: must be 'pending' or 'complete'", req.Status)}
	}

	user, err := s.resolveActor(ctx)
	if err != nil {
		return domain.CreateAttachmentResponse{}, err
	}
	req.CreatedBy = user.ID

	a, err := s.repo.CreateCaseAttachment(ctx, req)
	if err != nil {
		return domain.CreateAttachmentResponse{}, err
	}

	return domain.CreateAttachmentResponse{
		Message: "Attachment created successfully",
		Attachment: domain.AttachmentDetail{
			ID:         a.ID,
			SizeBytes:  a.SizeBytes,
			CreatedOn:  a.CreatedOn,
			CreatedBy:  user.Email,
			StorageKey: a.StorageKey,
			Status:     a.Status,
			// No DownloadURL: this service holds no bytes for a Postgres-sourced
			// attachment, only its storage_key. Resolving storage_key to an
			// actual download location is the downstream CSM backend's job.
		},
	}, nil
}

// ConfirmCaseAttachment implements CaseService for the CSM-native (Postgres)
// data source. It is the second half of the two-step upload flow: the caller
// (the CSM backend) registers a 'pending' row via CreateCaseAttachment
// *before* minting an SFTPGo upload credential, then calls this once the
// browser reports the upload succeeded, transitioning the row to 'complete'.
//
// Ownership: unlike UpdateAttachment/DeleteCaseAttachment (which any
// authenticated user may perform on any case attachment -- there is no
// per-resource ACL in this data source beyond authentication, see
// resolveActor's doc comment), confirming is restricted to the same actor
// who created the pending row. A pending row represents an upload a specific
// user initiated; there is no legitimate case yet for a different user to
// confirm it on their behalf, and allowing it would let any authenticated
// user "complete" an attachment they never uploaded.
func (s *caseService) ConfirmCaseAttachment(ctx context.Context, id string) (domain.ConfirmAttachmentResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ConfirmAttachmentResponse{}, err
	}

	user, err := s.resolveActor(ctx)
	if err != nil {
		return domain.ConfirmAttachmentResponse{}, err
	}

	existing, err := s.repo.GetCaseAttachmentByID(ctx, id)
	if err != nil {
		return domain.ConfirmAttachmentResponse{}, err
	}
	if existing.CreatedBy == nil || existing.CreatedBy.ID == nil || *existing.CreatedBy.ID != user.ID {
		return domain.ConfirmAttachmentResponse{}, &apierror.ForbiddenError{Msg: "attachment was not created by the current user"}
	}
	if existing.Status != domain.AttachmentStatusPending {
		return domain.ConfirmAttachmentResponse{}, &apierror.ConflictError{Msg: fmt.Sprintf("attachment is not pending (current status: %q)", existing.Status)}
	}

	a, err := s.repo.ConfirmCaseAttachment(ctx, id)
	if err != nil {
		return domain.ConfirmAttachmentResponse{}, err
	}

	return domain.ConfirmAttachmentResponse{
		Message: "Attachment confirmed successfully",
		Attachment: domain.AttachmentDetail{
			ID:         a.ID,
			SizeBytes:  a.SizeBytes,
			CreatedOn:  a.CreatedOn,
			CreatedBy:  user.Email,
			StorageKey: a.StorageKey,
			Status:     a.Status,
		},
	}, nil
}

// SearchCaseAttachments implements CaseService for the CSM-native (Postgres)
// data source.
//
// Read-path status decision: the underlying repository query filters out
// 'pending' rows entirely (see caseRepo.SearchCaseAttachments), so a case's
// attachment list never shows a still-uploading placeholder to other users.
// This is a deliberate product-behavior choice, not an oversight: a pending
// row may never complete (the upload could fail, or the tab could just
// close), and showing it in a shared list before that's known risks other
// team members seeing and trying to act on a file that doesn't exist yet. A
// specific-id lookup (GetAttachmentByID) is not filtered this way -- it
// still returns a pending row -- which is what the confirm step relies on,
// and is also how an uploader could be shown their own in-flight upload if
// the FE chooses to poll it directly rather than via this list.
func (s *caseService) SearchCaseAttachments(ctx context.Context, req domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error) {
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}
	if req.ReferenceType != domain.ReferenceTypeCase {
		return domain.SearchAttachmentsResponse{}, &apierror.ValidationError{Msg: "referenceType must be 'case' for this data source"}
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}

	attachments, total, err := s.repo.SearchCaseAttachments(ctx, req.ReferenceID, req.Pagination)
	if err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}

	return domain.SearchAttachmentsResponse{
		Attachments: attachments,
		Total:       total,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
		HasMore:     req.Pagination.Offset+len(attachments) < total,
	}, nil
}

func (s *caseService) SearchCaseActivities(_ context.Context, _ domain.SearchCaseActivitiesRequest) (domain.SearchCaseActivitiesResponse, error) {
	return domain.SearchCaseActivitiesResponse{}, &apierror.ServiceUnavailableError{Msg: "case activities are only supported for the ServiceNow data source"}
}

// GetCaseAttachmentContent implements CaseService for the CSM-native
// (Postgres) data source. This service never holds the file bytes for a
// Postgres-sourced attachment -- they live in SFTPGo, addressed by the
// attachment's storage_key (see GetAttachmentByID / SearchCaseAttachments).
// Callers must resolve content externally via that storage_key rather than
// through this endpoint.
func (s *caseService) GetCaseAttachmentContent(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", &apierror.ServiceUnavailableError{Msg: "this data source does not serve attachment bytes directly; resolve content via the attachment's storageKey"}
}

// DeleteCaseAttachment implements CaseService for the CSM-native (Postgres)
// data source. This only removes the metadata row -- it does not delete the
// backing SFTPGo file, which is the downstream CSM backend's responsibility
// when it also calls SFTPGo's own delete API.
func (s *caseService) DeleteCaseAttachment(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error) {
	if err := validateUUIDs("attachmentId", []string{req.AttachmentID}); err != nil {
		return domain.DeleteAttachmentResponse{}, err
	}
	if _, err := s.resolveActor(ctx); err != nil {
		return domain.DeleteAttachmentResponse{}, err
	}
	if err := s.repo.DeleteCaseAttachment(ctx, req.AttachmentID); err != nil {
		return domain.DeleteAttachmentResponse{}, err
	}
	return domain.DeleteAttachmentResponse{Message: "Attachment deleted successfully"}, nil
}

func (s *caseService) GetAttachment(_ context.Context, _ string) (domain.Attachment, error) {
	return domain.Attachment{}, &apierror.ServiceUnavailableError{Msg: "attachments are only supported for the ServiceNow data source"}
}

func (s *caseService) AddCaseTag(_ context.Context, _, _ string) (domain.Tag, error) {
	return domain.Tag{}, &apierror.ServiceUnavailableError{Msg: "case tags are only supported for the ServiceNow data source"}
}

func (s *caseService) RemoveCaseTag(_ context.Context, _, _ string) error {
	return &apierror.ServiceUnavailableError{Msg: "case tags are only supported for the ServiceNow data source"}
}

func (s *caseService) SearchTags(_ context.Context, _ domain.SearchTagsRequest) ([]domain.Tag, error) {
	return nil, &apierror.ServiceUnavailableError{Msg: "case tags are only supported for the ServiceNow data source"}
}

func (s *caseService) GetCaseFeedback(_ context.Context, _ string) (domain.CaseEmojiFeedback, error) {
	return domain.CaseEmojiFeedback{}, &apierror.ServiceUnavailableError{Msg: "case feedback is only supported for the ServiceNow data source"}
}

func (s *caseService) SubmitCaseFeedback(_ context.Context, _ string, _ domain.SubmitCaseFeedbackRequest) (domain.SubmitCaseFeedbackResponse, error) {
	return domain.SubmitCaseFeedbackResponse{}, &apierror.ServiceUnavailableError{Msg: "case feedback is only supported for the ServiceNow data source"}
}

// GetAttachmentByID implements CaseService for the CSM-native (Postgres) data
// source. Content is always nil: this service holds no bytes for a
// Postgres-sourced attachment, only its storage_key -- see
// GetCaseAttachmentContent's doc comment for why content must be resolved
// externally via StorageKey instead.
func (s *caseService) GetAttachmentByID(ctx context.Context, id string) (domain.AttachmentDetails, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AttachmentDetails{}, err
	}

	a, err := s.repo.GetCaseAttachmentByID(ctx, id)
	if err != nil {
		return domain.AttachmentDetails{}, err
	}

	var createdBy string
	if a.CreatedBy != nil {
		createdBy = a.CreatedBy.Email
	}

	return domain.AttachmentDetails{
		ID:            a.ID,
		ReferenceID:   a.ReferenceID,
		ReferenceType: &a.ReferenceType,
		Name:          a.Name,
		Type:          a.Type,
		SizeBytes:     a.SizeBytes,
		Description:   a.Description,
		CreatedBy:     createdBy,
		CreatedOn:     a.CreatedOn,
		DownloadURL:   a.DownloadURL,
		PreviewURL:    a.PreviewURL,
		Content:       nil,
		StorageKey:    a.StorageKey,
		Status:        a.Status,
	}, nil
}

// validatePGAttachmentUpdate mirrors the ServiceNow path's
// validateAttachmentUpdate, narrowed to the one reference type this data
// source's attachments table actually models: deployment attachments have no
// Postgres schema backing here, so that branch is rejected outright rather
// than silently accepted.
func validatePGAttachmentUpdate(req domain.UpdateAttachmentRequest) error {
	if req.ReferenceType != domain.ReferenceTypeCase {
		return &apierror.ValidationError{Msg: fmt.Sprintf("invalid reference type %q: only 'case' is supported for this data source", req.ReferenceType)}
	}
	if req.Description != nil {
		return &apierror.ValidationError{Msg: "description field is not allowed for case reference type"}
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		return &apierror.ValidationError{Msg: "name field is required for case reference type"}
	}
	return nil
}

// UpdateAttachment implements CaseService for the CSM-native (Postgres) data
// source. Only renaming is supported, mirroring the one mutation the
// ServiceNow path allows for reference type "case" (name required,
// description forbidden).
func (s *caseService) UpdateAttachment(ctx context.Context, req domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error) {
	if err := validateUUIDs("id", []string{req.AttachmentID}); err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}
	if err := validatePGAttachmentUpdate(req); err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}

	user, err := s.resolveActor(ctx)
	if err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}

	updatedOn, err := s.repo.UpdateCaseAttachmentName(ctx, req.AttachmentID, strings.TrimSpace(*req.Name), user.ID)
	if err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}

	return domain.UpdateAttachmentResponse{
		Message: "Attachment updated successfully",
		Attachment: domain.UpdatedAttachment{
			ID:        req.AttachmentID,
			UpdatedOn: updatedOn,
			UpdatedBy: user.Email,
		},
	}, nil
}
