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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/scim"
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
	MakeCall(ctx context.Context, to, message string) (notifications.Call, error)
}

// linkResolver abstracts recipientlinks.Resolver for testability.
type linkResolver interface {
	ResolveLinks(ctx context.Context, emails []string, projectID, caseID string) ([]recipientlinks.RecipientLink, error)
	CSMLink(caseID string) string
	ChangeRequestLink(audience, changeRequestID, projectID string) string
}

// identityProvisioner abstracts scim.Client for testability — the one
// operation project_contact.invited's identity step needs.
type identityProvisioner interface {
	EnsureExternalUser(ctx context.Context, email, givenName, familyName string) (scim.ExternalUser, error)
}

// onboardingStepRecorder abstracts entity.CustomerEntityClient's
// RecordOnboardingStep for testability — the one write this dispatcher
// makes back to entity-service.
type onboardingStepRecorder interface {
	RecordOnboardingStep(ctx context.Context, req entity.OnboardingStepRequest) error
	// SucceededEmailStep returns the SUCCEEDED EMAIL step on the ledger for
	// the membership, or nil when there is none -- the durable "an
	// invitation has already gone out, for this membership version" check
	// (see Dispatcher.invitationAlreadySent).
	SucceededEmailStep(ctx context.Context, membershipSfID string) (*entity.RecordedOnboardingStep, error)
	// SucceededStep is the same lookup for any step (the Welcome guard).
	SucceededStep(ctx context.Context, membershipSfID string, step entity.OnboardingStep) (*entity.RecordedOnboardingStep, error)
}

// OnboardingConfig is everything handleProjectContactInvited needs beyond
// what NewDispatcher already takes — supplied via Dispatcher.WithOnboarding
// rather than as yet more positional NewDispatcher parameters, since the
// whole feature is optional per deployment (both flags default off) and
// every other event type is untouched by it.
//
// Identity (satisfied by *scim.Client) creates the invitee's Asgardeo user
// when IdentityEnabled (CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED); Email sends the
// invitation when EmailEnabled (CSM_MIGRATION_ONBOARD_EMAIL_ENABLED) — a separate
// emailSender from the Dispatcher's own, because the invitation may go out
// from a different sender address (ONBOARD_EMAIL_FROM) than the case.*
// emails, and notifications.EmailClient binds its From at construction.
// Steps (satisfied by *entity.CustomerEntityClient) records each step's
// outcome on entity-service's onboarding-step ledger, best-effort — see
// recordOnboardingStep — and is also read back for the duplicate-invitation
// check. Recording tolerates a nil Steps, but EmailEnabled does not: with
// no ledger to read there is no duplicate check, so the EMAIL step fails as
// a configuration error rather than sending unguarded. PortalURL is the
// sign-in link the invitation points at (ONBOARD_PORTAL_URL).
type OnboardingConfig struct {
	Identity        identityProvisioner
	Email           emailSender
	Steps           onboardingStepRecorder
	IdentityEnabled bool
	EmailEnabled    bool
	PortalURL       string
	// EmailFrom is the invitation's sender (ONBOARD_EMAIL_FROM); "" means
	// Email's own FromAddress.
	EmailFrom string
	// ReplyTo (EMAIL_REPLY_TO) goes on onboarding emails only.
	ReplyTo []string
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
	identityExisted map[string]bool
}

// recordState is recordsMu/records' per-baseKey bookkeeping — see
// beginRecord's doc comment.
type recordState struct {
	refcount  int
	unhealthy bool
}

// NewDispatcher constructs a Dispatcher. See Dispatcher.emailSendingEnabled's,
// Dispatcher.emailDebugMode's, and Dispatcher.callSendingEnabled's doc
// comments for what those three controls do, and
// Dispatcher.defaultOnCallNumber's doc comment for the call fallback value.
func NewDispatcher(email emailSender, googleChat googleChatSender, call callSender, links linkResolver, emailSendingEnabled bool, emailDebugMode bool, emailDebugRecipients []string, callSendingEnabled bool, defaultOnCallNumber string, defaultCSMEmailCC []string) *Dispatcher {
	return &Dispatcher{
		email:                email,
		googleChat:           googleChat,
		call:                 call,
		links:                links,
		emailSendingEnabled:  emailSendingEnabled,
		emailDebugMode:       emailDebugMode,
		emailDebugRecipients: emailDebugRecipients,
		callSendingEnabled:   callSendingEnabled,
		defaultOnCallNumber:  defaultOnCallNumber,
		defaultCSMEmailCC:    defaultCSMEmailCC,
		done:                 make(map[string]bool),
		records:              make(map[string]*recordState),
		identityExisted:      make(map[string]bool),
	}
}

// WithOnboarding configures handleProjectContactInvited (see
// OnboardingConfig) and returns d for chaining. Not part of NewDispatcher's
// parameter list deliberately: the feature is optional per deployment and
// orthogonal to every other event type.
func (d *Dispatcher) WithOnboarding(cfg OnboardingConfig) *Dispatcher {
	d.onboarding = cfg
	return d
}

// beginRecord registers that a call is starting work on baseKey and returns
// a func to call exactly once, when that call is about to return, passing
// whether this call's own attempt hit any error. The returned func's result
// is true only for whichever call happens to be the last one still active
// for baseKey (refcount reaches 0) AND neither it nor any sibling that ran
// concurrently with it ever reported an error — i.e. only then is it
// provably safe to eagerly release baseKey's claimed channels before
// record.NoMoreRetries.
//
// This exists because a plain "am I the only one in flight right now"
// check, taken on its own right before returning, has a real gap: two
// concurrent calls finishing at close enough to the same instant can each
// observe the other still in flight and neither release anything, even
// though refcount is about to hit 0 — silently leaking that baseKey's
// claims forever (nothing will revisit them: NoMoreRetries only ever fires
// on a record that eventually exhausts every attempt, never one that
// succeeds first). Deciding under the same lock that performs the
// decrement — so exactly one caller ever observes "I just brought this to
// 0" — closes that gap.
//
// It also closes the specific bug CodeRabbit flagged in handleCaseCreated:
// a call that lost the claim race for every email group attempts nothing
// for email, so its own error is nil either way; the old release condition
// (chatOwned && no local errors) treated that as "fully done" and released
// chatKey while a different, still in-flight call was genuinely mid-
// SendEmail for that same record. Gating on refcount instead of on this
// call's own narrow view removes that false signal: a losing call's
// sibling is still counted as in flight until it actually returns.
func (d *Dispatcher) beginRecord(baseKey string) func(hadError bool) bool {
	d.recordsMu.Lock()
	st, ok := d.records[baseKey]
	if !ok {
		st = &recordState{}
		d.records[baseKey] = st
	}
	st.refcount++
	d.recordsMu.Unlock()

	return func(hadError bool) bool {
		d.recordsMu.Lock()
		defer d.recordsMu.Unlock()
		if hadError {
			st.unhealthy = true
		}
		st.refcount--
		if st.refcount > 0 {
			return false
		}
		delete(d.records, baseKey)
		return !st.unhealthy
	}
}

// claim atomically reserves key for the current attempt, returning true
// only if it wasn't already claimed — a single lock acquisition, unlike the
// separate check-then-later-mark pattern this replaced (alreadyDone,
// checked before an outbound call, followed by a separate markDone only
// after that call succeeds): two Handle calls racing on the same record
// (e.g. during a Kafka consumer-group rebalance transition — normally
// exclusive per partition, but not something this client's fencing is
// guaranteed to enforce down to the microsecond) could otherwise both
// observe an unclaimed key and both attempt the same outbound call before
// either one marks it done.
//
// Every call site must release a claim it doesn't end up keeping: call
// forget(key) immediately if the outbound call this claim was reserved for
// fails, so a genuine retry can reclaim it. A successful call leaves the
// claim in place — the caller's own full-record-succeeded-or-final-attempt
// check (see handleCaseCreated/handleIncidentCreated/sendPerGroup) is what
// eventually calls forget to release it for good.
func (d *Dispatcher) claim(key string) bool {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	if d.done[key] {
		return false
	}
	d.done[key] = true
	return true
}

// forget removes key — called either to release a claim whose outbound
// call just failed (see claim's doc comment), or once every channel for a
// record has fully succeeded (or record.NoMoreRetries is true), so the map
// doesn't hold onto a successful claim forever.
func (d *Dispatcher) forget(key string) {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	delete(d.done, key)
}

// recordBaseKey builds the per-event prefix every idempotency-tracked
// channel below keys off of. It hashes record.Value (the raw envelope
// bytes) rather than the record's Kafka coordinates (topic/partition/
// offset), which an earlier version of this used: a record that exhausts
// eventbus.Consumer's retries gets published to the dead-letter topic with
// the exact same Value but a brand new topic/partition/offset (see
// cmd/server/main.go's OnExhausted func, which republishes record.Key/
// record.Value unchanged) — keying off coordinates meant the DLQ consumer's
// very first attempt at a dead-lettered record computed a key that had
// never been claimed before, so an already-succeeded channel (e.g. a Chat
// alert sent on the main topic, before some other channel's persistent
// failure sent the record to the DLQ) got reclaimed and resent there. A
// content hash keys the same logical event identically everywhere it's
// delivered, main topic or DLQ.
func recordBaseKey(record eventbus.Record) string {
	sum := sha256.Sum256(record.Value)
	return hex.EncodeToString(sum[:])
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
	case events.TypeIncidentAcknowledged, events.TypeIncidentPriorityElevated, events.TypeIncidentCommentAdded:
		// The incident call-escalation ladder (internal/escalation) owns
		// these three; the notification dispatcher reacts to none of them. Same
		// reasoning as the sla.* case below — erroring here would burn this
		// consumer's retries and dead-letter a perfectly valid event that
		// simply is not this consumer's concern.
		return nil
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
	case events.TypeCaseBillableStatusChanged:
		// internal/timecardengine's own consumer group (a different group
		// ID, so it gets its own full copy of this same topic) is what
		// reacts to this one — same shape as the SLA case above, just with
		// its handler currently log-only rather than a real reaction (see
		// that package's own doc comment: entity-service's Publish call for
		// this event is itself still commented out, so neither consumer
		// group has ever actually received one yet). Returning nil (not an
		// error) is required here for the same reason as the SLA case:
		// erroring would burn this consumer's retries and dead-letter an
		// event that was never broken, just not this consumer's concern.
		return nil
	default:
		return fmt.Errorf("dispatch: unknown event type %q", env.Type)
	}
}

