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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// "Refuse Request Approval": a change that has a customer box ticked but nobody
// who can be asked must not be sent for approval. With no staff action that
// answers for the customer, such a change reaches Customer Approval / Customer
// Review with nobody to answer and can only be cancelled (or rolled back from
// Review). So Request Approval ({state: assess}) is refused, and so is turning a
// box ON after it (the add-only rule), whenever the Customer Project has nobody
// the customer stage would ask -- the SAME test provisionCustomerStage applies
// (customerGroupCanBeAsked: registered portal-user contacts, active users, never
// the requester). Same harness as the other crFlow tests (DSN-gated by
// CHANGE_REQUEST_TEST_DSN; run it as the superuser AND as csm_app, and on a copy
// whose approval tables have the sync's enum columns).

const (
	nobodyTail        = " required but nobody on this project can be asked (no registered contact other than the requester): register a contact for the project first"
	nobodyMsgApproval = "customer approval is" + nobodyTail
	nobodyMsgReview   = "customer review is" + nobodyTail
	nobodyMsgBoth     = "customer approval and customer review are" + nobodyTail
)

// crStandInUserID is the throw-away contact standInContact registers.
const crStandInUserID = "3bbbbbbb-0000-0000-0000-0000000000e1"

// nobodyMsg is the refusal for the boxes named.
func nobodyMsg(approval, review bool) string {
	switch {
	case approval && review:
		return nobodyMsgBoth
	case review:
		return nobodyMsgReview
	}
	return nobodyMsgApproval
}

// wantExact asserts err is a 400 whose message is exactly msg.
func (f *crFlow) wantExact(what string, err error, msg string) {
	f.t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		f.t.Fatalf("%s: err = %v (%T), want a *apierror.ValidationError %q", what, err, err, msg)
	}
	if ve.Msg != msg {
		f.t.Fatalf("%s: message = %q, want %q", what, ve.Msg, msg)
	}
}

// projectOf is the Customer Project stored on the change.
func (f *crFlow) projectOf(id string) string {
	f.t.Helper()
	var p *string
	if err := f.scoped.QueryRow(f.sys, `SELECT project_id::text FROM work_item WHERE id = $1`, id).Scan(&p); err != nil {
		f.t.Fatalf("read the project: %v", err)
	}
	if p == nil {
		return ""
	}
	return *p
}

// addContact adds the user as a contact of the project in the given
// project_contact state ("REGISTERED", "INVITED", "RE-INVITED", "DEACTIVATED"),
// holding the project role (a seedScope group: "PORTAL_USER" / "SECURITY_CONTACT"),
// on a fresh account contact. userID "" makes a contact whose account contact
// matches no "user" row at all.
func (f *crFlow) addContact(projectID, userID, state, role string) {
	f.t.Helper()
	var accountID string
	if err := f.scoped.QueryRow(f.sys, `SELECT account_id::text FROM project WHERE id = $1`, projectID).Scan(&accountID); err != nil {
		f.t.Fatalf("read the project's account: %v", err)
	}
	userName := "no-user-record-for-this-contact@example.com"
	if userID != "" {
		if err := f.scoped.QueryRow(f.sys, `SELECT user_name FROM "user" WHERE id = $1`, userID).Scan(&userName); err != nil {
			f.t.Fatalf("read user %s: %v", userID, err)
		}
	}
	var acID, pcID string
	if err := f.scoped.QueryRow(f.sys,
		`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1, $2) RETURNING id::text`, userName, accountID).Scan(&acID); err != nil {
		f.t.Fatalf("seed account_contact: %v", err)
	}
	if err := f.scoped.QueryRow(f.sys,
		`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1, $2, $3, $4::project_contact_state_enum) RETURNING id::text`,
		userName, acID, projectID, state).Scan(&pcID); err != nil {
		f.t.Fatalf("seed project_contact: %v", err)
	}
	f.execSQL(`INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
	           SELECT gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, id FROM project_group WHERE "group" = $2`, pcID, "CR Scope "+role)
}

