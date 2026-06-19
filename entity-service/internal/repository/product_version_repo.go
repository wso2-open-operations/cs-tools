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

// ProductVersionRepository defines the persistence operations for the product_versions table.
type ProductVersionRepository interface {
	// SearchProductVersions returns a filtered, paginated slice of product versions
	// together with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchProductVersions(ctx context.Context, req domain.SearchProductVersionsRequest) ([]domain.ProductVersion, int, error)
}

type productVersionRepo struct {
	db *pgxpool.Pool
}

// NewProductVersionRepository constructs a ProductVersionRepository backed by the given connection pool.
func NewProductVersionRepository(db *pgxpool.Pool) ProductVersionRepository {
	return &productVersionRepo{db: db}
}

// SearchProductVersions implements ProductVersionRepository.
func (r *productVersionRepo) SearchProductVersions(ctx context.Context, req domain.SearchProductVersionsRequest) ([]domain.ProductVersion, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	if req.ProductID != "" {
		where += fmt.Sprintf(" AND product_id = $%d", argIdx)
		filterArgs = append(filterArgs, req.ProductID)
		argIdx++
	}

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (version ILIKE $%d ESCAPE '\\')", argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM product_versions " + where

	dataQuery := fmt.Sprintf(
		`SELECT id, product_id, version, current_support_status,
		        release_date, support_eol_date, earliest_possible_support_eol_date,
		        created_at, updated_at
		 FROM product_versions %s
		 ORDER BY release_date DESC, id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var versions []domain.ProductVersion

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count product_versions: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query product_versions: %w", err)
		}
		defer rows.Close()

		result := make([]domain.ProductVersion, 0, req.Pagination.Limit)
		for rows.Next() {
			var pv domain.ProductVersion
			if err := rows.Scan(
				&pv.ID, &pv.ProductID, &pv.Version, &pv.CurrentSupportStatus,
				&pv.ReleaseDate, &pv.SupportEOLDate, &pv.EarliestPossibleSupportEOLDate,
				&pv.CreatedOn, &pv.UpdatedOn,
			); err != nil {
				return fmt.Errorf("scan product_version: %w", err)
			}
			result = append(result, pv)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate product_versions: %w", err)
		}
		versions = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return versions, total, nil
}
