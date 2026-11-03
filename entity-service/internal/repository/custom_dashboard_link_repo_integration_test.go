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
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These run CustomDashboardLinkRepository against a live PostgreSQL instance,
// on the fixture users and pool of custom_dashboard_repo_integration_test.go
// (owner A, B). Skipped unless ENTITY_TEST_DATABASE_URL is set.

var linkIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type linkFixture struct {
	links      CustomDashboardLinkRepository
	dashboards CustomDashboardRepository
	pool       *pgxpool.Pool
	ctx        context.Context
}

// newLinkFixture: A owns dashboards 1 ("Zeta") and 2 ("Alpha").
func newLinkFixture(t *testing.T) linkFixture {
	t.Helper()
	pool := newCustomDashboardIntegrationPool(t)
	dashboards := NewCustomDashboardRepository(pool)
	saveCustomDashboard(t, dashboards, cdbOwnerA, cdbDashboard1, "Zeta", `{"v":1}`)
	saveCustomDashboard(t, dashboards, cdbOwnerA, cdbDashboard2, "Alpha", `{"v":2}`)
	return linkFixture{links: NewCustomDashboardLinkRepository(pool), dashboards: dashboards, pool: pool, ctx: context.Background()}
}

// get is GetLink as owner, failing the test on an error.
func (f linkFixture) get(t *testing.T, owner, dashboard string) *domain.CustomDashboardLink {
	t.Helper()
	l, err := f.links.GetLink(f.ctx, owner, dashboard)
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	return l
}

// enable is EnableLink as owner, failing the test on an error.
func (f linkFixture) enable(t *testing.T, owner, dashboard string) *domain.CustomDashboardLink {
	t.Helper()
	l, err := f.links.EnableLink(f.ctx, owner, dashboard)
	if err != nil {
		t.Fatalf("EnableLink: %v", err)
	}
	return l
}

// disable is DisableLink as owner, failing the test on an error.
func (f linkFixture) disable(t *testing.T, owner, dashboard string) bool {
	t.Helper()
	ok, err := f.links.DisableLink(f.ctx, owner, dashboard)
	if err != nil {
		t.Fatalf("DisableLink: %v", err)
	}
	return ok
}

// opened records that user opened dashboard's link, as L3's open will.
func (f linkFixture) opened(t *testing.T, dashboard, user string) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx,
		`INSERT INTO custom_dashboard_link_access (dashboard_id, user_id) VALUES ($1, $2)`, dashboard, user); err != nil {
		t.Fatalf("record link access: %v", err)
	}
}

// accessRows counts the dashboard's custom_dashboard_link_access rows.
func (f linkFixture) accessRows(t *testing.T, dashboard string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx,
		`SELECT count(*) FROM custom_dashboard_link_access WHERE dashboard_id = $1`, dashboard).Scan(&n); err != nil {
		t.Fatalf("count link access: %v", err)
	}
	return n
}

// TestCustomDashboardLinkIntegration_OffUntilTurnedOn: a saved dashboard's
// link is off with no id and nobody; another owner's dashboard and an unknown
// one have no link.
func TestCustomDashboardLinkIntegration_OffUntilTurnedOn(t *testing.T) {
	f := newLinkFixture(t)

	l := f.get(t, cdbOwnerA, cdbDashboard1)
	if l == nil || l.Enabled || l.LinkID != "" || l.OpenedBy != 0 {
		t.Fatalf("a saved dashboard's link = %+v, want off with no id and no one", l)
	}
	// Another owner's dashboard, and no dashboard at all, are the same: nil.
	if l := f.get(t, cdbOwnerB, cdbDashboard1); l != nil {
		t.Errorf("owner B's view of A's link = %+v, want nil", l)
	}
	if l := f.get(t, cdbOwnerA, "cdb00000-0000-4000-8000-0000000000ff"); l != nil {
		t.Errorf("an unknown dashboard's link = %+v, want nil", l)
	}
}

