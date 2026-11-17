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

package domain

import "time"

// SalesforceOpportunityUpsert is one Salesforce Opportunity mapped onto
// sf_opportunity (migration 0080) by the Opportunity ingest, together with
// the line items (sf_opportunity_product) embedded in the same Sales Entity
// response. It carries only the columns Salesforce owns: type, owner,
// engagement_code and query_hour_state are ServiceNow-derived and absent
// here, so the write cannot blank them.
type SalesforceOpportunityUpsert struct {
	SfID string
	Name *string
	// AccountID is the CSM account.id resolved from the opportunity's
	// customerId; nil when the opportunity has no account in Salesforce.
	AccountID *string
	Stage     *string
	IsWon     *bool
	// CloseDate is Salesforce's support-account end-date roll-up, not
	// Opportunity.CloseDate — what the ServiceNow script stored (decision D6).
	CloseDate *time.Time
	// KeepExistingEula leaves eula_version and eula_version_decimal as they
	// are: Sales Entity did not send eulaVersion at all (a build without the
	// field), which is different from sending it empty.
	KeepExistingEula   bool
	EulaVersion        *string
	EulaVersionDecimal *string
	// LineItems is the complete set of the opportunity's line items; the
	// write makes sf_opportunity_product under this opportunity equal to it.
	LineItems []SalesforceOpportunityLineItemUpsert
	// KeepStaleLineItems skips the removal of the opportunity's line items
	// the set above does not mention, so the write only adds and updates
	// rows. Only the admin resync sets it (it never deletes anything).
	KeepStaleLineItems bool
}

// SalesforceOpportunityLineItemUpsert is one Salesforce OpportunityLineItem
// mapped onto sf_opportunity_product. development_support_hours and
// engagement_code are ServiceNow-side and absent here.
type SalesforceOpportunityLineItemUpsert struct {
	LineItemSfID       string
	Name               *string
	ProductName        *string
	Quantity           *float64
	ServiceStartDate   *time.Time
	ServiceEndDate     *time.Time
	ProductCode        *string
	ProductDescription *string
	ProductFamily      *string
	ProductUnit        *string
	EngProductCode     *string
	ProductSfID        *string
	Classification     *string
	Environment        *string
	TotalPrice         *float64
}

// SalesforceOpportunityUpsertResult reports what one opportunity write did.
type SalesforceOpportunityUpsertResult struct {
	// OpportunityID is the sf_opportunity.id the line items were written under.
	OpportunityID    string
	Created          bool
	LineItemsWritten int
	LineItemsDeleted int
}
