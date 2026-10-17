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
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// scriptedStatuses answers GetCall from a script, one entry per read; the
// last entry repeats. An entry with err set fails that read.
type scriptedStatuses struct {
	mu     sync.Mutex
	script []scriptedStatus
	reads  int
}

type scriptedStatus struct {
	status string
	err    error
}

func (s *scriptedStatuses) GetCall(_ context.Context, sid string) (notifications.Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.reads
	if i >= len(s.script) {
		i = len(s.script) - 1
	}
	s.reads++
	if s.script[i].err != nil {
		return notifications.Call{}, s.script[i].err
	}
	return notifications.Call{SID: sid, Status: s.script[i].status}, nil
}

// quickWatcher asks at once and then every millisecond, for at most four reads.
func quickWatcher(ctx context.Context, calls callStatusReader) *CallOutcomeWatcher {
	w := newCallOutcomeWatcher(ctx, calls)
	w.FirstCheck, w.PollEvery, w.GiveUpAfter = 0, time.Millisecond, 3*time.Millisecond
	return w
}

// captureLogs sends the default logger to a buffer until the test ends.
func captureLogs(t *testing.T) *strings.Builder {
	t.Helper()
	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestCallOutcomeWatcher_Await(t *testing.T) {
	cases := []struct {
		name       string
		script     []scriptedStatus
		wantResult string
		wantStatus string
	}{
		{"answered", []scriptedStatus{{status: "completed"}}, CallOutcomeCompleted, "completed"},
		{"rang out", []scriptedStatus{{status: "ringing"}, {status: "no-answer"}}, CallOutcomeNoAnswer, "no-answer"},
		{"busy", []scriptedStatus{{status: "queued"}, {status: "busy"}}, CallOutcomeBusy, "busy"},
		{"failed", []scriptedStatus{{status: "failed"}}, CallOutcomeFailed, "failed"},
		{"canceled", []scriptedStatus{{status: "Canceled"}}, CallOutcomeCanceled, "canceled"},
		{"read error, then an answer", []scriptedStatus{{err: errors.New("twilio 503")}, {status: "completed"}}, CallOutcomeCompleted, "completed"},
		{"still ringing at the end", []scriptedStatus{{status: "ringing"}}, CallOutcomeUnresolved, "ringing"},
		{"never readable", []scriptedStatus{{err: errors.New("twilio 503")}}, CallOutcomeUnresolved, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := quickWatcher(context.Background(), &scriptedStatuses{script: tc.script})
			result, status, ok := w.await("CA1")
			if !ok || result != tc.wantResult || status != tc.wantStatus {
				t.Fatalf("await = (%q, %q, %v); want (%q, %q, true)", result, status, ok, tc.wantResult, tc.wantStatus)
			}
		})
	}
}

// A watch logs one line: info for an answered call, a warning otherwise.
func TestCallOutcomeWatcher_LogsOneLine(t *testing.T) {
	buf := captureLogs(t)
	w := quickWatcher(context.Background(), &scriptedStatuses{script: []scriptedStatus{{status: "no-answer"}}})
	w.watch(watchedCall{incidentID: "inc-1", level: "LEVEL_0", attempt: 2, recipientID: "u-123", callSID: "CA1"})
	w.Wait()

	out := buf.String()
	if strings.Count(out, `msg="escalation: call outcome"`) != 1 {
		t.Fatalf("want one outcome line, got: %s", out)
	}
	for _, want := range []string{"level=WARN", "incidentId=inc-1", "attempt=2", "recipientId=u-123", "callSid=CA1", "result=no-answer"} {
		if !strings.Contains(out, want) {
			t.Fatalf("outcome line lacks %s: %s", want, out)
		}
	}
}

// Off is the default: a nil watcher, or a call with no sid, starts nothing.
func TestCallOutcomeWatcher_NilAndNoSIDAreNoOps(t *testing.T) {
	var off *CallOutcomeWatcher
	off.watch(watchedCall{callSID: "CA1"})
	off.Wait()

	calls := &scriptedStatuses{script: []scriptedStatus{{status: "completed"}}}
	w := quickWatcher(context.Background(), calls)
	w.watch(watchedCall{callSID: "  "})
	w.Wait()
	if calls.reads != 0 {
		t.Fatalf("Twilio was asked %d times for a call with no sid", calls.reads)
	}
}

