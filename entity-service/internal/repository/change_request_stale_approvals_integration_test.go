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
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// An approval is only actionable while the change request is in the state its
// stage belongs to. Reported bug: a reviewer kept Approve / Reject on the Review
// stage of a change that was already Closed (and during Customer Review, when
// the customer should be answering), because nothing cancelled the Review
// stage's still-requested approvers when the change left Review.
//
// These tests assert the stages, the approver rows' statuses and canDecide for
// the assigned internal approvers and the customer contacts after EVERY step of
// each lifecycle. Same harness as TestChangeRequestFlowIntegration_*
// (crFlow, DSN-gated by CHANGE_REQUEST_TEST_DSN):
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run StaleApprovals

var (
	// The assigned group's approvers: who gets the Peer and Review rows (the
	// creator is listed cancelled).
	crStaleAssigned = []string{crFlowPeerAID, crFlowPeerBID, crFlowOutsiderID}
	// Everybody whose canDecide the lifecycle tests look at.
	crStaleViewers = []string{
		crFlowCreatorID, crFlowPeerAID, crFlowPeerBID, crFlowOutsiderID,
		crCABMemberUserID1, crCABMemberUserID2, crECABMemberUserID, crScopeUserA1, crScopeUserA2,
	}
)

func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

// stageKey names a stage in assertions: its label, and "#2", "#3" ... for a
// label that repeats (a re-scheduled CAB / customer stage).
func stageKey(seen map[string]int, label string) string {
	seen[label]++
	if seen[label] == 1 {
		return label
	}
	return fmt.Sprintf("%s#%d", label, seen[label])
}

// liveRows maps each stage (see stageKey) that still has a REQUESTED approver to
// the sorted user ids holding one.
func (f *crFlow) liveRows(id string) map[string][]string {
	f.t.Helper()
	out := map[string][]string{}
	seen := map[string]int{}
	for _, st := range f.stages(id) {
		key := stageKey(seen, st.label)
		var users []string
		for uid, status := range st.approvers {
			if status == "REQUESTED" {
				users = append(users, uid)
			}
		}
		if len(users) > 0 {
			sort.Strings(users)
			out[key] = users
		}
	}
	return out
}

// wantLive asserts exactly which approver rows are REQUESTED, per stage.
func (f *crFlow) wantLive(id, when string, want map[string][]string) {
	f.t.Helper()
	norm := func(m map[string][]string) string {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, sortedCopy(m[k])))
		}
		return strings.Join(parts, "; ")
	}
	if got := f.liveRows(id); norm(got) != norm(want) {
		f.t.Fatalf("REQUESTED approver rows %s = {%s}, want {%s}", when, norm(got), norm(want))
	}
	total := 0
	for _, users := range want {
		total += len(users)
	}
	if n := f.requestedApprovers(id); n != total {
		f.t.Fatalf("requested approver rows %s = %d, want %d", when, n, total)
	}
}

