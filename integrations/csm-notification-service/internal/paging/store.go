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

package paging

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
	// caseAssigneePrefix holds a customer case's current assignee, one key per
	// case, shared by both ladders. The case events that start a chain carry
	// no assignee, and a chain needs it twice: a case already assigned when
	// it is raised stops on that engineer's public comment, and only the
	// assignee's comment counts. Written on every case.assigned.
	caseAssigneePrefix = "incident:escalation:case:assignee:"
	// caseAssigneeTTL outlives any case worth paging about; an expired key
	// only means the next chain waits for an assignment it already had.
	caseAssigneeTTL = 180 * 24 * time.Hour
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
	// transient failure (network, 5xx, 429) is not recorded; its wake entry
	// stays and the next tick retries it. Without this a single bad number on
	// a roster was retried every tick for the ladder's whole life, and since
	// that call was never "placed", the ladder could never complete.
	Failed []string `json:"failed,omitempty"`
	// CardPending is parallel to Plan.Calls too, and set only on a rung that
	// both rang and posted: CardPending[i] is true once call i has its outcome
	// (placed, or given up on) while its chat card failed transiently and is
	// still owed. The wake entry stays and the next tick posts the card alone:
	// re-dialling somebody because a webhook blinked is the wrong way round.
	// Absent, as on every ladder stored before it existed, nothing is owed.
	CardPending []bool `json:"cardPending,omitempty"`
	// Cancelled is set when an acknowledgement arrived, before the remaining
	// wake entries are dropped, so a retry of the cancellation knows the
	// dropping half already happened.
	Cancelled *time.Time `json:"cancelled,omitempty"`
	// AssigneeName is who the stopping incident.assigned named, for the SRE space's "Incident Acknowledged." card.
	AssigneeName string `json:"assigneeName,omitempty"`
	// SawStateChange and SawPublicComment remember which acknowledgement
	// gestures have arrived so far.
	//
	// They exist because acknowledgement takes BOTH -- the incident moved to
	// Work In Progress AND a public comment -- and the two arrive as separate
	// events, in either order, minutes apart. A ladder therefore has to
	// remember a half-acknowledgement across ticks and restarts; holding it
	// only in memory would let the first gesture be forgotten and the ladder
	// keep climbing past a responder who had already answered.
	SawStateChange   bool `json:"sawStateChange,omitempty"`
	SawPublicComment bool `json:"sawPublicComment,omitempty"`
	// SawAssigned and SawAssigneeComment are a customer case's two gestures:
	// an engineer assigned (or already assigned when the chain started) and a
	// public comment by that engineer after the chain started.
	SawAssigned        bool `json:"sawAssigned,omitempty"`
	SawAssigneeComment bool `json:"sawAssigneeComment,omitempty"`
	// CommentAuthors are support engineers who posted a public comment on the
	// case since the chain started, kept so a comment written just before the
	// author was assigned still counts once they are.
	CommentAuthors []string `json:"commentAuthors,omitempty"`
	// CancelReason names which of section 3.0's two acknowledgement gestures
	// stopped the ladder, stored alongside Cancelled so a retried cancellation
	// writes the same summary the first attempt would have.
	CancelReason string `json:"cancelReason,omitempty"`
	// EmailSent is parallel to Plan.Emails: true once that rung's email has
	// been sent, or given up on. Absent on a ladder with no rung emails.
	EmailSent []bool `json:"emailSent,omitempty"`
}

// AllSettled reports whether every call in the plan has reached an outcome —
// dialled, or given up on — and every rung email has gone. It is what decides
// the ladder has run its course.
func (s LadderState) AllSettled() bool {
	for i := range s.Placed {
		if s.owed(i) {
			return false
		}
	}
	for i := range s.Plan.Emails {
		if !s.emailDone(i) {
			return false
		}
	}
	return true
}

// owed reports whether call i still has something to deliver: the call
// itself, or the card it is owed (CardPending).
func (s LadderState) owed(i int) bool {
	return (!s.Placed[i] && s.failure(i) == "") || s.cardPending(i)
}

// cardPending reports whether call i is still owed its chat card.
func (s LadderState) cardPending(i int) bool {
	return i < len(s.CardPending) && s.CardPending[i]
}

// setCardPending records whether call i is still owed its card. Clearing one
// never recorded writes nothing, so a ladder that never owed a card stores
// exactly what it did before CardPending existed.
func (s *LadderState) setCardPending(i int, owed bool) {
	if !owed && i >= len(s.CardPending) {
		return
	}
	if len(s.CardPending) < len(s.Placed) {
		grown := make([]bool, len(s.Placed))
		copy(grown, s.CardPending)
		s.CardPending = grown
	}
	s.CardPending[i] = owed
}

// emailDone reports whether rung email i has been sent or given up on.
func (s LadderState) emailDone(i int) bool {
	return i < len(s.EmailSent) && s.EmailSent[i]
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
	// wake and state are this store's own keys. The CRE ladder keeps the
	// original ones, so ladders stored before the SRE ladder existed are
	// still found; see ForLadder.
	wake, state string
}

