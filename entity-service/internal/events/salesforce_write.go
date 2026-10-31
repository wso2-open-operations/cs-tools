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

package events

// The deferred Salesforce membership write is a conversation between this
// service and sales/sales-entity-service (digiops-sales, membershipwrite
// package) over two Event Hubs. The envelope is Envelope{Type, EntityID,
// Payload} on both sides, EntityID being the salesforce_write_request id so
// every record about one request shares a partition. These payloads are the
// contract; the field names must stay in step with the Sales side.

// TypeSalesforceMembershipWriteRequested is the command entity-service
// publishes when a portal invitation could not reach Salesforce and was
// committed locally instead (SALESFORCE_WRITE_EVENT_HUB_TOPIC). Sales Entity
// consumes it and performs the Salesforce half.
const TypeSalesforceMembershipWriteRequested Type = "salesforce.membership_write.requested"

// TypeSalesforceMembershipWriteCompleted is Sales Entity's answer when the
// Salesforce records exist (SALESFORCE_WRITE_RESULTS_TOPIC). This service
// consumes it, stamps the ids and completes the request.
const TypeSalesforceMembershipWriteCompleted Type = "salesforce.membership_write.completed"

// TypeSalesforceMembershipWriteFailed is Sales Entity's answer when the write
// will not happen: Salesforce refused it (terminal) or every retry was spent.
const TypeSalesforceMembershipWriteFailed Type = "salesforce.membership_write.failed"

// SalesforceMembershipWriteRequestedPayload is the invitation as the admin
// made it plus the Salesforce ids of the project and account it lands on:
// everything Sales Entity needs, because it cannot read this database. The
// request row, not the event, remains the record of what is owed; a stale or
// duplicate event is settled against the row.
type SalesforceMembershipWriteRequestedPayload struct {
	RequestID        string `json:"requestId"`
	Operation        string `json:"operation"`
	ProjectID        string `json:"projectId"`
	ProjectContactID string `json:"projectContactId"`

	ProjectSfID string `json:"projectSfId"`
	AccountSfID string `json:"accountSfId"`

	Email               string   `json:"email"`
	FirstName           string   `json:"firstName,omitempty"`
	LastName            string   `json:"lastName,omitempty"`
	Roles               []string `json:"roles"`
	State               string   `json:"state"`
	IsCsIntegrationUser bool     `json:"isCsIntegrationUser,omitempty"`
	// InviterEmail is set when a customer admin invited (empty for CSM
	// staff), and AllowedAccountSfIDs are the project's account and its
	// partners. Together they let Sales Entity run the invitation checks
	// the validator could not run while Salesforce was down: the inviter
	// must belong to an allowed account, an existing contact is adopted
	// only from an allowed account, and the address's domain must be in the
	// base account's (the inviter's, else the project's) or a partner's
	// domain list.
	InviterEmail        string   `json:"inviterEmail,omitempty"`
	AllowedAccountSfIDs []string `json:"allowedAccountSfIds"`
	RequestedOn         string   `json:"requestedOn,omitempty"`
}

// SalesforceMembershipWriteCompletedPayload carries the ids Salesforce
// assigned and the state it stored (its automation may keep REGISTERED for a
// contact who has signed in before).
type SalesforceMembershipWriteCompletedPayload struct {
	RequestID         string   `json:"requestId"`
	ProjectContactID  string   `json:"projectContactId"`
	ContactSfID       string   `json:"contactSfId"`
	MembershipSfID    string   `json:"membershipSfId"`
	State             string   `json:"state"`
	Roles             []string `json:"roles"`
	ContactCreated    bool     `json:"contactCreated"`
	MembershipCreated bool     `json:"membershipCreated"`
	LastModifiedDate  string   `json:"lastModifiedDate,omitempty"`
}

// SalesforceMembershipWriteFailedPayload says why the write will not happen.
// Terminal is true for a Salesforce refusal, false when the retries ran out
// while Salesforce was still unavailable.
type SalesforceMembershipWriteFailedPayload struct {
	RequestID        string `json:"requestId"`
	ProjectContactID string `json:"projectContactId"`
	Reason           string `json:"reason"`
	Terminal         bool   `json:"terminal"`
}
