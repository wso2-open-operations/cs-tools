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

// Tests for migration 0262_knowledge_article_user_and_case_fks.sql, which gives
// knowledge_article the foreign keys on author_id, revised_by_id and
// source_case_id that 0044 declares and the removed 000027 left out. Where
// 000027 made the table first, 0044 (CREATE TABLE IF NOT EXISTS) never added
// them.
//
// The first test reads the SQL and needs no database. The others run against
// a live PostgreSQL instance built from migrations/ ("user" and work_item must
// exist), in throwaway schemas inside a transaction that is rolled back:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run KnowledgeArticleFKMigration

package repository_test

import (
	"context"
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	kaFKMigrationPath = "../../migrations/0262_knowledge_article_user_and_case_fks.sql"
	kbTables0044Path  = "../../migrations/0044_kb_tables.sql"
)

// kbTables000027DDL is what 000027_replace_kb_tables_with_real_schema.up.sql
// created for knowledge_base and knowledge_article, copied verbatim (the file
// is gone; git show 67f35215a:entity-service/migrations/000027_...up.sql).
const kbTables000027DDL = `
CREATE TABLE knowledge_base (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    active BOOLEAN
);

CREATE TABLE knowledge_article (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    number VARCHAR(100),
    title VARCHAR(255) NOT NULL,
    body TEXT,
    state VARCHAR(40),
    knowledge_base_id UUID REFERENCES knowledge_base(id) ON DELETE SET NULL,
    author_id UUID,       -- LOCAL: no FK to "user" -- see file header
    revised_by_id UUID,   -- LOCAL: no FK to "user" -- see file header
    source_case_id UUID,  -- LOCAL: no FK to work_item -- see file header
    base_version_id UUID REFERENCES knowledge_article(id) ON DELETE SET NULL,
    latest BOOLEAN,
    rejection_comment TEXT,
    generated_with_ai BOOLEAN,
    ai_generated_by VARCHAR(100),
    helpful_count INTEGER,
    rating NUMERIC,
    use_count INTEGER,
    view_count INTEGER,
    published_on TIMESTAMPTZ,
    retired_on TIMESTAMPTZ,
    scheduled_publish_on TIMESTAMPTZ
);

CREATE INDEX idx_knowledge_article_knowledge_base_id ON knowledge_article (knowledge_base_id);
CREATE INDEX idx_knowledge_article_author_id ON knowledge_article (author_id);
CREATE INDEX idx_knowledge_article_revised_by_id ON knowledge_article (revised_by_id);
CREATE INDEX idx_knowledge_article_source_case_id ON knowledge_article (source_case_id);
CREATE INDEX idx_knowledge_article_base_version_id ON knowledge_article (base_version_id);
`

