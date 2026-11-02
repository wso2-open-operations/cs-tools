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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/businessdays"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// fakeStore is an in-memory implementation of the store interface —
// everything this package's tests need, with no real Redis.
type fakeStore struct {
	wake          map[string]time.Time
	clocks        map[string]ClockMeta
	claims        map[string]bool
	failClaim     bool
	failAdd       bool
	failAdvance   bool
	failSetPaused bool
	failWipe      bool
	// failWipeAttempts, when > 0, fails exactly that many WipeAll calls
	// before letting one through -- simulates a transient Redis blip that
	// withRedisRetry's own retry clears, distinct from failWipe's "always
	// fails" (retries exhausted).
	failWipeAttempts int
	wipeCalls        int
	// failAddAttempts, when > 0, fails exactly that many AddWake calls
	// (counted across the whole fakeStore, in call order) before letting
	// one through -- mirrors failWipeAttempts, for simulating a clock that
	// fails RecalculateFromEntity's first pass and succeeds on its repair
	// pass retry.
	failAddAttempts int
	addWakeCalls    int
	// failListClockKeys/failDeleteClock simulate a failure in
	// RecalculateFromEntity's own cleanup step (cleanupStaleClocks).
	failListClockKeys bool
	failDeleteClock   bool
	// afterListClockKeys, when set, runs immediately after ListClockKeys has
	// already captured its own snapshot (but before returning it) -- lets a
	// test simulate a clock being registered concurrently, in the narrow
	// window between RecalculateFromEntity's own pre-recalculation snapshot
	// and the rest of the call.
	afterListClockKeys func()
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		wake:   map[string]time.Time{},
		clocks: map[string]ClockMeta{},
		claims: map[string]bool{},
	}
}

func (s *fakeStore) AddWake(_ context.Context, member string, at time.Time) error {
	s.addWakeCalls++
	if s.failAdd || s.addWakeCalls <= s.failAddAttempts {
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
	if s.failSetPaused {
		return errors.New("set paused failed")
	}
	key := caseID + "|" + clockType
	meta := s.clocks[key]
	meta.Paused = paused
	s.clocks[key] = meta
	return nil
}

// ShiftWake mirrors the real Store's own shiftWakeScript: only a member
// already present in wake is moved, same "never resurrect an
// already-alerted tier" guard the real ZSCORE check gives, and the new
// baseline is recorded in the same call — see Store.ShiftWake's own doc
// comment for why these two writes must happen together.
func (s *fakeStore) ShiftWake(_ context.Context, caseID, clockType string, deltaSeconds, newBaselineSeconds int64) error {
	if deltaSeconds != 0 {
		for _, tier := range tierSequence {
			member := wakeMember(caseID, clockType, tier)
			if at, ok := s.wake[member]; ok {
				s.wake[member] = at.Add(time.Duration(deltaSeconds) * time.Second)
			}
		}
	}
	key := caseID + "|" + clockType
	meta := s.clocks[key]
	meta.ShiftedPauseSeconds = newBaselineSeconds
	s.clocks[key] = meta
	return nil
}

func (s *fakeStore) SetWakeAbsolute(_ context.Context, caseID, clockType string, wakes map[int]time.Time, newBaselineSeconds int64) error {
	for tier, at := range wakes {
		member := wakeMember(caseID, clockType, tier)
		if _, ok := s.wake[member]; ok {
			s.wake[member] = at
		}
	}
	key := caseID + "|" + clockType
	meta := s.clocks[key]
	meta.ShiftedPauseSeconds = newBaselineSeconds
	s.clocks[key] = meta
	return nil
}

func (s *fakeStore) SetShiftedPauseSeconds(_ context.Context, caseID, clockType string, seconds int64) error {
	key := caseID + "|" + clockType
	meta := s.clocks[key]
	meta.ShiftedPauseSeconds = seconds
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

func (s *fakeStore) WipeAll(_ context.Context) error {
	s.wipeCalls++
	if s.failWipe || s.wipeCalls <= s.failWipeAttempts {
		return errors.New("redis: wipe failed")
	}
	s.wake = map[string]time.Time{}
	s.clocks = map[string]ClockMeta{}
	s.claims = map[string]bool{}
	return nil
}

// failListClockKeys simulates a listing failure in RecalculateFromEntity's
// own cleanup step -- mirrors the real Store.ListClockKeys' error path.
func (s *fakeStore) ListClockKeys(_ context.Context) ([]string, error) {
	if s.failListClockKeys {
		return nil, errors.New("list clock keys failed")
	}
	ids := make([]string, 0, len(s.clocks))
	for id := range s.clocks {
		ids = append(ids, id)
	}
	if s.afterListClockKeys != nil {
		s.afterListClockKeys()
	}
	return ids, nil
}

// DeleteClock mirrors the real Store.DeleteClock: removes the clock hash and
// every tier's wake entry (a delete on an absent wake member is a no-op,
// same as the real RemoveWake).
func (s *fakeStore) DeleteClock(_ context.Context, caseID, clockType string) error {
	if s.failDeleteClock {
		return errors.New("delete clock failed")
	}
	delete(s.clocks, caseID+"|"+clockType)
	for _, tier := range tierSequence {
		delete(s.wake, wakeMember(caseID, clockType, tier))
	}
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

// fakeEmail is an emailSender test double.
type fakeEmail struct {
	calls   []fakeEmailCall
	failErr error
}

type fakeEmailCall struct {
	to                []string
	subject, htmlBody string
}

func (f *fakeEmail) SendEmail(_ context.Context, to, _, _, _ []string, subject, htmlBody string, _ []notifications.EmailAttachment) error {
	f.calls = append(f.calls, fakeEmailCall{to: to, subject: subject, htmlBody: htmlBody})
	return f.failErr
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

// weekdayDaytime is a Wednesday at 11:00 IST, outside both the Americas
// (21:00-06:00 IST) and weekend coverage windows, so chataudience.Resolve
// adds no audience that depends on when the test happens to run.
var weekdayDaytime = time.Date(2026, time.October, 14, 5, 30, 0, 0, time.UTC)

func newTestEngine(st store, chat *fakeChat, pub *fakePublisher) *Engine {
	return &Engine{store: st, pub: pub, chat: chat, links: fakeLinks{}, slaEmailSendingEnabled: true,
		now: func() time.Time { return weekdayDaytime }}
}

// entityWithCatastrophicDurations returns a fakeEntityClockStatus answering
// every clock type for caseID as Found, with CATASTROPHIC's own
// response/workaround/resolution durations (15m/4h/48h) — the shared
// default most RegisterClocks-driven tests in this file use as scaffolding
// (RegisterClocks now fetches each clock's real duration live from
// entity-service, per clock type, rather than a static severity-keyed table
// — see that function's own doc comment), so each test only needs to say
// what's different from it.
func entityWithCatastrophicDurations(caseID string) *fakeEntityClockStatus {
	return &fakeEntityClockStatus{states: map[string]clockState{
		caseID + "|" + ClockResponse:   {Found: true, DurationSeconds: int64((15 * time.Minute).Seconds())},
		caseID + "|" + ClockWorkaround: {Found: true, DurationSeconds: int64((4 * time.Hour).Seconds())},
		caseID + "|" + ClockResolution: {Found: true, DurationSeconds: int64((48 * time.Hour).Seconds())},
	}}
}

// --- RegisterClocks ---

func TestRegisterClocks_RegistersEveryClockThePolicyHasForThisSeverity(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	createdAt := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC) // a Monday

	e.entity = entityWithCatastrophicDurations("case-1")
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

// TestRegisterClocks_SkipsClockTypesThePolicyHasNoEntryFor verifies a case
// whose entity-service record has a duration for RESPONSE only (the real
// shape a LOW/S4 case takes against the live table today) registers exactly
// one clock, not all three — RegisterClocks fetches each clock type's
// duration independently, so a Found=false answer for workaround/resolution
// simply skips those, the same "no fallback duration" posture the old
// static table's absent entry had.
func TestRegisterClocks_SkipsClockTypesThePolicyHasNoEntryFor(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockResponse: {Found: true, DurationSeconds: int64((24 * time.Hour).Seconds())},
	}}
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

// TestRegisterClocks_NoEntityClient_RegistersNothing covers what "unknown
// severity" used to mean under the old static table: with no entity-service
// client configured at all (or, equivalently, one that reports Found=false
// for every clock type), there is nothing to compute a due date from, so
// nothing registers.
func TestRegisterClocks_NoEntityClient_RegistersNothing(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.RegisterClocks(context.Background(), "case-1", "UNKNOWN", time.Now(), "CS0001", "", "", "CASE", "", "")

	if len(st.clocks) != 0 {
		t.Errorf("clocks registered with no entity-service client configured: %+v", st.clocks)
	}
}

// TestRegisterClocks_EntityReportsNoPolicy_RegistersNothing is the same
// outcome via the other route: an entity-service client IS configured, but
// it reports Found=false for every clock type (no sla_policy currently
// resolves for this case) — same "no fallback duration" result.
func TestRegisterClocks_EntityReportsNoPolicy_RegistersNothing(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = &fakeEntityClockStatus{} // no states configured -- every lookup reports Found: false
	e.RegisterClocks(context.Background(), "case-1", "ANNOUNCEMENT", time.Now(), "CS0001", "", "", "ANNOUNCEMENT", "", "")

	if len(st.clocks) != 0 {
		t.Errorf("clocks registered when entity-service has no applicable policy: %+v", st.clocks)
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
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", createdAt, "CS0001", "", "", "CASE", "", "")

	// A later event completes the response clock and pauses workaround --
	// simulating CompleteResponseClock/ApplyStateEffects having already run
	// before the replay below. entity-service's own record is configured to
	// report workaround/resolution as paused, same as the real sequencing
	// (its own pause_condition evaluation lands before this Kafka consumer
	// sees the event).
	e.CompleteResponseClock(context.Background(), "case-1")
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockWorkaround: {Found: true, IsPaused: true},
		"case-1|" + ClockResolution: {Found: true, IsPaused: true},
	}}
	e.ApplyStateEffects(context.Background(), "case-1", "Awaiting Info")

	// case.created is redelivered (retry or DLQ replay) with the identical
	// payload.
	e.entity = entityWithCatastrophicDurations("case-1")
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
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockResponse:   {Found: true, DurationSeconds: int64((6 * time.Hour).Seconds())},
		"case-1|" + ClockWorkaround: {Found: true, DurationSeconds: int64((72 * time.Hour).Seconds())},
		"case-1|" + ClockResolution: {Found: true, DurationSeconds: int64((7 * 24 * time.Hour).Seconds())},
	}}
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

// TestRegisterClocks_BusinessDaySchedule_WakesAtTheBusinessDayAdjustedInstant
// is the regression test for the code-review-found bug: a 'WSO2 Business
// days' RESOLUTION clock must be woken at the real business-day-adjusted
// instant (skipping weekends), not at flat calendar time -- a 240h
// (10-business-day) clock starting on a Friday was previously woken at
// Friday+240h calendar time (ten calendar days later, landing on the
// following Monday) instead of the correct instant two business weeks
// later, which skips two weekends and therefore lands twelve calendar days
// out.
func TestRegisterClocks_BusinessDaySchedule_WakesAtTheBusinessDayAdjustedInstant(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})

	createdAt := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC) // a Friday
	duration := 240 * time.Hour                              // 10 business days
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockResolution: {Found: true, DurationSeconds: int64(duration.Seconds()), Schedule: "WSO2 Business days"},
	}}
	e.RegisterClocks(context.Background(), "case-1", "LOW", createdAt, "CS0001", "", "", "CASE", "", "")

	due100, ok := st.wake[wakeMember("case-1", ClockResolution, 100)]
	if !ok {
		t.Fatal("no wake entry for the resolution 100% tier")
	}
	wrongFlatDue := createdAt.Add(duration)
	if due100.Equal(wrongFlatDue) {
		t.Errorf("due100 = %v, equals the WRONG flat-wall-clock due date %v -- business-day math was not applied", due100, wrongFlatDue)
	}
	// The correct, business-day-adjusted due date: from Jan 2 09:00 UTC
	// (04:00 America/New_York, a Friday), the remaining 20h of that Friday
	// plus Jan 5-9 (120h) plus Jan 12-15 (96h) totals 236h, leaving 4h into
	// Jan 16 (a Friday) -- 04:00 NY = 09:00 UTC.
	wantDue := time.Date(2026, 1, 16, 9, 0, 0, 0, time.UTC)
	if !due100.Equal(wantDue) {
		t.Errorf("due100 = %v, want %v", due100, wantDue)
	}
	// The 50% tier (120h of business time) must also be business-day-aware,
	// not simply half of the flat wall-clock span.
	due50, ok := st.wake[wakeMember("case-1", ClockResolution, 50)]
	if !ok {
		t.Fatal("no wake entry for the resolution 50% tier")
	}
	wrongFlatHalf := createdAt.Add(duration / 2)
	if due50.Equal(wrongFlatHalf) {
		t.Errorf("due50 = %v, equals the WRONG flat-wall-clock half-point %v", due50, wrongFlatHalf)
	}
}

