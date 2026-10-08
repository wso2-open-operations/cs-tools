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
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// WSO2 answers a time as a customer's PROPOSAL only when a registered contact of the project is
// recorded as having proposed it. The predicate that says a time waits does not say who wrote it
// (the previous system lets WSO2 users write the same column, and an old date looks the same), so
// without a recorded proposer Accept would schedule the change for a time no customer ever
// consented to -- a staff action standing in for the customer's answer. The proposer is the LAST
// WRITER of the change (work_item.updated_by) read BEFORE the answer's own write replaces it, and
// nothing else: no comment, audit, outbox or other row names a proposer, and nothing is added to
// record one. A genuine proposal that another write has overwritten as last writer reads "not
// recorded" like any other time nobody proposed; Propose a different time is the way on.

// msgAcceptNobodyRecordedFull is the whole refusal: pinned word for word because the pages and
// the contract show it.
const msgAcceptNobodyRecordedFull = `nobody is recorded as having proposed this time (it may have been written by someone at WSO2 or left over from an earlier cycle), so it cannot be accepted: use "Propose a different time" to ask the customer to approve a time`

// leftoverInCustomerApproval is a change in Customer Approval that carries a date nobody is
// recorded as having proposed: a WSO2 user's, or one left over from an earlier cycle. It waits by
// the predicate (a date that differs from the plan, no answer, nothing else asked).
func leftoverInCustomerApproval(t *testing.T) (*crFlow, string) {
	t.Helper()
	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.syncWritesConversation(id, sp(rsStart2), "")
	p := f.proposalOf(id)
	if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded {
		t.Fatalf("a date nobody proposed = %+v, want it pending with no proposer recorded", p)
	}
	return f, id
}

func TestChangeRequestProposalIntegration_AcceptNeedsARecordedProposer(t *testing.T) {
	t.Run("a date nobody is recorded as having proposed is refused, in these words, with its own code, and nothing is written", func(t *testing.T) {
		f, id := leftoverInCustomerApproval(t)
		p := f.proposalOf(id)
		if p.CanAccept == nil || *p.CanAccept || p.AcceptBlockedReason == nil || *p.AcceptBlockedReason != msgAcceptNobodyRecordedFull {
			t.Fatalf("customerProposal = %+v, want Accept blocked with %q", p, msgAcceptNobodyRecordedFull)
		}
		before, rows := f.snap(id), f.rowCounts()
		_, err := f.accept(id)
		f.wantConflictExact("Accept of a date nobody proposed", err, msgAcceptNobodyRecordedFull)
		wantRefusalCode(t, "Accept of a date nobody proposed", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
		f.wantRefusedSame("Accept of a date nobody proposed", id, before, err)
		wantDelta(t, "a refused Accept", rows, f.rowCounts(), map[string]int{})
		// Nobody else may accept it either: the refusal is about the data, not the caller.
		_, err = f.staffPatch(id, crFlowPeerAID, f.acceptReq(id))
		wantRefusalCode(t, "another staff member accepting a date nobody proposed", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
	})

	t.Run("the same date once a registered contact has proposed it is accepted", func(t *testing.T) {
		f, id := leftoverInCustomerApproval(t)
		// A customer proposes the very time that is stored? Refused: it is not theirs to write again.
		_, err := f.proposeAs(id, crScopeUserA1, rsStart2)
		f.wantExact("a customer proposing the time that is stored", err, msgProposalStoredNotProposed)
		// Another time is theirs: the proposal is recorded and acceptable.
		f.mustPropose(id, crScopeUserA1, rsStart3)
		f.mustAccept(id)
		f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
		f.wantPlanned(id, "after Accept", rsStart3, endFor(rsStart3))
		f.wantConversation(id, "after Accept", rsStart3, "AGREE")
		// The staff member who accepted is the last writer now (the answer's own write), which does
		// not matter: nothing waits any more.
		if got := f.updatedBy(id); got != crFlowEmail(crFlowCreatorID) {
			t.Fatalf("work_item.updated_by after Accept = %q, want the member who accepted", got)
		}
	})

	t.Run("the proposer is read before the answer's own write replaces the last writer", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		if got := f.updatedBy(id); got != crFlowEmail(crScopeUserA1) {
			t.Fatalf("work_item.updated_by after the proposal = %q, want the proposing contact", got)
		}
		// Accept stamps the accepting staff member on the work item as its very first statement: a proposer
		// read after it would be a WSO2 user, and no proposal could ever be accepted.
		if _, err := f.staffPatch(id, crFlowPeerAID, f.acceptReq(id)); err != nil {
			t.Fatalf("Accept by a staff member who is not the last writer: %v", err)
		}
		// A different time too: it answers (DISAGREE) only a proposal somebody is recorded as having made.
		g := newCustomerGroupFlow(t)
		gid := g.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		g.mustPropose(gid, crScopeUserA2, rsStart2)
		if _, err := g.staffPatch(gid, crFlowPeerAID, g.counterReq(gid, sp(rsStart3), sp(rsEnd3))); err != nil {
			t.Fatalf("Propose a different time by a staff member who is not the last writer: %v", err)
		}
		g.wantConversation(gid, "after the different time", rsStart2, "DISAGREE")
	})

	t.Run("a proposer who is no longer a registered contact is not recorded", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED'::project_contact_state_enum
		           WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, crScopeProjectA, crFlowEmail(crScopeUserA1))
		before := f.snap(id)
		_, err := f.accept(id)
		wantRefusalCode(t, "Accept of a time proposed by a contact who has left", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
		f.wantRefusedSame("Accept of a time proposed by a contact who has left", id, before, err)
	})

	t.Run("a proposer who is a contact of ANOTHER project is not recorded", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.syncWritesConversationAs(id, crFlowEmail(crScopeUserB1), sp(rsStart2), "")
		p := f.proposalOf(id)
		if p == nil || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept {
			t.Fatalf("customerProposal for a time written by a contact of another project = %+v", p)
		}
	})
}

