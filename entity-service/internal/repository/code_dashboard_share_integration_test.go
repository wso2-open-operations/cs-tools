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
	"errors"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These check migration 0296 (sharing code dashboards) against a live
// PostgreSQL instance, on top of the code dashboard fixture. Skipped unless
// ENTITY_TEST_DATABASE_URL is set.

// Fixture recipient and teams, distinct from every other integration file's.
const (
	codeRecipientC = "c0de0000-0000-4000-8000-00000000000c"
	codeTeamVega   = "c0de0000-0000-4000-8000-0000000000e1"
	codeTeamAltair = "c0de0000-0000-4000-8000-0000000000e2"
)

type codeShareFixture struct {
	codeDashboardFixture
}

// newCodeShareFixture adds recipient C and teams Vega and Altair to the code
// dashboard fixture (owners A and B), removing them before and after the test.
func newCodeShareFixture(t *testing.T) codeShareFixture {
	t.Helper()
	f := codeShareFixture{newCodeDashboardFixture(t)}
	clean := func() {
		f.exec(t, "clean recipient", `DELETE FROM "user" WHERE id = $1`, codeRecipientC)
		f.exec(t, "clean teams", `DELETE FROM team WHERE id = ANY($1::uuid[])`, []string{codeTeamVega, codeTeamAltair})
	}
	clean()
	t.Cleanup(clean)
	f.exec(t, "seed recipient", `INSERT INTO "user" (id, created_on, updated_on, user_name) VALUES ($1, now(), now(), $2)`,
		codeRecipientC, "code-test-"+codeRecipientC)
	for _, team := range []struct{ id, name string }{{codeTeamVega, "Vega Test"}, {codeTeamAltair, "Altair Test"}} {
		f.exec(t, "seed team", `INSERT INTO team (id, created_on, updated_on, name, type) VALUES ($1, now(), now(), $2, 'cre-abt')`,
			team.id, team.name)
	}
	return f
}

func (f codeShareFixture) shareWith(t *testing.T, dashboard, user string) {
	t.Helper()
	f.exec(t, "share with user", `INSERT INTO code_dashboard_share (dashboard_id, shared_with_user_id) VALUES ($1, $2)`, dashboard, user)
}

func (f codeShareFixture) shareWithTeam(t *testing.T, dashboard, team string) {
	t.Helper()
	f.exec(t, "share with team", `INSERT INTO code_dashboard_team_share (dashboard_id, team_id) VALUES ($1, $2)`, dashboard, team)
}

// linkOn turns the dashboard's link on with a new link id, as the owner's
// switch will.
func (f codeShareFixture) linkOn(t *testing.T, dashboard string) {
	t.Helper()
	f.exec(t, "link on", `UPDATE code_dashboard SET link_enabled = true, link_id = gen_random_uuid() WHERE id = $1`, dashboard)
}

func (f codeShareFixture) opened(t *testing.T, dashboard, user string) {
	t.Helper()
	f.exec(t, "open link", `INSERT INTO code_dashboard_link_access (dashboard_id, user_id) VALUES ($1, $2)`, dashboard, user)
}

// count runs a SELECT count(*) and returns it.
func (f codeShareFixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func (f codeShareFixture) deleteUser(t *testing.T, id string) {
	t.Helper()
	f.exec(t, "delete user", `DELETE FROM "user" WHERE id = $1`, id)
}

// The share tables reference code_dashboard, not custom_dashboard; one row
// per (dashboard, recipient); the link id is there exactly when the link is
// on, and one link id names one dashboard.
func TestCodeDashboardShareIntegration_Constraints(t *testing.T) {
	f := newCodeShareFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "One")
	f.insert(t, codeOwnerA, codeDashboard2, "Two")
	f.shareWith(t, codeDashboard1, codeRecipientC)
	f.shareWithTeam(t, codeDashboard1, codeTeamVega)

	cases := []struct {
		name, sql string
		args      []any
		want      string // SQLSTATE
	}{
		{"same person twice", `INSERT INTO code_dashboard_share (dashboard_id, shared_with_user_id) VALUES ($1, $2)`,
			[]any{codeDashboard1, codeRecipientC}, "23505"},
		{"same team twice", `INSERT INTO code_dashboard_team_share (dashboard_id, team_id) VALUES ($1, $2)`,
			[]any{codeDashboard1, codeTeamVega}, "23505"},
		{"unknown dashboard", `INSERT INTO code_dashboard_share (dashboard_id, shared_with_user_id) VALUES ($1, $2)`,
			[]any{codeDashboard3, codeRecipientC}, "23503"},
		{"unknown team", `INSERT INTO code_dashboard_team_share (dashboard_id, team_id) VALUES ($1, $2)`,
			[]any{codeDashboard1, codeDashboard3}, "23503"},
		{"link access to an unknown dashboard", `INSERT INTO code_dashboard_link_access (dashboard_id, user_id) VALUES ($1, $2)`,
			[]any{codeDashboard3, codeRecipientC}, "23503"},
		{"link on without an id", `UPDATE code_dashboard SET link_enabled = true WHERE id = $1`, []any{codeDashboard1}, "23514"},
		{"an id with the link off", `UPDATE code_dashboard SET link_id = gen_random_uuid() WHERE id = $1`, []any{codeDashboard1}, "23514"},
	}
	for _, c := range cases {
		_, err := f.pool.Exec(f.ctx, c.sql, c.args...)
		if got := sqlstate(err); got != c.want {
			t.Errorf("%s: SQLSTATE %q (%v), want %q", c.name, got, err, c.want)
		}
	}

	f.linkOn(t, codeDashboard1)
	_, err := f.pool.Exec(f.ctx, `UPDATE code_dashboard SET link_enabled = true, link_id = (SELECT link_id FROM code_dashboard WHERE id = $1) WHERE id = $2`,
		codeDashboard1, codeDashboard2)
	if sqlstate(err) != "23505" {
		t.Errorf("two dashboards with one link id: SQLSTATE %q (%v), want 23505", sqlstate(err), err)
	}
	if n := f.count(t, `SELECT count(*) FROM code_dashboard WHERE id = $1 AND NOT link_enabled AND link_id IS NULL`, codeDashboard2); n != 1 {
		t.Error("a new dashboard's link is not off by default")
	}
}

