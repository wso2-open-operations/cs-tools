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

// CustomDashboardShareRepository reads and writes who a saved custom dashboard
// is shared with (migration 0252). The owner side takes the owner and only
// ever touches that owner's dashboards; the recipient side takes the
// recipient and only ever sees dashboards shared with them. ids must be UUIDs;
// the service checks that first, and that the recipient may receive a share.
type CustomDashboardShareRepository interface {
	// ListShares is who the owner's dashboard is shared with, ordered by name.
	// Empty both when there are none and when the dashboard is not the owner's.
	ListShares(ctx context.Context, ownerID, dashboardID string) ([]domain.CustomDashboardShare, error)
	// AddShare shares the owner's dashboard with a user; sharing it again is not
	// an error. false when the owner has no dashboard with that id; a
	// NotFoundError when there is no such user.
	AddShare(ctx context.Context, ownerID, dashboardID, userID string) (bool, error)
	// RemoveShare stops sharing the owner's dashboard with a user; false when it
	// was not shared with them (or is not the owner's).
	RemoveShare(ctx context.Context, ownerID, dashboardID, userID string) (bool, error)
	// ListSharedWithUser is every dashboard shared with the user, directly or
	// with a team the user is in now (migration 0253), without definitions,
	// ordered by name. Team shares count only for internal staff and never
	// for the dashboard's owner.
	ListSharedWithUser(ctx context.Context, userID string) ([]domain.SharedCustomDashboardSummary, error)
	// GetSharedWithUser is one dashboard shared with the user, directly or
	// with one of their teams as in ListSharedWithUser; nil when it is not.
	GetSharedWithUser(ctx context.Context, userID, dashboardID string) (*domain.SharedCustomDashboard, error)
}

type customDashboardShareRepo struct {
	db *pgxpool.Pool
}

// NewCustomDashboardShareRepository constructs a Postgres-backed CustomDashboardShareRepository.
func NewCustomDashboardShareRepository(db *pgxpool.Pool) CustomDashboardShareRepository {
	return &customDashboardShareRepo{db: db}
}

// dashboardShareUserName is a user's display name, as elsewhere in this package.
const dashboardShareUserName = `COALESCE(%[1]s.name, NULLIF(TRIM(CONCAT_WS(' ', %[1]s.first_name, %[1]s.last_name)), ''), '')`

// dashboardShareUserRef is the user reference for a share row; the zero value
// when the row carries no id.
func dashboardShareUserRef(id, email, name string) domain.UserReference {
	if ref := domain.NewUserReference(id, email, name); ref != nil {
		return *ref
	}
	return domain.UserReference{}
}

// ListShares is the users the owner's dashboard is shared with, ordered by
// display name; empty when the dashboard is not the owner's or has no shares.
func (r *customDashboardShareRepo) ListShares(ctx context.Context, ownerID, dashboardID string) ([]domain.CustomDashboardShare, error) {
	rows, err := r.db.Query(ctx,
		`SELECT u.id::text, COALESCE(u.email, ''), `+fmt.Sprintf(dashboardShareUserName, "u")+`, s.created_on
		 FROM custom_dashboard_share s
		 JOIN custom_dashboard d ON d.id = s.dashboard_id
		 JOIN "user" u ON u.id = s.shared_with_user_id
		 WHERE d.id = $2::uuid AND d.owner_user_id = $1::uuid
		 ORDER BY lower(`+fmt.Sprintf(dashboardShareUserName, "u")+`), u.id`,
		ownerID, dashboardID)
	if err != nil {
		return nil, fmt.Errorf("list custom dashboard shares: %w", err)
	}
	defer rows.Close()
	out := []domain.CustomDashboardShare{}
	for rows.Next() {
		var id, email, name string
		var sh domain.CustomDashboardShare
		if err := rows.Scan(&id, &email, &name, &sh.SharedOn); err != nil {
			return nil, fmt.Errorf("list custom dashboard shares: scan: %w", err)
		}
		sh.User = dashboardShareUserRef(id, email, name)
		out = append(out, sh)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list custom dashboard shares: %w", err)
	}
	return out, nil
}

// AddShare shares the owner's dashboard with userID; sharing again is not an
// error. The result is false when the dashboard is not the owner's, and an
// unknown user is a NotFoundError.
func (r *customDashboardShareRepo) AddShare(ctx context.Context, ownerID, dashboardID, userID string) (bool, error) {
	// One statement: the insert only happens for the owner's own dashboard,
	// and the result says whether that dashboard exists, shared before or not.
	var owned bool
	err := r.db.QueryRow(ctx,
		`WITH d AS (
		   SELECT id FROM custom_dashboard WHERE id = $2::uuid AND owner_user_id = $1::uuid
		 ), ins AS (
		   INSERT INTO custom_dashboard_share (dashboard_id, shared_with_user_id)
		   SELECT id, $3::uuid FROM d
		   ON CONFLICT (dashboard_id, shared_with_user_id) DO NOTHING
		   RETURNING 1
		 )
		 SELECT EXISTS (SELECT 1 FROM d)`,
		ownerID, dashboardID, userID).Scan(&owned)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "custom_dashboard_share_shared_with_user_id_fkey" {
			return false, &apierror.NotFoundError{Msg: "user not found"}
		}
		return false, fmt.Errorf("add custom dashboard share: %w", err)
	}
	return owned, nil
}