// A stored time that nobody is recorded as having proposed is not a proposal to answer. A plain
// Re-schedule over it is a plain Re-schedule: it must not write DISAGREE against a date nobody
// proposed (that would tell the customers WSO2 turned down a time they never asked for), and it
// must not be refused as if it had missed a live proposal.
func TestChangeRequestProposalIntegration_AStoredTimeNobodyProposedIsNoProposalToAnswer(t *testing.T) {
	t.Run("a plain Re-schedule is a plain Re-schedule: the window moves, the customers are asked again, no answer is written", func(t *testing.T) {
		f, id := leftoverInCustomerApproval(t)
		stages := f.stageLabels(id)
		if err := f.reschedule(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("a Re-schedule over a date nobody proposed: %v", err)
		}
		f.expect(id, "after the Re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.wantPlanned(id, "after the Re-schedule", rsStart3, rsEnd3)
		// The date stays as it was and so does the (absent) answer: DISAGREE is the answer to a proposal.
		f.wantConversation(id, "after the Re-schedule", rsStart2, "")
		if got := f.stageLabels(id); got != stages+",Customer Approval" {
			t.Fatalf("stages after the Re-schedule = %s, want %s and one fresh customer request", got, stages)
		}
		custom := f.customerStages(id)
		assertApprovers(t, "the customers asked again", custom[len(custom)-1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	})

	t.Run("the same as the page sends it: naming the stored time it showed", func(t *testing.T) {
		f, id := leftoverInCustomerApproval(t)
		if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("Propose a different time over a date nobody proposed: %v", err)
		}
		f.wantPlanned(id, "after the different time", rsStart3, rsEnd3)
		f.wantConversation(id, "after the different time", rsStart2, "")
	})

	t.Run("naming a time that is not the stored one is a 409 that says to read it again", func(t *testing.T) {
		f, id := leftoverInCustomerApproval(t)
		req := f.counterReq(id, sp(rsStart3), sp(rsEnd3))
		req.ExpectedCustomerUpdatedOn = sp(rsStartEarly)
		before := f.snap(id)
		_, err := f.patch(id, req)
		f.wantConflictExact("a Re-schedule naming another time", err,
			"the time stored on this change request changed after you opened it (it is now "+rsStart2+"); read it again before responding")
		f.wantRefusedSame("a Re-schedule naming another time", id, before, err)
	})

	t.Run("a request with no window is refused: there is no proposal to decline", func(t *testing.T) {
		f, id := leftoverInCustomerApproval(t)
		before := f.snap(id)
		for name, req := range map[string]domain.PatchChangeRequestRequest{
			"as the page sends a decline":    f.counterReq(id, nil, nil),
			"with nothing else":              {State: stateptr(domain.ChangeRequestStateAuthorize)},
			"with only the window it saw":    {State: stateptr(domain.ChangeRequestStateAuthorize), ExpectedPlannedStartOn: f.get(id).PlannedStartOn, ExpectedPlannedEndOn: f.get(id).PlannedEndOn},
			"naming the stored time as well": {State: stateptr(domain.ChangeRequestStateAuthorize), ExpectedCustomerUpdatedOn: sp(rsStart2)},
		} {
			_, err := f.patch(id, req)
			f.wantExact("authorize with no window, "+name, err, msgNoRecordedProposalToDecline)
			f.wantRefusedSame("authorize with no window, "+name, id, before, err)
		}
		// Re-stating the planned window is no change of it either: the Re-schedule's own refusal, and no answer.
		_, err := f.patch(id, f.counterReq(id, sp(rsStart1), sp(rsEnd1)))
		f.wantValidationError("the planned window restated", err, "re-scheduling requires a changed planned start or end")
		f.wantRefusedSame("the planned window restated", id, before, err)
	})

	t.Run("on a change with a parent record the same: the comment the existing trigger wrote there is no reference", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.giveParent(id, crScopeProjectA, false)
		f.syncWritesConversationAs(id, "wso2.engineer@example.com", sp(rsStart2), "")
		if err := f.reschedule(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("a Re-schedule over a date a WSO2 user wrote: %v", err)
		}
		f.wantConversation(id, "after the Re-schedule", rsStart2, "")
	})

	t.Run("a customer's proposal still waits: the same request is an answer to it", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		// Without naming the proposal it is a 409 (a client that never saw it never answers it) ...
		before := f.snap(id)
		err := f.reschedule(id, sp(rsStart3), sp(rsEnd3))
		f.wantConflictExact("a Re-schedule that never saw a customer's proposal", err, msgCounterProposalMissedFor(rsStart2))
		f.wantRefusedSame("a Re-schedule that never saw a customer's proposal", id, before, err)
		// ... and a decline with no window is a decline: DISAGREE and nothing else.
		if err := f.counter(id, nil, nil); err != nil {
			t.Fatalf("a decline of a customer's proposal: %v", err)
		}
		f.wantConversation(id, "after the decline", rsStart2, "DISAGREE")
	})
}

