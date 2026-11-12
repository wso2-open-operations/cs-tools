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
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These run SystemDashboardRepository against a live PostgreSQL instance, with
// every migration applied up to 100029, and check that the user-facing
// dashboard code and the orphan cleanup of migrations 0255, 0276 and 0277 never
// see or remove a system row. They reuse the orphan fixture (owner G,
// recipient H, teams Orion and Lyra). Skipped unless ENTITY_TEST_DATABASE_URL
// is set.

const (
	sdbKeyA = "sdb-test-overview.json"
	sdbKeyB = "sdb-test-cases.json"
)

type systemDashboardFixture struct {
	orphanFixture
	system SystemDashboardRepository
}

// newSystemDashboardFixture removes the fixture's system rows before and after
// the test.
func newSystemDashboardFixture(t *testing.T) systemDashboardFixture {
	t.Helper()
	f := systemDashboardFixture{orphanFixture: newOrphanFixture(t)}
	f.system = NewSystemDashboardRepository(f.pool)
	clean := func() {
		if _, err := f.pool.Exec(f.ctx, `DELETE FROM custom_dashboard WHERE system_key LIKE 'sdb-test-%'`); err != nil {
			t.Fatalf("clean system dashboards: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	return f
}

func (f systemDashboardFixture) save(t *testing.T, key, name, definition string) string {
	t.Helper()
	d, err := f.system.Upsert(f.ctx, key, domainSaveSystem(name, definition))
	if err != nil {
		t.Fatalf("Upsert %s: %v", key, err)
	}
	return d.ID
}

// sqlState is the SQLSTATE of a Postgres error, "" for any other error.
func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func TestSystemDashboardIntegration_UpsertInsertsThenOverwritesByKey(t *testing.T) {
	f := newSystemDashboardFixture(t)

	first, err := f.system.Upsert(f.ctx, sdbKeyA, domainSaveSystem("Overview", `{"v":1}`))
	if err != nil {
		t.Fatalf("Upsert insert: %v", err)
	}
	if first.ID == "" || first.SystemKey != sdbKeyA || !first.IsSystem || first.Name != "Overview" || first.Definition != `{"v":1}` {
		t.Fatalf("inserted = %+v", first)
	}
	var owner *string
	if err := f.pool.QueryRow(f.ctx, `SELECT owner_user_id::text FROM custom_dashboard WHERE id = $1`, first.ID).Scan(&owner); err != nil || owner != nil {
		t.Fatalf("owner = %v, %v; want NULL", owner, err)
	}

	time.Sleep(20 * time.Millisecond)
	second, err := f.system.Upsert(f.ctx, sdbKeyA, domainSaveSystem("Overview renamed", `{"v":2}`))
	if err != nil {
		t.Fatalf("Upsert overwrite: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("id = %s, want the first id %s kept", second.ID, first.ID)
	}
	if second.Name != "Overview renamed" || second.Definition != `{"v":2}` {
		t.Errorf("overwritten = %+v, want the new name and definition", second)
	}
	if !second.CreatedOn.Equal(first.CreatedOn) {
		t.Errorf("created_on changed: %v -> %v", first.CreatedOn, second.CreatedOn)
	}
	if !second.UpdatedOn.After(first.UpdatedOn) {
		t.Errorf("updated_on did not advance: %v -> %v", first.UpdatedOn, second.UpdatedOn)
	}
	var n int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM custom_dashboard WHERE system_key = $1`, sdbKeyA).Scan(&n); err != nil || n != 1 {
		t.Errorf("rows for the key = %d, %v; want 1", n, err)
	}
}

func TestSystemDashboardIntegration_ListAndGet(t *testing.T) {
	f := newSystemDashboardFixture(t)
	idB := f.save(t, sdbKeyB, "Cases", `{"b":1}`)
	idA := f.save(t, sdbKeyA, "Overview", `{"a":1}`)
	// A user's dashboard is not a system dashboard.
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanX, "Mine", `{"m":1}`)

	list, err := f.system.List(f.ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var keys, ids []string
	for _, d := range list {
		if strings.HasPrefix(d.SystemKey, "sdb-test-") {
			keys = append(keys, d.SystemKey)
			ids = append(ids, d.ID)
		}
		if !d.IsSystem || d.ID == cdbOrphanX {
			t.Errorf("List returned %+v, which is not a system dashboard", d)
		}
	}
	if len(keys) != 2 || keys[0] != sdbKeyB || keys[1] != sdbKeyA || ids[0] != idB || ids[1] != idA {
		t.Errorf("List keys = %v ids = %v, want [%s %s] ordered by key", keys, ids, sdbKeyB, sdbKeyA)
	}

	got, err := f.system.Get(f.ctx, sdbKeyA)
	if err != nil || got == nil || got.ID != idA || got.Definition != `{"a":1}` {
		t.Errorf("Get = %+v, %v", got, err)
	}
	if got, err := f.system.Get(f.ctx, "sdb-test-missing"); err != nil || got != nil {
		t.Errorf("Get missing = %+v, %v; want nil", got, err)
	}
}

// A user's own dashboard is never read or overwritten through the system
// repository, even when it has the same name.
func TestSystemDashboardIntegration_UserDashboardsAreOutOfReach(t *testing.T) {
	f := newSystemDashboardFixture(t)
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanX, "Overview", `{"user":true}`)
	f.save(t, sdbKeyA, "Overview", `{"system":true}`)

	if got, err := f.system.Get(f.ctx, cdbOrphanX); err != nil || got != nil {
		t.Errorf("Get by a user dashboard's id as key = %+v, %v; want nil", got, err)
	}
	own, err := f.dashboards.Get(f.ctx, cdbOwnerG, cdbOrphanX)
	if err != nil || own == nil || own.Definition != `{"user":true}` {
		t.Errorf("the user's dashboard = %+v, %v; want it untouched", own, err)
	}
}

func TestSystemDashboardIntegration_NeverInAUsersLists(t *testing.T) {
	f := newSystemDashboardFixture(t)
	sysID := f.save(t, sdbKeyA, "Overview", `{"a":1}`)
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanX, "Mine", `{"m":1}`)

	list, err := f.dashboards.List(f.ctx, cdbOwnerG)
	if err != nil || len(list) != 1 || list[0].ID != cdbOrphanX {
		t.Fatalf("owner List = %+v, %v; want only their own dashboard", list, err)
	}
	if got, err := f.dashboards.Get(f.ctx, cdbOwnerG, sysID); err != nil || got != nil {
		t.Errorf("owner Get of a system id = %+v, %v; want nil", got, err)
	}
	// A save through the user path with a system row's id must not take it over.
	if got, err := f.dashboards.Upsert(f.ctx, cdbOwnerG, sysID, domainSaveCustom("Hijack", `{"h":1}`)); err != nil || got != nil {
		t.Errorf("owner Upsert of a system id = %+v, %v; want nil", got, err)
	}
	if ok, err := f.dashboards.Delete(f.ctx, cdbOwnerG, sysID); err != nil || ok {
		t.Errorf("owner Delete of a system id = %v, %v; want false", ok, err)
	}
	shared, err := NewCustomDashboardShareRepository(f.pool).ListSharedWithUser(f.ctx, cdbRecipientH)
	if err != nil {
		t.Fatalf("ListSharedWithUser: %v", err)
	}
	for _, d := range shared {
		if d.ID == sysID {
			t.Errorf("a system dashboard is in a user's shared list: %+v", d)
		}
	}
	got, err := f.system.Get(f.ctx, sdbKeyA)
	if err != nil || got == nil || got.Name != "Overview" || got.Definition != `{"a":1}` {
		t.Errorf("the system row = %+v, %v; want it unchanged", got, err)
	}
}

func TestSystemDashboardIntegration_CheckConstraints(t *testing.T) {
	f := newSystemDashboardFixture(t)
	big := strings.Repeat("x", 70000)
	cases := []struct {
		name string
		sql  string
		args []any
		want string // SQLSTATE; "" = accepted
	}{
		{"system row with an owner",
			`INSERT INTO custom_dashboard (owner_user_id, is_system, system_key, name, definition) VALUES ($1, true, 'sdb-test-x', 'n', '{}')`,
			[]any{cdbOwnerG}, "23514"},
		{"system row without a key",
			`INSERT INTO custom_dashboard (owner_user_id, is_system, system_key, name, definition) VALUES (NULL, true, NULL, 'n', '{}')`,
			nil, "23514"},
		{"system row with a blank key",
			`INSERT INTO custom_dashboard (owner_user_id, is_system, system_key, name, definition) VALUES (NULL, true, '  ', 'n', '{}')`,
			nil, "23514"},
		{"non-system row with a key",
			`INSERT INTO custom_dashboard (owner_user_id, is_system, system_key, name, definition) VALUES ($1, false, 'sdb-test-x', 'n', '{}')`,
			[]any{cdbOwnerG}, "23514"},
		{"non-system row with a key and no owner",
			`INSERT INTO custom_dashboard (owner_user_id, is_system, system_key, name, definition) VALUES (NULL, false, 'sdb-test-x', 'n', '{}')`,
			nil, "23514"},
		{"system row with its link on",
			`INSERT INTO custom_dashboard (owner_user_id, is_system, system_key, name, definition, link_enabled, link_id) VALUES (NULL, true, 'sdb-test-x', 'n', '{}', true, gen_random_uuid())`,
			nil, "23514"},
		{"system definition over 64 KB",
			`INSERT INTO custom_dashboard (owner_user_id, is_system, system_key, name, definition) VALUES (NULL, true, 'sdb-test-big', 'n', $1)`,
			[]any{big}, ""},
		{"user definition over 64 KB",
			`INSERT INTO custom_dashboard (owner_user_id, name, definition) VALUES ($1, 'n', $2)`,
			[]any{cdbOwnerG, big}, "23514"},
		{"system definition over 1 MiB",
			`INSERT INTO custom_dashboard (owner_user_id, is_system, system_key, name, definition) VALUES (NULL, true, 'sdb-test-huge', 'n', $1)`,
			[]any{strings.Repeat("x", 1048577)}, "23514"},
		{"ordinary user row",
			`INSERT INTO custom_dashboard (owner_user_id, name, definition) VALUES ($1, 'sdb-test-ordinary', '{}')`,
			[]any{cdbOwnerG}, ""},
	}
	for _, c := range cases {
		_, err := f.pool.Exec(f.ctx, c.sql, c.args...)
		if got := sqlState(err); got != c.want {
			t.Errorf("%s: SQLSTATE %q (%v), want %q", c.name, got, err, c.want)
		}
	}
	f.exec(t, "clean", `DELETE FROM custom_dashboard WHERE system_key LIKE 'sdb-test-%' OR name = 'sdb-test-ordinary' OR owner_user_id = $1`, cdbOwnerG)

	// The key is unique.
	f.save(t, sdbKeyA, "One", `{}`)
	_, err := f.pool.Exec(f.ctx,
		`INSERT INTO custom_dashboard (owner_user_id, is_system, system_key, name, definition) VALUES (NULL, true, $1, 'Two', '{}')`, sdbKeyA)
	if sqlState(err) != "23505" {
		t.Errorf("duplicate key: SQLSTATE %q (%v), want 23505", sqlState(err), err)
	}
	// Two system rows with the same name under different keys are fine: NULL
	// owners never collide in the (owner, lower(name)) index.
	f.save(t, sdbKeyB, "One", `{}`)
}

// Deleting users, shares and teams runs every owner-deletion and orphan rule
// of 0255, 0276 and 0277; none of them may remove or change a system row.
func TestSystemDashboardIntegration_OrphanRulesLeaveSystemRowsAlone(t *testing.T) {
	f := newSystemDashboardFixture(t)
	sysID := f.save(t, sdbKeyA, "Overview", `{"a":1}`)

	// Another user's dashboard going through the whole owner-deletion path.
	saveCustomDashboard(t, f.dashboards, cdbOwnerG, cdbOrphanX, "Unshared", `{"u":1}`)
	f.deleteUser(t, cdbOwnerG)
	if f.exists(t, cdbOrphanX) {
		t.Error("the owner's unshared dashboard survived its owner (0255 broke)")
	}
	if !f.exists(t, sysID) {
		t.Fatal("a user deletion removed the system row")
	}

	// A share row put on a system row by hand, then removed: 0277's trigger
	// would delete any ownerless dashboard left with no share.
	f.shareWith(t, sysID, cdbRecipientH)
	f.shareWithTeam(t, sysID, cdbTeamOrion)
	f.exec(t, "unshare person", `DELETE FROM custom_dashboard_share WHERE dashboard_id = $1`, sysID)
	f.exec(t, "unshare team", `DELETE FROM custom_dashboard_team_share WHERE dashboard_id = $1`, sysID)
	if !f.exists(t, sysID) {
		t.Fatal("deleting the last share removed the system row")
	}
	// The same through the recipient and the team being deleted.
	f.shareWith(t, sysID, cdbRecipientH)
	f.shareWithTeam(t, sysID, cdbTeamLyra)
	f.deleteUser(t, cdbRecipientH)
	f.exec(t, "delete team", `DELETE FROM team WHERE id = $1`, cdbTeamLyra)
	if !f.exists(t, sysID) {
		t.Fatal("deleting the recipient and the team removed the system row")
	}
	// The cleanup function itself.
	f.exec(t, "orphan check", `SELECT custom_dashboard_delete_if_orphaned($1)`, sysID)
	if !f.exists(t, sysID) {
		t.Fatal("custom_dashboard_delete_if_orphaned removed the system row")
	}

	got, err := f.system.Get(f.ctx, sdbKeyA)
	if err != nil || got == nil || got.Definition != `{"a":1}` || got.Name != "Overview" {
		t.Errorf("the system row = %+v, %v; want it unchanged", got, err)
	}
}

// The guard in 0276's function. Its real trigger cannot fire for a system row
// (it needs an owner going from set to NULL), so a temporary trigger on a name
// change runs the same function, in a transaction that is rolled back: without
// the guard it would delete the unshared ownerless row.
func TestSystemDashboardIntegration_OwnerDeletionFunctionSkipsSystemRows(t *testing.T) {
	f := newSystemDashboardFixture(t)
	sysID := f.save(t, sdbKeyA, "Overview", `{"a":1}`)
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background()) //nolint:errcheck // rolled back by design
	for _, stmt := range []string{
		`CREATE TRIGGER trg_sdb_test_probe AFTER UPDATE OF name ON custom_dashboard
		 FOR EACH ROW EXECUTE FUNCTION custom_dashboard_delete_unshared_orphan()`,
		`UPDATE custom_dashboard SET name = 'Overview 2' WHERE id = '` + sysID + `'`,
	} {
		if _, err := tx.Exec(f.ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	var ok bool
	if err := tx.QueryRow(f.ctx, `SELECT EXISTS (SELECT 1 FROM custom_dashboard WHERE id = $1)`, sysID).Scan(&ok); err != nil || !ok {
		t.Fatalf("system row exists = %v, %v; want true", ok, err)
	}
	// The same function does delete an unshared ownerless user dashboard, so
	// the probe really runs it.
	if _, err := tx.Exec(f.ctx,
		`INSERT INTO custom_dashboard (id, owner_user_id, name, definition) VALUES ($1, NULL, 'sdb-test-orphan', '{}')`, cdbOrphanY); err != nil {
		t.Fatalf("insert orphan: %v", err)
	}
	if _, err := tx.Exec(f.ctx, `UPDATE custom_dashboard SET name = 'sdb-test-orphan 2' WHERE id = $1`, cdbOrphanY); err != nil {
		t.Fatalf("update orphan: %v", err)
	}
	if err := tx.QueryRow(f.ctx, `SELECT EXISTS (SELECT 1 FROM custom_dashboard WHERE id = $1)`, cdbOrphanY).Scan(&ok); err != nil || ok {
		t.Fatalf("probe trigger did not run the function on the ownerless user row: exists = %v, %v", ok, err)
	}
}

func domainSaveSystem(name, definition string) domain.SaveSystemDashboardRequest {
	return domain.SaveSystemDashboardRequest{Name: name, Definition: definition}
}

func domainSaveCustom(name, definition string) domain.SaveCustomDashboardRequest {
	return domain.SaveCustomDashboardRequest{Name: name, Definition: definition}
}
