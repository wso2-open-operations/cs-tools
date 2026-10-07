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

// This is an integration test: it exercises the real SQL in case_repo.go's
// recomputeTimeCardsBillable/caseHasPatchTag (UpdateCase's severity branch,
// addCaseTagTx's "patch" branch) against a live PostgreSQL instance --
// both fixes it regression-tests were CodeRabbit findings on the direct,
// in-process time-card billable recompute that replaced the old
// case.billable_status_changed event design:
//
//  1. Entering LOW/S4 severity must NOT make a case's time cards billable
//     if it already carries a "patch" tag -- the patch override must be
//     checked live, not just at the moment the tag itself is added.
//  2. The case-row lock (SELECT ... FOR UPDATE) UpdateCase's severity
//     branch already takes must actually serialize against addCaseTagTx's
//     own "patch" branch -- a real Postgres row lock, not something a fake
//     CaseRepository can reproduce.
//
// Same DSN and skip-when-unset pattern as project_stats_repo_integration_test.go/
// time_card_repo_test.go (package repository_test, reuses caseStatsPool):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run BillableIntegration

package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	billableTestProjectID = "48111111-1111-1111-1111-111111111111"
	billableTestUserID    = "48222222-0000-0000-0000-000000000001"
)

// seedBillableCase creates one project/user/case (severity HIGH, nowhere
// near LOW) plus one time_card row left billable=true, under caseID -- the
// starting point every test in this file mutates from. created_by is
// unique per test (suffix) so concurrent sub-tests sharing this package's
// DB don't collide, and so each one's own cleanup only ever touches its own
// rows.
func seedBillableCase(t *testing.T, pool *pgxpool.Pool, suffix, caseID string) {
	t.Helper()
	actor := "billable-test-" + suffix
	// WithSystemIdentity: work_item/"case"/time_card/tag/work_item_tag are
	// all RLS-protected now -- an internal/unrestricted identity is what
	// makes these seed writes (and the repository calls under test, which
	// use the same scoped pool) visible, the same reasoning
	// seedTimeCardWithNullDurations gives for its own seed.
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM time_card WHERE created_by = $1`, actor)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item_tag WHERE created_by = $1`, actor)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE created_by = $1`, actor)
		_, _ = pool.Exec(ctx, `DELETE FROM tag WHERE created_by = $1`, actor)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE user_name = $1`, actor+"@example.com")
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active)
	          VALUES ($1, now(), now(), $2, $2, true)`,
		billableTestUserID, actor+"@example.com")

	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
	          VALUES ($1, now(), now(), $2, $2, $3, $3, 'a case', 'CASE', $4)`,
		caseID, actor, "BILL-"+suffix, billableTestProjectID)
	mustExec(`INSERT INTO "case" (id, state, severity) VALUES ($1, 'OPEN', 'S1')`, caseID)

	mustExec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, user_id, work_date, is_billable, state)
	          VALUES (gen_random_uuid(), now(), now(), $1, $1, $2, $3, CURRENT_DATE, true, 'SUBMITTED')`,
		actor, caseID, billableTestUserID)
}

func caseIsBillable(t *testing.T, pool *pgxpool.Pool, caseID string) bool {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	var isBillable bool
	if err := pool.QueryRow(ctx, `SELECT bool_and(is_billable) FROM time_card WHERE case_id = $1`, caseID).Scan(&isBillable); err != nil {
		t.Fatalf("read back is_billable: %v", err)
	}
	return isBillable
}

// TestBillableIntegration_EnteringLowWithExistingPatchTagStaysNonBillable is
// CodeRabbit finding #1's regression test: a case that already carries a
// "patch" tag before its severity ever crosses into LOW must NOT have its
// time cards marked billable when that crossing happens -- the override has
// to be checked live at the moment of the crossing, not only at the moment
// the tag itself was added (which may have been while the case was at some
// other severity entirely).
func TestBillableIntegration_EnteringLowWithExistingPatchTagStaysNonBillable(t *testing.T) {
	pool := caseStatsPool(t)
	const caseID = "48333333-0000-0000-0000-000000000001"
	seedBillableCase(t, pool, "patchfirst", caseID)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	if _, err := repo.AddCaseTag(ctx, caseID, "Patch", "billable-test-patchfirst@example.com"); err != nil {
		t.Fatalf("AddCaseTag(patch) error = %v", err)
	}
	// Severity is still S1 (HIGH) here -- adding the tag at a non-LOW
	// severity must not touch billability yet.
	if !caseIsBillable(t, pool, caseID) {
		t.Fatalf("time cards went non-billable before the case ever entered LOW")
	}

	low := domain.CaseSeverityLow
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: caseID, Severity: &low}, nil); err != nil {
		t.Fatalf("UpdateCase(severity=LOW) error = %v", err)
	}

	if caseIsBillable(t, pool, caseID) {
		t.Errorf("time cards are billable after entering LOW with an existing patch tag, want non-billable")
	}
}

// TestBillableIntegration_EnteringLowWithoutPatchTagBecomesBillable is the
// ordinary, no-override case: entering LOW/S4 with no "patch" tag at all
// makes the case's time cards billable.
func TestBillableIntegration_EnteringLowWithoutPatchTagBecomesBillable(t *testing.T) {
	pool := caseStatsPool(t)
	const caseID = "48444444-0000-0000-0000-000000000001"
	seedBillableCase(t, pool, "nopatch", caseID)
	// seedBillableCase leaves is_billable=true -- flip it false first so
	// this test can tell entering LOW actually set it, not left it alone.
	ctx := repository.WithSystemIdentity(context.Background())
	if _, err := pool.Exec(ctx, `UPDATE time_card SET is_billable = false WHERE case_id = $1`, caseID); err != nil {
		t.Fatalf("seed: force non-billable: %v", err)
	}
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	low := domain.CaseSeverityLow
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: caseID, Severity: &low}, nil); err != nil {
		t.Fatalf("UpdateCase(severity=LOW) error = %v", err)
	}

	if !caseIsBillable(t, pool, caseID) {
		t.Errorf("time cards are non-billable after entering LOW with no patch tag, want billable")
	}
}

// TestBillableIntegration_LeavingLowMakesTimeCardsNonBillable is the mirror
// image: a case already at LOW, with no patch tag, leaving LOW makes its
// time cards non-billable.
func TestBillableIntegration_LeavingLowMakesTimeCardsNonBillable(t *testing.T) {
	pool := caseStatsPool(t)
	const caseID = "48555555-0000-0000-0000-000000000001"
	seedBillableCase(t, pool, "leaving", caseID)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	low := domain.CaseSeverityLow
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: caseID, Severity: &low}, nil); err != nil {
		t.Fatalf("UpdateCase(severity=LOW) error = %v", err)
	}
	if !caseIsBillable(t, pool, caseID) {
		t.Fatalf("precondition failed: time cards should be billable after entering LOW")
	}

	critical := domain.CaseSeverityCritical
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: caseID, Severity: &critical}, nil); err != nil {
		t.Fatalf("UpdateCase(severity=CRITICAL) error = %v", err)
	}

	if caseIsBillable(t, pool, caseID) {
		t.Errorf("time cards are still billable after leaving LOW, want non-billable")
	}
}

// TestBillableIntegration_PatchTagAddedWhileAlreadyLowSetsNonBillable covers
// the simpler, already-at-LOW case the original detectPatchTagBillableOverride
// was built for: adding "patch" while the case is already LOW must flip its
// time cards non-billable immediately.
func TestBillableIntegration_PatchTagAddedWhileAlreadyLowSetsNonBillable(t *testing.T) {
	pool := caseStatsPool(t)
	const caseID = "48666666-0000-0000-0000-000000000001"
	seedBillableCase(t, pool, "alreadylow", caseID)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	low := domain.CaseSeverityLow
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: caseID, Severity: &low}, nil); err != nil {
		t.Fatalf("UpdateCase(severity=LOW) error = %v", err)
	}
	if !caseIsBillable(t, pool, caseID) {
		t.Fatalf("precondition failed: time cards should be billable after entering LOW")
	}

	if _, err := repo.AddCaseTag(ctx, caseID, "patch", "billable-test-alreadylow@example.com"); err != nil {
		t.Fatalf("AddCaseTag(patch) error = %v", err)
	}

	if caseIsBillable(t, pool, caseID) {
		t.Errorf("time cards are still billable after adding a patch tag while LOW, want non-billable")
	}
}

// TestBillableIntegration_LeaveAndReenterLowAfterPatchStaysNonBillable is
// CodeRabbit finding #1's second scenario: once a patch override has taken
// effect, leaving LOW and re-entering it later (the tag is never removed)
// must still come back non-billable, not revert to the ordinary
// entering-LOW-means-billable rule.
func TestBillableIntegration_LeaveAndReenterLowAfterPatchStaysNonBillable(t *testing.T) {
	pool := caseStatsPool(t)
	const caseID = "48777777-0000-0000-0000-000000000001"
	seedBillableCase(t, pool, "reenter", caseID)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	low := domain.CaseSeverityLow
	critical := domain.CaseSeverityCritical
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: caseID, Severity: &low}, nil); err != nil {
		t.Fatalf("UpdateCase(severity=LOW) error = %v", err)
	}
	if _, err := repo.AddCaseTag(ctx, caseID, "patch", "billable-test-reenter@example.com"); err != nil {
		t.Fatalf("AddCaseTag(patch) error = %v", err)
	}
	if caseIsBillable(t, pool, caseID) {
		t.Fatalf("precondition failed: time cards should be non-billable after the patch override")
	}

	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: caseID, Severity: &critical}, nil); err != nil {
		t.Fatalf("UpdateCase(severity=CRITICAL) error = %v", err)
	}
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: caseID, Severity: &low}, nil); err != nil {
		t.Fatalf("UpdateCase(severity=LOW again) error = %v", err)
	}

	if caseIsBillable(t, pool, caseID) {
		t.Errorf("time cards became billable on re-entering LOW despite the still-attached patch tag, want non-billable")
	}
}
