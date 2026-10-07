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
	"encoding/json"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// What a customer reads of the approvals, and what an answer is given for. Same
// harness as the other customer-outcome tests (newCustomerGroupFlow, DSN-gated).

// approveAsFor is the customer userID's approval, given for the window they say
// they were shown.
func (f *crFlow) approveAsFor(id, userID string, expectedStart, expectedEnd *string) (domain.ChangeRequest, error) {
	return f.patchAsContact(id, userID, domain.PatchChangeRequestRequest{
		IsCustomerApproved: boolp(true), ExpectedPlannedStartOn: expectedStart, ExpectedPlannedEndOn: expectedEnd})
}

// An answer that names the window it was given for is only recorded while that
// is still the window: a page opened before the change was re-scheduled (and
// approved again, so that the customer is asked afresh) cannot approve a time its
// reader never saw. Nothing is recorded when it is refused, an answer that names
// nothing is recorded as it always was, and a window named in another spelling of
// the same instants is the same window.
func TestChangeRequestCustomerPrivacyIntegration_AnswerIsBoundToTheWindowSeen(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)

	const changedMsg = "the planned implementation time of this change request changed after you opened it"
	for name, w := range map[string][2]*string{
		"a start that is not the stored one": {sp(rsStart2), nil},
		"an end that is not the stored one":  {nil, sp(rsEnd2)},
		"both, both wrong":                   {sp(rsStart2), sp(rsEnd2)},
		"the start right, the end wrong":     {sp(rsStart1), sp(rsEnd2)},
	} {
		_, err := f.approveAsFor(id, crScopeUserA1, w[0], w[1])
		f.wantConflictContaining(name, err, changedMsg, "read it again")
	}
	_, err := f.approveAsFor(id, crScopeUserA1, sp("next tuesday"), nil)
	f.wantValidationError("an expected window that is no date", err, "expectedPlannedStartOn must be a date-time")
	f.expect(id, "after the refused answers", "CUSTOMER_APPROVAL", "authorize", "canceled")
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	if a, _ := f.customerOutcome(id); a {
		t.Fatal("a refused answer stamped the customer's approval")
	}

	// A change request with no window at all says so, rather than "not set to not set".
	f.execSQL(`UPDATE change_request SET start_on = NULL, end_on = NULL WHERE id = $1`, id)
	_, err = f.approveAsFor(id, crScopeUserA1, sp(rsStart1), nil)
	f.wantConflictContaining("a window that is gone", err, changedMsg, "(it is now no planned time is set)")
	f.setPlanned(id, rsStart1, rsEnd1)

	// The window moves: Alice proposes, the CAB approves it, the customer is asked
	// again. Bob's page still shows the old window.
	if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)}); err != nil {
		t.Fatalf("proposal: %v", err)
	}
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval of the new plan: %v", err)
	}
	f.expect(id, "asked again", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantCanAnswer(id, "asked again", true, crScopeUserA1, crScopeUserA2)

	_, err = f.approveAsFor(id, crScopeUserA2, sp(rsStart1), sp(rsEnd1))
	f.wantConflictContaining("Bob's stale page", err, changedMsg, "2030-03-08T09:00:00Z to 2030-03-08T11:00:00Z")
	f.expect(id, "after the stale answer", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantPlanned(id, "after the stale answer", rsStart2, rsEnd2)

	// The window he reads now, in the other layouts the API accepts, is the window.
	if _, err := f.approveAsFor(id, crScopeUserA2, sp("2030-03-08 09:00:00"), sp("2030-03-08T16:30:00+05:30")); err != nil {
		t.Fatalf("Bob approving the window he was shown: %v", err)
	}
	f.expect(id, "after the answer for the current window", "SCHEDULED", "implement", "canceled")
	f.wantPlanned(id, "after the answer for the current window", rsStart2, rsEnd2)
}

// The expected window belongs to a customer's answer and nothing else: it is not
// a way to run a precondition on a WSO2 user's PATCH, and a customer cannot send
// it alone or beside a proposed time.
func TestChangeRequestCustomerPrivacyIntegration_ExpectedWindowOnlyAccompaniesAnAnswer(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)

	_, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{ExpectedPlannedStartOn: sp(rsStart1)})
	f.wantValidationError("the expected window alone", err, "go with the customer's approval or review")
	_, err = f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2), ExpectedPlannedStartOn: sp(rsStart1)})
	f.wantValidationError("the expected window beside a proposal", err, "go with the customer's approval or review")
	_, err = f.patch(id, domain.PatchChangeRequestRequest{Description: sp("x"), ExpectedPlannedStartOn: sp(rsStart1)})
	f.wantValidationError("a WSO2 user's PATCH", err, "can only accompany a customer's approval or review")
	f.wantPlanned(id, "after the refused requests", rsStart1, rsEnd1)
	f.expect(id, "after the refused requests", "CUSTOMER_APPROVAL", "authorize", "canceled")
}

// A customer reads the approvals of their own change request, and what a customer
// is shown of WSO2's internal stages is that they exist and where they stand --
// not who sits on them. The customer's own stage is whole; staff see everything.
func TestChangeRequestCustomerPrivacyIntegration_ApprovalsHideWhoApprovesInternally(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)

	asCustomer, err := f.repo.GetChangeRequestApprovals(asContact(crScopeUserA1), id)
	if err != nil {
		t.Fatalf("approvals as a customer: %v", err)
	}
	if len(asCustomer.Approvals) != 3 {
		t.Fatalf("a customer sees %d stages, want Peer, CAB and Customer Approval: %+v", len(asCustomer.Approvals), asCustomer.Approvals)
	}
	for i, wantStage := range []string{"Peer Approval", "CAB Approval"} {
		a := asCustomer.Approvals[i]
		if a.Stage != wantStage || a.Status != domain.ChangeRequestApprovalStatusApproved {
			t.Errorf("internal stage %d = %q %q, want %q APPROVED (the stage and where it stands stay)", i, a.Stage, a.Status, wantStage)
		}
		if a.ApproverName != "" || a.AssignmentGroup != nil || len(a.Approvers) != 0 {
			t.Errorf("internal stage %q still names who approves it: %+v", a.Stage, a)
		}
	}
	own := asCustomer.Approvals[2]
	if own.Stage != stageCustApproval || own.ApproverName != "Customer Group" || len(own.Approvers) != 2 {
		t.Fatalf("the customer's own stage = %+v, want it whole (the two contacts asked)", own)
	}
	canDecide := false
	for _, ap := range own.Approvers {
		canDecide = canDecide || ap.CanDecide
	}
	if !canDecide {
		t.Errorf("Alice's own pending approval does not say she can decide it: %+v", own.Approvers)
	}
	raw, err := json.Marshal(asCustomer)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, internal := range []string{crFlowPeerAID, crCABMemberUserID1, crCABMemberUserID2} {
		if strings.Contains(string(raw), internal) {
			t.Errorf("the customer's approvals carry the internal user id %s: %s", internal, raw)
		}
	}

	// Staff still see who approves what.
	asStaff := f.approvalsAs(id, crFlowPeerAID)
	if len(asStaff.Approvals[0].Approvers) == 0 || asStaff.Approvals[0].ApproverName == "" || len(asStaff.Approvals[1].Approvers) == 0 {
		t.Fatalf("a staff member no longer sees the internal approvers: %+v", asStaff.Approvals)
	}
}
