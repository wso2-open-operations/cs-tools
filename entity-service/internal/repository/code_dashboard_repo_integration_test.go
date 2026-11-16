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
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These check migration 0295 (code_dashboard) and CodeDashboardRepository
// against a live PostgreSQL instance. Skipped unless ENTITY_TEST_DATABASE_URL is set (see
// account_repo_salesforce_integration_test.go for the setup).

// Fixture users and dashboard ids, distinct from every other integration file's.
const (
	codeOwnerA     = "c0de0000-0000-4000-8000-00000000000a"
	codeOwnerB     = "c0de0000-0000-4000-8000-00000000000b"
	codeDashboard1 = "c0de0000-0000-4000-8000-000000000001"
	codeDashboard2 = "c0de0000-0000-4000-8000-000000000002"
	codeDashboard3 = "c0de0000-0000-4000-8000-000000000003"
)

type codeDashboardFixture struct {
	ctx  context.Context
	pool *pgxpool.Pool
	repo CodeDashboardRepository
}

// newCodeDashboardFixture connects to ENTITY_TEST_DATABASE_URL (the test is
// skipped when it is unset) and seeds owners A and B, removing them and the
// fixture dashboards before and after the test.
func newCodeDashboardFixture(t *testing.T) codeDashboardFixture {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	f := codeDashboardFixture{ctx: context.Background()}
	pool, err := pgxpool.New(f.ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	f.pool = pool
	f.repo = NewCodeDashboardRepository(pool)
	clean := func() {
		f.exec(t, "clean dashboards", `DELETE FROM code_dashboard WHERE owner_user_id = ANY($1::uuid[]) OR id = ANY($2::uuid[]) OR id::text LIKE 'c0de0000-0000-4000-8000-1%'`,
			[]string{codeOwnerA, codeOwnerB}, []string{codeDashboard1, codeDashboard2, codeDashboard3})
		f.exec(t, "clean users", `DELETE FROM "user" WHERE id = ANY($1::uuid[])`, []string{codeOwnerA, codeOwnerB})
	}
	clean()
	t.Cleanup(clean)
	for _, id := range []string{codeOwnerA, codeOwnerB} {
		f.exec(t, "seed user", `INSERT INTO "user" (id, created_on, updated_on, user_name) VALUES ($1, now(), now(), $2)`,
			id, "code-test-"+id)
	}
	return f
}

// exec runs one statement, failing the test with what on an error.
func (f codeDashboardFixture) exec(t *testing.T, what, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

// insert adds a dashboard row directly.
func (f codeDashboardFixture) insert(t *testing.T, owner, id, name string) {
	t.Helper()
	f.exec(t, "insert "+name, `INSERT INTO code_dashboard (id, owner_user_id, name, source) VALUES ($1, $2, $3, 'function render() {}')`,
		id, owner, name)
}

// exists reports whether the dashboard row is still there.
func (f codeDashboardFixture) exists(t *testing.T, id string) bool {
	t.Helper()
	var ok bool
	if err := f.pool.QueryRow(f.ctx, `SELECT EXISTS (SELECT 1 FROM code_dashboard WHERE id = $1)`, id).Scan(&ok); err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	return ok
}

func TestCodeDashboardIntegration_CheckConstraints(t *testing.T) {
	f := newCodeDashboardFixture(t)
	insert := `INSERT INTO code_dashboard (owner_user_id, name, source) VALUES ($1, $2, $3)`
	cases := []struct {
		name string
		args []any
		want string // SQLSTATE; "" = accepted
	}{
		{"blank name", []any{codeOwnerA, "  ", "function render() {}"}, "23514"},
		{"no name", []any{codeOwnerA, nil, "function render() {}"}, "23502"},
		{"blank source", []any{codeOwnerA, "Blank", " \n "}, "23514"},
		{"no source", []any{codeOwnerA, "Missing", nil}, "23502"},
		{"source over 256 KB", []any{codeOwnerA, "Big", strings.Repeat("x", 262145)}, "23514"},
		{"source of 256 KB", []any{codeOwnerA, "At the cap", strings.Repeat("x", 262144)}, ""},
		{"60000 four-byte characters", []any{codeOwnerA, "Wide", strings.Repeat("😀", 60000)}, ""},
		{"name over 255 characters", []any{codeOwnerA, strings.Repeat("n", 256), "function render() {}"}, "22001"},
		{"unknown owner", []any{"c0de0000-0000-4000-8000-0000000000ff", "Nobody", "function render() {}"}, "23503"},
	}
	for _, c := range cases {
		_, err := f.pool.Exec(f.ctx, insert, c.args...)
		if got := sqlstate(err); got != c.want {
			t.Errorf("%s: SQLSTATE %q (%v), want %q", c.name, got, err, c.want)
		}
	}
}

// A name is unique per owner, ignoring case; another owner may use it.
func TestCodeDashboardIntegration_OneNamePerOwner(t *testing.T) {
	f := newCodeDashboardFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "Open Cases")
	_, err := f.pool.Exec(f.ctx, `INSERT INTO code_dashboard (id, owner_user_id, name, source) VALUES ($1, $2, 'OPEN cases', 'x')`,
		codeDashboard2, codeOwnerA)
	if sqlstate(err) != "23505" {
		t.Errorf("same name, same owner: SQLSTATE %q (%v), want 23505", sqlstate(err), err)
	}
	f.insert(t, codeOwnerB, codeDashboard2, "Open Cases")
}

// Deleting the owner deletes their dashboards shared with nobody, and leaves
// everyone else's (shared ones: code_dashboard_share_integration_test.go).
func TestCodeDashboardIntegration_GoesWithItsOwner(t *testing.T) {
	f := newCodeDashboardFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "One")
	f.insert(t, codeOwnerA, codeDashboard2, "Two")
	f.insert(t, codeOwnerB, codeDashboard3, "One")

	f.exec(t, "delete owner A", `DELETE FROM "user" WHERE id = $1`, codeOwnerA)
	if f.exists(t, codeDashboard1) || f.exists(t, codeDashboard2) {
		t.Error("a dashboard shared with nobody stayed after its owner was deleted")
	}
	if !f.exists(t, codeDashboard3) {
		t.Error("owner B's dashboard went with owner A")
	}
}

