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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubConversationRepo is a minimal repository.ConversationRepository whose
// unconfigured methods panic if called -- same convention as
// stubChangeRequestRepo.
type stubConversationRepo struct {
	searchConversations func(ctx context.Context, req domain.SearchConversationsRequest) ([]domain.SearchConversationView, int, error)
	getConversation     func(ctx context.Context, id string) (domain.ConversationDetails, error)
	updateConversation  func(ctx context.Context, id string, state domain.ConversationState) (domain.UpdatedConversation, error)
}

func (s *stubConversationRepo) SearchConversations(ctx context.Context, req domain.SearchConversationsRequest, _ string) ([]domain.SearchConversationView, int, error) {
	if s.searchConversations != nil {
		return s.searchConversations(ctx, req)
	}
	panic("SearchConversations called unexpectedly")
}
func (s *stubConversationRepo) GetConversation(ctx context.Context, id string) (domain.ConversationDetails, error) {
	if s.getConversation != nil {
		return s.getConversation(ctx, id)
	}
	panic("GetConversation called unexpectedly")
}
func (s *stubConversationRepo) UpdateConversation(ctx context.Context, id string, state domain.ConversationState, _ string) (domain.UpdatedConversation, error) {
	if s.updateConversation != nil {
		return s.updateConversation(ctx, id, state)
	}
	panic("UpdateConversation called unexpectedly")
}

// TestConversationService_SearchConversations_ScopesToCallerProjects is the
// core regression guard for the conversation authorization gap: before this
// fix, filters.projectIds was purely a caller-supplied, optional narrowing
// with nothing tying it to the caller's own AccessScope. Confirmed live
// against a real database copy: a real registered customer contact
// (anuradhab@wso2.com, registered to 4 projects) could see all 1,727
// conversations in the system this way, not just the 1,441 belonging to
// their own projects.
func TestConversationService_SearchConversations_ScopesToCallerProjects(t *testing.T) {
	tests := []struct {
		name            string
		scope           AccessScope
		requestedFilter []string
		wantRepoCalled  bool
		wantFilter      []string
	}{
		{
			name:            "external caller with no filter is narrowed to their own projects",
			scope:           AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},
			requestedFilter: nil,
			wantRepoCalled:  true,
			wantFilter:      []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
		},
		{
			name:            "external caller's own filter is intersected with scope, not unioned",
			scope:           AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},
			requestedFilter: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"},
			wantRepoCalled:  true,
			wantFilter:      []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
		},
		{
			name:            "external caller asking for an out-of-scope project alone gets zero results, not everyone's",
			scope:           AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},
			requestedFilter: []string{"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"},
			wantRepoCalled:  false,
		},
		{
			name:            "external caller with no registered projects at all gets zero results",
			scope:           AccessScope{ProjectIDs: nil},
			requestedFilter: nil,
			wantRepoCalled:  false,
		},
		{
			name:            "internal (unrestricted) caller's request passes through untouched",
			scope:           AccessScope{Unrestricted: true},
			requestedFilter: nil,
			wantRepoCalled:  true,
			wantFilter:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			var gotFilter []string
			repo := &stubConversationRepo{
				searchConversations: func(_ context.Context, req domain.SearchConversationsRequest) ([]domain.SearchConversationView, int, error) {
					called = true
					gotFilter = req.Filters.ProjectIDs
					return []domain.SearchConversationView{}, 0, nil
				},
			}
			svc := NewConversationService(repo, stubAccess{scope: tc.scope})

			_, err := svc.SearchConversations(context.Background(), domain.SearchConversationsRequest{
				Filters:    domain.SearchConversationsFilters{ProjectIDs: tc.requestedFilter},
				Pagination: domain.Pagination{Limit: 20},
			})
			if err != nil {
				t.Fatalf("SearchConversations: %v", err)
			}
			if called != tc.wantRepoCalled {
				t.Fatalf("repo called = %v, want %v", called, tc.wantRepoCalled)
			}
			if !called {
				return
			}
			if len(gotFilter) != len(tc.wantFilter) {
				t.Fatalf("repo received filters.projectIds = %v, want %v", gotFilter, tc.wantFilter)
			}
			for i := range gotFilter {
				if gotFilter[i] != tc.wantFilter[i] {
					t.Fatalf("repo received filters.projectIds = %v, want %v", gotFilter, tc.wantFilter)
				}
			}
		})
	}
}

