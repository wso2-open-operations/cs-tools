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

package slaengine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

type registerCall struct {
	caseID, clockType string
	startedAt, dueAt  time.Time
	req               RegisterClockRequest
}

type tierCall struct{ caseID, clockType, tier string }

// fakeEntityClock is a hand-written fake for entityClock, following this
// repo's own dispatch_test.go idiom (no mocking library).
type fakeEntityClock struct {
	registerCalls []registerCall
	registerErr   error

	getClockFn func(caseID, clockType string) (Clock, error)

	tierCalls          []tierCall
	tierResult         time.Time
	tierAlreadyReached bool
	tierErr            error
}

func (f *fakeEntityClock) RegisterClock(_ context.Context, req RegisterClockRequest) error {
	f.registerCalls = append(f.registerCalls, registerCall{req.CaseID, req.ClockType, req.StartedAt, req.DueAt, req})
	return f.registerErr
}

func (f *fakeEntityClock) GetClock(_ context.Context, caseID, clockType string) (Clock, error) {
	if f.getClockFn != nil {
		return f.getClockFn(caseID, clockType)
	}
	return Clock{}, nil
}

func (f *fakeEntityClock) SetTierReachedIfUnset(_ context.Context, caseID, clockType, tier string) (time.Time, bool, error) {
	f.tierCalls = append(f.tierCalls, tierCall{caseID, clockType, tier})
	return f.tierResult, f.tierAlreadyReached, f.tierErr
}

type wakeEntry struct {
	member string
	at     time.Time
}

type fakeWakeIndex struct {
	added   []wakeEntry
	removed []string
	due     []string

	addErr    error
	removeErr error
	dueErr    error
}

func (f *fakeWakeIndex) AddWake(_ context.Context, member string, at time.Time) error {
	f.added = append(f.added, wakeEntry{member, at})
	return f.addErr
}

func (f *fakeWakeIndex) RemoveWake(_ context.Context, member string) error {
	f.removed = append(f.removed, member)
	return f.removeErr
}

func (f *fakeWakeIndex) DueMembers(_ context.Context, _ time.Time) ([]string, error) {
	return f.due, f.dueErr
}

type publishCall struct {
	key, value []byte
}

type fakePublisher struct {
	calls []publishCall
	err   error
}

func (f *fakePublisher) Publish(_ context.Context, key, value []byte) error {
	f.calls = append(f.calls, publishCall{key, value})
	return f.err
}

type chatCall struct {
	product, clockType, tier, caseNumber string
}

// fakeChatSender is a hand-written fake for chatSender — Tick's happy path
// (and, since the Chat-retry fix, its alreadyReached path too) always
// reaches Engine.sendBreachAlert, so every Tick-exercising test needs one
// wired in even when it isn't asserting on the Chat send itself.
type fakeChatSender struct {
	calls []chatCall
	err   error
}

func (f *fakeChatSender) SendSLABreachAlert(_ context.Context, product, clockType, tier, caseNumber, _, _, _, _, _, _, _, _, _ string) error {
	f.calls = append(f.calls, chatCall{product, clockType, tier, caseNumber})
	return f.err
}

// fakeLinkResolver is a hand-written fake for linkResolver.
type fakeLinkResolver struct{}

func (fakeLinkResolver) CSMLink(caseID string) string { return "https://example.test/cases/" + caseID }

func newTestEngine(entity entityClock, wake wakeIndex, pub eventPublisher) *Engine {
	return &Engine{entity: entity, wake: wake, pub: pub, chat: &fakeChatSender{}, links: fakeLinkResolver{}, defaultChatProduct: "Test Product"}
}