// standInContact registers a throw-away contact (an active EXTERNAL user, a
// REGISTERED project contact holding PORTAL_USER, not the creator) on the project,
// so that a change that ticks a box on it is accepted by Request Approval, and
// returns leave, which deactivates that project contact again. It is how a test
// reaches a customer gate with NOBODY asked now that Request Approval refuses to
// send such a change: the residual edge, every registered contact gone AFTER
// approval was requested. Self-contained (it does not need seedScope).
func (f *crFlow) standInContact(projectID string) (leave func()) {
	f.t.Helper()
	var accountID string
	if err := f.scoped.QueryRow(f.sys, `SELECT account_id::text FROM project WHERE id = $1`, projectID).Scan(&accountID); err != nil {
		f.t.Fatalf("read the project's account: %v", err)
	}
	email := crFlowEmail(crStandInUserID)
	f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user, user_type)
	           VALUES ($1, now(), now(), 'cr-flow-test', 'cr-flow-test', $2, 'Stand In', 'Stand', 'In', $2, true, false, 'EXTERNAL'::user_type_enum)
	           ON CONFLICT (id) DO NOTHING`, crStandInUserID, email)
	var roleID, groupID, acID, pcID string
	if err := f.scoped.QueryRow(f.sys, `SELECT id::text FROM project_role WHERE role = 'PORTAL_USER'::project_role_enum LIMIT 1`).Scan(&roleID); err != nil {
		if err := f.scoped.QueryRow(f.sys,
			`INSERT INTO project_role (id, created_on, updated_on, created_by, updated_by, role)
			 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', 'PORTAL_USER'::project_role_enum) RETURNING id::text`).Scan(&roleID); err != nil {
			f.t.Fatalf("seed project_role: %v", err)
		}
	}
	if err := f.scoped.QueryRow(f.sys, `SELECT id::text FROM project_group WHERE "group" = 'CR StandIn PORTAL_USER' LIMIT 1`).Scan(&groupID); err != nil {
		if err := f.scoped.QueryRow(f.sys,
			`INSERT INTO project_group (id, created_on, updated_on, created_by, updated_by, "group")
			 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', 'CR StandIn PORTAL_USER') RETURNING id::text`).Scan(&groupID); err != nil {
			f.t.Fatalf("seed project_group: %v", err)
		}
		f.execSQL(`INSERT INTO project_group_role (id, created_on, updated_on, created_by, updated_by, project_group_id, project_role_id)
		           VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, $2::uuid)`, groupID, roleID)
	}
	if err := f.scoped.QueryRow(f.sys,
		`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1, $2) RETURNING id::text`, email, accountID).Scan(&acID); err != nil {
		f.t.Fatalf("seed account_contact: %v", err)
	}
	if err := f.scoped.QueryRow(f.sys,
		`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1, $2, $3, 'REGISTERED') RETURNING id::text`, email, acID, projectID).Scan(&pcID); err != nil {
		f.t.Fatalf("seed project_contact: %v", err)
	}
	f.execSQL(`INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
	           VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, $2::uuid)`, pcID, groupID)
	f.t.Cleanup(func() {
		_, _ = f.scoped.Exec(f.sys, `DELETE FROM project_contact WHERE id = $1::uuid`, pcID)
		_, _ = f.scoped.Exec(f.sys, `DELETE FROM account_contact WHERE id = $1::uuid`, acID)
		_, _ = f.scoped.Exec(f.sys, `DELETE FROM project_group WHERE "group" = 'CR StandIn PORTAL_USER'`)
		_, _ = f.scoped.Exec(f.sys, `DELETE FROM "user" WHERE id = $1`, crStandInUserID)
	})
	return func() {
		f.t.Helper()
		f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED'::project_contact_state_enum WHERE id = $1::uuid`, pcID)
	}
}

// requestApprovalThenContactsLeave is Request Approval for a change that is to end
// up at a customer gate with NOBODY asked. Request Approval refuses a change that
// ticks a box on a project nobody can be asked on, so the project has a stand-in
// contact when approval is requested and the stand-in is gone before the gate is
// reached -- the residual edge. Normal / Emergency only: a Standard change reaches
// its Customer Approval inside Request Approval itself (legacyInCustomerState).
func (f *crFlow) requestApprovalThenContactsLeave(id string) {
	f.t.Helper()
	leave := f.standInContact(f.projectOf(id))
	f.requestApproval(id)
	leave()
}

// legacyInCustomerState is the dead-end row an older build or a migration left
// behind: the change in the customer state, with no stage and nobody asked. The
// state is written directly, as Request Approval refuses to produce it any more.
func (f *crFlow) legacyInCustomerState(id, state string) {
	f.t.Helper()
	f.setState(id, state)
}

