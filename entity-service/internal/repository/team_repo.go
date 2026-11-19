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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TeamMemberRow is one row of a team's roster, joined to "user".
type TeamMemberRow struct {
	UserID string
	Name   string
	Email  *string
	// Role is team_member.role (migration 000029) -- "member" or "lead"
	// only, NOT NULL with a default, so always populated (never nil).
	Role string
}

// TeamRepository defines the persistence operations for the team/team_member
// tables (migrations 000028_team_tables.up.sql, 000029_team_member_role.up.sql).
type TeamRepository interface {
	// TeamExists reports whether a team with this id exists, so GetTeamMembers
	// can distinguish "team not found" from "team exists, no members" (an
	// empty roster is a valid, non-error state).
	TeamExists(ctx context.Context, teamID string) (bool, error)
	// GetTeamMembers returns every member of the given team, ordered by name.
	GetTeamMembers(ctx context.Context, teamID string) ([]TeamMemberRow, error)
	// SearchTeams returns a name-filtered, paginated slice of the team table
	// together with the total count of matching rows before pagination.
	SearchTeams(ctx context.Context, searchQuery string, limit, offset int) ([]domain.Team, int, error)
	// ListChatWebhooks returns every team that has a configured
	// chat_webhook_url (migration 0240), name order. A team with none is
	// left out entirely -- there is nothing for a caller to route to.
	ListChatWebhooks(ctx context.Context) ([]domain.TeamChatWebhook, error)
	// ListGroupEmails returns every team that has a configured group_email
	// (migration 0117), keyed by team.key (migration 0152 -- a stable,
	// lower-case handle, unlike name, which csm-notification-service looks
	// up a specific team by, e.g. "americas_cre_team" for its SLA-breach
	// overnight-coverage email), key order. A team with none is left out
	// entirely -- same "nothing for a caller to route to" posture as
	// ListChatWebhooks.
	ListGroupEmails(ctx context.Context) ([]domain.TeamGroupEmail, error)
	// GetTeamDetail returns the one team's own id/name/type plus its
	// chat_webhook_url (nil when unset). A NotFoundError is returned when no
	// team has this id. Backs the generic GET /teams/{id}.
	GetTeamDetail(ctx context.Context, teamID string) (domain.TeamDetail, error)
	// SetChatWebhookURL sets (or, with url nil, clears) the team's own
	// chat_webhook_url. A NotFoundError is returned when no team has this id.
	// Backs the generic PATCH /teams/{id} (the one field it accepts today).
	SetChatWebhookURL(ctx context.Context, teamID string, url *string) error
	// AssignTeamLead makes userID this team's one 'lead' (team_member.role,
	// migration 0170) -- the same role csm-notification-service's incident
	// call-escalation ladder reads to decide who it pages for this team, see
	// that service's own CLAUDE.md. Demotes any other current lead of this
	// team back to 'engineer' (the role's own default) in the same
	// transaction, and adds userID to the roster (role 'lead') if they are
	// not already a member -- so a caller may assign a lead from the full
	// user directory, not only from the team's existing roster. The team row
	// is locked (SELECT ... FOR UPDATE) for the duration of the transaction,
	// serializing two concurrent calls for the same team so they can never
	// both leave a 'lead' row behind -- without it, two assignments to
	// different users racing under READ COMMITTED could each demote-then-
	// promote without ever seeing the other's uncommitted write. A
	// NotFoundError is returned when no team has this id; a ValidationError
	// when no user has userID.
	AssignTeamLead(ctx context.Context, teamID, userID string) error
}

type teamRepo struct {
	db *pgxpool.Pool
}

// NewTeamRepository constructs a TeamRepository backed by the given connection pool.
func NewTeamRepository(db *pgxpool.Pool) TeamRepository {
	return &teamRepo{db: db}
}

func (r *teamRepo) TeamExists(ctx context.Context, teamID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM team WHERE id = $1::uuid)`, teamID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check team exists: %w", err)
	}
	return exists, nil
}

func (r *teamRepo) GetTeamMembers(ctx context.Context, teamID string) ([]TeamMemberRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT u.id, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), '')), u.email, tm.role
		 FROM team_member tm
		 JOIN "user" u ON u.id = tm.user_id
		 WHERE tm.team_id = $1::uuid
		 ORDER BY u.name`,
		teamID,
	)
	if err != nil {
		return nil, fmt.Errorf("query team members: %w", err)
	}
	defer rows.Close()

	var members []TeamMemberRow
	for rows.Next() {
		var m TeamMemberRow
		var name *string
		if err := rows.Scan(&m.UserID, &name, &m.Email, &m.Role); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		if name != nil {
			m.Name = *name
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team members: %w", err)
	}
	return members, nil
}

// SearchTeams implements TeamRepository. The search query is a
// case-insensitive substring match on the team's name.
func (r *teamRepo) SearchTeams(ctx context.Context, searchQuery string, limit, offset int) ([]domain.Team, int, error) {
	where := ""
	args := []any{}
	if searchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(searchQuery)
		args = append(args, "%"+escaped+"%")
		where = " WHERE name ILIKE $1 ESCAPE '\\'"
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM team"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count teams: %w", err)
	}

	dataArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(
		`SELECT id::text, name, type FROM team%s ORDER BY name, id LIMIT $%d OFFSET $%d`,
		where, len(args)+1, len(args)+2), dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query teams: %w", err)
	}
	defer rows.Close()

	teams := make([]domain.Team, 0, limit)
	for rows.Next() {
		var t domain.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.Type); err != nil {
			return nil, 0, fmt.Errorf("scan team: %w", err)
		}
		teams = append(teams, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate teams: %w", err)
	}
	return teams, total, nil
}

