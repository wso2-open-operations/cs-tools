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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// Behaviour of the SQL in migration 0295, against a real Postgres 18 with the
// migration applied. Skipped without ENTITY_TEST_DATABASE_URL. The audit log is
// append-only, so these tests leave their rows in it; use a throwaway database.

package repository_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func newAuditMigrationTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// auditSchema is the schema the test connection creates its tables in, which is
// where the migration put the audit objects.
func auditSchema(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var schema string
	if err := pool.QueryRow(context.Background(), "SELECT current_schema()").Scan(&schema); err != nil {
		t.Fatalf("current_schema: %v", err)
	}
	return schema
}

func auditExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func auditCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// auditTestTable drops the table and its registry row before and after the test.
func auditTestTable(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	drop := func() {
		_, _ = pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+name+" CASCADE")
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_config WHERE table_name = $1", name)
	}
	drop()
	t.Cleanup(drop)
}

func TestAuditMigration_CompositeKeysDoNotCollide(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	var a, b, plain, withNull, withEmpty string
	err := pool.QueryRow(context.Background(), `SELECT
	    audit_record_key('{"a":"a|b","b":"c"}'::jsonb, ARRAY['a','b']),
	    audit_record_key('{"a":"a","b":"b|c"}'::jsonb, ARRAY['a','b']),
	    audit_record_key('{"id":"42"}'::jsonb, ARRAY['id']),
	    audit_record_key('{"a":null,"b":"x"}'::jsonb, ARRAY['a','b']),
	    audit_record_key('{"a":"","b":"x"}'::jsonb, ARRAY['a','b'])`).Scan(&a, &b, &plain, &withNull, &withEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Errorf("keys (a|b, c) and (a, b|c) both give %q", a)
	}
	if withNull == withEmpty {
		t.Errorf("a NULL and an empty key part both give %q", withNull)
	}
	if plain != "42" {
		t.Errorf("one-column key = %q, want the plain value 42", plain)
	}
}

func TestAuditMigration_RegisterRejectsAMaskedOrExcludedKeyColumn(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	const table = "zz_audit_test_keys"
	auditTestTable(t, pool, table)
	auditExec(t, pool, "CREATE TABLE "+table+" (id text PRIMARY KEY, secret text)")
	schema := auditSchema(t, pool)

	for name, sql := range map[string]string{
		"masked":   "SELECT register_audit('" + schema + "', '" + table + "', ARRAY['id'], '{}', ARRAY['id'])",
		"excluded": "SELECT register_audit('" + schema + "', '" + table + "', ARRAY['id'], ARRAY['id'], '{}')",
	} {
		_, err := pool.Exec(context.Background(), sql)
		if err == nil || !strings.Contains(err.Error(), "cannot be excluded or masked") {
			t.Errorf("%s key column: err = %v, want a refusal", name, err)
		}
	}
	auditExec(t, pool, "SELECT register_audit('"+schema+"', '"+table+"', ARRAY['id'], '{}', ARRAY['secret'])")
}

func TestAuditMigration_ExcludingARegisteredTableStopsAuditingIt(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	const table = "zz_audit_test_exclude"
	auditTestTable(t, pool, table)
	auditExec(t, pool, "CREATE TABLE "+table+" (id int PRIMARY KEY, note text)")
	schema := auditSchema(t, pool)
	auditExec(t, pool, "SELECT register_audit('"+schema+"', '"+table+"')")
	auditExec(t, pool, "INSERT INTO "+table+" VALUES (1, 'a')")
	// An insert is not audited, so an update is what shows auditing is live. The log
	// is append-only and keeps rows from earlier runs, so count what this update adds.
	beforeUpdate := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = $1", table)
	auditExec(t, pool, "UPDATE "+table+" SET note = 'a2' WHERE id = 1")
	audited := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = $1", table)
	if n := audited - beforeUpdate; n != 1 {
		t.Fatalf("changing one column of a registered table wrote %d audit rows, want 1", n)
	}

	auditExec(t, pool, "SELECT exclude_from_audit('"+schema+"', '"+table+"', 'not needed')")

	if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger WHERE tgrelid = $1::regclass AND tgname = 'csm_audit_row_change'`, table); n != 0 {
		t.Errorf("audit trigger still on an excluded table")
	}
	auditExec(t, pool, "UPDATE "+table+" SET note = 'b' WHERE id = 1")
	if n := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = $1", table); n != audited {
		t.Errorf("excluded table still audited: %d rows, was %d", n, audited)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM audit_reconcile(false) WHERE table_name = $1", table); n != 0 {
		t.Errorf("reconcile reports %d problems for an excluded table, want 0", n)
	}
}

func TestAuditMigration_EveryPartitionRefusesTruncate(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	auditExec(t, pool, "SELECT audit_create_partitions(5)")

	missing := auditCount(t, pool, `SELECT count(*) FROM pg_inherits i
	    WHERE i.inhparent = 'csm_audit_log'::regclass
	      AND NOT EXISTS (SELECT 1 FROM pg_trigger g
	                       WHERE g.tgrelid = i.inhrelid AND g.tgname = 'csm_audit_log_no_truncate')`)
	if missing != 0 {
		t.Errorf("%d partition(s) have no TRUNCATE guard", missing)
	}

	for _, part := range []string{"csm_audit_log_default", "csm_audit_log"} {
		_, err := pool.Exec(context.Background(), "TRUNCATE "+part)
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("TRUNCATE %s: err = %v, want append-only refusal", part, err)
		}
	}
}

func TestAuditMigration_ATempTableCannotShadowTheRegistry(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()

	// A temp table is searched before any schema unless pg_temp is listed last.
	var schema string
	if err := conn.QueryRow(ctx, "SELECT current_schema()").Scan(&schema); err != nil {
		t.Fatalf("current_schema: %v", err)
	}
	for _, sql := range []string{
		"CREATE TEMP TABLE audit_config (LIKE " + pgx.Identifier{schema, "audit_config"}.Sanitize() + " INCLUDING DEFAULTS)",
		`INSERT INTO pg_temp.audit_config (schema_name, table_name, key_columns)
		   VALUES ('` + schema + `', 'zz_audit_test_shadow', '{id}')`,
	} {
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	var n int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM audit_reconcile(false) WHERE table_name = 'zz_audit_test_shadow'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("audit_reconcile read the temp table: %d problem(s) for a table only it lists", n)
	}
}

// Migration 0296 registers work_item with every column recorded: nothing
// excluded, nothing masked.
func TestAuditMigration_WorkItemIsAuditedInFull(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	var excluded, masked []string
	var enabled bool
	err := pool.QueryRow(context.Background(),
		`SELECT excluded_columns, masked_columns, enabled FROM audit_config
		  WHERE schema_name = $1 AND table_name = 'work_item'`, auditSchema(t, pool)).Scan(&excluded, &masked, &enabled)
	if err != nil {
		t.Fatalf("work_item is not registered: %v", err)
	}
	if !enabled {
		t.Error("work_item is registered but switched off")
	}
	if len(excluded) != 0 || len(masked) != 0 {
		t.Errorf("work_item excludes %v and masks %v, want every column recorded", excluded, masked)
	}
	if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
	    WHERE tgrelid = 'work_item'::regclass AND tgname = 'csm_audit_row_change' AND tgenabled = 'A'`); n != 1 {
		t.Errorf("work_item has %d enabled-always audit triggers, want 1", n)
	}
}

