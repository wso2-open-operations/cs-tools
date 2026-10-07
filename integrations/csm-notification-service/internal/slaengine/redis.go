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
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// wakeKey is the single Redis sorted-set key this engine uses as its
// scheduling index: member = "<caseId>|<clockType>|<tier>", score = the Unix
// timestamp that member becomes due at. One key for the whole engine (not
// one per case) — the ZRANGE ... BYSCORE query below scans the whole set in
// one round trip per tick regardless of how many clocks are registered.
// Ported from the pre-poll design's own WakeIndex (see this package's own
// CLAUDE.md section for the history) — ClockMeta/the alerted-tier cursor
// below are new, replacing that design's entity-service-backed durable
// clock row.
const wakeKey = "sla:wake"

// clockKeyPrefix namespaces one HASH per (caseID, clockType) pair, holding
// everything this engine needs to know about that clock: the Chat card's
// own display fields (set once at RegisterClocks, read back unchanged at
// alert time — there is no live entity-service lookup to refresh them from
// any more), the paused flag ApplyStateEffects toggles, and the
// alertedTier cursor Tick/CompleteResponseClock/ApplyStateEffects advance.
const clockKeyPrefix = "sla:clock:"

// clockTTL bounds how long a clock's hash survives with no further write
// touching it. This engine has no explicit "case closed for good, delete
// everything" signal of its own (ApplyStateEffects' CLOSED branch still
// writes a completion, which refreshes this same TTL) — a generous TTL,
// refreshed on every touch, lets a long-finished case's hash expire on its
// own rather than accumulating forever, the same reasoning the removed
// poll design's own tierTTL gave for its cursor keys.
const clockTTL = 90 * 24 * time.Hour

// tierClaimKeyPrefix namespaces one key per (caseID, clockType, tier,
// startedAt) — claimed via ClaimTier's Redis SETNX before Engine.alertTier
// ever runs, so that if this service is ever deployed with more than one
// replica, only the replica that wins the SETNX race sends that tier's
// alert. Ported from the removed poll design's own TierStore (same
// reasoning: two replicas racing on the same due wake member must not both
// alert), with one addition: startedAt is part of the key specifically so a
// claim made under an OLD incarnation of this (caseID, clockType) pair (see
// setClockScript's own doc comment on what "incarnation" means here) can
// never suppress the alert for a genuinely new incarnation's own tier --
// without it, a stale, unexpired claim key from before a severity revision
// would make ClaimTier report claimed=false for the new clock's first real
// crossing, and processDueMember would then silently drop that wake entry
// forever, same as the matching alertedTier bug this change's sibling fix
// addresses.
const tierClaimKeyPrefix = "sla:tier-claimed:"

// ClockMeta is one (caseID, clockType) clock's full Redis-held state —
// RegisterClocks writes CaseNumber..StartedAt once; Paused/AlertedTier are
// mutated by ApplyStateEffects/CompleteResponseClock/Tick afterward.
type ClockMeta struct {
	CaseNumber string
	WSO2CaseID string
	CaseTitle  string
	CaseType   string
	Product    string
	Team       string
	Priority   string
	// State is the case's own display-label status (e.g. "Work In
	// Progress"), refreshed by ApplyStateEffects on every case.status_changed
	// — "" until the first status change, since case.created carries no
	// status field of its own (a brand-new case is always freshly Open).
	State     string
	StartedAt time.Time
	Paused    bool
	// AlertedTier is the highest tier (0/50/75/100) already alerted for, OR
	// force-completed via CompleteResponseClock/ApplyStateEffects' CLOSED
	// branch — Tick drops a due wake member outright once its own tier is
	// at or below this value, the same "already handled, don't re-alert"
	// cursor the removed poll design's own TierStore kept, just stored
	// alongside the clock's own metadata instead of as a separate key.
	AlertedTier int
}

// Store wraps every Redis operation this engine needs — first Redis
// dependency in this repo (see this package's own CLAUDE.md section),
// local for now (REDIS_ADDR), Azure Cache for Redis later via the same
// protocol/client, only a connection-string/TLS change.
type Store struct {
	rdb *redis.Client
}

// NewStore constructs a Store. Connecting is lazy — go-redis dials on first
// use, not here — so a wrong addr only surfaces as an error from the first
// call below, matching every other lazy-connect client in this repo (e.g.
// eventbus.NewProducer).
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func clockKey(caseID, clockType string) string {
	return clockKeyPrefix + caseID + "|" + clockType
}