// canDecideStages lists the stages (see stageKey) on which the viewer's own
// row reads canDecide: true.
func (f *crFlow) canDecideStages(id, viewer string) []string {
	f.t.Helper()
	var out []string
	seen := map[string]int{}
	for _, a := range f.approvalsAs(id, viewer).Approvals {
		key := stageKey(seen, a.Stage)
		for _, ap := range a.Approvers {
			if ap.CanDecide {
				out = append(out, key)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// wantCanDecide asserts, for every viewer in crStaleViewers, the stages on which
// they can decide; viewers absent from want must be able to decide nowhere.
func (f *crFlow) wantCanDecide(id, when string, want map[string][]string) {
	f.t.Helper()
	for _, viewer := range crStaleViewers {
		got := f.canDecideStages(id, viewer)
		w := sortedCopy(want[viewer])
		if strings.Join(got, ",") != strings.Join(w, ",") {
			f.t.Fatalf("canDecide of %s %s = %v, want %v", viewer, when, got, w)
		}
	}
}

// everyone is a convenience for wantCanDecide: the same stages for each user.
func everyone(users []string, stages ...string) map[string][]string {
	out := map[string][]string{}
	for _, u := range users {
		out[u] = stages
	}
	return out
}

func mergeWant(parts ...map[string][]string) map[string][]string {
	out := map[string][]string{}
	for _, p := range parts {
		for k, v := range p {
			out[k] = v
		}
	}
	return out
}

// snapshot maps "stage/user" -> status for every approver row of the change.
func (f *crFlow) snapshot(id string) map[string]string {
	f.t.Helper()
	out := map[string]string{}
	seen := map[string]int{}
	for _, st := range f.stages(id) {
		key := stageKey(seen, st.label)
		for uid, status := range st.approvers {
			out[key+"/"+uid] = status
		}
	}
	return out
}

func (f *crFlow) wantStatuses(id, when, stage string, want map[string]string) {
	f.t.Helper()
	snap := f.snapshot(id)
	for uid, status := range want {
		if got := snap[stage+"/"+uid]; got != status {
			f.t.Fatalf("%s: %s approver %s = %q, want %q (all: %v)", when, stage, uid, got, status, snap)
		}
	}
}

func (f *crFlow) wantConflict(what string, err error, want string) {
	f.t.Helper()
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) {
		f.t.Fatalf("%s: err = %v (%T), want *apierror.ConflictError", what, err, err)
	}
	if ce.Msg != want {
		f.t.Fatalf("%s: message\n  %q\nwant\n  %q", what, ce.Msg, want)
	}
}

// (a) Normal, customerReviewRequired: Implement -> Review -> Customer Review ->
// customer approves -> Closed. The Review rows are REQUESTED and decidable in
// Review, cancelled and not decidable the moment the change moves on, and
// nothing is requested anywhere once it is Closed.
func TestChangeRequestFlowIntegration_StaleApprovals_ReviewToCustomerReviewToClosed(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)

	f.requestApproval(id)
	f.expect(id, "in Assess", "ASSESS", "canceled")
	f.wantLive(id, "in Assess", map[string][]string{"Peer Approval": crStaleAssigned})
	f.wantCanDecide(id, "in Assess", everyone(crStaleAssigned, "Peer Approval"))

	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	f.expect(id, "in Authorize", "AUTHORIZE", "canceled")
	f.wantLive(id, "in Authorize", map[string][]string{"CAB Approval": {crCABMemberUserID1, crCABMemberUserID2}})
	f.wantCanDecide(id, "in Authorize", everyone([]string{crCABMemberUserID1, crCABMemberUserID2}, "CAB Approval"))

	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.expect(id, "in Scheduled", "SCHEDULED", "implement", "canceled")
	f.wantLive(id, "in Scheduled", nil)
	f.wantCanDecide(id, "in Scheduled", nil)

	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.wantLive(id, "in Implement", nil)
	f.wantCanDecide(id, "in Implement", nil)

	// Review: the assigned group's internal members are asked and can decide.
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	f.wantLive(id, "in Review", map[string][]string{"Review": crStaleAssigned})
	f.wantCanDecide(id, "in Review", everyone(crStaleAssigned, "Review"))
	f.wantStatuses(id, "in Review", "Review", map[string]string{crFlowCreatorID: "CANCELLED"})

	// Customer Review: the Review rows are cancelled on the way, so the reviewers
	// can no longer approve or reject; the customer contacts can.
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	f.wantLive(id, "in Customer Review", map[string][]string{"Customer Review": {crScopeUserA1, crScopeUserA2}})
	f.wantCanDecide(id, "in Customer Review", everyone([]string{crScopeUserA1, crScopeUserA2}, "Customer Review"))
	for _, uid := range crStaleAssigned {
		f.wantStatuses(id, "in Customer Review", "Review", map[string]string{uid: "CANCELLED"})
	}
	// Nothing of theirs is pending any more: they are told who answers now.
	f.wantForbidden("a Review approver deciding in Customer Review", f.decide(id, crFlowPeerAID, "approved"), "only members of the customer group")
	f.expect(id, "after the refused Review decision", "CUSTOMER_REVIEW", "canceled")

	// The customer approves: Closed, and nothing is requested anywhere.
	if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
		t.Fatalf("customer review: %v", err)
	}
	f.expect(id, "in Closed", "CLOSED")
	f.wantLive(id, "in Closed", nil)
	f.wantCanDecide(id, "in Closed", nil)
	f.wantStatuses(id, "in Closed", "Customer Review", map[string]string{crScopeUserA1: "APPROVED", crScopeUserA2: "CANCELLED"})
}

// (b) Review -> Closed directly (customer review not required): the Review rows
// are cancelled by the move, whether or not one of the reviewers had decided.
func TestChangeRequestFlowIntegration_StaleApprovals_ReviewToClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		decide bool
	}{{"nobody decided the Review stage", false}, {"one reviewer approved the Review stage first", true}} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
			f.requestApproval(id)
			f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
			f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
			f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
			f.wantLive(id, "in Review", map[string][]string{"Review": crStaleAssigned})
			f.wantCanDecide(id, "in Review", everyone(crStaleAssigned, "Review"))

			if tc.decide {
				// Deciding Review records the answer and cancels the siblings; the
				// change stays in Review (a human moves it on).
				if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
					t.Fatalf("Review approval: %v", err)
				}
				f.expect(id, "after the Review approval", "REVIEW", "closed", "rollback", "canceled")
				f.wantLive(id, "after the Review approval", nil)
				f.wantCanDecide(id, "after the Review approval", nil)
				f.wantStatuses(id, "after the Review approval", "Review", map[string]string{
					crFlowPeerAID: "APPROVED", crFlowPeerBID: "CANCELLED", crFlowOutsiderID: "CANCELLED"})
			}

			f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
			f.wantLive(id, "in Closed", nil)
			f.wantCanDecide(id, "in Closed", nil)
			want := map[string]string{crFlowPeerBID: "CANCELLED", crFlowOutsiderID: "CANCELLED", crFlowCreatorID: "CANCELLED"}
			if tc.decide {
				want[crFlowPeerAID] = "APPROVED"
			} else {
				want[crFlowPeerAID] = "CANCELLED"
			}
			f.wantStatuses(id, "in Closed", "Review", want)
		})
	}
}

// (c) Review -> Rollback (customer review ticked or not) and Customer Review ->
// Rollback (no customer group, so the manual path): nothing stays requested.
func TestChangeRequestFlowIntegration_StaleApprovals_Rollback(t *testing.T) {
	for _, review := range []bool{false, true} {
		review := review
		t.Run(fmt.Sprintf("review->rollback, customerReviewRequired=%v", review), func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, review)
			f.requestApproval(id)
			f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
			f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
			if _, err := f.patchState(id, domain.ChangeRequestStateReview); err != nil {
				t.Fatalf("Review: %v", err)
			}
			f.wantLive(id, "in Review", map[string][]string{"Review": crStaleAssigned})
			f.wantCanDecide(id, "in Review", everyone(crStaleAssigned, "Review"))

			if _, err := f.patchState(id, domain.ChangeRequestStateRollback); err != nil {
				t.Fatalf("Roll back: %v", err)
			}
			f.wantRolledBack(id)
			f.wantLive(id, "in Rollback", nil)
			f.wantCanDecide(id, "in Rollback", nil)
		})
	}
	t.Run("customer review->rollback with no customer group", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		// Project C has no registered contact at the review: no Customer Review stage
		// (Request Approval refuses the project, so the contact it had when approval was
		// requested is gone before the review: the residual edge).
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), false, true)
		f.requestApprovalThenContactsLeave(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
		f.wantLive(id, "in Review", map[string][]string{"Review": crStaleAssigned})
		f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "rollback", "canceled")
		f.wantLive(id, "in Customer Review", nil)
		f.wantCanDecide(id, "in Customer Review", nil)
		if _, err := f.patchState(id, domain.ChangeRequestStateRollback); err != nil {
			t.Fatalf("Roll back: %v", err)
		}
		f.wantRolledBack(id)
		f.wantCanDecide(id, "in Rollback", nil)
	})
}

