package notify

import (
	_ "embed"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/regressionreport"
)

//go:embed templates/product_regression_report.html
var productRegressionReportTemplateRaw string

var productRegressionReportTemplate = bakeLogo(productRegressionReportTemplateRaw)

// productRegressionReportTimezone is the zone the report's month is named
// in: the one entity-service takes the month in.
const productRegressionReportTimezone = "Asia/Colombo"

// RenderProductRegressionReport fills in the monthly product regression
// report's HTML email template, the same branded shape as the other
// reports here: the introduction the report has always carried, naming
// the business unit and the month, then one table of the cases grouped by
// product with a count per product and in total -- the layout of the
// report it replaces. unitLabel names the business unit ("APIM").
// caseLinkBase is a case page prefix; when set, each row's case id links
// to <caseLinkBase>/<case number>, else it is plain text. [YEAR] is
// today's year, the footer-only detail every template here has.
func RenderProductRegressionReport(r regressionreport.Report, unitLabel, caseLinkBase string) string {
	loc, err := time.LoadLocation(productRegressionReportTimezone)
	if err != nil {
		loc = time.UTC
	}
	replacer := strings.NewReplacer(
		"<!-- [UNIT] -->", escapeHTML(unitLabel),
		"<!-- [MONTH] -->", escapeHTML(r.PeriodStart.In(loc).Format("January 2006")),
		"<!-- [TOTAL] -->", escapeHTML(caseCount(r.TotalCases)),
		"<!-- [ROWS] -->", renderRegressionRows(r.Groups, caseLinkBase),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(productRegressionReportTemplate)
}

// regressionRowColumns is how many columns the table has; a product's
// heading row spans them all.
const regressionRowColumns = 3

// caseCount is "1 Case" or "N Cases", as the report has always counted.
func caseCount(n int) string {
	if n == 1 {
		return "1 Case"
	}
	return strconv.Itoa(n) + " Cases"
}

// renderRegressionRows builds the table body: for each product, a heading
// row naming it with its count, then one row per case in the order given.
func renderRegressionRows(groups []regressionreport.Group, caseLinkBase string) string {
	if len(groups) == 0 {
		return fmt.Sprintf(`<tr><td colspan="%d" style="padding:16px 10px; text-align:center; color:#8a8f98;">No regression cases this month.</td></tr>`, regressionRowColumns)
	}
	const cell = `<td style="padding:8px 10px; border-bottom:1px solid #e8eaed; vertical-align:top;">%s</td>`
	var b strings.Builder
	for _, g := range groups {
		fmt.Fprintf(&b, `<tr><td colspan="%d" style="padding:10px; background-color:#f8f9fa; border-bottom:1px solid #e8eaed; font-weight:700;">WSO2 Product: %s <span style="font-weight:400; color:#8a8f98;">(%d)</span></td></tr>`,
			regressionRowColumns, escapeHTML(g.Product), len(g.Cases))
		for _, row := range g.Cases {
			b.WriteString("<tr>")
			fmt.Fprintf(&b, cell, regressionCaseLink(caseLinkBase, row))
			fmt.Fprintf(&b, cell, orDash(escapeHTML(row.Subject)))
			fmt.Fprintf(&b, cell, orDash(escapeHTML(row.DeployedProduct)))
			b.WriteString("</tr>")
		}
	}
	return b.String()
}

// regressionCaseLink is the row's case id, linked to the case page
// (<caseLinkBase>/<number>) when a base URL is configured. The key falls
// back to the number so the cell is never blank; the number is
// path-escaped because it is data.
func regressionCaseLink(caseLinkBase string, row regressionreport.Row) string {
	text := row.IssueKey
	if text == "" {
		text = row.Number
	}
	if caseLinkBase == "" || row.Number == "" {
		return escapeHTML(text)
	}
	href := strings.TrimRight(caseLinkBase, "/") + "/" + url.PathEscape(row.Number)
	return fmt.Sprintf(`<a href="%s" target="_blank" style="color:#ff7300;">%s</a>`, escapeHTML(href), escapeHTML(text))
}
