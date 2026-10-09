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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"alert-core-service/internal/model"
	"alert-core-service/internal/store"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeIncidents mirrors store.IncidentRepo's SQL semantics in memory.
type fakeIncidents struct {
	mu        sync.Mutex
	incidents map[int64]*model.Incident
	due       map[int64]*time.Time
	notes     []model.Note
	noteInc   map[int64]int64 // note id -> incident id
	nextID    int64
	locked    map[int64]bool
	folds     int
}

func newFakeIncidents() *fakeIncidents {
	return &fakeIncidents{incidents: map[int64]*model.Incident{}, due: map[int64]*time.Time{}, noteInc: map[int64]int64{}, locked: map[int64]bool{}}
}

func (f *fakeIncidents) Fold(_ context.Context, fp string, alertIDs []string, decide store.Decide) (store.FoldPlan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.folds++
	var current *model.Incident
	for _, inc := range f.incidents {
		if inc.Fingerprint == fp && (current == nil || inc.FirstSeen.After(current.FirstSeen)) {
			c := *inc
			current = &c
		}
	}
	recorded := map[string]bool{}
	for _, n := range f.notes {
		if slices.Contains(alertIDs, n.AlertID) {
			recorded[n.AlertID] = true
		}
	}
	plan := decide(current, recorded)
	now := time.Now()
	if c := plan.Current; c != nil {
		inc := f.incidents[c.ID]
		inc.AlertCount += c.Added
		if c.LastSeen.After(inc.LastSeen) {
			inc.LastSeen = c.LastSeen
		}
		if inc.Category == "" {
			inc.Category = c.Category
		}
		inc.FoldVersion++
		f.due[c.ID] = &now
		f.addNotes(c.ID, c.Notes)
	}
	for _, n := range plan.New {
		f.nextID++
		inc := n.Incident
		inc.ID = f.nextID
		inc.CreatedAt = now
		f.incidents[inc.ID] = &inc
		f.due[inc.ID] = &now
		f.addNotes(inc.ID, n.Notes)
	}
	return plan, nil
}

func (f *fakeIncidents) addNotes(incident int64, notes []model.Note) {
	for _, n := range notes {
		if slices.ContainsFunc(f.notes, func(e model.Note) bool { return e.AlertID == n.AlertID }) {
			continue
		}
		n.ID = int64(len(f.notes) + 1)
		n.CSMPending = true
		f.notes = append(f.notes, n)
		f.noteInc[n.ID] = incident
	}
}

func (f *fakeIncidents) ListDue(_ context.Context, limit int) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []int64
	for id, at := range f.due {
		if at != nil && !at.After(time.Now()) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids[:min(limit, len(ids))], nil
}

func (f *fakeIncidents) TryLock(_ context.Context, id int64) (func(), bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.locked[id] {
		return nil, false, nil
	}
	f.locked[id] = true
	return func() { f.mu.Lock(); delete(f.locked, id); f.mu.Unlock() }, true, nil
}

func (f *fakeIncidents) Get(_ context.Context, id int64) (model.Incident, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc, ok := f.incidents[id]
	if !ok {
		return model.Incident{}, false, nil
	}
	return *inc, true, nil
}

