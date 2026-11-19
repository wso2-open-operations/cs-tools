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

package slaengine

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func TestRegisterIncidentClocks_RegistersTheRunningClock(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	started := time.Date(2026, 11, 3, 7, 19, 53, 0, time.UTC)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {ID: "sla-row-1", Found: true, Stage: "IN_PROGRESS", StartedOn: &started, DurationSeconds: int64((4 * time.Hour).Seconds())},
	}}

	e.RegisterIncidentClocks(context.Background(), "inc-1", "CRITICAL", started, "INC0091536", "Router 5xx", "Artemis SRE Group", "Choreo")

	meta, found, _ := st.GetClock(context.Background(), "inc-1", ClockResolution)
	if !found {
		t.Fatal("resolution clock not registered")
	}
	if meta.SLAID != "sla-row-1" {
		t.Errorf("SLAID = %q, want entity-service's row id, so later checks ask about this exact clock", meta.SLAID)
	}
	if meta.CaseType != CaseTypeIncident || meta.CaseNumber != "INC0091536" || meta.Team != "Artemis SRE Group" || meta.Product != "Choreo" || meta.Priority != "CRITICAL" {
		t.Errorf("meta = %+v, want the incident's number, group, service and priority", meta)
	}
	if !meta.StartedAt.Equal(started) {
		t.Errorf("StartedAt = %v, want entity-service's start %v", meta.StartedAt, started)
	}
	for _, tier := range tierSequence {
		if _, ok := st.wake[wakeMember("inc-1", ClockResolution, tier)]; !ok {
			t.Errorf("no wake entry for tier %d", tier)
		}
	}
	for _, ct := range []string{ClockResponse, ClockWorkaround} {
		if _, found, _ := st.GetClock(context.Background(), "inc-1", ct); found {
			t.Errorf("clock %s registered, want only the clock entity-service has", ct)
		}
	}
}

// incident.priority_elevated carries no service: re-registering the same
// clock from it keeps what incident.created stored.
func TestRegisterIncidentClocks_ElevationKeepsTheStoredService(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	started := time.Date(2026, 11, 3, 7, 19, 53, 0, time.UTC)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, Stage: "IN_PROGRESS", StartedOn: &started, DurationSeconds: int64((4 * time.Hour).Seconds())},
	}}

	e.RegisterIncidentClocks(context.Background(), "inc-1", "HIGH", started, "INC0091536", "Router 5xx", "Artemis SRE Group", "Choreo")
	e.RegisterIncidentClocks(context.Background(), "inc-1", "CRITICAL", started, "INC0091536", "", "Artemis SRE Group", "")

	meta, _, _ := st.GetClock(context.Background(), "inc-1", ClockResolution)
	if meta.Product != "Choreo" || meta.CaseTitle != "Router 5xx" {
		t.Errorf("service/title = %q/%q, want the stored Choreo/Router 5xx kept", meta.Product, meta.CaseTitle)
	}
	if meta.Priority != "CRITICAL" {
		t.Errorf("Priority = %q, want the raised CRITICAL", meta.Priority)
	}
}

// A priority lowered before the clock was registered leaves entity-service's
// clock CANCELLED (possibly after breaching): scheduling it would alert.
func TestRegisterIncidentClocks_SkipsAClockThatAlreadyEnded(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	started := time.Now().Add(-6 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, Stage: "CANCELLED", HasBreached: true, StartedOn: &started, DurationSeconds: int64((4 * time.Hour).Seconds())},
	}}

	e.RegisterIncidentClocks(context.Background(), "inc-1", "PLANNING", started, "INC0091545", "", "Artemis SRE Group", "Choreo")

	if _, found, _ := st.GetClock(context.Background(), "inc-1", ClockResolution); found {
		t.Error("a CANCELLED clock was registered")
	}
	if len(st.wake) != 0 {
		t.Errorf("wake entries = %v, want none", st.wake)
	}
}