// ---------------------------------------------------------------------------
// The matrix: box ticked x project situation x action.
// ---------------------------------------------------------------------------

// nobodySituation is one state of the Customer Project's contacts.
type nobodySituation struct {
	name string
	// setup returns the project (nil: the change has none) after adding its contacts.
	setup func(f *crFlow) *string
	// asked: somebody can be asked.
	asked bool
}

func nobodySituations() []nobodySituation {
	c := func(f *crFlow) *string { return sp(crScopeProjectC) }
	return []nobodySituation{
		{"no project", func(f *crFlow) *string { return nil }, false},
		{"a project with no contacts", c, false},
		{"only contact is the creator", func(f *crFlow) *string {
			f.registerContact(crScopeProjectC, crScopeAccountID, crFlowCreatorID)
			return c(f)
		}, false},
		{"only an invited contact", func(f *crFlow) *string {
			f.addContact(crScopeProjectC, crScopeUserInvited, "INVITED", "PORTAL_USER")
			return c(f)
		}, false},
		{"only a re-invited contact", func(f *crFlow) *string {
			f.addContact(crScopeProjectC, crScopeUserAlice, "RE-INVITED", "PORTAL_USER")
			return c(f)
		}, false},
		{"only a deactivated contact", func(f *crFlow) *string {
			f.addContact(crScopeProjectC, crScopeUserAlice, "DEACTIVATED", "PORTAL_USER")
			return c(f)
		}, false},
		{"only a registered contact whose user is deactivated", func(f *crFlow) *string {
			f.addContact(crScopeProjectC, crScopeUserInactive, "REGISTERED", "PORTAL_USER")
			return c(f)
		}, false},
		{"only a registered contact without the portal-user role", func(f *crFlow) *string {
			f.addContact(crScopeProjectC, crScopeUserSecurity, "REGISTERED", "SECURITY_CONTACT")
			return c(f)
		}, false},
		{"only a registered contact with no user record", func(f *crFlow) *string {
			f.addContact(crScopeProjectC, "", "REGISTERED", "PORTAL_USER")
			return c(f)
		}, false},
		{"the creator and an invited contact", func(f *crFlow) *string {
			f.registerContact(crScopeProjectC, crScopeAccountID, crFlowCreatorID)
			f.addContact(crScopeProjectC, crScopeUserInvited, "INVITED", "PORTAL_USER")
			return c(f)
		}, false},
		{"one registered contact", func(f *crFlow) *string { return sp(crScopeProjectB) }, true},
		{"the creator and one registered contact", func(f *crFlow) *string {
			f.registerContact(crScopeProjectC, crScopeAccountID, crFlowCreatorID)
			f.addContact(crScopeProjectC, crScopeUserAlice, "REGISTERED", "PORTAL_USER")
			return c(f)
		}, true},
		{"several registered contacts", func(f *crFlow) *string { return sp(crScopeProjectA) }, true},
	}
}

// nobodyBoxes is one box combination.
type nobodyBoxes struct {
	name             string
	approval, review bool
}

var nobodyBoxCombinations = []nobodyBoxes{
	{"approval", true, false},
	{"review", false, true},
	{"both", true, true},
	{"none", false, false},
}

