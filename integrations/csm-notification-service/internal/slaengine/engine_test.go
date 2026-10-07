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
	"errors"
	"testing"
	"time"
)

// fakeStore is an in-memory implementation of the store interface —
// everything this package's tests need, with no real Redis.
type fakeStore struct {
	wake        map[string]time.Time
	clocks      map[string]ClockMeta
	claims      map[string]bool
	failClaim   bool
	failAdd     bool
	failAdvance bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		wake:   map[string]time.Time{},
		clocks: map[string]ClockMeta{},
		claims: map[string]bool{},
	}
}

func (s *fakeStore) AddWake(_ context.Context, member string, at time.Time) error {
	if s.failAdd {
		return errors.New("add wake failed")
	}
	s.wake[member] = at
	return nil
}

func (s *fakeStore) RemoveWake(_ context.Context, member string) error {
	delete(s.wake, member)
	return nil
}

func (s *fakeStore) DueMembers(_ context.Context, now time.Time) ([]string, error) {
	var out []string
	for m, at := range s.wake {
		if !at.After(now) {
			out = append(out, m)
		}
	}
	return out, nil
}

// SetClock mirrors the real Store's own setClockScript semantics: display
// fields always overwrite, but state/paused/alertedTier only initialize on
// the first call for this key -- a later call (simulating a case.created
// retry/replay) must never reset them -- UNLESS StartedAt genuinely
// changed (a new incarnation, e.g. a severity revision's fresh clock seen
// via Reconcile), in which case alertedTier resets to 0 same as the real
// setClockScript. See redis.go's own doc comment on both.
func (s *fakeStore) SetClock(_ context.Context, caseID, clockType string, meta ClockMeta) error {
	key := caseID + "|" + clockType
	if existing, ok := s.clocks[key]; ok {
		meta.State = existing.State
		meta.Paused = existing.Paused
		if existing.StartedAt.Equal(meta.StartedAt) {
			meta.AlertedTier = existing.AlertedTier
		}
	}
	s.clocks[key] = meta
	return nil
}

func (s *fakeStore) GetClock(_ context.Context, caseID, clockType string) (ClockMeta, bool, error) {
	meta, ok := s.clocks[caseID+"|"+clockType]
	return meta, ok, nil
}

func (s *fakeStore) SetPaused(_ context.Context, caseID, clockType string, paused bool) error {
	key := caseID + "|" + clockType
	meta := s.clocks[key]
	meta.Paused = paused
	s.clocks[key] = meta
	return nil
}

func (s *fakeStore) SetState(_ context.Context, caseID, clockType, state string) error {
	key := caseID + "|" + clockType
	meta := s.clocks[key]
	meta.State = state
	s.clocks[key] = meta
	return nil
}

func (s *fakeStore) AdvanceAlertedTier(_ context.Context, caseID, clockType string, tier int, incarnation time.Time) (bool, error) {
	if s.failAdvance {
		return false, errors.New("advance failed")
	}
	key := caseID + "|" + clockType
	meta := s.clocks[key]
	if !incarnation.IsZero() && !meta.StartedAt.IsZero() && !meta.StartedAt.Equal(incarnation) {
		return false, nil
	}
	if tier > meta.AlertedTier {
		meta.AlertedTier = tier
	}
	s.clocks[key] = meta
	return true, nil
}

func (s *fakeStore) ClaimTier(_ context.Context, caseID, clockType string, tier int, startedAt time.Time) (bool, error) {
	if s.failClaim {
		return false, errors.New("claim failed")
	}
	key := tierClaimKey(caseID, clockType, tier, startedAt)
	if s.claims[key] {
		return false, nil
	}
	s.claims[key] = true
	return true, nil
}

func (s *fakeStore) ReleaseTier(_ context.Context, caseID, clockType string, tier int, startedAt time.Time) error {
	delete(s.claims, tierClaimKey(caseID, clockType, tier, startedAt))
	return nil
}

// fakeChat is a chatSender test double.
type fakeChat struct {
	calls   []fakeChatCall
	failFor map[string]bool // audience -> fail
	spaces  map[string]bool
}

type fakeChatCall struct {
	audience, clockType, tier, caseNumber, wso2CaseID, caseTitle, caseType, product, team, teamLeadName, severity, state, openedAt, caseLink string
}

