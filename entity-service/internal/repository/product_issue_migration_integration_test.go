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

// Migrations 0303-0305 and 100029: product issues (work item type PRODUCT_ISSUE,
// its tables, the internal-only row-level security). Run against a real Postgres
// with those migrations applied; skipped without ENTITY_TEST_DATABASE_URL.
//
// Every test works inside one transaction that is rolled back, so nothing is
// left behind. Row-level security only applies to a role that does not bypass
// it, so the visibility checks switch to the non-superuser csm_app with
// SET LOCAL ROLE (granting it the new tables first, inside the same
// transaction) and set app.is_internal / app.viewer_email the way the service
// does. A test that needs csm_app skips when the role does not exist.

package repository_test

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var productIssueNewTables = []string{
	"product_issue", "github_issue_link", "work_item_link", "work_item_status_event",
	"comment_github_ref", "board_status_map", "work_item_creation_request",
}

func piTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	url := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return ctx, tx
}

func piExec(t *testing.T, ctx context.Context, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec failed: %v\n%s", err, sql)
	}
}

func piScalar[T any](t *testing.T, ctx context.Context, tx pgx.Tx, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := tx.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
		t.Fatalf("query failed: %v\n%s", err, sql)
	}
	return v
}

// piExpectError runs sql in a savepoint and returns the Postgres error (nil
// when it succeeded), leaving the outer transaction usable.
func piExpectError(ctx context.Context, tx pgx.Tx, sql string, args ...any) *pgconn.PgError {
	sp, err := tx.Begin(ctx)
	if err != nil {
		panic(err)
	}
	defer func() { _ = sp.Rollback(ctx) }()
	_, err = sp.Exec(ctx, sql, args...)
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if e, ok := err.(*pgconn.PgError); ok {
		pgErr = e
	} else {
		pgErr = &pgconn.PgError{Message: err.Error()}
	}
	return pgErr
}

func piExpectUnique(t *testing.T, ctx context.Context, tx pgx.Tx, wantConstraintOrIndex, sql string, args ...any) {
	t.Helper()
	e := piExpectError(ctx, tx, sql, args...)
	if e == nil {
		t.Fatalf("expected a unique violation on %s, got none: %s", wantConstraintOrIndex, sql)
	}
	if e.Code != "23505" || e.ConstraintName != wantConstraintOrIndex {
		t.Fatalf("got %s (%s) on %q, want 23505 on %s", e.Code, e.Message, e.ConstraintName, wantConstraintOrIndex)
	}
}

// piNewWorkItem inserts a work_item and returns its id.
func piNewWorkItem(t *testing.T, ctx context.Context, tx pgx.Tx, typ, subject string, projectID any) string {
	t.Helper()
	// Case-like types must carry a wso2_id (work_item_wso2_id_required_by_type).
	return piScalar[string](t, ctx, tx, `
		INSERT INTO work_item (created_on, updated_on, created_by, updated_by, number, subject, type, project_id, wso2_id)
		VALUES (NOW(), NOW(), 'test', 'test', next_work_item_number($1::work_item_type_enum), $2, $1::work_item_type_enum, $3::uuid,
		        CASE WHEN $1 = 'CASE' THEN 'PI-TEST-' || substr(md5(random()::text), 1, 10) END)
		RETURNING id::text`, typ, subject, projectID)
}

