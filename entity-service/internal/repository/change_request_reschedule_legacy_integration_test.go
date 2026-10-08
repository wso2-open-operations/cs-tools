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
	"fmt"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Re-schedule re-asks the customer on EVERY row. Migration 0189 added
// change_request.customer_approval_required with a default of false, so a row that
// already sat in Customer Approval -- one that was migrated from ServiceNow, or one
// created before the column existed -- has the box false although the customer IS
// being asked. Re-schedule used to go back through the CAB approval and then straight to
// Scheduled on such a row (the cascade reads that very column, approvalGateTarget), which
// is why it had to write the column. It no longer goes through CAB at all (the change
// itself has not changed, only its time): it replaces the customers' request with a fresh
// one in Customer Approval, writes NO requirement flag -- not our own column, never the
// sync-owned is_customer_approval_required (the previous system's record of the customer's answer)
// -- and is refused when nobody can be asked, as Request Approval is.
//
// Same harness as TestChangeRequestFlowIntegration_* (crFlow, DSN-gated by
// CHANGE_REQUEST_TEST_DSN, run as a superuser and as the non-superuser csm_app).

// legacyRequirement reads our requirement column and the sync-owned answer flag.
func (f *crFlow) legacyRequirement(id string) (required bool, syncedAnswer *bool) {
	f.t.Helper()
	if err := f.scoped.QueryRow(f.sys,
		`SELECT customer_approval_required, is_customer_approval_required FROM change_request WHERE id = $1`, id).Scan(&required, &syncedAnswer); err != nil {
		f.t.Fatalf("read the requirement columns: %v", err)
	}
	return required, syncedAnswer
}

func fmtBoolPtr(p *bool) string {
	if p == nil {
		return "NULL"
	}
	return fmt.Sprint(*p)
}

// legacyInCustomerApproval makes a change that looks like one a migration or an
// older build left in Customer Approval: the state set directly, our requirement
// column false (the 0189 default), the sync-owned answer flag as given, a planned
// window, and -- when syncShape -- the approval stages the sync writes (no label,
// the group's own, already decided).
func (f *crFlow) legacyInCustomerApproval(typ domain.ChangeRequestType, project *string, syncedAnswer *bool, syncShape bool) string {
	f.t.Helper()
	id := f.createWithProject(typ, project, false, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.setState(id, "CUSTOMER_APPROVAL")
	f.execSQL(`UPDATE change_request SET customer_approval_required = false, is_customer_approval_required = $2 WHERE id = $1`, id, syncedAnswer)
	if syncShape {
		cab := crCABGroupID
		f.seedSyncedStage(id, nil, 50, map[string]string{crFlowPeerAID: "APPROVED"})
		f.seedSyncedStage(id, &cab, 40, map[string]string{crCABMemberUserID1: "APPROVED"})
	}
	if req, _ := f.legacyRequirement(id); req {
		f.t.Fatal("the fixture's requirement column is not false")
	}
	return id
}

// legacyRescheduleCase runs the Re-schedule matrix for one change type: every shape a
// legacy row can have (native with no stages, migrated with synced stages and the sync
// flag false / true) against every project situation.
func legacyRescheduleCase(t *testing.T, typ domain.ChangeRequestType) {
	for _, shape := range []struct {
		name       string
		syncShape  bool
		syncAnswer *bool
	}{
		{"native: no stages, sync flag NULL", false, nil},
		{"migrated shape: synced stages, sync flag false", true, boolp(false)},
		{"migrated shape: synced stages, sync flag true", true, boolp(true)},
	} {
		for _, proj := range []struct {
			name    string
			project func(f *crFlow) *string
			// refused: nobody can be asked (a project with no contacts); asked: somebody
			// can; neither: no project at all, where nothing is judged and nobody is asked.
			refused, asked bool
		}{
			{"project with registered contacts", func(f *crFlow) *string { return sp(crScopeProjectA) }, false, true},
			{"project with no contacts", func(f *crFlow) *string { return f.noContactProject() }, true, false},
			{"no project at all", func(f *crFlow) *string { return nil }, false, false},
		} {
			shape, proj := shape, proj
			t.Run(shape.name+"/"+proj.name, func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				id := f.legacyInCustomerApproval(typ, proj.project(f), shape.syncAnswer, shape.syncShape)
				f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
				before := len(f.stages(id))
				snapBefore := f.snap(id)

				err := f.reschedule(id, sp(rsStart2), sp(rsEnd2))
				if proj.refused {
					// Nobody to ask: refused with Request Approval's words, nothing written.
					f.wantExact("Re-schedule with nobody to ask", err, nobodyMsgApproval)
					if after := f.snap(id); after != snapBefore {
						t.Fatalf("a refused Re-schedule changed the change request:\n  before: %s\n  after:  %s", snapBefore, after)
					}
					return
				}
				if err != nil {
					t.Fatalf("Re-schedule: %v", err)
				}
				f.expect(id, "after Re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
				f.wantPlanned(id, "after Re-schedule", rsStart2, rsEnd2)
				required, synced := f.legacyRequirement(id)
				if required {
					t.Fatal("customer_approval_required was written by the Re-schedule: it goes through no CAB, so it needs no flag")
				}
				if fmtBoolPtr(synced) != fmtBoolPtr(shape.syncAnswer) {
					t.Fatalf("the sync-owned is_customer_approval_required was written: %s, want %s", fmtBoolPtr(synced), fmtBoolPtr(shape.syncAnswer))
				}
				stages := f.customerStages(id)
				if !proj.asked {
					// No project: nobody to ask, no stage (a legacy row keeps its exits).
					if len(stages) != 0 || len(f.stages(id)) != before {
						t.Fatalf("stages with nobody to ask = %+v (was %d)", f.stages(id), before)
					}
					return
				}
				if got := len(f.stages(id)); got != before+1 {
					t.Fatalf("stages after the Re-schedule = %d, want %d (one fresh customer stage, no CAB)", got, before+1)
				}
				if len(stages) != 1 {
					t.Fatalf("customer stages = %+v, want the one fresh request", stages)
				}
				assertApprovers(t, "the customer asked again", stages[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

				// Only the customer's own answer schedules it.
				if _, err := f.approveAs(id, crScopeUserA2, true); err != nil {
					t.Fatalf("the customer's approval: %v", err)
				}
				f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
			})
		}
	}
}

func TestChangeRequestRescheduleLegacyIntegration_NormalAsksTheCustomerAgain(t *testing.T) {
	legacyRescheduleCase(t, domain.ChangeRequestTypeNormal)
}

// Standard has no internal approval either way: the same Re-schedule, the same row shapes.
func TestChangeRequestRescheduleLegacyIntegration_StandardAsksTheCustomerAgain(t *testing.T) {
	legacyRescheduleCase(t, domain.ChangeRequestTypeStandard)
}

// The customer's own proposed time on a row with the box false: it gives the row the
// request it lacked (the customer's first act does), and then WAITS for WSO2 -- nothing
// else is written, no flag, no CAB. WSO2 answers it like any other: Accept schedules it
// (the box stays false, no approval is recorded) or a different time asks the customers
// again.
func TestChangeRequestRescheduleLegacyIntegration_ACustomersProposalAsksThemAgain(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.legacyInCustomerApproval(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), nil, true)
		// The legacy change reached Customer Approval with nobody asked; the customer's
		// proposal gives it its request first (ensureCustomerStageForLegacy).
		if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)}); err != nil {
			t.Fatalf("the customer's proposal: %v", err)
		}
		f.expect(id, "after the proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.wantPlanned(id, "after the proposal", rsStart1, rsEnd1)
		f.wantConversation(id, "after the proposal", rsStart2, "")
		if required, _ := f.legacyRequirement(id); required {
			t.Fatal("customer_approval_required was written by the customer's proposal")
		}
		live := 0
		for _, st := range f.customerStages(id) {
			for uid, status := range st.approvers {
				if status == "REQUESTED" && (uid == crScopeUserA1 || uid == crScopeUserA2) {
					live++
				}
			}
		}
		if live != 2 {
			t.Fatalf("customer rows asked = %d, want both registered contacts", live)
		}
		f.mustAccept(id)
		f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
		f.wantPlanned(id, "after Accept", rsStart2, rsEnd2)
		if required, _ := f.legacyRequirement(id); required {
			t.Fatal("customer_approval_required was written by Accept")
		}
	})
	t.Run("answered with a different time", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.legacyInCustomerApproval(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), nil, true)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("WSO2's different time: %v", err)
		}
		f.expect(id, "after the counter", "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.wantPlanned(id, "after the counter", rsStart3, rsEnd3)
		f.wantConversation(id, "after the counter", rsStart2, "DISAGREE")
		if n := f.liveStageRows(id, stageCustApproval); n != 2 {
			t.Fatalf("customer rows asked again = %d, want both registered contacts", n)
		}
	})
}

