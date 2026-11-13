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

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The refresh's two promises: one customer's failure never keeps the others'
// snapshot from being published, and a page answers inside the timeouts.

func sgCaptureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// sgManyCustomers is n customers, each with two closed change requests.
func sgManyCustomers(n int) *fakeSuggestionRepo {
	repo := &fakeSuggestionRepo{}
	for i := 0; i < n; i++ {
		for j := 0; j < 2; j++ {
			c := sgCand(100+2*i+j, "Restart gateway", "CLOSED", sgPlan)
			c.AccountID = fmt.Sprintf("cccccccc-0000-4000-8000-%012d", i)
			repo.cands = append(repo.cands, c)
		}
	}
	return repo
}

func TestStandardChangeSuggestionService_Refresh_ACustomerThatFailsIsSkipped(t *testing.T) {
	for name, tc := range map[string]struct {
		break_ func(*fakeSuggestionRepo)
		cause  string
	}{
		"its change requests cannot be read": {func(r *fakeSuggestionRepo) {
			r.fail = map[string]error{sgAccountA: errors.New("read candidates: cannot decode a plan")}
		}, "cannot decode a plan"},
		"its computation panics": {func(r *fakeSuggestionRepo) { r.panics = map[string]bool{sgAccountA: true} }, "index out of range"},
		"its save fails": {func(r *fakeSuggestionRepo) {
			r.saveFail = map[string]error{sgAccountA: errors.New("save suggestions: value too long")}
		}, "value too long"},
	} {
		t.Run(name, func(t *testing.T) {
			logs := sgCaptureLog(t)
			repo := sgFixture()
			tc.break_(repo)
			svc := sgService(repo, true)
			pages, last := refreshAll(t, svc, 1) // fails the test if any page errors
			if pages != 2 || last.Totals == nil ||
				*last.Totals != (domain.StandardChangeSuggestionScan{Customers: 1, ChangeRequests: 2, Clusters: 1, Skipped: 1}) {
				t.Fatalf("pages = %d, totals = %+v; want B computed and A skipped, published", pages, last.Totals)
			}
			resp, err := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{IncludeIneligible: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(resp.SkippedCustomers) != 1 || resp.SkippedCustomers[0].Customer.ID != sgAccountA || resp.SkippedCustomers[0].Reason != suggestionSkippedError {
				t.Errorf("skippedCustomers = %+v, want A with the error reason", resp.SkippedCustomers)
			}
			if len(resp.Suggestions) != 1 || resp.Suggestions[0].Customer.ID != sgAccountB {
				t.Errorf("suggestions = %+v, want B's", resp.Suggestions)
			}
			out := logs.String()
			if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "accountId="+sgAccountA) || !strings.Contains(out, tc.cause) {
				t.Errorf("log = %s\nwant an ERROR naming A and the cause %q", out, tc.cause)
			}
		})
	}
}

func TestStandardChangeSuggestionService_Refresh_ErrorsThatAreNotTheCustomers(t *testing.T) {
	t.Run("a run no longer open (409) stops the page", func(t *testing.T) {
		repo := sgFixture()
		repo.saveFail = map[string]error{sgAccountA: &apierror.ConflictError{Msg: "gone"}}
		_, err := sgService(repo, true).Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{})
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) {
			t.Fatalf("err = %v, want the 409", err)
		}
		if n := len(repo.skipped[firstRunID(repo)]); n != 0 {
			t.Errorf("%d skips recorded, want none", n)
		}
	})
	t.Run("a skip that cannot be recorded fails the page", func(t *testing.T) {
		repo := sgFixture()
		repo.fail = map[string]error{sgAccountA: errors.New("connection refused")}
		repo.skipFail = errors.New("skip customer: connection refused")
		_, err := sgService(repo, true).Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{})
		if err == nil || !strings.Contains(err.Error(), "skip customer") {
			t.Fatalf("err = %v, want the failed skip", err)
		}
	})
}

