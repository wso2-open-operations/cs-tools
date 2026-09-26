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

package slaengine

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// tierKeyPrefix namespaces this engine's cursor keys — one plain string key
// per (caseID, clockType) pair, value = the highest tier (0/50/75/100) this
// engine has already alerted for (or seeded as a baseline — see Engine.
// processStatus in engine.go). Replaces the old wake-index ZSET design: with
// no due date of our own to schedule against anymore (see client.go's
// package doc comment for why), there's nothing to schedule — only a
// per-clock "have we already alerted for this" cursor to remember between
// polls. This cursor alone is only a hint for which tiers to check next —
// see tierClaimKeyPrefix below for what actually guards a tier from being
// alerted twice.
const tierKeyPrefix = "sla:tier:"

// tierClaimKeyPrefix namespaces one key per (caseID, clockType, tier) —
// claimed via ClaimTier's Redis SETNX before Engine.alertTier ever runs, so
// that if this service is ever deployed with more than one replica, only
// the replica that wins the SETNX race sends that tier's alert; a losing
// replica's ClaimTier call simply returns claimed=false and moves on.
// Mirrors the old sla_clocks design's own atomicity guarantee — there, the
// same role was played by entity-service's `UPDATE ... WHERE ... IS NULL`
// on a durable clock row — translated to Redis now that there's no such row
// to claim against. The plain cursor above alone is NOT enough for this:
// reading it and later writing it back are two separate round trips, and
// two replicas can both read the same stale value in between.
const tierClaimKeyPrefix = "sla:tier-claimed:"

// tierTTL bounds how long a clock's cursor survives with no further Tick
// touching it — entity-service's GET /sla-status only ever returns
// currently-active clocks, so a clock that completes/closes simply stops
// appearing and this engine has no explicit "clock finished" signal to react
// to. A generous TTL (refreshed on every Tick that still sees the clock —
// see setTier's caller) lets a stale cursor for a long-finished case expire
// on its own rather than accumulating in Redis forever; it comfortably
// outlives any realistic case lifetime, so it never fires while a clock is
// still genuinely active.
const tierTTL = 90 * 24 * time.Hour

// TierStore wraps the small set of Redis operations this engine needs —
// first Redis dependency in this repo (see this package's own CLAUDE.md
// section) — local for now (REDIS_ADDR), Azure Cache for Redis later via
// the same protocol/client, only a connection-string/TLS change.
type TierStore struct {
	rdb *redis.Client
}

// NewTierStore constructs a TierStore. Connecting is lazy — go-redis dials
// on first use, not here — so a wrong addr only surfaces as an error from
// the first call below, matching every other lazy-connect client in this
// repo (e.g. eventbus.NewProducer).
func NewTierStore(rdb *redis.Client) *TierStore {
	return &TierStore{rdb: rdb}
}

func tierKey(caseID, clockType string) string {
	return tierKeyPrefix + caseID + "|" + clockType
}

func tierClaimKey(caseID, clockType string, tier int) string {
	return tierClaimKeyPrefix + caseID + "|" + clockType + "|" + strconv.Itoa(tier)
}

// GetTier returns the last tier recorded for (caseID, clockType), and
// whether a cursor exists at all — found=false means this engine has never
// seen this clock before (or its cursor expired), which Engine.processStatus
// treats as "seed a baseline, don't alert" rather than "alert for
// everything up to its current tier."
func (s *TierStore) GetTier(ctx context.Context, caseID, clockType string) (tier int, found bool, err error) {
	val, err := s.rdb.Get(ctx, tierKey(caseID, clockType)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	tier, err = strconv.Atoi(val)
	if err != nil {
		return 0, false, err
	}
	return tier, true, nil
}

// SetTier records tier as the last tier reached for (caseID, clockType),
// refreshing tierTTL. Called both to seed/reseed a baseline (no alert sent)
// and to record a tier this call just alerted for.
func (s *TierStore) SetTier(ctx context.Context, caseID, clockType string, tier int) error {
	return s.rdb.Set(ctx, tierKey(caseID, clockType), strconv.Itoa(tier), tierTTL).Err()
}

// ClaimTier atomically claims (caseID, clockType, tier) via Redis SETNX —
// claimed=true means this call is the one that just claimed it and should
// go on to alert; claimed=false means some other call (a concurrent
// replica, or an earlier attempt on this same replica) already holds the
// claim, and this call must not alert again. See tierClaimKeyPrefix's own
// doc comment for why this exists separately from the plain cursor above.
func (s *TierStore) ClaimTier(ctx context.Context, caseID, clockType string, tier int) (claimed bool, err error) {
	return s.rdb.SetNX(ctx, tierClaimKey(caseID, clockType, tier), 1, tierTTL).Result()
}

// ReleaseTier gives back a claim made by ClaimTier — called when a claimed
// tier's alert fails to send (so a later tick, on this replica or another,
// can retry it instead of losing it for good), and when a tier regression
// (see Engine.processStatus) invalidates a claim from a now-superseded
// cycle.
func (s *TierStore) ReleaseTier(ctx context.Context, caseID, clockType string, tier int) error {
	return s.rdb.Del(ctx, tierClaimKey(caseID, clockType, tier)).Err()
}
