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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// CaseCommentTemplateRepository persists saved case comment templates.
//
// ownerUserID is a personal template's owner, or nil for the shared
// templates. Every method that touches one template matches the owner as well
// as the id (owner_user_id IS NOT DISTINCT FROM ownerUserID), so a personal
// call can never reach another user's template or a shared one, and a shared
// call can never reach a personal one: the row is simply not found.
type CaseCommentTemplateRepository interface {
	// List is userID's personal templates, then the shared ones, each by name.
	List(ctx context.Context, userID string) ([]domain.CaseCommentTemplate, error)
	Count(ctx context.Context, ownerUserID *string) (int, error)
	Create(ctx context.Context, ownerUserID *string, req domain.CreateCaseCommentTemplateRequest, actorEmail string) (domain.CaseCommentTemplate, error)
	Update(ctx context.Context, id string, ownerUserID *string, req domain.UpdateCaseCommentTemplateRequest, actorEmail string) (domain.CaseCommentTemplate, error)
	Delete(ctx context.Context, id string, ownerUserID *string) error
}

type caseCommentTemplateRepo struct {
	db *pgxpool.Pool
}

// NewCaseCommentTemplateRepository constructs a Postgres-backed repository.
func NewCaseCommentTemplateRepository(db *pgxpool.Pool) CaseCommentTemplateRepository {
	return &caseCommentTemplateRepo{db: db}
}

const caseCommentTemplateColumns = `id::text, name, body, applies_to, owner_user_id IS NULL, created_on, updated_on`

func scanCaseCommentTemplate(row pgx.Row) (domain.CaseCommentTemplate, error) {
	var t domain.CaseCommentTemplate
	var appliesTo string
	err := row.Scan(&t.ID, &t.Name, &t.Body, &appliesTo, &t.Shared, &t.CreatedOn, &t.UpdatedOn)
	t.AppliesTo = domain.CommentTemplateAppliesTo(appliesTo)
	return t, err
}

// caseCommentTemplateNameConflict maps the (owner, lower(name)) unique index
// to a 409 that names the clash; nil for any other error.
func caseCommentTemplateNameConflict(err error, name string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "case_comment_template_owner_name" {
		return &apierror.ConflictError{Msg: "a template named " + name + " already exists"}
	}
	return nil
}

func (r *caseCommentTemplateRepo) List(ctx context.Context, userID string) ([]domain.CaseCommentTemplate, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+caseCommentTemplateColumns+`
		   FROM case_comment_template
		  WHERE owner_user_id = $1::uuid OR owner_user_id IS NULL
		  ORDER BY owner_user_id IS NULL, LOWER(name), id`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("list case comment templates: %w", err)
	}
	defer rows.Close()
	templates := []domain.CaseCommentTemplate{}
	for rows.Next() {
		t, err := scanCaseCommentTemplate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan case comment template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list case comment templates: %w", err)
	}
	return templates, nil
}

func (r *caseCommentTemplateRepo) Count(ctx context.Context, ownerUserID *string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM case_comment_template WHERE owner_user_id IS NOT DISTINCT FROM $1::uuid`,
		ownerUserID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count case comment templates: %w", err)
	}
	return n, nil
}

func (r *caseCommentTemplateRepo) Create(ctx context.Context, ownerUserID *string, req domain.CreateCaseCommentTemplateRequest, actorEmail string) (domain.CaseCommentTemplate, error) {
	t, err := scanCaseCommentTemplate(r.db.QueryRow(ctx,
		`INSERT INTO case_comment_template (owner_user_id, name, body, applies_to, created_by, updated_by)
		 VALUES ($1::uuid, $2, $3, $4, $5, $5)
		 RETURNING `+caseCommentTemplateColumns,
		ownerUserID, req.Name, req.Body, string(req.AppliesTo), actorEmail))
	if err != nil {
		if conflict := caseCommentTemplateNameConflict(err, req.Name); conflict != nil {
			return domain.CaseCommentTemplate{}, conflict
		}
		return domain.CaseCommentTemplate{}, fmt.Errorf("create case comment template: %w", err)
	}
	return t, nil
}

func (r *caseCommentTemplateRepo) Update(ctx context.Context, id string, ownerUserID *string, req domain.UpdateCaseCommentTemplateRequest, actorEmail string) (domain.CaseCommentTemplate, error) {
	var appliesTo *string
	if req.AppliesTo != nil {
		s := string(*req.AppliesTo)
		appliesTo = &s
	}
	t, err := scanCaseCommentTemplate(r.db.QueryRow(ctx,
		`UPDATE case_comment_template
		    SET name       = COALESCE($3, name),
		        body       = COALESCE($4, body),
		        applies_to = COALESCE($5, applies_to),
		        updated_on = NOW(),
		        updated_by = $6
		  WHERE id = $1::uuid AND owner_user_id IS NOT DISTINCT FROM $2::uuid
		 RETURNING `+caseCommentTemplateColumns,
		id, ownerUserID, req.Name, req.Body, appliesTo, actorEmail))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CaseCommentTemplate{}, &apierror.NotFoundError{Msg: "comment template not found"}
	}
	if err != nil {
		name := ""
		if req.Name != nil {
			name = *req.Name
		}
		if conflict := caseCommentTemplateNameConflict(err, name); conflict != nil {
			return domain.CaseCommentTemplate{}, conflict
		}
		return domain.CaseCommentTemplate{}, fmt.Errorf("update case comment template: %w", err)
	}
	return t, nil
}

func (r *caseCommentTemplateRepo) Delete(ctx context.Context, id string, ownerUserID *string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM case_comment_template WHERE id = $1::uuid AND owner_user_id IS NOT DISTINCT FROM $2::uuid`,
		id, ownerUserID)
	if err != nil {
		return fmt.Errorf("delete case comment template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "comment template not found"}
	}
	return nil
}
