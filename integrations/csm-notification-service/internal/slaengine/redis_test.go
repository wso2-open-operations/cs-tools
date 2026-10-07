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
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// The engine's own tests (engine_test.go) run against an in-memory
// fakeStore, which keeps them about Engine's own logic rather than about
// go-redis or Lua. These cover the other half: that Store's real Lua
// scripts (setClockScript in particular) actually implement the
// incarnation-aware contract fakeStore only simulates in Go — the thing
// standing between a severity-revised clock and a tier whose alert gets
// silently swallowed forever (see setClockScript's own doc comment).
//
// Skipped unless a Redis is reachable, so `go test ./...` stays
// dependency-free on a laptop and in CI. To run them:
//
//	docker run --rm -p 6379:6379 redis
//	go test ./internal/slaengine/ -run TestStore -v
//
// REDIS_URL takes precedence, then REDIS_ADDR — same order as
// internal/paging's own testStore and this service's own main.go.
func testStore(t *testing.T) (*Store, func()) {
	t.Helper()
	var rdb *redis.Client
	addr := "localhost:6379"
	if url := os.Getenv("REDIS_URL"); url != "" {
		opts, err := redis.ParseURL(url)
		if err != nil {
			t.Skip("REDIS_URL is set but does not parse; skipping")
		}
		addr = opts.Addr
		rdb = redis.NewClient(opts)
	} else {
		if a := os.Getenv("REDIS_ADDR"); a != "" {
			addr = a
		}
		rdb = redis.NewClient(&redis.Options{Addr: addr})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		t.Skipf("no Redis at %s; skipping (docker run --rm -p 6379:6379 redis)", addr)
	}
	return NewStore(rdb), func() { _ = rdb.Close() }
}

func testCaseID(t *testing.T) string {
	t.Helper()
	return "slaengine-test-" + t.Name() + "-" + time.Now().Format("150405.000000000")
}

// TestStore_SetClock_PreservesAlertedTierAcrossReplayOfSameIncarnation
// confirms the existing, pre-this-change protection is untouched: a replay
// of the identical case.created event (same startedAt) must never reset a
// clock's progress.
func TestStore_SetClock_PreservesAlertedTierAcrossReplayOfSameIncarnation(t *testing.T) {
	store, closeStore := testStore(t)
	defer closeStore()
	ctx := context.Background()
	caseID := testCaseID(t)
	defer func() { _ = store.rdb.Del(ctx, clockKey(caseID, ClockResponse)) }()

	startedAt := time.Now().Truncate(time.Second)
	meta := ClockMeta{CaseNumber: "CS0001", Priority: "CATASTROPHIC", StartedAt: startedAt}
	if err := store.SetClock(ctx, caseID, ClockResponse, meta); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	if _, err := store.AdvanceAlertedTier(ctx, caseID, ClockResponse, 100, time.Time{}); err != nil {
		t.Fatalf("AdvanceAlertedTier() error = %v", err)
	}

	// Replay: identical startedAt, as a redelivered/retried case.created
	// would send.
	if err := store.SetClock(ctx, caseID, ClockResponse, meta); err != nil {
		t.Fatalf("SetClock() (replay) error = %v", err)
	}

	got, found, err := store.GetClock(ctx, caseID, ClockResponse)
	if err != nil || !found {
		t.Fatalf("GetClock() = (%+v, %v, %v)", got, found, err)
	}
	if got.AlertedTier != 100 {
		t.Errorf("AlertedTier = %d, want 100 preserved across a same-incarnation replay", got.AlertedTier)
	}
}

