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

package dto

import (
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// ProductSummary is one item of the portal's response for POST /products/search.
type ProductSummary struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Class     *string `json:"class,omitempty"`
	CreatedOn *string `json:"createdOn,omitempty"`
	UpdatedOn *string `json:"updatedOn,omitempty"`
}

// SearchProductsResponse is the portal's response for POST /products/search
// and GET /products. TotalRecords (not Total) to match the frontend's
// shared pagination envelope.
type SearchProductsResponse struct {
	Products     []ProductSummary `json:"products"`
	TotalRecords int              `json:"totalRecords"`
	Limit        int              `json:"limit"`
	Offset       int              `json:"offset"`
	HasMore      bool             `json:"hasMore"`
}

// MapSearchProducts builds the portal response from entity-service's SearchProductsResponse.
func MapSearchProducts(r entity.SearchProductsResponse) SearchProductsResponse {
	items := make([]ProductSummary, 0, len(r.Products))
	for _, p := range r.Products {
		items = append(items, ProductSummary{
			ID:        p.ID,
			Name:      p.Name,
			Class:     p.Class,
			CreatedOn: p.CreatedOn,
			UpdatedOn: p.UpdatedOn,
		})
	}
	return SearchProductsResponse{
		Products:     items,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
		HasMore:      r.HasMore,
	}
}

// GetProductsRequest is the portal's translated request for GET /products —
// built from the class/offset/limit query params by the handler.
// entity-service's SearchProductsRequest has no class filter parameter at
// all (unlike the old Ballerina backend's target service, which accepted
// filters.class server-side), so Class can't be forwarded upstream — see
// FilterProductsByClass, which applies it after the fact instead of
// silently dropping it.
type GetProductsRequest struct {
	Pagination entity.Pagination
	Class      string
}

// BuildEntitySearchProductsRequestFromQuery translates GET /products' query
// params into entity-service's POST /products/search request shape. Class
// is intentionally not forwarded — entity-service has nowhere to put it.
func BuildEntitySearchProductsRequestFromQuery(req GetProductsRequest) entity.SearchProductsRequest {
	return entity.SearchProductsRequest{Pagination: req.Pagination}
}

// normalizeProductClass folds a product class value into one comparable
// form: trimmed, lowercased, with spaces rewritten as underscores.
//
// The two sides of this comparison speak different dialects of the same
// value. The frontend sends the portal's own enum ("product_model" —
// PRODUCT_CLASS in the webapp's productConstants.ts, matching the Ballerina
// backend's entity:ProductClass), because the Ballerina backend's upstream
// accepted that spelling as a server-side filter. entity-service instead
// passes ServiceNow's display label straight through ("Product Model").
// Case-insensitive comparison alone never bridged the space/underscore
// difference, so GET /products?class=product_model filtered out every item
// on the page and returned `"products": []` alongside a non-zero
// totalRecords.
func normalizeProductClass(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "_")
}

// FilterProductsByClass keeps only products whose Class matches want, compared
// through normalizeProductClass so the frontend's enum spelling and
// ServiceNow's display label resolve to the same value. Applied after
// MapSearchProducts since entity-service can't filter by class server-side
// (see GetProductsRequest's doc comment).
// This is necessarily best-effort: TotalRecords/HasMore still describe
// entity-service's unfiltered page, since entity-service computed pagination
// before this backend ever saw (or could exclude) an off-class item — a
// page can come back with fewer than Limit on-class items even when more
// exist on a later page. A product with no Class at all (nil) never
// matches, so it's excluded rather than kept as "unknown". On the Postgres
// data source, entity-service's Class vocabulary is only "software"/
// "service" (see domain.ProductClass in entity-service/internal/domain/
// entity.go) — the frontend's own class value ("product_model") only ever
// matches something on the ServiceNow data source, whose Class field is a
// free-form passthrough label; requesting this endpoint against a
// Postgres-mode deployment will therefore always return zero products, the
// same "ServiceNow data source only" limitation documented elsewhere in
// this backend.
func FilterProductsByClass(r SearchProductsResponse, want string) SearchProductsResponse {
	if want == "" {
		return r
	}
	filtered := make([]ProductSummary, 0, len(r.Products))
	for _, p := range r.Products {
		if p.Class != nil && normalizeProductClass(*p.Class) == normalizeProductClass(want) {
			filtered = append(filtered, p)
		}
	}
	r.Products = filtered
	return r
}

// ProductVersionSummary is one item of the portal's response for
// POST /products/{id}/versions/search. Deliberately excludes entity-service's
// ProductID — already known from the request path, redundant here.
type ProductVersionSummary struct {
	ID                             string  `json:"id"`
	Version                        string  `json:"version"`
	CurrentSupportStatus           *string `json:"currentSupportStatus,omitempty"`
	ReleaseDate                    *string `json:"releaseDate,omitempty"`
	SupportEOLDate                 *string `json:"supportEolDate,omitempty"`
	EarliestPossibleSupportEOLDate *string `json:"earliestPossibleSupportEolDate,omitempty"`
	CreatedOn                      *string `json:"createdOn,omitempty"`
	UpdatedOn                      *string `json:"updatedOn,omitempty"`
}

// SearchProductVersionsResponse is the portal's response for
// POST /products/{id}/versions/search — Versions (not ProductVersions) and
// TotalRecords (not Total) to match the frontend's own
// ProductVersionsSearchResponse type (apps/customer-portal/webapp/src/
// features/project-details/types/products.ts).
type SearchProductVersionsResponse struct {
	Versions     []ProductVersionSummary `json:"versions"`
	TotalRecords int                     `json:"totalRecords"`
	Limit        int                     `json:"limit"`
	Offset       int                     `json:"offset"`
	HasMore      bool                    `json:"hasMore"`
}

// MapSearchProductVersions builds the portal response from entity-service's SearchProductVersionsResponse.
func MapSearchProductVersions(r entity.SearchProductVersionsResponse) SearchProductVersionsResponse {
	items := make([]ProductVersionSummary, 0, len(r.ProductVersions))
	for _, v := range r.ProductVersions {
		items = append(items, ProductVersionSummary{
			ID:                             v.ID,
			Version:                        v.Version,
			CurrentSupportStatus:           v.CurrentSupportStatus,
			ReleaseDate:                    v.ReleaseDate,
			SupportEOLDate:                 v.SupportEOLDate,
			EarliestPossibleSupportEOLDate: v.EarliestPossibleSupportEOLDate,
			CreatedOn:                      v.CreatedOn,
			UpdatedOn:                      v.UpdatedOn,
		})
	}
	return SearchProductVersionsResponse{
		Versions:     items,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
		HasMore:      r.HasMore,
	}
}
