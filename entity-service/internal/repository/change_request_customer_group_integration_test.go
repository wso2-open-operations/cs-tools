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
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The customer group answers Customer Approval / Customer Review through
// approval stages of its own ("Customer Approval" / "Customer Review"). Same
// harness as TestChangeRequestFlowIntegration_* (crFlow, DSN-gated by
// CHANGE_REQUEST_TEST_DSN).

const (
	stageCustApproval = "Customer Approval"
	stageCustReview   = "Customer Review"

	// The project contacts that make up the Customer Group (seedScope):
	// project A's registered portal users Alice and Bob, project B's Carol.
	crScopeUserA1 = crScopeUserAlice
	crScopeUserA2 = crScopeUserBob
	crScopeUserB1 = crScopeUserCarol
)

// registerContact makes an existing user a REGISTERED portal-user contact of
// the project (account contact, project contact, PORTAL_USER role).
func (f *crFlow) registerContact(projectID, accountID, userID string) {
	f.t.Helper()
	var email string
	if err := f.scoped.QueryRow(f.sys, `SELECT user_name FROM "user" WHERE id = $1`, userID).Scan(&email); err != nil {
		f.t.Fatalf("read user %s: %v", userID, err)
	}
	var acID, pcID string
	if err := f.scoped.QueryRow(f.sys,
		`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1, $2) RETURNING id::text`, email, accountID).Scan(&acID); err != nil {
		f.t.Fatalf("seed account_contact: %v", err)
	}
	if err := f.scoped.QueryRow(f.sys,
		`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1, $2, $3, 'REGISTERED') RETURNING id::text`, email, acID, projectID).Scan(&pcID); err != nil {
		f.t.Fatalf("seed project_contact: %v", err)
	}
	f.execSQL(`INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
	           SELECT gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, id FROM project_group WHERE "group" = 'CR Scope PORTAL_USER'`, pcID)
}

