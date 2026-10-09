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
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
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
