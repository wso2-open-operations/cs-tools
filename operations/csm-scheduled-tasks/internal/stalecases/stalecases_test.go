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

package stalecases_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/longopenreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/stalecases"
)

type fakeReports struct {
	report longopenreport.Report
	err    error
	calls  int
	gotAge int
}

func (f *fakeReports) Report(_ context.Context, olderThanDays int) (longopenreport.Report, error) {
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

func oneCase() longopenreport.Report {
	return longopenreport.Report{
		Cases:      []longopenreport.Row{{Number: "CS0000001", AssignedEngineer: "Jane Doe"}},
		CasesTotal: 1,
	}
}

func TestSendReportSendsToConfiguredRecipients(t *testing.T) {
	reports := &fakeReports{report: oneCase()}
	email := &fakeEmail{}
	to, cc := []string{"team@example.com"}, []string{"lead@example.com"}

	if err := stalecases.SendReport(reports, email, 30, to, cc, "", true)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if reports.gotAge != 30 {
		t.Errorf("olderThanDays asked for = %d, want 30", reports.gotAge)
	}
	if email.calls != 1 || !reflect.DeepEqual(email.to, to) || !reflect.DeepEqual(email.cc, cc) {
		t.Fatalf("calls=%d to=%v cc=%v", email.calls, email.to, email.cc)
	}
	if email.subject != "[Action-Required][Report] Cases open for more than 30 days" {
		t.Errorf("subject = %q", email.subject)
	}
	if !strings.Contains(email.body, "CS0000001") {
		t.Errorf("body lacks the case")
	}
}

func TestSendReportOnlyOnboardingRowsStillSends(t *testing.T) {
	reports := &fakeReports{report: longopenreport.Report{
		OnboardingCases: []longopenreport.Row{{Number: "CS0000002"}}, OnboardingCasesTotal: 1,
	}}
	email := &fakeEmail{}
	if err := stalecases.SendReport(reports, email, 30, []string{"team@example.com"}, nil, "", true)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if email.calls != 1 {
		t.Errorf("SendEmail calls = %d, want 1", email.calls)
	}
}

func TestSendReportBothTablesEmptySendsNothing(t *testing.T) {
	reports := &fakeReports{}
	email := &fakeEmail{}
	if err := stalecases.SendReport(reports, email, 30, []string{"team@example.com"}, nil, "", true)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if reports.calls != 1 || email.calls != 0 {
		t.Errorf("fetches=%d sends=%d, want 1 and 0", reports.calls, email.calls)
	}
}

func TestSendReportNoToSkipsFetch(t *testing.T) {
	reports := &fakeReports{report: oneCase()}
	email := &fakeEmail{}
	if err := stalecases.SendReport(reports, email, 30, nil, []string{"lead@example.com"}, "", true)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if reports.calls != 0 || email.calls != 0 {
		t.Errorf("fetches=%d sends=%d, want 0 and 0", reports.calls, email.calls)
	}
}

func TestSendReportAlertsDisabledSkipsFetch(t *testing.T) {
	reports := &fakeReports{report: oneCase()}
	email := &fakeEmail{}
	if err := stalecases.SendReport(reports, email, 30, []string{"team@example.com"}, nil, "", false)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if reports.calls != 0 || email.calls != 0 {
		t.Errorf("fetches=%d sends=%d, want 0 and 0", reports.calls, email.calls)
	}
}

func TestSendReportFetchErrorIsReturned(t *testing.T) {
	boom := errors.New("boom")
	reports := &fakeReports{err: boom}
	email := &fakeEmail{}
	err := stalecases.SendReport(reports, email, 30, []string{"team@example.com"}, nil, "", true)(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap the fetch error", err)
	}
	if email.calls != 0 {
		t.Errorf("SendEmail calls = %d, want 0", email.calls)
	}
}

func TestSendReportSendErrorIsReturned(t *testing.T) {
	boom := errors.New("smtp down")
	reports := &fakeReports{report: oneCase()}
	email := &fakeEmail{err: boom}
	err := stalecases.SendReport(reports, email, 30, []string{"team@example.com"}, nil, "", true)(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap the send error", err)
	}
}
