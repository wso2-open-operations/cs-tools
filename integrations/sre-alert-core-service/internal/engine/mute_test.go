// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package engine

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"alert-core-service/internal/csm"
	"alert-core-service/internal/model"
	"alert-core-service/internal/muterule"
)

func (f *fakeIncidents) Mute(_ context.Context, id int64, ruleID, ruleNumber string) error {
	return f.update(id, func(i *model.Incident) {
		if i.MutedAt == nil {
			now := time.Now()
			i.MutedAt = &now
		}
		i.Muted, i.MuteRuleID, i.MuteRuleNumber = true, ruleID, ruleNumber
	})
}

func (f *fakeIncidents) Unmute(_ context.Context, id int64) error {
	return f.update(id, func(i *model.Incident) {
		now := time.Now()
		i.Muted, i.UnmutedAt = false, &now
	})
}

func (f *fakeIncidents) MuteOutcome(_ context.Context, inc model.Incident) (bool, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lastKind := ""
	for _, n := range f.notes {
		if f.noteInc[n.ID] == inc.ID {
			lastKind = n.Kind
		}
	}
	superseded := false
	for _, other := range f.incidents {
		if other.Fingerprint == inc.Fingerprint && other.FirstSeen.After(inc.FirstSeen) {
			superseded = true
		}
	}
	return lastKind == model.NoteOK, superseded, nil
}

func (f *fakeIncidents) CloseMuted(_ context.Context, id, foldVersion int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc := f.incidents[id]
	if inc.FoldVersion != foldVersion {
		return false, nil
	}
	now := time.Now()
	inc.Muted, inc.UnmutedAt, inc.Status = false, &now, "closed"
	f.due[id] = nil
	for i := range f.notes {
		if f.noteInc[f.notes[i].ID] == id {
			f.notes[i].CSMPending, f.notes[i].ChatPending = false, false
		}
	}
	return true, nil
}

// fakeBlocks is a settable set of active alert mute rules.
type fakeBlocks struct {
	mu    sync.Mutex
	rules []muterule.Rule
}

func (b *fakeBlocks) set(rules ...muterule.Rule) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rules = rules
}

func (b *fakeBlocks) Match(f muterule.Fields) (muterule.Rule, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, bl := range b.rules {
		if muterule.Matches(bl, f) {
			return bl, true
		}
	}
	return muterule.Rule{}, false
}

// svcBlock matches item()'s alerts (service "svc", source "vendor").
func svcBlock(id string) muterule.Rule {
	return muterule.Rule{ID: id, Number: "AMR-" + id, Name: "maintenance",
		Conditions: []csm.AlertMuteRuleCondition{{Field: "service", Value: "SVC"}, {Field: "source", Value: " Vendor "}}}
}

func newMutingEngine(n *fakeNotifier) (*Engine, *fakeIncidents, *fakeBlocks) {
	rules := &fakeBlocks{}
	cfg := testConfig()
	cfg.Rules = rules
	cfg.MuteRecheckInterval = time.Minute
	e, incidents := newTestEngine(n, cfg)
	return e, incidents, rules
}

// makeDue makes incident id due now, as the next recheck would.
func (f *fakeIncidents) makeDue(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	f.due[id] = &now
}

func TestMute_MatchingIncidentIsMutedAndNothingIsSent(t *testing.T) {
	n := &fakeNotifier{csmOK: true, chatOK: true}
	e, incidents, rules := newMutingEngine(n)
	rules.set(svcBlock("b1"))
	handle(t, e, item("ALT1", 0, "critical"))

	e.DeliverDue(context.Background())
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if n.csmCalls != 0 || n.chatCalls != 0 {
		t.Fatalf("csm = %d chat = %d, want nothing sent for a muted incident", n.csmCalls, n.chatCalls)
	}
	if !inc.Muted || inc.MuteRuleID != "b1" || inc.MuteRuleNumber != "AMR-b1" || inc.MutedAt == nil {
		t.Fatalf("incident = %+v, want it muted by b1", inc)
	}
	if due := incidents.dueAt(inc.ID); due == nil || time.Until(*due) < 50*time.Second {
		t.Fatalf("due = %v, want a re-check about MuteRecheckInterval out", due)
	}

	// New alerts folding in while the rule still matches neither unmute nor deliver it.
	handle(t, e, item("ALT2", time.Second, "critical"))
	e.DeliverDue(context.Background())
	inc = incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if n.csmCalls != 0 || n.chatCalls != 0 || len(n.pushed) != 0 || !inc.Muted || inc.AlertCount != 2 {
		t.Fatalf("csm = %d chat = %d pushed = %v incident = %+v, want it still muted with the duplicate recorded", n.csmCalls, n.chatCalls, n.pushed, inc)
	}
}

