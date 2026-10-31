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

package paging

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// postStopped replies in a ladder's thread that it stopped (ackChat): for
// reason, or -- next set -- because a severity or priority change replaced it
// with a new chain. The reply goes where the rung cards went, and only once
// one has gone out. Best effort: a failure is logged, never returned, because
// the calls have already stopped and that is what matters.
func (e *Engine) postStopped(ctx context.Context, st LadderState, reason cancelReason, next *Trigger) {
	t := st.Plan.Trigger
	switch {
	case !e.cfg.Ladder.AckChatOn():
		return
	case reason == cancelEscalatedToSME:
		// The hand-off posts its own message (smeHandoffChat).
		return
	case !e.cfg.CallSendingEnabled || (st.PlacedCount() == 0 && len(st.Plan.IncidentCards) == 0):
		// No rung card or incident card went out, so the room never saw this chain.
		return
	}
	rooms := e.cardRooms(st.Plan)
	if len(rooms) == 0 {
		return
	}

	msg := e.stoppedWording(ctx, st, reason, next)
	msg.Chain = chainLabel(t)
	msg.IncidentRef = t.caseRef()
	msg.Title = t.Title
	msg.Priority = t.Priority
	msg.Paged = pagedList(st)
	msg.Cancelled = unplacedCount(st)
	msg.ThreadKey = t.chatThreadKey()
	msg.PortalLabel = "View incident"
	if t.isCase() {
		msg.PortalLabel = "View case"
	}

	for _, r := range rooms {
		room := r.roomFor()
		if !r.chat.HasAudienceSpace(room) {
			continue
		}
		if st.PlacedCount() == 0 && !slices.Contains(st.Plan.IncidentCards, room) {
			// Before any rung, only a room the incident card reached saw this chain.
			continue
		}
		msg.Audience, msg.Reply = room, r.reply || e.threadReplies(t)
		// The link the room's rung cards carry.
		var links incidentLinker = e.links
		if r.links != nil {
			links = r.links
		}
		msg.PortalURL = links.IncidentLink(t.IncidentID)
		if t.isCase() {
			msg.PortalURL = links.CaseLink(t.IncidentID)
		}
		if !r.reply && msg.Answered && reason != cancelAssigned && t.sreIncident() && e.threadReplies(t) {
			// An acknowledged SRE incident's thread already ends with the "Incident Acknowledged." card, so no stopped reply.
			continue
		}
		if !r.reply && reason == cancelAssigned && t.sreIncident() && e.threadReplies(t) {
			// An assigned SRE incident ends its thread with the "Incident Acknowledged." card, naming only the assignee.
			ack := notifications.IncidentAssignedAlert{IncidentID: t.IncidentID, Number: t.Number, AssigneeName: st.AssigneeName}
			if err := r.chat.SendIncidentAssignedAlert(ctx, room, ack); err != nil {
				slog.ErrorContext(ctx, "escalation: the acknowledged reply was not posted",
					"incidentId", t.IncidentID, "audience", room, "err", err)
				continue
			}
			slog.InfoContext(ctx, "escalation: acknowledged reply posted", "incidentId", t.IncidentID, "audience", room)
			continue
		}
		if err := r.chat.SendEscalationStopped(ctx, msg); err != nil {
			slog.ErrorContext(ctx, "escalation: the stopped reply was not posted",
				"incidentId", t.IncidentID, "audience", room, "err", err)
			continue
		}
		slog.InfoContext(ctx, "escalation: stopped reply posted",
			"incidentId", t.IncidentID, "audience", room, "outcome", msg.Outcome)
	}
}

