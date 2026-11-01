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

// Adding puts the person on the team once as an engineer, however often it
// is asked; removing is refused, with nothing written, while upcoming turns,
// a Case Alerts tier or (unless allowed) a lead role keep them on it, and
// otherwise takes them off with their rota marker grant for the team.
func TestScheduleIntegration_TeamMemberAddAndRemove(t *testing.T) {
	_, pool := newScheduleIntegrationRepo(t)
	access := NewRotaAccessRepository(pool)
	ctx := context.Background()
	t.Cleanup(func() {
		mustExec(t, pool, `DELETE FROM team_schedule_assignment WHERE created_by = 'member-fixture'`)
		mustExec(t, pool, `DELETE FROM team_member WHERE user_id = $1`, schedOutsiderID)
		mustExec(t, pool, `DELETE FROM team_schedule_rota_marker WHERE user_id = $1`, schedOutsiderID)
	})
	members := func(userID string) int {
		return countRows(t, pool, `SELECT count(*) FROM team_member m JOIN team t ON t.id = m.team_id
			WHERE t.key = $1 AND m.user_id = $2`, schedTeamKey, userID)
	}

	for i, want := range []bool{true, false} {
		added, err := access.AddTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor)
		if err != nil || added != want {
			t.Fatalf("add #%d = %v, %v; want %v", i+1, added, err, want)
		}
	}
	if n := members(schedOutsiderID); n != 1 {
		t.Fatalf("memberships after two adds = %d, want 1", n)
	}
	var role, createdBy string
	if err := pool.QueryRow(ctx, `SELECT m.role, m.created_by FROM team_member m JOIN team t ON t.id = m.team_id
		WHERE t.key = $1 AND m.user_id = $2`, schedTeamKey, schedOutsiderID).Scan(&role, &createdBy); err != nil {
		t.Fatalf("read membership: %v", err)
	}
	if role != "engineer" || createdBy != schedMarkerActor {
		t.Fatalf("membership = %s by %s; want engineer by %s", role, createdBy, schedMarkerActor)
	}
	if _, err := access.AddTeamMember(ctx, schedOutsiderID, "no-such-team", schedMarkerActor); err == nil {
		t.Fatal("adding to a team that does not exist succeeded")
	}

	// An upcoming turn keeps them on; a past one does not.
	mustExec(t, pool, `
		INSERT INTO team_schedule_assignment
		  (user_id, team_id, team_key, shift_id, rota_date, starts_at, ends_at, created_by, updated_by)
		SELECT $1::uuid, $2::uuid, $3, s.id, (NOW() + interval '2 days')::date,
		       NOW() + interval '2 days', NOW() + interval '2 days 4 hours', 'member-fixture', 'member-fixture'
		  FROM team_schedule_shift s ORDER BY s.code LIMIT 1`, schedOutsiderID, schedTeamID, schedTeamKey)
	mustExec(t, pool, `
		INSERT INTO team_schedule_assignment
		  (user_id, team_id, team_key, shift_id, rota_date, starts_at, ends_at, created_by, updated_by)
		SELECT $1::uuid, $2::uuid, $3, s.id, (NOW() - interval '9 days')::date,
		       NOW() - interval '9 days', NOW() - interval '9 days' + interval '4 hours', 'member-fixture', 'member-fixture'
		  FROM team_schedule_shift s ORDER BY s.code LIMIT 1`, schedOutsiderID, schedTeamID, schedTeamKey)
	res, err := access.RemoveTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor, true)
	if err != nil || res.Removed || !res.Member || res.UpcomingTurns != 1 {
		t.Fatalf("remove with an upcoming turn = %+v, %v; want kept with 1 upcoming turn", res, err)
	}
	if n := members(schedOutsiderID); n != 1 {
		t.Fatalf("a kept member was removed (%d rows)", n)
	}
	mustExec(t, pool, `DELETE FROM team_schedule_assignment WHERE created_by = 'member-fixture' AND ends_at > NOW()`)

	// A responder tier keeps them on.
	mustExec(t, pool, `UPDATE team_member m SET alert_tier = 'T3' FROM team t
		WHERE t.id = m.team_id AND t.key = $1 AND m.user_id = $2`, schedTeamKey, schedOutsiderID)
	if res, err := access.RemoveTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor, true); err != nil || res.Removed || res.AlertTier != "T3" {
		t.Fatalf("remove with a tier = %+v, %v; want kept with T3", res, err)
	}
	mustExec(t, pool, `UPDATE team_member m SET alert_tier = NULL FROM team t
		WHERE t.id = m.team_id AND t.key = $1 AND m.user_id = $2`, schedTeamKey, schedOutsiderID)

	// Nothing keeps them: they go, with their marker grant for the team.
	mustExec(t, pool, `INSERT INTO team_schedule_rota_marker (created_by, updated_by, user_id, team_key)
		VALUES ('member-fixture', 'member-fixture', $1, $2)`, schedOutsiderID, schedTeamKey)
	res, err = access.RemoveTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor, false)
	if err != nil || !res.Removed {
		t.Fatalf("remove = %+v, %v; want removed", res, err)
	}
	if n := members(schedOutsiderID); n != 0 {
		t.Fatalf("memberships after removal = %d, want 0", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_rota_marker WHERE user_id = $1`, schedOutsiderID); n != 0 {
		t.Fatalf("the marker grant outlived the membership (%d rows)", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_assignment WHERE user_id = $1`, schedOutsiderID); n != 1 {
		t.Fatalf("a past turn was touched (%d rows left, want 1)", n)
	}
	if res, err := access.RemoveTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor, false); err != nil || res.Member || res.Removed {
		t.Fatalf("removing again = %+v, %v; want not a member", res, err)
	}

	// A lead stays unless the caller may remove one.
	res, err = access.RemoveTeamMember(ctx, schedLeadID, schedTeamKey, schedMarkerActor, false)
	if err != nil || res.Removed || res.Role != "lead" {
		t.Fatalf("a rota admin removing the lead = %+v, %v; want kept as lead", res, err)
	}
	if n := members(schedLeadID); n == 0 {
		t.Fatal("the lead was removed")
	}
}

