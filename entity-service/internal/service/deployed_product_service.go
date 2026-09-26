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

type deployedProductService struct {
	repo repository.DeployedProductRepository
}

// NewDeployedProductService constructs a DeployedProductService backed by the given repository.
func NewDeployedProductService(repo repository.DeployedProductRepository) DeployedProductService {
	return &deployedProductService{repo: repo}
}

// SearchDeployedProducts implements DeployedProductService.
func (s *deployedProductService) SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) (domain.SearchDeployedProductsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}
	if err := validateUUIDs("deploymentIds", req.DeploymentIDs); err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}
	// The PostgreSQL-backed deployed_products schema has no category column yet
	// (see the repository's TODO(phase 2)), so a category filter can't be honored here.
	// Reject it explicitly rather than silently ignoring it and returning products
	// outside the requested category.
	if len(req.ProductCategories) > 0 {
		return domain.SearchDeployedProductsResponse{},
			&apierror.ValidationError{Msg: "productCategories filtering is not supported for the PostgreSQL data source"}
	}

	views, total, err := s.repo.SearchDeployedProducts(ctx, req)
	if err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}

	return domain.SearchDeployedProductsResponse{
		DeployedProducts: views,
		Total:            total,
		Limit:            req.Pagination.Limit,
		Offset:           req.Pagination.Offset,
		HasMore:          req.Pagination.Offset+len(views) < total,
	}, nil
}

// SearchProjectsByProductVersion implements DeployedProductService.
// deployed_product.project_id (migration 000014) is a direct FK to project,
// so unlike the ServiceNow implementation this doesn't need to page through
// deployments platform-wide to resolve the join -- the repository does it
// in one query. The same mandatoryExcludeClosureStates/
// mandatoryExcludeSubscriptionTypes policy defined alongside the ServiceNow
// implementation (sn_deployed_product_service.go) is applied here too, so
// the "can't be turned off" EOL-audience exclusion guarantee holds
// identically regardless of data source.
func (s *deployedProductService) SearchProjectsByProductVersion(ctx context.Context, req domain.SearchProjectsByProductVersionRequest) (domain.SearchProjectsByProductVersionResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}
	if err := validateUUIDs("productId", []string{req.ProductID}); err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}
	if err := validateUUIDs("productVersionId", []string{req.ProductVersionID}); err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}

	projects, total, err := s.repo.SearchProjectsByProductVersion(ctx, req, mandatoryExcludeClosureStates, mandatoryExcludeSubscriptionTypes)
	if err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}

	return domain.SearchProjectsByProductVersionResponse{
		Projects: projects,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(projects) < total,
	}, nil
}

// CreateDeployedProduct is not supported for the PostgreSQL data source.
func (s *deployedProductService) CreateDeployedProduct(_ context.Context, _ domain.CreateDeployedProductRequest) (domain.CreateDeployedProductResponse, error) {
	return domain.CreateDeployedProductResponse{}, &apierror.ValidationError{Msg: "CreateDeployedProduct is not supported for the PostgreSQL data source"}
}

// UpdateDeployedProduct is not supported for the PostgreSQL data source.
func (s *deployedProductService) UpdateDeployedProduct(_ context.Context, _ domain.UpdateDeployedProductRequest) (domain.UpdateDeployedProductResponse, error) {
	return domain.UpdateDeployedProductResponse{}, &apierror.ValidationError{Msg: "UpdateDeployedProduct is not supported for the PostgreSQL data source"}
}

// SearchDeployedProductMetrics implements DeployedProductService, backed by
// hourly_usage_summary (migration 000054) -- see DeployedProductRepository's own doc
// comment on resolveDeployedProductNodes for how a deployed product's
// instances are resolved.
func (s *deployedProductService) SearchDeployedProductMetrics(ctx context.Context, id string, req domain.DeployedProductMetricsRequest) (domain.DeployedProductMetricsResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}
	if err := validateDateRange(req.StartDate, req.EndDate); err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}

	return s.repo.SearchDeployedProductMetrics(ctx, id, req.DeploymentID, req.StartDate, req.EndDate)
}

// SearchDeployedProductUsageCounts implements DeployedProductService, same
// resolution and validation as SearchDeployedProductMetrics.
func (s *deployedProductService) SearchDeployedProductUsageCounts(ctx context.Context, id string, req domain.DeployedProductUsageCountsRequest) (domain.DeployedProductUsageCountsResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}
	if err := validateDateRange(req.StartDate, req.EndDate); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}

	return s.repo.SearchDeployedProductUsageCounts(ctx, id, req.DeploymentID, req.StartDate, req.EndDate)
}