// Request Approval, for every box combination x every project situation x the two
// types that go through it differently (Normal waits in Assess, Standard has no
// internal approval and lands in Customer Approval / Scheduled): refused with the
// exact message when a box is ticked and nobody can be asked, nothing written; a
// change with no project keeps the old refusal; every other case is accepted and
// lands where it always did.
func TestChangeRequestNobodyToAskIntegration_RequestApprovalMatrix(t *testing.T) {
	for _, sit := range nobodySituations() {
		sit := sit
		t.Run(sit.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			project := sit.setup(f)
			for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard} {
				for _, b := range nobodyBoxCombinations {
					id := f.createWithProject(typ, project, b.approval, b.review)
					what := fmt.Sprintf("%s change, %s ticked", typ, b.name)
					ticked := b.approval || b.review
					refused := ticked && (project == nil || !sit.asked)
					req := domain.PatchChangeRequestRequest{State: ptrCRState(domain.ChangeRequestStateAssess)}
					if refused {
						req.Title = sp("must not be written") // a refused PATCH writes nothing, not even this
					}
					before := f.bypassSnapshot(id)
					_, err := f.patch(id, req)
					switch {
					case !ticked:
						if err != nil {
							t.Fatalf("%s: Request Approval with nothing ticked: %v", what, err)
						}
					case project == nil:
						f.wantExact(what+": no project", err, lockMsgRequestNoProj)
					case !sit.asked:
						f.wantExact(what, err, nobodyMsg(b.approval, b.review))
					default:
						if err != nil {
							t.Fatalf("%s: Request Approval with somebody to ask: %v", what, err)
						}
					}
					if refused {
						// Nothing was written, not even the rest of the request.
						if after := f.bypassSnapshot(id); after != before {
							t.Fatalf("%s: a refused Request Approval changed the change:\n  before: %s\n  after:  %s", what, before, after)
						}
						if got := f.subjectOf(id); got != crFlowSubject {
							t.Fatalf("%s: a refused Request Approval wrote the title: %q", what, got)
						}
						f.expect(id, what+": after the refusal", "NEW", "assess", "canceled")
						continue
					}
					// Accepted: where it always went.
					want := "ASSESS"
					if typ == domain.ChangeRequestTypeStandard {
						want = "SCHEDULED"
						if b.approval {
							want = "CUSTOMER_APPROVAL"
						}
					}
					if got := f.state(id); got != want {
						t.Fatalf("%s: state after Request Approval = %s, want %s", what, got, want)
					}
					if typ == domain.ChangeRequestTypeStandard && b.approval {
						if st := f.customerStages(id); len(st) != 1 || liveStages(st) != 1 {
							t.Fatalf("%s: customer stages after Request Approval = %+v, want one live", what, st)
						}
					}
				}
			}
		})
	}
}

// Request Approval that carries the project and / or the box in the same PATCH is
// judged with them (the effective values), the project refusal first.
func TestChangeRequestNobodyToAskIntegration_RequestApprovalJudgesTheRequestsOwnValues(t *testing.T) {
	f := newCustomerGroupFlow(t)
	assess := ptrCRState(domain.ChangeRequestStateAssess)

	// The box and the project are set in the very PATCH that requests approval.
	id := f.createWithProject(domain.ChangeRequestTypeNormal, nil, false, false)
	_, err := f.patch(id, domain.PatchChangeRequestRequest{State: assess, CustomerReviewRequired: boolp(true)})
	f.wantExact("review ticked, still no project", err, lockMsgRequestNoProj)
	_, err = f.patch(id, domain.PatchChangeRequestRequest{State: assess, CustomerReviewRequired: boolp(true), ProjectID: sp(crScopeProjectC), DeploymentIDs: &[]string{}})
	f.wantExact("review ticked, a project nobody can be asked on", err, nobodyMsgReview)
	f.expect(id, "after the refusals", "NEW", "assess", "canceled")
	f.wantProject(id, "after the refusals", nil)
	if _, _, _, review := f.lockStored(id); review {
		t.Fatal("a refused Request Approval stored the box")
	}
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{State: assess, CustomerReviewRequired: boolp(true), ProjectID: sp(crScopeProjectA), DeploymentIDs: &[]string{}}); err != nil {
		t.Fatalf("review ticked, a project with contacts: %v", err)
	}
	f.expect(id, "after the accepted request", "ASSESS", "canceled")

	// A stored box on a project that is changed in the same PATCH: the project in
	// effect is the request's, not the stored one.
	id = f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	_, err = f.patch(id, domain.PatchChangeRequestRequest{State: assess, ProjectID: sp(crScopeProjectC), DeploymentIDs: &[]string{}})
	f.wantExact("the stored box, the request moves the change to a project nobody can be asked on", err, nobodyMsgApproval)
	f.wantProject(id, "after the refusal", sp(crScopeProjectA))
	// ... and the box cleared in the same PATCH needs nobody.
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{State: assess, ProjectID: sp(crScopeProjectC), DeploymentIDs: &[]string{}, CustomerApprovalRequired: boolp(false)}); err != nil {
		t.Fatalf("the box cleared in the PATCH that requests approval: %v", err)
	}
	f.expect(id, "after the box was cleared", "ASSESS", "canceled")

	// A resend of {state: assess} on a change that left New is the no-op it always
	// was, whatever its project's contacts have become.
	idDone := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	f.requestApproval(idDone)
	f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED'::project_contact_state_enum WHERE project_id = $1`, crScopeProjectA)
	if _, err := f.patchState(idDone, domain.ChangeRequestStateAssess); err != nil {
		t.Fatalf("a resent {state: assess} after the project's contacts left: %v", err)
	}
	f.expect(idDone, "after the resend", "ASSESS", "canceled")
}