func msgCounterProposalMissedFor(proposed string) string {
	return "the customer proposed a new time (" + proposed + ") after you opened this change request; read it again to accept it or propose a different time"
}

// wantNoProposalToAnswer is what every staff act does with a stored time that nobody is recorded as
// having proposed (storedOn): Accept is refused with its own code, a request with no window is
// refused (there is no proposal to decline), and a Re-schedule with a window is a plain Re-schedule
// that asks the customers again and writes no answer against the stored time.
func (f *crFlow) wantNoProposalToAnswer(id, storedOn string) {
	f.t.Helper()
	before := f.snap(id)
	_, err := f.accept(id)
	wantRefusalCode(f.t, "Accept of a time nobody proposed", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
	f.wantRefusedSame("Accept of a time nobody proposed", id, before, err)
	_, err = f.patch(id, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateAuthorize)})
	f.wantExact("authorize with no window over a time nobody proposed", err, msgNoRecordedProposalToDecline)
	f.wantRefusedSame("authorize with no window over a time nobody proposed", id, before, err)
	stages := f.stageLabels(id)
	if err := f.reschedule(id, sp(rsStart3), sp(rsEnd3)); err != nil {
		f.t.Fatalf("a Re-schedule over a time nobody proposed: %v", err)
	}
	f.wantPlanned(id, "after the Re-schedule", rsStart3, rsEnd3)
	f.wantConversation(id, "after the Re-schedule", storedOn, "")
	if got := f.stageLabels(id); got != stages+",Customer Approval" {
		f.t.Fatalf("stages after the Re-schedule = %s, want %s and one fresh customer request", got, stages)
	}
}