func (f *fakeIncidents) PendingNotes(_ context.Context, id int64, limit int) ([]model.Note, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.Note
	for _, n := range f.notes {
		if f.noteInc[n.ID] == id && (n.CSMPending || n.ChatPending) && len(out) < limit {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *fakeIncidents) update(id int64, fn func(*model.Incident)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f.incidents[id])
	return nil
}

func (f *fakeIncidents) RecordCSMAttemptStarted(_ context.Context, id int64, attempts int) error {
	return f.update(id, func(i *model.Incident) { i.CSMAttempts, i.CSMLastAttemptAt = attempts, time.Now() })
}

func (f *fakeIncidents) RecordCSMIncident(_ context.Context, id int64, csmID, number string) error {
	return f.update(id, func(i *model.Incident) {
		i.IncidentID, i.IncidentNumber, i.CSMConfirmed, i.Status, i.StateCheckedAt = csmID, number, true, "open", time.Now()
	})
}

func (f *fakeIncidents) RecordCSMAttemptFailure(_ context.Context, id int64, permanent bool) error {
	return f.update(id, func(i *model.Incident) { i.CSMPermanentlyFailed = permanent })
}

func (f *fakeIncidents) MarkFallback(_ context.Context, id int64) error {
	return f.update(id, func(i *model.Incident) { i.Fallback = true })
}

func (f *fakeIncidents) SyncStatus(_ context.Context, id int64, status string, checkedAt time.Time) error {
	return f.update(id, func(i *model.Incident) { i.Status, i.StateCheckedAt = status, checkedAt })
}

func (f *fakeIncidents) ClearNotes(_ context.Context, _ int64, noteIDs []int64, csm, chat bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.notes {
		if slices.Contains(noteIDs, f.notes[i].ID) {
			f.notes[i].CSMPending = f.notes[i].CSMPending && !csm
			f.notes[i].ChatPending = f.notes[i].ChatPending && !chat
		}
	}
	return nil
}

func (f *fakeIncidents) SettleNotes(_ context.Context, incident int64, csm, chat bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.notes {
		if f.noteInc[f.notes[i].ID] == incident {
			f.notes[i].CSMPending = f.notes[i].CSMPending && !csm
			f.notes[i].ChatPending = f.notes[i].ChatPending && !chat
		}
	}
	return nil
}

func (f *fakeIncidents) FinishDelivery(_ context.Context, id, foldVersion int64, next *time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.incidents[id].FoldVersion != foldVersion {
		now := time.Now()
		f.due[id] = &now
		return nil
	}
	f.due[id] = next
	return nil
}

// byFingerprint returns a fingerprint's incidents, oldest first.
func (f *fakeIncidents) byFingerprint(fp string) []model.Incident {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.Incident
	for _, inc := range f.incidents {
		if inc.Fingerprint == fp {
			out = append(out, *inc)
		}
	}
	slices.SortFunc(out, func(a, b model.Incident) int { return a.FirstSeen.Compare(b.FirstSeen) })
	return out
}

func (f *fakeIncidents) notesOf(id int64) []model.Note {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.Note
	for _, n := range f.notes {
		if f.noteInc[n.ID] == id {
			out = append(out, n)
		}
	}
	return out
}

func (f *fakeIncidents) dueAt(id int64) *time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.due[id]
}

// fakeNotifier records outbound calls and lets tests control CSM and Chat outcomes.
type fakeNotifier struct {
	mu           sync.Mutex
	csmDisabled  bool
	csmOK        bool
	csmPermanent bool
	chatOK       bool
	pushErr      error
	openStates   map[string]bool
	onCSM        func()

	csmCalls      int
	creationNotes []string
	chatCalls     int
	annotations   []string
	pushed        []string
	stateChecks   int
}

func (n *fakeNotifier) CSMEnabled() bool { return !n.csmDisabled }

func (n *fakeNotifier) NotifyCSM(_ context.Context, inc model.Incident, creationNote string) (string, string, bool, bool) {
	if n.onCSM != nil {
		n.onCSM()
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.csmCalls++
	n.creationNotes = append(n.creationNotes, creationNote)
	if !n.csmOK {
		return "", "", false, n.csmPermanent
	}
	return fmt.Sprintf("csm-%d", inc.ID), fmt.Sprintf("INC%07d", inc.ID), true, false
}

func (n *fakeNotifier) NotifyChat(context.Context, model.Incident) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.chatCalls++
	return n.chatOK
}

func (n *fakeNotifier) NotifyChatAnnotation(_ context.Context, _ model.Incident, text string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.annotations = append(n.annotations, text)
	return n.chatOK
}

func (n *fakeNotifier) PushWorkNote(_ context.Context, _, note string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.pushErr != nil {
		return n.pushErr
	}
	n.pushed = append(n.pushed, note)
	return nil
}

func (n *fakeNotifier) IncidentState(_ context.Context, _, number string) (bool, bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.stateChecks++
	open, found := n.openStates[number]
	return open, found, nil
}

func testConfig() Config {
	return Config{
		DedupWindow:          5 * time.Minute,
		MaxCSMAttempts:       3,
		CSMRetry:             CSMRetryConfig{BaseDelay: time.Hour, Multiplier: 2, MaxDelay: 4 * time.Hour},
		ChatThreadingEnabled: true,
		DeliveryConcurrency:  4,
	}
}

func newTestEngine(n *fakeNotifier, cfg Config) (*Engine, *fakeIncidents) {
	incidents := newFakeIncidents()
	return New(testLogger(), incidents, n, cfg), incidents
}

