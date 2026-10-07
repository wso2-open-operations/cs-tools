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

// Tests for migration 0203_knowledge_article_history.sql, which must create the
// knowledge_article_history table where it is missing and must never touch it
// where it already exists: the table holds an article's version history, and
// the staging database has it (and its rows) from a hand-made copy, so a
// migration, or a ServiceNow to Postgres data load that replays migrations,
// must not drop, empty or recreate it.
//
// The first two tests read the SQL and the migrations folder and need no
// database. The other two run against a live PostgreSQL instance and reuse the
// knowledge_article fixture from kb_article_repo_integration_test.go (same DSN
// and skip-when-unset pattern, package repository_test):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run KBArticleHistory

package repository_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const kbHistoryMigrationPath = "../../migrations/0203_knowledge_article_history.sql"

func readKBHistoryMigration(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(kbHistoryMigrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	return string(raw)
}

// sqlWithoutComments drops whole-line "--" comments, which is all the
// migration files here use.
func sqlWithoutComments(sql string) string {
	var b strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// TestKBArticleHistoryMigrationOnlyCreatesWhatIsMissing pins the shape of the
// migration: every statement is a CREATE ... IF NOT EXISTS and nothing in it can
// remove, empty or change an existing table. A future edit that adds a DROP,
// TRUNCATE, DELETE, ALTER or a bare CREATE TABLE fails here.
func TestKBArticleHistoryMigrationOnlyCreatesWhatIsMissing(t *testing.T) {
	body := strings.ToUpper(sqlWithoutComments(readKBHistoryMigration(t)))

	// Statement forms, so the "ON DELETE CASCADE" on the foreign key is not a hit.
	for _, banned := range []string{`\bDROP\s`, `\bTRUNCATE\b`, `\bDELETE\s+FROM\b`, `\bALTER\s`, `\bUPDATE\s+\S+\s+SET\b`, `\bINSERT\s+INTO\b`} {
		if regexp.MustCompile(banned).MatchString(body) {
			t.Errorf("migration matches %q; it may only create what is missing", banned)
		}
	}

	creates := regexp.MustCompile(`CREATE\s+(?:UNIQUE\s+)?(TABLE|INDEX)\s+(IF\s+NOT\s+EXISTS)?`).FindAllStringSubmatch(body, -1)
	if len(creates) != 2 {
		t.Fatalf("found %d CREATE statements, want 2 (the table and its index)", len(creates))
	}
	for _, c := range creates {
		if c[2] == "" {
			t.Errorf("CREATE %s has no IF NOT EXISTS", c[1])
		}
	}
}

// TestKBArticleHistoryLegacyFilesAreGone: the old 000030 pair is replaced by
// 0203. Its .down.sql is DROP TABLE and `make migrate` runs every unrecorded
// *.sql file, .down.sql first by name, so it would empty the table.
func TestKBArticleHistoryLegacyFilesAreGone(t *testing.T) {
	matches, err := filepath.Glob("../../migrations/*knowledge_article_history*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 || filepath.Base(matches[0]) != "0203_knowledge_article_history.sql" {
		t.Fatalf("migrations for knowledge_article_history = %v, want only 0203_knowledge_article_history.sql", matches)
	}
}

// TestKBArticleHistoryMigrationCreatesMissingTable runs the migration where the
// table does not exist. It works in a throwaway schema inside a transaction that
// is rolled back, so nothing is left behind and the table in the real schema is
// never touched.
func TestKBArticleHistoryMigrationCreatesMissingTable(t *testing.T) {
	pool := caseStatsPool(t)
	seedKBArticleNullFixture(t, pool)
	ctx := context.Background()
	migration := readKBHistoryMigration(t)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// public stays on the path so the foreign keys resolve to the real
	// knowledge_article and "user" tables; the table is created in the first
	// schema on the path.
	if _, err := tx.Exec(ctx, `CREATE SCHEMA kb_history_mig_test`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL search_path = kb_history_mig_test, public`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	exists := func() bool {
		t.Helper()
		var found bool
		if err := tx.QueryRow(ctx, `SELECT to_regclass('kb_history_mig_test.knowledge_article_history') IS NOT NULL`).Scan(&found); err != nil {
			t.Fatalf("to_regclass: %v", err)
		}
		return found
	}
	if exists() {
		t.Fatal("table exists in the test schema before the migration ran")
	}

	if _, err := tx.Exec(ctx, migration); err != nil {
		t.Fatalf("run migration on a database without the table: %v", err)
	}
	if !exists() {
		t.Fatal("migration did not create knowledge_article_history")
	}

	var cols string
	if err := tx.QueryRow(ctx, `
		SELECT string_agg(column_name || ':' || data_type || ':' || is_nullable, ', ' ORDER BY ordinal_position)
		  FROM information_schema.columns
		 WHERE table_schema = 'kb_history_mig_test' AND table_name = 'knowledge_article_history'`).Scan(&cols); err != nil {
		t.Fatalf("read columns: %v", err)
	}
	wantCols := "id:uuid:NO, knowledge_article_id:uuid:NO, title:text:NO, body:text:NO, state:text:NO, changed_by:uuid:NO, created_on:timestamp with time zone:NO"
	if cols != wantCols {
		t.Errorf("columns =\n  %s\nwant\n  %s", cols, wantCols)
	}

	var indexes int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM pg_indexes
		 WHERE schemaname = 'kb_history_mig_test' AND tablename = 'knowledge_article_history'
		   AND indexname = 'idx_knowledge_article_history_article_id'`).Scan(&indexes); err != nil {
		t.Fatalf("read indexes: %v", err)
	}
	if indexes != 1 {
		t.Errorf("article_id index count = %d, want 1", indexes)
	}

	// A row written to the new table, then the migration run again twice.
	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_article_history (id, knowledge_article_id, title, body, state, changed_by)
		VALUES (gen_random_uuid(), $1, 'kept', 'kept body', 'draft', $2)`, kbNullFullID, kbNullUserID); err != nil {
		t.Fatalf("insert history row: %v", err)
	}
	for run := 1; run <= 2; run++ {
		if _, err := tx.Exec(ctx, migration); err != nil {
			t.Fatalf("re-run %d: %v", run, err)
		}
	}
	var kept int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM knowledge_article_history WHERE title = 'kept' AND body = 'kept body'`).Scan(&kept); err != nil {
		t.Fatalf("count: %v", err)
	}
	if kept != 1 {
		t.Errorf("history rows after re-running the migration = %d, want the 1 that was written", kept)
	}
}

// TestKBArticleHistoryMigrationKeepsExistingRows is the case that matters on a
// database that already has the table: real history written by
// UpdateKBArticleState (the code path the portal uses) is still there, unchanged,
// after the migration has run again over it, and still readable through
// ListKBArticleHistory.
func TestKBArticleHistoryMigrationKeepsExistingRows(t *testing.T) {
	pool := caseStatsPool(t)
	seedKBArticleNullFixture(t, pool)
	ctx := context.Background()
	migration := readKBHistoryMigration(t)

	// Make sure the table is there (a no-op where it already is). Rows for the
	// fixture articles are removed with them by the ON DELETE CASCADE.
	if _, err := pool.Exec(ctx, migration); err != nil {
		t.Fatalf("run migration: %v", err)
	}

	repo := repository.NewKBArticleRepository(pool)
	if _, err := repo.UpdateKBArticleState(ctx, kbNullDraftID, domain.UpdateKBArticleStateRequest{
		State:        domain.KBArticleStatePendingReview,
		UpdatedBy:    kbNullUserID,
		CurrentState: domain.KBArticleStateDraft,
	}); err != nil {
		t.Fatalf("UpdateKBArticleState: %v", err)
	}

	before, err := repo.ListKBArticleHistory(ctx, kbNullDraftID)
	if err != nil {
		t.Fatalf("ListKBArticleHistory: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("history entries after one transition = %d, want 1", len(before))
	}
	if got := before[0]; got.State != domain.KBArticleStatePendingReview || got.ChangedBy != kbNullUserID || got.Body != "old body" {
		t.Fatalf("history entry = %+v, want state pending_review, changedBy %s, body \"old body\"", got, kbNullUserID)
	}

	for run := 1; run <= 3; run++ {
		if _, err := pool.Exec(ctx, migration); err != nil {
			t.Fatalf("re-run %d: %v", run, err)
		}
	}

	after, err := repo.ListKBArticleHistory(ctx, kbNullDraftID)
	if err != nil {
		t.Fatalf("ListKBArticleHistory after re-running the migration: %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("history entries after re-running the migration = %d, want 1", len(after))
	}
	a, b := after[0], before[0]
	if a.ID != b.ID || a.KBArticleID != b.KBArticleID || a.Title != b.Title || a.Body != b.Body ||
		a.State != b.State || a.ChangedBy != b.ChangedBy || !a.CreatedOn.Equal(b.CreatedOn) {
		t.Errorf("history entry after re-running the migration = %+v, want exactly %+v", a, b)
	}
}
