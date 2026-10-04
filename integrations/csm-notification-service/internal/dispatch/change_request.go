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

// The change_request.* notice handlers.

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

// handleCRApprovalRequested emails the people a change request is waiting on.
//
// UNLIKE EVERY OTHER HANDLER HERE, it does not resolve recipients or build a
// subject. csm-flow-service's cr_approval_notice flow does both before
// publishing: the audience comes from an approval group or a project's
// contacts, and the subject reproduces the legacy ticketing system's per-branch wording
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
	// The legacy ticketing system sent one email per recipient, so nobody ever saw who else was
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

// handleCRPlanDateNotice emails one turn of the plan-start-date conversation:
// a customer proposing a new date (internal audience), or WSO2 accepting or
// rejecting one (customer audience).
//
// Same division of labour as handleCRApprovalRequested — the flow resolves the
// recipients and builds the subject, both reproduced verbatim from the legacy ticketing system,
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
	// a project's contacts span organisations, and the legacy ticketing system sent these one
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
