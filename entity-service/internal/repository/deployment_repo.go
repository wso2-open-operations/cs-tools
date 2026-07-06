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

// DeploymentRepository defines the persistence operations for the deployments table.
type DeploymentRepository interface {
	// SearchDeployments returns a filtered, paginated slice of enriched deployment
	// views together with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchDeployments(ctx context.Context, req domain.SearchDeploymentsRequest) ([]domain.DeploymentView, int, error)
}

type deploymentRepo struct {
	db *pgxpool.Pool
}

// NewDeploymentRepository constructs a DeploymentRepository backed by the given connection pool.
func NewDeploymentRepository(db *pgxpool.Pool) DeploymentRepository {
	return &deploymentRepo{db: db}
}

// SearchDeployments implements DeploymentRepository.
func (r *deploymentRepo) SearchDeployments(ctx context.Context, req domain.SearchDeploymentsRequest) ([]domain.DeploymentView, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	if len(req.ProjectIDs) > 0 {
		// Cast the parameter to uuid[] so the column stays uncast and idx_deployments_project_id is usable.
		where += fmt.Sprintf(" AND d.project_id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.ProjectIDs)
		argIdx++
	}

	if len(req.DeploymentTypes) > 0 {
		// Convert []DeploymentType to []string — pgx has no codec for named string types.
		// Cast the parameter to deployment_type_enum[] so the column stays uncast and idx_deployments_type is usable.
		typeStrings := make([]string, len(req.DeploymentTypes))
		for i, t := range req.DeploymentTypes {
			typeStrings[i] = string(t)
		}
		where += fmt.Sprintf(" AND d.type = ANY($%d::deployment_type_enum[])", argIdx)
		filterArgs = append(filterArgs, typeStrings)
		argIdx++
	}

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (d.name ILIKE $%d ESCAPE '\\')", argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM deployments d " + where

	dataQuery := fmt.Sprintf(
		`SELECT d.id, d.number, d.name, d.type::TEXT, d.description,
		        d.created_at, d.updated_at,
		        u.id, u.first_name || ' ' || u.last_name,
		        p.id, p.name
		 FROM deployments d
		 JOIN users u    ON d.created_by  = u.id
		 JOIN projects p ON d.project_id  = p.id
		 %s
		 ORDER BY d.created_at DESC, d.id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var deployments []domain.DeploymentView

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count deployments: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query deployments: %w", err)
		}
		defer rows.Close()

		result := make([]domain.DeploymentView, 0, req.Pagination.Limit)
		for rows.Next() {
			var d domain.DeploymentView
			d.CreatedBy = &domain.EntityRef{}
			if err := rows.Scan(
				&d.ID, &d.Number, &d.Name, &d.Type, &d.Description,
				&d.CreatedOn, &d.UpdatedOn,
				&d.CreatedBy.ID, &d.CreatedBy.Name,
				&d.Project.ID, &d.Project.Name,
			); err != nil {
				return fmt.Errorf("scan deployment: %w", err)
			}
			result = append(result, d)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate deployments: %w", err)
		}
		deployments = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return deployments, total, nil
}