// The owner column is nullable and set to NULL, not cascaded: an ownerless
// row can exist (sharing will keep some), and ownerless rows never collide on
// name.
func TestCodeDashboardIntegration_OwnerIsSetNull(t *testing.T) {
	f := newCodeDashboardFixture(t)
	var rule, nullable string
	if err := f.pool.QueryRow(f.ctx, `
		SELECT c.confdeltype::text, col.is_nullable
		  FROM pg_constraint c, information_schema.columns col
		 WHERE c.conrelid = 'code_dashboard'::regclass AND c.contype = 'f'
		   AND col.table_name = 'code_dashboard' AND col.column_name = 'owner_user_id'`).Scan(&rule, &nullable); err != nil {
		t.Fatalf("read the owner key: %v", err)
	}
	if rule != "n" || nullable != "YES" {
		t.Errorf("owner_user_id: on delete %q, nullable %q; want SET NULL (n) and YES", rule, nullable)
	}
	f.exec(t, "ownerless 1", `INSERT INTO code_dashboard (id, owner_user_id, name, source) VALUES ($1, NULL, 'Kept', 'x')`, codeDashboard1)
	f.exec(t, "ownerless 2", `INSERT INTO code_dashboard (id, owner_user_id, name, source) VALUES ($1, NULL, 'kept', 'x')`, codeDashboard2)
}

