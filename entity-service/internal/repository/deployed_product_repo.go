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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// DeployedProductRepository defines the persistence operations for the
// deployed_product table (migration 0019).
type DeployedProductRepository interface {
	// SearchDeployedProducts returns a filtered, paginated slice of enriched deployed-product
	// views together with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error)

	// SearchDeployedProductMetrics returns per-day CORES readings (from
	// daily_usage_summary, migration 0054, with deployment_information's
	// core_count filling the days that have none) for every deployment_node resolved
	// to the given deployed product and deployment. A NotFoundError is
	// returned if the deployed product doesn't exist or isn't linked to
	// deploymentID.
	//
	// Reads daily_usage_summary, not its sibling hourly_usage_summary --
	// confirmed against real staging data that the two can disagree, and not
	// in hourly's favor: for one real deployed product, daily_usage_summary
	// held a reading for every one of 7 resolved nodes across months of real
	// days, while hourly_usage_summary (this method's previous sole source)
	// had no rows for any of them at all. Both tables are mirrored by an
	// external sync (see migration 0054's own doc comment) with no
	// foreign-key or rollup relationship to each other in this schema, so
	// there is no guarantee hourly ever catches up to what daily already has
	// -- daily_usage_summary is also the one actually shaped like what this
	// endpoint needs (summary_date/value, already one row per node per day
	// per count type), where hourly's per-event counted_on had to be
	// bucketed into days in Go. This endpoint has no production traffic yet,
	// so there's no existing behavior relying on hourly's own narrower (and,
	// confirmed directly, occasionally non-overlapping) coverage to preserve
	// -- a straight switch, not a union of both tables. A handful of real
	// rows repeat the same (node, day, count_type) key (a correction/
	// re-ingest with no dedup on the sync's side), so the query SUMs them
	// rather than assuming exactly one row per key.
	SearchDeployedProductMetrics(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductMetricsResponse, error)

	// SearchDeployedProductUsageCounts is the same resolution as
	// SearchDeployedProductMetrics, reading the same daily_usage_summary
	// table (see that method's own doc comment for why), but returns every
	// count_type found for the resolved instances, not just CORES.
	SearchDeployedProductUsageCounts(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductUsageCountsResponse, error)

	// SearchProjectsByProductVersion returns the deduplicated, paginated set
	// of projects with a deployed_product on the given product+version,
	// joining deployed_product directly to project (migration 0019's
	// project_id FK) rather than going through deployment the way
	// SearchDeployedProducts does -- there's no deployment-name/id to
	// display here, only the owning project. excludeClosureStates/
	// excludeSubscriptionTypes are the caller's fixed, non-optional
	// exclusion policy (mandatoryExcludeClosureStates/
	// mandatoryExcludeSubscriptionTypes in sn_deployed_product_service.go,
	// mirrored here for parity with that data source) -- not a
	// caller-supplied filter, so they're separate parameters rather than
	// part of domain.SearchProjectsByProductVersionRequest. COUNT and SELECT
	// are executed concurrently on separate pool connections, same as
	// SearchProjects/SearchDeployedProducts.
	SearchProjectsByProductVersion(ctx context.Context, req domain.SearchProjectsByProductVersionRequest, excludeClosureStates []string, excludeSubscriptionTypes []domain.SubscriptionType) ([]domain.EntityRef, int, error)

	// CreateDeployedProductFromServiceNow inserts a deployed_product row
	// using identity (id/number) ServiceNow has already assigned -- see
	// deployedProductService.createDeployedProductSNFirst's own doc comment
	// for why: deployed_product.number is NOT NULL UNIQUE and Postgres has no
	// generator for it, the same unresolved problem deployment.number had
	// before CreateDeploymentFromServiceNow.
	CreateDeployedProductFromServiceNow(ctx context.Context, req domain.CreateDeployedProductRequest, id, number, createdBy string, createdOn time.Time) (domain.CreatedDeployedProduct, error)

	// UpdateDeployedProductFields applies a Postgres-side update for the same
	// field group UpdateDeployedProductRequest itself enforces (detail
	// fields XOR Active=false) -- called on the dual-write data source's
	// Postgres-first leg; the ServiceNow mirror runs separately and
	// asynchronously. When req.DeploymentID is set, the update is
	// additionally scoped to a row whose deployment_id matches it (the same
	// IDOR guard snDeployedProductService.UpdateDeployedProduct enforces via
	// a live search) -- a mismatch is indistinguishable from "not found" and
	// returns the same NotFoundError.
	UpdateDeployedProductFields(ctx context.Context, req domain.UpdateDeployedProductRequest, updatedBy string) (domain.UpdatedDeployedProduct, error)

	// GetDeployedProductCategory returns the deployed product's own
	// product_category (lower-case, e.g. "ms"; nil when unset), or a
	// NotFoundError when no such row exists. A single-row lookup, not the
	// list/filter machinery SearchDeployedProducts already has -- used by
	// caseService's own project-type category allow-list enforcement at
	// case/SR creation time, which only ever checks one id at a time.
	GetDeployedProductCategory(ctx context.Context, id string) (*string, error)
}

