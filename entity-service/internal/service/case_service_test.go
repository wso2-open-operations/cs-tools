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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// alwaysUnrestrictedAccess is an AccessService stub for tests that exercise
// CaseService/ProjectService logic unrelated to access scoping -- every
// caller sees everything, so these tests aren't coupled to AccessService's
// own rules (see access_service_test.go for those).
type alwaysUnrestrictedAccess struct{}

func (alwaysUnrestrictedAccess) ResolveScope(context.Context) (AccessScope, error) {
	return AccessScope{Unrestricted: true}, nil
}

// stubCaseRepo is a minimal repository.CaseRepository whose SearchCases
// panics if called: tests using it prove ParseCaseFieldFilters' rejection of
// an unsupported field happens before the Postgres backend ever reaches the
// repository, not merely that the repository ignores the field.
type stubCaseRepo struct {
	aggregateCases                func(ctx context.Context, req domain.SearchCasesRequest, groupBy string) ([]domain.AggregateBucket, error)
	searchCases                   func(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error)
	createCaseAttachment          func(ctx context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error)
	searchCaseAttachments         func(ctx context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error)
	getCaseAttachmentByID         func(ctx context.Context, id string) (domain.Attachment, error)
	deleteCaseAttachment          func(ctx context.Context, id string) error
	updateAttachmentName          func(ctx context.Context, id, name, updatedBy string) (time.Time, error)
	confirmCaseAttachment         func(ctx context.Context, id string) (domain.Attachment, error)
	searchCaseComments            func(ctx context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error)
	updateCase                    func(ctx context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error)
	createCaseFromServiceNow      func(ctx context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error)
	createCaseComment             func(ctx context.Context, req domain.CreateCaseCommentRequest, createdOn *time.Time) (domain.CaseComment, error)
	createCase                    func(ctx context.Context, req domain.CreateCaseRequest) (domain.Case, error)
	getCaseByID                   func(ctx context.Context, id string, scope repository.SearchScope) (domain.CaseView, error)
	addCaseTag                    func(ctx context.Context, caseID, label, actorEmail string) (domain.Tag, error)
	setCaseWatchList              func(ctx context.Context, caseID string, userIDs []string, actorEmail string) ([]domain.WatchListUser, time.Time, error)
	accountDefaultWatcherEmails   func(ctx context.Context, projectID string) ([]string, error)
	getCaseEtaSharedOn            func(ctx context.Context, caseID string) (*time.Time, error)
	projectContactEmailsByRole    func(ctx context.Context, projectID, role string) ([]string, error)
	updateCaseAssignee            func(ctx context.Context, caseID string, userID *string, callerEmail string) (time.Time, bool, error)
	acknowledgeCase               func(ctx context.Context, caseID, actorID, actorEmail string) (bool, domain.AssignedEngineerRef, string, time.Time, error)
	updateCaseParent              func(ctx context.Context, caseID, parentID, callerEmail string) (time.Time, error)
	updateCaseFields              func(ctx context.Context, req domain.UpdateCaseRequest, actorID, actorEmail string) (time.Time, error)
	recordCaseFieldChangeActivity func(ctx context.Context, caseID, fieldName, oldValue, newValue, actorEmail string) error
	removeCaseTag                 func(ctx context.Context, caseID, tagID, callerEmail string) error
	setCaseTagSNSysID             func(ctx context.Context, caseID, tagID, snSysID string) error
	getCaseTagSNSysID             func(ctx context.Context, caseID, tagID string) (*string, error)
	markCaseFixIssued             func(ctx context.Context, caseID string) (time.Time, bool, error)
}

