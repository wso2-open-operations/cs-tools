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

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
)

// emailSender abstracts notifications.EmailClient for testability.
type emailSender interface {
	SendEmail(ctx context.Context, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error
}

// googleChatSender abstracts notifications.GoogleChatClient for testability.
type googleChatSender interface {
	SendIncidentAlert(ctx context.Context, product, title, shortDescription, portalURL string) error
	SendCaseCreatedAlert(ctx context.Context, product, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error
	SendCaseAcknowledgedAlert(ctx context.Context, product, severityLabel, severityColor, caseNumber, wso2CaseID, caseLink, acknowledgerName string) error
	SendSeverityChangedAlert(ctx context.Context, product, oldSeverityLabel, oldSeverityColor, newSeverityLabel, newSeverityColor, caseNumber, wso2CaseID, title, team, caseLink string) error
}

// callSender abstracts notifications.TwilioClient's MakeCall for testability.
type callSender interface {
	MakeCall(ctx context.Context, to, message string) error
}

// linkResolver abstracts recipientlinks.Resolver for testability.
type linkResolver interface {
	ResolveLinks(ctx context.Context, emails []string, projectID, caseID string) ([]recipientlinks.RecipientLink, error)
	CSMLink(caseID string) string
	IncidentLink(incidentID string) string
}

// Dispatcher turns a published events.Envelope into an actual notification
// send.
//
// Every case.* payload carries its own Recipients list (who to email) — this
// service resolves which portal link each recipient gets (via links, see
// groupByLink), not who to notify: there's no entity-service lookup here for
// watchers/assignee/reporter, so the caller (e.g. csm-portal-backend)
// supplies the audience directly at publish time. incident.created carries
// no recipients field; its Google Chat/call reactions already have their
// own real destination, a space and a phone number, in the event payload
// itself, and don't go through links at all.
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
	// handleIncidentCreated's own doc comment. Doesn't affect the Google
	// Chat alert.
	callSendingEnabled bool

	// defaultChatProduct/defaultOnCallNumber are handleCaseCreated's and
	// handleIncidentCreated's fallback values for their payload's own
	// Product/CallTo when a publisher omits them — see handleIncidentCreated's
	// doc comment for why a publisher (e.g. entity-service) might not know
	// either value itself. defaultChatProduct applies to both event types'
	// Google Chat alert (case.created has no call reaction, hence no
	// case.created-specific default for defaultOnCallNumber).
	defaultChatProduct  string
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
// Dispatcher.defaultChatProduct/defaultOnCallNumber's doc comment for the
// Google Chat/call fallback values.
func NewDispatcher(email emailSender, googleChat googleChatSender, call callSender, links linkResolver, emailSendingEnabled bool, emailDebugMode bool, emailDebugRecipients []string, callSendingEnabled bool, defaultChatProduct, defaultOnCallNumber string) *Dispatcher {
	return &Dispatcher{
		email:                email,
		googleChat:           googleChat,
		call:                 call,
		links:                links,
		emailSendingEnabled:  emailSendingEnabled,
		emailDebugMode:       emailDebugMode,
		emailDebugRecipients: emailDebugRecipients,
		callSendingEnabled:   callSendingEnabled,
		defaultChatProduct:   defaultChatProduct,
		defaultOnCallNumber:  defaultOnCallNumber,
		done:                 make(map[string]bool),
		records:              make(map[string]*recordState),
	}
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
		return d.handleIncidentCreated(ctx, record, env.EntityID, env.Payload)
	case events.TypeSLAClockRegister, events.TypeSLATierReached:
		// internal/slaengine's own consumer group (a different group ID, so
		// it gets its own full copy of this same topic) is what reacts to
		// these — nothing for the notification dispatcher to do. Returning
		// nil (not an error) is required here: erroring would burn this
		// consumer's retries and dead-letter an event that was never broken,
		// just not this consumer's concern.
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

// handleCaseCreated has two independent reactions, like handleIncidentCreated
// below: the case-created email (per resolved recipient link) and a Google
// Chat alert to the shared internal Chat space, via the same
// GoogleChatClient.SendIncidentAlert incident.created uses — its doc comment
// already covers "a newly created incident/case" for exactly this reuse. The
// Chat alert always targets the CSM portal's case link (links.CSMLink), not
// a per-recipient link, since there's no per-recipient audience for a Chat
// post the way there is for email. Product falls back to
// Dispatcher.defaultChatProduct when the payload omits it, the same as
// handleIncidentCreated's Product fallback — see that function's doc
// comment. If the resolved product is still empty (payload and
// DEFAULT_CHAT_PRODUCT both unset), the Chat alert is skipped (logged) the
// same way handleIncidentCreated skips its own Chat alert in that case,
// rather than calling SendIncidentAlert with an empty product — that would
// return a real "no space configured" error and, without the idempotency
// tracking described below, retry every already-succeeded email group
// alongside the Chat attempt every time.
//
// Every reaction here — each email group and the Chat alert — has
// per-record idempotency tracking, the same mechanism handleIncidentCreated
// uses: a real-world failure mode this was missing until it actually
// happened — a persistently-failing step (e.g. a misconfigured email OAuth2
// client) means every one of eventbus.Consumer's 3 retries, and then the
// DLQ consumer's own 3 retries, re-runs this whole function, so an
// unguarded already-succeeded channel would repost/resend up to 6 times for
// one event before the failing one is ever fixed. Email groups are tracked
// inside sendPerGroup itself (see its own doc comment); chatKey is tracked
// here directly, mirroring handleIncidentCreated's shape. Both kinds are
// forgotten together, once the whole call succeeds (len(errs) == 0, so
// Handle is about to return nil — no more retries coming) or
// record.NoMoreRetries is true (no further retry coming at all, on this
// topic or the dead-letter one — see its doc comment) — never on an
// individual channel's own success alone, which would release it while
// other channels in this same call are still failing and
// eventbus.Consumer keeps retrying, immediately re-arming that channel to
// resend on the very next attempt. Releasing is also ownership-gated for
// both kinds — see forgetEmailGroups' doc comment for the live duplicate-
// email bug that closed.
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
		_, emailErr = d.sendPerGroup(ctx, baseKey, groups, groupUserIDs, subject, func(caseLink string) string {
			return notifications.RenderCaseCreatedEmail(notifications.CaseCreatedEmailData{
				ReporterName:              p.ReporterName,
				ProjectName:               p.ProjectName,
				CaseNumber:                caseRef,
				CaseTitle:                 p.CaseTitle,
				CaseType:                  p.CaseType,
				Priority:                  p.Priority,
				Product:                   p.Product,
				CreatedAt:                 p.CreatedAt,
				Description:               p.Description,
				IncidentImpactDescription: p.IncidentImpactDescription,
				CaseLink:                  caseLink,
				CommentLink:               commentLinkFor(caseLink, ""),
			})
		})
		if emailErr != nil {
			errs = append(errs, emailErr)
		}
	}

	chatOwned := d.claim(chatKey)
	if chatOwned {
		product := p.Product
		if product == "" {
			product = d.defaultChatProduct
		}
		if product == "" {
			slog.WarnContext(ctx, "dispatch: no product for case.created (payload and DEFAULT_CHAT_PRODUCT both empty); skipping Google Chat alert")
		} else {
			severityLabel, severityColor := severityLabelAndColor(p.Priority)
			caseLink := d.links.CSMLink(p.CaseID)
			title := truncateTitle(p.CaseTitle, maxChatTitleLength)
			if chatErr := d.googleChat.SendCaseCreatedAlert(ctx, product, severityLabel, severityColor, displayCaseRef(p.CaseNumber, p.CaseID), p.WSO2CaseID, p.Product, title, p.Team, caseLink); chatErr != nil {
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
	owned, sendErr := d.sendPerGroup(ctx, baseKey, groups, groupUserIDs, subject, func(caseLink string) string {
		if p.IsInternalNote {
			// See events.CommentAddedPayload.IsInternalNote's own doc
			// comment: a distinct layout, and WSO2CaseID (not CaseNumber)
			// as the case reference — this audience is always wso2.com
			// staff, who recognize the internal reference, not ServiceNow's
			// own case number.
			return notifications.RenderInternalNoteEmail(p.Name, displayInternalRef(p.WSO2CaseID, p.CaseID), p.CaseTitle, p.CaseComment, commentLinkFor(caseLink, p.CommentID), caseLink)
		}
		return notifications.RenderCommentAddedEmail(p.Name, displayCaseRef(p.CaseNumber, p.CaseID), p.CaseTitle, p.CaseComment, commentLinkFor(caseLink, p.CommentID), caseLink)
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
	owned, sendErr := d.sendPerGroup(ctx, baseKey, groups, groupUserIDs, subject, func(caseLink string) string {
		return notifications.RenderStatusChangedEmail(caseRef, p.NewStatus, caseLink, commentLinkFor(caseLink, ""))
	})
	if record.NoMoreRetries {
		d.forgetEmailGroups(baseKey, slices.Collect(maps.Keys(groups)))
	} else if sendErr == nil {
		d.forgetEmailGroups(baseKey, owned)
	}
	return sendErr
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
	owned, sendErr := d.sendPerGroup(ctx, baseKey, groups, groupUserIDs, subject, func(caseLink string) string {
		return notifications.RenderCaseAssignedEmail(p.AssigneeName, p.AssigneeEmail, caseRef, caseLink, commentLinkFor(caseLink, ""))
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
// email/Recipients concept here at all. With only one channel, there's no
// cross-channel release race to guard against the way beginRecord/endRecord
// does for handleCaseCreated/handleIncidentCreated — this call's own
// chatOwned already fully determines whether it's safe to release: true
// means this call either just sent successfully or found nothing to do
// (no configured product), either way a real, complete outcome; false
// means it lost the claim race entirely and touched nothing, so it must
// never release a key a different, still in-flight call might rely on.
func (d *Dispatcher) handleCaseAcknowledged(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.CaseAcknowledgedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.acknowledged payload: %w", err)
	}

	chatKey := recordBaseKey(record) + "/chat"
	chatOwned := d.claim(chatKey)
	var chatErr error
	if chatOwned {
		product := p.Product
		if product == "" {
			product = d.defaultChatProduct
		}
		if product == "" {
			slog.WarnContext(ctx, "dispatch: no product for case.acknowledged (payload and DEFAULT_CHAT_PRODUCT both empty); skipping Google Chat alert")
		} else {
			severityLabel, severityColor := severityLabelAndColor(p.Severity)
			caseLink := d.links.CSMLink(p.CaseID)
			chatErr = d.googleChat.SendCaseAcknowledgedAlert(ctx, product, severityLabel, severityColor, displayCaseRef(p.CaseNumber, p.CaseID), p.WSO2CaseID, caseLink, p.AcknowledgerName)
			if chatErr != nil {
				d.forget(chatKey)
				chatOwned = false
			}
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
// RenderSeverityChangedEmail) and a Google Chat alert to the same space as
// the case's own case.created/case.acknowledged alerts
// (SendSeverityChangedAlert). Unlike handleCaseAcknowledged (Chat-only, one
// channel, no cross-channel release race — see its own doc comment), this
// needs the same beginRecord/endRecord refcounting handleCaseCreated uses,
// for the exact same reason: two channels means a losing call for one
// channel must not release the other while a different, still in-flight
// call genuinely owns it. Product falls back to Dispatcher.defaultChatProduct
// when the payload omits it, and the Chat alert is skipped (logged) rather
// than erroring when the resolved product is still empty — same reasoning
// as handleCaseCreated's own Chat block.
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

	groups, groupUserIDs, err := d.groupByLink(ctx, p.Recipients, p.ProjectID, p.CaseID)
	if err != nil {
		errs = append(errs, err)
	} else {
		title := p.CaseTitle
		if title == "" {
			// A publisher that hasn't sent CaseTitle still gets a meaningful
			// subject rather than a blank title slot — same fallback
			// handleStatusChanged/handleCaseAssigned use.
			title = "Severity changed to " + newLabel
		}
		subject := subjectLine(p.WSO2CaseID, p.CaseNumber, p.CaseID, title)
		var emailErr error
		_, emailErr = d.sendPerGroup(ctx, baseKey, groups, groupUserIDs, subject, func(caseLink string) string {
			return notifications.RenderSeverityChangedEmail(caseRef, oldLabel, newLabel, caseLink, commentLinkFor(caseLink, ""))
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
		product := p.Product
		if product == "" {
			product = d.defaultChatProduct
		}
		if product == "" {
			slog.WarnContext(ctx, "dispatch: no product for case.severity_changed (payload and DEFAULT_CHAT_PRODUCT both empty); skipping Google Chat alert")
		} else {
			caseLink := d.links.CSMLink(p.CaseID)
			title := truncateTitle(p.CaseTitle, maxChatTitleLength)
			if chatErr := d.googleChat.SendSeverityChangedAlert(ctx, product, oldLabel, oldColor, newLabel, newColor, caseRef, p.WSO2CaseID, title, p.Team, caseLink); chatErr != nil {
				errs = append(errs, chatErr)
				d.forget(chatKey)
			}
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
// of which is better than not calling it at all.
func (d *Dispatcher) sendPerGroup(ctx context.Context, baseKey string, groups, groupUserIDs map[string][]string, subject string, render func(caseLink string) string) ([]string, error) {
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
		if d.emailDebugMode {
			if len(d.emailDebugRecipients) == 0 {
				slog.WarnContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true but EMAIL_DEBUG_RECIPIENTS is empty; not sending",
					"subject", subject)
				owned = append(owned, caseLink)
				continue
			}
			slog.InfoContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true; redirecting email to configured debug recipients",
				"subject", subject, "realRecipientCount", len(to), "debugRecipientCount", len(d.emailDebugRecipients))
			to = d.emailDebugRecipients
		}
		if err := d.email.SendEmail(ctx, to, nil, nil, nil, subject, render(caseLink), nil); err != nil {
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

// handleIncidentCreated has two independent reactions, unlike every other
// event type here except handleCaseCreated: a Google Chat alert and a voice
// call. Both are attempted even if one fails, and their errors are combined
// — a Chat outage shouldn't suppress the call, or vice versa.
//
// Product/CallTo fall back to Dispatcher's configured defaults
// (defaultChatProduct/defaultOnCallNumber) when the payload's own value
// is empty — a publisher that has no way to determine either (e.g.
// entity-service, which knows nothing about Chat-space routing or on-call
// rotations) can omit them entirely; events.Validate allows this. A
// publisher that does know the right values per incident can still supply
// them and takes precedence over the defaults. If a resolved value is still
// empty (payload and default both unset), that one channel is skipped
// (logged, treated as succeeded) instead of calling SendIncidentAlert/
// MakeCall with an empty product/destination — both would just return a
// real error (an unmapped product, an empty call destination), which would
// otherwise burn all of eventbus.Consumer's retries and dead-letter an
// incident whose only problem is a missing operator default, not a
// transient failure.
//
// callSendingEnabled gates only the MakeCall step (CALL_SENDING_ENABLED):
// when false, this logs what would have been called instead of calling, and
// still marks the call "done" so a disabled call doesn't retry forever —
// the same log-only shape sendPerGroup's email sending used to have before
// EMAIL_DEBUG_MODE replaced it with a redirect-to-a-test-list behavior (see
// sendPerGroup's doc comment); calls have no equivalent debug-recipient
// concept, so this keeps the simpler disable-entirely shape. The Google
// Chat alert is unaffected either way.
//
// This is also the one handler that needs its own idempotency tracking:
// eventbus.Consumer retries this whole function on any error, and without
// tracking which side already succeeded, a Twilio failure alone would cause
// the (already-successful) Chat alert to be resent on every retry too —
// paging on-call once but posting 3 duplicate Chat cards, or vice versa.
// claim/forget key on this specific record (recordBaseKey, unique per event
// content, not per Kafka delivery — see its doc comment) plus which
// channel, so a retry only re-attempts the channel that's still actually
// failing. Both keys are released once either
// both channels have succeeded, or record.NoMoreRetries is true — the
// latter matters because a channel that never succeeds (e.g. Twilio stays
// down for all 3 attempts, on the main topic AND the DLQ topic) would
// otherwise never hit the "both succeeded" branch, and its key would sit in
// d.done forever: NoMoreRetries tells us there is truly no future retry
// left to protect against — on this topic or the dead-letter one — so it's
// safe to stop tracking. This is deliberately record.NoMoreRetries and not
// record.IsFinalAttempt: the main topic's own final attempt still has a DLQ
// tier of retries coming for the identical content (same recordBaseKey),
// so forgetting there would let an already-succeeded channel be reclaimed
// and resent once the dead-lettered record's first DLQ attempt arrives.
func (d *Dispatcher) handleIncidentCreated(ctx context.Context, record eventbus.Record, entityID string, raw json.RawMessage) error {
	var p events.IncidentCreatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode incident.created payload: %w", err)
	}

	base := recordBaseKey(record)
	chatKey, callKey := base+"/chat", base+"/call"
	endRecord := d.beginRecord(base)

	product := p.Product
	if product == "" {
		product = d.defaultChatProduct
	}
	callTo := p.CallTo
	if callTo == "" {
		callTo = d.defaultOnCallNumber
	}

	var chatErr error
	chatOwned := d.claim(chatKey)
	if chatOwned {
		if product == "" {
			slog.WarnContext(ctx, "dispatch: no product for incident.created (payload and DEFAULT_CHAT_PRODUCT both empty); skipping Google Chat alert")
		} else {
			chatErr = d.googleChat.SendIncidentAlert(ctx, product, p.Title, p.ShortDescription, d.links.IncidentLink(entityID))
			if chatErr != nil {
				d.forget(chatKey)
				chatOwned = false
			}
		}
	}

	var callErr error
	callOwned := d.claim(callKey)
	if callOwned {
		switch {
		case !d.callSendingEnabled:
			slog.InfoContext(ctx, "dispatch: call sending disabled (CALL_SENDING_ENABLED=false); not calling", "to", maskPhone(callTo))
		case callTo == "":
			slog.WarnContext(ctx, "dispatch: no callTo for incident.created (payload and INCIDENT_DEFAULT_CALL_TO both empty); skipping call")
		default:
			message := fmt.Sprintf("New incident: %s. %s", p.Title, p.ShortDescription)
			callErr = d.call.MakeCall(ctx, callTo, message)
			if callErr != nil {
				d.forget(callKey)
				callOwned = false
			}
		}
	}

	// record.NoMoreRetries always forgets both, regardless of ownership or
	// concurrent siblings — there is truly no future retry left for this
	// event's content either way (see NoMoreRetries' own doc comment for
	// why this must not be record.IsFinalAttempt), so nothing forgets these
	// keys later if we don't do it now.
	//
	// Otherwise, release only once endRecord confirms it's safe to — this
	// is the last call still in flight for this record, and neither it nor
	// any sibling that ran concurrently with it ever hit an error. An
	// earlier version of this gated on chatOwned && callOwned instead
	// (requiring this same call to have won both claims) — safer than
	// gating on chatErr/callErr being nil alone (a call that loses a claim
	// never touches that channel, so its own error variable stays nil,
	// indistinguishable from "I actually succeeded"), but still had a real
	// gap: two concurrent calls can legitimately split ownership (one wins
	// chat, the other wins call), in which case *neither* call ever
	// satisfies "I own both," even though both channels genuinely
	// succeeded — that combination would never release until
	// NoMoreRetries, for every future retry of a record that has already
	// fully succeeded. beginRecord/endRecord (see its own doc comment)
	// tracks completion across every concurrent call for this record
	// directly, instead of inferring it from what any single call happened
	// to own.
	// endRecord must run exactly once per call — it decrements beginRecord's
	// refcount — so it's called unconditionally here rather than only
	// inside the branch that uses its result.
	safeToRelease := endRecord(chatErr != nil || callErr != nil)
	if record.NoMoreRetries || safeToRelease {
		d.forget(chatKey)
		d.forget(callKey)
	}
	return errors.Join(chatErr, callErr)
}