// The migration can run again on a database that already has it. 0296 runs
// after it, as on a full re-run: 0295 alone would put back its own trigger
// function, which 0296 replaces.
func TestCodeDashboardIntegration_MigrationReruns(t *testing.T) {
	f := newCodeDashboardFixture(t)
	f.insert(t, codeOwnerA, codeDashboard1, "Stays")
	for _, name := range []string{"0295_code_dashboard.sql", "0296_code_dashboard_share.sql"} {
		sql, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			t.Fatalf("read migration: %v", err)
		}
		f.exec(t, "re-run "+name, string(sql))
	}
	if !f.exists(t, codeDashboard1) {
		t.Error("re-running the migration removed a saved dashboard")
	}
	f.exec(t, "delete owner A", `DELETE FROM "user" WHERE id = $1`, codeOwnerA)
	if f.exists(t, codeDashboard1) {
		t.Error("after a re-run, the dashboard stayed when its owner was deleted")
	}
}

// save upserts through the repository and fails the test unless it saved.
func (f codeDashboardFixture) save(t *testing.T, owner, id, name, source string) *domain.CodeDashboard {
	t.Helper()
	d, err := f.repo.Upsert(f.ctx, owner, id, domain.SaveCodeDashboardRequest{Name: name, Source: source})
	if err != nil {
		t.Fatalf("Upsert %s: %v", name, err)
	}
	if d == nil {
		t.Fatalf("Upsert %s: got nil, want the saved dashboard", name)
	}
	return d
}

func TestCodeDashboardRepoIntegration_UpsertCreatesThenReplaces(t *testing.T) {
	f := newCodeDashboardFixture(t)

	created := f.save(t, codeOwnerA, codeDashboard1, "Open Cases", "function render() { return 1; }")
	if created.ID != codeDashboard1 || created.Name != "Open Cases" || created.Source != "function render() { return 1; }" {
		t.Fatalf("created = %+v", created)
	}
	replaced := f.save(t, codeOwnerA, codeDashboard1, "Open Cases by State", "function render() { return 2; }")
	if replaced.Name != "Open Cases by State" || replaced.Source != "function render() { return 2; }" {
		t.Fatalf("replaced = %+v", replaced)
	}
	if !replaced.CreatedOn.Equal(created.CreatedOn) {
		t.Errorf("createdOn changed on replace: %v -> %v", created.CreatedOn, replaced.CreatedOn)
	}
	if replaced.UpdatedOn.Before(created.UpdatedOn) {
		t.Errorf("updatedOn went back: %v -> %v", created.UpdatedOn, replaced.UpdatedOn)
	}
	got, err := f.repo.Get(f.ctx, codeOwnerA, codeDashboard1)
	if err != nil || got == nil || got.Source != "function render() { return 2; }" {
		t.Fatalf("Get = %+v, %v; want the replaced source", got, err)
	}
}

func TestCodeDashboardRepoIntegration_ListIsOwnersOnlyByName(t *testing.T) {
	f := newCodeDashboardFixture(t)
	f.save(t, codeOwnerA, codeDashboard1, "zeta", "x")
	f.save(t, codeOwnerA, codeDashboard2, "Alpha", "x")
	f.save(t, codeOwnerB, codeDashboard3, "Beta", "x")

	list, err := f.repo.List(f.ctx, codeOwnerA)
	if err != nil {
		t.Fatalf("List A: %v", err)
	}
	if len(list) != 2 || list[0].ID != codeDashboard2 || list[0].Name != "Alpha" || list[1].Name != "zeta" {
		t.Fatalf("List A = %+v, want Alpha then zeta", list)
	}
	f.exec(t, "delete owner B", `DELETE FROM "user" WHERE id = $1`, codeOwnerB)
	none, err := f.repo.List(f.ctx, codeOwnerB)
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("List B = %+v, %v; want an empty list", none, err)
	}
}

