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

package productissues

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeReconciler struct {
	pages    []response
	cursors  []string
	limits   []int
	err      error
	requests int
}

func (f *fakeReconciler) ProductIssueReconcile(_ context.Context, body []byte) ([]byte, error) {
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	f.cursors = append(f.cursors, req.Cursor)
	f.limits = append(f.limits, req.Limit)
	if f.err != nil {
		return nil, f.err
	}
	page := f.pages[f.requests]
	f.requests++
	return json.Marshal(page)
}

func TestRunReconcile_FollowsTheCursorToTheEnd(t *testing.T) {
	f := &fakeReconciler{pages: []response{
		{ItemsChecked: 200, LinksUpdated: 3, NextCursor: "c1", More: true},
		{ItemsChecked: 1, NextCursor: "c2", More: true}, // stopped early, short of time
		{NotOnBoard: 4, NextCursor: "c3", More: true},
		{ItemsChecked: 7},
	}}
	if err := RunReconcile(f)(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if strings.Join(f.cursors, ",") != ",c1,c2,c3" {
		t.Errorf("cursors = %q", f.cursors)
	}
	for _, l := range f.limits {
		if l != pageLimit {
			t.Errorf("limit = %d, want %d", l, pageLimit)
		}
	}
}

func TestRunReconcile_FailuresFailTheRun(t *testing.T) {
	f := &fakeReconciler{pages: []response{
		{ItemsChecked: 5, Failed: 1, NextCursor: "c1", More: true},
		{Failed: 2},
	}}
	err := RunReconcile(f)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "3 board(s) or issue(s) failed") {
		t.Fatalf("err = %v", err)
	}
	if f.requests != 2 {
		t.Errorf("a failure stopped the run early: %d calls", f.requests)
	}
}

type sleeps struct{ waited []time.Duration }

func (s *sleeps) sleep(_ context.Context, d time.Duration) error {
	s.waited = append(s.waited, d)
	return nil
}

func TestRunReconcile_ShortRateLimitIsWaitedOutAndTheRunContinues(t *testing.T) {
	f := &fakeReconciler{pages: []response{
		{ItemsChecked: 5, NextCursor: "c1", More: true},
		{ItemsChecked: 2, NextCursor: "c2", More: true, RetryAfterSeconds: 120},
		{ItemsChecked: 9},
	}}
	s := &sleeps{}
	if err := runReconcile(f, s.sleep)(context.Background()); err != nil {
		t.Fatalf("a short rate limit is not a failure: %v", err)
	}
	if f.requests != 3 || strings.Join(f.cursors, ",") != ",c1,c2" || len(s.waited) != 1 || s.waited[0] != 120*time.Second {
		t.Errorf("calls=%d cursors=%q waited=%v: the run must wait 120 s and go on from c2", f.requests, f.cursors, s.waited)
	}
	// Failures seen before still fail the run.
	f = &fakeReconciler{pages: []response{{Failed: 1, NextCursor: "c1", More: true}, {RetryAfterSeconds: 5, NextCursor: "c2", More: true}, {}}}
	if err := runReconcile(f, (&sleeps{}).sleep)(context.Background()); err == nil {
		t.Error("a failure before the rate limit must still be reported")
	}
}

func TestRunReconcile_LongOrRepeatedRateLimitFailsTheRunWithItsPosition(t *testing.T) {
	cur := base64.RawURLEncoding.EncodeToString([]byte(`{"o":"example-org","b":7,"p":"items"}`))
	// Longer than 15 minutes: no wait at all.
	f := &fakeReconciler{pages: []response{{ItemsChecked: 40, NextCursor: cur, More: true, RetryAfterSeconds: 901}}}
	s := &sleeps{}
	err := runReconcile(f, s.sleep)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "example-org/7") || !strings.Contains(err.Error(), "40 items checked") || len(s.waited) != 0 {
		t.Fatalf("err = %v waited = %v", err, s.waited)
	}
	// Exactly 900 s is waited out; the fourth wait of a run fails.
	f = &fakeReconciler{pages: []response{
		{NextCursor: "c1", More: true, RetryAfterSeconds: 900},
		{ItemsChecked: 1, NextCursor: "c2", More: true, RetryAfterSeconds: 10},
		{ItemsChecked: 1, NextCursor: "c3", More: true, RetryAfterSeconds: 10},
		{ItemsChecked: 1, NextCursor: "c4", More: true, RetryAfterSeconds: 10},
	}}
	s = &sleeps{}
	err = runReconcile(f, s.sleep)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "c4") || len(s.waited) != 3 || s.waited[0] != 900*time.Second || f.requests != 4 {
		t.Fatalf("err = %v waited = %v calls = %d", err, s.waited, f.requests)
	}
}

func TestRunReconcile_WaitEndsWithTheContext(t *testing.T) {
	f := &fakeReconciler{pages: []response{{NextCursor: "c1", More: true, RetryAfterSeconds: 60}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RunReconcile(f)(ctx)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunReconcile_StopsOnAnErrorOrNoProgress(t *testing.T) {
	f := &fakeReconciler{err: errors.New("entity-service down")}
	if err := RunReconcile(f)(context.Background()); err == nil || !strings.Contains(err.Error(), "entity-service down") {
		t.Errorf("err = %v", err)
	}
	f = &fakeReconciler{pages: []response{{}}}
	if err := RunReconcile(f)(context.Background()); err != nil {
		t.Errorf("a first answer with nothing left: err = %v", err)
	}
	for _, next := range []string{"c1", ""} {
		f = &fakeReconciler{pages: []response{{ItemsChecked: 3, NextCursor: "c1", More: true}, {ItemsChecked: 2, NextCursor: next, More: true}}}
		if err := RunReconcile(f)(context.Background()); err == nil || !strings.Contains(err.Error(), "no progress") {
			t.Errorf("cursor %q after a call that moved: err = %v", next, err)
		}
	}
}

// more is the only end signal: a last call with a cursor but more unset ends the run.
func TestRunReconcile_MoreUnsetEndsTheRun(t *testing.T) {
	f := &fakeReconciler{pages: []response{{NextCursor: "c1", More: true}, {NextCursor: "c2"}}}
	if err := RunReconcile(f)(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if f.requests != 2 {
		t.Errorf("calls = %d, want 2", f.requests)
	}
}

func TestRunReconcile_StopsAtTheCallCap(t *testing.T) {
	f := &endlessReconciler{}
	err := RunReconcile(f)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "still more") || f.calls != maxPages {
		t.Fatalf("err = %v after %d calls", err, f.calls)
	}
}

type endlessReconciler struct{ calls int }

func (e *endlessReconciler) ProductIssueReconcile(context.Context, []byte) ([]byte, error) {
	e.calls++
	return json.Marshal(response{ItemsChecked: 1, NextCursor: string(rune('a' + e.calls%2)), More: true, Boards: e.calls})
}
