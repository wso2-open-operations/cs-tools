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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type taskSlaService struct {
	repo   repository.TaskSlaRepository
	access AccessService
}

// NewTaskSlaService constructs a TaskSlaService backed by Postgres.
func NewTaskSlaService(repo repository.TaskSlaRepository, access AccessService) TaskSlaService {
	return &taskSlaService{repo: repo, access: access}
}

// resolveTaskSlaProjectIDs resolves the caller's AccessScope into the
// projectIDs parameter TaskSlaRepository's methods take: nil for an
// unrestricted (internal) caller, or scope.ProjectIDs otherwise. ok is
// false when a non-unrestricted caller has zero registered projects --
// callers must short-circuit on that rather than pass an empty slice to
// the repo, which would be indistinguishable there from "no filter at
// all" (same class of bug already caught and fixed for change_request/
// conversation/escalation).
func (s *taskSlaService) resolveTaskSlaProjectIDs(ctx context.Context) (projectIDs []string, ok bool, err error) {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return nil, false, err
	}
	if scope.Unrestricted {
		return nil, true, nil
	}
	if len(scope.ProjectIDs) == 0 {
		return nil, false, nil
	}
	return scope.ProjectIDs, true, nil
}

// SearchTaskSlas implements TaskSlaService.
func (s *taskSlaService) SearchTaskSlas(ctx context.Context, req domain.SearchTaskSlasRequest) (domain.SearchTaskSlasResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchTaskSlasResponse{}, err
	}
	var taskIDs []string
	if req.Filters != nil {
		if err := validateUUIDs("filters.taskIds", req.Filters.TaskIDs); err != nil {
			return domain.SearchTaskSlasResponse{}, err
		}
		taskIDs = req.Filters.TaskIDs
	}

	// SearchTaskSlasFilters has no projectIds field for a caller to
	// (mis)supply -- scoping comes purely from the caller's own resolved
	// AccessScope. task_sla was found with no authorization at all before
	// this fix.
	projectIDs, ok, err := s.resolveTaskSlaProjectIDs(ctx)
	if err != nil {
		return domain.SearchTaskSlasResponse{}, err
	}
	if !ok {
		return domain.SearchTaskSlasResponse{
			TaskSlas: []domain.TaskSlaView{},
			Limit:    req.Pagination.Limit,
			Offset:   req.Pagination.Offset,
		}, nil
	}

	views, total, err := s.repo.SearchTaskSlas(ctx, taskIDs, projectIDs, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SearchTaskSlasResponse{}, err
	}

	return domain.SearchTaskSlasResponse{
		TaskSlas: views,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}

// GetTaskSla implements TaskSlaService.
func (s *taskSlaService) GetTaskSla(ctx context.Context, id string) (domain.TaskSlaDetail, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.TaskSlaDetail{}, err
	}
	projectIDs, ok, err := s.resolveTaskSlaProjectIDs(ctx)
	if err != nil {
		return domain.TaskSlaDetail{}, err
	}
	if !ok {
		return domain.TaskSlaDetail{}, &apierror.NotFoundError{Msg: "task sla not found"}
	}
	return s.repo.GetTaskSla(ctx, id, projectIDs)
}
