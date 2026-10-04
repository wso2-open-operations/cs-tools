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

// UpdateCase's closure timestamps on a live database: resolved_on is stamped
// when a case enters solution_proposed, closed_on when it enters closed, a
// repeated write of the same state keeps the first timestamp, and reopening
// clears both. Skipped without CASE_STATS_TEST_DSN.

package repository_test

import (
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestUpdateCase_ClosureTimestamps(t *testing.T) {
	pool := caseStatsPool(t)
	seedRLSScopedRemaining(t, pool)
	scoped := repository.NewScoped(pool)
	repo := repository.NewCaseRepository(scoped)
	ctx := rsInternal()
	caseID := rsWorkItems[1] // AWAITING_INFO on project A, no child cases

	setState := func(t *testing.T, st domain.CaseState) {
		t.Helper()
		if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: caseID, State: &st}); err != nil {
			t.Fatalf("UpdateCase(%s): %v", st, err)
		}
	}
	read := func(t *testing.T) (resolved, closed *time.Time) {
		t.Helper()
		if err := scoped.QueryRow(ctx, `SELECT resolved_on, closed_on FROM "case" WHERE id = $1`, caseID).Scan(&resolved, &closed); err != nil {
			t.Fatalf("read case: %v", err)
		}
		return resolved, closed
	}
	pause := func() { time.Sleep(20 * time.Millisecond) } // NOW() differs per transaction

	setState(t, domain.CaseStateSolutionProposed)
	resolved1, closed := read(t)
	if resolved1 == nil || closed != nil {
		t.Fatalf("after solution_proposed: resolved_on=%v closed_on=%v, want resolved set, closed empty", resolved1, closed)
	}

	pause()
	setState(t, domain.CaseStateSolutionProposed)
	if resolved, _ := read(t); resolved == nil || !resolved.Equal(*resolved1) {
		t.Errorf("repeated solution_proposed moved resolved_on: %v -> %v", resolved1, resolved)
	}

	pause()
	setState(t, domain.CaseStateClosed)
	resolved, closed1 := read(t)
	if closed1 == nil || resolved == nil || !resolved.Equal(*resolved1) {
		t.Fatalf("after closed: resolved_on=%v closed_on=%v, want resolved kept and closed set", resolved, closed1)
	}

	pause()
	setState(t, domain.CaseStateClosed)
	if _, closed := read(t); closed == nil || !closed.Equal(*closed1) {
		t.Errorf("repeated close moved closed_on: %v -> %v", closed1, closed)
	}

	setState(t, domain.CaseStateWorkInProgress)
	if resolved, closed := read(t); resolved != nil || closed != nil {
		t.Errorf("after reopening: resolved_on=%v closed_on=%v, want both cleared", resolved, closed)
	}
}