// GetDeployedProductCategory implements DeployedProductRepository.
func (r *deployedProductRepo) GetDeployedProductCategory(ctx context.Context, id string) (*string, error) {
	var category *string
	err := r.db.QueryRow(ctx, `SELECT product_category::TEXT FROM deployed_product WHERE id = $1`, id).Scan(&category)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, &apierror.NotFoundError{Msg: "deployed product not found"}
		}
		return nil, fmt.Errorf("get deployed product category: %w", err)
	}
	if category != nil {
		lower := strings.ToLower(*category)
		category = &lower
	}
	return category, nil
}

// resolveDeployedProductNodes looks up the given deployed product, confirms
// it belongs to deploymentID, and returns the deployment_node rows
// (id, node_id) resolved to it -- see instanceRefJoins' own doc comment in
// instance_repo.go for how a node is matched to a deployment (by project key and
// deployment number, only within the same project).
func (r *deployedProductRepo) resolveDeployedProductNodes(ctx context.Context, id, deploymentID string) (domain.ReferenceTableItem, []domain.ReferenceTableItem, error) {
	var name, number *string
	var dpDeploymentID, versionID *string
	// product_id IS NOT NULL matches the same "is this a real deployed
	// product" definition the deployment picker's own productCount already
	// enforces (see deployment_repo.go's own doc comment on that subquery)
	// -- without it, a caller who already knows a deployed product's id
	// could reach its usage/metrics directly even though that same record
	// is treated as having no product at all everywhere else, including
	// being hidden from ever being offered as a choice in the first place.
	err := r.db.QueryRow(ctx, `SELECT name, number, deployment_id, version_id FROM deployed_product WHERE id = $1 AND product_id IS NOT NULL`, id).
		Scan(&name, &number, &dpDeploymentID, &versionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.ReferenceTableItem{}, nil, &apierror.NotFoundError{Msg: "deployed product not found"}
		}
		return domain.ReferenceTableItem{}, nil, fmt.Errorf("get deployed product: %w", err)
	}
	// Case-insensitive: dpDeploymentID comes back from Postgres in its
	// canonical lower-case form, but deploymentID is caller-supplied and
	// validateUUIDs accepts upper-case hex too -- a naive == would false-404
	// a request that spelled its UUID in upper case.
	if dpDeploymentID == nil || !strings.EqualFold(*dpDeploymentID, deploymentID) {
		return domain.ReferenceTableItem{}, nil, &apierror.NotFoundError{Msg: "deployed product not found for the given deployment"}
	}

	dpRef := domain.ReferenceTableItem{ID: id}
	if name != nil {
		dpRef.Name = *name
	} else if number != nil {
		dpRef.Name = *number
	}

	if versionID == nil {
		return dpRef, nil, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT dn.id, dn.node_id
		FROM deployment_node dn
		JOIN project proj ON proj.key = dn.project_key
		JOIN deployment dep ON dep.number = dn.deployment_number AND dep.project_id = proj.id
		WHERE dn.product_version_id = $1
		AND dep.id = $2::uuid`,
		*versionID, deploymentID,
	)
	if err != nil {
		return domain.ReferenceTableItem{}, nil, fmt.Errorf("resolve deployed product instances: %w", err)
	}
	defer rows.Close()

	var nodes []domain.ReferenceTableItem
	for rows.Next() {
		var nodeID, nodeKey string
		if err := rows.Scan(&nodeID, &nodeKey); err != nil {
			return domain.ReferenceTableItem{}, nil, fmt.Errorf("scan deployed product instance: %w", err)
		}
		nodes = append(nodes, domain.ReferenceTableItem{ID: nodeID, Name: nodeKey})
	}
	if err := rows.Err(); err != nil {
		return domain.ReferenceTableItem{}, nil, fmt.Errorf("iterate deployed product instances: %w", err)
	}
	return dpRef, nodes, nil
}

// SearchDeployedProductMetrics implements DeployedProductRepository.
func (r *deployedProductRepo) SearchDeployedProductMetrics(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductMetricsResponse, error) {
	dpRef, nodes, err := r.resolveDeployedProductNodes(ctx, id, deploymentID)
	if err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}
	dateRange := domain.DeployedProductMetricsDateRange{Start: startDate, End: endDate}
	if len(nodes) == 0 {
		return domain.DeployedProductMetricsResponse{
			DeployedProduct: dpRef,
			Summary:         domain.DeployedProductMetricsSummary{DateRange: dateRange},
		}, nil
	}

	nodeIDs := make([]string, len(nodes))
	nodeNameByID := make(map[string]string, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
		nodeNameByID[n.ID] = n.Name
	}

	// A node's core count reaches Postgres two ways: as a CORES row of
	// daily_usage_summary (a product that reports it as a usage count, or an
	// offline report), and as core_count on its deployment_information
	// snapshot (what every live heartbeat's host info carries, and the only
	// place the instance-level metrics stats read). The chart used to read only
	// the first, so telemetry that reports cores through the host snapshot never
	// drew a Core Usage chart here although the instance stats showed it.
	// A CORES usage row still wins for its node and day. A snapshot fills the
	// days that have none, one value per node per day:
	//   - A snapshot covers every UTC day from its first report to its last. The
	//     same (node, hash) being reported again only moves payload_updated_on
	//     forward, and an unchanged hash means unchanged host info, so the cores
	//     held for all the days in between.
	//   - Days are taken in UTC, not in the session time zone.
	//   - Where snapshots overlap on a day, the one first reported later wins (it
	//     replaced the other). If that one carries no core count the node has no
	//     reading that day, rather than falling back to an older value.
	// A node-day with both a live (API_CALL) and an uploaded (FILE_UPLOAD) CORES row
	// counts once, the live one -- see dailyUsageSourcePreference.
	rows, err := r.db.Query(ctx, `
		WITH daily AS (
			SELECT DISTINCT ON (dus.deployment_node_id, dus.summary_date)
			       dus.summary_date AS day, dus.deployment_node_id AS node_id, dus.value AS cores
			FROM daily_usage_summary dus
			WHERE dus.deployment_node_id = ANY($1::uuid[])
			AND dus.count_type = 'CORES'
			AND dus.value IS NOT NULL
			AND dus.summary_date BETWEEN $2::date AND $3::date
			ORDER BY dus.deployment_node_id, dus.summary_date, `+dailyUsageSourcePreference+`
		), snapshot AS (
			SELECT day, node_id, cores FROM (
				SELECT DISTINCT ON (dn.id, g.day)
				       g.day, dn.id AS node_id, di.core_count AS cores
				FROM deployment_node dn
				JOIN deployment_information di ON di.node_id = dn.node_id
				CROSS JOIN LATERAL (
					SELECT (LEAST(di.payload_created_on, di.payload_updated_on) AT TIME ZONE 'UTC')::date AS first_day,
					       (GREATEST(di.payload_created_on, di.payload_updated_on) AT TIME ZONE 'UTC')::date AS last_day
				) span
				CROSS JOIN LATERAL (
					SELECT d::date AS day
					FROM generate_series(
						GREATEST(span.first_day, $2::date)::timestamp,
						LEAST(span.last_day, $3::date)::timestamp,
						interval '1 day'
					) AS d
				) g
				WHERE dn.id = ANY($1::uuid[])
				ORDER BY dn.id, g.day,
				         COALESCE(di.payload_created_on, di.payload_updated_on) DESC,
				         COALESCE(di.payload_updated_on, di.payload_created_on) DESC
			) latest
			WHERE cores IS NOT NULL
		)
		SELECT day, node_id, cores FROM daily
		UNION ALL
		SELECT s.day, s.node_id, s.cores FROM snapshot s
		WHERE NOT EXISTS (SELECT 1 FROM daily d WHERE d.day = s.day AND d.node_id = s.node_id)
		ORDER BY day`,
		nodeIDs, startDate, endDate,
	)
	if err != nil {
		return domain.DeployedProductMetricsResponse{}, fmt.Errorf("query deployed product cores: %w", err)
	}
	defer rows.Close()

	type dayEntry struct {
		instances []domain.DeployedProductMetricsInstance
	}
	byDate := map[string]*dayEntry{}
	var dates []string
	var allCores []int
	for rows.Next() {
		var day time.Time
		var nodeID string
		var cores int
		if err := rows.Scan(&day, &nodeID, &cores); err != nil {
			return domain.DeployedProductMetricsResponse{}, fmt.Errorf("scan deployed product cores: %w", err)
		}
		d := day.UTC().Format("2006-01-02")
		e, ok := byDate[d]
		if !ok {
			e = &dayEntry{}
			byDate[d] = e
			dates = append(dates, d)
		}
		e.instances = append(e.instances, domain.DeployedProductMetricsInstance{ID: nodeID, Name: nodeNameByID[nodeID], Cores: cores})
		allCores = append(allCores, cores)
	}
	if err := rows.Err(); err != nil {
		return domain.DeployedProductMetricsResponse{}, fmt.Errorf("iterate deployed product cores: %w", err)
	}
	sort.Strings(dates)

	chartData := make([]domain.DeployedProductMetricsChartEntry, 0, len(dates))
	for _, d := range dates {
		e := byDate[d]
		total, min, max := 0, e.instances[0].Cores, e.instances[0].Cores
		for _, inst := range e.instances {
			total += inst.Cores
			if inst.Cores < min {
				min = inst.Cores
			}
			if inst.Cores > max {
				max = inst.Cores
			}
		}
		chartData = append(chartData, domain.DeployedProductMetricsChartEntry{
			Date:          d,
			InstanceCount: len(e.instances),
			TotalCores:    total,
			MinCores:      min,
			MaxCores:      max,
			AvgCores:      float64(total) / float64(len(e.instances)),
			Instances:     e.instances,
		})
	}

	summary := domain.DeployedProductMetricsSummary{DateRange: dateRange, TotalInstances: len(nodes)}
	if len(allCores) > 0 {
		total, min, max := 0, allCores[0], allCores[0]
		for _, c := range allCores {
			total += c
			if c < min {
				min = c
			}
			if c > max {
				max = c
			}
		}
		avg := float64(total) / float64(len(allCores))
		summary.MinCores = &min
		summary.MaxCores = &max
		summary.AvgCores = &avg
	}

	return domain.DeployedProductMetricsResponse{DeployedProduct: dpRef, Summary: summary, ChartData: chartData}, nil
}

// SearchDeployedProductUsageCounts implements DeployedProductRepository.
func (r *deployedProductRepo) SearchDeployedProductUsageCounts(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductUsageCountsResponse, error) {
	dpRef, nodes, err := r.resolveDeployedProductNodes(ctx, id, deploymentID)
	if err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}
	dateRange := domain.DeployedProductMetricsDateRange{Start: startDate, End: endDate}
	if len(nodes) == 0 {
		return domain.DeployedProductUsageCountsResponse{
			DeployedProduct: dpRef,
			Summary:         domain.DeployedProductUsageCountsSummary{DateRange: dateRange, CountTypes: map[string]domain.CountTypeAggregation{}},
		}, nil
	}

	nodeIDs := make([]string, len(nodes))
	nodeNameByID := make(map[string]string, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
		nodeNameByID[n.ID] = n.Name
	}

	// One row per node, count type and day, the live one when there is both a live
	// and an uploaded row (dailyUsageSourcePreference).
	rows, err := r.db.Query(ctx, `
		SELECT summary_date, count_type, deployment_node_id, value FROM (
			SELECT DISTINCT ON (dus.deployment_node_id, dus.count_type, dus.summary_date)
			       dus.summary_date, dus.count_type, dus.deployment_node_id, dus.value
			FROM daily_usage_summary dus
			WHERE dus.deployment_node_id = ANY($1::uuid[])
			AND dus.value IS NOT NULL
			AND dus.summary_date BETWEEN $2::date AND $3::date
			ORDER BY dus.deployment_node_id, dus.count_type, dus.summary_date, `+dailyUsageSourcePreference+`
		) picked
		ORDER BY summary_date, count_type`,
		nodeIDs, startDate, endDate,
	)
	if err != nil {
		return domain.DeployedProductUsageCountsResponse{}, fmt.Errorf("query deployed product usage counts: %w", err)
	}
	defer rows.Close()

	type dateTypeEntry struct {
		instances []domain.UsageCountInstance
	}
	byDateType := map[string]*dateTypeEntry{}
	byDate := map[string][]string{} // date -> ordered count types seen
	var dates []string
	byType := map[string][]float64{} // count type -> every individual value in range, for the summary
	for rows.Next() {
		var day time.Time
		var countType, nodeID string
		var count int
		if err := rows.Scan(&day, &countType, &nodeID, &count); err != nil {
			return domain.DeployedProductUsageCountsResponse{}, fmt.Errorf("scan deployed product usage count: %w", err)
		}
		d := day.UTC().Format("2006-01-02")
		key := d + "|" + countType
		e, ok := byDateType[key]
		if !ok {
			e = &dateTypeEntry{}
			byDateType[key] = e
			if _, seen := byDate[d]; !seen {
				dates = append(dates, d)
			}
			byDate[d] = append(byDate[d], countType)
		}
		e.instances = append(e.instances, domain.UsageCountInstance{ID: nodeID, Name: nodeNameByID[nodeID], Value: float64(count)})
		byType[countType] = append(byType[countType], float64(count))
	}
	if err := rows.Err(); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, fmt.Errorf("iterate deployed product usage counts: %w", err)
	}
	sort.Strings(dates)

	chartData := make([]domain.DeployedProductUsageCountsChartEntry, 0, len(dates))
	for _, d := range dates {
		counts := make(map[string]domain.UsageCountEntry, len(byDate[d]))
		for _, ct := range byDate[d] {
			e := byDateType[d+"|"+ct]
			var sum float64
			for _, v := range e.instances {
				sum += v.Value
			}
			counts[ct] = domain.UsageCountEntry{Value: sum, Aggregation: "SUM", Instances: e.instances}
		}
		chartData = append(chartData, domain.DeployedProductUsageCountsChartEntry{Date: d, Counts: counts})
	}

	countTypes := make(map[string]domain.CountTypeAggregation, len(byType))
	for ct, values := range byType {
		sum, min, max := 0.0, values[0], values[0]
		for _, v := range values {
			sum += v
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
		countTypes[ct] = domain.CountTypeAggregation{Aggregation: "SUM", Min: min, Max: max, Avg: sum / float64(len(values))}
	}

	return domain.DeployedProductUsageCountsResponse{
		DeployedProduct: dpRef,
		Summary:         domain.DeployedProductUsageCountsSummary{DateRange: dateRange, CountTypes: countTypes},
		ChartData:       chartData,
	}, nil
}

type deployedProductRepo struct {
	db *Scoped
}

// NewDeployedProductRepository constructs a DeployedProductRepository whose
// every query runs under the caller identity on ctx (deployed_product has
// row-level security, migration 0176).
func NewDeployedProductRepository(db *Scoped) DeployedProductRepository {
	return &deployedProductRepo{db: db}
}

// SearchDeployedProducts implements DeployedProductRepository.
func (r *deployedProductRepo) SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
	filterArgs := []any{}
	argIdx := 1

	// deployed_product.deployment_id/product_id are both nullable (migration
	// 000014 sets them NULL when the parent deployment/product is deleted).
	// The data query below inner-joins both and so can never return such a
	// row; without these predicates the count query would still include it,
	// inflating total relative to what's actually returned.
	// DeployedProductView.Deployment/Product are non-pointer EntityRef
	// values, so switching to LEFT joins isn't a safe alternative -- that
	// would need a response-contract change and nullable scan handling.
	//
	// dp.active is how a deployed product is soft-deleted (PATCH .../products/{id}
	// {active: false} -- see DeployedProductRepository.UpdateDeployedProductFields).
	// This query never filtered on it at all, so a deactivated product kept
	// showing up in every list exactly as before, making "delete" appear to
	// silently do nothing. NULL counts as active, the same convention
	// AccessService.ResolveScope already uses for "user".is_active.
	// req.Active = false selects the deactivated ones only; omitted or true keeps
	// the active-only default.
	where := "WHERE dp.deployment_id IS NOT NULL AND dp.product_id IS NOT NULL"
	if req.Active != nil && !*req.Active {
		where += " AND dp.active = FALSE"
	} else {
		where += " AND (dp.active IS NULL OR dp.active = TRUE)"
	}

	if len(req.DeploymentIDs) > 0 {
		where += fmt.Sprintf(" AND dp.deployment_id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.DeploymentIDs)
		argIdx++
	}

	// Scoped by the deployed product's own project (dp.project_id), not its
	// deployment's. project.sf_id is nullable and a NULL never matches ANY().
	if len(req.ProjectSfIDs) > 0 {
		where += fmt.Sprintf(" AND dp.project_id IN (SELECT id FROM project WHERE sf_id = ANY($%d::text[]))", argIdx)
		filterArgs = append(filterArgs, req.ProjectSfIDs)
		argIdx++
	}

	// deployed_product.product_category (deployed_product_category_enum:
	// PDP/MS/PS/CL/PC) is already selected below -- the request's own
	// lowercase values (SearchDeployedProductsRequest.ProductCategories'
	// doc comment: e.g. "pdp") are upper-cased before the enum cast, same
	// convention every other enum-array filter in this codebase uses.
	//
	// A NULL category is treated as a wildcard (matches any requested
	// category), not excluded -- the same fail-open treatment this
	// codebase already gives sr_category_routing_rule's own classification
	// match, adopted there specifically because real deployed_product rows
	// were found mostly uncategorized. Without this, a project type that
	// restricts SR categories (ProjectFeatures.SrProductCategories) hid the
	// overwhelming majority of real, active deployed products from the SR
	// creation product dropdown -- "Product Version: Not available" even
	// with active products on the deployment -- purely because nothing has
	// ever backfilled this column, not because the product's category
	// genuinely doesn't match.
	if len(req.ProductCategories) > 0 {
		categories := make([]string, len(req.ProductCategories))
		for i, c := range req.ProductCategories {
			categories[i] = strings.ToUpper(c)
		}
		where += fmt.Sprintf(" AND (dp.product_category IS NULL OR dp.product_category = ANY($%d::text[]::deployed_product_category_enum[]))", argIdx)
		filterArgs = append(filterArgs, categories)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM deployed_product dp " + where

	dataQuery := fmt.Sprintf(
		`SELECT dp.id, dp.created_on, dp.updated_on,
		        dp.core_count, dp.tps_count, dp.product_category::TEXT,
		        dp.description, dp.update_level_info, pr.sf_id, COALESCE(dp.active, TRUE),
		        d.id, d.name, d.type::TEXT,
		        p.id, p.name, p.code,
		        pv.id, pv.version, pv.release_date, pv.support_eol_date
		 FROM deployed_product dp
		 JOIN deployment d ON dp.deployment_id = d.id
		 JOIN product p ON dp.product_id = p.id
		 LEFT JOIN product_version pv ON dp.version_id = pv.id
		 LEFT JOIN project pr ON dp.project_id = pr.id
		 %s
		 ORDER BY dp.created_on DESC, dp.id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var deployedProducts []domain.DeployedProductView

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count deployed products: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query deployed products: %w", err)
		}
		defer rows.Close()

		result := make([]domain.DeployedProductView, 0, req.Pagination.Limit)
		for rows.Next() {
			var dp domain.DeployedProductView
			// Version fields are nullable (LEFT JOIN).
			var pvID, pvName *string
			var pvReleaseDate, pvEoLDate *time.Time
			// update_level_info is nullable JSONB; decoded below into
			// dp.Updates once the row is scanned.
			var updateLevelInfo []byte
			// deployment.type is nullable (see SearchDeployments).
			var deploymentType *string
			if err := rows.Scan(
				&dp.ID, &dp.CreatedOn, &dp.UpdatedOn,
				&dp.Cores, &dp.TPS, &dp.Category,
				&dp.Description, &updateLevelInfo, &dp.ProjectSfID, &dp.Active,
				&dp.Deployment.ID, &dp.Deployment.Name, &deploymentType,
				&dp.Product.ID, &dp.Product.Name, &dp.Product.Abbreviation,
				&pvID, &pvName, &pvReleaseDate, &pvEoLDate,
			); err != nil {
				return fmt.Errorf("scan deployed product: %w", err)
			}
			if deploymentType != nil {
				// Lower case, the same canonical form SearchDeployments returns.
				t := domain.DeploymentType(strings.ToLower(*deploymentType))
				dp.DeploymentType = &t
			}
			if pvID != nil {
				dp.Version = &domain.DeployedProductVersionRef{
					ID:             *pvID,
					Name:           *pvName,
					ReleasedDate:   pvReleaseDate,
					SupportEoLDate: pvEoLDate,
				}
			}
			dp.Category = lowercaseCategory(dp.Category)
			// update_level_info is real, synced data, not something this
			// service ever writes into that shape itself -- a live check
			// found it stored as a JSON array (the only shape this decodes),
			// but also, for a real minority of rows, a JSON object or a
			// plain string. A row shaped like that must not fail the whole
			// search: Updates is already documented elsewhere as "not
			// populated... deliberately left nil rather than guessed at"
			// for the exact same reason (no confirmed shape to trust), so a
			// row this can't decode gets the identical nil treatment,
			// logged rather than silently dropped.
			if len(updateLevelInfo) > 0 {
				var updates []domain.ProductUpdateEntry
				if err := json.Unmarshal(updateLevelInfo, &updates); err != nil {
					slog.WarnContext(ctx, "search deployed products: update_level_info is not a JSON array, leaving Updates unset",
						"deployedProductId", dp.ID, "error", err)
				} else if len(updates) > 0 {
					dp.Updates = updates
				}
			}
			result = append(result, dp)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate deployed products: %w", err)
		}
		deployedProducts = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return deployedProducts, total, nil
}

