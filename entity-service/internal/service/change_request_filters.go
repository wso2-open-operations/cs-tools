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

// changeRequestFilterFieldSet is the exact set of ChangeRequestFieldFilter.Field
// values accepted by change request search's generic filter array. This does
// not touch the change-request struct's pre-existing named fields
// (states/impacts/closedOn/projectIds/number/searchQuery), which stay outside
// this array.
var changeRequestFilterFieldSet = map[string]bool{
	"createdOn": true, "assignmentGroupId": true, "approval": true,
}

// changeRequestFilterOpSet is the exact set of ChangeRequestFieldFilter.Op
// values accepted by change request search, independent of field.
// Field/op compatibility is enforced separately in
// ParseChangeRequestFieldFilters.
var changeRequestFilterOpSet = map[string]bool{
	"gte": true, "lte": true, "in": true, "eq": true,
}

// changeRequestApprovalValueSet is the exact set of values ServiceNow's raw
// task.approval field can hold on a change_request record. Confirmed live
// and meaningful on the production instance (distinct from phase_state,
// which is structurally dead there).
var changeRequestApprovalValueSet = map[string]bool{
	"not requested": true, "requested": true, "approved": true, "rejected": true,
}

// requireChangeRequestFilterValues rejects a filter entry whose op needs a
// non-empty values array but doesn't have one.
func requireChangeRequestFilterValues(f domain.ChangeRequestFieldFilter) error {
	if len(f.Values) == 0 {
		return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q requires a non-empty values array", f.Field, f.Op)}
	}
	return nil
}

// badChangeRequestFilterCombo reports a field/op combination that is not supported.
func badChangeRequestFilterCombo(f domain.ChangeRequestFieldFilter) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q does not support op %q", f.Field, f.Op)}
}

// parseChangeRequestFilterDate parses a single filter value into a
// date/time -- a full RFC3339 timestamp, a plain YYYY-MM-DD date (interpreted
// as UTC midnight), or a relative-date placeholder (e.g. "__daysAgo:90__"),
// mirroring parseCaseFilterDate in case_filters.go. now is the reference
// instant relative placeholders resolve against.
func parseChangeRequestFilterDate(f domain.ChangeRequestFieldFilter, value string, now time.Time) (*time.Time, error) {
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

// parsedChangeRequestFilters is the internal, named-field representation that
// SearchChangeRequestsFilters.Filters is translated into by
// ParseChangeRequestFieldFilters. snChangeRequestService.SearchChangeRequests
// builds the outbound ServiceNow payload from this, unchanged from how it
// read the old flat CreatedStartDate/CreatedEndDate/AssignmentGroupIDs
// request fields.
type parsedChangeRequestFilters struct {
	CreatedStartDate *time.Time
	CreatedEndDate   *time.Time
	// AssignmentGroupIDs are sys_user_group UUIDs (not yet converted to
	// sysids -- that conversion happens where the outbound payload is built,
	// same as before).
	AssignmentGroupIDs []string
	// Approval is ServiceNow's raw task.approval value on the change
	// request ("not requested" / "requested" / "approved" / "rejected"),
	// passed straight through to SN as filters.approval.
	Approval *string
}

// ParseChangeRequestFieldFilters translates the change-request-search wire
// contract's generic filter array (domain.ChangeRequestFieldFilter) into
// parsedChangeRequestFilters, mirroring ParseCaseFieldFilters in
// case_filters.go. The caller is responsible for the cross-field
// CreatedEndDate-not-before-CreatedStartDate check, mirroring how
// sn_change_request_service.go already does that check for
// ClosedEndDate/ClosedStartDate post-parse.
func ParseChangeRequestFieldFilters(filters []domain.ChangeRequestFieldFilter, now time.Time) (parsedChangeRequestFilters, error) {
	var p parsedChangeRequestFilters

	for _, f := range filters {
		if !changeRequestFilterFieldSet[f.Field] {
			return parsedChangeRequestFilters{}, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !changeRequestFilterOpSet[f.Op] {
			return parsedChangeRequestFilters{}, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}

		switch f.Field {
		case "createdOn":
			if err := requireChangeRequestFilterValues(f); err != nil {
				return parsedChangeRequestFilters{}, err
			}
			if len(f.Values) != 1 {
				return parsedChangeRequestFilters{}, &apierror.ValidationError{Msg: "filters: field \"createdOn\" accepts exactly one value"}
			}
			t, err := parseChangeRequestFilterDate(f, f.Values[0], now)
			if err != nil {
				return parsedChangeRequestFilters{}, err
			}
			switch f.Op {
			case "gte":
				p.CreatedStartDate = t
			case "lte":
				p.CreatedEndDate = t
			default:
				return parsedChangeRequestFilters{}, badChangeRequestFilterCombo(f)
			}

		case "assignmentGroupId":
			if f.Op != "in" {
				return parsedChangeRequestFilters{}, badChangeRequestFilterCombo(f)
			}
			if err := requireChangeRequestFilterValues(f); err != nil {
				return parsedChangeRequestFilters{}, err
			}
			if err := validateUUIDs("filters: assignmentGroupId", f.Values); err != nil {
				return parsedChangeRequestFilters{}, err
			}
			p.AssignmentGroupIDs = append(p.AssignmentGroupIDs, f.Values...)

		case "approval":
			if f.Op != "eq" {
				return parsedChangeRequestFilters{}, badChangeRequestFilterCombo(f)
			}
			if err := requireChangeRequestFilterValues(f); err != nil {
				return parsedChangeRequestFilters{}, err
			}
			if len(f.Values) != 1 {
				return parsedChangeRequestFilters{}, &apierror.ValidationError{Msg: "filters: field \"approval\" accepts exactly one value"}
			}
			if !changeRequestApprovalValueSet[f.Values[0]] {
				return parsedChangeRequestFilters{}, &apierror.ValidationError{Msg: "filters: field \"approval\" value must be one of \"not requested\", \"requested\", \"approved\", \"rejected\""}
			}
			v := f.Values[0]
			p.Approval = &v
		}
	}

	return p, nil
}
