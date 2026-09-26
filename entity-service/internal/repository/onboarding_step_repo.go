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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// OnboardingStepRepository persists the per-membership, per-step outcome of
// the customer onboarding flow (table onboarding_step, migration 000075).
type OnboardingStepRepository interface {
	// Upsert writes the latest outcome of one step for one membership. The
	// (membership_sf_id, step) pair is unique: a repeat updates the row and
	// increments attempt_count.
	Upsert(ctx context.Context, req domain.UpsertOnboardingStepRequest) (domain.OnboardingStep, error)
	// GetByMembership returns every step recorded for a membership, in step
	// order; an unknown membership yields an empty slice, not an error.
	GetByMembership(ctx context.Context, membershipSfID string) ([]domain.OnboardingStep, error)
	// Search returns a filtered, paginated slice of steps plus the total match
	// count, newest first.
	Search(ctx context.Context, req domain.SearchOnboardingStepsRequest) ([]domain.OnboardingStep, int, error)
}

type onboardingStepRepo struct {
	db *pgxpool.Pool
}

// NewOnboardingStepRepository constructs an OnboardingStepRepository backed by the pool.
func NewOnboardingStepRepository(db *pgxpool.Pool) OnboardingStepRepository {
	return &onboardingStepRepo{db: db}
}

const onboardingStepColumns = `
	id, membership_sf_id, contact_sf_id, email, project_id, project_contact_id,
	step::TEXT, status::TEXT, attempt_count, last_error, event_type, event_modified_on,
	created_on, updated_on`

func scanOnboardingStep(row pgx.Row) (domain.OnboardingStep, error) {
	var s domain.OnboardingStep
	var step, status string
	if err := row.Scan(
		&s.ID, &s.MembershipSfID, &s.ContactSfID, &s.Email, &s.ProjectID, &s.ProjectContactID,
		&step, &status, &s.AttemptCount, &s.LastError, &s.EventType, &s.EventModifiedOn,
		&s.CreatedOn, &s.UpdatedOn,
	); err != nil {
		return domain.OnboardingStep{}, err
	}
	s.Step = domain.OnboardingStepName(step)
	s.Status = domain.OnboardingStepStatus(status)
	return s, nil
}

func (r *onboardingStepRepo) Upsert(ctx context.Context, req domain.UpsertOnboardingStepRequest) (domain.OnboardingStep, error) {
	return upsertOnboardingStep(ctx, r.db, req)
}

// upsertOnboardingStep is shared with the membership upsert, which records the
// DATABASE step inside its own transaction (q is then that transaction).
//
// Retries and out-of-order deliveries hit the same (membership_sf_id, step)
// row, so the outcome columns (status, last_error, event_type,
// event_modified_on) only move when the incoming event is at least as new as
// the recorded one, or when the recorded row was stamped by a DELETED event
// (an undelete keeps the Salesforce LastModifiedDate, and the row must be
// allowed to leave that state). attempt_count and the audit columns advance
// on every write so a stale retry is still visible.
func upsertOnboardingStep(ctx context.Context, q querier, req domain.UpsertOnboardingStepRequest) (domain.OnboardingStep, error) {
	row, err := scanOnboardingStep(q.QueryRow(ctx, `
		INSERT INTO onboarding_step (
			created_by, updated_by, membership_sf_id, contact_sf_id, email,
			project_id, project_contact_id, step, status, last_error, event_type, event_modified_on
		) VALUES (
			$1, $1, $2, $3, $4,
			$5, $6, $7::onboarding_step_enum, $8::onboarding_step_status_enum, $9, $10, $11
		)
		ON CONFLICT (membership_sf_id, step) DO UPDATE SET
			status             = CASE WHEN EXCLUDED.event_modified_on >= onboarding_step.event_modified_on OR onboarding_step.event_type = 'DELETED' THEN EXCLUDED.status ELSE onboarding_step.status END,
			last_error         = CASE WHEN EXCLUDED.event_modified_on >= onboarding_step.event_modified_on OR onboarding_step.event_type = 'DELETED' THEN EXCLUDED.last_error ELSE onboarding_step.last_error END,
			event_type         = CASE WHEN EXCLUDED.event_modified_on >= onboarding_step.event_modified_on OR onboarding_step.event_type = 'DELETED' THEN EXCLUDED.event_type ELSE onboarding_step.event_type END,
			event_modified_on  = GREATEST(EXCLUDED.event_modified_on, onboarding_step.event_modified_on),
			contact_sf_id      = COALESCE(EXCLUDED.contact_sf_id, onboarding_step.contact_sf_id),
			email              = EXCLUDED.email,
			project_id         = COALESCE(EXCLUDED.project_id, onboarding_step.project_id),
			project_contact_id = COALESCE(EXCLUDED.project_contact_id, onboarding_step.project_contact_id),
			attempt_count      = onboarding_step.attempt_count + 1,
			updated_on         = NOW(),
			updated_by         = EXCLUDED.updated_by
		RETURNING `+onboardingStepColumns,
		req.UpdatedBy, req.MembershipSfID, req.ContactSfID, req.Email,
		req.ProjectID, req.ProjectContactID, string(req.Step), string(req.Status), req.LastError, req.EventType, req.EventModifiedOn,
	))
	if err != nil {
		return domain.OnboardingStep{}, fmt.Errorf("upsert onboarding step: %w", err)
	}
	return row, nil
}

