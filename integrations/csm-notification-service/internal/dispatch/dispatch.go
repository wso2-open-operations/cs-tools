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

// Package dispatch is the consumer side of the event bus: it turns a
// published events.Envelope back into an actual notification send, by
// rendering the matching HTML template (internal/notifications) and calling
// the matching channel client.
package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
)

// emailSender abstracts notifications.EmailClient for testability.
type emailSender interface {
	SendEmail(ctx context.Context, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error
	// SendEmailFrom sends with an explicit sender; "" means the client's
	// own FromAddress. Lets the onboarding invitation use its own sender
	// without a second client and token cache.
	SendEmailFrom(ctx context.Context, from string, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error
	// FromAddress is needed by a handler that BCCs its audience: the email
	// service requires a non-empty To, and the sender is the only address that
	// is always valid and discloses nothing.
	FromAddress() string
}

// googleChatSender abstracts notifications.GoogleChatClient for testability.
// Every method's first parameter is a Chat audience key (see
// internal/chataudience) — case.created/case.acknowledged/
// case.severity_changed all resolve it to the fixed
// chataudience.IncidentMonitor audience below; there is no product-based
// routing left in this service (incident.created has no Chat reaction at
// all — see handleIncidentCreated's own doc comment).
type googleChatSender interface {
	SendCaseCreatedAlert(ctx context.Context, audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error
	SendSecurityReportAnalysisAlert(ctx context.Context, audience, caseNumber, wso2CaseID, productName, title, team, caseLink string) error
	SendCaseAcknowledgedAlert(ctx context.Context, audience, severityLabel, severityColor, caseNumber, wso2CaseID, caseLink, acknowledgerName string) error
	SendSeverityChangedAlert(ctx context.Context, audience, oldSeverityLabel, oldSeverityColor, newSeverityLabel, newSeverityColor, caseNumber, wso2CaseID, title, team, caseLink string) error
}

// callSender abstracts notifications.TwilioClient's MakeCall for testability.
type callSender interface {
	MakeCall(ctx context.Context, to, message string) error
}

// linkResolver abstracts recipientlinks.Resolver for testability.
type linkResolver interface {
	ResolveLinks(ctx context.Context, emails []string, projectID, caseID string) ([]recipientlinks.RecipientLink, error)
	CSMLink(caseID string) string
	ChangeRequestLink(audience, changeRequestID, projectID string) string
}

// Dispatcher turns a published events.Envelope into an actual notification
// send.
//
// Every case.* payload carries its own Recipients list (who to email) — this
// service resolves which portal link each recipient gets (via links, see
// groupByLink), not who to notify: there's no entity-service lookup here for
// watchers/assignee/reporter, so the caller (e.g. csm-portal-backend)
// supplies the audience directly at publish time. incident.created carries
// no recipients field; its call reaction already has its own real
// destination, a phone number, in the event payload itself, and doesn't go
// through links at all.
type Dispatcher struct {
	email      emailSender
	googleChat googleChatSender
	call       callSender
	links      linkResolver

	// emailSendingEnabled (EMAIL_SENDING_ENABLED, the disable-entirely
	// `!= "false"` convention CALL_SENDING_ENABLED below also uses) is
	// checked first, before emailDebugMode: when false, sendPerGroup logs
	// instead of calling SendEmail at all, for every group, regardless of
	// emailDebugMode/emailDebugRecipients — a stronger switch than debug
	// mode's redirect-to-a-test-list, for temporarily silencing email
	// entirely (e.g. while investigating a delivery issue) without also
	// having to stop exercising the rest of the pipeline (link resolution,
	// Chat, Twilio). Does not affect Google Chat or Twilio.
	emailSendingEnabled bool

	// emailDebugMode/emailDebugRecipients (EMAIL_DEBUG_MODE/
	// EMAIL_DEBUG_RECIPIENTS) redirect sendPerGroup's actual SendEmail calls
	// for the four case.* types to emailDebugRecipients instead of each
	// group's real resolved recipients, without touching Twilio or Google
	// Chat — real emails still go out, just to a safe test list rather than
	// real watchers/customers, so a dev/staging deployment can be exercised
	// end-to-end without risking a real mailbox. Link resolution
	// (groupByLink) still runs either way, so this doesn't mask a broken
	// recipientlinks/entity-service path — only the final recipient list is
	// swapped. If emailDebugMode is true but emailDebugRecipients is empty
	// (misconfigured), sendPerGroup logs and skips that group rather than
	// calling SendEmail with zero recipients. Only consulted when
	// emailSendingEnabled is true.
	emailDebugMode       bool
	emailDebugRecipients []string

	// callSendingEnabled is the same kind of killswitch (CALL_SENDING_ENABLED)
	// for incident.created's Twilio call specifically — see
	// handleIncidentCreated's own doc comment.
	callSendingEnabled bool

	// defaultCSMEmailCC (DEFAULT_CSM_EMAIL_CC) is CC'd on every case.*
	// email's CSM-portal-link group ONLY — never the customer-portal
	// group, and never during EMAIL_DEBUG_MODE (a debug run must not leak
	// to this real, shared inbox just because it's redirecting the `to`
	// list to a test address) — see sendPerGroup's own doc comment for
	// exactly how the CSM group is identified (caseLink == the csmLink
	// argument each caller passes, from linkResolver.CSMLink).
	defaultCSMEmailCC []string

	// defaultOnCallNumber is handleIncidentCreated's fallback value for its
	// payload's own CallTo when a publisher omits it — see that function's
	// doc comment for why a publisher (e.g. entity-service) might not know
	// it itself.
	defaultOnCallNumber string

	// doneMu/done track which (record, channel) pairs have already
	// succeeded — see handleIncidentCreated's doc comment for why this
	// exists. In-memory only: this is a stopgap for not having a durable
	// idempotency store yet, not a substitute for one. It's lost on
	// restart, which is fine — a restart-triggered redelivery duplicating
	// one already-succeeded channel is the same accepted at-least-once
	// trade-off documented elsewhere in this package; what this map fixes
	// is the much more likely case, retries within a single process's
	// handling of one record.
	doneMu sync.Mutex
	done   map[string]bool

	// recordsMu/records track, per baseKey (recordBaseKey — one entry per
	// distinct event content, shared by every concurrent Handle call
	// currently processing it), how many calls are currently in flight and
	// whether any of them has hit an error. handleCaseCreated/
	// handleIncidentCreated use this (via beginRecord) to decide whether
	// it's safe to eagerly release their claimed channels before
	// record.NoMoreRetries — see beginRecord's own doc comment for the real
	// bug this closed.
	recordsMu sync.Mutex
	records   map[string]*recordState

	// onboarding is handleProjectContactInvited's configuration — see
	// OnboardingConfig and WithOnboarding. Its zero value (never configured)
	// behaves as both flags off with nowhere to record steps, so a
	// project_contact.invited record is logged and acknowledged rather than
	// retried; cmd/server/main.go always sets it.
	onboarding OnboardingConfig

	// identityExisted (guarded by doneMu, like done) remembers, per baseKey,
	// the Existed result of an identity step that already succeeded on an
	// earlier attempt at the same record — so a retry caused by a later
	// step's failure (the invitation email) doesn't re-run
	// EnsureExternalUser, which would now answer existed=true for a user
	// the previous attempt itself created, and send the "you already have
	// an account" wording to someone who has never been told they have
	// one. Released once the whole record succeeds or record.NoMoreRetries
	// is true — same lifecycle as done; see handleProjectContactInvited.
	// The memo is an optimisation, not the correctness guard: a retry in
	// another process (or after a restart) finds the earlier attempt's
	// IDENTITY row on the ledger instead and falls back to the neutral
	// wording — see identityAlreadyProvisioned.
	identityExisted map[string]bool
}

// Deps are the Dispatcher's outbound collaborators: the three channels and
// the per-recipient portal-link resolver.
type Deps struct {
	Email      emailSender
	GoogleChat googleChatSender
	Call       callSender
	Links      linkResolver
}

// Config are the Dispatcher's switches and defaults, named rather than
// positional so three adjacent booleans cannot be passed in the wrong
// order. The zero value sends nothing by email or call; see the
// Dispatcher field each one sets for what it controls.
type Config struct {
	// EmailSendingEnabled is EMAIL_SENDING_ENABLED (Dispatcher.emailSendingEnabled).
	EmailSendingEnabled bool
	// EmailDebugMode/EmailDebugRecipients are EMAIL_DEBUG_MODE/
	// EMAIL_DEBUG_RECIPIENTS (Dispatcher.emailDebugMode).
	EmailDebugMode       bool
	EmailDebugRecipients []string
	// CallSendingEnabled is CALL_SENDING_ENABLED (Dispatcher.callSendingEnabled).
	CallSendingEnabled bool
	// DefaultOnCallNumber is INCIDENT_DEFAULT_CALL_TO
	// (Dispatcher.defaultOnCallNumber).
	DefaultOnCallNumber string
	// DefaultCSMEmailCC is DEFAULT_CSM_EMAIL_CC (Dispatcher.defaultCSMEmailCC).
	DefaultCSMEmailCC []string
}

// NewDispatcher constructs a Dispatcher from its collaborators and
// configuration.
func NewDispatcher(deps Deps, cfg Config) *Dispatcher {
	return &Dispatcher{
		email:                deps.Email,
		googleChat:           deps.GoogleChat,
		call:                 deps.Call,
		links:                deps.Links,
		emailSendingEnabled:  cfg.EmailSendingEnabled,
		emailDebugMode:       cfg.EmailDebugMode,
		emailDebugRecipients: cfg.EmailDebugRecipients,
		callSendingEnabled:   cfg.CallSendingEnabled,
		defaultOnCallNumber:  cfg.DefaultOnCallNumber,
		defaultCSMEmailCC:    cfg.DefaultCSMEmailCC,
		done:                 make(map[string]bool),
		records:              make(map[string]*recordState),
		identityExisted:      make(map[string]bool),
	}
}

// Handle implements eventbus.Handle. A non-nil return causes the caller
// (eventbus.Consumer) to retry — see its package doc for the retry policy.
func (d *Dispatcher) Handle(ctx context.Context, record eventbus.Record) error {
	var env events.Envelope
	if err := json.Unmarshal(record.Value, &env); err != nil {
		return fmt.Errorf("dispatch: decode envelope: %w", err)
	}
	if !env.Type.IsKnown() {
		return fmt.Errorf("dispatch: unknown event type %q", env.Type)
	}
	// The only validation boundary left in this service: callers publish
	// directly to the event bus now (see events.Validate's doc comment), so
	// nothing has checked this record's required fields before it reaches
	// here.
	if err := events.Validate(env.EntityID, env.Type, env.Payload); err != nil {
		return fmt.Errorf("dispatch: invalid payload: %w", err)
	}

	switch env.Type {
	case events.TypeCaseCreated:
		return d.handleCaseCreated(ctx, record, env.Payload)
	case events.TypeCommentAdded:
		return d.handleCommentAdded(ctx, record, env.Payload)
	case events.TypeStatusChanged:
		return d.handleStatusChanged(ctx, record, env.Payload)
	case events.TypeCaseAssigned:
		return d.handleCaseAssigned(ctx, record, env.Payload)
	case events.TypeCaseAcknowledged:
		return d.handleCaseAcknowledged(ctx, record, env.Payload)
	case events.TypeSeverityChanged:
		return d.handleSeverityChanged(ctx, record, env.Payload)
	case events.TypeIncidentCreated:
		return d.handleIncidentCreated(ctx, record, env.Payload)
	case events.TypeCRApprovalRequested:
		return d.handleCRApprovalRequested(ctx, record, env.Payload)
	case events.TypeCRPlanDateNotice:
		return d.handleCRPlanDateNotice(ctx, record, env.Payload)
	case events.TypeProjectContactInvited:
		return d.handleProjectContactInvited(ctx, record, env.Payload)
	case events.TypeProjectContactRegistered:
		return d.handleProjectContactRegistered(ctx, record, env.Payload)
	case events.TypeSLATierReached:
		// Published by internal/slaengine's own Engine.Tick (a poller, not
		// a consumer of this topic) — nothing here reacts to it yet; it
		// exists for whatever future notification or other system consumes
		// it. Declared in KnownTypes/Validate so a malformed one is still
		// rejected, but this dispatcher's own main/DLQ consumers get a full
		// copy of this topic too and must not dead-letter a record that
		// simply isn't their concern — returning nil (not an error) is
		// required here: erroring would burn this consumer's retries for an
		// event that was never broken.
		return nil
	default:
		return fmt.Errorf("dispatch: unknown event type %q", env.Type)
	}
}
