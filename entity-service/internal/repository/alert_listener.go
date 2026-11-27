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

package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Reconnect waits of the alert_stored listener: from alertListenRetryMin, doubling up to alertListenRetryMax.
const (
	alertListenRetryMin = time.Second
	alertListenRetryMax = 30 * time.Second
)

// ListenAlertStored LISTENs on AlertStoredChannel and calls wake for every notification, and once after each
// (re)connect so alerts stored while it was not listening are not left for the next poll. It runs until ctx is done.
//
// It holds one connection of its own, opened from the pool's settings but outside the pool: a pooled connection would
// keep LISTENing after it went back, and a pool under load could starve it. So the service uses one Postgres
// connection more than DB_POOL_MAX_CONNS. LISTEN needs a session-level connection: through a pooler in transaction
// mode it never receives anything, and the processor's poll is then the only wake-up. A lost connection is reopened
// with backoff; meanwhile the poll covers.
func ListenAlertStored(ctx context.Context, pool *pgxpool.Pool, wake func()) {
	wait := alertListenRetryMin
	for ctx.Err() == nil {
		connected, err := listenAlertStoredOnce(ctx, pool, wake)
		if ctx.Err() != nil {
			return
		}
		if connected {
			wait = alertListenRetryMin
		}
		slog.WarnContext(ctx, "alert processor: the alert_stored listener stopped, reconnecting; the poll covers meanwhile", "in", wait, "err", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, alertListenRetryMax)
	}
}

// listenAlertStoredOnce listens on one connection until it fails or ctx is done; connected reports that LISTEN took
// effect, so the caller resets its backoff.
func listenAlertStoredOnce(ctx context.Context, pool *pgxpool.Pool, wake func()) (connected bool, err error) {
	conn, err := pgx.ConnectConfig(ctx, pool.Config().ConnConfig.Copy())
	if err != nil {
		return false, fmt.Errorf("connect: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}()
	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{AlertStoredChannel}.Sanitize()); err != nil {
		return false, fmt.Errorf("listen: %w", err)
	}
	wake()
	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			return true, fmt.Errorf("wait for notification: %w", err)
		}
		wake()
	}
}