// sreTeamWiring wires e the way cmd/server does for incidents: an assignment
// group resolves to SRE team keys by teamsByGroup (paging.SRETeamsForGroup in
// production), each key in spaces has its team.chat_webhook_url space, and
// groupEmails is the team.key -> group_email table. It records the group each
// alert resolved.
func sreTeamWiring(e *Engine, chat *fakeChat, teamsByGroup map[string][]string, spaces []string, groupEmails map[string]string) *[]string {
	var resolved []string
	e.WithIncidentSLATeams(func(group string) []string {
		resolved = append(resolved, group)
		return teamsByGroup[group]
	})
	if chat.spaces == nil {
		chat.spaces = map[string]bool{}
	}
	for _, key := range spaces {
		chat.spaces[notifications.IncidentSLAAudience(key)] = true
	}
	e.SetTeamGroupEmails(groupEmails)
	return &resolved
}

func incidentRecipients(calls []fakeEmailCall) []string {
	var out []string
	for _, c := range calls {
		out = append(out, c.to...)
	}
	return out
}

// An incident's tier reaches its SRE team(s) as a case's reaches its CRE
// team: each team's own SLA space (team.chat_webhook_url) and group email
// (team.group_email). A default group is every teamless team. The account
// CRE team in GetClockState's TeamEmail/TeamLeadName is never used, there is
// no Americas copy even overnight, and never the case audiences.
func TestTick_IncidentBreach_GoesToItsSRETeamsSpacesAndEmails(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, pub)
	e.email = email
	e.now = func() time.Time { return time.Date(2026, time.October, 14, 17, 0, 0, 0, time.UTC) } // 22:30 IST, the Americas window
	resolved := sreTeamWiring(e, chat,
		map[string][]string{"Default SRE Group": {"apollo", "artemis"}},
		[]string{"apollo", "artemis"},
		map[string]string{"apollo": "apollo@example.com", "artemis": "artemis@example.com", slaBreachAmericasTeamKey: "americas@example.com"})
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {
			Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past,
			AssigneeName: "Melan Rashitha", AssigneeEmail: "melan@example.com",
			TeamEmail: "cre-team@example.com", TeamLeadName: "CRE Lead", OnboardingTeamEmail: "onboarding@example.com",
		},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseTitle: "Router 5xx", CaseType: CaseTypeIncident, Team: "Default SRE Group", Product: "Choreo", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}

	if !slices.Equal(*resolved, []string{"Default SRE Group"}) {
		t.Errorf("resolved groups = %v, want the incident's assignment group", *resolved)
	}
	if len(chat.calls) != 0 {
		t.Errorf("case chat calls = %+v, want none for an incident", chat.calls)
	}
	var audiences []string
	for _, c := range chat.incidentCalls {
		audiences = append(audiences, c.audience)
		a := c.alert
		if a.Number != "INC0091536" || a.Tier != "100" || a.ClockType != ClockResolution || a.AssignmentGroup != "Default SRE Group" ||
			a.Service != "Choreo" || a.IncidentLink != "https://csm.example.com/incidents/inc-1" {
			t.Errorf("incident alert = %+v, want the incident's fields and link", a)
		}
	}
	if want := []string{notifications.IncidentSLAAudience("apollo"), notifications.IncidentSLAAudience("artemis")}; !slices.Equal(audiences, want) {
		t.Errorf("incident SLA audiences = %v, want %v", audiences, want)
	}

	if len(email.calls) != 2 {
		t.Fatalf("email calls = %d, want 2 (assignee, SRE teams; no Americas copy)", len(email.calls))
	}
	for _, c := range email.calls {
		if !strings.Contains(c.subject, "INC0091536") {
			t.Errorf("subject = %q, want the incident number", c.subject)
		}
		if strings.Contains(c.htmlBody, "View Case") || strings.Contains(c.htmlBody, "Case ID") || strings.Contains(c.htmlBody, "CRE Lead") {
			t.Errorf("email body uses case wording or the CRE team lead")
		}
		if !strings.Contains(c.htmlBody, "https://csm.example.com/incidents/inc-1") {
			t.Errorf("email body has no incident link")
		}
	}
	if want := []string{"melan@example.com", "apollo@example.com", "artemis@example.com"}; !slices.Equal(incidentRecipients(email.calls), want) {
		t.Errorf("email recipients = %v, want %v", incidentRecipients(email.calls), want)
	}
	if pub.calls != 1 {
		t.Errorf("publish calls = %d, want 1", pub.calls)
	}
}