// createWithProject creates a change request on the given project (nil = none)
// with the two customer gates as stated; the Customer Group is whatever the
// project's registered contacts are.
func (f *crFlow) createWithProject(typ domain.ChangeRequestType, project *string, approval, review bool) string {
	f.t.Helper()
	g := crFlowGroupID
	resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &g, ProjectID: project,
		CustomerApprovalRequired: boolp(approval), CustomerReviewRequired: boolp(review),
	}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		f.t.Fatalf("CreateChangeRequest(%s): %v", typ, err)
	}
	if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`, crFlowCreatorID, resp.ChangeRequest.ID); err != nil {
		f.t.Fatalf("set requested_by: %v", err)
	}
	return resp.ChangeRequest.ID
}

func sp(s string) *string { return &s }

// setProject moves the change request to another project; its deployments are
// cleared with it (the fixtures carry none, so this is just the project).
func (f *crFlow) setProject(id string, project string) {
	f.t.Helper()
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{ProjectID: &project, DeploymentIDs: &[]string{}}); err != nil {
		f.t.Fatalf("PATCH projectId %s: %v", project, err)
	}
}

// customerStages returns the change's Customer Approval / Customer Review
// stages in creation order.
func (f *crFlow) customerStages(id string) []crFlowStage {
	f.t.Helper()
	var out []crFlowStage
	for _, st := range f.stages(id) {
		if st.label == stageCustApproval || st.label == stageCustReview {
			out = append(out, st)
		}
	}
	return out
}

// liveStages counts the stages with a REQUESTED approver.
func liveStages(stages []crFlowStage) int {
	n := 0
	for _, st := range stages {
		for _, status := range st.approvers {
			if status == "REQUESTED" {
				n++
				break
			}
		}
	}
	return n
}

func (f *crFlow) labels(id string) []string {
	f.t.Helper()
	var out []string
	for _, st := range f.stages(id) {
		out = append(out, st.label)
	}
	return out
}

func (f *crFlow) approvalsAs(id, viewerID string) domain.ChangeRequestApprovals {
	f.t.Helper()
	ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true, ViewerEmail: crFlowEmail(viewerID)})
	got, err := f.repo.GetChangeRequestApprovals(ctx, id)
	if err != nil {
		f.t.Fatalf("GetChangeRequestApprovals: %v", err)
	}
	return got
}

func (f *crFlow) canDecideAs(id, viewerID string) map[string]bool {
	f.t.Helper()
	out := map[string]bool{}
	for _, a := range f.approvalsAs(id, viewerID).Approvals {
		for _, ap := range a.Approvers {
			if ap.CanDecide {
				out[a.Stage+"/"+ap.ID] = true
			}
		}
	}
	return out
}

func (f *crFlow) wantForbidden(what string, err error, contains string) {
	f.t.Helper()
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		f.t.Fatalf("%s: err = %v (%T), want *apierror.ForbiddenError", what, err, err)
	}
	if !strings.Contains(fe.Msg, contains) {
		f.t.Fatalf("%s: message %q should contain %q", what, fe.Msg, contains)
	}
}

// driveToCustomerApproval takes a Normal change from New through Request
// Approval, peer approval and CAB approval, to Customer Approval.
func (f *crFlow) driveToCustomerApproval(id string) {
	f.t.Helper()
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "ASSESS", "canceled")
	f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
}

// driveToCustomerReview takes a change that is Scheduled through Implement and
// Review to Customer Review.
func (f *crFlow) driveToCustomerReview(id string) {
	f.t.Helper()
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
}

func newCustomerGroupFlow(t *testing.T) *crFlow {
	t.Helper()
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	f.seedScope()
	return f
}

// Normal, both boxes ticked, customer group set: state, stages, approvers,
// legalNextStates and the recorded outcome after every step.
func TestChangeRequestFlowIntegration_CustomerGroupNormalLifecycle(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)

	cr := f.get(id)
	if got := contactNames(cr.CustomerContacts); strings.Join(got, ",") != "Alice Aaron,Bob Bell" {
		t.Fatalf("customerContacts read back = %v, want project A's Alice Aaron,Bob Bell", got)
	}
	f.driveToCustomerApproval(id)

	// Entering Customer Approval provisioned the stage: one REQUESTED row per
	// group member, assignment group = the customer group, and the customer's
	// approval is not offered to staff (asserted by driveToCustomerApproval:
	// legalNextStates is [authorize, canceled]).
	stages := f.stages(id)
	if got := f.labels(id); strings.Join(got, ",") != "Peer Approval,CAB Approval,Customer Approval" {
		t.Fatalf("stages after CAB approval = %v", got)
	}
	ca := stages[2]
	if ca.groupID != "" {
		t.Fatalf("Customer Approval stage group = %s, want none (the Customer Group is the project's contacts)", ca.groupID)
	}
	assertApprovers(t, "Customer Approval", ca.approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	if approved, _ := f.customerOutcome(id); approved {
		t.Fatal("is_customer_approval_required already true before any member decided")
	}

	// The Approvals read response names the stage and group.
	view := f.approvalsAs(id, crScopeUserA1)
	if len(view.Approvals) != 3 || view.Approvals[2].Stage != stageCustApproval || view.Approvals[2].ApproverName != "Customer Group" {
		t.Fatalf("approvals = %+v, want a third stage %q named after the group", view.Approvals, stageCustApproval)
	}

	// A member approves: the change is scheduled, the other member's row is
	// cancelled, and the customer's approval is recorded.
	if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
		t.Fatalf("customer approval: %v", err)
	}
	f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
	assertApprovers(t, "Customer Approval after", f.stages(id)[2].approvers, map[string]string{crScopeUserA1: "APPROVED", crScopeUserA2: "CANCELLED"})
	if approved, reviewed := f.customerOutcome(id); !approved || reviewed {
		t.Fatalf("outcome after customer approval = approved:%v reviewed:%v, want true/false", approved, reviewed)
	}

	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	// The Review stage still gets provisioned (the customer stage must not
	// shift its checkpoint ordinal), and no customer review stage exists yet.
	if got := f.labels(id); strings.Join(got, ",") != "Peer Approval,CAB Approval,Customer Approval,Review" {
		t.Fatalf("stages in Review = %v", got)
	}
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	if got := f.labels(id); strings.Join(got, ",") != "Peer Approval,CAB Approval,Customer Approval,Review,Customer Review" {
		t.Fatalf("stages in Customer Review = %v", got)
	}
	cr5 := f.stages(id)[4]
	assertApprovers(t, "Customer Review", cr5.approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

	// The other member decides this time.
	if err := f.decide(id, crScopeUserA2, "approved"); err != nil {
		t.Fatalf("customer review: %v", err)
	}
	f.expect(id, "after the customer's review", "CLOSED")
	assertApprovers(t, "Customer Review after", f.stages(id)[4].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "APPROVED"})
	if approved, reviewed := f.customerOutcome(id); !approved || !reviewed {
		t.Fatalf("final outcome = approved:%v reviewed:%v, want true/true", approved, reviewed)
	}
}

// Rejections: Customer Approval rejected cancels the change; Customer Review
// rejected moves it to Rollback. Neither stamps the customer flag.
func TestChangeRequestFlowIntegration_CustomerGroupRejections(t *testing.T) {
	t.Run("customer approval rejected cancels", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.driveToCustomerApproval(id)
		if err := f.decide(id, crScopeUserA2, "rejected"); err != nil {
			t.Fatalf("reject: %v", err)
		}
		f.expect(id, "after the customer rejected", "CANCELED")
		assertApprovers(t, "Customer Approval", f.stages(id)[2].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "REJECTED"})
		if approved, _ := f.customerOutcome(id); approved {
			t.Fatal("is_customer_approval_required = true on a rejected approval")
		}
	})
	t.Run("customer review rejected rolls back", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.expect(id, "after Request Approval (Standard, no customer approval)", "SCHEDULED", "implement", "canceled")
		f.driveToCustomerReview(id)
		// While the request is pending the manual rollback is refused...
		_, err := f.patchState(id, domain.ChangeRequestStateRollback)
		f.wantValidationError("manual rollback with a live customer stage", err, "approving or rejecting it")
		f.expect(id, "after the refused manual rollback", "CUSTOMER_REVIEW", "canceled")
		// ...and the members' rejection is what rolls the change back.
		if err := f.decide(id, crScopeUserA1, "rejected"); err != nil {
			t.Fatalf("reject: %v", err)
		}
		f.expect(id, "after the customer rejected the review", "ROLLBACK")
		if n := f.requestedApprovers(id); n != 0 {
			t.Fatalf("%d approver rows still REQUESTED after the rejection", n)
		}
		_, err = f.patchState(id, domain.ChangeRequestStateClosed)
		f.wantValidationError("close out of rollback", err, "a change request that is rolled back cannot be moved")
		assertApprovers(t, "Customer Review", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REJECTED", crScopeUserA2: "CANCELLED"})
		if _, reviewed := f.customerOutcome(id); reviewed {
			t.Fatal("is_customer_review_required = true on a rejected review")
		}
	})
}

// Standard + Customer Approval: Request Approval lands in Customer Approval
// with the stage already provisioned.
func TestChangeRequestFlowIntegration_CustomerGroupStandardRequestApproval(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, false)
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	st := f.customerStages(id)
	if len(st) != 1 || st[0].label != stageCustApproval {
		t.Fatalf("customer stages = %+v", st)
	}
	assertApprovers(t, "Customer Approval", st[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	f.expect(id, "after approval", "SCHEDULED", "implement", "canceled")
}

// Emergency on a project whose contacts could answer, with the customer box stored as ticked
// (a row from before the rule, or a migrated one): the CAB approval SCHEDULES it -- the customer
// is never asked, and no customer stage is provisioned. (Replaces the test in which an ECAB
// approval entered Customer Approval and provisioned the customer stage.)
func TestChangeRequestFlowIntegration_CustomerGroupEmergencyNeverAsksTheCustomer(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeEmergency, sp(crScopeProjectA), false, false)
	f.execSQL(`UPDATE change_request SET customer_approval_required = true WHERE id = $1`, id)
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "AUTHORIZE", "canceled")
	if got := f.labels(id); strings.Join(got, ",") != "CAB Approval" {
		t.Fatalf("stages after Request Approval = %v, want the one CAB stage", got)
	}
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.expect(id, "after the CAB approval", "SCHEDULED", "implement", "canceled")
	if st := f.customerStages(id); len(st) != 0 {
		t.Fatalf("customer stages = %+v, want none", st)
	}
}

// The creator is never an approver, even as a contact of the project;
// a non-member cannot decide and is told why.
func TestChangeRequestFlowIntegration_CustomerGroupCreatorAndNonMember(t *testing.T) {
	f := newCustomerGroupFlow(t)
	f.registerContact(crScopeProjectA, crScopeAccountID, crFlowCreatorID)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)

	assertApprovers(t, "Customer Approval", f.stages(id)[2].approvers,
		map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED", crFlowCreatorID: "CANCELLED"})

	err := f.decide(id, crFlowCreatorID, "approved")
	f.wantForbidden("creator deciding", err, "creator of a change request cannot approve")
	// A non-member (an outsider, a peer) gets a readable 403, not a bare 404.
	for _, uid := range []string{crFlowOutsiderID, crFlowPeerAID, crCABMemberUserID1, crScopeUserB1} {
		err := f.decide(id, uid, "approved")
		f.wantForbidden("non-member "+uid, err, `only members of the customer group`)
	}
	f.expect(id, "after refused decisions", "CUSTOMER_APPROVAL", "authorize", "canceled")

	// canDecide: true only for a member, on their own REQUESTED row.
	if got := f.canDecideAs(id, crScopeUserA1); len(got) != 1 || !got[stageCustApproval+"/"+crScopeUserA1] {
		t.Fatalf("canDecide for a member = %v, want only their own Customer Approval row", got)
	}
	for _, uid := range []string{crFlowCreatorID, crFlowOutsiderID, crScopeUserB1, crCABMemberUserID1} {
		if got := f.canDecideAs(id, uid); len(got) != 0 {
			t.Fatalf("canDecide for %s = %v, want none", uid, got)
		}
	}
}

// First responder wins; the loser's later attempt is refused.
func TestChangeRequestFlowIntegration_CustomerGroupFirstResponderWins(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, false)
	f.requestApproval(id)
	if err := f.decide(id, crScopeUserA2, "approved"); err != nil {
		t.Fatalf("first responder: %v", err)
	}
	f.expect(id, "after the first responder", "SCHEDULED", "implement", "canceled")
	err := f.decide(id, crScopeUserA1, "rejected")
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("late decision err = %v (%T), want NotFoundError (no pending approval)", err, err)
	}
	f.expect(id, "after the late decision", "SCHEDULED", "implement", "canceled")
	if got := f.canDecideAs(id, crScopeUserA1); len(got) != 0 {
		t.Fatalf("canDecide for the superseded member = %v", got)
	}
}

// Nobody to ask: a project without registered contacts, or nobody eligible among
// them -> no customer stage. There is NO manual way out any more: the customer's
// approval / review can only be given by the customer, so staff are left with
// Cancel and Re-schedule (Customer Approval) / Roll back (Customer Review). ("No
// project" is not a way to get here: a box ticked on a change with no Customer
// Project is refused at Request Approval -- see
// TestChangeRequestLockIntegration_RequestApprovalNeedsAProject -- and so is a box
// ticked on a project nobody can be asked on, see
// TestChangeRequestNobodyToAskIntegration_RequestApprovalMatrix.) What is left is
// the residual edge: the contacts the project had when approval was requested are
// gone by the time the gate is reached, which these cases build with a stand-in
// contact that leaves right after Request Approval (requestApprovalThenContactsLeave).
func TestChangeRequestFlowIntegration_CustomerGroupNobodyToAsk(t *testing.T) {
	cases := []struct {
		name    string
		project *string
		prepare func(f *crFlow)
	}{
		{"project without registered contacts", sp(crScopeProjectC), nil},
		{"project whose only contact is the creator", sp(crScopeProjectC), func(f *crFlow) {
			f.registerContact(crScopeProjectC, crScopeAccountID, crFlowCreatorID)
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Customer Approval", func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				if tc.prepare != nil {
					tc.prepare(f)
				}
				id := f.createWithProject(domain.ChangeRequestTypeNormal, tc.project, true, false)
				f.requestApprovalThenContactsLeave(id)
				f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
				if n := len(f.customerStages(id)); n != 0 {
					t.Fatalf("customer stage provisioned: %d", n)
				}
				_, err := f.patchState(id, domain.ChangeRequestStateScheduled)
				f.wantValidationError("manual scheduled with nobody to ask", err,
					`state "scheduled" cannot be set manually from customer_approval: the customer's approval can only be given by the customer in the Customer Portal; cancel the change or re-schedule it instead`)
				f.expect(id, "after the refused manual scheduled", "CUSTOMER_APPROVAL", "authorize", "canceled")
				if approved, _ := f.customerOutcome(id); approved {
					t.Fatal("a refused manual scheduled stamped is_customer_approval_required")
				}
				// What staff have: cancel it.
				f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
			})
			t.Run("Customer Review", func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				if tc.prepare != nil {
					tc.prepare(f)
				}
				id := f.createWithProject(domain.ChangeRequestTypeNormal, tc.project, false, true)
				f.requestApprovalThenContactsLeave(id)
				f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
				f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
				f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
				// Nobody is asked, so a failed review is staff's to give (Roll
				// back stays on offer) but a passed one never is.
				f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "rollback", "canceled")
				if n := len(f.customerStages(id)); n != 0 {
					t.Fatalf("customer review stage provisioned: %d", n)
				}
				_, err := f.patchState(id, domain.ChangeRequestStateClosed)
				f.wantValidationError("manual closed with nobody to ask", err,
					`state "closed" cannot be set manually from customer_review: the customer's review can only be given by the customer in the Customer Portal; roll the change back or cancel it instead`)
				f.expect(id, "after the refused manual closed", "CUSTOMER_REVIEW", "rollback", "canceled")
				if _, reviewed := f.customerOutcome(id); reviewed {
					t.Fatal("a refused manual closed stamped is_customer_review_required")
				}
				f.step(id, domain.ChangeRequestStateRollback, "ROLLBACK")
			})
		})
	}
}

