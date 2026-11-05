package notify

import (
	_ "embed"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/eolusagereport"
)

//go:embed templates/eol_product_usage_report.html
var eolProductUsageReportTemplateRaw string

var eolProductUsageReportTemplate = bakeLogo(eolProductUsageReportTemplateRaw)

// eolUsageIntro is the introduction the ServiceNow report carried, word for
// word.
var eolUsageIntro = []string{
	"This monthly report highlights customers who are still using EOL product versions in their deployments, as reflected in the deployment details available in the Customer Portal.",
	"We need to reach out to the below customers. If the deployment information is outdated, please advise the customer to clean it up and keep their deployment details up to date to maintain data consistency.",
	"If they are still using the EOL versions and have not initiated their migration yet, it is time to proactively engage them and encourage them to migrate to supported versions.",
	"The details are as follows:",
}

// RenderEolProductUsageReport fills in the monthly EOL product usage
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
func RenderEolProductUsageReport(r eolusagereport.Report, portalBaseURL string) string {
	var body string
	if r.TotalVersions == 0 {
		body = fmt.Sprintf(`<p style="margin:0 0 10px;">No customers are using EOL product versions in their Primary Production deployments, as reflected in the deployment details available in the Customer Portal, as of %s.</p>`,
			escapeHTML(r.AsOf))
	} else {
		var b strings.Builder
		for _, p := range eolUsageIntro {
			fmt.Fprintf(&b, `<p style="margin:0 0 10px;">%s</p>`, escapeHTML(p))
		}
		fmt.Fprintf(&b, `<p style="margin:20px 0 0; font-size:13px; color:#8a8f98;">As of %s: %s across %s, in Primary Production deployments.</p>`,
			escapeHTML(r.AsOf), escapeHTML(countOf(r.TotalVersions, "product version", "product versions")),
			escapeHTML(countOf(r.TotalAccounts, "account", "accounts")))
		b.WriteString(renderEolUsageRegions(r.Regions, portalBaseURL))
		body = b.String()
	}
	replacer := strings.NewReplacer(
		"<!-- [BODY] -->", body,
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(eolProductUsageReportTemplate)
}

// countOf is "1 account" or "N accounts".
func countOf(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// eolUsageHeadings are the table's columns and their widths, in order.
var eolUsageHeadings = []struct{ label, width string }{
	{"Account", "15%"}, {"Project", "19%"}, {"Product Version", "15%"}, {"Support Status", "8%"},
	{"EOL Date", "8%"}, {"CS ABT", "7%"}, {"Account Manager", "14%"}, {"Technical Owner", "14%"},
}

// renderEolUsageRegions builds one heading and one table per region.
func renderEolUsageRegions(regions []eolusagereport.Region, portalBaseURL string) string {
	var b strings.Builder
	for _, region := range regions {
		name := region.Region
		if name == "" {
			name = "No region"
		}
		versions := 0
		for _, acc := range region.Accounts {
			versions += accountVersions(acc)
		}
		fmt.Fprintf(&b, `<p style="margin:30px 0 10px; font-size:16px; font-weight:700;">%s <span style="font-weight:400; color:#8a8f98;">(%s, %s)</span></p>`,
			escapeHTML(name), escapeHTML(countOf(len(region.Accounts), "account", "accounts")),
			escapeHTML(countOf(versions, "product version", "product versions")))
		b.WriteString(`<table class="reportTable" border="0" cellpadding="0" cellspacing="0" width="100%" style="table-layout:fixed; font-size:12px; border-collapse:collapse;"><thead><tr>`)
		for _, h := range eolUsageHeadings {
			fmt.Fprintf(&b, `<th align="left" width="%s" style="padding:8px 10px; background-color:#f1f3f5; color:#465868; font-size:12px; text-transform:uppercase; letter-spacing:0.5px;">%s</th>`,
				h.width, escapeHTML(h.label))
		}
		b.WriteString(`</tr></thead><tbody>`)
		for _, acc := range region.Accounts {
			b.WriteString(renderEolUsageAccount(acc, portalBaseURL))
		}
		b.WriteString(`</tbody></table>`)
	}
	return b.String()
}

// accountVersions is how many version rows an account takes.
func accountVersions(acc eolusagereport.Account) int {
	n := 0
	for _, p := range acc.Projects {
		n += len(p.Versions)
	}
	return n
}

// eolUsageCell is one table cell; rowspan is left off when it is 1.
func eolUsageCell(rowspan int, content string) string {
	span := ""
	if rowspan > 1 {
		span = fmt.Sprintf(` rowspan="%d"`, rowspan)
	}
	return fmt.Sprintf(`<td%s style="padding:6px 8px; line-height:16px; border:1px solid #e8eaed; vertical-align:middle; word-wrap:break-word;">%s</td>`, span, content)
}

// renderEolUsageAccount is an account's rows: its own cells on the first
// row, spanning them all, and each project's cell on the project's first
// row, spanning the project's versions.
func renderEolUsageAccount(acc eolusagereport.Account, portalBaseURL string) string {
	total := accountVersions(acc)
	if total == 0 {
		return ""
	}
	var b strings.Builder
	first := true
	for _, p := range acc.Projects {
		for i, v := range p.Versions {
			b.WriteString("<tr>")
			if first {
				b.WriteString(eolUsageCell(total, eolUsageLink(portalBaseURL, "accounts", acc.AccountID, acc.AccountName)))
			}
			if i == 0 {
				name := p.ProjectName
				if p.ProjectKey != "" {
					name += " (" + p.ProjectKey + ")"
				}
				b.WriteString(eolUsageCell(len(p.Versions), eolUsageLink(portalBaseURL, "projects", p.ProjectID, name)))
			}
			b.WriteString(eolUsageCell(1, escapeHTML(strings.TrimSpace(v.ProductName+" "+v.Version))))
			b.WriteString(eolUsageCell(1, escapeHTML(supportStatusLabel(v.SupportStatus))))
			b.WriteString(eolUsageCell(1, `<span style="white-space:nowrap;">`+escapeHTML(v.EOLDate)+`</span>`))
			if first {
				b.WriteString(eolUsageCell(total, escapeHTML(acc.CsAbt)))
				b.WriteString(eolUsageCell(total, eolUsageMailto(acc.AccountManager)))
				b.WriteString(eolUsageCell(total, eolUsageMailto(acc.TechnicalOwner)))
				first = false
			}
			b.WriteString("</tr>")
		}
	}
	return b.String()
}

// supportStatusLabel is the stored enum text in Title case ("EXTENDED" ->
// "Extended"); blank when unset.
func supportStatusLabel(s *string) string {
	if s == nil {
		return ""
	}
	v := strings.ToLower(strings.TrimSpace(*s))
	if v == "" {
		return ""
	}
	return strings.ToUpper(v[:1]) + v[1:]
}

// eolUsageLink is text, linked to the portal page
// <portalBaseURL>/customers/<kind>/<id> when a base URL is configured and
// the row has an id; the id is path-escaped because it is data.
func eolUsageLink(portalBaseURL, kind, id, text string) string {
	if portalBaseURL == "" || id == "" {
		return escapeHTML(text)
	}
	href := strings.TrimRight(portalBaseURL, "/") + "/customers/" + kind + "/" + url.PathEscape(id)
	return fmt.Sprintf(`<a href="%s" target="_blank" style="color:#ff7300;">%s</a>`, escapeHTML(href), escapeHTML(text))
}

// eolUsageMailto is the person's email as a mailto link, blank when there is
// no one.
func eolUsageMailto(p *eolusagereport.Person) string {
	if p == nil || strings.TrimSpace(p.Email) == "" {
		return ""
	}
	email := strings.TrimSpace(p.Email)
	return fmt.Sprintf(`<a href="mailto:%s" style="color:#1a73e8;">%s</a>`, escapeHTML(email), escapeHTML(email))
}