func (s *stubCaseRepo) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
	if s.createCase != nil {
		return s.createCase(ctx, req)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) CreateCaseFromServiceNow(ctx context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error) {
	if s.createCaseFromServiceNow != nil {
		return s.createCaseFromServiceNow(ctx, req, id, number, wso2ID, createdBy, state)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) GetCaseByID(ctx context.Context, id string, scope repository.SearchScope) (domain.CaseView, error) {
	if s.getCaseByID != nil {
		return s.getCaseByID(ctx, id, scope)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) AggregateCases(ctx context.Context, req domain.SearchCasesRequest, groupBy string, _ repository.SearchScope) ([]domain.AggregateBucket, error) {
	if s.aggregateCases != nil {
		return s.aggregateCases(ctx, req, groupBy)
	}
	panic("AggregateCases called unexpectedly")
}
func (s *stubCaseRepo) SearchCases(ctx context.Context, req domain.SearchCasesRequest, scope repository.SearchScope) ([]domain.SearchCaseView, int, error) {
	if s.searchCases != nil {
		return s.searchCases(ctx, req)
	}
	panic("SearchCases called unexpectedly: the unsupported-field check should have short-circuited before reaching the repository")
}
func (s *stubCaseRepo) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest, createdOn *time.Time) (domain.CaseComment, error) {
	if s.createCaseComment != nil {
		return s.createCaseComment(ctx, req, createdOn)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
	if s.searchCaseComments != nil {
		return s.searchCaseComments(ctx, req)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
	if s.updateCase != nil {
		return s.updateCase(ctx, req)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error) {
	if s.createCaseAttachment != nil {
		return s.createCaseAttachment(ctx, req)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) SearchCaseAttachments(ctx context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error) {
	if s.searchCaseAttachments != nil {
		return s.searchCaseAttachments(ctx, caseID, pagination)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) GetCaseAttachmentByID(ctx context.Context, id string) (domain.Attachment, error) {
	if s.getCaseAttachmentByID != nil {
		return s.getCaseAttachmentByID(ctx, id)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) DeleteCaseAttachment(ctx context.Context, id string) error {
	if s.deleteCaseAttachment != nil {
		return s.deleteCaseAttachment(ctx, id)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) UpdateCaseAttachmentName(ctx context.Context, id, name, updatedBy string) (time.Time, error) {
	if s.updateAttachmentName != nil {
		return s.updateAttachmentName(ctx, id, name, updatedBy)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) ConfirmCaseAttachment(ctx context.Context, id string) (domain.Attachment, error) {
	if s.confirmCaseAttachment != nil {
		return s.confirmCaseAttachment(ctx, id)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) AddCaseTag(ctx context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
	if s.addCaseTag != nil {
		return s.addCaseTag(ctx, caseID, label, actorEmail)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) RemoveCaseTag(ctx context.Context, caseID, tagID, callerEmail string) error {
	if s.removeCaseTag != nil {
		return s.removeCaseTag(ctx, caseID, tagID, callerEmail)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) SetCaseTagSNSysID(ctx context.Context, caseID, tagID, snSysID string) error {
	if s.setCaseTagSNSysID != nil {
		return s.setCaseTagSNSysID(ctx, caseID, tagID, snSysID)
	}
	return nil
}
func (s *stubCaseRepo) GetCaseTagSNSysID(ctx context.Context, caseID, tagID string) (*string, error) {
	if s.getCaseTagSNSysID != nil {
		return s.getCaseTagSNSysID(ctx, caseID, tagID)
	}
	return nil, nil
}
func (s *stubCaseRepo) SearchTags(context.Context, string, string, int) ([]domain.Tag, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) SetCaseWatchList(ctx context.Context, caseID string, userIDs []string, actorEmail string) ([]domain.WatchListUser, time.Time, error) {
	if s.setCaseWatchList != nil {
		return s.setCaseWatchList(ctx, caseID, userIDs, actorEmail)
	}
	panic("not implemented")
}

// AccountDefaultWatcherEmails defaults to empty rather than panicking:
// every case.* publish path now calls it unconditionally to resolve a fresh
// Recipients list, and none of this stub's existing test cases (from before
// this method existed) care about its contents -- same reasoning as
// stubUserRepo's GetUserRoles/GetUserGroups defaults above.
func (s *stubCaseRepo) AccountDefaultWatcherEmails(ctx context.Context, projectID string) ([]string, error) {
	if s.accountDefaultWatcherEmails != nil {
		return s.accountDefaultWatcherEmails(ctx, projectID)
	}
	return nil, nil
}

// GetCaseEtaSharedOn defaults to nil (no fix ETA shared) rather than
// panicking, same reasoning as AccountDefaultWatcherEmails above.
func (s *stubCaseRepo) GetCaseEtaSharedOn(ctx context.Context, caseID string) (*time.Time, error) {
	if s.getCaseEtaSharedOn != nil {
		return s.getCaseEtaSharedOn(ctx, caseID)
	}
	return nil, nil
}

// ProjectContactEmailsByRole defaults to empty rather than panicking, same
// reasoning as AccountDefaultWatcherEmails above: publishCaseCreatedEvent now
// calls it for every announcement-type case create, and none of this stub's
// existing test cases care about its contents.
func (s *stubCaseRepo) ProjectContactEmailsByRole(ctx context.Context, projectID, role string) ([]string, error) {
	if s.projectContactEmailsByRole != nil {
		return s.projectContactEmailsByRole(ctx, projectID, role)
	}
	return nil, nil
}

func (s *stubCaseRepo) UpdateCaseAssignee(ctx context.Context, caseID string, userID *string, callerEmail string) (time.Time, bool, error) {
	if s.updateCaseAssignee != nil {
		return s.updateCaseAssignee(ctx, caseID, userID, callerEmail)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) AcknowledgeCase(ctx context.Context, caseID, actorID, actorEmail string) (bool, domain.AssignedEngineerRef, string, time.Time, error) {
	if s.acknowledgeCase != nil {
		return s.acknowledgeCase(ctx, caseID, actorID, actorEmail)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) UpdateCaseParent(ctx context.Context, caseID, parentID, callerEmail string) (time.Time, error) {
	if s.updateCaseParent != nil {
		return s.updateCaseParent(ctx, caseID, parentID, callerEmail)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) UpdateCaseFields(ctx context.Context, req domain.UpdateCaseRequest, actorID, actorEmail string) (time.Time, error) {
	if s.updateCaseFields != nil {
		return s.updateCaseFields(ctx, req, actorID, actorEmail)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) MarkCaseFixIssued(ctx context.Context, caseID string) (time.Time, bool, error) {
	if s.markCaseFixIssued != nil {
		return s.markCaseFixIssued(ctx, caseID)
	}
	panic("not implemented")
}

func (s *stubCaseRepo) SearchCaseActivities(context.Context, domain.SearchCaseActivitiesRequest) ([]domain.CaseActivity, int, error) {
	panic("not implemented")
}

// RecordCaseFieldChangeActivity defaults to a no-op (not a panic): it's a
// best-effort side effect of many UpdateCase branches now, so most existing
// tests exercising those branches never set this stub up at all. A test that
// specifically wants to assert on it sets recordCaseFieldChangeActivity.
func (s *stubCaseRepo) RecordCaseFieldChangeActivity(ctx context.Context, caseID, fieldName, oldValue, newValue, actorEmail string) error {
	if s.recordCaseFieldChangeActivity != nil {
		return s.recordCaseFieldChangeActivity(ctx, caseID, fieldName, oldValue, newValue, actorEmail)
	}
	return nil
}

// stubUserRepo is a minimal repository.UserRepository; SearchCases doesn't
// exercise it beyond the createdBy-current-user path, which these tests don't
// use.
type stubUserRepo struct {
	getUserByEmail       func(ctx context.Context, email string) (domain.User, error)
	searchUsers          func(ctx context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error)
	getUserDetail        func(ctx context.Context, id string) (domain.UserDetail, error)
	getUserRoles         func(ctx context.Context, id string) ([]string, error)
	getUserGroups        func(ctx context.Context, id string) ([]domain.UserGroupRef, error)
	getUserProjectAccess func(ctx context.Context, email string) ([]domain.UserContactAccess, error)
	createUser           func(ctx context.Context, req domain.CreateUserRequest, actor string) (domain.User, error)
	updateUserTimeZone   func(ctx context.Context, userID, timezone string) (time.Time, error)
}

func (s stubUserRepo) CreateUser(ctx context.Context, req domain.CreateUserRequest, actor string) (domain.User, error) {
	if s.createUser != nil {
		return s.createUser(ctx, req, actor)
	}
	panic("not implemented")
}

func (s stubUserRepo) UpdateUserTimeZone(ctx context.Context, userID, timezone string) (time.Time, error) {
	if s.updateUserTimeZone != nil {
		return s.updateUserTimeZone(ctx, userID, timezone)
	}
	panic("not implemented")
}

func (s stubUserRepo) GetUserDetail(ctx context.Context, id string) (domain.UserDetail, error) {
	if s.getUserDetail != nil {
		return s.getUserDetail(ctx, id)
	}
	panic("not implemented")
}
func (s stubUserRepo) GetUserProjectAccess(ctx context.Context, email string) ([]domain.UserContactAccess, error) {
	if s.getUserProjectAccess != nil {
		return s.getUserProjectAccess(ctx, email)
	}
	panic("not implemented")
}

func (s stubUserRepo) SearchUsers(ctx context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error) {
	if s.searchUsers != nil {
		return s.searchUsers(ctx, req)
	}
	panic("not implemented")
}
func (s stubUserRepo) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	if s.getUserByEmail != nil {
		return s.getUserByEmail(ctx, email)
	}
	panic("not implemented")
}

// GetUserRoles/GetUserGroups return empty rather than panicking: GetMe calls
// both unconditionally after GetUserByEmail succeeds, and none of this
// stub's existing test cases care about their contents.
func (s stubUserRepo) GetUserRoles(ctx context.Context, id string) ([]string, error) {
	if s.getUserRoles != nil {
		return s.getUserRoles(ctx, id)
	}
	return nil, nil
}
func (s stubUserRepo) GetUserGroups(ctx context.Context, id string) ([]domain.UserGroupRef, error) {
	if s.getUserGroups != nil {
		return s.getUserGroups(ctx, id)
	}
	return nil, nil
}

// TestCaseService_SearchCases_RejectsUnsupportedPostgresFields proves the
// Postgres-backed SearchCases path rejects each of these filter fields
// ParseCaseFieldFilters accepts but the Postgres repository has no query
// support for (they dot-walk into SN-specific concepts with no Postgres
// query support yet), rather than silently accepting the request and
// returning a broader-than-requested result set.
func TestCaseService_SearchCases_RejectsUnsupportedPostgresFields(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	cases := []struct {
		name   string
		filter domain.CaseFieldFilter
	}{
		{name: "accountEscalationActive", filter: domain.CaseFieldFilter{Field: "accountEscalationActive", Op: "eq", Values: []string{"true"}}},
		{name: "assignedUserId isEmpty (Unassigned)", filter: domain.CaseFieldFilter{Field: "assignedUserId", Op: "isEmpty"}},
		{name: "resolutionNotes isEmpty", filter: domain.CaseFieldFilter{Field: "resolutionNotes", Op: "isEmpty"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{tc.filter},
			}}
			_, err := svc.SearchCases(ctx, req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestCaseService_SearchCases_SupportedFieldsStillReachRepository proves the
// fields the Postgres repository does support are not caught by the
// unsupported-field rejection: each reaches repo.SearchCases unchanged.
func TestCaseService_SearchCases_SupportedFieldsStillReachRepository(t *testing.T) {
	uuid1 := "00000000-0000-0000-0000-000000000001"

	cases := []struct {
		name   string
		filter domain.CaseFieldFilter
	}{
		{name: "type", filter: domain.CaseFieldFilter{Field: "type", Op: "in", Values: []string{"case"}}},
		{name: "projectId", filter: domain.CaseFieldFilter{Field: "projectId", Op: "in", Values: []string{uuid1}}},
		{name: "deploymentId", filter: domain.CaseFieldFilter{Field: "deploymentId", Op: "in", Values: []string{uuid1}}},
		{name: "state", filter: domain.CaseFieldFilter{Field: "state", Op: "in", Values: []string{"open"}}},
		{name: "severity", filter: domain.CaseFieldFilter{Field: "severity", Op: "in", Values: []string{"high"}}},
		{name: "issueType", filter: domain.CaseFieldFilter{Field: "issueType", Op: "in", Values: []string{"error"}}},
		{name: "engagementType", filter: domain.CaseFieldFilter{Field: "engagementType", Op: "in", Values: []string{"migration"}}},
		{name: "createdBy", filter: domain.CaseFieldFilter{Field: "createdBy", Op: "in", Values: []string{"a@example.com"}}},
		{name: "workState", filter: domain.CaseFieldFilter{Field: "workState", Op: "in", Values: []string{"ongoing"}}},
		{name: "assignedUserId in", filter: domain.CaseFieldFilter{Field: "assignedUserId", Op: "in", Values: []string{uuid1}}},
		{name: "createdOn gte", filter: domain.CaseFieldFilter{Field: "createdOn", Op: "gte", Values: []string{"2026-01-01"}}},
		{name: "escalationLevel in", filter: domain.CaseFieldFilter{Field: "escalationLevel", Op: "in", Values: []string{"1", "2"}}},
		{name: "escalation isNotEmpty", filter: domain.CaseFieldFilter{Field: "escalation", Op: "isNotEmpty"}},
		{name: "escalation isEmpty", filter: domain.CaseFieldFilter{Field: "escalation", Op: "isEmpty"}},
		{name: "tag in", filter: domain.CaseFieldFilter{Field: "tag", Op: "in", Values: []string{"patch"}}},
		{name: "tag notIn", filter: domain.CaseFieldFilter{Field: "tag", Op: "notIn", Values: []string{"s_dip", "patch"}}},
		{name: "projectOnboardingStatus in", filter: domain.CaseFieldFilter{Field: "projectOnboardingStatus", Op: "in", Values: []string{"Completed"}}},
		{name: "projectOnboardingStatus notIn", filter: domain.CaseFieldFilter{Field: "projectOnboardingStatus", Op: "notIn", Values: []string{"In-Progress"}}},
		{name: "taskSLABusinessElapsedPercent gte", filter: domain.CaseFieldFilter{Field: "taskSLABusinessElapsedPercent", Op: "gte", Values: []string{"80"}}},
		{name: "taskSLABusinessElapsedPercent lte 0", filter: domain.CaseFieldFilter{Field: "taskSLABusinessElapsedPercent", Op: "lte", Values: []string{"0"}}},
		{name: "parentId eq", filter: domain.CaseFieldFilter{Field: "parentId", Op: "eq", Values: []string{uuid1}}},
		{name: "projectType in", filter: domain.CaseFieldFilter{Field: "projectType", Op: "in", Values: []string{"Subscription"}}},
		{name: "state notIn", filter: domain.CaseFieldFilter{Field: "state", Op: "notIn", Values: []string{"closed"}}},
		{name: "slaBreached eq", filter: domain.CaseFieldFilter{Field: "slaBreached", Op: "eq", Values: []string{"true"}}},
		{name: "resolvedOn gte", filter: domain.CaseFieldFilter{Field: "resolvedOn", Op: "gte", Values: []string{"2026-01-01"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			repo := &stubCaseRepo{
				searchCases: func(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error) {
					called = true
					return nil, 0, nil
				},
			}
			svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

			req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{tc.filter},
			}}
			if _, err := svc.SearchCases(ctx, req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !called {
				t.Fatalf("expected repo.SearchCases to be called for supported field %q", tc.name)
			}
		})
	}
}

// TestCaseService_SearchCases_ProductAndTeamFiltersReachRepository is the
// regression guard for the same "rejected outright despite a real backing
// column" bug as the other filters above: product, creTeam and sreTeam used
// to 400 unconditionally on this data source, even though caseRepo.SearchCases
// already joins product (for ProductName in every result row) and, once this
// fix's account/"group" joins were added, account.cre_team_id/sre_team_id the
// same way GetCaseByID already resolves them.
func TestCaseService_SearchCases_ProductAndTeamFiltersReachRepository(t *testing.T) {
	creTeamID := "00000000-0000-0000-0000-000000000001"
	sreTeamID := "00000000-0000-0000-0000-000000000002"
	var got domain.SearchCasesRequest
	repo := &stubCaseRepo{
		searchCases: func(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error) {
			got = req
			return nil, 0, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
		Filters: []domain.CaseFieldFilter{
			{Field: "product", Op: "in", Values: []string{"Asgardeo"}},
			{Field: "creTeam", Op: "in", Values: []string{creTeamID}},
			{Field: "sreTeam", Op: "in", Values: []string{sreTeamID}},
		},
	}}
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Parsed.ProductNames) != 1 || got.Parsed.ProductNames[0] != "Asgardeo" {
		t.Errorf("ProductNames = %v, want [Asgardeo]", got.Parsed.ProductNames)
	}
	if len(got.Parsed.CreTeamIDs) != 1 || got.Parsed.CreTeamIDs[0] != creTeamID {
		t.Errorf("CreTeamIDs = %v, want [%s]", got.Parsed.CreTeamIDs, creTeamID)
	}
	if len(got.Parsed.SreTeamIDs) != 1 || got.Parsed.SreTeamIDs[0] != sreTeamID {
		t.Errorf("SreTeamIDs = %v, want [%s]", got.Parsed.SreTeamIDs, sreTeamID)
	}
}

// TestCaseService_SearchCases_RejectsMalformedTeamUUID proves creTeam/sreTeam
// still validate as UUIDs before reaching the repository -- only the blanket
// "unsupported on Postgres" rejection was removed.
func TestCaseService_SearchCases_RejectsMalformedTeamUUID(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	for _, field := range []string{"creTeam", "sreTeam"} {
		t.Run(field, func(t *testing.T) {
			req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: field, Op: "in", Values: []string{"not-a-uuid"}}},
			}}
			_, err := svc.SearchCases(ctx, req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestCaseService_SearchCases_RejectsServiceNowOnlyOptions proves the Postgres
// path rejects the search options that only snCaseService implements: the
// slaBreached and account-escalation filters, and grouped counts.
// caseRepo.SearchCases models none of them, so accepting the request
// would silently drop the predicate and return a wider result set with a 200.
// The stub repository panics if reached, so a passing test proves the
// short-circuit, not merely that the repository ignored the option.
func TestCaseService_SearchCases_RejectsServiceNowOnlyOptions(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	cases := []struct {
		name    string
		req     domain.SearchCasesRequest
		wantMsg string
	}{
		{
			name: "accountEscalationActive",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "accountEscalationActive", Op: "eq", Values: []string{"true"}}},
			}},
			wantMsg: `field "accountEscalationActive" is not supported by this data source`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.SearchCases(ctx, tc.req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
			if ve.Msg != tc.wantMsg {
				t.Errorf("Msg = %q, want %q", ve.Msg, tc.wantMsg)
			}
		})
	}
}

// TestCaseService_SearchCaseComments covers the Postgres-backed comment
// listing path added to close the gap where POST /cases/{id}/comments/search
// was never registered in routes.go, even though comment creation worked and
// this service method (plus its repository query) was already fully
// implemented. Exercises: empty result, a single comment, multiple comments
// with the repository's most-recent-first ordering preserved through to the
// response, and pagination bookkeeping (hasMore).
func TestCaseService_SearchCaseComments(t *testing.T) {
	caseID := "11111111-1111-1111-1111-111111111111"
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

	t.Run("empty case has no comments", func(t *testing.T) {
		repo := &stubCaseRepo{
			searchCaseComments: func(_ context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
				if req.CaseID != caseID {
					t.Fatalf("CaseID = %q, want %q", req.CaseID, caseID)
				}
				return nil, 0, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

		resp, err := svc.SearchCaseComments(context.Background(), domain.SearchCaseCommentsRequest{CaseID: caseID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Comments) != 0 {
			t.Errorf("Comments = %v, want empty", resp.Comments)
		}
		if resp.Total != 0 || resp.HasMore {
			t.Errorf("Total = %d, HasMore = %v, want 0/false", resp.Total, resp.HasMore)
		}
	})

	t.Run("single comment", func(t *testing.T) {
		want := domain.CaseComment{
			ID:        "c1",
			CaseID:    caseID,
			Type:      domain.CommentTypeComment,
			Content:   "hello",
			CreatedBy: domain.NewUserReference("u1", "jane.doe@example.com", "Jane Doe"),
			CreatedOn: now,
		}
		repo := &stubCaseRepo{
			searchCaseComments: func(context.Context, domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
				return []domain.CaseComment{want}, 1, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

		resp, err := svc.SearchCaseComments(context.Background(), domain.SearchCaseCommentsRequest{CaseID: caseID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Comments) != 1 || resp.Comments[0].ID != "c1" {
			t.Fatalf("Comments = %+v, want [%+v]", resp.Comments, want)
		}
		if resp.Total != 1 || resp.HasMore {
			t.Errorf("Total = %d, HasMore = %v, want 1/false", resp.Total, resp.HasMore)
		}
	})

	t.Run("multiple comments preserve repository order and compute hasMore", func(t *testing.T) {
		// The repository orders by created_at DESC (most recent first); the
		// service must not re-sort, only pass the slice through.
		newest := domain.CaseComment{ID: "c3", CaseID: caseID, CreatedOn: now}
		middle := domain.CaseComment{ID: "c2", CaseID: caseID, CreatedOn: now.Add(-time.Hour)}
		oldest := domain.CaseComment{ID: "c1", CaseID: caseID, CreatedOn: now.Add(-2 * time.Hour)}
		repo := &stubCaseRepo{
			searchCaseComments: func(_ context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
				if req.Pagination.Limit != 2 {
					t.Fatalf("Pagination.Limit = %d, want 2 (page size requested)", req.Pagination.Limit)
				}
				// total (5) exceeds what's returned on this page (2 of the 3
				// shown here is illustrative; assert against the 5 below).
				return []domain.CaseComment{newest, middle, oldest}, 5, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

		resp, err := svc.SearchCaseComments(context.Background(), domain.SearchCaseCommentsRequest{
			CaseID:     caseID,
			Pagination: domain.Pagination{Limit: 2},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		gotIDs := []string{resp.Comments[0].ID, resp.Comments[1].ID, resp.Comments[2].ID}
		wantIDs := []string{"c3", "c2", "c1"}
		for i := range wantIDs {
			if gotIDs[i] != wantIDs[i] {
				t.Errorf("Comments[%d].ID = %q, want %q (order must not be reshuffled)", i, gotIDs[i], wantIDs[i])
			}
		}
		if resp.Total != 5 {
			t.Errorf("Total = %d, want 5", resp.Total)
		}
		if !resp.HasMore {
			t.Errorf("HasMore = false, want true (offset 0 + 3 returned < total 5)")
		}
	})

	t.Run("invalid case id is rejected before reaching the repository", func(t *testing.T) {
		repo := &stubCaseRepo{
			searchCaseComments: func(context.Context, domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
				t.Fatal("repository should not be reached for an invalid caseId")
				return nil, 0, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

		_, err := svc.SearchCaseComments(context.Background(), domain.SearchCaseCommentsRequest{CaseID: "not-a-uuid"})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})
}

// TestCaseService_UpdateCase_RejectsTypeTransferFields proves the case-type
// transfer fields are rejected before the
// Postgres-backed UpdateCase ever reaches the repository -- stubCaseRepo's
// UpdateCase panics if called, so a passing test here proves the rejection,
// not just a repository that happens to ignore the field. Postgres-backed
// cases have no engagement_type column and are always type "case" (see
// CreateCase's own "only type \"case\" is supported" guard), so none of these
// fields have anywhere to go on this data source.
func TestCaseService_UpdateCase_RejectsTypeTransferFields(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := context.Background()
	strPtr := func(s string) *string { return &s }
	engagement := domain.EngagementTypeMigration

	cases := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{name: "type", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, Type: strPtr("engagement")}},
		{
			name: "engagementType",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, EngagementType: &engagement},
		},
		{
			name: "catalogId",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, CatalogID: strPtr(testDeploymentUUID)},
		},
		{
			name: "catalogItemId",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, CatalogItemID: strPtr(testDeploymentUUID)},
		},
		{
			name: "variables",
			req: domain.UpdateCaseRequest{
				ID:        testDeploymentUUID,
				Variables: []domain.Variable{{ID: testDeploymentUUID, Value: "x"}},
			},
		},
		{name: "issueType", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, IssueType: func() *domain.CaseIssueType { v := domain.CaseIssueTypeQuestion; return &v }()}},
		{name: "addPublicComment", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, AddPublicComment: func() *bool { v := true; return &v }()}},
		{name: "product", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, Product: strPtr("WSO2 API Manager")}},
		{name: "publicTicket", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, PublicTicket: strPtr("gh-1")}},
		{name: "autocloseHoldUntil", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, AutocloseHoldUntil: func() *time.Time { v := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC); return &v }()}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateCase(ctx, tc.req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestCaseService_UpdateCase_RejectsExclusiveFieldCombinations proves the
// exclusiveCount/combinableCount split (mirroring sn_case_service.go's own
// UpdateCase exactly) rejects every combination its ServiceNow counterpart
// also rejects: two exclusive fields together, or one exclusive field
// alongside anything from the combinable bundle.
func TestCaseService_UpdateCase_RejectsExclusiveFieldCombinations(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := context.Background()
	open := domain.CaseStateOpen
	email := "assignee@example.com"
	ack := true
	subject := "New subject"
	parentID := testDeploymentUUID

	cases := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{name: "state+assigneeEmail", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &open, AssigneeEmail: json.RawMessage(`"` + email + `"`)}},
		{name: "assigneeEmail+acknowledge", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, AssigneeEmail: json.RawMessage(`"` + email + `"`), Acknowledge: &ack}},
		{name: "parentId+watchList", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, ParentID: &parentID, WatchList: &[]string{}}},
		{name: "acknowledge+subject", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, Acknowledge: &ack, Subject: &subject}},
		{name: "state+subject", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &open, Subject: &subject}},
		{name: "nothing", req: domain.UpdateCaseRequest{ID: testDeploymentUUID}},
		// Regression cases for a CodeRabbit finding on PR #1986:
		// resolutionCode/cause/closeNotes aren't counted by exclusiveCount or
		// combinableCount at all, so these used to sail past the mutual-
		// exclusion check and get silently dropped by whichever branch
		// handled the other field.
		{name: "assigneeEmail+closeNotes", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, AssigneeEmail: json.RawMessage(`"` + email + `"`), CloseNotes: &subject}},
		{name: "subject+closeNotes", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, Subject: &subject, CloseNotes: &subject}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateCase(ctx, tc.req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestCaseService_UpdateCase_UpdatesAssignee covers the happy path: the
// assignee is resolved by email, work_item.assigned_to_id is set via
// CaseRepository.UpdateCaseAssignee, the response echoes AssignedTo/
// AssignedToUser, and the change is mirrored to ServiceNow asynchronously.
func TestCaseService_UpdateCase_UpdatesAssignee(t *testing.T) {
	assigneeEmail := "assignee@example.com"
	called := make(chan string, 1)
	mirror := &stubMirrorCaseService{
		patchCaseAssigneeFn: func(_ context.Context, caseID string, email *string) error {
			if email == nil {
				called <- ""
				return nil
			}
			called <- *email
			return nil
		},
	}
	repo := &stubCaseRepo{
		getCaseByID: func(_ context.Context, id string, _ repository.SearchScope) (domain.CaseView, error) {
			// Unassigned before this call, matching this test's intent
			// (assignee changes from nobody to John Roe) -- purely display
			// data for the activity-feed entry now; the actual no-op
			// decision comes from updateCaseAssignee's own "changed" return.
			return domain.CaseView{ID: id, Number: "CS0001"}, nil
		},
		updateCaseAssignee: func(_ context.Context, caseID string, userID *string, callerEmail string) (time.Time, bool, error) {
			if userID == nil || *userID != "assignee-id" {
				t.Errorf("userID = %v, want assignee-id", userID)
			}
			return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), true, nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
		if email == "jane.doe@example.com" {
			return domain.User{ID: "actor-id", Email: email}, nil
		}
		return domain.User{ID: "assignee-id", Email: email, FirstName: "John", LastName: "Roe"}, nil
	}}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, AssigneeEmail: json.RawMessage(`"` + assigneeEmail + `"`)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Case.AssignedTo == nil || resp.Case.AssignedTo.Name != "John Roe" {
		t.Errorf("AssignedTo = %+v, want name John Roe", resp.Case.AssignedTo)
	}
	if resp.Case.AssignedToUser == nil || resp.Case.AssignedToUser.Email != assigneeEmail {
		t.Errorf("AssignedToUser = %+v, want email %q", resp.Case.AssignedToUser, assigneeEmail)
	}

	select {
	case got := <-called:
		if got != assigneeEmail {
			t.Errorf("mirror got %q, want %q", got, assigneeEmail)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.patchCaseAssignee was never called")
	}
}

// TestCaseService_UpdateCase_RejectsEmptyAssigneeEmail proves an explicitly
// empty assigneeEmail is a validation error rather than clearing the
// assignee -- an explicit null (json.RawMessage(`null`)) is the only way to
// clear it; a quoted empty string is still rejected.
func TestCaseService_UpdateCase_RejectsEmptyAssigneeEmail(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	_, err := svc.UpdateCase(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), domain.UpdateCaseRequest{ID: testDeploymentUUID, AssigneeEmail: json.RawMessage(`""`)})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestCaseService_UpdateCase_ClearsAssignee proves an explicit
// assigneeEmail:null PATCH clears work_item.assigned_to_id (nil userID
// passed through to CaseRepository.UpdateCaseAssignee), never publishes
// case.assigned (publishCaseAssigned's own `assigneeEmail == ""` guard),
// records the activity-feed entry with the literal "Unassigned" new value,
// returns AssignedTo/AssignedToUser as nil, and forwards the clear to the
// ServiceNow mirror as a nil *string (not an ambiguous empty string).
func TestCaseService_UpdateCase_ClearsAssignee(t *testing.T) {
	called := make(chan *string, 1)
	mirror := &stubMirrorCaseService{
		patchCaseAssigneeFn: func(_ context.Context, caseID string, email *string) error {
			called <- email
			return nil
		},
	}
	var recordedNewValue string
	repo := &stubCaseRepo{
		getCaseByID: func(_ context.Context, id string, _ repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{
				ID:               id,
				Number:           "CS0002",
				AssignedEngineer: domain.NewUserReference("assignee-id", "john.roe@example.com", "John Roe"),
			}, nil
		},
		updateCaseAssignee: func(_ context.Context, caseID string, userID *string, callerEmail string) (time.Time, bool, error) {
			if userID != nil {
				t.Errorf("userID = %v, want nil (clear)", *userID)
			}
			return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), true, nil
		},
		recordCaseFieldChangeActivity: func(_ context.Context, _, field, _, newValue, _ string) error {
			if field == "assigned_to_id" {
				recordedNewValue = newValue
			}
			return nil
		},
	}
	// resolveActor also goes through GetUserByEmail (for the caller, "jane.doe@example.com"
	// below) -- only an assignee lookup (any other email) is disallowed on a clear.
	userRepo := stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
		if email == "jane.doe@example.com" {
			return domain.User{ID: "actor-id", Email: email}, nil
		}
		t.Fatalf("GetUserByEmail should not be called for an assignee lookup on a clear, got email %q", email)
		return domain.User{}, nil
	}}
	publisher := &mockEventPublisher{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, publisher, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, AssigneeEmail: json.RawMessage(`null`)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Case.AssignedTo != nil {
		t.Errorf("AssignedTo = %+v, want nil", resp.Case.AssignedTo)
	}
	if resp.Case.AssignedToUser != nil {
		t.Errorf("AssignedToUser = %+v, want nil", resp.Case.AssignedToUser)
	}
	if len(publisher.calls) != 0 {
		t.Errorf("case.assigned published on a clear, got %d calls", len(publisher.calls))
	}
	if recordedNewValue != "Unassigned" {
		t.Errorf("activity new value = %q, want Unassigned", recordedNewValue)
	}

	select {
	case got := <-called:
		if got != nil {
			t.Errorf("mirror got %v, want nil (clear)", *got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.patchCaseAssignee was never called")
	}
}

// TestCaseService_UpdateCase_AcknowledgesCase covers the first-claim path:
// CaseRepository.AcknowledgeCase reports alreadyAcknowledged=false, and the
// claim is mirrored to ServiceNow.
func TestCaseService_UpdateCase_AcknowledgesCase(t *testing.T) {
	ack := true
	called := make(chan struct{}, 1)
	mirror := &stubMirrorCaseService{
		patchCaseAcknowledgeFn: func(context.Context, string) error {
			called <- struct{}{}
			return nil
		},
	}
	repo := &stubCaseRepo{
		acknowledgeCase: func(_ context.Context, caseID, actorID, actorEmail string) (bool, domain.AssignedEngineerRef, string, time.Time, error) {
			return false, domain.AssignedEngineerRef{ID: actorID, Name: "Jane Doe"}, "CS0001", time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
		return domain.User{ID: "actor-id", Email: email}, nil
	}}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, Acknowledge: &ack})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Case.AlreadyAcknowledged == nil || *resp.Case.AlreadyAcknowledged {
		t.Errorf("AlreadyAcknowledged = %v, want false", resp.Case.AlreadyAcknowledged)
	}
	if resp.Case.Number != "CS0001" {
		t.Errorf("Number = %q, want CS0001", resp.Case.Number)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.patchCaseAcknowledge was never called")
	}
}

// TestCaseService_UpdateCase_AcknowledgeNoOpSkipsMirror proves a repeat
// Acknowledge:true against an already-acknowledged case reports
// AlreadyAcknowledged=true and skips the ServiceNow mirror entirely -- there
// is nothing new to mirror.
func TestCaseService_UpdateCase_AcknowledgeNoOpSkipsMirror(t *testing.T) {
	ack := true
	mirrorCalled := false
	mirror := &stubMirrorCaseService{
		patchCaseAcknowledgeFn: func(context.Context, string) error {
			mirrorCalled = true
			return nil
		},
	}
	repo := &stubCaseRepo{
		acknowledgeCase: func(context.Context, string, string, string) (bool, domain.AssignedEngineerRef, string, time.Time, error) {
			return true, domain.AssignedEngineerRef{ID: "someone-else", Name: "First Claimer"}, "CS0001", time.Now(), nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
		return domain.User{ID: "actor-id", Email: email}, nil
	}}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, Acknowledge: &ack})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Case.AlreadyAcknowledged == nil || !*resp.Case.AlreadyAcknowledged {
		t.Errorf("AlreadyAcknowledged = %v, want true", resp.Case.AlreadyAcknowledged)
	}
	if resp.Case.AcknowledgedBy == nil || resp.Case.AcknowledgedBy.Name != "First Claimer" {
		t.Errorf("AcknowledgedBy = %+v, want name First Claimer", resp.Case.AcknowledgedBy)
	}
	time.Sleep(50 * time.Millisecond)
	if mirrorCalled {
		t.Error("mirror.patchCaseAcknowledge must not be called for a no-op acknowledge")
	}
}

// TestCaseService_UpdateCase_RejectsAcknowledgeFalse proves Acknowledge only
// accepts true -- there is no unacknowledge.
func TestCaseService_UpdateCase_RejectsAcknowledgeFalse(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	no := false
	_, err := svc.UpdateCase(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), domain.UpdateCaseRequest{ID: testDeploymentUUID, Acknowledge: &no})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestCaseService_UpdateCase_UpdatesParent covers ParentID's own dedicated
// branch, mirrored to ServiceNow.
func TestCaseService_UpdateCase_UpdatesParent(t *testing.T) {
	parentID := "22222222-2222-2222-2222-222222222222"
	called := make(chan string, 1)
	mirror := &stubMirrorCaseService{
		patchCaseParentFn: func(_ context.Context, caseID, gotParentID string) error {
			called <- gotParentID
			return nil
		},
	}
	repo := &stubCaseRepo{
		getCaseByID: func(_ context.Context, id string, _ repository.SearchScope) (domain.CaseView, error) {
			// updateCaseParent now fetches both the case (old parent, if
			// any) and the target parent (its number) around the write, for
			// the activity-feed entry's old/new values.
			if id == parentID {
				return domain.CaseView{ID: id, Number: "CS-PARENT"}, nil
			}
			return domain.CaseView{ID: id, Number: "CS0001"}, nil
		},
		updateCaseParent: func(_ context.Context, caseID, gotParentID, callerEmail string) (time.Time, error) {
			if gotParentID != parentID {
				t.Errorf("parentID = %q, want %q", gotParentID, parentID)
			}
			return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
		return domain.User{ID: "actor-id", Email: email}, nil
	}}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, ParentID: &parentID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got != parentID {
			t.Errorf("mirror got %q, want %q", got, parentID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.patchCaseParent was never called")
	}
}

// TestCaseService_UpdateCase_UpdatesFieldsBundle proves the combinable
// bundle accepts several plain fields together in one request, forwards
// them all to CaseRepository.UpdateCaseFields, and echoes the fix-ETA trio
// back on the response (the only fields in this bundle UpdatedCase has a
// slot for).
func TestCaseService_UpdateCase_UpdatesFieldsBundle(t *testing.T) {
	subject := "Updated subject"
	bestCaseFixEta := "2026-10-01"
	var gotReq domain.UpdateCaseRequest
	repo := &stubCaseRepo{
		updateCaseFields: func(_ context.Context, req domain.UpdateCaseRequest, actorID, actorEmail string) (time.Time, error) {
			gotReq = req
			return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
		return domain.User{ID: "actor-id", Email: email}, nil
	}}
	svc := NewCaseService(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, Subject: &subject, BestCaseFixEta: &bestCaseFixEta})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Subject == nil || *gotReq.Subject != subject {
		t.Errorf("repo saw Subject = %v, want %q", gotReq.Subject, subject)
	}
	if resp.Case.BestCaseFixEta == nil || *resp.Case.BestCaseFixEta != bestCaseFixEta {
		t.Errorf("response BestCaseFixEta = %v, want %q", resp.Case.BestCaseFixEta, bestCaseFixEta)
	}
}

// TestCaseService_UpdateCase_CompletesWorkaroundSLAOnProvided is the
// regression guard for a real, previously-accepted gap: no code path
// anywhere ever completed the workaround SLA clock -- ApplyCaseStateEffects
// only ever paused it, even on close. WorkaroundProvided:true (the
// "Provide Workaround" webapp action) is the one genuine "workaround was
// provided" signal that exists in the domain model, and now completes it.
func TestCaseService_UpdateCase_CompletesWorkaroundSLAOnProvided(t *testing.T) {
	workaroundProvided := true
	repo := &stubCaseRepo{
		updateCaseFields: func(_ context.Context, req domain.UpdateCaseRequest, actorID, actorEmail string) (time.Time, error) {
			return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
		return domain.User{ID: "actor-id", Email: email}, nil
	}}
	slaEngine := &fakeSLAEngineService{}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, nil, nil, slaEngine, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, WorkaroundProvided: &workaroundProvided}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.completeWorkaroundCalls) != 1 || slaEngine.completeWorkaroundCalls[0] != testDeploymentUUID {
		t.Fatalf("expected CompleteWorkaroundClock(%q), got %v", testDeploymentUUID, slaEngine.completeWorkaroundCalls)
	}
}

// TestCaseService_UpdateCase_DoesNotCompleteWorkaroundSLAOnRecall proves the
// hook above only fires on true (provided), not false (a recall) -- there's
// no "reopen a completed clock" operation, so a recall must not trigger it.
func TestCaseService_UpdateCase_DoesNotCompleteWorkaroundSLAOnRecall(t *testing.T) {
	workaroundProvided := false
	repo := &stubCaseRepo{
		updateCaseFields: func(_ context.Context, req domain.UpdateCaseRequest, actorID, actorEmail string) (time.Time, error) {
			return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
		return domain.User{ID: "actor-id", Email: email}, nil
	}}
	slaEngine := &fakeSLAEngineService{}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, nil, nil, slaEngine, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, WorkaroundProvided: &workaroundProvided}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.completeWorkaroundCalls) != 0 {
		t.Errorf("expected no CompleteWorkaroundClock call for a recall, got %v", slaEngine.completeWorkaroundCalls)
	}
}

// TestCaseService_UpdateCase_RejectsMalformedFixEtaDate proves a malformed
// date in the combinable bundle is a validation error before it ever
// reaches the repository (the stub panics if reached).
func TestCaseService_UpdateCase_RejectsMalformedFixEtaDate(t *testing.T) {
	bad := "not-a-date"
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	_, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, BestCaseFixEta: &bad})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestCaseService_UpdateCase_ResolutionFieldsRequireClosedOrSolutionProposedState
// proves resolutionCode/cause/closeNotes -- unlike every other field in the
// combinable bundle -- are only accepted alongside a state transition to
// closed or solution_proposed, mirroring sn_case_service.go's own
// snResolutionStates restriction exactly.
func TestCaseService_UpdateCase_ResolutionFieldsRequireClosedOrSolutionProposedState(t *testing.T) {
	closeNotes := "Fixed"
	open := domain.CaseStateOpen
	closed := domain.CaseStateClosed
	solutionProposed := domain.CaseStateSolutionProposed

	cases := []struct {
		name    string
		req     domain.UpdateCaseRequest
		wantErr bool
	}{
		{name: "no state at all", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, CloseNotes: &closeNotes}, wantErr: true},
		{name: "state open", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &open, CloseNotes: &closeNotes}, wantErr: true},
		{name: "state closed", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &closed, CloseNotes: &closeNotes}, wantErr: false},
		{name: "state solution_proposed", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &solutionProposed, CloseNotes: &closeNotes}, wantErr: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubCaseRepo{
				// UpdateCase's state/workState branch now always fetches
				// the case's prior state (for the activity-feed entry, not
				// just event publishing), so every State-bearing case here
				// needs this stubbed even though none of them care about
				// the activity log itself.
				getCaseByID: func(_ context.Context, id string, _ repository.SearchScope) (domain.CaseView, error) {
					st := domain.CaseStateOpen
					return domain.CaseView{ID: id, State: &st}, nil
				},
				updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
					st := domain.CaseState(*req.State)
					return domain.Case{ID: req.ID, State: &st}, nil, nil
				},
			}
			svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
			_, err := svc.UpdateCase(context.Background(), tc.req)
			if tc.wantErr {
				var ve *apierror.ValidationError
				if !asValidationError(err, &ve) {
					t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestCaseService_UpdateCase_RejectsInvalidCause proves an unrecognized
// Cause value is a validation error before it ever reaches the repository.
func TestCaseService_UpdateCase_RejectsInvalidCause(t *testing.T) {
	closed := domain.CaseStateClosed
	bogus := domain.CaseCause("not_a_real_cause")
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	_, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &closed, Cause: &bogus})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestCaseService_SearchCases_AnyOfReachesRepository proves an anyOf request is
// parsed into OR groups and handed to the repository (the Postgres path used to
// reject it outright), and that a field not allowed inside a branch is still a
// validation error rather than silently dropped.
func TestCaseService_SearchCases_AnyOfReachesRepository(t *testing.T) {
	var got domain.SearchCasesRequest
	repo := &stubCaseRepo{
		searchCases: func(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error) {
			got = req
			return nil, 0, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
		Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open"}}},
		AnyOf: []domain.CaseFilterBranch{
			{Filters: []domain.CaseFieldFilter{{Field: "severity", Op: "in", Values: []string{"critical"}}}},
			{Filters: []domain.CaseFieldFilter{{Field: "escalationLevel", Op: "in", Values: []string{"3", "4"}}}},
		},
	}}
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Parsed.OrGroups) != 2 {
		t.Fatalf("OrGroups = %d, want 2", len(got.Parsed.OrGroups))
	}
	if g := got.Parsed.OrGroups[1]; len(g.EscalationLevels) != 2 || g.EscalationLevels[0] != "3" {
		t.Errorf("second branch escalation levels = %v, want [3 4]", g.EscalationLevels)
	}

	bad := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
		AnyOf: []domain.CaseFilterBranch{
			{Filters: []domain.CaseFieldFilter{{Field: "projectOnboardingStatus", Op: "in", Values: []string{"Completed"}}}},
		},
	}}
	_, err := svc.SearchCases(ctx, bad)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("a field not allowed inside a branch must be a validation error, got %v", err)
	}
}

// stubMirrorCaseService is a minimal CaseService stub that implements
// CreateCase (called directly, synchronously, by createCaseSNFirst) plus the
// two narrow interfaces the async mirror dispatches through: snFieldPatcher
// (patchCaseFields) and snCommentMirror (CreateBareCaseComment). It
// deliberately does NOT implement UpdateCase or the full CreateCaseComment —
// the mirror never calls either of those on this stub, only the bare/lean
// methods, and a stray call to the wrong one should fail loudly (embedding
// CaseService with these left unset means such a call panics on a nil func).
// stubProjectContactRepo backs validateWatchListProjectMembership's tests.
// Only GetProjectContactByUserID is ever called by that check; a stray call
// to SearchProjectContacts panics rather than silently returning nothing.
type stubProjectContactRepo struct {
	getProjectContactByUserID func(ctx context.Context, projectID, userID, callerEmail string) (repository.ProjectContactRow, error)
	searchProjectContacts     func(ctx context.Context, projectID string, req domain.SearchProjectContactsRequest) ([]repository.ProjectContactRow, int, error)
}

func (s *stubProjectContactRepo) SearchProjectContacts(ctx context.Context, projectID string, req domain.SearchProjectContactsRequest, _ string) ([]repository.ProjectContactRow, int, error) {
	if s.searchProjectContacts != nil {
		return s.searchProjectContacts(ctx, projectID, req)
	}
	panic("not implemented")
}

func (s *stubProjectContactRepo) GetProjectContactByUserID(ctx context.Context, projectID, userID, callerEmail string) (repository.ProjectContactRow, error) {
	if s.getProjectContactByUserID != nil {
		return s.getProjectContactByUserID(ctx, projectID, userID, callerEmail)
	}
	panic("not implemented")
}

type stubMirrorCaseService struct {
	CaseService
	createCase                    func(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error)
	patchCaseFieldsFn             func(ctx context.Context, caseID string, state *domain.CaseState, severity *domain.CaseSeverity, workState *domain.CaseWorkState, markFixIssued *bool, resolution *caseResolutionFields) (domain.UpdatedCase, error)
	createBareCaseComment         func(ctx context.Context, caseID string, commentType domain.CommentType, content string) (domain.CaseCommentDetail, error)
	addCaseTagAsFn                func(ctx context.Context, caseID, label, actorEmail string) (domain.Tag, error)
	removeCaseTagFn               func(ctx context.Context, caseID, tagID string) error
	patchCaseWatchListFn          func(ctx context.Context, caseID string, userIDs []string) (domain.UpdatedCase, error)
	patchCaseAssigneeFn           func(ctx context.Context, caseID string, assigneeEmail *string) error
	patchCaseAcknowledgeFn        func(ctx context.Context, caseID string) error
	patchCaseParentFn             func(ctx context.Context, caseID, parentID string) error
	patchCaseFieldsBundleFn       func(ctx context.Context, caseID string, req domain.UpdateCaseRequest) error
	projectContactEmailsByRoleFn  func(ctx context.Context, projectID, role string) ([]string, error)
	accountDefaultWatcherEmailsFn func(ctx context.Context, projectID string) ([]string, error)
	getCaseEtaSharedOnFn          func(ctx context.Context, caseID string) (*time.Time, error)
	// searchCaseCommentsFn backs mirrorInitialSNComments' call right after
	// createCaseSNFirst's own Postgres insert succeeds (case_service.go) --
	// unset in the overwhelming majority of tests here, which don't care
	// about comment mirroring, so it defaults to "no comments, no error"
	// (see SearchCaseComments below) rather than requiring every existing
	// CreateCase test to configure it.
	searchCaseCommentsFn func(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error)
}

func (s *stubMirrorCaseService) ProjectContactEmailsByRole(ctx context.Context, projectID, role string) ([]string, error) {
	if s.projectContactEmailsByRoleFn != nil {
		return s.projectContactEmailsByRoleFn(ctx, projectID, role)
	}
	return nil, nil
}

func (s *stubMirrorCaseService) AccountDefaultWatcherEmails(ctx context.Context, projectID string) ([]string, error) {
	if s.accountDefaultWatcherEmailsFn != nil {
		return s.accountDefaultWatcherEmailsFn(ctx, projectID)
	}
	return nil, nil
}

func (s *stubMirrorCaseService) GetCaseEtaSharedOn(ctx context.Context, caseID string) (*time.Time, error) {
	if s.getCaseEtaSharedOnFn != nil {
		return s.getCaseEtaSharedOnFn(ctx, caseID)
	}
	return nil, nil
}

func (s *stubMirrorCaseService) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
	return s.createCase(ctx, req)
}

