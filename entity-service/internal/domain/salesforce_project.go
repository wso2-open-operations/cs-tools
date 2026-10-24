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

// SalesforceProjectUpsert is one Salesforce Project__c mapped onto project by
// the Project ingest. It carries only the ten columns Salesforce owns
// (SALESFORCE_SYNC_PLAN.md §6): the hour counters, closure states,
// onboarding fields, credentials, number and is_active are ServiceNow or CSM
// data and have no field here, so the ingest cannot write them.
type SalesforceProjectUpsert struct {
	SfID string
	Key  string
	Name *string
	// AccountID is the CSM account.id of Account__c, resolved by the caller
	// (EnsureAccount). nil keeps the stored account.
	AccountID   *string
	StartDate   *time.Time
	EndDate     *time.Time
	Description *string
	// ProjectTypeID is project_type.id resolved by exact name; nil when the
	// Salesforce label is empty or matches no row.
	ProjectTypeID           *string
	ComplianceViolationDate *time.Time
	// GoLiveDate is Go_Live_Date__c -> onboarding_go_live_date. CSM owns the
	// value and writes it to Salesforce (decision D4), so this mirrors it back.
	GoLiveDate *time.Time
	// Reactivate clears the DELETED marker (is_active = FALSE) the ingest
	// set: a RESTORED event.
	Reactivate bool
	// AllowInsert lets the upsert create a project Salesforce knows and CSM
	// does not. Off while csm-sync-service still writes project (plan §6):
	// a missing row is then a NotFoundError.
	AllowInsert bool
	// InsertID is the id a newly inserted project gets: the id the backing
	// system already uses for it, so the two never diverge. nil lets the
	// database generate one. Ignored when a row is found (update path).
	InsertID *string
	// RequireInsertID makes a missing InsertID fatal for an insert: a project
	// CSM does not have is then a NotFoundError rather than a row with a
	// random id. Set whenever a backing-system id resolver is configured.
	RequireInsertID bool
}

// SalesforceProjectUpsertResult reports how the project row was resolved.
type SalesforceProjectUpsertResult struct {
	ProjectID string
	// LinkedByKey is true when no row carried the sf_id and the row with the
	// same key had its sf_id stamped.
	LinkedByKey bool
	Created     bool
	Reactivated bool
}

// SalesforceOpportunityLinkUpsert is one Salesforce Linked_Opportunity__c
// mapped onto sf_opportunity_link, with both parents already resolved.
type SalesforceOpportunityLinkUpsert struct {
	LinkSfID      string
	Number        *string
	OpportunityID string
	ProjectID     string
}
