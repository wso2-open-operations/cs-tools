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

// Regression test for a real, reported bug: a case closed through this data
// source always had closed_by_user_id left NULL regardless of who closed
// it, so the case detail page's "Closed by" fell back to "System" every
// time. CaseRepository.UpdateCase now stamps closed_by_user_id from an
// actorID parameter (resolved by the service layer from the caller's own
// x-user-id-token, never from the request body) whenever a case-like work
// item's own state transitions to closed, and clears it back to NULL on a
// transition away from closed -- mirroring closed_on's own existing
// transition-gated behaviour exactly. Runs against a real Postgres. Skipped
// without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ClosedByIntegration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	cbCaseID       = "90000000-0000-0000-0000-000000000001"
	cbEngagementID = "90000000-0000-0000-0000-000000000002"
	cbCloserUserID = "90000000-0000-0000-0000-000000000003"
	// cbOtherUserID is a second, different actor -- used to prove a
	// redundant re-close (state already CLOSED, PATCHed to closed again)
	// never overwrites the real closer with whoever/whatever happened to
	// resend it.
	cbOtherUserID = "90000000-0000-0000-0000-000000000004"
)

func seedClosedByFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id IN ($1, $2)`, cbCaseID, cbEngagementID)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id IN ($1, $2)`, cbCloserUserID, cbOtherUserID)
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().UTC()
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}
	mustExecScoped := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed scoped (%.80s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name, email)
		VALUES ($1, $2, $2, 'cb-closer-user', 'Cara', 'Closer', 'cb-closer-user@test.local')`, cbCloserUserID, now)
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name, email)
		VALUES ($1, $2, $2, 'cb-other-user', 'Olly', 'Other', 'cb-other-user@test.local')`, cbOtherUserID, now)

	mustExecScoped(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		VALUES ($1, $2, $2, 'test', 'test', 'CB-TEST-0001', 'CB-TEST-WSO2-0001', 'closed-by integration test fixture', 'CASE')`,
		cbCaseID, now)
	mustExec(`INSERT INTO "case" (id, state) VALUES ($1, 'OPEN')`, cbCaseID)

	mustExecScoped(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		VALUES ($1, $2, $2, 'test', 'test', 'CB-TEST-0002', 'CB-TEST-WSO2-0002', 'closed-by integration test fixture (engagement)', 'ENGAGEMENT')`,
		cbEngagementID, now)
	mustExec(`INSERT INTO engagement (id, state) VALUES ($1, 'OPEN')`, cbEngagementID)
}

// TestClosedByIntegration_ClosingACaseStampsTheActor is the round trip for
// "case" itself: closing with a real actorID stamps closed_by_user_id, and
// GetCaseByID resolves it back into CaseView.ClosedBy with the closer's
// display name.
func TestClosedByIntegration_ClosingACaseStampsTheActor(t *testing.T) {
	pool := caseStatsPool(t)
	seedClosedByFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	closed := domain.CaseStateClosed
	closerID := cbCloserUserID
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: cbCaseID, State: &closed}, &closerID); err != nil {
		t.Fatalf("UpdateCase(state=closed): %v", err)
	}

	cv, err := repo.GetCaseByID(ctx, cbCaseID, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetCaseByID: %v", err)
	}
	if cv.ClosedBy == nil {
		t.Fatal("ClosedBy is nil, want the resolved closer")
	}
	if cv.ClosedBy.ID != cbCloserUserID {
		t.Errorf("ClosedBy.ID = %q, want %q", cv.ClosedBy.ID, cbCloserUserID)
	}
	if cv.ClosedBy.Name != "Cara Closer" {
		t.Errorf("ClosedBy.Name = %q, want %q", cv.ClosedBy.Name, "Cara Closer")
	}

	// Reopening clears it back to NULL -- the same transition-gated
	// behaviour closed_on itself already has, now mirrored for
	// closed_by_user_id.
	open := domain.CaseStateOpen
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: cbCaseID, State: &open}, nil); err != nil {
		t.Fatalf("UpdateCase(state=open): %v", err)
	}
	cv, err = repo.GetCaseByID(ctx, cbCaseID, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetCaseByID after reopen: %v", err)
	}
	if cv.ClosedBy != nil {
		t.Errorf("ClosedBy = %+v after reopening, want nil", cv.ClosedBy)
	}
}

