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
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// Call outcomes (ALERTING_CALL_OUTCOME_CHECK).
//
// A rung's call is logged "ALERT TRIGGERED ... status=queued": Twilio took
// it, nothing more. Whether the phone rang, was answered, was busy or failed
// is only in Twilio's console -- this service sets no StatusCallback, and
// could not take one: its only route is /health and it is visible to the
// organization alone. So, opt-in, a goroutine on the server's context asks
// Twilio how each placed call ended (TwilioClient.GetCall), the way a
// paging-number test call already does (testcall.go), and logs one line:
//
//	escalation: call outcome  incidentId=... level=LEVEL_0 attempt=1
//	    recipientId=... callSid=CA... result=no-answer twilioStatus=no-answer
//
// Logged only: it never changes the ladder. A call that went unanswered is
// already followed by the rung's next attempt and then the next rung; this
// line is for the person asking afterwards whether the phone rang.
//
// KNOWN GAP, the same one test calls accept: a restart during the wait loses
// that call's line. Twilio's console still has it.

// Call outcome results, as the log line spells them.
const (
	CallOutcomeCompleted = "completed"
	CallOutcomeNoAnswer  = "no-answer"
	CallOutcomeBusy      = "busy"
	CallOutcomeFailed    = "failed"
	CallOutcomeCanceled  = "canceled"
	// CallOutcomeUnresolved is a call Twilio still reported queued, ringing
	// or in progress when the watcher gave up; twilioStatus says which.
	CallOutcomeUnresolved = "unresolved"
)

// callStatusReader is the slice of notifications.TwilioClient the watcher uses.
type callStatusReader interface {
	GetCall(ctx context.Context, callSID string) (notifications.Call, error)
}

// callStatusTimeout bounds one status read; await also cuts it to the time
// left before GiveUpAfter.
const callStatusTimeout = 10 * time.Second

// CallOutcomeWatcher logs how each placed call ended. A nil watcher is off.
type CallOutcomeWatcher struct {
	calls callStatusReader
	// base outlives a record: the waits run on it and end with it.
	base context.Context

	// FirstCheck is how long after the call was placed Twilio is first
	// asked; PollEvery how often after that while the call is still queued,
	// ringing or in progress; GiveUpAfter when to stop asking. The defaults
	// cover Twilio's 60 s ring timeout plus a short message. Fields so tests
	// can shorten them.
	FirstCheck, PollEvery, GiveUpAfter time.Duration
	// now is time.Now unless a test substitutes one; GiveUpAfter is measured
	// on it.
	now func() time.Time

	wg sync.WaitGroup
}

// NewCallOutcomeWatcher constructs a watcher whose waits run on base (the
// server's context).
func NewCallOutcomeWatcher(base context.Context, calls *notifications.TwilioClient) *CallOutcomeWatcher {
	return newCallOutcomeWatcher(base, calls)
}

func newCallOutcomeWatcher(base context.Context, calls callStatusReader) *CallOutcomeWatcher {
	return &CallOutcomeWatcher{
		calls: calls, base: base,
		FirstCheck: time.Minute, PollEvery: 20 * time.Second, GiveUpAfter: 3 * time.Minute,
		now: time.Now,
	}
}

// watchedCall is what the outcome line says about a call besides its result.
// Ids only: the recipient is named by LogID.
type watchedCall struct {
	incidentID  string
	rule        string
	priority    string
	level       string
	attempt     int
	recipientID string
	callSID     string
}

// watch starts waiting for one placed call and returns at once.
func (w *CallOutcomeWatcher) watch(c watchedCall) {
	if w == nil || strings.TrimSpace(c.callSID) == "" {
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		result, twilioStatus, ok := w.await(c.callSID)
		if !ok {
			// The server is stopping; the console still has the call.
			return
		}
		attrs := []any{
			"incidentId", c.incidentID, "rule", c.rule, "priority", c.priority,
			"level", c.level, "attempt", c.attempt,
			"recipientId", c.recipientID, "callSid", c.callSID,
			"result", result, "twilioStatus", twilioStatus,
		}
		if result == CallOutcomeCompleted {
			slog.Info("escalation: call outcome", attrs...)
			return
		}
		slog.Warn("escalation: call outcome", attrs...)
	}()
}

// Wait blocks until every started watch has logged or given up. For tests and
// shutdown.
func (w *CallOutcomeWatcher) Wait() {
	if w != nil {
		w.wg.Wait()
	}
}

// await asks Twilio until the call has ended or GiveUpAfter has passed, and
// reports the result and the last status Twilio gave. ok is false when the
// server's context ended first.
//
//	completed, no-answer, busy, failed, canceled  that, as Twilio says it
//	queued, ringing, in-progress                  ask again; once GiveUpAfter
//	                                              has elapsed, unresolved
//
// A failed status read counts as "still going" and is asked again.
func (w *CallOutcomeWatcher) await(callSID string) (result, twilioStatus string, ok bool) {
	// GiveUpAfter is elapsed time from the start, not the sum of the sleeps:
	// a slow status read must not stretch the wait past it. Each read and
	// each pause is cut to the time left.
	deadline := w.now().Add(w.GiveUpAfter)
	last := "unknown"
	if !w.sleep(w.FirstCheck) {
		return "", "", false
	}
	for {
		left := deadline.Sub(w.now())
		if left <= 0 {
			return CallOutcomeUnresolved, last, true
		}
		ctx, cancel := context.WithTimeout(w.base, min(callStatusTimeout, left))
		got, err := w.calls.GetCall(ctx, callSID)
		cancel()
		if err != nil {
			if w.base.Err() != nil {
				return "", "", false
			}
			slog.Warn("escalation: could not read a call's status; asking again", "callSid", callSID, "err", err)
		} else {
			last = strings.ToLower(strings.TrimSpace(got.Status))
			switch last {
			case CallOutcomeCompleted, CallOutcomeNoAnswer, CallOutcomeBusy, CallOutcomeFailed, CallOutcomeCanceled:
				return last, last, true
			}
		}
		left = deadline.Sub(w.now())
		if left <= 0 {
			return CallOutcomeUnresolved, last, true
		}
		if !w.sleep(min(w.PollEvery, left)) {
			return "", "", false
		}
	}
}

// sleep waits d on the server's context, reporting false if it ends first.
func (w *CallOutcomeWatcher) sleep(d time.Duration) bool {
	if d <= 0 {
		return w.base.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-w.base.Done():
		return false
	}
}
