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
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// customerCanAnswer on the change request detail: the viewer-specific "may I
// answer this now" the customer portal offers Approve / Reject / Propose from.
// Same harness as the customer-outcome tests (newCustomerGroupFlow, DSN-gated by
// CHANGE_REQUEST_TEST_DSN; the contacts of project A are Alice and Bob, Carol is
// a contact of project B, Sam holds only SECURITY_CONTACT, Ivy is only invited,
// Ian's user is inactive), with every read made as an EXTERNAL caller.
//
// Under a superuser DSN (no row-level security) the repository's own checks
// give a stranger a present-but-false answer; under a non-superuser role the
// database hides the change request from a non-member (404) before any answer
// is computed. Either way a stranger is never told true, which is what the
// stranger helpers below accept.

// getAsContact is GET /change-requests/{id} sent by the customer userID.
func (f *crFlow) getAsContact(id, userID string) (domain.ChangeRequest, error) {
	return f.repo.GetChangeRequestByID(asContact(userID), id)
}

// canAnswerAs is the customerCanAnswer the customer userID is told for the
// change request. A customer is always told something: an unreadable change
// request or an absent value is a failure here.
func (f *crFlow) canAnswerAs(id, userID string) bool {
	f.t.Helper()
	cr, err := f.getAsContact(id, userID)
	if err != nil {
		f.t.Fatalf("GET as %s: %v", userID, err)
	}
	if cr.CustomerCanAnswer == nil {
		f.t.Fatalf("customerCanAnswer is absent for the customer %s (state %v), want true or false", userID, cr.State)
	}
	return *cr.CustomerCanAnswer
}

// wantCanAnswer asserts what each of the customers is told.
func (f *crFlow) wantCanAnswer(id, when string, want bool, users ...string) {
	f.t.Helper()
	for _, u := range users {
		if got := f.canAnswerAs(id, u); got != want {
			f.t.Fatalf("customerCanAnswer for %s %s = %v, want %v", u, when, got, want)
		}
	}
}

// strangerCanAnswer is what a caller who is not a registered contact of the
// change request's project is told: false, or nothing at all because the
// database hides the change request from them (404). Anything else is a failure.
func (f *crFlow) strangerCanAnswer(ctx context.Context, what, id string) bool {
	f.t.Helper()
	cr, err := f.repo.GetChangeRequestByID(ctx, id)
	if err != nil {
		var nf *apierror.NotFoundError
		if !errors.As(err, &nf) {
			f.t.Fatalf("%s: GET err = %v (%T), want a result or *apierror.NotFoundError", what, err, err)
		}
		return false
	}
	if cr.CustomerCanAnswer == nil {
		f.t.Fatalf("%s: customerCanAnswer is absent for an external caller, want false", what)
	}
	return *cr.CustomerCanAnswer
}

// wantApproveRefused asserts the customer's approve PATCH is refused -- the
// oracle for a false answer: what the portal would not offer is not accepted.
func (f *crFlow) wantApproveRefused(what, id, userID string) {
	f.t.Helper()
	if _, err := f.approveAs(id, userID, true); err == nil {
		f.t.Fatalf("%s: customerCanAnswer said false but approving as %s was accepted", what, userID)
	}
}

// wantProposeRefused is the proposal's twin of wantApproveRefused: customerCanAnswer
// is also the "may propose a new implementation time" signal, so a caller told
// false has a proposed window refused too, and it changes nothing (the change
// keeps its window and the customer's request stays pending for those it was
// sent to). contains, when given, is what the refusal says.
func (f *crFlow) wantProposeRefused(what, id, userID string, contains ...string) {
	f.t.Helper()
	before := f.get(id)
	stages := f.stageLabels(id)
	_, err := f.patchAsContact(id, userID, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)})
	if err == nil {
		f.t.Fatalf("%s: customerCanAnswer said false but proposing a new time as %s was accepted", what, userID)
	}
	for _, c := range contains {
		if !strings.Contains(err.Error(), c) {
			f.t.Fatalf("%s: the refusal %q should contain %q", what, err.Error(), c)
		}
	}
	after := f.get(id)
	if before.PlannedStartOn == nil || after.PlannedStartOn == nil || *before.PlannedStartOn != *after.PlannedStartOn ||
		before.State == nil || after.State == nil || *before.State != *after.State || f.stageLabels(id) != stages {
		f.t.Fatalf("%s: a refused proposal changed the change request (state %v -> %v, start %v -> %v, stages %s -> %s)",
			what, before.State, after.State, before.PlannedStartOn, after.PlannedStartOn, stages, f.stageLabels(id))
	}
}

