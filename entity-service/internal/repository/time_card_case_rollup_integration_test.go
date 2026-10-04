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

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const timeCardPartialNullCardID = "7a000000-0000-4000-8000-0000000000e1"

// A card with only some minute columns filled must still count those
// minutes in the per-case roll-up (a NULL bucket used to null the card's whole
// sum). Skipped without CASE_STATS_TEST_DSN.
func TestTimeCardIntegration_CaseRollupCountsPartiallyFilledCards(t *testing.T) {
	pool := caseStatsPool(t)
	seedTimeCardWithNullDurations(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	t.Cleanup(func() { _, _ = scoped.Exec(ctx, `DELETE FROM time_card WHERE id = $1`, timeCardPartialNullCardID) })
	if _, err := scoped.Exec(ctx, `INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, user_id, work_date, is_billable, state, analyzing_minutes, patching_minutes)
		VALUES ($1, now(), now(), 't', 't', $2, $3, CURRENT_DATE, true, 'SUBMITTED', 30, 15)`,
		timeCardPartialNullCardID, timeCardTestCaseID, timeCardTestUserID); err != nil {
		t.Fatalf("seed partial card: %v", err)
	}

	repo := repository.NewTimeCardRepository(scoped)
	got, _, err := repo.SearchCaseTimeCards(ctx, domain.SearchTimeCardsRequest{
		Filters:    &domain.SearchTimeCardsFilters{CaseID: ptrTo(timeCardTestCaseID)},
		Pagination: domain.Pagination{Limit: 10},
	}, "")
	if err != nil {
		t.Fatalf("SearchCaseTimeCards: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d case roll-ups, want 1", len(got))
	}
	s := got[0]
	if s.TotalCount != 2 || s.TotalTime <= 0 || s.Billable.TotalTime != s.TotalTime || s.NonBillable.TotalTime != 0 {
		t.Errorf("roll-up = %+v, want 2 cards whose 45 billable minutes all count", s)
	}
}
