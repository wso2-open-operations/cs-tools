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

type catalogService struct {
	repo repository.CatalogRepository
}

// NewCatalogService constructs a CatalogService backed by Postgres
// (sr_category, catalog_item, catalog_item_category, catalog_variable,
// sr_category_routing_rule -- migrations 000067-000071).
func NewCatalogService(repo repository.CatalogRepository) CatalogService {
	return &catalogService{repo: repo}
}

// SearchCatalogs implements CatalogService.
func (s *catalogService) SearchCatalogs(ctx context.Context, req domain.SearchCatalogsRequest) (domain.SearchCatalogsResponse, error) {
	if req.DeployedProductID == "" {
		return domain.SearchCatalogsResponse{}, &apierror.ValidationError{Msg: "deployedProductId is required"}
	}
	if err := validateUUIDs("deployedProductId", []string{req.DeployedProductID}); err != nil {
		return domain.SearchCatalogsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCatalogsResponse{}, err
	}

	catalogs, total, err := s.repo.SearchCatalogs(ctx, req.DeployedProductID, req.Pagination)
	if err != nil {
		return domain.SearchCatalogsResponse{}, err
	}
	return domain.SearchCatalogsResponse{
		Catalogs: catalogs,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}

// GetCatalogItemVariables implements CatalogService.
func (s *catalogService) GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) (domain.GetCatalogItemVariablesResponse, error) {
	if catalogID == "" {
		return domain.GetCatalogItemVariablesResponse{}, &apierror.ValidationError{Msg: "catalogId is required"}
	}
	if catalogItemID == "" {
		return domain.GetCatalogItemVariablesResponse{}, &apierror.ValidationError{Msg: "catalogItemId is required"}
	}
	if err := validateUUIDs("catalogId", []string{catalogID}); err != nil {
		return domain.GetCatalogItemVariablesResponse{}, err
	}
	if err := validateUUIDs("catalogItemId", []string{catalogItemID}); err != nil {
		return domain.GetCatalogItemVariablesResponse{}, err
	}

	variables, err := s.repo.GetCatalogItemVariables(ctx, catalogID, catalogItemID)
	if err != nil {
		return domain.GetCatalogItemVariablesResponse{}, err
	}
	return domain.GetCatalogItemVariablesResponse{Variables: variables}, nil
}
