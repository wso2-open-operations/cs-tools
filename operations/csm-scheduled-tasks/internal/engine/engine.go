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

// Package engine is the one-pass tick algorithm — see this component's own
// CLAUDE.md ("Engine tick") for the full design. Deciding whether a task is
// due, claiming it, and superseding a stale prior period are all
// entity-service's own job (internal/ledger just calls it); this package's
// only responsibility is: for each registered task, compute its current
// period, ask the ledger whether to run, and report the outcome back.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/schedule"
)

// LedgerClient is the subset of *ledger.Client the engine depends on —
// declared here (dependency-inversion style) so a test can substitute a
// fake without importing the real HTTP client.
type LedgerClient interface {
	Attempt(ctx context.Context, taskName string, periodKey time.Time, staleClaimAfter time.Duration) (ledger.Claim, error)
	Complete(ctx context.Context, id string, attemptCount int) error
	Fail(ctx context.Context, id string, attemptCount int, errMsg string, nextRetryOn time.Time) error
}

// EmailSender is the subset of *notify.Client the engine depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// Engine runs one Tick over every registered Task.
type Engine struct {
	Tasks  []registry.Task
	Ledger LedgerClient
	Email  EmailSender
	// DriverInterval is how often Tick itself is expected to be invoked
	// (i.e. the Choreo Scheduled Task's own trigger cadence). Used as the
	// default for a task's RetryBackoff when it's zero, and to size the
	// orphaned-claim safety margin passed to Ledger.Attempt.
	DriverInterval time.Duration
	// AlertRecipients is a static set of people who get every failure
	// alert, for every task, in addition to that task's own
	// registry.Task.To/Cc — a standing ops/on-call audience, distinct from
	// (and unconditional on) whatever per-task recipients a specific
	// sub-cron sets. Included in the email's To alongside the task's own
	// To, not Cc, so a task with no To/Cc of its own still actually gets
	// an email sent (rather than silently having nothing in To at all) as
	// long as this is set. Nil is a valid, deliberate choice for "no
	// standing alert audience configured."
	AlertRecipients []string
	// AlertsEnabled is cmd/server/main.go's ALERTS_ENABLED — the global email
	// kill switch for this whole component, despite the field name only
	// describing what it does here. recordFailure still records the failure
	// in the ledger and logs it either way, this only silences the email
	// itself. Sits above both AlertRecipients and every task's own To/Cc: set
	// to false to go quiet for a maintenance window or a known-noisy period
	// without touching either config. A report-style task (e.g.
	// internal/stalecases) reads the same underlying value directly, since
	// its email isn't sent through this struct at all — see that package's
	// own SendReport doc comment. Defaults to true.
	AlertsEnabled bool
}

// New constructs an Engine.
func New(tasks []registry.Task, ledgerClient LedgerClient, emailClient EmailSender, driverInterval time.Duration, alertRecipients []string, alertsEnabled bool) *Engine {
	return &Engine{Tasks: tasks, Ledger: ledgerClient, Email: emailClient, DriverInterval: driverInterval, AlertRecipients: alertRecipients, AlertsEnabled: alertsEnabled}
}

// Tick evaluates every registered task once against now. Call this exactly
// once per driver invocation — see cmd/server/main.go.
func (e *Engine) Tick(ctx context.Context, now time.Time) {
	for _, task := range e.Tasks {
		e.attempt(ctx, task, now)
	}
}

// staleClaimMargin is how many missed ticks a claim is allowed to go
// without a Complete/Fail report before Ledger.Attempt treats it as
// orphaned (this process crashed mid-handler) rather than still genuinely
// in progress.
const staleClaimMargin = 2

// noRetryDelay is the retry time given to a period that used its
// registry.Task.MaxAttempts: later than any period lasts, so it is never
// retried and the next period supersedes it.
const noRetryDelay = 10 * 365 * 24 * time.Hour