// (d) Cancel from every state that holds an approval (and from those that do
// not): every requested row is cancelled, internal stages included -- this
// supersedes "Cancel does not cancel the internal stages' pending approvers".
// Rows that were already decided keep their status.
func TestChangeRequestFlowIntegration_StaleApprovals_CancelFromEveryState(t *testing.T) {
	internal := func(stage string) map[string][]string { return map[string][]string{stage: crStaleAssigned} }
	cab := []string{crCABMemberUserID1, crCABMemberUserID2}
	cust := []string{crScopeUserA1, crScopeUserA2}
	for _, tc := range []struct {
		name             string
		approval, review bool
		drive            func(f *crFlow, id string)
		wantState        string
		live             map[string][]string
		decidable        map[string][]string
	}{
		{"assess", false, false, func(f *crFlow, id string) { f.requestApproval(id) }, "ASSESS",
			internal("Peer Approval"), everyone(crStaleAssigned, "Peer Approval")},
		{"authorize", false, false, func(f *crFlow, id string) {
			f.requestApproval(id)
			if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
				f.t.Fatalf("peer approval: %v", err)
			}
		}, "AUTHORIZE", map[string][]string{"CAB Approval": cab}, everyone(cab, "CAB Approval")},
		{"scheduled", false, false, func(f *crFlow, id string) {
			f.requestApproval(id)
			f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		}, "SCHEDULED", nil, nil},
		{"customer_approval", true, false, func(f *crFlow, id string) { f.driveToCustomerApproval(id) }, "CUSTOMER_APPROVAL",
			map[string][]string{"Customer Approval": cust}, everyone(cust, "Customer Approval")},
		{"implement", false, false, func(f *crFlow, id string) {
			f.requestApproval(id)
			f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
			f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		}, "IMPLEMENT", nil, nil},
		{"review", false, false, func(f *crFlow, id string) {
			f.requestApproval(id)
			f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
			f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
			f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
		}, "REVIEW", internal("Review"), everyone(crStaleAssigned, "Review")},
		{"customer_review", false, true, func(f *crFlow, id string) {
			f.requestApproval(id)
			f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
			f.driveToCustomerReview(id)
		}, "CUSTOMER_REVIEW", map[string][]string{"Customer Review": cust}, everyone(cust, "Customer Review")},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), tc.approval, tc.review)
			tc.drive(f, id)
			f.expect(id, "before Cancel", tc.wantState, f.legal(id)...)
			f.wantLive(id, "before Cancel", tc.live)
			f.wantCanDecide(id, "before Cancel", tc.decidable)
			before := f.snapshot(id)

			f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
			f.wantLive(id, "after Cancel", nil)
			f.wantCanDecide(id, "after Cancel", nil)
			after := f.snapshot(id)
			if len(after) != len(before) {
				t.Fatalf("Cancel changed the set of approver rows: %v -> %v", before, after)
			}
			for key, was := range before {
				want := was
				if was == "REQUESTED" {
					want = "CANCELLED"
				}
				if after[key] != want {
					t.Fatalf("Cancel: approver row %s = %q, want %q (was %q)", key, after[key], want, was)
				}
			}
			// Cancel is final: every approver reads cancelled/approved/..., none can decide.
			for _, uid := range crStaleViewers {
				if err := f.decide(id, uid, "approved"); err == nil {
					t.Fatalf("%s decided an approval on a cancelled change request", uid)
				}
			}
		})
	}
}

// (e) Re-schedule loop (Customer Approval, which does not move): the customers' request
// is replaced by a fresh one that IS actionable at once -- no CAB stage in between -- and
// the superseded customer stage stays as a cancelled record; a later Review stage is still
// provisioned and is decidable in Review only.
func TestChangeRequestFlowIntegration_StaleApprovals_RescheduleLoop(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	f.setPlanned(id, rsStart1, rsEnd1)
	cab := []string{crCABMemberUserID1, crCABMemberUserID2}
	cust := []string{crScopeUserA1, crScopeUserA2}

	f.requestApproval(id)
	f.wantLive(id, "in Assess", map[string][]string{"Peer Approval": crStaleAssigned})
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	f.wantLive(id, "in Authorize", map[string][]string{"CAB Approval": cab})
	f.wantCanDecide(id, "in Authorize", everyone(cab, "CAB Approval"))
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantLive(id, "in Customer Approval", map[string][]string{"Customer Approval": cust})
	f.wantCanDecide(id, "in Customer Approval", everyone(cust, "Customer Approval"))

	// Re-schedule: the change stays in Customer Approval, the old request is cancelled
	// and a fresh one is live; nothing else is asked.
	if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
		t.Fatalf("re-schedule: %v", err)
	}
	f.expect(id, "after Re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval,Customer Approval" {
		t.Fatalf("stages after Re-schedule = %s (no CAB again)", got)
	}
	f.wantLive(id, "after Re-schedule", map[string][]string{"Customer Approval#2": cust})
	f.wantCanDecide(id, "after Re-schedule", everyone(cust, "Customer Approval#2"))
	f.wantStatuses(id, "after Re-schedule", "Customer Approval", map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
	f.wantStatuses(id, "after Re-schedule", "CAB Approval", map[string]string{crCABMemberUserID1: "APPROVED", crCABMemberUserID2: "CANCELLED"})

	if err := f.decide(id, crScopeUserA2, "approved"); err != nil {
		t.Fatalf("customer approval: %v", err)
	}
	f.expect(id, "in Scheduled", "SCHEDULED", "implement", "canceled")
	f.wantLive(id, "in Scheduled", nil)
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.wantLive(id, "in Implement", nil)

	// The Review stage is still provisioned (the repeated customer stage did not shift
	// its ordinal) and is decidable in Review only.
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval,Customer Approval,Review" {
		t.Fatalf("stages in Review = %s", got)
	}
	f.wantLive(id, "in Review", map[string][]string{"Review": crStaleAssigned})
	f.wantCanDecide(id, "in Review", everyone(crStaleAssigned, "Review"))

	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	f.wantLive(id, "in Customer Review", map[string][]string{"Customer Review": cust})
	f.wantCanDecide(id, "in Customer Review", everyone(cust, "Customer Review"))
	if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
		t.Fatalf("customer review: %v", err)
	}
	f.expect(id, "in Closed", "CLOSED")
	f.wantLive(id, "in Closed", nil)
	f.wantCanDecide(id, "in Closed", nil)
}

