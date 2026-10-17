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

package repository

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ---- the UPDATE's column list ---------------------------------------------

// setClauses parses updateAccountFromSalesforceQuery's SET list into
// column -> right-hand side.
func setClauses(t *testing.T) map[string]string {
	t.Helper()
	body := updateAccountFromSalesforceQuery[strings.Index(updateAccountFromSalesforceQuery, "SET")+len("SET"):]
	out := map[string]string{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSuffix(strings.TrimSpace(line), ",")
		if line == "" {
			continue
		}
		col, rhs, ok := strings.Cut(line, " = ")
		if !ok {
			t.Fatalf("unparseable SET line %q", line)
		}
		out[col] = rhs
	}
	return out
}

// insertColumns parses insertAccountFromSalesforceQuery's column list.
func insertColumns(t *testing.T) map[string]bool {
	t.Helper()
	m := regexp.MustCompile(`(?s)INSERT INTO account \((.*?)\) VALUES`).FindStringSubmatch(insertAccountFromSalesforceQuery)
	if m == nil {
		t.Fatal("insert column list not found")
	}
	out := map[string]bool{}
	for _, c := range strings.Split(m[1], ",") {
		out[strings.TrimSpace(c)] = true
	}
	return out
}

// TestUpdateAccountFromSalesforceQuery_ColumnSet pins which account columns
// a Salesforce event may write. The CSM-only columns must be absent so an
// event can never blank them; the SE-1 columns must be COALESCEd with the
// stored value while Sales Entity does not send them; deactivation_date is
// no longer nulled; deleted_on is cleared.
func TestUpdateAccountFromSalesforceQuery_ColumnSet(t *testing.T) {
	set := setClauses(t)
	cases := []struct {
		column string
		// want is a regexp the right-hand side must match; "" means the
		// column must not be in the SET list at all.
		want string
	}{
		// CSM-only columns: never written by the ingest.
		{"cre_team_id", ""},
		{"sre_team_id", ""},
		{"support_tier", ""},
		{"support_timezone", ""},
		{"suspension_process_state", ""},
		{"ai_gen_response_enabled", ""},
		{"smart_knowledge_base_suggestions_enabled", ""},
		{"drive_location", ""},
		{"created_on", ""},
		{"created_by", ""},
		// number keeps the stored value (it is NOT NULL, so COALESCE picks it).
		{"number", `^COALESCE\(account\.number, `},
		// Salesforce-owned columns available today: plain assignment.
		{"name", `^\$2$`},
		{"phone", `^CASE WHEN \$32 THEN account\.phone ELSE \$8 END$`},
		{"technical_owner_id", `^\$15$`},
		{"street", `^\$16$`},
		{"city", `^\$17$`},
		{"state_province", `^\$18$`},
		{"postal_code", `^\$19$`},
		{"country", `^\$20$`},
		{"account_manager_id", `^\$21$`},
		{"activation_date", `^\$22$`},
		{"lost_date", `^\$23$`},
		{"lost_reason", `^\$24$`},
		// SE-1 columns: keep the stored value when Sales Entity sends none.
		{"customer_success_manager_id", `^COALESCE\(\$25, account\.customer_success_manager_id\)$`},
		{"secondary_technical_owner_id", `^COALESCE\(\$26, account\.secondary_technical_owner_id\)$`},
		{"renewal_account_manager_id", `^COALESCE\(\$27, account\.renewal_account_manager_id\)$`},
		{"account_vertical", `^COALESCE\(\$28, account\.account_vertical\)$`},
		{"lost_reason_category", `^COALESCE\(\$29, account\.lost_reason_category\)$`},
		{"deactivation_date", `^COALESCE\(\$30, account\.deactivation_date\)$`},
		// The primary partner is written as sent, so a partner cleared in
		// Salesforce clears here too; only an absent key keeps the stored value.
		{"primary_partner_sf_id", `^CASE WHEN \$33 THEN account\.primary_partner_sf_id ELSE \$31 END$`},
		// A CREATED/UPDATED/RESTORED event brings a soft-deleted account back.
		{"deleted_on", `^NULL$`},
	}
	for _, tc := range cases {
		t.Run(tc.column, func(t *testing.T) {
			rhs, ok := set[tc.column]
			if tc.want == "" {
				if ok {
					t.Errorf("%s is written (= %s); it is CSM-only and must be absent", tc.column, rhs)
				}
				return
			}
			if !ok {
				t.Fatalf("%s is not written", tc.column)
			}
			if !regexp.MustCompile(tc.want).MatchString(rhs) {
				t.Errorf("%s = %s, want match %s", tc.column, rhs, tc.want)
			}
		})
	}
}

