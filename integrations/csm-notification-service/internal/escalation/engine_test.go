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

package escalation

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

const testIncidentID = "11111111-1111-1111-1111-111111111111"

// --- fakes -----------------------------------------------------------------

// memStore is an in-memory ladderStore. Deliberately not a Redis mock: the
// engine's contract is "create-if-absent, read, write, schedule, retire", and
// exercising that against a map keeps these tests about the ladder's behaviour
// rather than about go-redis.
type memStore struct {
	states   map[string]LadderState
	wakes    map[string]time.Time
	failSave bool
}

func newMemStore() *memStore {
	return &memStore{states: map[string]LadderState{}, wakes: map[string]time.Time{}}
}

func (m *memStore) Create(_ context.Context, id string, st LadderState) (bool, error) {
	if _, exists := m.states[id]; exists {
		return false, nil
	}
	m.states[id] = st
	return true, nil
}

func (m *memStore) Save(_ context.Context, id string, st LadderState) error {
	if m.failSave {
		return errors.New("save failed")
	}
	m.states[id] = st
	return nil
}

func (m *memStore) Get(_ context.Context, id string) (LadderState, bool, error) {
	st, ok := m.states[id]
	return st, ok, nil
}

func (m *memStore) Delete(_ context.Context, id string) error {
	delete(m.states, id)
	return nil
}

func (m *memStore) AddWake(_ context.Context, member string, at time.Time) error {
	m.wakes[member] = at
	return nil
}

func (m *memStore) RemoveWakes(_ context.Context, members ...string) error {
	for _, member := range members {
		delete(m.wakes, member)
	}
	return nil
}

func (m *memStore) DueMembers(_ context.Context, now time.Time) ([]string, error) {
	var due []string
	for member, at := range m.wakes {
		if !at.After(now) {
			due = append(due, member)
		}
	}
	sort.Slice(due, func(i, j int) bool { return m.wakes[due[i]].Before(m.wakes[due[j]]) })
	return due, nil
}

// placedCall records one dialled call.
type placedCall struct {
	to   string
	ssml bool
}

type fakeCaller struct {
	placed []placedCall
	err    error
}

func (f *fakeCaller) MakeSSMLCall(_ context.Context, to string, _ notifications.Speech) (notifications.Call, error) {
	if f.err != nil {
		return notifications.Call{}, f.err
	}
	f.placed = append(f.placed, placedCall{to: to, ssml: true})
	return notifications.Call{SID: "CAtest", Status: "queued"}, nil
}

func (f *fakeCaller) MakeCall(_ context.Context, to, _ string) (notifications.Call, error) {
	if f.err != nil {
		return notifications.Call{}, f.err
	}
	f.placed = append(f.placed, placedCall{to: to})
	return notifications.Call{SID: "CAtest", Status: "queued"}, nil
}

type fakeNotes struct {
	notes []string
	err   error
}

func (f *fakeNotes) AppendWorkNote(_ context.Context, _, note string) error {
	if f.err != nil {
		return f.err
	}
	f.notes = append(f.notes, note)
	return nil
}

// testEngine wires an engine over the fakes above.
// testClock is "now" for every engine test: the same instant the tests report
// their incidents at, so a trigger is fresh rather than — as the real clock
// would see a fixed 2026-09-09 date — days stale and dropped by start's
// guard. Tests of that guard set their own clock.
var testClock = ist(2026, 9, 9, 10, 0)

func testEngine(store ladderStore, caller callPlacer, notes incidentNotes, cfg EngineConfig) *Engine {
	return &Engine{
		policies: DefaultPolicy,
		resolver: fullResolver(),
		calls:    caller,
		store:    store,
		notes:    notes,
		cfg:      cfg,
		clock:    func() time.Time { return testClock },
	}
}

func enabled() EngineConfig { return EngineConfig{CallSendingEnabled: true} }

// record builds a Kafka record carrying one event.
func record(t *testing.T, typ events.Type, payload any) eventbus.Record {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(events.Envelope{Type: typ, EntityID: testIncidentID, Payload: raw})
	if err != nil {
		t.Fatal(err)
	}
	return eventbus.Record{Value: body}
}