// TestCustomDashboardLinkIntegration_TurningOnTwiceKeepsTheLink: turning the
// link on gives a random UUID that is not the dashboard id; turning it on
// again keeps it, and each dashboard has its own.
func TestCustomDashboardLinkIntegration_TurningOnTwiceKeepsTheLink(t *testing.T) {
	f := newLinkFixture(t)

	on := f.enable(t, cdbOwnerA, cdbDashboard1)
	if on == nil || !on.Enabled || on.OpenedBy != 0 {
		t.Fatalf("EnableLink = %+v, want on", on)
	}
	if !linkIDPattern.MatchString(on.LinkID) {
		t.Fatalf("link id %q is not a UUID", on.LinkID)
	}
	if on.LinkID == cdbDashboard1 {
		t.Fatal("the link id is the dashboard id; it must not be guessable from it")
	}
	again := f.enable(t, cdbOwnerA, cdbDashboard1)
	if again == nil || again.LinkID != on.LinkID {
		t.Fatalf("EnableLink again = %+v, want the same link id %s", again, on.LinkID)
	}
	if got := f.get(t, cdbOwnerA, cdbDashboard1); got == nil || *got != *on {
		t.Errorf("GetLink = %+v, want %+v", got, on)
	}
	// Each dashboard has its own link; dashboard 2 stays off.
	if l := f.get(t, cdbOwnerA, cdbDashboard2); l == nil || l.Enabled {
		t.Errorf("dashboard 2's link = %+v, want off", l)
	}
}

// TestCustomDashboardLinkIntegration_TurningOffRemovesEveryoneAndOnAgainIsANew
// Link: turning the link off removes everyone who opened it, turning it off
// again is not an error, and turning it on again gives a new link.
func TestCustomDashboardLinkIntegration_TurningOffRemovesEveryoneAndOnAgainIsANewLink(t *testing.T) {
	f := newLinkFixture(t)

	first := f.enable(t, cdbOwnerA, cdbDashboard1)
	f.opened(t, cdbDashboard1, cdbOwnerB)
	if l := f.get(t, cdbOwnerA, cdbDashboard1); l == nil || l.OpenedBy != 1 {
		t.Fatalf("after B opened the link: %+v, want opened by 1", l)
	}

	if !f.disable(t, cdbOwnerA, cdbDashboard1) {
		t.Fatal("DisableLink on the owner's dashboard returned false")
	}
	if l := f.get(t, cdbOwnerA, cdbDashboard1); l == nil || l.Enabled || l.LinkID != "" || l.OpenedBy != 0 {
		t.Fatalf("after turning off: %+v, want off with no id and no one", l)
	}
	if n := f.accessRows(t, cdbDashboard1); n != 0 {
		t.Errorf("access rows after turning off = %d, want 0", n)
	}
	// Turning off a link that is off is not an error.
	if !f.disable(t, cdbOwnerA, cdbDashboard1) {
		t.Error("DisableLink on a link that is off returned false")
	}

	second := f.enable(t, cdbOwnerA, cdbDashboard1)
	if second == nil || second.LinkID == first.LinkID || second.OpenedBy != 0 {
		t.Fatalf("on again = %+v, want a new link id (was %s) opened by no one", second, first.LinkID)
	}
}

// TestCustomDashboardLinkIntegration_OnlyTheOwnerTurnsItOnOrOff: another owner
// can neither turn the link on nor off, and an unknown dashboard is not found.
func TestCustomDashboardLinkIntegration_OnlyTheOwnerTurnsItOnOrOff(t *testing.T) {
	f := newLinkFixture(t)

	if l := f.enable(t, cdbOwnerB, cdbDashboard1); l != nil {
		t.Fatalf("owner B turned on A's link: %+v, want nil", l)
	}
	if l := f.get(t, cdbOwnerA, cdbDashboard1); l == nil || l.Enabled {
		t.Fatalf("A's link after B's attempt = %+v, want still off", l)
	}

	on := f.enable(t, cdbOwnerA, cdbDashboard1)
	f.opened(t, cdbDashboard1, cdbOwnerB)
	if f.disable(t, cdbOwnerB, cdbDashboard1) {
		t.Fatal("owner B turned off A's link")
	}
	if l := f.get(t, cdbOwnerA, cdbDashboard1); l == nil || l.LinkID != on.LinkID || l.OpenedBy != 1 {
		t.Errorf("A's link after B's attempt = %+v, want unchanged %+v opened by 1", l, on)
	}
	if f.disable(t, cdbOwnerA, "cdb00000-0000-4000-8000-0000000000ff") {
		t.Error("DisableLink on an unknown dashboard returned true")
	}
}

