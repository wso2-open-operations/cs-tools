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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/timecards"
)

func TestFormatMinutes(t *testing.T) {
	tests := map[int]string{0: "0h 0m", 1: "0h 1m", 59: "0h 59m", 60: "1h 0m", 61: "1h 1m", 119: "1h 59m", 120: "2h 0m", 6005: "100h 5m", -90: "-1h 30m"}
	for in, want := range tests {
		if got := notify.FormatMinutes(in); got != want {
			t.Errorf("FormatMinutes(%d) = %q, want %q", in, got, want)
		}
	}
}

func sampleTimeCardReport() timecards.Report {
	return timecards.Report{
		Project: timecards.ReportProject{ID: "00000000-0000-0000-0000-000000000000", Key: "PRJ-1", Name: "Acme Corp"},
		From:    "2030-04-01", To: "2030-04-30",
		Totals: timecards.Totals{TotalMinutes: 535, BillableMinutes: 355, NonBillableMinutes: 180, PatchingMinutes: 75},
		Cases: []timecards.Case{
			{CaseNumber: "CS0000001", ExternalCaseID: "EXT-1001", CaseType: "case", TotalMinutes: 270, BillableMinutes: 210, NonBillableMinutes: 60, PatchingMinutes: 0},
			{CaseNumber: "CS0000002", ExternalCaseID: "", CaseType: "service_request", TotalMinutes: 205, BillableMinutes: 145, NonBillableMinutes: 60, PatchingMinutes: 75},
			{CaseNumber: "CS0000003", ExternalCaseID: "EXT-<b>3</b>", CaseType: "", TotalMinutes: 60, BillableMinutes: 0, NonBillableMinutes: 60, PatchingMinutes: 0},
		},
	}
}

func TestRenderTimeCardReport(t *testing.T) {
	html := notify.RenderTimeCardReport(sampleTimeCardReport())
	for _, want := range []string{
		"PRJ-1", "Acme Corp", "2030-04-01", "2030-04-30",
		"Total effort", "Billable", "Non-Billable", "Patching",
		"8h 55m", "5h 55m", "3h 0m", "1h 15m", // the four cards
		"CS0000001", "EXT-1001", "Service request", ">Case<", "4h 30m", "1h 0m",
		"<strong>TOTAL</strong>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	if strings.Contains(html, "EXT-<b>3</b>") || !strings.Contains(html, "EXT-&lt;b&gt;3&lt;/b&gt;") {
		t.Error("a dynamic value was not escaped")
	}
	if strings.Contains(html, "No time card data") {
		t.Error("a report with cases shows the no-data card")
	}
	if strings.Contains(html, "<!-- [") {
		t.Error("an unfilled placeholder is left in the mail")
	}
	// Email-client safe: no layout CSS a mail client drops, no scripts, no
	// vendor logo or name baked into the template.
	for _, banned := range []string{"display:flex", "display:grid", "<script", "cachefly", "WSO2"} {
		if strings.Contains(html, banned) {
			t.Errorf("template contains %q", banned)
		}
	}
	if out := os.Getenv("TIME_CARD_REPORT_SAMPLE_OUT"); out != "" {
		if err := os.WriteFile(out, []byte(html), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRenderTimeCardReportEmpty(t *testing.T) {
	r := sampleTimeCardReport()
	r.Cases, r.Totals = nil, timecards.Totals{}
	html := notify.RenderTimeCardReport(r)
	if !strings.Contains(html, "No time card data found for the selected period.") {
		t.Error("no-data card missing")
	}
	if !strings.Contains(html, "PRJ-1") || !strings.Contains(html, "2030-04-30") {
		t.Error("header missing from the no-data mail")
	}
	if strings.Contains(html, "TOTAL") || strings.Contains(html, "<table border=\"0\" cellpadding=\"0\" cellspacing=\"0\" width=\"100%\" style=\"border-collapse:collapse;\">") {
		t.Error("the case table is shown with no cases")
	}
	if out := os.Getenv("TIME_CARD_REPORT_EMPTY_OUT"); out != "" {
		if err := os.WriteFile(out, []byte(html), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// The report goes to external contacts, so it must carry no link at all, in
// particular none into the internal portal. The only URLs allowed are the
// two XML namespace identifiers every XHTML mail declares.
func TestTimeCardReportHasNoLinks(t *testing.T) {
	empty := sampleTimeCardReport()
	empty.Cases, empty.Totals = nil, timecards.Totals{}
	for name, html := range map[string]string{
		"with cases": notify.RenderTimeCardReport(sampleTimeCardReport()),
		"no data":    notify.RenderTimeCardReport(empty),
	} {
		html = strings.NewReplacer(
			"http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd", "",
			"http://www.w3.org/1999/xhtml", "",
		).Replace(html)
		for _, banned := range []string{"href", "<a ", "<img", "src=", "http://", "https://", "mailto:"} {
			if strings.Contains(strings.ToLower(html), banned) {
				t.Errorf("%s: the external report contains %q", name, banned)
			}
		}
	}
}

func TestTimeCardReportSubject(t *testing.T) {
	r := sampleTimeCardReport()
	if got, want := notify.TimeCardReportSubject(r), "Time Card Report - Acme Corp - April 2030"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	r.Project.Name = ""
	r.From = "2031-01-01"
	if got, want := notify.TimeCardReportSubject(r), "Time Card Report - PRJ-1 - January 2031"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	r.Project.Name = "Line\r\nBreak"
	if got := notify.TimeCardReportSubject(r); strings.ContainsAny(got, "\r\n") {
		t.Errorf("subject has a line break: %q", got)
	}
}