// TestClosedByIntegration_ClosingWithNoResolvedActorLeavesItNil proves the
// best-effort posture: a close with no resolvable actor (actorID nil, e.g.
// a missing/invalid x-user-id-token) still succeeds, simply leaving
// closed_by_user_id unset rather than failing the whole close or guessing.
func TestClosedByIntegration_ClosingWithNoResolvedActorLeavesItNil(t *testing.T) {
	pool := caseStatsPool(t)
	seedClosedByFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	closed := domain.CaseStateClosed
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: cbCaseID, State: &closed}, nil); err != nil {
		t.Fatalf("UpdateCase(state=closed): %v", err)
	}

	cv, err := repo.GetCaseByID(ctx, cbCaseID, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetCaseByID: %v", err)
	}
	if cv.ClosedBy != nil {
		t.Errorf("ClosedBy = %+v, want nil when no actor was resolved", cv.ClosedBy)
	}
	if cv.ClosedOn == nil {
		t.Error("ClosedOn is nil, want a timestamp even with no resolved actor")
	}
}

// TestClosedByIntegration_EngagementAlsoStampsTheActor is the same round
// trip for one of the four non-"case" case-like types (caseLikeExtensionUpdate's
// own branch, a separate SQL statement from updateCaseQuery) -- proving the
// fix isn't specific to "case" itself.
func TestClosedByIntegration_EngagementAlsoStampsTheActor(t *testing.T) {
	pool := caseStatsPool(t)
	seedClosedByFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	closed := domain.CaseStateClosed
	closerID := cbCloserUserID
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: cbEngagementID, State: &closed}, &closerID); err != nil {
		t.Fatalf("UpdateCase(state=closed): %v", err)
	}

	cv, err := repo.GetCaseByID(ctx, cbEngagementID, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetCaseByID: %v", err)
	}
	if cv.ClosedBy == nil || cv.ClosedBy.ID != cbCloserUserID {
		t.Errorf("ClosedBy = %+v, want ID %q", cv.ClosedBy, cbCloserUserID)
	}
}

// TestClosedByIntegration_RedundantRecloseDoesNotOverwriteTheRealCloser is
// the regression guard for a CodeRabbit-caught bug: a caller re-PATCHing an
// already-closed case's state to closed again -- a harmless, idempotent
// no-op everywhere else this codebase applies state transitions -- used to
// unconditionally re-stamp closed_by_user_id with whatever actorID that
// redundant call happened to carry, silently overwriting the real closer.
// closed_by_user_id must only ever be set on a GENUINE transition into
// closed (the stored state was not already closed).
func TestClosedByIntegration_RedundantRecloseDoesNotOverwriteTheRealCloser(t *testing.T) {
	pool := caseStatsPool(t)
	seedClosedByFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	closed := domain.CaseStateClosed
	closerID := cbCloserUserID
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: cbCaseID, State: &closed}, &closerID); err != nil {
		t.Fatalf("UpdateCase(state=closed) first close: %v", err)
	}

	otherID := cbOtherUserID
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: cbCaseID, State: &closed}, &otherID); err != nil {
		t.Fatalf("UpdateCase(state=closed) redundant re-close: %v", err)
	}

	cv, err := repo.GetCaseByID(ctx, cbCaseID, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetCaseByID: %v", err)
	}
	if cv.ClosedBy == nil || cv.ClosedBy.ID != cbCloserUserID {
		t.Errorf("ClosedBy = %+v after a redundant re-close, want it to still be the original closer %q", cv.ClosedBy, cbCloserUserID)
	}
}
