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

package alertprocessor

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/strategy"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeStore is an in-memory Store: one map of groups and one of alert links.
type fakeStore struct {
	groups map[string]strategy.Group
	links  map[string]string
	// failLinks makes the next LinkAlert calls fail, like a database outage after the sink was written.
	failLinks int
}

func newFakeStore() *fakeStore {
	return &fakeStore{groups: map[string]strategy.Group{}, links: map[string]string{}}
}

// InGroup works on a copy and keeps it only when fn succeeds, as a rolled-back transaction would.
func (s *fakeStore) InGroup(_ context.Context, fingerprint string, fn func(GroupTx) error) error {
	work := &fakeStore{groups: maps.Clone(s.groups), links: maps.Clone(s.links), failLinks: s.failLinks}
	err := fn(fakeTx{s: work, fingerprint: fingerprint})
	s.failLinks = work.failLinks
	if err == nil {
		s.groups, s.links = work.groups, work.links
	}
	return err
}

type fakeTx struct {
	s           *fakeStore
	fingerprint string
}

func (t fakeTx) Group(context.Context) (*strategy.Group, error) {
	if g, ok := t.s.groups[t.fingerprint]; ok {
		return &g, nil
	}
	return nil, nil
}
func (t fakeTx) SaveGroup(_ context.Context, g strategy.Group) error {
	t.s.groups[t.fingerprint] = g
	return nil
}
func (t fakeTx) AlertLinked(_ context.Context, id string) (bool, error) {
	_, ok := t.s.links[id]
	return ok, nil
}
func (t fakeTx) LinkAlert(_ context.Context, id, incidentID string) error {
	if t.s.failLinks > 0 {
		t.s.failLinks--
		return errors.New("database down")
	}
	t.s.links[id] = incidentID
	return nil
}

// fakeSink records what the processor asks for; closed lists incidents that are no longer open.
type fakeSink struct {
	opened  []string
	notes   []string
	closed  map[string]bool
	openErr error
	// byAlert is what Find answers: the incident each alert opened.
	byAlert map[string]string
}

func (s *fakeSink) Open(_ context.Context, e event.Event) (string, error) {
	if s.openErr != nil {
		return "", s.openErr
	}
	if id, ok := s.byAlert[e.AlertID]; ok {
		return id, nil
	}
	s.opened = append(s.opened, e.AlertID)
	s.byAlert[e.AlertID] = "inc-" + e.AlertID
	return s.byAlert[e.AlertID], nil
}
func (s *fakeSink) Find(_ context.Context, alertID string) (string, error) {
	return s.byAlert[alertID], nil
}
func (s *fakeSink) IsOpen(_ context.Context, id string) (bool, error) { return !s.closed[id], nil }
func (s *fakeSink) Fold(_ context.Context, id string, outcome strategy.Outcome, e event.Event) error {
	s.notes = append(s.notes, id+": "+string(outcome)+" "+e.AlertID)
	return nil
}

func newFakeSink() *fakeSink {
	return &fakeSink{closed: map[string]bool{}, byAlert: map[string]string{}}
}

func noRules(context.Context, time.Time) ([]domain.AlertMuteRule, error) { return nil, nil }

func newProcessor(store *fakeStore, sink *fakeSink, apply bool) *Processor {
	return &Processor{Store: store, Sink: sink, MuteRules: noRules, Chain: strategy.Core(5 * time.Minute), Apply: apply}
}

var firstSeen = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

func alert(id, severity string, after time.Duration) event.Event {
	return event.Event{AlertID: id, ReceivedOn: firstSeen.Add(after), Alert: model.Alert{
		Service: "client-prod-alert-integration", MetricName: "TEST", Severity: severity,
		Category: "service_interruption", Environment: "prod", Source: "elasticsearch", UniqueIdentifier: "r2",
	}}
}

func TestProcessorFoldsAFingerprint(t *testing.T) {
	store, sink := newFakeStore(), newFakeSink()
	p := newProcessor(store, sink, true)
	ctx := context.Background()

	steps := []event.Event{alert("A1", "critical", 0), alert("A2", "critical", time.Minute), alert("A3", "ok", 2*time.Minute)}
	for _, e := range steps {
		if err := p.Handle(ctx, e); err != nil {
			t.Fatalf("Handle(%s): %v", e.AlertID, err)
		}
	}
	if len(sink.opened) != 1 || sink.opened[0] != "A1" {
		t.Fatalf("opened %v, want one incident from A1", sink.opened)
	}
	if len(sink.notes) != 2 || !strings.Contains(sink.notes[0], "duplicate A2") || !strings.Contains(sink.notes[1], "resolve A3") {
		t.Fatalf("notes %v, want a Duplicate then an OK note", sink.notes)
	}
	for _, id := range []string{"A1", "A2", "A3"} {
		if store.links[id] != "inc-A1" {
			t.Errorf("%s linked to %q, want inc-A1", id, store.links[id])
		}
	}
	if g := store.groups[steps[0].Fingerprint()]; g.AlertCount != 3 || !g.LastSeenOn.Equal(steps[2].ReceivedOn) {
		t.Errorf("group = %+v, want 3 alerts last seen at the OK", g)
	}
}

