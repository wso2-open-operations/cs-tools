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

package dto

import (
	"strconv"
	"strings"
)

// This file mirrors, on the portal side, four numeric-id⇄string-enum
// mapping tables that live privately inside entity-service
// (internal/service/sn_case_service.go's snStateIDMap/snSeverityIDMap/
// snIssueTypeIDMap/snEngagementTypeIDMap and their label-parsing
// counterparts snCaseStateMap/snSeverityLabelMap/snIssueTypeToEnum). Those
// ids are ServiceNow's own choice-list keys — stable configuration, not
// something entity-service computes — so duplicating them here is safe, but
// if entity-service's copies ever change, this file must be updated too.
//
// The frontend (apps/customer-portal/webapp) was built against the old
// Ballerina backend, which forwarded these exact ServiceNow numeric ids
// directly. It still sends them today (POST /projects/{id}/cases/search's
// filters.statusIds/severityIds/issueIds/engagementTypeKeys) and still
// expects them back on read (CaseListItem's status/severity/issueType/
// engagementType: IdLabelRef, whose .id these tables populate) — even though
// cs-tools/entity-service's own contract is plain lowercase-snake-case
// domain enums (see case_filters.go). This file is the translation layer:
// caseXxxIDs (enum -> id) builds the .id half of an IdLabelRef from
// entity-service's response; caseXxxIDToEnum (id -> enum, the reverse) turns
// the frontend's numeric filter values back into entity-service's own enum
// vocabulary before they reach BuildEntitySearchCasesRequest.

// caseStateIDs mirrors entity-service's private snStateIDMap.
var caseStateIDs = map[string]string{
	"open":              "1",
	"work_in_progress":  "10",
	"awaiting_info":     "18",
	"waiting_on_wso2":   "1003",
	"reopened":          "1006",
	"solution_proposed": "6",
	"closed":            "3",
}

// caseSeverityIDs mirrors entity-service's private snSeverityIDMap.
var caseSeverityIDs = map[string]string{
	"catastrophic": "14",
	"critical":     "10",
	"high":         "11",
	"medium":       "12",
	"low":          "13",
}

// caseIssueTypeIDs mirrors entity-service's private snIssueTypeIDMap.
var caseIssueTypeIDs = map[string]string{
	"total_outage":            "1",
	"partial_outage":          "2",
	"performance_degradation": "3",
	"question":                "4",
	"security_or_compliance":  "5",
	"error":                   "6",
}

// caseEngagementTypeIDs mirrors entity-service's private snEngagementTypeIDMap.
var caseEngagementTypeIDs = map[string]string{
	"migration":               "1",
	"consultancy":             "2",
	"new_feature_improvement": "3",
	"follow_up":               "4",
	"onboarding":              "5",
}

func reverseStringMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

var (
	caseStateIDToEnum          = reverseStringMap(caseStateIDs)
	caseSeverityIDToEnum       = reverseStringMap(caseSeverityIDs)
	caseIssueTypeIDToEnum      = reverseStringMap(caseIssueTypeIDs)
	caseEngagementTypeIDToEnum = reverseStringMap(caseEngagementTypeIDs)
)

// caseStateLabelWords mirrors entity-service's private snCaseStateMap: the
// full (lowercased) ServiceNow state label, not a scanned word — state
// labels are short and consistent ("Open", "Work In Progress"), unlike
// severity labels.
var caseStateLabelWords = map[string]string{
	"open":              "open",
	"work in progress":  "work_in_progress",
	"waiting on wso2":   "waiting_on_wso2",
	"awaiting info":     "awaiting_info",
	"reopened":          "reopened",
	"solution proposed": "solution_proposed",
	"closed":            "closed",
}

// caseSeverityLabelWords mirrors entity-service's private snSeverityLabelMap:
// a single priority word scanned out of the full label, since severity
// labels vary in format ("Low (P4)", "2 - High", "3 - Moderate").
var caseSeverityLabelWords = map[string]string{
	"catastrophic": "catastrophic",
	"critical":     "critical",
	"high":         "high",
	"moderate":     "medium",
	"medium":       "medium",
	"low":          "low",
}

// caseIDsToEnums converts the frontend's numeric filter ids (statusIds,
// severityIds, issueIds, engagementTypeKeys) to entity-service's own enum
// vocabulary via idToEnum, silently skipping any id with no known mapping —
// consistent with entity-service's own domainStatesToSNIDs/
// domainSeveritiesToSNIDs/domainIssueTypesToSNIDs, which do the same in the
// opposite direction.
func caseIDsToEnums(ids []int, idToEnum map[string]string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if enum, ok := idToEnum[strconv.Itoa(id)]; ok {
			out = append(out, enum)
		}
	}
	return out
}

