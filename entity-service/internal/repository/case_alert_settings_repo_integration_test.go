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

// The Case Alerts settings SQL (migration 0233) against a live PostgreSQL.
// Skipped unless ENTITY_TEST_DATABASE_URL is set; see
// schedule_repo_integration_test.go for how to build the database. It clears
// every Case Alerts room, route and ladder value first, so point it at a test
// database only.

package repository

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	caseAlertFixtureTeamID = "ca5e0000-0000-4000-8000-000000000001"
	caseAlertFixtureTeam   = "casealertfixture"
	caseAlertActor         = "case.alerts.admin@example.test"
)

func newCaseAlertIntegrationRepo(t *testing.T) (CaseAlertSettingsRepository, *pgxpool.Pool) {
	t.Helper()
	_, pool := newScheduleIntegrationRepo(t)
	for _, stmt := range []string{
		`DELETE FROM paging_group_route`,
		`DELETE FROM paging_chat_room`,
		`DELETE FROM paging_ladder_value`,
	} {
		mustExec(t, pool, stmt)
	}
	mustExec(t, pool, `
		INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', $2, $2, 'sre-abt')
		ON CONFLICT (id) DO NOTHING`, caseAlertFixtureTeamID, caseAlertFixtureTeam)
	return NewCaseAlertSettingsRepository(pool), pool
}

func caseAlertRevision(t *testing.T, repo CaseAlertSettingsRepository) int64 {
	t.Helper()
	rev, err := repo.Revision(context.Background())
	if err != nil {
		t.Fatalf("Revision: %v", err)
	}
	return rev.Revision
}

func caseAlertCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// Rooms: one per scope, a saved URL that never reaches the history, a test
// result that is not a change, and a removal that takes the URL with it.
func TestCaseAlertIntegration_Rooms(t *testing.T) {
	repo, pool := newCaseAlertIntegrationRepo(t)
	ctx := context.Background()
	since := auditSince(t, ctx, pool)
	rev0 := caseAlertRevision(t, repo)

	shared, err := repo.SaveRoom(ctx, caseAlertActor, CaseAlertRoomWrite{Ladder: "SRE", Label: "SRE Case Paging", Enabled: true})
	if err != nil {
		t.Fatalf("SaveRoom shared: %v", err)
	}
	team, err := repo.SaveRoom(ctx, caseAlertActor, CaseAlertRoomWrite{Ladder: "SRE", TeamKey: caseAlertFixtureTeam, Label: "Fixture SRE", Enabled: true, Teamless: true})
	if err != nil {
		t.Fatalf("SaveRoom team: %v", err)
	}
	again, err := repo.SaveRoom(ctx, caseAlertActor, CaseAlertRoomWrite{Ladder: "SRE", TeamKey: caseAlertFixtureTeam, Label: "Fixture SRE renamed", Enabled: false})
	if err != nil || again.ID != team.ID || again.Label != "Fixture SRE renamed" || again.Enabled || again.Teamless {
		t.Fatalf("saving the same scope again = %+v, %v; want the same row changed", again, err)
	}
	if rooms, _ := repo.ListRooms(ctx); len(rooms) != 2 {
		t.Fatalf("rooms = %d, want 2", len(rooms))
	}
	if rev := caseAlertRevision(t, repo); rev != rev0+3 {
		t.Fatalf("revision after three saves = %d, want %d", rev, rev0+3)
	}

	secret := CaseAlertSecret{KeyID: "1", Nonce: []byte("nonce-123456"), Ciphertext: []byte("sealed-bytes-not-a-url")}
	if found, err := repo.SetSecret(ctx, caseAlertActor, shared.ID, secret, "spaces/AAAA · …wxyz"); err != nil || !found {
		t.Fatalf("SetSecret = %v, %v", found, err)
	}
	got, err := repo.GetRoom(ctx, shared.ID)
	if err != nil || got == nil || !got.HasSecret || got.WebhookHint == nil || *got.WebhookHint != "spaces/AAAA · …wxyz" || got.WebhookSetBy == nil {
		t.Fatalf("room after SetSecret = %+v, %v", got, err)
	}
	stored, err := repo.Secret(ctx, shared.ID)
	if err != nil || stored == nil || !bytes.Equal(stored.Ciphertext, secret.Ciphertext) || !bytes.Equal(stored.Nonce, secret.Nonce) {
		t.Fatalf("Secret = %+v, %v", stored, err)
	}
	if n := caseAlertCount(t, pool, `SELECT count(*) FROM team_schedule_audit WHERE table_name = 'paging_chat_secret'`); n != 0 {
		t.Fatalf("the sealed URL table was audited: %d rows", n)
	}
	// A URL as saved (0237) replaces the sealed one, and is not audited either.
	plain := CaseAlertSecret{URL: "https://chat.googleapis.com/v1/spaces/AAAA/messages?key=k&token=plain-url-token"}
	if _, err := repo.SetSecret(ctx, caseAlertActor, shared.ID, plain, "spaces/AAAA · …oken"); err != nil {
		t.Fatalf("SetSecret plain: %v", err)
	}
	if stored, err := repo.Secret(ctx, shared.ID); err != nil || stored == nil || stored.URL != plain.URL || stored.Ciphertext != nil || stored.KeyID != "" {
		t.Fatalf("Secret after a plain URL = %+v, %v", stored, err)
	}
	if all, err := repo.Secrets(ctx); err != nil || all[shared.ID].URL != plain.URL {
		t.Fatalf("Secrets = %+v, %v", all[shared.ID], err)
	}
	if n := caseAlertCount(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE changed_at >= $1 AND (old_row::text LIKE '%plain-url-token%' OR new_row::text LIKE '%plain-url-token%')`, since); n != 0 {
		t.Fatalf("the plain URL reached the history %d times", n)
	}
	// A row with no URL either way is refused.
	if _, err := repo.SetSecret(ctx, caseAlertActor, shared.ID, CaseAlertSecret{}, "x"); err == nil {
		t.Fatal("a secret row with no URL was stored")
	}
	if n := caseAlertCount(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'paging_chat_room' AND actor = $1 AND changed_at >= $2
		  AND (old_row::text LIKE '%sealed-bytes%' OR new_row::text LIKE '%sealed-bytes%')`, caseAlertActor, since); n != 0 {
		t.Fatalf("the sealed URL reached the history %d times", n)
	}
	if n := caseAlertCount(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'paging_chat_room' AND action = 'INSERT' AND actor = $1 AND changed_at >= $2`, caseAlertActor, since); n != 2 {
		t.Fatalf("audited room inserts = %d, want 2", n)
	}

	// A test: one claim per cooldown, and its result does not move the
	// revision -- nothing the ladders use changed.
	revBefore := caseAlertRevision(t, repo)
	now := time.Now().UTC().Truncate(time.Second)
	if ok, err := repo.ClaimTest(ctx, caseAlertActor, shared.ID, now, now.Add(-2*time.Minute)); err != nil || !ok {
		t.Fatalf("first ClaimTest = %v, %v", ok, err)
	}
	if ok, err := repo.ClaimTest(ctx, caseAlertActor, shared.ID, now.Add(time.Second), now.Add(-2*time.Minute+time.Second)); err != nil || ok {
		t.Fatalf("second ClaimTest within the cooldown = %v, %v; want refused", ok, err)
	}
	if err := repo.RecordTest(ctx, caseAlertActor, shared.ID, "delivered", now); err != nil {
		t.Fatalf("RecordTest: %v", err)
	}
	if rev := caseAlertRevision(t, repo); rev != revBefore {
		t.Fatalf("a test moved the revision from %d to %d", revBefore, rev)
	}
	// Replacing the URL clears the test, which was of the old one.
	if _, err := repo.SetSecret(ctx, caseAlertActor, shared.ID, secret, "spaces/AAAA · …abcd"); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetRoom(ctx, shared.ID); got.LastTestStatus != nil || got.LastTestAt != nil {
		t.Fatalf("the old test survived a new URL: %+v", got)
	}

	if found, err := repo.DeleteRoom(ctx, caseAlertActor, shared.ID); err != nil || !found {
		t.Fatalf("DeleteRoom = %v, %v", found, err)
	}
	if s, _ := repo.Secret(ctx, shared.ID); s != nil {
		t.Fatal("the URL outlived its room")
	}
	if found, _ := repo.DeleteRoom(ctx, caseAlertActor, shared.ID); found {
		t.Fatal("deleting a room twice found it twice")
	}
	if n := caseAlertCount(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'paging_chat_room' AND action = 'DELETE' AND actor = $1 AND changed_at >= $2`, caseAlertActor, since); n != 1 {
		t.Fatalf("audited room deletes = %d, want 1", n)
	}

	if fam, ok, err := repo.RosteredTeamFamily(ctx, caseAlertFixtureTeam); err != nil || !ok || fam != "SRE" {
		t.Fatalf("RosteredTeamFamily = %q, %v, %v", fam, ok, err)
	}
}