// A genuine proposal that another write has overwritten as the change's last writer reads "not
// recorded", with or without a parent record (the comment the existing trigger writes there is a
// log of what the system did and names nobody for any rule): staff are told so, the customer is not
// told the time waits for WSO2, Accept is refused with its own code and writes nothing. Propose a
// different time is the way on: a plain Re-schedule that writes no answer and asks the customers
// again, and the customer then approves -- one more customer approval, by design.
func TestChangeRequestProposalIntegration_AGenuineProposalAfterAnUnrelatedStaffEditReadsNotRecorded(t *testing.T) {
	type edit struct {
		name string
		run  func(f *crFlow, id string)
	}
	patchAs := func(req ...domain.PatchChangeRequestRequest) func(f *crFlow, id string) {
		return func(f *crFlow, id string) {
			f.t.Helper()
			for _, r := range req {
				if _, err := f.staffPatch(id, crFlowPeerBID, r); err != nil {
					f.t.Fatalf("an unrelated staff edit: %v", err)
				}
			}
		}
	}
	edits := []edit{
		{"a work note", patchAs(domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")})},
		{"an additional note", patchAs(domain.PatchChangeRequestRequest{Comment: sp("we will get back to you")})},
		{"the hold on and off", patchAs(domain.PatchChangeRequestRequest{OnHold: boolp(true), OnHoldReason: sp("freeze")}, domain.PatchChangeRequestRequest{OnHold: boolp(false)})},
		{"an engineer assigned", patchAs(domain.PatchChangeRequestRequest{AssignedEngineerID: sp(crFlowPeerAID)})},
	}
	for _, withParent := range []bool{false, true} {
		withParent := withParent
		where := "no parent record"
		if withParent {
			where = "under a parent record"
		}
		for _, e := range edits {
			e := e
			t.Run(where+", after "+e.name, func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
				if withParent {
					f.giveParent(id, crScopeProjectA, false)
				}
				f.mustPropose(id, crScopeUserA1, rsStart2)
				if p := f.proposalOf(id); p == nil || p.ProposerRecorded == nil || !*p.ProposerRecorded || p.CanAccept == nil || !*p.CanAccept {
					t.Fatalf("before the edit the proposal is recorded and acceptable: %+v", p)
				}

				e.run(f, id)
				if got := f.updatedBy(id); got != crFlowEmail(crFlowPeerBID) {
					t.Fatalf("work_item.updated_by after the edit = %q, want the staff member who edited last", got)
				}

				// What staff are told: a time is stored, nobody is recorded as having proposed it.
				p := f.proposalOf(id)
				if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept ||
					p.AcceptBlockedReason == nil || *p.AcceptBlockedReason != msgAcceptNobodyRecordedFull ||
					p.ProposedByEmail != nil || p.ProposedByName != nil || p.ProposedOn != nil {
					t.Fatalf("customerProposal after the edit = %+v, want a stored time, nobody recorded, Accept blocked in these words", p)
				}
				// What the customers are told: nothing waits for WSO2 on their account (not even the one who proposed it).
				for _, uid := range []string{crScopeUserA1, crScopeUserA2} {
					mine, err := f.getAsContact(id, uid)
					if err != nil || mine.CustomerProposal == nil || mine.CustomerProposal.Answer != "unanswered" || mine.CustomerProposal.ProposerRecorded != nil || mine.CustomerProposal.EndOn != nil {
						t.Fatalf("%s's view after the edit = %+v (%v), want history: unanswered", uid, mine.CustomerProposal, err)
					}
				}

				// Accept is refused, with its own code, and writes nothing.
				before := f.snap(id)
				_, err := f.accept(id)
				f.wantConflictExact("Accept after an unrelated edit", err, msgAcceptNobodyRecordedFull)
				wantRefusalCode(t, "Accept after an unrelated edit", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
				f.wantRefusedSame("Accept after an unrelated edit", id, before, err)

				// The way on: Propose a different time, as the page sends it (it names the stored time).
				stages := f.stageLabels(id)
				if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
					t.Fatalf("Propose a different time after an unrelated edit: %v", err)
				}
				f.wantPlanned(id, "after the different time", rsStart3, rsEnd3)
				f.wantConversation(id, "after the different time", rsStart2, "") // no DISAGREE: nobody proposed that time
				if got := f.stageLabels(id); got != stages+",Customer Approval" {
					t.Fatalf("stages after the different time = %s, want %s and one fresh customer request", got, stages)
				}
				custom := f.customerStages(id)
				assertApprovers(t, "the customers asked again", custom[len(custom)-1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
				// ... and the customer then approves it: one extra approval, which is the whole cost.
				if err := f.decide(id, crScopeUserA2, "approved"); err != nil {
					t.Fatalf("the customer's approval of the different time: %v", err)
				}
				f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
				f.wantPlanned(id, "after the customer's approval", rsStart3, rsEnd3)
			})
		}
	}
}

// The proposer is the change's last writer when that writer is a registered contact of its project,
// and nobody else is looked at: not the comment the existing trigger writes on the parent record
// (which names the customer who proposed), not any other log.
func TestChangeRequestProposalIntegration_TheProposerIsTheLastWriterAndNothingElse(t *testing.T) {
	for _, tc := range []struct {
		name   string
		writer string
		// recorded: whether the last writer is named as the proposer.
		recorded bool
	}{
		{"the proposing contact", crFlowEmail(crScopeUserA1), true},
		{"a colleague who is a registered contact of the project", crFlowEmail(crScopeUserA2), true},
		{"a WSO2 user", "wso2.engineer@example.com", false},
		{"the sync's own stamp", crSyncStamp, false},
		{"nobody at all (a blank last writer)", "", false},
		{"a contact of another project", crFlowEmail(crScopeUserB1), false},
	} {
		tc := tc
		for _, withParent := range []bool{false, true} {
			withParent := withParent
			where := "no parent record"
			if withParent {
				where = "a parent record whose comment names the proposing contact"
			}
			t.Run(tc.name+", "+where, func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
				if withParent {
					f.giveParent(id, crScopeProjectA, false)
				}
				f.mustPropose(id, crScopeUserA1, rsStart2)
				if withParent {
					if got := f.commentsOn("3aaaaaaa-0000-0000-0000-0000000000c1"); len(got) != 1 || !strings.HasPrefix(got[0], crFlowEmail(crScopeUserA1)+": ") {
						t.Fatalf("the trigger's comment = %q, want one that names the proposing contact", got)
					}
				}
				f.execSQL(`UPDATE work_item SET updated_by = $2 WHERE id = $1`, id, tc.writer)
				p := f.proposalOf(id)
				if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded != tc.recorded {
					t.Fatalf("customerProposal with %s as the last writer = %+v, want proposerRecorded %v", tc.name, p, tc.recorded)
				}
				if tc.recorded {
					if p.ProposedByEmail == nil || *p.ProposedByEmail != tc.writer || p.ProposedOn == nil || p.CanAccept == nil || !*p.CanAccept {
						t.Fatalf("customerProposal = %+v, want the last writer named (%s) and Accept possible", p, tc.writer)
					}
				} else if p.ProposedByEmail != nil || p.CanAccept == nil || *p.CanAccept {
					t.Fatalf("customerProposal = %+v, want nobody named and Accept blocked", p)
				}
			})
		}
	}
}

// A stored time that nobody is recorded as having proposed, in every shape the data can leave one:
// whatever the last writer is (a WSO2 user, the sync's stamp, a blank), however it got there (written
// by the previous system, left over from an earlier cycle, a migrated row), on a native change and on
// a migrated one. Every staff act treats it alike (wantNoProposalToAnswer).
func TestChangeRequestProposalIntegration_AStoredTimeNobodyProposedInEveryShape(t *testing.T) {
	t.Run("a migrated row, the sync's own stamp as the last writer", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		f.syncWritesConversationAs(id, crSyncStamp, sp(rsStart2), "")
		f.wantNoProposalToAnswer(id, rsStart2)
	})
	t.Run("a migrated row with a blank last writer", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		f.syncWritesConversationAs(id, "", sp(rsStart2), "")
		f.wantNoProposalToAnswer(id, rsStart2)
	})
	t.Run("a date a WSO2 user wrote in the previous system", func(t *testing.T) {
		f, id := leftoverInCustomerApproval(t)
		f.wantNoProposalToAnswer(id, rsStart2)
	})
	t.Run("a stale date on re-entry: the change left Customer Approval with a customer's proposal waiting, staff worked on it, and it came back", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		// Out of Customer Approval (the sync, or an engineer's rollback), a staff member writes to it, back in again.
		f.setState(id, "AUTHORIZE")
		if _, err := f.staffPatch(id, crFlowPeerBID, domain.PatchChangeRequestRequest{WorkNote: sp("back for another look")}); err != nil {
			t.Fatalf("a staff edit while the change is out of Customer Approval: %v", err)
		}
		f.setState(id, "CUSTOMER_APPROVAL")
		if p := f.proposalOf(id); p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded {
			t.Fatalf("customerProposal on re-entry = %+v, want the old date waiting with nobody recorded", p)
		}
		f.wantNoProposalToAnswer(id, rsStart2)
	})
	t.Run("a customer's proposal that a WSO2 user then rewrote", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		f.syncWritesConversationAs(id, "wso2.engineer@example.com", sp(rsStart3), "")
		f.wantNoProposalToAnswer(id, rsStart3)
	})
}

