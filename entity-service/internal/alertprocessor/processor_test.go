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
)

// storedAlert is one row of the fake alert table.
type storedAlert struct {
	fingerprint string
	receivedOn  time.Time
	incidentID  string
}

// fakeStore is an in-memory alert table; InGroup keeps its changes only when fn succeeds, as a rolled-back transaction would.
type fakeStore struct {
	alerts map[string]storedAlert
	// failLinks makes the next LinkAlert calls fail, like a database outage after the incident was written.
	failLinks int
}

func newFakeStore() *fakeStore { return &fakeStore{alerts: map[string]storedAlert{}} }

// add stores an alert the way the consumer does, before its event is handled.
func (s *fakeStore) add(e event.Event) event.Event {
	s.alerts[e.AlertID] = storedAlert{fingerprint: e.Fingerprint(), receivedOn: e.ReceivedOn}
	return e
}

func (s *fakeStore) InGroup(_ context.Context, fingerprint string, fn func(GroupTx) error) error {
	work := &fakeStore{alerts: maps.Clone(s.alerts), failLinks: s.failLinks}
	err := fn(fakeTx{s: work, fingerprint: fingerprint})
	s.failLinks = work.failLinks
	if err == nil {
		s.alerts = work.alerts
	}
	return err
}

type fakeTx struct {
	s           *fakeStore
	fingerprint string
}

func (t fakeTx) Group(context.Context) (*strategy.Group, error) {
	var latest storedAlert
	for _, a := range t.s.alerts {
		if a.fingerprint == t.fingerprint && a.incidentID != "" && a.receivedOn.After(latest.receivedOn) {
			latest = a
		}
	}
	if latest.incidentID == "" {
		return nil, nil
	}
	g := &strategy.Group{Fingerprint: t.fingerprint, IncidentID: latest.incidentID}
	for _, a := range t.s.alerts {
		if a.fingerprint != t.fingerprint || a.incidentID != latest.incidentID {
			continue
		}
		if g.AlertCount == 0 || a.receivedOn.Before(g.FirstSeenOn) {
			g.FirstSeenOn = a.receivedOn
		}
		if a.receivedOn.After(g.LastSeenOn) {
			g.LastSeenOn = a.receivedOn
		}
		g.AlertCount++
	}
	return g, nil
}

