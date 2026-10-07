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
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The customer requirements lock (change_request_customer_lock.go): the Customer
// Project and the two creation-form boxes are free in New, the project is FROZEN
// by Request Approval and the boxes are ADD-ONLY from then on. Same harness as
// the other crFlow tests (DSN-gated by CHANGE_REQUEST_TEST_DSN; run it as the
// superuser AND as the non-superuser application role).
//
// The state-by-state table lives in change_request_customer_lock_test.go as a unit
// test of the rule; these tests drive it through PatchChangeRequest against
// Postgres, with the whole PATCH transaction (the refusal writes nothing, not even
// the other fields of the same request).

const (
	lockMsgFrozen        = "projectId can no longer be changed: the Customer Project is fixed once approval has been requested"
	lockMsgTurnOff       = "can no longer be turned off: once approval has been requested a customer requirement can be added but never removed"
	lockMsgNeedsProject  = "cannot be turned on: this change request has no Customer Project, and one can no longer be set after approval was requested"
	lockMsgReturnToNew   = `state "new" cannot be set: a change request that has left New cannot return to it. Cancel it and clone it instead.`
	lockMsgRequestNoProj = "approval cannot be requested: the customer's approval and/or review is required but no Customer Project is set, so there is nobody to ask. Select a Customer Project first (or clear the requirement)."
)

// stored reads the three stored fields the lock is about.
func (f *crFlow) lockStored(id string) (state string, project *string, approval, review bool) {
	f.t.Helper()
	if err := f.scoped.QueryRow(f.sys,
		`SELECT COALESCE(cr.state::text, ''), wi.project_id::text, cr.customer_approval_required, cr.customer_review_required
		 FROM change_request cr JOIN work_item wi ON wi.id = cr.id WHERE cr.id = $1`, id).Scan(&state, &project, &approval, &review); err != nil {
		f.t.Fatalf("read the lock fields: %v", err)
	}
	return state, project, approval, review
}

func (f *crFlow) wantProject(id, when string, want *string) {
	f.t.Helper()
	_, got, _, _ := f.lockStored(id)
	switch {
	case want == nil && got == nil:
	case want != nil && got != nil && strings.EqualFold(*want, *got):
	default:
		f.t.Fatalf("stored project %s = %v, want %v", when, lockDeref(got), lockDeref(want))
	}
}

func lockDeref(s *string) string {
	if s == nil {
		return "<none>"
	}
	return *s
}

func (f *crFlow) subjectOf(id string) string {
	f.t.Helper()
	var s string
	if err := f.scoped.QueryRow(f.sys, `SELECT subject FROM work_item WHERE id = $1`, id).Scan(&s); err != nil {
		f.t.Fatalf("read subject: %v", err)
	}
	return s
}

// In New everything is editable, in both directions, with or without a project:
// the Customer Project is chosen, moved and moved back, and the Customer Group
// that follows it is read back; both boxes are ticked and unticked.
func TestChangeRequestLockIntegration_NewIsFullyEditable(t *testing.T) {
	f := newCustomerGroupFlow(t)
	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard, domain.ChangeRequestTypeEmergency} {
		id := f.createWithProject(typ, nil, false, false)
		f.expect(id, "after create", "NEW", "assess", "canceled")

		// Both boxes, both ways, with no project at all.
		for _, v := range []bool{true, false, true, false} {
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(v), CustomerReviewRequired: boolp(!v)}); err != nil {
				t.Fatalf("%s: set boxes %v/%v in New: %v", typ, v, !v, err)
			}
		}
		// The project: chosen, moved to another customer's, back, resent.
		for _, p := range []string{crScopeProjectA, crScopeProjectB, crScopeProjectA, crScopeProjectA, crScopeProjectC} {
			f.setProject(id, p)
			f.wantProject(id, "after choosing "+p, &p)
		}
		f.setProject(id, crScopeProjectA)
		if got := contactNames(f.get(id).CustomerContacts); strings.Join(got, ",") != "Alice Aaron,Bob Bell" {
			t.Fatalf("%s: customer contacts after choosing project A = %v", typ, got)
		}
		// And the whole form at once, as the Edit dialog sends it.
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{
			ProjectID: sp(crScopeProjectB), DeploymentIDs: &[]string{}, CustomerApprovalRequired: boolp(true), CustomerReviewRequired: boolp(true), Title: sp("renamed"),
		}); err != nil {
			t.Fatalf("%s: the whole form in New: %v", typ, err)
		}
		if _, p, a, r := f.lockStored(id); p == nil || *p != crScopeProjectB || !a || !r {
			t.Fatalf("%s: stored after the whole form = project %v approval %v review %v", typ, lockDeref(p), a, r)
		}
	}
}

// A state of NULL (a pre-lifecycle row) counts as New: free, and Request
// Approval still applies.
func TestChangeRequestLockIntegration_NullStateCountsAsNew(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	f.setState(id, "")
	f.setProject(id, crScopeProjectB)
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)}); err != nil {
		t.Fatalf("tick on a change with no state: %v", err)
	}
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(false)}); err != nil {
		t.Fatalf("untick on a change with no state: %v", err)
	}
	_, err := f.patchState(id, domain.ChangeRequestStateNew)
	if err != nil {
		t.Fatalf("{state: new} on a change with no state: %v", err)
	}
}

