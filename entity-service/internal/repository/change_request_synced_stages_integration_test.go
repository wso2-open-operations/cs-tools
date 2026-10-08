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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Approval stages MIGRATED from ServiceNow (csm-sync-service mirrors
// sysapproval_group / sysapproval_approver): no checkpoint_label, an approver
// whose user row may be missing, a group that may or may not be the CAB's. The
// flow used to read such a stage by its POSITION alone (0 = Peer, 1 = CAB), so an
// Emergency change in Authorize whose single synced stage sat at position 0 had a
// PEER stage in Authorize: its approver's decision was refused as stale (409) and
// canDecide was false; a stale position-0 stage of a Normal change that had moved
// on was cancelled by the next state change. These tests pin what is read of an
// unlabeled stage now (runtimeApprovalStageKind): what its group, the change's
// type and state, and its position prove -- and that nothing legitimately pending
// is wrongly cancelled, hidden or refused. They seed SN-shaped rows directly (the
// local seed has none), as the other stale-approval tests do.

// seedSyncedStage inserts an approval stage with NO label, assigned to groupID
// (nil: none -- an unsynced ServiceNow group also yields NULL), created
// ageMinutes ago (stages are ordered by created_on: that is their position), with
// one approver row per entry (user id -> state; "" as the user id inserts a row
// whose user is NULL, which a sync can leave behind).
func (f *crFlow) seedSyncedStage(id string, groupID *string, ageMinutes int, rows map[string]string) string {
	f.t.Helper()
	var stageID string
	if err := f.scoped.QueryRow(f.sys,
		`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status)
		 VALUES (gen_random_uuid(), now() - make_interval(mins => $2::int), now(), 'sn-sync', 'sn-sync', $1, $3::uuid, 'REQUESTED') RETURNING id::text`,
		id, ageMinutes, groupID).Scan(&stageID); err != nil {
		f.t.Fatalf("seed a synced stage: %v", err)
	}
	for uid, status := range rows {
		var user any = uid
		if uid == "" {
			user = nil
		}
		f.execSQL(`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
		           VALUES (gen_random_uuid(), now(), now(), 'sn-sync', 'sn-sync', $1::uuid, $2, $3::uuid, $4)`, stageID, id, user, status)
	}
	return stageID
}

func (f *crFlow) approverState(stageID, userID string) string {
	f.t.Helper()
	var st string
	if err := f.scoped.QueryRow(f.sys, `SELECT state FROM approval_stage_approver WHERE stage_id = $1::uuid AND approver_user_id = $2::uuid`, stageID, userID).Scan(&st); err != nil {
		f.t.Fatalf("read the approver row: %v", err)
	}
	return st
}

// Emergency in Authorize, one synced stage at position 0 and no label and no group: the
// only approval an Emergency change has (it has no peer stage), so the CAB's. Its approver can
// decide, and approving cascades to Scheduled exactly as a native CAB approval. The label shown
// is the positional one, unchanged: with no group there is nothing to name it by.
func TestChangeRequestSyncedStagesIntegration_EmergencyInAuthorizeWithAStageAtPositionZero(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeEmergency, nil, false, false)
	f.setState(id, "AUTHORIZE")
	stage := f.seedSyncedStage(id, nil, 30, map[string]string{crCABMemberUserID1: "REQUESTED"})

	f.wantCanDecide(id, "in Authorize", map[string][]string{crCABMemberUserID1: {"Assess"}})
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("the approver's decision on the synced stage: %v", err)
	}
	f.expect(id, "after the approver's decision", "SCHEDULED", "implement", "canceled")
	if got := f.approverState(stage, crCABMemberUserID1); got != "APPROVED" {
		t.Fatalf("approver row = %s, want APPROVED", got)
	}
}

