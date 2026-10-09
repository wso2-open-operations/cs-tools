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
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestSyncSchedulerTickRunsSyncUnderLock(t *testing.T) {
	lock := NewLock(testDatabaseURL(t))
	var calls atomic.Int32
	s := NewSyncScheduler(lock, time.Hour, func(ctx context.Context) error {
		calls.Add(1)
		if !lock.Running() {
			t.Error("expected the job lock to be held while the sync runs")
		}
		return nil
	})

	s.tick(context.Background())

	if got := calls.Load(); got != 1 {
		t.Errorf("expected 1 sync call, got %d", got)
	}
}

func TestSyncSchedulerTickSkipsWhenLockBusy(t *testing.T) {
	url := testDatabaseURL(t)
	other := NewLock(url) // a second replica holding the lock
	held := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = TryRun(context.Background(), other, func(ctx context.Context) (struct{}, error) {
			close(held)
			<-release
			return struct{}{}, nil
		})
	}()
	<-held

	var calls atomic.Int32
	s := NewSyncScheduler(NewLock(url), time.Hour, func(ctx context.Context) error {
		calls.Add(1)
		return nil
	})
	s.tick(context.Background())
	close(release)
	<-done

	if got := calls.Load(); got != 0 {
		t.Errorf("expected the sync to be skipped while the lock is busy, got %d calls", got)
	}
}

func TestSyncSchedulerKeepsTickingAfterFailure(t *testing.T) {
	lock := NewLock(testDatabaseURL(t))
	var calls atomic.Int32
	second := make(chan struct{})
	s := NewSyncScheduler(lock, 20*time.Millisecond, func(ctx context.Context) error {
		if calls.Add(1) == 2 {
			close(second)
		}
		return errors.New("github unavailable")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	select {
	case <-second:
	case <-time.After(5 * time.Second):
		t.Fatalf("expected a second tick after a failed one, got %d calls", calls.Load())
	}
}

func TestSyncSchedulerDoesNotTickBeforeFirstInterval(t *testing.T) {
	lock := NewLock(testDatabaseURL(t))
	var calls atomic.Int32
	s := NewSyncScheduler(lock, time.Hour, func(ctx context.Context) error {
		calls.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()

	if got := calls.Load(); got != 0 {
		t.Errorf("expected no sync at startup, got %d calls", got)
	}
}

func TestSyncSchedulerStopsOnContextCancel(t *testing.T) {
	lock := NewLock(testDatabaseURL(t))
	var calls atomic.Int32
	s := NewSyncScheduler(lock, 10*time.Millisecond, func(ctx context.Context) error {
		calls.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond) // let any in-flight tick finish
	after := calls.Load()
	time.Sleep(100 * time.Millisecond)

	if after == 0 {
		t.Fatal("expected the scheduler to tick before cancel")
	}
	if got := calls.Load(); got != after {
		t.Errorf("expected no ticks after cancel, went from %d to %d", after, got)
	}
}

func TestSyncSchedulerTickRecoversFromPanic(t *testing.T) {
	lock := NewLock(testDatabaseURL(t))
	var calls atomic.Int32
	s := NewSyncScheduler(lock, time.Hour, func(ctx context.Context) error {
		if calls.Add(1) == 1 {
			panic("boom")
		}
		return nil
	})

	s.tick(context.Background()) // must not panic out of tick

	if lock.Running() {
		t.Error("expected the job lock to be released after a panicking run")
	}
	s.tick(context.Background())
	if got := calls.Load(); got != 2 {
		t.Errorf("expected the next tick to run normally after a panic, got %d calls", got)
	}
}
