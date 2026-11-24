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

package eolusagescan_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/eolusagereport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/eolusagescan"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/schedule"
)

type fakeReports struct {
	report  eolusagereport.Report
	err     error
	calls   int
	gotAsOf string
}

func (f *fakeReports) Report(_ context.Context, asOf string) (eolusagereport.Report, error) {
	f.calls++
	f.gotAsOf = asOf
	return f.report, f.err
}

type fakeEmail struct {
	to, cc  []string
	subject string
	body    string
	calls   int
	err     error
}

func (f *fakeEmail) SendEmail(_ context.Context, to, cc []string, subject, htmlBody string) error {
	f.calls++
	f.to, f.cc, f.subject, f.body = to, cc, subject, htmlBody
	return f.err
}

// at is a clock fixed at 04:30 UTC (10:00 in Colombo) on the given date.
func at(y int, m time.Month, d int) func() time.Time {
	return func() time.Time { return time.Date(y, m, d, 4, 30, 0, 0, time.UTC) }
}

func status(s string) *string { return &s }

func sampleReport() eolusagereport.Report {
	return eolusagereport.Report{
		AsOf: "2026-11-02", TotalVersions: 3, TotalAccounts: 2,
		Regions: []eolusagereport.Region{
			{Region: "APAC", Accounts: []eolusagereport.Account{{
				AccountID: "a1", AccountName: "Acme", CsAbt: "Team Alpha", Region: "APAC",
				AccountManager: &eolusagereport.Person{Name: "Account Manager One", Email: "am.one@example.com"},
				TechnicalOwner: &eolusagereport.Person{Name: "Technical Owner One", Email: "TO.One@example.com"},
				Projects: []eolusagereport.Project{{ProjectID: "p1", ProjectName: "Acme Sub", ProjectKey: "ACMESUB",
					Versions: []eolusagereport.Version{
						{ProductName: "WSO2 API Manager", Version: "2.6.0", SupportStatus: status("EXTENDED"), EOLDate: "2022-10-22"},
						{ProductName: "WSO2 Identity Server", Version: "5.10.0", SupportStatus: status("EXTENDED"), EOLDate: "2026-03-25"},
					}}},
			}}},
			{Region: "UK", Accounts: []eolusagereport.Account{{
				AccountID: "a2", AccountName: "Beta", Region: "UK",
				// The same technical owner as Acme's, in another case: listed once.
				TechnicalOwner: &eolusagereport.Person{Email: "to.one@EXAMPLE.com"},
				Projects: []eolusagereport.Project{{ProjectID: "p2", ProjectName: "Beta Sub",
					Versions: []eolusagereport.Version{{ProductName: "WSO2 Enterprise Integrator", Version: "6.6.0", EOLDate: "2025-12-22"}}}},
			}}},
		},
	}
}

func TestSendReportSendsOnTheFirstMondayToOwnersAndLeadership(t *testing.T) {
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{}
	handler := eolusagescan.SendReport(reports, email, []string{"leadership@example.com"}, []string{"am.one@example.com", "cc@example.com"},
		eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 11, 2)})
	if err := handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	if reports.gotAsOf != "2026-11-02" {
		t.Errorf("asOf = %q, want the run date in Colombo", reports.gotAsOf)
	}
	if email.calls != 1 || email.subject != eolusagescan.Subject {
		t.Fatalf("calls=%d subject=%q", email.calls, email.subject)
	}
	wantTo := []string{"leadership@example.com", "am.one@example.com", "TO.One@example.com"}
	if !reflect.DeepEqual(email.to, wantTo) {
		t.Errorf("to = %v, want %v (leadership, then each owner once)", email.to, wantTo)
	}
	if !reflect.DeepEqual(email.cc, []string{"cc@example.com"}) {
		t.Errorf("cc = %v, want the configured cc minus anyone already in To", email.cc)
	}
}

func TestSendReportTakesTheDateInColombo(t *testing.T) {
	// 20:00 UTC on Sunday the 1st is 01:30 on Monday the 2nd in Colombo.
	reports := &fakeReports{report: sampleReport()}
	handler := eolusagescan.SendReport(reports, &fakeEmail{}, []string{"lead@example.com"}, nil,
		eolusagescan.Options{EmailsEnabled: true, Now: func() time.Time { return time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC) }})
	if err := handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	if reports.gotAsOf != "2026-11-02" {
		t.Errorf("asOf = %q, want 2026-11-02", reports.gotAsOf)
	}
}

func TestSendReportSkipsMondaysAfterTheFirstWeek(t *testing.T) {
	for _, day := range []int{8, 9, 16, 30} {
		reports := &fakeReports{report: sampleReport()}
		email := &fakeEmail{}
		handler := eolusagescan.SendReport(reports, email, []string{"lead@example.com"}, nil,
			eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 11, day)})
		if err := handler(context.Background()); err != nil {
			t.Fatalf("day %d: %v", day, err)
		}
		if reports.calls != 0 || email.calls != 0 {
			t.Errorf("day %d: fetched %d, sent %d; want nothing", day, reports.calls, email.calls)
		}
	}
	// 7 December 2026 is the month's first Monday: it sends.
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{}
	handler := eolusagescan.SendReport(reports, email, []string{"lead@example.com"}, nil,
		eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 12, 7)})
	if err := handler(context.Background()); err != nil || email.calls != 1 {
		t.Errorf("7 Dec: err=%v sent=%d, want one mail", err, email.calls)
	}
}