func readMigration(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// kaFKLine is the catalog line for one of 0262's constraints, validated.
func kaFKLine(column, target string) string {
	return "con knowledge_article_" + column + "_fkey FOREIGN KEY (" + column + ") REFERENCES " + target + "(id) ON DELETE SET NULL"
}

// TestKnowledgeArticleFKMigrationShape: 0262 adds exactly the foreign keys
// 0044 puts on columns that 000027 left plain, under 0044's constraint names,
// NOT VALID and then validated, and nothing in it can remove or rewrite what a
// database already has.
func TestKnowledgeArticleFKMigrationShape(t *testing.T) {
	body := strings.ToUpper(sqlWithoutComments(readMigration(t, kaFKMigrationPath)))

	for _, banned := range []string{`\bDROP\s`, `\bTRUNCATE\b`, `\bDELETE\s+FROM\b`, `\bUPDATE\s+\S+\s+SET\b`, `\bINSERT\s+INTO\b`, `\bCREATE\s`, `\bCOMMIT\b`, `\bROLLBACK\b`, `\bSTART\s+TRANSACTION\b`, `\bBEGIN\s*(;|TRANSACTION\b|WORK\b|ISOLATION\b)`} {
		if regexp.MustCompile(banned).MatchString(body) {
			t.Errorf("migration matches %q; it may only add the missing foreign keys", banned)
		}
	}
	for _, want := range []string{
		`'ALTER TABLE KNOWLEDGE_ARTICLE ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %S(ID) ON DELETE SET NULL NOT VALID'`,
		`'ALTER TABLE KNOWLEDGE_ARTICLE VALIDATE CONSTRAINT %I'`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("migration lacks %s", want)
		}
	}

	// Columns 0044 references with ON DELETE SET NULL to a table other than the
	// two KB tables themselves, which 000027 already referenced.
	declared := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^\s*(\w+)\s+UUID\s+REFERENCES\s+("?\w+"?)\(id\)\s+ON DELETE SET NULL`).FindAllStringSubmatch(readMigration(t, kbTables0044Path), -1) {
		if m[2] != "knowledge_base" && m[2] != "knowledge_article" {
			declared[m[1]] = m[2]
		}
	}
	added := map[string]string{}
	for _, m := range regexp.MustCompile(`\('(\w+)',\s*'("?\w+"?)'::regclass,\s*'(\w+)'\)`).FindAllStringSubmatch(sqlWithoutComments(readMigration(t, kaFKMigrationPath)), -1) {
		added[m[1]] = m[2]
		if want := "knowledge_article_" + m[1] + "_fkey"; m[3] != want {
			t.Errorf("constraint for %s is named %s, want 0044's %s", m[1], m[3], want)
		}
	}
	if len(declared) != 3 || !equalStringMaps(declared, added) {
		t.Errorf("0262 adds %v, 0044 declares %v", added, declared)
	}
}

func equalStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// tableCatalog describes one table in one schema: columns, constraints (with
// NOT VALID when unvalidated) and indexes, with the schema name taken out so
// two schemas can be compared.
func tableCatalog(t *testing.T, ctx context.Context, tx pgx.Tx, schema, table string) string {
	t.Helper()
	var out string
	if err := tx.QueryRow(ctx, `
		SELECT coalesce(string_agg(item, E'\n' ORDER BY item), '') FROM (
			SELECT 'col ' || column_name || ' ' || data_type || ' ' || is_nullable
			       || ' len=' || coalesce(character_maximum_length::text, '-') || ' default=' || coalesce(column_default, '-') AS item
			  FROM information_schema.columns
			 WHERE table_schema = $1 AND table_name = $2
			UNION ALL
			SELECT 'con ' || con.conname || ' ' || pg_get_constraintdef(con.oid)
			  FROM pg_constraint con
			  JOIN pg_class cl ON cl.oid = con.conrelid
			  JOIN pg_namespace n ON n.oid = cl.relnamespace
			 WHERE n.nspname = $1 AND cl.relname = $2
			UNION ALL
			SELECT 'idx ' || indexname || ' ' || replace(indexdef, $1 || '.', '')
			  FROM pg_indexes
			 WHERE schemaname = $1 AND tablename = $2
		) items`, schema, table).Scan(&out); err != nil {
		t.Fatalf("read catalog of %s.%s: %v", schema, table, err)
	}
	return out
}

// kaRows reads every knowledge_article row on the search path.
func kaRows(t *testing.T, ctx context.Context, tx pgx.Tx) string {
	t.Helper()
	var out string
	if err := tx.QueryRow(ctx, `SELECT coalesce(string_agg(row_to_json(a)::TEXT, ',' ORDER BY a.id), '') FROM knowledge_article a`).Scan(&out); err != nil {
		t.Fatalf("read rows: %v", err)
	}
	return out
}

// kaFreshCatalog is knowledge_article as 0044 makes it, built in its own
// schema of tx.
func kaFreshCatalog(t *testing.T, ctx context.Context, tx pgx.Tx, schema string) string {
	t.Helper()
	kbManagerUseSchema(t, ctx, tx, schema)
	kbManagerExec(t, ctx, tx, "run 0044", readMigration(t, kbTables0044Path))
	return tableCatalog(t, ctx, tx, schema, "knowledge_article")
}

const (
	kaTestUserID    = "3a111111-0000-0000-0000-0000000000a1"
	kaTestArticleID = "3a111111-0000-0000-0000-0000000000a2"
	kaOrphanUserID  = "3a111111-0000-0000-0000-0000000000a8"
	kaOrphanCaseID  = "3a111111-0000-0000-0000-0000000000a9"
)

// kaSeed adds a user and an article that references it, to tables made by
// 000027 on the search path.
func kaSeed(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	kbManagerExec(t, ctx, tx, "insert user", `
		INSERT INTO "user" (id, created_on, updated_on, user_name, email)
		VALUES ($1, now(), now(), 'ka-fk-mig-test', 'ka-fk-mig-test@example.com')`, kaTestUserID)
	kbManagerExec(t, ctx, tx, "insert article", `
		INSERT INTO knowledge_article (id, created_on, updated_on, created_by, updated_by, title, state, author_id, revised_by_id, latest)
		VALUES ($1, now(), now(), 'ka-fk-mig-test', 'ka-fk-mig-test', 'Article', 'draft', $2, $2, true)`, kaTestArticleID, kaTestUserID)
}

// TestKnowledgeArticleFKMigrationConverges: where 0044 made knowledge_article
// 0262 changes nothing; where 000027 made it, 0262 brings it to exactly what
// 0044 makes, run once or twice, with its rows untouched.
func TestKnowledgeArticleFKMigrationConverges(t *testing.T) {
	ctx, tx := kbManagerMigrationTx(t, "ka_fk_mig_fresh", "ka_fk_mig_old")
	migration := readMigration(t, kaFKMigrationPath)

	fresh := kaFreshCatalog(t, ctx, tx, "ka_fk_mig_fresh")
	for _, line := range []string{kaFKLine("author_id", `"user"`), kaFKLine("revised_by_id", `"user"`), kaFKLine("source_case_id", "work_item")} {
		if !strings.Contains(fresh+"\n", line+"\n") {
			t.Fatalf("0044 does not make %q:\n%s", line, fresh)
		}
	}
	kbManagerExec(t, ctx, tx, "run 0262 over 0044's table", migration)
	if got := tableCatalog(t, ctx, tx, "ka_fk_mig_fresh", "knowledge_article"); got != fresh {
		t.Errorf("0262 changed 0044's table.\nbefore:\n%s\n\nafter:\n%s", fresh, got)
	}

	kbManagerUseSchema(t, ctx, tx, "ka_fk_mig_old")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbTables000027DDL)
	kaSeed(t, ctx, tx)
	rowsBefore := kaRows(t, ctx, tx)
	if old := tableCatalog(t, ctx, tx, "ka_fk_mig_old", "knowledge_article"); old == fresh {
		t.Fatalf("000027's table already matches 0044's; the test proves nothing")
	}

	for run := 1; run <= 2; run++ {
		kbManagerExec(t, ctx, tx, "run 0262 over 000027's table", migration)
		if got := tableCatalog(t, ctx, tx, "ka_fk_mig_old", "knowledge_article"); got != fresh {
			t.Errorf("run %d: 000027 + 0262 differs from 0044.\n0044:\n%s\n\n000027 + 0262:\n%s", run, fresh, got)
		}
	}
	if got := kaRows(t, ctx, tx); got != rowsBefore {
		t.Errorf("rows changed.\nbefore: %s\nafter:  %s", rowsBefore, got)
	}
}

// TestKnowledgeArticleFKMigrationKeepsRowsThatPointNowhere: a 000027 table
// whose rows name a user and a work item that do not exist. 0262 succeeds and
// changes no row; those two constraints stay NOT VALID (new rows are checked),
// the third is validated. Clearing the values as the header says and
// validating by hand then reaches 0044's table.
func TestKnowledgeArticleFKMigrationKeepsRowsThatPointNowhere(t *testing.T) {
	ctx, tx := kbManagerMigrationTx(t, "ka_fk_mig_fresh", "ka_fk_mig_orphans")
	migration := readMigration(t, kaFKMigrationPath)
	fresh := kaFreshCatalog(t, ctx, tx, "ka_fk_mig_fresh")

	kbManagerUseSchema(t, ctx, tx, "ka_fk_mig_orphans")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbTables000027DDL)
	kaSeed(t, ctx, tx)
	// One missing reference per row: a row updated in the transaction that
	// inserted it has every foreign key re-checked, which real rows (inserted
	// long before the cleanup) do not.
	kbManagerExec(t, ctx, tx, "insert an article naming a missing author", `
		INSERT INTO knowledge_article (id, created_on, updated_on, created_by, updated_by, title, author_id, revised_by_id)
		VALUES (gen_random_uuid(), now(), now(), 'ka-fk-mig-test', 'ka-fk-mig-test', 'Orphan author', $1, $2)`, kaOrphanUserID, kaTestUserID)
	kbManagerExec(t, ctx, tx, "insert an article naming a missing case", `
		INSERT INTO knowledge_article (id, created_on, updated_on, created_by, updated_by, title, revised_by_id, source_case_id)
		VALUES (gen_random_uuid(), now(), now(), 'ka-fk-mig-test', 'ka-fk-mig-test', 'Orphan case', $1, $2)`, kaTestUserID, kaOrphanCaseID)
	rowsBefore := kaRows(t, ctx, tx)

	for run := 1; run <= 2; run++ {
		kbManagerExec(t, ctx, tx, "run 0262", migration)
		got := tableCatalog(t, ctx, tx, "ka_fk_mig_orphans", "knowledge_article") + "\n"
		for _, line := range []string{
			kaFKLine("author_id", `"user"`) + " NOT VALID",
			kaFKLine("revised_by_id", `"user"`),
			kaFKLine("source_case_id", "work_item") + " NOT VALID",
		} {
			if !strings.Contains(got, line+"\n") {
				t.Errorf("run %d: want %q:\n%s", run, line, got)
			}
		}
		if rows := kaRows(t, ctx, tx); rows != rowsBefore {
			t.Fatalf("run %d changed rows.\nbefore: %s\nafter:  %s", run, rowsBefore, rows)
		}
	}

	sp, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	_, err = sp.Exec(ctx, `
		INSERT INTO knowledge_article (id, created_on, updated_on, created_by, updated_by, title, source_case_id)
		VALUES (gen_random_uuid(), now(), now(), 'ka-fk-mig-test', 'ka-fk-mig-test', 'New', gen_random_uuid())`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" || pgErr.ConstraintName != "knowledge_article_source_case_id_fkey" {
		t.Errorf("a new article for an unknown case: err = %v, want 23503 on knowledge_article_source_case_id_fkey", err)
	}
	_ = sp.Rollback(ctx)

	// What the header tells an operator to do.
	for _, c := range []struct{ column, target string }{{"author_id", `"user"`}, {"source_case_id", "work_item"}} {
		kbManagerExec(t, ctx, tx, "clear "+c.column, `
			UPDATE knowledge_article a SET `+c.column+` = NULL
			 WHERE `+c.column+` IS NOT NULL
			   AND NOT EXISTS (SELECT 1 FROM `+c.target+` t WHERE t.id = a.`+c.column+`)`)
		kbManagerExec(t, ctx, tx, "validate "+c.column, `ALTER TABLE knowledge_article VALIDATE CONSTRAINT knowledge_article_`+c.column+`_fkey`)
	}
	if got := tableCatalog(t, ctx, tx, "ka_fk_mig_orphans", "knowledge_article"); got != fresh {
		t.Errorf("after clearing and validating by hand, the table differs from 0044's.\n0044:\n%s\n\nnow:\n%s", fresh, got)
	}
}

// TestKnowledgeArticleFKMigrationLeavesExistingConstraintsAlone: a foreign key
// that is already there under another name and delete action is kept and not
// doubled, and a constraint holding 0044's name for something else blocks
// only its own column; the others are still added.
func TestKnowledgeArticleFKMigrationLeavesExistingConstraintsAlone(t *testing.T) {
	ctx, tx := kbManagerMigrationTx(t, "ka_fk_mig_has_fk")

	kbManagerUseSchema(t, ctx, tx, "ka_fk_mig_has_fk")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbTables000027DDL)
	kbManagerExec(t, ctx, tx, "add an author foreign key of another name",
		`ALTER TABLE knowledge_article ADD CONSTRAINT fk_ka_author FOREIGN KEY (author_id) REFERENCES "user"(id)`)
	kbManagerExec(t, ctx, tx, "take 0044's name for the revised_by_id foreign key",
		`ALTER TABLE knowledge_article ADD CONSTRAINT knowledge_article_revised_by_id_fkey CHECK (true)`)

	kbManagerExec(t, ctx, tx, "run 0262", readMigration(t, kaFKMigrationPath))

	var fks []string
	rows, err := tx.Query(ctx, `
		SELECT conname || ' ' || pg_get_constraintdef(oid) FROM pg_constraint
		 WHERE conrelid = 'knowledge_article'::regclass AND (contype = 'f' OR conname LIKE '%_fkey')
		   AND pg_get_constraintdef(oid) NOT LIKE '%knowledge_base(id)%' AND pg_get_constraintdef(oid) NOT LIKE '%knowledge_article(id)%'`)
	if err != nil {
		t.Fatalf("read constraints: %v", err)
	}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		fks = append(fks, s)
	}
	rows.Close()
	sort.Strings(fks)
	want := []string{
		`fk_ka_author FOREIGN KEY (author_id) REFERENCES "user"(id)`,
		`knowledge_article_revised_by_id_fkey CHECK (true)`,
		`knowledge_article_source_case_id_fkey FOREIGN KEY (source_case_id) REFERENCES work_item(id) ON DELETE SET NULL`,
	}
	if strings.Join(fks, "\n") != strings.Join(want, "\n") {
		t.Errorf("constraints:\n%s\nwant:\n%s", strings.Join(fks, "\n"), strings.Join(want, "\n"))
	}
}
