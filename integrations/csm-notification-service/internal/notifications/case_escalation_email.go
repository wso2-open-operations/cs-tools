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
	"strconv"
	"strings"
	"time"
)

// The case escalation email ports ServiceNow's "Internal Escalation
// notification" flow (discovery scripts 79/80/82): its subject, its wording
// and both of its tables, row for row, in this service's own email shell
// (header line, white card, WSO2 logo footer) like every other email here.
// SN's "Acknowledge Escalation" button is a "View Case" link here: SN's only
// opened the case too (its escalation table has no acknowledgement field and
// its API no acknowledge operation), so the label no longer promises an
// action nothing records.

//go:embed templates/case_escalated.html
var caseEscalatedTemplateRaw string

var caseEscalatedTemplate = bakeLogo(caseEscalatedTemplateRaw)

// escalationPriorityLabels is ServiceNow's case priority display value for
// each severity, as its email's "Priority" row shows it ("Medium (P3)").
var escalationPriorityLabels = map[string]string{
	"CATASTROPHIC": "Catastrophic (P0)",
	"CRITICAL":     "Critical (P1)",
	"HIGH":         "High (P2)",
	"MEDIUM":       "Medium (P3)",
	"LOW":          "Low (P4)",
}

// escalationTimeZone is the zone ServiceNow renders "Escalation Time" in
// (its system zone, Asia/Colombo: UTC+05:30 all year, no DST). A fixed zone,
// so the container needs no tzdata.
var escalationTimeZone = time.FixedZone("Asia/Colombo", 5*3600+30*60)

// EscalationLevelName is "EL<n>", ServiceNow's ESCALATION_LEVEL_LABEL.
func EscalationLevelName(level int) string {
	return "EL" + strconv.Itoa(level)
}

// CaseEscalatedSubject is ServiceNow's subject, "Case Escalation To EL3".
func CaseEscalatedSubject(level int) string {
	return "Case Escalation To " + EscalationLevelName(level)
}

// CaseEscalatedEmailData is what RenderCaseEscalatedEmail substitutes.
type CaseEscalatedEmailData struct {
	CaseNumber    string
	CaseTitle     string
	ActorEmail    string
	AccountName   string
	PreviousLevel int
	CurrentLevel  int
	// EscalatedOn is RFC3339; shown as SN does, "2026-10-08 14:33:17" in
	// Asia/Colombo. Unparseable is shown as given.
	EscalatedOn string
	Reason      string
	Product     string
	// Severity is the raw uppercase severity ("MEDIUM").
	Severity              string
	Environment           string
	AssignedEngineerEmail string
	Link                  string
	// IntendedFor, when non-empty, shows a "Sent to: <value>" row — see
	// RenderCommentAddedEmail's own doc comment for the convention.
	IntendedFor string
}

// RenderCaseEscalatedEmail fills in the case escalation template: the two
// tables of ServiceNow's email, row for row (discovery script 82). SN prints
// an empty value as an empty cell; so does this.
func RenderCaseEscalatedEmail(d CaseEscalatedEmailData) string {
	row := func(b *strings.Builder, label, value string) {
		b.WriteString(`<tr><td style="color:#71717a; width:150px; padding:2px 0; vertical-align:top;">` + escapeHTML(label) +
			`</td><td style="padding:2px 0;">` + escapeHTML(value) + `</td></tr>`)
	}
	escalatedOn := d.EscalatedOn
	if t, err := time.Parse(time.RFC3339, d.EscalatedOn); err == nil {
		escalatedOn = t.In(escalationTimeZone).Format("2006-01-02 15:04:05")
	}
	priority := escalationPriorityLabels[strings.ToUpper(strings.TrimSpace(d.Severity))]
	if priority == "" {
		priority = d.Severity
	}

	var details, summary strings.Builder
	row(&details, "Case ID", d.CaseNumber)
	row(&details, "Escalated By", d.ActorEmail)
	row(&details, "Customer Account", d.AccountName)
	row(&details, "Escalation Path", EscalationLevelName(d.PreviousLevel)+" -> "+EscalationLevelName(d.CurrentLevel))
	row(&details, "Escalation Time", escalatedOn)
	row(&details, "Escalation Reason", d.Reason)
	row(&summary, "Title", d.CaseTitle)
	row(&summary, "Product", d.Product)
	row(&summary, "Priority", priority)
	row(&summary, "Environment", d.Environment)
	row(&summary, "Assigned Engineer", d.AssignedEngineerEmail)

	actor := d.ActorEmail
	if actor == "" {
		actor = "Someone"
	}
	tmpl := applyOptionalBlock(caseEscalatedTemplate, "INTENDED_FOR", d.IntendedFor)
	return strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(d.IntendedFor),
		"<!-- [ACTOR] -->", escapeHTML(actor),
		"<!-- [CASE_NUMBER] -->", escapeHTML(d.CaseNumber),
		"<!-- [PREVIOUS_LEVEL] -->", EscalationLevelName(d.PreviousLevel),
		"<!-- [LEVEL] -->", EscalationLevelName(d.CurrentLevel),
		"<!-- [CASE_LINK] -->", escapeHTML(d.Link),
		"<!-- [DETAIL_ROWS] -->", details.String(),
		"<!-- [SUMMARY_ROWS] -->", summary.String(),
	).Replace(tmpl)
}
