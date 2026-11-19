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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// IncidentAudiencePrefix namespaces INCIDENT_CHAT_SPACES entries so an assignment group never collides with a GOOGLE_CHAT_SPACES audience.
const IncidentAudiencePrefix = "incident/"

// IncidentAudience is the Chat audience key of an incident's assignment group.
func IncidentAudience(assignmentGroup string) string {
	return IncidentAudiencePrefix + strings.ToLower(strings.TrimSpace(assignmentGroup))
}

// IncidentFallbackAudience is where an incident's card and paging replies go
// when its assignment group has no space of its own: the SRE ladder's shared
// room, registered by SetIncidentRouteSpaces. "#" cannot start a group the
// mapping produces, so no assignment group can take this key.
const IncidentFallbackAudience = IncidentAudiencePrefix + "#unmapped"

// IncidentRoutes is the group fan-out taken from the Alert Settings mapping:
// a default group (an incident with no SRE team of its own) -> the SRE teams
// whose rooms take such incidents (sre.teamlessChats). It is shared by the
// dispatcher's card and the paging engine's replies so both pick the same
// rooms, and set once the paging configuration is loaded. The zero value and
// a nil pointer hold nothing. Safe for concurrent use.
type IncidentRoutes struct {
	mu     sync.RWMutex
	fanout map[string][]string
	// teamless is the sre.teamlessChats team keys with a room: where an S0
	// case whose account has no SRE team gets its card (TeamlessAudiences).
	teamless []string
	// knownSRETeam reports whether a group is an SRE team of its own on the
	// SRE ladder (paging.KnownSRETeam); nil until the paging configuration is
	// loaded, when every group counts as unknown.
	knownSRETeam func(group string) bool
}

// SetKnownSRETeam sets the SRE ladder's known-team check (paging.KnownSRETeam).
func (r *IncidentRoutes) SetKnownSRETeam(known func(group string) bool) {
	r.mu.Lock()
	r.knownSRETeam = known
	r.mu.Unlock()
}

// CaseSREAudiences is where an S0 case's card, and its SRE paging replies,
// go: the rooms of its account SRE group (IncidentAudiences) when that group
// is an SRE team of its own, else the teamless rooms (TeamlessAudiences) --
// a case with no SRE team, or one the SRE ladder does not know and so pages
// like a teamless case. The dispatcher and the paging engine both call it, so
// the card, the replies and the calls agree.
func CaseSREAudiences(routes *IncidentRoutes, env map[string][]string, sreGroup string, has func(string) bool) []string {
	if strings.TrimSpace(sreGroup) != "" && routes != nil {
		routes.mu.RLock()
		known := routes.knownSRETeam
		routes.mu.RUnlock()
		if known != nil && known(sreGroup) {
			audiences, _ := IncidentAudiences(routes, env, sreGroup, has)
			return audiences
		}
	}
	return TeamlessAudiences(routes, has)
}

// SetTeamless replaces the teamless rooms' team keys.
func (r *IncidentRoutes) SetTeamless(keys []string) {
	r.mu.Lock()
	r.teamless = slices.Clone(keys)
	r.mu.Unlock()
}

// TeamlessAudiences is the audiences, each with a space, of the teamless
// rooms: where an S0 case whose account has no SRE team gets its card, and
// its SRE paging replies under it -- the rooms its rung cards always went
// to. Else IncidentFallbackAudience when has knows it, else none. The
// dispatcher and the paging engine both use it, so the card and the replies
// pick the same rooms.
func TeamlessAudiences(routes *IncidentRoutes, has func(string) bool) []string {
	var keys []string
	if routes != nil {
		routes.mu.RLock()
		keys = slices.Clone(routes.teamless)
		routes.mu.RUnlock()
	}
	var audiences []string
	for _, k := range keys {
		if audience := IncidentAudience(k); has(audience) && !slices.Contains(audiences, audience) {
			audiences = append(audiences, audience)
		}
	}
	if len(audiences) == 0 && has(IncidentFallbackAudience) {
		return []string{IncidentFallbackAudience}
	}
	return audiences
}

// Set replaces the routed fan-out (assignment group -> groups or team keys).
func (r *IncidentRoutes) Set(fanout map[string][]string) {
	m := make(map[string][]string, len(fanout))
	for group, targets := range fanout {
		if key := strings.ToLower(strings.TrimSpace(group)); key != "" && len(targets) > 0 {
			m[key] = targets
		}
	}
	r.mu.Lock()
	r.fanout = m
	r.mu.Unlock()
}

