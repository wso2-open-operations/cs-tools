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

// The refusals of a customer's answer and of a proposed implementation time each
// carry a machine-readable code (apierror/codes.go), so that the customer portal
// never has to read the wording of a message to know which refusal it got. This
// is the contract, refusal by refusal, against a real database: the message and
// the HTTP status (the error type) are asserted unchanged by the other suites;
// here is the code each one is raised with.
//
// Same harness as the other customer suites: the DSN-gated crFlow, the PATCH
// sent by an EXTERNAL caller.

// wantRefusalCode asserts err is a 409 (*apierror.ConflictError) or 403
// (*apierror.ForbiddenError) carrying exactly code.
func wantRefusalCode(t *testing.T, what string, err error, wantStatus int, code string) {
	t.Helper()
	var got string
	var ce *apierror.ConflictError
	var fe *apierror.ForbiddenError
	switch {
	case errors.As(err, &ce):
		if wantStatus != 409 {
			t.Fatalf("%s: a 409 (%q), want a %d", what, ce.Msg, wantStatus)
		}
		got = ce.Code
	case errors.As(err, &fe):
		if wantStatus != 403 {
			t.Fatalf("%s: a 403 (%q), want a %d", what, fe.Msg, wantStatus)
		}
		got = fe.Code
	default:
		t.Fatalf("%s: err = %v (%T), want a %d refusal", what, err, err, wantStatus)
	}
	if got != code {
		t.Fatalf("%s: code = %q, want %q", what, got, code)
	}
}

func TestChangeRequestErrorCodesIntegration_CustomerRefusals(t *testing.T) {
	f := newCustomerGroupFlow(t)
	f.registerContact(crScopeProjectA, crScopeAccountID, crFlowCreatorID)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.requestApproval(id)
	f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "authorize", "canceled")

	propose := func(user string) error {
		_, err := f.patchAsContact(id, user, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStartEarly)})
		return err
	}

	// Who may act: not a PORTAL_USER contact, the creator, a field a customer may
	// not set. Each is "forbidden": the caller may not do this here, ever.
	_, err := f.approveAs(id, crScopeUserSecurity, true)
	wantRefusalCode(t, "an answer by a contact holding no PORTAL_USER role", err, 403, apierror.CodeChangeRequestForbidden)
	wantRefusalCode(t, "a proposal by a contact holding no PORTAL_USER role", propose(crScopeUserSecurity), 403, apierror.CodeChangeRequestForbidden)
	_, err = f.approveAs(id, crFlowCreatorID, true)
	wantRefusalCode(t, "an answer by the creator", err, 403, apierror.CodeChangeRequestForbidden)
	wantRefusalCode(t, "a proposal by the creator", propose(crFlowCreatorID), 403, apierror.CodeChangeRequestForbidden)
	_, err = f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{Title: sp("hijacked")})
	wantRefusalCode(t, "a field a customer may not set", err, 403, apierror.CodeChangeRequestForbidden)

	// A contact registered after the request went out was never asked.
	const zed = "3bbbbbbb-0000-0000-0000-0000000000a9"
	f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user, user_type)
	           VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, 'Zed Late', 'Zed', 'Late', $2, true, false, 'EXTERNAL'::user_type_enum)`, zed, crFlowEmail(zed))
	f.registerContact(crScopeProjectA, crScopeAccountID, zed)
	_, err = f.approveAs(id, zed, true)
	wantRefusalCode(t, "an answer by a contact who was not asked", err, 403, apierror.CodeChangeRequestNotAsked)
	wantRefusalCode(t, "a proposal by a contact who was not asked", propose(zed), 403, apierror.CodeChangeRequestNotAsked)

	// An answer given for a window the page no longer shows.
	_, err = f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{
		IsCustomerApproved: boolp(true), ExpectedPlannedStartOn: sp(rsStart2), ExpectedPlannedEndOn: sp(rsEnd1)})
	wantRefusalCode(t, "an answer for a window that moved", err, 409, apierror.CodeChangeRequestScheduleChanged)

	// A proposal while WSO2 has the change on hold.
	f.execSQL(`UPDATE change_request SET is_on_hold = true WHERE id = $1`, id)
	wantRefusalCode(t, "a proposal on a change on hold", propose(crScopeUserA1), 409, apierror.CodeChangeRequestOnHold)
	// An answer is not refused by a hold (only a proposal is), so it carries no code.
	f.execSQL(`UPDATE change_request SET is_on_hold = false WHERE id = $1`, id)

	f.expect(id, "after the refused attempts", "CUSTOMER_APPROVAL", "authorize", "canceled")

	// The first answer is recorded; the second contact then arrives late.
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("the first answer: %v", err)
	}
	_, err = f.approveAs(id, crScopeUserA2, true)
	wantRefusalCode(t, "an answer after a sibling answered", err, 409, apierror.CodeChangeRequestApprovalNotPending)
	_, err = f.approveAs(id, crScopeUserA2, false)
	wantRefusalCode(t, "a rejection after a sibling answered", err, 409, apierror.CodeChangeRequestApprovalNotPending)
	wantRefusalCode(t, "a proposal after the customer answered", propose(crScopeUserA2), 409, apierror.CodeChangeRequestNotProposable)
}

// Nothing waits for the customer at all: the stage was decided or withdrawn while
// the change request still reads Customer Approval (a re-sync or a hand edit could
// leave it so), so there is nothing to answer and nobody a new time could go to.
func TestChangeRequestErrorCodesIntegration_NothingPending(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)

	// Alice's row decided, Bob's withdrawn: no REQUESTED row is left, and a decided
	// stage is never reopened.
	f.execSQL(`UPDATE approval_stage_approver SET state = 'APPROVED' WHERE work_item_id = $1 AND approver_user_id = $2`, id, crScopeUserA1)
	f.execSQL(`UPDATE approval_stage_approver SET state = 'CANCELLED' WHERE work_item_id = $1 AND approver_user_id = $2`, id, crScopeUserA2)
	f.expect(id, "with no customer request left", "CUSTOMER_APPROVAL", "authorize", "canceled")

	_, err := f.approveAs(id, crScopeUserA2, true)
	wantRefusalCode(t, "an answer with nothing pending", err, 409, apierror.CodeChangeRequestApprovalNotPending)
	_, err = f.patchAsContact(id, crScopeUserA2, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStartEarly)})
	wantRefusalCode(t, "a proposal with nobody asked", err, 409, apierror.CodeChangeRequestNotProposable)
}

// A proposal on a change request that is not in Customer Approval is not open to
// one (a change request the customer is not yet asked about is a 404, so this is
// the state after the answer, or a change request somebody moved on).
func TestChangeRequestErrorCodesIntegration_NotProposableOutOfState(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("approval: %v", err)
	}
	f.expect(id, "after the customer approved", "SCHEDULED", "implement", "canceled")
	_, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStartEarly)})
	wantRefusalCode(t, "a proposal in Scheduled", err, 409, apierror.CodeChangeRequestNotProposable)
}
