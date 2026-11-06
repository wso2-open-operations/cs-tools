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

package repository_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The approved-content lock (change_request_approved_lock.go) against Postgres:
// once a change request has been approved its fields are read-only, a resent
// value is accepted, work notes and the lifecycle still go through, and before
// approval (or on a change cancelled before it) every field is editable.
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run ChangeRequestApprovedLockIntegration

// approvedLockSeed gives the change request the content a create would have
// written, so a resend can be told from a change.
func approvedLockSeed(f *crFlow, id string) {
	f.t.Helper()
	f.execSQL(`UPDATE work_item SET subject = $2, description = 'Approved description', assigned_to_id = NULL WHERE id = $1`, id, crFlowSubject)
	f.execSQL(`UPDATE change_request SET implementation_plan = '<p>Approved plan</p>', justification = 'Approved justification',
	           impact_description = NULL, risk_impact_analysis = 'Approved risk', rollback_process = NULL, backout_plan = 'Approved backout',
	           impact = 'LOW', priority = 'MODERATE', is_planning_visible_to_customers = NULL WHERE id = $1`, id)
}

func (f *crFlow) approvedLockRead(id string) (description, plan, updatedBy string) {
	f.t.Helper()
	if err := f.scoped.QueryRow(f.sys,
		`SELECT COALESCE(wi.description, ''), COALESCE(cr.implementation_plan, ''), COALESCE(wi.updated_by, '')
		 FROM work_item wi JOIN change_request cr ON cr.id = wi.id WHERE wi.id = $1`, id).Scan(&description, &plan, &updatedBy); err != nil {
		f.t.Fatalf("read back: %v", err)
	}
	return description, plan, updatedBy
}

func approvedLockRefusal(t *testing.T, err error, fields ...string) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v (%T), want a ValidationError", err, err)
	}
	if ve.Code != apierror.CodeChangeRequestApprovedReadOnly {
		t.Errorf("errorCode = %q, want %q", ve.Code, apierror.CodeChangeRequestApprovedReadOnly)
	}
	want := strings.Join(fields, ", ") + " can no longer be changed: the change request has been approved"
	if !strings.HasPrefix(ve.Msg, want) {
		t.Errorf("message = %q, want it to start %q", ve.Msg, want)
	}
}

func TestChangeRequestApprovedLockIntegration_ApprovedStatesAreReadOnly(t *testing.T) {
	str := func(v string) *string { return &v }
	for _, state := range []string{"CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "CLOSED", "ROLLBACK"} {
		t.Run(state, func(t *testing.T) {
			f := newCRFlow(t)
			f.seedAssignedGroup()
			id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
			approvedLockSeed(f, id)
			f.setState(id, state)
			f.execSQL(`UPDATE work_item SET updated_by = 'before-the-patch' WHERE id = $1`, id)
			actor := crFlowEmail(crFlowCreatorID)

			// One field. The tail says what is still possible: the deviation from the
			// implementation plan only once implementation has started and until the
			// change is final.
			_, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{Description: str("changed")}, actor)
			approvedLockRefusal(t, err, "description")
			tail := "; only work notes can be added"
			if state == "IMPLEMENT" || state == "REVIEW" || state == "CUSTOMER_REVIEW" {
				tail = "; only work notes and the deviation from the implementation plan can be added"
			}
			var ve *apierror.ValidationError
			if errors.As(err, &ve) && !strings.HasSuffix(ve.Msg, "(current state: "+strings.ToLower(state)+"), so its fields are read-only"+tail) {
				t.Errorf("message = %q, want it to end with the state and %q", ve.Msg, tail)
			}

			// The implementation plan, and several at once: every changed field is named.
			plan := str("<p>Another plan</p>")
			team := crFlowGroupID
			_, err = f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{
				Title: str("A new title"), ImplementationPlan: &plan, AssignedTeamID: &team,
			}, actor)
			approvedLockRefusal(t, err, "title", "implementationPlan")

			// A refused PATCH writes nothing, not even the rest of its own request.
			wn := "a work note riding with a refused edit"
			_, err = f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{Justification: str("new"), WorkNote: &wn}, actor)
			approvedLockRefusal(t, err, "justification")
			if d, p, by := f.approvedLockRead(id); d != "Approved description" || p != "<p>Approved plan</p>" || by != "before-the-patch" {
				t.Fatalf("after the refusals: description %q, plan %q, updated_by %q -- something was written", d, p, by)
			}
			var notes int
			if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM comment WHERE work_item_id = $1`, id).Scan(&notes); err != nil {
				t.Fatalf("count comments: %v", err)
			}
			if notes != 0 {
				t.Fatalf("a refused PATCH wrote %d comment rows", notes)
			}

			// The stored values resent (a client that sends the whole form back): accepted.
			// The two plans of an older build are compared through the column the detail reads.
			impact := domain.ChangeRequestImpact("low")
			moderate := domain.ChangeRequestPriority("moderate")
			mp := &moderate
			no := false
			if _, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{
				Title:                        str(crFlowSubject),
				Description:                  str("Approved description"),
				ImplementationPlan:           func() **string { v := str("<p>Approved plan</p>"); return &v }(),
				ImpactDescription:            str("Approved risk"),
				RollbackPlan:                 str("Approved backout"),
				Impact:                       &impact,
				Priority:                     &mp,
				IsPlanningVisibleToCustomers: &no,
				AssignedTeamID:               &team,
			}, actor); err != nil {
				t.Fatalf("the stored values resent: %v", err)
			}

			// Work notes are still added.
			note := "Implementation went to plan."
			if _, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{WorkNote: &note}, actor); err != nil {
				t.Fatalf("a work note: %v", err)
			}
			if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM comment WHERE work_item_id = $1 AND type = 'WORK_NOTE'`, id).Scan(&notes); err != nil {
				t.Fatalf("count work notes: %v", err)
			}
			if notes != 1 {
				t.Fatalf("work notes = %d, want 1", notes)
			}

			// On hold is the lifecycle, not the approved content.
			yes := true
			if _, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{OnHold: &yes, OnHoldReason: str("waiting on the customer")}, actor); err != nil {
				t.Fatalf("on hold: %v", err)
			}
		})
	}
}

