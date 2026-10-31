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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The deviation from the implementation plan (change_request_deviation.go)
// against Postgres: recorded from Implement on (Implement, Review, Customer
// Review), before that only a clone's carried-over one can be cleared in New,
// final once the change is closed, canceled or rolled back, carried by a create
// (a clone), Request Approval refused until it is folded into the plan and
// cleared, and never read by a customer.
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run ChangeRequestDeviationIntegration

const wantDeviationCarriedOverMsg = "implementationDeviation can only be cleared before implementation: fold the deviation from the implementation plan carried over from the cloned change request into the implementation plan, then clear it"

const wantDeviationNotIncorporatedMsg = "approval cannot be requested until the deviation from the implementation plan carried over from the cloned change request has been folded into the implementation plan and cleared"

func (f *crFlow) storedDeviation(id string) (deviation *string, updatedBy string) {
	f.t.Helper()
	if err := f.scoped.QueryRow(f.sys,
		`SELECT cr.implementation_deviation, COALESCE(wi.updated_by, '')
		 FROM work_item wi JOIN change_request cr ON cr.id = wi.id WHERE wi.id = $1`, id).Scan(&deviation, &updatedBy); err != nil {
		f.t.Fatalf("read the deviation: %v", err)
	}
	return deviation, updatedBy
}

// seedDeviation stores the deviation (nil: none) and marks the row, so a
// refused PATCH can be shown to have written nothing.
func (f *crFlow) seedDeviation(id string, deviation *string) {
	f.t.Helper()
	f.execSQL(`UPDATE change_request SET implementation_deviation = $2 WHERE id = $1`, id, deviation)
	f.execSQL(`UPDATE work_item SET updated_by = 'before-the-patch' WHERE id = $1`, id)
}