// Ticking a box ON after Request Approval (the add-only rule), through the states
// in which it is still allowed: refused with the same message when nobody can be
// asked (naming only the boxes being turned on), accepted when somebody can. A
// change with no project keeps the add-only rule's own message; unticking and
// resends are never this rule's business.
func TestChangeRequestNobodyToAskIntegration_TickingABoxOnAfterRequestApproval(t *testing.T) {
	for _, sit := range nobodySituations() {
		sit := sit
		t.Run(sit.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			project := sit.setup(f)
			for _, b := range nobodyBoxCombinations {
				// Nothing is ticked when approval is requested, so Request Approval is
				// accepted on any project.
				id := f.createWithProject(domain.ChangeRequestTypeNormal, project, false, false)
				f.requestApproval(id)
				f.expect(id, "after Request Approval", "ASSESS", "canceled")
				what := fmt.Sprintf("%s ticked in Assess", b.name)

				ticked := b.approval || b.review
				refused := ticked && (project == nil || !sit.asked)
				req := domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(b.approval), CustomerReviewRequired: boolp(b.review)}
				if refused {
					req.Title = sp("must not be written") // a refused PATCH writes nothing, not even this
				}
				before := f.bypassSnapshot(id)
				_, err := f.patch(id, req)
				switch {
				case !ticked:
					// Both sent unticked on unticked boxes: the stored value resent.
					if err != nil {
						t.Fatalf("%s: resending the unticked boxes: %v", what, err)
					}
				case project == nil:
					f.wantValidationError(what, err, "cannot be turned on: this change request has no Customer Project")
				case !sit.asked:
					f.wantExact(what, err, nobodyMsg(b.approval, b.review))
				default:
					if err != nil {
						t.Fatalf("%s: turning the box on with somebody to ask: %v", what, err)
					}
					if _, _, a, r := f.lockStored(id); a != b.approval || r != b.review {
						t.Fatalf("%s: stored boxes = %v/%v", what, a, r)
					}
				}
				if refused {
					if after := f.bypassSnapshot(id); after != before {
						t.Fatalf("%s: a refused PATCH changed the change:\n  before: %s\n  after:  %s", what, before, after)
					}
					if got := f.subjectOf(id); got != crFlowSubject {
						t.Fatalf("%s: a refused PATCH wrote the title: %q", what, got)
					}
					if _, _, a, r := f.lockStored(id); a || r {
						t.Fatalf("%s: a refused PATCH stored the boxes %v/%v", what, a, r)
					}
				}
			}
		})
	}
}

