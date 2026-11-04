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
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These run CustomDashboardTeamShareRepository against a live PostgreSQL
// instance, on the fixture users and pool of
// custom_dashboard_repo_integration_test.go (owner A, B), plus two teams of
// their own. Skipped unless ENTITY_TEST_DATABASE_URL is set.

const (
	cdbTeamOrion = "cdb00000-0000-4000-8000-0000000000e1"
	cdbTeamLyra  = "cdb00000-0000-4000-8000-0000000000e2"
)

type teamShareFixture struct {
	shares CustomDashboardTeamShareRepository
	pool   *pgxpool.Pool
	ctx    context.Context
}

// newTeamShareFixture: A owns dashboards 1 ("Zeta") and 2 ("Alpha"); teams
// "Orion Test" (cre-abt) and "Lyra Test" (sre-abt) exist.
func newTeamShareFixture(t *testing.T) teamShareFixture {
	t.Helper()
	pool := newCustomDashboardIntegrationPool(t)
	ctx := context.Background()
	clean := func() {
		// custom_dashboard_team_share rows cascade with their teams.
		if _, err := pool.Exec(ctx, `DELETE FROM team WHERE id = ANY($1::uuid[])`,
			[]string{cdbTeamOrion, cdbTeamLyra}); err != nil {
			t.Fatalf("clean teams: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	for _, team := range []struct{ id, name, typ string }{
		{cdbTeamOrion, "Orion Test", "cre-abt"},
		{cdbTeamLyra, "Lyra Test", "sre-abt"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO team (id, created_on, updated_on, name, type) VALUES ($1, now(), now(), $2, $3)`,
			team.id, team.name, team.typ); err != nil {
			t.Fatalf("seed team %s: %v", team.name, err)
		}
	}
	dashboards := NewCustomDashboardRepository(pool)
	saveCustomDashboard(t, dashboards, cdbOwnerA, cdbDashboard1, "Zeta", `{"v":1}`)
	saveCustomDashboard(t, dashboards, cdbOwnerA, cdbDashboard2, "Alpha", `{"v":2}`)
	return teamShareFixture{shares: NewCustomDashboardTeamShareRepository(pool), pool: pool, ctx: ctx}
}

func (f teamShareFixture) add(t *testing.T, owner, dashboard, team string) bool {
	t.Helper()
	ok, err := f.shares.AddTeamShare(f.ctx, owner, dashboard, team)
	if err != nil {
		t.Fatalf("AddTeamShare: %v", err)
	}
	return ok
}

func (f teamShareFixture) teamNames(t *testing.T, owner, dashboard string) []string {
	t.Helper()
	list, err := f.shares.ListTeamShares(f.ctx, owner, dashboard)
	if err != nil {
		t.Fatalf("ListTeamShares: %v", err)
	}
	names := []string{}
	for _, sh := range list {
		names = append(names, sh.Team.Name)
	}
	return names
}

func TestCustomDashboardTeamShareIntegration_OwnerAddsListsAndRemoves(t *testing.T) {
	f := newTeamShareFixture(t)

	if !f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamOrion) || !f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamLyra) {
		t.Fatal("AddTeamShare on the owner's dashboard returned false")
	}
	// Sharing again is not an error and adds no second row.
	if !f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamOrion) {
		t.Fatal("AddTeamShare a second time returned false")
	}

	list, err := f.shares.ListTeamShares(f.ctx, cdbOwnerA, cdbDashboard1)
	if err != nil {
		t.Fatalf("ListTeamShares: %v", err)
	}
	if len(list) != 2 || list[0].Team.Name != "Lyra Test" || list[1].Team.Name != "Orion Test" {
		t.Fatalf("team shares = %+v, want Lyra Test then Orion Test", list)
	}
	if list[1].Team.ID != cdbTeamOrion || list[1].Team.Type != "cre-abt" || list[1].SharedOn.IsZero() {
		t.Errorf("team share = %+v", list[1])
	}
	// Dashboard 2 is shared with no team.
	if names := f.teamNames(t, cdbOwnerA, cdbDashboard2); len(names) != 0 {
		t.Errorf("dashboard 2 team shares = %v, want none", names)
	}

	removed, err := f.shares.RemoveTeamShare(f.ctx, cdbOwnerA, cdbDashboard1, cdbTeamLyra)
	if err != nil || !removed {
		t.Fatalf("RemoveTeamShare = %v, %v; want true, nil", removed, err)
	}
	again, err := f.shares.RemoveTeamShare(f.ctx, cdbOwnerA, cdbDashboard1, cdbTeamLyra)
	if err != nil || again {
		t.Fatalf("second RemoveTeamShare = %v, %v; want false, nil", again, err)
	}
	if names := f.teamNames(t, cdbOwnerA, cdbDashboard1); len(names) != 1 || names[0] != "Orion Test" {
		t.Fatalf("team shares after remove = %v, want Orion Test only", names)
	}
}

func TestCustomDashboardTeamShareIntegration_OnlyTheOwnerManagesTeamShares(t *testing.T) {
	f := newTeamShareFixture(t)
	f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamOrion)

	// B does not own dashboard 1: cannot add, list or remove its team shares.
	if ok := f.add(t, cdbOwnerB, cdbDashboard1, cdbTeamLyra); ok {
		t.Error("B could share A's dashboard with a team")
	}
	if list, err := f.shares.ListTeamShares(f.ctx, cdbOwnerB, cdbDashboard1); err != nil || len(list) != 0 {
		t.Errorf("B listed A's team shares: %+v, %v", list, err)
	}
	if removed, err := f.shares.RemoveTeamShare(f.ctx, cdbOwnerB, cdbDashboard1, cdbTeamOrion); err != nil || removed {
		t.Errorf("B removed a team share of A's dashboard: %v, %v", removed, err)
	}

	if names := f.teamNames(t, cdbOwnerA, cdbDashboard1); len(names) != 1 || names[0] != "Orion Test" {
		t.Fatalf("A's team shares after the attempts = %v, want Orion Test only", names)
	}
}

func TestCustomDashboardTeamShareIntegration_UnknownTeamAndDashboard(t *testing.T) {
	f := newTeamShareFixture(t)

	_, err := f.shares.AddTeamShare(f.ctx, cdbOwnerA, cdbDashboard1, "cdb00000-0000-4000-8000-0000000000ef")
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("AddTeamShare with an unknown team: err = %v, want a NotFoundError", err)
	}
	if ok := f.add(t, cdbOwnerA, "cdb00000-0000-4000-8000-0000000000fe", cdbTeamOrion); ok {
		t.Error("AddTeamShare on a dashboard that does not exist returned true")
	}
}

func TestCustomDashboardTeamShareIntegration_SharesGoWithTheTeamAndTheDashboard(t *testing.T) {
	f := newTeamShareFixture(t)
	f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamOrion)
	f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamLyra)
	f.add(t, cdbOwnerA, cdbDashboard2, cdbTeamOrion)

	// A team that is deleted is no longer listed.
	if _, err := f.pool.Exec(f.ctx, `DELETE FROM team WHERE id = $1`, cdbTeamLyra); err != nil {
		t.Fatalf("delete team: %v", err)
	}
	if names := f.teamNames(t, cdbOwnerA, cdbDashboard1); len(names) != 1 || names[0] != "Orion Test" {
		t.Fatalf("team shares after the team was deleted = %v, want Orion Test only", names)
	}

	// A dashboard that is deleted takes its team shares with it.
	if _, err := f.pool.Exec(f.ctx, `DELETE FROM custom_dashboard WHERE id = $1`, cdbDashboard2); err != nil {
		t.Fatalf("delete dashboard: %v", err)
	}
	var left int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM custom_dashboard_team_share WHERE dashboard_id = $1`, cdbDashboard2).Scan(&left); err != nil {
		t.Fatalf("count: %v", err)
	}
	if left != 0 {
		t.Errorf("team share rows of the deleted dashboard = %d, want 0", left)
	}
}

// joinTeam makes user a member of team, internal staff unless typ says otherwise.
func (f teamShareFixture) joinTeam(t *testing.T, user, team, typ string) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, `UPDATE "user" SET user_type = $2::user_type_enum WHERE id = $1`, user, typ); err != nil {
		t.Fatalf("set user type: %v", err)
	}
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO team_member (created_on, updated_on, team_id, user_id) VALUES (now(), now(), $1, $2)`, team, user); err != nil {
		t.Fatalf("join team: %v", err)
	}
}

func (f teamShareFixture) sharedWith(t *testing.T, user string) []domain.SharedCustomDashboardSummary {
	t.Helper()
	list, err := NewCustomDashboardShareRepository(f.pool).ListSharedWithUser(f.ctx, user)
	if err != nil {
		t.Fatalf("ListSharedWithUser: %v", err)
	}
	return list
}

func TestCustomDashboardTeamShareIntegration_TeamMembersSeeTheDashboard(t *testing.T) {
	f := newTeamShareFixture(t)
	recipients := NewCustomDashboardShareRepository(f.pool)
	f.joinTeam(t, cdbOwnerB, cdbTeamOrion, "INTERNAL")
	f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamOrion)

	list := f.sharedWith(t, cdbOwnerB)
	if len(list) != 1 || list[0].ID != cdbDashboard1 || list[0].Name != "Zeta" {
		t.Fatalf("shared with B = %+v, want Zeta", list)
	}
	via := list[0].SharedViaTeam
	if via == nil || via.ID != cdbTeamOrion || via.Name != "Orion Test" || via.Type != "cre-abt" || list[0].SharedOn.IsZero() {
		t.Fatalf("sharedViaTeam = %+v, sharedOn %v; want Orion Test", via, list[0].SharedOn)
	}
	got, err := recipients.GetSharedWithUser(f.ctx, cdbOwnerB, cdbDashboard1)
	if err != nil || got == nil || got.Definition != `{"v":1}` || got.SharedViaTeam == nil || got.SharedViaTeam.Name != "Orion Test" {
		t.Fatalf("GetSharedWithUser = %+v, %v", got, err)
	}
	// Dashboard 2 is not shared with Orion.
	if none, err := recipients.GetSharedWithUser(f.ctx, cdbOwnerB, cdbDashboard2); err != nil || none != nil {
		t.Errorf("B got dashboard 2: %+v, %v", none, err)
	}
}

func TestCustomDashboardTeamShareIntegration_MembershipIsLive(t *testing.T) {
	f := newTeamShareFixture(t)
	f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamOrion)

	// Not in the team: nothing.
	if list := f.sharedWith(t, cdbOwnerB); len(list) != 0 {
		t.Fatalf("shared with B before joining = %+v, want none", list)
	}
	// Joins after the share was made: sees it.
	f.joinTeam(t, cdbOwnerB, cdbTeamOrion, "INTERNAL")
	if list := f.sharedWith(t, cdbOwnerB); len(list) != 1 {
		t.Fatalf("shared with B after joining = %+v, want Zeta", list)
	}
	// Leaves: no longer sees it.
	if _, err := f.pool.Exec(f.ctx, `DELETE FROM team_member WHERE user_id = $1`, cdbOwnerB); err != nil {
		t.Fatalf("leave team: %v", err)
	}
	if list := f.sharedWith(t, cdbOwnerB); len(list) != 0 {
		t.Fatalf("shared with B after leaving = %+v, want none", list)
	}
	// The team share is removed: a member no longer sees it either.
	f.joinTeam(t, cdbOwnerB, cdbTeamOrion, "INTERNAL")
	if _, err := f.shares.RemoveTeamShare(f.ctx, cdbOwnerA, cdbDashboard1, cdbTeamOrion); err != nil {
		t.Fatalf("RemoveTeamShare: %v", err)
	}
	if list := f.sharedWith(t, cdbOwnerB); len(list) != 0 {
		t.Fatalf("shared with B after the team share was removed = %+v, want none", list)
	}
}

func TestCustomDashboardTeamShareIntegration_OnlyInternalStaffAndNeverTheOwner(t *testing.T) {
	f := newTeamShareFixture(t)
	f.joinTeam(t, cdbOwnerA, cdbTeamOrion, "INTERNAL")
	f.joinTeam(t, cdbOwnerB, cdbTeamOrion, "EXTERNAL")
	f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamOrion)

	// The owner is in the team, but their own dashboard is not "shared with" them.
	if list := f.sharedWith(t, cdbOwnerA); len(list) != 0 {
		t.Errorf("shared with the owner = %+v, want none", list)
	}
	// An external member does not get it through the team.
	if list := f.sharedWith(t, cdbOwnerB); len(list) != 0 {
		t.Errorf("shared with an external member = %+v, want none", list)
	}
}

func TestCustomDashboardTeamShareIntegration_OneEntryPerDashboard(t *testing.T) {
	f := newTeamShareFixture(t)
	f.joinTeam(t, cdbOwnerB, cdbTeamOrion, "INTERNAL")
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO team_member (created_on, updated_on, team_id, user_id) VALUES (now(), now(), $1, $2)`, cdbTeamLyra, cdbOwnerB); err != nil {
		t.Fatalf("join second team: %v", err)
	}
	// Dashboard 1: shared with both of B's teams; the first by name is named.
	f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamOrion)
	f.add(t, cdbOwnerA, cdbDashboard1, cdbTeamLyra)
	// Dashboard 2: shared with B's team and with B directly; the direct share wins.
	f.add(t, cdbOwnerA, cdbDashboard2, cdbTeamOrion)
	if ok, err := NewCustomDashboardShareRepository(f.pool).AddShare(f.ctx, cdbOwnerA, cdbDashboard2, cdbOwnerB); err != nil || !ok {
		t.Fatalf("AddShare = %v, %v", ok, err)
	}

	list := f.sharedWith(t, cdbOwnerB)
	if len(list) != 2 || list[0].Name != "Alpha" || list[1].Name != "Zeta" {
		t.Fatalf("shared with B = %+v, want Alpha then Zeta, once each", list)
	}
	if list[0].SharedViaTeam != nil {
		t.Errorf("Alpha sharedViaTeam = %+v, want nil (shared directly)", list[0].SharedViaTeam)
	}
	if list[1].SharedViaTeam == nil || list[1].SharedViaTeam.Name != "Lyra Test" {
		t.Errorf("Zeta sharedViaTeam = %+v, want Lyra Test", list[1].SharedViaTeam)
	}
}

