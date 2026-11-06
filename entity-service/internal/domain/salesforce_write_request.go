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
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package domain

import (
	"encoding/json"
	"time"
)

// Salesforce write requests are portal membership writes whose Salesforce
// half was deferred because sales/sales-entity-service was unavailable. See
// migration 0249 and service.ProjectMembershipWriteService.Invite.

// SalesforceWriteOperationInvite is the only deferrable operation: a first
// invitation of an address the project has no membership for yet. Role
// changes, deactivation and re-invitation need the Salesforce ids a deferred
// row does not have, so they keep failing fast while Salesforce is down.
const SalesforceWriteOperationInvite = "INVITE"

// SalesforceWriteRequestStatus is the lifecycle of one deferred write.
type SalesforceWriteRequestStatus string

const (
	// SalesforceWriteRequestPending: owed to Salesforce; the consumer has not
	// succeeded yet (it may be retrying).
	SalesforceWriteRequestPending SalesforceWriteRequestStatus = "PENDING"
	// SalesforceWriteRequestSucceeded: the Salesforce record exists and the
	// CSM rows carry its ids.
	SalesforceWriteRequestSucceeded SalesforceWriteRequestStatus = "SUCCEEDED"
	// SalesforceWriteRequestFailed: Salesforce refused the write for a reason
	// a retry will not fix (validation; Retryable false), or every retry tier
	// was exhausted while Salesforce was unavailable (Retryable true).
	// Nothing retries it on its own; a later sweep or a manual retry does.
	SalesforceWriteRequestFailed SalesforceWriteRequestStatus = "FAILED"
	// SalesforceWriteRequestCancelled: the admin removed the contact before
	// the Salesforce write happened (a FAILED or never-published request), so
	// there is nothing left to write.
	SalesforceWriteRequestCancelled SalesforceWriteRequestStatus = "CANCELLED"
)

// SalesforceSync is the membership-level view of the request status that
// the portals read: "" (never deferred, or synced before deferral existed),
// PENDING, or FAILED. SUCCEEDED collapses to "" because a synced membership
// is indistinguishable from one written synchronously.
const (
	SalesforceSyncPending = string(SalesforceWriteRequestPending)
	SalesforceSyncFailed  = string(SalesforceWriteRequestFailed)
)

// SalesforceWriteIntent is the portal invitation as the admin made it,
// stored as salesforce_write_request.intent and replayed by the consumer.
// It is deliberately the request, not the resolved Salesforce record: the
// consumer resolves (searches, then creates or adopts) at delivery time, so
// a record that appeared in Salesforce meanwhile is adopted, not duplicated.
type SalesforceWriteIntent struct {
	Email               string   `json:"email"`
	FirstName           string   `json:"firstName,omitempty"`
	LastName            string   `json:"lastName,omitempty"`
	Roles               []string `json:"roles"`
	State               string   `json:"state"`
	IsCsIntegrationUser bool     `json:"isCsIntegrationUser,omitempty"`
	InviterEmail        string   `json:"inviterEmail,omitempty"`
	// AllowedAccountSfIDs are the Salesforce accounts an existing contact may
	// belong to and still be adopted: the project's own account and its
	// partners, read from this database at deferral time. The invitation
	// validator's Salesforce-side checks (domain allow-list, own-or-partner
	// adoption) cannot run while Salesforce is down, so the consumer runs
	// them from this list before it writes.
	AllowedAccountSfIDs []string `json:"allowedAccountSfIds"`
}

// DeferredSalesforceWrite, set on a SalesforceMembershipUpsert, tells the
// membership repository to record a salesforce_write_request row in the
// same transaction as the membership rows it writes.
type DeferredSalesforceWrite struct {
	Operation string
	Intent    SalesforceWriteIntent
}

// SalesforceWriteRequest is one row of salesforce_write_request.
type SalesforceWriteRequest struct {
	ID               string
	Operation        string
	ProjectID        string
	ProjectContactID string
	Email            string
	Intent           json.RawMessage
	Status           SalesforceWriteRequestStatus
	Attempts         int
	LastError        *string
	LastAttemptOn    *time.Time
	PublishedOn      *time.Time
	CompletedOn      *time.Time
	// Retryable is meaningful for FAILED only: true when the retries ran out
	// while Salesforce was unavailable, false when it refused the write.
	Retryable bool
	CreatedOn time.Time
	UpdatedOn time.Time
}
