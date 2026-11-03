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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ProductVersionRepository defines the persistence operations for the
// product_version table (migration 0016, catalogue fields from 0270).
type ProductVersionRepository interface {
	// SearchProductVersions returns a filtered, paginated slice of product versions
	// together with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	// With req.ProductID set it keeps the per-product order (release date,
	// newest first); without, it orders by product name, then release date.
	SearchProductVersions(ctx context.Context, req domain.SearchProductVersionsRequest) ([]domain.ProductVersion, int, error)
	// GetProductVersions returns the rows with these ids, in the search order.
	GetProductVersions(ctx context.Context, ids []string) ([]domain.ProductVersion, error)
}

type productVersionRepo struct {
	db *pgxpool.Pool
}

// NewProductVersionRepository constructs a ProductVersionRepository backed by the given connection pool.
func NewProductVersionRepository(db *pgxpool.Pool) ProductVersionRepository {
	return &productVersionRepo{db: db}
}

const productVersionFrom = ` FROM product_version pv
	JOIN product p ON p.id = pv.product_id
	LEFT JOIN release_line rl ON rl.id = pv.release_line_id
	LEFT JOIN product_version_carbon c ON c.product_version_id = pv.id`

// productVersionSelect reads a version with its catalogue fields. Status is
// the enum text (uppercase), as the search always returned.
const productVersionSelect = `SELECT pv.id, pv.product_id, pv.version, pv.current_support_status,
	pv.release_date, pv.support_eol_date, pv.earliest_possible_support_eol_date,
	pv.deployment_profile, pv.created_on, pv.updated_on,
	p.name, p.code, pv.serial_number, pv.support_eol_announcement_date,
	pv.public_id, pv.public_name, pv.public_abbreviation, pv.description, pv.is_public, pv.show_in_support_matrix,
	pv.tags, pv.release_links, pv.legacy_status, pv.release_line_id::text, rl.name,
	c.product_version_id IS NOT NULL, c.platform_release, c.platform_version, c.carbon_kernel_version,
	c.release_chunk, c.kernel_patch_no, pv.retired_on` + productVersionFrom

func scanProductVersion(row pgx.Row) (domain.ProductVersion, error) {
	var pv domain.ProductVersion
	var hasCarbon bool
	var carbon domain.CatalogCarbon
	err := row.Scan(
		&pv.ID, &pv.ProductID, &pv.Version, &pv.CurrentSupportStatus,
		&pv.ReleaseDate, &pv.SupportEOLDate, &pv.EarliestPossibleSupportEOLDate,
		&pv.DeploymentProfile, &pv.CreatedOn, &pv.UpdatedOn,
		&pv.ProductName, &pv.ProductCode, &pv.SerialNumber, &pv.SupportEOLAnnouncementDate,
		&pv.PublicID, &pv.PublicName, &pv.PublicAbbreviation, &pv.Description, &pv.IsPublic, &pv.ShowInSupportMatrix,
		&pv.Tags, &pv.ReleaseLinks, &pv.LegacyStatus, &pv.ReleaseLineID, &pv.ReleaseLineName,
		&hasCarbon, &carbon.PlatformRelease, &carbon.PlatformVersion, &carbon.CarbonKernelVersion,
		&carbon.ReleaseChunk, &carbon.KernelPatchNo, &pv.RetiredOn,
	)
	if err != nil {
		return pv, err
	}
	if hasCarbon {
		pv.Carbon = &carbon
	}
	if pv.Tags == nil {
		pv.Tags = []string{}
	}
	return pv, nil
}

const (
	perProductVersionOrder = `pv.release_date DESC, pv.id`
	allVersionsOrder       = `lower(p.name), pv.release_date DESC NULLS LAST, pv.version, pv.deployment_profile NULLS FIRST, pv.id`
)

// SearchProductVersions implements ProductVersionRepository.
func (r *productVersionRepo) SearchProductVersions(ctx context.Context, req domain.SearchProductVersionsRequest) ([]domain.ProductVersion, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"
	order := allVersionsOrder

	if req.ProductID != "" {
		where += fmt.Sprintf(" AND pv.product_id = $%d", argIdx)
		filterArgs = append(filterArgs, req.ProductID)
		argIdx++
		order = perProductVersionOrder
	}

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (pv.version ILIKE $%d ESCAPE '\\')", argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	// An empty profile (an explicit opt-out) applies no profile filter.
	if req.DeploymentProfile != nil && *req.DeploymentProfile != "" {
		where += fmt.Sprintf(" AND lower(pv.deployment_profile) = lower($%d)", argIdx)
		filterArgs = append(filterArgs, *req.DeploymentProfile)
		argIdx++
	}

	if req.PublicID != nil {
		where += fmt.Sprintf(" AND pv.public_id = $%d", argIdx)
		filterArgs = append(filterArgs, *req.PublicID)
		argIdx++
	}

	switch req.Feed {
	case domain.CatalogFeedSupportMatrix:
		where += " AND pv.retired_on IS NULL AND pv.show_in_support_matrix AND pv.public_id IS NOT NULL"
	case domain.CatalogFeedReleaseMatrix:
		where += " AND pv.retired_on IS NULL AND pv.is_public AND pv.public_id IS NOT NULL"
	}
	if req.ExcludeRetired {
		where += " AND pv.retired_on IS NULL"
	}

	countQuery := "SELECT COUNT(*)" + productVersionFrom + " " + where

	dataQuery := fmt.Sprintf(
		`%s %s
		 ORDER BY %s
		 LIMIT $%d OFFSET $%d`,
		productVersionSelect, where, order, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var versions []domain.ProductVersion

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count product_version: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query product_version: %w", err)
		}
		defer rows.Close()

		result := make([]domain.ProductVersion, 0, req.Pagination.Limit)
		for rows.Next() {
			pv, err := scanProductVersion(rows)
			if err != nil {
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

// GetProductVersions implements ProductVersionRepository.
func (r *productVersionRepo) GetProductVersions(ctx context.Context, ids []string) ([]domain.ProductVersion, error) {
	rows, err := r.db.Query(ctx, productVersionSelect+` WHERE pv.id = ANY($1::uuid[]) ORDER BY `+allVersionsOrder, ids)
	if err != nil {
		return nil, fmt.Errorf("get product versions: %w", err)
	}
	defer rows.Close()
	out := []domain.ProductVersion{}
	for rows.Next() {
		pv, err := scanProductVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("get product versions: scan: %w", err)
		}
		out = append(out, pv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get product versions: %w", err)
	}
	return out, nil
}
