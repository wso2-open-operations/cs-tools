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

// SalesforceOpportunityRepository writes the Salesforce Opportunity ingest
// into sf_opportunity and sf_opportunity_product (migration 0080). Every
// write records its salesforce_ingest_state row in the same transaction, so
// the ledger can never disagree with the tables.
type SalesforceOpportunityRepository interface {
	// UpsertFromSalesforce writes one opportunity row by sf_id (resolveOpportunityBySfIDQuery's
	// pick) and makes its sf_opportunity_product rows equal to row.LineItems, then records state.
	UpsertFromSalesforce(ctx context.Context, row domain.SalesforceOpportunityUpsert, state domain.UpsertSalesforceIngestStateRequest) (domain.SalesforceOpportunityUpsertResult, error)
	// DeleteBySfID hard-deletes every sf_opportunity row carrying sfID and
	// records state. The foreign keys cascade the line items and the project
	// links and null out invoices. It returns how many rows were deleted
	// (0 when the opportunity was never ingested).
	DeleteBySfID(ctx context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error)
}

type sfOpportunityRepo struct {
	db db.Pool
}

// NewSalesforceOpportunityRepository constructs a SalesforceOpportunityRepository backed by the pool.
func NewSalesforceOpportunityRepository(db db.Pool) SalesforceOpportunityRepository {
	return &sfOpportunityRepo{db: db}
}

// sfOpportunityLockKey serialises every write of one opportunity (and so of
// its line items): sf_id is not unique (migration 0095), so two concurrent
// events for a new opportunity would otherwise both insert.
func sfOpportunityLockKey(sfID string) string { return "sf-opportunity:" + sfID }

// updateSfOpportunityQuery lists ONLY the columns Salesforce owns. type,
// owner, engagement_code and query_hour_state are written by ServiceNow-side
// flows and must never appear here (TestSfOpportunitySQL_NeverTouchesServiceNowColumns).
// $8 true keeps both EULA columns: Sales Entity did not send eulaVersion.
const updateSfOpportunityQuery = `
	UPDATE sf_opportunity SET
		name = $2,
		account_id = $3,
		stage = $4,
		is_won = $5,
		close_date = $6,
		eula_version = CASE WHEN $8::boolean THEN sf_opportunity.eula_version ELSE $7 END,
		eula_version_decimal = CASE WHEN $8::boolean THEN sf_opportunity.eula_version_decimal ELSE $9::text::numeric END,
		updated_on = now(),
		updated_by = $1,
		sync_time_stamp = now()
	FROM (SELECT o.id, count(*) OVER () AS n FROM sf_opportunity o WHERE o.sf_id = $10
		ORDER BY ` + opportunityReferencedOrder + ` LIMIT 1) t
	WHERE sf_opportunity.id = t.id
	RETURNING sf_opportunity.id::text, t.n`

const insertSfOpportunityQuery = `
	INSERT INTO sf_opportunity (
		id, created_on, updated_on, created_by, updated_by,
		sf_id, name, account_id, stage, is_won, close_date,
		eula_version, eula_version_decimal, sync_time_stamp
	) VALUES (
		gen_random_uuid(), now(), now(), $1, $1,
		$10, $2, $3, $4, $5, $6,
		CASE WHEN $8::boolean THEN NULL ELSE $7 END,
		CASE WHEN $8::boolean THEN NULL ELSE $9::text::numeric END,
		now()
	)
	RETURNING id::text`

// updateSfOpportunityProductQuery lists ONLY the columns Salesforce owns; it writes one copy,
// preferring the one already under the opportunity ($2), then the oldest.
const updateSfOpportunityProductQuery = `
	UPDATE sf_opportunity_product SET
		opportunity_id = $2,
		name = $3,
		product_name = $4,
		quantity = $5::numeric,
		service_start_date = $6,
		service_end_date = $7,
		product_code = $8,
		product_description = $9,
		product_family = $10,
		product_unit = $11,
		eng_product_code = $12,
		product_sf_id = $13,
		classification = $14,
		environment = $15,
		total_price = $16::numeric,
		updated_on = now(),
		updated_by = $1,
		sync_time_stamp = now()
	FROM (SELECT li.id, count(*) OVER () AS n FROM sf_opportunity_product li WHERE li.line_item_sf_id = $17
		ORDER BY (li.opportunity_id IS NOT DISTINCT FROM $2::uuid) DESC, li.created_on, li.id LIMIT 1) t
	WHERE sf_opportunity_product.id = t.id
	RETURNING sf_opportunity_product.id::text, t.n`

const insertSfOpportunityProductQuery = `
	INSERT INTO sf_opportunity_product (
		id, created_on, updated_on, created_by, updated_by,
		line_item_sf_id, opportunity_id, name, product_name, quantity,
		service_start_date, service_end_date, product_code, product_description,
		product_family, product_unit, eng_product_code, product_sf_id,
		classification, environment, total_price, sync_time_stamp
	) VALUES (
		gen_random_uuid(), now(), now(), $1, $1,
		$17, $2, $3, $4, $5::numeric,
		$6, $7, $8, $9,
		$10, $11, $12, $13,
		$14, $15, $16::numeric, now()
	)`

