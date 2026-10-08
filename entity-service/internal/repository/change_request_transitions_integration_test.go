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
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The transition graph of PATCH {state}, end to end: a staff request moves a change
// request only along the edges of one table -- the table legalNextStates is
// rendered from -- and everything else is a 400 that changes nothing. In
// particular no PATCH revives a closed, canceled or rolled-back change (a change
// canceled in Customer Approval was sent to Implement with no customer answer) and
// none skips a gate (a change with both customer boxes ticked went from New to
// Implement with Peer and CAB approval skipped and the customer never asked).
//
// Same harness as TestChangeRequestFlowIntegration_* (crFlow, DSN-gated by
// CHANGE_REQUEST_TEST_DSN, run as a superuser and as the non-superuser csm_app).

var allChangeRequestStates = []string{
	"NEW", "ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "ROLLBACK", "CLOSED", "CANCELED",
}

// wantStaffMoves is the table, written out and NOT derived from the production
// helpers: the states a staff PATCH may name from each state, for customer_review_required
// = review. Rollback out of Customer Review is on it while nobody is being asked.
func wantStaffMoves(state string, review bool) []string {
	var out []string
	switch state {
	case "", "NEW":
		out = []string{"assess"}
	case "CUSTOMER_APPROVAL":
		out = []string{"authorize"} // Re-schedule
	case "SCHEDULED":
		out = []string{"implement"}
	case "IMPLEMENT":
		out = []string{"review"}
	case "REVIEW":
		if review {
			out = []string{"customer_review", "rollback"}
		} else {
			out = []string{"closed", "rollback"}
		}
	case "CUSTOMER_REVIEW":
		out = []string{"rollback"}
	case "ASSESS", "AUTHORIZE":
		// waiting for their approvals: Cancel only
	default: // CLOSED, CANCELED, ROLLBACK: final
		return nil
	}
	out = append(out, "canceled")
	sort.Strings(out)
	return out
}

// finalMsg is the refusal of every request to move a final change.
func finalMsg(requested, from string) string {
	phrase := strings.ToLower(from)
	if from == "ROLLBACK" {
		phrase = "rolled back"
	}
	return fmt.Sprintf(`state %q cannot be set manually from %s: a change request that is %s cannot be moved`, requested, strings.ToLower(from), phrase)
}

// attemptState is what the portal sends for a state: Re-schedule carries the new window.
func (f *crFlow) attemptState(id, requested string) error {
	if requested == "authorize" {
		return f.reschedule(id, sp(rsStart2), sp(rsEnd2))
	}
	_, err := f.patchState(id, domain.ChangeRequestState(requested))
	return err
}

// transitionFixture is a Normal change on project A (registered contacts) in the
// given stored state, with the planned window set, as the flow leaves it: no
// stage, no live customer request (those are the stage tests' business).
func (f *crFlow) transitionFixture(state string, approval, review bool) string {
	f.t.Helper()
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), approval, review)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.setState(id, state)
	return id
}

func (f *crFlow) wantValidationRefusal(what, id string, requested string, wantMsg string) {
	f.t.Helper()
	before := f.bypassSnapshot(id)
	err := f.attemptState(id, requested)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		f.t.Fatalf("%s: err = %v (%T), want a 400 (*apierror.ValidationError)", what, err, err)
	}
	if wantMsg != "" && ve.Msg != wantMsg {
		f.t.Fatalf("%s: message = %q, want %q", what, ve.Msg, wantMsg)
	}
	if after := f.bypassSnapshot(id); after != before {
		f.t.Fatalf("%s: a refused PATCH changed the change request:\n  before: %s\n  after:  %s", what, before, after)
	}
}

