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
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// A customer answers a change request in the customer portal, which sends PATCH
// {isCustomerApproved} / {isCustomerReviewed} / {plannedStartOn}; the same
// answers are also decisions on the Customer Approval / Customer Review stage in
// the Approvals tab. Same harness as the other TestChangeRequestFlowIntegration_*
// tests (crFlow, DSN-gated by CHANGE_REQUEST_TEST_DSN), with the PATCH sent by an
// EXTERNAL caller: an identity with Unrestricted false, the caller the customer
// portal's requests carry. The DSN connects as a Postgres superuser, which
// bypasses row-level security, so the checks asserted here are the repository's
// own (registered-contact test, state guard, whitelist), never RLS.
//
// Project A's registered contacts are Alice and Bob (the Customer Group), Carol
// is a contact of project B, Sam holds only SECURITY_CONTACT on A, Ivy is only
// invited to A, Ian is a registered contact of A whose user is inactive.

// asContact is the context of a customer: an external identity.
func asContact(userID string) context.Context {
	return repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: crFlowEmail(userID)})
}

// patchAsContact is PATCH /change-requests/{id} sent by the customer userID.
func (f *crFlow) patchAsContact(id, userID string, req domain.PatchChangeRequestRequest) (domain.ChangeRequest, error) {
	return f.repo.PatchChangeRequest(asContact(userID), id, req, crFlowEmail(userID))
}

func (f *crFlow) approveAs(id, userID string, approved bool) (domain.ChangeRequest, error) {
	return f.patchAsContact(id, userID, domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(approved)})
}

func (f *crFlow) reviewAs(id, userID string, reviewed bool) (domain.ChangeRequest, error) {
	return f.patchAsContact(id, userID, domain.PatchChangeRequestRequest{IsCustomerReviewed: boolp(reviewed)})
}

func (f *crFlow) wantConflictContaining(what string, err error, contains ...string) {
	f.t.Helper()
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) {
		f.t.Fatalf("%s: err = %v (%T), want *apierror.ConflictError", what, err, err)
	}
	for _, c := range contains {
		if !strings.Contains(ce.Msg, c) {
			f.t.Fatalf("%s: message %q should contain %q", what, ce.Msg, c)
		}
	}
}

// wantRefusedAsStranger asserts the caller was refused because they are not a
// registered contact of the change request's project: a 403 from the
// repository's own check when the database does not enforce row-level
// security (the superuser DSN the suite normally runs with), or a 404 from the
// row-level security that hides the change request from a non-member when it
// does (a non-superuser role). Either way the caller learns nothing and
// changes nothing.
func (f *crFlow) wantRefusedAsStranger(what string, err error) {
	f.t.Helper()
	var fe *apierror.ForbiddenError
	var nf *apierror.NotFoundError
	if !errors.As(err, &fe) && !errors.As(err, &nf) {
		f.t.Fatalf("%s: err = %v (%T), want *apierror.ForbiddenError or *apierror.NotFoundError", what, err, err)
	}
	if fe != nil && !strings.Contains(fe.Msg, "only a registered PORTAL_USER contact on this change request's own project") {
		f.t.Fatalf("%s: message %q should say who may answer", what, fe.Msg)
	}
}

// updatedBy is work_item.updated_by, what a PATCH stamps.
func (f *crFlow) updatedBy(id string) string {
	f.t.Helper()
	var by string
	if err := f.scoped.QueryRow(f.sys, `SELECT COALESCE(updated_by, '') FROM work_item WHERE id = $1`, id).Scan(&by); err != nil {
		f.t.Fatalf("read updated_by: %v", err)
	}
	return by
}

// customerSnapshot renders everything an answer must leave consistent: the
// state, both stamped flags and, for every customer stage, the status of each
// named approver (aliases standing in for user ids so two changes compare).
func (f *crFlow) customerSnapshot(id string, alias map[string]string) string {
	f.t.Helper()
	approved, reviewed := f.customerOutcome(id)
	parts := []string{f.state(id), fmt.Sprintf("approved=%v", approved), fmt.Sprintf("reviewed=%v", reviewed)}
	for _, st := range f.customerStages(id) {
		var rows []string
		for uid, status := range st.approvers {
			rows = append(rows, alias[uid]+"="+status)
		}
		sort.Strings(rows)
		parts = append(parts, st.label+"{"+strings.Join(rows, ",")+"}")
	}
	return strings.Join(parts, " | ")
}