func (f *crFlow) workNoteCount(id string) int {
	f.t.Helper()
	var n int
	if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM comment WHERE work_item_id = $1`, id).Scan(&n); err != nil {
		f.t.Fatalf("count comments: %v", err)
	}
	return n
}

func deviationOf(cr domain.ChangeRequest) string {
	if cr.ImplementationDeviation == nil {
		return "<nil>"
	}
	return *cr.ImplementationDeviation
}

func ptrOrNil(v *string) string {
	if v == nil {
		return "<nil>"
	}
	return *v
}

// deviationRefusal asserts that err is the 400 a change of the deviation gets
// in the (upper-case, "" = NULL) stored state: want is "not-yet",
// "carried-over" or "final".
func deviationRefusal(t *testing.T, what string, err error, state, want string) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("%s: err = %v (%T), want a ValidationError", what, err, err)
	}
	name := strings.ToLower(state)
	if name == "" {
		name = "new"
	}
	var code, msg string
	switch want {
	case "not-yet":
		code = apierror.CodeChangeRequestDeviationNotYet
		msg = "implementationDeviation cannot be changed yet: the deviation from the implementation plan can be recorded once implementation has started (current state: " + name + ")"
	case "carried-over":
		code, msg = apierror.CodeChangeRequestDeviationNotYet, wantDeviationCarriedOverMsg
	case "final":
		code = apierror.CodeChangeRequestDeviationFinal
		if state == "ROLLBACK" {
			name = "rolled back"
		}
		msg = "implementationDeviation can no longer be changed: the change request is " + name + ", so its record is final"
	default:
		t.Fatalf("unknown refusal %q", want)
	}
	if ve.Code != code {
		t.Errorf("%s: errorCode = %q, want %q", what, ve.Code, code)
	}
	if ve.Msg != msg {
		t.Errorf("%s: message = %q\nwant      %q", what, ve.Msg, msg)
	}
}

// The rule by stored state, one row per state: setting one when none is stored,
// changing a stored one, clearing a stored one. Resending the stored value and a
// blank value when none is stored are accepted in every state. A refused PATCH
// writes nothing, not even the work note riding with it. In an approved state a
// PATCH that also changes the plan gets the approved-content lock's refusal, whose
// tail names the deviation only where it can be recorded.
func TestChangeRequestDeviationIntegration_PerState(t *testing.T) {
	str := func(v string) *string { return &v }
	const recorded = "<p>Recorded earlier.</p>"
	for _, row := range []struct {
		state              string // "" = NULL
		set, change, clear string // "ok" or the refusal (deviationRefusal)
		approved           bool
	}{
		{"", "not-yet", "carried-over", "ok", false},
		{"NEW", "not-yet", "carried-over", "ok", false},
		{"ASSESS", "not-yet", "not-yet", "not-yet", false},
		{"AUTHORIZE", "not-yet", "not-yet", "not-yet", false},
		{"CUSTOMER_APPROVAL", "not-yet", "not-yet", "not-yet", true},
		{"SCHEDULED", "not-yet", "not-yet", "not-yet", true},
		{"IMPLEMENT", "ok", "ok", "ok", true},
		{"REVIEW", "ok", "ok", "ok", true},
		{"CUSTOMER_REVIEW", "ok", "ok", "ok", true},
		{"CLOSED", "final", "final", "final", true},
		{"CANCELED", "final", "final", "final", false},
		{"ROLLBACK", "final", "final", "final", true},
	} {
		name := row.state
		if name == "" {
			name = "NULL"
		}
		t.Run(name, func(t *testing.T) {
			f := newCRFlow(t)
			f.seedAssignedGroup()
			id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
			approvedLockSeed(f, id)
			f.setState(id, row.state)

			try := func(what string, stored *string, value, want string, after *string) {
				t.Helper()
				f.seedDeviation(id, stored)
				notes := f.workNoteCount(id)
				note := "a work note riding with the deviation"
				cr, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: &value, WorkNote: &note})
				if want == "ok" {
					if err != nil {
						t.Fatalf("%s: %v, want it accepted", what, err)
					}
					// The PATCH receipt is the detail read.
					if got := deviationOf(cr); got != ptrOrNil(after) {
						t.Fatalf("%s: receipt deviation = %q, want %q", what, got, ptrOrNil(after))
					}
					if got, _ := f.storedDeviation(id); ptrOrNil(got) != ptrOrNil(after) {
						t.Fatalf("%s: stored deviation = %q, want %q", what, ptrOrNil(got), ptrOrNil(after))
					}
					return
				}
				deviationRefusal(t, what, err, row.state, want)
				if got, by := f.storedDeviation(id); ptrOrNil(got) != ptrOrNil(stored) || by != "before-the-patch" {
					t.Fatalf("%s: after the refusal the deviation is %q, updated_by %q -- something was written", what, ptrOrNil(got), by)
				}
				if got := f.workNoteCount(id); got != notes {
					t.Fatalf("%s: a refused PATCH wrote %d comment rows", what, got-notes)
				}
			}

			try("set (none stored)", nil, "<p>Restarted the nodes one at a time.</p>", row.set, str("<p>Restarted the nodes one at a time.</p>"))
			try("change (one stored)", str(recorded), "<p>Something else.</p>", row.change, str("<p>Something else.</p>"))
			try("clear (one stored)", str(recorded), "   ", row.clear, nil)
			// Never a change, so accepted in every state.
			try("resend (one stored)", str(recorded), recorded, "ok", str(recorded))
			try("blank (none stored)", nil, "", "ok", nil)

			if !row.approved {
				return
			}
			// Once approved the plan is read-only, and a PATCH that tries both writes
			// neither. The approved lock is judged first; its tail says whether the
			// deviation can still be recorded.
			f.seedDeviation(id, str(recorded))
			plan := str("<p>Another plan</p>")
			other := "<p>Something else</p>"
			_, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationPlan: &plan, ImplementationDeviation: &other})
			approvedLockRefusal(t, err, "implementationPlan")
			tail := "; only work notes can be added"
			if row.set == "ok" {
				tail = "; only work notes and the deviation from the implementation plan can be added"
			}
			var ve *apierror.ValidationError
			if errors.As(err, &ve) && !strings.HasSuffix(ve.Msg, tail) {
				t.Errorf("approved lock message = %q, want it to end %q", ve.Msg, tail)
			}
			if got, by := f.storedDeviation(id); ptrOrNil(got) != recorded || by != "before-the-patch" {
				t.Fatalf("a refused PATCH changed the deviation to %q (updated_by %q)", ptrOrNil(got), by)
			}
		})
	}
}

func TestChangeRequestDeviationIntegration_FinalStatesKeepTheRecord(t *testing.T) {
	for state, word := range map[string]string{"CLOSED": "closed", "CANCELED": "canceled", "ROLLBACK": "rolled back"} {
		t.Run(state, func(t *testing.T) {
			f := newCRFlow(t)
			f.seedAssignedGroup()
			id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
			f.execSQL(`UPDATE change_request SET implementation_deviation = '<p>Recorded</p>' WHERE id = $1`, id)
			f.setState(id, state)
			f.execSQL(`UPDATE work_item SET updated_by = 'before-the-patch' WHERE id = $1`, id)

			for name, value := range map[string]string{"another value": "<p>Changed afterwards</p>", "a blank value": ""} {
				v := value
				note := "a work note riding with a refused edit"
				_, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: &v, WorkNote: &note})
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("%s: err = %v (%T), want a ValidationError", name, err, err)
				}
				if ve.Code != apierror.CodeChangeRequestDeviationFinal {
					t.Errorf("%s: errorCode = %q, want %q", name, ve.Code, apierror.CodeChangeRequestDeviationFinal)
				}
				want := "implementationDeviation can no longer be changed: the change request is " + word + ", so its record is final"
				if ve.Msg != want {
					t.Errorf("%s: message = %q, want %q", name, ve.Msg, want)
				}
			}
			if got, by := f.storedDeviation(id); got == nil || *got != "<p>Recorded</p>" || by != "before-the-patch" {
				t.Fatalf("after the refusals: deviation %v, updated_by %q -- something was written", got, by)
			}
			var notes int
			if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM comment WHERE work_item_id = $1`, id).Scan(&notes); err != nil {
				t.Fatalf("count comments: %v", err)
			}
			if notes != 0 {
				t.Fatalf("a refused PATCH wrote %d comment rows", notes)
			}

			// The stored value resent (a client sending the whole form back): accepted.
			same := "<p>Recorded</p>"
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: &same}); err != nil {
				t.Fatalf("the stored deviation resent: %v", err)
			}
			// Work notes still go through on a final change.
			note := "Closed after the review."
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{WorkNote: &note}); err != nil {
				t.Fatalf("a work note on a final change: %v", err)
			}
		})
	}

	t.Run("no deviation stored: a blank one is the no-op", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.setState(id, "CLOSED")
		blank := ""
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: &blank}); err != nil {
			t.Fatalf("a blank deviation on a closed change with none: %v", err)
		}
	})
}

