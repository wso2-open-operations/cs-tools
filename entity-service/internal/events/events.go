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
	// TypeWorkaroundProvided is published whenever a case's
	// WorkaroundProvided field is set to true via PATCH — see
	// WorkaroundProvidedPayload's own doc comment for why this exists at
	// all.
	TypeWorkaroundProvided Type = "case.workaround_provided"
	TypeIncidentCreated    Type = "incident.created"
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
	// TypeIncidentAssigned is published when an engineer is set as an
	// incident's assignee. It is how the SRE escalation ladder stops: the SRE
	// flow's acknowledgement is "assignee set on incident". The CRE ladder
	// ignores it.
	TypeIncidentAssigned Type = "incident.assigned"
	// TypeProjectContactInvited is Postgres-data-source-only. Published by
	// the Salesforce membership ingest (salesforceEventService) after a
	// Project_Contact__c in state INVITED / RE-INVITED has been written to the
	// database. csm-notification-service consumes it to create the Asgardeo
	// identity (via scim-operations-service) and send the invitation email —
	// see ProjectContactInvitedPayload. Keyed by the Salesforce membership Id.
	TypeProjectContactInvited Type = "project_contact.invited"
	// TypeSLAClockRegister belongs to csm-notification-service's own
	// internal/slaengine, not its internal/dispatch — see
	// SLAClockRegisterPayload's own doc comment. Published once, from
	// sn_case_service.go's publishCaseCreated; unlike every payload above,
	// there is no separate "tier reached"/breach event type here —
	// csm-notification-service's slaengine owns that half of the mechanism
	// entirely (it also sends the Google Chat breach alert directly,
	// without a second event round-trip through this topic).
	TypeSLAClockRegister Type = "sla.clock.register"
	// TypeSLARecalculationRequested asks csm-notification-service's own
	// Redis-held SLA tracking (see that repo's own CLAUDE.md, "SLA
	// breach-alerting engine") to rebuild itself from this service's durable
	// `sla`/`sla_policy` tables -- the CSM portal's "Recalculate SLA" admin
	// action (SLAStatusService.RequestRecalculation), for recovering from a
	// Redis state that no longer agrees with reality. Not an email/Chat
	// trigger, so no Recipients field, same reasoning as
	// TypeKBArticlePublished below. Published once per click, keyed by a
	// fixed entity id (there is no single entity this is "about") -- see
	// SLARecalculationRequestedPayload's own doc comment.
	TypeSLARecalculationRequested Type = "sla.recalculation_requested"
	// TypeKBArticlePublished is published when a KB article transitions
	// to the published state, for csm-notification-service's Flow 2
	// (embedding -> Pinecone) to consume on its own dedicated handling,
	// same reasoning as TypeSLAClockRegister above -- not an email/Chat
	// trigger, so no Recipients field.
	TypeKBArticlePublished Type = "kb.article_published"
	// TypeProjectContactRegistered is published when a membership moves into
	// REGISTERED; csm-notification-service sends the Welcome email.
	TypeProjectContactRegistered Type = "project_contact.registered"
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
	// CaseCommentTruncated -- see CaseCreatedPayload.DescriptionTruncated's
	// own doc comment; identical reasoning and fix, applied to a comment's
	// own body instead of a case description.
	CaseCommentTruncated bool   `json:"caseCommentTruncated,omitempty"`
	CommentID            string `json:"commentId"`
	// IsInternalNote is true when this comment is a work note
	// (domain.CommentTypeWorkNote) — never customer-visible, and Recipients
	// is already filtered to wso2.com addresses only for this case (see
	// snCaseService.publishCommentAdded's own doc comment). Mirrors
	// csm-notification-service's own IsInternalNote field, which renders a
	// distinct email layout for it.
	IsInternalNote bool     `json:"isInternalNote,omitempty"`
	Recipients     []string `json:"recipients"`
	// AuthorEmail is the comment author's own resolved email -- already
	// available at publish time (the same author lookup that resolves Name
	// above), added so csm-notification-service can classify the author as
	// internal/external (the same role-then-domain classification
	// internal/recipientlinks already applies to a *recipient's* email) to
	// decide whether to run frustration detection on this comment. Empty
	// when the author couldn't be resolved -- csm-notification-service skips
	// the check rather than guessing.
	AuthorEmail string `json:"authorEmail,omitempty"`
	// Product is the case's deployed product's display name (e.g. "WSO2 API
	// Manager"), the same value CaseCreatedPayload.Product carries -- purely
	// display, shown on a frustration-detection Chat alert's card, if one is
	// sent.
	Product string `json:"product,omitempty"`
	// Team/IsEvaluationAccount/ProjectOnboardingStatus let
	// csm-notification-service route a frustration-detection Chat alert
	// through chataudience.Resolve the same way an SLA breach alert is
	// routed -- team-based, with the Evaluation/Onboarding/Americas/weekend
	// overlays -- rather than always the fixed Incident Monitor audience.
	// Team is the case's account's CRE team display name (e.g. "Castor"),
	// the same value CaseCreatedPayload.Team carries. IsEvaluationAccount/
	// ProjectOnboardingStatus are Postgres-only facts (see
	// CaseRepository.ProjectOnboardingInfo) and are the zero value on a pure
	// ServiceNow deployment with no Postgres pool configured -- Resolve
	// treats that the same as "not an evaluation account, no onboarding
	// status," not an error.
	Team                    string `json:"team,omitempty"`
	IsEvaluationAccount     bool   `json:"isEvaluationAccount,omitempty"`
	ProjectOnboardingStatus string `json:"projectOnboardingStatus,omitempty"`
	// IsSupportEngineerResponse is true when this comment is a public
	// comment (req.Type == domain.CommentTypeComment) authored by a user
	// holding CSEngineerRole -- entity-service's own "support engineer"
	// vocabulary, the same one the CSM-native SLA engine already uses to
	// complete a case's response clock (see case_service.go's
	// isSupportEngineerAuthor / sn_case_service.go's
	// isSupportEngineerAuthorSN). Lets csm-notification-service's own SLA
	// tracking complete a case's response clock the moment a qualifying
	// reply lands, without needing any identity/role resolution of its
	// own -- the signal entity-service is uniquely positioned to compute,
	// since it owns the role data. False whenever this can't be confirmed
	// (CSEngineerRole unset, the author's email doesn't resolve, or the
	// role lookup fails) -- never guessed. Deliberately no omitempty: false
	// is a meaningful, confirmed answer here ("checked, not a support
	// engineer"), not an absent value -- omitempty would make it
	// indistinguishable from an older publisher that never sent this field
	// at all, which csm-notification-service's own slaEngine needs to tell
	// apart from a real negative.
	IsSupportEngineerResponse bool `json:"isSupportEngineerResponse"`
	// CaseType/EngagementType let csm-notification-service route a
	// frustration-detection Chat alert to the Migration audience instead of
	// the usual team/Incident Monitor routing, when the comment landed on a
	// migration engagement -- the one case this routes by engagement type
	// rather than by team. CaseType mirrors CaseCreatedPayload.CaseType's own
	// convention (strings.ToUpper(derefString(cv.Type)), e.g. "ENGAGEMENT");
	// EngagementType is cv.EngagementType's own lowercase value (e.g.
	// "migration"), unlike CaseType -- kept as-is since it already matches
	// domain.EngagementTypeMigration's own spelling. Both empty for every
	// non-engagement case-like type.
	CaseType       string `json:"caseType,omitempty"`
	EngagementType string `json:"engagementType,omitempty"`
}