func TestChangeRequestApprovedLockIntegration_EditableUntilApproved(t *testing.T) {
	str := func(v string) *string { return &v }
	for _, state := range []string{"NEW", "ASSESS", "AUTHORIZE", "CANCELED", ""} {
		name := state
		if name == "" {
			name = "NULL"
		}
		t.Run(name, func(t *testing.T) {
			f := newCRFlow(t)
			f.seedAssignedGroup()
			id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
			approvedLockSeed(f, id)
			f.setState(id, state)
			plan := str("<p>Revised plan</p>")
			if _, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{
				Description: str("Revised description"), ImplementationPlan: &plan,
			}, crFlowEmail(crFlowCreatorID)); err != nil {
				t.Fatalf("an edit before approval: %v", err)
			}
			if d, p, _ := f.approvedLockRead(id); d != "Revised description" || p != "<p>Revised plan</p>" {
				t.Fatalf("description %q, plan %q: the edit was not written", d, p)
			}
		})
	}
}

// The planned-date sort and filters read the window as every row shows it: a
// non-finite stored value ('infinity', '-infinity') is no date, so it sorts with
// the NULLs (last, either way) and no range matches it.
func TestChangeRequestPlannedDateSortIntegration_NonFiniteDatesAreNoDate(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	finite := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	none := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	inf := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	negInf := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.execSQL(`UPDATE change_request SET start_on = '2030-01-01T09:00:00Z', end_on = '2030-01-01T11:00:00Z' WHERE id = $1`, finite)
	f.execSQL(`UPDATE change_request SET start_on = NULL, end_on = NULL WHERE id = $1`, none)
	f.execSQL(`UPDATE change_request SET start_on = 'infinity', end_on = 'infinity' WHERE id = $1`, inf)
	f.execSQL(`UPDATE change_request SET start_on = '-infinity', end_on = '-infinity' WHERE id = $1`, negInf)

	search := func(order domain.ChangeRequestSortOrder, field domain.ChangeRequestSortField, mod func(*domain.SearchChangeRequestsFilters)) []string {
		t.Helper()
		filters := domain.SearchChangeRequestsFilters{SearchQuery: crFlowSubject}
		if mod != nil {
			mod(&filters)
		}
		rows, _, err := f.repo.SearchChangeRequests(f.sys, domain.SearchChangeRequestsRequest{
			Filters:    filters,
			SortBy:     domain.ChangeRequestSort{Field: field, Order: order},
			Pagination: domain.Pagination{Limit: 50},
		}, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("SearchChangeRequests: %v", err)
		}
		ids := make([]string, 0, len(rows))
		for _, cr := range rows {
			ids = append(ids, cr.ID)
		}
		return ids
	}
	for _, field := range []domain.ChangeRequestSortField{domain.ChangeRequestSortFieldPlannedStartOn, domain.ChangeRequestSortFieldPlannedEndOn} {
		for _, order := range []domain.ChangeRequestSortOrder{domain.ChangeRequestSortOrderAsc, domain.ChangeRequestSortOrderDesc} {
			got := search(order, field, nil)
			if len(got) != 4 || got[0] != finite {
				t.Fatalf("%s %s: order = %v, want the change with a real date (%s) first and the three without one after it", field, order, got, finite)
			}
		}
	}

	from := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	for name, mod := range map[string]func(*domain.SearchChangeRequestsFilters){
		"planned start from": func(fl *domain.SearchChangeRequestsFilters) { fl.PlannedStartFrom = &from },
		"planned start to":   func(fl *domain.SearchChangeRequestsFilters) { fl.PlannedStartTo = &to },
		"planned end from":   func(fl *domain.SearchChangeRequestsFilters) { fl.PlannedEndFrom = &from },
		"planned end to":     func(fl *domain.SearchChangeRequestsFilters) { fl.PlannedEndTo = &to },
	} {
		if got := search(domain.ChangeRequestSortOrderDesc, domain.ChangeRequestSortFieldPlannedStartOn, mod); len(got) != 1 || got[0] != finite {
			t.Fatalf("%s: matched %v, want only %s (a non-finite date is no date)", name, got, finite)
		}
	}
}
