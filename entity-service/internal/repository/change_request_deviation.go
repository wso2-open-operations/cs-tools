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

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// This file owns the "Deviation from the implementation plan"
// (change_request.implementation_deviation, migration 0245): what the
// implementation or the review actually did differently from the approved
// implementation plan. The plan itself is under the approved-content lock
// (change_request_approved_lock.go); the deviation is deliberately not one of
// its fields, because it is written exactly when the plan can no longer
// change. It is only relevant during implementation, so it is recorded from
// Implement on (Implement, Review, Customer Review) and is final once the change
// request reaches a final state (closed, canceled, rollback). Before
// implementation nothing can be recorded, with one exception: a clone copies the
// deviation of its source, so in New the deviation it carried over can be cleared
// (after folding it into the implementation plan), and Request Approval is
// refused until it has been, so that the plan that goes for approval already
// says it. In every state resending the stored value is the accepted no-op, and
// a blank value is a change only when one is stored. The state judged is always
// the stored one, before the PATCH is applied.
//
// Blank is IsBlankRichText (rich_text_blank.go): no text once the markup is
// removed, the rule of the webapp's isBlankHtml, so "<p><br></p>" (an emptied
// editor) and "<p>&nbsp;</p>" are no deviation on either side. It is applied
// everywhere "is there a deviation" is decided -- what a create or a PATCH
// stores, whether a PATCH changes the stored value, whether one is stored, and
// Request Approval -- and to the stored value as well as the request's, so a
// blank one an earlier build stored as text counts as none too.

// normalizeImplementationDeviation is the value a request's (or the stored)
// deviation counts as: nil (NULL) for a blank one, the value as sent otherwise.
func normalizeImplementationDeviation(v *string) *string {
	if v == nil || IsBlankRichText(*v) {
		return nil
	}
	return v
}

// sameImplementationDeviation reports whether two deviations, both normalised,
// are the same: both none, or the same text.
func sameImplementationDeviation(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// readStoredImplementationDeviation is the change request's stored deviation,
// normalised (nil when it has none, or only a blank one).
func readStoredImplementationDeviation(ctx context.Context, q crQuerier, id string) (*string, error) {
	var stored *string
	err := q.QueryRow(ctx, `SELECT cr.implementation_deviation FROM change_request cr WHERE cr.id = $1`, id).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return nil, err
	}
	return normalizeImplementationDeviation(stored), nil
}

// externalCreateDeviationRefusal is the 403 for a create from an external
// (customer) caller that carries a deviation from the implementation plan.
const externalCreateDeviationRefusal = "customers cannot set implementationDeviation: the deviation from the implementation plan is recorded by WSO2 staff"

// RefuseExternalCreateDeviation refuses a create from an external caller
// (IsExternalCaller) that carries a deviation that is not blank, with the 403
// CodeChangeRequestForbidden, as classifyExternalPatch refuses a customer's
// PATCH that carries one. POST /change-requests is internal-only at the route
// (routes.go), so this is a second line: the service's create that calls the
// previous system first calls it before that system is called (a refusal after
// it would strand a record there), and both repository create paths call it
// again before anything is written (the one after the previous system, before
// it takes the system identity).
func RefuseExternalCreateDeviation(ctx context.Context, deviation *string) error {
	if normalizeImplementationDeviation(deviation) == nil || !IsExternalCaller(ctx) {
		return nil
	}
	return &apierror.ForbiddenError{Msg: externalCreateDeviationRefusal, Code: apierror.CodeChangeRequestForbidden}
}

// deviationRecordableState reports whether the (upper-case) stored state is one
// in which the deviation can be recorded: implementation has started and the
// change request is not final yet.
func deviationRecordableState(state string) bool {
	switch state {
	case crStateImplement, crStateReview, crStateCustomerReview:
		return true
	}
	return false
}

// deviationFinalMsg is the 400 for a PATCH that changes the deviation of a
// change request in a final state.
func deviationFinalMsg(state string) string {
	word := lockStateName(state)
	if state == crStateRollback {
		word = "rolled back"
	}
	return fmt.Sprintf("implementationDeviation can no longer be changed: the change request is %s, so its record is final", word)
}

// deviationCarriedOverMsg is the 400 for a PATCH that sets or changes (rather
// than clears) the deviation a change request in New carried over from the
// change request it was cloned from.
const deviationCarriedOverMsg = "implementationDeviation can only be cleared before implementation: fold the deviation from the implementation plan carried over from the cloned change request into the implementation plan, then clear it"

