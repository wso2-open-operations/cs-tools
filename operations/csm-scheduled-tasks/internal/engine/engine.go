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
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/schedule"
	"golang.org/x/oauth2"
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
	// BookkeepingTimeout bounds each ledger record-back (Complete/Fail) and
	// alert e-mail made after a handler has returned. Those calls run on a
	// context detached from the tick's own cancellation — see
	// bookkeepingContext — so this is the only thing limiting them. Zero
	// means defaultBookkeepingTimeout.
	BookkeepingTimeout time.Duration
	// Concurrency is how many task attempts Tick runs at once. Tasks are
	// always started in shortest-interval-first order (see orderTasks);
	// values above 1 additionally stop one slow handler from holding every
	// later task. Zero or one means strictly sequential.
	Concurrency int
}

// New constructs an Engine.
func New(tasks []registry.Task, ledgerClient LedgerClient, emailClient EmailSender, driverInterval time.Duration, alertRecipients []string, alertsEnabled bool) *Engine {
	return &Engine{Tasks: tasks, Ledger: ledgerClient, Email: emailClient, DriverInterval: driverInterval, AlertRecipients: alertRecipients, AlertsEnabled: alertsEnabled}
}

// Stage names which step of one task's attempt a TaskError came from.
type Stage string

// The stages an attempt moves through, in order. "schedule" and "claim"
// happen before the handler runs; "complete"/"fail" are the ledger
// report-back; "alert" is the failure email.
const (
	StageSchedule Stage = "schedule"
	StageClaim    Stage = "claim"
	StageHandler  Stage = "handler"
	StageComplete Stage = "complete"
	StageFail     Stage = "fail"
	StageAlert    Stage = "alert"
)

// TaskError is one task's failure within a Tick: which task, at which
// stage, and the underlying error. Tick joins every TaskError it collects
// into the single error it returns, so a caller (cmd/server/main.go) can
// decide the process exit status from "did anything at all go wrong,"
// while a log reader or test can still errors.As its way to the specifics.
type TaskError struct {
	Task  string
	Stage Stage
	Err   error
}

func (e *TaskError) Error() string {
	return fmt.Sprintf("%s: %s: %v", e.Task, e.Stage, e.Err)
}

// Unwrap exposes the underlying error to errors.Is/errors.As.
func (e *TaskError) Unwrap() error { return e.Err }

// ErrInterrupted is joined into Tick's returned error when ctx was
// cancelled (SIGTERM, a Choreo timeout, a manual stop) before every
// registered task had been evaluated. The tasks not reached were neither
// claimed nor run; they are simply picked up by the next invocation.
var ErrInterrupted = errors.New("tick interrupted before every task was evaluated")