// handleCaseCreated has two independent reactions: the case-created email
// (per resolved recipient link) and a Google Chat alert, always to the
// fixed chataudience.IncidentMonitor audience (no team detection, no
// per-product routing — this service has no real per-product Chat space
// need). The Chat alert always targets the CSM portal's case link
// (links.CSMLink), not a per-recipient link, since there's no per-recipient
// audience for a Chat post the way there is for email.
//
// Every reaction here — each email group and the Chat alert — has
// per-record idempotency tracking: a real-world failure mode this was
// missing until it actually happened — a persistently-failing step (e.g. a
// misconfigured email OAuth2 client) means every one of eventbus.Consumer's
// 3 retries, and then the DLQ consumer's own 3 retries, re-runs this whole
// function, so an unguarded already-succeeded channel would repost/resend
// up to 6 times for one event before the failing one is ever fixed. Email
// groups are tracked inside sendPerGroup itself (see its own doc comment);
// chatKey is tracked here directly. Both kinds are forgotten together,
// once the whole call succeeds (len(errs) == 0, so Handle is about to
// return nil — no more retries coming) or record.NoMoreRetries is true (no
// further retry coming at all, on this topic or the dead-letter one — see
// its doc comment) — never on an individual channel's own success alone,
// which would release it while other channels in this same call are still
// failing and eventbus.Consumer keeps retrying, immediately re-arming that
// channel to resend on the very next attempt. Releasing is also
// ownership-gated for both kinds — see forgetEmailGroups' doc comment for
// the live duplicate-email bug that closed.
func (d *Dispatcher) handleCaseCreated(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.CaseCreatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.created payload: %w", err)
	}

	baseKey := recordBaseKey(record)
	chatKey := baseKey + "/chat"
	endRecord := d.beginRecord(baseKey)

	var errs []error

	groups, groupUserIDs, err := d.groupByLink(ctx, p.Recipients, p.ProjectID, p.CaseID)
	if err != nil {
		errs = append(errs, err)
	} else {
		caseRef := displayCaseRef(p.CaseNumber, p.CaseID)
		subject := subjectLine(p.WSO2CaseID, p.CaseNumber, p.CaseID, p.CaseTitle)
		var emailErr error
		_, emailErr = d.sendPerGroup(ctx, baseKey, groups, groupUserIDs, subject, d.links.CSMLink(p.CaseID), func(caseLink, intendedFor string) (string, []notifications.InlineImage) {
			return notifications.RenderCaseCreatedEmail(notifications.CaseCreatedEmailData{
				ReporterName:              p.ReporterName,
				ProjectName:               p.ProjectName,
				CaseNumber:                caseRef,
				CaseTitle:                 p.CaseTitle,
				CaseType:                  emailCaseTypeLabel(p.CaseType),
				Priority:                  emailSeverityLabel(p.Priority),
				Product:                   p.Product,
				CreatedAt:                 p.CreatedAt,
				Description:               p.Description,
				IncidentImpactDescription: p.IncidentImpactDescription,
				CaseLink:                  caseLink,
				CommentLink:               commentLinkFor(caseLink, ""),
				IntendedFor:               intendedFor,
			})
		})
		if emailErr != nil {
			errs = append(errs, emailErr)
		}
	}

	// Chat is deliberately skipped for entity-service's four non-"case"
	// types (engagement, service_request, security_report_analysis,
	// announcement) — explicit product direction: those types notify their
	// audience by email only. An exclude-list rather than an include-list
	// on purpose: CaseType is a freeform display string in general (only
	// entity-service's own real payloads use this exact UPPER_SNAKE
	// vocabulary), so anything else — including "CASE" itself, an empty
	// value, or an unrecognized one — stays chat-eligible, matching this
	// file's own established "don't suppress on an unrecognized value"
	// convention (see e.g. the unmatched-product Chat-space fallback).
	//
	// Also skipped for a LOW/S4-severity "case" — explicit product
	// direction: S4 is WSO2's own best-efforts support tier and doesn't
	// warrant a Chat alert the way S0-S3 do. Only affects "case" in
	// practice (the other four types never carry a severity at all, so
	// p.Priority is always "" for them, never "LOW").
	//
	// chatKey is simply never claimed when skipped; forgetting an unclaimed
	// key below is a harmless no-op (see Dispatcher.forget), so nothing
	// else in this function needs to change. Always resolves to the fixed
	// chataudience.IncidentMonitor audience (no team detection, no
	// per-product routing — see googleChatSender's own doc comment); an
	// unconfigured audience is a no-op inside SendCaseCreatedAlert itself,
	// not distinguishable here from a genuine send, so there's no "empty
	// product, skip" branch left the way there used to be.
	if !isNonCaseCaseType(p.CaseType) && !isLowSeverity(p.Priority) {
		chatOwned := d.claim(chatKey)
		if chatOwned {
			caseLink := d.links.CSMLink(p.CaseID)
			title := truncateTitle(p.CaseTitle, maxChatTitleLength)
			severityLabel, severityColor := severityLabelAndColor(p.Priority)
			chatErr := d.googleChat.SendCaseCreatedAlert(ctx, chataudience.IncidentMonitor, severityLabel, severityColor, displayCaseRef(p.CaseNumber, p.CaseID), p.WSO2CaseID, p.Product, title, p.Team, caseLink)
			if chatErr != nil {
				errs = append(errs, chatErr)
				d.forget(chatKey)
			}
		}
	}

	// record.NoMoreRetries always forgets every key unconditionally,
	// regardless of ownership or concurrent siblings — see
	// handleIncidentCreated's matching comment for why (and why this must
	// not be record.IsFinalAttempt: a record dead-lettered off the main
	// topic gets a fresh attempt cycle on the DLQ topic under the exact
	// same content key — see recordBaseKey — so releasing on the main
	// topic's own final attempt would reopen an already-succeeded channel
	// to being reclaimed and resent there). Safe because there is no future
	// retry left to ever reclaim-and-resend a key regardless of who
	// currently holds it.
	//
	// Otherwise, only release once endRecord reports it's safe to — i.e.
	// this is the last call still in flight for baseKey, and neither it nor
	// any sibling that ran concurrently with it ever hit an error. Gating
	// on chatOwned && len(errs) == 0 alone (this function's own narrow
	// view, checked before beginRecord existed) was a real bug: a call
	// that lost the claim race for every email group attempts nothing for
	// email, so its own errs stays nil regardless — that let a losing call
	// release chatKey while a different, still in-flight call was
	// genuinely mid-SendEmail for the very same record. See beginRecord's
	// own doc comment for the full reasoning. Once endRecord confirms it's
	// safe, every group in the original groups map is released directly
	// (not just whatever this call itself happened to claim) — same
	// reasoning as the NoMoreRetries branch: confirmed safe regardless of
	// exact ownership.
	// endRecord must run exactly once per call — it decrements beginRecord's
	// refcount — so it's called unconditionally here rather than only
	// inside the else-if, even though its result is ignored on the
	// NoMoreRetries branch.
	safeToRelease := endRecord(len(errs) > 0)
	if record.NoMoreRetries || safeToRelease {
		d.forget(chatKey)
		d.forgetEmailGroups(baseKey, slices.Collect(maps.Keys(groups)))
	}

	return errors.Join(errs...)
}

// handleCommentAdded's email step is tracked the same way handleCaseCreated's
// is (see sendPerGroup's own doc comment) — a group that already sent must
// not resend just because another group in the same record is still
// failing.
func (d *Dispatcher) handleCommentAdded(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.CommentAddedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.comment_added payload: %w", err)
	}
	groups, groupUserIDs, err := d.groupByLink(ctx, p.Recipients, p.ProjectID, p.CaseID)
	if err != nil {
		return err
	}
	baseKey := recordBaseKey(record)
	subject := subjectLine(p.WSO2CaseID, p.CaseNumber, p.CaseID, p.CaseTitle)
	owned, sendErr := d.sendPerGroup(ctx, baseKey, groups, groupUserIDs, subject, d.links.CSMLink(p.CaseID), func(caseLink, intendedFor string) (string, []notifications.InlineImage) {
		if p.IsInternalNote {
			// See events.CommentAddedPayload.IsInternalNote's own doc
			// comment: a distinct layout, and WSO2CaseID (not CaseNumber)
			// as the case reference — this audience is always wso2.com
			// staff, who recognize the internal reference, not ServiceNow's
			// own case number.
			return notifications.RenderInternalNoteEmail(p.Name, displayInternalRef(p.WSO2CaseID, p.CaseID), p.CaseTitle, p.CaseComment, commentLinkFor(caseLink, p.CommentID), caseLink, intendedFor)
		}
		return notifications.RenderCommentAddedEmail(p.Name, displayCaseRef(p.CaseNumber, p.CaseID), p.CaseTitle, p.CaseComment, commentLinkFor(caseLink, p.CommentID), caseLink, intendedFor)
	})
	if record.NoMoreRetries {
		d.forgetEmailGroups(baseKey, slices.Collect(maps.Keys(groups)))
	} else if sendErr == nil {
		d.forgetEmailGroups(baseKey, owned)
	}
	return sendErr
}

// handleStatusChanged's email step is tracked the same way — see
// handleCommentAdded's doc comment.
func (d *Dispatcher) handleStatusChanged(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.StatusChangedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.status_changed payload: %w", err)
	}
	groups, groupUserIDs, err := d.groupByLink(ctx, p.Recipients, p.ProjectID, p.CaseID)
	if err != nil {
		return err
	}
	baseKey := recordBaseKey(record)
	caseRef := displayCaseRef(p.CaseNumber, p.CaseID)
	title := p.CaseTitle
	if title == "" {
		// A publisher that hasn't been updated to send CaseTitle yet still
		// gets a meaningful subject rather than a blank title slot.
		title = "Status changed to " + p.NewStatus
	}
	subject := subjectLine(p.WSO2CaseID, p.CaseNumber, p.CaseID, title)
	owned, sendErr := d.sendPerGroup(ctx, baseKey, groups, groupUserIDs, subject, d.links.CSMLink(p.CaseID), func(caseLink, intendedFor string) (string, []notifications.InlineImage) {
		return notifications.RenderStatusChangedEmail(caseRef, p.NewStatus, caseLink, commentLinkFor(caseLink, ""), intendedFor), nil
	})
	if record.NoMoreRetries {
		d.forgetEmailGroups(baseKey, slices.Collect(maps.Keys(groups)))
	} else if sendErr == nil {
		d.forgetEmailGroups(baseKey, owned)
	}
	return sendErr
}