// A team with no SLA space gets no card; its group email still goes out.
func TestTick_IncidentBreach_NoSLASpace_StillEmails(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
	sreTeamWiring(e, chat, map[string][]string{"Artemis SRE Group": {"artemis"}}, nil, map[string]string{"artemis": "artemis@example.com"})
	past := time.Now().Add(-3 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, IsActive: true, Stage: "IN_PROGRESS", StartedOn: &past, AssigneeEmail: "melan@example.com"},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseType: CaseTypeIncident, Team: "Artemis SRE Group", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(chat.calls)+len(chat.incidentCalls) != 0 {
		t.Errorf("chat calls = %+v / %+v, want none", chat.calls, chat.incidentCalls)
	}
	if want := []string{"melan@example.com", "artemis@example.com"}; !slices.Equal(incidentRecipients(email.calls), want) {
		t.Errorf("email recipients = %v, want %v", incidentRecipients(email.calls), want)
	}
}

// A group that maps to no SRE team reaches the assignee only: never the
// account CRE team, never Incident Monitor.
func TestTick_IncidentBreach_NoSRETeam_AssigneeOnly(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
	sreTeamWiring(e, chat, nil, []string{"apollo"}, map[string]string{"apollo": "apollo@example.com"})
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past, AssigneeEmail: "melan@example.com", TeamEmail: "cre-team@example.com"},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseType: CaseTypeIncident, Team: "Choreo Runtime Group", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(chat.calls)+len(chat.incidentCalls) != 0 {
		t.Errorf("chat calls = %+v / %+v, want none", chat.calls, chat.incidentCalls)
	}
	if want := []string{"melan@example.com"}; !slices.Equal(incidentRecipients(email.calls), want) {
		t.Errorf("email recipients = %v, want the assignee only", incidentRecipients(email.calls))
	}
}

// An incident clock cancelled after it was registered (priority lowered,
// service or category changed, incident canceled) sends nothing at a later
// tier: the check asks about the clock's own row, which is now CANCELLED.
func TestTick_IncidentCancelledAfterRegistration_SendsNothing(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
	sreTeamWiring(e, chat, map[string][]string{"Artemis SRE Group": {"artemis"}}, []string{"artemis"}, map[string]string{"artemis": "artemis@example.com"})
	past := time.Now().Add(-4 * time.Hour)
	e.entity = &fakeEntityClockStatus{
		// The newest row for the incident is a different, active clock: only
		// the exact-row lookup can tell that this wake's clock has ended.
		states: map[string]clockState{
			"inc-1|" + ClockResolution: {ID: "sla-row-2", Found: true, IsActive: true, Stage: "IN_PROGRESS", StartedOn: &past},
		},
		statesBySLAID: map[string]clockState{
			"sla-row-1": {ID: "sla-row-1", Found: true, Stage: "CANCELLED", StartedOn: &past},
		},
	}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091532", CaseType: CaseTypeIncident, Team: "Artemis SRE Group", Priority: "HIGH", StartedAt: past, SLAID: "sla-row-1"}
	st.wake[wakeMember("inc-1", ClockResolution, 75)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(chat.incidentCalls)+len(chat.calls) != 0 || len(email.calls) != 0 {
		t.Errorf("chat = %+v / %+v, email = %+v; want nothing for a cancelled clock", chat.incidentCalls, chat.calls, email.calls)
	}
}

// SLA_EMAIL_SENDING_ENABLED=false holds incident alerts back too: no SLA
// Chat card and no email.
func TestTick_IncidentBreach_SendingDisabled_SendsNothing(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
	e.slaEmailSendingEnabled = false
	sreTeamWiring(e, chat, map[string][]string{"Artemis SRE Group": {"artemis"}}, []string{"artemis"}, map[string]string{"artemis": "artemis@example.com"})
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past, AssigneeEmail: "melan@example.com"},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseType: CaseTypeIncident, Team: "Artemis SRE Group", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(chat.incidentCalls)+len(chat.calls) != 0 || len(email.calls) != 0 {
		t.Errorf("chat = %+v / %+v, email = %+v; want nothing while sending is disabled", chat.incidentCalls, chat.calls, email.calls)
	}
}

