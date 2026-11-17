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
// The first four tests read the SQL and the migrations folder and need no
// database. The others run against a live PostgreSQL instance built from
// migrations/ (knowledge_base and "user" must exist); the migration tests work
// in throwaway schemas inside a transaction that is rolled back, and the one
// about lock_timeout holds a lock on "user" from a second connection for about
// five seconds:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run 'KBManager|KBLegacyStragglers|MigrationLocking'

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
// group_id references nothing; the only ALTERs add and validate that one
// foreign key, in separate DO blocks under a lock_timeout. Nothing may remove
// or change what a database already has.
func TestKBManagerMigrationOnlyAddsWhatIsMissing(t *testing.T) {
	body := strings.ToUpper(sqlWithoutComments(readKBManagerMigration(t)))
	flat := strings.Join(strings.Fields(body), " ")

	for _, problem := range migrationLockingProblems(body) {
		t.Error(problem)
	}

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

var (
	migrationDOBlock      = regexp.MustCompile(`(?s)\bDO\s+\$\$(.*?)\$\$\s*;`)
	migrationQuietMessage = regexp.MustCompile(`\bRAISE\s+(NOTICE|INFO|LOG|DEBUG)\b`)
)

// migrationLockingProblems lists how a migration that adds foreign keys to
// tables that may already hold rows (0251, 0262) breaks the locking rules:
// lock_timeout is set by its first statement and reset by its last, the
// constraints are added in one DO block and validated in another, so the
// SHARE ROW EXCLUSIVE locks an ADD takes end before the validation scan, and
// what it leaves for an operator is a RAISE WARNING, not a quieter level.
// body is upper case with its comments removed.
func migrationLockingProblems(body string) []string {
	var problems []string
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "SET LOCK_TIMEOUT = '5S';") {
		problems = append(problems, "the first statement must be SET lock_timeout = '5s'")
	}
	if !strings.HasSuffix(body, "RESET LOCK_TIMEOUT;") {
		problems = append(problems, "the last statement must be RESET lock_timeout")
	}
	var adds, validates bool
	for i, block := range migrationDOBlock.FindAllStringSubmatch(body, -1) {
		add := strings.Contains(block[1], "ADD CONSTRAINT")
		validate := strings.Contains(block[1], "VALIDATE CONSTRAINT")
		if add && validate {
			problems = append(problems, fmt.Sprintf("DO block %d both adds and validates a constraint; validate in a DO block of its own", i+1))
		}
		adds = adds || add
		validates = validates || validate
	}
	if !adds || !validates {
		problems = append(problems, "want one DO block that adds the constraints and another that validates them")
	}
	if m := migrationQuietMessage.FindString(body); m != "" {
		problems = append(problems, fmt.Sprintf("%s: an outcome an operator must act on is a RAISE WARNING", m))
	}
	return problems
}

// TestMigrationLockingProblems is the checker above, both ways.
func TestMigrationLockingProblems(t *testing.T) {
	const add = "DO $$ BEGIN ALTER TABLE T ADD CONSTRAINT C FOREIGN KEY (X) REFERENCES U(ID) NOT VALID; END $$;"
	const validate = "DO $$ BEGIN IF N > 0 THEN RAISE WARNING 'LEFT'; RETURN; END IF; ALTER TABLE T VALIDATE CONSTRAINT C; END $$;"
	const set, reset = "SET LOCK_TIMEOUT = '5S';", "RESET LOCK_TIMEOUT;"
	cases := []struct {
		name, body string
		want       []string
	}{
		{"separate blocks under a lock_timeout", set + "\nCREATE TABLE IF NOT EXISTS T (X INT);\n" + add + "\n" + validate + "\n" + reset, nil},
		{"no lock_timeout", add + "\n" + validate + "\n" + reset, []string{"the first statement must be SET lock_timeout = '5s'"}},
		{"lock_timeout after the first statement", "CREATE TABLE IF NOT EXISTS T (X INT);\n" + set + add + validate + reset, []string{"the first statement must be SET lock_timeout = '5s'"}},
		{"no reset", set + add + validate, []string{"the last statement must be RESET lock_timeout"}},
		{"add and validate in one block", set + "DO $$ BEGIN ALTER TABLE T ADD CONSTRAINT C FOREIGN KEY (X) REFERENCES U(ID) NOT VALID; ALTER TABLE T VALIDATE CONSTRAINT C; END $$;" + reset,
			[]string{"DO block 1 both adds and validates a constraint; validate in a DO block of its own"}},
		{"no validation", set + add + reset, []string{"want one DO block that adds the constraints and another that validates them"}},
		{"a notice", set + add + strings.Replace(validate, "RAISE WARNING", "RAISE NOTICE", 1) + reset, []string{"RAISE NOTICE: an outcome an operator must act on is a RAISE WARNING"}},
	}
	for _, c := range cases {
		if got := migrationLockingProblems(c.body); strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("%s: problems = %q, want %q", c.name, got, c.want)
		}
	}
}

