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
	"strings"
	"testing"
	"time"
)

var plannedTestNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

// The two layouts entity-service accepts are accepted, and nothing else: the
// Postgres values that used to ride through `::timestamptz` ('tomorrow', 'now',
// 'infinity', a bare date, a zone name) are refused here before the request leaves.
func TestValidatePlannedWindow_AcceptsOnlyTheTwoLayoutsInRange(t *testing.T) {
	ok := []string{
		"2030-03-01T09:00:00Z",
		"2030-03-01T09:00:00+05:30",
		"2030-03-01T09:00:00.123456Z",
		"2030-03-01 09:00:00",
		"2000-01-01 00:00:00",
		"2100-12-31 23:59:59",
	}
	for _, v := range ok {
		if err := ValidatePlannedWindow("plannedStartOn", sp(v), "plannedEndOn", nil, PlannedWindowRules{}); err != nil {
			t.Errorf("%q refused: %v", v, err)
		}
	}
	bad := []string{
		"tomorrow", "now", "today", "yesterday", "epoch", "infinity", "-infinity", "allballs",
		"", " ", "2030-03-01", "2030-03-01T09:00:00", "2030-03-01 09:00", "2030-03-01 09:00:00 UTC",
		"2030-03-01 09:00:00 America/New_York", "01/03/2030 09:00:00", "2030-13-01 09:00:00",
		"2030-02-30 09:00:00", "1999-12-31 23:59:59", "2101-01-01 00:00:00", "0001-01-01 00:00:00",
		"99999-01-01 00:00:00", "2030-03-01 09:00:00; drop table work_item",
	}
	for _, v := range bad {
		err := ValidatePlannedWindow("plannedStartOn", sp(v), "plannedEndOn", nil, PlannedWindowRules{})
		if err == nil {
			t.Errorf("%q accepted", v)
			continue
		}
		if !IsPlannedWindowError(err) || !strings.Contains(err.Error(), "plannedStartOn must be a valid date-time") {
			t.Errorf("%q: error = %v, want it to name plannedStartOn and say 'must be a valid date-time'", v, err)
		}
	}
	// The end is named when the end is the bad one.
	err := ValidatePlannedWindow("plannedStartOn", sp("2030-03-01 09:00:00"), "plannedEndOn", sp("tomorrow"), PlannedWindowRules{})
	if err == nil || !strings.Contains(err.Error(), "plannedEndOn must be a valid date-time") {
		t.Errorf("a bad end: error = %v, want it to name plannedEndOn", err)
	}
}

// An absent bound is not a mistake (a request may move one side of the stored
// window) and is not checked.
func TestValidatePlannedWindow_AbsentBoundsAreNotChecked(t *testing.T) {
	if err := ValidatePlannedWindow("a", nil, "b", nil, PlannedWindowRules{Now: plannedTestNow, RequireFuture: true, RequireOrder: true}); err != nil {
		t.Fatalf("nothing supplied: %v", err)
	}
	if err := ValidatePlannedWindow("a", nil, "b", sp("2030-03-01 09:00:00"), PlannedWindowRules{Now: plannedTestNow, RequireFuture: true, RequireOrder: true}); err != nil {
		t.Fatalf("an end alone: %v", err)
	}
}

// A customer's proposal is held to "still to come"; a staff edit is not.
func TestValidatePlannedWindow_PastIsRefusedOnlyForAProposal(t *testing.T) {
	past := "2026-10-06 11:59:59"
	if err := ValidatePlannedWindow("plannedStartOn", sp(past), "plannedEndOn", nil, PlannedWindowRules{}); err != nil {
		t.Errorf("staff may record a start that has gone: %v", err)
	}
	proposal := PlannedWindowRules{Now: plannedTestNow, RequireFuture: true}
	for _, v := range []string{past, "2026-10-06 12:00:00", "2026-10-06T12:00:00Z", "2026-10-06T17:30:00+05:30", "2020-01-01 00:00:00"} {
		err := ValidatePlannedWindow("plannedStartOn", sp(v), "plannedEndOn", nil, proposal)
		if err == nil || !strings.Contains(err.Error(), "plannedStartOn is in the past") {
			t.Errorf("start %q for a proposal: error = %v, want 'plannedStartOn is in the past'", v, err)
		}
	}
	// 12:00:01 UTC is the first instant that is still to come.
	if err := ValidatePlannedWindow("plannedStartOn", sp("2026-10-06 12:00:01"), "plannedEndOn", nil, proposal); err != nil {
		t.Errorf("a start one second ahead: %v", err)
	}
	// An end in the past is refused too, and named.
	err := ValidatePlannedWindow("plannedStartOn", nil, "plannedEndOn", sp(past), proposal)
	if err == nil || !strings.Contains(err.Error(), "plannedEndOn is in the past") {
		t.Errorf("a past end: error = %v", err)
	}
}

// A proposal with both bounds must be a window with a duration; entity-service
// answers the same two messages.
func TestValidatePlannedWindow_OrderIsRefusedOnlyWhenAsked(t *testing.T) {
	rules := PlannedWindowRules{Now: plannedTestNow, RequireFuture: true, RequireOrder: true}
	err := ValidatePlannedWindow("plannedStartOn", sp("2030-03-01 10:00:00"), "plannedEndOn", sp("2030-03-01 09:00:00"), rules)
	if err == nil || !strings.Contains(err.Error(), "must not be after the planned end") {
		t.Errorf("inverted: error = %v", err)
	}
	err = ValidatePlannedWindow("plannedStartOn", sp("2030-03-01 10:00:00"), "plannedEndOn", sp("2030-03-01T10:00:00Z"), rules)
	if err == nil || !strings.Contains(err.Error(), "must not be the same as the planned end") {
		t.Errorf("zero length (the two layouts of one instant): error = %v", err)
	}
	if err := ValidatePlannedWindow("plannedStartOn", sp("2030-03-01 09:00:00"), "plannedEndOn", sp("2030-03-01 10:00:00"), rules); err != nil {
		t.Errorf("a proper window: %v", err)
	}
	// Without RequireOrder (a plain edit, a create) this layer does not refuse
	// what entity-service would accept.
	if err := ValidatePlannedWindow("plannedStartOn", sp("2030-03-01 10:00:00"), "plannedEndOn", sp("2030-03-01 09:00:00"), PlannedWindowRules{}); err != nil {
		t.Errorf("order must not be enforced on a plain edit: %v", err)
	}
}
