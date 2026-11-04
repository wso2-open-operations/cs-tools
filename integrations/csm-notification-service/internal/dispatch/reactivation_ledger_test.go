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
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestRedisReactivationLedger runs the ledger's real Redis calls (SET NX,
// GET, the compare-and-delete script, expiry) against miniredis, so CI covers
// them without a Redis server: only one reservation wins, a released one can
// be taken again, a reservation is released only by the attempt holding it,
// an expired one can be taken, and a sent marker is never released.
func TestRedisReactivationLedger(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	ctx := context.Background()
	l := NewRedisReactivationLedger(rdb)
	const key = "project-1/2026-10-29T09:30:00Z"

	first, sent, err := l.Reserve(ctx, key)
	if err != nil || first == "" || sent {
		t.Fatalf("first Reserve = %q %v %v", first, sent, err)
	}
	if tok, sent, err := l.Reserve(ctx, key); err != nil || tok != "" || sent {
		t.Errorf("second Reserve while sending = %q %v %v, want taken, not sent", tok, sent, err)
	}
	// Another attempt's token releases nothing.
	if err := l.Release(ctx, key, "not-mine"); err != nil {
		t.Fatal(err)
	}
	if !mr.Exists(reactivationKeyPrefix + key) {
		t.Fatal("a release with another attempt's token removed the reservation")
	}
	if err := l.Release(ctx, key, first); err != nil {
		t.Fatal(err)
	}
	second, _, err := l.Reserve(ctx, key)
	if err != nil || second == "" || second == first {
		t.Fatalf("Reserve after Release = %q %v, want a new reservation", second, err)
	}

	// The reservation expires on its own; the late release of the attempt
	// that held it must not remove the next attempt's reservation.
	mr.FastForward(reactivationSendingTTL + 1)
	third, _, err := l.Reserve(ctx, key)
	if err != nil || third == "" {
		t.Fatalf("Reserve after expiry = %q %v, want reserved", third, err)
	}
	if err := l.Release(ctx, key, second); err != nil {
		t.Fatal(err)
	}
	if v, _ := mr.Get(reactivationKeyPrefix + key); v != reactivationSending+third {
		t.Errorf("after a stale release the value is %q, want the newer reservation kept", v)
	}

	if err := l.MarkSent(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(ctx, key, third); err != nil {
		t.Fatal(err)
	}
	if tok, sent, err := l.Reserve(ctx, key); err != nil || tok != "" || !sent {
		t.Errorf("Reserve after MarkSent and Release = %q %v %v, want already sent", tok, sent, err)
	}
	if ttl := mr.TTL(reactivationKeyPrefix + key); ttl != reactivationSentTTL {
		t.Errorf("sent marker TTL = %s, want %s", ttl, reactivationSentTTL)
	}

	// Redis gone: every call errors (the handler then sends anyway).
	mr.Close()
	if _, _, err := l.Reserve(ctx, key); err == nil {
		t.Error("Reserve with Redis down: err = nil, want an error")
	}
}