// Migration 0297 registers the work item types (their id is the work_item id).
func TestAuditMigration_WorkItemTypesAreAudited(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)
	for _, table := range []string{
		"announcement", "case", "change_request", "change_task", "conversation",
		"engagement", "incident", "incident_alert_task", "incident_task", "problem",
		"problem_task", "security_report_analysis", "service_request", "standard_change_proposal",
	} {
		if n := auditCount(t, pool, `SELECT count(*) FROM audit_config
		    WHERE schema_name = $1 AND table_name = $2 AND enabled`, schema, table); n != 1 {
			t.Errorf("%s is not registered with the audit log", table)
			continue
		}
		if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
		    WHERE tgrelid = format('%I.%I', $1::text, $2::text)::regclass
		      AND tgname = 'csm_audit_row_change' AND tgenabled = 'A'`, schema, table); n != 1 {
			t.Errorf("%s has %d enabled-always audit triggers, want 1", table, n)
		}
	}
}

// Migration 0302 registers the team tables, every column recorded.
func TestAuditMigration_TeamTablesAreAudited(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)
	for _, table := range []string{"team", "team_member"} {
		var excluded, masked []string
		err := pool.QueryRow(context.Background(), `SELECT excluded_columns, masked_columns FROM audit_config
		    WHERE schema_name = $1 AND table_name = $2 AND enabled`, schema, table).Scan(&excluded, &masked)
		if err != nil {
			t.Errorf("%s is not registered with the audit log: %v", table, err)
			continue
		}
		if len(excluded) != 0 || len(masked) != 0 {
			t.Errorf("%s excludes %v and masks %v, want every column recorded", table, excluded, masked)
		}
		if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
		    WHERE tgrelid = format('%I.%I', $1::text, $2::text)::regclass
		      AND tgname = 'csm_audit_row_change' AND tgenabled = 'A'`, schema, table); n != 1 {
			t.Errorf("%s has %d enabled-always audit triggers, want 1", table, n)
		}
	}
}