func (s *stubMirrorCaseService) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error) {
	if s.searchCaseCommentsFn != nil {
		return s.searchCaseCommentsFn(ctx, req)
	}
	return domain.SearchCaseCommentsResponse{}, nil
}

func (s *stubMirrorCaseService) patchCaseFields(ctx context.Context, caseID string, state *domain.CaseState, severity *domain.CaseSeverity, workState *domain.CaseWorkState, markFixIssued *bool, resolution *caseResolutionFields) (domain.UpdatedCase, error) {
	return s.patchCaseFieldsFn(ctx, caseID, state, severity, workState, markFixIssued, resolution)
}

func (s *stubMirrorCaseService) CreateBareCaseComment(ctx context.Context, caseID string, commentType domain.CommentType, content string) (domain.CaseCommentDetail, error) {
	return s.createBareCaseComment(ctx, caseID, commentType, content)
}

func (s *stubMirrorCaseService) patchCaseAssignee(ctx context.Context, caseID string, assigneeEmail *string) error {
	return s.patchCaseAssigneeFn(ctx, caseID, assigneeEmail)
}

func (s *stubMirrorCaseService) patchCaseAcknowledge(ctx context.Context, caseID string) error {
	return s.patchCaseAcknowledgeFn(ctx, caseID)
}

func (s *stubMirrorCaseService) patchCaseParent(ctx context.Context, caseID, parentID string) error {
	return s.patchCaseParentFn(ctx, caseID, parentID)
}

func (s *stubMirrorCaseService) patchCaseFieldsBundle(ctx context.Context, caseID string, req domain.UpdateCaseRequest) error {
	return s.patchCaseFieldsBundleFn(ctx, caseID, req)
}

func (s *stubMirrorCaseService) AddCaseTagAs(ctx context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
	return s.addCaseTagAsFn(ctx, caseID, label, actorEmail)
}

func (s *stubMirrorCaseService) RemoveCaseTag(ctx context.Context, caseID, tagID string) error {
	return s.removeCaseTagFn(ctx, caseID, tagID)
}

func (s *stubMirrorCaseService) patchCaseWatchList(ctx context.Context, caseID string, userIDs []string) (domain.UpdatedCase, error) {
	return s.patchCaseWatchListFn(ctx, caseID, userIDs)
}

// TestCaseService_UpdateCase_MirrorsFieldToServiceNow is the pilot's core
// regression guard, table-driven over all three mirrored fields
// (State/Severity/WorkState): each must reach the mirror via patchCaseFields
// (the bare, read-free PATCH — see that method's own doc comment) with
// exactly the one field it set and the other two nil, never via the full
// snCaseService.UpdateCase (this stub doesn't even implement that — see
// stubMirrorCaseService's own doc comment).
func TestCaseService_UpdateCase_MirrorsFieldToServiceNow(t *testing.T) {
	state := domain.CaseStateOpen
	severity := domain.CaseSeverityHigh
	workState := domain.CaseWorkStateOngoing

	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{name: "state", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &state}},
		{name: "severity", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &severity}},
		{name: "workState", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, WorkState: &workState}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var gotCaseID string
			var gotState *domain.CaseState
			var gotSeverity *domain.CaseSeverity
			var gotWorkState *domain.CaseWorkState
			called := make(chan struct{})
			mirror := &stubMirrorCaseService{
				patchCaseFieldsFn: func(_ context.Context, caseID string, state *domain.CaseState, severity *domain.CaseSeverity, workState *domain.CaseWorkState, _ *bool, _ *caseResolutionFields) (domain.UpdatedCase, error) {
					mu.Lock()
					gotCaseID, gotState, gotSeverity, gotWorkState = caseID, state, severity, workState
					mu.Unlock()
					close(called)
					return domain.UpdatedCase{}, nil
				},
			}
			failures := &recordingSNWritebackFailures{}
			dispatcher := NewSNWritebackDispatcher(failures)

			repo := &stubCaseRepo{
				// UpdateCase's state/workState branch now always fetches
				// the case's prior state/workState (for the activity-feed
				// entry, not just event publishing) -- irrelevant to this
				// test's own mirror assertions, just needs to not panic.
				getCaseByID: func(_ context.Context, id string, _ repository.SearchScope) (domain.CaseView, error) {
					priorState := domain.CaseStateClosed
					priorWorkState := domain.CaseWorkStatePaused
					return domain.CaseView{ID: id, State: &priorState, WorkState: &priorWorkState}, nil
				},
				updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
					return domain.Case{ID: req.ID, State: req.State, Severity: req.Severity, WorkState: req.WorkState}, req.Severity, nil
				},
			}
			svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

			if _, err := svc.UpdateCase(context.Background(), tc.req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			select {
			case <-called:
			case <-time.After(2 * time.Second):
				t.Fatal("mirror.patchCaseFields was never called")
			}

			mu.Lock()
			defer mu.Unlock()
			if gotCaseID != testDeploymentUUID {
				t.Errorf("mirror got caseID %q, want %q", gotCaseID, testDeploymentUUID)
			}
			wantState, wantSeverity, wantWorkState := tc.req.State, tc.req.Severity, tc.req.WorkState
			if (gotState == nil) != (wantState == nil) || (gotState != nil && *gotState != *wantState) {
				t.Errorf("mirror got state %v, want %v", gotState, wantState)
			}
			if (gotSeverity == nil) != (wantSeverity == nil) || (gotSeverity != nil && *gotSeverity != *wantSeverity) {
				t.Errorf("mirror got severity %v, want %v", gotSeverity, wantSeverity)
			}
			if (gotWorkState == nil) != (wantWorkState == nil) || (gotWorkState != nil && *gotWorkState != *wantWorkState) {
				t.Errorf("mirror got workState %v, want %v", gotWorkState, wantWorkState)
			}
			if got := failures.count(); got != 0 {
				t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
			}
		})
	}
}

// TestCaseService_UpdateCase_PublishesStatusChanged is the regression guard
// for a real bug: caseService.UpdateCase never published case.status_changed
// at all, for any DATA_SOURCE that writes state through this repository
// (not just DATA_SOURCE=postgres-servicenow-dual-write) -- the wiring
// snCaseService.UpdateCase already had (publishStatusChanged) was simply
// missing here. Also proves the display label sent matches ServiceNow's own
// wording (caseStateDisplayLabel), not the raw lowercase domain enum value.
func TestCaseService_UpdateCase_PublishesStatusChanged(t *testing.T) {
	newState := domain.CaseStateWaitingOnWSO2
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			openState := domain.CaseStateOpen
			return domain.CaseView{
				ID: testDeploymentUUID, Number: "CS0001", State: &openState,
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
				WatchList:      []domain.WatchListUser{{Email: "watcher@example.com"}},
			}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			return domain.Case{ID: req.ID, State: req.State}, nil, nil
		},
	}
	publisher := &mockEventPublisher{}
	svc := NewCaseService(repo, stubUserRepo{}, publisher, alwaysUnrestrictedAccess{}, nil)

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &newState}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected exactly 1 publish call, got %d", len(publisher.calls))
	}
	if publisher.calls[0].eventType != events.TypeStatusChanged || publisher.calls[0].entityID != testDeploymentUUID {
		t.Fatalf("unexpected publish call: %+v", publisher.calls[0])
	}
	var payload events.StatusChangedPayload
	if err := json.Unmarshal(publisher.calls[0].payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.NewStatus != "Waiting on WSO2" {
		t.Errorf("NewStatus = %q, want %q (ServiceNow's own display label)", payload.NewStatus, "Waiting on WSO2")
	}
}

// TestCaseService_UpdateCase_StatusChanged_RecipientsIncludeFreshDefaultsAndDropStaleWatchers
// covers both new behaviors at once: the account's default-watcher
// stakeholders are unioned into Recipients fresh (resolved via
// AccountDefaultWatcherEmails, never read from the persisted watch list --
// see CaseRepository.AccountDefaultWatcherEmails' own doc comment), and a
// persisted watcher who is no longer a REGISTERED project contact (and not
// internal staff) is silently dropped by filterActiveWatchListUsers right
// before the email goes out, rather than still being emailed indefinitely.
func TestCaseService_UpdateCase_StatusChanged_RecipientsIncludeFreshDefaultsAndDropStaleWatchers(t *testing.T) {
	const projectID = "proj-1"
	const activeWatcherID = "11111111-1111-1111-1111-111111111111"
	const staleWatcherID = "22222222-2222-2222-2222-222222222222"
	newState := domain.CaseStateWaitingOnWSO2

	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			openState := domain.CaseStateOpen
			return domain.CaseView{
				ID: testDeploymentUUID, Number: "CS0001", State: &openState,
				ProjectDetails: &domain.EntityRef{ID: projectID, Name: "Project One"},
				WatchList: []domain.WatchListUser{
					{ID: activeWatcherID, Email: "active@example.com"},
					{ID: staleWatcherID, Email: "stale@example.com"},
				},
			}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			return domain.Case{ID: req.ID, State: req.State}, nil, nil
		},
		accountDefaultWatcherEmails: func(_ context.Context, gotProjectID string) ([]string, error) {
			if gotProjectID != projectID {
				t.Errorf("AccountDefaultWatcherEmails projectID = %q, want %q", gotProjectID, projectID)
			}
			return []string{"tow@example.com"}, nil
		},
	}
	contactRepo := &stubProjectContactRepo{
		getProjectContactByUserID: func(_ context.Context, _ string, userID, _ string) (repository.ProjectContactRow, error) {
			if userID == activeWatcherID {
				return repository.ProjectContactRow{Email: "active@example.com", RegistrationState: "REGISTERED"}, nil
			}
			return repository.ProjectContactRow{}, &apierror.NotFoundError{Msg: "no project contact"}
		},
	}
	userRepo := stubUserRepo{getUserDetail: func(_ context.Context, id string) (domain.UserDetail, error) {
		// staleWatcherID resolves to a real user, but neither a registered
		// contact nor internal staff -- the case this whole check exists
		// for (left the project, account not deactivated outright).
		return domain.UserDetail{ID: id, Email: "stale@example.com", UserType: domain.UserTypeExternal}, nil
	}}
	publisher := &mockEventPublisher{}
	svc := NewCaseService(repo, userRepo, publisher, alwaysUnrestrictedAccess{}, contactRepo)

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &newState}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected exactly 1 publish call, got %d", len(publisher.calls))
	}
	var payload events.StatusChangedPayload
	if err := json.Unmarshal(publisher.calls[0].payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	wantPresent := []string{"active@example.com", "tow@example.com"}
	for _, email := range wantPresent {
		found := false
		for _, got := range payload.Recipients {
			if got == email {
				found = true
			}
		}
		if !found {
			t.Errorf("Recipients = %v, want it to include %q", payload.Recipients, email)
		}
	}
	for _, got := range payload.Recipients {
		if got == "stale@example.com" {
			t.Errorf("Recipients = %v, want it to NOT include the stale watcher's email", payload.Recipients)
		}
	}
}

