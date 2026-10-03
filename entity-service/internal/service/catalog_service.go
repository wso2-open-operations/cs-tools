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
	// snMirror is set only under DATA_SOURCE=postgres-servicenow-dual-write
	// (see NewCatalogServiceWithSNFallback). Only SearchCatalogs reads from it
	// when non-nil -- GetCatalogItemVariables always reads repo (see its own
	// doc comment below for why that split exists). sr_category/catalog_item
	// exist and are populated in Postgres (99/312 rows respectively, checked
	// live), but SearchCatalogs' own availability check requires a matching
	// sr_category_routing_rule row (0 rows) and, more fundamentally, the
	// deployed_product row itself: a deployed product that predates
	// dual-write (or was never touched through this service's own SN-first
	// write paths) has no row in Postgres' deployed_product table at all --
	// the same "Postgres was never backfilled with ServiceNow's existing
	// history" gap documented on deploymentService.SearchDeployments,
	// causing SearchCatalogs' own existence check to fail outright with
	// NotFoundError before the catalog data is even considered.
	snMirror CatalogService
}

// NewCatalogService constructs a CatalogService backed by Postgres
// (sr_category, catalog_item, catalog_item_category, catalog_variable,
// sr_category_routing_rule -- migrations 000067-000071).
func NewCatalogService(repo repository.CatalogRepository) CatalogService {
	return &catalogService{repo: repo}
}

// NewCatalogServiceWithSNFallback constructs a CatalogService for
// DATA_SOURCE=postgres-servicenow-dual-write -- see the snMirror field's own
// doc comment for why every read goes to ServiceNow rather than Postgres
// under this mode.
func NewCatalogServiceWithSNFallback(repo repository.CatalogRepository, snMirror CatalogService) CatalogService {
	return &catalogService{repo: repo, snMirror: snMirror}
}

// SearchCatalogs implements CatalogService.
func (s *catalogService) SearchCatalogs(ctx context.Context, req domain.SearchCatalogsRequest) (domain.SearchCatalogsResponse, error) {
	if s.snMirror != nil {
		return s.snMirror.SearchCatalogs(ctx, req)
	}

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
//
// Unlike SearchCatalogs, this always reads repo (Postgres) even under
// DATA_SOURCE=postgres-servicenow-dual-write: catalog_variable's extra fields
// (read_only/hidden/reference_table/max_length/validation) and the sibling
// catalog_variable_choice table (migration 0125) are now kept current by a
// separate sync service, so Postgres is trusted for this read. This does not
// change the plain `servicenow` data source, which still calls ServiceNow
// live.
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
