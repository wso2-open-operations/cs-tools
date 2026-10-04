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

// Package reportguard makes a report-style handler's send happen at most
// once per period, using nothing but the scheduled-task ledger that already
// exists.
//
// The problem it solves: a report handler sends its e-mail and returns; the
// only record that it happened is the engine's ledger Complete that
// follows. If that Complete fails, or the process is killed between the
// send and the Complete, the row is reclaimed after the orphan window and
// the identical report is mailed again.
//
// The guard records the send under a companion ledger row — task name
// "<task>.sent", same period key — claimed before the send and completed
// right after it. A re-run for the same period then finds that row already
// succeeded and skips the send. The ledger enforces "one succeeded row per
// task name and period" already, so no new protocol or endpoint is needed;
// the companion rows are ordinary rows that housekeeping deletes on the
// usual retention like any other.
//
// The window that remains: a crash after the send but before the
// companion Complete leaves that row claimed, and once it looks orphaned
// the next run sends again. That is a send-then-crash in a few-millisecond
// gap, rather than any failure of the main row's bookkeeping.
package reportguard

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

// Ledger is the subset of *ledger.Client the guard depends on.
type Ledger interface {
	Attempt(ctx context.Context, taskName string, periodKey time.Time, staleClaimAfter time.Duration) (ledger.Claim, error)
	Complete(ctx context.Context, id string, attemptCount int) error
	Fail(ctx context.Context, id string, attemptCount int, errMsg string, nextRetryOn time.Time) error
}

// MarkerSuffix is appended to a task's name to form its companion row name.
const MarkerSuffix = ".sent"

// recordTimeout bounds the companion Complete/Fail after the send, which
// run detached from the handler's cancellation for the same reason the
// engine's own record-back does.
const recordTimeout = 10 * time.Second

// Guard runs a send at most once per period for one task.
type Guard struct {
	ledger Ledger
	marker string
	// StaleClaimAfter is passed with the companion claim. Zero uses the
	// ledger's own default.
	StaleClaimAfter time.Duration
}

// New returns a Guard for taskName, recording under taskName+MarkerSuffix.
func New(l Ledger, taskName string) *Guard {
	return &Guard{ledger: l, marker: taskName + MarkerSuffix}
}

// Once runs send unless this period's send is already recorded.
//
//   - Period already recorded as sent: send is skipped, nil is returned.
//   - Companion row held by another live attempt: an error is returned and
//     nothing is sent, so the task retries rather than doubling up.
//   - Companion claim fails (ledger unreachable): an error is returned and
//     nothing is sent — a missed report retries next tick; a duplicate
//     cannot be taken back.
//   - send fails: the companion row is marked failed (immediately
//     retryable) and send's error is returned.
//   - send succeeds but the companion Complete fails: logged, nil returned.
//     The mail is out; failing the task would only make a retry resend it.
//
// If ctx carries no period (registry.WithPeriod — the engine always sets
// it) the guard cannot key the record and runs send unguarded.
func (g *Guard) Once(ctx context.Context, send func(ctx context.Context) error) error {
	period, ok := registry.PeriodFrom(ctx)
	if !ok {
		slog.WarnContext(ctx, "reportguard: no period on context; sending without a once-per-period guard", "marker", g.marker)
		return send(ctx)
	}

	claim, err := g.ledger.Attempt(ctx, g.marker, period, g.StaleClaimAfter)
	if err != nil {
		return fmt.Errorf("reportguard: claim %s for %s: %w", g.marker, period.Format(time.RFC3339), err)
	}
	if !claim.Allowed {
		if claim.Run.SucceededOn != nil {
			slog.InfoContext(ctx, "reportguard: already sent for this period; skipping", "marker", g.marker, "period", period)
			return nil
		}
		return fmt.Errorf("reportguard: %s for %s is held by another attempt; not sending", g.marker, period.Format(time.RFC3339))
	}

	sendErr := send(ctx)

	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	if sendErr != nil {
		if err := g.ledger.Fail(rctx, claim.Run.ID, claim.Run.AttemptCount, sendErr.Error(), time.Now()); err != nil {
			slog.ErrorContext(ctx, "reportguard: could not release the companion row after a failed send", "marker", g.marker, "err", err)
		}
		return sendErr
	}
	if err := g.ledger.Complete(rctx, claim.Run.ID, claim.Run.AttemptCount); err != nil {
		slog.ErrorContext(ctx, "reportguard: report sent but the companion row could not be completed; a re-run after the orphan window may resend it",
			"marker", g.marker, "period", period, "err", err)
	}
	return nil
}