// A change that was re-scheduled under the OLD flow -- back in Authorize with a fresh
// CAB stage live and the customer's request cancelled -- may still be in flight when this
// code runs. It finishes through the CAB as it always did (no data fix): the approval
// cascades to Customer Approval and asks the customers afresh. It never reads as a customer's
// proposal waiting for WSO2 (it is in Authorize and nobody wrote customer_updated_on).
func TestChangeRequestFlowIntegration_StaleApprovals_OldFlowRescheduleStillFinishesThroughTheCAB(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	// Reconstruct what the old Re-schedule left: the window moved, the state back in
	// Authorize, the customer's rows cancelled, a fresh CAB stage with live rows.
	f.setPlanned(id, rsStart2, rsEnd2)
	f.setState(id, "AUTHORIZE")
	f.execSQL(`UPDATE approval_stage_approver SET state = 'CANCELLED'
	           WHERE work_item_id = $1 AND stage_id IN (SELECT id FROM approval_stage WHERE work_item_id = $1 AND checkpoint_label = 'Customer Approval')`, id)
	cabGroup := crCABGroupID
	f.seedLooseStageInGroup(id, sp("CAB Approval"), &cabGroup, 1, map[string]string{crCABMemberUserID1: "REQUESTED", crCABMemberUserID2: "REQUESTED"})
	f.expect(id, "in Authorize again", "AUTHORIZE", "canceled")
	f.wantAnswer(id, "in Authorize", "none")
	// A stale proposal date does not make it a proposal, and none of the new acts takes it.
	f.syncWritesConversation(id, sp(rsStart3), "")
	f.wantAnswer(id, "in Authorize with a stale date", "unanswered")
	_, err := f.accept(id)
	if err == nil {
		t.Fatal("Accept proposed time was accepted in Authorize")
	}

	if err := f.decide(id, crCABMemberUserID2, "approved"); err != nil {
		t.Fatalf("CAB approval of the old-flow re-schedule: %v", err)
	}
	f.expect(id, "back in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantCanAnswer(id, "asked again", true, crScopeUserA1, crScopeUserA2)
	// ...and the stale date, now that the change is back in Customer Approval, is a date that differs
	// from the plan with no answer: it WAITS, but nobody is named as its proposer (the last writer is
	// the CAB approver, not a contact of the project), so it is no customer's proposal. Accept is
	// refused for it (no staff action stands in for a consent nobody gave), a customer is not told
	// it is theirs or that WSO2 is deciding on it, and a Re-schedule is a plain one. (C3: a stale
	// date on re-entry never reads as "the customer proposed".)
	f.wantAnswer(id, "back in Customer Approval with the stale date", "pending")
	p := f.proposalOf(id)
	if p == nil || p.ProposerRecorded == nil || *p.ProposerRecorded {
		t.Fatalf("customerProposal on re-entry = %+v, want a pending time whose proposer is not recorded", p)
	}
	if p.ProposedByName != nil || p.ProposedByEmail != nil || p.ProposedOn != nil {
		t.Fatalf("a stale date on re-entry was attributed to somebody: %+v", p)
	}
	if p.CanAccept == nil || *p.CanAccept || p.AcceptBlockedReason == nil || !strings.HasPrefix(*p.AcceptBlockedReason, msgAcceptNobodyRecorded) {
		t.Fatalf("a stale date on re-entry can be accepted: %+v", p)
	}
	before := f.snap(id)
	_, err = f.accept(id)
	wantRefusalCode(t, "Accept of a stale date on re-entry", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
	f.wantRefusedSame("Accept of a stale date on re-entry", id, before, err)
	if seen, err := f.getAsContact(id, crScopeUserA1); err != nil || seen.CustomerProposal == nil || seen.CustomerProposal.Answer != "unanswered" {
		t.Fatalf("a customer's view of a stale date on re-entry = %+v (%v), want it as history", seen.CustomerProposal, err)
	}
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("the customer's approval after the old-flow re-schedule: %v", err)
	}
	f.expect(id, "in Scheduled", "SCHEDULED", "implement", "canceled")
	f.wantPlanned(id, "in Scheduled", rsStart2, rsEnd2)
	f.wantAnswer(id, "in Scheduled with the stale date", "unanswered")
}

// (e, continued) Re-schedule on a Standard change: the same -- the customer is asked again,
// in the same state, with no approval repeated. (The Emergency subtest is retired: an Emergency
// change never reaches Customer Approval; what a Re-schedule does to a legacy one that is there
// is TestChangeRequestFlowIntegration_RescheduleLegacyEmergencyInCustomerApproval.)
func TestChangeRequestFlowIntegration_StaleApprovals_RescheduleStandard(t *testing.T) {
	cust := []string{crScopeUserA1, crScopeUserA2}
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.requestApproval(id)
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantLive(id, "in Customer Approval", map[string][]string{"Customer Approval": cust})
	if err := f.reschedule(id, nil, sp(rsEnd2)); err != nil {
		t.Fatalf("re-schedule: %v", err)
	}
	f.expect(id, "after Re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantLive(id, "after Re-schedule", map[string][]string{"Customer Approval#2": cust})
	f.wantCanDecide(id, "after Re-schedule", everyone(cust, "Customer Approval#2"))
	f.wantStatuses(id, "after Re-schedule", "Customer Approval", map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
}

// (f) The Emergency and Standard paths are unaffected: the Emergency change's one CAB stage is
// decidable in Authorize and nothing else is ever requested; Standard has no internal stage
// at all and a customer stage only when Customer Approval / Review is ticked.
func TestChangeRequestFlowIntegration_StaleApprovals_EmergencyAndStandardUnaffected(t *testing.T) {
	cust := []string{crScopeUserA1, crScopeUserA2}
	t.Run("emergency", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeEmergency, sp(crScopeProjectA), false, false)
		f.requestApproval(id)
		f.expect(id, "in Authorize", "AUTHORIZE", "canceled")
		cab := []string{crCABMemberUserID1, crCABMemberUserID2}
		f.wantLive(id, "in Authorize", map[string][]string{"CAB Approval": cab})
		f.wantCanDecide(id, "in Authorize", everyone(cab, "CAB Approval"))
		if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
			t.Fatalf("CAB approval: %v", err)
		}
		f.expect(id, "in Scheduled", "SCHEDULED", "implement", "canceled")
		f.wantLive(id, "in Scheduled", nil)
		f.wantCanDecide(id, "in Scheduled", nil)
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
		if got := f.stageLabels(id); got != "CAB Approval" {
			t.Fatalf("emergency stages in Review = %s, want only the CAB stage", got)
		}
		f.wantLive(id, "in Review", nil)
		f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
		f.wantLive(id, "in Closed", nil)
	})
	t.Run("standard", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), false, false)
		f.requestApproval(id)
		f.expect(id, "in Scheduled", "SCHEDULED", "implement", "canceled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
		f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
		if n := len(f.stages(id)); n != 0 {
			t.Fatalf("standard change has %d stages, want none", n)
		}
	})
	t.Run("standard with both customer steps", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, true)
		f.requestApproval(id)
		f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.wantLive(id, "in Customer Approval", map[string][]string{"Customer Approval": cust})
		f.wantCanDecide(id, "in Customer Approval", everyone(cust, "Customer Approval"))
		if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
			t.Fatalf("customer approval: %v", err)
		}
		f.expect(id, "in Scheduled", "SCHEDULED", "implement", "canceled")
		f.wantLive(id, "in Scheduled", nil)
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
		f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
		f.wantLive(id, "in Customer Review", map[string][]string{"Customer Review": cust})
		f.wantCanDecide(id, "in Customer Review", everyone(cust, "Customer Review"))
		if err := f.decide(id, crScopeUserA2, "approved"); err != nil {
			t.Fatalf("customer review: %v", err)
		}
		f.expect(id, "in Closed", "CLOSED")
		f.wantLive(id, "in Closed", nil)
		if got := f.stageLabels(id); got != "Customer Approval,Customer Review" {
			t.Fatalf("standard stages = %s", got)
		}
	})
}

