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

// Package events defines the wire shape of every record on the case-events
// Kafka topic that internal/service.EventPublisherService produces to. It's
// kept in sync by hand with csm-notification-service's own
// internal/events.Envelope and apps/csm-portal/backend's own copy of this
// same package, since all three live in separate Go modules and none of them
// import each other — every one of them must agree on this shape for any two
// to make sense of each other.
package events

import "encoding/json"

// Type identifies which kind of domain event Envelope.Payload holds. Values
// mirror csm-notification-service's internal/events.Type constants exactly.
type Type string

const (
	// CASE OR INCIDENT? Two different entities. A case is POST /cases and
	// domain.CaseView; an incident is POST /incidents and
	// domain.IncidentView. Their event families are separate for that
	// reason, and a comment on one is not a comment on the other.
	//
	// "SRE incident" is not a third thing: sre-alert-ingestion-service turns
	// a vendor alert into a platform incident through the same
	// POST /incidents, so it produces exactly what the incident.* events
	// describe.
	TypeCaseCreated      Type = "case.created"
	TypeCommentAdded     Type = "case.comment_added"
	TypeStatusChanged    Type = "case.status_changed"
	TypeCaseAssigned     Type = "case.assigned"
	TypeCaseAcknowledged Type = "case.acknowledged"
	TypeSeverityChanged  Type = "case.severity_changed"
	TypeIncidentCreated  Type = "incident.created"
	// TypeIncidentAcknowledged / TypeIncidentPriorityElevated drive the
	// incident call-escalation ladder in csm-notification-service. The ladder
	// starts on incident.created (or a priority elevation) and keeps calling
	// until one of two things happens, per the escalation specification:
	// the incident moves out of NEW ("update the ticket status to Work In
	// Progress to stop further notifications"), or — for an elevation — a
	// public comment is added. incident.acknowledged is the stop signal for
	// the first of those.
	TypeIncidentAcknowledged     Type = "incident.acknowledged"
	TypeIncidentPriorityElevated Type = "incident.priority_elevated"
	// TypeIncidentCommentAdded is the second way a call escalation stops.
	// Section 3.0 gives two acknowledgement gestures, one per trigger: a newly
	// reported incident is acknowledged by moving it to Work In Progress
	// (TypeIncidentAcknowledged), while a PRIORITY ELEVATION is acknowledged
	// by adding a public comment — which is what section 10.0's own voice
	// message instructs an elevation's recipient to do. Without this event an
	// elevation's ladder had no stop signal at all: the incident has normally
	// already left NEW by the time its priority is raised, so
	// TypeIncidentAcknowledged can never fire again for it.
	TypeIncidentCommentAdded Type = "incident.comment_added"
	// TypeCaseBillableStatusChanged is Postgres-data-source-only (unlike
	// every other type here, which is ServiceNow-only) — see
	// CaseBillableStatusChangedPayload's own doc comment for what it's for
	// and why the two data sources aren't symmetric here.
	//
	// TODO: the consumer group plumbing exists on the
	// csm-notification-service side (its own dedicated consumer group,
	// internal/timecardengine.Engine — not folded into dispatch.Dispatcher's
	// group, since eventbus.Consumer.Run processes one record at a time,
	// fully sequentially/blocking, and a bulk update over "several time
	// cards" must not delay unrelated email/Chat delivery on the same
	// consumer instance), but its Handle only logs today — the actual
	// reaction (bulk-flip every time card on the case to match
	// Payload.IsBillable) needs a time_cards table/repo/service on this
	// data source first (it has none today; time cards are
	// ServiceNow-only, see internal/service/sn_time_card_service.go).
	// Publishing this event is therefore still commented out at its one
	// call site (case_service.go's UpdateCase) — the detection logic is
	// real and live, only the actual Publish call is inert, so there's
	// nothing for that consumer to receive yet either.
	TypeCaseBillableStatusChanged Type = "case.billable_status_changed"
	// TypeSLAClockRegister belongs to csm-notification-service's own
	// internal/slaengine, not its internal/dispatch — see
	// SLAClockRegisterPayload's own doc comment. Published once, from
	// sn_case_service.go's publishCaseCreated; unlike every payload above,
	// there is no separate "tier reached"/breach event type here —
	// csm-notification-service's slaengine owns that half of the mechanism
	// entirely (it also sends the Google Chat breach alert directly,
	// without a second event round-trip through this topic).
	TypeSLAClockRegister Type = "sla.clock.register"
)

