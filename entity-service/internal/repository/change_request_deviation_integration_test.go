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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The deviation from the implementation plan (change_request_deviation.go)
// against Postgres: editable in every state that is not final (the approved
// ones included, where the plan itself is read-only), final once the change is
// closed, canceled or rolled back, carried by a create (a clone), and never read
// by a customer.
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run ChangeRequestDeviationIntegration

func (f *crFlow) storedDeviation(id string) (deviation *string, updatedBy string) {
	f.t.Helper()
	if err := f.scoped.QueryRow(f.sys,
		`SELECT cr.implementation_deviation, COALESCE(wi.updated_by, '')
		 FROM work_item wi JOIN change_request cr ON cr.id = wi.id WHERE wi.id = $1`, id).Scan(&deviation, &updatedBy); err != nil {
		f.t.Fatalf("read the deviation: %v", err)
	}
	return deviation, updatedBy
}

func deviationOf(cr domain.ChangeRequest) string {
	if cr.ImplementationDeviation == nil {
		return "<nil>"
	}
	return *cr.ImplementationDeviation
}

func TestChangeRequestDeviationIntegration_EditableUntilFinal(t *testing.T) {
	str := func(v string) *string { return &v }
	for _, state := range []string{"", "NEW", "ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW"} {
		name := state
		if name == "" {
			name = "NULL"
		}
		t.Run(name, func(t *testing.T) {
			f := newCRFlow(t)
			f.seedAssignedGroup()
			id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
			approvedLockSeed(f, id)
			f.setState(id, state)

			// Set, then read back through the detail (the PATCH receipt is that read).
			deviation := "<p>Restarted the nodes one at a time.</p>"
			cr, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: &deviation})
			if err != nil {
				t.Fatalf("recording the deviation: %v", err)
			}
			if got := deviationOf(cr); got != deviation {
				t.Fatalf("receipt deviation = %q, want %q", got, deviation)
			}
			if got := deviationOf(f.get(id)); got != deviation {
				t.Fatalf("detail deviation = %q, want %q", got, deviation)
			}

			// Once approved the plan is still read-only, and a PATCH that tries both
			// writes neither.
			if state != "" && state != "NEW" && state != "ASSESS" && state != "AUTHORIZE" {
				plan := str("<p>Another plan</p>")
				other := "<p>Something else</p>"
				_, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationPlan: &plan, ImplementationDeviation: &other})
				approvedLockRefusal(t, err, "implementationPlan")
				if got, _ := f.storedDeviation(id); got == nil || *got != deviation {
					t.Fatalf("a refused PATCH changed the deviation to %v", got)
				}
			}

			// A blank value clears it.
			blank := "   "
			if _, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: &blank}); err != nil {
				t.Fatalf("clearing the deviation: %v", err)
			}
			if got, _ := f.storedDeviation(id); got != nil {
				t.Fatalf("after clearing, the deviation is %q, want NULL", *got)
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
// Review may record the deviation with it.
func TestChangeRequestDeviationIntegration_TheClosingPatchMayRecordIt(t *testing.T) {
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
		t.Fatalf("a ServiceNow-first create with a deviation: %v", err)
	}
	if got := deviationOf(f.get(resp.ChangeRequest.ID)); got != deviation {
		t.Fatalf("ServiceNow-first create: deviation = %q, want %q", got, deviation)
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
	deviation := "<p>Internal: the vendor patch was applied by hand.</p>"
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{ImplementationDeviation: &deviation}); err != nil {
		t.Fatalf("recording the deviation in Customer Approval: %v", err)
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
