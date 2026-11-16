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
	"errors"
	"math"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const slaReportTestProjectID = "48888888-0000-0000-0000-000000000001"

type fakeSlaReportRepo struct {
	project    *domain.SlaReportProject
	rows       []repository.SlaReportClockRow
	clocksErr  error
	gotFrom    time.Time
	gotTo      time.Time
	clocksRuns int
}

func (f *fakeSlaReportRepo) Project(context.Context, string) (domain.SlaReportProject, error) {
	if f.project == nil {
		return domain.SlaReportProject{}, &apierror.NotFoundError{Msg: "project not found"}
	}
	return *f.project, nil
}

func (f *fakeSlaReportRepo) Clocks(_ context.Context, _ string, from, to time.Time) ([]repository.SlaReportClockRow, error) {
	f.clocksRuns++
	f.gotFrom, f.gotTo = from, to
	return f.rows, f.clocksErr
}

func newSlaReportTestService(repo repository.SlaReportRepository) *slaReportService {
	return &slaReportService{
		repo: repo,
		now:  func() time.Time { return time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC) },
	}
}

func hours(h float64) *float64 { return ptr(h * 3600) }

// clockRow builds one case-with-a-clock row. The target is in hours.
func clockRow(id, number, issue, severity string, target, stage string, breached bool, elapsedHours, targetHours float64) repository.SlaReportClockRow {
	return repository.SlaReportClockRow{
		CaseID: id, CaseNumber: number, IssueType: issue, Severity: severity, State: "WORK_IN_PROGRESS",
		Opened:      time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC),
		Target:      ptr(target),
		Stage:       ptr(stage),
		HasBreached: ptr(breached),
		// elapsed and target are in seconds on the wire
		ElapsedSeconds: hours(elapsedHours),
		TargetSeconds:  hours(targetHours),
	}
}

func TestFormatReportDuration(t *testing.T) {
	cases := []struct {
		seconds float64
		want    string
	}{
		{0, "0 Minutes"},
		{-5, "0 Minutes"},
		{60, "1 Minute"},
		{1800, "30 Minutes"},
		{3600, "1 Hour"},
		{7200, "2 Hours"},
		{86400, "1 Day"},
		{172800, "2 Days"},
		{3600 + 1800, "1 Hour 30 Minutes"},
		{86400 + 4*3600 + 30*60, "1 Day 4 Hours 30 Minutes"},
		{59.6, "1 Minute"},
		// the report shows minutes at its finest, so under a minute reads as 0
		{45, "0 Minutes"},
		{90, "1 Minute"},
	}
	for _, tc := range cases {
		if got := formatReportDuration(tc.seconds); got != tc.want {
			t.Errorf("formatReportDuration(%v) = %q, want %q", tc.seconds, got, tc.want)
		}
	}
}

func TestPercentileCont(t *testing.T) {
	if got := percentileCont(nil, 0.95); got != 0 {
		t.Errorf("empty = %v", got)
	}
	if got := percentileCont([]float64{7}, 0.95); got != 7 {
		t.Errorf("single = %v", got)
	}
	// rank = 0.95 * 3 = 2.85: between 30 and 40 -> 30 + 0.85*10
	if got := percentileCont([]float64{10, 20, 30, 40}, 0.95); math.Abs(got-38.5) > 1e-9 {
		t.Errorf("interpolated = %v, want 38.5", got)
	}
	if got := percentileCont([]float64{10, 20, 30, 40, 50}, 0.5); got != 30 {
		t.Errorf("median = %v, want 30", got)
	}
}

func TestClassifySlaClock(t *testing.T) {
	cases := []struct {
		stage        string
		breached     bool
		wantStatus   string
		wantFinished bool
	}{
		{"ACHIEVED", false, slaStatusMet, true},
		{"COMPLETED", false, slaStatusMet, true},
		{"COMPLETED", true, slaStatusMissed, true},
		{"BREACHED", true, slaStatusMissed, false},
		{"IN_PROGRESS", false, slaStatusRunning, false},
		{"IN_PROGRESS", true, slaStatusMissed, false},
		{"PAUSED", false, slaStatusPaused, false},
		{"PAUSED", true, slaStatusMissed, false},
		{"CANCELLED", false, slaStatusCancelled, false},
		{"something_new", false, "", false},
	}
	for _, tc := range cases {
		status, finished := classifySlaClock(tc.stage, tc.breached)
		if status != tc.wantStatus || finished != tc.wantFinished {
			t.Errorf("classifySlaClock(%q, %v) = %q, %v; want %q, %v", tc.stage, tc.breached, status, finished, tc.wantStatus, tc.wantFinished)
		}
	}
}