// membershipOf reads a person's rows on a team straight from the table:
// the role and tier of each.
func membershipOf(t *testing.T, ctx context.Context, pool *pgxpool.Pool, teamKey, userID string) [][2]string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT m.role, COALESCE(m.alert_tier, '') FROM team_member m JOIN team t ON t.id = m.team_id
		WHERE t.key = $1 AND m.user_id = $2 ORDER BY m.id`, teamKey, userID)
	if err != nil {
		t.Fatalf("read membership: %v", err)
	}
	defer rows.Close()
	var out [][2]string
	for rows.Next() {
		var r [2]string
		if err := rows.Scan(&r[0], &r[1]); err != nil {
			t.Fatalf("scan membership: %v", err)
		}
		out = append(out, r)
	}
	return out
}

func str(s string) *string { return &s }

// Changing a membership writes the row and says so in the audit trail; a
// tier another member holds, a lead, and somebody not on the team are all
// left alone.
func TestScheduleIntegration_TeamMemberRoleAndTier(t *testing.T) {
	_, pool := newScheduleIntegrationRepo(t)
	access := NewRotaAccessRepository(pool)
	ctx := context.Background()
	since := auditSince(t, ctx, pool)
	t.Cleanup(func() { mustExec(t, pool, `DELETE FROM team_member WHERE user_id = $1`, schedOutsiderID) })

	res, err := access.UpdateTeamMember(ctx, schedMemberID, schedTeamKey, schedMarkerActor,
		domain.TeamMemberPatch{Role: str("sub_lead"), AlertTierSet: true, AlertTier: str("T1")})
	if err != nil || !res.Updated || res.Membership.Role != "sub_lead" || res.Membership.AlertTier == nil || *res.Membership.AlertTier != "T1" {
		t.Fatalf("update = %+v, %v; want sub_lead T1", res, err)
	}
	if got := membershipOf(t, ctx, pool, schedTeamKey, schedMemberID); len(got) != 1 || got[0] != [2]string{"sub_lead", "T1"} {
		t.Fatalf("row after update = %v, want [[sub_lead T1]]", got)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'team_member' AND action = 'UPDATE' AND actor = $1 AND changed_at >= $2`, schedMarkerActor, since); n != 1 {
		t.Fatalf("audit rows for the update = %d, want 1", n)
	}

	// The same again changes nothing, and writes nothing.
	res, err = access.UpdateTeamMember(ctx, schedMemberID, schedTeamKey, schedMarkerActor,
		domain.TeamMemberPatch{Role: str("sub_lead"), AlertTierSet: true, AlertTier: str("T1")})
	if err != nil || res.Updated || !res.Member {
		t.Fatalf("repeat = %+v, %v; want no change", res, err)
	}

	// T1 has one holder: the second person is refused, row untouched.
	if _, err := access.AddTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor); err != nil {
		t.Fatalf("add: %v", err)
	}
	res, err = access.UpdateTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor,
		domain.TeamMemberPatch{AlertTierSet: true, AlertTier: str("T1")})
	if err != nil || res.Updated || res.TierHolder != schedMemberID {
		t.Fatalf("a taken tier = %+v, %v; want refused naming the holder", res, err)
	}
	if got := membershipOf(t, ctx, pool, schedTeamKey, schedOutsiderID); len(got) != 1 || got[0] != [2]string{"engineer", ""} {
		t.Fatalf("row after a refused tier = %v, want [[engineer ]]", got)
	}

	// Role alone leaves the tier; null clears it.
	res, err = access.UpdateTeamMember(ctx, schedMemberID, schedTeamKey, schedMarkerActor, domain.TeamMemberPatch{Role: str("engineer")})
	if err != nil || !res.Updated || res.Membership.AlertTier == nil || *res.Membership.AlertTier != "T1" {
		t.Fatalf("role only = %+v, %v; want engineer, tier kept", res, err)
	}
	res, err = access.UpdateTeamMember(ctx, schedMemberID, schedTeamKey, schedMarkerActor, domain.TeamMemberPatch{AlertTierSet: true})
	if err != nil || !res.Updated || res.Membership.AlertTier != nil {
		t.Fatalf("clear = %+v, %v; want no tier", res, err)
	}
	if got := membershipOf(t, ctx, pool, schedTeamKey, schedMemberID); len(got) != 1 || got[0] != [2]string{"engineer", ""} {
		t.Fatalf("row after clearing = %v, want [[engineer ]]", got)
	}

	// A lead is not changed, and neither is somebody who is not on the team.
	res, err = access.UpdateTeamMember(ctx, schedLeadID, schedTeamKey, schedMarkerActor, domain.TeamMemberPatch{Role: str("engineer")})
	if err != nil || !res.Lead || res.Updated {
		t.Fatalf("a lead = %+v, %v; want kept as lead", res, err)
	}
	if got := membershipOf(t, ctx, pool, schedTeamKey, schedLeadID); len(got) != 1 || got[0][0] != "lead" {
		t.Fatalf("the lead's row = %v, want lead", got)
	}
	if res, err := access.UpdateTeamMember(ctx, schedOtherID, schedTeamKey, schedMarkerActor, domain.TeamMemberPatch{Role: str("sub_lead")}); err != nil || res.Member {
		t.Fatalf("a non-member = %+v, %v; want not a member", res, err)
	}
	if _, err := access.UpdateTeamMember(ctx, schedMemberID, "no-such-team", schedMarkerActor, domain.TeamMemberPatch{Role: str("sub_lead")}); err == nil {
		t.Fatal("updating on a team that does not exist succeeded")
	}
}

