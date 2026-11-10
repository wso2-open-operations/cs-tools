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

package service_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// fakeQueryHourRepo serves fixed rows and records what was written.
type fakeQueryHourRepo struct {
	lines    []repository.QueryHourLineRow
	projects map[string][]repository.QueryHourProjectRow
	states   map[string]*domain.QueryHourState

	projectIDs []string

	wroteState map[string]*domain.QueryHourState
	// recomputed lists, in order, the projects whose hours were recomputed.
	recomputed   []string
	recomputeErr map[string]error
	// recomputeDelay is how long each project takes, cut short when its
	// context is done; onRecompute runs after each project.
	recomputeDelay time.Duration
	onRecompute    func(id string)
}

func (f *fakeQueryHourRepo) ActiveLines(ctx context.Context) ([]repository.QueryHourLineRow, error) {
	// Like a database read, a done context fails the state comparison.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return f.lines, nil
}
func (f *fakeQueryHourRepo) ProjectConsumption(_ context.Context, oppID string, _, _ time.Time) ([]repository.QueryHourProjectRow, error) {
	return f.projects[oppID], nil
}
func (f *fakeQueryHourRepo) OpportunityStates(context.Context) (map[string]*domain.QueryHourState, error) {
	return f.states, nil
}
func (f *fakeQueryHourRepo) SetOpportunityState(_ context.Context, id string, s *domain.QueryHourState) error {
	if f.wroteState == nil {
		f.wroteState = map[string]*domain.QueryHourState{}
	}
	f.wroteState[id] = s
	return nil
}
func (f *fakeQueryHourRepo) RecomputeProjectHours(ctx context.Context, id string) error {
	f.recomputed = append(f.recomputed, id)
	if f.onRecompute != nil {
		defer f.onRecompute(id)
	}
	if f.recomputeDelay > 0 {
		select {
		case <-time.After(f.recomputeDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.recomputeErr[id]
}
func (f *fakeQueryHourRepo) ProjectIDsWithLinkedLines(context.Context) ([]string, error) {
	return f.projectIDs, nil
}
func (f *fakeQueryHourRepo) SiblingProjectIDs(context.Context, string) ([]string, error) {
	return nil, nil
}

func qhLine(product string, qty float64) repository.QueryHourLineRow {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	return repository.QueryHourLineRow{
		OpportunityID: "opp-1", SalesforceID: "006X", Name: "Renewal",
		AccountID: "acc-1", AccountName: "Acme",
		AccountManagerEmail: "am@example.com", TechnicalOwnerEmail: "to@example.com",
		ProductName: product, Quantity: qty, ServiceStart: start, ServiceEnd: end,
	}
}

// A product name the mapping does not know must be COUNTED, not
// silently treated as zero hours: a renamed Salesforce SKU would
// otherwise reduce an entitlement to nothing with no signal.
func TestQueryHourUnmatchedProductIsCounted(t *testing.T) {
	repo := &fakeQueryHourRepo{lines: []repository.QueryHourLineRow{
		qhLine("Development Support - 100 hours", 1),
		qhLine("Development Support - 75 hours", 1), // not a real tier
	}}
	repo.projects = map[string][]repository.QueryHourProjectRow{
		"opp-1": {{ProjectID: "p1", ProjectKey: "ACME", BillableMinutes: 100 * 60}},
	}
	rep, err := service.NewQueryHourService(repo, "@example.com").WeeklyReport(context.Background())
	if err != nil {
		t.Fatalf("WeeklyReport: %v", err)
	}
	var opp *domain.QueryHourOpportunity
	for _, a := range append(rep.Exceeded, rep.CloseToExceed...) {
		for i := range a.Opportunities {
			opp = &a.Opportunities[i]
		}
	}
	if opp == nil {
		t.Fatal("the opportunity consumed its whole allowance and should be reported")
	}
	if opp.UnmatchedLines != 1 {
		t.Errorf("unmatchedLines = %d, want 1", opp.UnmatchedLines)
	}
	if opp.TotalMinutes != 100*60 {
		t.Errorf("total = %d, want %d -- the unknown line grants nothing", opp.TotalMinutes, 100*60)
	}
}

// The hourly recompute has no arithmetic of its own for project hours: it
// runs the shared per-project function for every project with a line in
// service, and a failing project neither stops the others nor the state
// comparison (so the notifications) that follows.
func TestQueryHourRecomputeDelegatesProjectHours(t *testing.T) {
	repo := &fakeQueryHourRepo{
		projectIDs:   []string{"p1", "p2", "p3"},
		recomputeErr: map[string]error{"p2": errors.New("boom")},
		lines:        []repository.QueryHourLineRow{qhLine("Development Support - 100 hours", 1)},
		projects: map[string][]repository.QueryHourProjectRow{
			"opp-1": {{ProjectID: "p1", ProjectKey: "ACME", BillableMinutes: 100 * 60}},
		},
	}
	transitions, err := service.NewQueryHourService(repo, "@example.com").Recompute(context.Background())
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if want := []string{"p1", "p2", "p3"}; !reflect.DeepEqual(repo.recomputed, want) {
		t.Errorf("recomputed = %v, want %v", repo.recomputed, want)
	}
	if len(transitions) != 1 {
		t.Errorf("transitions = %d, want 1: a failed project must not suppress the state comparison", len(transitions))
	}
}

// Recompute alone detects crossings: it writes the state, the per-project
// function (used by the other triggers) must not.
// The hourly job logs a summary and keeps going past a failing project; the
// summary line carries the project count, failures and the slowest project.
func TestQueryHourRecomputeSummaryLogsCountsAndSlowest(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(old)

	repo := &fakeQueryHourRepo{projectIDs: []string{"p1", "p2", "p3"}, recomputeErr: map[string]error{"p2": errors.New("boom")}}
	if _, err := service.NewQueryHourService(repo, "@example.com").Recompute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repo.recomputed, []string{"p1", "p2", "p3"}) {
		t.Errorf("recomputed = %v, a failing project must not stop the rest", repo.recomputed)
	}
	out := buf.String()
	for _, want := range []string{"projects=3", "failures=1", "slowestProjectId=", "totalMs=", "durationMs="} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q:\n%s", want, out)
		}
	}
}