// From Request Approval on the Customer Project is frozen: in every state a
// different project is a 400 that names the state, writes nothing (not even the
// rest of the same PATCH), and the stored project is resent without complaint. A
// change with no project cannot be given one either (NULL -> X is a change).
func TestChangeRequestLockIntegration_ProjectIsFrozenAfterRequestApproval(t *testing.T) {
	f := newCustomerGroupFlow(t)
	for _, state := range []string{"ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "ROLLBACK", "CLOSED", "CANCELED"} {
		withProject := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		noProject := f.createWithProject(domain.ChangeRequestTypeNormal, nil, false, false)
		f.setState(withProject, state)
		f.setState(noProject, state)

		// Moved to another customer's project, deployments cleared as the form does.
		_, err := f.patch(withProject, domain.PatchChangeRequestRequest{ProjectID: sp(crScopeProjectB), DeploymentIDs: &[]string{}, Title: sp("must not be written")})
		f.wantValidationError("moving the project in "+state, err, lockMsgFrozen)
		f.wantValidationError("moving the project in "+state, err, "(current state: "+strings.ToLower(state)+")")
		f.wantProject(withProject, "after the refused move in "+state, sp(crScopeProjectA))
		if got := f.subjectOf(withProject); got != crFlowSubject {
			t.Fatalf("%s: a refused PATCH wrote the title: %q", state, got)
		}
		// Without deployments in the request too (a project-only PATCH).
		_, err = f.patch(withProject, domain.PatchChangeRequestRequest{ProjectID: sp(crScopeProjectC)})
		f.wantValidationError("a project-only move in "+state, err, lockMsgFrozen)

		// The stored value resent is an accepted no-op, with other fields along.
		if _, err := f.patch(withProject, domain.PatchChangeRequestRequest{ProjectID: sp(crScopeProjectA), Title: sp("renamed in " + state)}); err != nil {
			t.Fatalf("%s: resending the stored project: %v", state, err)
		}
		if got := f.subjectOf(withProject); got != "renamed in "+state {
			t.Fatalf("%s: the resend did not write the rest of the PATCH: %q", state, got)
		}

		// No project stored: giving it one is a change too.
		_, err = f.patch(noProject, domain.PatchChangeRequestRequest{ProjectID: sp(crScopeProjectA)})
		f.wantValidationError("choosing a project in "+state, err, lockMsgFrozen)
		f.wantProject(noProject, "after the refused choice in "+state, nil)
	}
}

// The same through the real transition: Request Approval is what freezes it, for
// every type, and the project cannot be moved while the customer is being asked.
func TestChangeRequestLockIntegration_RequestApprovalFreezesTheProject(t *testing.T) {
	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard, domain.ChangeRequestTypeEmergency} {
		typ := typ
		t.Run(string(typ), func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
			id := f.createWithProject(typ, sp(crScopeProjectA), true, false)
			f.setProject(id, crScopeProjectB) // free in New
			f.setProject(id, crScopeProjectA)
			f.requestApproval(id)

			_, err := f.patch(id, domain.PatchChangeRequestRequest{ProjectID: sp(crScopeProjectB), DeploymentIDs: &[]string{}})
			f.wantValidationError("moving the project after Request Approval", err, lockMsgFrozen)
			f.wantProject(id, "after the refused move", sp(crScopeProjectA))
			// A change that is being asked for the customer's approval right now keeps asking the same contacts.
			if typ == domain.ChangeRequestTypeStandard {
				assertApprovers(t, "the customer's request", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
			}
		})
	}
}