func TestProductIssueNumberSeries(t *testing.T) {
	ctx, tx := piTx(t)

	pi := piScalar[string](t, ctx, tx, `SELECT next_work_item_number('PRODUCT_ISSUE')`)
	if !regexp.MustCompile(`^PI\d{7}$`).MatchString(pi) {
		t.Errorf("PRODUCT_ISSUE number = %q, want PI + 7 digits", pi)
	}
	pi2 := piScalar[string](t, ctx, tx, `SELECT next_work_item_number('PRODUCT_ISSUE')`)
	if pi2 <= pi {
		t.Errorf("second PRODUCT_ISSUE number %q is not after %q", pi2, pi)
	}
	if cs := piScalar[string](t, ctx, tx, `SELECT next_work_item_number('CASE')`); !regexp.MustCompile(`^CS\d{7}$`).MatchString(cs) {
		t.Errorf("CASE number = %q, want CS + 7 digits", cs)
	}
	if scp := piScalar[string](t, ctx, tx, `SELECT next_series_number('SCP')`); !regexp.MustCompile(`^SCP\d{7}$`).MatchString(scp) {
		t.Errorf("SCP number = %q, want SCP + 7 digits", scp)
	}

	// Every type that had a series before keeps it; a type with none still has none.
	want := map[string]*string{
		"CASE": piStr("CS"), "SERVICE_REQUEST": piStr("CS"), "SECURITY_REPORT_ANALYSIS": piStr("CS"),
		"ANNOUNCEMENT": piStr("CS"), "ENGAGEMENT": piStr("CS"), "INCIDENT": piStr("INC"),
		"PROBLEM": piStr("PRB"), "CHANGE_REQUEST": piStr("CHG"), "INCIDENT_TASK": piStr("TASK"),
		"CHANGE_TASK": piStr("CTASK"), "PROBLEM_TASK": piStr("PTASK"), "INCIDENT_ALERT_TASK": piStr("ICT"),
		"CONVERSATION": piStr("CHAT"), "PRODUCT_ISSUE": piStr("PI"), "STANDARD_CHANGE_PROPOSAL": nil,
	}
	for typ, series := range want {
		var got *string
		if err := tx.QueryRow(ctx, `SELECT work_item_number_series($1::work_item_type_enum)`, typ).Scan(&got); err != nil {
			t.Fatalf("series for %s: %v", typ, err)
		}
		if (got == nil) != (series == nil) || (got != nil && *got != *series) {
			t.Errorf("series for %s = %v, want %v", typ, piDeref(got), piDeref(series))
		}
	}
}

func piStr(s string) *string { return &s }
func piDeref(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}

func TestProductIssueUniqueConstraints(t *testing.T) {
	ctx, tx := piTx(t)
	pi := piNewWorkItem(t, ctx, tx, "PRODUCT_ISSUE", "pi one", nil)
	pi2 := piNewWorkItem(t, ctx, tx, "PRODUCT_ISSUE", "pi two", nil)
	cs := piNewWorkItem(t, ctx, tx, "CASE", "case one", nil)
	cs2 := piNewWorkItem(t, ctx, tx, "CASE", "case two", nil)

	// github_issue_link: owner/repository case-insensitive, and node_id.
	piExec(t, ctx, tx, `INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id)
		VALUES ($1, 'Acme-Org', 'Product-Repo', 7, 'I_node1')`, pi)
	piExpectUnique(t, ctx, tx, "github_issue_link_issue_key",
		`INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id)
		 VALUES ($1, 'acme-org', 'PRODUCT-REPO', 7, 'I_node2')`, pi2)
	piExpectUnique(t, ctx, tx, "github_issue_link_node_id_key",
		`INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id)
		 VALUES ($1, 'acme-org', 'product-repo', 8, 'I_node1')`, pi2)
	piExec(t, ctx, tx, `INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id)
		VALUES ($1, 'acme-org', 'product-repo', 8, 'I_node3')`, pi2)

	// work_item_link: one active AFFECTS per from_id; an inactive one does not count.
	piExec(t, ctx, tx, `INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by)
		VALUES ($1, $2, 'AFFECTS', 'CREATION', 'test', 'test')`, pi, cs)
	piExpectUnique(t, ctx, tx, "work_item_link_one_active_affects_key",
		`INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by)
		 VALUES ($1, $2, 'AFFECTS', 'MANUAL', 'test', 'test')`, pi, cs2)
	piExec(t, ctx, tx, `UPDATE work_item_link SET is_active = false WHERE from_id = $1`, pi)
	piExec(t, ctx, tx, `INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by)
		VALUES ($1, $2, 'AFFECTS', 'MANUAL', 'test', 'test')`, pi, cs2)
	if e := piExpectError(ctx, tx, `INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by)
		VALUES ($1, $2, 'BLOCKS', 'MANUAL', 'test', 'test')`, pi2, cs); e == nil || e.Code != "23514" {
		t.Errorf("an unknown link kind must violate the CHECK, got %v", e)
	}

	// work_item_status_event: dedupe_key.
	ev := `INSERT INTO work_item_status_event (work_item_id, to_status, waits_on, occurred_at, source, dedupe_key)
		VALUES ($1, 'OPEN', 'PRODUCT_TEAM', NOW(), 'WEBHOOK', $2)`
	piExec(t, ctx, tx, ev, pi, "delivery-1")
	piExpectUnique(t, ctx, tx, "work_item_status_event_dedupe_key", ev, pi2, "delivery-1")
	piExec(t, ctx, tx, ev, pi, "delivery-2")

	// work_item_creation_request: (case_id, idempotency_key).
	cr := `INSERT INTO work_item_creation_request (idempotency_key, case_id, request_hash, kind, created_by, updated_by)
		VALUES ($2, $1, 'h', 'PRODUCT_ISSUE', 'test', 'test')`
	piExec(t, ctx, tx, cr, cs, "key-1")
	piExpectUnique(t, ctx, tx, "work_item_creation_request_case_key", cr, cs, "key-1")
	piExec(t, ctx, tx, cr, cs, "key-2")
	piExec(t, ctx, tx, cr, cs2, "key-1")
}

