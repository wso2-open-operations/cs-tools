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
	"context"
	"fmt"
	"strings"
)

// slaSeverityDisplay/slaSeverityLabelAndColor duplicate internal/dispatch's
// own severityDisplay/severityLabelAndColor (dispatch.go) — a small,
// deliberate duplication rather than an import: dispatch already imports
// this package, so the reverse would create a cycle, and those two are
// unexported there anyway. Keep both copies in sync by hand if either
// changes — the same convention this codebase already uses for
// cross-service duplicated schemas (see e.g. internal/events' own package
// doc comment).
var slaSeverityDisplay = map[string]struct{ label, color string }{
	"CATASTROPHIC": {"Catastrophic (P0)", "#7F1D1D"},
	"CRITICAL":     {"Critical (P1)", "#DC2626"},
	"HIGH":         {"High (P2)", "#F97316"},
	"MEDIUM":       {"Medium (P3)", "#7C3AED"},
	"LOW":          {"Low (P4)", "#6B7280"},
}

// slaSeverityLabelAndColor resolves severity to its Chat display label/color
// — see dispatch.severityLabelAndColor's own doc comment for the same
// case/whitespace-insensitive matching and "Unknown"/gray fallback
// reasoning.
func slaSeverityLabelAndColor(severity string) (label, color string) {
	if d, ok := slaSeverityDisplay[strings.ToUpper(strings.TrimSpace(severity))]; ok {
		return d.label, d.color
	}
	label = strings.TrimSpace(severity)
	if label == "" {
		label = "Unknown"
	}
	return label, "#6B7280"
}

// slaClockTypeLabels maps the raw clock-type strings entity-service uses
// (see entity-service's internal/service/sla_policy.go — "response"/
// "workaround"/"resolution") to their Chat display label.
var slaClockTypeLabels = map[string]string{
	"response":   "Response",
	"workaround": "Workaround",
	"resolution": "Resolution",
}

func slaClockTypeLabel(clockType string) string {
	if label, ok := slaClockTypeLabels[clockType]; ok {
		return label
	}
	return clockType
}

// maxSLACardTitleLength/truncateSLACardTitle mirror dispatch's own
// maxChatTitleLength/truncateTitle (dispatch.go) — same reasoning as
// slaSeverityDisplay above for why this is a small duplicate, not an
// import.
const maxSLACardTitleLength = 140

func truncateSLACardTitle(title string) string {
	r := []rune(title)
	if len(r) <= maxSLACardTitleLength {
		return title
	}
	return string(r[:maxSLACardTitleLength]) + "..."
}

// slaBreachColor is the SLA-percentage line's color: red at 100% (a genuine
// breach), amber at 50/75% (still "at risk", not yet breached).
func slaBreachColor(tier string) string {
	if tier == "100" {
		return "#DC2626"
	}
	return "#EA580C"
}

// SendSLABreachAlert posts a card announcing that an SLA clock has crossed
// a tier (50%, 75%, or 100% elapsed), to the Google Chat space configured
// for product. Called directly from internal/slaengine.Engine, not routed
// through internal/dispatch (see that package's own CLAUDE.md note: there
// is no dispatch.go reaction for SLA events at all — slaengine owns
// sending this itself).
//
// Two things distinguish this card from every case.*/incident.* alert
// elsewhere in this file — both matching the reference mockups this was
// built from exactly:
//
//   - Explicit "<b>Label:</b> value" lines for every field, not the
//     minimal unlabeled style those cards use.
//   - A real button ("Open in CSM Portal") — the same chatButtonList/
//     chatButton shape SendIncidentAlert above already uses; simply not
//     something the case.*/incident.created cards happen to use
//     themselves.
//
// Header wording distinguishes a genuine breach from an early warning:
// "[<SEVERITY>] <ClockTypeLabel> SLA Violation - <caseRef>" at 100%
// elapsed (severity uppercase, unresolved — e.g. "CRITICAL", not the
// resolved "Critical (P1)" label the body's Priority line uses), versus
// "<ClockTypeLabel> SLA at Risk - <caseRef>" at 50/75%. No Subtitle: unlike
// the case.*/incident.* cards, this card's header is the whole heading on
// one line.
func (c *GoogleChatClient) SendSLABreachAlert(ctx context.Context, product, clockType, tier, caseNumber, wso2CaseID, caseTitle, caseType, productName, team, severity, state, openedAt, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	if tier == "" {
		return fmt.Errorf("notifications: tier is required")
	}

	clockLabel := slaClockTypeLabel(clockType)
	severityLabel, severityColor := slaSeverityLabelAndColor(severity)
	caseRef := chatHeaderCaseRef(caseNumber, wso2CaseID)

	var title string
	if tier == "100" {
		title = fmt.Sprintf("[%s] %s SLA Violation - %s", strings.ToUpper(strings.TrimSpace(severity)), clockLabel, caseRef)
	} else {
		title = fmt.Sprintf("%s SLA at Risk - %s", clockLabel, caseRef)
	}

	var lines []string
	if caseTitle != "" {
		lines = append(lines, caseAlertLine(`<b>Title :</b> %s`, truncateSLACardTitle(caseTitle)))
	}
	lines = append(lines, caseAlertLine(`<b>Case ID :</b> %s`, caseNumber))
	if caseType != "" {
		lines = append(lines, caseAlertLine(`<b>Type :</b> %s`, caseType))
	}
	if productName != "" {
		lines = append(lines, caseAlertLine(`<b>Product :</b> %s`, productName))
	}
	if team != "" {
		lines = append(lines, caseAlertLine(`<b>Team :</b> <b>%s</b>`, team))
	}
	lines = append(lines, caseAlertLine(`<b>Priority :</b> <font color="%s">%s</font>`, severityColor, severityLabel))
	if state != "" {
		lines = append(lines, caseAlertLine(`<b>State :</b> %s`, state))
	}
	if openedAt != "" {
		lines = append(lines, caseAlertLine(`<b>Opened At :</b> %s`, openedAt))
	}
	lines = append(lines, caseAlertLine(`<b>SLA Percentage :</b> <font color="%s"><b>%s%%</b></font>`, slaBreachColor(tier), tier))
	text := strings.Join(lines, "<br>")

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "sla-breach-alert",
				Card: chatCard{
					Header: &chatCardHeader{Title: title},
					Sections: []chatCardSection{
						{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: text}}}},
						{
							Widgets: []chatCardWidget{
								{
									ButtonList: &chatButtonList{
										Buttons: []chatButton{
											{
												Text:    "Open in CSM Portal",
												OnClick: chatOnClick{OpenLink: chatOpenLink{URL: caseLink}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return c.sendCard(ctx, product, msg)
}