// A schedule overridden to fire on other days (SUB_CRON_SCHEDULES) still
// sends only on the first Monday: the 1st to the 6th of December 2026 are
// Tuesday to Sunday.
func TestSendReportSendsOnlyOnAMonday(t *testing.T) {
	for day := 1; day <= 6; day++ {
		reports := &fakeReports{report: sampleReport()}
		email := &fakeEmail{}
		handler := eolusagescan.SendReport(reports, email, []string{"lead@example.com"}, nil,
			eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 12, day), Anchor: at(2026, 12, day)})
		if err := handler(context.Background()); err != nil {
			t.Fatalf("day %d: %v", day, err)
		}
		if reports.calls != 0 || email.calls != 0 {
			t.Errorf("day %d: fetched %d, sent %d; want nothing", day, reports.calls, email.calls)
		}
	}
}

func TestSendReportRetriedLaterInTheWeekStillSendsAsOfTheFirstMonday(t *testing.T) {
	// The first Monday of December 2026 is the 7th. Its run failed and the
	// engine retries it on the 9th: the clock is in the second week, the
	// period is not.
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{}
	handler := eolusagescan.SendReport(reports, email, []string{"lead@example.com"}, nil,
		eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 12, 9), Anchor: at(2026, 12, 7)})
	if err := handler(context.Background()); err != nil || email.calls != 1 {
		t.Fatalf("err=%v sent=%d, want the retry to send", err, email.calls)
	}
	if reports.gotAsOf != "2026-12-07" {
		t.Errorf("asOf = %q, want the first Monday, 2026-12-07", reports.gotAsOf)
	}

	// A second-week Monday's own run still does not send.
	email = &fakeEmail{}
	handler = eolusagescan.SendReport(&fakeReports{report: sampleReport()}, email, []string{"lead@example.com"}, nil,
		eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 12, 14), Anchor: at(2026, 12, 14)})
	if err := handler(context.Background()); err != nil || email.calls != 0 {
		t.Fatalf("err=%v sent=%d, want no mail for the 14th", err, email.calls)
	}
}

func TestSendReportAnyDaySendsWhateverTheDate(t *testing.T) {
	email := &fakeEmail{}
	handler := eolusagescan.SendReport(&fakeReports{report: sampleReport()}, email, []string{"lead@example.com"}, nil,
		eolusagescan.Options{EmailsEnabled: true, AnyDay: true, Now: at(2026, 11, 19)})
	if err := handler(context.Background()); err != nil || email.calls != 1 {
		t.Fatalf("err=%v sent=%d, want a manual run to send on the 19th", err, email.calls)
	}
}

func TestSendReportWithoutRecipientsOrEmailsSkips(t *testing.T) {
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{}
	if err := eolusagescan.SendReport(reports, email, nil, []string{"cc@example.com"},
		eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 11, 2)})(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := eolusagescan.SendReport(reports, email, []string{"lead@example.com"}, nil,
		eolusagescan.Options{EmailsEnabled: false, Now: at(2026, 11, 2)})(context.Background()); err != nil {
		t.Fatal(err)
	}
	if reports.calls != 0 || email.calls != 0 {
		t.Errorf("fetched %d, sent %d; want nothing without a to, or with emails off", reports.calls, email.calls)
	}
}

func TestSendReportWithNoEOLUsageSendsANoteToLeadershipOnly(t *testing.T) {
	email := &fakeEmail{}
	reports := &fakeReports{report: eolusagereport.Report{AsOf: "2026-11-02", Regions: []eolusagereport.Region{}}}
	handler := eolusagescan.SendReport(reports, email, []string{"lead@example.com"}, []string{"cc@example.com", "LEAD@example.com"},
		eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 11, 2)})
	if err := handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(email.to, []string{"lead@example.com"}) || !reflect.DeepEqual(email.cc, []string{"cc@example.com"}) {
		t.Errorf("to=%v cc=%v, want the configured audience only", email.to, email.cc)
	}
	if !strings.Contains(email.body, "No customers are using EOL product versions") {
		t.Error("the note does not say there are no customers")
	}
}

func TestSendReportFailures(t *testing.T) {
	handler := eolusagescan.SendReport(&fakeReports{err: errors.New("upstream 503")}, &fakeEmail{}, []string{"lead@example.com"}, nil,
		eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 11, 2)})
	if err := handler(context.Background()); err == nil || !strings.Contains(err.Error(), "upstream 503") ||
		!strings.Contains(err.Error(), "2026-11-02") {
		t.Errorf("fetch failure = %v, want an error naming the cause and the date", err)
	}
	handler = eolusagescan.SendReport(&fakeReports{report: sampleReport()}, &fakeEmail{err: errors.New("smtp down")}, []string{"lead@example.com"}, nil,
		eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 11, 2)})
	if err := handler(context.Background()); err == nil || !strings.Contains(err.Error(), "smtp down") {
		t.Errorf("send failure = %v, want an error", err)
	}
}

