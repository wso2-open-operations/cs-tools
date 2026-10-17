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

// Each rota admin role against all three families. See
// schedule_repo_integration_test.go for how to build the database.

package repository

import (
	"context"
	"slices"
	"testing"
)

const (
	// An SME team, so that "reaches no SME team" is asserted against a team
	// this file owns rather than against whatever the database was seeded with.
	schedSmeTeamID  = "5c8e0000-0000-4000-8000-000000000009"
	schedSmeTeamKey = "schedsmefixture"
)

// Every rota admin reaches every team of its own family and none of the other
// two, and administers its own family alone. The family split is enforced in
// RotaAdminTeamsFor's SQL and nowhere else -- the service tests run on a fake
// that returns whatever teams it is given -- so this is the test that holds
// the line for all three roles, not only CRE's.
//
// Every key returned is checked, not only the fixtures: a seeded database
// holds more teams than these, and an admin reaching one of them in another
// family is the same bug.
func TestScheduleIntegration_EachRotaAdminReachesItsOwnFamilyOnly(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	mustExec(t, pool, `
		INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', $2, $2, 'sme-abt')
		ON CONFLICT (id) DO NOTHING`, schedSmeTeamID, schedSmeTeamKey)
	t.Cleanup(func() {
		mustExec(t, pool, `DELETE FROM user_role WHERE user_id = $1`, schedOtherID)
		mustExec(t, pool, `DELETE FROM team WHERE id = $1`, schedSmeTeamID)
	})

	fixtureOf := map[string]string{"CRE": schedTeamKey, "SRE": schedSreTeamKey, "SME": schedSmeTeamKey}
	for _, tc := range []struct{ role, family string }{
		{"cre_rota_admin", "CRE"},
		{"sre_rota_admin", "SRE"},
		{"sme_rota_admin", "SME"},
	} {
		t.Run(tc.role, func(t *testing.T) {
			// Internal staff holding this one role, on no team at all, so
			// every team they reach comes from the role.
			mustExec(t, pool, `DELETE FROM user_role WHERE user_id = $1`, schedOtherID)
			mustExec(t, pool, `
				INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
				SELECT gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', $1, r.id
				  FROM role r WHERE r.name IN ('internal', $2)`, schedOtherID, tc.role)

			teams, err := repo.RotaAdminTeamsFor(ctx, schedOtherEmail)
			if err != nil {
				t.Fatalf("RotaAdminTeamsFor: %v", err)
			}
			for family, key := range fixtureOf {
				if got, want := slices.Contains(teams, key), family == tc.family; got != want {
					t.Errorf("a %s reaching the %s fixture team %s: %v, want %v", tc.role, family, key, got, want)
				}
			}

			var strays []string
			if err := pool.QueryRow(ctx, `
				SELECT COALESCE(array_agg(t.key ORDER BY t.key), '{}')
				  FROM team t
				 WHERE t.key = ANY($1)
				   AND `+teamFamilyExpr+` <> $2`, teams, tc.family).Scan(&strays); err != nil {
				t.Fatalf("family of the teams reached: %v", err)
			}
			if len(strays) > 0 {
				t.Errorf("a %s reached teams outside %s: %v", tc.role, tc.family, strays)
			}

			families, err := repo.RotaAdminFamilies(ctx, schedOtherEmail)
			if err != nil {
				t.Fatalf("RotaAdminFamilies: %v", err)
			}
			if !slices.Equal(families, []string{tc.family}) {
				t.Errorf("a %s administers %v, want [%s]", tc.role, families, tc.family)
			}
		})
	}
}