// The state judged is the stored one: the PATCH that closes the change from
// Review may record the deviation with it, and the one that moves it from
// Scheduled to Implement may not (implementation has not started in the stored
// state): it is recorded once the change is in Implement.
func TestChangeRequestDeviationIntegration_TheStoredStateIsJudged(t *testing.T) {
	t.Run("the closing PATCH may record it", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.setState(id, "REVIEW")
		closed := domain.ChangeRequestStateClosed
		deviation := "<p>Kept the old index until the review.</p>"
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{State: &closed, ImplementationDeviation: &deviation}); err != nil {
			t.Fatalf("closing with the deviation: %v", err)
		}
		f.wantState(id, "CLOSED")
		if got, _ := f.storedDeviation(id); got == nil || *got != deviation {
			t.Fatalf("deviation = %v, want it recorded with the close", got)
		}
	})

	t.Run("the move to Implement cannot carry it", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.setState(id, "SCHEDULED")
		f.seedDeviation(id, nil)
		implement := domain.ChangeRequestStateImplement
		deviation := "<p>Started an hour late.</p>"
		_, err := f.patch(id, domain.PatchChangeRequestRequest{State: &implement, ImplementationDeviation: &deviation})
		deviationRefusal(t, "Scheduled to Implement with a deviation", err, "SCHEDULED", "not-yet")
		f.wantState(id, "SCHEDULED")
		if got, by := f.storedDeviation(id); got != nil || by != "before-the-patch" {
			t.Fatalf("after the refusal: deviation %q, updated_by %q -- something was written", ptrOrNil(got), by)
		}

		if _, err := f.patch(id, domain.PatchChangeRequestRequest{State: &implement}); err != nil {
			t.Fatalf("Scheduled to Implement: %v", err)
		}
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: &deviation}); err != nil {
			t.Fatalf("the deviation in Implement: %v", err)
		}
		if got, _ := f.storedDeviation(id); ptrOrNil(got) != deviation {
			t.Fatalf("deviation = %q, want %q", ptrOrNil(got), deviation)
		}
	})
}