// Account contacts must be one plain address and, when domains are set, in
// one of them; the configured to is taken as given.
func TestAudienceDropsMalformedAndOutsideDomainContacts(t *testing.T) {
	r := eolusagereport.Report{Regions: []eolusagereport.Region{{Accounts: []eolusagereport.Account{
		{AccountManager: &eolusagereport.Person{Email: "am@example.com"}, TechnicalOwner: &eolusagereport.Person{Email: "owner@other.org"}},
		{AccountManager: &eolusagereport.Person{Email: "not-an-address"}, TechnicalOwner: &eolusagereport.Person{Email: "Someone <x@example.com>"}},
		{AccountManager: &eolusagereport.Person{Email: "a@example.com, b@example.com"}, TechnicalOwner: &eolusagereport.Person{Email: "TO@EXAMPLE.COM"}},
	}}}}

	got, dropped := eolusagescan.Audience(r, []string{"lead@leadership.example.net"}, []string{"example.com"})
	want := []string{"lead@leadership.example.net", "am@example.com", "TO@EXAMPLE.COM"}
	if !reflect.DeepEqual(got, want) || dropped != 4 {
		t.Errorf("with domains: Audience = %v (dropped %d), want %v (dropped 4)", got, dropped, want)
	}

	got, dropped = eolusagescan.Audience(r, nil, nil)
	want = []string{"am@example.com", "owner@other.org", "TO@EXAMPLE.COM"}
	if !reflect.DeepEqual(got, want) || dropped != 3 {
		t.Errorf("no domains: Audience = %v (dropped %d), want %v (dropped 3)", got, dropped, want)
	}
}

func TestSendReportMailsOnlyAllowedDomains(t *testing.T) {
	email := &fakeEmail{}
	handler := eolusagescan.SendReport(&fakeReports{report: sampleReport()}, email, []string{"lead@example.com"}, nil,
		eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 11, 2), AllowedRecipientDomains: []string{"wso2.example"}})
	if err := handler(context.Background()); err != nil || email.calls != 1 {
		t.Fatalf("err=%v sent=%d", err, email.calls)
	}
	if !reflect.DeepEqual(email.to, []string{"lead@example.com"}) {
		t.Errorf("to = %v, want only the configured leadership address", email.to)
	}
}

func TestAudienceAndCc(t *testing.T) {
	got, dropped := eolusagescan.Audience(sampleReport(), []string{" lead@example.com ", "", "AM.ONE@example.com"}, nil)
	want := []string{"lead@example.com", "AM.ONE@example.com", "TO.One@example.com"}
	if !reflect.DeepEqual(got, want) || dropped != 0 {
		t.Errorf("Audience = %v (dropped %d), want %v", got, dropped, want)
	}
	if cc := eolusagescan.Cc([]string{"a@x.com"}, []string{"A@x.com", "b@x.com", "b@x.com", " "}); !reflect.DeepEqual(cc, []string{"b@x.com"}) {
		t.Errorf("Cc = %v, want [b@x.com]", cc)
	}
}

func TestSendReportUsesTheClaimedPeriodOverTheClock(t *testing.T) {
	// The engine claimed the first Monday's period (7 Dec), but the handler
	// runs after the next Monday's firing on the clock (14 Dec 04:31): the
	// claimed period decides, so it sends; the report is as of that Monday.
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{}
	late := func() time.Time { return time.Date(2026, 12, 14, 4, 31, 0, 0, time.UTC) }
	handler := eolusagescan.SendReport(reports, email, []string{"lead@example.com"}, nil,
		eolusagescan.Options{EmailsEnabled: true, Now: late, Anchor: late})
	ctx := schedule.WithPeriod(context.Background(), time.Date(2026, 12, 7, 4, 30, 0, 0, time.UTC))
	if err := handler(ctx); err != nil || email.calls != 1 {
		t.Fatalf("err=%v sent=%d, want the claimed first-Monday period to send", err, email.calls)
	}
	if reports.gotAsOf != "2026-12-07" {
		t.Errorf("asOf = %q, want 2026-12-07", reports.gotAsOf)
	}
}

// With only a debug list (no "to"), the report is still built and sent to
// the debug list, marked as a test copy: that is how it is tried without
// its real recipients.
func TestSendReportDebugListWithoutRecipientsStillSends(t *testing.T) {
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{}
	debug := []string{"debug@example.invalid"}
	handler := eolusagescan.SendReport(reports, notify.ForReport(email, debug), nil, nil,
		eolusagescan.Options{EmailsEnabled: true, Now: at(2026, 12, 7)})
	if err := handler(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if reports.calls != 1 || email.calls != 1 {
		t.Fatalf("fetched %d, sent %d; want 1 and 1", reports.calls, email.calls)
	}
	if !reflect.DeepEqual(email.to, debug) {
		t.Errorf("to = %v, want the debug list %v", email.to, debug)
	}
	if !strings.Contains(email.subject, "[Test copy]") {
		t.Errorf("subject = %q, want a test copy", email.subject)
	}
}