// An approver who rejects (rather than approves) a stage resolves it as
// decisively; a later move of the change leaves nothing requested either.
func TestChangeRequestFlowIntegration_StaleApprovals_RejectedReviewThenClosed(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	f.requestApproval(id)
	f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
	if err := f.decide(id, crFlowPeerBID, "rejected"); err != nil {
		t.Fatalf("Review rejection: %v", err)
	}
	f.expect(id, "after the Review rejection", "REVIEW", "closed", "rollback", "canceled")
	f.wantLive(id, "after the Review rejection", nil)
	f.wantCanDecide(id, "after the Review rejection", nil)
	f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
	f.wantStatuses(id, "in Closed", "Review", map[string]string{crFlowPeerBID: "REJECTED", crFlowPeerAID: "CANCELLED", crFlowOutsiderID: "CANCELLED"})
}

// (g) Decision-time guard. A legacy row -- one the reconcile never saw (written
// before it existed, or by a path that does not run it) -- on a stage the change
// has left is refused with the exact message, changes nothing, and reads
// canDecide: false.
func TestChangeRequestFlowIntegration_StaleApprovals_DecisionGuard(t *testing.T) {
	const wantReviewInCustomerReview = "this approval is no longer pending: the change request is in Customer Review, but the Review stage can only be decided while it is in Review"
	reviewStale := func(f *crFlow, id, user string) {
		f.t.Helper()
		f.execSQL(`UPDATE approval_stage_approver SET state = 'REQUESTED'
		           WHERE approver_user_id = $2::uuid AND stage_id = (SELECT id FROM approval_stage WHERE work_item_id = $1 AND checkpoint_label = 'Review')`, id, user)
	}

	t.Run("a Review row left REQUESTED while the change is in Customer Review", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.driveToCustomerReview(id)
		reviewStale(f, id, crFlowPeerAID) // the legacy row

		// The read model still says REQUESTED (it is what the database holds)...
		view := f.approvalsAs(id, crFlowPeerAID)
		found := false
		for _, a := range view.Approvals {
			if a.Stage != "Review" {
				continue
			}
			for _, ap := range a.Approvers {
				if ap.ID == crFlowPeerAID {
					found = true
					if ap.Status != "REQUESTED" || ap.CanDecide {
						t.Fatalf("legacy Review row reads status=%s canDecide=%v, want REQUESTED / false", ap.Status, ap.CanDecide)
					}
				}
			}
		}
		if !found {
			t.Fatal("legacy Review row not in the approvals read model")
		}
		f.wantCanDecide(id, "with a legacy Review row", everyone([]string{crScopeUserA1, crScopeUserA2}, "Customer Review"))

		// ...but deciding it is refused, for approval and rejection alike.
		for _, decision := range []string{"approved", "rejected"} {
			f.wantConflict("the stale Review row, "+decision, f.decide(id, crFlowPeerAID, decision), wantReviewInCustomerReview)
		}
		f.expect(id, "after the refused decisions", "CUSTOMER_REVIEW", "canceled")
		f.wantLive(id, "after the refused decisions", map[string][]string{
			"Review": {crFlowPeerAID}, "Customer Review": {crScopeUserA1, crScopeUserA2}})

		// The customer still answers; the final state then sweeps the legacy row.
		if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
			t.Fatalf("customer review: %v", err)
		}
		f.expect(id, "in Closed", "CLOSED")
		f.wantLive(id, "in Closed", nil)
	})

	t.Run("a Review row left REQUESTED on a Closed change", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
		f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
		reviewStale(f, id, crFlowPeerBID)
		f.wantCanDecide(id, "on a Closed change with a legacy row", nil)
		f.wantConflict("the Closed change's Review row", f.decide(id, crFlowPeerBID, "approved"),
			"this approval is no longer pending: the change request is in Closed, but the Review stage can only be decided while it is in Review")
		f.wantLive(id, "after the refused decision", map[string][]string{"Review": {crFlowPeerBID}})
		f.expect(id, "after the refused decision", "CLOSED")
	})

	t.Run("a Peer row left REQUESTED in Authorize", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.requestApproval(id)
		if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
			t.Fatalf("peer approval: %v", err)
		}
		f.execSQL(`UPDATE approval_stage_approver SET state = 'REQUESTED' WHERE approver_user_id = $2::uuid AND work_item_id = $1`, id, crFlowPeerBID)
		f.wantCanDecide(id, "with a legacy Peer row", everyone([]string{crCABMemberUserID1, crCABMemberUserID2}, "CAB Approval"))
		f.wantConflict("the stale Peer row", f.decide(id, crFlowPeerBID, "approved"),
			"this approval is no longer pending: the change request is in Authorize, but the Peer Approval stage can only be decided while it is in Assess")
		f.expect(id, "after the refused decision", "AUTHORIZE", "canceled")
	})

	t.Run("a caller with a stale row AND a live one decides the live one", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.requestApproval(id)
		if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
			t.Fatalf("peer approval: %v", err)
		}
		// A CAB member who also holds a (legacy) REQUESTED row on the Peer stage.
		f.forceApprover(id, "Peer Approval", crCABMemberUserID1)
		f.wantCanDecide(id, "with the stale Peer row", mergeWant(
			everyone([]string{crCABMemberUserID1}, "CAB Approval"), everyone([]string{crCABMemberUserID2}, "CAB Approval")))
		if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
			t.Fatalf("CAB approval by a caller who also holds a stale Peer row: %v", err)
		}
		f.expect(id, "after the CAB approval", "SCHEDULED", "implement", "canceled")
		// The decision resolved the CAB stage only; the closing reconcile swept the stale row.
		f.wantStatuses(id, "after the CAB approval", "CAB Approval", map[string]string{crCABMemberUserID1: "APPROVED", crCABMemberUserID2: "CANCELLED"})
		f.wantStatuses(id, "after the CAB approval", "Peer Approval", map[string]string{crCABMemberUserID1: "CANCELLED"})
		f.wantLive(id, "after the CAB approval", nil)
	})
}

