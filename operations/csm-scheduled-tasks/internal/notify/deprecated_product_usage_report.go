package notify

import (
	_ "embed"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/deprecatedusagereport"
)

//go:embed templates/deprecated_product_usage_report.html
var deprecatedProductUsageReportTemplateRaw string

var deprecatedProductUsageReportTemplate = bakeLogo(deprecatedProductUsageReportTemplateRaw)

// deprecatedUsageIntro is the introduction: the ServiceNow report's opening
// sentence, then the same call to action as the EOL product usage report.
var deprecatedUsageIntro = []string{
	"This monthly report highlights customers who use deprecated product versions in their deployments, as reflected in the deployment details available in the Customer Portal.",
	"We need to reach out to the below customers. If the deployment information is outdated, please advise the customer to clean it up and keep their deployment details up to date to maintain data consistency.",
	"Deprecated versions are reaching, or have already passed, their end of life. If these customers have not planned their migration yet, it is time to proactively engage them and encourage them to migrate to supported versions.",
	"The details are as follows:",
}

// RenderDeprecatedProductUsageReport fills in the monthly deprecated product usage
// report's HTML email template, the same branded shape as the other reports
// here: the introduction the ServiceNow report carried, the report date and
// totals, then one table per region in the report's order. Within a region
// each account's cells (account, CS ABT, account manager, technical
// owner) span all its version rows, and each project's cell spans its own,
// the layout of the report it replaces. A report with no versions renders a
// short "no customers" note instead. portalBaseURL is the CSM portal's base
// URL; when set, account and project names link to their pages, else they
// are plain text. [YEAR] is today's year, the footer-only detail every
// template here has.
func RenderDeprecatedProductUsageReport(r deprecatedusagereport.Report, portalBaseURL string) string {
	var body string
	if r.TotalVersions == 0 {
		body = fmt.Sprintf(`<p style="margin:0 0 10px;">No customers are using deprecated product versions in their Primary Production deployments, as reflected in the deployment details available in the Customer Portal, as of %s.</p>`,
			escapeHTML(r.AsOf))
	} else {
		var b strings.Builder
		for _, p := range deprecatedUsageIntro {
			fmt.Fprintf(&b, `<p style="margin:0 0 10px;">%s</p>`, escapeHTML(p))
		}
		fmt.Fprintf(&b, `<p style="margin:20px 0 0; font-size:13px; color:#8a8f98;">As of %s: %s across %s, in Primary Production deployments.</p>`,
			escapeHTML(r.AsOf), escapeHTML(deprecatedCountOf(r.TotalVersions, "product version", "product versions")),
			escapeHTML(deprecatedCountOf(r.TotalAccounts, "account", "accounts")))
		b.WriteString(renderDeprecatedUsageRegions(r.Regions, portalBaseURL))
		body = b.String()
	}
	replacer := strings.NewReplacer(
		"<!-- [BODY] -->", body,
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(deprecatedProductUsageReportTemplate)
}

// deprecatedCountOf is "1 account" or "N accounts".
func deprecatedCountOf(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// deprecatedUsageHeadings are the table's columns and their widths, in order.
var deprecatedUsageHeadings = []struct{ label, width string }{
	{"Account", "15%"}, {"Project", "19%"}, {"Product Version", "15%"}, {"Support Status", "8%"},
	{"EOL Date", "8%"}, {"CS ABT", "7%"}, {"Account Manager", "14%"}, {"Technical Owner", "14%"},
}

// renderDeprecatedUsageRegions builds one heading and one table per region.
func renderDeprecatedUsageRegions(regions []deprecatedusagereport.Region, portalBaseURL string) string {
	var b strings.Builder
	for _, region := range regions {
		name := region.Region
		if name == "" {
			name = "No region"
		}
		versions := 0
		for _, acc := range region.Accounts {
			versions += deprecatedAccountVersions(acc)
		}
		fmt.Fprintf(&b, `<p style="margin:30px 0 10px; font-size:16px; font-weight:700;">%s <span style="font-weight:400; color:#8a8f98;">(%s, %s)</span></p>`,
			escapeHTML(name), escapeHTML(deprecatedCountOf(len(region.Accounts), "account", "accounts")),
			escapeHTML(deprecatedCountOf(versions, "product version", "product versions")))
		b.WriteString(`<table class="reportTable" border="0" cellpadding="0" cellspacing="0" width="100%" style="table-layout:fixed; font-size:12px; border-collapse:collapse;"><thead><tr>`)
		for _, h := range deprecatedUsageHeadings {
			fmt.Fprintf(&b, `<th align="left" width="%s" style="padding:8px 10px; background-color:#f1f3f5; color:#465868; font-size:12px; text-transform:uppercase; letter-spacing:0.5px;">%s</th>`,
				h.width, escapeHTML(h.label))
		}
		b.WriteString(`</tr></thead><tbody>`)
		for _, acc := range region.Accounts {
			b.WriteString(renderDeprecatedUsageAccount(acc, portalBaseURL))
		}
		b.WriteString(`</tbody></table>`)
	}
	return b.String()
}

// deprecatedAccountVersions is how many version rows an account takes.
func deprecatedAccountVersions(acc deprecatedusagereport.Account) int {
	n := 0
	for _, p := range acc.Projects {
		n += len(p.Versions)
	}
	return n
}

// deprecatedUsageCell is one table cell; rowspan is left off when it is 1.
func deprecatedUsageCell(rowspan int, content string) string {
	span := ""
	if rowspan > 1 {
		span = fmt.Sprintf(` rowspan="%d"`, rowspan)
	}
	return fmt.Sprintf(`<td%s style="padding:6px 8px; line-height:16px; border:1px solid #e8eaed; vertical-align:middle; word-wrap:break-word;">%s</td>`, span, content)
}

// renderDeprecatedUsageAccount is an account's rows: its own cells on the first
// row, spanning them all, and each project's cell on the project's first
// row, spanning the project's versions.
func renderDeprecatedUsageAccount(acc deprecatedusagereport.Account, portalBaseURL string) string {
	total := deprecatedAccountVersions(acc)
	if total == 0 {
		return ""
	}
	var b strings.Builder
	first := true
	for _, p := range acc.Projects {
		for i, v := range p.Versions {
			b.WriteString("<tr>")
			if first {
				b.WriteString(deprecatedUsageCell(total, deprecatedUsageLink(portalBaseURL, "accounts", acc.AccountID, acc.AccountName)))
			}
			if i == 0 {
				name := p.ProjectName
				if p.ProjectKey != "" {
					name += " (" + p.ProjectKey + ")"
				}
				b.WriteString(deprecatedUsageCell(len(p.Versions), deprecatedUsageLink(portalBaseURL, "projects", p.ProjectID, name)))
			}
			b.WriteString(deprecatedUsageCell(1, escapeHTML(strings.TrimSpace(v.ProductName+" "+v.Version))))
			b.WriteString(deprecatedUsageCell(1, escapeHTML(deprecatedSupportStatusLabel(v.SupportStatus))))
			b.WriteString(deprecatedUsageCell(1, `<span style="white-space:nowrap;">`+escapeHTML(deprecatedEOLDate(v.EOLDate))+`</span>`))
			if first {
				b.WriteString(deprecatedUsageCell(total, escapeHTML(acc.CsAbt)))
				b.WriteString(deprecatedUsageCell(total, deprecatedUsageMailto(acc.AccountManager)))
				b.WriteString(deprecatedUsageCell(total, deprecatedUsageMailto(acc.TechnicalOwner)))
				first = false
			}
			b.WriteString("</tr>")
		}
	}
	return b.String()
}

// deprecatedSupportStatusLabel is the stored enum text in Title case ("EXTENDED" ->
// "Extended"); blank when unset.
func deprecatedSupportStatusLabel(s *string) string {
	if s == nil {
		return ""
	}
	v := strings.ToLower(strings.TrimSpace(*s))
	if v == "" {
		return ""
	}
	return strings.ToUpper(v[:1]) + v[1:]
}

// deprecatedUsageLink is text, linked to the portal page
// <portalBaseURL>/customers/<kind>/<id> when a base URL is configured and
// the row has an id; the id is path-escaped because it is data.
func deprecatedUsageLink(portalBaseURL, kind, id, text string) string {
	if portalBaseURL == "" || id == "" {
		return escapeHTML(text)
	}
	href := strings.TrimRight(portalBaseURL, "/") + "/customers/" + kind + "/" + url.PathEscape(id)
	return fmt.Sprintf(`<a href="%s" target="_blank" style="color:#ff7300;">%s</a>`, escapeHTML(href), escapeHTML(text))
}

// deprecatedUsageMailto is the person's email as a mailto link, blank when there is
// no one.
func deprecatedUsageMailto(p *deprecatedusagereport.Person) string {
	if p == nil || strings.TrimSpace(p.Email) == "" {
		return ""
	}
	email := strings.TrimSpace(p.Email)
	return fmt.Sprintf(`<a href="mailto:%s" style="color:#1a73e8;">%s</a>`, escapeHTML(email), escapeHTML(email))
}

// deprecatedEOLDate is a version's EOL date as the table shows it: a
// deprecated version may not have one yet.
func deprecatedEOLDate(d string) string {
	if d == "" {
		return "Not set"
	}
	return d
}
