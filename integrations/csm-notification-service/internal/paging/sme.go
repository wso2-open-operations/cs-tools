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
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// The Special Ops (SME) page.
//
// SME -- Subject Matter Specialist -- is a separate team paged ONLY when an
// assigned SaaS SRE incident is escalated to a Special Ops team. entity-service
// publishes incident.special_ops_alert on the operations topic (sre-events)
// whenever an incident's assignment group changes INTO a Special Ops group --
// whatever made the change, which today is the "Escalate to Special Ops Team"
// button -- and only when it has SRE_EVENT_HUB_TOPIC set. The dispatcher's
// sre-events consumer hands it here (HandleSpecialOpsAlert); the paging
// engines' own consumers ignore the type, so one alert pages once. The SaaS
// SRE chain stops, and an SME ladder starts for the chosen SME team: L1 of
// its current Day/Night window at once, L2 and L3 after sme.timing's interval
// each (five minutes by default), each rung asked at the moment it opens.
// Someone rostered on the window with no tier counts as L1. The ladder runs on
// the SRE engine's ticker under its own key (smeLadderKey), so several SME
// teams can run on one incident at once.
//
//	same SME team alerted again while its page is open   ignored
//	a different SME team                                 its own ladder
//	an engineer assigned afterwards                      stops the ladders raised
//	                                                     before the assignment,
//	                                                     so a later alert pages
//	an assignment handled before the alert it answers    the alert pages nobody
//
// The assignment and the alert come on different topics, so either may be
// handled first; both are compared by time (the assignment's assignedOn, the
// alert's changedOn), not by arrival.
//
// IaaS SRE incidents, CRE teams and unknown groups never page SME (the gate
// below). The escalate button needs an assignee first: entity-service refuses
// a handoff on an unassigned incident (409 incident_handoff_needs_assignee),
// and the CSM portal disables the button.

// smePageTTL bounds an open SME page that no assignment ever closes.
const smePageTTL = 24 * time.Hour

// SpecialistResolver is implemented by a Resolver that can tell whether an
// assignment group is a SaaS SRE team -- the only kind an SME page may follow.
// A resolver that cannot (the roster) places no SME page at all.
type SpecialistResolver interface {
	SaaSSRETeam(ctx context.Context, group string) (bool, error)
}

// specialOpsPress is one move into a Special Ops group, as the page needs it.
type specialOpsPress struct {
	IncidentID, Number, Title, Product, Priority string
	// Service and State fill the SME space's incident card.
	Service, State string
	// TeamKey/TeamLabel are the Special Ops team; SMETeam the rota team to
	// page when the publisher names it.
	TeamKey, TeamLabel, SMETeam string
	// PreviousGroup/PreviousGroupID are the group the incident left.
	PreviousGroup, PreviousGroupID string
	AssignmentGroup                string
	ChangedBy                      string
	At                             time.Time
}

// HandleSpecialOpsAlert places the SME page for incident.special_ops_alert;
// dispatch calls it from the sre-events consumer. An error is returned only
// for what a retry can fix (Redis, the rota unreachable, a transient call
// failure); everything else is logged and, where it concerns the incident,
// written onto it. Only the SRE engine pages.
func (e *Engine) HandleSpecialOpsAlert(ctx context.Context, incidentID string, p events.IncidentSpecialOpsAlertPayload) error {
	if e.cfg.Kind != LadderSRE {
		return nil
	}
	group := p.AssignmentGroupName
	if group == "" {
		group = p.AssignmentGroupID
	}
	return e.handleSpecialOps(ctx, specialOpsPress{
		IncidentID: incidentID, Number: p.Number, Title: p.Subject, Product: p.Product, Priority: p.Priority,
		Service: p.ServiceName, State: p.State,
		TeamKey: p.TeamKey, TeamLabel: p.TeamLabel, SMETeam: p.SMETeam,
		PreviousGroup: p.PreviousAssignmentGroupName, PreviousGroupID: p.PreviousAssignmentGroupID,
		AssignmentGroup: group, ChangedBy: p.ChangedBy, At: reportedAt(p.ChangedOn),
	})
}

