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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func sreCase(sreGroup string) Trigger {
	return Trigger{IncidentID: "case-1", Record: RecordCase, Team: "Phoenix", At: time.Now(),
		Routing: RoutingContext{Ladder: LadderSRE, AssignedCRETeam: "Phoenix", SRETeam: sreGroup}}
}

// An S0 case with an account SRE team replies under the card the dispatcher
// posted to that team's room, as an incident's rungs do.
func TestSRECaseRung_RepliesUnderItsCardInTheSRERoom(t *testing.T) {
	chat := &fakeChat{}
	e := incidentSpaceEngine(chat)
	routes := &notifications.IncidentRoutes{}
	routes.SetKnownSRETeam(func(group string) bool { return group == "MS/PC SRE Group" })
	e.cfg.IncidentRoutes = routes
	deliverFirstRung(t, e, sreCase("MS/PC SRE Group"))
	if len(chat.posted) != 1 {
		t.Fatalf("posted %d cards; want 1", len(chat.posted))
	}
	got := chat.posted[0]
	if got.Audience != notifications.IncidentAudience("MS/PC SRE Group") || !got.Reply || got.ThreadKey != notifications.IncidentThreadKey("case-1") {
		t.Errorf("card to %q reply=%v thread=%q; want a reply under the case card in its SRE room", got.Audience, got.Reply, got.ThreadKey)
	}
}

// A case with no account SRE team replies under its card in the teamless
// rooms, where the dispatcher posted it.
func TestSRECaseRung_NoSRETeamRepliesInTheTeamlessRooms(t *testing.T) {
	chat := &fakeChat{}
	e := incidentSpaceEngine(chat)
	routes := &notifications.IncidentRoutes{}
	routes.SetTeamless([]string{"apollo", "artemis"})
	e.cfg.IncidentRoutes = routes
	deliverFirstRung(t, e, sreCase(""))
	if len(chat.posted) != 2 {
		t.Fatalf("posted %+v; want one reply in each teamless room", chat.posted)
	}
	for i, want := range []string{"apollo", "artemis"} {
		got := chat.posted[i]
		if got.Audience != notifications.IncidentAudience(want) || !got.Reply || got.ThreadKey != notifications.IncidentThreadKey("case-1") {
			t.Errorf("card to %q reply=%v thread=%q; want a reply under the case card in %s's room", got.Audience, got.Reply, got.ThreadKey, want)
		}
	}
}

// With no teamless room and no shared SRE room, nothing was carded, so the
// ladder keeps its own full card.
func TestSRECaseRung_NoRoomAtAllKeepsTheLaddersOwnCard(t *testing.T) {
	chat := &fakeChat{}
	deliverFirstRung(t, incidentSpaceEngine(chat), sreCase(""))
	if len(chat.posted) != 1 || chat.posted[0].Audience != "SRE Case Paging" || chat.posted[0].Reply {
		t.Fatalf("posted %+v; want the ladder's own full card", chat.posted)
	}
}

// The CRE ladder is untouched: a case there keeps its own spaces and thread,
// even when the case carries an account SRE team.
func TestSRECaseRung_CRELadderUnchanged(t *testing.T) {
	cre := sreCase("MS/PC SRE Group")
	cre.Routing.Ladder = LadderCRE
	if got := cre.chatThreadKey(); got != "incident-escalation-case-1" {
		t.Errorf("CRE case thread = %q; want incident-escalation-case-1", got)
	}

	e := incidentSpaceEngine(&fakeChat{})
	e.cfg.Kind = LadderCRE
	if rooms := e.incidentRooms(cre); len(rooms) != 0 {
		t.Errorf("CRE engine rooms = %+v; want none", rooms)
	}
	// Even an SRE-ladder trigger is never routed to SRE rooms by the CRE engine.
	if rooms := e.incidentRooms(sreCase("MS/PC SRE Group")); len(rooms) != 0 {
		t.Errorf("CRE engine rooms for an SRE case = %+v; want none", rooms)
	}
}

// The route table carries the teamless rooms that have a space, for an S0
// case whose account has no SRE team.
func TestBuildIncidentRoutes_Teamless(t *testing.T) {
	l := LadderConfig{
		TeamChats:     map[string]Chat{"apollo": {Audience: "Apollo SRE"}, "iaas": {Audience: "IaaS SRE"}},
		TeamlessChats: []string{"apollo", "artemis", "iaas"},
	}
	table := BuildIncidentRoutes(l, func(c Chat) (string, bool) { return "https://chat.example/" + c.Audience, c.Audience != "" })
	if got := strings.Join(table.Teamless, ","); got != "apollo,iaas" {
		t.Errorf("Teamless = %q; want apollo,iaas (artemis has no room)", got)
	}
}