// RemoveShare stops sharing the owner's dashboard with userID; false when the
// dashboard is not the owner's or was not shared with that user.
func (r *customDashboardShareRepo) RemoveShare(ctx context.Context, ownerID, dashboardID, userID string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM custom_dashboard_share s
		 USING custom_dashboard d
		 WHERE s.dashboard_id = d.id
		   AND d.id = $2::uuid AND d.owner_user_id = $1::uuid
		   AND s.shared_with_user_id = $3::uuid`,
		ownerID, dashboardID, userID)
	if err != nil {
		return false, fmt.Errorf("remove custom dashboard share: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// sharedWithUserFrom is the join both recipient reads use: the dashboards
// shared with $1, each with its owner and the one share that grants it (alias
// s: team_id / team_name / team_type are NULL for a direct share). Team
// membership is read live from team_member: whoever is in the team now sees
// the dashboard. A team share counts only when $1 is internal staff (person
// shares check that when they are added; a team's members are not checked
// then) and never for the dashboard's owner. A dashboard whose owner was
// deleted (owner_user_id NULL, migration 0255) still shows, with an empty
// owner. With several shares of one dashboard, the direct one wins, then the
// team first by name.
const sharedWithUserFrom = `
		 FROM (
		   SELECT DISTINCT ON (g.dashboard_id) g.dashboard_id, g.created_on, g.team_id, g.team_name, g.team_type
		   FROM (
		     SELECT ps.dashboard_id, ps.created_on, NULL::uuid AS team_id, NULL::text AS team_name, NULL::text AS team_type, 0 AS via
		     FROM custom_dashboard_share ps
		     WHERE ps.shared_with_user_id = $1::uuid
		     UNION ALL
		     SELECT ts.dashboard_id, ts.created_on, t.id, t.name::text, t.type::text, 1
		     FROM custom_dashboard_team_share ts
		     JOIN team t ON t.id = ts.team_id
		     JOIN team_member tm ON tm.team_id = t.id AND tm.user_id = $1::uuid
		     JOIN "user" me ON me.id = tm.user_id AND me.user_type = 'INTERNAL'
		   ) g
		   ORDER BY g.dashboard_id, g.via, lower(g.team_name), g.created_on
		 ) s
		 JOIN custom_dashboard d ON d.id = s.dashboard_id AND d.owner_user_id IS DISTINCT FROM $1::uuid
		 LEFT JOIN "user" o ON o.id = d.owner_user_id
		 WHERE TRUE`

// ListSharedWithUser is the dashboards shared with userID, directly or through
// a team they are in now, without definitions, ordered by name. Each comes
// once, with the share that grants it (see sharedWithUserFrom).
func (r *customDashboardShareRepo) ListSharedWithUser(ctx context.Context, userID string) ([]domain.SharedCustomDashboardSummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT d.id::text, d.name, COALESCE(o.id::text, ''), COALESCE(o.email, ''), `+fmt.Sprintf(dashboardShareUserName, "o")+`, s.created_on, d.updated_on, `+sharedViaTeamColumns+
			sharedWithUserFrom+`
		 ORDER BY lower(d.name), d.id`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("list dashboards shared with user: %w", err)
	}
	defer rows.Close()
	out := []domain.SharedCustomDashboardSummary{}
	for rows.Next() {
		var ownerID, ownerEmail, ownerName string
		var teamID, teamName, teamType *string
		var d domain.SharedCustomDashboardSummary
		if err := rows.Scan(&d.ID, &d.Name, &ownerID, &ownerEmail, &ownerName, &d.SharedOn, &d.UpdatedOn, &teamID, &teamName, &teamType); err != nil {
			return nil, fmt.Errorf("list dashboards shared with user: scan: %w", err)
		}
		d.Owner = domain.NewUserReference(ownerID, ownerEmail, ownerName)
		d.SharedViaTeam = sharedViaTeam(teamID, teamName, teamType)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list dashboards shared with user: %w", err)
	}
	return out, nil
}

// GetSharedWithUser is one dashboard shared with userID, with its definition;
// nil when it is not shared with them (or they own it).
func (r *customDashboardShareRepo) GetSharedWithUser(ctx context.Context, userID, dashboardID string) (*domain.SharedCustomDashboard, error) {
	var ownerID, ownerEmail, ownerName string
	var teamID, teamName, teamType *string
	var d domain.SharedCustomDashboard
	err := r.db.QueryRow(ctx,
		`SELECT d.id::text, d.name, d.definition, COALESCE(o.id::text, ''), COALESCE(o.email, ''), `+fmt.Sprintf(dashboardShareUserName, "o")+`, s.created_on, d.updated_on, `+sharedViaTeamColumns+
			sharedWithUserFrom+`
		   AND d.id = $2::uuid`,
		userID, dashboardID).Scan(&d.ID, &d.Name, &d.Definition, &ownerID, &ownerEmail, &ownerName, &d.SharedOn, &d.UpdatedOn, &teamID, &teamName, &teamType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get dashboard shared with user: %w", err)
	}
	d.Owner = domain.NewUserReference(ownerID, ownerEmail, ownerName)
	d.SharedViaTeam = sharedViaTeam(teamID, teamName, teamType)
	return &d, nil
}

// sharedViaTeamColumns are the granting share's team, NULL for a direct share.
const sharedViaTeamColumns = `s.team_id::text, s.team_name, s.team_type`

// sharedViaTeam is the team a shared dashboard reached the caller through; nil
// for a direct share.
func sharedViaTeam(id, name, typ *string) *domain.CustomDashboardTeamShareTeam {
	if id == nil {
		return nil
	}
	team := &domain.CustomDashboardTeamShareTeam{ID: *id}
	if name != nil {
		team.Name = *name
	}
	if typ != nil {
		team.Type = *typ
	}
	return team
}
