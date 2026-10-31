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
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

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
// consented to -- a staff action standing in for the customer's answer. The proposal records its
// proposer on change_request (customer_proposed_by_user_id / _for / _on, migration 0249) in its own
// write, so nothing written to the change afterwards by anybody else (a work note, the hold, a
// whole-form resend, another writer's write to the work item) un-records it. The record counts only
// while it is about the stored time and names a registered contact of the project; entering Customer
// Approval voids it, and so does a plain Re-schedule. A time
// stored before 0249 has no record (customer_proposed_for NULL): for it alone the proposer is still
// the LAST WRITER (work_item.updated_by) read BEFORE the answer's own write replaces it, and a write
// that replaces that writer records them first (migration 0250). No comment, audit, outbox or other
// row names a proposer.

// msgAcceptNobodyRecordedFull is the whole refusal: pinned word for word because the pages and
// the contract show it.
const msgAcceptNobodyRecordedFull = `nobody is recorded as having proposed this time (it may have been written by someone at WSO2 or left over from an earlier cycle), so it cannot be accepted: use "Propose a different time" to ask the customer to approve a time`

// leftoverInCustomerApproval is a change in Customer Approval that carries a date nobody is
// recorded as having proposed: a WSO2 user's, or one left over from an earlier cycle. It waits by
// the predicate (a date that differs from the plan, no answer, nothing else asked). It has no record
// (the shape of a time stored before migration 0249) and its last writer is a staff member.
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

	t.Run("the answer's own write does not touch the record: a staff member who is not the last writer accepts or counters", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		if got := f.updatedBy(id); got != crFlowEmail(crScopeUserA1) {
			t.Fatalf("work_item.updated_by after the proposal = %q, want the proposing contact", got)
		}
		f.wantProposalRecord(id, "after the proposal", crScopeUserA1, rsStart2)
		// Accept stamps the accepting staff member on the work item as its very first statement; the
		// record is read from change_request, which that stamp does not touch.
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
		// The record is history now (nothing waits), and still says who proposed that time.
		g.wantProposalRecord(gid, "after the different time", crScopeUserA2, rsStart2)
	})

	// With a parent record too: the comment the existing trigger wrote there for the proposal names
	// the contact, and is no reference either.
	for where, withParent := range map[string]bool{"no parent record": false, "under a parent record whose comment names the contact": true} {
		withParent := withParent
		t.Run("a proposer who is no longer a registered contact is not recorded, "+where, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			parent := ""
			if withParent {
				parent = f.giveParent(id, crScopeProjectA, false)
			}
			f.mustPropose(id, crScopeUserA1, rsStart2)
			f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED'::project_contact_state_enum
			           WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, crScopeProjectA, crFlowEmail(crScopeUserA1))
			// The record still names them; the rule asks whether they are a registered contact NOW.
			f.wantProposalRecord(id, "after the contact left", crScopeUserA1, rsStart2)
			if withParent {
				f.wantTheCommentNames(parent, crScopeUserA1)
			}
			p := f.proposalOf(id)
			if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept ||
				p.AcceptBlockedReason == nil || *p.AcceptBlockedReason != msgAcceptNobodyRecordedFull || p.ProposedByEmail != nil {
				t.Fatalf("customerProposal after the contact left = %+v, want a stored time, nobody recorded, Accept blocked", p)
			}
			before := f.snap(id)
			_, err := f.accept(id)
			wantRefusalCode(t, "Accept of a time proposed by a contact who has left", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
			f.wantRefusedSame("Accept of a time proposed by a contact who has left", id, before, err)
		})
	}

	t.Run("a proposer who is a contact of ANOTHER project is not recorded, by the record or (legacy) as the last writer", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.syncWritesConversationAs(id, crFlowEmail(crScopeUserB1), sp(rsStart2), "")
		p := f.proposalOf(id)
		if p == nil || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept {
			t.Fatalf("customerProposal for a time written by a contact of another project = %+v", p)
		}
		f.execSQL(`UPDATE change_request SET customer_proposed_by_user_id = $2::uuid, customer_proposed_for = customer_updated_on,
		                  customer_proposed_on = now() WHERE id = $1`, id, crScopeUserB1)
		f.wantProposalRecord(id, "naming a contact of another project", crScopeUserB1, rsStart2)
		p = f.proposalOf(id)
		if p == nil || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept {
			t.Fatalf("customerProposal for a time recorded as proposed by a contact of another project = %+v", p)
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

// A customer's genuine proposal stays recorded as theirs whatever staff write to the change after it
// (a work note, an additional note, the hold on and off, a whole-form resend, an engineer assigned
// by another writer), with or without a parent record: staff are told who proposed it and when, the
// customer who proposed it is told it waits for WSO2 and their colleague that a colleague's does,
// and Accept applies it. Until migration 0249 every one of these writes replaced the last writer,
// which was all that named the proposer, so the proposal read "not recorded" and Accept was refused.
func TestChangeRequestProposalIntegration_AGenuineProposalStaysRecordedAfterAnyStaffWrite(t *testing.T) {
	type edit struct {
		name string
		run  func(f *crFlow, id string)
	}
	patchAs := func(req ...domain.PatchChangeRequestRequest) func(f *crFlow, id string) {
		return func(f *crFlow, id string) {
			f.t.Helper()
			for _, r := range req {
				if _, err := f.staffPatch(id, crFlowPeerBID, r); err != nil {
					f.t.Fatalf("a staff edit: %v", err)
				}
			}
		}
	}
	edits := []edit{
		{"a work note", patchAs(domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")})},
		{"an additional note", patchAs(domain.PatchChangeRequestRequest{Comment: sp("we will get back to you")})},
		{"the hold on and off", patchAs(domain.PatchChangeRequestRequest{OnHold: boolp(true), OnHoldReason: sp("freeze")}, domain.PatchChangeRequestRequest{OnHold: boolp(false)})},
		// In Customer Approval the change has been approved, so its fields are read-only; a
		// whole-form resend still goes through and still replaces the last writer.
		{"a whole-form resend", patchAs(domain.PatchChangeRequestRequest{Title: sp(crFlowSubject)})},
		// The PATCH refuses a new assignee once the change is approved; another writer of the work
		// item (an integration, a manual fix) can still assign one, and stamp itself as last writer.
		{"an engineer assigned by another writer", func(f *crFlow, id string) {
			f.execSQL(`UPDATE work_item SET assigned_to_id = $2::uuid, updated_on = now(), updated_by = $3 WHERE id = $1`,
				id, crFlowPeerAID, crFlowEmail(crFlowPeerBID))
		}},
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
				f.wantProposalRecord(id, "after the proposal", crScopeUserA1, rsStart2)

				e.run(f, id)
				if got := f.updatedBy(id); got != crFlowEmail(crFlowPeerBID) {
					t.Fatalf("work_item.updated_by after the edit = %q, want the staff member who edited last", got)
				}
				f.wantProposalRecord(id, "after the edit", crScopeUserA1, rsStart2)

				// What staff are told: the customer proposed it, who and when, and Accept is possible.
				p := f.proposalOf(id)
				if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || !*p.ProposerRecorded || p.CanAccept == nil || !*p.CanAccept ||
					p.AcceptBlockedReason != nil || p.ProposedByEmail == nil || *p.ProposedByEmail != crFlowEmail(crScopeUserA1) ||
					p.ProposedByName == nil || *p.ProposedByName != "Alice Aaron" || p.ProposedOn == nil {
					t.Fatalf("customerProposal after the edit = %+v, want Alice's proposal recorded, with when, and Accept possible", p)
				}
				// What the customers are told: it waits for WSO2, and whose it is.
				for uid, own := range map[string]bool{crScopeUserA1: true, crScopeUserA2: false} {
					mine, err := f.getAsContact(id, uid)
					if err != nil || mine.CustomerProposal == nil || mine.CustomerProposal.Answer != "pending" ||
						mine.CustomerProposal.ProposedByViewer == nil || *mine.CustomerProposal.ProposedByViewer != own || mine.CustomerProposal.ProposedByEmail != nil {
						t.Fatalf("%s's view after the edit = %+v (%v), want pending, proposedByViewer %v, no names", uid, mine.CustomerProposal, err, own)
					}
				}

				// Accept applies it: Scheduled, the proposal as the planned window, AGREE.
				f.mustAccept(id)
				f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
				f.wantPlanned(id, "after Accept", rsStart2, endFor(rsStart2))
				f.wantConversation(id, "after Accept", rsStart2, "AGREE")
			})
		}
	}

	t.Run("after a staff edit Propose a different time answers the proposal (DISAGREE), and so does a decline", func(t *testing.T) {
		for name, window := range map[string][2]*string{"a different time": {sp(rsStart3), sp(rsEnd3)}, "a decline": {nil, nil}} {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.mustPropose(id, crScopeUserA1, rsStart2)
			patchAs(domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")})(f, id)
			// A request that never saw the proposal does not answer it: it is still waiting.
			before := f.snap(id)
			err := f.reschedule(id, sp(rsStart3), sp(rsEnd3))
			f.wantConflictExact(name+": a Re-schedule that never saw the proposal", err, msgCounterProposalMissedFor(rsStart2))
			f.wantRefusedSame(name+": a Re-schedule that never saw the proposal", id, before, err)
			if err := f.counter(id, window[0], window[1]); err != nil {
				t.Fatalf("%s after a staff edit: %v", name, err)
			}
			f.wantConversation(id, name+" after a staff edit", rsStart2, "DISAGREE")
		}
	})
}

// The proposer is the contact the proposal recorded, whoever wrote the change last: a colleague, a
// WSO2 user, the sync's own stamp, nobody, a contact of another project. Nothing else is looked at:
// not the last writer (of a row that has a record), not the comment the existing trigger writes on
// the parent record, not any other log.
func TestChangeRequestProposalIntegration_TheProposerIsTheRecordNotTheLastWriter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		writer string
	}{
		{"the proposing contact", crFlowEmail(crScopeUserA1)},
		{"a colleague who is a registered contact of the project", crFlowEmail(crScopeUserA2)},
		{"a WSO2 user", "wso2.engineer@example.com"},
		{"the sync's own stamp", crSyncStamp},
		{"nobody at all (a blank last writer)", ""},
		{"a contact of another project", crFlowEmail(crScopeUserB1)},
	} {
		tc := tc
		for _, withParent := range []bool{false, true} {
			withParent := withParent
			where := "no parent record"
			if withParent {
				where = "a parent record whose comment names the proposing contact"
			}
			t.Run(tc.name+" as the last writer, "+where, func(t *testing.T) {
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
				if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || !*p.ProposerRecorded ||
					p.ProposedByEmail == nil || *p.ProposedByEmail != crFlowEmail(crScopeUserA1) || p.ProposedOn == nil || p.CanAccept == nil || !*p.CanAccept {
					t.Fatalf("customerProposal with %s as the last writer = %+v, want Alice named as the proposer and Accept possible", tc.name, p)
				}
				f.mustAccept(id)
				f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
			})
		}
	}
}