func TestProductIssueBoardStatusMapSeed(t *testing.T) {
	ctx, tx := piTx(t)
	type row struct {
		normalized, waitsOn string
		accrues, terminal   bool
	}
	want := map[string]row{
		"  open ":     {"OPEN", "PRODUCT_TEAM", true, false},
		"IN PROGRESS": {"IN_PROGRESS", "PRODUCT_TEAM", true, false},
		"wow":         {"WAITING_ON_PRODUCT_TEAM", "PRODUCT_TEAM", true, false},
		" Woc":        {"WAITING_ON_CS", "CS", false, false},
		"RESOLVED  ":  {"RESOLVED", "NOBODY", false, true},
		"reopened":    {"REOPENED", "PRODUCT_TEAM", true, false},
	}
	for raw, w := range want {
		var g row
		err := tx.QueryRow(ctx, `SELECT normalized_status, waits_on, accrues_sla, is_terminal
			FROM board_status_map WHERE lower(btrim(raw_status)) = lower(btrim($1))`, raw).
			Scan(&g.normalized, &g.waitsOn, &g.accrues, &g.terminal)
		if err != nil {
			t.Errorf("lookup %q: %v", raw, err)
			continue
		}
		if g != w {
			t.Errorf("lookup %q = %+v, want %+v", raw, g, w)
		}
	}
	if n := piScalar[int](t, ctx, tx, `SELECT count(*) FROM board_status_map WHERE accrues_sla = false AND NOT is_terminal`); n != 1 {
		t.Errorf("%d non-terminal columns pause the SLA clock, want exactly 1 (WOC)", n)
	}
	piExpectUnique(t, ctx, tx, "board_status_map_raw_status_key",
		`INSERT INTO board_status_map (raw_status, normalized_status, waits_on, accrues_sla) VALUES (' WOW ', 'OTHER', 'CS', true)`)
}

// A comment on a product issue must not reach GitHub: the issue link lives in
// github_issue_link, never in work_item.github_*, which is what
// trg_github_outbound_comment reads. The case below, with those columns set,
// shows the trigger is live in this database, so the empty queue for the
// product issue means something.
func TestProductIssueCommentEnqueuesNothing(t *testing.T) {
	ctx, tx := piTx(t)
	pi := piNewWorkItem(t, ctx, tx, "PRODUCT_ISSUE", "pi", nil)
	piExec(t, ctx, tx, `INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id)
		VALUES ($1, 'acme-org', 'product-repo', 41, 'I_queue1')`, pi)

	queued := func(id string) int {
		return piScalar[int](t, ctx, tx, `SELECT count(*) FROM github_outbound_queue WHERE work_item_id = $1`, id)
	}
	piExec(t, ctx, tx, `INSERT INTO comment (created_on, created_by, work_item_id, content, type) VALUES (NOW(), 'test', $1, 'hello', 'COMMENT')`, pi)
	if n := queued(pi); n != 0 {
		t.Fatalf("a comment on a product issue enqueued %d github_outbound_queue rows, want 0", n)
	}

	linked := piNewWorkItem(t, ctx, tx, "CASE", "linked case", nil)
	piExec(t, ctx, tx, `UPDATE work_item SET github_owner = 'acme-org', github_repository = 'cust-repo', github_issue_number = 9 WHERE id = $1`, linked)
	piExec(t, ctx, tx, `INSERT INTO comment (created_on, created_by, work_item_id, content, type) VALUES (NOW(), 'test', $1, 'hello', 'COMMENT')`, linked)
	if n := queued(linked); n != 1 {
		t.Fatalf("control: a comment on a case with a GitHub issue enqueued %d rows, want 1 (the trigger is not live, so this test proves nothing)", n)
	}
}