// Owner D and three dashboards of D's own, for the test of deleting the owner:
// D is removed during the test, so A's fixture dashboards are left alone.
const (
	cdbOwnerD            = "cdb00000-0000-4000-8000-00000000000d"
	cdbDashboardPersonD  = "cdb00000-0000-4000-8000-0000000000d1"
	cdbDashboardTeamD    = "cdb00000-0000-4000-8000-0000000000d2"
	cdbDashboardUnshared = "cdb00000-0000-4000-8000-0000000000d3"
)

// TestCustomDashboardIntegration_DeletingTheOwnerKeepsOnlySharedDashboards
// deletes an owner with a dashboard shared with a person, one shared with a
// team and one shared with nobody (migration 0255): only the shared two stay,
// with no owner, still visible to their recipients, and nobody can change them.
func TestCustomDashboardIntegration_DeletingTheOwnerKeepsOnlySharedDashboards(t *testing.T) {
	f := newTeamShareFixture(t)
	ids := []string{cdbDashboardPersonD, cdbDashboardTeamD, cdbDashboardUnshared}
	clean := func() {
		// Shared dashboards outlive D with no owner, so they go by id.
		if _, err := f.pool.Exec(f.ctx, `DELETE FROM custom_dashboard WHERE id = ANY($1::uuid[])`, ids); err != nil {
			t.Fatalf("clean dashboards: %v", err)
		}
		if _, err := f.pool.Exec(f.ctx, `DELETE FROM "user" WHERE id = $1`, cdbOwnerD); err != nil {
			t.Fatalf("clean user D: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name, email, first_name, last_name)
		VALUES ($1, now(), now(), 'cdb-test-d', 'dan.d@example.test', 'Dan', 'Dee')`, cdbOwnerD); err != nil {
		t.Fatalf("seed user D: %v", err)
	}
	dashboards := NewCustomDashboardRepository(f.pool)
	people := NewCustomDashboardShareRepository(f.pool)
	saveCustomDashboard(t, dashboards, cdbOwnerD, cdbDashboardPersonD, "Person shared", `{"v":1}`)
	saveCustomDashboard(t, dashboards, cdbOwnerD, cdbDashboardTeamD, "Team shared", `{"v":2}`)
	saveCustomDashboard(t, dashboards, cdbOwnerD, cdbDashboardUnshared, "Unshared", `{"v":3}`)
	if ok, err := people.AddShare(f.ctx, cdbOwnerD, cdbDashboardPersonD, cdbOwnerA); err != nil || !ok {
		t.Fatalf("AddShare = %v, %v", ok, err)
	}
	f.joinTeam(t, cdbOwnerB, cdbTeamOrion, "INTERNAL")
	f.add(t, cdbOwnerD, cdbDashboardTeamD, cdbTeamOrion)

	if _, err := f.pool.Exec(f.ctx, `DELETE FROM "user" WHERE id = $1`, cdbOwnerD); err != nil {
		t.Fatalf("delete user D: %v", err)
	}

	// The unshared one went with D; the shared ones stay, with no owner.
	var left []string
	if err := f.pool.QueryRow(f.ctx,
		`SELECT COALESCE(array_agg(name ORDER BY name), '{}') FROM custom_dashboard
		 WHERE id = ANY($1::uuid[]) AND owner_user_id IS NULL`, ids).Scan(&left); err != nil {
		t.Fatalf("read dashboards: %v", err)
	}
	if len(left) != 2 || left[0] != "Person shared" || left[1] != "Team shared" {
		t.Fatalf("dashboards left = %v, want Person shared and Team shared, with no owner", left)
	}

	// The person and the team member still see theirs, with no owner.
	toA, err := people.ListSharedWithUser(f.ctx, cdbOwnerA)
	if err != nil || len(toA) != 1 || toA[0].ID != cdbDashboardPersonD || toA[0].Owner != nil {
		t.Fatalf("shared with A = %+v, %v; want Person shared with no owner", toA, err)
	}
	got, err := people.GetSharedWithUser(f.ctx, cdbOwnerA, cdbDashboardPersonD)
	if err != nil || got == nil || got.Definition != `{"v":1}` || got.Owner != nil {
		t.Fatalf("GetSharedWithUser A = %+v, %v", got, err)
	}
	toB := f.sharedWith(t, cdbOwnerB)
	if len(toB) != 1 || toB[0].ID != cdbDashboardTeamD || toB[0].Owner != nil || toB[0].SharedViaTeam == nil {
		t.Fatalf("shared with B = %+v; want Team shared through Orion, with no owner", toB)
	}

	// Nobody can change or delete them now: the owner routes match no one.
	if d, err := dashboards.Upsert(f.ctx, cdbOwnerA, cdbDashboardPersonD, domain.SaveCustomDashboardRequest{Name: "Taken", Definition: `{"v":9}`}); err != nil || d != nil {
		t.Errorf("A's Upsert of the ownerless dashboard = %+v, %v; want nil, nil", d, err)
	}
	if deleted, err := dashboards.Delete(f.ctx, cdbOwnerA, cdbDashboardPersonD); err != nil || deleted {
		t.Errorf("A's Delete of the ownerless dashboard = %v, %v; want false, nil", deleted, err)
	}
	if ok, err := people.AddShare(f.ctx, cdbOwnerA, cdbDashboardPersonD, cdbOwnerB); err != nil || ok {
		t.Errorf("A's AddShare on the ownerless dashboard = %v, %v; want false, nil", ok, err)
	}
	if got, _ := people.GetSharedWithUser(f.ctx, cdbOwnerA, cdbDashboardPersonD); got == nil || got.Definition != `{"v":1}` {
		t.Errorf("after the refused edits A sees %+v, want the unchanged dashboard", got)
	}
}
