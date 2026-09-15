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

package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/store"
)

type fakeClaimer struct {
	batches [][]store.Change
	calls   int
	types   []string
	err     error
}

func (f *fakeClaimer) ClaimChanges(_ context.Context, types []string, _ int) ([]store.Change, error) {
	f.types = types
	if f.err != nil {
		return nil, f.err
	}
	if f.calls < len(f.batches) {
		b := f.batches[f.calls]
		f.calls++
		return b, nil
	}
	f.calls++
	return nil, nil
}

type fakeDispatcher struct {
	got []eventbus.Record
	err error
}

func (f *fakeDispatcher) Handle(_ context.Context, rec eventbus.Record) error {
	f.got = append(f.got, rec)
	return f.err
}

func change(id int64) store.Change {
	return store.Change{
		ID:         id,
		EntityType: "change_request",
		EntityID:   "cr-1",
		Changes:    map[string]map[string]any{"state": {"from": "NEW", "to": "ASSESS"}},
		Snapshot:   map[string]any{"number": "CHG-TEST-0001", "state": "ASSESS"},
	}
}

// TestDrainOnce_RendersTheEventAFlowExpects is the contract between the trigger
// and the flow: a row change has to arrive looking exactly like an
// entity.changed event, or Match -- which was written against the bus shape --
// silently never fires.
func TestDrainOnce_RendersTheEventAFlowExpects(t *testing.T) {
	claimer := &fakeClaimer{batches: [][]store.Change{{change(1)}}}
	disp := &fakeDispatcher{}
	d := &Drainer{Claimer: claimer, Dispatcher: disp, EntityTypes: []string{"change_request"}}

	n, err := d.DrainOnce(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("DrainOnce = (%d, %v), want (1, nil)", n, err)
	}
	if len(disp.got) != 1 {
		t.Fatalf("dispatched %d records, want 1", len(disp.got))
	}

	var env events.Envelope
	if err := json.Unmarshal(disp.got[0].Value, &env); err != nil {
		t.Fatalf("dispatched record is not a valid envelope: %v", err)
	}
	if env.Type != events.TypeEntityChanged {
		t.Errorf("type = %s, want %s", env.Type, events.TypeEntityChanged)
	}
	if env.EntityID != "cr-1" {
		t.Errorf("entityId = %s, want cr-1", env.EntityID)
	}
	var payload events.EntityChangedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.EntityType != "change_request" {
		t.Errorf("entityType = %s", payload.EntityType)
	}
	if got := payload.Changes["state"]["to"]; got != "ASSESS" {
		t.Errorf("changes.state.to = %v, want ASSESS", got)
	}
	if got := payload.Changes["state"]["from"]; got != "NEW" {
		t.Errorf("changes.state.from = %v, want NEW — without it Match cannot tell 'changed to' from 'is'", got)
	}
	// Keyed by entity so ordering per entity survives, same as the bus.
	if string(disp.got[0].Key) != "cr-1" {
		t.Errorf("key = %q, want cr-1", disp.got[0].Key)
	}
	// An outbox row is claimed once and never redelivered, so a flow keying off
	// these flags must treat this as its only chance.
	if !disp.got[0].NoMoreRetries || !disp.got[0].IsFinalAttempt {
		t.Error("an outbox-sourced record must declare no further retries")
	}
}

// TestDrainOnce_PassesTheEntityTypeFilter proves the drainer only claims what
// some registered flow reacts to -- the outbox is shared, and another
// consumer's rows are none of this engine's business.
func TestDrainOnce_PassesTheEntityTypeFilter(t *testing.T) {
	claimer := &fakeClaimer{}
	d := &Drainer{Claimer: claimer, Dispatcher: &fakeDispatcher{}, EntityTypes: []string{"change_request", "case"}}
	if _, err := d.DrainOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(claimer.types) != 2 || claimer.types[0] != "change_request" {
		t.Errorf("claimed types = %v, want the registry's list passed through", claimer.types)
	}
}

// TestDrainOnce_OneBadRowDoesNotWedgeTheBatch: the row is already claimed, so
// failing the whole drain would stall every later row behind it forever.
func TestDrainOnce_OneBadRowDoesNotWedgeTheBatch(t *testing.T) {
	disp := &fakeDispatcher{err: errors.New("flow exploded")}
	claimer := &fakeClaimer{batches: [][]store.Change{{change(1), change(2)}}}
	d := &Drainer{Claimer: claimer, Dispatcher: disp}

	n, err := d.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("a failing flow must not fail the drain: %v", err)
	}
	if n != 2 || len(disp.got) != 2 {
		t.Errorf("drained %d dispatched %d, want both rows attempted", n, len(disp.got))
	}
}

// TestDrainOnce_ClaimErrorSurfaces: a broken claim is a real fault worth
// logging and retrying, unlike one bad row.
func TestDrainOnce_ClaimErrorSurfaces(t *testing.T) {
	sentinel := errors.New("db down")
	d := &Drainer{Claimer: &fakeClaimer{err: sentinel}, Dispatcher: &fakeDispatcher{}}
	if _, err := d.DrainOnce(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("got %v, want the claim error", err)
	}
}