// conversationDetailsWithProject builds a minimal domain.ConversationDetails
// with the given project id (nil if projectID == "").
func conversationDetailsWithProject(id, projectID string) domain.ConversationDetails {
	d := domain.ConversationDetails{ID: id}
	if projectID != "" {
		d.Project = &domain.EntityRef{ID: projectID}
	}
	return d
}

// TestConversationService_GetConversation_DeniesOutOfScopeProject is the
// by-id counterpart: today, GetConversation has no project check at all in
// its SQL (`WHERE wi.id = $1 AND wi.type = 'CONVERSATION'`), so any caller
// who knows/guesses a UUID can read a conversation from any project.
func TestConversationService_GetConversation_DeniesOutOfScopeProject(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	repo := &stubConversationRepo{
		getConversation: func(context.Context, string) (domain.ConversationDetails, error) {
			return conversationDetailsWithProject(id, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), nil
		},
	}

	t.Run("external caller outside the project gets NotFound, not the row", func(t *testing.T) {
		svc := NewConversationService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})
		_, err := svc.GetConversation(context.Background(), id)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("external caller inside the project succeeds", func(t *testing.T) {
		svc := NewConversationService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}}})
		d, err := svc.GetConversation(context.Background(), id)
		if err != nil {
			t.Fatalf("GetConversation: %v", err)
		}
		if d.ID != id {
			t.Errorf("d.ID = %q, want %q", d.ID, id)
		}
	})

	t.Run("internal caller succeeds regardless of project", func(t *testing.T) {
		svc := NewConversationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
		if _, err := svc.GetConversation(context.Background(), id); err != nil {
			t.Fatalf("GetConversation: %v", err)
		}
	})

	t.Run("a conversation with no project (deleted project) fails closed for a non-unrestricted caller", func(t *testing.T) {
		noProjectRepo := &stubConversationRepo{
			getConversation: func(context.Context, string) (domain.ConversationDetails, error) {
				return conversationDetailsWithProject(id, ""), nil
			},
		}
		svc := NewConversationService(noProjectRepo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})
		_, err := svc.GetConversation(context.Background(), id)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError for a project-less conversation, got %T: %v", err, err)
		}
	})
}

// TestConversationService_UpdateConversation_DeniesOutOfScopeProject checks
// the mutation path: the authorization check must happen BEFORE
// repo.UpdateConversation is ever called, not after.
func TestConversationService_UpdateConversation_DeniesOutOfScopeProject(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"

	t.Run("external caller outside the project is refused before any write", func(t *testing.T) {
		updateCalled := false
		repo := &stubConversationRepo{
			getConversation: func(context.Context, string) (domain.ConversationDetails, error) {
				return conversationDetailsWithProject(id, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), nil
			},
			updateConversation: func(context.Context, string, domain.ConversationState) (domain.UpdatedConversation, error) {
				updateCalled = true
				return domain.UpdatedConversation{}, nil
			},
		}
		svc := NewConversationService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

		_, err := svc.UpdateConversation(ctx, id, domain.UpdateConversationRequest{State: domain.ConversationStateClosed})
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
		if updateCalled {
			t.Error("repo.UpdateConversation must never be called for an out-of-scope conversation")
		}
	})

	t.Run("external caller inside the project succeeds", func(t *testing.T) {
		updateCalled := false
		repo := &stubConversationRepo{
			getConversation: func(context.Context, string) (domain.ConversationDetails, error) {
				return conversationDetailsWithProject(id, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), nil
			},
			updateConversation: func(context.Context, string, domain.ConversationState) (domain.UpdatedConversation, error) {
				updateCalled = true
				return domain.UpdatedConversation{ID: id}, nil
			},
		}
		svc := NewConversationService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

		if _, err := svc.UpdateConversation(ctx, id, domain.UpdateConversationRequest{State: domain.ConversationStateClosed}); err != nil {
			t.Fatalf("UpdateConversation: %v", err)
		}
		if !updateCalled {
			t.Error("repo.UpdateConversation should have been called for an in-scope conversation")
		}
	})

	t.Run("internal caller succeeds without a scope check", func(t *testing.T) {
		repo := &stubConversationRepo{
			updateConversation: func(context.Context, string, domain.ConversationState) (domain.UpdatedConversation, error) {
				return domain.UpdatedConversation{ID: id}, nil
			},
		}
		svc := NewConversationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

		if _, err := svc.UpdateConversation(ctx, id, domain.UpdateConversationRequest{State: domain.ConversationStateClosed}); err != nil {
			t.Fatalf("UpdateConversation: %v", err)
		}
	})
}
