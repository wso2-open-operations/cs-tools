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

// DeploymentRepository defines the persistence operations for the
// deployment table (migration 000013).
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

	// deployment.project_id is nullable (migration 000013 sets it NULL when
	// the owning project is deleted). The data query below inner-joins
	// project and so can never return such a row; without this predicate
	// the count query would still include it, inflating total relative to
	// what's actually returned. DeploymentView.Project is a non-pointer
	// EntityRef, so switching the join to LEFT instead isn't a safe
	// alternative -- that would need a response-contract change (a nullable
	// Project field) and nullable scan handling, not just a query fix.
	where := "WHERE d.project_id IS NOT NULL"

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

	countQuery := "SELECT COUNT(*) FROM deployment d " + where

	// deployment.created_by is a plain VARCHAR audit string (an email, by
	// this codebase's own convention -- see e.g. commentService writing the
	// caller's resolved email into comment.created_by), never a UUID FK
	// into "user". A plain "= u.id" join here would either fail to
	// type-check or silently match nothing. Resolve it by email instead,
	// LEFT JOIN so a deployment created by an unrecognized identity still
	// returns a row -- CreatedBy comes back nil rather than a fabricated
	// EntityRef with an empty id (see the domain package's own
	// "empty strings must never appear" convention).
	dataQuery := fmt.Sprintf(
		`SELECT d.id, d.number, d.name, d.type::TEXT, d.description,
		        d.created_on, d.updated_on,
		        u.id, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), '')),
		        p.id, p.name
		 FROM deployment d
		 LEFT JOIN "user" u ON LOWER(u.email) = LOWER(d.created_by)
		 JOIN project p ON d.project_id = p.id
		 %s
		 ORDER BY d.created_on DESC, d.id
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
			var creatorID, creatorName *string
			if err := rows.Scan(
				&d.ID, &d.Number, &d.Name, &d.Type, &d.Description,
				&d.CreatedOn, &d.UpdatedOn,
				&creatorID, &creatorName,
				&d.Project.ID, &d.Project.Name,
			); err != nil {
				return fmt.Errorf("scan deployment: %w", err)
			}
			if creatorID != nil {
				name := ""
				if creatorName != nil {
					name = *creatorName
				}
				d.CreatedBy = &domain.EntityRef{ID: *creatorID, Name: name}
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
