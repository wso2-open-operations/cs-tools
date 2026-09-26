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
	"golang.org/x/sync/errgroup"
)

type globalService struct {
	repo   repository.ReferenceDataRepository
	search repository.GlobalSearchRepository
	access AccessService
}

// NewGlobalService constructs a Postgres-backed GlobalService. GetSystemMetadata
// is populated for real (project types come from the project_type table); see
// its own doc comment for the two fields left empty. GlobalSearch is scoped to
// what the caller may see -- see its own doc comment.
func NewGlobalService(repo repository.ReferenceDataRepository, search repository.GlobalSearchRepository, access AccessService) GlobalService {
	return &globalService{repo: repo, search: search, access: access}
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

// globalSearchSortFields maps the accepted sortBy.field values to the
// repository's sort keys. One field applies to both tables: "name" is a
// project's name / a case's subject.
var globalSearchSortFields = map[string]repository.SearchSortField{
	"name":      repository.SearchSortName,
	"createdOn": repository.SearchSortCreatedOn,
	"updatedOn": repository.SearchSortUpdatedOn,
}

// GlobalSearch implements GlobalService. It searches projects and cases the
// caller is allowed to see (AccessService.ResolveScope): a customer's results
// are limited to projects they are a registered contact of and the cases in
// them, while internal users and "internal" system clients see everything. A
// caller with no resolvable access gets an error, never an unscoped result.
//
// Defaults, when the request doesn't say: projects sort by name ascending and
// cases by updatedOn descending; an explicit sortBy.field applies to both, with
// a default order of ascending for "name" and descending for the date fields.
// projectsPagination/casesPagination default to the standard page size.
//
// activeChatsCount/actionRequiredCount/outstandingCount on each project are 0:
// see globalSearchRepo.SearchProjects.
func (s *globalService) GlobalSearch(ctx context.Context, req domain.GlobalSearchRequest) (domain.GlobalSearchResponse, error) {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.GlobalSearchResponse{}, err
	}

	var query string
	var tables []string
	if req.Filters != nil {
		query, tables = req.Filters.SearchQuery, req.Filters.Tables
		if err := validateSearchQuery(query); err != nil {
			return domain.GlobalSearchResponse{}, err
		}
		for _, t := range tables {
			if !validGlobalSearchTables[t] {
				return domain.GlobalSearchResponse{}, &apierror.ValidationError{Msg: "filters.tables contains invalid value: " + t}
			}
		}
	}

	sortField, desc, err := resolveGlobalSearchSort(req.SortBy)
	if err != nil {
		return domain.GlobalSearchResponse{}, err
	}

	projectsPage, casesPage := domain.Pagination{}, domain.Pagination{}
	if req.ProjectsPagination != nil {
		projectsPage = *req.ProjectsPagination
	}
	if req.CasesPagination != nil {
		casesPage = *req.CasesPagination
	}
	if err := normalizePagination(&projectsPage); err != nil {
		return domain.GlobalSearchResponse{}, err
	}
	if err := normalizePagination(&casesPage); err != nil {
		return domain.GlobalSearchResponse{}, err
	}

	wantProjects, wantCases := len(tables) == 0, len(tables) == 0
	for _, t := range tables {
		wantProjects = wantProjects || t == "projects"
		wantCases = wantCases || t == "cases"
	}

	resp := domain.GlobalSearchResponse{
		Query:    query,
		Projects: []domain.GlobalSearchProject{},
		Cases:    []domain.GlobalSearchCase{},
	}

	eg, egCtx := errgroup.WithContext(ctx)
	if wantProjects {
		eg.Go(func() error {
			field, d := projectSort(req.SortBy, sortField, desc)
			projects, total, err := s.search.SearchProjects(egCtx, scope, query, field, d, projectsPage)
			resp.Projects, resp.ProjectsTotal = projects, total
			return err
		})
	}
	if wantCases {
		eg.Go(func() error {
			field, d := caseSort(req.SortBy, sortField, desc)
			cases, total, err := s.search.SearchCases(egCtx, scope, query, field, d, casesPage)
			resp.Cases, resp.CasesTotal = cases, total
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return domain.GlobalSearchResponse{}, err
	}
	return resp, nil
}

// resolveGlobalSearchSort validates sortBy and returns the chosen field and
// direction. The field is empty when the caller named none, in which case each
// table falls back to its own default (see projectSort/caseSort).
func resolveGlobalSearchSort(sortBy *domain.GlobalSearchSort) (repository.SearchSortField, bool, error) {
	if sortBy == nil || (sortBy.Field == "" && sortBy.Order == "") {
		return "", false, nil
	}
	if sortBy.Order != "" && !validGlobalSearchSortOrder[sortBy.Order] {
		return "", false, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + sortBy.Order}
	}
	if sortBy.Field == "" {
		return "", sortBy.Order == "desc", nil
	}
	field, ok := globalSearchSortFields[sortBy.Field]
	if !ok {
		return "", false, &apierror.ValidationError{Msg: "sortBy.field must be one of: name, createdOn, updatedOn"}
	}
	desc := field != repository.SearchSortName
	if sortBy.Order != "" {
		desc = sortBy.Order == "desc"
	}
	return field, desc, nil
}

// projectSort/caseSort apply each table's default when the caller named no
// field, but still honour an explicit order on its own.
func projectSort(sortBy *domain.GlobalSearchSort, field repository.SearchSortField, desc bool) (repository.SearchSortField, bool) {
	if field != "" {
		return field, desc
	}
	return repository.SearchSortName, sortBy != nil && sortBy.Order == "desc"
}

func caseSort(sortBy *domain.GlobalSearchSort, field repository.SearchSortField, desc bool) (repository.SearchSortField, bool) {
	if field != "" {
		return field, desc
	}
	return repository.SearchSortUpdatedOn, sortBy == nil || sortBy.Order != "asc"
}
