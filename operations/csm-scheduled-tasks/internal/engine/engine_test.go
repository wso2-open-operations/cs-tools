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

package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"golang.org/x/oauth2"
)

// fakeLedger is an in-memory LedgerClient. Every call is recorded so a
// test can assert on exactly which ledger writes happened and in what
// order; the per-method error fields make any stage fail on demand.
type fakeLedger struct {
	mu sync.Mutex

	allowed    bool
	attemptErr error
	// attemptFn, when set, overrides allowed/attemptErr per task.
	attemptFn func(taskName string) (ledger.Claim, error)

	completeErr error
	failErr     error

	attempts  []string
	completes []string
	fails     []failCall

	// ctxSeen records, per record-back call, whether the context it was
	// given was still live and carried a deadline — the bookkeeping
	// contract engine.bookkeepingContext promises.
	ctxSeen []ctxState

	// staleAfter records the staleClaimAfter each task's claim carried.
	staleAfter map[string]time.Duration
}

type failCall struct {
	id     string
	errMsg string
}

// ctxState is what a fake observed about the context a call arrived on.
type ctxState struct {
	live        bool // ctx.Err() == nil at call time
	hasDeadline bool
}

func observe(ctx context.Context) ctxState {
	_, hasDeadline := ctx.Deadline()
	return ctxState{live: ctx.Err() == nil, hasDeadline: hasDeadline}
}

func (f *fakeLedger) Attempt(_ context.Context, taskName string, _ time.Time, staleAfter time.Duration) (ledger.Claim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts = append(f.attempts, taskName)
	if f.staleAfter == nil {
		f.staleAfter = map[string]time.Duration{}
	}
	f.staleAfter[taskName] = staleAfter
	if f.attemptFn != nil {
		return f.attemptFn(taskName)
	}
	if f.attemptErr != nil {
		return ledger.Claim{}, f.attemptErr
	}
	return ledger.Claim{Allowed: f.allowed, Run: ledger.Run{ID: "run-" + taskName, TaskName: taskName, AttemptCount: 1}}, nil
}

func (f *fakeLedger) Complete(ctx context.Context, id string, _ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completes = append(f.completes, id)
	f.ctxSeen = append(f.ctxSeen, observe(ctx))
	return f.completeErr
}

func (f *fakeLedger) Fail(ctx context.Context, id string, _ int, errMsg string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fails = append(f.fails, failCall{id: id, errMsg: errMsg})
	f.ctxSeen = append(f.ctxSeen, observe(ctx))
	return f.failErr
}

type sentEmail struct {
	to, cc  []string
	subject string
	body    string
	ctx     ctxState
}

type fakeEmail struct {
	mu      sync.Mutex
	sendErr error
	sent    []sentEmail
}

func (f *fakeEmail) SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentEmail{to: to, cc: cc, subject: subject, body: htmlBody, ctx: observe(ctx)})
	return f.sendErr
}

func okHandler(context.Context) error { return nil }

func newEngine(tasks []registry.Task, l *fakeLedger, m *fakeEmail) *Engine {
	return New(tasks, l, m, 5*time.Minute, []string{"oncall@example.com"}, true)
}

// stagesOf flattens Tick's joined error into the set of task/stage pairs it
// names, so a test can assert on exactly which failures were reported.
func stagesOf(err error) map[string]bool {
	out := map[string]bool{}
	var walk func(error)
	walk = func(e error) {
		if e == nil {
			return
		}
		var te *TaskError
		if errors.As(e, &te) {
			out[te.Task+"/"+string(te.Stage)] = true
		}
		if j, ok := e.(interface{ Unwrap() []error }); ok {
			for _, inner := range j.Unwrap() {
				walk(inner)
			}
		}
	}
	walk(err)
	return out
}

func TestTick_EverythingSucceedsReturnsNil(t *testing.T) {
	l := &fakeLedger{allowed: true}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: okHandler},
		{Name: "b", Schedule: "0 3 * * *", Handler: okHandler},
	}, l, m)

	if err := eng.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(l.completes) != 2 {
		t.Fatalf("expected both tasks completed, got %v", l.completes)
	}
	if len(l.fails) != 0 || len(m.sent) != 0 {
		t.Fatalf("expected no failures recorded and no email, got fails=%v sent=%d", l.fails, len(m.sent))
	}
}