func (e *Engine) attempt(ctx context.Context, task registry.Task, now time.Time) {
	period, err := schedule.PeriodKey(task.Schedule, now)
	if err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: invalid schedule, skipping this tick", "task", task.Name, "schedule", task.Schedule, "err", err)
		return
	}

	claim, err := e.Ledger.Attempt(ctx, task.Name, period, staleClaimMargin*e.DriverInterval)
	if err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: claim attempt failed", "task", task.Name, "period", period, "err", err)
		return
	}
	if !claim.Allowed {
		slog.InfoContext(ctx, "csm-scheduled-tasks: not due, skipping", "task", task.Name, "period", period)
		return
	}

	slog.InfoContext(ctx, "csm-scheduled-tasks: running", "task", task.Name, "period", period, "attempt", claim.Run.AttemptCount)
	if handlerErr := task.Handler(ctx); handlerErr != nil {
		e.recordFailure(ctx, task, period, claim.Run.ID, claim.Run.AttemptCount, handlerErr, now)
		return
	}
	e.recordSuccess(ctx, task, period, claim.Run.ID, claim.Run.AttemptCount)
}

// recordSuccess only updates the ledger — there is no success email yet;
// see registry.Task.To's own doc comment. attemptCount is the claim being
// completed — see LedgerClient.Complete's own doc comment for why that
// binding matters.
func (e *Engine) recordSuccess(ctx context.Context, task registry.Task, period time.Time, runID string, attemptCount int) {
	slog.InfoContext(ctx, "csm-scheduled-tasks: succeeded", "task", task.Name, "period", period)
	if err := e.Ledger.Complete(ctx, runID, attemptCount); err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: failed to record success in ledger", "task", task.Name, "runId", runID, "err", err)
	}
}

func (e *Engine) recordFailure(ctx context.Context, task registry.Task, period time.Time, runID string, attemptCount int, handlerErr error, now time.Time) {
	slog.ErrorContext(ctx, "csm-scheduled-tasks: failed", "task", task.Name, "period", period, "err", handlerErr)

	backoff := task.RetryBackoff
	if backoff <= 0 {
		backoff = e.DriverInterval
	}
	nextRetry := now.Add(backoff)
	nextRetryText := nextRetry.Format(time.RFC3339)
	if task.MaxAttempts > 0 && attemptCount >= task.MaxAttempts {
		// The ledger has no "give up" state, so the retry is put past any
		// period: the next period's claim supersedes this row long before.
		nextRetry = now.Add(noRetryDelay)
		nextRetryText = fmt.Sprintf("none: %d of %d attempts used for this period", attemptCount, task.MaxAttempts)
		slog.WarnContext(ctx, "csm-scheduled-tasks: attempt limit reached, not retrying this period",
			"task", task.Name, "period", period, "attempt", attemptCount, "maxAttempts", task.MaxAttempts)
	}
	if err := e.Ledger.Fail(ctx, runID, attemptCount, handlerErr.Error(), nextRetry); err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: failed to record failure in ledger", "task", task.Name, "runId", runID, "err", err)
	}

	if !e.AlertsEnabled {
		return
	}

	// AlertRecipients (the standing ops audience) always gets included
	// alongside whatever this specific task's own To adds — see
	// Engine.AlertRecipients' own doc comment for why that's in To, not Cc.
	to := make([]string, 0, len(task.To)+len(e.AlertRecipients))
	to = append(to, task.To...)
	to = append(to, e.AlertRecipients...)
	if len(to) == 0 {
		return
	}
	// Plain ASCII only — this is a mail Subject header, not HTML, so
	// there's no entity-reference escape hatch the way alert.html's body
	// has for non-ASCII characters (see notify.escapeHTML's own doc
	// comment for why that matters: the external email-sending service
	// doesn't reliably preserve non-ASCII bytes through its own send path).
	subject := fmt.Sprintf("[csm-scheduled-tasks] FAILED: %s - %s", task.Name, period.Format(time.RFC3339))
	body := notify.RenderAlertEmail(notify.AlertEmailData{
		TaskName:     task.Name,
		Period:       period.Format(time.RFC3339),
		AttemptCount: attemptCount,
		NextRetry:    nextRetryText,
		Error:        handlerErr.Error(),
	})
	if err := e.Email.SendEmail(ctx, to, task.Cc, subject, body); err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: failed to send alert email", "task", task.Name, "err", err)
	}
}
