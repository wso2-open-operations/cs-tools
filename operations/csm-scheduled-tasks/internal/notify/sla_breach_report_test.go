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
	"regexp"
	"strconv"
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
	row := func(team, wso2, number, product, ctype, prio, target string, pct float64) slabreachreport.Row {
		return slabreachreport.Row{
			Team: team, WSO2CaseID: wso2, CaseNumber: number, Product: product,
			CaseType: ctype, Priority: prio, Target: target, BusinessElapsedPercent: pct,
			Definition: "P1 - Response (Open Source)",
			BreachedOn: time.Date(2026, 11, 3, 9, 30, 0, 0, loc),
		}
	}
	return slabreachreport.Report{
		PeriodStart: time.Date(2026, 11, 2, 0, 0, 0, 0, loc),
		PeriodEnd:   time.Date(2026, 11, 9, 0, 0, 0, 0, loc),
		Rows: []slabreachreport.Row{
			row("Atlas", "WSO2-1001", "CS0001", "WSO2 API Manager", "QUESTION", "S1", "RESPONSE", 3315.17),
			row("Castor", "WSO2-1002", "CS0002", "WSO2 Identity Server", "ERROR", "S2", "RESOLUTION", 810.83),
			row("Castor", "WSO2-1003", "CS0003", "WSO2 API Manager", "QUESTION", "S3", "WORKAROUND", 153.71),
			row("Draco", "WSO2-1004", "CS0004", "WSO2 Enterprise Integrator", "QUESTION", "S4", "RESPONSE", 148.42),
		},
	}
}

// TestRenderSLABreachReportCarriesTheReportsOwnColumns pins the six
// columns the source report lists, and the values that fill them.
func TestRenderSLABreachReportCarriesTheReportsOwnColumns(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro, "")
	for _, want := range []string{
		"WSO2 Case Id", "WSO2 Product", "Case Type", "Priority", "Target", "Business Elapsed %",
		"WSO2-1001", "WSO2 API Manager", "QUESTION", "S1", "RESPONSE", "3315%",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered body is missing %q", want)
		}
	}
}

// TestRenderSLABreachReportCountsTaskSLAs: the source counts clocks, not
// cases -- one case can contribute two breaches in a week.
func TestRenderSLABreachReportCountsTaskSLAs(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro, "")
	if !strings.Contains(body, "4 total Task SLAs") {
		t.Fatal(`body must carry "4 total Task SLAs"`)
	}
}

// TestRenderSLABreachReportGroupsByIntegrationCSTeam mirrors the source's
// own grouping label and per-group counts; Castor's two rows share one
// heading.
func TestRenderSLABreachReportGroupsByIntegrationCSTeam(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro, "")
	if got := strings.Count(body, "Integration CS Team: "); got != 3 {
		t.Fatalf("team headings = %d, want 3 (Castor's two rows share one)", got)
	}
	if !strings.Contains(body, `Integration CS Team: Castor <span style="font-weight:400; color:#8a8f98;">(2)</span>`) {
		t.Fatal("Castor's heading must carry its own count of 2")
	}
}

// TestRenderSLABreachReportHeadingIsVariantSpecific guards against the
// onboarding report rendering the CRE report's title.
func TestRenderSLABreachReportHeadingIsVariantSpecific(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(),
		"Weekly SLA Breach Report (Onboarding)", "Cases handled by the Onboarding team.", "")
	if !strings.Contains(body, "Weekly SLA Breach Report (Onboarding)") ||
		!strings.Contains(body, "Cases handled by the Onboarding team.") {
		t.Fatal("heading and intro must reach the body")
	}
}

// TestRenderSLABreachReportEmptyWeekSaysSo: the onboarding report really
// does come back empty, so this is the common case, not an edge case.
func TestRenderSLABreachReportEmptyWeekSaysSo(t *testing.T) {
	body := RenderSLABreachReport(slabreachreport.Report{
		PeriodStart: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
	}, testSLAHeading, testSLAIntro, "")
	if !strings.Contains(body, "No SLA breaches this week.") {
		t.Fatal("an empty week must say so")
	}
	if !strings.Contains(body, "0 total Task SLAs") {
		t.Fatal("an empty report still reports its count")
	}
}

// TestRenderSLABreachReportShowsWSO2IdButLinksByCaseNumber: the column
// is the WSO2 reference, but the support portal addresses a case by its
// ServiceNow number.
func TestRenderSLABreachReportShowsWSO2IdButLinksByCaseNumber(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro,
		"https://internal.support.wso2.com/support/cases")
	if !strings.Contains(body, `href="https://internal.support.wso2.com/support/cases/CS0002"`) {
		t.Fatal("href must use the case number")
	}
	if !strings.Contains(body, `>WSO2-1002</a>`) {
		t.Fatal("the link text must be the WSO2 case id, the source report's own column")
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
		Rows: []slabreachreport.Row{{Team: "T", WSO2CaseID: "WSO2-1", Product: `<script>alert(1)</script>`}},
	}, testSLAHeading, testSLAIntro, "")
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("a product containing markup must be escaped")
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

