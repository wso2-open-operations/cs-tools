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

package mailpaging

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

const goodConfig = `{"rules":{"sre":{"levelWait":"3m","levels":[
 {"name":"R1","phones":["+15550000001"]},
 {"name":"R2","phones":["+15550000002","+15550000003"]},
 {"phones":["+15550000004"]}]}}}`

func TestParse_Valid(t *testing.T) {
	c, err := Parse([]byte(goodConfig))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	r := c.Rules["sre"]
	if r.Wait() != 3*time.Minute || len(r.Levels) != 3 {
		t.Fatalf("rule = %+v", r)
	}
	if r.Levels[2].Name != "R3" {
		t.Errorf("unnamed level = %q, want R3", r.Levels[2].Name)
	}
}

func TestParse_DefaultsWait(t *testing.T) {
	c, err := Parse([]byte(`{"rules":{"x":{"levels":[{"phones":["+15550000001"]}]}}}`))
	if err != nil || c.Rules["x"].Wait() != DefaultLevelWait {
		t.Fatalf("Parse() = %v, %v; want the default wait", c.Rules["x"].Wait(), err)
	}
}

func TestParse_Rejects(t *testing.T) {
	for name, doc := range map[string]string{
		"no rules":      `{"rules":{}}`,
		"no levels":     `{"rules":{"x":{"levels":[]}}}`,
		"no phones":     `{"rules":{"x":{"levels":[{"name":"R1","phones":[]}]}}}`,
		"bad phone":     `{"rules":{"x":{"levels":[{"phones":["0771234567"]}]}}}`,
		"dup level":     `{"rules":{"x":{"levels":[{"name":"R1","phones":["+15550000001"]},{"name":"R1","phones":["+15550000002"]}]}}}`,
		"short wait":    `{"rules":{"x":{"levelWait":"1s","levels":[{"phones":["+15550000001"]}]}}}`,
		"bad wait":      `{"rules":{"x":{"levelWait":"soon","levels":[{"phones":["+15550000001"]}]}}}`,
		"unknown field": `{"rules":{"x":{"levelz":[]}}}`,
	} {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: Parse() error = nil, want an error", name)
		}
	}
}

type fakeStore struct {
	mu      sync.Mutex
	started map[string]bool
	wake    map[string]time.Time
}

func newFakeStore() *fakeStore {
	return &fakeStore{started: map[string]bool{}, wake: map[string]time.Time{}}
}