// A second proposal replaces the record: the contact who proposed last is the proposer, for the time
// they proposed. A record that is no longer about the stored time (another writer moved it) names
// nobody, and neither does a record whose contact is gone.
func TestChangeRequestProposalIntegration_TheRecordNamesTheProposerOfTheStoredTimeOnly(t *testing.T) {
	t.Run("a second proposal by another contact replaces the recorded proposer", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		f.wantProposalRecord(id, "after Alice's proposal", crScopeUserA1, rsStart2)
		f.mustPropose(id, crScopeUserA2, rsStart3)
		f.wantProposalRecord(id, "after Bob's proposal", crScopeUserA2, rsStart3)
		p := f.proposalOf(id)
		if p == nil || p.StartOn != rsStart3 || p.ProposerRecorded == nil || !*p.ProposerRecorded || p.ProposedByEmail == nil ||
			*p.ProposedByEmail != crFlowEmail(crScopeUserA2) || p.ProposedByName == nil || *p.ProposedByName != "Bob Bell" {
			t.Fatalf("customerProposal after the second proposal = %+v, want Bob's time, recorded as Bob's", p)
		}
		for uid, own := range map[string]bool{crScopeUserA1: false, crScopeUserA2: true} {
			mine, err := f.getAsContact(id, uid)
			if err != nil || mine.CustomerProposal == nil || mine.CustomerProposal.ProposedByViewer == nil || *mine.CustomerProposal.ProposedByViewer != own {
				t.Fatalf("%s's view after the second proposal = %+v (%v), want proposedByViewer %v", uid, mine.CustomerProposal, err, own)
			}
		}
		f.mustAccept(id)
		f.wantPlanned(id, "after Accept", rsStart3, endFor(rsStart3))
	})

	t.Run("a record whose time is no longer the stored one reads not recorded, and no staff act answers the stored time", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		// Another writer moves the stored time and leaves the customer as the last writer: neither
		// the record (about rsStart2) nor the last writer (not read for a row that has a record) names anybody.
		f.syncWritesConversationAs(id, crFlowEmail(crScopeUserA1), sp(rsStart3), "")
		f.wantProposalRecord(id, "after the stored time moved", crScopeUserA1, rsStart2)
		p := f.proposalOf(id)
		if p == nil || p.Answer != "pending" || p.StartOn != rsStart3 || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept ||
			p.AcceptBlockedReason == nil || *p.AcceptBlockedReason != msgAcceptNobodyRecordedFull || p.ProposedByEmail != nil {
			t.Fatalf("customerProposal after the stored time moved = %+v, want it pending, nobody recorded, Accept blocked", p)
		}
		if mine, err := f.getAsContact(id, crScopeUserA1); err != nil || mine.CustomerProposal == nil || mine.CustomerProposal.Answer != "unanswered" {
			t.Fatalf("Alice's view after the stored time moved = %+v (%v), want history: unanswered", mine.CustomerProposal, err)
		}
		f.wantNoProposalToAnswer(id, rsStart3)
	})

	t.Run("a record that names nobody (its user gone: ON DELETE SET NULL) reads not recorded", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		// What the foreign key does when the user row is deleted (pinned from the catalog by
		// TheRecordedUserIsAForeignKeyClearedOnDelete); the fixture's users are shared, so the
		// column is cleared the way the delete would clear it.
		f.execSQL(`UPDATE change_request SET customer_proposed_by_user_id = NULL WHERE id = $1`, id)
		p := f.proposalOf(id)
		if p == nil || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept {
			t.Fatalf("customerProposal with nobody in the record = %+v, want nobody recorded and Accept blocked", p)
		}
		// The proposing contact is still the last writer; that names nobody for a row with a record.
		if got := f.updatedBy(id); got != crFlowEmail(crScopeUserA1) {
			t.Fatalf("work_item.updated_by = %q, want the proposing contact (the fixture must have them as the last writer)", got)
		}
		_, err := f.accept(id)
		wantRefusalCode(t, "Accept of a time whose record names nobody", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
	})

	t.Run("the record is never written by anything but the proposal: a refused proposal leaves it as it was", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		before := f.snap(id)
		_, err := f.proposeAs(id, crScopeUserA2, rsStart2)
		f.wantExact("Bob proposing the time Alice proposed", err, msgProposalAlreadyProposed)
		f.wantRefusedSame("Bob proposing the time Alice proposed", id, before, err)
		f.wantProposalRecord(id, "after the refused proposal", crScopeUserA1, rsStart2)
	})
}