// A move is one transaction: the person is on the target as the same
// non-lead role without a tier and off the source without their marker, with
// both writes audited; whatever keeps them on the source keeps everything as
// it was; and a failure of the delete takes the insert back with it.
func TestScheduleIntegration_TeamMemberMove(t *testing.T) {
	_, pool := newScheduleIntegrationRepo(t)
	access := NewRotaAccessRepository(pool)
	ctx := context.Background()
	since := auditSince(t, ctx, pool)
	t.Cleanup(func() {
		mustExec(t, pool, `DROP TRIGGER IF EXISTS move_fixture_fail ON team_member`)
		mustExec(t, pool, `DROP FUNCTION IF EXISTS move_fixture_fail()`)
		mustExec(t, pool, `DELETE FROM team_schedule_assignment WHERE created_by = 'move-fixture'`)
		mustExec(t, pool, `DELETE FROM team_member WHERE user_id = $1`, schedOutsiderID)
		mustExec(t, pool, `DELETE FROM team_schedule_rota_marker WHERE user_id = $1`, schedOutsiderID)
	})
	move := func() domain.TeamMemberMove {
		t.Helper()
		res, err := access.MoveTeamMember(ctx, schedOutsiderID, schedTeamKey, schedOtherTeam, schedMarkerActor)
		if err != nil {
			t.Fatalf("move: %v", err)
		}
		return res
	}
	onSource := func() [][2]string { return membershipOf(t, ctx, pool, schedTeamKey, schedOutsiderID) }
	onTarget := func() [][2]string { return membershipOf(t, ctx, pool, schedOtherTeam, schedOutsiderID) }

	// Not on the source: nothing to move.
	if res := move(); res.OnSource || res.Moved {
		t.Fatalf("move of a non-member = %+v", res)
	}
	if _, err := access.AddTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := access.UpdateTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor, domain.TeamMemberPatch{Role: str("sub_lead")}); err != nil {
		t.Fatalf("make sub_lead: %v", err)
	}
	mustExec(t, pool, `INSERT INTO team_schedule_rota_marker (created_by, updated_by, user_id, team_key)
		VALUES ('move-fixture', 'move-fixture', $1, $2)`, schedOutsiderID, schedTeamKey)

	// An upcoming turn keeps them where they are.
	mustExec(t, pool, `
		INSERT INTO team_schedule_assignment
		  (user_id, team_id, team_key, shift_id, rota_date, starts_at, ends_at, created_by, updated_by)
		SELECT $1::uuid, $2::uuid, $3, s.id, (NOW() + interval '2 days')::date,
		       NOW() + interval '2 days', NOW() + interval '2 days 4 hours', 'move-fixture', 'move-fixture'
		  FROM team_schedule_shift s ORDER BY s.code LIMIT 1`, schedOutsiderID, schedTeamID, schedTeamKey)
	if res := move(); res.Moved || res.UpcomingTurns != 1 || len(onTarget()) != 0 || len(onSource()) != 1 {
		t.Fatalf("move with a turn = %+v, source %v, target %v; want refused, nothing written", res, onSource(), onTarget())
	}
	mustExec(t, pool, `DELETE FROM team_schedule_assignment WHERE created_by = 'move-fixture'`)

	// A tier on the source keeps them.
	mustExec(t, pool, `UPDATE team_member m SET alert_tier = 'T2' FROM team t
		WHERE t.id = m.team_id AND t.key = $1 AND m.user_id = $2`, schedTeamKey, schedOutsiderID)
	if res := move(); res.Moved || res.AlertTier != "T2" || len(onTarget()) != 0 || len(onSource()) != 1 {
		t.Fatalf("move with a tier = %+v; want refused, nothing written", res)
	}
	mustExec(t, pool, `UPDATE team_member m SET alert_tier = NULL FROM team t
		WHERE t.id = m.team_id AND t.key = $1 AND m.user_id = $2`, schedTeamKey, schedOutsiderID)

	// The second half failing takes the first back with it.
	mustExec(t, pool, `
		CREATE FUNCTION move_fixture_fail() RETURNS trigger AS $$
		BEGIN
		  IF OLD.user_id = '`+schedOutsiderID+`'::uuid THEN RAISE EXCEPTION 'move fixture: delete refused'; END IF;
		  RETURN OLD;
		END $$ LANGUAGE plpgsql`)
	mustExec(t, pool, `CREATE TRIGGER move_fixture_fail BEFORE DELETE ON team_member
		FOR EACH ROW EXECUTE FUNCTION move_fixture_fail()`)
	if _, err := access.MoveTeamMember(ctx, schedOutsiderID, schedTeamKey, schedOtherTeam, schedMarkerActor); err == nil {
		t.Fatal("a move whose delete fails reported success")
	}
	if len(onTarget()) != 0 || len(onSource()) != 1 {
		t.Fatalf("after a failed move: source %v, target %v; want the insert rolled back", onSource(), onTarget())
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_rota_marker WHERE user_id = $1 AND team_key = $2`, schedOutsiderID, schedTeamKey); n != 1 {
		t.Fatalf("marker rows after a failed move = %d, want 1", n)
	}
	mustExec(t, pool, `DROP TRIGGER move_fixture_fail ON team_member`)

	// Nothing keeps them: they arrive as sub_lead with no tier, and leave.
	res := move()
	if !res.Moved || res.Membership.Role != "sub_lead" || res.Membership.TeamKey != schedOtherTeam || res.Membership.AlertTier != nil {
		t.Fatalf("move = %+v; want moved as sub_lead", res)
	}
	if got := onTarget(); len(got) != 1 || got[0] != [2]string{"sub_lead", ""} {
		t.Fatalf("target rows = %v, want [[sub_lead ]]", got)
	}
	if got := onSource(); len(got) != 0 {
		t.Fatalf("source rows = %v, want none", got)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_rota_marker WHERE user_id = $1 AND team_key = $2`, schedOutsiderID, schedTeamKey); n != 0 {
		t.Fatalf("the source marker outlived the move (%d rows)", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'team_member' AND action = 'INSERT' AND actor = $1 AND changed_at >= $2
		  AND new_row->>'user_id' = $3 AND new_row->>'role' = 'sub_lead'`, schedMarkerActor, since, schedOutsiderID); n < 1 {
		t.Fatalf("the insert on the target was not audited (%d)", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'team_member' AND action = 'DELETE' AND actor = $1 AND changed_at >= $2
		  AND old_row->>'user_id' = $3`, schedMarkerActor, since, schedOutsiderID); n != 1 {
		t.Fatalf("the delete on the source was audited %d times, want 1", n)
	}

	// Again: already there, nothing written. On both: reported, nothing written.
	if res := move(); res.Moved || !res.OnTarget || res.OnSource || res.Membership.Role != "sub_lead" {
		t.Fatalf("repeat move = %+v; want already moved", res)
	}
	if _, err := access.AddTeamMember(ctx, schedOutsiderID, schedTeamKey, schedMarkerActor); err != nil {
		t.Fatalf("add back: %v", err)
	}
	if res := move(); res.Moved || !res.OnSource || !res.OnTarget || len(onSource()) != 1 || len(onTarget()) != 1 {
		t.Fatalf("move when on both = %+v; want refused, both rows kept", res)
	}

	// A lead is not moved.
	lead, err := access.MoveTeamMember(ctx, schedLeadID, schedTeamKey, schedOtherTeam, schedMarkerActor)
	if err != nil || lead.Moved || !lead.Lead {
		t.Fatalf("move of a lead = %+v, %v; want kept as lead", lead, err)
	}
	if n := len(membershipOf(t, ctx, pool, schedTeamKey, schedLeadID)); n != 1 {
		t.Fatalf("the lead's source rows = %d, want 1", n)
	}
	if _, err := access.MoveTeamMember(ctx, schedOutsiderID, schedTeamKey, "no-such-team", schedMarkerActor); err == nil {
		t.Fatal("moving to a team that does not exist succeeded")
	}
}