// KBArticlePublishedPayload is the Payload shape for
// TypeKBArticlePublished. KnowledgeArticleID mirrors Envelope's own
// EntityID (validated to match, same pattern as CaseID on every other
// payload here) -- kept as an explicit field anyway rather than relying
// on EntityID alone, matching how every other payload in this file does
// it, since a consumer decoding just the payload (without inspecting the
// envelope) still gets a complete, self-describing record.
type KBArticlePublishedPayload struct {
	KnowledgeArticleID string `json:"knowledgeArticleId"`
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
	// CaseType — see CaseCreatedPayload's own doc comment (the same
	// strings.ToUpper(req.Type) convention). security_report_analysis never
	// carries a severity, so csm-notification-service uses this to render a
	// dedicated ack card with no severity line instead of "Unknown".
	CaseType string `json:"caseType,omitempty"`
}

// WorkaroundProvidedPayload is the Payload shape for
// TypeWorkaroundProvided — mirrors csm-notification-service's own
// WorkaroundProvidedPayload. WorkaroundProvided (on UpdateCaseRequest) is
// the one genuine "a workaround was provided" signal anywhere in the
// domain model, and entity-service's own Postgres-side SLAEngineService
// already completes its own workaround clock when it's set to true (see
// SLAEngineService.CompleteWorkaroundClock's own doc comment) — but that's
// a different tracker in a different process from
// csm-notification-service's Redis-based SLA engine (see that repo's own
// CLAUDE.md, "SLA breach-alerting engine"), and nothing published this
// signal to it at all before this event existed: unlike response (a
// qualifying public comment, already carried by case.comment_added's own
// IsSupportEngineerResponse) and resolution (a status change to Closed,
// already carried by case.status_changed), a workaround being provided had
// no event of its own, so that service's own workaround clock could still
// fire a breach alert well after a workaround had genuinely been provided.
// No Recipients/email reaction and no Chat alert — this is a pure tracking
// signal, not a notification. false (a recall) is deliberately never
// published either, mirroring CompleteWorkaroundClock's own "no uncomplete
// operation" posture.
type WorkaroundProvidedPayload struct {
	CaseID string `json:"caseId"`
}