// handleCRApprovalRequested emails the people a change request is waiting on.
//
// UNLIKE EVERY OTHER HANDLER HERE, it does not resolve recipients or build a
// subject. csm-flow-service's cr_approval_notice flow does both before
// publishing: the audience comes from an approval group or a project's
// contacts, and the subject reproduces ServiceNow's per-branch wording
// verbatim. Re-deriving either here would mean maintaining a second copy of
// logic that only exists to match a system being decommissioned.
//
// It also does not use groupByLink. That splits a case's recipients by which
// portal each should be linked to, and needs a caseID to do it — a change
// request has neither. Internal and customer audiences never share one notice
// (they are separate branches of the original flow), so the audience on the
// payload picks the portal for the whole send.
// crAudienceCustomer is the audience value csm-flow-service publishes for a
// notice bound for a project's contacts rather than a WSO2 approval group.
const crAudienceCustomer = "customer"

func (d *Dispatcher) handleCRApprovalRequested(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.CRApprovalRequestedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode change_request.approval_requested payload: %w", err)
	}
	if len(p.Recipients) == 0 {
		// The flow does not publish a notice with nobody to send to, so this is
		// a malformed event rather than a quiet state. Dropping it beats
		// burning retries on something no retry can fix.
		slog.WarnContext(ctx, "dispatch: change_request.approval_requested with no recipients, dropping",
			"changeRequestId", p.ChangeRequestID, "state", p.State)
		return nil
	}
	if p.Subject == "" {
		slog.WarnContext(ctx, "dispatch: change_request.approval_requested with no subject, dropping",
			"changeRequestId", p.ChangeRequestID)
		return nil
	}

	recipients := p.Recipients
	if !d.emailSendingEnabled {
		slog.InfoContext(ctx, "dispatch: email sending disabled, skipping CR approval notice",
			"changeRequestId", p.ChangeRequestID, "recipients", len(recipients))
		return nil
	}
	var intendedFor string
	if d.emailDebugMode {
		if len(d.emailDebugRecipients) == 0 {
			slog.WarnContext(ctx, "dispatch: email debug mode on with no debug recipients, skipping CR approval notice",
				"changeRequestId", p.ChangeRequestID)
			return nil
		}
		intendedFor = strings.Join(recipients, ", ")
		recipients = d.emailDebugRecipients
	}

	body := notifications.RenderCRApprovalRequestedEmail(notifications.CRApprovalEmailData{
		Number:        p.Number,
		State:         p.State,
		Audience:      p.Audience,
		Team:          p.Team,
		GroupName:     p.GroupName,
		RequesterName: p.RequesterName,
		ProjectName:   p.ProjectName,
		Link:          d.links.ChangeRequestLink(p.Audience, p.ChangeRequestID, p.ProjectID),
		IntendedFor:   intendedFor,
	})

	// A customer audience goes in BCC, an internal one in To.
	//
	// ServiceNow sent one email per recipient, so nobody ever saw who else was
	// notified. Collapsing that into one message is right -- the notice is
	// identical for everyone -- but it must not also publish a customer's
	// contact list to itself: a project's contacts routinely span several
	// organisations, so To would disclose addresses across companies that have
	// no relationship with each other.
	//
	// Internal notices stay in To deliberately. That audience is one WSO2
	// approval group who already know each other, and a visible To is what lets
	// them reply to the group and see that a colleague has picked it up.
	to, bcc := recipients, []string(nil)
	if p.Audience == crAudienceCustomer {
		to, bcc = []string{d.email.FromAddress()}, recipients
	}
	if err := d.email.SendEmail(ctx, to, nil, bcc, nil, p.Subject, body, nil); err != nil {
		return fmt.Errorf("dispatch: send CR approval notice for %s: %w", p.ChangeRequestID, err)
	}
	slog.InfoContext(ctx, "dispatch: CR approval notice sent",
		"changeRequestId", p.ChangeRequestID, "number", p.Number,
		"state", p.State, "audience", p.Audience, "recipients", len(recipients))
	return nil
}

// handleCaseAssigned's email step is tracked the same way — see
// handleCommentAdded's doc comment.
func (d *Dispatcher) handleCaseAssigned(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.CaseAssignedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.assigned payload: %w", err)
	}
	groups, groupUserIDs, err := d.groupByLink(ctx, p.Recipients, p.ProjectID, p.CaseID)
	if err != nil {
		return err
	}
	baseKey := recordBaseKey(record)
	caseRef := displayCaseRef(p.CaseNumber, p.CaseID)
	title := p.CaseTitle
	if title == "" {
		// A publisher that hasn't been updated to send CaseTitle yet still
		// gets a meaningful subject rather than a blank title slot.
		title = "Case assigned"
	}
	subject := subjectLine(p.WSO2CaseID, p.CaseNumber, p.CaseID, title)
	owned, sendErr := d.sendPerGroup(ctx, baseKey, groups, groupUserIDs, subject, d.links.CSMLink(p.CaseID), func(caseLink, intendedFor string) (string, []notifications.InlineImage) {
		return notifications.RenderCaseAssignedEmail(p.AssigneeName, p.AssigneeEmail, caseRef, caseLink, commentLinkFor(caseLink, ""), intendedFor), nil
	})
	if record.NoMoreRetries {
		d.forgetEmailGroups(baseKey, slices.Collect(maps.Keys(groups)))
	} else if sendErr == nil {
		d.forgetEmailGroups(baseKey, owned)
	}
	return sendErr
}

// handleCaseAcknowledged has exactly one reaction, unlike every other
// case.* handler above: a Google Chat alert only — see
// events.CaseAcknowledgedPayload's own doc comment for why there's no
// email/Recipients concept here at all. Always resolves to the fixed
// chataudience.IncidentMonitor audience (see handleCaseCreated's own doc
// comment). With only one channel, there's no cross-channel release race
// to guard against the way beginRecord/endRecord does for
// handleCaseCreated/handleIncidentCreated — this call's own chatOwned
// already fully determines whether it's safe to release: true means this
// call either just sent successfully or found nothing to do, either way a
// real, complete outcome; false means it lost the claim race entirely and
// touched nothing, so it must never release a key a different, still
// in-flight call might rely on.
func (d *Dispatcher) handleCaseAcknowledged(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.CaseAcknowledgedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.acknowledged payload: %w", err)
	}

	chatKey := recordBaseKey(record) + "/chat"
	chatOwned := d.claim(chatKey)
	var chatErr error
	if chatOwned {
		severityLabel, severityColor := severityLabelAndColor(p.Severity)
		caseLink := d.links.CSMLink(p.CaseID)
		chatErr = d.googleChat.SendCaseAcknowledgedAlert(ctx, chataudience.IncidentMonitor, severityLabel, severityColor, displayCaseRef(p.CaseNumber, p.CaseID), p.WSO2CaseID, caseLink, p.AcknowledgerName)
		if chatErr != nil {
			d.forget(chatKey)
			chatOwned = false
		}
	}

	// Deliberately just chatOwned, not "|| record.NoMoreRetries" the way
	// every multi-channel handler's release condition reads: NoMoreRetries
	// force-releases there because a *different* concurrent call may have
	// already claimed-and-succeeded a sibling channel and returned before
	// this call even started, leaving nothing to eventually release it. That
	// scenario can't happen here — there's only one channel/claim total, so
	// whichever call actually owns it (chatOwned true) is the only call
	// that will ever release it, either here on success/skip or inline on
	// failure above. A losing call (chatOwned false) forgetting the key
	// just because NoMoreRetries is also true would release a claim a
	// different, still in-flight call (genuinely mid-SendCaseAcknowledgedAlert)
	// relies on staying held — the same class of bug beginRecord/endRecord
	// closed for handleCaseCreated/handleIncidentCreated, see those doc
	// comments.
	if chatOwned {
		d.forget(chatKey)
	}
	return chatErr
}

// handleSeverityChanged has two independent reactions, like handleCaseCreated
// above: the severity-changed email (per resolved recipient link,
// RenderSeverityChangedEmail) and a Google Chat alert, always to the fixed
// chataudience.IncidentMonitor audience (SendSeverityChangedAlert — see
// handleCaseCreated's own doc comment). Unlike handleCaseAcknowledged
// (Chat-only, one channel, no cross-channel release race — see its own doc
// comment), this needs the same beginRecord/endRecord refcounting
// handleCaseCreated uses, for the exact same reason: two channels means a
// losing call for one channel must not release the other while a
// different, still in-flight call genuinely owns it.
func (d *Dispatcher) handleSeverityChanged(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.SeverityChangedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.severity_changed payload: %w", err)
	}

	baseKey := recordBaseKey(record)
	chatKey := baseKey + "/chat"
	endRecord := d.beginRecord(baseKey)

	var errs []error

	caseRef := displayCaseRef(p.CaseNumber, p.CaseID)
	oldLabel, oldColor := severityLabelAndColor(p.OldSeverity)
	newLabel, newColor := severityLabelAndColor(p.NewSeverity)
	emailOldLabel := emailSeverityLabel(p.OldSeverity)
	emailNewLabel := emailSeverityLabel(p.NewSeverity)

	groups, groupUserIDs, err := d.groupByLink(ctx, p.Recipients, p.ProjectID, p.CaseID)
	if err != nil {
		errs = append(errs, err)
	} else {
		title := p.CaseTitle
		if title == "" {
			// A publisher that hasn't sent CaseTitle still gets a meaningful
			// subject rather than a blank title slot — same fallback
			// handleStatusChanged/handleCaseAssigned use.
			title = "Severity changed to " + emailNewLabel
		}
		subject := subjectLine(p.WSO2CaseID, p.CaseNumber, p.CaseID, title)
		var emailErr error
		_, emailErr = d.sendPerGroup(ctx, baseKey, groups, groupUserIDs, subject, d.links.CSMLink(p.CaseID), func(caseLink, intendedFor string) (string, []notifications.InlineImage) {
			return notifications.RenderSeverityChangedEmail(caseRef, emailOldLabel, emailNewLabel, caseLink, commentLinkFor(caseLink, ""), intendedFor), nil
		})
		if emailErr != nil {
			errs = append(errs, emailErr)
		}
	}

	// The Chat alert is attempted independently of email — a broken
	// recipient-link resolution (groupByLink failing above) shouldn't also
	// suppress the Chat alert, same "both attempted even if one fails"
	// reasoning as handleIncidentCreated's own doc comment, and the same
	// shape handleCaseCreated's own chat block uses (outside its email
	// if/else, not nested inside the success branch).
	chatOwned := d.claim(chatKey)
	if chatOwned {
		caseLink := d.links.CSMLink(p.CaseID)
		title := truncateTitle(p.CaseTitle, maxChatTitleLength)
		if chatErr := d.googleChat.SendSeverityChangedAlert(ctx, chataudience.IncidentMonitor, oldLabel, oldColor, newLabel, newColor, caseRef, p.WSO2CaseID, title, p.Team, caseLink); chatErr != nil {
			errs = append(errs, chatErr)
			d.forget(chatKey)
		}
	}

	// Same release reasoning as handleCaseCreated's own matching comment —
	// see its doc comment for the full explanation of endRecord/
	// record.NoMoreRetries.
	safeToRelease := endRecord(len(errs) > 0)
	if record.NoMoreRetries || safeToRelease {
		d.forget(chatKey)
		d.forgetEmailGroups(baseKey, slices.Collect(maps.Keys(groups)))
	}

	return errors.Join(errs...)
}

