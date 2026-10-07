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

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SalesforceOpportunityLineItemRepository writes the standalone
// OpportunityLineItem ingest into sf_opportunity_product, with the
// salesforce_ingest_state row in the same transaction. It uses the very
// UPDATE and INSERT the Opportunity ingest's derived line items use, so both
// paths write the same columns and never development_support_hours or
// engagement_code.
type SalesforceOpportunityLineItemRepository interface {
	// UpsertFromSalesforce writes one line item by line_item_sf_id under
	// opportunityID (the sf_opportunity row of opportunitySfID) and records
	// state. It reports whether a new row was inserted.
	UpsertFromSalesforce(ctx context.Context, opportunitySfID, opportunityID string, li domain.SalesforceOpportunityLineItemUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error)
	// DeleteByLineItemSfID hard-deletes every row carrying lineItemSfID and
	// records state; it returns how many rows went (0 when never ingested).
	DeleteByLineItemSfID(ctx context.Context, lineItemSfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error)
}

type sfOpportunityLineItemRepo struct {
	db db.Pool
}

// NewSalesforceOpportunityLineItemRepository constructs a SalesforceOpportunityLineItemRepository.
func NewSalesforceOpportunityLineItemRepository(db db.Pool) SalesforceOpportunityLineItemRepository {
	return &sfOpportunityLineItemRepo{db: db}
}

// selectLineItemOpportunitySfIDQuery finds the opportunity a stored line item
// belongs to, so a delete can take that opportunity's lock.
const selectLineItemOpportunitySfIDQuery = `
	SELECT o.sf_id FROM sf_opportunity_product p
	JOIN sf_opportunity o ON o.id = p.opportunity_id
	WHERE p.line_item_sf_id = $1 AND o.sf_id IS NOT NULL
	LIMIT 1`

const deleteSfOpportunityProductQuery = `DELETE FROM sf_opportunity_product WHERE line_item_sf_id = $1`

func (r *sfOpportunityLineItemRepo) UpsertFromSalesforce(ctx context.Context, opportunitySfID, opportunityID string, li domain.SalesforceOpportunityLineItemUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("upsert line item from salesforce: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	created, err := writeSfOpportunityLineItem(ctx, tx, opportunitySfID, opportunityID, li, state)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("upsert line item from salesforce: commit: %w", err)
	}
	return created, nil
}

// writeSfOpportunityLineItem takes the PARENT opportunity's lock, the one
// the Opportunity ingest holds while it replaces the line items as a set:
// line_item_sf_id is not unique, so a standalone upsert and a set replace
// running at once must not both insert the same line item.
func writeSfOpportunityLineItem(ctx context.Context, q querier, opportunitySfID, opportunityID string, li domain.SalesforceOpportunityLineItemUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, sfOpportunityLockKey(opportunitySfID)); err != nil {
		return false, fmt.Errorf("upsert line item from salesforce: lock: %w", err)
	}
	args := []any{
		domain.SalesforceSyncActor,
		opportunityID, li.Name, li.ProductName, li.Quantity,
		li.ServiceStartDate, li.ServiceEndDate, li.ProductCode, li.ProductDescription,
		li.ProductFamily, li.ProductUnit, li.EngProductCode, li.ProductSfID,
		li.Classification, li.Environment, li.TotalPrice,
		li.LineItemSfID,
	}
	_, n, err := updateOneBySfID(ctx, q, updateSfOpportunityProductQuery, "sf_opportunity_product", li.LineItemSfID, args...)
	if err != nil {
		return false, fmt.Errorf("upsert line item %s: update: %w", li.LineItemSfID, err)
	}
	created := false
	if n == 0 {
		if _, err := q.Exec(ctx, insertSfOpportunityProductQuery, args...); err != nil {
			return false, fmt.Errorf("upsert line item %s: insert: %w", li.LineItemSfID, err)
		}
		created = true
	}
	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return false, err
	}
	return created, nil
}

func (r *sfOpportunityLineItemRepo) DeleteByLineItemSfID(ctx context.Context, lineItemSfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete line item by sf id: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	n, err := deleteSfOpportunityLineItem(ctx, tx, lineItemSfID, state)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("delete line item by sf id: commit: %w", err)
	}
	return n, nil
}

// deleteSfOpportunityLineItem locks the owning opportunity (when the line
// item is stored under one) so the delete serialises with that
// opportunity's set replace, then deletes by line_item_sf_id.
func deleteSfOpportunityLineItem(ctx context.Context, q querier, lineItemSfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	var oppSfID string
	err := q.QueryRow(ctx, selectLineItemOpportunitySfIDQuery, lineItemSfID).Scan(&oppSfID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return 0, fmt.Errorf("delete line item by sf id: find opportunity: %w", err)
	default:
		if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, sfOpportunityLockKey(oppSfID)); err != nil {
			return 0, fmt.Errorf("delete line item by sf id: lock: %w", err)
		}
	}
	tag, err := q.Exec(ctx, deleteSfOpportunityProductQuery, lineItemSfID)
	if err != nil {
		return 0, fmt.Errorf("delete line item by sf id: %w", err)
	}
	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
