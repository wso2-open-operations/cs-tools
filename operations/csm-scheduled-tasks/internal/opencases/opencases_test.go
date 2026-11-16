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

package opencases_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/longopenreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/opencases"
)

type fakeReports struct {
	report longopenreport.Report
	err    error
	calls  int
}

func (f *fakeReports) OpenBeforeYesterday(context.Context) (longopenreport.Report, error) {
	f.calls++
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
		Cases: []longopenreport.Row{{
			Number: "CS0000001", InternalID: "ID-1", State: "open", Severity: "high",
			AssignedEngineer: "Jane Doe", AccountName: "Example Corp",
			CreatedOn: time.Now().Add(-72 * time.Hour), UpdatedOn: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC),
		}},
		CasesTotal: 1,
	}
}

func TestSendReportSendsToRecipients(t *testing.T) {
	reports := &fakeReports{report: oneCase()}
	email := &fakeEmail{}
	to, cc := []string{"team@example.com"}, []string{"lead@example.com"}

	if err := opencases.SendReport(reports, email, to, cc, "", true)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if email.calls != 1 || !reflect.DeepEqual(email.to, to) || !reflect.DeepEqual(email.cc, cc) {
		t.Fatalf("calls=%d to=%v cc=%v", email.calls, email.to, email.cc)
	}
	if email.subject != "[Immediate-Action-Required][Report] Opened before yesterday cases" {
		t.Errorf("subject = %q", email.subject)
	}
	for _, want := range []string{"CS0000001", "Example Corp", "Jane Doe", "Open", "S2", "2026-11-03", ">3<"} {
		if !strings.Contains(email.body, want) {
			t.Errorf("body lacks %q", want)
		}
	}
	if strings.Contains(email.body, "This list is cut") {
		t.Error("an untruncated report must not carry the cut notice")
	}
}

func TestSendReportZeroRowsSendsNothing(t *testing.T) {
	reports := &fakeReports{report: longopenreport.Report{Cases: []longopenreport.Row{}}}
	email := &fakeEmail{}
	if err := opencases.SendReport(reports, email, []string{"team@example.com"}, nil, "", true)(context.Background()); err != nil {
		t.Fatalf("an empty report must succeed: %v", err)
	}
	if reports.calls != 1 || email.calls != 0 {
		t.Errorf("fetches=%d sends=%d, want 1 and 0", reports.calls, email.calls)
	}
}

func TestSendReportTruncatedSaysSoWithTotal(t *testing.T) {
	r := oneCase()
	r.CasesTotal, r.CasesTruncated = 5321, true
	email := &fakeEmail{}
	if err := opencases.SendReport(&fakeReports{report: r}, email, []string{"team@example.com"}, nil, "", true)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if email.calls != 1 {
		t.Fatalf("a truncated report is still sent, sends=%d", email.calls)
	}
	if !strings.Contains(email.body, "This list is cut: showing only the oldest 1 of 5321.") {
		t.Errorf("body lacks the cut notice with the total")
	}
}

func TestSendReportShortCircuits(t *testing.T) {
	for name, tc := range map[string]struct {
		to      []string
		enabled bool
	}{
		"alerts disabled":  {[]string{"team@example.com"}, false},
		"no recipients":    {nil, true},
		"empty recipients": {[]string{}, true},
	} {
		t.Run(name, func(t *testing.T) {
			reports, email := &fakeReports{report: oneCase()}, &fakeEmail{}
			if err := opencases.SendReport(reports, email, tc.to, nil, "", tc.enabled)(context.Background()); err != nil {
				t.Fatal(err)
			}
			if reports.calls != 0 || email.calls != 0 {
				t.Errorf("fetches=%d sends=%d, want none", reports.calls, email.calls)
			}
		})
	}
}

func TestSendReportErrors(t *testing.T) {
	boom := errors.New("boom")
	err := opencases.SendReport(&fakeReports{err: boom}, &fakeEmail{}, []string{"a@example.com"}, nil, "", true)(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("fetch error = %v", err)
	}
	err = opencases.SendReport(&fakeReports{report: oneCase()}, &fakeEmail{err: boom}, []string{"a@example.com"}, nil, "", true)(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("send error = %v", err)
	}
}

func TestSendReportLinksRowsByTypeAndShowsTypeColumn(t *testing.T) {
	r := longopenreport.Report{Cases: []longopenreport.Row{
		{ID: "00000000-0000-0000-0000-000000000001", Number: "CS0000001", Type: "case", State: "open", CreatedOn: time.Now().Add(-72 * time.Hour), UpdatedOn: time.Now()},
		{ID: "00000000-0000-0000-0000-000000000002", Number: "CS0000002", Type: "service_request", State: "open", CreatedOn: time.Now().Add(-72 * time.Hour), UpdatedOn: time.Now()},
		{ID: "00000000-0000-0000-0000-000000000003", Number: "CS0000003", Type: "security_report_analysis", State: "open", CreatedOn: time.Now().Add(-72 * time.Hour), UpdatedOn: time.Now()},
	}, CasesTotal: 3}
	email := &fakeEmail{}
	if err := opencases.SendReport(&fakeReports{report: r}, email, []string{"team@example.com"}, nil, "https://portal.example.com/", true)(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`href="https://portal.example.com/cases/00000000-0000-0000-0000-000000000001"`,
		`href="https://portal.example.com/operations/service-requests/00000000-0000-0000-0000-000000000002"`,
		`href="https://portal.example.com/security-center/security-reports/00000000-0000-0000-0000-000000000003"`,
		">Service request</td>", ">Security report analysis</td>",
	} {
		if !strings.Contains(email.body, want) {
			t.Errorf("body lacks %q", want)
		}
	}
}