// Every state by every request, with every combination of the two customer boxes:
// a staff PATCH is accepted exactly when the request is an edge of the table, and
// that set is exactly what the change's legalNextStates offers. Refused requests
// are 400s and change nothing; accepted ones land where the table says.
func TestChangeRequestTransitionsIntegration_EveryStateByEveryRequest(t *testing.T) {
	f := newCustomerGroupFlow(t)
	lands := map[string]string{
		"assess": "ASSESS", "implement": "IMPLEMENT", "review": "REVIEW",
		"closed": "CLOSED", "customer_review": "CUSTOMER_REVIEW", "rollback": "ROLLBACK", "canceled": "CANCELED",
	}
	for _, approval := range []bool{false, true} {
		for _, review := range []bool{false, true} {
			for _, state := range append([]string{""}, allChangeRequestStates...) {
				label := state
				if label == "" {
					label = "NULL"
				}
				want := wantStaffMoves(state, review)

				// legalNextStates of such a change is that very set. (A NULL state, a
				// pre-lifecycle legacy row, is treated as New by the PATCH but has no
				// legalNextStates at all, as it never had: the one row left out.)
				id := f.transitionFixture(state, approval, review)
				if state != "" {
					assertStates(t, fmt.Sprintf("approval=%v review=%v in %s: legalNextStates", approval, review, label), sortedCopy(f.legal(id)), want...)
				}

				var accepted []string
				for _, requested := range []string{"new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "rollback", "closed", "canceled"} {
					current := strings.ToLower(state)
					if current == "" {
						current = "new"
					}
					if requested == current {
						continue // a resend: TestChangeRequestTransitionsIntegration_ResendsAreNoOps
					}
					id := f.transitionFixture(state, approval, review)
					what := fmt.Sprintf("approval=%v review=%v %s -> %s", approval, review, label, requested)
					before := f.bypassSnapshot(id)
					err := f.attemptState(id, requested)
					if err == nil {
						accepted = append(accepted, requested)
						// "authorize" is the wire name of Re-schedule, which does not move the
						// state: the change stays in Customer Approval, nothing goes through CAB
						// again, and the customers are asked again with exactly one new stage.
						wantLand := lands[requested]
						if requested == "authorize" {
							wantLand = "CUSTOMER_APPROVAL"
						}
						if got := f.state(id); got != wantLand {
							t.Fatalf("%s: accepted, but the change is now %s, want %s", what, got, wantLand)
						}
						if requested == "authorize" {
							if got := f.stageLabels(id); got != stageCustApproval {
								t.Fatalf("%s: stages after the Re-schedule = %q, want exactly one new customer stage and no CAB", what, got)
							}
							if n := f.liveStageRows(id, stageCustApproval); n != 2 {
								t.Fatalf("%s: %d customer rows asked, want both contacts", what, n)
							}
						}
						continue
					}
					var ve *apierror.ValidationError
					if !errors.As(err, &ve) {
						t.Fatalf("%s: refused with %v (%T), want a 400", what, err, err)
					}
					if after := f.bypassSnapshot(id); after != before {
						t.Fatalf("%s: a refused PATCH changed the change request:\n  before: %s\n  after:  %s", what, before, after)
					}
				}
				sort.Strings(accepted)
				if strings.Join(accepted, ",") != strings.Join(want, ",") {
					t.Fatalf("approval=%v review=%v in %s: the PATCH accepted %v, want exactly %v", approval, review, label, accepted, want)
				}
			}
		}
	}
}