// Another owner can neither read, replace nor delete a dashboard; nor can
// anyone claim an ownerless one by saving under its id.
func TestCodeDashboardRepoIntegration_OtherOwnerSeesNothing(t *testing.T) {
	f := newCodeDashboardFixture(t)
	f.save(t, codeOwnerA, codeDashboard1, "Mine", "a")

	if got, err := f.repo.Get(f.ctx, codeOwnerB, codeDashboard1); err != nil || got != nil {
		t.Fatalf("Get by B = %+v, %v; want nil, nil", got, err)
	}
	if d, err := f.repo.Upsert(f.ctx, codeOwnerB, codeDashboard1, domain.SaveCodeDashboardRequest{Name: "Taken", Source: "b"}); err != nil || d != nil {
		t.Fatalf("Upsert by B = %+v, %v; want nil, nil", d, err)
	}
	if ok, err := f.repo.Delete(f.ctx, codeOwnerB, codeDashboard1); err != nil || ok {
		t.Fatalf("Delete by B = %v, %v; want false, nil", ok, err)
	}
	if still, err := f.repo.Get(f.ctx, codeOwnerA, codeDashboard1); err != nil || still == nil || still.Name != "Mine" || still.Source != "a" {
		t.Fatalf("A's dashboard after B's attempts = %+v, %v; want it unchanged", still, err)
	}

	f.exec(t, "ownerless", `INSERT INTO code_dashboard (id, owner_user_id, name, source) VALUES ($1, NULL, 'Ownerless', 'o')`, codeDashboard2)
	if d, err := f.repo.Upsert(f.ctx, codeOwnerA, codeDashboard2, domain.SaveCodeDashboardRequest{Name: "Claimed", Source: "a"}); err != nil || d != nil {
		t.Fatalf("Upsert over an ownerless dashboard = %+v, %v; want nil, nil", d, err)
	}
	var owner *string
	var name string
	if err := f.pool.QueryRow(f.ctx, `SELECT owner_user_id::text, name FROM code_dashboard WHERE id = $1`, codeDashboard2).Scan(&owner, &name); err != nil {
		t.Fatal(err)
	}
	if owner != nil || name != "Ownerless" {
		t.Errorf("the ownerless dashboard became owner %v, name %q", owner, name)
	}
}

func TestCodeDashboardRepoIntegration_DuplicateNameIsConflict(t *testing.T) {
	f := newCodeDashboardFixture(t)
	f.save(t, codeOwnerA, codeDashboard1, "Open Cases", "x")

	_, err := f.repo.Upsert(f.ctx, codeOwnerA, codeDashboard2, domain.SaveCodeDashboardRequest{Name: "OPEN CASES", Source: "x"})
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Upsert with a taken name: err = %v, want a ConflictError", err)
	}
	f.save(t, codeOwnerB, codeDashboard2, "Open Cases", "x")
}

func TestCodeDashboardRepoIntegration_DeleteAndGetMissing(t *testing.T) {
	f := newCodeDashboardFixture(t)
	f.save(t, codeOwnerA, codeDashboard1, "Temp", "x")

	if ok, err := f.repo.Delete(f.ctx, codeOwnerA, codeDashboard1); err != nil || !ok {
		t.Fatalf("Delete = %v, %v; want true, nil", ok, err)
	}
	if ok, err := f.repo.Delete(f.ctx, codeOwnerA, codeDashboard1); err != nil || ok {
		t.Fatalf("second Delete = %v, %v; want false, nil", ok, err)
	}
	if got, err := f.repo.Get(f.ctx, codeOwnerA, codeDashboard1); err != nil || got != nil {
		t.Fatalf("Get after delete = %+v, %v; want nil, nil", got, err)
	}
}

// codeFillID is a dashboard id for filling an owner up to the limit.
func codeFillID(i int) string { return fmt.Sprintf("c0de0000-0000-4000-8000-1%011d", i) }

