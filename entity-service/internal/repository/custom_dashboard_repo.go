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

// CustomDashboardRepository reads and writes saved custom dashboards
// (migration 0251). Every method takes the owner and only ever sees that
// owner's rows: another owner's dashboard is the same as no dashboard.
// ids must be UUIDs; the service checks that first.
type CustomDashboardRepository interface {
	// List is the owner's dashboards without their definitions, ordered by name.
	List(ctx context.Context, ownerID string) ([]domain.CustomDashboardSummary, error)
	// Get is one of the owner's dashboards; nil when the owner has none with that id.
	Get(ctx context.Context, ownerID, id string) (*domain.CustomDashboard, error)
	// Upsert creates the dashboard, or replaces it when the owner already has one with this id.
	// nil when the id belongs to another owner; a ConflictError when the owner has another
	// dashboard with that name; a ValidationError when a new dashboard would take the owner
	// past domain.MaxCustomDashboards (checked under a per-owner lock).
	Upsert(ctx context.Context, ownerID, id string, req domain.SaveCustomDashboardRequest) (*domain.CustomDashboard, error)
	// Delete removes one of the owner's dashboards; false when the owner has none with that id.
	Delete(ctx context.Context, ownerID, id string) (bool, error)
}

type customDashboardRepo struct {
	db *pgxpool.Pool
}

// NewCustomDashboardRepository constructs a Postgres-backed CustomDashboardRepository.
func NewCustomDashboardRepository(db *pgxpool.Pool) CustomDashboardRepository {
	return &customDashboardRepo{db: db}
}

const customDashboardColumns = `id::text, name, definition, created_on, updated_on`

func scanCustomDashboard(row pgx.Row) (domain.CustomDashboard, error) {
	var d domain.CustomDashboard
	err := row.Scan(&d.ID, &d.Name, &d.Definition, &d.CreatedOn, &d.UpdatedOn)
	return d, err
}

// customDashboardNameConflict is the unique index on (owner, lower(name)).
func customDashboardNameConflict(err error, name string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "custom_dashboard_owner_name" {
		return &apierror.ConflictError{Msg: "you already have a dashboard named: " + name}
	}
	return nil
}

// List is the owner's dashboards without their definitions, ordered by name
// (case-insensitively, then id); an empty slice when there are none.
func (r *customDashboardRepo) List(ctx context.Context, ownerID string) ([]domain.CustomDashboardSummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, name, created_on, updated_on FROM custom_dashboard
		 WHERE owner_user_id = $1::uuid
		 ORDER BY lower(name), id`,
		ownerID)
	if err != nil {
		return nil, fmt.Errorf("list custom dashboards: %w", err)
	}
	defer rows.Close()
	out := []domain.CustomDashboardSummary{}
	for rows.Next() {
		var s domain.CustomDashboardSummary
		if err := rows.Scan(&s.ID, &s.Name, &s.CreatedOn, &s.UpdatedOn); err != nil {
			return nil, fmt.Errorf("list custom dashboards: scan: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list custom dashboards: %w", err)
	}
	return out, nil
}

// Get is one of the owner's dashboards with its definition; nil when the
// owner has none with that id (another owner's dashboard included).
func (r *customDashboardRepo) Get(ctx context.Context, ownerID, id string) (*domain.CustomDashboard, error) {
	d, err := scanCustomDashboard(r.db.QueryRow(ctx,
		`SELECT `+customDashboardColumns+` FROM custom_dashboard
		 WHERE owner_user_id = $1::uuid AND id = $2::uuid`,
		ownerID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get custom dashboard: %w", err)
	}
	return &d, nil
}

// Upsert saves the dashboard inside one transaction that first takes a lock
// for the owner, then counts the owner's dashboards: a new one beyond
// domain.MaxCustomDashboards is refused there, so two saves at the same time
// cannot both pass the count. Replacing one of the owner's own dashboards is
// never refused. Saves by different owners do not wait for each other.
func (r *customDashboardRepo) Upsert(ctx context.Context, ownerID, id string, req domain.SaveCustomDashboardRequest) (*domain.CustomDashboard, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("upsert custom dashboard: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once Commit has succeeded

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, "custom-dashboard-owner:"+ownerID); err != nil {
		return nil, fmt.Errorf("upsert custom dashboard: lock: %w", err)
	}
	var count int
	var replacing bool
	if err := tx.QueryRow(ctx,
		`SELECT count(*), COALESCE(bool_or(id = $2::uuid), false)
		 FROM custom_dashboard WHERE owner_user_id = $1::uuid`,
		ownerID, id).Scan(&count, &replacing); err != nil {
		return nil, fmt.Errorf("upsert custom dashboard: count: %w", err)
	}
	if !replacing && count >= domain.MaxCustomDashboards {
		return nil, &apierror.ValidationError{
			Msg: fmt.Sprintf("at most %d saved dashboards are allowed", domain.MaxCustomDashboards),
		}
	}

	// The WHERE on the update keeps another owner's row untouched: the
	// statement then returns no row.
	d, err := scanCustomDashboard(tx.QueryRow(ctx,
		`INSERT INTO custom_dashboard (id, owner_user_id, name, definition)
		 VALUES ($1::uuid, $2::uuid, $3, $4)
		 ON CONFLICT (id) DO UPDATE
		 SET name = EXCLUDED.name, definition = EXCLUDED.definition, updated_on = NOW()
		 WHERE custom_dashboard.owner_user_id = EXCLUDED.owner_user_id
		 RETURNING `+customDashboardColumns,
		id, ownerID, req.Name, req.Definition))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		if conflict := customDashboardNameConflict(err, req.Name); conflict != nil {
			return nil, conflict
		}
		return nil, fmt.Errorf("upsert custom dashboard: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("upsert custom dashboard: commit: %w", err)
	}
	return &d, nil
}

// Delete removes one of the owner's dashboards, and its shares with it (they
// cascade); false when the owner has none with that id.
func (r *customDashboardRepo) Delete(ctx context.Context, ownerID, id string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM custom_dashboard WHERE owner_user_id = $1::uuid AND id = $2::uuid`,
		ownerID, id)
	if err != nil {
		return false, fmt.Errorf("delete custom dashboard: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