func (f *fakeChat) SendSLABreachAlert(_ context.Context, audience, clockType, tier, caseNumber, wso2CaseID, caseTitle, caseType, productName, team, teamLeadName, severity, state, openedAt, caseLink string) error {
	f.calls = append(f.calls, fakeChatCall{audience, clockType, tier, caseNumber, wso2CaseID, caseTitle, caseType, productName, team, teamLeadName, severity, state, openedAt, caseLink})
	if f.failFor != nil && f.failFor[audience] {
		return errors.New("chat send failed")
	}
	return nil
}

func (f *fakeChat) HasAudienceSpace(audience string) bool {
	if f.spaces == nil {
		return audience == "Incident Monitor"
	}
	return f.spaces[audience]
}

// fakePublisher is an eventPublisher test double.
type fakePublisher struct {
	calls   int
	failErr error
}

func (p *fakePublisher) Publish(_ context.Context, _, _ []byte) error {
	p.calls++
	return p.failErr
}

// fakeLinks is a linkResolver test double.
type fakeLinks struct{}

func (fakeLinks) CSMLink(caseID string) string { return "https://csm.example.com/cases/" + caseID }

func testDurations() map[string]map[string]time.Duration {
	return map[string]map[string]time.Duration{
		"CATASTROPHIC": {ClockResponse: 15 * time.Minute, ClockWorkaround: 4 * time.Hour, ClockResolution: 48 * time.Hour},
		"LOW":          {ClockResponse: 24 * time.Hour},
		"MEDIUM":       {ClockResponse: 6 * time.Hour, ClockWorkaround: 72 * time.Hour, ClockResolution: 7 * 24 * time.Hour},
	}
}

func newTestEngine(st *fakeStore, chat *fakeChat, pub *fakePublisher) *Engine {
	return &Engine{store: st, pub: pub, chat: chat, links: fakeLinks{}, durations: testDurations()}
}

// --- RegisterClocks ---

func TestRegisterClocks_RegistersEveryClockThePolicyHasForThisSeverity(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	createdAt := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC) // a Monday

	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", createdAt, "CS0001", "WSO2-1", "Title", "CASE", "API Manager", "Team Nova")

	for _, ct := range []string{ClockResponse, ClockWorkaround, ClockResolution} {
		meta, found, _ := st.GetClock(context.Background(), "case-1", ct)
		if !found {
			t.Fatalf("clock %s not registered", ct)
		}
		if meta.CaseNumber != "CS0001" || meta.Team != "Team Nova" || meta.Priority != "CATASTROPHIC" {
			t.Errorf("clock %s meta = %+v, missing expected display fields", ct, meta)
		}
		for _, tier := range tierSequence {
			if _, ok := st.wake[wakeMember("case-1", ct, tier)]; !ok {
				t.Errorf("no wake entry for %s/%s/%d", "case-1", ct, tier)
			}
		}
	}
}

// TestRegisterClocks_SkipsClockTypesThePolicyHasNoEntryFor verifies LOW's
// policy (response only — see migration 0192's own seed data) registers
// exactly one clock, not all three.
func TestRegisterClocks_SkipsClockTypesThePolicyHasNoEntryFor(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.RegisterClocks(context.Background(), "case-1", "LOW", time.Now(), "CS0001", "", "", "CASE", "", "")

	if _, found, _ := st.GetClock(context.Background(), "case-1", ClockResponse); !found {
		t.Error("response clock not registered for LOW")
	}
	if _, found, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround); found {
		t.Error("workaround clock registered for LOW, want absent")
	}
	if _, found, _ := st.GetClock(context.Background(), "case-1", ClockResolution); found {
		t.Error("resolution clock registered for LOW, want absent")
	}
}

func TestRegisterClocks_UnknownSeverity_RegistersNothing(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.RegisterClocks(context.Background(), "case-1", "UNKNOWN", time.Now(), "CS0001", "", "", "CASE", "", "")

	if len(st.clocks) != 0 {
		t.Errorf("clocks registered for an unknown severity: %+v", st.clocks)
	}
}

