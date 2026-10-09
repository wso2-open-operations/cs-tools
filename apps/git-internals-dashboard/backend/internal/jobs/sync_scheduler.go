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
	"log/slog"
	"time"
)

// SyncScheduler runs a GitHub sync on a fixed interval, guarded by the same
// Lock as the recompute Scheduler and POST /sync/runs, so a scheduled sync
// never interleaves with either — on this replica or any other.
//
// The sync itself is injected as run so this package stays free of GitHub
// and ingest concerns, and so the loop is testable without the network. run
// owns its own deadline.
type SyncScheduler struct {
	lock     *Lock
	interval time.Duration
	run      func(ctx context.Context) error
}

// NewSyncScheduler builds a SyncScheduler. Call Start to begin ticking.
func NewSyncScheduler(lock *Lock, interval time.Duration, run func(ctx context.Context) error) *SyncScheduler {
	return &SyncScheduler{lock: lock, interval: interval, run: run}
}

// Start runs one sync every s.interval until ctx is done, with no tick at
// startup: syncing on every boot would hit GitHub on each restart or
// redeploy, and the per-repo watermark lets the next tick catch up anyway.
// Runs in its own goroutine; Start returns immediately.
func (s *SyncScheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

// tick runs one sync under the job lock, logging the outcome (success,
// skipped because the lock is busy, or failure) rather than propagating an
// error — there is no caller to return one to, and a failed tick must not
// stop the loop.
func (s *SyncScheduler) tick(ctx context.Context) {
	start := time.Now()
	// This runs in a bare goroutine, outside middleware.Recovery: an
	// unrecovered panic in the sync would take down the whole server. TryRun
	// releases the job lock on the way out of the unwinding stack.
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "scheduled github sync panicked", "panic", r)
		}
	}()
	_, ran, err := TryRun(ctx, s.lock, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, s.run(ctx)
	})
	if err != nil {
		slog.ErrorContext(ctx, "scheduled github sync failed", "err", err)
		return
	}
	if !ran {
		slog.InfoContext(ctx, "scheduled github sync skipped: job lock busy")
		return
	}
	slog.InfoContext(ctx, "scheduled github sync complete", "durationMs", time.Since(start).Milliseconds())
}
