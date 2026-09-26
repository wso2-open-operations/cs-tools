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
	"strconv"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// fakeStatusLister is a hand-written fake for statusLister, following this
// repo's own dispatch_test.go idiom (no mocking library).
type fakeStatusLister struct {
	statuses []SLAStatus
	err      error
}

func (f *fakeStatusLister) FetchAllActiveSLAStatuses(context.Context) ([]SLAStatus, error) {
	return f.statuses, f.err
}

type tierCall struct {
	caseID, clockType string
	tier              int
}

// fakeTierStore is a hand-written in-memory fake for tierStore, including a
// fake claim/release ledger for ClaimTier/ReleaseTier — forceClaimLoss lets
// a test simulate a concurrent replica (or an earlier attempt) already
// holding a given tier's claim, without needing a real Redis.
type fakeTierStore struct {
	tiers  map[string]int
	claims map[string]bool

	getErr     error
	setErr     error
	claimErr   error
	releaseErr error

	forceClaimLoss map[string]bool

	sets         []tierCall
	claimCalls   []tierCall
	releaseCalls []tierCall
}

func newFakeTierStore() *fakeTierStore {
	return &fakeTierStore{tiers: map[string]int{}, claims: map[string]bool{}, forceClaimLoss: map[string]bool{}}
}

func (f *fakeTierStore) key(caseID, clockType string) string { return caseID + "|" + clockType }

func (f *fakeTierStore) claimKey(caseID, clockType string, tier int) string {
	return f.key(caseID, clockType) + "|" + strconv.Itoa(tier)
}

func (f *fakeTierStore) GetTier(_ context.Context, caseID, clockType string) (int, bool, error) {
	if f.getErr != nil {
		return 0, false, f.getErr
	}
	tier, ok := f.tiers[f.key(caseID, clockType)]
	return tier, ok, nil
}

func (f *fakeTierStore) SetTier(_ context.Context, caseID, clockType string, tier int) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.tiers[f.key(caseID, clockType)] = tier
	f.sets = append(f.sets, tierCall{caseID, clockType, tier})
	return nil
}

func (f *fakeTierStore) ClaimTier(_ context.Context, caseID, clockType string, tier int) (bool, error) {
	if f.claimErr != nil {
		return false, f.claimErr
	}
	f.claimCalls = append(f.claimCalls, tierCall{caseID, clockType, tier})
	key := f.claimKey(caseID, clockType, tier)
	if f.forceClaimLoss[key] || f.claims[key] {
		return false, nil
	}
	f.claims[key] = true
	return true, nil
}

func (f *fakeTierStore) ReleaseTier(_ context.Context, caseID, clockType string, tier int) error {
	if f.releaseErr != nil {
		return f.releaseErr
	}
	f.releaseCalls = append(f.releaseCalls, tierCall{caseID, clockType, tier})
	delete(f.claims, f.claimKey(caseID, clockType, tier))
	return nil
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

// fakeChatSender is a hand-written fake for chatSender.
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

func newTestEngine(entity statusLister, store tierStore, pub eventPublisher) *Engine {
	return &Engine{entity: entity, store: store, pub: pub, chat: &fakeChatSender{}, links: fakeLinkResolver{}, defaultChatProduct: "Test Product"}
}

func TestTierForStatus(t *testing.T) {
	tests := []struct {
		name   string
		status SLAStatus
		want   int
	}{
		{"well below 50", SLAStatus{BusinessElapsedPercent: 12.5}, 0},
		{"just under 50", SLAStatus{BusinessElapsedPercent: 49.99}, 0},
		{"exactly 50", SLAStatus{BusinessElapsedPercent: 50}, 50},
		{"between 50 and 75", SLAStatus{BusinessElapsedPercent: 60}, 50},
		{"exactly 75", SLAStatus{BusinessElapsedPercent: 75}, 75},
		{"between 75 and 100", SLAStatus{BusinessElapsedPercent: 90}, 75},
		{"exactly 100", SLAStatus{BusinessElapsedPercent: 100}, 100},
		{"over 100 (breached, still climbing)", SLAStatus{BusinessElapsedPercent: 260.44, HasBreached: true}, 100},
		{"hasBreached trusted even if percent somehow under 100", SLAStatus{BusinessElapsedPercent: 42, HasBreached: true}, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tierForStatus(tt.status); got != tt.want {
				t.Errorf("tierForStatus(%+v) = %d, want %d", tt.status, got, tt.want)
			}
		})
	}
}