func (s *fakeStore) Started(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started[id], nil
}
func (s *fakeStore) MarkStarted(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started[id] = true
	return nil
}
func (s *fakeStore) Schedule(_ context.Context, m string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.wake[m]; !ok {
		s.wake[m] = at
	}
	return nil
}
func (s *fakeStore) Due(_ context.Context, now time.Time, limit int64) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for m, at := range s.wake {
		if !at.After(now) {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out, nil
}
func (s *fakeStore) Claim(_ context.Context, m string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.wake[m]; !ok {
		return false, nil
	}
	delete(s.wake, m)
	return true, nil
}

type fakeCaller struct {
	mu    sync.Mutex
	calls []string
	msgs  []string
	fail  map[string]bool
}

func (c *fakeCaller) MakeCall(_ context.Context, to, msg string) (notifications.Call, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail[to] {
		return notifications.Call{}, errors.New("boom")
	}
	c.calls = append(c.calls, to)
	c.msgs = append(c.msgs, msg)
	return notifications.Call{SID: "CA1"}, nil
}

func newTestEngine(t *testing.T, sending bool) (*Engine, *fakeStore, *fakeCaller, *time.Time) {
	t.Helper()
	cfg, err := Parse([]byte(goodConfig))
	if err != nil {
		t.Fatal(err)
	}
	st, cl := newFakeStore(), &fakeCaller{fail: map[string]bool{}}
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	e := NewEngine(cfg, st, cl, sending)
	e.now = func() time.Time { return now }
	return e, st, cl, &now
}

var alert = events.MailAlertReceivedPayload{RuleName: "sre", MessageID: "m1", DedupeKey: "m1/sre", Subject: "Disk full"}

func TestLadder_CallsEachLevelAfterTheWait(t *testing.T) {
	e, _, cl, now := newTestEngine(t, true)
	ctx := context.Background()
	if err := e.HandleAlert(ctx, alert); err != nil {
		t.Fatalf("HandleAlert() error = %v", err)
	}
	if len(cl.calls) != 0 {
		t.Fatalf("calls before the first tick = %v, want none", cl.calls)
	}
	_ = e.Tick(ctx)
	if got := strings.Join(cl.calls, ","); got != "+15550000001" {
		t.Fatalf("after t0 calls = %q, want R1 only", got)
	}
	*now = now.Add(2*time.Minute + 59*time.Second)
	_ = e.Tick(ctx)
	if len(cl.calls) != 1 {
		t.Fatalf("calls before the wait = %v, want R1 only", cl.calls)
	}
	*now = now.Add(time.Second)
	_ = e.Tick(ctx)
	if got := strings.Join(cl.calls, ","); got != "+15550000001,+15550000002,+15550000003" {
		t.Fatalf("after 3m calls = %q, want R1 then both R2 users", got)
	}
	*now = now.Add(3 * time.Minute)
	_ = e.Tick(ctx)
	if len(cl.calls) != 4 || cl.calls[3] != "+15550000004" {
		t.Fatalf("after 6m calls = %v, want R3 last", cl.calls)
	}
	*now = now.Add(time.Hour)
	_ = e.Tick(ctx)
	if len(cl.calls) != 4 {
		t.Fatalf("calls after the last level = %v, want no more", cl.calls)
	}
	if !strings.Contains(cl.msgs[0], "Escalation level R1") || !strings.Contains(cl.msgs[0], "Disk full") {
		t.Errorf("message = %q", cl.msgs[0])
	}
}

func TestLadder_RedeliveryStartsNoSecondLadder(t *testing.T) {
	e, _, cl, _ := newTestEngine(t, true)
	ctx := context.Background()
	_ = e.HandleAlert(ctx, alert)
	_ = e.HandleAlert(ctx, alert)
	_ = e.Tick(ctx)
	if len(cl.calls) != 1 {
		t.Fatalf("calls = %v, want one R1 call", cl.calls)
	}
}

func TestLadder_FailedCallDoesNotStopTheLevelOrTheLadder(t *testing.T) {
	e, _, cl, now := newTestEngine(t, true)
	ctx := context.Background()
	cl.fail["+15550000002"] = true
	_ = e.HandleAlert(ctx, alert)
	_ = e.Tick(ctx)
	*now = now.Add(3 * time.Minute)
	_ = e.Tick(ctx)
	if got := strings.Join(cl.calls, ","); got != "+15550000001,+15550000003" {
		t.Fatalf("calls = %q, want the other R2 user still called", got)
	}
	*now = now.Add(3 * time.Minute)
	_ = e.Tick(ctx)
	if len(cl.calls) != 3 {
		t.Fatalf("calls = %v, want R3 after the failure", cl.calls)
	}
}

func TestLadder_UnknownRuleIsIgnored(t *testing.T) {
	e, st, _, _ := newTestEngine(t, true)
	p := alert
	p.RuleName = "other"
	if err := e.HandleAlert(context.Background(), p); err != nil || len(st.wake) != 0 {
		t.Fatalf("HandleAlert() = %v, wake = %v; want nil and nothing scheduled", err, st.wake)
	}
}

func TestLadder_SendingDisabledPlacesNoCalls(t *testing.T) {
	e, _, cl, now := newTestEngine(t, false)
	ctx := context.Background()
	_ = e.HandleAlert(ctx, alert)
	_ = e.Tick(ctx)
	*now = now.Add(10 * time.Minute)
	_ = e.Tick(ctx)
	if len(cl.calls) != 0 {
		t.Fatalf("calls = %v, want none while sending is disabled", cl.calls)
	}
}

func TestSpokenMessage_CleansSubject(t *testing.T) {
	got := spokenMessage("sre", "R1", "line1\nline2\t"+strings.Repeat("x", 300))
	if strings.ContainsAny(got, "\n\t") || len([]rune(got)) > 220 {
		t.Errorf("message = %q", got)
	}
}
