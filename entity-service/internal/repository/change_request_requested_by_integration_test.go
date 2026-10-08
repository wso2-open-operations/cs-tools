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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The nobody-to-ask refusal judges the creators AS THEY WILL STAND once the PATCH is written: the
// requester never approves their own change, so a requestedById that names the project's only
// registered contact would leave nobody to ask. Request Approval, a Re-schedule and a requestedById
// change after Request Approval are all judged with the request's own requestedById in place of the
// stored one, and a change that would strand a customer gate still ahead is refused with the same words
// Request Approval uses. Same harness as the other nobody-to-ask tests (the project has exactly one
// registered contact, the stand-in, so naming them as the requester leaves nobody).

// requestedBy is change_request.requested_by_user_id as stored ("" when NULL).
func (f *crFlow) requestedBy(id string) string {
	f.t.Helper()
	var rb *string
	if err := f.scoped.QueryRow(f.sys, `SELECT requested_by_user_id::text FROM change_request WHERE id = $1`, id).Scan(&rb); err != nil {
		f.t.Fatalf("read requested_by_user_id: %v", err)
	}
	if rb == nil {
		return ""
	}
	return *rb
}

// asRequester is the PATCH value requestedById names: a user id, or nil to clear it.
func asRequester(userID *string) **string { return &userID }