// cloneWithDeviation creates a change request the way a clone of one that has a
// deviation does: a create carrying it. approval ticks the customer's approval box.
func (f *crFlow) cloneWithDeviation(typ domain.ChangeRequestType, deviation string, approval bool) string {
	f.t.Helper()
	group := crFlowGroupID
	resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &group, ImplementationDeviation: &deviation,
		CustomerApprovalRequired: boolp(approval),
	}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		f.t.Fatalf("a create carrying a deviation (%s): %v", typ, err)
	}
	f.execSQL(`UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`, crFlowCreatorID, resp.ChangeRequest.ID)
	return resp.ChangeRequest.ID
}

func wantDeviationNotIncorporated(t *testing.T, what string, err error) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("%s: err = %v (%T), want a ValidationError", what, err, err)
	}
	if ve.Code != apierror.CodeChangeRequestDeviationNotIncorporated {
		t.Errorf("%s: errorCode = %q, want %q", what, ve.Code, apierror.CodeChangeRequestDeviationNotIncorporated)
	}
	if ve.Msg != wantDeviationNotIncorporatedMsg {
		t.Errorf("%s: message = %q\nwant      %q", what, ve.Msg, wantDeviationNotIncorporatedMsg)
	}
}

// A clone carries its source's deviation into New. Request Approval is refused
// while it is there (whatever the type), and accepted once it is cleared -- in the
// same PATCH as Request Approval (with the plan that now says it) or before. A
// resent {state: "assess"} on a change that already left New is not judged.
func TestChangeRequestDeviationIntegration_RequestApprovalNeedsItFoldedIn(t *testing.T) {
	const carried = "<p>Rolled out to one region first.</p>"
	assess := domain.ChangeRequestStateAssess

	for _, typ := range []domain.ChangeRequestType{domain.ChangeRequestTypeNormal, domain.ChangeRequestTypeStandard, domain.ChangeRequestTypeEmergency} {
		t.Run("refused while it is there: "+string(typ), func(t *testing.T) {
			f := newCRFlow(t)
			f.seedAssignedGroup()
			id := f.cloneWithDeviation(typ, carried, false)
			for _, state := range []string{"NEW", ""} {
				f.setState(id, state)
				f.seedDeviation(id, func() *string { v := carried; return &v }())
				_, err := f.patchState(id, assess)
				wantDeviationNotIncorporated(t, "Request Approval in "+state, err)
				// Resending the stored deviation with it changes nothing.
				same := carried
				_, err = f.patch(id, domain.PatchChangeRequestRequest{State: &assess, ImplementationDeviation: &same})
				wantDeviationNotIncorporated(t, "Request Approval resending the deviation in "+state, err)
				f.wantState(id, state)
				if got, by := f.storedDeviation(id); ptrOrNil(got) != carried || by != "before-the-patch" {
					t.Fatalf("after the refusals: deviation %q, updated_by %q -- something was written", ptrOrNil(got), by)
				}
				var stages int
				if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, id).Scan(&stages); err != nil {
					t.Fatalf("count stages: %v", err)
				}
				if stages != 0 {
					t.Fatalf("a refused Request Approval provisioned %d stages", stages)
				}
			}
		})
	}

	t.Run("cleared in the same PATCH, with the plan that now says it", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
		id := f.cloneWithDeviation(domain.ChangeRequestTypeNormal, carried, false)
		plan := func() **string { v := "<p>Roll out to one region first, then the rest.</p>"; p := &v; return &p }()
		clear := ""
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{State: &assess, ImplementationPlan: plan, ImplementationDeviation: &clear}); err != nil {
			t.Fatalf("Request Approval clearing the deviation: %v", err)
		}
		f.wantState(id, "ASSESS")
		if got, _ := f.storedDeviation(id); got != nil {
			t.Fatalf("deviation = %q, want it cleared", *got)
		}
		if got := f.get(id); got.ImplementationPlan == nil || *got.ImplementationPlan != **plan {
			t.Fatalf("implementation plan = %v, want the edited one", got.ImplementationPlan)
		}

		// Already out of New: a resent {state: "assess"} is the no-op it always was,
		// even with a deviation stored (a row an older build left).
		f.execSQL(`UPDATE change_request SET implementation_deviation = '<p>left by an older build</p>' WHERE id = $1`, id)
		if _, err := f.patchState(id, assess); err != nil {
			t.Fatalf("a resent Request Approval in Assess: %v", err)
		}
		f.wantState(id, "ASSESS")
	})

	t.Run("cleared first, then Request Approval", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
		id := f.cloneWithDeviation(domain.ChangeRequestTypeNormal, carried, false)
		clear := " "
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: &clear}); err != nil {
			t.Fatalf("clearing the carried-over deviation in New: %v", err)
		}
		f.requestApproval(id)
		f.wantState(id, "ASSESS")
	})

	// A Request Approval that also SETS a deviation is refused by the deviation's
	// own edit rule (checkDeviationEditable, under the gate), which runs before the
	// state switch and so before rules 6, 7 and 8: in New with none stored it is
	// "not yet" (here ahead of the missing project of rule 6), with one stored it is
	// "carried over" (not rule 8's "not incorporated"). Nothing is written.
	t.Run("a deviation set with it gets the edit rule's refusal first", func(t *testing.T) {
		for _, c := range []struct {
			name, sent, want string
			stored           *string
			approval         bool
		}{
			{name: "none stored", sent: "<p>x</p>", want: "not-yet", approval: true},
			{name: "one stored", sent: "<p>other</p>", want: "carried-over", stored: func() *string { v := carried; return &v }()},
		} {
			t.Run(c.name, func(t *testing.T) {
				f := newCRFlow(t)
				f.seedAssignedGroup()
				id := f.cloneWithDeviation(domain.ChangeRequestTypeNormal, carried, c.approval)
				f.seedDeviation(id, c.stored)
				sent := c.sent
				_, err := f.patch(id, domain.PatchChangeRequestRequest{State: &assess, ImplementationDeviation: &sent})
				deviationRefusal(t, "Request Approval setting a deviation ("+c.name+")", err, "NEW", c.want)
				f.wantState(id, "NEW")
				if got, by := f.storedDeviation(id); ptrOrNil(got) != ptrOrNil(c.stored) || by != "before-the-patch" {
					t.Fatalf("after the refusal: deviation %q, updated_by %q -- something was written", ptrOrNil(got), by)
				}
				var stages int
				if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, id).Scan(&stages); err != nil {
					t.Fatalf("count stages: %v", err)
				}
				if stages != 0 {
					t.Fatalf("a refused Request Approval provisioned %d stages", stages)
				}
			})
		}
	})

	// The guards that come before it keep their own words: the missing project
	// (rule 6) is named first.
	t.Run("after the project guard", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		id := f.cloneWithDeviation(domain.ChangeRequestTypeNormal, carried, true)
		_, err := f.patchState(id, assess)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.HasPrefix(ve.Msg, "approval cannot be requested: the customer's approval and/or review is required but no Customer Project is set") {
			t.Fatalf("err = %v, want the missing project refusal first", err)
		}
		f.wantState(id, "NEW")
	})
}

