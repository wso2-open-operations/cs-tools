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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
)

//go:embed templates/worklogs_report.html
var worklogsReportTemplateRaw string

var worklogsReportTemplate = bakeLogo(worklogsReportTemplateRaw)

// The layout follows the ServiceNow original -- a stat grid, then a card
// per measure holding one pill per team, then a section per team -- in
// WSO2's palette rather than ServiceNow's navy. Every figure it shows was
// already in the API response; nothing here needed a new field.
const (
	cellLeft  = `style="padding:5px 8px; border:1px solid #dfe3e6;"`
	cellRight = `align="right" style="padding:5px 8px; border:1px solid #dfe3e6;"`
	headLeft  = `align="left" style="background-color:#f1f3f4; padding:6px 8px; border:1px solid #dfe3e6;"`
	headRight = `align="right" style="background-color:#f1f3f4; padding:6px 8px; border:1px solid #dfe3e6;"`

	// statHead/statCell are the four-column summary grid; statSub/statSubCell
	// the narrower row of components beneath it.
	statHead    = `style="background-color:#f1f3f5; text-align:center; padding:6px 8px; font-size:13px; color:#465868;"`
	statCell    = `style="background-color:#ffffff; text-align:center; padding:2px 8px 8px; font-size:22px; font-weight:700; color:#465868;"`
	statSub     = `style="background-color:#f8f9fa; text-align:center; padding:5px 8px; font-size:12px; color:#8a8f98;"`
	statSubCell = `style="background-color:#ffffff; text-align:center; padding:2px 8px 7px; font-size:14px; color:#465868;"`
)

// pill is one labelled figure: the name above, the value below in WSO2
// orange. ServiceNow used these for its per-team and per-type rows.
//
// A pill is a pair of cells in a shared two-row table rather than a table
// of its own -- that is what keeps every orange bar on one line when a
// team name wraps, which "Americas CS Team - Integration" always does.
type pillCell struct{ label, value string }

// pillRow lays the pills out side by side at equal width.
func pillRow(cells []pillCell) string {
	if len(cells) == 0 {
		return ""
	}
	var labels, values strings.Builder
	for _, c := range cells {
		labels.WriteString(`<td style="padding:0 4px 0 0; vertical-align:bottom;">` +
			`<div style="background-color:#f1f3f5; border:1px solid #e8eaed; border-bottom:0; border-radius:6px 6px 0 0; text-align:center; padding:4px 6px; font-size:12px; color:#465868; word-wrap:break-word;">` +
			escapeHTML(c.label) + `</div></td>`)
		values.WriteString(`<td style="padding:0 4px 0 0; vertical-align:top;">` +
			`<div style="background-color:#ff7101; border-radius:0 0 6px 6px; text-align:center; padding:4px 6px; font-size:13px; font-weight:700; color:#ffffff;">` +
			escapeHTML(c.value) + `</div></td>`)
	}
	return `<table border="0" cellpadding="0" cellspacing="0" width="100%" style="table-layout:fixed; margin:0 0 6px;"><tbody>` +
		`<tr>` + labels.String() + `</tr><tr>` + values.String() + `</tr></tbody></table>`
}

// breakdownCard is one measure -- Members, Issues, Approved/Pending/Total
// time -- with its headline figure and the per-team (and, where the
// original had one, per-type) pills beside it.
func breakdownCard(title, total string, rows ...string) string {
	var body strings.Builder
	for _, r := range rows {
		body.WriteString(r)
	}
	return `<table border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:0 0 10px; border:1px solid #e8eaed; border-radius:8px;"><tbody><tr>` +
		`<td width="130" style="padding:8px 10px; vertical-align:middle; text-align:center; border-right:1px solid #e8eaed;">` +
		`<div style="font-size:13px; color:inherit; opacity:0.75;">` + escapeHTML(title) + `</div>` +
		`<div style="font-size:20px; font-weight:700; color:inherit;">` + escapeHTML(total) + `</div></td>` +
		`<td style="padding:8px 6px 2px 10px;">` + body.String() + `</td>` +
		`</tr></tbody></table>`
}