// createdEvent is an incident.created carrying the full escalation payload.
func createdEvent(t *testing.T, priority string, at time.Time) eventbus.Record {
	t.Helper()
	return record(t, events.TypeIncidentCreated, events.IncidentCreatedPayload{
		Title:            "Gateway returning 500s in production",
		ShortDescription: "Every request is failing",
		Number:           "INC0012345",
		Priority:         priority,
		Account:          "Automation Test Account",
		Team:             "Atlas",
		Product:          "WSO2 API Manager",
		ABTEligible:      abtYes(),
		ReportedAt:       at.Format(time.RFC3339),
	})
}

// --- tests -----------------------------------------------------------------

// The whole point of the change: a created incident now schedules the ladder
// section 7.0 defines, rather than the single call it used to get.
func TestEngine_IncidentCreatedSchedulesTheLadder(t *testing.T) {
	store, caller := newMemStore(), &fakeCaller{}
	e := testEngine(store, caller, &fakeNotes{}, enabled())

	// 10:00 IST on a Wednesday is business hours, so no LEVEL_0 (R1).
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}

	st, found, _ := store.Get(context.Background(), testIncidentID)
	if !found {
		t.Fatal("expected a ladder to be stored")
	}
	// CRITICAL is P1: four levels of 3 calls, one recipient each, LEVEL_0
	// skipped during business hours.
	if len(st.Plan.Calls) != 12 {
		t.Errorf("scheduled %d calls, want 12 (LEVEL_1..LEVEL_4 x 3 attempts)", len(st.Plan.Calls))
	}
	if len(store.wakes) != len(st.Plan.Calls) {
		t.Errorf("scheduled %d wake entries for %d calls", len(store.wakes), len(st.Plan.Calls))
	}
	if st.Plan.Calls[0].Level != Level1 {
		t.Errorf("ladder starts at %s, want LEVEL_1 during business hours", st.Plan.Calls[0].Level)
	}
	// The first call is the priority's own initial wait after the report, not
	// after consume time.
	if want := at.Add(6 * time.Minute); !st.Plan.Calls[0].At.Equal(want) {
		t.Errorf("first call at %s, want %s (P1's 6-minute initial wait from the report)",
			st.Plan.Calls[0].At, want)
	}
	if len(caller.placed) != 0 {
		t.Errorf("scheduling must not dial anything yet, placed %d", len(caller.placed))
	}
}

// Kafka is at-least-once. Restarting a ladder on a redelivery would re-dial
// everyone from the first level, so a duplicate trigger must be ignored.
func TestEngine_RedeliveredCreatedDoesNotRestartTheLadder(t *testing.T) {
	store, caller := newMemStore(), &fakeCaller{}
	e := testEngine(store, caller, &fakeNotes{}, enabled())
	at := ist(2026, 9, 9, 10, 0)
	rec := createdEvent(t, "CRITICAL", at)

	if err := e.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	// Place the first call, so the ladder is genuinely part-way through.
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	placedBefore := len(caller.placed)

	if err := e.Handle(context.Background(), rec); err != nil {
		t.Fatalf("a redelivered trigger must not error: %v", err)
	}
	st, _, _ := store.Get(context.Background(), testIncidentID)
	if !st.Placed[0] {
		t.Error("the redelivery reset the ladder's progress")
	}
	if len(caller.placed) != placedBefore {
		t.Errorf("the redelivery dialled again: %d calls, want %d", len(caller.placed), placedBefore)
	}
}

