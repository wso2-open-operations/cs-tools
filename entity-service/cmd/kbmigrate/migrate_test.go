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

package main

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestRunMigration_EndToEnd exercises the full orchestration
// (runMigration -> migrateOneChain -> migrateOneHistoryRow) against a real,
// throwaway local Postgres database carrying the actual 000001-000030
// migrated schema (kbmigrate_verify in this sandbox -- see the return notes
// for exactly how it was built, since migrations 000001-000019 as committed
// don't apply cleanly from scratch on their own, a pre-existing issue
// unrelated to this backfill). A fake SN server (reusing fakeSNServer from
// snclient_test.go) stands in for ServiceNow, and the real
// realUserResolver/realCaseResolver hit the same two real Postgres
// databases resolve_test.go already exercises directly.
//
// Skips (not fails) if kbmigrate_verify / kbmigrate_test_csmsync aren't
// reachable -- see connectOrSkip and dsnFromEnv in resolve_test.go.
const defaultTestVerifyDSN = "postgres://localhost:5432/kbmigrate_verify?sslmode=disable"

func TestRunMigration_EndToEnd(t *testing.T) {
	entityPool := connectOrSkip(t, dsnFromEnv("KBMIGRATE_TEST_VERIFY_DSN", defaultTestVerifyDSN))
	defer entityPool.Close()
	csmSyncPool := connectOrSkip(t, dsnFromEnv("KBMIGRATE_TEST_CSMSYNC_DSN", defaultTestCSMSyncDSN))
	defer csmSyncPool.Close()

	ctx := context.Background()
	resetVerifyDB(t, ctx, entityPool)
	resetCSMSyncUsers(t, ctx, csmSyncPool)

	// -- Users: an author who resolves, a reviser who resolves, and a
	// manager sys_id that is deliberately never seeded anywhere (unresolvable).
	authorSysID := sysIDForTest(0xA1)
	revisorSysID := sysIDForTest(0xA2)
	unresolvableSysID := sysIDForTest(0xDEAD)
	seedResolvableUser(t, ctx, csmSyncPool, entityPool, authorSysID, "author@example.com", "author-entity-id")
	seedResolvableUser(t, ctx, csmSyncPool, entityPool, revisorSysID, "revisor@example.com", "revisor-entity-id")

	// -- A pre-existing case for source_case_id to resolve against, and a
	// case sys_id that has no corresponding row (must resolve to NULL, not error).
	existingCaseSysID := sysIDForTest(0xCA5E)
	existingCaseUUID := sysIDToUUID(existingCaseSysID)
	mustExec(t, ctx, entityPool, `INSERT INTO cases (id) VALUES ($1)`, existingCaseUUID)
	missingCaseSysID := sysIDForTest(0xCA5EDEAD)

	kbSysID := sysIDForTest(0xB1)

	// One article per collapsed state, covering every entry in
	// workflowStateCollapse at least once (draft, review, scheduled_publish,
	// published, pending_retirement, retired, outdated).
	draftSysID := sysIDForTest(1)
	reviewSysID := sysIDForTest(2)
	scheduledPublishSysID := sysIDForTest(3)
	publishedSysID := sysIDForTest(4)
	pendingRetirementSysID := sysIDForTest(5)
	retiredSysID := sysIDForTest(6)
	outdatedSysID := sysIDForTest(7)

	// A version chain: v1 (root) -> v2 -> latest (v3), so the walk produces
	// two kb_article_history rows under one kb_articles row.
	chainV1SysID := sysIDForTest(10)
	chainV2SysID := sysIDForTest(11)
	chainLatestSysID := sysIDForTest(12)

	// An article whose author cannot be resolved -- the whole article must
	// be skipped, never inserted.
	unresolvableAuthorArticleSysID := sysIDForTest(20)

	knownTime := "2025-01-01 00:00:00"
	laterTime := "2025-06-01 00:00:00"

	rowsByType := map[string][]map[string]string{
		"kb_knowledge_base": {
			{
				"sys_id": kbSysID, "title": "Integration Test KB", "active": "true",
				"kb_managers":    authorSysID + "," + unresolvableSysID,
				"sys_created_on": knownTime, "sys_updated_on": laterTime,
			},
		},
		"kb_knowledge": {
			knowledgeRow(draftSysID, kbSysID, authorSysID, "", "draft", "", "not ready", knownTime, laterTime, existingCaseSysID),
			knowledgeRow(reviewSysID, kbSysID, authorSysID, "", "review", "", "", knownTime, laterTime, ""),
			knowledgeRow(scheduledPublishSysID, kbSysID, authorSysID, "", "scheduled_publish", "", "", knownTime, laterTime, ""),
			knowledgeRow(publishedSysID, kbSysID, authorSysID, revisorSysID, "published", "", "", knownTime, laterTime, missingCaseSysID),
			knowledgeRow(pendingRetirementSysID, kbSysID, authorSysID, "", "pending_retirement", "", "", knownTime, laterTime, ""),
			knowledgeRow(retiredSysID, kbSysID, authorSysID, "", "retired", "", "", knownTime, laterTime, ""),
			knowledgeRow(outdatedSysID, kbSysID, authorSysID, "", "outdated", "", "", knownTime, laterTime, ""),
			knowledgeRow(chainV1SysID, kbSysID, authorSysID, "", "retired", "", "", knownTime, laterTime, ""),
			knowledgeRowWithBase(chainV2SysID, kbSysID, authorSysID, revisorSysID, "retired", chainV1SysID, knownTime, laterTime),
			knowledgeRowLatestWithBase(chainLatestSysID, kbSysID, authorSysID, revisorSysID, "published", chainV2SysID, knownTime, laterTime),
			knowledgeRow(unresolvableAuthorArticleSysID, kbSysID, unresolvableSysID, "", "draft", "", "", knownTime, laterTime, ""),
		},
	}
	// Mark the single (non-chain) rows as latest=true; chain rows other
	// than chainLatestSysID must stay latest=false so runMigration only
	// seeds a chain from the true head.
	for _, table := range []string{"kb_knowledge"} {
		for _, row := range rowsByType[table] {
			if row["sys_id"] != chainV1SysID && row["sys_id"] != chainV2SysID {
				row["latest"] = "true"
			}
		}
	}

	fake := newFakeSNServer(rowsByType)
	sn := newSNClient("https://wso2sndev.example.invalid", "id", "secret", fake)
	store := newKBStore(entityPool)
	users := newRealUserResolver(csmSyncPool, entityPool)
	cases := newRealCaseResolver(entityPool)

	sum, err := runMigration(ctx, sn, store, users, cases)
	if err != nil {
		t.Fatalf("runMigration: %v", err)
	}

	// 7 single-row articles + 1 chain head = 8 successfully inserted; 1 skipped (unresolvable author).
	if sum.ArticlesInserted != 8 {
		t.Errorf("ArticlesInserted = %d, want 8 (sum=%+v)", sum.ArticlesInserted, sum)
	}
	if len(sum.ArticlesSkipped) != 1 {
		t.Errorf("ArticlesSkipped = %d, want 1 (the unresolvable-author article): %+v", len(sum.ArticlesSkipped), sum.ArticlesSkipped)
	}
	if sum.HistoryRowsInserted != 2 {
		t.Errorf("HistoryRowsInserted = %d, want 2 (chainV1, chainV2)", sum.HistoryRowsInserted)
	}
	if sum.KnowledgeBasesCreated != 1 {
		t.Errorf("KnowledgeBasesCreated = %d, want 1", sum.KnowledgeBasesCreated)
	}
	if sum.KBManagersInserted != 1 {
		t.Errorf("KBManagersInserted = %d, want 1 (only the resolvable manager)", sum.KBManagersInserted)
	}
	if len(sum.KBManagersSkipped) != 1 {
		t.Errorf("KBManagersSkipped = %d, want 1 (the unresolvable manager)", len(sum.KBManagersSkipped))
	}

	// Verify every CHECK constraint / trigger actually passed for every
	// collapsed state by reading the rows back and confirming Postgres's
	// own view of each state's timestamps -- not just "insert didn't error".
	assertArticleState(t, ctx, entityPool, draftSysID, stateDraft, false, false, false, true)
	assertArticleState(t, ctx, entityPool, reviewSysID, statePendingReview, true, false, false, false)
	assertArticleState(t, ctx, entityPool, scheduledPublishSysID, statePublished, true, true, false, false)
	assertArticleState(t, ctx, entityPool, publishedSysID, statePublished, true, true, false, false)
	assertArticleState(t, ctx, entityPool, pendingRetirementSysID, stateRetired, true, false, true, false)
	assertArticleState(t, ctx, entityPool, retiredSysID, stateRetired, true, false, true, false)
	assertArticleState(t, ctx, entityPool, outdatedSysID, stateRetired, true, false, true, false)

	// source_case_id resolution: existing case populates it, a missing one
	// (or absent u_case_id) leaves it NULL rather than erroring the article.
	assertSourceCaseID(t, ctx, entityPool, draftSysID, &existingCaseUUID)
	assertSourceCaseID(t, ctx, entityPool, publishedSysID, nil) // missingCaseSysID: not found -> NULL

	// updated_by: only set where revised_by was non-empty.
	assertUpdatedBy(t, ctx, entityPool, draftSysID, false)
	assertUpdatedBy(t, ctx, entityPool, publishedSysID, true)

	if err := entityPool.QueryRow(ctx, `SELECT COUNT(*) FROM kb_articles WHERE source_sys_id = $1`, unresolvableAuthorArticleSysID).Scan(new(int)); err != nil {
		t.Fatalf("count check: %v", err)
	}
	var skippedCount int
	if err := entityPool.QueryRow(ctx, `SELECT COUNT(*) FROM kb_articles WHERE source_sys_id = $1`, unresolvableAuthorArticleSysID).Scan(&skippedCount); err != nil {
		t.Fatalf("count skipped article: %v", err)
	}
	if skippedCount != 0 {
		t.Errorf("expected the unresolvable-author article to NOT be inserted, found %d rows", skippedCount)
	}

	// -- Idempotency: rerun the exact same migration and confirm nothing
	// new is inserted, everything is reported as already-migrated/skipped.
	sum2, err := runMigration(ctx, sn, store, users, cases)
	if err != nil {
		t.Fatalf("runMigration (2nd run): %v", err)
	}
	if sum2.ArticlesInserted != 0 {
		t.Errorf("2nd run ArticlesInserted = %d, want 0 (idempotent rerun)", sum2.ArticlesInserted)
	}
	if sum2.HistoryRowsInserted != 0 {
		t.Errorf("2nd run HistoryRowsInserted = %d, want 0 (idempotent rerun)", sum2.HistoryRowsInserted)
	}
	if sum2.KnowledgeBasesCreated != 0 || sum2.KnowledgeBasesReused != 1 {
		t.Errorf("2nd run knowledge base dedupe-by-name failed: created=%d reused=%d", sum2.KnowledgeBasesCreated, sum2.KnowledgeBasesReused)
	}
	if sum2.KBManagersInserted != 0 {
		t.Errorf("2nd run KBManagersInserted = %d, want 0 (ON CONFLICT DO NOTHING)", sum2.KBManagersInserted)
	}
}