// A clone is a create carrying the source's deviation: both create paths store
// it, and a blank one is not stored.
func TestChangeRequestDeviationIntegration_CreateCarriesIt(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	typ := domain.ChangeRequestTypeNormal
	group := crFlowGroupID
	deviation := "<p>Rolled out to one region first.</p>"

	resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &group, ImplementationDeviation: &deviation,
	}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("a create with a deviation: %v", err)
	}
	if got := deviationOf(f.get(resp.ChangeRequest.ID)); got != deviation {
		t.Fatalf("plain create: deviation = %q, want %q", got, deviation)
	}

	resp, err = f.repo.CreateChangeRequestFromServiceNow(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &group, ImplementationDeviation: &deviation,
	}, "3aaaaaaa-0000-0000-0000-0000000000d1", "CRFLOWSNDEV1", "x@example.com")
	if err != nil {
		t.Fatalf("a previous-system-first create with a deviation: %v", err)
	}
	if got := deviationOf(f.get(resp.ChangeRequest.ID)); got != deviation {
		t.Fatalf("previous-system-first create: deviation = %q, want %q", got, deviation)
	}

	blank := "  "
	resp, err = f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &group, ImplementationDeviation: &blank,
	}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("a create with a blank deviation: %v", err)
	}
	if got, _ := f.storedDeviation(resp.ChangeRequest.ID); got != nil {
		t.Fatalf("a blank deviation was stored as %q, want NULL", *got)
	}
}