// TestReconcileClock_BusinessDaySchedule_WakesAtTheBusinessDayAdjustedInstant
// is reconcileClock's own counterpart to the RegisterClocks regression test
// above -- the Redis-rebuild path must apply the identical business-day math.
func TestReconcileClock_BusinessDaySchedule_WakesAtTheBusinessDayAdjustedInstant(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})

	startedAt := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC) // a Friday
	duration := 240 * time.Hour
	now := startedAt // reconcile immediately, before anything is due

	e.reconcileClock(context.Background(), activeSLAClock{
		CaseID: "case-1", ClockType: ClockResolution, Priority: "LOW",
		StartedOn: timePtr(startedAt), DurationSeconds: int64(duration.Seconds()),
		Schedule: "WSO2 Business days",
	}, now)

	due100, ok := st.wake[wakeMember("case-1", ClockResolution, 100)]
	if !ok {
		t.Fatal("no wake entry for the resolution 100% tier")
	}
	wrongFlatDue := startedAt.Add(duration)
	if due100.Equal(wrongFlatDue) {
		t.Errorf("due100 = %v, equals the WRONG flat-wall-clock due date %v", due100, wrongFlatDue)
	}
	wantDue := time.Date(2026, 1, 16, 9, 0, 0, 0, time.UTC)
	if !due100.Equal(wantDue) {
		t.Errorf("due100 = %v, want %v", due100, wantDue)
	}
}

// --- ApplyStateEffects ---

// TestApplyStateEffects_AwaitingInfo_PausesWorkaroundAndResolution confirms
// ApplyStateEffects applies whatever pause status entity-service's own
// record reports for each clock — it is entity-service's answer, not the
// literal status string "Awaiting Info", that decides pause/resume now (see
// ApplyStateEffects' own doc comment for why the old hardcoded status-string
// list was removed).
func TestApplyStateEffects_AwaitingInfo_PausesWorkaroundAndResolution(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")

	// entity-service's own pause_condition already evaluated this case's new
	// status to "paused" for both clocks by the time this event is
	// processed (its own pause/resume effects apply synchronously, in the
	// same request that published case.status_changed).
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockWorkaround: {Found: true, IsPaused: true},
		"case-1|" + ClockResolution: {Found: true, IsPaused: true},
	}}
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

func TestApplyStateEffects_Closed_CompletesResolutionAndResponseAndPausesWorkaround(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")

	e.ApplyStateEffects(context.Background(), "case-1", "Closed")

	meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResolution)
	if meta.AlertedTier != 100 {
		t.Errorf("resolution AlertedTier = %d, want 100 (force-completed on close)", meta.AlertedTier)
	}
	if wMeta, _, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround); !wMeta.Paused {
		t.Error("workaround clock not paused on close")
	}
	// Regression for a real reported bug: a case that
	// closes without ever getting a qualifying support-engineer reply
	// (CompleteResponseClock never called -- a work note doesn't qualify,
	// and neither does a comment whose author didn't resolve as a
	// recognized CS-engineer role) used to leave the response clock
	// neither paused nor completed, so its Redis wake entries stayed live
	// and a breach alert fired well after the case had already closed.
	rMeta, _, _ := st.GetClock(context.Background(), "case-1", ClockResponse)
	if rMeta.AlertedTier != 100 {
		t.Errorf("response AlertedTier = %d, want 100 (force-completed on close, same as resolution)", rMeta.AlertedTier)
	}
}

// TestApplyStateEffects_Closed_WithoutAQualifyingReply_NeverAlertsOnResponse
// is the end-to-end regression for a real reported bug: a case closes with no
// CompleteResponseClock ever called (no qualifying support-engineer reply --
// only a work note was added), and a Tick long after close must not fire a
// response-clock breach alert.
func TestApplyStateEffects_Closed_WithoutAQualifyingReply_NeverAlertsOnResponse(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	e := newTestEngine(st, chat, &fakePublisher{})
	start := time.Now().Add(-2 * time.Hour)
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", start, "CS0001", "", "", "CASE", "", "")

	// The case closes with no qualifying support-engineer reply ever having
	// completed the response clock -- matching the issue's own repro (a
	// comment and a work note were added, neither of which qualifies).
	e.ApplyStateEffects(context.Background(), "case-1", "Closed")

	// Now time passes well beyond every tier's original due date (the
	// issue's own "wait more than 1 hour" step) and a tick runs.
	if err := e.Tick(context.Background(), time.Now().Add(1*time.Hour)); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	for _, c := range chat.calls {
		if c.clockType == ClockResponse {
			t.Fatalf("a response-clock breach alert was sent after the case closed with no qualifying reply: %+v", c)
		}
	}
}

// TestApplyStateEffects_Closed_NoClockTypeAlertsAfterClose is the same proof
// as the response-only test above, but for all three clock types at once,
// closing a case directly from "Open" (no intermediate pause) with no
// WorkaroundProvided-equivalent signal and no qualifying reply ever having
// run -- the worst case for every clock. Workaround is only paused on
// close (not completed, a documented accepted gap -- see ApplyStateEffects'
// own doc comment), but processDueMember drops a paused clock's wake entry
// without alerting regardless of tier, so it must be just as silent as the
// two force-completed clocks here.
func TestApplyStateEffects_Closed_NoClockTypeAlertsAfterClose(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	e := newTestEngine(st, chat, &fakePublisher{})
	start := time.Now().Add(-2 * time.Hour)
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", start, "CS0001", "", "", "CASE", "", "")

	e.ApplyStateEffects(context.Background(), "case-1", "Closed")

	if err := e.Tick(context.Background(), time.Now().Add(1*time.Hour)); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(chat.calls) != 0 {
		t.Fatalf("expected no breach alerts of any clock type after close, got: %+v", chat.calls)
	}
}

func TestApplyStateEffects_ResumesOnAnyOtherStatus(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockWorkaround: {Found: true, IsPaused: true},
		"case-1|" + ClockResolution: {Found: true, IsPaused: true},
	}}
	e.ApplyStateEffects(context.Background(), "case-1", "Awaiting Info")

	// entity-service now reports both clocks resumed.
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockWorkaround: {Found: true, IsPaused: false},
		"case-1|" + ClockResolution: {Found: true, IsPaused: false},
	}}
	e.ApplyStateEffects(context.Background(), "case-1", "Work In Progress")

	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround); meta.Paused {
		t.Error("workaround clock still paused after resuming status")
	}
	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResolution); meta.Paused {
		t.Error("resolution clock still paused after resuming status")
	}
}

// TestApplyStateEffects_Resume_ShiftsWakeEntriesByTheNewPauseSegment is the
// end-to-end regression for the real incident this package's own
// resumeShift exists to fix: a clock paused across one of its own tiers
// used to lose that tier's alert forever (see processDueMember's old
// behavior, pinned until this change by
// TestTick_PausedClock_DropsWakeEntryWithoutAlerting). Now a resume reads
// entity-service's own accumulated pause_duration and pushes every
// not-yet-alerted tier forward by the newly-completed segment's length.
func TestApplyStateEffects_Resume_ShiftsWakeEntriesByTheNewPauseSegment(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	createdAt := time.Now()
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", createdAt, "CS0001", "", "", "CASE", "", "")

	before := map[string]time.Time{
		ClockWorkaround: st.wake[wakeMember("case-1", ClockWorkaround, 50)],
		ClockResolution: st.wake[wakeMember("case-1", ClockResolution, 50)],
	}

	e.ApplyStateEffects(context.Background(), "case-1", "Awaiting Info")
	// entity-service's own CSM-native engine has, by the time this Kafka
	// event is processed, already accumulated 2 hours of pause_duration for
	// this clock in its own durable row.
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|workaround": {Found: true, PauseDurationSeconds: 7200},
		"case-1|resolution": {Found: true, PauseDurationSeconds: 7200},
	}}

	e.ApplyStateEffects(context.Background(), "case-1", "Work In Progress")

	for _, ct := range []string{ClockWorkaround, ClockResolution} {
		got := st.wake[wakeMember("case-1", ct, 50)]
		want := before[ct].Add(2 * time.Hour)
		if !got.Equal(want) {
			t.Errorf("%s 50%% tier due at %v after resume, want %v (shifted by the 2h pause)", ct, got, want)
		}
		meta, _, _ := st.GetClock(context.Background(), "case-1", ct)
		if meta.ShiftedPauseSeconds != 7200 {
			t.Errorf("%s ShiftedPauseSeconds = %d, want 7200", ct, meta.ShiftedPauseSeconds)
		}
		if meta.Paused {
			t.Errorf("%s still marked paused after resume", ct)
		}
	}
	// response is never touched by ApplyStateEffects at all.
	if _, exists := st.clocks["case-1|response"]; !exists {
		t.Fatal("response clock missing from fake store")
	}
}

