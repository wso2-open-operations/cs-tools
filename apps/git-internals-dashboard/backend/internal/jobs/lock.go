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

// Package jobs holds the cross-replica job mutex and the in-process
// recompute scheduler (SPEC §8, port of v3's src/server/jobs/{lock,recompute}.ts).
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// lockKey is an arbitrary fixed key, unique to this app's job lock.
const lockKey = 847_362_915

// Lock is a cross-replica mutex for the recompute tick and manual sync, so
// they never interleave — including across multiple Choreo replicas under
// autoscaling. Backed by a Postgres session-level advisory lock
// (pg_try_advisory_lock), held on one dedicated *pgx.Conn for the process
// lifetime — NOT a pooled connection, since advisory locks are tied to the
// exact session that acquired them and a pool may hand the "unlock" call a
// different connection otherwise. Reconnects lazily on the next attempt if
// the connection errors.
//
// running is a same-process fast-path flag, claimed synchronously (under mu)
// before anything else runs, and rolled back in every exit path so a failed
// attempt never wedges the lock for good.
type Lock struct {
	databaseURL string

	mu      sync.Mutex
	running bool
	conn    *pgx.Conn
}

// NewLock returns a Lock that connects to databaseURL on first use.
func NewLock(databaseURL string) *Lock {
	return &Lock{databaseURL: databaseURL}
}

// acquire claims the in-process fast path and the Postgres advisory lock, in
// that order. Returns (true, nil) only when both are held — the caller must
// call release() exactly once in that case.
func (l *Lock) acquire(ctx context.Context) (bool, error) {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return false, nil // in-process fast path
	}
	l.running = true
	l.mu.Unlock()

	conn, err := l.getConn(ctx)
	if err != nil {
		l.clearRunning()
		return false, fmt.Errorf("joblock: connect: %w", err)
	}

	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey).Scan(&locked); err != nil {
		l.dropConn(ctx)
		l.clearRunning()
		return false, fmt.Errorf("joblock: try advisory lock: %w", err)
	}
	if !locked {
		l.clearRunning()
		return false, nil // another replica holds it
	}
	return true, nil
}

// release rolls back the in-process flag and, best-effort, the Postgres
// advisory lock. Failure to unlock is logged, not returned — the caller
// already has whatever result fn produced, and the session-scoped lock is
// released automatically if this connection is ever dropped anyway.
func (l *Lock) release() {
	l.clearRunning()

	l.mu.Lock()
	conn := l.conn
	l.mu.Unlock()
	if conn == nil {
		return
	}
	unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := conn.Exec(unlockCtx, `SELECT pg_advisory_unlock($1)`, lockKey); err != nil {
		slog.Error("joblock: failed to release advisory lock; dropping connection", "err", err)
		// The advisory lock is scoped to this session. If we can't unlock it
		// explicitly, closing the session releases it server-side anyway —
		// otherwise every future acquire on every replica would see
		// locked=false forever (AUDIT-FINDINGS A3). getConn reconnects
		// lazily on the next attempt.
		l.dropConn(unlockCtx)
	}
}

// Running reports whether this replica currently holds the lock (the
// in-process fast-path flag) — SPEC §6.9's GET /sync/status "running" field
// reflects this replica's activity only; watermarks and lastRun come from
// the DB and are shared across replicas.
func (l *Lock) Running() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

func (l *Lock) clearRunning() {
	l.mu.Lock()
	l.running = false
	l.mu.Unlock()
}

func (l *Lock) getConn(ctx context.Context) (*pgx.Conn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn != nil {
		return l.conn, nil
	}
	conn, err := pgx.Connect(ctx, l.databaseURL)
	if err != nil {
		return nil, err
	}
	l.conn = conn
	return conn, nil
}

func (l *Lock) dropConn(ctx context.Context) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		return
	}
	_ = l.conn.Close(ctx)
	l.conn = nil
}

// TryRun claims l and, only if acquired, runs fn and returns its result with
// ran=true. If the lock is busy (this replica already running it, or
// another replica holds the Postgres advisory lock), fn never runs and
// TryRun returns the zero value with ran=false, err=nil. A failure to
// acquire the lock itself (e.g. a connection error) returns ran=false with a
// non-nil err.
//
// A free function rather than a method because Go methods cannot introduce
// their own type parameters distinct from the receiver's, and one process
// shares a single *Lock across callers needing different result types (the
// recompute tick returns a TickSummary; a future manual-sync route returns
// its own summary).
func TryRun[T any](ctx context.Context, l *Lock, fn func(ctx context.Context) (T, error)) (result T, ran bool, err error) {
	acquired, err := l.acquire(ctx)
	if err != nil {
		var zero T
		return zero, false, err
	}
	if !acquired {
		var zero T
		return zero, false, nil
	}
	defer l.release()
	result, err = fn(ctx)
	return result, true, err
}
