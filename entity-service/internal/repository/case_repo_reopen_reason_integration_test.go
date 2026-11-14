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

// Integration test for updateCaseQuery's reopen_reason write -- the CASE WHEN
// gate keyed off the row's own pre-statement state (see updateCaseQuery's own
// doc comment), proven here against a real Postgres rather than just the
// stubbed-repository unit tests in internal/service. reopen_reason is
// deliberately never selected back by GetCaseByID/CaseView (write-only by
// design, migration 0292's own comment), so this reads it directly off the
// table rather than through any repository method. Runs against a real
// Postgres. Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ReopenReasonIntegration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const reopenReasonCaseID = "90000000-0000-0000-0000-000000000005"

func seedReopenReasonFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, reopenReasonCaseID)
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().UTC()
	if _, err := scoped.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		VALUES ($1, $2, $2, 'test', 'test', 'RR-TEST-0001', 'RR-TEST-WSO2-0001', 'reopen-reason integration test fixture', 'CASE')`,
		reopenReasonCaseID, now); err != nil {
		t.Fatalf("seed work_item: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO "case" (id, state) VALUES ($1, 'CLOSED')`, reopenReasonCaseID); err != nil {
		t.Fatalf("seed case: %v", err)
	}
}

func readReopenReason(t *testing.T, pool *pgxpool.Pool, caseID string) *string {
	t.Helper()
	var reason *string
	if err := pool.QueryRow(context.Background(), `SELECT reopen_reason FROM "case" WHERE id = $1`, caseID).Scan(&reason); err != nil {
		t.Fatalf("read reopen_reason: %v", err)
	}
	return reason
}

// TestReopenReasonIntegration_AdminReopenPersistsReason proves a genuine
// admin reopen (state moves OUT of closed) writes the given ReopenReason,
// and that a plain close->close no-op never touches the column at all.
func TestReopenReasonIntegration_AdminReopenPersistsReason(t *testing.T) {
	pool := caseStatsPool(t)
	seedReopenReasonFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	// A redundant close->closed PATCH must leave reopen_reason untouched --
	// the CASE WHEN only fires when the new state is NOT closed.
	closed := domain.CaseStateClosed
	stray := "should never be written"
	if _, _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: reopenReasonCaseID, State: &closed, ReopenReason: &stray}, nil); err != nil {
		t.Fatalf("UpdateCase(state=closed): %v", err)
	}
	if got := readReopenReason(t, pool, reopenReasonCaseID); got != nil {
		t.Errorf("reopen_reason = %v after a closed->closed PATCH, want nil", got)
	}

	// A genuine reopen (closed -> work_in_progress) writes the reason.
	workInProgress := domain.CaseStateWorkInProgress
	reason := "customer reported the same issue recurring"
	if _, _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: reopenReasonCaseID, State: &workInProgress, ReopenReason: &reason}, nil); err != nil {
		t.Fatalf("UpdateCase(state=work_in_progress): %v", err)
	}
	got := readReopenReason(t, pool, reopenReasonCaseID)
	if got == nil || *got != reason {
		t.Errorf("reopen_reason = %v, want %q", got, reason)
	}

	// Re-closing and reopening again with no reason clears the stale value
	// rather than leaving the previous cycle's reason behind.
	if _, _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: reopenReasonCaseID, State: &closed}, nil); err != nil {
		t.Fatalf("UpdateCase(state=closed) second close: %v", err)
	}
	if _, _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: reopenReasonCaseID, State: &workInProgress}, nil); err != nil {
		t.Fatalf("UpdateCase(state=work_in_progress) second reopen: %v", err)
	}
	if got := readReopenReason(t, pool, reopenReasonCaseID); got != nil {
		t.Errorf("reopen_reason = %v after a reopen with no reason given, want nil", got)
	}
}

// TestReopenReasonIntegration_AdminReopenClearsStaleResolvedOn is the
// regression guard for a CodeRabbit-caught gap in updateCaseQuery's
// resolved_on CASE expression: before its fourth branch was added, reopening
// a closed case into anything other than the literal "reopened"/
// "solution_proposed" values (e.g. work_in_progress) left the PRIOR
// resolution's resolved_on untouched, which a later re-close's own
// COALESCE(resolved_on, NOW()) would then reuse instead of recording the new
// resolution.
func TestReopenReasonIntegration_AdminReopenClearsStaleResolvedOn(t *testing.T) {
	pool := caseStatsPool(t)
	seedReopenReasonFixture(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	if _, err := pool.Exec(ctx, `UPDATE "case" SET resolved_on = NOW() WHERE id = $1`, reopenReasonCaseID); err != nil {
		t.Fatalf("seed resolved_on: %v", err)
	}
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	workInProgress := domain.CaseStateWorkInProgress
	if _, _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: reopenReasonCaseID, State: &workInProgress}, nil); err != nil {
		t.Fatalf("UpdateCase(state=work_in_progress): %v", err)
	}
	var resolvedOn *time.Time
	if err := pool.QueryRow(ctx, `SELECT resolved_on FROM "case" WHERE id = $1`, reopenReasonCaseID).Scan(&resolvedOn); err != nil {
		t.Fatalf("read resolved_on: %v", err)
	}
	if resolvedOn != nil {
		t.Errorf("resolved_on = %v after reopening into work_in_progress, want nil", resolvedOn)
	}

	// Re-closing afterward must record a FRESH resolved_on, not silently stay
	// nil forever -- confirms the fourth branch didn't just move the bug
	// rather than fix it.
	closed := domain.CaseStateClosed
	if _, _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: reopenReasonCaseID, State: &closed}, nil); err != nil {
		t.Fatalf("UpdateCase(state=closed): %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT resolved_on FROM "case" WHERE id = $1`, reopenReasonCaseID).Scan(&resolvedOn); err != nil {
		t.Fatalf("read resolved_on after re-close: %v", err)
	}
	if resolvedOn == nil {
		t.Error("resolved_on is nil after re-closing, want a fresh timestamp")
	}
}
