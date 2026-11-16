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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/slabreachreport"
)

const (
	testSLAHeading = "Weekly SLA Breach Report"
	testSLAIntro   = "Cases handled by the CRE team."
)

func slaBreachFixture() slabreachreport.Report {
	loc := time.UTC
	return slabreachreport.Report{
		PeriodStart: time.Date(2026, 11, 2, 0, 0, 0, 0, loc),
		PeriodEnd:   time.Date(2026, 11, 9, 0, 0, 0, 0, loc),
		Rows: []slabreachreport.Row{
			{Team: "Atlas", CaseNumber: "CS0001", Subject: "Gateway down", Account: "Acme",
				Severity: "CRITICAL", Definition: "P1 - Response (Open Source)",
				BreachedOn: time.Date(2026, 11, 3, 9, 30, 0, 0, loc)},
			{Team: "Castor", CaseNumber: "CS0002", Subject: "Slow queries", Account: "Globex",
				Severity: "HIGH", Definition: "P2 - Resolution (Open Source)",
				BreachedOn: time.Date(2026, 11, 4, 11, 0, 0, 0, loc)},
			{Team: "Castor", CaseNumber: "CS0003", Subject: "Token error", Account: "Initech",
				Severity: "MEDIUM", Definition: "P3 - Response (Open Source)",
				BreachedOn: time.Date(2026, 11, 5, 14, 0, 0, 0, loc)},
			{Team: "Draco", CaseNumber: "CS0004", Subject: "Cert expiry", Account: "Umbrella",
				Severity: "LOW", Definition: "Query - Response (Open Source)",
				BreachedOn: time.Date(2026, 11, 6, 8, 0, 0, 0, loc)},
		},
	}
}

// TestRenderSLABreachReportCarriesHeadingIntroAndRows: the heading and
// intro are what distinguish the two reports' mails from each other.
func TestRenderSLABreachReportCarriesHeadingIntroAndRows(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro, "")
	for _, want := range []string{
		testSLAHeading, testSLAIntro, "4 Cases",
		"CS0001", "CS0002", "CS0003", "CS0004",
		"Team: Atlas", "Team: Castor", "Team: Draco",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered body is missing %q", want)
		}
	}
}

// TestRenderSLABreachReportHeadingIsVariantSpecific guards against the
// onboarding report rendering the CRE report's title.
func TestRenderSLABreachReportHeadingIsVariantSpecific(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(),
		"Weekly SLA Breach Report (Onboarding)", "Cases handled by the Onboarding team.", "")
	if !strings.Contains(body, "Weekly SLA Breach Report (Onboarding)") {
		t.Fatal("heading must reach the body")
	}
	if !strings.Contains(body, "Cases handled by the Onboarding team.") {
		t.Fatal("intro must reach the body")
	}
}

// TestRenderSLABreachReportGroupsContiguousTeams: Castor's two rows share
// one heading, so there must be exactly three team headings, not four.
func TestRenderSLABreachReportGroupsContiguousTeams(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro, "")
	if got := strings.Count(body, "Team: "); got != 3 {
		t.Fatalf("team headings = %d, want 3 (Castor's two rows share one)", got)
	}
	if !strings.Contains(body, `Team: Castor <span style="font-weight:400; color:#8a8f98;">(2)</span>`) {
		t.Fatal("Castor's heading must carry its own count of 2")
	}
}

// TestRenderSLABreachReportEmptyWeekSaysSo keeps a quiet week legible
// rather than rendering an empty table.
func TestRenderSLABreachReportEmptyWeekSaysSo(t *testing.T) {
	body := RenderSLABreachReport(slabreachreport.Report{
		PeriodStart: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
	}, testSLAHeading, testSLAIntro, "")
	if !strings.Contains(body, "No SLA breaches this week.") {
		t.Fatal("an empty week must say so")
	}
	if !strings.Contains(body, "0 Cases") {
		t.Fatal("an empty report still reports its count")
	}
}

// TestRenderSLABreachReportLinksByCaseNumber: the support portal
// addresses a case by number, so the link must use it -- and stay plain
// text with no base URL configured.
func TestRenderSLABreachReportLinksByCaseNumber(t *testing.T) {
	withBase := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro,
		"https://internal.support.wso2.com/support/cases")
	if !strings.Contains(withBase, `href="https://internal.support.wso2.com/support/cases/CS0002"`) {
		t.Fatal("case number must link to <base>/<number>")
	}
	without := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro, "")
	if strings.Contains(without, `href="https://internal.support.wso2.com`) {
		t.Fatal("no base URL configured must leave the cell as plain text")
	}
}

// TestRenderSLABreachReportEscapesRowData: row values are case data, not
// markup.
func TestRenderSLABreachReportEscapesRowData(t *testing.T) {
	body := RenderSLABreachReport(slabreachreport.Report{
		Rows: []slabreachreport.Row{{Team: "T", CaseNumber: "CS1", Subject: `<script>alert(1)</script>`}},
	}, testSLAHeading, testSLAIntro, "")
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("a subject containing markup must be escaped")
	}
}

// TestRenderSLABreachReportNamesTheWeek: PeriodEnd is exclusive, so the
// label must end on the day before it.
func TestRenderSLABreachReportNamesTheWeek(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro, "")
	if !strings.Contains(body, "2 Nov - 8 Nov 2026") {
		t.Fatal("week label must run to the day before the exclusive end")
	}
}