// TestRenderSLABreachReportTeamHeadingSurvivesDarkMode: the heading cell
// carries an inline light background, and the template's dark-mode block
// forces light text onto every td. Without a dark-mode override for that
// cell the team name renders light-on-light. The class and the rule have
// to stay together, so both are asserted here.
func TestRenderSLABreachReportTeamHeadingSurvivesDarkMode(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro, "")
	if !strings.Contains(body, `<td class="teamHeading"`) {
		t.Fatal("the team heading cell must carry the teamHeading class")
	}
	if !strings.Contains(body, ".reportTable .teamHeading { background-color: #33383d !important; }") {
		t.Fatal("the dark-mode block must override the heading cell's inline background")
	}
}

// cellStyleFor returns the opening <td ...> tag of the cell that encloses
// the first occurrence of needle, so a test can assert on that one cell's
// style rather than on the whole document.
func cellStyleFor(t *testing.T, body, needle string) string {
	t.Helper()
	at := strings.Index(body, needle)
	if at < 0 {
		t.Fatalf("rendered body is missing %q", needle)
	}
	open := strings.LastIndex(body[:at], "<td")
	if open < 0 {
		t.Fatalf("%q is not inside a table cell", needle)
	}
	end := strings.Index(body[open:], ">")
	if end < 0 {
		t.Fatalf("unterminated <td> before %q", needle)
	}
	return body[open : open+end]
}

// TestRenderSLABreachReportLongCaseIdCannotOverflowItsColumn pins the fix
// for a real rendering bug: the table is table-layout:fixed, so the case
// id column cannot widen, and a nowrap id wider than it does not clip or
// wrap -- it paints across WSO2 Product. Staging's ids are long enough to
// show it where production's "WSO2-1001" never would.
func TestRenderSLABreachReportLongCaseIdCannotOverflowItsColumn(t *testing.T) {
	const longID = "AUTOMATIONTESTCUSMSSUB-100"
	r := slaBreachFixture()
	r.Rows[0].WSO2CaseID = longID
	body := RenderSLABreachReport(r, testSLAHeading, testSLAIntro, "")

	style := cellStyleFor(t, body, longID)
	if strings.Contains(style, "white-space:nowrap") {
		t.Fatalf("the case id cell must not be nowrap, or a long id overflows into the next column; got %s", style)
	}
	if !strings.Contains(style, "word-break:break-all") || !strings.Contains(style, "overflow-wrap:anywhere") {
		t.Fatalf("the case id cell must break a long id in both engine families; got %s", style)
	}
}

// TestRenderSLABreachReportLinkedLongCaseIdAlsoBreaks: the linked form
// wraps the id in an <a>, so the cell style has to be asserted there too
// -- a fix applied to only one of the two branches is the likely slip.
func TestRenderSLABreachReportLinkedLongCaseIdAlsoBreaks(t *testing.T) {
	const longID = "AUTOMATIONTESTCUSMSSUB-100"
	r := slaBreachFixture()
	r.Rows[0].WSO2CaseID = longID
	body := RenderSLABreachReport(r, testSLAHeading, testSLAIntro, "https://example.invalid/cases")

	style := cellStyleFor(t, body, longID)
	if strings.Contains(style, "white-space:nowrap") {
		t.Fatalf("the linked case id cell must not be nowrap either; got %s", style)
	}
}

// TestRenderSLABreachReportPercentStaysOnOneLine guards the other half of
// the change: the elapsed percentage is always short, so it keeps nowrap.
// Dropping nowrap everywhere would "fix" the overflow by costing every
// numeric column a wrap.
func TestRenderSLABreachReportPercentStaysOnOneLine(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro, "")
	style := cellStyleFor(t, body, "3315%")
	if !strings.Contains(style, "white-space:nowrap") {
		t.Fatalf("the elapsed percentage cell should stay on one line; got %s", style)
	}
}

// TestSLABreachReportColumnWidthsSumTo100 guards the width rebalance:
// under table-layout:fixed the declared widths are the layout, so a typo
// that makes them sum to anything else silently reflows every column.
func TestSLABreachReportColumnWidthsSumTo100(t *testing.T) {
	body := RenderSLABreachReport(slaBreachFixture(), testSLAHeading, testSLAIntro, "")
	widths := regexp.MustCompile(`<th[^>]*width="(\d+)%"`).FindAllStringSubmatch(body, -1)
	if len(widths) != slaBreachRowColumns {
		t.Fatalf("expected %d column widths, found %d", slaBreachRowColumns, len(widths))
	}
	total := 0
	for _, m := range widths {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("bad width %q", m[1])
		}
		total += n
	}
	if total != 100 {
		t.Fatalf("column widths sum to %d%%, want 100%%", total)
	}
}
