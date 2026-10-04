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

package dispatch

// The case.* handlers and the per-recipient-link email grouping they share.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

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
			// staff, who recognize the internal reference, not the backing
			// data source's own case number.
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
