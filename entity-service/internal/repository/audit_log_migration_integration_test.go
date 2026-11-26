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
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
		// The migration skips a type table the schema does not have yet.
		if auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE oid = to_regclass(format('%I.%I', $1::text, $2::text))", schema, table) == 0 {
			t.Logf("%s does not exist in this schema; not expected to be registered", table)
			continue
		}
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

// updated_on and updated_by are never logged, on any table: the log row already
// says who changed the record and when, so they would only repeat it.
func TestAuditMigration_UpdatedOnAndUpdatedByAreNeverLogged(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	// The audit log is append-only, so a name used by an earlier run would still
	// have its rows here: every run gets its own.
	table := "zz_audit_test_updated_cols_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	auditTestTable(t, pool, table)
	auditExec(t, pool, "CREATE TABLE "+table+` (id int PRIMARY KEY, note text, updated_on timestamptz, updated_by text)`)
	// Registered with no exclusions of its own: the function leaves them out anyway.
	auditExec(t, pool, "SELECT register_audit('"+auditSchema(t, pool)+"', '"+table+"')")
	auditExec(t, pool, "INSERT INTO "+table+" VALUES (1, 'a', now(), 'someone')")

	fields := func(action string) string {
		t.Helper()
		var got string
		if err := pool.QueryRow(context.Background(),
			`SELECT COALESCE(string_agg(field_name, ',' ORDER BY field_name), '') FROM csm_audit_log WHERE table_name = $1 AND action = $2`,
			table, action).Scan(&got); err != nil {
			t.Fatalf("fields: %v", err)
		}
		return got
	}

	// A change to the business column and to both stamps logs the business column only.
	auditExec(t, pool, "UPDATE "+table+" SET note = 'b', updated_on = now() + interval '1 day', updated_by = 'other' WHERE id = 1")
	if got := fields("UPDATE"); got != "note" {
		t.Errorf("update logged %q, want only note", got)
	}

	// A change that touches only the stamps logs nothing at all.
	before := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = $1", table)
	auditExec(t, pool, "UPDATE "+table+" SET updated_on = now() + interval '2 days', updated_by = 'third' WHERE id = 1")
	if n := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = $1", table) - before; n != 0 {
		t.Errorf("a stamps-only update wrote %d audit rows, want 0", n)
	}

	// A delete logs every non-NULL column except the stamps.
	auditExec(t, pool, "DELETE FROM "+table+" WHERE id = 1")
	if got := fields("DELETE"); got != "id,note" {
		t.Errorf("delete logged %q, want id,note", got)
	}
}

// A trigger that is on the table but is not what register_audit builds (other
// arguments, or one that also fires on INSERT) is reported, and a repair puts the
// registered one back.
func TestAuditMigration_ReconcileReportsATriggerThatDiffersFromItsRegistration(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)
	for name, replacement := range map[string]string{
		"other arguments": "AFTER UPDATE OR DELETE ON %s FOR EACH ROW EXECUTE FUNCTION audit_log_row_change('id', 'note', '')",
		"fires on insert": "AFTER INSERT OR UPDATE OR DELETE ON %s FOR EACH ROW EXECUTE FUNCTION audit_log_row_change('id', '', '')",
	} {
		table := "zz_audit_test_drift_" + strings.ReplaceAll(name, " ", "_")
		auditTestTable(t, pool, table)
		auditExec(t, pool, "CREATE TABLE "+table+" (id int PRIMARY KEY, note text)")
		auditExec(t, pool, "SELECT register_audit('"+schema+"', '"+table+"')")
		if n := auditCount(t, pool, "SELECT count(*) FROM audit_reconcile(false) WHERE table_name = $1", table); n != 0 {
			t.Fatalf("%s: a fresh registration reports %d problem(s), want 0", name, n)
		}

		auditExec(t, pool, "DROP TRIGGER csm_audit_row_change ON "+table)
		auditExec(t, pool, "CREATE TRIGGER csm_audit_row_change "+strings.Replace(replacement, "%s", table, 1))
		auditExec(t, pool, "ALTER TABLE "+table+" ENABLE ALWAYS TRIGGER csm_audit_row_change")

		if n := auditCount(t, pool, "SELECT count(*) FROM audit_reconcile(false) WHERE table_name = $1 AND problem LIKE '%differs%'", table); n != 1 {
			t.Errorf("%s: reconcile reports %d difference(s), want 1", name, n)
		}
		auditExec(t, pool, "SELECT * FROM audit_reconcile(true)")
		if n := auditCount(t, pool, "SELECT count(*) FROM audit_reconcile(false) WHERE table_name = $1", table); n != 0 {
			t.Errorf("%s: %d problem(s) left after a repair, want 0", name, n)
		}
	}
}

func TestAuditMigration_RegisterRefusesColumnsItCannotAuditSafely(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)

	const commaTable = "zz_audit_test_comma"
	auditTestTable(t, pool, commaTable)
	auditExec(t, pool, `CREATE TABLE `+commaTable+` (id int PRIMARY KEY, "a,b" text)`)
	_, err := pool.Exec(context.Background(), "SELECT register_audit('"+schema+"', '"+commaTable+"', NULL, '{}', ARRAY['a,b'])")
	if err == nil || !strings.Contains(err.Error(), "comma") {
		t.Errorf("a masked column named with a comma: err = %v, want a refusal", err)
	}

	const byteaTable = "zz_audit_test_bytea_key"
	auditTestTable(t, pool, byteaTable)
	auditExec(t, pool, "CREATE TABLE "+byteaTable+" (id bytea PRIMARY KEY, note text)")
	_, err = pool.Exec(context.Background(), "SELECT register_audit('"+schema+"', '"+byteaTable+"')")
	if err == nil || !strings.Contains(err.Error(), "binary") {
		t.Errorf("a bytea key: err = %v, want a refusal", err)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM audit_config WHERE table_name = $1", byteaTable); n != 0 {
		t.Errorf("a refused registration left %d registry row(s)", n)
	}
}

// A primary key with an INCLUDE column keys on the key columns only.
func TestAuditMigration_RegisterKeysOnTheKeyColumnsNotIncludedOnes(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	const table = "zz_audit_test_include"
	auditTestTable(t, pool, table)
	auditExec(t, pool, "CREATE TABLE "+table+" (id int, extra text, PRIMARY KEY (id) INCLUDE (extra))")
	auditExec(t, pool, "SELECT register_audit('"+auditSchema(t, pool)+"', '"+table+"')")

	var keys string
	if err := pool.QueryRow(context.Background(),
		"SELECT array_to_string(key_columns, ',') FROM audit_config WHERE table_name = $1", table).Scan(&keys); err != nil {
		t.Fatalf("read key columns: %v", err)
	}
	if keys != "id" {
		t.Errorf("key columns = %q, want id only", keys)
	}
}

// The trigger function is only ever called by a trigger, which needs no grant
// at run time, so PUBLIC gets no EXECUTE on it.
func TestAuditMigration_TriggerFunctionIsNotExecutableByPublic(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	if n := auditCount(t, pool, `SELECT count(*) FROM pg_proc p, aclexplode(p.proacl) a
	    WHERE p.oid = 'audit_log_row_change'::regproc AND a.grantee = 0 AND a.privilege_type = 'EXECUTE'`); n != 0 {
		t.Errorf("audit_log_row_change has %d PUBLIC EXECUTE grant(s), want 0", n)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM pg_proc WHERE oid = 'audit_log_row_change'::regproc AND proacl IS NOT NULL"); n != 1 {
		t.Errorf("audit_log_row_change still has the default (PUBLIC) privileges")
	}
}
