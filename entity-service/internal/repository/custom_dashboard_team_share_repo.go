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

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// CustomDashboardTeamShareRepository reads and writes which teams a saved
// custom dashboard is shared with (migration 0253), the owner side only: it
// takes the owner and only ever touches that owner's dashboards. Who is in a
// team is not stored here; the recipient reads join team_member when they
// run. ids must be UUIDs; the service checks that first.
type CustomDashboardTeamShareRepository interface {
	// ListTeamShares is which teams the owner's dashboard is shared with,
	// ordered by team name. Empty both when there are none and when the
	// dashboard is not the owner's.
	ListTeamShares(ctx context.Context, ownerID, dashboardID string) ([]domain.CustomDashboardTeamShare, error)
	// AddTeamShare shares the owner's dashboard with a team; sharing it again
	// is not an error. false when the owner has no dashboard with that id; a
	// NotFoundError when there is no such team.
	AddTeamShare(ctx context.Context, ownerID, dashboardID, teamID string) (bool, error)
	// RemoveTeamShare stops sharing the owner's dashboard with a team; false
	// when it was not shared with it (or is not the owner's).
	RemoveTeamShare(ctx context.Context, ownerID, dashboardID, teamID string) (bool, error)
}

type customDashboardTeamShareRepo struct {
	db *pgxpool.Pool
}

// NewCustomDashboardTeamShareRepository constructs a Postgres-backed CustomDashboardTeamShareRepository.
func NewCustomDashboardTeamShareRepository(db *pgxpool.Pool) CustomDashboardTeamShareRepository {
	return &customDashboardTeamShareRepo{db: db}
}

// ListTeamShares is the teams the owner's dashboard is shared with, ordered by
// name; empty when the dashboard is not the owner's or has no team shares.
func (r *customDashboardTeamShareRepo) ListTeamShares(ctx context.Context, ownerID, dashboardID string) ([]domain.CustomDashboardTeamShare, error) {
	rows, err := r.db.Query(ctx,
		`SELECT t.id::text, t.name, t.type, s.created_on
		 FROM custom_dashboard_team_share s
		 JOIN custom_dashboard d ON d.id = s.dashboard_id
		 JOIN team t ON t.id = s.team_id
		 WHERE d.id = $2::uuid AND d.owner_user_id = $1::uuid
		 ORDER BY lower(t.name), t.id`,
		ownerID, dashboardID)
	if err != nil {
		return nil, fmt.Errorf("list custom dashboard team shares: %w", err)
	}
	defer rows.Close()
	out := []domain.CustomDashboardTeamShare{}
	for rows.Next() {
		var sh domain.CustomDashboardTeamShare
		if err := rows.Scan(&sh.Team.ID, &sh.Team.Name, &sh.Team.Type, &sh.SharedOn); err != nil {
			return nil, fmt.Errorf("list custom dashboard team shares: scan: %w", err)
		}
		out = append(out, sh)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list custom dashboard team shares: %w", err)
	}
	return out, nil
}

// AddTeamShare shares the owner's dashboard with teamID; sharing again is not
// an error. The result is false when the dashboard is not the owner's, and an
// unknown team is a NotFoundError.
func (r *customDashboardTeamShareRepo) AddTeamShare(ctx context.Context, ownerID, dashboardID, teamID string) (bool, error) {
	// One statement, as AddShare: the insert only happens for the owner's own
	// dashboard, and the result says whether that dashboard exists.
	var owned bool
	err := r.db.QueryRow(ctx,
		`WITH d AS (
		   SELECT id FROM custom_dashboard WHERE id = $2::uuid AND owner_user_id = $1::uuid
		 ), ins AS (
		   INSERT INTO custom_dashboard_team_share (dashboard_id, team_id)
		   SELECT id, $3::uuid FROM d
		   ON CONFLICT (dashboard_id, team_id) DO NOTHING
		   RETURNING 1
		 )
		 SELECT EXISTS (SELECT 1 FROM d)`,
		ownerID, dashboardID, teamID).Scan(&owned)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "custom_dashboard_team_share_team_id_fkey" {
			return false, &apierror.NotFoundError{Msg: "team not found"}
		}
		return false, fmt.Errorf("add custom dashboard team share: %w", err)
	}
	return owned, nil
}

// RemoveTeamShare stops sharing the owner's dashboard with teamID; false when
// the dashboard is not the owner's or was not shared with that team.
func (r *customDashboardTeamShareRepo) RemoveTeamShare(ctx context.Context, ownerID, dashboardID, teamID string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM custom_dashboard_team_share s
		 USING custom_dashboard d
		 WHERE s.dashboard_id = d.id
		   AND d.id = $2::uuid AND d.owner_user_id = $1::uuid
		   AND s.team_id = $3::uuid`,
		ownerID, dashboardID, teamID)
	if err != nil {
		return false, fmt.Errorf("remove custom dashboard team share: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
