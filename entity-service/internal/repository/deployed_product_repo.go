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
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// DeployedProductRepository defines the persistence operations for the
// deployed_product table (migration 000014).
type DeployedProductRepository interface {
	// SearchDeployedProducts returns a filtered, paginated slice of enriched deployed-product
	// views together with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error)

	// SearchDeployedProductMetrics returns per-day CORES readings (from
	// usage_count, migration 000054) for every deployment_node resolved to
	// the given deployed product and deployment. A NotFoundError is returned
	// if the deployed product doesn't exist or isn't linked to deploymentID.
	SearchDeployedProductMetrics(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductMetricsResponse, error)

	// SearchDeployedProductUsageCounts is the same resolution as
	// SearchDeployedProductMetrics, but returns every usage_count.count_type
	// found for the resolved instances, not just CORES.
	SearchDeployedProductUsageCounts(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductUsageCountsResponse, error)
}

// resolveDeployedProductNodes looks up the given deployed product, confirms
// it belongs to deploymentID, and returns the deployment_node rows
// (id, node_id) resolved to it -- see instanceRefJoins' own doc comment in
// instance_repo.go for why this resolution (deployment_ref cast to uuid) is
// a best-effort join, unverified against real data.
func (r *deployedProductRepo) resolveDeployedProductNodes(ctx context.Context, id, deploymentID string) (domain.ReferenceTableItem, []domain.ReferenceTableItem, error) {
	var name, number *string
	var dpDeploymentID, versionID *string
	err := r.db.QueryRow(ctx, `SELECT name, number, deployment_id, version_id FROM deployed_product WHERE id = $1`, id).
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
		WHERE dn.product_version_id = $1
		AND CASE
			WHEN dn.deployment_ref ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'
			THEN dn.deployment_ref::uuid
		END = $2::uuid`,
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

	rows, err := r.db.Query(ctx, `
		SELECT uc.counted_on::date, uc.deployment_node_id, uc.count
		FROM usage_count uc
		WHERE uc.deployment_node_id = ANY($1::uuid[])
		AND uc.count_type = 'CORES'
		AND uc.counted_on::date BETWEEN $2::date AND $3::date
		ORDER BY uc.counted_on::date`,
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

	rows, err := r.db.Query(ctx, `
		SELECT uc.counted_on::date, uc.count_type, uc.deployment_node_id, uc.count
		FROM usage_count uc
		WHERE uc.deployment_node_id = ANY($1::uuid[])
		AND uc.counted_on::date BETWEEN $2::date AND $3::date
		ORDER BY uc.counted_on::date, uc.count_type`,
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
	db *pgxpool.Pool
}

// NewDeployedProductRepository constructs a DeployedProductRepository backed by the given connection pool.
func NewDeployedProductRepository(db *pgxpool.Pool) DeployedProductRepository {
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
	where := "WHERE dp.deployment_id IS NOT NULL AND dp.product_id IS NOT NULL"

	if len(req.DeploymentIDs) > 0 {
		where += fmt.Sprintf(" AND dp.deployment_id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.DeploymentIDs)
		argIdx++
	}

	// TODO(phase 2): req.ProductCategories is not applied here. The deployed_product
	// schema has no category column today, so deployedProductService rejects any
	// non-empty ProductCategories before this method is ever called (see
	// deployed_product_service.go) rather than silently ignoring it. Filter it in here
	// once the Postgres cohort's product-category modeling lands, and drop that
	// rejection at the same time.

	countQuery := "SELECT COUNT(*) FROM deployed_product dp " + where

	// update_level_info (JSONB) -- domain.DeployedProductView.Updates -- is
	// deliberately not selected here: its actual JSON shape isn't confirmed
	// against any real payload, so it's left unpopulated (nil, the correct
	// "none recorded" value per that field's own doc comment) rather than
	// guessed at. cores/tps/category, in contrast, are plain scalar columns
	// with an unambiguous mapping, so they are selected.
	dataQuery := fmt.Sprintf(
		`SELECT dp.id, dp.created_on, dp.updated_on,
		        dp.core_count, dp.tps_count, dp.product_category::TEXT,
		        d.id, d.name,
		        p.id, p.name,
		        pv.id, pv.version, pv.release_date, pv.support_eol_date
		 FROM deployed_product dp
		 JOIN deployment d ON dp.deployment_id = d.id
		 JOIN product p ON dp.product_id = p.id
		 LEFT JOIN product_version pv ON dp.version_id = pv.id
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
			if err := rows.Scan(
				&dp.ID, &dp.CreatedOn, &dp.UpdatedOn,
				&dp.Cores, &dp.TPS, &dp.Category,
				&dp.Deployment.ID, &dp.Deployment.Name,
				&dp.Product.ID, &dp.Product.Name,
				&pvID, &pvName, &pvReleaseDate, &pvEoLDate,
			); err != nil {
				return fmt.Errorf("scan deployed product: %w", err)
			}
			if pvID != nil {
				dp.Version = &domain.DeployedProductVersionRef{
					ID:             *pvID,
					Name:           *pvName,
					ReleasedDate:   pvReleaseDate,
					SupportEoLDate: pvEoLDate,
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
