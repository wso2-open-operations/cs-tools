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
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ProductRepository defines the persistence operations for the product table
// (migration 0015). domain.Product.Class (values "software"/"service")
// maps to the real product.category column (product_category_enum: SOFTWARE/
// SERVICE) -- there is no "class" column or "product_class_enum" type in the
// migrations; category is the one real column with matching semantics
// (manufacturer/business_unit/unit are different classification axes on the
// same table, not substitutes for this one).
type ProductRepository interface {
	// SearchProducts returns a filtered, paginated slice of products together
	// with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchProducts(ctx context.Context, req domain.SearchProductsRequest) ([]domain.Product, int, error)
	// GetProduct returns nil when there is no such id.
	GetProduct(ctx context.Context, id string) (*domain.Product, error)
}

// productSelect reads a product with its catalogue fields and version counts.
const productSelect = `SELECT p.id, p.name, p.category, p.created_on, p.updated_on,
	p.code, lower(p.unit::text), lower(p.business_unit::text), lower(p.manufacturer::text), p.description,
	(SELECT count(*) FROM product_version pv WHERE pv.product_id = p.id AND pv.retired_on IS NULL),
	(SELECT count(*) FROM product_version pv WHERE pv.product_id = p.id AND pv.retired_on IS NOT NULL),
	(SELECT count(*) FROM product_version pv WHERE pv.product_id = p.id AND pv.retired_on IS NULL
		AND pv.current_support_status IS DISTINCT FROM 'DISCONTINUED')
	FROM product p`

func scanProduct(row pgx.Row) (domain.Product, error) {
	var p domain.Product
	var category string
	if err := row.Scan(&p.ID, &p.Name, &category, &p.CreatedOn, &p.UpdatedOn,
		&p.Code, &p.Unit, &p.BusinessUnit, &p.Manufacturer, &p.Description,
		&p.VersionCount, &p.RetiredVersionCount, &p.ActiveVersionCount); err != nil {
		return p, err
	}
	p.Class = domain.ProductClass(strings.ToLower(category))
	return p, nil
}

type productRepo struct {
	db *pgxpool.Pool
}

// NewProductRepository constructs a ProductRepository backed by the given connection pool.
func NewProductRepository(db *pgxpool.Pool) ProductRepository {
	return &productRepo{db: db}
}

// SearchProducts implements ProductRepository.
func (r *productRepo) SearchProducts(ctx context.Context, req domain.SearchProductsRequest) ([]domain.Product, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (p.name ILIKE $%d ESCAPE '\\')", argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	if req.Class != "" {
		where += fmt.Sprintf(" AND p.category = $%d::product_category_enum", argIdx)
		filterArgs = append(filterArgs, strings.ToUpper(string(req.Class)))
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM product p " + where

	dataQuery := fmt.Sprintf(
		`%s %s
		 ORDER BY p.created_on DESC, p.id
		 LIMIT $%d OFFSET $%d`,
		productSelect, where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var products []domain.Product

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count products: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query products: %w", err)
		}
		defer rows.Close()

		result := make([]domain.Product, 0, req.Pagination.Limit)
		for rows.Next() {
			p, err := scanProduct(rows)
			if err != nil {
				return fmt.Errorf("scan product: %w", err)
			}
			result = append(result, p)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate products: %w", err)
		}
		products = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

// GetProduct implements ProductRepository.
func (r *productRepo) GetProduct(ctx context.Context, id string) (*domain.Product, error) {
	p, err := scanProduct(r.db.QueryRow(ctx, productSelect+` WHERE p.id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	return &p, nil
}
