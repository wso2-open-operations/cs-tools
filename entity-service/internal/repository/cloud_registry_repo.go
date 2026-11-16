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

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// cloudQuerier is the sliver of the pool the registry read needs.
type cloudQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Three small reads rather than one join: the regions and the weights are
// independent one-to-many children of the cloud, and joining both would
// multiply rows (regions x weights) for nothing.
const (
	cloudsSQL = `
    SELECT slug, display_name, offering_key, incident_prefix, weighted
      FROM cloud_registry
     WHERE is_active IS TRUE
     ORDER BY sort_order, slug
`
	cloudRegionsSQL = `
    SELECT r.cloud_slug, r.region_key, r.parent_service_id::text
      FROM cloud_registry_region r
      JOIN cloud_registry c ON c.slug = r.cloud_slug
     WHERE c.is_active IS TRUE
     ORDER BY r.cloud_slug, r.sort_order, r.region_key
`
	// ::float8 so 13.75 reaches Go as the same double the literal 13.75 is.
	cloudWeightsSQL = `
    SELECT w.cloud_slug, w.service_offering_id::text, w.weight::float8
      FROM cloud_registry_offering_weight w
      JOIN cloud_registry c ON c.slug = w.cloud_slug
     WHERE c.is_active IS TRUE
`
)

// loadCloudDefinitions reads every ACTIVE cloud with its regions and weights.
// An inactive cloud is simply absent, so it is treated as unknown everywhere.
func loadCloudDefinitions(ctx context.Context, db cloudQuerier) ([]domain.CloudDefinition, error) {
	rows, err := db.Query(ctx, cloudsSQL)
	if err != nil {
		return nil, fmt.Errorf("query cloud registry: %w", err)
	}
	defer rows.Close()

	var defs []domain.CloudDefinition
	weighted := map[string]bool{}
	idx := map[string]int{}
	for rows.Next() {
		var d domain.CloudDefinition
		var w bool
		if err := rows.Scan(&d.Slug, &d.DisplayName, &d.OfferingKey, &d.IncidentPrefix, &w); err != nil {
			return nil, fmt.Errorf("scan cloud registry: %w", err)
		}
		weighted[d.Slug] = w
		idx[d.Slug] = len(defs)
		defs = append(defs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cloud registry: %w", err)
	}
	rows.Close()

	regionRows, err := db.Query(ctx, cloudRegionsSQL)
	if err != nil {
		return nil, fmt.Errorf("query cloud regions: %w", err)
	}
	defer regionRows.Close()
	for regionRows.Next() {
		var slug string
		var reg domain.AvailabilityRegion
		if err := regionRows.Scan(&slug, &reg.Key, &reg.ParentID); err != nil {
			return nil, fmt.Errorf("scan cloud region: %w", err)
		}
		if i, ok := idx[slug]; ok {
			defs[i].Plan.Regions = append(defs[i].Plan.Regions, reg)
		}
	}
	if err := regionRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cloud regions: %w", err)
	}
	regionRows.Close()

	// A weighted cloud with no weight rows still gets an EMPTY, non-nil map:
	// nil is what marks a cloud unweighted, and the two must not be confused.
	for i := range defs {
		if weighted[defs[i].Slug] {
			defs[i].Plan.Weights = map[string]float64{}
		}
	}

	weightRows, err := db.Query(ctx, cloudWeightsSQL)
	if err != nil {
		return nil, fmt.Errorf("query cloud weights: %w", err)
	}
	defer weightRows.Close()
	for weightRows.Next() {
		var slug, offering string
		var weight float64
		if err := weightRows.Scan(&slug, &offering, &weight); err != nil {
			return nil, fmt.Errorf("scan cloud weight: %w", err)
		}
		if i, ok := idx[slug]; ok && defs[i].Plan.Weights != nil {
			defs[i].Plan.Weights[offering] = weight
		}
	}
	if err := weightRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cloud weights: %w", err)
	}
	return defs, nil
}

// Clouds returns the active cloud registry.
func (r *cloudStatusDashboardRepository) Clouds(ctx context.Context) ([]domain.CloudDefinition, error) {
	return loadCloudDefinitions(ctx, r.db)
}

// Clouds returns the active cloud registry.
func (r *cloudStatusRepository) Clouds(ctx context.Context) ([]domain.CloudDefinition, error) {
	return loadCloudDefinitions(ctx, r.db)
}