// Saving the dashboard again (Edit with AI, Rename) must not touch its link.
func TestCustomDashboardLinkIntegration_SavingTheDashboardKeepsItsLink(t *testing.T) {
	f := newLinkFixture(t)

	on := f.enable(t, cdbOwnerA, cdbDashboard1)
	f.opened(t, cdbDashboard1, cdbOwnerB)
	saveCustomDashboard(t, f.dashboards, cdbOwnerA, cdbDashboard1, "Zeta renamed", `{"v":3}`)

	if l := f.get(t, cdbOwnerA, cdbDashboard1); l == nil || l.LinkID != on.LinkID || !l.Enabled || l.OpenedBy != 1 {
		t.Errorf("link after saving = %+v, want unchanged %+v opened by 1", l, on)
	}
}

// userType sets user's type; links, like team shares, are for internal staff only.
func (f linkFixture) userType(t *testing.T, user, typ string) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, `UPDATE "user" SET user_type = $2::user_type_enum WHERE id = $1`, user, typ); err != nil {
		t.Fatalf("set user type: %v", err)
	}
}

// open is OpenLink as user, failing the test on an error: the dashboard id
// ("" for no dashboard) and whether user owns it.
func (f linkFixture) open(t *testing.T, user, linkID string) (string, bool) {
	t.Helper()
	id, own, err := f.links.OpenLink(f.ctx, user, linkID)
	if err != nil {
		t.Fatalf("OpenLink: %v", err)
	}
	return id, own
}

// sharedWith is user's Shared Dashboards list, failing the test on an error.
func (f linkFixture) sharedWith(t *testing.T, user string) []domain.SharedCustomDashboardSummary {
	t.Helper()
	list, err := NewCustomDashboardShareRepository(f.pool).ListSharedWithUser(f.ctx, user)
	if err != nil {
		t.Fatalf("ListSharedWithUser: %v", err)
	}
	return list
}

