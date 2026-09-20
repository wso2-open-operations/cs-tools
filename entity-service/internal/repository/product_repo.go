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
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ProductRepository defines the persistence operations for the product table
// (migration 000010). domain.Product.Class (values "software"/"service")
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
		where += fmt.Sprintf(" AND (name ILIKE $%d ESCAPE '\\')", argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	if req.Class != "" {
		where += fmt.Sprintf(" AND category = $%d::product_category_enum", argIdx)
		filterArgs = append(filterArgs, strings.ToUpper(string(req.Class)))
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM product " + where

	dataQuery := fmt.Sprintf(
		`SELECT id, name, category, created_on, updated_on
		 FROM product %s
		 ORDER BY created_on DESC, id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
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
			var p domain.Product
			var category string
			if err := rows.Scan(&p.ID, &p.Name, &category, &p.CreatedOn, &p.UpdatedOn); err != nil {
				return fmt.Errorf("scan product: %w", err)
			}
			p.Class = domain.ProductClass(strings.ToLower(category))
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