func tierClaimKey(caseID, clockType string, tier int, startedAt time.Time) string {
	// UnixNano, not Unix -- see SetClock's own doc comment on why whole
	// seconds aren't enough to tell two genuinely different incarnations
	// apart.
	return tierClaimKeyPrefix + caseID + "|" + clockType + "|" + strconv.Itoa(tier) + "|" + strconv.FormatInt(startedAt.UnixNano(), 10)
}

// AddWake schedules member to become due at at.
func (s *Store) AddWake(ctx context.Context, member string, at time.Time) error {
	return s.rdb.ZAdd(ctx, wakeKey, redis.Z{Score: float64(at.Unix()), Member: member}).Err()
}

// RemoveWake drops member from the index — Tick's processDueMember calls
// this exactly once per member it examines, regardless of outcome (alerted,
// already handled, paused, or malformed): see that function's own doc
// comment for why a member must never be left to be re-examined on every
// future tick forever once its due time has passed.
func (s *Store) RemoveWake(ctx context.Context, member string) error {
	return s.rdb.ZRem(ctx, wakeKey, member).Err()
}

// DueMembers returns every member whose score (epoch seconds) is <= now.
func (s *Store) DueMembers(ctx context.Context, now time.Time) ([]string, error) {
	return s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     wakeKey,
		Start:   0,
		Stop:    now.Unix(),
		ByScore: true,
	}).Result()
}

// setClockScript always (re)writes the registration-time display fields
// (safe to refresh on every call -- they're one-time facts from
// case.created, never mutated by any later event), but only ever
// INITIALIZES state/paused/alertedTier via HSETNX, never overwrites them.
//
// This matters because RegisterClocks -- and therefore SetClock -- is not
// actually called exactly once per case: dispatch.handleCaseCreated's own
// email/Chat reactions can fail and retry the whole record, and a
// dead-lettered record gets a fresh retry pass on the DLQ consumer under
// the exact same case.created payload (see recordBaseKey's own doc
// comment in internal/dispatch) -- both redeliver the identical
// case.created event to RegisterClocks again, potentially long after
// ApplyStateEffects/CompleteResponseClock have already paused or
// force-completed this clock in response to later events. A plain HSET of
// every field on that replay would silently reset alertedTier back to 0
// and paused back to false -- re-arming wake entries for a clock that was
// already genuinely finished, and firing a false breach alert the next
// time Tick finds one of them due.
//
// The one exception: a genuinely NEW incarnation of the same (caseID,
// clockType) pair -- entity-service's own ReviseCaseClocks cancels a case's
// existing clocks and registers entirely fresh ones (a new start_on, no
// relation to the old elapsed time) on a severity change, and
// Engine.Reconcile reads whichever row is currently active, so it can see a
// case's clock jump to a new startedAt with no event in between telling
// this engine so. Detected here by comparing the hash's current startedAt
// to the incoming one: a genuine change resets alertedTier to 0 (a
// replay of the SAME incarnation leaves it alone, via the HSETNX below,
// same as always) -- without this, a tier already alerted under the OLD
// incarnation would permanently block Tick from ever alerting the NEW
// incarnation's own tiers (processDueMember's own "AlertedTier >= tier"
// check has no notion of "that was a different clock"). See ClaimTier's
// own doc comment for the matching half of this fix -- a stale tier claim
// from the old incarnation needs the same treatment.
var setClockScript = redis.NewScript(`
local previousStartedAt = redis.call('HGET', KEYS[1], 'startedAt')
if previousStartedAt and previousStartedAt ~= ARGV[8] then
	redis.call('HSET', KEYS[1], 'alertedTier', '0')
end
redis.call('HSET', KEYS[1],
	'caseNumber', ARGV[1], 'wso2CaseId', ARGV[2], 'caseTitle', ARGV[3],
	'caseType', ARGV[4], 'product', ARGV[5], 'team', ARGV[6], 'priority', ARGV[7],
	'startedAt', ARGV[8])
redis.call('HSETNX', KEYS[1], 'state', ARGV[9])
redis.call('HSETNX', KEYS[1], 'paused', '0')
redis.call('HSETNX', KEYS[1], 'alertedTier', '0')
redis.call('EXPIRE', KEYS[1], ARGV[10])
return 1
`)