// Whatever else the request says about the box, a Re-schedule leaves the row
// requiring the customer's approval (the box is "true" for any change that waits in
// Customer Approval); a REFUSED re-schedule touches nothing, the column included.
func TestChangeRequestRescheduleLegacyIntegration_BoxInTheRequestAndRefusals(t *testing.T) {
	t.Run("a whole-form resend of the false box is not a way to skip the customer", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.legacyInCustomerApproval(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), nil, false)
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{
			State: stateptr(domain.ChangeRequestStateAuthorize), PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2),
			CustomerApprovalRequired: boolp(false)}); err != nil {
			t.Fatalf("Re-schedule carrying customerApprovalRequired: false: %v", err)
		}
		// The stored false is a no-op write: the Re-schedule asks the customer again anyway,
		// whatever the box says (it goes through no CAB), and writes no flag.
		if required, _ := f.legacyRequirement(id); required {
			t.Fatal("customer_approval_required = true after a Re-schedule that resent the stored false: nothing writes it")
		}
		f.expect(id, "after the Re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
		if n := f.liveStageRows(id, stageCustApproval); n != 2 {
			t.Fatalf("customer request has %d live rows after the Re-schedule, want 2 (asked again)", n)
		}
	})
	t.Run("a refused re-schedule leaves the column alone", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.legacyInCustomerApproval(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), nil, false)
		// "Time Change = No": the stored window again.
		err := f.reschedule(id, sp(rsStart1), sp(rsEnd1))
		f.wantValidationError("re-schedule to the same window", err, "re-scheduling requires a changed planned start or end")
		// On hold.
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{OnHold: boolp(true), OnHoldReason: sp("freeze")}); err != nil {
			t.Fatalf("put on hold: %v", err)
		}
		f.wantValidationError("re-schedule on hold", f.reschedule(id, sp(rsStart2), sp(rsEnd2)), "change request is on hold")
		f.expect(id, "after the refused re-schedules", "CUSTOMER_APPROVAL", "authorize", "canceled")
		if required, _ := f.legacyRequirement(id); required {
			t.Fatal("a refused Re-schedule wrote customer_approval_required")
		}
	})
	t.Run("a change that was never in Customer Approval is not touched by a refused authorize", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.requestApproval(id)
		f.wantValidationError("authorize out of Assess", f.reschedule(id, sp(rsStart2), sp(rsEnd2)), "cannot be set manually")
		if required, _ := f.legacyRequirement(id); required {
			t.Fatal("a refused authorize from Assess wrote customer_approval_required")
		}
	})
}