// No statement of the proposer path reads the comment table (or any log) to name a proposer. The
// unit test of change_request_proposer_test.go proves it for the functions; this one traces every
// statement the acts and the detail read send to the database, on a change whose parent record
// carries the comment the existing trigger wrote for the proposal.
func TestChangeRequestProposalIntegration_NoStatementReadsACommentToNameAProposer(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	parent := f.giveParent(id, crScopeProjectA, false)
	f.mustPropose(id, crScopeUserA1, rsStart2)
	if got := f.commentsOn(parent); len(got) != 1 {
		t.Fatalf("the parent record carries %q, want the one comment the trigger wrote for the proposal", got)
	}
	trace := f.traceSQL()
	// The detail read, as staff and as a customer, then every staff answer: refused and not.
	f.proposalOf(id)
	if _, err := f.getAsContact(id, crScopeUserA1); err != nil {
		t.Fatalf("a customer's read: %v", err)
	}
	f.mustAccept(id) // the proposer is recorded: this is the happy path
	g := newCustomerGroupFlow(t)
	gid := g.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	g.giveParent(gid, crScopeProjectA, false)
	g.mustPropose(gid, crScopeUserA1, rsStart2)
	gtrace := g.traceSQL()
	if _, err := g.staffPatch(gid, crFlowPeerBID, domain.PatchChangeRequestRequest{WorkNote: sp("an edit")}); err != nil {
		t.Fatalf("an edit: %v", err)
	}
	g.proposalOf(gid)
	if _, err := g.accept(gid); err == nil {
		t.Fatal("Accept after an unrelated edit was accepted")
	}
	if err := g.counter(gid, sp(rsStart3), sp(rsEnd3)); err != nil {
		t.Fatalf("Propose a different time: %v", err)
	}
	for name, tr := range map[string]*sqlTrace{"the happy path": trace, "the overwritten proposal": gtrace} {
		stmts := tr.statements()
		if len(stmts) < 10 || !tr.sawWorkItemLastWriterRead() || !tr.sawTheContactCheckInABatch() {
			t.Fatalf("%s: the trace holds %d statements, without the proposer's own reads (the last writer, the contact check): the tracer is not working", name, len(stmts))
		}
		for _, q := range stmts {
			if readsCommentTable(q) {
				t.Fatalf("%s: a statement reads the comment table in the proposer path:\n%s", name, q)
			}
		}
	}
}