// Tick places what is due and nothing more, and a second tick at the same
// instant must not repeat a call already placed.
func TestEngine_TickPlacesDueCallsOnceEach(t *testing.T) {
	store, caller := newMemStore(), &fakeCaller{}
	e := testEngine(store, caller, &fakeNotes{}, enabled())
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}

	// P1's LEVEL_1 attempts are at +6, +8 and +10 minutes.
	if err := e.Tick(context.Background(), at.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(caller.placed) != 2 {
		t.Fatalf("placed %d calls by T+9m, want 2 (LEVEL_1 attempts 1 and 2)", len(caller.placed))
	}
	if caller.placed[0].to != "+94770000001" {
		t.Errorf("called %s, want the LEVEL_1 recipient", caller.placed[0].to)
	}

	if err := e.Tick(context.Background(), at.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(caller.placed) != 2 {
		t.Errorf("a repeated tick re-dialled: %d calls, want 2", len(caller.placed))
	}
}

// Acknowledgement is the only thing that stops a ladder. It must drop every
// outstanding call and record what happened.
func TestEngine_AcknowledgementCancelsAndWritesTheSummary(t *testing.T) {
	store, caller, notes := newMemStore(), &fakeCaller{}, &fakeNotes{}
	e := testEngine(store, caller, notes, enabled())
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}

	ack := record(t, events.TypeIncidentAcknowledged, events.IncidentAcknowledgedPayload{
		PreviousState: "NEW", NewState: "IN_PROGRESS",
	})
	if err := e.Handle(context.Background(), ack); err != nil {
		t.Fatal(err)
	}

	if len(store.wakes) != 0 {
		t.Errorf("%d calls still scheduled after acknowledgement", len(store.wakes))
	}
	if _, found, _ := store.Get(context.Background(), testIncidentID); found {
		t.Error("the ladder's state should be cleared once its summary is written")
	}
	if len(notes.notes) != 1 {
		t.Fatalf("wrote %d work notes, want 1", len(notes.notes))
	}
	if !containsAll(notes.notes[0], "Execution Summary Of the Escalation Flow", "call(s) cancelled") {
		t.Errorf("the work note does not read like section 11.0's:\n%s", notes.notes[0])
	}

	// No further calls go out afterwards.
	before := len(caller.placed)
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(caller.placed) != before {
		t.Errorf("the ladder kept calling after acknowledgement: %d, want %d", len(caller.placed), before)
	}
}

// The gap this closes. An elevation-triggered ladder had no stop signal at
// all: section 10.0's voice message tells its recipient to add a public
// comment, and the incident has normally already left NEW, so
// incident.acknowledged can never fire for it again. Without this the ladder
// ran all the way to the Head of CRE no matter what anyone did.
func TestEngine_PublicCommentStopsAnElevationLadder(t *testing.T) {
	store, caller, notes := newMemStore(), &fakeCaller{}, &fakeNotes{}
	e := testEngine(store, caller, notes, enabled())
	at := ist(2026, 9, 9, 10, 0)

	elevated := record(t, events.TypeIncidentPriorityElevated, events.IncidentPriorityElevatedPayload{
		OldPriority: "MODERATE",
		NewPriority: "CRITICAL",
		Number:      "INC0012345",
		Team:        "Atlas",
		ElevatedAt:  at.Format(time.RFC3339),
	})
	if err := e.Handle(context.Background(), elevated); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	placedBefore := len(caller.placed)
	if placedBefore == 0 {
		t.Fatal("expected the elevation ladder to have started calling")
	}

	comment := record(t, events.TypeIncidentCommentAdded, events.IncidentCommentAddedPayload{
		CommentID: "c-1", IsPublic: true,
	})
	if err := e.Handle(context.Background(), comment); err != nil {
		t.Fatal(err)
	}

	if len(store.wakes) != 0 {
		t.Errorf("%d calls still scheduled after a public comment", len(store.wakes))
	}
	if _, found, _ := store.Get(context.Background(), testIncidentID); found {
		t.Error("the ladder should stop being tracked once its summary is written")
	}
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(caller.placed) != placedBefore {
		t.Errorf("the ladder kept calling after the comment: %d, want %d", len(caller.placed), placedBefore)
	}
	if len(notes.notes) != 1 {
		t.Fatalf("wrote %d work notes, want 1", len(notes.notes))
	}
	// The summary must say which gesture stopped it, not just that something did.
	if !containsAll(notes.notes[0], "Public comment added", "call(s) cancelled") {
		t.Errorf("the work note does not name the gesture that stopped the ladder:\n%s", notes.notes[0])
	}
}

