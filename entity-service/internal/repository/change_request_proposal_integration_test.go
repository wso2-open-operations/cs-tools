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
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// A customer's proposed time and WSO2's answer to it: the previous system's own mechanism,
// customer_updated_on (the proposed START) and customer_updated_date_confirmation (WSO2's
// answer). See change_request_customer_proposal.go. The harness (planLength, proposeAs,
// acceptReq, counterReq, snap, rowCounts, migratedInCustomerApproval, ...) is in
// change_request_proposal_harness_integration_test.go. Fixtures use the repository's
// synthetic project A (contacts Alice and Bob) and shapes only.

// ---------------------------------------------------------------------------
// When does a proposal "wait"? The allowlist predicate, on every shape of data
// ---------------------------------------------------------------------------

// The predicate matrix. Each row is a change as the data can leave it -- a native one, a
// migrated one in flight, history, a date WSO2 wrote, an old date, a change waiting on an
// internal approval -- and says what the read model makes of it and what Accept does. A
// time waits only in Customer Approval with a finite proposed start that differs from the
// plan, no answer, and no approval but the customer's still asked; anything else is history
// or not ours to answer, and no act of this feature touches it. A waiting time is a
// customer's PROPOSAL, and answerable by WSO2, only when a registered contact of the project
// is recorded as having proposed it (recorded): Accept is refused for any other (409, the
// proposer's own errorCode) and a customer is never told it is theirs.
func TestChangeRequestProposalIntegration_PredicateMatrix(t *testing.T) {
	future := time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Second).Format(time.RFC3339)
	past := "2020-06-01T09:00:00Z"
	type row struct {
		name  string
		setup func(f *crFlow) string
		// answer is customerProposal.answer ("none": the field is absent).
		answer string
		// acceptRefusal: Accept's 409 (a fragment); "" when the proposal waits and Accept is not tried here.
		acceptRefusal string
		// recorded: ProposerRecorded expected when the time waits.
		recorded bool
		// canAccept / blocked: what a staff reader is told when the proposal waits.
		canAccept bool
		blocked   string
	}
	inCA := func(f *crFlow) string {
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		return id
	}
	// An approval that is NOT the customer's, still being asked, in each of the ways the data shows one.
	blockedBy := func(label *string, group *string, withCustomerGroup bool) func(f *crFlow) string {
		return func(f *crFlow) string {
			f.seedSREGroup()
			id := f.migratedInCustomerApproval()
			if !withCustomerGroup {
				f.execSQL(`UPDATE change_request SET customer_group_id = NULL WHERE id = $1`, id)
			}
			f.syncWritesConversation(id, sp(future), "")
			if label != nil {
				f.seedLooseStageInGroup(id, label, group, 30, map[string]string{crCABMemberUserID1: "REQUESTED"})
			} else {
				f.seedMigratedStage(id, group, 30, map[string]string{crCABMemberUserID1: "REQUESTED"})
			}
			return id
		}
	}
	cab, sre, review := crCABGroupID, crFlowSREGroupID, "Review"
	rows := []row{
		{name: "M1 a migrated change in Customer Approval, the sync wrote the proposal, the previous system's own customer stage (unlabeled, the customer group) asks Alice and Bob",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "pending", recorded: false, canAccept: false, blocked: msgAcceptNobodyRecorded},
		{name: "M1a the same, the date being one a customer proposed in the previous system (the sync set the customer as the last writer)",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.customerWroteProposal(id, crScopeUserA1, future)
				return id
			}, answer: "pending", recorded: true, canAccept: true},
		{name: "M1b the same with no customer_group_id: the stage cannot be proved to be the customer's, so it blocks",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.execSQL(`UPDATE change_request SET customer_group_id = NULL WHERE id = $1`, id)
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M2 AUTHORIZE with a live unlabeled CAB-group stage and a stale proposed date: never a proposal",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.setState(id, "AUTHORIZE")
				f.execSQL(`UPDATE approval_stage_approver SET state = 'CANCELLED' WHERE work_item_id = $1`, id)
				c := crCABGroupID
				f.seedMigratedStage(id, &c, 20, map[string]string{crCABMemberUserID1: "REQUESTED", crCABMemberUserID2: "REQUESTED"})
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "unanswered", acceptRefusal: "but it is in Authorize"},
		{name: "M2b a native change waiting on its live CAB stage with a stale date",
			setup: func(f *crFlow) string {
				id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
				f.setPlanned(id, rsStart1, rsEnd1)
				f.requestApproval(id)
				if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
					t.Fatalf("peer approval: %v", err)
				}
				f.expect(id, "in Authorize", "AUTHORIZE", "canceled")
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "unanswered", acceptRefusal: "but it is in Authorize"},
		{name: "M3 CLOSED, an AGREE in the history, the proposal applied",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.setState(id, "CLOSED")
				f.syncWritesConversation(id, sp(rsStart1), "AGREE")
				return id
			}, answer: "agreed", acceptRefusal: "but it is in Closed"},
		{name: "M3b SCHEDULED, an AGREE whose date is not the plan (the sync moved them apart)",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.setState(id, "SCHEDULED")
				f.syncWritesConversation(id, sp(future), "AGREE")
				return id
			}, answer: "agreed", acceptRefusal: "but it is in Scheduled"},
		{name: "M4 Customer Approval, AGREE, the proposed date is the planned start",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.syncWritesConversation(id, sp(rsStart1), "AGREE")
				return id
			}, answer: "agreed", acceptRefusal: msgNothingWaiting},
		{name: "M4b Customer Approval, DISAGREE standing: WSO2 asked for another time",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.syncWritesConversation(id, sp(future), "DISAGREE")
				return id
			}, answer: "disagreed", acceptRefusal: msgNothingWaiting},
		{name: "M5a an approval of the CAB's own group (unlabeled) is still asked",
			setup: blockedBy(nil, &cab, true), answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M5b an approval labelled Review is still asked",
			setup: blockedBy(&review, nil, true), answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M5c an approval of a group nobody has named is still asked",
			setup: blockedBy(nil, &sre, true), answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M5d an unlabeled approval with no group is still asked",
			setup: blockedBy(nil, nil, true), answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M6 a NULL state with a proposed date",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.setState(id, "")
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "unanswered", acceptRefusal: "but it is in New"},
		{name: "M7 Customer Approval, no answer, the proposed date is the planned start",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.syncWritesConversation(id, sp(rsStart1), "")
				return id
			}, answer: "unanswered", acceptRefusal: msgNothingWaiting},
		{name: "M8 a date a WSO2 user wrote in the previous system (the last writer is staff): waits, proposer not recorded, Accept blocked",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.syncWritesConversation(id, sp(future), "")
				return id
			}, answer: "pending", recorded: false, canAccept: false, blocked: msgAcceptNobodyRecorded},
		{name: "M8b the same on a change that has a parent record: the comment the existing trigger writes there is no reference, the last writer is staff",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.giveParent(id, crScopeProjectA, false)
				f.syncWritesConversationAs(id, "wso2.engineer@example.com", sp(future), "")
				return id
			}, answer: "pending", recorded: false, canAccept: false, blocked: msgAcceptNobodyRecorded},
		{name: "M8c a migrated row whose last writer is the sync's own stamp: nobody is recorded",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.syncWritesConversationAs(id, crSyncStamp, sp(future), "")
				return id
			}, answer: "pending", recorded: false, canAccept: false, blocked: msgAcceptNobodyRecorded},
		{name: "M8d the same with a blank last writer",
			setup: func(f *crFlow) string {
				id := f.migratedInCustomerApproval()
				f.syncWritesConversationAs(id, "", sp(future), "")
				return id
			}, answer: "pending", recorded: false, canAccept: false, blocked: msgAcceptNobodyRecorded},
		{name: "M9 a stale date from an old cycle, in the past: waits, proposer not recorded, Accept is blocked for that first",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.syncWritesConversation(id, sp(past), "")
				return id
			}, answer: "pending", recorded: false, canAccept: false, blocked: msgAcceptNobodyRecorded},
		{name: "M9b a date in the past that a customer proposed (recorded): Accept is blocked because it has passed",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.customerWroteProposal(id, crScopeUserA1, past)
				return id
			}, answer: "pending", recorded: true, canAccept: false, blocked: "has already passed"},
		{name: "M10 a date the customer proposed through the API: waits, proposer recorded",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.mustPropose(id, crScopeUserA1, rsStart2)
				return id
			}, answer: "pending", recorded: true, canAccept: true},
		{name: "M10b the same after a staff edit replaced the last writer: waits, proposer no longer recorded (nothing else names them)",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.mustPropose(id, crScopeUserA1, rsStart2)
				if _, err := f.patch(id, domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")}); err != nil {
					t.Fatalf("a staff edit: %v", err)
				}
				return id
			}, answer: "pending", recorded: false, canAccept: false, blocked: msgAcceptNobodyRecorded},
		{name: "M10c the same under a parent record: the comment the existing trigger wrote at the proposal names the customer, and is not read",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.giveParent(id, crScopeProjectA, false)
				f.mustPropose(id, crScopeUserA1, rsStart2)
				if _, err := f.patch(id, domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")}); err != nil {
					t.Fatalf("a staff edit: %v", err)
				}
				if got := f.updatedBy(id); got == crFlowEmail(crScopeUserA1) {
					t.Fatalf("the staff edit left the customer as the last writer (%q): the row proves nothing", got)
				}
				if got := f.commentsOn("3aaaaaaa-0000-0000-0000-0000000000c1"); len(got) != 1 {
					t.Fatalf("the trigger's comment on the parent record = %q, want exactly one for the proposal (the fixture must have one to not read)", got)
				}
				return id
			}, answer: "pending", recorded: false, canAccept: false, blocked: msgAcceptNobodyRecorded},
		{name: "M10d a proposal by one contact, a later write to the change by their colleague (also a registered contact): the last writer is the recorded proposer, nothing else is looked at",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.mustPropose(id, crScopeUserA1, rsStart2)
				f.execSQL(`UPDATE work_item SET updated_by = $2 WHERE id = $1`, id, crFlowEmail(crScopeUserA2))
				return id
			}, answer: "pending", recorded: true, canAccept: true},
		{name: "M11 an infinite proposed date reads as none and does not break the read",
			setup: func(f *crFlow) string {
				id := inCA(f)
				f.execSQL(`UPDATE change_request SET customer_updated_on = 'infinity'::timestamptz WHERE id = $1`, id)
				return id
			}, answer: "none"},
		{name: "M12 no proposal at all",
			setup: inCA, answer: "none"},
	}
	for _, r := range rows {
		r := r
		t.Run(r.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := r.setup(f)
			cr := f.get(id)
			got := "none"
			if cr.CustomerProposal != nil {
				got = cr.CustomerProposal.Answer
			}
			if got != r.answer {
				t.Fatalf("answer = %q, want %q (%+v)", got, r.answer, cr.CustomerProposal)
			}
			if r.answer == "none" {
				if cr.CustomerProposal != nil {
					t.Fatalf("customerProposal = %+v, want absent", cr.CustomerProposal)
				}
				return
			}
			p := cr.CustomerProposal
			if p.StartOn == "" {
				t.Fatal("the proposal carries no start")
			}
			if r.answer == "pending" {
				// What a staff reader is told about who proposed it and whether Accept would work.
				if p.ProposerRecorded == nil || *p.ProposerRecorded != r.recorded {
					t.Fatalf("proposerRecorded = %v, want %v", p.ProposerRecorded, r.recorded)
				}
				if r.recorded != (p.ProposedByEmail != nil) || (!r.recorded && (p.ProposedByName != nil || p.ProposedOn != nil)) {
					t.Fatalf("proposer fields = %v / %v / %v with recorded=%v", p.ProposedByEmail, p.ProposedByName, p.ProposedOn, r.recorded)
				}
				if p.CanAccept == nil || *p.CanAccept != r.canAccept {
					t.Fatalf("canAccept = %v, want %v", p.CanAccept, r.canAccept)
				}
				if r.blocked != "" && (p.AcceptBlockedReason == nil || !strings.Contains(*p.AcceptBlockedReason, r.blocked)) {
					t.Fatalf("acceptBlockedReason = %v, want it to say %q", p.AcceptBlockedReason, r.blocked)
				}
				if r.canAccept && p.AcceptBlockedReason != nil {
					t.Fatalf("acceptBlockedReason = %q while Accept is possible", *p.AcceptBlockedReason)
				}
				if p.EndOn == nil {
					t.Fatal("a waiting proposal over a planned window carries its end")
				}
				// What a customer is told about the same time: theirs (or a colleague's) when somebody is
				// recorded as having proposed it, and nothing waits for WSO2 on their account otherwise.
				seen, err := f.getAsContact(id, crScopeUserA2)
				if err != nil || seen.CustomerProposal == nil {
					t.Fatalf("a customer's read of the change request = %+v (%v)", seen.CustomerProposal, err)
				}
				cp := seen.CustomerProposal
				if r.recorded {
					if cp.Answer != "pending" || cp.ProposerRecorded == nil || !*cp.ProposerRecorded || cp.ProposedByViewer == nil || cp.EndOn == nil {
						t.Fatalf("a customer's view of a recorded proposal = %+v, want it pending with its proposer", cp)
					}
				} else if cp.Answer != "unanswered" || cp.EndOn != nil || cp.ProposerRecorded != nil || cp.ProposedByViewer != nil || cp.CanAccept != nil {
					t.Fatalf("a customer's view of a time nobody is recorded as having proposed = %+v, want history: unanswered and nothing that says it waits", cp)
				}
				if cp.ProposedByEmail != nil || cp.ProposedByName != nil || cp.ProposedOn != nil || cp.AcceptBlockedReason != nil {
					t.Fatalf("a customer was told who proposed it or what WSO2 may do: %+v", cp)
				}
				if !r.canAccept {
					// What the read model says Accept would refuse, the PATCH refuses in the same words and
					// writes nothing; with no proposer recorded it is the proposer's own code.
					before := f.snap(id)
					_, err := f.patch(id, domain.PatchChangeRequestRequest{
						ConfirmCustomerUpdatedDate: sp("agree"), ExpectedCustomerUpdatedOn: sp(p.StartOn),
						ExpectedPlannedStartOn: cr.PlannedStartOn, ExpectedPlannedEndOn: cr.PlannedEndOn})
					if err == nil {
						t.Fatal("Accept of a time the read model blocks was accepted")
					}
					reason := *p.AcceptBlockedReason
					if !r.recorded {
						wantRefusalCode(t, "Accept of a time nobody proposed", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
					}
					if !strings.Contains(err.Error(), reason) {
						t.Fatalf("Accept refused with %q, the read model said %q", err.Error(), reason)
					}
					f.wantRefusedSame("Accept of a time the read model blocks", id, before, err)
				}
				return
			}
			// History or not ours to answer: no waiting-only fact, and Accept is refused with
			// nothing written.
			if p.EndOn != nil || p.ProposerRecorded != nil || p.CanAccept != nil || p.ProposedByEmail != nil {
				t.Fatalf("a proposal that is not waiting carries waiting-only facts: %+v", p)
			}
			before := f.snap(id)
			_, err := f.patch(id, domain.PatchChangeRequestRequest{
				ConfirmCustomerUpdatedDate: sp("agree"), ExpectedCustomerUpdatedOn: sp(p.StartOn),
				ExpectedPlannedStartOn: cr.PlannedStartOn, ExpectedPlannedEndOn: cr.PlannedEndOn})
			f.wantConflictContaining("Accept on a proposal that is not waiting", err, r.acceptRefusal)
			f.wantRefusedSame("Accept on a proposal that is not waiting", id, before, err)
		})
	}
}