// TestRegisterClocks_Replay_DoesNotResetPausedOrAlertedTier is the
// regression guard for a real bug: dispatch.handleCaseCreated's own
// email/Chat reactions can fail and retry the whole record, and a
// dead-lettered record gets a fresh retry pass with the identical
// case.created payload -- both redeliver the same event to RegisterClocks
// again. A clock already force-completed (e.g. by CompleteResponseClock)
// or paused (by ApplyStateEffects) in response to a LATER event must not
// be silently reset back to "never alerted, never paused" by a replay of
// the ORIGINAL case.created.
func TestRegisterClocks_Replay_DoesNotResetPausedOrAlertedTier(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	createdAt := time.Now()
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", createdAt, "CS0001", "", "", "CASE", "", "")

	// A later event completes the response clock and pauses workaround --
	// simulating CompleteResponseClock/ApplyStateEffects having already run
	// before the replay below.
	e.CompleteResponseClock(context.Background(), "case-1")
	e.ApplyStateEffects(context.Background(), "case-1", "Awaiting Info")

	// case.created is redelivered (retry or DLQ replay) with the identical
	// payload.
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", createdAt, "CS0001", "", "", "CASE", "", "")

	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResponse); meta.AlertedTier != 100 {
		t.Errorf("response AlertedTier = %d after replay, want 100 (must not reset)", meta.AlertedTier)
	}
	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround); !meta.Paused {
		t.Error("workaround Paused = false after replay, want true (must not reset)")
	}
}

// TestRegisterClocks_MediumResolution_AvoidsWeekendDueDate verifies the
// resolution clock's 100% wake entry rolls forward off a weekend, while
// response/workaround (not "1 Business Week" durations) do not.
func TestRegisterClocks_MediumResolution_AvoidsWeekendDueDate(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	// 7 days after this Monday 10:00 lands on a Monday too (not a weekend),
	// so pick a start that makes the resolution due date land on a Saturday
	// absent the roll: Jan 5 2026 is a Monday; +7 days = Jan 12 (Monday) --
	// use Jan 3 (Saturday) + ... actually simplest: start Thursday so due
	// (+7d) is also Thursday -- use a start where +7d lands on Saturday by
	// picking a Saturday start.
	createdAt := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC) // a Saturday
	e.RegisterClocks(context.Background(), "case-1", "MEDIUM", createdAt, "CS0001", "", "", "CASE", "", "")

	due100 := st.wake[wakeMember("case-1", ClockResolution, 100)]
	if due100.Weekday() == time.Saturday || due100.Weekday() == time.Sunday {
		t.Errorf("resolution 100%% due date = %v (%s), want rolled off the weekend", due100, due100.Weekday())
	}
	due100Response := st.wake[wakeMember("case-1", ClockResponse, 100)]
	wantResponseDue := createdAt.Add(6 * time.Hour)
	if !due100Response.Equal(wantResponseDue) {
		t.Errorf("response 100%% due date = %v, want %v (never weekend-rolled)", due100Response, wantResponseDue)
	}
}

// --- ApplyStateEffects ---

func TestApplyStateEffects_AwaitingInfo_PausesWorkaroundAndResolution(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")

	e.ApplyStateEffects(context.Background(), "case-1", "Awaiting Info")

	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround); !meta.Paused {
		t.Error("workaround clock not paused on Awaiting Info")
	}
	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResolution); !meta.Paused {
		t.Error("resolution clock not paused on Awaiting Info")
	}
	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResponse); meta.Paused {
		t.Error("response clock paused -- it should never be touched by ApplyStateEffects")
	}
}

func TestApplyStateEffects_Closed_CompletesResolutionAndPausesWorkaround(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")

	e.ApplyStateEffects(context.Background(), "case-1", "Closed")

	meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResolution)
	if meta.AlertedTier != 100 {
		t.Errorf("resolution AlertedTier = %d, want 100 (force-completed on close)", meta.AlertedTier)
	}
	if wMeta, _, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround); !wMeta.Paused {
		t.Error("workaround clock not paused on close")
	}
}

func TestApplyStateEffects_ResumesOnAnyOtherStatus(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")
	e.ApplyStateEffects(context.Background(), "case-1", "Awaiting Info")

	e.ApplyStateEffects(context.Background(), "case-1", "Work In Progress")

	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround); meta.Paused {
		t.Error("workaround clock still paused after resuming status")
	}
	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResolution); meta.Paused {
		t.Error("resolution clock still paused after resuming status")
	}
}

// --- CompleteResponseClock ---

func TestCompleteResponseClock_AdvancesToTier100(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")

	e.CompleteResponseClock(context.Background(), "case-1")

	meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResponse)
	if meta.AlertedTier != 100 {
		t.Errorf("response AlertedTier = %d, want 100", meta.AlertedTier)
	}
}

// --- Tick / processDueMember ---