// TestInsertAccountFromSalesforceQuery_ColumnSet: a new account gets every
// Salesforce-owned column plus number (the Salesforce Id, decision D9), and
// none of the CSM-only ones, which keep their defaults.
func TestInsertAccountFromSalesforceQuery_ColumnSet(t *testing.T) {
	cols := insertColumns(t)
	for _, c := range []string{"cre_team_id", "sre_team_id", "support_tier", "support_timezone", "suspension_process_state",
		"ai_gen_response_enabled", "smart_knowledge_base_suggestions_enabled", "drive_location", "deleted_on"} {
		if cols[c] {
			t.Errorf("insert writes %s", c)
		}
	}
	for _, c := range []string{"number", "sf_id", "name", "street", "city", "state_province", "postal_code", "country",
		"account_manager_id", "activation_date", "lost_date", "lost_reason", "customer_success_manager_id",
		"secondary_technical_owner_id", "renewal_account_manager_id", "account_vertical", "lost_reason_category",
		"deactivation_date", "primary_partner_sf_id"} {
		if !cols[c] {
			t.Errorf("insert does not write %s", c)
		}
	}
	// upsertAccountFromSalesforce passes the insert args[:31], so it must
	// reference exactly $1-$31.
	params := map[string]bool{}
	for _, m := range regexp.MustCompile(`\$(\d+)`).FindAllStringSubmatch(insertAccountFromSalesforceQuery, -1) {
		params[m[1]] = true
	}
	for i := 1; i <= 31; i++ {
		if !params[strconv.Itoa(i)] {
			t.Errorf("insert does not reference $%d", i)
		}
	}
	if len(params) != 31 {
		t.Errorf("insert references %d distinct parameters, want 31", len(params))
	}
}

// TestAccountSelectColumns_HasPrimaryPartner: hasPrimaryPartner is true only
// when the account has a primary partner, as in ServiceNow
// (u_primary_partner_account_id). Any other partner link does not count.
func TestAccountSelectColumns_HasPrimaryPartner(t *testing.T) {
	if strings.Contains(accountSelectColumns, "account_relationship") {
		t.Error("accountSelectColumns derives hasPrimaryPartner from account_relationship; a non-primary partner link must not count")
	}
	if !strings.Contains(accountSelectColumns, "COALESCE(TRIM(a.primary_partner_sf_id), '') <> ''") {
		t.Error("accountSelectColumns does not derive hasPrimaryPartner from a.primary_partner_sf_id")
	}
}

// ---- a querier standing in for the transaction ---------------------------

// accountTxQuerier records every statement and answers the account UPDATEs
// with the configured affected-row counts, in order. The ledger write goes
// through QueryRow and is recorded there.
type accountTxQuerier struct {
	execs         []recordedExec
	updateResults []int64
	rowQueries    []recordedExec
	execErr       error
}