// TestCaseService_UpdateCase_DoesNotPublishStatusChangedOnNoOp proves a
// caller re-PATCHing the case's current state does not send every watcher a
// false "status changed" notification -- the same no-op guard
// snCaseService.UpdateCase already applies.
func TestCaseService_UpdateCase_DoesNotPublishStatusChangedOnNoOp(t *testing.T) {
	sameState := domain.CaseStateOpen
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ID: testDeploymentUUID, State: &sameState}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			return domain.Case{ID: req.ID, State: req.State}, nil, nil
		},
	}
	publisher := &mockEventPublisher{}
	svc := NewCaseService(repo, stubUserRepo{}, publisher, alwaysUnrestrictedAccess{}, nil)

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &sameState}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Errorf("expected no publish call for a no-op state re-PATCH, got %d", len(publisher.calls))
	}
}

// TestCaseService_UpdateCase_PublishesSeverityChanged is the regression
// guard for the case.severity_changed half of the same gap
// TestCaseService_UpdateCase_PublishesStatusChanged closes for
// case.status_changed.
func TestCaseService_UpdateCase_PublishesSeverityChanged(t *testing.T) {
	newSeverity := domain.CaseSeverityCritical
	oldSeverity := domain.CaseSeverityLow
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{
				ID: testDeploymentUUID, Number: "CS0001",
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
				WatchList:      []domain.WatchListUser{{Email: "watcher@example.com"}},
			}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			os := oldSeverity
			return domain.Case{ID: req.ID, Severity: req.Severity}, &os, nil
		},
	}
	publisher := &mockEventPublisher{}
	svc := NewCaseService(repo, stubUserRepo{}, publisher, alwaysUnrestrictedAccess{}, nil)

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &newSeverity}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected exactly 1 publish call, got %d", len(publisher.calls))
	}
	if publisher.calls[0].eventType != events.TypeSeverityChanged {
		t.Fatalf("unexpected publish call: %+v", publisher.calls[0])
	}
	var payload events.SeverityChangedPayload
	if err := json.Unmarshal(publisher.calls[0].payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.OldSeverity != "LOW" || payload.NewSeverity != "CRITICAL" {
		t.Errorf("payload severities = %q -> %q, want LOW -> CRITICAL", payload.OldSeverity, payload.NewSeverity)
	}
}

// TestCaseService_UpdateCase_PublishesSeverityChangedOnFirstSet is the
// regression guard for a real gap: domain.Case's own doc comment says
// Severity is nil for the large majority of real Postgres cases (see
// domain.go), so the very first severity ever set on such a case must still
// publish case.severity_changed with an empty OldSeverity -- exactly what
// snCaseService.UpdateCase already does via derefSeverity. A prior revision
// of this guard required oldSeverity != nil, which silently skipped every
// one of these first-time sets.
func TestCaseService_UpdateCase_PublishesSeverityChangedOnFirstSet(t *testing.T) {
	newSeverity := domain.CaseSeverityCritical
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{
				ID: testDeploymentUUID, Number: "CS0001",
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
				WatchList:      []domain.WatchListUser{{Email: "watcher@example.com"}},
			}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			return domain.Case{ID: req.ID, Severity: req.Severity}, nil, nil
		},
	}
	publisher := &mockEventPublisher{}
	svc := NewCaseService(repo, stubUserRepo{}, publisher, alwaysUnrestrictedAccess{}, nil)

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &newSeverity}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected exactly 1 publish call, got %d", len(publisher.calls))
	}
	var payload events.SeverityChangedPayload
	if err := json.Unmarshal(publisher.calls[0].payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.OldSeverity != "" || payload.NewSeverity != "CRITICAL" {
		t.Errorf("payload severities = %q -> %q, want \"\" -> CRITICAL", payload.OldSeverity, payload.NewSeverity)
	}
}

// TestCaseService_UpdateCase_DoesNotPublishSeverityChangedOnNoOp mirrors
// TestCaseService_UpdateCase_DoesNotPublishStatusChangedOnNoOp for severity.
func TestCaseService_UpdateCase_DoesNotPublishSeverityChangedOnNoOp(t *testing.T) {
	sameSeverity := domain.CaseSeverityLow
	repo := &stubCaseRepo{
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			os := sameSeverity
			return domain.Case{ID: req.ID, Severity: req.Severity}, &os, nil
		},
	}
	publisher := &mockEventPublisher{}
	svc := NewCaseService(repo, stubUserRepo{}, publisher, alwaysUnrestrictedAccess{}, nil)

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &sameSeverity}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Errorf("expected no publish call for a no-op severity re-PATCH, got %d", len(publisher.calls))
	}
}

// TestCaseService_UpdateCase_AppliesStateSLAEffects is the regression guard
// for a real, live-observed gap: the Postgres/dual-write path never
// paused/resumed/completed workaround or resolution clocks on any state
// transition at all -- SLAEngineService.ApplyCaseStateEffects was only ever
// wired into snCaseService.UpdateCase (the plain ServiceNow data source).
// Fires unconditionally on req.State != nil, even a no-op re-PATCH -- every
// effect ApplyCaseStateEffects applies is idempotent, matching
// snCaseService's own call site (see its own doc comment).
func TestCaseService_UpdateCase_AppliesStateSLAEffects(t *testing.T) {
	newState := domain.CaseStateAwaitingInfo
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			openState := domain.CaseStateOpen
			return domain.CaseView{ID: testDeploymentUUID, State: &openState}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			return domain.Case{ID: req.ID, State: req.State}, nil, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, nil, nil, slaEngine, "")

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &newState}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.stateCalls) != 1 || slaEngine.stateCalls[0].caseID != testDeploymentUUID || slaEngine.stateCalls[0].state != newState {
		t.Fatalf("expected ApplyCaseStateEffects(%q, %q), got %v", testDeploymentUUID, newState, slaEngine.stateCalls)
	}
}

// TestCaseService_UpdateCase_RevisesSLAClocksOnSeverityChange is the
// regression guard for the severity half of the same gap
// TestCaseService_UpdateCase_AppliesStateSLAEffects closes for state:
// the Postgres/dual-write path never cancelled a case's old-severity clocks
// and registered fresh ones on a severity change -- SLAEngineService.
// ReviseCaseClocks was only ever wired into snCaseService.UpdateCase.
// Gated on a genuine change, unlike ApplyCaseStateEffects above -- matching
// reviseCaseSLAClocks' own call site (sn_case_service.go), which only fires
// when severity actually changed.
func TestCaseService_UpdateCase_RevisesSLAClocksOnSeverityChange(t *testing.T) {
	newSeverity := domain.CaseSeverityCritical
	oldSeverity := domain.CaseSeverityLow
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{
				ID: testDeploymentUUID, Number: "CS0001",
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
			}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			os := oldSeverity
			return domain.Case{ID: req.ID, Severity: req.Severity}, &os, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, nil, nil, slaEngine, "")

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &newSeverity}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.reviseCalls) != 1 {
		t.Fatalf("expected exactly 1 ReviseCaseClocks call, got %d", len(slaEngine.reviseCalls))
	}
	got := slaEngine.reviseCalls[0]
	if got.caseID != testDeploymentUUID || got.severity == nil || *got.severity != domain.CaseSeverityCritical || got.projectID != "proj-1" {
		t.Errorf("unexpected ReviseCaseClocks call: %+v", got)
	}
}

// TestCaseService_UpdateCase_DoesNotReviseSLAClocksOnNoOp proves the hook
// above is genuinely gated on a real change, not unconditional like
// ApplyCaseStateEffects -- a caller re-PATCHing the case's current severity
// must not spuriously cancel and re-register clocks that never changed.
func TestCaseService_UpdateCase_DoesNotReviseSLAClocksOnNoOp(t *testing.T) {
	sameSeverity := domain.CaseSeverityLow
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ID: testDeploymentUUID, Number: "CS0001"}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			os := sameSeverity
			return domain.Case{ID: req.ID, Severity: req.Severity}, &os, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, nil, nil, slaEngine, "")

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &sameSeverity}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slaEngine.reviseCalls) != 0 {
		t.Errorf("expected no ReviseCaseClocks call for a no-op severity re-PATCH, got %d", len(slaEngine.reviseCalls))
	}
}

// TestCaseService_UpdateCase_ReappliesStateSLAEffectsAfterSeverityRevision is
// the regression guard for a real bug CodeRabbit caught on this PR:
// ReviseCaseClocks' own replacement clocks always start IN_PROGRESS, with
// no awareness of the case's current state -- a case already paused
// (Awaiting Info/Solution Proposed) at the moment its severity changes
// would otherwise get fresh workaround/resolution clocks that immediately
// start counting down unpaused, producing a false breach later. Proves
// ApplyCaseStateEffects is re-applied against the case's current (paused)
// state right after ReviseCaseClocks, using the same GetCaseByID fetch.
func TestCaseService_UpdateCase_ReappliesStateSLAEffectsAfterSeverityRevision(t *testing.T) {
	newSeverity := domain.CaseSeverityCritical
	oldSeverity := domain.CaseSeverityLow
	awaitingInfo := domain.CaseStateAwaitingInfo
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{
				ID: testDeploymentUUID, Number: "CS0001", State: &awaitingInfo,
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
			}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			os := oldSeverity
			return domain.Case{ID: req.ID, Severity: req.Severity}, &os, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, nil, nil, slaEngine, "")

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &newSeverity}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.reviseCalls) != 1 {
		t.Fatalf("expected exactly 1 ReviseCaseClocks call, got %d", len(slaEngine.reviseCalls))
	}
	if len(slaEngine.stateCalls) != 1 || slaEngine.stateCalls[0].caseID != testDeploymentUUID || slaEngine.stateCalls[0].state != awaitingInfo {
		t.Fatalf("expected ApplyCaseStateEffects(%q, %q) to re-pause the fresh clocks, got %v", testDeploymentUUID, awaitingInfo, slaEngine.stateCalls)
	}
}

// TestCaseService_UpdateCase_RecordsSNWritebackFailureOnMirrorError covers
// the failure path: Postgres already committed by the time Dispatch runs, so
// a failed mirror write must not surface as an UpdateCase error — it's
// recorded to sn_writeback_failures instead (see SNWritebackDispatcher).
func TestCaseService_UpdateCase_RecordsSNWritebackFailureOnMirrorError(t *testing.T) {
	mirror := &stubMirrorCaseService{
		patchCaseFieldsFn: func(context.Context, string, *domain.CaseState, *domain.CaseSeverity, *domain.CaseWorkState, *bool, *caseResolutionFields) (domain.UpdatedCase, error) {
			return domain.UpdatedCase{}, errors.New("sn downstream unreachable")
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)

	workState := domain.CaseWorkStatePaused
	repo := &stubCaseRepo{
		getCaseByID: func(_ context.Context, id string, _ repository.SearchScope) (domain.CaseView, error) {
			priorWorkState := domain.CaseWorkStateOngoing
			return domain.CaseView{ID: id, WorkState: &priorWorkState}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			return domain.Case{ID: req.ID, WorkState: req.WorkState}, nil, nil
		},
	}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	resp, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, WorkState: &workState})
	if err != nil {
		t.Fatalf("UpdateCase must still succeed on a failed best-effort mirror write, got: %v", err)
	}
	if resp.Case.WorkState == nil || *resp.Case.WorkState != workState {
		t.Errorf("UpdateCase response WorkState = %v, want %v", resp.Case.WorkState, workState)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
	req := failures.calls[0]
	if req.EntityType != "case" || req.EntityID != testDeploymentUUID || req.Operation != "update" {
		t.Errorf("unexpected failure record: %+v", req)
	}
}

// TestCaseService_UpdateCase_MarkFixIssued_MirrorsToServiceNow proves a
// first-time markFixIssued write persists via MarkCaseFixIssued and
// dispatches an async ServiceNow mirror carrying markFixIssued:true, the
// same shape as State/Severity/WorkState's own mirror
// (TestCaseService_UpdateCase_MirrorsFieldToServiceNow).
func TestCaseService_UpdateCase_MarkFixIssued_MirrorsToServiceNow(t *testing.T) {
	fixIssuedOn := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

	var mu sync.Mutex
	var gotCaseID string
	var gotMarkFixIssued *bool
	called := make(chan struct{})
	mirror := &stubMirrorCaseService{
		patchCaseFieldsFn: func(_ context.Context, caseID string, state *domain.CaseState, severity *domain.CaseSeverity, workState *domain.CaseWorkState, markFixIssued *bool, resolution *caseResolutionFields) (domain.UpdatedCase, error) {
			mu.Lock()
			gotCaseID, gotMarkFixIssued = caseID, markFixIssued
			mu.Unlock()
			close(called)
			return domain.UpdatedCase{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)

	repo := &stubCaseRepo{
		markCaseFixIssued: func(_ context.Context, caseID string) (time.Time, bool, error) {
			if caseID != testDeploymentUUID {
				t.Errorf("MarkCaseFixIssued got caseID %q, want %q", caseID, testDeploymentUUID)
			}
			return fixIssuedOn, false, nil
		},
	}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	markFixIssued := true
	resp, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, MarkFixIssued: &markFixIssued})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Case.FixIssued == nil || !resp.Case.FixIssued.Equal(fixIssuedOn) {
		t.Errorf("resp.Case.FixIssued = %v, want %v", resp.Case.FixIssued, fixIssuedOn)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.patchCaseFields was never called")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotCaseID != testDeploymentUUID {
		t.Errorf("mirror got caseID %q, want %q", gotCaseID, testDeploymentUUID)
	}
	if gotMarkFixIssued == nil || !*gotMarkFixIssued {
		t.Errorf("mirror got markFixIssued %v, want true", gotMarkFixIssued)
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestCaseService_UpdateCase_MarkFixIssued_AlreadySetIsNoOp proves a second
// markFixIssued:true call on a case whose fix is already marked issued
// succeeds without error and does NOT dispatch a second ServiceNow mirror
// write -- there is nothing new to mirror.
func TestCaseService_UpdateCase_MarkFixIssued_AlreadySetIsNoOp(t *testing.T) {
	existingFixIssuedOn := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

	mirrorCalled := false
	mirror := &stubMirrorCaseService{
		patchCaseFieldsFn: func(context.Context, string, *domain.CaseState, *domain.CaseSeverity, *domain.CaseWorkState, *bool, *caseResolutionFields) (domain.UpdatedCase, error) {
			mirrorCalled = true
			return domain.UpdatedCase{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)

	repo := &stubCaseRepo{
		markCaseFixIssued: func(context.Context, string) (time.Time, bool, error) {
			return existingFixIssuedOn, true, nil
		},
	}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	markFixIssued := true
	resp, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, MarkFixIssued: &markFixIssued})
	if err != nil {
		t.Fatalf("a repeat markFixIssued call on an already-marked case must succeed, not error: %v", err)
	}
	if resp.Case.FixIssued == nil || !resp.Case.FixIssued.Equal(existingFixIssuedOn) {
		t.Errorf("resp.Case.FixIssued = %v, want the existing timestamp %v", resp.Case.FixIssued, existingFixIssuedOn)
	}

	// Give any wrongly-dispatched async mirror a moment to run before asserting
	// it never did.
	time.Sleep(50 * time.Millisecond)
	if mirrorCalled {
		t.Error("mirror.patchCaseFields must not be called for a no-op (already-set) markFixIssued update")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records, got %d", got)
	}
}

// TestCaseService_UpdateCase_MarkFixIssued_RejectsFalse proves false is
// rejected -- markFixIssued is a one-way claim like Acknowledge, with no
// unmark path.
func TestCaseService_UpdateCase_MarkFixIssued_RejectsFalse(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

	markFixIssued := false
	_, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, MarkFixIssued: &markFixIssued})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("markFixIssued:false must be a validation error, got %v", err)
	}
}

// TestCaseService_UpdateCase_MarkFixIssued_RejectsCombinationWithOtherFields
// proves markFixIssued cannot be combined with State/Severity/WorkState/
// WatchList in the same request, mirroring WatchList's own mutual-exclusion
// guard.
func TestCaseService_UpdateCase_MarkFixIssued_RejectsCombinationWithOtherFields(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	markFixIssued := true
	state := domain.CaseStateOpen
	severity := domain.CaseSeverityHigh
	workState := domain.CaseWorkStateOngoing
	watchList := []string{}

	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{name: "state", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, MarkFixIssued: &markFixIssued, State: &state}},
		{name: "severity", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, MarkFixIssued: &markFixIssued, Severity: &severity}},
		{name: "workState", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, MarkFixIssued: &markFixIssued, WorkState: &workState}},
		{name: "watchList", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, MarkFixIssued: &markFixIssued, WatchList: &watchList}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateCase(context.Background(), tc.req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("combining markFixIssued with %s must be a validation error, got %v", tc.name, err)
			}
		})
	}
}

// validCreateCaseRequest returns a minimally valid, type="case" CreateCase
// request — one of the two types this data source supports, alongside
// "announcement" (see CreateCase's own "only type \"case\" or \"announcement\""
// check; validAnnouncementCreateCaseRequest below covers the other one).
func validCreateCaseRequest() domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type:              "case",
		ProjectID:         testDeploymentUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeploymentUUID,
		Subject:           "Something is broken",
		Description:       "Steps to reproduce the problem",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}
}

// validAnnouncementCreateCaseRequest returns a minimally valid,
// type="announcement" CreateCase request -- no deploymentId/deployedProductId,
// same as validateCreateCaseRequest's own announcement conditional requires.
func validAnnouncementCreateCaseRequest() domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type:        "announcement",
		ProjectID:   testDeploymentUUID,
		Subject:     "Scheduled maintenance window",
		Description: "Maintenance details go here",
	}
}

// TestCaseService_CreateCase_SNFailureLeavesPostgresUntouched is the pilot's
// core regression guard for the orphan bug this whole design change exists
// to fix: if ServiceNow never accepts the case, the Postgres repository must
// never be called at all — no row, no orphan. Also guards createCaseSNFirst's
// single-attempt behavior: the SN mirror must be called exactly once, with
// no internal retry (see that function's own doc comment for why an internal
// retry was removed).
func TestCaseService_CreateCase_SNFailureLeavesPostgresUntouched(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return domain.CreateCaseResponse{}, errors.New("sn downstream unreachable")
		},
	}
	// No createCaseFromServiceNow override -- stubCaseRepo panics if it's
	// ever called, which is exactly the assertion: Postgres must stay
	// untouched.
	repo := &stubCaseRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	_, err := svc.CreateCase(context.Background(), validCreateCaseRequest())
	if err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the case")
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("expected exactly 1 SN attempt (no internal retry), got %d", attempts)
	}
}

