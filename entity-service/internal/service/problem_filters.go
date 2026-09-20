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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// problemFilterFieldSet is the exact set of ProblemFieldFilter.Field values
// accepted by problem search. Anything else is rejected outright.
var problemFilterFieldSet = map[string]bool{
	"state": true, "assignmentGroupId": true, "assignedUserId": true,
}

// problemFilterOpSet is the exact set of ProblemFieldFilter.Op values
// accepted by problem search, independent of field. Field/op compatibility
// is enforced separately in ParseProblemFieldFilters -- today every
// supported field only accepts "in", but the set is kept apart from the
// per-field check to mirror case_filters.go/incident_filters.go's shape.
var problemFilterOpSet = map[string]bool{
	"in": true,
}

// requireProblemFilterValues rejects a filter entry whose op needs a
// non-empty values array but doesn't have one.
func requireProblemFilterValues(f domain.ProblemFieldFilter) error {
	if len(f.Values) == 0 {
		return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q requires a non-empty values array", f.Field, f.Op)}
	}
	return nil
}

// badProblemFilterCombo reports a field/op combination that is not supported.
func badProblemFilterCombo(f domain.ProblemFieldFilter) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q does not support op %q", f.Field, f.Op)}
}

// parsedProblemFilters is the internal, named-field representation that
// SearchProblemsFilters.Filters is translated into by
// ParseProblemFieldFilters. snProblemService.SearchProblems builds the
// outbound ServiceNow payload from this.
type parsedProblemFilters struct {
	// StateKeys are ServiceNow's raw problem_state numeric keys, already
	// translated from the wire-level domain.ProblemState enum values via
	// snProblemStateKeyMap.
	StateKeys []int
	// AssignmentGroupIDs are sys_user_group UUIDs (not yet converted to
	// sysids -- that conversion happens where the outbound payload is built).
	AssignmentGroupIDs []string
	// AssignedUserIDs are sys_user UUIDs (not yet converted to sysids -- that
	// conversion happens where the outbound payload is built, same as
	// AssignmentGroupIDs above). Wire field name is "assignedUserId" (singular,
	// matching case search's own convention); it is sent to Ballerina/SN as the
	// plural "assignedUserIds" JSON key.
	AssignedUserIDs []string
}

// ParseProblemFieldFilters translates the problem-search wire contract's
// generic filter array (domain.ProblemFieldFilter) into parsedProblemFilters,
// mirroring ParseIncidentFieldFilters in incident_filters.go.
func ParseProblemFieldFilters(filters []domain.ProblemFieldFilter) (parsedProblemFilters, error) {
	var p parsedProblemFilters

	for _, f := range filters {
		if !problemFilterFieldSet[f.Field] {
			return parsedProblemFilters{}, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !problemFilterOpSet[f.Op] {
			return parsedProblemFilters{}, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}

		switch f.Field {
		case "state":
			if f.Op != "in" {
				return parsedProblemFilters{}, badProblemFilterCombo(f)
			}
			if err := requireProblemFilterValues(f); err != nil {
				return parsedProblemFilters{}, err
			}
			for _, v := range f.Values {
				state := domain.ProblemState(v)
				if !validProblemState[state] {
					return parsedProblemFilters{}, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q value %q is not a valid problem state", f.Field, v)}
				}
				p.StateKeys = append(p.StateKeys, snProblemStateKeyMap[state])
			}

		case "assignmentGroupId":
			if f.Op != "in" {
				return parsedProblemFilters{}, badProblemFilterCombo(f)
			}
			if err := requireProblemFilterValues(f); err != nil {
				return parsedProblemFilters{}, err
			}
			if err := validateUUIDs("filters: assignmentGroupId", f.Values); err != nil {
				return parsedProblemFilters{}, err
			}
			p.AssignmentGroupIDs = append(p.AssignmentGroupIDs, f.Values...)

		case "assignedUserId":
			if f.Op != "in" {
				return parsedProblemFilters{}, badProblemFilterCombo(f)
			}
			if err := requireProblemFilterValues(f); err != nil {
				return parsedProblemFilters{}, err
			}
			if err := validateUUIDs("filters: assignedUserId", f.Values); err != nil {
				return parsedProblemFilters{}, err
			}
			p.AssignedUserIDs = append(p.AssignedUserIDs, f.Values...)
		}
	}

	return p, nil
}
