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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The Emergency rule against Postgres (change_request_emergency.go): an Emergency change
// takes no customer step, and its one approval is the CAB's. Same harness as every
// TestChangeRequest*Integration_* test (crFlow, DSN-gated by CHANGE_REQUEST_TEST_DSN, run
// as a superuser and as the non-superuser csm_app).
//
// Two origins of data are covered, because the rule has to hold for both:
//
//   - changes created here: the two customer boxes are refused (create, PATCH, re-type);
//   - changes that already exist -- a "legacy" row from before the rule, a MIGRATED
//     one (whose requirement flags is_customer_*_required are the sync's, and whose single
//     stage carries no label), a row an earlier build gave an "ECAB Approval" stage: the flow
//     ignores whatever boxes they hold, decides their one stage as the CAB's, and never
//     provisions a customer stage for them.

const emergencyMsgPrefix = "Emergency changes proceed without customer consent, so customer approval and customer review cannot be required"

func (f *crFlow) wantEmergencyRefusal(what string, err error, fields ...string) {
	f.t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		f.t.Fatalf("%s: err = %v (%T), want a *apierror.ValidationError", what, err, err)
	}
	if !strings.HasPrefix(ve.Msg, emergencyMsgPrefix) {
		f.t.Fatalf("%s: message %q does not start with the reason %q", what, ve.Msg, emergencyMsgPrefix)
	}
	for _, field := range fields {
		if !strings.Contains(ve.Msg, field) {
			f.t.Fatalf("%s: message %q does not name %s", what, ve.Msg, field)
		}
	}
}

func (f *crFlow) storedModel(id string) string {
	f.t.Helper()
	var m string
	if err := f.scoped.QueryRow(f.sys, `SELECT COALESCE(change_model::text, '') FROM change_request WHERE id = $1`, id).Scan(&m); err != nil {
		f.t.Fatalf("read change_model: %v", err)
	}
	return m
}

