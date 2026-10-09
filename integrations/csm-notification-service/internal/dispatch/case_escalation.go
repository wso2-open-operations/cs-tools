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

// handleCaseEscalated emails a case escalation to the people it resolved --
// the payload's Recipients, which entity-service took from the escalation's
// notification list (ServiceNow's u_notification_list). Ports ServiceNow's
// "Internal Escalation notification" flow: one message, To = the list, which
// is internal WSO2 staff (team leads, account owners, management).
//
// The send is the only side effect and the last step, so a retry after a
// failed send cannot duplicate anything.
func (d *Dispatcher) handleCaseEscalated(ctx context.Context, _ eventbus.Record, raw json.RawMessage) error {
	var p events.CaseEscalatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.escalated payload: %w", err)
	}

	recipients := p.Recipients
	if !d.emailSendingEnabled {
		slog.InfoContext(ctx, "dispatch: email sending disabled, skipping case escalation email",
			"caseId", p.CaseID, "escalationId", p.EscalationID, "recipients", len(recipients))
		return nil
	}
	var intendedFor string
	if d.emailDebugMode {
		if len(d.emailDebugRecipients) == 0 {
			slog.WarnContext(ctx, "dispatch: email debug mode on with no debug recipients, skipping case escalation email",
				"caseId", p.CaseID, "escalationId", p.EscalationID)
			return nil
		}
		intendedFor = strings.Join(recipients, ", ")
		recipients = d.emailDebugRecipients
	}

	subject := notifications.CaseEscalatedSubject(p.CurrentLevel)
	body := notifications.RenderCaseEscalatedEmail(notifications.CaseEscalatedEmailData{
		CaseNumber:            displayCaseRef(p.CaseNumber, p.CaseID),
		CaseTitle:             p.CaseTitle,
		ActorEmail:            p.ActorEmail,
		AccountName:           p.AccountName,
		PreviousLevel:         p.PreviousLevel,
		CurrentLevel:          p.CurrentLevel,
		EscalatedOn:           p.EscalatedOn,
		Reason:                p.Reason,
		Product:               p.Product,
		Severity:              p.Severity,
		Environment:           p.Environment,
		AssignedEngineerEmail: p.AssignedEngineerEmail,
		Link:                  d.links.CSMLink(p.CaseID),
		IntendedFor:           intendedFor,
	})

	if err := d.email.SendEmail(ctx, recipients, nil, nil, nil, subject, body, nil); err != nil {
		return fmt.Errorf("dispatch: send case escalation email for %s: %w", p.CaseID, err)
	}
	slog.InfoContext(ctx, "dispatch: case escalation email sent",
		"caseId", p.CaseID, "escalationId", p.EscalationID,
		"level", p.CurrentLevel, "recipients", len(recipients))
	return nil
}