func TestStandardChangeSuggestionService_Refresh_MostCustomersFailing(t *testing.T) {
	t.Run("is not published, and readers keep the previous snapshot", func(t *testing.T) {
		logs := sgCaptureLog(t)
		repo := sgFixture()
		svc := sgService(repo, true)
		refreshAll(t, svc, 0)
		before, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})

		broken := errors.New("read candidates: column cr.plan does not exist")
		repo.fail = map[string]error{sgAccountA: broken, sgAccountB: broken}
		_, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{})
		if err == nil || !strings.Contains(err.Error(), "2 of the run's 2 customers failed") || !strings.Contains(err.Error(), "not published") {
			t.Fatalf("err = %v, want the run refused", err)
		}
		after, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
		if after.ComputedOn == nil || !after.ComputedOn.Equal(*before.ComputedOn) || len(after.Suggestions) != len(before.Suggestions) || len(after.SkippedCustomers) != 0 {
			t.Errorf("after the refused run: %+v; want the previous snapshot unchanged", after)
		}
		if !strings.Contains(logs.String(), "most customers failed") {
			t.Errorf("log = %s, want the refusal logged", logs.String())
		}
	})
	t.Run("half of them failing is still published", func(t *testing.T) {
		repo := sgManyCustomers(4)
		repo.fail = map[string]error{repo.cands[0].AccountID: errors.New("x"), repo.cands[2].AccountID: errors.New("y")}
		_, last := refreshAll(t, sgService(repo, true), 0)
		if last.Totals == nil || last.Totals.Skipped != 2 || last.Totals.Customers != 2 {
			t.Fatalf("totals = %+v, want 2 computed and 2 skipped, published", last.Totals)
		}
	})
	t.Run("overruns do not count: they are each customer's own size", func(t *testing.T) {
		old := suggestionCustomerTimeout
		suggestionCustomerTimeout = 20 * time.Millisecond
		t.Cleanup(func() { suggestionCustomerTimeout = old })
		repo := sgFixture()
		repo.slow = map[string]bool{sgAccountA: true}
		repo.fail = map[string]error{sgAccountB: errors.New("bad row")}
		svc := sgService(repo, true)
		_, last := refreshAll(t, svc, 0)
		if last.Totals == nil || last.Totals.Skipped != 2 {
			t.Fatalf("totals = %+v, want both skipped and the run published", last.Totals)
		}
		resp, _ := svc.GetSuggestions(context.Background(), domain.StandardChangeSuggestionsRequest{})
		if len(resp.SkippedCustomers) != 2 || resp.SkippedCustomers[0].Reason != suggestionSkippedTooLarge || resp.SkippedCustomers[1].Reason != suggestionSkippedError {
			t.Errorf("skippedCustomers = %+v, want A too large and B an error", resp.SkippedCustomers)
		}
	})
}

