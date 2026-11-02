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
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ReactivationLedger is the durable record of "Back to Open state" emails
// already sent, so a redelivery of the same project.reactivated event (after
// a restart or a consumer-group rebalance, before the offset was committed)
// does not send the email again. A notice is identified by its project and
// its reactivatedOn: a later reopen of the same project is a new notice.
type ReactivationLedger interface {
	// Reserve takes the notice for this attempt. reserved false with sent
	// true: it was already sent. reserved false with sent false: another
	// attempt is sending it right now (retry later).
	Reserve(ctx context.Context, key string) (reserved, sent bool, err error)
	// MarkSent records the notice as sent, after the email went out.
	MarkSent(ctx context.Context, key string) error
	// Release gives the reservation back after a failed send, so a retry can
	// take it.
	Release(ctx context.Context, key string) error
}

// Values and lifetimes of a Redis ledger entry. A reservation expires on its
// own after reactivationSendingTTL, so an attempt that died between Reserve
// and MarkSent cannot block the notice for ever; a sent marker is kept for
// reactivationSentTTL, far longer than any redelivery.
//
// The send is cut off after reactivationSendTimeout, well inside the
// reservation, so a live attempt never outlasts its own reservation. A
// delivery that finds a reservation held waits for it (polling every
// reactivationWaitInterval) instead of failing: the consumer's retries and
// the dead-letter topic's take seconds, so failing at once would exhaust
// them while the reservation of an attempt that crashed is still live, and
// the email would never be sent.
const (
	reactivationSending      = "sending"
	reactivationSent         = "sent"
	reactivationSendingTTL   = 2 * time.Minute
	reactivationSendTimeout  = time.Minute
	reactivationWaitInterval = 5 * time.Second
	reactivationSentTTL      = 30 * 24 * time.Hour
	reactivationKeyPrefix    = "csm-notification:project-reactivated:"
)

// RedisReactivationLedger is the ReactivationLedger over the service's Redis
// (REDIS_URL / REDIS_ADDR, shared with the SLA engine and paging).
type RedisReactivationLedger struct {
	rdb *redis.Client
}

// NewRedisReactivationLedger constructs the ledger over rdb.
func NewRedisReactivationLedger(rdb *redis.Client) *RedisReactivationLedger {
	return &RedisReactivationLedger{rdb: rdb}
}

// Reserve implements ReactivationLedger with SET NX: only one attempt can
// hold the reservation, across replicas and restarts.
func (l *RedisReactivationLedger) Reserve(ctx context.Context, key string) (bool, bool, error) {
	k := reactivationKeyPrefix + key
	ok, err := l.rdb.SetNX(ctx, k, reactivationSending, reactivationSendingTTL).Result()
	if err != nil {
		return false, false, fmt.Errorf("reactivation ledger: reserve: %w", err)
	}
	if ok {
		return true, false, nil
	}
	v, err := l.rdb.Get(ctx, k).Result()
	if errors.Is(err, redis.Nil) {
		// Expired between the two calls: let the retry try again.
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("reactivation ledger: read: %w", err)
	}
	return false, v == reactivationSent, nil
}

// MarkSent implements ReactivationLedger.
func (l *RedisReactivationLedger) MarkSent(ctx context.Context, key string) error {
	if err := l.rdb.Set(ctx, reactivationKeyPrefix+key, reactivationSent, reactivationSentTTL).Err(); err != nil {
		return fmt.Errorf("reactivation ledger: mark sent: %w", err)
	}
	return nil
}

// Release implements ReactivationLedger. Only a reservation is removed: a
// sent marker is never given back.
func (l *RedisReactivationLedger) Release(ctx context.Context, key string) error {
	k := reactivationKeyPrefix + key
	v, err := l.rdb.Get(ctx, k).Result()
	if errors.Is(err, redis.Nil) || (err == nil && v != reactivationSending) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reactivation ledger: release: %w", err)
	}
	if err := l.rdb.Del(ctx, k).Err(); err != nil {
		return fmt.Errorf("reactivation ledger: release: %w", err)
	}
	return nil
}
