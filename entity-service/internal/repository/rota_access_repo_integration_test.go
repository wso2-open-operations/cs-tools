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

// The rota access SQL against a live PostgreSQL, on the schedule fixtures.
// Skipped unless ENTITY_TEST_DATABASE_URL is set; see
// schedule_repo_integration_test.go for how to build the database.

package repository

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const schedMarkerActor = "rota.access.admin@example.test"

// auditSince is the database's clock now. The schedule's history is never
// cleared -- that is what it is for -- so a test counts only what it wrote.
func auditSince(t *testing.T, ctx context.Context, pool *pgxpool.Pool) time.Time {
	t.Helper()
	var at time.Time
	if err := pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
		t.Fatalf("clock: %v", err)
	}
	return at
}

// A marker's grant counts while they are internal staff on the team, and
// stops on its own when they leave it -- no clean-up needed for a leaver.
func TestScheduleIntegration_RotaMarkerCountsWhileOnTheTeam(t *testing.T) {
	schedule, pool := newScheduleIntegrationRepo(t)
	access := NewRotaAccessRepository(pool)
	ctx := context.Background()
	since := auditSince(t, ctx, pool)
	mustExec(t, pool, `
		INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
		SELECT gen_random_uuid(), NOW(), NOW(), 'fixture', 'fixture', $1, r.id
		  FROM role r WHERE r.name = 'internal'`, schedMemberID)

	added, err := access.GrantRotaMarker(ctx, schedMemberID, schedTeamKey, schedMarkerActor)
	if err != nil || !added {
		t.Fatalf("GrantRotaMarker = %v, %v", added, err)
	}
	if again, err := access.GrantRotaMarker(ctx, schedMemberID, schedTeamKey, schedMarkerActor); err != nil || again {
		t.Fatalf("a second grant = %v, %v; want no change", again, err)
	}
	teams, err := schedule.RotaMarkerTeamsFor(ctx, schedMemberEmail)
	if err != nil || !slices.Equal(teams, []string{schedTeamKey}) {
		t.Fatalf("RotaMarkerTeamsFor = %v, %v", teams, err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'team_schedule_rota_marker' AND action = 'INSERT' AND actor = $1 AND changed_at >= $2`, schedMarkerActor, since); n != 1 {
		t.Fatalf("audit rows for the grant = %d, want 1", n)
	}

	// Leaves the team: the grant stays listed, inactive, and stops counting.
	mustExec(t, pool, `DELETE FROM team_member WHERE user_id = $1`, schedMemberID)
	if teams, _ := schedule.RotaMarkerTeamsFor(ctx, schedMemberEmail); len(teams) != 0 {
		t.Fatalf("a marker who left the team still marks %v", teams)
	}
	markers, err := access.ListRotaMarkers(ctx, []string{"CRE"}, nil)
	if err != nil {
		t.Fatalf("ListRotaMarkers: %v", err)
	}
	var found bool
	for _, m := range markers {
		if m.UserID == schedMemberID && m.TeamKey == schedTeamKey {
			found = true
			if m.Active || m.Family != "CRE" || m.GrantedBy != schedMarkerActor {
				t.Fatalf("listed %+v; want inactive, CRE, granted by %s", m, schedMarkerActor)
			}
		}
	}
	if !found {
		t.Fatal("the grant was not listed")
	}
	if sre, _ := access.ListRotaMarkers(ctx, []string{"SRE"}, nil); slices.ContainsFunc(sre, func(m domain.RotaMarkerGrant) bool { return m.UserID == schedMemberID }) {
		t.Fatal("a CRE team's marker was listed under SRE")
	}

	removed, err := access.RevokeRotaMarker(ctx, schedMemberID, schedTeamKey, schedMarkerActor)
	if err != nil || !removed {
		t.Fatalf("RevokeRotaMarker = %v, %v", removed, err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'team_schedule_rota_marker' AND action = 'DELETE' AND actor = $1 AND changed_at >= $2`, schedMarkerActor, since); n != 1 {
		t.Fatalf("audit rows for the removal = %d, want 1", n)
	}
}

// Rota admin grants live in user_role. Granting twice adds one row, removing
// takes every copy, and only rota roles reach the schedule's history.
func TestScheduleIntegration_RotaAdminGrantAndRemoval(t *testing.T) {
	_, pool := newScheduleIntegrationRepo(t)
	access := NewRotaAccessRepository(pool)
	ctx := context.Background()
	since := auditSince(t, ctx, pool)

	for i, want := range []bool{true, false} {
		added, err := access.GrantRotaAdmin(ctx, schedMemberID, "sre_rota_admin", schedMarkerActor)
		if err != nil || added != want {
			t.Fatalf("grant #%d = %v, %v; want %v", i+1, added, err, want)
		}
	}
	admins, err := access.ListRotaAdmins(ctx)
	if err != nil {
		t.Fatalf("ListRotaAdmins: %v", err)
	}
	var listed int
	for _, a := range admins {
		if a.UserID == schedMemberID && a.Family == "SRE" {
			listed++
			if a.Active {
				t.Fatalf("listed %+v as active; the member is not internal staff", a)
			}
		}
	}
	if listed != 1 {
		t.Fatalf("listed the grant %d times, want once", listed)
	}

	removed, err := access.RevokeRotaAdmin(ctx, schedMemberID, "sre_rota_admin", schedMarkerActor)
	if err != nil || !removed {
		t.Fatalf("RevokeRotaAdmin = %v, %v", removed, err)
	}
	if again, _ := access.RevokeRotaAdmin(ctx, schedMemberID, "sre_rota_admin", schedMarkerActor); again {
		t.Fatal("a second removal reported removing something")
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'user_role' AND actor = $1 AND action IN ('INSERT', 'DELETE') AND changed_at >= $2`, schedMarkerActor, since); n != 2 {
		t.Fatalf("audit rows for the rota admin grant and removal = %d, want 2", n)
	}
	// The fixture's own internal-role grant is not a rota change.
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit a
		JOIN role r ON r.id = (a.new_row ->> 'role_id')::uuid
		WHERE a.table_name = 'user_role' AND r.name = 'internal' AND a.changed_at >= $1`, since); n != 0 {
		t.Fatalf("%d internal-role grants reached the schedule's history", n)
	}
}

func TestScheduleIntegration_RotaAccessLookups(t *testing.T) {
	_, pool := newScheduleIntegrationRepo(t)
	access := NewRotaAccessRepository(pool)
	ctx := context.Background()

	if fam, ok, err := access.RosteredTeamFamily(ctx, "SCHEDSREFIXTURE"); err != nil || !ok || fam != "SRE" {
		t.Fatalf("RosteredTeamFamily(sre) = %q, %v, %v", fam, ok, err)
	}
	if _, ok, err := access.RosteredTeamFamily(ctx, "no-such-team"); err != nil || ok {
		t.Fatalf("a missing team was found: %v, %v", ok, err)
	}
	if ok, err := access.UserIsInternal(ctx, schedAdminID); err != nil || !ok {
		t.Fatalf("UserIsInternal(admin) = %v, %v", ok, err)
	}
	if ok, err := access.UserIsInternal(ctx, schedOutsiderID); err != nil || ok {
		t.Fatalf("UserIsInternal(outsider) = %v, %v", ok, err)
	}
}