// The creator rule and the internal-only rule still hold on a stage read this way.
func TestChangeRequestSyncedStagesIntegration_TheApproverRulesStillHold(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeEmergency, nil, false, false)
	f.setState(id, "AUTHORIZE")
	f.seedSyncedStage(id, nil, 30, map[string]string{
		crFlowCreatorID: "REQUESTED", crCABMemberUserID1: "REQUESTED", crScopeUserA1: "REQUESTED"})

	// The creator never approves their own change, whatever stage it is.
	f.wantForbidden("the creator on a synced Emergency stage", f.decide(id, crFlowCreatorID, "approved"), "creator of a change request cannot approve")
	// A customer holding a row on what is an internal stage is not let in either.
	f.wantForbidden("a customer on a synced Emergency stage", f.decide(id, crScopeUserA1, "approved"), "only active internal")
	if got := f.canDecideStages(id, crFlowCreatorID); len(got) != 0 {
		t.Fatalf("canDecide for the creator = %v", got)
	}
	if got := f.canDecideStages(id, crScopeUserA1); len(got) != 0 {
		t.Fatalf("canDecide for a customer on an internal stage = %v", got)
	}
	if got := f.canDecideStages(id, crCABMemberUserID1); len(got) != 1 {
		t.Fatalf("canDecide for the CAB member = %v, want the one stage", got)
	}
	f.expect(id, "after the refused decisions", "AUTHORIZE", "canceled")
}

// A synced stage in the CAB's own group is the CAB's, wherever it sits in the
// order; deciding it cascades like a native one. In any other state it is left
// alone instead of being cancelled for a guess.
func TestChangeRequestSyncedStagesIntegration_ACabGroupStageWithNoLabel(t *testing.T) {
	f := newCustomerGroupFlow(t)
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	cab := crCABGroupID
	id := f.createWithProject(domain.ChangeRequestTypeNormal, nil, false, false)
	f.setState(id, "AUTHORIZE")
	// Two earlier synced stages (positions 0 and 1) that nothing can decide in this state...
	f.seedSyncedStage(id, nil, 50, map[string]string{crFlowPeerAID: "APPROVED"})
	f.seedSyncedStage(id, nil, 40, map[string]string{crFlowPeerBID: "CANCELLED"})
	// ...and the CAB group's own, third, with no label.
	stage := f.seedSyncedStage(id, &cab, 30, map[string]string{crCABMemberUserID1: "REQUESTED"})

	f.wantCanDecide(id, "in Authorize", map[string][]string{crCABMemberUserID1: {"Customer Approval"}})
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("the CAB approver's decision on the synced stage: %v", err)
	}
	f.expect(id, "after the CAB group's decision", "SCHEDULED", "implement", "canceled")
	if got := f.approverState(stage, crCABMemberUserID1); got != "APPROVED" {
		t.Fatalf("approver row = %s", got)
	}

	// The same row on a change that is already Scheduled: not a CAB approval any
	// more, but nobody cancels it for that.
	other := f.createWithProject(domain.ChangeRequestTypeNormal, nil, false, false)
	f.setState(other, "SCHEDULED")
	stage2 := f.seedSyncedStage(other, &cab, 30, map[string]string{crCABMemberUserID1: "REQUESTED"})
	f.step(other, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	if got := f.approverState(stage2, crCABMemberUserID1); got != "REQUESTED" {
		t.Fatalf("a CAB-group row on a change that moved on was cancelled: %s", got)
	}
	// A finished change takes every row, as it always did.
	f.setState(other, "REVIEW")
	f.step(other, domain.ChangeRequestStateClosed, "CLOSED")
	if got := f.approverState(stage2, crCABMemberUserID1); got != "CANCELLED" {
		t.Fatalf("a closed change left the synced row %s, want CANCELLED", got)
	}
}

// A Normal change that has moved on to Authorize with a stale position-0 stage:
// the stage is not a Peer approval gone out of date (a guess), it is of no known
// kind -- so its approver is neither refused nor hidden, and the change's own
// state moves leave the row alone.
func TestChangeRequestSyncedStagesIntegration_AStalePositionZeroStageIsNeverCancelledOrRefused(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, nil, false, false)
	f.setState(id, "AUTHORIZE")
	stage := f.seedSyncedStage(id, nil, 30, map[string]string{crFlowPeerAID: "REQUESTED"})
	f.wantCanDecide(id, "in Authorize", map[string][]string{crFlowPeerAID: {"Assess"}})

	// The change moves through several states (state moves run the reconcile).
	f.setState(id, "SCHEDULED")
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	if got := f.approverState(stage, crFlowPeerAID); got != "REQUESTED" {
		t.Fatalf("a state move cancelled the synced position-0 row: %s", got)
	}
	f.wantCanDecide(id, "in Implement", map[string][]string{crFlowPeerAID: {"Assess"}})
	// Deciding it is recorded; with no known kind nothing cascades.
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("deciding the synced stage in Implement: %v", err)
	}
	f.expect(id, "after the decision", "IMPLEMENT", "review", "canceled")
	if got := f.approverState(stage, crFlowPeerAID); got != "APPROVED" {
		t.Fatalf("approver row = %s, want APPROVED", got)
	}
}