// windowOf is the planned window's length.
func windowOf(t *testing.T, cr domain.ChangeRequest) time.Duration {
	t.Helper()
	if cr.PlannedStartOn == nil || cr.PlannedEndOn == nil {
		t.Fatalf("planned window = %v .. %v, want both", cr.PlannedStartOn, cr.PlannedEndOn)
	}
	start, err := time.Parse(time.RFC3339, *cr.PlannedStartOn)
	if err != nil {
		t.Fatalf("planned start %q: %v", *cr.PlannedStartOn, err)
	}
	end, err := time.Parse(time.RFC3339, *cr.PlannedEndOn)
	if err != nil {
		t.Fatalf("planned end %q: %v", *cr.PlannedEndOn, err)
	}
	return end.Sub(start)
}

// A Normal change with both gates, from New to Closed: invisible to the customer
// until their approval is first asked, then true for both contacts exactly while the
// customer's request is pending, and false again right after the answer -- in the
// PATCH receipt too. Staff views never carry it.
func TestChangeRequestCustomerCanAnswerIntegration_Lifecycle(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)

	// Until the customer's approval is first asked (New, Assess, Authorize) the
	// change request was never designated to them: they are not told false, they
	// are told nothing, because it does not exist for them (404).
	f.wantHiddenFrom(id, "in New", crScopeUserA1, crScopeUserA2)
	f.requestApproval(id)
	f.wantHiddenFrom(id, "in Assess", crScopeUserA1, crScopeUserA2)
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	f.wantHiddenFrom(id, "in Authorize", crScopeUserA1, crScopeUserA2)
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")

	// Customer Approval: both contacts were asked.
	f.wantCanAnswer(id, "in Customer Approval", true, crScopeUserA1, crScopeUserA2)

	// Staff views carry no answer: an internal identity, an unrestricted one that
	// happens to hold a contact's email, and staff who also hold an external record.
	staffViews := map[string]context.Context{
		"the system identity":           f.sys,
		"an unrestricted caller":        repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true, ViewerEmail: crFlowEmail(crScopeUserA1)}),
		"staff with an external record": repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: crFlowEmail(crScopeUserA1), HasInternalAccess: true}),
	}
	for name, ctx := range staffViews {
		cr, err := f.repo.GetChangeRequestByID(ctx, id)
		if err != nil {
			t.Fatalf("GET as %s: %v", name, err)
		}
		if cr.CustomerCanAnswer != nil {
			t.Fatalf("customerCanAnswer for %s = %v, want it absent", name, *cr.CustomerCanAnswer)
		}
		raw, err := json.Marshal(cr)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(raw), "customerCanAnswer") {
			t.Fatalf("the staff view's JSON mentions customerCanAnswer: %s", raw)
		}
	}
	// A customer's view carries it, in the JSON as well.
	cr, err := f.getAsContact(id, crScopeUserA1)
	if err != nil {
		t.Fatalf("GET as Alice: %v", err)
	}
	raw, err := json.Marshal(cr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"customerCanAnswer":true`) {
		t.Fatalf("the customer's JSON lacks customerCanAnswer:true: %s", raw)
	}

	// Alice approves: the receipt already says she can no longer answer; Bob's
	// request was cancelled and the change moved on.
	receipt, err := f.approveAs(id, crScopeUserA1, true)
	if err != nil {
		t.Fatalf("Alice approves: %v", err)
	}
	if receipt.CustomerCanAnswer == nil || *receipt.CustomerCanAnswer {
		t.Fatalf("customerCanAnswer in the PATCH receipt after approving = %v, want false", receipt.CustomerCanAnswer)
	}
	f.wantCanAnswer(id, "after Alice approved (Scheduled)", false, crScopeUserA1, crScopeUserA2)

	// Review: nothing to answer until Customer Review, then both again.
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.wantCanAnswer(id, "in Implement", false, crScopeUserA1, crScopeUserA2)
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	f.wantCanAnswer(id, "in Review", false, crScopeUserA1, crScopeUserA2)
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	f.wantCanAnswer(id, "in Customer Review", true, crScopeUserA1, crScopeUserA2)

	receipt, err = f.reviewAs(id, crScopeUserA2, true)
	if err != nil {
		t.Fatalf("Bob confirms the review: %v", err)
	}
	if receipt.CustomerCanAnswer == nil || *receipt.CustomerCanAnswer {
		t.Fatalf("customerCanAnswer in the PATCH receipt after the review = %v, want false", receipt.CustomerCanAnswer)
	}
	f.expect(id, "after the customer review", "CLOSED")
	f.wantCanAnswer(id, "when Closed", false, crScopeUserA1, crScopeUserA2)
}

// Rejections end the request too: the answer is false for the rejecting contact
// and for the one whose request the rejection cancelled -- through either door.
func TestChangeRequestCustomerCanAnswerIntegration_FalseAfterAnyAnswer(t *testing.T) {
	t.Run("customer approval rejected through PATCH", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.driveToCustomerApproval(id)
		f.wantCanAnswer(id, "before the rejection", true, crScopeUserA1, crScopeUserA2)
		if _, err := f.approveAs(id, crScopeUserA2, false); err != nil {
			t.Fatalf("Bob rejects: %v", err)
		}
		f.expect(id, "after the rejection", "CANCELED")
		f.wantCanAnswer(id, "after the rejection (Canceled)", false, crScopeUserA1, crScopeUserA2)
	})
	t.Run("customer approval decided through the approvals route", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.driveToCustomerApproval(id)
		if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
			t.Fatalf("Alice approves: %v", err)
		}
		f.expect(id, "after the approval", "SCHEDULED", "implement", "canceled")
		f.wantCanAnswer(id, "after the approval (Scheduled)", false, crScopeUserA1, crScopeUserA2)
	})
	t.Run("customer review rejected rolls back", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.driveToCustomerReview(id)
		f.wantCanAnswer(id, "in Customer Review", true, crScopeUserA1, crScopeUserA2)
		if _, err := f.reviewAs(id, crScopeUserA1, false); err != nil {
			t.Fatalf("Alice fails the review: %v", err)
		}
		f.expect(id, "after the failed review", "ROLLBACK")
		f.wantCanAnswer(id, "after the failed review (Rollback)", false, crScopeUserA1, crScopeUserA2)
	})
}

// Who is told true: only a contact who was asked. Everyone else is told false
// (or sees nothing), and the answer is the oracle for what the PATCH accepts: each
// caller told false has their approval refused, the one told true gets it accepted.
func TestChangeRequestCustomerCanAnswerIntegration_WhoMayAnswer(t *testing.T) {
	f := newCustomerGroupFlow(t)
	f.registerContact(crScopeProjectA, crScopeAccountID, crFlowCreatorID)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers,
		map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED", crFlowCreatorID: "CANCELLED"})

	strangers := []struct {
		name string
		ctx  context.Context
		user string // for the PATCH oracle
	}{
		{"a contact of ANOTHER project", asContact(crScopeUserB1), crScopeUserB1},
		{"a contact who was only invited", asContact(crScopeUserInvited), crScopeUserInvited},
		{"an internal user who is not a contact", asContact(crFlowPeerAID), crFlowPeerAID},
		{"a contact holding no PORTAL_USER role", asContact(crScopeUserSecurity), crScopeUserSecurity},
		{"a registered contact whose user is inactive", asContact(crScopeUserInactive), crScopeUserInactive},
		{"the creator, although a registered contact", asContact(crFlowCreatorID), crFlowCreatorID},
		{"an email with no user record", repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: "nobody@example.com"}), ""},
	}
	for _, s := range strangers {
		if f.strangerCanAnswer(s.ctx, s.name, id) {
			t.Fatalf("%s is told customerCanAnswer = true", s.name)
		}
		if s.user != "" {
			f.wantApproveRefused(s.name, id, s.user)
			f.wantProposeRefused(s.name, id, s.user)
		}
	}

	// The creator holds a REQUESTED row (somebody put them back): still not told
	// true, still refused -- the creator never approves their own change.
	f.execSQL(`UPDATE approval_stage_approver SET state = 'REQUESTED' WHERE work_item_id = $1 AND approver_user_id = $2`, id, crFlowCreatorID)
	if f.strangerCanAnswer(asContact(crFlowCreatorID), "the creator with a requested row", id) {
		t.Fatal("the creator holding a requested row is told customerCanAnswer = true")
	}
	_, err := f.approveAs(id, crFlowCreatorID, true)
	f.wantForbidden("the creator approving", err, "creator of a change request cannot approve it")
	f.execSQL(`UPDATE approval_stage_approver SET state = 'CANCELLED' WHERE work_item_id = $1 AND approver_user_id = $2`, id, crFlowCreatorID)

	// A contact who was asked but is no longer a REGISTERED contact (Bob, taken
	// back to invited): the request is still on file, but the answer would be
	// refused, so he is not told true.
	f.execSQL(`UPDATE project_contact SET state = 'INVITED'::project_contact_state_enum WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, crScopeProjectA, crFlowEmail(crScopeUserA2))
	if f.strangerCanAnswer(asContact(crScopeUserA2), "a contact no longer registered", id) {
		t.Fatal("a contact who is no longer registered is told customerCanAnswer = true")
	}
	f.wantApproveRefused("a contact no longer registered", id, crScopeUserA2)
	f.wantProposeRefused("a contact no longer registered", id, crScopeUserA2)
	f.execSQL(`UPDATE project_contact SET state = 'REGISTERED'::project_contact_state_enum WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, crScopeProjectA, crFlowEmail(crScopeUserA2))

	// Nothing above changed the change request, and the real contacts are told true.
	f.expect(id, "after the refused answers", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantCanAnswer(id, "for the contacts who were asked", true, crScopeUserA1, crScopeUserA2)
	receipt, err := f.approveAs(id, crScopeUserA1, true)
	if err != nil {
		t.Fatalf("Alice, told true, approving: %v", err)
	}
	if receipt.CustomerCanAnswer == nil || *receipt.CustomerCanAnswer {
		t.Fatalf("customerCanAnswer in the receipt = %v, want false", receipt.CustomerCanAnswer)
	}
}

// A contact the request was never sent to is not told true, whether they registered
// after it went out or no request was ever made.
func TestChangeRequestCustomerCanAnswerIntegration_NobodyWasAsked(t *testing.T) {
	t.Run("a contact registered after the request went out", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.driveToCustomerApproval(id)
		const zed = "3bbbbbbb-0000-0000-0000-0000000000a9"
		f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user, user_type)
		           VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, 'Zed Late', 'Zed', 'Late', $2, true, false, 'EXTERNAL'::user_type_enum)`, zed, crFlowEmail(zed))
		f.registerContact(crScopeProjectA, crScopeAccountID, zed)

		f.wantCanAnswer(id, "for the contact registered afterwards", false, zed)
		f.wantApproveRefused("a late contact", id, zed)
		// ...and proposing a new time is the answer's twin: it would cancel the
		// pending approvals of the contacts who WERE asked, so it is theirs alone.
		f.setPlanned(id, rsStart1, rsEnd1)
		f.wantProposeRefused("a late contact", id, zed, "only members of the customer group", "have been asked")
		f.wantCanAnswer(id, "for the contacts who were asked", true, crScopeUserA1, crScopeUserA2)
		assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	})
	t.Run("the change reached Customer Approval with nobody to ask, strict: invisible", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		f.useVisibility(visStrictSinceLongAgo())
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
		f.requestApprovalThenContactsLeave(id)
		f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
		const late = "3bbbbbbb-0000-0000-0000-0000000000a8"
		f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user, user_type)
		           VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, 'Lena Late', 'Lena', 'Late', $2, true, false, 'EXTERNAL'::user_type_enum)`, late, crFlowEmail(late))
		f.registerContact(crScopeProjectC, crScopeAccountID, late)

		// Nobody was asked, so nobody was designated: the change request does not
		// exist for the contact who registered afterwards.
		f.wantHiddenFrom(id, "with no customer request pending", late)
		f.wantApproveRefused("answering with nobody asked", id, late)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.wantProposeRefused("proposing with nobody asked", id, late)
	})
	t.Run("the change reached Customer Approval with nobody to ask, legacy: the customer may answer", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
		f.requestApprovalThenContactsLeave(id)
		f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
		const late = "3bbbbbbb-0000-0000-0000-0000000000a8"
		f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user, user_type)
		           VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, 'Lena Late', 'Lena', 'Late', $2, true, false, 'EXTERNAL'::user_type_enum)`, late, crFlowEmail(late))
		f.registerContact(crScopeProjectC, crScopeAccountID, late)

		// A legacy change request is visible to its project's registered contacts and
		// the customer's first act creates the stage it lacks, so Lena is told true
		// (and the read itself creates nothing).
		f.wantCanAnswer(id, "with no customer request pending (legacy)", true, late)
		f.wantCanAnswer(id, "with no customer request pending (legacy), read again", true, late)
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("a read provisioned a customer stage: %+v", f.customerStages(id))
		}
	})
}