// An SRE group the ladder does not know is paged like a case with no SRE
// team, so its replies go to the teamless rooms with its calls.
func TestSRECaseRung_UnknownSREGroupRepliesInTheTeamlessRooms(t *testing.T) {
	chat := &fakeChat{}
	e := incidentSpaceEngine(chat)
	routes := &notifications.IncidentRoutes{}
	routes.SetTeamless([]string{"apollo"})
	routes.SetKnownSRETeam(func(string) bool { return false })
	e.cfg.IncidentRoutes = routes
	deliverFirstRung(t, e, sreCase("U2 SRE Group"))
	if len(chat.posted) != 1 || chat.posted[0].Audience != notifications.IncidentAudience("apollo") || !chat.posted[0].Reply {
		t.Fatalf("posted %+v; want one reply in the teamless room", chat.posted)
	}
}

// KnownSRETeam is the engine's own known-team check (hasOwnSRETeam).
func TestKnownSRETeam(t *testing.T) {
	l := LadderConfig{Teams: TeamKeys{
		ABTs:          []string{"apollo_abt_sre_team", "iaas"},
		Aliases:       map[string]string{"MS/PC SRE Group": "apollo_abt_sre_team", "Choreo SRE Team": "iaas", "Old Group": "ms/pc sre group"},
		DefaultGroups: []string{"Default SRE Group"},
	}}
	for group, want := range map[string]bool{
		"MS/PC SRE Group":   true,  // alias to a team in abts
		"choreo sre team":   true,  // alias, any case
		"U2 SRE Group":      false, // no alias, not in abts
		"Old Group":         false, // alias to a key not in abts
		"Default SRE Group": false, // a default group
		"":                  false,
	} {
		if got := KnownSRETeam(l, group); got != want {
			t.Errorf("KnownSRETeam(%q) = %v; want %v", group, got, want)
		}
	}
}

// Each S0 episode of a case gets its own thread: the SRE ladder keys its
// messages by the record that started the chain, the same key the
// dispatcher's room card has (notifications.CaseS0ThreadKey). Lowering below
// S0 stops the running chain in its own thread; raising to S0 again starts a
// chain in a new one.
func TestSRECaseRung_EachS0EpisodeHasItsOwnThread(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	e := ladderEngine(LadderSRE, &fakeChat{}, store)

	created := caseCreated(t, "Atlas", "CATASTROPHIC")
	if err := e.Handle(ctx, created); err != nil {
		t.Fatal(err)
	}
	st, found, _ := store.Get(ctx, testIncidentID)
	if !found {
		t.Fatal("no SRE chain for an S0 case")
	}
	if want := notifications.CaseS0ThreadKey(testIncidentID, created.Value); st.Plan.Trigger.chatThreadKey() != want {
		t.Errorf("first episode thread = %q; want %q", st.Plan.Trigger.chatThreadKey(), want)
	}
	firstThread := st.Plan.Trigger.chatThreadKey()

	if err := e.Handle(ctx, severityChanged(t, "CATASTROPHIC", "CRITICAL")); err != nil {
		t.Fatal(err)
	}
	raised := severityChanged(t, "CRITICAL", "CATASTROPHIC")
	if err := e.Handle(ctx, raised); err != nil {
		t.Fatal(err)
	}
	st, found, _ = store.Get(ctx, testIncidentID)
	if !found {
		t.Fatal("no SRE chain after the raise to S0")
	}
	second := st.Plan.Trigger.chatThreadKey()
	if want := notifications.CaseS0ThreadKey(testIncidentID, raised.Value); second != want {
		t.Errorf("second episode thread = %q; want %q", second, want)
	}
	if second == firstThread {
		t.Errorf("the raise to S0 reused thread %q; want a new one", second)
	}
}

// The per-episode thread is the SRE ladder's only: a CRE-ladder case keeps
// its own thread, and an incident keeps the incident card's.
func TestSRECaseRung_EpisodeThreadOnlyOnTheSRELadder(t *testing.T) {
	cre := sreCase("MS/PC SRE Group")
	cre.Routing.Ladder = LadderCRE
	cre.CardThread = "incident-case-1-abc"
	if got := cre.chatThreadKey(); got != "incident-escalation-case-1" {
		t.Errorf("CRE case thread = %q; want incident-escalation-case-1", got)
	}
	inc := Trigger{IncidentID: "inc-1", Record: RecordIncident, Routing: RoutingContext{Ladder: LadderSRE}}
	if got := inc.chatThreadKey(); got != notifications.IncidentThreadKey("inc-1") {
		t.Errorf("incident thread = %q; want %q", got, notifications.IncidentThreadKey("inc-1"))
	}
	sre := sreCase("MS/PC SRE Group")
	if got := sre.chatThreadKey(); got != notifications.IncidentThreadKey("case-1") {
		t.Errorf("SRE case chain saved before CardThread = %q; want the case's one thread", got)
	}
}
