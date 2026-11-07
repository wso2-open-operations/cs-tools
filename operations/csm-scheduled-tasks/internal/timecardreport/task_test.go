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

package timecardreport

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/timecards"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestPreviousMonth(t *testing.T) {
	tests := []struct {
		name, period, from, to string
	}{
		{"mid year", "2030-05-02T05:30:00Z", "2030-04-01", "2030-04-30"},
		{"31 day month before", "2030-04-02T05:30:00Z", "2030-03-01", "2030-03-31"},
		{"january rolls the year back", "2031-01-02T05:30:00Z", "2030-12-01", "2030-12-31"},
		{"march after a leap february", "2032-03-02T05:30:00Z", "2032-02-01", "2032-02-29"},
		{"march after a plain february", "2031-03-02T05:30:00Z", "2031-02-01", "2031-02-28"},
		{"century year is not leap", "2100-03-02T05:30:00Z", "2100-02-01", "2100-02-28"},
		{"first instant of the month", "2030-07-01T00:00:00Z", "2030-06-01", "2030-06-30"},
		{"last instant of the month", "2030-07-31T23:59:59Z", "2030-06-01", "2030-06-30"},
		{"a non-UTC period is read in UTC", "2030-05-01T00:30:00+02:00", "2030-03-01", "2030-03-31"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			from, to := PreviousMonth(at(tc.period))
			if got := from.Format(dateLayout); got != tc.from {
				t.Errorf("from = %s, want %s", got, tc.from)
			}
			if got := to.Format(dateLayout); got != tc.to {
				t.Errorf("to = %s, want %s", got, tc.to)
			}
		})
	}
}

func TestParseProjectKeys(t *testing.T) {
	got := ParseProjectKeys(" PRJ-2 , PRJ-1,,prj-2 ,PRJ-3, ")
	if want := []string{"PRJ-2", "PRJ-1", "PRJ-3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, raw := range []string{"", "  ", " , ,"} {
		if got := ParseProjectKeys(raw); len(got) != 0 {
			t.Errorf("%q: got %v, want none", raw, got)
		}
	}
}