// kbManagerTableDefinition matches a statement that creates one of the two
// manager tables: CREATE TABLE, optionally IF NOT EXISTS, optionally schema
// qualified and quoted. A statement that only names a table (ALTER TABLE, an
// index, a policy, a grant) does not match, and neither does a longer name.
var kbManagerTableDefinition = regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"?\w+"?\s*\.\s*)?"?(knowledge_base_manager_(?:user|group))\b"?`)

// stripSQLComments removes -- comments to the end of the line and /* */
// blocks, so a definition quoted in a comment is not taken for one.
// sqlWithoutComments drops whole-line comments only, which is all the shape
// tests need; a CREATE TABLE in some other migration can also sit after code
// on a line or in a block comment.
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

// migrationWarnings collects the WARNINGs the server sends on the connection
// a migration test runs on.
type migrationWarnings struct {
	mu   sync.Mutex
	msgs []string
}

func (w *migrationWarnings) add(n *pgconn.Notice) {
	if n.SeverityUnlocalized != "WARNING" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.msgs = append(w.msgs, n.Message)
}

// take returns the WARNINGs received since the last call.
func (w *migrationWarnings) take() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	msgs := w.msgs
	w.msgs = nil
	return msgs
}

// expectWarnings fails unless exactly want (in order) arrived since the last
// call.
func (w *migrationWarnings) expectWarnings(t *testing.T, what string, want ...string) {
	t.Helper()
	if got := w.take(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s: WARNINGs\n%q\nwant\n%q", what, got, want)
	}
}

// kbManagerMigrationTx opens the rolled-back transaction the migration tests
// work in, with one schema per name, on a connection of its own whose
// WARNINGs are collected.
func kbManagerMigrationTx(t *testing.T, schemas ...string) (context.Context, pgx.Tx, *migrationWarnings) {
	t.Helper()
	dsn := os.Getenv("CASE_STATS_TEST_DSN")
	if dsn == "" {
		t.Skip("CASE_STATS_TEST_DSN not set")
	}
	ctx := context.Background()
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	warnings := &migrationWarnings{}
	cfg.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) { warnings.add(n) }
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	for _, schema := range schemas {
		kbManagerExec(t, ctx, tx, "create schema "+schema, `CREATE SCHEMA `+schema)
	}
	return ctx, tx, warnings
}

// execWhileTableIsBusy runs sql in a savepoint of tx while another connection
// holds ROW EXCLUSIVE on table in a transaction it has not committed, as a
// writer that has not finished does, and returns sql's error. The savepoint
// is rolled back and the other transaction ended before it returns.
func execWhileTableIsBusy(t *testing.T, ctx context.Context, tx pgx.Tx, table, sql string) error {
	t.Helper()
	blocker, err := caseStatsPool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin the busy transaction: %v", err)
	}
	defer func() { _ = blocker.Rollback(ctx) }()
	if _, err := blocker.Exec(ctx, `LOCK TABLE `+table+` IN ROW EXCLUSIVE MODE`); err != nil {
		t.Fatalf("lock %s: %v", table, err)
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	_, execErr := sp.Exec(ctx, sql)
	if err := sp.Rollback(ctx); err != nil {
		t.Fatalf("roll back to the savepoint: %v", err)
	}
	return execErr
}

// expectLockTimeout fails unless err is the lock_timeout's 55P03. The test's
// own statement_timeout ends the wait with 57014 instead when the migration
// sets no lock_timeout.
func expectLockTimeout(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("with the referenced table busy: err = %v, want 55P03 (lock_not_available) from the migration's lock_timeout", err)
	}
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
// the user foreign key validated, and neither run raises a WARNING.
func TestKBManagerMigrationFreshAndOldDatabasesConverge(t *testing.T) {
	ctx, tx, warnings := kbManagerMigrationTx(t, "kb_manager_mig_fresh", "kb_manager_mig_old")
	migration := readKBManagerMigration(t)

	kbManagerUseSchema(t, ctx, tx, "kb_manager_mig_fresh")
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_fresh"); got != "" {
		t.Fatalf("test schema already has manager tables:\n%s", got)
	}
	kbManagerExec(t, ctx, tx, "run migration where the tables are missing", migration)
	warnings.expectWarnings(t, "run where the tables are missing")
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
	warnings.expectWarnings(t, "run over 000027's tables")

	if old := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_old"); fresh != old {
		t.Errorf("0251 leaves fresh and 000027 databases with different tables.\nfresh:\n%s\n\n000027 + 0251:\n%s", fresh, old)
	}
}

// TestKBManagerMigrationKeepsExistingRows is a database that recorded
// 000020-000027, with manager rows that name real users: running 0251 over
// it, more than once, adds the validated foreign key and changes no row.
func TestKBManagerMigrationKeepsExistingRows(t *testing.T) {
	ctx, tx, _ := kbManagerMigrationTx(t, "kb_manager_mig_existing")
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
// are checked, and every run raises a WARNING with the count until the row is
// dealt with. The header's cleanup then validates it.
func TestKBManagerMigrationKeepsRowsThatNameNoUser(t *testing.T) {
	ctx, tx, warnings := kbManagerMigrationTx(t, "kb_manager_mig_orphans")
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
		warnings.expectWarnings(t, fmt.Sprintf("run %d", run),
			`knowledge_base_manager_user.user_id: 1 row(s) match no row of "user", so knowledge_base_manager_user_user_id_fkey stays NOT VALID until they are deleted or corrected (see 0251_knowledge_base_manager_tables.sql)`)
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

	// What the migration's header tells an operator to do.
	kbManagerExec(t, ctx, tx, "delete the rows", `
		DELETE FROM knowledge_base_manager_user m
		 WHERE NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = m.user_id)`)
	kbManagerExec(t, ctx, tx, "validate by hand", `ALTER TABLE knowledge_base_manager_user VALIDATE CONSTRAINT knowledge_base_manager_user_user_id_fkey`)
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_orphans"); !strings.Contains(got+"\n", kbManagerUserFKLine+"\n") {
		t.Errorf("after validating by hand, want the foreign key validated:\n%s", got)
	}
}

// TestKBManagerMigrationLeavesAnExistingUserForeignKeyAlone: a database whose
// manager table already has a foreign key from user_id to "user", under its
// own name and delete action, keeps it and gets no second one.
func TestKBManagerMigrationLeavesAnExistingUserForeignKeyAlone(t *testing.T) {
	ctx, tx, warnings := kbManagerMigrationTx(t, "kb_manager_mig_has_fk")

	kbManagerUseSchema(t, ctx, tx, "kb_manager_mig_has_fk")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbManager000027DDL)
	kbManagerExec(t, ctx, tx, "add a user foreign key of another name",
		`ALTER TABLE knowledge_base_manager_user ADD CONSTRAINT fk_kb_manager_user_user FOREIGN KEY (user_id) REFERENCES "user"(id)`)
	before := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_has_fk")

	kbManagerExec(t, ctx, tx, "run migration", readKBManagerMigration(t))
	warnings.expectWarnings(t, "run with a user foreign key already there")
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_has_fk"); got != before {
		t.Errorf("tables changed.\nbefore:\n%s\n\nafter:\n%s", before, got)
	}
}

// TestKBManagerMigrationWarnsWhenTheNameIsTaken: a 000027 table where another
// constraint already holds knowledge_base_manager_user_user_id_fkey. 0251 does
// not fail on it: it adds nothing, leaves the table as it was and says so in a
// WARNING on every run.
func TestKBManagerMigrationWarnsWhenTheNameIsTaken(t *testing.T) {
	ctx, tx, warnings := kbManagerMigrationTx(t, "kb_manager_mig_name_taken")

	kbManagerUseSchema(t, ctx, tx, "kb_manager_mig_name_taken")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbManager000027DDL)
	kbManagerExec(t, ctx, tx, "take the name for something else",
		`ALTER TABLE knowledge_base_manager_user ADD CONSTRAINT knowledge_base_manager_user_user_id_fkey CHECK (true)`)
	before := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_name_taken")

	for run := 1; run <= 2; run++ {
		kbManagerExec(t, ctx, tx, "run migration", readKBManagerMigration(t))
		warnings.expectWarnings(t, fmt.Sprintf("run %d", run),
			`knowledge_base_manager_user.user_id: constraint knowledge_base_manager_user_user_id_fkey already exists and is not a foreign key from this column to "user", so no foreign key was added for it (see 0251_knowledge_base_manager_tables.sql)`)
		if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_name_taken"); got != before {
			t.Errorf("run %d changed the tables.\nbefore:\n%s\n\nafter:\n%s", run, before, got)
		}
	}
}

// TestKBManagerMigrationValidatesWhatAnEarlierRunLeft: a run that stopped
// after the ADD (a lock_timeout in the validation, say) left the constraint
// NOT VALID. The next run validates it.
func TestKBManagerMigrationValidatesWhatAnEarlierRunLeft(t *testing.T) {
	ctx, tx, warnings := kbManagerMigrationTx(t, "kb_manager_mig_half_done")

	kbManagerUseSchema(t, ctx, tx, "kb_manager_mig_half_done")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbManager000027DDL)
	kbManagerSeed(t, ctx, tx)
	kbManagerExec(t, ctx, tx, "add the constraint as the first DO block does", `
		ALTER TABLE knowledge_base_manager_user
			ADD CONSTRAINT knowledge_base_manager_user_user_id_fkey
			FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE NOT VALID`)

	kbManagerExec(t, ctx, tx, "run migration", readKBManagerMigration(t))
	warnings.expectWarnings(t, "run over a constraint left NOT VALID")
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_half_done"); !strings.Contains(got+"\n", kbManagerUserFKLine+"\n") {
		t.Errorf("want the foreign key validated:\n%s", got)
	}
}

// TestKBManagerMigrationGivesUpWhenUserIsBusy: adding the foreign key needs
// SHARE ROW EXCLUSIVE on "user", which waits for every transaction that has
// written to it, and every writer that comes after waits behind it. With
// "user" held by another transaction, 0251 gives up after its lock_timeout
// instead of holding writers back for as long as that transaction lasts; run
// again once it has ended, it adds and validates the key.
func TestKBManagerMigrationGivesUpWhenUserIsBusy(t *testing.T) {
	ctx, tx, _ := kbManagerMigrationTx(t, "kb_manager_mig_busy")
	migration := readKBManagerMigration(t)

	kbManagerUseSchema(t, ctx, tx, "kb_manager_mig_busy")
	kbManagerExec(t, ctx, tx, "create the tables as 000027 did", kbManager000027DDL)
	kbManagerExec(t, ctx, tx, "bound the wait if the migration sets no lock_timeout", `SET LOCAL statement_timeout = '20s'`)

	expectLockTimeout(t, execWhileTableIsBusy(t, ctx, tx, `"user"`, migration))
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_busy"); strings.Contains(got, "user_id_fkey") {
		t.Errorf("the run that gave up left a constraint behind:\n%s", got)
	}

	kbManagerExec(t, ctx, tx, "run migration again", migration)
	if got := kbManagerCatalog(t, ctx, tx, "kb_manager_mig_busy"); !strings.Contains(got+"\n", kbManagerUserFKLine+"\n") {
		t.Errorf("the second run did not add the validated user foreign key:\n%s", got)
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
