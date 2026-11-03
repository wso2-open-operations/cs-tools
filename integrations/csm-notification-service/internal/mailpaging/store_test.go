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

package mailpaging

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStore connects like internal/slaengine's tests do (REDIS_URL, else
// REDIS_ADDR or localhost) and skips when there is no Redis.
func redisStore(t *testing.T) (*RedisStore, *redis.Client) {
	t.Helper()
	var rdb *redis.Client
	if url := os.Getenv("REDIS_URL"); url != "" {
		opts, err := redis.ParseURL(url)
		if err != nil {
			t.Skip("REDIS_URL does not parse; skipping")
		}
		rdb = redis.NewClient(opts)
	} else {
		addr := "localhost:6379"
		if a := os.Getenv("REDIS_ADDR"); a != "" {
			addr = a
		}
		rdb = redis.NewClient(&redis.Options{Addr: addr})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		t.Skip("no Redis; skipping")
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return NewRedisStore(rdb), rdb
}

// Every Store call must work on a Redis that shards keys, so each touches one
// key; this runs them for real.
func TestRedisStore_RoundTrip(t *testing.T) {
	s, rdb := redisStore(t)
	ctx := context.Background()
	id := "mailpaging-test-" + time.Now().Format("150405.000000000")
	member := `{"a":"` + id + `","r":"x","s":"","l":0}`
	defer func() {
		_ = rdb.ZRem(ctx, wakeKey, member)
		_ = rdb.Del(ctx, startedKeyPrefix+id)
	}()

	if ok, err := s.Started(ctx, id); err != nil || ok {
		t.Fatalf("Started() before = %v, %v; want false", ok, err)
	}
	if err := s.MarkStarted(ctx, id); err != nil {
		t.Fatalf("MarkStarted() error = %v", err)
	}
	if ok, err := s.Started(ctx, id); err != nil || !ok {
		t.Fatalf("Started() after = %v, %v; want true", ok, err)
	}

	past := time.Now().Add(-time.Minute)
	if err := s.Schedule(ctx, member, past); err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	// A second schedule of the same step must not move it.
	if err := s.Schedule(ctx, member, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("second Schedule() error = %v", err)
	}
	due, err := s.Due(ctx, time.Now(), 100)
	if err != nil {
		t.Fatalf("Due() error = %v", err)
	}
	found := false
	for _, m := range due {
		found = found || m == member
	}
	if !found {
		t.Fatalf("Due() = %v, want it to include the scheduled step", due)
	}
	if err := s.Ack(ctx, member); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
	if due, _ := s.Due(ctx, time.Now(), 100); contains(due, member) {
		t.Fatal("Due() still returns an acknowledged step")
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Replicas racing for one step: exactly one gets the lease, and the step is
// due again only once the lease has run out.
func TestRedisStore_LeaseIsExclusiveAndExpires(t *testing.T) {
	s, rdb := redisStore(t)
	ctx := context.Background()
	member := `{"a":"mailpaging-test-lease-` + time.Now().Format("150405.000000000") + `","r":"x","s":"","l":0}`
	defer func() { _ = rdb.ZRem(ctx, wakeKey, member) }()

	now := time.Now().Truncate(time.Second)
	if err := s.Schedule(ctx, member, now.Add(-time.Minute)); err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	const replicas = 8
	var wg sync.WaitGroup
	var won atomic.Int32
	for i := 0; i < replicas; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, err := s.Lease(ctx, member, now, 2*time.Minute); err == nil && ok {
				won.Add(1)
			}
		}()
	}
	wg.Wait()
	if won.Load() != 1 {
		t.Fatalf("%d replicas got the lease, want exactly 1", won.Load())
	}
	if due, _ := s.Due(ctx, now.Add(time.Minute), 100); contains(due, member) {
		t.Fatal("a leased step is due while its lease is held")
	}
	if due, _ := s.Due(ctx, now.Add(2*time.Minute), 100); !contains(due, member) {
		t.Fatal("a step whose lease ran out is not due again")
	}
	if ok, err := s.Lease(ctx, member, now.Add(2*time.Minute), 2*time.Minute); err != nil || !ok {
		t.Fatalf("Lease() after expiry = %v, %v; want true", ok, err)
	}
}
