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
	// hourly_usage_summary, migration 000054) for every deployment_node resolved to
	// the given deployed product and deployment. A NotFoundError is returned
	// if the deployed product doesn't exist or isn't linked to deploymentID.
	SearchDeployedProductMetrics(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductMetricsResponse, error)

	// SearchDeployedProductUsageCounts is the same resolution as
	// SearchDeployedProductMetrics, but returns every hourly_usage_summary.count_type
	// found for the resolved instances, not just CORES.
	SearchDeployedProductUsageCounts(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductUsageCountsResponse, error)

	// SearchProjectsByProductVersion returns the deduplicated, paginated set
	// of projects with a deployed_product on the given product+version,
	// joining deployed_product directly to project (migration 000014's
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
}

// resolveDeployedProductNodes looks up the given deployed product, confirms
// it belongs to deploymentID, and returns the deployment_node rows
// (id, node_id) resolved to it -- see instanceRefJoins' own doc comment in
// instance_repo.go for how a node is matched to a deployment (by project key and
// deployment number, only within the same project).
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

	rows, err := r.db.Query(ctx, `
		SELECT uc.counted_on::date, uc.deployment_node_id, uc.count
		FROM hourly_usage_summary uc
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
		FROM hourly_usage_summary uc
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
