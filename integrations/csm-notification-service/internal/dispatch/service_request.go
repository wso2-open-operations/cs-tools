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

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// srCustomerCommentTag is the SR tag that opts an SR into the customer-comment
// Chat alert. Matched trimmed and case-insensitively: tags are stored as
// typed (see events.SRCommentAddedPayload.Tags).
const srCustomerCommentTag = "devops-sm"

// handleSRCreated posts ServiceNow's new-SR card to the SR's SRE team space.
//
// All three sr.* handlers route the same way: the payload's sreTeamName is
// the Chat audience, a GOOGLE_CHAT_SPACES key like any CRE team name. An SR
// with no SRE team, or a team with no configured space, is a no-op (logged
// at info) rather than an error -- no retry can conjure a space, and an
// unconfigured team must not dead-letter every SR it owns. There is no
// Incident Monitor fallback: ServiceNow's flow posted only to the team's own
// space.
//
// Chat is the only side effect, so these use handleCaseAcknowledged's
// single-channel shape (sendSRChat), not handleCaseCreated's multi-channel
// one: a failed post returns the error so eventbus.Consumer retries the
// record (and dead-letters it after exhausting retries), since nothing else
// in the record has succeeded that a retry could repeat.
func (d *Dispatcher) handleSRCreated(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.SRCreatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode sr.created payload: %w", err)
	}
	audience, ok := d.srAudience(ctx, events.TypeSRCreated, p.SRRef)
	if !ok {
		return nil
	}
	return d.sendSRChat(ctx, record, func() error {
		return d.googleChat.SendSRCreatedAlert(ctx, audience, notifications.SRCreatedAlert{
			CaseID:              p.CaseID,
			Number:              p.Number,
			WSO2CaseID:          p.WSO2CaseID,
			Subject:             p.Subject,
			AssignmentGroupName: p.AssignmentGroupName,
			State:               p.State,
			Description:         p.Description,
			CaseLink:            d.links.ServiceRequestLink(p.CaseID),
		})
	})
}

// handleSRAcknowledged posts ServiceNow's acknowledgement card, threaded
// under the SR's created card. Routed and tracked exactly like
// handleSRCreated.
func (d *Dispatcher) handleSRAcknowledged(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.SRAcknowledgedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode sr.acknowledged payload: %w", err)
	}
	audience, ok := d.srAudience(ctx, events.TypeSRAcknowledged, p.SRRef)
	if !ok {
		return nil
	}
	return d.sendSRChat(ctx, record, func() error {
		return d.googleChat.SendSRAcknowledgedAlert(ctx, audience, notifications.SRAcknowledgedAlert{
			CaseID:              p.CaseID,
			Number:              p.Number,
			WSO2CaseID:          p.WSO2CaseID,
			AssignmentGroupName: p.AssignmentGroupName,
			SRETeamName:         p.SRETeamName,
			CaseLink:            d.links.ServiceRequestLink(p.CaseID),
		})
	})
}

// handleSRCommentAdded alerts the SRE team when a customer comments on a
// devops-sm SR -- new behaviour, ServiceNow never had it. Fires only for a
// customer-visible comment (not a work note) on an SR tagged devops-sm whose
// author IsCustomer classifies as a customer, the same author check
// checkFrustration uses. Anything else is skipped silently.
//
// The cheap payload checks run before the space check and the role lookup,
// so the vast majority of SR comments cost nothing. Unlike checkFrustration
// (best-effort, because the email there is the handler's real job), a failed
// author lookup returns its error: this alert is the only thing this handler
// does, and swallowing the failure would lose it for good.
func (d *Dispatcher) handleSRCommentAdded(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.SRCommentAddedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode sr.comment_added payload: %w", err)
	}
	if p.CommentType != events.SRCommentTypeComment || !hasSRTag(p.Tags, srCustomerCommentTag) {
		slog.DebugContext(ctx, "dispatch: sr.comment_added not a comment on a devops-sm SR, skipping",
			"caseId", p.CaseID, "commentId", p.CommentID, "commentType", string(p.CommentType))
		return nil
	}
	audience, ok := d.srAudience(ctx, events.TypeSRCommentAdded, p.SRRef)
	if !ok {
		return nil
	}
	isCustomer, err := d.links.IsCustomer(ctx, p.AuthorEmail)
	if err != nil {
		return fmt.Errorf("dispatch: classify sr.comment_added author for %s: %w", p.CaseID, err)
	}
	if !isCustomer {
		slog.DebugContext(ctx, "dispatch: sr.comment_added author is not a customer, skipping",
			"caseId", p.CaseID, "commentId", p.CommentID)
		return nil
	}
	return d.sendSRChat(ctx, record, func() error {
		return d.googleChat.SendSRCustomerCommentAlert(ctx, audience, notifications.SRCustomerCommentAlert{
			CaseID:      p.CaseID,
			Number:      p.Number,
			WSO2CaseID:  p.WSO2CaseID,
			Subject:     truncateTitle(p.Subject, maxChatTitleLength),
			AuthorName:  p.AuthorName,
			AuthorEmail: p.AuthorEmail,
			Content:     p.Content,
			CommentLink: commentLinkFor(d.links.ServiceRequestLink(p.CaseID), p.CommentID),
		})
	})
}

// srAudience resolves an SR event's Chat audience -- its SRE team's name --
// or reports that there is nowhere to post, logging why (ids only).
func (d *Dispatcher) srAudience(ctx context.Context, t events.Type, ref events.SRRef) (string, bool) {
	team := strings.TrimSpace(ref.SRETeamName)
	if team == "" {
		slog.InfoContext(ctx, "dispatch: SR has no SRE team, no chat alert",
			"type", string(t), "caseId", ref.CaseID, "number", ref.Number)
		return "", false
	}
	if !d.googleChat.HasAudienceSpace(team) {
		slog.InfoContext(ctx, "dispatch: SRE team has no configured chat space, no chat alert",
			"type", string(t), "caseId", ref.CaseID, "number", ref.Number, "sreTeamId", ref.SRETeamID)
		return "", false
	}
	return team, true
}

// sendSRChat runs send under the record's single chat claim -- the same
// shape as handleCaseAcknowledged, the closest existing Chat-only handler,
// and for the same reasons (see its doc comment):
//
//   - The claim keeps two concurrent Handle calls for one record (a
//     consumer-group rebalance) from both posting: the loser touches nothing
//     and returns nil.
//   - A failed post releases the claim and returns its error, so the retry
//     posts it. That is never a re-send: the post is the only side effect,
//     so a failed attempt has sent nothing.
//   - A successful post releases the claim too. The record then commits and
//     is never retried, so holding the key would only leak it in d.done
//     (checkFrustration holds its key past success because a *sibling*
//     channel can still fail and retry the record; there is no sibling
//     here). A redelivery after a successful post -- a crash before the
//     offset commit -- re-posts, the accepted at-least-once trade-off
//     d.done's own doc comment describes.
//
// Not record.NoMoreRetries-gated, for handleCaseAcknowledged's reason: with
// one claim, only its owner ever releases it.
func (d *Dispatcher) sendSRChat(ctx context.Context, record eventbus.Record, send func() error) error {
	chatKey := recordBaseKey(record) + "/chat"
	if !d.claim(chatKey) {
		return nil
	}
	defer d.forget(chatKey)
	return send()
}

// hasSRTag reports whether tags contains want, trimmed and case-insensitively.
func hasSRTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), want) {
			return true
		}
	}
	return false
}