// A position-0 stage on a Normal change that really is in Assess is the Peer
// stage, as it always was: its approval is the Peer approval and cascades.
func TestChangeRequestSyncedStagesIntegration_APositionZeroStageInAssessIsStillPeer(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, nil, false, false)
	f.setState(id, "ASSESS")
	f.seedSyncedStage(id, nil, 30, map[string]string{crFlowPeerAID: "REQUESTED"})
	f.wantCanDecide(id, "in Assess", map[string][]string{crFlowPeerAID: {"Assess"}})
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("the peer approval of a synced stage: %v", err)
	}
	f.expect(id, "after the peer approval", "AUTHORIZE", "canceled")
}

// A synced stage at position 2 is never taken for the customer's stage, however
// the change is placed: the customer's answer is recorded on the stage this
// service writes for them (for a legacy change request with nobody asked, the
// first customer act provisions it), never on the synced row they also hold; the
// synced stage is left as it was (no outcome is recorded through it, nothing moves
// because of it), and the customer portal's approvals read cuts it down to its
// label and status like any internal stage.
func TestChangeRequestSyncedStagesIntegration_APositionTwoStageIsNeverTheCustomers(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
	// A legacy change request nobody was asked about: Request Approval refuses the
	// project with no contacts today, so the contact it had at Request Approval is
	// gone before the gate (the residual edge).
	f.requestApprovalThenContactsLeave(id)
	f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
	// A synced third stage naming the project's contact (and an internal person).
	f.registerContact(crScopeProjectC, crScopeAccountID, crScopeUserA1)
	synced := f.seedSyncedStage(id, nil, 0, map[string]string{crScopeUserA1: "REQUESTED", crFlowPeerAID: "REQUESTED"})

	// Read as a customer: the stage is there (label, status) and nothing of its approvers.
	view, err := f.repo.GetChangeRequestApprovals(asContact(crScopeUserA1), id)
	if err != nil {
		t.Fatalf("approvals as the contact: %v", err)
	}
	if len(view.Approvals) != 3 {
		t.Fatalf("approvals as the contact = %d stages, want 3", len(view.Approvals))
	}
	third := view.Approvals[2]
	if third.ApproverName != "" || third.AssignmentGroup != nil || len(third.Approvers) != 0 {
		t.Fatalf("the unlabeled stage leaks to a customer: %+v", third)
	}
	if third.Stage == "" || third.Status == "" {
		t.Fatalf("the unlabeled stage lost its label or status: %+v", third)
	}

	// The customer approves. The change request is legacy and has no live customer
	// stage, so the first customer act provisions the labelled one; the decision is
	// recorded THERE (the oldest REQUESTED row the contact holds is the synced one,
	// which is not the customer's stage and must not be picked).
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("the customer's approval: %v", err)
	}
	f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
	if a, _ := f.customerOutcome(id); !a {
		t.Fatal("the customer's approval was not recorded")
	}
	st := f.customerStages(id)
	if len(st) != 1 {
		t.Fatalf("customer stages = %+v, want the one provisioned for the answer", st)
	}
	assertApprovers(t, "the customer's stage", st[0].approvers, map[string]string{crScopeUserA1: "APPROVED"})
	if got := f.approverState(synced, crScopeUserA1); got != "REQUESTED" {
		t.Fatalf("the customer's answer landed on the synced stage: its row is %s, want it untouched", got)
	}
	if got := f.approverState(synced, crFlowPeerAID); got != "REQUESTED" {
		t.Fatalf("the synced stage's other row = %s, want untouched", got)
	}
}

