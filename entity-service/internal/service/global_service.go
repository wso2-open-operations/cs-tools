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

type globalService struct {
	repo repository.ReferenceDataRepository
}

// NewGlobalService constructs a Postgres-backed GlobalService. GetSystemMetadata
// is populated for real (project types come from the project_type table); see
// its own doc comment for the two fields left empty. GlobalSearch has no
// Postgres implementation yet -- see its own doc comment.
func NewGlobalService(repo repository.ReferenceDataRepository) GlobalService {
	return &globalService{repo: repo}
}

// GetSystemMetadata implements GlobalService. TimeZones and FeedbackEmojis
// are left empty: both are static ServiceNow-side configuration (a curated
// time zone list, feedback emoji image assets), not project/case data backed
// by any Postgres table. TODO: populate once such a table exists.
func (s *globalService) GetSystemMetadata(ctx context.Context) (domain.SystemMetadataResponse, error) {
	projectTypes, err := s.repo.ListProjectTypes(ctx)
	if err != nil {
		return domain.SystemMetadataResponse{}, err
	}

	items := make([]domain.ReferenceTableItem, 0, len(projectTypes))
	for _, pt := range projectTypes {
		items = append(items, domain.ReferenceTableItem{ID: pt.ID, Name: pt.Name})
	}

	return domain.SystemMetadataResponse{ProjectTypes: items}, nil
}

// GlobalSearch implements GlobalService. Cross-entity project+case search is
// a materially larger feature than the reference-data reads GetSystemMetadata
// serves (it needs its own query/ranking design across two tables), so it is
// not implemented for the Postgres data source yet. TODO: implement once
// scoped.
func (s *globalService) GlobalSearch(ctx context.Context, req domain.GlobalSearchRequest) (domain.GlobalSearchResponse, error) {
	return domain.GlobalSearchResponse{}, &apierror.ValidationError{Msg: "global search (POST /search) is not yet implemented for the Postgres data source"}
}