var t0 = time.Now().UTC().Add(-time.Minute)

func item(id string, offset time.Duration, severity string) Item {
	return Item{ID: id, Alert: model.Alert{Service: "svc", MetricName: "cpu", Severity: severity, Source: "vendor", UniqueIdentifier: "u1", ReceivedAt: t0.Add(offset)}}
}

func fpOf(it Item) string {
	a := it.Alert
	return model.Fingerprint(a.Source, a.Service, a.MetricName, a.Environment, a.UniqueIdentifier)
}

func handle(t *testing.T, e *Engine, items ...Item) {
	t.Helper()
	if err := e.HandleGroup(context.Background(), fpOf(items[0]), items); err != nil {
		t.Fatalf("HandleGroup: %v", err)
	}
}

func kinds(notes []model.Note) []string {
	out := make([]string, len(notes))
	for i, n := range notes {
		out[i] = n.Kind
	}
	return out
}

func TestHandleGroup_NewAlertCreatesIncidentWithoutCallingCSM(t *testing.T) {
	n := &fakeNotifier{csmOK: true}
	e, incidents := newTestEngine(n, testConfig())

	handle(t, e, item("ALT1", 0, "critical"))

	got := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	if len(got) != 1 || got[0].AlertCount != 1 || got[0].Severity != 1 || !got[0].FirstSeen.Equal(t0) {
		t.Fatalf("incidents = %+v, want one critical incident first seen at the alert's arrival time", got)
	}
	if n.csmCalls != 0 {
		t.Fatalf("NotifyCSM calls = %d, want 0: processing must never wait on CSM", n.csmCalls)
	}
	if notes := incidents.notesOf(got[0].ID); len(notes) != 1 || notes[0].Kind != model.NoteCreated || notes[0].ChatPending {
		t.Fatalf("notes = %+v, want one creation note not owed to Chat", notes)
	}
}

func TestDeliverDue_CreatesInCSMWithCreationNoteOnce(t *testing.T) {
	n := &fakeNotifier{csmOK: true}
	e, incidents := newTestEngine(n, testConfig())
	handle(t, e, item("ALT1", 0, "critical"))

	e.DeliverDue(context.Background())
	e.DeliverDue(context.Background())

	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if n.csmCalls != 1 || !inc.CSMConfirmed || inc.IncidentNumber != "INC0000001" {
		t.Fatalf("csm calls = %d incident = %+v, want one confirmed create", n.csmCalls, inc)
	}
	if !strings.Contains(n.creationNotes[0], "Incident auto-created from Alert: ALT1") {
		t.Fatalf("creation note = %q, want it sent with the create", n.creationNotes[0])
	}
	if len(n.pushed) != 0 || n.chatCalls != 0 {
		t.Fatalf("pushed = %v chat = %d, want the creation note not pushed again and no Chat", n.pushed, n.chatCalls)
	}
	if due := incidents.dueAt(inc.ID); due != nil {
		t.Fatalf("due = %v, want nothing left to deliver", due)
	}
}

func TestHandleGroup_DuplicatesInsideWindowFoldIntoOneIncident(t *testing.T) {
	n := &fakeNotifier{csmOK: true}
	e, incidents := newTestEngine(n, testConfig())

	handle(t, e, item("ALT1", 0, "critical"))
	handle(t, e, item("ALT2", time.Minute, "critical"), item("ALT3", 4*time.Minute+59*time.Second, "critical"))

	got := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	if len(got) != 1 || got[0].AlertCount != 3 || !got[0].LastSeen.Equal(t0.Add(4*time.Minute+59*time.Second)) {
		t.Fatalf("incidents = %+v, want one incident absorbing all three alerts", got)
	}
	if k := kinds(incidents.notesOf(got[0].ID)); !slices.Equal(k, []string{"created", "duplicate", "duplicate"}) {
		t.Fatalf("note kinds = %v", k)
	}
}

func TestHandleGroup_AlertAtWindowBoundaryOpensNewIncident(t *testing.T) {
	n := &fakeNotifier{csmOK: true}
	e, incidents := newTestEngine(n, testConfig())

	handle(t, e, item("ALT1", 0, "critical"), item("ALT2", 2*time.Minute, "critical"), item("ALT3", 5*time.Minute, "critical"), item("ALT4", 6*time.Minute, "critical"))

	got := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	if len(got) != 2 {
		t.Fatalf("incidents = %d, want 2: the alert at T+5m starts a new incident", len(got))
	}
	if got[0].AlertCount != 2 || got[1].AlertCount != 2 || !got[1].FirstSeen.Equal(t0.Add(5*time.Minute)) {
		t.Fatalf("incidents = %+v, want 2 alerts each, the second starting at T+5m", got)
	}
}

