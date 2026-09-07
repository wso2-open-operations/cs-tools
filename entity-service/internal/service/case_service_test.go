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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubCaseRepo is a minimal repository.CaseRepository whose SearchCases
// panics if called: tests using it prove ParseCaseFieldFilters' rejection of
// an unsupported field happens before the Postgres backend ever reaches the
// repository, not merely that the repository ignores the field.
type stubCaseRepo struct {
	searchCases           func(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error)
	createCaseAttachment  func(ctx context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error)
	searchCaseAttachments func(ctx context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error)
	getCaseAttachmentByID func(ctx context.Context, id string) (domain.Attachment, error)
	deleteCaseAttachment  func(ctx context.Context, id string) error
	updateAttachmentName  func(ctx context.Context, id, name, updatedBy string) (time.Time, error)
	confirmCaseAttachment func(ctx context.Context, id string) (domain.Attachment, error)
	searchCaseComments    func(ctx context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error)
	updateCase            func(ctx context.Context, req domain.UpdateCaseRequest) (domain.Case, error)
	countNonClosedChild   func(ctx context.Context, parentID string) (int, error)
	updateCaseParent      func(ctx context.Context, caseID string, parentID *string) (domain.Case, error)
	getCaseByID           func(ctx context.Context, id string) (domain.CaseView, error)
	createCaseComment     func(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CaseComment, error)
}

func (s *stubCaseRepo) CreateCase(context.Context, domain.CreateCaseRequest) (domain.Case, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) GetCaseByID(ctx context.Context, id string) (domain.CaseView, error) {
	if s.getCaseByID != nil {
		return s.getCaseByID(ctx, id)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) SearchCases(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error) {
	if s.searchCases != nil {
		return s.searchCases(ctx, req)
	}
	panic("SearchCases called unexpectedly: the unsupported-field check should have short-circuited before reaching the repository")
}
func (s *stubCaseRepo) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CaseComment, error) {
	if s.createCaseComment != nil {
		return s.createCaseComment(ctx, req)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
	if s.searchCaseComments != nil {
		return s.searchCaseComments(ctx, req)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.Case, error) {
	if s.updateCase != nil {
		return s.updateCase(ctx, req)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) CountNonClosedChildCases(ctx context.Context, parentID string) (int, error) {
	if s.countNonClosedChild != nil {
		return s.countNonClosedChild(ctx, parentID)
	}
	panic("CountNonClosedChildCases called unexpectedly")
}
func (s *stubCaseRepo) UpdateCaseParent(ctx context.Context, caseID string, parentID *string) (domain.Case, error) {
	if s.updateCaseParent != nil {
		return s.updateCaseParent(ctx, caseID, parentID)
	}
	panic("UpdateCaseParent called unexpectedly")
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

// stubUserRepo is a minimal repository.UserRepository; SearchCases doesn't
// exercise it beyond the createdBy-current-user path, which these tests don't
// use.
type stubUserRepo struct {
	getUserByEmail func(ctx context.Context, email string) (domain.User, error)
}

func (stubUserRepo) SearchUsers(context.Context, domain.SearchUsersRequest) ([]domain.User, int, error) {
	panic("not implemented")
}
func (s stubUserRepo) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	if s.getUserByEmail != nil {
		return s.getUserByEmail(ctx, email)
	}
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
		{name: "creTeam", filter: domain.CaseFieldFilter{Field: "creTeam", Op: "in", Values: []string{"00000000-0000-0000-0000-000000000000"}}},
		{name: "sreTeam", filter: domain.CaseFieldFilter{Field: "sreTeam", Op: "in", Values: []string{"00000000-0000-0000-0000-000000000000"}}},
		{name: "assignedUserId isEmpty (Unassigned)", filter: domain.CaseFieldFilter{Field: "assignedUserId", Op: "isEmpty"}},
		{name: "resolutionNotes isEmpty", filter: domain.CaseFieldFilter{Field: "resolutionNotes", Op: "isEmpty"}},
		// state+in IS supported by this backend; only the exclusion is not.
		{name: "state notIn", filter: domain.CaseFieldFilter{Field: "state", Op: "notIn", Values: []string{"closed"}}},
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
			name: "slaBreached",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "slaBreached", Op: "eq", Values: []string{"true"}}},
			}},
			wantMsg: `field "slaBreached" is not supported by this data source`,
		},
		{
			name: "accountEscalationActive",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "accountEscalationActive", Op: "eq", Values: []string{"true"}}},
			}},
			wantMsg: `field "accountEscalationActive" is not supported by this data source`,
		},
		{
			name: "resolvedOn gte",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "resolvedOn", Op: "gte", Values: []string{"2026-01-01"}}},
			}},
			wantMsg: `field "resolvedOn" is not supported by this data source`,
		},
		{
			name: "resolvedOn lte",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "resolvedOn", Op: "lte", Values: []string{"2026-01-31"}}},
			}},
			wantMsg: `field "resolvedOn" is not supported by this data source`,
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
		svc := NewCaseService(repo, stubUserRepo{})

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
		svc := NewCaseService(repo, stubUserRepo{})

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
		svc := NewCaseService(repo, stubUserRepo{})

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
		svc := NewCaseService(repo, stubUserRepo{})

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
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{})
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

