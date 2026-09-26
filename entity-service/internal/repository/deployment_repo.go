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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
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
	// CreateDeploymentFromServiceNow inserts a deployment row using identity
	// (id/number) ServiceNow has already assigned -- see
	// deploymentService.createDeploymentSNFirst's own doc comment for why:
	// deployment.number is NOT NULL UNIQUE and Postgres has no generator for
	// it, the same unresolved problem case.number had before
	// CreateCaseFromServiceNow.
	CreateDeploymentFromServiceNow(ctx context.Context, req domain.CreateDeploymentRequest, id, number, createdBy string, createdOn time.Time) (domain.CreatedDeployment, error)
	// UpdateDeploymentFields applies a Postgres-side update for the same
	// field group UpdateDeploymentRequest itself enforces (detail fields XOR
	// Active=false) -- called on the DUAL_WRITE data source's Postgres-first
	// leg; the ServiceNow mirror runs separately and asynchronously.
	UpdateDeploymentFields(ctx context.Context, req domain.UpdateDeploymentRequest, updatedBy string) (domain.UpdatedDeployment, error)
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
	// is_active = TRUE: "deleting" a deployment (PATCH .../deployments/{id}
	// {"active": false}) deactivates it, it is never actually removed --
	// see UpdateDeploymentFields. SearchDeploymentsRequest has no
	// include-inactive filter in its wire contract at all (nor does the
	// ServiceNow-backed search expose one), so a deactivated deployment must
	// simply stop appearing here, permanently, the same as ServiceNow's own
	// listing already does for a deactivated record. Without this, "delete"
	// silently updated is_active in Postgres but the deployment kept
	// appearing in every search result exactly as before.
	where := "WHERE d.project_id IS NOT NULL AND d.is_active = TRUE"

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
			var deploymentType *string
			var creatorID, creatorName *string
			if err := rows.Scan(
				&d.ID, &d.Number, &d.Name, &deploymentType, &d.Description,
				&d.CreatedOn, &d.UpdatedOn,
				&creatorID, &creatorName,
				&d.Project.ID, &d.Project.Name,
			); err != nil {
				return fmt.Errorf("scan deployment: %w", err)
			}
			// deployment.type (migration 000013) has no NOT NULL constraint --
			// 38 of 2859 rows are NULL on staging, checked live -- but
			// DeploymentView.Type is a required (non-pointer) field on the
			// wire, matching the OpenAPI contract every consumer already
			// expects. Same "keep the wire type required, fix the scan side
			// only" precedent as CaseView.InternalID (see this file's own
			// history for why a pointer wire type isn't the answer here
			// either): default to "" rather than crashing the whole search.
			//
			// ToLower: deployment_type_enum's Postgres labels are UPPER_SNAKE
			// ("STAGING"), but domain.DeploymentType's canonical form is
			// lowercase (domain/entity.go's DeploymentTypeStaging = "staging"
			// etc.) -- without this, every DeploymentView.Type read back from
			// Postgres carried the wrong case, which backend-v2's
			// deploymentTypeRef (keyed lowercase) would silently fail to
			// resolve to a numeric id for.
			d.Type = domain.DeploymentType(strings.ToLower(stringOrEmpty(deploymentType)))
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

const createDeploymentFromServiceNowQuery = `
	INSERT INTO deployment (
		id, created_on, updated_on, created_by, updated_by,
		number, name, description, type, is_active, project_id
	)
	VALUES (
		$1, $2, $2, $3, $3,
		$4, $5, NULLIF($6, ''), $7::deployment_type_enum, TRUE, $8
	)
	RETURNING id, created_on, created_by`

// CreateDeploymentFromServiceNow implements DeploymentRepository.
func (r *deploymentRepo) CreateDeploymentFromServiceNow(ctx context.Context, req domain.CreateDeploymentRequest, id, number, createdBy string, createdOn time.Time) (domain.CreatedDeployment, error) {
	// deployment_type_enum's Postgres labels are UPPER_SNAKE ("STAGING"), but
	// domain.DeploymentType's canonical form is lowercase ("staging",
	// domain/entity.go's DeploymentTypeStaging etc.) -- ToUpper before the
	// cast, same convention case_repo.go's own inserts follow for every
	// domain enum they write (e.g. strings.ToUpper(string(req.IssueType))).
	var deploymentType string
	if req.Type != nil {
		deploymentType = strings.ToUpper(string(*req.Type))
	}

	var created domain.CreatedDeployment
	err := r.db.QueryRow(ctx, createDeploymentFromServiceNowQuery,
		id, createdOn, createdBy,
		number, req.Name, req.Description, deploymentType, req.ProjectID,
	).Scan(&created.ID, &created.CreatedOn, &created.CreatedBy)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation -- number or id already exists
				return domain.CreatedDeployment{}, &apierror.ValidationError{Msg: "a deployment with this identity already exists"}
			case "23503": // foreign_key_violation -- project_id does not exist
				return domain.CreatedDeployment{}, &apierror.ValidationError{Msg: "projectId does not exist"}
			}
		}
		return domain.CreatedDeployment{}, fmt.Errorf("create deployment from servicenow: %w", err)
	}
	return created, nil
}

const updateDeploymentFieldsQuery = `
	UPDATE deployment SET
		updated_on = NOW(), updated_by = $2,
		name = COALESCE($3, name),
		type = COALESCE($4::deployment_type_enum, type),
		description = CASE WHEN $5 THEN $6 ELSE description END,
		is_active = COALESCE($7, is_active)
	WHERE id = $1
	RETURNING id, updated_on, updated_by`

// UpdateDeploymentFields implements DeploymentRepository.
//
// req.Description is **string -- nil means "not provided" (COALESCE keeps
// the existing value via the CASE below, matching every other field here),
// a non-nil pointer to nil means "clear it" ($6 bound as NULL with
// descriptionProvided true), and a pointer to a value means "set it". A
// plain COALESCE($6, description) cannot distinguish "not provided" from
// "explicitly clear" -- the CASE/boolean-flag pair is what does.
func (r *deploymentRepo) UpdateDeploymentFields(ctx context.Context, req domain.UpdateDeploymentRequest, updatedBy string) (domain.UpdatedDeployment, error) {
	// See CreateDeploymentFromServiceNow's identical comment: deployment_type_enum
	// is UPPER_SNAKE in Postgres, domain.DeploymentType is lowercase.
	var deploymentType *string
	if req.Type != nil {
		t := strings.ToUpper(string(*req.Type))
		deploymentType = &t
	}

	descriptionProvided := req.Description != nil
	var description *string
	if descriptionProvided {
		description = *req.Description
	}

	var updated domain.UpdatedDeployment
	err := r.db.QueryRow(ctx, updateDeploymentFieldsQuery,
		req.ID, updatedBy,
		req.Name, deploymentType,
		descriptionProvided, description,
		req.Active,
	).Scan(&updated.ID, &updated.UpdatedOn, &updated.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.UpdatedDeployment{}, &apierror.NotFoundError{Msg: "deployment not found"}
	}
	if err != nil {
		return domain.UpdatedDeployment{}, fmt.Errorf("update deployment fields: %w", err)
	}
	return updated, nil
}
