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

package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ---- a scripted querier ----------------------------------------------------

// scriptedRow answers one QueryRow: vals are assigned to the Scan
// destinations in order (a nil value leaves the destination at its zero
// value), or err is returned.
type scriptedRow struct {
	vals []any
	err  error
}

func (r scriptedRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.vals) {
		return errors.New("scriptedRow: scan destination count mismatch")
	}
	for i, d := range dest {
		if r.vals[i] == nil {
			continue
		}
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(r.vals[i]))
	}
	return nil
}

// scriptedRows answers one Query with a fixed result set.
type scriptedRows struct {
	pgx.Rows
	rows [][]any
	i    int
}

func (r *scriptedRows) Next() bool             { r.i++; return r.i <= len(r.rows) }
func (r *scriptedRows) Scan(dest ...any) error { return scriptedRow{vals: r.rows[r.i-1]}.Scan(dest...) }
func (r *scriptedRows) Close()                 {}
func (r *scriptedRows) Err() error             { return nil }

// scriptStep answers the first statement whose SQL contains match; each step
// is used once, in script order among the steps that match.
type scriptStep struct {
	match string
	row   scriptedRow
	rows  [][]any
}

// scriptedQuerier stands in for the transaction: QueryRow and Query consume
// scriptStep entries by SQL substring, Exec records the statement and answers
// with execTags (by substring; "UPDATE 0" when none matches).
type scriptedQuerier struct {
	t        *testing.T
	script   []scriptStep
	execTags map[string]string
	queries  []string
	execs    []recordedExec
}

func (q *scriptedQuerier) take(sql string) scriptStep {
	q.queries = append(q.queries, sql)
	for i, st := range q.script {
		if strings.Contains(sql, st.match) {
			q.script = append(q.script[:i], q.script[i+1:]...)
			return st
		}
	}
	q.t.Fatalf("unscripted statement:\n%s", sql)
	return scriptStep{}
}

func (q *scriptedQuerier) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	return q.take(sql).row
}

func (q *scriptedQuerier) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	return &scriptedRows{rows: q.take(sql).rows}, nil
}

func (q *scriptedQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execs = append(q.execs, recordedExec{sql: sql, args: args})
	for match, tag := range q.execTags {
		if strings.Contains(sql, match) {
			return pgconn.NewCommandTag(tag), nil
		}
	}
	return pgconn.NewCommandTag("UPDATE 0"), nil
}

func (q *scriptedQuerier) execsMatching(sub string) []recordedExec {
	var out []recordedExec
	for _, e := range q.execs {
		if strings.Contains(e.sql, sub) {
			out = append(out, e)
		}
	}
	return out
}

var noRows = scriptedRow{err: pgx.ErrNoRows}

func boolPtr(b bool) *bool { return &b }

func sampleContactUpsert() domain.SalesforceContactUpsert {
	return domain.SalesforceContactUpsert{
		ContactSfID: "003000000000001AAA", Email: "jane@acme.com", Name: "Jane Doe", FirstName: "Jane", LastName: "Doe",
		AccountID: "acct-new", AccountSfID: "001000000000001AAA", IsPrimaryContact: boolPtr(true),
		GlobalRoles:        []string{"external", "customer"},
		ManagedGlobalRoles: []string{"customer", "partner"},
		ManagedAdminRoles:  []string{globalRoleCustomerAdminName, globalRolePartnerAdminName},
		AdminRoleName:      globalRoleCustomerAdminName,
	}
}

// ---- the Contact writer ----------------------------------------------------