// Shares go with the dashboard, with the recipient and with the team.
func TestCodeDashboardShareIntegration_SharesGoWithWhatTheyName(t *testing.T) {
	f := newCodeShareFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "One")
	f.insert(t, codeOwnerA, codeDashboard2, "Two")
	for _, d := range []string{codeDashboard1, codeDashboard2} {
		f.shareWith(t, d, codeRecipientC)
		f.shareWithTeam(t, d, codeTeamVega)
		f.linkOn(t, d)
		f.opened(t, d, codeOwnerB)
	}

	f.exec(t, "delete dashboard 1", `DELETE FROM code_dashboard WHERE id = $1`, codeDashboard1)
	for _, table := range []string{"code_dashboard_share", "code_dashboard_team_share", "code_dashboard_link_access"} {
		if n := f.count(t, `SELECT count(*) FROM `+table+` WHERE dashboard_id = $1`, codeDashboard1); n != 0 {
			t.Errorf("%s: %d rows left for the deleted dashboard", table, n)
		}
	}
	f.deleteUser(t, codeRecipientC)
	f.exec(t, "delete team Vega", `DELETE FROM team WHERE id = $1`, codeTeamVega)
	f.deleteUser(t, codeOwnerB)
	if n := f.count(t, `SELECT (SELECT count(*) FROM code_dashboard_share WHERE dashboard_id = $1)
		+ (SELECT count(*) FROM code_dashboard_team_share WHERE dashboard_id = $1)
		+ (SELECT count(*) FROM code_dashboard_link_access WHERE dashboard_id = $1)`, codeDashboard2); n != 0 {
		t.Errorf("%d share rows left after the recipient, the team and the link opener were deleted", n)
	}
	if !f.exists(t, codeDashboard2) {
		t.Error("an owned dashboard went with its recipients")
	}
}

// Deleting the owner keeps a dashboard a person or a team still has, with no
// owner, and deletes the one shared with nobody. Its link is turned off and
// everyone who had it only through the link loses it.
func TestCodeDashboardShareIntegration_DeletingTheOwnerKeepsOnlySharedDashboards(t *testing.T) {
	f := newCodeShareFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "Person")
	f.insert(t, codeOwnerA, codeDashboard2, "Team")
	f.insert(t, codeOwnerA, codeDashboard3, "Link only")
	f.shareWith(t, codeDashboard1, codeRecipientC)
	f.shareWithTeam(t, codeDashboard2, codeTeamVega)
	for _, d := range []string{codeDashboard1, codeDashboard2, codeDashboard3} {
		f.linkOn(t, d)
		f.opened(t, d, codeOwnerB)
	}

	f.deleteUser(t, codeOwnerA)
	if !f.exists(t, codeDashboard1) || !f.exists(t, codeDashboard2) {
		t.Fatal("a dashboard a person or a team still has went with its owner")
	}
	if f.exists(t, codeDashboard3) {
		t.Error("a dashboard only its link kept stayed after its owner went")
	}
	if n := f.count(t, `SELECT count(*) FROM code_dashboard WHERE id = ANY($1::uuid[]) AND owner_user_id IS NULL AND NOT link_enabled AND link_id IS NULL`,
		[]string{codeDashboard1, codeDashboard2}); n != 2 {
		t.Errorf("%d of 2 kept dashboards are ownerless with their link off", n)
	}
	if n := f.count(t, `SELECT count(*) FROM code_dashboard_link_access WHERE user_id = $1`, codeOwnerB); n != 0 {
		t.Errorf("%d link opens left after the owner went", n)
	}
	if n := f.count(t, `SELECT count(*) FROM code_dashboard_share WHERE dashboard_id = $1`, codeDashboard1); n != 1 {
		t.Error("the person share went with the owner")
	}
}