// NewStore constructs a Store. Connecting is lazy, matching
// slaengine.NewStore and every other lazy-connect client here.
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb, wake: wakeKey, state: statePrefix}
}

// ForLadder returns a store over the same Redis whose ladder state and wake
// index belong to one ladder.
//
// A P0 incident climbs both ladders at once. Sharing one namespace would make
// them one ladder -- the second SETNX would find the first one's state and
// decide it was a redelivery -- and sharing one wake index would let each
// engine's tick place the other's calls through its own channel. The call
// history (MarkCalled) stays shared: somebody reached is reached.
func (s *Store) ForLadder(l Ladder) *Store {
	if l != LadderSRE {
		return s
	}
	return &Store{rdb: s.rdb, wake: "incident:escalation:sre:wake", state: "incident:escalation:sre:state:"}
}

func (s *Store) stateKey(incidentID string) string { return s.state + incidentID }

// CaseAssignee returns the case's current assignee email, or "" when none is
// known.
func (s *Store) CaseAssignee(ctx context.Context, caseID string) (string, error) {
	v, err := s.rdb.Get(ctx, caseAssigneePrefix+caseID).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return v, err
}

// ClaimCall takes one due call for this replica; see ladderStore.
func (s *Store) ClaimCall(ctx context.Context, member string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, s.wake+":claim:"+member, "1", ttl).Result()
}

// ReleaseCall gives a claimed call back.
func (s *Store) ReleaseCall(ctx context.Context, member string) error {
	return s.rdb.Del(ctx, s.wake+":claim:"+member).Err()
}

// SetCaseAssignee records the case's current assignee.
func (s *Store) SetCaseAssignee(ctx context.Context, caseID, email string) error {
	return s.rdb.Set(ctx, caseAssigneePrefix+caseID, email, caseAssigneeTTL).Err()
}

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
	return s.rdb.SetNX(ctx, s.stateKey(incidentID), body, stateTTL).Result()
}

// Save overwrites the ladder state unconditionally — used both to record a
// placed call and to replace a running ladder on a priority elevation.
func (s *Store) Save(ctx context.Context, incidentID string, st LadderState) error {
	body, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("escalation: encode ladder state: %w", err)
	}
	return s.rdb.Set(ctx, s.stateKey(incidentID), body, stateTTL).Err()
}

// Get loads a ladder. The second return is false when no ladder is running for
// this incident, which is a normal outcome, not an error — an acknowledgement
// for an incident that never had a ladder is the common case.
func (s *Store) Get(ctx context.Context, incidentID string) (LadderState, bool, error) {
	body, err := s.rdb.Get(ctx, s.stateKey(incidentID)).Bytes()
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
	return s.rdb.Del(ctx, s.stateKey(incidentID)).Err()
}

// AddWake schedules call index at the given time.
func (s *Store) AddWake(ctx context.Context, member string, at time.Time) error {
	return s.rdb.ZAdd(ctx, s.wake, redis.Z{Score: float64(at.Unix()), Member: member}).Err()
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
	return s.rdb.ZRem(ctx, s.wake, args...).Err()
}

// DueMembers returns every member whose due time has passed.
func (s *Store) DueMembers(ctx context.Context, now time.Time) ([]string, error) {
	return s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     s.wake,
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

// lastCalledKey is a hash of recipient -> the last time this engine called
// them, across every ladder.
//
// It exists for one rule: the evening pairing calls the incident's own ABT
// member plus "one other member of that rota", and who that is must be the
// person who has gone longest without a call. Spreading that load is the point
// of the rule -- picking the first name in a stable sort would put the same
// person on every out-of-hours incident.
//
// Keyed by email because that is what a Recipient is identified by everywhere
// else in this package. It never leaves Redis and is never logged.
const lastCalledKey = "incident:escalation:lastcalled"

// lastCalledTTL expires the whole hash if the engine stops running. Long
// enough that a quiet fortnight does not reset everyone's history, short
// enough that a decommissioned deployment does not leave it behind forever.
const lastCalledTTL = 90 * 24 * time.Hour

// MarkCalled records that somebody was called, for the fairness rule above.
//
// Best-effort by contract: the caller logs and carries on. Losing one entry
// makes the next pairing slightly less fair, which is not worth failing a page
// over.
func (s *Store) MarkCalled(ctx context.Context, email string, at time.Time) error {
	if email == "" {
		return nil
	}
	if err := s.rdb.HSet(ctx, lastCalledKey, email, at.UTC().Format(time.RFC3339)).Err(); err != nil {
		return err
	}
	return s.rdb.Expire(ctx, lastCalledKey, lastCalledTTL).Err()
}

// LastCalled answers when each of these people was last called. Somebody with
// no entry is absent from the map, which the caller reads as "never", and
// therefore as the longest wait of all.
func (s *Store) LastCalled(ctx context.Context, emails []string) (map[string]time.Time, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	vals, err := s.rdb.HMGet(ctx, lastCalledKey, emails...).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(emails))
	for i, v := range vals {
		raw, ok := v.(string)
		if !ok || raw == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			// A malformed entry is treated as no entry: the person simply
			// sorts as never called, which is the safe direction.
			continue
		}
		out[emails[i]] = t
	}
	return out, nil
}

