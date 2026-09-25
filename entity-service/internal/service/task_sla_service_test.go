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

// stubTaskSlaRepo is a minimal repository.TaskSlaRepository whose
// unconfigured methods panic if called -- same convention as
// stubEscalationRepo.
type stubTaskSlaRepo struct {
	searchTaskSlas func(ctx context.Context, taskIDs, projectIDs []string) ([]domain.TaskSlaView, int, error)
	getTaskSla     func(ctx context.Context, id string, projectIDs []string) (domain.TaskSlaDetail, error)
}

func (s *stubTaskSlaRepo) SearchTaskSlas(ctx context.Context, taskIDs, projectIDs []string, _, _ int) ([]domain.TaskSlaView, int, error) {
	if s.searchTaskSlas != nil {
		return s.searchTaskSlas(ctx, taskIDs, projectIDs)
	}
	panic("SearchTaskSlas called unexpectedly")
}
func (s *stubTaskSlaRepo) GetTaskSla(ctx context.Context, id string, projectIDs []string) (domain.TaskSlaDetail, error) {
	if s.getTaskSla != nil {
		return s.getTaskSla(ctx, id, projectIDs)
	}
	panic("GetTaskSla called unexpectedly")
}

// TestTaskSlaService_SearchTaskSlas_ScopesToCallerProjects is the core
// regression guard for the task_sla authorization gap: SearchTaskSlas
// applied no authorization at all before this fix. Confirmed live against
// a real database copy: the sla table's work_item_id spans both
// project-bearing types (CASE, SERVICE_REQUEST, ...) and project-less ones
// (INCIDENT, 114,759 of 127,558 real sla rows) -- resolving to
// wi.project_id = ANY(scope.ProjectIDs) for an external caller naturally
// excludes the project-less rows via NULL semantics, with no separate
// work_item-type branch needed.
func TestTaskSlaService_SearchTaskSlas_ScopesToCallerProjects(t *testing.T) {
	t.Run("external caller's own resolved projects are passed to the repo", func(t *testing.T) {
		called := false
		var gotProjectIDs []string
		repo := &stubTaskSlaRepo{
			searchTaskSlas: func(_ context.Context, _, projectIDs []string) ([]domain.TaskSlaView, int, error) {
				called = true
				gotProjectIDs = projectIDs
				return []domain.TaskSlaView{}, 0, nil
			},
		}
		svc := NewTaskSlaService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})

		_, err := svc.SearchTaskSlas(context.Background(), domain.SearchTaskSlasRequest{Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchTaskSlas: %v", err)
		}
		if !called {
			t.Fatal("expected repo.SearchTaskSlas to be called")
		}
		if len(gotProjectIDs) != 1 || gotProjectIDs[0] != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
			t.Fatalf("repo received projectIDs = %v, want [aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa]", gotProjectIDs)
		}
	})

	t.Run("external caller with no registered projects at all gets zero results, repo never called", func(t *testing.T) {
		repo := &stubTaskSlaRepo{}
		svc := NewTaskSlaService(repo, stubAccess{scope: AccessScope{ProjectIDs: nil}})

		resp, err := svc.SearchTaskSlas(context.Background(), domain.SearchTaskSlasRequest{Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchTaskSlas: %v", err)
		}
		if len(resp.TaskSlas) != 0 {
			t.Errorf("resp.TaskSlas = %v, want empty", resp.TaskSlas)
		}
	})

	t.Run("internal (unrestricted) caller passes no project filter", func(t *testing.T) {
		var gotProjectIDs []string
		called := false
		repo := &stubTaskSlaRepo{
			searchTaskSlas: func(_ context.Context, _, projectIDs []string) ([]domain.TaskSlaView, int, error) {
				called = true
				gotProjectIDs = projectIDs
				return []domain.TaskSlaView{}, 0, nil
			},
		}
		svc := NewTaskSlaService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})

		_, err := svc.SearchTaskSlas(context.Background(), domain.SearchTaskSlasRequest{Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchTaskSlas: %v", err)
		}
		if !called || gotProjectIDs != nil {
			t.Errorf("called=%v gotProjectIDs=%v, want called=true and nil", called, gotProjectIDs)
		}
	})
}

// TestTaskSlaService_GetTaskSla_ScopesToCallerProjects is the by-id
// counterpart: GetTaskSla applied no project check at all before this fix.
func TestTaskSlaService_GetTaskSla_ScopesToCallerProjects(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"

	t.Run("external caller's own resolved projects are passed to the repo", func(t *testing.T) {
		var gotProjectIDs []string
		repo := &stubTaskSlaRepo{
			getTaskSla: func(_ context.Context, _ string, projectIDs []string) (domain.TaskSlaDetail, error) {
				gotProjectIDs = projectIDs
				return domain.TaskSlaDetail{ID: id}, nil
			},
		}
		svc := NewTaskSlaService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})

		d, err := svc.GetTaskSla(context.Background(), id)
		if err != nil {
			t.Fatalf("GetTaskSla: %v", err)
		}
		if d.ID != id {
			t.Errorf("d.ID = %q, want %q", d.ID, id)
		}
		if len(gotProjectIDs) != 1 || gotProjectIDs[0] != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
			t.Fatalf("repo received projectIDs = %v, want [aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa]", gotProjectIDs)
		}
	})

	t.Run("external caller with no registered projects at all gets NotFound, repo never called", func(t *testing.T) {
		repo := &stubTaskSlaRepo{}
		svc := NewTaskSlaService(repo, stubAccess{scope: AccessScope{ProjectIDs: nil}})

		_, err := svc.GetTaskSla(context.Background(), id)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("internal caller passes no project filter", func(t *testing.T) {
		var gotProjectIDs []string
		called := false
		repo := &stubTaskSlaRepo{
			getTaskSla: func(_ context.Context, _ string, projectIDs []string) (domain.TaskSlaDetail, error) {
				called = true
				gotProjectIDs = projectIDs
				return domain.TaskSlaDetail{ID: id}, nil
			},
		}
		svc := NewTaskSlaService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})

		if _, err := svc.GetTaskSla(context.Background(), id); err != nil {
			t.Fatalf("GetTaskSla: %v", err)
		}
		if !called || gotProjectIDs != nil {
			t.Errorf("called=%v gotProjectIDs=%v, want called=true and nil", called, gotProjectIDs)
		}
	})
}