// TestCaseService_CreateCase_SNSuccessCreatesPostgresRowWithMatchingIdentity
// covers the other half: on ServiceNow success, the Postgres insert must use
// EXACTLY the id/number/internalId/createdBy ServiceNow returned -- not
// anything generated locally -- so both systems agree on identity from the
// moment the Postgres row exists.
func TestCaseService_CreateCase_SNSuccessCreatesPostgresRowWithMatchingIdentity(t *testing.T) {
	const (
		snID         = "33333333-3333-3333-3333-333333333333"
		snNumber     = "CS0023001"
		snInternalID = "WSO2-CS-1"
		snCreatedBy  = "jane.doe@example.com"
	)
	createdOn := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{
				Message: "Case created successfully.",
				Case: domain.CreateCaseDetails{
					ID: snID, InternalID: snInternalID, Number: snNumber,
					CreatedBy: snCreatedBy, CreatedOn: createdOn, State: "Open",
				},
			}, nil
		},
	}

	var mu sync.Mutex
	var gotID, gotNumber, gotWso2ID, gotCreatedBy string
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error) {
			mu.Lock()
			gotID, gotNumber, gotWso2ID, gotCreatedBy = id, number, wso2ID, createdBy
			mu.Unlock()
			respState := domain.CaseStateOpen
			return domain.Case{
				ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy,
				CreatedOn: createdOn, State: &respState,
			}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	resp, err := svc.CreateCase(context.Background(), validCreateCaseRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotID != snID || gotNumber != snNumber || gotWso2ID != snInternalID || gotCreatedBy != snCreatedBy {
		t.Errorf("CreateCaseFromServiceNow got (%q, %q, %q, %q), want (%q, %q, %q, %q)",
			gotID, gotNumber, gotWso2ID, gotCreatedBy, snID, snNumber, snInternalID, snCreatedBy)
	}
	if resp.Case.ID != snID || resp.Case.Number != snNumber || resp.Case.InternalID != snInternalID {
		t.Errorf("CreateCase response = %+v, want identity matching ServiceNow's (%q, %q, %q)", resp.Case, snID, snNumber, snInternalID)
	}
}

// TestCaseService_CreateCase_MirrorsInitialServiceNowComments is the
// regression guard for a real, reported bug: "the first comment of a case
// should be the title and description" worked in an environment reading
// comments live from ServiceNow but not under dual-write, because nothing
// copied ServiceNow's own auto-generated first journal entry into Postgres's
// comment table. createCaseSNFirst must now fetch whatever comments
// ServiceNow already has for the case right after its own Postgres insert
// succeeds, and write each one (except "activity", which CreateCaseComment
// itself refuses) into Postgres via CreateCaseCommentMirrored -- also
// proving each one's ServiceNow createdOn is preserved rather than
// stamped NOW(), which SearchCaseComments' "ORDER BY created_on DESC"
// would otherwise misorder relative to the real chronology (CodeRabbit
// caught this on PR #2204).
func TestCaseService_CreateCase_MirrorsInitialServiceNowComments(t *testing.T) {
	const snID = "44444444-4444-4444-4444-444444444444"
	firstCommentOn := time.Date(2026, 9, 30, 9, 50, 24, 0, time.UTC)
	secondCommentOn := time.Date(2026, 9, 30, 9, 50, 25, 0, time.UTC)
	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{
				Case: domain.CreateCaseDetails{ID: snID, Number: "CS0023005", InternalID: "WSO2-CS-5", CreatedBy: "jane.doe@example.com", State: "Open"},
			}, nil
		},
		searchCaseCommentsFn: func(_ context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error) {
			if req.CaseID != snID {
				t.Errorf("SearchCaseComments caseId = %q, want %q", req.CaseID, snID)
			}
			return domain.SearchCaseCommentsResponse{
				Comments: []domain.CaseComment{
					{
						ID: "c1", Type: domain.CommentTypeComment,
						Content:   "<p><strong>Description:</strong></p><p>Something is broken.</p>",
						CreatedBy: &domain.UserReference{Email: "jane.doe@example.com", Name: "Jane Doe"},
						CreatedOn: firstCommentOn,
					},
					{
						ID: "c2", Type: domain.CommentTypeComment,
						Content:   "Change request (CHG0039108) has been created.",
						CreatedBy: &domain.UserReference{Email: "system", Name: "system"},
						CreatedOn: secondCommentOn,
					},
					{
						// ServiceNow's own audit trail -- must be skipped, not
						// forwarded to a CreateCaseComment that would reject it.
						ID: "c3", Type: domain.CommentTypeActivity, Content: "State changed to Open",
					},
				},
			}, nil
		},
	}

	type mirroredCall struct {
		req       domain.CreateCaseCommentRequest
		createdOn time.Time
	}
	var mu sync.Mutex
	var mirrored []mirroredCall
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error) {
			respState := domain.CaseStateOpen
			return domain.Case{ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy, State: &respState}, nil
		},
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, createdOn *time.Time) (domain.CaseComment, error) {
			if createdOn == nil {
				t.Fatalf("mirrorInitialSNComments must pass a non-nil createdOn, got nil for %+v", req)
			}
			mu.Lock()
			mirrored = append(mirrored, mirroredCall{req: req, createdOn: *createdOn})
			mu.Unlock()
			return domain.CaseComment{ID: "mirrored", CaseID: req.CaseID, Type: req.Type, Content: req.Content, CreatedOn: *createdOn}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	if _, err := svc.CreateCase(context.Background(), validCreateCaseRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(mirrored) != 2 {
		t.Fatalf("mirrored %d comments, want 2 (the activity-typed one must be skipped): %+v", len(mirrored), mirrored)
	}
	if mirrored[0].req.CaseID != snID || mirrored[0].req.Content != "<p><strong>Description:</strong></p><p>Something is broken.</p>" || mirrored[0].req.CreatedBy != "jane.doe@example.com" {
		t.Errorf("mirrored[0].req = %+v, want the description comment attributed to jane.doe@example.com", mirrored[0].req)
	}
	if !mirrored[0].createdOn.Equal(firstCommentOn) {
		t.Errorf("mirrored[0].createdOn = %v, want ServiceNow's own %v (not NOW())", mirrored[0].createdOn, firstCommentOn)
	}
	if mirrored[1].req.CreatedBy != "system" {
		t.Errorf("mirrored[1].req.CreatedBy = %q, want %q", mirrored[1].req.CreatedBy, "system")
	}
	if !mirrored[1].createdOn.Equal(secondCommentOn) {
		t.Errorf("mirrored[1].createdOn = %v, want ServiceNow's own %v (not NOW())", mirrored[1].createdOn, secondCommentOn)
	}
}

// TestCaseService_CreateCase_PublishesOnlyAfterPostgresSucceeds is the
// regression guard for a real bug: createCaseSNFirst never called
// publishCaseCreatedEvent at all, so case.created never fired for
// DATA_SOURCE=postgres-servicenow-dual-write's case pilot even with Event
// Hub fully configured and enabled -- the wiring incidentService.
// createIncidentSNFirst already had (see
// TestIncidentService_CreateIncident_PublishesOnlyAfterPostgresSucceeds) was
// simply missing here. Also proves the event only fires after
// CreateCaseFromServiceNow has actually confirmed the Postgres row, same
// ordering guarantee as incident's own version -- never right after the
// ServiceNow POST, which a consumer could observe before the Postgres
// -backed read API (the only one live in this mode) can return anything for
// it.
func TestCaseService_CreateCase_PublishesOnlyAfterPostgresSucceeds(t *testing.T) {
	const caseID = "44444444-4444-4444-4444-444444444444"
	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{
				Message: "Case created successfully.",
				Case: domain.CreateCaseDetails{
					ID: caseID, InternalID: "WSO2-CS-2", Number: "CS0023002",
					CreatedBy: "jane.doe@example.com", State: "Open",
				},
			}, nil
		},
	}
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error) {
			respState := domain.CaseStateOpen
			return domain.Case{ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy, State: &respState}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			severity := domain.CaseSeverityHigh
			return domain.CaseView{
				ID: caseID, Number: "CS0023002", InternalID: "WSO2-CS-2", Subject: "s", Description: "d",
				CreatedOn:      time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
				WatchList:      []domain.WatchListUser{{Email: "watcher@example.com"}},
				Severity:       &severity,
			}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	publisher := &mockEventPublisher{}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, publisher, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	if _, err := svc.CreateCase(context.Background(), validCreateCaseRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected exactly 1 publish call after Postgres success, got %d", len(publisher.calls))
	}
	if publisher.calls[0].eventType != events.TypeCaseCreated || publisher.calls[0].entityID != caseID {
		t.Errorf("unexpected publish call: %+v", publisher.calls[0])
	}
}

// TestCaseService_CreateCase_RegistersSLAClocksOnlyAfterPostgresSucceeds is
// the regression guard for a real, live-observed bug: SLA clock
// registration used to run unconditionally inside snCaseService.CreateCase,
// reached via snMirror.CreateCase BEFORE this service's own Postgres insert
// below. "sla".work_item_id has a hard foreign key against work_item(id)
// (migration 0048), so every dual-write case creation failed clock
// registration with a foreign-key violation (SQLSTATE 23503) — logged
// ("sla engine: register clock failed"), never surfaced, never retried, so
// no SLA breach alert ever fired for a dual-write case. Fixed the same way
// TestCaseService_CreateCase_PublishesOnlyAfterPostgresSucceeds's own
// publish call already was: the mirror's own automatic registration is
// disabled (routes.go constructs snCaseMirrorSvc with a nil slaEngine), and
// caseService itself registers the clocks — see
// registerCaseSLAClocksEvent — once its own insert has actually succeeded.
func TestCaseService_CreateCase_RegistersSLAClocksOnlyAfterPostgresSucceeds(t *testing.T) {
	const caseID = "44444444-4444-4444-4444-444444444444"
	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{
				Message: "Case created successfully.",
				Case: domain.CreateCaseDetails{
					ID: caseID, InternalID: "WSO2-CS-2", Number: "CS0023002",
					CreatedBy: "jane.doe@example.com", State: "Open",
				},
			}, nil
		},
	}
	severity := domain.CaseSeverityCatastrophic
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error) {
			respState := domain.CaseStateOpen
			return domain.Case{ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy, State: &respState}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{
				ID: caseID, Number: "CS0023002", InternalID: "WSO2-CS-2", Subject: "s", Description: "d",
				CreatedOn:      time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
				WatchList:      []domain.WatchListUser{{Email: "watcher@example.com"}},
				Severity:       &severity,
			}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	slaEngine := &fakeSLAEngineService{}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, slaEngine, "")

	if _, err := svc.CreateCase(context.Background(), validCreateCaseRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.registerCalls) != 1 {
		t.Fatalf("expected exactly 1 RegisterCaseClocks call after Postgres success, got %d", len(slaEngine.registerCalls))
	}
	got := slaEngine.registerCalls[0]
	if got.caseID != caseID || got.severity == nil || *got.severity != domain.CaseSeverityCatastrophic || got.projectID != "proj-1" {
		t.Errorf("unexpected RegisterCaseClocks call: %+v", got)
	}
}

// TestCaseService_CreateCase_NeverPersistsAccountDefaultWatchers is the
// regression guard for the replacement design: the account's four named
// stakeholders must never be written to work_item_watcher (SetCaseWatchList
// is never called when req.WatchList is empty, even though
// AccountDefaultWatcherEmails resolves real stakeholders for this project) --
// see addRequestedWatchers' own doc comment for why. They still reach the
// case.created email itself, resolved fresh via AccountDefaultWatcherEmails
// and unioned into Recipients by publishCaseCreatedEvent, alongside the
// case's own (here empty) WatchList.
func TestCaseService_CreateCase_NeverPersistsAccountDefaultWatchers(t *testing.T) {
	const caseID = "44444444-4444-4444-4444-444444444444"
	const projectID = "proj-1"
	stakeholderEmails := []string{"csm@example.com", "tow@example.com"}

	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{
				Message: "Case created successfully.",
				Case: domain.CreateCaseDetails{
					ID: caseID, InternalID: "WSO2-CS-2", Number: "CS0023002",
					CreatedBy: "jane.doe@example.com", State: "Open",
				},
			}, nil
		},
	}
	setWatchListCalled := false
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error) {
			respState := domain.CaseStateOpen
			return domain.Case{ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy, ProjectID: projectID, State: &respState}, nil
		},
		accountDefaultWatcherEmails: func(_ context.Context, gotProjectID string) ([]string, error) {
			if gotProjectID != projectID {
				t.Errorf("AccountDefaultWatcherEmails projectID = %q, want %q", gotProjectID, projectID)
			}
			return stakeholderEmails, nil
		},
		setCaseWatchList: func(context.Context, string, []string, string) ([]domain.WatchListUser, time.Time, error) {
			setWatchListCalled = true
			return nil, time.Time{}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			severity := domain.CaseSeverityHigh
			return domain.CaseView{
				ID: caseID, Number: "CS0023002", InternalID: "WSO2-CS-2", Subject: "s", Description: "d",
				CreatedOn:      time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
				ProjectDetails: &domain.EntityRef{ID: projectID, Name: "Project One"},
				WatchList:      []domain.WatchListUser{{Email: "watcher@example.com"}},
				Severity:       &severity,
			}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	publisher := &mockEventPublisher{}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, publisher, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	if _, err := svc.CreateCase(context.Background(), validCreateCaseRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setWatchListCalled {
		t.Error("SetCaseWatchList was called -- account default watchers must never be persisted into work_item_watcher")
	}
	if len(publisher.calls) == 0 {
		t.Fatal("expected case.created to be published")
	}

	var payload events.CaseCreatedPayload
	if err := json.Unmarshal(publisher.calls[0].payload, &payload); err != nil {
		t.Fatalf("decode case.created payload: %v", err)
	}
	wantRecipients := append([]string{"watcher@example.com"}, stakeholderEmails...)
	for _, email := range wantRecipients {
		found := false
		for _, got := range payload.Recipients {
			if got == email {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Recipients = %v, want it to include %q", payload.Recipients, email)
		}
	}
}

// TestCaseService_CreateCase_DoesNotPublishWhenPostgresFails proves the
// other half: if ServiceNow already has the case but the Postgres insert
// fails (real drift, logged separately), no event fires -- a consumer must
// never see case.created for a case the Postgres-backed read API cannot
// return.
func TestCaseService_CreateCase_DoesNotPublishWhenPostgresFails(t *testing.T) {
	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{
				Message: "Case created successfully.",
				Case: domain.CreateCaseDetails{
					ID: "55555555-5555-5555-5555-555555555555", InternalID: "WSO2-CS-3", Number: "CS0023003",
					CreatedBy: "jane.doe@example.com", State: "Open",
				},
			}, nil
		},
	}
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(context.Context, domain.CreateCaseRequest, string, string, string, string, string) (domain.Case, error) {
			return domain.Case{}, errors.New("postgres insert failed")
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	publisher := &mockEventPublisher{}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, publisher, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	if _, err := svc.CreateCase(context.Background(), validCreateCaseRequest()); err == nil {
		t.Fatal("expected an error when the Postgres insert fails")
	}
	if len(publisher.calls) != 0 {
		t.Errorf("expected no publish call when the Postgres insert fails, got %d", len(publisher.calls))
	}
}

// TestCaseService_CreateCase_PlainPostgresPublishesCaseCreated is the
// regression guard for a real gap: with no ServiceNow mirror configured at
// all (s.snMirror == nil -- NewCaseService, not NewCaseServiceWithSNWriteback),
// CreateCase's plain path used to return straight after s.repo.CreateCase
// succeeded, with no call to publishCaseCreatedEvent at all -- case.created
// was only ever published from createCaseSNFirst, the SN-mirror-only path.
// So a case created with no mirror configured never published
// case.created, regardless of how Event Hub was configured.
func TestCaseService_CreateCase_PlainPostgresPublishesCaseCreated(t *testing.T) {
	const caseID = "66666666-6666-6666-6666-666666666666"
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
			respState := domain.CaseStateOpen
			return domain.Case{ID: caseID, Number: "CS0023004", InternalID: "WSO2-CS-4", CreatedBy: req.CreatedBy, State: &respState}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			severity := domain.CaseSeverityHigh
			return domain.CaseView{
				ID: caseID, Number: "CS0023004", InternalID: "WSO2-CS-4", Subject: "s", Description: "d",
				CreatedOn:      time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
				WatchList:      []domain.WatchListUser{{Email: "watcher@example.com"}},
				Severity:       &severity,
			}, nil
		},
	}
	userRepo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: "user-1", Email: "jane.doe@example.com"}, nil
		},
	}
	publisher := &mockEventPublisher{}
	svc := NewCaseService(repo, userRepo, publisher, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	if _, err := svc.CreateCase(ctx, validCreateCaseRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected exactly 1 publish call on the plain Postgres path, got %d", len(publisher.calls))
	}
	if publisher.calls[0].eventType != events.TypeCaseCreated || publisher.calls[0].entityID != caseID {
		t.Errorf("unexpected publish call: %+v", publisher.calls[0])
	}
}

// TestCaseService_CreateCase_AnnouncementTypeReachesServiceNowMirror is the
// regression guard for a real bug: CreateCase used to reject req.Type !=
// "case" unconditionally, before ever checking whether an SN mirror was even
// configured — so an "announcement" case (which snCaseTypeMap/
// snCaseService.CreateCase both explicitly support) was rejected outright on
// the SN-first path too, even though ServiceNow itself was about to handle
// the create just fine. The type restriction is only actually true for the
// pure-Postgres fallback below (no equivalent extension table for
// engagement/service_request/security_report_analysis/announcement yet); it
// must never block a request this same call is about to dispatch to
// ServiceNow instead.
func TestCaseService_CreateCase_AnnouncementTypeReachesServiceNowMirror(t *testing.T) {
	var gotType string
	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			gotType = req.Type
			return domain.CreateCaseResponse{Case: domain.CreateCaseDetails{ID: "case-1", Number: "CS001", CreatedBy: "user-1", State: "Open"}}, nil
		},
	}
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error) {
			return domain.Case{ID: id, Number: number, CreatedBy: createdBy}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	req := domain.CreateCaseRequest{
		CreatedBy:   "user-1",
		Type:        "announcement",
		ProjectID:   testDeploymentUUID,
		Subject:     "Scheduled maintenance",
		Description: "Details",
	}
	if _, err := svc.CreateCase(context.Background(), req); err != nil {
		t.Fatalf("unexpected error creating an announcement-type case: %v", err)
	}
	if gotType != "announcement" {
		t.Fatalf("expected the SN mirror to receive type=announcement, got %q", gotType)
	}
}

// TestCaseService_CreateCase_DoesNotRetryValidationError guards against a
// second SN attempt on a deterministic client error -- createCaseSNFirst
// makes exactly one attempt (see its own doc comment: the internal retry was
// removed, since retryable transport failures are handled at the gateway/
// caller level, not duplicated here).
func TestCaseService_CreateCase_DoesNotRetryValidationError(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "severity contains invalid value"}
		},
	}
	repo := &stubCaseRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	_, err := svc.CreateCase(context.Background(), validCreateCaseRequest())
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("expected exactly 1 SN attempt (validation errors are not retried), got %d", attempts)
	}
}

// TestCaseService_CreateCase_RejectsUnsupportedTypesOnPostgres is the control
// half of the type-guard widening: proves a made-up bogus type is still
// rejected outright, never reaching snMirror or the repository, even though
// "case"/"announcement"/"service_request"/"engagement"/
// "security_report_analysis" are now all accepted -- guards against the
// guard having been widened too far.
func TestCaseService_CreateCase_RejectsUnsupportedTypesOnPostgres(t *testing.T) {
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			t.Fatal("snMirror.CreateCase should not be reached for an unsupported type")
			return domain.CreateCaseResponse{}, nil
		},
	}
	repo := &stubCaseRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	for _, typ := range []string{"bogus_type"} {
		t.Run(typ, func(t *testing.T) {
			req := domain.CreateCaseRequest{
				Type:        typ,
				ProjectID:   testDeploymentUUID,
				Subject:     "x",
				Description: "y",
			}
			_, err := svc.CreateCase(context.Background(), req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError for type %q, got %T: %v", typ, err, err)
			}
		})
	}
}

// TestCaseService_CreateCase_SNOnlyTypesNowSupportedWithoutMirror used to be
// TestCaseService_CreateCase_RejectsSNOnlyTypesWithoutMirror, the regression
// guard for a CodeRabbit finding on PR #1930: announcement/service_request/
// engagement/security_report_analysis used to exist only on the SN-first
// path, so reaching caseRepo's direct-Postgres insert with one of these four
// types on a pure-Postgres data source (s.snMirror == nil, NewCaseService
// rather than NewCaseServiceWithSNWriteback) would surface as an opaque 500
// (announcement's empty deployment id cast as ::uuid) or 503 (no
// work_item.number generator for the other three) instead of a clean
// validation error. CaseRepository.CreateCase now has a dedicated query per
// type (case_repo.go's createCaseTx dispatch, unblocked by migration 0140's
// next_portal_work_item_number()/next_portal_wso2_id()), so these four are no
// longer rejected here at all -- this test now asserts the opposite: they
// reach repo.CreateCase and succeed, exactly like "case" always has.
func TestCaseService_CreateCase_SNOnlyTypesNowSupportedWithoutMirror(t *testing.T) {
	var gotTypes []string
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
			gotTypes = append(gotTypes, req.Type)
			return domain.Case{ID: "case-1", Number: "CS-PORTAL-000001", CreatedBy: req.CreatedBy}, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

	for _, req := range []domain.CreateCaseRequest{
		{CreatedBy: "user-1", Type: "announcement", ProjectID: testDeploymentUUID, Subject: "x", Description: "y"},
		withCreatedBy(validServiceRequestCreateCaseRequest(), "user-1"),
	} {
		t.Run(req.Type, func(t *testing.T) {
			if _, err := svc.CreateCase(context.Background(), req); err != nil {
				t.Fatalf("expected type %q to succeed without an SN mirror, got %T: %v", req.Type, err, err)
			}
		})
	}
	if len(gotTypes) != 2 {
		t.Fatalf("expected repo.CreateCase to be reached for both types, got %v", gotTypes)
	}
}

