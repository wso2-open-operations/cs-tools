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

package eventbus

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryPolicy_WaitDoublesUpToTheCap(t *testing.T) {
	p := RetryPolicy{Attempts: 10, Delay: 30 * time.Second, MaxDelay: 15 * time.Minute}.normalized()
	want := []time.Duration{
		30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute,
		15 * time.Minute, 15 * time.Minute, 15 * time.Minute, 15 * time.Minute,
	}
	for i, w := range want {
		if got := p.wait(i + 2); got != w {
			t.Errorf("wait before attempt %d = %s, want %s", i+2, got, w)
		}
	}
	// 30s+1m+2m+4m+8m = 15m30s, plus four capped waits of 15m.
	if got, want := p.TotalWait(), 15*time.Minute+30*time.Second+4*15*time.Minute; got != want {
		t.Errorf("TotalWait = %s, want %s", got, want)
	}
}

func TestRetryPolicy_FixedDelayWithoutACap(t *testing.T) {
	p := DefaultRetryPolicy
	for n := 2; n <= 3; n++ {
		if got := p.wait(n); got != 2*time.Second {
			t.Errorf("wait(%d) = %s, want 2s", n, got)
		}
	}
	if got := p.TotalWait(); got != 4*time.Second {
		t.Errorf("TotalWait = %s, want 4s", got)
	}
}

func TestRetryPolicy_NormalizedFillsZeroes(t *testing.T) {
	p := RetryPolicy{}.normalized()
	if p.Attempts != 3 || p.Delay != 2*time.Second || p.MaxDelay != 0 {
		t.Errorf("normalized zero policy = %+v", p)
	}
}

// TestProcessRecord_RetriesThenDeadLetters is the main-topic tier: every
// attempt fails, the record goes to OnExhausted, and the offset is
// committed. NoMoreRetries stays false throughout because a DLQ tier follows.
func TestProcessRecord_RetriesThenDeadLetters(t *testing.T) {
	policy := RetryPolicy{Attempts: 3, Delay: time.Millisecond}
	var finals, noMore, calls int
	handle := func(_ context.Context, r Record) error {
		calls++
		if r.IsFinalAttempt {
			finals++
		}
		if r.NoMoreRetries {
			noMore++
		}
		return errors.New("salesforce down")
	}
	deadLettered := 0
	onExhausted := func(_ context.Context, _ Record, handleErr error) error {
		deadLettered++
		if handleErr == nil || handleErr.Error() != "salesforce down" {
			t.Errorf("onExhausted got %v", handleErr)
		}
		return nil
	}
	if !processRecord(context.Background(), Record{Topic: "t"}, handle, onExhausted, policy) {
		t.Fatal("want commit after dead-lettering")
	}
	if calls != 3 || finals != 1 || noMore != 0 || deadLettered != 1 {
		t.Errorf("calls=%d finals=%d noMoreRetries=%d deadLettered=%d", calls, finals, noMore, deadLettered)
	}
}

// TestProcessRecord_DLQTierSignalsTheLastAttempt is the dead-letter tier
// (no OnExhausted): the final call carries NoMoreRetries so the handler can
// record the give-up, and the record is then committed (dropped).
func TestProcessRecord_DLQTierSignalsTheLastAttempt(t *testing.T) {
	policy := RetryPolicy{Attempts: 4, Delay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
	var seen []bool
	handle := func(_ context.Context, r Record) error {
		seen = append(seen, r.NoMoreRetries)
		return errors.New("still down")
	}
	if !processRecord(context.Background(), Record{}, handle, nil, policy) {
		t.Fatal("want commit after exhausting the DLQ tier")
	}
	if len(seen) != 4 || seen[0] || seen[1] || seen[2] || !seen[3] {
		t.Errorf("NoMoreRetries per attempt = %v, want only the last true", seen)
	}
}

func TestProcessRecord_SucceedsOnARetry(t *testing.T) {
	policy := RetryPolicy{Attempts: 3, Delay: time.Millisecond}
	calls := 0
	handle := func(context.Context, Record) error {
		calls++
		if calls < 2 {
			return errors.New("blip")
		}
		return nil
	}
	exhausted := false
	if !processRecord(context.Background(), Record{}, handle, func(context.Context, Record, error) error { exhausted = true; return nil }, policy) {
		t.Fatal("want commit")
	}
	if calls != 2 || exhausted {
		t.Errorf("calls=%d exhausted=%v", calls, exhausted)
	}
}

// TestProcessRecord_DeadLetterFailureGivesTheHandlerALastCall covers the
// one path where content is lost: the dead-letter publish itself failed. The
// handler is called once more with NoMoreRetries so it can leave a record.
func TestProcessRecord_DeadLetterFailureGivesTheHandlerALastCall(t *testing.T) {
	policy := RetryPolicy{Attempts: 2, Delay: time.Millisecond}
	var lastCallNoMore bool
	calls := 0
	handle := func(_ context.Context, r Record) error {
		calls++
		lastCallNoMore = r.NoMoreRetries
		return errors.New("down")
	}
	onExhausted := func(context.Context, Record, error) error { return errors.New("dlq unreachable") }
	if !processRecord(context.Background(), Record{}, handle, onExhausted, policy) {
		t.Fatal("want commit: the record cannot be held")
	}
	if calls != 3 || !lastCallNoMore {
		t.Errorf("calls=%d lastCallNoMoreRetries=%v, want 3 and true", calls, lastCallNoMore)
	}
}

func TestProcessRecord_ShutdownMidWaitDoesNotCommit(t *testing.T) {
	policy := RetryPolicy{Attempts: 3, Delay: time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	handle := func(context.Context, Record) error {
		cancel()
		return errors.New("down")
	}
	if processRecord(ctx, Record{}, handle, nil, policy) {
		t.Fatal("a record interrupted by shutdown must not be committed")
	}
}