// A request context that is already done stops the loop instead of failing
// every remaining project one by one.
func TestQueryHourRecomputeStopsWhenContextDone(t *testing.T) {
	repo := &fakeQueryHourRepo{projectIDs: []string{"p1", "p2"}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = service.NewQueryHourService(repo, "@example.com").Recompute(ctx)
	if len(repo.recomputed) != 0 {
		t.Errorf("recomputed %v after the context was done", repo.recomputed)
	}
}

// Project hours get only part of the request's time: when they would use it
// all, the loop stops early and the state comparison still runs, so the
// usage notifications still go out.
func TestQueryHourRecomputeReservesTimeForStateComparison(t *testing.T) {
	repo := &fakeQueryHourRepo{
		projectIDs:     []string{"p1", "p2", "p3", "p4", "p5"},
		recomputeDelay: 250 * time.Millisecond,
		lines:          []repository.QueryHourLineRow{qhLine("Development Support - 100 hours", 1)},
		projects: map[string][]repository.QueryHourProjectRow{
			"opp-1": {{ProjectID: "p1", ProjectKey: "ACME", BillableMinutes: 100 * 60}},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	transitions, err := service.NewQueryHourService(repo, "@example.com").Recompute(ctx)
	if err != nil {
		t.Fatalf("Recompute: %v -- the state comparison must keep its share of the budget", err)
	}
	if len(transitions) != 1 {
		t.Errorf("transitions = %d, want 1", len(transitions))
	}
	if n := len(repo.recomputed); n == 0 || n >= len(repo.projectIDs) {
		t.Errorf("recomputed %v, want the loop cut short by its budget", repo.recomputed)
	}
}

// A run cut short leaves a cursor: the next run starts after the last project
// it reached, so the projects at the end of the list are not starved.
func TestQueryHourRecomputeResumesAfterLastProjectReached(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := &fakeQueryHourRepo{projectIDs: []string{"p1", "p2", "p3", "p4"}}
	repo.onRecompute = func(id string) {
		if id == "p2" {
			cancel()
		}
	}
	svc := service.NewQueryHourService(repo, "@example.com")
	_, _ = svc.Recompute(ctx)
	if want := []string{"p1", "p2"}; !reflect.DeepEqual(repo.recomputed, want) {
		t.Fatalf("first run recomputed %v, want %v", repo.recomputed, want)
	}
	repo.recomputed, repo.onRecompute = nil, nil
	if _, err := svc.Recompute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"p3", "p4", "p1", "p2"}; !reflect.DeepEqual(repo.recomputed, want) {
		t.Errorf("second run recomputed %v, want %v", repo.recomputed, want)
	}
	// The cursor follows ids, not positions: a project that drops off the
	// list does not shift where the next run starts.
	repo.recomputed, repo.projectIDs = nil, []string{"p1", "p3"}
	if _, err := svc.Recompute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"p3", "p1"}; !reflect.DeepEqual(repo.recomputed, want) {
		t.Errorf("third run recomputed %v, want %v (resume after p2)", repo.recomputed, want)
	}
}

// A project the budget skipped still has its old onboarding figure stored.
// Its opportunity gets no state write and no notice from that figure; an
// opportunity whose projects were all recomputed is compared as usual.
func TestQueryHourRecomputeDefersOpportunityOfSkippedProject(t *testing.T) {
	fresh := qhLine("Development Support - 100 hours", 1)
	stale := qhLine("Development Support - 100 hours", 1)
	stale.OpportunityID, stale.SalesforceID = "opp-2", "006Y"
	repo := &fakeQueryHourRepo{
		projectIDs:     []string{"p1", "p9"},
		recomputeDelay: 250 * time.Millisecond,
		lines:          []repository.QueryHourLineRow{fresh, stale},
		projects: map[string][]repository.QueryHourProjectRow{
			"opp-1": {{ProjectID: "p1", ProjectKey: "ACME", BillableMinutes: 100 * 60}},
			// p9's stored onboarding figure predates time it has since
			// consumed: without it, the pool looks exhausted.
			"opp-2": {{ProjectID: "p9", ProjectKey: "GLOBEX", BillableMinutes: 100 * 60, OnboardingMinutes: 0}},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	transitions, err := service.NewQueryHourService(repo, "@example.com").Recompute(ctx)
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if want := []string{"p1", "p9"}; !reflect.DeepEqual(repo.recomputed, want) {
		t.Fatalf("recomputed = %v, want %v (p9 cut off by the budget)", repo.recomputed, want)
	}
	if _, ok := repo.wroteState["opp-2"]; ok {
		t.Errorf("wrote state for opp-2 from a skipped project's stale figure")
	}
	if _, ok := repo.wroteState["opp-1"]; !ok {
		t.Errorf("opp-1 state not written: only opportunities with a skipped project are deferred")
	}
	for _, tr := range transitions {
		if tr.ProjectID == "p9" {
			t.Errorf("notified %+v from a skipped project's stale figure", tr)
		}
	}
	if len(transitions) != 1 {
		t.Errorf("transitions = %d, want 1 (opp-1 only)", len(transitions))
	}
}

// A project cut off mid-recompute by the budget is skipped, not failed, and
// the next run starts with it rather than putting it last.
func TestQueryHourRecomputeCutOffProjectRunsFirstNextTime(t *testing.T) {
	repo := &fakeQueryHourRepo{projectIDs: []string{"p1", "p2", "p3"}, recomputeDelay: 250 * time.Millisecond}
	svc := service.NewQueryHourService(repo, "@example.com")
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	_, _ = svc.Recompute(ctx)
	if want := []string{"p1", "p2"}; !reflect.DeepEqual(repo.recomputed, want) {
		t.Fatalf("first run recomputed %v, want %v", repo.recomputed, want)
	}
	repo.recomputed, repo.recomputeDelay = nil, 0
	if _, err := svc.Recompute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"p2", "p3", "p1"}; !reflect.DeepEqual(repo.recomputed, want) {
		t.Errorf("second run recomputed %v, want %v", repo.recomputed, want)
	}
}

// A project that fails on its own does not defer its opportunity: one that
// keeps failing must not silence that opportunity's notices for good.
func TestQueryHourRecomputeFailedProjectDoesNotDefer(t *testing.T) {
	repo := &fakeQueryHourRepo{
		projectIDs:   []string{"p1"},
		recomputeErr: map[string]error{"p1": errors.New("boom")},
		lines:        []repository.QueryHourLineRow{qhLine("Development Support - 100 hours", 1)},
		projects: map[string][]repository.QueryHourProjectRow{
			"opp-1": {{ProjectID: "p1", ProjectKey: "ACME", BillableMinutes: 100 * 60}},
		},
	}
	transitions, err := service.NewQueryHourService(repo, "@example.com").Recompute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(transitions) != 1 {
		t.Errorf("transitions = %d, want 1", len(transitions))
	}
}

func TestQueryHourRecomputeProjectHoursDoesNotTouchState(t *testing.T) {
	repo := &fakeQueryHourRepo{}
	svc := service.NewQueryHourService(repo, "@example.com")
	if err := svc.RecomputeProjectHours(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	if len(repo.wroteState) != 0 {
		t.Errorf("wrote state %v", repo.wroteState)
	}
	if !reflect.DeepEqual(repo.recomputed, []string{"p1"}) {
		t.Errorf("recomputed = %v", repo.recomputed)
	}
}

// The thresholds, at their exact boundaries.
func TestQueryHourThresholds(t *testing.T) {
	const total = 100 * 60 // 6000 minutes
	for _, tc := range []struct {
		name            string
		consumed        int
		wantState       string
		wantExceeded    bool
		wantCloseToEdge bool
	}{
		{"under 75%", 4499, "", false, false},
		{"exactly 75%", 4500, "NOTIFIED_75", false, false},
		{"exactly 90%", 5400, "NOTIFIED_90", false, true},
		{"exactly 100%", 6000, "CLOSURE_NOTICE_100", false, true},
		{"over 100%", 6001, "CLOSURE_NOTICE_100", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeQueryHourRepo{
				lines: []repository.QueryHourLineRow{qhLine("Development Support - 100 hours", 1)},
				projects: map[string][]repository.QueryHourProjectRow{"opp-1": {{
					ProjectID: "p1", ProjectKey: "ACME", BillableMinutes: tc.consumed,
				}}},
			}
			if _, err := service.NewQueryHourService(repo, "@example.com").Recompute(context.Background()); err != nil {
				t.Fatalf("Recompute: %v", err)
			}
			got := repo.wroteState["opp-1"]
			var gotStr string
			if got != nil {
				gotStr = string(*got)
			}
			if gotStr != tc.wantState {
				t.Errorf("state = %q, want %q", gotStr, tc.wantState)
			}
		})
	}

	// Remaining under ten hours but not yet overdrawn is "close to
	// exceeding" -- ServiceNow's `remaining < 600` minutes.
	repo := &fakeQueryHourRepo{
		lines: []repository.QueryHourLineRow{qhLine("Development Support - 100 hours", 1)},
		projects: map[string][]repository.QueryHourProjectRow{"opp-1": {{
			ProjectID: "p1", ProjectKey: "ACME", BillableMinutes: 6000 - 599,
		}}},
	}
	rep, err := service.NewQueryHourService(repo, "@example.com").WeeklyReport(context.Background())
	if err != nil {
		t.Fatalf("WeeklyReport: %v", err)
	}
	if len(rep.CloseToExceed) != 1 || len(rep.Exceeded) != 0 {
		t.Errorf("599 minutes left: exceeded=%d closeTo=%d, want 0 and 1",
			len(rep.Exceeded), len(rep.CloseToExceed))
	}
}

// Only a RISE notifies. ServiceNow's trigger is "state changes TO 1, 2
// or 3", so a fall, or no change, tells nobody.
func TestQueryHourOnlyRisesNotify(t *testing.T) {
	n90 := domain.QueryHourStateNotified90
	n75 := domain.QueryHourStateNotified75
	for _, tc := range []struct {
		name     string
		previous *domain.QueryHourState
		consumed int
		want     int
	}{
		{"nothing to 90% notifies", nil, 5400, 1},
		{"75% to 90% notifies", &n75, 5400, 1},
		{"90% unchanged is silent", &n90, 5400, 0},
		{"90% down to 75% is silent", &n90, 4500, 0},
		{"90% down to nothing is silent", &n90, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeQueryHourRepo{
				lines:  []repository.QueryHourLineRow{qhLine("Development Support - 100 hours", 1)},
				states: map[string]*domain.QueryHourState{"opp-1": tc.previous},
				projects: map[string][]repository.QueryHourProjectRow{"opp-1": {{
					ProjectID: "p1", ProjectKey: "ACME", BillableMinutes: tc.consumed,
				}}},
			}
			got, err := service.NewQueryHourService(repo, "@example.com").Recompute(context.Background())
			if err != nil {
				t.Fatalf("Recompute: %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("transitions = %d, want %d", len(got), tc.want)
			}
		})
	}
}

// The audience is derived from the accounts in BOTH tables, restricted
// to internal addresses and deduped. An earlier reading of this port
// took only the exceeded accounts, which is why its derived list came
// out roughly a third short.
func TestQueryHourRecipientsSpanBothTables(t *testing.T) {
	exceeded := qhLine("Development Support - 100 hours", 1)
	closeTo := qhLine("Development Support - 100 hours", 1)
	closeTo.OpportunityID = "opp-2"
	closeTo.AccountID = "acc-2"
	closeTo.AccountName = "Beta"
	closeTo.AccountManagerEmail = "beta-am@example.com"
	closeTo.TechnicalOwnerEmail = "customer@elsewhere.com" // not internal

	repo := &fakeQueryHourRepo{
		lines: []repository.QueryHourLineRow{exceeded, closeTo},
		projects: map[string][]repository.QueryHourProjectRow{
			"opp-1": {{ProjectID: "p1", BillableMinutes: 7000}},      // exceeded
			"opp-2": {{ProjectID: "p2", BillableMinutes: 6000 - 60}}, // close
		},
	}
	rep, err := service.NewQueryHourService(repo, "@example.com").WeeklyReport(context.Background())
	if err != nil {
		t.Fatalf("WeeklyReport: %v", err)
	}
	if len(rep.Exceeded) != 1 || len(rep.CloseToExceed) != 1 {
		t.Fatalf("exceeded=%d closeTo=%d, want one each", len(rep.Exceeded), len(rep.CloseToExceed))
	}
	want := []string{"am@example.com", "beta-am@example.com", "to@example.com"}
	if len(rep.Recipients) != len(want) {
		t.Fatalf("recipients = %v, want %v", rep.Recipients, want)
	}
	for i := range want {
		if rep.Recipients[i] != want[i] {
			t.Errorf("recipients = %v, want %v", rep.Recipients, want)
			break
		}
	}
	for _, r := range rep.Recipients {
		if r == "customer@elsewhere.com" {
			t.Error("an external owner must never receive an internal report")
		}
	}
}

func TestFormatQueryHourMinutes(t *testing.T) {
	for in, want := range map[int]string{
		0: "0h 0m", 59: "0h 59m", 60: "1h 0m", 6000: "100h 0m", -90: "-1h 30m",
	} {
		if got := service.FormatQueryHourMinutes(in); got != want {
			t.Errorf("FormatQueryHourMinutes(%d) = %q, want %q", in, got, want)
		}
	}
}