// IDLabelRef is a compact {id, label} reference, matching the frontend's
// shared IdLabelRef type (apps/customer-portal/webapp/src/types/common.ts) —
// used only by the case-search response, where entity-service's enum-valued
// fields (state, severity, issueType, engagementType, type) must round-trip
// as an id/label pair instead of the plain string entity-service itself
// returns. Every other dto in this package keeps using Ref{id, name} for
// entity references; don't reuse this type there.
type IDLabelRef struct {
	ID    string `json:"id,omitempty"`
	Label string `json:"label"`
}

// caseStatusRef builds the {id, label} the frontend's CaseListItem.status
// expects from entity-service's plain State label string. entity-service's
// ServiceNow-backed case search returns the raw SN label directly (e.g.
// "Work In Progress"), not a normalized enum, so label is used verbatim; id
// is resolved by matching the full lowercased label against
// caseStateLabelWords. Falls back to a label-only ref (empty id) for a label
// this table doesn't recognize, rather than dropping the field entirely —
// the frontend still renders unrecognized labels, just without a usable id.
// caseStateDisplayLabels turns entity-service's domain enum into the label the
// frontend renders verbatim.
//
// entity-service deliberately returns UPPER/lower_snake_case domain enums rather
// than raw ServiceNow labels — its CLAUDE.md forbids the latter outright — while
// the Ballerina backend forwarded SN's own display text. So the portal showed
// "work_in_progress" where it used to show "Work In Progress". Translating here
// keeps entity-service's contract intact and restores what the frontend expects;
// values match CaseStatus in features/support/constants/supportConstants.ts.
var caseStateDisplayLabels = map[string]string{
	"open":              "Open",
	"work_in_progress":  "Work In Progress",
	"awaiting_info":     "Awaiting Info",
	"waiting_on_wso2":   "Waiting On WSO2",
	"reopened":          "Reopened",
	"solution_proposed": "Solution Proposed",
	"closed":            "Closed",
}

// caseSeverityDisplayLabels turns the severity enum into the SN-style label the
// frontend maps to its S0–S4 display names.
//
// The frontend keys SEVERITY_LABEL_TO_DISPLAY (features/dashboard/constants/dashboard.ts)
// on exactly these strings — "Low (P4)" becomes "S4(Query)" — so the enum alone
// misses the lookup and renders raw. These also match acceptedSeverityValues in
// GET /projects/{id}/features, which is the same vocabulary.
var caseSeverityDisplayLabels = map[string]string{
	"catastrophic": "Catastrophic (P0)",
	"critical":     "Critical (P1)",
	"high":         "High (P2)",
	"medium":       "Medium (P3)",
	"low":          "Low (P4)",
}

// displayLabelOr returns the mapped display label for enum, falling back to the
// value as received. Falling back rather than blanking means an enum this table
// does not know still renders something, the same tolerance the *Ref helpers use
// for unrecognised labels.
func displayLabelOr(table map[string]string, value string) string {
	if label, ok := table[strings.ToLower(strings.TrimSpace(value))]; ok {
		return label
	}
	return value
}

// caseStateLookupKey normalises either representation of a case state — the
// domain enum ("work_in_progress") or ServiceNow's display text
// ("Work In Progress") — to caseStateLabelWords' space-separated key form.
func caseStateLookupKey(label string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(label)), "_", " ")
}

// caseStateClosed is entity-service's domain enum for a closed case — the key
// caseStateIDs, caseStateLabelWords, and caseStateDisplayLabels all agree on.
const caseStateClosed = "closed"

// IsCaseStateClosed reports whether a case state means "closed", accepting
// every representation this one field travels as: entity-service's domain enum
// ("closed"), ServiceNow's display text ("Closed"), and the numeric SN
// choice-list id the frontend still speaks ("3"). Handlers pass
// entity.CaseView.State, which carries one of the first two depending on the
// active data source (see SearchCaseView's doc comment); the id form is
// accepted so a caller holding a portal-facing status id resolves the same way.
//
// Exported because the case write guards live in internal/handler but
// the state vocabulary they need lives here, alongside the tables it is keyed
// on — a handler must never re-hardcode "closed"/"3" itself.
func IsCaseStateClosed(state string) bool {
	trimmed := strings.TrimSpace(state)
	if trimmed == "" {
		return false
	}
	if caseStateLabelWords[caseStateLookupKey(trimmed)] == caseStateClosed {
		return true
	}
	return caseStateIDToEnum[trimmed] == caseStateClosed
}

func caseStatusRef(label string) *IDLabelRef {
	if label == "" {
		return nil
	}
	// caseStateLabelWords is keyed on ServiceNow's space-separated display text
	// ("work in progress"), but cs-tools/entity-service emits the underscore
	// domain enum ("work_in_progress") — so the lookup missed on every
	// multi-word state and returned an empty id. Normalising underscores to
	// spaces lets the one table match both forms.
	if enum, ok := caseStateLabelWords[caseStateLookupKey(label)]; ok {
		return &IDLabelRef{ID: caseStateIDs[enum], Label: displayLabelOr(caseStateDisplayLabels, enum)}
	}
	return &IDLabelRef{Label: displayLabelOr(caseStateDisplayLabels, label)}
}

