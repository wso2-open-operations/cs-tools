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

	gotFrom, gotTo    time.Time
	gotDefs, gotStage []string
	calls             int
}

func (f *fakeSLABreachRepo) ListBreaches(_ context.Context, from, to time.Time, definitions, stages []string) ([]domain.SLABreachRow, error) {
	f.calls++
	f.gotFrom, f.gotTo, f.gotDefs, f.gotStage = from, to, definitions, stages
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
// days. Every weekday of the same week must resolve to the identical
// window, which is what makes a retry or a late re-run reproduce the
// report rather than slide it.
func TestSLABreachReportWeekIsThePreviousWholeWeek(t *testing.T) {
	loc := mustColomboLoc(t)
	wantFrom := time.Date(2026, 11, 2, 0, 0, 0, 0, loc) // Mon
	wantTo := time.Date(2026, 11, 9, 0, 0, 0, 0, loc)   // Mon

	// Monday 9th through Sunday 15th -- every day of the week after.
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

// TestSLABreachReportWeekExplicitWeekOverridesNow covers the re-send path:
// an explicit week reports that week's predecessor regardless of now.
func TestSLABreachReportWeekExplicitWeekOverridesNow(t *testing.T) {
	loc := mustColomboLoc(t)
	week := time.Date(2026, 10, 14, 0, 0, 0, 0, loc) // a Wednesday
	now := time.Date(2026, 12, 25, 0, 0, 0, 0, loc)  // months later
	from, to := SLABreachReportWeek(week, now, loc)

	wantFrom := time.Date(2026, 10, 5, 0, 0, 0, 0, loc)
	wantTo := time.Date(2026, 10, 12, 0, 0, 0, 0, loc)
	if !from.Equal(wantFrom) || !to.Equal(wantTo) {
		t.Fatalf("got [%s, %s), want [%s, %s)", from, to, wantFrom, wantTo)
	}
}

// TestBuildReportSplitsOnOnboardingStatus is the report's whole reason for
// returning two lists: IN_PROGRESS/NOT_STARTED are the onboarding team's,
// everything else -- including an empty status -- is the CRE team's.
func TestBuildReportSplitsOnOnboardingStatus(t *testing.T) {
	repo := &fakeSLABreachRepo{rows: []domain.SLABreachRow{
		{CaseNumber: "CS01", OnboardingStatus: "IN_PROGRESS"},
		{CaseNumber: "CS02", OnboardingStatus: "COMPLETED"},
		{CaseNumber: "CS03", OnboardingStatus: "NOT_STARTED"},
		{CaseNumber: "CS04", OnboardingStatus: ""},
		{CaseNumber: "CS05", OnboardingStatus: "ON_HOLD"},
	}}
	got, err := newSLABreachSvc(repo).BuildReport(context.Background(), domain.SLABreachReportRequest{
		Definitions: []string{"P1 - Response (Open Source)"},
	})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	onboarding := []string{}
	for _, r := range got.Onboarding {
		onboarding = append(onboarding, r.CaseNumber)
	}
	cre := []string{}
	for _, r := range got.CRE {
		cre = append(cre, r.CaseNumber)
	}
	if len(onboarding) != 2 || onboarding[0] != "CS01" || onboarding[1] != "CS03" {
		t.Fatalf("onboarding = %v, want [CS01 CS03]", onboarding)
	}
	if len(cre) != 3 || cre[0] != "CS02" || cre[1] != "CS04" || cre[2] != "CS05" {
		t.Fatalf("cre = %v, want [CS02 CS04 CS05]", cre)
	}
}

// TestBuildReportEmptySectionsAreNotNil keeps a quiet week serializing as
// [] rather than null -- the task renders len() on both unconditionally.
func TestBuildReportEmptySectionsAreNotNil(t *testing.T) {
	got, err := newSLABreachSvc(&fakeSLABreachRepo{}).BuildReport(context.Background(), domain.SLABreachReportRequest{
		Definitions: []string{"P1 - Response (Open Source)"},
	})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if got.Onboarding == nil || got.CRE == nil {
		t.Fatalf("sections must be non-nil, got onboarding=%v cre=%v", got.Onboarding, got.CRE)
	}
}

// TestBuildReportRequiresDefinitions: defaulting to "every policy" would
// silently widen the report well past the ten it is meant to cover.
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
// reaches the repository: trimmed, upper-cased stages and trimmed
// definitions.
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
	if !repo.gotTo.After(repo.gotFrom) || repo.gotTo.Sub(repo.gotFrom) != 7*24*time.Hour {
		t.Fatalf("window = [%s, %s), want exactly one week", repo.gotFrom, repo.gotTo)
	}
}
