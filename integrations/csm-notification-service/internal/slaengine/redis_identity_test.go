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
	"slices"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// testClockStartedAt is a fixed, arbitrary incarnation timestamp shared by
// every test below -- these tests are about SLAID identity matching, not
// incarnation timing, so a single fixed value keeps each test's own setup
// focused on the one thing it's actually varying.
var testClockStartedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// These tests exercise setPausedIfMatchScript/deleteClockHashIfMatchScript's
// real Lua logic against miniredis (an in-process fake Redis server, already
// a dependency here via internal/paging's own severity_record_test.go) --
// unlike redis_test.go's own TestStore_* tests, these never skip: there is
// no external Redis to reach, so they run on every `go test ./...` and in
// CI, giving the two CodeRabbit-flagged races below real, always-on
// coverage rather than relying on someone having a local Redis running.

func newMiniredisStore(t *testing.T) *Store {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewStore(rdb)
}

// TestStore_SetPaused_SkipsWriteOnSLAIDMismatch is the regression guard for
// the pause-write race CodeRabbit flagged: Engine.entityPauseStatus reads a
// specific clock's live pause verdict over a real network round trip, and
// ApplyStateEffects then writes it with SetPaused -- if the clock is
// replaced by a fresh incarnation (ReviseClocks) in that window, an
// identity-blind write would stamp the OLD clock's verdict onto the NEW
// one. The write must be skipped entirely, and the hash's own paused flag
// must stay exactly as the (correct, already-written-by-the-replacement)
// value it was.
func TestStore_SetPaused_SkipsWriteOnSLAIDMismatch(t *testing.T) {
	store := newMiniredisStore(t)
	ctx := context.Background()
	caseID := "case-mismatch"

	// The clock currently tracked is a fresh replacement (slaId "new-row"),
	// already correctly unpaused.
	if err := store.SetClock(ctx, caseID, ClockWorkaround, ClockMeta{StartedAt: testClockStartedAt, SLAID: "new-row"}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}

	// A stale pause verdict, captured under the OLD incarnation's id, tries
	// to write "paused" -- this must be rejected.
	applied, err := store.SetPaused(ctx, caseID, ClockWorkaround, true, "old-row")
	if err != nil {
		t.Fatalf("SetPaused() error = %v", err)
	}
	if applied {
		t.Error("applied = true, want false -- the clock was replaced, this stale write must be skipped")
	}

	meta, found, err := store.GetClock(ctx, caseID, ClockWorkaround)
	if err != nil || !found {
		t.Fatalf("GetClock() = (%+v, %v, %v)", meta, found, err)
	}
	if meta.Paused {
		t.Error("Paused = true, want false -- the stale write must not have taken effect on the replacement clock")
	}
	if meta.SLAID != "new-row" {
		t.Errorf("SLAID = %q, want it untouched (\"new-row\")", meta.SLAID)
	}
}

