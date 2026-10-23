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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// An SME space gets one "Incident Escalated" card naming who escalated it and the L1 on call, then every rung as a reply under it.
func TestSMECards_IncidentCardThenReplies(t *testing.T) {
	ctx := context.Background()
	smeChat := &fakeChat{}
	e := smeEngine(smeTieredReader(), newMemStore(), &fakeNotes{}, &fakeChat{}, smeChat, smeOn)

	p := handoff("Apollo", "asgardeo", "asgardeo", testClock)
	p.Priority, p.ServiceName, p.State = "P1", "Gateway", "IN_PROGRESS"
	raise(t, e, p)
	for _, at := range []time.Duration{5 * time.Minute, 10 * time.Minute} {
		if err := e.Tick(ctx, testClock.Add(at)); err != nil {
			t.Fatal(err)
		}
	}

	if len(smeChat.smeIncidents) != 1 {
		t.Fatalf("SME incident cards = %d; want exactly one", len(smeChat.smeIncidents))
	}
	card := smeChat.smeIncidents[0]
	if card.EscalatedBy != "lead@example.com" || card.EscalatedFrom != "Apollo" {
		t.Errorf("escalated by %q from %q; want lead@example.com from Apollo", card.EscalatedBy, card.EscalatedFrom)
	}
	if card.OnCall != "s-a" || card.SMETeam != "Asgardeo" || card.Priority != "P1" || card.Service != "Gateway" || card.State != "IN_PROGRESS" {
		t.Errorf("card = %+v; want L1 s-a on call for Asgardeo, P1, Gateway, IN_PROGRESS", card)
	}
	if len(smeChat.incidents) != 0 {
		t.Errorf("an SME space got %d SRE \"Incident Reported\" cards; want none", len(smeChat.incidents))
	}
	if len(smeChat.posted) != 3 {
		t.Fatalf("SME rungs = %d; want L1, L2 and L3", len(smeChat.posted))
	}
	for _, r := range smeChat.posted {
		if !r.Reply || r.ThreadKey == "" {
			t.Errorf("rung %s: reply %v, thread %q; want a reply in the incident card's thread", r.Rung, r.Reply, r.ThreadKey)
		}
	}
}

// An SME ladder stopped by an assignment ends its thread with its own stopped reply, never the SRE "Incident Acknowledged." card.
func TestSMECards_AssignmentPostsTheStoppedReply(t *testing.T) {
	ctx := context.Background()
	smeChat := &fakeChat{}
	e := smeEngine(smeTieredReader(), newMemStore(), &fakeNotes{}, &fakeChat{}, smeChat, smeOn)

	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock))
	handle(t, e, record(t, events.TypeIncidentAssigned, events.IncidentAssignedPayload{
		AssigneeID: "s-a", AssigneeName: "s-a", AssignedOn: testClock.Add(2 * time.Minute).Format(time.RFC3339)}))
	if err := e.Tick(ctx, testClock.Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}

	if len(smeChat.acks) != 0 {
		t.Errorf("the SME space got %d \"Incident Acknowledged.\" cards; want none", len(smeChat.acks))
	}
	if len(smeChat.stopped) != 1 || !smeChat.stopped[0].Reply {
		t.Errorf("stopped replies = %+v; want one, as a reply under the incident card", smeChat.stopped)
	}
}

// With nobody rostered on any tier the SME space still gets the "Incident Escalated" card, saying so, and no rung.
func TestSMECards_NobodyOnCallStillPostsTheCard(t *testing.T) {
	smeChat, store, notes := &fakeChat{}, newMemStore(), &fakeNotes{}
	e := smeEngine(rotaReader(true), store, notes, &fakeChat{}, smeChat, smeOn)

	raise(t, e, handoff("Apollo", "bijira", "bijira", testClock))

	if len(smeChat.smeIncidents) != 1 {
		t.Fatalf("SME incident cards = %d; want one with nobody on call", len(smeChat.smeIncidents))
	}
	if card := smeChat.smeIncidents[0]; card.OnCall != "" || card.SMETeam != "Bijira" || card.EscalatedBy != "lead@example.com" {
		t.Errorf("card = %+v; want Bijira, escalated by lead@example.com, nobody on call", card)
	}
	if len(smeChat.posted) != 0 {
		t.Errorf("SME rungs = %d; want none with nobody on call", len(smeChat.posted))
	}
	if len(notesWith(notes, "NO_RECIPIENTS")) != 1 {
		t.Errorf("work notes = %v; want NO_RECIPIENTS", notes.notes)
	}
}

// With L1 empty the card goes out at the escalation naming L2 and when it is called, and L2 then posts only its reply.
func TestSMECards_NoL1PostsTheCardAtOnce(t *testing.T) {
	ctx := context.Background()
	r := rotaReader(true)
	r.onDuty = []onDutyAssignment{held("s-l2", "asgardeo", "SME_ASG_DAY", "L2")}
	smeChat := &fakeChat{}
	e := smeEngine(r, newMemStore(), &fakeNotes{}, &fakeChat{}, smeChat, smeOn)

	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock))
	if len(smeChat.smeIncidents) != 1 {
		t.Fatalf("at once: SME incident cards = %d; want one", len(smeChat.smeIncidents))
	}
	if card := smeChat.smeIncidents[0]; card.OnCall != "s-l2" || card.OnCallRung != "L2" || card.OnCallAt != istStamp(testClock.Add(5*time.Minute)) {
		t.Errorf("card on call = %q %q %q; want s-l2 at L2, called five minutes after the escalation", card.OnCall, card.OnCallRung, card.OnCallAt)
	}
	if len(smeChat.posted) != 0 {
		t.Fatalf("at once: SME rungs = %d; want none before L2 is due", len(smeChat.posted))
	}
	if err := e.Tick(ctx, testClock.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(smeChat.smeIncidents) != 1 {
		t.Errorf("SME incident cards = %d after L2; want still one", len(smeChat.smeIncidents))
	}
	if len(smeChat.posted) != 1 || smeChat.posted[0].RecipientName != "s-l2" || !smeChat.posted[0].Reply {
		t.Errorf("SME rungs = %+v; want L2 s-l2 as a reply", smeChat.posted)
	}
}

// With L1 empty, an assignment before L2 still ends the card's thread with a stopped reply, the card having told the room.
func TestSMECards_NoL1AssignmentBeforeL2PostsTheStoppedReply(t *testing.T) {
	ctx := context.Background()
	r := rotaReader(true)
	r.onDuty = []onDutyAssignment{held("s-l2", "asgardeo", "SME_ASG_DAY", "L2")}
	smeChat := &fakeChat{}
	e := smeEngine(r, newMemStore(), &fakeNotes{}, &fakeChat{}, smeChat, smeOn)

	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock))
	handle(t, e, record(t, events.TypeIncidentAssigned, events.IncidentAssignedPayload{
		AssigneeID: "s-l2", AssigneeName: "s-l2", AssignedOn: testClock.Add(2 * time.Minute).Format(time.RFC3339)}))
	if err := e.Tick(ctx, testClock.Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}

	if len(smeChat.posted) != 0 {
		t.Errorf("SME rungs = %d; want none, assigned before L2", len(smeChat.posted))
	}
	if len(smeChat.stopped) != 1 || !smeChat.stopped[0].Reply {
		t.Errorf("stopped replies = %+v; want one, as a reply under the incident card", smeChat.stopped)
	}
}