// commentTableRead matches a statement that reads (or joins) the comment table.
var commentTableRead = regexp.MustCompile(`(?is)\b(from|join)\s+"?comment"?\b`)

func readsCommentTable(sql string) bool { return commentTableRead.MatchString(sql) }

// sqlTrace records every statement a pool sends: those sent one by one (pgx.QueryTracer, the
// statements of a transaction) and those sent in a batch (pgx.BatchTracer, which is how the scoped
// reads of the repository reach the database, the detail read among them).
type sqlTrace struct {
	mu  sync.Mutex
	sql []string
}

func (t *sqlTrace) add(sql string) {
	t.mu.Lock()
	t.sql = append(t.sql, sql)
	t.mu.Unlock()
}

func (t *sqlTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	t.add(d.SQL)
	return ctx
}

func (t *sqlTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (t *sqlTrace) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	return ctx
}

func (t *sqlTrace) TraceBatchQuery(_ context.Context, _ *pgx.Conn, d pgx.TraceBatchQueryData) {
	t.add(d.SQL)
}

func (t *sqlTrace) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData) {}

func (t *sqlTrace) statements() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.sql...)
}

// sawWorkItemLastWriterRead: the proposer's own read of the last writer is among the statements.
func (t *sqlTrace) sawWorkItemLastWriterRead() bool {
	for _, q := range t.statements() {
		if strings.Contains(q, "updated_by") && strings.Contains(q, "FROM work_item") {
			return true
		}
	}
	return false
}