// isSaaSSREGroup applies the gate to the group the incident left, by name and
// then by id (either may be what sre.teams.aliases maps).
func (e *Engine) isSaaSSREGroup(ctx context.Context, gate SpecialistResolver, pr specialOpsPress) (bool, error) {
	for _, g := range []string{pr.PreviousGroup, pr.PreviousGroupID} {
		if strings.TrimSpace(g) == "" {
			continue
		}
		ok, err := gate.SaaSSRETeam(ctx, g)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// handleSpecialOps places the SME page for one alert; see the top of this file.
func (e *Engine) handleSpecialOps(ctx context.Context, pr specialOpsPress) error {
	incidentID := pr.IncidentID
	log := slog.With("incidentId", incidentID, "incident", pr.Number, "specialOpsTeam", pr.TeamKey)
	if !e.cfg.SME.Enabled {
		log.InfoContext(ctx, "escalation: SME paging is off (sme.enabled); ignoring the Special Ops alert")
		return nil
	}
	if strings.TrimSpace(incidentID) == "" {
		log.WarnContext(ctx, "escalation: Special Ops alert carries no incident id; no SME page")
		return nil
	}
	gate, ok := e.resolver.(SpecialistResolver)
	if !ok {
		log.WarnContext(ctx, "escalation: this resolver cannot read an SME rota; no SME page")
		return nil
	}
	saas, err := e.isSaaSSREGroup(ctx, gate, pr)
	if err != nil {
		return fmt.Errorf("escalation: tell whether %s was a SaaS SRE incident: %w", incidentID, err)
	}
	if !saas {
		log.InfoContext(ctx, "escalation: the incident did not come from a SaaS SRE team; no SME page",
			"previousAssignmentGroup", pr.PreviousGroup, "previousAssignmentGroupId", pr.PreviousGroupID)
		return nil
	}

	// The SaaS SRE chain stops whatever happens to the page: the incident is
	// Special Ops' now.
	if err := e.stopForHandoff(ctx, incidentID); err != nil {
		return err
	}

	team := teamKeyFor(pr.SMETeam)
	if team == "" {
		team = e.cfg.SME.TeamFor(pr.TeamKey)
	}
	t := smeTrigger(pr, team)
	t.Routing.CallOnLeave = e.cfg.SME.OnLeave.Calls()
	if team == "" {
		log.WarnContext(ctx, "escalation: no SME team mapped for this Special Ops team; no SME page")
		e.writeSMENote(ctx, t, pr, nil, "NO_SME_TEAM",
			fmt.Sprintf("no SME team mapped for Special Ops team %s", orNone(pr.TeamKey)))
		return nil
	}
	log = log.With("smeTeam", team)

	// An alert an assignment has already answered -- a replay of one whose
	// page was closed, or one raised before an assignment handled first --
	// pages nobody.
	closed, err := e.store.SMEClosedThrough(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: read closed SME pages for %s: %w", incidentID, err)
	}
	if !closed.IsZero() && !pr.At.After(closed) {
		log.InfoContext(ctx, "escalation: ignored, SME page already answered by an assignment")
		return nil
	}

	opened, err := e.store.OpenSMEPage(ctx, incidentID, team, pr.At, smePageTTL)
	if err != nil {
		e.releaseSMEPage(ctx, incidentID, team)
		return fmt.Errorf("escalation: open SME page for %s: %w", incidentID, err)
	}
	if !opened {
		log.InfoContext(ctx, "escalation: ignored, SME page already open")
		return nil
	}
	// Read again now the page is open: an assignment handled between the
	// first read and the open either saw the open page and closed it, or
	// recorded its time before this read.
	if closed, err = e.store.SMEClosedThrough(ctx, incidentID); err != nil {
		e.releaseSMEPage(ctx, incidentID, team)
		return fmt.Errorf("escalation: read closed SME pages for %s: %w", incidentID, err)
	}
	if !closed.IsZero() && !pr.At.After(closed) {
		e.releaseSMEPage(ctx, incidentID, team)
		log.InfoContext(ctx, "escalation: ignored, an engineer was assigned after this alert was raised")
		return nil
	}

	started, err := e.startSMELadder(ctx, t, pr)
	if err != nil || !started {
		// Nobody will be reached, so nothing is open: a retry, or the next
		// alert, tries again.
		e.releaseSMEPage(ctx, incidentID, team)
	}
	if err == nil {
		e.postSMEHandoff(ctx, pr, team, started)
	}
	return err
}

// postSMEHandoff tells the incident's SRE team space, once per press, that
// the incident went to an SME team (sre.smeHandoffChat). Best effort: a
// failure is logged, never retried -- the SME page matters, this message
// does not hold it up.
func (e *Engine) postSMEHandoff(ctx context.Context, pr specialOpsPress, team string, started bool) {
	// The incident card's spaces first, without sre.smeHandoffChat, so the message replies under it; else the SRE space when that is on.
	targets := e.incidentRooms(Trigger{IncidentID: pr.IncidentID, Team: pr.PreviousGroup})
	if len(targets) > 0 && e.cfg.SME.Channel == ChannelLog {
		// A log (dry run) SME ladder pages nobody, so the incident space is not told it does.
		return
	}
	if len(targets) == 0 {
		if !e.cfg.Ladder.SMEHandoffChat {
			return
		}
		target, ok := e.sreRoomFor(pr.PreviousGroup)
		if !ok {
			return
		}
		targets = []chatRoom{target}
	}
	for _, target := range targets {
		e.postSMEHandoffTo(ctx, target, pr, team, started)
	}
}

// postSMEHandoffTo posts the SME handoff message to one space.
func (e *Engine) postSMEHandoffTo(ctx context.Context, target chatRoom, pr specialOpsPress, team string, started bool) {
	if !target.chat.HasAudienceSpace(target.room) {
		slog.WarnContext(ctx, "escalation: no Google Chat space for the SME handoff message", "incidentId", pr.IncidentID, "audience", target.room)
		return
	}
	// The SME team paged, named as the SME cards and closing message name it.
	smeTeam := smeTeamLabel(team)
	err := target.chat.SendEscalationHandoff(ctx, notifications.EscalationHandoff{
		Audience:    target.room,
		SMETeam:     smeTeam,
		By:          pr.ChangedBy,
		Paging:      started,
		IncidentRef: pr.Number,
		Title:       pr.Title,
		Priority:    pr.Priority,
		PortalURL:   e.links.IncidentLink(pr.IncidentID),
		PortalLabel: "View incident",
		ThreadKey:   notifications.IncidentThreadKey(pr.IncidentID),
		// Always an SRE incident, so a reply under its "Incident Reported" card wherever the ladder posted one.
		Reply: target.reply || e.threadReplies(Trigger{Team: pr.PreviousGroup, Routing: RoutingContext{Ladder: LadderSRE}}),
	})
	if err != nil {
		slog.ErrorContext(ctx, "escalation: the SME handoff message was not posted", "incidentId", pr.IncidentID, "audience", target.room, "err", err)
		return
	}
	slog.InfoContext(ctx, "escalation: SME handoff message posted", "incidentId", pr.IncidentID, "audience", target.room, "smeTeam", team)
}

// smeLadderKey is the store key of the incident's SME ladder for one team.
// Not the incident id -- several SME teams can run on one incident, beside
// its SRE chain -- and with no '|', which ends a wake member's key.
func smeLadderKey(incidentID, team string) string { return incidentID + ":sme:" + team }

// stopForHandoff stops the incident's running SRE chain, writing its summary
// the way any other cancellation does.
func (e *Engine) stopForHandoff(ctx context.Context, incidentID string) error {
	st, found, err := e.store.Get(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: load ladder for %s: %w", incidentID, err)
	}
	if !found {
		return nil
	}
	return e.stopLadder(ctx, incidentID, st, cancelEscalatedToSME)
}

// smeTrigger is the one-call plan's trigger for an alert.
func smeTrigger(pr specialOpsPress, team string) Trigger {
	label := pr.TeamLabel
	if label == "" {
		label = pr.TeamKey
	}
	return Trigger{
		IncidentID: pr.IncidentID,
		Number:     pr.Number,
		Title:      pr.Title,
		Team:       label,
		Kind:       TriggerSpecialOps,
		At:         pr.At,
		Priority:   pr.Priority,
		// The SME space's incident card names these.
		Service:       pr.Service,
		State:         pr.State,
		EscalatedBy:   pr.ChangedBy,
		EscalatedFrom: pr.PreviousGroup,
		Routing: RoutingContext{
			Product:         pr.Product,
			AssignedCRETeam: pr.AssignmentGroup,
			Ladder:          LadderSME,
			SMETeam:         team,
			At:              pr.At,
		},
	}
}

// startSMELadder plans the SME ladder -- one call per rung, L1 to L3 of the
// team's current window, on sme.timing -- and schedules it on the SRE
// engine's ticker. It reports whether anybody will be called; an error means
// a retry may do better.
func (e *Engine) startSMELadder(ctx context.Context, t Trigger, p specialOpsPress) (bool, error) {
	sme := e.cfg.SME
	if len(e.smeNotifiersFor(t.Routing.SMETeam)) == 0 && e.cfg.CallSendingEnabled {
		// sme.channel names a channel with no client (logged at startup).
		slog.ErrorContext(ctx, "escalation: no notifier for the SME page's channel; nobody was contacted",
			"incidentId", t.IncidentID, "channel", string(sme.Channel))
		e.writeSMENote(ctx, t, p, nil, "NO_CHANNEL", string(sme.Channel))
		return false, nil
	}
	plan, err := BuildPlan(ctx, t, e.policies, e.resolver, sme.Channel)
	if err != nil {
		return false, fmt.Errorf("escalation: plan the SME ladder for %s: %w", t.IncidentID, err)
	}
	e.applySMESafety(&plan)
	for _, is := range plan.Issues {
		slog.WarnContext(ctx, "escalation: SME rung cannot be called",
			"incidentId", t.IncidentID, "smeTeam", t.Routing.SMETeam, "level", is.Level.String(), "reason", is.Reason)
	}
	if len(plan.Calls) == 0 {
		reason, detail := "NO_RECIPIENTS", fmt.Sprintf("nobody on duty for SME team %s at %s", t.Routing.SMETeam, istStamp(t.At))
		for _, is := range plan.Issues {
			if is.Reason == "RESOLVE_FAILED" {
				return false, fmt.Errorf("escalation: resolve the SME on duty for %s: %s", t.IncidentID, is.Detail)
			}
			if is.Reason == "NO_NUMBER" || is.Reason == "NUMBER_NOT_ALLOWED" {
				reason, detail = is.Reason, is.Detail
			}
		}
		slog.WarnContext(ctx, "escalation: nobody on the SME team's rota can be paged; no SME page",
			"incidentId", t.IncidentID, "smeTeam", t.Routing.SMETeam, "reason", reason)
		// The on call is optional: the SME space still hears of the escalation, its card saying nobody is rostered.
		e.postSMEIncidentCard(ctx, plan)
		e.writeSMENote(ctx, t, p, nil, reason, detail)
		return false, nil
	}

	key := smeLadderKey(t.IncidentID, t.Routing.SMETeam)
	st := LadderState{Plan: plan, Placed: make([]bool, len(plan.Calls))}
	created, err := e.store.Create(ctx, key, st)
	if err != nil {
		return false, fmt.Errorf("escalation: store the SME ladder for %s: %w", t.IncidentID, err)
	}
	if !created {
		// A ladder for this team is still stored (a retry after a crash):
		// make sure its calls are scheduled rather than planning another.
		if st, _, err = e.store.Get(ctx, key); err != nil {
			return false, fmt.Errorf("escalation: load the SME ladder for %s: %w", t.IncidentID, err)
		}
	} else if rooms := e.postSMEIncidentCard(ctx, st.Plan); len(rooms) > 0 {
		// Recorded before any rung is scheduled, so L1 posts only its reply; a room the card missed gets it from the first rung.
		e.recordIncidentCards(ctx, key, st.Plan.Trigger, rooms)
	}
	if _, err := e.seedPendingWakes(ctx, key, st); err != nil {
		return false, fmt.Errorf("escalation: schedule the SME ladder for %s: %w", t.IncidentID, err)
	}
	first := st.Plan.Calls[0].Recipient
	slog.InfoContext(ctx, "escalation: SME paged",
		"incidentId", t.IncidentID, "smeTeam", t.Routing.SMETeam, "recipientId", first.LogID(),
		"shift", first.ShiftCode, "channel", string(sme.Channel), "calls", len(st.Plan.Calls))
	e.writeSMENote(ctx, t, p, &first, "", "")
	return true, nil
}

// postSMEIncidentCard posts the "Incident Escalated" card to the SME team's spaces when the escalation is handled, whoever is on call, and returns the rooms it reached. Best effort: a failure is logged.
func (e *Engine) postSMEIncidentCard(ctx context.Context, plan Plan) []string {
	t := plan.Trigger
	if !e.cfg.CallSendingEnabled || !e.cfg.SME.Channel.Uses(ChannelChat) {
		return nil
	}
	var rooms []string
	for _, n := range e.cardRooms(plan) {
		room := n.roomFor()
		if n.reply || slices.Contains(rooms, room) {
			continue
		}
		if !n.chat.HasAudienceSpace(room) {
			slog.WarnContext(ctx, "escalation: no Google Chat space for the SME incident card", "incidentId", t.IncidentID, "audience", room)
			continue
		}
		link := ""
		if n.links != nil {
			link = n.links.IncidentLink(t.IncidentID)
		}
		if err := n.chat.SendSMEIncidentEscalatedAlert(ctx, room, smeIncidentCard(plan, link)); err != nil {
			// The status only: a Chat error response can echo the card's content.
			status := 0
			var upstream *apierror.Error
			if errors.As(err, &upstream) {
				status = upstream.StatusCode
			}
			slog.ErrorContext(ctx, "escalation: the SME incident card was not posted", "incidentId", t.IncidentID, "audience", room, "statusCode", status)
			continue
		}
		slog.InfoContext(ctx, "escalation: SME incident card posted", "incidentId", t.IncidentID, "audience", room, "smeTeam", t.Routing.SMETeam)
		rooms = append(rooms, room)
	}
	return rooms
}

// applySMESafety drops the calls sme.safety.allowedNumbers does not allow,
// recording each as NUMBER_NOT_ALLOWED; a call channel also drops a recipient
// with no number (NO_NUMBER). A chat card needs no number.
func (e *Engine) applySMESafety(plan *Plan) {
	sme := e.cfg.SME
	if !sme.Channel.Uses(ChannelCall) {
		return
	}
	kept := plan.Calls[:0]
	for _, c := range plan.Calls {
		switch {
		case c.Recipient.Phone != "" && !sme.Dialable(c.Recipient.Phone):
			plan.Issues = append(plan.Issues, PlanIssue{Level: c.Level, At: c.At, Reason: "NUMBER_NOT_ALLOWED", Detail: c.Recipient.Email})
			if sme.Channel != ChannelCall {
				c.Recipient.Phone = ""
				kept = append(kept, c)
			}
		case c.Recipient.Phone == "" && sme.Channel == ChannelCall:
			// BuildPlan has reported NO_NUMBER already.
		default:
			kept = append(kept, c)
		}
	}
	plan.Calls = kept
}

// releaseSMEPage drops a page key that reached nobody. Best-effort: a key left
// behind only ignores the next press for the same team until it expires.
func (e *Engine) releaseSMEPage(ctx context.Context, incidentID, team string) {
	if err := e.store.CloseSMEPage(ctx, incidentID, team); err != nil {
		slog.WarnContext(ctx, "escalation: could not release an SME page that reached nobody; "+
			"a press for this team is ignored until it expires",
			"incidentId", incidentID, "smeTeam", team, "err", err)
	}
}

// closeSMEPages answers the incident's SME pages raised at or before an
// assignment made at answeredAt: it closes those open now, stops their
// ladders (their summaries go onto the incident), and records the time, so
// one still on its way pages nobody.
func (e *Engine) closeSMEPages(ctx context.Context, incidentID string, answeredAt time.Time) error {
	if e.cfg.Kind != LadderSRE || !e.cfg.SME.Enabled {
		return nil
	}
	teams, err := e.store.CloseSMEPages(ctx, incidentID, answeredAt)
	if err != nil {
		return fmt.Errorf("escalation: close SME pages for %s: %w", incidentID, err)
	}
	var errs []error
	for _, team := range teams {
		key := smeLadderKey(incidentID, team)
		st, found, err := e.store.Get(ctx, key)
		if err != nil {
			errs = append(errs, fmt.Errorf("escalation: load SME ladder %s: %w", key, err))
			continue
		}
		if found {
			errs = append(errs, e.stopLadder(ctx, key, st, cancelAssigned))
		}
	}
	if len(teams) > 0 {
		slog.InfoContext(ctx, "escalation: engineer assigned; SME pages closed", "incidentId", incidentID, "closed", len(teams))
	}
	return errors.Join(errs...)
}

// writeSMENote records the press on the incident: who was paged for which
// SME team and window, or why nobody was. who is nil when nobody was found;
// failure is empty when the page went out. Never fails the record, as
// writeNote.
func (e *Engine) writeSMENote(ctx context.Context, t Trigger, p specialOpsPress, who *Recipient, failure, detail string) {
	const stamp = "2006-01-02 15:04:05"
	var b strings.Builder
	b.WriteString("Execution Summary Of the Special Ops (SME) Page\n\n")
	b.WriteString(fmt.Sprintf("Escalated to Special Ops: %s by %s\n", t.At.Format(stamp), orNone(p.ChangedBy)))
	b.WriteString(fmt.Sprintf("Special Ops team: %s; SME team: %s; previous assignment group: %s\n\n",
		orNone(t.Team), orNone(t.Routing.SMETeam), orNone(p.PreviousGroup)))
	switch {
	case who == nil:
		b.WriteString(fmt.Sprintf("[%s][SME][ERROR][%s][%s]", t.At.Format(stamp), failure, detail))
	case failure != "":
		b.WriteString(fmt.Sprintf("[%s][SME][ERROR][%s][%s][shift %s]", t.At.Format(stamp), failure, who.Email, orNone(who.ShiftCode)))
		if detail != "" {
			b.WriteString(fmt.Sprintf("[%s]", detail))
		}
	default:
		b.WriteString(fmt.Sprintf("[%s][SME][OK][Page via %s][%s][shift %s]",
			t.At.Format(stamp), e.cfg.SME.Channel, who.Email, orNone(who.ShiftCode)))
	}
	note := b.String()
	if e.notes == nil {
		slog.InfoContext(ctx, "escalation: no incident-notes client configured; SME page summary not written back",
			"incidentId", t.IncidentID)
		return
	}
	if err := e.notes.AppendWorkNote(ctx, t.IncidentID, note); err != nil {
		slog.ErrorContext(ctx, "escalation: could not write the SME page summary; it is lost for this incident",
			"incidentId", t.IncidentID, "error", err)
	}
}