// TestStore_SetPaused_AppliesOnMatchingSLAID proves the ordinary, intended
// path still works: the identity check must not block a write whose SLAID
// genuinely matches the tracked clock.
func TestStore_SetPaused_AppliesOnMatchingSLAID(t *testing.T) {
	store := newMiniredisStore(t)
	ctx := context.Background()
	caseID := "case-match"

	if err := store.SetClock(ctx, caseID, ClockWorkaround, ClockMeta{StartedAt: testClockStartedAt, SLAID: "row-1"}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	applied, err := store.SetPaused(ctx, caseID, ClockWorkaround, true, "row-1")
	if err != nil {
		t.Fatalf("SetPaused() error = %v", err)
	}
	if !applied {
		t.Error("applied = false, want true -- the SLAID matches, this write must go through")
	}
	meta, _, err := store.GetClock(ctx, caseID, ClockWorkaround)
	if err != nil {
		t.Fatalf("GetClock() error = %v", err)
	}
	if !meta.Paused {
		t.Error("Paused = false, want true -- the matching write must have taken effect")
	}
}

// TestStore_SetPaused_AppliesWhenNoIdentityToCheck covers both "either side
// empty" cases: a legacy clock Redis holds with no SLAID yet (stored side
// empty), and a caller reacting to a live event rather than a stale
// snapshot, which deliberately passes "" (given side empty) -- see
// ApplyStateEffects' own CLOSED branch and PauseWorkaroundClock. Neither
// must be blocked: this is this method's original, identity-blind behavior,
// unchanged for both.
func TestStore_SetPaused_AppliesWhenNoIdentityToCheck(t *testing.T) {
	t.Run("stored SLAID empty (legacy clock)", func(t *testing.T) {
		store := newMiniredisStore(t)
		ctx := context.Background()
		caseID := "case-legacy"
		if err := store.SetClock(ctx, caseID, ClockWorkaround, ClockMeta{StartedAt: testClockStartedAt}); err != nil {
			t.Fatalf("SetClock() error = %v", err)
		}
		applied, err := store.SetPaused(ctx, caseID, ClockWorkaround, true, "some-row")
		if err != nil {
			t.Fatalf("SetPaused() error = %v", err)
		}
		if !applied {
			t.Error("applied = false, want true -- no stored SLAID to disagree with, must not block")
		}
	})

	t.Run("given slaID empty (a live-event caller)", func(t *testing.T) {
		store := newMiniredisStore(t)
		ctx := context.Background()
		caseID := "case-live-event"
		if err := store.SetClock(ctx, caseID, ClockWorkaround, ClockMeta{StartedAt: testClockStartedAt, SLAID: "row-1"}); err != nil {
			t.Fatalf("SetClock() error = %v", err)
		}
		applied, err := store.SetPaused(ctx, caseID, ClockWorkaround, true, "")
		if err != nil {
			t.Fatalf("SetPaused() error = %v", err)
		}
		if !applied {
			t.Error("applied = false, want true -- an empty given slaID means \"no identity to check\", must not block")
		}
	})
}

// TestStore_DeleteClockIfMatch_SkipsOnSLAIDMismatch is the regression guard
// for the cleanup-delete race CodeRabbit flagged: RecalculateFromEntity's
// cleanup pass verifies a clock is terminal (by its own SLAID), then
// deletes it; if a registration lands in that window and replaces the hash
// with a fresh, genuinely live incarnation, an identity-blind delete would
// wipe that live clock's tracking (hash AND wake entries) entirely. The
// whole delete must be skipped, not just the hash -- confirmed here by
// seeding a real wake entry too and asserting it survives.
func TestStore_DeleteClockIfMatch_SkipsOnSLAIDMismatch(t *testing.T) {
	store := newMiniredisStore(t)
	ctx := context.Background()
	caseID := "case-delete-mismatch"

	if err := store.SetClock(ctx, caseID, ClockWorkaround, ClockMeta{StartedAt: testClockStartedAt, SLAID: "live-replacement"}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	if err := store.AddWake(ctx, wakeMember(caseID, ClockWorkaround, 50), testClockStartedAt); err != nil {
		t.Fatalf("AddWake() error = %v", err)
	}

	applied, err := store.DeleteClockIfMatch(ctx, caseID, ClockWorkaround, "old-terminal-row")
	if err != nil {
		t.Fatalf("DeleteClockIfMatch() error = %v", err)
	}
	if applied {
		t.Error("applied = true, want false -- the clock was replaced, this delete must be skipped")
	}

	meta, found, err := store.GetClock(ctx, caseID, ClockWorkaround)
	if err != nil || !found {
		t.Fatalf("GetClock() = (%+v, %v, %v), want the replacement clock to survive untouched", meta, found, err)
	}
	if meta.SLAID != "live-replacement" {
		t.Errorf("SLAID = %q, want it untouched", meta.SLAID)
	}
	due, err := store.DueMembers(ctx, testClockStartedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("DueMembers() error = %v", err)
	}
	if !slices.Contains(due, wakeMember(caseID, ClockWorkaround, 50)) {
		t.Error("the replacement clock's own wake entry was removed -- the skipped delete must not touch it")
	}
}

// TestStore_DeleteClockIfMatch_DeletesOnMatch proves the ordinary, intended
// path still works: a genuinely matching SLAID deletes the hash and its
// wake entries, same as the original DeleteClock always did.
func TestStore_DeleteClockIfMatch_DeletesOnMatch(t *testing.T) {
	store := newMiniredisStore(t)
	ctx := context.Background()
	caseID := "case-delete-match"

	if err := store.SetClock(ctx, caseID, ClockWorkaround, ClockMeta{StartedAt: testClockStartedAt, SLAID: "row-1"}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	if err := store.AddWake(ctx, wakeMember(caseID, ClockWorkaround, 50), testClockStartedAt); err != nil {
		t.Fatalf("AddWake() error = %v", err)
	}

	applied, err := store.DeleteClockIfMatch(ctx, caseID, ClockWorkaround, "row-1")
	if err != nil {
		t.Fatalf("DeleteClockIfMatch() error = %v", err)
	}
	if !applied {
		t.Error("applied = false, want true -- the SLAID matches, this delete must go through")
	}
	if _, found, err := store.GetClock(ctx, caseID, ClockWorkaround); err != nil || found {
		t.Errorf("GetClock() found = %v, err = %v, want the hash gone", found, err)
	}
	due, err := store.DueMembers(ctx, testClockStartedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("DueMembers() error = %v", err)
	}
	if slices.Contains(due, wakeMember(caseID, ClockWorkaround, 50)) {
		t.Error("the wake entry survived a matching delete, want it removed")
	}
}

// TestStore_DeleteClockIfMatch_AppliesWhenNoIdentityToCheck preserves the
// original, identity-blind behavior for a legacy clock with no SLAID yet
// (RecalculateFromEntity's cleanup pass still deletes it outright, exactly
// as DeleteClock always did).
func TestStore_DeleteClockIfMatch_AppliesWhenNoIdentityToCheck(t *testing.T) {
	store := newMiniredisStore(t)
	ctx := context.Background()
	caseID := "case-delete-legacy"

	if err := store.SetClock(ctx, caseID, ClockWorkaround, ClockMeta{StartedAt: testClockStartedAt}); err != nil {
		t.Fatalf("SetClock() error = %v", err)
	}
	applied, err := store.DeleteClockIfMatch(ctx, caseID, ClockWorkaround, "")
	if err != nil {
		t.Fatalf("DeleteClockIfMatch() error = %v", err)
	}
	if !applied {
		t.Error("applied = false, want true -- no SLAID on either side to disagree, must not block")
	}
}
