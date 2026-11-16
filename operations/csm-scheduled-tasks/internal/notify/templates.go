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
)

//go:embed templates/alert.html
var alertTemplateRaw string

//go:embed templates/open_cases_report.html
var openCasesReportTemplateRaw string

//go:embed templates/unresolved_products_report.html
var unresolvedProductsReportTemplateRaw string

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
var openCasesReportTemplate = bakeLogo(openCasesReportTemplateRaw)
var unresolvedProductsReportTemplate = bakeLogo(unresolvedProductsReportTemplateRaw)

// logoTemplates lists every template baked via bakeLogo -- SetLogoURL
// rewrites each of these, so a new template that carries a logo must be
// added here or it won't pick up an override.
func logoTemplates() []*string {
	return []*string{
		&alertTemplate, &openCasesReportTemplate, &unresolvedProductsReportTemplate,
		&staleCasesReportTemplate, &migrationCaseReportTemplate, &productRegressionReportTemplate,
		&queryHoursTemplate, &worklogsReportTemplate,
	}
}

// SetLogoURL replaces wso2LogoURL in every already-baked template with url.
// An empty value keeps the built-in. Mirrors
// integrations/csm-notification-service's own notifications.SetLogoURLs --
// call once at startup, before any email is rendered.
func SetLogoURL(url string) {
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	escaped := html.EscapeString(url)
	for _, t := range logoTemplates() {
		*t = strings.ReplaceAll(*t, wso2LogoURL, escaped)
	}
}

// defaultCompanyName is the name baked into every template by default.
const defaultCompanyName = "WSO2"

// companyName is the name every rendered email currently carries, returned
// by CompanyName (e.g. for the product regression report's own Go-formatted
// group header, which isn't a baked HTML template -- humanizeState's
// separate "wso2" mapping deliberately does NOT read this, see its own doc
// comment). companyNameHTML is the same name HTML-escaped, the baseline
// the next SetCompanyName call rewrites from -- tracking it (rather than
// always diffing from defaultCompanyName) is what makes repeated calls
// correctly reversible. Both are written only by SetCompanyName, so call it
// once at startup, before any email is rendered.
var (
	companyName     = defaultCompanyName
	companyNameHTML = defaultCompanyName
)

// CompanyName returns the name configured by SetCompanyName (or
// defaultCompanyName if it was never called, or called with an empty value).
func CompanyName() string {
	return companyName
}

// companyNamePhrases lists every fixed phrase (not a bare "WSO2") that
// carries the company name in a template body -- never a bare "WSO2", since
// that also appears in the Apache license header every raw template file
// carries as a leading HTML comment. The logo alt text is a generic "Logo"
// in the source templates, not "WSO2", so it is never a target either.
func companyNamePhrases(from, to string) [][2]string {
	return [][2]string{
		{
			fmt.Sprintf("&copy; <!-- [YEAR] --> %s LLC. All Rights Reserved.", from),
			fmt.Sprintf("&copy; <!-- [YEAR] --> %s LLC. All Rights Reserved.", to),
		},
		{
			fmt.Sprintf("%s Case Id", from),
			fmt.Sprintf("%s Case Id", to),
		},
		{
			fmt.Sprintf("%s Case ID", from),
			fmt.Sprintf("%s Case ID", to),
		},
		{
			fmt.Sprintf("%s ID</th>", from),
			fmt.Sprintf("%s ID</th>", to),
		},
		{
			fmt.Sprintf("%s Product</th>", from),
			fmt.Sprintf("%s Product</th>", to),
		},
	}
}

