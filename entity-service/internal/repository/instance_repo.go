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
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// InstanceRepository defines the persistence operations for the "instance"
// concept -- a single running deployment node, backed by deployment_node
// (migration 000054) and its satellite facts tables (deployment_information,
// hourly_usage_summary, daily_usage_summary). See instanceRefJoins' own doc comment
// for the caveats around resolving an instance's project/deployment/
// deployed-product references.
type InstanceRepository interface {
	// SearchInstances returns a paginated list of instances (deployment_node
	// rows), each enriched with its latest deployment_information snapshot as
	// Metadata, filtered by optional project/deployment/deployed-product IDs
	// (mutually exclusive, enforced by the caller) and an optional
	// created_on date range.
	SearchInstances(ctx context.Context, req domain.SearchInstancesRequest) ([]domain.Instance, int, error)

	// SearchInstanceMetrics returns each matching instance's deployment_information
	// history (core count / JDK version / raw deployment metadata) as a time series,
	// newest first, within the given date range.
	SearchInstanceMetrics(ctx context.Context, filters domain.InstanceDateRangeFilters) ([]domain.InstanceMetric, int, error)

	// SearchInstanceUsage returns each matching instance's hourly_usage_summary history,
	// grouped into one InstanceSummary per (instance, day).
	SearchInstanceUsage(ctx context.Context, filters domain.InstanceDateRangeFilters) ([]domain.InstanceUsageEntry, int, error)

	// SearchInstanceMetricsStats returns the CORES metric (the only numeric
	// metric deployment_information carries) aggregated across every matching
	// instance, one total per day.
	SearchInstanceMetricsStats(ctx context.Context, filters domain.InstanceDateRangeFilters) (domain.InstanceMetricsStatsResponse, error)

	// SearchInstanceUsageStats returns daily_usage_summary aggregated across
	// every matching instance, one total per (day, usage type), optionally
	// narrowed to a single data source ("API_CALL"/"FILE_UPLOAD").
	SearchInstanceUsageStats(ctx context.Context, filters domain.InstanceDateRangeFilters, dataSource *string) (domain.InstanceUsageStatsResponse, error)
}

type instanceRepo struct {
	db *pgxpool.Pool
}

// NewInstanceRepository constructs an InstanceRepository backed by the given connection pool.
func NewInstanceRepository(db *pgxpool.Pool) InstanceRepository {
	return &instanceRepo{db: db}
}

// instanceRefJoins resolves an instance's Project/Deployment/Product/
// DeployedProduct references.
//
// product_version_id is a real foreign key (dn.product_version_id ->
// product_version.id -> product.id), so the Product reference is always
// reliable. deployment_node has no foreign key to project, deployment or
// deployed_product, only two free-text columns copied from the reported
// payload, so the rest is resolved from them:
//
//   - project_key is matched to project.key (unique, and populated on every
//     node), so a node's Project never depends on its deployment resolving.
//   - deployment_number is matched to deployment.number (unique) ONLY IF that
//     deployment belongs to the node's own project. The reported value is not
//     always a deployment number: staging has a sys_id-like hex string and a
//     bare "320" that happens to equal the number of a deployment in a
//     different project, so matching on the number alone would attach those
//     nodes to the wrong project. Requiring the project to agree leaves them
//     unresolved instead (Deployment/DeployedProduct nil, Project still set).
//   - DeployedProduct additionally requires deployed_product.version_id to
//     match the node's product_version, since deployment_id alone doesn't
//     uniquely identify one deployed product.
//
// Verified against staging's 16 nodes: 14 resolve to a project and 11 to a
// deployment (each within the node's own project), and none resolves to a
// deployment of another project.
const instanceRefJoins = `
	LEFT JOIN product_version pv ON pv.id = dn.product_version_id
	LEFT JOIN product p ON p.id = pv.product_id
	LEFT JOIN project proj ON proj.key = dn.project_key
	LEFT JOIN deployment dep ON dep.number = dn.deployment_number AND dep.project_id = proj.id
	LEFT JOIN deployed_product dprod ON dprod.deployment_id = dep.id AND dprod.version_id = dn.product_version_id`

