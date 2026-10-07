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
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
)

// outageNoticeChannel is the NOTIFY channel migration 0186's trigger signals
// on every committed insert or update of outage.
const outageNoticeChannel = "outage_notice"

// OutageChangeListener holds one pooled connection LISTENing on the outage
// change channel. Not safe for concurrent use: one drainer owns one listener.
type OutageChangeListener struct {
	db   db.Pool
	conn *pgxpool.Conn
}

// NewOutageChangeListener returns a listener that is not yet listening.
func NewOutageChangeListener(db db.Pool) *OutageChangeListener {
	return &OutageChangeListener{db: db}
}

// Listen takes a dedicated connection and subscribes. Calling it while already
// listening is a no-op.
func (l *OutageChangeListener) Listen(ctx context.Context) error {
	if l.conn != nil {
		return nil
	}
	conn, err := l.db.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire listener connection: %w", err)
	}
	if _, err := conn.Exec(ctx, "LISTEN "+outageNoticeChannel); err != nil {
		conn.Release()
		return fmt.Errorf("listen %s: %w", outageNoticeChannel, err)
	}
	l.conn = conn
	return nil
}

// Wait blocks until an outage change is notified (true) or timeout passes
// (false, nil). Any other error means the connection is unusable; the caller
// should Close and Listen again.
//
// A timeout leaves the connection open and listening: pgx closes a connection
// only on a non-timeout network error.
func (l *OutageChangeListener) Wait(ctx context.Context, timeout time.Duration) (bool, error) {
	if l.conn == nil {
		return false, errors.New("outage change listener is not listening")
	}
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := l.conn.Conn().WaitForNotification(wctx)
	switch {
	case err == nil:
		return true, nil
	case ctx.Err() != nil:
		return false, ctx.Err()
	case errors.Is(wctx.Err(), context.DeadlineExceeded):
		return false, nil
	default:
		return false, err
	}
}

// Close stops listening and returns the connection, or discards it if it can
// no longer be cleaned up, so no pooled connection is left subscribed.
func (l *OutageChangeListener) Close() {
	if l.conn == nil {
		return
	}
	if _, err := l.conn.Exec(context.Background(), "UNLISTEN "+outageNoticeChannel); err != nil {
		_ = l.conn.Conn().Close(context.Background())
	}
	l.conn.Release()
	l.conn = nil
}