func TestDeliverDue_SupersededIncidentStillDelivered(t *testing.T) {
	n := &fakeNotifier{csmOK: true}
	e, incidents := newTestEngine(n, testConfig())

	handle(t, e, item("ALT1", 0, "critical"))
	handle(t, e, item("ALT2", 6*time.Minute, "critical"))
	e.DeliverDue(context.Background())

	got := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	if n.csmCalls != 2 || !got[0].CSMConfirmed || !got[1].CSMConfirmed {
		t.Fatalf("csm calls = %d incidents = %+v, want both incidents created in CSM", n.csmCalls, got)
	}
}

func TestHandleGroup_OutOfOrderGroupIsFoldedInArrivalOrder(t *testing.T) {
	n := &fakeNotifier{}
	e, incidents := newTestEngine(n, testConfig())

	handle(t, e, item("ALT9", 2*time.Minute, "critical"), item("ALT1", 0, "critical"))

	got := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	notes := incidents.notesOf(got[0].ID)
	if len(got) != 1 || notes[0].AlertID != "ALT1" || !got[0].FirstSeen.Equal(t0) {
		t.Fatalf("incidents = %+v notes = %+v, want ALT1 to create it", got, notes)
	}
}

func TestHandleGroup_ReplayIsANoOp(t *testing.T) {
	n := &fakeNotifier{}
	e, incidents := newTestEngine(n, testConfig())

	handle(t, e, item("ALT1", 0, "critical"), item("ALT2", time.Minute, "critical"))
	handle(t, e, item("ALT1", 0, "critical"), item("ALT2", time.Minute, "critical"))

	got := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	if len(got) != 1 || got[0].AlertCount != 2 || len(incidents.notesOf(got[0].ID)) != 2 {
		t.Fatalf("incidents = %+v, want a replayed group to change nothing", got)
	}
}

func TestHandleGroup_ResolvingAlerts(t *testing.T) {
	n := &fakeNotifier{}
	e, incidents := newTestEngine(n, testConfig())

	handle(t, e, item("ALT1", 0, "ok"))
	if got := incidents.byFingerprint(fpOf(item("ALT1", 0, ""))); len(got) != 0 {
		t.Fatalf("an OK with nothing to resolve must not open an incident, got %+v", got)
	}

	handle(t, e, item("ALT2", time.Minute, "critical"), item("ALT3", 2*time.Minute, "clear"))
	got := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	if k := kinds(incidents.notesOf(got[0].ID)); !slices.Equal(k, []string{"created", "ok"}) {
		t.Fatalf("note kinds = %v, want the clear recorded as an OK note", k)
	}
}

func TestDeliverDue_CSMDisabled_ChatCardThenOneDigestPerSweep(t *testing.T) {
	n := &fakeNotifier{csmDisabled: true, chatOK: true}
	e, incidents := newTestEngine(n, testConfig())

	handle(t, e, item("ALT1", 0, "critical"))
	e.DeliverDue(context.Background())

	dups := []Item{}
	for i := range 10 {
		dups = append(dups, item(fmt.Sprintf("ALT%d", i+2), time.Duration(i+1)*time.Second, "critical"))
	}
	handle(t, e, dups...)
	e.DeliverDue(context.Background())
	e.DeliverDue(context.Background())

	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if n.csmCalls != 0 || n.chatCalls != 1 || !inc.Fallback {
		t.Fatalf("csm = %d chat cards = %d fallback = %v, want one Chat card and no CSM", n.csmCalls, n.chatCalls, inc.Fallback)
	}
	if len(n.annotations) != 1 || !strings.Contains(n.annotations[0], "10 duplicate alerts received.") {
		t.Fatalf("annotations = %q, want one digest covering all 10 duplicates", n.annotations)
	}
	for _, note := range incidents.notesOf(inc.ID) {
		if note.CSMPending || note.ChatPending {
			t.Fatalf("note %+v still pending with CSM disabled", note)
		}
	}
	if due := incidents.dueAt(inc.ID); due != nil {
		t.Fatalf("due = %v, want nothing left to deliver", due)
	}
}