func TestTick_AlertsADueTierAndRemovesTheWakeEntry(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)

	past := time.Now().Add(-time.Minute)
	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", Team: "Team Nova", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}

	if pub.calls != 1 {
		t.Errorf("publish calls = %d, want 1", pub.calls)
	}
	if len(chat.calls) != 1 || chat.calls[0].tier != "50" {
		t.Fatalf("chat calls = %+v, want exactly one call for tier 50", chat.calls)
	}
	if _, stillWaiting := st.wake[wakeMember("case-1", "response", 50)]; stillWaiting {
		t.Error("wake entry not removed after a successful alert")
	}
	meta, _, _ := st.GetClock(context.Background(), "case-1", "response")
	if meta.AlertedTier != 50 {
		t.Errorf("AlertedTier = %d, want 50 after alerting", meta.AlertedTier)
	}
}

func TestTick_PausedClock_DropsWakeEntryWithoutAlerting(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)

	past := time.Now().Add(-time.Minute)
	st.clocks["case-1|workaround"] = ClockMeta{Paused: true}
	st.wake[wakeMember("case-1", "workaround", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if pub.calls != 0 || len(chat.calls) != 0 {
		t.Error("alerted a paused clock")
	}
	if _, stillWaiting := st.wake[wakeMember("case-1", "workaround", 50)]; stillWaiting {
		t.Error("wake entry for a paused clock not removed (known gap: dropped, not rescheduled)")
	}
}

func TestTick_AlreadyAlertedTier_DropsWakeEntryWithoutDoubleAlerting(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)

	past := time.Now().Add(-time.Minute)
	// CompleteResponseClock (or an earlier tick) already advanced past 50.
	st.clocks["case-1|response"] = ClockMeta{AlertedTier: 100}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if pub.calls != 0 || len(chat.calls) != 0 {
		t.Error("alerted a tier already covered by an early completion")
	}
}

func TestTick_UnregisteredClock_DropsStrayWakeEntry(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	st.wake[wakeMember("case-1", "response", 50)] = time.Now().Add(-time.Minute)

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if _, stillWaiting := st.wake[wakeMember("case-1", "response", 50)]; stillWaiting {
		t.Error("stray wake entry (no registered clock) not removed")
	}
}

// TestTick_PublishFailure_LeavesWakeEntryForRetry is the regression guard
// for the one real retry path: a Kafka publish failure must not lose the
// tier -- the wake entry stays, and the claim is released so a later tick
// can try again.
func TestTick_PublishFailure_LeavesWakeEntryForRetry(t *testing.T) {
	st := newFakeStore()
	pub := &fakePublisher{failErr: errors.New("event hub unreachable")}
	e := newTestEngine(st, &fakeChat{}, pub)

	past := time.Now().Add(-time.Minute)
	st.clocks["case-1|response"] = ClockMeta{}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err == nil {
		t.Fatal("expected an error from a publish failure")
	}
	if _, stillWaiting := st.wake[wakeMember("case-1", "response", 50)]; !stillWaiting {
		t.Error("wake entry removed despite a publish failure -- it should be retried")
	}
	if st.claims[tierClaimKey("case-1", "response", 50, time.Time{})] {
		t.Error("tier claim not released after a publish failure")
	}
}

// TestTick_ChatFailure_StillRemovesWakeEntryAndAdvancesCursor is the
// regression guard for the explicit "don't retry a Chat failure" decision
// — see Engine.sendBreachAlert's own doc comment.
func TestTick_ChatFailure_StillRemovesWakeEntryAndAdvancesCursor(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{failFor: map[string]bool{"Incident Monitor": true}}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)

	past := time.Now().Add(-time.Minute)
	st.clocks["case-1|response"] = ClockMeta{}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil (a chat failure must not fail the tick)", err)
	}
	if _, stillWaiting := st.wake[wakeMember("case-1", "response", 50)]; stillWaiting {
		t.Error("wake entry not removed despite the publish succeeding")
	}
	meta, _, _ := st.GetClock(context.Background(), "case-1", "response")
	if meta.AlertedTier != 50 {
		t.Errorf("AlertedTier = %d, want 50 even though the chat send failed", meta.AlertedTier)
	}
}