// Groups is the groups whose spaces get a group's incident messages:
// INCIDENT_CHAT_FANOUT's list when it names the group (env wins), else the
// mapping's, else the group itself.
func (r *IncidentRoutes) Groups(env map[string][]string, group string) []string {
	key := strings.ToLower(strings.TrimSpace(group))
	if key == "" {
		return nil
	}
	if groups := IncidentGroups(env, group); len(groups) != 1 || strings.ToLower(strings.TrimSpace(groups[0])) != key {
		return groups
	}
	if r != nil {
		r.mu.RLock()
		targets := r.fanout[key]
		r.mu.RUnlock()
		if len(targets) > 0 {
			return targets
		}
	}
	return []string{group}
}

// IncidentAudiences is the incident audiences, each with a space, that a
// group's card and paging replies go to: the audiences of routes.Groups that
// has knows, else IncidentFallbackAudience when has knows it (fallback is
// then true), else none. The dispatcher and the paging engine both use it,
// so the card and the replies under it always pick the same rooms.
func IncidentAudiences(routes *IncidentRoutes, env map[string][]string, group string, has func(string) bool) (audiences []string, fallback bool) {
	if strings.TrimSpace(group) == "" {
		return nil, false
	}
	for _, target := range routes.Groups(env, group) {
		audience := IncidentAudience(target)
		if has(audience) && !slices.Contains(audiences, audience) {
			audiences = append(audiences, audience)
		}
	}
	if len(audiences) == 0 && has(IncidentFallbackAudience) {
		return []string{IncidentFallbackAudience}, true
	}
	return audiences, false
}

// IncidentGroups is the assignment groups whose INCIDENT_CHAT_SPACES spaces get a group's incident messages: its INCIDENT_CHAT_FANOUT list, else the group itself.
func IncidentGroups(fanout map[string][]string, group string) []string {
	key := strings.ToLower(strings.TrimSpace(group))
	if key == "" {
		return nil
	}
	for g, targets := range fanout {
		if strings.ToLower(strings.TrimSpace(g)) == key && len(targets) > 0 {
			return targets
		}
	}
	return []string{group}
}

// IncidentCreatedAlert is SendIncidentCreatedAlert's card content.
type IncidentCreatedAlert struct {
	// IncidentID keys the card's thread (IncidentThreadKey).
	IncidentID       string
	Number           string
	Team             string
	Priority         string
	ShortDescription string
	Service          string
	Environment      string
	Category         string
	State            string
	IncidentLink     string
	// Case renders the card for an S0 customer case on the SRE ladder:
	// "Case Reported", "View Case", "Case Details" and the case severity. An
	// incident leaves it false and gets the card exactly as before.
	Case bool
	// Project is the case's project, shown in Case Details in place of State
	// (a case card carries no state); left out when empty. Case only.
	Project string
	// ThreadKey is the card's thread; empty is IncidentThreadKey(IncidentID).
	// An S0 case sets CaseS0ThreadKey, so each time it becomes S0 it starts a
	// thread of its own.
	ThreadKey string
}

// casePriorityLabels renders a case severity on the card when Case is set.
var casePriorityLabels = map[string]struct{ word, label string }{
	"CATASTROPHIC": {"Catastrophic", "S0 - Catastrophic"},
	"S0":           {"Catastrophic", "S0 - Catastrophic"},
}

// IncidentAssignedAlert is SendIncidentAssignedAlert's card content.
type IncidentAssignedAlert struct {
	// IncidentID keys the card's thread (IncidentThreadKey).
	IncidentID   string
	Number       string
	AssigneeName string
	UpdatedBy    string
	IncidentLink string
}

// incidentPriorityLabels renders an incident priority the way ServiceNow's incident card does.
var incidentPriorityLabels = map[string]struct{ word, label string }{
	"CRITICAL": {"Critical", "P1 - Critical"},
	"HIGH":     {"High", "P2 - High"},
	"MODERATE": {"Moderate", "P3 - Moderate"},
	"LOW":      {"Low", "P4 - Low"},
	"PLANNING": {"Planning", "P5 - Planning"},
	"P1":       {"Critical", "P1 - Critical"},
	"P2":       {"High", "P2 - High"},
	"P3":       {"Moderate", "P3 - Moderate"},
	"P4":       {"Low", "P4 - Low"},
	"P5":       {"Planning", "P5 - Planning"},
}

