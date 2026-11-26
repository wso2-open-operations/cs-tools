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
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// ProjectAggregateService counts projects per grouping value
// (POST /projects/aggregate). Postgres data sources only: the route is not
// registered otherwise.
type ProjectAggregateService interface {
	AggregateProjects(ctx context.Context, req domain.AggregateProjectsRequest) (domain.AggregateResponse, error)
}

// defaultProjectAggregateMaxGroups and maxProjectAggregateGroups bound
// AggregateProjectsRequest.MaxGroups.
const (
	defaultProjectAggregateMaxGroups = 10
	maxProjectAggregateGroups        = 50
)

var validProjectGroupBy = map[string]bool{
	domain.ProjectGroupByOnboardingOwner:  true,
	domain.ProjectGroupByOnboardingStatus: true,
	domain.ProjectGroupByRegion:           true,
}

type projectAggregateService struct {
	repo   repository.ProjectAggregateRepository
	access AccessService
}

// NewProjectAggregateService constructs a ProjectAggregateService.
func NewProjectAggregateService(repo repository.ProjectAggregateRepository, access AccessService) ProjectAggregateService {
	return &projectAggregateService{repo: repo, access: access}
}

// AggregateProjects implements ProjectAggregateService. The filters are
// checked exactly as POST /projects/search checks them, and the result is
// narrowed to the caller's scope the same way, so a bucket never counts a
// project the caller could not list.
func (s *projectAggregateService) AggregateProjects(ctx context.Context, req domain.AggregateProjectsRequest) (domain.AggregateResponse, error) {
	if req.GroupBy == "" {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy is required"}
	}
	if !validProjectGroupBy[req.GroupBy] {
		return domain.AggregateResponse{}, apierror.InvalidValue("groupBy", req.GroupBy, "grouping",
			[]string{domain.ProjectGroupByOnboardingOwner, domain.ProjectGroupByOnboardingStatus, domain.ProjectGroupByRegion})
	}
	if req.MaxGroups < 0 || req.MaxGroups > maxProjectAggregateGroups {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("maxGroups must be between 0 and %d", maxProjectAggregateGroups)}
	}
	f := req.Filters
	if err := validateSearchQuery(f.SearchQuery); err != nil {
		return domain.AggregateResponse{}, err
	}
	if err := validateProjectSearchFilters(f); err != nil {
		return domain.AggregateResponse{}, err
	}
	if f.ArrTodayGte != "" {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: errArrTodayGteUnsupported}
	}
	if f.AccountID != "" {
		if err := validateUUIDs("accountId", []string{f.AccountID}); err != nil {
			return domain.AggregateResponse{}, err
		}
	}
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.AggregateResponse{}, err
	}
	buckets, keyless, err := s.repo.AggregateProjects(ctx, req, scope)
	if err != nil {
		return domain.AggregateResponse{}, err
	}
	maxGroups := req.MaxGroups
	if maxGroups == 0 {
		maxGroups = defaultProjectAggregateMaxGroups
	}
	resp := domain.AggregateResponse{Groups: buckets, OthersCount: keyless}
	if resp.Groups == nil {
		resp.Groups = []domain.AggregateBucket{}
	}
	if maxGroups < len(buckets) {
		resp.Groups = buckets[:maxGroups]
		for _, b := range buckets[maxGroups:] {
			resp.OthersCount += b.Count
		}
	}
	resp.TotalRecords = resp.OthersCount
	for _, b := range resp.Groups {
		resp.TotalRecords += b.Count
	}
	return resp, nil
}
