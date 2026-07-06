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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type productVersionService struct {
	repo repository.ProductVersionRepository
}

// NewProductVersionService constructs a ProductVersionService backed by the given repository.
func NewProductVersionService(repo repository.ProductVersionRepository) ProductVersionService {
	return &productVersionService{repo: repo}
}

// SearchProductVersions implements ProductVersionService.
func (s *productVersionService) SearchProductVersions(ctx context.Context, req domain.SearchProductVersionsRequest) (domain.SearchProductVersionsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProductVersionsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchProductVersionsResponse{}, err
	}

	versions, total, err := s.repo.SearchProductVersions(ctx, req)
	if err != nil {
		return domain.SearchProductVersionsResponse{}, err
	}

	return domain.SearchProductVersionsResponse{
		ProductVersions: versions,
		Total:           total,
		Limit:           req.Pagination.Limit,
		Offset:          req.Pagination.Offset,
		HasMore:         req.Pagination.Offset+len(versions) < total,
	}, nil
}