// groupByLink resolves each recipient's own case link (see
// recipientlinks.Resolver.ResolveLinks) and buckets recipients by the link
// they resolved to — at most two buckets today, customer portal vs CSM
// portal, so recipients sharing a link still go out in one SendEmail call
// rather than one per person. Recipients is sourced from the triggering
// event's own payload (see the Dispatcher doc comment), not any
// fixed/configured list. An empty Recipients slice should have been
// rejected already by events.Validate; the explicit check here is a
// defensive backstop, not the primary guard.
// groupByLink also returns groupUserIDs, a parallel map from the same
// caseLink keys to each group's recipients' entity-service user ids (empty
// string entries omitted) — purely for logging (see sendPerGroup), never
// used to build a SendEmail call. This repo's own convention is to never
// log a raw recipient email address (see internal/entity's do() doc
// comment); a user id lets a delivery still be traced back to a specific
// recipient without one.
func (d *Dispatcher) groupByLink(ctx context.Context, recipients []string, projectID, caseID string) (groups map[string][]string, groupUserIDs map[string][]string, err error) {
	if len(recipients) == 0 {
		return nil, nil, fmt.Errorf("dispatch: event payload has no recipients")
	}
	links, err := d.links.ResolveLinks(ctx, recipients, projectID, caseID)
	if err != nil {
		return nil, nil, fmt.Errorf("dispatch: resolve recipient links: %w", err)
	}
	groups = make(map[string][]string, 2)
	groupUserIDs = make(map[string][]string, 2)
	for _, l := range links {
		groups[l.CaseLink] = append(groups[l.CaseLink], l.Email)
		if l.UserID != "" {
			groupUserIDs[l.CaseLink] = append(groupUserIDs[l.CaseLink], l.UserID)
		}
	}
	return groups, groupUserIDs, nil
}

// commentLinkFor appends the comment permalink fragment to a resolved case
// link. Fragments are client-side only, so the same suffix works regardless
// of which portal's URL shape caseLink has — see recipientlinks' package doc
// for why the customer portal simply ignores it today rather than erroring.
// An empty commentID (every case.* type except case.comment_added, which
// has no comment to link to) yields the bare case link.
func commentLinkFor(caseLink, commentID string) string {
	if commentID == "" {
		return caseLink
	}
	return caseLink + "#" + url.PathEscape(commentID)
}

// severityDisplay maps a case's raw severity (as entity-service sends it —
// see events.CaseCreatedPayload.Priority's own doc comment, an uppercase
// string like "CRITICAL") to the label/color the case.created and
// case.acknowledged Chat cards use — matching an existing internal
// WSO2-support Chat format: S0-catastrophic, S1-critical, S2-high,
// S3-medium. LOW (S4) is this service's own extrapolation to keep the map
// total over every domain.CaseSeverity value — the reference format never
// showed one, since low-severity cases don't usually get a Chat alert in
// practice.
var severityDisplay = map[string]struct{ label, color string }{
	"CATASTROPHIC": {"Catastrophic (P0)", "#7F1D1D"},
	"CRITICAL":     {"Critical (P1)", "#DC2626"},
	// HIGH's color is deliberately a distinctly orange hue (not a
	// red-leaning orange like Tailwind's orange-600, #EA580C, used
	// originally) — that read too close to CRITICAL's red at a glance in
	// a real Chat card, a live-testing correction.
	"HIGH":   {"High (P2)", "#F97316"},
	"MEDIUM": {"Medium (P3)", "#7C3AED"},
	"LOW":    {"Low (P4)", "#6B7280"},
}

// nonCaseCaseTypes are entity-service's own case.created CaseType values
// (strings.ToUpper(req.Type)) for the four types that never carry a
// severity — engagement/service_request/security_report_analysis/
// announcement, see that service's own CLAUDE.md — for which
// handleCaseCreated skips the Google Chat alert and notifies by email only.
var nonCaseCaseTypes = map[string]bool{
	"ENGAGEMENT":               true,
	"SERVICE_REQUEST":          true,
	"SECURITY_REPORT_ANALYSIS": true,
	"ANNOUNCEMENT":             true,
}

// isNonCaseCaseType reports whether caseType is one of the four types Chat
// is skipped for. See handleCaseCreated's own call site comment for why
// this is an exclude-list, not an include-list.
func isNonCaseCaseType(caseType string) bool {
	return nonCaseCaseTypes[caseType]
}

// isLowSeverity reports whether severity is entity-service's LOW/S4 value
// (case/whitespace-insensitive) — handleCaseCreated's own gate for skipping
// its Google Chat alert on a LOW-severity "case".
func isLowSeverity(severity string) bool {
	return strings.EqualFold(strings.TrimSpace(severity), "LOW")
}

// severityLabelAndColor resolves severity to its Chat display label/color
// (case/whitespace-insensitive), falling back to the raw (trimmed) value
// itself in a neutral gray for a severity this service doesn't recognize —
// or, when severity is blank (it's an optional field on both
// CaseCreatedPayload.Priority and CaseAcknowledgedPayload.Severity), to
// "Unknown" — never blank, so an absent or unrecognized value still
// renders something readable instead of an empty line.
func severityLabelAndColor(severity string) (label, color string) {
	if d, ok := severityDisplay[strings.ToUpper(strings.TrimSpace(severity))]; ok {
		return d.label, d.color
	}
	label = strings.TrimSpace(severity)
	if label == "" {
		label = "Unknown"
	}
	return label, "#6B7280"
}

// emailSeverityLabels maps entity-service's raw uppercase severity value
// (e.g. "HIGH", as sent on CaseCreatedPayload.Priority/SeverityChangedPayload.
// OldSeverity/NewSeverity) to the title-case "<Label>(S<n>)" format shown in
// case.created/case.severity_changed emails — S0..S4 matching entity-service's
// own case_severity_enum labels (CATASTROPHIC=S0 .. LOW=S4, see that
// service's own CLAUDE.md), not the P0..P4 notation severityLabelAndColor
// above uses for Google Chat cards. Deliberately a separate, email-specific
// convention per explicit request — not meant to be reconciled with Chat's
// own labels.
var emailSeverityLabels = map[string]string{
	"CATASTROPHIC": "Catastrophic(S0)",
	"CRITICAL":     "Critical(S1)",
	"HIGH":         "High(S2)",
	"MEDIUM":       "Medium(S3)",
	"LOW":          "Low(S4)",
}

// emailSeverityLabel resolves severity to its email display label
// (case/whitespace-insensitive), falling back to the raw trimmed value for
// anything unrecognized — including blank, which stays blank so an absent
// Priority still renders as an empty field rather than a fabricated label.
func emailSeverityLabel(severity string) string {
	if label, ok := emailSeverityLabels[strings.ToUpper(strings.TrimSpace(severity))]; ok {
		return label
	}
	return strings.TrimSpace(severity)
}

// caseTypeLabels maps entity-service's raw uppercase CaseType value (e.g.
// "SECURITY_REPORT_ANALYSIS", as sent on CaseCreatedPayload.CaseType — see
// that service's own strings.ToUpper(req.Type)) to the title-case wording
// shown in the case-created email's "Case Type" row — same "don't show raw
// enum casing to a reader" reasoning as emailSeverityLabels above.
var caseTypeLabels = map[string]string{
	"CASE":                     "Case",
	"ENGAGEMENT":               "Engagement",
	"SERVICE_REQUEST":          "Service Request",
	"SECURITY_REPORT_ANALYSIS": "Security Report Analysis",
	"ANNOUNCEMENT":             "Announcement",
}

// emailCaseTypeLabel resolves caseType to its email display label
// (case/whitespace-insensitive) via caseTypeLabels, falling back to the raw
// trimmed value for anything unrecognized rather than blanking it out.
func emailCaseTypeLabel(caseType string) string {
	if label, ok := caseTypeLabels[strings.ToUpper(strings.TrimSpace(caseType))]; ok {
		return label
	}
	return strings.TrimSpace(caseType)
}

// maxChatTitleLength bounds truncateTitle's output — long enough to still
// be informative in a Chat card, short enough that a card doesn't dominate
// the space with one case's title.
const maxChatTitleLength = 140

// truncateTitle shortens title to at most max runes, appending "..." when
// it had to cut — rune-based (not byte-based) so a multi-byte character
// never gets split mid-encoding.
func truncateTitle(title string, max int) string {
	r := []rune(title)
	if len(r) <= max {
		return title
	}
	return string(r[:max]) + "..."
}

// displayCaseRef returns caseNumber (the case's human-readable reference,
// e.g. "CS0023001") when the publisher supplied one, falling back to
// caseID (a UUID, meaningless to an end user) only so a subject/body line
// is never blank while every publisher is on a version of the schema that
// carries CaseNumber — see CaseCreatedPayload.CaseNumber's own doc comment.
func displayCaseRef(caseNumber, caseID string) string {
	if caseNumber != "" {
		return caseNumber
	}
	return caseID
}

// displayInternalRef returns wso2CaseID (the CSM portal's own case
// identifier, e.g. "WSO2-1000" — ServiceNow's u_wso2_case_id custom field,
// see events.CaseCreatedPayload.WSO2CaseID's own doc comment) when the
// publisher supplied one, falling back to caseID (the raw UUID, meaningless
// to an end user) only so the subject is never blank while a publisher
// hasn't been updated to send it yet.
func displayInternalRef(wso2CaseID, caseID string) string {
	if wso2CaseID != "" {
		return wso2CaseID
	}
	return caseID
}

