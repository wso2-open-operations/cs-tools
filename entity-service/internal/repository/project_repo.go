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
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ProjectRepository defines the persistence operations for the projects table.
type ProjectRepository interface {
	// SearchProjects returns a filtered, paginated slice of projects together
	// with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchProjects(ctx context.Context, req domain.SearchProjectsRequest) ([]domain.Project, int, error)
	// GetProjectByID returns the project with the given UUID, or a NotFoundError
	// if no such project exists.
	GetProjectByID(ctx context.Context, id string) (domain.Project, error)
}

type projectRepo struct {
	db *pgxpool.Pool
}

// NewProjectRepository constructs a ProjectRepository backed by the given connection pool.
func NewProjectRepository(db *pgxpool.Pool) ProjectRepository {
	return &projectRepo{db: db}
}

// SearchProjects implements ProjectRepository.
func (r *projectRepo) SearchProjects(ctx context.Context, req domain.SearchProjectsRequest) ([]domain.Project, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		// All three ILIKE branches reference the same positional parameter — PostgreSQL
		// allows a single $N to appear multiple times in one query.
		where += fmt.Sprintf(
			" AND (name ILIKE $%d ESCAPE '\\' OR project_key ILIKE $%d ESCAPE '\\' OR subscription_type::TEXT ILIKE $%d ESCAPE '\\')",
			argIdx, argIdx, argIdx,
		)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM projects " + where

	dataQuery := fmt.Sprintf(
		`SELECT id, account_id, sf_id, name, project_key, subscription_type,
		        start_date, end_date, created_at, updated_at
		 FROM projects %s
		 ORDER BY created_at DESC, id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var projects []domain.Project

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count projects: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query projects: %w", err)
		}
		defer rows.Close()

		result := make([]domain.Project, 0, req.Pagination.Limit)
		for rows.Next() {
			var p domain.Project
			if err := rows.Scan(
				&p.ID, &p.AccountID, &p.SfID, &p.Name, &p.ProjectKey, &p.SubscriptionType,
				&p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt,
			); err != nil {
				return fmt.Errorf("scan project: %w", err)
			}
			result = append(result, p)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate projects: %w", err)
		}
		projects = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

// GetProjectByID implements ProjectRepository.
func (r *projectRepo) GetProjectByID(ctx context.Context, id string) (domain.Project, error) {
	var p domain.Project
	err := r.db.QueryRow(ctx,
		`SELECT id, account_id, sf_id, name, project_key, subscription_type,
		        start_date, end_date, created_at, updated_at
		 FROM projects WHERE id = $1`, id,
	).Scan(
		&p.ID, &p.AccountID, &p.SfID, &p.Name, &p.ProjectKey, &p.SubscriptionType,
		&p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, &apierror.NotFoundError{Msg: "project not found"}
	}
	if err != nil {
		return domain.Project{}, fmt.Errorf("get project by id: %w", err)
	}
	return p, nil
}