// The same rule in the later states in which a box can still be turned on, and the
// gates' own refusals keep their precedence: past the approval gate the box's own
// message answers (not this one), a review box is still judged in Implement and
// Review, and a box already ticked is never re-judged.
func TestChangeRequestNobodyToAskIntegration_TickingAfterNewInTheLaterStates(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), false, false)
	f.requestApproval(id)

	// Authorize: both boxes can still be turned on, and nobody can be asked.
	f.setState(id, "AUTHORIZE")
	_, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)})
	f.wantExact("approval ticked in Authorize", err, nobodyMsgApproval)
	// Scheduled / Implement: the approval gate is passed, its own message answers first.
	f.setState(id, "SCHEDULED")
	_, err = f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)})
	f.wantValidationError("approval ticked in Scheduled", err, "customerApprovalRequired can no longer be changed")
	for _, st := range []string{"SCHEDULED", "IMPLEMENT", "REVIEW"} {
		f.setState(id, st)
		_, err = f.patch(id, domain.PatchChangeRequestRequest{CustomerReviewRequired: boolp(true)})
		f.wantExact("review ticked in "+st, err, nobodyMsgReview)
		// Both at once, the approval box past its gate: the approval box's message first.
		_, err = f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true), CustomerReviewRequired: boolp(true)})
		f.wantValidationError("both ticked in "+st, err, "customerApprovalRequired can no longer be changed")
	}
	// Customer Review and after: the review gate is passed too.
	f.setState(id, "CUSTOMER_REVIEW")
	_, err = f.patch(id, domain.PatchChangeRequestRequest{CustomerReviewRequired: boolp(true)})
	f.wantValidationError("review ticked in Customer Review", err, "customerReviewRequired can no longer be changed")

	// With somebody to ask the same ticks are accepted in the same states.
	id = f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	f.requestApproval(id)
	f.setState(id, "AUTHORIZE")
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)}); err != nil {
		t.Fatalf("approval ticked in Authorize with somebody to ask: %v", err)
	}
	f.setState(id, "REVIEW")
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerReviewRequired: boolp(true)}); err != nil {
		t.Fatalf("review ticked in Review with somebody to ask: %v", err)
	}

	// A box that is already ticked is never re-judged: its project's contacts left
	// after it was ticked (the residual edge), and resending it, or any other
	// edit, is accepted.
	f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED'::project_contact_state_enum WHERE project_id = $1`, crScopeProjectA)
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true), CustomerReviewRequired: boolp(true), Title: sp("renamed")}); err != nil {
		t.Fatalf("resending the ticked boxes after the contacts left: %v", err)
	}
	if got := f.subjectOf(id); got != "renamed" {
		t.Fatalf("the resend did not write the rest of the PATCH: %q", got)
	}
	// Unticking is the lock's refusal, not this one.
	_, err = f.patch(id, domain.PatchChangeRequestRequest{CustomerReviewRequired: boolp(false)})
	f.wantValidationError("unticking", err, "can no longer be turned off")
}

// An asked contact is asked at the gate and can answer: after an ACCEPTED Request
// Approval the customer stage asks the project's registered contacts (never the
// creator, never an invited one), they are told they can answer, and their answer
// moves the change on. Approval, review and both, on a Normal and a Standard change.
func TestChangeRequestNobodyToAskIntegration_TheContactIsAskedAtTheGateAndCanAnswer(t *testing.T) {
	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard} {
		for _, b := range nobodyBoxCombinations[:3] {
			typ, b := typ, b
			t.Run(fmt.Sprintf("%s/%s", typ, b.name), func(t *testing.T) {
				f := newCustomerGroupFlow(t)
				// Project C: the creator and an invited contact (neither is asked) and
				// Alice (registered): the one who is asked.
				f.registerContact(crScopeProjectC, crScopeAccountID, crFlowCreatorID)
				f.addContact(crScopeProjectC, crScopeUserInvited, "INVITED", "PORTAL_USER")
				f.addContact(crScopeProjectC, crScopeUserAlice, "REGISTERED", "PORTAL_USER")
				id := f.createWithProject(typ, sp(crScopeProjectC), b.approval, b.review)
				f.requestApproval(id)

				if b.approval {
					if typ == domain.ChangeRequestTypeNormal {
						f.expect(id, "after Request Approval", "ASSESS", "canceled")
						f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
					} else {
						f.expect(id, "after Request Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
					}
					st := f.customerStages(id)
					if len(st) != 1 || st[0].label != stageCustApproval {
						t.Fatalf("customer stages at the gate = %+v, want one Customer Approval", st)
					}
					// Alice is asked; the creator (a registered contact too) is listed
					// cancelled like on every stage; the invited contact is not a member.
					assertApprovers(t, "Customer Approval", st[0].approvers, map[string]string{crScopeUserAlice: "REQUESTED", crFlowCreatorID: "CANCELLED"})
					f.wantCanAnswer(id, "at Customer Approval", true, crScopeUserAlice)
					f.wantCanAnswer(id, "at Customer Approval", false, crFlowCreatorID)
					f.wantApproveRefused("an invited contact", id, crScopeUserInvited)
					if _, err := f.approveAs(id, crScopeUserAlice, true); err != nil {
						t.Fatalf("the contact's approval: %v", err)
					}
					f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
				} else if typ == domain.ChangeRequestTypeNormal {
					f.expect(id, "after Request Approval", "ASSESS", "canceled")
					f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
				} else {
					f.expect(id, "after Request Approval", "SCHEDULED", "implement", "canceled")
				}
				if b.review {
					f.driveToCustomerReview(id)
					st := f.customerStages(id)
					last := st[len(st)-1]
					if last.label != stageCustReview {
						t.Fatalf("customer stages at the review gate = %+v, want a Customer Review last", st)
					}
					assertApprovers(t, "Customer Review", last.approvers, map[string]string{crScopeUserAlice: "REQUESTED", crFlowCreatorID: "CANCELLED"})
					f.wantCanAnswer(id, "at Customer Review", true, crScopeUserAlice)
					if _, err := f.reviewAs(id, crScopeUserAlice, true); err != nil {
						t.Fatalf("the contact's review: %v", err)
					}
					f.expect(id, "after the customer's review", "CLOSED")
				}
			})
		}
	}
}

