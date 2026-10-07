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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
)

// Session advisory-lock keys for the two outage email sweeps, one per flow so
// neither waits on the other. Fixed values: every replica and every caller
// (the outage notice drainer, the HTTP sweep endpoints) must agree on them.
const (
	outageNotificationSweepLockKey  int64 = 0x6f75_7467_0000_0001
	outageCommunicationSweepLockKey int64 = 0x6f75_7467_0000_0002
)

// tryAdvisoryLock takes a session-level advisory lock on its own pooled
// connection without waiting.
//
// *** WHY A LOCK AT ALL. *** A sweep decides what an outage is owed, records
// it, then returns it for sending. Two sweeps running at once -- two
// entity-service replicas' drainers, or a drainer and a manual call to the
// sweep endpoint -- can both read an outage before either records it, and the
// email goes out twice. Holding the lock for the whole sweep makes them take
// turns; a sweep that cannot get it simply skips, because the one holding it
// is already sending whatever is owed.
//
// ok is false when another session holds it. release must be called exactly
// once when ok is true.
func tryAdvisoryLock(ctx context.Context, pool db.Pool, key int64) (release func(), ok bool, err error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire connection for sweep lock: %w", err)
	}
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok); err != nil {
		conn.Release()
		return nil, false, fmt.Errorf("take sweep lock: %w", err)
	}
	if !ok {
		conn.Release()
		return nil, false, nil
	}
	return func() {
		// A fresh context: the caller's may already be cancelled, and a lock
		// left on a pooled connection would block every later sweep for as
		// long as that connection lives. If unlocking fails, close the
		// connection instead -- ending the session releases its locks.
		if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", key); err != nil {
			_ = conn.Conn().Close(context.Background())
		}
		conn.Release()
	}, true, nil
}
