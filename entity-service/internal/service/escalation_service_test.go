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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubEscalationRepo is a minimal repository.EscalationRepository whose
// unconfigured SearchEscalations panics if called -- same convention as
// stubIncidentRepo.
type stubEscalationRepo struct {
	searchEscalations func(ctx context.Context, caseIDs []string, currentLevels []int, projectIDs []string) ([]domain.Escalation, int, error)
}

func (s *stubEscalationRepo) SearchEscalations(ctx context.Context, caseIDs []string, currentLevels []int, projectIDs []string, _, _ string, _, _ int) ([]domain.Escalation, int, error) {
	if s.searchEscalations != nil {
		return s.searchEscalations(ctx, caseIDs, currentLevels, projectIDs)
	}
	panic("SearchEscalations called unexpectedly")
}

// TestEscalationService_SearchEscalations_ScopesToCallerProjects is the core
// regression guard for the escalation authorization gap: SearchEscalations
// applied no authorization at all before this fix -- caseIDs was the only
// caller-supplied filter, and omitting it (WHERE 1=1) returned every
// escalation on every case in the system. Unlike change_request/conversation,
// SearchEscalationsFilters has no projectIds field for a caller to narrow
// with at all -- scoping here comes purely from the caller's own resolved
// AccessScope.
func TestEscalationService_SearchEscalations_ScopesToCallerProjects(t *testing.T) {
	t.Run("external caller's own resolved projects are passed to the repo", func(t *testing.T) {
		called := false
		var gotProjectIDs []string
		repo := &stubEscalationRepo{
			searchEscalations: func(_ context.Context, _ []string, _ []int, projectIDs []string) ([]domain.Escalation, int, error) {
				called = true
				gotProjectIDs = projectIDs
				return []domain.Escalation{}, 0, nil
			},
		}
		svc := NewEscalationService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})

		_, err := svc.SearchEscalations(context.Background(), domain.SearchEscalationsRequest{Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchEscalations: %v", err)
		}
		if !called {
			t.Fatal("expected repo.SearchEscalations to be called")
		}
		if len(gotProjectIDs) != 1 || gotProjectIDs[0] != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
			t.Fatalf("repo received projectIDs = %v, want [aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa]", gotProjectIDs)
		}
	})

	t.Run("external caller with no registered projects at all gets zero results, repo never called", func(t *testing.T) {
		repo := &stubEscalationRepo{}
		svc := NewEscalationService(repo, stubAccess{scope: AccessScope{ProjectIDs: nil}})

		resp, err := svc.SearchEscalations(context.Background(), domain.SearchEscalationsRequest{Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchEscalations: %v", err)
		}
		if len(resp.Escalations) != 0 {
			t.Errorf("resp.Escalations = %v, want empty", resp.Escalations)
		}
	})

	t.Run("internal (unrestricted) caller passes no project filter, sees everything", func(t *testing.T) {
		called := false
		var gotProjectIDs []string
		repo := &stubEscalationRepo{
			searchEscalations: func(_ context.Context, _ []string, _ []int, projectIDs []string) ([]domain.Escalation, int, error) {
				called = true
				gotProjectIDs = projectIDs
				return []domain.Escalation{}, 0, nil
			},
		}
		svc := NewEscalationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})

		_, err := svc.SearchEscalations(context.Background(), domain.SearchEscalationsRequest{Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchEscalations: %v", err)
		}
		if !called {
			t.Fatal("expected repo.SearchEscalations to be called")
		}
		if gotProjectIDs != nil {
			t.Errorf("repo received projectIDs = %v, want nil (no filter) for an unrestricted caller", gotProjectIDs)
		}
	})
}
