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

package notify

import (
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/productissuereport"
)

func piStr(s string) *string { return &s }

func piDash(t *testing.T, start, end time.Time) productissuereport.Dashboard {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	return productissuereport.Dashboard{PeriodStart: start.In(loc), PeriodEnd: end.In(loc)}
}

func TestProductIssueReportSubject(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	d := func(sy int, sm time.Month, sd int, ey int, em time.Month, ed int) productissuereport.Dashboard {
		return productissuereport.Dashboard{PeriodStart: time.Date(sy, sm, sd, 0, 0, 0, 0, loc), PeriodEnd: time.Date(ey, em, ed, 0, 0, 0, 0, loc)}
	}
	cases := []struct {
		period   string
		d        productissuereport.Dashboard
		products []string
		want     string
	}{
		{"week", d(2026, 11, 9, 2026, 11, 16), nil, "Product issues: weekly report, 9-15 Nov 2026"},
		{"week", d(2026, 11, 30, 2026, 12, 7), nil, "Product issues: weekly report, 30 Nov-6 Dec 2026"},
		{"week", d(2026, 12, 28, 2027, 1, 4), nil, "Product issues: weekly report, 28 Dec 2026-3 Jan 2027"},
		{"month", d(2026, 11, 1, 2026, 12, 1), nil, "Product issues: monthly report, November 2026"},
		{"week", d(2026, 11, 9, 2026, 11, 16), []string{"API Manager", "Identity Server"}, "Product issues: weekly report, 9-15 Nov 2026 (API Manager, Identity Server)"},
		{"month", d(2026, 11, 1, 2026, 12, 1), []string{"API Manager"}, "Product issues: monthly report, November 2026 (API Manager)"},
		// A control character in a product name never reaches the mail header.
		{"month", d(2026, 11, 1, 2026, 12, 1), []string{"Evil\r\nBcc: x@example.com"}, "Product issues: monthly report, November 2026 (Evil  Bcc: x@example.com)"},
	}
	for _, c := range cases {
		if got := ProductIssueReportSubject(c.period, c.d, c.products); got != c.want {
			t.Errorf("subject = %q, want %q", got, c.want)
		}
	}
	// A UTC-expressed period is still named in Kolkata.
	utc := productissuereport.Dashboard{
		PeriodStart: time.Date(2026, 11, 8, 18, 30, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 11, 15, 18, 30, 0, 0, time.UTC)}
	if got := ProductIssueReportSubject("week", utc, nil); got != "Product issues: weekly report, 9-15 Nov 2026" {
		t.Errorf("utc subject = %q", got)
	}
}

func TestRenderProductIssueReportEmptyData(t *testing.T) {
	body := RenderProductIssueReport(piDash(t, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)), "week", nil, "")
	for _, want := range []string{"No product issues.", "No issues are waiting on CS.", "No cases are blocked by product issues.", "No open issue has a breached SLA.", "SLA compliance"} {
		if !strings.Contains(body, want) {
			t.Errorf("empty report lacks %q", want)
		}
	}
	if strings.Contains(body, "<!-- [") {
		t.Error("an unreplaced placeholder is left")
	}
	if strings.Contains(body, "href=\"/") || strings.Contains(body, "dashboard/product-issues") {
		t.Error("no link base: no dashboard link")
	}
}

func TestRenderProductIssueReportNullComplianceIsDash(t *testing.T) {
	d := piDash(t, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC))
	d.ByProduct = []productissuereport.ProductRow{{ProductName: piStr("API Manager"), Open: 4}}
	body := RenderProductIssueReport(d, "week", nil, "")
	if !strings.Contains(body, "&ndash;") || strings.Contains(body, "0.0%") {
		t.Error("null compliance must render as a dash, not 0%")
	}
	pct := 87.54
	d.ByProduct[0].CompliancePercentage = &pct
	if !strings.Contains(RenderProductIssueReport(d, "week", nil, ""), "87.5%") {
		t.Error("compliance not rendered to one decimal")
	}
}

func TestRenderProductIssueReportEscapesAndLinks(t *testing.T) {
	d := piDash(t, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC))
	evil := "<script>alert(1)</script>"
	d.ByProduct = []productissuereport.ProductRow{{ProductName: piStr(evil), Open: 1}, {Owner: "wso2", Repository: "x"}}
	item := productissuereport.Item{Title: evil, Owner: "wso2", Repository: "r", IssueNumber: 7, HTMLURL: "https://github.com/wso2/r/issues/7",
		ProductName: piStr("API Manager"), Priority: "P1", BoardStatus: "WAITING_ON_CS", Case: &productissuereport.CaseRef{ID: "id/1", Number: "CS001"},
		SLA: productissuereport.SLA{State: "BREACHED", Percentage: 120}}
	bad := item
	bad.HTMLURL = "javascript:alert(1)"
	d.BreachedOpen = []productissuereport.Item{item, bad}
	d.WocAgeing.Oldest = []productissuereport.Item{item}
	d.BlockedCases = productissuereport.BlockedCases{Total: 1, Cases: []productissuereport.BlockedCase{{
		CaseID: "c1", CaseNumber: "CS001", Subject: evil,
		Issues: []productissuereport.BlockedIssue{{Number: "PI1", Title: "t", BoardStatus: "TRIAGE", ProductName: piStr("IS")}}}}}

	body := RenderProductIssueReport(d, "week", []string{"API Manager"}, "https://portal.example.com/")
	if strings.Contains(body, evil) || strings.Contains(body, "javascript:") {
		t.Error("unescaped content or unsafe URL in the body")
	}
	for _, want := range []string{
		"&lt;script&gt;", "Unmapped (wso2/x)",
		"https://portal.example.com/dashboard/product-issues",
		"https://portal.example.com/cases/c1",
		"https://portal.example.com/cases/id%2F1",
		"https://github.com/wso2/r/issues/7",
		"(products: API Manager)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q", want)
		}
	}

	plain := RenderProductIssueReport(d, "week", nil, "")
	if strings.Contains(plain, "/cases/") || strings.Contains(plain, "dashboard/product-issues") {
		t.Error("with no link base the dashboard and cases are plain text")
	}
	if !strings.Contains(plain, "CS001") {
		t.Error("case number must still show")
	}
}

func TestRenderProductIssueReportLimits(t *testing.T) {
	d := piDash(t, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC))
	for i := 0; i < 20; i++ {
		d.WocAgeing.Oldest = append(d.WocAgeing.Oldest, productissuereport.Item{Title: "oldest-" + string(rune('a'+i)), Owner: "o", Repository: "r"})
		d.BlockedCases.Cases = append(d.BlockedCases.Cases, productissuereport.BlockedCase{CaseID: "c", CaseNumber: "N" + string(rune('a'+i))})
	}
	d.BlockedCases.Total = 33
	body := RenderProductIssueReport(d, "week", nil, "")
	if strings.Count(body, "oldest-") != 10 {
		t.Errorf("oldest rows = %d, want 10", strings.Count(body, "oldest-"))
	}
	if !strings.Contains(body, "Showing the 20 oldest of 33 blocked cases.") {
		t.Error("blocked cases not capped at 20 with a note")
	}
}
