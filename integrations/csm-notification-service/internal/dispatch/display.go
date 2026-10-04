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

package dispatch

// Display helpers shared by the handlers: case references, subjects, severity and case-type labels.

import (
	"fmt"
	"net/url"
	"strings"
)

// commentLinkFor appends the comment permalink fragment to a resolved case
// link. Fragments are client-side only, so the same suffix works regardless
// of which portal's URL shape caseLink has — see recipientlinks' package doc
// for why the customer portal simply ignores it today rather than erroring.
// An empty commentID (every case.* type except case.comment_added, which
// has no comment to link to) yields the bare case link.
func commentLinkFor(caseLink, commentID string) string {
	if commentID == "" {
		return caseLink
	}
	return caseLink + "#" + url.PathEscape(commentID)
}

// severityDisplay maps a case's raw severity (as entity-service sends it —
// see events.CaseCreatedPayload.Priority's own doc comment, an uppercase
// string like "CRITICAL") to the label/color the case.created and
// case.acknowledged Chat cards use — matching an existing internal
// WSO2-support Chat format: S0-catastrophic, S1-critical, S2-high,
// S3-medium. LOW (S4) is this service's own extrapolation to keep the map
// total over every domain.CaseSeverity value — the reference format never
// showed one, since low-severity cases don't usually get a Chat alert in
// practice.
var severityDisplay = map[string]struct{ label, color string }{
	"CATASTROPHIC": {"Catastrophic (P0)", "#7F1D1D"},
	"CRITICAL":     {"Critical (P1)", "#DC2626"},
	// HIGH's color is deliberately a distinctly orange hue (not a
	// red-leaning orange like Tailwind's orange-600, #EA580C, used
	// originally) — that read too close to CRITICAL's red at a glance in
	// a real Chat card, a live-testing correction.
	"HIGH":   {"High (P2)", "#F97316"},
	"MEDIUM": {"Medium (P3)", "#7C3AED"},
	"LOW":    {"Low (P4)", "#6B7280"},
}

// nonCaseCaseTypes are entity-service's own case.created CaseType values
// (strings.ToUpper(req.Type)) for the four types that never carry a
// severity — engagement/service_request/security_report_analysis/
// announcement, see that service's own CLAUDE.md — for which
// handleCaseCreated skips the Google Chat alert and notifies by email only.
var nonCaseCaseTypes = map[string]bool{
	"ENGAGEMENT":               true,
	"SERVICE_REQUEST":          true,
	"SECURITY_REPORT_ANALYSIS": true,
	"ANNOUNCEMENT":             true,
}

// isNonCaseCaseType reports whether caseType is one of the four types Chat
// is skipped for. See handleCaseCreated's own call site comment for why
// this is an exclude-list, not an include-list.
func isNonCaseCaseType(caseType string) bool {
	return nonCaseCaseTypes[caseType]
}

// isLowSeverity reports whether severity is entity-service's LOW/S4 value
// (case/whitespace-insensitive) — handleCaseCreated's own gate for skipping
// its Google Chat alert on a LOW-severity "case".
func isLowSeverity(severity string) bool {
	return strings.EqualFold(strings.TrimSpace(severity), "LOW")
}

// severityLabelAndColor resolves severity to its Chat display label/color
// (case/whitespace-insensitive), falling back to the raw (trimmed) value
// itself in a neutral gray for a severity this service doesn't recognize —
// or, when severity is blank (it's an optional field on both
// CaseCreatedPayload.Priority and CaseAcknowledgedPayload.Severity), to
// "Unknown" — never blank, so an absent or unrecognized value still
// renders something readable instead of an empty line.
func severityLabelAndColor(severity string) (label, color string) {
	if d, ok := severityDisplay[strings.ToUpper(strings.TrimSpace(severity))]; ok {
		return d.label, d.color
	}
	label = strings.TrimSpace(severity)
	if label == "" {
		label = "Unknown"
	}
	return label, "#6B7280"
}

// emailSeverityLabels maps entity-service's raw uppercase severity value
// (e.g. "HIGH", as sent on CaseCreatedPayload.Priority/SeverityChangedPayload.
// OldSeverity/NewSeverity) to the title-case "<Label>(S<n>)" format shown in
// case.created/case.severity_changed emails — S0..S4 matching entity-service's
// own case_severity_enum labels (CATASTROPHIC=S0 .. LOW=S4, see that
// service's own CLAUDE.md), not the P0..P4 notation severityLabelAndColor
// above uses for Google Chat cards. Deliberately a separate, email-specific
// convention per explicit request — not meant to be reconciled with Chat's
// own labels.
var emailSeverityLabels = map[string]string{
	"CATASTROPHIC": "Catastrophic(S0)",
	"CRITICAL":     "Critical(S1)",
	"HIGH":         "High(S2)",
	"MEDIUM":       "Medium(S3)",
	"LOW":          "Low(S4)",
}

