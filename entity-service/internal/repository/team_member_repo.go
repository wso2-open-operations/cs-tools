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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TeamMemberRepository reads team membership by rank.
//
// Its own file and its own interface rather than another method on
// ScheduleRepository: that one is about the rota - who is on which window -
// and this is about the org chart behind it, which does not change by the day.
// They share the team table and nothing else.
type TeamMemberRepository interface {
	// MembersByTeamKeys returns every membership of the named teams, newest
	// role first is not a thing, so ordered by team then role then email for a
	// stable response.
	//
	// roles filters when non-empty; an empty slice means every role.
	MembersByTeamKeys(ctx context.Context, teamKeys, roles, alertTiers, teamTypes []string) ([]domain.TeamMemberEntry, error)
}

type teamMemberRepository struct{ db db.Pool }

func NewTeamMemberRepository(db db.Pool) TeamMemberRepository {
	return &teamMemberRepository{db: db}
}

// The join is on team.key rather than team.id because every caller names teams
// the way the rota does - "vega", "cre-leadership" - and has no reason to know
// a uuid. team.key is UNIQUE (migration 000101), so this cannot fan out.
//
// name and email are COALESCEd: both are nullable on "user", and a membership
// whose user row is half filled should still resolve a rung rather than fail
// the whole lookup. An empty email is the caller's problem to notice, not a
// reason to return nothing.
const teamMemberQuery = `
SELECT t.key, COALESCE(t.type, ''), m.role, COALESCE(m.alert_tier, ''), u.id::text, COALESCE(u.name, ''), COALESCE(u.email, '')
  FROM team_member m
  JOIN team t ON t.id = m.team_id
  JOIN "user" u ON u.id = m.user_id
 WHERE (cardinality($1::text[]) = 0 OR t.key = ANY($1))
   AND (cardinality($2::text[]) = 0 OR m.role = ANY($2))
   AND (cardinality($3::text[]) = 0 OR m.alert_tier = ANY($3))
   AND (cardinality($4::text[]) = 0 OR t.type = ANY($4))
   AND (cardinality($1::text[]) > 0 OR cardinality($4::text[]) > 0)
 ORDER BY t.key, m.role, COALESCE(m.alert_tier, ''), COALESCE(u.email, '')`

func (r *teamMemberRepository) MembersByTeamKeys(ctx context.Context, teamKeys, roles, alertTiers, teamTypes []string) ([]domain.TeamMemberEntry, error) {
	// One of the two selectors must be present: an unfiltered read would be
	// every membership in the organisation.
	if len(teamKeys) == 0 && len(teamTypes) == 0 {
		return nil, nil
	}
	if roles == nil {
		roles = []string{}
	}
	if alertTiers == nil {
		alertTiers = []string{}
	}
	if teamTypes == nil {
		teamTypes = []string{}
	}
	rows, err := r.db.Query(ctx, teamMemberQuery, teamKeys, roles, alertTiers, teamTypes)
	if err != nil {
		return nil, fmt.Errorf("query team members: %w", err)
	}
	defer rows.Close()

	var out []domain.TeamMemberEntry
	for rows.Next() {
		var e domain.TeamMemberEntry
		if err := rows.Scan(&e.TeamKey, &e.TeamType, &e.Role, &e.AlertTier, &e.UserID, &e.Name, &e.Email); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team members: %w", err)
	}
	return out, nil
}
