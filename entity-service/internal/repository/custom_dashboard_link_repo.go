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

// CustomDashboardLinkRepository reads and writes whether anyone with a saved
// dashboard's link may view it (migration 0269). The owner side takes the
// owner and only ever touches that owner's dashboards; OpenLink is the
// recipient side. Once opened, the dashboard is read like any other shared
// one (CustomDashboardShareRepository). ids must be UUIDs; the service checks
// that first.
type CustomDashboardLinkRepository interface {
	// GetLink is the owner's dashboard's link state; nil when the owner has
	// no dashboard with that id.
	GetLink(ctx context.Context, ownerID, dashboardID string) (*domain.CustomDashboardLink, error)
	// EnableLink turns the link on with a new random link id; when it is
	// already on, it keeps its id. nil when the owner has no dashboard with
	// that id.
	EnableLink(ctx context.Context, ownerID, dashboardID string) (*domain.CustomDashboardLink, error)
	// DisableLink turns the link off: the link id goes, and so does the access
	// of everyone who opened it. Turning off a link that is off is not an
	// error. false when the owner has no dashboard with that id.
	DisableLink(ctx context.Context, ownerID, dashboardID string) (bool, error)
	// OpenLink is the user opening a dashboard's link: the dashboard's id,
	// and whether the user owns it. "" when no dashboard has that link on, or
	// the user is not internal staff. The user then sees the dashboard under
	// Shared Dashboards, unless they own it or already have it through a
	// person or team share.
	OpenLink(ctx context.Context, userID, linkID string) (dashboardID string, own bool, err error)
}

type customDashboardLinkRepo struct {
	db *pgxpool.Pool
}

// NewCustomDashboardLinkRepository constructs a Postgres-backed CustomDashboardLinkRepository.
func NewCustomDashboardLinkRepository(db *pgxpool.Pool) CustomDashboardLinkRepository {
	return &customDashboardLinkRepo{db: db}
}

// customDashboardLinkState reads the option, the link id and the number of
// people who opened it, for the owner's dashboard $2.
const customDashboardLinkState = `
	SELECT d.link_enabled, COALESCE(d.link_id::text, ''),
	       (SELECT count(*) FROM custom_dashboard_link_access a WHERE a.dashboard_id = d.id)
	FROM custom_dashboard d
	WHERE d.id = $2::uuid AND d.owner_user_id = $1::uuid`

func scanCustomDashboardLink(row pgx.Row) (*domain.CustomDashboardLink, error) {
	var l domain.CustomDashboardLink
	if err := row.Scan(&l.Enabled, &l.LinkID, &l.OpenedBy); err != nil {
		return nil, err
	}
	return &l, nil
}

// GetLink is the link state of the owner's dashboard; nil when the dashboard
// is not the owner's.
func (r *customDashboardLinkRepo) GetLink(ctx context.Context, ownerID, dashboardID string) (*domain.CustomDashboardLink, error) {
	l, err := scanCustomDashboardLink(r.db.QueryRow(ctx, customDashboardLinkState, ownerID, dashboardID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get custom dashboard link: %w", err)
	}
	return l, nil
}

// EnableLink turns the owner's dashboard's link on. A link that is already on
// keeps its id, so a second click changes nothing and copies already sent
// still work; only turning it off and on again makes a new one. nil when the
// dashboard is not the owner's.
func (r *customDashboardLinkRepo) EnableLink(ctx context.Context, ownerID, dashboardID string) (*domain.CustomDashboardLink, error) {
	// The update and the read in one statement: the CTE's RETURNING is the
	// row as updated, and a dashboard that is not the owner's returns no row.
	l, err := scanCustomDashboardLink(r.db.QueryRow(ctx,
		`WITH d AS (
		   UPDATE custom_dashboard
		   SET link_enabled = true, link_id = COALESCE(link_id, gen_random_uuid())
		   WHERE id = $2::uuid AND owner_user_id = $1::uuid
		   RETURNING id, link_enabled, link_id
		 )
		 SELECT d.link_enabled, d.link_id::text,
		        (SELECT count(*) FROM custom_dashboard_link_access a WHERE a.dashboard_id = d.id)
		 FROM d`,
		ownerID, dashboardID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("enable custom dashboard link: %w", err)
	}
	return l, nil
}

// DisableLink turns the owner's dashboard's link off and removes the access
// of everyone who opened it, in one transaction, so nobody keeps the
// dashboard through a link that is off. false when the dashboard is not the
// owner's.
func (r *customDashboardLinkRepo) DisableLink(ctx context.Context, ownerID, dashboardID string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("disable custom dashboard link: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once Commit has succeeded

	tag, err := tx.Exec(ctx,
		`UPDATE custom_dashboard SET link_enabled = false, link_id = NULL
		 WHERE id = $2::uuid AND owner_user_id = $1::uuid`,
		ownerID, dashboardID)
	if err != nil {
		return false, fmt.Errorf("disable custom dashboard link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM custom_dashboard_link_access WHERE dashboard_id = $1::uuid`, dashboardID); err != nil {
		return false, fmt.Errorf("disable custom dashboard link: remove access: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("disable custom dashboard link: commit: %w", err)
	}
	return true, nil
}

// OpenLink records that userID opened the link linkID and returns its
// dashboard. A row is added only when the user would not see the dashboard
// otherwise: not for its owner, nor for a user it is already shared with
// directly or through a team they are in now (the Shared Dashboards list
// shows one entry per dashboard either way, see sharedWithUserFrom). Opening
// it again adds nothing. "" when no dashboard has that link on, or the user
// is not internal staff (internal only, as team shares).
//
// The dashboard row is read FOR SHARE so an open cannot overlap DisableLink:
// the access row's foreign key only takes FOR KEY SHARE, which does not wait
// for the switch-off's update, so without it an open that read the link as on
// could commit its row after DisableLink removed the others. With it the open
// waits for the switch-off and re-checks link_enabled (no row, ""), or locks
// first and its row is removed by DisableLink's delete, which runs after.
func (r *customDashboardLinkRepo) OpenLink(ctx context.Context, userID, linkID string) (string, bool, error) {
	var dashboardID string
	var own bool
	err := r.db.QueryRow(ctx,
		`WITH d AS (
		   SELECT id, owner_user_id FROM custom_dashboard WHERE link_id = $2::uuid AND link_enabled
		   FOR SHARE
		 ), me AS (
		   SELECT id FROM "user" WHERE id = $1::uuid AND user_type = 'INTERNAL'
		 ), ins AS (
		   INSERT INTO custom_dashboard_link_access (dashboard_id, user_id)
		   SELECT d.id, me.id FROM d, me
		   WHERE d.owner_user_id IS DISTINCT FROM me.id
		     AND NOT EXISTS (SELECT 1 FROM custom_dashboard_share ps
		                     WHERE ps.dashboard_id = d.id AND ps.shared_with_user_id = me.id)
		     AND NOT EXISTS (SELECT 1 FROM custom_dashboard_team_share ts
		                     JOIN team_member tm ON tm.team_id = ts.team_id AND tm.user_id = me.id
		                     WHERE ts.dashboard_id = d.id)
		   ON CONFLICT (dashboard_id, user_id) DO NOTHING
		   RETURNING 1
		 )
		 SELECT d.id::text, d.owner_user_id IS NOT DISTINCT FROM me.id FROM d, me`,
		userID, linkID).Scan(&dashboardID, &own)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("open custom dashboard link: %w", err)
	}
	return dashboardID, own, nil
}
