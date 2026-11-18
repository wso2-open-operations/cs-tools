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

// An incident's tier goes to its assignment group's incident SLA spaces (a
// default group fans out to both teams), never the case audiences, plus the
// assignee and group emails -- and no Americas copy, even overnight.
func TestTick_IncidentBreach_GoesToIncidentSLASpacesAndEmails(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, pub)
	e.email = email
	e.now = func() time.Time { return time.Date(2026, time.October, 14, 17, 0, 0, 0, time.UTC) } // 22:30 IST, the Americas window
	e.SetTeamGroupEmails(map[string]string{slaBreachAmericasTeamKey: "americas@example.com"})
	var routedGroup string
	e.WithIncidentSLAAudiences(func(group string) []string {
		routedGroup = group
		return []string{"incident-sla/apollo sre group", "incident-sla/artemis sre group"}
	})
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {
			Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past,
			AssigneeName: "Melan Rashitha", AssigneeEmail: "melan@example.com",
			TeamEmail: "default-sre@example.com", TeamLeadName: "SRE Manager",
		},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseTitle: "Router 5xx", CaseType: CaseTypeIncident, Team: "Default SRE Group", Product: "Choreo", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}

	if routedGroup != "Default SRE Group" {
		t.Errorf("routed group = %q, want the incident's assignment group", routedGroup)
	}
	if len(chat.calls) != 0 {
		t.Errorf("case chat calls = %+v, want none for an incident", chat.calls)
	}
	var audiences []string
	for _, c := range chat.incidentCalls {
		audiences = append(audiences, c.audience)
		a := c.alert
		if a.Number != "INC0091536" || a.Tier != "100" || a.ClockType != ClockResolution || a.AssignmentGroup != "Default SRE Group" ||
			a.GroupManagerName != "SRE Manager" || a.Service != "Choreo" || a.IncidentLink != "https://csm.example.com/incidents/inc-1" {
			t.Errorf("incident alert = %+v, want the incident's fields and link", a)
		}
	}
	if want := []string{"incident-sla/apollo sre group", "incident-sla/artemis sre group"}; !slices.Equal(audiences, want) {
		t.Errorf("incident SLA audiences = %v, want %v", audiences, want)
	}

	if len(email.calls) != 2 {
		t.Fatalf("email calls = %d, want 2 (assignee and group, no Americas copy)", len(email.calls))
	}
	var recipients []string
	for _, c := range email.calls {
		recipients = append(recipients, c.to...)
		if !strings.Contains(c.subject, "INC0091536") {
			t.Errorf("subject = %q, want the incident number", c.subject)
		}
		if strings.Contains(c.htmlBody, "View Case") || strings.Contains(c.htmlBody, "Case ID") {
			t.Errorf("email body uses case wording for an incident")
		}
		if !strings.Contains(c.htmlBody, "https://csm.example.com/incidents/inc-1") {
			t.Errorf("email body has no incident link")
		}
	}
	if want := []string{"melan@example.com", "default-sre@example.com"}; !slices.Equal(recipients, want) {
		t.Errorf("email recipients = %v, want %v", recipients, want)
	}
	if pub.calls != 1 {
		t.Errorf("publish calls = %d, want 1", pub.calls)
	}
}

// With no incident SLA space configured the emails still go out.
func TestTick_IncidentBreach_NoSLASpace_StillEmails(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
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
	if len(email.calls) != 1 || !slices.Equal(email.calls[0].to, []string{"melan@example.com"}) {
		t.Errorf("email calls = %+v, want the assignee email only (no group email on file)", email.calls)
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
	e.WithIncidentSLAAudiences(func(string) []string { return []string{"incident-sla/artemis sre group"} })
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
	e.WithIncidentSLAAudiences(func(string) []string { return []string{"incident-sla/artemis sre group"} })
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past, AssigneeEmail: "melan@example.com", TeamEmail: "artemis@example.com"},
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
	chat := &fakeChat{failFor: map[string]bool{"incident-sla/apollo sre group": true}}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
	e.WithIncidentSLAAudiences(func(string) []string {
		return []string{"incident-sla/apollo sre group", "incident-sla/artemis sre group"}
	})
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past, AssigneeEmail: "melan@example.com", TeamEmail: "default-sre@example.com"},
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
		t.Errorf("email calls = %d, want the assignee and group emails despite the Chat failure", len(email.calls))
	}
}

// The onboarding team is a case concern: an incident on an onboarding
// project emails its assignee and assignment group only.
func TestTick_IncidentBreach_OnboardingProject_NoOnboardingTeamCopy(t *testing.T) {
	st := newFakeStore()
	email := &fakeEmail{}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.email = email
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {
			Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past,
			AssigneeEmail: "melan@example.com", TeamEmail: "artemis@example.com", OnboardingTeamEmail: "onboarding@example.com",
		},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseType: CaseTypeIncident, Team: "Artemis SRE Group", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	var recipients []string
	for _, c := range email.calls {
		recipients = append(recipients, c.to...)
	}
	if want := []string{"melan@example.com", "artemis@example.com"}; !slices.Equal(recipients, want) {
		t.Errorf("email recipients = %v, want %v (no onboarding team copy for an incident)", recipients, want)
	}
}