func TestTick_DeniedClaimIsNotAnError(t *testing.T) {
	ran := false
	l := &fakeLedger{allowed: false}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(context.Context) error { ran = true; return nil }},
	}, l, &fakeEmail{})

	if err := eng.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("a denied claim must not be reported as a failure, got: %v", err)
	}
	if ran {
		t.Fatal("handler must not run on a denied claim")
	}
	if len(l.completes) != 0 || len(l.fails) != 0 {
		t.Fatalf("no ledger report-back expected on a denied claim, got completes=%v fails=%v", l.completes, l.fails)
	}
}

func TestTick_HandlerErrorIsReturnedRecordedAndAlerted(t *testing.T) {
	l := &fakeLedger{allowed: true}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(context.Context) error { return errors.New("boom") },
			To: []string{"owner@example.com"}, Cc: []string{"cc@example.com"}},
	}, l, m)

	err := eng.Tick(context.Background(), time.Now())
	if err == nil {
		t.Fatal("expected a non-nil error when a handler fails")
	}
	if got := stagesOf(err); !got["a/handler"] {
		t.Fatalf("expected a handler-stage TaskError for task a, got %v (err: %v)", got, err)
	}
	if len(l.fails) != 1 || l.fails[0].id != "run-a" || l.fails[0].errMsg != "boom" {
		t.Fatalf("expected one Fail for run-a carrying the handler error, got %+v", l.fails)
	}
	if len(l.completes) != 0 {
		t.Fatalf("Complete must not be called on a failed handler, got %v", l.completes)
	}
	if len(m.sent) != 1 {
		t.Fatalf("expected exactly one alert email, got %d", len(m.sent))
	}
	e := m.sent[0]
	if strings.Join(e.to, ",") != "owner@example.com,oncall@example.com" {
		t.Errorf("To must merge the task's own To with AlertRecipients, got %v", e.to)
	}
	if strings.Join(e.cc, ",") != "cc@example.com" {
		t.Errorf("Cc must stay per-task, got %v", e.cc)
	}
	if !strings.Contains(e.subject, "FAILED: a") {
		t.Errorf("subject should name the task, got %q", e.subject)
	}
	if !strings.Contains(e.body, "boom") {
		t.Errorf("body should carry the handler error, got %q", e.body)
	}
}

func TestTick_AlertsDisabledStillRecordsButSendsNothing(t *testing.T) {
	l := &fakeLedger{allowed: true}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(context.Context) error { return errors.New("boom") }},
	}, l, m)
	eng.AlertsEnabled = false

	if err := eng.Tick(context.Background(), time.Now()); err == nil {
		t.Fatal("the failure must still be returned with alerts disabled")
	}
	if len(l.fails) != 1 {
		t.Fatalf("the failure must still be recorded in the ledger, got %+v", l.fails)
	}
	if len(m.sent) != 0 {
		t.Fatalf("no email expected with alerts disabled, got %d", len(m.sent))
	}
}

func TestTick_NoRecipientsSendsNothing(t *testing.T) {
	l := &fakeLedger{allowed: true}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(context.Context) error { return errors.New("boom") }},
	}, l, m)
	eng.AlertRecipients = nil

	_ = eng.Tick(context.Background(), time.Now())
	if len(m.sent) != 0 {
		t.Fatalf("no email expected with no recipients at all, got %d", len(m.sent))
	}
}

func TestTick_ClaimErrorIsReturned(t *testing.T) {
	ran := false
	l := &fakeLedger{attemptErr: errors.New("upstream returned 503")}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(context.Context) error { ran = true; return nil }},
	}, l, &fakeEmail{})

	err := eng.Tick(context.Background(), time.Now())
	if got := stagesOf(err); !got["a/claim"] {
		t.Fatalf("expected a claim-stage TaskError, got %v (err: %v)", got, err)
	}
	if ran {
		t.Fatal("handler must not run when the claim itself failed")
	}
}