func TestEngine_Handle_IgnoresOtherEventTypes(t *testing.T) {
	entity := &fakeEntityClock{}
	wake := &fakeWakeIndex{}
	e := newTestEngine(entity, wake, &fakePublisher{})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{}}`)}
	if err := e.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
	if len(entity.registerCalls) != 0 || len(wake.added) != 0 {
		t.Errorf("expected no registration for an unrelated event type, got registerCalls=%d added=%d", len(entity.registerCalls), len(wake.added))
	}
}

func TestEngine_Handle_RegistersClockAndSeedsWakeEntries(t *testing.T) {
	entity := &fakeEntityClock{}
	wake := &fakeWakeIndex{}
	e := newTestEngine(entity, wake, &fakePublisher{})

	record := eventbus.Record{Value: []byte(`{"type":"sla.clock.register","entityId":"CASE-1","payload":{"caseId":"CASE-1","durations":{"response":"2h"}}}`)}
	if err := e.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}

	if len(entity.registerCalls) != 1 {
		t.Fatalf("expected 1 RegisterClock call, got %d", len(entity.registerCalls))
	}
	reg := entity.registerCalls[0]
	if reg.caseID != "CASE-1" || reg.clockType != "response" {
		t.Errorf("registerCall = %+v, want caseID=CASE-1 clockType=response", reg)
	}
	if got, want := reg.dueAt.Sub(reg.startedAt), 2*time.Hour; got != want {
		t.Errorf("dueAt - startedAt = %v, want %v", got, want)
	}

	if len(wake.added) != 3 {
		t.Fatalf("expected 3 wake entries (50/75/100), got %d", len(wake.added))
	}
	wantMembers := map[string]bool{
		"CASE-1|response|50":  true,
		"CASE-1|response|75":  true,
		"CASE-1|response|100": true,
	}
	for _, w := range wake.added {
		if !wantMembers[w.member] {
			t.Errorf("unexpected wake member %q", w.member)
		}
	}
}

// TestEngine_Handle_RejectsWholeRecordOnOneInvalidDuration verifies that a
// mix of one valid and one unparsable duration fails events.Validate before
// registerClocks ever runs — mirroring internal/events' own validRecipients
// philosophy (one bad entry fails the whole event, dead-lettered for
// visibility, rather than silently registering a subset).
func TestEngine_Handle_RejectsWholeRecordOnOneInvalidDuration(t *testing.T) {
	entity := &fakeEntityClock{}
	wake := &fakeWakeIndex{}
	e := newTestEngine(entity, wake, &fakePublisher{})

	record := eventbus.Record{Value: []byte(`{"type":"sla.clock.register","entityId":"CASE-1","payload":{"caseId":"CASE-1","durations":{"response":"not-a-duration","resolution":"4h"}}}`)}
	if err := e.Handle(context.Background(), record); err == nil {
		t.Fatal("Handle() error = nil, want the whole record rejected for its one invalid duration")
	}
	if len(entity.registerCalls) != 0 {
		t.Errorf("expected no clock registered when the record as a whole was rejected, got %+v", entity.registerCalls)
	}
}

func TestEngine_Handle_PropagatesRegisterError(t *testing.T) {
	entity := &fakeEntityClock{registerErr: errors.New("entity-service unreachable")}
	wake := &fakeWakeIndex{}
	e := newTestEngine(entity, wake, &fakePublisher{})

	record := eventbus.Record{Value: []byte(`{"type":"sla.clock.register","entityId":"CASE-1","payload":{"caseId":"CASE-1","durations":{"response":"2h"}}}`)}
	if err := e.Handle(context.Background(), record); err == nil {
		t.Fatal("Handle() error = nil, want a propagated error")
	}
	if len(wake.added) != 0 {
		t.Errorf("expected no wake entries seeded after a failed RegisterClock, got %d", len(wake.added))
	}
}

func TestEngine_Tick_FiresDueMemberAndPublishes(t *testing.T) {
	reached := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	entity := &fakeEntityClock{tierResult: reached}
	wake := &fakeWakeIndex{due: []string{"CASE-1|response|50"}}
	pub := &fakePublisher{}
	e := newTestEngine(entity, wake, pub)

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}

	if len(entity.tierCalls) != 1 || entity.tierCalls[0] != (tierCall{"CASE-1", "response", "50"}) {
		t.Fatalf("tierCalls = %+v, want one call for CASE-1/response/50", entity.tierCalls)
	}
	if len(pub.calls) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(pub.calls))
	}
	if string(pub.calls[0].key) != "CASE-1" {
		t.Errorf("publish key = %q, want CASE-1", pub.calls[0].key)
	}
	var env events.Envelope
	if err := json.Unmarshal(pub.calls[0].value, &env); err != nil {
		t.Fatalf("failed to decode published envelope: %v", err)
	}
	if env.Type != events.TypeSLATierReached || env.EntityID != "CASE-1" {
		t.Errorf("envelope = %+v, want type=sla.tier_reached entityId=CASE-1", env)
	}
	var payload events.SLATierReachedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload != (events.SLATierReachedPayload{CaseID: "CASE-1", ClockType: "response", Tier: "50"}) {
		t.Errorf("payload = %+v, want CASE-1/response/50", payload)
	}

	chat := e.chat.(*fakeChatSender)
	if len(chat.calls) != 1 || chat.calls[0] != (chatCall{"Test Product", "response", "50", "CASE-1"}) {
		t.Errorf("chat.calls = %+v, want one alert for CASE-1/response/50", chat.calls)
	}

	if len(wake.removed) != 1 || wake.removed[0] != "CASE-1|response|50" {
		t.Errorf("removed = %v, want the fired member removed", wake.removed)
	}
}

// TestEngine_Tick_SkipsPublishWhenAlreadyReached verifies the accepted
// trade-off documented on Tick's own doc comment: when entity-service
// reports the tier was already claimed by an earlier call — including,
// critically, an in-process early completion on entity-service's own side
// (e.g. a qualifying support engineer comment satisfying the response SLA,
// or a case closing) — this engine does not publish again AND does not
// send a Chat alert; it just drops the now-stale wake entry. The Chat
// assertion here is the regression guard for a real bug: an earlier
// version sent the Chat alert unconditionally on alreadyReached=true
// (reasoning it was just retrying a failed send), which produced a false
// "SLA at risk" alert for a response SLA a support engineer had already
// satisfied in time. See Tick's doc comment for the full reasoning.
func TestEngine_Tick_SkipsPublishWhenAlreadyReached(t *testing.T) {
	reached := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	entity := &fakeEntityClock{tierResult: reached, tierAlreadyReached: true}
	wake := &fakeWakeIndex{due: []string{"CASE-1|response|50"}}
	pub := &fakePublisher{}
	e := newTestEngine(entity, wake, pub)

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}

	if len(pub.calls) != 0 {
		t.Fatalf("expected no publish when alreadyReached=true, got %d", len(pub.calls))
	}
	if chat := e.chat.(*fakeChatSender); len(chat.calls) != 0 {
		t.Fatalf("expected no chat alert when alreadyReached=true, got %+v", chat.calls)
	}
	if len(wake.removed) != 1 || wake.removed[0] != "CASE-1|response|50" {
		t.Errorf("expected the stale wake entry cleaned up regardless, removed = %v", wake.removed)
	}
}

func TestEngine_Tick_SkipsPausedClock(t *testing.T) {
	pausedAt := time.Now()
	entity := &fakeEntityClock{getClockFn: func(string, string) (Clock, error) { return Clock{PausedOn: &pausedAt}, nil }}
	wake := &fakeWakeIndex{due: []string{"CASE-1|response|50"}}
	pub := &fakePublisher{}
	e := newTestEngine(entity, wake, pub)

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(entity.tierCalls) != 0 || len(pub.calls) != 0 {
		t.Errorf("expected a paused clock to skip both the tier write and the publish, got tierCalls=%d publishes=%d", len(entity.tierCalls), len(pub.calls))
	}
	if len(wake.removed) != 1 {
		t.Errorf("expected the wake entry dropped for a paused clock, removed=%v", wake.removed)
	}
}

func TestEngine_Tick_KeepsWakeEntryOnPublishFailure(t *testing.T) {
	entity := &fakeEntityClock{}
	wake := &fakeWakeIndex{due: []string{"CASE-1|response|50"}}
	pub := &fakePublisher{err: errors.New("kafka unreachable")}
	e := newTestEngine(entity, wake, pub)

	if err := e.Tick(context.Background(), time.Now()); err == nil {
		t.Fatal("Tick() error = nil, want the publish failure propagated")
	}
	if len(wake.removed) != 0 {
		t.Errorf("expected the wake entry kept for retry after a publish failure, removed=%v", wake.removed)
	}
}

func TestEngine_Tick_DropsMalformedMember(t *testing.T) {
	entity := &fakeEntityClock{}
	wake := &fakeWakeIndex{due: []string{"malformed-no-pipes"}}
	pub := &fakePublisher{}
	e := newTestEngine(entity, wake, pub)

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(entity.tierCalls) != 0 || len(pub.calls) != 0 {
		t.Error("expected a malformed member to never reach entity/publish calls")
	}
	if len(wake.removed) != 1 || wake.removed[0] != "malformed-no-pipes" {
		t.Errorf("expected the malformed member dropped, removed=%v", wake.removed)
	}
}
