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

import "strconv"

// crStateIDs mirrors entity-service's private snCRStateIDMap
// (internal/service/sn_change_request_service.go) — ServiceNow's own
// numeric choice-list key for each change-request state. Unlike case
// search, entity-service's change-request search response already
// normalizes State/Impact to these exact domain enum strings (see
// snCRStateLabelToString/snCRImpactLabelToString), so no label-word-parsing
// is needed here — just a direct enum lookup, both directions.
//
// Authorize ("-3") is in the vocabulary on purpose: a change request a customer
// was asked about goes back to Authorize when the customer proposes a new
// implementation time (it is re-approved internally before the customer is asked
// again), and it stays visible to that customer while it is there. New and
// Assess are not: no change request that is visible to a customer is ever in
// either (it left New when approval was requested and a designated one never
// returns to Assess), so they have no id or label to show.
var crStateIDs = map[string]string{
	"authorize":         "-3",
	"customer_approval": "5",
	"scheduled":         "-2",
	"implement":         "-1",
	"review":            "0",
	"customer_review":   "1",
	"rollback":          "2",
	"closed":            "3",
	"canceled":          "4",
}

// crImpactIDs mirrors entity-service's private snCRImpactIDMap.
var crImpactIDs = map[string]string{
	"high":   "1",
	"medium": "2",
	"low":    "3",
}

// crStateFilterOnlyIDs are ServiceNow ids a search may NAME although no response
// ever carries them (see crStateIDs). A search for state New or Assess is a
// well-formed question whose answer is "none": entity-service decides what the
// caller may see, so it is asked, rather than the id being dropped here (a
// dropped id would turn "only New" into "no state filter", every visible change
// request).
var crStateFilterOnlyIDs = map[string]string{
	"-5": "new",
	"-4": "assess",
}

var (
	crStateIDToEnum  = withEntries(reverseStringMap(crStateIDs), crStateFilterOnlyIDs)
	crImpactIDToEnum = reverseStringMap(crImpactIDs)
)

func withEntries(base, extra map[string]string) map[string]string {
	for k, v := range extra {
		base[k] = v
	}
	return base
}

// crStateLabels/crImpactLabels supply portal-facing display text for these
// enum values — entity-service's change-request search response carries the
// domain enum string only (not ServiceNow's own display label, unlike case
// search's SN-backed path), so this is this backend's own presentation
// text, not a mirror of anything entity-service or ServiceNow provides.
var crStateLabels = map[string]string{
	"authorize":         "Authorize",
	"customer_approval": "Customer Approval",
	"scheduled":         "Scheduled",
	"implement":         "Implement",
	"review":            "Review",
	"customer_review":   "Customer Review",
	"rollback":          "Rollback",
	"closed":            "Closed",
	"canceled":          "Canceled",
}

var crImpactLabels = map[string]string{
	"high":   "High",
	"medium": "Medium",
	"low":    "Low",
}

// crIDsToEnums converts the frontend's numeric filter ids (stateKeys,
// impactKeys) to entity-service's own enum vocabulary, silently skipping
// any id with no known mapping.
func crIDsToEnums(ids []int, idToEnum map[string]string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if enum, ok := idToEnum[strconv.Itoa(id)]; ok {
			out = append(out, enum)
		}
	}
	return out
}

// crStateRef builds the {id, label} the frontend's ChangeRequestSummary.state
// expects from entity-service's already-normalized State enum string.
func crStateRef(state *string) *IDLabelRef {
	if state == nil || *state == "" {
		return nil
	}
	label := crStateLabels[*state]
	if label == "" {
		label = *state
	}
	return &IDLabelRef{ID: crStateIDs[*state], Label: label}
}

// crImpactRef mirrors crStateRef for impact.
func crImpactRef(impact *string) *IDLabelRef {
	if impact == nil || *impact == "" {
		return nil
	}
	label := crImpactLabels[*impact]
	if label == "" {
		label = *impact
	}
	return &IDLabelRef{ID: crImpactIDs[*impact], Label: label}
}

// normalizeChangeRequestStateChoices is normalizeCaseSeverityChoices for
// change-request states (see that function's own doc comment for the shape).
//
// GET /projects/{id}/filters' changeRequestStates never went through this at
// all -- unlike CaseStates/Severities/IssueTypes/EngagementTypes on the same
// response, which are all normalized a few lines above. On the Postgres data
// source, ReferenceDataRepository.EnumLabels (entity-service) returns the
// raw enum label as both id and label (e.g. {"id":"ROLLBACK","label":"ROLLBACK"}),
// since Postgres enums have no separate numeric id -- so
// filters.stateIds?.map(Number) on the frontend converted every selection to
// NaN, which Array.prototype.includes still matched via SameValueZero
// against crStateIDs's own reverse-mapped set of also-NaN entries, so the
// value silently reached the request as null instead of a real state key.
// Every state selection was equally broken, not just Rollback.
func normalizeChangeRequestStateChoices(items []ReferenceItem) []ReferenceItem {
	return normalizeChoices(items, nil, crStateIDs, crStateLabels)
}

// normalizeChangeRequestImpactChoices is normalizeChangeRequestStateChoices
// for change-request impact.
func normalizeChangeRequestImpactChoices(items []ReferenceItem) []ReferenceItem {
	return normalizeChoices(items, nil, crImpactIDs, crImpactLabels)
}

// crTypeRef builds a label-only {label} ref (no id) for entity-service's
// Type field: unlike State/Impact, entity-service's change-request search
// response passes ServiceNow's raw type label straight through unnormalized
// (see snCRTypeLabelToString), and there's no label-parsing table available
// to resolve it back to a numeric id the way case search's fuzzy word-match
// does for severity — a label-only ref still renders correctly, just
// without an id a caller could round-trip into a filter.
func crTypeRef(t *string) *IDLabelRef {
	if t == nil || *t == "" {
		return nil
	}
	return &IDLabelRef{Label: *t}
}
