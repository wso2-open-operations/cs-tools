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

// Change-request notice events.
//
// These three types and their payloads are the contract between this service,
// which decides WHO should be told about a change request, and
// csm-notification-service, which sends the mail. The structs are duplicated
// there by hand (separate Go modules); keep the two in step.
//
// They moved here from csm-flow-service when the change-request notices were
// folded into the service that already owns the rows and the database they
// live in.
package events

// "Send CR Approval For Customer" and "Send CR Approval For Internal" — but
// the only real differences are the recipient source and whether the owning
// team appears in the subject, so one payload with this discriminator carries
// both.
// TypeCRApprovalRequested asks csm-notification-service to tell the people a
// change request is now waiting on. This service resolves WHO -- it owns the
// rows and the database the audiences live in -- and that one sends the mail.
const TypeCRApprovalRequested Type = "change_request.approval_requested"

// CRApprovalAudience says which side of the change a notice is addressed to.
// It selects the portal the link points at, and whether the recipient list
// goes in To or BCC.
type CRApprovalAudience string

const (
	// CRAudienceCustomer: recipients are the customer project's contacts.
	CRAudienceCustomer CRApprovalAudience = "customer"
	// CRAudienceInternal: recipients are the members of a WSO2 approval group.
	CRAudienceInternal CRApprovalAudience = "internal"
)

// TypeCRPlanDateNotice reports one turn of the plan-start-date
// conversation between WSO2 and a customer. One type
// rather than three, because the three notices differ only in wording and
// audience -- Kind says which.
const TypeCRPlanDateNotice Type = "change_request.plan_date_notice"

// CRPlanDateKind is which turn of the conversation a notice reports.
type CRPlanDateKind string

const (
	// CRPlanDateCustomerProposed: the customer moved the proposed start date.
	// Internal audience.
	CRPlanDateCustomerProposed CRPlanDateKind = "customer_proposed"
	// CRPlanDateAccepted / CRPlanDateRejected: WSO2 answered. Customer audience.
	CRPlanDateAccepted CRPlanDateKind = "accepted"
	CRPlanDateRejected CRPlanDateKind = "rejected"
)

// CRPlanDateNoticePayload is TypeCRPlanDateNotice's payload.
type CRPlanDateNoticePayload struct {
	ChangeRequestID string `json:"changeRequestId"`
	Number          string `json:"number"`
	// Kind selects the body wording; Audience selects the portal to link to.
	Kind     CRPlanDateKind     `json:"kind"`
	Audience CRApprovalAudience `json:"audience"`
	// GroupName is the approval group resolved for an internal notice, empty
	// for a customer one.
	GroupName string `json:"groupName,omitempty"`
	// ActorName is whoever changed the date, rendered LAST NAME FIRST because
	// that is the order the ServiceNow templates interpolate the two pills in.
	ActorName        string `json:"actorName,omitempty"`
	ProjectID        string `json:"projectId,omitempty"`
	ProjectName      string `json:"projectName,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	Description      string `json:"description,omitempty"`
	// Subject is rendered by the flow, verbatim from the original.
	Subject string `json:"subject"`
	// Recipients are already resolved and de-duplicated. Never empty.
	Recipients []string `json:"recipients"`
}

// CRApprovalRequestedPayload is TypeCRApprovalRequested's payload.
type CRApprovalRequestedPayload struct {
	ChangeRequestID string `json:"changeRequestId"`
	// Number is the human-readable CR reference (e.g. "CHG0031234"). It is what
	// the subject line shows; the id is meaningless to a recipient.
	Number string `json:"number"`
	// State is the approval state the CR just entered, as the domain value
	// (ASSESS / AUTHORIZE / CUSTOMER_APPROVAL / REVIEW / CUSTOMER_REVIEW).
	State string `json:"state"`
	// Audience selects which of the two ServiceNow subflows this reproduces.
	Audience CRApprovalAudience `json:"audience"`
	// Team is the owning team derived from the change request's git reference
	// (Choreo / Asgardeo / MS). Empty for a customer-audience notice: the
	// customer subflow's subject never carried it.
	Team string `json:"team,omitempty"`
	// GroupName is the WSO2 approval group whose members were resolved
	// ("Devops Approval" / "CAB Approval" / "Devops Review"). Empty for a
	// customer-audience notice.
	GroupName string `json:"groupName,omitempty"`
	// RequesterName is the change request's requester, shown in the email body
	// ("<name> requested approval for ...").
	RequesterName string `json:"requesterName,omitempty"`
	// ProjectName is the customer project's name, shown in the body.
	ProjectName string `json:"projectName,omitempty"`
	// ProjectID is the project the change request belongs to. Carried for the
	// sending service's benefit, not this one's: a customer-audience notice
	// links into the customer portal, whose change-request page is nested under
	// the project (/projects/<id>/operations/change-requests/<crId>), so a
	// notice without it cannot be linked at all.
	ProjectID string `json:"projectId,omitempty"`
	// Subject is the fully rendered subject line, so the sending service does
	// not have to reproduce this flow's branch-specific wording.
	Subject string `json:"subject"`
	// Recipients are the resolved email addresses. Never empty: a notice with
	// nobody to send to is not published at all.
	Recipients []string `json:"recipients"`
}