const instanceRefColumns = `proj.id, proj.name, dep.id, dep.name, p.id, p.name, dprod.id, dprod.name`

func buildOptionalRef(id, name *string) *domain.ReferenceTableItem {
	if id == nil {
		return nil
	}
	n := ""
	if name != nil {
		n = *name
	}
	return &domain.ReferenceTableItem{ID: *id, Name: n}
}

// instanceIDFilterClause builds the WHERE fragment and args for the
// mutually-exclusive project/deployment/deployed-product ID filters shared
// by every instance query. argIdx is the next free placeholder index.
func instanceIDFilterClause(projectIDs, deploymentIDs, deployedProductIDs []string, argIdx int) (string, []any) {
	switch {
	case len(projectIDs) > 0:
		return fmt.Sprintf(" AND proj.id = ANY($%d::uuid[])", argIdx), []any{projectIDs}
	case len(deploymentIDs) > 0:
		return fmt.Sprintf(" AND dep.id = ANY($%d::uuid[])", argIdx), []any{deploymentIDs}
	case len(deployedProductIDs) > 0:
		return fmt.Sprintf(" AND dprod.id = ANY($%d::uuid[])", argIdx), []any{deployedProductIDs}
	default:
		return "", nil
	}
}

// SearchInstances implements InstanceRepository.
func (r *instanceRepo) SearchInstances(ctx context.Context, req domain.SearchInstancesRequest) ([]domain.Instance, int, error) {
	where := "WHERE TRUE"
	args := []any{}
	argIdx := 1

	if req.Filters != nil {
		if req.Filters.StartDate != nil {
			where += fmt.Sprintf(" AND dn.created_on::date >= $%d::date", argIdx)
			args = append(args, *req.Filters.StartDate)
			argIdx++
		}
		if req.Filters.EndDate != nil {
			where += fmt.Sprintf(" AND dn.created_on::date <= $%d::date", argIdx)
			args = append(args, *req.Filters.EndDate)
			argIdx++
		}
		clause, clauseArgs := instanceIDFilterClause(req.Filters.ProjectIDs, req.Filters.DeploymentIDs, req.Filters.DeployedProductIDs, argIdx)
		where += clause
		args = append(args, clauseArgs...)
		argIdx += len(clauseArgs)
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM deployment_node dn "+instanceRefJoins+" "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count instances: %w", err)
	}

	query := fmt.Sprintf(
		`SELECT dn.id, dn.node_id, dn.created_on, dn.updated_on, %s
		 FROM deployment_node dn %s %s
		 ORDER BY dn.created_on DESC, dn.id
		 LIMIT $%d OFFSET $%d`,
		instanceRefColumns, instanceRefJoins, where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query instances: %w", err)
	}
	defer rows.Close()

	nodeIDs := make([]string, 0, req.Pagination.Limit)
	instances := make([]domain.Instance, 0, req.Pagination.Limit)
	for rows.Next() {
		var inst domain.Instance
		var nodeID string
		var createdOn, updatedOn time.Time
		var projID, projName, depID, depName, prodID, prodName, dprodID, dprodName *string
		if err := rows.Scan(
			&inst.ID, &nodeID, &createdOn, &updatedOn,
			&projID, &projName, &depID, &depName, &prodID, &prodName, &dprodID, &dprodName,
		); err != nil {
			return nil, 0, fmt.Errorf("scan instance: %w", err)
		}
		inst.Key = nodeID
		inst.CreatedOn = createdOn.UTC().Format(time.RFC3339)
		inst.UpdatedOn = updatedOn.UTC().Format(time.RFC3339)
		inst.Project = buildOptionalRef(projID, projName)
		inst.Deployment = buildOptionalRef(depID, depName)
		inst.Product = buildOptionalRef(prodID, prodName)
		inst.DeployedProduct = buildOptionalRef(dprodID, dprodName)
		instances = append(instances, inst)
		nodeIDs = append(nodeIDs, nodeID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate instances: %w", err)
	}

	if len(nodeIDs) == 0 {
		return instances, total, nil
	}

	metadataByNode, err := r.latestDeploymentInformation(ctx, nodeIDs)
	if err != nil {
		return nil, 0, err
	}
	for i := range instances {
		instances[i].Metadata = metadataByNode[instances[i].Key]
	}

	return instances, total, nil
}

