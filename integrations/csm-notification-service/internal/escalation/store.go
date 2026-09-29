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

package escalation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis keys. Same shape as internal/slaengine's own wake index (one ZSET for
// the whole engine, scanned in a single round trip per tick), plus a JSON
// document per running ladder — this engine needs the plan itself to survive a
// restart, which the SLA engine gets from entity-service instead.
const (
	// wakeKey is the scheduling index: member "<incidentId>|<callIndex>",
	// score = the Unix timestamp that call is due at.
	wakeKey = "incident:escalation:wake"
	// statePrefix namespaces one LadderState document per incident.
	statePrefix = "incident:escalation:state:"
	// stateTTL is a backstop against leaking a ladder whose completion or
	// cancellation never ran. It must comfortably exceed the longest possible
	// ladder — P4's is under two hours (see TimeToFinalLevel) — so it never
	// expires one that is still live.
	stateTTL = 7 * 24 * time.Hour
)

// LadderState is everything needed to resume a running ladder after a restart:
// the expanded plan, which of its calls have already been placed, and whether
// an acknowledgement has stopped it.
type LadderState struct {
	Plan Plan `json:"plan"`
	// Placed is parallel to Plan.Calls: Placed[i] is true once call i has
	// actually been dialled, so a redelivered wake entry cannot call the same
	// person twice for the same attempt.
	Placed []bool `json:"placed"`
	// Failed is parallel to Plan.Calls too: a non-empty Failed[i] is the
	// reason call i was given up on rather than retried. Only a PERMANENT
	// failure lands here — the call provider rejecting the request itself
	// (a 4xx: an invalid or unverified number, a malformed document). A
	// transient failure (network, 5xx) is not recorded at all; its wake entry
	// stays and the next tick retries it. Without this a single bad number on
	// a roster was retried every tick for the ladder's whole life, and since
	// that call was never "placed", the ladder could never complete.
	Failed []string `json:"failed,omitempty"`
	// Cancelled is set when an acknowledgement arrived, before the remaining
	// wake entries are dropped, so a retry of the cancellation knows the
	// dropping half already happened.
	Cancelled *time.Time `json:"cancelled,omitempty"`
	// CancelReason names which of section 3.0's two acknowledgement gestures
	// stopped the ladder, stored alongside Cancelled so a retried cancellation
	// writes the same summary the first attempt would have.
	CancelReason string `json:"cancelReason,omitempty"`
}

// AllSettled reports whether every call in the plan has reached an outcome —
// dialled, or given up on. It is what decides the ladder has run its course.
func (s LadderState) AllSettled() bool {
	for i, done := range s.Placed {
		if !done && s.failure(i) == "" {
			return false
		}
	}
	return true
}

// failure is the recorded permanent-failure reason for call i, or "".
func (s LadderState) failure(i int) string {
	if i < len(s.Failed) {
		return s.Failed[i]
	}
	return ""
}

// setFailure records a permanent failure for call i.
func (s *LadderState) setFailure(i int, reason string) {
	if len(s.Failed) < len(s.Placed) {
		grown := make([]string, len(s.Placed))
		copy(grown, s.Failed)
		s.Failed = grown
	}
	s.Failed[i] = reason
}

// PlacedCount is how many calls have actually been dialled.
func (s LadderState) PlacedCount() int {
	n := 0
	for _, done := range s.Placed {
		if done {
			n++
		}
	}
	return n
}

// ReachedLevel is the highest rung this ladder actually got to, which is what
// says how far an incident escalated before somebody picked it up. Returns
// "NONE" when nothing has been dialled yet.
func (s LadderState) ReachedLevel() string {
	reached := "NONE"
	for i, done := range s.Placed {
		if done && i < len(s.Plan.Calls) {
			reached = s.Plan.Calls[i].Level.String()
		}
	}
	return reached
}

// Store is the Redis-backed ladder store and wake index.
type Store struct {
	rdb *redis.Client
}

// NewStore constructs a Store. Connecting is lazy, matching
// slaengine.NewWakeIndex and every other lazy-connect client here.
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func stateKey(incidentID string) string { return statePrefix + incidentID }

// Create stores a new ladder only if none is running for this incident, and
// reports whether it actually created one.
//
// SETNX, not SET, is what makes a redelivered incident.created harmless: Kafka
// delivery is at-least-once, and restarting a ladder on a redelivery would
// re-dial everyone from LEVEL_0. The engine treats "not created" as "already
// running, nothing to do".
func (s *Store) Create(ctx context.Context, incidentID string, st LadderState) (bool, error) {
	body, err := json.Marshal(st)
	if err != nil {
		return false, fmt.Errorf("escalation: encode ladder state: %w", err)
	}
	return s.rdb.SetNX(ctx, stateKey(incidentID), body, stateTTL).Result()
}

// Save overwrites the ladder state unconditionally — used both to record a
// placed call and to replace a running ladder on a priority elevation.
func (s *Store) Save(ctx context.Context, incidentID string, st LadderState) error {
	body, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("escalation: encode ladder state: %w", err)
	}
	return s.rdb.Set(ctx, stateKey(incidentID), body, stateTTL).Err()
}

// Get loads a ladder. The second return is false when no ladder is running for
// this incident, which is a normal outcome, not an error — an acknowledgement
// for an incident that never had a ladder is the common case.
func (s *Store) Get(ctx context.Context, incidentID string) (LadderState, bool, error) {
	body, err := s.rdb.Get(ctx, stateKey(incidentID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return LadderState{}, false, nil
	}
	if err != nil {
		return LadderState{}, false, err
	}
	var st LadderState
	if err := json.Unmarshal(body, &st); err != nil {
		return LadderState{}, false, fmt.Errorf("escalation: decode ladder state: %w", err)
	}
	return st, true, nil
}

// Delete drops a ladder's state, once it has either run out or been
// acknowledged and its work note written.
func (s *Store) Delete(ctx context.Context, incidentID string) error {
	return s.rdb.Del(ctx, stateKey(incidentID)).Err()
}

// AddWake schedules call index at the given time.
func (s *Store) AddWake(ctx context.Context, member string, at time.Time) error {
	return s.rdb.ZAdd(ctx, wakeKey, redis.Z{Score: float64(at.Unix()), Member: member}).Err()
}

// RemoveWakes drops the given members. Variadic so cancelling a ladder retires
// every remaining call in one round trip.
func (s *Store) RemoveWakes(ctx context.Context, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	args := make([]any, len(members))
	for i, m := range members {
		args[i] = m
	}
	return s.rdb.ZRem(ctx, wakeKey, args...).Err()
}

// DueMembers returns every member whose due time has passed.
func (s *Store) DueMembers(ctx context.Context, now time.Time) ([]string, error) {
	return s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     wakeKey,
		Start:   0,
		Stop:    now.Unix(),
		ByScore: true,
	}).Result()
}

// wakeMember encodes one scheduled call. The index is the call's position in
// Plan.Calls, which is what makes a wake entry resolvable back to a specific
// recipient and attempt without duplicating any of that in the member string.
func wakeMember(incidentID string, index int) string {
	return incidentID + "|" + strconv.Itoa(index)
}

// parseWakeMember is wakeMember's inverse. An incident id containing "|" would
// break this, which is why the index is appended last and split from the
// right.
func parseWakeMember(member string) (incidentID string, index int, ok bool) {
	sep := strings.LastIndex(member, "|")
	if sep <= 0 {
		return "", 0, false
	}
	index, err := strconv.Atoi(member[sep+1:])
	if err != nil || index < 0 {
		return "", 0, false
	}
	return member[:sep], index, true
}