func TestBuildSlaReport_SummaryAndCases(t *testing.T) {
	rows := []repository.SlaReportClockRow{
		// Incident S1: two finished response clocks (1h and 3h, target 4h), one running.
		clockRow("c1", "CS0001", "ERROR", "S1", "RESPONSE", "ACHIEVED", false, 1, 4),
		clockRow("c2", "CS0002", "TOTAL_OUTAGE", "S1", "RESPONSE", "COMPLETED", true, 3, 4),
		clockRow("c3", "CS0003", "ERROR", "S1", "RESPONSE", "IN_PROGRESS", false, 2, 4),
		// A breached clock that is still running does not count toward the percentile.
		clockRow("c4", "CS0004", "ERROR", "S1", "RESPONSE", "BREACHED", true, 900, 4),
		// A query with a resolution clock and no response clock.
		clockRow("c5", "CS0005", "QUESTION", "S3", "RESOLUTION", "ACHIEVED", false, 24, 72),
		// A case with no clock at all.
		{CaseID: "c6", CaseNumber: "CS0006", IssueType: "ERROR", Severity: "S2", State: "OPEN",
			Opened: time.Date(2026, 10, 6, 8, 0, 0, 0, time.UTC)},
	}

	summary, cases := buildSlaReport(rows)

	if len(cases) != 6 {
		t.Fatalf("cases = %d, want 6", len(cases))
	}
	if cases[5].Response.Elapsed != "—" || cases[5].Response.Status != "" {
		t.Errorf("a case without a clock shows %+v", cases[5].Response)
	}
	if cases[0].CaseType != "Incident" || cases[0].Severity != "Critical (P1)" || cases[0].State != "Work In Progress" {
		t.Errorf("case 0 labels = %+v", cases[0])
	}
	if cases[0].Response.Status != slaStatusMet || cases[0].Response.Elapsed != "1 Hour" {
		t.Errorf("case 0 response = %+v", cases[0].Response)
	}
	if cases[3].Response.Status != slaStatusMissed {
		t.Errorf("a breached running clock is Missed, got %+v", cases[3].Response)
	}
	if cases[4].CaseType != "Query" {
		t.Errorf("QUESTION is a Query, got %q", cases[4].CaseType)
	}

	// Groups with at least one case only: Incident S1, Incident S2, Query S3.
	if len(summary) != 3 {
		t.Fatalf("summary rows = %d (%+v)", len(summary), summary)
	}
	if summary[0].Severity != "S1" || summary[0].CaseType != "incident" || summary[0].Cases != 4 {
		t.Errorf("row 0 = %+v", summary[0])
	}
	if summary[1].Severity != "S2" || summary[1].Cases != 1 {
		t.Errorf("row 1 = %+v", summary[1])
	}
	if summary[2].CaseType != "query" || summary[2].Severity != "S3" {
		t.Errorf("row 2 = %+v", summary[2])
	}

	resp := summary[0].Response
	// Finished clocks: 1h and 3h; p95 = 1h + 0.95*(3h-1h) = 2.9h = 2h 54m.
	if resp.Elapsed != "2 Hours 54 Minutes" {
		t.Errorf("p95 response elapsed = %q", resp.Elapsed)
	}
	if resp.Target != "4 Hours" {
		t.Errorf("response target = %q", resp.Target)
	}
	if resp.Met != 1 || resp.Missed != 2 || resp.Running != 1 {
		t.Errorf("response counts = met %d missed %d running %d", resp.Met, resp.Missed, resp.Running)
	}
	if got := summary[0].Workaround; got.Elapsed != "—" || got.Target != "Not applicable" || got.Met+got.Missed+got.Running != 0 {
		t.Errorf("workaround with no clock = %+v", got)
	}
	if got := summary[2].Resolution; got.Elapsed != "1 Day" || got.Target != "3 Days" || got.Met != 1 {
		t.Errorf("query resolution = %+v", got)
	}
}