// A change in Customer Approval, then Customer Review, answered by customers
// through PATCH alone: the calling contact's row, the siblings, the state, the
// stamped flags, legalNextStates and the actor after every step -- and a second
// contact arriving later is refused, with nothing written.
func TestChangeRequestCustomerOutcomeIntegration_PatchLifecycle(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	f.driveToCustomerApproval(id)
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	if a, r := f.customerOutcome(id); a || r {
		t.Fatalf("flags before any answer = %v/%v, want false/false", a, r)
	}

	// Alice approves: Scheduled, her row Approved, Bob's Cancelled, the flag stamped.
	cr, err := f.approveAs(id, crScopeUserA1, true)
	if err != nil {
		t.Fatalf("Alice approves: %v", err)
	}
	if cr.State == nil || *cr.State != "scheduled" || !cr.HasCustomerApproved || cr.HasCustomerReviewed {
		t.Fatalf("response after approval = state %v approved %v reviewed %v, want scheduled / true / false", cr.State, cr.HasCustomerApproved, cr.HasCustomerReviewed)
	}
	f.expect(id, "after the customer approved", "SCHEDULED", "implement", "canceled")
	assertApprovers(t, "Customer Approval after", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "APPROVED", crScopeUserA2: "CANCELLED"})
	if a, r := f.customerOutcome(id); !a || r {
		t.Fatalf("flags after approval = %v/%v, want true/false", a, r)
	}
	if got := f.updatedBy(id); got != crFlowEmail(crScopeUserA1) {
		t.Fatalf("work_item.updated_by = %q, want the approving contact %q", got, crFlowEmail(crScopeUserA1))
	}
	if n := f.requestedApprovers(id); n != 0 {
		t.Fatalf("%d approver rows still requested after the answer", n)
	}

	// Bob, the contact who lost the race, tries afterwards: refused with the
	// readable stale-approval message, in either direction, nothing written.
	for _, approved := range []bool{true, false} {
		_, err = f.approveAs(id, crScopeUserA2, approved)
		f.wantConflictContaining(fmt.Sprintf("Bob answering %v after Alice", approved), err,
			"this approval is no longer pending", "in Scheduled", "Customer Approval stage can only be decided while it is in Customer Approval")
	}
	f.expect(id, "after Bob's refused answers", "SCHEDULED", "implement", "canceled")
	assertApprovers(t, "Customer Approval after Bob's refused answers", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "APPROVED", crScopeUserA2: "CANCELLED"})
	if a, r := f.customerOutcome(id); !a || r {
		t.Fatalf("flags after Bob's refused answers = %v/%v, want true/false", a, r)
	}

	// Review: Implement -> Review -> Customer Review, then Bob answers.
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	assertApprovers(t, "Customer Review", f.customerStages(id)[1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

	cr, err = f.reviewAs(id, crScopeUserA2, true)
	if err != nil {
		t.Fatalf("Bob confirms the review: %v", err)
	}
	if cr.State == nil || *cr.State != "closed" || !cr.HasCustomerApproved || !cr.HasCustomerReviewed {
		t.Fatalf("response after review = state %v approved %v reviewed %v, want closed / true / true", cr.State, cr.HasCustomerApproved, cr.HasCustomerReviewed)
	}
	f.expect(id, "after the customer review", "CLOSED")
	assertApprovers(t, "Customer Review after", f.customerStages(id)[1].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "APPROVED"})
	if n := f.requestedApprovers(id); n != 0 {
		t.Fatalf("%d approver rows still requested on a closed change", n)
	}
	_, err = f.reviewAs(id, crScopeUserA1, true)
	f.wantConflictContaining("Alice answering the review after Bob", err, "this approval is no longer pending", "in Closed", "Customer Review stage")
	f.expect(id, "after Alice's refused review", "CLOSED")
}

// The two answers a customer can refuse with: Customer Approval rejected
// cancels, Customer Review rejected rolls back; neither stamps its flag.
func TestChangeRequestCustomerOutcomeIntegration_PatchRejections(t *testing.T) {
	t.Run("customer approval rejected cancels", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.driveToCustomerApproval(id)

		cr, err := f.approveAs(id, crScopeUserA2, false)
		if err != nil {
			t.Fatalf("Bob rejects: %v", err)
		}
		if cr.State == nil || *cr.State != "canceled" || cr.HasCustomerApproved {
			t.Fatalf("response after rejection = state %v approved %v, want canceled / false", cr.State, cr.HasCustomerApproved)
		}
		f.expect(id, "after the customer rejected", "CANCELED")
		assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "REJECTED"})
		if a, _ := f.customerOutcome(id); a {
			t.Fatal("is_customer_approval_required stamped by a rejection")
		}
		if n := f.requestedApprovers(id); n != 0 {
			t.Fatalf("%d approver rows still requested on a cancelled change", n)
		}
		_, err = f.approveAs(id, crScopeUserA1, true)
		f.wantConflictContaining("approving a cancelled change", err, "in Canceled")
		f.expect(id, "after the refused approval", "CANCELED")
	})

	t.Run("customer review rejected rolls back", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.driveToCustomerReview(id)

		cr, err := f.reviewAs(id, crScopeUserA1, false)
		if err != nil {
			t.Fatalf("Alice fails the review: %v", err)
		}
		if cr.State == nil || *cr.State != "rollback" || cr.HasCustomerReviewed {
			t.Fatalf("response after rejection = state %v reviewed %v, want rollback / false", cr.State, cr.HasCustomerReviewed)
		}
		f.expect(id, "after the customer failed the review", "ROLLBACK")
		assertApprovers(t, "Customer Review", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REJECTED", crScopeUserA2: "CANCELLED"})
		if _, r := f.customerOutcome(id); r {
			t.Fatal("is_customer_review_required stamped by a rejection")
		}
		_, err = f.reviewAs(id, crScopeUserA2, true)
		f.wantConflictContaining("confirming a rolled-back change", err, "in Rollback")
		f.expect(id, "after the refused confirmation", "ROLLBACK")
	})
}