// seedLooseStage inserts an approval stage (label nil = none) whose created_on is
// ageMinutes in the past -- stages are ordered by created_on, which is also their
// ordinal position -- with one approver row per entry of rows (user id -> status).
func (f *crFlow) seedLooseStage(id string, label *string, ageMinutes int, rows map[string]string) string {
	f.t.Helper()
	var stageID string
	if err := f.scoped.QueryRow(f.sys,
		`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, checkpoint_label, raw_status)
		 VALUES (gen_random_uuid(), now() - make_interval(mins => $2::int), now(), 'cr-flow-test', 'cr-flow-test', $1, $3, 'REQUESTED') RETURNING id::text`,
		id, ageMinutes, label).Scan(&stageID); err != nil {
		f.t.Fatalf("seed stage: %v", err)
	}
	for uid, status := range rows {
		f.execSQL(`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
		           VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, $2, $3::uuid, $4)`, stageID, id, uid, status)
	}
	return stageID
}

// seedLooseStageInGroup is seedLooseStage with the stage's assignment group set (what a
// stage written by the old Re-schedule, or by the sync, carries).
func (f *crFlow) seedLooseStageInGroup(id string, label *string, groupID *string, ageMinutes int, rows map[string]string) string {
	f.t.Helper()
	stageID := f.seedLooseStage(id, label, ageMinutes, rows)
	f.execSQL(`UPDATE approval_stage SET assignment_group_id = $2::uuid WHERE id = $1::uuid`, stageID, groupID)
	return stageID
}

func (f *crFlow) setState(id, state string) {
	f.t.Helper()
	if state == "" {
		f.execSQL(`UPDATE change_request SET state = NULL WHERE id = $1`, id)
		return
	}
	f.execSQL(`UPDATE change_request SET state = $2::change_request_state_enum WHERE id = $1`, id, state)
}

