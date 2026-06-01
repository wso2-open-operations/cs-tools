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

// CaseRepository defines the persistence operations for the cases table.
type CaseRepository interface {
	// SearchCases returns a filtered, paginated slice of cases together with the
	// total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchCases(ctx context.Context, req domain.SearchCasesRequest) ([]domain.Case, int, error)
}

type caseRepo struct {
	db *pgxpool.Pool
}

// NewCaseRepository constructs a CaseRepository backed by the given connection pool.
func NewCaseRepository(db *pgxpool.Pool) CaseRepository {
	return &caseRepo{db: db}
}

// SearchCases implements CaseRepository.
func (r *caseRepo) SearchCases(ctx context.Context, req domain.SearchCasesRequest) ([]domain.Case, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	if len(req.ProjectIDs) > 0 {
		where += fmt.Sprintf(" AND project_id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.ProjectIDs)
		argIdx++
	}

	if len(req.DeploymentIDs) > 0 {
		where += fmt.Sprintf(" AND deployment_id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.DeploymentIDs)
		argIdx++
	}

	if len(req.DeployedProductIDs) > 0 {
		where += fmt.Sprintf(" AND deployed_product_id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.DeployedProductIDs)
		argIdx++
	}

	if len(req.StateKeys) > 0 {
		stateStrings := make([]string, len(req.StateKeys))
		for i, s := range req.StateKeys {
			stateStrings[i] = string(s)
		}
		where += fmt.Sprintf(" AND state = ANY($%d::case_state_enum[])", argIdx)
		filterArgs = append(filterArgs, stateStrings)
		argIdx++
	}

	if len(req.PriorityKeys) > 0 {
		priorityStrings := make([]string, len(req.PriorityKeys))
		for i, p := range req.PriorityKeys {
			priorityStrings[i] = string(p)
		}
		where += fmt.Sprintf(" AND priority = ANY($%d::case_priority_enum[])", argIdx)
		filterArgs = append(filterArgs, priorityStrings)
		argIdx++
	}

	if len(req.IssueTypeKeys) > 0 {
		issueTypeStrings := make([]string, len(req.IssueTypeKeys))
		for i, it := range req.IssueTypeKeys {
			issueTypeStrings[i] = string(it)
		}
		where += fmt.Sprintf(" AND issue_type = ANY($%d::case_issue_type_enum[])", argIdx)
		filterArgs = append(filterArgs, issueTypeStrings)
		argIdx++
	}

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(` AND (subject ILIKE $%d ESCAPE '\' OR number ILIKE $%d ESCAPE '\' OR wso2_id ILIKE $%d ESCAPE '\')`, argIdx, argIdx, argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	sortCol := string(req.SortBy.Field)
	sortDir := string(req.SortBy.Order)

	countQuery := "SELECT COUNT(*) FROM cases " + where

	dataQuery := fmt.Sprintf(
		`SELECT id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		        subject, description, priority, issue_type, state, created_at, updated_at, closed_at
		 FROM cases %s
		 ORDER BY %s %s NULLS LAST, id
		 LIMIT $%d OFFSET $%d`,
		where, sortCol, sortDir, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var cases []domain.Case

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count cases: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query cases: %w", err)
		}
		defer rows.Close()

		result := make([]domain.Case, 0, req.Pagination.Limit)
		for rows.Next() {
			var c domain.Case
			if err := rows.Scan(
				&c.ID, &c.Number, &c.Wso2ID, &c.CreatedBy,
				&c.ProjectID, &c.DeploymentID, &c.DeployedProductID,
				&c.Subject, &c.Description, &c.Priority, &c.IssueType, &c.State,
				&c.CreatedAt, &c.UpdatedAt, &c.ClosedAt,
			); err != nil {
				return fmt.Errorf("scan case: %w", err)
			}
			result = append(result, c)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate cases: %w", err)
		}
		cases = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return cases, total, nil
}
