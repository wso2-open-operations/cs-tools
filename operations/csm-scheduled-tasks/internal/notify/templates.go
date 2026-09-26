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
	"html"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhoursreport"
)

//go:embed templates/alert.html
var alertTemplateRaw string

//go:embed templates/stale_cases_report.html
var staleCasesReportTemplateRaw string

//go:embed templates/open_cases_report.html
var openCasesReportTemplateRaw string

// wso2LogoURL is the white WSO2 logo variant — the alert template's header
// sits on an orange background, unlike
// integrations/csm-notification-service's own equivalent constant of the
// same name (a white background there, black logo). Same reasoning
// otherwise: Gmail (and most major webmail clients) strip data: URI images
// from received HTML mail as a security measure, so an inline base64 logo
// shows as broken regardless of encoding — this CDN URL is WSO2's own
// public asset host, not third-party hosting, and is the only option
// confirmed to actually render.
const wso2LogoURL = "https://wso2.cachefly.net/wso2/sites/all/image_resources/logos/WSO2-Logo-White.png"

// bakeLogo substitutes wso2LogoURL into raw's <!-- [LOGO_SRC] --> placeholder
// once at package init, since the logo never varies between emails.
func bakeLogo(raw string) string {
	return strings.Replace(raw, "<!-- [LOGO_SRC] -->", wso2LogoURL, 1)
}

var alertTemplate = bakeLogo(alertTemplateRaw)
var staleCasesReportTemplate = bakeLogo(staleCasesReportTemplateRaw)
var openCasesReportTemplate = bakeLogo(openCasesReportTemplateRaw)