// errCountChildren is a sentinel used to prove UpdateCase surfaces a
// CountNonClosedChildCases failure unchanged rather than swallowing it or
// misreporting it as a validation error.
var errCountChildren = errors.New("count children failed")

// TestCaseService_UpdateCase_PreventClosureOfParent covers the ported
// ServiceNow "Prevent Closure Of Parent" business rule on the native path:
// a case may not be closed while it still has non-closed child cases, but the
// close proceeds once none remain, and a lookup failure is surfaced, not
// swallowed.
func TestCaseService_UpdateCase_PreventClosureOfParent(t *testing.T) {
	closed := domain.CaseStateClosed
	ctx := context.Background()

	t.Run("open children block the close", func(t *testing.T) {
		gotParent := ""
		repo := &stubCaseRepo{
			countNonClosedChild: func(_ context.Context, parentID string) (int, error) {
				gotParent = parentID
				return 2, nil
			},
			// updateCase intentionally left nil: it must never be reached when the
			// guard rejects, so a call would panic and fail the test.
		}
		svc := NewCaseService(repo, stubUserRepo{})

		_, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testCaseID, State: &closed})

		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
		if ve.Msg != "cannot close this case while it has open child cases" {
			t.Fatalf("unexpected message: %q", ve.Msg)
		}
		if gotParent != testCaseID {
			t.Fatalf("expected child count to be queried for %q, got %q", testCaseID, gotParent)
		}
	})

	t.Run("no open children allows the close", func(t *testing.T) {
		updateCalled := false
		repo := &stubCaseRepo{
			countNonClosedChild: func(_ context.Context, _ string) (int, error) { return 0, nil },
			updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, error) {
				updateCalled = true
				return domain.Case{ID: req.ID, State: domain.CaseStateClosed}, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{})

		resp, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testCaseID, State: &closed})
		if err != nil {
			t.Fatalf("expected close to succeed, got error: %v", err)
		}
		if !updateCalled {
			t.Fatal("expected repo.UpdateCase to be called once the guard passes")
		}
		if resp.Case.State != domain.CaseStateClosed {
			t.Fatalf("expected closed state in response, got %q", resp.Case.State)
		}
	})

	t.Run("a child-count failure is surfaced, not masked", func(t *testing.T) {
		repo := &stubCaseRepo{
			countNonClosedChild: func(_ context.Context, _ string) (int, error) { return 0, errCountChildren },
		}
		svc := NewCaseService(repo, stubUserRepo{})

		_, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testCaseID, State: &closed})
		if !errors.Is(err, errCountChildren) {
			t.Fatalf("expected the child-count error to be surfaced, got %T: %v", err, err)
		}
		var ve *apierror.ValidationError
		if asValidationError(err, &ve) {
			t.Fatal("a lookup failure must not be reported as a validation error")
		}
	})
}

