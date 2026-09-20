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

package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// CatalogRepository backs the service-request catalog reads (POST
// /catalogs/search, GET /catalogs/{catalogId}/items/{catalogItemId}/variables)
// from sr_category, catalog_item, catalog_item_category, catalog_variable and
// sr_category_routing_rule (migrations 000067-000071).
//
// "Catalog" in this data source's response is an sr_category row: it's the
// only catalog-level entity here with both a UUID and a display name.
// ServiceNow's own sc_catalog level was collapsed into sr_category.catalog, a
// plain enum (migration 000067), so there is no per-catalog UUID to return
// instead -- and the customer portal renders each "catalog" as a card of
// catalog items, which is what an sr_category is.
type CatalogRepository interface {
	// SearchCatalogs returns the catalogs (sr_category rows) that have at least
	// one catalog item available for the given deployed product, each with its
	// available items, plus the total catalog count before pagination. Returns
	// a NotFoundError if the deployed product does not exist.
	SearchCatalogs(ctx context.Context, deployedProductID string, pagination domain.Pagination) ([]domain.Catalog, int, error)
	// GetCatalogItemVariables returns the variables (form fields) of the
	// catalog item, ordered by their "order" column. Returns a NotFoundError
	// if the item does not belong to the catalog.
	GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) ([]domain.CatalogItemVariable, error)
}

type catalogRepo struct {
	db *pgxpool.Pool
}

// NewCatalogRepository constructs a CatalogRepository backed by the given connection pool.
func NewCatalogRepository(db *pgxpool.Pool) CatalogRepository {
	return &catalogRepo{db: db}
}

// availableCatalogItemsCTE resolves, for the deployed product bound to $1,
// which catalog items sr_category_routing_rule (migration 000071) makes
// available. A rule matches when its product_unit equals the deployed
// product's product.unit AND its classification equals
// deployed_product.product_category -- both pairs share a label set
// (product_unit_enum; PDP/MS/PS/CL/PC) but are distinct enum types, hence the
// ::TEXT casts. A NULL product_unit/classification on the RULE is a wildcard
// ("any").
//
// The two attributes are treated differently when the DEPLOYED PRODUCT's own
// value is NULL, because the staging data differs: product.unit is NULL for
// every product (17/17), and 214 deployed products have no product at all --
// a systemic data gap, so a strict unit comparison could never match any rule
// that names a unit and the catalog would always be empty. An unknown unit is
// therefore not treated as a conflict (unit is only compared when both sides
// are known). deployed_product.product_category, by contrast, is populated
// for most rows, so a NULL there is a per-record gap: such a deployed product
// only matches rules that don't require a specific classification, rather
// than being shown every classification's items. TODO: once product.unit is
// populated, make unit strict again.
const availableCatalogItemsCTE = `
	WITH dp AS (
		SELECT p.unit::TEXT AS unit, d.product_category::TEXT AS classification
		FROM deployed_product d
		LEFT JOIN product p ON p.id = d.product_id
		WHERE d.id = $1
	),
	available AS (
		SELECT DISTINCT r.catalog_item_id
		FROM sr_category_routing_rule r
		CROSS JOIN dp
		WHERE r.catalog_item_id IS NOT NULL
		  AND (r.product_unit IS NULL OR dp.unit IS NULL OR r.product_unit::TEXT = dp.unit)
		  AND (r.classification IS NULL OR r.classification::TEXT = dp.classification)
	)`

// catalogsWithAvailableItems is the shared FROM/WHERE for the count and page
// queries: active categories with at least one available item.
const catalogsWithAvailableItems = `
	FROM sr_category c
	WHERE c.is_active IS DISTINCT FROM FALSE
	  AND EXISTS (
		SELECT 1 FROM catalog_item_category cic
		JOIN available a ON a.catalog_item_id = cic.catalog_item_id
		WHERE cic.sr_category_id = c.id
	  )`