// The residual edge, documented and not built for: every registered contact
// deactivated AFTER Request Approval. The change reaches the customer gate with
// nobody asked and is the dead end it always was: staff keep Cancel (and
// Re-schedule from Customer Approval, Roll back from Customer Review), no
// customer stage is written, nothing is stamped, and a contact who registers later
// is picked up by restating the project. Nothing in this rule judges a change that
// is beyond New.
func TestChangeRequestNobodyToAskIntegration_ResidualEdgeContactsLeaveAfterRequestApproval(t *testing.T) {
	t.Run("Customer Approval", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.requestApproval(id)
		f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED'::project_contact_state_enum WHERE project_id = $1`, crScopeProjectA)
		f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("a customer stage was provisioned with every contact deactivated: %+v", f.customerStages(id))
		}
		f.wantRefusedAndUnchanged("manual scheduled with nobody asked", id, domain.PatchChangeRequestRequest{State: ptrCRState("scheduled")}, bypassApprovalMsg)
		// Nothing else about the change is judged by the refusal: other edits go through.
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{Title: sp("renamed"), CustomerApprovalRequired: boolp(true)}); err != nil {
			t.Fatalf("an edit of a change beyond New whose contacts left: %v", err)
		}
		f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
	})
	t.Run("Customer Review", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED'::project_contact_state_enum WHERE project_id = $1`, crScopeProjectA)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
		f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "rollback", "canceled")
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("a customer review stage was provisioned with every contact deactivated: %+v", f.customerStages(id))
		}
		f.step(id, domain.ChangeRequestStateRollback, "ROLLBACK")
	})
}

// The legacy dead-end row (a change a migration or an older build left in a
// customer state with nobody asked) is never re-judged: staff still have Cancel /
// Re-schedule / Roll back, every PATCH that does not turn a box on goes through,
// and the stage is still provisioned for a contact once there is one.
func TestChangeRequestNobodyToAskIntegration_ALegacyDeadEndRowIsNotRejudged(t *testing.T) {
	f := newCustomerGroupFlow(t)
	approval := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), true, false)
	f.legacyInCustomerState(approval, "CUSTOMER_APPROVAL")
	review := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectC), false, true)
	f.legacyInCustomerState(review, "CUSTOMER_REVIEW")
	f.expect(approval, "legacy, Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.expect(review, "legacy, Customer Review", "CUSTOMER_REVIEW", "rollback", "canceled")

	for _, id := range []string{approval, review} {
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{Title: sp("renamed"), WorkNote: sp("a note")}); err != nil {
			t.Fatalf("an edit of a legacy dead-end row: %v", err)
		}
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{ProjectID: sp(crScopeProjectC), DeploymentIDs: &[]string{}}); err != nil {
			t.Fatalf("restating the project of a legacy dead-end row: %v", err)
		}
	}
	// A box that is turned on is judged, wherever the row is.
	_, err := f.patch(approval, domain.PatchChangeRequestRequest{CustomerReviewRequired: boolp(true)})
	f.wantExact("turning the review box on of a legacy row in Customer Approval", err, nobodyMsgReview)
	// Re-schedule is what the rule never touches.
	f.setPlanned(approval, rsStart1, rsEnd1)
	if err := f.reschedule(approval, sp(rsStart2), sp(rsEnd2)); err != nil {
		t.Fatalf("Re-schedule of a legacy dead-end row: %v", err)
	}
	f.step(review, domain.ChangeRequestStateRollback, "ROLLBACK")
}

