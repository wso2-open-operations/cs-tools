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

package notifications

import (
	"context"
	"fmt"
	"strings"
)

// EscalationAlert is one rung of an incident call-escalation ladder, delivered
// to a Google Chat space instead of (or alongside) a phone call.
//
// READ THIS BEFORE TREATING CHAT AS THE PAGING CHANNEL. The ladder exists to
// wake someone up. A chat message does not: the specification already sends
// one to the Incident Monitor space the moment an incident is raised, and the
// ladder exists precisely because that was not enough to get an unattended
// incident attended to at three in the morning. Delivering the ladder itself
// over chat changes it from a pager into a louder version of the notification
// that already failed.
//
// It is still worth having, for two reasons. It is the only channel that can
// be exercised end to end without a telephony account, which is what makes the
// flow demonstrable today. And it gives the room visibility of an escalation
// in progress, which the calls themselves do not: a call reaches one person,
// while the card shows everyone which rung an incident has reached and who is
// being asked to pick it up.
type EscalationAlert struct {
	// Product routes the card to a space, the same way every other alert in
	// this package routes.
	Product string
	// Rung is the level being contacted, e.g. "LEVEL_2".
	Rung string
	// RungRole names who that rung is, e.g. "ABT team leads".
	RungRole string
	// Attempt is which notification of this rung it is, 1-based.
	//
	// Not rendered. The escalation engine posts one card per rung, so every
	// card it builds is attempt 1, and a line reporting that would say
	// nothing. Kept on the struct because it is part of what a rung is, and a
	// caller outside that flow may have a reason to show it.
	Attempt int
	// Priority is the incident's priority, which sets the whole clock.
	Priority string
	// IncidentRef is the human-readable reference, e.g. "INC0012345".
	IncidentRef string
	// Title is the incident subject, for recognising it at a glance.
	Title string
	// RecipientName is who this rung resolved to. Deliberately a name and
	// never a phone number: a space is a room, and the ladder's own logs
	// already keep numbers out of places people read.
	RecipientName string
	// Instruction is what stops the ladder, which differs by trigger.
	Instruction string
	// Rule is the section 5.0 row that selected these recipients, so the room
	// can see which path an alert took.
	Rule string
	// PortalURL opens the incident.
	PortalURL string
	// Elapsed is how long the incident has been unattended, e.g. "18m".
	// Empty means the ladder has only just started.
	Elapsed string
	// NextRung and NextIn say where this goes if nobody picks it up, e.g.
	// "LEVEL_3" and "7m". Both empty on the final rung, which has nowhere
	// left to climb.
	//
	// These exist because a space flattens time. Cards arrive minutes apart
	// but sit in a list, and a reader scanning it cannot see from the cards
	// alone whether an incident is escalating quickly or has been quietly
	// stuck; the timestamps are there but easy to miss and hard to compare.
	// Saying how long it has gone unattended and when it climbs next puts the
	// urgency in the card rather than leaving it to be inferred.
	NextRung string
	NextIn   string
	// ThreadKey groups every rung of one incident's ladder into a single
	// conversation, so the space shows an escalation unfolding in one place
	// instead of scattering it through everything else being posted.
	ThreadKey string
}

// SendEscalationAlert posts one rung of the ladder to the product's space.
//
// The card is deliberately unlike this package's case.* cards: those announce
// something that happened, while this one is asking the room to act, so the
// rung and the instruction carry the weight and everything else is context.
func (c *GoogleChatClient) SendEscalationAlert(ctx context.Context, a EscalationAlert) error {
	if a.Rung == "" {
		return fmt.Errorf("notifications: rung is required")
	}
	if a.IncidentRef == "" {
		return fmt.Errorf("notifications: incidentRef is required")
	}
	return c.sendCard(ctx, a.Product, buildEscalationCard(a))
}