// SearchCatalogs implements CatalogRepository.
func (r *catalogRepo) SearchCatalogs(ctx context.Context, deployedProductID string, pagination domain.Pagination) ([]domain.Catalog, int, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM deployed_product WHERE id = $1)`, deployedProductID).Scan(&exists); err != nil {
		return nil, 0, fmt.Errorf("check deployed product: %w", err)
	}
	if !exists {
		return nil, 0, &apierror.NotFoundError{Msg: "deployed product not found"}
	}

	var total int
	catalogs := make([]domain.Catalog, 0, pagination.Limit)

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, availableCatalogItemsCTE+` SELECT COUNT(*) `+catalogsWithAvailableItems, deployedProductID).Scan(&total); err != nil {
			return fmt.Errorf("count catalogs: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx,
			availableCatalogItemsCTE+` SELECT c.id, c.name `+catalogsWithAvailableItems+`
			 ORDER BY c.name, c.id LIMIT $2 OFFSET $3`,
			deployedProductID, pagination.Limit, pagination.Offset)
		if err != nil {
			return fmt.Errorf("query catalogs: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			c := domain.Catalog{CatalogItems: []domain.CatalogItem{}}
			if err := rows.Scan(&c.ID, &c.Name); err != nil {
				return fmt.Errorf("scan catalog: %w", err)
			}
			catalogs = append(catalogs, c)
		}
		return rows.Err()
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}
	if len(catalogs) == 0 {
		return catalogs, total, nil
	}

	ids := make([]string, len(catalogs))
	byID := make(map[string]int, len(catalogs))
	for i, c := range catalogs {
		ids[i] = c.ID
		byID[c.ID] = i
	}

	rows, err := r.db.Query(ctx,
		availableCatalogItemsCTE+`
		 SELECT cic.sr_category_id, ci.id, ci.name
		 FROM catalog_item_category cic
		 JOIN catalog_item ci ON ci.id = cic.catalog_item_id
		 JOIN available a ON a.catalog_item_id = ci.id
		 WHERE cic.sr_category_id = ANY($2::text[]::uuid[])
		 ORDER BY ci.name, ci.id`,
		deployedProductID, ids)
	if err != nil {
		return nil, 0, fmt.Errorf("query catalog items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var categoryID string
		var item domain.CatalogItem
		if err := rows.Scan(&categoryID, &item.ID, &item.Name); err != nil {
			return nil, 0, fmt.Errorf("scan catalog item: %w", err)
		}
		if i, ok := byID[categoryID]; ok {
			catalogs[i].CatalogItems = append(catalogs[i].CatalogItems, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate catalog items: %w", err)
	}
	return catalogs, total, nil
}

// GetCatalogItemVariables implements CatalogRepository.
//
// catalog_variable (migration 000070) carries only id/name/question_text/
// type/order/is_mandatory/is_active/default_value. ReadOnly, Hidden,
// MaxLength, ReferenceTable, Validation and Choices have no backing column
// yet, so they stay at their zero value (false/nil/omitted). TODO: populate
// them once the schema gains them (choice-based variables in particular
// render as free text until a choices table exists).
func (r *catalogRepo) GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) ([]domain.CatalogItemVariable, error) {
	var belongs bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM catalog_item_category WHERE sr_category_id = $1 AND catalog_item_id = $2)`,
		catalogID, catalogItemID).Scan(&belongs); err != nil {
		return nil, fmt.Errorf("check catalog item: %w", err)
	}
	if !belongs {
		return nil, &apierror.NotFoundError{Msg: "catalog item not found in this catalog"}
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, question_text, "order", type, name, is_mandatory,
		        (is_active IS DISTINCT FROM FALSE), default_value
		 FROM catalog_variable
		 WHERE catalog_item_id = $1
		 ORDER BY "order" NULLS LAST, id`, catalogItemID)
	if err != nil {
		return nil, fmt.Errorf("query catalog variables: %w", err)
	}
	defer rows.Close()

	out := []domain.CatalogItemVariable{}
	for rows.Next() {
		var (
			v                  domain.CatalogItemVariable
			questionText, typ  *string
			order              *int
			mandatory          *bool
			name, defaultValue *string
		)
		if err := rows.Scan(&v.ID, &questionText, &order, &typ, &name, &mandatory, &v.Active, &defaultValue); err != nil {
			return nil, fmt.Errorf("scan catalog variable: %w", err)
		}
		// question_text/type/order/is_mandatory are nullable columns, but the
		// response fields are plain (non-pointer) values -- NULL becomes the
		// zero value rather than an error.
		v.QuestionText = stringOrEmpty(questionText)
		v.Type = stringOrEmpty(typ)
		if order != nil {
			v.Order = *order
		}
		v.Mandatory = mandatory != nil && *mandatory
		v.Name = name
		v.DefaultValue = defaultValue
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate catalog variables: %w", err)
	}
	return out, nil
}
