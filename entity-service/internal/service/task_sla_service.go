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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type taskSlaService struct {
	repo repository.TaskSlaRepository
}

// NewTaskSlaService constructs a TaskSlaService backed by Postgres.
func NewTaskSlaService(repo repository.TaskSlaRepository) TaskSlaService {
	return &taskSlaService{repo: repo}
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

	views, total, err := s.repo.SearchTaskSlas(ctx, taskIDs, req.Pagination.Limit, req.Pagination.Offset)
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
	return s.repo.GetTaskSla(ctx, id)
}
