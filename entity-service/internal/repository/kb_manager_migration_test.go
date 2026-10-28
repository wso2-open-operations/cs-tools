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
// knowledge base manager tables. 0251 creates them where they are missing, with
// the foreign key from user_id to "user" that 000027 left out, and adds that
// foreign key where the tables came from 000027, without touching their rows.
//
// The first three tests read the SQL and the migrations folder and need no
// database. The others run against a live PostgreSQL instance built from
// migrations/ (knowledge_base and "user" must exist); the migration tests work
// in throwaway schemas inside a transaction that is rolled back:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run 'KBManager|KBLegacyStragglers'

package repository_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
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

// kbManagerUserFKLine is the catalog line for the foreign key 0251 gives
// user_id, validated. An unvalidated one ends in " NOT VALID".
const kbManagerUserFKLine = `con knowledge_base_manager_user knowledge_base_manager_user_user_id_fkey FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE`

func readKBManagerMigration(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(kbManagerMigrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	return string(raw)
}

// TestKBManagerMigrationOnlyAddsWhatIsMissing pins the shape: two tables and
// three indexes, every one IF NOT EXISTS; user_id references "user" and
// group_id references nothing; and the only ALTERs add and validate that one
// foreign key. Nothing may remove or change what a database already has.
func TestKBManagerMigrationOnlyAddsWhatIsMissing(t *testing.T) {
	body := strings.ToUpper(sqlWithoutComments(readKBManagerMigration(t)))
	flat := strings.Join(strings.Fields(body), " ")

	// Statement forms, so "ON DELETE CASCADE" and the DO block's own BEGIN
	// are not hits; transaction control would break `make migrate`.
	for _, banned := range []string{`\bDROP\s`, `\bTRUNCATE\b`, `\bDELETE\s+FROM\b`, `\bUPDATE\s+\S+\s+SET\b`, `\bINSERT\s+INTO\b`, `\bCOMMIT\b`, `\bROLLBACK\b`, `\bSTART\s+TRANSACTION\b`, `\bBEGIN\s*(;|TRANSACTION\b|WORK\b|ISOLATION\b)`} {
		if regexp.MustCompile(banned).MatchString(body) {
			t.Errorf("migration matches %q; it may only add what is missing", banned)
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

	if !strings.Contains(flat, `USER_ID UUID NOT NULL REFERENCES "USER"(ID) ON DELETE CASCADE,`) {
		t.Error(`user_id must be UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE`)
	}
	if !strings.Contains(flat, `GROUP_ID UUID NOT NULL, GROUP_NAME`) {
		t.Error("group_id must stay a plain UUID NOT NULL, as in 000027")
	}

	alters := regexp.MustCompile(`\bALTER\s[^;]*;`).FindAllString(flat, -1)
	wantAlters := []string{
		`ALTER TABLE KNOWLEDGE_BASE_MANAGER_USER ADD CONSTRAINT KNOWLEDGE_BASE_MANAGER_USER_USER_ID_FKEY FOREIGN KEY (USER_ID) REFERENCES "USER"(ID) ON DELETE CASCADE NOT VALID;`,
		`ALTER TABLE KNOWLEDGE_BASE_MANAGER_USER VALIDATE CONSTRAINT KNOWLEDGE_BASE_MANAGER_USER_USER_ID_FKEY;`,
	}
	if strings.Join(alters, "\n") != strings.Join(wantAlters, "\n") {
		t.Errorf("ALTER statements:\n%s\nwant:\n%s", strings.Join(alters, "\n"), strings.Join(wantAlters, "\n"))
	}
}

// kbManagerTableDefinition matches a statement that creates one of the two
// manager tables: CREATE TABLE, optionally IF NOT EXISTS, optionally schema
// qualified and quoted. A statement that only names a table (ALTER TABLE, an
// index, a policy, a grant) does not match, and neither does a longer name.
var kbManagerTableDefinition = regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"?\w+"?\s*\.\s*)?"?(knowledge_base_manager_(?:user|group))\b"?`)

// stripSQLComments removes -- comments to the end of the line and /* */
// blocks, so a definition quoted in a comment is not taken for one.
func stripSQLComments(sql string) string {
	sql = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(sql, " ")
	return regexp.MustCompile(`--[^\n]*`).ReplaceAllString(sql, "")
}

// kbManagerTablesDefined lists the manager tables an SQL file creates.
func kbManagerTablesDefined(sql string) []string {
	var tables []string
	for _, m := range kbManagerTableDefinition.FindAllStringSubmatch(stripSQLComments(sql), -1) {
		tables = append(tables, strings.ToLower(m[1]))
	}
	return tables
}

// TestKBManagerTableDefinitionMatcher is the matcher behind
// TestKBLegacyStragglersAreGone, both ways: every way of creating one of the
// tables is caught, and nothing that only refers to them is.
func TestKBManagerTableDefinitionMatcher(t *testing.T) {
	defines := []string{
		"CREATE TABLE knowledge_base_manager_user (id UUID PRIMARY KEY);",
		"create table if not exists knowledge_base_manager_group(id uuid);",
		"CREATE\n    TABLE\n    IF NOT EXISTS\n    knowledge_base_manager_user\n(\n    id UUID\n);",
		"CREATE TABLE public.knowledge_base_manager_user (id UUID);",
		`CREATE TABLE "knowledge_base_manager_group" (id UUID);`,
		`CREATE TABLE IF NOT EXISTS "public"."knowledge_base_manager_user" (id UUID);`,
		"SELECT 1; CREATE TABLE knowledge_base_manager_group (id UUID); -- second definition",
	}
	for _, sql := range defines {
		if len(kbManagerTablesDefined(sql)) != 1 {
			t.Errorf("not caught as a definition: %q", sql)
		}
	}

	refersOnly := []string{
		`ALTER TABLE knowledge_base_manager_user ADD CONSTRAINT fk_x FOREIGN KEY (user_id) REFERENCES "user"(id) NOT VALID;`,
		"ALTER TABLE knowledge_base_manager_user ENABLE ROW LEVEL SECURITY;",
		"ALTER TABLE knowledge_base_manager_group FORCE ROW LEVEL SECURITY;",
		"DROP POLICY IF EXISTS kb_manager_read ON knowledge_base_manager_user;\nCREATE POLICY kb_manager_read ON knowledge_base_manager_user FOR SELECT USING (is_internal());",
		"CREATE INDEX IF NOT EXISTS idx_kb_manager_group_group_id ON knowledge_base_manager_group (group_id);",
		"CREATE UNIQUE INDEX idx_x ON public.knowledge_base_manager_user (user_id);",
		"GRANT SELECT ON knowledge_base_manager_user TO csm_app;",
		"INSERT INTO knowledge_base_manager_user (id) VALUES (gen_random_uuid());",
		"CREATE TABLE knowledge_base_manager_user_audit (id UUID);",
		"CREATE TABLE IF NOT EXISTS knowledge_base (id UUID);",
		"-- CREATE TABLE knowledge_base_manager_user (the old definition)\nSELECT 1;",
		"SELECT 1; -- unlike CREATE TABLE knowledge_base_manager_group, this adds nothing",
		"/* CREATE TABLE knowledge_base_manager_user (\n   id UUID\n) */ SELECT 1;",
	}
	for _, sql := range refersOnly {
		if got := kbManagerTablesDefined(sql); len(got) != 0 {
			t.Errorf("taken for a definition of %v: %q", got, sql)
		}
	}
}

// TestKBLegacyStragglersAreGone: the 000020-000027 chain is replaced by 0251
// and must not come back. 000020 references products, users and cases, which
// this schema has never had, so any of these files stops a fresh run. 0251 is
// the only file that may create the manager tables; other migrations may
// alter them, index them or put policies on them.
func TestKBLegacyStragglersAreGone(t *testing.T) {
	legacy, err := filepath.Glob("../../migrations/00002[0-7]_*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(legacy) != 0 {
		t.Errorf("legacy KB migrations still present: %v", legacy)
	}

	if got := strings.Join(kbManagerTablesDefined(readKBManagerMigration(t)), ", "); got != "knowledge_base_manager_user, knowledge_base_manager_group" {
		t.Fatalf("%s defines %q; the matcher no longer recognises it", filepath.Base(kbManagerMigrationPath), got)
	}

	all, err := filepath.Glob("../../migrations/*.sql")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range all {
		if filepath.Base(f) == filepath.Base(kbManagerMigrationPath) {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if tables := kbManagerTablesDefined(string(raw)); len(tables) != 0 {
			t.Errorf("%s creates %v; only %s may create the knowledge base manager tables", filepath.Base(f), tables, filepath.Base(kbManagerMigrationPath))
		}
	}
}

// kbManagerCatalog describes the manager tables in one schema: columns,
// constraints (with NOT VALID when unvalidated) and indexes, with the schema
// name taken out so two schemas can be compared.
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

// kbManagerMigrationTx opens the rolled-back transaction the migration tests
// work in, with one schema per name.
func kbManagerMigrationTx(t *testing.T, schemas ...string) (context.Context, pgx.Tx) {
	t.Helper()
	pool := caseStatsPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	for _, schema := range schemas {
		kbManagerExec(t, ctx, tx, "create schema "+schema, `CREATE SCHEMA `+schema)
	}
	return ctx, tx
}

// kbManagerUseSchema puts schema first on the search path. public stays on it
// so knowledge_base and "user" resolve to the real tables; the manager tables
// are created in schema, whether or not public has them.
func kbManagerUseSchema(t *testing.T, ctx context.Context, tx pgx.Tx, schema string) {
	t.Helper()
	kbManagerExec(t, ctx, tx, "set search_path", `SET LOCAL search_path = `+schema+`, public`)
}

func kbManagerExec(t *testing.T, ctx context.Context, tx pgx.Tx, what, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

// kbManagerRows reads every row of both manager tables on the search path.
func kbManagerRows(t *testing.T, ctx context.Context, tx pgx.Tx) string {
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

const (
	kbManagerTestBaseID = "3a111111-0000-0000-0000-0000000000f1"
	kbManagerTestUserID = "3a111111-0000-0000-0000-0000000000e1"
)

// kbManagerSeed adds a knowledge base and a user, then a manager row for that
// user and a manager group, to the manager tables on the search path.
func kbManagerSeed(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	kbManagerExec(t, ctx, tx, "insert knowledge base", `
		INSERT INTO knowledge_base (id, created_on, updated_on, created_by, updated_by, title, active)
		VALUES ($1, now(), now(), 'kb-manager-mig-test', 'kb-manager-mig-test', 'KB manager migration test', true)`, kbManagerTestBaseID)
	kbManagerExec(t, ctx, tx, "insert user", `
		INSERT INTO "user" (id, created_on, updated_on, user_name, email)
		VALUES ($1, now(), now(), 'kb-manager-mig-test', 'kb-manager-mig-test@example.com')`, kbManagerTestUserID)
	kbManagerExec(t, ctx, tx, "insert manager user", `
		INSERT INTO knowledge_base_manager_user (id, knowledge_base_id, user_id, created_on, created_by)
		VALUES (gen_random_uuid(), $1, $2, now(), 'kb-manager-mig-test')`, kbManagerTestBaseID, kbManagerTestUserID)
	kbManagerExec(t, ctx, tx, "insert manager group", `
		INSERT INTO knowledge_base_manager_group (id, knowledge_base_id, group_id, group_name, created_on, created_by)
		VALUES (gen_random_uuid(), $1, gen_random_uuid(), 'KB managers', now(), 'kb-manager-mig-test')`, kbManagerTestBaseID)
}

// TestKBManagerMigrationFreshAndOldDatabasesConverge runs 0251 where the
// tables are missing (a fresh database) and where 000027 made them (no foreign
// key on user_id): both end with the same columns, constraints and indexes,
// the user foreign key validated.
func TestKBManagerMigrationFreshAndOldDatabasesConverge(t *testing.T) {
	ctx, tx := kbManagerMigrationTx(t, "kb_manager_mig_fresh", "kb_manager_mig_old")
	migration := readKBManagerMigration(t)

	kbManagerUseSchema(t, ctx, tx, "kb_manager_mig_fresh")
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_fresh"); got != "" {
		t.Fatalf("test schema already has manager tables:\n%s", got)
	}
	kbManagerExec(t, ctx, tx, "run migration where the tables are missing", migration)
	fresh := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_fresh")
	if !strings.Contains(fresh+"\n", kbManagerUserFKLine+"\n") {
		t.Fatalf("fresh tables lack the validated user foreign key:\n%s", fresh)
	}
	if strings.Contains(fresh, "knowledge_base_manager_group_group_id_fkey") {
		t.Errorf("group_id must have no foreign key:\n%s", fresh)
	}

	kbManagerUseSchema(t, ctx, tx, "kb_manager_mig_old")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbManager000027DDL)
	if before := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_old"); strings.Contains(before, "user_id_fkey") {
		t.Fatalf("000027's tables already have a user foreign key:\n%s", before)
	}
	kbManagerExec(t, ctx, tx, "run migration over 000027's tables", migration)

	if old := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_old"); fresh != old {
		t.Errorf("0251 leaves fresh and 000027 databases with different tables.\nfresh:\n%s\n\n000027 + 0251:\n%s", fresh, old)
	}
}

// TestKBManagerMigrationKeepsExistingRows is a database that recorded
// 000020-000027, with manager rows that name real users: running 0251 over
// it, more than once, adds the validated foreign key and changes no row.
func TestKBManagerMigrationKeepsExistingRows(t *testing.T) {
	ctx, tx := kbManagerMigrationTx(t, "kb_manager_mig_existing")
	migration := readKBManagerMigration(t)

	kbManagerUseSchema(t, ctx, tx, "kb_manager_mig_existing")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbManager000027DDL)
	kbManagerSeed(t, ctx, tx)
	rowsBefore := kbManagerRows(t, ctx, tx)

	kbManagerExec(t, ctx, tx, "first run", migration)
	afterFirst := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_existing")
	if !strings.Contains(afterFirst+"\n", kbManagerUserFKLine+"\n") {
		t.Errorf("first run did not add the validated user foreign key:\n%s", afterFirst)
	}
	kbManagerExec(t, ctx, tx, "second run", migration)
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_existing"); got != afterFirst {
		t.Errorf("second run changed the tables.\nafter first:\n%s\n\nafter second:\n%s", afterFirst, got)
	}
	if got := kbManagerRows(t, ctx, tx); got != rowsBefore {
		t.Errorf("rows changed.\nbefore: %s\nafter:  %s", rowsBefore, got)
	}
}

// TestKBManagerMigrationKeepsRowsThatNameNoUser: a 000027 database whose
// manager table already holds a row for a user that does not exist. 0251 still
// succeeds and deletes nothing; the foreign key is added NOT VALID, so new rows
// are checked, and validating it is left until the row is dealt with.
func TestKBManagerMigrationKeepsRowsThatNameNoUser(t *testing.T) {
	ctx, tx := kbManagerMigrationTx(t, "kb_manager_mig_orphans")
	migration := readKBManagerMigration(t)

	kbManagerUseSchema(t, ctx, tx, "kb_manager_mig_orphans")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbManager000027DDL)
	kbManagerSeed(t, ctx, tx)
	const orphanUserID = "3a111111-0000-0000-0000-0000000000d1"
	kbManagerExec(t, ctx, tx, "insert a manager row for a user that does not exist", `
		INSERT INTO knowledge_base_manager_user (id, knowledge_base_id, user_id, created_on, created_by)
		VALUES (gen_random_uuid(), $1, $2, now(), 'kb-manager-mig-test')`, kbManagerTestBaseID, orphanUserID)
	rowsBefore := kbManagerRows(t, ctx, tx)

	for run := 1; run <= 2; run++ {
		kbManagerExec(t, ctx, tx, "run migration", migration)
		if got := kbManagerRows(t, ctx, tx); got != rowsBefore {
			t.Fatalf("run %d changed rows.\nbefore: %s\nafter:  %s", run, rowsBefore, got)
		}
		if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_orphans"); !strings.Contains(got+"\n", kbManagerUserFKLine+" NOT VALID\n") {
			t.Fatalf("run %d: want the user foreign key added NOT VALID:\n%s", run, got)
		}
	}

	sp, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	_, err = sp.Exec(ctx, `
		INSERT INTO knowledge_base_manager_user (id, knowledge_base_id, user_id, created_on, created_by)
		VALUES (gen_random_uuid(), $1, gen_random_uuid(), now(), 'kb-manager-mig-test')`, kbManagerTestBaseID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" || pgErr.ConstraintName != "knowledge_base_manager_user_user_id_fkey" {
		t.Errorf("a new row for an unknown user: err = %v, want 23503 on knowledge_base_manager_user_user_id_fkey", err)
	}
	_ = sp.Rollback(ctx)

	// What the migration's NOTICE tells an operator to do.
	kbManagerExec(t, ctx, tx, "delete the row", `DELETE FROM knowledge_base_manager_user WHERE user_id = $1`, orphanUserID)
	kbManagerExec(t, ctx, tx, "validate by hand", `ALTER TABLE knowledge_base_manager_user VALIDATE CONSTRAINT knowledge_base_manager_user_user_id_fkey`)
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_orphans"); !strings.Contains(got+"\n", kbManagerUserFKLine+"\n") {
		t.Errorf("after validating by hand, want the foreign key validated:\n%s", got)
	}
}

// TestKBManagerMigrationLeavesAnExistingUserForeignKeyAlone: a database whose
// manager table already has a foreign key from user_id to "user", under its
// own name and delete action, keeps it and gets no second one.
func TestKBManagerMigrationLeavesAnExistingUserForeignKeyAlone(t *testing.T) {
	ctx, tx := kbManagerMigrationTx(t, "kb_manager_mig_has_fk")

	kbManagerUseSchema(t, ctx, tx, "kb_manager_mig_has_fk")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbManager000027DDL)
	kbManagerExec(t, ctx, tx, "add a user foreign key of another name",
		`ALTER TABLE knowledge_base_manager_user ADD CONSTRAINT fk_kb_manager_user_user FOREIGN KEY (user_id) REFERENCES "user"(id)`)
	before := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_has_fk")

	kbManagerExec(t, ctx, tx, "run migration", readKBManagerMigration(t))
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_has_fk"); got != before {
		t.Errorf("tables changed.\nbefore:\n%s\n\nafter:\n%s", before, got)
	}
}

// TestKBManagerReposNameTheMissingReference: a manager row for a user or a
// knowledge base that does not exist is a ValidationError that names the
// request field, never the table and column from the database's own message.
// Runs against the database's own tables (0251 ran there).
func TestKBManagerReposNameTheMissingReference(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()

	baseID, userID := "3a111111-0000-0000-0000-0000000000c1", "3a111111-0000-0000-0000-0000000000c2"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM knowledge_base WHERE id = $1`, baseID)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, userID)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `
		INSERT INTO knowledge_base (id, created_on, updated_on, created_by, updated_by, title, active)
		VALUES ($1, now(), now(), 'kb-manager-repo-test', 'kb-manager-repo-test', 'KB manager repo test', true)`, baseID); err != nil {
		t.Fatalf("insert knowledge base: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO "user" (id, created_on, updated_on, user_name, email)
		VALUES ($1, now(), now(), 'kb-manager-repo-test', 'kb-manager-repo-test@example.com')`, userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	const unknownID = "3a111111-0000-0000-0000-0000000000c9"

	users := repository.NewKBManagerUserRepository(pool)
	groups := repository.NewKBManagerGroupRepository(pool)
	cases := []struct {
		name, want string
		create     func() error
	}{
		{"user that does not exist", "userId does not refer to an existing user", func() error {
			_, err := users.CreateKBManagerUser(ctx, domain.CreateKBManagerUserRequest{KnowledgeBaseID: baseID, UserID: unknownID}, "kb-manager-repo-test")
			return err
		}},
		{"user on a knowledge base that does not exist", "knowledgeBaseId does not refer to an existing knowledge base", func() error {
			_, err := users.CreateKBManagerUser(ctx, domain.CreateKBManagerUserRequest{KnowledgeBaseID: unknownID, UserID: userID}, "kb-manager-repo-test")
			return err
		}},
		{"group on a knowledge base that does not exist", "knowledgeBaseId does not refer to an existing knowledge base", func() error {
			_, err := groups.CreateKBManagerGroup(ctx, domain.CreateKBManagerGroupRequest{KnowledgeBaseID: unknownID, GroupID: unknownID}, "kb-manager-repo-test")
			return err
		}},
	}
	for _, c := range cases {
		err := c.create()
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != c.want {
			t.Errorf("%s: err = %v, want ValidationError %q", c.name, err, c.want)
		}
	}

	if _, err := users.CreateKBManagerUser(ctx, domain.CreateKBManagerUserRequest{KnowledgeBaseID: baseID, UserID: userID}, "kb-manager-repo-test"); err != nil {
		t.Errorf("a manager row for an existing user and knowledge base: %v", err)
	}
}