// SetCompanyName replaces the company name in the fixed phrases a template
// body carries (the copyright line, and the "WSO2 Case Id"/"WSO2 Case
// ID"/"WSO2 ID"/"WSO2 Product" report column headers) and updates
// CompanyName's return value (read by the product regression report's own
// group header -- NOT by humanizeState, see its own doc comment for why) --
// not the bare word "WSO2" wherever it appears, which would also corrupt
// the Apache license header every raw template carries as a leading HTML
// comment. An empty value keeps the current name. Unlike SetLogoURL, this
// rewrites from the
// *current* name, not a hardcoded default, so repeated calls are correctly
// reversible. Call once at startup, before any email is rendered.
func SetCompanyName(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	escaped := html.EscapeString(name)
	phrases := companyNamePhrases(companyNameHTML, escaped)
	for _, t := range logoTemplates() {
		for _, phrase := range phrases {
			*t = strings.ReplaceAll(*t, phrase[0], phrase[1])
		}
	}
	companyName = name
	companyNameHTML = escaped
}

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

// OpenCasesReportData holds every value substituted into the "cases still in
// Open state" HTML email template.
type OpenCasesReportData struct {
	Cases []entitycases.Case
	// CaseLinkBase is the CSM portal's base URL; when set, each row's number
	// links to the item's page in the portal. Empty renders plain text.
	CaseLinkBase string
}