// TestApplyStateEffects_Resume_BusinessDayClock_RecomputesWakeTimesRatherThanShiftingFlat
// is the regression test for a real, PR-review-found bug: a business-day
// clock's wake entries used to be shifted by a FLAT wall-clock pause delta
// (ShiftWake's own ZINCRBY), while entity-service computes a business-day
// clock's real due instant as AddBusinessDuration(start, duration +
// businessPause) -- a flat increment and a business-day-aware recomputation
// disagree by however much of the pause segment actually fell on a weekend.
// This clock is registered with a 116-hour ('WSO2 Business days') resolution
// duration, started Monday 2024-01-08 00:00 America/New_York, and paused
// across a weekend-spanning segment; the 100% tier's wake time after resume
// must equal AddBusinessDuration(start, duration+businessPause), not
// start+duration shifted by the pause's flat wall-clock length.
func TestApplyStateEffects_Resume_BusinessDayClock_RecomputesWakeTimesRatherThanShiftingFlat(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	startedAt := time.Date(2024, 1, 8, 0, 0, 0, 0, mustLoadLocation(t, "America/New_York")) // a Monday
	duration := 116 * time.Hour

	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockResolution: {Found: true, DurationSeconds: int64(duration.Seconds()), Schedule: slaScheduleWSO2BusinessDays},
	}}
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", startedAt, "CS0001", "", "", "CASE", "", "")

	e.ApplyStateEffects(context.Background(), "case-1", "Awaiting Info")
	// entity-service's own CSM-native engine has, by the time this Kafka
	// event is processed, already accumulated a business-hours-aware pause
	// total for this clock -- spanning a weekend, so it differs from the
	// flat wall-clock pause length.
	businessPause := 8 * time.Hour
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockResolution: {
			Found: true, DurationSeconds: int64(duration.Seconds()), Schedule: slaScheduleWSO2BusinessDays,
			BusinessPauseDurationSeconds: int64(businessPause.Seconds()),
		},
	}}

	e.ApplyStateEffects(context.Background(), "case-1", "Work In Progress")

	got := st.wake[wakeMember("case-1", ClockResolution, 100)]
	want := businessdays.AddBusinessDuration(startedAt, duration+businessPause)
	if !got.Equal(want) {
		t.Errorf("resolution 100%% tier due at %v after resume, want %v (business-day-recomputed, not flat-shifted)", got, want)
	}

	meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResolution)
	if meta.ShiftedPauseSeconds != int64(businessPause.Seconds()) {
		t.Errorf("ShiftedPauseSeconds = %d, want %d (the business pause total, not a flat delta)", meta.ShiftedPauseSeconds, int64(businessPause.Seconds()))
	}
}

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("time.LoadLocation(%q): %v", name, err)
	}
	return loc
}

// TestApplyStateEffects_Resume_NoEntityClient_SkipsShiftWithoutPanicking
// confirms the fail-safe posture documented on resumeShift/entityPauseStatus:
// with no entity client wired up, a status change that would otherwise
// resume a currently-paused clock still clears the paused flag (fail-open
// default) and resumeShift's own pause-duration lookup is skipped without
// panicking.
func TestApplyStateEffects_Resume_NoEntityClient_SkipsShiftWithoutPanicking(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")
	// Pause the clock directly (bypassing ApplyStateEffects' own
	// entity-based pause detection, which is not what this test is about)
	// to set up the "currently paused" precondition a resume needs.
	if err := st.SetPaused(context.Background(), "case-1", ClockWorkaround, true); err != nil {
		t.Fatalf("seed pause: %v", err)
	}

	e.entity = nil
	e.ApplyStateEffects(context.Background(), "case-1", "Work In Progress")

	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround); meta.Paused {
		t.Error("workaround clock still paused after resuming with no entity client wired up")
	}
}

// --- CompleteResponseClock ---

func TestCompleteResponseClock_AdvancesToTier100(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")

	e.CompleteResponseClock(context.Background(), "case-1")

	meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResponse)
	if meta.AlertedTier != 100 {
		t.Errorf("response AlertedTier = %d, want 100", meta.AlertedTier)
	}
}

// --- PauseWorkaroundClock ---

// TestPauseWorkaroundClock_SetsPaused is the regression test for a real,
// reported bug found reviewing entity-service's own matching fix: this
// engine used to force-complete (AdvanceAlertedTier to 100 -- a one-way
// ratchet nothing can undo) the workaround clock on workaroundProvided:true,
// matching entity-service's own now-fixed CompleteWorkaroundClock hook. The
// real WORKAROUND-target policies pause this clock on that same signal,
// they don't complete it, so this engine's independent copy of the same old
// mistake had to be fixed the same way: PAUSE, not complete.
func TestPauseWorkaroundClock_SetsPaused(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")

	if err := e.PauseWorkaroundClock(context.Background(), "case-1"); err != nil {
		t.Fatalf("PauseWorkaroundClock() error = %v", err)
	}

	meta, _, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround)
	if !meta.Paused {
		t.Error("workaround clock Paused = false, want true")
	}
	if meta.AlertedTier != 0 {
		t.Errorf("workaround AlertedTier = %d, want 0 -- a pause must not touch the alerted-tier cursor the way the old force-complete did", meta.AlertedTier)
	}
}

// TestPauseWorkaroundClock_StoreFailure_ReturnsError confirms a store
// failure here is still returned, not just logged, so
// dispatch.handleWorkaroundProvided can fail the record for a retry instead
// of silently acknowledging a pause that was never actually applied.
func TestPauseWorkaroundClock_StoreFailure_ReturnsError(t *testing.T) {
	st := newFakeStore()
	st.failSetPaused = true
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")

	if err := e.PauseWorkaroundClock(context.Background(), "case-1"); err == nil {
		t.Fatal("expected PauseWorkaroundClock() to return an error on a store failure, got nil")
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

// TestTick_BreachAlert_IncidentMonitorFallbackRetargetsToSLADefault is the
// regression test for the SLA-breach-alert-specific webhook split: a case
// with no team-specific Chat space resolves to chataudience.IncidentMonitor
// (fakeChat's default HasAudienceSpace), but sendBreachAlert must post it
// under chataudience.SLADefault instead -- a separate incoming webhook into
// the same Google Chat space, not a different space -- so the shared
// Incident Monitor webhook no longer carries SLA breach traffic at all.
func TestTick_BreachAlert_IncidentMonitorFallbackRetargetsToSLADefault(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)

	past := time.Now().Add(-time.Minute)
	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) != 1 {
		t.Fatalf("chat calls = %+v, want exactly one", chat.calls)
	}
	if got := chat.calls[0].audience; got != "SLA Default" {
		t.Errorf("audience = %q, want %q (the Incident Monitor fallback must retarget to the SLA webhook)", got, "SLA Default")
	}
}

// TestTick_BreachAlert_RealTeamAudienceIsNotRetargeted confirms the
// retargeting above is scoped to the literal IncidentMonitor fallback only
// -- a case whose team has its own configured Chat space must still post
// there under the team's own name, unaffected by the SLADefault split.
func TestTick_BreachAlert_RealTeamAudienceIsNotRetargeted(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{spaces: map[string]bool{"Team Nova": true}}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)

	past := time.Now().Add(-time.Minute)
	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", Team: "Team Nova", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) != 1 {
		t.Fatalf("chat calls = %+v, want exactly one", chat.calls)
	}
	if got := chat.calls[0].audience; got != "Team Nova" {
		t.Errorf("audience = %q, want %q (a real team's own space must not be retargeted)", got, "Team Nova")
	}
}

// TestTick_SLANotificationsDisabled_SkipsChatButKeepsPublishAndCursor guards
// SLA_EMAIL_SENDING_ENABLED=false: no Chat card (and no entity-service
// lookup), but the event is still published and the tier is still marked
// alerted and removed from the wake index, so re-enabling cannot replay it.
func TestTick_SLANotificationsDisabled_SkipsChatButKeepsPublishAndCursor(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	ent := &fakeEntityClockStatus{}
	e := newTestEngine(st, chat, pub)
	e.slaEmailSendingEnabled = false
	e.entity = ent

	past := time.Now().Add(-time.Minute)
	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", Team: "Team Nova", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) != 0 {
		t.Errorf("chat calls = %+v, want none when SLA notifications are disabled", chat.calls)
	}
	if len(ent.calls) != 0 {
		t.Errorf("entity lookups = %v, want none when disabled", ent.calls)
	}
	if pub.calls != 1 {
		t.Errorf("publish calls = %d, want 1 (event still published)", pub.calls)
	}
	if _, stillWaiting := st.wake[wakeMember("case-1", "response", 50)]; stillWaiting {
		t.Error("wake entry not removed")
	}
	meta, _, _ := st.GetClock(context.Background(), "case-1", "response")
	if meta.AlertedTier != 50 {
		t.Errorf("AlertedTier = %d, want 50 (cursor advances so re-enable does not flood)", meta.AlertedTier)
	}
}

// fakeEntityClockStatus is an entityClockStatusClient test double.
type fakeEntityClockStatus struct {
	// states, keyed by "<caseID>|<clockType>", answers GetClockState; a key
	// this map doesn't mention defaults to clockState{Found: false} (never
	// registered in entity-service), so a test only needs to name the clock
	// it specifically cares about.
	states map[string]clockState
	err    error
	calls  []string // "<caseID>|<clockType>", in call order
}

func (f *fakeEntityClockStatus) GetClockState(_ context.Context, caseID, clockType string) (clockState, error) {
	f.calls = append(f.calls, caseID+"|"+clockType)
	if f.err != nil {
		return clockState{}, f.err
	}
	return f.states[caseID+"|"+clockType], nil
}

// flakyThenFoundEntityClockStatus answers "not found" for the first
// notFoundCalls lookups of a given (caseID, clockType), then found -- a
// fixture for the entity-service-hasn't-registered-the-clock-yet race
// getClockStateWithRetry exists to close.
type flakyThenFoundEntityClockStatus struct {
	notFoundCalls int
	calls         int
	found         clockState
}

func (f *flakyThenFoundEntityClockStatus) GetClockState(_ context.Context, caseID, clockType string) (clockState, error) {
	f.calls++
	if f.calls <= f.notFoundCalls {
		return clockState{Found: false}, nil
	}
	return f.found, nil
}

