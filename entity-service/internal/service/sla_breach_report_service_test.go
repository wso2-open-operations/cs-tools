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
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeSLABreachRepo struct {
	rows []domain.SLABreachRow
	err  error

	gotFrom, gotTo            time.Time
	gotDefs, gotStage         []string
	gotOnboarding, gotExclude []string
	calls                     int
}

func (f *fakeSLABreachRepo) ListBreaches(_ context.Context, from, to time.Time, definitions, stages, onboardingStatuses, excludeOnboardingStatuses []string) ([]domain.SLABreachRow, error) {
	f.calls++
	f.gotFrom, f.gotTo, f.gotDefs, f.gotStage = from, to, definitions, stages
	f.gotOnboarding, f.gotExclude = onboardingStatuses, excludeOnboardingStatuses
	return f.rows, f.err
}

func newSLABreachSvc(repo *fakeSLABreachRepo) SLABreachReportService {
	return NewSLABreachReportService(repo)
}

func mustColomboLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(SLABreachReportTimezone)
	if err != nil {
		t.Skipf("tz database unavailable: %v", err)
	}
	return loc
}

// TestSLABreachReportWeekIsThePreviousWholeWeek pins the window to the
// calendar week before the one the run falls in -- never a trailing seven
// days. Every day of the same week must resolve to the identical window,
// which is what makes a retry or a late re-run reproduce the report
// rather than slide it.
func TestSLABreachReportWeekIsThePreviousWholeWeek(t *testing.T) {
	loc := mustColomboLoc(t)
	wantFrom := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	wantTo := time.Date(2026, 11, 9, 0, 0, 0, 0, loc)

	for day := 9; day <= 15; day++ {
		now := time.Date(2026, 11, day, 13, 30, 0, 0, loc)
		from, to := SLABreachReportWeek(time.Time{}, now, loc)
		if !from.Equal(wantFrom) || !to.Equal(wantTo) {
			t.Fatalf("run on Nov %d: got [%s, %s), want [%s, %s)",
				day, from.Format(time.RFC3339), to.Format(time.RFC3339),
				wantFrom.Format(time.RFC3339), wantTo.Format(time.RFC3339))
		}
	}
}

// TestSLABreachReportWeekExplicitWeekOverridesNow covers the re-send
// path: an explicit week reports that week's predecessor regardless of
// now.
func TestSLABreachReportWeekExplicitWeekOverridesNow(t *testing.T) {
	loc := mustColomboLoc(t)
	week := time.Date(2026, 10, 14, 0, 0, 0, 0, loc)
	now := time.Date(2026, 12, 25, 0, 0, 0, 0, loc)
	from, to := SLABreachReportWeek(week, now, loc)

	wantFrom := time.Date(2026, 10, 5, 0, 0, 0, 0, loc)
	wantTo := time.Date(2026, 10, 12, 0, 0, 0, 0, loc)
	if !from.Equal(wantFrom) || !to.Equal(wantTo) {
		t.Fatalf("got [%s, %s), want [%s, %s)", from, to, wantFrom, wantTo)
	}
}

// TestBuildReportPassesOnboardingFiltersThrough: the two ServiceNow
// reports differ in BOTH stage and onboarding status, and their status
// sets are not complements, so each filter must reach the repository
// independently rather than being derived from the other.
func TestBuildReportPassesOnboardingFiltersThrough(t *testing.T) {
	repo := &fakeSLABreachRepo{}
	_, err := newSLABreachSvc(repo).BuildReport(context.Background(), domain.SLABreachReportRequest{
		Definitions:               []string{"P1 - Response (Open Source)"},
		OnboardingStatuses:        []string{"In-Progress"},
		ExcludeOnboardingStatuses: []string{"In-Progress", "Not-Started"},
	})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if len(repo.gotOnboarding) != 1 || repo.gotOnboarding[0] != "IN_PROGRESS" {
		t.Fatalf("onboardingStatuses = %#v, want [IN_PROGRESS]", repo.gotOnboarding)
	}
	if len(repo.gotExclude) != 2 || repo.gotExclude[0] != "IN_PROGRESS" || repo.gotExclude[1] != "NOT_STARTED" {
		t.Fatalf("excludeOnboardingStatuses = %#v, want [IN_PROGRESS NOT_STARTED]", repo.gotExclude)
	}
}