// SetClock writes a clock's display-field set, initializing
// state/paused/alertedTier only the first time this (caseID, clockType)
// pair is ever seen -- see setClockScript's own doc comment for why a
// later call (a retry/replay of the same case.created event) must never
// reset them.
//
// startedAt is stored as UnixNano, not Unix (whole seconds) -- a
// CodeRabbit-caught gap in an earlier version of the incarnation check
// below: entity-service's ReviseCaseClocks can register a replacement
// clock within the same wall-clock second as the one it replaces (a fast
// severity re-revision, or simply two clock types of the same case
// revised back-to-back), which a whole-seconds timestamp can't tell apart
// from the original -- the incarnation check would then wrongly treat a
// genuinely new clock as a replay, preserving a stale alertedTier/tier
// claim exactly like the bug this check exists to prevent. tierClaimKey
// below uses the same precision for the identical reason.
func (s *Store) SetClock(ctx context.Context, caseID, clockType string, meta ClockMeta) error {
	key := clockKey(caseID, clockType)
	return setClockScript.Run(ctx, s.rdb, []string{key},
		meta.CaseNumber, meta.WSO2CaseID, meta.CaseTitle, meta.CaseType,
		meta.Product, meta.Team, meta.Priority, meta.StartedAt.UnixNano(),
		meta.State, int(clockTTL.Seconds()),
	).Err()
}

// GetClock reads one clock's full state back. found=false means this
// engine has no record of this (caseID, clockType) pair at all — either it
// was never registered (e.g. a case that already existed before this
// feature deployed — see RegisterClocks' own doc comment on backfill), or
// its hash expired.
func (s *Store) GetClock(ctx context.Context, caseID, clockType string) (meta ClockMeta, found bool, err error) {
	res, err := s.rdb.HGetAll(ctx, clockKey(caseID, clockType)).Result()
	if err != nil {
		return ClockMeta{}, false, err
	}
	if len(res) == 0 {
		return ClockMeta{}, false, nil
	}
	meta = ClockMeta{
		CaseNumber: res["caseNumber"],
		WSO2CaseID: res["wso2CaseId"],
		CaseTitle:  res["caseTitle"],
		CaseType:   res["caseType"],
		Product:    res["product"],
		Team:       res["team"],
		Priority:   res["priority"],
		State:      res["state"],
		Paused:     res["paused"] == "1",
	}
	if v, err := strconv.ParseInt(res["startedAt"], 10, 64); err == nil {
		meta.StartedAt = time.Unix(0, v)
	}
	if v, err := strconv.Atoi(res["alertedTier"]); err == nil {
		meta.AlertedTier = v
	}
	return meta, true, nil
}

// SetPaused toggles one clock's paused flag — ApplyStateEffects' only write
// for the AWAITING_INFO/SOLUTION_PROPOSED/resume branches. A no-op (HSET
// creates a near-empty hash) against a clock this engine never registered —
// harmless, since without a RegisterClocks call there is also no wake entry
// for Tick to ever examine against it.
func (s *Store) SetPaused(ctx context.Context, caseID, clockType string, paused bool) error {
	key := clockKey(caseID, clockType)
	if err := s.rdb.HSet(ctx, key, "paused", boolString(paused)).Err(); err != nil {
		return err
	}
	return s.rdb.Expire(ctx, key, clockTTL).Err()
}

// SetState refreshes one clock's own display State field — called from
// ApplyStateEffects on every case.status_changed, so a breach alert fired
// later shows the case's current status, not a stale "" from registration.
func (s *Store) SetState(ctx context.Context, caseID, clockType, state string) error {
	key := clockKey(caseID, clockType)
	if err := s.rdb.HSet(ctx, key, "state", state).Err(); err != nil {
		return err
	}
	return s.rdb.Expire(ctx, key, clockTTL).Err()
}