// Tick evaluates every registered task once against now. Call this exactly
// once per driver invocation — see cmd/server/main.go.
//
// The returned error is nil only when every task either ran and was
// recorded cleanly, or was legitimately not due (a denied claim is a normal
// outcome, not a failure). Anything else — a handler error, a ledger call
// that failed at any stage, an alert that could not be sent, or an
// interruption part-way through — is joined into the result as one
// *TaskError per task (plus ErrInterrupted), so the process can exit
// non-zero and the scheduler's run history shows a failed run rather than
// an unbroken row of green over a silent outage.
func (e *Engine) Tick(ctx context.Context, now time.Time) error {
	ordered := orderTasks(e.Tasks, now)

	workers := e.Concurrency
	if workers < 1 {
		workers = 1
	}
	// One result slot per task, in start order, so the joined error and the
	// ledger alert list tasks deterministically whatever order they finish.
	results := make([]error, len(ordered))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var interrupted error
	for i, task := range ordered {
		sem <- struct{}{}
		if ctxErr := ctx.Err(); ctxErr != nil {
			<-sem
			slog.ErrorContext(ctx, "csm-scheduled-tasks: tick interrupted; remaining tasks not evaluated", "nextTask", task.Name, "err", ctxErr)
			interrupted = fmt.Errorf("%w (next task: %s): %v", ErrInterrupted, task.Name, ctxErr)
			break
		}
		wg.Add(1)
		go func(i int, task registry.Task) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = e.attempt(ctx, task, now)
		}(i, task)
	}
	wg.Wait()

	var errs []error
	var ledgerErrs []*TaskError
	for _, err := range results {
		if err != nil {
			errs = append(errs, err)
			ledgerErrs = append(ledgerErrs, ledgerStageErrors(err)...)
		}
	}
	if interrupted != nil {
		errs = append(errs, interrupted)
	}
	// Ledger-stage failures get one aggregated alert per tick, sent after
	// every task has been evaluated — see alertLedgerFailures. The
	// handler-failure alert (recordFailure) is per task because each
	// failure is a different story; a dead ledger is one story told six
	// times, so it is one email.
	if len(ledgerErrs) > 0 {
		if err := e.alertLedgerFailures(ctx, now, ledgerErrs); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ledgerStageErrors extracts every *TaskError in err (which may be a joined
// error) whose stage is a ledger call — claim, complete or fail. Those are
// the failures recordFailure's own per-task alert never covers: a handler
// that never ran because the claim failed has no handler error to alert on,
// and a Complete/Fail that failed leaves the ledger unaware of what the
// handler just did.
func ledgerStageErrors(err error) []*TaskError {
	var out []*TaskError
	var walk func(error)
	walk = func(e error) {
		if e == nil {
			return
		}
		if j, ok := e.(interface{ Unwrap() []error }); ok {
			for _, inner := range j.Unwrap() {
				walk(inner)
			}
			return
		}
		var te *TaskError
		if errors.As(e, &te) {
			switch te.Stage {
			case StageClaim, StageComplete, StageFail:
				out = append(out, te)
			}
		}
	}
	walk(err)
	return out
}

// maxLedgerAlertEntries bounds how many failures one ledger alert lists.
// Seven tasks are registered today, so this is a guard against a future
// registry growing past what an email can usefully show, not a limit that
// bites now; the count beyond it is still reported.
const maxLedgerAlertEntries = 25

// ledgerAlertTaskName is the Task field of the *TaskError returned when the
// aggregated ledger alert itself could not be sent.
const ledgerAlertTaskName = "ledger-alert"

// alertLedgerFailures sends the one-per-tick "LEDGER ERROR" email. The
// email client does not depend on the ledger, so this still works when
// entity-service (or the token endpoint, for entity-service's scopes) is
// what's down. Recipients are the standing AlertRecipients plus every
// affected task's own To/Cc, de-duplicated — the same audience those
// tasks' handler failures would have reached. Honours AlertsEnabled like
// every other email. Returns a *TaskError if the send itself failed, so the
// exit status reflects that the on-call audience was NOT told.
func (e *Engine) alertLedgerFailures(ctx context.Context, now time.Time, failures []*TaskError) error {
	if !e.AlertsEnabled {
		return nil
	}
	affected := map[string]registry.Task{}
	for _, t := range e.Tasks {
		affected[t.Name] = t
	}
	var to, cc []string
	to = appendUnique(to, e.AlertRecipients...)
	for _, f := range failures {
		if t, ok := affected[f.Task]; ok {
			to = appendUnique(to, t.To...)
			cc = appendUnique(cc, t.Cc...)
		}
	}
	if len(to) == 0 {
		return nil
	}

	data := notify.LedgerAlertData{TickTime: now.Format(time.RFC3339)}
	for i, f := range failures {
		if i >= maxLedgerAlertEntries {
			data.Omitted = len(failures) - i
			break
		}
		data.Failures = append(data.Failures, notify.LedgerFailure{
			Task:  f.Task,
			Stage: string(f.Stage),
			Error: summarizeError(f.Err),
		})
	}
	// Plain ASCII subject — see recordFailure for why.
	subject := fmt.Sprintf("[csm-scheduled-tasks] LEDGER ERROR: %d task(s) - %s", len(failures), now.Format(time.RFC3339))
	bctx, cancel := e.bookkeepingContext(ctx)
	defer cancel()
	if err := e.Email.SendEmail(bctx, to, cc, subject, notify.RenderLedgerAlertEmail(data)); err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: failed to send ledger error alert", "failures", len(failures), "err", err)
		return &TaskError{Task: ledgerAlertTaskName, Stage: StageAlert, Err: err}
	}
	slog.InfoContext(ctx, "csm-scheduled-tasks: ledger error alert sent", "failures", len(failures), "to", len(to))
	return nil
}

// appendUnique appends each of vals to dst unless already present.
func appendUnique(dst []string, vals ...string) []string {
	for _, v := range vals {
		seen := false
		for _, d := range dst {
			if d == v {
				seen = true
				break
			}
		}
		if !seen {
			dst = append(dst, v)
		}
	}
	return dst
}

// maxSummaryLen bounds summarizeError's fallback branch.
const maxSummaryLen = 300

// summarizeError turns a ledger client error into one short, log-safe line
// for the ledger alert email. The typed failures this component actually
// produces are named explicitly — an entity-service non-2xx
// (*apierror.Error, whose Error() already omits the body), a token-endpoint
// rejection (*oauth2.RetrieveError, whose Error() would otherwise include
// the token response body), a timeout, a cancellation, or a transport-level
// failure (*url.Error, whose Error() would otherwise echo the full request
// URL). Anything else falls back to the error text with whitespace
// collapsed and length bounded.
func summarizeError(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("entity-service returned HTTP %d", apiErr.StatusCode)
	}
	var tokenErr *oauth2.RetrieveError
	if errors.As(err, &tokenErr) {
		if tokenErr.Response != nil {
			return fmt.Sprintf("token endpoint returned HTTP %d", tokenErr.Response.StatusCode)
		}
		return "token endpoint rejected the credentials request"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "request timed out"
	}
	if errors.Is(err, context.Canceled) {
		return "request cancelled (the process was interrupted)"
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return fmt.Sprintf("%s request failed: %s", urlErr.Op, collapse(urlErr.Err.Error()))
	}
	return collapse(err.Error())
}