func TestTick_CompleteErrorIsReturned(t *testing.T) {
	l := &fakeLedger{allowed: true, completeErr: errors.New("upstream returned 500")}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: okHandler},
	}, l, &fakeEmail{})

	err := eng.Tick(context.Background(), time.Now())
	if got := stagesOf(err); !got["a/complete"] {
		t.Fatalf("a failed Complete must be reported, got %v (err: %v)", got, err)
	}
}

func TestTick_FailErrorIsReportedAlongsideHandlerError(t *testing.T) {
	l := &fakeLedger{allowed: true, failErr: errors.New("upstream returned 500")}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(context.Context) error { return errors.New("boom") }},
	}, l, &fakeEmail{})

	err := eng.Tick(context.Background(), time.Now())
	got := stagesOf(err)
	if !got["a/handler"] || !got["a/fail"] {
		t.Fatalf("expected both the handler and the Fail-stage errors, got %v (err: %v)", got, err)
	}
}

func TestTick_AlertSendFailureIsReported(t *testing.T) {
	l := &fakeLedger{allowed: true}
	m := &fakeEmail{sendErr: errors.New("mail service down")}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(context.Context) error { return errors.New("boom") }},
	}, l, m)

	err := eng.Tick(context.Background(), time.Now())
	if got := stagesOf(err); !got["a/alert"] {
		t.Fatalf("a failed alert send must be reported, got %v (err: %v)", got, err)
	}
}

func TestTick_InvalidScheduleIsReturned(t *testing.T) {
	l := &fakeLedger{allowed: true}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "not a cron", Handler: okHandler},
	}, l, &fakeEmail{})

	err := eng.Tick(context.Background(), time.Now())
	if got := stagesOf(err); !got["a/schedule"] {
		t.Fatalf("expected a schedule-stage TaskError, got %v (err: %v)", got, err)
	}
	if len(l.attempts) != 0 {
		t.Fatal("no claim must be attempted for an invalid schedule")
	}
}

func TestTick_OneTaskFailingDoesNotStopTheRest(t *testing.T) {
	l := &fakeLedger{allowed: true}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(context.Context) error { return errors.New("boom") }},
		{Name: "b", Schedule: "*/5 * * * *", Handler: okHandler},
	}, l, &fakeEmail{})

	err := eng.Tick(context.Background(), time.Now())
	if err == nil {
		t.Fatal("expected task a's failure to be returned")
	}
	if len(l.completes) != 1 || l.completes[0] != "run-b" {
		t.Fatalf("task b must still run and complete, got completes=%v", l.completes)
	}
}

func TestTick_CancelledContextIsInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	l := &fakeLedger{allowed: true}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: okHandler},
	}, l, &fakeEmail{})

	err := eng.Tick(ctx, time.Now())
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if len(l.attempts) != 0 {
		t.Fatalf("no task must be claimed once the context is cancelled, got %v", l.attempts)
	}
}

func TestTick_CancellationMidTickStopsBeforeTheNextTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l := &fakeLedger{allowed: true}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(context.Context) error { cancel(); return nil }},
		{Name: "b", Schedule: "*/5 * * * *", Handler: okHandler},
	}, l, &fakeEmail{})

	err := eng.Tick(ctx, time.Now())
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if strings.Join(l.attempts, ",") != "a" {
		t.Fatalf("only task a should have been claimed, got %v", l.attempts)
	}
}

// ── bookkeeping survives cancellation ──────────────────────────────────

// cancelDuringHandler returns a handler that cancels the tick's context
// (as SIGTERM would) and then returns ret — simulating a handler
// interrupted mid-flight.
func cancelDuringHandler(cancel context.CancelFunc, ret func(ctx context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		cancel()
		return ret(ctx)
	}
}