// TestCaseService_UpdateCase_GuardOnlyRunsOnClose proves the parent-closure
// guard is scoped to the transition to closed: any other state change, a
// severity change, or a work-state change must not trigger a child-count
// query. countNonClosedChild is left nil, so it panics if the guard runs when
// it should not.
func TestCaseService_UpdateCase_GuardOnlyRunsOnClose(t *testing.T) {
	ctx := context.Background()
	wip := domain.CaseStateWorkInProgress
	reopened := domain.CaseStateReopened
	high := domain.CaseSeverityHigh
	paused := domain.CaseWorkStatePaused

	cases := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{name: "state to work_in_progress", req: domain.UpdateCaseRequest{ID: testCaseID, State: &wip}},
		{name: "state to reopened", req: domain.UpdateCaseRequest{ID: testCaseID, State: &reopened}},
		{name: "severity change", req: domain.UpdateCaseRequest{ID: testCaseID, Severity: &high}},
		{name: "work-state change", req: domain.UpdateCaseRequest{ID: testCaseID, WorkState: &paused}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updateCalled := false
			repo := &stubCaseRepo{
				// countNonClosedChild left nil: it panics if the guard wrongly runs.
				updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, error) {
					updateCalled = true
					return domain.Case{ID: req.ID}, nil
				},
			}
			svc := NewCaseService(repo, stubUserRepo{})

			if _, err := svc.UpdateCase(ctx, tc.req); err != nil {
				t.Fatalf("expected update to succeed without the guard running, got: %v", err)
			}
			if !updateCalled {
				t.Fatal("expected repo.UpdateCase to be reached")
			}
		})
	}
}

// TestCaseService_UpdateCase_SetParent covers the native parent-linking write
// path: a lone parentId links or clears the parent, is validated, and is
// mutually exclusive with state/severity/workState. Repo access is stubbed.
func TestCaseService_UpdateCase_SetParent(t *testing.T) {
	ctx := context.Background()
	parentStr := func(v string) *string { return &v }

	t.Run("valid parent id links the case", func(t *testing.T) {
		var gotCase string
		var gotParent *string
		repo := &stubCaseRepo{
			updateCaseParent: func(_ context.Context, caseID string, parentID *string) (domain.Case, error) {
				gotCase, gotParent = caseID, parentID
				return domain.Case{ID: caseID, State: domain.CaseStateOpen}, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{})

		resp, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testCaseUUID, ParentID: parentStr(testParentCaseUUID)})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if gotCase != testCaseUUID {
			t.Fatalf("case id = %q, want %q", gotCase, testCaseUUID)
		}
		if gotParent == nil || *gotParent != testParentCaseUUID {
			t.Fatalf("parent id = %v, want %q", gotParent, testParentCaseUUID)
		}
		if resp.Case.ID != testCaseUUID {
			t.Fatalf("response case id = %q, want %q", resp.Case.ID, testCaseUUID)
		}
	})

	t.Run("empty parent id clears the link (nil to repo)", func(t *testing.T) {
		sawCall := false
		repo := &stubCaseRepo{
			updateCaseParent: func(_ context.Context, caseID string, parentID *string) (domain.Case, error) {
				sawCall = true
				if parentID != nil {
					t.Fatalf("expected nil parentID for a clear, got %q", *parentID)
				}
				return domain.Case{ID: caseID}, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{})

		if _, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testCaseUUID, ParentID: parentStr("")}); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if !sawCall {
			t.Fatal("expected repo.UpdateCaseParent to be called")
		}
	})

	t.Run("parentId cannot be combined with a state change", func(t *testing.T) {
		closed := domain.CaseStateClosed
		// updateCaseParent left nil: it must not be reached.
		repo := &stubCaseRepo{}
		svc := NewCaseService(repo, stubUserRepo{})

		_, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testCaseUUID, ParentID: parentStr(testParentCaseUUID), State: &closed})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
		if ve.Msg != "parentId cannot be combined with state, severity, or workState" {
			t.Fatalf("unexpected message: %q", ve.Msg)
		}
	})

	t.Run("a non-uuid parent id is rejected", func(t *testing.T) {
		repo := &stubCaseRepo{} // must not reach the repo
		svc := NewCaseService(repo, stubUserRepo{})

		_, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testCaseUUID, ParentID: parentStr("not-a-uuid")})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("a case cannot be its own parent", func(t *testing.T) {
		repo := &stubCaseRepo{} // must not reach the repo
		svc := NewCaseService(repo, stubUserRepo{})

		_, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testCaseUUID, ParentID: parentStr(testCaseUUID)})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
		if ve.Msg != "a case cannot be its own parent" {
			t.Fatalf("unexpected message: %q", ve.Msg)
		}
	})

	t.Run("a dangling parent reference surfaces the repo validation error", func(t *testing.T) {
		repo := &stubCaseRepo{
			updateCaseParent: func(_ context.Context, _ string, _ *string) (domain.Case, error) {
				return domain.Case{}, &apierror.ValidationError{Msg: "parent case does not exist"}
			},
		}
		svc := NewCaseService(repo, stubUserRepo{})

		_, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: testCaseUUID, ParentID: parentStr(testParentCaseUUID)})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
		if ve.Msg != "parent case does not exist" {
			t.Fatalf("unexpected message: %q", ve.Msg)
		}
	})
}

