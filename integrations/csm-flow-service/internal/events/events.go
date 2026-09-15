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

// Package events is the wire schema of the domain events on the shared event
// bus (topic cs-events). It is a copy of the subset of
// csm-notification-service's internal/events that the flow engine consumes —
// the Envelope and the event Type catalog — kept deliberately in sync with
// that service's canonical copy by hand (separate Go modules, neither imports
// the other; see docs/architecture.md §14 and §18). The flow engine reads
// these off the bus to decide which flows to run; it never redefines the
// envelope shape.
package events

import "encoding/json"

// Type identifies which event a record carries.
type Type string

const (
	TypeCaseCreated      Type = "case.created"
	TypeCommentAdded     Type = "case.comment_added"
	TypeStatusChanged    Type = "case.status_changed"
	TypeCaseAssigned     Type = "case.assigned"
	TypeCaseAcknowledged Type = "case.acknowledged"
	TypeSeverityChanged  Type = "case.severity_changed"
	TypeIncidentCreated  Type = "incident.created"

	TypeSLAClockRegister Type = "sla.clock.register"
	TypeSLATierReached   Type = "sla.tier_reached"

	// TypeEntityChanged is the generic change event the outbox will publish
	// once entity-service emits it (docs/architecture.md §16.1). It is the
	// trigger most ported record_update flows will match on. Additive — it
	// sits alongside the typed events above on the same topic.
	TypeEntityChanged Type = "entity.changed"

	// TypeCRApprovalRequested is emitted by the cr_approval_notice flow when a
	// change request enters an approval-relevant state, carrying the already
	// resolved recipient list and the fields the email renders.
	//
	// NOT YET IN csm-notification-service. Unlike every other type in this
	// file, this one does not exist in that service's canonical copy — it is
	// introduced here because the ported flow has to say "notify these people"
	// somehow, and this service is forbidden from sending email itself (see
	// CLAUDE.md). Nothing consumes it until a matching handler is added there,
	// which is a change in that repo, not this one. Keep the two copies in
	// sync by hand, as with every other type here.
	TypeCRApprovalRequested Type = "change_request.approval_requested"
)

// KnownTypes lists every Type the flow engine recognises.
var KnownTypes = []Type{
	TypeCaseCreated, TypeCommentAdded, TypeStatusChanged, TypeCaseAssigned,
	TypeCaseAcknowledged, TypeSeverityChanged, TypeIncidentCreated,
	TypeSLAClockRegister, TypeSLATierReached, TypeEntityChanged,
}

// IsKnown reports whether t is one of KnownTypes.
func (t Type) IsKnown() bool {
	for _, known := range KnownTypes {
		if t == known {
			return true
		}
	}
	return false
}

// Envelope is the wire shape of every record on the event bus. Payload's shape
// depends on Type. EntityID is what the event is about (a case ID for case.*,
// an incident ID for incident.created) and is the Kafka partition key, so it is
// readable without unmarshaling Payload first — everything with the same
// EntityID lands on the same partition and is processed in publish order.
type Envelope struct {
	Type     Type            `json:"type"`
	EntityID string          `json:"entityId"`
	Payload  json.RawMessage `json:"payload"`
}

// EntityChangedPayload is TypeEntityChanged's payload — the generic change
// document conditions evaluate against (docs/architecture.md §6.2). Snapshot is
// the post-write entity; Changes carries before/after pairs per field; Actor is
// who made the change. Fields are decoded lazily by the eval context, so this
// stays a thin typed view over the raw JSON.
type EntityChangedPayload struct {
	EntityType string                    `json:"entityType"`
	EntityID   string                    `json:"entityId"`
	Changes    map[string]map[string]any `json:"changes,omitempty"`
	Snapshot   map[string]any            `json:"snapshot,omitempty"`
	Actor      map[string]any            `json:"actor,omitempty"`
}

// The typed case.* payloads below mirror csm-notification-service's copies —
// the fields the flow engine reads when a flow reacts to one of the existing
// typed events rather than to entity.changed. Only the commonly-read fields
// are kept; extend from the canonical copy if a ported flow needs more.

// CaseCreatedPayload is TypeCaseCreated's payload.
type CaseCreatedPayload struct {
	ReporterName string   `json:"reporterName"`
	ProjectName  string   `json:"projectName"`
	ProjectID    string   `json:"projectId"`
	CaseID       string   `json:"caseId"`
	CaseNumber   string   `json:"caseNumber,omitempty"`
	WSO2CaseID   string   `json:"wso2CaseId,omitempty"`
	CaseTitle    string   `json:"caseTitle"`
	CaseType     string   `json:"caseType"`
	Priority     string   `json:"priority"`
	Product      string   `json:"product,omitempty"`
	Team         string   `json:"team,omitempty"`
	CreatedAt    string   `json:"createdAt"`
	Description  string   `json:"description"`
	Recipients   []string `json:"recipients"`
}

// CommentAddedPayload is TypeCommentAdded's payload.
type CommentAddedPayload struct {
	Name           string   `json:"name"`
	ProjectID      string   `json:"projectId"`
	CaseID         string   `json:"caseId"`
	CaseNumber     string   `json:"caseNumber,omitempty"`
	WSO2CaseID     string   `json:"wso2CaseId,omitempty"`
	CaseTitle      string   `json:"caseTitle"`
	CaseComment    string   `json:"caseComment"`
	CommentID      string   `json:"commentId"`
	IsInternalNote bool     `json:"isInternalNote,omitempty"`
	Recipients     []string `json:"recipients"`
}

// StatusChangedPayload is TypeStatusChanged's payload.
type StatusChangedPayload struct {
	ProjectID  string   `json:"projectId"`
	CaseID     string   `json:"caseId"`
	CaseNumber string   `json:"caseNumber,omitempty"`
	WSO2CaseID string   `json:"wso2CaseId,omitempty"`
	CaseTitle  string   `json:"caseTitle,omitempty"`
	NewStatus  string   `json:"newStatus"`
	Recipients []string `json:"recipients"`
}

// CaseAssignedPayload is TypeCaseAssigned's payload.
type CaseAssignedPayload struct {
	AssigneeName  string   `json:"assigneeName"`
	AssigneeEmail string   `json:"assigneeEmail"`
	ProjectID     string   `json:"projectId"`
	CaseID        string   `json:"caseId"`
	CaseNumber    string   `json:"caseNumber,omitempty"`
	WSO2CaseID    string   `json:"wso2CaseId,omitempty"`
	CaseTitle     string   `json:"caseTitle,omitempty"`
	Recipients    []string `json:"recipients"`
}

// IncidentCreatedPayload is TypeIncidentCreated's payload.
type IncidentCreatedPayload struct {
	Product          string `json:"product"`
	Title            string `json:"title"`
	ShortDescription string `json:"shortDescription"`
	CallTo           string `json:"callTo"`
}

// CRApprovalAudience distinguishes who a CR approval notice is addressed to.
// The ServiceNow flow expressed this structurally — two separate subflows,
// "Send CR Approval For Customer" and "Send CR Approval For Internal" — but
// the only real differences are the recipient source and whether the owning
// team appears in the subject, so one payload with this discriminator carries
// both.
type CRApprovalAudience string

const (
	// CRAudienceCustomer: recipients are the customer project's contacts.
	CRAudienceCustomer CRApprovalAudience = "customer"
	// CRAudienceInternal: recipients are the members of a WSO2 approval group.
	CRAudienceInternal CRApprovalAudience = "internal"
)

// TypeCRPlanDateNotice is published by csm-flow-service's cr_plan_date_notice
// flow: the plan-start-date conversation between WSO2 and a customer. One type
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