// caseSeverityRef mirrors caseStatusRef for severity, scanning label words
// (see caseSeverityLabelWords) rather than matching the full label, since
// severity label formats vary.
func caseSeverityRef(label *string) *IDLabelRef {
	if label == nil || *label == "" {
		return nil
	}
	for _, word := range strings.Fields(*label) {
		w := strings.ToLower(strings.Trim(word, "(),"))
		if enum, ok := caseSeverityLabelWords[w]; ok {
			return &IDLabelRef{ID: caseSeverityIDs[enum], Label: displayLabelOr(caseSeverityDisplayLabels, enum)}
		}
	}
	return &IDLabelRef{Label: displayLabelOr(caseSeverityDisplayLabels, *label)}
}

// caseIssueTypeRef mirrors caseStatusRef for issue type, matching
// entity-service's own snIssueTypeToEnum transform (lowercase, spaces to
// underscores) rather than a lookup table of raw labels.
func caseIssueTypeRef(label *string) *IDLabelRef {
	if label == nil || *label == "" {
		return nil
	}
	enum := strings.ToLower(strings.ReplaceAll(*label, " ", "_"))
	if id, ok := caseIssueTypeIDs[enum]; ok {
		return &IDLabelRef{ID: id, Label: *label}
	}
	return &IDLabelRef{Label: *label}
}

// caseEngagementTypeRef mirrors caseStatusRef for engagement type.
// entity-service has no equivalent label-parsing helper for this field (its
// SN search response passes the raw label straight through, unparsed) — this
// transform (lowercase, spaces and slashes to underscores) is this backend's
// own best-effort match against caseEngagementTypeIDs's domain.EngagementType
// keys, not a mirror of an existing entity-service function.
func caseEngagementTypeRef(label *string) *IDLabelRef {
	if label == nil || *label == "" {
		return nil
	}
	enum := strings.ToLower(strings.NewReplacer(" ", "_", "/", "_").Replace(*label))
	if id, ok := caseEngagementTypeIDs[enum]; ok {
		return &IDLabelRef{ID: id, Label: *label}
	}
	return &IDLabelRef{Label: *label}
}

// caseTypeLabels supplies portal-facing display text for entity-service's
// case type domain values — these never come from ServiceNow as a label at
// all (Type is entity-service's own domain string, e.g. "case",
// "service_request"), so there's nothing to parse; this is just this
// backend's own presentation text.
var caseTypeLabels = map[string]string{
	"case":                     "Case",
	"service_request":          "Service Request",
	"security_report_analysis": "Security Report Analysis",
	"engagement":               "Engagement",
	"announcement":             "Announcement",
}

// caseTypeRef builds the {id, label} the frontend's CaseListItem.type/
// caseTypes expect. id is entity-service's Type value, EXCEPT "case" is
// translated to "default_case" — entity-service's own canonical value ties
// to its Postgres case_type_enum (see case_service.go's caseTypeAliases doc
// comment), but the frontend's CaseType.DEFAULT_CASE constant (and code that
// compares type.id against it directly, e.g. support.ts's isSecurityReportCase-
// style helpers) was built against ServiceNow's raw "default_case" wire
// value and has no knowledge of "case" at all.
func caseTypeRef(caseType string) *IDLabelRef {
	if caseType == "" {
		return nil
	}
	id := caseType
	if id == "case" {
		id = "default_case"
	}
	label := caseTypeLabels[caseType]
	if label == "" {
		label = caseType
	}
	return &IDLabelRef{ID: id, Label: label}
}

// caseEscalationLevelIDs maps entity-service's escalation-level id to the label
// the portal displays. Mirrors validEscalationLevel in entity-service's
// sn_case_service.go, which accepts exactly "0".."5".
//
// entity-service returns the id alone, per its convention of never emitting
// {id, label} objects; the frontend reads escalationLevel.id and renders the
// label, so the pair is assembled here — the same split already used for case
// status and severity.
var caseEscalationLevelIDs = map[string]string{
	"0": "EL0",
	"1": "EL1",
	"2": "EL2",
	"3": "EL3",
	"4": "EL4",
	"5": "EL5",
}

// caseEscalationLevelRef builds the {id, label} escalation-level reference.
//
// An unrecognised id passes through as its own label rather than blanking the
// field, the same tolerance caseStatusRef and caseSeverityRef apply.
func caseEscalationLevelRef(id *string) *IDLabelRef {
	if id == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*id)
	if trimmed == "" {
		return nil
	}
	label, ok := caseEscalationLevelIDs[trimmed]
	if !ok {
		label = trimmed
	}
	return &IDLabelRef{ID: trimmed, Label: label}
}