// A work note is an internal jotting, not section 3.0's acknowledgement
// gesture. Somebody writing one while triaging must not silence their own
// pager.
func TestEngine_WorkNoteDoesNotStopTheLadder(t *testing.T) {
	store, caller := newMemStore(), &fakeCaller{}
	e := testEngine(store, caller, &fakeNotes{}, enabled())
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	scheduled := len(store.wakes)

	workNote := record(t, events.TypeIncidentCommentAdded, events.IncidentCommentAddedPayload{
		CommentID: "c-1", IsPublic: false,
	})
	if err := e.Handle(context.Background(), workNote); err != nil {
		t.Fatal(err)
	}
	if len(store.wakes) != scheduled {
		t.Errorf("a work note cancelled %d calls; it must cancel none", scheduled-len(store.wakes))
	}
	if _, found, _ := store.Get(context.Background(), testIncidentID); !found {
		t.Error("a work note must leave the ladder running")
	}
}

// Both of section 3.0's gestures stop any running ladder, not only the one
// their own trigger started — a responder who comments on a newly reported
// incident rather than moving it to Work In Progress is just as visibly
// attending to it.
func TestEngine_PublicCommentAlsoStopsANewIncidentLadder(t *testing.T) {
	store, notes := newMemStore(), &fakeNotes{}
	e := testEngine(store, &fakeCaller{}, notes, enabled())
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	comment := record(t, events.TypeIncidentCommentAdded, events.IncidentCommentAddedPayload{
		CommentID: "c-1", IsPublic: true,
	})
	if err := e.Handle(context.Background(), comment); err != nil {
		t.Fatal(err)
	}
	if len(store.wakes) != 0 {
		t.Errorf("%d calls still scheduled after a public comment", len(store.wakes))
	}
	if len(notes.notes) != 1 {
		t.Error("expected the execution summary to be written")
	}
}

// A comment on an incident with no running ladder is the overwhelmingly common
// case — every ordinary comment on every incident — and must be free.
func TestEngine_PublicCommentWithNoLadderIsANoOp(t *testing.T) {
	store, notes := newMemStore(), &fakeNotes{}
	e := testEngine(store, &fakeCaller{}, notes, enabled())
	comment := record(t, events.TypeIncidentCommentAdded, events.IncidentCommentAddedPayload{
		CommentID: "c-1", IsPublic: true,
	})
	if err := e.Handle(context.Background(), comment); err != nil {
		t.Fatalf("expected a no-op, got %v", err)
	}
	if len(notes.notes) != 0 {
		t.Error("nothing should be written for an incident with no ladder")
	}
}

// Acknowledging an incident that never had a ladder is the common case and
// must not error — it would dead-letter a valid event.
func TestEngine_AcknowledgementWithNoLadderIsANoOp(t *testing.T) {
	store, notes := newMemStore(), &fakeNotes{}
	e := testEngine(store, &fakeCaller{}, notes, enabled())
	ack := record(t, events.TypeIncidentAcknowledged, events.IncidentAcknowledgedPayload{
		PreviousState: "NEW", NewState: "IN_PROGRESS",
	})
	if err := e.Handle(context.Background(), ack); err != nil {
		t.Fatalf("expected a no-op, got %v", err)
	}
	if len(notes.notes) != 0 {
		t.Error("nothing should be written for an incident with no ladder")
	}
}

