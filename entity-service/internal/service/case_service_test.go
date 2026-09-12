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
	"fmt"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
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
func (s *stubCaseRepo) UpdateCase(context.Context, domain.UpdateCaseRequest) (domain.Case, domain.CaseSeverity, error) {
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

// stubUserRepo is a minimal repository.UserRepository; SearchCases doesn't
// exercise it beyond the createdBy-current-user path, which these tests don't
// use.
type stubUserRepo struct {
	getUserByEmail func(ctx context.Context, email string) (domain.User, error)
	getUsersByIDs  func(ctx context.Context, ids []string) ([]domain.User, error)
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
func (s stubUserRepo) GetUsersByIDs(ctx context.Context, ids []string) ([]domain.User, error) {
	if s.getUsersByIDs != nil {
		return s.getUsersByIDs(ctx, ids)
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
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil)
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
			svc := NewCaseService(repo, stubUserRepo{}, nil)
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
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil)
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
		svc := NewCaseService(repo, stubUserRepo{}, nil)

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
		svc := NewCaseService(repo, stubUserRepo{}, nil)

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
		svc := NewCaseService(repo, stubUserRepo{}, nil)

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
		svc := NewCaseService(repo, stubUserRepo{}, nil)

		_, err := svc.SearchCaseComments(context.Background(), domain.SearchCaseCommentsRequest{CaseID: "not-a-uuid"})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})
}

