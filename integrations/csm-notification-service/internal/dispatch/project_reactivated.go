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

// The reason lines of the "Back to Open state" email, worded as the
// ServiceNow email it replaces.
const (
	reactivatedRenewalLine = "The following project has been reopened following the successful renewal of the project subscription."
	reactivatedInvoiceLine = "The following project has been reopened following the payment of the due invoice."
	reactivatedBothLine    = "The following project has been reopened following the successful renewal of the project subscription and the payment of the due invoice."
	reactivatedOtherLine   = "The following project has been reopened."
)

// handleProjectReactivated sends the "Back to Open state" email when
// entity-service's ACP reopen check has set a suspended project back to Open.
// To are the payload's recipients (account owner, renewal account manager,
// technical owner); CC is ACP_REACTIVATION_CC. One email, one channel: a
// failed send is returned and retried, and nothing sent can be sent twice by
// a retry, since a retry only follows a failure. A concurrent delivery of the
// same record is held off by the claim.
func (d *Dispatcher) handleProjectReactivated(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.ProjectReactivatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode project.reactivated payload: %w", err)
	}
	logAttrs := []any{"projectId", p.ProjectID, "projectKey", p.ProjectKey}
	if len(p.Recipients) == 0 {
		slog.WarnContext(ctx, "dispatch: project.reactivated has no recipients, no email sent", logAttrs...)
		return nil
	}
	if !d.emailSendingEnabled {
		slog.InfoContext(ctx, "dispatch: email sending disabled, skipping the back-to-open email", logAttrs...)
		return nil
	}
	to := p.Recipients
	cc := withoutAddresses(d.acpReactivationCC, to)
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
		return fmt.Errorf("dispatch: back-to-open email for project %s is already in progress", p.ProjectID)
	}
	defer d.forget(key)

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
		return fmt.Errorf("dispatch: send back-to-open email for project %s: %w", p.ProjectID, err)
	}
	slog.InfoContext(ctx, "dispatch: back-to-open email sent", append(logAttrs, "to", len(to), "cc", len(cc))...)
	return nil
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
// ServiceNow email greets ("Dear Alex,"); "Team" when there is no name.
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