func TestTick_ClaimLost_DropsWakeEntryWithoutAlerting(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)

	past := time.Now().Add(-time.Minute)
	st.clocks["case-1|response"] = ClockMeta{}
	st.wake[wakeMember("case-1", "response", 50)] = past
	// Simulate a concurrent replica (or an earlier attempt) already holding
	// the claim.
	st.claims[tierClaimKey("case-1", "response", 50, time.Time{})] = true

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if pub.calls != 0 || len(chat.calls) != 0 {
		t.Error("alerted despite losing the claim race")
	}
	if _, stillWaiting := st.wake[wakeMember("case-1", "response", 50)]; stillWaiting {
		t.Error("wake entry not removed after losing the claim race")
	}
}

func TestTick_MalformedMember_DropsIt(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	st.wake["not-a-valid-member"] = time.Now().Add(-time.Minute)

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if _, stillThere := st.wake["not-a-valid-member"]; stillThere {
		t.Error("malformed wake member not dropped")
	}
}

// --- helpers ---

func TestAvoidWeekend(t *testing.T) {
	sat := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)
	if got := avoidWeekend(sat); got.Weekday() != time.Monday {
		t.Errorf("avoidWeekend(Saturday) weekday = %v, want Monday", got.Weekday())
	}
	sun := time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC)
	if got := avoidWeekend(sun); got.Weekday() != time.Monday {
		t.Errorf("avoidWeekend(Sunday) weekday = %v, want Monday", got.Weekday())
	}
	mon := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	if got := avoidWeekend(mon); !got.Equal(mon) {
		t.Errorf("avoidWeekend(Monday) = %v, want unchanged", got)
	}
}

func TestParseWakeMember(t *testing.T) {
	caseID, clockType, tier, ok := parseWakeMember(wakeMember("case-1", "response", 75))
	if !ok || caseID != "case-1" || clockType != "response" || tier != 75 {
		t.Errorf("parseWakeMember() = (%q, %q, %d, %v), want (case-1, response, 75, true)", caseID, clockType, tier, ok)
	}
	if _, _, _, ok := parseWakeMember("missing-parts"); ok {
		t.Error("parseWakeMember(malformed) ok = true, want false")
	}
}

// --- Reconcile ---

// fakeActiveClocksClient is an entityActiveClocksClient test double.
type fakeActiveClocksClient struct {
	clocks []activeSLAClock
	err    error
}

func (f *fakeActiveClocksClient) GetActiveCSMSLAClocks(context.Context) ([]activeSLAClock, error) {
	return f.clocks, f.err
}

func timePtr(t time.Time) *time.Time { return &t }

// TestReconcileClock_PastTiersAreClaimedNotAlerted is the core regression
// guard for the whole reconciliation feature: a clock whose response window
// closed well before "now" (as it would after a real Redis wipe) must NOT
// get a wake entry for that already-past tier -- scheduling one would have
// the very next Tick immediately alert on stale history. The past tier is
// pre-claimed via AlertedTier instead, and a still-future tier still gets a
// real wake entry so normal ticking resumes.
func TestReconcileClock_PastTiersAreClaimedNotAlerted(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})

	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	// CATASTROPHIC response = 15m. Started 2h ago: all three tiers (50/75/100%
	// of 15m) are long past.
	startedAt := now.Add(-2 * time.Hour)

	e.reconcileClock(context.Background(), activeSLAClock{
		CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC",
		StartedOn: timePtr(startedAt), CaseNumber: "CS0001", Team: "Team Nova",
	}, now)

	meta, found, _ := st.GetClock(context.Background(), "case-1", ClockResponse)
	if !found {
		t.Fatal("clock not registered")
	}
	if meta.AlertedTier != 100 {
		t.Errorf("AlertedTier = %d, want 100 (every tier already past)", meta.AlertedTier)
	}
	if meta.CaseNumber != "CS0001" || meta.Team != "Team Nova" {
		t.Errorf("meta = %+v, missing expected display fields", meta)
	}
	for _, tier := range tierSequence {
		if _, ok := st.wake[wakeMember("case-1", ClockResponse, tier)]; ok {
			t.Errorf("wake entry scheduled for already-past tier %d, want none (would cause an immediate stale alert)", tier)
		}
	}
}