// (h) ServiceNow-synced data: a stage of unknown kind (an unrecognised label, or
// no label past the first two positions) and a change with a NULL state are
// never guarded -- nothing is refused, nothing is cancelled by a state move
// (only a final state sweeps them).
func TestChangeRequestFlowIntegration_StaleApprovals_UnknownStagesAreNotGuarded(t *testing.T) {
	t.Run("a stage with an unrecognised label", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.setState(id, "IMPLEMENT")
		f.seedLooseStage(id, sp("SN Change Approval"), 5, map[string]string{crFlowPeerAID: "REQUESTED", crFlowPeerBID: "REQUESTED"})
		f.wantCanDecide(id, "in Implement", everyone([]string{crFlowPeerAID, crFlowPeerBID}, "SN Change Approval"))

		// Moving on does not touch it (not a stage the flow knows; with one
		// stage already there no Review stage is provisioned either)...
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
		f.wantLive(id, "in Review", map[string][]string{"SN Change Approval": {crFlowPeerAID, crFlowPeerBID}})
		f.wantCanDecide(id, "in Review", everyone([]string{crFlowPeerAID, crFlowPeerBID}, "SN Change Approval"))

		// ...and it can be decided in any state.
		if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
			t.Fatalf("deciding the unknown stage: %v", err)
		}
		f.expect(id, "after deciding the unknown stage", "REVIEW", "closed", "rollback", "canceled")
	})

	t.Run("unlabelled stages on a change that has moved past them", func(t *testing.T) {
		// A position is only a guess for a stage with no label (the shape of a
		// ServiceNow-synced one): positions 0 and 1 read as Peer / CAB by the
		// historical convention, but only while the change is in the state that
		// guess implies. This change is in Implement, so none of the three is a
		// stage of known kind -- nothing is refused as out of state, nothing is
		// hidden from its approver, and (below) nothing is cancelled by a move
		// to another state. (Before migrated data was taken into account the first
		// two were taken for the long-past Assess / Authorize approvals and
		// refused with a 409.)
		f := newCRFlow(t)
		f.seedAssignedGroup()
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.setState(id, "IMPLEMENT")
		f.seedLooseStage(id, nil, 30, map[string]string{crFlowPeerAID: "REQUESTED"})
		f.seedLooseStage(id, nil, 20, map[string]string{crFlowPeerBID: "REQUESTED"})
		f.seedLooseStage(id, nil, 10, map[string]string{crFlowOutsiderID: "REQUESTED"})
		// The labels shown are the display ones (positional), unchanged.
		f.wantCanDecide(id, "in Implement", map[string][]string{
			crFlowPeerAID: {"Assess"}, crFlowPeerBID: {"Authorize"}, crFlowOutsiderID: {"Customer Approval"}})
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
		f.wantCanDecide(id, "in Review", map[string][]string{
			crFlowPeerAID: {"Assess"}, crFlowPeerBID: {"Authorize"}, crFlowOutsiderID: {"Customer Approval"}})
		if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
			t.Fatalf("deciding the unlabelled position-0 stage of a change that is in Review: %v", err)
		}
		if err := f.decide(id, crFlowOutsiderID, "approved"); err != nil {
			t.Fatalf("deciding the unlabelled third stage: %v", err)
		}
		f.expect(id, "after deciding the unlabelled stages", "REVIEW", "closed", "rollback", "canceled")
	})

	t.Run("a change with no state", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.setState(id, "")
		f.seedLooseStage(id, sp("Review"), 5, map[string]string{crFlowPeerAID: "REQUESTED"})
		f.wantCanDecide(id, "with a NULL state", map[string][]string{crFlowPeerAID: {"Review"}})
		if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
			t.Fatalf("deciding a Review row of a change with no state: %v", err)
		}
		f.wantStatuses(id, "after the decision", "Review", map[string]string{crFlowPeerAID: "APPROVED"})
	})

	t.Run("a final state sweeps unknown stages too", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.setState(id, "REVIEW")
		f.seedLooseStage(id, sp("SN Change Approval"), 5, map[string]string{crFlowPeerAID: "REQUESTED"})
		f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
		f.wantLive(id, "in Closed", nil)
	})
}