func TestGetClockStateWithRetry(t *testing.T) {
	t.Run("found on the first attempt makes no extra calls", func(t *testing.T) {
		entity := &flakyThenFoundEntityClockStatus{notFoundCalls: 0, found: clockState{Found: true, DurationSeconds: 900}}
		state, err := getClockStateWithRetry(context.Background(), entity, "case-1", "response")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !state.Found || entity.calls != 1 {
			t.Errorf("state.Found=%v calls=%d, want Found=true calls=1", state.Found, entity.calls)
		}
	})

	t.Run("not found once then found succeeds within the retry budget", func(t *testing.T) {
		entity := &flakyThenFoundEntityClockStatus{notFoundCalls: 1, found: clockState{Found: true, DurationSeconds: 900}}
		start := time.Now()
		state, err := getClockStateWithRetry(context.Background(), entity, "case-1", "response")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !state.Found {
			t.Error("state.Found = false, want true after the retry catches up")
		}
		if entity.calls != entityClockStateRetryAttempts {
			t.Errorf("calls = %d, want exactly %d (found on the last allowed attempt)", entity.calls, entityClockStateRetryAttempts)
		}
		if elapsed := time.Since(start); elapsed < entityClockStateRetryDelay {
			t.Errorf("elapsed = %v, want at least one retry delay (%v)", elapsed, entityClockStateRetryDelay)
		}
	})

	t.Run("never found returns the last not-found answer after exhausting retries", func(t *testing.T) {
		entity := &flakyThenFoundEntityClockStatus{notFoundCalls: 1000}
		state, err := getClockStateWithRetry(context.Background(), entity, "case-1", "response")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state.Found {
			t.Error("state.Found = true, want false -- this clock genuinely never registers")
		}
		if entity.calls != entityClockStateRetryAttempts {
			t.Errorf("calls = %d, want exactly %d (the full retry budget, then give up)", entity.calls, entityClockStateRetryAttempts)
		}
	})

	t.Run("a lookup error is returned immediately, never retried", func(t *testing.T) {
		entity := &fakeEntityClockStatus{err: errors.New("entity-service unreachable")}
		_, err := getClockStateWithRetry(context.Background(), entity, "case-1", "response")
		if err == nil {
			t.Fatal("expected an error")
		}
		if len(entity.calls) != 1 {
			t.Errorf("calls = %d, want exactly 1 (an error is not retried by this helper)", len(entity.calls))
		}
	})
}

// TestRegisterClocks_ClockNotYetRegistered_RetriesAndThenRegistersIt is the
// regression test for the publish-before-register race: entity-service
// publishes case.created before its own SLA engine has registered the
// case's clocks (see sn_case_service.go's own CreateCase), so this Kafka
// consumer's very first GetClockState read can land before that write has
// happened. RegisterClocks must not give up on the first "not found".
func TestRegisterClocks_ClockNotYetRegistered_RetriesAndThenRegistersIt(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, nil, nil)
	e.entity = &flakyThenFoundEntityClockStatus{
		notFoundCalls: 1,
		found:         clockState{Found: true, DurationSeconds: int64((15 * time.Minute).Seconds())},
	}

	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS001", "WSO2-1", "Title", "case", "API Manager", "Team Nova")

	meta, found, err := st.GetClock(context.Background(), "case-1", ClockResponse)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected the response clock to be registered once the retry sees it as found")
	}
	if meta.CaseNumber != "CS001" {
		t.Errorf("CaseNumber = %q, want CS001", meta.CaseNumber)
	}
}

// TestTick_EntityServiceReportsClockAlreadyResolved_SuppressesTheAlert is the
// regression guard for the real, reproduced incident this check exists for:
// this engine's own Redis copy of a clock (AlertedTier still 0) disagreeing
// with entity-service's own durable record (the clock has actually already
// completed, e.g. a case closed or a qualifying comment landed, but the
// Redis-side AdvanceAlertedTier write that should have recorded that was
// lost) must not produce a false breach alert. The Kafka sla.tier_reached
// publish still happens regardless (it has no consumer today, and the
// wall-clock threshold genuinely was crossed) -- only the Chat send itself,
// and the bookkeeping that follows it, are what entity-service's answer
// changes.
func TestTick_EntityServiceReportsClockAlreadyResolved_SuppressesTheAlert(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)
	past := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {Found: true, IsActive: false, Stage: "ACHIEVED", HasBreached: false, StartedOn: &past},
	}}

	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", Team: "Team Nova", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}

	if len(chat.calls) != 0 {
		t.Errorf("chat calls = %+v, want none -- entity-service reports this clock already resolved with no breach", chat.calls)
	}
	if pub.calls != 1 {
		t.Errorf("publish calls = %d, want 1 -- the sla.tier_reached event still publishes even when the chat send is suppressed", pub.calls)
	}
	if _, stillWaiting := st.wake[wakeMember("case-1", "response", 100)]; stillWaiting {
		t.Error("wake entry not removed even though the alert was suppressed")
	}
	meta, _, _ := st.GetClock(context.Background(), "case-1", "response")
	if meta.AlertedTier != 100 {
		t.Errorf("AlertedTier = %d, want 100 -- a confirmed-resolved clock must still be marked done so a later tier for the same clock isn't re-checked and re-suppressed from scratch", meta.AlertedTier)
	}
}

// TestTick_EntityServiceConfirmsClockStillActive_StillSendsTheAlert proves
// the new check is not a blanket suppression -- a clock entity-service
// still considers live (the overwhelmingly common case: a real, ongoing
// SLA warning or breach) must alert exactly as before this check existed.
func TestTick_EntityServiceConfirmsClockStillActive_StillSendsTheAlert(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)
	past := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {Found: true, IsActive: true, Stage: "IN_PROGRESS", StartedOn: &past},
	}}

	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) == 0 {
		t.Error("chat calls = none, want at least one -- entity-service confirms this clock is still active")
	}
}

// TestTick_BreachEmail_SendsToAssigneeAndTeam is the regression test for the
// restored SLA breach email reaction: once GetClockState resolves an
// assignee and a team email for this clock (the same call sendBreachAlert
// already makes for its own stale-alert check), sendBreachEmails sends one
// email to each, on top of the Chat card.
func TestTick_BreachEmail_SendsToAssigneeAndTeam(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, pub)
	e.email = email
	past := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {
			Found: true, IsActive: true, Stage: "IN_PROGRESS", StartedOn: &past,
			AssigneeName: "Jane Doe", AssigneeEmail: "jane@example.com",
			TeamEmail: "team-nova@example.com", TeamLeadName: "Lead Person",
		},
	}}

	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", Team: "Team Nova", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(email.calls) != 2 {
		t.Fatalf("email calls = %+v, want exactly 2 (assignee + team)", email.calls)
	}
	if got := email.calls[0].to; len(got) != 1 || got[0] != "jane@example.com" {
		t.Errorf("first email to = %v, want [jane@example.com]", got)
	}
	if got := email.calls[1].to; len(got) != 1 || got[0] != "team-nova@example.com" {
		t.Errorf("second email to = %v, want [team-nova@example.com]", got)
	}
}

// TestTick_BreachEmail_UnresolvedRecipientsAreSkipped confirms a case with
// no assignee and a team with no configured group_email sends no email at
// all -- a normal state, not an error.
func TestTick_BreachEmail_UnresolvedRecipientsAreSkipped(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, pub)
	e.email = email
	past := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {Found: true, IsActive: true, Stage: "IN_PROGRESS", StartedOn: &past},
	}}

	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(email.calls) != 0 {
		t.Errorf("email calls = %+v, want none -- neither recipient resolved", email.calls)
	}
}

// TestTick_BreachEmail_NoEmailClientConfigured_IsANoOp confirms a
// deployment that never wired an email client into the engine (e.email ==
// nil, the zero value every other existing test's own Engine literal
// already leaves it at) sends no email and does not panic, even when
// entity-service resolves a real assignee/team.
func TestTick_BreachEmail_NoEmailClientConfigured_IsANoOp(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)
	past := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {Found: true, IsActive: true, Stage: "IN_PROGRESS", StartedOn: &past, AssigneeEmail: "jane@example.com"},
	}}

	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) == 0 {
		t.Error("chat calls = none, want at least one -- the Chat side must be unaffected by no email client being configured")
	}
}

// TestTick_BreachEmail_DebugModeRedirectsToDebugRecipients confirms
// EMAIL_DEBUG_MODE redirects both the assignee and the team email to
// e.emailDebugRecipients instead of the real resolved addresses, with the
// body naming the real intended recipient -- same posture as
// dispatch.Dispatcher's own EMAIL_DEBUG_MODE handling for case.* emails.
func TestTick_BreachEmail_DebugModeRedirectsToDebugRecipients(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, pub)
	e.email = email
	e.emailDebugMode = true
	e.emailDebugRecipients = []string{"debug@example.com"}
	past := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {
			Found: true, IsActive: true, Stage: "IN_PROGRESS", StartedOn: &past,
			AssigneeEmail: "jane@example.com", TeamEmail: "team-nova@example.com",
		},
	}}

	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(email.calls) != 2 {
		t.Fatalf("email calls = %+v, want exactly 2", email.calls)
	}
	for i, c := range email.calls {
		if len(c.to) != 1 || c.to[0] != "debug@example.com" {
			t.Errorf("email.calls[%d].to = %v, want [debug@example.com]", i, c.to)
		}
	}
	if !strings.Contains(email.calls[0].htmlBody, "jane@example.com") {
		t.Error("debug-redirected assignee email body does not name the real intended recipient")
	}
}

// TestTick_BreachEmail_TeamResolvedButNoGroupEmail_DebugModeShowsTeamName
// confirms the one exception to "unresolved recipient is skipped": in debug
// mode, a team that resolved (meta.Team non-empty) but has no group_email
// configured still sends to the debug recipients, naming the team rather
// than being silently skipped -- so team routing itself can be verified in
// a test environment with incomplete team data.
func TestTick_BreachEmail_TeamResolvedButNoGroupEmail_DebugModeShowsTeamName(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, pub)
	e.email = email
	e.emailDebugMode = true
	e.emailDebugRecipients = []string{"debug@example.com"}
	past := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {Found: true, IsActive: true, Stage: "IN_PROGRESS", StartedOn: &past},
	}}

	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", Team: "Team Nova", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(email.calls) != 1 {
		t.Fatalf("email calls = %+v, want exactly 1 (team only -- no assignee resolved at all)", email.calls)
	}
	if !strings.Contains(email.calls[0].htmlBody, "Team Nova") {
		t.Error("debug-mode team email body does not name the unresolved team")
	}
}

// TestTick_BreachEmail_SLAEmailSendingDisabled_SendsNoEmailEither confirms
// SLA_EMAIL_SENDING_ENABLED=false (slaEmailSendingEnabled) suppresses the
// breach email exactly as it already suppresses the Chat card -- the same
// single killswitch covers both, per that field's own doc comment.
func TestTick_BreachEmail_SLAEmailSendingDisabled_SendsNoEmailEither(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, pub)
	e.email = email
	e.slaEmailSendingEnabled = false
	past := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {Found: true, IsActive: true, Stage: "IN_PROGRESS", StartedOn: &past, AssigneeEmail: "jane@example.com"},
	}}

	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) != 0 || len(email.calls) != 0 {
		t.Errorf("chat calls = %d, email calls = %d, want 0/0 -- SLA_EMAIL_SENDING_ENABLED=false suppresses both", len(chat.calls), len(email.calls))
	}
}

// TestTick_EntityServiceReportsResolvedButBreached_StillSendsTheAlert proves
// the suppression is specific to a clean, no-breach resolution -- a clock
// that genuinely DID breach before it was later completed (e.g. an
// engineer's reply came in long after the deadline) is a real violation
// worth reporting, not the false-alarm case this feature exists to catch.
func TestTick_EntityServiceReportsResolvedButBreached_StillSendsTheAlert(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)
	past := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {Found: true, IsActive: false, Stage: "ACHIEVED", HasBreached: true, StartedOn: &past},
	}}

	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) == 0 {
		t.Error("chat calls = none, want at least one -- a resolved clock that genuinely breached is a real violation, not a false alarm")
	}
}