// TestUpsertContactTx_NewContact walks a first ingest end to end: a new user,
// the customer role granted and partner revoked (the managed pair), a new
// account_contact carrying Salesforce's primary flag, the account-move sweep,
// and the admin derivation.
func TestUpsertContactTx_NewContact(t *testing.T) {
	q := &scriptedQuerier{t: t, execTags: map[string]string{"SET is_active = FALSE": "UPDATE 1"}, script: []scriptStep{
		{match: "FROM salesforce_ingest_state", row: noRows},
		{match: `FROM "user" WHERE sf_id`, row: noRows},
		{match: `FROM "user" WHERE LOWER(email)`, rows: nil},
		{match: `INSERT INTO "user"`, row: scriptedRow{vals: []any{"user-1", "jane@acme.com"}}},
		{match: "FROM role WHERE name", rows: [][]any{{"r-ext", "external"}, {"r-cust", "customer"}}},
		{match: "FROM user_role ur JOIN role", rows: nil},
		{match: "FROM account_contact ac WHERE ac.sf_id", row: noRows},
		{match: "FROM account_contact WHERE account_id", row: noRows},
		{match: "INSERT INTO account_contact", row: scriptedRow{vals: []any{"ac-1"}}},
		{match: "SELECT EXISTS", row: scriptedRow{vals: []any{false}}},
	}}
	res, err := upsertContactTx(context.Background(), q, sampleContactUpsert())
	if err != nil {
		t.Fatalf("upsertContactTx: %v", err)
	}
	if res.UserID != "user-1" || !res.CreatedUser || res.AccountContactID != "ac-1" || !res.CreatedAccountContact ||
		res.DeactivatedAccountContacts != 1 || res.IsAccountAdmin {
		t.Errorf("result = %+v", res)
	}

	grants := q.execsMatching("VALUES (gen_random_uuid()")
	if len(grants) != 2 {
		t.Errorf("grants = %d, want external and customer", len(grants))
	}
	revokes := q.execsMatching("DELETE FROM user_role")
	if len(revokes) != 2 {
		t.Fatalf("revokes = %d, want the org pair's other half and then the admin pair", len(revokes))
	}
	if got := revokes[0].args[1]; !reflect.DeepEqual(got, []string{"partner"}) {
		t.Errorf("org revoke = %v, want [partner]", got)
	}
	if got := revokes[1].args[1]; !reflect.DeepEqual(got, []string{globalRoleCustomerAdminName, globalRolePartnerAdminName}) {
		t.Errorf("admin revoke = %v, want both (not an admin)", got)
	}
	if len(q.execsMatching(`UPDATE "user" SET is_active = TRUE`)) != 0 {
		t.Error("a user never soft-deleted by this writer must not be force-activated")
	}
	moved := q.execsMatching("SET is_active = FALSE")
	if len(moved) != 1 || moved[0].args[0] != "003000000000001AAA" || moved[0].args[1] != "acct-new" {
		t.Errorf("account-move sweep = %+v", moved)
	}
}

// TestUpsertContactTx_RestoreReactivatesUser: a contact this writer
// soft-deleted (ledger stamped DELETED) and Salesforce restored gets its
// "user" row back; the existing row is updated in place.
func TestUpsertContactTx_RestoreReactivatesUser(t *testing.T) {
	deleted := time.Date(2026, 9, 18, 6, 37, 7, 0, time.UTC)
	ledger := scriptedRow{vals: []any{"contact", "003000000000001AAA", deleted, "DELETED", "SUCCEEDED", nil, 2, deleted, deleted, 0}}
	q := &scriptedQuerier{t: t, script: []scriptStep{
		{match: "FROM salesforce_ingest_state", row: ledger},
		{match: `FROM "user" WHERE sf_id`, row: scriptedRow{vals: []any{"user-1", "jane@acme.com", int64(1)}}},
		{match: "FROM role WHERE name", rows: [][]any{{"r-ext", "external"}, {"r-cust", "customer"}}},
		{match: "FROM user_role ur JOIN role", rows: [][]any{{"external"}, {"customer"}}},
		{match: "FROM account_contact ac WHERE ac.sf_id", row: scriptedRow{vals: []any{"ac-1", int64(1)}}},
		{match: "SELECT EXISTS", row: scriptedRow{vals: []any{true}}},
	}}
	res, err := upsertContactTx(context.Background(), q, sampleContactUpsert())
	if err != nil {
		t.Fatalf("upsertContactTx: %v", err)
	}
	if res.CreatedUser || res.CreatedAccountContact || !res.IsAccountAdmin {
		t.Errorf("result = %+v", res)
	}
	if len(q.execsMatching(`UPDATE "user" SET is_active = TRUE`)) != 1 {
		t.Error("a restored contact's user must be reactivated")
	}
	if len(q.execsMatching("VALUES (gen_random_uuid()")) != 0 {
		t.Error("global roles already held must not be granted again")
	}
	if len(q.execsMatching("FROM role r")) != 1 {
		t.Error("the derived admin role must be granted")
	}
}