// LEGACY: a time stored before migration 0249 has no record (customer_proposed_for NULL), and for
// it the rule that held until then still holds: the proposer is the change's last writer, read
// before the act's own write, when that writer is a registered contact of the project. Since
// migration 0250 a write that replaces such a writer records them first, so a proposal made before
// the deploy and still waiting stays acceptable whoever writes to the change after it; a migrated
// date stays "not recorded" exactly as before; the next proposal records itself.
func TestChangeRequestProposalIntegration_ALegacyProposalIsReadByItsLastWriter(t *testing.T) {
	// legacyProposal is a change in Customer Approval holding the start a contact proposed before
	// 0249: the date and the contact as the last writer, and no record.
	legacyProposal := func(t *testing.T, writer string) (*crFlow, string) {
		t.Helper()
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.syncWritesConversationAs(id, writer, sp(rsStart2), "")
		f.wantProposalRecord(id, "on a legacy row", "", "")
		return f, id
	}

	t.Run("the last writer is the proposer when a registered contact of the project, and nobody else is", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			writer   string
			recorded bool
		}{
			{"the proposing contact", crFlowEmail(crScopeUserA1), true},
			{"a colleague who is a registered contact", crFlowEmail(crScopeUserA2), true},
			{"a WSO2 user", "wso2.engineer@example.com", false},
			{"the sync's own stamp", crSyncStamp, false},
			{"nobody (a blank last writer)", "", false},
			{"a contact of another project", crFlowEmail(crScopeUserB1), false},
		} {
			f, id := legacyProposal(t, tc.writer)
			p := f.proposalOf(id)
			if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded != tc.recorded {
				t.Fatalf("%s: customerProposal = %+v, want proposerRecorded %v", tc.name, p, tc.recorded)
			}
			if tc.recorded {
				if p.ProposedByEmail == nil || *p.ProposedByEmail != tc.writer || p.ProposedOn == nil || p.CanAccept == nil || !*p.CanAccept {
					t.Fatalf("%s: customerProposal = %+v, want the last writer named and Accept possible", tc.name, p)
				}
			} else if p.ProposedByEmail != nil || p.CanAccept == nil || *p.CanAccept {
				t.Fatalf("%s: customerProposal = %+v, want nobody named and Accept blocked", tc.name, p)
			}
		}
	})

	t.Run("Accept of a legacy proposal: its own first write records the last writer it replaces (0250), then applies it", func(t *testing.T) {
		f, id := legacyProposal(t, crFlowEmail(crScopeUserA1))
		if _, err := f.staffPatch(id, crFlowPeerAID, f.acceptReq(id)); err != nil {
			t.Fatalf("Accept of a legacy proposal: %v", err)
		}
		f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
		f.wantPlanned(id, "after Accept", rsStart2, endFor(rsStart2))
		f.wantProposalRecord(id, "after Accept", crScopeUserA1, rsStart2)
	})

	t.Run("a staff write after it no longer loses the proposer: migration 0250 records them first", func(t *testing.T) {
		f, id := legacyProposal(t, crFlowEmail(crScopeUserA1))
		before := f.proposalOf(id)
		if before == nil || before.ProposedOn == nil {
			t.Fatalf("customerProposal of a legacy proposal = %+v, want the last writer and when they wrote", before)
		}
		if _, err := f.staffPatch(id, crFlowPeerBID, domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")}); err != nil {
			t.Fatalf("a staff work note: %v", err)
		}
		f.wantProposalRecord(id, "after a staff work note", crScopeUserA1, rsStart2)
		p := f.proposalOf(id)
		if p == nil || p.ProposerRecorded == nil || !*p.ProposerRecorded || p.ProposedByEmail == nil || *p.ProposedByEmail != crFlowEmail(crScopeUserA1) ||
			p.ProposedOn == nil || *p.ProposedOn != *before.ProposedOn || p.CanAccept == nil || !*p.CanAccept {
			t.Fatalf("customerProposal of a legacy proposal after a staff write = %+v, want Alice recorded as proposing it at %s and Accept possible", p, *before.ProposedOn)
		}
		f.mustAccept(id)
		f.wantPlanned(id, "after Accept", rsStart2, endFor(rsStart2))
	})

	t.Run("a migrated-shaped row (a stored date, no record, a staff member as the last writer) reads not recorded, as before", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		f.syncWritesConversationAs(id, crFlowEmail(crFlowPeerBID), sp(rsStart2), "")
		f.wantProposalRecord(id, "on a migrated row", "", "")
		p := f.proposalOf(id)
		if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept {
			t.Fatalf("customerProposal of a migrated row = %+v, want a stored time, nobody recorded", p)
		}
		f.wantNoProposalToAnswer(id, rsStart2)
	})

	t.Run("the next proposal on a legacy row records itself, and is then recorded whatever is written after it", func(t *testing.T) {
		f, id := legacyProposal(t, "wso2.engineer@example.com")
		f.mustPropose(id, crScopeUserA2, rsStart3)
		f.wantProposalRecord(id, "after the proposal", crScopeUserA2, rsStart3)
		if _, err := f.staffPatch(id, crFlowPeerBID, domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")}); err != nil {
			t.Fatalf("a staff work note: %v", err)
		}
		if p := f.proposalOf(id); p == nil || p.ProposerRecorded == nil || !*p.ProposerRecorded || p.ProposedByEmail == nil || *p.ProposedByEmail != crFlowEmail(crScopeUserA2) {
			t.Fatalf("customerProposal = %+v, want Bob's proposal recorded", p)
		}
		f.mustAccept(id)
	})
}