func TestRecipientsDedupeAndFilter(t *testing.T) {
	got := Recipients([]timecards.Contact{
		{Email: "jane.doe@example.com"},
		{Email: "  "},
		{Email: ""},
		{Email: "JANE.DOE@example.com"},
		{Email: " john.roe@example.com "},
		{Email: "bot@example.com", IsCsIntegrationUser: true},
	})
	if want := []string{"jane.doe@example.com", "john.roe@example.com"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// fakeSource is a Source with canned answers and a record of what was asked.
type fakeSource struct {
	projects     map[string][]timecards.Project // search text to results
	contacts     map[string][]timecards.Contact // project id to contacts
	reports      map[string]timecards.Report    // project id to report
	searchErr    map[string]error
	contactsErr  map[string]error
	reportErr    map[string]error
	searches     []string
	contactCalls []string
	reportCalls  [][3]string
}

func (f *fakeSource) SearchProjects(_ context.Context, q string) ([]timecards.Project, error) {
	f.searches = append(f.searches, q)
	if err := f.searchErr[q]; err != nil {
		return nil, err
	}
	return f.projects[q], nil
}

func (f *fakeSource) ProjectContacts(_ context.Context, id string) ([]timecards.Contact, error) {
	f.contactCalls = append(f.contactCalls, id)
	if err := f.contactsErr[id]; err != nil {
		return nil, err
	}
	return f.contacts[id], nil
}

func (f *fakeSource) Report(_ context.Context, id, from, to string) (timecards.Report, error) {
	f.reportCalls = append(f.reportCalls, [3]string{id, from, to})
	if err := f.reportErr[id]; err != nil {
		return timecards.Report{}, err
	}
	return f.reports[id], nil
}

type sentMail struct {
	to, cc        []string
	subject, body string
}

type fakeMail struct {
	sent   []sentMail
	failTo map[string]bool // an address whose send fails
}

func (m *fakeMail) SendEmail(_ context.Context, to, cc []string, subject, body string) error {
	for _, a := range to {
		if m.failTo[a] {
			return errors.New("mail refused")
		}
	}
	m.sent = append(m.sent, sentMail{to: to, cc: cc, subject: subject, body: body})
	return nil
}

func report(id, key, name string, cases ...timecards.Case) timecards.Report {
	r := timecards.Report{Project: timecards.ReportProject{ID: id, Key: key, Name: name}, From: "2030-04-01", To: "2030-04-30", Cases: cases}
	for _, c := range cases {
		r.Totals.TotalMinutes += c.TotalMinutes
		r.Totals.BillableMinutes += c.BillableMinutes
		r.Totals.NonBillableMinutes += c.NonBillableMinutes
		r.Totals.PatchingMinutes += c.PatchingMinutes
	}
	return r
}

var oneCase = timecards.Case{CaseID: "00000000-0000-0000-0000-000000000001", CaseNumber: "CS0000001",
	ExternalCaseID: "EXT-1", CaseType: "case", TotalMinutes: 90, BillableMinutes: 60, NonBillableMinutes: 30, PatchingMinutes: 15}

func period() time.Time { return at("2030-05-02T05:30:00Z") }

func TestResolveProject(t *testing.T) {
	ctx := context.Background()
	src := &fakeSource{
		projects: map[string][]timecards.Project{
			// A substring search: the real project sits among look-alikes.
			"PRJ-1": {{ID: "id-10", Key: "PRJ-10"}, {ID: "id-1", Key: "prj-1", Name: "Acme Corp"}, {ID: "id-11", Key: "PRJ-11"}},
			"NONE":  {{ID: "id-x", Key: "NONE-2"}},
			"DUP":   {{ID: "id-a", Key: "DUP"}, {ID: "id-b", Key: "dup"}},
		},
		searchErr: map[string]error{"BOOM": errors.New("upstream returned 500")},
	}
	p, err := ResolveProject(ctx, src, "PRJ-1")
	if err != nil || p.ID != "id-1" {
		t.Fatalf("exact match among substring results: got %+v, %v", p, err)
	}
	if _, err := ResolveProject(ctx, src, "NONE"); err == nil || !strings.Contains(err.Error(), "no project has the key") {
		t.Errorf("none: err = %v", err)
	}
	if _, err := ResolveProject(ctx, src, "DUP"); err == nil || !strings.Contains(err.Error(), "2 projects have the key") {
		t.Errorf("ambiguous: err = %v", err)
	}
	if _, err := ResolveProject(ctx, src, "BOOM"); err == nil || !strings.Contains(err.Error(), "search project") {
		t.Errorf("search failure: err = %v", err)
	}
}

func TestSendReportsPerRecipientPrivacy(t *testing.T) {
	src := &fakeSource{
		projects: map[string][]timecards.Project{"PRJ-1": {{ID: "id-1", Key: "PRJ-1", Name: "Acme Corp"}}},
		contacts: map[string][]timecards.Contact{"id-1": {
			{Email: "jane.doe@example.com"}, {Email: "john.roe@example.com"}, {Email: "JANE.DOE@example.com"}}},
		reports: map[string]timecards.Report{"id-1": report("id-1", "PRJ-1", "Acme Corp", oneCase)},
	}
	mail := &fakeMail{}
	err := SendReports(src, mail, period, Config{ProjectKeys: []string{"PRJ-1"}, Cc: []string{"ops@example.com"}}, true)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 2 {
		t.Fatalf("mails = %d, want 2 (one per distinct contact)", len(mail.sent))
	}
	for i, m := range mail.sent {
		if len(m.to) != 1 {
			t.Errorf("mail %d has %d To addresses, want 1", i, len(m.to))
		}
		for j, other := range mail.sent {
			if i != j && (strings.Contains(m.body, other.to[0]) || strings.Contains(strings.Join(m.cc, ","), other.to[0])) {
				t.Errorf("mail %d carries another recipient's address", i)
			}
		}
		if !reflect.DeepEqual(m.cc, []string{"ops@example.com"}) {
			t.Errorf("mail %d cc = %v", i, m.cc)
		}
		if m.subject != "Time Card Report - Acme Corp - April 2030" {
			t.Errorf("subject = %q", m.subject)
		}
	}
	if want := [][3]string{{"id-1", "2030-04-01", "2030-04-30"}}; !reflect.DeepEqual(src.reportCalls, want) {
		t.Errorf("report calls = %v, want %v", src.reportCalls, want)
	}
}

func TestSendReportsExplicitToIsOneMailAndSkipsContacts(t *testing.T) {
	src := &fakeSource{
		projects: map[string][]timecards.Project{"PRJ-1": {{ID: "id-1", Key: "PRJ-1"}}},
		reports:  map[string]timecards.Report{"id-1": report("id-1", "PRJ-1", "", oneCase)},
	}
	mail := &fakeMail{}
	to := []string{"jane.doe@example.com", "john.roe@example.com"}
	err := SendReports(src, mail, period, Config{ProjectKeys: []string{"PRJ-1"}, To: to}, true)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(src.contactCalls) != 0 {
		t.Errorf("contacts were fetched with an explicit To: %v", src.contactCalls)
	}
	if len(mail.sent) != 1 || !reflect.DeepEqual(mail.sent[0].to, to) {
		t.Fatalf("sent = %+v, want one mail to %v", mail.sent, to)
	}
	// No project name: the key stands in.
	if mail.sent[0].subject != "Time Card Report - PRJ-1 - April 2030" {
		t.Errorf("subject = %q", mail.sent[0].subject)
	}
}

func TestSendReportsZeroRecipientsSkipsWithoutFailing(t *testing.T) {
	src := &fakeSource{
		projects: map[string][]timecards.Project{"PRJ-1": {{ID: "id-1", Key: "PRJ-1"}}},
		contacts: map[string][]timecards.Contact{"id-1": {{Email: ""}, {Email: "bot@example.com", IsCsIntegrationUser: true}}},
	}
	mail := &fakeMail{}
	if err := SendReports(src, mail, period, Config{ProjectKeys: []string{"PRJ-1"}}, true)(context.Background()); err != nil {
		t.Fatalf("zero recipients must not fail: %v", err)
	}
	if len(mail.sent) != 0 || len(src.reportCalls) != 0 {
		t.Errorf("sent %d mails and fetched %d reports for a project with nobody to mail", len(mail.sent), len(src.reportCalls))
	}
}

func TestSendReportsEmptyCasesStillSends(t *testing.T) {
	src := &fakeSource{
		projects: map[string][]timecards.Project{"PRJ-1": {{ID: "id-1", Key: "PRJ-1", Name: "Acme Corp"}}},
		contacts: map[string][]timecards.Contact{"id-1": {{Email: "jane.doe@example.com"}}},
		reports:  map[string]timecards.Report{"id-1": report("id-1", "PRJ-1", "Acme Corp")},
	}
	mail := &fakeMail{}
	if err := SendReports(src, mail, period, Config{ProjectKeys: []string{"PRJ-1"}}, true)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 1 || !strings.Contains(mail.sent[0].body, "No time card data found for the selected period.") {
		t.Fatalf("want one mail with the no-data card, got %+v", mail.sent)
	}
}

func TestSendReportsPartialFailureNamesProjectsAndContinues(t *testing.T) {
	src := &fakeSource{
		projects: map[string][]timecards.Project{
			"PRJ-1": {{ID: "id-1", Key: "PRJ-1"}},
			"PRJ-2": {{ID: "id-2", Key: "PRJ-2"}},
			"PRJ-3": {{ID: "id-3", Key: "PRJ-3"}},
			"PRJ-4": {{ID: "id-4", Key: "PRJ-4"}},
			"PRJ-5": {}, // unknown key
		},
		contacts: map[string][]timecards.Contact{
			"id-1": {{Email: "jane.doe@example.com"}},
			"id-2": {{Email: "john.roe@example.com"}},
			"id-3": {{Email: "ann.poe@example.com"}},
			"id-4": {{Email: "bob.moe@example.com"}, {Email: "cy.lee@example.com"}},
		},
		reports: map[string]timecards.Report{
			"id-1": report("id-1", "PRJ-1", "One", oneCase),
			"id-3": report("id-3", "PRJ-3", "Three", oneCase),
			"id-4": report("id-4", "PRJ-4", "Four", oneCase),
		},
		reportErr: map[string]error{"id-2": errors.New("upstream returned 500")},
	}
	mail := &fakeMail{failTo: map[string]bool{"cy.lee@example.com": true}}
	keys := []string{"PRJ-1", "PRJ-2", "PRJ-3", "PRJ-4", "PRJ-5"}
	err := SendReports(src, mail, period, Config{ProjectKeys: keys}, true)(context.Background())
	if err == nil {
		t.Fatal("want a combined error")
	}
	msg := err.Error()
	for _, want := range []string{"3 of 5 projects failed (PRJ-2, PRJ-4, PRJ-5)", "project PRJ-2:", "project PRJ-4:", "project PRJ-5:"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q lacks %q", msg, want)
		}
	}
	for _, leaked := range []string{"jane.doe", "john.roe", "ann.poe", "bob.moe", "cy.lee"} {
		if strings.Contains(msg, leaked) {
			t.Errorf("error leaks an address: %q", msg)
		}
	}
	// PRJ-1, PRJ-3 and the good half of PRJ-4 went out despite the failures.
	var got []string
	for _, m := range mail.sent {
		got = append(got, m.to[0])
	}
	if want := []string{"jane.doe@example.com", "ann.poe@example.com", "bob.moe@example.com"}; !reflect.DeepEqual(got, want) {
		t.Errorf("sent to %v, want %v", got, want)
	}
}

func TestSendReportsNoKeysIsANoOp(t *testing.T) {
	src := &fakeSource{}
	mail := &fakeMail{}
	for _, keys := range [][]string{nil, {}} {
		if err := SendReports(src, mail, period, Config{ProjectKeys: keys}, true)(context.Background()); err != nil {
			t.Fatalf("keys %v: %v", keys, err)
		}
	}
	if len(src.searches) != 0 || len(mail.sent) != 0 {
		t.Errorf("a no-op run called the service: searches=%v mails=%d", src.searches, len(mail.sent))
	}
}

func TestSendReportsDisabledSendsNothing(t *testing.T) {
	src := &fakeSource{}
	mail := &fakeMail{}
	if err := SendReports(src, mail, period, Config{ProjectKeys: []string{"PRJ-1"}}, false)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(src.searches) != 0 {
		t.Errorf("ALERTS_ENABLED=false must not reach the service: %v", src.searches)
	}
}

// A retry of the same period computes the same window, however late it runs.
func TestSendReportsRetryUsesSameWindow(t *testing.T) {
	src := &fakeSource{
		projects: map[string][]timecards.Project{"PRJ-1": {{ID: "id-1", Key: "PRJ-1"}}},
		contacts: map[string][]timecards.Contact{"id-1": {{Email: "jane.doe@example.com"}}},
		reports:  map[string]timecards.Report{"id-1": report("id-1", "PRJ-1", "A", oneCase)},
	}
	h := SendReports(src, &fakeMail{}, period, Config{ProjectKeys: []string{"PRJ-1"}}, true)
	for i := 0; i < 2; i++ {
		if err := h(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(src.reportCalls) != 2 || src.reportCalls[0] != src.reportCalls[1] {
		t.Errorf("report calls differ between attempts: %v", src.reportCalls)
	}
}

// The run logs which project it reported on and the figures it carried, without
// any recipient address.
func TestSendReportsLogsProjectKeyAndSummary(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	src := &fakeSource{
		projects: map[string][]timecards.Project{"PRJ-1": {{ID: "id-1", Key: "PRJ-1", Name: "Acme Corp"}}},
		contacts: map[string][]timecards.Contact{"id-1": {{Email: "jane.doe@example.com"}}},
		reports:  map[string]timecards.Report{"id-1": report("id-1", "PRJ-1", "Acme Corp", oneCase)},
	}
	if err := SendReports(src, &fakeMail{}, period, Config{ProjectKeys: []string{"PRJ-1"}}, true)(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"sending the time card report", "sent the time card report",
		"project=PRJ-1", "from=2030-04-01", "to=2030-04-30", "cases=1",
		"totalMinutes=90", "billableMinutes=60", "nonBillableMinutes=30", "patchingMinutes=15",
		"mailsSent=1", "mailsFailed=0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "jane.doe@example.com") {
		t.Errorf("log carries a recipient address:\n%s", out)
	}
}