// Routes: one per group and ladder, to a team or a rota, never both.
func TestCaseAlertIntegration_Routes(t *testing.T) {
	repo, pool := newCaseAlertIntegrationRepo(t)
	ctx := context.Background()

	fam, ok, err := repo.ActiveRotaFamily(ctx, "SRE_SAAS")
	if err != nil || !ok || fam != "SRE" {
		t.Fatalf("ActiveRotaFamily(SRE_SAAS) = %q, %v, %v", fam, ok, err)
	}
	if _, ok, _ := repo.ActiveRotaFamily(ctx, "NO_SUCH"); ok {
		t.Fatal("an unknown rota was found")
	}

	toTeam, err := repo.SaveRoute(ctx, caseAlertActor, CaseAlertRouteWrite{Ladder: "SRE", GroupName: "fixture sre group", TeamKey: caseAlertFixtureTeam, Enabled: true})
	if err != nil {
		t.Fatalf("SaveRoute team: %v", err)
	}
	toRota, err := repo.SaveRoute(ctx, caseAlertActor, CaseAlertRouteWrite{Ladder: "SRE", GroupName: "fixture sre group", RotaCode: "SRE_SAAS", Enabled: true})
	if err != nil || toRota.ID != toTeam.ID || toRota.TeamKey != "" || toRota.RotaCode != "SRE_SAAS" {
		t.Fatalf("re-routing the same group = %+v, %v; want the same row, now to the rota", toRota, err)
	}
	if _, err := repo.SaveRoute(ctx, caseAlertActor, CaseAlertRouteWrite{Ladder: "SRE", GroupName: "Not Lower", TeamKey: caseAlertFixtureTeam, Enabled: true}); err == nil {
		t.Fatal("a group name that is not lower case was stored")
	}
	routes, err := repo.ListRoutes(ctx)
	if err != nil || len(routes) != 1 {
		t.Fatalf("routes = %+v, %v", routes, err)
	}
	if found, err := repo.DeleteRoute(ctx, caseAlertActor, toTeam.ID); err != nil || !found {
		t.Fatalf("DeleteRoute = %v, %v", found, err)
	}
	if n := caseAlertCount(t, pool, `SELECT count(*) FROM paging_group_route`); n != 0 {
		t.Fatalf("routes left = %d", n)
	}

	if err := repo.RecordApplied(ctx, "service:csm-notification-service", 7, "shadow", map[string]any{"rooms": 2}); err != nil {
		t.Fatalf("RecordApplied: %v", err)
	}
	rev, err := repo.Revision(ctx)
	if err != nil || rev.AppliedRevision == nil || *rev.AppliedRevision != 7 || rev.AppliedMode == nil || *rev.AppliedMode != "shadow" {
		t.Fatalf("revision after RecordApplied = %+v, %v", rev, err)
	}
}

