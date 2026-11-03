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

package service

import "strings"

// translateIncidentSLACondition is sn_sla_condition_translate.go's
// translateSLACondition, for an INCIDENT-targeting sla_policy row instead of
// a case-targeting one. It is a deliberately SEPARATE function, not a branch
// inside translateSLACondition, because case and incident policies share raw
// field NAMES ("priority", "state") that mean completely different things in
// completely different numeric vocabularies on the external sync job's own
// wire format:
//
//   - case state code 3 = CLOSED; incident state code 3 = ON_HOLD.
//   - case state code 6 = SOLUTION_PROPOSED; incident state code 6 = RESOLVED.
//   - case has no raw priority codes 1/2 at all (its own codes are 10-14);
//     incident priority code 1 = CRITICAL, 2 = HIGH.
//
// Running an incident policy's condition text through translateSLACondition
// (or vice versa) would silently translate "state"/"priority" using the
// WRONG vocabulary -- e.g. the real "Cloud Incident - High Priority" row's
// stop_condition "stateIN3,6,7" would come out as "stateINCLOSED,SOLUTION_PROPOSED,7"
// under the case translator, which is wrong on every value and not even a
// real incident state label. Keeping the two translators fully separate,
// each with its own vocabulary, is what makes this collision impossible by
// construction rather than by careful branching.
//
// Like translateSLACondition, this runs at evaluation time on every read
// (not a one-time migration) for the same reason: the external sync job
// re-writes sla_policy's raw condition text on every delta sync/backfill, so
// a one-off rewrite of the stored text would be silently reverted.
func translateIncidentSLACondition(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	tokens := strings.Split(raw, "^")
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		prefix := ""
		rest := tok
		switch {
		case strings.HasPrefix(tok, "NQ"):
			prefix, rest = "NQ", tok[2:]
		case strings.HasPrefix(tok, "OR"):
			prefix, rest = "OR", tok[2:]
		}
		rest = strings.TrimSpace(rest)
		if rest == "" || rest == "EQ" {
			// Same stray-separator/end-of-query-terminator skip as
			// translateSLACondition -- "EQ" is the external sync job's own
			// terminator regardless of which entity type the policy targets.
			continue
		}
		out = append(out, prefix+translateIncidentSLAClauseToken(rest))
	}
	return strings.Join(out, "^")
}

// incidentSLAConditionFields names the raw field spellings this translator
// recognizes -- "business_service"/"category" are never looked up here (no
// raw-to-neutral rename and no value translation: the external sync job's
// own sys_id/lowercase-label spellings already match service.IncidentFacts'
// own Field()/Present() output once internal/slacondition's case-insensitive
// comparison is applied), only "priority"/"state" need an actual value
// translation step.
var incidentSLAConditionFields = []string{"priority", "state"}

// matchIncidentSLAConditionField returns the longest entry of
// incidentSLAConditionFields that prefixes token, mirroring
// matchSLAConditionField's longest-prefix convention.
func matchIncidentSLAConditionField(token string) (field, rest string, ok bool) {
	best := ""
	for _, f := range incidentSLAConditionFields {
		if strings.HasPrefix(token, f) && len(f) > len(best) {
			best = f
		}
	}
	if best == "" {
		return "", token, false
	}
	return best, token[len(best):], true
}

// translateIncidentSLAClauseToken translates one clause (already stripped of
// its "^OR"/"^NQ" prefix). A field outside incidentSLAConditionFields
// (business_service, category, or anything else) is returned completely
// unchanged -- exactly matchSLAConditionField's own "not in this engine's
// vocabulary" fallback, and correct here for the same reason: the raw text
// already matches what IncidentFacts exposes, case-insensitively.
func translateIncidentSLAClauseToken(token string) string {
	field, rest, ok := matchIncidentSLAConditionField(token)
	if !ok {
		return token
	}
	switch {
	case strings.HasPrefix(rest, "ISNOTEMPTY"):
		return field + "ISNOTEMPTY"
	case strings.HasPrefix(rest, "ISEMPTY"):
		return field + "ISEMPTY"
	case strings.HasPrefix(rest, "!="):
		return field + "!=" + translateIncidentSLAConditionValue(field, rest[len("!="):])
	case strings.HasPrefix(rest, "NOT IN"):
		return field + "NOT IN" + translateIncidentSLAConditionValueList(field, rest[len("NOT IN"):])
	case strings.HasPrefix(rest, "IN"):
		return field + "IN" + translateIncidentSLAConditionValueList(field, rest[len("IN"):])
	case strings.HasPrefix(rest, "="):
		return field + "=" + translateIncidentSLAConditionValue(field, rest[len("="):])
	default:
		return token
	}
}

// translateIncidentSLAConditionValue translates one raw ServiceNow numeric
// code to this engine's own incident label vocabulary. A value with no
// known translation (should not happen for the real, confirmed domain of
// both codes below) is left exactly as it was -- same fail-closed fallback
// as translateSLAConditionValue: a raw code can never equal a real decoded
// IncidentFacts value, so the clause just never matches, rather than this
// function guessing or erroring.
func translateIncidentSLAConditionValue(field, value string) string {
	value = strings.TrimSpace(value)
	switch field {
	case "priority":
		if label, ok := slaIncidentPriorityCodes[value]; ok {
			return label
		}
	case "state":
		if label, ok := slaIncidentStateCodes[value]; ok {
			return label
		}
	}
	return value
}

// translateIncidentSLAConditionValueList applies
// translateIncidentSLAConditionValue to every comma-separated member of an
// IN/NOT IN clause's value list.
func translateIncidentSLAConditionValueList(field, raw string) string {
	parts := strings.Split(raw, ",")
	translated := make([]string, len(parts))
	for i, p := range parts {
		translated[i] = translateIncidentSLAConditionValue(field, p)
	}
	return strings.Join(translated, ",")
}

// slaIncidentPriorityCodes maps the external sync job's own numeric incident
// priority choice-list values to this database's incident_priority_enum
// labels. Confirmed against the one real incident sla_policy row
// ("Cloud Incident - High Priority", priorityIN1,2 = "High Priority" =
// Critical+High) and ServiceNow's standard incident priority numbering.
var slaIncidentPriorityCodes = map[string]string{
	"1": "CRITICAL",
	"2": "HIGH",
	"3": "MODERATE",
	"4": "LOW",
	"5": "PLANNING",
}

// slaIncidentStateCodes maps the external sync job's own numeric incident
// state choice-list values to this database's incident_state_enum labels --
// a COMPLETELY DIFFERENT numbering than slaCaseStateCodes' own case-state
// codes despite sharing the raw field name "state" (see this file's own doc
// comment). Confirmed against domain.go's own SearchIncidentsFilters
// "incidentStateKeys" doc comment: 1 New, 2 In Progress, 3 On Hold,
// 6 Resolved, 7 Closed, 8 Canceled. Spelled "CANCELED" (one L), matching the
// real stored incident_state_enum label (migration 0058) -- NOT
// domain.IncidentStateCancelled's own Go constant, which is misspelled
// "CANCELLED" (two L's) relative to the actual enum.
var slaIncidentStateCodes = map[string]string{
	"1": "NEW",
	"2": "IN_PROGRESS",
	"3": "ON_HOLD",
	"6": "RESOLVED",
	"7": "CLOSED",
	"8": "CANCELED",
}
