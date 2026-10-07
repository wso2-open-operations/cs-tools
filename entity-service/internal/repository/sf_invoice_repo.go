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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SalesforceInvoiceRepository writes the Salesforce Invoice__c ingest into
// sf_invoice (migration 0081), recording the salesforce_ingest_state row in
// the same transaction.
type SalesforceInvoiceRepository interface {
	// UpsertFromSalesforce writes one invoice row by sf_id and records state. It
	// reports whether a new row was inserted.
	UpsertFromSalesforce(ctx context.Context, row domain.SalesforceInvoiceUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error)
	// DeleteBySfID hard-deletes every sf_invoice row carrying sfID and
	// records state; it returns how many rows went (0 when never ingested).
	DeleteBySfID(ctx context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error)
}

type sfInvoiceRepo struct {
	db db.Pool
}

// NewSalesforceInvoiceRepository constructs a SalesforceInvoiceRepository backed by the pool.
func NewSalesforceInvoiceRepository(db db.Pool) SalesforceInvoiceRepository {
	return &sfInvoiceRepo{db: db}
}

// sfInvoiceLockKey serialises every write of one invoice: sf_id is not
// unique, so two concurrent events for a new invoice would otherwise both
// insert.
func sfInvoiceLockKey(sfID string) string { return "sf-invoice:" + sfID }

// updateSfInvoiceQuery writes every data column of one copy: the one already under the
// opportunity ($5), then the oldest. Nothing references sf_invoice rows.
const updateSfInvoiceQuery = `
	UPDATE sf_invoice SET
		name = $2,
		description = $3,
		classification = $4,
		opportunity_id = $5::uuid,
		invoiced_amount = $6::numeric,
		invoice_date = $7::date,
		invoiced_due_date = $8::date,
		original_invoice_due_date = $9::date,
		invoiced_paid_date = $10::date,
		service_start_date = $11::date,
		service_end_date = $12::date,
		updated_on = now(),
		updated_by = $1,
		sync_time_stamp = now()
	FROM (SELECT i.id, count(*) OVER () AS n FROM sf_invoice i WHERE i.sf_id = $13
		ORDER BY (i.opportunity_id IS NOT DISTINCT FROM $5::uuid) DESC, i.created_on, i.id LIMIT 1) t
	WHERE sf_invoice.id = t.id
	RETURNING sf_invoice.id::text, t.n`

const insertSfInvoiceQuery = `
	INSERT INTO sf_invoice (
		id, created_on, updated_on, created_by, updated_by,
		sf_id, name, description, classification, opportunity_id,
		invoiced_amount, invoice_date, invoiced_due_date, original_invoice_due_date,
		invoiced_paid_date, service_start_date, service_end_date, sync_time_stamp
	) VALUES (
		gen_random_uuid(), now(), now(), $1, $1,
		$13, $2, $3, $4, $5::uuid,
		$6::numeric, $7::date, $8::date, $9::date,
		$10::date, $11::date, $12::date, now()
	)`

const deleteSfInvoiceQuery = `DELETE FROM sf_invoice WHERE sf_id = $1`

func (r *sfInvoiceRepo) UpsertFromSalesforce(ctx context.Context, row domain.SalesforceInvoiceUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("upsert invoice from salesforce: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	created, err := writeSfInvoice(ctx, tx, row, state)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("upsert invoice from salesforce: commit: %w", err)
	}
	return created, nil
}

// writeSfInvoice is UpsertFromSalesforce's body: lock on the sf_id, update
// one row carrying it, else insert one, then record the ledger.
func writeSfInvoice(ctx context.Context, q querier, row domain.SalesforceInvoiceUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, sfInvoiceLockKey(row.SfID)); err != nil {
		return false, fmt.Errorf("upsert invoice from salesforce: lock: %w", err)
	}
	args := []any{
		domain.SalesforceSyncActor,
		row.Name, row.Description, row.Classification, row.OpportunityID,
		row.InvoicedAmount, row.InvoiceDate, row.InvoicedDueDate, row.OriginalInvoiceDueDate,
		row.InvoicedPaidDate, row.ServiceStartDate, row.ServiceEndDate,
		row.SfID,
	}
	_, n, err := updateOneBySfID(ctx, q, updateSfInvoiceQuery, "sf_invoice", row.SfID, args...)
	if err != nil {
		return false, fmt.Errorf("upsert invoice from salesforce: update by sf_id: %w", err)
	}
	created := false
	if n == 0 {
		if _, err := q.Exec(ctx, insertSfInvoiceQuery, args...); err != nil {
			return false, fmt.Errorf("upsert invoice from salesforce: insert: %w", err)
		}
		created = true
	}
	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return false, err
	}
	return created, nil
}

func (r *sfInvoiceRepo) DeleteBySfID(ctx context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete invoice by sf_id: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	n, err := deleteSfInvoice(ctx, tx, sfID, state)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("delete invoice by sf_id: commit: %w", err)
	}
	return n, nil
}

func deleteSfInvoice(ctx context.Context, q querier, sfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, sfInvoiceLockKey(sfID)); err != nil {
		return 0, fmt.Errorf("delete invoice by sf_id: lock: %w", err)
	}
	tag, err := q.Exec(ctx, deleteSfInvoiceQuery, sfID)
	if err != nil {
		return 0, fmt.Errorf("delete invoice by sf_id: %w", err)
	}
	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
