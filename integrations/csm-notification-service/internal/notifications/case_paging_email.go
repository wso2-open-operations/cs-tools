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

package notifications

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The Case Alerts rung email: one per rung of the CRE ladder (L1 the ABT
// team lead, L2 every ABT team lead, L3 the CRE head, L4 the CS head) while a
// case stays unacknowledged. Kept simple and readable on a phone: a bar and a
// callout in the severity's colour, the WSO2 logo, the rung first (the inbox
// preview shows it), then the case, the clock and one "View case" button.
// Tables only, inline styles only, ASCII only (entities for the rest): the
// email service does not reliably carry non-ASCII bytes.

//go:embed templates/case_paging.html
var casePagingTemplateRaw string

var casePagingTemplate = bakeLogo(casePagingTemplateRaw)

// CasePagingEmailData is what RenderCasePagingEmail substitutes.
type CasePagingEmailData struct {
	// Greeting follows "Hi": a name, or "ABT Team Leads".
	Greeting string
	// RungLine is the body's first line, CasePagingRungLine: the inbox
	// preview shows it under the shared subject.
	RungLine string
	// Opening is the first sentence, which says why this reader is getting it.
	Opening string
	// CaseNumber and WSO2CaseID identify the case; Title is its short
	// description.
	CaseNumber string
	WSO2CaseID string
	Title      string
	// SeverityCode is "S0".."S4", which picks the colours; Severity is the
	// display label ("S1 - Critical").
	SeverityCode string
	Severity     string
	// Rung is 1..4 and EscalationLevel its display label
	// ("L2 - All ABT Team Leads"); Rung picks the label's colour.
	Rung            int
	EscalationLevel string
	Account         string
	Team            string
	// CreatedAt is when the chain started, already formatted ("... IST").
	CreatedAt string
	// Waiting is how long the case has been unacknowledged ("15 minutes").
	Waiting string
	// Notified lists the earlier rungs, one line each; empty on the first.
	Notified []string
	// NextEscalation names the next rung and when ("CRE head in 10 minutes");
	// empty on the last.
	NextEscalation string
	// CaseURL is the CSM portal's case page; empty renders no link.
	CaseURL string
	// IntendedFor is the real recipients of a debug-redirected send; empty on
	// a real one, which then shows no such line.
	IntendedFor string
}

// CasePagingSubject is the same for every rung of one chain --
//
//	[Case Alerts] S0 CS0099001 not acknowledged
//
// -- so a case's emails group into one conversation (Gmail groups by
// subject; the email service sets no threading headers). Which rung this is
// goes in the body's first line, CasePagingRungLine, which the inbox preview
// shows. A severity change starts a new chain, and so a new conversation.
func CasePagingSubject(severityCode, caseNumber string) string {
	return "[Case Alerts] " + strings.TrimSpace(strings.TrimSpace(severityCode)+" "+strings.TrimSpace(caseNumber)) + " not acknowledged"
}

// CasePagingRungLine says which rung an email is and who it is for:
// "S0 alert at L1 for Atlas ABT Team Lead".
func CasePagingRungLine(severityCode string, rung int, audience string) string {
	line := "alert at L" + strconv.Itoa(rung)
	if code := strings.TrimSpace(severityCode); code != "" {
		line = code + " " + line
	}
	if a := strings.TrimSpace(audience); a != "" {
		line += " for " + a
	}
	return line
}

// casePagingColours is one severity's colours: the bar, its text shade, and
// the rung callout's tint and text.
type casePagingColours struct{ bar, text, tint, tintText string }

var casePagingSeverityColours = map[string]casePagingColours{
	"S0": {"#d92626", "#b91c1c", "#fdecec", "#7f1d1d"},
	"S1": {"#e8590c", "#c2410c", "#fff4e6", "#7a2e04"},
	"S2": {"#f59f00", "#b45309", "#fff9db", "#7c4a03"},
	"S3": {"#1c7ed6", "#1d4ed8", "#e7f5ff", "#0b3d91"},
}

// casePagingNeutral is any other severity (S4, or unknown).
var casePagingNeutral = casePagingColours{"#868e96", "#495057", "#f1f3f5", "#343a40"}

// casePagingRungColours goes from blue at L1 to red at L4, the heads.
var casePagingRungColours = map[int]string{1: "#1d4ed8", 2: "#b45309", 3: "#c2410c", 4: "#b91c1c"}

// RenderCasePagingEmail fills the rung email. Every value is escaped; none of
// it is markup.
func RenderCasePagingEmail(d CasePagingEmailData) string {
	tmpl := applyOptionalBlock(casePagingTemplate, "INTENDED_FOR", d.IntendedFor)
	tmpl = applyOptionalBlock(tmpl, "RUNG_LINE", d.RungLine)
	tmpl = applyOptionalBlock(tmpl, "CASE_URL", d.CaseURL)

	colours, ok := casePagingSeverityColours[strings.ToUpper(strings.TrimSpace(d.SeverityCode))]
	if !ok {
		colours = casePagingNeutral
	}
	const (
		labelCell = `<td width="150" style="padding:6px 12px 6px 0; color:#5f6368; vertical-align:top; border-bottom:1px solid #f1f3f4;">`
		valueCell = `<td style="padding:6px 0; vertical-align:top; border-bottom:1px solid #f1f3f4;%s">`
	)
	var rows strings.Builder
	row := func(label, value, style string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		rows.WriteString("            <tr>" + labelCell + escapeHTML(label) + "</td>" +
			fmt.Sprintf(valueCell, style) + escapeHTML(value) + "</td></tr>\n")
	}
	const bold = " font-weight:bold;"
	row("Case number", d.CaseNumber, bold)
	if d.WSO2CaseID != d.CaseNumber {
		row("WSO2 case ID", d.WSO2CaseID, bold)
	}
	row("Short description", d.Title, "")
	row("Severity", d.Severity, bold+" color:"+colours.text+";")
	row("Account", d.Account, "")
	row("Team", d.Team, "")
	levelStyle := bold
	if c, ok := casePagingRungColours[d.Rung]; ok {
		levelStyle += " color:" + c + ";"
	}
	row("Escalation level", d.EscalationLevel, levelStyle)
	row("Created", d.CreatedAt, "")
	row("Waiting for", d.Waiting, bold)
	first := true
	for _, line := range d.Notified {
		if strings.TrimSpace(line) == "" {
			continue
		}
		label := ""
		if first {
			label, first = "Already notified", false
		}
		rows.WriteString("            <tr>" + labelCell + escapeHTML(label) + "</td>" +
			fmt.Sprintf(valueCell, "") + escapeHTML(line) + "</td></tr>\n")
	}
	row("Next escalation", d.NextEscalation, "")

	return strings.NewReplacer(
		"<!-- [BAR_COLOR] -->", colours.bar,
		"<!-- [CALLOUT_BG] -->", colours.tint,
		"<!-- [CALLOUT_TEXT] -->", colours.tintText,
		"<!-- [INTENDED_FOR] -->", escapeHTML(d.IntendedFor),
		"<!-- [GREETING] -->", escapeHTML(d.Greeting),
		"<!-- [RUNG_LINE] -->", escapeHTML(d.RungLine),
		"<!-- [OPENING] -->", escapeHTML(d.Opening),
		"<!-- [DETAIL_ROWS] -->\n", rows.String(),
		"<!-- [CASE_URL] -->", escapeHTML(d.CaseURL),
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	).Replace(tmpl)
}
