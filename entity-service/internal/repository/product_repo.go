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

// ProductRepository defines the persistence operations for the products table.
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

	countQuery := "SELECT COUNT(*) FROM products " + where

	dataQuery := fmt.Sprintf(
		`SELECT id, name, class, created_at, updated_at
		 FROM products %s
		 ORDER BY created_at DESC, id
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
			if err := rows.Scan(&p.ID, &p.Name, &p.Class, &p.CreatedOn, &p.UpdatedOn); err != nil {
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