// A request that names the state the change is in is no move. The states staff can
// never choose (scheduled, customer_approval, authorize, and rollback outside the
// review states) keep the refusal of their own: naming them is refused whether or
// not the change is already there.
func TestChangeRequestTransitionsIntegration_ResendsAreNoOps(t *testing.T) {
	f := newCustomerGroupFlow(t)
	// Changes in the states that hold a stage a resend would otherwise be asked to
	// provision (a resent Request Approval or customer_review restates its stage:
	// idempotent, but only a no-op once the stage is there) are walked there.
	inAssess := func() string {
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.requestApproval(id)
		return id
	}
	inCustomerReview := func() string {
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.driveToCustomerReview(id)
		return id
	}
	direct := func(state string, review bool) func() string {
		return func() string { return f.transitionFixture(state, false, review) }
	}
	for _, tc := range []struct {
		state, requested string
		fixture          func() string
		ok               bool
	}{
		{"NEW", "new", direct("NEW", false), true},
		{"ASSESS", "assess", inAssess, true},
		{"IMPLEMENT", "implement", direct("IMPLEMENT", false), true},
		{"REVIEW", "review", direct("REVIEW", false), true},
		{"CUSTOMER_REVIEW", "customer_review", inCustomerReview, true},
		{"CLOSED", "closed", direct("CLOSED", false), true},
		{"CANCELED", "canceled", direct("CANCELED", false), true},
		{"AUTHORIZE", "authorize", direct("AUTHORIZE", false), false},
		{"CUSTOMER_APPROVAL", "customer_approval", direct("CUSTOMER_APPROVAL", false), false},
		{"SCHEDULED", "scheduled", direct("SCHEDULED", false), false},
		{"ROLLBACK", "rollback", direct("ROLLBACK", false), false},
	} {
		id := tc.fixture()
		if got := f.state(id); got != tc.state {
			t.Fatalf("fixture for %s is in %s", tc.state, got)
		}
		before := fmt.Sprint(f.state(id), f.stages(id))
		err := f.attemptState(id, tc.requested)
		if tc.ok && err != nil {
			t.Errorf("a resent %s in %s: %v", tc.requested, tc.state, err)
		}
		if !tc.ok {
			f.wantValidationError(fmt.Sprintf("a resent %s in %s", tc.requested, tc.state), err, "")
		}
		if after := fmt.Sprint(f.state(id), f.stages(id)); after != before {
			t.Errorf("a resent %s in %s changed the change:\n  before: %s\n  after:  %s", tc.requested, tc.state, before, after)
		}
	}
}

