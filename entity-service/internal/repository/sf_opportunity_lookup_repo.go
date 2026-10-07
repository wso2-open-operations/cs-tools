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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
)

// SalesforceOpportunityLookup resolves an sf_opportunity row by Salesforce
// Opportunity Id, for the child ingests (invoices, line items) that need
// their parent's row id before writing.
type SalesforceOpportunityLookup interface {
	// LookupOpportunityIDBySfID returns the sf_opportunity.id the ingest writes for sfID
	// (resolveOpportunityBySfIDQuery), or nil when there is none.
	LookupOpportunityIDBySfID(ctx context.Context, sfID string) (*string, error)
}

type sfOpportunityLookupRepo struct {
	db db.Pool
}

// NewSalesforceOpportunityLookup constructs a SalesforceOpportunityLookup.
func NewSalesforceOpportunityLookup(db db.Pool) SalesforceOpportunityLookup {
	return &sfOpportunityLookupRepo{db: db}
}

func (r *sfOpportunityLookupRepo) LookupOpportunityIDBySfID(ctx context.Context, sfID string) (*string, error) {
	return resolveIDBySfID(ctx, r.db, resolveOpportunityBySfIDQuery, "sf_opportunity", sfID)
}