const (
	msgNothingWaiting = "no new time proposed by the customer is waiting for a response on this change request"
	// msgAcceptNobodyRecorded is the beginning of the refusal (and of the read model's reason) for
	// a time nobody is recorded as having proposed; the whole text is pinned by
	// TestChangeRequestProposalIntegration_AcceptNeedsARecordedProposer.
	msgAcceptNobodyRecorded = "nobody is recorded as having proposed this time"
)

// ---------------------------------------------------------------------------
// WSO2 accepts the proposed time
// ---------------------------------------------------------------------------

// Accept proposed time: one request of its own that names the proposal and the planned
// window the page showed. Every way it can be refused is refused with the words the
// contract promises and leaves the change request exactly as it was; the one that is fine
// schedules the change with the proposal as its start.
func TestChangeRequestProposalIntegration_AcceptHappyAndEveryRefusal(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.mustPropose(id, crScopeUserA1, rsStart2)
	good := f.acceptReq(id)
	before := f.snap(id)
	with := func(mod func(*domain.PatchChangeRequestRequest)) domain.PatchChangeRequestRequest {
		r := good
		mod(&r)
		return r
	}
	type kind int
	const (
		validation kind = iota
		conflict
	)
	for _, tc := range []struct {
		name string
		req  domain.PatchChangeRequestRequest
		kind kind
		msg  string
	}{
		{"decline through the answer field", with(func(r *domain.PatchChangeRequestRequest) { r.ConfirmCustomerUpdatedDate = sp("disagree") }), validation, msgAcceptMustBeAgree},
		{"an empty answer", with(func(r *domain.PatchChangeRequestRequest) { r.ConfirmCustomerUpdatedDate = sp("") }), validation, msgAcceptMustBeAgree},
		{"no version of the proposal", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedCustomerUpdatedOn = nil }), validation,
			"expectedCustomerUpdatedOn is required with confirmCustomerUpdatedDate: it names the proposed time you are accepting"},
		{"no planned start shown", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedStartOn = nil }), validation, msgAcceptNeedsWindow},
		{"no planned end shown", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedEndOn = nil }), validation, msgAcceptNeedsWindow},
		{"with a title", with(func(r *domain.PatchChangeRequestRequest) { r.Title = sp("renamed") }), validation, msgAcceptAlone},
		{"with a state", with(func(r *domain.PatchChangeRequestRequest) {
			r.State = stateptr(domain.ChangeRequestStateScheduled)
		}), validation, msgAcceptAlone},
		{"with a window", with(func(r *domain.PatchChangeRequestRequest) { r.PlannedStartOn = sp(rsStart3) }), validation, msgAcceptAlone},
		{"with the hold released", with(func(r *domain.PatchChangeRequestRequest) { r.OnHold = boolp(false) }), validation, msgAcceptAlone},
		{"with the customer's approval", with(func(r *domain.PatchChangeRequestRequest) { r.IsCustomerApproved = boolp(true) }), validation,
			"isCustomerApproved cannot be set on the customer's behalf: the customer's approval can only be given by the customer in the Customer Portal"},
		{"a proposal version that is no date", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedCustomerUpdatedOn = sp("infinity") }), validation,
			"expectedCustomerUpdatedOn must be a date-time as the change request shows it"},
		{"a planned start that is no date", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedStartOn = sp("now") }), validation,
			"expectedPlannedStartOn must be a date-time as the change request shows it"},
		{"another proposal than the waiting one", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedCustomerUpdatedOn = sp(rsStart3) }), conflict,
			"the customer's proposed time changed after you opened this change request (it is now 2030-03-08T09:00:00Z); read it again before responding"},
		{"a planned start that is not the stored one", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedStartOn = sp(rsStart3) }), conflict,
			"the planned implementation time of this change request changed after you opened it (it is now 2030-03-01T09:00:00Z to 2030-03-01T11:00:00Z); read it again before responding"},
		{"a planned end that is not the stored one", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedEndOn = sp(rsEnd3) }), conflict,
			"the planned implementation time of this change request changed after you opened it (it is now 2030-03-01T09:00:00Z to 2030-03-01T11:00:00Z); read it again before responding"},
	} {
		_, err := f.patch(id, tc.req)
		if tc.kind == validation {
			f.wantValidationError(tc.name, err, tc.msg)
		} else {
			f.wantConflictExact(tc.name, err, tc.msg)
		}
		if after := f.snap(id); after != before {
			t.Fatalf("%s: a refused Accept changed the change request:\n  before: %s\n  after:  %s", tc.name, before, after)
		}
	}
	// The window the Accept names is no longer the stored one: the same machine-readable code as a
	// customer's answer for a window that moved (the words are the staff ones).
	_, err0 := f.patch(id, with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedStartOn = sp(rsStart3) }))
	wantRefusalCode(t, "an Accept for a planned window that moved", err0, 409, apierror.CodeChangeRequestScheduleChanged)
	// Exact texts of the validation refusals that are whole sentences.
	f.wantExact("an Accept that is a decline", func() error {
		_, err := f.patch(id, with(func(r *domain.PatchChangeRequestRequest) { r.ConfirmCustomerUpdatedDate = sp("disagree") }))
		return err
	}(), msgAcceptMustBeAgree)

	// A customer cannot answer for WSO2, and a stranger does not learn the change request exists.
	_, err := f.patchAsContact(id, crScopeUserA1, good)
	f.wantForbidden("a customer sending Accept", err, "customers can only record the customer's approval or review")
	_, err = f.patchAsContact(id, crScopeUserB1, good)
	f.wantRefusedAsStranger("a customer of another project sending Accept", err)
	if after := f.snap(id); after != before {
		t.Fatalf("a customer's Accept changed the change request:\n  before: %s\n  after:  %s", before, after)
	}

	// The one that is fine: Scheduled, the proposal is the start, the length is kept, AGREE.
	cr, err := f.patch(id, good)
	if err != nil {
		t.Fatalf("Accept proposed time: %v", err)
	}
	if cr.State == nil || *cr.State != "scheduled" || cr.PlannedStartOn == nil || *cr.PlannedStartOn != rsStart2 || cr.PlannedEndOn == nil || *cr.PlannedEndOn != endFor(rsStart2) {
		t.Fatalf("the receipt = state %v, window %v .. %v", cr.State, cr.PlannedStartOn, cr.PlannedEndOn)
	}
	if cr.ConfirmCustomerUpdatedDate == nil || *cr.ConfirmCustomerUpdatedDate != "agree" || cr.CustomerProposal == nil || cr.CustomerProposal.Answer != "agreed" {
		t.Fatalf("the receipt's answer = %v / %+v, want agree / agreed", cr.ConfirmCustomerUpdatedDate, cr.CustomerProposal)
	}
	assertStates(t, "legalNextStates after Accept", cr.LegalNextStates, "implement", "canceled")
	f.wantConversation(id, "after Accept", rsStart2, "AGREE")
	if cr.HasCustomerApproved {
		t.Fatal("Accept recorded the customer's approval")
	}
	// A second Accept finds nothing waiting: the change is Scheduled now.
	_, err = f.patch(id, good)
	f.wantConflictContaining("a second Accept", err, "a proposed time can only be accepted while the change request is in Customer Approval, but it is in Scheduled")
}