// Cancel, then try to revive: out of Customer Approval and Customer Review (with
// the customer's request live, and with nobody asked) and out of Closed, no
// request moves the change, whatever it names, however it is spelled -- and
// nothing is written.
func TestChangeRequestTransitionsIntegration_CanceledOrClosedCannotBeRevived(t *testing.T) {
	// The revivals first: the states a canceled change was sent to with no customer
	// answer (implement, review, closed, customer_review), then the rest.
	everything := []string{"implement", "review", "closed", "customer_review", "scheduled", "rollback", "assess", "authorize", "customer_approval", "new"}

	revive := func(t *testing.T, f *crFlow, id, finalState string, also ...string) {
		t.Helper()
		if got := f.state(id); got != finalState {
			t.Fatalf("the change is %s, want %s", got, finalState)
		}
		name := strings.ToLower(finalState)
		targets := append(append([]string{}, everything...), also...)
		for _, requested := range targets {
			if requested == name {
				continue
			}
			f.wantValidationRefusal(name+" -> "+requested, id, requested, finalMsg(requested, finalState))
		}
		// However it is spelled.
		for _, spelled := range []string{" IMPLEMENT ", "Implement", " REVIEW", "\tcustomer_review"} {
			before := f.bypassSnapshot(id)
			_, err := f.patchState(id, domain.ChangeRequestState(spelled))
			f.wantValidationError(name+" -> "+spelled, err, "a change request that is "+strings.NewReplacer("ROLLBACK", "rolled back", "CLOSED", "closed", "CANCELED", "canceled").Replace(finalState)+" cannot be moved")
			if after := f.bypassSnapshot(id); after != before {
				t.Fatalf("%s -> %q changed the change request", name, spelled)
			}
		}
		// Nothing is waiting on anybody any more, and nothing was woken.
		if n := f.requestedApprovers(id); n != 0 {
			t.Fatalf("%d approver rows REQUESTED on a %s change after the refused revivals", n, name)
		}
		if got := f.legal(id); got != nil {
			t.Fatalf("legalNextStates of a %s change = %v, want none", name, got)
		}
		// The window and the other fields can still be edited: only the state is final.
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{WorkNote: sp("after the fact")}); err != nil {
			t.Fatalf("a work note on a %s change: %v", name, err)
		}
	}

	t.Run("canceled in Customer Approval with the customer's request live", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.driveToCustomerApproval(id)
		if got := liveStages(f.customerStages(id)); got != 1 {
			t.Fatalf("live customer stages = %d, want 1", got)
		}
		f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
		revive(t, f, id, "CANCELED", "authorize")
		if approved, reviewed := f.customerOutcome(id); approved || reviewed {
			t.Fatal("a customer answer was recorded on the canceled change")
		}
	})
	t.Run("canceled in Customer Approval with nobody to ask", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, f.noContactProject(), true, true)
		f.setPlanned(id, rsStart1, rsEnd1)
		// Request Approval refuses a project nobody can be asked on: the change gets to
		// the gate with nobody asked by the residual edge (the contact it had when
		// approval was requested is gone before the gate).
		f.requestApprovalThenContactsLeave(id)
		f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
		if got := len(f.customerStages(id)); got != 0 {
			t.Fatalf("customer stages = %d, want none (nobody to ask)", got)
		}
		f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
		revive(t, f, id, "CANCELED")
	})
	t.Run("a legacy change in Customer Approval, flag false, synced stages", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.legacyInCustomerApproval(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), nil, true)
		f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
		revive(t, f, id, "CANCELED")
	})
	t.Run("canceled in Customer Review with the customer's request live", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.driveToCustomerReview(id)
		if got := liveStages(f.customerStages(id)); got != 1 {
			t.Fatalf("live customer stages = %d, want 1", got)
		}
		f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
		revive(t, f, id, "CANCELED")
		if _, reviewed := f.customerOutcome(id); reviewed {
			t.Fatal("a customer review was recorded on the canceled change")
		}
	})
	t.Run("canceled in Customer Review with nobody to ask", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, f.noContactProject(), false, true)
		f.requestApprovalThenContactsLeave(id) // the residual edge, as above
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
		f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "rollback", "canceled")
		f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
		revive(t, f, id, "CANCELED")
	})
	t.Run("closed through Review", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
		f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
		revive(t, f, id, "CLOSED", "canceled")
	})
	t.Run("closed by the customer's own review", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.driveToCustomerReview(id)
		f.customerConfirmsReview(id)
		f.expect(id, "after the customer's review", "CLOSED")
		revive(t, f, id, "CLOSED", "canceled")
	})
	t.Run("rolled back by the customer's rejected review", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.driveToCustomerReview(id)
		if _, err := f.reviewAs(id, crScopeUserA1, false); err != nil {
			t.Fatalf("the customer's rejection: %v", err)
		}
		f.expect(id, "after the customer's rejected review", "ROLLBACK")
		revive(t, f, id, "ROLLBACK", "canceled")
	})
	t.Run("canceled by the customer's rejection of the approval", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.driveToCustomerApproval(id)
		if _, err := f.approveAs(id, crScopeUserA1, false); err != nil {
			t.Fatalf("the customer's rejection: %v", err)
		}
		f.expect(id, "after the customer's rejection", "CANCELED")
		revive(t, f, id, "CANCELED")
	})
}