// (i) Migration 0193 cancels exactly the stale rows and nothing else, and is safe
// to run twice.
func TestChangeRequestFlowIntegration_StaleApprovals_Migration(t *testing.T) {
	f := newCustomerGroupFlow(t)
	seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
	const stamp = "migration:0193_change_request_cancel_stale_approvals"

	type cr struct {
		id    string
		state string
	}
	mk := func(state string) cr {
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.setState(id, state)
		return cr{id, state}
	}
	closed, canceled, rollback := mk("CLOSED"), mk("CANCELED"), mk("ROLLBACK")
	customerReview, authorize, review, assess, noState := mk("CUSTOMER_REVIEW"), mk("AUTHORIZE"), mk("REVIEW"), mk("ASSESS"), mk("")

	A, B, O := crFlowPeerAID, crFlowPeerBID, crFlowOutsiderID
	C1, C2 := crCABMemberUserID1, crCABMemberUserID2
	// expect: "<label>#<stage>/<user>" -> status after the migration. A stage is
	// addressed by the label it is seeded with (or "-" when it has none).
	type seeded struct {
		crID  string
		label string
		stage string
		rows  map[string]string
	}
	var all []seeded
	seed := func(c cr, label *string, age int, rows map[string]string) {
		l := "-"
		if label != nil {
			l = *label
		}
		all = append(all, seeded{c.id, l, f.seedLooseStage(c.id, label, age, rows), rows})
	}
	// (a) final changes: everything requested is cancelled, whatever the stage.
	seed(closed, sp("Review"), 50, map[string]string{A: "REQUESTED", B: "APPROVED"})
	seed(closed, nil, 40, map[string]string{O: "REQUESTED"})
	seed(canceled, sp("Peer Approval"), 50, map[string]string{A: "REQUESTED"})
	seed(canceled, sp("CAB Approval"), 40, map[string]string{C1: "REQUESTED"})
	seed(rollback, sp("Customer Review"), 50, map[string]string{crScopeUserA1: "REQUESTED"})
	// (b) labelled stages whose state the change is not in.
	seed(customerReview, sp("Peer Approval"), 60, map[string]string{O: "REJECTED", B: "CANCELLED"})
	seed(customerReview, sp("Review"), 50, map[string]string{A: "REQUESTED", B: "CANCELLED"})
	seed(customerReview, sp("Customer Review"), 40, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	seed(authorize, sp("Peer Approval"), 60, map[string]string{A: "REQUESTED"})
	seed(authorize, sp("Assess"), 55, map[string]string{B: "REQUESTED"})
	seed(authorize, sp("CAB Approval"), 50, map[string]string{C1: "REQUESTED"})
	seed(authorize, sp("Authorize"), 45, map[string]string{C2: "REQUESTED"})
	seed(authorize, sp("ECAB Approval"), 40, map[string]string{crECABMemberUserID: "REQUESTED"})
	seed(review, sp("Review"), 60, map[string]string{O: "REQUESTED"})
	seed(review, sp("Customer Approval"), 50, map[string]string{crScopeUserA1: "REQUESTED"})
	seed(review, sp("Customer Review"), 45, map[string]string{crScopeUserA2: "REQUESTED"})
	seed(assess, sp("Peer Approval"), 50, map[string]string{A: "REQUESTED", B: "REQUESTED"})
	// never touched: unlabelled / unrecognised stages of non-final changes, a NULL state.
	seed(review, nil, 40, map[string]string{A: "REQUESTED"})
	seed(review, sp("SN Change Approval"), 30, map[string]string{B: "REQUESTED"})
	seed(authorize, nil, 35, map[string]string{O: "REQUESTED"})
	seed(noState, sp("Review"), 50, map[string]string{A: "REQUESTED"})

	// What the migration must leave: every requested row is cancelled EXCEPT these.
	live := map[string]bool{}
	liveRow := func(c cr, label string, uid string) { live[c.id+"|"+label+"|"+uid] = true }
	liveRow(customerReview, "Customer Review", crScopeUserA1)
	liveRow(customerReview, "Customer Review", crScopeUserA2)
	liveRow(authorize, "CAB Approval", C1)
	liveRow(authorize, "Authorize", C2)
	liveRow(authorize, "ECAB Approval", crECABMemberUserID)
	liveRow(authorize, "-", O)
	liveRow(review, "Review", O)
	liveRow(review, "-", A)
	liveRow(review, "SN Change Approval", B)
	liveRow(assess, "Peer Approval", A)
	liveRow(assess, "Peer Approval", B)
	liveRow(noState, "Review", A)

	type rowState struct{ status, updatedBy, updatedOn string }
	read := func() map[string]rowState {
		t.Helper()
		out := map[string]rowState{}
		for _, s := range all {
			rows, err := f.scoped.Query(f.sys,
				`SELECT approver_user_id::text, state, updated_by, updated_on::text FROM approval_stage_approver WHERE stage_id = $1`, s.stage)
			if err != nil {
				t.Fatalf("read rows: %v", err)
			}
			for rows.Next() {
				var uid string
				var rs rowState
				if err := rows.Scan(&uid, &rs.status, &rs.updatedBy, &rs.updatedOn); err != nil {
					rows.Close()
					t.Fatalf("scan: %v", err)
				}
				out[s.crID+"|"+s.label+"|"+uid] = rs
			}
			rows.Close()
		}
		return out
	}
	before := read()

	sqlBytes, err := os.ReadFile("../../migrations/0193_change_request_cancel_stale_approvals.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	run := func(pass int) {
		t.Helper()
		if _, err := f.pool.Exec(f.sys, string(sqlBytes)); err != nil {
			t.Fatalf("running migration 0193 (pass %d): %v", pass, err)
		}
	}
	run(1)
	after := read()

	for key, was := range before {
		got := after[key]
		switch {
		case was.status != "REQUESTED":
			if got != was {
				t.Errorf("%s: a %q row was touched: %+v -> %+v", key, was.status, was, got)
			}
		case live[key]:
			if got != was {
				t.Errorf("%s: a live REQUESTED row was touched: %+v -> %+v", key, was, got)
			}
		default:
			if got.status != "CANCELLED" || got.updatedBy != stamp {
				t.Errorf("%s: stale REQUESTED row = %+v, want cancelled by %q", key, got, stamp)
			}
		}
	}
	// Whatever a live row stood for, it is the ONLY thing still requested.
	for key, rs := range after {
		if rs.status == "REQUESTED" && !live[key] {
			t.Errorf("%s is still requested after the migration", key)
		}
	}

	// Idempotent: a second run changes nothing (not even updated_on).
	run(2)
	again := read()
	for key, rs := range after {
		if again[key] != rs {
			t.Errorf("%s changed on the second run: %+v -> %+v", key, rs, again[key])
		}
	}
	// And the change requests themselves were never touched.
	for _, c := range []cr{closed, canceled, rollback, customerReview, authorize, review, assess, noState} {
		if got := f.state(c.id); got != c.state {
			t.Errorf("migration changed a change request's state: %q -> %q", c.state, got)
		}
	}
}

// A state written from outside the approval flow -- the GitHub sync closing a
// change when its issue closes -- leaves nothing actionable either: the sync's
// own state writer runs the same reconcile in the same transaction.
func TestChangeRequestFlowIntegration_StaleApprovals_GithubStateWriteCancels(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	f.requestApproval(id)
	f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
	f.wantLive(id, "in Review", map[string][]string{"Review": crStaleAssigned})

	gh := repository.NewGithubMutationRepository(f.scoped)
	// A write of the state it already holds changes nothing (and cancels nothing).
	if changed, err := gh.SetState(f.sys, id, "REVIEW"); err != nil || changed {
		t.Fatalf("SetState(REVIEW) on a change already in Review = %v, %v, want false, nil", changed, err)
	}
	f.wantLive(id, "after the redundant write", map[string][]string{"Review": crStaleAssigned})

	if changed, err := gh.SetState(f.sys, id, "CLOSED"); err != nil || !changed {
		t.Fatalf("SetState(CLOSED) = %v, %v, want true, nil", changed, err)
	}
	f.expect(id, "after the GitHub close", "CLOSED")
	f.wantLive(id, "after the GitHub close", nil)
	f.wantCanDecide(id, "after the GitHub close", nil)
	f.wantStatuses(id, "after the GitHub close", "Review", map[string]string{crFlowPeerAID: "CANCELLED", crFlowPeerBID: "CANCELLED", crFlowOutsiderID: "CANCELLED"})
}
