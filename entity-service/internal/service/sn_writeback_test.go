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

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type recordingSNWritebackFailures struct {
	mu    sync.Mutex
	calls []domain.CreateSNWritebackFailureRequest
}

func (r *recordingSNWritebackFailures) Create(_ context.Context, req domain.CreateSNWritebackFailureRequest) (domain.SNWritebackFailure, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, req)
	return domain.SNWritebackFailure{ID: "f1"}, nil
}

func (r *recordingSNWritebackFailures) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// waitFor polls until cond returns true or the timeout elapses, failing the
// test on timeout — Dispatch is fire-and-forget, so tests observe its
// background worker's effect rather than a return value.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestSNWritebackDispatcher_SuccessDoesNotRecordFailure(t *testing.T) {
	failures := &recordingSNWritebackFailures{}
	d := NewSNWritebackDispatcher(failures)

	done := make(chan struct{})
	d.Dispatch(context.Background(), "account", "acc-1", "update", map[string]string{"name": "Example Corp"}, func(context.Context) error {
		close(done)
		return nil
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("writeFn was never invoked")
	}
	// Give the worker a moment past writeFn returning to make sure a failure
	// row is never inserted for a successful write.
	time.Sleep(50 * time.Millisecond)
	if got := failures.count(); got != 0 {
		t.Fatalf("expected 0 failure records for a successful write, got %d", got)
	}
}

func TestSNWritebackDispatcher_FailureIsRecorded(t *testing.T) {
	failures := &recordingSNWritebackFailures{}
	d := NewSNWritebackDispatcher(failures)

	wantErr := errors.New("sn downstream unreachable")
	d.Dispatch(context.Background(), "account", "acc-2", "create", map[string]string{"name": "Example Corp"}, func(context.Context) error {
		return wantErr
	})

	waitFor(t, func() bool { return failures.count() == 1 })

	req := failures.calls[0]
	if req.EntityType != "account" || req.EntityID != "acc-2" || req.Operation != "create" {
		t.Fatalf("unexpected failure record: %+v", req)
	}
	if req.Error != wantErr.Error() {
		t.Fatalf("expected error %q, got %q", wantErr.Error(), req.Error)
	}
	if len(req.Payload) == 0 {
		t.Fatal("expected a non-empty marshaled payload")
	}
}

func TestSNWritebackDispatcher_DoesNotBlockCallerOnSlowWrite(t *testing.T) {
	failures := &recordingSNWritebackFailures{}
	d := NewSNWritebackDispatcher(failures)

	release := make(chan struct{})
	started := make(chan struct{})
	dispatchReturned := make(chan struct{})

	go func() {
		d.Dispatch(context.Background(), "account", "acc-3", "update", map[string]string{}, func(context.Context) error {
			close(started)
			<-release
			return nil
		})
		close(dispatchReturned)
	}()

	select {
	case <-dispatchReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("Dispatch blocked on writeFn instead of returning immediately")
	}

	<-started
	close(release)
}

// TestSNWritebackDispatcher_SerializesSameEntityInDispatchOrder proves the
// fix for the case WatchList reordering finding: two writeback jobs for the
// same entity ("case:case-1") are dispatched in order A-then-B, with A's
// mirror write artificially delayed so that, absent per-entity
// serialization, B (on a different worker) would finish first and A's stale
// write would land afterward and clobber it. With the fix, B must not start
// until A has completed, so the final mirrored state always reflects B.
func TestSNWritebackDispatcher_SerializesSameEntityInDispatchOrder(t *testing.T) {
	failures := &recordingSNWritebackFailures{}
	d := NewSNWritebackDispatcher(failures)

	var mu sync.Mutex
	var applied []string   // order values were applied to the "mirror" in
	var started []string   // order writeFns actually started running in
	aStarted := make(chan struct{})
	done := make(chan struct{}, 2)

	// A: dispatched first, but its write is slow -- long enough that, without
	// per-entity serialization, B would race ahead of it on another worker.
	d.Dispatch(context.Background(), "case", "case-1", "update", map[string]string{"watchList": "A"}, func(context.Context) error {
		mu.Lock()
		started = append(started, "A")
		mu.Unlock()
		close(aStarted)
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		applied = append(applied, "A")
		mu.Unlock()
		done <- struct{}{}
		return nil
	})

	// B: dispatched immediately after A, with a fast write. If B ran
	// concurrently with A on a different worker, it would finish well before
	// A's 100ms sleep elapses, and A's later completion would overwrite B's
	// result -- the exact drift CodeRabbit flagged.
	d.Dispatch(context.Background(), "case", "case-1", "update", map[string]string{"watchList": "B"}, func(context.Context) error {
		mu.Lock()
		started = append(started, "B")
		applied = append(applied, "B")
		mu.Unlock()
		done <- struct{}{}
		return nil
	})

	<-aStarted
	// Give a would-be concurrent B a generous window to have started and
	// finished if serialization were missing.
	time.Sleep(30 * time.Millisecond)
	mu.Lock()
	startedSoFar := append([]string(nil), started...)
	mu.Unlock()
	if len(startedSoFar) != 1 || startedSoFar[0] != "A" {
		t.Fatalf("expected only A to have started while A's write is in flight, got %v", startedSoFar)
	}

	<-done
	<-done

	mu.Lock()
	defer mu.Unlock()
	if got := append([]string(nil), started...); len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("expected writeFns to start in dispatch order [A B], got %v", got)
	}
	if got := append([]string(nil), applied...); len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("final mirrored state must reflect the newer write B applied last, got %v", got)
	}
}

// TestSNWritebackDispatcher_DifferentEntitiesRunConcurrently guards against
// an over-broad fix: serializing per-entity must not collapse into global
// serialization. Two jobs for different entities, both blocked on the same
// gate, must both be able to start before either releases -- proving they
// run on separate workers concurrently rather than queued behind each other.
func TestSNWritebackDispatcher_DifferentEntitiesRunConcurrently(t *testing.T) {
	failures := &recordingSNWritebackFailures{}
	d := NewSNWritebackDispatcher(failures)

	var mu sync.Mutex
	startedCount := 0
	bothStarted := make(chan struct{})
	release := make(chan struct{})

	makeWriteFn := func() func(context.Context) error {
		return func(context.Context) error {
			mu.Lock()
			startedCount++
			n := startedCount
			mu.Unlock()
			if n == 2 {
				close(bothStarted)
			}
			<-release
			return nil
		}
	}

	d.Dispatch(context.Background(), "case", "case-A", "update", map[string]string{}, makeWriteFn())
	d.Dispatch(context.Background(), "case", "case-B", "update", map[string]string{}, makeWriteFn())

	select {
	case <-bothStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected both different-entity jobs to start concurrently; per-entity serialization must not become global serialization")
	}
	close(release)
}

func TestSNWritebackDispatcher_DetachesFromCallerContext(t *testing.T) {
	failures := &recordingSNWritebackFailures{}
	d := NewSNWritebackDispatcher(failures)

	callerCtx, cancel := context.WithCancel(context.Background())
	ran := make(chan error, 1)

	d.Dispatch(callerCtx, "account", "acc-4", "update", map[string]string{}, func(ctx context.Context) error {
		// Simulate the request completing (and its context being canceled)
		// before the background write finishes.
		<-time.After(30 * time.Millisecond)
		ran <- ctx.Err()
		return nil
	})

	cancel() // caller's HTTP request "returns" here

	select {
	case err := <-ran:
		if err != nil {
			t.Fatalf("expected writeFn's context to survive caller cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("writeFn was never invoked")
	}
}
