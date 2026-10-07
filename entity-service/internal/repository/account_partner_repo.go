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
)

// AccountPartnerRepository answers which accounts are partners of an account,
// from account_relationship.
type AccountPartnerRepository interface {
	// PartnerAccountSfIDs returns the Salesforce ids of the accounts that are
	// partners of accountID (this platform's account UUID). An account with
	// no partners answers an empty slice, not an error.
	PartnerAccountSfIDs(ctx context.Context, accountID string) ([]string, error)
}

type accountPartnerRepo struct {
	db db.Pool
}

// NewAccountPartnerRepository constructs an AccountPartnerRepository.
func NewAccountPartnerRepository(db db.Pool) AccountPartnerRepository {
	return &accountPartnerRepo{db: db}
}

// Partner links are stored twice, once in each direction, the way the
// ServiceNow sync writes them: "Is Partner Of" from the partner to the
// customer, and its reverse "Is Customer Of" from the customer to the
// partner. Either row is enough, so both are read. The labels are matched
// rather than relationship_type_id, which is a ServiceNow sys_id and not
// guaranteed to be the same in every instance.
const (
	relationshipLabelPartnerOf  = "Is Partner Of"
	relationshipLabelCustomerOf = "Is Customer Of"
)

// PartnerAccountSfIDs implements AccountPartnerRepository.
func (r *accountPartnerRepo) PartnerAccountSfIDs(ctx context.Context, accountID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT a.sf_id
		FROM account_relationship ar
		JOIN account a ON a.id = CASE WHEN ar.is_reverse_relationship THEN ar.to_account_id ELSE ar.from_account_id END
		WHERE ((ar.to_account_id = $1 AND NOT ar.is_reverse_relationship AND ar.relationship_label = $2)
		    OR (ar.from_account_id = $1 AND ar.is_reverse_relationship AND ar.relationship_label = $3))
		  AND NULLIF(TRIM(a.sf_id), '') IS NOT NULL`,
		accountID, relationshipLabelPartnerOf, relationshipLabelCustomerOf)
	if err != nil {
		return nil, fmt.Errorf("list partner accounts: %w", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list partner accounts: scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list partner accounts: %w", err)
	}
	return ids, nil
}
