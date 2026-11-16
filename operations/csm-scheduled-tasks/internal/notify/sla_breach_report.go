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
	_ "embed"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/slabreachreport"
)

//go:embed templates/sla_breach_report.html
var slaBreachReportTemplateRaw string

var slaBreachReportTemplate = bakeLogo(slaBreachReportTemplateRaw)

// slaBreachReportTimezone is the zone the report's week is named in: the
// one entity-service takes the week in.
const slaBreachReportTimezone = "Asia/Colombo"

// slaBreachRowColumns is how many columns each table has; a team's
// heading row spans them all.
const slaBreachRowColumns = 6

// RenderSLABreachReport fills in one weekly SLA breach report's HTML
// email template: the introduction the source report has always carried,
// then its single table, grouped by the account's integration CS team.
//
// heading names the report ("Weekly SLA Breach Report (Onboarding)") and
// intro is its one-line description of whose cases these are -- the two
// ServiceNow reports are separate definitions with separate filters, so
// each renders its own mail. caseLinkBase is a case page prefix; when
// set, each row's case number links to <caseLinkBase>/<case number>,
// else it is plain text. [YEAR] is today's year, the footer-only detail
// every template here has.
func RenderSLABreachReport(r slabreachreport.Report, heading, intro, caseLinkBase string) string {
	loc, err := time.LoadLocation(slaBreachReportTimezone)
	if err != nil {
		loc = time.UTC
	}
	replacer := strings.NewReplacer(
		"<!-- [HEADING] -->", escapeHTML(heading),
		"<!-- [INTRO] -->", escapeHTML(intro),
		"<!-- [WEEK] -->", escapeHTML(slaBreachWeekLabel(r, loc)),
		"<!-- [TOTAL] -->", escapeHTML(slaBreachCaseCount(len(r.Rows))),
		"<!-- [ROWS] -->", renderSLABreachRows(r.Rows, caseLinkBase, loc),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(slaBreachReportTemplate)
}

// slaBreachWeekLabel names the week as "2 Nov - 8 Nov 2026". PeriodEnd is
// exclusive, so the last day shown is the day before it; a zero period
// (an empty report built without one) yields "".
func slaBreachWeekLabel(r slabreachreport.Report, loc *time.Location) string {
	if r.PeriodStart.IsZero() || r.PeriodEnd.IsZero() {
		return ""
	}
	start := r.PeriodStart.In(loc)
	last := r.PeriodEnd.In(loc).AddDate(0, 0, -1)
	return start.Format("2 Jan") + " - " + last.Format("2 Jan 2006")
}

// slaBreachCaseCount is "1 Case" or "N Cases", as the source report
// counted.
func slaBreachCaseCount(n int) string {
	if n == 1 {
		return "1 Case"
	}
	return strconv.Itoa(n) + " Cases"
}

// renderSLABreachRows builds one table's body: for each integration CS
// team, a heading row naming it with its count, then one row per breach
// in the order given. Rows arrive already ordered by team, so a team's
// rows are contiguous and a plain run-length pass is enough -- no
// regrouping, and the server's ordering stays the single source of truth.
func renderSLABreachRows(rows []slabreachreport.Row, caseLinkBase string, loc *time.Location) string {
	if len(rows) == 0 {
		return fmt.Sprintf(`<tr><td colspan="%d" style="padding:16px 10px; text-align:center; color:#8a8f98;">No SLA breaches this week.</td></tr>`, slaBreachRowColumns)
	}
	const cell = `<td style="padding:5px 8px; line-height:16px; border-bottom:1px solid #e8eaed; vertical-align:top; word-wrap:break-word;">%s</td>`
	// The case number and the breach date are single tokens; breaking
	// either across two lines doubles the row height and helps nobody.
	const nowrapCell = `<td style="padding:5px 8px; line-height:16px; border-bottom:1px solid #e8eaed; vertical-align:top; white-space:nowrap;">%s</td>`

	var b strings.Builder
	team := ""
	for i, row := range rows {
		if i == 0 || row.Team != team {
			team = row.Team
			fmt.Fprintf(&b, `<tr><td colspan="%d" style="padding:10px; background-color:#f8f9fa; border-bottom:1px solid #e8eaed; font-weight:700;">Team: %s <span style="font-weight:400; color:#8a8f98;">(%d)</span></td></tr>`,
				slaBreachRowColumns, orDash(escapeHTML(team)), slaBreachTeamCount(rows, i))
		}
		b.WriteString("<tr>")
		fmt.Fprintf(&b, nowrapCell, slaBreachCaseLink(caseLinkBase, row))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(row.Subject)))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(row.Account)))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(row.Severity)))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(row.Definition)))
		fmt.Fprintf(&b, nowrapCell, orDash(escapeHTML(slaBreachTime(row.BreachedOn, loc))))
		b.WriteString("</tr>")
	}
	return b.String()
}

// slaBreachTeamCount counts the run of rows sharing rows[start]'s team.
func slaBreachTeamCount(rows []slabreachreport.Row, start int) int {
	n := 0
	for i := start; i < len(rows) && rows[i].Team == rows[start].Team; i++ {
		n++
	}
	return n
}

// slaBreachTime renders a breach instant in the report's own zone. A zero
// time yields "" so the cell falls back to a dash rather than printing
// year one.
func slaBreachTime(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	return t.In(loc).Format("2 Jan 15:04")
}

// slaBreachCaseLink is the row's case number, linked to the case page
// (<caseLinkBase>/<case number>) when a base URL is configured. The
// support portal addresses a case by its number, not its id -- the same
// link shape the migration case scan uses. A row without a number falls
// back to the portal's own reference so the cell is never blank, and is
// left as plain text because the link needs the number.
func slaBreachCaseLink(caseLinkBase string, row slabreachreport.Row) string {
	if row.CaseNumber == "" {
		return orDash(escapeHTML(row.WSO2CaseID))
	}
	if caseLinkBase == "" {
		return escapeHTML(row.CaseNumber)
	}
	href := strings.TrimRight(caseLinkBase, "/") + "/" + url.PathEscape(row.CaseNumber)
	return fmt.Sprintf(`<a href="%s" target="_blank" style="color:#ff7300;">%s</a>`, escapeHTML(href), escapeHTML(row.CaseNumber))
}