// deviationNotYetMsg is the 400 for a PATCH that changes the deviation before
// implementation has started (a change request in New that has none, or one in
// Assess, Authorize, Customer Approval or Scheduled).
func deviationNotYetMsg(state string) string {
	return fmt.Sprintf("implementationDeviation cannot be changed yet: the deviation from the implementation plan can be recorded once implementation has started (current state: %s)", lockStateName(state))
}

// deviationNotIncorporatedMsg is the 400 for Request Approval while the change
// request still has a deviation from the implementation plan.
const deviationNotIncorporatedMsg = "approval cannot be requested until the deviation from the implementation plan carried over from the cloned change request has been folded into the implementation plan and cleared"

// checkDeviationEditable refuses a change of the deviation in a stored state in
// which it cannot be recorded: before implementation (400
// CodeChangeRequestDeviationNotYet; in New clearing one is accepted) and once the
// state is final (400 CodeChangeRequestDeviationFinal). deviation is the
// request's value (nil when the request does not carry the field, which is never
// judged). The comparison is with the stored column, both normalised (a blank
// value on either side is none), so a resend is accepted. Nothing is queried in a
// state in which it can be recorded.
func checkDeviationEditable(ctx context.Context, q crQuerier, id string, snap changeRequestGateSnapshot, deviation *string) error {
	if deviation == nil || deviationRecordableState(snap.state) {
		return nil
	}
	requested := normalizeImplementationDeviation(deviation)
	stored, err := readStoredImplementationDeviation(ctx, q, id)
	if err != nil {
		var nf *apierror.NotFoundError
		if errors.As(err, &nf) {
			return err
		}
		return fmt.Errorf("patch change request: compare deviation: %w", err)
	}
	if sameImplementationDeviation(stored, requested) {
		return nil
	}
	return deviationEditRefusal(snap.state, stored != nil, requested == nil)
}

// deviationEditRefusal is the refusal (or nil) for a request that CHANGES the
// deviation of a change request in a state in which it cannot be recorded.
// stored says whether one is stored; clearing whether the request clears it.
func deviationEditRefusal(state string, stored, clearing bool) error {
	switch {
	case deviationRecordableState(state):
		return nil
	case terminalChangeRequestState(state):
		return &apierror.ValidationError{Msg: deviationFinalMsg(state), Code: apierror.CodeChangeRequestDeviationFinal}
	case changeRequestCreationPhase(state):
		if clearing {
			return nil
		}
		if stored {
			return &apierror.ValidationError{Msg: deviationCarriedOverMsg, Code: apierror.CodeChangeRequestDeviationNotYet}
		}
	}
	return &apierror.ValidationError{Msg: deviationNotYetMsg(state), Code: apierror.CodeChangeRequestDeviationNotYet}
}

// checkRequestApprovalDeviationCleared is a Request Approval guard ({state:
// assess} on a change still in the creation phase): it is refused while the
// deviation in effect is not blank -- the request's own value when it carries
// the field, else the stored one, both by the blank rule above. A deviation in
// New can only have come from the create (the CSM webapp sends one only for a
// clone; a PATCH in New can only clear it), and the change has to go for
// approval with it folded into its implementation plan; clearing it (and editing
// the plan) in the same PATCH is accepted. A resend on a change that has already
// left New is not Request Approval and is never judged here.
func checkRequestApprovalDeviationCleared(ctx context.Context, q crQuerier, id, state string, deviation *string) error {
	if !changeRequestCreationPhase(state) {
		return nil
	}
	if deviation != nil {
		if normalizeImplementationDeviation(deviation) == nil {
			return nil
		}
		return &apierror.ValidationError{Msg: deviationNotIncorporatedMsg, Code: apierror.CodeChangeRequestDeviationNotIncorporated}
	}
	stored, err := readStoredImplementationDeviation(ctx, q, id)
	if err != nil {
		var nf *apierror.NotFoundError
		if errors.As(err, &nf) {
			return err
		}
		return fmt.Errorf("patch change request: read deviation: %w", err)
	}
	if stored == nil {
		return nil
	}
	return &apierror.ValidationError{Msg: deviationNotIncorporatedMsg, Code: apierror.CodeChangeRequestDeviationNotIncorporated}
}