// TestKBManagers_GroupTypedRowSchemaCheck confirms a group-typed kb_managers
// row (user_id NULL, group_id set) -- the shape 000029 introduced but that
// kbmigrate itself never produces (see this tool's return notes: SN
// kb_managers is a user-only glide_list, so this backfill never populates
// group_id) -- still satisfies the real chk_kb_managers_user_xor_group CHECK
// constraint and uq_kb_managers_kb_group UNIQUE constraint end to end.
func TestKBManagers_GroupTypedRowSchemaCheck(t *testing.T) {
	entityPool := connectOrSkip(t, dsnFromEnv("KBMIGRATE_TEST_VERIFY_DSN", defaultTestVerifyDSN))
	defer entityPool.Close()
	ctx := context.Background()

	var kbID string
	err := entityPool.QueryRow(ctx,
		`INSERT INTO knowledge_bases (name, is_active) VALUES ($1, true) RETURNING id`,
		"Group Manager Test KB",
	).Scan(&kbID)
	if err != nil {
		t.Fatalf("insert knowledge_base: %v", err)
	}

	if _, err := entityPool.Exec(ctx,
		`INSERT INTO kb_managers (knowledge_base_id, group_id) VALUES ($1, $2)`,
		kbID, "some-external-group-id",
	); err != nil {
		t.Fatalf("insert group-typed kb_manager: %v", err)
	}

	// Both user_id and group_id set must be rejected by chk_kb_managers_user_xor_group.
	if _, err := entityPool.Exec(ctx,
		`INSERT INTO kb_managers (knowledge_base_id, user_id, group_id) VALUES ($1, 'some-user', $2)`,
		kbID, "another-group-id",
	); err == nil {
		t.Fatal("expected chk_kb_managers_user_xor_group to reject a row with both user_id and group_id set")
	}
}