// A change with BOTH customer boxes ticked, walked through its lifecycle: at every
// step the requests that would skip a gate are refused with a message that says
// what the change is waiting for, nothing changes, and the legitimate way on still
// works afterwards.
func TestChangeRequestTransitionsIntegration_NoStepIsSkipped(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	f.setPlanned(id, rsStart1, rsEnd1)

	jump := func(from string, requested string, why, opens string) {
		t.Helper()
		f.wantValidationRefusal(from+" -> "+requested, id, requested,
			fmt.Sprintf(`state %q cannot be set manually from %s: %s; the moves open to staff from %s are: %s`, requested, from, why, from, opens))
	}

	// New: approval has not been requested.
	const whyNew = `approval has not been requested yet (Request Approval is state "assess")`
	for _, requested := range []string{"implement", "review", "closed", "customer_review"} {
		jump("new", requested, whyNew, "assess, canceled")
	}
	for _, requested := range []string{"scheduled", "authorize", "customer_approval", "rollback"} {
		f.wantValidationRefusal("new -> "+requested, id, requested, "")
	}
	f.expect(id, "after the refused jumps out of New", "NEW", "assess", "canceled")

	// Assess: waiting for the peer approval.
	f.requestApproval(id)
	const whyAssess = "it is waiting for its peer approval, which moves it on by itself"
	for _, requested := range []string{"implement", "review", "closed", "customer_review"} {
		jump("assess", requested, whyAssess, "only canceled")
	}
	for _, requested := range []string{"scheduled", "authorize", "customer_approval", "rollback"} {
		f.wantValidationRefusal("assess -> "+requested, id, requested, "")
	}
	f.expect(id, "after the refused jumps out of Assess", "ASSESS", "canceled")
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}

	// Authorize: waiting for the CAB approval.
	f.expect(id, "after the peer approval", "AUTHORIZE", "canceled")
	const whyAuthorize = "it is waiting for its CAB approval, which moves it on by itself (to Customer Approval first when the customer's approval is required)"
	for _, requested := range []string{"implement", "review", "closed", "customer_review"} {
		jump("authorize", requested, whyAuthorize, "only canceled")
	}
	for _, requested := range []string{"scheduled", "customer_approval", "rollback"} {
		f.wantValidationRefusal("authorize -> "+requested, id, requested, "")
	}
	f.expect(id, "after the refused jumps out of Authorize", "AUTHORIZE", "canceled")
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}

	// Customer Approval: the customer's answer, not a staff action.
	f.expect(id, "after the CAB approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	for _, requested := range []string{"scheduled", "implement", "review", "closed", "customer_review", "rollback", "assess", "new"} {
		f.wantValidationRefusal("customer_approval -> "+requested, id, requested, "")
	}
	f.expect(id, "after the refused jumps out of Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.customerApproves(id)

	// Scheduled: Start implementation is the next step, nothing later.
	f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
	const whyRun = "a change request goes through implement and review in order, one step at a time"
	for _, requested := range []string{"review", "closed", "customer_review"} {
		jump("scheduled", requested, whyRun, "implement, canceled")
	}
	for _, requested := range []string{"authorize", "assess", "rollback"} {
		f.wantValidationRefusal("scheduled -> "+requested, id, requested, "")
	}
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")

	// Implement: Mark implemented is the next step.
	for _, requested := range []string{"closed", "customer_review"} {
		jump("implement", requested, whyRun, "review, canceled")
	}
	for _, requested := range []string{"scheduled", "authorize", "assess", "rollback"} {
		f.wantValidationRefusal("implement -> "+requested, id, requested, "")
	}
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")

	// Review with the customer's review required: Customer Review, never Closed;
	// and never back.
	f.wantValidationRefusal("review -> closed", id, "closed",
		`state "closed" cannot be set from review: customer review is required for this change request (customerReviewRequired is true); move it to customer_review first`)
	jump("review", "implement", "a change request cannot go back to an earlier step", "customer_review, rollback, canceled")
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	f.customerConfirmsReview(id)
	f.expect(id, "after the customer's review", "CLOSED")
}

// The same two refusals on a change whose customer review is not required: Review
// offers Closed, and Customer Review is not a state it can reach.
func TestChangeRequestTransitionsIntegration_ReviewWithoutCustomerReview(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	f.requestApproval(id)
	f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
	f.wantValidationRefusal("review -> customer_review", id, "customer_review",
		`state "customer_review" cannot be set: customer review is not required for this change request (customerReviewRequired is false); close it from review instead`)
	f.wantValidationRefusal("review -> scheduled", id, "scheduled", "")
	f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
}