// TestCaseService_CreateCaseComment_PublishesCaseMentioned covers
// publishCaseMentioned, called from CreateCaseComment right after a
// successful comment insert. Exercises: mentioned ids resolved to emails via
// GetUsersByIDs and published as case.mentioned's Recipients; an id that
// doesn't resolve is silently dropped rather than failing the request; a
// work-note comment's recipients are filtered to wso2.com addresses only,
// and skipped entirely (no publish) when that filtering empties the list; no
// MentionedUserIDs at all means no publish and GetUsersByIDs is never
// called; and a nil publisher means CreateCaseComment still succeeds.
func TestCaseService_CreateCaseComment_PublishesCaseMentioned(t *testing.T) {
	caseID := "11111111-1111-1111-1111-111111111111"
	authorID := "22222222-2222-2222-2222-222222222222"
	mentionedID1 := "33333333-3333-3333-3333-333333333333"
	mentionedID2 := "44444444-4444-4444-4444-444444444444"
	commentID := "c1"
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	author := domain.User{ID: authorID, FirstName: "Jane", LastName: "Doe", Email: "jane.doe@example.com"}
	cv := domain.CaseView{
		ID:             caseID,
		Number:         "CS0009001",
		InternalID:     "WSO2-009",
		Subject:        "Something is broken",
		ProjectDetails: domain.EntityRef{ID: "p1", Name: "Project One"},
	}

	newRepo := func(t *testing.T, req domain.CreateCaseCommentRequest) *stubCaseRepo {
		return &stubCaseRepo{
			createCaseComment: func(_ context.Context, gotReq domain.CreateCaseCommentRequest) (domain.CaseComment, error) {
				if gotReq.CreatedBy != authorID {
					t.Fatalf("CreatedBy = %q, want %q", gotReq.CreatedBy, authorID)
				}
				return domain.CaseComment{ID: commentID, CaseID: caseID, CreatedOn: now}, nil
			},
			getCaseByID: func(_ context.Context, id string) (domain.CaseView, error) {
				if id != caseID {
					t.Fatalf("GetCaseByID id = %q, want %q", id, caseID)
				}
				return cv, nil
			},
		}
	}
	newUserRepo := func(getUsersByIDs func(ctx context.Context, ids []string) ([]domain.User, error)) stubUserRepo {
		return stubUserRepo{
			getUserByEmail: func(context.Context, string) (domain.User, error) { return author, nil },
			getUsersByIDs:  getUsersByIDs,
		}
	}

	t.Run("resolves mentioned ids and publishes case.mentioned", func(t *testing.T) {
		req := domain.CreateCaseCommentRequest{
			CaseID:           caseID,
			Type:             domain.CommentTypeComment,
			Content:          "hey @jane @unresolved check this out",
			MentionedUserIDs: []string{mentionedID1, mentionedID2},
		}
		repo := newRepo(t, req)
		var gotIDs []string
		userRepo := newUserRepo(func(_ context.Context, ids []string) ([]domain.User, error) {
			gotIDs = ids
			// mentionedID2 deliberately doesn't resolve, to prove an
			// unresolved id is dropped rather than failing the publish.
			return []domain.User{{ID: mentionedID1, Email: "mention1@example.com"}}, nil
		})
		publisher := &mockEventPublisher{}
		svc := NewCaseService(repo, userRepo, publisher)

		_, err := svc.CreateCaseComment(contextWithUserIDToken(fakeJWTWithEmail(t, author.Email)), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gotIDs) != 2 || gotIDs[0] != mentionedID1 || gotIDs[1] != mentionedID2 {
			t.Fatalf("GetUsersByIDs called with %v, want %v", gotIDs, req.MentionedUserIDs)
		}
		if len(publisher.calls) != 1 {
			t.Fatalf("Publish calls = %d, want 1", len(publisher.calls))
		}
		call := publisher.calls[0]
		if call.eventType != events.TypeCaseMentioned || call.entityID != caseID {
			t.Fatalf("Publish(type=%q, entityID=%q), want (%q, %q)", call.eventType, call.entityID, events.TypeCaseMentioned, caseID)
		}
		var payload events.CaseMentionedPayload
		if err := json.Unmarshal(call.payload, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if payload.MentionerName != "Jane Doe" {
			t.Errorf("MentionerName = %q, want %q", payload.MentionerName, "Jane Doe")
		}
		if payload.ProjectID != "p1" || payload.CaseNumber != "CS0009001" || payload.WSO2CaseID != "WSO2-009" || payload.CaseTitle != "Something is broken" {
			t.Errorf("case detail fields not populated from GetCaseByID: %+v", payload)
		}
		if payload.CommentID != commentID || payload.CaseComment != req.Content {
			t.Errorf("CommentID/CaseComment = %q/%q, want %q/%q", payload.CommentID, payload.CaseComment, commentID, req.Content)
		}
		if payload.IsInternalNote {
			t.Errorf("IsInternalNote = true, want false for a customer-visible comment")
		}
		if len(payload.Recipients) != 1 || payload.Recipients[0] != "mention1@example.com" {
			t.Errorf("Recipients = %v, want [mention1@example.com] (unresolved id dropped)", payload.Recipients)
		}
	})

	t.Run("work note filters recipients to wso2.com addresses only", func(t *testing.T) {
		req := domain.CreateCaseCommentRequest{
			CaseID:           caseID,
			Type:             domain.CommentTypeWorkNote,
			Content:          "internal only",
			MentionedUserIDs: []string{mentionedID1, mentionedID2},
		}
		repo := newRepo(t, req)
		userRepo := newUserRepo(func(context.Context, []string) ([]domain.User, error) {
			return []domain.User{
				{ID: mentionedID1, Email: "internal@wso2.com"},
				{ID: mentionedID2, Email: "customer@example.com"},
			}, nil
		})
		publisher := &mockEventPublisher{}
		svc := NewCaseService(repo, userRepo, publisher)

		if _, err := svc.CreateCaseComment(contextWithUserIDToken(fakeJWTWithEmail(t, author.Email)), req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(publisher.calls) != 1 {
			t.Fatalf("Publish calls = %d, want 1", len(publisher.calls))
		}
		var payload events.CaseMentionedPayload
		if err := json.Unmarshal(publisher.calls[0].payload, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if !payload.IsInternalNote {
			t.Errorf("IsInternalNote = false, want true for a work note")
		}
		if len(payload.Recipients) != 1 || payload.Recipients[0] != "internal@wso2.com" {
			t.Errorf("Recipients = %v, want [internal@wso2.com] (customer address filtered out)", payload.Recipients)
		}
	})

	t.Run("work note with no wso2.com recipients skips publishing entirely", func(t *testing.T) {
		req := domain.CreateCaseCommentRequest{
			CaseID:           caseID,
			Type:             domain.CommentTypeWorkNote,
			Content:          "internal only",
			MentionedUserIDs: []string{mentionedID1},
		}
		repo := newRepo(t, req)
		userRepo := newUserRepo(func(context.Context, []string) ([]domain.User, error) {
			return []domain.User{{ID: mentionedID1, Email: "customer@example.com"}}, nil
		})
		publisher := &mockEventPublisher{}
		svc := NewCaseService(repo, userRepo, publisher)

		if _, err := svc.CreateCaseComment(contextWithUserIDToken(fakeJWTWithEmail(t, author.Email)), req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(publisher.calls) != 0 {
			t.Fatalf("Publish calls = %d, want 0 (no wso2.com recipients left after filtering)", len(publisher.calls))
		}
	})

	t.Run("no mentions means no publish and GetUsersByIDs is never called", func(t *testing.T) {
		req := domain.CreateCaseCommentRequest{CaseID: caseID, Type: domain.CommentTypeComment, Content: "no mentions here"}
		repo := newRepo(t, req)
		userRepo := newUserRepo(func(context.Context, []string) ([]domain.User, error) {
			t.Fatal("GetUsersByIDs should not be called when MentionedUserIDs is empty")
			return nil, nil
		})
		publisher := &mockEventPublisher{}
		svc := NewCaseService(repo, userRepo, publisher)

		if _, err := svc.CreateCaseComment(contextWithUserIDToken(fakeJWTWithEmail(t, author.Email)), req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(publisher.calls) != 0 {
			t.Fatalf("Publish calls = %d, want 0", len(publisher.calls))
		}
	})

	t.Run("nil publisher does not fail comment creation", func(t *testing.T) {
		req := domain.CreateCaseCommentRequest{
			CaseID:           caseID,
			Type:             domain.CommentTypeComment,
			Content:          "hey @jane",
			MentionedUserIDs: []string{mentionedID1},
		}
		repo := newRepo(t, req)
		userRepo := newUserRepo(func(context.Context, []string) ([]domain.User, error) {
			t.Fatal("GetUsersByIDs should not be called when the publisher is nil")
			return nil, nil
		})
		svc := NewCaseService(repo, userRepo, nil)

		if _, err := svc.CreateCaseComment(contextWithUserIDToken(fakeJWTWithEmail(t, author.Email)), req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("mentioned-user resolution failure does not fail comment creation", func(t *testing.T) {
		req := domain.CreateCaseCommentRequest{
			CaseID:           caseID,
			Type:             domain.CommentTypeComment,
			Content:          "hey @jane",
			MentionedUserIDs: []string{mentionedID1},
		}
		repo := newRepo(t, req)
		userRepo := newUserRepo(func(context.Context, []string) ([]domain.User, error) {
			return nil, fmt.Errorf("db unavailable")
		})
		publisher := &mockEventPublisher{}
		svc := NewCaseService(repo, userRepo, publisher)

		if _, err := svc.CreateCaseComment(contextWithUserIDToken(fakeJWTWithEmail(t, author.Email)), req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(publisher.calls) != 0 {
			t.Fatalf("Publish calls = %d, want 0", len(publisher.calls))
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
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil)
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