// The refusals that need a change request of their own: the proposal has passed, the change
// is on hold, the planned window has no length to keep. Each leaves it exactly as it was, and
// the read model says why Accept is blocked in the same words.
func TestChangeRequestProposalIntegration_OnHoldAndPast(t *testing.T) {
	// The hold is set the way the data can carry it without a write of ours (the sync, or an
	// engineer's edit that is not what this test is about), so the proposer stays recorded and the
	// refusal order shows: a staff PATCH of the hold is a write like any other and replaces the last
	// writer (the next subtest).
	t.Run("on hold", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		f.execSQL(`UPDATE change_request SET is_on_hold = true WHERE id = $1`, id)
		p := f.proposalOf(id)
		if p == nil || p.CanAccept == nil || *p.CanAccept || p.AcceptBlockedReason == nil || *p.AcceptBlockedReason != "change request is on hold; take it off hold (onHold: false) before changing its state" {
			t.Fatalf("customerProposal while on hold = %+v", p)
		}
		before := f.snap(id)
		_, err := f.accept(id)
		f.wantExact("Accept on hold", err, "change request is on hold; take it off hold (onHold: false) before changing its state")
		f.wantRefusedSame("Accept on hold", id, before, err)
		// A Propose-a-different-time is a state-changing request too, and held to the same gate.
		err = f.counter(id, sp(rsStart3), sp(rsEnd3))
		f.wantValidationError("a different time on hold", err, "change request is on hold")
		// A customer cannot propose on hold either (409).
		_, err = f.proposeAs(id, crScopeUserA2, rsStart3)
		f.wantConflictExact("a proposal on hold", err, "this change request is on hold, so a new implementation time cannot be proposed now")
		f.wantRefusedSame("a different time / a proposal on hold", id, before, err)
		// Released the way it was set, it works: nothing else wrote to the change meanwhile.
		f.execSQL(`UPDATE change_request SET is_on_hold = false WHERE id = $1`, id)
		f.mustAccept(id)
		f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
	})
	for name, withParent := range map[string]bool{"no parent record": false, "under a parent record": true} {
		withParent := withParent
		t.Run("a staff PATCH of the hold replaces the last writer, "+name+": the proposer is no longer recorded", func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			if withParent {
				f.giveParent(id, crScopeProjectA, false)
			}
			f.mustPropose(id, crScopeUserA1, rsStart2)
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{OnHold: boolp(true), OnHoldReason: sp("freeze")}); err != nil {
				t.Fatalf("put on hold: %v", err)
			}
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{OnHold: boolp(false)}); err != nil {
				t.Fatalf("take off hold: %v", err)
			}
			p := f.proposalOf(id)
			if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept ||
				p.AcceptBlockedReason == nil || !strings.HasPrefix(*p.AcceptBlockedReason, msgAcceptNobodyRecorded) {
				t.Fatalf("customerProposal after the hold = %+v, want a pending time whose proposer is no longer recorded", p)
			}
			before := f.snap(id)
			_, err := f.accept(id)
			wantRefusalCode(t, "Accept after a staff edit", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
			f.wantRefusedSame("Accept after a staff edit", id, before, err)
			// What stays possible: asking the customers to approve a time of WSO2's.
			if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
				t.Fatalf("Propose a different time after a staff edit: %v", err)
			}
			f.wantPlanned(id, "after the different time", rsStart3, rsEnd3)
		})
	}
	t.Run("the proposed start has passed", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		// The customer proposed a time that was in the future; it is in the past now.
		f.mustPropose(id, crScopeUserA1, rsStart2)
		past := time.Now().UTC().Add(-time.Hour).Truncate(time.Second).Format(time.RFC3339)
		f.execSQL(`UPDATE change_request SET customer_updated_on = $2::text::timestamptz WHERE id = $1`, id, past)
		p := f.proposalOf(id)
		if p == nil || p.Answer != "pending" || p.CanAccept == nil || *p.CanAccept || p.AcceptBlockedReason == nil {
			t.Fatalf("customerProposal after the date passed = %+v", p)
		}
		want := `the time the customer proposed (` + past + `) has already passed, so it cannot be accepted: use "Propose a different time" to ask the customer to approve another time`
		if *p.AcceptBlockedReason != want {
			t.Fatalf("acceptBlockedReason = %q, want %q", *p.AcceptBlockedReason, want)
		}
		before := f.snap(id)
		_, err := f.accept(id)
		f.wantConflictExact("Accept of a time that has passed", err, want)
		f.wantRefusedSame("Accept of a time that has passed", id, before, err)
		// A different time is still WSO2's to propose.
		if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("a different time after the proposal passed: %v", err)
		}
		f.wantConversation(id, "after the different time", past, "DISAGREE")
	})
	t.Run("the planned window has no length to keep", func(t *testing.T) {
		for name, mod := range map[string]string{
			"no end":       `UPDATE change_request SET end_on = NULL WHERE id = $1`,
			"no start":     `UPDATE change_request SET start_on = NULL WHERE id = $1`,
			"an empty one": `UPDATE change_request SET end_on = start_on WHERE id = $1`,
		} {
			mod := mod
			t.Run(name, func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
				f.customerWroteProposal(id, crScopeUserA1, rsStart2)
				f.execSQL(mod, id)
				cr := f.get(id)
				p := cr.CustomerProposal
				if p == nil || p.Answer != "pending" || p.CanAccept == nil || *p.CanAccept || p.EndOn != nil ||
					p.AcceptBlockedReason == nil || *p.AcceptBlockedReason != msgAcceptNoLength {
					t.Fatalf("customerProposal over a window with no length = %+v", p)
				}
				before := f.snap(id)
				// The page shows whatever the window is; the expectation names it as it reads.
				_, err := f.patch(id, domain.PatchChangeRequestRequest{
					ConfirmCustomerUpdatedDate: sp("agree"), ExpectedCustomerUpdatedOn: sp(p.StartOn),
					ExpectedPlannedStartOn: sp(rsStart1), ExpectedPlannedEndOn: sp(rsEnd1)})
				if err == nil {
					t.Fatal("Accept over a window with no length was accepted")
				}
				f.wantRefusedSame("Accept over a window with no length", id, before, err)
			})
		}
	})
}