// The requested state is trimmed and read without regard to case, whatever the
// caller sent; a value that is not a state is refused outright.
func TestChangeRequestTransitionsIntegration_SpellingOfTheRequestedState(t *testing.T) {
	f := newCustomerGroupFlow(t)

	// A skipped step stays skipped in any spelling.
	id := f.transitionFixture("NEW", true, true)
	for _, spelled := range []string{" IMPLEMENT ", "Implement", "\timplement\n", "REVIEW", "Closed", "  customer_review"} {
		f.wantValidationRefusal("new -> "+fmt.Sprintf("%q", spelled), id, spelled, "")
	}
	// An allowed one is allowed in any spelling.
	if _, err := f.patchState(id, "  Assess "); err != nil {
		t.Fatalf("Request Approval spelled \"  Assess \": %v", err)
	}
	f.expect(id, "after Request Approval", "ASSESS", "canceled")
	if _, err := f.patchState(id, " CANCELED\t"); err != nil {
		t.Fatalf("Cancel spelled \" CANCELED\\t\": %v", err)
	}
	f.expect(id, "after Cancel", "CANCELED")

	// Not a state.
	id = f.transitionFixture("NEW", false, false)
	for _, bad := range []string{"frobnicate", "", "   ", "in_review", "assess;"} {
		before := f.bypassSnapshot(id)
		_, err := f.patchState(id, domain.ChangeRequestState(bad))
		f.wantValidationError(fmt.Sprintf("state %q", bad), err, fmt.Sprintf("state %q is not a change request state", bad))
		if after := f.bypassSnapshot(id); after != before {
			t.Fatalf("a refused state %q changed the change request", bad)
		}
	}
}

// Rows in the shape the sync leaves them -- no labels on the stages, our own
// requirement columns false, the sync-owned flags as ServiceNow wrote them -- are
// held to the same table: a jump is refused, Cancel works, and nothing moves a
// finished one.
func TestChangeRequestTransitionsIntegration_MigratedShapedRows(t *testing.T) {
	f := newCustomerGroupFlow(t)
	cab := crCABGroupID
	migrated := func(state string, review bool) string {
		id := f.createWithProject(domain.ChangeRequestTypeNormal, nil, false, false)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.setState(id, state)
		f.execSQL(`UPDATE change_request SET customer_review_required = $2, is_customer_approval_required = NULL, is_customer_review_required = NULL WHERE id = $1`, id, review)
		// Devops Approval, CAB Approval, and a second Devops Approval stage that was
		// never required: unlabeled, decided, as the sync writes them.
		f.seedSyncedStage(id, nil, 60, map[string]string{crFlowPeerAID: "APPROVED"})
		f.seedSyncedStage(id, &cab, 50, map[string]string{crCABMemberUserID1: "APPROVED"})
		f.seedSyncedStage(id, nil, 40, map[string]string{crFlowPeerBID: "NOT_REQUIRED"})
		return id
	}
	for _, state := range []string{"AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "ROLLBACK", "CLOSED", "CANCELED"} {
		for _, review := range []bool{false, true} {
			want := wantStaffMoves(state, review)
			var accepted []string
			for _, requested := range []string{"assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "rollback", "closed", "canceled"} {
				if requested == strings.ToLower(state) {
					continue
				}
				id := migrated(state, review)
				before := f.bypassSnapshot(id)
				err := f.attemptState(id, requested)
				if err == nil {
					accepted = append(accepted, requested)
					continue
				}
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("%s review=%v -> %s: %v (%T), want a 400", state, review, requested, err, err)
				}
				if after := f.bypassSnapshot(id); after != before {
					t.Fatalf("%s review=%v -> %s: a refused PATCH changed the change request", state, review, requested)
				}
			}
			sort.Strings(accepted)
			if strings.Join(accepted, ",") != strings.Join(want, ",") {
				t.Fatalf("migrated-shaped %s review=%v: the PATCH accepted %v, want exactly %v", state, review, accepted, want)
			}
		}
	}
}
