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

package migrationscan_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/migrationreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/migrationscan"
)

type fakeReports struct {
	report migrationreport.Report
	err    error
	calls  int
	gotAge int
}

func (f *fakeReports) Report(_ context.Context, olderThanDays int) (migrationreport.Report, error) {
	f.calls++
	f.gotAge = olderThanDays
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

func sampleReport() migrationreport.Report {
	created := time.Date(2026, 3, 2, 9, 30, 0, 0, time.UTC)
	commented := time.Date(2026, 9, 1, 4, 5, 0, 0, time.UTC)
	return migrationreport.Report{
		GeneratedAt:   time.Date(2026, 10, 18, 22, 30, 0, 0, time.UTC),
		OlderThanDays: 180,
		CreatedBefore: time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC),
		OnHold: []migrationreport.Row{{
			ID: "11111111-1111-1111-1111-111111111111", Number: "ENG0000001", IssueKey: "ACME-MIG-1",
			CreatedOn: created, LastCustomerCommentOn: &commented, AssignedTo: "Dana Example",
			Product: "Example Product", Account: "Acme", CsTeam: "Team Alpha", SalesRegion: "EU",
			AccountManager: "Morgan Example", TechnicalOwner: "Sam Example",
		}},
		WaitingOnAccounts: []migrationreport.Row{},
		InProgress: []migrationreport.Row{{
			ID: "22222222-2222-2222-2222-222222222222", Number: "ENG0000002", IssueKey: "ACME-MIG-2",
			CreatedOn: created, Account: "Acme", CsTeam: "Team <Beta>",
		}},
		Recipients: []string{"team-alpha@example.com", "team-beta@example.com"},
	}
}

func TestSendReportMergesConfiguredAndDerivedRecipients(t *testing.T) {
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{}
	handler := migrationscan.SendReport(reports, email, 180,
		[]string{"leads@example.com", " Team-Alpha@example.com "}, []string{"management@example.com", "leads@example.com"},
		"https://support.example.com/cases/", true)

	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if reports.gotAge != 180 {
		t.Errorf("olderThanDays asked for = %d, want 180", reports.gotAge)
	}
	if email.calls != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", email.calls)
	}
	wantTo := []string{"leads@example.com", "Team-Alpha@example.com", "team-beta@example.com"}
	if strings.Join(email.to, ",") != strings.Join(wantTo, ",") {
		t.Errorf("to = %v, want %v (configured first, derived after, no case-insensitive duplicates)", email.to, wantTo)
	}
	if strings.Join(email.cc, ",") != "management@example.com" {
		t.Errorf("cc = %v, want only management@example.com: an address already on To is not copied", email.cc)
	}
	if email.subject != migrationscan.Subject {
		t.Errorf("subject = %q", email.subject)
	}
	for _, want := range []string{
		"ENG0000001", "ACME-MIG-1", "https://support.example.com/cases/11111111-1111-1111-1111-111111111111",
		"Dana Example", "Team Alpha", "EU", "Morgan Example / Sam Example", "2026-03-02 15:00:00", "2026-09-01 09:35:00",
		"<strong>180 days</strong>",
		"Team &lt;Beta&gt;",
	} {
		if !strings.Contains(email.body, want) {
			t.Errorf("body lacks %q", want)
		}
	}
	if strings.Contains(email.body, "<Beta>") {
		t.Errorf("body carries an unescaped team name")
	}
	if strings.Contains(email.body, "<!-- [") {
		t.Errorf("body still carries a template placeholder")
	}
}

func TestSendReportFailsWithoutAnyRecipient(t *testing.T) {
	report := sampleReport()
	report.Recipients = nil
	reports := &fakeReports{report: report}
	email := &fakeEmail{}
	handler := migrationscan.SendReport(reports, email, 180, nil, []string{"cc-only@example.com"}, "", true)

	err := handler(context.Background())
	if err == nil {
		t.Fatal("expected an error: no team mail group and no configured to")
	}
	if email.calls != 0 {
		t.Errorf("SendEmail calls = %d, want 0", email.calls)
	}
}

func TestSendReportSkipsWhenEmailsDisabled(t *testing.T) {
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{}
	handler := migrationscan.SendReport(reports, email, 180, []string{"a@example.com"}, nil, "", false)

	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if reports.calls != 0 || email.calls != 0 {
		t.Errorf("calls = reports %d, email %d; want none when emails are disabled", reports.calls, email.calls)
	}
}

func TestSendReportWrapsFetchAndSendErrors(t *testing.T) {
	boom := errors.New("boom")
	handler := migrationscan.SendReport(&fakeReports{err: boom}, &fakeEmail{}, 180, []string{"a@example.com"}, nil, "", true)
	if err := handler(context.Background()); !errors.Is(err, boom) {
		t.Errorf("fetch error = %v, want wrapped %v", err, boom)
	}

	email := &fakeEmail{err: boom}
	handler = migrationscan.SendReport(&fakeReports{report: sampleReport()}, email, 180, []string{"a@example.com"}, nil, "", true)
	if err := handler(context.Background()); !errors.Is(err, boom) {
		t.Errorf("send error = %v, want wrapped %v", err, boom)
	}
}

func TestSendReportSendsEmptyTables(t *testing.T) {
	report := sampleReport()
	report.OnHold, report.WaitingOnAccounts, report.InProgress = nil, nil, nil
	email := &fakeEmail{}
	handler := migrationscan.SendReport(&fakeReports{report: report}, email, 180, nil, nil, "", true)

	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if email.calls != 1 {
		t.Fatalf("an empty scan must still be sent; calls = %d", email.calls)
	}
	for _, heading := range []string{"Migrations Customer Requested to Hold", "Migrations Waiting on Accounts Team", "Migrations in Progress for Over 180 Days"} {
		if !strings.Contains(email.body, heading) {
			t.Errorf("empty scan lacks the heading %q: every table is shown, header row and all", heading)
		}
	}
	if strings.Count(email.body, "Case ID</th>") != 3 {
		t.Errorf("want three header rows, got %d", strings.Count(email.body, "Case ID</th>"))
	}
	if strings.Count(email.body, "No migrations in this table.") != 3 {
		t.Errorf("each empty table should say so; got %d", strings.Count(email.body, "No migrations in this table."))
	}
}