// customer_updated_on is a column the previous system writes as well, and nothing there keeps it inside the
// range every planned window is held to. Accept writes the proposal as the planned start and
// start + the planned length as the end, so a date that would put the end past the range is not one
// WSO2 can accept (409, the read model says so in the same words); the last window inside the range
// is, and a different time is still WSO2's to propose.
func TestChangeRequestProposalIntegration_AnAcceptStaysInsideTheRangeOfEveryWindow(t *testing.T) {
	for name, tc := range map[string]struct{ start, end, refusal string }{
		"the end passes the last year by the planned length": {"2100-12-31T23:00:00Z", "2101-01-01T01:00:00Z", ""},
		"a date in the year 9999":                            {"9999-12-31T23:30:00Z", "10000-01-01T01:30:00Z", ""},
		"the last window that fits":                          {"2100-12-31T20:00:00Z", "2100-12-31T22:00:00Z", "fits"},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.customerWroteProposal(id, crScopeUserA1, tc.start)
			p := f.proposalOf(id)
			if p == nil || p.Answer != "pending" || p.CanAccept == nil {
				t.Fatalf("customerProposal = %+v, want a waiting proposal", p)
			}
			if tc.refusal == "fits" {
				if !*p.CanAccept || p.AcceptBlockedReason != nil {
					t.Fatalf("a window ending inside the range is blocked: %+v", p)
				}
				f.mustAccept(id)
				f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
				f.wantPlanned(id, "after Accept", tc.start, tc.end)
				return
			}
			want := "the time the customer proposed (" + tc.start + ") is too far ahead to be accepted: the window would end after the year 2100 (" + tc.end +
				`), so use "Propose a different time" to ask the customer to approve another time`
			if *p.CanAccept || p.AcceptBlockedReason == nil || *p.AcceptBlockedReason != want {
				t.Fatalf("customerProposal = %+v, want Accept blocked with %q", p, want)
			}
			before := f.snap(id)
			_, err := f.accept(id)
			f.wantConflictExact("Accept beyond the range", err, want)
			f.wantRefusedSame("Accept beyond the range", id, before, err)
			// WSO2's own window is the answer that remains.
			if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
				t.Fatalf("a different time after an unacceptable proposal: %v", err)
			}
			f.wantConversation(id, "after the different time", tc.start, "DISAGREE")
		})
	}
}

// The refusal for a window with no length is checked with a window the page can name: the
// expectation must match the stored window, so a window the stored row does not have is the
// stale-window 409 first, and with no end at all the page has nothing to name. The cases
// above settle on "refused, nothing written"; this one pins the exact words where the
// stored window is whole but empty (start == end, which the page shows and can name).
func TestChangeRequestProposalIntegration_AnEmptyWindowHasNoLengthToKeep(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.customerWroteProposal(id, crScopeUserA1, rsStart2)
	f.execSQL(`UPDATE change_request SET end_on = start_on WHERE id = $1`, id)
	req := f.acceptReq(id)
	before := f.snap(id)
	_, err := f.patch(id, req)
	f.wantConflictExact("Accept over an empty window", err, msgAcceptNoLength)
	f.wantRefusedSame("Accept over an empty window", id, before, err)
}

const (
	msgAcceptMustBeAgree = `confirmCustomerUpdatedDate must be "agree": to decline a proposal, propose a different time (state "authorize" with the new planned window)`
	msgAcceptNeedsWindow = "expectedPlannedStartOn and expectedPlannedEndOn are required with confirmCustomerUpdatedDate: they name the planned time the proposal replaces"
	msgAcceptAlone       = "confirmCustomerUpdatedDate cannot be combined with other fields; only expectedCustomerUpdatedOn, expectedPlannedStartOn and expectedPlannedEndOn go with it"
	msgAcceptNoLength    = `the planned window has no length, so the customer's proposed start cannot be applied to it: use "Propose a different time"`
)

// ---------------------------------------------------------------------------
// WSO2 proposes a different time, or declines
// ---------------------------------------------------------------------------