// The boxes against Postgres, state by state: what the rule says (the unit test
// holds the full table), with the stored value unchanged after a refusal.
func TestChangeRequestLockIntegration_BoxesAreAddOnlyAfterNew(t *testing.T) {
	type outcome struct{ approvalOn, approvalOnBare, approvalOff, reviewOn, reviewOnBare, reviewOff string }
	want := map[string]outcome{
		"NEW":               {"ok", "ok", "ok", "ok", "ok", "ok"},
		"ASSESS":            {"ok", "needs-project", "off", "ok", "needs-project", "off"},
		"AUTHORIZE":         {"ok", "needs-project", "off", "ok", "needs-project", "off"},
		"CUSTOMER_APPROVAL": {"gate", "gate", "off", "ok", "needs-project", "off"},
		"SCHEDULED":         {"gate", "gate", "off", "ok", "needs-project", "off"},
		"IMPLEMENT":         {"gate", "gate", "off", "ok", "needs-project", "off"},
		"REVIEW":            {"gate", "gate", "off", "ok", "needs-project", "off"},
		"CUSTOMER_REVIEW":   {"gate", "gate", "off", "gate", "gate", "off"},
		"ROLLBACK":          {"gate", "gate", "off", "gate", "gate", "off"},
		"CLOSED":            {"gate", "gate", "off", "gate", "gate", "off"},
		"CANCELED":          {"gate", "gate", "off", "gate", "gate", "off"},
	}
	msgOf := map[string]string{
		"off":           lockMsgTurnOff,
		"needs-project": lockMsgNeedsProject,
		"gate":          "can no longer be changed: the change request has already",
	}
	f := newCustomerGroupFlow(t)
	for state, w := range want {
		try := func(project *string, ticked bool, field string, value bool, outcome string) {
			t.Helper()
			id := f.createWithProject(domain.ChangeRequestTypeNormal, project, ticked, ticked)
			f.setState(id, state)
			req := domain.PatchChangeRequestRequest{CustomerApprovalRequired: &value}
			name := "customerApprovalRequired"
			if field == "review" {
				req = domain.PatchChangeRequestRequest{CustomerReviewRequired: &value}
				name = "customerReviewRequired"
			}
			req.Title = sp("must not be written")
			_, err := f.patch(id, req)
			label := fmt.Sprintf("%s %s=%v (project %v, was %v)", state, name, value, project != nil, ticked)
			if outcome == "ok" {
				if err != nil {
					t.Fatalf("%s: %v", label, err)
				}
				_, _, a, r := f.lockStored(id)
				if got := map[string]bool{"approval": a, "review": r}[field]; got != value {
					t.Fatalf("%s: stored %v", label, got)
				}
				return
			}
			f.wantValidationError(label, err, name+" "+msgOf[outcome])
			_, _, a, r := f.lockStored(id)
			if a != ticked || r != ticked {
				t.Fatalf("%s: a refused PATCH changed the boxes to %v/%v", label, a, r)
			}
			if got := f.subjectOf(id); got != crFlowSubject {
				t.Fatalf("%s: a refused PATCH wrote the rest of the request: %q", label, got)
			}
		}
		try(sp(crScopeProjectA), false, "approval", true, w.approvalOn)
		try(nil, false, "approval", true, w.approvalOnBare)
		try(sp(crScopeProjectA), true, "approval", false, w.approvalOff)
		try(sp(crScopeProjectA), false, "review", true, w.reviewOn)
		try(nil, false, "review", true, w.reviewOnBare)
		try(sp(crScopeProjectA), true, "review", false, w.reviewOff)
		// An unchanged value never fails: ticked stays ticked, unticked stays unticked.
		try(sp(crScopeProjectA), true, "approval", true, "ok")
		try(nil, false, "review", false, "ok")
	}
}

// Request Approval is refused, in the same transaction and with nothing written,
// when a box is ticked and there is no Customer Project to ask: for every type
// (Standard included: it would otherwise land in Customer Approval with nobody).
// Clearing the box, or choosing the project, in the same PATCH lets it through.
func TestChangeRequestLockIntegration_RequestApprovalNeedsAProject(t *testing.T) {
	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard, domain.ChangeRequestTypeEmergency} {
		for _, boxes := range []struct {
			name             string
			approval, review bool
		}{{"approval", true, false}, {"review", false, true}, {"both", true, true}} {
			typ, boxes := typ, boxes
			t.Run(string(typ)+"/"+boxes.name, func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
				id := f.createWithProject(typ, nil, boxes.approval, boxes.review)

				// Request Approval alone, then with an unrelated field riding along.
				_, err := f.patchState(id, domain.ChangeRequestStateAssess)
				f.wantValidationError("Request Approval with no project", err, lockMsgRequestNoProj)
				_, err = f.patch(id, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateAssess), Title: sp("must not be written")})
				f.wantValidationError("Request Approval with no project and a title", err, lockMsgRequestNoProj)
				f.expect(id, "after the refused Request Approval", "NEW", "assess", "canceled")
				if n := len(f.stages(id)); n != 0 {
					t.Fatalf("a refused Request Approval provisioned %d stages", n)
				}
				if got := f.subjectOf(id); got != crFlowSubject {
					t.Fatalf("a refused Request Approval wrote the rest of the PATCH: %q", got)
				}

				// Clearing the requirement in the same PATCH is a valid Request Approval.
				cleared := domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateAssess)}
				if boxes.approval {
					cleared.CustomerApprovalRequired = boolp(false)
				}
				if boxes.review {
					cleared.CustomerReviewRequired = boolp(false)
				}
				// (a box left ticked still refuses it)
				if boxes.approval && boxes.review {
					half := cleared
					half.CustomerReviewRequired = nil
					_, err := f.patch(id, half)
					f.wantValidationError("only one of the two boxes cleared", err, lockMsgRequestNoProj)
				}
				id2 := f.createWithProject(typ, nil, boxes.approval, boxes.review)
				if _, err := f.patch(id2, cleared); err != nil {
					t.Fatalf("Request Approval with the requirement cleared in the same PATCH: %v", err)
				}
				// Choosing the project in the same PATCH is one too.
				id3 := f.createWithProject(typ, nil, boxes.approval, boxes.review)
				if _, err := f.patch(id3, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateAssess), ProjectID: sp(crScopeProjectA)}); err != nil {
					t.Fatalf("Request Approval with the project chosen in the same PATCH: %v", err)
				}
				f.wantProject(id3, "after Request Approval", sp(crScopeProjectA))
			})
		}
	}
}