// An approver whose user row is missing (the sync can leave one): nothing in the
// approvals read, canDecide or the state moves fails or cancels because of it.
func TestChangeRequestSyncedStagesIntegration_AnApproverWithNoUser(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, nil, false, false)
	f.setState(id, "ASSESS")
	f.seedSyncedStage(id, nil, 30, map[string]string{"": "REQUESTED", crFlowPeerAID: "REQUESTED"})

	view := f.approvalsAs(id, crFlowPeerAID)
	if len(view.Approvals) != 1 || len(view.Approvals[0].Approvers) != 2 {
		t.Fatalf("approvals = %+v, want one stage with both approvers", view.Approvals)
	}
	named := 0
	for _, ap := range view.Approvals[0].Approvers {
		if ap.ID == "" {
			t.Fatalf("an approver with no user has no id: %+v", ap)
		}
		if ap.Name != "" {
			named++
		}
		if ap.CanDecide && ap.Status != "REQUESTED" {
			t.Fatalf("canDecide on a row that is not requested: %+v", ap)
		}
	}
	if got := f.canDecideStages(id, crFlowPeerAID); len(got) != 1 {
		t.Fatalf("canDecide for the approver who has a user = %v, want the one stage", got)
	}
	// A state move runs the reconcile over the row with no user.
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval beside a row with no user: %v", err)
	}
	f.expect(id, "after the peer approval", "AUTHORIZE", "canceled")
}

