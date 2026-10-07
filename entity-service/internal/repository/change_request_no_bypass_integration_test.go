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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The compliance rule, end to end: no staff action records the customer's
// approval or review on the customer's behalf. A change in Customer Approval /
// Customer Review moves on only through the customer's own answer; staff keep
// Cancel, Re-schedule (Customer Approval) and Roll back (Customer Review).
// "Bypass customer approval" / "Bypass customer review" -- a manual
// {state: scheduled} / {state: closed}, which also stamped the customer's flag --
// no longer exist, in any situation that used to allow them.
//
// Same harness as TestChangeRequestFlowIntegration_* (crFlow, DSN-gated by
// CHANGE_REQUEST_TEST_DSN, run as a superuser and as the non-superuser csm_app).

// bypassSnapshot renders everything a refused PATCH must leave exactly as it was:
// the state, both outcome flags, every approval stage with every approver row,
// work_item.updated_on / updated_by, the planned window and the journal.
func (f *crFlow) bypassSnapshot(id string) string {
	f.t.Helper()
	var state *string
	var approved, reviewed *bool
	var updatedOn, updatedBy string
	var start, end *string
	var comments int
	if err := f.scoped.QueryRow(f.sys,
		`SELECT cr.state::text, cr.is_customer_approval_required, cr.is_customer_review_required, wi.updated_on::text, COALESCE(wi.updated_by, ''),
		        cr.start_on::text, cr.end_on::text, (SELECT COUNT(*) FROM comment WHERE work_item_id = wi.id)
		 FROM change_request cr JOIN work_item wi ON wi.id = cr.id WHERE cr.id = $1`, id).Scan(&state, &approved, &reviewed, &updatedOn, &updatedBy, &start, &end, &comments); err != nil {
		f.t.Fatalf("snapshot %s: %v", id, err)
	}
	str := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}
	flag := func(p *bool) string {
		if p == nil {
			return "<nil>"
		}
		return fmt.Sprint(*p)
	}
	parts := []string{
		fmt.Sprintf("state=%s approved=%s reviewed=%s updated_on=%s updated_by=%s start=%s end=%s comments=%d",
			str(state), flag(approved), flag(reviewed), updatedOn, updatedBy, str(start), str(end), comments),
	}
	for _, st := range f.stages(id) {
		var rows []string
		for uid, status := range st.approvers {
			rows = append(rows, uid+"="+status)
		}
		sort.Strings(rows)
		parts = append(parts, st.label+"{"+strings.Join(rows, ",")+"}")
	}
	return strings.Join(parts, " | ")
}

// wantRefusedAndUnchanged sends req as WSO2 staff and asserts it is a 400 with
// exactly the message wantMsg and that nothing at all changed.
func (f *crFlow) wantRefusedAndUnchanged(what, id string, req domain.PatchChangeRequestRequest, wantMsg string) {
	f.t.Helper()
	before := f.bypassSnapshot(id)
	_, err := f.patch(id, req)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		f.t.Fatalf("%s: err = %v (%T), want *apierror.ValidationError", what, err, err)
	}
	if ve.Msg != wantMsg {
		f.t.Fatalf("%s: message = %q, want %q", what, ve.Msg, wantMsg)
	}
	if after := f.bypassSnapshot(id); after != before {
		f.t.Fatalf("%s: a refused PATCH changed the change request:\n  before: %s\n  after:  %s", what, before, after)
	}
}

const (
	bypassApprovalMsg      = `state "scheduled" cannot be set manually from customer_approval: the customer's approval can only be given by the customer in the Customer Portal; cancel the change or re-schedule it instead`
	bypassReviewMsgOpen    = `state "closed" cannot be set manually from customer_review: the customer's review can only be given by the customer in the Customer Portal; roll the change back or cancel it instead`
	bypassReviewMsgPending = `state "closed" cannot be set manually from customer_review: the customer's review can only be given by the customer in the Customer Portal; cancel the change instead`
	bypassApprovedFlagMsg  = `isCustomerApproved cannot be set on the customer's behalf: the customer's approval can only be given by the customer in the Customer Portal`
	bypassReviewedFlagMsg  = `isCustomerReviewed cannot be set on the customer's behalf: the customer's review can only be given by the customer in the Customer Portal`
)

