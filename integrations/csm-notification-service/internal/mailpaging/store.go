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
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// wakeKey is the one sorted set the ladder runs from: member = a
	// self-contained step (see step), score = when it is due.
	wakeKey = "mailpage:wake"
	// startedKeyPrefix marks an alert whose ladder was scheduled, so a
	// redelivered event does not start a second one.
	startedKeyPrefix = "mailpage:started:"
	// startedTTL outlives any realistic redelivery window.
	startedTTL = 7 * 24 * time.Hour
)

// Store is the Redis state behind the ladder. Every call touches exactly one
// key: a sharded Redis rejects a call across slots (CROSSSLOT).
type Store interface {
	// Started reports whether the alert's ladder was already scheduled.
	Started(ctx context.Context, alertID string) (bool, error)
	// MarkStarted records that it was.
	MarkStarted(ctx context.Context, alertID string) error
	// Schedule adds a step unless the identical step is already pending.
	Schedule(ctx context.Context, member string, at time.Time) error
	// Due returns up to limit steps due at or before now, including a leased
	// step whose lease has run out.
	Due(ctx context.Context, now time.Time, limit int64) ([]string, error)
	// Lease takes a due step for ttl and reports whether this caller took it:
	// the step stays stored, but is not due again until the lease runs out, so
	// only one replica works on it at a time and a worker that dies mid-step
	// leaves it to be picked up again.
	Lease(ctx context.Context, member string, now time.Time, ttl time.Duration) (bool, error)
	// Ack removes a step once its work is done.
	Ack(ctx context.Context, member string) error
}

// RedisStore is Store over Redis.
type RedisStore struct{ rdb *redis.Client }

// NewRedisStore wraps rdb.
func NewRedisStore(rdb *redis.Client) *RedisStore { return &RedisStore{rdb: rdb} }

// Started implements Store.
func (s *RedisStore) Started(ctx context.Context, alertID string) (bool, error) {
	n, err := s.rdb.Exists(ctx, startedKeyPrefix+alertID).Result()
	return n > 0, err
}

// MarkStarted implements Store.
func (s *RedisStore) MarkStarted(ctx context.Context, alertID string) error {
	return s.rdb.Set(ctx, startedKeyPrefix+alertID, "1", startedTTL).Err()
}

// Schedule implements Store.
func (s *RedisStore) Schedule(ctx context.Context, member string, at time.Time) error {
	return s.rdb.ZAddNX(ctx, wakeKey, redis.Z{Score: float64(at.Unix()), Member: member}).Err()
}

// Due implements Store.
func (s *RedisStore) Due(ctx context.Context, now time.Time, limit int64) ([]string, error) {
	return s.rdb.ZRangeByScore(ctx, wakeKey, &redis.ZRangeBy{
		Min: "-inf", Max: fmt.Sprint(now.Unix()), Count: limit,
	}).Result()
}

// leaseScript moves a due step's score to the lease expiry. It reads and
// writes only the wake set, so it is one atomic single-key operation: of two
// callers racing for the same step, the second sees a score in the future.
var leaseScript = redis.NewScript(`
local score = redis.call('ZSCORE', KEYS[1], ARGV[1])
if not score or tonumber(score) > tonumber(ARGV[2]) then
	return 0
end
redis.call('ZADD', KEYS[1], 'XX', ARGV[3], ARGV[1])
return 1
`)

// Lease implements Store.
func (s *RedisStore) Lease(ctx context.Context, member string, now time.Time, ttl time.Duration) (bool, error) {
	n, err := leaseScript.Run(ctx, s.rdb, []string{wakeKey}, member, now.Unix(), now.Add(ttl).Unix()).Int()
	return n == 1, err
}

// Ack implements Store.
func (s *RedisStore) Ack(ctx context.Context, member string) error {
	return s.rdb.ZRem(ctx, wakeKey, member).Err()
}
