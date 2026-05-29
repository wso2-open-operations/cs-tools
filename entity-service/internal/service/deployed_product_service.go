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

	deployedProducts, total, err := s.repo.SearchDeployedProducts(ctx, req)
	if err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}

	return domain.SearchDeployedProductsResponse{
		DeployedProducts: deployedProducts,
		Total:            total,
		Limit:            req.Pagination.Limit,
		Offset:           req.Pagination.Offset,
		HasMore:          req.Pagination.Offset+len(deployedProducts) < total,
	}, nil
}
