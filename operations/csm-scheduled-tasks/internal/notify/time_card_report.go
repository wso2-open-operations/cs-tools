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
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/timecards"
)

//go:embed templates/time_card_report.html
var timeCardReportTemplate string

const (
	tcHead = `align="left" style="padding:8px 10px; background-color:#f1f3f4; border:1px solid #dfe3e6; font-size:12px; text-transform:uppercase; letter-spacing:0.5px;"`
	tcCell = `align="left" style="padding:7px 10px; border:1px solid #dfe3e6; font-size:13px;"`
	tcNum  = `align="right" style="padding:7px 10px; border:1px solid #dfe3e6; font-size:13px; white-space:nowrap;"`
	tcSum  = `align="right" style="padding:7px 10px; border:1px solid #dfe3e6; background-color:#f8f9fa; font-size:13px; white-space:nowrap;"`
)

// FormatMinutes writes a whole number of minutes as "<h>h <m>m", by integer
// division and remainder so the minutes part is always 0 to 59 ("1h 0m", never
// "60m"). A negative value, which entity-service does not send, keeps its sign.
func FormatMinutes(minutes int) string {
	sign := ""
	if minutes < 0 {
		sign = "-"
		minutes = -minutes
	}
	return fmt.Sprintf("%s%dh %dm", sign, minutes/60, minutes%60)
}

// humanizeCaseType turns a lowercase work item type ("service_request") into
// "Service request"; an empty one is a dash.
func humanizeCaseType(t string) string {
	t = strings.TrimSpace(strings.ReplaceAll(t, "_", " "))
	if t == "" {
		return "-"
	}
	return strings.ToUpper(t[:1]) + t[1:]
}

func orDashText(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// TimeCardReportSubject is "Time Card Report - <project name> - <Month YYYY>",
// naming the month of the report's start date in English. The project key
// stands in for a project with no name. A date that does not parse leaves the
// month out rather than failing the send.
func TimeCardReportSubject(r timecards.Report) string {
	name := strings.TrimSpace(r.Project.Name)
	if name == "" {
		name = r.Project.Key
	}
	subject := "Time Card Report - " + name
	if from, err := time.Parse("2006-01-02", r.From); err == nil {
		subject += fmt.Sprintf(" - %s %d", from.Month(), from.Year())
	}
	// A header value must stay on one line.
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)
}

// RenderTimeCardReport fills in the monthly time card report: the project and
// period, four summary cards, and one table row per case with a TOTAL row. A
// report with no cases shows a "no data" card in their place and is still a
// complete mail. Every figure is formatted once from whole minutes
// (FormatMinutes); every dynamic value is escaped.
func RenderTimeCardReport(r timecards.Report) string {
	var b strings.Builder
	if len(r.Cases) == 0 {
		b.WriteString(`<table border="0" cellpadding="0" cellspacing="0" width="100%"><tr><td align="center" style="padding:28px 20px; background-color:#f8f9fa; border:1px solid #dfe3e6; color:#465868; font-size:14px;">No time card data found for the selected period.</td></tr></table>`)
	} else {
		b.WriteString(`<table border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:0 0 24px; border-collapse:separate;"><tr>`)
		cards := []struct {
			label   string
			minutes int
		}{
			{"Total effort", r.Totals.TotalMinutes},
			{"Billable", r.Totals.BillableMinutes},
			{"Non-Billable", r.Totals.NonBillableMinutes},
			{"Patching", r.Totals.PatchingMinutes},
		}
		for _, c := range cards {
			b.WriteString(`<td width="25%" align="center" style="padding:14px 8px; background-color:#f8f9fa; border:1px solid #dfe3e6;"><div style="font-size:12px; color:#8a8f98; text-transform:uppercase; letter-spacing:0.5px;">` +
				escapeHTML(c.label) + `</div><div style="margin-top:6px; font-size:20px; font-weight:700; color:#465868;">` +
				escapeHTML(FormatMinutes(c.minutes)) + `</div></td>`)
		}
		b.WriteString(`</tr></table>`)

		b.WriteString(`<table border="0" cellpadding="0" cellspacing="0" width="100%" style="border-collapse:collapse;"><thead><tr>`)
		for _, h := range []string{"External Case ID", "Case Number", "Case Type", "Patching", "Billable", "Non-Billable", "Total"} {
			b.WriteString("<th " + tcHead + ">" + escapeHTML(h) + "</th>")
		}
		b.WriteString("</tr></thead><tbody>")
		for _, c := range r.Cases {
			b.WriteString("<tr><td " + tcCell + ">" + escapeHTML(orDashText(c.ExternalCaseID)) + "</td>" +
				"<td " + tcCell + ">" + escapeHTML(c.CaseNumber) + "</td>" +
				"<td " + tcCell + ">" + escapeHTML(humanizeCaseType(c.CaseType)) + "</td>" +
				"<td " + tcNum + ">" + escapeHTML(FormatMinutes(c.PatchingMinutes)) + "</td>" +
				"<td " + tcNum + ">" + escapeHTML(FormatMinutes(c.BillableMinutes)) + "</td>" +
				"<td " + tcNum + ">" + escapeHTML(FormatMinutes(c.NonBillableMinutes)) + "</td>" +
				"<td " + tcNum + ">" + escapeHTML(FormatMinutes(c.TotalMinutes)) + "</td></tr>")
		}
		b.WriteString("<tr><td colspan=\"3\" " + tcCell + "><strong>TOTAL</strong></td>" +
			"<td " + tcSum + "><strong>" + escapeHTML(FormatMinutes(r.Totals.PatchingMinutes)) + "</strong></td>" +
			"<td " + tcSum + "><strong>" + escapeHTML(FormatMinutes(r.Totals.BillableMinutes)) + "</strong></td>" +
			"<td " + tcSum + "><strong>" + escapeHTML(FormatMinutes(r.Totals.NonBillableMinutes)) + "</strong></td>" +
			"<td " + tcSum + "><strong>" + escapeHTML(FormatMinutes(r.Totals.TotalMinutes)) + "</strong></td></tr>")
		b.WriteString("</tbody></table>")
	}

	name := strings.TrimSpace(r.Project.Name)
	if name == "" {
		name = "-"
	}
	return strings.NewReplacer(
		"<!-- [TITLE] -->", escapeHTML(TimeCardReportSubject(r)),
		"<!-- [PROJECT_KEY] -->", escapeHTML(r.Project.Key),
		"<!-- [PROJECT_NAME] -->", escapeHTML(name),
		"<!-- [PERIOD_FROM] -->", escapeHTML(r.From),
		"<!-- [PERIOD_TO] -->", escapeHTML(r.To),
		"<!-- [BODY] -->", b.String(),
	).Replace(timeCardReportTemplate)
}