// A priority elevation is its own trigger with its own, faster timings, so it
// replaces whatever is running rather than being ignored as a duplicate.
func TestEngine_PriorityElevationReplacesTheRunningLadder(t *testing.T) {
	store, caller := newMemStore(), &fakeCaller{}
	e := testEngine(store, caller, &fakeNotes{}, enabled())
	at := ist(2026, 9, 9, 10, 0)

	if err := e.Handle(context.Background(), createdEvent(t, "LOW", at)); err != nil {
		t.Fatal(err)
	}
	slow, _, _ := store.Get(context.Background(), testIncidentID)
	slowFirst := slow.Plan.Calls[0].At

	elevated := record(t, events.TypeIncidentPriorityElevated, events.IncidentPriorityElevatedPayload{
		OldPriority: "LOW",
		NewPriority: "CRITICAL",
		Title:       "Gateway returning 500s in production",
		Number:      "INC0012345",
		Team:        "Atlas",
		ABTEligible: abtYes(),
		ElevatedAt:  at.Add(time.Minute).Format(time.RFC3339),
	})
	if err := e.Handle(context.Background(), elevated); err != nil {
		t.Fatal(err)
	}

	fast, found, _ := store.Get(context.Background(), testIncidentID)
	if !found {
		t.Fatal("expected the replacement ladder to be stored")
	}
	if fast.Plan.Trigger.Kind != TriggerPriorityElevated {
		t.Errorf("stored trigger is %s, want a priority elevation", fast.Plan.Trigger.Kind)
	}
	if !fast.Plan.Calls[0].At.Before(slowFirst) {
		t.Errorf("the elevated ladder's first call is %s, no sooner than LOW's %s",
			fast.Plan.Calls[0].At, slowFirst)
	}
	// The old ladder's calls must not still be scheduled alongside the new
	// ones, or the incident gets both ladders.
	if len(store.wakes) != len(fast.Plan.Calls) {
		t.Errorf("%d wake entries for a %d-call ladder; the old ladder was not retired",
			len(store.wakes), len(fast.Plan.Calls))
	}
}

// Section 7.0 has no row below P4, so a planning-priority incident has no
// ladder. That is a skip, not an error — erroring would dead-letter it.
func TestEngine_PriorityWithNoPolicyIsSkipped(t *testing.T) {
	store := newMemStore()
	e := testEngine(store, &fakeCaller{}, &fakeNotes{}, enabled())
	if err := e.Handle(context.Background(), createdEvent(t, "PLANNING", ist(2026, 9, 9, 10, 0))); err != nil {
		t.Fatalf("expected a skip, got %v", err)
	}
	if len(store.states) != 0 {
		t.Error("a priority with no policy must not create a ladder")
	}
}

// A publisher that has not been updated still emits incident.created with no
// priority. That must keep working exactly as before, not error.
func TestEngine_CreatedWithoutEscalationFieldsIsSkipped(t *testing.T) {
	store := newMemStore()
	e := testEngine(store, &fakeCaller{}, &fakeNotes{}, enabled())
	legacy := record(t, events.TypeIncidentCreated, events.IncidentCreatedPayload{
		Title:            "Gateway returning 500s",
		ShortDescription: "Every request is failing",
	})
	if err := e.Handle(context.Background(), legacy); err != nil {
		t.Fatalf("a payload without escalation fields must not error: %v", err)
	}
	if len(store.states) != 0 {
		t.Error("no ladder should be scheduled without a priority")
	}
}

// This engine shares a topic with every other event in the service. Anything
// it does not own must be a silent no-op, or it burns its own retries and
// dead-letters a record that was never broken.
func TestEngine_IgnoresEventsItDoesNotOwn(t *testing.T) {
	e := testEngine(newMemStore(), &fakeCaller{}, &fakeNotes{}, enabled())
	for _, r := range []eventbus.Record{
		record(t, events.TypeCaseCreated, map[string]string{"anything": "at all"}),
		record(t, events.TypeSLATierReached, map[string]string{"anything": "at all"}),
	} {
		if err := e.Handle(context.Background(), r); err != nil {
			t.Errorf("expected a no-op for another consumer's event, got %v", err)
		}
	}
}

// A ladder nobody answers runs to its end, records that, and stops being
// tracked.
func TestEngine_ExhaustedLadderWritesTheSummaryAndStopsTracking(t *testing.T) {
	store, caller, notes := newMemStore(), &fakeCaller{}, &fakeNotes{}
	e := testEngine(store, caller, notes, enabled())
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}

	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(caller.placed) != 12 {
		t.Errorf("placed %d calls, want the whole 12-call ladder", len(caller.placed))
	}
	if _, found, _ := store.Get(context.Background(), testIncidentID); found {
		t.Error("an exhausted ladder should stop being tracked")
	}
	if len(notes.notes) != 1 {
		t.Fatalf("wrote %d work notes, want 1", len(notes.notes))
	}
	if containsAll(notes.notes[0], "cancelled") {
		t.Error("an exhausted ladder's summary must not claim calls were cancelled")
	}
}