// subjectLine builds every case.* email's subject in this service's one
// standard format: "[WSO2 Support] (<wso2 case id>/<case number>) <title>" —
// matching the CSM portal frontend's own "wso2CaseId / caseNumber" pairing
// (see caseIdentity.ts's caseIdLabel). The first slot is
// displayInternalRef(wso2CaseID, caseID) (falls back to the raw UUID only if
// a publisher hasn't sent WSO2CaseID yet), the second is
// displayCaseRef(caseNumber, caseID) (same fallback reasoning for
// CaseNumber). title is empty for a publisher that hasn't been updated to
// send CaseTitle yet (case.status_changed/case.assigned did not originally
// carry one) — still a valid, if less descriptive, subject rather than a
// missing one.
func subjectLine(wso2CaseID, caseNumber, caseID, title string) string {
	return fmt.Sprintf("[WSO2 Support] (%s/%s) %s", displayInternalRef(wso2CaseID, caseID), displayCaseRef(caseNumber, caseID), title)
}

// maskPhone redacts all but the last 4 characters of an E.164 phone number
// for logging — this repo's own convention is to log only ids and sanitised
// summaries, not raw PII, and a phone number is PII the same way a recipient
// email address is (see internal/recipientlinks' own equivalent reasoning).
// A number with 4 or fewer characters (never valid E.164, but defensive
// against a malformed default) is masked entirely rather than echoed as-is.
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return strings.Repeat("*", len(phone))
	}
	return strings.Repeat("*", len(phone)-4) + phone[len(phone)-4:]
}

// sendPerGroup renders and sends one email per distinct resolved link, in
// sorted link order (deterministic, rather than Go's randomized map
// iteration). render is called once per group with that group's own case
// link, so each group's body carries the portal link its recipients can
// actually open.
//
// Each group's own send is tracked with the same per-record idempotency
// mechanism handleIncidentCreated's channels use (alreadyDone/markDone),
// keyed by baseKey (see recordBaseKey) plus the group's own case link — a
// retry that resends because some OTHER group (or, for case.created, the
// Chat alert) is still failing must not resend a group that already
// succeeded. This only marks a group done on success; it never calls
// forget itself, since sendPerGroup doesn't know whether some other channel
// in the same caller (e.g. handleCaseCreated's Chat alert) still needs to
// succeed too before it's safe to release tracking — see
// forgetEmailGroups, which every caller invokes once it knows the whole
// record's outcome.
//
// When emailDebugMode is true, each group's real recipients are replaced
// with emailDebugRecipients before sending — the email still actually goes
// out (unlike the old EMAIL_SENDING_ENABLED=false log-only killswitch this
// replaced), just to a safe configured test list instead of real
// watchers/customers. A group is skipped entirely (logged, marked done —
// retrying won't fix a missing debug-recipient config) if emailDebugMode is
// true but emailDebugRecipients is empty — sending to zero recipients would
// either be rejected by the email provider or silently do nothing, neither
// of which is better than not calling it at all. render's second argument,
// intendedFor, is only ever non-empty on a debug-redirected send (the
// group's real, pre-redirect recipients, joined) — a real send always
// passes "", so every template's "Sent to:" row is omitted entirely rather
// than shown on production mail.
// inlineImageExtensions maps an InlineImage's ContentType to the file
// extension its EmailAttachment.ContentName is given — email-service
// requires a contentName on every attachment, but an inline image's name is
// otherwise never shown to the recipient (Content-Disposition: inline, not
// attachment), so any reasonably-shaped name satisfies that requirement.
// Falls back to no extension for a content type outside this small,
// deliberately narrow list — sanitizeRichText's own safeImageDataURI regex
// only ever admits one of these exact raster subtypes (never a wildcard
// "image/*", which would also let through image/svg+xml — XML, not a
// raster format, and capable of carrying active content), so this covers
// every real case; keep the two lists in sync if either ever changes.
var inlineImageExtensions = map[string]string{
	"image/png":  ".png",
	"image/jpg":  ".jpg",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// inlineAttachments converts sanitizeRichText's own extracted images into
// the EmailAttachment shape EmailClient.SendEmail expects, each marked
// Inline with its matching ContentID — the exact pairing the returned HTML
// body's own cid:<contentId> references depend on.
func inlineAttachments(images []notifications.InlineImage) []notifications.EmailAttachment {
	if len(images) == 0 {
		return nil
	}
	attachments := make([]notifications.EmailAttachment, len(images))
	for i, img := range images {
		attachments[i] = notifications.EmailAttachment{
			ContentName: img.ContentID + inlineImageExtensions[img.ContentType],
			ContentType: img.ContentType,
			Attachment:  img.Data,
			Inline:      true,
			ContentID:   img.ContentID,
		}
	}
	return attachments
}

// sendPerGroup sends one email per distinct resolved link group — see
// groupByLink's own doc comment. csmLink (linkResolver.CSMLink(caseID),
// resolved by every caller the same way) identifies which of the (at most
// two) groups is the CSM-portal one: only that group's SendEmail call gets
// d.defaultCSMEmailCC on its cc list, and only when emailDebugMode is
// false — a debug run redirects `to` to a safe test list specifically so
// nothing goes to a real mailbox, and CC'ing the real, shared default
// inbox regardless would defeat that. The customer-portal group (any
// caseLink other than csmLink) never gets this CC, under any
// circumstances — see Dispatcher.defaultCSMEmailCC's own doc comment.
func (d *Dispatcher) sendPerGroup(ctx context.Context, baseKey string, groups, groupUserIDs map[string][]string, subject, csmLink string, render func(caseLink, intendedFor string) (string, []notifications.InlineImage)) ([]string, error) {
	var errs []error
	var owned []string
	for _, caseLink := range slices.Sorted(maps.Keys(groups)) {
		key := baseKey + "/email/" + caseLink
		if !d.claim(key) {
			continue
		}
		to := groups[caseLink]
		if !d.emailSendingEnabled {
			slog.InfoContext(ctx, "dispatch: email sending disabled (EMAIL_SENDING_ENABLED=false); not sending", "subject", subject)
			owned = append(owned, caseLink)
			continue
		}
		var cc []string
		if caseLink == csmLink && len(d.defaultCSMEmailCC) > 0 {
			cc = d.defaultCSMEmailCC
		}
		var intendedFor string
		if d.emailDebugMode {
			if len(d.emailDebugRecipients) == 0 {
				slog.WarnContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true but EMAIL_DEBUG_RECIPIENTS is empty; not sending",
					"subject", subject)
				owned = append(owned, caseLink)
				continue
			}
			slog.InfoContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true; redirecting email to configured debug recipients",
				"subject", subject, "realRecipientCount", len(to), "debugRecipientCount", len(d.emailDebugRecipients))
			intendedFor = strings.Join(to, ", ")
			to = d.emailDebugRecipients
			cc = nil
		}
		htmlBody, images := render(caseLink, intendedFor)
		if err := d.email.SendEmail(ctx, to, cc, nil, nil, subject, htmlBody, inlineAttachments(images)); err != nil {
			errs = append(errs, err)
			d.forget(key)
			continue
		}
		// Log the recipients' entity-service user ids, never the email
		// addresses themselves (this repo's own "no recipient emails in
		// logs" convention — see internal/entity's do() doc comment) —
		// still traceable to a specific recipient without raw PII.
		slog.InfoContext(ctx, "dispatch: email sent", "subject", subject, "recipientCount", len(to), "recipientUserIds", groupUserIDs[caseLink])
		owned = append(owned, caseLink)
	}
	return owned, errors.Join(errs...)
}

// forgetEmailGroups releases sendPerGroup's per-group idempotency tracking
// for every caseLink in caseLinks. Every caller (handleCaseCreated/
// handleCommentAdded/handleStatusChanged/handleCaseAssigned) uses this two
// different ways depending on which release condition just triggered:
//
//   - record.NoMoreRetries (no further retry coming at all, ever) passes
//     every caseLink in the original groups map — safe precisely because
//     there is no future retry left to ever reclaim-and-resend a key
//     regardless of who currently holds it, the same reasoning
//     handleIncidentCreated's own NoMoreRetries branch uses for chatKey/
//     callKey directly.
//   - the whole call succeeding this round (sendErr == nil, or
//     chatOwned && len(errs) == 0 for handleCaseCreated) passes owned —
//     sendPerGroup's own return value, i.e. exactly the groups THIS call
//     actually claimed. This is the one that must stay ownership-gated: a
//     future retry IS still coming on this branch, so releasing a group
//     this call never claimed (some other, still in-flight call already
//     owns it — see claim's doc comment for when two Handle calls can race
//     on the very same record) would let that other call's key get
//     reclaimed and resent by the next retry while the original call is
//     still mid-send. This was a real, live-observed duplicate-email bug
//     before sendPerGroup returned owned at all (every group in the
//     caller's groups map was released together, regardless of which ones
//     this call actually sent).
//
// Ranging over a nil/empty caseLinks slice (e.g. handleCaseCreated's
// groupByLink failed this attempt, so sendPerGroup was never even called)
// is a safe no-op.
func (d *Dispatcher) forgetEmailGroups(baseKey string, caseLinks []string) {
	for _, caseLink := range caseLinks {
		d.forget(baseKey + "/email/" + caseLink)
	}
}