// A migrated-shaped change in New is unaffected: its number is CHG-prefixed, the
// ServiceNow requirement flags are set (they are ServiceNow's record, not our
// boxes), our own columns are false (migration 0189's default), a legacy customer
// group is stored, and the stage the sync wrote has no label. Request Approval
// reads OUR boxes only, so it is accepted whatever the project's contacts are, and
// nothing sync-owned is written by it.
func TestChangeRequestNobodyToAskIntegration_AMigratedShapedChangeInNewIsUnaffected(t *testing.T) {
	for _, sit := range []struct {
		name    string
		project func(f *crFlow) *string
	}{
		{"a project with no contacts", func(f *crFlow) *string { return sp(crScopeProjectC) }},
		{"only the creator", func(f *crFlow) *string {
			f.registerContact(crScopeProjectC, crScopeAccountID, crFlowCreatorID)
			return sp(crScopeProjectC)
		}},
		{"no project", func(f *crFlow) *string { return nil }},
		{"a project with contacts", func(f *crFlow) *string { return sp(crScopeProjectA) }},
	} {
		sit := sit
		t.Run(sit.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.createWithProject(domain.ChangeRequestTypeNormal, sit.project(f), false, false)
			f.execSQL(`UPDATE work_item SET number = $2 WHERE id = $1`, id, "CHG9990001")
			f.execSQL(`UPDATE change_request SET customer_approval_required = false, customer_review_required = false,
			                  is_customer_approval_required = true, is_customer_review_required = true, customer_group_id = $2::uuid WHERE id = $1`, id, seededGroupID)
			// The sync's own stage: no label, an uppercase status, the legacy group.
			f.execSQL(`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status)
			           VALUES (gen_random_uuid(), now() - interval '1 hour', now(), 'sn-sync', 'sn-sync', $1, $2::uuid, 'NOT_REQUIRED')`, id, seededGroupID)

			if _, err := f.patchState(id, domain.ChangeRequestStateAssess); err != nil {
				t.Fatalf("Request Approval of a migrated-shaped change in New: %v", err)
			}
			f.expect(id, "after Request Approval", "ASSESS", "canceled")
			var approvalFlag, reviewFlag, ours1, ours2 bool
			var group *string
			if err := f.scoped.QueryRow(f.sys,
				`SELECT is_customer_approval_required, is_customer_review_required, customer_approval_required, customer_review_required, customer_group_id::text
				 FROM change_request WHERE id = $1`, id).Scan(&approvalFlag, &reviewFlag, &ours1, &ours2, &group); err != nil {
				t.Fatalf("read the flags: %v", err)
			}
			if !approvalFlag || !reviewFlag || ours1 || ours2 || group == nil || *group != seededGroupID {
				t.Fatalf("Request Approval wrote a sync-owned or requirement column: sync %v/%v ours %v/%v group %v", approvalFlag, reviewFlag, ours1, ours2, group)
			}
		})
	}
}

// The refusal predicts EXACTLY the "nobody asked" outcome of the provisioning,
// for every situation: a change that ticks the box is refused at Request Approval
// if and only if the very same change, put in Customer Approval as a legacy row
// and then re-derived (restating its project, which runs provisionCustomerStage),
// gets no stage -- and a stage with the contacts asked when it is accepted. One
// definition, asked by both.
func TestChangeRequestNobodyToAskIntegration_TheRefusalPredictsTheProvisioningOutcome(t *testing.T) {
	for _, sit := range nobodySituations() {
		sit := sit
		if sit.name == "no project" {
			continue // the other refusal's case: nothing to derive
		}
		t.Run(sit.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			project := sit.setup(f)

			refused := f.createWithProject(domain.ChangeRequestTypeNormal, project, true, false)
			_, raErr := f.patchState(refused, domain.ChangeRequestStateAssess)
			wasRefused := raErr != nil
			if wasRefused != !sit.asked {
				t.Fatalf("Request Approval refused = %v (%v), want %v", wasRefused, raErr, !sit.asked)
			}

			legacy := f.createWithProject(domain.ChangeRequestTypeNormal, project, true, false)
			f.legacyInCustomerState(legacy, "CUSTOMER_APPROVAL")
			f.setProject(legacy, *project)
			provisioned := len(f.customerStages(legacy)) > 0
			if provisioned == wasRefused {
				t.Fatalf("the refusal (%v) and the provisioning (stage written: %v) disagree", wasRefused, provisioned)
			}
			if provisioned && liveStages(f.customerStages(legacy)) != 1 {
				t.Fatalf("customer stages of the provisioned change = %+v, want one live", f.customerStages(legacy))
			}
		})
	}
}
