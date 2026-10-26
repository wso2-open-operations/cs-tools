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

// Tests for migration 0251_knowledge_base_manager_tables.sql, which replaces
// the old-style 000020-000027 KB chain. The chain could not run on a database
// built from these migrations, and its only lasting effect was the two
// knowledge base manager tables, so 0251 must create those where they are
// missing and leave them exactly as they are where 000027 already made them.
//
// The first two tests read the SQL and the migrations folder and need no
// database. The other two run against a live PostgreSQL instance built from
// migrations/ (knowledge_base must exist), each in throwaway schemas inside a
// transaction that is rolled back:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run KBManagerMigration

package repository_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

const kbManagerMigrationPath = "../../migrations/0251_knowledge_base_manager_tables.sql"

// kbManager000027DDL is what 000027_replace_kb_tables_with_real_schema.up.sql
// created for the two manager tables, copied verbatim (the file is gone).
// Databases that recorded 000027 have exactly this.
const kbManager000027DDL = `
CREATE TABLE knowledge_base_manager_user (
    id UUID PRIMARY KEY,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,  -- LOCAL: no FK to "user" -- see file header
    created_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    CONSTRAINT uq_kb_manager_user UNIQUE (knowledge_base_id, user_id)
);

CREATE INDEX idx_kb_manager_user_knowledge_base_id ON knowledge_base_manager_user (knowledge_base_id);
CREATE INDEX idx_kb_manager_user_user_id ON knowledge_base_manager_user (user_id);

CREATE TABLE knowledge_base_manager_group (
    id UUID PRIMARY KEY,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    group_id UUID NOT NULL,
    group_name VARCHAR(255),
    created_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    CONSTRAINT uq_kb_manager_group UNIQUE (knowledge_base_id, group_id)
);

CREATE INDEX idx_kb_manager_group_knowledge_base_id ON knowledge_base_manager_group (knowledge_base_id);
`

