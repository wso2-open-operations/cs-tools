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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// updateTagQuerier answers the one-row UPDATE ... RETURNING with rows copies
// (none: no row), everything else with one row, recording the statements.
type updateTagQuerier struct {
	partnerQuerier
	update   string
	rows     int64
	prevOpps []string // answer to previousLineItemOpportunitiesQuery
}

// Query answers the opportunity-to-projects listing the project hours hook
// issues: no linked projects, so nothing further is recomputed.
func (q *updateTagQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "SELECT DISTINCT project_id::text FROM sf_opportunity_link") {
		q.statements = append(q.statements, sql)
		q.args = append(q.args, args)
		return &fakeRows{}, nil
	}
	return q.partnerQuerier.Query(ctx, sql, args...)
}

func (q *updateTagQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if sql == previousLineItemOpportunitiesQuery {
		q.statements = append(q.statements, sql)
		q.args = append(q.args, args)
		return scanRow{vals: []any{append([]string{}, q.prevOpps...)}}
	}
	if sql == q.update {
		q.statements = append(q.statements, sql)
		q.args = append(q.args, args)
		if q.rows == 0 {
			return scanRow{err: pgx.ErrNoRows}
		}
		return scanRow{vals: []any{"row", q.rows}}
	}
	return q.partnerQuerier.QueryRow(ctx, sql, args...)
}

func TestWriteSfInvoice_UpdateElseInsertThenLedger(t *testing.T) {
	for _, tc := range []struct {
		rows        int64
		wantCreated bool
		want        []string
	}{
		{1, false, []string{"pg_advisory_xact_lock", updateSfInvoiceQuery, "salesforce_ingest_state"}},
		{0, true, []string{"pg_advisory_xact_lock", updateSfInvoiceQuery, insertSfInvoiceQuery, "salesforce_ingest_state"}},
	} {
		q := &updateTagQuerier{update: updateSfInvoiceQuery, rows: tc.rows}
		created, err := writeSfInvoice(context.Background(), q, domain.SalesforceInvoiceUpsert{SfID: "a0I1"}, partnerState())
		if err != nil || created != tc.wantCreated {
			t.Fatalf("rows=%d created=%v err=%v", tc.rows, created, err)
		}
		if len(q.statements) != len(tc.want) {
			t.Fatalf("rows=%d statements=%d, want %d", tc.rows, len(q.statements), len(tc.want))
		}
		for i, w := range tc.want {
			if !strings.Contains(q.statements[i], w) {
				t.Errorf("rows=%d statement %d = %.50q", tc.rows, i, q.statements[i])
			}
		}
		if q.args[0][0] != "sf-invoice:a0I1" {
			t.Errorf("lock key = %v", q.args[0][0])
		}
	}
}

// Every data column of sf_invoice is written on update and insert.
func TestSfInvoiceSQL_WritesEveryColumn(t *testing.T) {
	for _, col := range []string{"name", "description", "classification", "opportunity_id", "invoiced_amount", "invoice_date",
		"invoiced_due_date", "original_invoice_due_date", "invoiced_paid_date", "service_start_date", "service_end_date", "sync_time_stamp"} {
		if !strings.Contains(updateSfInvoiceQuery, col+" = ") || !strings.Contains(insertSfInvoiceQuery, col) {
			t.Errorf("column %s missing from the invoice upsert", col)
		}
	}
}