// stoppedWording is what the reply says happened.
func (e *Engine) stoppedWording(ctx context.Context, st LadderState, reason cancelReason, next *Trigger) notifications.EscalationStopped {
	t := st.Plan.Trigger
	if next != nil {
		field := "Priority"
		if t.isCase() {
			field = "Severity"
		}
		to := strings.ToUpper(strings.TrimSpace(next.Priority))
		// No rung is named: the new plan is not built yet, and its first rung
		// is L0 on a CRE ladder whose rule has one, L1 otherwise.
		return notifications.EscalationStopped{
			Outcome:   fmt.Sprintf("%s changed to %s", field, to),
			Detail:    fmt.Sprintf("%s changed from %s to %s. A new chain for %s starts from the first rung.", field, strings.ToUpper(strings.TrimSpace(t.Priority)), to, to),
			Restarted: true,
		}
	}

	how := ""
	switch reason {
	case cancelAcknowledged:
		how = "moved out of NEW and a public comment"
	case cancelCaseAcknowledged:
		how = "assigned and a public comment"
	case cancelStateChange:
		how = "moved out of NEW"
	case cancelPublicComment:
		how = "a public comment"
	case cancelAssigned:
		how = "assigned"
	case cancelAssigneeComment:
		how = "a public comment by the assignee"
	case cancelCaseClosed:
		return notifications.EscalationStopped{Outcome: "Case closed", Detail: "The case was closed."}
	default:
		if strings.HasPrefix(string(reason), "Severity changed to ") {
			return notifications.EscalationStopped{
				Outcome: string(reason),
				Detail:  "This chain does not page that severity.",
			}
		}
		return notifications.EscalationStopped{Outcome: string(reason)}
	}

	detail := fmt.Sprintf("Acknowledged (%s).", how)
	if who := e.acknowledger(ctx, t); who != "" {
		detail = fmt.Sprintf("Acknowledged by %s (%s).", who, how)
	}
	return notifications.EscalationStopped{Outcome: "Acknowledged", Detail: detail, Answered: true}
}

// acknowledger is who picked the record up, when it is known: a case's
// assignee. An incident's acknowledgement events name nobody.
func (e *Engine) acknowledger(ctx context.Context, t Trigger) string {
	if !t.isCase() || e.store == nil {
		return ""
	}
	who, err := e.store.CaseAssignee(ctx, t.IncidentID)
	if err != nil {
		slog.WarnContext(ctx, "escalation: could not read the case assignee for the stopped reply",
			"incidentId", t.IncidentID, "err", err)
		return ""
	}
	return who
}

// cardRooms is where a plan's rung cards go -- the same choice place makes --
// and so where its stopped reply goes.
func (e *Engine) cardRooms(plan Plan) []chatNotifier {
	notifiers := withChannel(e.notifiersFor(plan), e.cfg.Channel, e.channelFor(plan.Trigger))
	if plan.Trigger.Routing.Ladder == LadderSME {
		notifiers = e.smeNotifiersFor(plan.Trigger.Routing.SMETeam)
	}
	var out []chatNotifier
	for _, n := range onlyChat(notifiers) {
		switch c := n.(type) {
		case chatNotifier:
			out = append(out, c)
		case fanoutChat:
			out = append(out, c.chats...)
		}
	}
	return out
}

// chainLabel names the chain for the people reading the reply.
func chainLabel(t Trigger) string {
	switch t.Routing.Ladder {
	case LadderSME:
		return smeTeamLabel(t.Routing.SMETeam) + " SME"
	case LadderSRE:
		if c := t.Routing.SREChain(); c != "" {
			return c
		}
		return "SRE"
	}
	return "CRE"
}

// pagedList is who each rung reached, once per person and rung, in order: a
// rung's repeat attempts page the same people again.
func pagedList(st LadderState) []string {
	var out []string
	seen := map[string]bool{}
	for i, done := range st.Placed {
		if !done || i >= len(st.Plan.Calls) {
			continue
		}
		c := st.Plan.Calls[i]
		if c.Recipient.Name == "" {
			continue
		}
		label := c.Level.LabelIn(st.Plan.Trigger.Routing.Ladder) + " " + c.Recipient.Name
		if !seen[label] {
			seen[label] = true
			out = append(out, label)
		}
	}
	return out
}

// unplacedCount is how many calls were still to come: neither placed nor
// rejected for good.
func unplacedCount(st LadderState) int {
	n := 0
	for i := range st.Placed {
		if !st.Placed[i] && st.failure(i) == "" {
			n++
		}
	}
	return n
}