// TestUpsertAccountContact_PrimaryFlag pins is_primary_contact: written from
// Salesforce on insert and update when known, FALSE on insert and untouched
// on update when not.
func TestUpsertAccountContact_PrimaryFlag(t *testing.T) {
	q := &scriptedQuerier{t: t, script: []scriptStep{
		{match: "FROM account_contact ac WHERE ac.sf_id", row: noRows},
		{match: "FROM account_contact WHERE account_id", row: noRows},
		{match: "INSERT INTO account_contact", row: scriptedRow{vals: []any{"ac-1"}}},
	}}
	if _, _, err := upsertAccountContact(context.Background(), q, "003A", "acct-1", "jane@acme.com", boolPtr(true), "actor", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q.queries[2], "COALESCE($5::boolean, FALSE)") {
		t.Errorf("insert must default the flag to FALSE:\n%s", q.queries[2])
	}

	q = &scriptedQuerier{t: t, script: []scriptStep{
		{match: "FROM account_contact ac WHERE ac.sf_id", row: scriptedRow{vals: []any{"ac-1", int64(1)}}},
	}}
	if _, _, err := upsertAccountContact(context.Background(), q, "003A", "acct-1", "jane@acme.com", nil, "actor", ""); err != nil {
		t.Fatal(err)
	}
	upd := q.execsMatching("UPDATE account_contact")
	if len(upd) != 1 || !strings.Contains(upd[0].sql, "is_primary_contact = COALESCE($4::boolean, is_primary_contact)") {
		t.Fatalf("update must keep the stored flag when Salesforce sent none: %+v", upd)
	}
	if upd[0].args[3] != (*bool)(nil) {
		t.Errorf("flag arg = %v, want nil", upd[0].args[3])
	}
}

// TestSyncGlobalRoles_ManagedPair: the other half of {customer, partner} is
// revoked when the classification is known, and nothing is revoked when it
// is not (managed empty).
func TestSyncGlobalRoles_ManagedPair(t *testing.T) {
	script := func() []scriptStep {
		return []scriptStep{
			{match: "FROM role WHERE name", rows: [][]any{{"r-ext", "external"}, {"r-part", "partner"}}},
			{match: "FROM user_role ur JOIN role", rows: [][]any{{"external"}, {"customer"}}},
		}
	}
	q := &scriptedQuerier{t: t, script: script()}
	if err := syncGlobalRoles(context.Background(), q, "user-1", []string{"external", "partner"}, []string{"customer", "partner"}, "actor"); err != nil {
		t.Fatal(err)
	}
	if revokes := q.execsMatching("DELETE FROM user_role"); len(revokes) != 1 || !reflect.DeepEqual(revokes[0].args[1], []string{"customer"}) {
		t.Errorf("a reclassified account must flip customer to partner: %+v", revokes)
	}
	if grants := q.execsMatching("INSERT INTO user_role"); len(grants) != 1 || grants[0].args[2] != "r-part" {
		t.Errorf("grants = %+v, want partner only", grants)
	}

	q = &scriptedQuerier{t: t, script: script()}
	if err := syncGlobalRoles(context.Background(), q, "user-1", []string{"external", "partner"}, nil, "actor"); err != nil {
		t.Fatal(err)
	}
	if len(q.execsMatching("DELETE FROM user_role")) != 0 {
		t.Error("an unknown classification must revoke nothing")
	}
}

// TestDeactivateMovedAccountContacts_KeepsAccountsWithLiveMemberships pins
// the sweep's predicate: other accounts only, and never one where the contact
// still holds a live membership on one of that account's projects.
func TestDeactivateMovedAccountContacts_KeepsAccountsWithLiveMemberships(t *testing.T) {
	q := &scriptedQuerier{t: t, execTags: map[string]string{"UPDATE account_contact": "UPDATE 2"}}
	n, err := deactivateMovedAccountContacts(context.Background(), q, "003A", "acct-new", "jane@acme.com", "actor")
	if err != nil || n != 2 {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	sql := q.execs[0].sql
	for _, want := range []string{
		"ac.sf_id = $1", "ac.account_id <> $2", "NOT EXISTS", "p.account_id = ac.account_id", "DEACTIVATED",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("sweep must mention %q:\n%s", want, sql)
		}
	}
}

// ---- admin re-derivation on membership DELETED -----------------------------

