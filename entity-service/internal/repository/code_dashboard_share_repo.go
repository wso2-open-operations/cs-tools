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

// CodeDashboardShareRepository is sharing a saved code dashboard (migration
// 0296) with people, teams and anyone internal who has its link: the custom
// dashboard share, team share and link repositories in one, by the same rules
// and with the same queries, on the code_dashboard tables. The owner side
// takes the owner and only ever touches that owner's dashboards; the recipient
// side takes the recipient and only ever sees dashboards shared with them.
// ids must be UUIDs; the service checks that first, and that a recipient may
// receive a share.
type CodeDashboardShareRepository interface {
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
	// ListTeamShares is which teams the owner's dashboard is shared with,
	// ordered by team name; empty as ListShares.
	ListTeamShares(ctx context.Context, ownerID, dashboardID string) ([]domain.CustomDashboardTeamShare, error)
	// AddTeamShare shares the owner's dashboard with a team, as AddShare; a
	// NotFoundError when there is no such team.
	AddTeamShare(ctx context.Context, ownerID, dashboardID, teamID string) (bool, error)
	// RemoveTeamShare stops sharing the owner's dashboard with a team, as
	// RemoveShare.
	RemoveTeamShare(ctx context.Context, ownerID, dashboardID, teamID string) (bool, error)
	// GetLink is the owner's dashboard's link state; nil when the owner has no
	// dashboard with that id.
	GetLink(ctx context.Context, ownerID, dashboardID string) (*domain.CustomDashboardLink, error)
	// EnableLink turns the link on with a new random link id; when it is
	// already on, it keeps its id. nil when the owner has no dashboard with
	// that id.
	EnableLink(ctx context.Context, ownerID, dashboardID string) (*domain.CustomDashboardLink, error)
	// DisableLink turns the link off: the link id goes, and so does the access
	// of everyone who opened it. false when the owner has no dashboard with
	// that id.
	DisableLink(ctx context.Context, ownerID, dashboardID string) (bool, error)
	// OpenLink is the user opening a dashboard's link: the dashboard's id, and
	// whether the user owns it. "" when no dashboard has that link on, or the
	// user is not internal staff.
	OpenLink(ctx context.Context, userID, linkID string) (dashboardID string, own bool, err error)
	// ListSharedWithUser is every code dashboard shared with the user,
	// directly, with a team the user is in now, or through a link they opened
	// while it is on, without sources, ordered by name. Team and link shares
	// count only for internal staff, and nothing counts for the owner.
	ListSharedWithUser(ctx context.Context, userID string) ([]domain.SharedCustomDashboardSummary, error)
	// GetSharedWithUser is one code dashboard shared with the user in any of
	// the ways of ListSharedWithUser, with its source; nil when it is not.
	GetSharedWithUser(ctx context.Context, userID, dashboardID string) (*domain.SharedCodeDashboard, error)
}

type codeDashboardShareRepo struct {
	db *pgxpool.Pool
}

// NewCodeDashboardShareRepository constructs a Postgres-backed CodeDashboardShareRepository.
func NewCodeDashboardShareRepository(db *pgxpool.Pool) CodeDashboardShareRepository {
	return &codeDashboardShareRepo{db: db}
}

// ListShares is customDashboardShareRepo.ListShares on code_dashboard_share.
func (r *codeDashboardShareRepo) ListShares(ctx context.Context, ownerID, dashboardID string) ([]domain.CustomDashboardShare, error) {
	rows, err := r.db.Query(ctx,
		`SELECT u.id::text, COALESCE(u.email, ''), `+fmt.Sprintf(dashboardShareUserName, "u")+`, s.created_on
		 FROM code_dashboard_share s
		 JOIN code_dashboard d ON d.id = s.dashboard_id
		 JOIN "user" u ON u.id = s.shared_with_user_id
		 WHERE d.id = $2::uuid AND d.owner_user_id = $1::uuid
		 ORDER BY lower(`+fmt.Sprintf(dashboardShareUserName, "u")+`), u.id`,
		ownerID, dashboardID)
	if err != nil {
		return nil, fmt.Errorf("list code dashboard shares: %w", err)
	}
	defer rows.Close()
	out := []domain.CustomDashboardShare{}
	for rows.Next() {
		var id, email, name string
		var sh domain.CustomDashboardShare
		if err := rows.Scan(&id, &email, &name, &sh.SharedOn); err != nil {
			return nil, fmt.Errorf("list code dashboard shares: scan: %w", err)
		}
		sh.User = dashboardShareUserRef(id, email, name)
		out = append(out, sh)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list code dashboard shares: %w", err)
	}
	return out, nil
}