// A failed call leaves its wake entry in place so the next tick retries it.
// This is a paging system: a duplicate call costs far less than a lost page.
func TestEngine_FailedCallIsRetriedNextTick(t *testing.T) {
	store, caller := newMemStore(), &fakeCaller{err: errors.New("twilio is down")}
	e := testEngine(store, caller, &fakeNotes{}, enabled())
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}

	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err == nil {
		t.Fatal("expected the failed call to be reported")
	}
	st, _, _ := store.Get(context.Background(), testIncidentID)
	if st.Placed[0] {
		t.Error("a call that failed must not be recorded as placed")
	}
	if _, scheduled := store.wakes[wakeMember(testIncidentID, 0)]; !scheduled {
		t.Error("a failed call must stay scheduled so the next tick retries it")
	}

	caller.err = nil
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(caller.placed) != 1 {
		t.Errorf("the retry placed %d calls, want 1", len(caller.placed))
	}
}

// The killswitch silences the dialling without stopping the ladder, so a
// deployment can watch a real run end to end without paging anyone.
func TestEngine_CallSendingDisabledStillRunsTheLadder(t *testing.T) {
	store, caller, notes := newMemStore(), &fakeCaller{}, &fakeNotes{}
	e := testEngine(store, caller, notes, EngineConfig{CallSendingEnabled: false})
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(caller.placed) != 0 {
		t.Errorf("dialled %d calls with sending disabled, want 0", len(caller.placed))
	}
	if len(notes.notes) != 1 {
		t.Error("the ladder should still record its execution summary")
	}
}

// SSML is opt-in, and picking it must change which call path is used.
func TestEngine_UsesSSMLWhenConfigured(t *testing.T) {
	store, caller := newMemStore(), &fakeCaller{}
	e := testEngine(store, caller, &fakeNotes{}, EngineConfig{CallSendingEnabled: true, UseSSML: true})
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(caller.placed) != 1 || !caller.placed[0].ssml {
		t.Errorf("expected one SSML call, got %+v", caller.placed)
	}
}

// A level whose recipients have no phone number must not abort the ladder —
// section 11.0 logs it as NO_NUMBER and carries on.
func TestEngine_LevelWithNoReachableRecipientIsSkippedNotFatal(t *testing.T) {
	store, caller := newMemStore(), &fakeCaller{}
	e := testEngine(store, caller, &fakeNotes{}, enabled())
	e.resolver = StaticResolver{ByLevel: map[Level][]Recipient{
		Level1: {rec("no.number@wso2.com", "")},
		Level2: {rec("team.lead@wso2.com", "+94770000002")},
		Level3: {rec("bu.head@wso2.com", "+94770000003")},
		Level4: {rec("cre.head@wso2.com", "+94770000004")},
	}}
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	st, found, _ := store.Get(context.Background(), testIncidentID)
	if !found {
		t.Fatal("expected a ladder despite one unreachable level")
	}
	if len(st.Plan.Calls) != 9 {
		t.Errorf("scheduled %d calls, want 9 (LEVEL_1 unreachable, three levels of 3)", len(st.Plan.Calls))
	}
	for _, c := range st.Plan.Calls {
		if c.Level == Level1 {
			t.Error("a recipient with no number must not be called")
		}
	}
}

// containsAll reports whether s contains every substring.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// A deployment without entity-service access still runs a real ladder; the
// summary is logged instead of written back. Guards the typed-nil-in-interface
// trap in NewEngine: a nil *EntityClient assigned straight into the
// incidentNotes field would make writeNote call a nil receiver and panic.
func TestNewEngine_NilNotesClientDoesNotPanic(t *testing.T) {
	e := NewEngine(DefaultPolicy, fullResolver(), nil, nil, nil, enabled())
	if e.notes != nil {
		t.Fatal("a nil *EntityClient must leave the interface field nil, not hold a nil pointer")
	}
	// writeNote is the path that would panic; it must log and return instead.
	if err := e.writeNote(context.Background(), Plan{}, nil, nil, nil, ""); err != nil {
		t.Errorf("writeNote with no client should be a no-op, got %v", err)
	}
}