// withCreatedBy returns a copy of req with CreatedBy set, so a test can skip
// CreateCase's x-user-id-token resolution path entirely.
func withCreatedBy(req domain.CreateCaseRequest, createdBy string) domain.CreateCaseRequest {
	req.CreatedBy = createdBy
	return req
}

// validServiceRequestCreateCaseRequest returns a minimally valid,
// type="service_request" CreateCase request -- deployment/deployedProduct
// ARE required for this type (unlike announcement), per
// validateCreateCaseRequest's own conditional.
func validServiceRequestCreateCaseRequest() domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type:              "service_request",
		ProjectID:         testDeploymentUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeploymentUUID,
		CatalogID:         testDeploymentUUID,
		CatalogItemID:     testDeploymentUUID,
		Variables:         []domain.Variable{{ID: testDeploymentUUID, Value: "v"}},
	}
}

// validEngagementCreateCaseRequest returns a minimally valid,
// type="engagement" CreateCase request.
func validEngagementCreateCaseRequest() domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type:                  "engagement",
		ProjectID:             testDeploymentUUID,
		DeploymentID:          testDeploymentUUID,
		DeployedProductID:     testDeploymentUUID,
		Subject:               "Migration engagement",
		Description:           "Engagement details go here",
		EngagementType:        domain.EngagementTypeMigration,
		EngagementPaymentType: domain.EngagementPaymentTypePaid,
	}
}

// validSecurityReportAnalysisCreateCaseRequest returns a minimally valid,
// type="security_report_analysis" CreateCase request.
func validSecurityReportAnalysisCreateCaseRequest() domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type:              "security_report_analysis",
		ProjectID:         testDeploymentUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeploymentUUID,
		Subject:           "Suspicious activity report",
		Description:       "Report details go here",
	}
}

// caseFamilyCreateCaseTestCase table-drives the three new types' shared
// CREATE regression guards (success/state-mapping, unknown-SN-state,
// SN-failure-orphan-guard) below -- each type's own workItemType/stateEnum
// values differ, but the three-way behavior being proven is identical.
type caseFamilyCreateCaseTestCase struct {
	name    string
	reqFunc func() domain.CreateCaseRequest
}

var caseFamilyCreateCaseTestCases = []caseFamilyCreateCaseTestCase{
	{name: "service_request", reqFunc: validServiceRequestCreateCaseRequest},
	{name: "engagement", reqFunc: validEngagementCreateCaseRequest},
	{name: "security_report_analysis", reqFunc: validSecurityReportAnalysisCreateCaseRequest},
}

// TestCaseService_CreateCase_CaseFamily_SNSuccessStoresTypeAndMappedState
// covers the SN-first path end to end for each of the three new types: the
// Postgres insert must receive req.Type unchanged and the state ServiceNow's
// create response actually returned, mapped through that type's own
// sn<Type>StateToEnum -- not just "no error".
func TestCaseService_CreateCase_CaseFamily_SNSuccessStoresTypeAndMappedState(t *testing.T) {
	for _, tc := range caseFamilyCreateCaseTestCases {
		t.Run(tc.name, func(t *testing.T) {
			const (
				snID         = "55555555-5555-5555-5555-555555555555"
				snNumber     = "WI0001001"
				snInternalID = "WSO2-WI-1"
				snCreatedBy  = "jane.doe@example.com"
			)
			wantType := tc.name
			mirror := &stubMirrorCaseService{
				createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
					if req.Type != wantType {
						t.Fatalf("snMirror.CreateCase got type %q, want %q", req.Type, wantType)
					}
					return domain.CreateCaseResponse{
						Message: "Case created successfully.",
						Case: domain.CreateCaseDetails{
							ID: snID, InternalID: snInternalID, Number: snNumber,
							CreatedBy: snCreatedBy, CreatedOn: time.Now(), State: "Work In Progress",
						},
					}, nil
				},
			}

			var mu sync.Mutex
			var gotType, gotState string
			var gotEngagementType domain.EngagementType
			var gotEngagementPaymentType domain.EngagementPaymentType
			repo := &stubCaseRepo{
				createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error) {
					mu.Lock()
					gotType, gotState = req.Type, state
					gotEngagementType, gotEngagementPaymentType = req.EngagementType, req.EngagementPaymentType
					mu.Unlock()
					st := domain.CaseState("work_in_progress")
					return domain.Case{ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy, State: &st}, nil
				},
			}
			dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
			svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

			resp, err := svc.CreateCase(context.Background(), tc.reqFunc())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			mu.Lock()
			defer mu.Unlock()
			if gotType != wantType {
				t.Errorf("CreateCaseFromServiceNow got req.Type %q, want %q", gotType, wantType)
			}
			if gotState != "WORK_IN_PROGRESS" {
				t.Errorf("CreateCaseFromServiceNow got state %q, want \"WORK_IN_PROGRESS\" (mapped from ServiceNow's \"Work In Progress\" label)", gotState)
			}
			if resp.Case.State != "work_in_progress" {
				t.Errorf("CreateCase response state = %q, want \"work_in_progress\"", resp.Case.State)
			}
			if wantType == "engagement" {
				if gotEngagementType != domain.EngagementTypeMigration {
					t.Errorf("req.EngagementType did not reach CreateCaseFromServiceNow: got %q, want %q", gotEngagementType, domain.EngagementTypeMigration)
				}
				if gotEngagementPaymentType != domain.EngagementPaymentTypePaid {
					t.Errorf("req.EngagementPaymentType did not reach CreateCaseFromServiceNow: got %q, want %q", gotEngagementPaymentType, domain.EngagementPaymentTypePaid)
				}
			}
		})
	}
}

// TestCaseService_CreateCase_CaseFamily_UnknownSNStateFailsClosed proves an
// unrecognized ServiceNow state label on a create response is a hard error
// for each of the three new types -- not silently defaulted to OPEN -- and
// that the Postgres repository is never reached in that case (ServiceNow
// already has the record at that point, which is real drift needing operator
// attention, but the caller must still see an error rather than a fabricated
// success).
func TestCaseService_CreateCase_CaseFamily_UnknownSNStateFailsClosed(t *testing.T) {
	for _, tc := range caseFamilyCreateCaseTestCases {
		t.Run(tc.name, func(t *testing.T) {
			mirror := &stubMirrorCaseService{
				createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
					return domain.CreateCaseResponse{
						Case: domain.CreateCaseDetails{ID: testDeploymentUUID, Number: "WI0002", InternalID: "WSO2-WI-2", CreatedBy: "jane.doe@example.com", State: "Pending Review"},
					}, nil
				},
			}
			// No createCaseFromServiceNow override -- stubCaseRepo panics if
			// it's ever called, proving Postgres is never reached for an
			// unmappable state.
			repo := &stubCaseRepo{}
			dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
			svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

			_, err := svc.CreateCase(context.Background(), tc.reqFunc())
			if err == nil {
				t.Fatal("expected an error for an unrecognized ServiceNow state")
			}
		})
	}
}

// TestCaseService_CreateCase_CaseFamily_SNFailureLeavesPostgresUntouched
// mirrors the case-flavored orphan guard
// (TestCaseService_CreateCase_SNFailureLeavesPostgresUntouched) for each of
// the three new types: if ServiceNow never accepts the record, Postgres must
// never be touched.
func TestCaseService_CreateCase_CaseFamily_SNFailureLeavesPostgresUntouched(t *testing.T) {
	for _, tc := range caseFamilyCreateCaseTestCases {
		t.Run(tc.name, func(t *testing.T) {
			mirror := &stubMirrorCaseService{
				createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
					return domain.CreateCaseResponse{}, errors.New("sn downstream unreachable")
				},
			}
			repo := &stubCaseRepo{}
			dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
			svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

			_, err := svc.CreateCase(context.Background(), tc.reqFunc())
			if err == nil {
				t.Fatal("expected an error when ServiceNow never accepts the record")
			}
		})
	}
}

// TestCaseService_CreateCase_CaseFamily_DoesNotRetryValidationError guards
// against wasted latency on a deterministic client error for each of the
// three new types, same as case's own
// TestCaseService_CreateCase_DoesNotRetryValidationError.
func TestCaseService_CreateCase_CaseFamily_DoesNotRetryValidationError(t *testing.T) {
	for _, tc := range caseFamilyCreateCaseTestCases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			attempts := 0
			mirror := &stubMirrorCaseService{
				createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
					mu.Lock()
					attempts++
					mu.Unlock()
					return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "some field contains invalid value"}
				},
			}
			repo := &stubCaseRepo{}
			dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
			svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

			_, err := svc.CreateCase(context.Background(), tc.reqFunc())
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}

			mu.Lock()
			defer mu.Unlock()
			if attempts != 1 {
				t.Errorf("expected exactly 1 SN attempt (validation errors are not retried), got %d", attempts)
			}
		})
	}
}

// TestCaseService_CreateCaseComment_MirrorsToServiceNow is the comment
// mirror's core regression guard: a successful Postgres comment create must
// dispatch to the mirror's CreateBareCaseComment (never the full
// CreateCaseComment, which this stub doesn't even implement) with exactly
// the case id, type, and content that were just written to Postgres, and
// must not record a sn_writeback_failures row when the mirror succeeds.
func TestCaseService_CreateCaseComment_MirrorsToServiceNow(t *testing.T) {
	var mu sync.Mutex
	var gotCaseID string
	var gotType domain.CommentType
	var gotContent string
	called := make(chan struct{})
	mirror := &stubMirrorCaseService{
		createBareCaseComment: func(_ context.Context, caseID string, commentType domain.CommentType, content string) (domain.CaseCommentDetail, error) {
			mu.Lock()
			gotCaseID, gotType, gotContent = caseID, commentType, content
			mu.Unlock()
			close(called)
			return domain.CaseCommentDetail{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)

	createdOn := time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC)
	repo := &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "comment-1", CaseID: req.CaseID, Type: req.Type, Content: req.Content, CreatedOn: createdOn}, nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: "user-1", Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCaseCommentRequest{CaseID: testDeploymentUUID, Type: domain.CommentTypeComment, Content: "Working on it"}
	if _, err := svc.CreateCaseComment(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.CreateBareCaseComment was never called")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotCaseID != testDeploymentUUID {
		t.Errorf("mirror got caseID %q, want %q", gotCaseID, testDeploymentUUID)
	}
	if gotType != domain.CommentTypeComment {
		t.Errorf("mirror got type %q, want %q", gotType, domain.CommentTypeComment)
	}
	if gotContent != "Working on it" {
		t.Errorf("mirror got content %q, want %q", gotContent, "Working on it")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestCaseService_CreateCaseComment_CompletesResponseSLAForSupportEngineer is
// the regression guard for a real, live-observed bug: a support engineer's
// reply on a dual-write case never stopped the response SLA clock at all,
// because SLAEngineService.CompleteResponseClock was only ever wired into
// snCaseService.applyResponseSLAOnComment (the plain ServiceNow data
// source's own hook) -- caseService.createCaseCommentAs (the active,
// Postgres-primary path in dual-write mode) had no equivalent, so the
// response clock kept running to breach regardless of how quickly the case
// was actually answered.
func TestCaseService_CreateCaseComment_CompletesResponseSLAForSupportEngineer(t *testing.T) {
	repo := &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "comment-1", CaseID: req.CaseID, Type: req.Type, Content: req.Content}, nil
		},
	}
	userRepo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: "user-1", Email: "engineer@example.com"}, nil
		},
		getUserRoles: func(context.Context, string) ([]string, error) {
			return []string{"sn_customerservice_agent"}, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	mirror := &stubMirrorCaseService{
		createBareCaseComment: func(context.Context, string, domain.CommentType, string) (domain.CaseCommentDetail, error) {
			return domain.CaseCommentDetail{}, nil
		},
	}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}), mirror, slaEngine, "sn_customerservice_agent")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com"))
	req := domain.CreateCaseCommentRequest{CaseID: testDeploymentUUID, Type: domain.CommentTypeComment, Content: "Looking into it"}
	if _, err := svc.CreateCaseComment(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.completeCalls) != 1 || slaEngine.completeCalls[0] != testDeploymentUUID {
		t.Fatalf("expected CompleteResponseClock(%q), got %v", testDeploymentUUID, slaEngine.completeCalls)
	}
}

// TestCaseService_CreateCaseComment_DoesNotCompleteResponseSLAForNonEngineer
// proves the hook above is genuinely role-gated, not unconditional -- a
// reply from someone who does not hold csEngineerRole must not
// complete the clock, the same "can't confirm, skip" posture
// snCaseService.applyResponseSLAOnComment's own doc comment describes.
func TestCaseService_CreateCaseComment_DoesNotCompleteResponseSLAForNonEngineer(t *testing.T) {
	repo := &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "comment-1", CaseID: req.CaseID, Type: req.Type, Content: req.Content}, nil
		},
	}
	userRepo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: "user-2", Email: "customer@example.com"}, nil
		},
		getUserRoles: func(context.Context, string) ([]string, error) {
			return []string{"customer"}, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	mirror := &stubMirrorCaseService{
		createBareCaseComment: func(context.Context, string, domain.CommentType, string) (domain.CaseCommentDetail, error) {
			return domain.CaseCommentDetail{}, nil
		},
	}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}), mirror, slaEngine, "sn_customerservice_agent")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "customer@example.com"))
	req := domain.CreateCaseCommentRequest{CaseID: testDeploymentUUID, Type: domain.CommentTypeComment, Content: "Any update?"}
	if _, err := svc.CreateCaseComment(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.completeCalls) != 0 {
		t.Errorf("expected no CompleteResponseClock call for a non-engineer author, got %v", slaEngine.completeCalls)
	}
}

// TestCaseService_CreateCaseComment_PublishesCommentAdded is the regression
// guard for a real bug: caseService.CreateCaseComment never published
// case.comment_added at all, for any DATA_SOURCE that writes comments
// through this repository -- the wiring snCaseService.CreateCaseComment
// already had (publishCommentAdded) was simply missing here. Also proves
// the author's display name is built from the already-resolved actor
// (FirstName/LastName), with no SearchCaseComments re-fetch needed the way
// snCaseService's own version requires.
func TestCaseService_CreateCaseComment_PublishesCommentAdded(t *testing.T) {
	repo := &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "comment-1", CaseID: req.CaseID, Type: req.Type, Content: req.Content}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{
				ID: testDeploymentUUID, Number: "CS0001", InternalID: "WSO2-CS-1", Subject: "s",
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
				WatchList:      []domain.WatchListUser{{Email: "watcher@example.com"}},
			}, nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: "user-1", Email: "jane.doe@example.com", FirstName: "Jane", LastName: "Doe"}, nil
	}}
	publisher := &mockEventPublisher{}
	svc := NewCaseService(repo, userRepo, publisher, alwaysUnrestrictedAccess{}, nil)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCaseCommentRequest{CaseID: testDeploymentUUID, Type: domain.CommentTypeComment, Content: "Working on it"}
	if _, err := svc.CreateCaseComment(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected exactly 1 publish call, got %d", len(publisher.calls))
	}
	if publisher.calls[0].eventType != events.TypeCommentAdded || publisher.calls[0].entityID != testDeploymentUUID {
		t.Fatalf("unexpected publish call: %+v", publisher.calls[0])
	}
	var payload events.CommentAddedPayload
	if err := json.Unmarshal(publisher.calls[0].payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.Name != "Jane Doe" {
		t.Errorf("Name = %q, want %q", payload.Name, "Jane Doe")
	}
	if payload.CaseComment != "Working on it" || payload.CommentID != "comment-1" {
		t.Errorf("payload = %+v, want content %q and commentId %q", payload, "Working on it", "comment-1")
	}
}

// TestCaseService_CreateCaseComment_RecordsSNWritebackFailureOnMirrorError
// covers the failure path: Postgres already committed the comment by the
// time Dispatch runs, so a failed mirror write must not surface as a
// CreateCaseComment error -- it's recorded to sn_writeback_failures instead.
func TestCaseService_CreateCaseComment_RecordsSNWritebackFailureOnMirrorError(t *testing.T) {
	mirror := &stubMirrorCaseService{
		createBareCaseComment: func(context.Context, string, domain.CommentType, string) (domain.CaseCommentDetail, error) {
			return domain.CaseCommentDetail{}, errors.New("sn downstream unreachable")
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)

	repo := &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "comment-1", CaseID: req.CaseID, Type: req.Type, Content: req.Content}, nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: "user-1", Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCaseCommentRequest{CaseID: testDeploymentUUID, Type: domain.CommentTypeComment, Content: "Working on it"}
	resp, err := svc.CreateCaseComment(ctx, req)
	if err != nil {
		t.Fatalf("CreateCaseComment must still succeed on a failed best-effort mirror write, got: %v", err)
	}
	if resp.Comment.ID != "comment-1" {
		t.Errorf("CreateCaseComment response ID = %q, want %q", resp.Comment.ID, "comment-1")
	}

	waitFor(t, func() bool { return failures.count() == 1 })
	failReq := failures.calls[0]
	if failReq.EntityType != "case_comment" || failReq.EntityID != testDeploymentUUID || failReq.Operation != "create" {
		t.Errorf("unexpected failure record: %+v", failReq)
	}
}

// TestCaseService_CreateCaseComment_SkipsMirrorForActivityType confirms
// "activity" comments -- which CreateBareCaseComment always rejects, since
// ServiceNow has no concept of that type -- never even reach Dispatch. A
// permanent, 100%-guaranteed incompatibility must not be recorded to
// sn_writeback_failures as if it were a transient, backfillable failure.
func TestCaseService_CreateCaseComment_SkipsMirrorForActivityType(t *testing.T) {
	// Dispatch runs this callback on the dispatcher's own worker goroutine,
	// not the test goroutine -- t.Fatal/FailNow is only safe to call from
	// the goroutine running the test itself, so a wrongly-invoked call is
	// recorded here and asserted on the main goroutine below instead.
	var calledWrongly atomic.Bool
	mirror := &stubMirrorCaseService{
		createBareCaseComment: func(context.Context, string, domain.CommentType, string) (domain.CaseCommentDetail, error) {
			calledWrongly.Store(true)
			return domain.CaseCommentDetail{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)

	repo := &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "comment-1", CaseID: req.CaseID, Type: req.Type, Content: req.Content}, nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: "user-1", Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCaseCommentRequest{CaseID: testDeploymentUUID, Type: domain.CommentTypeActivity, Content: "system note"}
	if _, err := svc.CreateCaseComment(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Give any wrongly-dispatched goroutine a moment to run before asserting
	// zero failures were recorded -- a skip means no dispatch at all, not a
	// dispatch that happens to succeed or fail silently.
	time.Sleep(100 * time.Millisecond)
	if calledWrongly.Load() {
		t.Error("CreateBareCaseComment must never be called for an activity-type comment")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a skipped activity-type mirror, got %d", got)
	}
}

// TestCaseService_CreateCaseComment_DoesNotMirrorWithoutSNWriteback confirms
// the mirror only ever applies under
// DATA_SOURCE=postgres-servicenow-dual-write (snWriteback/snMirror set) --
// NewCaseService's plain construction (every other DataSource) must behave
// exactly as it did before this feature existed.
func TestCaseService_CreateCaseComment_DoesNotMirrorWithoutSNWriteback(t *testing.T) {
	repo := &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "comment-1", CaseID: req.CaseID, Type: req.Type, Content: req.Content}, nil
		},
	}
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: "user-1", Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseService(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCaseCommentRequest{CaseID: testDeploymentUUID, Type: domain.CommentTypeComment, Content: "Working on it"}
	if _, err := svc.CreateCaseComment(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No mirror configured at all -- nothing to assert beyond "this didn't
	// panic trying to dispatch through a nil snWriteback/snMirror", which a
	// clean return already proves.
}

// TestCaseService_CreateCaseCommentAs_UsesActorEmailDirectly covers the M2M
// path this method exists for: no x-user-id-token is required, and --
// mirroring addCaseTagAs -- the caller-supplied actorEmail is used directly
// for created_by and the published event's author name, with no
// userRepo.GetUserByEmail lookup at all (stubUserRepo{} panics if it were
// called, since no getUserByEmail func is configured here). Also proves the
// created comment's CreatedBy in the response is the actorEmail, not a
// resolved user row's email.
func TestCaseService_CreateCaseCommentAs_UsesActorEmailDirectly(t *testing.T) {
	var gotCreatedBy string
	repo := &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			gotCreatedBy = req.CreatedBy
			return domain.CaseComment{ID: "comment-1", CaseID: req.CaseID, Type: req.Type, Content: req.Content}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{
				ID: testDeploymentUUID, Number: "CS0001", InternalID: "WSO2-CS-1", Subject: "s",
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
				WatchList:      []domain.WatchListUser{{Email: "watcher@example.com"}},
			}, nil
		},
	}
	// Deliberately no getUserByEmail configured -- CreateCaseCommentAs must
	// never call it (unlike CreateCaseComment).
	userRepo := stubUserRepo{}
	publisher := &mockEventPublisher{}
	svc := NewCaseService(repo, userRepo, publisher, alwaysUnrestrictedAccess{}, nil)

	const actorEmail = "umt-service@example.com"
	req := domain.CreateCaseCommentRequest{CaseID: testDeploymentUUID, Type: domain.CommentTypeComment, Content: "Working on it"}
	resp, err := svc.CreateCaseCommentAs(context.Background(), req, actorEmail)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCreatedBy != actorEmail {
		t.Errorf("repo got created_by %q, want %q", gotCreatedBy, actorEmail)
	}
	if resp.Comment.CreatedBy != actorEmail {
		t.Errorf("response CreatedBy = %q, want %q", resp.Comment.CreatedBy, actorEmail)
	}
	if len(publisher.calls) != 1 {
		t.Fatalf("expected exactly 1 publish call, got %d", len(publisher.calls))
	}
	var payload events.CommentAddedPayload
	if err := json.Unmarshal(publisher.calls[0].payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.Name != actorEmail {
		t.Errorf("published author Name = %q, want %q (actorEmail, since no user row was looked up)", payload.Name, actorEmail)
	}
}