// TestTick_EntityServiceHasNoRecordOfClock_SendsAnywayFailOpen is the
// regression guard for a CodeRabbit-caught gap in an earlier version of this
// check: entity-service's own CSM clock registration is itself best-effort,
// on a different trigger than this engine's Redis registration, so "no row
// found" must never be read as "confirmed resolved" -- it's exactly as
// unconfirmed as a lookup error.
func TestTick_EntityServiceHasNoRecordOfClock_SendsAnywayFailOpen(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)
	e.entity = &fakeEntityClockStatus{} // no states configured -- every lookup reports Found: false

	past := time.Now().Add(-time.Minute)
	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) == 0 {
		t.Error("chat calls = none, want at least one -- no record in entity-service is unconfirmed, not evidence of resolution")
	}
}

// TestTick_EntityServiceReportsADifferentIncarnation_SendsAnywayFailOpen is
// the regression guard for the other CodeRabbit-caught gap: a severity
// revision cancels the old clock and registers a fresh one for the same
// (case, clockType) pair. entity-service reporting that NEW, currently-active
// incarnation must not be read as confirming the OLD, Redis-tracked
// incarnation (the one this wake entry is actually about) is resolved.
func TestTick_EntityServiceReportsADifferentIncarnation_SendsAnywayFailOpen(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)

	redisStartedAt := time.Now().Add(-4 * time.Hour)
	// entity-service's own row started far more recently than Redis's own
	// copy of this clock -- a different incarnation, well outside
	// clockIncarnationTolerance, even though its stage happens to be
	// terminal/non-breached (what would otherwise look like a clean
	// suppression candidate).
	entityStartedOn := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {Found: true, IsActive: false, Stage: "ACHIEVED", HasBreached: false, StartedOn: &entityStartedOn},
	}}

	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: redisStartedAt}
	st.wake[wakeMember("case-1", "response", 50)] = redisStartedAt

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) == 0 {
		t.Error("chat calls = none, want at least one -- a mismatched incarnation must not confirm this wake entry's own clock is resolved")
	}
}

// TestTick_EntityServiceCheckFails_SendsAnywayFailOpen proves a transient
// failure of the new verification call (entity-service briefly unreachable,
// a network error) does not suppress a real alert -- silently losing a
// genuine breach notification over a failed double-check would be worse
// than the occasional false positive this whole feature exists to reduce.
func TestTick_EntityServiceCheckFails_SendsAnywayFailOpen(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	pub := &fakePublisher{}
	e := newTestEngine(st, chat, pub)
	e.entity = &fakeEntityClockStatus{err: errors.New("entity-service unreachable")}

	past := time.Now().Add(-time.Minute)
	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) == 0 {
		t.Error("chat calls = none, want at least one -- a failed verification must fail open, not silently drop a real alert")
	}
}

// TestTick_PausedClock_LeavesWakeEntryForResumeToReschedule pins the fixed
// behavior: a tier due while paused is no longer dropped (see
// processDueMember's own doc comment) — it stays in the wake index,
// unaltered and unalerted, until Engine.resumeShift pushes it forward once
// the clock actually resumes. This replaces the old
// TestTick_PausedClock_DropsWakeEntryWithoutAlerting, which pinned the
// now-fixed "known gap" (dropped outright, permanently losing the tier).
func TestTick_PausedClock_LeavesWakeEntryForResumeToReschedule(t *testing.T) {
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
	at, stillWaiting := st.wake[wakeMember("case-1", "workaround", 50)]
	if !stillWaiting {
		t.Fatal("wake entry for a paused clock was removed; it must survive for resumeShift to reschedule")
	}
	if !at.Equal(past) {
		t.Errorf("wake entry's due time changed while still paused: got %v, want unchanged %v", at, past)
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
	chat := &fakeChat{failFor: map[string]bool{"SLA Default": true}}
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
	clocks        []activeSLAClock
	err           error
	clockStateErr error
	// clockStateStage, when set, is reported as every matched clock's own
	// Stage in GetClockState -- lets a test simulate a clock that reached a
	// terminal stage (ACHIEVED/CANCELLED/COMPLETED) sometime after it was
	// still returned by GetActiveCSMSLAClocks/listed in f.clocks.
	clockStateStage string
	// clocksOnSecondCall, when non-nil, is returned by the SECOND
	// GetActiveCSMSLAClocks call instead of clocks -- lets a test simulate
	// entity-service's own data changing between RecalculateFromEntity's
	// main pass and its settle pass (e.g. a clock registered by a genuine,
	// concurrent case update that landed too late for the main pass to see).
	clocksOnSecondCall []activeSLAClock
	getActiveCallCount int
	// extraClockStates, keyed by "caseID|clockType", answers GetClockState
	// for a clock that is NOT in f.clocks at all -- lets a test simulate
	// entity-service's own single-clock lookup confirming a clock is still
	// live even though OFFSET-based pagination skipped it out of the bulk
	// GetActiveCSMSLAClocks list (checked before falling back to the
	// f.clocks-derived behavior below).
	extraClockStates map[string]clockState
}

func (f *fakeActiveClocksClient) GetActiveCSMSLAClocks(context.Context) ([]activeSLAClock, error) {
	f.getActiveCallCount++
	if f.getActiveCallCount == 2 && f.clocksOnSecondCall != nil {
		return f.clocksOnSecondCall, f.err
	}
	return f.clocks, f.err
}

// GetClockState lets fakeActiveClocksClient also serve as Engine.entity
// (declared as entityClockStatusClient) in RecalculateFromEntity's/
// ReviseClocks' own tests, which need a value satisfying both interfaces
// the way the real *entity.CustomerEntityClient does. Derives its answer
// from f.clocks (matched by caseID/clockType) rather than a separate,
// parallel fixture: refreshAndReconcileClock re-fetches this per clock
// before rebuilding it (the repair pass's/ReviseClocks' own "ask
// entity-service again, don't trust a stale snapshot" behavior), so a test
// that sets up f.clocks for one case/type also gets a matching
// GetClockState answer for free.
func (f *fakeActiveClocksClient) GetClockState(_ context.Context, caseID, clockType string) (clockState, error) {
	if f.clockStateErr != nil {
		return clockState{}, f.clockStateErr
	}
	if f.extraClockStates != nil {
		if state, ok := f.extraClockStates[caseID+"|"+clockType]; ok {
			return state, nil
		}
	}
	for _, c := range f.clocks {
		if c.CaseID == caseID && c.ClockType == clockType {
			return clockState{
				Found:                true,
				Stage:                f.clockStateStage,
				StartedOn:            c.StartedOn,
				PausedOn:             c.PausedOn,
				PauseDurationSeconds: c.PauseDurationSeconds,
				IsPaused:             c.IsPaused,
				DurationSeconds:      c.DurationSeconds,
				Schedule:             c.Schedule,
			}, nil
		}
	}
	return clockState{Found: false}, nil
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
		DurationSeconds: int64((15 * time.Minute).Seconds()),
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
		StartedOn: timePtr(startedAt), DurationSeconds: int64((15 * time.Minute).Seconds()),
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
		StartedOn: timePtr(startedAt), DurationSeconds: int64((15 * time.Minute).Seconds()),
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
		StartedOn: timePtr(time.Now()), IsPaused: true, DurationSeconds: int64((4 * time.Hour).Seconds()),
	}, time.Now())

	meta, found, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround)
	if !found || !meta.Paused {
		t.Errorf("meta = %+v (found=%v), want Paused=true", meta, found)
	}
}

// TestReconcileClock_PauseDurationShiftsDueTimes confirms a clock's
// accumulated pause time (entity-service's own pause_duration, carried on
// activeSLAClock.PauseDurationSeconds) is folded into every tier's due time
// during reconciliation — a clock that spent an hour paused should have its
// still-future tiers pushed an hour later than a flat start+duration
// calculation would give, and the shifted-pause baseline recorded so a
// later real resume diffs against the right starting point.
func TestReconcileClock_PauseDurationShiftsDueTimes(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})

	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	// CATASTROPHIC response = 15m, started 5m ago -- with no pause at all,
	// every tier would still be in the future (50% due at +7.5m).
	startedAt := now.Add(-5 * time.Minute)
	pausedFor := int64(3600) // one hour, already resumed (not IsPaused)

	e.reconcileClock(context.Background(), activeSLAClock{
		CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC",
		StartedOn: timePtr(startedAt), PauseDurationSeconds: pausedFor,
		DurationSeconds: int64((15 * time.Minute).Seconds()),
	}, now)

	wantAt := startedAt.Add(7*time.Minute + 30*time.Second).Add(time.Hour)
	gotAt, ok := st.wake[wakeMember("case-1", ClockResponse, 50)]
	if !ok {
		t.Fatal("no wake entry for the 50% tier")
	}
	if !gotAt.Equal(wantAt) {
		t.Errorf("50%% tier due at %v, want %v (startedAt + 7.5m + 1h pause)", gotAt, wantAt)
	}

	meta, found, _ := st.GetClock(context.Background(), "case-1", ClockResponse)
	if !found {
		t.Fatal("clock not registered")
	}
	if meta.ShiftedPauseSeconds != pausedFor {
		t.Errorf("ShiftedPauseSeconds = %d, want %d (the baseline a later resume diffs against)", meta.ShiftedPauseSeconds, pausedFor)
	}
}

// TestReconcileClock_OpenPauseIsFoldedIntoDueTimesAndTheBaseline covers a
// clock that is STILL paused at the moment reconciliation runs: the
// already-open segment (now - PausedOn) must push the due times out (so a
// tick right after an eventual resume doesn't immediately fire a tier whose
// due time never accounted for the ongoing pause) AND must be folded into
// ShiftedPauseSeconds too (a CodeRabbit-caught double-count bug: an earlier
// version excluded the open segment from the baseline on the reasoning that
// it "hasn't completed yet," but it's already baked into the due times
// above regardless of whether it has completed — excluding it from the
// baseline just means the real resume's diff re-applies the same segment a
// second time, delaying every tier by its length).
func TestReconcileClock_OpenPauseIsFoldedIntoDueTimesAndTheBaseline(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})

	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-5 * time.Minute)
	pausedOn := now.Add(-30 * time.Minute) // the clock has been paused for the last 30m
	completedPause := int64(3600)          // plus an hour from earlier, already-resumed segments

	e.reconcileClock(context.Background(), activeSLAClock{
		CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC",
		StartedOn: timePtr(startedAt), IsPaused: true, PausedOn: timePtr(pausedOn),
		PauseDurationSeconds: completedPause, DurationSeconds: int64((15 * time.Minute).Seconds()),
	}, now)

	wantAt := startedAt.Add(7*time.Minute + 30*time.Second).Add(time.Hour).Add(30 * time.Minute)
	gotAt, ok := st.wake[wakeMember("case-1", ClockResponse, 50)]
	if !ok {
		t.Fatal("no wake entry for the 50% tier")
	}
	if !gotAt.Equal(wantAt) {
		t.Errorf("50%% tier due at %v, want %v (startedAt + 7.5m + 1h completed + 30m open pause)", gotAt, wantAt)
	}

	wantBaseline := completedPause + 1800 // the completed total plus the 30m open segment
	meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResponse)
	if meta.ShiftedPauseSeconds != wantBaseline {
		t.Errorf("ShiftedPauseSeconds = %d, want %d (completed total + the still-open segment, matching what was baked into the due times above)", meta.ShiftedPauseSeconds, wantBaseline)
	}
	if !meta.Paused {
		t.Error("Paused = false, want true")
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
		{CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds())},
		{CaseID: "case-2", ClockType: ClockWorkaround, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), IsPaused: true, DurationSeconds: int64((4 * time.Hour).Seconds())},
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