// handleIncidentCreated has exactly one reaction — a voice call — unlike
// this file's other two-reaction handlers: incident.created no longer has
// a Google Chat alert at all, per explicit product direction (an incident
// pages on-call directly; a separate Chat post was redundant with that).
// entity-service's own IncidentCreatedPayload.Product field is still
// accepted on the wire (decode-compatibility, unused) but no longer read
// here — see that field's own doc comment.
//
// CallTo falls back to Dispatcher.defaultOnCallNumber when the payload's
// own value is empty — a publisher that has no way to determine on-call
// rotations itself (e.g. entity-service) can omit it entirely; events.Validate
// allows this. A publisher that does know the right number per incident can
// still supply it and takes precedence over the default. If the resolved
// number is still empty (payload and default both unset), the call is
// skipped (logged, treated as succeeded) instead of calling MakeCall with
// an empty destination — that would just return a real error, which would
// otherwise burn all of eventbus.Consumer's retries and dead-letter an
// incident whose only problem is a missing operator default, not a
// transient failure.
//
// callSendingEnabled gates the whole call step (CALL_SENDING_ENABLED): when
// false, this logs what would have been called instead of calling, and
// still marks it "done" so a disabled call doesn't retry forever — the same
// log-only shape sendPerGroup's email sending used to have before
// EMAIL_DEBUG_MODE replaced it with a redirect-to-a-test-list behavior (see
// sendPerGroup's doc comment); calls have no equivalent debug-recipient
// concept, so this keeps the simpler disable-entirely shape.
//
// With only one channel left, there's no cross-channel release race to
// guard against the way beginRecord/endRecord exists for elsewhere in this
// file — this call's own callOwned already fully determines whether it's
// safe to release, the same reasoning handleCaseAcknowledged's own doc
// comment gives for its own single-channel shape. claim/forget keys on this
// specific record (recordBaseKey, unique per event content, not per Kafka
// delivery — see its doc comment): eventbus.Consumer retries this whole
// function on any error, and without tracking, a transient failure would
// resend an already-succeeded call on every retry too.
func (d *Dispatcher) handleIncidentCreated(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.IncidentCreatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode incident.created payload: %w", err)
	}

	callTo := p.CallTo
	if callTo == "" {
		callTo = d.defaultOnCallNumber
	}

	callKey := recordBaseKey(record) + "/call"
	callOwned := d.claim(callKey)
	var callErr error
	if callOwned {
		switch {
		case !d.callSendingEnabled:
			slog.InfoContext(ctx, "dispatch: call sending disabled (CALL_SENDING_ENABLED=false); not calling", "to", maskPhone(callTo))
		case callTo == "":
			slog.WarnContext(ctx, "dispatch: no callTo for incident.created (payload and INCIDENT_DEFAULT_CALL_TO both empty); skipping call")
		default:
			message := fmt.Sprintf("New incident: %s. %s", p.Title, p.ShortDescription)
			var placed notifications.Call
			placed, callErr = d.call.MakeCall(ctx, callTo, message)
			if callErr == nil {
				slog.InfoContext(ctx, "dispatch: incident call placed",
					"incident", p.Number, "to", maskPhone(callTo),
					"callSid", placed.SID, "callStatus", placed.Status)
			}
			if callErr != nil {
				d.forget(callKey)
				callOwned = false
			}
		}
	}

	// Deliberately just callOwned, not "|| record.NoMoreRetries" — see
	// handleCaseAcknowledged's own doc comment for why that would be wrong
	// with only one channel/claim total: whichever call actually owns it
	// is the only call that will ever release it.
	if callOwned {
		d.forget(callKey)
	}
	return callErr
}

// handleCRPlanDateNotice emails one turn of the plan-start-date conversation:
// a customer proposing a new date (internal audience), or WSO2 accepting or
// rejecting one (customer audience).
//
// Same division of labour as handleCRApprovalRequested — the flow resolves the
// recipients and builds the subject, both reproduced from ServiceNow verbatim,
// and this service renders and sends. The audience decides two things here:
// which portal the link points at, and whether the recipient list is visible.
func (d *Dispatcher) handleCRPlanDateNotice(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.CRPlanDateNoticePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode change_request.plan_date_notice payload: %w", err)
	}
	if len(p.Recipients) == 0 || p.Subject == "" {
		slog.WarnContext(ctx, "dispatch: plan date notice with no recipients or subject, dropping",
			"changeRequestId", p.ChangeRequestID, "kind", p.Kind)
		return nil
	}

	recipients := p.Recipients
	if !d.emailSendingEnabled {
		slog.InfoContext(ctx, "dispatch: email sending disabled, skipping plan date notice",
			"changeRequestId", p.ChangeRequestID, "recipients", len(recipients))
		return nil
	}
	var intendedFor string
	if d.emailDebugMode {
		if len(d.emailDebugRecipients) == 0 {
			slog.WarnContext(ctx, "dispatch: email debug mode on with no debug recipients, skipping plan date notice",
				"changeRequestId", p.ChangeRequestID)
			return nil
		}
		intendedFor = strings.Join(recipients, ", ")
		recipients = d.emailDebugRecipients
	}

	body, images := notifications.RenderCRPlanDateNoticeEmail(notifications.CRPlanDateEmailData{
		Kind:             p.Kind,
		Number:           p.Number,
		ActorName:        p.ActorName,
		ProjectName:      p.ProjectName,
		ShortDescription: p.ShortDescription,
		Description:      p.Description,
		Link:             d.links.ChangeRequestLink(p.Audience, p.ChangeRequestID, p.ProjectID),
		IntendedFor:      intendedFor,
	})

	// Customer contacts go in BCC for the same reason as the approval notice:
	// a project's contacts span organisations, and ServiceNow sent these one
	// per person so nobody ever saw the rest of the list.
	to, bcc := recipients, []string(nil)
	if p.Audience == crAudienceCustomer {
		to, bcc = []string{d.email.FromAddress()}, recipients
	}
	if err := d.email.SendEmail(ctx, to, nil, bcc, nil, p.Subject, body, inlineAttachments(images)); err != nil {
		return fmt.Errorf("dispatch: send plan date notice for %s: %w", p.ChangeRequestID, err)
	}
	slog.InfoContext(ctx, "dispatch: plan date notice sent",
		"changeRequestId", p.ChangeRequestID, "number", p.Number,
		"kind", p.Kind, "audience", p.Audience, "recipients", len(recipients))
	return nil
}