// collapse folds all whitespace runs in s into single spaces and truncates
// to maxSummaryLen.
func collapse(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > maxSummaryLen {
		return s[:maxSummaryLen] + "…"
	}
	return s
}

// staleClaimMargin is how many missed ticks a claim is allowed to go
// without a Complete/Fail report before Ledger.Attempt treats it as
// orphaned (this process crashed mid-handler) rather than still genuinely
// in progress.
const staleClaimMargin = 2

// maxHandlerTimeout caps the timeout derived from a schedule, so a daily
// task's handler is bounded to something reasonable rather than a day.
const maxHandlerTimeout = 30 * time.Minute

// handlerTimeout is the budget for one Handler call: task.Timeout when set,
// otherwise the schedule's shortest inter-fire gap capped at
// maxHandlerTimeout — a handler still running when its own next period
// comes due has already lost that period.
func handlerTimeout(task registry.Task, now time.Time) time.Duration {
	if task.Timeout > 0 {
		return task.Timeout
	}
	gap, err := schedule.MinInterval(task.Schedule, now)
	if err != nil || gap <= 0 || gap > maxHandlerTimeout {
		return maxHandlerTimeout
	}
	return gap
}

// staleClaimAfter sizes the ledger's orphaned-claim window for one claim:
// the usual staleClaimMargin driver ticks, widened when the handler's own
// timeout plus one tick is longer — otherwise a handler legitimately still
// running could be reclaimed and run a second time concurrently by the
// next invocation.
func (e *Engine) staleClaimAfter(timeout time.Duration) time.Duration {
	window := staleClaimMargin * e.DriverInterval
	if w := timeout + e.DriverInterval; w > window {
		window = w
	}
	return window
}