// emailSeverityLabel resolves severity to its email display label
// (case/whitespace-insensitive), falling back to the raw trimmed value for
// anything unrecognized — including blank, which stays blank so an absent
// Priority still renders as an empty field rather than a fabricated label.
func emailSeverityLabel(severity string) string {
	if label, ok := emailSeverityLabels[strings.ToUpper(strings.TrimSpace(severity))]; ok {
		return label
	}
	return strings.TrimSpace(severity)
}

// caseTypeLabels maps entity-service's raw uppercase CaseType value (e.g.
// "SECURITY_REPORT_ANALYSIS", as sent on CaseCreatedPayload.CaseType — see
// that service's own strings.ToUpper(req.Type)) to the title-case wording
// shown in the case-created email's "Case Type" row — same "don't show raw
// enum casing to a reader" reasoning as emailSeverityLabels above.
var caseTypeLabels = map[string]string{
	"CASE":                     "Case",
	"ENGAGEMENT":               "Engagement",
	"SERVICE_REQUEST":          "Service Request",
	"SECURITY_REPORT_ANALYSIS": "Security Report Analysis",
	"ANNOUNCEMENT":             "Announcement",
}

// emailCaseTypeLabel resolves caseType to its email display label
// (case/whitespace-insensitive) via caseTypeLabels, falling back to the raw
// trimmed value for anything unrecognized rather than blanking it out.
func emailCaseTypeLabel(caseType string) string {
	if label, ok := caseTypeLabels[strings.ToUpper(strings.TrimSpace(caseType))]; ok {
		return label
	}
	return strings.TrimSpace(caseType)
}

// maxChatTitleLength bounds truncateTitle's output — long enough to still
// be informative in a Chat card, short enough that a card doesn't dominate
// the space with one case's title.
const maxChatTitleLength = 140

// truncateTitle shortens title to at most max runes, appending "..." when
// it had to cut — rune-based (not byte-based) so a multi-byte character
// never gets split mid-encoding.
func truncateTitle(title string, max int) string {
	r := []rune(title)
	if len(r) <= max {
		return title
	}
	return string(r[:max]) + "..."
}

// displayCaseRef returns caseNumber (the case's human-readable reference,
// e.g. "CS0023001") when the publisher supplied one, falling back to
// caseID (a UUID, meaningless to an end user) only so a subject/body line
// is never blank while every publisher is on a version of the schema that
// carries CaseNumber — see CaseCreatedPayload.CaseNumber's own doc comment.
func displayCaseRef(caseNumber, caseID string) string {
	if caseNumber != "" {
		return caseNumber
	}
	return caseID
}

// displayInternalRef returns wso2CaseID (the CSM portal's own case
// identifier, e.g. "WSO2-1000" — the backing data source's internal case-id field,
// see events.CaseCreatedPayload.WSO2CaseID's own doc comment) when the
// publisher supplied one, falling back to caseID (the raw UUID, meaningless
// to an end user) only so the subject is never blank while a publisher
// hasn't been updated to send it yet.
func displayInternalRef(wso2CaseID, caseID string) string {
	if wso2CaseID != "" {
		return wso2CaseID
	}
	return caseID
}

// subjectLine builds every case.* email's subject in this service's one
// standard format: "[WSO2 Support] (<wso2 case id>/<case number>) <title>" —
// matching the CSM portal frontend's own "wso2CaseId / caseNumber" pairing
// (see caseIdentity.ts's caseIdLabel). The first slot is
// displayInternalRef(wso2CaseID, caseID) (falls back to the raw UUID only if
// a publisher hasn't sent WSO2CaseID yet), the second is
// displayCaseRef(caseNumber, caseID) (same fallback reasoning for
// CaseNumber). title is empty for a publisher that hasn't been updated to
// send CaseTitle yet (case.status_changed/case.assigned did not originally
// carry one) — still a valid, if less descriptive, subject rather than a
// missing one.
func subjectLine(wso2CaseID, caseNumber, caseID, title string) string {
	return fmt.Sprintf("[WSO2 Support] (%s/%s) %s", displayInternalRef(wso2CaseID, caseID), displayCaseRef(caseNumber, caseID), title)
}