// lowercaseCategory converts deployed_product_category_enum's UPPER-case
// label ("MS", "PDP", ...) read from the database into the lower-case code
// the rest of the contract uses: the search request's ProductCategories
// filter values and ProjectFeatures.SrProductCategories/
// DefaultCaseProductCategories are all lower case, and clients compare the
// returned category against them case-sensitively. A nil (NULL) category
// stays nil, never an empty string. The input is not mutated.
func lowercaseCategory(c *string) *string {
	if c == nil {
		return nil
	}
	v := strings.ToLower(*c)
	return &v
}

// SearchProjectsByProductVersion implements DeployedProductRepository.
// productID/productVersionID are validated as UUIDs by the caller
// (deployedProductService.SearchProjectsByProductVersion) before reaching
// here, and are always passed as query parameters ($1/$2 below), never
// interpolated into the SQL string -- same discipline as every other filter
// in this file.
func (r *deployedProductRepo) SearchProjectsByProductVersion(ctx context.Context, req domain.SearchProjectsByProductVersionRequest, excludeClosureStates []string, excludeSubscriptionTypes []domain.SubscriptionType) ([]domain.EntityRef, int, error) {
	filterArgs := []any{req.ProductID, req.ProductVersionID}
	argIdx := 3

	// Mandatory, unconditional (not gated on a caller-supplied slice, unlike
	// the two exclusions below): a project whose subscription contract has
	// ended (end_date in the past) is treated as inaccessible by the
	// customer portal itself (isProjectSuspended in
	// apps/customer-portal/webapp/src/utils/permission.ts, which checks
	// end_date independently of wso2_closure_state — a project's closure
	// state is frequently left NULL when its subscription simply expired
	// rather than being explicitly marked Restricted/Suspended). An EOL
	// announcement audience must not include a project the customer portal
	// itself already blocks the customer from viewing. end_date is a plain
	// DATE column (no time-of-day); the cutoff is the last millisecond of
	// that day (end_date + 1 day - 1ms), not simply "the next UTC day",
	// so this matches apps/customer-portal/webapp/src/utils/permission.ts's
	// own isProjectContractEnded (end-of-day UTC, strictly after) and
	// isProjectContractEnded in sn_project_service.go to the millisecond —
	// a plain date-vs-date comparison here would exclude the project one
	// millisecond later than both of those (only at the next day's exact
	// midnight instead of 23:59:59.999 on end_date's own day), a real,
	// if practically negligible, inconsistency between the ServiceNow and
	// Postgres cohorts a reviewer flagged. Both sides of the comparison are
	// plain "timestamp without time zone" (NOW() AT TIME ZONE 'UTC' yields
	// the current UTC wall-clock reading in that type), so this needs no
	// timezone-conversion assumption the way comparing a timestamptz
	// directly against a bare "date + interval" would.
	where := "WHERE dp.product_id = $1 AND dp.version_id = $2" +
		" AND (proj.end_date IS NULL OR proj.end_date + INTERVAL '1 day' - INTERVAL '1 millisecond' >= (NOW() AT TIME ZONE 'UTC'))"

	// Same NULL-permissive, upper-cased-vocabulary matching as
	// ProjectRepository.SearchProjects' ExcludeClosureStates clause -- see
	// that clause's own doc comment for why. This is the mandatory
	// exclusion policy, not a caller-supplied filter, so excludeClosureStates
	// is only ever the fixed mandatoryExcludeClosureStates slice.
	if len(excludeClosureStates) > 0 {
		upper := make([]string, len(excludeClosureStates))
		for i, s := range excludeClosureStates {
			upper[i] = strings.ToUpper(s)
		}
		where += fmt.Sprintf(" AND (proj.wso2_closure_state IS NULL OR proj.wso2_closure_state::text <> ALL($%d::text[]))", argIdx)
		filterArgs = append(filterArgs, upper)
		argIdx++
	}

	// Same NULL-permissive matching as ProjectRepository.SearchProjects'
	// ExcludeSubscriptionTypes clause -- see that clause's own doc comment
	// for why project_type.name is normalized in SQL rather than compared
	// as-is. Likewise always the fixed mandatoryExcludeSubscriptionTypes
	// slice, not a caller-supplied filter.
	if len(excludeSubscriptionTypes) > 0 {
		types := make([]string, len(excludeSubscriptionTypes))
		for i, t := range excludeSubscriptionTypes {
			types[i] = string(t)
		}
		where += fmt.Sprintf(" AND (pt.name IS NULL OR lower(replace(pt.name, ' ', '_')) <> ALL($%d::text[]))", argIdx)
		filterArgs = append(filterArgs, types)
		argIdx++
	}

	// DISTINCT: a project can have more than one deployed_product row
	// matching this exact product+version (e.g. two deployments each
	// running it), which would otherwise duplicate the project in both the
	// count and the result.
	countQuery := "SELECT COUNT(DISTINCT proj.id) FROM deployed_product dp" +
		" JOIN project proj ON dp.project_id = proj.id" +
		" LEFT JOIN project_type pt ON pt.id = proj.project_type_id " + where

	dataQuery := fmt.Sprintf(
		`SELECT DISTINCT proj.id, proj.name
		 FROM deployed_product dp
		 JOIN project proj ON dp.project_id = proj.id
		 LEFT JOIN project_type pt ON pt.id = proj.project_type_id
		 %s
		 ORDER BY proj.name, proj.id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var projects []domain.EntityRef

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count projects by product version: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query projects by product version: %w", err)
		}
		defer rows.Close()

		result := make([]domain.EntityRef, 0, req.Pagination.Limit)
		for rows.Next() {
			var p domain.EntityRef
			if err := rows.Scan(&p.ID, &p.Name); err != nil {
				return fmt.Errorf("scan project by product version: %w", err)
			}
			result = append(result, p)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate projects by product version: %w", err)
		}
		projects = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

// deployedProductCreateFKField maps deployed_product's own foreign-key
// constraint names (migration 0019's auto-generated
// "<table>_<column>_fkey" names) to the request field that referenced the
// missing row, for CreateDeployedProductFromServiceNow's 23503 handling --
// same map-based convention as change_request_repo.go's
// changeRequestPatchFKField/changeRequestPatchCRFKField.
var deployedProductCreateFKField = map[string]string{
	"deployed_product_project_id_fkey":    "projectId",
	"deployed_product_deployment_id_fkey": "deploymentId",
	"deployed_product_product_id_fkey":    "productId",
	"deployed_product_version_id_fkey":    "versionId",
}

const createDeployedProductFromServiceNowQuery = `
	INSERT INTO deployed_product (
		id, created_on, updated_on, created_by, updated_by,
		number, description, active,
		core_count, tps_count,
		project_id, deployment_id, product_id, version_id,
		product_category
	)
	VALUES (
		$1, $2, $2, $3, $3,
		$4, $5, TRUE,
		$6, $7,
		$8::uuid, $9::uuid, $10::uuid, $11::uuid,
		$12::text::deployed_product_category_enum
	)
	RETURNING id, created_on, created_by`

// CreateDeployedProductFromServiceNow implements DeployedProductRepository.
// name/life_cycle_stage/life_cycle_stage_status/update_level_info are left
// NULL -- CreateDeployedProductRequest carries no fields for them (SN's own
// create payload doesn't send them either, see
// snCreateDeployedProductPayload), matching parity between the two data
// sources rather than inventing values ServiceNow itself doesn't set on
// create. product_category is the one exception: see
// domain.CreateDeployedProductRequest.Category's own doc comment for why
// it's written here despite having no SN-side equivalent.
func (r *deployedProductRepo) CreateDeployedProductFromServiceNow(ctx context.Context, req domain.CreateDeployedProductRequest, id, number, createdBy string, createdOn time.Time) (domain.CreatedDeployedProduct, error) {
	var category *string
	if req.Category != nil {
		c := strings.ToUpper(*req.Category)
		category = &c
	}

	var created domain.CreatedDeployedProduct
	err := r.db.QueryRow(ctx, createDeployedProductFromServiceNowQuery,
		id, createdOn, createdBy,
		number, req.Description,
		req.Cores, req.TPS,
		req.ProjectID, req.DeploymentID, req.ProductID, req.VersionID,
		category,
	).Scan(&created.ID, &created.CreatedOn, &created.CreatedBy)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation -- number or id already exists
				return domain.CreatedDeployedProduct{}, &apierror.ValidationError{Msg: "a deployed product with this identity already exists"}
			case "23503": // foreign_key_violation -- one of the referenced ids does not exist
				field := deployedProductCreateFKField[pgErr.ConstraintName]
				if field == "" {
					field = "one or more referenced fields"
				}
				return domain.CreatedDeployedProduct{}, &apierror.ValidationError{Msg: field + " does not refer to an existing record"}
			}
		}
		return domain.CreatedDeployedProduct{}, fmt.Errorf("create deployed product from servicenow: %w", err)
	}
	return created, nil
}

// parseUpdateDeployedProductDescription reports whether req.Description (a
// json.RawMessage, see that field's own doc comment) was provided at all,
// and if so, the *string value to write -- nil for an explicit
// "description":null (clear it), a non-nil pointer for a real value.
// Absent (len(raw) == 0) returns provided=false so the caller's COALESCE-style
// CASE leaves the existing column value untouched. A value that decodes to
// neither null nor a JSON string is rejected as a ValidationError rather than
// reaching the database as a type-mismatched write -- decodeRequest's own
// json.Unmarshal already guarantees raw is syntactically valid JSON by this
// point, just not necessarily a string.
func parseUpdateDeployedProductDescription(raw json.RawMessage) (provided bool, value *string, err error) {
	if len(raw) == 0 {
		return false, nil, nil
	}
	if string(raw) == "null" {
		return true, nil, nil
	}
	var s string
	if unmarshalErr := json.Unmarshal(raw, &s); unmarshalErr != nil {
		return false, nil, &apierror.ValidationError{Msg: "description must be a string or null"}
	}
	return true, &s, nil
}

const updateDeployedProductFieldsQuery = `
	UPDATE deployed_product SET
		updated_on = NOW(), updated_by = $2,
		core_count = COALESCE($3, core_count),
		tps_count = COALESCE($4, tps_count),
		description = CASE WHEN $5 THEN $6 ELSE description END,
		update_level_info = CASE WHEN $7 THEN $8::jsonb ELSE update_level_info END,
		active = COALESCE($9, active),
		product_category = COALESCE($11::text::deployed_product_category_enum, product_category)
	WHERE id = $1::uuid
	AND ($10::uuid IS NULL OR deployment_id = $10::uuid)
	RETURNING id, updated_on, updated_by`

// UpdateDeployedProductFields implements DeployedProductRepository.
//
// Cores/TPS have no "explicit clear" state on the wire (both are plain
// nilable Go pointers, domain.UpdateDeployedProductRequest's own
// json:"cores"/json:"tps" tags have no omitempty-defeating RawMessage
// trick) -- nil always means "not provided", so a plain COALESCE is enough,
// same limitation ServiceNow's own snUpdateDeployedProductPayload has for
// these two fields (parity between data sources, not a gap introduced here).
//
// Description, in contrast, is json.RawMessage precisely so "not provided",
// "clear it", and "set it" all stay distinguishable -- see
// parseUpdateDeployedProductDescription.
//
// Updates (update_level_info, JSONB) is a whole-array replace when req.Updates
// is non-nil (including a non-nil empty slice, the caller's way of clearing
// all history -- see UpdateDeployedProductRequest's own doc comment), encoded
// via encoding/json using domain.ProductUpdateEntry's own
// updateLevel/date/details wire tags. This is the first write to this
// column anywhere in this codebase -- SearchDeployedProducts deliberately
// leaves it unselected (its own TODO(phase 2) comment) because no real
// payload had confirmed its shape; this establishes that shape going
// forward, matching the wire contract's own field names rather than
// inventing a different one.
func (r *deployedProductRepo) UpdateDeployedProductFields(ctx context.Context, req domain.UpdateDeployedProductRequest, updatedBy string) (domain.UpdatedDeployedProduct, error) {
	descriptionProvided, description, err := parseUpdateDeployedProductDescription(req.Description)
	if err != nil {
		return domain.UpdatedDeployedProduct{}, err
	}

	updatesProvided := req.Updates != nil
	var updatesJSON []byte
	if updatesProvided {
		updatesJSON, err = json.Marshal(req.Updates)
		if err != nil {
			return domain.UpdatedDeployedProduct{}, fmt.Errorf("marshal deployed product updates: %w", err)
		}
	}

	var category *string
	if req.Category != nil {
		c := strings.ToUpper(*req.Category)
		category = &c
	}

	var updated domain.UpdatedDeployedProduct
	err = r.db.QueryRow(ctx, updateDeployedProductFieldsQuery,
		req.ID, updatedBy,
		req.Cores, req.TPS,
		descriptionProvided, description,
		updatesProvided, updatesJSON,
		req.Active,
		req.DeploymentID,
		category,
	).Scan(&updated.ID, &updated.UpdatedOn, &updated.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		if req.DeploymentID != nil {
			return domain.UpdatedDeployedProduct{}, &apierror.NotFoundError{Msg: "deployed product not found for the given deployment"}
		}
		return domain.UpdatedDeployedProduct{}, &apierror.NotFoundError{Msg: "deployed product not found"}
	}
	if err != nil {
		return domain.UpdatedDeployedProduct{}, fmt.Errorf("update deployed product fields: %w", err)
	}
	if req.Updates != nil {
		updated.Updates = req.Updates
	}
	return updated, nil
}
