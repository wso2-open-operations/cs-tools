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
// being asked. A Re-schedule on it used to go back through the CAB approval and
// then straight to Scheduled, because the cascade reads that very column
// (approvalGateTarget): the new plan was never put to the customer. The Re-schedule
// now writes OUR requirement column true with the new window; the sync-owned
// is_customer_approval_required (ServiceNow's record of the customer's answer) is
// never written.
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

func TestChangeRequestRescheduleLegacyIntegration_NormalAsksTheCustomerAgain(t *testing.T) {
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
			asked   bool
		}{
			{"project with registered contacts", func(f *crFlow) *string { return sp(crScopeProjectA) }, true},
			{"project with no contacts", func(f *crFlow) *string { return f.noContactProject() }, false},
			{"no project at all", func(f *crFlow) *string { return nil }, false},
		} {
			shape, proj := shape, proj
			t.Run(shape.name+"/"+proj.name, func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				id := f.legacyInCustomerApproval(domain.ChangeRequestTypeNormal, proj.project(f), shape.syncAnswer, shape.syncShape)
				f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
				before := len(f.stages(id))

				if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
					t.Fatalf("Re-schedule: %v", err)
				}
				f.expect(id, "after Re-schedule", "AUTHORIZE", "canceled")
				f.wantPlanned(id, "after Re-schedule", rsStart2, rsEnd2)
				required, synced := f.legacyRequirement(id)
				if !required {
					t.Fatal("customer_approval_required is still false after the Re-schedule: the CAB approval would schedule the change without the customer")
				}
				if fmtBoolPtr(synced) != fmtBoolPtr(shape.syncAnswer) {
					t.Fatalf("the sync-owned is_customer_approval_required was written: %s, want %s", fmtBoolPtr(synced), fmtBoolPtr(shape.syncAnswer))
				}
				if got := len(f.stages(id)); got != before+1 {
					t.Fatalf("stages after the Re-schedule = %d, want %d (one fresh CAB stage)", got, before+1)
				}

				// The CAB approves the new plan: back to Customer Approval, never Scheduled.
				if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
					t.Fatalf("CAB approval of the new plan: %v", err)
				}
				f.expect(id, "after the new CAB approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
				// (A row whose sync-owned answer flag is already true shows it: that is
				// ServiceNow's record, which nothing here writes -- checked above.)
				if a, _ := f.customerOutcome(id); a && (shape.syncAnswer == nil || !*shape.syncAnswer) {
					t.Fatal("the customer's approval is recorded although nobody answered")
				}
				stages := f.customerStages(id)
				if !proj.asked {
					// Nobody to ask: the change waits in Customer Approval (cancel or
					// re-schedule are the only ways out), it is not scheduled.
					if len(stages) != 0 {
						t.Fatalf("customer stages with nobody to ask = %+v", stages)
					}
					return
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

// Standard has no internal approval to repeat: the change stays in Customer
// Approval and the customer is asked again -- on a row with the box false too.
func TestChangeRequestRescheduleLegacyIntegration_StandardAsksTheCustomerAgain(t *testing.T) {
	for _, syncShape := range []bool{false, true} {
		for _, withContacts := range []bool{true, false} {
			t.Run(fmt.Sprintf("syncShape=%v/contacts=%v", syncShape, withContacts), func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				var project *string
				if withContacts {
					project = sp(crScopeProjectA)
				}
				id := f.legacyInCustomerApproval(domain.ChangeRequestTypeStandard, project, boolp(false), syncShape)
				if withContacts {
					// The change reached Customer Approval before the requirement column
					// existed: nobody has been asked yet. A restated project asks them.
					f.setProject(id, crScopeProjectA)
					if got := liveStages(f.customerStages(id)); got != 1 {
						t.Fatalf("live customer stages before the Re-schedule = %d, want 1", got)
					}
				}
				if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
					t.Fatalf("Re-schedule: %v", err)
				}
				f.expect(id, "after Re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
				f.wantPlanned(id, "after Re-schedule", rsStart2, rsEnd2)
				if required, _ := f.legacyRequirement(id); !required {
					t.Fatal("customer_approval_required is still false after the Re-schedule")
				}
				stages := f.customerStages(id)
				if !withContacts {
					if len(stages) != 0 {
						t.Fatalf("customer stages with nobody to ask = %+v", stages)
					}
					return
				}
				if len(stages) != 2 {
					t.Fatalf("customer stages after the Re-schedule = %+v, want the superseded request and a fresh one", stages)
				}
				assertApprovers(t, "the superseded request", stages[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
				assertApprovers(t, "the customer asked again", stages[1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
				if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
					t.Fatalf("the customer's approval: %v", err)
				}
				f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
			})
		}
	}
}

// The customer's own proposed time is the same Re-schedule: on a row with the box
// false the new plan goes back to the customer as well.
func TestChangeRequestRescheduleLegacyIntegration_ACustomersProposalAsksThemAgain(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.legacyInCustomerApproval(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), nil, true)
	// The legacy change reached Customer Approval with nobody asked; the customer's
	// proposal gives it its request first (ensureCustomerStageForLegacy).
	if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)}); err != nil {
		t.Fatalf("the customer's proposal: %v", err)
	}
	f.expect(id, "after the proposal", "AUTHORIZE", "canceled")
	if required, _ := f.legacyRequirement(id); !required {
		t.Fatal("customer_approval_required is still false after the customer's proposal")
	}
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval of the proposed plan: %v", err)
	}
	f.expect(id, "after the CAB approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	live := 0
	for _, st := range f.customerStages(id) {
		for uid, status := range st.approvers {
			if status == "REQUESTED" && (uid == crScopeUserA1 || uid == crScopeUserA2) {
				live++
			}
		}
	}
	if live != 2 {
		t.Fatalf("customer rows asked again = %d, want both registered contacts", live)
	}
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
		if required, _ := f.legacyRequirement(id); !required {
			t.Fatal("customer_approval_required = false after a Re-schedule that resent the stored false")
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