// ListChatWebhooks implements TeamRepository.
func (r *teamRepo) ListChatWebhooks(ctx context.Context) ([]domain.TeamChatWebhook, error) {
	rows, err := r.db.Query(ctx,
		`SELECT name, chat_webhook_url FROM team
		 WHERE chat_webhook_url IS NOT NULL AND chat_webhook_url <> ''
		 ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("query team chat webhooks: %w", err)
	}
	defer rows.Close()

	webhooks := []domain.TeamChatWebhook{}
	for rows.Next() {
		var w domain.TeamChatWebhook
		if err := rows.Scan(&w.Name, &w.ChatWebhookURL); err != nil {
			return nil, fmt.Errorf("scan team chat webhook: %w", err)
		}
		webhooks = append(webhooks, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team chat webhooks: %w", err)
	}
	return webhooks, nil
}

// ListGroupEmails implements TeamRepository.
func (r *teamRepo) ListGroupEmails(ctx context.Context) ([]domain.TeamGroupEmail, error) {
	rows, err := r.db.Query(ctx,
		`SELECT key, group_email FROM team
		 WHERE group_email IS NOT NULL AND group_email <> ''
		 ORDER BY key`,
	)
	if err != nil {
		return nil, fmt.Errorf("query team group emails: %w", err)
	}
	defer rows.Close()

	emails := []domain.TeamGroupEmail{}
	for rows.Next() {
		var e domain.TeamGroupEmail
		if err := rows.Scan(&e.Key, &e.GroupEmail); err != nil {
			return nil, fmt.Errorf("scan team group email: %w", err)
		}
		emails = append(emails, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team group emails: %w", err)
	}
	return emails, nil
}

// GetTeamDetail implements TeamRepository.
func (r *teamRepo) GetTeamDetail(ctx context.Context, teamID string) (domain.TeamDetail, error) {
	var d domain.TeamDetail
	err := r.db.QueryRow(ctx, `SELECT id, name, type, chat_webhook_url FROM team WHERE id = $1::uuid`, teamID).
		Scan(&d.ID, &d.Name, &d.Type, &d.ChatWebhookURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TeamDetail{}, &apierror.NotFoundError{Msg: "team not found"}
	}
	if err != nil {
		return domain.TeamDetail{}, fmt.Errorf("get team detail: %w", err)
	}
	return d, nil
}

// SetChatWebhookURL implements TeamRepository.
func (r *teamRepo) SetChatWebhookURL(ctx context.Context, teamID string, url *string) error {
	tag, err := r.db.Exec(ctx, `UPDATE team SET chat_webhook_url = $1, updated_on = NOW() WHERE id = $2::uuid`, url, teamID)
	if err != nil {
		return fmt.Errorf("set team chat webhook url: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "team not found"}
	}
	return nil
}

// AssignTeamLead implements TeamRepository.
func (r *teamRepo) AssignTeamLead(ctx context.Context, teamID, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("assign team lead: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once Commit has succeeded

	// Locks the team row for the rest of this transaction, serializing two
	// concurrent assignments for the same team -- without this, two calls
	// for different users could each demote-then-promote under READ
	// COMMITTED without ever seeing the other's uncommitted write, leaving
	// the team with two 'lead' rows (the escalation ladder's "one lead per
	// team" contract broken). A plain existence check has no such effect.
	var lockedID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM team WHERE id = $1::uuid FOR UPDATE`, teamID).Scan(&lockedID)
	switch {
	case err == nil:
	case errors.Is(err, pgx.ErrNoRows):
		return &apierror.NotFoundError{Msg: "team not found"}
	default:
		return fmt.Errorf("assign team lead: lock team: %w", err)
	}

	var userExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "user" WHERE id = $1::uuid)`, userID).Scan(&userExists); err != nil {
		return fmt.Errorf("assign team lead: check user exists: %w", err)
	}
	if !userExists {
		return &apierror.ValidationError{Msg: "userId does not refer to an existing user"}
	}

	// Demote whoever else currently holds 'lead' on this team -- never
	// touches userID's own row, which the next step either updates or
	// inserts as the new lead.
	if _, err := tx.Exec(ctx,
		`UPDATE team_member SET role = 'engineer', updated_on = NOW()
		 WHERE team_id = $1::uuid AND role = 'lead' AND user_id <> $2::uuid`,
		teamID, userID,
	); err != nil {
		return fmt.Errorf("assign team lead: demote previous lead: %w", err)
	}

	var existingMemberID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM team_member WHERE team_id = $1::uuid AND user_id = $2::uuid LIMIT 1`,
		teamID, userID,
	).Scan(&existingMemberID)
	switch {
	case err == nil:
		if _, err := tx.Exec(ctx,
			`UPDATE team_member SET role = 'lead', updated_on = NOW() WHERE id = $1::uuid`,
			existingMemberID,
		); err != nil {
			return fmt.Errorf("assign team lead: promote existing member: %w", err)
		}
	case errors.Is(err, pgx.ErrNoRows):
		// userID is not yet on this team's roster at all -- add them as its
		// lead rather than refusing, since the caller may legitimately pick
		// anyone from the full user directory, not only the existing roster.
		if _, err := tx.Exec(ctx,
			`INSERT INTO team_member (id, team_id, user_id, role, created_on, updated_on)
			 VALUES (gen_random_uuid(), $1::uuid, $2::uuid, 'lead', NOW(), NOW())`,
			teamID, userID,
		); err != nil {
			return fmt.Errorf("assign team lead: insert new member: %w", err)
		}
	default:
		return fmt.Errorf("assign team lead: check existing membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("assign team lead: commit: %w", err)
	}
	return nil
}