// Migration 0303 registers the Salesforce opportunity and invoice tables,
// sla_policy and plg_note_revision. Only sync_time_stamp and updated_on on the four
// Salesforce tables (both rewritten by every ingest run) are left out.
func TestAuditMigration_SalesforceSlaAndNoteTablesAreAudited(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)
	for table, wantExcluded := range map[string]string{
		"sf_invoice": "sync_time_stamp,updated_on", "sf_opportunity": "sync_time_stamp,updated_on",
		"sf_opportunity_link": "sync_time_stamp,updated_on", "sf_opportunity_product": "sync_time_stamp,updated_on",
		"sla_policy": "", "plg_note_revision": "",
	} {
		var excluded, masked []string
		err := pool.QueryRow(context.Background(), `SELECT excluded_columns, masked_columns FROM audit_config
		    WHERE schema_name = $1 AND table_name = $2 AND enabled`, schema, table).Scan(&excluded, &masked)
		if err != nil {
			t.Errorf("%s is not registered with the audit log: %v", table, err)
			continue
		}
		if got := strings.Join(excluded, ","); got != wantExcluded || len(masked) != 0 {
			t.Errorf("%s excludes %q and masks %v, want exclude %q and no mask", table, got, masked, wantExcluded)
		}
		if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
		    WHERE tgrelid = format('%I.%I', $1::text, $2::text)::regclass
		      AND tgname = 'csm_audit_row_change' AND tgenabled = 'A'`, schema, table); n != 1 {
			t.Errorf("%s has %d enabled-always audit triggers, want 1", table, n)
		}
	}
}

// Migration 0304: an INSERT is not audited, an UPDATE and a DELETE are. Counts
// what each statement adds, since the append-only log keeps rows from earlier runs.
func TestAuditMigration_InsertsAreNotAudited(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	const table = "zz_audit_test_noinsert"
	auditTestTable(t, pool, table)
	auditExec(t, pool, "CREATE TABLE "+table+" (id int PRIMARY KEY, note text, qty int)")
	auditExec(t, pool, "SELECT register_audit('"+auditSchema(t, pool)+"', '"+table+"')")

	const countSQL = `SELECT count(*) FROM csm_audit_log WHERE table_name = $1 AND action = $2`
	count := func(action string) int { return auditCount(t, pool, countSQL, table, action) }

	insertsBefore := count("INSERT")
	updatesBefore := count("UPDATE")
	deletesBefore := count("DELETE")

	auditExec(t, pool, "INSERT INTO "+table+" VALUES (1, 'a', 2)")
	if n := count("INSERT") - insertsBefore; n != 0 {
		t.Errorf("an insert wrote %d audit rows, want 0", n)
	}
	if n := count("UPDATE") - updatesBefore; n != 0 {
		t.Errorf("an insert wrote %d UPDATE audit rows, want 0", n)
	}

	auditExec(t, pool, "UPDATE "+table+" SET note = 'b' WHERE id = 1")
	if n := count("UPDATE") - updatesBefore; n != 1 {
		t.Errorf("changing one column wrote %d audit rows, want 1", n)
	}

	auditExec(t, pool, "DELETE FROM "+table+" WHERE id = 1")
	if n := count("DELETE") - deletesBefore; n != 3 {
		t.Errorf("a delete wrote %d audit rows, want 3 (one per column)", n)
	}
}

// No audit trigger fires on INSERT, the registry's included; all still fire on
// UPDATE and DELETE and stay enabled when session_replication_role is replica.
// Scoped to this schema's audit_log_row_change(), so a trigger of the same name
// in another schema cannot change the result.
func TestAuditMigration_NoRegisteredTriggerFiresOnInsert(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)
	const mine = `SELECT count(*) FROM pg_trigger WHERE tgname = 'csm_audit_row_change'
	    AND tgfoid = to_regprocedure(format('%I.audit_log_row_change()', $1::text))::oid AND `
	if n := auditCount(t, pool, mine+"tgtype & 4 > 0", schema); n != 0 {
		t.Errorf("%d audit triggers still fire on INSERT, want 0", n)
	}
	if n := auditCount(t, pool, mine+"(tgtype & 8 = 0 OR tgtype & 16 = 0 OR tgenabled <> 'A')", schema); n != 0 {
		t.Errorf("%d audit triggers miss UPDATE or DELETE, or are not enabled-always, want 0", n)
	}
	if n := auditCount(t, pool, mine+"true", schema); n == 0 {
		t.Error("no audit triggers found; the migrations did not register any table")
	}
}

// Registering a table writes no audit row: the registry is not audited on insert
// either, and its own columns (created_by, created_on) say who added the row.
func TestAuditMigration_RegisteringATableWritesNoAuditRow(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	const table = "zz_audit_test_registrar"
	auditTestTable(t, pool, table)
	auditExec(t, pool, "CREATE TABLE "+table+" (id int PRIMARY KEY)")
	schema := auditSchema(t, pool)

	const countSQL = `SELECT count(*) FROM csm_audit_log WHERE table_name = 'audit_config'`
	before := auditCount(t, pool, countSQL)
	auditExec(t, pool, "SELECT register_audit($1, $2)", schema, table)
	if n := auditCount(t, pool, countSQL) - before; n != 0 {
		t.Errorf("registering a table wrote %d audit_config rows, want 0", n)
	}
}

// audit_actor() names the person behind the request the way the audit trigger
// does, so a created_by default and the audit log agree.
func TestAuditMigration_AuditActorFollowsTheTriggerRule(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	actor := func(auditActor, viewer string) string {
		t.Helper()
		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `SELECT set_config('app.audit_actor', $1, true), set_config('app.viewer_email', $2, true)`,
			auditActor, viewer); err != nil {
			t.Fatalf("set_config: %v", err)
		}
		var got string
		if err := tx.QueryRow(ctx, "SELECT audit_actor()").Scan(&got); err != nil {
			t.Fatalf("audit_actor: %v", err)
		}
		return got
	}
	var role string
	if err := pool.QueryRow(context.Background(), "SELECT session_user::text").Scan(&role); err != nil {
		t.Fatalf("session_user: %v", err)
	}
	for _, c := range []struct{ name, auditActor, viewer, want string }{
		{"the audit actor wins", "a@wso2.com", "v@wso2.com", "a@wso2.com"},
		{"the viewer when there is no audit actor", "", "v@wso2.com", "v@wso2.com"},
		{"the database role when neither is set", "", "", role},
	} {
		if got := actor(c.auditActor, c.viewer); got != c.want {
			t.Errorf("%s: audit_actor() = %q, want %q", c.name, got, c.want)
		}
	}
}

// Migration 0304 gives the audited tables that said nothing about who created or
// last changed a row those columns: created_on and created_by as defaults, and
// updated_on and updated_by from a BEFORE UPDATE trigger, so no statement has to
// change. All nullable, since rows from before the migration have no value.
func TestAuditMigration_RowsRecordWhoCreatedAndChangedThem(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)
	for _, c := range []struct {
		table, column, wantDefault string
	}{
		{"work_item_watcher", "created_on", "now()"},
		{"work_item_watcher", "created_by", "audit_actor()"},
		{"work_item_watcher", "updated_on", ""},
		{"work_item_watcher", "updated_by", ""},
		{"case_escalation_notification_list", "created_on", "now()"},
		{"case_escalation_notification_list", "created_by", "audit_actor()"},
		{"case_escalation_notification_list", "updated_on", ""},
		{"case_escalation_notification_list", "updated_by", ""},
		{"comment", "updated_on", ""},
		{"comment", "updated_by", ""},
	} {
		var nullable, def string
		err := pool.QueryRow(context.Background(), `SELECT is_nullable, COALESCE(column_default, '')
		    FROM information_schema.columns
		    WHERE table_schema = $1 AND table_name = $2 AND column_name = $3`, schema, c.table, c.column).Scan(&nullable, &def)
		if err != nil {
			t.Errorf("%s.%s is missing: %v", c.table, c.column, err)
			continue
		}
		if def != c.wantDefault {
			t.Errorf("%s.%s default is %q, want %q", c.table, c.column, def, c.wantDefault)
		}
		if nullable != "YES" {
			t.Errorf("%s.%s is NOT NULL: rows from before the migration have no value", c.table, c.column)
		}
		if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
		    WHERE tgrelid = format('%I.%I', $1::text, $2::text)::regclass AND tgname = 'csm_stamp_update'
		      AND tgenabled = 'O'`, schema, c.table); n != 1 {
			t.Errorf("%s has %d update-stamp triggers, want 1", c.table, n)
		}
	}
}