func TestRederiveAdminAfterDeactivate(t *testing.T) {
	basisFor := func(called *[]string, ok bool) AdminRoleBasisFunc {
		return func(_ context.Context, contactSfID string) (AdminRoleBasis, bool) {
			*called = append(*called, contactSfID)
			return AdminRoleBasis{AdminRole: globalRoleCustomerAdminName, Managed: []string{globalRoleCustomerAdminName, globalRolePartnerAdminName}}, ok
		}
	}

	t.Run("the last ADMIN membership gone revokes the admin role", func(t *testing.T) {
		var called []string
		q := &scriptedQuerier{t: t, script: []scriptStep{
			{match: "FROM project_contact pc", row: scriptedRow{vals: []any{"user-1", "003A"}}},
			{match: "SELECT EXISTS", row: scriptedRow{vals: []any{false}}},
		}}
		if err := rederiveAdminAfterDeactivate(context.Background(), q, "a0eA", basisFor(&called, true)); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(called, []string{"003A"}) {
			t.Errorf("basis called with %v", called)
		}
		if revokes := q.execsMatching("DELETE FROM user_role"); len(revokes) != 1 ||
			!reflect.DeepEqual(revokes[0].args[1], []string{globalRoleCustomerAdminName, globalRolePartnerAdminName}) {
			t.Errorf("revokes = %+v", revokes)
		}
	})

	t.Run("an unreadable contact leaves the roles alone", func(t *testing.T) {
		var called []string
		q := &scriptedQuerier{t: t, script: []scriptStep{
			{match: "FROM project_contact pc", row: scriptedRow{vals: []any{"user-1", "003A"}}},
		}}
		if err := rederiveAdminAfterDeactivate(context.Background(), q, "a0eA", basisFor(&called, false)); err != nil {
			t.Fatal(err)
		}
		if len(q.execs) != 0 {
			t.Errorf("no role may change on a guessed basis: %+v", q.execs)
		}
	})

	t.Run("no user or no contact id skips the basis", func(t *testing.T) {
		for _, row := range []scriptedRow{noRows, {vals: []any{"user-1", ""}}} {
			var called []string
			q := &scriptedQuerier{t: t, script: []scriptStep{{match: "FROM project_contact pc", row: row}}}
			if err := rederiveAdminAfterDeactivate(context.Background(), q, "a0eA", basisFor(&called, true)); err != nil {
				t.Fatal(err)
			}
			if len(called) != 0 || len(q.execs) != 0 {
				t.Errorf("called = %v, execs = %d", called, len(q.execs))
			}
		}
	})
}

// ---- readers ----------------------------------------------------------------

// TestSearchProjectContactsExcludesDeactivated pins the contacts list to live
// memberships; the count and the page share the predicate.
func TestSearchProjectContactsExcludesDeactivated(t *testing.T) {
	if !strings.Contains(searchProjectContactsBaseWhere, "pc.state <> 'DEACTIVATED'") ||
		!strings.Contains(searchProjectContactsBaseWhere, "pc.state IS NULL") {
		t.Errorf("contacts search must leave out DEACTIVATED rows: %s", searchProjectContactsBaseWhere)
	}
}

// scriptedTx lets upsertMembershipTx, which takes a pgx.Tx, run against a
// scriptedQuerier; only the three querier methods are ever called.
type scriptedTx struct {
	pgx.Tx
	q *scriptedQuerier
}

func (tx scriptedTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return tx.q.Exec(ctx, sql, args...)
}

func (tx scriptedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return tx.q.Query(ctx, sql, args...)
}

func (tx scriptedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tx.q.QueryRow(ctx, sql, args...)
}

// TestUpsertMembershipTx_TakesContactLock: the membership upsert takes the
// Contact writer's advisory lock before it reads anything, so it cannot race
// a Contact event for the same new contact into a duplicate "user" row.
func TestUpsertMembershipTx_TakesContactLock(t *testing.T) {
	q := &scriptedQuerier{t: t, script: []scriptStep{
		{match: "FROM project", row: noRows},
	}}
	_, err := upsertMembershipTx(context.Background(), scriptedTx{q: q}, domain.SalesforceMembershipUpsert{
		ContactSfID: "003000000000001AAA", ContactEmail: "jane@acme.com", ProjectKey: "ACME",
	})
	if err == nil {
		t.Fatal("want the scripted project-not-found error")
	}
	if len(q.execs) == 0 || !strings.Contains(q.execs[0].sql, "pg_advisory_xact_lock") ||
		q.execs[0].args[0] != "salesforce-contact|003000000000001AAA" {
		t.Fatalf("first statement = %+v, want the salesforce-contact lock", q.execs)
	}
	if len(q.queries) != 1 {
		t.Errorf("queries = %d, want the lock to come before the project read", len(q.queries))
	}
}

// TestLockSalesforceContact_BlankIDTakesNoLock: without a contact id there is
// nothing to serialise on, and one shared key would serialise every such write.
func TestLockSalesforceContact_BlankIDTakesNoLock(t *testing.T) {
	q := &scriptedQuerier{t: t}
	if err := lockSalesforceContact(context.Background(), q, "  "); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(q.execs) != 0 {
		t.Errorf("execs = %+v, want none", q.execs)
	}
}
