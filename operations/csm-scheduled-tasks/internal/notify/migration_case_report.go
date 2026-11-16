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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/migrationreport"
)

//go:embed templates/migration_case_report.html
var migrationCaseReportTemplateRaw string

var migrationCaseReportTemplate = bakeLogo(migrationCaseReportTemplateRaw)

// migrationCaseReportTimezone is the zone the scan's dates are written in:
// the one the team reading it works in, and the one entity-service takes
// the age cut-off in.
const migrationCaseReportTimezone = "Asia/Colombo"

// RenderMigrationCaseReport fills in the weekly migration case scan's HTML
// email template, the same branded shape as the stale and open case
// reports: an introduction naming the age threshold, then the three
// tables with the columns the scan has always had. caseLinkBase is the
// support portal's case page prefix; when set, each row's WSO2 ID links to
// <caseLinkBase>/<case id>, where the id is the case's UUID (the only
// identifier the case page opens by), else it is plain text. [YEAR] is today's year, the footer-only detail every
// template here has.
func RenderMigrationCaseReport(r migrationreport.Report, caseLinkBase string) string {
	loc, err := time.LoadLocation(migrationCaseReportTimezone)
	if err != nil {
		loc = time.UTC
	}
	replacer := strings.NewReplacer(
		"<!-- [OLDER_THAN_DAYS] -->", strconv.Itoa(r.OlderThanDays),
		"<!-- [CREATED_BEFORE] -->", escapeHTML(r.CreatedBefore.In(loc).Format("2006-01-02")),
		"<!-- [ON_HOLD_COUNT] -->", strconv.Itoa(len(r.OnHold)),
		"<!-- [ON_HOLD_ROWS] -->", renderMigrationRows(r.OnHold, caseLinkBase, loc),
		"<!-- [WAITING_ON_ACCOUNTS_COUNT] -->", strconv.Itoa(len(r.WaitingOnAccounts)),
		"<!-- [WAITING_ON_ACCOUNTS_ROWS] -->", renderMigrationRows(r.WaitingOnAccounts, caseLinkBase, loc),
		"<!-- [IN_PROGRESS_COUNT] -->", strconv.Itoa(len(r.InProgress)),
		"<!-- [IN_PROGRESS_ROWS] -->", renderMigrationRows(r.InProgress, caseLinkBase, loc),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(migrationCaseReportTemplate)
}

// migrationRowColumns is how many columns the scan's tables have; the
// empty-table row spans them all.
const migrationRowColumns = 9

// renderMigrationRows builds one <tr> per migration, in the order given
// (oldest first, as entity-service returns them). An empty table says so
// rather than rendering a header over nothing.
func renderMigrationRows(rows []migrationreport.Row, caseLinkBase string, loc *time.Location) string {
	if len(rows) == 0 {
		return fmt.Sprintf(`<tr><td colspan="%d" style="padding:12px 8px; text-align:center; color:#8a8f98;">No migrations in this table.</td></tr>`, migrationRowColumns)
	}
	// Tight padding and an explicit line-height: nine columns over a
	// couple of hundred rows, so every extra pixel per row is a screen of
	// scrolling. The two timestamp columns are held on one line each --
	// they wrapped before, which turned most rows into two.
	const cell = `<td style="padding:4px 8px; line-height:16px; border-bottom:1px solid #e8eaed; vertical-align:top;">%s</td>`
	const nowrapCell = `<td style="padding:4px 8px; line-height:16px; border-bottom:1px solid #e8eaed; vertical-align:top; white-space:nowrap;">%s</td>`
	const stamp = "2006-01-02 15:04:05"
	var b strings.Builder
	for _, row := range rows {
		lastComment := ""
		if row.LastCustomerCommentOn != nil {
			lastComment = row.LastCustomerCommentOn.In(loc).Format(stamp)
		}
		owners := strings.Join(nonEmpty(row.AccountManager, row.TechnicalOwner), " / ")
		b.WriteString("<tr>")
		fmt.Fprintf(&b, cell, escapeHTML(row.Number))
		fmt.Fprintf(&b, nowrapCell, migrationCaseLink(caseLinkBase, row))
		fmt.Fprintf(&b, nowrapCell, escapeHTML(row.CreatedOn.In(loc).Format(stamp)))
		fmt.Fprintf(&b, nowrapCell, orDash(escapeHTML(lastComment)))
		fmt.Fprintf(&b, nowrapCell, orDash(escapeHTML(row.AssignedTo)))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(row.Product)))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(row.CsTeam)))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(row.SalesRegion)))
		fmt.Fprintf(&b, cell, orDash(escapeHTML(owners)))
		b.WriteString("</tr>")
	}
	return b.String()
}

// migrationCaseLink is the row's WSO2 ID, linked to the case page in the
// support portal (<caseLinkBase>/<case id>) when a base URL is configured.
// The case page opens by the case's UUID, not its number. The key falls
// back to the number so the cell is never blank; the id is path-escaped
// because it is data.
func migrationCaseLink(caseLinkBase string, row migrationreport.Row) string {
	text := row.IssueKey
	if text == "" {
		text = row.Number
	}
	if caseLinkBase == "" || row.ID == "" {
		return escapeHTML(text)
	}
	href := strings.TrimRight(caseLinkBase, "/") + "/" + url.PathEscape(row.ID)
	return fmt.Sprintf(`<a href="%s" target="_blank" style="color:#ff7300;">%s</a>`, escapeHTML(href), escapeHTML(text))
}

// nonEmpty drops the blank strings so a missing owner leaves no stray
// separator.
func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