// latestDeploymentInformation batch-fetches each node's most recent
// deployment_information row (by payload_updated_on), keyed by node_id.
func (r *instanceRepo) latestDeploymentInformation(ctx context.Context, nodeIDs []string) (map[string]*domain.InstanceMetadata, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (node_id)
		       node_id, id, jdk_version, core_count, deployment_info,
		       created_on, updated_on, payload_created_on, payload_updated_on
		FROM deployment_information
		WHERE node_id = ANY($1::text[])
		ORDER BY node_id, payload_updated_on DESC`,
		nodeIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("query deployment_information: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*domain.InstanceMetadata, len(nodeIDs))
	for rows.Next() {
		var nodeID, id string
		var jdkVersion *string
		var coreCount *int
		var rawInfo []byte
		var createdOn, updatedOn, reportedCreatedOn, reportedUpdatedOn time.Time
		if err := rows.Scan(&nodeID, &id, &jdkVersion, &coreCount, &rawInfo, &createdOn, &updatedOn, &reportedCreatedOn, &reportedUpdatedOn); err != nil {
			return nil, fmt.Errorf("scan deployment_information: %w", err)
		}
		var info map[string]any
		if len(rawInfo) > 0 {
			if err := json.Unmarshal(rawInfo, &info); err != nil {
				return nil, fmt.Errorf("parse deployment_info json for node %q: %w", nodeID, err)
			}
		}
		customCreated := reportedCreatedOn.UTC().Format(time.RFC3339)
		customUpdated := reportedUpdatedOn.UTC().Format(time.RFC3339)
		result[nodeID] = &domain.InstanceMetadata{
			ID:        id,
			CoreCount: coreCount,
			// Updates has no backing column on deployment_information --
			// deployed_product.update_level_info is a different, per-deployed-
			// product concept, not per-node. Left nil, same "no confirmed
			// source" precedent as that field elsewhere in this codebase.
			Updates:            nil,
			JDKVersion:         jdkVersion,
			DeploymentMetadata: info,
			CreatedOn:          createdOn.UTC().Format(time.RFC3339),
			UpdatedOn:          updatedOn.UTC().Format(time.RFC3339),
			CustomCreatedOn:    &customCreated,
			CustomUpdatedOn:    &customUpdated,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deployment_information: %w", err)
	}
	return result, nil
}

// SearchInstanceMetrics implements InstanceRepository.
func (r *instanceRepo) SearchInstanceMetrics(ctx context.Context, filters domain.InstanceDateRangeFilters) ([]domain.InstanceMetric, int, error) {
	where := "WHERE di.payload_updated_on::date BETWEEN $1::date AND $2::date"
	args := []any{filters.StartDate, filters.EndDate}
	clause, clauseArgs := instanceIDFilterClause(filters.ProjectIDs, filters.DeploymentIDs, filters.DeployedProductIDs, len(args)+1)
	where += clause
	args = append(args, clauseArgs...)

	query := fmt.Sprintf(
		`SELECT dn.id, dn.node_id, %s,
		        di.payload_updated_on, di.created_on, di.core_count, di.jdk_version, di.deployment_info
		 FROM deployment_node dn
		 JOIN deployment_information di ON di.node_id = dn.node_id
		 %s %s
		 ORDER BY dn.id, di.payload_updated_on DESC`,
		instanceRefColumns, instanceRefJoins, where,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query instance metrics: %w", err)
	}
	defer rows.Close()

	metricByInstance := map[string]*domain.InstanceMetric{}
	order := make([]string, 0)
	for rows.Next() {
		var instanceID, nodeID string
		var projID, projName, depID, depName, prodID, prodName, dprodID, dprodName *string
		var reportedUpdatedOn, createdOn time.Time
		var jdkVersion *string
		var coreCount *int
		var rawInfo []byte
		if err := rows.Scan(
			&instanceID, &nodeID, &projID, &projName, &depID, &depName, &prodID, &prodName, &dprodID, &dprodName,
			&reportedUpdatedOn, &createdOn, &coreCount, &jdkVersion, &rawInfo,
		); err != nil {
			return nil, 0, fmt.Errorf("scan instance metric: %w", err)
		}

		m, ok := metricByInstance[instanceID]
		if !ok {
			m = &domain.InstanceMetric{
				InstanceID:      instanceID,
				InstanceKey:     nodeID,
				Project:         buildOptionalRef(projID, projName),
				Deployment:      buildOptionalRef(depID, depName),
				Product:         buildOptionalRef(prodID, prodName),
				DeployedProduct: buildOptionalRef(dprodID, dprodName),
			}
			metricByInstance[instanceID] = m
			order = append(order, instanceID)
		}

		var info map[string]any
		if len(rawInfo) > 0 {
			if err := json.Unmarshal(rawInfo, &info); err != nil {
				return nil, 0, fmt.Errorf("parse deployment_info json for instance %q: %w", instanceID, err)
			}
		}
		m.DataPoints = append(m.DataPoints, domain.InstanceDataPoint{
			Date:               reportedUpdatedOn.UTC().Format("2006-01-02"),
			CreatedOn:          createdOn.UTC().Format(time.RFC3339),
			CoreCount:          coreCount,
			JDKVersion:         jdkVersion,
			Updates:            nil,
			DeploymentMetadata: info,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate instance metrics: %w", err)
	}

	metrics := make([]domain.InstanceMetric, 0, len(order))
	for _, id := range order {
		metrics = append(metrics, *metricByInstance[id])
	}
	return metrics, len(metrics), nil
}

// SearchInstanceUsage implements InstanceRepository.
func (r *instanceRepo) SearchInstanceUsage(ctx context.Context, filters domain.InstanceDateRangeFilters) ([]domain.InstanceUsageEntry, int, error) {
	where := "WHERE uc.counted_on::date BETWEEN $1::date AND $2::date"
	args := []any{filters.StartDate, filters.EndDate}
	clause, clauseArgs := instanceIDFilterClause(filters.ProjectIDs, filters.DeploymentIDs, filters.DeployedProductIDs, len(args)+1)
	where += clause
	args = append(args, clauseArgs...)

	query := fmt.Sprintf(
		`SELECT dn.id, dn.node_id, %s,
		        uc.counted_on::date, uc.count_type, SUM(uc.count)
		 FROM deployment_node dn
		 JOIN hourly_usage_summary uc ON uc.deployment_node_id = dn.id
		 %s %s
		 GROUP BY dn.id, dn.node_id, proj.id, proj.name, dep.id, dep.name, p.id, p.name, dprod.id, dprod.name,
		          uc.counted_on::date, uc.count_type
		 ORDER BY dn.id, uc.counted_on::date`,
		instanceRefColumns, instanceRefJoins, where,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query instance usage: %w", err)
	}
	defer rows.Close()

	entryByInstance := map[string]*domain.InstanceUsageEntry{}
	// summaryByInstance holds each instance's period summaries as pointers so
	// a later row for the same (instance, date) can keep filling the same
	// Counts map instead of overwriting a value copy already appended to
	// PeriodSummaries.
	summaryByInstance := map[string]map[string]*domain.InstanceSummary{}
	order := make([]string, 0)
	for rows.Next() {
		var instanceID, nodeID string
		var projID, projName, depID, depName, prodID, prodName, dprodID, dprodName *string
		var countedOn time.Time
		var countType string
		var count int
		if err := rows.Scan(
			&instanceID, &nodeID, &projID, &projName, &depID, &depName, &prodID, &prodName, &dprodID, &dprodName,
			&countedOn, &countType, &count,
		); err != nil {
			return nil, 0, fmt.Errorf("scan instance usage: %w", err)
		}

		entry, ok := entryByInstance[instanceID]
		if !ok {
			entry = &domain.InstanceUsageEntry{
				InstanceID:      instanceID,
				InstanceKey:     nodeID,
				Project:         buildOptionalRef(projID, projName),
				Deployment:      buildOptionalRef(depID, depName),
				Product:         buildOptionalRef(prodID, prodName),
				DeployedProduct: buildOptionalRef(dprodID, dprodName),
			}
			entryByInstance[instanceID] = entry
			summaryByInstance[instanceID] = map[string]*domain.InstanceSummary{}
			order = append(order, instanceID)
		}

		period := countedOn.UTC().Format("2006-01-02")
		summary, ok := summaryByInstance[instanceID][period]
		if !ok {
			summary = &domain.InstanceSummary{Period: period, Counts: map[string]int{}}
			summaryByInstance[instanceID][period] = summary
		}
		summary.Counts[countType] = count
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate instance usage: %w", err)
	}

	usages := make([]domain.InstanceUsageEntry, 0, len(order))
	for _, id := range order {
		entry := *entryByInstance[id]
		periods := make([]string, 0, len(summaryByInstance[id]))
		for p := range summaryByInstance[id] {
			periods = append(periods, p)
		}
		sort.Strings(periods)
		entry.PeriodSummaries = make([]domain.InstanceSummary, 0, len(periods))
		for _, p := range periods {
			entry.PeriodSummaries = append(entry.PeriodSummaries, *summaryByInstance[id][p])
		}
		usages = append(usages, entry)
	}
	return usages, len(usages), nil
}

// SearchInstanceMetricsStats implements InstanceRepository.
//
// deployment_information carries no data-source distinction at all (only
// daily_usage_summary does), so the caller (instance_service.go) rejects a
// non-nil DataSource filter before this is ever called rather than silently
// ignoring it.
func (r *instanceRepo) SearchInstanceMetricsStats(ctx context.Context, filters domain.InstanceDateRangeFilters) (domain.InstanceMetricsStatsResponse, error) {
	where := "WHERE di.payload_updated_on::date BETWEEN $1::date AND $2::date"
	args := []any{filters.StartDate, filters.EndDate}
	clause, clauseArgs := instanceIDFilterClause(filters.ProjectIDs, filters.DeploymentIDs, filters.DeployedProductIDs, len(args)+1)
	where += clause
	args = append(args, clauseArgs...)

	query := fmt.Sprintf(
		`SELECT di.payload_updated_on::date, dn.id, di.core_count
		 FROM deployment_node dn
		 JOIN deployment_information di ON di.node_id = dn.node_id
		 %s %s`,
		instanceRefJoins, where,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return domain.InstanceMetricsStatsResponse{}, fmt.Errorf("query instance metrics stats: %w", err)
	}
	defer rows.Close()

	dailyTotal := map[string]int{}
	instancesSeen := map[string]struct{}{}
	for rows.Next() {
		var day time.Time
		var instanceID string
		var cores *int
		if err := rows.Scan(&day, &instanceID, &cores); err != nil {
			return domain.InstanceMetricsStatsResponse{}, fmt.Errorf("scan instance metrics stats: %w", err)
		}
		instancesSeen[instanceID] = struct{}{}
		if cores == nil {
			continue
		}
		dailyTotal[day.UTC().Format("2006-01-02")] += *cores
	}
	if err := rows.Err(); err != nil {
		return domain.InstanceMetricsStatsResponse{}, fmt.Errorf("iterate instance metrics stats: %w", err)
	}

	dates := make([]string, 0, len(dailyTotal))
	for d := range dailyTotal {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	stats := make(map[string]map[string]int, len(dates))
	var summary domain.InstanceMetricSummary
	if len(dates) > 0 {
		min, max, sum := dailyTotal[dates[0]], dailyTotal[dates[0]], 0
		for _, d := range dates {
			v := dailyTotal[d]
			stats[d] = map[string]int{"CORES": v}
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
			sum += v
		}
		summary = domain.InstanceMetricSummary{
			Current: float64(dailyTotal[dates[len(dates)-1]]),
			Min:     float64(min),
			Max:     float64(max),
			Avg:     float64(sum) / float64(len(dates)),
		}
	}

	return domain.InstanceMetricsStatsResponse{
		Stats:     stats,
		Summary:   summary,
		Total:     len(instancesSeen),
		StartDate: filters.StartDate,
		EndDate:   filters.EndDate,
	}, nil
}

// SearchInstanceUsageStats implements InstanceRepository.
func (r *instanceRepo) SearchInstanceUsageStats(ctx context.Context, filters domain.InstanceDateRangeFilters, dataSource *string) (domain.InstanceUsageStatsResponse, error) {
	where := "WHERE dus.summary_date BETWEEN $1::date AND $2::date"
	args := []any{filters.StartDate, filters.EndDate}
	clause, clauseArgs := instanceIDFilterClause(filters.ProjectIDs, filters.DeploymentIDs, filters.DeployedProductIDs, len(args)+1)
	where += clause
	args = append(args, clauseArgs...)
	if dataSource != nil {
		where += fmt.Sprintf(" AND dus.data_source = $%d::usage_data_source_enum", len(args)+1)
		args = append(args, *dataSource)
	}

	query := fmt.Sprintf(
		// daily_usage_summary.usage_type was renamed to count_type after
		// this was first written (migration 000054 was edited in place
		// post-merge) -- matching hourly_usage_summary.count_type's own column name
		// for the same open-ended count-type concept.
		`SELECT dus.summary_date, dn.id, dus.count_type, SUM(dus.value)
		 FROM deployment_node dn
		 JOIN daily_usage_summary dus ON dus.deployment_node_id = dn.id
		 %s %s
		 GROUP BY dus.summary_date, dn.id, dus.count_type`,
		instanceRefJoins, where,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return domain.InstanceUsageStatsResponse{}, fmt.Errorf("query instance usage stats: %w", err)
	}
	defer rows.Close()

	stats := map[string]map[string]int{}
	instancesSeen := map[string]struct{}{}
	for rows.Next() {
		var day time.Time
		var instanceID string
		var usageType *string
		var value *int
		if err := rows.Scan(&day, &instanceID, &usageType, &value); err != nil {
			return domain.InstanceUsageStatsResponse{}, fmt.Errorf("scan instance usage stats: %w", err)
		}
		instancesSeen[instanceID] = struct{}{}
		if usageType == nil || value == nil {
			continue
		}
		d := day.UTC().Format("2006-01-02")
		if stats[d] == nil {
			stats[d] = map[string]int{}
		}
		stats[d][*usageType] += *value
	}
	if err := rows.Err(); err != nil {
		return domain.InstanceUsageStatsResponse{}, fmt.Errorf("iterate instance usage stats: %w", err)
	}

	return domain.InstanceUsageStatsResponse{
		Stats:     stats,
		Total:     len(instancesSeen),
		StartDate: filters.StartDate,
		EndDate:   filters.EndDate,
	}, nil
}
