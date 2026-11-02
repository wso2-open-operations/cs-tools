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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package dispatch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestRedisReactivationLedger runs the ledger against a real Redis: only one
// reservation wins, a released one can be taken again, a sent marker is
// never released. Skipped unless a Redis is reachable (REDIS_URL, then
// REDIS_ADDR, then localhost:6379), like the paging store's tests:
//
//	docker run --rm -p 6379:6379 redis
//	go test ./internal/dispatch/ -run TestRedisReactivationLedger -v
func TestRedisReactivationLedger(t *testing.T) {
	var rdb *redis.Client
	addr := "localhost:6379"
	if url := os.Getenv("REDIS_URL"); url != "" {
		opts, err := redis.ParseURL(url)
		if err != nil {
			t.Skip("REDIS_URL is set but does not parse; skipping")
		}
		addr = opts.Addr
		rdb = redis.NewClient(opts)
	} else {
		if a := os.Getenv("REDIS_ADDR"); a != "" {
			addr = a
		}
		rdb = redis.NewClient(&redis.Options{Addr: addr})
	}
	defer func() { _ = rdb.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("no Redis at %s; skipping", addr)
	}
	key := "test/" + time.Now().UTC().Format(time.RFC3339Nano)
	defer rdb.Del(context.Background(), reactivationKeyPrefix+key)
	l := NewRedisReactivationLedger(rdb)

	if reserved, sent, err := l.Reserve(ctx, key); err != nil || !reserved || sent {
		t.Fatalf("first Reserve = %v %v %v", reserved, sent, err)
	}
	if reserved, sent, err := l.Reserve(ctx, key); err != nil || reserved || sent {
		t.Errorf("second Reserve while sending = %v %v %v, want taken, not sent", reserved, sent, err)
	}
	if err := l.Release(ctx, key); err != nil {
		t.Fatal(err)
	}
	if reserved, _, err := l.Reserve(ctx, key); err != nil || !reserved {
		t.Errorf("Reserve after Release = %v %v", reserved, err)
	}
	if err := l.MarkSent(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(ctx, key); err != nil {
		t.Fatal(err)
	}
	if reserved, sent, err := l.Reserve(ctx, key); err != nil || reserved || !sent {
		t.Errorf("Reserve after MarkSent and Release = %v %v %v, want already sent", reserved, sent, err)
	}
}