func TestProcessorOpensAgainOnceTheIncidentIsClosed(t *testing.T) {
	store, sink := newFakeStore(), newFakeSink()
	p := newProcessor(store, sink, true)
	ctx := context.Background()
	_ = p.Handle(ctx, alert("A1", "critical", 0))
	sink.closed["inc-A1"] = true
	if err := p.Handle(ctx, alert("A2", "critical", time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(sink.opened) != 2 || store.links["A2"] != "inc-A2" {
		t.Fatalf("opened %v, A2 linked to %q; want a new incident for A2", sink.opened, store.links["A2"])
	}
}

func TestProcessorRedeliveryIsANoOp(t *testing.T) {
	store, sink := newFakeStore(), newFakeSink()
	p := newProcessor(store, sink, true)
	e := alert("A1", "critical", 0)
	for range 2 {
		if err := p.Handle(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	if len(sink.opened) != 1 || len(sink.notes) != 0 {
		t.Fatalf("opened %v notes %v, want the redelivery to do nothing", sink.opened, sink.notes)
	}
}

func TestProcessorWithoutApplyOnlyDecides(t *testing.T) {
	store, sink := newFakeStore(), newFakeSink()
	if err := newProcessor(store, sink, false).Handle(context.Background(), alert("A1", "critical", 0)); err != nil {
		t.Fatal(err)
	}
	if len(sink.opened) != 0 || len(store.groups) != 0 || len(store.links) != 0 {
		t.Fatalf("hybrid wrote: opened %v groups %v links %v", sink.opened, store.groups, store.links)
	}
}

func TestProcessorFailedOpenIsRetried(t *testing.T) {
	store, sink := newFakeStore(), &fakeSink{closed: map[string]bool{}, byAlert: map[string]string{}, openErr: errors.New("db down")}
	p := newProcessor(store, sink, true)
	if err := p.Handle(context.Background(), alert("A1", "critical", 0)); err == nil {
		t.Fatal("Handle succeeded, want the error so the event is retried")
	}
	g := store.groups[alert("A1", "critical", 0).Fingerprint()]
	if len(store.links) != 0 || g.OpeningAlertID != "A1" || g.IncidentID != "" {
		t.Fatalf("after a failed open: group %+v links %v; want only a pending open for A1", g, store.links)
	}
}

func TestProcessorMuteRulesFailOpen(t *testing.T) {
	store, sink := newFakeStore(), newFakeSink()
	p := newProcessor(store, sink, true)
	p.MuteRules = func(context.Context, time.Time) ([]domain.AlertMuteRule, error) {
		return nil, errors.New("unavailable")
	}
	if err := p.Handle(context.Background(), alert("A1", "critical", 0)); err != nil {
		t.Fatal(err)
	}
	if len(sink.opened) != 1 {
		t.Fatalf("opened %v, want the alert to page when mute rules cannot be read", sink.opened)
	}
}

func TestProcessorRetryAfterARecordFailureOpensNoSecondIncident(t *testing.T) {
	store, sink := newFakeStore(), newFakeSink()
	p := newProcessor(store, sink, true)
	ctx := context.Background()
	store.failLinks = 1
	if err := p.Handle(ctx, alert("A1", "critical", 0)); err == nil {
		t.Fatal("Handle succeeded, want the record failure so the event is retried")
	}
	if err := p.Handle(ctx, alert("A1", "critical", 0)); err != nil {
		t.Fatal(err)
	}
	if len(sink.opened) != 1 || store.links["A1"] != "inc-A1" {
		t.Fatalf("opened %v, A1 linked to %q; want one incident, linked on the retry", sink.opened, store.links["A1"])
	}
}

func TestProcessorLaterAlertFinishesAnOpenThatNeverRecorded(t *testing.T) {
	store, sink := newFakeStore(), newFakeSink()
	p := newProcessor(store, sink, true)
	ctx := context.Background()
	store.failLinks = 1
	_ = p.Handle(ctx, alert("A1", "critical", 0)) // incident created, never recorded, event given up on
	if err := p.Handle(ctx, alert("A2", "critical", time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(sink.opened) != 1 || store.links["A1"] != "inc-A1" || store.links["A2"] != "inc-A1" {
		t.Fatalf("opened %v links %v; want A2 folded into A1's incident and A1 linked", sink.opened, store.links)
	}
	if len(sink.notes) != 1 || !strings.Contains(sink.notes[0], "duplicate A2") {
		t.Fatalf("notes %v, want one Duplicate note for A2", sink.notes)
	}
}

func TestProcessorLaterAlertOpensWhenTheEarlierOpenCreatedNothing(t *testing.T) {
	store, sink := newFakeStore(), newFakeSink()
	p := newProcessor(store, sink, true)
	ctx := context.Background()
	sink.openErr = errors.New("csm down")
	_ = p.Handle(ctx, alert("A1", "critical", 0))
	sink.openErr = nil
	if err := p.Handle(ctx, alert("A2", "critical", time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(sink.opened) != 1 || sink.opened[0] != "A2" || store.links["A2"] != "inc-A2" {
		t.Fatalf("opened %v links %v; want A2 to open the incident", sink.opened, store.links)
	}
}
