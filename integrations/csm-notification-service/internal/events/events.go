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

// Package events defines the domain events csm-portal-backend and
// customer-portal-backend publish directly to the event bus (this service
// has no HTTP ingest endpoint — see internal/dispatch and cmd/server/main.go)
// and that this service's consumer reads back to decide what notification to
// send. Validate is the only structural check this service still performs on
// them, since there's no HTTP handler upstream doing it before publish
// anymore.
//
// v1 payloads are still denormalized for display values (names, titles) that
// this service has no other way to obtain — but case links are no longer one
// of them: the four case.* payloads below carry ProjectID/CaseID (and
// CommentID, for case.comment_added) instead of pre-built CaseLink/
// CommentLink strings, and internal/dispatch resolves each recipient's own
// portal-appropriate link itself via internal/recipientlinks. Recipients is
// still caller-supplied, unchanged — this service resolves which *link* a
// recipient gets, not *who* to notify; audience resolution would need its
// own entity-service lookups (watchers/assignee/reporter) that don't exist
// here.
package events

import "encoding/json"

// Type identifies which of the event types below Envelope.Payload holds.
type Type string

const (
	TypeCaseCreated      Type = "case.created"
	TypeCommentAdded     Type = "case.comment_added"
	TypeStatusChanged    Type = "case.status_changed"
	TypeCaseAssigned     Type = "case.assigned"
	TypeCaseAcknowledged Type = "case.acknowledged"
	TypeSeverityChanged  Type = "case.severity_changed"
	TypeIncidentCreated  Type = "incident.created"

	// TypeSLATierReached belongs to internal/slaengine, not internal/dispatch
	// — see SLATierReachedPayload below. Not an email trigger (no
	// Recipients), so dispatch.Handle's switch has no case for it; it's
	// declared here anyway since this is the one place every event Type
	// this service touches is registered. Published by internal/slaengine's
	// own Engine.Tick (polling entity-service's GET /sla-status, not
	// consuming a Kafka registration event — this service no longer has
	// one; see that package's own doc comment for the full redesign).
	TypeSLATierReached Type = "sla.tier_reached"

	// TypeCRApprovalRequested is published by csm-flow-service's
	// cr_approval_notice flow when a change request enters an approval state.
	// Unlike the case.* types, its recipients and subject arrive already
	// resolved: the flow owns the branch-specific wording and the audience
	// lookup, so this service renders and sends rather than deciding who.
	TypeCRApprovalRequested Type = "change_request.approval_requested"

	// TypeCaseBillableStatusChanged is Postgres-data-source-only on the
	// entity-service side, and — like TypeSLATierReached above — not an
	// email/Chat trigger, so dispatch.Handle's switch has no
	// case for it either. Unlike TypeSLATierReached, it isn't even handled
	// by dispatch's own no-op case: internal/timecardengine.Engine consumes
	// it instead, on its own dedicated consumer group (see
	// cmd/server/main.go's TIME_CARD_CONSUMER_GROUP/_COUNT) — because
	// eventbus.Consumer.Run processes one record at a time, fully
	// sequentially (fetch, handle, commit, repeat), so a future bulk update
	// over "several time cards," each its own HTTP round trip to
	// entity-service, must not delay unrelated email/Chat delivery on
	// dispatch's own consumer instance.
	//
	// TODO: internal/timecardengine.Engine.Handle only logs today — the
	// actual reaction (bulk-flipping every time card's billable flag for
	// the case) needs a Postgres time_cards table/repo/service on
	// entity-service first (it has none today; time cards are
	// ServiceNow-only there). entity-service's own Publish call for this
	// event is itself still commented out for the same reason, so this
	// consumer group exists ahead of ever actually receiving one — see
	// that type's own doc comment in entity-service's copy of this file.
	// Declared here anyway, kept in sync by hand with entity-service's own
	// internal/events/events.go, so the two schemas never drift even while
	// this type is otherwise dormant.
	TypeCaseBillableStatusChanged Type = "case.billable_status_changed"

	// TypeProjectContactInvited is published by entity-service's Salesforce
	// membership ingest once a Project_Contact__c in state INVITED /
	// RE-INVITED has been written to Postgres (see that repo's own CLAUDE.md,
	// "Salesforce membership ingest and onboarding steps"). This service is
	// its consumer: dispatch.handleProjectContactInvited provisions the
	// invitee's Asgardeo identity through the SCIM operations service
	// (internal/scim) and sends the invitation email, recording each step's
	// outcome back on entity-service's onboarding-step ledger
	// (internal/entity.RecordOnboardingStep). Keyed by the Salesforce
	// membership Id — see ProjectContactInvitedPayload.
	TypeProjectContactInvited Type = "project_contact.invited"
)