// Propose a different time: a staff {state: "authorize"} with the window WSO2 wants, naming the
// proposal it answers and the planned window the page showed. Every way it can be refused is
// refused with the contract's words and leaves the change request exactly as it was; the one
// that is fine writes DISAGREE and the window, asks the customers again, and goes through no CAB.
func TestChangeRequestProposalIntegration_CounterHappyAndEveryRefusal(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.mustPropose(id, crScopeUserA1, rsStart2)
	stages := f.stageLabels(id)
	before := f.snap(id)
	good := f.counterReq(id, sp(rsStart3), sp(rsEnd3))
	with := func(mod func(*domain.PatchChangeRequestRequest)) domain.PatchChangeRequestRequest {
		r := good
		mod(&r)
		return r
	}
	const planChanged = "the planned implementation time of this change request changed after you opened it (it is now 2030-03-01T09:00:00Z to 2030-03-01T11:00:00Z); read it again before responding"
	missed := "the customer proposed a new time (2030-03-08T09:00:00Z) after you opened this change request; read it again to accept it or propose a different time"
	for _, tc := range []struct {
		name  string
		req   domain.PatchChangeRequestRequest
		check func(what string, err error)
	}{
		{"a Re-schedule that never saw the proposal", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedCustomerUpdatedOn = nil }),
			func(w string, err error) { f.wantConflictExact(w, err, missed) }},
		{"another version of the proposal", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedCustomerUpdatedOn = sp(rsStart3) }),
			func(w string, err error) { f.wantConflictExact(w, err, missed) }},
		{"a planned start that is not the stored one", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedStartOn = sp(rsStart3) }),
			func(w string, err error) { f.wantConflictExact(w, err, planChanged) }},
		{"a planned end that is not the stored one", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedEndOn = sp(rsEnd3) }),
			func(w string, err error) { f.wantConflictExact(w, err, planChanged) }},
		{"the customer's own time, as a window", with(func(r *domain.PatchChangeRequestRequest) { r.PlannedStartOn, r.PlannedEndOn = sp(rsStart2), sp(rsEnd2) }),
			func(w string, err error) { f.wantExact(w, err, msgCounterIsTheProposal) }},
		{"the customer's own start alone: a staff start moves the start only, the end stays", with(func(r *domain.PatchChangeRequestRequest) { r.PlannedStartOn, r.PlannedEndOn = sp(rsStart2), nil }),
			func(w string, err error) {
				f.wantValidationError(w, err, "the planned start must not be after the planned end")
			}},
		{"a window that ends before it starts", with(func(r *domain.PatchChangeRequestRequest) { r.PlannedStartOn, r.PlannedEndOn = sp(rsStart3), sp(rsEnd1) }),
			func(w string, err error) {
				f.wantValidationError(w, err, "the planned start must not be after the planned end")
			}},
		{"a window with no length", with(func(r *domain.PatchChangeRequestRequest) {
			r.PlannedStartOn, r.PlannedEndOn = sp(rsStart3), sp(rsStart3)
		}),
			func(w string, err error) {
				f.wantValidationError(w, err, "the planned start must not be the same as the planned end")
			}},
		{"a start that is no date", with(func(r *domain.PatchChangeRequestRequest) { r.PlannedStartOn = sp("infinity") }),
			func(w string, err error) { f.wantValidationError(w, err, "plannedStartOn must be a valid date-time") }},
		{"a proposal version that is no date", with(func(r *domain.PatchChangeRequestRequest) { r.ExpectedCustomerUpdatedOn = sp("next tuesday") }),
			func(w string, err error) {
				f.wantValidationError(w, err, "expectedCustomerUpdatedOn must be a date-time as the change request shows it")
			}},
	} {
		_, err := f.patch(id, tc.req)
		if err == nil {
			t.Fatalf("%s: accepted, want a refusal", tc.name)
		}
		tc.check(tc.name, err)
		if after := f.snap(id); after != before {
			t.Fatalf("%s: a refused request changed the change request:\n  before: %s\n  after:  %s", tc.name, before, after)
		}
	}

	// Nobody sends a version of a proposal that is not waiting (here: not even one was made).
	other := newCustomerGroupFlow(t)
	oid := other.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	_, err := other.patch(oid, domain.PatchChangeRequestRequest{
		State: stateptr(domain.ChangeRequestStateAuthorize), PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2), ExpectedCustomerUpdatedOn: sp(rsStart3)})
	other.wantConflictExact("a version of a proposal that is not waiting", err, msgProposalNoLongerWaiting)
	f = newCustomerGroupFlow(t)
	id = f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.mustPropose(id, crScopeUserA1, rsStart2)
	stages = f.stageLabels(id)
	good = f.counterReq(id, sp(rsStart3), sp(rsEnd3))

	// The one that is fine.
	cr, err := f.patch(id, good)
	if err != nil {
		t.Fatalf("Propose a different time: %v", err)
	}
	if cr.State == nil || *cr.State != "customer_approval" || cr.PlannedStartOn == nil || *cr.PlannedStartOn != rsStart3 || cr.PlannedEndOn == nil || *cr.PlannedEndOn != rsEnd3 {
		t.Fatalf("the receipt = state %v, window %v .. %v", cr.State, cr.PlannedStartOn, cr.PlannedEndOn)
	}
	if p := cr.CustomerProposal; p == nil || p.Answer != "disagreed" || p.StartOn != rsStart2 || p.EndOn != nil || p.CanAccept != nil {
		t.Fatalf("customerProposal after the counter = %+v, want a disagreed proposal and nothing waiting", p)
	}
	assertStates(t, "legalNextStates after the counter", cr.LegalNextStates, "authorize", "canceled")
	f.wantConversation(id, "after the counter", rsStart2, "DISAGREE")
	if got := f.stageLabels(id); got != stages+",Customer Approval" {
		t.Fatalf("stages after the counter = %s, want %s and one fresh customer stage", got, stages)
	}
	custom := f.customerStages(id)
	assertApprovers(t, "the customers' superseded request", custom[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
	assertApprovers(t, "the customers asked again", custom[1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	// A second answer to the same proposal is a plain Re-schedule now (the answer stands).
	if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
		t.Fatalf("a Re-schedule after the counter: %v", err)
	}
	f.wantPlanned(id, "after the Re-schedule", rsStart2, rsEnd2)
	f.wantConversation(id, "after the Re-schedule", rsStart2, "DISAGREE")
}

// Decline: the previous system's Disagree with the plan left as it is. WSO2 need not invent a time just to
// say no: only the answer is written, the customers keep their live request, and nobody has to
// be found to ask. Re-stating the planned window is the same decline ("keep our time").
func TestChangeRequestProposalIntegration_DeclineKeepsTheWindow(t *testing.T) {
	for name, window := range map[string][2]*string{
		"no window sent":                  {nil, nil},
		"the planned window restated":     {sp(rsStart1), sp(rsEnd1)},
		"the planned start restated":      {sp(rsStart1), nil},
		"the same instants, another zone": {sp("2030-03-01T14:30:00+05:30"), sp("2030-03-01T16:30:00+05:30")},
	} {
		window := window
		t.Run(name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.mustPropose(id, crScopeUserA2, rsStart2)
			stages := f.stageLabels(id)
			// Nobody can be asked any more: a decline needs nobody (the proposer is the requester and
			// the only registered contact left, so the proposal is still recorded as theirs).
			restore := f.onlyTheProposerIsLeft(id, crScopeUserA2)

			if err := f.counter(id, window[0], window[1]); err != nil {
				t.Fatalf("decline: %v", err)
			}
			f.expect(id, "after the decline", "CUSTOMER_APPROVAL", "authorize", "canceled")
			f.wantPlanned(id, "after the decline", rsStart1, rsEnd1)
			f.wantConversation(id, "after the decline", rsStart2, "DISAGREE")
			f.wantAnswer(id, "after the decline", "disagreed")
			if got := f.stageLabels(id); got != stages {
				t.Fatalf("a decline changed the stages: %s, was %s", got, stages)
			}
			assertApprovers(t, "the customers' request after a decline", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
			restore()
			f.wantCanAnswer(id, "after the decline", true, crScopeUserA1, crScopeUserA2)
			// The customer is not told they can propose the time WSO2 declined again.
			_, err := f.proposeAs(id, crScopeUserA1, rsStart2)
			f.wantExact("proposing the declined time again", err, msgProposalWSO2AskedOther)
			// ...but may propose another.
			f.mustPropose(id, crScopeUserA1, rsStart3)
			f.wantAnswer(id, "after a new proposal", "pending")
		})
	}
}

const (
	msgCounterIsTheProposal    = `the time you are proposing is the one the customer proposed: use "Accept proposed time" instead`
	msgProposalNoLongerWaiting = "the customer's proposed time is no longer waiting for a response; read the change request again"
	msgProposalWSO2AskedOther  = "WSO2 asked for a different time than that one: propose another start"
	msgProposalAlreadyProposed = "that time is already proposed and is waiting for WSO2's response"
)

// Nobody to ask: a counter that changes the window asks the customers again, so with nobody
// to ask it is refused with Request Approval's words (and nothing is written); the proposal
// stays waiting, Accept still works, a decline still works.
func TestChangeRequestProposalIntegration_CounterNeedsSomebodyToAsk(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.mustPropose(id, crScopeUserA1, rsStart2)
	// Nobody can be asked (the proposer is the requester and the only registered contact left), yet
	// the proposer is still a registered contact: the proposal is still recorded as theirs.
	f.onlyTheProposerIsLeft(id, crScopeUserA1)
	before := f.snap(id)
	for i := 0; i < 2; i++ {
		err := f.counter(id, sp(rsStart3), sp(rsEnd3))
		f.wantExact("a counter with nobody to ask", err, nobodyMsgApproval)
		f.wantRefusedSame("a counter with nobody to ask", id, before, err)
	}
	f.wantAnswer(id, "after the refused counters", "pending")
	// Accept asks nobody: it works.
	f.mustAccept(id)
	f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
}

// ---------------------------------------------------------------------------
// No additional entries
// ---------------------------------------------------------------------------

// The feature adds NOTHING to the database but what each act has to write: no comment, activity
// or marker row of its own, no column, table or type. Every act is counted over EVERY table
// before and after, and the delta must be exactly what the contract says -- the rows that
// the previous system's own mechanism already produces (the event outbox row of any change_request update;
// the plan-start-date comment on the parent record and the GitHub queue row it enqueues) and the
// one customer stage a re-ask provisions, as a Re-schedule always did.
func TestChangeRequestProposalIntegration_NoExtraRows(t *testing.T) {
	measure := func(t *testing.T, name string, linkedParent *bool, setup func(f *crFlow) string, run func(f *crFlow, id string), want map[string]int) {
		t.Run(name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := setup(f)
			if linkedParent != nil {
				f.giveParent(id, crScopeProjectA, *linkedParent)
			}
			before := f.rowCounts()
			run(f, id)
			wantDelta(t, name, before, f.rowCounts(), want)
		})
	}
	inCA := func(f *crFlow) string { return f.reachCustomerApproval(domain.ChangeRequestTypeNormal) }
	proposed := func(f *crFlow) string {
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		return id
	}
	yes, no := true, false

	// A customer proposes: one event_outbox row (the update of change_request) and, under a
	// parent, the plan-start-date comment of trigger 0053 -- and, linked to GitHub, the queue row
	// the comment enqueues. No stage, approver, activity, comment of ours.
	measure(t, "a customer proposes, no parent", nil, inCA,
		func(f *crFlow, id string) { f.mustPropose(id, crScopeUserA1, rsStart2) }, map[string]int{"event_outbox": 1})
	measure(t, "a customer proposes, under a parent record", &no, inCA,
		func(f *crFlow, id string) { f.mustPropose(id, crScopeUserA1, rsStart2) }, map[string]int{"event_outbox": 1, "comment": 1})
	measure(t, "a customer proposes, parent linked to a GitHub issue", &yes, inCA,
		func(f *crFlow, id string) { f.mustPropose(id, crScopeUserA1, rsStart2) }, map[string]int{"event_outbox": 1, "comment": 1, "github_outbound_queue": 1})
	// ...and a proposal re-stated by the other colleague moves the date again: one comment more.
	measure(t, "a second proposal, under a parent", &no, proposed,
		func(f *crFlow, id string) { f.mustPropose(id, crScopeUserA2, rsStart3) }, map[string]int{"event_outbox": 1, "comment": 1})

	// WSO2 accepts: the outbox row, and the GitHub queue row when linked (state and dates moved).
	// No comment: customer_updated_on does not change, and the change leaves Customer Approval.
	measure(t, "Accept, no parent", nil, proposed,
		func(f *crFlow, id string) { f.mustAccept(id) }, map[string]int{"event_outbox": 1})
	measure(t, "Accept, parent linked to a GitHub issue", &yes, proposed,
		func(f *crFlow, id string) { f.mustAccept(id) }, map[string]int{"event_outbox": 1, "github_outbound_queue": 1})

	// A different time: the outbox row, the customers' fresh request (one stage, one row per
	// contact), the GitHub queue row when linked (the dates moved).
	measure(t, "a different time, no parent", nil, proposed,
		func(f *crFlow, id string) {
			if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
				t.Fatalf("a different time: %v", err)
			}
		}, map[string]int{"event_outbox": 1, "approval_stage": 1, "approval_stage_approver": 2})
	measure(t, "a different time, parent linked to a GitHub issue", &yes, proposed,
		func(f *crFlow, id string) {
			if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
				t.Fatalf("a different time: %v", err)
			}
		}, map[string]int{"event_outbox": 1, "approval_stage": 1, "approval_stage_approver": 2, "github_outbound_queue": 1})

	// A decline: the answer, nothing else (the plan did not move, the customers keep their request).
	measure(t, "a decline", &yes, proposed,
		func(f *crFlow, id string) {
			if err := f.counter(id, nil, nil); err != nil {
				t.Fatalf("a decline: %v", err)
			}
		}, map[string]int{"event_outbox": 1})

	// A plain Re-schedule: the same re-ask as ever, no CAB stage.
	measure(t, "a plain Re-schedule", &no, inCA,
		func(f *crFlow, id string) {
			if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
				t.Fatalf("Re-schedule: %v", err)
			}
		}, map[string]int{"event_outbox": 1, "approval_stage": 1, "approval_stage_approver": 2})

	// A refused act writes nothing at all.
	measure(t, "a refused proposal, a refused Accept, a refused counter", &yes, inCA,
		func(f *crFlow, id string) {
			if _, err := f.proposeAs(id, crScopeUserA1, rsStart1); err == nil {
				t.Fatal("the planned start was accepted as a proposal")
			}
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{ConfirmCustomerUpdatedDate: sp("agree"), ExpectedCustomerUpdatedOn: sp(rsStart2),
				ExpectedPlannedStartOn: sp(rsStart1), ExpectedPlannedEndOn: sp(rsEnd1)}); err == nil {
				t.Fatal("Accept with nothing waiting was accepted")
			}
			if err := f.counter(id, sp(rsStart1), sp(rsEnd1)); err == nil {
				t.Fatal("a Re-schedule to the stored window was accepted")
			}
		}, map[string]int{})

	// A change the previous system already asked (migrated shape: an unlabeled customer-group stage with
	// REQUESTED rows): the customer's first act gives it the labelled stage it lacks, one row per
	// contact -- the existing behaviour of every customer act on such a row, pinned
	// (change_request_synced_stages_integration_test.go) -- and writes the proposal. That is the
	// whole delta. WSO2's Accept then adds nothing and leaves the previous system's rows alone.
	t.Run("a migrated change: the customer's proposal, then Accept", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		before := f.rowCounts()
		f.mustPropose(id, crScopeUserA1, rsStart2)
		afterProposal := f.rowCounts()
		wantDelta(t, "the first customer act on a migrated change", before, afterProposal,
			map[string]int{"event_outbox": 1, "approval_stage": 1, "approval_stage_approver": 2})
		f.mustAccept(id)
		wantDelta(t, "Accept on a migrated change", afterProposal, f.rowCounts(), map[string]int{"event_outbox": 1})
	})
}

// ---------------------------------------------------------------------------
// The parity trigger runs as the customer's own transaction
// ---------------------------------------------------------------------------

