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

package jobs

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
)

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://gid:gid@localhost:5433/gid?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Skipf("skipping: postgres unreachable at %s: %v", url, err)
	}
	pool.Close()
	return url
}

// TestLockMutualExclusionAcrossReplicas simulates two Choreo replicas (two
// independent Lock instances, each with its own dedicated connection)
// racing for the same Postgres advisory lock: exactly one must run.
func TestLockMutualExclusionAcrossReplicas(t *testing.T) {
	url := testDatabaseURL(t)
	replicaA := NewLock(url)
	replicaB := NewLock(url)

	release := make(chan struct{})
	started := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)

	var aRan, bRan bool
	var aErr, bErr error

	go func() {
		defer wg.Done()
		_, ran, err := TryRun(context.Background(), replicaA, func(ctx context.Context) (struct{}, error) {
			close(started)
			<-release
			return struct{}{}, nil
		})
		aRan, aErr = ran, err
	}()

	<-started // ensure A holds the lock before B attempts it
	go func() {
		defer wg.Done()
		_, ran, err := TryRun(context.Background(), replicaB, func(ctx context.Context) (struct{}, error) {
			return struct{}{}, nil
		})
		bRan, bErr = ran, err
	}()

	time.Sleep(100 * time.Millisecond) // give B's attempt time to reach Postgres
	close(release)
	wg.Wait()

	if aErr != nil {
		t.Fatalf("replica A: unexpected error: %v", aErr)
	}
	if bErr != nil {
		t.Fatalf("replica B: unexpected error: %v", bErr)
	}
	if !aRan {
		t.Error("expected replica A (holder) to have run")
	}
	if bRan {
		t.Error("expected replica B to be skipped while A holds the lock")
	}
}

// TestLockSameInstanceFastPath exercises the in-process running flag: a
// second TryRun on the SAME Lock instance while the first is in flight must
// be skipped without ever reaching Postgres.
func TestLockSameInstanceFastPath(t *testing.T) {
	url := testDatabaseURL(t)
	lock := NewLock(url)

	release := make(chan struct{})
	started := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)

	var firstRan, secondRan bool

	go func() {
		defer wg.Done()
		_, ran, _ := TryRun(context.Background(), lock, func(ctx context.Context) (struct{}, error) {
			close(started)
			<-release
			return struct{}{}, nil
		})
		firstRan = ran
	}()

	<-started
	go func() {
		defer wg.Done()
		_, ran, _ := TryRun(context.Background(), lock, func(ctx context.Context) (struct{}, error) {
			return struct{}{}, nil
		})
		secondRan = ran
	}()

	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if !firstRan {
		t.Error("expected the first call to run")
	}
	if secondRan {
		t.Error("expected the second call on the same instance to be skipped")
	}
}

// TestLockReleasesAfterRunSoASubsequentCallCanAcquireIt guards against the
// lock wedging: after a successful TryRun completes, a later call (even on a
// different instance) must be able to acquire it.
func TestLockReleasesAfterRunSoASubsequentCallCanAcquireIt(t *testing.T) {
	url := testDatabaseURL(t)
	lockA := NewLock(url)
	lockB := NewLock(url)

	_, ranA, err := TryRun(context.Background(), lockA, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, nil
	})
	if err != nil || !ranA {
		t.Fatalf("expected first run to succeed, ran=%v err=%v", ranA, err)
	}

	_, ranB, err := TryRun(context.Background(), lockB, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, nil
	})
	if err != nil || !ranB {
		t.Fatalf("expected second run (different instance) to succeed after release, ran=%v err=%v", ranB, err)
	}
}

// TestLockDropsConnectionWhenUnlockFails (AUDIT-FINDINGS A3): if
// pg_advisory_unlock's Exec fails at release time, release() must drop the
// dedicated connection rather than merely log — otherwise l.conn keeps
// pointing at a dead connection forever and every future acquire on this
// instance fails, wedging sync/recompute cluster-wide until a restart.
func TestLockDropsConnectionWhenUnlockFails(t *testing.T) {
	url := testDatabaseURL(t)
	lock := NewLock(url)

	acquired, err := lock.acquire(context.Background())
	if err != nil || !acquired {
		t.Fatalf("expected to acquire the lock, acquired=%v err=%v", acquired, err)
	}

	// Simulate a broken connection at release time (network blip / statement
	// timeout): close the underlying connection out from under the lock so
	// the unlock Exec inside release() fails on it.
	lock.mu.Lock()
	conn := lock.conn
	lock.mu.Unlock()
	if conn == nil {
		t.Fatal("expected acquire to have set a dedicated connection")
	}
	if err := conn.Close(context.Background()); err != nil {
		t.Fatalf("simulate broken connection: %v", err)
	}

	lock.release()

	// Without the fix, l.conn still points at the now-closed connection and
	// getConn never reconnects (it only dials when l.conn == nil), so this
	// second acquire on the SAME instance would fail forever.
	acquired2, err := lock.acquire(context.Background())
	if err != nil || !acquired2 {
		t.Fatalf("expected the same lock instance to acquire again after a broken unlock, acquired=%v err=%v", acquired2, err)
	}
	lock.release()
}
