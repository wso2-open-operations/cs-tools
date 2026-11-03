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
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/paging"
)

const goodConfig = `{"rules":{"sre":{"levelWait":"3m","levels":[
 {"name":"R1","emails":["a@example.com"]},
 {"name":"R2","emails":["b@example.com","C@Example.com"]},
 {"emails":["d@example.com"]}]}}}`

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

func TestParse_NormalisesEmails(t *testing.T) {
	c, err := Parse([]byte(goodConfig))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(c.Rules["sre"].Levels[1].Emails, ","); got != "b@example.com,c@example.com" {
		t.Errorf("R2 emails = %q, want lower-cased", got)
	}
}

func TestParse_DefaultsWait(t *testing.T) {
	c, err := Parse([]byte(`{"rules":{"x":{"levels":[{"emails":["a@example.com"]}]}}}`))
	if err != nil || c.Rules["x"].Wait() != DefaultLevelWait {
		t.Fatalf("Parse() = %v, %v; want the default wait", c.Rules["x"].Wait(), err)
	}
}

func TestParse_Rejects(t *testing.T) {
	for name, doc := range map[string]string{
		"no rules":      `{"rules":{}}`,
		"no levels":     `{"rules":{"x":{"levels":[]}}}`,
		"no emails":     `{"rules":{"x":{"levels":[{"name":"R1","emails":[]}]}}}`,
		"bad email":     `{"rules":{"x":{"levels":[{"emails":["not-an-email"]}]}}}`,
		"phones field":  `{"rules":{"x":{"levels":[{"phones":["+15550000001"]}]}}}`,
		"dup level":     `{"rules":{"x":{"levels":[{"name":"R1","emails":["a@example.com"]},{"name":"R1","emails":["b@example.com"]}]}}}`,
		"short wait":    `{"rules":{"x":{"levelWait":"1s","levels":[{"emails":["a@example.com"]}]}}}`,
		"bad wait":      `{"rules":{"x":{"levelWait":"soon","levels":[{"emails":["a@example.com"]}]}}}`,
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

type fakeLookup struct {
	numbers map[string]string
	calls   int
	failFor int
}

func (l *fakeLookup) DialNumbers(_ context.Context, emails []string) (map[string]paging.DialNumber, error) {
	l.calls++
	if l.calls <= l.failFor {
		return nil, errors.New("entity-service down")
	}
	out := map[string]paging.DialNumber{}
	for _, e := range emails {
		if n, ok := l.numbers[e]; ok {
			out[e] = paging.DialNumber{Number: n, Source: paging.DialSourceProfile}
		}
	}
	return out, nil
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

var testNumbers = map[string]string{
	"a@example.com": "+15550000001",
	"b@example.com": "+15550000002",
	"c@example.com": "+15550000003",
	"d@example.com": "+15550000004",
}

func newTestEngine(t *testing.T, sending bool) (*Engine, *fakeStore, *fakeCaller, *time.Time) {
	t.Helper()
	cfg, err := Parse([]byte(goodConfig))
	if err != nil {
		t.Fatal(err)
	}
	st, cl := newFakeStore(), &fakeCaller{fail: map[string]bool{}}
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	numbers := map[string]string{}
	for k, v := range testNumbers {
		numbers[k] = v
	}
	e := NewEngine(cfg, st, &fakeLookup{numbers: numbers}, cl, sending)
	e.now = func() time.Time { return now }
	e.lookupRetryDelay = time.Millisecond
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

func TestLadder_PersonWithNoNumberIsSkippedAndTheRestCalled(t *testing.T) {
	e, _, cl, now := newTestEngine(t, true)
	ctx := context.Background()
	delete(e.phones.(*fakeLookup).numbers, "b@example.com")
	_ = e.HandleAlert(ctx, alert)
	_ = e.Tick(ctx)
	*now = now.Add(3 * time.Minute)
	_ = e.Tick(ctx)
	if got := strings.Join(cl.calls, ","); got != "+15550000001,+15550000003" {
		t.Fatalf("calls = %q, want the R2 user with a number still called", got)
	}
}

func TestLadder_NotE164NumberIsNotDialled(t *testing.T) {
	e, _, cl, _ := newTestEngine(t, true)
	e.phones.(*fakeLookup).numbers["a@example.com"] = "0771234567"
	_ = e.HandleAlert(context.Background(), alert)
	_ = e.Tick(context.Background())
	if len(cl.calls) != 0 {
		t.Fatalf("calls = %v, want none for a number that is not E.164", cl.calls)
	}
}

func TestLadder_LookupRetriesThenSucceeds(t *testing.T) {
	e, _, cl, _ := newTestEngine(t, true)
	lk := e.phones.(*fakeLookup)
	lk.failFor = lookupAttempts - 1
	_ = e.HandleAlert(context.Background(), alert)
	_ = e.Tick(context.Background())
	if lk.calls != lookupAttempts || len(cl.calls) != 1 {
		t.Fatalf("lookups = %d, calls = %v; want %d lookups and the R1 call", lk.calls, cl.calls, lookupAttempts)
	}
}

func TestLadder_LookupFailureSkipsTheLevelNotTheLadder(t *testing.T) {
	e, _, cl, now := newTestEngine(t, true)
	lk := e.phones.(*fakeLookup)
	lk.failFor = lookupAttempts
	ctx := context.Background()
	_ = e.HandleAlert(ctx, alert)
	_ = e.Tick(ctx)
	if len(cl.calls) != 0 {
		t.Fatalf("calls = %v, want none while the lookup is down", cl.calls)
	}
	*now = now.Add(3 * time.Minute)
	_ = e.Tick(ctx)
	if got := strings.Join(cl.calls, ","); got != "+15550000002,+15550000003" {
		t.Fatalf("calls = %q, want R2 called once the lookup is back", got)
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