// TestBuildReportNormalisesServiceNowSpelling: the source writes "In
// progress" for a stage and "In-Progress" for an onboarding status,
// while Postgres stores IN_PROGRESS. A configuration copied verbatim
// from ServiceNow must still match, or the report is silently empty.
func TestBuildReportNormalisesServiceNowSpelling(t *testing.T) {
	repo := &fakeSLABreachRepo{}
	_, err := newSLABreachSvc(repo).BuildReport(context.Background(), domain.SLABreachReportRequest{
		Definitions:        []string{"P1 - Response (Open Source)"},
		Stages:             []string{"In progress"},
		OnboardingStatuses: []string{"In-Progress"},
	})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if len(repo.gotStage) != 1 || repo.gotStage[0] != "IN_PROGRESS" {
		t.Fatalf("stages = %#v, want [IN_PROGRESS]", repo.gotStage)
	}
	if len(repo.gotOnboarding) != 1 || repo.gotOnboarding[0] != "IN_PROGRESS" {
		t.Fatalf("onboardingStatuses = %#v, want [IN_PROGRESS]", repo.gotOnboarding)
	}
}

// TestBuildReportRowsAreNotNil keeps a quiet week serializing as []
// rather than null -- the task renders len() on it unconditionally.
func TestBuildReportRowsAreNotNil(t *testing.T) {
	got, err := newSLABreachSvc(&fakeSLABreachRepo{}).BuildReport(context.Background(), domain.SLABreachReportRequest{
		Definitions: []string{"P1 - Response (Open Source)"},
	})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if got.Rows == nil {
		t.Fatal("Rows must be non-nil so an empty report serializes as []")
	}
}

// TestBuildReportReturnsTheRepositoryRows checks the rows come back in
// the repository's own order, which is the report's grouping order.
func TestBuildReportReturnsTheRepositoryRows(t *testing.T) {
	repo := &fakeSLABreachRepo{rows: []domain.SLABreachRow{
		{Team: "Atlas", CaseNumber: "CS01"},
		{Team: "Castor", CaseNumber: "CS02"},
	}}
	got, err := newSLABreachSvc(repo).BuildReport(context.Background(), domain.SLABreachReportRequest{
		Definitions: []string{"P1 - Response (Open Source)"},
	})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if len(got.Rows) != 2 || got.Rows[0].CaseNumber != "CS01" || got.Rows[1].CaseNumber != "CS02" {
		t.Fatalf("rows = %#v", got.Rows)
	}
}

// TestBuildReportRequiresDefinitions: defaulting to "every policy" would
// silently widen the report well past the ten the source filter named.
func TestBuildReportRequiresDefinitions(t *testing.T) {
	repo := &fakeSLABreachRepo{}
	for _, defs := range [][]string{nil, {}, {"  ", ""}} {
		_, err := newSLABreachSvc(repo).BuildReport(context.Background(), domain.SLABreachReportRequest{Definitions: defs})
		if err == nil {
			t.Fatalf("definitions=%v: want a validation error, got nil", defs)
		}
	}
	if repo.calls != 0 {
		t.Fatalf("repository must not be queried without definitions, got %d calls", repo.calls)
	}
}

// TestBuildReportNormalisesStagesAndPassesFilters checks what actually
// reaches the repository, and that absent status filters stay empty
// rather than being invented.
func TestBuildReportNormalisesStagesAndPassesFilters(t *testing.T) {
	repo := &fakeSLABreachRepo{}
	_, err := newSLABreachSvc(repo).BuildReport(context.Background(), domain.SLABreachReportRequest{
		Definitions: []string{" P1 - Response (Open Source) ", "", "P2 - Resolution (Open Source)"},
		Stages:      []string{" paused ", "", "Completed"},
	})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if len(repo.gotDefs) != 2 || repo.gotDefs[0] != "P1 - Response (Open Source)" {
		t.Fatalf("definitions = %#v", repo.gotDefs)
	}
	if len(repo.gotStage) != 2 || repo.gotStage[0] != "PAUSED" || repo.gotStage[1] != "COMPLETED" {
		t.Fatalf("stages = %#v, want [PAUSED COMPLETED]", repo.gotStage)
	}
	if len(repo.gotOnboarding) != 0 || len(repo.gotExclude) != 0 {
		t.Fatalf("absent status filters must stay empty, got %#v / %#v", repo.gotOnboarding, repo.gotExclude)
	}
	if !repo.gotTo.After(repo.gotFrom) || repo.gotTo.Sub(repo.gotFrom) != 7*24*time.Hour {
		t.Fatalf("window = [%s, %s), want exactly one week", repo.gotFrom, repo.gotTo)
	}
}
