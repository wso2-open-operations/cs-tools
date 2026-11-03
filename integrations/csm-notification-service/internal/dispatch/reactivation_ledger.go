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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ReactivationLedger is the durable record of "Back to Open state" emails
// already sent, so a redelivery of the same project.reactivated event (after
// a restart or a consumer-group rebalance, before the offset was committed)
// does not send the email again. A notice is identified by its project and
// its reactivatedOn: a later reopen of the same project is a new notice.
type ReactivationLedger interface {
	// Reserve takes the notice for this attempt and returns the attempt's
	// token (empty when not reserved). Empty with sent true: it was already
	// sent. Empty with sent false: another attempt is sending it right now
	// (retry later).
	Reserve(ctx context.Context, key string) (token string, sent bool, err error)
	// MarkSent records the notice as sent, after the email went out.
	MarkSent(ctx context.Context, key string) error
	// Release gives this attempt's reservation (token) back after a failed
	// send, so a retry can take it. A reservation another attempt took
	// since, or a sent marker, is left as it is.
	Release(ctx context.Context, key, token string) error
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
	reactivationSending      = "sending:"
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
// hold the reservation, across replicas and restarts. The reservation's
// value is "sending:" plus the attempt's random token, so Release can tell
// its own reservation from a later attempt's.
func (l *RedisReactivationLedger) Reserve(ctx context.Context, key string) (string, bool, error) {
	k := reactivationKeyPrefix + key
	token, err := newReservationToken()
	if err != nil {
		return "", false, fmt.Errorf("reactivation ledger: reserve: %w", err)
	}
	ok, err := l.rdb.SetNX(ctx, k, reactivationSending+token, reactivationSendingTTL).Result()
	if err != nil {
		return "", false, fmt.Errorf("reactivation ledger: reserve: %w", err)
	}
	if ok {
		return token, false, nil
	}
	v, err := l.rdb.Get(ctx, k).Result()
	if errors.Is(err, redis.Nil) {
		// Expired between the two calls: let the retry try again.
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reactivation ledger: read: %w", err)
	}
	return "", v == reactivationSent, nil
}

// newReservationToken is a random token naming one attempt's reservation.
func newReservationToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// releaseScript deletes the key only while it still holds this attempt's
// reservation, in one step: a GET then DEL could delete a reservation
// another attempt took after this one's expired.
var releaseScript = redis.NewScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end return 0`)

// MarkSent implements ReactivationLedger.
func (l *RedisReactivationLedger) MarkSent(ctx context.Context, key string) error {
	if err := l.rdb.Set(ctx, reactivationKeyPrefix+key, reactivationSent, reactivationSentTTL).Err(); err != nil {
		return fmt.Errorf("reactivation ledger: mark sent: %w", err)
	}
	return nil
}

// Release implements ReactivationLedger. Only this attempt's reservation is
// removed (releaseScript, compare and delete): a sent marker is never given
// back, and neither is a reservation another attempt holds, whatever the
// send and reservation timings are.
func (l *RedisReactivationLedger) Release(ctx context.Context, key, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	if err := releaseScript.Run(ctx, l.rdb, []string{reactivationKeyPrefix + key}, reactivationSending+token).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("reactivation ledger: release: %w", err)
	}
	return nil
}
