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

package repository

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestChangeRequestContentApproved_EveryState(t *testing.T) {
	want := map[string]bool{
		"":                      false,
		crStateNew:              false,
		crStateAssess:           false,
		crStateAuthorize:        false,
		crStateCustomerApproval: true,
		crStateScheduled:        true,
		crStateImplement:        true,
		crStateReview:           true,
		crStateCustomerReview:   true,
		crStateClosed:           true,
		crStateRollback:         true,
		// A change can be cancelled before anybody approved it.
		crStateCanceled: false,
	}
	for state, approved := range want {
		if got := changeRequestContentApproved(state); got != approved {
			t.Errorf("changeRequestContentApproved(%q) = %v, want %v", state, got, approved)
		}
	}
}

func TestChangeRequestLinksLockedStates_CoverEveryApprovedState(t *testing.T) {
	// The deployments are judged by the links rule, so it must hold every state the
	// approved-content lock holds, or a deployment could still move after approval.
	for state := range changeRequestApprovedStates {
		if !changeRequestLinksLockedStates[state] {
			t.Errorf("changeRequestLinksLockedStates[%q] = false; the deployments must be locked once the change is approved", state)
		}
	}
	for _, state := range []string{crStateNew, crStateAssess, crStateAuthorize} {
		if changeRequestLinksLockedStates[state] {
			t.Errorf("changeRequestLinksLockedStates[%q] = true; the deployments are editable until the change is approved", state)
		}
	}
}

func TestChangeRequestPatchTouchesApprovedContent(t *testing.T) {
	s := func(v string) *string { return &v }
	b := func(v bool) *bool { return &v }
	state := domain.ChangeRequestState("implement")
	notContent := []domain.PatchChangeRequestRequest{
		{State: &state},
		{WorkNote: s("a note")},
		{Comment: s("a comment")},
		{OnHold: b(true), OnHoldReason: s("waiting")},
		{RequestApproval: b(true)},
		// The deviation from the implementation plan is written after the
		// approval, so it is never part of the approved content.
		{ImplementationDeviation: s("<p>done differently</p>")},
	}
	for i, req := range notContent {
		if changeRequestPatchTouchesApprovedContent(req) {
			t.Errorf("case %d: a request with no field of the approved content touches it", i)
		}
	}
	content := []domain.PatchChangeRequestRequest{
		{Title: s("x")},
		{Description: s("x")},
		{Justification: s("x")},
		{ImpactDescription: s("x")},
		{RollbackPlan: s("x")},
		{TestPlan: s("x")},
		{AssignedEngineerID: s("00000000-0000-0000-0000-000000000001")},
		{AssignedTeamID: s("00000000-0000-0000-0000-000000000001")},
		{CaseID: s("00000000-0000-0000-0000-000000000001")},
		{PlannedStartOn: s("2030-01-01T00:00:00Z")},
		{IsPlanningVisibleToCustomers: b(true)},
		{CustomerReviewRequired: b(true)},
	}
	for i, req := range content {
		if !changeRequestPatchTouchesApprovedContent(req) {
			t.Errorf("case %d: a request that names a field of the approved content does not touch it", i)
		}
	}
}

func TestApprovedLockFields_StaffTimeResponseMovesOnlyTheWindow(t *testing.T) {
	s := func(v string) *string { return &v }
	req := domain.PatchChangeRequestRequest{
		PlannedStartOn: s("2030-01-01T00:00:00Z"),
		PlannedEndOn:   s("2030-01-01T02:00:00Z"),
		Description:    s("x"),
	}
	names := func(fields []approvedLockField) string {
		out := make([]string, len(fields))
		for i, f := range fields {
			out[i] = f.name
		}
		return strings.Join(out, ",")
	}
	if got := names(approvedLockFields(req, false)); got != "description,plannedStartOn,plannedEndOn" {
		t.Errorf("an edit: fields = %s", got)
	}
	// A Re-schedule out of Customer Approval moves the window of an approved change
	// (the customer is asked again); anything else it carries is still judged.
	if got := names(approvedLockFields(req, true)); got != "description" {
		t.Errorf("a staff time response: fields = %s", got)
	}
}

func TestApprovedLockFields_ExplicitNullBindsNothing(t *testing.T) {
	var nullText *string
	var nullPriority *domain.ChangeRequestPriority
	req := domain.PatchChangeRequestRequest{
		ImplementationPlan: &nullText,
		Priority:           &nullPriority,
	}
	fields := approvedLockFields(req, false)
	if len(fields) != 2 {
		t.Fatalf("fields = %d, want 2", len(fields))
	}
	for _, f := range fields {
		if f.bind || !strings.HasSuffix(f.predicate, "IS NOT NULL") {
			t.Errorf("%s: a clear is judged as \"the column holds a value\" with nothing bound, got %+v", f.name, f)
		}
	}
}

func TestApprovedContentQuery_NumbersEveryBoundValueAfterTheID(t *testing.T) {
	s := func(v string) *string { return &v }
	var clear *string
	req := domain.PatchChangeRequestRequest{
		Title:              s("t"),
		ImplementationPlan: &clear, // binds nothing
		Description:        s("d"),
		AssignedTeamID:     s("00000000-0000-0000-0000-000000000901"),
	}
	query, args := approvedContentQuery(approvedLockFields(req, false))
	if len(args) != 4 || args[0] != nil {
		t.Fatalf("args = %v, want the id slot then three values", args)
	}
	for i := 2; i <= 4; i++ {
		if !strings.Contains(query, fmt.Sprintf("$%d", i)) {
			t.Errorf("query has no $%d:\n%s", i, query)
		}
	}
	if strings.Contains(query, "$5") || strings.Contains(query, "%!") {
		t.Errorf("query numbers more values than it binds, or a placeholder was not filled:\n%s", query)
	}
	for _, name := range []string{"'title'", "'implementationPlan'", "'description'", "'assignedTeamId'"} {
		if !strings.Contains(query, name) {
			t.Errorf("query does not name %s:\n%s", name, query)
		}
	}
}

func TestApprovedContentLockedMsg(t *testing.T) {
	// While the change is open the deviation from the implementation plan can still be recorded.
	got := approvedContentLockedMsg([]string{"description", "implementationPlan"}, crStateScheduled)
	want := "description, implementationPlan can no longer be changed: the change request has been approved (current state: scheduled), so its fields are read-only; only work notes and the deviation from the implementation plan can be added"
	if got != want {
		t.Errorf("message = %q\nwant      %q", got, want)
	}
	// Once it is closed or rolled back, that is final too.
	for _, state := range []string{crStateClosed, crStateRollback} {
		got := approvedContentLockedMsg([]string{"description"}, state)
		want := "description can no longer be changed: the change request has been approved (current state: " + lockStateName(state) + "), so its fields are read-only; only work notes can be added"
		if got != want {
			t.Errorf("%s: message = %q\nwant      %q", state, got, want)
		}
	}
}