// Envelope is the wire shape of every record on the case-events topic.
// EntityID is whatever the event is about (a case ID for the case.* types,
// an incident ID for incident.created) and is also the Kafka partition key
// (see eventbus.Producer.Publish) — every event about the same case/incident
// lands on the same partition and is processed in publish order.
type Envelope struct {
	Type     Type            `json:"type"`
	EntityID string          `json:"entityId"`
	Payload  json.RawMessage `json:"payload"`
}

// CommentAddedPayload is the Payload shape for TypeCommentAdded — mirrors
// csm-notification-service's own CommentAddedPayload (its internal/events/
// validate.go is the schema authority; keep this in sync by hand the same
// way Envelope is kept in sync). An earlier version of this struct was
// {Timestamp string} only — deliberately minimal, on the assumption that
// Envelope's own EntityID plus a timestamp was all a consumer would need —
// but csm-notification-service's actual schema requires every field below
// (see its events.Validate), so that version was never actually
// publishable: csm-notification-service would reject it outright. Name is
// the comment author's resolved display name (see
// snCaseService.publishCommentAdded's own doc comment for how this service
// obtains it, since ServiceNow's create-comment response doesn't carry
// one), not the case reporter.
type CommentAddedPayload struct {
	Name       string `json:"name"`
	ProjectID  string `json:"projectId"`
	CaseID     string `json:"caseId"`
	CaseNumber string `json:"caseNumber,omitempty"`
	// WSO2CaseID is ServiceNow's u_wso2_case_id custom field (domain.CaseView.
	// InternalID) — the CSM portal's own case identifier (e.g. "WSO2-1000"),
	// distinct from CaseNumber ("CS..."). Mirrors csm-notification-service's
	// own WSO2CaseID field, used in its subjectLine.
	WSO2CaseID string `json:"wso2CaseId,omitempty"`

	CaseTitle   string `json:"caseTitle"`
	CaseComment string `json:"caseComment"`
	CommentID   string `json:"commentId"`
	// IsInternalNote is true when this comment is a work note
	// (domain.CommentTypeWorkNote) — never customer-visible, and Recipients
	// is already filtered to wso2.com addresses only for this case (see
	// snCaseService.publishCommentAdded's own doc comment). Mirrors
	// csm-notification-service's own IsInternalNote field, which renders a
	// distinct email layout for it.
	IsInternalNote bool     `json:"isInternalNote,omitempty"`
	Recipients     []string `json:"recipients"`
}

// StatusChangedPayload is the Payload shape for TypeStatusChanged — mirrors
// csm-notification-service's own StatusChangedPayload, same reasoning as
// CommentAddedPayload above (an earlier {Timestamp, NewStatus} version was
// similarly never actually publishable).
type StatusChangedPayload struct {
	ProjectID  string `json:"projectId"`
	CaseID     string `json:"caseId"`
	CaseNumber string `json:"caseNumber,omitempty"`
	// WSO2CaseID — see CommentAddedPayload's own doc comment.
	WSO2CaseID string   `json:"wso2CaseId,omitempty"`
	CaseTitle  string   `json:"caseTitle,omitempty"`
	NewStatus  string   `json:"newStatus"`
	Recipients []string `json:"recipients"`
}