// bypassScenario is one way a change request comes to sit in a customer state.
type bypassScenario struct {
	name string
	// setup returns a change request in the state, in the situation named.
	setup func(f *crFlow) string
	// live: a customer stage with a REQUESTED approver exists (customer review:
	// Roll back is then the customer's to give, so staff are offered only Cancel).
	live bool
}

// A late contact for project C, which has none: not the creator, active, registered.
func (f *crFlow) lateContactOnProjectC() string {
	f.t.Helper()
	const late = "3bbbbbbb-0000-0000-0000-0000000000b7"
	f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user, user_type)
	           VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, 'Lena Late', 'Lena', 'Late', $2, true, false, 'EXTERNAL'::user_type_enum)`, late, crFlowEmail(late))
	f.registerContact(crScopeProjectC, crScopeAccountID, late)
	return late
}

// settleCustomerStage settles the live customer stage without moving the change:
// the contact's row Approved / Rejected, the rest Cancelled -- a stage "already
// decided" while the change still sits in its state (what a ServiceNow-synced or
// half-written row can look like).
func (f *crFlow) settleCustomerStage(id, label, decidedRow string) {
	f.t.Helper()
	f.execSQL(`UPDATE approval_stage_approver SET state = 'CANCELLED'
	           WHERE stage_id IN (SELECT id FROM approval_stage WHERE work_item_id = $1 AND checkpoint_label = $2)`, id, label)
	f.execSQL(`UPDATE approval_stage_approver SET state = $3
	           WHERE approver_user_id = $4::uuid AND stage_id IN (SELECT id FROM approval_stage WHERE work_item_id = $1 AND checkpoint_label = $2)`, id, label, decidedRow, crScopeUserA1)
}

func customerApprovalScenarios() []bypassScenario {
	return []bypassScenario{
		{"no project at all (a legacy change)", func(f *crFlow) string {
			id := f.createWithProject(domain.ChangeRequestTypeStandard, nil, true, false)
			f.setState(id, "CUSTOMER_APPROVAL")
			return id
		}, false},
		// Request Approval refuses a ticked box on a project nobody can be asked on, so
		// these two reach the gate by the residual edge: the contact the project had
		// when approval was requested is gone before the gate (requestApprovalThenContactsLeave).
		{"a project with no registered contacts", func(f *crFlow) string {
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
			f.requestApprovalThenContactsLeave(id)
			f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
			return id
		}, false},
		{"a project whose only contact is the creator", func(f *crFlow) string {
			f.registerContact(crScopeProjectC, crScopeAccountID, crFlowCreatorID)
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
			f.requestApprovalThenContactsLeave(id)
			f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
			return id
		}, false},
		{"a legacy change with contacts but no stage (nobody asked)", func(f *crFlow) string {
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
			f.setState(id, "CUSTOMER_APPROVAL")
			return id
		}, false},
		{"the customer group is asked (a live stage)", func(f *crFlow) string {
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
			f.driveToCustomerApproval(id)
			return id
		}, true},
		{"the stage was already decided", func(f *crFlow) string {
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
			f.driveToCustomerApproval(id)
			f.settleCustomerStage(id, stageCustApproval, "APPROVED")
			return id
		}, false},
		{"the request was withdrawn (every row cancelled)", func(f *crFlow) string {
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
			f.driveToCustomerApproval(id)
			f.execSQL(`UPDATE approval_stage_approver SET state = 'CANCELLED' WHERE work_item_id = $1 AND state = 'REQUESTED'`, id)
			return id
		}, false},
	}
}

func customerReviewScenarios() []bypassScenario {
	toReview := func(f *crFlow, project *string) string {
		id := f.createWithProject(domain.ChangeRequestTypeNormal, project, false, true)
		// A project nobody can be asked on is refused at Request Approval, so those
		// scenarios (C) lose their contact after it (the residual edge); the others
		// have contacts throughout.
		if project != nil && *project == crScopeProjectC {
			f.requestApprovalThenContactsLeave(id)
		} else {
			f.requestApproval(id)
		}
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
		return id
	}
	return []bypassScenario{
		{"no project at all (a legacy change)", func(f *crFlow) string {
			id := f.createWithProject(domain.ChangeRequestTypeStandard, nil, false, true)
			f.setState(id, "CUSTOMER_REVIEW")
			return id
		}, false},
		{"a project with no registered contacts", func(f *crFlow) string {
			id := toReview(f, sp(crScopeProjectC))
			f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "rollback", "canceled")
			return id
		}, false},
		{"a project whose only contact is the creator", func(f *crFlow) string {
			f.registerContact(crScopeProjectC, crScopeAccountID, crFlowCreatorID)
			id := toReview(f, sp(crScopeProjectC))
			f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "rollback", "canceled")
			return id
		}, false},
		{"a legacy change with contacts but no stage (nobody asked)", func(f *crFlow) string {
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
			f.setState(id, "CUSTOMER_REVIEW")
			return id
		}, false},
		{"the customer group is asked (a live stage)", func(f *crFlow) string {
			id := toReview(f, sp(crScopeProjectA))
			f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
			return id
		}, true},
		{"the stage was already decided", func(f *crFlow) string {
			id := toReview(f, sp(crScopeProjectA))
			f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
			f.settleCustomerStage(id, stageCustReview, "APPROVED")
			return id
		}, false},
	}
}

// A manual {state: scheduled} out of Customer Approval is refused in EVERY
// situation that used to allow it (no project, no contacts, the only contact is the
// creator, a legacy change with no stage, a live stage, a stage already decided),
// with and without isCustomerApproved in the body, and every refusal changes
// nothing: the state, the flags, the approver rows, updated_on.
func TestChangeRequestNoBypassIntegration_ManualScheduledOutOfCustomerApprovalIsRefused(t *testing.T) {
	for _, sc := range customerApprovalScenarios() {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := sc.setup(f)
			f.expect(id, "before the attempts", "CUSTOMER_APPROVAL", "authorize", "canceled")
			if approved, _ := f.customerOutcome(id); approved {
				t.Fatal("the flag is already stamped before any attempt")
			}

			sched := domain.ChangeRequestStateScheduled
			f.wantRefusedAndUnchanged("manual scheduled", id, domain.PatchChangeRequestRequest{State: &sched}, bypassApprovalMsg)
			f.wantRefusedAndUnchanged("manual SCHEDULED, upper case", id, domain.PatchChangeRequestRequest{State: ptrCRState("SCHEDULED")}, bypassApprovalMsg)
			// The flag is refused first, as a customer's answer in the request of a caller who is not the customer.
			for _, v := range []bool{true, false} {
				f.wantRefusedAndUnchanged(fmt.Sprintf("manual scheduled + isCustomerApproved=%v", v), id,
					domain.PatchChangeRequestRequest{State: &sched, IsCustomerApproved: boolp(v)}, bypassApprovedFlagMsg)
				f.wantRefusedAndUnchanged(fmt.Sprintf("isCustomerApproved=%v alone", v), id,
					domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(v)}, bypassApprovedFlagMsg)
			}
			f.wantRefusedAndUnchanged("isCustomerReviewed alone", id, domain.PatchChangeRequestRequest{IsCustomerReviewed: boolp(true)}, bypassReviewedFlagMsg)
			// Together with the on-hold release (the old "take it off hold and approve in one call").
			f.wantRefusedAndUnchanged("manual scheduled + onHold=false", id, domain.PatchChangeRequestRequest{State: &sched, OnHold: boolp(false)}, bypassApprovalMsg)

			// Every other way out of Customer Approval that skips the customer is
			// refused with the same message (the PATCH has no full transition graph).
			for _, to := range []domain.ChangeRequestState{
				domain.ChangeRequestStateImplement, domain.ChangeRequestStateReview, domain.ChangeRequestStateClosed,
			} {
				to := to
				want := strings.Replace(bypassApprovalMsg, `"scheduled"`, fmt.Sprintf("%q", string(to)), 1)
				f.wantRefusedAndUnchanged("manual "+string(to), id, domain.PatchChangeRequestRequest{State: &to}, want)
			}

			f.expect(id, "after the refused attempts", "CUSTOMER_APPROVAL", "authorize", "canceled")
			if approved, reviewed := f.customerOutcome(id); approved || reviewed {
				t.Fatalf("flags after the refused attempts = %v/%v, want false/false", approved, reviewed)
			}
			// What staff have: cancel the change.
			f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
			if approved, _ := f.customerOutcome(id); approved {
				t.Fatal("cancelling stamped the customer's approval")
			}
		})
	}
}

// Same for the review: a manual {state: closed} out of Customer Review is refused
// everywhere it used to work, with and without isCustomerReviewed in the body; the
// advice names Roll back only where staff may still roll the change back (not
// while the customer is being asked), and nothing changes.
func TestChangeRequestNoBypassIntegration_ManualClosedOutOfCustomerReviewIsRefused(t *testing.T) {
	for _, sc := range customerReviewScenarios() {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := sc.setup(f)
			wantLegal := []string{"rollback", "canceled"}
			wantMsg := bypassReviewMsgOpen
			if sc.live {
				wantLegal = []string{"canceled"}
				wantMsg = bypassReviewMsgPending
			}
			f.expect(id, "before the attempts", "CUSTOMER_REVIEW", wantLegal...)
			if _, reviewed := f.customerOutcome(id); reviewed {
				t.Fatal("the flag is already stamped before any attempt")
			}

			closed := domain.ChangeRequestStateClosed
			f.wantRefusedAndUnchanged("manual closed", id, domain.PatchChangeRequestRequest{State: &closed}, wantMsg)
			f.wantRefusedAndUnchanged("manual CLOSED, upper case", id, domain.PatchChangeRequestRequest{State: ptrCRState("CLOSED")}, wantMsg)
			for _, v := range []bool{true, false} {
				f.wantRefusedAndUnchanged(fmt.Sprintf("manual closed + isCustomerReviewed=%v", v), id,
					domain.PatchChangeRequestRequest{State: &closed, IsCustomerReviewed: boolp(v)}, bypassReviewedFlagMsg)
				f.wantRefusedAndUnchanged(fmt.Sprintf("isCustomerReviewed=%v alone", v), id,
					domain.PatchChangeRequestRequest{IsCustomerReviewed: boolp(v)}, bypassReviewedFlagMsg)
			}
			f.wantRefusedAndUnchanged("isCustomerApproved alone", id, domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(true)}, bypassApprovedFlagMsg)
			f.wantRefusedAndUnchanged("manual closed + onHold=false", id, domain.PatchChangeRequestRequest{State: &closed, OnHold: boolp(false)}, wantMsg)

			for _, to := range []domain.ChangeRequestState{
				domain.ChangeRequestStateScheduled, domain.ChangeRequestStateImplement, domain.ChangeRequestStateReview,
			} {
				to := to
				want := strings.Replace(wantMsg, `"closed"`, fmt.Sprintf("%q", string(to)), 1)
				if to == domain.ChangeRequestStateScheduled {
					// The scheduled case judges the state itself first, with its own text out of
					// Customer Review (it is not a state the review leads to).
					want = `state "scheduled" cannot be set manually: it is reached automatically through the approval flow (Request Approval, then peer/CAB approval), or by the customer's own approval from customer_approval`
				}
				f.wantRefusedAndUnchanged("manual "+string(to), id, domain.PatchChangeRequestRequest{State: &to}, want)
			}

			f.expect(id, "after the refused attempts", "CUSTOMER_REVIEW", wantLegal...)
			if approved, reviewed := f.customerOutcome(id); approved || reviewed {
				t.Fatalf("flags after the refused attempts = %v/%v, want false/false", approved, reviewed)
			}
			if sc.live {
				// A failed review is the customer's to give while they are asked: refused.
				_, err := f.patchState(id, domain.ChangeRequestStateRollback)
				f.wantValidationError("manual rollback while the customer is asked", err, "approving or rejecting it in the change request's approvals")
				f.expect(id, "after the refused rollback", "CUSTOMER_REVIEW", "canceled")
				f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
			} else {
				// Nobody is asked: staff may still roll the change back (unchanged), never close it.
				f.step(id, domain.ChangeRequestStateRollback, "ROLLBACK")
			}
			if _, reviewed := f.customerOutcome(id); reviewed {
				t.Fatal("the way out of Customer Review stamped the customer's review")
			}
		})
	}
}