// WorklogsReportSubject is the mail's subject line, in the same shape the
// delivered reports use: "[2026-10-18] CS Worklogs Daily Report" and
// "[2026-10-05 - 2026-10-11] CS Worklogs Weekly Report".
// caseLink renders one issue key, linked when a base URL is configured.
//
// ServiceNow linked each key into its agent workspace, a URL that stops
// existing at cutover. caseLinkBase is the CSM portal, the equivalent
// internal surface and where the engineers reading these reports
// actually work -- not the customer portal, which is a different
// audience most of them cannot open. The path matches
// csm-notification-service's recipientlinks resolver, which already
// builds <CSM base>/cases/{id} for internal recipients.
//
// Empty renders plain text: an unset deployment shows the key rather
// than a link to the wrong host.
func caseLink(caseLinkBase string, iss worklogreport.IssueRef) string {
	key := escapeHTML(iss.Key)
	if caseLinkBase == "" || iss.ID == "" {
		return key
	}
	href := strings.TrimRight(caseLinkBase, "/") + "/cases/" + url.PathEscape(iss.ID)
	return `<a href="` + escapeHTML(href) +
		`" style="color:#ff7300; text-decoration:none;">` + key + `</a>`
}

func WorklogsReportSubject(r worklogreport.Report) string {
	if r.Cadence == worklogreport.CadenceWeekly {
		return fmt.Sprintf("[%s - %s] CS Worklogs Weekly Report", r.From, r.To)
	}
	return fmt.Sprintf("[%s] CS Worklogs Daily Report", r.From)
}

