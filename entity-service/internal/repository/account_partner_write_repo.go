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

// AccountPartnerWriteRepository is the write side of account_relationship's
// partner links, used by the Salesforce partner refresh. It writes only the
// "Is Partner Of" / "Is Customer Of" pair that AccountPartnerRepository
// reads; rows of any other label are never touched.
type AccountPartnerWriteRepository interface {
	// ReplacePartners makes customerID's stored partners equal to
	// partnerIDs (account UUIDs), in one transaction under an advisory lock
	// on customerSfID, and records state in the same transaction. Each
	// partner is stored as the forward row (partner "Is Partner Of"
	// customer) and its mirror (customer "Is Customer Of" partner, reverse),
	// the shape the ServiceNow sync left in the table. It returns how many
	// rows it inserted and deleted.
	ReplacePartners(ctx context.Context, customerSfID, customerID string, partnerIDs []string, state domain.UpsertSalesforceIngestStateRequest) (added, removed int, err error)
}

type accountPartnerWriteRepo struct {
	db db.Pool
}

// NewAccountPartnerWriteRepository constructs an AccountPartnerWriteRepository.
func NewAccountPartnerWriteRepository(db db.Pool) AccountPartnerWriteRepository {
	return &accountPartnerWriteRepo{db: db}
}

// accountPartnersLockKey serialises every refresh of one customer's partner
// set: two concurrent refreshes would otherwise both insert the same missing
// row (account_relationship has no unique key to stop them).
func accountPartnersLockKey(customerSfID string) string { return "account-partners:" + customerSfID }

// insertForwardPartnerLinksQuery adds "partner Is Partner Of customer" for
// every partner in $3 that does not have it yet. relationship_type_id stays
// NULL: it is a ServiceNow sys_id with no meaning here, and the reader
// matches labels instead.
//
// $1 actor, $2 customer id, $3 partner ids, $4 'Is Partner Of', $5 'Is Customer Of'.
const insertForwardPartnerLinksQuery = `
	INSERT INTO account_relationship (
		id, created_on, updated_on, created_by, updated_by,
		from_account_id, to_account_id, relationship_type_id,
		relationship_label, reverse_relationship_label, is_reverse_relationship
	)
	SELECT gen_random_uuid(), now(), now(), $1, $1,
		p.id, $2::uuid, NULL,
		$4::text, $5::text, false
	FROM unnest($3::uuid[]) AS p(id)
	WHERE NOT EXISTS (
		SELECT 1 FROM account_relationship ar
		WHERE ar.from_account_id = p.id AND ar.to_account_id = $2::uuid
		  AND ar.relationship_label = $4::text AND ar.is_reverse_relationship = false
	)`

// insertReversePartnerLinksQuery adds the mirror row, "customer Is Customer
// Of partner" with is_reverse_relationship = true and the labels swapped.
// Same parameters as insertForwardPartnerLinksQuery.
const insertReversePartnerLinksQuery = `
	INSERT INTO account_relationship (
		id, created_on, updated_on, created_by, updated_by,
		from_account_id, to_account_id, relationship_type_id,
		relationship_label, reverse_relationship_label, is_reverse_relationship
	)
	SELECT gen_random_uuid(), now(), now(), $1, $1,
		$2::uuid, p.id, NULL,
		$5::text, $4::text, true
	FROM unnest($3::uuid[]) AS p(id)
	WHERE NOT EXISTS (
		SELECT 1 FROM account_relationship ar
		WHERE ar.from_account_id = $2::uuid AND ar.to_account_id = p.id
		  AND ar.relationship_label = $5::text AND ar.is_reverse_relationship = true
	)`

// deleteStaleForwardPartnerLinksQuery is the REMOVE_DIFF of the ServiceNow
// script: the customer's forward partner rows whose partner is no longer in
// the set. Only the partner label is matched, so no other relationship of
// the customer is affected.
//
// $1 customer id, $2 partner ids, $3 'Is Partner Of'.
const deleteStaleForwardPartnerLinksQuery = `
	DELETE FROM account_relationship
	WHERE to_account_id = $1::uuid AND relationship_label = $3::text AND is_reverse_relationship = false
	  AND NOT (from_account_id = ANY($2::uuid[]))`

// deleteStaleReversePartnerLinksQuery removes the matching mirror rows.
//
// $1 customer id, $2 partner ids, $3 'Is Customer Of'.
const deleteStaleReversePartnerLinksQuery = `
	DELETE FROM account_relationship
	WHERE from_account_id = $1::uuid AND relationship_label = $3::text AND is_reverse_relationship = true
	  AND NOT (to_account_id = ANY($2::uuid[]))`

// ReplacePartners implements AccountPartnerWriteRepository.
func (r *accountPartnerWriteRepo) ReplacePartners(ctx context.Context, customerSfID, customerID string, partnerIDs []string, state domain.UpsertSalesforceIngestStateRequest) (int, int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("replace account partners: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	added, removed, err := replaceAccountPartners(ctx, tx, customerSfID, customerID, partnerIDs, state)
	if err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("replace account partners: commit: %w", err)
	}
	return added, removed, nil
}

// replaceAccountPartners is ReplacePartners' body, taking the transaction as
// a querier so it can be exercised without a database.
func replaceAccountPartners(ctx context.Context, q querier, customerSfID, customerID string, partnerIDs []string, state domain.UpsertSalesforceIngestStateRequest) (int, int, error) {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, accountPartnersLockKey(customerSfID)); err != nil {
		return 0, 0, fmt.Errorf("replace account partners: lock: %w", err)
	}
	if partnerIDs == nil {
		partnerIDs = []string{}
	}
	added, removed := 0, 0
	for _, stmt := range []struct {
		what string
		sql  string
		args []any
		out  *int
	}{
		{"insert forward links", insertForwardPartnerLinksQuery, []any{domain.SalesforceSyncActor, customerID, partnerIDs, relationshipLabelPartnerOf, relationshipLabelCustomerOf}, &added},
		{"insert reverse links", insertReversePartnerLinksQuery, []any{domain.SalesforceSyncActor, customerID, partnerIDs, relationshipLabelPartnerOf, relationshipLabelCustomerOf}, &added},
		{"delete stale forward links", deleteStaleForwardPartnerLinksQuery, []any{customerID, partnerIDs, relationshipLabelPartnerOf}, &removed},
		{"delete stale reverse links", deleteStaleReversePartnerLinksQuery, []any{customerID, partnerIDs, relationshipLabelCustomerOf}, &removed},
	} {
		tag, err := q.Exec(ctx, stmt.sql, stmt.args...)
		if err != nil {
			return 0, 0, fmt.Errorf("replace account partners: %s: %w", stmt.what, err)
		}
		*stmt.out += int(tag.RowsAffected())
	}
	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return 0, 0, err
	}
	return added, removed, nil
}