// A deactivated user (or a contact who is not registered) is not an eligible approver.
func TestChangeRequestFlowIntegration_CustomerGroupInactiveMemberSkipped(t *testing.T) {
	f := newCustomerGroupFlow(t)
	if _, err := f.scoped.Exec(f.sys, `UPDATE "user" SET is_active = false WHERE id = $1`, crScopeUserA2); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, false)
	f.requestApproval(id)
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED"})
}

// The Customer Group follows the contacts of the project the change was
// requested with, and only that project: the project is chosen in New and frozen
// by Request Approval, so a change in Customer Approval can never be re-pointed
// at another customer's contacts (the reason for the lock). What can still change
// under it is WHO the project's contacts are: resending the stored projectId (an
// accepted no-op write) re-derives the group, replacing a live stage whose
// contacts are out of date with one for who is registered now -- never two live
// stages; and cancelling the change cancels what is pending. (This test used to
// move the project around while the stage was live; the lock refuses that, which
// TestChangeRequestLockIntegration_ProjectIsFrozenAfterRequestApproval pins.)
func TestChangeRequestFlowIntegration_CustomerGroupFollowsTheProject(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, true)
	f.requestApproval(id)
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	st := f.customerStages(id)
	if len(st) != 1 {
		t.Fatalf("stages after Request Approval = %+v", st)
	}
	assertApprovers(t, "after Request Approval", st[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

	// Resending the same project, or any unrelated PATCH, changes nothing.
	f.setProject(id, crScopeProjectA)
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{Title: sp("renamed")}); err != nil {
		t.Fatalf("unrelated PATCH: %v", err)
	}
	if st := f.customerStages(id); len(st) != 1 || liveStages(st) != 1 {
		t.Fatalf("stages after idempotent resend = %+v", st)
	}

	// The project is frozen: another customer's project is refused and nothing
	// follows it.
	_, err := f.patch(id, domain.PatchChangeRequestRequest{ProjectID: sp(crScopeProjectB), DeploymentIDs: &[]string{}})
	f.wantValidationError("moving the project in Customer Approval", err, "projectId can no longer be changed")
	if st := f.customerStages(id); len(st) != 1 || liveStages(st) != 1 {
		t.Fatalf("stages after the refused move = %+v", st)
	}
	assertApprovers(t, "after the refused move", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

	// The contacts changed (Bob is no longer registered): resending the project
	// replaces the stage with one for who is left.
	f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED' WHERE email = $1`, crFlowEmail(crScopeUserBob))
	f.setProject(id, crScopeProjectA)
	st = f.customerStages(id)
	if len(st) != 2 || liveStages(st) != 1 {
		t.Fatalf("stages after the contact set changed = %+v, want 2 (1 live)", st)
	}
	assertApprovers(t, "old stage", st[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
	assertApprovers(t, "after Bob left", st[1].approvers, map[string]string{crScopeUserA1: "REQUESTED"})
	// A contact who registered later is picked up the same way.
	f.execSQL(`UPDATE project_contact SET state = 'REGISTERED' WHERE email = $1`, crFlowEmail(crScopeUserBob))
	f.setProject(id, crScopeProjectA)
	st = f.customerStages(id)
	if len(st) != 3 || liveStages(st) != 1 {
		t.Fatalf("stages after a contact registered = %+v, want 3 (1 live)", st)
	}
	assertApprovers(t, "after Bob registered again", st[2].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

	// Cancelling the change cancels what is pending.
	f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
	if liveStages(f.customerStages(id)) != 0 {
		t.Fatalf("a live customer stage remains on a cancelled change")
	}
}

// Customer A's change request is only ever put to customer A's contacts, and
// customer B's contacts can neither be asked nor decide: the isolation the
// derived Customer Group exists for.
func TestChangeRequestFlowIntegration_CustomerGroupIsolatesCustomers(t *testing.T) {
	f := newCustomerGroupFlow(t)
	idA := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, false)
	idB := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectB), true, false)
	f.requestApproval(idA)
	f.requestApproval(idB)
	assertApprovers(t, "customer A's stage", f.customerStages(idA)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	assertApprovers(t, "customer B's stage", f.customerStages(idB)[0].approvers, map[string]string{crScopeUserB1: "REQUESTED"})

	// Carol (customer B) cannot answer customer A's request, nor can A's people
	// answer B's -- and a refused answer changes nothing.
	f.wantForbidden("customer B deciding A's change", f.decide(idA, crScopeUserB1, "approved"), "only members of the customer group")
	f.wantForbidden("customer A deciding B's change", f.decide(idB, crScopeUserA1, "approved"), "only members of the customer group")
	f.expect(idA, "A after the refused decision", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.expect(idB, "B after the refused decision", "CUSTOMER_APPROVAL", "authorize", "canceled")
	if got := f.canDecideAs(idA, crScopeUserB1); len(got) != 0 {
		t.Fatalf("canDecide for customer B on A's change = %v", got)
	}
	if err := f.decide(idA, crScopeUserA2, "approved"); err != nil {
		t.Fatalf("customer A's own contact: %v", err)
	}
	f.expect(idA, "A after its own contact approved", "SCHEDULED", "implement", "canceled")
	f.expect(idB, "B untouched by A's decision", "CUSTOMER_APPROVAL", "authorize", "canceled")
}

// While a customer stage is live the manual transitions are refused with a
// readable 400, and nothing is stamped: the customer's own answer is the only
// way on (the same refusal as with nobody to ask), Rollback out of Customer
// Review is theirs too while they are asked, and Cancel stays.
func TestChangeRequestFlowIntegration_CustomerGroupRefusesManualTransition(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, true)
	f.requestApproval(id)
	_, err := f.patchState(id, domain.ChangeRequestStateScheduled)
	f.wantValidationMessage("manual scheduled", err,
		`state "scheduled" cannot be set manually from customer_approval: the customer's approval can only be given by the customer in the Customer Portal; cancel the change or re-schedule it instead`)
	f.expect(id, "after the refused scheduled", "CUSTOMER_APPROVAL", "authorize", "canceled")
	if approved, _ := f.customerOutcome(id); approved {
		t.Fatal("refused PATCH stamped is_customer_approval_required")
	}
	if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	_, err = f.patchState(id, domain.ChangeRequestStateClosed)
	f.wantValidationMessage("manual closed", err,
		`state "closed" cannot be set manually from customer_review: the customer's review can only be given by the customer in the Customer Portal; cancel the change instead`)
	f.expect(id, "after the refused close", "CUSTOMER_REVIEW", "canceled")
	// Rolling back is the members' call too (rejecting the review).
	_, err = f.patchState(id, domain.ChangeRequestStateRollback)
	f.wantValidationError("manual rollback", err, "approving or rejecting it in the change request's approvals")
	f.wantValidationError("manual rollback", err, `the customer group (the registered contacts of the change request's project)`)
	f.expect(id, "after the refused rollback", "CUSTOMER_REVIEW", "canceled")
	if _, reviewed := f.customerOutcome(id); reviewed {
		t.Fatal("refused PATCH stamped is_customer_review_required")
	}
	// Cancel is still allowed.
	f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
}

// scripts/csm-compose/seed-entity-service.sql ships two fixtures for the local
// stack: CHG-FIXED-007 (Customer Approval) and CHG-FIXED-008 (Customer Review),
// each on project 401, whose registered contacts (the Customer Group) are the
// two customer personas dave.mendis and erin.jayawardena (external users, not
// jane.doe / john.smith), with its stage provisioned for them. Read-only here
// (deciding would consume them); skipped when the database was not built from
// the seed.
func TestChangeRequestFlowIntegration_SeedCustomerGroupFixtures(t *testing.T) {
	f := newCRFlow(t)
	for _, tc := range []struct {
		id, number, state, stage string
	}{
		{"00000000-0000-0000-0000-000000001303", "CHG-FIXED-007", "customer_approval", stageCustApproval},
		{"00000000-0000-0000-0000-000000001304", "CHG-FIXED-008", "customer_review", stageCustReview},
	} {
		var n string
		if err := f.scoped.QueryRow(f.sys, `SELECT number FROM work_item WHERE id = $1`, tc.id).Scan(&n); err != nil {
			t.Skipf("seed fixture %s not loaded: %v", tc.number, err)
		}
		cr := f.get(tc.id)
		if cr.State == nil || *cr.State != tc.state {
			t.Fatalf("%s state = %v, want %s", tc.number, cr.State, tc.state)
		}
		if got := contactNames(cr.CustomerContacts); strings.Join(got, ",") != "Dave Mendis,Erin Jayawardena" {
			t.Fatalf("%s customerContacts = %v, want the project's Dave Mendis,Erin Jayawardena", tc.number, got)
		}
		// A live customer request leaves only Cancel -- and, at Customer
		// Approval, Re-schedule ("authorize").
		wantLegal := []string{"canceled"}
		if tc.state == "customer_approval" {
			wantLegal = []string{"authorize", "canceled"}
		}
		assertStates(t, tc.number+" legalNextStates", cr.LegalNextStates, wantLegal...)
		ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true, ViewerEmail: "dave.mendis@example.com"})
		view, err := f.repo.GetChangeRequestApprovals(ctx, tc.id)
		if err != nil {
			t.Fatalf("%s approvals: %v", tc.number, err)
		}
		if len(view.Approvals) != 1 || view.Approvals[0].Stage != tc.stage || view.Approvals[0].ApproverName != "Customer Group" {
			t.Fatalf("%s approvals = %+v", tc.number, view.Approvals)
		}
		if len(view.Approvals[0].Approvers) != 2 {
			t.Fatalf("%s approvers = %+v, want dave.mendis and erin.jayawardena", tc.number, view.Approvals[0].Approvers)
		}
		can := 0
		for _, ap := range view.Approvals[0].Approvers {
			if ap.CanDecide {
				can++
				if ap.Name != "Dave Mendis" {
					t.Fatalf("%s canDecide on %q, want only the viewer (Dave Mendis)", tc.number, ap.Name)
				}
			}
		}
		if can != 1 {
			t.Fatalf("%s canDecide rows for dave.mendis = %d, want 1", tc.number, can)
		}
	}
}