// The customer's own answer, Cancel, Re-schedule and Roll back still work out of
// the very states the bypass is gone from, after any number of refused staff
// attempts, and the customer's answer is what stamps the flag.
func TestChangeRequestNoBypassIntegration_TheCustomersOwnWaysStillWork(t *testing.T) {
	t.Run("approve", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
		f.driveToCustomerApproval(id)
		sched := domain.ChangeRequestStateScheduled
		f.wantRefusedAndUnchanged("staff scheduling", id, domain.PatchChangeRequestRequest{State: &sched}, bypassApprovalMsg)
		if _, err := f.approveAs(id, crScopeUserA2, true); err != nil {
			t.Fatalf("the customer's approval: %v", err)
		}
		f.expect(id, "after the customer approved", "SCHEDULED", "implement", "canceled")
		if approved, _ := f.customerOutcome(id); !approved {
			t.Fatal("the customer's approval did not stamp the flag")
		}
		assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "APPROVED"})
	})
	t.Run("reject cancels", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.driveToCustomerApproval(id)
		if _, err := f.approveAs(id, crScopeUserA1, false); err != nil {
			t.Fatalf("the customer's rejection: %v", err)
		}
		f.expect(id, "after the customer rejected", "CANCELED")
		if approved, _ := f.customerOutcome(id); approved {
			t.Fatal("a rejection stamped the approval")
		}
	})
	t.Run("review closes", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.driveToCustomerReview(id)
		closed := domain.ChangeRequestStateClosed
		f.wantRefusedAndUnchanged("staff closing", id, domain.PatchChangeRequestRequest{State: &closed}, bypassReviewMsgPending)
		if _, err := f.reviewAs(id, crScopeUserA2, true); err != nil {
			t.Fatalf("the customer's review: %v", err)
		}
		f.expect(id, "after the customer's review", "CLOSED")
		if _, reviewed := f.customerOutcome(id); !reviewed {
			t.Fatal("the customer's review did not stamp the flag")
		}
	})
	t.Run("a failed review rolls back", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.driveToCustomerReview(id)
		if _, err := f.reviewAs(id, crScopeUserA1, false); err != nil {
			t.Fatalf("the customer's failed review: %v", err)
		}
		f.expect(id, "after the customer failed the review", "ROLLBACK")
		if _, reviewed := f.customerOutcome(id); reviewed {
			t.Fatal("a failed review stamped the flag")
		}
	})
	t.Run("Re-schedule from Customer Approval asks the customer again", func(t *testing.T) {
		for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard} {
			typ := typ
			t.Run(string(typ), func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				id := f.createWithProject(typ, sp(crScopeProjectA), true, false)
				f.setPlanned(id, rsStart1, rsEnd1)
				if typ == domain.ChangeRequestTypeStandard {
					f.requestApproval(id)
					f.expect(id, "after Request Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
				} else {
					f.driveToCustomerApproval(id)
				}
				if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
					t.Fatalf("Re-schedule: %v", err)
				}
				f.wantPlanned(id, "after Re-schedule", rsStart2, rsEnd2)
				if typ == domain.ChangeRequestTypeStandard {
					f.expect(id, "after Re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
					if n := f.liveStageRows(id, stageCustApproval); n != 2 {
						t.Fatalf("customer request has %d live rows after the Re-schedule, want 2 (asked again)", n)
					}
				} else {
					f.expect(id, "after Re-schedule", "AUTHORIZE", "canceled")
				}
				if approved, _ := f.customerOutcome(id); approved {
					t.Fatal("a Re-schedule stamped the customer's approval")
				}
			})
		}
	})
	t.Run("Roll back from Review and from Customer Review (nobody asked) is unchanged", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), false, false)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
		f.step(id, domain.ChangeRequestStateRollback, "ROLLBACK")

		id2 := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), false, true)
		f.requestApprovalThenContactsLeave(id2) // refused with nobody to ask; the contact leaves after (residual edge)
		f.approvePeerAndCAB(id2, "SCHEDULED", "implement", "canceled")
		f.step(id2, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id2, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
		f.step(id2, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "rollback", "canceled")
		f.step(id2, domain.ChangeRequestStateRollback, "ROLLBACK")
	})
	t.Run("Cancel from both customer states", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		ca := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.driveToCustomerApproval(ca)
		f.step(ca, domain.ChangeRequestStateCanceled, "CANCELED")
		cr := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.requestApproval(cr)
		f.approvePeerAndCAB(cr, "SCHEDULED", "implement", "canceled")
		f.driveToCustomerReview(cr)
		f.step(cr, domain.ChangeRequestStateCanceled, "CANCELED")
	})
}

