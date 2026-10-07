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

// Runs against a real Postgres with every migration applied (same DSN as
// project_case_stats_repo_integration_test.go). Skipped without
// CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseFeedbackIntegration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	cfCaseID     = "80000000-0000-0000-0000-000000000001"
	cfUserID     = "80000000-0000-0000-0000-000000000002"
	cfEmojiID    = "80000000-0000-0000-0000-000000000003" // a "Very Satisfied - Reasons" fixture row this test seeds itself
	cfChipID     = "80000000-0000-0000-0000-000000000004" // one of that emoji's own options
	cfWrongID    = "80000000-0000-0000-0000-000000000005" // a DIFFERENT emoji, for the chip-mismatch test
	cfWrongOpt   = "80000000-0000-0000-0000-000000000006"
	cfOpenCaseID = "80000000-0000-0000-0000-000000000007" // a second, still-OPEN case for the not-yet-closed rejection test
)

// seedCaseFeedbackFixture creates one CLOSED work_item/"case" (cfCaseID --
// feedback is a post-closure survey, so every test that expects a
// submission to succeed, or to fail for an emoji/chip-specific reason,
// needs a case that's actually closed to get past that gate first), one
// still-OPEN work_item/"case" (cfOpenCaseID, for the "not yet closed"
// rejection test), one user, one feedback metric ("Very Satisfied -
// Reasons") with one option, and a second, unrelated metric+option pair
// (for the "chip belongs to a different emoji" rejection test) -- all
// under ids this test owns and cleans up itself, never touching whatever a
// real synced environment's own rows look like.
func seedCaseFeedbackFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item_feedback WHERE work_item_id IN ($1, $2)`, cfCaseID, cfOpenCaseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id IN ($1, $2)`, cfCaseID, cfOpenCaseID)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, cfUserID)
		_, _ = pool.Exec(ctx, `DELETE FROM work_item_feedback_metric_option WHERE id IN ($1, $2)`, cfChipID, cfWrongOpt)
		_, _ = pool.Exec(ctx, `DELETE FROM work_item_feedback_metric WHERE id IN ($1, $2)`, cfEmojiID, cfWrongID)
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
		VALUES ($1, $2, $2, 'cf-test-user', 'CF', 'Tester', 'cf-test-user@test.local')`, cfUserID, now)

	mustExecScoped(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		VALUES ($1, $2, $2, 'test', 'test', 'CF-TEST-0001', 'CF-TEST-WSO2-0001', 'Case feedback integration test fixture', 'CASE')`,
		cfCaseID, now)
	mustExec(`INSERT INTO "case" (id, state) VALUES ($1, 'CLOSED')`, cfCaseID)

	mustExecScoped(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		VALUES ($1, $2, $2, 'test', 'test', 'CF-TEST-0002', 'CF-TEST-WSO2-0002', 'Case feedback integration test fixture (open)', 'CASE')`,
		cfOpenCaseID, now)
	mustExec(`INSERT INTO "case" (id, state) VALUES ($1, 'OPEN')`, cfOpenCaseID)

	mustExec(`INSERT INTO work_item_feedback_metric (id, name, datatype, is_active, selected_image, unselected_image, created_on, updated_on, created_by, updated_by)
		VALUES ($1, 'Very Satisfied - Reasons', 'checkbox', true, '/assets/feedback/verySatisfiedSelected.svg', '/assets/feedback/verySatisfiedUnselected.svg', $2, $2, 'test', 'test')`,
		cfEmojiID, now)
	mustExec(`INSERT INTO work_item_feedback_metric_option (id, metric_id, label, value, created_on, updated_on, created_by, updated_by)
		VALUES ($1, $2, 'Excellent service', 1, $3, $3, 'test', 'test')`,
		cfChipID, cfEmojiID, now)

	mustExec(`INSERT INTO work_item_feedback_metric (id, name, datatype, is_active, selected_image, unselected_image, created_on, updated_on, created_by, updated_by)
		VALUES ($1, 'Very Dissatisfied - Reasons', 'checkbox', true, '/assets/feedback/veryDissatisfiedSelected.svg', '/assets/feedback/veryDissatisfiedUnselected.svg', $2, $2, 'test', 'test')`,
		cfWrongID, now)
	mustExec(`INSERT INTO work_item_feedback_metric_option (id, metric_id, label, value, created_on, updated_on, created_by, updated_by)
		VALUES ($1, $2, 'Slow response', 1, $3, $3, 'test', 'test')`,
		cfWrongOpt, cfWrongID, now)
}

func TestCaseFeedbackIntegration_GetBeforeSubmitIsNotFound(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseFeedbackFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	_, found, err := repo.GetCaseFeedback(repository.WithSystemIdentity(context.Background()), cfCaseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected found=false before any feedback is submitted")
	}
}

func TestCaseFeedbackIntegration_CreateThenGetRoundTrips(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseFeedbackFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	comment := "Great support, thanks!"
	created, err := repo.CreateCaseFeedback(ctx, cfCaseID, repository.CreateCaseFeedbackParams{
		EmojiID:           cfEmojiID,
		ChipIDs:           []string{cfChipID},
		AdditionalComment: &comment,
		SubmittedByUserID: cfUserID,
		ActorEmail:        "cf-test-user@test.local",
	})
	if err != nil {
		t.Fatalf("CreateCaseFeedback: %v", err)
	}
	if created.ID == "" || created.CreatedOn == "" {
		t.Fatalf("unexpected CreateCaseFeedback result: %+v", created)
	}

	row, found, err := repo.GetCaseFeedback(ctx, cfCaseID)
	if err != nil {
		t.Fatalf("GetCaseFeedback: %v", err)
	}
	if !found {
		t.Fatal("expected found=true after a submission")
	}
	if row.ID != created.ID {
		t.Errorf("got feedback id %q, want %q", row.ID, created.ID)
	}
	if row.EmojiID != cfEmojiID || row.EmojiName != "Very Satisfied" {
		t.Errorf("unexpected emoji: id=%q name=%q", row.EmojiID, row.EmojiName)
	}
	if row.EmojiSelectedImage != "/assets/feedback/verySatisfiedSelected.svg" {
		t.Errorf("unexpected emoji selected image: %q", row.EmojiSelectedImage)
	}
	if len(row.ChipIDs) != 1 || row.ChipIDs[0] != cfChipID {
		t.Errorf("unexpected chip ids: %v", row.ChipIDs)
	}
	if row.CreatedBy != "CF Tester" {
		t.Errorf("got createdBy %q, want the resolved user display name", row.CreatedBy)
	}
	if row.AdditionalComment == nil || *row.AdditionalComment != comment {
		t.Errorf("got comment %v, want %q", row.AdditionalComment, comment)
	}

	// A second submission for the same case must be rejected, not silently
	// accepted or silently ignored -- work_item_feedback.work_item_id is
	// UNIQUE, one submission per case ever.
	_, err = repo.CreateCaseFeedback(ctx, cfCaseID, repository.CreateCaseFeedbackParams{
		EmojiID:           cfEmojiID,
		SubmittedByUserID: cfUserID,
		ActorEmail:        "cf-test-user@test.local",
	})
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("got %v, want *apierror.ConflictError on a second submission", err)
	}
}

// TestCaseFeedbackIntegration_RejectsSubmissionOnAnOpenCase is the
// regression guard for the actual business rule this form exists for: case
// feedback is a post-closure satisfaction survey (the portal only ever
// offers it once a case has closed), so a submission against a case that's
// still OPEN must be rejected, not silently accepted.
func TestCaseFeedbackIntegration_RejectsSubmissionOnAnOpenCase(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseFeedbackFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	_, err := repo.CreateCaseFeedback(ctx, cfOpenCaseID, repository.CreateCaseFeedbackParams{
		EmojiID:           cfEmojiID,
		ChipIDs:           []string{cfChipID},
		SubmittedByUserID: cfUserID,
		ActorEmail:        "cf-test-user@test.local",
	})
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("got %v, want *apierror.ConflictError for a submission against a still-open case", err)
	}

	// Confirm it genuinely never wrote anything -- not just that it
	// returned an error over some other, unrelated problem.
	_, found, getErr := repo.GetCaseFeedback(ctx, cfOpenCaseID)
	if getErr != nil {
		t.Fatalf("GetCaseFeedback: %v", getErr)
	}
	if found {
		t.Fatal("a rejected submission against an open case left a feedback row behind")
	}
}

// TestCaseFeedbackIntegration_RejectsSubmissionForAnOutOfScopeCase is the
// regression guard for "a caller may only submit feedback for a case they
// actually have access to". This is enforced entirely by RLS on
// CreateCaseFeedback's own existence/state query -- not by any explicit
// access check in case_service.go (see that method's own doc comment for
// why an earlier revision's extra GetCaseByID call was removed: it was
// redundant with, and far more expensive than, the protection this single
// query already provides for free). A service-layer test with a stub
// CaseRepository cannot exercise this at all -- RLS only exists in real
// Postgres -- so this is the one real test of that guarantee.
//
// Known environment limitation (same as other RLS integration tests in this
// package -- e.g. change_request_repo_integration_test.go's own note on
// this): a CASE_STATS_TEST_DSN that connects as a Postgres superuser (the
// local compose stack's own "postgres" role, confirmed live) bypasses RLS
// unconditionally, so this assertion only genuinely exercises the policy
// against a non-superuser connection role. Verified by hand against this
// package's own schema with a real non-superuser role and the exact session
// GUCs Scoped itself sets (app.is_internal/app.viewer_project_ids): an
// internal identity sees a no-project work_item row, a non-internal one
// with an empty viewer_project_ids does not -- confirming the mechanism
// this test exercises is real, even on a run where the DSN's own role
// can't surface a failure if it ever regressed.
func TestCaseFeedbackIntegration_RejectsSubmissionForAnOutOfScopeCase(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseFeedbackFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	// cfCaseID has no project_id at all (see seedCaseFeedbackFixture), so it
	// can never be in any non-Unrestricted caller's app.viewer_project_ids --
	// a stranger scope is rejected the same way a genuinely unregistered
	// contact would be.
	strangerCtx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{
		Unrestricted: false,
		ViewerEmail:  "cf-stranger@test.local",
	})
	_, err := repo.CreateCaseFeedback(strangerCtx, cfCaseID, repository.CreateCaseFeedbackParams{
		EmojiID:           cfEmojiID,
		ChipIDs:           []string{cfChipID},
		SubmittedByUserID: cfUserID,
		ActorEmail:        "cf-stranger@test.local",
	})
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("got %v, want *apierror.NotFoundError for a case outside the caller's scope", err)
	}

	// Confirm it genuinely never wrote anything -- checked as an internal
	// (Unrestricted) caller, since the stranger scope above couldn't see
	// this case well enough to even ask.
	systemCtx := repository.WithSystemIdentity(context.Background())
	_, found, getErr := repo.GetCaseFeedback(systemCtx, cfCaseID)
	if getErr != nil {
		t.Fatalf("GetCaseFeedback: %v", getErr)
	}
	if found {
		t.Fatal("a rejected out-of-scope submission left a feedback row behind")
	}
}

func TestCaseFeedbackIntegration_UnknownEmojiIsRejected(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseFeedbackFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	_, err := repo.CreateCaseFeedback(ctx, cfCaseID, repository.CreateCaseFeedbackParams{
		EmojiID:           "00000000-0000-0000-0000-000000000099",
		SubmittedByUserID: cfUserID,
		ActorEmail:        "cf-test-user@test.local",
	})
	var validationErr *apierror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("got %v, want *apierror.ValidationError for an unknown emojiId", err)
	}
}

func TestCaseFeedbackIntegration_ChipFromAnotherEmojiIsRejected(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseFeedbackFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	_, err := repo.CreateCaseFeedback(ctx, cfCaseID, repository.CreateCaseFeedbackParams{
		EmojiID:           cfEmojiID,
		ChipIDs:           []string{cfWrongOpt}, // belongs to cfWrongID, not cfEmojiID
		SubmittedByUserID: cfUserID,
		ActorEmail:        "cf-test-user@test.local",
	})
	var validationErr *apierror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("got %v, want *apierror.ValidationError for a chip belonging to a different emoji", err)
	}
}

func TestCaseFeedbackIntegration_ListFeedbackEmojisIncludesSeededFixture(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseFeedbackFixture(t, pool)
	repo := repository.NewReferenceDataRepository(pool)

	emojis, err := repo.ListFeedbackEmojis(context.Background())
	if err != nil {
		t.Fatalf("ListFeedbackEmojis: %v", err)
	}
	var found *repository.FeedbackEmojiRow
	for i := range emojis {
		if emojis[i].ID == cfEmojiID {
			found = &emojis[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("seeded emoji %s not found in ListFeedbackEmojis result (%d emojis returned)", cfEmojiID, len(emojis))
	}
	if found.Name != "Very Satisfied" || found.Value != "5" {
		t.Errorf("unexpected emoji: %+v", found)
	}
	if len(found.Chips) != 1 || found.Chips[0].ID != cfChipID || found.Chips[0].Name != "Excellent service" {
		t.Errorf("unexpected chips: %+v", found.Chips)
	}
}