func readKBManagerMigration(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(kbManagerMigrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	return string(raw)
}

// TestKBManagerMigrationOnlyCreatesWhatIsMissing pins the shape: two tables
// and three indexes, every one IF NOT EXISTS, and nothing that can remove or
// change what a database that ran 000027 already has.
func TestKBManagerMigrationOnlyCreatesWhatIsMissing(t *testing.T) {
	body := strings.ToUpper(sqlWithoutComments(readKBManagerMigration(t)))

	// Statement forms, so the "ON DELETE CASCADE" on the foreign keys is not a hit.
	for _, banned := range []string{`\bDROP\s`, `\bTRUNCATE\b`, `\bDELETE\s+FROM\b`, `\bALTER\s`, `\bUPDATE\s+\S+\s+SET\b`, `\bINSERT\s+INTO\b`, `\bBEGIN\b`, `\bCOMMIT\b`} {
		if regexp.MustCompile(banned).MatchString(body) {
			t.Errorf("migration matches %q; it may only create what is missing", banned)
		}
	}

	creates := regexp.MustCompile(`CREATE\s+(?:UNIQUE\s+)?(TABLE|INDEX)\s+(IF\s+NOT\s+EXISTS\s+)?(\w+)`).FindAllStringSubmatch(body, -1)
	var names []string
	for _, c := range creates {
		if c[2] == "" {
			t.Errorf("CREATE %s %s has no IF NOT EXISTS", c[1], c[3])
		}
		names = append(names, c[1]+" "+strings.ToLower(c[3]))
	}
	want := []string{
		"TABLE knowledge_base_manager_user",
		"INDEX idx_kb_manager_user_knowledge_base_id",
		"INDEX idx_kb_manager_user_user_id",
		"TABLE knowledge_base_manager_group",
		"INDEX idx_kb_manager_group_knowledge_base_id",
	}
	if strings.Join(names, ", ") != strings.Join(want, ", ") {
		t.Errorf("CREATE statements = %v, want %v", names, want)
	}
}

// TestKBLegacyStragglersAreGone: the 000020-000027 chain is replaced by 0251
// and must not come back. 000020 references products, users and cases, which
// this schema has never had, so any of these files stops a fresh run; and
// 0251 is the only file that may define the manager tables.
func TestKBLegacyStragglersAreGone(t *testing.T) {
	legacy, err := filepath.Glob("../../migrations/00002[0-7]_*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(legacy) != 0 {
		t.Errorf("legacy KB migrations still present: %v", legacy)
	}

	all, err := filepath.Glob("../../migrations/*.sql")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range all {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if strings.Contains(string(raw), "knowledge_base_manager_") && filepath.Base(f) != filepath.Base(kbManagerMigrationPath) {
			t.Errorf("%s mentions the knowledge base manager tables; only %s may define them", filepath.Base(f), filepath.Base(kbManagerMigrationPath))
		}
	}
}

// kbManagerCatalog describes the manager tables in one schema: columns,
// constraints and indexes, with the schema name taken out so two schemas can
// be compared.
func kbManagerCatalog(t *testing.T, ctx context.Context, tx pgx.Tx, schema string) string {
	t.Helper()
	var out string
	if err := tx.QueryRow(ctx, `
		SELECT coalesce(string_agg(item, E'\n' ORDER BY item), '') FROM (
			SELECT 'col ' || table_name || '.' || column_name || ' ' || data_type || ' ' || is_nullable
			       || ' len=' || coalesce(character_maximum_length::text, '-') || ' default=' || coalesce(column_default, '-') AS item
			  FROM information_schema.columns
			 WHERE table_schema = $1 AND table_name LIKE 'knowledge_base_manager_%'
			UNION ALL
			SELECT 'con ' || cl.relname || ' ' || con.conname || ' ' || pg_get_constraintdef(con.oid)
			  FROM pg_constraint con
			  JOIN pg_class cl ON cl.oid = con.conrelid
			  JOIN pg_namespace n ON n.oid = cl.relnamespace
			 WHERE n.nspname = $1 AND cl.relname LIKE 'knowledge_base_manager_%'
			UNION ALL
			SELECT 'idx ' || tablename || ' ' || indexname || ' ' || replace(indexdef, $1 || '.', '')
			  FROM pg_indexes
			 WHERE schemaname = $1 AND tablename LIKE 'knowledge_base_manager_%'
		) items`, schema).Scan(&out); err != nil {
		t.Fatalf("read catalog of %s: %v", schema, err)
	}
	return out
}

// TestKBManagerMigrationCreatesTheSameTablesAs000027 runs 0251 where the
// tables are missing (a fresh database) and compares what it made with what
// 000027 made on the databases that ran it: same columns, constraints and
// indexes, so the two kinds of database cannot drift apart.
func TestKBManagerMigrationCreatesTheSameTablesAs000027(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()
	migration := readKBManagerMigration(t)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// public stays on the path so knowledge_base resolves to the real table;
	// the manager tables are created in the first schema on the path, whether
	// or not public already has them.
	for _, schema := range []string{"kb_manager_mig_fresh", "kb_manager_mig_old"} {
		if _, err := tx.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
			t.Fatalf("create schema %s: %v", schema, err)
		}
	}

	if _, err := tx.Exec(ctx, `SET LOCAL search_path = kb_manager_mig_fresh, public`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_fresh"); got != "" {
		t.Fatalf("test schema already has manager tables:\n%s", got)
	}
	if _, err := tx.Exec(ctx, migration); err != nil {
		t.Fatalf("run migration where the tables are missing: %v", err)
	}
	fresh := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_fresh")
	if !strings.Contains(fresh, "idx knowledge_base_manager_group idx_kb_manager_group_knowledge_base_id") {
		t.Fatalf("migration did not create the manager tables and indexes:\n%s", fresh)
	}

	if _, err := tx.Exec(ctx, `SET LOCAL search_path = kb_manager_mig_old, public`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if _, err := tx.Exec(ctx, kbManager000027DDL); err != nil {
		t.Fatalf("create the tables as 000027 did: %v", err)
	}
	old := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_old")

	if fresh != old {
		t.Errorf("0251 and 000027 make different tables.\n0251:\n%s\n\n000027:\n%s", fresh, old)
	}
}

// TestKBManagerMigrationLeavesExistingTablesAlone is a database that recorded
// 000020-000027: the manager tables exist, with rows. Running 0251 over them,
// more than once, changes neither the tables nor the rows.
func TestKBManagerMigrationLeavesExistingTablesAlone(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()
	migration := readKBManagerMigration(t)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `CREATE SCHEMA kb_manager_mig_existing`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL search_path = kb_manager_mig_existing, public`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if _, err := tx.Exec(ctx, kbManager000027DDL); err != nil {
		t.Fatalf("create the tables as 000027 did: %v", err)
	}

	const baseID = "3a111111-0000-0000-0000-0000000000f1"
	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_base (id, created_on, updated_on, created_by, updated_by, title, active)
		VALUES ($1, now(), now(), 'kb-manager-mig-test', 'kb-manager-mig-test', 'KB manager migration test', true)`, baseID); err != nil {
		t.Fatalf("insert knowledge base: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_base_manager_user (id, knowledge_base_id, user_id, created_on, created_by)
		VALUES (gen_random_uuid(), $1, gen_random_uuid(), now(), 'kb-manager-mig-test')`, baseID); err != nil {
		t.Fatalf("insert manager user: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_base_manager_group (id, knowledge_base_id, group_id, group_name, created_on, created_by)
		VALUES (gen_random_uuid(), $1, gen_random_uuid(), 'KB managers', now(), 'kb-manager-mig-test')`, baseID); err != nil {
		t.Fatalf("insert manager group: %v", err)
	}

	rows := func() string {
		t.Helper()
		var out string
		if err := tx.QueryRow(ctx, `
			SELECT (SELECT coalesce(string_agg(row_to_json(u)::TEXT, ',' ORDER BY u.id), '') FROM knowledge_base_manager_user u)
			    || '|' ||
			       (SELECT coalesce(string_agg(row_to_json(g)::TEXT, ',' ORDER BY g.id), '') FROM knowledge_base_manager_group g)`).Scan(&out); err != nil {
			t.Fatalf("read rows: %v", err)
		}
		return out
	}
	catalogBefore := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_existing")
	rowsBefore := rows()

	for run := 1; run <= 2; run++ {
		if _, err := tx.Exec(ctx, migration); err != nil {
			t.Fatalf("run %d over the existing tables: %v", run, err)
		}
	}

	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_existing"); got != catalogBefore {
		t.Errorf("tables changed.\nbefore:\n%s\n\nafter:\n%s", catalogBefore, got)
	}
	if got := rows(); got != rowsBefore {
		t.Errorf("rows changed.\nbefore: %s\nafter:  %s", rowsBefore, got)
	}
}