func (r *onboardingStepRepo) GetByMembership(ctx context.Context, membershipSfID string) ([]domain.OnboardingStep, error) {
	return getOnboardingStepsByMembership(ctx, r.db, membershipSfID)
}

func getOnboardingStepsByMembership(ctx context.Context, q querier, membershipSfID string) ([]domain.OnboardingStep, error) {
	rows, err := q.Query(ctx, `SELECT `+onboardingStepColumns+`
		FROM onboarding_step WHERE membership_sf_id = $1 ORDER BY step`, membershipSfID)
	if err != nil {
		return nil, fmt.Errorf("query onboarding steps by membership: %w", err)
	}
	defer rows.Close()
	out := []domain.OnboardingStep{}
	for rows.Next() {
		s, err := scanOnboardingStep(rows)
		if err != nil {
			return nil, fmt.Errorf("scan onboarding step: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate onboarding steps: %w", err)
	}
	return out, nil
}

func (r *onboardingStepRepo) Search(ctx context.Context, req domain.SearchOnboardingStepsRequest) ([]domain.OnboardingStep, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1
	if req.Filters.ProjectID != nil && *req.Filters.ProjectID != "" {
		where += fmt.Sprintf(" AND project_id = $%d", argIdx)
		args = append(args, *req.Filters.ProjectID)
		argIdx++
	}
	if len(req.Filters.MembershipSfIDs) > 0 {
		where += fmt.Sprintf(" AND membership_sf_id = ANY($%d::text[])", argIdx)
		args = append(args, req.Filters.MembershipSfIDs)
		argIdx++
	}
	if len(req.Filters.Statuses) > 0 {
		statuses := make([]string, 0, len(req.Filters.Statuses))
		for _, s := range req.Filters.Statuses {
			statuses = append(statuses, string(s))
		}
		where += fmt.Sprintf(" AND status::TEXT = ANY($%d::text[])", argIdx)
		args = append(args, statuses)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM onboarding_step " + where
	dataQuery := fmt.Sprintf("SELECT %s FROM onboarding_step %s ORDER BY updated_on DESC, id LIMIT $%d OFFSET $%d",
		onboardingStepColumns, where, argIdx, argIdx+1)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var out []domain.OnboardingStep
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count onboarding steps: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query onboarding steps: %w", err)
		}
		defer rows.Close()
		res := make([]domain.OnboardingStep, 0, req.Pagination.Limit)
		for rows.Next() {
			s, err := scanOnboardingStep(rows)
			if err != nil {
				return fmt.Errorf("scan onboarding step: %w", err)
			}
			res = append(res, s)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate onboarding steps: %w", err)
		}
		out = res
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// querier is the subset of pgx shared by a pool and a transaction, so the
// step upsert can run standalone or inside the membership transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