// handleProjectContactInvited is the consumer side of the customer
// onboarding flow (Salesforce membership → entity-service's Postgres →
// Asgardeo identity → invitation email → first-access registration): two
// sequential steps, IDENTITY then EMAIL, each recorded on entity-service's
// onboarding-step ledger (recordOnboardingStep) and each behind its own
// deployment flag (OnboardingConfig.IdentityEnabled/EmailEnabled, both
// default off).
//
// Unlike every case.* handler, there is nothing to resolve: the invitee is
// the payload's own single email address (no recipient list, no
// groupByLink, no CC), and the sign-in link is one configured portal URL,
// not a per-recipient case link. Sequential, not two independent
// reactions like handleCaseCreated's email+Chat: the email's wording
// depends on the identity step's answer (a just-created account gets the
// "welcome" template, an account that already existed gets "the project
// was added"), so a failed identity step returns before any email is
// attempted — the invitee must not be told to sign in to an account that
// doesn't exist.
//
// A resend (ProjectContactInvitedPayload.IsResend, set by entity-service
// when an admin presses "Resend invitation") is the one case that skips
// the duplicate-invitation ledger check below and uses the short reminder
// template instead of either of the other two. The identity step is
// unchanged: the SCIM endpoint is create-if-absent, so a resend simply
// finds the account the first invitation created.
//
// An integration user (IsIntegrationUser) never signs in and gets no
// email: both steps are recorded SKIPPED and nothing else happens. A step
// whose flag is off is likewise recorded SKIPPED. A step that fails records
// FAILED with the error text and returns the error, so eventbus.Consumer's
// usual retry/dead-letter path applies; a step that succeeds records
// SUCCEEDED. Recording itself is best-effort and never changes the
// handler's outcome — see recordOnboardingStep.
//
// Retry safety: EnsureExternalUser is idempotent upstream (a repeat is a
// 200), so re-running the identity step is harmless in itself — but it
// would answer existed=true for a user the previous attempt created, and
// the retry's email would then use the wrong wording. Dispatcher.
// identityExisted remembers the first successful answer per record for
// exactly that case; it's released on full success or record.NoMoreRetries
// (never IsFinalAttempt — see recordBaseKey for why a dead-lettered record
// keeps the same key on the DLQ topic). The whole attempt is guarded by a
// per-record claim() (see inflightKey below), so two Handle calls racing on
// the same record cannot both provision or both send: the loser returns an
// error and its retry runs alone, finding the winner's remembered answer.
// No claim/forget tracking is needed for the email itself: nothing after
// SendEmail can fail in a way that triggers a retry (step recording is
// best-effort), so a sent invitation is never re-sent by this handler's own
// retries.
func (d *Dispatcher) handleProjectContactInvited(ctx context.Context, record eventbus.Record, raw json.RawMessage) (retErr error) {
	var p events.ProjectContactInvitedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode project_contact.invited payload: %w", err)
	}
	// Never the invitee's email address — this repo's own "no recipient
	// emails in logs" convention; the membership id is enough to find the
	// row (and its email) on entity-service's ledger.
	logAttrs := []any{"membershipSfId", p.MembershipSfID, "contactSfId", p.ContactSfID, "projectKey", p.ProjectKey}

	if p.IsIntegrationUser {
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepIdentity, entity.OnboardingStepSkipped, nil)
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepSkipped, nil)
		slog.InfoContext(ctx, "dispatch: project_contact.invited for an integration user; identity and email skipped", logAttrs...)
		return nil
	}

	// One attempt at a time per record. Two Handle calls racing on the same
	// record (a consumer-group rebalance, or two of this process's
	// consumers) must not both run the identity or the email step: the
	// loser fails here without touching anything and its retry runs the
	// whole sequence alone, finding whatever the winner remembered. The
	// guard covers the entire attempt, not just the SCIM call — a call that
	// arrived after the memo was written but before the winner's email
	// went out would otherwise send a second invitation. Released whenever
	// this call returns; the memo below outlives it for sequential retries.
	inflightKey := recordBaseKey(record) + "/onboarding"
	if !d.claim(inflightKey) {
		return fmt.Errorf("dispatch: onboarding for membership %s is already in progress", p.MembershipSfID)
	}
	defer d.forget(inflightKey)

	identityKey := recordBaseKey(record) + "/identity"
	defer func() {
		// Drop the remembered identity answer once no retry can ever need
		// it again: the whole record succeeded, or nothing will redeliver
		// its content anywhere (NoMoreRetries — see its doc comment).
		if retErr == nil || record.NoMoreRetries {
			d.forgetIdentityExisted(identityKey)
		}
	}()

	// Step 1 — IDENTITY.
	var existed bool
	switch {
	case !d.onboarding.IdentityEnabled:
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepIdentity, entity.OnboardingStepSkipped, nil)
		slog.InfoContext(ctx, "dispatch: identity provisioning disabled (CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED != true); skipping", logAttrs...)
	default:
		if remembered, ok := d.rememberedIdentityExisted(identityKey); ok {
			// A previous attempt at this same record already provisioned
			// the identity (and recorded SUCCEEDED); this retry is here for
			// a later step. Reuse its answer rather than asking again — see
			// the doc comment above for why asking again gives the wrong
			// email wording.
			existed = remembered
			slog.InfoContext(ctx, "dispatch: identity already provisioned by an earlier attempt at this record; not repeating", append(logAttrs, "existed", existed)...)
			break
		}
		if d.onboarding.Identity == nil {
			err := fmt.Errorf("dispatch: identity provisioning enabled but no SCIM client configured")
			d.recordOnboardingStep(ctx, p, entity.OnboardingStepIdentity, entity.OnboardingStepFailed, err)
			return err
		}
		user, err := d.onboarding.Identity.EnsureExternalUser(ctx, p.Email, p.GivenName, p.FamilyName)
		if err != nil {
			err = fmt.Errorf("dispatch: provision identity for membership %s: %w", p.MembershipSfID, err)
			d.recordOnboardingStep(ctx, p, entity.OnboardingStepIdentity, entity.OnboardingStepFailed, err)
			return err
		}
		existed = user.Existed
		d.rememberIdentityExisted(identityKey, existed)
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepIdentity, entity.OnboardingStepSucceeded, nil)
		slog.InfoContext(ctx, "dispatch: identity provisioned", append(logAttrs, "asgardeoUserId", user.ID, "existed", existed)...)
	}

	// Step 2 — EMAIL.
	switch {
	case !d.onboarding.EmailEnabled:
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepSkipped, nil)
		slog.InfoContext(ctx, "dispatch: invitation email disabled (CSM_MIGRATION_ONBOARD_EMAIL_ENABLED != true); skipping", logAttrs...)
	case !d.emailSendingEnabled:
		// The service-wide killswitch silences this email the same way it
		// silences every other one here — recorded SKIPPED, not FAILED,
		// since retrying won't change an operator's decision.
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepSkipped, nil)
		slog.InfoContext(ctx, "dispatch: email sending disabled (EMAIL_SENDING_ENABLED=false); not sending invitation", logAttrs...)
	default:
		to := []string{p.Email}
		var intendedFor string
		if d.emailDebugMode {
			if len(d.emailDebugRecipients) == 0 {
				d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepSkipped, nil)
				slog.WarnContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true but EMAIL_DEBUG_RECIPIENTS is empty; not sending invitation", logAttrs...)
				break
			}
			// Same redirect every other email here gets in debug mode: a
			// real send, just to the configured test list instead of the
			// invitee — so a staging deployment can't invite a real
			// customer contact by accident.
			slog.InfoContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true; redirecting invitation to configured debug recipients", append(logAttrs, "debugRecipientCount", len(d.emailDebugRecipients))...)
			intendedFor = p.Email
			to = d.emailDebugRecipients
		}
		if d.onboarding.Email == nil {
			err := fmt.Errorf("dispatch: invitation email enabled but no email client configured")
			d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepFailed, err)
			return err
		}
		if !p.IsResend && d.onboarding.Steps == nil {
			// A resend never reaches the read below, so a missing recorder
			// costs it nothing but the (best-effort) record of the send;
			// failing a deliberate resend over it would be gratuitous.
			//
			// recordOnboardingStep tolerates a nil recorder -- a step
			// outcome nobody can write down is worth a warning, not a
			// failed record. The read below is not that: without it there
			// is no duplicate check at all, and sending anyway is the
			// second invitation this whole block exists to prevent. So it
			// is a configuration error, reported the same way as the two
			// nil checks above, and the record follows the normal
			// retry/DLQ path instead of panicking inside the consumer.
			err := fmt.Errorf("dispatch: invitation email enabled but no onboarding-step ledger configured; cannot check whether an invitation was already sent")
			d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepFailed, err)
			return err
		}

		// Last check before sending: has an invitation for this version of
		// the membership already gone out? (A re-invitation -- the
		// membership moved back into INVITED or RE-INVITED, say after a
		// deactivation -- is a newer version and must send again; see
		// invitationAlreadySent.) The other two guards cannot answer that. The
		// in-process claim above lives for one process, and the ingest's
		// duplicate check only recognises an unchanged Salesforce version,
		// so neither covers a redelivery after a restart, a replay from the
		// dead-letter topic, or the case this was written for -- the
		// customer portal onboarding a contact synchronously, with the
		// Salesforce event for the same contact reaching the ingest
		// afterwards as a new version.
		//
		// It is a read of shared state, not a lock, and the difference
		// matters: two replicas can both read "not sent" before either
		// sends, and a send whose SUCCEEDED write then fails leaves no
		// record for the next delivery to find. Both windows are narrow --
		// the first needs concurrent delivery of one record, which only
		// happens across a consumer-group rebalance, and the second needs
		// the ledger write and the offset commit to fail together -- and
		// the cost of losing either race is one repeated welcome e-mail.
		// Closing them properly needs an atomic reservation in
		// entity-service (claim the EMAIL step, and be told whether you
		// won); see the PR discussion. Until there is a reason to build
		// that, this catches every duplicate we can actually foresee.
		//
		// A failure to read the ledger is not a reason to send, and not a
		// reason to give up either, so it is returned and the record is
		// retried.
		//
		// A resend skips the check entirely. The guard exists to stop an
		// *accidental* second invitation -- every portal invitation also
		// writes the membership back to Salesforce, so the same contact
		// returns through the ingest as an event and would otherwise be
		// invited twice. An admin pressing "Resend invitation" is not
		// that: entity-service republishes this event with isResend set
		// precisely because the invitation should go out again. The EMAIL
		// step is still recorded either way, so the ledger's attemptCount
		// keeps showing how many invitations actually went out.
		if p.IsResend {
			slog.InfoContext(ctx, "dispatch: resend requested; not checking the invitation ledger", logAttrs...)
		} else if sent, err := d.invitationAlreadySent(ctx, p, logAttrs); err != nil {
			return fmt.Errorf("dispatch: check invitation already sent for membership %s: %w", p.MembershipSfID, err)
		} else if sent {
			break
		}

		data := notifications.ProjectContactInvitedEmailData{
			DisplayName: inviteeDisplayName(p.GivenName, p.FamilyName, p.Email),
			Email:       p.Email,
			ProjectName: displayProjectName(p.ProjectName, p.ProjectKey),
			ProjectKey:  p.ProjectKey,
			Roles:       p.Roles,
			PortalURL:   d.onboarding.PortalURL,
			IntendedFor: intendedFor,
		}
		// The "existing" wording only when the identity step actually ran
		// this record and said so. With identity disabled nothing here can
		// know whether an account exists, so the "new" template is sent
		// with AccountCreated=false: it then says neither "an account has
		// been created for you" nor "you already have one", only how to
		// sign in.
		var subject, body string
		switch {
		case p.IsResend:
			// A resend always uses the reminder wording, whatever the
			// identity step answered. By now the account exists (the first
			// invitation, or this record's own identity step, created it),
			// so the "existing" template would tell someone who may never
			// have opened the first email that they already have an
			// account -- and the "new" one would welcome them a second
			// time. The reminder claims neither.
			subject = "Reminder: " + invitationSubject(data.ProjectName)
			body = notifications.RenderProjectContactInvitedReminderEmail(data)
		case d.onboarding.IdentityEnabled && existed:
			subject = invitationSubject(data.ProjectName)
			body = notifications.RenderProjectContactInvitedExistingEmail(data)
		default:
			data.AccountCreated = d.onboarding.IdentityEnabled
			subject = invitationSubject(data.ProjectName)
			body = notifications.RenderProjectContactInvitedNewEmail(data)
		}
		if err := d.onboarding.Email.SendEmailFrom(ctx, d.onboarding.EmailFrom, to, nil, nil, d.onboarding.ReplyTo, subject, body, nil); err != nil {
			err = fmt.Errorf("dispatch: send invitation for membership %s: %w", p.MembershipSfID, err)
			d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepFailed, err)
			return err
		}
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepSucceeded, nil)
		slog.InfoContext(ctx, "dispatch: invitation email sent", append(logAttrs, "existingAccount", d.onboarding.IdentityEnabled && existed, "resend", p.IsResend)...)
	}

	return nil
}

// invitationSubject is the subject of the new and existing-account invitations.
func invitationSubject(projectName string) string {
	return "Invitation To Use WSO2 Support for " + projectName
}

// handleProjectContactRegistered sends the Welcome email after a contact's
// first sign-in. Same flags as the invitation; WELCOME_EMAIL guards duplicates.
func (d *Dispatcher) handleProjectContactRegistered(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.ProjectContactRegisteredPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode project_contact.registered payload: %w", err)
	}
	logAttrs := []any{"membershipSfId", p.MembershipSfID, "contactSfId", p.ContactSfID, "projectKey", p.ProjectKey}
	recordStep := func(status entity.OnboardingStepStatus, lastErr error) {
		d.writeOnboardingStep(ctx, entity.OnboardingStepRequest{
			MembershipSfID:  p.MembershipSfID,
			Step:            entity.OnboardingStepWelcomeEmail,
			Status:          status,
			EventType:       string(events.TypeProjectContactRegistered),
			EventModifiedOn: parseEventModifiedOn(p.EventModifiedOn),
			Email:           p.Email,
			ContactSfID:     p.ContactSfID,
		}, lastErr)
	}
	skip := func(msg string) error {
		// A SKIPPED write may replace SUCCEEDED for the same version, so a replay must not overwrite a sent Welcome.
		if d.onboarding.Steps != nil {
			sent, err := d.onboarding.Steps.SucceededStep(ctx, p.MembershipSfID, entity.OnboardingStepWelcomeEmail)
			if err != nil {
				return fmt.Errorf("dispatch: check welcome email already sent for membership %s: %w", p.MembershipSfID, err)
			}
			if sent != nil {
				slog.InfoContext(ctx, "dispatch: welcome email already recorded as sent; not recording a skip", logAttrs...)
				return nil
			}
		}
		recordStep(entity.OnboardingStepSkipped, nil)
		slog.InfoContext(ctx, msg, logAttrs...)
		return nil
	}
	fail := func(err error) error {
		recordStep(entity.OnboardingStepFailed, err)
		return err
	}

	switch {
	case p.IsIntegrationUser:
		return skip("dispatch: project_contact.registered for an integration user; welcome email skipped")
	case !d.onboarding.EmailEnabled:
		return skip("dispatch: welcome email disabled (CSM_MIGRATION_ONBOARD_EMAIL_ENABLED != true); skipping")
	case !d.emailSendingEnabled:
		return skip("dispatch: email sending disabled (EMAIL_SENDING_ENABLED=false); not sending welcome email")
	case d.emailDebugMode && len(d.emailDebugRecipients) == 0:
		return skip("dispatch: EMAIL_DEBUG_MODE=true but EMAIL_DEBUG_RECIPIENTS is empty; not sending welcome email")
	case d.onboarding.Email == nil:
		return fail(fmt.Errorf("dispatch: welcome email enabled but no email client configured"))
	case d.onboarding.Steps == nil:
		return fail(fmt.Errorf("dispatch: welcome email enabled but no onboarding-step ledger configured; cannot check whether it was already sent"))
	}
	to := []string{p.Email}
	var intendedFor string
	if d.emailDebugMode {
		slog.InfoContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true; redirecting welcome email to configured debug recipients", append(logAttrs, "debugRecipientCount", len(d.emailDebugRecipients))...)
		intendedFor = p.Email
		to = d.emailDebugRecipients
	}

	key := recordBaseKey(record) + "/welcome"
	if !d.claim(key) {
		return fmt.Errorf("dispatch: welcome email for membership %s is already in progress", p.MembershipSfID)
	}
	defer d.forget(key)

	// One Welcome per membership: any SUCCEEDED row means it already went out.
	sent, err := d.onboarding.Steps.SucceededStep(ctx, p.MembershipSfID, entity.OnboardingStepWelcomeEmail)
	if err != nil {
		return fmt.Errorf("dispatch: check welcome email already sent for membership %s: %w", p.MembershipSfID, err)
	}
	if sent != nil {
		slog.InfoContext(ctx, "dispatch: welcome email already recorded as sent; not sending again", logAttrs...)
		return nil
	}

	projectName := displayProjectName(p.ProjectName, p.ProjectKey)
	body := notifications.RenderProjectContactRegisteredEmail(notifications.ProjectContactRegisteredEmailData{
		DisplayName: inviteeDisplayName(p.GivenName, p.FamilyName, p.Email),
		ProjectName: projectName,
		ProjectKey:  p.ProjectKey,
		PortalURL:   d.onboarding.PortalURL,
		IntendedFor: intendedFor,
	})
	subject := "Welcome to WSO2 Support for " + projectName
	if err := d.onboarding.Email.SendEmailFrom(ctx, d.onboarding.EmailFrom, to, nil, nil, d.onboarding.ReplyTo, subject, body, nil); err != nil {
		return fail(fmt.Errorf("dispatch: send welcome email for membership %s: %w", p.MembershipSfID, err))
	}
	recordStep(entity.OnboardingStepSucceeded, nil)
	slog.InfoContext(ctx, "dispatch: welcome email sent", logAttrs...)
	return nil
}

