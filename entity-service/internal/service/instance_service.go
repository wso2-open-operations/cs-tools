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

// instanceDataSourceEnum maps InstanceStatsFilters.DataSource's integer
// discriminator (1 = API Call, 2 = File Upload, per that field's own doc
// comment) to daily_usage_summary.data_source's real enum labels.
var instanceDataSourceEnum = map[int]string{
	1: "API_CALL",
	2: "FILE_UPLOAD",
}

type instanceService struct {
	repo repository.InstanceRepository
}

// NewInstanceService constructs an InstanceService backed by Postgres.
func NewInstanceService(repo repository.InstanceRepository) InstanceService {
	return &instanceService{repo: repo}
}

func validateInstanceIDFilters(projectIDs, deploymentIDs, deployedProductIDs []string) error {
	if err := validateUUIDs("projectIds", projectIDs); err != nil {
		return err
	}
	if err := validateUUIDs("deploymentIds", deploymentIDs); err != nil {
		return err
	}
	if err := validateUUIDs("deployedProductIds", deployedProductIDs); err != nil {
		return err
	}
	return validateExclusiveIDFilters(projectIDs, deploymentIDs, deployedProductIDs)
}

// SearchInstances implements InstanceService.
func (s *instanceService) SearchInstances(ctx context.Context, req domain.SearchInstancesRequest) (domain.SearchInstancesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchInstancesResponse{}, err
	}
	if req.Filters != nil {
		if err := validateInstanceIDFilters(req.Filters.ProjectIDs, req.Filters.DeploymentIDs, req.Filters.DeployedProductIDs); err != nil {
			return domain.SearchInstancesResponse{}, err
		}
		// StartDate/EndDate are optional here (unlike every other instance
		// endpoint, where InstanceDateRangeFilters makes them required and
		// validateDateRange already checks them) -- decodeRequest only
		// confirms the JSON shape, so a malformed value would otherwise
		// reach instanceRepo.SearchInstances' own ::date cast unchecked.
		if req.Filters.StartDate != nil {
			if err := validateDateOnly("startDate", *req.Filters.StartDate); err != nil {
				return domain.SearchInstancesResponse{}, err
			}
		}
		if req.Filters.EndDate != nil {
			if err := validateDateOnly("endDate", *req.Filters.EndDate); err != nil {
				return domain.SearchInstancesResponse{}, err
			}
		}
		if req.Filters.StartDate != nil && req.Filters.EndDate != nil && *req.Filters.StartDate > *req.Filters.EndDate {
			return domain.SearchInstancesResponse{}, &apierror.ValidationError{Msg: "endDate must not be before startDate"}
		}
	}

	instances, total, err := s.repo.SearchInstances(ctx, req)
	if err != nil {
		return domain.SearchInstancesResponse{}, err
	}

	return domain.SearchInstancesResponse{
		Instances: instances,
		Total:     total,
		Limit:     req.Pagination.Limit,
		Offset:    req.Pagination.Offset,
	}, nil
}

func validateInstanceDateRangeFiltersPostgres(f domain.InstanceDateRangeFilters) error {
	if err := validateDateRange(f.StartDate, f.EndDate); err != nil {
		return err
	}
	return validateInstanceIDFilters(f.ProjectIDs, f.DeploymentIDs, f.DeployedProductIDs)
}

// SearchInstanceMetrics implements InstanceService.
func (s *instanceService) SearchInstanceMetrics(ctx context.Context, req domain.InstanceMetricsRequest) (domain.InstanceMetricsResponse, error) {
	if err := validateInstanceDateRangeFiltersPostgres(req.Filters); err != nil {
		return domain.InstanceMetricsResponse{}, err
	}

	metrics, total, err := s.repo.SearchInstanceMetrics(ctx, req.Filters)
	if err != nil {
		return domain.InstanceMetricsResponse{}, err
	}

	return domain.InstanceMetricsResponse{
		Metrics:        metrics,
		TotalInstances: total,
		StartDate:      req.Filters.StartDate,
		EndDate:        req.Filters.EndDate,
	}, nil
}

// SearchInstanceUsage implements InstanceService.
func (s *instanceService) SearchInstanceUsage(ctx context.Context, req domain.InstanceUsageRequest) (domain.InstanceUsageResponse, error) {
	if err := validateInstanceDateRangeFiltersPostgres(req.Filters); err != nil {
		return domain.InstanceUsageResponse{}, err
	}

	usages, total, err := s.repo.SearchInstanceUsage(ctx, req.Filters)
	if err != nil {
		return domain.InstanceUsageResponse{}, err
	}

	return domain.InstanceUsageResponse{
		Usages:         usages,
		TotalInstances: total,
		StartDate:      req.Filters.StartDate,
		EndDate:        req.Filters.EndDate,
	}, nil
}

// SearchInstanceMetricsStats implements InstanceService.
//
// deployment_information (the only source of the CoreCount/JDK/metadata this
// endpoint reports on) has no data-source column at all -- only
// daily_usage_summary does, which backs SearchInstanceUsageStats instead. A
// DataSource filter is therefore rejected outright rather than silently
// ignored, same convention as SearchDeployedProducts' ProductCategories
// rejection (deployed_product_service.go).
func (s *instanceService) SearchInstanceMetricsStats(ctx context.Context, req domain.InstanceMetricsStatsRequest) (domain.InstanceMetricsStatsResponse, error) {
	if err := validateInstanceDateRangeFiltersPostgres(req.Filters.InstanceDateRangeFilters); err != nil {
		return domain.InstanceMetricsStatsResponse{}, err
	}
	if req.Filters.DataSource != nil {
		return domain.InstanceMetricsStatsResponse{}, &apierror.ValidationError{
			Msg: "dataSource filtering is not supported for instance metrics stats on the PostgreSQL data source",
		}
	}

	return s.repo.SearchInstanceMetricsStats(ctx, req.Filters.InstanceDateRangeFilters)
}

// SearchInstanceUsageStats implements InstanceService.
func (s *instanceService) SearchInstanceUsageStats(ctx context.Context, req domain.InstanceUsageStatsRequest) (domain.InstanceUsageStatsResponse, error) {
	if err := validateInstanceDateRangeFiltersPostgres(req.Filters.InstanceDateRangeFilters); err != nil {
		return domain.InstanceUsageStatsResponse{}, err
	}

	var dataSource *string
	if req.Filters.DataSource != nil {
		enumValue, ok := instanceDataSourceEnum[*req.Filters.DataSource]
		if !ok {
			return domain.InstanceUsageStatsResponse{}, &apierror.ValidationError{Msg: "dataSource must be 1 (API Call) or 2 (File Upload)"}
		}
		dataSource = &enumValue
	}

	return s.repo.SearchInstanceUsageStats(ctx, req.Filters.InstanceDateRangeFilters, dataSource)
}