// Trigger 0053 writes the plan-start-date comment on the parent record in the customer's own
// transaction. Under row-level security (the csm_app role this suite is also run as) the
// customer's identity may not write a comment on a parent of another project; the proposal
// runs its write as the system once the customer's access to the change request is proven (the
// work_item write that opens the transaction, exactly as the customer-stage provisioning does),
// so the comment is written whichever project the parent is in, as the customer, once.
func TestChangeRequestProposalIntegration_TriggerCommentUnderRLS(t *testing.T) {
	for name, project := range map[string]string{
		"a parent in the same project":           crScopeProjectA,
		"a parent in another project":            crScopeProjectB,
		"a parent in a project without contacts": crScopeProjectC,
	} {
		project := project
		t.Run(name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			parent := f.giveParent(id, project, false)
			f.mustPropose(id, crScopeUserA1, rsStart2)
			got := f.commentsOn(parent)
			want := crFlowEmail(crScopeUserA1) + ": Plan start date change update from (not set) to 2030-03-08 09:00"
			if len(got) != 1 || got[0] != want {
				t.Fatalf("comments on the parent = %q, want exactly [%q]", got, want)
			}
			// The proposer is the author; a second proposal comments again with the old date.
			f.mustPropose(id, crScopeUserA2, rsStart3)
			got = f.commentsOn(parent)
			if len(got) != 2 || got[1] != crFlowEmail(crScopeUserA2)+": Plan start date change update from 2030-03-08 09:00 to 2030-03-15 09:00" {
				t.Fatalf("comments on the parent after a second proposal = %q", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The conversation repeats
// ---------------------------------------------------------------------------

// Propose, counter, propose, decline, propose, accept: every turn writes what the contract says,
// the answer is cleared by each new proposal, each re-ask adds one customer stage, a decline
// adds none, and the planned length is the one WSO2's last window set.
func TestChangeRequestProposalIntegration_RepeatedCycles(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	base := f.stageLabels(id)

	// 1. A1 proposes T1.
	f.mustPropose(id, crScopeUserA1, rsStart2)
	f.wantConversation(id, "after T1", rsStart2, "")
	f.wantAnswer(id, "after T1", "pending")
	// 2. WSO2 proposes W2: DISAGREE, the plan is W2, the customers are asked again.
	if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
		t.Fatalf("W2: %v", err)
	}
	f.wantConversation(id, "after W2", rsStart2, "DISAGREE")
	f.wantPlanned(id, "after W2", rsStart3, rsEnd3)
	f.wantAnswer(id, "after W2", "disagreed")
	if got := f.stageLabels(id); got != base+",Customer Approval" {
		t.Fatalf("stages after W2 = %s", got)
	}
	// 3. A2 proposes T2: the answer is cleared, the live request stays (and is theirs to answer).
	f.mustPropose(id, crScopeUserA2, rsStartEarly)
	f.wantConversation(id, "after T2", rsStartEarly, "")
	f.wantAnswer(id, "after T2", "pending")
	f.wantCanAnswer(id, "while T2 waits", true, crScopeUserA1, crScopeUserA2)
	if got := f.stageLabels(id); got != base+",Customer Approval" {
		t.Fatalf("stages after T2 = %s (a proposal adds none)", got)
	}
	// ...T2 equal to the standing proposal is refused, not silently ignored.
	_, err := f.proposeAs(id, crScopeUserA1, rsStartEarly)
	f.wantExact("T2 again", err, msgProposalAlreadyProposed)
	// 4. WSO2 declines T2 (plan kept).
	if err := f.counter(id, nil, nil); err != nil {
		t.Fatalf("decline T2: %v", err)
	}
	f.wantConversation(id, "after the decline", rsStartEarly, "DISAGREE")
	f.wantPlanned(id, "after the decline", rsStart3, rsEnd3)
	if got := f.stageLabels(id); got != base+",Customer Approval" {
		t.Fatalf("stages after the decline = %s (a decline adds none)", got)
	}
	// 5. A1 proposes T3 (the time of T1, which is not the standing one any more).
	f.mustPropose(id, crScopeUserA1, rsStart2)
	f.wantConversation(id, "after T3", rsStart2, "")
	p := f.proposalOf(id)
	if p == nil || p.Answer != "pending" || p.StartOn != rsStart2 || p.EndOn == nil || *p.EndOn != endFor(rsStart2) {
		t.Fatalf("customerProposal after T3 = %+v", p)
	}
	// 6. WSO2 accepts T3: the window is T3 with W2's length; no CAB, no further stage.
	f.mustAccept(id)
	f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
	f.wantPlanned(id, "after Accept", rsStart2, endFor(rsStart2))
	f.wantConversation(id, "after Accept", rsStart2, "AGREE")
	if got := f.stageLabels(id); got != base+",Customer Approval" {
		t.Fatalf("stages after Accept = %s", got)
	}
	assertApprovers(t, "the customers' last request", f.customerStages(id)[1].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
	if a, _ := f.customerOutcome(id); a {
		t.Fatal("the customer's approval was recorded by a conversation they never answered")
	}
}

// A change scheduled by Accept goes on like any other: Implement, then Review (its stage is
// provisioned although Customer Approval ran twice), then the customer's own review (the customer
// review box is ours to keep, whatever the proposal conversation did), then Closed. The customer's
// approval was never recorded by the conversation, and the customer's review is recorded by them.
func TestChangeRequestProposalIntegration_AnAcceptedChangeGoesOnToItsClose(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	f.mustPropose(id, crScopeUserA1, rsStart2)
	if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
		t.Fatalf("a different time: %v", err)
	}
	f.mustPropose(id, crScopeUserA2, rsStartEarly)
	f.mustAccept(id)
	f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
	f.wantPlanned(id, "after Accept", rsStartEarly, endFor(rsStartEarly))

	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval,Customer Approval,Review" {
		t.Fatalf("stages in Review = %s", got)
	}
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
		t.Fatalf("the customer's review: %v", err)
	}
	f.expect(id, "in Closed", "CLOSED")
	if approved, reviewed := f.customerOutcome(id); approved || !reviewed {
		t.Fatalf("customer outcome after the close = approved %v, reviewed %v: the customer's approval was never given, their review was", approved, reviewed)
	}
}

// ---------------------------------------------------------------------------
// Who may answer, and who may not
// ---------------------------------------------------------------------------

// The requester may accept (accepting a customer's scheduling preference authorizes nothing: the
// CAB already approved, and the requester is usually the person working it with the customer),
// and so may any other staff member; an external caller may not, nor may a customer of another
// project learn the change request is there.
func TestChangeRequestProposalIntegration_CreatorAndOtherStaffMayAnswerCustomersMayNot(t *testing.T) {
	f := newCustomerGroupFlow(t)
	creator := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	other := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.mustPropose(creator, crScopeUserA1, rsStart2)
	f.mustPropose(other, crScopeUserA1, rsStart2)

	// The requester (the creator the harness stamps on every change) accepts.
	if _, err := f.staffPatch(creator, crFlowCreatorID, f.acceptReq(creator)); err != nil {
		t.Fatalf("the requester accepting: %v", err)
	}
	f.expect(creator, "after the requester accepted", "SCHEDULED", "implement", "canceled")
	// Another staff member (a peer approver, not the creator) accepts the other.
	if _, err := f.staffPatch(other, crFlowPeerAID, f.acceptReq(other)); err != nil {
		t.Fatalf("another staff member accepting: %v", err)
	}
	f.expect(other, "after another staff member accepted", "SCHEDULED", "implement", "canceled")
	if got := f.updatedBy(other); got != crFlowEmail(crFlowPeerAID) {
		t.Fatalf("work_item.updated_by after Accept = %q, want the member who accepted", got)
	}
	// What a customer sends is refused (the whitelist), however it is spelled.
	fresh := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.mustPropose(fresh, crScopeUserA1, rsStart2)
	for name, req := range map[string]domain.PatchChangeRequestRequest{
		"Accept":                f.acceptReq(fresh),
		"a different time":      f.counterReq(fresh, sp(rsStart3), sp(rsEnd3)),
		"a decline":             f.counterReq(fresh, nil, nil),
		"the version alone":     {ExpectedCustomerUpdatedOn: sp(rsStart2)},
		"agree beside a window": {ConfirmCustomerUpdatedDate: sp("agree"), PlannedStartOn: sp(rsStart3)},
	} {
		_, err := f.patchAsContact(fresh, crScopeUserA2, req)
		f.wantForbidden("a customer sending "+name, err, "customers can only record the customer's approval or review")
	}
	f.expect(fresh, "after the customer's refused answers", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantAnswer(fresh, "after the customer's refused answers", "pending")
}

// ---------------------------------------------------------------------------
// A customer's proposal: what else is refused
// ---------------------------------------------------------------------------

func TestChangeRequestProposalIntegration_CustomerRefusals(t *testing.T) {
	t.Run("another approval is still being asked", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		// A CAB-group approval is still requested although the change reads Customer Approval
		// (the data can say so); the customer's proposal could never be answered, so it is
		// refused rather than written.
		cab := crCABGroupID
		f.seedMigratedStage(id, &cab, 20, map[string]string{crCABMemberUserID1: "REQUESTED"})
		before := f.snap(id)
		_, err := f.proposeAs(id, crScopeUserA1, rsStart2)
		f.wantConflictExact("a proposal while an approval that is not the customer's is asked", err,
			"this change request is also waiting for an approval that is not the customer's, so a new time cannot be proposed for it right now")
		wantRefusalCode(t, "a proposal while another approval is asked", err, 409, apierror.CodeChangeRequestProposalNotNow)
		// The first act on a migrated change gives it its labelled stage before this refusal: that
		// existing provisioning is rolled back with the refused transaction.
		f.wantRefusedSame("a proposal while another approval is asked", id, before, err)
	})
	t.Run("the change has no planned window to move", func(t *testing.T) {
		for name, sql := range map[string]string{
			"no start":    `UPDATE change_request SET start_on = NULL WHERE id = $1`,
			"no end":      `UPDATE change_request SET end_on = NULL WHERE id = $1`,
			"an empty":    `UPDATE change_request SET end_on = start_on WHERE id = $1`,
			"an inverted": `UPDATE change_request SET end_on = start_on - interval '1 hour' WHERE id = $1`,
		} {
			sql := sql
			t.Run(name, func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
				f.execSQL(sql, id)
				before := f.snap(id)
				_, err := f.proposeAs(id, crScopeUserA1, rsStart2)
				f.wantConflictExact("a proposal over a window with no length", err, "this change request has no planned window to move, so a new time cannot be proposed for it")
				wantRefusalCode(t, name+": a proposal over a window with no length", err, 409, apierror.CodeChangeRequestNoPlannedWindow)
				f.wantRefusedSame("a proposal over a window with no length", id, before, err)
			})
		}
	})
	t.Run("the proposed window would end beyond the year 2100", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		// A stored plan 90 years long (a hand edit or a sync can leave one): the end it would give
		// a proposal is held to the range every window is.
		f.execSQL(`UPDATE change_request SET end_on = start_on + interval '90 years' WHERE id = $1`, id)
		before := f.snap(id)
		_, err := f.proposeAs(id, crScopeUserA1, rsStart2)
		f.wantValidationError("a proposal whose end would pass 2100", err, "is too far ahead")
		f.wantRefusedSame("a proposal whose end would pass 2100", id, before, err)
	})
	t.Run("the same time twice", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		before := f.snap(id)
		_, err := f.proposeAs(id, crScopeUserA2, rsStart2)
		f.wantExact("a colleague proposing the same time", err, msgProposalAlreadyProposed)
		_, err = f.patchAsContact(id, crScopeUserA2, domain.PatchChangeRequestRequest{PlannedStartOn: sp("2030-03-08T14:30:00+05:30"), PlannedEndOn: sp("2030-03-08T16:30:00+05:30")})
		f.wantExact("the same instant in another zone", err, msgProposalAlreadyProposed)
		f.wantRefusedSame("the same time twice", id, before, err)
	})
	t.Run("a customer who was not asked, one whose request was superseded", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.execSQL(`UPDATE approval_stage_approver SET state = 'CANCELLED' WHERE work_item_id = $1 AND approver_user_id = $2::uuid`, id, crScopeUserA2)
		before := f.snap(id)
		_, err := f.proposeAs(id, crScopeUserA2, rsStart2)
		f.wantForbidden("a contact whose row is cancelled", err, "who have been asked for the customer's approval")
		f.wantRefusedSame("a contact whose row is cancelled", id, before, err)
	})
}