func (q *accountTxQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execs = append(q.execs, recordedExec{sql: sql, args: args})
	if q.execErr != nil && strings.Contains(sql, "account") && !strings.Contains(sql, "pg_advisory_xact_lock") {
		return pgconn.CommandTag{}, q.execErr
	}
	if strings.Contains(sql, "UPDATE account") && len(q.updateResults) > 0 {
		n := q.updateResults[0]
		q.updateResults = q.updateResults[1:]
		if n > 0 {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (q *accountTxQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("Query is not used by the account upsert")
}

func (q *accountTxQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "UPDATE account") {
		// The one-row UPDATE ... RETURNING id, copies: recorded with the Exec updates.
		q.execs = append(q.execs, recordedExec{sql: sql, args: args})
		if q.execErr != nil {
			return scanRow{err: q.execErr}
		}
		var n int64
		if len(q.updateResults) > 0 {
			n, q.updateResults = q.updateResults[0], q.updateResults[1:]
		}
		if n == 0 {
			return scanRow{err: pgx.ErrNoRows}
		}
		return scanRow{vals: []any{"acct-row", n}}
	}
	q.rowQueries = append(q.rowQueries, recordedExec{sql: sql, args: args})
	return nopRow{}
}

// nopRow scans successfully without touching its destinations.
type nopRow struct{}

func (nopRow) Scan(...any) error { return nil }

func (q *accountTxQuerier) statements(substr string) []recordedExec {
	var out []recordedExec
	for _, e := range q.execs {
		if strings.Contains(e.sql, substr) {
			out = append(out, e)
		}
	}
	return out
}

func sampleAccountState(eventType string) domain.UpsertSalesforceIngestStateRequest {
	return domain.UpsertSalesforceIngestStateRequest{
		Entity: domain.SalesforceIngestEntityAccount, SfID: "001xx", EventType: eventType,
		EventModifiedOn: time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC), Status: domain.SalesforceIngestSucceeded,
	}
}

// TestUpsertAccountFromSalesforce_ResolutionAndLedger walks the three-step
// resolution and checks the ledger row is written on the same querier (the
// transaction) every time.
func TestUpsertAccountFromSalesforce_ResolutionAndLedger(t *testing.T) {
	cases := []struct {
		name          string
		updateResults []int64
		wantUpdates   int
		wantInsert    bool
	}{
		{name: "existing sf_id is updated", updateResults: []int64{1}, wantUpdates: 1},
		{name: "duplicate sf_id: one row updated, every copy restored", updateResults: []int64{2, 2}, wantUpdates: 2},
		{name: "ServiceNow row is linked by number", updateResults: []int64{0, 1}, wantUpdates: 2},
		{name: "unknown account is inserted", updateResults: []int64{0, 0}, wantUpdates: 2, wantInsert: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &accountTxQuerier{updateResults: tc.updateResults}
			row := domain.SalesforceAccountUpsert{SfID: "001xx", Name: "Acme", Number: "001xx"}
			if err := upsertAccountFromSalesforce(context.Background(), q, row, sampleAccountState(domain.SalesforceEventUpdated)); err != nil {
				t.Fatalf("upsert: %v", err)
			}
			if len(q.statements("pg_advisory_xact_lock")) != 1 {
				t.Error("advisory lock not taken")
			}
			if got := len(q.statements("UPDATE account")); got != tc.wantUpdates {
				t.Errorf("updates = %d, want %d", got, tc.wantUpdates)
			}
			inserts := q.statements("INSERT INTO account")
			if (len(inserts) == 1) != tc.wantInsert {
				t.Fatalf("inserts = %d, want insert %v", len(inserts), tc.wantInsert)
			}
			if tc.wantInsert && len(inserts[0].args) != 31 {
				t.Errorf("insert args = %d, want 31", len(inserts[0].args))
			}
			if len(q.rowQueries) != 1 || !strings.Contains(q.rowQueries[0].sql, "INSERT INTO salesforce_ingest_state") {
				t.Fatalf("ledger writes = %+v, want one salesforce_ingest_state upsert", q.rowQueries)
			}
			if got := q.rowQueries[0].args; got[0] != domain.SalesforceIngestEntityAccount || got[1] != "001xx" || got[3] != domain.SalesforceEventUpdated {
				t.Errorf("ledger args = %v", got)
			}
		})
	}
}

// TestUpsertAccountFromSalesforce_AbsentSE1FieldsAreNull: when Sales Entity
// sends no SE-1 fields the COALESCE parameters are NULL, which is what keeps
// the stored values (see the column-set test for the COALESCE itself).
func TestUpsertAccountFromSalesforce_AbsentSE1FieldsAreNull(t *testing.T) {
	q := &accountTxQuerier{updateResults: []int64{1}}
	owner := "user-1"
	row := domain.SalesforceAccountUpsert{SfID: "001xx", Name: "Acme", Number: "001xx", AccountManagerID: &owner}
	if err := upsertAccountFromSalesforce(context.Background(), q, row, sampleAccountState(domain.SalesforceEventUpdated)); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	args := q.statements("UPDATE account")[0].args
	if len(args) != 33 {
		t.Fatalf("update args = %d, want 33", len(args))
	}
	if p, ok := args[20].(*string); !ok || p == nil || *p != "user-1" {
		t.Errorf("$21 account_manager_id = %v, want user-1", args[20])
	}
	for i := 24; i <= 29; i++ {
		switch v := args[i].(type) {
		case *string:
			if v != nil {
				t.Errorf("$%d = %q, want NULL", i+1, *v)
			}
		case *time.Time:
			if v != nil {
				t.Errorf("$%d = %v, want NULL", i+1, *v)
			}
		default:
			t.Errorf("$%d has type %T", i+1, v)
		}
	}
}

