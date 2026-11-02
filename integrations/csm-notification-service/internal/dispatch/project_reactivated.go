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
// KIND, either express or implied. See the License for the
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

// The reason lines of the "Back to Open state" email, worded as the legacy
// email it replaces.
const (
	reactivatedRenewalLine = "The following project has been reopened following the successful renewal of the project subscription."
	reactivatedInvoiceLine = "The following project has been reopened following the payment of the due invoice."
	reactivatedBothLine    = "The following project has been reopened following the successful renewal of the project subscription and the payment of the due invoice."
	reactivatedOtherLine   = "The following project has been reopened."
)

// acpDefaultInternalDomain is where the "Back to Open state" email may go
// when no domain was configured.
const acpDefaultInternalDomain = "@wso2.com"

// handleProjectReactivated sends the "Back to Open state" email when
// entity-service's ACP reopen check has set a suspended project back to Open.
// To are the payload's recipients (account owner, renewal account manager,
// technical owner) that are well-formed internal addresses; CC is
// ACP_REACTIVATION_CC. With no such recipient the CC lists get the email in
// To, so somebody hears the project reopened.
//
// Duplicates: entity-service publishes at least once, so the same notice
// can arrive again (a redelivery after a restart or a rebalance, before the
// offset was committed). With a ledger (Redis), the notice is reserved before
// the send and marked sent after it, keyed by project and reactivatedOn: a
// notice already sent is skipped, one being sent by another attempt is
// retried later. Within one process the claim also holds off a concurrent
// delivery of the same record, which returns nil (the winner owns the send).
// Without a ledger only the claim guards, so a redelivery after a restart
// can send the email again.
func (d *Dispatcher) handleProjectReactivated(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.ProjectReactivatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode project.reactivated payload: %w", err)
	}
	logAttrs := []any{"projectId", p.ProjectID, "projectKey", p.ProjectKey}
	to := d.reactivationRecipients(ctx, p.Recipients, logAttrs)
	cc := withoutAddresses(d.acpReactivationCC, to)
	if len(to) == 0 {
		if len(cc) == 0 {
			slog.WarnContext(ctx, "dispatch: project.reactivated has no internal recipient and no CC list, no email sent", logAttrs...)
			return nil
		}
		slog.WarnContext(ctx, "dispatch: project.reactivated has no internal recipient, sending the back-to-open email to the CC lists", logAttrs...)
		to, cc = cc, nil
	}
	if !d.emailSendingEnabled {
		slog.InfoContext(ctx, "dispatch: email sending disabled, skipping the back-to-open email", logAttrs...)
		return nil
	}
	var intendedFor string
	if d.emailDebugMode {
		if len(d.emailDebugRecipients) == 0 {
			slog.WarnContext(ctx, "dispatch: email debug mode on with no debug recipients, skipping the back-to-open email", logAttrs...)
			return nil
		}
		intendedFor = strings.Join(append(append([]string{}, to...), cc...), ", ")
		to, cc = d.emailDebugRecipients, nil
	}

	key := recordBaseKey(record) + "/project-reactivated"
	if !d.claim(key) {
		// A concurrent delivery of the same record owns the send; returning
		// an error here would retry it after the winner released the claim.
		slog.InfoContext(ctx, "dispatch: back-to-open email already in progress for this record", logAttrs...)
		return nil
	}
	defer d.forget(key)

	ledgerKey := p.ProjectID + "/" + p.ReactivatedOn
	if d.acpReactivationLedger != nil {
		reserved, sent, err := d.acpReactivationLedger.Reserve(ctx, ledgerKey)
		switch {
		case err != nil:
			return fmt.Errorf("dispatch: reserve back-to-open email for project %s: %w", p.ProjectID, err)
		case sent:
			slog.InfoContext(ctx, "dispatch: back-to-open email already sent, not sending it again", logAttrs...)
			return nil
		case !reserved:
			return fmt.Errorf("dispatch: back-to-open email for project %s is being sent by another attempt", p.ProjectID)
		}
	}

	body := notifications.RenderProjectReactivatedEmail(notifications.ProjectReactivatedEmailData{
		GreetingName: greetingFirstName(p.AccountOwnerName),
		ReasonLine:   reactivatedReasonLine(p.Reasons),
		ProjectName:  p.ProjectName,
		ProjectKey:   p.ProjectKey,
		ProjectLink:  d.links.ProjectLink(p.ProjectID),
		AccountOwner: firstNonBlank(p.AccountOwnerName, p.AccountOwnerEmail),
		StartDate:    p.StartDate,
		EndDate:      p.EndDate,
		IntendedFor:  intendedFor,
	})
	subject := "Back to Open state : " + p.ProjectName
	if err := d.email.SendEmail(ctx, to, cc, nil, nil, subject, body, nil); err != nil {
		if d.acpReactivationLedger != nil {
			if relErr := d.acpReactivationLedger.Release(context.WithoutCancel(ctx), ledgerKey); relErr != nil {
				slog.WarnContext(ctx, "dispatch: release back-to-open reservation failed; it expires on its own", append(logAttrs, "err", relErr)...)
			}
		}
		return fmt.Errorf("dispatch: send back-to-open email for project %s: %w", p.ProjectID, err)
	}
	if d.acpReactivationLedger != nil {
		// Not cancellable: a shutdown that lost this write would let the
		// redelivery after the restart send the email again.
		if err := d.acpReactivationLedger.MarkSent(context.WithoutCancel(ctx), ledgerKey); err != nil {
			slog.ErrorContext(ctx, "dispatch: back-to-open email sent but not recorded as sent; a redelivery may send it again",
				append(logAttrs, "err", err)...)
		}
	}
	slog.InfoContext(ctx, "dispatch: back-to-open email sent", append(logAttrs, "to", len(to), "cc", len(cc))...)
	return nil
}

