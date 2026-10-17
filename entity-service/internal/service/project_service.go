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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const errArrTodayGteUnsupported = "arrTodayGte is not supported on this data source yet: account ARR is not stored; remove the filter"

type projectService struct {
	repo   repository.ProjectRepository
	access AccessService
}

// NewProjectService constructs a ProjectService backed by the given
// repository, scoping every read through access (see AccessService).
func NewProjectService(repo repository.ProjectRepository, access AccessService) ProjectService {
	return &projectService{repo: repo, access: access}
}

// SearchProjects implements ProjectService.
func (s *projectService) SearchProjects(ctx context.Context, req domain.SearchProjectsRequest) (domain.SearchProjectsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchProjectsResponse{}, err
	}
	// Same rules as ServiceNow; the repository relies on them for its ORDER BY whitelist.
	if err := validateProjectSearchFilters(req); err != nil {
		return domain.SearchProjectsResponse{}, err
	}
	// No account ARR column yet: reject rather than return unfiltered results.
	if req.ArrTodayGte != "" {
		return domain.SearchProjectsResponse{}, &apierror.ValidationError{Msg: errArrTodayGteUnsupported}
	}
	if req.AccountID != "" {
		if err := validateUUIDs("accountId", []string{req.AccountID}); err != nil {
			return domain.SearchProjectsResponse{}, err
		}
	}
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.SearchProjectsResponse{}, err
	}

	projects, total, err := s.repo.SearchProjects(ctx, req, scope)
	if err != nil {
		return domain.SearchProjectsResponse{}, err
	}

	views := make([]domain.ProjectView, len(projects))
	for i, p := range projects {
		views[i] = domain.ProjectView{
			ID:                   p.ID,
			Name:                 p.Name,
			Key:                  p.Key,
			SfID:                 nilIfEmpty(&p.SfID),
			SubscriptionType:     p.SubscriptionType,
			StartDate:            p.StartDate,
			EndDate:              p.EndDate,
			CreatedOn:            p.CreatedOn,
			ActiveCasesCount:     p.ActiveCasesCount,
			Account:              p.Account,
			ProjectClosureFields: p.ProjectClosureFields,
			OnboardingStatus:     p.OnboardingStatus,
			OnboardingOwner:      p.OnboardingOwner,
			GoLiveDate:           p.GoLiveDate,
		}
	}

	return domain.SearchProjectsResponse{
		Projects: views,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(projects) < total,
	}, nil
}

// GetProjectByID implements ProjectService.
func (s *projectService) GetProjectByID(ctx context.Context, id string) (domain.ProjectDetailsView, error) {
	scope, err := resolveScopeForID(ctx, s.access, id)
	if err != nil {
		return domain.ProjectDetailsView{}, err
	}
	return s.repo.GetProjectByID(ctx, id, scope)
}

// GetProjectConsumptionStatus implements ProjectService.
func (s *projectService) GetProjectConsumptionStatus(ctx context.Context, id string) (domain.ProjectConsumptionStatus, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ProjectConsumptionStatus{}, err
	}
	return s.repo.GetProjectConsumptionStatus(ctx, id)
}

// UpdateProjectConsumption implements ProjectService.
func (s *projectService) UpdateProjectConsumption(ctx context.Context, id string, req domain.ProjectConsumptionUpdateRequest, updatedBy string) error {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return err
	}
	if updatedBy == "" {
		updatedBy = "choreo-subscription-service"
	}
	return s.repo.UpdateProjectConsumption(ctx, id, req, updatedBy)
}