// escapeHTML mirrors integrations/csm-notification-service's own
// internal/notifications.escapeHTML exactly: HTML-escapes s and
// additionally converts every non-ASCII rune to a numeric HTML character
// reference. A browser renders raw UTF-8 fine, but the external
// email-sending service both this client and that one call doesn't
// reliably preserve non-ASCII bytes through its own send path — a raw
// multi-byte character arrives as "?" in the recipient's inbox. A numeric
// character reference is pure ASCII on the wire, so it survives regardless,
// and any HTML-capable mail client decodes it back on display.
func escapeHTML(s string) string {
	var b strings.Builder
	for _, r := range html.EscapeString(s) {
		if r > 127 {
			b.WriteString("&#")
			b.WriteString(strconv.Itoa(int(r)))
			b.WriteByte(';')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// escapeMultiline HTML-escapes s (see escapeHTML) and converts its newlines
// to <br> — a wrapped Go error's message can legitimately contain
// newlines (e.g. from a downstream multi-line response body folded into
// %w), so this keeps that structure instead of running it all together.
func escapeMultiline(s string) string {
	return strings.ReplaceAll(escapeHTML(s), "\n", "<br>")
}

// AlertEmailData holds every value substituted into the "sub-cron failed"
// HTML email template.
type AlertEmailData struct {
	TaskName     string
	Period       string
	AttemptCount int
	NextRetry    string
	Error        string
}

// RenderAlertEmail fills in the "sub-cron failed" HTML email template.
// [YEAR] is today's year, not part of AlertEmailData — it's a footer
// copyright detail, not information about the failure itself.
func RenderAlertEmail(data AlertEmailData) string {
	replacer := strings.NewReplacer(
		"<!-- [TASK_NAME] -->", escapeHTML(data.TaskName),
		"<!-- [PERIOD] -->", escapeHTML(data.Period),
		"<!-- [ATTEMPT_COUNT] -->", strconv.Itoa(data.AttemptCount),
		"<!-- [NEXT_RETRY] -->", escapeHTML(data.NextRetry),
		"<!-- [ERROR] -->", escapeMultiline(data.Error),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(alertTemplate)
}

// StaleCasesReportData holds every value substituted into the "cases open
// too long" HTML email template.
type StaleCasesReportData struct {
	ThresholdDays int
	Cases         []entitycases.Case
}

// RenderStaleCasesReport fills in the "cases open too long" HTML email
// template — one table row per case, oldest (by CreatedOn) first, exactly
// the order internal/entitycases.Client.SearchOpenCasesOlderThan already
// returns them in. [YEAR] is today's year, the same footer-only detail
// RenderAlertEmail's own doc comment describes.
func RenderStaleCasesReport(data StaleCasesReportData) string {
	replacer := strings.NewReplacer(
		"<!-- [CASE_COUNT] -->", strconv.Itoa(len(data.Cases)),
		"<!-- [THRESHOLD_DAYS] -->", strconv.Itoa(data.ThresholdDays),
		"<!-- [CASE_ROWS] -->", renderCaseRows(data.Cases),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(staleCasesReportTemplate)
}

// OpenCasesReportData holds every value substituted into the "cases still in
// Open state" HTML email template.
type OpenCasesReportData struct {
	Cases []entitycases.Case
}

// RenderOpenCasesReport fills in the "cases still in Open state" HTML email
// template — shares renderCaseRows with RenderStaleCasesReport (the table
// shape is identical), but its own template file/copy per this component's
// own CLAUDE.md ("Per-task report emails" — each report-sending sub-cron
// owns its own template).
func RenderOpenCasesReport(data OpenCasesReportData) string {
	replacer := strings.NewReplacer(
		"<!-- [CASE_COUNT] -->", strconv.Itoa(len(data.Cases)),
		"<!-- [CASE_ROWS] -->", renderCaseRows(data.Cases),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(openCasesReportTemplate)
}

// renderCaseRows builds one <tr> per case for RenderStaleCasesReport. A case
// with no account or no assignee (nil AssignedEngineer/AccountDetails on
// entity-service's side, already flattened to "" by entitycases.Client)
// shows an em dash rather than a blank cell, so the report never reads as
// broken/missing data versus genuinely unset.
func renderCaseRows(cases []entitycases.Case) string {
	if len(cases) == 0 {
		return `<tr><td colspan="7" style="padding:16px 10px; text-align:center; color:#8a8f98;">No matching cases.</td></tr>`
	}

	now := time.Now()
	var b strings.Builder
	for _, c := range cases {
		daysOpen := int(now.Sub(c.CreatedOn).Hours() / 24)
		fmt.Fprintf(&b,
			`<tr>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s<br/><span style="color:#8a8f98; font-size:12px;">%s</span></td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`<td align="right" style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%d</td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`</tr>`,
			orDash(escapeHTML(c.Number)), escapeHTML(caseTitle(c)),
			orDash(escapeHTML(c.Account)),
			orDash(escapeHTML(c.AssignedTo)),
			escapeHTML(humanizeState(c.State)),
			orDash(escapeHTML(c.Severity)),
			daysOpen,
			c.UpdatedOn.Format("2006-01-02"),
		)
	}
	return b.String()
}

// caseTitle returns a short display title for a case row: its subject, or
// its internal (WSO2) case id when Subject is unavailable — never both,
// there's no room for it in one report row.
func caseTitle(c entitycases.Case) string {
	if c.Subject != "" {
		return c.Subject
	}
	return c.InternalID
}

// orDash returns s, or an em dash if s is empty — see renderCaseRows' own
// doc comment for why.
func orDash(s string) string {
	if s == "" {
		return "&mdash;"
	}
	return s
}

// humanizeState turns entity-service's snake_case case state (e.g.
// "work_in_progress") into a display form ("Work In Progress"). "wso2" is
// special-cased to the all-caps brand form ("WSO2") rather than the
// title-case default ("Wso2") every other word gets — the only state value
// (CaseStateWaitingOnWSO2 = "waiting_on_wso2") that contains it.
func humanizeState(state string) string {
	words := strings.Split(state, "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		if strings.EqualFold(w, "wso2") {
			words[i] = "WSO2"
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

//go:embed templates/query_hours_weekly_report.html
var queryHoursWeeklyReportTemplateRaw string

// QueryHoursWeeklyReportData holds everything substituted into the weekly
// query-support consumption report template.
type QueryHoursWeeklyReportData struct {
	Report queryhoursreport.Report
	// SalesforceBaseURL turns account, opportunity and project cells into
	// links. Empty leaves them as plain text — the report is still complete
	// and readable, which is why an unset value is not an error.
	SalesforceBaseURL string
}

// timeConvert formats a minute count the way the whole query-hour family
// does: "100h 0m".
//
// This is SN_Utils.timeConvert, reproduced exactly, and the exactness matters
// more than it looks. The ServiceNow original is:
//
//	return sign + " " + rhours + "h " + rminutes + "m";
//
// with `sign` being "-" for negatives and "" otherwise — so EVERY value
// carries a space after the sign position. Negatives render "- 4h 35m" and
// positives render " 4h 35m", with a leading space. Dropping that space, or
// rendering "-4h 35m", makes every cell in a side-by-side comparison against
// the original differ.
func timeConvert(minutes int) string {
	sign := ""
	if minutes < 0 {
		sign = "-"
		minutes = -minutes
	}
	return sign + " " + strconv.Itoa(minutes/60) + "h " + strconv.Itoa(minutes%60) + "m"
}

// RenderQueryHoursWeeklyReport fills in the weekly query-support consumption
// report template.
//
// Its own template and its own renderer, per this component's CLAUDE.md
// ("Per-task report emails"). It shares nothing with the per-project
// threshold email that csm-notification-service sends — see
// internal/queryhoursreport's package doc for why keeping them apart is a
// correctness requirement rather than a style preference.
func RenderQueryHoursWeeklyReport(data QueryHoursWeeklyReportData) string {
	replacer := strings.NewReplacer(
		"<!-- [LOGO_SRC] -->", bakeLogo(wso2LogoURL),
		"<!-- [GENERATED_ON] -->", escapeHTML(data.Report.GeneratedOn),
		"<!-- [EXCEEDED_COUNT] -->", strconv.Itoa(data.Report.ExceededCount),
		"<!-- [GOING_TO_EXCEED_COUNT] -->", strconv.Itoa(data.Report.GoingToExceedCount),
		"<!-- [UNMATCHED_NOTE] -->", unmatchedNote(data.Report.UnmatchedLineCount),
		"<!-- [EXCEEDED_ROWS] -->", renderReportAccounts(data.Report.Exceeded, data.SalesforceBaseURL),
		"<!-- [GOING_TO_EXCEED_ROWS] -->", renderReportAccounts(data.Report.GoingToExceed, data.SalesforceBaseURL),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(queryHoursWeeklyReportTemplateRaw)
}

// unmatchedNote surfaces product lines whose name matched none of the six
// entitlement packs.
//
// Silence here is the failure mode worth guarding: ServiceNow contributes
// zero hours for an unrecognised product name and says nothing, so a pack
// renamed in Salesforce understates every entitlement derived from it and the
// report still looks perfectly normal. This makes that visible in the one
// place people actually read.
func unmatchedNote(count int) string {
	if count == 0 {
		return ""
	}
	return `<p style="margin:0 0 24px; padding:12px 14px; border-left:4px solid #ff7101; background:#fff6ef; font-size:13px; color:#465868;">` +
		`<strong>` + strconv.Itoa(count) + `</strong> in-service product line(s) matched none of the six known ` +
		`query-hour packs and contributed zero hours. A pack renamed or added in Salesforce understates ` +
		`every entitlement derived from it.</p>`
}

// renderReportAccounts renders one table's worth of account sections.
func renderReportAccounts(accounts []queryhoursreport.Account, sfBase string) string {
	if len(accounts) == 0 {
		return `<tr><td colspan="7" style="padding:14px 10px; border:1px solid #d7dade; color:#8a8f98;">` +
			`No accounts in this category.</td></tr>`
	}
	var b strings.Builder
	for _, account := range accounts {
		renderReportAccount(&b, account, sfBase)
	}
	return b.String()
}

// renderReportAccount renders one account, spanning its name down every row
// it owns and each group's totals down that group's rows — the rowspan layout
// the ServiceNow report uses.
func renderReportAccount(b *strings.Builder, account queryhoursreport.Account, sfBase string) {
	firstRowOfAccount := true

	for _, group := range account.Groups {
		firstRowOfGroup := true

		for _, opportunity := range group.Opportunities {
			firstRowOfOpportunity := true

			for _, project := range opportunity.Projects {
				b.WriteString("<tr>")

				if firstRowOfAccount {
					b.WriteString(cell(account.RowCount, "left", "",
						link(accountURL(sfBase, account.SFID), account.Name)))
					firstRowOfAccount = false
				}
				if firstRowOfOpportunity {
					b.WriteString(cell(len(opportunity.Projects), "left", "",
						link(opportunityURL(sfBase, opportunity.SFID), opportunity.Name)))
					firstRowOfOpportunity = false
				}
				if firstRowOfGroup {
					b.WriteString(cell(group.RowCount, "right", "color:#26814f; font-weight:700;",
						escapeHTML(timeConvert(group.EntitlementMinutes))))
				}

				b.WriteString(projectCell(project, sfBase))
				b.WriteString(consumedCell(project))

				if firstRowOfGroup {
					b.WriteString(cell(group.RowCount, "right", "font-weight:700;",
						escapeHTML(timeConvert(group.ConsumedMinutes))))
					b.WriteString(cell(group.RowCount, "right",
						"font-weight:700; color:"+remainsColour(group)+";",
						escapeHTML(timeConvert(group.RemainingMinutes))))
					firstRowOfGroup = false
				}

				b.WriteString("</tr>")
			}
		}
	}
}

// remainsColour mirrors the original's crimson/tomato signalling.
func remainsColour(group queryhoursreport.Group) string {
	switch {
	case group.Exceeded:
		return "#dc143c"
	case group.GoingToExceed:
		return "#ff6347"
	default:
		return "#26814f"
	}
}

// cell writes one rowspan-aware table cell.
func cell(rowspan int, align, extraStyle, content string) string {
	if rowspan < 1 {
		rowspan = 1
	}
	return `<td align="` + align + `" rowspan="` + strconv.Itoa(rowspan) +
		`" style="padding:6px 10px; border:1px solid #d7dade; ` + extraStyle + `">` + content + `</td>`
}

// projectCell renders the project cell: name, key, and a Salesforce link —
// the seven-column format's own layout. The per-project threshold email
// deliberately shows a bare project name instead; do not converge them.
func projectCell(project queryhoursreport.Project, sfBase string) string {
	style := "padding:6px 10px; border:1px solid #d7dade;"
	keyColour := "#585555"
	if project.Duplicate {
		style += " background:#eceff1; color:#8a8f98;"
		keyColour = "#8a8f98"
	}
	out := `<td style="` + style + `">` + escapeHTML(project.Name)
	if project.Key != "" {
		out += `<br/><span style="color:` + keyColour + `; font-size:12px;">- Project Key : <b>` +
			escapeHTML(project.Key) + `</b></span>`
	}
	if url := projectURL(sfBase, project.SFID); url != "" {
		out += `<div style="text-align:right; font-size:12px;">[` + link(url, "salesforce") + `]</div>`
	}
	return out + `</td>`
}

// consumedCell renders one project's own consumed figure.
func consumedCell(project queryhoursreport.Project) string {
	style := "padding:6px 10px; border:1px solid #d7dade; font-weight:700;"
	if project.Duplicate {
		style += " background:#eceff1; color:#8a8f98;"
	}
	return `<td align="right" style="` + style + `">` +
		escapeHTML(timeConvert(project.ConsumedMinutes)) + `</td>`
}

// link renders an anchor, or plain text when there is no URL to point at.
func link(url, text string) string {
	if url == "" {
		return escapeHTML(text)
	}
	return `<a href="` + escapeHTML(url) + `" target="_blank" ` +
		`style="color:#1798c1; font-weight:600; text-decoration:none;">` + escapeHTML(text) + `</a>`
}

// The three Salesforce record paths the report links to. Each returns empty
// when either the base URL or the record's Salesforce id is missing, which
// the caller renders as plain text rather than a broken link.
func accountURL(base, sfID string) string     { return sfRecordURL(base, "Account", sfID) }
func opportunityURL(base, sfID string) string { return sfRecordURL(base, "Opportunity", sfID) }
func projectURL(base, sfID string) string     { return sfRecordURL(base, "Project__c", sfID) }

func sfRecordURL(base, object, sfID string) string {
	if base == "" || sfID == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + "/lightning/r/" + object + "/" + url.PathEscape(sfID) + "/view"
}
