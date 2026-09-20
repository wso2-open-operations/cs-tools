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

// ProjectRepository defines the persistence operations for the project
// table (migration 000009). domain.Project.SubscriptionType/ClosureStatus
// and domain.ProjectAccountRef.Tier have no corresponding column anywhere in
// the migrations (SubscriptionType/ClosureStatus are ServiceNow vocabulary
// with values -- e.g. "managed_cloud_subscription", "read_only" -- that
// don't match any of project's several different closure-state columns;
// account has no tier-like column at all), so they are left as their zero
// value rather than guessed at. AgentEnabled/KbReferencesEnabled DO have a
// clear real-column match (account.ai_gen_response_enabled/
// smart_knowledge_base_suggestions_enabled) despite the name difference and
// are populated from them.
type ProjectRepository interface {
	// SearchProjects returns a filtered, paginated slice of projects together
	// with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchProjects(ctx context.Context, req domain.SearchProjectsRequest) ([]domain.Project, int, error)
	// GetProjectByID returns the enriched project detail with the linked account,
	// or a NotFoundError if no such project exists.
	GetProjectByID(ctx context.Context, id string) (domain.ProjectDetailsView, error)
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
		where += fmt.Sprintf(" AND (name ILIKE $%d ESCAPE '\\' OR key ILIKE $%d ESCAPE '\\')", argIdx, argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM project " + where

	dataQuery := fmt.Sprintf(
		`SELECT id, account_id, sf_id, name, key,
		        start_date, end_date, created_on, updated_on
		 FROM project %s
		 ORDER BY created_on DESC, id
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
			// account_id/start_date/end_date are nullable (migration 000009);
			// domain.Project's fields are pointers to match -- see that
			// struct's own doc comment. A non-pointer scan here used to
			// error "cannot scan NULL into *time.Time" the moment any of the
			// 13-14 (of 1956) rows with a NULL date reached this query.
			if err := rows.Scan(
				&p.ID, &p.AccountID, &p.SfID, &p.Name, &p.Key,
				&p.StartDate, &p.EndDate, &p.CreatedOn, &p.UpdatedOn,
			); err != nil {
				return fmt.Errorf("scan project: %w", err)
			}
			// SubscriptionType/ClosureStatus have no real column -- see this
			// repository's own doc comment.
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
func (r *projectRepo) GetProjectByID(ctx context.Context, id string) (domain.ProjectDetailsView, error) {
	var v domain.ProjectDetailsView
	// account.ai_gen_response_enabled/smart_knowledge_base_suggestions_enabled
	// are nullable BOOLEAN columns, but ProjectAccountRef.AgentEnabled/
	// KbReferencesEnabled are plain (non-pointer) bool -- scan into *bool
	// and treat a NULL column as false, not an error.
	var agentEnabled, kbReferencesEnabled *bool
	// account is a LEFT JOIN, not an INNER JOIN: project.account_id
	// (migration 000009) is nullable and genuinely NULL on live data (14 of
	// 1956 rows) -- an INNER JOIN here used to make every such project
	// invisible (zero rows -> misreported as 404 "project not found"), the
	// same class of false-404 GetCaseByID had for its own optional joins
	// before that was fixed (see this file's "Case-like work_item types"
	// history). aID/aName are scanned nullable for the same reason;
	// ActivationDate/Region are already pointer fields on ProjectAccountRef
	// so they tolerate NULL (whether from a real account or a LEFT JOIN
	// producing no row at all) without a separate local var.
	var aID, aName *string
	err := r.db.QueryRow(ctx,
		`SELECT p.id, p.sf_id, p.name, p.key,
		        p.start_date, p.end_date, p.created_on, p.updated_on,
		        a.id, a.name, a.activation_date, a.region,
		        a.ai_gen_response_enabled, a.smart_knowledge_base_suggestions_enabled
		 FROM project p
		 LEFT JOIN account a ON p.account_id = a.id
		 WHERE p.id = $1`, id,
	).Scan(
		&v.ID, &v.SfID, &v.Name, &v.Key,
		&v.StartDate, &v.EndDate, &v.CreatedOn, &v.UpdatedOn,
		&aID, &aName, &v.Account.ActivationDate, &v.Account.Region,
		&agentEnabled, &kbReferencesEnabled,
	)
	// v.SubscriptionType and v.Account.Tier have no real column -- see this
	// repository's own doc comment; left as their zero value.
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProjectDetailsView{}, &apierror.NotFoundError{Msg: "project not found"}
	}
	if err != nil {
		return domain.ProjectDetailsView{}, fmt.Errorf("get project by id: %w", err)
	}
	if aID != nil {
		v.Account.ID = *aID
	}
	if aName != nil {
		v.Account.Name = *aName
	}
	v.Account.AgentEnabled = agentEnabled != nil && *agentEnabled
	v.Account.KbReferencesEnabled = kbReferencesEnabled != nil && *kbReferencesEnabled
	return v, nil
}
