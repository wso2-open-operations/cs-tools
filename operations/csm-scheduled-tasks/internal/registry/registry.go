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

// Package registry defines the Task type — the shape every sub-cron takes —
// and nothing else. cmd/server/main.go owns the actual list of registered
// tasks; this package deliberately has no built-in tasks of its own, so
// adding a real sub-cron never means editing this package.
package registry

import (
	"context"
	"time"
)

type periodKey struct{}

// WithPeriod returns ctx carrying the period key the engine claimed for the
// current Handler call. The engine sets it; handlers read it with
// PeriodFrom when they need to tie a side effect to "this period" (see
// internal/reportguard).
func WithPeriod(ctx context.Context, period time.Time) context.Context {
	return context.WithValue(ctx, periodKey{}, period)
}

// PeriodFrom returns the period key set by WithPeriod, and whether one was
// set.
func PeriodFrom(ctx context.Context) (time.Time, bool) {
	p, ok := ctx.Value(periodKey{}).(time.Time)
	return p, ok
}

// Task is one registered sub-cron. See this component's own CLAUDE.md
// ("Adding a sub-cron") for the full walkthrough of adding a real one.
type Task struct {
	// Name is this task's stable identity — it is sent to entity-service as
	// scheduled_task_run.taskName and used as the ledger key. Renaming a
	// task orphans its prior history there; treat it like an API contract,
	// not a display label.
	Name string
	// Schedule is a standard 5-field cron expression (minute granularity)
	// — e.g. "0 2 * * *" for daily at 02:00 UTC — or a 6-field one with a
	// leading seconds field (e.g. "*/30 * * * * *") for faster cadences;
	// see internal/schedule.PeriodKey's own doc comment. Must be valid per
	// github.com/adhocore/gronx.IsValid, checked once at startup
	// (see cmd/server/main.go) rather than silently skipped every tick.
	Schedule string
	// Handler does the actual work for one period. A non-nil error is
	// recorded as a failed attempt (see engine.Engine.Tick) and never
	// retried more than once per driver tick — the ledger, not this
	// function, decides whether/when to try again.
	//
	// Handler must be idempotent per period: there is no checkpointing, so a
	// worker that crashes mid-run has its claim reclaimed once it looks
	// orphaned (see entity-service's StaleClaimAfterSeconds), and the next
	// claimant re-runs Handler from scratch for that same period key.
	Handler func(ctx context.Context) error
	// RetryBackoff is how far into the future the next retry is scheduled
	// after a failure. Defaults to the driver's own tick interval when
	// zero (see cmd/server/main.go) — there is no point backing off
	// shorter than how often this process even runs.
	RetryBackoff time.Duration
	// Timeout bounds one Handler call. Zero means "derive it from the
	// schedule": the shortest gap between two consecutive firings, capped
	// (see engine.handlerTimeout) — a task that is still running when its
	// own next period comes due has already lost. Set it explicitly only
	// for a handler whose single-run budget is genuinely unrelated to its
	// cadence. The engine also widens the ledger's orphaned-claim window to
	// cover this, so a long-running handler is not reclaimed mid-run.
	Timeout time.Duration
	// To/Cc are additional recipients emailed when this specific task
	// fails, on top of the standing ALERT_RECIPIENTS audience every task
	// already alerts (see engine.Engine.AlertRecipients' own doc comment)
	// — deliberately per-task, not shared: two sub-crons can alert
	// completely different extra audiences (e.g. a billing job's own
	// on-call team vs. a usage report's product owner). Set from
	// cmd/server/main.go's SUB_CRON_RECIPIENTS config (via
	// recipientsFor), not hardcoded here — see this component's own
	// CLAUDE.md, "Adding a sub-cron." Both nil is the common case: this
	// task has no audience beyond ALERT_RECIPIENTS. Report-style tasks send
	// their own success e-mail from inside the handler instead — see
	// "Per-task report emails" in that same doc.
	To []string
	Cc []string
}
