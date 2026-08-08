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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubCaseRepo is a minimal repository.CaseRepository whose SearchCases
// panics if called: tests using it prove ParseCaseFieldFilters' rejection of
// an unsupported field happens before the Postgres backend ever reaches the
// repository, not merely that the repository ignores the field.
type stubCaseRepo struct {
	searchCases func(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error)
}

func (s *stubCaseRepo) CreateCase(context.Context, domain.CreateCaseRequest) (domain.Case, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) GetCaseByID(context.Context, string) (domain.CaseView, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) SearchCases(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error) {
	if s.searchCases != nil {
		return s.searchCases(ctx, req)
	}
	panic("SearchCases called unexpectedly: the unsupported-field check should have short-circuited before reaching the repository")
}
func (s *stubCaseRepo) CreateCaseComment(context.Context, domain.CreateCaseCommentRequest) (domain.CaseComment, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) SearchCaseComments(context.Context, domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) UpdateCase(context.Context, domain.UpdateCaseRequest) (domain.Case, error) {
	panic("not implemented")
}

// stubUserRepo is a minimal repository.UserRepository; SearchCases doesn't
// exercise it beyond the createdBy-current-user path, which these tests don't
// use.
type stubUserRepo struct{}

func (stubUserRepo) SearchUsers(context.Context, domain.SearchUsersRequest) ([]domain.User, int, error) {
	panic("not implemented")
}
func (stubUserRepo) GetUserByEmail(context.Context, string) (domain.User, error) {
	panic("not implemented")
}

// TestCaseService_SearchCases_RejectsUnsupportedPostgresFields proves the
// Postgres-backed SearchCases path rejects each of the 9 filter fields
// ParseCaseFieldFilters accepts but the Postgres repository has no query
// support for (they dot-walk into SN-specific concepts with no Postgres
// schema equivalent), rather than silently accepting the request and
// returning a broader-than-requested result set.
func TestCaseService_SearchCases_RejectsUnsupportedPostgresFields(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	cases := []struct {
		name   string
		filter domain.CaseFieldFilter
	}{
		{name: "tag in", filter: domain.CaseFieldFilter{Field: "tag", Op: "in", Values: []string{"beta"}}},
		{name: "tag notIn", filter: domain.CaseFieldFilter{Field: "tag", Op: "notIn", Values: []string{"beta"}}},
		{name: "parentId", filter: domain.CaseFieldFilter{Field: "parentId", Op: "eq", Values: []string{"00000000-0000-0000-0000-000000000000"}}},
		{name: "product", filter: domain.CaseFieldFilter{Field: "product", Op: "in", Values: []string{"API Manager"}}},
		{name: "projectOnboardingStatus", filter: domain.CaseFieldFilter{Field: "projectOnboardingStatus", Op: "in", Values: []string{"Completed"}}},
		{name: "projectType", filter: domain.CaseFieldFilter{Field: "projectType", Op: "in", Values: []string{"Subscription"}}},
		{name: "integrationCsTeam", filter: domain.CaseFieldFilter{Field: "integrationCsTeam", Op: "in", Values: []string{"00000000-0000-0000-0000-000000000000"}}},
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
// 11 fields the Postgres repository does support are not caught by the new
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
			svc := NewCaseService(repo, stubUserRepo{})
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

// TestCaseService_SearchCases_RejectsServiceNowOnlyOptions proves the Postgres
// path rejects the search options that only snCaseService implements: the
// Task-SLA percent filter, the two escalation filters, OR groups, and grouped
// counts. caseRepo.SearchCases models none of them, so accepting the request
// would silently drop the predicate and return a wider result set with a 200.
// The stub repository panics if reached, so a passing test proves the
// short-circuit, not merely that the repository ignored the option.
func TestCaseService_SearchCases_RejectsServiceNowOnlyOptions(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	cases := []struct {
		name    string
		req     domain.SearchCasesRequest
		wantMsg string
	}{
		{
			name: "taskSLABusinessElapsedPercent",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "taskSLABusinessElapsedPercent", Op: "gte", Values: []string{"80"}}},
			}},
			wantMsg: `field "taskSLABusinessElapsedPercent" is not supported by this data source`,
		},
		{
			name: "taskSLABusinessElapsedPercent lte 0",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "taskSLABusinessElapsedPercent", Op: "lte", Values: []string{"0"}}},
			}},
			wantMsg: `field "taskSLABusinessElapsedPercent" is not supported by this data source`,
		},
		{
			name: "escalationLevel",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "escalationLevel", Op: "in", Values: []string{"level_1"}}},
			}},
			wantMsg: `field "escalationLevel" is not supported by this data source`,
		},
		{
			name: "escalation",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "escalation", Op: "isNotEmpty"}},
			}},
			wantMsg: `field "escalation" is not supported by this data source`,
		},
		{
			name: "anyOf",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				AnyOf: []domain.CaseFilterBranch{
					{Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open"}}}},
				},
			}},
			wantMsg: "anyOf is not supported by this data source",
		},
		{
			name:    "groupBy",
			req:     domain.SearchCasesRequest{GroupBy: "state"},
			wantMsg: "groupBy is not supported by this data source",
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