// TestCaseService_CreateCase_Announcement_SNSuccessStoresAnnouncementType
// covers the announcement branch of the SN-first path end to end: the
// Postgres insert must receive req.Type == "announcement" and store the
// real ANNOUNCEMENT work_item type plus the state ServiceNow's create
// response actually returned (mapped through snAnnouncementStateToEnum),
// not just "no error" -- CreateCaseFromServiceNow's stub here asserts on
// req.Type directly, which is what the repository branches on.
func TestCaseService_CreateCase_Announcement_SNSuccessStoresAnnouncementType(t *testing.T) {
	const (
		snID         = "44444444-4444-4444-4444-444444444444"
		snNumber     = "AN0001001"
		snInternalID = "WSO2-AN-1"
		snCreatedBy  = "jane.doe@example.com"
	)
	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			if req.Type != "announcement" {
				t.Fatalf("snMirror.CreateCase got type %q, want \"announcement\"", req.Type)
			}
			return domain.CreateCaseResponse{
				Message: "Case created successfully.",
				Case: domain.CreateCaseDetails{
					ID: snID, InternalID: snInternalID, Number: snNumber,
					CreatedBy: snCreatedBy, CreatedOn: time.Now(), State: "Open",
				},
			}, nil
		},
	}

	var mu sync.Mutex
	var gotType, gotAnnouncementState string
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, announcementState string) (domain.Case, error) {
			mu.Lock()
			gotType, gotAnnouncementState = req.Type, announcementState
			mu.Unlock()
			state := domain.CaseState(strings.ToLower(announcementState))
			return domain.Case{ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy, State: &state}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	resp, err := svc.CreateCase(context.Background(), validAnnouncementCreateCaseRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotType != "announcement" {
		t.Errorf("CreateCaseFromServiceNow got req.Type %q, want \"announcement\"", gotType)
	}
	if gotAnnouncementState != "OPEN" {
		t.Errorf("CreateCaseFromServiceNow got announcementState %q, want \"OPEN\" (mapped from ServiceNow's \"Open\" label)", gotAnnouncementState)
	}
	if resp.Case.State != "open" {
		t.Errorf("CreateCase response state = %q, want \"open\"", resp.Case.State)
	}
}

// TestCaseService_CreateCase_Announcement_UnknownSNStateFailsClosed proves an
// unrecognized ServiceNow state label on an announcement create response is
// a hard error -- not silently defaulted to OPEN -- and that the Postgres
// repository is never reached in that case (ServiceNow already has the
// announcement at that point, which is real drift needing operator
// attention, but the caller must still see an error rather than a
// fabricated success).
func TestCaseService_CreateCase_Announcement_UnknownSNStateFailsClosed(t *testing.T) {
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{
				Case: domain.CreateCaseDetails{ID: testDeploymentUUID, Number: "AN0002", InternalID: "WSO2-AN-2", CreatedBy: "jane.doe@example.com", State: "Pending Review"},
			}, nil
		},
	}
	// No createCaseFromServiceNow override -- stubCaseRepo panics if it's
	// ever called, proving Postgres is never reached for an unmappable state.
	repo := &stubCaseRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	_, err := svc.CreateCase(context.Background(), validAnnouncementCreateCaseRequest())
	if err == nil {
		t.Fatal("expected an error for an unrecognized ServiceNow announcement state")
	}
}

// TestCaseService_CreateCase_Announcement_SNFailureLeavesPostgresUntouched
// mirrors the case-flavored orphan guard (TestCaseService_CreateCase_SNFailureLeavesPostgresUntouched)
// for announcement: if ServiceNow never accepts the announcement, Postgres
// must never be touched, same as case.
func TestCaseService_CreateCase_Announcement_SNFailureLeavesPostgresUntouched(t *testing.T) {
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubCaseRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	_, err := svc.CreateCase(context.Background(), validAnnouncementCreateCaseRequest())
	if err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the announcement")
	}
}

// TestCaseService_AddCaseTag_MirrorsToServiceNow covers the writeback wiring:
// on a successful Postgres tag attach, the mirror's AddCaseTagAs is
// dispatched asynchronously (by label, not the Postgres tag id -- see
// addCaseTagAs's own doc comment) and does not block or affect the response.
func TestCaseService_AddCaseTag_MirrorsToServiceNow(t *testing.T) {
	called := make(chan struct {
		caseID, label, actorEmail string
	}, 1)
	mirror := &stubMirrorCaseService{
		addCaseTagAsFn: func(_ context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
			called <- struct{ caseID, label, actorEmail string }{caseID, label, actorEmail}
			return domain.Tag{}, nil
		},
	}
	repo := &stubCaseRepo{
		addCaseTag: func(_ context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
			return domain.Tag{ID: "t-1", Label: label}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.AddCaseTag(ctx, testDeploymentUUID, "patch-me"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.caseID != testDeploymentUUID || got.label != "patch-me" {
			t.Errorf("mirror got %+v, want caseId=%q label=%q", got, testDeploymentUUID, "patch-me")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.AddCaseTagAs was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestCaseService_AddCaseTag_MirrorFailureRecordsWritebackFailure covers the
// failure half: Postgres already succeeded, so the call must still report
// success, but the mirror error lands in sn_writeback_failures for manual
// backfill.
func TestCaseService_AddCaseTag_MirrorFailureRecordsWritebackFailure(t *testing.T) {
	mirror := &stubMirrorCaseService{
		addCaseTagAsFn: func(context.Context, string, string, string) (domain.Tag, error) {
			return domain.Tag{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubCaseRepo{
		addCaseTag: func(_ context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
			return domain.Tag{ID: "t-1", Label: label}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.AddCaseTag(ctx, testDeploymentUUID, "patch-me"); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

// TestCaseService_AddCaseTag_MirrorSuccessPersistsSNSysID covers the
// id-mapping half of the AddCaseTag mirror: on success, the ServiceNow
// label_entry sys_id the mirror returns (converted to its raw form) is
// persisted onto the (caseID, tagID) work_item_tag attachment via
// SetCaseTagSNSysID.
func TestCaseService_AddCaseTag_MirrorSuccessPersistsSNSysID(t *testing.T) {
	mirror := &stubMirrorCaseService{
		addCaseTagAsFn: func(context.Context, string, string, string) (domain.Tag, error) {
			return domain.Tag{ID: testSNSysID2}, nil // the ServiceNow-side id, sysidToUUID-converted
		},
	}
	type setCall struct{ caseID, tagID, snSysID string }
	setCalls := make(chan setCall, 1)
	repo := &stubCaseRepo{
		addCaseTag: func(_ context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
			return domain.Tag{ID: "t-1", Label: label}, nil // the Postgres-side tag id
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{}, nil
		},
		setCaseTagSNSysID: func(_ context.Context, caseID, tagID, snSysID string) error {
			setCalls <- setCall{caseID: caseID, tagID: tagID, snSysID: snSysID}
			return nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.AddCaseTag(ctx, testDeploymentUUID, "patch-me"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-setCalls:
		if got.caseID != testDeploymentUUID {
			t.Errorf("SetCaseTagSNSysID caseID = %q, want %q", got.caseID, testDeploymentUUID)
		}
		if got.tagID != "t-1" {
			t.Errorf("SetCaseTagSNSysID tagID = %q, want %q (the Postgres tag id)", got.tagID, "t-1")
		}
		wantSysID := uuidToSysid(testSNSysID2)
		if got.snSysID != wantSysID {
			t.Errorf("SetCaseTagSNSysID snSysID = %q, want %q", got.snSysID, wantSysID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SetCaseTagSNSysID was never called")
	}
}

// TestCaseService_AddCaseTag_CleansUpSNTagWhenAttachmentRemovedConcurrently
// covers the narrow race between AddCaseTag's mirror and a concurrent
// RemoveCaseTag: if the (caseID, tagID) attachment is gone by the time
// SetCaseTagSNSysID runs (RemoveCaseTag deleted it, found no sn_sys_id
// stored yet, and so skipped its own REMOVE mirror), the ServiceNow tag this
// callback just created would otherwise be permanently orphaned. The
// callback must react to SetCaseTagSNSysID's *apierror.NotFoundError by
// removing that just-created ServiceNow tag itself.
func TestCaseService_AddCaseTag_CleansUpSNTagWhenAttachmentRemovedConcurrently(t *testing.T) {
	mirror := &stubMirrorCaseService{
		addCaseTagAsFn: func(context.Context, string, string, string) (domain.Tag, error) {
			return domain.Tag{ID: testSNSysID2}, nil // the ServiceNow-side id, sysidToUUID-converted
		},
	}
	removeCalled := make(chan struct{ caseID, tagID string }, 1)
	mirror.removeCaseTagFn = func(_ context.Context, caseID, tagID string) error {
		removeCalled <- struct{ caseID, tagID string }{caseID, tagID}
		return nil
	}
	repo := &stubCaseRepo{
		addCaseTag: func(_ context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
			return domain.Tag{ID: "t-1", Label: label}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{}, nil
		},
		setCaseTagSNSysID: func(context.Context, string, string, string) error {
			// Simulates RemoveCaseTag having already deleted the
			// work_item_tag row before this persist runs.
			return &apierror.NotFoundError{Msg: "tag not found on this case"}
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.AddCaseTag(ctx, testDeploymentUUID, "patch-me"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-removeCalled:
		if got.caseID != testDeploymentUUID {
			t.Errorf("cleanup RemoveCaseTag caseID = %q, want %q", got.caseID, testDeploymentUUID)
		}
		if got.tagID != testSNSysID2 {
			t.Errorf("cleanup RemoveCaseTag tagID = %q, want %q (the just-created ServiceNow tag id)", got.tagID, testSNSysID2)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.RemoveCaseTag (cleanup) was never called")
	}
	// A handled NotFoundError is not itself a mirror failure to record --
	// only if the cleanup RemoveCaseTag call above also failed would this
	// count.
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successfully cleaned-up race, got %d", got)
	}
}

// TestCaseService_RemoveCaseTag_MirrorsWithStoredSNSysID covers the REMOVE
// mirror's happy path: when a ServiceNow label_entry sys_id is already
// stored for this (caseID, tagID) attachment, the mirror fires against it
// (converted back to UUID form for the mirror's own uuidToSysid).
func TestCaseService_RemoveCaseTag_MirrorsWithStoredSNSysID(t *testing.T) {
	storedSysID := uuidToSysid(testSNSysID2)
	repo := &stubCaseRepo{
		removeCaseTag: func(context.Context, string, string, string) error { return nil },
		getCaseTagSNSysID: func(context.Context, string, string) (*string, error) {
			return &storedSysID, nil
		},
	}
	type removeCall struct{ caseID, tagID string }
	called := make(chan removeCall, 1)
	mirror := &stubMirrorCaseService{
		removeCaseTagFn: func(_ context.Context, caseID, tagID string) error {
			called <- removeCall{caseID: caseID, tagID: tagID}
			return nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if err := svc.RemoveCaseTag(ctx, testDeploymentUUID, testUUID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.caseID != testDeploymentUUID {
			t.Errorf("mirror RemoveCaseTag caseID = %q, want %q", got.caseID, testDeploymentUUID)
		}
		if got.tagID != testSNSysID2 {
			t.Errorf("mirror RemoveCaseTag tagID = %q, want %q (sysidToUUID of the stored sys_id)", got.tagID, testSNSysID2)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.RemoveCaseTag was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestCaseService_RemoveCaseTag_SkipsMirrorWhenNoSNSysIDStored covers the
// pre-migration-attachment case: a NULL sn_sys_id (the AddCaseTag mirror
// never ran, hasn't finished, or failed) must make the REMOVE mirror skip
// silently -- no mirror call, no sn_writeback_failures record, no error to
// the caller.
func TestCaseService_RemoveCaseTag_SkipsMirrorWhenNoSNSysIDStored(t *testing.T) {
	repo := &stubCaseRepo{
		removeCaseTag: func(context.Context, string, string, string) error { return nil },
		getCaseTagSNSysID: func(context.Context, string, string) (*string, error) {
			return nil, nil // no mapping stored yet
		},
	}
	mirrorCalled := make(chan struct{}, 1)
	mirror := &stubMirrorCaseService{
		removeCaseTagFn: func(context.Context, string, string) error {
			mirrorCalled <- struct{}{}
			return nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if err := svc.RemoveCaseTag(ctx, testDeploymentUUID, testUUID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-mirrorCalled:
		t.Fatal("mirror.RemoveCaseTag was called despite no ServiceNow mapping being stored")
	case <-time.After(300 * time.Millisecond):
		// expected: no mirror call
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a deliberate skip, got %d", got)
	}
}

// TestCaseService_RemoveCaseTag_RecordsSNWritebackFailureOnLookupError covers
// the real-error half of the same lookup: unlike a genuinely successful
// lookup that finds no mapping (nil, nil -- see
// TestCaseService_RemoveCaseTag_SkipsMirrorWhenNoSNSysIDStored), a lookup
// that fails outright must not be silently discarded. The Postgres delete
// still succeeds and is still authoritative, but the lookup failure is
// recorded to sn_writeback_failures for manual backfill instead of vanishing.
func TestCaseService_RemoveCaseTag_RecordsSNWritebackFailureOnLookupError(t *testing.T) {
	lookupErr := errors.New("sn sys id lookup: connection reset")
	repo := &stubCaseRepo{
		removeCaseTag: func(context.Context, string, string, string) error { return nil },
		getCaseTagSNSysID: func(context.Context, string, string) (*string, error) {
			return nil, lookupErr
		},
	}
	mirrorCalled := make(chan struct{}, 1)
	mirror := &stubMirrorCaseService{
		removeCaseTagFn: func(context.Context, string, string) error {
			mirrorCalled <- struct{}{}
			return nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if err := svc.RemoveCaseTag(ctx, testDeploymentUUID, testUUID); err != nil {
		t.Fatalf("Postgres delete must still be reported as a success despite the lookup error, got: %v", err)
	}

	select {
	case <-mirrorCalled:
		t.Fatal("mirror.RemoveCaseTag was called despite the sys_id lookup having failed")
	case <-time.After(300 * time.Millisecond):
		// expected: no mirror call, the lookup error means we don't know
		// what to remove
	}

	waitFor(t, func() bool { return failures.count() == 1 })
	req := failures.calls[0]
	if req.EntityType != "case_tag" || req.EntityID != testDeploymentUUID || req.Operation != "remove" {
		t.Errorf("unexpected failure record: %+v", req)
	}
	if req.Error != lookupErr.Error() {
		t.Errorf("failure record Error = %q, want %q", req.Error, lookupErr.Error())
	}
}

// TestCaseService_UpdateCase_WatchList_MirrorsToServiceNow covers the
// writeback wiring for WatchList specifically: on a successful Postgres
// watch-list replace, the mirror's patchCaseWatchList (the bare,
// read-free/event-free PATCH -- see that method's own doc comment) is
// dispatched asynchronously with the same user ids, and does not block or
// affect the response.
func TestCaseService_UpdateCase_WatchList_MirrorsToServiceNow(t *testing.T) {
	userIDs := []string{testDeploymentUUID}

	called := make(chan []string, 1)
	mirror := &stubMirrorCaseService{
		patchCaseWatchListFn: func(_ context.Context, caseID string, gotUserIDs []string) (domain.UpdatedCase, error) {
			called <- gotUserIDs
			return domain.UpdatedCase{}, nil
		},
	}
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			// No project linked: validateWatchListProjectMembership has
			// nothing to check against and skips, same as any other
			// project-less case (see that method's own doc comment) --
			// this test is about the SN mirror dispatch, not that check.
			return domain.CaseView{}, nil
		},
		setCaseWatchList: func(_ context.Context, caseID string, gotUserIDs []string, actorEmail string) ([]domain.WatchListUser, time.Time, error) {
			return nil, time.Now(), nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &userIDs}
	if _, err := svc.UpdateCase(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if len(got) != 1 || got[0] != testDeploymentUUID {
			t.Errorf("mirror got %v, want %v", got, userIDs)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.patchCaseWatchList was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestCaseService_UpdateCase_WatchList_MirrorGetsExactlySubmittedIDs proves
// the ServiceNow mirror and Postgres always agree on a watch-list edit now:
// neither merges in the account's four stakeholders any more (see
// addRequestedWatchers' own doc comment for why they're never persisted at
// all) -- both simply get exactly what the caller submitted.
func TestCaseService_UpdateCase_WatchList_MirrorGetsExactlySubmittedIDs(t *testing.T) {
	const projectID = "6fa0b42d-1bfa-a694-a002-c9d3604bcb77"
	const submittedID = "11111111-1111-1111-1111-111111111111"
	userIDs := []string{submittedID}

	called := make(chan []string, 1)
	mirror := &stubMirrorCaseService{
		patchCaseWatchListFn: func(_ context.Context, caseID string, gotUserIDs []string) (domain.UpdatedCase, error) {
			called <- gotUserIDs
			return domain.UpdatedCase{}, nil
		},
	}
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ProjectDetails: &domain.EntityRef{ID: projectID}}, nil
		},
		setCaseWatchList: func(context.Context, string, []string, string) ([]domain.WatchListUser, time.Time, error) {
			return nil, time.Now(), nil
		},
	}
	contactRepo := &stubProjectContactRepo{
		getProjectContactByUserID: func(context.Context, string, string, string) (repository.ProjectContactRow, error) {
			return repository.ProjectContactRow{Email: "member@example.com", RegistrationState: "REGISTERED"}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, contactRepo, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &userIDs}
	if _, err := svc.UpdateCase(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if len(got) != 1 || got[0] != submittedID {
			t.Errorf("mirror got %v, want exactly %v (no stakeholder merge)", got, userIDs)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.patchCaseWatchList was never called")
	}
}

// TestCaseService_UpdateCase_WatchList_MirrorFailureRecordsWritebackFailure
// covers the failure half: Postgres already succeeded, so the call must
// still report success, but the mirror error lands in sn_writeback_failures
// for manual backfill.
func TestCaseService_UpdateCase_WatchList_MirrorFailureRecordsWritebackFailure(t *testing.T) {
	userIDs := []string{testDeploymentUUID}

	mirror := &stubMirrorCaseService{
		patchCaseWatchListFn: func(context.Context, string, []string) (domain.UpdatedCase, error) {
			return domain.UpdatedCase{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{}, nil
		},
		setCaseWatchList: func(context.Context, string, []string, string) ([]domain.WatchListUser, time.Time, error) {
			return nil, time.Now(), nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewCaseServiceWithSNWriteback(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &userIDs}
	if _, err := svc.UpdateCase(ctx, req); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

// TestCaseService_UpdateCase_WatchList_RejectsNonProjectMember is the
// regression test for validateWatchListProjectMembership: a watch-list
// update naming a user who is not a project_contact on the case's project
// must be rejected wholesale, and SetCaseWatchList must never be called.
func TestCaseService_UpdateCase_WatchList_RejectsNonProjectMember(t *testing.T) {
	const projectID = "6fa0b42d-1bfa-a694-a002-c9d3604bcb77"
	const memberUserID = "11111111-1111-1111-1111-111111111111"
	const nonMemberUserID = "22222222-2222-2222-2222-222222222222"
	userIDs := []string{memberUserID, nonMemberUserID}

	setCaseWatchListCalled := false
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ProjectDetails: &domain.EntityRef{ID: projectID}}, nil
		},
		setCaseWatchList: func(context.Context, string, []string, string) ([]domain.WatchListUser, time.Time, error) {
			setCaseWatchListCalled = true
			return nil, time.Now(), nil
		},
	}
	contactRepo := &stubProjectContactRepo{
		getProjectContactByUserID: func(_ context.Context, gotProjectID, userID, _ string) (repository.ProjectContactRow, error) {
			if gotProjectID != projectID {
				t.Errorf("projectID = %q, want %q", gotProjectID, projectID)
			}
			if userID == nonMemberUserID {
				return repository.ProjectContactRow{}, &apierror.NotFoundError{Msg: "contact not found on this project"}
			}
			return repository.ProjectContactRow{Email: "member@example.com", RegistrationState: "REGISTERED"}, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{getUserDetail: func(context.Context, string) (domain.UserDetail, error) {
		return domain.UserDetail{UserType: domain.UserTypeExternal, Email: "outsider@example.com"}, nil
	}}, nil, alwaysUnrestrictedAccess{}, contactRepo)

	req := domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &userIDs}
	_, err := svc.UpdateCase(context.Background(), req)

	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected a ValidationError, got %v", err)
	}
	if setCaseWatchListCalled {
		t.Error("SetCaseWatchList was called despite a non-member user in the watch list")
	}
}

// TestCaseService_UpdateCase_WatchList_RejectsUnregisteredContact is the
// regression test for a CodeRabbit finding on this same change:
// validateWatchListProjectMembership only checked that a project_contact row
// existed, not its state -- an INVITED (not yet accepted) or DEACTIVATED
// contact would pass. Neither actually belongs on the project today; only
// REGISTERED should.
func TestCaseService_UpdateCase_WatchList_RejectsUnregisteredContact(t *testing.T) {
	const projectID = "6fa0b42d-1bfa-a694-a002-c9d3604bcb77"
	const invitedUserID = "11111111-1111-1111-1111-111111111111"
	userIDs := []string{invitedUserID}

	setCaseWatchListCalled := false
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ProjectDetails: &domain.EntityRef{ID: projectID}}, nil
		},
		setCaseWatchList: func(context.Context, string, []string, string) ([]domain.WatchListUser, time.Time, error) {
			setCaseWatchListCalled = true
			return nil, time.Now(), nil
		},
	}
	contactRepo := &stubProjectContactRepo{
		getProjectContactByUserID: func(context.Context, string, string, string) (repository.ProjectContactRow, error) {
			return repository.ProjectContactRow{Email: "invited@example.com", RegistrationState: "INVITED"}, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{getUserDetail: func(context.Context, string) (domain.UserDetail, error) {
		return domain.UserDetail{UserType: domain.UserTypeExternal, Email: "outsider@example.com"}, nil
	}}, nil, alwaysUnrestrictedAccess{}, contactRepo)

	req := domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &userIDs}
	_, err := svc.UpdateCase(context.Background(), req)

	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected a ValidationError for an INVITED (not yet REGISTERED) contact, got %v", err)
	}
	if setCaseWatchListCalled {
		t.Error("SetCaseWatchList was called despite an unregistered contact in the watch list")
	}
}

// TestCaseService_UpdateCase_WatchList_AcceptsProjectMembers is the positive
// counterpart: every user in the watch list resolves as a project_contact,
// so the update proceeds normally.
func TestCaseService_UpdateCase_WatchList_AcceptsProjectMembers(t *testing.T) {
	const projectID = "6fa0b42d-1bfa-a694-a002-c9d3604bcb77"
	userIDs := []string{"11111111-1111-1111-1111-111111111111", "33333333-3333-3333-3333-333333333333"}

	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ProjectDetails: &domain.EntityRef{ID: projectID}}, nil
		},
		setCaseWatchList: func(context.Context, string, []string, string) ([]domain.WatchListUser, time.Time, error) {
			return nil, time.Now(), nil
		},
	}
	contactRepo := &stubProjectContactRepo{
		getProjectContactByUserID: func(context.Context, string, string, string) (repository.ProjectContactRow, error) {
			return repository.ProjectContactRow{Email: "member@example.com", RegistrationState: "REGISTERED"}, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}, nil, alwaysUnrestrictedAccess{}, contactRepo)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &userIDs}
	if _, err := svc.UpdateCase(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// watchListTestService builds a Postgres-backed caseService whose case lives on
// a project, with contactState/userDetail controlling how the two watcher
// lookups answer for every id. contactState "" means no project_contact row.
func watchListTestService(contactState string, user domain.UserDetail) (CaseService, *bool) {
	const projectID = "6fa0b42d-1bfa-a694-a002-c9d3604bcb77"
	written := false
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ProjectDetails: &domain.EntityRef{ID: projectID}}, nil
		},
		setCaseWatchList: func(context.Context, string, []string, string) ([]domain.WatchListUser, time.Time, error) {
			written = true
			return nil, time.Now(), nil
		},
	}
	contactRepo := &stubProjectContactRepo{
		getProjectContactByUserID: func(context.Context, string, string, string) (repository.ProjectContactRow, error) {
			if contactState == "" {
				return repository.ProjectContactRow{}, &apierror.NotFoundError{Msg: "contact not found on this project"}
			}
			return repository.ProjectContactRow{Email: "jane.doe@example.com", RegistrationState: contactState}, nil
		},
	}
	users := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testDeploymentUUID, Email: "actor@example.com"}, nil
		},
		getUserDetail: func(context.Context, string) (domain.UserDetail, error) { return user, nil },
	}
	return NewCaseService(repo, users, nil, alwaysUnrestrictedAccess{}, contactRepo), &written
}

func TestCaseService_UpdateCase_WatchList_AcceptsInternalUserWithoutContactRow(t *testing.T) {
	svc, written := watchListTestService("", domain.UserDetail{UserType: domain.UserTypeInternal, Email: "jane.doe@example.com"})
	ids := []string{"00000000-0000-0000-0000-000000000001"}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "actor@example.com"))
	if _, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &ids}); err != nil {
		t.Fatalf("internal user must be accepted as a watcher, got %v", err)
	}
	if !*written {
		t.Error("SetCaseWatchList was not called for an accepted internal watcher")
	}
}

func TestCaseService_UpdateCase_WatchList_AcceptsRegisteredContactWithoutUserLookup(t *testing.T) {
	// getUserDetail panics if reached: a registered contact must short-circuit.
	svc, _ := watchListTestService("REGISTERED", domain.UserDetail{})
	svc.(*caseService).userRepo = stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "actor@example.com"}, nil
	}}
	ids := []string{"00000000-0000-0000-0000-000000000002"}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "actor@example.com"))
	if _, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &ids}); err != nil {
		t.Fatalf("registered contact must be accepted as a watcher, got %v", err)
	}
}

func TestCaseService_UpdateCase_WatchList_RejectsExternalNonContactWithoutUUIDInMessage(t *testing.T) {
	const id = "00000000-0000-0000-0000-000000000003"
	cases := map[string]domain.UserDetail{
		"by email": {UserType: domain.UserTypeExternal, Email: "jane.doe@example.com", Name: "Jane Doe"},
		"by name":  {UserType: domain.UserTypeExternal, Name: "Jane Doe"},
		"no data":  {UserType: domain.UserTypeExternal},
	}
	for name, user := range cases {
		t.Run(name, func(t *testing.T) {
			svc, written := watchListTestService("INVITED", user)
			ids := []string{id}
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "actor@example.com"))
			_, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &ids})
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected a ValidationError, got %v", err)
			}
			if strings.Contains(ve.Msg, id) || uuidInMessageRE.MatchString(ve.Msg) {
				t.Errorf("message must not contain a raw UUID, got %q", ve.Msg)
			}
			if user.Email != "" && !strings.Contains(ve.Msg, user.Email) {
				t.Errorf("message should name the user by email, got %q", ve.Msg)
			}
			if *written {
				t.Error("SetCaseWatchList was called despite a rejected watcher")
			}
		})
	}
}