// TestCaseService_UpdateCase_ParentLinkWorkNote covers the ported "Add Work notes
// on Parent addition" rule: linking a parent posts a work note attributed to the
// acting user; the link still succeeds if the note can't be written; and clearing a
// parent (or linking without an identity) posts nothing.
func TestCaseService_UpdateCase_ParentLinkWorkNote(t *testing.T) {
	parentStr := func(v string) *string { return &v }
	actor := domain.User{ID: "aaaaaaaa-0000-0000-0000-000000000001", Email: "agent@wso2.com"}
	authedCtx := contextWithUserIDToken(fakeJWTWithEmail(t, "agent@wso2.com"))

	t.Run("posts a work note on link, attributed to the actor", func(t *testing.T) {
		var got domain.CreateCaseCommentRequest
		repo := &stubCaseRepo{
			updateCaseParent: func(_ context.Context, caseID string, _ *string) (domain.Case, error) {
				return domain.Case{ID: caseID, State: domain.CaseStateOpen}, nil
			},
			getCaseByID: func(_ context.Context, id string) (domain.CaseView, error) {
				return domain.CaseView{ID: id, Number: "WSO2-500"}, nil
			},
			createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest) (domain.CaseComment, error) {
				got = req
				return domain.CaseComment{ID: "comment-1"}, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) { return actor, nil }})

		if _, err := svc.UpdateCase(authedCtx, domain.UpdateCaseRequest{ID: testCaseUUID, ParentID: parentStr(testParentCaseUUID)}); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if got.Type != domain.CommentTypeWorkNote {
			t.Fatalf("note type = %q, want work_note", got.Type)
		}
		if got.CreatedBy != actor.ID {
			t.Fatalf("note author = %q, want actor %q", got.CreatedBy, actor.ID)
		}
		if got.CaseID != testCaseUUID {
			t.Fatalf("note caseId = %q, want %q", got.CaseID, testCaseUUID)
		}
		if got.Content == "" || !contains(got.Content, "WSO2-500") {
			t.Fatalf("note content should reference the parent number, got %q", got.Content)
		}
	})

	t.Run("link still succeeds when the note write fails", func(t *testing.T) {
		repo := &stubCaseRepo{
			updateCaseParent: func(_ context.Context, caseID string, _ *string) (domain.Case, error) {
				return domain.Case{ID: caseID}, nil
			},
			getCaseByID: func(_ context.Context, id string) (domain.CaseView, error) {
				return domain.CaseView{ID: id, Number: "WSO2-500"}, nil
			},
			createCaseComment: func(context.Context, domain.CreateCaseCommentRequest) (domain.CaseComment, error) {
				return domain.CaseComment{}, errCountChildren // any error
			},
		}
		svc := NewCaseService(repo, stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) { return actor, nil }})

		if _, err := svc.UpdateCase(authedCtx, domain.UpdateCaseRequest{ID: testCaseUUID, ParentID: parentStr(testParentCaseUUID)}); err != nil {
			t.Fatalf("a failed work note must not fail the link; got %v", err)
		}
	})

	t.Run("clearing a parent posts no note", func(t *testing.T) {
		// createCaseComment/getCaseByID left nil: they panic if the note path runs.
		repo := &stubCaseRepo{
			updateCaseParent: func(_ context.Context, caseID string, parentID *string) (domain.Case, error) {
				if parentID != nil {
					t.Fatalf("expected nil parent for clear")
				}
				return domain.Case{ID: caseID}, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{})
		if _, err := svc.UpdateCase(authedCtx, domain.UpdateCaseRequest{ID: testCaseUUID, ParentID: parentStr("")}); err != nil {
			t.Fatalf("expected clear to succeed, got %v", err)
		}
	})

	t.Run("no identity token: link succeeds, note skipped", func(t *testing.T) {
		// getCaseByID/createCaseComment nil; resolveActor fails first so they're never called.
		repo := &stubCaseRepo{
			updateCaseParent: func(_ context.Context, caseID string, _ *string) (domain.Case, error) {
				return domain.Case{ID: caseID}, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{})
		if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testCaseUUID, ParentID: parentStr(testParentCaseUUID)}); err != nil {
			t.Fatalf("expected success without a token, got %v", err)
		}
	})
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }
