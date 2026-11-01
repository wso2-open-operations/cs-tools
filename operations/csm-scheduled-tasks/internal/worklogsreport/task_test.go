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

package worklogsreport_test

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogsreport"
)

// The range these reports cover is derived in Asia/Colombo, not UTC, and
// the daily cron fires at an hour where the two zones disagree about the
// date. That disagreement has cost this codebase real bugs before, so it
// is pinned here rather than left to reasoning.
func TestReportRangesAreDerivedInColombo(t *testing.T) {
	loc, err := time.LoadLocation(worklogsreport.ReportTimezone)
	if err != nil {
		t.Fatalf("load %s: %v", worklogsreport.ReportTimezone, err)
	}

	t.Run("weekly covers the Monday-to-Sunday just gone", func(t *testing.T) {
		// 16:00 UTC on Monday 2026-10-12 -- when ServiceNow's
		// WeeklyReportAutomation V2 fires. The delivered mail for that
		// run is subject-lined [2026-10-05 - 2026-10-11].
		run := time.Date(2026, 10, 12, 16, 0, 0, 0, time.UTC)
		from, to := worklogsreport.WeeklyRange(run, loc)
		if got := from.Format("2006-01-02"); got != "2026-10-05" {
			t.Errorf("from = %s, the delivered mail says 2026-10-05", got)
		}
		if got := to.Format("2006-01-02"); got != "2026-10-11" {
			t.Errorf("to = %s, the delivered mail says 2026-10-11", got)
		}
		if d := to.Sub(from).Hours() / 24; d != 6 {
			t.Errorf("span = %v days between endpoints, want 6", d)
		}
	})

	t.Run("daily reports yesterday in Colombo, not in UTC", func(t *testing.T) {
		// 03:55 UTC on 2026-10-19 is 09:25 on the 19th in Colombo, which
		// is when the delivered daily mail arrives. Its subject is
		// [2026-10-18] -- yesterday in Colombo. Deriving it in UTC from
		// this instant gives the same answer, so the case that actually
		// separates the two zones is below.
		run := time.Date(2026, 10, 19, 3, 55, 0, 0, time.UTC)
		if got := worklogsreport.DailyDate(run, loc).Format("2006-01-02"); got != "2026-10-18" {
			t.Errorf("daily date = %s, the delivered mail says 2026-10-18", got)
		}
	})

	// *** A RETRY MUST REPORT THE PERIOD IT OWED, NOT THE ONE IT RAN IN. ***
	//
	// The engine retries a failed period on every tick until the next
	// firing, so the instant a run finally succeeds is not the instant
	// it was scheduled for. Both ranges are therefore derived from the
	// SCHEDULED FIRING, which is what callers pass in.
	//
	// Taking the wall clock instead was a real defect, and the shape of
	// it is worth keeping written down: a daily run fires 03:55 UTC on
	// the 19th owing the 18th; it fails; a retry succeeds at 19:00 UTC,
	// by which point it is already the 20th in Colombo, so a wall-clock
	// derivation reports the 19th. The 18th would never be sent and the
	// 19th would go out twice.
	t.Run("a retry reports the firing's period, not the retry's", func(t *testing.T) {
		firing := time.Date(2026, 10, 19, 3, 55, 0, 0, time.UTC)
		retry := time.Date(2026, 10, 19, 19, 0, 0, 0, time.UTC)

		if got := worklogsreport.DailyDate(firing, loc).Format("2006-01-02"); got != "2026-10-18" {
			t.Fatalf("firing reports %s, want 2026-10-18", got)
		}
		// The retry instant alone gives a different, wrong answer --
		// which is precisely why the task anchors on the firing.
		if got := worklogsreport.DailyDate(retry, loc).Format("2006-01-02"); got == "2026-10-18" {
			t.Skip("the two instants no longer differ; the retry hazard this guards is gone")
		} else if got != "2026-10-19" {
			t.Fatalf("retry instant reports %s, want 2026-10-19", got)
		}

		// Weekly, same hazard: a Monday firing owes Mon..Sun, and a
		// retry late that evening must not slide the window forward.
		wf := time.Date(2026, 10, 12, 16, 0, 0, 0, time.UTC)
		from, to := worklogsreport.WeeklyRange(wf, loc)
		if from.Format("2006-01-02") != "2026-10-05" || to.Format("2006-01-02") != "2026-10-11" {
			t.Errorf("weekly firing = %s..%s, want 2026-10-05..2026-10-11",
				from.Format("2006-01-02"), to.Format("2006-01-02"))
		}
		rf, rt := worklogsreport.WeeklyRange(time.Date(2026, 10, 12, 19, 0, 0, 0, time.UTC), loc)
		if rf.Format("2006-01-02") == from.Format("2006-01-02") {
			t.Skip("the two instants no longer differ; the retry hazard this guards is gone")
		}
		t.Logf("a wall-clock retry would have reported %s..%s instead",
			rf.Format("2006-01-02"), rt.Format("2006-01-02"))
	})

	t.Run("the returned dates carry no time-of-day", func(t *testing.T) {
		// work_date is a bare DATE upstream; a range carrying a clock
		// time would make the boundary depend on when the cron ran.
		from, to := worklogsreport.WeeklyRange(time.Now(), loc)
		for _, d := range []time.Time{from, to, worklogsreport.DailyDate(time.Now(), loc)} {
			if d.Hour() != 0 || d.Minute() != 0 || d.Second() != 0 {
				t.Errorf("%v carries a time of day", d)
			}
		}
	})
}

// fakeReports records the range it was asked for.
type fakeReports struct {
	cadence  worklogreport.Cadence
	from, to time.Time
}

func (f *fakeReports) Report(_ context.Context, c worklogreport.Cadence, from, to time.Time) (worklogreport.Report, error) {
	f.cadence, f.from, f.to = c, from, to
	return worklogreport.Report{Cadence: c, From: from.Format("2006-01-02"), To: to.Format("2006-01-02")}, nil
}

// fakeEmail records who it was asked to mail.
type fakeEmail struct {
	sent   int
	to, cc []string
}

func (f *fakeEmail) SendEmail(_ context.Context, to, cc []string, _, _ string) error {
	f.sent++
	f.to, f.cc = to, cc
	return nil
}

// The handler must ask for the period its ANCHOR names, whatever the
// wall clock says. Retries run at an arbitrary later instant, and the
// period they owe does not move with them.
func TestSendUsesTheAnchorNotTheClock(t *testing.T) {
	// The firing this run belongs to: 03:55 UTC on the 19th, owing the
	// 18th. The process happens to be executing much later.
	firing := time.Date(2026, 10, 19, 3, 55, 0, 0, time.UTC)

	reports := &fakeReports{}
	email := &fakeEmail{}
	handler := worklogsreport.SendDaily(reports, email,
		func() time.Time { return firing },
		[]string{"leads-list@example.com"}, []string{"all-list@example.com"},
		worklogreport.SourceConfigured, "", true)

	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got := reports.from.Format("2006-01-02"); got != "2026-10-18" {
		t.Errorf("asked for %s, want 2026-10-18 -- the anchor's period", got)
	}
	if !reports.from.Equal(reports.to) {
		t.Errorf("daily range spans %s..%s, want a single day",
			reports.from.Format("2006-01-02"), reports.to.Format("2006-01-02"))
	}
	if email.sent != 1 {
		t.Fatalf("sent %d mails, want 1", email.sent)
	}
	// Configured lists win over whatever the report derived.
	if len(email.to) != 1 || email.to[0] != "leads-list@example.com" {
		t.Errorf("to = %v, want the configured list", email.to)
	}
	if len(email.cc) != 1 || email.cc[0] != "all-list@example.com" {
		t.Errorf("cc = %v, want the configured list", email.cc)
	}
}

// The weekly handler anchors the same way, and a Monday firing owes the
// Monday-to-Sunday before it.
func TestSendWeeklyUsesTheAnchor(t *testing.T) {
	firing := time.Date(2026, 10, 12, 16, 0, 0, 0, time.UTC)
	reports := &fakeReports{}
	handler := worklogsreport.SendWeekly(reports, &fakeEmail{},
		func() time.Time { return firing },
		[]string{"leads-list@example.com"}, nil, worklogreport.SourceConfigured, "", true)

	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if reports.from.Format("2006-01-02") != "2026-10-05" || reports.to.Format("2006-01-02") != "2026-10-11" {
		t.Errorf("asked for %s..%s, want 2026-10-05..2026-10-11",
			reports.from.Format("2006-01-02"), reports.to.Format("2006-01-02"))
	}
}

// An empty To list sends nothing and still succeeds, and must not even
// fetch: a report nobody receives is work with no consumer.
func TestSendWithNoRecipientsFetchesNothing(t *testing.T) {
	reports := &fakeReports{}
	email := &fakeEmail{}
	handler := worklogsreport.SendDaily(reports, email,
		func() time.Time { return time.Now() }, nil, nil, worklogreport.SourceConfigured, "", true)

	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if email.sent != 0 || !reports.from.IsZero() {
		t.Errorf("fetched or sent with no recipients: sent=%d from=%v", email.sent, reports.from)
	}
}