func TestDeliverDue_CSMFailureBacksOffThenGivesUp(t *testing.T) {
	n := &fakeNotifier{csmOK: false, chatOK: true}
	cfg := testConfig()
	cfg.CSMRetry.BaseDelay = time.Millisecond
	cfg.CSMRetry.MaxDelay = time.Millisecond
	e, incidents := newTestEngine(n, cfg)
	handle(t, e, item("ALT1", 0, "critical"))

	for range 5 {
		e.DeliverDue(context.Background())
		time.Sleep(3 * time.Millisecond)
	}

	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if n.csmCalls != 3 || !inc.CSMPermanentlyFailed {
		t.Fatalf("csm calls = %d incident = %+v, want 3 attempts then permanent failure", n.csmCalls, inc)
	}
	if n.chatCalls != 1 || !inc.Fallback {
		t.Fatalf("chat cards = %d, want exactly one fallback card", n.chatCalls)
	}
	if due := incidents.dueAt(inc.ID); due != nil {
		t.Fatalf("due = %v, want delivery settled after giving up", due)
	}
}

func TestDeliverDue_CSMBackoffSchedulesNextAttempt(t *testing.T) {
	n := &fakeNotifier{csmOK: false, chatOK: true}
	e, incidents := newTestEngine(n, testConfig())
	handle(t, e, item("ALT1", 0, "critical"))

	e.DeliverDue(context.Background())
	e.DeliverDue(context.Background())

	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	due := incidents.dueAt(inc.ID)
	if n.csmCalls != 1 || due == nil || time.Until(*due) < 50*time.Minute {
		t.Fatalf("csm calls = %d due = %v, want one attempt and the next scheduled about an hour out", n.csmCalls, due)
	}
}

func TestDeliverDue_PushesDuplicateNotesInOrderAndRetriesFailures(t *testing.T) {
	n := &fakeNotifier{csmOK: true}
	e, incidents := newTestEngine(n, testConfig())
	handle(t, e, item("ALT1", 0, "critical"))
	e.DeliverDue(context.Background())

	n.pushErr = errors.New("csm down")
	handle(t, e, item("ALT2", time.Minute, "critical"), item("ALT3", 2*time.Minute, "critical"))
	e.DeliverDue(context.Background())
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if due := incidents.dueAt(inc.ID); due == nil || len(n.pushed) != 0 {
		t.Fatalf("due = %v pushed = %v, want a scheduled retry and nothing pushed", due, n.pushed)
	}

	n.pushErr = nil
	incidents.mu.Lock()
	now := time.Now()
	incidents.due[inc.ID] = &now
	incidents.mu.Unlock()
	e.DeliverDue(context.Background())
	if len(n.pushed) != 2 || !strings.Contains(n.pushed[0], "ALT2") || !strings.Contains(n.pushed[1], "ALT3") {
		t.Fatalf("pushed = %v, want ALT2 then ALT3", n.pushed)
	}
	if len(n.annotations) != 0 {
		t.Fatalf("annotations = %v, want no Chat replies for a CSM-confirmed incident", n.annotations)
	}
}

func TestDeliverDue_ClosedInCSMStartsNewIncidentOnNextAlert(t *testing.T) {
	n := &fakeNotifier{csmOK: true, openStates: map[string]bool{"INC0000001": false}}
	e, incidents := newTestEngine(n, testConfig())
	handle(t, e, item("ALT1", 0, "critical"))
	e.DeliverDue(context.Background())

	if n.stateChecks != 0 {
		t.Fatalf("state checks = %d, want none right after the create", n.stateChecks)
	}

	handle(t, e, item("ALT2", 2*time.Minute, "critical"))
	e.DeliverDue(context.Background())
	handle(t, e, item("ALT3", 3*time.Minute, "critical"))

	got := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))
	if n.stateChecks != 1 || got[0].Status != "closed" || len(got) != 2 || got[0].AlertCount != 2 {
		t.Fatalf("state checks = %d incidents = %+v, want the closed incident replaced", n.stateChecks, got)
	}
}

