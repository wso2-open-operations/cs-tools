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
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// Case Paging from customer cases. The rules are the agreed ones
// (task-call-alert-flow/Case Paging Rules.xlsx, 2026-10-07):
//
//   - A case S0 pages CRE and SRE; S1-S4 page CRE only (which severities is
//     cre.trigger.priorities -- S4 is off unless listed). Routing decides
//     which ladders, as it does for incidents (DefaultRouting).
//   - Any severity change stops the running chain and starts a new one for the
//     new severity, from the first tier, even on a case already acknowledged.
//     A severity a ladder does not page (S4 when it is off; anything below S0
//     for SRE) just stops that ladder's chain.
//   - CRE stops when an engineer is assigned AND that engineer posts a public
//     comment. SRE on an S0 case stops when any engineer is assigned, or --
//     on a case already assigned when it became S0 -- when the assignee posts
//     a public comment. Only gestures after the chain started count; a
//     customer's comment and a work note never stop paging.
//   - A closed case stops both.
//
// The case events that start a chain carry no assignee, so the current one is
// remembered from case.assigned (Store.SetCaseAssignee).

// Case gesture reasons, reported on the work note.
const (
	cancelCaseAcknowledged cancelReason = "Acknowledged (assigned and public comment)"
	cancelAssigneeComment  cancelReason = "Public comment by the assignee"
	cancelCaseClosed       cancelReason = "Case closed"
)

// caseGesture is one thing an engineer did on a case.
type caseGesture int

const (
	// gestureAssigned is an engineer assigned to the case.
	gestureAssigned caseGesture = iota
	// gestureAssigneeComment is a public comment by the current assignee.
	gestureAssigneeComment
	// gestureEngineerComment is a support engineer's public comment while
	// they are not (yet) the assignee: remembered, so it counts if they are
	// assigned next.
	gestureEngineerComment
)

// handleCase reacts to one customer case event. Validate has already run.
func (e *Engine) handleCase(ctx context.Context, env events.Envelope) error {
	caseID := env.EntityID
	switch env.Type {
	case events.TypeCaseCreated:
		var p events.CaseCreatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("escalation: decode case.created payload: %w", err)
		}
		// Only a support case carries a severity; a service request,
		// engagement, security report or announcement is never paged.
		if !strings.EqualFold(strings.TrimSpace(p.CaseType), "case") {
			return nil
		}
		return e.start(ctx, triggerFromCaseCreated(caseID, p), false)

	case events.TypeSeverityChanged:
		var p events.SeverityChangedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("escalation: decode case.severity_changed payload: %w", err)
		}
		t := triggerFromSeverityChanged(caseID, p, e.now())
		if e.admit(ctx, &t) {
			// A new chain for the new severity, replacing whatever runs.
			return e.schedule(ctx, t, true)
		}
		if e.pagesSeverity(ctx, t) {
			// Refused for a reason that is not the severity -- a team or
			// shift setting in trigger -- which decides whether a chain may
			// START, not whether a running one should stop. Silencing a case
			// raised outside an allowed shift would be the worst outcome.
			slog.InfoContext(ctx, "escalation: severity change refused by a team or shift setting; the running chain carries on",
				"incidentId", caseID, "ladder", ladderName(e.cfg.Kind), "priority", t.Priority)
			return nil
		}
		// This ladder does not page the new severity: S4 while it is off, or
		// the SRE ladder below S0. Whatever it was running stops.
		return e.stopRunning(ctx, caseID, cancelReason("Severity changed to "+strings.ToUpper(strings.TrimSpace(p.NewSeverity))))

	case events.TypeCaseAssigned:
		var p events.CaseAssignedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("escalation: decode case.assigned payload: %w", err)
		}
		assignee := normaliseEmail(p.AssigneeEmail)
		if err := e.store.SetCaseAssignee(ctx, caseID, assignee); err != nil {
			return fmt.Errorf("escalation: record assignee for case %s: %w", caseID, err)
		}
		return e.caseGesture(ctx, caseID, gestureAssigned, assignee)

	case events.TypeCommentAdded:
		var p events.CommentAddedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("escalation: decode case.comment_added payload: %w", err)
		}
		if p.IsInternalNote {
			// A work note is not a reply to anybody, and the engine's own
			// execution summary arrives here as one.
			return nil
		}
		author := normaliseEmail(p.AuthorEmail)
		if author == "" {
			return nil
		}
		assignee, err := e.store.CaseAssignee(ctx, caseID)
		if err != nil {
			return fmt.Errorf("escalation: read assignee for case %s: %w", caseID, err)
		}
		if assignee != "" && strings.EqualFold(author, assignee) {
			return e.caseGesture(ctx, caseID, gestureAssigneeComment, author)
		}
		if p.IsSupportEngineerResponse {
			return e.caseGesture(ctx, caseID, gestureEngineerComment, author)
		}
		// A customer, or anybody else who is not on the case: never stops it.
		return nil

	case events.TypeStatusChanged:
		var p events.StatusChangedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("escalation: decode case.status_changed payload: %w", err)
		}
		if !isClosedCaseStatus(p.NewStatus) {
			return nil
		}
		return e.stopRunning(ctx, caseID, cancelCaseClosed)
	}
	return nil
}

