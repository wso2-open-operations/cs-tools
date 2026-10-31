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

// Integration tests for the per-card limit (8 hours / 480 minutes per time
// card, not a cumulative daily total). The create path's 481-minute rejection
// lives in the service layer (see time_card_service_test.go); here the
// repository must (a) NOT reject a second card on the same case/day/user even
// when the sum exceeds 480, and (b) reject an edit whose resulting card total
// exceeds 480. Same DSN and skip-when-unset pattern as time_card_repo_test.go
// (caseStatsPool, this same package).
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run TimeCardPerCardLimit

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

func dailyCapCreateRequest(minutes int) domain.CreateTimeCardRequest {
	return domain.CreateTimeCardRequest{
		CaseID:         dailyCapTestCaseID,
		Date:           dailyCapTestDate,
		IsBillable:     true,
		WorkLogComment: ptrTo("test"),
		TimeAnalyzing:  minutes,
	}
}

// TestTimeCardPerCardLimit_MultipleCardsSameCaseDayAccepted: a 480-minute card
// followed by a 30-minute card on the same case, day and user are both
// accepted, even though they sum to 510.
func TestTimeCardPerCardLimit_MultipleCardsSameCaseDayAccepted(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	if _, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(480), dailyCapTestUserID); err != nil {
		t.Fatalf("first CreateTimeCard(480) error = %v, want success", err)
	}
	view, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(30), dailyCapTestUserID)
	if err != nil {
		t.Fatalf("second CreateTimeCard(30) error = %v, want success -- the limit is per card, not a daily total", err)
	}
	if view.TotalTime != 30 {
		t.Errorf("second card totalTime = %v, want 30", view.TotalTime)
	}
}

// TestTimeCardPerCardLimit_CreateAllowsExactly480: the inclusive boundary.
func TestTimeCardPerCardLimit_CreateAllowsExactly480(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	view, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(480), dailyCapTestUserID)
	if err != nil {
		t.Fatalf("CreateTimeCard(480) error = %v, want success at exactly the per-card limit", err)
	}
	if view.TotalTime != 480 {
		t.Errorf("created card totalTime = %v, want 480", view.TotalTime)
	}
}

// TestTimeCardPerCardLimit_UpdateRejectsOver480: editing a 100-minute card up
// to 481 minutes is rejected.
func TestTimeCardPerCardLimit_UpdateRejectsOver480(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	created, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(100), dailyCapTestUserID)
	if err != nil {
		t.Fatalf("seed CreateTimeCard() error = %v", err)
	}

	newMinutes := 481
	_, err = repo.UpdateTimeCardFields(ctx, domain.UpdateTimeCardRequest{
		ID:            created.ID,
		TimeAnalyzing: &newMinutes,
	}, dailyCapTestUserID)
	var valErr *apierror.ValidationError
	if err == nil || !errorsAsValidation(err, &valErr) {
		t.Fatalf("UpdateTimeCardFields(481) error = %v, want a *apierror.ValidationError for exceeding the per-card limit", err)
	}
	if valErr.Msg != domain.MaxTimeCardMinutesMessage {
		t.Errorf("error message = %q, want %q", valErr.Msg, domain.MaxTimeCardMinutesMessage)
	}
}

// TestTimeCardPerCardLimit_UpdateJudgesMergedTotal: a partial edit that only
// touches one minute field must be judged against the card's resulting total
// (the stored values of the other four fields included), and an edit that
// lands exactly on 480 is accepted.
func TestTimeCardPerCardLimit_UpdateJudgesMergedTotal(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	created, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(400), dailyCapTestUserID)
	if err != nil {
		t.Fatalf("seed CreateTimeCard() error = %v", err)
	}

	// 400 stored (analyzing) + 81 patching = 481: rejected, though 81 alone is fine.
	patching := 81
	_, err = repo.UpdateTimeCardFields(ctx, domain.UpdateTimeCardRequest{ID: created.ID, TimePatching: &patching}, dailyCapTestUserID)
	var valErr *apierror.ValidationError
	if err == nil || !errorsAsValidation(err, &valErr) {
		t.Fatalf("UpdateTimeCardFields(+81) error = %v, want a *apierror.ValidationError", err)
	}

	// 400 + 80 = 480: accepted.
	patching = 80
	view, err := repo.UpdateTimeCardFields(ctx, domain.UpdateTimeCardRequest{ID: created.ID, TimePatching: &patching}, dailyCapTestUserID)
	if err != nil {
		t.Fatalf("UpdateTimeCardFields(+80) error = %v, want success at exactly 480", err)
	}
	if view.TotalTime != 480 {
		t.Errorf("updated card totalTime = %v, want 480", view.TotalTime)
	}
}

// TestTimeCardPerCardLimit_CommentOnlyEditIgnoresLimit: an edit touching no
// minute field never re-checks the total.
func TestTimeCardPerCardLimit_CommentOnlyEditIgnoresLimit(t *testing.T) {
	pool := caseStatsPool(t)
	seedDailyCapFixture(t, pool)

	repo := repository.NewTimeCardRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	created, err := repo.CreateTimeCard(ctx, dailyCapCreateRequest(480), dailyCapTestUserID)
	if err != nil {
		t.Fatalf("seed CreateTimeCard() error = %v", err)
	}
	comment := "updated comment only"
	if _, err := repo.UpdateTimeCardFields(ctx, domain.UpdateTimeCardRequest{
		ID:             created.ID,
		WorkLogComment: &comment,
	}, dailyCapTestUserID); err != nil {
		t.Fatalf("UpdateTimeCardFields() error = %v, want success for a comment-only edit", err)
	}
}

func errorsAsValidation(err error, target **apierror.ValidationError) bool {
	ve, ok := err.(*apierror.ValidationError)
	if ok {
		*target = ve
	}
	return ok
}