// AddShare is customDashboardShareRepo.AddShare on code_dashboard_share: one
// statement, inserting only for the owner's own dashboard.
func (r *codeDashboardShareRepo) AddShare(ctx context.Context, ownerID, dashboardID, userID string) (bool, error) {
	var owned bool
	err := r.db.QueryRow(ctx,
		`WITH d AS (
		   SELECT id FROM code_dashboard WHERE id = $2::uuid AND owner_user_id = $1::uuid
		 ), ins AS (
		   INSERT INTO code_dashboard_share (dashboard_id, shared_with_user_id)
		   SELECT id, $3::uuid FROM d
		   ON CONFLICT (dashboard_id, shared_with_user_id) DO NOTHING
		   RETURNING 1
		 )
		 SELECT EXISTS (SELECT 1 FROM d)`,
		ownerID, dashboardID, userID).Scan(&owned)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "code_dashboard_share_shared_with_user_id_fkey" {
			return false, &apierror.NotFoundError{Msg: "user not found"}
		}
		return false, fmt.Errorf("add code dashboard share: %w", err)
	}
	return owned, nil
}

// RemoveShare is customDashboardShareRepo.RemoveShare on code_dashboard_share.
func (r *codeDashboardShareRepo) RemoveShare(ctx context.Context, ownerID, dashboardID, userID string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM code_dashboard_share s
		 USING code_dashboard d
		 WHERE s.dashboard_id = d.id
		   AND d.id = $2::uuid AND d.owner_user_id = $1::uuid
		   AND s.shared_with_user_id = $3::uuid`,
		ownerID, dashboardID, userID)
	if err != nil {
		return false, fmt.Errorf("remove code dashboard share: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ListTeamShares is customDashboardTeamShareRepo.ListTeamShares on
// code_dashboard_team_share.
func (r *codeDashboardShareRepo) ListTeamShares(ctx context.Context, ownerID, dashboardID string) ([]domain.CustomDashboardTeamShare, error) {
	rows, err := r.db.Query(ctx,
		`SELECT t.id::text, t.name, t.type, s.created_on
		 FROM code_dashboard_team_share s
		 JOIN code_dashboard d ON d.id = s.dashboard_id
		 JOIN team t ON t.id = s.team_id
		 WHERE d.id = $2::uuid AND d.owner_user_id = $1::uuid
		 ORDER BY lower(t.name), t.id`,
		ownerID, dashboardID)
	if err != nil {
		return nil, fmt.Errorf("list code dashboard team shares: %w", err)
	}
	defer rows.Close()
	out := []domain.CustomDashboardTeamShare{}
	for rows.Next() {
		var sh domain.CustomDashboardTeamShare
		if err := rows.Scan(&sh.Team.ID, &sh.Team.Name, &sh.Team.Type, &sh.SharedOn); err != nil {
			return nil, fmt.Errorf("list code dashboard team shares: scan: %w", err)
		}
		out = append(out, sh)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list code dashboard team shares: %w", err)
	}
	return out, nil
}

// AddTeamShare is customDashboardTeamShareRepo.AddTeamShare on
// code_dashboard_team_share.
func (r *codeDashboardShareRepo) AddTeamShare(ctx context.Context, ownerID, dashboardID, teamID string) (bool, error) {
	var owned bool
	err := r.db.QueryRow(ctx,
		`WITH d AS (
		   SELECT id FROM code_dashboard WHERE id = $2::uuid AND owner_user_id = $1::uuid
		 ), ins AS (
		   INSERT INTO code_dashboard_team_share (dashboard_id, team_id)
		   SELECT id, $3::uuid FROM d
		   ON CONFLICT (dashboard_id, team_id) DO NOTHING
		   RETURNING 1
		 )
		 SELECT EXISTS (SELECT 1 FROM d)`,
		ownerID, dashboardID, teamID).Scan(&owned)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "code_dashboard_team_share_team_id_fkey" {
			return false, &apierror.NotFoundError{Msg: "team not found"}
		}
		return false, fmt.Errorf("add code dashboard team share: %w", err)
	}
	return owned, nil
}

// RemoveTeamShare is customDashboardTeamShareRepo.RemoveTeamShare on
// code_dashboard_team_share.
func (r *codeDashboardShareRepo) RemoveTeamShare(ctx context.Context, ownerID, dashboardID, teamID string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM code_dashboard_team_share s
		 USING code_dashboard d
		 WHERE s.dashboard_id = d.id
		   AND d.id = $2::uuid AND d.owner_user_id = $1::uuid
		   AND s.team_id = $3::uuid`,
		ownerID, dashboardID, teamID)
	if err != nil {
		return false, fmt.Errorf("remove code dashboard team share: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// codeDashboardLinkState is customDashboardLinkState on code_dashboard.
const codeDashboardLinkState = `
	SELECT d.link_enabled, COALESCE(d.link_id::text, ''),
	       (SELECT count(*) FROM code_dashboard_link_access a WHERE a.dashboard_id = d.id)
	FROM code_dashboard d
	WHERE d.id = $2::uuid AND d.owner_user_id = $1::uuid`

// GetLink is customDashboardLinkRepo.GetLink on code_dashboard.
func (r *codeDashboardShareRepo) GetLink(ctx context.Context, ownerID, dashboardID string) (*domain.CustomDashboardLink, error) {
	l, err := scanCustomDashboardLink(r.db.QueryRow(ctx, codeDashboardLinkState, ownerID, dashboardID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get code dashboard link: %w", err)
	}
	return l, nil
}

// EnableLink is customDashboardLinkRepo.EnableLink on code_dashboard: a link
// that is already on keeps its id.
func (r *codeDashboardShareRepo) EnableLink(ctx context.Context, ownerID, dashboardID string) (*domain.CustomDashboardLink, error) {
	l, err := scanCustomDashboardLink(r.db.QueryRow(ctx,
		`WITH d AS (
		   UPDATE code_dashboard
		   SET link_enabled = true, link_id = COALESCE(link_id, gen_random_uuid())
		   WHERE id = $2::uuid AND owner_user_id = $1::uuid
		   RETURNING id, link_enabled, link_id
		 )
		 SELECT d.link_enabled, d.link_id::text,
		        (SELECT count(*) FROM code_dashboard_link_access a WHERE a.dashboard_id = d.id)
		 FROM d`,
		ownerID, dashboardID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("enable code dashboard link: %w", err)
	}
	return l, nil
}

// DisableLink is customDashboardLinkRepo.DisableLink on code_dashboard: the
// link goes off and everyone's access through it goes, in one transaction.
func (r *codeDashboardShareRepo) DisableLink(ctx context.Context, ownerID, dashboardID string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("disable code dashboard link: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once Commit has succeeded

	tag, err := tx.Exec(ctx,
		`UPDATE code_dashboard SET link_enabled = false, link_id = NULL
		 WHERE id = $2::uuid AND owner_user_id = $1::uuid`,
		ownerID, dashboardID)
	if err != nil {
		return false, fmt.Errorf("disable code dashboard link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM code_dashboard_link_access WHERE dashboard_id = $1::uuid`, dashboardID); err != nil {
		return false, fmt.Errorf("disable code dashboard link: remove access: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("disable code dashboard link: commit: %w", err)
	}
	return true, nil
}

// OpenLink is customDashboardLinkRepo.OpenLink on code_dashboard, with the
// same FOR SHARE read of the dashboard row so an open cannot overlap
// DisableLink and leave an access row behind (see that method).
func (r *codeDashboardShareRepo) OpenLink(ctx context.Context, userID, linkID string) (string, bool, error) {
	var dashboardID string
	var own bool
	err := r.db.QueryRow(ctx,
		`WITH d AS (
		   SELECT id, owner_user_id FROM code_dashboard WHERE link_id = $2::uuid AND link_enabled
		   FOR SHARE
		 ), me AS (
		   SELECT id FROM "user" WHERE id = $1::uuid AND user_type = 'INTERNAL'
		 ), ins AS (
		   INSERT INTO code_dashboard_link_access (dashboard_id, user_id)
		   SELECT d.id, me.id FROM d, me
		   WHERE d.owner_user_id IS DISTINCT FROM me.id
		   ON CONFLICT (dashboard_id, user_id) DO NOTHING
		   RETURNING 1
		 )
		 SELECT d.id::text, d.owner_user_id IS NOT DISTINCT FROM me.id FROM d, me`,
		userID, linkID).Scan(&dashboardID, &own)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("open code dashboard link: %w", err)
	}
	return dashboardID, own, nil
}

// sharedCodeDashboardsFrom is sharedWithUserFrom on the code_dashboard tables:
// each code dashboard shared with $1 once, with the share that grants it (the
// direct one wins, then the team first by name, then the link), team and link
// shares only for internal staff, nothing for the owner, an ownerless
// dashboard with an empty owner.
const sharedCodeDashboardsFrom = `
		 FROM (
		   SELECT DISTINCT ON (g.dashboard_id) g.dashboard_id, g.created_on, g.team_id, g.team_name, g.team_type, g.via
		   FROM (
		     SELECT ps.dashboard_id, ps.created_on, NULL::uuid AS team_id, NULL::text AS team_name, NULL::text AS team_type, 0 AS via
		     FROM code_dashboard_share ps
		     WHERE ps.shared_with_user_id = $1::uuid
		     UNION ALL
		     SELECT ts.dashboard_id, ts.created_on, t.id, t.name::text, t.type::text, 1
		     FROM code_dashboard_team_share ts
		     JOIN team t ON t.id = ts.team_id
		     JOIN team_member tm ON tm.team_id = t.id AND tm.user_id = $1::uuid
		     JOIN "user" me ON me.id = tm.user_id AND me.user_type = 'INTERNAL'
		     UNION ALL
		     SELECT la.dashboard_id, la.created_on, NULL::uuid, NULL::text, NULL::text, 2
		     FROM code_dashboard_link_access la
		     JOIN code_dashboard ld ON ld.id = la.dashboard_id AND ld.link_enabled
		     JOIN "user" lme ON lme.id = la.user_id AND lme.user_type = 'INTERNAL'
		     WHERE la.user_id = $1::uuid
		   ) g
		   ORDER BY g.dashboard_id, g.via, lower(g.team_name), g.created_on
		 ) s
		 JOIN code_dashboard d ON d.id = s.dashboard_id AND d.owner_user_id IS DISTINCT FROM $1::uuid
		 LEFT JOIN "user" o ON o.id = d.owner_user_id
		 WHERE TRUE`

// ListSharedWithUser is the code dashboards shared with userID, without
// sources, ordered by name, each once with the share that grants it.
func (r *codeDashboardShareRepo) ListSharedWithUser(ctx context.Context, userID string) ([]domain.SharedCustomDashboardSummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT d.id::text, d.name, COALESCE(o.id::text, ''), COALESCE(o.email, ''), `+fmt.Sprintf(dashboardShareUserName, "o")+`, s.created_on, d.updated_on, `+sharedViaColumns+
			sharedCodeDashboardsFrom+`
		 ORDER BY lower(d.name), d.id`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("list code dashboards shared with user: %w", err)
	}
	defer rows.Close()
	out := []domain.SharedCustomDashboardSummary{}
	for rows.Next() {
		var ownerID, ownerEmail, ownerName string
		var teamID, teamName, teamType *string
		var d domain.SharedCustomDashboardSummary
		if err := rows.Scan(&d.ID, &d.Name, &ownerID, &ownerEmail, &ownerName, &d.SharedOn, &d.UpdatedOn, &teamID, &teamName, &teamType, &d.SharedViaLink); err != nil {
			return nil, fmt.Errorf("list code dashboards shared with user: scan: %w", err)
		}
		d.Owner = domain.NewUserReference(ownerID, ownerEmail, ownerName)
		d.SharedViaTeam = sharedViaTeam(teamID, teamName, teamType)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list code dashboards shared with user: %w", err)
	}
	return out, nil
}

// GetSharedWithUser is one code dashboard shared with userID, with its
// source; nil when it is not shared with them (or they own it).
func (r *codeDashboardShareRepo) GetSharedWithUser(ctx context.Context, userID, dashboardID string) (*domain.SharedCodeDashboard, error) {
	var ownerID, ownerEmail, ownerName string
	var teamID, teamName, teamType *string
	var d domain.SharedCodeDashboard
	err := r.db.QueryRow(ctx,
		`SELECT d.id::text, d.name, d.source, COALESCE(o.id::text, ''), COALESCE(o.email, ''), `+fmt.Sprintf(dashboardShareUserName, "o")+`, s.created_on, d.updated_on, `+sharedViaColumns+
			sharedCodeDashboardsFrom+`
		   AND d.id = $2::uuid`,
		userID, dashboardID).Scan(&d.ID, &d.Name, &d.Source, &ownerID, &ownerEmail, &ownerName, &d.SharedOn, &d.UpdatedOn, &teamID, &teamName, &teamType, &d.SharedViaLink)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get code dashboard shared with user: %w", err)
	}
	d.Owner = domain.NewUserReference(ownerID, ownerEmail, ownerName)
	d.SharedViaTeam = sharedViaTeam(teamID, teamName, teamType)
	return &d, nil
}