// "Nobody is eligible" says how many people were looked at and why none counted:
// counts by user_type / inactive, never names, and the rule itself unchanged (the
// excluded are still not approvers; with one eligible member the request goes
// through and the excluded are simply not asked).
func TestChangeRequestSyncedStagesIntegration_NobodyEligibleIsDiagnosable(t *testing.T) {
	// members seeds a group of five nobody can serve with: two whose type could not
	// be derived, an internal user who is inactive, an external one and a system one.
	// (seedGroupMembersOfType empties a CAB / ECAB group on each call, so the first
	// member goes through it and the rest are added beside it.)
	members := func(f *crFlow, group string) {
		const prefix = "3aaaaaaa-0000-0000-0000-0000000000c"
		seedGroupMembersOfType(t, f.scoped, group, "NOT_AVAILABLE", prefix+"1")
		seedMoreGroupMember(t, f, group, "NOT_AVAILABLE", prefix+"2")
		seedMoreGroupMember(t, f, group, "INTERNAL", prefix+"3")
		f.execSQL(`UPDATE "user" SET is_active = false WHERE id = $1`, prefix+"3")
		seedMoreGroupMember(t, f, group, "EXTERNAL", prefix+"4")
		seedMoreGroupMember(t, f, group, "SYSTEM", prefix+"5")
	}

	t.Run("the CAB group", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		members(f, crCABGroupID)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		_, err := f.patchState(id, domain.ChangeRequestStateAssess)
		f.wantValidationError("Request Approval into a CAB nobody can serve", err, `the "CAB Approval" group has no active internal (WSO2) members to provision as CAB Approval approvers`)
		f.wantValidationError("Request Approval into a CAB nobody can serve", err, `(the "CAB Approval" group: 5 members, none eligible: `)
		for _, want := range []string{"2 user_type NOT_AVAILABLE", "1 inactive", "1 external", "1 system"} {
			f.wantValidationError("the counts", err, want)
		}
		assertNoNames(t, err)
		if got := f.state(id); got != "NEW" {
			t.Fatalf("state after the refusal = %s, want NEW", got)
		}
	})

	t.Run("the CAB group, asked by an Emergency change", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		members(f, crCABGroupID)
		id := f.create(domain.ChangeRequestTypeEmergency, crFlowGroupID)
		_, err := f.patchState(id, domain.ChangeRequestStateAssess)
		f.wantValidationError("Request Approval into a CAB nobody can serve", err, `the "CAB Approval" group has no active internal (WSO2) members`)
		f.wantValidationError("Request Approval into a CAB nobody can serve", err, `(the "CAB Approval" group: 5 members, none eligible: `)
		f.wantValidationError("the counts", err, "2 user_type NOT_AVAILABLE")
		assertNoNames(t, err)
	})

	t.Run("the assigned team of the Review stage", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		// The team's members have since turned out to be of no usable type.
		f.execSQL(`UPDATE "user" SET user_type = 'NOT_AVAILABLE'::user_type_enum WHERE id IN ($1, $2, $3, $4)`,
			crFlowCreatorID, crFlowPeerAID, crFlowPeerBID, crFlowOutsiderID)
		f.execSQL(`UPDATE "user" SET is_active = false WHERE id = $1`, crFlowOutsiderID)
		_, err := f.patchState(id, domain.ChangeRequestStateReview)
		f.wantValidationError("Review of a team nobody can serve", err, "the assigned team has no active internal (WSO2) members")
		f.wantValidationError("Review of a team nobody can serve", err, "the assigned team: 4 members, none eligible: ")
		f.wantValidationError("the counts", err, "3 user_type NOT_AVAILABLE")
		f.wantValidationError("the counts", err, "1 inactive")
		f.expect(id, "after the refusal", "IMPLEMENT", "review", "canceled")
	})

	t.Run("the peer pool names both groups it tried", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		seedGroupMembersOfType(t, f.scoped, crFlowGroupID, "NOT_AVAILABLE", "3aaaaaaa-0000-0000-0000-0000000000c1", "3aaaaaaa-0000-0000-0000-0000000000c2")
		seedExternalGroupMembers(t, f.scoped, crFlowGroupID, "3aaaaaaa-0000-0000-0000-0000000000c4")
		// Only the creator and customers/untyped users are left in the assigned group.
		f.execSQL(`DELETE FROM team_member WHERE user_id IN ($1, $2, $3)`, crFlowPeerAID, crFlowPeerBID, crFlowOutsiderID)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		_, err := f.patchState(id, domain.ChangeRequestStateAssess)
		f.wantValidationError("Request Approval with nobody eligible", err, "no eligible peer approvers")
		f.wantValidationError("the assigned group's counts", err, "the assigned group: 4 members, none eligible: ")
		f.wantValidationError("the assigned group's counts", err, "2 user_type NOT_AVAILABLE")
		f.wantValidationError("the assigned group's counts", err, "1 external")
		f.wantValidationError("the assigned group's counts", err, "1 creator")
		assertNoNames(t, err)
	})

	t.Run("the rule is not loosened: one eligible member is enough, the others are not asked", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		seedGroupMembersOfType(t, f.scoped, crCABGroupID, "NOT_AVAILABLE", "3aaaaaaa-0000-0000-0000-0000000000c1")
		seedMoreGroupMember(t, f, crCABGroupID, "INTERNAL", crCABMemberUserID1)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.requestApproval(id)
		if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
			t.Fatalf("peer approval: %v", err)
		}
		assertApprovers(t, "CAB stage", f.stage(id, "CAB Approval").approvers, map[string]string{crCABMemberUserID1: "REQUESTED"})
	})
}

// seedMoreGroupMember adds one more member of the given type to a group without
// emptying it first (seedGroupMembersOfType starts by isolating a CAB / ECAB
// group).
func seedMoreGroupMember(t *testing.T, f *crFlow, groupID, userType, userID string) {
	t.Helper()
	id := userID
	cleanup := func() { _, _ = f.scoped.Exec(f.sys, `DELETE FROM "user" WHERE id = $1`, id) }
	cleanup()
	t.Cleanup(cleanup)
	f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
	           VALUES ($1, now(), now(), 'cr-flow-test', 'cr-flow-test', $2, 'CR Flow User', 'CR', 'Flow User', $2, true, false)`, id, crFlowEmail(id))
	setTestUserType(t, f.scoped, f.sys, id, userType)
	f.execSQL(`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
	           VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, $2, $3::uuid)`, seededGroupID, id, groupID)
}

// assertNoNames: the diagnostics are counts, never who.
func assertNoNames(t *testing.T, err error) {
	t.Helper()
	for _, leak := range []string{"@", "CR Flow User", "crflow-"} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("the refusal names someone (%q): %s", leak, err.Error())
		}
	}
}