// A stored time that nobody is recorded as having proposed, in every shape the data can leave one:
// whatever the last writer is (a WSO2 user, the sync's stamp, a blank), however it got there (written
// by the previous system, left over from an earlier cycle, a migrated row, a record voided on
// re-entering Customer Approval, a stored time another writer moved off its record), on a native
// change and on a migrated one. Every staff act treats it alike (wantNoProposalToAnswer).
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
	for where, withParent := range map[string]bool{"": false, ", under a parent record whose comment names the contact": true} {
		withParent := withParent
		t.Run("a stale date on re-entry: the change left Customer Approval with a customer's proposal waiting, staff worked on it, and it came back"+where, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			parent := ""
			if withParent {
				parent = f.giveParent(id, crScopeProjectA, false)
			}
			f.mustPropose(id, crScopeUserA1, rsStart2)
			// Out of Customer Approval (the sync, or an engineer's rollback), a staff member writes to it, back in again.
			f.setState(id, "AUTHORIZE")
			if _, err := f.staffPatch(id, crFlowPeerBID, domain.PatchChangeRequestRequest{WorkNote: sp("back for another look")}); err != nil {
				t.Fatalf("a staff edit while the change is out of Customer Approval: %v", err)
			}
			f.setState(id, "CUSTOMER_APPROVAL")
			// Entering Customer Approval voided the record (the 0249 trigger): it is about the stored time
			// and names nobody, so the row is not read by the legacy last-writer rule either -- nor by the
			// comment on the parent record, which still names the contact.
			f.wantProposalRecord(id, "on re-entry", "", rsStart2)
			if withParent {
				f.wantTheCommentNames(parent, crScopeUserA1)
			}
			if p := f.proposalOf(id); p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded {
				t.Fatalf("customerProposal on re-entry = %+v, want the old date waiting with nobody recorded", p)
			}
			f.wantNoProposalToAnswer(id, rsStart2)
		})
	}
	t.Run("a stale date on re-entry with no staff write in between: the customer is still the last writer, and the record is still void", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		f.setState(id, "AUTHORIZE")
		f.setState(id, "CUSTOMER_APPROVAL")
		if got := f.updatedBy(id); got != crFlowEmail(crScopeUserA1) {
			t.Fatalf("work_item.updated_by = %q, want the proposing contact (the fixture must have them as the last writer)", got)
		}
		f.wantProposalRecord(id, "on re-entry", "", rsStart2)
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
	g.mustAccept(gid) // still recorded after a staff edit
	// A legacy time (no record) on a parent record: the last-writer rule, refused, then a different time.
	h := newCustomerGroupFlow(t)
	hid := h.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	h.giveParent(hid, crScopeProjectA, false)
	h.syncWritesConversationAs(hid, "wso2.engineer@example.com", sp(rsStart2), "")
	htrace := h.traceSQL()
	h.proposalOf(hid)
	if _, err := h.accept(hid); err == nil {
		t.Fatal("Accept of a time nobody proposed was accepted")
	}
	if err := h.counter(hid, sp(rsStart3), sp(rsEnd3)); err != nil {
		t.Fatalf("Propose a different time: %v", err)
	}
	// A record that no longer counts (its contact has left the project) under a parent record whose
	// comment names that contact: the record path names nobody, and nothing falls back to the comment.
	k := newCustomerGroupFlow(t)
	kid := k.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	kparent := k.giveParent(kid, crScopeProjectA, false)
	k.mustPropose(kid, crScopeUserA1, rsStart2)
	k.wantTheCommentNames(kparent, crScopeUserA1)
	k.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED'::project_contact_state_enum
	           WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, crScopeProjectA, crFlowEmail(crScopeUserA1))
	ktrace := k.traceSQL()
	if p := k.proposalOf(kid); p == nil || p.ProposerRecorded == nil || *p.ProposerRecorded || p.ProposedByEmail != nil {
		t.Fatalf("customerProposal of a record whose contact has left = %+v, want nobody recorded", p)
	}
	if _, err := k.accept(kid); err == nil {
		t.Fatal("Accept of a time whose recorded contact has left was accepted")
	}
	if err := k.counter(kid, sp(rsStart3), sp(rsEnd3)); err != nil {
		t.Fatalf("Propose a different time: %v", err)
	}
	k.wantConversation(kid, "after the different time", rsStart2, "")
	for name, tr := range map[string]*sqlTrace{"the happy path": trace, "a proposal after a staff edit": gtrace, "a legacy time nobody proposed": htrace,
		"a record whose contact has left": ktrace} {
		stmts := tr.statements()
		if len(stmts) < 10 || !tr.sawTheProposalRecordRead() || !tr.sawTheContactCheckInABatch() {
			t.Fatalf("%s: the trace holds %d statements, without the proposer's own reads (the record, the contact check): the tracer is not working", name, len(stmts))
		}
		for _, q := range stmts {
			if readsCommentTable(q) {
				t.Fatalf("%s: a statement reads the comment table in the proposer path:\n%s", name, q)
			}
		}
	}
}