func TestTick_FailureRecordAndAlertSurviveCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l := &fakeLedger{allowed: true}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: cancelDuringHandler(cancel, func(ctx context.Context) error { return ctx.Err() })},
	}, l, m)

	err := eng.Tick(ctx, time.Now())
	if got := stagesOf(err); !got["a/handler"] || got["a/fail"] || got["a/alert"] {
		t.Fatalf("expected only the handler failure to be reported, got %v (err: %v)", got, err)
	}
	if len(l.fails) != 1 || !l.ctxSeen[0].live || !l.ctxSeen[0].hasDeadline {
		t.Fatalf("Fail must be called on a live, deadline-bound context after cancellation, got fails=%d ctx=%+v", len(l.fails), l.ctxSeen)
	}
	if len(m.sent) != 1 || !m.sent[0].ctx.live || !m.sent[0].ctx.hasDeadline {
		t.Fatalf("the alert must be sent on a live, deadline-bound context after cancellation, got %d sent, ctx=%+v", len(m.sent), m.sent)
	}
}

func TestTick_SuccessRecordSurvivesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l := &fakeLedger{allowed: true}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: cancelDuringHandler(cancel, func(context.Context) error { return nil })},
	}, l, &fakeEmail{})

	if err := eng.Tick(ctx, time.Now()); err != nil {
		t.Fatalf("a handler that finished before noticing the cancellation must still complete cleanly, got %v", err)
	}
	if len(l.completes) != 1 || !l.ctxSeen[0].live || !l.ctxSeen[0].hasDeadline {
		t.Fatalf("Complete must be called on a live, deadline-bound context after cancellation, got completes=%d ctx=%+v", len(l.completes), l.ctxSeen)
	}
}

func TestTick_LedgerAlertSurvivesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l := &fakeLedger{attemptFn: func(string) (ledger.Claim, error) {
		cancel()
		return ledger.Claim{}, context.Canceled
	}}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{{Name: "a", Schedule: "*/5 * * * *", Handler: okHandler}}, l, m)

	err := eng.Tick(ctx, time.Now())
	if got := stagesOf(err); !got["a/claim"] {
		t.Fatalf("expected the claim failure, got %v (err: %v)", got, err)
	}
	if len(m.sent) != 1 || !m.sent[0].ctx.live || !m.sent[0].ctx.hasDeadline {
		t.Fatalf("the ledger alert must be sent on a live, deadline-bound context after cancellation, got %d sent", len(m.sent))
	}
}

func TestTick_BookkeepingTimeoutIsApplied(t *testing.T) {
	l := &fakeLedger{allowed: true}
	eng := newEngine([]registry.Task{{Name: "a", Schedule: "*/5 * * * *", Handler: okHandler}}, l, &fakeEmail{})
	eng.BookkeepingTimeout = time.Second

	var seen time.Time
	eng.Ledger = ledgerFunc{onComplete: func(ctx context.Context) {
		seen, _ = ctx.Deadline()
	}, fakeLedger: l}
	before := time.Now()
	_ = eng.Tick(context.Background(), before)
	if seen.IsZero() || seen.Sub(before) > 2*time.Second {
		t.Fatalf("Complete's context deadline should be about BookkeepingTimeout from now, got %v", seen.Sub(before))
	}
}

// ledgerFunc wraps a fakeLedger so a test can inspect the context Complete
// receives without changing the shared fake.
type ledgerFunc struct {
	onComplete func(ctx context.Context)
	*fakeLedger
}

func (w ledgerFunc) Complete(ctx context.Context, id string, n int) error {
	w.onComplete(ctx)
	return w.fakeLedger.Complete(ctx, id, n)
}

// ── ordering, timeouts, concurrency, cadence ───────────────────────────

func TestTick_RunsShortestIntervalFirst(t *testing.T) {
	l := &fakeLedger{allowed: true}
	eng := newEngine([]registry.Task{
		{Name: "daily", Schedule: "0 3 * * *", Handler: okHandler},
		{Name: "quarter", Schedule: "*/15 * * * *", Handler: okHandler},
		{Name: "five-a", Schedule: "*/5 * * * *", Handler: okHandler},
		{Name: "five-b", Schedule: "*/5 * * * *", Handler: okHandler},
	}, l, &fakeEmail{})

	if err := eng.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(l.attempts, ","); got != "five-a,five-b,quarter,daily" {
		t.Fatalf("expected shortest-interval-first, registration order among equals; got %s", got)
	}
}