// TestEngine_Tick_SeedsBaselineWithoutAlerting verifies the flood-avoidance
// rule this whole redesign hinges on: the first time this engine ever sees
// a (caseID, clockType) pair, it records the clock's CURRENT tier as a
// baseline and sends nothing — critical for the ~120,000 pre-existing
// in-progress "sla" rows this engine sees on its very first poll after
// deploy, most already well past 50%/75% elapsed.
func TestEngine_Tick_SeedsBaselineWithoutAlerting(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 82}}}
	store := newFakeTierStore()
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(pub.calls) != 0 {
		t.Errorf("expected no publish on first sight, got %d", len(pub.calls))
	}
	if chat := e.chat.(*fakeChatSender); len(chat.calls) != 0 {
		t.Errorf("expected no chat alert on first sight, got %+v", chat.calls)
	}
	if tier, ok := store.tiers["CASE-1|response"]; !ok || tier != 75 {
		t.Errorf("expected baseline seeded at tier 75 (82%% -> below 100), got tier=%d found=%v", tier, ok)
	}
}

// TestEngine_Tick_AlertsOnlyNewlyCrossedTier verifies the normal steady
// -state case: a clock already seen at tier 50 that has now reached 75
// alerts for 75 only, not 50 again.
func TestEngine_Tick_AlertsOnlyNewlyCrossedTier(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 80, CaseNumber: "CS0001"}}}
	store := newFakeTierStore()
	store.tiers["CASE-1|response"] = 50
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
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
	if payload != (events.SLATierReachedPayload{CaseID: "CASE-1", ClockType: "response", Tier: "75"}) {
		t.Errorf("payload = %+v, want CASE-1/response/75", payload)
	}

	chat := e.chat.(*fakeChatSender)
	if len(chat.calls) != 1 || chat.calls[0] != (chatCall{"Test Product", "response", "75", "CS0001"}) {
		t.Errorf("chat.calls = %+v, want one alert for CASE-1/response/75", chat.calls)
	}
	if store.tiers["CASE-1|response"] != 75 {
		t.Errorf("cursor = %d, want advanced to 75", store.tiers["CASE-1|response"])
	}
}

// TestEngine_Tick_AlertsEveryTierCrossedSinceLastPoll verifies that a clock
// whose percentage jumped past more than one checkpoint between polls (a
// slow ticker interval, or a burst of ServiceNow sync activity) still fires
// an alert for each intermediate tier, not just the highest one reached —
// in ascending order.
func TestEngine_Tick_AlertsEveryTierCrossedSinceLastPoll(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "resolution", HasBreached: true, BusinessElapsedPercent: 140}}}
	store := newFakeTierStore()
	store.tiers["CASE-1|resolution"] = 50
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}

	chat := e.chat.(*fakeChatSender)
	if len(chat.calls) != 2 {
		t.Fatalf("expected 2 chat alerts (75 then 100), got %+v", chat.calls)
	}
	if chat.calls[0].tier != "75" || chat.calls[1].tier != "100" {
		t.Errorf("chat.calls = %+v, want tier order 75, 100", chat.calls)
	}
	if len(pub.calls) != 2 {
		t.Errorf("expected 2 publishes, got %d", len(pub.calls))
	}
	if store.tiers["CASE-1|resolution"] != 100 {
		t.Errorf("cursor = %d, want advanced to 100", store.tiers["CASE-1|resolution"])
	}
}

func TestEngine_Tick_NoOpWhenTierUnchanged(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 55}}}
	store := newFakeTierStore()
	store.tiers["CASE-1|response"] = 50
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(pub.calls) != 0 {
		t.Errorf("expected no publish when the tier hasn't advanced, got %d", len(pub.calls))
	}
	if chat := e.chat.(*fakeChatSender); len(chat.calls) != 0 {
		t.Errorf("expected no chat alert when the tier hasn't advanced, got %+v", chat.calls)
	}
}