// An ownerless dashboard stays while a person or a team has it and goes with
// the last of them, whichever goes last.
func TestCodeDashboardShareIntegration_GoesWithItsLastShare(t *testing.T) {
	f := newCodeShareFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "Person last")
	f.insert(t, codeOwnerA, codeDashboard2, "Team last")
	f.shareWith(t, codeDashboard1, codeRecipientC)
	f.shareWithTeam(t, codeDashboard1, codeTeamVega)
	f.shareWithTeam(t, codeDashboard2, codeTeamVega)
	f.shareWithTeam(t, codeDashboard2, codeTeamAltair)
	f.deleteUser(t, codeOwnerA)

	f.exec(t, "delete team Vega", `DELETE FROM team WHERE id = $1`, codeTeamVega)
	if !f.exists(t, codeDashboard1) {
		t.Fatal("the dashboard went with the team while C still had it")
	}
	if !f.exists(t, codeDashboard2) {
		t.Fatal("the dashboard went with team Vega while team Altair still had it")
	}
	f.deleteUser(t, codeRecipientC)
	if f.exists(t, codeDashboard1) {
		t.Error("the dashboard is still there after its last person share went")
	}
	f.exec(t, "delete team Altair", `DELETE FROM team WHERE id = $1`, codeTeamAltair)
	if f.exists(t, codeDashboard2) {
		t.Error("the dashboard is still there after its last team share went")
	}
}

// A dashboard that has its owner is never deleted when its last share goes.
func TestCodeDashboardShareIntegration_AnOwnedDashboardStays(t *testing.T) {
	f := newCodeShareFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "Owned")
	f.insert(t, codeOwnerA, codeDashboard2, "Owned too")
	f.shareWith(t, codeDashboard1, codeRecipientC)
	f.shareWithTeam(t, codeDashboard2, codeTeamVega)
	f.linkOn(t, codeDashboard1)

	f.exec(t, "unshare", `DELETE FROM code_dashboard_share WHERE dashboard_id = $1`, codeDashboard1)
	f.exec(t, "delete team Vega", `DELETE FROM team WHERE id = $1`, codeTeamVega)
	f.exec(t, "link off", `UPDATE code_dashboard SET link_enabled = false, link_id = NULL WHERE id = $1`, codeDashboard1)
	if !f.exists(t, codeDashboard1) || !f.exists(t, codeDashboard2) {
		t.Error("an owned dashboard was deleted when its last share or its link went")
	}
}

// The owner and the only recipient deleted in one statement: the dashboard
// goes, whichever trigger runs last.
func TestCodeDashboardShareIntegration_OwnerAndLastRecipientTogether(t *testing.T) {
	f := newCodeShareFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "Orphan")
	f.shareWith(t, codeDashboard1, codeRecipientC)

	f.exec(t, "delete both users", `DELETE FROM "user" WHERE id = ANY($1::uuid[])`, []string{codeOwnerA, codeRecipientC})
	if f.exists(t, codeDashboard1) {
		t.Error("the dashboard is still there after its owner and its only recipient went together")
	}
}

// No API turns an ownerless dashboard's link on or off; when something does,
// one nobody else has goes once the link is off, and one shared stays.
func TestCodeDashboardShareIntegration_TurningAnOrphansLinkOff(t *testing.T) {
	f := newCodeShareFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "Link only")
	f.insert(t, codeOwnerA, codeDashboard2, "Link and a share")
	f.shareWithTeam(t, codeDashboard1, codeTeamVega)
	f.shareWith(t, codeDashboard2, codeRecipientC)
	f.deleteUser(t, codeOwnerA)

	both := []string{codeDashboard1, codeDashboard2}
	f.exec(t, "links on", `UPDATE code_dashboard SET link_enabled = true, link_id = gen_random_uuid() WHERE id = ANY($1::uuid[])`, both)
	f.exec(t, "delete team Vega", `DELETE FROM team WHERE id = $1`, codeTeamVega)
	if !f.exists(t, codeDashboard1) {
		t.Fatal("the ownerless dashboard went with its last share although its link is on")
	}
	f.exec(t, "links off", `UPDATE code_dashboard SET link_enabled = false, link_id = NULL WHERE id = ANY($1::uuid[])`, both)
	if f.exists(t, codeDashboard1) {
		t.Error("the ownerless dashboard nobody has is still there after its link was turned off")
	}
	if !f.exists(t, codeDashboard2) {
		t.Error("the ownerless dashboard shared with C went when its link was turned off")
	}
}

