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

package reportguard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

// memLedger is a minimal in-memory model of the ledger's per-(name, period)
// rule: one row, claimable when new or when its retry is due, denied once
// succeeded or while claimed.
type memLedger struct {
	rows        map[string]*ledger.Run
	attemptErr  error
	completeErr error
	attempts    []string
}

func key(name string, p time.Time) string { return name + "@" + p.UTC().Format(time.RFC3339) }

func (m *memLedger) Attempt(_ context.Context, name string, p time.Time, _ time.Duration) (ledger.Claim, error) {
	m.attempts = append(m.attempts, name)
	if m.attemptErr != nil {
		return ledger.Claim{}, m.attemptErr
	}
	if m.rows == nil {
		m.rows = map[string]*ledger.Run{}
	}
	r, ok := m.rows[key(name, p)]
	if !ok {
		r = &ledger.Run{ID: key(name, p), TaskName: name, PeriodKey: p, AttemptCount: 1}
		m.rows[r.ID] = r
		return ledger.Claim{Allowed: true, Run: *r}, nil
	}
	if r.SucceededOn == nil && r.NextRetryOn != nil && !r.NextRetryOn.After(time.Now()) {
		r.AttemptCount++
		r.NextRetryOn = nil
		return ledger.Claim{Allowed: true, Run: *r}, nil
	}
	return ledger.Claim{Allowed: false, Run: *r}, nil
}

func (m *memLedger) Complete(_ context.Context, id string, _ int) error {
	if m.completeErr != nil {
		return m.completeErr
	}
	now := time.Now()
	m.rows[id].SucceededOn = &now
	return nil
}

func (m *memLedger) Fail(_ context.Context, id string, _ int, _ string, next time.Time) error {
	m.rows[id].NextRetryOn = &next
	return nil
}

var period = time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)

func periodCtx() context.Context { return registry.WithPeriod(context.Background(), period) }

func TestOnce_SendsOncePerPeriod(t *testing.T) {
	l := &memLedger{}
	g := New(l, "stale_cases_report")
	sends := 0
	send := func(context.Context) error { sends++; return nil }

	for i := 0; i < 3; i++ {
		if err := g.Once(periodCtx(), send); err != nil {
			t.Fatalf("run %d: unexpected error %v", i, err)
		}
	}
	if sends != 1 {
		t.Fatalf("a re-run for the same period must not resend, got %d sends", sends)
	}
	if l.attempts[0] != "stale_cases_report.sent" {
		t.Fatalf("companion row name: got %q", l.attempts[0])
	}

	next := registry.WithPeriod(context.Background(), period.Add(24*time.Hour))
	if err := g.Once(next, send); err != nil || sends != 2 {
		t.Fatalf("the next period must send again, sends=%d err=%v", sends, err)
	}
}

func TestOnce_FailedSendIsRetryable(t *testing.T) {
	l := &memLedger{}
	g := New(l, "r")
	if err := g.Once(periodCtx(), func(context.Context) error { return errors.New("mail down") }); err == nil {
		t.Fatal("the send error must be returned")
	}
	sent := false
	if err := g.Once(periodCtx(), func(context.Context) error { sent = true; return nil }); err != nil || !sent {
		t.Fatalf("a failed send must be retryable in the same period, sent=%v err=%v", sent, err)
	}
}

func TestOnce_ClaimErrorSendsNothing(t *testing.T) {
	l := &memLedger{attemptErr: errors.New("ledger down")}
	sent := false
	err := New(l, "r").Once(periodCtx(), func(context.Context) error { sent = true; return nil })
	if err == nil || sent {
		t.Fatalf("with the ledger unreachable nothing may be sent, sent=%v err=%v", sent, err)
	}
}

func TestOnce_HeldByAnotherAttemptSendsNothing(t *testing.T) {
	l := &memLedger{}
	// A claimed, unresolved companion row: what a concurrent or crashed
	// sender leaves behind.
	_, _ = l.Attempt(context.Background(), "r"+MarkerSuffix, period, 0)
	sent := false
	err := New(l, "r").Once(periodCtx(), func(context.Context) error { sent = true; return nil })
	if err == nil || sent {
		t.Fatalf("a held companion row must block the send, sent=%v err=%v", sent, err)
	}
}

func TestOnce_CompanionCompleteFailureStillSucceeds(t *testing.T) {
	l := &memLedger{completeErr: errors.New("ledger blip")}
	sent := false
	if err := New(l, "r").Once(periodCtx(), func(context.Context) error { sent = true; return nil }); err != nil || !sent {
		t.Fatalf("a sent report must not fail the task, sent=%v err=%v", sent, err)
	}
}

func TestOnce_NoPeriodSendsUnguarded(t *testing.T) {
	l := &memLedger{}
	sent := false
	if err := New(l, "r").Once(context.Background(), func(context.Context) error { sent = true; return nil }); err != nil || !sent {
		t.Fatalf("without a period the send must still happen, sent=%v err=%v", sent, err)
	}
	if len(l.attempts) != 0 {
		t.Fatal("no companion claim without a period")
	}
}
