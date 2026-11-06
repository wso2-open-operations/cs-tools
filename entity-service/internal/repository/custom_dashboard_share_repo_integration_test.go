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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These run CustomDashboardShareRepository against a live PostgreSQL instance,
// on the fixture users and pool of custom_dashboard_repo_integration_test.go
// (owner A, B), plus a third user C. Skipped unless ENTITY_TEST_DATABASE_URL is
// set.

const cdbOwnerC = "cdb00000-0000-4000-8000-00000000000c"

type shareFixture struct {
	shares     CustomDashboardShareRepository
	dashboards CustomDashboardRepository
	ctx        context.Context
}

// newShareFixture: A owns dashboards 1 ("Zeta") and 2 ("Alpha"); B and C exist.
func newShareFixture(t *testing.T) shareFixture {
	t.Helper()
	pool := newCustomDashboardIntegrationPool(t)
	ctx := context.Background()
	clean := func() {
		if _, err := pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, cdbOwnerC); err != nil {
			t.Fatalf("clean user C: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name, email, first_name, last_name)
		VALUES ($1, now(), now(), 'cdb-test-c', 'carol.c@example.test', 'Carol', 'Cee')`, cdbOwnerC); err != nil {
		t.Fatalf("seed user C: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET email = 'anna.a@example.test', first_name = 'Anna', last_name = 'Aye' WHERE id = $1`, cdbOwnerA); err != nil {
		t.Fatalf("name user A: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET email = 'ben.b@example.test', first_name = 'Ben', last_name = 'Bee' WHERE id = $1`, cdbOwnerB); err != nil {
		t.Fatalf("name user B: %v", err)
	}
	dashboards := NewCustomDashboardRepository(pool)
	saveCustomDashboard(t, dashboards, cdbOwnerA, cdbDashboard1, "Zeta", `{"v":1}`)
	saveCustomDashboard(t, dashboards, cdbOwnerA, cdbDashboard2, "Alpha", `{"v":2}`)
	return shareFixture{shares: NewCustomDashboardShareRepository(pool), dashboards: dashboards, ctx: ctx}
}

func (f shareFixture) add(t *testing.T, owner, dashboard, user string) bool {
	t.Helper()
	ok, err := f.shares.AddShare(f.ctx, owner, dashboard, user)
	if err != nil {
		t.Fatalf("AddShare: %v", err)
	}
	return ok
}

func TestCustomDashboardShareIntegration_OwnerAddsListsAndRemoves(t *testing.T) {
	f := newShareFixture(t)

	if !f.add(t, cdbOwnerA, cdbDashboard1, cdbOwnerC) || !f.add(t, cdbOwnerA, cdbDashboard1, cdbOwnerB) {
		t.Fatal("AddShare on the owner's dashboard returned false")
	}
	// Sharing again is not an error and adds no second row.
	if !f.add(t, cdbOwnerA, cdbDashboard1, cdbOwnerB) {
		t.Fatal("AddShare a second time returned false")
	}

	shares, err := f.shares.ListShares(f.ctx, cdbOwnerA, cdbDashboard1)
	if err != nil {
		t.Fatalf("ListShares: %v", err)
	}
	if len(shares) != 2 || shares[0].User.Name != "Ben Bee" || shares[1].User.Name != "Carol Cee" {
		t.Fatalf("shares = %+v, want Ben Bee then Carol Cee", shares)
	}
	if shares[0].User.Email != "ben.b@example.test" || shares[0].User.ID == nil || *shares[0].User.ID != cdbOwnerB || shares[0].SharedOn.IsZero() {
		t.Errorf("share = %+v", shares[0])
	}

	removed, err := f.shares.RemoveShare(f.ctx, cdbOwnerA, cdbDashboard1, cdbOwnerB)
	if err != nil || !removed {
		t.Fatalf("RemoveShare = %v, %v; want true, nil", removed, err)
	}
	again, err := f.shares.RemoveShare(f.ctx, cdbOwnerA, cdbDashboard1, cdbOwnerB)
	if err != nil || again {
		t.Fatalf("second RemoveShare = %v, %v; want false, nil", again, err)
	}
	left, _ := f.shares.ListShares(f.ctx, cdbOwnerA, cdbDashboard1)
	if len(left) != 1 || left[0].User.Name != "Carol Cee" {
		t.Fatalf("shares after remove = %+v, want Carol Cee only", left)
	}
}

func TestCustomDashboardShareIntegration_OnlyTheOwnerManagesShares(t *testing.T) {
	f := newShareFixture(t)
	f.add(t, cdbOwnerA, cdbDashboard1, cdbOwnerC)

	// B does not own dashboard 1: cannot add, list or remove its shares.
	if ok := f.add(t, cdbOwnerB, cdbDashboard1, cdbOwnerB); ok {
		t.Error("B could add a share to A's dashboard")
	}
	if list, err := f.shares.ListShares(f.ctx, cdbOwnerB, cdbDashboard1); err != nil || len(list) != 0 {
		t.Errorf("B listed A's shares: %+v, %v", list, err)
	}
	if removed, err := f.shares.RemoveShare(f.ctx, cdbOwnerB, cdbDashboard1, cdbOwnerC); err != nil || removed {
		t.Errorf("B removed a share of A's dashboard: %v, %v", removed, err)
	}
	// A recipient cannot manage the shares either.
	if ok := f.add(t, cdbOwnerC, cdbDashboard1, cdbOwnerB); ok {
		t.Error("recipient C could add a share")
	}

	list, _ := f.shares.ListShares(f.ctx, cdbOwnerA, cdbDashboard1)
	if len(list) != 1 || list[0].User.Name != "Carol Cee" {
		t.Fatalf("A's shares after the attempts = %+v, want Carol Cee only", list)
	}
}

func TestCustomDashboardShareIntegration_UnknownUserAndDashboard(t *testing.T) {
	f := newShareFixture(t)

	_, err := f.shares.AddShare(f.ctx, cdbOwnerA, cdbDashboard1, "cdb00000-0000-4000-8000-0000000000ff")
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("AddShare with an unknown user: err = %v, want a NotFoundError", err)
	}
	if ok := f.add(t, cdbOwnerA, "cdb00000-0000-4000-8000-0000000000fe", cdbOwnerB); ok {
		t.Error("AddShare on a dashboard that does not exist returned true")
	}
}

func TestCustomDashboardShareIntegration_RecipientSeesOnlyWhatIsShared(t *testing.T) {
	f := newShareFixture(t)
	f.add(t, cdbOwnerA, cdbDashboard1, cdbOwnerC)
	f.add(t, cdbOwnerA, cdbDashboard2, cdbOwnerC)

	list, err := f.shares.ListSharedWithUser(f.ctx, cdbOwnerC)
	if err != nil {
		t.Fatalf("ListSharedWithUser: %v", err)
	}
	if len(list) != 2 || list[0].Name != "Alpha" || list[1].Name != "Zeta" {
		t.Fatalf("shared with C = %+v, want Alpha then Zeta", list)
	}
	if list[0].Owner.Name != "Anna Aye" || list[0].Owner.Email != "anna.a@example.test" || list[0].UpdatedOn.IsZero() {
		t.Errorf("owner = %+v", list[0].Owner)
	}

	got, err := f.shares.GetSharedWithUser(f.ctx, cdbOwnerC, cdbDashboard1)
	if err != nil || got == nil || got.Definition != `{"v":1}` || got.Owner.Name != "Anna Aye" {
		t.Fatalf("GetSharedWithUser = %+v, %v", got, err)
	}

	// B has nothing shared: an empty list, and no access to A's dashboard.
	if other, err := f.shares.ListSharedWithUser(f.ctx, cdbOwnerB); err != nil || len(other) != 0 {
		t.Errorf("shared with B = %+v, %v; want none", other, err)
	}
	if none, err := f.shares.GetSharedWithUser(f.ctx, cdbOwnerB, cdbDashboard1); err != nil || none != nil {
		t.Errorf("B got A's dashboard: %+v, %v", none, err)
	}
	// The owner's own dashboards are not "shared with" the owner.
	if own, err := f.shares.ListSharedWithUser(f.ctx, cdbOwnerA); err != nil || len(own) != 0 {
		t.Errorf("shared with A = %+v, %v; want none", own, err)
	}
}

func TestCustomDashboardShareIntegration_RecipientSeesTheOwnersLatestVersion(t *testing.T) {
	f := newShareFixture(t)
	f.add(t, cdbOwnerA, cdbDashboard1, cdbOwnerC)

	if _, err := f.dashboards.Upsert(f.ctx, cdbOwnerA, cdbDashboard1, domain.SaveCustomDashboardRequest{Name: "Zeta v2", Definition: `{"v":3}`}); err != nil {
		t.Fatalf("owner edits: %v", err)
	}
	got, err := f.shares.GetSharedWithUser(f.ctx, cdbOwnerC, cdbDashboard1)
	if err != nil || got == nil || got.Name != "Zeta v2" || got.Definition != `{"v":3}` {
		t.Fatalf("after the owner's edit C sees %+v, %v", got, err)
	}
}

func TestCustomDashboardShareIntegration_UnshareAndDeleteRemoveAccess(t *testing.T) {
	f := newShareFixture(t)
	f.add(t, cdbOwnerA, cdbDashboard1, cdbOwnerC)
	f.add(t, cdbOwnerA, cdbDashboard2, cdbOwnerC)

	if _, err := f.shares.RemoveShare(f.ctx, cdbOwnerA, cdbDashboard1, cdbOwnerC); err != nil {
		t.Fatalf("RemoveShare: %v", err)
	}
	if got, _ := f.shares.GetSharedWithUser(f.ctx, cdbOwnerC, cdbDashboard1); got != nil {
		t.Error("C still sees dashboard 1 after it was unshared")
	}

	if _, err := f.dashboards.Delete(f.ctx, cdbOwnerA, cdbDashboard2); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if list, _ := f.shares.ListSharedWithUser(f.ctx, cdbOwnerC); len(list) != 0 {
		t.Errorf("shared with C after unshare and delete = %+v, want none", list)
	}
}