// TestStore_SetClock_ResetsAlertedTierOnNewIncarnation is the direct
// regression test for the CodeRabbit-flagged bug: a genuinely different
// startedAt (a severity revision's fresh clock, as Engine.Reconcile would
// observe) must reset alertedTier to 0 — otherwise a tier already alerted
// under the OLD incarnation permanently blocks the NEW incarnation's own
// tiers from ever alerting (processDueMember's "AlertedTier >= tier" check
// has no notion of "that was a different clock").
func TestStore_SetClock_ResetsAlertedTierOnNewIncarnation(t *testing.T) {
	store, closeStore := testStore(t)
	defer closeStore()
	ctx := context.Background()
	caseID := testCaseID(t)
	defer func() { _ = store.rdb.Del(ctx, clockKey(caseID, ClockWorkaround)) }()

	oldStartedAt := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	if err := store.SetClock(ctx, caseID, ClockWorkaround, ClockMeta{Priority: "CATASTROPHIC", StartedAt: oldStartedAt}); err != nil {
		t.Fatalf("SetClock() (old incarnation) error = %v", err)
	}
	if _, err := store.AdvanceAlertedTier(ctx, caseID, ClockWorkaround, 100, time.Time{}); err != nil {
		t.Fatalf("AdvanceAlertedTier() error = %v", err)
	}

	// A severity revision: entity-service cancels the old clock and
	// registers a brand new one with a fresh start_on. Reconcile (or any
	// future ReviseCaseClocks-equivalent hook) observes this as a new
	// startedAt for the same (caseID, clockType) key.
	newStartedAt := time.Now().Truncate(time.Second)
	if err := store.SetClock(ctx, caseID, ClockWorkaround, ClockMeta{Priority: "HIGH", StartedAt: newStartedAt}); err != nil {
		t.Fatalf("SetClock() (new incarnation) error = %v", err)
	}

	got, found, err := store.GetClock(ctx, caseID, ClockWorkaround)
	if err != nil || !found {
		t.Fatalf("GetClock() = (%+v, %v, %v)", got, found, err)
	}
	if got.AlertedTier != 0 {
		t.Errorf("AlertedTier = %d, want 0 reset on a new incarnation (startedAt changed)", got.AlertedTier)
	}
	if !got.StartedAt.Equal(newStartedAt) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, newStartedAt)
	}
}

// TestStore_ClaimTier_IsScopedPerIncarnation confirms a stale, unexpired
// tier claim from an OLD incarnation does not block the NEW incarnation's
// own, independent claim for the identical (caseID, clockType, tier) —
// the matching half of the fix above; without this, ClaimTier would report
// claimed=false for the new clock's first genuine crossing even after
// alertedTier was correctly reset to 0.
func TestStore_ClaimTier_IsScopedPerIncarnation(t *testing.T) {
	store, closeStore := testStore(t)
	defer closeStore()
	ctx := context.Background()
	caseID := testCaseID(t)
	oldStartedAt := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	newStartedAt := time.Now().Truncate(time.Second)
	defer func() {
		_ = store.rdb.Del(ctx, tierClaimKey(caseID, ClockResponse, 50, oldStartedAt))
		_ = store.rdb.Del(ctx, tierClaimKey(caseID, ClockResponse, 50, newStartedAt))
	}()

	claimedOld, err := store.ClaimTier(ctx, caseID, ClockResponse, 50, oldStartedAt)
	if err != nil || !claimedOld {
		t.Fatalf("ClaimTier() (old incarnation) = (%v, %v), want (true, nil)", claimedOld, err)
	}

	// The new incarnation's own claim for the SAME tier must succeed
	// independently — a different key, since startedAt differs.
	claimedNew, err := store.ClaimTier(ctx, caseID, ClockResponse, 50, newStartedAt)
	if err != nil || !claimedNew {
		t.Errorf("ClaimTier() (new incarnation) = (%v, %v), want (true, nil) -- a stale claim from the old incarnation must not block it", claimedNew, err)
	}

	// A second claim attempt under the SAME (new) incarnation must still
	// correctly report already-claimed -- the fix must not make every claim
	// trivially succeed.
	claimedAgain, err := store.ClaimTier(ctx, caseID, ClockResponse, 50, newStartedAt)
	if err != nil || claimedAgain {
		t.Errorf("ClaimTier() (repeat, same incarnation) = (%v, %v), want (false, nil)", claimedAgain, err)
	}
}

