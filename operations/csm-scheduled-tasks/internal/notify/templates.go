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
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
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

// RotaReminderData is everything a rota reminder says
// (internal/rotareminder builds it). Every text value is plain text, escaped
// here.
type RotaReminderData struct {
	Greeting string
	Intro    string
	// Columns and Rows are the rota table: one row per person and rotation.
	Columns     []string
	Rows        [][]string
	ScheduleURL string
	// IntendedFor is set only when reminders are redirected away from their
	// real recipients (a non-production environment): it names who the email
	// would have gone to.
	IntendedFor []string
}

// RenderRotaReminder writes a rota reminder as a plain, formal message: a
// greeting, one paragraph, a simple table, the request to arrange a swap, and a
// sign-off. No template, colours or branding. It is HTML only because the email
// service sends HTML.
func RenderRotaReminder(data RotaReminderData) string {
	var b strings.Builder
	b.WriteString(`<div style="font-family:Arial, Helvetica, sans-serif; font-size:14px; line-height:20px;">` + "\n")
	if len(data.IntendedFor) > 0 {
		fmt.Fprintf(&b, "<p><i>[Test copy] Rota reminders are redirected in this environment. This email would have been sent to: %s.</i></p>\n",
			escapeHTML(strings.Join(data.IntendedFor, ", ")))
	}
	fmt.Fprintf(&b, "<p>%s</p>\n<p>%s</p>\n", escapeHTML(data.Greeting), escapeHTML(data.Intro))
	b.WriteString(`<table border="1" cellpadding="6" cellspacing="0" style="border-collapse:collapse; font-size:14px;">` + "\n<tr>")
	for _, c := range data.Columns {
		fmt.Fprintf(&b, `<th align="left">%s</th>`, escapeHTML(c))
	}
	b.WriteString("</tr>\n")
	for _, row := range data.Rows {
		b.WriteString("<tr>")
		for _, cell := range row {
			fmt.Fprintf(&b, "<td>%s</td>", orDash(escapeHTML(cell)))
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</table>\n")
	b.WriteString("<p>If you are unable to cover your shift, please arrange a swap with your team lead and have the Team Schedule updated accordingly.")
	if data.ScheduleURL != "" {
		b.WriteString(scheduleLink(data.ScheduleURL))
	}
	b.WriteString("</p>\n<p>Regards,<br/>CSM Team Schedule</p>\n" +
		`<p style="color:#666666; font-size:12px;">This is an automated message. Please do not reply to this email.</p>` + "\n</div>\n")
	return b.String()
}

// scheduleLink is the sentence pointing at the Team Schedule, with the address
// written out as well as linked, so it survives a client that strips links.
func scheduleLink(url string) string {
	u := escapeHTML(url)
	return fmt.Sprintf(` The latest roster is available in the Team Schedule: <a href="%s">%s</a>`, u, u)
}

// RotaReminderFailureData is the alert for a rota reminder run in which some
// windows' emails failed and the others went out (internal/rotareminder).
type RotaReminderFailureData struct {
	Task string
	// Rows are the failed windows: rotation, day, the people on it, the error.
	Rows [][]string
}

// RenderRotaReminderFailure writes that alert, as plainly as the reminders
// themselves. It says why there is no retry, so that whoever reads it knows
// the people listed have to be told by hand.
func RenderRotaReminderFailure(data RotaReminderFailureData) string {
	var b strings.Builder
	b.WriteString(`<div style="font-family:Arial, Helvetica, sans-serif; font-size:14px; line-height:20px;">` + "\n")
	fmt.Fprintf(&b, "<p>Some rota reminders from <b>%s</b> could not be sent. The others went out, so this run is not retried (a retry would email everybody again). Please tell the people below about their shift.</p>\n",
		escapeHTML(data.Task))
	b.WriteString(`<table border="1" cellpadding="6" cellspacing="0" style="border-collapse:collapse; font-size:14px;">` + "\n")
	b.WriteString(`<tr><th align="left">Rotation</th><th align="left">Day</th><th align="left">People</th><th align="left">Error</th></tr>` + "\n")
	for _, row := range data.Rows {
		b.WriteString("<tr>")
		for _, cell := range row {
			fmt.Fprintf(&b, "<td>%s</td>", orDash(escapeHTML(cell)))
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</table>\n</div>\n")
	return b.String()
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