// The Special Ops (SME) page's keys (sme.go). Shared by nothing else, and not
// per ladder: only the SRE engine places an SME page.
const (
	// smePagePrefix + "<incidentId>:<smeTeam>" is one open SME page, SETNX'd
	// so a second alert for the same team while it is open is ignored. The
	// value is the alert's changedOn.
	smePagePrefix = "incident:escalation:sme:page:"
	// smeOpenPrefix + "<incidentId>" is the set of SME teams with an open
	// page on the incident, so an assignment can close them all.
	smeOpenPrefix = "incident:escalation:sme:open:"
	// smeClosedPrefix + "<incidentId>" is when the latest assignment was
	// made (or the latest changedOn among the pages it closed, if later). An
	// alert timed at or before it has been answered, whichever was handled
	// first.
	smeClosedPrefix = "incident:escalation:sme:closed:"
	// smeClosedTTL outlives any redelivery or dead-letter retry.
	smeClosedTTL = 7 * 24 * time.Hour
)

func smePageKey(incidentID, team string) string { return smePagePrefix + incidentID + ":" + team }

// OpenSMEPage opens the incident's page for one SME team, reporting false when
// one is already open.
func (s *Store) OpenSMEPage(ctx context.Context, incidentID, team string, changedOn time.Time, ttl time.Duration) (bool, error) {
	opened, err := s.rdb.SetNX(ctx, smePageKey(incidentID, team), changedOn.UTC().Format(time.RFC3339Nano), ttl).Result()
	if err != nil || !opened {
		return opened, err
	}
	open := smeOpenPrefix + incidentID
	if err := s.rdb.SAdd(ctx, open, team).Err(); err != nil {
		return true, err
	}
	return true, s.rdb.Expire(ctx, open, ttl).Err()
}

// CloseSMEPage drops one team's page, for an alert that paged nobody.
func (s *Store) CloseSMEPage(ctx context.Context, incidentID, team string) error {
	if err := s.rdb.Del(ctx, smePageKey(incidentID, team)).Err(); err != nil {
		return err
	}
	return s.rdb.SRem(ctx, smeOpenPrefix+incidentID, team).Err()
}

// CloseSMEPages records an assignment made at answeredAt: the SME pages on
// the incident raised at or before it are answered. It closes those open now,
// keeping any raised after it (an escalation after this assignment), and
// remembers the time, so an alert raised before it but handled after it --
// or a replay of a closed one -- pages nobody. Reports the SME teams whose
// pages it closed, so their ladders can be stopped.
func (s *Store) CloseSMEPages(ctx context.Context, incidentID string, answeredAt time.Time) ([]string, error) {
	latest, err := s.SMEClosedThrough(ctx, incidentID)
	if err != nil {
		return nil, err
	}
	if answeredAt.After(latest) {
		latest = answeredAt
	}
	open := smeOpenPrefix + incidentID
	teams, err := s.rdb.SMembers(ctx, open).Result()
	if err != nil {
		return nil, err
	}
	var keys, closedTeams, answered []string
	for _, team := range teams {
		key := smePageKey(incidentID, team)
		raw, err := s.rdb.Get(ctx, key).Result()
		if errors.Is(err, redis.Nil) {
			closedTeams = append(closedTeams, team)
			continue
		}
		if err != nil {
			return nil, err
		}
		if at, perr := time.Parse(time.RFC3339Nano, raw); perr == nil && at.After(answeredAt) {
			continue // raised after this assignment: still open
		}
		keys = append(keys, key)
		closedTeams = append(closedTeams, team)
		answered = append(answered, team)
	}
	if !latest.IsZero() {
		if err := s.rdb.Set(ctx, smeClosedPrefix+incidentID, latest.UTC().Format(time.RFC3339Nano), smeClosedTTL).Err(); err != nil {
			return nil, err
		}
	}
	if len(keys) > 0 {
		if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
			return nil, err
		}
	}
	if len(closedTeams) > 0 {
		members := make([]any, len(closedTeams))
		for i, t := range closedTeams {
			members[i] = t
		}
		if err := s.rdb.SRem(ctx, open, members...).Err(); err != nil {
			return nil, err
		}
	}
	return answered, nil
}

// SMEClosedThrough is the time through which the incident's SME alerts have
// been answered by an assignment, zero when none.
func (s *Store) SMEClosedThrough(ctx context.Context, incidentID string) (time.Time, error) {
	raw, err := s.rdb.Get(ctx, smeClosedPrefix+incidentID).Result()
	if errors.Is(err, redis.Nil) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		// Unreadable is treated as absent: a later alert still pages.
		return time.Time{}, nil
	}
	return at, nil
}

// testCallClaimPrefix + "<userId>" holds one paging-number test call per
// person at a time (testcall.go).
const testCallClaimPrefix = "paging:testcall:"

// ClaimTestCall takes the person's test-call slot for ttl, reporting false
// while a recent one still holds it.
func (s *Store) ClaimTestCall(ctx context.Context, userID string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, testCallClaimPrefix+userID, "1", ttl).Result()
}