func TestTick_HandlerRunsUnderScheduleDerivedTimeout(t *testing.T) {
	var deadline time.Time
	var ok bool
	l := &fakeLedger{allowed: true}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(ctx context.Context) error {
			deadline, ok = ctx.Deadline()
			return nil
		}},
	}, l, &fakeEmail{})

	start := time.Now()
	_ = eng.Tick(context.Background(), start)
	if !ok {
		t.Fatal("the handler must run with a deadline")
	}
	if d := deadline.Sub(start); d < 4*time.Minute || d > 6*time.Minute {
		t.Fatalf("a */5 handler's timeout should be about 5m, got %s", d)
	}
}

func TestTick_ExplicitTimeoutWinsAndStopsAHungHandler(t *testing.T) {
	l := &fakeLedger{allowed: true}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Timeout: 20 * time.Millisecond, Handler: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}},
	}, l, m)

	err := eng.Tick(context.Background(), time.Now())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a hung handler must be cut off by its timeout, got %v", err)
	}
	if len(l.fails) != 1 {
		t.Fatalf("the timeout must be recorded as a failure, got %+v", l.fails)
	}
}

func TestTick_HandlerSeesTheClaimedPeriod(t *testing.T) {
	var got time.Time
	var ok bool
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "0 3 * * *", Handler: func(ctx context.Context) error {
			got, ok = registry.PeriodFrom(ctx)
			return nil
		}},
	}, &fakeLedger{allowed: true}, &fakeEmail{})

	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	_ = eng.Tick(context.Background(), now)
	if !ok || !got.Equal(time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)) {
		t.Fatalf("the handler must see the period it was claimed for, got %v (set=%v)", got, ok)
	}
}

func TestHandlerTimeout(t *testing.T) {
	now := time.Date(2026, 10, 1, 10, 2, 0, 0, time.UTC)
	cases := []struct {
		task registry.Task
		want time.Duration
	}{
		{registry.Task{Schedule: "*/5 * * * *"}, 5 * time.Minute},
		{registry.Task{Schedule: "*/15 * * * *"}, 15 * time.Minute},
		{registry.Task{Schedule: "0 3 * * *"}, maxHandlerTimeout},
		{registry.Task{Schedule: "*/5 * * * *", Timeout: time.Minute}, time.Minute},
		{registry.Task{Schedule: "bogus"}, maxHandlerTimeout},
	}
	for _, tc := range cases {
		if got := handlerTimeout(tc.task, now); got != tc.want {
			t.Errorf("%q timeout=%s: got %s, want %s", tc.task.Schedule, tc.task.Timeout, got, tc.want)
		}
	}
}

func TestTick_ClaimWindowCoversTheHandlerTimeout(t *testing.T) {
	l := &fakeLedger{allowed: false}
	eng := newEngine([]registry.Task{
		{Name: "five", Schedule: "*/5 * * * *", Handler: okHandler},
		{Name: "quarter", Schedule: "*/15 * * * *", Handler: okHandler},
	}, l, &fakeEmail{}) // DriverInterval 5m

	_ = eng.Tick(context.Background(), time.Now())
	if got := l.staleAfter["five"]; got != 10*time.Minute {
		t.Errorf("*/5 task: want the usual 2 x 5m window, got %s", got)
	}
	if got := l.staleAfter["quarter"]; got != 20*time.Minute {
		t.Errorf("*/15 task: window must cover its 15m timeout plus a tick (20m), got %s", got)
	}
}

