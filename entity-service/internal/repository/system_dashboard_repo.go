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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SystemDashboardRepository reads and writes system dashboards: the
// custom_dashboard rows with is_system set and no owner (migration 100029).
// It never reads or writes any other row of the table.
type SystemDashboardRepository interface {
	// List is every system dashboard with its definition, ordered by key.
	List(ctx context.Context) ([]domain.SystemDashboard, error)
	// Get is the system dashboard with this key; nil when there is none.
	Get(ctx context.Context, key string) (*domain.SystemDashboard, error)
	// Upsert creates the system dashboard, or overwrites its name and
	// definition (keeping its id and created_on) when the key already exists.
	Upsert(ctx context.Context, key string, req domain.SaveSystemDashboardRequest) (*domain.SystemDashboard, error)
}

type systemDashboardRepo struct {
	db *pgxpool.Pool
}

// NewSystemDashboardRepository constructs a Postgres-backed SystemDashboardRepository.
func NewSystemDashboardRepository(db *pgxpool.Pool) SystemDashboardRepository {
	return &systemDashboardRepo{db: db}
}

const systemDashboardColumns = `id::text, system_key, is_system, name, definition, created_on, updated_on`

func scanSystemDashboard(row pgx.Row) (domain.SystemDashboard, error) {
	var d domain.SystemDashboard
	err := row.Scan(&d.ID, &d.SystemKey, &d.IsSystem, &d.Name, &d.Definition, &d.CreatedOn, &d.UpdatedOn)
	return d, err
}

// List is every system dashboard ordered by key; an empty slice when there
// are none.
func (r *systemDashboardRepo) List(ctx context.Context) ([]domain.SystemDashboard, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+systemDashboardColumns+` FROM custom_dashboard
		 WHERE is_system ORDER BY system_key`)
	if err != nil {
		return nil, fmt.Errorf("list system dashboards: %w", err)
	}
	defer rows.Close()
	out := []domain.SystemDashboard{}
	for rows.Next() {
		d, err := scanSystemDashboard(rows)
		if err != nil {
			return nil, fmt.Errorf("list system dashboards: scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list system dashboards: %w", err)
	}
	return out, nil
}

// Get is the system dashboard with this key; nil when there is none.
func (r *systemDashboardRepo) Get(ctx context.Context, key string) (*domain.SystemDashboard, error) {
	d, err := scanSystemDashboard(r.db.QueryRow(ctx,
		`SELECT `+systemDashboardColumns+` FROM custom_dashboard
		 WHERE is_system AND system_key = $1`, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get system dashboard: %w", err)
	}
	return &d, nil
}

// Upsert is one INSERT ... ON CONFLICT on the system_key index, so two
// startups at the same time cannot both insert. The conflict target names the
// partial index's predicate, which only system rows satisfy (a non-system row
// always has a NULL key), so no user's dashboard can be overwritten.
func (r *systemDashboardRepo) Upsert(ctx context.Context, key string, req domain.SaveSystemDashboardRequest) (*domain.SystemDashboard, error) {
	d, err := scanSystemDashboard(r.db.QueryRow(ctx,
		`INSERT INTO custom_dashboard (owner_user_id, is_system, system_key, name, definition)
		 VALUES (NULL, true, $1, $2, $3)
		 ON CONFLICT (system_key) WHERE system_key IS NOT NULL DO UPDATE
		 SET name = EXCLUDED.name, definition = EXCLUDED.definition, updated_on = NOW()
		 RETURNING `+systemDashboardColumns,
		key, req.Name, req.Definition))
	if err != nil {
		return nil, fmt.Errorf("upsert system dashboard: %w", err)
	}
	return &d, nil
}