// orderTasks returns tasks sorted by their schedule's MinInterval,
// shortest first, keeping registration order among equals — so the
// latency-sensitive five-minute tasks are started before a fifteen-minute
// or daily one can hold the tick. A schedule that cannot be parsed sorts
// last (its attempt reports the error).
func orderTasks(tasks []registry.Task, now time.Time) []registry.Task {
	type keyed struct {
		task registry.Task
		gap  time.Duration
	}
	ks := make([]keyed, len(tasks))
	for i, t := range tasks {
		gap, err := schedule.MinInterval(t.Schedule, now)
		if err != nil {
			gap = time.Duration(math.MaxInt64)
		}
		ks[i] = keyed{t, gap}
	}
	sort.SliceStable(ks, func(i, j int) bool { return ks[i].gap < ks[j].gap })
	out := make([]registry.Task, len(ks))
	for i, k := range ks {
		out[i] = k.task
	}
	return out
}

// ValidateCadence checks every task's schedule against the driver
// interval: a schedule whose shortest gap between firings is shorter than
// driverInterval can never be honoured, and its failed periods would be
// superseded before their retry (which defaults to driverInterval) came
// due. Returns an error naming every offending task, or nil.
func ValidateCadence(tasks []registry.Task, driverInterval time.Duration, now time.Time) error {
	var bad []string
	for _, t := range tasks {
		gap, err := schedule.MinInterval(t.Schedule, now)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s (%v)", t.Name, err))
			continue
		}
		if gap < driverInterval {
			bad = append(bad, fmt.Sprintf("%s fires every %s", t.Name, gap))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("driver interval %s is longer than the tightest registered schedule; set DRIVER_INTERVAL to the scheduler trigger cadence (and that trigger to at most the tightest schedule): %s",
			driverInterval, strings.Join(bad, "; "))
	}
	return nil
}

// defaultBookkeepingTimeout is Engine.BookkeepingTimeout when unset: long
// enough for one entity-service PATCH or one e-mail service POST on a
// healthy network, short enough that a hung ledger cannot hold a
// terminating process open past Choreo's grace period.
const defaultBookkeepingTimeout = 10 * time.Second

// bookkeepingContext derives the context every post-handler call — the
// ledger Complete/Fail and the alert e-mails — runs on. It is detached
// from ctx's cancellation on purpose: on SIGTERM (a Choreo timeout, a
// redeploy, a manual stop) the in-flight handler returns context.Canceled,
// and if the record-back then used that same cancelled ctx it would fail
// before dialling — the row would stay claimed with no retry time (so it
// is only reclaimable after the orphan window), and the alert would never
// be sent. Exactly the moment the bookkeeping matters most is the moment
// the original ctx is unusable. The timeout still bounds it. The handler
// itself keeps the real ctx and aborts promptly; only the bookkeeping is
// allowed to finish.
func (e *Engine) bookkeepingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := e.BookkeepingTimeout
	if timeout <= 0 {
		timeout = defaultBookkeepingTimeout
	}
	return context.WithTimeout(context.WithoutCancel(ctx), timeout)
}

// attempt runs one task's claim → handler → report-back cycle and returns
// nil, or a *TaskError naming the stage that went wrong. A denied claim
// returns nil.
func (e *Engine) attempt(ctx context.Context, task registry.Task, now time.Time) error {
	period, err := schedule.PeriodKey(task.Schedule, now)
	if err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: invalid schedule, skipping this tick", "task", task.Name, "schedule", task.Schedule, "err", err)
		return &TaskError{Task: task.Name, Stage: StageSchedule, Err: err}
	}

	timeout := handlerTimeout(task, now)
	claim, err := e.Ledger.Attempt(ctx, task.Name, period, e.staleClaimAfter(timeout))
	if err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: claim attempt failed", "task", task.Name, "period", period, "err", err)
		return &TaskError{Task: task.Name, Stage: StageClaim, Err: err}
	}
	if !claim.Allowed {
		slog.InfoContext(ctx, "csm-scheduled-tasks: not due, skipping", "task", task.Name, "period", period)
		return nil
	}

	slog.InfoContext(ctx, "csm-scheduled-tasks: running", "task", task.Name, "period", period, "attempt", claim.Run.AttemptCount, "timeout", timeout.String())
	hctx, cancel := context.WithTimeout(registry.WithPeriod(ctx, period), timeout)
	handlerErr := task.Handler(hctx)
	cancel()
	if handlerErr != nil {
		return e.recordFailure(ctx, task, period, claim.Run.ID, claim.Run.AttemptCount, handlerErr, now)
	}
	return e.recordSuccess(ctx, task, period, claim.Run.ID, claim.Run.AttemptCount)
}

