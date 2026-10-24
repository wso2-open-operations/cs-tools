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
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// projectRow is one row of the in-memory project table.
type projectRow struct {
	id, sfID, key string
	isActive      *bool
}

// projectQuerier stands in for the transaction writeSalesforceProject and
// softDeleteSalesforceProject run on, answering their statements (keyed on
// the SQL constants) against an in-memory project table.
type projectQuerier struct {
	rows       []*projectRow
	ledgerRow  *domain.SalesforceIngestState
	statements []string
	updates    [][]any
	ledger     []domain.UpsertSalesforceIngestStateRequest
}

func (q *projectQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.statements = append(q.statements, sql)
	switch sql {
	case updateSalesforceProjectQuery:
		q.updates = append(q.updates, args)
		for _, r := range q.rows {
			if r.id == args[11].(string) {
				r.sfID, r.key = args[1].(string), args[2].(string)
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
		}
		return pgconn.NewCommandTag("UPDATE 0"), nil
	case reactivateSalesforceProjectQuery, softDeleteSalesforceProjectQuery:
		n := 0
		for _, r := range q.rows {
			if r.sfID != args[0].(string) {
				continue
			}
			if sql == reactivateSalesforceProjectQuery {
				if r.isActive == nil || *r.isActive {
					continue
				}
				v := true
				r.isActive = &v
			} else {
				v := false
				r.isActive = &v
			}
			n++
		}
		return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", n)), nil
	}
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (q *projectQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, fmt.Errorf("unexpected Query")
}

func (q *projectQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.statements = append(q.statements, sql)
	switch sql {
	case resolveSalesforceProjectQuery:
		sfID, key := args[0].(string), args[1].(string)
		var byKey *projectRow
		for _, r := range q.rows {
			if r.sfID == sfID {
				return scanRow{vals: []any{r.id, false}}
			}
			if r.key == key {
				byKey = r
			}
		}
		if byKey != nil {
			return scanRow{vals: []any{byKey.id, true}}
		}
		return scanRow{err: pgx.ErrNoRows}
	case projectByIDQuery:
		for _, r := range q.rows {
			if r.id == args[0].(string) {
				sf := r.sfID
				return scanRow{vals: []any{r.key, &sf}}
			}
		}
		return scanRow{err: pgx.ErrNoRows}
	case insertSalesforceProjectQuery:
		v := true
		id := fmt.Sprintf("proj-%d", len(q.rows)+1)
		if given := args[11].(*string); given != nil {
			id = *given
		}
		r := &projectRow{id: id, sfID: args[1].(string), key: args[2].(string), isActive: &v}
		q.rows = append(q.rows, r)
		return scanRow{vals: []any{r.id}}
	}
	if strings.Contains(sql, "INSERT INTO salesforce_ingest_state") {
		q.ledger = append(q.ledger, domain.UpsertSalesforceIngestStateRequest{
			Entity: args[0].(string), SfID: args[1].(string), EventModifiedOn: args[2].(time.Time),
			EventType: args[3].(string), Status: domain.SalesforceIngestStatus(args[4].(string)),
		})
		return scanRow{}
	}
	if strings.Contains(sql, "FROM salesforce_ingest_state WHERE entity") {
		if q.ledgerRow == nil {
			return scanRow{err: pgx.ErrNoRows}
		}
		st := q.ledgerRow
		return scanRow{vals: []any{st.Entity, st.SfID, st.EventModifiedOn, st.EventType, string(st.Status), st.LastError, st.AttemptCount, st.CreatedOn, st.UpdatedOn, st.RetryCount}}
	}
	return scanRow{err: fmt.Errorf("unexpected QueryRow: %.60s", sql)}
}

// scanRow assigns vals to the destinations by type, leaving the rest zeroed.
type scanRow struct {
	vals []any
	err  error
}

func (r scanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, v := range r.vals {
		if i >= len(dest) {
			break
		}
		switch d := dest[i].(type) {
		case *string:
			*d = v.(string)
		case *bool:
			*d = v.(bool)
		case *time.Time:
			*d = v.(time.Time)
		case **string:
			*d = v.(*string)
		case *int:
			*d = v.(int)
		case *int64:
			*d = v.(int64)
		}
	}
	return nil
}

func projectUpsert(allowInsert bool) domain.SalesforceProjectUpsert {
	name := "CSM Sync Test"
	return domain.SalesforceProjectUpsert{SfID: "a0dSF", Key: "CSMSYNCTEST", Name: &name, AllowInsert: allowInsert}
}

func projectState(eventType string) domain.UpsertSalesforceIngestStateRequest {
	return domain.UpsertSalesforceIngestStateRequest{
		Entity: domain.SalesforceIngestEntityProject, SfID: "a0dSF", EventModifiedOn: time.Unix(1, 0).UTC(),
		EventType: eventType, Status: domain.SalesforceIngestSucceeded,
	}
}

func TestWriteSalesforceProject_UpdatesBySfID(t *testing.T) {
	q := &projectQuerier{rows: []*projectRow{{id: "proj-a", sfID: "a0dSF", key: "CSMSYNCTEST"}}}
	res, err := writeSalesforceProject(context.Background(), q, projectUpsert(false), projectState("UPDATED"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res.ProjectID != "proj-a" || res.Created || res.LinkedByKey || len(q.updates) != 1 {
		t.Errorf("result = %+v updates = %d", res, len(q.updates))
	}
	if !strings.Contains(q.statements[0], "pg_advisory_xact_lock") {
		t.Errorf("first statement = %.50s, want the lock", q.statements[0])
	}
	if len(q.ledger) != 1 || q.ledger[0].Entity != "project" || !strings.Contains(q.statements[len(q.statements)-1], "salesforce_ingest_state") {
		t.Errorf("ledger = %+v, last statement %.50s", q.ledger, q.statements[len(q.statements)-1])
	}
}

// TestWriteSalesforceProject_StampsSfIDOnKeyMatch: a ServiceNow-synced row
// without the sf_id is found by key and gets the sf_id.
func TestWriteSalesforceProject_StampsSfIDOnKeyMatch(t *testing.T) {
	q := &projectQuerier{rows: []*projectRow{{id: "proj-sn", sfID: "", key: "CSMSYNCTEST"}}}
	res, err := writeSalesforceProject(context.Background(), q, projectUpsert(false), projectState("UPDATED"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res.ProjectID != "proj-sn" || !res.LinkedByKey || q.rows[0].sfID != "a0dSF" {
		t.Errorf("result = %+v row = %+v", res, q.rows[0])
	}
}

// TestWriteSalesforceProject_UpdateOnly: with inserts off a missing project
// is the NotFoundError the retry job matches, and nothing (not even the
// ledger) is written in the transaction.
func TestWriteSalesforceProject_UpdateOnly(t *testing.T) {
	q := &projectQuerier{}
	_, err := writeSalesforceProject(context.Background(), q, projectUpsert(false), projectState("UPDATED"))
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) || !IsMissingParentError(nf.Msg) {
		t.Fatalf("err = %v, want a missing-parent NotFoundError", err)
	}
	if len(q.rows) != 0 || len(q.ledger) != 0 {
		t.Errorf("rows = %d ledger = %d, want nothing written", len(q.rows), len(q.ledger))
	}
}

func TestWriteSalesforceProject_InsertWhenAllowed(t *testing.T) {
	q := &projectQuerier{}
	res, err := writeSalesforceProject(context.Background(), q, projectUpsert(true), projectState("CREATED"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Created || len(q.rows) != 1 || q.rows[0].key != "CSMSYNCTEST" || len(q.ledger) != 1 {
		t.Errorf("result = %+v rows = %d ledger = %d", res, len(q.rows), len(q.ledger))
	}
	if !strings.Contains(insertSalesforceProjectQuery, "gen_random_uuid()") {
		t.Error("insert has no generated-id fallback")
	}
}

func (q *projectQuerier) insertCount() int {
	n := 0
	for _, st := range q.statements {
		if st == insertSalesforceProjectQuery {
			n++
		}
	}
	return n
}

func (q *projectQuerier) ran(sql string) bool {
	for _, st := range q.statements {
		if st == sql {
			return true
		}
	}
	return false
}

// TestWriteSalesforceProject_InsertsWithGivenID: InsertID becomes the row id;
// without one the statement is handed NULL so the database generates it.
func TestWriteSalesforceProject_InsertsWithGivenID(t *testing.T) {
	given := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	q := &projectQuerier{}
	row := projectUpsert(true)
	row.InsertID, row.RequireInsertID = &given, true
	res, err := writeSalesforceProject(context.Background(), q, row, projectState("CREATED"))
	if err != nil || !res.Created || res.ProjectID != given || len(q.rows) != 1 || q.rows[0].id != given {
		t.Fatalf("res = %+v err = %v rows = %+v", res, err, q.rows)
	}

	q = &projectQuerier{}
	if _, err := writeSalesforceProject(context.Background(), q, projectUpsert(true), projectState("CREATED")); err != nil {
		t.Fatalf("plain insert: %v", err)
	}
	if q.ran(projectByIDQuery) {
		t.Error("plain insert checked for a derived id")
	}
	if !strings.Contains(insertSalesforceProjectQuery, "COALESCE($12::uuid, gen_random_uuid())") {
		t.Error("insert does not fall back to a generated id when none is given")
	}
}

// TestWriteSalesforceProject_RequiredIDNeverRandom: with RequireInsertID and no
// id, a missing project is the retryable NotFoundError and nothing is written.
func TestWriteSalesforceProject_RequiredIDNeverRandom(t *testing.T) {
	q := &projectQuerier{}
	row := projectUpsert(true)
	row.RequireInsertID = true
	_, err := writeSalesforceProject(context.Background(), q, row, projectState("CREATED"))
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) || !IsMissingParentError(nf.Msg) {
		t.Fatalf("err = %v, want a missing-parent NotFoundError", err)
	}
	if q.insertCount() != 0 || len(q.rows) != 0 || len(q.ledger) != 0 {
		t.Errorf("inserts = %d rows = %d ledger = %d, want nothing written", q.insertCount(), len(q.rows), len(q.ledger))
	}
}

// TestWriteSalesforceProject_DerivedIDCollision: the id belongs to a project
// with another key and sf_id. It is not overwritten, nothing is inserted, and
// the error is not one the retry job matches.
func TestWriteSalesforceProject_DerivedIDCollision(t *testing.T) {
	given := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	q := &projectQuerier{rows: []*projectRow{{id: given, sfID: "a0dOTHER", key: "OTHERKEY"}}}
	row := projectUpsert(true)
	row.InsertID, row.RequireInsertID = &given, true
	_, err := writeSalesforceProject(context.Background(), q, row, projectState("CREATED"))
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) || IsMissingParentError(ce.Msg) {
		t.Fatalf("err = %v, want a ConflictError that is not a missing-parent error", err)
	}
	if q.insertCount() != 0 || len(q.rows) != 1 || q.rows[0].key != "OTHERKEY" || q.rows[0].sfID != "a0dOTHER" || len(q.ledger) != 0 {
		t.Errorf("rows = %+v inserts = %d ledger = %d, want the held row untouched and nothing written", q.rows[0], q.insertCount(), len(q.ledger))
	}
}

// TestWriteSalesforceProject_ExistingRowIgnoresInsertID: a concurrent event
// inserted first, so this one updates; the derived id is neither checked nor used.
func TestWriteSalesforceProject_ExistingRowIgnoresInsertID(t *testing.T) {
	given := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	for name, existing := range map[string]*projectRow{
		"by sf_id": {id: "proj-a", sfID: "a0dSF", key: "CSMSYNCTEST"},
		"by key":   {id: "proj-b", sfID: "", key: "CSMSYNCTEST"},
	} {
		q := &projectQuerier{rows: []*projectRow{existing}}
		row := projectUpsert(true)
		row.InsertID, row.RequireInsertID = &given, true
		res, err := writeSalesforceProject(context.Background(), q, row, projectState("UPDATED"))
		if err != nil || res.Created || res.ProjectID != existing.id {
			t.Fatalf("%s: res = %+v err = %v", name, res, err)
		}
		if q.insertCount() != 0 || q.ran(projectByIDQuery) {
			t.Errorf("%s: inserted or checked the derived id on an update", name)
		}
	}
}

// TestWriteSalesforceProject_DeleteAndRestore: DELETED sets the marker;
// RESTORED clears it, and so does any upsert after a DELETED ledger row (or
// after a RESTORED that failed, which the retry job re-runs as UPDATED).
func TestWriteSalesforceProject_DeleteAndRestore(t *testing.T) {
	q := &projectQuerier{rows: []*projectRow{{id: "proj-a", sfID: "a0dSF", key: "CSMSYNCTEST"}}}
	found, err := softDeleteSalesforceProject(context.Background(), q, "a0dSF", projectState("DELETED"))
	if err != nil || !found || q.rows[0].isActive == nil || *q.rows[0].isActive {
		t.Fatalf("delete found=%v err=%v row=%+v", found, err, q.rows[0])
	}
	if len(q.ledger) != 1 || q.ledger[0].EventType != "DELETED" {
		t.Errorf("ledger = %+v", q.ledger)
	}

	row := projectUpsert(false)
	row.Reactivate = true
	res, err := writeSalesforceProject(context.Background(), q, row, projectState("RESTORED"))
	if err != nil || !res.Reactivated || !*q.rows[0].isActive {
		t.Errorf("restore res=%+v err=%v row=%+v", res, err, q.rows[0])
	}

	for _, prev := range []domain.SalesforceIngestState{
		{EventType: "DELETED", Status: domain.SalesforceIngestSucceeded},
		{EventType: "RESTORED", Status: domain.SalesforceIngestFailed},
	} {
		v := false
		q.rows[0].isActive = &v
		q.ledgerRow = &prev
		res, err = writeSalesforceProject(context.Background(), q, projectUpsert(false), projectState("UPDATED"))
		if err != nil || !res.Reactivated {
			t.Errorf("after %s/%s: res=%+v err=%v", prev.EventType, prev.Status, res, err)
		}
	}

	// An UPDATED after an ordinary UPDATED leaves is_active alone, whatever
	// ServiceNow set it to.
	v := false
	q.rows[0].isActive = &v
	q.ledgerRow = &domain.SalesforceIngestState{EventType: "UPDATED", Status: domain.SalesforceIngestSucceeded}
	res, _ = writeSalesforceProject(context.Background(), q, projectUpsert(false), projectState("UPDATED"))
	if res.Reactivated || *q.rows[0].isActive {
		t.Errorf("plain update reactivated the project")
	}

	found, err = softDeleteSalesforceProject(context.Background(), &projectQuerier{}, "a0dSF", projectState("DELETED"))
	if err != nil || found {
		t.Errorf("delete of a missing project found=%v err=%v", found, err)
	}
}

// TestSalesforceProjectSQL_ColumnList pins the exact column list the UPDATE
// writes — the ten Salesforce-owned columns plus the audit pair — and that
// no ServiceNow- or CSM-owned column appears in the UPDATE or INSERT.
func TestSalesforceProjectSQL_ColumnList(t *testing.T) {
	want := []string{"account_id", "compliance_violation_date", "description", "end_date", "key", "name",
		"onboarding_go_live_date", "project_type_id", "sf_id", "start_date", "updated_by", "updated_on"}
	if got := setColumns(updateSalesforceProjectQuery); !reflect.DeepEqual(got, want) {
		t.Errorf("project UPDATE columns = %v, want %v", got, want)
	}
	forbidden := []string{
		// closure states
		"wso2_closure_state", "end_date_closure_state", "invoice_due_date_closure_state", "compliance_violation_closure_state",
		// hour counters
		"consumed_duration", "remaining_onboarding_duration", "total_onboarding_duration", "total_query_duration",
		"remaining_query_duration", "consumed_onboarding_duration",
		// onboarding workflow
		"onboarding_status", "onboarding_go_live_plan_date", "onboarding_expiry_date", "onboarding_owner_id",
		// credentials and product consumption
		"product_consumption_client_id", "product_consumption_client_secret", "product_consumption_primary_secret_key",
		"product_consumption_secondary_secret_key", "product_consumption_license_secrets",
		"choreo_application_id", "choreo_application_status", "consumption_tracking_file_generated_on",
		// other ServiceNow/CSM columns
		"number", "planned_end_date", "assignment_group_id", "is_pdp_subscription", "last_case_created_on", "portal_wso2_id_counter",
	}
	for name, sql := range map[string]string{"update": updateSalesforceProjectQuery, "insert": insertSalesforceProjectQuery} {
		for _, col := range forbidden {
			if regexp.MustCompile(`\b` + col + `\b`).MatchString(sql) {
				t.Errorf("project %s writes %q", name, col)
			}
		}
	}
	if regexp.MustCompile(`\bis_active\b`).MatchString(updateSalesforceProjectQuery) {
		t.Error("project UPDATE writes is_active")
	}
}