// TestEngine_Tick_RebaselinesOnRegressionWithoutAlerting covers an SLA
// policy reset or a fresh tracking cycle under the same (caseID,
// clockType) — entity-service's own live data has a small but real
// fraction of these (about 0.5% of clocks, per a live check). The
// percentage genuinely goes backwards; this must rebaseline quietly, not
// treat it as some kind of error or, worse, alert on the drop.
func TestEngine_Tick_RebaselinesOnRegressionWithoutAlerting(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 5}}}
	store := newFakeTierStore()
	store.tiers["CASE-1|response"] = 100
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(pub.calls) != 0 {
		t.Errorf("expected no publish on a tier regression, got %d", len(pub.calls))
	}
	if chat := e.chat.(*fakeChatSender); len(chat.calls) != 0 {
		t.Errorf("expected no chat alert on a tier regression, got %+v", chat.calls)
	}
	if store.tiers["CASE-1|response"] != 0 {
		t.Errorf("cursor = %d, want rebaselined to 0", store.tiers["CASE-1|response"])
	}
}

func TestEngine_Tick_SkipsPausedClockEntirely(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 80, IsPaused: true}}}
	store := newFakeTierStore()
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(store.sets) != 0 {
		t.Errorf("expected a paused clock to skip the tier store entirely, got %d writes", len(store.sets))
	}
	if len(pub.calls) != 0 {
		t.Errorf("expected no publish for a paused clock, got %d", len(pub.calls))
	}
}

// TestEngine_Tick_StopsAtFirstFailedTierAndKeepsCursor verifies that a
// publish/chat failure partway through a multi-tier crossing leaves the
// cursor at the last SUCCESSFULLY alerted tier, not the clock's current
// tier — so the next Tick retries exactly the remaining tiers, never
// silently skipping or double-alerting the one that already succeeded.
func TestEngine_Tick_StopsAtFirstFailedTierAndKeepsCursor(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", HasBreached: true, BusinessElapsedPercent: 140}}}
	store := newFakeTierStore()
	store.tiers["CASE-1|response"] = 50
	pub := &fakePublisher{err: errors.New("event hub unreachable")}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err == nil {
		t.Fatal("Tick() error = nil, want the publish failure propagated")
	}
	if len(pub.calls) != 1 {
		t.Errorf("expected exactly 1 failed publish attempt (stopping before tier 100), got %d", len(pub.calls))
	}
	if store.tiers["CASE-1|response"] != 50 {
		t.Errorf("cursor = %d, want left at 50 (no tier succeeded)", store.tiers["CASE-1|response"])
	}
}

func TestEngine_Tick_ChatFailurePropagatesAndKeepsCursor(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 60}}}
	store := newFakeTierStore()
	store.tiers["CASE-1|response"] = 0
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)
	e.chat = &fakeChatSender{err: errors.New("chat webhook unreachable")}

	if err := e.Tick(context.Background()); err == nil {
		t.Fatal("Tick() error = nil, want the chat send failure propagated")
	}
	if store.tiers["CASE-1|response"] != 0 {
		t.Errorf("cursor = %d, want left at 0 (chat send failed before it could advance)", store.tiers["CASE-1|response"])
	}
}

// TestEngine_Tick_JoinsErrorsAcrossStatusesButProcessesBoth verifies one
// clock's failure doesn't stop another clock in the same poll from being
// processed.
func TestEngine_Tick_JoinsErrorsAcrossStatusesButProcessesBoth(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{
		{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 80},
		{CaseID: "CASE-2", ClockType: "response", BusinessElapsedPercent: 80},
	}}
	store := newFakeTierStore()
	store.tiers["CASE-1|response"] = 50
	store.tiers["CASE-2|response"] = 50
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)
	e.chat = &fakeChatSender{err: errors.New("chat webhook unreachable")}

	err := e.Tick(context.Background())
	if err == nil {
		t.Fatal("Tick() error = nil, want both failures joined")
	}
	if len(pub.calls) != 2 {
		t.Errorf("expected both clocks' publish attempted despite the shared chat failure, got %d", len(pub.calls))
	}
}