// SLARecalculationRequestedPayload is the Payload shape for
// TypeSLARecalculationRequested -- mirrors csm-notification-service's own
// SLARecalculationRequestedPayload. RequestedByEmail/RequestedAt are purely
// for that service's own log line (whoever clicked it, and when the
// request was made, as opposed to whenever the consumer happens to pick it
// up) -- nothing in the recalculation itself is scoped by them. RequestedAt
// is RFC3339, same convention as this package's other timestamp fields
// (e.g. ElevatedAt in csm-notification-service's own events.go).
type SLARecalculationRequestedPayload struct {
	RequestedByEmail string `json:"requestedByEmail"`
	RequestedAt      string `json:"requestedAt"`
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
	// Team, SRETeamID and SRETeamName: see CaseCreatedPayload's own doc comment.
	Team        string   `json:"team,omitempty"`
	SRETeamID   string   `json:"sreTeamId,omitempty"`
	SRETeamName string   `json:"sreTeamName,omitempty"`
	Recipients  []string `json:"recipients"`
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
	// Team is the case's account's CRE team display name (e.g. "Castor")
	// — cv.AccountDetails.CreTeam.Name, "" when the case has no account or
	// the account has no CRE team assigned. Displayed in
	// csm-notification-service's Chat cards — purely a display value there,
	// no routing role (unlike Product). Depends on ServiceNow's
	// case-embedded account object actually carrying creTeam/sreTeam — see
	// caseTeamName's own doc comment for the current caveat around that.
	Team string `json:"team,omitempty"`
	// SRETeamID/SRETeamName are the account's SRE team (account.sre_team_id); an S0 case's SRE ladder pages it.
	SRETeamID   string `json:"sreTeamId,omitempty"`
	SRETeamName string `json:"sreTeamName,omitempty"`
	CreatedAt   string `json:"createdAt"`
	Description string `json:"description"`
	// DescriptionTruncated is true when Description had its embedded
	// base64-encoded inline image(s) stripped out (replaced with a plain
	// text placeholder) before this event was published -- a raw case
	// description containing a pasted screenshot can carry several MB of
	// base64 image data, which once blew past Event Hub's own per-message
	// size limit and silently failed this entire publish (see this
	// service's own stripInlineBase64Images for the fix). csm-notification-
	// service uses this flag to re-fetch the full, untouched description via
	// GET /cases/{id} -- outside Kafka's size limit entirely -- when
	// building the notification EMAIL, so the email still shows the
	// original image(s); the Google Chat card, which never rendered
	// embedded images from this field to begin with, simply uses
	// Description as sent. omitempty: false is the normal, unremarkable
	// case (nothing was stripped) and must read the same as an older
	// publisher that never sent this field at all.
	DescriptionTruncated bool     `json:"descriptionTruncated,omitempty"`
	Recipients           []string `json:"recipients"`
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
	// ContactType is how the incident was raised, as the incident view spells
	// it: AZURE, SITE_247 or SENTINEL when a monitoring source raised it,
	// EMAIL, PHONE, SELF_SERVICE and so on when a person did. The SRE ladder
	// reads it: a monitoring-raised incident climbs the SRE ladder whatever
	// team it is assigned to, or when it is assigned to none. Optional.
	ContactType string `json:"contactType,omitempty"`

	// The remaining fields feed csm-notification-service's call-escalation
	// ladder (its internal/paging), which needs the priority that keys
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
	// Category, State, Service and Environment fill the consumer's "Incident Reported" Chat card.
	Category    string `json:"category,omitempty"`
	State       string `json:"state,omitempty"`
	Service     string `json:"service,omitempty"`
	Environment string `json:"environment,omitempty"`
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

// ProjectContactInvitedPayload is the payload of TypeProjectContactInvited:
// everything csm-notification-service needs to provision the invited person
// and address the invitation, so it never has to re-read Salesforce. Roles
// are the raw Salesforce Project_Role__c values (e.g. "Admin",
// "Portal user"). IsIntegrationUser=true means: record the identity and email
// steps as SKIPPED — integration users never sign in and get no email. Type is
// the Salesforce Contact_Type__c ("OWN CONTACT" / "PARTNER CONTACT" /
// "RELATED CONTACT"). Mirror any change here in csm-notification-service's
// own copy of this struct.
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
	// version this event describes, RFC 3339 UTC. The consumer stamps its
	// onboarding-step writes with it so a delayed older invitation cannot
	// overwrite a newer one's outcome (the step upsert only applies writes
	// whose eventModifiedOn is not older than the stored one). Empty only
	// when Salesforce returned no parseable date.
	EventModifiedOn string `json:"eventModifiedOn,omitempty"`
	// Resend marks a deliberate re-send of an invitation that was already
	// sent once (POST /projects/{id}/contacts/{email}/resend-invitation).
	// csm-notification-service refuses to send a second invitation for a
	// membership it has already recorded an EMAIL step for -- that guard is
	// what stops a duplicate Salesforce event turning into a duplicate
	// email -- and this field is the one thing that tells it a second send
	// is what was actually asked for. Omitted on every ordinary invitation,
	// so the wire shape is unchanged for them. Mirror any change here in
	// csm-notification-service's own copy of this struct.
	Resend bool `json:"isResend,omitempty"`
}