// A Re-schedule starts the customer's request over: false the moment it is
// superseded, true again for the contacts once they are asked again.
func TestChangeRequestCustomerCanAnswerIntegration_RescheduleFlips(t *testing.T) {
	t.Run("Normal: back through CAB, then asked again", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.driveToCustomerApproval(id)
		f.wantCanAnswer(id, "before the proposal", true, crScopeUserA1, crScopeUserA2)

		receipt, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)})
		if err != nil {
			t.Fatalf("proposal: %v", err)
		}
		if receipt.CustomerCanAnswer == nil || *receipt.CustomerCanAnswer {
			t.Fatalf("customerCanAnswer in the proposal's receipt = %v, want false (the change is back in Authorize)", receipt.CustomerCanAnswer)
		}
		f.expect(id, "after the proposal", "AUTHORIZE", "canceled")
		f.wantCanAnswer(id, "while the new plan awaits CAB", false, crScopeUserA1, crScopeUserA2)
		f.wantApproveRefused("answering while the new plan awaits CAB", id, crScopeUserA2)

		if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
			t.Fatalf("CAB approval of the new plan: %v", err)
		}
		f.expect(id, "after the new CAB approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.wantCanAnswer(id, "once the customer is asked again", true, crScopeUserA1, crScopeUserA2)
		if _, err := f.approveAs(id, crScopeUserA2, true); err != nil {
			t.Fatalf("approval of the new plan: %v", err)
		}
		f.wantCanAnswer(id, "after approving the new plan", false, crScopeUserA1, crScopeUserA2)
	})

	t.Run("Standard: stays in Customer Approval, asked again at once", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, false)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.requestApproval(id)
		f.wantCanAnswer(id, "before the proposal", true, crScopeUserA1, crScopeUserA2)

		receipt, err := f.patchAsContact(id, crScopeUserA2, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)})
		if err != nil {
			t.Fatalf("proposal: %v", err)
		}
		f.expect(id, "after the proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
		if receipt.CustomerCanAnswer == nil || !*receipt.CustomerCanAnswer {
			t.Fatalf("customerCanAnswer in the proposal's receipt = %v, want true (the customer is asked again)", receipt.CustomerCanAnswer)
		}
		// The superseded request's rows are cancelled and fresh ones are live.
		custom := f.customerStages(id)
		if len(custom) != 2 {
			t.Fatalf("customer stages = %+v, want the superseded one and a fresh one", custom)
		}
		assertApprovers(t, "superseded request", custom[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
		assertApprovers(t, "fresh request", custom[1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
		f.wantCanAnswer(id, "after the proposal", true, crScopeUserA1, crScopeUserA2)
	})
}

// A refused proposal changes nothing and leaves the answer as it was.
func TestChangeRequestCustomerCanAnswerIntegration_RefusedProposalLeavesItAlone(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart1)}); err == nil {
		t.Fatal("a proposal of the stored start was accepted")
	}
	f.wantCanAnswer(id, "after a refused proposal", true, crScopeUserA1, crScopeUserA2)
}

// The search rows never carry it: it is a detail-only, viewer-specific field.
func TestChangeRequestCustomerCanAnswerIntegration_NotOnSearchRows(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)
	rows, _, err := f.repo.SearchChangeRequests(asContact(crScopeUserA1), domain.SearchChangeRequestsRequest{
		Filters:    domain.SearchChangeRequestsFilters{ProjectIDs: []string{crScopeProjectA}},
		Pagination: domain.Pagination{Offset: 0, Limit: 50},
	}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, row := range rows {
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatalf("marshal row: %v", err)
		}
		if strings.Contains(string(raw), "customerCanAnswer") {
			t.Fatalf("a search row mentions customerCanAnswer: %s", raw)
		}
		if row.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("the change request %s is not among the %d search rows", id, len(rows))
	}
}