// reactivationRecipients keeps the payload's recipients that are well-formed
// addresses in the internal domain, each once. A dropped one is counted in a
// WARN, never logged by address.
func (d *Dispatcher) reactivationRecipients(ctx context.Context, recipients []string, logAttrs []any) []string {
	domain := strings.ToLower(strings.TrimSpace(d.acpInternalDomain))
	if domain == "" {
		domain = acpDefaultInternalDomain
	}
	var out []string
	invalid, external := 0, 0
	seen := map[string]bool{}
	for _, r := range recipients {
		a := strings.TrimSpace(r)
		switch {
		case !events.LooksLikeEmail(a):
			invalid++
		case !strings.HasSuffix(strings.ToLower(a), domain):
			external++
		case !seen[strings.ToLower(a)]:
			seen[strings.ToLower(a)] = true
			out = append(out, a)
		}
	}
	if invalid > 0 || external > 0 {
		slog.WarnContext(ctx, "dispatch: project.reactivated recipients dropped",
			append(logAttrs, "invalid", invalid, "notInternal", external, "kept", len(out))...)
	}
	return out
}

// reactivatedReasonLine picks the sentence for why the project reopened.
func reactivatedReasonLine(reasons []string) string {
	var renewal, paid bool
	for _, r := range reasons {
		switch r {
		case events.ProjectReactivatedReasonRenewal:
			renewal = true
		case events.ProjectReactivatedReasonInvoicePaid:
			paid = true
		}
	}
	switch {
	case renewal && paid:
		return reactivatedBothLine
	case renewal:
		return reactivatedRenewalLine
	case paid:
		return reactivatedInvoiceLine
	}
	return reactivatedOtherLine
}

// greetingFirstName is the first word of the account owner's name, as the
// legacy email greets ("Dear Alex,"); "Team" when there is no name.
func greetingFirstName(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}
	return "Team"
}

// firstNonBlank returns the first value that is not blank, trimmed.
func firstNonBlank(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// withoutAddresses returns list without the addresses already in exclude
// (compared ignoring case), so nobody gets the email twice.
func withoutAddresses(list, exclude []string) []string {
	seen := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		seen[strings.ToLower(strings.TrimSpace(e))] = true
	}
	var out []string
	for _, a := range list {
		k := strings.ToLower(strings.TrimSpace(a))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, strings.TrimSpace(a))
	}
	return out
}