// A change that needs nobody has no use for a project: Request Approval works
// without one. And a RESEND of Request Approval on a change that already left New
// is the idempotent no-op it always was, even for a legacy change that ticked a
// box and has no project (created before the lock, or by ServiceNow).
func TestChangeRequestLockIntegration_RequestApprovalWithoutRequirementsAndResends(t *testing.T) {
	f := newCustomerGroupFlow(t)
	seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard, domain.ChangeRequestTypeEmergency} {
		id := f.createWithProject(typ, nil, false, false)
		f.requestApproval(id)
	}
	// A legacy shape: Standard, in Customer Approval, ticked, no project.
	legacy := f.createWithProject(domain.ChangeRequestTypeStandard, nil, true, false)
	f.setState(legacy, "CUSTOMER_APPROVAL")
	if _, err := f.patchState(legacy, domain.ChangeRequestStateAssess); err != nil {
		t.Fatalf("a resent Request Approval on a legacy change in Customer Approval: %v", err)
	}
	f.expect(legacy, "after the resend", "CUSTOMER_APPROVAL", "authorize", "canceled")
	// ...and staff cannot answer for the customer: the manual scheduled is refused
	// (such a change can be cancelled or re-scheduled).
	_, err := f.patchState(legacy, domain.ChangeRequestStateScheduled)
	f.wantValidationError("manual scheduled out of a legacy change with no project", err, "can only be given by the customer in the Customer Portal")
	f.expect(legacy, "after the refused manual scheduled", "CUSTOMER_APPROVAL", "authorize", "canceled")
}

// {state: "new"} is a no-op while New (or NULL) and a readable 400 in every other
// state; nothing else in the request is written. A closed, canceled or rolled-back
// change is refused as every request to move one is ("... cannot be moved").
func TestChangeRequestLockIntegration_ReturnToNewIsRefused(t *testing.T) {
	f := newCustomerGroupFlow(t)
	for _, state := range []string{"ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "ROLLBACK", "CLOSED", "CANCELED"} {
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.setState(id, state)
		_, err := f.patch(id, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateNew), Title: sp("must not be written")})
		want := lockMsgReturnToNew
		switch state {
		case "ROLLBACK":
			want = `state "new" cannot be set manually from rollback: a change request that is rolled back cannot be moved`
		case "CLOSED", "CANCELED":
			want = `state "new" cannot be set manually from ` + strings.ToLower(state) + `: a change request that is ` + strings.ToLower(state) + ` cannot be moved`
		}
		f.wantValidationError("{state: new} in "+state, err, want)
		if got := f.state(id); got != state {
			t.Fatalf("{state: new} moved %s to %s", state, got)
		}
		if got := f.subjectOf(id); got != crFlowSubject {
			t.Fatalf("a refused {state: new} wrote the rest of the PATCH in %s: %q", state, got)
		}
	}
	// New and NULL: accepted, stays New.
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	if _, err := f.patchState(id, domain.ChangeRequestStateNew); err != nil {
		t.Fatalf("{state: new} in New: %v", err)
	}
	f.expect(id, "after {state: new} in New", "NEW", "assess", "canceled")
}

// A change that has reached a customer stage can never get back to a point where
// the customer is no longer asked. Re-schedule sends a Normal / Emergency change
// back to Authorize and a Standard one stays in Customer Approval; in none of them
// can the box be unticked (the hole the lock exists to close), the project moved
// or the change sent back to New, and the approval that follows asks again:
// the same contacts, in a fresh stage, the old one kept as a record.
func TestChangeRequestLockIntegration_RescheduleCannotReopenTheCustomersApproval(t *testing.T) {
	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeEmergency, domain.ChangeRequestTypeStandard} {
		typ := typ
		t.Run(string(typ), func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
			id := f.createWithProject(typ, sp(crScopeProjectA), true, false)
			f.setPlanned(id, rsStart1, rsEnd1)
			cabApprover := crCABMemberUserID1
			switch typ {
			case domain.ChangeRequestTypeNormal:
				f.driveToCustomerApproval(id)
			case domain.ChangeRequestTypeEmergency:
				cabApprover = crECABMemberUserID
				f.requestApproval(id)
				if err := f.decide(id, crECABMemberUserID, "approved"); err != nil {
					t.Fatalf("ECAB approval: %v", err)
				}
			default:
				f.requestApproval(id)
			}
			f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
			if len(f.customerStages(id)) != 1 {
				t.Fatalf("customer stages before the re-schedule = %+v", f.customerStages(id))
			}

			// Re-schedule: the window moves, the customer's request is superseded.
			if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
				t.Fatalf("Re-schedule: %v", err)
			}
			wantState := "AUTHORIZE"
			if typ == domain.ChangeRequestTypeStandard {
				wantState = "CUSTOMER_APPROVAL"
			}
			if got := f.state(id); got != wantState {
				t.Fatalf("state after the Re-schedule = %s, want %s", got, wantState)
			}

			// The hole: untick the box, in this state, alone or with anything else.
			for _, req := range []domain.PatchChangeRequestRequest{
				{CustomerApprovalRequired: boolp(false)},
				{CustomerApprovalRequired: boolp(false), CustomerReviewRequired: boolp(false), Title: sp("must not be written")},
			} {
				_, err := f.patch(id, req)
				f.wantValidationError("unticking after the Re-schedule", err, "customerApprovalRequired "+lockMsgTurnOff)
			}
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{ProjectID: sp(crScopeProjectB), DeploymentIDs: &[]string{}}); err == nil {
				t.Fatal("the project was moved after the Re-schedule")
			}
			f.wantValidationError("back to New after the Re-schedule", func() error { _, err := f.patchState(id, domain.ChangeRequestStateNew); return err }(), lockMsgReturnToNew)
			if _, project, approval, _ := f.lockStored(id); !approval || project == nil || *project != crScopeProjectA {
				t.Fatalf("stored after the refused edits: approval %v, project %v", approval, lockDeref(project))
			}
			if got := f.subjectOf(id); got != crFlowSubject {
				t.Fatalf("a refused edit wrote the title: %q", got)
			}

			// Nothing else reopened: the internal approval runs again and the customer is asked again.
			if typ != domain.ChangeRequestTypeStandard {
				f.wantCanAnswer(id, "while the new plan awaits internal approval", false, crScopeUserA1, crScopeUserA2)
				if err := f.decide(id, cabApprover, "approved"); err != nil {
					t.Fatalf("approval of the new plan: %v", err)
				}
			}
			f.expect(id, "after the new plan is approved", "CUSTOMER_APPROVAL", "authorize", "canceled")
			st := f.customerStages(id)
			if len(st) != 2 || liveStages(st) != 1 {
				t.Fatalf("customer stages after the re-approval = %+v, want 2 (the old one cancelled, a fresh one live)", st)
			}
			assertApprovers(t, "the superseded request", st[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
			assertApprovers(t, "the fresh request", st[1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
			f.wantCanAnswer(id, "once the customer is asked again", true, crScopeUserA1, crScopeUserA2)
		})
	}
}