func TestStandardChangeSuggestionService_Refresh_PageTiming(t *testing.T) {
	t.Run("the numbers fit inside the request and the caller's timeouts", func(t *testing.T) {
		// The scheduled task's per-call timeout (callTimeout in
		// operations/csm-scheduled-tasks) is 60s, the same as REQUEST_TIMEOUT.
		const callerTimeout = 60 * time.Second
		if margin := min(config.DefaultRequestTimeout, callerTimeout) - suggestionPageTimeout; margin < 15*time.Second {
			t.Errorf("a page may take %v: only %v inside the 60s timeouts", suggestionPageTimeout, margin)
		}
		if need := suggestionCustomerTimeout + 2*suggestionWriteTimeout; need > suggestionPageTimeout {
			t.Errorf("one customer needs %v, more than a page's %v", need, suggestionPageTimeout)
		}
		if suggestionPublishTimeout >= suggestionPageTimeout {
			t.Errorf("the publish's %v does not fit a page's %v", suggestionPublishTimeout, suggestionPageTimeout)
		}
	})

	t.Run("worst case: every customer takes its whole deadline and both writes", func(t *testing.T) {
		repo := sgManyCustomers(10)
		svc := sgService(repo, true)
		clock := sgNow
		svc.now = func() time.Time { return clock }
		var starts []time.Duration
		repo.onRead = func(string) { starts = append(starts, clock.Sub(sgNow)); clock = clock.Add(suggestionCustomerTimeout) }
		repo.saveFail = map[string]error{}
		for _, c := range repo.cands {
			repo.saveFail[c.AccountID] = errors.New("slow, then failed")
		}
		repo.onSave = func(string) { clock = clock.Add(suggestionWriteTimeout) }
		repo.onSkip = func(string) { clock = clock.Add(suggestionWriteTimeout) }
		resp, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{})
		if err != nil || !resp.More {
			t.Fatalf("resp = %+v, %v", resp, err)
		}
		if took := clock.Sub(sgNow); took > suggestionPageTimeout {
			t.Errorf("the page took %v, more than its %v", took, suggestionPageTimeout)
		}
		for i, at := range starts {
			if at+suggestionCustomerTimeout+2*suggestionWriteTimeout > suggestionPageTimeout {
				t.Errorf("customer %d started at %v, without the time for its deadline and writes", i+1, at)
			}
		}
		if len(starts) != 1 || resp.Skipped != 1 {
			t.Errorf("customers started at %v (skipped %d); want one: the second has not the time", starts, resp.Skipped)
		}
	})

	t.Run("customers keep starting while one fits, never after", func(t *testing.T) {
		repo := sgManyCustomers(10)
		svc := sgService(repo, true)
		clock := sgNow
		svc.now = func() time.Time { return clock }
		var starts []time.Duration
		repo.onRead = func(string) { starts = append(starts, clock.Sub(sgNow)); clock = clock.Add(4 * time.Second) }
		repo.onSave = func(string) { clock = clock.Add(time.Second) }
		resp, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{})
		if err != nil || !resp.More || resp.Customers != 4 {
			t.Fatalf("resp = %+v, %v; want four customers (started at 0, 5, 10 and 15s)", resp, err)
		}
		last := starts[len(starts)-1]
		if last > suggestionPageTimeout-suggestionCustomerTimeout-2*suggestionWriteTimeout {
			t.Errorf("the last customer started at %v", last)
		}
	})

	t.Run("a hard stop at the page's deadline, in real time", func(t *testing.T) {
		oldPage, oldCustomer, oldWrite := suggestionPageTimeout, suggestionCustomerTimeout, suggestionWriteTimeout
		suggestionPageTimeout, suggestionCustomerTimeout, suggestionWriteTimeout = 300*time.Millisecond, 100*time.Millisecond, 20*time.Millisecond
		t.Cleanup(func() {
			suggestionPageTimeout, suggestionCustomerTimeout, suggestionWriteTimeout = oldPage, oldCustomer, oldWrite
		})
		repo := sgManyCustomers(10)
		repo.slow = map[string]bool{}
		for _, c := range repo.cands {
			repo.slow[c.AccountID] = true
		}
		svc := sgService(repo, true)
		svc.now = time.Now
		started := time.Now()
		resp, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{})
		took := time.Since(started)
		if err != nil || !resp.More || resp.Skipped == 0 || resp.Skipped > 2 {
			t.Fatalf("resp = %+v, %v; want one or two customers skipped as too large, then more", resp, err)
		}
		if took > suggestionPageTimeout+100*time.Millisecond {
			t.Errorf("the page took %v, past its %v", took, suggestionPageTimeout)
		}
	})

	t.Run("a slow start leaves the first page to start the run only", func(t *testing.T) {
		repo := sgFixture()
		svc := sgService(repo, true)
		clock := sgNow
		svc.now = func() time.Time { return clock }
		repo.onStart = func() { clock = clock.Add(suggestionPageTimeout - suggestionCustomerTimeout) }
		first, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{})
		if err != nil || !first.More || first.RunID == "" || first.NextCursor != "" || first.Customers != 0 || len(repo.loaded) != 0 {
			t.Fatalf("first = %+v, %v, read %v; want the run started and no customer", first, err, repo.loaded)
		}
		req := domain.RefreshStandardChangeSuggestionsRequest{RunID: first.RunID}
		for {
			resp, err := svc.Refresh(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if !resp.More {
				if resp.Totals == nil || resp.Totals.Customers != 2 {
					t.Fatalf("totals = %+v", resp.Totals)
				}
				return
			}
			req = domain.RefreshStandardChangeSuggestionsRequest{RunID: resp.RunID, Cursor: resp.NextCursor}
		}
	})

	t.Run("a publish that might not fit is left to the next call", func(t *testing.T) {
		repo := &fakeSuggestionRepo{cands: sgGroup(2, "Restart gateway")} // customer A only
		svc := sgService(repo, true)
		clock := sgNow
		svc.now = func() time.Time { return clock }
		repo.onRead = func(string) { clock = clock.Add(suggestionPageTimeout - suggestionPublishTimeout + time.Second) }
		first, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{})
		if err != nil || !first.More || first.Customers != 1 || first.NextCursor != sgAccountA || first.ComputedOn != nil {
			t.Fatalf("first = %+v, %v; want A computed and the publish left for later", first, err)
		}
		last, err := svc.Refresh(context.Background(), domain.RefreshStandardChangeSuggestionsRequest{RunID: first.RunID, Cursor: first.NextCursor})
		if err != nil || last.More || last.ComputedOn == nil || last.Customers != 0 || last.Totals.Customers != 1 {
			t.Fatalf("last = %+v, %v; want it published", last, err)
		}
	})

	t.Run("a request deadline too short for a customer starts none", func(t *testing.T) {
		logs := sgCaptureLog(t)
		repo := sgFixture()
		svc := sgService(repo, true)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		resp, err := svc.Refresh(ctx, domain.RefreshStandardChangeSuggestionsRequest{})
		if err != nil || !resp.More || resp.Customers+resp.Skipped != 0 || len(repo.loaded) != 0 {
			t.Fatalf("resp = %+v, %v, read %v; want no customer started", resp, err, repo.loaded)
		}
		if !strings.Contains(logs.String(), "no time left on this page for a customer") {
			t.Errorf("log = %s, want a warning", logs.String())
		}
	})
}

var _ repository.StandardChangeSuggestionRepository = (*fakeSuggestionRepo)(nil)