// Both doors give one outcome: the same answer through PATCH and through the
// decision route leaves the same state, the same flags and the same rows.
func TestChangeRequestCustomerOutcomeIntegration_PatchEqualsDecisionRoute(t *testing.T) {
	alias := map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob"}
	type answer struct {
		name     string
		approval bool // Customer Approval (else Customer Review)
		approved bool
		by       string
	}
	for _, a := range []answer{
		{"customer approval approved", true, true, crScopeUserA1},
		{"customer approval rejected", true, false, crScopeUserA2},
		{"customer review approved", false, true, crScopeUserA2},
		{"customer review rejected", false, false, crScopeUserA1},
	} {
		a := a
		t.Run(a.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			setup := func() string {
				id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, !a.approval)
				f.driveToCustomerApproval(id)
				if !a.approval {
					if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
						t.Fatalf("approval on the way to review: %v", err)
					}
					f.driveToCustomerReview(id)
				}
				return id
			}
			viaPatch, viaRoute := setup(), setup()

			var err error
			if a.approval {
				_, err = f.approveAs(viaPatch, a.by, a.approved)
			} else {
				_, err = f.reviewAs(viaPatch, a.by, a.approved)
			}
			if err != nil {
				t.Fatalf("PATCH: %v", err)
			}
			decision := "rejected"
			if a.approved {
				decision = "approved"
			}
			if err := f.decide(viaRoute, a.by, decision); err != nil {
				t.Fatalf("decision route: %v", err)
			}

			gotPatch, gotRoute := f.customerSnapshot(viaPatch, alias), f.customerSnapshot(viaRoute, alias)
			if gotPatch != gotRoute {
				t.Fatalf("the two doors disagree:\n  PATCH:          %s\n  decision route: %s", gotPatch, gotRoute)
			}
			t.Logf("both doors: %s", gotPatch)
		})
	}
}