// A customer's own proposal of a new time (the Re-schedule a contact starts) is
// the same: they cannot take their own approval away by it either.
func TestChangeRequestLockIntegration_CustomerProposalCannotReopenTheApproval(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	start := time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Second)
	if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{
		PlannedStartOn: sp(start.Format(time.RFC3339)), PlannedEndOn: sp(start.Add(2 * time.Hour).Format(time.RFC3339))}); err != nil {
		t.Fatalf("the customer's proposal: %v", err)
	}
	f.expect(id, "after the proposal", "AUTHORIZE", "canceled")
	// The customer is not entitled to the box either: it is not one of the fields they may send.
	_, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(false)})
	if err == nil {
		t.Fatal("a customer unticked customerApprovalRequired")
	}
	// ...and WSO2 cannot after it.
	_, err = f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(false)})
	f.wantValidationError("unticking after the customer's proposal", err, "customerApprovalRequired "+lockMsgTurnOff)
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.expect(id, "after the new plan is approved", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantCanAnswer(id, "asked again", true, crScopeUserA1, crScopeUserA2)
}

// Customer Review: the same shape. A change in Review cannot turn its review
// requirement off to be closed straight from Review, a change in Customer Review
// cannot be un-asked, and a customer who rejects the review sends it to Rollback,
// where nothing can be edited back either.
func TestChangeRequestLockIntegration_CustomerReviewCannotBeReopened(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), false, true)
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "SCHEDULED", "implement", "canceled")
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")

	untick := func(when string) {
		t.Helper()
		_, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerReviewRequired: boolp(false), Title: sp("must not be written")})
		f.wantValidationError("unticking the review "+when, err, "customerReviewRequired "+lockMsgTurnOff)
		if _, _, _, review := f.lockStored(id); !review {
			t.Fatalf("customerReviewRequired was unticked %s", when)
		}
		if got := f.subjectOf(id); got != crFlowSubject {
			t.Fatalf("a refused edit wrote the title %s: %q", when, got)
		}
	}
	untick("in Review")
	_, err := f.patch(id, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateClosed), CustomerReviewRequired: boolp(false)})
	f.wantValidationError("closing from Review and unticking the review in one PATCH", err, "customerReviewRequired "+lockMsgTurnOff)
	_, err = f.patchState(id, domain.ChangeRequestStateClosed)
	f.wantValidationError("closing a Review that requires the customer", err, "customer review is required")
	f.expect(id, "still in Review", "REVIEW", "customer_review", "rollback", "canceled")

	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	untick("in Customer Review")
	assertApprovers(t, "the customer's review request", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

	if err := f.decide(id, crScopeUserA1, "rejected"); err != nil {
		t.Fatalf("the customer rejects the review: %v", err)
	}
	f.expect(id, "after the rejected review", "ROLLBACK")
	untick("in Rollback")
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)}); err == nil {
		t.Fatal("ticking the approval box on a rolled-back change was accepted")
	}
}

