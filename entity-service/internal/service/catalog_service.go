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
	// (see NewCatalogServiceWithSNFallback). Both SearchCatalogs and
	// GetCatalogItemVariables read from it when non-nil -- they have to agree
	// on where a catalog item comes from, since the id a caller passes to
	// GetCatalogItemVariables is always one SearchCatalogs itself just
	// returned. sr_category/catalog_item exist and are populated in Postgres
	// (99/312 rows respectively, checked live), but SearchCatalogs' own
	// availability check requires a matching sr_category_routing_rule row
	// (0 rows, checked live) and catalog_item_category (also 0 rows) to link
	// an item to a catalog at all, and, more fundamentally, the
	// deployed_product row itself: a deployed product that predates
	// dual-write (or was never touched through this service's own SN-first
	// write paths) has no row in Postgres' deployed_product table at all --
	// the same "Postgres was never backfilled with ServiceNow's existing
	// history" gap documented on deploymentService.SearchDeployments,
	// causing SearchCatalogs' own existence check to fail outright with
	// NotFoundError before the catalog data is even considered. This is why
	// SearchCatalogs falls back to ServiceNow under dual-write -- but
	// GetCatalogItemVariables used to stay on Postgres regardless, so every
	// catalog item SearchCatalogs returned (sourced from ServiceNow, with no
	// Postgres catalog_item_category row to match) 404'd the instant a
	// caller asked for its variables ("catalog item not found in this
	// catalog") -- confirmed live: SR creation's request-type step was
	// 100% broken under dual-write, not incidentally from missing data but
	// systematically, because the two methods disagreed about which system
	// is the source of truth for the same catalog item. Now both follow the
	// same source under dual-write.
	snMirror CatalogService
	// access tells WSO2 staff in the CSM portal from everyone else, for
	// catalog items marked csm_only (migration 0291). Nil, or a caller it
	// can't resolve, counts as not staff: those items stay hidden.
	access AccessService
}

// isCSMPortalStaff reports whether the caller is WSO2 staff acting outside
// the customer portal: an Unrestricted scope (the CSM portal backend for a
// staff email, an M2M client) that did not come through the customer portal.
// Staff signed in to the customer portal get the customer's view. A nil
// resolver or an unresolvable caller answers false.
func isCSMPortalStaff(ctx context.Context, access AccessService) bool {
	if access == nil {
		return false
	}
	scope, err := access.ResolveScope(ctx)
	if err != nil {
		return false
	}
	return scope.Unrestricted && !scope.ViaCustomerPortal
}

// NewCatalogService constructs a CatalogService backed by Postgres
// (sr_category, catalog_item, catalog_item_category, catalog_variable,
// sr_category_routing_rule -- migrations 000067-000071).
//
// access resolves the caller for csm_only items; see the access field.
func NewCatalogService(repo repository.CatalogRepository, access AccessService) CatalogService {
	return &catalogService{repo: repo, access: access}
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

	catalogs, total, err := s.repo.SearchCatalogs(ctx, req.DeployedProductID, isCSMPortalStaff(ctx, s.access), req.Pagination)
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
// Mirrors SearchCatalogs' own snMirror -- see the snMirror field's own doc
// comment for why: under DATA_SOURCE=postgres-servicenow-dual-write, a
// catalog item id only ever came from this same service's own SearchCatalogs
// call, which is itself ServiceNow-sourced in that mode, so this has to read
// the same system or every item 404s. On the plain `postgres` data source
// (snMirror nil), catalog_variable's extra fields
// (read_only/hidden/reference_table/max_length/validation) and the sibling
// catalog_variable_choice table (migration 0125) are kept current by a
// separate sync service, so Postgres is trusted for this read there -- and
// SearchCatalogs is Postgres-only in that mode too, so the two methods still
// agree. The plain `servicenow` data source is unaffected either way: it
// never constructs this type at all.
func (s *catalogService) GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) (domain.GetCatalogItemVariablesResponse, error) {
	if s.snMirror != nil {
		return s.snMirror.GetCatalogItemVariables(ctx, catalogID, catalogItemID)
	}

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

	variables, err := s.repo.GetCatalogItemVariables(ctx, catalogID, catalogItemID, isCSMPortalStaff(ctx, s.access))
	if err != nil {
		return domain.GetCatalogItemVariablesResponse{}, err
	}
	return domain.GetCatalogItemVariablesResponse{Variables: variables}, nil
}
