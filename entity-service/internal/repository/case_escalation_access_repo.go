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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// CaseDeescalatorRole is the stored role whose holders may de-escalate any case.
const CaseDeescalatorRole = "case_deescalator"

const caseDeescalatorRoleDescription = "May de-escalate any case, besides the leads of the case's ABT team"

// CaseEscalationAccessRepository reads and changes who holds the
// case_deescalator role.
type CaseEscalationAccessRepository interface {
	// ListDeescalators returns every holder of the role, one row per person.
	ListDeescalators(ctx context.Context) ([]domain.CaseDeescalatorGrant, error)
	// GrantDeescalator gives userID the role. false when they already had it.
	GrantDeescalator(ctx context.Context, userID, actorEmail string) (bool, error)
	// RevokeDeescalator removes every copy of the role from userID. false when
	// they did not hold it.
	RevokeDeescalator(ctx context.Context, userID, actorEmail string) (bool, error)
	// UserIsInternal is whether userID is an active internal user.
	UserIsInternal(ctx context.Context, userID string) (bool, error)
	// IsDeactivated is whether a user with this email exists and has been
	// deactivated (is_active = FALSE). No such user, or an unset flag, is not.
	IsDeactivated(ctx context.Context, userEmail string) (bool, error)
}

type caseEscalationAccessRepository struct {
	db *pgxpool.Pool
}

// NewCaseEscalationAccessRepository constructs a CaseEscalationAccessRepository.
func NewCaseEscalationAccessRepository(db *pgxpool.Pool) CaseEscalationAccessRepository {
	return &caseEscalationAccessRepository{db: db}
}

// ListDeescalators implements CaseEscalationAccessRepository.
func (r *caseEscalationAccessRepository) ListDeescalators(ctx context.Context) ([]domain.CaseDeescalatorGrant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (u.id)
		       u.id::text, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), ''),
		       COALESCE(u.email, ''), COALESCE(ur.created_by, ''), ur.created_on,
		       u.user_type = 'INTERNAL'::user_type_enum AND COALESCE(u.is_active, TRUE)
		  FROM user_role ur
		  JOIN role ro  ON ro.id = ur.role_id
		  JOIN "user" u ON u.id = ur.user_id
		 WHERE ro.name = $1
		 ORDER BY u.id, ur.created_on`, CaseDeescalatorRole)
	if err != nil {
		return nil, fmt.Errorf("list case de-escalators: %w", err)
	}
	defer rows.Close()
	out := []domain.CaseDeescalatorGrant{}
	for rows.Next() {
		var g domain.CaseDeescalatorGrant
		if err := rows.Scan(&g.UserID, &g.Name, &g.Email, &g.GrantedBy, &g.GrantedOn, &g.Active); err != nil {
			return nil, fmt.Errorf("scan case de-escalator: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// inTx runs fn in a transaction that names the actor for the audit trail.
func (r *caseEscalationAccessRepository) inTx(ctx context.Context, actorEmail string, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GrantDeescalator implements CaseEscalationAccessRepository. The role row is
// created here the first time it is needed (no migration seeds it), by name:
// ON CONFLICT (name) leaves an existing row alone, including one the
// ServiceNow sync created under another id. "Not already held" is asked in
// the insert itself, since user_role has no unique key to conflict on.
func (r *caseEscalationAccessRepository) GrantDeescalator(ctx context.Context, userID, actorEmail string) (bool, error) {
	var added bool
	err := r.inTx(ctx, actorEmail, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO role (id, created_on, updated_on, created_by, updated_by, name, description)
			VALUES (gen_random_uuid(), now(), now(), $1, $1, $2, $3)
			ON CONFLICT (name) DO NOTHING`,
			actorEmail, CaseDeescalatorRole, caseDeescalatorRoleDescription); err != nil {
			return fmt.Errorf("create role %s: %w", CaseDeescalatorRole, err)
		}
		var roleID string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM role WHERE name = $1`, CaseDeescalatorRole).Scan(&roleID); err != nil {
			return fmt.Errorf("look up role %s: %w", CaseDeescalatorRole, err)
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
			SELECT gen_random_uuid(), now(), now(), $3, $3, $1::uuid, $2::uuid
			 WHERE NOT EXISTS (SELECT 1 FROM user_role WHERE user_id = $1::uuid AND role_id = $2::uuid)`,
			userID, roleID, actorEmail)
		if err != nil {
			return fmt.Errorf("grant %s: %w", CaseDeescalatorRole, err)
		}
		added = tag.RowsAffected() > 0
		return nil
	})
	return added, err
}

// RevokeDeescalator implements CaseEscalationAccessRepository. Every copy
// goes, since duplicates exist and any one left behind would keep the role.
func (r *caseEscalationAccessRepository) RevokeDeescalator(ctx context.Context, userID, actorEmail string) (bool, error) {
	var removed bool
	err := r.inTx(ctx, actorEmail, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			DELETE FROM user_role ur USING role ro
			 WHERE ur.role_id = ro.id AND ur.user_id = $1::uuid AND ro.name = $2`, userID, CaseDeescalatorRole)
		if err != nil {
			return fmt.Errorf("revoke %s: %w", CaseDeescalatorRole, err)
		}
		removed = tag.RowsAffected() > 0
		return nil
	})
	return removed, err
}

// UserIsInternal implements CaseEscalationAccessRepository.
func (r *caseEscalationAccessRepository) UserIsInternal(ctx context.Context, userID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM "user"
		                WHERE id = $1::uuid
		                  AND user_type = 'INTERNAL'::user_type_enum
		                  AND COALESCE(is_active, TRUE))`, userID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check internal user: %w", err)
	}
	return ok, nil
}

// IsDeactivated implements CaseEscalationAccessRepository.
func (r *caseEscalationAccessRepository) IsDeactivated(ctx context.Context, userEmail string) (bool, error) {
	var gone bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM "user" WHERE lower(email) = lower($1) AND is_active = FALSE)`, userEmail).Scan(&gone)
	if err != nil {
		return false, fmt.Errorf("check deactivated user: %w", err)
	}
	return gone, nil
}
