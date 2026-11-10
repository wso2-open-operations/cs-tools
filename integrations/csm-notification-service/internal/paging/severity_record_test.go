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

package paging

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// The severity-change record and lock run their real Redis calls (the
// compare-and-set and compare-and-delete scripts, SET NX, expiry) against
// miniredis: the record never moves back, each ladder keeps its own, and only
// the holder's token releases the lock.
func TestStore_SeverityChangeRecordAndLock(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	ctx := context.Background()
	cre, sre := NewStore(rdb), NewStore(rdb).ForLadder(LadderSRE)

	newer, older := testClock.Add(time.Hour), testClock
	if err := cre.SetSeverityChangedAt(ctx, "case-1", newer); err != nil {
		t.Fatal(err)
	}
	if err := cre.SetSeverityChangedAt(ctx, "case-1", older); err != nil {
		t.Fatal(err)
	}
	if got, err := cre.SeverityChangedAt(ctx, "case-1"); err != nil || !got.Equal(newer) {
		t.Fatalf("record = %v, %v; want %v: an older change must not move it back", got, err, newer)
	}
	if got, _ := sre.SeverityChangedAt(ctx, "case-1"); !got.IsZero() {
		t.Fatalf("SRE record = %v; want none: each ladder keeps its own", got)
	}

	if ok, err := cre.LockCaseSeverity(ctx, "case-1", "a", time.Minute); err != nil || !ok {
		t.Fatalf("first lock = %v, %v; want taken", ok, err)
	}
	if ok, _ := cre.LockCaseSeverity(ctx, "case-1", "b", time.Minute); ok {
		t.Fatal("a second holder took a held lock")
	}
	if ok, _ := sre.LockCaseSeverity(ctx, "case-1", "c", time.Minute); !ok {
		t.Fatal("the SRE ladder's lock was held by the CRE ladder's")
	}
	if err := cre.UnlockCaseSeverity(ctx, "case-1", "b"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := cre.LockCaseSeverity(ctx, "case-1", "b", time.Minute); ok {
		t.Fatal("another holder's token released the lock")
	}
	if err := cre.UnlockCaseSeverity(ctx, "case-1", "a"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := cre.LockCaseSeverity(ctx, "case-1", "b", time.Minute); !ok {
		t.Fatal("the lock was not free after its holder released it")
	}
	mr.FastForward(2 * time.Minute)
	if ok, _ := cre.LockCaseSeverity(ctx, "case-1", "d", time.Minute); !ok {
		t.Fatal("an expired lock was not free")
	}

	// Renewal: only the holder's token extends the lock, and an extended
	// lock outlives its first TTL.
	if ok, err := cre.RenewCaseSeverity(ctx, "case-1", "x", time.Minute); err != nil || ok {
		t.Fatalf("renew with another token = %v, %v; want refused", ok, err)
	}
	mr.FastForward(50 * time.Second)
	if ok, err := cre.RenewCaseSeverity(ctx, "case-1", "d", time.Minute); err != nil || !ok {
		t.Fatalf("renew by the holder = %v, %v; want extended", ok, err)
	}
	mr.FastForward(50 * time.Second)
	if ok, _ := cre.LockCaseSeverity(ctx, "case-1", "e", time.Minute); ok {
		t.Fatal("a renewed lock expired at its first TTL")
	}
	mr.FastForward(time.Minute)
	if ok, _ := cre.RenewCaseSeverity(ctx, "case-1", "d", time.Minute); ok {
		t.Fatal("an expired lock was renewed")
	}
}

// A held lock is renewed while the work runs, so the work's context stays
// live past the TTL; once another holder has the lock, the work's context is
// cancelled, and releasing never frees the new holder's lock.
func TestEngine_SeverityLockLeaseRenewsAndCancelsOnLoss(t *testing.T) {
	prev := severityLockRenew
	severityLockRenew = 10 * time.Millisecond
	t.Cleanup(func() { severityLockRenew = prev })

	ctx := context.Background()
	store := newMemStore()
	cre := ladderEngine(LadderCRE, &fakeChat{}, store)

	held, unlock, err := cre.lockCaseSeverity(ctx, testIncidentID)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if held.Err() != nil {
		t.Fatal("renewing a held lock cancelled the work")
	}
	store.stealSeverityLock(testIncidentID)
	select {
	case <-held.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("losing the lock did not cancel the work")
	}
	unlock()
	if token, ok := store.severityLockHolder(testIncidentID); !ok || token != "someone-else" {
		t.Fatalf("lock holder = %q, %v after release; the new holder's lock must stay", token, ok)
	}

	other := newMemStore()
	cre = ladderEngine(LadderCRE, &fakeChat{}, other)
	held, unlock, err = cre.lockCaseSeverity(ctx, testIncidentID)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	if _, ok := other.severityLockHolder(testIncidentID); ok {
		t.Fatal("the lock was not released")
	}
	if held.Err() == nil {
		t.Fatal("the work's context outlived the release")
	}
}

// A severity change that finds another handler of the same case holding the
// lock waits, then fails so the consumer retries it; it never acts unlocked.
func TestEngine_SeverityChangeWaitsForTheCaseLock(t *testing.T) {
	prev := severityLockWait
	severityLockWait = 200 * time.Millisecond
	t.Cleanup(func() { severityLockWait = prev })

	ctx := context.Background()
	store := newMemStore()
	cre := ladderEngine(LadderCRE, &fakeChat{}, store)
	if err := cre.Handle(ctx, caseCreated(t, "Atlas", "HIGH")); err != nil {
		t.Fatal(err)
	}
	if ok, _ := store.LockCaseSeverity(ctx, testIncidentID, "other", time.Minute); !ok {
		t.Fatal("could not take the lock for the test")
	}

	err := cre.Handle(ctx, severityChangedAt(t, "HIGH", "CRITICAL", testClock.Add(time.Hour)))
	if err == nil || !strings.Contains(err.Error(), "still being handled") {
		t.Fatalf("err = %v; want a retryable error while the case is locked", err)
	}
	if st, _, _ := store.Get(ctx, testIncidentID); st.Plan.Trigger.Kind == TriggerSeverityChanged {
		t.Fatal("the severity change acted without the lock")
	}

	_ = store.UnlockCaseSeverity(ctx, testIncidentID, "other")
	if err := cre.Handle(ctx, severityChangedAt(t, "HIGH", "CRITICAL", testClock.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := store.Get(ctx, testIncidentID); st.Plan.Trigger.Kind != TriggerSeverityChanged {
		t.Fatal("the retried severity change did not start its chain")
	}
	if _, held := store.severityLockHolder(testIncidentID); held {
		t.Fatal("the lock was not released after the change was handled")
	}
}
