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
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type projectService struct {
	repo repository.ProjectRepository
}

// NewProjectService constructs a ProjectService backed by the given repository.
func NewProjectService(repo repository.ProjectRepository) ProjectService {
	return &projectService{repo: repo}
}

// SearchProjects implements ProjectService.
func (s *projectService) SearchProjects(ctx context.Context, req domain.SearchProjectsRequest) (domain.SearchProjectsResponse, error) {
	if req.Pagination.Limit <= 0 {
		req.Pagination.Limit = defaultLimit
	}
	if req.Pagination.Limit > maxLimit {
		return domain.SearchProjectsResponse{}, &apierror.ValidationError{Msg: "limit cannot exceed 100"}
	}
	if req.Pagination.Offset < 0 {
		req.Pagination.Offset = 0
	}
	if utf8.RuneCountInString(req.SearchQuery) > maxSearchQueryLen {
		return domain.SearchProjectsResponse{}, &apierror.ValidationError{Msg: "searchQuery cannot exceed 200 characters"}
	}

	projects, total, err := s.repo.SearchProjects(ctx, req)
	if err != nil {
		return domain.SearchProjectsResponse{}, err
	}

	return domain.SearchProjectsResponse{
		Projects: projects,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(projects) < total,
	}, nil
}