func TestMute_ReleasedWhileFiringIsDeliveredWithItsNotes(t *testing.T) {
	n := &fakeNotifier{csmOK: true, chatOK: true}
	e, incidents, rules := newMutingEngine(n)
	rules.set(svcBlock("b1"))
	handle(t, e, item("ALT1", 0, "critical"))
	e.DeliverDue(context.Background())
	handle(t, e, item("ALT2", time.Second, "critical"))
	e.DeliverDue(context.Background())

	rules.set()
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	incidents.makeDue(inc.ID)
	e.DeliverDue(context.Background())

	inc = incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if n.csmCalls != 1 || !inc.CSMConfirmed || inc.Muted || inc.UnmutedAt == nil || inc.MuteRuleNumber != "AMR-b1" {
		t.Fatalf("csm = %d incident = %+v, want it unmuted and created in CSM once", n.csmCalls, inc)
	}
	if !strings.Contains(n.creationNotes[0], "ALT1") || len(n.pushed) != 1 || !strings.Contains(n.pushed[0], "Duplicate") {
		t.Fatalf("creation = %q pushed = %q, want the creation note and the muted duplicate delivered", n.creationNotes, n.pushed)
	}
	if n.chatCalls != 0 {
		t.Fatalf("chat = %d, want no Chat once CSM confirmed", n.chatCalls)
	}
}

func TestMute_ResolvedWhileMutedIsClosedQuietly(t *testing.T) {
	n := &fakeNotifier{csmOK: true, chatOK: true}
	e, incidents, rules := newMutingEngine(n)
	rules.set(svcBlock("b1"))
	handle(t, e, item("ALT1", 0, "critical"))
	e.DeliverDue(context.Background())
	handle(t, e, item("ALT2", time.Second, "OK"))
	e.DeliverDue(context.Background())

	rules.set()
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	incidents.makeDue(inc.ID)
	e.DeliverDue(context.Background())

	inc = incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if n.csmCalls != 0 || n.chatCalls != 0 || len(n.pushed) != 0 {
		t.Fatalf("csm = %d chat = %d pushed = %v, want nothing ever sent", n.csmCalls, n.chatCalls, n.pushed)
	}
	if inc.Muted || inc.Status != "closed" || inc.CSMConfirmed || incidents.dueAt(inc.ID) != nil {
		t.Fatalf("incident = %+v due = %v, want it closed quietly with nothing due", inc, incidents.dueAt(inc.ID))
	}
	if slices.ContainsFunc(incidents.notesOf(inc.ID), func(n model.Note) bool { return n.CSMPending || n.ChatPending }) {
		t.Fatal("a quietly closed incident must owe nothing")
	}

	// A re-fire inside the dedup window opens a new incident instead of folding into the closed one.
	handle(t, e, item("ALT3", 2*time.Second, "critical"))
	got := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	if len(got) != 2 || got[1].Status != "new" {
		t.Fatalf("incidents = %+v, want a new incident after the quiet close", got)
	}
	e.DeliverDue(context.Background())
	if n.csmCalls != 1 {
		t.Fatalf("csm = %d, want the new incident delivered", n.csmCalls)
	}
}

