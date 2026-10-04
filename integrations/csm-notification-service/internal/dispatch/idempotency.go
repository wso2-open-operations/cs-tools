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

package dispatch

// Per-record idempotency: the content-keyed claims that stop a retry from resending a channel that already succeeded.

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

// recordState is recordsMu/records' per-baseKey bookkeeping — see
// beginRecord's doc comment.
type recordState struct {
	refcount  int
	unhealthy bool
}

// beginRecord registers that a call is starting work on baseKey and returns
// a func to call exactly once, when that call is about to return, passing
// whether this call's own attempt hit any error. The returned func's result
// is true only for whichever call happens to be the last one still active
// for baseKey (refcount reaches 0) AND neither it nor any sibling that ran
// concurrently with it ever reported an error — i.e. only then is it
// provably safe to eagerly release baseKey's claimed channels before
// record.NoMoreRetries.
//
// This exists because a plain "am I the only one in flight right now"
// check, taken on its own right before returning, has a real gap: two
// concurrent calls finishing at close enough to the same instant can each
// observe the other still in flight and neither release anything, even
// though refcount is about to hit 0 — silently leaking that baseKey's
// claims forever (nothing will revisit them: NoMoreRetries only ever fires
// on a record that eventually exhausts every attempt, never one that
// succeeds first). Deciding under the same lock that performs the
// decrement — so exactly one caller ever observes "I just brought this to
// 0" — closes that gap.
//
// It also closes the specific bug CodeRabbit flagged in handleCaseCreated:
// a call that lost the claim race for every email group attempts nothing
// for email, so its own error is nil either way; the old release condition
// (chatOwned && no local errors) treated that as "fully done" and released
// chatKey while a different, still in-flight call was genuinely mid-
// SendEmail for that same record. Gating on refcount instead of on this
// call's own narrow view removes that false signal: a losing call's
// sibling is still counted as in flight until it actually returns.
func (d *Dispatcher) beginRecord(baseKey string) func(hadError bool) bool {
	d.recordsMu.Lock()
	st, ok := d.records[baseKey]
	if !ok {
		st = &recordState{}
		d.records[baseKey] = st
	}
	st.refcount++
	d.recordsMu.Unlock()

	return func(hadError bool) bool {
		d.recordsMu.Lock()
		defer d.recordsMu.Unlock()
		if hadError {
			st.unhealthy = true
		}
		st.refcount--
		if st.refcount > 0 {
			return false
		}
		delete(d.records, baseKey)
		return !st.unhealthy
	}
}

// claim atomically reserves key for the current attempt, returning true
// only if it wasn't already claimed — a single lock acquisition, unlike the
// separate check-then-later-mark pattern this replaced (alreadyDone,
// checked before an outbound call, followed by a separate markDone only
// after that call succeeds): two Handle calls racing on the same record
// (e.g. during a Kafka consumer-group rebalance transition — normally
// exclusive per partition, but not something this client's fencing is
// guaranteed to enforce down to the microsecond) could otherwise both
// observe an unclaimed key and both attempt the same outbound call before
// either one marks it done.
//
// Every call site must release a claim it doesn't end up keeping: call
// forget(key) immediately if the outbound call this claim was reserved for
// fails, so a genuine retry can reclaim it. A successful call leaves the
// claim in place — the caller's own full-record-succeeded-or-final-attempt
// check (see handleCaseCreated/handleIncidentCreated/sendPerGroup) is what
// eventually calls forget to release it for good.
func (d *Dispatcher) claim(key string) bool {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	if d.done[key] {
		return false
	}
	d.done[key] = true
	return true
}

// forget removes key — called either to release a claim whose outbound
// call just failed (see claim's doc comment), or once every channel for a
// record has fully succeeded (or record.NoMoreRetries is true), so the map
// doesn't hold onto a successful claim forever.
func (d *Dispatcher) forget(key string) {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	delete(d.done, key)
}

// recordBaseKey builds the per-event prefix every idempotency-tracked
// channel below keys off of. It hashes record.Value (the raw envelope
// bytes) rather than the record's Kafka coordinates (topic/partition/
// offset), which an earlier version of this used: a record that exhausts
// eventbus.Consumer's retries gets published to the dead-letter topic with
// the exact same Value but a brand new topic/partition/offset (see
// cmd/server/main.go's OnExhausted func, which republishes record.Key/
// record.Value unchanged) — keying off coordinates meant the DLQ consumer's
// very first attempt at a dead-lettered record computed a key that had
// never been claimed before, so an already-succeeded channel (e.g. a Chat
// alert sent on the main topic, before some other channel's persistent
// failure sent the record to the DLQ) got reclaimed and resent there. A
// content hash keys the same logical event identically everywhere it's
// delivered, main topic or DLQ.
func recordBaseKey(record eventbus.Record) string {
	sum := sha256.Sum256(record.Value)
	return hex.EncodeToString(sum[:])
}

// forgetEmailGroups releases sendPerGroup's per-group idempotency tracking
// for every caseLink in caseLinks. Every caller (handleCaseCreated/
// handleCommentAdded/handleStatusChanged/handleCaseAssigned) uses this two
// different ways depending on which release condition just triggered:
//
//   - record.NoMoreRetries (no further retry coming at all, ever) passes
//     every caseLink in the original groups map — safe precisely because
//     there is no future retry left to ever reclaim-and-resend a key
//     regardless of who currently holds it, the same reasoning
//     handleIncidentCreated's own NoMoreRetries branch uses for chatKey/
//     callKey directly.
//   - the whole call succeeding this round (sendErr == nil, or
//     chatOwned && len(errs) == 0 for handleCaseCreated) passes owned —
//     sendPerGroup's own return value, i.e. exactly the groups THIS call
//     actually claimed. This is the one that must stay ownership-gated: a
//     future retry IS still coming on this branch, so releasing a group
//     this call never claimed (some other, still in-flight call already
//     owns it — see claim's doc comment for when two Handle calls can race
//     on the very same record) would let that other call's key get
//     reclaimed and resent by the next retry while the original call is
//     still mid-send. This was a real, live-observed duplicate-email bug
//     before sendPerGroup returned owned at all (every group in the
//     caller's groups map was released together, regardless of which ones
//     this call actually sent).
//
// Ranging over a nil/empty caseLinks slice (e.g. handleCaseCreated's
// groupByLink failed this attempt, so sendPerGroup was never even called)
// is a safe no-op.
func (d *Dispatcher) forgetEmailGroups(baseKey string, caseLinks []string) {
	for _, caseLink := range caseLinks {
		d.forget(baseKey + "/email/" + caseLink)
	}
}