func TestTick_ConcurrencyLetsShortTasksFinishPastASlowOne(t *testing.T) {
	release := make(chan struct{})
	l := &fakeLedger{allowed: true}
	var mu sync.Mutex
	var finished []string
	mark := func(name string) {
		mu.Lock()
		finished = append(finished, name)
		mu.Unlock()
	}
	eng := newEngine([]registry.Task{
		{Name: "slow", Schedule: "*/5 * * * *", Handler: func(ctx context.Context) error {
			select {
			case <-release:
			case <-ctx.Done():
			}
			mark("slow")
			return nil
		}},
		{Name: "fast", Schedule: "*/5 * * * *", Handler: func(context.Context) error {
			mark("fast")
			close(release)
			return nil
		}},
	}, l, &fakeEmail{})
	eng.Concurrency = 2

	done := make(chan error, 1)
	go func() { done <- eng.Tick(context.Background(), time.Now()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("with two workers the fast task must run while the slow one waits")
	}
	if strings.Join(finished, ",") != "fast,slow" {
		t.Fatalf("expected fast to finish first, got %v", finished)
	}
}

func TestValidateCadence(t *testing.T) {
	now := time.Now()
	tasks := []registry.Task{
		{Name: "five", Schedule: "*/5 * * * *"},
		{Name: "daily", Schedule: "0 3 * * *"},
	}
	if err := ValidateCadence(tasks, 5*time.Minute, now); err != nil {
		t.Fatalf("a 5m driver covers a */5 schedule, got %v", err)
	}
	err := ValidateCadence(tasks, time.Hour, now)
	if err == nil {
		t.Fatal("a 1h driver cannot honour a */5 schedule")
	}
	if !strings.Contains(err.Error(), "five fires every 5m0s") || strings.Contains(err.Error(), "daily") {
		t.Fatalf("the error should name only the offending task, got %v", err)
	}
	if err := ValidateCadence([]registry.Task{{Name: "x", Schedule: "bogus"}}, time.Minute, now); err == nil {
		t.Fatal("an invalid schedule must be reported")
	}
}

// ── ledger-failure alerting ────────────────────────────────────────────

func TestTick_ClaimFailuresProduceOneAggregatedLedgerAlert(t *testing.T) {
	l := &fakeLedger{attemptErr: &apierror.Error{StatusCode: 503, Body: "secret body"}}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: okHandler, To: []string{"owner@example.com"}, Cc: []string{"cc@example.com"}},
		{Name: "b", Schedule: "*/5 * * * *", Handler: okHandler, To: []string{"oncall@example.com"}},
		{Name: "c", Schedule: "*/5 * * * *", Handler: okHandler},
	}, l, m)

	err := eng.Tick(context.Background(), time.Now())
	if err == nil {
		t.Fatal("claim failures must be returned")
	}
	if len(m.sent) != 1 {
		t.Fatalf("expected exactly one aggregated ledger alert, got %d", len(m.sent))
	}
	e := m.sent[0]
	if !strings.Contains(e.subject, "LEDGER ERROR: 3 task(s)") {
		t.Errorf("subject should count the affected tasks, got %q", e.subject)
	}
	for _, name := range []string{"a", "b", "c"} {
		if !strings.Contains(e.body, "<strong>"+name+"</strong>") {
			t.Errorf("body should list task %s, got %q", name, e.body)
		}
	}
	if !strings.Contains(e.body, "claim") || !strings.Contains(e.body, "entity-service returned HTTP 503") {
		t.Errorf("body should name the stage and the summarised error, got %q", e.body)
	}
	if strings.Contains(e.body, "secret body") {
		t.Errorf("the upstream response body must never reach the alert, got %q", e.body)
	}
	if strings.Join(e.to, ",") != "oncall@example.com,owner@example.com" {
		t.Errorf("To must be AlertRecipients plus affected tasks' To, de-duplicated; got %v", e.to)
	}
	if strings.Join(e.cc, ",") != "cc@example.com" {
		t.Errorf("Cc must be the union of affected tasks' Cc, got %v", e.cc)
	}
}

func TestTick_CompleteFailureProducesLedgerAlert(t *testing.T) {
	l := &fakeLedger{allowed: true, completeErr: errors.New("upstream returned 500")}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{{Name: "a", Schedule: "*/5 * * * *", Handler: okHandler}}, l, m)

	_ = eng.Tick(context.Background(), time.Now())
	if len(m.sent) != 1 {
		t.Fatalf("expected one ledger alert for the failed Complete, got %d", len(m.sent))
	}
	if !strings.Contains(m.sent[0].body, "complete") {
		t.Errorf("body should name the complete stage, got %q", m.sent[0].body)
	}
}