func TestChangeRequestNobodyToAskIntegration_TheRequesterIsJudgedAsItWillStand(t *testing.T) {
	standIn := crStandInUserID
	other := crFlowPeerAID // staff, not a contact of the project

	// attempt names the stand-in as the requester: refused with msg, nothing written; naming somebody
	// who is no contact, clearing the field, and resending the stored one are accepted.
	attempt := func(t *testing.T, f *crFlow, id, when, msg string) {
		t.Helper()
		before := f.snap(id)
		stored := f.requestedBy(id)
		_, err := f.patch(id, domain.PatchChangeRequestRequest{RequestedByID: asRequester(&standIn), Title: sp("must not be written")})
		f.wantExact("naming the only contact as the requester "+when, err, msg)
		f.wantRefusedSame("naming the only contact as the requester "+when, id, before, err)
		if got := f.requestedBy(id); got != stored {
			t.Fatalf("a refused request changed the requester %s: %q -> %q", when, stored, got)
		}
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{RequestedByID: asRequester(&other)}); err != nil {
			t.Fatalf("naming somebody who is no contact %s: %v", when, err)
		}
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{RequestedByID: asRequester(nil)}); err != nil {
			t.Fatalf("clearing the requester %s: %v", when, err)
		}
		// A resend of the stored value is never a change, whatever the contacts have become.
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{RequestedByID: asRequester(sp(crFlowCreatorID))}); err != nil {
			t.Fatalf("naming the creator as the requester again %s: %v", when, err)
		}
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{RequestedByID: asRequester(sp(crFlowCreatorID))}); err != nil {
			t.Fatalf("resending the stored requester %s: %v", when, err)
		}
	}

	t.Run("Request Approval carrying a requestedById", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		f.standInContact(crScopeProjectC)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
		before := f.snap(id)
		// With the stored requester the stand-in is askable (Request Approval would be accepted)...
		_, err := f.patch(id, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateAssess), RequestedByID: asRequester(&standIn)})
		// ...but as the requester they are the creator, and nobody is left.
		f.wantExact("Request Approval naming the only contact as the requester", err, nobodyMsgApproval)
		f.wantRefusedSame("Request Approval naming the only contact as the requester", id, before, err)
		if got := f.requestedBy(id); got != crFlowCreatorID {
			t.Fatalf("the refused Request Approval wrote the requester: %q", got)
		}
		// Naming anybody else, or nobody, is Request Approval as ever.
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateAssess), RequestedByID: asRequester(&other)}); err != nil {
			t.Fatalf("Request Approval naming another requester: %v", err)
		}
		f.expect(id, "after Request Approval", "ASSESS", "canceled")
		if got := f.requestedBy(id); got != other {
			t.Fatalf("requester = %q, want the one Request Approval carried", got)
		}
	})

	t.Run("the approval gate still ahead, at each state before it", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		f.standInContact(crScopeProjectC)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.requestApproval(id)
		f.expect(id, "in Assess", "ASSESS", "canceled")
		attempt(t, f, id, "in Assess", nobodyMsgApproval)
		if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
			t.Fatalf("peer approval: %v", err)
		}
		f.expect(id, "in Authorize", "AUTHORIZE", "canceled")
		attempt(t, f, id, "in Authorize", nobodyMsgApproval)
		if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
			t.Fatalf("CAB approval: %v", err)
		}
		f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
		attempt(t, f, id, "in Customer Approval, where the customer is being asked", nobodyMsgApproval)

		// A Re-schedule carrying a requestedById is judged the same way.
		_, err := f.patch(id, domain.PatchChangeRequestRequest{
			State: stateptr(domain.ChangeRequestStateAuthorize), PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2), RequestedByID: asRequester(&standIn)})
		f.wantExact("a Re-schedule naming the only contact as the requester", err, nobodyMsgApproval)
		f.wantPlanned(id, "after the refused Re-schedule", rsStart1, rsEnd1)
		if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
			t.Fatalf("a Re-schedule that names no requester: %v", err)
		}
	})

	t.Run("the review gate still ahead, with the boxes it names", func(t *testing.T) {
		for _, tc := range []struct {
			name             string
			approval, review bool
			msg              string
		}{
			{"review only", false, true, nobodyMsgReview},
			{"both boxes", true, true, nobodyMsgBoth},
		} {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				f.standInContact(crScopeProjectC)
				id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), tc.approval, tc.review)
				f.requestApproval(id)
				if tc.approval {
					// Both gates are ahead in Assess; once the approval gate is passed only the review is.
					attempt(t, f, id, "in Assess", tc.msg)
					f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
					attempt(t, f, id, "in Customer Approval", tc.msg)
					// (The customer's own answer is not this test's business: the gate is passed directly.)
					f.execSQL(`UPDATE change_request SET state = 'SCHEDULED' WHERE id = $1`, id)
				} else {
					attempt(t, f, id, "in Assess", tc.msg)
					f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
				}
				// Once the approval gate is passed only the review box is judged.
				reviewOnly := nobodyMsgReview
				attempt(t, f, id, "in Scheduled", reviewOnly)
				f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
				attempt(t, f, id, "in Implement", reviewOnly)
				f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
				attempt(t, f, id, "in Review", reviewOnly)
				f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
				attempt(t, f, id, "in Customer Review, where the customer is being asked", reviewOnly)
			})
		}
	})

	t.Run("a gate that is passed, or a change that is final, is not judged", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		f.standInContact(crScopeProjectC)
		// An approval-only change past its gate: Scheduled, Implement, Review, Closed.
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.execSQL(`UPDATE change_request SET state = 'SCHEDULED' WHERE id = $1`, id)
		for _, state := range []string{"SCHEDULED", "IMPLEMENT", "REVIEW", "CLOSED", "CANCELED"} {
			f.setState(id, state)
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{RequestedByID: asRequester(&standIn)}); err != nil {
				t.Fatalf("naming the only contact as the requester in %s, past the only gate the change has: %v", state, err)
			}
			if got := f.requestedBy(id); got != standIn {
				t.Fatalf("requester in %s = %q, want the contact", state, got)
			}
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{RequestedByID: asRequester(sp(crFlowCreatorID))}); err != nil {
				t.Fatalf("naming the creator again in %s: %v", state, err)
			}
		}
		// A change that needs no customer is never judged either (in New, or after).
		plain := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), false, false)
		if _, err := f.patch(plain, domain.PatchChangeRequestRequest{RequestedByID: asRequester(&standIn)}); err != nil {
			t.Fatalf("naming the only contact as the requester of a change that needs no customer: %v", err)
		}
		f.requestApproval(plain)
		if _, err := f.patch(plain, domain.PatchChangeRequestRequest{RequestedByID: asRequester(&other)}); err != nil {
			t.Fatalf("changing the requester of a change that needs no customer after Request Approval: %v", err)
		}
	})

	// A resend of the stored requester is no change, whatever the project's contacts have become since:
	// never re-judged. A CHANGE of the requester while nobody can be asked leaves nobody to ask and is refused.
	t.Run("a resend is never re-judged, a change with nobody left to ask is", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		leave := f.standInContact(crScopeProjectC)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
		f.requestApproval(id)
		f.expect(id, "in Assess", "ASSESS", "canceled")
		leave() // every registered contact is gone after approval was requested
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{RequestedByID: asRequester(sp(crFlowCreatorID)), Title: sp("a whole-form resend")}); err != nil {
			t.Fatalf("resending the stored requester when nobody can be asked any more: %v", err)
		}
		_, err := f.patch(id, domain.PatchChangeRequestRequest{RequestedByID: asRequester(&other)})
		f.wantExact("changing the requester when nobody can be asked", err, nobodyMsgApproval)
		if got := f.requestedBy(id); got != crFlowCreatorID {
			t.Fatalf("a refused change wrote the requester: %q", got)
		}
	})

	// A change that already waits in Customer Approval with the box still false (a migrated or older row): the
	// requester edit is not a gate the box controls, but a Re-schedule asks the customers anyway, and is judged
	// with the requester it carries.
	t.Run("a Re-schedule of a legacy row with the box false carrying a requestedById", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		f.standInContact(crScopeProjectC)
		id := f.legacyInCustomerApproval(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), nil, false)
		f.setPlanned(id, rsStart1, rsEnd1)
		_, err := f.patch(id, domain.PatchChangeRequestRequest{
			State: stateptr(domain.ChangeRequestStateAuthorize), PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2), RequestedByID: asRequester(&standIn)})
		f.wantExact("a Re-schedule naming the only contact as the requester", err, nobodyMsgApproval)
		f.wantPlanned(id, "after the refused Re-schedule", rsStart1, rsEnd1)
		if got := f.requestedBy(id); got != crFlowCreatorID {
			t.Fatalf("a refused Re-schedule wrote the requester: %q", got)
		}
		if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
			t.Fatalf("the same Re-schedule without the requester: %v", err)
		}
	})

	t.Run("a change in New is left to Request Approval", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		f.standInContact(crScopeProjectC)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
		// In New the requester may be edited freely (the edit dialog's own flow): Request Approval is where it is judged.
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{RequestedByID: asRequester(&standIn)}); err != nil {
			t.Fatalf("editing the requester in New: %v", err)
		}
		_, err := f.patchState(id, domain.ChangeRequestStateAssess)
		f.wantExact("Request Approval with the only contact as the stored requester", err, nobodyMsgApproval)
	})
}