// A change nobody could be asked is not stranded for good: registering a contact
// and restating the project asks them (the stage is provisioned), and THEIR answer
// -- never a staff action -- is what moves the change on.
func TestChangeRequestNoBypassIntegration_ANobodyToAskChangeIsAskedOnceAContactRegisters(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
	f.requestApprovalThenContactsLeave(id) // Request Approval refuses it with nobody to ask; the contact leaves after (residual edge)
	f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantRefusedAndUnchanged("manual scheduled with nobody to ask", id, domain.PatchChangeRequestRequest{State: ptrCRState("scheduled")}, bypassApprovalMsg)

	late := f.lateContactOnProjectC()
	f.setProject(id, crScopeProjectC)
	st := f.customerStages(id)
	if len(st) != 1 || liveStages(st) != 1 {
		t.Fatalf("customer stages after a contact registered and the project was restated = %+v, want one live", st)
	}
	assertApprovers(t, "the newly asked contact", st[0].approvers, map[string]string{late: "REQUESTED"})
	f.wantRefusedAndUnchanged("manual scheduled with the contact asked", id, domain.PatchChangeRequestRequest{State: ptrCRState("scheduled")}, bypassApprovalMsg)
	if _, err := f.approveAs(id, late, true); err != nil {
		t.Fatalf("the late contact's own answer: %v", err)
	}
	f.expect(id, "after the contact's answer", "SCHEDULED", "implement", "canceled")
	if approved, _ := f.customerOutcome(id); !approved {
		t.Fatal("the contact's answer did not stamp the flag")
	}
}