func TestBuildSlaReport_OrderAndUnknownSeverity(t *testing.T) {
	mk := func(id, issue, severity string) repository.SlaReportClockRow {
		return repository.SlaReportClockRow{CaseID: id, CaseNumber: id, IssueType: issue, Severity: severity, Opened: time.Now()}
	}
	summary, _ := buildSlaReport([]repository.SlaReportClockRow{
		mk("a", "QUESTION", "S0"), mk("b", "ERROR", "S3"), mk("c", "ERROR", "S0"), mk("d", "ERROR", "SX"), mk("e", "ERROR", ""),
	})
	var got []string
	for _, row := range summary {
		got = append(got, row.CaseType+":"+row.Severity)
	}
	if summary[2].Label != "Incident - No severity" {
		t.Errorf("a case without a severity is labelled %q", summary[2].Label)
	}
	want := []string{"incident:S0", "incident:S3", "incident:", "incident:SX", "query:S0"}
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestBuildSlaReport_TargetIsTheMostCommonPolicyDuration(t *testing.T) {
	rows := []repository.SlaReportClockRow{
		clockRow("a", "CS1", "ERROR", "S2", "RESOLUTION", "ACHIEVED", false, 10, 72),
		clockRow("b", "CS2", "ERROR", "S2", "RESOLUTION", "ACHIEVED", false, 11, 72),
		clockRow("c", "CS3", "ERROR", "S2", "RESOLUTION", "ACHIEVED", false, 12, 48),
	}
	summary, _ := buildSlaReport(rows)
	if got := summary[0].Resolution.Target; got != "3 Days" {
		t.Errorf("target = %q, want 3 Days", got)
	}
}

func TestGetProjectSlaReport_Validation(t *testing.T) {
	svc := newSlaReportTestService(&fakeSlaReportRepo{project: &domain.SlaReportProject{ID: slaReportTestProjectID, Key: "ACME"}})
	ctx := context.Background()
	cases := []struct {
		name, id, from, to string
	}{
		{"bad project id", "not-a-uuid", "2026-01-01", "2026-01-31"},
		{"missing from", slaReportTestProjectID, "", "2026-01-31"},
		{"missing to", slaReportTestProjectID, "2026-01-01", ""},
		{"not a date", slaReportTestProjectID, "yesterday-ish", "2026-01-31"},
		{"from after to", slaReportTestProjectID, "2026-02-01", "2026-01-31"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GetProjectSlaReport(ctx, tc.id, tc.from, tc.to)
			var validation *apierror.ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("err = %v, want a ValidationError", err)
			}
		})
	}
}

func TestGetProjectSlaReport_UnknownProjectIsNotFound(t *testing.T) {
	svc := newSlaReportTestService(&fakeSlaReportRepo{})
	_, err := svc.GetProjectSlaReport(context.Background(), slaReportTestProjectID, "2026-01-01", "2026-01-31")
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("err = %v, want a NotFoundError", err)
	}
}

func TestGetProjectSlaReport_WindowAndEcho(t *testing.T) {
	repo := &fakeSlaReportRepo{
		project: &domain.SlaReportProject{ID: slaReportTestProjectID, Key: "ACME", Name: ptr("Acme")},
		rows:    []repository.SlaReportClockRow{clockRow("c1", "CS1", "ERROR", "S1", "RESPONSE", "ACHIEVED", false, 1, 4)},
	}
	svc := newSlaReportTestService(repo)
	got, err := svc.GetProjectSlaReport(context.Background(), slaReportTestProjectID, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("GetProjectSlaReport: %v", err)
	}
	// A date-only `to` covers the whole day.
	if !repo.gotFrom.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("from = %v", repo.gotFrom)
	}
	if want := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond); !repo.gotTo.Equal(want) {
		t.Errorf("to = %v, want %v", repo.gotTo, want)
	}
	if got.From != "2026-01-01" || got.To != "2026-01-31" {
		t.Errorf("echoed window = %s to %s", got.From, got.To)
	}
	if got.Project.Key != "ACME" || got.Project.Name == nil || *got.Project.Name != "Acme" {
		t.Errorf("project = %+v", got.Project)
	}
	if len(got.Cases) != 1 || len(got.Summary) != 1 {
		t.Errorf("cases %d summary %d", len(got.Cases), len(got.Summary))
	}
}

func TestGetProjectSlaReport_NoCasesGivesEmptyLists(t *testing.T) {
	repo := &fakeSlaReportRepo{project: &domain.SlaReportProject{ID: slaReportTestProjectID, Key: "ACME"}}
	got, err := newSlaReportTestService(repo).GetProjectSlaReport(context.Background(), slaReportTestProjectID, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("GetProjectSlaReport: %v", err)
	}
	if got.Summary == nil || got.Cases == nil || len(got.Summary) != 0 || len(got.Cases) != 0 {
		t.Errorf("empty report must serialise as [], got summary=%v cases=%v", got.Summary, got.Cases)
	}
}

func TestSlaReportUnsupportedService(t *testing.T) {
	_, err := NewSlaReportUnsupportedService().GetProjectSlaReport(context.Background(), slaReportTestProjectID, "2026-01-01", "2026-01-31")
	var validation *apierror.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want a ValidationError", err)
	}
}