// A change with its box ticked that reached the customer state with NO contacts to
// ask (the project has none registered) has no stage rows, and nobody can answer
// for the customer: it can be cancelled or re-scheduled, or wait for a contact.
// The lock holds there too -- the box stays ticked, the project cannot be swapped
// for one that has contacts -- and a contact who registers afterwards is picked up
// by resending the project, the only trigger that survives the lock. Request
// Approval refuses a ticked box on a project nobody can be asked on, so the way
// to get here is the residual edge: the contact the project had when approval
// was requested left before the gate (requestApprovalThenContactsLeave).
func TestChangeRequestLockIntegration_NoContactsReachedTheStage(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
	f.requestApprovalThenContactsLeave(id)
	f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.expect(id, "in Customer Approval with nobody to ask", "CUSTOMER_APPROVAL", "authorize", "canceled")
	if n := len(f.stages(id)); n != 2 {
		t.Fatalf("stages = %v, want only peer and CAB (no customer stage rows)", f.labels(id))
	}

	_, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(false)})
	f.wantValidationError("unticking with nobody to ask", err, "customerApprovalRequired "+lockMsgTurnOff)
	_, err = f.patch(id, domain.PatchChangeRequestRequest{ProjectID: sp(crScopeProjectA), DeploymentIDs: &[]string{}})
	f.wantValidationError("swapping in a project that has contacts", err, lockMsgFrozen)
	if len(f.customerStages(id)) != 0 {
		t.Fatalf("a refused edit provisioned a customer stage: %+v", f.customerStages(id))
	}

	// A contact registers on the project afterwards: resending the stored project
	// re-derives the Customer Group and asks them.
	f.registerContact(crScopeProjectC, crScopeAccountID, crScopeUserA1)
	f.setProject(id, crScopeProjectC)
	st := f.customerStages(id)
	if len(st) != 1 || liveStages(st) != 1 {
		t.Fatalf("customer stages after a contact registered and the project was resent = %+v, want one live", st)
	}
	assertApprovers(t, "the newly asked contact", st[0].approvers, map[string]string{crScopeUserA1: "REQUESTED"})
	f.expect(id, "now waiting on the contact", "CUSTOMER_APPROVAL", "authorize", "canceled")
}

// The outcome flags (the STAMP of the customer's answer, not the requirement) stay
// one-way: once the customer approved, nobody can set it back, staff or customer,
// and the requirement box is locked on as well.
func TestChangeRequestLockIntegration_OutcomeFlagsStayOneWay(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, false)
	f.requestApproval(id)
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("the customer approves: %v", err)
	}
	f.expect(id, "after the customer approved", "SCHEDULED", "implement", "canceled")
	if approved, _ := f.customerOutcome(id); !approved {
		t.Fatal("the approval was not stamped")
	}
	for who, send := range map[string]func() error{
		"staff": func() error {
			_, err := f.patch(id, domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(false)})
			return err
		},
		"the customer who approved": func() error { _, err := f.approveAs(id, crScopeUserA1, false); return err },
		"another contact":           func() error { _, err := f.approveAs(id, crScopeUserA2, false); return err },
	} {
		if err := send(); err == nil {
			t.Fatalf("%s set the stamped approval back to false", who)
		}
		if approved, _ := f.customerOutcome(id); !approved {
			t.Fatalf("the stamped approval was cleared by %s", who)
		}
	}
	_, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(false)})
	f.wantValidationError("unticking after the answer", err, "customerApprovalRequired "+lockMsgTurnOff)
}

// Deployments and deployment products keep their until-implement window but
// resolve against the FROZEN project: another project's deployment is refused
// after New whether it is sent as a list or, skipping the list rules, as the
// single-valued fields.
func TestChangeRequestLockIntegration_DeploymentsFollowTheFrozenProject(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepProd}}); err != nil {
		t.Fatalf("a deployment of the project in New: %v", err)
	}
	f.requestApproval(id)

	_, err := f.patch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepOtherB}})
	f.wantValidationError("another project's deployment as a list", err, "does not belong to the selected project")
	_, err = f.patch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepOtherB}, ProjectID: sp(crScopeProjectB)})
	f.wantValidationError("another project's deployment together with its project", err, lockMsgFrozen)
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepStage, crScopeDepProd}}); err != nil {
		t.Fatalf("this project's deployments in Assess: %v", err)
	}

	// The single-valued fields skip the list rules; they follow the project too.
	_, err = f.patch(id, domain.PatchChangeRequestRequest{DeploymentID: sp(crScopeDepOtherB), Title: sp("must not be written")})
	f.wantValidationError("another project's deployment as deploymentId", err, "deploymentId does not belong to the change request's project")
	_, err = f.patch(id, domain.PatchChangeRequestRequest{DeployedProductID: sp(crScopeDPOtherB)})
	f.wantValidationError("another project's product as deployedProductId", err, "deployedProductId does not belong to the change request's project")
	if got := f.subjectOf(id); got != crFlowSubject {
		t.Fatalf("a refused PATCH wrote the title: %q", got)
	}
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{DeploymentID: sp(crScopeDepDev), DeployedProductID: sp(crScopeDPProdOne)}); err != nil {
		t.Fatalf("this project's single-valued deployment fields: %v", err)
	}
	// Re-sending the stored value is a no-op in every state, even past the window.
	f.setState(id, "IMPLEMENT")
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{DeploymentID: sp(crScopeDepDev)}); err != nil {
		t.Fatalf("resending the stored deploymentId in Implement: %v", err)
	}
	_, err = f.patch(id, domain.PatchChangeRequestRequest{DeploymentID: sp(crScopeDepProd)})
	f.wantValidationError("deploymentId past the window", err, "deploymentId can no longer be changed")

	// A change with no project can take no deployment, after New or in it.
	bare := f.createWithProject(domain.ChangeRequestTypeNormal, nil, false, false)
	_, err = f.patch(bare, domain.PatchChangeRequestRequest{DeploymentID: sp(crScopeDepProd)})
	f.wantValidationError("a deployment on a change with no project", err, "deploymentId requires a Customer Project")
}