// The staff refusal is about WHO is sending, not what is stored: WSO2 staff who
// also hold an external record (a customer contact) are "internal" on a PATCH
// (their answer is the decision route's, on their own approver row), and an
// internal client credential is no customer either. Neither can send the flags.
func TestChangeRequestNoBypassIntegration_NoStaffIdentityCanSendTheFlags(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	f.driveToCustomerApproval(id)
	staffWhoIsAlsoAContact := repository.WithCallerIdentity(context.Background(), repository.SearchScope{
		ViewerEmail: crFlowEmail(crScopeUserA1), HasInternalAccess: true})
	m2m := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true})
	for who, ctx := range map[string]context.Context{
		"WSO2 staff who is also a registered contact": staffWhoIsAlsoAContact,
		"an internal client credential":               m2m,
		"the system identity":                         f.sys,
	} {
		before := f.bypassSnapshot(id)
		for _, req := range []domain.PatchChangeRequestRequest{
			{IsCustomerApproved: boolp(true)},
			{IsCustomerApproved: boolp(false)},
			{IsCustomerReviewed: boolp(true)},
			{IsCustomerApproved: boolp(true), State: ptrCRState("scheduled")},
		} {
			_, err := f.repo.PatchChangeRequest(ctx, id, req, crFlowEmail(crScopeUserA1))
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "on the customer's behalf") {
				t.Fatalf("%s sending %+v: err = %v (%T), want the on-the-customer's-behalf refusal", who, req, err, err)
			}
		}
		if after := f.bypassSnapshot(id); after != before {
			t.Fatalf("%s: refused PATCHes changed the change request:\n  before: %s\n  after:  %s", who, before, after)
		}
	}
	// The decision route answers only on the caller's OWN approver row: WSO2 staff
	// who are no member of the customer group cannot decide the customer's stage,
	// live or not.
	f.wantForbidden("WSO2 staff deciding the customer's approval", f.decide(id, crFlowPeerAID, "approved"), "only members of the customer group")
	f.wantForbidden("WSO2 staff rejecting the customer's approval", f.decide(id, crFlowPeerAID, "rejected"), "only members of the customer group")
	f.expect(id, "after the refused staff decisions", "CUSTOMER_APPROVAL", "authorize", "canceled")
}