func TestCaseService_UpdateCase_WatchList_UnknownUserMessageHasNoUUID(t *testing.T) {
	const id = "00000000-0000-0000-0000-000000000004"
	svc, _ := watchListTestService("", domain.UserDetail{})
	svc.(*caseService).userRepo = stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testDeploymentUUID, Email: "actor@example.com"}, nil
		},
		getUserDetail: func(_ context.Context, got string) (domain.UserDetail, error) {
			return domain.UserDetail{}, &apierror.NotFoundError{Msg: "no user found with id: " + got}
		},
	}
	ids := []string{id}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "actor@example.com"))
	_, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &ids})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected a ValidationError, got %v", err)
	}
	if uuidInMessageRE.MatchString(ve.Msg) {
		t.Errorf("message must not contain a raw UUID, got %q", ve.Msg)
	}
}

var uuidInMessageRE = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// TestCaseService_UpdateCase_WatchList_PersistsExactlySubmittedIDs proves
// the replacement design: the account's four named stakeholders are never
// merged in any more (see addRequestedWatchers' own doc comment) -- a watch-
// list edit persists exactly what the caller submitted, nothing added,
// nothing silently kept from a previous write.
func TestCaseService_UpdateCase_WatchList_PersistsExactlySubmittedIDs(t *testing.T) {
	const projectID = "6fa0b42d-1bfa-a694-a002-c9d3604bcb77"
	const submittedID = "11111111-1111-1111-1111-111111111111"
	userIDs := []string{submittedID}

	var setWatchListUserIDs []string
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ProjectDetails: &domain.EntityRef{ID: projectID}}, nil
		},
		setCaseWatchList: func(_ context.Context, _ string, ids []string, _ string) ([]domain.WatchListUser, time.Time, error) {
			setWatchListUserIDs = ids
			return nil, time.Now(), nil
		},
	}
	contactRepo := &stubProjectContactRepo{
		getProjectContactByUserID: func(_ context.Context, _ string, userID, _ string) (repository.ProjectContactRow, error) {
			return repository.ProjectContactRow{Email: "member@example.com", RegistrationState: "REGISTERED"}, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}, nil, alwaysUnrestrictedAccess{}, contactRepo)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &userIDs}
	if _, err := svc.UpdateCase(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(setWatchListUserIDs) != 1 || setWatchListUserIDs[0] != submittedID {
		t.Errorf("SetCaseWatchList userIDs = %v, want exactly %v (no stakeholder merge)", setWatchListUserIDs, userIDs)
	}
}

// TestCaseService_UpdateCase_WatchList_SkipsCheckWhenNoProjectLinked confirms
// a case with no project linked (ProjectDetails nil, a documented real
// state) never consults projectContactRepo at all -- there is nothing to
// validate membership against.
func TestCaseService_UpdateCase_WatchList_SkipsCheckWhenNoProjectLinked(t *testing.T) {
	userIDs := []string{"11111111-1111-1111-1111-111111111111"}

	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ProjectDetails: nil}, nil
		},
		setCaseWatchList: func(context.Context, string, []string, string) ([]domain.WatchListUser, time.Time, error) {
			return nil, time.Now(), nil
		},
	}
	// No getProjectContactByUserID stub set: a call to it panics, so this
	// test also proves the check is skipped, not just that it would pass.
	svc := NewCaseService(repo, stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}, nil, alwaysUnrestrictedAccess{}, &stubProjectContactRepo{})

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateCaseRequest{ID: testDeploymentUUID, WatchList: &userIDs}
	if _, err := svc.UpdateCase(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCaseService_UpdateCase_DualWriteRequiresResolutionFieldsAndMirrorsThem(t *testing.T) {
	closed := domain.CaseStateClosed
	notes := "Fixed"
	code := domain.CaseResolutionCodeSolvedFixedBySupportGuidanceProvided
	cause := domain.CaseCauseProductBug

	newSvc := func(mirror *stubMirrorCaseService) CaseService {
		repo := &stubCaseRepo{
			getCaseByID: func(_ context.Context, id string, _ repository.SearchScope) (domain.CaseView, error) {
				st := domain.CaseStateOpen
				return domain.CaseView{ID: id, State: &st}, nil
			},
			updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
				st := *req.State
				return domain.Case{ID: req.ID, State: &st}, nil, nil
			},
		}
		dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
		return NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")
	}

	t.Run("closed without resolution fields is rejected", func(t *testing.T) {
		svc := newSvc(&stubMirrorCaseService{})
		_, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &closed, CloseNotes: &notes})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("closed with all three reaches the mirror", func(t *testing.T) {
		got := make(chan *caseResolutionFields, 1)
		mirror := &stubMirrorCaseService{patchCaseFieldsFn: func(_ context.Context, _ string, _ *domain.CaseState, _ *domain.CaseSeverity, _ *domain.CaseWorkState, _ *bool, r *caseResolutionFields) (domain.UpdatedCase, error) {
			got <- r
			return domain.UpdatedCase{}, nil
		}}
		svc := newSvc(mirror)
		if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &closed, ResolutionCode: &code, Cause: &cause, CloseNotes: &notes}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		select {
		case r := <-got:
			if r == nil || r.Code == nil || r.Cause == nil || r.CloseNotes == nil {
				t.Fatalf("mirror did not receive all resolution fields: %+v", r)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("mirror.patchCaseFields was never called")
		}
	})

	// TestCaseService_UpdateCase_DualWriteRequiresResolutionFieldsAndMirrorsThem's
	// own "closed without resolution fields is rejected" case covers type
	// "case" (the stub's getCaseByID returns no Type at all, which this
	// requirement conservatively treats as "case" -- see UpdateCase's own
	// comment on that fallback). This is the regression test for a real,
	// reported bug: the exact same requirement fired for every other
	// case-like type too -- engagement/service_request/
	// security_report_analysis/announcement -- even though
	// resolutionCode/cause/closeNotes have no backing column for any of
	// them, and no close flow for those types ever collects them. Closing a
	// Security Report (security_report_analysis) with no resolution fields
	// must succeed.
	// TestCaseService_UpdateCase_DualWriteRequiresResolutionFieldsAndMirrorsThem's
	// own "closed without resolution fields is rejected" case covers type
	// "case" (the stub's getCaseByID returns no Type at all, which this
	// requirement conservatively treats as "case" -- see UpdateCase's own
	// comment on that fallback).
	//
	// This is the regression test for a CodeRabbit-caught follow-up: an
	// external caller is not simply exempted from supplying resolution
	// fields -- leaving them nil would make the ServiceNow mirror write fail
	// outright (ServiceNow's own case-closure workflow requires them), so
	// they're defaulted to real, valid values instead ("solved by
	// customer" / "unknown cause" / a generic note) and those defaults must
	// actually reach both the Postgres write and the mirror.
	t.Run("external caller's missing resolution fields are defaulted, not left nil", func(t *testing.T) {
		var gotReq domain.UpdateCaseRequest
		repo := &stubCaseRepo{
			getCaseByID: func(_ context.Context, id string, _ repository.SearchScope) (domain.CaseView, error) {
				st := domain.CaseStateOpen
				typ := "case"
				return domain.CaseView{ID: id, State: &st, Type: &typ}, nil
			},
			updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
				gotReq = req
				st := *req.State
				return domain.Case{ID: req.ID, State: &st}, nil, nil
			},
		}
		dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
		gotMirror := make(chan *caseResolutionFields, 1)
		mirror := &stubMirrorCaseService{patchCaseFieldsFn: func(_ context.Context, _ string, _ *domain.CaseState, _ *domain.CaseSeverity, _ *domain.CaseWorkState, _ *bool, r *caseResolutionFields) (domain.UpdatedCase, error) {
			gotMirror <- r
			return domain.UpdatedCase{}, nil
		}}
		// A non-Unrestricted caller must carry a user token that resolves to a
		// known user before UpdateCase reaches the resolution-field defaults.
		users := stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			return domain.User{ID: "00000000-0000-0000-0000-000000000001", Email: email, UserType: domain.UserTypeExternal}, nil
		}}
		svc := NewCaseServiceWithSNWriteback(repo, users, nil, stubAccess{scope: AccessScope{Unrestricted: false}}, nil, dispatcher, mirror, nil, "")
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

		if _, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &closed}); err != nil {
			t.Fatalf("unexpected error closing a case with no resolution fields as an external caller: %v", err)
		}
		if gotReq.ResolutionCode == nil || *gotReq.ResolutionCode != domain.CaseResolutionCodeSolvedByCustomer {
			t.Errorf("ResolutionCode = %v, want %q", gotReq.ResolutionCode, domain.CaseResolutionCodeSolvedByCustomer)
		}
		if gotReq.Cause == nil || *gotReq.Cause != domain.CaseCauseUnknown {
			t.Errorf("Cause = %v, want %q", gotReq.Cause, domain.CaseCauseUnknown)
		}
		if gotReq.CloseNotes == nil || *gotReq.CloseNotes != externalCloseDefaultNotes {
			t.Errorf("CloseNotes = %v, want %q", gotReq.CloseNotes, externalCloseDefaultNotes)
		}
		select {
		case r := <-gotMirror:
			if r == nil || r.Code == nil || *r.Code != domain.CaseResolutionCodeSolvedByCustomer {
				t.Errorf("mirror resolution code = %v, want %q", r, domain.CaseResolutionCodeSolvedByCustomer)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("mirror.patchCaseFields was never called")
		}
	})

	// TestCaseService_UpdateCase_MixedIdentityStillRequiresResolutionFields
	// is the regression test for the other CodeRabbit-caught gap: a caller
	// whose email ALSO has an active EXTERNAL row resolves to a non-
	// Unrestricted, project-scoped AccessScope (accessService.scopeForUser's
	// "external wins" rule), but is still genuinely WSO2 staff and must
	// still be required to supply resolution fields -- checking
	// scope.Unrestricted alone (an earlier version of this fix) would wrongly
	// exempt them.
	t.Run("caller with a mixed internal+external identity still requires resolution fields", func(t *testing.T) {
		repo := &stubCaseRepo{
			getCaseByID: func(_ context.Context, id string, _ repository.SearchScope) (domain.CaseView, error) {
				st := domain.CaseStateOpen
				typ := "case"
				return domain.CaseView{ID: id, State: &st, Type: &typ}, nil
			},
		}
		dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
		mirror := &stubMirrorCaseService{}
		scope := AccessScope{ProjectIDs: []string{"p1"}, ViewerEmail: writeEmail, HasInternalAccess: true}
		svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, stubAccess{scope: scope}, nil, dispatcher, mirror, nil, "")

		_, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &closed})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected ValidationError for a mixed-identity caller with no resolution fields, got %T: %v", err, err)
		}
	})

	t.Run("closing a non-case type without resolution fields is allowed", func(t *testing.T) {
		repo := &stubCaseRepo{
			getCaseByID: func(_ context.Context, id string, _ repository.SearchScope) (domain.CaseView, error) {
				st := domain.CaseStateOpen
				typ := "security_report_analysis"
				return domain.CaseView{ID: id, State: &st, Type: &typ}, nil
			},
			updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
				st := *req.State
				return domain.Case{ID: req.ID, State: &st}, nil, nil
			},
		}
		dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
		mirror := &stubMirrorCaseService{patchCaseFieldsFn: func(_ context.Context, _ string, _ *domain.CaseState, _ *domain.CaseSeverity, _ *domain.CaseWorkState, _ *bool, _ *caseResolutionFields) (domain.UpdatedCase, error) {
			return domain.UpdatedCase{}, nil
		}}
		svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil, dispatcher, mirror, nil, "")

		if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &closed}); err != nil {
			t.Fatalf("unexpected error closing a security report with no resolution fields: %v", err)
		}
	})
}

func TestCaseService_AggregateCases_GroupsAndCapsBuckets(t *testing.T) {
	repo := &stubCaseRepo{aggregateCases: func(_ context.Context, _ domain.SearchCasesRequest, groupBy string) ([]domain.AggregateBucket, error) {
		if groupBy != "state" {
			t.Errorf("groupBy = %q, want state", groupBy)
		}
		return []domain.AggregateBucket{{Key: "open", Label: "open", Count: 5}, {Key: "closed", Label: "closed", Count: 3}, {Key: "reopened", Label: "reopened", Count: 1}}, nil
	}}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "state", MaxGroups: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Groups) != 2 || resp.OthersCount != 1 || resp.TotalRecords != 9 {
		t.Errorf("got groups=%d others=%d total=%d, want 2/1/9", len(resp.Groups), resp.OthersCount, resp.TotalRecords)
	}
	if _, err := svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "bogus"}); err == nil {
		t.Error("expected a ValidationError for an unknown groupBy")
	}
}