// TestRecalculationProgressLogInterval confirms the interval scales with
// the total clock count instead of staying flat -- a flat "every 50" would
// be one log line per clock for a small staging set and tens of thousands
// of lines for a large production one (checked live at over 100,000
// clocks).
func TestRecalculationProgressLogInterval(t *testing.T) {
	tests := []struct {
		total int
		want  int
	}{
		{0, slaRecalculationProgressLogMinInterval},
		{1, slaRecalculationProgressLogMinInterval},
		{500, slaRecalculationProgressLogMinInterval}, // 500/20 = 25, below the floor
		{1000, 50},     // 1000/20 = 50, right at the floor
		{128409, 6420}, // 128409/20 = 6420 (integer division)
	}
	for _, tt := range tests {
		if got := recalculationProgressLogInterval(tt.total); got != tt.want {
			t.Errorf("recalculationProgressLogInterval(%d) = %d, want %d", tt.total, got, tt.want)
		}
	}
}

// TestEngine_RecalculateFromEntity_CleanupRemovesStaleClocksNoLongerActive
// confirms the admin recalculation action both rebuilds every clock the
// client's current list reports AND removes, via its own cleanup diff (not
// a blanket wipe -- see RecalculateFromEntity's own doc comment for why),
// any clock Redis still held for a case the client no longer reports at all
// (case-stale here).
func TestEngine_RecalculateFromEntity_CleanupRemovesStaleClocksNoLongerActive(t *testing.T) {
	st := newFakeStore()
	// Pre-existing Redis state for a clock the client will NOT report as
	// active any more -- simulates the exact scenario this action exists
	// for (entity-service's own data changed out from under Redis).
	if err := st.SetClock(context.Background(), "case-stale", ClockResponse, ClockMeta{CaseNumber: "CS0000"}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}

	client := &fakeActiveClocksClient{clocks: []activeSLAClock{
		{CaseID: "case-fresh", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds())},
	}}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	if err := e.RecalculateFromEntity(context.Background()); err != nil {
		t.Fatalf("RecalculateFromEntity() error = %v", err)
	}

	if _, found, _ := st.GetClock(context.Background(), "case-stale", ClockResponse); found {
		t.Error("case-stale's clock hash survived RecalculateFromEntity's own cleanup pass, want it removed")
	}
	if _, found, _ := st.GetClock(context.Background(), "case-fresh", ClockResponse); !found {
		t.Error("case-fresh's clock was not rebuilt by RecalculateFromEntity")
	}
}

// TestEngine_RecalculateFromEntity_NoEntityClientConfigured_ReturnsError
// confirms a misconfigured Engine (entity unset, or set to something that
// doesn't also satisfy entityActiveClocksClient) fails loudly rather than
// silently doing nothing.
func TestEngine_RecalculateFromEntity_NoEntityClientConfigured_ReturnsError(t *testing.T) {
	e := newTestEngine(newFakeStore(), &fakeChat{}, &fakePublisher{})
	if err := e.RecalculateFromEntity(context.Background()); err == nil {
		t.Fatal("RecalculateFromEntity() error = nil, want an error for a nil entity client")
	}
}

// TestEngine_RecalculateFromEntity_FetchFailure_LeavesRedisUntouched
// confirms the main pass's own fetch failure stops RecalculateFromEntity
// before anything is written or removed -- a case this matters for
// specifically because the old design used to wipe Redis before fetching at
// all (a CodeRabbit-caught ordering bug), leaving the engine with nothing to
// rebuild from on a fetch failure; the current design never wipes anything
// up front, but a fetch failure must still leave existing clocks exactly as
// they were, not attempt a cleanup pass against an empty/partial fresh set.
func TestEngine_RecalculateFromEntity_FetchFailure_LeavesRedisUntouched(t *testing.T) {
	st := newFakeStore()
	if err := st.SetClock(context.Background(), "case-untouched", ClockResponse, ClockMeta{CaseNumber: "CS0000"}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = &fakeActiveClocksClient{err: errors.New("entity-service unreachable")}

	if err := e.RecalculateFromEntity(context.Background()); err == nil {
		t.Fatal("RecalculateFromEntity() error = nil, want the fetch failure propagated")
	}
	if _, found, _ := st.GetClock(context.Background(), "case-untouched", ClockResponse); !found {
		t.Error("RecalculateFromEntity touched Redis despite the active-clock fetch failing")
	}
}

// TestEngine_RecalculateFromEntity_SettlePassCatchesClockMissedByMainPass is
// the regression test for the whole reason the settle (second) pass exists:
// a clock registered by a genuine, concurrent case update that lands too
// late for the main pass's own snapshot to include it must still be picked
// up by this same recalculation, not left with no Redis entry at all until
// the next one.
func TestEngine_RecalculateFromEntity_SettlePassCatchesClockMissedByMainPass(t *testing.T) {
	st := newFakeStore()
	client := &fakeActiveClocksClient{
		clocks: []activeSLAClock{
			{CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds())},
		},
		// case-2 only shows up on the settle pass's own re-fetch -- as if it
		// were registered live, by a real case update, sometime during the
		// main pass.
		clocksOnSecondCall: []activeSLAClock{
			{CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds())},
			{CaseID: "case-2", ClockType: ClockResponse, Priority: "HIGH", StartedOn: timePtr(time.Now()), DurationSeconds: int64((time.Hour).Seconds())},
		},
	}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	if err := e.RecalculateFromEntity(context.Background()); err != nil {
		t.Fatalf("RecalculateFromEntity() error = %v", err)
	}
	if _, found, _ := st.GetClock(context.Background(), "case-2", ClockResponse); !found {
		t.Error("case-2's clock, only visible on the settle pass's re-fetch, was never rebuilt")
	}
}

// TestEngine_RecalculateFromEntity_CleanupListFailureIsNonFatal confirms a
// failure listing Redis's own existing clock keys (the cleanup step's first
// call) skips cleanup for this run but never fails the recalculation itself
// -- every clock the settle pass reconciled is already correct regardless.
func TestEngine_RecalculateFromEntity_CleanupListFailureIsNonFatal(t *testing.T) {
	st := newFakeStore()
	st.failListClockKeys = true
	client := &fakeActiveClocksClient{clocks: []activeSLAClock{
		{CaseID: "case-fresh", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds())},
	}}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	if err := e.RecalculateFromEntity(context.Background()); err != nil {
		t.Fatalf("RecalculateFromEntity() error = %v, want nil (a cleanup listing failure is logged, not fatal)", err)
	}
	if _, found, _ := st.GetClock(context.Background(), "case-fresh", ClockResponse); !found {
		t.Error("case-fresh's clock was not rebuilt despite the cleanup listing failure")
	}
}

// TestEngine_RecalculateFromEntity_CleanupDeleteFailureIsNonFatal confirms
// one stale clock's delete failing during cleanup is logged and skipped,
// not allowed to fail the whole recalculation -- the clocks the settle pass
// actually rebuilt are unaffected either way.
func TestEngine_RecalculateFromEntity_CleanupDeleteFailureIsNonFatal(t *testing.T) {
	st := newFakeStore()
	if err := st.SetClock(context.Background(), "case-stale", ClockResponse, ClockMeta{CaseNumber: "CS0000"}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	st.failDeleteClock = true
	client := &fakeActiveClocksClient{clocks: []activeSLAClock{
		{CaseID: "case-fresh", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds())},
	}}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	if err := e.RecalculateFromEntity(context.Background()); err != nil {
		t.Fatalf("RecalculateFromEntity() error = %v, want nil (a cleanup delete failure is logged, not fatal)", err)
	}
	if _, found, _ := st.GetClock(context.Background(), "case-fresh", ClockResponse); !found {
		t.Error("case-fresh's clock was not rebuilt despite the cleanup delete failure for an unrelated stale clock")
	}
}

// TestEngine_RecalculateFromEntity_CleanupNeverDeletesAClockRegisteredAfterTheSnapshot
// is the regression test for a CodeRabbit-caught gap: an earlier version
// took the cleanup candidate snapshot (Store.ListClockKeys) AFTER the settle
// pass, so a clock registered live (RegisterClocks, on case.created,
// independent of this recalculation's own guard) in the window between that
// snapshot and the settle pass's own fetch -- not yet visible to
// entity-service's bulk list either -- would be in Redis but missing from
// fresh, and cleanup would delete a clock that was never stale at all. The
// snapshot is now taken BEFORE the main pass even starts, so a clock that
// didn't exist in Redis yet at that moment is never a cleanup candidate in
// the first place, no matter what happens afterward.
func TestEngine_RecalculateFromEntity_CleanupNeverDeletesAClockRegisteredAfterTheSnapshot(t *testing.T) {
	st := newFakeStore()
	client := &fakeActiveClocksClient{} // entity-service reports nothing active at all, either pass
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	// Simulates RegisterClocks firing from a live case.created event right
	// after RecalculateFromEntity's own pre-pass ListClockKeys snapshot --
	// a clock entity-service's own bulk list doesn't report either, exactly
	// the race this fix protects against.
	st.afterListClockKeys = func() {
		if err := st.SetClock(context.Background(), "case-new", ClockResponse, ClockMeta{CaseNumber: "CS0099"}); err != nil {
			t.Fatalf("SetClock() error = %v", err)
		}
	}

	if err := e.RecalculateFromEntity(context.Background()); err != nil {
		t.Fatalf("RecalculateFromEntity() error = %v", err)
	}
	if _, found, _ := st.GetClock(context.Background(), "case-new", ClockResponse); !found {
		t.Error("case-new's clock was deleted despite being registered after the pre-recalculation snapshot was taken")
	}
}

