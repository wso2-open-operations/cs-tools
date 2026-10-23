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
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhours"
)

//go:embed templates/query_hours.html
var queryHoursTemplateRaw string

var queryHoursTemplate = bakeLogo(queryHoursTemplateRaw)

const (
	qhCell = `style="padding:7px 10px; border:1px solid #dfe3e6;"`
	qhNum  = `align="right" style="padding:7px 10px; border:1px solid #dfe3e6;"`
	qhHead = `style="background-color:#f1f3f4; padding:8px 10px; border:1px solid #dfe3e6;"`
)

// QueryHoursWeeklySubject is the subject ServiceNow uses.
const QueryHoursWeeklySubject = "Weekly Query Support Consumption Report"

// RenderQueryHoursWeekly fills in the weekly consumption report: the
// accounts that have exceeded their allowance, then those close to it,
// each broken down by opportunity and project.
func RenderQueryHoursWeekly(r queryhours.Report) string {
	var b strings.Builder
	b.WriteString(`<table border="0" cellpadding="0" cellspacing="0" class="reportTable" style="margin:0 0 26px; border-collapse:collapse; font-size:14px;"><tbody>`)
	b.WriteString(`<tr><td ` + qhCell + `>Accounts exceeded</td><td ` + qhNum + `><strong>` +
		strconv.Itoa(len(r.Exceeded)) + `</strong></td></tr>`)
	b.WriteString(`<tr><td ` + qhCell + `>Accounts going to exceed</td><td ` + qhNum + `><strong>` +
		strconv.Itoa(len(r.CloseToExceed)) + `</strong></td></tr>`)
	b.WriteString(`</tbody></table>`)

	b.WriteString(renderQueryHourTable("Hours Exceeded Accounts", "#26814f", r.Exceeded))
	b.WriteString(renderQueryHourTable("Accounts Going to Exceed", "#187ba8", r.CloseToExceed))

	return strings.NewReplacer(
		"<!-- [HEADER] -->", "Weekly Report",
		"<!-- [TITLE] -->", "Weekly Query Support Consumption Report",
		"<!-- [SUBTITLE] -->", escapeHTML(r.GeneratedOn),
		"<!-- [BODY] -->", b.String(),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	).Replace(queryHoursTemplate)
}

