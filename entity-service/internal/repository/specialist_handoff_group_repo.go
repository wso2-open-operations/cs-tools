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
)

// SpecialistHandoffTeamGroup is the assignment group of a Special Ops team
// that the handoff configuration names by its Team Schedule key only: the
// team's own id and name.
type SpecialistHandoffTeamGroup struct {
	ID   string
	Name string
}

// SpecialistHandoffGroupRepository gives Special Ops teams configured without
// a groupId their assignment group.
type SpecialistHandoffGroupRepository interface {
	// EnsureTeamGroups returns, by lower-cased key, the id and name of every
	// team whose key (compared without case) is one of keys, and makes sure
	// "group" has a row with that id and name, so an incident can be moved
	// to it (work_item.assignment_group_id references "group"). A group that
	// already has the id -- one the ServiceNow sync brought, or an earlier
	// call's -- is left exactly as it is. A key no team holds is absent from
	// the result.
	EnsureTeamGroups(ctx context.Context, keys []string) (map[string]SpecialistHandoffTeamGroup, error)
}

type specialistHandoffGroupRepo struct {
	db *pgxpool.Pool
}

// NewSpecialistHandoffGroupRepository constructs a
// SpecialistHandoffGroupRepository. team and "group" are not RLS-protected.
func NewSpecialistHandoffGroupRepository(db *pgxpool.Pool) SpecialistHandoffGroupRepository {
	return &specialistHandoffGroupRepo{db: db}
}

// specialistHandoffGroupActor is created_by/updated_by on the groups
// EnsureTeamGroups adds.
const specialistHandoffGroupActor = "specialist-handoff"

// ensureTeamGroupsSQL reads the teams and adds their missing groups in one
// statement, so the two never disagree.
const ensureTeamGroupsSQL = `
	WITH t AS (
		SELECT id, name, LOWER(key) AS key
		FROM team
		WHERE LOWER(key) = ANY($1::text[])
	), added AS (
		INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, description, is_active)
		SELECT id, NOW(), NOW(), $2, $2, name,
		       'Special Ops escalation target: the Team Schedule team ' || key, TRUE
		FROM t
		ON CONFLICT (id) DO NOTHING
	)
	SELECT key, id::text, name FROM t`

// EnsureTeamGroups implements SpecialistHandoffGroupRepository.
func (r *specialistHandoffGroupRepo) EnsureTeamGroups(ctx context.Context, keys []string) (map[string]SpecialistHandoffTeamGroup, error) {
	out := map[string]SpecialistHandoffTeamGroup{}
	if len(keys) == 0 {
		return out, nil
	}
	lower := make([]string, len(keys))
	for i, k := range keys {
		lower[i] = strings.ToLower(strings.TrimSpace(k))
	}
	rows, err := r.db.Query(ctx, ensureTeamGroupsSQL, lower, specialistHandoffGroupActor)
	if err != nil {
		return nil, fmt.Errorf("ensure specialist handoff team groups: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var g SpecialistHandoffTeamGroup
		if err := rows.Scan(&key, &g.ID, &g.Name); err != nil {
			return nil, fmt.Errorf("scan specialist handoff team group: %w", err)
		}
		out[key] = g
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ensure specialist handoff team groups: %w", err)
	}
	return out, nil
}
