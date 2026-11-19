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

// Who created and last changed a row (created_by, updated_by, audit_actor())
// against a real Postgres 18. Skipped without ENTITY_TEST_DATABASE_URL.

package repository_test

import (
	"context"
	"strings"
	"testing"
)

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

// Inserts are not in the audit log, so created_by is the only record of who added
// a row: it is mandatory on the access tables, and defaults to the actor so an
// insert that leaves it out is stamped, not rejected.
func TestAuditMigration_CreatedByIsMandatoryOnTheAccessTables(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	schema := auditSchema(t, pool)
	for _, table := range []string{"role", "team", "team_member", "user", "user_role"} {
		var nullable, columnDefault string
		err := pool.QueryRow(context.Background(), `SELECT is_nullable, COALESCE(column_default, '')
		    FROM information_schema.columns
		    WHERE table_schema = $1 AND table_name = $2 AND column_name = 'created_by'`, schema, table).Scan(&nullable, &columnDefault)
		if err != nil {
			t.Errorf("%s.created_by: %v", table, err)
			continue
		}
		if nullable != "NO" {
			t.Errorf("%s.created_by is nullable, want NOT NULL", table)
		}
		if !strings.Contains(columnDefault, "audit_actor()") {
			t.Errorf("%s.created_by default = %q, want audit_actor()", table, columnDefault)
		}
	}
}
