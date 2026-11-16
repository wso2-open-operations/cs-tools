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

package slabreachscan

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/slabreachreport"
)

type fakeFetcher struct {
	report                    slabreachreport.Report
	err                       error
	calls                     int
	gotDefs, gotStage         []string
	gotOnboarding, gotExclude []string
	gotWeek                   time.Time
}

func (f *fakeFetcher) Report(_ context.Context, definitions, stages, onboardingStatuses, excludeOnboardingStatuses []string, week time.Time) (slabreachreport.Report, error) {
	f.calls++
	f.gotDefs, f.gotStage = definitions, stages
	f.gotOnboarding, f.gotExclude = onboardingStatuses, excludeOnboardingStatuses
	f.gotWeek = week
	return f.report, f.err
}

type fakeSender struct {
	calls   int
	to, cc  []string
	subject string
	body    string
}

func (f *fakeSender) SendEmail(_ context.Context, to, cc []string, subject, htmlBody string) error {
	f.calls++
	f.to, f.cc, f.subject, f.body = to, cc, subject, htmlBody
	return nil
}

func defs() []string   { return DefaultDefinitions() }
func cre() Variant     { return CREVariant() }
func someTo() []string { return []string{"recipient@example.invalid"} }

// TestCREVariantFilters pins the CRE report's own filter set: the source
// selects stages Paused and Completed and EXCLUDES onboarding statuses
// In-Progress and Not-Started. It must not carry an "is one of" status
// filter -- that belongs to the other report.
func TestCREVariantFilters(t *testing.T) {
	v := cre()
	if len(v.Stages) != 2 || v.Stages[0] != "PAUSED" || v.Stages[1] != "COMPLETED" {
		t.Fatalf("stages = %#v", v.Stages)
	}
	if len(v.ExcludeOnboardingStatuses) != 2 ||
		v.ExcludeOnboardingStatuses[0] != "IN_PROGRESS" || v.ExcludeOnboardingStatuses[1] != "NOT_STARTED" {
		t.Fatalf("excluded statuses = %#v", v.ExcludeOnboardingStatuses)
	}
	if len(v.OnboardingStatuses) != 0 {
		t.Fatalf("the CRE report must not restrict to a status set, got %#v", v.OnboardingStatuses)
	}
	if got := TaskName(v); got != "sla_breach_weekly_report" {
		t.Fatalf("task name = %q", got)
	}
	if v.Subject != "Weekly SLA Breach Report" {
		t.Fatalf("subject = %q", v.Subject)
	}
}

