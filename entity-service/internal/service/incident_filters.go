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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// incidentFilterFieldSet is the exact set of IncidentFieldFilter.Field values
// accepted by incident search. Anything else is rejected outright.
var incidentFilterFieldSet = map[string]bool{
	"state": true, "assignmentGroupId": true, "businessServiceId": true,
	"createdOn": true, "slaViolated": true, "madeSla": true, "productName": true,
}

// incidentFilterOpSet is the exact set of IncidentFieldFilter.Op values
// accepted by incident search, independent of field. Field/op compatibility
// is enforced separately in ParseIncidentFieldFilters -- "in" covers state/
// assignmentGroupId/businessServiceId/productName, "gte"/"lte" cover
// createdOn (mirrors case_filters.go's "createdOn" handling exactly,
// including its relative-date placeholder support, e.g. "__daysAgo:90__"),
// and "eq" covers slaViolated/madeSla (single boolean value, mirroring case
// search's "number"/"internalId" single-value eq fields).
var incidentFilterOpSet = map[string]bool{
	"in": true, "gte": true, "lte": true, "eq": true,
}

// parseIncidentFilterBool mirrors case_filters.go's parseCaseFilterPercent
// error-shape convention, retyped for a boolean-valued eq filter. Only the
// exact lowercase "true"/"false" the OpenAPI contract documents are accepted
// -- not every string strconv.ParseBool tolerates (e.g. "1", "t", "TRUE").
func parseIncidentFilterBool(f domain.IncidentFieldFilter, value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q value %q must be a boolean", f.Field, f.Op, value)}
	}
}

// parseIncidentFilterDate mirrors case_filters.go's parseCaseFilterDate,
// retyped for domain.IncidentFieldFilter -- same RFC3339/date-only/
// relative-placeholder parsing, same error message shape.
func parseIncidentFilterDate(f domain.IncidentFieldFilter, value string, now time.Time) (*time.Time, error) {
	if resolved, matched, err := resolveRelativeDate(value, now); err != nil {
		return nil, err
	} else if matched {
		value = resolved
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		// A date-only lte bound means "on or before that whole day".
		if f.Op == "lte" {
			t = t.AddDate(0, 0, 1).Add(-time.Nanosecond)
		}
		return &t, nil
	}
	return nil, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q value %q must be an RFC3339 timestamp, YYYY-MM-DD date, or a recognized relative-date placeholder", f.Field, f.Op, value)}
}

// requireIncidentFilterValues rejects a filter entry whose op needs a
// non-empty values array but doesn't have one.
func requireIncidentFilterValues(f domain.IncidentFieldFilter) error {
	if len(f.Values) == 0 {
		return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q requires a non-empty values array", f.Field, f.Op)}
	}
	return nil
}

// badIncidentFilterCombo reports a field/op combination that is not supported.
func badIncidentFilterCombo(f domain.IncidentFieldFilter) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q does not support op %q", f.Field, f.Op)}
}

// parsedIncidentFilters is the internal, named-field representation that
// SearchIncidentsFilters.Filters is translated into by
// ParseIncidentFieldFilters. snIncidentService.SearchIncidents builds the
// outbound ServiceNow payload from this, unchanged from how it read the old
// flat StateKeys/AssignmentGroupIDs/BusinessServiceIDs request fields.
type parsedIncidentFilters struct {
	// StateKeys are ServiceNow's raw incident_state numeric keys, already
	// translated from the wire-level domain.IncidentState enum values via
	// snIncidentStateKeyMap.
	StateKeys []int
	// AssignmentGroupIDs are sys_user_group UUIDs (not yet converted to
	// sysids -- that conversion happens where the outbound payload is built,
	// same as before).
	AssignmentGroupIDs []string
	// BusinessServiceIDs are business_service UUIDs (not yet converted to
	// sysids).
	BusinessServiceIDs []string
	// StartCreatedDate is the inclusive lower bound of a createdOn "gte" filter.
	StartCreatedDate *time.Time
	// EndCreatedDate is the inclusive upper bound of a createdOn "lte" filter.
	EndCreatedDate *time.Time
	// SlaViolated is set from a "slaViolated" "eq" filter: true/false restricts
	// to incidents with/without at least one breached SLA record; nil means
	// the filter was not supplied.
	SlaViolated *bool
	// MadeSla is set from a "madeSla" "eq" filter: true/false restricts to
	// incidents matching ServiceNow's raw `made_sla` field; nil means the
	// filter was not supplied. Deliberately kept separate from SlaViolated
	// above -- see domain.SearchIncidentsFilters Filters "madeSla" doc
	// comment: this is SN's own less-reliable raw signal, kept only for
	// exact parity with SN's native incident dashboards.
	MadeSla *bool
	// ProductNames are the values of a "productName" "in" filter, matched as a
	// union against the incident's backing business_service name.
	ProductNames []string
}