func (f *crFlow) workItemCount() int {
	f.t.Helper()
	var n int
	if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM work_item WHERE subject = $1`, crFlowSubject).Scan(&n); err != nil {
		f.t.Fatalf("count change requests: %v", err)
	}
	return n
}

// The two sync-owned requirement flags, as the sync writes them.
func (f *crFlow) syncFlags(id string) (approval, review bool) {
	f.t.Helper()
	if err := f.scoped.QueryRow(f.sys,
		`SELECT COALESCE(is_customer_approval_required, false), COALESCE(is_customer_review_required, false) FROM change_request WHERE id = $1`, id).Scan(&approval, &review); err != nil {
		f.t.Fatalf("read the sync's customer flags: %v", err)
	}
	return approval, review
}

// A create of an Emergency change with either box ticked is a 400 on every create path, and
// nothing is written. Normal and Standard keep their boxes; an Emergency change with neither (or
// both explicitly off) is created.
func TestChangeRequestEmergencyIntegration_CreateRefusesTheBoxes(t *testing.T) {
	f := newCustomerGroupFlow(t)
	emergency := domain.ChangeRequestTypeEmergency
	g := crFlowGroupID
	yes, no := boolp(true), boolp(false)
	create := func(typ domain.ChangeRequestType, approval, review *bool) error {
		_, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
			Subject: crFlowSubject, Type: &typ, GroupID: &g, ProjectID: sp(crScopeProjectA), CustomerApprovalRequired: approval, CustomerReviewRequired: review,
		}, crFlowEmail(crFlowCreatorID))
		return err
	}
	for _, tc := range []struct {
		name             string
		approval, review *bool
		fields           []string
	}{
		{"approval", yes, nil, []string{"customerApprovalRequired"}},
		{"review", nil, yes, []string{"customerReviewRequired"}},
		{"approval, review off", yes, no, []string{"customerApprovalRequired"}},
		{"both", yes, yes, []string{"customerApprovalRequired", "customerReviewRequired"}},
	} {
		before := f.workItemCount()
		f.wantEmergencyRefusal("plain create, "+tc.name, create(emergency, tc.approval, tc.review), tc.fields...)
		// The create that calls the previous system first runs the same check before it writes anything.
		_, err := f.repo.CreateChangeRequestFromServiceNow(f.sys, domain.CreateChangeRequestRequest{
			Subject: crFlowSubject, Type: &emergency, GroupID: &g, CustomerApprovalRequired: tc.approval, CustomerReviewRequired: tc.review,
		}, "3ccccccc-0000-0000-0000-00000000e001", "CHG-EMERGENCY-RULE-1", crFlowEmail(crFlowCreatorID))
		f.wantEmergencyRefusal("previous-system-first create, "+tc.name, err, tc.fields...)
		if after := f.workItemCount(); after != before {
			t.Fatalf("%s: a refused create left %d change request(s) behind", tc.name, after-before)
		}
	}
	if err := create(emergency, nil, nil); err != nil {
		t.Fatalf("an Emergency change with no box: %v", err)
	}
	if err := create(emergency, no, no); err != nil {
		t.Fatalf("an Emergency change with both boxes explicitly off: %v", err)
	}
	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard} {
		if err := create(typ, yes, yes); err != nil {
			t.Fatalf("a %s change with both boxes: %v", typ, err)
		}
	}
}

// On a stored Emergency change a PATCH cannot turn a box on, in any state and with or without a
// Customer Project, and nothing of the request is written. A write of the value a box already
// holds is the no-op it is everywhere in the lock, and the rest of the request goes through.
func TestChangeRequestEmergencyIntegration_BoxesAreRefusedInEveryState(t *testing.T) {
	f := newCustomerGroupFlow(t)
	for _, state := range []string{"", "NEW", "ASSESS", "AUTHORIZE", "SCHEDULED", "IMPLEMENT", "REVIEW", "CLOSED", "CANCELED"} {
		for _, project := range []*string{nil, sp(crScopeProjectA)} {
			id := f.createWithProject(domain.ChangeRequestTypeEmergency, project, false, false)
			f.setState(id, state)
			label := state + "/project=" + lockDeref(project)
			for _, req := range []struct {
				what   string
				req    domain.PatchChangeRequestRequest
				fields []string
			}{
				{"approval", domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)}, []string{"customerApprovalRequired"}},
				{"review", domain.PatchChangeRequestRequest{CustomerReviewRequired: boolp(true)}, []string{"customerReviewRequired"}},
				{"both", domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true), CustomerReviewRequired: boolp(true)}, []string{"customerApprovalRequired", "customerReviewRequired"}},
			} {
				r := req.req
				r.Title = sp("must not be written")
				_, err := f.patch(id, r)
				f.wantEmergencyRefusal(label+" "+req.what, err, req.fields...)
				if _, _, a, rv := f.lockStored(id); a || rv {
					t.Fatalf("%s %s: a refused PATCH left the boxes %v/%v", label, req.what, a, rv)
				}
				if got := f.subjectOf(id); got != crFlowSubject {
					t.Fatalf("%s %s: a refused PATCH wrote the rest of the request: %q", label, req.what, got)
				}
			}
			// Turning a box off / resending the type: no change, accepted.
			emergency := domain.ChangeRequestTypeEmergency
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(false), CustomerReviewRequired: boolp(false), Type: &emergency, Title: sp("the whole form, sent back")}); err != nil {
				t.Fatalf("%s: resending the stored values: %v", label, err)
			}
			if got := f.subjectOf(id); got != "the whole form, sent back" {
				t.Fatalf("%s: the rest of the resend was not written: %q", label, got)
			}
		}
	}
}

// A legacy Emergency row (before the rule, or migrated) can carry a ticked box: a whole-form resend of
// what it holds is a no-op, turning the OTHER box on is refused, and untick follows the lock (add-only
// after New).
func TestChangeRequestEmergencyIntegration_ALegacyTickedBoxIsNotRewritten(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeEmergency, sp(crScopeProjectA), false, false)
	f.execSQL(`UPDATE change_request SET customer_approval_required = true WHERE id = $1`, id)
	emergency := domain.ChangeRequestTypeEmergency
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true), Type: &emergency, Title: sp("resent")}); err != nil {
		t.Fatalf("resending the stored ticked box of a legacy Emergency change: %v", err)
	}
	_, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true), CustomerReviewRequired: boolp(true)})
	f.wantEmergencyRefusal("turning the review box on", err, "customerReviewRequired")
	if _, _, a, r := f.lockStored(id); !a || r {
		t.Fatalf("the stored boxes read %v/%v, want the legacy true/false untouched", a, r)
	}
	// New: any edit is free, so the legacy box may be turned off.
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(false)}); err != nil {
		t.Fatalf("turning a legacy box off in New: %v", err)
	}
}

// A change cannot be re-typed INTO Emergency while a box stays ticked, whatever the box (and in the same
// request the boxes can be turned off); after Request Approval the type is frozen, which keeps its own
// message; an Emergency change may be re-typed OUT of Emergency, with a box, in the same request.
func TestChangeRequestEmergencyIntegration_RetypingIntoEmergency(t *testing.T) {
	f := newCustomerGroupFlow(t)
	emergency, normal := domain.ChangeRequestTypeEmergency, domain.ChangeRequestTypeNormal
	for _, from := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard} {
		for _, boxes := range []struct {
			name             string
			approval, review bool
			fields           []string
		}{
			{"approval", true, false, []string{"customerApprovalRequired"}},
			{"review", false, true, []string{"customerReviewRequired"}},
			{"both", true, true, []string{"customerApprovalRequired", "customerReviewRequired"}},
		} {
			id := f.createWithProject(from, sp(crScopeProjectA), boxes.approval, boxes.review)
			name := string(from) + "/" + boxes.name
			_, err := f.patch(id, domain.PatchChangeRequestRequest{Type: &emergency, Title: sp("must not be written")})
			f.wantEmergencyRefusal("re-typing "+name, err, boxes.fields...)
			if !strings.Contains(err.Error(), "before changing the type to emergency") {
				t.Fatalf("%s: message %q does not say what to do", name, err.Error())
			}
			if got, want := f.storedModel(id), strings.ToUpper(string(from)); got != want {
				t.Fatalf("%s: stored model after the refusal = %s, want %s", name, got, want)
			}
			if got := f.subjectOf(id); got != crFlowSubject {
				t.Fatalf("%s: a refused re-type wrote the rest of the request: %q", name, got)
			}
			// The boxes off in the same request: accepted, and the change is an Emergency one.
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{Type: &emergency, CustomerApprovalRequired: boolp(false), CustomerReviewRequired: boolp(false)}); err != nil {
				t.Fatalf("%s: re-typing with the boxes cleared: %v", name, err)
			}
			if got := f.storedModel(id); got != "EMERGENCY" {
				t.Fatalf("%s: stored model = %s, want EMERGENCY", name, got)
			}
			if _, _, a, r := f.lockStored(id); a || r {
				t.Fatalf("%s: stored boxes %v/%v after re-typing, want both off", name, a, r)
			}
			// ...and back out of Emergency with a box, in one request.
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{Type: &normal, CustomerApprovalRequired: boolp(true)}); err != nil {
				t.Fatalf("%s: re-typing out of Emergency with a box: %v", name, err)
			}
		}
	}
	// Only one box ticked and cleared: the other one still stops it.
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	_, err := f.patch(id, domain.PatchChangeRequestRequest{Type: &emergency, CustomerApprovalRequired: boolp(false)})
	f.wantEmergencyRefusal("re-typing with one box left", err, "customerReviewRequired")
	// A change with no box is re-typed freely.
	id = f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{Type: &emergency}); err != nil {
		t.Fatalf("re-typing a change with no box: %v", err)
	}
	// After Request Approval the type is frozen and says so, as it did before this rule.
	id = f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.requestApproval(id)
	_, err = f.patch(id, domain.PatchChangeRequestRequest{Type: &emergency})
	f.wantValidationError("re-typing after Request Approval", err, "type can no longer be changed")
}

// A row that carries the boxes anyway -- from before the rule, or migrated -- is acted on without the
// customer: it can be sent for approval with a box ticked and no project (nobody is to be asked), goes
// CAB -> Scheduled, and Customer Review is refused with the Emergency reason. The GitHub sync's state
// writer reads the review gate the same way. A Normal change with the same boxes keeps asking the
// customer (the controls).
func TestChangeRequestEmergencyIntegration_LegacyBoxesAreIgnoredByTheGate(t *testing.T) {
	t.Run("Request Approval with a box ticked and no project", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeEmergency, nil, false, false)
		f.execSQL(`UPDATE change_request SET customer_approval_required = true, customer_review_required = true WHERE id = $1`, id)
		f.requestApproval(id) // would be refused for a Normal change: nobody to ask
		f.expect(id, "after Request Approval", "AUTHORIZE", "canceled")
		if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
			t.Fatalf("CAB approval: %v", err)
		}
		f.expect(id, "after the CAB approval", "SCHEDULED", "implement", "canceled")
		// The control: the same boxes on a Normal change are refused.
		n := f.createWithProject(domain.ChangeRequestTypeNormal, nil, true, false)
		_, err := f.patchState(n, domain.ChangeRequestStateAssess)
		f.wantValidationError("Request Approval of a Normal change with a box and no project", err, "no Customer Project is set")
	})
	t.Run("the GitHub sync's state writer", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		gh := repository.NewGithubMutationRepository(f.scoped)
		id := f.createWithProject(domain.ChangeRequestTypeEmergency, sp(crScopeProjectA), false, false)
		f.execSQL(`UPDATE change_request SET customer_review_required = true WHERE id = $1`, id)
		f.setState(id, "REVIEW")
		if changed, err := gh.SetState(f.sys, id, "CLOSED"); err != nil || !changed {
			t.Fatalf("SetState(REVIEW -> CLOSED) of an Emergency change with a stored review box = %v, %v, want true, nil", changed, err)
		}
		// The control: a Normal change with the review box must go through Customer Review.
		n := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.setState(n, "REVIEW")
		if changed, err := gh.SetState(f.sys, n, "CLOSED"); err == nil || changed {
			t.Fatalf("SetState(REVIEW -> CLOSED) of a Normal change with the review box = %v, %v, want a refusal", changed, err)
		}
	})
	t.Run("a customer state it is already in keeps asking the customer", func(t *testing.T) {
		// The rule keeps an Emergency change from ENTERING a customer state; it does not take a
		// change that is already waiting there out of the customer's hands (a row from before the
		// rule, or a migrated one): the contacts are asked as for any other change, and their own
		// answer moves it.
		f := newCustomerGroupFlow(t)
		for _, tc := range []struct {
			state, after string
			legal        []string
			answer       func(f *crFlow, id string) error
		}{
			{"CUSTOMER_APPROVAL", "SCHEDULED", []string{"implement", "canceled"}, func(f *crFlow, id string) error { _, err := f.approveAs(id, crScopeUserA1, true); return err }},
			{"CUSTOMER_REVIEW", "CLOSED", nil, func(f *crFlow, id string) error { _, err := f.reviewAs(id, crScopeUserA2, true); return err }},
		} {
			id := f.createWithProject(domain.ChangeRequestTypeEmergency, sp(crScopeProjectA), false, false)
			f.execSQL(`UPDATE change_request SET customer_approval_required = true, customer_review_required = true WHERE id = $1`, id)
			f.setState(id, tc.state)
			// Restating the project is what asks the contacts of a change in a customer state.
			f.setProject(id, crScopeProjectA)
			st := f.customerStages(id)
			if len(st) != 1 {
				t.Fatalf("%s: %d customer stages for the Emergency change, want the one the contacts are asked in", tc.state, len(st))
			}
			assertApprovers(t, tc.state+" stage", st[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
			f.wantCanAnswer(id, "in "+tc.state+" with the stage", true, crScopeUserA1, crScopeUserA2)
			if err := tc.answer(f, id); err != nil {
				t.Fatalf("%s: the customer's answer: %v", tc.state, err)
			}
			f.expect(id, "after the customer's answer in "+tc.state, tc.after, tc.legal...)
		}
	})
}

// A MIGRATED Emergency change that is sitting in a customer state: the previous system itself asked the
// customer group (an UNLABELED stage in customer_group_id, EXTERNAL approvers), our own boxes
// are false and the sync's requirement flags may be set. It is not stranded: it reads
// customerCanAnswer true for the contacts, the customer's first act gives it the stage this
// service's flow answers on, and the answer moves it -- Customer Approval to Scheduled, Customer
// Review to Closed -- exactly as on a Normal change in the same shape. A Re-schedule re-asks
// them, and the sync's flags are never written by any of it.
func TestChangeRequestEmergencyIntegration_MigratedEmergencyInACustomerStateIsAnswerable(t *testing.T) {
	// syncFlags: the sync's two requirement flags are set (they are, on the migrated rows that have a
	// customer step at all). A row whose flag is already set cannot be given the opposite answer
	// (the lock on isCustomerApproved / isCustomerReviewed), so the rejection cases leave them off.
	migrated := func(t *testing.T, state string, syncFlags bool) (*crFlow, string) {
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		f.execSQL(`UPDATE change_request SET change_model = 'EMERGENCY', is_customer_approval_required = $2,
		                  is_customer_review_required = $2 WHERE id = $1`, id, syncFlags)
		f.setState(id, state)
		if got := f.get(id).Type; got == nil || *got != "emergency" {
			t.Fatalf("type = %v, want emergency", got)
		}
		return f, id
	}
	t.Run("Customer Approval: the customer approves", func(t *testing.T) {
		f, id := migrated(t, "CUSTOMER_APPROVAL", true)
		f.wantCanAnswer(id, "in Customer Approval, the migrated stage only", true, crScopeUserA1, crScopeUserA2)
		f.wantCanAnswer(id, "in Customer Approval, the migrated stage only", false, crScopeUserSecurity, crScopeUserInactive)
		if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
			t.Fatalf("the customer's approval: %v", err)
		}
		f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
		if st := f.customerStages(id); len(st) != 1 {
			t.Fatalf("customer stages = %+v, want the one the answer was recorded on", st)
		}
		assertApprovers(t, "the stage the answer landed on", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "APPROVED", crScopeUserA2: "CANCELLED"})
		if sa, sr := f.syncFlags(id); !sa || !sr {
			t.Fatalf("the sync's flags after the answer = %v/%v, want them untouched (true/true)", sa, sr)
		}
	})
	t.Run("Customer Approval: the decision route does the same", func(t *testing.T) {
		f, id := migrated(t, "CUSTOMER_APPROVAL", true)
		if _, err := f.repo.DecideChangeRequestApproval(asContact(crScopeUserA2), id, crScopeUserA2, "approved", crFlowEmail(crScopeUserA2)); err != nil {
			t.Fatalf("the customer's decision: %v", err)
		}
		f.expect(id, "after the decision", "SCHEDULED", "implement", "canceled")
		assertApprovers(t, "the stage the decision landed on", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "APPROVED"})
	})
	t.Run("Customer Approval: the customer rejects", func(t *testing.T) {
		f, id := migrated(t, "CUSTOMER_APPROVAL", false)
		if _, err := f.approveAs(id, crScopeUserA2, false); err != nil {
			t.Fatalf("the customer's rejection: %v", err)
		}
		f.expect(id, "after the customer's rejection", "CANCELED")
	})
	t.Run("Customer Review: the customer confirms", func(t *testing.T) {
		f, id := migrated(t, "CUSTOMER_REVIEW", true)
		f.wantCanAnswer(id, "in Customer Review, the migrated stage only", true, crScopeUserA1, crScopeUserA2)
		if _, err := f.reviewAs(id, crScopeUserA2, true); err != nil {
			t.Fatalf("the customer's review: %v", err)
		}
		f.expect(id, "after the customer's review", "CLOSED")
		if sa, sr := f.syncFlags(id); !sa || !sr {
			t.Fatalf("the sync's flags after the review = %v/%v, want them untouched (true/true)", sa, sr)
		}
	})
	t.Run("Customer Review: the customer fails it", func(t *testing.T) {
		f, id := migrated(t, "CUSTOMER_REVIEW", false)
		if _, err := f.reviewAs(id, crScopeUserA1, false); err != nil {
			t.Fatalf("the customer's failed review: %v", err)
		}
		f.expect(id, "after the failed review", "ROLLBACK")
	})
	t.Run("Customer Approval: a Re-schedule asks the contacts again", func(t *testing.T) {
		f, id := migrated(t, "CUSTOMER_APPROVAL", false)
		if err := f.reschedule(id, nil, sp(rsEnd2)); err != nil {
			t.Fatalf("re-schedule: %v", err)
		}
		f.expect(id, "after Re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
		st := f.customerStages(id)
		if len(st) != 1 {
			t.Fatalf("customer stages = %+v, want the fresh one this service asks in", st)
		}
		assertApprovers(t, "the fresh stage", st[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
		if _, err := f.approveAs(id, crScopeUserA2, true); err != nil {
			t.Fatalf("the customer's approval of the new window: %v", err)
		}
		f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
	})
}

// An Emergency change that is ALREADY waiting in Customer Approval (a migrated one, or one from before
// the rule) takes part in the customer's proposed-time conversation exactly as any other change does:
// the same predicate, the same recorded-proposer rule for Accept, the same plain Re-schedule over a date
// nobody proposed, the same counter-proposal that asks the project's contacts again. The Emergency rule
// keeps a change from ENTERING Customer Approval; it never looks at the type once the change is there,
// and it never writes the sync's requirement flags.
func TestChangeRequestEmergencyIntegration_MigratedEmergencyFollowsTheProposedTimeRules(t *testing.T) {
	// A migrated change in Customer Approval (the unlabeled customer stage asks the project's two
	// contacts), typed Emergency, with both of the sync's requirement flags set.
	emergencyInCustomerApproval := func(t *testing.T) (*crFlow, string) {
		t.Helper()
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		f.execSQL(`UPDATE change_request SET change_model = 'EMERGENCY', is_customer_approval_required = true,
		                  is_customer_review_required = true WHERE id = $1`, id)
		return f, id
	}
	untouchedSyncFlags := func(t *testing.T, f *crFlow, id, when string) {
		t.Helper()
		if sa, sr := f.syncFlags(id); !sa || !sr {
			t.Fatalf("the sync's flags %s = %v/%v, want them untouched (true/true)", when, sa, sr)
		}
	}

	t.Run("a registered contact's proposal waits, is recorded, and WSO2's Accept schedules it: no CAB, no second ask", func(t *testing.T) {
		f, id := emergencyInCustomerApproval(t)
		f.mustPropose(id, crScopeUserA1, rsStart2)
		f.expect(id, "after the proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
		// The customer's first act gave the migrated row the stage this service answers on (as on any change in
		// that shape); from here on nothing may add one.
		stages := len(f.stages(id))
		p := f.proposalOf(id)
		if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || !*p.ProposerRecorded || p.CanAccept == nil || !*p.CanAccept {
			t.Fatalf("customerProposal = %+v, want it pending, its proposer recorded and Accept open", p)
		}
		f.mustAccept(id)
		f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
		f.wantPlanned(id, "after Accept", rsStart2, endFor(rsStart2))
		f.wantConversation(id, "after Accept", rsStart2, "AGREE")
		if got := len(f.stages(id)); got != stages {
			t.Fatalf("stages after Accept = %d, want the %d it had (no CAB stage, no second ask)", got, stages)
		}
		untouchedSyncFlags(t, f, id, "after Accept")
	})

	t.Run("a date nobody is recorded as having proposed cannot be accepted, and a plain Re-schedule over it asks the contacts again", func(t *testing.T) {
		f, id := emergencyInCustomerApproval(t)
		f.syncWritesConversation(id, sp(rsStart2), "")
		p := f.proposalOf(id)
		if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || *p.ProposerRecorded || p.CanAccept == nil || *p.CanAccept {
			t.Fatalf("customerProposal for a date nobody proposed = %+v, want it pending with no proposer recorded and Accept closed", p)
		}
		before := f.snap(id)
		_, err := f.accept(id)
		f.wantConflictExact("Accept of a date nobody proposed", err, msgAcceptNobodyRecordedFull)
		wantRefusalCode(t, "Accept of a date nobody proposed", err, 409, apierror.CodeChangeRequestProposerNotRecorded)
		f.wantRefusedSame("Accept of a date nobody proposed", id, before, err)

		if err := f.reschedule(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("a Re-schedule over a date nobody proposed: %v", err)
		}
		f.expect(id, "after the Re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.wantPlanned(id, "after the Re-schedule", rsStart3, rsEnd3)
		f.wantConversation(id, "after the Re-schedule", rsStart2, "") // no answer is written against a date nobody proposed
		custom := f.customerStages(id)
		assertApprovers(t, "the contacts asked again", custom[len(custom)-1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
		untouchedSyncFlags(t, f, id, "after the Re-schedule")
	})

	t.Run("a different time answers the contact's proposal and asks the project's contacts again, still in Customer Approval", func(t *testing.T) {
		f, id := emergencyInCustomerApproval(t)
		f.mustPropose(id, crScopeUserA2, rsStart2)
		if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("Propose a different time: %v", err)
		}
		f.expect(id, "after the different time", "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.wantPlanned(id, "after the different time", rsStart3, rsEnd3)
		f.wantConversation(id, "after the different time", rsStart2, "DISAGREE")
		custom := f.customerStages(id)
		assertApprovers(t, "the contacts asked again", custom[len(custom)-1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
		untouchedSyncFlags(t, f, id, "after the different time")
	})
}

// A MIGRATED Emergency change (csm-sync-service mirrors it): ONE stage, no label, in the CAB group at
// position 0, UPPER_SNAKE raw_status and approver states, the sync's customer flags ticked, our own boxes
// false. It displays as the CAB stage, its approver can decide it, deciding schedules it, and the sync's
// flags are neither read as a requirement nor ever written.
func TestChangeRequestEmergencyIntegration_MigratedEmergencyDisplaysDecidesAndSchedulesAsCAB(t *testing.T) {
	for _, ours := range []bool{false, true} {
		name := "our boxes off"
		if ours {
			name = "our boxes on too (a dev row)"
		}
		t.Run(name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.createWithProject(domain.ChangeRequestTypeEmergency, sp(crScopeProjectA), false, false)
			f.execSQL(`UPDATE change_request SET is_customer_approval_required = true, is_customer_review_required = true,
			                                       customer_approval_required = $2, customer_review_required = $2 WHERE id = $1`, id, ours)
			f.setState(id, "AUTHORIZE")
			cab := crCABGroupID
			stage := f.seedSyncedStage(id, &cab, 30, map[string]string{crCABMemberUserID1: "REQUESTED", crCABMemberUserID2: "REQUESTED"})

			// Displayed as the CAB stage, by its group, not as the "Assess" its position would give it.
			view := f.approvalsAs(id, crCABMemberUserID1)
			if len(view.Approvals) != 1 {
				t.Fatalf("approvals = %+v, want the one stage", view.Approvals)
			}
			a := view.Approvals[0]
			if a.Stage != "CAB Approval" || a.ApproverName != "CAB Approval" || a.Status != domain.ChangeRequestApprovalStatusPending {
				t.Fatalf("the migrated stage reads %q / %q / %q, want CAB Approval / CAB Approval / PENDING", a.Stage, a.ApproverName, a.Status)
			}
			if a.AssignmentGroup == nil || a.AssignmentGroup.ID != crCABGroupID {
				t.Fatalf("assignmentGroup = %+v, want the CAB group", a.AssignmentGroup)
			}
			f.wantCanDecide(id, "in Authorize", map[string][]string{crCABMemberUserID1: {"CAB Approval"}, crCABMemberUserID2: {"CAB Approval"}})
			f.expect(id, "in Authorize", "AUTHORIZE", "canceled")

			// Its approver decides it, and it schedules: never Customer Approval, whatever the flags say.
			if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
				t.Fatalf("the approver's decision on the migrated stage: %v", err)
			}
			f.expect(id, "after the CAB approval", "SCHEDULED", "implement", "canceled")
			if got := f.approverState(stage, crCABMemberUserID1); got != "APPROVED" {
				t.Fatalf("approver row = %s, want APPROVED", got)
			}
			if got := f.approverState(stage, crCABMemberUserID2); got != "CANCELLED" {
				t.Fatalf("the sibling's row = %s, want CANCELLED", got)
			}
			if st := f.customerStages(id); len(st) != 0 {
				t.Fatalf("customer stages = %+v, want none", st)
			}
			// The sync's flags are the sync's: not written by a decision.
			if sa, sr := f.syncFlags(id); !sa || !sr {
				t.Fatalf("the sync's flags after the decision = %v/%v, want them untouched (true/true)", sa, sr)
			}
			// Review goes straight to Closed.
			f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
			f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
			f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
			if sa, sr := f.syncFlags(id); !sa || !sr {
				t.Fatalf("the sync's flags at the end = %v/%v, want them untouched (true/true)", sa, sr)
			}
		})
	}
}

// What a migrated Emergency change that is long finished reads like: the CAB stage, approved, with its
// sync-shaped approver states; nothing can be decided on it. The same rows on a Normal change keep
// their positional name.
func TestChangeRequestEmergencyIntegration_AFinishedMigratedEmergencyStillReadsAsCAB(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeEmergency, sp(crScopeProjectA), false, false)
	f.setState(id, "CLOSED")
	cab := crCABGroupID
	stage := f.seedSyncedStage(id, &cab, 30, map[string]string{crCABMemberUserID1: "APPROVED", crCABMemberUserID2: "NOT_REQUIRED"})
	f.execSQL(`UPDATE approval_stage SET raw_status = 'APPROVED' WHERE id = $1::uuid`, stage)
	view := f.approvalsAs(id, crCABMemberUserID1)
	if len(view.Approvals) != 1 || view.Approvals[0].Stage != "CAB Approval" || view.Approvals[0].Status != domain.ChangeRequestApprovalStatusApproved {
		t.Fatalf("approvals = %+v, want the one approved CAB Approval stage", view.Approvals)
	}
	got := map[string]string{}
	for _, ap := range view.Approvals[0].Approvers {
		got[ap.ID] = ap.Status
	}
	if got[crCABMemberUserID1] != "APPROVED" || got[crCABMemberUserID2] != "NOT_REQUIRED" {
		t.Fatalf("approver states = %v, want APPROVED and NOT_REQUIRED as the sync wrote them", got)
	}
	f.wantCanDecide(id, "in Closed", nil)

	n := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	f.setState(n, "CLOSED")
	f.seedSyncedStage(n, &cab, 30, map[string]string{crCABMemberUserID1: "APPROVED"})
	if view := f.approvalsAs(n, crCABMemberUserID1); len(view.Approvals) != 1 || view.Approvals[0].Stage != "Assess" {
		t.Fatalf("the same stage on a Normal change reads %+v, want the positional name it always had", view.Approvals)
	}
}

// An in-flight Emergency change that an earlier build gave an "ECAB Approval" stage (a group of its
// own): it is still shown under that name, the approvers that were asked can decide it as the CAB
// stage, and deciding it schedules the change. Nobody who was not asked can (a decision needs the
// caller's own REQUESTED row), the creator cannot, and a resent Request Approval adds no stage.
func TestChangeRequestEmergencyIntegration_AHistoricECABStageStillDecides(t *testing.T) {
	setup := func(t *testing.T) (*crFlow, string, string) {
		f := newCustomerGroupFlow(t)
		seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
		id := f.createWithProject(domain.ChangeRequestTypeEmergency, sp(crScopeProjectA), false, false)
		f.setState(id, "AUTHORIZE")
		label, ecab := "ECAB Approval", crECABGroupID
		stage := f.seedLooseStageInGroup(id, &label, &ecab, 30, map[string]string{crECABMemberUserID: "REQUESTED", crFlowCreatorID: "CANCELLED"})
		return f, id, stage
	}
	t.Run("shown, decided as CAB, schedules", func(t *testing.T) {
		f, id, stage := setup(t)
		f.execSQL(`UPDATE change_request SET customer_approval_required = true WHERE id = $1`, id) // an old-build ticked box
		view := f.approvalsAs(id, crECABMemberUserID)
		if len(view.Approvals) != 1 || view.Approvals[0].Stage != "ECAB Approval" || view.Approvals[0].ApproverName != "ECAB Approval" {
			t.Fatalf("approvals = %+v, want the stage shown as it was written", view.Approvals)
		}
		f.wantCanDecide(id, "in Authorize", map[string][]string{crECABMemberUserID: {"ECAB Approval"}})
		if got := f.canDecideAs(id, crCABMemberUserID1); len(got) != 0 {
			t.Fatalf("canDecide for a CAB member with no row on the stage = %v, want none", got)
		}
		// A CAB member who was not asked has no pending approval on it; the creator cannot approve.
		err := f.decide(id, crCABMemberUserID1, "approved")
		var nf *apierror.NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("a CAB member with no row deciding the historic stage: err = %v (%T), want a NotFoundError", err, err)
		}
		f.wantForbidden("the creator on the historic stage", f.decide(id, crFlowCreatorID, "approved"), "creator")
		f.expect(id, "after the refused decisions", "AUTHORIZE", "canceled")
		// A resent Request Approval provisions nothing further.
		if _, err := f.patchState(id, domain.ChangeRequestStateAssess); err != nil {
			t.Fatalf("a resent Request Approval on the in-flight change: %v", err)
		}
		if got := f.labels(id); strings.Join(got, ",") != "ECAB Approval" {
			t.Fatalf("stages after the resent Request Approval = %v, want only the historic one", got)
		}
		// The asked approver decides: Scheduled (never Customer Approval: an Emergency change asks no customer).
		if err := f.decide(id, crECABMemberUserID, "approved"); err != nil {
			t.Fatalf("the historic stage's approver deciding it: %v", err)
		}
		f.expect(id, "after the decision", "SCHEDULED", "implement", "canceled")
		if got := f.approverState(stage, crECABMemberUserID); got != "APPROVED" {
			t.Fatalf("approver row = %s, want APPROVED", got)
		}
		if st := f.customerStages(id); len(st) != 0 {
			t.Fatalf("customer stages = %+v, want none", st)
		}
	})
	t.Run("a rejection keeps it in Authorize", func(t *testing.T) {
		f, id, stage := setup(t)
		if err := f.decide(id, crECABMemberUserID, "rejected"); err != nil {
			t.Fatalf("rejecting the historic stage: %v", err)
		}
		f.expect(id, "after the rejection", "AUTHORIZE", "canceled")
		if got := f.approverState(stage, crECABMemberUserID); got != "REJECTED" {
			t.Fatalf("approver row = %s, want REJECTED", got)
		}
	})
	t.Run("a Cancel retires it like any stage", func(t *testing.T) {
		f, id, stage := setup(t)
		f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
		if got := f.approverState(stage, crECABMemberUserID); got != "CANCELLED" {
			t.Fatalf("approver row after Cancel = %s, want CANCELLED", got)
		}
	})
	t.Run("the change moves on without it: the stage is stale, as any CAB stage", func(t *testing.T) {
		f, id, _ := setup(t)
		f.setState(id, "SCHEDULED")
		err := f.decide(id, crECABMemberUserID, "approved")
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) || !strings.Contains(ce.Msg, "no longer pending") {
			t.Fatalf("deciding the historic stage after the change left Authorize: err = %v (%T), want the stale-approval conflict", err, err)
		}
	})
}