// CaseAssignedPayload is the Payload shape for TypeCaseAssigned — mirrors
// csm-notification-service's own CaseAssignedPayload, same reasoning as
// CommentAddedPayload above. AssigneeName/AssigneeEmail identify who the
// case is now assigned *to*, not who performed the assignment: this
// service has no inbound identity layer able to resolve the latter (see
// snCaseService.publishCaseAssigned's own doc comment), but the new
// assignee's email is directly available on the update request with no
// extra lookup needed.
type CaseAssignedPayload struct {
	AssigneeName  string `json:"assigneeName"`
	AssigneeEmail string `json:"assigneeEmail"`
	ProjectID     string `json:"projectId"`
	CaseID        string `json:"caseId"`
	CaseNumber    string `json:"caseNumber,omitempty"`
	// WSO2CaseID — see CommentAddedPayload's own doc comment.
	WSO2CaseID string   `json:"wso2CaseId,omitempty"`
	CaseTitle  string   `json:"caseTitle,omitempty"`
	Recipients []string `json:"recipients"`
}

// CaseAcknowledgedPayload is the Payload shape for TypeCaseAcknowledged —
// mirrors csm-notification-service's own CaseAcknowledgedPayload. Unlike
// every other case.* payload above, this one has no Recipients/email
// audience at all: acknowledging a case only triggers a Google Chat alert
// (see snCaseService.publishCaseAcknowledged's own doc comment for why
// there's no email reaction). Severity is the raw uppercase severity
// string (e.g. "CRITICAL"), the same value CaseCreatedPayload.Priority
// carries — csm-notification-service maps it to a display label/color for
// the Chat card.
type CaseAcknowledgedPayload struct {
	CaseID     string `json:"caseId"`
	CaseNumber string `json:"caseNumber,omitempty"`
	// WSO2CaseID — see CommentAddedPayload's own doc comment.
	WSO2CaseID string `json:"wso2CaseId,omitempty"`
	Severity   string `json:"severity,omitempty"`
	// Product — see CaseCreatedPayload's own doc comment. The acknowledged
	// case's own product, so this alert routes to the same Google Chat
	// space as its case.created alert did.
	Product string `json:"product,omitempty"`
	// Team — see CaseCreatedPayload's own doc comment.
	Team             string `json:"team,omitempty"`
	AcknowledgerName string `json:"acknowledgerName"`
}

// SeverityChangedPayload is the Payload shape for TypeSeverityChanged —
// mirrors csm-notification-service's own SeverityChangedPayload, same
// reasoning as CommentAddedPayload above. Unlike CaseAcknowledgedPayload,
// this one does carry Recipients: a severity change has both an email
// reaction (same audience as case.status_changed/case.assigned — the
// case's watch list) and a Google Chat alert, so it needs both an audience
// and a routing Product, same as CaseCreatedPayload's own doc comment for
// why Product is here.
type SeverityChangedPayload struct {
	ProjectID  string `json:"projectId"`
	CaseID     string `json:"caseId"`
	CaseNumber string `json:"caseNumber,omitempty"`
	// WSO2CaseID — see CommentAddedPayload's own doc comment.
	WSO2CaseID string `json:"wso2CaseId,omitempty"`
	CaseTitle  string `json:"caseTitle,omitempty"`
	// OldSeverity/NewSeverity are raw uppercase severity strings (e.g.
	// "CRITICAL"), the same convention as CaseAcknowledgedPayload.Severity.
	OldSeverity string `json:"oldSeverity"`
	NewSeverity string `json:"newSeverity"`
	// Product — see CaseCreatedPayload's own doc comment.
	Product string `json:"product,omitempty"`
	// Team — see CaseCreatedPayload's own doc comment.
	Team       string   `json:"team,omitempty"`
	Recipients []string `json:"recipients"`
}

// CaseBillableStatusChangedPayload is the Payload shape for
// TypeCaseBillableStatusChanged — published (once a consumer exists — see
// that type's own TODO) when a case's severity crosses into or out of LOW
// on the Postgres data source. Type is always "case" and fixed forever for
// a Postgres-backed case (see case_service.go's UpdateCase, which rejects
// changing Type at all on this data source), so unlike the ServiceNow data
// source — where Type can transfer between case/engagement/service_request
// and severity is only ever meaningful for Type=="case" — the "does this
// case count as S4 (WSO2's own support-policy tier for LOW severity, see
// entity-service's sla_policy.go)" question collapses to a single check:
// is the new severity LOW or not. IsBillable is the resulting target state
// (true entering LOW, false leaving it) — precomputed here rather than left
// for a consumer to re-derive from raw severity strings, since severity's
// mapping to "billable" is business policy this service already owns (the
// same reasoning sla_policy.go already established for SLA durations).
// No Recipients/Product/Team: this event has no notification reaction at
// all, only the (not yet built) time-card side effect.
type CaseBillableStatusChangedPayload struct {
	CaseID     string `json:"caseId"`
	IsBillable bool   `json:"isBillable"`
}

