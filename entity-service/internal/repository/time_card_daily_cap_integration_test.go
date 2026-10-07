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

// Integration tests for the "no more than 8 hours (480 minutes) per ticket
// per submitter per day" cap (digiops-cs#3270), enforced inside
// CreateTimeCard/UpdateTimeCardFields's own transactions. Same DSN and
// skip-when-unset pattern as time_card_repo_test.go (caseStatsPool, this
// same package) -- the cap check runs raw SQL (an advisory lock, a SUM
// across sibling rows) that only a real Postgres can verify.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run TimeCardDailyCap

package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	dailyCapTestProjectID = "41111111-1111-1111-1111-111111111112"
	dailyCapTestUserID    = "42222222-0000-0000-0000-000000000002"
	dailyCapTestCaseID    = "45555555-0000-0000-0000-000000000002"
	dailyCapTestDate      = "2026-01-15"
)

// seedDailyCapFixture creates one project/user/case for the daily-cap tests,
// isolated from time_card_repo_test.go's own fixture (different ids,
// different created_by marker) so the two test files can run in the same
// package without interfering with each other's rows.
func seedDailyCapFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM time_card WHERE created_by = 'time-card-cap-test'`)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE created_by = 'time-card-cap-test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE user_name = 'time-card-cap-test@example.com'`)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, dailyCapTestProjectID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, end_date)
	          VALUES ($1, now(), now(), 'time-card-cap-test', 'time-card-cap-test', 'TCCAPTEST', 'sf-tccaptest', (now() + INTERVAL '30 days')::date)`,
		dailyCapTestProjectID)

	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active)
	          VALUES ($1, now(), now(), 'time-card-cap-test@example.com', 'time-card-cap-test@example.com', true)`,
		dailyCapTestUserID)

	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
	          VALUES ($1, now(), now(), 'time-card-cap-test', 'time-card-cap-test', 'TCCAP01', 'TCCAP-1', 'a case', 'CASE', $2)`,
		dailyCapTestCaseID, dailyCapTestProjectID)
	mustExec(`INSERT INTO "case" (id, state, severity) VALUES ($1, 'OPEN', 'S2')`, dailyCapTestCaseID)
}

// seedSiblingTimeCard inserts one already-existing time_card row for
// dailyCapTestUserID/dailyCapTestCaseID/dailyCapTestDate, in the given
// state, with totalMinutes split evenly across the five duration columns
// (the split doesn't matter to the cap, which sums all five).
func seedSiblingTimeCard(t *testing.T, pool *pgxpool.Pool, id string, totalMinutes int, state string) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	if _, err := scoped.Exec(ctx, `
		INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, user_id, work_date, is_billable, state, analyzing_minutes)
		VALUES ($1, now(), now(), 'time-card-cap-test', 'time-card-cap-test', $2, $3, $4::date, true, $5::text::time_card_state_enum, $6)`,
		id, dailyCapTestCaseID, dailyCapTestUserID, dailyCapTestDate, state, totalMinutes,
	); err != nil {
		t.Fatalf("seed sibling time card: %v", err)
	}
}

func dailyCapCreateRequest(minutes int) domain.CreateTimeCardRequest {
	return domain.CreateTimeCardRequest{
		CaseID:         dailyCapTestCaseID,
		Date:           dailyCapTestDate,
		IsBillable:     true,
		WorkLogComment: ptrTo("test"),
		TimeAnalyzing:  minutes,
	}
}

// TestTimeCardDailyCap_CreateRejectsOverCap: an existing 300-minute sibling
// plus a new 250-minute card (550 total) is over the 480-minute cap.
func TestTimeCardDailyCap_CreateRejectsOverCap(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)
	seedSiblingTimeCard(t, pool, "47777777-0000-0000-0000-000000000001", 300, "SUBMITTED")

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	_, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(250), dailyCapTestUserID)
	var valErr *apierror.ValidationError
	if err == nil || !errorsAsValidation(err, &valErr) {
		t.Fatalf("CreateTimeCard() error = %v, want a *apierror.ValidationError for exceeding the daily cap", err)
	}
}

// TestTimeCardDailyCap_CreateAllowsExactlyAtCap: 300 existing + 180 new =
// exactly 480, the inclusive boundary, must succeed.
func TestTimeCardDailyCap_CreateAllowsExactlyAtCap(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)
	seedSiblingTimeCard(t, pool, "47777777-0000-0000-0000-000000000002", 300, "SUBMITTED")

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	view, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(180), dailyCapTestUserID)
	if err != nil {
		t.Fatalf("CreateTimeCard() error = %v, want success at exactly the 480-minute cap", err)
	}
	if view.TotalTime != 180 {
		t.Errorf("created card totalTime = %v, want 180", view.TotalTime)
	}
}

// TestTimeCardDailyCap_RejectedSiblingsDontCount: a REJECTED sibling's
// minutes must not count toward the cap.
func TestTimeCardDailyCap_RejectedSiblingsDontCount(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)
	seedSiblingTimeCard(t, pool, "47777777-0000-0000-0000-000000000003", 400, "REJECTED")

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	if _, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(300), dailyCapTestUserID); err != nil {
		t.Fatalf("CreateTimeCard() error = %v, want success -- the 400-minute sibling is REJECTED and must not count", err)
	}
}

// TestTimeCardDailyCap_UpdateRejectsOverCap: editing a 100-minute card up to
// 300 minutes, with an unrelated 300-minute sibling already logged the same
// day, would total 600 -- over the cap.
func TestTimeCardDailyCap_UpdateRejectsOverCap(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)
	seedSiblingTimeCard(t, pool, "47777777-0000-0000-0000-000000000004", 300, "SUBMITTED")

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	created, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(100), dailyCapTestUserID)
	if err != nil {
		t.Fatalf("seed CreateTimeCard() error = %v", err)
	}

	newMinutes := 300
	_, err = repo.UpdateTimeCardFields(ctx, domain.UpdateTimeCardRequest{
		ID:            created.ID,
		TimeAnalyzing: &newMinutes,
	}, dailyCapTestUserID)
	var valErr *apierror.ValidationError
	if err == nil || !errorsAsValidation(err, &valErr) {
		t.Fatalf("UpdateTimeCardFields() error = %v, want a *apierror.ValidationError for exceeding the daily cap", err)
	}
}

// TestTimeCardDailyCap_UpdateExcludesOwnPriorValue: editing a card's own
// minutes must compare against OTHER cards only -- not double-count this
// same card's stored value before the edit.
func TestTimeCardDailyCap_UpdateExcludesOwnPriorValue(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	created, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(400), dailyCapTestUserID)
	if err != nil {
		t.Fatalf("seed CreateTimeCard() error = %v", err)
	}

	// Raising this same card from 400 to 480 must succeed: if the check
	// wrongly counted the card's own pre-edit 400 minutes as a "sibling" on
	// top of its new 480, this would be rejected as 880.
	newMinutes := 480
	view, err := repo.UpdateTimeCardFields(ctx, domain.UpdateTimeCardRequest{
		ID:            created.ID,
		TimeAnalyzing: &newMinutes,
	}, dailyCapTestUserID)
	if err != nil {
		t.Fatalf("UpdateTimeCardFields() error = %v, want success -- the card's own prior value must not be double-counted", err)
	}
	if view.TotalTime != 480 {
		t.Errorf("updated card totalTime = %v, want 480", view.TotalTime)
	}
}

// TestTimeCardDailyCap_UpdateSkipsCheckWhenMinutesAndDateUnchanged: an edit
// touching only the work log comment must not run the cap check at all --
// confirmed here by an edit that would exceed the cap if it DID run the
// check. The 480-minute sibling is seeded by raw SQL *after* the 480-minute
// card is created through the repo (so creating it doesn't itself trip the
// cap, which it correctly would if both existed up front): the two rows
// together are already 960 minutes, an over-cap state the repo itself would
// never have allowed to arise on its own, deliberately constructed so a
// comment-only edit that DID still re-run the check would be rejected.
func TestTimeCardDailyCap_UpdateSkipsCheckWhenMinutesAndDateUnchanged(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	created, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(480), dailyCapTestUserID)
	if err != nil {
		t.Fatalf("seed CreateTimeCard() error = %v", err)
	}
	seedSiblingTimeCard(t, pool, "47777777-0000-0000-0000-000000000005", 480, "SUBMITTED")

	comment := "updated comment only"
	if _, err := repo.UpdateTimeCardFields(ctx, domain.UpdateTimeCardRequest{
		ID:             created.ID,
		WorkLogComment: &comment,
	}, dailyCapTestUserID); err != nil {
		t.Fatalf("UpdateTimeCardFields() error = %v, want success -- a comment-only edit must skip the daily cap check entirely", err)
	}
}

func errorsAsValidation(err error, target **apierror.ValidationError) bool {
	ve, ok := err.(*apierror.ValidationError)
	if ok {
		*target = ve
	}
	return ok
}