// ParseIncidentFieldFilters translates the incident-search wire contract's
// generic filter array (domain.IncidentFieldFilter) into parsedIncidentFilters,
// mirroring ParseCaseFieldFilters in case_filters.go. now is the reference
// instant relative-date placeholders (e.g. "__daysAgo:90__") resolve against.
func ParseIncidentFieldFilters(filters []domain.IncidentFieldFilter, now time.Time) (parsedIncidentFilters, error) {
	var p parsedIncidentFilters

	for _, f := range filters {
		if !incidentFilterFieldSet[f.Field] {
			return parsedIncidentFilters{}, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !incidentFilterOpSet[f.Op] {
			return parsedIncidentFilters{}, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}

		switch f.Field {
		case "state":
			if f.Op != "in" {
				return parsedIncidentFilters{}, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return parsedIncidentFilters{}, err
			}
			for _, v := range f.Values {
				state := domain.IncidentState(v)
				if !validIncidentState[state] {
					return parsedIncidentFilters{}, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q value %q is not a valid incident state", f.Field, v)}
				}
				p.StateKeys = append(p.StateKeys, snIncidentStateKeyMap[state])
			}

		case "assignmentGroupId":
			if f.Op != "in" {
				return parsedIncidentFilters{}, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return parsedIncidentFilters{}, err
			}
			if err := validateUUIDs("filters: assignmentGroupId", f.Values); err != nil {
				return parsedIncidentFilters{}, err
			}
			p.AssignmentGroupIDs = append(p.AssignmentGroupIDs, f.Values...)

		case "businessServiceId":
			if f.Op != "in" {
				return parsedIncidentFilters{}, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return parsedIncidentFilters{}, err
			}
			if err := validateUUIDs("filters: businessServiceId", f.Values); err != nil {
				return parsedIncidentFilters{}, err
			}
			p.BusinessServiceIDs = append(p.BusinessServiceIDs, f.Values...)

		case "createdOn":
			if err := requireIncidentFilterValues(f); err != nil {
				return parsedIncidentFilters{}, err
			}
			t, err := parseIncidentFilterDate(f, f.Values[0], now)
			if err != nil {
				return parsedIncidentFilters{}, err
			}
			switch f.Op {
			case "gte":
				p.StartCreatedDate = t
			case "lte":
				p.EndCreatedDate = t
			default:
				return parsedIncidentFilters{}, badIncidentFilterCombo(f)
			}

		case "slaViolated":
			if f.Op != "eq" {
				return parsedIncidentFilters{}, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return parsedIncidentFilters{}, err
			}
			if len(f.Values) != 1 {
				return parsedIncidentFilters{}, &apierror.ValidationError{Msg: "filters: slaViolated eq requires exactly one value"}
			}
			b, err := parseIncidentFilterBool(f, f.Values[0])
			if err != nil {
				return parsedIncidentFilters{}, err
			}
			p.SlaViolated = &b

		case "madeSla":
			if f.Op != "eq" {
				return parsedIncidentFilters{}, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return parsedIncidentFilters{}, err
			}
			if len(f.Values) != 1 {
				return parsedIncidentFilters{}, &apierror.ValidationError{Msg: "filters: madeSla eq requires exactly one value"}
			}
			b, err := parseIncidentFilterBool(f, f.Values[0])
			if err != nil {
				return parsedIncidentFilters{}, err
			}
			p.MadeSla = &b

		case "productName":
			if f.Op != "in" {
				return parsedIncidentFilters{}, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return parsedIncidentFilters{}, err
			}
			p.ProductNames = append(p.ProductNames, f.Values...)
		}
	}

	return p, nil
}