func TestTick_HandlerAndFailStageFailuresAlertSeparately(t *testing.T) {
	l := &fakeLedger{allowed: true, failErr: errors.New("upstream returned 500")}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{
		{Name: "a", Schedule: "*/5 * * * *", Handler: func(context.Context) error { return errors.New("boom") }},
	}, l, m)

	_ = eng.Tick(context.Background(), time.Now())
	if len(m.sent) != 2 {
		t.Fatalf("expected the handler alert plus the ledger alert, got %d", len(m.sent))
	}
	if !strings.Contains(m.sent[0].subject, "FAILED: a") || !strings.Contains(m.sent[1].subject, "LEDGER ERROR") {
		t.Errorf("unexpected subjects: %q, %q", m.sent[0].subject, m.sent[1].subject)
	}
}

func TestTick_LedgerAlertHonoursAlertsDisabled(t *testing.T) {
	l := &fakeLedger{attemptErr: errors.New("down")}
	m := &fakeEmail{}
	eng := newEngine([]registry.Task{{Name: "a", Schedule: "*/5 * * * *", Handler: okHandler}}, l, m)
	eng.AlertsEnabled = false

	if err := eng.Tick(context.Background(), time.Now()); err == nil {
		t.Fatal("the claim failure must still be returned")
	}
	if len(m.sent) != 0 {
		t.Fatalf("no email expected with alerts disabled, got %d", len(m.sent))
	}
}

func TestTick_LedgerAlertSendFailureIsReported(t *testing.T) {
	l := &fakeLedger{attemptErr: errors.New("down")}
	m := &fakeEmail{sendErr: errors.New("mail down")}
	eng := newEngine([]registry.Task{{Name: "a", Schedule: "*/5 * * * *", Handler: okHandler}}, l, m)

	err := eng.Tick(context.Background(), time.Now())
	got := stagesOf(err)
	if !got["a/claim"] || !got[ledgerAlertTaskName+"/alert"] {
		t.Fatalf("expected both the claim failure and the unsent ledger alert, got %v (err: %v)", got, err)
	}
}

func TestSummarizeError(t *testing.T) {
	tokenErr := &oauth2.RetrieveError{
		Response: &http.Response{StatusCode: 401},
		Body:     []byte(`{"error":"invalid_client","error_description":"very long secret-bearing body"}`),
	}
	cases := []struct {
		name    string
		err     error
		want    string
		mustNot string
	}{
		{"entity-service non-2xx", fmt.Errorf("ledger: wrap: %w", &apierror.Error{StatusCode: 503, Body: "body"}), "entity-service returned HTTP 503", "body"},
		{"token endpoint", fmt.Errorf("ledger: POST /x: %w", &url.Error{Op: "Post", URL: "https://example.invalid/x", Err: tokenErr}), "token endpoint returned HTTP 401", "secret-bearing"},
		{"timeout", fmt.Errorf("ledger: %w", context.DeadlineExceeded), "request timed out", ""},
		{"cancelled", fmt.Errorf("ledger: %w", context.Canceled), "request cancelled (the process was interrupted)", ""},
		{"transport", &url.Error{Op: "Post", URL: "https://example.invalid/scheduled-tasks/attempts?x=1", Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}}, "Post request failed: dial tcp: connection refused", "example.invalid"},
		{"fallback collapses whitespace", errors.New("line one\n\n   line two"), "line one line two", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeError(tc.err)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if tc.mustNot != "" && strings.Contains(got, tc.mustNot) {
				t.Errorf("summary %q must not contain %q", got, tc.mustNot)
			}
		})
	}
	long := strings.Repeat("x", maxSummaryLen+50)
	if got := summarizeError(errors.New(long)); len([]rune(got)) != maxSummaryLen+1 {
		t.Errorf("fallback must be bounded to %d chars plus the ellipsis, got %d", maxSummaryLen, len([]rune(got)))
	}
}