// An update stamps updated_on and updated_by with the person behind the request;
// an insert stamps created_by the same way and leaves updated_* empty.
func TestAuditMigration_UpdateStampsTheActor(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	const table = "zz_audit_test_stamp"
	auditTestTable(t, pool, table)
	auditExec(t, pool, `CREATE TABLE `+table+` (id int PRIMARY KEY, note text,
	    created_by varchar(255) DEFAULT audit_actor(), updated_on timestamptz, updated_by varchar(255))`)
	auditExec(t, pool, "CREATE TRIGGER csm_stamp_update BEFORE UPDATE ON "+table+
		" FOR EACH ROW EXECUTE FUNCTION audit_stamp_update()")

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.audit_actor', 'a@wso2.com', true)"); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO "+table+" (id, note) VALUES (1, 'a')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var createdBy string
	var updatedOn *string
	if err := tx.QueryRow(ctx, "SELECT created_by, updated_on::text FROM "+table+" WHERE id = 1").Scan(&createdBy, &updatedOn); err != nil {
		t.Fatalf("read after insert: %v", err)
	}
	if createdBy != "a@wso2.com" || updatedOn != nil {
		t.Errorf("after insert created_by=%q updated_on=%v, want a@wso2.com and NULL", createdBy, updatedOn)
	}

	if _, err := tx.Exec(ctx, "SELECT set_config('app.audit_actor', 'b@wso2.com', true)"); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	if _, err := tx.Exec(ctx, "UPDATE "+table+" SET note = 'b' WHERE id = 1"); err != nil {
		t.Fatalf("update: %v", err)
	}
	var updatedBy *string
	if err := tx.QueryRow(ctx, "SELECT updated_by, updated_on::text FROM "+table+" WHERE id = 1").Scan(&updatedBy, &updatedOn); err != nil {
		t.Fatalf("read after update: %v", err)
	}
	if updatedBy == nil || *updatedBy != "b@wso2.com" || updatedOn == nil {
		t.Errorf("after update updated_by=%v updated_on=%v, want b@wso2.com and a time", updatedBy, updatedOn)
	}
}

// A column added to an audited table later is recorded with no change to the
// registration: the trigger reads whatever columns the row has.
func TestAuditMigration_AColumnAddedLaterIsAudited(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	const table = "zz_audit_test_newcol"
	auditTestTable(t, pool, table)
	auditExec(t, pool, "CREATE TABLE "+table+" (id int PRIMARY KEY, note text)")
	auditExec(t, pool, "SELECT register_audit('"+auditSchema(t, pool)+"', '"+table+"')")
	auditExec(t, pool, "INSERT INTO "+table+" VALUES (1, 'a')")

	auditExec(t, pool, "ALTER TABLE "+table+" ADD COLUMN added_later text")
	// The log is append-only, so rows from an earlier run stay: count what this
	// update adds, not what the log holds.
	const countSQL = `SELECT count(*) FROM csm_audit_log
	    WHERE table_name = $1 AND field_name = 'added_later' AND action = 'UPDATE'`
	before := auditCount(t, pool, countSQL, table)
	auditExec(t, pool, "UPDATE "+table+" SET added_later = 'x' WHERE id = 1")

	if n := auditCount(t, pool, countSQL, table) - before; n != 1 {
		t.Errorf("a column added after registration wrote %d audit rows, want 1", n)
	}
}