// Every state, every type, every box combination, and the exact legalNextStates:
// the table the webapp renders. The customer's approval / review are never on it
// ("scheduled" from any state; "closed" from Customer Review).
func TestChangeRequestNoBypassIntegration_LegalNextStatesExactTable(t *testing.T) {
	f := newCustomerGroupFlow(t)
	// state -> exact list, with review ticked / unticked where it matters.
	want := func(state string, review bool) []string {
		switch state {
		case "NEW":
			return []string{"assess", "canceled"}
		case "ASSESS":
			return []string{"canceled"} // waits for the peer approval, which moves it on by itself
		case "AUTHORIZE":
			return []string{"canceled"}
		case "CUSTOMER_APPROVAL":
			return []string{"authorize", "canceled"} // authorize = Re-schedule; never scheduled
		case "SCHEDULED":
			return []string{"implement", "canceled"}
		case "IMPLEMENT":
			return []string{"review", "canceled"}
		case "REVIEW":
			if review {
				return []string{"customer_review", "rollback", "canceled"}
			}
			return []string{"closed", "rollback", "canceled"}
		case "CUSTOMER_REVIEW":
			return []string{"rollback", "canceled"} // never closed
		}
		return nil // ROLLBACK, CLOSED, CANCELED: terminal
	}
	states := []string{"NEW", "ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "ROLLBACK", "CLOSED", "CANCELED"}
	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeStandard, domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeEmergency} {
		for _, approval := range []bool{false, true} {
			for _, review := range []bool{false, true} {
				id := f.createWithProject(typ, sp(crScopeProjectC), approval, review)
				for _, st := range states {
					f.setState(id, st)
					got := f.legal(id)
					w := want(st, review)
					if strings.Join(got, ",") != strings.Join(w, ",") || (got == nil) != (w == nil) {
						t.Errorf("%s approval=%v review=%v in %s: legalNextStates = %v, want %v", typ, approval, review, st, got, w)
					}
				}
			}
		}
	}
	// A live customer stage: the customer's request is out, so Roll back (a
	// failed review) is theirs; Re-schedule and Cancel stay.
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	f.setState(id, "CUSTOMER_APPROVAL")
	f.setProject(id, crScopeProjectA) // restating the project asks the contacts
	assertStates(t, "customer_approval with a live stage", f.legal(id), "authorize", "canceled")
	f.setState(id, "CUSTOMER_REVIEW")
	f.setProject(id, crScopeProjectA)
	assertStates(t, "customer_review with a live stage", f.legal(id), "canceled")
	// The same change once nobody is being asked.
	f.execSQL(`UPDATE approval_stage_approver SET state = 'CANCELLED' WHERE work_item_id = $1 AND state = 'REQUESTED'`, id)
	assertStates(t, "customer_review with the request withdrawn", f.legal(id), "rollback", "canceled")
}

