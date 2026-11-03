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
	// Due returns up to limit steps due at or before now.
	Due(ctx context.Context, now time.Time, limit int64) ([]string, error)
	// Claim removes a due step and reports whether this caller removed it, so
	// only one replica acts on it.
	Claim(ctx context.Context, member string) (bool, error)
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

// Claim implements Store.
func (s *RedisStore) Claim(ctx context.Context, member string) (bool, error) {
	n, err := s.rdb.ZRem(ctx, wakeKey, member).Result()
	return n == 1, err
}
