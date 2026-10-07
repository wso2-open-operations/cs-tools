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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
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
}

type teamRepo struct {
	db db.Pool
}

// NewTeamRepository constructs a TeamRepository backed by the given connection pool.
func NewTeamRepository(db db.Pool) TeamRepository {
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