// SendIncidentCreatedAlert posts ServiceNow's "<team> | <priority> Priority Incident Reported" card.
func (c *GoogleChatClient) SendIncidentCreatedAlert(ctx context.Context, audience string, a IncidentCreatedAlert) error {
	if a.IncidentID == "" {
		return fmt.Errorf("notifications: incidentId is required")
	}
	priority := incidentPriorityLabels[strings.ToUpper(strings.TrimSpace(a.Priority))]
	threadKey := a.ThreadKey
	if threadKey == "" {
		threadKey = IncidentThreadKey(a.IncidentID)
	}
	noun := "Incident"
	if a.Case {
		priority, noun = casePriorityLabels[strings.ToUpper(strings.TrimSpace(a.Priority))], "Case"
	}
	title := "Priority " + noun + " Reported"
	if priority.word != "" {
		title = priority.word + " " + title
	}
	if strings.TrimSpace(a.Team) != "" {
		title = strings.ToUpper(strings.TrimSpace(a.Team)) + " | " + title
	}
	subtitle := "#" + incidentRef(a.Number, a.IncidentID)
	for _, part := range []string{a.Service, a.Environment} {
		if strings.TrimSpace(part) != "" {
			subtitle += " | " + strings.TrimSpace(part)
		}
	}

	summary := []chatCardWidget{{TextParagraph: &chatTextParagraph{
		Text: caseAlertLine(`<b>Short Description:</b><br>%s`, orEmDash(a.ShortDescription)),
	}}}
	if a.IncidentLink != "" {
		summary = append(summary, chatCardWidget{ButtonList: &chatButtonList{
			Buttons: []chatButton{{Text: "View " + noun, OnClick: chatOnClick{OpenLink: chatOpenLink{URL: a.IncidentLink}}}},
		}})
	}
	details := []string{
		caseAlertLine(`<b>Category:</b> %s`, orEmDash(strings.ToLower(a.Category))),
		caseAlertLine(`<b>Priority:</b> %s`, orEmDash(priority.label)),
	}
	switch {
	case !a.Case:
		details = append(details, caseAlertLine(`<b>State:</b> %s`, orEmDash(humanizeEnumLabel(a.State))))
	case strings.TrimSpace(a.Project) != "":
		details = append(details, caseAlertLine(`<b>Project:</b> %s`, strings.TrimSpace(a.Project)))
	}
	sections := []chatCardSection{
		{Widgets: summary},
		{
			Header:      noun + " Details",
			Collapsible: true,
			Widgets:     []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: strings.Join(details, "<br>")}}},
		},
	}

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{{CardID: "incident-created-alert", Card: chatCard{
			Header:   &chatCardHeader{Title: title, Subtitle: subtitle},
			Sections: sections,
		}}},
		Thread: &chatThread{ThreadKey: threadKey},
	}
	return c.sendCardToAudience(ctx, audience, a.Number, a.IncidentID, msg)
}

// SMEIncidentEscalatedAlert is SendSMEIncidentEscalatedAlert's card content.
type SMEIncidentEscalatedAlert struct {
	// IncidentID keys the card's thread (IncidentThreadKey).
	IncidentID       string
	Number           string
	SMETeam          string
	Priority         string
	ShortDescription string
	Service          string
	State            string
	// EscalatedBy is who escalated it; EscalatedFrom the SRE team it left.
	EscalatedBy   string
	EscalatedFrom string
	// OnCall is the SME the first rung pages.
	OnCall string
	// OnCallRung and OnCallAt are the tier and IST time of that first rung when it is not L1, so the card says when anyone is called.
	OnCallRung   string
	OnCallAt     string
	EscalatedAt  string
	IncidentLink string
}

// SendSMEIncidentEscalatedAlert posts the "<SME team> | <priority> Priority Incident Escalated" card an SME ladder's rung replies sit under.
func (c *GoogleChatClient) SendSMEIncidentEscalatedAlert(ctx context.Context, audience string, a SMEIncidentEscalatedAlert) error {
	if a.IncidentID == "" {
		return fmt.Errorf("notifications: incidentId is required")
	}
	return c.sendCardToAudience(ctx, audience, a.Number, a.IncidentID, buildSMEIncidentEscalatedCard(a))
}