// Migration 0301 registers case_escalation_notification_list, every column recorded.
func TestAuditMigration_CaseEscalationNotificationListIsAudited(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	var excluded, masked []string
	err := pool.QueryRow(context.Background(), `SELECT excluded_columns, masked_columns FROM audit_config
	    WHERE schema_name = $1 AND table_name = 'case_escalation_notification_list' AND enabled`,
		auditSchema(t, pool)).Scan(&excluded, &masked)
	if err != nil {
		t.Fatalf("case_escalation_notification_list is not registered with the audit log: %v", err)
	}
	if len(excluded) != 0 || len(masked) != 0 {
		t.Errorf("excludes %v and masks %v, want every column recorded", excluded, masked)
	}
	if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
	    WHERE tgrelid = 'case_escalation_notification_list'::regclass
	      AND tgname = 'csm_audit_row_change' AND tgenabled = 'A'`); n != 1 {
		t.Errorf("%d enabled-always audit triggers, want 1", n)
	}
}

// Migration 0300 registers the product, deployment and project access
// definition tables, every column recorded.
func TestAuditMigration_ProductAndDeploymentTablesAreAudited(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)
	for _, table := range []string{
		"deployed_product", "deployment", "deployment_attachment", "product", "product_version",
		"product_vulnerability", "project_group", "project_group_role", "project_role", "project_type",
	} {
		var excluded, masked []string
		err := pool.QueryRow(context.Background(), `SELECT excluded_columns, masked_columns FROM audit_config
		    WHERE schema_name = $1 AND table_name = $2 AND enabled`, schema, table).Scan(&excluded, &masked)
		if err != nil {
			t.Errorf("%s is not registered with the audit log: %v", table, err)
			continue
		}
		if len(excluded) != 0 || len(masked) != 0 {
			t.Errorf("%s excludes %v and masks %v, want every column recorded", table, excluded, masked)
		}
		if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
		    WHERE tgrelid = format('%I.%I', $1::text, $2::text)::regclass
		      AND tgname = 'csm_audit_row_change' AND tgenabled = 'A'`, schema, table); n != 1 {
			t.Errorf("%s has %d enabled-always audit triggers, want 1", table, n)
		}
	}
}

// Migration 0299 registers the work item related tables that are wanted in the
// audit log, every column recorded.
func TestAuditMigration_WorkItemRelatedTablesAreAudited(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)
	for _, table := range []string{
		"case_attachment", "case_escalation", "comment", "comment_attachment", "customer_call",
		"sla", "time_card", "work_item_attachment", "work_item_feedback", "work_item_watcher",
	} {
		var excluded, masked []string
		err := pool.QueryRow(context.Background(), `SELECT excluded_columns, masked_columns FROM audit_config
		    WHERE schema_name = $1 AND table_name = $2 AND enabled`, schema, table).Scan(&excluded, &masked)
		if err != nil {
			t.Errorf("%s is not registered with the audit log: %v", table, err)
			continue
		}
		if len(excluded) != 0 || len(masked) != 0 {
			t.Errorf("%s excludes %v and masks %v, want every column recorded", table, excluded, masked)
		}
		if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
		    WHERE tgrelid = format('%I.%I', $1::text, $2::text)::regclass
		      AND tgname = 'csm_audit_row_change' AND tgenabled = 'A'`, schema, table); n != 1 {
			t.Errorf("%s has %d enabled-always audit triggers, want 1", table, n)
		}
	}
}

// Migration 0298 registers the core people, access and customer tables. Only
// account.sync_time_stamp (rewritten by every sync run) is left out.
func TestAuditMigration_CoreTablesAreAudited(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)
	for table, wantExcluded := range map[string]string{
		"account": "sync_time_stamp", "account_contact": "", "project": "", "project_contact": "",
		"project_contact_group": "", "user": "", "role": "", "user_role": "",
	} {
		var excluded []string
		var masked []string
		err := pool.QueryRow(context.Background(), `SELECT excluded_columns, masked_columns FROM audit_config
		    WHERE schema_name = $1 AND table_name = $2 AND enabled`, schema, table).Scan(&excluded, &masked)
		if err != nil {
			t.Errorf("%s is not registered with the audit log: %v", table, err)
			continue
		}
		if got := strings.Join(excluded, ","); got != wantExcluded || len(masked) != 0 {
			t.Errorf("%s excludes %q and masks %v, want exclude %q and no mask", table, got, masked, wantExcluded)
		}
		if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
		    WHERE tgrelid = format('%I.%I', $1::text, $2::text)::regclass
		      AND tgname = 'csm_audit_row_change' AND tgenabled = 'A'`, schema, table); n != 1 {
			t.Errorf("%s has %d enabled-always audit triggers, want 1", table, n)
		}
	}
}