// A customer_group_id still stored on a change request (from before the group
// was derived) is no longer used for approvals: a project without registered
// contacts gets no customer stage, whoever the old group's members are -- and
// nobody answers for the customer.
func TestChangeRequestFlowIntegration_StoredCustomerGroupIsNoLongerUsedForApprovals(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
	// seededGroupID has real members (the seeded users); store it as the legacy group.
	f.execSQL(`UPDATE change_request SET customer_group_id = $1 WHERE id = $2`, seededGroupID, id)
	// The legacy group does not make anybody askable either: Request Approval is
	// refused for the project with no contacts, whoever the old group's members are.
	_, err := f.patchState(id, domain.ChangeRequestStateAssess)
	f.wantExact("Request Approval with only a legacy group", err, nobodyMsgApproval)
	// What is left is the residual edge: the contacts are gone after Request Approval.
	f.requestApprovalThenContactsLeave(id)
	f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
	if n := len(f.customerStages(id)); n != 0 {
		t.Fatalf("a customer stage was provisioned from the legacy group: %+v", f.customerStages(id))
	}
	_, err = f.patchState(id, domain.ChangeRequestStateScheduled)
	f.wantValidationError("manual scheduled", err, "can only be given by the customer in the Customer Portal")
	f.expect(id, "after the refused manual scheduled", "CUSTOMER_APPROVAL", "authorize", "canceled")
}