// The creates are the creation phase: a change is created in New, so everything
// is accepted there, on both create paths, whatever the boxes and the project --
// including a ticked box and no project, which Request Approval (not the create) is
// what refuses. Clone is a create. The local seed's fixtures that ask the customer
// have a project, so the lock never meets one of them without.
func TestChangeRequestLockIntegration_CreatesAreFreeInNew(t *testing.T) {
	f := newCustomerGroupFlow(t)
	typ := domain.ChangeRequestTypeNormal
	for _, project := range []*string{nil, sp(crScopeProjectA)} {
		id := f.createWithProject(typ, project, true, true)
		f.expect(id, "after create", "NEW", "assess", "canceled")
		if _, _, a, r := f.lockStored(id); !a || !r {
			t.Fatalf("a create with both boxes ticked stored %v/%v", a, r)
		}
	}
	group := crFlowGroupID
	resp, err := f.repo.CreateChangeRequestFromServiceNow(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &group, CustomerApprovalRequired: boolp(true), CustomerReviewRequired: boolp(true),
	}, "3aaaaaaa-0000-0000-0000-0000000000f7", "CRFLOWSN007", "x@example.com")
	if err != nil {
		t.Fatalf("a ServiceNow-first create with both boxes ticked and no project: %v", err)
	}
	f.expect(resp.ChangeRequest.ID, "after the ServiceNow-first create", "NEW", "assess", "canceled")
	_, err = f.patchState(resp.ChangeRequest.ID, domain.ChangeRequestStateAssess)
	f.wantValidationError("Request Approval of the ServiceNow-first change", err, lockMsgRequestNoProj)

	// The local seed.
	var offenders int
	if err := f.scoped.QueryRow(f.sys, `
		SELECT COUNT(*) FROM change_request cr JOIN work_item wi ON wi.id = cr.id
		WHERE wi.number LIKE 'CHG-FIXED-%' AND (cr.customer_approval_required OR cr.customer_review_required)
		  AND COALESCE(cr.state::text, 'NEW') <> 'NEW' AND wi.project_id IS NULL`).Scan(&offenders); err != nil {
		t.Fatalf("read the seeded fixtures: %v", err)
	}
	if offenders != 0 {
		t.Fatalf("%d seeded CHG-FIXED-* change requests ask the customer, left New and have no Customer Project", offenders)
	}
}