// piAsApp switches the transaction to csm_app with the given session settings.
// is_project_member reads app.viewer_project_ids (a uuid[] literal), so that is
// what a customer session sets.
func piAsApp(t *testing.T, ctx context.Context, tx pgx.Tx, internal bool, viewerEmail, projectIDs string) {
	t.Helper()
	piExec(t, ctx, tx, `SELECT set_config('app.is_internal', $1, true), set_config('app.viewer_email', $2, true),
		set_config('app.viewer_project_ids', $3, true)`,
		map[bool]string{true: "true", false: "false"}[internal], viewerEmail, projectIDs)
	piExec(t, ctx, tx, `SET LOCAL ROLE csm_app`)
}

func piBackToSuper(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	piExec(t, ctx, tx, `RESET ROLE`)
}

func TestProductIssueRowLevelSecurity(t *testing.T) {
	ctx, tx := piTx(t)
	if piScalar[int](t, ctx, tx, `SELECT count(*) FROM pg_roles WHERE rolname = 'csm_app'`) == 0 {
		t.Skip("role csm_app does not exist")
	}
	if piScalar[bool](t, ctx, tx, `SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = 'csm_app'`) {
		t.Skip("csm_app bypasses row-level security")
	}

	// Fixture: a project with one registered contact, a case and a product issue
	// in it (the product issue copies the case's project_id), each with a comment,
	// an activity row and a watcher; a user for the watcher.
	account := piScalar[string](t, ctx, tx, `INSERT INTO account (created_on, updated_on, created_by, updated_by, name, number)
		VALUES (NOW(), NOW(), 'test', 'test', 'PI RLS account', 'PIRLS-ACCT') RETURNING id::text`)
	project := piScalar[string](t, ctx, tx, `INSERT INTO project (created_on, updated_on, created_by, updated_by, key)
		VALUES (NOW(), NOW(), 'test', 'test', 'PIRLSKEY') RETURNING id::text`)
	contact := piScalar[string](t, ctx, tx, `INSERT INTO account_contact (created_on, updated_on, created_by, updated_by, user_name, account_id)
		VALUES (NOW(), NOW(), 'test', 'test', 'pi-rls-customer', $1) RETURNING id::text`, account)
	const customerEmail = "pi-rls-customer@example.test"
	piExec(t, ctx, tx, `INSERT INTO project_contact (created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		VALUES (NOW(), NOW(), 'test', 'test', $1, $2, $3, 'REGISTERED')`, customerEmail, contact, project)
	user := piScalar[string](t, ctx, tx, `INSERT INTO "user" (created_on, updated_on, user_name) VALUES (NOW(), NOW(), 'pi-rls-user') RETURNING id::text`)

	caseID := piNewWorkItem(t, ctx, tx, "CASE", "customer case", project)
	piID := piNewWorkItem(t, ctx, tx, "PRODUCT_ISSUE", "internal product issue", project)
	piExec(t, ctx, tx, `INSERT INTO product_issue (id, priority) VALUES ($1, 'P2')`, piID)
	piExec(t, ctx, tx, `INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id) VALUES ($1, 'o', 'r', 1, 'I_rls')`, piID)
	piExec(t, ctx, tx, `INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by) VALUES ($1, $2, 'AFFECTS', 'CREATION', 't', 't')`, piID, caseID)
	piExec(t, ctx, tx, `INSERT INTO work_item_status_event (work_item_id, to_status, waits_on, occurred_at, source, dedupe_key) VALUES ($1, 'OPEN', 'PRODUCT_TEAM', NOW(), 'WEBHOOK', 'rls-1')`, piID)
	piExec(t, ctx, tx, `INSERT INTO work_item_creation_request (idempotency_key, case_id, request_hash, kind, created_by, updated_by) VALUES ('rls', $1, 'h', 'PRODUCT_ISSUE', 't', 't')`, caseID)
	for _, id := range []string{caseID, piID} {
		commentID := piScalar[string](t, ctx, tx, `INSERT INTO comment (created_on, created_by, work_item_id, content) VALUES (NOW(), 'test', $1, 'c') RETURNING id::text`, id)
		piExec(t, ctx, tx, `INSERT INTO work_item_activity (created_on, created_by, work_item_id) VALUES (NOW(), 'test', $1)`, id)
		piExec(t, ctx, tx, `INSERT INTO work_item_watcher (work_item_id, user_id) VALUES ($1, $2)`, id, user)
		if id == piID {
			piExec(t, ctx, tx, `INSERT INTO comment_github_ref (comment_id, github_comment_id, origin) VALUES ($1, 99, 'PORTAL')`, commentID)
		}
	}

	// The role needs table privileges; the migrations do not grant them.
	piExec(t, ctx, tx, `GRANT SELECT, INSERT, UPDATE, DELETE ON `+strings.Join(productIssueNewTables, ", ")+` TO csm_app`)

	count := func(table string, where string, args ...any) int {
		return piScalar[int](t, ctx, tx, `SELECT count(*) FROM `+table+` WHERE `+where, args...)
	}

	// Customer: a project member, not internal.
	piAsApp(t, ctx, tx, false, customerEmail, "{"+project+"}")
	if n := count("work_item", "id = $1", caseID); n != 1 {
		t.Errorf("customer sees %d of its own case rows, want 1", n)
	}
	for _, c := range []struct{ table, col string }{
		{"comment", "work_item_id"}, {"work_item_activity", "work_item_id"}, {"work_item_watcher", "work_item_id"},
	} {
		if n := count(c.table, c.col+" = $1", caseID); n != 1 {
			t.Errorf("customer sees %d %s rows of its own case, want 1", n, c.table)
		}
		if n := count(c.table, c.col+" = $1", piID); n != 0 {
			t.Errorf("customer sees %d %s rows of a product issue, want 0", n, c.table)
		}
	}
	if n := count("work_item", "id = $1", piID); n != 0 {
		t.Errorf("customer sees the product issue work_item row (%d)", n)
	}
	if n := count("work_item", "type = 'PRODUCT_ISSUE'"); n != 0 {
		t.Errorf("customer sees %d PRODUCT_ISSUE work items", n)
	}
	for _, table := range productIssueNewTables {
		if n := count(table, "true"); n != 0 {
			t.Errorf("customer reads %d rows of %s, want 0", n, table)
		}
	}
	// A customer cannot create one either, nor flip an existing row's type. Each
	// refusal must be the row-level security check (42501), not some other error.
	expectRLSRefusal := func(what, sql string, args ...any) {
		t.Helper()
		e := piExpectError(ctx, tx, sql, args...)
		if e == nil {
			t.Errorf("a customer session %s", what)
			return
		}
		if e.Code != "42501" || !strings.Contains(e.Message, "row-level security") {
			t.Errorf("a customer session %s: refused with %s %q, want 42501 row-level security", what, e.Code, e.Message)
		}
	}
	expectRLSRefusal("inserted a PRODUCT_ISSUE work item", `INSERT INTO work_item (created_on, updated_on, created_by, updated_by, number, subject, type, project_id)
		VALUES (NOW(), NOW(), 't', 't', 'PIX-1', 'x', 'PRODUCT_ISSUE', $1)`, project)
	expectRLSRefusal("changed its case into a PRODUCT_ISSUE", `UPDATE work_item SET type = 'PRODUCT_ISSUE' WHERE id = $1`, caseID)
	expectRLSRefusal("inserted a work_item_link", `INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by) VALUES ($1, $2, 'AFFECTS', 'MANUAL', 't', 't')`, caseID, piID)
	piBackToSuper(t, ctx, tx)

	// Internal session: everything.
	piAsApp(t, ctx, tx, true, "", "")
	for _, id := range []string{caseID, piID} {
		if n := count("work_item", "id = $1", id); n != 1 {
			t.Errorf("internal session sees %d work_item rows for %s, want 1", n, id)
		}
		for _, table := range []string{"comment", "work_item_activity", "work_item_watcher"} {
			if n := count(table, "work_item_id = $1", id); n != 1 {
				t.Errorf("internal session sees %d %s rows for %s, want 1", n, table, id)
			}
		}
	}
	for _, table := range productIssueNewTables {
		if n := count(table, "true"); n < 1 {
			t.Errorf("internal session reads no rows of %s", table)
		}
	}
	if n := piScalar[int](t, ctx, tx, `WITH u AS (UPDATE work_item_link SET is_active = false WHERE from_id = $1 RETURNING 1) SELECT count(*) FROM u`, piID); n != 1 {
		t.Errorf("internal session updated %d work_item_link rows, want 1", n)
	}
	piBackToSuper(t, ctx, tx)
}