func TestMute_LaterOKOnQuietlyClosedIncidentIsNotDelivered(t *testing.T) {
	n := &fakeNotifier{csmOK: true, chatOK: true}
	e, incidents, rules := newMutingEngine(n)
	rules.set(svcBlock("b1"))
	handle(t, e, item("ALT1", 0, "critical"))
	e.DeliverDue(context.Background())
	handle(t, e, item("ALT2", time.Second, "OK"))
	e.DeliverDue(context.Background())
	rules.set()
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	incidents.makeDue(inc.ID)
	e.DeliverDue(context.Background()) // closed quietly

	// Another OK folds into the closed incident and makes it due again.
	handle(t, e, item("ALT3", 2*time.Second, "OK"))
	if incidents.dueAt(inc.ID) == nil {
		t.Fatal("test setup: the late OK should leave the incident due")
	}
	e.DeliverDue(context.Background())

	inc = incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if n.csmCalls != 0 || n.chatCalls != 0 || len(n.pushed) != 0 {
		t.Fatalf("csm = %d chat = %d pushed = %v, want nothing sent for a quietly closed incident", n.csmCalls, n.chatCalls, n.pushed)
	}
	if inc.Status != "closed" || incidents.dueAt(inc.ID) != nil {
		t.Fatalf("incident = %+v due = %v, want it still closed with nothing due", inc, incidents.dueAt(inc.ID))
	}
	if slices.ContainsFunc(incidents.notesOf(inc.ID), func(n model.Note) bool { return n.CSMPending || n.ChatPending }) {
		t.Fatal("the late OK note must be settled, not left pending")
	}
}

func TestMute_SupersededGenerationIsClosedAndLatestDelivered(t *testing.T) {
	n := &fakeNotifier{csmOK: true, chatOK: true}
	e, incidents, rules := newMutingEngine(n)
	rules.set(svcBlock("b1"))
	handle(t, e, item("ALT1", 0, "critical"))
	handle(t, e, item("ALT2", 6*time.Minute, "critical"))
	e.DeliverDue(context.Background())
	got := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	if len(got) != 2 || !got[0].Muted || !got[1].Muted {
		t.Fatalf("incidents = %+v, want both generations muted", got)
	}

	rules.set()
	incidents.makeDue(got[0].ID)
	incidents.makeDue(got[1].ID)
	e.DeliverDue(context.Background())
	got = incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	if n.csmCalls != 1 || got[0].Status != "closed" || got[0].CSMConfirmed || !got[1].CSMConfirmed {
		t.Fatalf("csm = %d incidents = %+v, want only the latest generation delivered", n.csmCalls, got)
	}
}

func TestMute_NonMatchingAndAlreadyDeliveredIncidentsAreNotMuted(t *testing.T) {
	n := &fakeNotifier{csmOK: true, chatOK: true}
	e, incidents, rules := newMutingEngine(n)
	other := svcBlock("b1")
	other.Conditions[0].Value = "another-service"
	rules.set(other)
	handle(t, e, item("ALT1", 0, "critical"))
	e.DeliverDue(context.Background())
	if n.csmCalls != 1 {
		t.Fatalf("csm = %d, want a non-matching incident delivered", n.csmCalls)
	}

	// A rule that starts after delivery doesn't silence the incident's later notes.
	rules.set(svcBlock("b2"))
	handle(t, e, item("ALT2", time.Second, "critical"))
	e.DeliverDue(context.Background())
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if inc.Muted || len(n.pushed) != 1 {
		t.Fatalf("incident = %+v pushed = %v, want the delivered incident left alone", inc, n.pushed)
	}
}

func TestMute_OffWithoutCSM(t *testing.T) {
	n := &fakeNotifier{csmDisabled: true, chatOK: true}
	e, incidents, rules := newMutingEngine(n)
	rules.set(svcBlock("b1"))
	handle(t, e, item("ALT1", 0, "critical"))
	e.DeliverDue(context.Background())
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if inc.Muted || n.chatCalls != 1 {
		t.Fatalf("incident = %+v chat = %d, want no muting without CSM", inc, n.chatCalls)
	}
}

func TestMute_ChatFallbackGraceCountsFromRelease(t *testing.T) {
	n := &fakeNotifier{csmOK: false, chatOK: true}
	e, incidents, rules := newMutingEngine(n)
	e.cfg.ChatFallbackDelay = time.Minute
	rules.set(svcBlock("b1"))
	handle(t, e, item("ALT1", 0, "critical"))
	e.DeliverDue(context.Background())
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	incidents.age(inc.ID, time.Hour) // muted for an hour

	rules.set()
	e.DeliverDue(context.Background())
	if n.csmCalls != 1 || n.chatCalls != 0 {
		t.Fatalf("csm = %d chat = %d, want CSM tried and Chat held for the grace period after release", n.csmCalls, n.chatCalls)
	}
}