// TestEngine_RecalculateFromEntity_CleanupSkipsACandidateEntityServiceStillConfirmsActive
// is the regression test for a second CodeRabbit-caught gap in the same
// cleanup step: OFFSET-based pagination (GetActiveCSMSLAClocks) can skip a
// row if a clock closes between two pages, independently of anything this
// engine does -- the skipped clock is still genuinely active, but absent
// from both passes' own fetched lists through no fault of its own.
// cleanupStaleClocks must double-check a candidate directly against
// entity-service's own single-clock lookup before deleting it, and skip the
// delete when that lookup still reports it Found and not in a terminal
// stage.
func TestEngine_RecalculateFromEntity_CleanupSkipsACandidateEntityServiceStillConfirmsActive(t *testing.T) {
	st := newFakeStore()
	if err := st.SetClock(context.Background(), "case-skipped", ClockResponse, ClockMeta{CaseNumber: "CS0001"}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	client := &fakeActiveClocksClient{
		// Neither pass's own bulk list reports case-skipped at all -- as if
		// OFFSET pagination skipped it -- but entity-service's own
		// single-clock lookup still confirms it is genuinely active.
		extraClockStates: map[string]clockState{
			"case-skipped|" + ClockResponse: {Found: true, Stage: "IN_PROGRESS"},
		},
	}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	if err := e.RecalculateFromEntity(context.Background()); err != nil {
		t.Fatalf("RecalculateFromEntity() error = %v", err)
	}
	if _, found, _ := st.GetClock(context.Background(), "case-skipped", ClockResponse); !found {
		t.Error("case-skipped's clock was deleted despite entity-service's own lookup still confirming it active")
	}
}

// TestEngine_RecalculateFromEntity_FailedClockRepairsOnFreshContext is the
// regression test for a CodeRabbit-caught gap: a clock that fails to
// rebuild on the main pass (for any reason, context exhaustion included --
// see RecalculateFromEntity's own doc comment) must get a real second
// chance on a FRESH context, not be left half-rebuilt (a clock hash with no
// wake entry, in this case) until the next recalculation or a restart.
func TestEngine_RecalculateFromEntity_FailedClockRepairsOnFreshContext(t *testing.T) {
	st := newFakeStore()
	st.failAddAttempts = 1 // the only AddWake call in the main pass fails; the repair pass's own succeeds
	client := &fakeActiveClocksClient{clocks: []activeSLAClock{
		{CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds())},
	}}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	if err := e.RecalculateFromEntity(context.Background()); err != nil {
		t.Fatalf("RecalculateFromEntity() error = %v", err)
	}
	meta, found, err := st.GetClock(context.Background(), "case-1", ClockResponse)
	if err != nil {
		t.Fatalf("GetClock() error = %v", err)
	}
	if !found {
		t.Fatal("case-1's clock hash was not written at all")
	}
	if _, wakeFound := st.wake[wakeMember("case-1", ClockResponse, 50)]; !wakeFound {
		t.Errorf("case-1's wake entry is still missing after the repair pass; clock = %+v", meta)
	}
}

// TestEngine_RecalculateFromEntity_SkipsWhenAlreadyRunning confirms the
// recalculating guard rejects a second call while one is already marked
// in-flight, rather than racing a second wipe against it.
func TestEngine_RecalculateFromEntity_SkipsWhenAlreadyRunning(t *testing.T) {
	st := newFakeStore()
	if err := st.SetClock(context.Background(), "case-untouched", ClockResponse, ClockMeta{CaseNumber: "CS0000"}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = &fakeActiveClocksClient{}
	e.recalculating.Store(true)

	if err := e.RecalculateFromEntity(context.Background()); err != nil {
		t.Fatalf("RecalculateFromEntity() error = %v, want nil (skip, not fail)", err)
	}
	if _, found, _ := st.GetClock(context.Background(), "case-untouched", ClockResponse); !found {
		t.Error("RecalculateFromEntity wiped Redis despite recalculating already being true")
	}
}

// TestRefreshAndReconcileClock_UsesFreshStateNotTheGivenDisplaySnapshot is
// the regression test for an offline-review-caught gap in
// RecalculateFromEntity's own repair pass: between the main pass's one-time
// fetch and the repair pass running (up to slaRecalculationTimeout later),
// a live case update can genuinely pause or resume the same clock.
// Reapplying the stale snapshot's own IsPaused as-is would silently
// clobber that newer, real state. refreshAndReconcileClock must always ask
// entity-service again rather than trust whatever pause state its own
// display argument happens to carry.
func TestRefreshAndReconcileClock_UsesFreshStateNotTheGivenDisplaySnapshot(t *testing.T) {
	st := newFakeStore()
	client := &fakeActiveClocksClient{clocks: []activeSLAClock{
		// entity-service's own CURRENT, real answer: resumed.
		{CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds()), IsPaused: false},
	}}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	// A stale snapshot claiming the clock is still paused -- exactly what
	// the repair pass's own "failed" slice would hold if this clock paused,
	// was fetched, and then resumed again before the repair pass ran.
	stale := activeSLAClock{CaseID: "case-1", ClockType: ClockResponse, IsPaused: true}
	found, ok := e.refreshAndReconcileClock(context.Background(), stale, time.Now())
	if !found || !ok {
		t.Fatalf("refreshAndReconcileClock() = (%v, %v), want (true, true)", found, ok)
	}
	meta, clockFound, err := st.GetClock(context.Background(), "case-1", ClockResponse)
	if err != nil {
		t.Fatalf("GetClock() error = %v", err)
	}
	if !clockFound {
		t.Fatal("clock was not written")
	}
	if meta.Paused {
		t.Error("refreshAndReconcileClock wrote Paused=true, want the fresh (resumed) value -- it trusted the stale display snapshot instead of re-fetching")
	}
}

// TestRefreshAndReconcileClock_DoesNotRebuildATerminalClock is the
// regression test for a CodeRabbit-caught gap: GetClockState returns a row
// "regardless of whether it's currently active" (unlike
// GetActiveCSMSLAClocks), so Found alone does not mean "still live". A
// clock already ACHIEVED/CANCELLED/COMPLETED must not be written back into
// Redis as a live one with fresh future wake entries.
func TestRefreshAndReconcileClock_DoesNotRebuildATerminalClock(t *testing.T) {
	st := newFakeStore()
	client := &fakeActiveClocksClient{
		clocks: []activeSLAClock{
			{CaseID: "case-1", ClockType: ClockResponse, StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds())},
		},
		clockStateStage: "COMPLETED",
	}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	found, ok := e.refreshAndReconcileClock(context.Background(), activeSLAClock{CaseID: "case-1", ClockType: ClockResponse}, time.Now())
	if found || !ok {
		t.Fatalf("refreshAndReconcileClock() = (%v, %v), want (false, true) -- a terminal clock is \"nothing to rebuild\", not an error", found, ok)
	}
	if _, clockFound, _ := st.GetClock(context.Background(), "case-1", ClockResponse); clockFound {
		t.Error("a COMPLETED clock was written back into Redis as if it were still live")
	}
}