// buildSMEIncidentEscalatedCard lays the SME card out like the "Incident Reported" card, adding who escalated it and who is on call.
func buildSMEIncidentEscalatedCard(a SMEIncidentEscalatedAlert) chatCardMessage {
	priority := incidentPriorityLabels[strings.ToUpper(strings.TrimSpace(a.Priority))]
	title := "Priority Incident Escalated"
	if priority.word != "" {
		title = priority.word + " " + title
	}
	if team := strings.TrimSpace(a.SMETeam); team != "" {
		title = strings.ToUpper(team) + " | " + title
	}
	subtitle := "#" + incidentRef(a.Number, a.IncidentID)
	if service := strings.TrimSpace(a.Service); service != "" {
		subtitle += " | " + service
	}

	summary := []chatCardWidget{{TextParagraph: &chatTextParagraph{
		Text: caseAlertLine(`<b>Short Description:</b><br>%s`, orEmDash(a.ShortDescription)),
	}}}
	if by := strings.TrimSpace(a.EscalatedBy); by != "" {
		text := caseAlertLine("<b>%s</b>", by)
		if from := strings.TrimSpace(a.EscalatedFrom); from != "" {
			text += caseAlertLine(" from %s", from)
		}
		summary = append(summary, chatCardWidget{DecoratedText: &chatDecoratedText{TopLabel: "Escalated by", Text: text, WrapText: true}})
	}
	onCall := `<font color="#B3261E">Nobody rostered</font>`
	if name := strings.TrimSpace(a.OnCall); name != "" {
		onCall = caseAlertLine("<b>%s</b>", name)
		if rung := strings.TrimSpace(a.OnCallRung); rung != "" {
			// L1 is empty: name who is called first and when, so the room does not read it as nobody being paged.
			onCall = caseAlertLine(`<font color="#B3261E">Nobody rostered before %s</font><br>`, rung) + caseAlertLine("<b>%s</b> (%s)", name, rung)
			if at := strings.TrimSpace(a.OnCallAt); at != "" {
				onCall += caseAlertLine(", called at %s", at)
			}
		}
	}
	summary = append(summary, chatCardWidget{DecoratedText: &chatDecoratedText{TopLabel: "SME on call", Text: onCall, WrapText: true}})
	if a.IncidentLink != "" {
		summary = append(summary, chatCardWidget{ButtonList: &chatButtonList{
			Buttons: []chatButton{{Text: "View Incident", OnClick: chatOnClick{OpenLink: chatOpenLink{URL: a.IncidentLink}}}},
		}})
	}
	details := []string{
		caseAlertLine(`<b>Priority:</b> %s`, orEmDash(priority.label)),
		caseAlertLine(`<b>State:</b> %s`, orEmDash(humanizeEnumLabel(a.State))),
	}
	if from := strings.TrimSpace(a.EscalatedFrom); from != "" {
		details = append(details, caseAlertLine(`<b>Escalated from:</b> %s`, from))
	}
	if at := strings.TrimSpace(a.EscalatedAt); at != "" {
		details = append(details, caseAlertLine(`<b>Escalated at:</b> %s`, at))
	}

	return chatCardMessage{
		CardsV2: []chatCardWrapper{{CardID: "incident-sme-escalated-alert", Card: chatCard{
			Header: &chatCardHeader{Title: title, Subtitle: subtitle},
			Sections: []chatCardSection{
				{Widgets: summary},
				{
					Header:      "Incident Details",
					Collapsible: true,
					Widgets:     []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: strings.Join(details, "<br>")}}},
				},
			},
		}}},
		Thread: &chatThread{ThreadKey: IncidentThreadKey(a.IncidentID)},
	}
}

// SendIncidentAssignedAlert posts ServiceNow's "Incident Acknowledged." reply under the incident's card.
func (c *GoogleChatClient) SendIncidentAssignedAlert(ctx context.Context, audience string, a IncidentAssignedAlert) error {
	if a.IncidentID == "" {
		return fmt.Errorf("notifications: incidentId is required")
	}
	lines := []string{caseAlertLine(`Incident acknowledged by %s.`, orEmDash(a.AssigneeName))}
	if a.UpdatedBy != "" {
		lines = append(lines, caseAlertLine(`Incident updated by %s.`, a.UpdatedBy))
	}
	// The link keeps the reply usable on its own in a space that never got the card (the group changed since creation).
	if a.IncidentLink != "" {
		lines = append(lines, caseAlertLine(`<a href="%s">View incident</a>`, a.IncidentLink))
	}
	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{{CardID: "incident-assigned-alert", Card: chatCard{
			Header:   &chatCardHeader{Title: "Incident Acknowledged.", Subtitle: incidentRef(a.Number, a.IncidentID)},
			Sections: []chatCardSection{{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: strings.Join(lines, "<br>")}}}}},
		}}},
		Thread: &chatThread{ThreadKey: IncidentThreadKey(a.IncidentID)},
	}
	return c.sendCardToAudience(ctx, audience, a.Number, a.IncidentID, msg)
}

// incidentRef is the incident's number, or its id when an older publisher sent no number.
func incidentRef(number, incidentID string) string {
	if strings.TrimSpace(number) != "" {
		return number
	}
	return incidentID
}

// IncidentThreadKey keeps an incident's created card and its replies, the paging rungs included, in one Chat thread.
func IncidentThreadKey(incidentID string) string {
	return "incident-" + incidentID
}

// CaseS0ThreadKey is the thread of one S0 episode of a case: the dispatcher's
// SRE room card and the SRE ladder's messages for the record that made the
// case S0 (created at S0, or raised to it). Both read that same record, so
// both derive the same key from its bytes without sharing any state; a later
// raise to S0 is a new record and so a new thread, and a redelivered record
// keeps its thread.
func CaseS0ThreadKey(caseID string, record []byte) string {
	sum := sha256.Sum256(record)
	return IncidentThreadKey(caseID) + "-" + hex.EncodeToString(sum[:6])
}

// humanizeEnumLabel turns an UPPER_SNAKE value such as IN_PROGRESS into "In Progress".
func humanizeEnumLabel(v string) string {
	words := strings.Fields(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(v)), "_", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
