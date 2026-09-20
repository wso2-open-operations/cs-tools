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

type groupService struct {
	repo repository.GroupRepository
}

// NewGroupService constructs a GroupService backed by Postgres.
func NewGroupService(repo repository.GroupRepository) GroupService {
	return &groupService{repo: repo}
}

// SearchGroups implements GroupService.
func (s *groupService) SearchGroups(ctx context.Context, req domain.SearchGroupsRequest) (domain.SearchGroupsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchGroupsResponse{}, err
	}
	searchQuery := ""
	if req.Filters != nil {
		if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
			return domain.SearchGroupsResponse{}, err
		}
		searchQuery = req.Filters.SearchQuery
	}

	groups, total, err := s.repo.SearchGroups(ctx, searchQuery, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SearchGroupsResponse{}, err
	}

	return domain.SearchGroupsResponse{
		Groups: groups,
		Total:  total,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}
