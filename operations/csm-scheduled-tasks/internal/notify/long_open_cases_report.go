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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/longopenreport"
)

//go:embed templates/stale_cases_report.html
var staleCasesReportTemplateRaw string

var staleCasesReportTemplate = bakeLogo(staleCasesReportTemplateRaw)

// longOpenReportTimezone is the zone the report's dates are written in: the
// one the team reading it works in, matching the migration case scan.
const longOpenReportTimezone = "Asia/Colombo"

// longOpenReportColumns is how many columns each table has; the empty-table
// row spans them all.
const longOpenReportColumns = 8

// RenderLongOpenCasesReport fills in the stale cases report's HTML email
// template: an introduction naming the age threshold, then the report's two
// tables, "Ongoing cases" first and "Ongoing cases on onboarding projects"
// second. Both case columns link to the case page in the CSM portal when
// caseLinkBase is set, and are plain text otherwise. A table
// entity-service cut at its cap carries a "Showing N of TOTAL" note, and an
// empty table says "No cases" in place of an empty grid. [YEAR] is today's
// year, the footer-only detail every template here has.
func RenderLongOpenCasesReport(r longopenreport.Report, olderThanDays int, caseLinkBase string) string {
	loc, err := time.LoadLocation(longOpenReportTimezone)
	if err != nil {
		loc = time.UTC
	}
	replacer := strings.NewReplacer(
		"<!-- [OLDER_THAN_DAYS] -->", strconv.Itoa(olderThanDays),
		"<!-- [CASES_ROWS] -->", renderLongOpenRows(r.Cases, caseLinkBase, loc),
		"<!-- [CASES_NOTE] -->", longOpenTruncationNote(len(r.Cases), r.CasesTotal, r.CasesTruncated),
		"<!-- [ONBOARDING_CASES_ROWS] -->", renderLongOpenRows(r.OnboardingCases, caseLinkBase, loc),
		"<!-- [ONBOARDING_CASES_NOTE] -->", longOpenTruncationNote(len(r.OnboardingCases), r.OnboardingCasesTotal, r.OnboardingCasesTruncated),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(staleCasesReportTemplate)
}

// longOpenTruncationNote is the "Showing N of TOTAL" line under a table that
// entity-service cut at its cap, or "" when the table is the whole result.
func longOpenTruncationNote(shown, total int, truncated bool) string {
	if !truncated && total <= shown {
		return ""
	}
	return fmt.Sprintf(`<p style="margin:8px 0 0; font-size:13px; color:#b3261e;"><strong>Showing only the oldest %d of %d.</strong> The rest are not listed.</p>`, shown, total)
}

// renderLongOpenRows builds one <tr> per case, in the order given (oldest
// first, as entity-service returns them). An empty table says so rather than
// rendering a header over nothing.
func renderLongOpenRows(rows []longopenreport.Row, caseLinkBase string, loc *time.Location) string {
	if len(rows) == 0 {
		return fmt.Sprintf(`<tr><td colspan="%d" style="padding:12px 8px; text-align:center; color:#8a8f98;">No cases.</td></tr>`, longOpenReportColumns)
	}
	const cell = `<td style="padding:4px 8px; line-height:16px; border-bottom:1px solid #e8eaed; vertical-align:top;">%s</td>`
	const nowrapCell = `<td style="padding:4px 8px; line-height:16px; border-bottom:1px solid #e8eaed; vertical-align:top; white-space:nowrap;">%s</td>`
	const stamp = "2006-01-02 15:04"
	var b strings.Builder
	for _, row := range rows {
		b.WriteString("<tr>")
		fmt.Fprintf(&b, nowrapCell, longOpenCaseLink(caseLinkBase, row, row.Number))
		fmt.Fprintf(&b, nowrapCell, longOpenCaseLink(caseLinkBase, row, row.InternalID))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(row.CsTeam)))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(row.AssignedEngineer)))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(humanizeState(row.State))))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(severityLabel(row.Severity))))
		fmt.Fprintf(&b, nowrapCell, escapeHTML(row.UpdatedOn.In(loc).Format(stamp)))
		fmt.Fprintf(&b, nowrapCell, escapeHTML(row.CreatedOn.In(loc).Format(stamp)))
		b.WriteString("</tr>")
	}
	return b.String()
}

// longOpenCaseLink is a case cell linked to the case page in the CSM portal
// (<caseLinkBase>/cases/<case id>) when a base URL is configured; plain text
// otherwise. The case page opens by the case's UUID only, so neither the
// number nor the internal ID can be used in the path. The id is path-escaped
// because it is data.
func longOpenCaseLink(caseLinkBase string, row longopenreport.Row, text string) string {
	if text == "" {
		return "-"
	}
	if caseLinkBase == "" || row.ID == "" {
		return escapeHTML(text)
	}
	href := strings.TrimRight(caseLinkBase, "/") + "/cases/" + url.PathEscape(row.ID)
	return fmt.Sprintf(`<a href="%s" target="_blank" style="color:#ff7300;">%s</a>`, escapeHTML(href), escapeHTML(text))
}

// severityLabel is the S0-S4 label the support team reads, for the API's
// severity words; an unknown value is shown as it came.
func severityLabel(severity string) string {
	switch strings.ToLower(severity) {
	case "catastrophic":
		return "S0"
	case "critical":
		return "S1"
	case "high":
		return "S2"
	case "medium":
		return "S3"
	case "low":
		return "S4"
	}
	return severity
}