// CaseCreatedPayload is the Payload shape for TypeCaseCreated — mirrors
// csm-notification-service's own CaseCreatedPayload (its internal/events/
// validate.go is the schema authority; keep this in sync by hand the same
// way Envelope above is kept in sync). IncidentImpactDescription is omitted
// here rather than always encoded empty, since this service has no data
// source for it yet — omitting an optional field and encoding it empty are
// equivalent on the wire (see the notification service's decodeStrict).
type CaseCreatedPayload struct {
	ReporterName string `json:"reporterName"`
	ProjectName  string `json:"projectName"`
	ProjectID    string `json:"projectId"`
	CaseID       string `json:"caseId"`
	CaseNumber   string `json:"caseNumber,omitempty"`
	// WSO2CaseID — see CommentAddedPayload's own doc comment.
	WSO2CaseID string `json:"wso2CaseId,omitempty"`
	CaseTitle  string `json:"caseTitle"`
	CaseType   string `json:"caseType"`
	Priority   string `json:"priority"`
	// Product is the case's deployed product's display name (e.g. "WSO2 API
	// Manager") — cv.DeployedProductDetails.Product.Name, "" when the case
	// has no deployed product. Doubles as csm-notification-service's Google
	// Chat space routing key for this event (see that service's own
	// handleCaseCreated/dispatch.NewDispatcher doc comments) — an operator's
	// GOOGLE_CHAT_SPACES config needs a Product entry matching each deployed
	// product's display name for per-product routing to work; otherwise
	// dispatch.Dispatcher falls back to DEFAULT_CHAT_PRODUCT, same as before
	// this field was populated.
	Product string `json:"product,omitempty"`
	// Team is the case's account's CRE team display name (e.g. "Team Nova")
	// — cv.AccountDetails.CreTeam.Name, "" when the case has no account or
	// the account has no CRE team assigned. A purely-display value in
	// csm-notification-service's Chat cards, same as Product; unlike
	// Product, it plays no role in routing. Depends on ServiceNow's
	// case-embedded account object actually carrying creTeam/sreTeam — see
	// caseTeamName's own doc comment for the current caveat around that.
	Team        string   `json:"team,omitempty"`
	CreatedAt   string   `json:"createdAt"`
	Description string   `json:"description"`
	Recipients  []string `json:"recipients"`
}

