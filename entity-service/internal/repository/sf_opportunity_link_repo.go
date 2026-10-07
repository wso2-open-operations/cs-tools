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

// SalesforceOpportunityLinkRepository writes the Salesforce
// Linked_Opportunity__c ingest into sf_opportunity_link (migration 0080), the
// project <-> opportunity join. Every write records its
// salesforce_ingest_state row in the same transaction.
type SalesforceOpportunityLinkRepository interface {
	// UpsertFromSalesforce writes one link row by link_sf_id (else inserts) and
	// records state. created is true for an insert.
	UpsertFromSalesforce(ctx context.Context, row domain.SalesforceOpportunityLinkUpsert, state domain.UpsertSalesforceIngestStateRequest) (created bool, err error)
	// DeleteByLinkSfID hard-deletes every link carrying linkSfID and records
	// state; it returns how many rows went (0 when never ingested).
	DeleteByLinkSfID(ctx context.Context, linkSfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error)
	// LookupOpportunityIDBySfID returns the sf_opportunity row the ingest writes
	// for sfID (resolveOpportunityBySfIDQuery), or nil when there is none.
	LookupOpportunityIDBySfID(ctx context.Context, sfID string) (*string, error)
}

type sfOpportunityLinkRepo struct {
	db db.Pool
}

// NewSalesforceOpportunityLinkRepository constructs a SalesforceOpportunityLinkRepository backed by the pool.
func NewSalesforceOpportunityLinkRepository(db db.Pool) SalesforceOpportunityLinkRepository {
	return &sfOpportunityLinkRepo{db: db}
}

// sfOpportunityLinkLockKey serialises every write of one link: link_sf_id is
// not unique, so two concurrent events for a new link would both insert.
func sfOpportunityLinkLockKey(sfID string) string { return "sf-opportunity-link:" + sfID }

// updateSfOpportunityLinkQuery writes every column of one copy: the one already joining
// the same opportunity and project, then the oldest.
const updateSfOpportunityLinkQuery = `
	UPDATE sf_opportunity_link SET
		number = $2,
		opportunity_id = $3::uuid,
		project_id = $4::uuid,
		updated_on = now(),
		updated_by = $1,
		sync_time_stamp = now()
	FROM (SELECT l.id, count(*) OVER () AS n FROM sf_opportunity_link l WHERE l.link_sf_id = $5
		ORDER BY (l.opportunity_id IS NOT DISTINCT FROM $3::uuid AND l.project_id IS NOT DISTINCT FROM $4::uuid) DESC,
			l.created_on, l.id LIMIT 1) t
	WHERE sf_opportunity_link.id = t.id
	RETURNING sf_opportunity_link.id::text, t.n`

const insertSfOpportunityLinkQuery = `
	INSERT INTO sf_opportunity_link (
		id, created_on, updated_on, created_by, updated_by,
		link_sf_id, number, opportunity_id, project_id, sync_time_stamp
	) VALUES (
		gen_random_uuid(), now(), now(), $1, $1,
		$5, $2, $3::uuid, $4::uuid, now()
	)`

const deleteSfOpportunityLinkQuery = `DELETE FROM sf_opportunity_link WHERE link_sf_id = $1`

func (r *sfOpportunityLinkRepo) UpsertFromSalesforce(ctx context.Context, row domain.SalesforceOpportunityLinkUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("upsert opportunity link from salesforce: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	created, err := writeSfOpportunityLink(ctx, tx, row, state)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("upsert opportunity link from salesforce: commit: %w", err)
	}
	return created, nil
}

// writeSfOpportunityLink is UpsertFromSalesforce's body, run on q (the
// transaction): lock, update one row by link_sf_id, else insert, then the ledger.
func writeSfOpportunityLink(ctx context.Context, q querier, row domain.SalesforceOpportunityLinkUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, sfOpportunityLinkLockKey(row.LinkSfID)); err != nil {
		return false, fmt.Errorf("upsert opportunity link from salesforce: lock: %w", err)
	}
	args := []any{domain.SalesforceSyncActor, row.Number, row.OpportunityID, row.ProjectID, row.LinkSfID}
	_, n, err := updateOneBySfID(ctx, q, updateSfOpportunityLinkQuery, "sf_opportunity_link", row.LinkSfID, args...)
	if err != nil {
		return false, fmt.Errorf("upsert opportunity link from salesforce: update: %w", err)
	}
	created := false
	if n == 0 {
		if _, err := q.Exec(ctx, insertSfOpportunityLinkQuery, args...); err != nil {
			return false, fmt.Errorf("upsert opportunity link from salesforce: insert: %w", err)
		}
		created = true
	}
	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return false, fmt.Errorf("upsert opportunity link from salesforce: %w", err)
	}
	return created, nil
}

func (r *sfOpportunityLinkRepo) DeleteByLinkSfID(ctx context.Context, linkSfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete opportunity link: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	n, err := deleteSfOpportunityLink(ctx, tx, linkSfID, state)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("delete opportunity link: commit: %w", err)
	}
	return n, nil
}

func deleteSfOpportunityLink(ctx context.Context, q querier, linkSfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, sfOpportunityLinkLockKey(linkSfID)); err != nil {
		return 0, fmt.Errorf("delete opportunity link: lock: %w", err)
	}
	tag, err := q.Exec(ctx, deleteSfOpportunityLinkQuery, linkSfID)
	if err != nil {
		return 0, fmt.Errorf("delete opportunity link: %w", err)
	}
	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return 0, fmt.Errorf("delete opportunity link: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *sfOpportunityLinkRepo) LookupOpportunityIDBySfID(ctx context.Context, sfID string) (*string, error) {
	return resolveIDBySfID(ctx, r.db, resolveOpportunityBySfIDQuery, "sf_opportunity", sfID)
}