// TestUpsertAccountFromSalesforce_PrimaryPartnerArgs: $31 carries the
// primary partner and $33 says whether to keep the stored one; the insert
// gets $31 too, without the update-only flags.
func TestUpsertAccountFromSalesforce_PrimaryPartnerArgs(t *testing.T) {
	partner := "001PP"
	for _, tc := range []struct {
		name      string
		row       domain.SalesforceAccountUpsert
		wantValue *string
		wantKeep  bool
	}{
		{"set", domain.SalesforceAccountUpsert{PrimaryPartnerSfID: &partner}, &partner, false},
		{"cleared", domain.SalesforceAccountUpsert{}, nil, false},
		{"not sent", domain.SalesforceAccountUpsert{KeepExistingPrimaryPartner: true}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &accountTxQuerier{updateResults: []int64{0, 0}}
			row := tc.row
			row.SfID, row.Name, row.Number = "001xx", "Acme", "001xx"
			if err := upsertAccountFromSalesforce(context.Background(), q, row, sampleAccountState(domain.SalesforceEventUpdated)); err != nil {
				t.Fatalf("upsert: %v", err)
			}
			for _, stmt := range append(q.statements("UPDATE account"), q.statements("INSERT INTO account")...) {
				p, ok := stmt.args[30].(*string)
				if !ok || (p == nil) != (tc.wantValue == nil) || (p != nil && *p != *tc.wantValue) {
					t.Errorf("$31 = %v, want %v", stmt.args[30], tc.wantValue)
				}
			}
			update := q.statements("UPDATE account")[0]
			if keep, ok := update.args[32].(bool); !ok || keep != tc.wantKeep {
				t.Errorf("$33 = %v, want %v", update.args[32], tc.wantKeep)
			}
		})
	}
}

// A failed account write returns before the ledger is written, so the
// transaction rolls back with no SUCCEEDED row.
func TestUpsertAccountFromSalesforce_WriteErrorSkipsLedger(t *testing.T) {
	q := &accountTxQuerier{execErr: errors.New("value too long")}
	err := upsertAccountFromSalesforce(context.Background(), q, domain.SalesforceAccountUpsert{SfID: "001xx", Name: "Acme", Number: "001xx"}, sampleAccountState(domain.SalesforceEventUpdated))
	if err == nil {
		t.Fatal("err = nil, want the write error")
	}
	if len(q.rowQueries) != 0 {
		t.Errorf("ledger writes = %d, want 0", len(q.rowQueries))
	}
}

// TestSoftDeleteAccountBySfID sets deleted_on, leaves deactivation_date (the
// Salesforce contract end date) alone, and records the DELETED ledger row in
// the same transaction.
func TestSoftDeleteAccountBySfID(t *testing.T) {
	for _, tc := range []struct {
		name      string
		affected  int64
		wantFound bool
	}{
		{name: "account present", affected: 1, wantFound: true},
		{name: "account never ingested", affected: 0, wantFound: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &accountTxQuerier{updateResults: []int64{tc.affected}}
			found, err := softDeleteAccountBySfID(context.Background(), q, "001xx", sampleAccountState(domain.SalesforceEventDeleted))
			if err != nil {
				t.Fatalf("soft delete: %v", err)
			}
			if found != tc.wantFound {
				t.Errorf("found = %v, want %v", found, tc.wantFound)
			}
			updates := q.statements("UPDATE account")
			if len(updates) != 1 {
				t.Fatalf("updates = %d, want 1", len(updates))
			}
			if !strings.Contains(updates[0].sql, "deleted_on = COALESCE(deleted_on, now())") {
				t.Errorf("soft delete does not set deleted_on: %s", updates[0].sql)
			}
			if strings.Contains(updates[0].sql, "deactivation_date") {
				t.Errorf("soft delete touches deactivation_date: %s", updates[0].sql)
			}
			if len(q.rowQueries) != 1 || q.rowQueries[0].args[3] != domain.SalesforceEventDeleted {
				t.Errorf("ledger writes = %+v, want one DELETED row", q.rowQueries)
			}
		})
	}
}