// Saves racing at the limit: with the owner at one below the cap, many new
// dashboards saved at the same moment may add exactly one.
func TestCodeDashboardRepoIntegration_LimitHoldsUnderConcurrentSaves(t *testing.T) {
	f := newCodeDashboardFixture(t)
	for i := 0; i < domain.MaxCodeDashboards-1; i++ {
		f.save(t, codeOwnerA, codeFillID(i), fmt.Sprintf("Fill %d", i), "x")
	}

	const racers = 10
	var wg sync.WaitGroup
	saved := make([]bool, racers)
	errs := make([]error, racers)
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			d, err := f.repo.Upsert(f.ctx, codeOwnerA, codeFillID(1000+i),
				domain.SaveCodeDashboardRequest{Name: fmt.Sprintf("Racer %d", i), Source: "x"})
			saved[i], errs[i] = d != nil, err
		}(i)
	}
	close(start)
	wg.Wait()

	ok, refused := 0, 0
	for i := range saved {
		var v *apierror.ValidationError
		switch {
		case errs[i] == nil && saved[i]:
			ok++
		case errors.As(errs[i], &v):
			refused++
		default:
			t.Errorf("racer %d: saved %v, err %v; want saved or the limit error", i, saved[i], errs[i])
		}
	}
	if ok != 1 || refused != racers-1 {
		t.Errorf("saved %d, refused %d; want exactly 1 saved and %d refused", ok, refused, racers-1)
	}
}

// At the limit, replacing one of the owner's own dashboards still works, and
// another owner is not affected by the first owner's count.
func TestCodeDashboardRepoIntegration_LimitAllowsReplacingAndOtherOwners(t *testing.T) {
	f := newCodeDashboardFixture(t)
	for i := 0; i < domain.MaxCodeDashboards; i++ {
		f.save(t, codeOwnerA, codeFillID(i), fmt.Sprintf("Fill %d", i), "x")
	}
	_, err := f.repo.Upsert(f.ctx, codeOwnerA, codeDashboard1, domain.SaveCodeDashboardRequest{Name: "One too many", Source: "x"})
	var v *apierror.ValidationError
	if !errors.As(err, &v) || v.Msg != "at most 50 saved advanced dashboards are allowed" {
		t.Fatalf("a new dashboard at the limit: err %v, want the limit error", err)
	}
	if d := f.save(t, codeOwnerA, codeFillID(0), "Fill 0 renamed", "y"); d.Name != "Fill 0 renamed" {
		t.Errorf("replacing at the limit: name %q", d.Name)
	}
	f.save(t, codeOwnerB, codeDashboard2, "Owner B", "x")
}

// Upsert takes the owner's own lock before it counts: while another
// transaction holds it, a save for the owner waits and a save for another
// owner does not. The JSON dashboards' lock for the same owner does not
// hold it up.
func TestCodeDashboardRepoIntegration_UpsertWaitsForTheOwnersLock(t *testing.T) {
	f := newCodeDashboardFixture(t)

	holder, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(f.ctx) //nolint:errcheck // released below; a second Rollback is a no-op
	if _, err := holder.Exec(f.ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, "custom-dashboard-owner:"+codeOwnerA); err != nil {
		t.Fatalf("take the JSON dashboards' lock: %v", err)
	}
	quick, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	if d, err := f.repo.Upsert(quick, codeOwnerA, codeDashboard3, domain.SaveCodeDashboardRequest{Name: "Not held up", Source: "x"}); err != nil || d == nil {
		t.Fatalf("a save while the JSON dashboards' lock was held = %+v, %v; want it saved at once", d, err)
	}
	if _, err := holder.Exec(f.ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, "code-dashboard-owner:"+codeOwnerA); err != nil {
		t.Fatalf("take the owner's lock: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := f.repo.Upsert(f.ctx, codeOwnerA, codeDashboard1, domain.SaveCodeDashboardRequest{Name: "Waits", Source: "x"})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("a save for the owner finished while the owner's lock was held (err %v): it does not take the lock", err)
	case <-time.After(500 * time.Millisecond):
	}
	f.save(t, codeOwnerB, codeDashboard2, "Other owner", "x")

	if err := holder.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the waiting save: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the waiting save did not finish after the lock was released")
	}
}
