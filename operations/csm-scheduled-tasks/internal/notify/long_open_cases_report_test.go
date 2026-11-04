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

package notify_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/longopenreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

func longOpenFixture() longopenreport.Report {
	created := time.Date(2026, 8, 1, 4, 5, 0, 0, time.UTC)
	updated := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	return longopenreport.Report{
		OlderThanDays: 30,
		Cases: []longopenreport.Row{
			{Number: "CS0000001", InternalID: "WSO2-1001", CreatedOn: created, UpdatedOn: updated,
				State: "work_in_progress", Severity: "high", AssignedEngineer: "Jane Doe", CsTeam: "Team Alpha"},
			{Number: "CS0000002", CreatedOn: created, UpdatedOn: updated, State: "waiting_on_wso2",
				AssignedEngineer: "<script>alert(1)</script>", CsTeam: "Team <b>Beta</b>"},
		},
		CasesTotal:     1500,
		CasesTruncated: true,
		// OnboardingCases left empty.
	}
}

func TestRenderLongOpenCasesReport(t *testing.T) {
	body := notify.RenderLongOpenCasesReport(longOpenFixture(), 30)

	for _, want := range []string{
		"<strong>30 days</strong>", "need action",
		"CS0000001", "WSO2-1001", "Team Alpha", "Jane Doe", "Work In Progress", "high",
		"Waiting On WSO2",
		"2026-10-30 15:30", "2026-08-01 09:35", // Asia/Colombo
		"Showing only the oldest 2 of 1500.",
		"No cases.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q", want)
		}
	}
	// Table order, labels and column order.
	ongoing := strings.Index(body, ">Ongoing cases<")
	onboarding := strings.Index(body, ">Ongoing cases on onboarding projects<")
	if ongoing < 0 || onboarding < 0 || ongoing > onboarding {
		t.Errorf("headings missing or out of order: ongoing=%d onboarding=%d", ongoing, onboarding)
	}
	cols := []string{">Case<", ">WSO2 Case ID<", ">Integration CS Team<", ">Assignee<", ">State<", ">Priority<", ">Last updated<", ">Created<"}
	pos := 0
	for _, c := range cols {
		i := strings.Index(body[pos:], c)
		if i < 0 {
			t.Fatalf("column %q missing or out of order", c)
		}
		pos += i + len(c)
	}
	// The onboarding table is empty and not truncated: exactly one note, one empty line.
	if strings.Count(body, "Showing only") != 1 || strings.Count(body, "No cases.") != 1 {
		t.Errorf("notes: showing=%d empty=%d", strings.Count(body, "Showing only"), strings.Count(body, "No cases."))
	}
	// Hostile values are escaped.
	if strings.Contains(body, "<script>") || strings.Contains(body, "<b>Beta") {
		t.Errorf("body carries unescaped data")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("escaped script text missing")
	}
	if strings.Contains(body, "<!-- [") {
		t.Errorf("body still carries a template placeholder")
	}
}

func TestRenderLongOpenCasesReportCaseNumbersArePlainText(t *testing.T) {
	body := notify.RenderLongOpenCasesReport(longOpenFixture(), 30)
	if strings.Contains(body, "<a ") || !strings.Contains(body, "CS0000001") {
		t.Errorf("case numbers must be plain text, never links")
	}
}

func TestRenderLongOpenCasesReportBothTruncated(t *testing.T) {
	r := longOpenFixture()
	r.OnboardingCases = []longopenreport.Row{{Number: "CS0000009"}}
	r.OnboardingCasesTotal, r.OnboardingCasesTruncated = 40, true
	body := notify.RenderLongOpenCasesReport(r, 30)
	if !strings.Contains(body, "Showing only the oldest 1 of 40.") || strings.Count(body, "Showing only") != 2 {
		t.Errorf("both tables should carry a note")
	}
}

// Writes the rendered fixture to LONG_OPEN_HTML_OUT so the layout can be
// eyeballed; sends nothing.
func TestRenderLongOpenCasesReportToFile(t *testing.T) {
	out := os.Getenv("LONG_OPEN_HTML_OUT")
	if out == "" {
		t.Skip("LONG_OPEN_HTML_OUT is not set")
	}
	body := notify.RenderLongOpenCasesReport(longOpenFixture(), 30)
	if err := os.WriteFile(out, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