// A migrated change in Customer Approval: WSO2 answers a proposal the sync wrote (or a customer
// made) exactly as it answers any other. Accept leaves the previous system's own REQUESTED rows alone
// (an unlabeled stage is of unknown kind and never cancelled by a guess), writes no stage, and
// the proposal that a customer's first act made waits like any other.
func TestChangeRequestProposalIntegration_AMigratedChangeIsAnsweredLikeAnyOther(t *testing.T) {
	t.Run("a date the sync wrote is not a customer's proposal: Accept is refused, nothing is written", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		future := time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Second).Format(time.RFC3339)
		f.syncWritesConversation(id, sp(future), "")
		p := f.proposalOf(id)
		if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept {
			t.Fatalf("customerProposal for a date the sync wrote = %+v, want it pending, no proposer recorded, Accept blocked", p)
		}
		before := f.snap(id)
		_, err := f.accept(id)
		wantRefusalCode(t, "Accept of a date the sync wrote", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
		f.wantRefusedSame("Accept of a date the sync wrote", id, before, err)
	})
	t.Run("Accept leaves the previous system's rows alone", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		future := time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Second).Format(time.RFC3339)
		// A customer's own proposal, as the sync leaves it: the contact is the last writer.
		f.customerWroteProposal(id, crScopeUserA1, future)
		f.wantAnswer(id, "a migrated proposal", "pending")
		stages := f.stages(id)
		f.mustAccept(id)
		f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
		f.wantPlanned(id, "after Accept", future, endFor(future))
		f.wantConversation(id, "after Accept", future, "AGREE")
		after := f.stages(id)
		if len(after) != len(stages) {
			t.Fatalf("Accept wrote a stage: %d -> %d", len(stages), len(after))
		}
		assertApprovers(t, "the previous system's rows after Accept", after[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
		if a, _ := f.customerOutcome(id); a {
			t.Fatal("Accept recorded the customer's approval")
		}
		var synced *bool
		if err := f.scoped.QueryRow(f.sys, `SELECT is_customer_approval_required FROM change_request WHERE id = $1`, id).Scan(&synced); err != nil || synced == nil || *synced {
			t.Fatalf("the sync-owned is_customer_approval_required = %v (%v), want it untouched (false)", synced, err)
		}
	})
	t.Run("a customer's first act on it, then a different time", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		f.mustPropose(id, crScopeUserA1, rsStart2)
		f.wantAnswer(id, "after the customer's proposal", "pending")
		// The labelled stage the first customer act gave it, next to the previous system's.
		if got := f.stageLabels(id); got != ",Customer Approval" {
			t.Fatalf("stages after the customer's proposal = %q, want the previous system's unlabeled one and the labelled stage", got)
		}
		if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("a different time on a migrated change: %v", err)
		}
		f.wantConversation(id, "after the different time", rsStart2, "DISAGREE")
		f.wantPlanned(id, "after the different time", rsStart3, rsEnd3)
		if got := f.stageLabels(id); got != ",Customer Approval,Customer Approval" {
			t.Fatalf("stages after the different time = %q", got)
		}
		stages := f.stages(id)
		assertApprovers(t, "the previous system's rows are not ours to cancel", stages[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
		assertApprovers(t, "the superseded labelled request", stages[1].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
		assertApprovers(t, "the customers asked again", stages[2].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	})
}

// ---------------------------------------------------------------------------
// Two requests at once
// ---------------------------------------------------------------------------

// raceBoth runs a and b at the same moment (a barrier releases both) and returns their errors.
func raceBoth(a, b func() error) (errA, errB error) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)
	go func() { defer wg.Done(); <-start; errA = a() }()
	go func() { defer wg.Done(); <-start; errB = b() }()
	close(start)
	wg.Wait()
	return errA, errB
}