func (t fakeTx) Unlinked(_ context.Context, since, until time.Time, except string) ([]string, error) {
	var ids []string
	for id, a := range t.s.alerts {
		if id != except && a.fingerprint == t.fingerprint && a.incidentID == "" && !a.receivedOn.Before(since) && !a.receivedOn.After(until) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (t fakeTx) AlertLinked(_ context.Context, id string) (bool, error) {
	return t.s.alerts[id].incidentID != "", nil
}

func (t fakeTx) LinkAlert(_ context.Context, id, incidentID string) error {
	if t.s.failLinks > 0 {
		t.s.failLinks--
		return errors.New("database down")
	}
	a := t.s.alerts[id]
	a.incidentID = incidentID
	t.s.alerts[id] = a
	return nil
}

func (s *fakeStore) linkOf(id string) string { return s.alerts[id].incidentID }

// fakeWriter records what the processor asks for; byAlert is what Find answers.
type fakeWriter struct {
	opened  []string
	notes   []string
	closed  map[string]bool
	byAlert map[string]string
	openErr error
}

func newFakeWriter() *fakeWriter {
	return &fakeWriter{closed: map[string]bool{}, byAlert: map[string]string{}}
}

func (s *fakeWriter) Open(_ context.Context, e event.Event) (string, error) {
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
func (s *fakeWriter) Find(_ context.Context, alertID string) (string, error) {
	return s.byAlert[alertID], nil
}
func (s *fakeWriter) IsOpen(_ context.Context, id string) (bool, error) { return !s.closed[id], nil }
func (s *fakeWriter) Fold(_ context.Context, id string, outcome strategy.Outcome, e event.Event) error {
	s.notes = append(s.notes, id+": "+string(outcome)+" "+e.AlertID)
	return nil
}

func newProcessor(store *fakeStore, writer *fakeWriter, apply bool) *Processor {
	return &Processor{Store: store, Incidents: writer, Chain: strategy.Default(5 * time.Minute), Lookback: 5 * time.Minute, Apply: apply}
}

var firstSeen = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

func alert(id, severity string, after time.Duration) event.Event {
	return event.Event{AlertID: id, ReceivedOn: firstSeen.Add(after), Alert: model.Alert{
		Service: "client-prod-alert-integration", MetricName: "TEST", Severity: severity,
		Category: "service_interruption", Environment: "prod", Source: "elasticsearch", UniqueIdentifier: "r2",
	}}
}

func TestProcessorFoldsAFingerprint(t *testing.T) {
	store, writer := newFakeStore(), newFakeWriter()
	p := newProcessor(store, writer, true)
	ctx := context.Background()
	for _, e := range []event.Event{alert("A1", "critical", 0), alert("A2", "critical", time.Minute), alert("A3", "ok", 2*time.Minute)} {
		if err := p.Handle(ctx, store.add(e)); err != nil {
			t.Fatalf("Handle(%s): %v", e.AlertID, err)
		}
	}
	if len(writer.opened) != 1 || writer.opened[0] != "A1" {
		t.Fatalf("opened %v, want one incident from A1", writer.opened)
	}
	if len(writer.notes) != 2 || !strings.Contains(writer.notes[0], "duplicate A2") || !strings.Contains(writer.notes[1], "resolve A3") {
		t.Fatalf("notes %v, want a Duplicate then an OK note", writer.notes)
	}
	for _, id := range []string{"A1", "A2", "A3"} {
		if store.linkOf(id) != "inc-A1" {
			t.Errorf("%s linked to %q, want inc-A1", id, store.linkOf(id))
		}
	}
}

func TestProcessorOpensAgainOnceTheIncidentIsClosed(t *testing.T) {
	store, writer := newFakeStore(), newFakeWriter()
	p := newProcessor(store, writer, true)
	ctx := context.Background()
	_ = p.Handle(ctx, store.add(alert("A1", "critical", 0)))
	writer.closed["inc-A1"] = true
	if err := p.Handle(ctx, store.add(alert("A2", "critical", time.Minute))); err != nil {
		t.Fatal(err)
	}
	if len(writer.opened) != 2 || store.linkOf("A2") != "inc-A2" {
		t.Fatalf("opened %v, A2 linked to %q; want a new incident for A2", writer.opened, store.linkOf("A2"))
	}
}

func TestProcessorRedeliveryIsANoOp(t *testing.T) {
	store, writer := newFakeStore(), newFakeWriter()
	p := newProcessor(store, writer, true)
	e := store.add(alert("A1", "critical", 0))
	for range 2 {
		if err := p.Handle(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	if len(writer.opened) != 1 || len(writer.notes) != 0 {
		t.Fatalf("opened %v notes %v, want the redelivery to do nothing", writer.opened, writer.notes)
	}
}

func TestProcessorWithoutApplyOnlyDecides(t *testing.T) {
	store, writer := newFakeStore(), newFakeWriter()
	if err := newProcessor(store, writer, false).Handle(context.Background(), store.add(alert("A1", "critical", 0))); err != nil {
		t.Fatal(err)
	}
	if len(writer.opened) != 0 || store.linkOf("A1") != "" {
		t.Fatalf("hybrid wrote: opened %v, A1 linked to %q", writer.opened, store.linkOf("A1"))
	}
}

func TestProcessorFailedOpenIsRetried(t *testing.T) {
	store, writer := newFakeStore(), newFakeWriter()
	writer.openErr = errors.New("csm down")
	if err := newProcessor(store, writer, true).Handle(context.Background(), store.add(alert("A1", "critical", 0))); err == nil {
		t.Fatal("Handle succeeded, want the error so the event is retried")
	}
	if store.linkOf("A1") != "" {
		t.Fatalf("a failed open linked A1 to %q", store.linkOf("A1"))
	}
}

func TestProcessorRetryAfterALinkFailureOpensNoSecondIncident(t *testing.T) {
	store, writer := newFakeStore(), newFakeWriter()
	p := newProcessor(store, writer, true)
	e := store.add(alert("A1", "critical", 0))
	store.failLinks = 1
	if err := p.Handle(context.Background(), e); err == nil {
		t.Fatal("Handle succeeded, want the link failure so the event is retried")
	}
	if err := p.Handle(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if len(writer.opened) != 1 || store.linkOf("A1") != "inc-A1" {
		t.Fatalf("opened %v, A1 linked to %q; want one incident, linked on the retry", writer.opened, store.linkOf("A1"))
	}
}

func TestProcessorLaterAlertFindsAnIncidentNeverLinked(t *testing.T) {
	store, writer := newFakeStore(), newFakeWriter()
	p := newProcessor(store, writer, true)
	ctx := context.Background()
	store.failLinks = 1
	_ = p.Handle(ctx, store.add(alert("A1", "critical", 0))) // incident created, never linked, event given up on
	if err := p.Handle(ctx, store.add(alert("A2", "critical", time.Minute))); err != nil {
		t.Fatal(err)
	}
	if len(writer.opened) != 1 || store.linkOf("A1") != "inc-A1" || store.linkOf("A2") != "inc-A1" {
		t.Fatalf("opened %v, A1→%q A2→%q; want A2 folded into A1's incident and A1 linked", writer.opened, store.linkOf("A1"), store.linkOf("A2"))
	}
}

func TestProcessorLaterAlertOpensWhenTheEarlierOpenCreatedNothing(t *testing.T) {
	store, writer := newFakeStore(), newFakeWriter()
	p := newProcessor(store, writer, true)
	ctx := context.Background()
	writer.openErr = errors.New("csm down")
	_ = p.Handle(ctx, store.add(alert("A1", "critical", 0)))
	writer.openErr = nil
	if err := p.Handle(ctx, store.add(alert("A2", "critical", time.Minute))); err != nil {
		t.Fatal(err)
	}
	if len(writer.opened) != 1 || writer.opened[0] != "A2" || store.linkOf("A2") != "inc-A2" {
		t.Fatalf("opened %v, A2 linked to %q; want A2 to open the incident", writer.opened, store.linkOf("A2"))
	}
}