// deleteStaleSfOpportunityProductsQuery removes this opportunity's line items
// that the incoming set does not mention — the REMOVE_DIFF of the ServiceNow
// script's _initOpportunity. A row with no line_item_sf_id is not in the set
// either, hence the COALESCE (NULL = ANY(...) is NULL, not false).
const deleteStaleSfOpportunityProductsQuery = `
	DELETE FROM sf_opportunity_product
	WHERE opportunity_id = $1 AND NOT (COALESCE(line_item_sf_id, '') = ANY($2::text[]))`

const deleteSfOpportunityQuery = `DELETE FROM sf_opportunity WHERE sf_id = $1`

func (r *sfOpportunityRepo) UpsertFromSalesforce(ctx context.Context, row domain.SalesforceOpportunityUpsert, state domain.UpsertSalesforceIngestStateRequest) (domain.SalesforceOpportunityUpsertResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.SalesforceOpportunityUpsertResult{}, fmt.Errorf("upsert opportunity from salesforce: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	res, err := writeSfOpportunity(ctx, tx, row, state)
	if err != nil {
		return domain.SalesforceOpportunityUpsertResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.SalesforceOpportunityUpsertResult{}, fmt.Errorf("upsert opportunity from salesforce: commit: %w", err)
	}
	return res, nil
}

// writeSfOpportunity is UpsertFromSalesforce's body, taking the transaction
// as a querier so it can be exercised without a database:
//
//  1. lock on the sf_id;
//  2. update the one resolved row carrying the sf_id, else insert one;
//  3. upsert each line item by line_item_sf_id under the opportunity's row,
//     then delete the row's line items the set does not mention;
//  4. record the ledger.
func writeSfOpportunity(ctx context.Context, q querier, row domain.SalesforceOpportunityUpsert, state domain.UpsertSalesforceIngestStateRequest) (domain.SalesforceOpportunityUpsertResult, error) {
	var res domain.SalesforceOpportunityUpsertResult
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, sfOpportunityLockKey(row.SfID)); err != nil {
		return res, fmt.Errorf("upsert opportunity from salesforce: lock: %w", err)
	}

	args := []any{
		domain.SalesforceSyncActor,
		row.Name, row.AccountID, row.Stage, row.IsWon, row.CloseDate,
		row.EulaVersion, row.KeepExistingEula, row.EulaVersionDecimal,
		row.SfID,
	}
	id, n, err := updateOneBySfID(ctx, q, updateSfOpportunityQuery, "sf_opportunity", row.SfID, args...)
	if err != nil {
		return res, fmt.Errorf("upsert opportunity from salesforce: update by sf_id: %w", err)
	}
	res.OpportunityID = id
	if n == 0 {
		if err := q.QueryRow(ctx, insertSfOpportunityQuery, args...).Scan(&res.OpportunityID); err != nil {
			return res, fmt.Errorf("upsert opportunity from salesforce: insert: %w", err)
		}
		res.Created = true
	}

	keep := make([]string, 0, len(row.LineItems))
	for _, li := range row.LineItems {
		liArgs := []any{
			domain.SalesforceSyncActor,
			res.OpportunityID, li.Name, li.ProductName, li.Quantity,
			li.ServiceStartDate, li.ServiceEndDate, li.ProductCode, li.ProductDescription,
			li.ProductFamily, li.ProductUnit, li.EngProductCode, li.ProductSfID,
			li.Classification, li.Environment, li.TotalPrice,
			li.LineItemSfID,
		}
		_, n, err := updateOneBySfID(ctx, q, updateSfOpportunityProductQuery, "sf_opportunity_product", li.LineItemSfID, liArgs...)
		if err != nil {
			return res, fmt.Errorf("upsert opportunity line item %s: update: %w", li.LineItemSfID, err)
		}
		if n == 0 {
			if _, err := q.Exec(ctx, insertSfOpportunityProductQuery, liArgs...); err != nil {
				return res, fmt.Errorf("upsert opportunity line item %s: insert: %w", li.LineItemSfID, err)
			}
		}
		keep = append(keep, li.LineItemSfID)
		res.LineItemsWritten++
	}
	tag, err := q.Exec(ctx, deleteStaleSfOpportunityProductsQuery, res.OpportunityID, keep)
	if err != nil {
		return res, fmt.Errorf("upsert opportunity from salesforce: delete stale line items: %w", err)
	}
	res.LineItemsDeleted = int(tag.RowsAffected())

	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return res, err
	}
	return res, nil
}

func (r *sfOpportunityRepo) DeleteBySfID(ctx context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete opportunity by sf_id: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	n, err := deleteSfOpportunity(ctx, tx, sfID, state)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("delete opportunity by sf_id: commit: %w", err)
	}
	return n, nil
}

func deleteSfOpportunity(ctx context.Context, q querier, sfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, sfOpportunityLockKey(sfID)); err != nil {
		return 0, fmt.Errorf("delete opportunity by sf_id: lock: %w", err)
	}
	tag, err := q.Exec(ctx, deleteSfOpportunityQuery, sfID)
	if err != nil {
		return 0, fmt.Errorf("delete opportunity by sf_id: %w", err)
	}
	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