// A Chat space that fails does not stop the other space or the emails.
func TestTick_IncidentBreach_ChatFailure_StillSendsTheRest(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{failFor: map[string]bool{notifications.IncidentSLAAudience("apollo"): true}}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
	sreTeamWiring(e, chat, map[string][]string{"Default SRE Group": {"apollo", "artemis"}}, []string{"apollo", "artemis"},
		map[string]string{"apollo": "apollo@example.com", "artemis": "artemis@example.com"})
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past, AssigneeEmail: "melan@example.com"},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseType: CaseTypeIncident, Team: "Default SRE Group", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(chat.incidentCalls) != 2 {
		t.Errorf("incident SLA chat attempts = %d, want both spaces tried", len(chat.incidentCalls))
	}
	if len(email.calls) != 2 {
		t.Errorf("email calls = %d, want the assignee and team emails despite the Chat failure", len(email.calls))
	}
}

// The onboarding team is a case concern: an incident on an onboarding
// project emails its assignee and SRE team only.
func TestTick_IncidentBreach_OnboardingProject_NoOnboardingTeamCopy(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
	sreTeamWiring(e, chat, map[string][]string{"Artemis SRE Group": {"artemis"}}, nil, map[string]string{"artemis": "artemis@example.com"})
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {
			Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past,
			AssigneeEmail: "melan@example.com", OnboardingTeamEmail: "onboarding@example.com",
		},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseType: CaseTypeIncident, Team: "Artemis SRE Group", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if want := []string{"melan@example.com", "artemis@example.com"}; !slices.Equal(incidentRecipients(email.calls), want) {
		t.Errorf("email recipients = %v, want %v (no onboarding team copy for an incident)", incidentRecipients(email.calls), want)
	}
}

// incident.priority_elevated may omit the group and number too: an empty one
// keeps the stored value, a non-empty one still replaces it.
func TestRegisterIncidentClocks_ElevationKeepsStoredGroupAndNumber(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	started := time.Date(2026, 11, 3, 7, 19, 53, 0, time.UTC)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, Stage: "IN_PROGRESS", StartedOn: &started, DurationSeconds: int64((4 * time.Hour).Seconds())},
	}}

	e.RegisterIncidentClocks(context.Background(), "inc-1", "HIGH", started, "INC0091536", "Router 5xx", "Artemis SRE Group", "Choreo")
	e.RegisterIncidentClocks(context.Background(), "inc-1", "CRITICAL", started, "", "", "", "")
	meta, _, _ := st.GetClock(context.Background(), "inc-1", ClockResolution)
	if meta.Team != "Artemis SRE Group" || meta.CaseNumber != "INC0091536" {
		t.Errorf("group/number = %q/%q, want the stored Artemis SRE Group/INC0091536 kept", meta.Team, meta.CaseNumber)
	}

	e.RegisterIncidentClocks(context.Background(), "inc-1", "CRITICAL", started, "INC0091999", "", "Apollo SRE Group", "")
	meta, _, _ = st.GetClock(context.Background(), "inc-1", ClockResolution)
	if meta.Team != "Apollo SRE Group" || meta.CaseNumber != "INC0091999" {
		t.Errorf("group/number = %q/%q, want the new non-empty values", meta.Team, meta.CaseNumber)
	}
}