// sawTheContactCheckInABatch: the registered-contact check of the proposer, which the detail read
// sends through the scoped (batched) path, is among the statements.
func (t *sqlTrace) sawTheContactCheckInABatch() bool {
	for _, q := range t.statements() {
		if strings.Contains(q, "FROM project_contact pc") {
			return true
		}
	}
	return false
}

// traceSQL points the flow at a pool that records every statement it sends (the same database and
// role as the flow's own pool) and returns the recording.
func (f *crFlow) traceSQL() *sqlTrace {
	f.t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv("CHANGE_REQUEST_TEST_DSN"))
	if err != nil {
		f.t.Fatalf("parse DSN: %v", err)
	}
	tr := &sqlTrace{}
	cfg.ConnConfig.Tracer = tr
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		f.t.Fatalf("connect: %v", err)
	}
	f.t.Cleanup(pool.Close)
	f.pool, f.scoped = pool, repository.NewScoped(pool)
	f.repo = repository.NewChangeRequestRepository(f.scoped)
	return tr
}

// A customer proposes a start; WSO2 proposes another; the customer proposes a third; WSO2 accepts;
// the change goes on to Closed. Every turn is answered as the contract says, with the proposer read
// from the last writer each time: the writes in between (the staff member's counter, the second
// contact's proposal) replace the last writer, and the rule follows them, with or without a parent
// record (the comments the existing trigger writes there change nothing).
func TestChangeRequestProposalIntegration_ARecordedProposerIsAcceptedEndToEnd(t *testing.T) {
	for name, withParent := range map[string]bool{"no parent record": false, "under a parent record": true} {
		withParent := withParent
		t.Run(name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
			f.setPlanned(id, rsStart1, rsEnd1)
			f.driveToCustomerApproval(id)
			if withParent {
				f.giveParent(id, crScopeProjectA, false)
			}
			f.mustPropose(id, crScopeUserA1, rsStart2)
			if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
				t.Fatalf("a different time: %v", err)
			}
			f.wantConversation(id, "after the different time", rsStart2, "DISAGREE")
			f.mustPropose(id, crScopeUserA2, rsStartEarly)
			p := f.proposalOf(id)
			if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || !*p.ProposerRecorded || p.ProposedByEmail == nil || *p.ProposedByEmail != crFlowEmail(crScopeUserA2) {
				t.Fatalf("customerProposal after the second proposal = %+v, want it pending and recorded as A2's", p)
			}
			f.mustAccept(id)
			f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
			f.wantPlanned(id, "after Accept", rsStartEarly, endFor(rsStartEarly))
			f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
			f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
			f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
			if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
				t.Fatalf("the customer's review: %v", err)
			}
			f.expect(id, "in Closed", "CLOSED")
		})
	}
}

// A customer who proposes exactly the time that is stored but nobody proposed is told so, not
// that it "is already proposed and waiting for WSO2's response" (it is not a proposal, and a
// second write of the same value could not make them its proposer).
func TestChangeRequestProposalIntegration_ACustomerCannotClaimAStoredTimeByWritingItAgain(t *testing.T) {
	f, id := leftoverInCustomerApproval(t)
	before := f.snap(id)
	_, err := f.proposeAs(id, crScopeUserA1, rsStart2)
	f.wantExact("proposing the stored time", err, msgProposalStoredNotProposed)
	if !strings.Contains(msgProposalStoredNotProposed, "nobody is recorded as having proposed it") {
		t.Fatalf("the refusal does not say why: %s", msgProposalStoredNotProposed)
	}
	f.wantRefusedSame("proposing the stored time", id, before, err)
	// A recorded proposal keeps the message it always had.
	g := newCustomerGroupFlow(t)
	gid := g.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	g.mustPropose(gid, crScopeUserA1, rsStart2)
	_, err = g.proposeAs(gid, crScopeUserA2, rsStart2)
	g.wantExact("proposing a time somebody already proposed", err, msgProposalAlreadyProposed)
}

const msgProposalStoredNotProposed = "that time is already stored on this change request, although nobody is recorded as having proposed it: propose a different start"

const msgNoRecordedProposalToDecline = "no customer is recorded as having proposed the time stored on this change request, so there is no proposal to decline: send the new planned window to re-schedule it"