// wantTheCommentNames asserts the parent record carries the one comment the existing trigger (0053)
// wrote for the proposal, and that it names the proposing contact (its author is the change's last
// writer at the time, the contact): the comment a fallback would read a proposer from.
func (f *crFlow) wantTheCommentNames(parent, userID string) {
	f.t.Helper()
	if got := f.commentsOn(parent); len(got) != 1 || !strings.HasPrefix(got[0], crFlowEmail(userID)+": ") {
		f.t.Fatalf("the parent record carries %q, want the one comment the trigger wrote for the proposal, naming %s", got, crFlowEmail(userID))
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

// sawTheProposalRecordRead: the proposer's own read of the record (migration 0249) is among the statements.
func (t *sqlTrace) sawTheProposalRecordRead() bool {
	for _, q := range t.statements() {
		if strings.Contains(q, "customer_proposed_for") && strings.Contains(q, "FROM change_request") {
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
// from the record each time: the second contact's proposal replaces it, and the rule follows, with
// or without a parent record (the comments the existing trigger writes there change nothing).
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
			f.wantProposalRecord(id, "after the second proposal", crScopeUserA2, rsStartEarly)
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

	// The customer's own act reads the record too, not the last writer. Each case below answers the
	// other way by the last-writer rule.
	t.Run("after a staff write a colleague who sends the recorded time is told it waits for WSO2", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		if _, err := f.staffPatch(id, crFlowPeerBID, domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")}); err != nil {
			t.Fatalf("a staff work note: %v", err)
		}
		if got := f.updatedBy(id); got != crFlowEmail(crFlowPeerBID) {
			t.Fatalf("work_item.updated_by after the work note = %q, want the staff member (the fixture must have staff as the last writer)", got)
		}
		before := f.snap(id)
		_, err := f.proposeAs(id, crScopeUserA2, rsStart2)
		f.wantExact("a colleague proposing the recorded time after a staff write", err, msgProposalAlreadyProposed)
		f.wantRefusedSame("a colleague proposing the recorded time after a staff write", id, before, err)
		f.wantProposalRecord(id, "after the refused proposal", crScopeUserA1, rsStart2)
	})
	t.Run("after re-entry the proposing contact, still the last writer, is told the time is stored and nobody proposed it", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		f.setState(id, "AUTHORIZE")
		f.setState(id, "CUSTOMER_APPROVAL")
		if got := f.updatedBy(id); got != crFlowEmail(crScopeUserA1) {
			t.Fatalf("work_item.updated_by = %q, want the proposing contact (the fixture must have them as the last writer)", got)
		}
		f.wantProposalRecord(id, "on re-entry", "", rsStart2)
		before := f.snap(id)
		_, err := f.proposeAs(id, crScopeUserA1, rsStart2)
		f.wantExact("the proposing contact sending the voided time again", err, msgProposalStoredNotProposed)
		f.wantRefusedSame("the proposing contact sending the voided time again", id, before, err)
	})
}

// A plain Re-schedule asks the customers again about a new window and answers nothing: a new round,
// with no change of state. A record that did not count when it ran (its contact had left the project,
// or another approval was being asked, so nothing waited) must not count again later: Accept would
// then apply the earlier round's time over the window the customers were asked about. The Re-schedule
// voids it as entering Customer Approval does (who and when cleared, customer_proposed_for kept at
// the stored time). A counter-proposal and a decline keep it (DISAGREE makes it history; see
// AcceptNeedsARecordedProposer).
func TestChangeRequestProposalIntegration_APlainRescheduleVoidsTheRecord(t *testing.T) {
	t.Run("the proposer left the project, a plain Re-schedule, the proposer registered again: still not recorded", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		setAlice := func(state string) {
			f.execSQL(`UPDATE project_contact SET state = $3::project_contact_state_enum
			           WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, crScopeProjectA, crFlowEmail(crScopeUserA1), state)
		}
		setAlice("DEACTIVATED")
		// Nobody current is recorded, so nothing waits: a plain Re-schedule, no answer written.
		if err := f.reschedule(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("a Re-schedule over a time whose proposer has left: %v", err)
		}
		f.wantPlanned(id, "after the Re-schedule", rsStart3, rsEnd3)
		f.wantConversation(id, "after the Re-schedule", rsStart2, "")
		f.wantProposalRecord(id, "after the Re-schedule", "", rsStart2)
		setAlice("REGISTERED")
		p := f.proposalOf(id)
		if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept ||
			p.AcceptBlockedReason == nil || *p.AcceptBlockedReason != msgAcceptNobodyRecordedFull || p.ProposedByEmail != nil {
			t.Fatalf("customerProposal once the proposer is registered again = %+v, want the earlier round's time stored, nobody recorded, Accept blocked", p)
		}
		if mine, err := f.getAsContact(id, crScopeUserA1); err != nil || mine.CustomerProposal == nil || mine.CustomerProposal.Answer != "unanswered" {
			t.Fatalf("Alice's view once registered again = %+v (%v), want history: unanswered", mine.CustomerProposal, err)
		}
		before := f.snap(id)
		_, err := f.accept(id)
		wantRefusalCode(t, "Accept of the earlier round's time", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
		f.wantRefusedSame("Accept of the earlier round's time", id, before, err)
	})

	t.Run("a plain Re-schedule while another approval is asked, which then resolves: still not recorded", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		// An approval that is not the customer's is asked (an unlabeled stage of the CAB's group, as the
		// sync can leave one): nothing waits, so the request is a plain Re-schedule.
		cab := crCABGroupID
		stage := f.seedLooseStageInGroup(id, nil, &cab, 1, map[string]string{crCABMemberUserID1: "REQUESTED"})
		f.wantAnswer(id, "while another approval is asked", "unanswered")
		if err := f.reschedule(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("a Re-schedule while another approval is asked: %v", err)
		}
		f.wantConversation(id, "after the Re-schedule", rsStart2, "")
		f.wantProposalRecord(id, "after the Re-schedule", "", rsStart2)
		// The other approval resolves: the stored time waits again by the predicate, and nobody is recorded.
		f.execSQL(`UPDATE approval_stage_approver SET state = 'APPROVED' WHERE stage_id = $1::uuid AND state = 'REQUESTED'`, stage)
		p := f.proposalOf(id)
		if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept {
			t.Fatalf("customerProposal once the other approval resolved = %+v, want the earlier round's time stored, nobody recorded, Accept blocked", p)
		}
		before := f.snap(id)
		_, err := f.accept(id)
		wantRefusalCode(t, "Accept of the earlier round's time", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
		f.wantRefusedSame("Accept of the earlier round's time", id, before, err)
	})

	t.Run("on a legacy row the Re-schedule leaves a void record, which the last-writer rule does not read", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.syncWritesConversationAs(id, crFlowEmail(crScopeUserA1), sp(rsStart2), "")
		f.wantProposalRecord(id, "on a legacy row", "", "")
		f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED'::project_contact_state_enum
		           WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, crScopeProjectA, crFlowEmail(crScopeUserA1))
		if err := f.reschedule(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("a Re-schedule over a legacy time whose writer has left: %v", err)
		}
		f.wantProposalRecord(id, "after the Re-schedule", "", rsStart2)
		f.execSQL(`UPDATE project_contact SET state = 'REGISTERED'::project_contact_state_enum
		           WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, crScopeProjectA, crFlowEmail(crScopeUserA1))
		// Even with the contact written back as the last writer, the row has a (void) record now.
		f.execSQL(`UPDATE work_item SET updated_by = $2 WHERE id = $1`, id, crFlowEmail(crScopeUserA1))
		if p := f.proposalOf(id); p == nil || p.ProposerRecorded == nil || *p.ProposerRecorded {
			t.Fatalf("customerProposal = %+v, want nobody recorded", p)
		}
	})
}

// Migration 0249 is safe to run again (a renamed migration file runs again from scratch, see the
// repo-root CLAUDE.md): two more runs change no stored record, a recorded proposal still reads
// recorded and acceptable, and the trigger still voids the record on entering Customer Approval.
// DDL needs the table's owner: under a role that is not one (csm_app) the test is skipped.
func TestChangeRequestProposalIntegration_ProposerMigrationIsSafeToRunAgain(t *testing.T) {
	f := newCustomerGroupFlow(t)
	var owner bool
	if err := f.pool.QueryRow(context.Background(),
		`SELECT pg_has_role(current_user, c.relowner, 'USAGE') FROM pg_class c WHERE c.oid = 'change_request'::regclass`).Scan(&owner); err != nil {
		t.Fatalf("read the owner of change_request: %v", err)
	}
	if !owner {
		t.Skip("the connected role does not own change_request, so it cannot run the migration's DDL: connect as its owner")
	}
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.mustPropose(id, crScopeUserA1, rsStart2)
	voided := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.mustPropose(voided, crScopeUserA2, rsStart3)
	f.setState(voided, "AUTHORIZE")
	f.setState(voided, "CUSTOMER_APPROVAL")

	sqlBytes, err := os.ReadFile("../../migrations/0249_change_request_customer_proposer.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := f.pool.Exec(context.Background(), string(sqlBytes)); err != nil {
			t.Fatalf("re-running migration 0249 (pass %d): %v", i+1, err)
		}
	}
	f.wantProposalRecord(id, "after re-running the migration", crScopeUserA1, rsStart2)
	f.wantProposalRecord(voided, "after re-running the migration", "", rsStart3)
	if p := f.proposalOf(id); p == nil || p.ProposerRecorded == nil || !*p.ProposerRecorded || p.CanAccept == nil || !*p.CanAccept {
		t.Fatalf("customerProposal after re-running the migration = %+v, want Alice's proposal recorded and Accept possible", p)
	}
	// The trigger is there once, and still voids on entering Customer Approval.
	var triggers int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_trigger WHERE tgrelid = 'change_request'::regclass AND tgname = 'change_request_void_customer_proposer'`).Scan(&triggers); err != nil || triggers != 1 {
		t.Fatalf("void triggers on change_request after re-running the migration = %d (%v), want 1", triggers, err)
	}
	f.setState(id, "AUTHORIZE")
	f.setState(id, "CUSTOMER_APPROVAL")
	f.wantProposalRecord(id, "on re-entry after re-running the migration", "", rsStart2)
}

// A record whose user is gone names nobody because customer_proposed_by_user_id references "user"
// ON DELETE SET NULL (migration 0249): a delete of the user clears the column instead of being
// refused or leaving an id that names nobody. Read from the catalog, which every role can read, so
// it holds under csm_app as under the owner.
func TestChangeRequestProposalIntegration_TheRecordedUserIsAForeignKeyClearedOnDelete(t *testing.T) {
	f := newCustomerGroupFlow(t)
	rows, err := f.pool.Query(context.Background(), `
		SELECT c.confrelid = '"user"'::regclass,
		       (SELECT r.attname::text FROM pg_attribute r WHERE r.attrelid = c.confrelid AND r.attnum = c.confkey[1]),
		       c.confdeltype::text, array_length(c.conkey, 1)
		FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		WHERE c.conrelid = 'change_request'::regclass AND c.contype = 'f' AND a.attname = 'customer_proposed_by_user_id'`)
	if err != nil {
		t.Fatalf("read the foreign keys of customer_proposed_by_user_id: %v", err)
	}
	defer rows.Close()
	var found int
	for rows.Next() {
		var toUser bool
		var refColumn, onDelete string
		var columns int
		if err := rows.Scan(&toUser, &refColumn, &onDelete, &columns); err != nil {
			t.Fatalf("scan: %v", err)
		}
		found++
		if !toUser || refColumn != "id" || onDelete != "n" || columns != 1 {
			t.Fatalf(`foreign key of customer_proposed_by_user_id = (references "user": %v, column %q, on delete: %q, columns: %d), want "user"(id) ON DELETE SET NULL ('n') on that one column`,
				toUser, refColumn, onDelete, columns)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the foreign keys of customer_proposed_by_user_id: %v", err)
	}
	if found != 1 {
		t.Fatalf("customer_proposed_by_user_id has %d foreign keys, want exactly one", found)
	}
}

// The plan-date notice "Customer has updated the plan start date" takes its actor from the record of
// who proposed the date that the proposal's own outbox row carries (service.crPlanDateActor), not from
// the last writer CRNoticeRepository.Details reads when the outbox is drained. The real outbox row of
// a proposal carries the record for the very instant it moved the date to; a staff write saved after
// it, before any drain, changes who Details names (a staff member, or a WSO2 user, which would
// suppress the notice) but not the contact UserActor names from that row. UserActor names a user by
// the rule Details names the last writer by, pinned below on the same users as both.
func TestChangeRequestProposalIntegration_TheProposalNoticeNamesTheRecordedProposer(t *testing.T) {
	const (
		wso2UserID     = "3eeeeeee-0000-0000-0000-000000000001"
		wso2NoEmailID  = "3eeeeeee-0000-0000-0000-000000000002"
		namelessUserID = "3eeeeeee-0000-0000-0000-000000000003"
		noSuchUserID   = "3eeeeeee-0000-0000-0000-0000000000ff"
	)
	// seedUsers adds a WSO2 user (mixed-case email), a WSO2 user with no email (named by user name
	// alone, as work_item.updated_by then is) and a customer-side user with no name at all.
	seedUsers := func(f *crFlow) {
		f.t.Helper()
		clean := func() {
			_, _ = f.scoped.Exec(f.sys, `DELETE FROM "user" WHERE id IN ($1, $2, $3)`, wso2UserID, wso2NoEmailID, namelessUserID)
		}
		clean()
		f.t.Cleanup(clean)
		for _, u := range []struct {
			id, userName                     string
			name, firstName, lastName, email *string
		}{
			{wso2UserID, "cr-notice-actor-staff@wso2.com", sp("Nimal Perera"), sp("Nimal"), sp("Perera"), sp("CR-Notice-Actor-Staff@WSO2.com")},
			{wso2NoEmailID, "cr-notice-actor-noemail@wso2.com", sp("No Email"), nil, nil, nil},
			{namelessUserID, "cr-notice-actor-nameless", nil, nil, nil, sp("cr-notice-actor-nameless@example.com")},
		} {
			f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
			           VALUES ($1, now(), now(), 'cr-notice-test', 'cr-notice-test', $2, $3, $4, $5, $6, true, false)`,
				u.id, u.userName, u.name, u.firstName, u.lastName, u.email)
		}
	}

	t.Run("a proposal's outbox row records the contact for the date it moved, whoever writes the change before the drain", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		seedUsers(f)
		notices := repository.NewCRNoticeRepository(f.scoped)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)

		var raw []byte
		if err := f.scoped.QueryRow(f.sys,
			`SELECT snapshot::text::bytea FROM event_outbox WHERE entity_type = 'change_request' AND entity_id = $1 ORDER BY id DESC LIMIT 1`,
			id).Scan(&raw); err != nil {
			t.Fatalf("read the proposal's outbox row: %v", err)
		}
		var snap map[string]any
		if err := json.Unmarshal(raw, &snap); err != nil {
			t.Fatalf("decode the snapshot: %v", err)
		}
		by, _ := snap["customer_proposed_by_user_id"].(string)
		forText, _ := snap["customer_proposed_for"].(string)
		dateText, _ := snap["customer_updated_on"].(string)
		proposedFor, errFor := time.Parse(time.RFC3339Nano, forText)
		date, errDate := time.Parse(time.RFC3339Nano, dateText)
		want, _ := time.Parse(time.RFC3339, rsStart2)
		if by != crScopeUserA1 || errFor != nil || errDate != nil || !proposedFor.Equal(date) || !date.Equal(want) {
			t.Fatalf("the proposal's snapshot records %q for %q, date %q; want Alice's user for the date, both %s", by, forText, dateText, rsStart2)
		}

		// A staff work note before the drain: Details now names that staff member, not Alice.
		if _, err := f.staffPatch(id, crFlowPeerBID, domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")}); err != nil {
			t.Fatalf("a staff work note: %v", err)
		}
		d, err := notices.Details(f.sys, id)
		if err != nil {
			t.Fatalf("Details: %v", err)
		}
		if d.ActorName != "Flow User CR" || d.ActorIsWSO2 {
			t.Fatalf("Details' actor after the work note = %q (WSO2 %v), want the staff member who wrote it", d.ActorName, d.ActorIsWSO2)
		}
		// A WSO2 user as the last writer: Details would suppress the notice.
		f.execSQL(`UPDATE work_item SET updated_by = $2 WHERE id = $1`, id, "cr-notice-actor-staff@wso2.com")
		if d, err = notices.Details(f.sys, id); err != nil || !d.ActorIsWSO2 {
			t.Fatalf("Details' actor with a WSO2 last writer = %+v (%v), want a WSO2 actor", d, err)
		}
		// The recorded contact is named from the row, whoever wrote last.
		actor, err := notices.UserActor(f.sys, by)
		if err != nil {
			t.Fatalf("UserActor: %v", err)
		}
		if actor != (repository.CRNoticeActor{Name: "Last First"}) {
			t.Fatalf("UserActor(the recorded contact) = %+v, want Alice by Details' rule (last name first: \"Last First\") and not WSO2", actor)
		}
	})

	t.Run("UserActor names a user by the rule Details names the last writer by", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		seedUsers(f)
		notices := repository.NewCRNoticeRepository(f.scoped)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		for _, tc := range []struct {
			name, userID, writer string
			want                 repository.CRNoticeActor
		}{
			{"a WSO2 user, by email", wso2UserID, "cr-notice-actor-staff@wso2.com", repository.CRNoticeActor{Name: "Perera Nimal", IsWSO2: true}},
			{"a WSO2 user with no email, by user name", wso2NoEmailID, "cr-notice-actor-noemail@wso2.com", repository.CRNoticeActor{Name: "No Email", IsWSO2: true}},
			{"a customer contact", crScopeUserA1, crFlowEmail(crScopeUserA1), repository.CRNoticeActor{Name: "Last First"}},
			{"a user with no name", namelessUserID, "cr-notice-actor-nameless", repository.CRNoticeActor{}},
		} {
			got, err := notices.UserActor(f.sys, tc.userID)
			if err != nil {
				t.Fatalf("%s: UserActor: %v", tc.name, err)
			}
			if got != tc.want {
				t.Errorf("%s: UserActor = %+v, want %+v", tc.name, got, tc.want)
			}
			f.execSQL(`UPDATE work_item SET updated_by = $2 WHERE id = $1`, id, tc.writer)
			d, err := notices.Details(f.sys, id)
			if err != nil {
				t.Fatalf("%s: Details: %v", tc.name, err)
			}
			if (repository.CRNoticeActor{Name: d.ActorName, IsWSO2: d.ActorIsWSO2}) != got {
				t.Errorf("%s: Details names the last writer %q (WSO2 %v), UserActor names the user %+v: want the same", tc.name, d.ActorName, d.ActorIsWSO2, got)
			}
		}
		// A user that does not exist (deleted since the row was written): unnamed, not WSO2, no error.
		if got, err := notices.UserActor(f.sys, noSuchUserID); err != nil || got != (repository.CRNoticeActor{}) {
			t.Fatalf("UserActor(a user that does not exist) = %+v, %v; want a zero value and no error", got, err)
		}
	})
}

const msgProposalStoredNotProposed = "that time is already stored on this change request, although nobody is recorded as having proposed it: propose a different start"

const msgNoRecordedProposalToDecline = "no customer is recorded as having proposed the time stored on this change request, so there is no proposal to decline: send the new planned window to re-schedule it"