// advanceAlertedTierScript atomically sets a clock hash's alertedTier field
// to ARGV[1] only if it is currently absent or lower than ARGV[1] — never
// moving it backward. Mirrors the removed poll design's own TierStore
// advanceTierScript exactly, just against a hash field instead of a plain
// string key — see that script's own doc comment (preserved in git
// history) for the full concurrency reasoning: two callers (a Tick claim
// and CompleteResponseClock/ApplyStateEffects' CLOSED branch, say) must
// never let whichever writes second silently move the cursor backward.
//
// ARGV[3], when non-empty, scopes this update to one clock incarnation --
// a CodeRabbit-caught race in an earlier version of this engine's
// reconciliation feature: a tick that read a clock's metadata (ClaimTier's
// own incarnation-scoped claim already prevents it from double-alerting
// under a DIFFERENT clock, but a claim is not enough on its own) can still
// race a concurrent registration of a REPLACEMENT clock (entity-service's
// ReviseCaseClocks on a severity change, observed via Reconcile) for the
// same (caseID, clockType) key -- if that replacement lands between this
// tick's claim and this call, an unscoped update here would advance the
// NEW clock's own, just-reset cursor using the OLD tick's tier, based on
// work done for a clock that no longer exists. Returns 0 (no update
// applied) when ARGV[3] is given and no longer matches the hash's current
// startedAt -- the caller must treat that as "this clock has moved on,
// don't touch its state any further" (see processDueMember's own call
// site). An empty ARGV[3] (CompleteResponseClock/ApplyStateEffects' CLOSED
// branch, which have no previously-read incarnation to compare against at
// all) always applies, exactly as before this check existed.
var advanceAlertedTierScript = redis.NewScript(`
if ARGV[3] ~= '' then
	local currentStartedAt = redis.call('HGET', KEYS[1], 'startedAt')
	if currentStartedAt and currentStartedAt ~= ARGV[3] then
		return 0
	end
end
local current = redis.call('HGET', KEYS[1], 'alertedTier')
local candidate = tonumber(ARGV[1])
if (not current) or (candidate > tonumber(current)) then
	redis.call('HSET', KEYS[1], 'alertedTier', ARGV[1])
end
redis.call('EXPIRE', KEYS[1], ARGV[2])
return 1
`)

// AdvanceAlertedTier atomically records tier as the highest tier
// alerted/completed for (caseID, clockType) — but only if the currently
// stored value is absent or lower. Used both by Tick (right after a
// successful alert) and by CompleteResponseClock/ApplyStateEffects' CLOSED
// branch (passing 100 to force-complete every tier at once, matching the
// removed pre-poll design's "mark all three tiers reached" semantics for an
// early completion).
//
// incarnation should be the clock's StartedAt as this caller itself last
// observed it (from GetClock) — Tick always has one; CompleteResponseClock/
// ApplyStateEffects pass the zero time.Time{}, meaning "apply regardless of
// whatever incarnation currently exists" (they're reacting to a live event
// about the case right now, not working from a possibly-stale snapshot, so
// there's nothing to scope against). applied=false means the given
// incarnation no longer matches — see advanceAlertedTierScript's own doc
// comment for why a caller must not then touch this clock's wake entry.
func (s *Store) AdvanceAlertedTier(ctx context.Context, caseID, clockType string, tier int, incarnation time.Time) (applied bool, err error) {
	var incarnationArg string
	if !incarnation.IsZero() {
		incarnationArg = strconv.FormatInt(incarnation.UnixNano(), 10)
	}
	res, err := advanceAlertedTierScript.Run(ctx, s.rdb, []string{clockKey(caseID, clockType)}, tier, int(clockTTL.Seconds()), incarnationArg).Result()
	if err != nil {
		return false, err
	}
	n, ok := res.(int64)
	return ok && n == 1, nil
}

// ClaimTier atomically claims (caseID, clockType, tier) under the clock's
// current incarnation (startedAt — see tierClaimKeyPrefix's own doc comment
// for why this is part of the key) via Redis SETNX — claimed=true means
// this call is the one that just claimed it and should go on to alert;
// claimed=false means some other call (a concurrent replica, an earlier
// attempt, or a stale claim from the SAME incarnation already handled)
// already holds the claim. Callers pass the clock's own meta.StartedAt
// (from GetClock), not a value they compute themselves, so a claim is
// always scoped to whichever incarnation that caller actually observed.
func (s *Store) ClaimTier(ctx context.Context, caseID, clockType string, tier int, startedAt time.Time) (claimed bool, err error) {
	return s.rdb.SetNX(ctx, tierClaimKey(caseID, clockType, tier, startedAt), 1, clockTTL).Result()
}

// ReleaseTier gives back a claim made by ClaimTier — called when a claimed
// tier's alert fails to send (the Kafka publish specifically — see
// Engine.alertTier's own doc comment for why a Chat-send failure doesn't
// trigger this), so a later tick can retry it instead of losing it for
// good. startedAt must be the same incarnation value the matching ClaimTier
// call used, or this deletes nothing (a different key).
func (s *Store) ReleaseTier(ctx context.Context, caseID, clockType string, tier int, startedAt time.Time) error {
	return s.rdb.Del(ctx, tierClaimKey(caseID, clockType, tier, startedAt)).Err()
}

func boolString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