// buildEscalationCard assembles the card, split from the send so its exact
// shape can be asserted on without a webhook.
func buildEscalationCard(a EscalationAlert) chatCardMessage {

	header := fmt.Sprintf("%s %s %s", escalationSeverityMark(a.Priority), rungVerb(a.Rung), a.Rung)
	subtitle := a.IncidentRef
	if a.Title != "" {
		subtitle = a.IncidentRef + " - " + a.Title
	}

	var body strings.Builder
	body.WriteString(caseAlertLine("<b>%s</b>", a.RungRole))
	if a.RecipientName != "" {
		body.WriteString("<br>")
		body.WriteString(caseAlertLine("Calling %s", a.RecipientName))
	}
	body.WriteString("<br>")
	body.WriteString(caseAlertLine(`<font color="#5F6368">Priority %s</font>`, a.Priority))
	body.WriteString(caseAlertLine(`<font color="#5F6368"> - unattended %s</font>`, elapsedOrNew(a.Elapsed)))
	if a.Rule != "" {
		body.WriteString(caseAlertLine(`<font color="#5F6368"> - path %s</font>`, a.Rule))
	}
	if a.NextRung != "" && a.NextIn != "" {
		body.WriteString("<br>")
		body.WriteString(caseAlertLine(`<font color="#B3261E">Escalates to %s`, a.NextRung))
		body.WriteString(caseAlertLine(` in %s unless acknowledged</font>`, a.NextIn))
	} else if a.NextRung == "" {
		body.WriteString("<br>")
		body.WriteString(`<font color="#B3261E">This is the final rung. Nothing escalates past it.</font>`)
	}
	if a.Instruction != "" {
		body.WriteString("<br><br>")
		body.WriteString(caseAlertLine("<b>%s</b>", a.Instruction))
	}
	if a.PortalURL != "" {
		body.WriteString("<br>")
		body.WriteString(fmt.Sprintf(`<a href="%s">View incident</a>`, a.PortalURL))
	}

	msg := chatCardMessage{
		Thread: threadFor(a.ThreadKey),
		CardsV2: []chatCardWrapper{{
			CardID: "incident-escalation",
			Card: chatCard{
				Header: &chatCardHeader{Title: header, Subtitle: subtitle},
				Sections: []chatCardSection{{
					Widgets: []chatCardWidget{
						{TextParagraph: &chatTextParagraph{Text: body.String()}},
					},
				}},
			},
		}},
	}
	return msg
}

// elapsedOrNew renders how long an incident has gone unattended, naming the
// zero case rather than leaving a card that says "unattended" and stops.
func elapsedOrNew(elapsed string) string {
	if elapsed == "" {
		return "just raised"
	}
	return elapsed
}

// threadFor groups a ladder's rungs, or leaves the message standalone when no
// key is given.
func threadFor(key string) *chatThread {
	if key == "" {
		return nil
	}
	return &chatThread{ThreadKey: key}
}

// rungVerb keeps the header honest about what LEVEL_0 is.
//
// Section 3.0 calls it the notification level and says in as many words that
// it is "represented as the initial escalation level (but not an actual
// escalation level)" - it exists to tell the rotation an incident has arrived,
// before any escalation has happened. A card announcing "escalated to LEVEL_0"
// contradicts the document it implements, and tells a reader the incident has
// already been through a rung it has not.
func rungVerb(rung string) string {
	if rung == "LEVEL_0" {
		return "notifying"
	}
	return "escalated to"
}

// escalationSeverityMark gives the header a glyph that reads at a glance in a
// busy space. Only the two fastest priorities get the loud one: if every rung
// of every incident shouted, none of them would.
func escalationSeverityMark(priority string) string {
	switch strings.ToUpper(strings.TrimSpace(priority)) {
	case "P0", "P1", "CATASTROPHIC", "CRITICAL":
		return "\U0001F534" // red circle
	default:
		return "\U0001F7E0" // orange circle
	}
}