// Request Approval racing a project edit. The PATCH takes the work_item lock and
// only then reads the change_request side, so an edit that waited behind a
// Request Approval judges the project against the state the change is in NOW.
// Deterministic: a transaction stands in for the Request Approval in progress (it
// holds the row locks and has moved the state to Assess, uncommitted), the project
// edit is started and left to block on the lock, and only then is the first
// transaction committed. Read-and-lock in one statement would have judged the edit
// against New (a snapshot older than the lock's grant) and moved the project.
func TestChangeRequestLockIntegration_RequestApprovalRacingAProjectEdit(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)

	locked, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	// However the test ends, the transaction standing in for the other request is
	// let go (a failure must not leave it holding the pool's connection, which would
	// hang the pool's Close instead of failing).
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	var wg sync.WaitGroup
	var once sync.Once
	var txErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer once.Do(func() { close(locked) })
		txErr = f.scoped.InTx(f.sys, func(tx pgx.Tx) error {
			// What Request Approval does first: the work_item lock, then the state.
			if _, err := tx.Exec(f.sys, `SELECT 1 FROM work_item WHERE id = $1 FOR NO KEY UPDATE`, id); err != nil {
				return err
			}
			if _, err := tx.Exec(f.sys, `UPDATE change_request SET state = 'ASSESS' WHERE id = $1`, id); err != nil {
				return err
			}
			once.Do(func() { close(locked) })
			<-release
			return nil
		})
	}()
	<-locked
	if txErr != nil {
		t.Fatalf("the Request Approval transaction: %v", txErr)
	}

	type result struct{ err error }
	done := make(chan result, 1)
	go func() {
		_, err := f.patch(id, domain.PatchChangeRequestRequest{ProjectID: sp(crScopeProjectB), DeploymentIDs: &[]string{}})
		done <- result{err}
	}()
	// Wait until the edit is really blocked on the lock (it holds a backend that
	// is waiting on a Lock), then let the Request Approval commit.
	blocked := false
	for i := 0; i < 100 && !blocked; i++ {
		var n int
		if err := f.pool.QueryRow(f.sys,
			`SELECT COUNT(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query ILIKE '%FROM work_item%FOR NO KEY UPDATE%'`).Scan(&n); err != nil {
			t.Fatalf("look for the blocked edit: %v", err)
		}
		blocked = n > 0
		if !blocked {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if !blocked {
		t.Fatal("the project edit never blocked on the work_item lock")
	}
	select {
	case r := <-done:
		t.Fatalf("the project edit finished while the Request Approval held the lock: %v", r.err)
	default:
	}
	releaseOnce.Do(func() { close(release) })
	wg.Wait()
	if txErr != nil {
		t.Fatalf("the Request Approval transaction: %v", txErr)
	}
	r := <-done
	f.wantValidationError("a project edit that waited behind Request Approval", r.err, lockMsgFrozen)
	f.wantProject(id, "after the raced edit", sp(crScopeProjectA))
}

// And the other order: a Request Approval that waited behind a project edit sees
// the project that edit set, so the box it needs a project for is satisfied.
func TestChangeRequestLockIntegration_RequestApprovalAfterAProjectEditSeesTheProject(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeStandard, nil, true, false)

	locked, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	// However the test ends, the transaction standing in for the other request is
	// let go (a failure must not leave it holding the pool's connection, which would
	// hang the pool's Close instead of failing).
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	var wg sync.WaitGroup
	var once sync.Once
	var txErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer once.Do(func() { close(locked) })
		txErr = f.scoped.InTx(f.sys, func(tx pgx.Tx) error {
			if _, err := tx.Exec(f.sys, `UPDATE work_item SET project_id = $2::uuid WHERE id = $1`, id, crScopeProjectA); err != nil {
				return err
			}
			once.Do(func() { close(locked) })
			<-release
			return nil
		})
	}()
	<-locked
	if txErr != nil {
		t.Fatalf("the project edit transaction: %v", txErr)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.patchState(id, domain.ChangeRequestStateAssess)
		done <- err
	}()
	blocked := false
	for i := 0; i < 100 && !blocked; i++ {
		var n int
		if err := f.pool.QueryRow(f.sys,
			`SELECT COUNT(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query ILIKE '%FROM work_item%FOR NO KEY UPDATE%'`).Scan(&n); err != nil {
			t.Fatalf("look for the blocked request: %v", err)
		}
		blocked = n > 0
		if !blocked {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if !blocked {
		t.Fatal("Request Approval never blocked on the work_item lock")
	}
	releaseOnce.Do(func() { close(release) })
	wg.Wait()
	if txErr != nil {
		t.Fatalf("the project edit transaction: %v", txErr)
	}
	if err := <-done; err != nil {
		t.Fatalf("Request Approval after the project was chosen: %v", err)
	}
	f.expect(id, "after Request Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
}

// A PATCH that carries a state and a decision in progress on the same change
// request do not deadlock. A decision locks change_request first and then INSERTs
// approval_stage / approval_stage_approver rows, whose foreign keys take FOR KEY
// SHARE on the work_item row; the PATCH holds the work_item lock while it waits
// for the change_request one. The work_item lock is FOR NO KEY UPDATE (what the
// PATCH's own UPDATE takes) so the two are compatible; FOR UPDATE would make the
// decision wait for the PATCH and the PATCH for the decision, which Postgres
// answers by aborting one of them (verified: with FOR UPDATE this test fails with
// SQLSTATE 40P01). Deterministic: the transaction standing in for the decision
// takes its lock, waits until the PATCH is waiting on it, and only then does the
// INSERT.
func TestChangeRequestLockIntegration_APatchAndADecisionDoNotDeadlock(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)

	locked, proceed := make(chan struct{}), make(chan struct{})
	var proceedOnce, lockedOnce sync.Once
	t.Cleanup(func() { proceedOnce.Do(func() { close(proceed) }) })
	var wg sync.WaitGroup
	var txErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer lockedOnce.Do(func() { close(locked) })
		txErr = f.scoped.InTx(f.sys, func(tx pgx.Tx) error {
			// What a decision does first.
			if _, err := tx.Exec(f.sys, `SELECT 1 FROM change_request WHERE id = $1 FOR UPDATE`, id); err != nil {
				return err
			}
			lockedOnce.Do(func() { close(locked) })
			<-proceed
			// ...and then provisions / cancels approval rows, which reference work_item.
			_, err := tx.Exec(f.sys,
				`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id)
				 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1)`, id)
			return err
		})
	}()
	<-locked
	if txErr != nil {
		t.Fatalf("the decision's transaction: %v", txErr)
	}

	patched := make(chan error, 1)
	go func() {
		_, err := f.patchState(id, domain.ChangeRequestStateCanceled)
		patched <- err
	}()
	waiting := false
	for i := 0; i < 100 && !waiting; i++ {
		var n int
		if err := f.pool.QueryRow(f.sys,
			`SELECT COUNT(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query ILIKE '%FROM change_request%FOR UPDATE%'`).Scan(&n); err != nil {
			t.Fatalf("look for the waiting PATCH: %v", err)
		}
		waiting = n > 0
		if !waiting {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if !waiting {
		t.Fatal("the PATCH never waited on the change_request lock")
	}
	proceedOnce.Do(func() { close(proceed) })
	wg.Wait()
	if txErr != nil {
		t.Fatalf("the decision's INSERT after the PATCH took the work_item lock: %v (a deadlock between the two?)", txErr)
	}
	if err := <-patched; err != nil {
		t.Fatalf("the PATCH: %v", err)
	}
	f.expect(id, "after the PATCH", "CANCELED")
}
