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
	"strings"
	"testing"

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
	audited := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = $1", table)
	if audited == 0 {
		t.Fatal("registered table wrote no audit rows")
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