// RenderWorklogsReport fills in the CS Worklogs HTML email template. One
// template serves both cadences: the daily report shows a Leave column and
// disapproved entries, the weekly one worked days and a per-day average,
// and the shared columns are identical.
func RenderWorklogsReport(r worklogreport.Report, caseLinkBase string) string {
	daily := r.Cadence != worklogreport.CadenceWeekly

	cadenceLabel := "Weekly"
	rangeLabel := "Date range: " + r.From + " to " + r.To
	if daily {
		cadenceLabel = "Daily"
		rangeLabel = "Date: " + r.From
	}

	replacer := strings.NewReplacer(
		"<!-- [CADENCE_LABEL] -->", cadenceLabel,
		"<!-- [RANGE_LABEL] -->", escapeHTML(rangeLabel),
		"<!-- [SUMMARY_GRID] -->", renderWorklogSummary(r, daily),
		"<!-- [BREAKDOWNS] -->", renderWorklogBreakdowns(r),
		"<!-- [TEAM_SECTIONS] -->", renderWorklogTeamSections(r, daily, caseLinkBase),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(worklogsReportTemplate)
}

// teamAcronyms are the parts of a team key that are initialisms rather
// than words. Without them "americas_cre_team" title-cases to "Americas
// Cre Team", which reads worse than the key it replaced.
var teamAcronyms = map[string]string{
	"cs": "CS", "cre": "CRE", "sre": "SRE", "abt": "ABT", "sme": "SME",
	"mi": "MI", "bi": "BI", "iam": "IAM", "apim": "APIM", "rnd": "RnD",
	"qa": "QA", "hr": "HR", "it": "IT", "us": "US", "uk": "UK", "eu": "EU",
}

// teamDisplayName turns a key-shaped team name into something a reader
// expects: "customer_onboarding_team" into "Customer Onboarding Team".
//
// Production stores the key in team.name for most teams, so the mail was
// printing raw keys. A name that already reads as a name -- it has a
// space or a capital, like "Americas CS Team - Integraion" -- is left
// exactly as it is, typo and all: that is the name the recipients know,
// and it is not this function's business to correct it.
func teamDisplayName(name string) string {
	if name == "" || strings.ToLower(name) != name || strings.ContainsAny(name, " -") {
		return name
	}
	parts := strings.Split(name, "_")
	for i, w := range parts {
		if w == "" {
			continue
		}
		if a, ok := teamAcronyms[w]; ok {
			parts[i] = a
			continue
		}
		parts[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(parts, " ")
}

func hrs(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) }

// days renders a worked-day count, which is a whole number unless somebody
// took half-day leave.
func days(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', 1, 64)
}

// renderWorklogSummary is the four-measure grid at the top, with the
// components of each measure on the row beneath -- the shape the
// ServiceNow report has always used. The third column differs by
// cadence: the daily counts who was not on leave, the weekly the mean
// number of people logging on a day.
func renderWorklogSummary(r worklogreport.Report, daily bool) string {
	t := r.Totals
	thirdLabel, thirdValue := "Average People Per Day", hrs(t.AveragePeoplePerDay)
	if daily {
		thirdLabel, thirdValue = "Non Leave Members", days(t.NonLeaveMembers)
	}
	var b strings.Builder
	b.WriteString(`<table class="statGrid" border="0" cellpadding="0" cellspacing="1" width="100%" style="table-layout:fixed; background-color:#e8eaed; margin:0 0 14px; font-size:14px;"><tbody>`)

	b.WriteString(`<tr>`)
	b.WriteString(`<td ` + statHead + `>Total Members</td>`)
	b.WriteString(`<td ` + statHead + ` colspan="2">Total Issues</td>`)
	b.WriteString(`<td ` + statHead + ` colspan="2">Total Support Time (Hrs)</td>`)
	b.WriteString(`<td ` + statHead + ` colspan="2">Average Support Time (per person)</td>`)
	b.WriteString(`</tr><tr>`)
	b.WriteString(`<td ` + statCell + `>` + strconv.Itoa(t.Members) + `</td>`)
	b.WriteString(`<td ` + statCell + ` colspan="2">` + strconv.Itoa(t.Issues) + `</td>`)
	b.WriteString(`<td ` + statCell + ` colspan="2">` + hrs(t.TotalHours) + `</td>`)
	b.WriteString(`<td ` + statCell + ` colspan="2">` + hrs(t.AvgHoursPerPerson) + `</td>`)
	b.WriteString(`</tr><tr>`)
	b.WriteString(`<td ` + statSub + `>` + escapeHTML(thirdLabel) + `</td>`)
	b.WriteString(`<td ` + statSub + `>Queries</td>`)
	b.WriteString(`<td ` + statSub + `>Incidents</td>`)
	b.WriteString(`<td ` + statSub + `>Approved</td>`)
	b.WriteString(`<td ` + statSub + `>Pending</td>`)
	b.WriteString(`<td ` + statSub + `>Approved</td>`)
	b.WriteString(`<td ` + statSub + `>Pending</td>`)
	b.WriteString(`</tr><tr>`)
	b.WriteString(`<td ` + statSubCell + `>` + escapeHTML(thirdValue) + `</td>`)
	b.WriteString(`<td ` + statSubCell + `>` + strconv.Itoa(t.QueryIssues) + `</td>`)
	b.WriteString(`<td ` + statSubCell + `>` + strconv.Itoa(t.IncidentIssues) + `</td>`)
	b.WriteString(`<td ` + statSubCell + `>` + hrs(t.ApprovedHours) + `</td>`)
	b.WriteString(`<td ` + statSubCell + `>` + hrs(t.PendingHours) + `</td>`)
	b.WriteString(`<td ` + statSubCell + `>` + hrs(t.AvgApprovedHoursPerPerson) + `</td>`)
	b.WriteString(`<td ` + statSubCell + `>` + hrs(t.AvgPendingHoursPerPerson) + `</td>`)
	b.WriteString(`</tr></tbody></table>`)
	return b.String()
}

// renderWorklogBreakdowns is the five cards between the grid and the team
// sections. Members and Approved Time break down by team only; the other
// three also carry the work-item types, exactly as the original does.
func renderWorklogBreakdowns(r worklogreport.Report) string {
	if len(r.Teams) == 0 && len(r.ByType) == 0 {
		return `<p style="margin:0 0 14px; font-size:13px; color:#8a8f98;">No time was logged in this period.</p>`
	}
	teamPills := func(value func(worklogreport.TeamReport) string) string {
		cells := make([]pillCell, 0, len(r.Teams))
		for _, t := range r.Teams {
			cells = append(cells, pillCell{teamDisplayName(t.TeamName), value(t)})
		}
		return pillRow(cells)
	}
	typePills := func(value func(worklogreport.TypeBucket) string) string {
		cells := make([]pillCell, 0, len(r.ByType))
		for _, t := range r.ByType {
			cells = append(cells, pillCell{t.Label, value(t)})
		}
		return pillRow(cells)
	}

	var b strings.Builder
	b.WriteString(breakdownCard("Members", strconv.Itoa(r.Totals.Members),
		teamPills(func(t worklogreport.TeamReport) string { return strconv.Itoa(t.Members) })))
	b.WriteString(breakdownCard("Issues", strconv.Itoa(r.Totals.Issues),
		teamPills(func(t worklogreport.TeamReport) string { return strconv.Itoa(t.Issues) }),
		typePills(func(t worklogreport.TypeBucket) string { return strconv.Itoa(t.Issues) })))
	b.WriteString(breakdownCard("Approved Time", hrs(r.Totals.ApprovedHours),
		teamPills(func(t worklogreport.TeamReport) string { return hrs(t.ApprovedHours) }),
		typePills(func(t worklogreport.TypeBucket) string { return hrs(t.ApprovedHours) })))
	b.WriteString(breakdownCard("Pending Time", hrs(r.Totals.PendingHours),
		teamPills(func(t worklogreport.TeamReport) string { return hrs(t.PendingHours) }),
		typePills(func(t worklogreport.TypeBucket) string { return hrs(t.PendingHours) })))
	b.WriteString(breakdownCard("Total Time", hrs(r.Totals.TotalHours),
		teamPills(func(t worklogreport.TeamReport) string { return hrs(t.TotalHours) }),
		typePills(func(t worklogreport.TypeBucket) string { return hrs(t.TotalHours) })))
	return b.String()
}

// renderWorklogTeamSections builds the per-team developer tables -- the
// bulk of the mail. Each opens with the team's own figures on one strip,
// then the member table, whose header groups the approved and pending
// columns under a shared heading the way the ServiceNow report does.
//
// The weekly report carries worked days and an average per day where the
// daily one carries leave and disapproved entries; everything else is
// shared.
func renderWorklogTeamSections(r worklogreport.Report, daily bool, caseLinkBase string) string {
	const (
		stat     = `style="text-align:center; padding:6px 8px;"`
		statName = `style="padding:6px 10px; width:18%;"`
		grpHead  = `style="background-color:#f1f3f5; border-right:2px solid #ffffff; text-align:center; font-weight:700; padding:5px 8px; border-bottom:1px solid #dfe3e6;"`
		subHead  = `style="background-color:#f1f3f5; border-right:2px solid #ffffff; text-align:center; padding:4px 8px;"`
		devCell  = `style="padding:4px 8px; line-height:16px; border-bottom:1px solid #e8eaed;"`
		devMid   = `style="padding:4px 8px; line-height:16px; text-align:center; border-bottom:1px solid #e8eaed;"`
	)
	var b strings.Builder
	for _, t := range r.Teams {
		b.WriteString(`<table class="teamStrip" border="0" cellpadding="0" cellspacing="0" width="100%" style="table-layout:fixed; margin:18px 0 0; background-color:#fff4ec; border:1px solid #ffd9bf; border-radius:8px 8px 0 0; font-size:13px;"><tbody><tr>`)
		b.WriteString(`<td ` + statName + `><div style="font-size:11px; color:inherit; opacity:0.7; letter-spacing:0.5px;">TEAM</div>` +
			`<div style="font-size:17px; font-weight:700; color:inherit;">` + escapeHTML(teamDisplayName(t.TeamName)) + `</div></td>`)
		for _, c := range []struct{ v, l string }{
			{strconv.Itoa(t.Members), "Members"},
			{strconv.Itoa(t.Issues), "Issues"},
			{hrs(t.ApprovedHours), "Approved Hrs."},
			{hrs(t.PendingHours), "Pending Hrs."},
			{hrs(t.AvgApprovedHoursPerPerson), "Avg. Approved Hrs/person"},
			{hrs(t.AvgTotalHoursPerPerson), "Avg. Total Hrs/person"},
		} {
			b.WriteString(`<td ` + stat + `><div style="font-size:17px; font-weight:700; color:inherit;">` +
				escapeHTML(c.v) + `</div><div style="font-size:11px; color:inherit; opacity:0.7;">` + escapeHTML(c.l) + `</div></td>`)
		}
		b.WriteString(`</tr></tbody></table>`)

		secondLabel, lastLabel := "Worked Days", "Avg. Hrs/Day"
		if daily {
			secondLabel, lastLabel = "Leave", "Disapproved Entries"
		}
		b.WriteString(`<table border="0" cellpadding="0" cellspacing="0" width="100%" class="reportTable" style="table-layout:fixed; border-collapse:collapse; font-size:12px; border:1px solid #e8eaed; border-top:0;"><thead>`)
		b.WriteString(`<tr>`)
		b.WriteString(`<th rowspan="2" ` + grpHead + ` width="16%">Developer</th>`)
		b.WriteString(`<th rowspan="2" ` + grpHead + ` width="7%">` + escapeHTML(secondLabel) + `</th>`)
		b.WriteString(`<th colspan="2" ` + grpHead + ` width="14%">Approved Issues</th>`)
		b.WriteString(`<th colspan="2" ` + grpHead + ` width="14%">Pending Issues</th>`)
		b.WriteString(`<th rowspan="2" ` + grpHead + `>Issues</th>`)
		b.WriteString(`<th rowspan="2" ` + grpHead + ` width="11%">` + escapeHTML(lastLabel) + `</th>`)
		b.WriteString(`</tr><tr>`)
		b.WriteString(`<th ` + subHead + `>Issues #</th><th ` + subHead + `>Time (Hours)</th>`)
		b.WriteString(`<th ` + subHead + `>Issues #</th><th ` + subHead + `>Time (Hours)</th>`)
		b.WriteString(`</tr></thead><tbody>`)

		if len(t.Developers) == 0 {
			b.WriteString(`<tr><td colspan="8" style="padding:10px; text-align:center; color:#8a8f98;">No members.</td></tr>`)
		}
		for _, d := range t.Developers {
			b.WriteString(`<tr>`)
			b.WriteString(`<td ` + devCell + `>` + escapeHTML(d.Email) + `</td>`)
			if daily {
				b.WriteString(`<td ` + devMid + `>` + escapeHTML(dashIfEmpty(d.Leave)) + `</td>`)
			} else {
				b.WriteString(`<td ` + devMid + `>` + days(d.WorkedDays) + `</td>`)
			}
			b.WriteString(`<td ` + devMid + `>` + strconv.Itoa(d.ApprovedIssues) + `</td>`)
			b.WriteString(`<td ` + devMid + `>` + hrs(d.ApprovedHours) + `</td>`)
			b.WriteString(`<td ` + devMid + `>` + strconv.Itoa(d.PendingIssues) + `</td>`)
			b.WriteString(`<td ` + devMid + `>` + hrs(d.PendingHours) + `</td>`)
			links := make([]string, 0, len(d.Issues))
			for _, iss := range d.Issues {
				links = append(links, caseLink(caseLinkBase, iss))
			}
			b.WriteString(`<td ` + devCell + `>` + strings.Join(links, " ") + `</td>`)
			if daily {
				b.WriteString(`<td ` + devMid + `>` + strconv.Itoa(d.DisapprovedEntries) + `</td>`)
			} else {
				b.WriteString(`<td ` + devMid + `>` + hrs(d.AvgTimePerDay) + `</td>`)
			}
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</tbody></table>`)
	}
	return b.String()
}

// dashIfEmpty keeps an empty cell from reading as broken data, the same
// convention renderCaseRows uses.
func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