// KnownTypes lists every Type this service accepts, in the order they're
// checked — used both for request validation and for generating docs/errors
// that enumerate valid values.
var KnownTypes = []Type{
	TypeCaseCreated, TypeCommentAdded, TypeStatusChanged, TypeCaseAssigned, TypeCaseAcknowledged, TypeSeverityChanged, TypeIncidentCreated,
	TypeSLATierReached, TypeCaseBillableStatusChanged,
	TypeCRApprovalRequested, TypeCRPlanDateNotice,
	TypeProjectContactInvited,
}

// Envelope is the wire shape of every record on the event bus: Payload's
// shape depends on Type (see the Type constants' matching Payload struct
// below). EntityID is whatever this event is about — a case ID for the
// case.* types, an incident ID for incident.created — and is duplicated at
// the envelope level (also present inside most payloads) because it's used
// as the Kafka record's partition key — see eventbus.Producer.Publish — so
// it must be readable without unmarshaling Payload first. Everything with
// the same EntityID lands on the same partition and is processed in publish
// order.
//
// Deduplicating a retried publish, or two independent callers racing to
// publish the same logical event, is the publishing backend's own concern —
// this service has no database and deliberately doesn't talk to one
// directly.
type Envelope struct {
	Type     Type            `json:"type"`
	EntityID string          `json:"entityId"`
	Payload  json.RawMessage `json:"payload"`
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

// CaseCreatedPayload is TypeCaseCreated's payload — one field per
// notifications.CaseCreatedEmailData value, since case.created currently has
// exactly one reaction (the case-created email). Recipients is who to email
// — the caller (e.g. csm-portal-backend) already knows the audience (case
// watchers, assignee, reporter) at publish time, so it's supplied here
// rather than resolved by this service. ProjectID is required to build the
// customer-portal link (see internal/recipientlinks) — ProjectName is a
// separate, purely-display value shown in the email body, not used for link
// construction.
type CaseCreatedPayload struct {
	ReporterName string `json:"reporterName"`
	ProjectName  string `json:"projectName"`
	ProjectID    string `json:"projectId"`
	CaseID       string `json:"caseId"`
	// CaseNumber is the case's human-readable reference (e.g. "CS0023001")
	// — purely for display in the email body/subject; CaseID (the UUID)
	// remains what's used for link construction and the caseId/entityId
	// match Validate enforces. Optional: internal/dispatch's
	// displayCaseRef falls back to CaseID (the UUID, meaningless to an end
	// user, but better than a blank subject/body) when a publisher hasn't
	// been updated to send CaseNumber yet.
	CaseNumber string `json:"caseNumber,omitempty"`
	// WSO2CaseID is the CSM portal's own case identifier (e.g.
	// "WSO2-1000" — ServiceNow's u_wso2_case_id custom field), distinct
	// from both CaseNumber (ServiceNow's own "CS..." number) and CaseID
	// (the raw UUID) — matches the "<wso2CaseId>/<caseNumber>" pairing the
	// CSM portal frontend already shows (see caseIdentity.ts's
	// caseIdLabel). internal/dispatch's subjectLine uses this in the
	// subject's first slot, falling back to CaseID only when a publisher
	// hasn't sent it yet.
	WSO2CaseID                string   `json:"wso2CaseId,omitempty"`
	CaseTitle                 string   `json:"caseTitle"`
	CaseType                  string   `json:"caseType"`
	Priority                  string   `json:"priority"`
	Product                   string   `json:"product,omitempty"`
	Team                      string   `json:"team,omitempty"`
	CreatedAt                 string   `json:"createdAt"`
	Description               string   `json:"description"`
	IncidentImpactDescription string   `json:"incidentImpactDescription,omitempty"`
	Recipients                []string `json:"recipients"`
}

// CommentAddedPayload is TypeCommentAdded's payload. See CaseCreatedPayload's
// doc comment for why Recipients is here. CaseID must match the envelope's
// EntityID — see Validate's doc comment — same requirement as the other
// three case.* payloads below. CommentID is the new comment's id — appended
// by internal/dispatch as a URL fragment (#<commentId>) to the resolved case
// link, matching the CSM portal frontend's own comment-permalink format
// (CsmCaseCommentBubble sets id={comment.id} and reads location.hash
// directly) — the customer portal has no such fragment handling today, so
// the same suffix is simply inert there, not an error.
type CommentAddedPayload struct {
	Name      string `json:"name"`
	ProjectID string `json:"projectId"`
	CaseID    string `json:"caseId"`
	// CaseNumber — see CaseCreatedPayload's own doc comment.
	CaseNumber string `json:"caseNumber,omitempty"`
	// WSO2CaseID — see CaseCreatedPayload's own doc comment.
	WSO2CaseID  string `json:"wso2CaseId,omitempty"`
	CaseTitle   string `json:"caseTitle"`
	CaseComment string `json:"caseComment"`
	CommentID   string `json:"commentId"`
	// IsInternalNote is true when this comment is an internal note — never
	// customer-visible, and the publisher is expected to have already
	// restricted Recipients accordingly (see entity-service's own
	// CommentAddedPayload.IsInternalNote doc comment; this service doesn't
	// re-check the recipient list itself, since it has no notion of who
	// counts as "internal" beyond what the publisher already decided).
	// dispatch.handleCommentAdded renders a distinct layout for it —
	// RenderInternalNoteEmail instead of RenderCommentAddedEmail — dropping
	// the "Re: <title>" strap and using WSO2CaseID instead of CaseNumber as
	// the case reference, matching an existing internal WSO2-support email
	// format recipients are already used to.
	IsInternalNote bool     `json:"isInternalNote,omitempty"`
	Recipients     []string `json:"recipients"`
}

// StatusChangedPayload is TypeStatusChanged's payload. See
// CaseCreatedPayload's doc comment for why Recipients is here, why
// ProjectID is required, and for CaseNumber.
type StatusChangedPayload struct {
	ProjectID  string `json:"projectId"`
	CaseID     string `json:"caseId"`
	CaseNumber string `json:"caseNumber,omitempty"`
	// WSO2CaseID — see CaseCreatedPayload's own doc comment.
	WSO2CaseID string `json:"wso2CaseId,omitempty"`
	// CaseTitle is used only for the email subject line (dispatch's
	// subjectLine) — optional, same as CaseNumber, so an older publisher
	// still produces a valid (just less descriptive) subject.
	CaseTitle  string   `json:"caseTitle,omitempty"`
	NewStatus  string   `json:"newStatus"`
	Recipients []string `json:"recipients"`
}

// CaseAssignedPayload is TypeCaseAssigned's payload. See CaseCreatedPayload's
// doc comment for why Recipients is here, and for why ProjectID is required.
type CaseAssignedPayload struct {
	AssigneeName  string `json:"assigneeName"`
	AssigneeEmail string `json:"assigneeEmail"`
	ProjectID     string `json:"projectId"`
	CaseID        string `json:"caseId"`
	CaseNumber    string `json:"caseNumber,omitempty"`
	// WSO2CaseID — see CaseCreatedPayload's own doc comment.
	WSO2CaseID string `json:"wso2CaseId,omitempty"`
	// CaseTitle — see StatusChangedPayload's own doc comment.
	CaseTitle  string   `json:"caseTitle,omitempty"`
	Recipients []string `json:"recipients"`
}

// CaseAcknowledgedPayload is TypeCaseAcknowledged's payload — Chat-only,
// unlike every other case.* payload above: acknowledging a case has no
// email reaction, so there's no Recipients/watch-list concept here at all
// (see entity-service's own CaseAcknowledgedPayload doc comment). Severity
// is the raw uppercase severity string (e.g. "CRITICAL"), the same value
// CaseCreatedPayload.Priority carries — dispatch.severityDisplay maps it to
// a display label/color for the Chat card. Product routes this alert to
// the same Google Chat space as the case's own case.created alert, same
// convention as CaseCreatedPayload.Product.
type CaseAcknowledgedPayload struct {
	CaseID     string `json:"caseId"`
	CaseNumber string `json:"caseNumber,omitempty"`
	// WSO2CaseID — see CaseCreatedPayload's own doc comment.
	WSO2CaseID       string `json:"wso2CaseId,omitempty"`
	Severity         string `json:"severity,omitempty"`
	Product          string `json:"product,omitempty"`
	Team             string `json:"team,omitempty"`
	AcknowledgerName string `json:"acknowledgerName"`
}

// SeverityChangedPayload is TypeSeverityChanged's payload. Unlike
// CaseAcknowledgedPayload, this carries Recipients — a severity change has
// both an email reaction (same audience/link-resolution shape as
// StatusChangedPayload/CaseAssignedPayload) and a Google Chat alert
// (Product, same routing convention as CaseCreatedPayload.Product), so
// dispatch.handleSeverityChanged is a two-channel handler like
// handleCaseCreated, not a one-channel handler like handleCaseAcknowledged.
// OldSeverity/NewSeverity are the raw uppercase severity strings (e.g.
// "CRITICAL"), the same convention CaseAcknowledgedPayload.Severity uses —
// dispatch.severityLabelAndColor maps each to its own display label/color.
type SeverityChangedPayload struct {
	ProjectID  string `json:"projectId"`
	CaseID     string `json:"caseId"`
	CaseNumber string `json:"caseNumber,omitempty"`
	// WSO2CaseID — see CaseCreatedPayload's own doc comment.
	WSO2CaseID  string   `json:"wso2CaseId,omitempty"`
	CaseTitle   string   `json:"caseTitle,omitempty"`
	OldSeverity string   `json:"oldSeverity"`
	NewSeverity string   `json:"newSeverity"`
	Product     string   `json:"product,omitempty"`
	Team        string   `json:"team,omitempty"`
	Recipients  []string `json:"recipients"`
}

// IncidentCreatedPayload is TypeIncidentCreated's payload. Unlike the case.*
// events above, this one has two reactions, not one: a Google Chat alert
// (Product/Title/ShortDescription map onto GoogleChatClient.SendIncidentAlert's
// params, alongside the portal link — see below) and a Twilio voice call to
// CallTo, reading Title and ShortDescription aloud.
//
// There is deliberately no IncidentLink field: unlike an earlier version of
// this struct, the "Open in Portal" button target is built by this service
// itself (dispatch.handleIncidentCreated calls
// recipientlinks.Resolver.IncidentLink(entityID)), the same way case.created
// already gets its own portal link built here rather than trusting a
// caller-supplied one. A publisher only needs to know the fact that an
// incident was created, not this service's portal URL configuration.
type IncidentCreatedPayload struct {
	// Product selects which configured Google Chat space receives the alert
	// (e.g. "api-manager"); matched case/whitespace-insensitively against
	// GOOGLE_CHAT_SPACES.
	Product          string `json:"product"`
	Title            string `json:"title"`
	ShortDescription string `json:"shortDescription"`
	// CallTo is the on-call phone number (E.164, e.g. "+14155552671") the
	// voice call is placed to.
	CallTo string `json:"callTo"`
}

// SLATierReachedPayload is TypeSLATierReached's payload — published by
// internal/slaengine.Engine.Tick when a poll of entity-service's GET
// /sla-status shows a clock has newly crossed a tier (50, 75, or 100
// percent elapsed) since the last poll. Nothing in this service consumes it
// yet; it exists for whatever future notification (e.g. a breach-warning
// email) or other system reacts to it.
type SLATierReachedPayload struct {
	CaseID    string `json:"caseId"`
	ClockType string `json:"clockType"`
	Tier      string `json:"tier"`
}

// CaseBillableStatusChangedPayload is the Payload shape for
// TypeCaseBillableStatusChanged — mirrors entity-service's own
// CaseBillableStatusChangedPayload exactly; see that type's own doc comment
// for why LOW severity is the one thing this reacts to and why IsBillable
// is precomputed there rather than left for a consumer to re-derive.
type CaseBillableStatusChangedPayload struct {
	CaseID     string `json:"caseId"`
	IsBillable bool   `json:"isBillable"`
}

// TypeCRPlanDateNotice is published by csm-flow-service's cr_plan_date_notice
// flow — the plan-start-date conversation between WSO2 and a customer. One
// type for all three notices because they differ only in wording and audience.
const TypeCRPlanDateNotice Type = "change_request.plan_date_notice"

// CRPlanDateNoticePayload is TypeCRPlanDateNotice's payload. Mirrors
// csm-flow-service's struct of the same name.
type CRPlanDateNoticePayload struct {
	ChangeRequestID string `json:"changeRequestId"`
	Number          string `json:"number"`
	// Kind is "customer_proposed" (internal audience), or "accepted" /
	// "rejected" (customer audience). It selects the body wording.
	Kind string `json:"kind"`
	// Audience is "internal" or "customer" — picks the portal to link to, and
	// whether the recipient list goes in To or BCC.
	Audience  string `json:"audience"`
	GroupName string `json:"groupName,omitempty"`
	// ActorName is whoever changed the date, already rendered LAST NAME FIRST
	// by the flow, matching the ServiceNow templates' pill order.
	ActorName        string   `json:"actorName,omitempty"`
	ProjectID        string   `json:"projectId,omitempty"`
	ProjectName      string   `json:"projectName,omitempty"`
	ShortDescription string   `json:"shortDescription,omitempty"`
	Description      string   `json:"description,omitempty"`
	Subject          string   `json:"subject"`
	Recipients       []string `json:"recipients"`
}

// CRApprovalRequestedPayload is TypeCRApprovalRequested's payload. Mirrors
// csm-flow-service's copy; keep the two in sync by hand.
type CRApprovalRequestedPayload struct {
	ChangeRequestID string `json:"changeRequestId"`
	// Number is the human-readable CR reference (e.g. "CHG0031234").
	Number string `json:"number"`
	// State is the approval state just entered: ASSESS / AUTHORIZE /
	// CUSTOMER_APPROVAL / REVIEW / CUSTOMER_REVIEW.
	State string `json:"state"`
	// Audience is "internal" (a WSO2 approval group) or "customer" (the
	// project's contacts). It selects the portal the link points at.
	Audience string `json:"audience"`
	// Team is the owning team for an internal notice (Choreo / Asgardeo / MS),
	// empty for a customer one.
	Team string `json:"team,omitempty"`
	// GroupName is the approval group whose members were resolved, empty for a
	// customer notice.
	GroupName     string `json:"groupName,omitempty"`
	RequesterName string `json:"requesterName,omitempty"`
	ProjectName   string `json:"projectName,omitempty"`
	// ProjectID is the project the change request belongs to, needed to build a
	// customer-portal link: that portal nests its change-request page under the
	// project. Absent on an internal notice, which links into the CSM portal.
	ProjectID string `json:"projectId,omitempty"`
	// Subject is the fully rendered subject line. Used verbatim: the flow
	// reproduces ServiceNow's per-branch wording, and re-deriving it here would
	// mean keeping two copies of that in step.
	Subject string `json:"subject"`
	// Recipients are already resolved and de-duplicated. Never empty — a notice
	// with nobody to send to is not published.
	Recipients []string `json:"recipients"`
}

// ProjectContactInvitedPayload is TypeProjectContactInvited's payload —
// mirrors entity-service's own ProjectContactInvitedPayload exactly (keep
// the two in sync by hand, the same way every other shared payload here
// is): everything this service needs to provision the invited person and
// address the invitation, so it never has to re-read Salesforce.
//
// MembershipSfID is the Salesforce Project_Contact__c Id — also the
// envelope's EntityID (Validate enforces the match, like the case.* types'
// CaseID) and the key every onboarding-step write is recorded under.
// ContactSfID is the Salesforce Contact Id, passed through to the step
// ledger for cross-referencing. GivenName/FamilyName may both be empty
// (Salesforce doesn't require a first name) — dispatch falls back to the
// email's local part for display. Roles are the raw Salesforce
// Project_Role__c values (e.g. "Admin", "Portal user"), shown in the email
// when non-empty. IsIntegrationUser=true means the contact is a machine
// account that never signs in: dispatch records IDENTITY and EMAIL as
// SKIPPED and does nothing else. Type is the Salesforce Contact_Type__c
// ("OWN CONTACT" / "PARTNER CONTACT" / an integration-user type) — carried
// for completeness, not used to branch on here today.
type ProjectContactInvitedPayload struct {
	MembershipSfID    string   `json:"membershipSfId"`
	ContactSfID       string   `json:"contactSfId"`
	Email             string   `json:"email"`
	GivenName         string   `json:"givenName"`
	FamilyName        string   `json:"familyName"`
	ProjectName       string   `json:"projectName"`
	ProjectKey        string   `json:"projectKey"`
	Roles             []string `json:"roles"`
	IsIntegrationUser bool     `json:"isIntegrationUser"`
	Type              string   `json:"type"`
	// EventModifiedOn is the Salesforce LastModifiedDate of the membership
	// version this event describes (RFC 3339); dispatch stamps its
	// onboarding-step writes with it. Optional: an empty value means
	// entity-service could not parse the Salesforce date.
	EventModifiedOn string `json:"eventModifiedOn,omitempty"`
	// IsResend marks a deliberate re-invitation — an admin pressing
	// "Resend invitation" in the portal, which entity-service republishes
	// as this same event with the marker set. Optional: an absent value
	// means a normal, first invitation. dispatch then skips the
	// duplicate-invitation ledger check (the whole point of a resend is to
	// send again) and uses the short reminder wording, which claims
	// nothing about whether the account was just created — see
	// dispatch.handleProjectContactInvited.
	IsResend bool `json:"isResend,omitempty"`
}