// A stopping server ends the wait without a line.
func TestCallOutcomeWatcher_StopsWithTheServer(t *testing.T) {
	buf := captureLogs(t)
	ctx, cancel := context.WithCancel(context.Background())
	w := newCallOutcomeWatcher(ctx, &scriptedStatuses{script: []scriptedStatus{{status: "completed"}}})
	w.FirstCheck = time.Hour
	w.watch(watchedCall{callSID: "CA1"})
	cancel()
	w.Wait()
	if strings.Contains(buf.String(), "call outcome") {
		t.Fatalf("logged an outcome after shutdown: %s", buf.String())
	}
}

// Through the engine: a placed call is watched, and neither the ALERT
// TRIGGERED line nor the outcome line names the person.
func TestEngine_PlacedCallIsWatchedAndLoggedByID(t *testing.T) {
	buf := captureLogs(t)
	e := testEngine(newMemStore(), &fakeCaller{}, &fakeNotes{}, enabled())
	w := quickWatcher(context.Background(), &scriptedStatuses{script: []scriptedStatus{{status: "ringing"}, {status: "completed"}}})
	e.WithCallOutcomeWatcher(w)

	plan := Plan{Trigger: Trigger{IncidentID: "inc-1", Priority: "HIGH"}}
	call := PlannedCall{Level: Level0, Ordinal: 1, Recipient: Recipient{
		UserID: "u-123", Name: "Jane Doe", Email: "jane@example.com", Phone: "+94770000001",
	}}
	if err, cardErr := e.placeVia(context.Background(), plan, call, e.notifiers, ChannelCall); err != nil || cardErr != nil {
		t.Fatalf("placeVia = %v, %v", err, cardErr)
	}
	w.Wait()

	out := buf.String()
	if !strings.Contains(out, `msg="escalation: ALERT TRIGGERED"`) || !strings.Contains(out, `msg="escalation: call outcome"`) {
		t.Fatalf("want both the trigger and the outcome line: %s", out)
	}
	if strings.Count(out, "recipientId=u-123") != 2 {
		t.Fatalf("want recipientId=u-123 on both lines: %s", out)
	}
	if !strings.Contains(out, "callSid=CAtest") || !strings.Contains(out, "result=completed") {
		t.Fatalf("outcome line lacks the call or its result: %s", out)
	}
	for _, leak := range []string{"Jane", "jane@example.com", "0000001"} {
		if strings.Contains(out, leak) {
			t.Fatalf("logs carry %q: %s", leak, out)
		}
	}
}

// Without a watcher the engine logs exactly what it did before: no outcome line.
func TestEngine_NoWatcherNoOutcomeLine(t *testing.T) {
	buf := captureLogs(t)
	e := testEngine(newMemStore(), &fakeCaller{}, &fakeNotes{}, enabled())
	call := PlannedCall{Level: Level0, Ordinal: 1, Recipient: Recipient{UserID: "u-123", Phone: "+94770000001"}}
	if err, _ := e.placeVia(context.Background(), Plan{}, call, e.notifiers, ChannelCall); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "call outcome") {
		t.Fatalf("an engine with no watcher logged an outcome: %s", buf.String())
	}
}

// slowStatuses answers "ringing" and moves the fake clock on by each read's
// cost, the way a slow Twilio would; its last answer is "completed".
type slowStatuses struct {
	clock *time.Time
	cost  time.Duration
	reads int
	until int
}

func (s *slowStatuses) GetCall(_ context.Context, sid string) (notifications.Call, error) {
	*s.clock = s.clock.Add(s.cost)
	s.reads++
	if s.reads >= s.until {
		return notifications.Call{SID: sid, Status: "completed"}, nil
	}
	return notifications.Call{SID: sid, Status: "ringing"}, nil
}

// GiveUpAfter is elapsed time: slow reads count against it, not only the
// pauses between them.
func TestCallOutcomeWatcher_GiveUpAfterIsElapsedTime(t *testing.T) {
	clock := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	calls := &slowStatuses{clock: &clock, cost: 10 * time.Second, until: 4}
	w := newCallOutcomeWatcher(context.Background(), calls)
	w.FirstCheck, w.PollEvery, w.GiveUpAfter = 0, 0, 30*time.Second
	w.now = func() time.Time { return clock }

	result, status, ok := w.await("CA1")
	if !ok || result != CallOutcomeUnresolved || status != "ringing" {
		t.Fatalf("await = (%q, %q, %v); want unresolved after the deadline", result, status, ok)
	}
	if calls.reads != 3 {
		t.Errorf("read %d times in 30s of 10s reads; want 3", calls.reads)
	}
}
