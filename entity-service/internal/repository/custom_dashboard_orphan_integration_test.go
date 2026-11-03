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

import "testing"

// These check migration 0271: a dashboard whose owner was deleted goes once
// nobody can see it any more, i.e. when its last person or team share goes
// and its link is off. They run on the team share fixture (teams Orion and
// Lyra) with an owner G and a recipient H of their own. Skipped unless
// ENTITY_TEST_DATABASE_URL is set.

const (
	cdbOwnerG     = "cdb00000-0000-4000-8000-000000000010"
	cdbRecipientH = "cdb00000-0000-4000-8000-000000000011"
	cdbOrphanX    = "cdb00000-0000-4000-8000-0000000000a1"
	cdbOrphanY    = "cdb00000-0000-4000-8000-0000000000a2"
)

type orphanFixture struct {
	teamShareFixture
	dashboards CustomDashboardRepository
}

// newOrphanFixture adds G and H; G's dashboards and both users are removed
// after the test, whatever state it left them in.
func newOrphanFixture(t *testing.T) orphanFixture {
	t.Helper()
	f := orphanFixture{teamShareFixture: newTeamShareFixture(t)}
	f.dashboards = NewCustomDashboardRepository(f.pool)
	clean := func() {
		if _, err := f.pool.Exec(f.ctx, `DELETE FROM custom_dashboard WHERE id = ANY($1::uuid[])`,
			[]string{cdbOrphanX, cdbOrphanY}); err != nil {
			t.Fatalf("clean dashboards: %v", err)
		}
		if _, err := f.pool.Exec(f.ctx, `DELETE FROM "user" WHERE id = ANY($1::uuid[])`,
			[]string{cdbOwnerG, cdbRecipientH}); err != nil {
			t.Fatalf("clean users: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	for _, id := range []string{cdbOwnerG, cdbRecipientH} {
		if _, err := f.pool.Exec(f.ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name) VALUES ($1, now(), now(), $2)`,
			id, "cdb-test-"+id); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
	}
	return f
}

// shareWith shares dashboard with user directly.
func (f orphanFixture) shareWith(t *testing.T, dashboard, user string) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO custom_dashboard_share (dashboard_id, shared_with_user_id) VALUES ($1, $2)`,
		dashboard, user); err != nil {
		t.Fatalf("share with user: %v", err)
	}
}

// shareWithTeam shares dashboard with team.
func (f orphanFixture) shareWithTeam(t *testing.T, dashboard, team string) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO custom_dashboard_team_share (dashboard_id, team_id) VALUES ($1, $2)`,
		dashboard, team); err != nil {
		t.Fatalf("share with team: %v", err)
	}
}

// exec runs one statement, failing the test with what on an error.
func (f orphanFixture) exec(t *testing.T, what, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

// deleteUser deletes the user, which runs the owner-deletion rules under test.
func (f orphanFixture) deleteUser(t *testing.T, id string) {
	t.Helper()
	f.exec(t, "delete user", `DELETE FROM "user" WHERE id = $1`, id)
}

// exists reports whether the dashboard row is still there.
func (f orphanFixture) exists(t *testing.T, dashboard string) bool {
	t.Helper()
	var ok bool
	if err := f.pool.QueryRow(f.ctx, `SELECT EXISTS (SELECT 1 FROM custom_dashboard WHERE id = $1)`, dashboard).Scan(&ok); err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	return ok
}

// enableLink turns the dashboard's link on as owner.
func (f orphanFixture) enableLink(t *testing.T, owner, dashboard string) {
	t.Helper()
	if l, err := NewCustomDashboardLinkRepository(f.pool).EnableLink(f.ctx, owner, dashboard); err != nil || l == nil {
		t.Fatalf("EnableLink = %+v, %v", l, err)
	}
}

// An ownerless dashboard stays while a team or a person still has it, and
// goes with the last of them.
func TestCustomDashboardOrphanIntegration_GoesWithItsLastShare(t *testing.T) {
	f := newOrphanFixture(t)
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanX, "Orphan", `{"v":1}`)
	f.shareWith(t, cdbOrphanX, cdbRecipientH)
	f.shareWithTeam(t, cdbOrphanX, cdbTeamOrion)

	f.deleteUser(t, cdbOwnerG)
	if !f.exists(t, cdbOrphanX) {
		t.Fatal("the dashboard went with its owner while a person and a team had it")
	}
	f.exec(t, "delete team", `DELETE FROM team WHERE id = $1`, cdbTeamOrion)
	if !f.exists(t, cdbOrphanX) {
		t.Fatal("the dashboard went with the team while H still had it")
	}
	f.deleteUser(t, cdbRecipientH)
	if f.exists(t, cdbOrphanX) {
		t.Error("the dashboard is still there with no owner, no share and its link off")
	}
}

// The same, with the person going first and the team last.
func TestCustomDashboardOrphanIntegration_GoesWithItsLastTeamShare(t *testing.T) {
	f := newOrphanFixture(t)
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanX, "Orphan", `{"v":1}`)
	f.shareWith(t, cdbOrphanX, cdbRecipientH)
	f.shareWithTeam(t, cdbOrphanX, cdbTeamOrion)
	f.shareWithTeam(t, cdbOrphanX, cdbTeamLyra)

	f.deleteUser(t, cdbOwnerG)
	f.deleteUser(t, cdbRecipientH)
	f.exec(t, "delete team Orion", `DELETE FROM team WHERE id = $1`, cdbTeamOrion)
	if !f.exists(t, cdbOrphanX) {
		t.Fatal("the dashboard went while team Lyra still had it")
	}
	f.exec(t, "delete team Lyra", `DELETE FROM team WHERE id = $1`, cdbTeamLyra)
	if f.exists(t, cdbOrphanX) {
		t.Error("the dashboard is still there after its last team share went")
	}
}

// An ownerless dashboard whose link is on stays when its last share goes:
// anyone with the link can still open it.
func TestCustomDashboardOrphanIntegration_ALinkThatIsOnKeepsIt(t *testing.T) {
	f := newOrphanFixture(t)
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanX, "Linked", `{"v":1}`)
	f.enableLink(t, cdbOwnerG, cdbOrphanX)
	f.shareWith(t, cdbOrphanX, cdbRecipientH)

	f.deleteUser(t, cdbOwnerG)
	f.deleteUser(t, cdbRecipientH)
	if !f.exists(t, cdbOrphanX) {
		t.Error("the dashboard went although its link is on")
	}
}

// A dashboard that still has its owner is never deleted by this: not when
// its last share is removed by the owner, nor when the recipient is deleted.
func TestCustomDashboardOrphanIntegration_AnOwnedDashboardStays(t *testing.T) {
	f := newOrphanFixture(t)
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanX, "Owned", `{"v":1}`)
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanY, "Owned too", `{"v":2}`)
	f.shareWith(t, cdbOrphanX, cdbRecipientH)
	f.shareWithTeam(t, cdbOrphanY, cdbTeamOrion)

	if ok, err := NewCustomDashboardShareRepository(f.pool).RemoveShare(f.ctx, cdbOwnerG, cdbOrphanX, cdbRecipientH); err != nil || !ok {
		t.Fatalf("RemoveShare = %v, %v", ok, err)
	}
	if ok, err := f.shares.RemoveTeamShare(f.ctx, cdbOwnerG, cdbOrphanY, cdbTeamOrion); err != nil || !ok {
		t.Fatalf("RemoveTeamShare = %v, %v", ok, err)
	}
	f.shareWith(t, cdbOrphanX, cdbRecipientH)
	f.deleteUser(t, cdbRecipientH)
	if !f.exists(t, cdbOrphanX) || !f.exists(t, cdbOrphanY) {
		t.Error("an owned dashboard was deleted when its last share went")
	}
}

// The owner and the only recipient deleted in one statement: whichever
// trigger runs last sees the other change, so the dashboard goes.
func TestCustomDashboardOrphanIntegration_OwnerAndLastRecipientTogether(t *testing.T) {
	f := newOrphanFixture(t)
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanX, "Orphan", `{"v":1}`)
	f.shareWith(t, cdbOrphanX, cdbRecipientH)

	f.exec(t, "delete both users", `DELETE FROM "user" WHERE id = ANY($1::uuid[])`, []string{cdbOwnerG, cdbRecipientH})
	if f.exists(t, cdbOrphanX) {
		t.Error("the dashboard is still there after its owner and its only recipient went together")
	}
}

// No API turns off the link of an ownerless dashboard today; when something
// does, a dashboard nobody else has goes with it, and one shared with
// somebody stays.
func TestCustomDashboardOrphanIntegration_TurningAnOrphansLinkOff(t *testing.T) {
	f := newOrphanFixture(t)
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanX, "Link only", `{"v":1}`)
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanY, "Link and a share", `{"v":2}`)
	f.enableLink(t, cdbOwnerG, cdbOrphanX)
	f.enableLink(t, cdbOwnerG, cdbOrphanY)
	f.shareWith(t, cdbOrphanY, cdbRecipientH)
	f.deleteUser(t, cdbOwnerG)

	f.exec(t, "turn the links off", `UPDATE custom_dashboard SET link_enabled = false, link_id = NULL WHERE id = ANY($1::uuid[])`,
		[]string{cdbOrphanX, cdbOrphanY})
	if f.exists(t, cdbOrphanX) {
		t.Error("the ownerless dashboard nobody has is still there after its link was turned off")
	}
	if !f.exists(t, cdbOrphanY) {
		t.Error("the ownerless dashboard shared with H went when its link was turned off")
	}
}