func TestEngine_Tick_NoStatusesIsANoOp(t *testing.T) {
	e := newTestEngine(&fakeStatusLister{}, newFakeTierStore(), &fakePublisher{})
	if err := e.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
}

func TestEngine_Tick_PropagatesListError(t *testing.T) {
	e := newTestEngine(&fakeStatusLister{err: errors.New("entity-service unreachable")}, newFakeTierStore(), &fakePublisher{})
	if err := e.Tick(context.Background()); err == nil {
		t.Fatal("Tick() error = nil, want the list failure propagated")
	}
}

// TestEngine_Tick_LosingClaimRaceDoesNotAlert simulates the exact race a
// second concurrent replica would hit: both replicas read the same stale
// cursor and both decide tier 75 needs alerting, but only one wins the
// Redis SETNX claim. The losing call must not alert, must not error, and
// must not advance the cursor itself — the winner's own SetTier call is
// what advances it.
func TestEngine_Tick_LosingClaimRaceDoesNotAlert(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 80}}}
	store := newFakeTierStore()
	store.tiers["CASE-1|response"] = 50
	store.forceClaimLoss["CASE-1|response|75"] = true
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(pub.calls) != 0 {
		t.Errorf("expected no publish for a tier this call lost the claim race for, got %d", len(pub.calls))
	}
	if chat := e.chat.(*fakeChatSender); len(chat.calls) != 0 {
		t.Errorf("expected no chat alert for a tier this call lost the claim race for, got %+v", chat.calls)
	}
	if store.tiers["CASE-1|response"] != 50 {
		t.Errorf("cursor = %d, want left at 50 (this call never won a claim to advance past)", store.tiers["CASE-1|response"])
	}
}

// TestEngine_Tick_FailedAlertReleasesClaimForRetry verifies that a tier
// this call DID win the claim for, but then failed to alert, gives the
// claim back — unlike the pre-redesign engine's own equivalent failure
// case (which permanently lost the alert), a Redis claim is cheap to
// release, so there's no reason to accept that loss here.
func TestEngine_Tick_FailedAlertReleasesClaimForRetry(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 60}}}
	store := newFakeTierStore()
	store.tiers["CASE-1|response"] = 0
	pub := &fakePublisher{err: errors.New("event hub unreachable")}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err == nil {
		t.Fatal("Tick() error = nil, want the publish failure propagated")
	}
	if len(store.claimCalls) != 1 || store.claimCalls[0] != (tierCall{"CASE-1", "response", 50}) {
		t.Fatalf("claimCalls = %+v, want one claim attempt for tier 50", store.claimCalls)
	}
	if len(store.releaseCalls) != 1 || store.releaseCalls[0] != (tierCall{"CASE-1", "response", 50}) {
		t.Errorf("releaseCalls = %+v, want the failed tier's claim released", store.releaseCalls)
	}
	if store.claims[store.claimKey("CASE-1", "response", 50)] {
		t.Error("expected the claim to no longer be held after release, so a later tick can retry it")
	}
}

// TestEngine_Tick_RegressionReleasesClaimsAboveNewTier verifies the fix for
// the edge case a plain cursor alone can't handle: an SLA policy reset (or
// a fresh tracking cycle) drops the percentage back down after a clock
// already reached a high tier. Without releasing the old cycle's claims,
// the new cycle's own genuine re-crossing of the same tier numbers would
// silently find them already claimed and never alert.
func TestEngine_Tick_RegressionReleasesClaimsAboveNewTier(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 5}}}
	store := newFakeTierStore()
	store.tiers["CASE-1|response"] = 100
	store.claims[store.claimKey("CASE-1", "response", 50)] = true
	store.claims[store.claimKey("CASE-1", "response", 75)] = true
	store.claims[store.claimKey("CASE-1", "response", 100)] = true
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	for _, tier := range []int{50, 75, 100} {
		if store.claims[store.claimKey("CASE-1", "response", tier)] {
			t.Errorf("expected tier %d's claim released after the regression, still held", tier)
		}
	}
}

