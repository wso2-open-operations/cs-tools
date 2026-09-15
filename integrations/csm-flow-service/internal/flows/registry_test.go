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

package flows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
)

// fakeFlow is a test double for Flow. A real flow's own test would instead
// construct the real flow type and assert on Match/Run directly (see
// example_flow_test.go for that shape); fakeFlow exists only to test the
// Registry's routing in isolation from any real flow.
type fakeFlow struct {
	key   string
	match func(Event) bool
	run   func(context.Context, Event, Deps) error
	ran   bool
}

func (f *fakeFlow) Key() string        { return f.key }
func (f *fakeFlow) Match(e Event) bool { return f.match(e) }
func (f *fakeFlow) Run(ctx context.Context, e Event, d Deps) error {
	f.ran = true
	if f.run != nil {
		return f.run(ctx, e, d)
	}
	return nil
}

// recordFor marshals an envelope into a bus record, the way a real producer
// would put it on the wire.
func recordFor(t *testing.T, env events.Envelope) eventbus.Record {
	t.Helper()
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return eventbus.Record{Topic: "cs-events", Value: b}
}

func envelope(typ events.Type, entityID string, payload any) events.Envelope {
	raw, _ := json.Marshal(payload)
	return events.Envelope{Type: typ, EntityID: entityID, Payload: raw}
}

func TestRegistry_RunsOnlyMatchingFlows(t *testing.T) {
	matches := &fakeFlow{key: "matches", match: func(Event) bool { return true }}
	skips := &fakeFlow{key: "skips", match: func(Event) bool { return false }}

	r := NewRegistry(Deps{}, matches, skips)
	rec := recordFor(t, envelope(events.TypeCommentAdded, "case-1",
		events.CommentAddedPayload{CaseID: "case-1", CaseComment: "hi"}))

	if err := r.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !matches.ran {
		t.Error("matching flow did not run")
	}
	if skips.ran {
		t.Error("non-matching flow ran")
	}
}

func TestRegistry_MatchSeesDecodedEnvelope(t *testing.T) {
	var gotType events.Type
	var gotEntityID string
	spy := &fakeFlow{
		key: "spy",
		match: func(e Event) bool {
			gotType = e.Envelope.Type
			gotEntityID = e.Envelope.EntityID
			return false
		},
	}
	r := NewRegistry(Deps{}, spy)
	rec := recordFor(t, envelope(events.TypeStatusChanged, "case-42", events.StatusChangedPayload{CaseID: "case-42", NewStatus: "closed"}))

	_ = r.Handle(context.Background(), rec)
	if gotType != events.TypeStatusChanged {
		t.Errorf("Match saw type %q, want %q", gotType, events.TypeStatusChanged)
	}
	if gotEntityID != "case-42" {
		t.Errorf("Match saw entityID %q, want %q", gotEntityID, "case-42")
	}
}

func TestRegistry_RunErrorIsReturned(t *testing.T) {
	wantErr := errors.New("boom")
	failing := &fakeFlow{key: "failing", match: func(Event) bool { return true },
		run: func(context.Context, Event, Deps) error { return wantErr }}

	r := NewRegistry(Deps{}, failing)
	rec := recordFor(t, envelope(events.TypeCommentAdded, "case-1", events.CommentAddedPayload{CaseID: "case-1"}))

	if err := r.Handle(context.Background(), rec); !errors.Is(err, wantErr) {
		t.Errorf("Handle returned %v, want %v (so the record is retried)", err, wantErr)
	}
}

func TestRegistry_PoisonRecordDropped(t *testing.T) {
	ran := &fakeFlow{key: "ran", match: func(Event) bool { return true }}
	r := NewRegistry(Deps{}, ran)

	// Not a valid envelope — must be dropped (nil error, so its offset commits),
	// never retried forever, and no flow should run.
	if err := r.Handle(context.Background(), eventbus.Record{Topic: "cs-events", Value: []byte("{not json")}); err != nil {
		t.Errorf("poison record should be dropped (nil), got %v", err)
	}
	if ran.ran {
		t.Error("a flow ran on an undecodable record")
	}
}