// A configured list that is blank is not an audience. {"to": [""]} has
// length one, so a raw length check let it through -- and with the
// derived fallback that once existed, the report went to the
// registry-derived recipients instead: exactly what configuring the
// lists exists to prevent.
func TestSendTreatsABlankRecipientListAsEmpty(t *testing.T) {
	for _, blank := range [][]string{{""}, {"   "}, {"", " "}} {
		reports := &fakeReports{}
		email := &fakeEmail{}
		handler := worklogsreport.SendDaily(reports, email,
			func() time.Time { return time.Now() }, blank, nil, worklogreport.SourceConfigured, "", true)

		if err := handler(context.Background()); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if email.sent != 0 {
			t.Errorf("to=%q sent %d mails, want 0", blank, email.sent)
		}
		if !reports.from.IsZero() {
			t.Errorf("to=%q fetched a report nobody would receive", blank)
		}
	}
}

// fakeDerived returns a report carrying a registry-derived audience.
type fakeDerived struct{ to, cc []string }

func (f *fakeDerived) Report(_ context.Context, c worklogreport.Cadence, from, to time.Time) (worklogreport.Report, error) {
	return worklogreport.Report{
		Cadence:    c,
		Recipients: worklogreport.Recipients{To: f.to, Cc: f.cc},
	}, nil
}

// Switching the source to "derived" hands the audience to the team
// registry. It is a config change, not a code change -- but it changes
// who receives the report, so it must take effect completely rather
// than merging with the configured lists.
func TestSendDerivedSourceUsesTheRegistryAudience(t *testing.T) {
	reports := &fakeDerived{
		to: []string{"registry-lead@example.com"},
		cc: []string{"registry-list@example.com"},
	}
	email := &fakeEmail{}
	handler := worklogsreport.SendDaily(reports, email,
		func() time.Time { return time.Now() },
		[]string{"configured@example.com"}, []string{"configured-cc@example.com"},
		worklogreport.SourceDerived, "", true)

	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	// To is still the registry's alone. A configured to must never leak
	// into it: widening who the report is ADDRESSED to stays a
	// deliberate change of source.
	if len(email.to) != 1 || email.to[0] != "registry-lead@example.com" {
		t.Errorf("to = %v, want the registry's audience alone", email.to)
	}
	// Cc is the documented exception: the configured list is appended to
	// the derived one, so a standing list can be copied without taking
	// ownership of the whole audience. See Resolve's own comment for why
	// this cannot be expressed in the team registry instead.
	if len(email.cc) != 2 ||
		email.cc[0] != "registry-list@example.com" ||
		email.cc[1] != "configured-cc@example.com" {
		t.Errorf("cc = %v, want the registry's list then the configured one", email.cc)
	}
}

// A derived audience that comes back empty must fail loudly. SendEmail
// treats no recipients as success, so the alternative is a report that
// silently never arrives -- the worst failure this feature has.
func TestSendDerivedSourceFailsOnAnEmptyAudience(t *testing.T) {
	email := &fakeEmail{}
	handler := worklogsreport.SendDaily(&fakeDerived{}, email,
		func() time.Time { return time.Now() },
		[]string{"configured@example.com"}, nil,
		worklogreport.SourceDerived, "", true)

	err := handler(context.Background())
	if err == nil {
		t.Fatal("want an error when the derived audience is empty")
	}
	if email.sent != 0 {
		t.Errorf("sent %d mails with no audience", email.sent)
	}
}

func TestParseSource(t *testing.T) {
	for raw, want := range map[string]worklogreport.Source{
		"":           worklogreport.SourceConfigured,
		"configured": worklogreport.SourceConfigured,
		"derived":    worklogreport.SourceDerived,
		"DERIVED":    worklogreport.SourceDerived,
		"  derived ": worklogreport.SourceDerived,
	} {
		got, ok := worklogreport.ParseSource(raw)
		if !ok || got != want {
			t.Errorf("ParseSource(%q) = %v, %v; want %v, true", raw, got, ok, want)
		}
	}
	// A typo must not quietly mail a different set of people.
	if got, ok := worklogreport.ParseSource("drived"); ok || got != worklogreport.SourceConfigured {
		t.Errorf("ParseSource(typo) = %v, %v; want configured, false", got, ok)
	}
}