// No other entry point can write the customer's outcome either. Creating a change
// request (both create paths) ignores any state and carries no flag; the GitHub
// sync's state writer refuses to move a change out of a customer state.
func TestChangeRequestNoBypassIntegration_OtherDoorsAreClosedToo(t *testing.T) {
	t.Run("create ignores a requested state and stamps nothing", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		typ := domain.ChangeRequestTypeNormal
		for i, st := range []domain.ChangeRequestState{domain.ChangeRequestStateCustomerApproval, domain.ChangeRequestStateScheduled, domain.ChangeRequestStateClosed} {
			st := st
			g := crFlowGroupID
			resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ, GroupID: &g, State: &st}, crFlowEmail(crFlowCreatorID))
			if err != nil {
				t.Fatalf("CreateChangeRequest(state=%s): %v", st, err)
			}
			cr := f.get(resp.ChangeRequest.ID)
			if cr.State == nil || *cr.State != "new" || cr.HasCustomerApproved || cr.HasCustomerReviewed {
				t.Fatalf("a change created with state %s = state %v approved %v reviewed %v, want new / false / false", st, cr.State, cr.HasCustomerApproved, cr.HasCustomerReviewed)
			}
			sn, err := f.repo.CreateChangeRequestFromServiceNow(f.sys, domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ, State: &st},
				fmt.Sprintf("3aaaaaaa-0000-0000-0000-0000000001%02d", i), fmt.Sprintf("CRNOBYP%03d", i), crFlowEmail(crFlowCreatorID))
			if err != nil {
				t.Fatalf("CreateChangeRequestFromServiceNow(state=%s): %v", st, err)
			}
			cr = f.get(sn.ChangeRequest.ID)
			if cr.State == nil || *cr.State != "new" || cr.HasCustomerApproved || cr.HasCustomerReviewed {
				t.Fatalf("a ServiceNow-first change created with state %s = state %v approved %v reviewed %v, want new / false / false", st, cr.State, cr.HasCustomerApproved, cr.HasCustomerReviewed)
			}
		}
	})
	t.Run("the GitHub sync cannot move a change out of a customer state", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		gh := repository.NewGithubMutationRepository(f.scoped)
		for _, tc := range []struct{ state, target string }{
			{"CUSTOMER_APPROVAL", "SCHEDULED"}, {"CUSTOMER_APPROVAL", "IMPLEMENT"}, {"CUSTOMER_APPROVAL", "REVIEW"},
			{"CUSTOMER_REVIEW", "CLOSED"}, {"CUSTOMER_REVIEW", "REVIEW"},
		} {
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
			f.setState(id, tc.state)
			before := f.bypassSnapshot(id)
			changed, err := gh.SetState(f.sys, id, tc.target)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || changed || !strings.Contains(ve.Msg, "only the customer's own answer") {
				t.Fatalf("SetState(%s -> %s) = %v, %v, want false and a refusal naming the customer's answer", tc.state, tc.target, changed, err)
			}
			if after := f.bypassSnapshot(id); after != before {
				t.Fatalf("SetState(%s -> %s) changed the change request:\n  before: %s\n  after:  %s", tc.state, tc.target, before, after)
			}
			// A write of the state it already holds is the no-op it always was.
			if changed, err := gh.SetState(f.sys, id, tc.state); err != nil || changed {
				t.Fatalf("SetState(%s) on a change already there = %v, %v, want false, nil", tc.state, changed, err)
			}
		}
		// Everywhere else the sync works as before.
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.setState(id, "SCHEDULED")
		if changed, err := gh.SetState(f.sys, id, "IMPLEMENT"); err != nil || !changed {
			t.Fatalf("SetState(SCHEDULED -> IMPLEMENT) = %v, %v, want true, nil", changed, err)
		}
	})
}
