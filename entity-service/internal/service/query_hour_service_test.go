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
	"context"
	"errors"
	"reflect"
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
}

func (f *fakeQueryHourRepo) ActiveLines(context.Context) ([]repository.QueryHourLineRow, error) {
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
func (f *fakeQueryHourRepo) RecomputeProjectHours(_ context.Context, id string) error {
	f.recomputed = append(f.recomputed, id)
	return f.recomputeErr[id]
}
func (f *fakeQueryHourRepo) ProjectIDsWithActiveLines(context.Context) ([]string, error) {
	return f.projectIDs, nil
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