// ProjectContactRegisteredPayload is the payload of TypeProjectContactRegistered.
// csm-notification-service decodes it strictly: keep both copies identical.
type ProjectContactRegisteredPayload struct {
	MembershipSfID    string `json:"membershipSfId"`
	ContactSfID       string `json:"contactSfId"`
	Email             string `json:"email"`
	GivenName         string `json:"givenName"`
	FamilyName        string `json:"familyName"`
	ProjectName       string `json:"projectName"`
	ProjectKey        string `json:"projectKey"`
	IsIntegrationUser bool   `json:"isIntegrationUser,omitempty"`
	EventModifiedOn   string `json:"eventModifiedOn,omitempty"`
}

// IncidentAssignedPayload is TypeIncidentAssigned's payload, mirrored by
// hand in csm-notification-service's events package.
type IncidentAssignedPayload struct {
	// AssigneeID is the engineer now assigned.
	AssigneeID string `json:"assigneeId"`
	// AssigneeName is for the escalation's execution summary; omitted when
	// ServiceNow returned no display name.
	AssigneeName string `json:"assigneeName,omitempty"`
	// AssignedOn is when the assignee was set, RFC3339 UTC. Case Paging's
	// Special Ops (SME) page compares an alert's changedOn against it, so an
	// assignment handled before the alert it answers still answers it.
	// csm-notification-service decodes strictly: deploy the version that
	// knows the field first.
	AssignedOn string `json:"assignedOn,omitempty"`
	// Number, Team and UpdatedBy fill the consumer's threaded "Incident Acknowledged" Chat reply.
	Number    string `json:"number,omitempty"`
	Team      string `json:"team,omitempty"`
	UpdatedBy string `json:"updatedBy,omitempty"`
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
