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
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// oppQuerier is an in-memory sf_opportunity / sf_opportunity_product
// standing in for the transaction: it answers the statements
// writeSfOpportunity and deleteSfOpportunity issue, keyed on the SQL
// constants, and records every statement in order.
type oppQuerier struct {
	// opportunities is sf_id -> row id of the existing sf_opportunity rows.
	opportunities map[string]string
	// products is line_item_sf_id -> opportunity_id of the existing rows.
	products   map[string]string
	statements []string
	ledger     []domain.UpsertSalesforceIngestStateRequest
	nextID     int
	// hoursFor lists the opportunities the project hours hook ran for.
	hoursFor []string
	// linkedProjects are the projects linked to the opportunity being deleted;
	// projectsRecomputed lists the projects the hours hook ran for.
	linkedProjects     []string
	projectsRecomputed []string
}

func (q *oppQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.statements = append(q.statements, sql)
	switch sql {
	case insertSfOpportunityProductQuery:
		q.products[args[16].(string)] = args[1].(string)
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	case deleteStaleSfOpportunityProductsQuery:
		oppID, keep := args[0].(string), args[1].([]string)
		n := 0
		for li, owner := range q.products {
			if owner == oppID && !contains(keep, li) {
				delete(q.products, li)
				n++
			}
		}
		return pgconn.NewCommandTag(fmt.Sprintf("DELETE %d", n)), nil
	case deleteSfOpportunityQuery:
		id, ok := q.opportunities[args[0].(string)]
		if !ok {
			return pgconn.NewCommandTag("DELETE 0"), nil
		}
		delete(q.opportunities, args[0].(string))
		for li, owner := range q.products { // ON DELETE CASCADE
			if owner == id {
				delete(q.products, li)
			}
		}
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (q *oppQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "SELECT DISTINCT project_id::text FROM sf_opportunity_link") {
		// The project hours hook: no linked projects.
		q.statements = append(q.statements, sql)
		q.hoursFor = append(q.hoursFor, args[0].(string))
		return &fakeRows{}, nil
	}
	return nil, fmt.Errorf("unexpected Query")
}

func (q *oppQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.statements = append(q.statements, sql)
	switch sql {
	case projectHoursStateQuery:
		q.projectsRecomputed = append(q.projectsRecomputed, args[0].(string))
		return scanRow{err: pgx.ErrNoRows}
	case previousOpportunityProjectsQuery:
		return scanRow{vals: []any{q.linkedProjects}}
	case previousLineItemOpportunitiesQuery:
		prev := []string{}
		if owner, ok := q.products[args[0].(string)]; ok {
			prev = append(prev, owner)
		}
		return scanRow{vals: []any{prev}}
	case insertSfOpportunityQuery:
		q.nextID++
		id := fmt.Sprintf("opp-row-%d", q.nextID)
		q.opportunities[args[9].(string)] = id
		return oppRow{id: id}
	case updateSfOpportunityQuery:
		if id, ok := q.opportunities[args[9].(string)]; ok {
			return scanRow{vals: []any{id, int64(1)}}
		}
		return scanRow{err: pgx.ErrNoRows}
	case updateSfOpportunityProductQuery:
		id := args[16].(string)
		if _, ok := q.products[id]; ok {
			q.products[id] = args[1].(string)
			return scanRow{vals: []any{"li-row", int64(1)}}
		}
		return scanRow{err: pgx.ErrNoRows}
	}
	if strings.Contains(sql, "salesforce_ingest_state") {
		q.ledger = append(q.ledger, domain.UpsertSalesforceIngestStateRequest{
			Entity: args[0].(string), SfID: args[1].(string), EventModifiedOn: args[2].(time.Time),
			EventType: args[3].(string), Status: domain.SalesforceIngestStatus(args[4].(string)),
		})
		return oppRow{}
	}
	return oppRow{err: fmt.Errorf("unexpected QueryRow: %.60s", sql)}
}

// oppRow scans id into the first destination and leaves the rest zeroed,
// which is all the ledger scan needs here.
type oppRow struct {
	id  string
	err error
}

func (r oppRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > 0 {
		if p, ok := dest[0].(*string); ok {
			*p = r.id
		}
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func oppState(eventType string) domain.UpsertSalesforceIngestStateRequest {
	return domain.UpsertSalesforceIngestStateRequest{
		Entity: domain.SalesforceIngestEntityOpportunity, SfID: "006xx", EventModifiedOn: time.Unix(1, 0).UTC(),
		EventType: eventType, Status: domain.SalesforceIngestSucceeded,
	}
}

func lineItem(id string) domain.SalesforceOpportunityLineItemUpsert {
	name := "Line " + id
	return domain.SalesforceOpportunityLineItemUpsert{LineItemSfID: id, Name: &name}
}

func productIDsOwnedBy(q *oppQuerier, oppID string) []string {
	out := []string{}
	for li, owner := range q.products {
		if owner == oppID {
			out = append(out, li)
		}
	}
	sort.Strings(out)
	return out
}

// TestWriteSfOpportunity_InsertsNewOpportunityWithLineItems: an opportunity
// not in CSM is inserted, its line items inserted under the new row, and the
// ledger written in the same transaction (after the rows).
func TestWriteSfOpportunity_InsertsNewOpportunityWithLineItems(t *testing.T) {
	q := &oppQuerier{opportunities: map[string]string{}, products: map[string]string{}}
	res, err := writeSfOpportunity(context.Background(), q, domain.SalesforceOpportunityUpsert{
		SfID: "006xx", LineItems: []domain.SalesforceOpportunityLineItemUpsert{lineItem("00k1"), lineItem("00k2")},
	}, oppState("CREATED"))
	if err != nil {
		t.Fatalf("writeSfOpportunity: %v", err)
	}
	if !res.Created || res.OpportunityID != "opp-row-1" || res.LineItemsWritten != 2 || res.LineItemsDeleted != 0 {
		t.Errorf("result = %+v", res)
	}
	if got := productIDsOwnedBy(q, "opp-row-1"); !reflect.DeepEqual(got, []string{"00k1", "00k2"}) {
		t.Errorf("line items = %v", got)
	}
	if !strings.Contains(q.statements[0], "pg_advisory_xact_lock") {
		t.Errorf("first statement = %q, want the advisory lock", q.statements[0])
	}
	// Project hours are recomputed once for the opportunity written.
	if !reflect.DeepEqual(q.hoursFor, []string{"opp-row-1"}) {
		t.Errorf("project hours recomputed for %v, want [opp-row-1]", q.hoursFor)
	}
	if len(q.ledger) != 1 || q.ledger[0].Entity != "opportunity" || q.ledger[0].EventType != "CREATED" {
		t.Errorf("ledger = %+v", q.ledger)
	}
	if !strings.Contains(q.statements[len(q.statements)-1], "salesforce_ingest_state") {
		t.Errorf("last statement is not the ledger write: %.60s", q.statements[len(q.statements)-1])
	}
}

// TestWriteSfOpportunity_LineItemSetReplace: an existing opportunity keeps
// its row, an existing line item is updated, a new one is inserted, and a
// line item Salesforce no longer lists is deleted. Another opportunity's
// line items are not touched.
func TestWriteSfOpportunity_LineItemSetReplace(t *testing.T) {
	q := &oppQuerier{
		opportunities: map[string]string{"006xx": "opp-a", "006yy": "opp-b"},
		products:      map[string]string{"00k-keep": "opp-a", "00k-gone": "opp-a", "00k-other": "opp-b"},
	}
	res, err := writeSfOpportunity(context.Background(), q, domain.SalesforceOpportunityUpsert{
		SfID: "006xx", LineItems: []domain.SalesforceOpportunityLineItemUpsert{lineItem("00k-keep"), lineItem("00k-new")},
	}, oppState("UPDATED"))
	if err != nil {
		t.Fatalf("writeSfOpportunity: %v", err)
	}
	if res.Created || res.OpportunityID != "opp-a" || res.LineItemsWritten != 2 || res.LineItemsDeleted != 1 {
		t.Errorf("result = %+v", res)
	}
	if got := productIDsOwnedBy(q, "opp-a"); !reflect.DeepEqual(got, []string{"00k-keep", "00k-new"}) {
		t.Errorf("opp-a line items = %v, want keep + new", got)
	}
	if got := productIDsOwnedBy(q, "opp-b"); !reflect.DeepEqual(got, []string{"00k-other"}) {
		t.Errorf("opp-b line items = %v, want untouched", got)
	}
	insertedOpp := 0
	for _, s := range q.statements {
		if s == insertSfOpportunityQuery {
			insertedOpp++
		}
	}
	if insertedOpp != 0 {
		t.Errorf("opportunity inserted %d times, want an update only", insertedOpp)
	}
}

// TestWriteSfOpportunity_EmptyLineItemsClearsTheSet: an opportunity with no
// line items in Salesforce ends with none in CSM.
func TestWriteSfOpportunity_EmptyLineItemsClearsTheSet(t *testing.T) {
	q := &oppQuerier{opportunities: map[string]string{"006xx": "opp-a"}, products: map[string]string{"00k1": "opp-a", "00k2": "opp-a"}}
	res, err := writeSfOpportunity(context.Background(), q, domain.SalesforceOpportunityUpsert{SfID: "006xx"}, oppState("UPDATED"))
	if err != nil {
		t.Fatalf("writeSfOpportunity: %v", err)
	}
	if res.LineItemsDeleted != 2 || len(q.products) != 0 {
		t.Errorf("result = %+v, products left = %v", res, q.products)
	}
}

// TestDeleteSfOpportunity_CascadeIntent: DELETED is a hard delete by sf_id —
// the line items go with it through ON DELETE CASCADE — and the ledger
// records DELETED in the same transaction; a missing row deletes nothing and
// still records the ledger.
func TestDeleteSfOpportunity_CascadeIntent(t *testing.T) {
	q := &oppQuerier{opportunities: map[string]string{"006xx": "opp-a"}, products: map[string]string{"00k1": "opp-a"}}
	n, err := deleteSfOpportunity(context.Background(), q, "006xx", oppState("DELETED"))
	if err != nil || n != 1 {
		t.Fatalf("delete = %d, %v; want 1, nil", n, err)
	}
	if len(q.opportunities) != 0 || len(q.products) != 0 {
		t.Errorf("rows left: opportunities=%v products=%v", q.opportunities, q.products)
	}
	if len(q.ledger) != 1 || q.ledger[0].EventType != "DELETED" {
		t.Errorf("ledger = %+v", q.ledger)
	}
	if !regexp.MustCompile(`^DELETE FROM sf_opportunity WHERE sf_id = \$1$`).MatchString(deleteSfOpportunityQuery) {
		t.Errorf("delete query = %q, want a hard delete by sf_id", deleteSfOpportunityQuery)
	}

	n, err = deleteSfOpportunity(context.Background(), q, "006xx", oppState("DELETED"))
	if err != nil || n != 0 || len(q.ledger) != 2 {
		t.Errorf("second delete = %d, %v, ledger rows %d; want 0, nil, 2", n, err, len(q.ledger))
	}
}

// Deleting an opportunity recomputes the projects that were linked to it.
func TestDeleteSfOpportunity_RecomputesItsProjects(t *testing.T) {
	q := &oppQuerier{opportunities: map[string]string{"006xx": "opp-a"}, products: map[string]string{},
		linkedProjects: []string{"p1", "p2"}}
	if _, err := deleteSfOpportunity(context.Background(), q, "006xx", oppState("DELETED")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(q.projectsRecomputed, []string{"p1", "p2"}) {
		t.Errorf("projects recomputed = %v, want [p1 p2]", q.projectsRecomputed)
	}
}

// setColumns returns the column names assigned in an UPDATE ... SET list.
func setColumns(sql string) []string {
	body := sql[strings.Index(sql, " SET")+4 : strings.Index(sql, "WHERE")]
	cols := []string{}
	for _, m := range regexp.MustCompile(`(?m)^\s*([a-z_]+)\s*=`).FindAllStringSubmatch(body, -1) {
		cols = append(cols, m[1])
	}
	sort.Strings(cols)
	return cols
}

// TestSfOpportunitySQL_NeverTouchesServiceNowColumns pins the exact column
// lists: the ServiceNow-derived columns (type, owner, engagement_code,
// query_hour_state; development_support_hours and engagement_code on the
// line items) must never be written by the ingest, neither by the UPDATE
// nor the INSERT.
func TestSfOpportunitySQL_NeverTouchesServiceNowColumns(t *testing.T) {
	wantOpp := []string{"account_id", "close_date", "eula_version", "eula_version_decimal", "is_won", "name", "stage", "sync_time_stamp", "updated_by", "updated_on"}
	if got := setColumns(updateSfOpportunityQuery); !reflect.DeepEqual(got, wantOpp) {
		t.Errorf("sf_opportunity UPDATE columns = %v, want %v", got, wantOpp)
	}
	wantLine := []string{"classification", "eng_product_code", "environment", "name", "opportunity_id", "product_code", "product_description",
		"product_family", "product_name", "product_sf_id", "product_unit", "quantity", "service_end_date", "service_start_date",
		"sync_time_stamp", "total_price", "updated_by", "updated_on"}
	if got := setColumns(updateSfOpportunityProductQuery); !reflect.DeepEqual(got, wantLine) {
		t.Errorf("sf_opportunity_product UPDATE columns = %v, want %v", got, wantLine)
	}
	forbidden := regexp.MustCompile(`\b(type|owner|engagement_code|query_hour_state|development_support_hours)\b`)
	for name, sql := range map[string]string{
		"insert opportunity": insertSfOpportunityQuery, "update opportunity": updateSfOpportunityQuery,
		"insert line item": insertSfOpportunityProductQuery, "update line item": updateSfOpportunityProductQuery,
	} {
		if m := forbidden.FindString(sql); m != "" {
			t.Errorf("%s writes ServiceNow-owned column %q", name, m)
		}
	}
}

// A line item the opportunity set takes over from another opportunity
// re-derives the opportunity it left, after the new one.
func TestWriteSfOpportunity_LineItemMoveDerivesPreviousOpportunity(t *testing.T) {
	q := &oppQuerier{
		opportunities: map[string]string{"006xx": "opp-a", "006yy": "opp-b"},
		products:      map[string]string{"00k-moved": "opp-b"},
	}
	if _, err := writeSfOpportunity(context.Background(), q, domain.SalesforceOpportunityUpsert{
		SfID: "006xx", KeepStaleLineItems: true, LineItems: []domain.SalesforceOpportunityLineItemUpsert{lineItem("00k-moved")},
	}, oppState("UPDATED")); err != nil {
		t.Fatal(err)
	}
	var derived int
	for _, s := range q.statements {
		if s == deriveOnboardingForOpportunityQuery {
			derived++
		}
	}
	if derived != 2 {
		t.Errorf("derive statements = %d, want 2 (new opportunity, then the previous one)", derived)
	}
}
