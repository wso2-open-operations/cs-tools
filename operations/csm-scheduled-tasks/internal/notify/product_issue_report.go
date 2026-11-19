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
	"unicode"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/productissuereport"
)

//go:embed templates/product_issue_report.html
var productIssueReportTemplateRaw string

var productIssueReportTemplate = bakeLogo(productIssueReportTemplateRaw)

// productIssueReportTimezone is the calendar the report's period is named in.
const productIssueReportTimezone = "Asia/Kolkata"

const (
	// productIssueOldestShown is how many oldest waiting-on-CS issues the
	// mail lists; the dashboard route sends up to 20.
	productIssueOldestShown = 10
	// productIssueBlockedShown is how many blocked cases the mail lists.
	productIssueBlockedShown = 20
)

func productIssueLocation() *time.Location {
	if loc, err := time.LoadLocation(productIssueReportTimezone); err == nil {
		return loc
	}
	return time.FixedZone("IST", 5*3600+30*60)
}

// ProductIssueReportSubject is "Product issues: weekly report, 9-15 Nov 2026"
// or "Product issues: monthly report, November 2026"; for a lead's report
// (products non-empty) their products follow in parentheses.
func ProductIssueReportSubject(period string, d productissuereport.Dashboard, products []string) string {
	kind := "weekly"
	if period == productissuereport.PeriodMonth {
		kind = "monthly"
	}
	s := "Product issues: " + kind + " report"
	if label := productIssuePeriodLabel(period, d, productIssueLocation()); label != "" {
		s += ", " + label
	}
	if len(products) > 0 {
		s += " (" + strings.Join(products, ", ") + ")"
	}
	// Product names are data (product_repo_mapping): a control character such as
	// CR/LF must never reach a mail header.
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

// productIssuePeriodLabel names the period: "9-15 Nov 2026" for a week (the
// end is exclusive, so the last day is the day before it) and "November 2026"
// for a month. A response with no period yields "".
func productIssuePeriodLabel(period string, d productissuereport.Dashboard, loc *time.Location) string {
	if d.PeriodStart.IsZero() {
		return ""
	}
	start := d.PeriodStart.In(loc)
	if period == productissuereport.PeriodMonth {
		return start.Format("January 2006")
	}
	if d.PeriodEnd.IsZero() {
		return start.Format("2 Jan 2006")
	}
	last := d.PeriodEnd.In(loc).AddDate(0, 0, -1)
	switch {
	case start.Year() != last.Year():
		return start.Format("2 Jan 2006") + "-" + last.Format("2 Jan 2006")
	case start.Month() != last.Month():
		return start.Format("2 Jan") + "-" + last.Format("2 Jan 2006")
	default:
		return start.Format("2") + "-" + last.Format("2 Jan 2006")
	}
}

// RenderProductIssueReport fills in the product issue report's HTML email.
// products is the lead's product list (empty for the all-products report).
// linkBase, when set, links the dashboard ({base}/dashboard/product-issues)
// and each case ({base}/cases/{caseId}); otherwise they are plain text.
func RenderProductIssueReport(d productissuereport.Dashboard, period string, products []string, linkBase string) string {
	loc := productIssueLocation()
	linkBase = strings.TrimRight(linkBase, "/")
	kind := "Weekly"
	if period == productissuereport.PeriodMonth {
		kind = "Monthly"
	}
	scope := ""
	if len(products) > 0 {
		scope = " (products: " + strings.Join(products, ", ") + ")"
	}
	dashLink := ""
	if linkBase != "" {
		dashLink = fmt.Sprintf(`<p style="margin:0 0 10px;"><a href="%s" target="_blank" style="color:#ff7300;">Open the product issues dashboard</a></p>`,
			escapeHTML(linkBase+"/dashboard/product-issues"))
	}
	generated := ""
	if !d.GeneratedOn.IsZero() {
		generated = d.GeneratedOn.In(loc).Format("2 Jan 2006 15:04") + " IST"
	}

	replacer := strings.NewReplacer(
		"<!-- [HEADING] -->", escapeHTML("Product issues: "+strings.ToLower(kind)+" report"),
		"<!-- [PERIOD] -->", escapeHTML(productIssuePeriodLabel(period, d, loc)),
		"<!-- [SCOPE] -->", escapeHTML(scope),
		"<!-- [GENERATED] -->", escapeHTML(generated),
		"<!-- [DASHBOARD_LINK] -->", dashLink,
		"<!-- [TOTALS] -->", renderProductIssueTotals(d.Totals),
		"<!-- [PRODUCT_HEAD] -->", productIssueHead("Product", "Open", "Opened", "Resolved", "Waiting on CS", "Waiting on product team", "Breached open", "SLA compliance"),
		"<!-- [PRODUCT_ROWS] -->", renderProductIssueProducts(d.ByProduct),
		"<!-- [AGEING] -->", renderProductIssueAgeing(d.WocAgeing.Buckets),
		"<!-- [OLDEST] -->", renderProductIssueItems(firstItems(d.WocAgeing.Oldest, productIssueOldestShown), linkBase, loc, true, "No issues are waiting on CS."),
		"<!-- [BLOCKED_TOTAL] -->", escapeHTML(strconv.Itoa(d.BlockedCases.Total)),
		"<!-- [BLOCKED] -->", renderProductIssueBlocked(d.BlockedCases, linkBase, loc),
		"<!-- [BREACHED] -->", renderProductIssueItems(d.BreachedOpen, linkBase, loc, false, "No open issue has a breached SLA."),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(productIssueReportTemplate)
}

const (
	piHeadCell = `<th align="left" style="padding:8px 10px; background-color:#f1f3f5; color:#465868; font-size:12px; letter-spacing:0.5px;">%s</th>`
	piCell     = `<td style="padding:5px 8px; line-height:16px; border-bottom:1px solid #e8eaed; vertical-align:top; word-wrap:break-word;">%s</td>`
	piNowrap   = `<td style="padding:5px 8px; line-height:16px; border-bottom:1px solid #e8eaed; vertical-align:top; white-space:nowrap;">%s</td>`
	piTable    = `<table class="reportTable" border="0" cellpadding="0" cellspacing="0" width="100%" style="font-size:12px; border-collapse:collapse;">`
)

func piEmptyRow(cols int, msg string) string {
	return fmt.Sprintf(`<tr><td colspan="%d" style="padding:16px 10px; text-align:center; color:#8a8f98;">%s</td></tr>`, cols, escapeHTML(msg))
}

func productIssueHead(cols ...string) string {
	var b strings.Builder
	for _, c := range cols {
		fmt.Fprintf(&b, piHeadCell, escapeHTML(c))
	}
	return b.String()
}

func piInt(n int) string { return strconv.Itoa(n) }

// piCompliance renders a compliance percentage; null (no SLA outcome in the
// period) is a dash, not 0%.
func piCompliance(p *float64) string {
	if p == nil {
		return "&ndash;"
	}
	return escapeHTML(strconv.FormatFloat(*p, 'f', 1, 64) + "%")
}

func piProductName(name *string, owner, repo string) string {
	if name != nil && strings.TrimSpace(*name) != "" {
		return *name
	}
	if owner != "" || repo != "" {
		return "Unmapped (" + owner + "/" + repo + ")"
	}
	return "Unmapped"
}

func renderProductIssueTotals(t productissuereport.Totals) string {
	type kpi struct{ label, value string }
	kpis := []kpi{
		{"Open", piInt(t.Open)},
		{"Opened in period", piInt(t.OpenedInPeriod)},
		{"Resolved in period", piInt(t.ResolvedInPeriod)},
		{"Waiting on CS", piInt(t.WaitingOnCs)},
		{"Waiting on product team", piInt(t.WaitingOnProductTeam)},
		{"Open, no case linked", piInt(t.UnmatchedOpen)},
		{"Open, non-standard column", piInt(t.NonStandardColumnOpen)},
		{"Open, SLA at risk", piInt(t.OpenBySLA.AtRisk)},
		{"Open, SLA breached", piInt(t.OpenBySLA.Breached)},
		{"SLA met in period", piInt(t.SLAMet)},
		{"SLA breached in period", piInt(t.SLABreached)},
		{"SLA compliance", ""},
	}
	var b strings.Builder
	b.WriteString(piTable + "<tbody>")
	for i, k := range kpis {
		if i%4 == 0 {
			b.WriteString("<tr>")
		}
		v := escapeHTML(k.value)
		if k.label == "SLA compliance" {
			v = piCompliance(t.CompliancePercentage)
		}
		fmt.Fprintf(&b, `<td style="padding:10px; border-bottom:1px solid #e8eaed; vertical-align:top; width:25%%;"><span style="color:#8a8f98; font-size:12px;">%s</span><br/><span style="font-size:20px; font-weight:700;">%s</span></td>`, escapeHTML(k.label), v)
		if i%4 == 3 {
			b.WriteString("</tr>")
		}
	}
	b.WriteString("</tbody></table>")
	return b.String()
}

func renderProductIssueProducts(rows []productissuereport.ProductRow) string {
	if len(rows) == 0 {
		return piEmptyRow(8, "No product issues.")
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString("<tr>")
		fmt.Fprintf(&b, piCell, escapeHTML(piProductName(r.ProductName, r.Owner, r.Repository)))
		for _, n := range []int{r.Open, r.OpenedInPeriod, r.ResolvedInPeriod, r.WaitingOnCs, r.WaitingOnProductTeam, r.OpenBySLA.Breached} {
			fmt.Fprintf(&b, piNowrap, piInt(n))
		}
		fmt.Fprintf(&b, piNowrap, piCompliance(r.CompliancePercentage))
		b.WriteString("</tr>")
	}
	return b.String()
}

func renderProductIssueAgeing(buckets []productissuereport.AgeingBucket) string {
	if len(buckets) == 0 {
		return `<p style="margin:0; color:#8a8f98;">No ageing data.</p>`
	}
	var head, row strings.Builder
	for _, bk := range buckets {
		fmt.Fprintf(&head, piHeadCell, escapeHTML(bk.Label))
		fmt.Fprintf(&row, piNowrap, piInt(bk.Count))
	}
	return piTable + "<thead><tr>" + head.String() + "</tr></thead><tbody><tr>" + row.String() + "</tr></tbody></table>"
}

func firstItems(items []productissuereport.Item, n int) []productissuereport.Item {
	if len(items) > n {
		return items[:n]
	}
	return items
}

// piDate renders an instant as a calendar date in the report's zone; zero is "".
func piDate(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	return t.In(loc).Format("2 Jan 2006")
}

// piCaseLink is the case number, linked to {base}/cases/{caseId} when a base is
// set and the case has an id.
func piCaseLink(linkBase, id, number string) string {
	if number == "" {
		number = id
	}
	if linkBase == "" || id == "" {
		return orDash(escapeHTML(number))
	}
	href := linkBase + "/cases/" + url.PathEscape(id)
	return fmt.Sprintf(`<a href="%s" target="_blank" style="color:#ff7300;">%s</a>`, escapeHTML(href), escapeHTML(number))
}

func piIssueText(it productissuereport.Item) string {
	ref := it.Owner + "/" + it.Repository
	if it.IssueNumber > 0 {
		ref += "#" + strconv.FormatInt(it.IssueNumber, 10)
	}
	text := ref
	if it.Title != "" {
		text += " " + it.Title
	}
	return text
}

// renderProductIssueItems renders an issue list. woc swaps the last column
// from the opened date to the date the issue started waiting on CS.
func renderProductIssueItems(items []productissuereport.Item, linkBase string, loc *time.Location, woc bool, empty string) string {
	lastHead := "Opened"
	if woc {
		lastHead = "Waiting since"
	}
	head := productIssueHead("Issue", "Product", "Priority", "Status", "SLA", "Case", lastHead)
	if len(items) == 0 {
		return piTable + "<tbody>" + piEmptyRow(7, empty) + "</tbody></table>"
	}
	var b strings.Builder
	b.WriteString(piTable + "<thead><tr>" + head + "</tr></thead><tbody>")
	for _, it := range items {
		b.WriteString("<tr>")
		text := escapeHTML(piIssueText(it))
		if strings.HasPrefix(it.HTMLURL, "https://") {
			text = fmt.Sprintf(`<a href="%s" target="_blank" style="color:#ff7300;">%s</a>`, escapeHTML(it.HTMLURL), text)
		}
		fmt.Fprintf(&b, piCell, text)
		fmt.Fprintf(&b, piCell, orDash(escapeHTML(piProductName(it.ProductName, it.Owner, it.Repository))))
		fmt.Fprintf(&b, piNowrap, orDash(escapeHTML(it.Priority)))
		fmt.Fprintf(&b, piNowrap, orDash(escapeHTML(it.BoardStatus)))
		sla := ""
		if it.SLA.State != "" {
			sla = it.SLA.State + " " + strconv.FormatFloat(it.SLA.Percentage, 'f', 0, 64) + "%"
		}
		fmt.Fprintf(&b, piNowrap, orDash(escapeHTML(sla)))
		if it.Case != nil {
			fmt.Fprintf(&b, piNowrap, piCaseLink(linkBase, it.Case.ID, it.Case.Number))
		} else {
			fmt.Fprintf(&b, piNowrap, "&mdash;")
		}
		when := it.OpenedOn
		if woc && it.WocSince != nil {
			when = *it.WocSince
		}
		fmt.Fprintf(&b, piNowrap, orDash(escapeHTML(piDate(when, loc))))
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")
	return b.String()
}

func renderProductIssueBlocked(bc productissuereport.BlockedCases, linkBase string, loc *time.Location) string {
	head := productIssueHead("Case", "Subject", "Blocking issues", "Oldest issue opened")
	if len(bc.Cases) == 0 {
		return piTable + "<tbody>" + piEmptyRow(4, "No cases are blocked by product issues.") + "</tbody></table>"
	}
	shown := bc.Cases
	if len(shown) > productIssueBlockedShown {
		shown = shown[:productIssueBlockedShown]
	}
	var b strings.Builder
	b.WriteString(piTable + "<thead><tr>" + head + "</tr></thead><tbody>")
	for _, c := range shown {
		b.WriteString("<tr>")
		fmt.Fprintf(&b, piNowrap, piCaseLink(linkBase, c.CaseID, c.CaseNumber))
		fmt.Fprintf(&b, piCell, orDash(escapeHTML(c.Subject)))
		var issues []string
		for _, is := range c.Issues {
			line := is.Number
			if is.Title != "" {
				line += " " + is.Title
			}
			if is.ProductName != nil && *is.ProductName != "" {
				line += " [" + *is.ProductName + "]"
			}
			if is.BoardStatus != "" {
				line += " (" + is.BoardStatus + ")"
			}
			issues = append(issues, escapeHTML(line))
		}
		fmt.Fprintf(&b, piCell, orDash(strings.Join(issues, "<br/>")))
		fmt.Fprintf(&b, piNowrap, orDash(escapeHTML(piDate(c.OldestOpenedOn, loc))))
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")
	if bc.Total > len(shown) {
		fmt.Fprintf(&b, `<p style="margin:8px 0 0; color:#8a8f98; font-size:12px;">Showing the %d oldest of %d blocked cases.</p>`, len(shown), bc.Total)
	}
	return b.String()
}