// caseGesture records one gesture on a running case chain and stops it when
// the ladder's rule is met.
func (e *Engine) caseGesture(ctx context.Context, caseID string, g caseGesture, who string) error {
	st, found, err := e.store.Get(ctx, caseID)
	if err != nil {
		return fmt.Errorf("escalation: load ladder for case %s: %w", caseID, err)
	}
	if !found || !st.Plan.Trigger.isCase() {
		return nil
	}
	if st.Cancelled != nil {
		// A stop that did not finish (the summary or the delete failed):
		// finish it with the reason it was stopped for.
		return e.stopLadder(ctx, caseID, st, cancelReason(st.CancelReason))
	}

	switch g {
	case gestureAssigned:
		st.SawAssigned = true
		if containsFold(st.CommentAuthors, who) {
			st.SawAssigneeComment = true
		}
		if e.cfg.Kind == LadderSRE {
			// Any engineer assigned after the case became S0.
			return e.stopLadder(ctx, caseID, st, cancelAssigned)
		}
	case gestureAssigneeComment:
		st.SawAssigneeComment = true
	case gestureEngineerComment:
		if !containsFold(st.CommentAuthors, who) {
			st.CommentAuthors = append(st.CommentAuthors, who)
		}
		if err := e.store.Save(ctx, caseID, st); err != nil {
			return fmt.Errorf("escalation: record comment on case %s: %w", caseID, err)
		}
		return nil
	}

	if e.cfg.Kind == LadderSRE {
		if st.SawAssigneeComment {
			// The case was already assigned when it became S0; the assignee
			// answering stops SRE as well as CRE.
			return e.stopLadder(ctx, caseID, st, cancelAssigneeComment)
		}
	} else {
		both := st.SawAssigned && st.SawAssigneeComment
		either := st.SawAssigned || st.SawAssigneeComment
		if both || (!e.cfg.Ladder.RequireBothGestures() && either) {
			return e.stopLadder(ctx, caseID, st, cancelCaseAcknowledged)
		}
	}

	if err := e.store.Save(ctx, caseID, st); err != nil {
		return fmt.Errorf("escalation: record acknowledgement for case %s: %w", caseID, err)
	}
	missing := "an engineer assigned"
	if st.SawAssigned {
		missing = "a public comment by the assignee"
	}
	slog.InfoContext(ctx, "escalation: half acknowledged; the chain keeps climbing",
		"incidentId", caseID, "ladder", ladderName(e.cfg.Kind), "stillNeeds", missing,
		"reachedLevel", st.ReachedLevel())
	return nil
}

// pagesSeverity reports whether this ladder pages t's severity at all: routing
// takes it (for the SRE ladder, only S0), it has a clock, and it is in
// trigger.priorities. Unlike admit it ignores the team and shift settings.
func (e *Engine) pagesSeverity(ctx context.Context, t Trigger) bool {
	if !e.claims(ctx, &t) {
		return false
	}
	if _, ok := PolicyFor(e.policies, t); !ok {
		return false
	}
	return matchesPriority(e.cfg.Ladder.Start.Priorities, t.Priority)
}

// stopRunning stops whatever chain this ladder runs for the case, if any.
func (e *Engine) stopRunning(ctx context.Context, caseID string, reason cancelReason) error {
	st, found, err := e.store.Get(ctx, caseID)
	if err != nil {
		return fmt.Errorf("escalation: load ladder for case %s: %w", caseID, err)
	}
	if !found {
		return nil
	}
	if st.Cancelled != nil {
		reason = cancelReason(st.CancelReason)
	}
	return e.stopLadder(ctx, caseID, st, reason)
}

// seedCaseAssignment marks a new case chain assigned when the case already
// has an assignee: a case raised while somebody is on it stops on that
// engineer's comment, without waiting for an assignment that already
// happened. Incident ladders are left alone.
func (e *Engine) seedCaseAssignment(ctx context.Context, st *LadderState) error {
	if !st.Plan.Trigger.isCase() {
		return nil
	}
	assignee, err := e.store.CaseAssignee(ctx, st.Plan.Trigger.IncidentID)
	if err != nil {
		return fmt.Errorf("escalation: read assignee for case %s: %w", st.Plan.Trigger.IncidentID, err)
	}
	st.SawAssigned = assignee != ""
	return nil
}

// triggerFromCaseCreated builds a new chain's trigger from a support case.
func triggerFromCaseCreated(caseID string, p events.CaseCreatedPayload) Trigger {
	at := reportedAt(p.CreatedAt)
	return Trigger{
		IncidentID: caseID,
		Number:     p.CaseNumber,
		WSO2CaseID: p.WSO2CaseID,
		Priority:   p.Priority,
		Title:      p.CaseTitle,
		Account:    p.ProjectName,
		Team:       p.Team,
		Kind:       TriggerNewIncident,
		At:         at,
		Record:     RecordCase,
		Routing: RoutingContext{
			Product:         p.Product,
			AssignedCRETeam: p.Team,
			Shift:           ShiftAt(at),
			At:              at,
		},
	}
}

// triggerFromSeverityChanged builds the new chain's trigger for a case's new
// severity. The event carries no timestamp, so the chain runs from now.
func triggerFromSeverityChanged(caseID string, p events.SeverityChangedPayload, now time.Time) Trigger {
	return Trigger{
		IncidentID: caseID,
		Number:     p.CaseNumber,
		WSO2CaseID: p.WSO2CaseID,
		Priority:   p.NewSeverity,
		Title:      p.CaseTitle,
		Team:       p.Team,
		Kind:       TriggerSeverityChanged,
		At:         now,
		Record:     RecordCase,
		Routing: RoutingContext{
			Product:         p.Product,
			AssignedCRETeam: p.Team,
			Shift:           ShiftAt(now),
			At:              now,
		},
	}
}

// isClosedCaseStatus reports whether a case.status_changed label means the
// case is finished. entity-service's labels end at "Closed"; ServiceNow may
// pass its own through, so a resolved or cancelled label counts too.
func isClosedCaseStatus(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "closed" || strings.Contains(s, "resolved") || strings.Contains(s, "cancel")
}

func normaliseEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