// invitationAlreadySent reports whether the ledger already records an
// invitation sent for this version of the membership, logging why when it
// does. It is the durable duplicate-invitation guard of
// handleProjectContactInvited.
//
// The ledger keeps one EMAIL row per membership, so "is there a SUCCEEDED
// EMAIL row" alone cannot tell a duplicate from a re-invitation: the first
// invitation would block every later one forever. What tells them apart is
// the membership version. Salesforce emits several UPDATED events per save
// and events are redelivered, but all of those carry the same (or, for a
// delayed delivery, an older) LastModifiedDate; a genuine re-invitation is a
// new save and carries a newer one. So the invitation counts as already
// sent only when the SUCCEEDED row's eventModifiedOn is not before the
// event's -- the same "not older" rule entity-service's step upsert applies.
//
// When either timestamp is missing or unparseable the versions cannot be
// compared, and this falls back to the conservative answer: any SUCCEEDED
// row means sent. A lost re-invitation can be recovered with a resend; a
// duplicate e-mail cannot be taken back.
func (d *Dispatcher) invitationAlreadySent(ctx context.Context, p events.ProjectContactInvitedPayload, logAttrs []any) (bool, error) {
	step, err := d.onboarding.Steps.SucceededEmailStep(ctx, p.MembershipSfID)
	if err != nil {
		return false, err
	}
	if step == nil {
		return false, nil
	}
	recordedOn, recordedOK := parseLedgerTime(step.EventModifiedOn)
	eventOn, eventOK := parseLedgerTime(p.EventModifiedOn)
	if !recordedOK || !eventOK {
		slog.WarnContext(ctx, "dispatch: cannot compare invitation versions (missing or unparseable eventModifiedOn); an invitation is already recorded as sent, so not sending again",
			append(logAttrs, "eventModifiedOn", p.EventModifiedOn, "recordedEventModifiedOn", step.EventModifiedOn)...)
		return true, nil
	}
	// Postgres keeps microseconds, so a timestamp with finer precision (only
	// the processing-time fallback has one) comes back rounded; compare at
	// the precision the ledger can actually hold, or a redelivery of that
	// same event could look newer than its own record.
	eventOn = eventOn.Truncate(time.Microsecond)
	if recordedOn.Before(eventOn) {
		slog.InfoContext(ctx, "dispatch: invitation recorded as sent for an older membership version; this is a re-invitation, sending",
			append(logAttrs, "eventModifiedOn", eventOn, "recordedEventModifiedOn", recordedOn)...)
		return false, nil
	}
	slog.InfoContext(ctx, "dispatch: invitation already recorded as sent for this membership version; not sending again",
		append(logAttrs, "eventModifiedOn", eventOn, "recordedEventModifiedOn", recordedOn)...)
	return true, nil
}

// parseLedgerTime parses an RFC 3339 eventModifiedOn. An empty, malformed or
// zero value (entity-service marshals an unset time.Time as
// 0001-01-01T00:00:00Z) is reported as absent.
func parseLedgerTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil || ts.IsZero() {
		return time.Time{}, false
	}
	return ts.UTC(), true
}

// recordOnboardingStep writes one step's outcome to entity-service's
// onboarding-step ledger (PUT /onboarding-steps/{membershipSfId}/{step}),
// best-effort: a failure to record is logged at ERROR and otherwise
// ignored. It is synchronous on purpose — the volume is a handful of
// invitations a day and the two writes per record keep IDENTITY recorded
// before EMAIL is attempted — but bounded by recordOnboardingStepTimeout
// so a slow ledger cannot hold the record for long. It must never mask the primary outcome — a step that genuinely
// succeeded must not turn into a retried (and, for email, re-sent) record
// because the ledger was briefly unreachable, and a step that failed must
// return its own error, not the ledger's. lastErr, when non-nil, becomes
// the row's lastError (entity-service keeps it only for a FAILED status).
//
// EventModifiedOn is the payload's eventModifiedOn — the Salesforce
// LastModifiedDate of the membership version this event describes. entity-
// service only applies a step write whose eventModifiedOn is not older than
// the row's stored one, so a delayed delivery of an older invitation cannot
// overwrite the outcome recorded for a newer one, while retries of the same
// version (equal timestamps) still land. Only when the payload carries no
// timestamp (entity-service could not parse the Salesforce date) does this
// fall back to the processing time.
func (d *Dispatcher) recordOnboardingStep(ctx context.Context, p events.ProjectContactInvitedPayload, step entity.OnboardingStep, status entity.OnboardingStepStatus, lastErr error) {
	d.writeOnboardingStep(ctx, entity.OnboardingStepRequest{
		MembershipSfID:  p.MembershipSfID,
		Step:            step,
		Status:          status,
		EventType:       string(events.TypeProjectContactInvited),
		EventModifiedOn: parseEventModifiedOn(p.EventModifiedOn),
		Email:           p.Email,
		ContactSfID:     p.ContactSfID,
	}, lastErr)
}

// writeOnboardingStep is recordOnboardingStep's event-agnostic core.
func (d *Dispatcher) writeOnboardingStep(ctx context.Context, req entity.OnboardingStepRequest, lastErr error) {
	if d.onboarding.Steps == nil {
		slog.WarnContext(ctx, "dispatch: no onboarding-step recorder configured; step outcome not recorded",
			"membershipSfId", req.MembershipSfID, "step", req.Step, "status", req.Status)
		return
	}
	if lastErr != nil {
		req.LastError = lastErr.Error()
	}
	// Detached from the handler's context on purpose. A shutdown or a
	// consumer-group rebalance cancels ctx, and it would cancel this write
	// too -- losing the EMAIL=SUCCEEDED row for an e-mail that has already
	// gone out. The same shutdown is likely to lose the offset commit, so
	// the record comes back on restart, finds no record of the send, and
	// invites the person twice. One cause, both failures, which is exactly
	// the coincidence the ledger check relies on being rare. The timeout
	// still bounds it, so a hung ledger cannot hold the handler.
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordOnboardingStepTimeout)
	defer cancel()
	if err := d.onboarding.Steps.RecordOnboardingStep(recordCtx, req); err != nil {
		slog.ErrorContext(ctx, "dispatch: failed to record onboarding step; continuing",
			"membershipSfId", req.MembershipSfID, "step", req.Step, "status", req.Status, "err", err)
		return
	}
	slog.InfoContext(ctx, "dispatch: onboarding step recorded", "membershipSfId", req.MembershipSfID, "step", req.Step, "status", req.Status)
}

// recordOnboardingStepTimeout bounds one best-effort ledger write.
const recordOnboardingStepTimeout = 5 * time.Second

// parseEventModifiedOn returns the payload's Salesforce timestamp, or
// the processing time when the payload has none (events.Validate has already
// rejected a malformed one).
func parseEventModifiedOn(eventModifiedOn string) time.Time {
	if eventModifiedOn != "" {
		if ts, err := time.Parse(time.RFC3339Nano, eventModifiedOn); err == nil {
			return ts.UTC()
		}
	}
	return time.Now().UTC()
}

// rememberIdentityExisted/rememberedIdentityExisted/forgetIdentityExisted
// are Dispatcher.identityExisted's accessors — see that field's doc comment.
func (d *Dispatcher) rememberIdentityExisted(key string, existed bool) {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	d.identityExisted[key] = existed
}

func (d *Dispatcher) rememberedIdentityExisted(key string) (existed, ok bool) {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	existed, ok = d.identityExisted[key]
	return existed, ok
}

func (d *Dispatcher) forgetIdentityExisted(key string) {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	delete(d.identityExisted, key)
}

// inviteeDisplayName is how the invitation addresses its reader: the
// Salesforce given and family names joined, or — since Salesforce doesn't
// require a first name and test data frequently has neither — the email's
// local part (the part before "@"), which is at least recognisably theirs.
func inviteeDisplayName(givenName, familyName, email string) string {
	if name := strings.TrimSpace(strings.TrimSpace(givenName) + " " + strings.TrimSpace(familyName)); name != "" {
		return name
	}
	if local, _, ok := strings.Cut(email, "@"); ok && local != "" {
		return local
	}
	return email
}

// displayProjectName is the project as the invitation names it: the
// Salesforce project name, falling back to its key, then to a generic
// phrase, so neither the subject nor the body ever has an empty slot.
func displayProjectName(projectName, projectKey string) string {
	if projectName != "" {
		return projectName
	}
	if projectKey != "" {
		return projectKey
	}
	return "your project"
}