// TestSendReportSkipsWithoutRecipients: with no "to" the task must not
// even reach entity-service.
func TestSendReportSkipsWithoutRecipients(t *testing.T) {
	fetch, send := &fakeFetcher{}, &fakeSender{}
	if err := SendReport(fetch, send, cre(), defs(), nil, nil, "", true, time.Time{})(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if fetch.calls != 0 || send.calls != 0 {
		t.Fatalf("fetch=%d send=%d, want 0 and 0", fetch.calls, send.calls)
	}
}

// TestSendReportSkipsWhenEmailsDisabled covers ALERTS_ENABLED=false.
func TestSendReportSkipsWhenEmailsDisabled(t *testing.T) {
	fetch, send := &fakeFetcher{}, &fakeSender{}
	if err := SendReport(fetch, send, cre(), defs(), someTo(), nil, "", false, time.Time{})(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if fetch.calls != 0 || send.calls != 0 {
		t.Fatalf("fetch=%d send=%d, want 0 and 0", fetch.calls, send.calls)
	}
}

// TestSendReportSendsAnEmptyWeek is the deliberate difference from the
// regression reports: a clean week is the result this report exists to
// show, so it is still sent.
func TestSendReportSendsAnEmptyWeek(t *testing.T) {
	fetch := &fakeFetcher{report: slabreachreport.Report{
		PeriodStart: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
	}}
	send := &fakeSender{}
	if err := SendReport(fetch, send, cre(), defs(), someTo(), nil, "", true, time.Time{})(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if send.calls != 1 {
		t.Fatalf("send calls = %d, want 1 even with no breaches", send.calls)
	}
}

// TestSendReportSubjectAndAudience pins the subject word for word and
// checks the configured audience is what reaches the sender.
func TestSendReportSubjectAndAudience(t *testing.T) {
	fetch := &fakeFetcher{report: slabreachreport.Report{
		Rows: []slabreachreport.Row{{Team: "Atlas", CaseNumber: "CS01", Definition: "P1 - Response (Open Source)"}},
	}}
	send := &fakeSender{}
	if err := SendReport(fetch, send, cre(), defs(), []string{"to@example.invalid"}, []string{"cc@example.invalid"}, "", true, time.Time{})(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if send.subject != "Weekly SLA Breach Report" {
		t.Fatalf("subject = %q", send.subject)
	}
	if len(send.to) != 1 || send.to[0] != "to@example.invalid" || len(send.cc) != 1 || send.cc[0] != "cc@example.invalid" {
		t.Fatalf("to=%v cc=%v", send.to, send.cc)
	}
	if !strings.Contains(send.body, "CS01") {
		t.Fatal("body is missing the case number")
	}
}

// TestSendReportPassesEveryFilterThrough: the ten definitions and the
// variant's own three filters must reach entity-service unchanged.
func TestSendReportPassesEveryFilterThrough(t *testing.T) {
	fetch, send := &fakeFetcher{}, &fakeSender{}
	if err := SendReport(fetch, send, cre(), defs(), someTo(), nil, "", true, time.Time{})(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if len(fetch.gotDefs) != 10 {
		t.Fatalf("definitions = %d, want the 10 the source reports name", len(fetch.gotDefs))
	}
	if len(fetch.gotStage) != 2 || fetch.gotStage[0] != "PAUSED" {
		t.Fatalf("stages = %v", fetch.gotStage)
	}
	if len(fetch.gotExclude) != 2 {
		t.Fatalf("excluded statuses = %v", fetch.gotExclude)
	}
	if len(fetch.gotOnboarding) != 0 {
		t.Fatalf("the CRE report must send no is-one-of status filter, got %v", fetch.gotOnboarding)
	}
}

// TestSendReportRefusesWithoutDefinitions: entity-service rejects an
// empty list, so fail here rather than 400 on every tick.
func TestSendReportRefusesWithoutDefinitions(t *testing.T) {
	fetch, send := &fakeFetcher{}, &fakeSender{}
	if err := SendReport(fetch, send, cre(), nil, someTo(), nil, "", true, time.Time{})(context.Background()); err == nil {
		t.Fatal("want an error with no definitions configured")
	}
	if fetch.calls != 0 || send.calls != 0 {
		t.Fatalf("fetch=%d send=%d, want 0 and 0", fetch.calls, send.calls)
	}
}

// TestSendReportPropagatesFetchFailure keeps a failed period retryable
// rather than silently sending nothing.
func TestSendReportPropagatesFetchFailure(t *testing.T) {
	fetch := &fakeFetcher{err: errors.New("boom")}
	send := &fakeSender{}
	if err := SendReport(fetch, send, cre(), defs(), someTo(), nil, "", true, time.Time{})(context.Background()); err == nil {
		t.Fatal("want the fetch error propagated")
	}
	if send.calls != 0 {
		t.Fatalf("send calls = %d, want 0", send.calls)
	}
}

// TestApplyStageOverride: set-but-empty means every stage, which is a
// real choice and must not fall back to the default.
func TestApplyStageOverride(t *testing.T) {
	t.Setenv("SLA_BREACH_REPORT_CRE_STAGES", "BREACHED")
	if got := ApplyStageOverride(cre()).Stages; len(got) != 1 || got[0] != "BREACHED" {
		t.Fatalf("stages = %#v", got)
	}
	t.Setenv("SLA_BREACH_REPORT_CRE_STAGES", "")
	if got := ApplyStageOverride(cre()).Stages; len(got) != 0 {
		t.Fatalf("set-but-empty = %#v, want every stage", got)
	}
}

// TestParseList covers the override parsing both overrides rely on.
func TestParseList(t *testing.T) {
	if got := ParseList("  "); got != nil {
		t.Fatalf("blank = %#v, want nil so the default is kept", got)
	}
	got := ParseList(" PAUSED , ,COMPLETED ")
	if len(got) != 2 || got[0] != "PAUSED" || got[1] != "COMPLETED" {
		t.Fatalf("got %#v", got)
	}
}

// TestRunNowWeekRequiresOneWholeWeek: the report always covers a single
// Monday-to-Monday week, so a range spanning a boundary has no answer it
// could give. Reporting one of the two weeks silently would be worse
// than refusing.
func TestRunNowWeekRequiresOneWholeWeek(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }

	// Not given at all: the scheduled path, no error, zero week.
	if w, err := RunNowWeek(time.Time{}, time.Time{}, time.UTC); err != nil || !w.IsZero() {
		t.Fatalf("no dates: got (%v, %v), want (zero, nil)", w, err)
	}
	// Mon 12 Oct to Sun 18 Oct is one week.
	if w, err := RunNowWeek(day(12), day(18), time.UTC); err != nil || !w.Equal(day(12)) {
		t.Fatalf("same week: got (%v, %v)", w, err)
	}
	// Mid-week to mid-week, still one week.
	if _, err := RunNowWeek(day(14), day(16), time.UTC); err != nil {
		t.Fatalf("same week mid-range: %v", err)
	}
	// Sun 18 to Mon 19 crosses the boundary.
	if _, err := RunNowWeek(day(18), day(19), time.UTC); err == nil {
		t.Fatal("a range crossing a Monday must be refused")
	}
	// Reversed.
	if _, err := RunNowWeek(day(16), day(14), time.UTC); err == nil {
		t.Fatal("--to-date before --from-date must be refused")
	}
	// Only one given.
	if _, err := RunNowWeek(day(14), time.Time{}, time.UTC); err == nil {
		t.Fatal("--from-date without --to-date must be refused")
	}
}

// TestSendReportPassesTheWeekThrough: a manual run's week has to reach
// entity-service, or the run silently reports last week instead.
func TestSendReportPassesTheWeekThrough(t *testing.T) {
	week := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	fetch, send := &fakeFetcher{}, &fakeSender{}
	if err := SendReport(fetch, send, cre(), defs(), someTo(), nil, "", true, week)(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if !fetch.gotWeek.Equal(week) {
		t.Fatalf("week = %v, want %v", fetch.gotWeek, week)
	}
	// The scheduled path must still ask for last week.
	fetch2 := &fakeFetcher{}
	if err := SendReport(fetch2, &fakeSender{}, cre(), defs(), someTo(), nil, "", true, time.Time{})(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if !fetch2.gotWeek.IsZero() {
		t.Fatalf("scheduled run must send no week, got %v", fetch2.gotWeek)
	}
}

func onboarding() Variant { return OnboardingVariant() }

// TestOnboardingVariantFilters pins the onboarding report's own filter
// set. Both differ from the CRE report's, which is the whole reason the
// two are separate tasks rather than two sections of one.
func TestOnboardingVariantFilters(t *testing.T) {
	v := onboarding()
	if len(v.Stages) != 1 || v.Stages[0] != "IN_PROGRESS" {
		t.Fatalf("stages = %#v, want [IN_PROGRESS]", v.Stages)
	}
	if len(v.OnboardingStatuses) != 1 || v.OnboardingStatuses[0] != "IN_PROGRESS" {
		t.Fatalf("onboarding statuses = %#v, want [IN_PROGRESS]", v.OnboardingStatuses)
	}
	if len(v.ExcludeOnboardingStatuses) != 0 {
		t.Fatalf("the onboarding report uses is-one-of, not is-not-one-of, got %#v", v.ExcludeOnboardingStatuses)
	}
	if got := TaskName(v); got != "sla_breach_weekly_report_onboarding" {
		t.Fatalf("task name = %q", got)
	}
	if v.Subject != "Weekly SLA Breach Report (Onboarding)" {
		t.Fatalf("subject = %q", v.Subject)
	}
}

// TestTheTwoVariantsAreNotComplements documents a real gap in the
// source configuration: the CRE report excludes NOT_STARTED and the
// onboarding report includes only IN_PROGRESS, so a NOT_STARTED case is
// reported by neither. Confirmed against the source's own View Report,
// which shows the onboarding report returning 0 rows and still being
// kept scheduled. If this test ever fails because the sets were
// "tidied", that is a behaviour change away from ServiceNow and needs
// saying out loud.
func TestTheTwoVariantsAreNotComplements(t *testing.T) {
	for _, inc := range onboarding().OnboardingStatuses {
		if inc == "NOT_STARTED" {
			t.Fatal("NOT_STARTED is reported by the onboarding report; the source reports it in neither")
		}
	}
	found := false
	for _, exc := range cre().ExcludeOnboardingStatuses {
		if exc == "NOT_STARTED" {
			found = true
		}
	}
	if !found {
		t.Fatal("the CRE report no longer excludes NOT_STARTED")
	}
}

// TestOnboardingSendReportPassesItsOwnFilters: the variant's filters,
// not the CRE report's, must reach entity-service.
func TestOnboardingSendReportPassesItsOwnFilters(t *testing.T) {
	fetch, send := &fakeFetcher{}, &fakeSender{}
	if err := SendReport(fetch, send, onboarding(), defs(), someTo(), nil, "", true, time.Time{})(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if len(fetch.gotStage) != 1 || fetch.gotStage[0] != "IN_PROGRESS" {
		t.Fatalf("stages = %v", fetch.gotStage)
	}
	if len(fetch.gotOnboarding) != 1 || fetch.gotOnboarding[0] != "IN_PROGRESS" {
		t.Fatalf("onboarding statuses = %v", fetch.gotOnboarding)
	}
	if len(fetch.gotExclude) != 0 {
		t.Fatalf("must send no is-not-one-of filter, got %v", fetch.gotExclude)
	}
	if send.subject != "Weekly SLA Breach Report (Onboarding)" {
		t.Fatalf("subject = %q", send.subject)
	}
}

// TestOnboardingStageOverrideIsIndependent: the two tasks must take
// their stage overrides from different variables, or configuring one
// silently changes the other.
func TestOnboardingStageOverrideIsIndependent(t *testing.T) {
	t.Setenv("SLA_BREACH_REPORT_ONBOARDING_STAGES", "BREACHED")
	if got := ApplyStageOverride(onboarding()).Stages; len(got) != 1 || got[0] != "BREACHED" {
		t.Fatalf("onboarding stages = %#v", got)
	}
	if got := ApplyStageOverride(cre()).Stages; len(got) != 2 || got[0] != "PAUSED" {
		t.Fatalf("the CRE report must be unaffected, got %#v", got)
	}
}
