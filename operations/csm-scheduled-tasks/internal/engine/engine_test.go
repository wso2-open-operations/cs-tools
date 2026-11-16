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
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

// fakeLedger allows every claim at the given attempt count and records the
// retry time of the failure reported back.
type fakeLedger struct {
	attemptCount int
	nextRetry    time.Time
}

func (l *fakeLedger) Attempt(_ context.Context, _ string, _ time.Time, _ time.Duration) (ledger.Claim, error) {
	return ledger.Claim{Allowed: true, Run: ledger.Run{ID: "run-1", AttemptCount: l.attemptCount}}, nil
}

func (l *fakeLedger) Complete(context.Context, string, int) error { return nil }

func (l *fakeLedger) Fail(_ context.Context, _ string, _ int, _ string, nextRetryOn time.Time) error {
	l.nextRetry = nextRetryOn
	return nil
}

type fakeEmail struct{ bodies []string }

func (e *fakeEmail) SendEmail(_ context.Context, _, _ []string, _, htmlBody string) error {
	e.bodies = append(e.bodies, htmlBody)
	return nil
}

// A task with MaxAttempts is retried after its backoff until a failed
// attempt reaches the cap; that one is given no retry within any period,
// and its alert says so.
func TestEngine_MaxAttemptsStopsRetriesForThePeriod(t *testing.T) {
	now := time.Date(2026, 11, 2, 4, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name        string
		attempt     int
		maxAttempts int
		wantRetry   time.Time
	}{
		{"no cap", 5, 0, now.Add(4 * time.Hour)},
		{"below the cap", 1, 2, now.Add(4 * time.Hour)},
		{"at the cap", 2, 2, now.Add(noRetryDelay)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := &fakeLedger{attemptCount: tc.attempt}
			email := &fakeEmail{}
			task := registry.Task{
				Name:         "acp_closure",
				Schedule:     "0 4 * * 1-5",
				Handler:      func(context.Context) error { return errors.New("boom") },
				RetryBackoff: 4 * time.Hour,
				MaxAttempts:  tc.maxAttempts,
				To:           []string{"ops@example.com"},
			}
			New([]registry.Task{task}, l, email, time.Hour, nil, true).Tick(context.Background(), now)

			if !l.nextRetry.Equal(tc.wantRetry) {
				t.Errorf("next retry = %v, want %v", l.nextRetry, tc.wantRetry)
			}
			if len(email.bodies) != 1 {
				t.Fatalf("alerts sent = %d, want 1", len(email.bodies))
			}
			if capped := strings.Contains(email.bodies[0], "attempts used for this period"); capped != (tc.wantRetry.Equal(now.Add(noRetryDelay))) {
				t.Errorf("alert says no more retries = %v, want %v", capped, !capped)
			}
		})
	}
}

// denyLedger denies every claim, as entity-service does for a task whose
// window is not now.
type denyLedger struct{ fakeLedger }

func (*denyLedger) Attempt(context.Context, string, time.Time, time.Duration) (ledger.Claim, error) {
	return ledger.Claim{Allowed: false}, nil
}

// A task that is not due leaves no per-task line at the default log level;
// the tick still reports how many ran.
func TestEngine_NotDueTaskIsNotLoggedAtInfo(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	task := registry.Task{Name: "weekly_report", Schedule: "0 4 * * 1", Handler: func(context.Context) error { return nil }}
	New([]registry.Task{task}, &denyLedger{}, &fakeEmail{}, time.Hour, nil, true).Tick(context.Background(), time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC))

	out := buf.String()
	if strings.Contains(out, "weekly_report") {
		t.Errorf("not-due task was logged at info:\n%s", out)
	}
	if !strings.Contains(out, "ran=0") || !strings.Contains(out, "registered=1") {
		t.Errorf("tick summary missing:\n%s", out)
	}
}