func TestDeliver_FoldDuringDeliveryKeepsIncidentDue(t *testing.T) {
	n := &fakeNotifier{csmOK: true}
	e, incidents := newTestEngine(n, testConfig())
	handle(t, e, item("ALT1", 0, "critical"))

	n.onCSM = func() { handle(t, e, item("ALT2", time.Minute, "critical")) }
	e.DeliverDue(context.Background())

	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if due := incidents.dueAt(inc.ID); due == nil || due.After(time.Now()) {
		t.Fatalf("due = %v, want the incident still due for the note folded mid-delivery", due)
	}
	n.onCSM = nil
	e.DeliverDue(context.Background())
	if len(n.pushed) != 1 || !strings.Contains(n.pushed[0], "ALT2") {
		t.Fatalf("pushed = %v, want ALT2's note", n.pushed)
	}
}

func TestDeliver_SkipsIncidentLockedElsewhere(t *testing.T) {
	n := &fakeNotifier{csmOK: true}
	e, incidents := newTestEngine(n, testConfig())
	handle(t, e, item("ALT1", 0, "critical"))
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]

	unlock, _, _ := incidents.TryLock(context.Background(), inc.ID)
	e.DeliverDue(context.Background())
	unlock()
	if n.csmCalls != 0 {
		t.Fatalf("csm calls = %d, want 0 while another worker delivers it", n.csmCalls)
	}
	e.DeliverDue(context.Background())
	if n.csmCalls != 1 {
		t.Fatalf("csm calls = %d, want 1 once the lock is free", n.csmCalls)
	}
}

func (f *fakeIncidents) age(id int64, by time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.incidents[id].CreatedAt = f.incidents[id].CreatedAt.Add(-by)
	now := time.Now()
	f.due[id] = &now
}

func TestDeliverDue_ChatFallbackWaitsForCSMGracePeriod(t *testing.T) {
	n := &fakeNotifier{csmOK: false, chatOK: true}
	cfg := testConfig()
	cfg.ChatFallbackDelay = time.Minute
	e, incidents := newTestEngine(n, cfg)
	handle(t, e, item("ALT1", 0, "critical"))

	e.DeliverDue(context.Background())
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	due := incidents.dueAt(inc.ID)
	if n.csmCalls != 1 || n.chatCalls != 0 {
		t.Fatalf("csm = %d chat = %d, want one CSM attempt and no Chat inside the grace period", n.csmCalls, n.chatCalls)
	}
	if due == nil || due.Sub(inc.CreatedAt) > time.Minute+time.Second {
		t.Fatalf("due = %v, want the next pass at the end of the grace period (created %v)", due, inc.CreatedAt)
	}

	handle(t, e, item("ALT2", time.Second, "critical"))
	incidents.age(inc.ID, time.Minute)
	e.DeliverDue(context.Background())
	if n.chatCalls != 1 || len(n.annotations) != 1 || !strings.Contains(n.annotations[0], "Duplicate alert received.") {
		t.Fatalf("chat = %d annotations = %q, want the card once the grace period passed, then the waiting duplicate", n.chatCalls, n.annotations)
	}
}

func TestDeliverDue_PermanentCSMRejectionSkipsChatGracePeriod(t *testing.T) {
	n := &fakeNotifier{csmOK: false, csmPermanent: true, chatOK: true}
	cfg := testConfig()
	cfg.ChatFallbackDelay = time.Hour
	e, _ := newTestEngine(n, cfg)
	handle(t, e, item("ALT1", 0, "critical"))

	e.DeliverDue(context.Background())
	if n.chatCalls != 1 {
		t.Fatalf("chat = %d, want the card at once after a permanent CSM rejection", n.chatCalls)
	}
}

func TestDeliverDue_CSMConfirmingInsideGracePeriodNeverUsesChat(t *testing.T) {
	n := &fakeNotifier{csmOK: false, chatOK: true}
	cfg := testConfig()
	cfg.ChatFallbackDelay = time.Minute
	cfg.CSMRetry.BaseDelay = time.Millisecond
	e, incidents := newTestEngine(n, cfg)
	handle(t, e, item("ALT1", 0, "critical"))
	e.DeliverDue(context.Background())

	n.csmOK = true
	time.Sleep(3 * time.Millisecond)
	e.DeliverDue(context.Background())
	inc := incidents.byFingerprint(fpOf(item("ALT1", 0, "")))[0]
	if !inc.CSMConfirmed || n.chatCalls != 0 || inc.Fallback {
		t.Fatalf("confirmed = %v chat = %d, want CSM to win without any Chat post", inc.CSMConfirmed, n.chatCalls)
	}
}