func TestAuditMigration_PartitionCopiesOfTheTriggerAreNotReported(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	const table = "zz_audit_test_parent"
	auditTestTable(t, pool, table)
	auditExec(t, pool, "CREATE TABLE "+table+" (id int PRIMARY KEY) PARTITION BY RANGE (id)")
	auditExec(t, pool, "CREATE TABLE "+table+"_1 PARTITION OF "+table+" FOR VALUES FROM (0) TO (100)")
	auditExec(t, pool, "SELECT register_audit('"+auditSchema(t, pool)+"', '"+table+"')")

	if n := auditCount(t, pool, "SELECT count(*) FROM audit_reconcile(false) WHERE table_name LIKE $1", table+"%"); n != 0 {
		t.Errorf("reconcile reports %d problem(s) for a registered partitioned table, want 0", n)
	}
}

// retentionPartitions makes two monthly partitions around the 18-month cutoff,
// each holding one row: "expired" ended exactly 18 months ago, "recent" ended
// 17 months ago. Both are dropped again after the test.
func retentionPartitions(t *testing.T, pool *pgxpool.Pool) (expired, recent string) {
	t.Helper()
	expired, recent = "zz_audit_retention_expired", "zz_audit_retention_recent"
	drop := func() {
		for _, name := range []string{expired, recent} {
			_, _ = pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+name)
		}
	}
	drop()
	t.Cleanup(drop)

	cutoff := `(date_trunc('month', now() AT TIME ZONE 'UTC') - make_interval(months => 18)) AT TIME ZONE 'UTC'`
	for _, p := range []struct{ name, from, to string }{
		{expired, cutoff + " - interval '1 month'", cutoff},
		{recent, cutoff, cutoff + " + interval '1 month'"},
	} {
		var from, to string
		if err := pool.QueryRow(context.Background(), "SELECT ("+p.from+")::text, ("+p.to+")::text").Scan(&from, &to); err != nil {
			t.Fatalf("bounds for %s: %v", p.name, err)
		}
		auditExec(t, pool, "CREATE TABLE "+p.name+" PARTITION OF csm_audit_log FOR VALUES FROM ('"+from+"') TO ('"+to+"')")
		auditExec(t, pool, `INSERT INTO csm_audit_log (created_on, schema_name, table_name, record_id, field_name, action, created_by, db_role, transaction_id)
		    VALUES ($1::timestamptz, 'public', 'zz_retention', '1', 'f', 'UPDATE', 'tester', 'tester', 1)`, from)
	}
	return expired, recent
}

func TestAuditMigration_RetirePartitionsDeletesTheExpiredAndKeepsTheRest(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	expired, recent := retentionPartitions(t, pool)

	got, err := repo.RetirePartitions(context.Background(), 18)
	if err != nil {
		t.Fatalf("RetirePartitions: %v", err)
	}

	names := strings.Join(got, " ")
	if !strings.Contains(names, expired) {
		t.Errorf("retired %q, want it to include %s (ended exactly 18 months ago)", names, expired)
	}
	if strings.Contains(names, recent) {
		t.Errorf("retired %q, which includes %s (ended 17 months ago)", names, recent)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE relname = $1", expired); n != 0 {
		t.Errorf("%s still exists: it must be dropped, not only detached", expired)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = 'zz_retention'"); n != 1 {
		t.Errorf("zz_retention rows = %d, want 1: only the row in the recent partition survives", n)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE relname = 'csm_audit_log_default'"); n != 1 {
		t.Error("the catch-all partition must never be deleted")
	}
}

func TestAuditMigration_RetirePartitionsIsRepeatable(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	expired, _ := retentionPartitions(t, pool)

	if _, err := repo.RetirePartitions(context.Background(), 18); err != nil {
		t.Fatalf("first run: %v", err)
	}
	again, err := repo.RetirePartitions(context.Background(), 18)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if strings.Contains(strings.Join(again, " "), expired) {
		t.Errorf("second run retired %s again", expired)
	}
}

// A longer retention keeps what a shorter one deletes.
func TestAuditMigration_RetirePartitionsFollowsTheRetentionPeriod(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	expired, _ := retentionPartitions(t, pool)

	if got, err := repo.RetirePartitions(context.Background(), 24); err != nil {
		t.Fatalf("RetirePartitions(24): %v", err)
	} else if strings.Contains(strings.Join(got, " "), expired) {
		t.Errorf("a 24-month retention deleted %s, which ended 18 months ago", expired)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE relname = $1", expired); n != 1 {
		t.Errorf("%s was deleted by a 24-month retention", expired)
	}
}

// Zero or negative puts the cutoff in the future and would delete every
// partition, so the function refuses it.
func TestAuditMigration_RetirePartitionsRefusesAnUnsafeRetention(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	expired, recent := retentionPartitions(t, pool)

	for _, months := range []int{0, -1} {
		if _, err := repo.RetirePartitions(context.Background(), months); err == nil {
			t.Errorf("RetirePartitions(%d) succeeded, want an error", months)
		}
	}
	for _, name := range []string{expired, recent} {
		if n := auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE relname = $1", name); n != 1 {
			t.Errorf("%s was deleted by a refused call", name)
		}
	}
}