// withTeam adds the team Orion Test with user in it; removed after the test
// (its team shares and members cascade).
func (f linkFixture) withTeam(t *testing.T, user string) {
	t.Helper()
	clean := func() {
		if _, err := f.pool.Exec(f.ctx, `DELETE FROM team WHERE id = $1`, cdbTeamOrion); err != nil {
			t.Fatalf("clean team: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO team (id, created_on, updated_on, name, type) VALUES ($1, now(), now(), 'Orion Test', 'cre-abt')`, cdbTeamOrion); err != nil {
		t.Fatalf("seed team: %v", err)
	}
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO team_member (created_on, updated_on, team_id, user_id) VALUES (now(), now(), $1, $2)`, cdbTeamOrion, user); err != nil {
		t.Fatalf("join team: %v", err)
	}
}

// TestCustomDashboardLinkIntegration_OpeningTheLinkSharesTheDashboard: opening
// the link lists the dashboard as shared via the link, with its owner; opening
// it again adds nothing.
func TestCustomDashboardLinkIntegration_OpeningTheLinkSharesTheDashboard(t *testing.T) {
	f := newLinkFixture(t)
	f.userType(t, cdbOwnerB, "INTERNAL")
	on := f.enable(t, cdbOwnerA, cdbDashboard1)

	id, own := f.open(t, cdbOwnerB, on.LinkID)
	if id != cdbDashboard1 || own {
		t.Fatalf("OpenLink = %q, own %v; want dashboard 1, not own", id, own)
	}
	list := f.sharedWith(t, cdbOwnerB)
	if len(list) != 1 || list[0].ID != cdbDashboard1 || !list[0].SharedViaLink || list[0].SharedViaTeam != nil {
		t.Fatalf("B's shared list = %+v, want dashboard 1 via link", list)
	}
	if list[0].Owner == nil || list[0].Owner.ID == nil || *list[0].Owner.ID != cdbOwnerA {
		t.Errorf("owner = %+v, want A", list[0].Owner)
	}
	got, err := NewCustomDashboardShareRepository(f.pool).GetSharedWithUser(f.ctx, cdbOwnerB, cdbDashboard1)
	if err != nil || got == nil || got.Definition != `{"v":1}` || !got.SharedViaLink {
		t.Fatalf("GetSharedWithUser = %+v, %v; want dashboard 1 via link", got, err)
	}
	// Opening it again adds nothing.
	f.open(t, cdbOwnerB, on.LinkID)
	if n := f.accessRows(t, cdbDashboard1); n != 1 {
		t.Errorf("access rows after opening twice = %d, want 1", n)
	}
	if l := f.get(t, cdbOwnerA, cdbDashboard1); l == nil || l.OpenedBy != 1 {
		t.Errorf("owner's count = %+v, want opened by 1", l)
	}

	// Turning the link off takes it away; the old link no longer opens.
	f.disable(t, cdbOwnerA, cdbDashboard1)
	if list := f.sharedWith(t, cdbOwnerB); len(list) != 0 {
		t.Errorf("B's shared list after the link is off = %+v, want none", list)
	}
	if id, _ := f.open(t, cdbOwnerB, on.LinkID); id != "" {
		t.Errorf("OpenLink on a link that is off = %q, want none", id)
	}
}

// An open that overlaps the owner turning the link off must not leave an
// access row behind: it would survive the switch-off and give the user the
// dashboard again, without the new link, once the link is turned back on.
// The owner's switch-off is held open here (DisableLink's two statements, not
// yet committed) while the open runs on another connection.
func TestCustomDashboardLinkIntegration_AnOpenDuringTheSwitchOffAddsNothing(t *testing.T) {
	f := newLinkFixture(t)
	f.userType(t, cdbOwnerB, "INTERNAL")
	on := f.enable(t, cdbOwnerA, cdbDashboard1)

	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(f.ctx) //nolint:errcheck // no-op once Commit has succeeded
	if _, err := tx.Exec(f.ctx,
		`UPDATE custom_dashboard SET link_enabled = false, link_id = NULL WHERE id = $1`, cdbDashboard1); err != nil {
		t.Fatalf("switch off: %v", err)
	}
	if _, err := tx.Exec(f.ctx,
		`DELETE FROM custom_dashboard_link_access WHERE dashboard_id = $1`, cdbDashboard1); err != nil {
		t.Fatalf("remove access: %v", err)
	}

	type result struct {
		id  string
		err error
	}
	done := make(chan result, 1)
	go func() {
		id, _, err := f.links.OpenLink(f.ctx, cdbOwnerB, on.LinkID)
		done <- result{id, err}
	}()
	// Let the open either finish or block on the dashboard row before the
	// switch-off commits. Neither within the deadline means the overlap was
	// never set up, so the assertions below would prove nothing.
	deadline := time.Now().Add(5 * time.Second)
	blocked := false
	for time.Now().Before(deadline) && len(done) == 0 {
		if err := f.pool.QueryRow(f.ctx,
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			                WHERE datname = current_database() AND wait_event_type = 'Lock'
			                  AND query LIKE '%custom_dashboard_link_access%')`).Scan(&blocked); err != nil {
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

	r := <-done
	if r.err != nil {
		t.Fatalf("OpenLink: %v", r.err)
	}
	if r.id != "" {
		t.Errorf("OpenLink during the switch-off = %q, want none", r.id)
	}
	if n := f.accessRows(t, cdbDashboard1); n != 0 {
		t.Errorf("access rows after the switch-off = %d, want 0", n)
	}
}

// TestCustomDashboardLinkIntegration_TheOwnerAndUnknownLinksAddNothing: the
// owner opening their own link gets it as their own and adds no row; an
// unknown link, or the dashboard's own id, opens nothing.
func TestCustomDashboardLinkIntegration_TheOwnerAndUnknownLinksAddNothing(t *testing.T) {
	f := newLinkFixture(t)
	f.userType(t, cdbOwnerA, "INTERNAL")
	on := f.enable(t, cdbOwnerA, cdbDashboard1)

	id, own := f.open(t, cdbOwnerA, on.LinkID)
	if id != cdbDashboard1 || !own {
		t.Fatalf("owner's OpenLink = %q, own %v; want dashboard 1, own", id, own)
	}
	if n := f.accessRows(t, cdbDashboard1); n != 0 {
		t.Errorf("access rows after the owner opened it = %d, want 0", n)
	}
	if id, _ := f.open(t, cdbOwnerA, "cdb00000-0000-4000-8000-0000000000ff"); id != "" {
		t.Errorf("OpenLink on an unknown link = %q, want none", id)
	}
	// The dashboard's own id is not its link.
	if id, _ := f.open(t, cdbOwnerA, cdbDashboard1); id != "" {
		t.Errorf("OpenLink with the dashboard id = %q, want none", id)
	}
}

// TestCustomDashboardLinkIntegration_OnlyInternalStaffOpenALink: a user who is
// not internal staff opens nothing and adds no row, and a row left from when
// they were internal no longer lists the dashboard.
func TestCustomDashboardLinkIntegration_OnlyInternalStaffOpenALink(t *testing.T) {
	f := newLinkFixture(t)
	on := f.enable(t, cdbOwnerA, cdbDashboard1)

	for _, typ := range []string{"EXTERNAL", "NOT_AVAILABLE"} {
		f.userType(t, cdbOwnerB, typ)
		if id, _ := f.open(t, cdbOwnerB, on.LinkID); id != "" {
			t.Errorf("%s user's OpenLink = %q, want none", typ, id)
		}
	}
	if n := f.accessRows(t, cdbDashboard1); n != 0 {
		t.Errorf("access rows = %d, want 0", n)
	}
	// A row left from when the user was internal does not count once they are not.
	f.userType(t, cdbOwnerB, "INTERNAL")
	f.open(t, cdbOwnerB, on.LinkID)
	f.userType(t, cdbOwnerB, "EXTERNAL")
	if list := f.sharedWith(t, cdbOwnerB); len(list) != 0 {
		t.Errorf("external B's shared list = %+v, want none", list)
	}
}

// A user already shared the dashboard directly or through a team who opens
// the link gets an access row, but the Shared list still shows the share;
// when that share goes, the dashboard stays through the link, as for a user
// shared it after opening the link.
func TestCustomDashboardLinkIntegration_AlreadySharedStillRecordsTheOpen(t *testing.T) {
	f := newLinkFixture(t)
	f.userType(t, cdbOwnerB, "INTERNAL")
	shares := NewCustomDashboardShareRepository(f.pool)
	on1 := f.enable(t, cdbOwnerA, cdbDashboard1)
	on2 := f.enable(t, cdbOwnerA, cdbDashboard2)

	// Dashboard 1 is shared with B directly, dashboard 2 with B's team.
	if ok, err := shares.AddShare(f.ctx, cdbOwnerA, cdbDashboard1, cdbOwnerB); err != nil || !ok {
		t.Fatalf("AddShare: %v, %v", ok, err)
	}
	f.withTeam(t, cdbOwnerB)
	if ok, err := NewCustomDashboardTeamShareRepository(f.pool).AddTeamShare(f.ctx, cdbOwnerA, cdbDashboard2, cdbTeamOrion); err != nil || !ok {
		t.Fatalf("AddTeamShare: %v, %v", ok, err)
	}
	before := f.sharedWith(t, cdbOwnerB)

	if id, _ := f.open(t, cdbOwnerB, on1.LinkID); id != cdbDashboard1 {
		t.Errorf("OpenLink 1 = %q, want dashboard 1", id)
	}
	if id, _ := f.open(t, cdbOwnerB, on2.LinkID); id != cdbDashboard2 {
		t.Errorf("OpenLink 2 = %q, want dashboard 2", id)
	}
	if n1, n2 := f.accessRows(t, cdbDashboard1), f.accessRows(t, cdbDashboard2); n1 != 1 || n2 != 1 {
		t.Errorf("access rows for dashboards already shared with B = %d, %d, want 1 each", n1, n2)
	}
	after := f.sharedWith(t, cdbOwnerB)
	if len(after) != 2 || len(before) != 2 {
		t.Fatalf("B's shared list = %+v (before %+v), want the two shares", after, before)
	}
	for i := range after {
		if after[i].ID != before[i].ID || after[i].SharedViaLink || after[i].SharedOn != before[i].SharedOn {
			t.Errorf("entry %d after opening the links = %+v, want unchanged %+v", i, after[i], before[i])
		}
	}
	// Alpha (2) is through the team, Zeta (1) direct; ordered by name.
	if after[0].ID != cdbDashboard2 || after[0].SharedViaTeam == nil || after[1].ID != cdbDashboard1 || after[1].SharedViaTeam != nil {
		t.Errorf("B's shared list = %+v, want Alpha via the team, then Zeta direct", after)
	}

	// Both shares go: B still has both dashboards, through the links.
	if ok, err := shares.RemoveShare(f.ctx, cdbOwnerA, cdbDashboard1, cdbOwnerB); err != nil || !ok {
		t.Fatalf("RemoveShare: %v, %v", ok, err)
	}
	if ok, err := NewCustomDashboardTeamShareRepository(f.pool).RemoveTeamShare(f.ctx, cdbOwnerA, cdbDashboard2, cdbTeamOrion); err != nil || !ok {
		t.Fatalf("RemoveTeamShare: %v, %v", ok, err)
	}
	if list := f.sharedWith(t, cdbOwnerB); len(list) != 2 || !list[0].SharedViaLink || !list[1].SharedViaLink || list[0].SharedViaTeam != nil {
		t.Errorf("B's shared list after both shares went = %+v, want both via the link", list)
	}
}

// A user who opened the link and is later shared the dashboard directly sees
// one entry, the direct share; when that share goes, the link one is back.
func TestCustomDashboardLinkIntegration_ADirectShareWinsOverTheLink(t *testing.T) {
	f := newLinkFixture(t)
	f.userType(t, cdbOwnerB, "INTERNAL")
	shares := NewCustomDashboardShareRepository(f.pool)
	on := f.enable(t, cdbOwnerA, cdbDashboard1)
	f.open(t, cdbOwnerB, on.LinkID)

	if ok, err := shares.AddShare(f.ctx, cdbOwnerA, cdbDashboard1, cdbOwnerB); err != nil || !ok {
		t.Fatalf("AddShare: %v, %v", ok, err)
	}
	if list := f.sharedWith(t, cdbOwnerB); len(list) != 1 || list[0].SharedViaLink {
		t.Fatalf("B's shared list with both = %+v, want one entry, the direct share", list)
	}
	if ok, err := shares.RemoveShare(f.ctx, cdbOwnerA, cdbDashboard1, cdbOwnerB); err != nil || !ok {
		t.Fatalf("RemoveShare: %v, %v", ok, err)
	}
	if list := f.sharedWith(t, cdbOwnerB); len(list) != 1 || !list[0].SharedViaLink {
		t.Errorf("B's shared list after the direct share went = %+v, want it via the link", list)
	}
}

const (
	cdbOwnerF            = "cdb00000-0000-4000-8000-00000000000f"
	cdbDashboardLinkedF  = "cdb00000-0000-4000-8000-0000000000f1"
	cdbDashboardOpenedF  = "cdb00000-0000-4000-8000-0000000000f2"
	cdbDashboardLinkOffF = "cdb00000-0000-4000-8000-0000000000f3"
	cdbDashboardSharedF  = "cdb00000-0000-4000-8000-0000000000f4"
)

// Deleting the owner turns every link of theirs off (migration 0270): the
// people who had a dashboard only through its link lose it and the link opens
// nothing, so a dashboard shared with nobody else goes with the owner, opened
// by somebody or not, like one whose link was already off (migration 0255).
// One shared with B directly stays, with no owner, its link off and no access
// rows, and B keeps it through the share.
func TestCustomDashboardLinkIntegration_DeletingTheOwnerTurnsTheLinkOff(t *testing.T) {
	f := newLinkFixture(t)
	ids := []string{cdbDashboardLinkedF, cdbDashboardOpenedF, cdbDashboardLinkOffF, cdbDashboardSharedF}
	clean := func() {
		// A kept dashboard outlives F with no owner, so they go by id.
		if _, err := f.pool.Exec(f.ctx, `DELETE FROM custom_dashboard WHERE id = ANY($1::uuid[])`, ids); err != nil {
			t.Fatalf("clean dashboards: %v", err)
		}
		if _, err := f.pool.Exec(f.ctx, `DELETE FROM "user" WHERE id = $1`, cdbOwnerF); err != nil {
			t.Fatalf("clean user F: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name, email, first_name, last_name)
		VALUES ($1, now(), now(), 'cdb-test-f', 'fay.f@example.test', 'Fay', 'Eff')`, cdbOwnerF); err != nil {
		t.Fatalf("seed user F: %v", err)
	}
	f.userType(t, cdbOwnerB, "INTERNAL")
	saveCustomDashboard(t, f.dashboards, cdbOwnerF, cdbDashboardLinkedF, "Linked", `{"v":1}`)
	saveCustomDashboard(t, f.dashboards, cdbOwnerF, cdbDashboardOpenedF, "Opened", `{"v":2}`)
	saveCustomDashboard(t, f.dashboards, cdbOwnerF, cdbDashboardLinkOffF, "Link off", `{"v":3}`)
	saveCustomDashboard(t, f.dashboards, cdbOwnerF, cdbDashboardSharedF, "Shared", `{"v":4}`)
	linked := f.enable(t, cdbOwnerF, cdbDashboardLinkedF)
	opened := f.enable(t, cdbOwnerF, cdbDashboardOpenedF)
	f.open(t, cdbOwnerB, opened.LinkID)
	f.enable(t, cdbOwnerF, cdbDashboardLinkOffF)
	f.disable(t, cdbOwnerF, cdbDashboardLinkOffF)
	shared := f.enable(t, cdbOwnerF, cdbDashboardSharedF)
	if ok, err := NewCustomDashboardShareRepository(f.pool).AddShare(f.ctx, cdbOwnerF, cdbDashboardSharedF, cdbOwnerB); err != nil || !ok {
		t.Fatalf("AddShare: %v, %v", ok, err)
	}
	f.open(t, cdbOwnerB, shared.LinkID)

	if _, err := f.pool.Exec(f.ctx, `DELETE FROM "user" WHERE id = $1`, cdbOwnerF); err != nil {
		t.Fatalf("delete user F: %v", err)
	}

	// Only the one shared with B directly is left, with no owner and no link.
	var left []string
	if err := f.pool.QueryRow(f.ctx,
		`SELECT COALESCE(array_agg(name ORDER BY name), '{}') FROM custom_dashboard
		 WHERE id = ANY($1::uuid[]) AND owner_user_id IS NULL AND NOT link_enabled AND link_id IS NULL`, ids).Scan(&left); err != nil {
		t.Fatalf("read dashboards: %v", err)
	}
	var all int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM custom_dashboard WHERE id = ANY($1::uuid[])`, ids).Scan(&all); err != nil {
		t.Fatalf("count dashboards: %v", err)
	}
	if len(left) != 1 || left[0] != "Shared" || all != 1 {
		t.Fatalf("dashboards left = %v (%d in all), want only Shared, with no owner and its link off", left, all)
	}
	if n := f.accessRows(t, cdbDashboardSharedF); n != 0 {
		t.Errorf("access rows on the kept dashboard = %d, want 0", n)
	}

	// B keeps it through the share, and the old links open nothing.
	list := f.sharedWith(t, cdbOwnerB)
	if len(list) != 1 || list[0].ID != cdbDashboardSharedF || list[0].SharedViaLink || list[0].Owner != nil {
		t.Fatalf("B's shared list = %+v, want Shared through the direct share, with no owner", list)
	}
	for name, l := range map[string]string{"Linked": linked.LinkID, "Opened": opened.LinkID, "Shared": shared.LinkID} {
		if id, _ := f.open(t, cdbOwnerB, l); id != "" {
			t.Errorf("%s's old link opened %q after the owner went, want nothing", name, id)
		}
	}
}