// Ladder values: one per key and ladder, saved together, a value that does
// not change moves nothing, and every change is in the history.
func TestCaseAlertIntegration_Values(t *testing.T) {
	repo, pool := newCaseAlertIntegrationRepo(t)
	ctx := context.Background()
	since := auditSince(t, ctx, pool)
	rev0 := caseAlertRevision(t, repo)

	if err := repo.SaveValues(ctx, caseAlertActor, "SRE", map[string][]byte{
		"timing.interval":   []byte(`"5m"`),
		"teams.defaultRota": []byte(`"SRE_SAAS"`),
	}); err != nil {
		t.Fatalf("SaveValues: %v", err)
	}
	rev1 := caseAlertRevision(t, repo)
	if rev1 != rev0+2 {
		t.Fatalf("revision %d -> %d after saving two values, want +2", rev0, rev1)
	}

	// The same value again is no change; a new one replaces the row.
	if err := repo.SaveValues(ctx, caseAlertActor, "SRE", map[string][]byte{"timing.interval": []byte(`"5m"`)}); err != nil {
		t.Fatalf("SaveValues same: %v", err)
	}
	if rev := caseAlertRevision(t, repo); rev != rev1 {
		t.Fatalf("an unchanged value moved the revision %d -> %d", rev1, rev)
	}
	if err := repo.SaveValues(ctx, caseAlertActor, "SRE", map[string][]byte{"timing.interval": []byte(`"10m"`)}); err != nil {
		t.Fatalf("SaveValues new: %v", err)
	}
	if rev := caseAlertRevision(t, repo); rev != rev1+1 {
		t.Fatalf("a changed value moved the revision %d -> %d, want +1", rev1, rev)
	}

	values, err := repo.ListValues(ctx)
	if err != nil || len(values) != 2 {
		t.Fatalf("values = %+v, %v", values, err)
	}
	var interval CaseAlertValueRow
	for _, v := range values {
		if v.Key == "timing.interval" {
			interval = v
		}
	}
	if string(interval.Value) != `"10m"` || interval.Ladder != "SRE" || interval.UpdatedBy != caseAlertActor {
		t.Fatalf("timing.interval = %+v", interval)
	}
	if got, err := repo.GetValue(ctx, interval.ID); err != nil || got == nil || got.Key != "timing.interval" {
		t.Fatalf("GetValue = %+v, %v", got, err)
	}

	// A key that is not shaped like the file's is refused by the table too.
	if err := repo.SaveValues(ctx, caseAlertActor, "SRE", map[string][]byte{"timing interval": []byte(`"5m"`)}); err == nil {
		t.Fatal("a key with a space was stored")
	}
	if err := repo.SaveValues(ctx, caseAlertActor, "SME", map[string][]byte{"onLeave": []byte(`"skip"`)}); err == nil {
		t.Fatal("an SME value was stored")
	}

	if found, err := repo.DeleteValue(ctx, caseAlertActor, interval.ID); err != nil || !found {
		t.Fatalf("DeleteValue = %v, %v", found, err)
	}
	if found, _ := repo.DeleteValue(ctx, caseAlertActor, interval.ID); found {
		t.Fatal("deleting a value twice found it twice")
	}
	if n := caseAlertCount(t, pool, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'paging_ladder_value' AND actor = $1 AND changed_at >= $2`, caseAlertActor, since); n != 4 {
		t.Fatalf("audited value changes = %d, want 4 (two inserts, one update, one delete)", n)
	}

	// The lookups the checks use.
	if ok, err := repo.TeamTypeExists(ctx, "SRE-ABT"); err != nil || !ok {
		t.Fatalf("TeamTypeExists(sre-abt) = %v, %v", ok, err)
	}
	if ok, _ := repo.TeamTypeExists(ctx, "sre-nothing"); ok {
		t.Fatal("an unknown team type was found")
	}
	if ok, err := repo.ShiftCodeExists(ctx, "SRE_TZ1"); err != nil || !ok {
		t.Fatalf("ShiftCodeExists(SRE_TZ1) = %v, %v", ok, err)
	}
	if ok, _ := repo.ShiftCodeExists(ctx, "NO_SUCH_SHIFT"); ok {
		t.Fatal("an unknown shift was found")
	}
	if ok, _ := repo.ActiveUserExists(ctx, "nobody.at.all@example.test"); ok {
		t.Fatal("an unknown user was found")
	}

	// The lists a route is made from, and the check on its team's name.
	groups, err := repo.RouteGroups(ctx)
	if err != nil || !hasCaseAlertTeam(groups, caseAlertFixtureTeam, "SRE") {
		t.Fatalf("RouteGroups = %+v, %v; want the fixture SRE team", groups, err)
	}
	teams, err := repo.RouteTeams(ctx)
	if err != nil || !hasCaseAlertTeam(teams, caseAlertFixtureTeam, "SRE") {
		t.Fatalf("RouteTeams = %+v, %v; want the fixture SRE team", teams, err)
	}
	for _, g := range teams {
		if g.Family == "SME" {
			t.Fatalf("an SME team is offered as a route's target: %+v", g)
		}
	}
	rotas, err := repo.SRERotas(ctx)
	if err != nil || len(rotas) == 0 || rotas[0].Code == "" {
		t.Fatalf("SRERotas = %+v, %v", rotas, err)
	}
	for _, r := range rotas {
		if !strings.HasPrefix(r.Code, "SRE_") {
			t.Fatalf("a rota that is not SRE: %+v", r)
		}
	}
	if fam, ok, err := repo.TeamFamilyByName(ctx, " "+strings.ToUpper(caseAlertFixtureTeam)+" "); err != nil || !ok || fam != "SRE" {
		t.Fatalf("TeamFamilyByName = %q, %v, %v", fam, ok, err)
	}
	if _, ok, _ := repo.TeamFamilyByName(ctx, "no team has this name"); ok {
		t.Fatal("a name no team has was found")
	}
	if cre, err := repo.IsCRETeamName(ctx, "no team has this name"); err != nil || cre {
		t.Fatalf("IsCRETeamName(no team) = %v, %v", cre, err)
	}
	if cre, err := repo.IsCRETeamName(ctx, caseAlertFixtureTeam); err != nil || cre {
		t.Fatalf("IsCRETeamName(an SRE team) = %v, %v", cre, err)
	}

	if err := repo.RecordApplied(ctx, "service:csm-notification-service", 9, "db", map[string]any{
		"running": map[string]any{"ladders": map[string]any{"SRE": map[string]any{"timing.interval": map[string]any{"value": "5m", "source": "saved"}}}},
	}); err != nil {
		t.Fatalf("RecordApplied: %v", err)
	}
	rev, err := repo.Revision(ctx)
	if err != nil || !bytes.Contains(rev.AppliedSummary, []byte(`"timing.interval"`)) {
		t.Fatalf("applied summary = %s, %v", rev.AppliedSummary, err)
	}
}

func hasCaseAlertTeam(teams []CaseAlertTeamRow, key, family string) bool {
	for _, t := range teams {
		if t.Key == key && t.Family == family {
			return true
		}
	}
	return false
}
