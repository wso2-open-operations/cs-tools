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

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// stoppedRun drives one engine: each record handled in turn, and the first
// rung ticked after the first record, so the room has seen the chain.
func stoppedRun(t *testing.T, e *Engine, store *memStore, first eventbus.Record, rest ...eventbus.Record) {
	t.Helper()
	ctx := context.Background()
	if err := e.Handle(ctx, first); err != nil {
		t.Fatal(err)
	}
	st, found, _ := store.Get(ctx, testIncidentID)
	if !found {
		t.Fatal("no chain started")
	}
	if err := e.Tick(ctx, st.Plan.Calls[0].At); err != nil {
		t.Fatal(err)
	}
	for _, r := range rest {
		if err := e.Handle(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
}

// A CRE case chain acknowledged by its assignee replies in its thread: who,
// how, what it paged and how many calls it cancelled.
func TestStoppedChat_CRECaseAcknowledged(t *testing.T) {
	chat, store := &fakeChat{}, newMemStore()
	e := ladderEngine(LadderCRE, chat, store)

	stoppedRun(t, e, store, caseCreated(t, "Atlas", "HIGH"),
		caseAssigned(t, "eng@wso2.com"),
		caseComment(t, "eng@wso2.com", false, true))

	if len(chat.stopped) != 1 {
		t.Fatalf("stopped replies = %d; want 1", len(chat.stopped))
	}
	s := chat.stopped[0]
	if !s.Answered || s.Restarted || s.Outcome != "Acknowledged" || s.Chain != "CRE" {
		t.Errorf("reply = %+v; want an acknowledged CRE reply", s)
	}
	if !strings.Contains(s.Detail, "eng@wso2.com") || !strings.Contains(s.Detail, "assigned and a public comment") {
		t.Errorf("detail = %q; want the assignee and both gestures", s.Detail)
	}
	if s.ThreadKey != "incident-escalation-"+testIncidentID || s.Audience != "Incident Monitor" || s.Reply {
		t.Errorf("posted to %q thread %q reply=%v; want the rung cards' room and thread", s.Audience, s.ThreadKey, s.Reply)
	}
	if len(s.Paged) == 0 || s.Cancelled == 0 {
		t.Errorf("paged %v, cancelled %d; want the first rung paged and the rest cancelled", s.Paged, s.Cancelled)
	}
	if s.IncidentRef == "" || s.PortalLabel != "View case" || !strings.Contains(s.PortalURL, testIncidentID) {
		t.Errorf("ref %q link %q %q; want the case and its link", s.IncidentRef, s.PortalLabel, s.PortalURL)
	}
}

// A severity change that replaces a running chain says so once, in the same
// thread; a replay of the change says nothing more.
func TestStoppedChat_SeverityChangeRestartsTheChain(t *testing.T) {
	chat, store := &fakeChat{}, newMemStore()
	e := ladderEngine(LadderCRE, chat, store)

	stoppedRun(t, e, store, caseCreated(t, "Atlas", "HIGH"),
		severityChanged(t, "HIGH", "CRITICAL"),
		severityChanged(t, "HIGH", "CRITICAL")) // redelivered

	if len(chat.stopped) != 1 {
		t.Fatalf("stopped replies = %d; want 1 for the change and none for its replay", len(chat.stopped))
	}
	s := chat.stopped[0]
	if !s.Restarted || s.Answered || s.Outcome != "Severity changed to CRITICAL" {
		t.Errorf("reply = %+v; want a restart for the new severity", s)
	}
	if !strings.Contains(s.Detail, "from HIGH to CRITICAL") {
		t.Errorf("detail = %q; want both severities", s.Detail)
	}
	// A CRE chain's first rung may be L0 or L1, so the reply names neither.
	if !strings.Contains(s.Detail, "starts from the first rung") || strings.Contains(s.Detail, "from L1") {
		t.Errorf("detail = %q; want the restart without a fixed rung label", s.Detail)
	}
	if s.ThreadKey != "incident-escalation-"+testIncidentID {
		t.Errorf("thread = %q; want the case's thread, where the new chain posts too", s.ThreadKey)
	}
	if st, found, _ := store.Get(context.Background(), testIncidentID); !found || NormalisePriority(st.Plan.Trigger.Priority) != "P1" {
		t.Fatal("the new CRITICAL chain is not running")
	}
}

// Stops that are not an acknowledgement say what they were.
func TestStoppedChat_OtherStops(t *testing.T) {
	for name, tc := range map[string]struct {
		stop    eventbus.Record
		outcome string
	}{
		"case closed":        {caseStatus(t, "Closed"), "Case closed"},
		"severity not paged": {severityChanged(t, "HIGH", "LOW"), "Severity changed to LOW"},
	} {
		t.Run(name, func(t *testing.T) {
			chat, store := &fakeChat{}, newMemStore()
			e := ladderEngine(LadderCRE, chat, store)
			e.cfg.Ladder.Start.Priorities = []string{"S0", "S1", "S2", "S3"}

			stoppedRun(t, e, store, caseCreated(t, "Atlas", "HIGH"), tc.stop)

			if len(chat.stopped) != 1 {
				t.Fatalf("stopped replies = %d; want 1", len(chat.stopped))
			}
			if s := chat.stopped[0]; s.Outcome != tc.outcome || s.Answered || s.Restarted {
				t.Errorf("reply = %+v; want %q, neither answered nor restarted", s, tc.outcome)
			}
		})
	}
}

// An SRE incident stops on an assignee, and its space gets the "Incident Acknowledged." card instead of a stopped reply.
func TestStoppedChat_SREIncidentAssigned(t *testing.T) {
	chat, store := &fakeChat{}, newMemStore()
	e := ladderEngine(LadderSRE, chat, store)

	stoppedRun(t, e, store, created(t, "Apollo", "HIGH"), assigned(t))

	if len(chat.stopped) != 0 || len(chat.acks) != 1 {
		t.Fatalf("stopped %d, acknowledged %d; want only the acknowledged card", len(chat.stopped), len(chat.acks))
	}
	if a := chat.acks[0]; a.AssigneeName != "a-l2" || a.IncidentID != testIncidentID {
		t.Errorf("acknowledged = %+v; want the assignee", a)
	}
}

// An incident whose rungs replied under its card replies there when its chain restarts.
func TestStoppedChat_RepliesUnderTheIncidentCard(t *testing.T) {
	chat := &fakeChat{}
	e := incidentSpaceEngine(chat)
	e.cfg.CallSendingEnabled = true
	plan := Plan{
		Trigger: Trigger{IncidentID: "inc-1", Record: RecordIncident, Team: "MS/PC SRE Group", Priority: "HIGH", Routing: RoutingContext{Ladder: LadderSRE}},
		Calls:   []PlannedCall{{Level: Level0, Ordinal: 1, Recipient: Recipient{Name: "Jane Doe"}}, {Level: Level1, Ordinal: 1, Recipient: Recipient{Name: "Lead"}}},
	}
	st := LadderState{Plan: plan, Placed: []bool{true, false}}

	e.postStopped(context.Background(), st, cancelStateChange, &Trigger{Priority: "CRITICAL"})

	if len(chat.stopped) != 1 {
		t.Fatalf("stopped replies = %d; want 1", len(chat.stopped))
	}
	s := chat.stopped[0]
	if s.Audience != notifications.IncidentAudience("MS/PC SRE Group") || !s.Reply || s.ThreadKey != notifications.IncidentThreadKey("inc-1") {
		t.Errorf("posted to %q reply=%v thread=%q; want a reply under the incident card", s.Audience, s.Reply, s.ThreadKey)
	}
	if s.Cancelled != 1 || strings.Join(s.Paged, ", ") != "L1 Jane Doe" {
		t.Errorf("paged %v cancelled %d; want L1 Jane Doe paged and one call cancelled", s.Paged, s.Cancelled)
	}
}

// No reply when the room never saw the chain, when it is switched off, for a
// hand-off to the SMEs (which posts its own), or for a stop being finished.
func TestStoppedChat_StaysQuiet(t *testing.T) {
	ctx := context.Background()
	placed := func() LadderState {
		return LadderState{
			Plan:   Plan{Trigger: Trigger{IncidentID: testIncidentID, Number: "INC1", Priority: "HIGH"}, Calls: []PlannedCall{{Level: Level1, Ordinal: 1, At: testClock, Recipient: Recipient{Name: "Jane"}}}},
			Placed: []bool{true},
		}
	}
	off := false
	for name, tc := range map[string]struct {
		setup  func(e *Engine)
		st     func() LadderState
		reason cancelReason
	}{
		"no rung went out": {nil, func() LadderState { st := placed(); st.Placed[0] = false; return st }, cancelAcknowledged},
		"ackChat off":      {func(e *Engine) { e.cfg.Ladder.AckChat = &off }, placed, cancelAcknowledged},
		"sending disabled": {func(e *Engine) { e.cfg.CallSendingEnabled = false }, placed, cancelAcknowledged},
		"handed to SMEs":   {nil, placed, cancelEscalatedToSME},
	} {
		t.Run(name, func(t *testing.T) {
			chat := &fakeChat{}
			e := ladderEngine(LadderCRE, chat, newMemStore())
			if tc.setup != nil {
				tc.setup(e)
			}
			e.postStopped(ctx, tc.st(), tc.reason, nil)
			if len(chat.stopped) != 0 {
				t.Errorf("posted %+v; want nothing", chat.stopped)
			}
		})
	}

	t.Run("finishing an earlier stop", func(t *testing.T) {
		chat, store := &fakeChat{}, newMemStore()
		e := ladderEngine(LadderCRE, chat, store)
		st := placed()
		now := time.Now()
		st.Cancelled, st.CancelReason = &now, string(cancelAcknowledged)
		if err := e.stopLadder(ctx, testIncidentID, st, cancelAcknowledged); err != nil {
			t.Fatal(err)
		}
		if len(chat.stopped) != 0 {
			t.Errorf("a retried stop posted again: %+v", chat.stopped)
		}
	})

	t.Run("ack before the first rung", func(t *testing.T) {
		chat, store := &fakeChat{}, newMemStore()
		e := ladderEngine(LadderCRE, chat, store)
		for _, r := range []eventbus.Record{caseCreated(t, "Atlas", "HIGH"), caseAssigned(t, "eng@wso2.com"), caseComment(t, "eng@wso2.com", false, true)} {
			if err := e.Handle(ctx, r); err != nil {
				t.Fatal(err)
			}
		}
		if len(chat.stopped) != 0 {
			t.Errorf("posted %+v for a chain the room never saw", chat.stopped)
		}
	})
}

// ackChat defaults to on, can be turned off, and is allowed on both ladders.
func TestConfig_AckChat(t *testing.T) {
	if !(LadderConfig{}).AckChatOn() {
		t.Error("ackChat is off by default; want on")
	}
	cfg, err := loadYAML(t, "enabled: true\ncre:\n  enabled: true\n  ackChat: false\nsre:\n  enabled: true\n  ackChat: true\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.CRE.AckChatOn() || !cfg.SRE.AckChatOn() {
		t.Errorf("cre on=%v sre on=%v; want cre off, sre on", cfg.CRE.AckChatOn(), cfg.SRE.AckChatOn())
	}
}

// An answered SRE incident posts no "paging stopped" reply under its incident card, however it was answered.
func TestStoppedChat_SREIncidentAnsweredUnderTheIncidentCardPostsNothing(t *testing.T) {
	for _, reason := range []cancelReason{cancelStateChange, cancelAcknowledged, cancelPublicComment, cancelAssigned} {
		t.Run(string(reason), func(t *testing.T) {
			chat := &fakeChat{}
			e := incidentSpaceEngine(chat)
			e.cfg.CallSendingEnabled = true
			plan := Plan{
				Trigger: Trigger{IncidentID: "inc-1", Record: RecordIncident, Team: "MS/PC SRE Group", Priority: "HIGH", Routing: RoutingContext{Ladder: LadderSRE}},
				Calls:   []PlannedCall{{Level: Level1, Ordinal: 1, Recipient: Recipient{Name: "Jane Doe"}}},
			}
			e.postStopped(context.Background(), LadderState{Plan: plan, Placed: []bool{true}}, reason, nil)
			if len(chat.stopped) != 0 || len(chat.acks) != 0 {
				t.Errorf("stopped %+v, acknowledged %+v; want nothing", chat.stopped, chat.acks)
			}
		})
	}
}