// TestStore_SetClock_DistinguishesIncarnationsWithinTheSameSecond is the
// direct regression test for the subsecond-precision bug: two genuinely
// different incarnations whose start times fall within the same
// wall-clock second must still be told apart -- a whole-seconds timestamp
// would make the second SetClock look like a replay of the first, wrongly
// preserving its alertedTier cursor instead of resetting it.
func TestStore_SetClock_DistinguishesIncarnationsWithinTheSameSecond(t *testing.T) {
	store, closeStore := testStore(t)
	defer closeStore()
	ctx := context.Background()
	caseID := testCaseID(t)
	defer func() { _ = store.rdb.Del(ctx, clockKey(caseID, ClockResponse)) }()

	base := time.Now().Truncate(time.Second) // deterministically the start of a Unix second
	firstStartedAt := base
	secondStartedAt := base.Add(500 * time.Millisecond) // same Unix second, different instant

	if firstStartedAt.Unix() != secondStartedAt.Unix() {
		t.Fatalf("test setup broken: the two timestamps must fall in the same Unix second (got %d and %d)", firstStartedAt.Unix(), secondStartedAt.Unix())
	}

	if err := store.SetClock(ctx, caseID, ClockResponse, ClockMeta{Priority: "CATASTROPHIC", StartedAt: firstStartedAt}); err != nil {
		t.Fatalf("SetClock() (first incarnation) error = %v", err)
	}
	if _, err := store.AdvanceAlertedTier(ctx, caseID, ClockResponse, 100, time.Time{}); err != nil {
		t.Fatalf("AdvanceAlertedTier() error = %v", err)
	}

	if err := store.SetClock(ctx, caseID, ClockResponse, ClockMeta{Priority: "HIGH", StartedAt: secondStartedAt}); err != nil {
		t.Fatalf("SetClock() (second incarnation, same Unix second) error = %v", err)
	}

	got, found, err := store.GetClock(ctx, caseID, ClockResponse)
	if err != nil || !found {
		t.Fatalf("GetClock() = (%+v, %v, %v)", got, found, err)
	}
	if got.AlertedTier != 0 {
		t.Errorf("AlertedTier = %d, want 0 -- the second incarnation (same Unix second, different instant) must not be mistaken for a replay of the first", got.AlertedTier)
	}
	if !got.StartedAt.Equal(secondStartedAt) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, secondStartedAt)
	}
}

// TestStore_AdvanceAlertedTier_SkipsUpdateWhenIncarnationMismatches
// confirms the primitive processDueMember's own incarnation guard relies
// on: a caller-supplied incarnation that no longer matches the hash's
// current startedAt must neither report applied=true nor touch the
// stored cursor.
func TestStore_AdvanceAlertedTier_SkipsUpdateWhenIncarnationMismatches(t *testing.T) {
	store, closeStore := testStore(t)
	defer closeStore()
	ctx := context.Background()
	caseID := testCaseID(t)
	defer func() { _ = store.rdb.Del(ctx, clockKey(caseID, ClockResponse)) }()

	actualStartedAt := time.Now().Truncate(time.Second)
	staleStartedAt := actualStartedAt.Add(-time.Hour)

	if err := store.SetClock(ctx, caseID, ClockResponse, ClockMeta{Priority: "CATASTROPHIC", StartedAt: actualStartedAt}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}

	applied, err := store.AdvanceAlertedTier(ctx, caseID, ClockResponse, 50, staleStartedAt)
	if err != nil {
		t.Fatalf("AdvanceAlertedTier() error = %v", err)
	}
	if applied {
		t.Error("applied = true, want false -- the incarnation no longer matches")
	}

	got, _, err := store.GetClock(ctx, caseID, ClockResponse)
	if err != nil {
		t.Fatalf("GetClock() error = %v", err)
	}
	if got.AlertedTier != 0 {
		t.Errorf("AlertedTier = %d, want 0 -- a mismatched-incarnation call must not touch the cursor", got.AlertedTier)
	}

	// The matching incarnation must still succeed normally.
	applied, err = store.AdvanceAlertedTier(ctx, caseID, ClockResponse, 50, actualStartedAt)
	if err != nil || !applied {
		t.Fatalf("AdvanceAlertedTier() (matching incarnation) = (%v, %v), want (true, nil)", applied, err)
	}
}