// recordSuccess only updates the ledger — there is no success email; see
// registry.Task.To's own doc comment. attemptCount is the claim being
// completed — see LedgerClient.Complete's own doc comment for why that
// binding matters. Returns a *TaskError if the ledger could not record the
// success: the handler's work is done, but the row is left open and will
// be reclaimed as orphaned, so the caller must not report a clean run.
func (e *Engine) recordSuccess(ctx context.Context, task registry.Task, period time.Time, runID string, attemptCount int) error {
	slog.InfoContext(ctx, "csm-scheduled-tasks: succeeded", "task", task.Name, "period", period)
	bctx, cancel := e.bookkeepingContext(ctx)
	defer cancel()
	if err := e.Ledger.Complete(bctx, runID, attemptCount); err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: failed to record success in ledger", "task", task.Name, "runId", runID, "err", err)
		return &TaskError{Task: task.Name, Stage: StageComplete, Err: err}
	}
	return nil
}

// recordFailure records the failed attempt in the ledger and sends the
// failure alert. It always returns a non-nil error — at minimum the
// handler's own, as a StageHandler *TaskError — joined with a further
// *TaskError for each bookkeeping step (ledger Fail, alert email) that
// also failed, so none of those secondary failures is lost in the exit
// status.
func (e *Engine) recordFailure(ctx context.Context, task registry.Task, period time.Time, runID string, attemptCount int, handlerErr error, now time.Time) error {
	slog.ErrorContext(ctx, "csm-scheduled-tasks: failed", "task", task.Name, "period", period, "err", handlerErr)
	errs := []error{&TaskError{Task: task.Name, Stage: StageHandler, Err: handlerErr}}

	backoff := task.RetryBackoff
	if backoff <= 0 {
		backoff = e.DriverInterval
	}
	nextRetry := now.Add(backoff)
	bctx, cancel := e.bookkeepingContext(ctx)
	defer cancel()
	if err := e.Ledger.Fail(bctx, runID, attemptCount, handlerErr.Error(), nextRetry); err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: failed to record failure in ledger", "task", task.Name, "runId", runID, "err", err)
		errs = append(errs, &TaskError{Task: task.Name, Stage: StageFail, Err: err})
	}

	if !e.AlertsEnabled {
		return errors.Join(errs...)
	}

	// AlertRecipients (the standing ops audience) always gets included
	// alongside whatever this specific task's own To adds — see
	// Engine.AlertRecipients' own doc comment for why that's in To, not Cc.
	to := make([]string, 0, len(task.To)+len(e.AlertRecipients))
	to = append(to, task.To...)
	to = append(to, e.AlertRecipients...)
	if len(to) == 0 {
		return errors.Join(errs...)
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
		NextRetry:    nextRetry.Format(time.RFC3339),
		Error:        handlerErr.Error(),
	})
	if err := e.Email.SendEmail(bctx, to, task.Cc, subject, body); err != nil {
		slog.ErrorContext(ctx, "csm-scheduled-tasks: failed to send alert email", "task", task.Name, "err", err)
		errs = append(errs, &TaskError{Task: task.Name, Stage: StageAlert, Err: err})
	}
	return errors.Join(errs...)
}