// TestEngine_Tick_PropagatesEachTierStoreError pins the behavior of every
// TierStore failure path processStatus has: a failed cursor read must skip
// the clock (not reseed it and silently swallow a genuine crossing), and
// every other store failure must likewise stop that clock's processing and
// surface an error from Tick, rather than being absorbed.
func TestEngine_Tick_PropagatesEachTierStoreError(t *testing.T) {
	someErr := errors.New("redis unreachable")

	tests := []struct {
		name       string
		percent    float64 // -> tier via tierForStatus
		seedCursor bool
		cursor     int
		setErrFn   func(*fakeTierStore)
	}{
		{
			name:     "GetTier fails",
			percent:  60,
			setErrFn: func(s *fakeTierStore) { s.getErr = someErr },
		},
		{
			name:     "SetTier fails seeding the first-sight baseline",
			percent:  60,
			setErrFn: func(s *fakeTierStore) { s.setErr = someErr },
		},
		{
			name:       "ReleaseTier fails cleaning up a regression",
			percent:    5,
			seedCursor: true,
			cursor:     100,
			setErrFn:   func(s *fakeTierStore) { s.releaseErr = someErr },
		},
		{
			name:       "ClaimTier fails on a genuine crossing",
			percent:    80,
			seedCursor: true,
			cursor:     50,
			setErrFn:   func(s *fakeTierStore) { s.claimErr = someErr },
		},
		{
			name:       "SetTier fails advancing the cursor after a successful alert",
			percent:    80,
			seedCursor: true,
			cursor:     50,
			setErrFn: func(s *fakeTierStore) {
				// Only the *second* SetTier call in this flow (the
				// post-alert cursor advance) should fail — the store has
				// no earlier SetTier call to conflict with in this
				// particular scenario, so a plain unconditional setErr is
				// enough here.
				s.setErr = someErr
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: tt.percent}}}
			store := newFakeTierStore()
			if tt.seedCursor {
				store.tiers["CASE-1|response"] = tt.cursor
			}
			tt.setErrFn(store)
			e := newTestEngine(entity, store, &fakePublisher{})

			if err := e.Tick(context.Background()); err == nil {
				t.Fatal("Tick() error = nil, want the store failure propagated")
			}
		})
	}
}

// TestEngine_Tick_GetTierFailureSkipsRatherThanReseeding is the specific
// regression guard TestEngine_Tick_PropagatesEachTierStoreError's first case
// only pins loosely: a Redis GetTier failure must be treated as "this clock
// couldn't be checked this poll," not conflated with GetTier's own found=false
// result (no error, just no cursor yet). Conflating the two would silently
// reseed the baseline on every transient Redis error and swallow whatever
// genuine crossing that poll should have caught.
func TestEngine_Tick_GetTierFailureSkipsRatherThanReseeding(t *testing.T) {
	entity := &fakeStatusLister{statuses: []SLAStatus{{CaseID: "CASE-1", ClockType: "response", BusinessElapsedPercent: 80}}}
	store := newFakeTierStore()
	store.tiers["CASE-1|response"] = 50 // a real, already-established cursor
	store.getErr = errors.New("redis unreachable")
	pub := &fakePublisher{}
	e := newTestEngine(entity, store, pub)

	if err := e.Tick(context.Background()); err == nil {
		t.Fatal("Tick() error = nil, want the GetTier failure propagated")
	}
	if len(pub.calls) != 0 {
		t.Errorf("expected no publish when the cursor couldn't be read, got %d", len(pub.calls))
	}
	if chat := e.chat.(*fakeChatSender); len(chat.calls) != 0 {
		t.Errorf("expected no chat alert when the cursor couldn't be read, got %+v", chat.calls)
	}
	if store.tiers["CASE-1|response"] != 50 {
		t.Errorf("cursor = %d, want left untouched at 50, not reseeded to the clock's current tier", store.tiers["CASE-1|response"])
	}
}
