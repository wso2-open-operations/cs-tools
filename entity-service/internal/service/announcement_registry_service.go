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
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// maxAnnouncementRegistryRows bounds one registry read. It is the same bound
// the registry had when it paged through /cases/search (200 pages of 50), so
// a result that used to fail loudly still does, and nothing is silently
// truncated.
const maxAnnouncementRegistryRows = 10000

// AnnouncementRegistryService serves the CSM announcement registry's one-shot
// read of every announcement case matching a search.
type AnnouncementRegistryService interface {
	// SearchRegistryCases returns every announcement case matching req's
	// filters (states, projects, free-text search), newest-updated first.
	// Pagination and sort in req are ignored: the registry groups the whole
	// set before it paginates. Internal callers only.
	SearchRegistryCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error)

	// SearchRegistryRows returns one page of the grouped registry (one row per
	// published announcement request, plus one per case no published request
	// owns), newest first. Grouping, ordering and paging run in the database,
	// so there is no row cap. It supersedes SearchRegistryCases for the
	// registry list. req.Pagination applies to the grouped rows. Internal
	// callers only.
	SearchRegistryRows(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchAnnouncementRegistryRowsResponse, error)
}

type announcementRegistryService struct {
	repo   repository.AnnouncementRegistryRepository
	access AccessService
}

// NewAnnouncementRegistryService constructs an AnnouncementRegistryService.
func NewAnnouncementRegistryService(repo repository.AnnouncementRegistryRepository, access AccessService) AnnouncementRegistryService {
	return &announcementRegistryService{repo: repo, access: access}
}

// prepareRegistryRequest is the validation shared by both registry reads:
// internal callers only, no anyOf, type forced to announcement, then the same
// filter parsing as /cases/search. It returns the prepared request and the
// caller's scope.
func (s *announcementRegistryService) prepareRegistryRequest(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesRequest, repository.SearchScope, error) {
	if err := RequireInternalCaller(ctx, s.access, "the announcement registry is only available to internal callers"); err != nil {
		return domain.SearchCasesRequest{}, repository.SearchScope{}, err
	}
	if len(req.Filters.AnyOf) > 0 {
		return domain.SearchCasesRequest{}, repository.SearchScope{}, &apierror.ValidationError{Msg: "anyOf is not supported by the announcement registry search"}
	}

	// This endpoint is announcements only, whatever the caller sent for type.
	filters := make([]domain.CaseFieldFilter, 0, len(req.Filters.Filters)+1)
	for _, f := range req.Filters.Filters {
		if strings.EqualFold(f.Field, "type") {
			continue
		}
		filters = append(filters, f)
	}
	filters = append(filters, domain.CaseFieldFilter{Field: "type", Op: "in", Values: []string{"announcement"}})
	req.Filters.Filters = filters

	// Same validation and filter parsing as /cases/search, so every filter
	// means exactly what it means there.
	prepared, err := prepareCaseSearchFilters(ctx, req)
	if err != nil {
		return domain.SearchCasesRequest{}, repository.SearchScope{}, err
	}
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.SearchCasesRequest{}, repository.SearchScope{}, err
	}

	return prepared, scope, nil
}

// SearchRegistryCases implements AnnouncementRegistryService.
func (s *announcementRegistryService) SearchRegistryCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error) {
	prepared, scope, err := s.prepareRegistryRequest(ctx, req)
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}

	cases, err := s.repo.SearchAnnouncementCases(ctx, prepared, scope, maxAnnouncementRegistryRows)
	if err != nil {
		if errors.Is(err, repository.ErrTooManyRegistryRows) {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{
				Msg: fmt.Sprintf("more than %d announcements match; narrow the search", maxAnnouncementRegistryRows),
			}
		}
		return domain.SearchCasesResponse{}, err
	}
	return domain.SearchCasesResponse{Cases: cases, Total: len(cases), Limit: len(cases), Offset: 0}, nil
}

// SearchRegistryRows implements AnnouncementRegistryService.
func (s *announcementRegistryService) SearchRegistryRows(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchAnnouncementRegistryRowsResponse, error) {
	prepared, scope, err := s.prepareRegistryRequest(ctx, req)
	if err != nil {
		return domain.SearchAnnouncementRegistryRowsResponse{}, err
	}
	// Pagination applies to the grouped rows: default 20 per page, at most 50.
	if err := normalizePagination(&prepared.Pagination); err != nil {
		return domain.SearchAnnouncementRegistryRowsResponse{}, err
	}
	return s.repo.SearchAnnouncementRegistryRows(ctx, prepared, scope)
}