// A reassigned incident alerts its current group's SRE team (entity-service's
// AssignmentGroupName), not the team of the group it was registered with.
func TestTick_IncidentBreach_ReassignedIncidentAlertsItsCurrentGroup(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
	resolved := sreTeamWiring(e, chat,
		map[string][]string{"Artemis SRE Group": {"artemis"}, "Apollo SRE Group": {"apollo"}},
		[]string{"apollo", "artemis"},
		map[string]string{"apollo": "apollo@example.com", "artemis": "artemis@example.com"})
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past, AssignmentGroupName: "Apollo SRE Group"},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseType: CaseTypeIncident, Team: "Artemis SRE Group", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if !slices.Equal(*resolved, []string{"Apollo SRE Group"}) {
		t.Errorf("resolved groups = %v, want the current Apollo SRE Group", *resolved)
	}
	if len(chat.incidentCalls) != 1 || chat.incidentCalls[0].audience != notifications.IncidentSLAAudience("apollo") || chat.incidentCalls[0].alert.AssignmentGroup != "Apollo SRE Group" {
		t.Errorf("incident alert = %+v, want Apollo's SLA space naming Apollo SRE Group", chat.incidentCalls)
	}
	if want := []string{"apollo@example.com"}; !slices.Equal(incidentRecipients(email.calls), want) || !strings.Contains(email.calls[0].htmlBody, "Apollo SRE Group") {
		t.Errorf("email recipients = %v, want Apollo's group email naming Apollo SRE Group", incidentRecipients(email.calls))
	}
}

// Until cmd/server has set up incident routing, an incident tier stays due
// and unclaimed (cases never wait on it); once ready, it is sent.
func TestTick_IncidentTierWaitsForRoutingReady(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	e := newTestEngine(st, chat, &fakePublisher{}).HoldIncidentTiersUntilRoutingReady()
	sreTeamWiring(e, chat, map[string][]string{"Artemis SRE Group": {"artemis"}}, []string{"artemis"}, nil)
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseType: CaseTypeIncident, Team: "Artemis SRE Group", Priority: "CRITICAL", StartedAt: past}
	member := wakeMember("inc-1", ClockResolution, 100)
	st.wake[member] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(chat.incidentCalls) != 0 {
		t.Fatalf("incident alert sent before routing was ready: %+v", chat.incidentCalls)
	}
	if _, ok := st.wake[member]; !ok {
		t.Fatal("the held tier's wake entry was removed, want it left due")
	}
	if meta, _, _ := st.GetClock(context.Background(), "inc-1", ClockResolution); meta.AlertedTier >= 100 {
		t.Fatalf("AlertedTier = %d, want the held tier not marked alerted", meta.AlertedTier)
	}

	e.IncidentRoutingReady()
	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(chat.incidentCalls) != 1 {
		t.Errorf("incident alerts after routing ready = %d, want 1", len(chat.incidentCalls))
	}
}

// An incident escalated to an SME team alerts that team only (the one
// entity-service names for the current group), not the SRE team it came from.
func TestTick_IncidentBreach_EscalatedToSMEAlertsTheSMETeam(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
	resolved := sreTeamWiring(e, chat,
		map[string][]string{"Apollo SRE Group": {"apollo"}},
		[]string{"apollo", "asgardeo"},
		map[string]string{"apollo": "apollo@example.com", "asgardeo": "asgardeo@example.com"})
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {
			Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past,
			AssigneeEmail: "melan@example.com", AssignmentGroupName: "Asgardeo", AssignmentGroupTeamKey: " Asgardeo ",
		},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseType: CaseTypeIncident, Team: "Apollo SRE Group", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(*resolved) != 0 {
		t.Errorf("SRE mapping consulted for %v, want the SME team used directly", *resolved)
	}
	if len(chat.incidentCalls) != 1 || chat.incidentCalls[0].audience != notifications.IncidentSLAAudience("asgardeo") || chat.incidentCalls[0].alert.AssignmentGroup != "Asgardeo" {
		t.Errorf("incident alert = %+v, want Asgardeo's SLA space only", chat.incidentCalls)
	}
	if want := []string{"melan@example.com", "asgardeo@example.com"}; !slices.Equal(incidentRecipients(email.calls), want) {
		t.Errorf("email recipients = %v, want %v", incidentRecipients(email.calls), want)
	}
}