// The answer belongs to one state: refused (409, the stale-approval message)
// everywhere else, and every refusal leaves the change as it was.
func TestChangeRequestCustomerOutcomeIntegration_PatchOutOfState(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	refuse := func(when, wantIn string, approvalFlag, reviewFlag bool) {
		t.Helper()
		before := f.customerSnapshot(id, map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob"})
		if approvalFlag {
			for _, v := range []bool{true, false} {
				_, err := f.approveAs(id, crScopeUserA1, v)
				f.wantConflictContaining(when+": isCustomerApproved="+fmt.Sprint(v), err, "this approval is no longer pending", wantIn, "Customer Approval stage can only be decided while it is in Customer Approval")
			}
		}
		if reviewFlag {
			for _, v := range []bool{true, false} {
				_, err := f.reviewAs(id, crScopeUserA1, v)
				f.wantConflictContaining(when+": isCustomerReviewed="+fmt.Sprint(v), err, "this approval is no longer pending", wantIn, "Customer Review stage can only be decided while it is in Customer Review")
			}
		}
		if after := f.customerSnapshot(id, map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob"}); after != before {
			t.Fatalf("%s: a refused answer changed the change request:\n  before: %s\n  after:  %s", when, before, after)
		}
	}

	// Before the customer's approval is first asked (New, Assess, Authorize) the
	// change request was never designated to a customer: it does not exist for
	// them, so there is nothing to refuse -- 404, whatever they send.
	hidden := func(when string) {
		t.Helper()
		before := f.customerSnapshot(id, map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob"})
		for _, v := range []bool{true, false} {
			_, err := f.approveAs(id, crScopeUserA1, v)
			f.wantNotFound(when+": isCustomerApproved="+fmt.Sprint(v), err)
			_, err = f.reviewAs(id, crScopeUserA1, v)
			f.wantNotFound(when+": isCustomerReviewed="+fmt.Sprint(v), err)
		}
		if after := f.customerSnapshot(id, map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob"}); after != before {
			t.Fatalf("%s: a refused answer changed the change request:\n  before: %s\n  after:  %s", when, before, after)
		}
	}
	hidden("in New")
	f.requestApproval(id)
	hidden("in Assess")
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	hidden("in Authorize")
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	// Customer Approval takes the approval, not the review.
	refuse("in Customer Approval (the review flag)", "in Customer Approval", false, true)

	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("approval: %v", err)
	}
	refuse("in Scheduled", "in Scheduled", true, true)
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	refuse("in Implement", "in Implement", true, true)
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	refuse("in Review", "in Review", true, true)
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	// Customer Review takes the review, not the approval.
	refuse("in Customer Review (the approval flag)", "in Customer Review", true, false)
	if _, err := f.reviewAs(id, crScopeUserA2, false); err != nil {
		t.Fatalf("rejecting the review: %v", err)
	}
	refuse("in Rollback", "in Rollback", true, true)
}

// Who may answer: only a registered PORTAL_USER contact of the change request's
// OWN project, and not its creator.
func TestChangeRequestCustomerOutcomeIntegration_PatchWhoMayAnswer(t *testing.T) {
	f := newCustomerGroupFlow(t)
	f.registerContact(crScopeProjectA, crScopeAccountID, crFlowCreatorID)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers,
		map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED", crFlowCreatorID: "CANCELLED"})

	before := f.customerSnapshot(id, map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob", crFlowCreatorID: "creator"})
	for _, tc := range []struct {
		name, user string
		stranger   bool   // not a member of the project's contacts at all: refused before the change request is even looked at
		want       string // the 403's message, for a caller who is a member
	}{
		{"a contact of ANOTHER project", crScopeUserB1, true, ""},
		{"a contact who was only invited", crScopeUserInvited, true, ""},
		{"an internal user who is not a contact", crFlowPeerAID, true, ""},
		{"a contact holding no PORTAL_USER role", crScopeUserSecurity, false, "only a registered PORTAL_USER contact on this change request's own project"},
		{"a registered contact whose user is inactive (never asked)", crScopeUserInactive, false, "only members of the customer group"},
		{"the creator, although a registered contact", crFlowCreatorID, false, "creator of a change request cannot approve it"},
	} {
		for _, approved := range []bool{true, false} {
			_, err := f.approveAs(id, tc.user, approved)
			what := fmt.Sprintf("%s answering %v", tc.name, approved)
			if tc.stranger {
				f.wantRefusedAsStranger(what, err)
			} else {
				f.wantForbidden(what, err, tc.want)
			}
		}
	}
	if after := f.customerSnapshot(id, map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob", crFlowCreatorID: "creator"}); after != before {
		t.Fatalf("a refused answer changed the change request:\n  before: %s\n  after:  %s", before, after)
	}

	// A caller with no user record at all (an unknown email) is not a contact.
	_, err := f.repo.PatchChangeRequest(
		repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: "nobody@example.com"}),
		id, domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(true)}, "nobody@example.com")
	f.wantRefusedAsStranger("an unknown caller", err)

	// A real contact still can.
	if _, err := f.approveAs(id, crScopeUserA2, true); err != nil {
		t.Fatalf("a real contact: %v", err)
	}
	f.expect(id, "after Bob approved", "SCHEDULED", "implement", "canceled")
}