// resetDefaultPartition swaps the catch-all partition for an empty one, the only
// way to clear it (the log is append-only). The purge tests leave rows in it,
// and a row there is a health problem for every other test.
func resetDefaultPartition(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	reset := func() {
		ctx := context.Background()
		for _, sql := range []string{
			"ALTER TABLE csm_audit_log DETACH PARTITION csm_audit_log_default",
			"DROP TABLE csm_audit_log_default",
			"CREATE TABLE csm_audit_log_default PARTITION OF csm_audit_log DEFAULT",
			"CREATE TRIGGER csm_audit_log_no_truncate BEFORE TRUNCATE ON csm_audit_log_default FOR EACH STATEMENT EXECUTE FUNCTION audit_log_reject_change()",
		} {
			if _, err := pool.Exec(ctx, sql); err != nil {
				t.Fatalf("reset default partition: %s: %v", sql, err)
			}
		}
	}
	reset()
	t.Cleanup(reset)
}

// defaultRow puts one row into the catch-all partition, created monthsAgo months
// ago. 40 and 3 months back have no partition in a database built by the
// migration, so both land in the catch-all.
func defaultRow(t *testing.T, pool *pgxpool.Pool, recordID string, monthsAgo int) {
	t.Helper()
	auditExec(t, pool, `INSERT INTO csm_audit_log (created_on, schema_name, table_name, record_id, field_name, action, created_by, db_role, transaction_id)
	    VALUES (now() - make_interval(months => $2), 'public', 'zz_default_purge', $1, 'f', 'UPDATE', 'tester', 'tester', 1)`, recordID, monthsAgo)
}

func TestAuditMigration_PurgeDefaultPartitionDeletesOnlyTheExpiredRows(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	resetDefaultPartition(t, pool)
	defaultRow(t, pool, "old-1", 40)
	defaultRow(t, pool, "old-2", 19)
	defaultRow(t, pool, "young", 3)

	deleted, err := repo.PurgeDefaultPartition(context.Background(), 18)
	if err != nil {
		t.Fatalf("PurgeDefaultPartition: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2 (the rows 40 and 19 months old)", deleted)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = 'zz_default_purge'"); n != 1 {
		t.Errorf("zz_default_purge rows = %d, want 1", n)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = 'zz_default_purge' AND record_id = 'young'"); n != 1 {
		t.Error("the row from 3 months ago must survive")
	}
}

