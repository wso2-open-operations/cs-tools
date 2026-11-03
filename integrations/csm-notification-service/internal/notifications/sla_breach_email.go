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
	"strings"
)

//go:embed templates/sla_breach_team.html
var slaBreachTeamTemplateRaw string

//go:embed templates/sla_breach_assignee.html
var slaBreachAssigneeTemplateRaw string

var (
	slaBreachTeamTemplate     = bakeLogo(slaBreachTeamTemplateRaw)
	slaBreachAssigneeTemplate = bakeLogo(slaBreachAssigneeTemplateRaw)
)

// slaBreachHeaderTitle builds the one header/subject wording shared by the
// SLA breach Chat card (SendSLABreachAlert) and both breach emails below —
// kept as one function so all three always read identically for the same
// tier crossing: "<ClockLabel> SLA at Risk - <caseRef>" below 100%,
// "[<SEVERITY>] <ClockLabel> SLA Violation - <caseRef>" at a genuine 100%
// breach (severity uppercase, unresolved, matching SendSLABreachAlert's own
// header convention exactly).
func slaBreachHeaderTitle(clockType, tier, severity, caseNumber, wso2CaseID string) string {
	clockLabel := slaClockTypeLabel(clockType)
	caseRef := chatHeaderCaseRef(caseNumber, wso2CaseID)
	if tier == "100" {
		return fmt.Sprintf("[%s] %s SLA Violation - %s", strings.ToUpper(strings.TrimSpace(severity)), clockLabel, caseRef)
	}
	return fmt.Sprintf("%s SLA at Risk - %s", clockLabel, caseRef)
}

// SLABreachEmailSubject is slaBreachHeaderTitle exported for
// internal/slaengine's own use as both emails' Subject line — the same
// wording as the header inside the body, so a reader sees one consistent
// description of the same event in their inbox and in the email itself.
func SLABreachEmailSubject(clockType, tier, severity, caseNumber, wso2CaseID string) string {
	return slaBreachHeaderTitle(clockType, tier, severity, caseNumber, wso2CaseID)
}

// SLABreachEmailData holds every value shared by RenderSLABreachAssigneeEmail/
// RenderSLABreachTeamEmail — the same fields SendSLABreachAlert's own Chat
// card takes, since both describe the identical tier crossing; see
// Engine.sendBreachEmails' own doc comment for the exact source of each
// (the GetClockState call sendBreachAlert already makes carries all four
// email-only fields now, no second lookup).
// IntendedFor, when non-empty, shows a "Sent to: <value>" row — the real
// recipient this specific send actually went to. Only ever populated for a
// debug-redirected send (see Engine.sendBreachEmails' own EMAIL_DEBUG_MODE
// handling); a real (non-debug) send leaves this empty and the row is
// omitted entirely, not rendered blank.
type SLABreachEmailData struct {
	ClockType    string
	Tier         string
	Severity     string
	CaseNumber   string
	WSO2CaseID   string
	CaseTitle    string
	CaseType     string
	Product      string
	Team         string
	TeamLeadName string
	State        string
	OpenedAt     string
	CaseLink     string
	IntendedFor  string
}

// RenderSLABreachAssigneeEmail fills in the short, personal "your case's
// SLA is at risk" notice sent to the case's assigned engineer.
// assigneeName may be empty (entity-service has no assignee name resolved,
// or the case has none at all) — the greeting then reads "Hello," which is
// a little bare but never wrong, same posture as every other optional
// display field in this package.
func RenderSLABreachAssigneeEmail(assigneeName string, d SLABreachEmailData) string {
	tmpl := applyOptionalBlock(slaBreachAssigneeTemplate, "INTENDED_FOR", d.IntendedFor)
	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(d.IntendedFor),
		"<!-- [HEADER_TITLE] -->", escapeHTML(slaBreachHeaderTitle(d.ClockType, d.Tier, d.Severity, d.CaseNumber, d.WSO2CaseID)),
		"<!-- [ASSIGNEE_NAME] -->", escapeHTML(assigneeName),
		"<!-- [CLOCK_LABEL] -->", escapeHTML(slaClockTypeLabel(d.ClockType)),
		"<!-- [CASE_NUMBER] -->", escapeHTML(d.CaseNumber),
		"<!-- [CASE_LINK] -->", escapeHTML(d.CaseLink),
		"<!-- [SLA_COLOR] -->", slaBreachColor(d.Tier),
		"<!-- [TIER] -->", escapeHTML(d.Tier),
	)
	return replacer.Replace(tmpl)
}

// RenderSLABreachTeamEmail fills in the full-detail breach notice sent to
// the case's team email group — the same field set as SendSLABreachAlert's
// own Chat card (Title/Case ID/Type/Product/Team/Priority/State/Opened At/
// SLA Percentage), as an email instead of (or in addition to) that Chat
// post. teamLeadName, when non-empty, is appended to the Team line in
// parens ("<team> (<lead>)") — "" leaves the Team line as the bare team
// name, same as the Chat card already does.
func RenderSLABreachTeamEmail(teamLeadName string, d SLABreachEmailData) string {
	tmpl := applyOptionalBlock(slaBreachTeamTemplate, "CASE_TITLE", d.CaseTitle)
	tmpl = applyOptionalBlock(tmpl, "CASE_TYPE", d.CaseType)
	tmpl = applyOptionalBlock(tmpl, "PRODUCT", d.Product)
	tmpl = applyOptionalBlock(tmpl, "TEAM", d.Team)
	tmpl = applyOptionalBlock(tmpl, "STATE", d.State)
	tmpl = applyOptionalBlock(tmpl, "OPENED_AT", d.OpenedAt)
	tmpl = applyOptionalBlock(tmpl, "INTENDED_FOR", d.IntendedFor)

	teamDisplay := d.Team
	if teamLeadName != "" {
		teamDisplay = d.Team + " (" + teamLeadName + ")"
	}
	priorityLabel, priorityColor := slaSeverityLabelAndColor(d.Severity)
	slaColor := slaBreachColor(d.Tier)

	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(d.IntendedFor),
		"<!-- [HEADER_TITLE] -->", escapeHTML(slaBreachHeaderTitle(d.ClockType, d.Tier, d.Severity, d.CaseNumber, d.WSO2CaseID)),
		"<!-- [CASE_TITLE] -->", escapeHTML(truncateSLACardTitle(d.CaseTitle)),
		"<!-- [CASE_NUMBER] -->", escapeHTML(d.CaseNumber),
		"<!-- [CASE_TYPE] -->", escapeHTML(d.CaseType),
		"<!-- [PRODUCT] -->", escapeHTML(d.Product),
		"<!-- [TEAM_DISPLAY] -->", escapeHTML(teamDisplay),
		"<!-- [PRIORITY_LABEL] -->", escapeHTML(priorityLabel),
		"<!-- [PRIORITY_COLOR] -->", priorityColor,
		"<!-- [STATE] -->", escapeHTML(d.State),
		"<!-- [OPENED_AT] -->", escapeHTML(d.OpenedAt),
		"<!-- [SLA_COLOR] -->", slaColor,
		"<!-- [TIER] -->", escapeHTML(d.Tier),
		"<!-- [CASE_LINK] -->", escapeHTML(d.CaseLink),
	)
	return replacer.Replace(tmpl)
}
