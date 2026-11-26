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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeStore is an in-memory Store: one map of groups and one of alert links.
type fakeStore struct {
	groups map[string]Group
	links  map[string]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{groups: map[string]Group{}, links: map[string]string{}}
}

func (s *fakeStore) InGroup(_ context.Context, fingerprint string, fn func(GroupTx) error) error {
	return fn(fakeTx{s: s, fingerprint: fingerprint})
}

type fakeTx struct {
	s           *fakeStore
	fingerprint string
}

func (t fakeTx) Group(context.Context) (*Group, error) {
	if g, ok := t.s.groups[t.fingerprint]; ok {
		return &g, nil
	}
	return nil, nil
}
func (t fakeTx) SaveGroup(_ context.Context, g Group) error {
	t.s.groups[t.fingerprint] = g
	return nil
}
func (t fakeTx) AlertLinked(_ context.Context, id string) (bool, error) {
	_, ok := t.s.links[id]
	return ok, nil
}
func (t fakeTx) LinkAlert(_ context.Context, id, incidentID string) error {
	t.s.links[id] = incidentID
	return nil
}

// fakeSink records what the processor asks for; closed lists incidents that are no longer open.
type fakeSink struct {
	opened  []string
	notes   []string
	closed  map[string]bool
	openErr error
}

func (s *fakeSink) Open(_ context.Context, e Event) (string, error) {
	if s.openErr != nil {
		return "", s.openErr
	}
	s.opened = append(s.opened, e.AlertID)
	return "inc-" + e.AlertID, nil
}
func (s *fakeSink) IsOpen(_ context.Context, id string) (bool, error) { return !s.closed[id], nil }
func (s *fakeSink) Note(_ context.Context, id, html string) error {
	s.notes = append(s.notes, id+": "+html)
	return nil
}

func noRules(context.Context, time.Time) ([]domain.AlertMuteRule, error) { return nil, nil }

func newProcessor(store *fakeStore, sink *fakeSink, apply bool) *Processor {
	return &Processor{Store: store, Sink: sink, MuteRules: noRules, Chain: CoreChain(5 * time.Minute), Apply: apply}
}

func withID(e Event, id string) Event {
	e.AlertID = id
	return e
}

func TestProcessorFoldsAFingerprint(t *testing.T) {
	store, sink := newFakeStore(), &fakeSink{closed: map[string]bool{}}
	p := newProcessor(store, sink, true)
	ctx := context.Background()

	steps := []Event{withID(event("critical", 0), "A1"), withID(event("critical", time.Minute), "A2"), withID(event("ok", 2*time.Minute), "A3")}
	for _, e := range steps {
		if err := p.Handle(ctx, e); err != nil {
			t.Fatalf("Handle(%s): %v", e.AlertID, err)
		}
	}
	if len(sink.opened) != 1 || sink.opened[0] != "A1" {
		t.Fatalf("opened %v, want one incident from A1", sink.opened)
	}
	if len(sink.notes) != 2 || !strings.Contains(sink.notes[0], "Duplicate alert received") || !strings.Contains(sink.notes[1], "OK alert received") {
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
	store, sink := newFakeStore(), &fakeSink{closed: map[string]bool{}}
	p := newProcessor(store, sink, true)
	ctx := context.Background()
	_ = p.Handle(ctx, withID(event("critical", 0), "A1"))
	sink.closed["inc-A1"] = true
	if err := p.Handle(ctx, withID(event("critical", time.Minute), "A2")); err != nil {
		t.Fatal(err)
	}
	if len(sink.opened) != 2 || store.links["A2"] != "inc-A2" {
		t.Fatalf("opened %v, A2 linked to %q; want a new incident for A2", sink.opened, store.links["A2"])
	}
}

func TestProcessorRedeliveryIsANoOp(t *testing.T) {
	store, sink := newFakeStore(), &fakeSink{closed: map[string]bool{}}
	p := newProcessor(store, sink, true)
	e := withID(event("critical", 0), "A1")
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
	store, sink := newFakeStore(), &fakeSink{closed: map[string]bool{}}
	if err := newProcessor(store, sink, false).Handle(context.Background(), withID(event("critical", 0), "A1")); err != nil {
		t.Fatal(err)
	}
	if len(sink.opened) != 0 || len(store.groups) != 0 || len(store.links) != 0 {
		t.Fatalf("hybrid wrote: opened %v groups %v links %v", sink.opened, store.groups, store.links)
	}
}

func TestProcessorFailedOpenIsRetried(t *testing.T) {
	store, sink := newFakeStore(), &fakeSink{closed: map[string]bool{}, openErr: errors.New("db down")}
	p := newProcessor(store, sink, true)
	if err := p.Handle(context.Background(), withID(event("critical", 0), "A1")); err == nil {
		t.Fatal("Handle succeeded, want the error so the event is retried")
	}
	if len(store.links) != 0 || len(store.groups) != 0 {
		t.Fatalf("a failed open left state: groups %v links %v", store.groups, store.links)
	}
}

func TestProcessorMuteRulesFailOpen(t *testing.T) {
	store, sink := newFakeStore(), &fakeSink{closed: map[string]bool{}}
	p := newProcessor(store, sink, true)
	p.MuteRules = func(context.Context, time.Time) ([]domain.AlertMuteRule, error) {
		return nil, errors.New("unavailable")
	}
	if err := p.Handle(context.Background(), withID(event("critical", 0), "A1")); err != nil {
		t.Fatal(err)
	}
	if len(sink.opened) != 1 {
		t.Fatalf("opened %v, want the alert to page when mute rules cannot be read", sink.opened)
	}
}