// renderQueryHourTable renders one of the report's two tables, with the
// same seven columns ServiceNow's has.
func renderQueryHourTable(caption, colour string, accounts []queryhours.Account) string {
	var b strings.Builder
	b.WriteString(`<h3 style="margin:24px 0 8px; font-size:16px; color:` + colour + `;">` +
		escapeHTML(caption) + `</h3>`)
	if len(accounts) == 0 {
		b.WriteString(`<p style="margin:0 0 16px; font-size:13px; color:#8a8f98;">None.</p>`)
		return b.String()
	}
	b.WriteString(`<table border="0" cellpadding="0" cellspacing="0" width="100%" class="reportTable" style="margin:0 0 20px; border-collapse:collapse; font-size:12px;"><thead><tr>`)
	for _, h := range []string{"Account", "Opportunity", "Total Query Hour", "Project", "Consumed", "Total Consumed", "Remains"} {
		align := `align="left"`
		if h != "Account" && h != "Opportunity" && h != "Project" {
			align = `align="right"`
		}
		b.WriteString(`<th ` + align + ` ` + qhHead + `>` + escapeHTML(h) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)

	for _, a := range accounts {
		for _, o := range a.Opportunities {
			// A row per project, so the account and opportunity figures
			// repeat down the group rather than relying on rowspans
			// that mail clients render inconsistently.
			if len(o.Projects) == 0 {
				b.WriteString(`<tr>`)
				b.WriteString(`<td ` + qhCell + `>` + escapeHTML(a.AccountName) + `</td>`)
				b.WriteString(`<td ` + qhCell + `>` + escapeHTML(o.Name) + `</td>`)
				b.WriteString(`<td ` + qhNum + `>` + qhMinutes(o.TotalMinutes) + `</td>`)
				b.WriteString(`<td ` + qhCell + `>&mdash;</td><td ` + qhNum + `>&mdash;</td>`)
				b.WriteString(`<td ` + qhNum + `>` + qhMinutes(o.ConsumedMinutes) + `</td>`)
				b.WriteString(`<td align="right" style="padding:7px 10px; border:1px solid #dfe3e6; color:` +
					qhRemainColour(o) + `;"><strong>` + qhMinutes(o.RemainingMinutes) + `</strong></td>`)
				b.WriteString(`</tr>`)
				continue
			}
			for _, p := range o.Projects {
				b.WriteString(`<tr>`)
				b.WriteString(`<td ` + qhCell + `>` + escapeHTML(a.AccountName) + `</td>`)
				b.WriteString(`<td ` + qhCell + `>` + escapeHTML(o.Name) + `</td>`)
				b.WriteString(`<td ` + qhNum + `>` + qhMinutes(o.TotalMinutes) + `</td>`)
				b.WriteString(`<td ` + qhCell + `>` + escapeHTML(p.ProjectName) +
					`<br/><span style="color:#8a8f98;">` + escapeHTML(p.ProjectKey) + `</span></td>`)
				b.WriteString(`<td ` + qhNum + `>` + qhMinutes(p.ConsumedMinutes) + `</td>`)
				b.WriteString(`<td ` + qhNum + `>` + qhMinutes(o.ConsumedMinutes) + `</td>`)
				b.WriteString(`<td align="right" style="padding:7px 10px; border:1px solid #dfe3e6; color:` +
					qhRemainColour(o) + `;"><strong>` + qhMinutes(o.RemainingMinutes) + `</strong></td>`)
				b.WriteString(`</tr>`)
			}
		}
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func qhRemainColour(o queryhours.Opportunity) string {
	switch {
	case o.IsExceeded:
		return "#DC143C"
	case o.IsCloseToExceed:
		return "#FF6347"
	default:
		return "#465868"
	}
}

// qhMinutes renders minutes as ServiceNow's mails do, "Xh Ym".
func qhMinutes(mins int) string {
	sign := ""
	if mins < 0 {
		sign, mins = "-", -mins
	}
	return fmt.Sprintf("%s%dh %dm", sign, mins/60, mins%60)
}

// QueryHourNotificationSubject is the subject for one project crossing a
// threshold, verbatim from the ServiceNow action.
func QueryHourNotificationSubject(t queryhours.Transition) string {
	switch t.To {
	case queryhours.StateClosure100:
		return "Query Hour Exceeded in " + t.AccountName
	case queryhours.StateNotified90:
		return "90% of Query Hours Utilized"
	default:
		return "75% of Query Hours Utilized"
	}
}

// queryHourNotificationMessage is the body copy for each threshold, also
// verbatim -- the three differ in more than their percentage, so they
// are kept apart rather than templated from one sentence.
func queryHourNotificationMessage(t queryhours.Transition) string {
	switch t.To {
	case queryhours.StateClosure100:
		return "Kindly note that, <b>Allocated query support hours are exceeded in " +
			escapeHTML(t.AccountName) + "</b>. Therefore, It is advised to start the closure " +
			"management process or notify customers to repurchase additional subscription hours."
	case queryhours.StateNotified90:
		return "Kindly note that, Allocated 90% of query support hours are utilized in " +
			escapeHTML(t.AccountName) + ". Query support will be disabled on 100% usage. " +
			"Therefore, It is advised to start the closure management process or notify " +
			"customers to repurchase additional subscription hours."
	default:
		return "Kindly note that, Allocated 75% of query support hours are utilized in " +
			escapeHTML(t.AccountName) + ". Query support will be disabled on 100% usage. " +
			"Therefore, It is advised to notify customers to repurchase additional subscription hours."
	}
}

// RenderQueryHourNotification fills in the per-project usage notice.
func RenderQueryHourNotification(t queryhours.Transition) string {
	name := t.AccountManagerName
	if strings.TrimSpace(name) == "" {
		name = "Account Manager"
	}

	var b strings.Builder
	b.WriteString(`<p style="margin:0 0 16px;">Hi ` + escapeHTML(name) + `,</p>`)
	b.WriteString(`<p style="margin:0 0 20px;">` + queryHourNotificationMessage(t) + `</p>`)
	b.WriteString(`<table border="0" cellpadding="0" cellspacing="0" width="100%" class="reportTable" style="margin:0 0 20px; border-collapse:collapse; font-size:13px;"><thead><tr>`)
	for _, h := range []string{"Account", "Total Query Hour", "Project", "Consumed", "Remains"} {
		b.WriteString(`<th ` + qhHead + `>` + escapeHTML(h) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody><tr>`)
	b.WriteString(`<td ` + qhCell + `>` + escapeHTML(t.AccountName) + `</td>`)
	b.WriteString(`<td ` + qhNum + `>` + escapeHTML(t.TotalText) + `</td>`)
	b.WriteString(`<td ` + qhCell + `>` + escapeHTML(t.ProjectName) + `</td>`)
	b.WriteString(`<td ` + qhNum + `>` + escapeHTML(t.ConsumedText) + `</td>`)
	b.WriteString(`<td ` + qhNum + `>` + escapeHTML(t.RemainingText) + `</td>`)
	b.WriteString(`</tr></tbody></table>`)

	return strings.NewReplacer(
		"<!-- [HEADER] -->", "Usage Notification",
		"<!-- [TITLE] -->", escapeHTML(QueryHourNotificationSubject(t)),
		"<!-- [SUBTITLE] -->", escapeHTML(t.ProjectKey),
		"<!-- [BODY] -->", b.String(),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	).Replace(queryHoursTemplate)
}