// TestReconcileClock_FutureTiersGetRealWakeEntries confirms the opposite
// case: a clock that just started has every tier still ahead, so all three
// get real wake entries and none are pre-claimed.
func TestReconcileClock_FutureTiersGetRealWakeEntries(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})

	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-1 * time.Minute) // CATASTROPHIC response = 15m, barely started

	e.reconcileClock(context.Background(), activeSLAClock{
		CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC",
		StartedOn: timePtr(startedAt),
	}, now)

	meta, found, _ := st.GetClock(context.Background(), "case-1", ClockResponse)
	if !found {
		t.Fatal("clock not registered")
	}
	if meta.AlertedTier != 0 {
		t.Errorf("AlertedTier = %d, want 0 (nothing past yet)", meta.AlertedTier)
	}
	for _, tier := range tierSequence {
		if _, ok := st.wake[wakeMember("case-1", ClockResponse, tier)]; !ok {
			t.Errorf("no wake entry for still-future tier %d", tier)
		}
	}
}

// TestReconcileClock_MixOfPastAndFutureTiers confirms a clock straddling
// "now" (50%/75% already past, 100% still ahead) splits correctly: the
// past tiers are claimed via the single highest-past-tier value (75, which
// already covers 50 — see processDueMember's own ">=" comparison), and only
// the 100% tier gets a real wake entry.
func TestReconcileClock_MixOfPastAndFutureTiers(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})

	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	// CATASTROPHIC response = 15m: 50%=7.5m, 75%=11.25m, 100%=15m.
	// Started 12m ago: 50%/75% are past, 100% is still ~3m away.
	startedAt := now.Add(-12 * time.Minute)

	e.reconcileClock(context.Background(), activeSLAClock{
		CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC",
		StartedOn: timePtr(startedAt),
	}, now)

	meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResponse)
	if meta.AlertedTier != 75 {
		t.Errorf("AlertedTier = %d, want 75", meta.AlertedTier)
	}
	if _, ok := st.wake[wakeMember("case-1", ClockResponse, 100)]; !ok {
		t.Error("no wake entry for the still-future 100%% tier")
	}
	for _, tier := range []int{50, 75} {
		if _, ok := st.wake[wakeMember("case-1", ClockResponse, tier)]; ok {
			t.Errorf("wake entry scheduled for already-past tier %d, want none", tier)
		}
	}
}

// TestReconcileClock_PausedSetsPausedFlag confirms a Postgres row reported
// as PAUSED (stage='PAUSED') carries that through to Redis, so Tick's own
// "drop if paused" check (see processDueMember) takes effect immediately
// rather than only after the next real pause event.
func TestReconcileClock_PausedSetsPausedFlag(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})

	e.reconcileClock(context.Background(), activeSLAClock{
		CaseID: "case-1", ClockType: ClockWorkaround, Priority: "CATASTROPHIC",
		StartedOn: timePtr(time.Now()), IsPaused: true,
	}, time.Now())

	meta, found, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround)
	if !found || !meta.Paused {
		t.Errorf("meta = %+v (found=%v), want Paused=true", meta, found)
	}
}

// TestReconcileClock_UnknownSeverityOrClockType_SkipsGracefully confirms a
// row this engine's duration policy doesn't cover (e.g. a severity it was
// never fetched for) is skipped, not a panic or a half-written clock.
func TestReconcileClock_UnknownSeverityOrClockType_SkipsGracefully(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})

	e.reconcileClock(context.Background(), activeSLAClock{
		CaseID: "case-1", ClockType: ClockResponse, Priority: "UNKNOWN", StartedOn: timePtr(time.Now()),
	}, time.Now())
	e.reconcileClock(context.Background(), activeSLAClock{
		CaseID: "case-2", ClockType: ClockWorkaround, Priority: "LOW", StartedOn: timePtr(time.Now()),
	}, time.Now())
	e.reconcileClock(context.Background(), activeSLAClock{
		CaseID: "case-3", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: nil,
	}, time.Now())

	if len(st.clocks) != 0 {
		t.Errorf("clocks = %+v, want none registered", st.clocks)
	}
}

// TestEngine_Reconcile_ProcessesEveryClockFromTheClient is the end-to-end
// path: Reconcile fetches the client's full list and rebuilds each one.
func TestEngine_Reconcile_ProcessesEveryClockFromTheClient(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	client := &fakeActiveClocksClient{clocks: []activeSLAClock{
		{CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now())},
		{CaseID: "case-2", ClockType: ClockWorkaround, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), IsPaused: true},
	}}

	if err := e.Reconcile(context.Background(), client); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if _, found, _ := st.GetClock(context.Background(), "case-1", ClockResponse); !found {
		t.Error("case-1 response clock not rebuilt")
	}
	meta, found, _ := st.GetClock(context.Background(), "case-2", ClockWorkaround)
	if !found || !meta.Paused {
		t.Errorf("case-2 workaround clock = %+v (found=%v), want Paused=true", meta, found)
	}
}

