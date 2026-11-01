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

// The Salesforce resync lets an admin compare the Salesforce data of one
// account or one project with what the database holds, then overwrite chosen
// components from Salesforce. The preview writes nothing; the apply never
// deletes a row.

// Resync scopes.
const (
	SalesforceResyncScopeAccount = "account"
	SalesforceResyncScopeProject = "project"
)

// Resync component keys. Which of them a request covers depends on its scope.
const (
	SalesforceResyncComponentAccount             = "account"
	SalesforceResyncComponentPartners            = "partners"
	SalesforceResyncComponentProjects            = "projects"
	SalesforceResyncComponentProject             = "project"
	SalesforceResyncComponentOpportunities       = "opportunities"
	SalesforceResyncComponentLinkedOpportunities = "linkedOpportunities"
	SalesforceResyncComponentInvoices            = "invoices"
	SalesforceResyncComponentContacts            = "contacts"
	SalesforceResyncComponentMemberships         = "memberships"
)

// Component and record states of a resync preview.
const (
	SalesforceResyncStatusInSync  = "in_sync"
	SalesforceResyncStatusDiffers = "differs"
	SalesforceResyncStatusSkipped = "skipped"

	SalesforceResyncRecordDiffers      = "differs"
	SalesforceResyncRecordMissingInCSM = "missing_in_csm"
	SalesforceResyncRecordOnlyInCSM    = "only_in_csm"
	SalesforceResyncApplyApplied       = "applied"
	SalesforceResyncApplyFailed        = "failed"
	SalesforceResyncApplySkipped       = "skipped"
)

// SalesforceResyncRequest is the body of POST /salesforce/resync/preview.
type SalesforceResyncRequest struct {
	// Scope is "account" or "project".
	Scope string `json:"scope"`
	// ID is the CSM id (uuid) of the account or project.
	ID string `json:"id"`
}

// SalesforceResyncApplyRequest is the body of POST /salesforce/resync/apply.
type SalesforceResyncApplyRequest struct {
	Scope string `json:"scope"`
	ID    string `json:"id"`
	// Components are the component keys to overwrite from Salesforce.
	Components []string `json:"components"`
}

// SalesforceResyncTarget names the account or project a resync ran for.
type SalesforceResyncTarget struct {
	Scope string  `json:"scope"`
	ID    string  `json:"id"`
	SfID  string  `json:"sfId"`
	Name  *string `json:"name"`
}

// SalesforceResyncCounts tallies the records of one component.
type SalesforceResyncCounts struct {
	InSync       int `json:"inSync"`
	Differs      int `json:"differs"`
	MissingInCSM int `json:"missingInCsm"`
	OnlyInCSM    int `json:"onlyInCsm"`
}

// SalesforceResyncFieldDiff is one field that differs. SfValue is the
// Salesforce value as the ingest would write it, except when WouldTruncate:
// then it is the full Salesforce value, which the ingest would cut (or, for
// the account columns, write as NULL) to fit its column.
type SalesforceResyncFieldDiff struct {
	Field         string  `json:"field"`
	CsmValue      *string `json:"csmValue"`
	SfValue       *string `json:"sfValue"`
	WouldTruncate bool    `json:"wouldTruncate"`
}

// SalesforceResyncRecord is one record that is not in sync. Fields is empty
// for a record that exists on one side only.
type SalesforceResyncRecord struct {
	SfID   string                      `json:"sfId"`
	Label  string                      `json:"label"`
	State  string                      `json:"state"`
	Fields []SalesforceResyncFieldDiff `json:"fields"`
}

// SalesforceResyncComponent is the comparison of one component.
type SalesforceResyncComponent struct {
	Key           string                   `json:"key"`
	Label         string                   `json:"label"`
	Status        string                   `json:"status"`
	SkippedReason *string                  `json:"skippedReason"`
	Counts        SalesforceResyncCounts   `json:"counts"`
	Records       []SalesforceResyncRecord `json:"records"`
}

// SalesforceResyncPreview is the response of POST /salesforce/resync/preview.
type SalesforceResyncPreview struct {
	Target       SalesforceResyncTarget      `json:"target"`
	Capped       bool                        `json:"capped"`
	CappedReason *string                     `json:"cappedReason"`
	Components   []SalesforceResyncComponent `json:"components"`
}

// SalesforceResyncApplyError is one record an apply could not write.
type SalesforceResyncApplyError struct {
	SfID    string `json:"sfId"`
	Message string `json:"message"`
}

// SalesforceResyncApplyComponent is the outcome of one applied component.
type SalesforceResyncApplyComponent struct {
	Key           string  `json:"key"`
	Status        string  `json:"status"`
	Succeeded     int     `json:"succeeded"`
	Failed        int     `json:"failed"`
	SkippedReason *string `json:"skippedReason"`
	// SkippedStale counts the records left alone because they changed in
	// Salesforce after the apply read them; SkippedStaleSfIDs lists the first
	// of them. They are neither succeeded nor failed.
	SkippedStale      int                          `json:"skippedStale"`
	SkippedStaleSfIDs []string                     `json:"skippedStaleSfIds"`
	Errors            []SalesforceResyncApplyError `json:"errors"`
}

// SalesforceResyncApplyResponse is the response of POST /salesforce/resync/apply.
type SalesforceResyncApplyResponse struct {
	Components []SalesforceResyncApplyComponent `json:"components"`
}