// The deviation is internal: a customer who reads the change request gets no
// deviation, and cannot write one.
func TestChangeRequestDeviationIntegration_CustomersNeverReadOrWriteIt(t *testing.T) {
	f := newCustomerGroupFlow(t)
	f.useVisibility(visStrictSinceLongAgo())
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("the customer approves: %v", err)
	}
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	deviation := "<p>Internal: the vendor patch was applied by hand.</p>"
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: &deviation}); err != nil {
		t.Fatalf("recording the deviation in Implement: %v", err)
	}

	cr, err := f.getAsContact(id, crScopeUserA1)
	if err != nil {
		t.Fatalf("GET as a designated contact: %v", err)
	}
	if cr.ImplementationDeviation != nil {
		t.Fatalf("a customer's read carries the deviation %q; want null", *cr.ImplementationDeviation)
	}
	if got := deviationOf(f.get(id)); got != deviation {
		t.Fatalf("staff read = %q, want %q", got, deviation)
	}

	other := "<p>written by a customer</p>"
	_, err = f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{ImplementationDeviation: &other})
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("a customer's PATCH of the deviation: err = %v (%T), want a ForbiddenError", err, err)
	}
	if got, _ := f.storedDeviation(id); got == nil || *got != deviation {
		t.Fatalf("after a customer's attempt the deviation is %v", got)
	}
}

// Blank is the webapp's isBlankHtml rule (repository.IsBlankRichText): markup
// with no text -- an emptied editor's "<p><br></p>", "<p>&nbsp;</p>", an image
// alone -- is no deviation. It is stored as NULL on create and on PATCH, it
// clears a stored one, and a stored value that is blank (as an earlier build
// stored "<p><br></p>" as text) counts as none: changing it to "" is no change in
// any state, and it does not hold up Request Approval.
func TestChangeRequestDeviationIntegration_BlankMarkupIsNoDeviation(t *testing.T) {
	str := func(v string) *string { return &v }
	const emptied = "<p><br></p>"

	t.Run("both creates store NULL", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		typ := domain.ChangeRequestTypeNormal
		group := crFlowGroupID
		for name, blank := range map[string]string{"emptied editor": emptied, "nbsp": "<p>&nbsp;</p>", "image only": `<p><img src="https://example.com/a.png"></p>`} {
			resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
				Subject: crFlowSubject, Type: &typ, GroupID: &group, ImplementationDeviation: str(blank),
			}, crFlowEmail(crFlowCreatorID))
			if err != nil {
				t.Fatalf("%s: create: %v", name, err)
			}
			if got, _ := f.storedDeviation(resp.ChangeRequest.ID); got != nil {
				t.Fatalf("%s: create stored %q, want NULL", name, *got)
			}
		}
		resp, err := f.repo.CreateChangeRequestFromServiceNow(f.sys, domain.CreateChangeRequestRequest{
			Subject: crFlowSubject, Type: &typ, GroupID: &group, ImplementationDeviation: str(emptied),
		}, "3aaaaaaa-0000-0000-0000-0000000000d4", "CRFLOWSNDEV4", "x@example.com")
		if err != nil {
			t.Fatalf("previous-system-first create: %v", err)
		}
		if got, _ := f.storedDeviation(resp.ChangeRequest.ID); got != nil {
			t.Fatalf("previous-system-first create stored %q, want NULL", *got)
		}
	})

	t.Run("in Implement it clears a stored one", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.setState(id, "IMPLEMENT")
		f.seedDeviation(id, str("<p>Recorded.</p>"))
		cr, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: str(emptied)})
		if err != nil {
			t.Fatalf("clearing with an emptied editor: %v", err)
		}
		if cr.ImplementationDeviation != nil {
			t.Fatalf("receipt deviation = %q, want null", *cr.ImplementationDeviation)
		}
		if got, _ := f.storedDeviation(id); got != nil {
			t.Fatalf("stored deviation = %q, want NULL", *got)
		}
	})

	t.Run("a blank value is no change where it cannot be recorded", func(t *testing.T) {
		for _, state := range []string{"NEW", "ASSESS", "SCHEDULED", "CLOSED", "CANCELED", "ROLLBACK"} {
			t.Run(state, func(t *testing.T) {
				f := newCRFlow(t)
				f.seedAssignedGroup()
				id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
				f.setState(id, state)
				for _, stored := range []*string{nil, str(emptied)} {
					for _, sent := range []string{"", "<p>&nbsp;</p>", emptied} {
						f.seedDeviation(id, stored)
						if _, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: str(sent)}); err != nil {
							t.Fatalf("stored %q, sent %q: %v, want the no-op accepted", ptrOrNil(stored), sent, err)
						}
					}
				}
				// A stored blank one is none, so setting text is the refusal for none stored.
				f.seedDeviation(id, str(emptied))
				_, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: str("<p>New text.</p>")})
				want := "not-yet"
				if terminal := state == "CLOSED" || state == "CANCELED" || state == "ROLLBACK"; terminal {
					want = "final"
				}
				deviationRefusal(t, "setting text over a stored blank one", err, state, want)
			})
		}
	})

	t.Run("Request Approval", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)

		// A blank one stored as text by an earlier build does not hold it up.
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.execSQL(`UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`, crFlowCreatorID, id)
		f.seedDeviation(id, str(emptied))
		f.requestApproval(id)
		f.wantState(id, "ASSESS")

		// An emptied editor in the same PATCH clears a carried-over one.
		id = f.cloneWithDeviation(domain.ChangeRequestTypeNormal, "<p>Rolled out to one region first.</p>", false)
		assess := domain.ChangeRequestStateAssess
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{State: &assess, ImplementationDeviation: str(emptied)}); err != nil {
			t.Fatalf("Request Approval clearing with an emptied editor: %v", err)
		}
		f.wantState(id, "ASSESS")
		if got, _ := f.storedDeviation(id); got != nil {
			t.Fatalf("deviation = %q, want it cleared", *got)
		}
	})
}