// TestEngine_Reconcile_ClientErrorPropagates confirms a fetch failure is
// returned (so main.go can log it) rather than silently swallowed.
func TestEngine_Reconcile_ClientErrorPropagates(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	client := &fakeActiveClocksClient{err: errors.New("entity-service unreachable")}

	if err := e.Reconcile(context.Background(), client); err == nil {
		t.Fatal("Reconcile() error = nil, want the client's error propagated")
	}
}

// raceSimulatingStore wraps fakeStore and, right after a ClaimTier call
// succeeds for a chosen (caseID, clockType), simulates a concurrent
// registration replacing that clock with a new incarnation -- the exact
// race window between processDueMember's own GetClock read and its later
// AdvanceAlertedTier/RemoveWake calls that a CodeRabbit review caught: a
// second replica (or an overlapping Reconcile pass) landing a severity
// revision's fresh clock in that window.
type raceSimulatingStore struct {
	*fakeStore
	caseID, clockType string
	replacement       ClockMeta
	replacementWakeAt time.Time
}

func (r *raceSimulatingStore) ClaimTier(ctx context.Context, caseID, clockType string, tier int, startedAt time.Time) (bool, error) {
	claimed, err := r.fakeStore.ClaimTier(ctx, caseID, clockType, tier, startedAt)
	if claimed && err == nil && caseID == r.caseID && clockType == r.clockType {
		r.fakeStore.clocks[caseID+"|"+clockType] = r.replacement
		r.fakeStore.wake[wakeMember(caseID, clockType, tier)] = r.replacementWakeAt
	}
	return claimed, err
}

// TestEngine_ProcessDueMember_IncarnationChangedMidFlight_DoesNotClobberNewClock
// is the direct regression test for the CodeRabbit-flagged race: a tick
// that already won its (incarnation-scoped) claim under an OLD clock must
// not let its own AdvanceAlertedTier/RemoveWake calls afterward corrupt a
// DIFFERENT, NEWER incarnation that was registered for the identical
// (caseID, clockType) in the meantime -- neither clobbering its
// just-reset alerted-tier cursor nor deleting its own, independently
// scheduled wake entry.
func TestEngine_ProcessDueMember_IncarnationChangedMidFlight_DoesNotClobberNewClock(t *testing.T) {
	base := newFakeStore()
	oldStartedAt := time.Now().Add(-time.Hour)
	newStartedAt := time.Now().Add(time.Minute) // a genuinely different incarnation
	newWakeAt := time.Now().Add(time.Hour)       // the new clock's own, still-future tier-50 due time

	base.clocks["case-1|response"] = ClockMeta{StartedAt: oldStartedAt, Priority: "CATASTROPHIC"}
	base.wake[wakeMember("case-1", "response", 50)] = time.Now().Add(-time.Minute) // due now, under the OLD incarnation

	raceStore := &raceSimulatingStore{
		fakeStore:         base,
		caseID:            "case-1",
		clockType:         "response",
		replacement:       ClockMeta{StartedAt: newStartedAt, Priority: "HIGH"},
		replacementWakeAt: newWakeAt,
	}
	e := &Engine{store: raceStore, pub: &fakePublisher{}, chat: &fakeChat{}, links: fakeLinks{}, durations: testDurations()}

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}

	got := base.clocks["case-1|response"]
	if !got.StartedAt.Equal(newStartedAt) {
		t.Fatalf("test setup broken: clock is not the new incarnation (got StartedAt = %v)", got.StartedAt)
	}
	if got.AlertedTier != 0 {
		t.Errorf("new incarnation's AlertedTier = %d, want 0 -- the stale tick must not advance a cursor that isn't its own", got.AlertedTier)
	}
	gotWakeAt, stillPresent := base.wake[wakeMember("case-1", "response", 50)]
	if !stillPresent {
		t.Fatal("new incarnation's own wake entry was removed by the stale tick -- its real future alert is now lost")
	}
	if !gotWakeAt.Equal(newWakeAt) {
		t.Errorf("wake entry due time = %v, want the new incarnation's own %v (unchanged)", gotWakeAt, newWakeAt)
	}
}
