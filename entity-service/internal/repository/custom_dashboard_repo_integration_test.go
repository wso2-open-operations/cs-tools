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
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These run CustomDashboardRepository against a live PostgreSQL instance.
// Skipped unless ENTITY_TEST_DATABASE_URL is set (see
// account_repo_salesforce_integration_test.go for the setup).

// Fixture users and dashboard ids, distinct from every other integration file's.
const (
	cdbOwnerA     = "cdb00000-0000-4000-8000-00000000000a"
	cdbOwnerB     = "cdb00000-0000-4000-8000-00000000000b"
	cdbDashboard1 = "cdb00000-0000-4000-8000-000000000001"
	cdbDashboard2 = "cdb00000-0000-4000-8000-000000000002"
)

// newCustomDashboardIntegrationPool connects to ENTITY_TEST_DATABASE_URL (the
// test is skipped when it is unset) and seeds owners A and B, removing their
// dashboards and the fixture dashboards before and after the test.
func newCustomDashboardIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	clean := func() {
		// Dashboards first: a shared one outlives its owner (migration 0255),
		// and an earlier run may have left the fixture ids with no owner.
		if _, err := pool.Exec(ctx, `DELETE FROM custom_dashboard WHERE owner_user_id = ANY($1::uuid[]) OR id = ANY($2::uuid[])`,
			[]string{cdbOwnerA, cdbOwnerB}, []string{cdbDashboard1, cdbDashboard2}); err != nil {
			t.Fatalf("clean dashboards: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM "user" WHERE id = ANY($1::uuid[])`,
			[]string{cdbOwnerA, cdbOwnerB}); err != nil {
			t.Fatalf("clean users: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	for _, id := range []string{cdbOwnerA, cdbOwnerB} {
		if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name) VALUES ($1, now(), now(), $2)`,
			id, "cdb-test-"+id); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
	}
	return pool
}

func saveCustomDashboard(t *testing.T, repo CustomDashboardRepository, owner, id, name, definition string) *domain.CustomDashboard {
	t.Helper()
	d, err := repo.Upsert(context.Background(), owner, id, domain.SaveCustomDashboardRequest{Name: name, Definition: definition})
	if err != nil {
		t.Fatalf("Upsert %s: %v", name, err)
	}
	if d == nil {
		t.Fatalf("Upsert %s: got nil, want the saved dashboard", name)
	}
	return d
}

func TestCustomDashboardIntegration_UpsertCreatesThenReplaces(t *testing.T) {
	repo := NewCustomDashboardRepository(newCustomDashboardIntegrationPool(t))
	ctx := context.Background()

	created := saveCustomDashboard(t, repo, cdbOwnerA, cdbDashboard1, "My Cases", `{"v":1}`)
	if created.ID != cdbDashboard1 || created.Name != "My Cases" || created.Definition != `{"v":1}` {
		t.Fatalf("created = %+v", created)
	}

	replaced := saveCustomDashboard(t, repo, cdbOwnerA, cdbDashboard1, "My Open Cases", `{"v":2}`)
	if replaced.Name != "My Open Cases" || replaced.Definition != `{"v":2}` {
		t.Fatalf("replaced = %+v", replaced)
	}
	if !replaced.CreatedOn.Equal(created.CreatedOn) {
		t.Errorf("createdOn changed on replace: %v -> %v", created.CreatedOn, replaced.CreatedOn)
	}
	if replaced.UpdatedOn.Before(created.UpdatedOn) {
		t.Errorf("updatedOn went back: %v -> %v", created.UpdatedOn, replaced.UpdatedOn)
	}

	list, err := repo.List(ctx, cdbOwnerA)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != cdbDashboard1 || list[0].Name != "My Open Cases" {
		t.Fatalf("List = %+v, want the one replaced dashboard", list)
	}

	got, err := repo.Get(ctx, cdbOwnerA, cdbDashboard1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.Definition != `{"v":2}` {
		t.Fatalf("Get = %+v, want the replaced definition", got)
	}
}

func TestCustomDashboardIntegration_ListIsOwnersOnlyByName(t *testing.T) {
	repo := NewCustomDashboardRepository(newCustomDashboardIntegrationPool(t))

	saveCustomDashboard(t, repo, cdbOwnerA, cdbDashboard1, "zeta", `{}`)
	saveCustomDashboard(t, repo, cdbOwnerA, cdbDashboard2, "Alpha", `{}`)

	list, err := repo.List(context.Background(), cdbOwnerA)
	if err != nil {
		t.Fatalf("List A: %v", err)
	}
	if len(list) != 2 || list[0].Name != "Alpha" || list[1].Name != "zeta" {
		t.Fatalf("List A = %+v, want Alpha then zeta", list)
	}

	other, err := repo.List(context.Background(), cdbOwnerB)
	if err != nil {
		t.Fatalf("List B: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("List B = %+v, want none", other)
	}
}

func TestCustomDashboardIntegration_OtherOwnerSeesNothing(t *testing.T) {
	repo := NewCustomDashboardRepository(newCustomDashboardIntegrationPool(t))
	ctx := context.Background()

	saveCustomDashboard(t, repo, cdbOwnerA, cdbDashboard1, "Mine", `{"owner":"A"}`)

	got, err := repo.Get(ctx, cdbOwnerB, cdbDashboard1)
	if err != nil || got != nil {
		t.Fatalf("Get by B = %+v, %v; want nil, nil", got, err)
	}

	overwritten, err := repo.Upsert(ctx, cdbOwnerB, cdbDashboard1, domain.SaveCustomDashboardRequest{Name: "Taken", Definition: `{"owner":"B"}`})
	if err != nil || overwritten != nil {
		t.Fatalf("Upsert by B = %+v, %v; want nil, nil", overwritten, err)
	}

	deleted, err := repo.Delete(ctx, cdbOwnerB, cdbDashboard1)
	if err != nil || deleted {
		t.Fatalf("Delete by B = %v, %v; want false, nil", deleted, err)
	}

	still, err := repo.Get(ctx, cdbOwnerA, cdbDashboard1)
	if err != nil || still == nil || still.Name != "Mine" || still.Definition != `{"owner":"A"}` {
		t.Fatalf("A's dashboard after B's attempts = %+v, %v; want it unchanged", still, err)
	}
}

func TestCustomDashboardIntegration_DuplicateNameIsConflict(t *testing.T) {
	repo := NewCustomDashboardRepository(newCustomDashboardIntegrationPool(t))

	saveCustomDashboard(t, repo, cdbOwnerA, cdbDashboard1, "My Cases", `{}`)

	_, err := repo.Upsert(context.Background(), cdbOwnerA, cdbDashboard2, domain.SaveCustomDashboardRequest{Name: "MY CASES", Definition: `{}`})
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Upsert with a taken name: err = %v, want a ConflictError", err)
	}

	// Another owner may use the same name.
	saveCustomDashboard(t, repo, cdbOwnerB, cdbDashboard2, "My Cases", `{}`)
}

func TestCustomDashboardIntegration_DeleteAndGetMissing(t *testing.T) {
	repo := NewCustomDashboardRepository(newCustomDashboardIntegrationPool(t))
	ctx := context.Background()

	saveCustomDashboard(t, repo, cdbOwnerA, cdbDashboard1, "Temp", `{}`)

	deleted, err := repo.Delete(ctx, cdbOwnerA, cdbDashboard1)
	if err != nil || !deleted {
		t.Fatalf("Delete = %v, %v; want true, nil", deleted, err)
	}
	again, err := repo.Delete(ctx, cdbOwnerA, cdbDashboard1)
	if err != nil || again {
		t.Fatalf("second Delete = %v, %v; want false, nil", again, err)
	}
	got, err := repo.Get(ctx, cdbOwnerA, cdbDashboard1)
	if err != nil || got != nil {
		t.Fatalf("Get after delete = %+v, %v; want nil, nil", got, err)
	}
}

// cdbFillID is a dashboard id for filling an owner up to the limit.
func cdbFillID(i int) string { return fmt.Sprintf("cdb00000-0000-4000-8000-1%011d", i) }

// Saves racing at the limit: with the owner at one below the cap, many new
// dashboards saved at the same moment may add exactly one. This does not by
// itself prove the lock (the saves rarely overlap closely enough without it);
// TestCustomDashboardIntegration_UpsertWaitsForTheOwnersLock does.
func TestCustomDashboardIntegration_LimitHoldsUnderConcurrentSaves(t *testing.T) {
	repo := NewCustomDashboardRepository(newCustomDashboardIntegrationPool(t))
	for i := 0; i < domain.MaxCustomDashboards-1; i++ {
		saveCustomDashboard(t, repo, cdbOwnerA, cdbFillID(i), fmt.Sprintf("Fill %d", i), `{}`)
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
			d, err := repo.Upsert(context.Background(), cdbOwnerA, cdbFillID(1000+i),
				domain.SaveCustomDashboardRequest{Name: fmt.Sprintf("Racer %d", i), Definition: `{}`})
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
	list, err := repo.List(context.Background(), cdbOwnerA)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != domain.MaxCustomDashboards {
		t.Errorf("owner has %d dashboards, want exactly %d", len(list), domain.MaxCustomDashboards)
	}
}

// At the limit, replacing one of the owner's own dashboards still works, and
// another owner is not affected by the first owner's count.
func TestCustomDashboardIntegration_LimitAllowsReplacingAndOtherOwners(t *testing.T) {
	repo := NewCustomDashboardRepository(newCustomDashboardIntegrationPool(t))
	for i := 0; i < domain.MaxCustomDashboards; i++ {
		saveCustomDashboard(t, repo, cdbOwnerA, cdbFillID(i), fmt.Sprintf("Fill %d", i), `{}`)
	}
	_, err := repo.Upsert(context.Background(), cdbOwnerA, cdbDashboard1, domain.SaveCustomDashboardRequest{Name: "One too many", Definition: `{}`})
	var v *apierror.ValidationError
	if !errors.As(err, &v) {
		t.Fatalf("a new dashboard at the limit: err %v, want the limit error", err)
	}
	if d := saveCustomDashboard(t, repo, cdbOwnerA, cdbFillID(0), "Fill 0 renamed", `{"x":1}`); d.Name != "Fill 0 renamed" {
		t.Errorf("replacing at the limit: name %q", d.Name)
	}
	saveCustomDashboard(t, repo, cdbOwnerB, cdbDashboard2, "Owner B", `{}`)
}

// Upsert takes the owner's lock before it counts: while another transaction
// holds that lock, a save for the owner waits, a save for another owner does
// not, and the waiting save finishes once the lock is released. Without the
// lock the first save would finish at once, and the count and the insert of
// two saves could interleave.
func TestCustomDashboardIntegration_UpsertWaitsForTheOwnersLock(t *testing.T) {
	pool := newCustomDashboardIntegrationPool(t)
	repo := NewCustomDashboardRepository(pool)
	ctx := context.Background()

	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx) //nolint:errcheck // released below; a second Rollback is a no-op
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, "custom-dashboard-owner:"+cdbOwnerA); err != nil {
		t.Fatalf("take the owner's lock: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := repo.Upsert(ctx, cdbOwnerA, cdbDashboard1, domain.SaveCustomDashboardRequest{Name: "Waits", Definition: `{}`})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("a save for the owner finished while the owner's lock was held (err %v): it does not take the lock", err)
	case <-time.After(500 * time.Millisecond):
	}

	saveCustomDashboard(t, repo, cdbOwnerB, cdbDashboard2, "Other owner", `{}`)

	if err := holder.Rollback(ctx); err != nil {
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
