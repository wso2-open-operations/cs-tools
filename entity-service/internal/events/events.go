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
	TypeCaseCreated      Type = "case.created"
	TypeCommentAdded     Type = "case.comment_added"
	TypeStatusChanged    Type = "case.status_changed"
	TypeCaseAssigned     Type = "case.assigned"
	TypeCaseAcknowledged Type = "case.acknowledged"
	TypeSeverityChanged  Type = "case.severity_changed"
	TypeIncidentCreated  Type = "incident.created"
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