// IncidentCreatedPayload is the Payload shape for TypeIncidentCreated —
// mirrors csm-notification-service's own IncidentCreatedPayload, which also
// has Product (Google Chat space) and CallTo (on-call phone number) fields,
// and no IncidentLink field at all. All three are deliberately omitted or
// absent here: this service has no product→Chat-space mapping or on-call
// number of its own to supply (csm-notification-service's
// dispatch.Dispatcher substitutes its own configured defaults,
// DEFAULT_CHAT_PRODUCT/INCIDENT_DEFAULT_CALL_TO, when either is absent —
// see that service's events.Validate, which accepts this), and doesn't know
// that service's own portal URL configuration either — the "Open in Portal"
// link is built by csm-notification-service itself
// (recipientlinks.Resolver.IncidentLink(entityID)), the same way it already
// builds case.created's portal link rather than trusting a caller-supplied
// one.
type IncidentCreatedPayload struct {
	Title            string `json:"title"`
	ShortDescription string `json:"shortDescription"`

	// The remaining fields feed csm-notification-service's call-escalation
	// ladder (its internal/escalation), which needs the priority that keys
	// the timing table plus the routing attributes that select recipients.
	// Every one is optional on the wire: publishIncidentCreated resolves them
	// from a post-create read of the incident, and that read is best-effort —
	// when it fails, this event is published with Title/ShortDescription
	// alone, exactly as it was before these fields existed, and the ladder
	// simply does not start. Keep in sync with csm-notification-service's own
	// IncidentCreatedPayload by hand, same as every payload above.
	//
	// Note this struct has no Product or CallTo, unlike the consumer's
	// version: this service has no product-to-Chat-space mapping and no
	// on-call paging system of its own, so it has never supplied either and
	// the consumer substitutes its own configured defaults.
	Number   string `json:"number,omitempty"`
	Priority string `json:"priority,omitempty"`
	Account  string `json:"account,omitempty"`
	Team     string `json:"team,omitempty"`
	// ABTEligible is a POINTER so an absent value stays absent on the wire.
	// It splits the consumer's rule table in half — the ABT rows against the
	// sub-team ones — so "not told" is a different situation from "told no",
	// and it is the situation today: this service has no product-to-BU
	// mapping and never sets it. Sending false would claim an answer nobody
	// gave. Keep in sync with csm-notification-service's own payload.
	ABTEligible *bool `json:"abtEligible,omitempty"`
	// ReportedAt is when the incident was opened, RFC3339. Every call in the
	// ladder is an offset from this rather than from consume time, so a
	// backlogged consumer cannot shift the whole ladder later than the
	// escalation specification intends — the same reasoning
	// SLAClockRegisterPayload.CaseCreatedAt applies to an SLA clock.
	ReportedAt string `json:"reportedAt,omitempty"`
}

// IncidentCommentAddedPayload is the Payload shape for
// TypeIncidentCommentAdded — published by CreateComment when a comment lands
// on an incident.
//
// IsPublic is the whole point of the event. Section 3.0's acknowledgement
// gesture for a priority elevation is a PUBLIC comment; a work note is an
// internal jotting and must not stop anyone's pager. The consumer decides what
// to do with each, rather than this service publishing only the public ones —
// keeping the event a statement of fact, the same way every payload here does.
//
// KNOWN GAP: this carries no author. Incidents have no customer-portal surface
// in this platform (csm-notification-service's recipientlinks builds only a
// CSM /operations/incidents link for them), so a public comment on one is
// written by internal staff in practice and the distinction does not yet
// matter. If incidents ever become customer-visible, an author must be added
// and checked before a comment is allowed to cancel an escalation — otherwise
// a customer's own comment would silence the page meant to get their incident
// attended to. Resolving one needs a follow-up comment search (see
// snCaseService.resolveCommentAuthor), so it is flagged here rather than
// built speculatively.
type IncidentCommentAddedPayload struct {
	// CommentID is the created comment, for traceability in the escalation
	// execution summary.
	CommentID string `json:"commentId"`
	// IsPublic is false for a work note.
	IsPublic bool `json:"isPublic"`
}