// The work note must report what was dialled, not what was scheduled.
//
// A cancellation and a call due at the same instant race, and the
// cancellation wins — it drops the wake entries before that tick places
// anything. Reporting by scheduled time alone wrote a summary claiming a call
// that had been retired microseconds earlier and never happened, which was
// found by running the engine on a compressed clock where that race is
// common rather than vanishingly rare.
func TestEngine_SummaryReportsOnlyCallsActuallyPlaced(t *testing.T) {
	store, caller, notes := newMemStore(), &fakeCaller{}, &fakeNotes{}
	e := testEngine(store, caller, notes, enabled())
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	// Place the first two LEVEL_1 attempts (+6m, +8m), leaving the third
	// (+10m) scheduled but not yet dialled.
	if err := e.Tick(context.Background(), at.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	placed := len(caller.placed)

	// Acknowledge at an instant AFTER the third attempt's scheduled time, so
	// a summary driven by scheduled time would list it even though the
	// cancellation retired it first.
	st, _, _ := store.Get(context.Background(), testIncidentID)
	st.Cancelled = func() *time.Time { t := at.Add(30 * time.Minute); return &t }()
	st.CancelReason = "Acknowledged"
	if err := store.Save(context.Background(), testIncidentID, st); err != nil {
		t.Fatal(err)
	}

	lines := st.Plan.ExecutionSummary(st.Placed, st.Failed, st.Cancelled, st.CancelReason)
	calls := 0
	for _, l := range lines {
		if strings.Contains(l, "[OK][Call]") {
			calls++
		}
	}
	if calls != placed {
		t.Errorf("summary reports %d calls, but only %d were placed:\n%s",
			calls, placed, strings.Join(lines, "\n"))
	}

	// The closing count has to agree with the lines: every call is either
	// dialled or cancelled, and a call due at the cancellation instant used to
	// fall out of both totals.
	dropped := 0
	for _, l := range lines {
		if !strings.Contains(l, "call(s) cancelled]") {
			continue
		}
		for _, f := range strings.Fields(l) {
			if n, err := strconv.Atoi(f); err == nil {
				dropped = n
			}
		}
	}
	if calls+dropped != len(st.Plan.Calls) {
		t.Errorf("%d dialled + %d cancelled = %d, but the plan has %d calls",
			calls, dropped, calls+dropped, len(st.Plan.Calls))
	}

	// Without the flags the old approximation still applies, for a caller
	// that genuinely has no engine (cmd/ladder-harness).
	approx := st.Plan.ExecutionSummary(nil, nil, st.Cancelled, st.CancelReason)
	approxCalls := 0
	for _, l := range approx {
		if strings.Contains(l, "[OK][Call]") {
			approxCalls++
		}
	}
	if approxCalls <= calls {
		t.Errorf("expected the flag-less approximation to over-report; got %d vs %d", approxCalls, calls)
	}
}

// The first deployment of this engine replays the whole topic (its consumer
// group is new, and eventbus reads a new group from the first offset), and a
// DLQ retry or a long outage can hand it an old trigger any time after. A
// trigger whose ladder has already run its course must be dropped, or the
// next tick burst-dials every rung at once for an incident nobody is waiting
// on.
func TestEngine_TriggerOlderThanItsLadderIsDropped(t *testing.T) {
	store, caller := newMemStore(), &fakeCaller{}
	e := testEngine(store, caller, &fakeNotes{}, enabled())
	reported := ist(2026, 9, 9, 10, 0)
	// Consumed a day later: every one of P1's calls is long past.
	e.clock = func() time.Time { return reported.Add(24 * time.Hour) }

	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", reported)); err != nil {
		t.Fatalf("a stale trigger must be skipped, not errored: %v", err)
	}
	if len(store.states) != 0 || len(store.wakes) != 0 {
		t.Errorf("a stale trigger scheduled a ladder: %d states, %d wakes", len(store.states), len(store.wakes))
	}
	if err := e.Tick(context.Background(), reported.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(caller.placed) != 0 {
		t.Errorf("a stale trigger dialled %d calls", len(caller.placed))
	}
}

// A short backlog is the opposite case and must still work: the ladder is
// measured from the report time precisely so that a delayed consume catches
// up to the rung it should be on, rather than starting late.
func TestEngine_ShortBacklogStillSchedulesTheRemainder(t *testing.T) {
	store, caller := newMemStore(), &fakeCaller{}
	e := testEngine(store, caller, &fakeNotes{}, enabled())
	reported := ist(2026, 9, 9, 10, 0)
	// Consumed ten minutes late: P1's LEVEL_1 has started, LEVEL_2..4 have not.
	e.clock = func() time.Time { return reported.Add(10 * time.Minute) }

	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", reported)); err != nil {
		t.Fatal(err)
	}
	st, found, _ := store.Get(context.Background(), testIncidentID)
	if !found {
		t.Fatal("a ten-minute backlog must still schedule the ladder")
	}
	if len(st.Plan.Calls) != 12 {
		t.Errorf("scheduled %d calls, want the full 12; offsets are from the report time", len(st.Plan.Calls))
	}
	// The calls already due go out on the next tick, catching the ladder up.
	if err := e.Tick(context.Background(), reported.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(caller.placed) != 3 {
		t.Errorf("placed %d calls on catch-up, want 3 (LEVEL_1's attempts at +6, +8, +10)", len(caller.placed))
	}
}

// Found by a live run: Twilio rejected a call outright (a 4xx — an unverified
// number on a trial account), and the engine retried it every tick for the
// rest of the ladder's life. Since the call was never "placed", the ladder
// could never complete either. A rejection the provider will repeat for the
// same request is a permanent failure: record it, list it in the work note
// the way section 11.0 lists a missing number, and let the ladder move on.
func TestEngine_ProviderRejectionIsRecordedNotRetried(t *testing.T) {
	store, notes := newMemStore(), &fakeNotes{}
	caller := &fakeCaller{err: &apierror.Error{StatusCode: 400,
		Body: `{"code":21219,"message":"The number is unverified.","status":400}`}}
	e := testEngine(store, caller, notes, enabled())
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}

	// The first LEVEL_1 attempt comes due and is rejected.
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatalf("a permanent rejection must not surface as a tick error (it would retry): %v", err)
	}
	st, _, _ := store.Get(context.Background(), testIncidentID)
	if st.Placed[0] {
		t.Error("a rejected call must not be recorded as placed")
	}
	if got := st.failure(0); got != "REJECTED_400_21219" {
		t.Errorf("failure reason = %q, want REJECTED_400_21219", got)
	}
	if _, still := store.wakes[wakeMember(testIncidentID, 0)]; still {
		t.Error("a rejected call must be retired from the wake index, not retried")
	}

	// Every later call is rejected the same way; the ladder must still run
	// its course and write its summary rather than sit in Redis forever.
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get(context.Background(), testIncidentID); found {
		t.Error("a ladder whose every call was rejected must still complete")
	}
	if len(notes.notes) != 1 {
		t.Fatalf("wrote %d work notes, want 1", len(notes.notes))
	}
	if !strings.Contains(notes.notes[0], "[LEVEL_1][ERROR][CALL_FAILED][sub.lead@wso2.com][REJECTED_400_21219]") {
		t.Errorf("the work note does not report the rejection:\n%s", notes.notes[0])
	}
	if strings.Contains(notes.notes[0], "[OK][Call]") {
		t.Error("the work note claims a call was placed; none were")
	}
}

// A transient failure is the other case and must keep the old behaviour: the
// call stays scheduled and the next tick retries it.
func TestEngine_TransientFailureIsStillRetried(t *testing.T) {
	store := newMemStore()
	caller := &fakeCaller{err: &apierror.Error{StatusCode: 503, Body: "service unavailable"}}
	e := testEngine(store, caller, &fakeNotes{}, enabled())
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err == nil {
		t.Fatal("a transient failure must surface so the tick is retried")
	}
	st, _, _ := store.Get(context.Background(), testIncidentID)
	if st.failure(0) != "" {
		t.Error("a 5xx must not be recorded as a permanent failure")
	}
	if _, still := store.wakes[wakeMember(testIncidentID, 0)]; !still {
		t.Error("a transiently failed call must stay scheduled")
	}
}