// customDashboardFunctionsAsMigrated checks that each custom dashboard
// trigger function in the database is still the body its own migration
// gives it (0276, 0277): 0296 must not change any of them.
func customDashboardFunctionsAsMigrated(t *testing.T, f codeShareFixture) {
	t.Helper()
	defined := map[string]string{
		"custom_dashboard_delete_unshared_orphan": "0276_custom_dashboard_owner_deletion_turns_link_off.sql",
		"custom_dashboard_delete_if_orphaned":     "0277_custom_dashboard_delete_orphan_after_last_share.sql",
		"custom_dashboard_share_deleted":          "0277_custom_dashboard_delete_orphan_after_last_share.sql",
		"custom_dashboard_orphan_link_off":        "0277_custom_dashboard_delete_orphan_after_last_share.sql",
	}
	for name, file := range defined {
		sql, err := os.ReadFile("../../migrations/" + file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		m := regexp.MustCompile(`(?s)FUNCTION ` + name + `\([^)]*\) RETURNS \w+ AS \$\$(.*?)\$\$`).FindSubmatch(sql)
		if m == nil {
			t.Fatalf("%s: no body for %s", file, name)
		}
		var body string
		if err := f.pool.QueryRow(f.ctx, `SELECT prosrc FROM pg_proc WHERE proname = $1`, name).Scan(&body); err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if body != string(m[1]) {
			t.Errorf("%s is not the body %s gives it", name, file)
		}
	}
}

// The migration runs again on a database that has it, keeping every share,
// and leaves the custom dashboard functions as their migrations made them.
func TestCodeDashboardShareIntegration_MigrationReruns(t *testing.T) {
	f := newCodeShareFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "Shared")
	f.shareWith(t, codeDashboard1, codeRecipientC)
	f.shareWithTeam(t, codeDashboard1, codeTeamVega)
	f.linkOn(t, codeDashboard1)
	f.opened(t, codeDashboard1, codeOwnerB)
	customDashboardFunctionsAsMigrated(t, f)

	sql, err := os.ReadFile("../../migrations/0296_code_dashboard_share.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	f.exec(t, "re-run 0296", string(sql))
	if n := f.count(t, `SELECT (SELECT count(*) FROM code_dashboard_share WHERE dashboard_id = $1) + (SELECT count(*) FROM code_dashboard_team_share WHERE dashboard_id = $1)
		+ (SELECT count(*) FROM code_dashboard_link_access WHERE dashboard_id = $1) + (SELECT count(*) FROM code_dashboard WHERE id = $1 AND link_enabled)`, codeDashboard1); n != 4 {
		t.Errorf("after a re-run %d of 4 share rows and link are left", n)
	}
	customDashboardFunctionsAsMigrated(t, f)
	f.deleteUser(t, codeOwnerA)
	if !f.exists(t, codeDashboard1) {
		t.Error("after a re-run, a shared dashboard went with its owner")
	}
}

// A JSON dashboard id for the test that the two kinds stay apart.
const codeJSONDashboard = "c0de0000-0000-4000-8000-0000000000f1"

// sharesRepo is the repository under test on the fixture's pool.
func (f codeShareFixture) sharesRepo() CodeDashboardShareRepository {
	return NewCodeDashboardShareRepository(f.pool)
}

// userType sets user's type; team and link shares are for internal staff only.
func (f codeShareFixture) userType(t *testing.T, user, typ string) {
	t.Helper()
	f.exec(t, "set user type", `UPDATE "user" SET user_type = $2::user_type_enum WHERE id = $1`, user, typ)
}

func (f codeShareFixture) joinTeam(t *testing.T, user, team string) {
	t.Helper()
	f.exec(t, "join team", `INSERT INTO team_member (created_on, updated_on, team_id, user_id) VALUES (now(), now(), $1, $2)`, team, user)
}

// sharedWith is ListSharedWithUser, failing the test on an error.
func (f codeShareFixture) sharedWith(t *testing.T, user string) []domain.SharedCustomDashboardSummary {
	t.Helper()
	list, err := f.sharesRepo().ListSharedWithUser(f.ctx, user)
	if err != nil {
		t.Fatalf("ListSharedWithUser: %v", err)
	}
	return list
}

func (f codeShareFixture) enable(t *testing.T, owner, dashboard string) *domain.CustomDashboardLink {
	t.Helper()
	l, err := f.sharesRepo().EnableLink(f.ctx, owner, dashboard)
	if err != nil || l == nil || !l.Enabled || l.LinkID == "" {
		t.Fatalf("EnableLink = %+v, %v", l, err)
	}
	return l
}

func TestCodeDashboardShareRepoIntegration_OwnerAddsListsAndRemoves(t *testing.T) {
	f := newCodeShareFixture(t)
	r := f.sharesRepo()
	f.insert(t, codeOwnerA, codeDashboard1, "One")

	for i := 0; i < 2; i++ { // sharing again is not an error
		if ok, err := r.AddShare(f.ctx, codeOwnerA, codeDashboard1, codeRecipientC); err != nil || !ok {
			t.Fatalf("AddShare #%d = %v, %v", i, ok, err)
		}
		if ok, err := r.AddTeamShare(f.ctx, codeOwnerA, codeDashboard1, codeTeamVega); err != nil || !ok {
			t.Fatalf("AddTeamShare #%d = %v, %v", i, ok, err)
		}
	}
	if ok, err := r.AddTeamShare(f.ctx, codeOwnerA, codeDashboard1, codeTeamAltair); err != nil || !ok {
		t.Fatalf("AddTeamShare Altair = %v, %v", ok, err)
	}
	shares, err := r.ListShares(f.ctx, codeOwnerA, codeDashboard1)
	if err != nil || len(shares) != 1 || shares[0].User.ID == nil || *shares[0].User.ID != codeRecipientC || shares[0].SharedOn.IsZero() {
		t.Fatalf("ListShares = %+v, %v; want C", shares, err)
	}
	teams, err := r.ListTeamShares(f.ctx, codeOwnerA, codeDashboard1)
	if err != nil || len(teams) != 2 || teams[0].Team.Name != "Altair Test" || teams[1].Team.Name != "Vega Test" || teams[0].Team.Type != "cre-abt" {
		t.Fatalf("ListTeamShares = %+v, %v; want Altair then Vega", teams, err)
	}

	if ok, err := r.RemoveShare(f.ctx, codeOwnerA, codeDashboard1, codeRecipientC); err != nil || !ok {
		t.Fatalf("RemoveShare = %v, %v", ok, err)
	}
	if ok, err := r.RemoveShare(f.ctx, codeOwnerA, codeDashboard1, codeRecipientC); err != nil || ok {
		t.Errorf("RemoveShare again = %v, %v; want false", ok, err)
	}
	if ok, err := r.RemoveTeamShare(f.ctx, codeOwnerA, codeDashboard1, codeTeamVega); err != nil || !ok {
		t.Fatalf("RemoveTeamShare = %v, %v", ok, err)
	}
	if ok, err := r.RemoveTeamShare(f.ctx, codeOwnerA, codeDashboard1, codeTeamVega); err != nil || ok {
		t.Errorf("RemoveTeamShare again = %v, %v; want false", ok, err)
	}
	if shares, _ := r.ListShares(f.ctx, codeOwnerA, codeDashboard1); len(shares) != 0 {
		t.Errorf("ListShares after removing = %+v", shares)
	}
	if teams, _ := r.ListTeamShares(f.ctx, codeOwnerA, codeDashboard1); len(teams) != 1 || teams[0].Team.ID != codeTeamAltair {
		t.Errorf("ListTeamShares after removing Vega = %+v", teams)
	}
}

// Another user's dashboard is the same as a missing one, on every owner call.
func TestCodeDashboardShareRepoIntegration_OnlyTheOwnerManagesSharing(t *testing.T) {
	f := newCodeShareFixture(t)
	r := f.sharesRepo()
	f.insert(t, codeOwnerA, codeDashboard1, "A's")
	f.shareWith(t, codeDashboard1, codeRecipientC)
	f.shareWithTeam(t, codeDashboard1, codeTeamVega)
	f.linkOn(t, codeDashboard1)

	if ok, err := r.AddShare(f.ctx, codeOwnerB, codeDashboard1, codeRecipientC); err != nil || ok {
		t.Errorf("B AddShare = %v, %v", ok, err)
	}
	if ok, err := r.RemoveShare(f.ctx, codeOwnerB, codeDashboard1, codeRecipientC); err != nil || ok {
		t.Errorf("B RemoveShare = %v, %v", ok, err)
	}
	if ok, err := r.AddTeamShare(f.ctx, codeOwnerB, codeDashboard1, codeTeamAltair); err != nil || ok {
		t.Errorf("B AddTeamShare = %v, %v", ok, err)
	}
	if ok, err := r.RemoveTeamShare(f.ctx, codeOwnerB, codeDashboard1, codeTeamVega); err != nil || ok {
		t.Errorf("B RemoveTeamShare = %v, %v", ok, err)
	}
	if s, err := r.ListShares(f.ctx, codeOwnerB, codeDashboard1); err != nil || len(s) != 0 {
		t.Errorf("B ListShares = %+v, %v", s, err)
	}
	if s, err := r.ListTeamShares(f.ctx, codeOwnerB, codeDashboard1); err != nil || len(s) != 0 {
		t.Errorf("B ListTeamShares = %+v, %v", s, err)
	}
	if l, err := r.GetLink(f.ctx, codeOwnerB, codeDashboard1); err != nil || l != nil {
		t.Errorf("B GetLink = %+v, %v", l, err)
	}
	if l, err := r.EnableLink(f.ctx, codeOwnerB, codeDashboard1); err != nil || l != nil {
		t.Errorf("B EnableLink = %+v, %v", l, err)
	}
	if ok, err := r.DisableLink(f.ctx, codeOwnerB, codeDashboard1); err != nil || ok {
		t.Errorf("B DisableLink = %v, %v", ok, err)
	}
	if n := f.count(t, `SELECT (SELECT count(*) FROM code_dashboard_share WHERE dashboard_id = $1) + (SELECT count(*) FROM code_dashboard_team_share WHERE dashboard_id = $1)
		+ (SELECT count(*) FROM code_dashboard WHERE id = $1 AND link_enabled)`, codeDashboard1); n != 3 {
		t.Errorf("B changed A's sharing: %d of 3 left", n)
	}
}

func TestCodeDashboardShareRepoIntegration_UnknownUserTeamAndDashboard(t *testing.T) {
	f := newCodeShareFixture(t)
	r := f.sharesRepo()
	f.insert(t, codeOwnerA, codeDashboard1, "One")
	unknown := "c0de0000-0000-4000-8000-0000000000ff"

	var nf *apierror.NotFoundError
	if _, err := r.AddShare(f.ctx, codeOwnerA, codeDashboard1, unknown); !errors.As(err, &nf) || nf.Msg != "user not found" {
		t.Errorf("AddShare unknown user: %v; want user not found", err)
	}
	if _, err := r.AddTeamShare(f.ctx, codeOwnerA, codeDashboard1, unknown); !errors.As(err, &nf) || nf.Msg != "team not found" {
		t.Errorf("AddTeamShare unknown team: %v; want team not found", err)
	}
	if ok, err := r.AddShare(f.ctx, codeOwnerA, codeDashboard3, codeRecipientC); err != nil || ok {
		t.Errorf("AddShare unknown dashboard = %v, %v; want false", ok, err)
	}
	if ok, err := r.AddTeamShare(f.ctx, codeOwnerA, codeDashboard3, codeTeamVega); err != nil || ok {
		t.Errorf("AddTeamShare unknown dashboard = %v, %v; want false", ok, err)
	}
	if l, err := r.GetLink(f.ctx, codeOwnerA, codeDashboard3); err != nil || l != nil {
		t.Errorf("GetLink unknown dashboard = %+v, %v", l, err)
	}
}

// A recipient sees a dashboard shared directly, through a team they are in
// now (internal staff only) and through a link they opened while it is on;
// each once, the direct share first, then the team, then the link; the owner
// never sees their own; the source is the owner's current one.
func TestCodeDashboardShareRepoIntegration_RecipientSeesWhatIsShared(t *testing.T) {
	f := newCodeShareFixture(t)
	r := f.sharesRepo()
	f.insert(t, codeOwnerA, codeDashboard1, "Zeta")
	f.insert(t, codeOwnerA, codeDashboard2, "Alpha")
	f.insert(t, codeOwnerA, codeDashboard3, "Mid")
	f.userType(t, codeRecipientC, "INTERNAL")
	f.userType(t, codeOwnerA, "INTERNAL")
	f.joinTeam(t, codeRecipientC, codeTeamVega)
	f.joinTeam(t, codeOwnerA, codeTeamVega)

	f.shareWith(t, codeDashboard1, codeRecipientC)
	f.shareWithTeam(t, codeDashboard1, codeTeamVega)
	f.shareWithTeam(t, codeDashboard2, codeTeamVega)
	link := f.enable(t, codeOwnerA, codeDashboard3)
	if id, own, err := r.OpenLink(f.ctx, codeRecipientC, link.LinkID); err != nil || id != codeDashboard3 || own {
		t.Fatalf("OpenLink = %q, %v, %v", id, own, err)
	}

	list := f.sharedWith(t, codeRecipientC)
	if len(list) != 3 || list[0].Name != "Alpha" || list[1].Name != "Mid" || list[2].Name != "Zeta" {
		t.Fatalf("shared with C = %+v, want Alpha, Mid, Zeta", list)
	}
	if list[0].SharedViaTeam == nil || list[0].SharedViaTeam.ID != codeTeamVega || list[0].SharedViaLink {
		t.Errorf("Alpha = %+v, want through team Vega", list[0])
	}
	if list[1].SharedViaTeam != nil || !list[1].SharedViaLink {
		t.Errorf("Mid = %+v, want through the link", list[1])
	}
	if list[2].SharedViaTeam != nil || list[2].SharedViaLink || list[2].Owner == nil || list[2].Owner.ID == nil || *list[2].Owner.ID != codeOwnerA {
		t.Errorf("Zeta = %+v, want the direct share, owner A", list[2])
	}
	if own := f.sharedWith(t, codeOwnerA); len(own) != 0 {
		t.Errorf("the owner sees their own dashboards: %+v", own)
	}

	f.exec(t, "owner edits", `UPDATE code_dashboard SET source = 'function render() { return 2; }' WHERE id = $1`, codeDashboard1)
	got, err := r.GetSharedWithUser(f.ctx, codeRecipientC, codeDashboard1)
	if err != nil || got == nil || got.Source != "function render() { return 2; }" || got.Name != "Zeta" || got.SharedViaTeam != nil {
		t.Fatalf("GetSharedWithUser = %+v, %v; want the owner's current source, direct", got, err)
	}
	if none, err := r.GetSharedWithUser(f.ctx, codeOwnerB, codeDashboard1); err != nil || none != nil {
		t.Errorf("B got a dashboard not shared with them: %+v, %v", none, err)
	}
	if own, err := r.GetSharedWithUser(f.ctx, codeOwnerA, codeDashboard1); err != nil || own != nil {
		t.Errorf("the owner got their own as shared: %+v, %v", own, err)
	}

	// Team membership is live, and team and link shares count only for internal staff.
	f.exec(t, "leave team", `DELETE FROM team_member WHERE user_id = $1 AND team_id = $2`, codeRecipientC, codeTeamVega)
	if l := f.sharedWith(t, codeRecipientC); len(l) != 2 || l[0].Name != "Mid" {
		t.Errorf("after leaving the team = %+v, want Mid and Zeta", l)
	}
	f.joinTeam(t, codeRecipientC, codeTeamVega)
	f.userType(t, codeRecipientC, "EXTERNAL")
	if l := f.sharedWith(t, codeRecipientC); len(l) != 1 || l[0].Name != "Zeta" {
		t.Errorf("external C = %+v, want only the direct share", l)
	}
}

func TestCodeDashboardShareRepoIntegration_TheLinkSwitch(t *testing.T) {
	f := newCodeShareFixture(t)
	r := f.sharesRepo()
	f.insert(t, codeOwnerA, codeDashboard1, "Linked")
	f.userType(t, codeRecipientC, "INTERNAL")

	if l, err := r.GetLink(f.ctx, codeOwnerA, codeDashboard1); err != nil || l == nil || l.Enabled || l.LinkID != "" || l.OpenedBy != 0 {
		t.Fatalf("GetLink before = %+v, %v; want off", l, err)
	}
	first := f.enable(t, codeOwnerA, codeDashboard1)
	if again := f.enable(t, codeOwnerA, codeDashboard1); again.LinkID != first.LinkID {
		t.Errorf("turning it on twice changed the link: %s -> %s", first.LinkID, again.LinkID)
	}
	if first.LinkID == codeDashboard1 {
		t.Error("the link id is the dashboard's own id")
	}
	if _, _, err := r.OpenLink(f.ctx, codeRecipientC, first.LinkID); err != nil {
		t.Fatalf("OpenLink: %v", err)
	}
	if _, _, err := r.OpenLink(f.ctx, codeRecipientC, first.LinkID); err != nil {
		t.Fatalf("OpenLink again: %v", err)
	}
	if l, _ := r.GetLink(f.ctx, codeOwnerA, codeDashboard1); l == nil || l.OpenedBy != 1 {
		t.Errorf("GetLink after two opens by C = %+v, want openedBy 1", l)
	}

	// Saving the dashboard again keeps its link.
	if _, err := f.repo.Upsert(f.ctx, codeOwnerA, codeDashboard1, domain.SaveCodeDashboardRequest{Name: "Linked", Source: "function render() { return 3; }"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if l, _ := r.GetLink(f.ctx, codeOwnerA, codeDashboard1); l == nil || !l.Enabled || l.LinkID != first.LinkID || l.OpenedBy != 1 {
		t.Errorf("GetLink after saving = %+v, want the same link, opened by 1", l)
	}

	if ok, err := r.DisableLink(f.ctx, codeOwnerA, codeDashboard1); err != nil || !ok {
		t.Fatalf("DisableLink = %v, %v", ok, err)
	}
	if ok, err := r.DisableLink(f.ctx, codeOwnerA, codeDashboard1); err != nil || !ok {
		t.Errorf("DisableLink when off = %v, %v; want true", ok, err)
	}
	if l := f.sharedWith(t, codeRecipientC); len(l) != 0 {
		t.Errorf("C still has it after the link went off: %+v", l)
	}
	if id, _, err := r.OpenLink(f.ctx, codeRecipientC, first.LinkID); err != nil || id != "" {
		t.Errorf("OpenLink with the old link = %q, %v; want none", id, err)
	}
	if second := f.enable(t, codeOwnerA, codeDashboard1); second.LinkID == first.LinkID || second.OpenedBy != 0 {
		t.Errorf("on again = %+v; want a new link, opened by nobody", second)
	}
}

// The owner opening their own link gets it as their own and adds no row; an
// unknown link, the dashboard's own id, or a caller who is not internal staff
// opens nothing.
func TestCodeDashboardShareRepoIntegration_WhoALinkOpensFor(t *testing.T) {
	f := newCodeShareFixture(t)
	r := f.sharesRepo()
	f.insert(t, codeOwnerA, codeDashboard1, "Linked")
	f.userType(t, codeOwnerA, "INTERNAL")
	f.userType(t, codeRecipientC, "EXTERNAL")
	link := f.enable(t, codeOwnerA, codeDashboard1)

	if id, own, err := r.OpenLink(f.ctx, codeOwnerA, link.LinkID); err != nil || id != codeDashboard1 || !own {
		t.Errorf("owner OpenLink = %q, %v, %v; want own", id, own, err)
	}
	for _, c := range []struct{ name, user, link string }{
		{"external user", codeRecipientC, link.LinkID},
		{"user of no type", codeOwnerB, link.LinkID},
		{"unknown link", codeOwnerA, "c0de0000-0000-4000-8000-0000000000ff"},
		{"the dashboard's id", codeOwnerA, codeDashboard1},
	} {
		if id, _, err := r.OpenLink(f.ctx, c.user, c.link); err != nil || id != "" {
			t.Errorf("%s: OpenLink = %q, %v; want none", c.name, id, err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM code_dashboard_link_access WHERE dashboard_id = $1`, codeDashboard1); n != 0 {
		t.Errorf("%d access rows, want 0", n)
	}
}

// An open that overlaps the switch-off waits for it and adds nothing, as
// for custom dashboards.
func TestCodeDashboardShareRepoIntegration_AnOpenDuringTheSwitchOffAddsNothing(t *testing.T) {
	f := newCodeShareFixture(t)
	r := f.sharesRepo()
	f.insert(t, codeOwnerA, codeDashboard1, "Linked")
	f.userType(t, codeRecipientC, "INTERNAL")
	on := f.enable(t, codeOwnerA, codeDashboard1)

	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(f.ctx) //nolint:errcheck // no-op once Commit has succeeded
	if _, err := tx.Exec(f.ctx, `UPDATE code_dashboard SET link_enabled = false, link_id = NULL WHERE id = $1`, codeDashboard1); err != nil {
		t.Fatalf("switch off: %v", err)
	}
	if _, err := tx.Exec(f.ctx, `DELETE FROM code_dashboard_link_access WHERE dashboard_id = $1`, codeDashboard1); err != nil {
		t.Fatalf("remove access: %v", err)
	}

	type result struct {
		id  string
		err error
	}
	done := make(chan result, 1)
	go func() {
		id, _, err := r.OpenLink(f.ctx, codeRecipientC, on.LinkID)
		done <- result{id, err}
	}()
	deadline := time.Now().Add(5 * time.Second)
	blocked := false
	for time.Now().Before(deadline) && len(done) == 0 {
		if err := f.pool.QueryRow(f.ctx,
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			                WHERE datname = current_database() AND wait_event_type = 'Lock'
			                  AND query LIKE '%code_dashboard_link_access%')`).Scan(&blocked); err != nil {
			t.Fatalf("pg_stat_activity: %v", err)
		}
		if blocked {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !blocked && len(done) == 0 {
		t.Fatal("OpenLink neither blocked on the dashboard row nor finished within 5s")
	}
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatalf("commit switch-off: %v", err)
	}
	if res := <-done; res.err != nil || res.id != "" {
		t.Errorf("OpenLink during the switch-off = %q, %v; want none", res.id, res.err)
	}
	if n := f.count(t, `SELECT count(*) FROM code_dashboard_link_access WHERE dashboard_id = $1`, codeDashboard1); n != 0 {
		t.Errorf("access rows after the switch-off = %d, want 0", n)
	}
}

// The two kinds stay apart: a JSON dashboard shared with C is not in C's code
// dashboard list, the code repository cannot share a JSON dashboard, and the
// JSON one cannot share a code dashboard.
func TestCodeDashboardShareRepoIntegration_JSONAndCodeDashboardsStayApart(t *testing.T) {
	f := newCodeShareFixture(t)
	t.Cleanup(func() {
		f.exec(t, "clean JSON dashboard", `DELETE FROM custom_dashboard WHERE id = $1`, codeJSONDashboard)
	})
	r := f.sharesRepo()
	f.insert(t, codeOwnerA, codeDashboard1, "Code")
	saveCustomDashboard(t, NewCustomDashboardRepository(f.pool), codeOwnerA, codeJSONDashboard, "JSON", `{"v":1}`)
	if ok, err := NewCustomDashboardShareRepository(f.pool).AddShare(f.ctx, codeOwnerA, codeJSONDashboard, codeRecipientC); err != nil || !ok {
		t.Fatalf("JSON AddShare = %v, %v", ok, err)
	}

	if l := f.sharedWith(t, codeRecipientC); len(l) != 0 {
		t.Errorf("C's code dashboard list has a JSON dashboard: %+v", l)
	}
	if got, err := r.GetSharedWithUser(f.ctx, codeRecipientC, codeJSONDashboard); err != nil || got != nil {
		t.Errorf("GetSharedWithUser of a JSON dashboard = %+v, %v", got, err)
	}
	if ok, err := r.AddShare(f.ctx, codeOwnerA, codeJSONDashboard, codeRecipientC); err != nil || ok {
		t.Errorf("code AddShare of a JSON dashboard = %v, %v; want false", ok, err)
	}
	if l, err := r.EnableLink(f.ctx, codeOwnerA, codeJSONDashboard); err != nil || l != nil {
		t.Errorf("code EnableLink of a JSON dashboard = %+v, %v; want nil", l, err)
	}
	if ok, err := NewCustomDashboardShareRepository(f.pool).AddShare(f.ctx, codeOwnerA, codeDashboard1, codeRecipientC); err != nil || ok {
		t.Errorf("JSON AddShare of a code dashboard = %v, %v; want false", ok, err)
	}
	if json, err := NewCustomDashboardShareRepository(f.pool).ListSharedWithUser(f.ctx, codeRecipientC); err != nil || len(json) != 1 || json[0].ID != codeJSONDashboard {
		t.Errorf("C's JSON list = %+v, %v; want only the JSON dashboard", json, err)
	}
}

// A link share counts only while the link is on, even if an access row is
// left behind (the switch-off removes them, but the list does not rely on
// that).
func TestCodeDashboardShareRepoIntegration_ALinkThatIsOffSharesNothing(t *testing.T) {
	f := newCodeShareFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "Linked")
	f.userType(t, codeRecipientC, "INTERNAL")
	f.linkOn(t, codeDashboard1)
	f.opened(t, codeDashboard1, codeRecipientC)
	if l := f.sharedWith(t, codeRecipientC); len(l) != 1 || !l[0].SharedViaLink {
		t.Fatalf("shared with C = %+v, want the dashboard through its link", l)
	}

	f.exec(t, "link off, rows kept", `UPDATE code_dashboard SET link_enabled = false, link_id = NULL WHERE id = $1`, codeDashboard1)
	if l := f.sharedWith(t, codeRecipientC); len(l) != 0 {
		t.Errorf("shared with C after the link went off = %+v, want none", l)
	}
	if got, err := f.sharesRepo().GetSharedWithUser(f.ctx, codeRecipientC, codeDashboard1); err != nil || got != nil {
		t.Errorf("GetSharedWithUser after the link went off = %+v, %v", got, err)
	}
}