// Every race below has exactly ONE winner whichever request commits first, the loser is told
// 409 or 400 (never a 500) and wrote nothing, and what is stored is consistent with the winner:
// the second request to take the row lock reads what the first committed. The rounds repeat so
// that both orders are met.
func TestChangeRequestProposalIntegration_RacesHaveOneWinner(t *testing.T) {
	isRefusal := func(err error) bool {
		var ce *apierror.ConflictError
		var ve *apierror.ValidationError
		return errors.As(err, &ce) || errors.As(err, &ve)
	}
	const rounds = 5
	t.Run("two Accepts at once", func(t *testing.T) {
		for round := 0; round < rounds; round++ {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.mustPropose(id, crScopeUserA1, rsStart2)
			req := f.acceptReq(id)
			before := f.rowCounts()
			a, b := raceBoth(
				func() error { _, err := f.staffPatch(id, crFlowCreatorID, req); return err },
				func() error { _, err := f.staffPatch(id, crFlowPeerAID, req); return err })
			if (a == nil) == (b == nil) {
				t.Fatalf("round %d: Accept errors = %v / %v, want exactly one winner", round, a, b)
			}
			if loser := map[bool]error{true: b, false: a}[a == nil]; !isRefusal(loser) {
				t.Fatalf("round %d: the losing Accept = %v (%T), want a 409 or 400", round, loser, loser)
			}
			f.expect(id, "after the race", "SCHEDULED", "implement", "canceled")
			f.wantPlanned(id, "after the race", rsStart2, endFor(rsStart2))
			f.wantConversation(id, "after the race", rsStart2, "AGREE")
			wantDelta(t, "two Accepts", before, f.rowCounts(), map[string]int{"event_outbox": 1})
		}
	})
	t.Run("Accept against the customer's new proposal", func(t *testing.T) {
		acceptWon, proposalWon := 0, 0
		for round := 0; round < rounds*2; round++ {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.mustPropose(id, crScopeUserA1, rsStart2)
			req := f.acceptReq(id)
			a, b := raceBoth(
				func() error { _, err := f.staffPatch(id, crFlowCreatorID, req); return err },
				func() error { _, err := f.proposeAs(id, crScopeUserA2, rsStart3); return err })
			if (a == nil) == (b == nil) {
				t.Fatalf("round %d: Accept / proposal errors = %v / %v, want exactly one winner", round, a, b)
			}
			if a == nil {
				acceptWon++
				if !isRefusal(b) {
					t.Fatalf("round %d: the losing proposal = %v (%T)", round, b, b)
				}
				f.expect(id, "after Accept won", "SCHEDULED", "implement", "canceled")
				f.wantPlanned(id, "after Accept won", rsStart2, endFor(rsStart2))
				f.wantConversation(id, "after Accept won", rsStart2, "AGREE")
			} else {
				proposalWon++
				if !isRefusal(a) {
					t.Fatalf("round %d: the losing Accept = %v (%T)", round, a, a)
				}
				f.expect(id, "after the proposal won", "CUSTOMER_APPROVAL", "authorize", "canceled")
				f.wantPlanned(id, "after the proposal won", rsStart1, rsEnd1)
				f.wantConversation(id, "after the proposal won", rsStart3, "")
			}
		}
		t.Logf("Accept won %d, the proposal won %d", acceptWon, proposalWon)
	})
	t.Run("Accept against a different time", func(t *testing.T) {
		for round := 0; round < rounds; round++ {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.mustPropose(id, crScopeUserA1, rsStart2)
			acceptReq := f.acceptReq(id)
			counterReq := f.counterReq(id, sp(rsStart3), sp(rsEnd3))
			a, b := raceBoth(
				func() error { _, err := f.staffPatch(id, crFlowCreatorID, acceptReq); return err },
				func() error { _, err := f.staffPatch(id, crFlowPeerAID, counterReq); return err })
			if (a == nil) == (b == nil) {
				t.Fatalf("round %d: Accept / counter errors = %v / %v, want exactly one winner", round, a, b)
			}
			if a == nil {
				if !isRefusal(b) {
					t.Fatalf("round %d: the losing counter = %v (%T)", round, b, b)
				}
				f.expect(id, "after Accept won", "SCHEDULED", "implement", "canceled")
				f.wantConversation(id, "after Accept won", rsStart2, "AGREE")
				if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval" {
					t.Fatalf("round %d: stages after Accept won = %s", round, got)
				}
			} else {
				if !isRefusal(a) {
					t.Fatalf("round %d: the losing Accept = %v (%T)", round, a, a)
				}
				f.expect(id, "after the counter won", "CUSTOMER_APPROVAL", "authorize", "canceled")
				f.wantConversation(id, "after the counter won", rsStart2, "DISAGREE")
				f.wantPlanned(id, "after the counter won", rsStart3, rsEnd3)
			}
		}
	})
	t.Run("a colleague's approval against Accept", func(t *testing.T) {
		for round := 0; round < rounds*2; round++ {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.mustPropose(id, crScopeUserA1, rsStart2)
			req := f.acceptReq(id)
			a, b := raceBoth(
				func() error { _, err := f.staffPatch(id, crFlowCreatorID, req); return err },
				func() error { _, err := f.approveAs(id, crScopeUserA2, true); return err })
			if (a == nil) == (b == nil) {
				t.Fatalf("round %d: Accept / approval errors = %v / %v, want exactly one winner", round, a, b)
			}
			f.expect(id, "after the race", "SCHEDULED", "implement", "canceled")
			approved, _ := f.customerOutcome(id)
			if a == nil {
				// WSO2 accepted: the proposal is the plan, nobody approved.
				if approved || !isRefusal(b) {
					t.Fatalf("round %d: Accept won but approved=%v, the approval's error = %v", round, approved, b)
				}
				f.wantPlanned(id, "after Accept won", rsStart2, endFor(rsStart2))
				f.wantConversation(id, "after Accept won", rsStart2, "AGREE")
			} else {
				// The colleague approved the plan they saw: it stands, with the customer's approval recorded.
				if !approved || !isRefusal(a) {
					t.Fatalf("round %d: the approval won but approved=%v, Accept's error = %v", round, approved, a)
				}
				f.wantPlanned(id, "after the approval won", rsStart1, rsEnd1)
				f.wantConversation(id, "after the approval won", rsStart2, "")
			}
		}
	})
	t.Run("two customers propose at once", func(t *testing.T) {
		for round := 0; round < rounds; round++ {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			a, b := raceBoth(
				func() error { _, err := f.proposeAs(id, crScopeUserA1, rsStart2); return err },
				func() error { _, err := f.proposeAs(id, crScopeUserA2, rsStart3); return err })
			if a != nil || b != nil {
				t.Fatalf("round %d: two different proposals at once = %v / %v, want both recorded in turn", round, a, b)
			}
			on, conf := f.conversation(id)
			if (on != rsStart2 && on != rsStart3) || conf != "" {
				t.Fatalf("round %d: stored proposal %q / %q", round, on, conf)
			}
			f.wantPlanned(id, "after the proposals", rsStart1, rsEnd1)
		}
	})
	t.Run("two different times at once", func(t *testing.T) {
		for round := 0; round < rounds; round++ {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.mustPropose(id, crScopeUserA1, rsStart2)
			one := f.counterReq(id, sp(rsStart3), sp(rsEnd3))
			two := f.counterReq(id, sp(rsStartEarly), sp(endFor(rsStartEarly)))
			a, b := raceBoth(
				func() error { _, err := f.staffPatch(id, crFlowCreatorID, one); return err },
				func() error { _, err := f.staffPatch(id, crFlowPeerAID, two); return err })
			if (a == nil) == (b == nil) {
				t.Fatalf("round %d: two different times = %v / %v, want exactly one winner (the other sees the answer written)", round, a, b)
			}
			f.wantConversation(id, "after the race", rsStart2, "DISAGREE")
			if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval,Customer Approval" {
				t.Fatalf("round %d: stages = %s, want exactly one fresh customer stage", round, got)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// The outbox rows the notices read
// ---------------------------------------------------------------------------

// The plan-date notices (service.crPlanDateTurnOf over event_outbox) read three things from a
// change_request row change: a moved customer_updated_on while the snapshot is in CUSTOMER_APPROVAL
// (the customer proposed -- read FIRST, because the 0052 trigger clears the answer in the same write),
// else a confirmation that changed to AGREE (WSO2 accepted) or DISAGREE (WSO2 answered with another
// time OR declined: both write DISAGREE, so both are the one notice the previous system's Disagree sends --
// service.TestPlanDate_ADeclineSendsTheSameNoticeAsADifferentTime). The real outbox rows of a real
// proposal, different time, decline and Accept carry exactly that, so the existing notices turn
// without a new notice kind.
func TestChangeRequestProposalIntegration_OutboxRowsCarryTheTurnsOfTheNotices(t *testing.T) {
	last := func(f *crFlow, id string) (changes map[string]map[string]any, snapshot map[string]any) {
		t.Helper()
		var rawChanges, rawSnapshot []byte
		if err := f.scoped.QueryRow(f.sys,
			`SELECT changes::text::bytea, snapshot::text::bytea FROM event_outbox WHERE entity_type = 'change_request' AND entity_id = $1 ORDER BY id DESC LIMIT 1`,
			id).Scan(&rawChanges, &rawSnapshot); err != nil {
			t.Fatalf("read the last outbox row: %v", err)
		}
		if err := json.Unmarshal(rawChanges, &changes); err != nil {
			t.Fatalf("decode changes: %v", err)
		}
		if err := json.Unmarshal(rawSnapshot, &snapshot); err != nil {
			t.Fatalf("decode snapshot: %v", err)
		}
		return changes, snapshot
	}
	to := func(changes map[string]map[string]any, col string) (string, bool) {
		d, ok := changes[col]
		if !ok {
			return "", false
		}
		v, _ := d["to"].(string)
		return v, true
	}

	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)

	// A customer proposes: the date moved, in Customer Approval -> "the customer proposed".
	f.mustPropose(id, crScopeUserA1, rsStart2)
	ch, snap := last(f, id)
	if _, moved := to(ch, "customer_updated_on"); !moved || snap["state"] != "CUSTOMER_APPROVAL" {
		t.Fatalf("a proposal's outbox row = %v / state %v, want customer_updated_on changed in CUSTOMER_APPROVAL", ch, snap["state"])
	}
	if _, answered := to(ch, "customer_updated_date_confirmation"); answered {
		t.Fatalf("a first proposal's outbox row carries an answer: %v", ch)
	}

	// WSO2 asks for a different time: the answer changed to DISAGREE, the date did not move.
	if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
		t.Fatalf("a different time: %v", err)
	}
	ch, _ = last(f, id)
	if v, ok := to(ch, "customer_updated_date_confirmation"); !ok || v != "DISAGREE" {
		t.Fatalf("a different time's outbox row = %v, want the answer changed to DISAGREE", ch)
	}
	if _, moved := to(ch, "customer_updated_on"); moved {
		t.Fatalf("an answer moved the proposed date: %v", ch)
	}

	// The customer proposes again while that answer stands: ONE row whose diff holds BOTH columns
	// (the 0052 trigger / the explicit clear), and the date is read first -> still "the customer proposed".
	f.mustPropose(id, crScopeUserA2, rsStartEarly)
	ch, snap = last(f, id)
	if _, moved := to(ch, "customer_updated_on"); !moved || snap["state"] != "CUSTOMER_APPROVAL" {
		t.Fatalf("a proposal over a standing answer: outbox row = %v / state %v", ch, snap["state"])
	}
	if _, cleared := ch["customer_updated_date_confirmation"]; !cleared {
		t.Fatalf("the standing answer was not cleared in the same row: %v", ch)
	}

	// WSO2 declines (keeps the plan): the SAME row as a different time as far as a notice can tell --
	// the answer changed to DISAGREE, the proposed date did not move -- and nothing else of the change
	// moved: no state, no planned window (the decline writes the answer and nothing more).
	if err := f.counter(id, nil, nil); err != nil {
		t.Fatalf("a decline: %v", err)
	}
	ch, snap = last(f, id)
	if v, ok := to(ch, "customer_updated_date_confirmation"); !ok || v != "DISAGREE" {
		t.Fatalf("a decline's outbox row = %v, want the answer changed to DISAGREE", ch)
	}
	if _, moved := to(ch, "customer_updated_on"); moved || snap["state"] != "CUSTOMER_APPROVAL" {
		t.Fatalf("a decline's outbox row moved the proposed date or left the state at %v: %v", snap["state"], ch)
	}
	for _, col := range []string{"state", "start_on", "end_on"} {
		if _, moved := ch[col]; moved {
			t.Fatalf("a decline's outbox row carries a change of %s: %v (a decline keeps the plan and the state)", col, ch)
		}
	}
	// The customer proposes once more over the standing DISAGREE: the date is read first, again.
	f.mustPropose(id, crScopeUserA1, rsStart2)
	ch, snap = last(f, id)
	if _, moved := to(ch, "customer_updated_on"); !moved || snap["state"] != "CUSTOMER_APPROVAL" {
		t.Fatalf("a proposal over a decline: outbox row = %v / state %v", ch, snap["state"])
	}
	if _, cleared := ch["customer_updated_date_confirmation"]; !cleared {
		t.Fatalf("the declined answer was not cleared in the same row: %v", ch)
	}

	// WSO2 accepts: the answer changed to AGREE, the date did not move, the state left Customer Approval.
	f.mustAccept(id)
	ch, snap = last(f, id)
	if v, ok := to(ch, "customer_updated_date_confirmation"); !ok || v != "AGREE" {
		t.Fatalf("Accept's outbox row = %v, want the answer changed to AGREE", ch)
	}
	if _, moved := to(ch, "customer_updated_on"); moved || snap["state"] != "SCHEDULED" {
		t.Fatalf("Accept's outbox row moved the date or left the state at %v: %v", snap["state"], ch)
	}
}