func resetVerifyDB(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	mustExec(t, ctx, pool, `TRUNCATE kb_article_history, kb_articles, kb_managers, knowledge_bases, cases, users CASCADE`)
}

func resetCSMSyncUsers(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	mustExec(t, ctx, pool, `TRUNCATE "user" CASCADE`)
}

func seedResolvableUser(t *testing.T, ctx context.Context, csmSyncPool, entityPool *pgxpool.Pool, snSysID, email, entityUserID string) {
	t.Helper()
	uuid := sysIDToUUID(snSysID)
	mustExec(t, ctx, csmSyncPool, `INSERT INTO "user" (id, email, user_name) VALUES ($1, $2, $3)`, uuid, email, entityUserID)
	mustExec(t, ctx, entityPool, `INSERT INTO users (id, email) VALUES ($1, $2)`, entityUserID, email)
}

// knowledgeRow builds a fake kb_knowledge Table API row for a single,
// non-chained article (base_version empty). latest defaults to "false" and
// is overwritten by the caller for head-of-chain rows.
func knowledgeRow(sysID, kb, author, revisedBy, workflowState, generatedBy, rejectReason, createdOn, updatedOn, caseSysID string) map[string]string {
	return map[string]string{
		"sys_id": sysID, "short_description": "Article " + sysID, "text": "<p>body " + sysID + "</p>",
		"workflow_state": workflowState, "author": author, "revised_by": revisedBy,
		"kb_knowledge_base": kb, "u_case_id": caseSysID, "u_reject_reason": rejectReason,
		"generated_with_now_assist": "false", "u_generated_by": generatedBy,
		"helpful_count": "0", "rating": "", "use_count": "0", "sys_view_count": "0",
		"base_version": "", "latest": "false",
		"sys_created_on": createdOn, "sys_updated_on": updatedOn,
	}
}