// SLAClockRegisterPayload is the Payload shape for TypeSLAClockRegister —
// mirrors csm-notification-service's own SLAClockRegisterPayload exactly;
// keep the two in sync by hand, same reasoning as every payload above.
// Durations is a Go duration string (e.g. "2h") per clock type
// ("response"/"workaround"/"resolution" — see internal/service/
// sla_policy.go), added to CaseCreatedAt (not the publish/consume-time
// "now" — a delayed publish or consumer backlog must not start the SLA
// clock late) by csm-notification-service's slaengine to compute each
// clock's due time. CaseCreatedAt is an RFC3339 timestamp; an empty or
// unparsable value falls back to consume-time "now" (see slaengine's own
// registerClocks). AvoidWeekendDueDate names the subset of those clock
// types whose computed due date must not land on a Saturday/Sunday
// (currently only ever "resolution", for MEDIUM severity's "1 Business
// Week" SLA — see sla_policy.go's slaAvoidWeekendClockTypes) —
// csm-notification-service's slaengine is what actually performs that
// roll-forward, since only it knows the real startedAt/dueAt at consume
// time. The remaining fields (including State — the case's own state at
// registration time, UPPER_SNAKE_CASE, e.g. "WORK_IN_PROGRESS") are purely
// for display in a Google Chat breach card and are stored verbatim on the
// registered sla_clocks row — see domain.SLAClock's own doc comment for
// why they're a point-in-time snapshot, not kept live.
type SLAClockRegisterPayload struct {
	CaseID              string            `json:"caseId"`
	Durations           map[string]string `json:"durations"`
	CaseCreatedAt       string            `json:"caseCreatedAt,omitempty"`
	AvoidWeekendDueDate []string          `json:"avoidWeekendDueDate,omitempty"`
	CaseNumber          string            `json:"caseNumber,omitempty"`
	WSO2CaseID          string            `json:"wso2CaseId,omitempty"`
	CaseTitle           string            `json:"caseTitle,omitempty"`
	CaseType            string            `json:"caseType,omitempty"`
	Product             string            `json:"product,omitempty"`
	Team                string            `json:"team,omitempty"`
	Priority            string            `json:"priority,omitempty"`
	State               string            `json:"state,omitempty"`
}

// IncidentAcknowledgedPayload is the Payload shape for
// TypeIncidentAcknowledged — the signal that cancels a running call
// escalation. Published by UpdateIncident when an incident genuinely leaves
// the NEW state, never on a no-op re-PATCH (same guard reasoning as
// publishSeverityChanged).
//
// Carries no Recipients: nothing is sent to anyone on acknowledgement, it
// only stops what is already running. NewState is included so the consumer
// can distinguish "picked up" (IN_PROGRESS) from a terminal state
// (RESOLVED/CLOSED/CANCELLED), both of which cancel the ladder but mean
// different things in the execution summary.
type IncidentAcknowledgedPayload struct {
	// PreviousState is the state the incident left, e.g. "NEW".
	PreviousState string `json:"previousState"`
	// NewState is the state it moved to, e.g. "IN_PROGRESS".
	NewState string `json:"newState"`
}

// Deliberately no acknowledger identity: UpdateIncidentRequest carries no
// actor, and this service has no way to resolve who performed an update —
// the same reason CaseAssignedPayload stopped claiming to carry an assigner.
// A consumer that needs it must get it from the incident's own activity feed.

// IncidentPriorityElevatedPayload is the Payload shape for
// TypeIncidentPriorityElevated — the second trigger that starts a call
// escalation, alongside incident.created.
//
// Published only when the priority genuinely increases in urgency; a
// downgrade or a no-op re-PATCH publishes nothing. The escalation ladder's
// timings are keyed by the NEW priority, so that is what a consumer schedules
// against.
type IncidentPriorityElevatedPayload struct {
	// OldPriority is the priority before the change, e.g. "MODERATE".
	OldPriority string `json:"oldPriority"`
	// NewPriority is the priority after the change, e.g. "HIGH" — this is
	// what the escalation timings are keyed by, after the consumer maps a
	// domain priority onto the specification's own P0-P4 scale.
	NewPriority string `json:"newPriority"`
	// Title is the incident subject, carried for display only — the escalation
	// voice message is built from priority, account, case id and team, not
	// from this. Optional on purpose: it comes from a nilable ServiceNow
	// field, and an elevation must never be lost because the subject was
	// empty. Consumers treat an empty title the way they treat an absent
	// product, not as a malformed event.
	Title string `json:"title,omitempty"`

	// The remaining fields mirror IncidentCreatedPayload's own escalation
	// inputs and are optional for the same reason. Unlike the created event,
	// these come from the post-PATCH incident this service already holds, so
	// no extra read is needed to populate them.
	Number      string `json:"number,omitempty"`
	Account     string `json:"account,omitempty"`
	Team        string `json:"team,omitempty"`
	ABTEligible *bool  `json:"abtEligible,omitempty"`
	// ElevatedAt is when the priority actually changed, RFC3339.
	ElevatedAt string `json:"elevatedAt,omitempty"`
}