// POST /change-requests is internal-only at the route; behind it, both create
// paths refuse a customer's deviation (403 change_request_forbidden, as a
// customer's PATCH carrying one is refused) before anything is written -- the
// previous-system-first one before it takes the system identity.
func TestChangeRequestDeviationIntegration_ACustomerCannotCreateWithOne(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	customer := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: "deviation.customer@example.com"})
	typ := domain.ChangeRequestTypeNormal
	group := crFlowGroupID
	deviation := "<p>Written by a customer.</p>"
	req := domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ, GroupID: &group, ImplementationDeviation: &deviation}
	count := func() int {
		t.Helper()
		var n int
		if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM work_item WHERE subject = $1`, crFlowSubject).Scan(&n); err != nil {
			t.Fatalf("count change requests: %v", err)
		}
		return n
	}
	before := count()

	wantForbidden := func(what string, err error) {
		t.Helper()
		var fe *apierror.ForbiddenError
		if !errors.As(err, &fe) {
			t.Fatalf("%s: err = %v (%T), want a ForbiddenError", what, err, err)
		}
		if fe.Code != apierror.CodeChangeRequestForbidden {
			t.Errorf("%s: errorCode = %q, want %q", what, fe.Code, apierror.CodeChangeRequestForbidden)
		}
	}
	_, err := f.repo.CreateChangeRequest(customer, req, "deviation.customer@example.com")
	wantForbidden("plain create", err)
	const snID = "3aaaaaaa-0000-0000-0000-0000000000d3"
	_, err = f.repo.CreateChangeRequestFromServiceNow(customer, req, snID, "CRFLOWSNDEV3", "deviation.customer@example.com")
	wantForbidden("previous-system-first create", err)

	if got := count(); got != before {
		t.Fatalf("a refused create wrote %d change requests", got-before)
	}
	var exists bool
	if err := f.scoped.QueryRow(f.sys, `SELECT EXISTS (SELECT 1 FROM work_item WHERE id = $1)`, snID).Scan(&exists); err != nil {
		t.Fatalf("look up the refused previous-system-first create: %v", err)
	}
	if exists {
		t.Fatal("the refused previous-system-first create wrote its row")
	}

	// Staff are not judged: the same create goes through.
	resp, err := f.repo.CreateChangeRequest(f.sys, req, crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("staff create with a deviation: %v", err)
	}
	if got := deviationOf(f.get(resp.ChangeRequest.ID)); got != deviation {
		t.Fatalf("staff create stored %q, want %q", got, deviation)
	}
}