func knowledgeRowWithBase(sysID, kb, author, revisedBy, workflowState, baseVersion, createdOn, updatedOn string) map[string]string {
	row := knowledgeRow(sysID, kb, author, revisedBy, workflowState, "", "", createdOn, updatedOn, "")
	row["base_version"] = baseVersion
	return row
}

func knowledgeRowLatestWithBase(sysID, kb, author, revisedBy, workflowState, baseVersion, createdOn, updatedOn string) map[string]string {
	row := knowledgeRowWithBase(sysID, kb, author, revisedBy, workflowState, baseVersion, createdOn, updatedOn)
	row["latest"] = "true"
	return row
}

func assertArticleState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sourceSysID, wantState string, wantSubmitted, wantPublished, wantRetired, wantRejection bool) {
	t.Helper()
	var state string
	var submittedAt, publishedAt, retiredAt *string
	var rejectionComment *string
	err := pool.QueryRow(ctx,
		`SELECT state::text, submitted_at::text, published_at::text, retired_at::text, rejection_comment
		 FROM kb_articles WHERE source_sys_id = $1`, sourceSysID,
	).Scan(&state, &submittedAt, &publishedAt, &retiredAt, &rejectionComment)
	if err != nil {
		t.Fatalf("read back article %s: %v", sourceSysID, err)
	}
	if state != wantState {
		t.Errorf("article %s: state = %q, want %q", sourceSysID, state, wantState)
	}
	if (submittedAt != nil) != wantSubmitted {
		t.Errorf("article %s: submitted_at set = %v, want %v", sourceSysID, submittedAt != nil, wantSubmitted)
	}
	if (publishedAt != nil) != wantPublished {
		t.Errorf("article %s: published_at set = %v, want %v", sourceSysID, publishedAt != nil, wantPublished)
	}
	if (retiredAt != nil) != wantRetired {
		t.Errorf("article %s: retired_at set = %v, want %v", sourceSysID, retiredAt != nil, wantRetired)
	}
	if (rejectionComment != nil) != wantRejection {
		t.Errorf("article %s: rejection_comment set = %v, want %v", sourceSysID, rejectionComment != nil, wantRejection)
	}
}

func assertSourceCaseID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sourceSysID string, want *string) {
	t.Helper()
	var got *string
	if err := pool.QueryRow(ctx, `SELECT source_case_id::text FROM kb_articles WHERE source_sys_id = $1`, sourceSysID).Scan(&got); err != nil {
		t.Fatalf("read back source_case_id for %s: %v", sourceSysID, err)
	}
	switch {
	case want == nil && got != nil:
		t.Errorf("article %s: source_case_id = %v, want NULL", sourceSysID, *got)
	case want != nil && (got == nil || *got != *want):
		t.Errorf("article %s: source_case_id = %v, want %v", sourceSysID, got, *want)
	}
}

func assertUpdatedBy(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sourceSysID string, wantSet bool) {
	t.Helper()
	var got *string
	if err := pool.QueryRow(ctx, `SELECT updated_by FROM kb_articles WHERE source_sys_id = $1`, sourceSysID).Scan(&got); err != nil {
		t.Fatalf("read back updated_by for %s: %v", sourceSysID, err)
	}
	if (got != nil) != wantSet {
		t.Errorf("article %s: updated_by set = %v, want %v", sourceSysID, got != nil, wantSet)
	}
}