// RenderOpenCasesReport fills in the "cases still in Open state" HTML email
// template. It uses renderCaseRows, with its own template file/copy per this component's
// own CLAUDE.md ("Per-task report emails" — each report-sending sub-cron
// owns its own template).
func RenderOpenCasesReport(data OpenCasesReportData) string {
	replacer := strings.NewReplacer(
		"<!-- [CASE_COUNT] -->", strconv.Itoa(len(data.Cases)),
		"<!-- [CASE_ROWS] -->", renderCaseRows(data.Cases, data.CaseLinkBase),
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
	// AfterTable are paragraphs written after the table.
	AfterTable []string
	// Closing, when set, replaces the request to arrange a swap (an email to
	// the rota admins is not about anybody's own shift).
	Closing string
}

// RenderRotaReminder writes a rota reminder as a plain, formal message: a
// greeting, one paragraph, a simple table, the request to arrange a swap, and a
// sign-off. No template, colours or branding. It is HTML only because the email
// service sends HTML.
func RenderRotaReminder(data RotaReminderData) string {
	var b strings.Builder
	b.WriteString(`<div style="font-family:Arial, Helvetica, sans-serif; font-size:14px; line-height:20px;">` + "\n")
	if len(data.IntendedFor) > 0 {
		fmt.Fprintf(&b, "<p><i>[Test copy] Not sent to the rota. This email would have been sent to: %s.</i></p>\n",
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
	for _, p := range data.AfterTable {
		fmt.Fprintf(&b, "<p>%s</p>\n", escapeHTML(p))
	}
	closing := "If you are unable to cover your shift, please arrange a swap with your team lead and have the Team Schedule updated accordingly."
	if data.Closing != "" {
		closing = data.Closing
	}
	b.WriteString("<p>" + escapeHTML(closing))
	if data.ScheduleURL != "" {
		b.WriteString(scheduleLink(data.ScheduleURL))
	}
	b.WriteString("</p>\n<p>Regards,<br/>CSM Team Schedule</p>\n" +
		`<p style="color:#666666; font-size:12px;">This is an automated message. Please do not reply to this email.</p>` + "\n</div>\n")
	return b.String()
}

// WeekendRotaData is the weekend rota's summary email (internal/rotareminder
// builds it): a short introduction, one plain table per rota day with a row
// per window (Rotation and its hours | the family's team | the support team),
// then the rotation definitions. Every text value is plain text, escaped here.
type WeekendRotaData struct {
	// Intro is the opening sentence; Checklist the points under it.
	Intro     string
	Checklist []string
	// Days are the tables, one per rota day, in order.
	Days []WeekendRotaDay
	// TeamTitle and SupportTitle head the day tables' team columns ("CRE
	// Team", "SRE Team"). The support column is left out when no row on any
	// day has support people.
	TeamTitle, SupportTitle string
	// Definitions are the rotation definitions table: one row per window.
	Definitions []RotationDefinition
	// ZoneNotes are the small print under the definitions: what each zone
	// name means, and a coming daylight saving change.
	ZoneNotes []string
	// Notes are written as a list after the tables (what is not yet
	// marked).
	Notes       []string
	ScheduleURL string
	// IntendedFor is set only when reminders are redirected (see
	// RotaReminderData).
	IntendedFor []string
}

// WeekendRotaDay is one day's table: its heading ("Saturday, 14 November")
// and a row per window.
type WeekendRotaDay struct {
	Title string
	Rows  []WeekendRotaRow
}

// WeekendRotaRow is one window of a day: its rotation, its hours that day on
// its own clock ("06:00 AM – 09:00 PM IST"), the family's people on it, and
// the support people whose turns fall in it, by their rotation.
type WeekendRotaRow struct {
	Rotation, Hours string
	Team            []string
	Support         []WeekendRotaGroup
}

// RotationDefinition is one row of the rotation definitions table.
type RotationDefinition struct {
	Rotation, Members, LocalTime, UTCWindow, Role string
	// RoleNote is small print under the role; empty for none.
	RoleNote string
}

// WeekendRotaGroup is one support rotation inside a row: its name, its hours
// and its people.
type WeekendRotaGroup struct {
	Rotation, Hours string
	People          []string
}

// Colours of the weekend summary: greys only, so it stays a plain message
// rather than a branded template.
const (
	weekendMuted  = "#555555"
	weekendBorder = "#d9d9d9"
	weekendShade  = "#f5f5f5"
)

// RenderWeekendRota writes the weekend summary: the greeting and what to do,
// one table per day, the rotation definitions and zone notes, the other
// notes, the handover request, the Team Schedule link and the sign-off.
func RenderWeekendRota(data WeekendRotaData) string {
	var b strings.Builder
	b.WriteString(`<div style="font-family:Arial, Helvetica, sans-serif; font-size:14px; line-height:20px; max-width:900px;">` + "\n")
	if len(data.IntendedFor) > 0 {
		fmt.Fprintf(&b, "<p><i>[Test copy] Not sent to the rota. This email would have been sent to: %s.</i></p>\n",
			escapeHTML(strings.Join(data.IntendedFor, ", ")))
	}
	fmt.Fprintf(&b, "<p>Hello Team,</p>\n<p>%s</p>\n", escapeHTML(data.Intro))
	writeList(&b, data.Checklist)
	withSupport := false
	for _, d := range data.Days {
		for _, r := range d.Rows {
			withSupport = withSupport || len(r.Support) > 0
		}
	}
	for _, d := range data.Days {
		writeWeekendDay(&b, d, data.TeamTitle, data.SupportTitle, withSupport)
	}
	writeDefinitions(&b, data.Definitions)
	if len(data.ZoneNotes) > 0 {
		fmt.Fprintf(&b, `<p style="color:%s; font-size:12px; margin-top:6px;">`, weekendMuted)
		for i, n := range data.ZoneNotes {
			if i > 0 {
				b.WriteString("<br/>")
			}
			b.WriteString(escapeHTML(n))
		}
		b.WriteString("</p>\n")
	}
	if len(data.Notes) > 0 {
		b.WriteString(`<p style="margin:20px 0 6px;"><b>Notes</b></p>` + "\n")
		writeList(&b, data.Notes)
	}
	b.WriteString(`<p style="margin:20px 0 6px;"><b>Handover</b></p>` + "\n" +
		`<p style="margin-top:0;">Please ensure a proper handover to the next team or team member before signing off. ` +
		"This will help ensure there are no gaps in operational coverage during the weekends.</p>\n" +
		"<p>More importantly, we truly appreciate the team's commitment and ownership in supporting our customers and making " +
		"sure they are well taken care of. Your dedication and teamwork are greatly appreciated.</p>\n")
	if data.ScheduleURL != "" {
		b.WriteString("<p>" + strings.TrimSpace(scheduleLink(data.ScheduleURL)) + "</p>\n")
	}
	b.WriteString("<p>Regards,<br/>CSM Team Schedule</p>\n")
	fmt.Fprintf(&b, `<p style="color:%s; font-size:12px;">This is an auto-generated email. Please do not reply to this email.</p>`+"\n</div>\n", weekendMuted)
	return b.String()
}

// writeList writes items as a bulleted list; nothing when there are none.
func writeList(b *strings.Builder, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString(`<ul style="margin:0 0 16px; padding-left:20px;">` + "\n")
	for _, it := range items {
		fmt.Fprintf(b, `<li style="margin-bottom:4px;">%s</li>`+"\n", escapeHTML(it))
	}
	b.WriteString("</ul>\n")
}

// writeWeekendDay writes one day's table: a header row, then a row per
// window: the rotation with that day's hours under it, the family's people
// one per line, and, when withSupport, the support people in it, each
// support rotation's names with its name and hours under them.
func writeWeekendDay(b *strings.Builder, d WeekendRotaDay, teamTitle, supportTitle string, withSupport bool) {
	cell := `<td valign="top" style="padding:8px 10px; border:1px solid ` + weekendBorder + `;">%s</td>`
	small := `<br/><span style="color:` + weekendMuted + `; font-size:12px;">%s</span>`
	fmt.Fprintf(b, `<p style="margin:20px 0 6px; font-size:15px;"><b>%s</b></p>`+"\n", escapeHTML(d.Title))
	fmt.Fprintf(b, `<table width="100%%" cellpadding="0" cellspacing="0" style="border-collapse:collapse; width:100%%; font-size:14px;">`+"\n")
	// Fixed widths, so every day's table lines up with the others.
	heads := []struct{ name, width string }{{"Rotation", "40%"}, {teamTitle, "60%"}}
	if withSupport {
		heads = []struct{ name, width string }{{"Rotation", "30%"}, {teamTitle, "35%"}, {supportTitle, "35%"}}
	}
	b.WriteString("<tr>")
	for _, h := range heads {
		fmt.Fprintf(b, `<th align="left" width="%s" style="background:%s; padding:8px 10px; border:1px solid %s;">%s</th>`,
			h.width, weekendShade, weekendBorder, escapeHTML(h.name))
	}
	b.WriteString("</tr>\n")
	names := func(people []string) string {
		out := make([]string, 0, len(people))
		for _, p := range people {
			out = append(out, escapeHTML(p))
		}
		return strings.Join(out, "<br/>")
	}
	if len(d.Rows) == 0 {
		fmt.Fprintf(b, `<tr><td colspan="%d" style="padding:8px 10px; border:1px solid %s; color:%s;"><i>Nobody rostered</i></td></tr>`+"\n",
			len(heads), weekendBorder, weekendMuted)
	}
	for _, r := range d.Rows {
		rotation := orDash(escapeHTML(r.Rotation))
		if r.Hours != "" {
			rotation += fmt.Sprintf(small, escapeHTML(r.Hours))
		}
		b.WriteString("<tr>")
		fmt.Fprintf(b, cell, rotation)
		fmt.Fprintf(b, cell, orDash(names(r.Team)))
		if withSupport {
			var groups []string
			for _, g := range r.Support {
				label := g.Rotation
				if g.Hours != "" {
					label += ", " + g.Hours
				}
				groups = append(groups, names(g.People)+fmt.Sprintf(small, escapeHTML(label)))
			}
			fmt.Fprintf(b, cell, orDash(strings.Join(groups, "<br/><br/>")))
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</table>\n")
}

// writeDefinitions writes the rotation definitions table: each rotation with
// its member count under it, its hours on its own clock and in UTC, and its
// role with any note under it. Nothing when there are none.
func writeDefinitions(b *strings.Builder, defs []RotationDefinition) {
	if len(defs) == 0 {
		return
	}
	cell := `<td valign="top" style="padding:8px 10px; border:1px solid ` + weekendBorder + `;">%s</td>`
	small := `<br/><span style="color:` + weekendMuted + `; font-size:12px;">%s</span>`
	b.WriteString(`<p style="margin:20px 0 6px; font-size:15px;"><b>Rotation Definitions</b></p>` + "\n")
	b.WriteString(`<table width="100%" cellpadding="0" cellspacing="0" style="border-collapse:collapse; width:100%; font-size:14px;">` + "\n<tr>")
	for _, h := range []struct{ name, width string }{{"Rotation", "30%"}, {"Local time", "25%"}, {"UTC window", "15%"}, {"Role", "30%"}} {
		fmt.Fprintf(b, `<th align="left" width="%s" style="background:%s; padding:8px 10px; border:1px solid %s;">%s</th>`,
			h.width, weekendShade, weekendBorder, h.name)
	}
	b.WriteString("</tr>\n")
	for _, r := range defs {
		rotation := orDash(escapeHTML(r.Rotation))
		if r.Members != "" {
			rotation += fmt.Sprintf(small, escapeHTML(r.Members))
		}
		role := orDash(escapeHTML(r.Role))
		if r.RoleNote != "" {
			role += fmt.Sprintf(small, escapeHTML(r.RoleNote))
		}
		b.WriteString("<tr>")
		fmt.Fprintf(b, cell, rotation)
		fmt.Fprintf(b, cell, orDash(escapeHTML(r.LocalTime)))
		fmt.Fprintf(b, cell, orDash(escapeHTML(r.UTCWindow)))
		fmt.Fprintf(b, cell, role)
		b.WriteString("</tr>\n")
	}
	b.WriteString("</table>\n")
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

// renderCaseRows builds one <tr> per case for RenderOpenCasesReport. A case
// with no account or no assignee (nil AssignedEngineer/AccountDetails on
// entity-service's side, already flattened to "" by entitycases.Client)
// shows an em dash rather than a blank cell, so the report never reads as
// broken/missing data versus genuinely unset.
func renderCaseRows(cases []entitycases.Case, caseLinkBase string) string {
	if len(cases) == 0 {
		return `<tr><td colspan="8" style="padding:16px 10px; text-align:center; color:#8a8f98;">No matching cases.</td></tr>`
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
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`<td align="right" style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%d</td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`</tr>`,
			openCaseNumberCell(caseLinkBase, c), escapeHTML(caseTitle(c)),
			orDash(escapeHTML(caseTypeLabel(c.Type))),
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

// caseTypeLabel is the label the report's Type column shows for an
// entity-service case type value; an unknown value is humanized as it came.
func caseTypeLabel(caseType string) string {
	switch caseType {
	case "case":
		return "Case"
	case "service_request":
		return "Service request"
	case "security_report_analysis":
		return "Security report analysis"
	}
	return humanizeState(caseType)
}

// caseTypePortalPath is the CSM portal's detail route prefix for a case
// type. Each of the three types the report covers has its own route, and
// every one opens by the item's UUID. "" for a type with no known route.
func caseTypePortalPath(caseType string) string {
	switch caseType {
	case "case":
		return "/cases"
	case "service_request":
		return "/operations/service-requests"
	case "security_report_analysis":
		return "/security-center/security-reports"
	}
	return ""
}

// openCaseNumberCell is the row's number, linked to its page in the CSM
// portal (<base><type route>/<case id>) when a base URL is configured, the
// row has an id and its type has a known route; plain text (or a dash for a
// missing number) otherwise. The id is path-escaped because it is data.
func openCaseNumberCell(caseLinkBase string, c entitycases.Case) string {
	text := orDash(escapeHTML(c.Number))
	path := caseTypePortalPath(c.Type)
	if caseLinkBase == "" || c.ID == "" || path == "" || c.Number == "" {
		return text
	}
	href := strings.TrimRight(caseLinkBase, "/") + path + "/" + url.PathEscape(c.ID)
	return fmt.Sprintf(`<a href="%s" target="_blank" style="color:#ff7300;">%s</a>`, escapeHTML(href), text)
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
// Deliberately NOT routed through CompanyName/SetCompanyName: this label
// only ever appears in stale_cases_report/open_cases_report, both sent only
// to whatever address list a task's own SUB_CRON_RECIPIENTS names -- an
// internal, operator-configured audience by both design and code (neither
// report has any customer-contact derivation at all, unlike the genuinely
// customer-facing time_card_report) -- so there's no branding reason to
// make it configurable.
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

// UnresolvedProductNode is one row of the unresolved products digest: a node whose
// reported product or version is not in the product catalogue.
type UnresolvedProductNode struct {
	NodeID           string
	ProjectKey       string
	DeploymentNumber string
	ReportedProduct  string
	ReportedVersion  string
	UnresolvedSince  time.Time
	LastSeenOn       time.Time
}

// UnresolvedProductsReportData holds every value substituted into the unresolved
// products digest template.
type UnresolvedProductsReportData struct {
	WindowHours int
	Nodes       []UnresolvedProductNode
	// Truncated says entity-service had more nodes than it returned.
	Truncated bool
}

// RenderUnresolvedProductsReport fills in the unresolved products digest, one row per
// node in the order given (entity-service groups them by what they reported).
func RenderUnresolvedProductsReport(data UnresolvedProductsReportData) string {
	replacer := strings.NewReplacer(
		"<!-- [NODE_COUNT] -->", strconv.Itoa(len(data.Nodes)),
		"<!-- [WINDOW_HOURS] -->", strconv.Itoa(data.WindowHours),
		"<!-- [NODE_ROWS] -->", renderUnresolvedProductRows(data.Nodes),
		"<!-- [TRUNCATED_NOTE] -->", unresolvedProductsTruncatedNote(data.Truncated),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(unresolvedProductsReportTemplate)
}

func renderUnresolvedProductRows(nodes []UnresolvedProductNode) string {
	if len(nodes) == 0 {
		return `<tr><td colspan="7" style="padding:16px 10px; text-align:center; color:#8a8f98;">No unresolved products.</td></tr>`
	}
	var b strings.Builder
	for _, n := range nodes {
		fmt.Fprintf(&b,
			`<tr>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;"><strong>%s</strong></td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed; font-family:monospace; font-size:12px;">%s</td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed;">%s</td>`+
				`</tr>`,
			escapeHTML(n.ReportedProduct),
			orDash(escapeHTML(n.ReportedVersion)),
			escapeHTML(shortNodeID(n.NodeID)),
			orDash(escapeHTML(n.ProjectKey)),
			orDash(escapeHTML(n.DeploymentNumber)),
			n.UnresolvedSince.UTC().Format("2006-01-02"),
			n.LastSeenOn.UTC().Format("2006-01-02"),
		)
	}
	return b.String()
}

// unresolvedProductsTruncatedNote is the line shown when the list was cut off at
// entity-service's limit, so a reader knows more nodes are missing than are listed.
func unresolvedProductsTruncatedNote(truncated bool) string {
	if !truncated {
		return ""
	}
	return `<p style="margin:0 0 20px; color:#b45309;"><strong>This list is incomplete:</strong> more nodes matched than one digest can hold. ` +
		`The full list is at entity-service <code>GET /instances/unresolved-products</code>.</p>`
}

// shortNodeID keeps a long node id readable in a table cell: products report 64-character
// hashes. The first twelve characters are enough to find the node.
func shortNodeID(id string) string {
	if len(id) <= 16 {
		return id
	}
	return id[:12] + "…"
}