// The catch-all is replaced by a new table, so it must still be the attached
// default partition, keep its TRUNCATE guard, and stay append-only.
func TestAuditMigration_PurgeDefaultPartitionLeavesTheCatchAllIntact(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	resetDefaultPartition(t, pool)
	defaultRow(t, pool, "old", 40)
	defaultRow(t, pool, "young", 3)

	if _, err := repo.PurgeDefaultPartition(context.Background(), 18); err != nil {
		t.Fatalf("PurgeDefaultPartition: %v", err)
	}

	if n := auditCount(t, pool, `SELECT count(*) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
	    WHERE i.inhparent = 'csm_audit_log'::regclass AND c.relname = 'csm_audit_log_default'
	      AND pg_get_expr(c.relpartbound, c.oid) = 'DEFAULT'`); n != 1 {
		t.Errorf("csm_audit_log_default is not the attached default partition (%d)", n)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE relname = 'csm_audit_log_default_purge'"); n != 0 {
		t.Error("the table swapped out was not dropped")
	}
	if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
	    WHERE tgrelid = 'csm_audit_log_default'::regclass AND tgname = 'csm_audit_log_no_truncate'`); n != 1 {
		t.Error("the new catch-all has lost its TRUNCATE guard")
	}
	if _, err := pool.Exec(context.Background(), "DELETE FROM csm_audit_log_default"); err == nil {
		t.Error("a DELETE on the new catch-all succeeded: the log must stay append-only")
	}
}

// With nothing past retention it changes nothing and swaps nothing.
func TestAuditMigration_PurgeDefaultPartitionWithNothingExpiredChangesNothing(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	resetDefaultPartition(t, pool)
	defaultRow(t, pool, "young", 3)
	var before uint32
	if err := pool.QueryRow(context.Background(), "SELECT 'csm_audit_log_default'::regclass::oid").Scan(&before); err != nil {
		t.Fatalf("oid: %v", err)
	}

	deleted, err := repo.PurgeDefaultPartition(context.Background(), 18)
	if err != nil {
		t.Fatalf("PurgeDefaultPartition: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
	var after uint32
	if err := pool.QueryRow(context.Background(), "SELECT 'csm_audit_log_default'::regclass::oid").Scan(&after); err != nil {
		t.Fatalf("oid: %v", err)
	}
	if before != after {
		t.Error("the catch-all table was replaced although nothing was past retention")
	}
}

func TestAuditMigration_PurgeDefaultPartitionIsRepeatable(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	resetDefaultPartition(t, pool)
	defaultRow(t, pool, "old", 40)

	if deleted, err := repo.PurgeDefaultPartition(context.Background(), 18); err != nil || deleted != 1 {
		t.Fatalf("first run = (%d, %v), want (1, nil)", deleted, err)
	}
	if deleted, err := repo.PurgeDefaultPartition(context.Background(), 18); err != nil || deleted != 0 {
		t.Errorf("second run = (%d, %v), want (0, nil)", deleted, err)
	}
}

func TestAuditMigration_PurgeDefaultPartitionRefusesAnUnsafeRetention(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	resetDefaultPartition(t, pool)
	defaultRow(t, pool, "young", 3)

	for _, months := range []int{0, -1} {
		if _, err := repo.PurgeDefaultPartition(context.Background(), months); err == nil {
			t.Errorf("PurgeDefaultPartition(%d) succeeded, want an error", months)
		}
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = 'zz_default_purge'"); n != 1 {
		t.Errorf("rows = %d after refused calls, want 1", n)
	}
}

func auditHealth(t *testing.T, repo repository.AuditLogRepository, monthsAhead int) string {
	t.Helper()
	problems, err := repo.PartitionHealth(context.Background(), monthsAhead)
	if err != nil {
		t.Fatalf("PartitionHealth(%d): %v", monthsAhead, err)
	}
	return strings.Join(problems, " | ")
}

// The daily run creates monthsAhead months, so a missed one is reported the day
// it is missed, not once the buffer is down to two.
func TestAuditMigration_HealthReportsAMissedFuturePartition(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	if _, err := repo.CreatePartitions(context.Background(), 3); err != nil {
		t.Fatalf("create: %v", err)
	}

	// However many future months the database holds, asking for that many is
	// healthy and asking for one more is one short of the configured buffer.
	have := auditCount(t, pool, `SELECT count(*) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
	    WHERE i.inhparent = 'csm_audit_log'::regclass AND c.relname ~ '^csm_audit_log_\d{4}_\d{2}$'
	      AND to_date(substr(c.relname, 15), 'YYYY_MM') > date_trunc('month', now() AT TIME ZONE 'UTC')::date`)
	if got := auditHealth(t, repo, have); got != "" {
		t.Errorf("health with %d months ahead = %q, want healthy", have, got)
	}
	if got := auditHealth(t, repo, have+1); !strings.Contains(got, "future monthly partition") {
		t.Errorf("health with %d months ahead = %q, want a missing-partition problem", have+1, got)
	}
	// The monitoring call with no arguments keeps its old minimum of 2.
	if n := auditCount(t, pool, "SELECT count(*) FROM audit_partition_health()"); n != 0 {
		t.Errorf("audit_partition_health() = %d problems, want 0", n)
	}
}

// A missing month inside the horizon is reported even when a partition further
// out exists: counting partitions would let the distant one cover for it.
func TestAuditMigration_HealthReportsAMissingMonthDespiteADistantPartition(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	ctx := context.Background()
	if _, err := repo.CreatePartitions(ctx, 3); err != nil {
		t.Fatalf("create: %v", err)
	}

	var next, bound, from, to string
	if err := pool.QueryRow(ctx, `SELECT
	    'csm_audit_log_' || to_char(date_trunc('month', now() AT TIME ZONE 'UTC') + interval '1 month', 'YYYY_MM'),
	    ((date_trunc('month', now() AT TIME ZONE 'UTC') + interval '40 months') AT TIME ZONE 'UTC')::text,
	    ((date_trunc('month', now() AT TIME ZONE 'UTC') + interval '41 months') AT TIME ZONE 'UTC')::text`).Scan(&next, &from, &to); err != nil {
		t.Fatalf("names: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT pg_get_expr(relpartbound, oid) FROM pg_class WHERE relname = $1`, next).Scan(&bound); err != nil {
		t.Fatalf("bound of %s: %v", next, err)
	}

	const distant = "zz_audit_health_distant"
	restore := func() {
		_, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS "+distant)
		var attached int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		    WHERE i.inhparent = 'csm_audit_log'::regclass AND c.relname = $1`, next).Scan(&attached)
		if attached == 0 {
			_, _ = pool.Exec(ctx, "ALTER TABLE csm_audit_log ATTACH PARTITION "+next+" "+bound)
		}
	}
	t.Cleanup(restore)
	restore()

	if got := auditHealth(t, repo, 3); got != "" {
		t.Fatalf("health before the test = %q, want healthy", got)
	}
	auditExec(t, pool, "ALTER TABLE csm_audit_log DETACH PARTITION "+next)
	auditExec(t, pool, "CREATE TABLE "+distant+" PARTITION OF csm_audit_log FOR VALUES FROM ('"+from+"') TO ('"+to+"')")

	got := auditHealth(t, repo, 3)
	if !strings.Contains(got, "missing future monthly partition") || !strings.Contains(got, strings.TrimPrefix(next, "csm_audit_log_")) {
		t.Errorf("health = %q, want the missing month %s reported although a distant partition exists", got, next)
	}
}

// Deleting waits on the same advisory lock as creating partitions. The lock
// timeout has to cover that wait too: with the timeout set after the lock, a run
// queued behind another one waits as long as the other holds it.
func TestAuditMigration_DeletionGivesUpWhileAnotherRunHoldsTheLock(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	ctx := context.Background()

	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('csm_audit_log_partitions', 0))"); err != nil {
		t.Fatalf("take the lock: %v", err)
	}

	for _, call := range []string{
		"SELECT audit_retire_partitions(18, '300ms')",
		"SELECT audit_purge_default_partition(18, '300ms')",
	} {
		t.Run(call, func(t *testing.T) {
			// Far longer than the lock timeout: a call that ignores it only ends
			// when this deadline cancels it, with a different error.
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			started := time.Now()
			_, err := pool.Exec(callCtx, call)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
				t.Fatalf("%s = %v, want a lock timeout (55P03)", call, err)
			}
			if took := time.Since(started); took > 3*time.Second {
				t.Errorf("%s gave up after %s, want about the 300ms timeout", call, took)
			}
		})
	}
}