// A contact registered after the request went out was not asked: refused, with
// the change as it was. And a change in Customer Approval with nobody asked (no
// eligible contact when it got there) has nothing to answer: it stays where it
// is, and WSO2 cannot answer for the customer either (cancel or re-schedule are
// what staff have).
func TestChangeRequestCustomerOutcomeIntegration_PatchWhenNobodyWasAsked(t *testing.T) {
	t.Run("a contact registered afterwards was not asked", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.driveToCustomerApproval(id)
		// Zed registers on project A once the request is already out.
		const zed = "3bbbbbbb-0000-0000-0000-0000000000a9"
		f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user, user_type)
		           VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, 'Zed Late', 'Zed', 'Late', $2, true, false, 'EXTERNAL'::user_type_enum)`, zed, crFlowEmail(zed))
		f.registerContact(crScopeProjectA, crScopeAccountID, zed)

		_, err := f.approveAs(id, zed, true)
		f.wantForbidden("a late contact", err, "only members of the customer group")
		f.expect(id, "after the late contact's refused answer", "CUSTOMER_APPROVAL", "authorize", "canceled")
		assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	})

	// A change request in Customer Approval with nobody asked (no eligible
	// contact when it got there, or it got there under an older build) has no
	// live stage to answer. What happens depends on whether customers may see it
	// at all: a LEGACY one (created before the strict-visibility cutover) is
	// visible to its project's registered contacts, and the customer's first act
	// gives it the stage it lacks, so the customer's answer is recorded like any
	// other; a STRICT one was never designated to anybody, so it does not exist
	// for the customers, and WSO2 does not answer for them either: it waits.
	t.Run("no customer request is pending, strict: invisible to the customers", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		f.useVisibility(visStrictSinceLongAgo())
		// Project C has no contact when the change reaches Customer Approval.
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
		f.requestApprovalThenContactsLeave(id)
		f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("customer stage provisioned for a project without contacts: %+v", f.customerStages(id))
		}
		// A contact registers later; nobody was ever asked, and this change
		// request was never designated to them.
		const late = "3bbbbbbb-0000-0000-0000-0000000000a8"
		f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user, user_type)
		           VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, 'Lena Late', 'Lena', 'Late', $2, true, false, 'EXTERNAL'::user_type_enum)`, late, crFlowEmail(late))
		f.registerContact(crScopeProjectC, crScopeAccountID, late)

		_, err := f.approveAs(id, late, true)
		f.wantNotFound("answering a change request nobody designated to them", err)
		_, err = f.approveAs(id, late, false)
		f.wantNotFound("rejecting a change request nobody designated to them", err)
		f.expect(id, "after the refused answers", "CUSTOMER_APPROVAL", "authorize", "canceled")
		if a, _ := f.customerOutcome(id); a {
			t.Fatal("flag stamped by a refused answer")
		}
		// WSO2 does not record it for them: the manual scheduled is refused, and
		// nothing is stamped.
		_, err = f.patchState(id, domain.ChangeRequestStateScheduled)
		f.wantValidationError("WSO2 answering for a customer who was never asked", err, "can only be given by the customer in the Customer Portal")
		f.expect(id, "after the refused manual scheduled", "CUSTOMER_APPROVAL", "authorize", "canceled")
		if a, _ := f.customerOutcome(id); a {
			t.Fatal("a refused manual scheduled stamped the flag")
		}
	})

	t.Run("no customer request is pending, legacy: the first customer act gives it the stage", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		// The default policy has no cutover: every change request is legacy.
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
		f.requestApprovalThenContactsLeave(id)
		f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("customer stage provisioned for a project without contacts: %+v", f.customerStages(id))
		}
		const late = "3bbbbbbb-0000-0000-0000-0000000000a8"
		f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user, user_type)
		           VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, 'Lena Late', 'Lena', 'Late', $2, true, false, 'EXTERNAL'::user_type_enum)`, late, crFlowEmail(late))
		f.registerContact(crScopeProjectC, crScopeAccountID, late)

		// An answer that is refused (here: it names a window the change request
		// does not have) provisions nothing: the stage is created only by an act
		// that is then recorded.
		if _, err := f.patchAsContact(id, late, domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(true), ExpectedPlannedStartOn: sp("2031-01-01T00:00:00Z")}); err == nil {
			t.Fatal("an answer for a window the change request does not have was accepted")
		}
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("a refused answer left a customer stage behind: %+v", f.customerStages(id))
		}
		if _, err := f.approveAs(id, late, true); err != nil {
			t.Fatalf("the first customer act on a stage-less legacy change request: %v", err)
		}
		f.expect(id, "after the answer", "SCHEDULED", "implement", "canceled")
		st := f.customerStages(id)
		if len(st) != 1 {
			t.Fatalf("customer stages after the answer = %+v, want exactly the one provisioned", st)
		}
		assertApprovers(t, "the provisioned stage", st[0].approvers, map[string]string{late: "APPROVED"})
		if a, _ := f.customerOutcome(id); !a {
			t.Fatal("the answer did not stamp the customer's approval")
		}
	})
}

// Once the customer's approval or review is recorded as true it is final: a
// rejection of a flag that is already true is refused, whoever stamped it.
func TestChangeRequestCustomerOutcomeIntegration_PatchRespectsTheFlagLock(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)
	// The flag is already true while the request is still out -- as a row synced
	// from ServiceNow can be (WSO2 itself can no longer stamp it: see
	// ..._InternalFlagStampRefused).
	f.execSQL(`UPDATE change_request SET is_customer_approval_required = true WHERE id = $1`, id)
	f.expect(id, "with the flag already stored (state untouched)", "CUSTOMER_APPROVAL", "authorize", "canceled")
	if a, _ := f.customerOutcome(id); !a {
		t.Fatal("the stored flag is not read back")
	}
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

	_, err := f.approveAs(id, crScopeUserA1, false)
	f.wantValidationError("rejecting an approval already recorded", err, "locked once set to true")
	f.expect(id, "after the refused rejection", "CUSTOMER_APPROVAL", "authorize", "canceled")
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

	// Approving is still the contact's answer: true stays true, the state moves.
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("approving: %v", err)
	}
	f.expect(id, "after the approval", "SCHEDULED", "implement", "canceled")
	if a, _ := f.customerOutcome(id); !a {
		t.Fatal("flag lost")
	}
}

// An external caller may send the customer's answer or a proposed window and
// nothing else, however it is combined: the rest of the contract is for WSO2.
func TestChangeRequestCustomerOutcomeIntegration_ExternalWhitelist(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)
	alias := map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob"}
	before := f.customerSnapshot(id, alias)
	subject := func() string {
		if cr := f.get(id); cr.Subject != nil {
			return *cr.Subject
		}
		return ""
	}
	beforeSubject := subject()

	st := func(s domain.ChangeRequestState) *domain.ChangeRequestState { return &s }
	impact := domain.ChangeRequestImpact("high")
	for name, req := range map[string]domain.PatchChangeRequestRequest{
		"a title":                    {Title: sp("hijacked")},
		"a description":              {Description: sp("hijacked")},
		"impact":                     {Impact: &impact},
		"a state":                    {State: st(domain.ChangeRequestStateCanceled)},
		"scheduled by hand":          {State: st(domain.ChangeRequestStateScheduled)},
		"the answer plus a state":    {IsCustomerApproved: boolp(true), State: st(domain.ChangeRequestStateScheduled)},
		"the answer plus a title":    {IsCustomerApproved: boolp(true), Title: sp("hijacked")},
		"a window plus a title":      {PlannedStartOn: sp(rsStart2), Title: sp("hijacked")},
		"a project":                  {ProjectID: sp(crScopeProjectB)},
		"an assignee":                {AssignedEngineerID: sp(crFlowPeerAID)},
		"a team":                     {AssignedTeamID: sp(crFlowGroupID)},
		"request approval":           {RequestApproval: boolp(true)},
		"on hold":                    {OnHold: boolp(true)},
		"a journal comment":          {Comment: sp("hello")},
		"a work note":                {WorkNote: sp("internal")},
		"the customer approval gate": {CustomerApprovalRequired: boolp(false)},
		"planning visibility":        {IsPlanningVisibleToCustomers: boolp(true)},
		"the justification":          {Justification: sp("hijacked")},
	} {
		_, err := f.patchAsContact(id, crScopeUserA1, req)
		f.wantForbidden(name, err, "customers can only record the customer's approval or review")
	}
	if after := f.customerSnapshot(id, alias); after != before {
		t.Fatalf("a refused PATCH changed the change request:\n  before: %s\n  after:  %s", before, after)
	}
	if got := subject(); got != beforeSubject {
		t.Fatalf("subject = %q, want %q", got, beforeSubject)
	}

	// Ambiguous, not forbidden: the four allowed fields in a combination that
	// means nothing.
	_, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(true), IsCustomerReviewed: boolp(true)})
	f.wantValidationError("both outcomes", err, "not both")
	_, err = f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(true), PlannedStartOn: sp(rsStart2)})
	f.wantValidationError("an answer and a window", err, "separate requests")
	if after := f.customerSnapshot(id, alias); after != before {
		t.Fatalf("a refused PATCH changed the change request:\n  before: %s\n  after:  %s", before, after)
	}

	// WSO2 users (an internal identity) are unaffected: the whole contract.
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{Title: sp("edited by WSO2")}); err != nil {
		t.Fatalf("an internal PATCH of the title: %v", err)
	}
}

// WSO2 cannot record the customer's answer by hand: {isCustomerApproved: true}
// from an internal caller used to stamp the flag (leaving the state where it
// was); it is refused now, with the stage, the state and the flag exactly as
// they were. The customer's own answer still works afterwards.
func TestChangeRequestCustomerOutcomeIntegration_InternalFlagStampRefused(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)
	alias := map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob"}
	before := f.customerSnapshot(id, alias)
	for _, v := range []bool{true, false} {
		_, err := f.patch(id, domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(v)})
		f.wantValidationError(fmt.Sprintf("internal isCustomerApproved=%v", v), err, "isCustomerApproved cannot be set on the customer's behalf")
	}
	f.expect(id, "after the refused internal stamps", "CUSTOMER_APPROVAL", "authorize", "canceled")
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	if a, _ := f.customerOutcome(id); a {
		t.Fatal("an internal caller stamped the customer's approval")
	}
	if after := f.customerSnapshot(id, alias); after != before {
		t.Fatalf("a refused internal stamp changed the change request:\n  before: %s\n  after:  %s", before, after)
	}
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("the customer's own answer after the refused stamps: %v", err)
	}
	f.expect(id, "after the customer approved", "SCHEDULED", "implement", "canceled")
}

// Two contacts answer at once: exactly one answer is recorded, the other is
// refused as stale, the flag is stamped once.
func TestChangeRequestCustomerOutcomeIntegration_ConcurrentAnswers(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)

	type result struct {
		user string
		err  error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, uid := range []string{crScopeUserA1, crScopeUserA2} {
		uid := uid
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.approveAs(id, uid, true)
			results <- result{uid, err}
		}()
	}
	wg.Wait()
	close(results)

	var winners, losers []string
	for r := range results {
		if r.err == nil {
			winners = append(winners, r.user)
			continue
		}
		var ce *apierror.ConflictError
		if !errors.As(r.err, &ce) {
			t.Fatalf("the losing answer by %s: err = %v (%T), want *apierror.ConflictError", r.user, r.err, r.err)
		}
		losers = append(losers, r.user)
	}
	if len(winners) != 1 || len(losers) != 1 {
		t.Fatalf("winners %v, losers %v; want exactly one of each", winners, losers)
	}
	f.expect(id, "after the race", "SCHEDULED", "implement", "canceled")
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{winners[0]: "APPROVED", losers[0]: "CANCELLED"})
	if a, _ := f.customerOutcome(id); !a {
		t.Fatal("flag not stamped")
	}
}

// "Propose new implementation time" is the customer's proposal of a START: it is
// written to customer_updated_on (the column the previous system used for it) and the change
// WAITS in Customer Approval for WSO2's answer -- no state move, no CAB stage, the
// planned window and the customers' request untouched. WSO2 then accepts it (the
// change is Scheduled with the proposal as its start, the planned length kept, no
// CAB, no second ask) or proposes a different time (DISAGREE, the customers are asked
// again), for every change type that reaches Customer Approval (an Emergency change never does).
func TestChangeRequestCustomerOutcomeIntegration_ProposeNewTimeWaitsForWSO2(t *testing.T) {
	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard} {
		typ := typ
		wantStages := map[domain.ChangeRequestType]string{
			domain.ChangeRequestTypeNormal:   "Peer Approval,CAB Approval,Customer Approval",
			domain.ChangeRequestTypeStandard: "Customer Approval",
		}[typ]
		t.Run(string(typ), func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(typ)
			if got := f.stageLabels(id); got != wantStages {
				t.Fatalf("stages in Customer Approval = %s, want %s", got, wantStages)
			}

			// Refused: the planned start again, a window that changes the length, garbage.
			_, err := f.proposeAs(id, crScopeUserA1, rsStart1)
			f.wantValidationError("proposing the planned start", err, "plannedStartOn is the planned start already")
			_, err = f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd3)})
			f.wantValidationError("proposing a different length", err, "keeps the planned length of 2 hours")
			_, err = f.proposeAs(id, crScopeUserA1, "next tuesday")
			f.wantValidationError("proposing garbage", err, "plannedStartOn must be a valid date-time")
			f.wantConversation(id, "after the refused proposals", "", "")

			// Alice proposes a new start.
			cr, err := f.proposeAs(id, crScopeUserA1, rsStart2)
			if err != nil {
				t.Fatalf("proposal: %v", err)
			}
			if cr.State == nil || *cr.State != "customer_approval" {
				t.Fatalf("state in the receipt = %v, want customer_approval (a proposal moves nothing)", cr.State)
			}
			if p := cr.CustomerProposal; p == nil || p.Answer != "pending" || p.StartOn != rsStart2 || p.EndOn == nil || *p.EndOn != endFor(rsStart2) ||
				p.ProposedByViewer == nil || !*p.ProposedByViewer {
				t.Fatalf("customerProposal in the proposer's receipt = %+v, want a pending proposal that is theirs", p)
			}
			if cr.CustomerCanAnswer == nil || !*cr.CustomerCanAnswer {
				t.Fatalf("customerCanAnswer in the receipt = %v, want true (their request is still live)", cr.CustomerCanAnswer)
			}
			f.expect(id, "after the proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
			f.wantPlanned(id, "after the proposal", rsStart1, rsEnd1)
			f.wantConversation(id, "after the proposal", rsStart2, "")
			if got := f.stageLabels(id); got != wantStages {
				t.Fatalf("stages after the proposal = %s, want them unchanged (%s)", got, wantStages)
			}
			custom := f.customerStages(id)
			if len(custom) != 1 {
				t.Fatalf("customer stages = %+v, want the one request", custom)
			}
			assertApprovers(t, "the customers' request is untouched", custom[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
			if a, _ := f.customerOutcome(id); a {
				t.Fatal("flag stamped by a proposal")
			}
			if got := f.updatedBy(id); got != crFlowEmail(crScopeUserA1) {
				t.Fatalf("work_item.updated_by = %q, want the proposing contact", got)
			}

			// WSO2's side of the same read: the proposer and whether Accept would work.
			p := f.proposalOf(id)
			if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || !*p.ProposerRecorded ||
				p.ProposedByEmail == nil || *p.ProposedByEmail != crFlowEmail(crScopeUserA1) || p.CanAccept == nil || !*p.CanAccept {
				t.Fatalf("customerProposal for WSO2 = %+v", p)
			}
			// A colleague sees a proposal that is not theirs.
			if other, err := f.getAsContact(id, crScopeUserA2); err != nil || other.CustomerProposal == nil ||
				other.CustomerProposal.ProposedByViewer == nil || *other.CustomerProposal.ProposedByViewer || other.CustomerProposal.ProposedByEmail != nil {
				t.Fatalf("a colleague's view of the proposal = %+v (%v)", other.CustomerProposal, err)
			}
			f.wantCanAnswer(id, "after the proposal", true, crScopeUserA1, crScopeUserA2)

			// Accept proposed time: Scheduled, the proposal is the start, the length is kept,
			// nothing goes through CAB, the customers' request is closed.
			f.mustAccept(id)
			f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
			f.wantPlanned(id, "after Accept", rsStart2, endFor(rsStart2))
			f.wantConversation(id, "after Accept", rsStart2, "AGREE")
			f.wantAnswer(id, "after Accept", "agreed")
			if got := f.stageLabels(id); got != wantStages {
				t.Fatalf("stages after Accept = %s, want %s (no CAB, no second ask)", got, wantStages)
			}
			assertApprovers(t, "the customers' request after Accept", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
			if a, _ := f.customerOutcome(id); a {
				t.Fatal("Accept recorded the customer's approval: no staff action does")
			}
		})
	}
}

// ... and WSO2 may answer a proposal with a different time instead: the customers
// are asked again with a fresh request, nothing goes through CAB, the change stays
// where it is.
func TestChangeRequestCustomerOutcomeIntegration_ProposeNewTimeIsCounteredByWSO2(t *testing.T) {
	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard} {
		typ := typ
		t.Run(string(typ), func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(typ)
			stages := f.stageLabels(id)
			f.mustPropose(id, crScopeUserA2, rsStartEarly)

			if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
				t.Fatalf("Propose a different time: %v", err)
			}
			f.expect(id, "after the counter", "CUSTOMER_APPROVAL", "authorize", "canceled")
			f.wantPlanned(id, "after the counter", rsStart3, rsEnd3)
			f.wantConversation(id, "after the counter", rsStartEarly, "DISAGREE")
			f.wantAnswer(id, "after the counter", "disagreed")
			if got := f.stageLabels(id); got != stages+",Customer Approval" {
				t.Fatalf("stages after the counter = %s, want %s and one fresh customer stage (no CAB)", got, stages)
			}
			custom := f.customerStages(id)
			assertApprovers(t, "the superseded request", custom[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
			assertApprovers(t, "the customers asked again", custom[1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
			f.wantCanAnswer(id, "after the counter", true, crScopeUserA1, crScopeUserA2)

			// The customer answers WSO2's time; the window is WSO2's.
			if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
				t.Fatalf("approval of WSO2's time: %v", err)
			}
			f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
			f.wantPlanned(id, "after the customer's approval", rsStart3, rsEnd3)
			if a, _ := f.customerOutcome(id); !a {
				t.Fatal("the customer's own approval was not recorded")
			}
		})
	}
}

// A proposal is refused where it makes no sense, and by whom it may not come.
func TestChangeRequestCustomerOutcomeIntegration_ProposeNewTimeRefusals(t *testing.T) {
	f := newCustomerGroupFlow(t)
	f.registerContact(crScopeProjectA, crScopeAccountID, crFlowCreatorID)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	propose := func(user string) error {
		_, err := f.patchAsContact(id, user, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStartEarly)})
		return err
	}

	// Not before the customer's approval is what is being waited for: until
	// then (New, Assess, Authorize) the change request was never designated to a
	// customer, so there is nothing for them to propose on (404).
	f.wantNotFound("proposing in New", propose(crScopeUserA1))
	f.requestApproval(id)
	f.wantNotFound("proposing in Assess", propose(crScopeUserA1))
	f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")

	// Not by anyone who is not a contact of this project, nor by the creator.
	for _, user := range []string{crScopeUserB1, crScopeUserInvited, crFlowPeerAID} {
		f.wantRefusedAsStranger("proposal by "+user, propose(user))
	}
	f.wantForbidden("proposal by a contact holding no PORTAL_USER role", propose(crScopeUserSecurity), "only a registered PORTAL_USER contact on this change request's own project")
	f.wantForbidden("proposal by the creator", propose(crFlowCreatorID), "creator of a change request cannot approve it")

	// Not while the change is on hold.
	f.execSQL(`UPDATE change_request SET is_on_hold = true WHERE id = $1`, id)
	f.wantConflictContaining("proposing on hold", propose(crScopeUserA1), "on hold")
	f.execSQL(`UPDATE change_request SET is_on_hold = false WHERE id = $1`, id)

	f.expect(id, "after the refused proposals", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantPlanned(id, "after the refused proposals", rsStart1, rsEnd1)
	assertApprovers(t, "customer request untouched", f.customerStages(id)[0].approvers,
		map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED", crFlowCreatorID: "CANCELLED"})

	// Not once the customer has answered.
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("approval: %v", err)
	}
	f.wantConflictContaining("proposing in Scheduled", propose(crScopeUserA2), "in Scheduled")
	f.wantPlanned(id, "after the refused proposal in Scheduled", rsStart1, rsEnd1)
}