// TestEngine_ReviseClocks_RemovesStaleWakeEntriesForATerminalClock confirms
// ReviseClocks' own stale-cleanup path (normally for a clock type the new
// severity has no policy for at all) also fires for a clock that reached a
// terminal stage on its own -- the same CodeRabbit-caught gap as the test
// above, exercised through ReviseClocks specifically.
func TestEngine_ReviseClocks_RemovesStaleWakeEntriesForATerminalClock(t *testing.T) {
	st := newFakeStore()
	if err := st.SetClock(context.Background(), "case-1", ClockResponse, ClockMeta{CaseNumber: "CS0001", StartedAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	if err := st.AddWake(context.Background(), wakeMember("case-1", ClockResponse, 50), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("AddWake() error = %v", err)
	}
	client := &fakeActiveClocksClient{
		clocks: []activeSLAClock{
			{CaseID: "case-1", ClockType: ClockResponse, StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds())},
		},
		clockStateStage: "ACHIEVED",
	}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	e.ReviseClocks(context.Background(), "case-1", "HIGH", "CS0001", "WSO2-1", "Something broke", "api-manager", "Phoenix")

	if _, stillThere := st.wake[wakeMember("case-1", ClockResponse, 50)]; stillThere {
		t.Error("stale wake entry survived ReviseClocks for a clock that had already reached a terminal stage")
	}
}

// TestEngine_RecalculateFromEntity_StillFailedAfterRepair_ReturnsError is
// the regression test for an offline-review-caught gap: a recalculation
// whose repair pass STILL leaves clocks broken must report that as an
// error, not log "recalculation complete" as if it fully succeeded.
func TestEngine_RecalculateFromEntity_StillFailedAfterRepair_ReturnsError(t *testing.T) {
	st := newFakeStore()
	st.failAdd = true // every AddWake call fails, main pass and repair pass alike
	client := &fakeActiveClocksClient{clocks: []activeSLAClock{
		{CaseID: "case-1", ClockType: ClockResponse, Priority: "CATASTROPHIC", StartedOn: timePtr(time.Now()), DurationSeconds: int64((15 * time.Minute).Seconds())},
	}}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	err := e.RecalculateFromEntity(context.Background())
	if err == nil {
		t.Fatal("RecalculateFromEntity() error = nil, want an error -- the clock is still broken after the repair pass")
	}
}

// TestEngine_ReviseClocks_RebuildsFromNewSeverity confirms a priority
// change that leaves this clock type still covered by some policy gets
// rebuilt from entity-service's own current (new-severity) state --
// closing a real, reported gap: before ReviseClocks existed, nothing here
// ever reacted to case.severity_changed at all, so Redis kept ticking down
// the OLD severity's clock, under its OLD duration, indefinitely.
func TestEngine_ReviseClocks_RebuildsFromNewSeverity(t *testing.T) {
	st := newFakeStore()
	client := &fakeActiveClocksClient{clocks: []activeSLAClock{
		{CaseID: "case-1", ClockType: ClockResponse, StartedOn: timePtr(time.Now()), DurationSeconds: int64(time.Hour.Seconds())},
	}}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	e.ReviseClocks(context.Background(), "case-1", "HIGH", "CS0001", "WSO2-1", "Something broke", "api-manager", "Phoenix")

	meta, found, err := st.GetClock(context.Background(), "case-1", ClockResponse)
	if err != nil {
		t.Fatalf("GetClock() error = %v", err)
	}
	if !found {
		t.Fatal("case-1's response clock was not rebuilt")
	}
	if meta.Priority != "HIGH" || meta.CaseNumber != "CS0001" {
		t.Errorf("meta = %+v, want the new severity and the event's own display fields", meta)
	}
}

// TestEngine_ReviseClocks_RemovesStaleWakeEntriesForAClockTypeNoLongerCovered
// confirms a clock type the NEW severity's policy set has no policy for at
// all (unlike RecalculateFromEntity's own repair pass, there is no
// preceding wipe here to already clear this) gets its stale wake entries
// actively removed, not left ticking down toward an alert for a clock that
// no longer exists under any policy.
func TestEngine_ReviseClocks_RemovesStaleWakeEntriesForAClockTypeNoLongerCovered(t *testing.T) {
	st := newFakeStore()
	startedAt := time.Now().Add(-time.Hour)
	if err := st.SetClock(context.Background(), "case-1", ClockWorkaround, ClockMeta{CaseNumber: "CS0001", StartedAt: startedAt}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	if err := st.AddWake(context.Background(), wakeMember("case-1", ClockWorkaround, 50), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("AddWake() error = %v", err)
	}
	// The new severity's client has NO entry for case-1/workaround at all --
	// entity-service's own GetClockState reports not-found for it.
	client := &fakeActiveClocksClient{}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	e.ReviseClocks(context.Background(), "case-1", "LOW", "CS0001", "WSO2-1", "Something broke", "api-manager", "Phoenix")

	if _, stillThere := st.wake[wakeMember("case-1", ClockWorkaround, 50)]; stillThere {
		t.Error("stale workaround wake entry survived ReviseClocks despite the new severity having no policy for it")
	}
}

// TestEngine_ReviseClocks_PreservesDisplayFieldsFromExistingRedisMeta
// confirms CaseType/State -- carried by neither the severity-changed event
// nor GetClockState -- are read back from whatever this clock's own Redis
// hash already held from before the revision, so a rebuilt clock's Chat
// card still shows them correctly rather than going blank.
func TestEngine_ReviseClocks_PreservesDisplayFieldsFromExistingRedisMeta(t *testing.T) {
	st := newFakeStore()
	if err := st.SetClock(context.Background(), "case-1", ClockResponse, ClockMeta{CaseNumber: "CS0001", CaseType: "CASE", State: "Work In Progress", StartedAt: time.Now()}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	client := &fakeActiveClocksClient{clocks: []activeSLAClock{
		{CaseID: "case-1", ClockType: ClockResponse, StartedOn: timePtr(time.Now()), DurationSeconds: int64(time.Hour.Seconds())},
	}}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = client

	e.ReviseClocks(context.Background(), "case-1", "HIGH", "CS0001", "WSO2-1", "Something broke", "api-manager", "Phoenix")

	meta, found, err := st.GetClock(context.Background(), "case-1", ClockResponse)
	if err != nil {
		t.Fatalf("GetClock() error = %v", err)
	}
	if !found {
		t.Fatal("clock was not rebuilt")
	}
	if meta.CaseType != "CASE" || meta.State != "Work In Progress" {
		t.Errorf("meta = %+v, want CaseType/State carried forward from the pre-revision hash", meta)
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
	newWakeAt := time.Now().Add(time.Hour)      // the new clock's own, still-future tier-50 due time

	base.clocks["case-1|response"] = ClockMeta{StartedAt: oldStartedAt, Priority: "CATASTROPHIC"}
	base.wake[wakeMember("case-1", "response", 50)] = time.Now().Add(-time.Minute) // due now, under the OLD incarnation

	raceStore := &raceSimulatingStore{
		fakeStore:         base,
		caseID:            "case-1",
		clockType:         "response",
		replacement:       ClockMeta{StartedAt: newStartedAt, Priority: "HIGH"},
		replacementWakeAt: newWakeAt,
	}
	e := &Engine{store: raceStore, pub: &fakePublisher{}, chat: &fakeChat{}, links: fakeLinks{}, slaEmailSendingEnabled: true}

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

// --- isSameClockIncarnation / isTerminalSLAStage ---

func TestIsSameClockIncarnation(t *testing.T) {
	base := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name            string
		entityStartedOn *time.Time
		redisStartedAt  time.Time
		want            bool
	}{
		{"identical instant", &base, base, true},
		{"a few seconds apart (ordinary processing lag)", ptrTime(base.Add(2 * time.Second)), base, true},
		{"just inside the tolerance", ptrTime(base.Add(clockIncarnationTolerance - time.Second)), base, true},
		{"just outside the tolerance", ptrTime(base.Add(clockIncarnationTolerance + time.Second)), base, false},
		{"entity-service's own instant is EARLIER (clock skew), still close", ptrTime(base.Add(-2 * time.Second)), base, true},
		{"a genuinely new incarnation, hours later", ptrTime(base.Add(4 * time.Hour)), base, false},
		{"entity-service reports no startedOn at all", nil, base, true},
		{"redis has no startedAt at all (defensive, shouldn't happen for a real wake entry)", &base, time.Time{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isSameClockIncarnation(c.entityStartedOn, c.redisStartedAt); got != c.want {
				t.Errorf("isSameClockIncarnation(%v, %v) = %v, want %v", c.entityStartedOn, c.redisStartedAt, got, c.want)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestIsTerminalSLAStage(t *testing.T) {
	cases := map[string]bool{
		"ACHIEVED":    true,
		"CANCELLED":   true,
		"COMPLETED":   true,
		"achieved":    true, // case-insensitive
		"IN_PROGRESS": false,
		"BREACHED":    false, // deliberately not terminal -- see isTerminalSLAStage's own doc comment
		"PAUSED":      false,
		"":            false,
	}
	for stage, want := range cases {
		if got := isTerminalSLAStage(stage); got != want {
			t.Errorf("isTerminalSLAStage(%q) = %v, want %v", stage, got, want)
		}
	}
}

// --- Redis retry + degraded-fallback deferral (processDueMemberFallback) ---

// alwaysFailingGetClockStore wraps fakeStore and fails every GetClock call,
// simulating Redis being unreachable for that one operation (e.g. a
// partial/transient outage) while DueMembers itself still worked (the wake
// member was found, which is how processDueMember ever gets this far).
type alwaysFailingGetClockStore struct {
	*fakeStore
	getClockCalls int
}

func (s *alwaysFailingGetClockStore) GetClock(ctx context.Context, caseID, clockType string) (ClockMeta, bool, error) {
	s.getClockCalls++
	return ClockMeta{}, false, errors.New("redis unavailable")
}

// TestTick_RedisUnavailable_DefersRatherThanSendingADegradedAlert is the
// end-to-end regression for the simplified fallback: when Redis's GetClock
// fails even after withRedisRetry's bounded retries, Tick no longer sends a
// degraded alert from entity-service's own live state -- it logs and leaves
// the wake entry exactly where it was, so the next tick (once Redis
// recovers) handles it through the normal, idempotent processDueMember
// path. Tick itself reports no error (a Redis-unreachable GetClock is a
// handled, not a failed, outcome) and the wake entry survives untouched.
func TestTick_RedisUnavailable_DefersRatherThanSendingADegradedAlert(t *testing.T) {
	base := newFakeStore()
	wakeAt := time.Now().Add(-time.Minute)
	base.wake[wakeMember("case-1", ClockResponse, 50)] = wakeAt
	st := &alwaysFailingGetClockStore{fakeStore: base}

	chat := &fakeChat{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {Found: true, BusinessElapsedPercent: 62, Stage: "IN_PROGRESS"},
	}}

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil (a Redis-unreachable member is deferred, not failed)", err)
	}
	if st.getClockCalls != redisRetryAttempts {
		t.Errorf("GetClock calls = %d, want %d (every retry attempt)", st.getClockCalls, redisRetryAttempts)
	}
	if len(chat.calls) != 0 {
		t.Fatalf("chat calls = %+v, want none -- no alert should be sent while Redis is unreachable", chat.calls)
	}
	gotWakeAt, stillPresent := base.wake[wakeMember("case-1", ClockResponse, 50)]
	if !stillPresent {
		t.Fatal("wake entry was removed, want it left in place for the next tick to re-examine once Redis recovers")
	}
	if !gotWakeAt.Equal(wakeAt) {
		t.Errorf("wake entry due time = %v, want unchanged %v", gotWakeAt, wakeAt)
	}
}

// TestTick_RedisUnavailable_MultipleDueMembers_NoneAlertAndAllSurvive covers
// the shape that used to risk a duplicate send under the old fallback design
// (both the 50% and 75% wake members due in the same tick, e.g. after a
// prolonged outage): with no degraded send at all, both are simply left for
// the next successful tick, which is what actually claims and sends each
// tier exactly once via the normal path.
func TestTick_RedisUnavailable_MultipleDueMembers_NoneAlertAndAllSurvive(t *testing.T) {
	base := newFakeStore()
	base.wake[wakeMember("case-1", ClockResponse, 50)] = time.Now().Add(-time.Minute)
	base.wake[wakeMember("case-1", ClockResponse, 75)] = time.Now().Add(-30 * time.Second)
	st := &alwaysFailingGetClockStore{fakeStore: base}

	chat := &fakeChat{}
	e := newTestEngine(st, chat, &fakePublisher{})

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(chat.calls) != 0 {
		t.Fatalf("chat calls = %+v, want none", chat.calls)
	}
	for _, tier := range []int{50, 75} {
		if _, ok := base.wake[wakeMember("case-1", ClockResponse, tier)]; !ok {
			t.Errorf("wake entry for tier %d was removed, want it left for the next tick", tier)
		}
	}
}

// --- ApplyStateEffects pause status is wired from entity-service, not a
// --- hardcoded state list ---

// TestApplyStateEffects_PauseStatus_DrivenByEntityService_NotByStatusString
// is the direct regression for the removed hardcoded pause-state list: a
// status string that was NEVER in that old list ("Pending Customer Update")
// still pauses the clock, as long as entity-service's own record says so --
// proving pause/resume is no longer decided by matching the status string
// at all.
func TestApplyStateEffects_PauseStatus_DrivenByEntityService_NotByStatusString(t *testing.T) {
	st := newFakeStore()
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.entity = entityWithCatastrophicDurations("case-1")
	e.RegisterClocks(context.Background(), "case-1", "CATASTROPHIC", time.Now(), "CS0001", "", "", "CASE", "", "")

	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockWorkaround: {Found: true, IsPaused: true},
		"case-1|" + ClockResolution: {Found: true, IsPaused: true},
	}}
	e.ApplyStateEffects(context.Background(), "case-1", "Pending Customer Update")

	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround); !meta.Paused {
		t.Error("workaround clock not paused for a status entity-service reports as paused, even though that status string was never in the old hardcoded list")
	}
	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockResolution); !meta.Paused {
		t.Error("resolution clock not paused for a status entity-service reports as paused")
	}

	// The converse: "Awaiting Info" -- the status that USED to always mean
	// pause under the old hardcoded list -- does NOT pause when
	// entity-service's own record says this clock is not paused (e.g. a
	// policy whose pause_condition doesn't include this state at all).
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|" + ClockWorkaround: {Found: true, IsPaused: false},
		"case-1|" + ClockResolution: {Found: true, IsPaused: false},
	}}
	e.ApplyStateEffects(context.Background(), "case-1", "Awaiting Info")

	if meta, _, _ := st.GetClock(context.Background(), "case-1", ClockWorkaround); meta.Paused {
		t.Error("workaround clock paused on \"Awaiting Info\" even though entity-service's own record says it is not paused -- the old hardcoded list must no longer decide this")
	}
}

func TestEntityPauseStatus_NoEntityClient_DefaultsToNotPaused(t *testing.T) {
	e := &Engine{}
	if got := e.entityPauseStatus(context.Background(), "case-1", ClockWorkaround); got {
		t.Error("entityPauseStatus() = true with no entity client, want false (fail open toward sending, not suppressing)")
	}
}

func TestEntityPauseStatus_LookupFailure_DefaultsToNotPaused(t *testing.T) {
	e := &Engine{entity: &fakeEntityClockStatus{err: errors.New("unreachable")}}
	if got := e.entityPauseStatus(context.Background(), "case-1", ClockWorkaround); got {
		t.Error("entityPauseStatus() = true on a lookup failure, want false (fail open)")
	}
}

func TestEntityPauseStatus_NotFound_DefaultsToNotPaused(t *testing.T) {
	e := &Engine{entity: &fakeEntityClockStatus{}} // no states -- Found: false
	if got := e.entityPauseStatus(context.Background(), "case-1", ClockWorkaround); got {
		t.Error("entityPauseStatus() = true when entity-service has no record, want false")
	}
}
