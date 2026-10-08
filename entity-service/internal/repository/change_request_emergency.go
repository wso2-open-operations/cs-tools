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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// This file owns the Emergency change rule: an Emergency change is acted on
// WITHOUT the customer's consent, so it never ENTERS Customer Approval or
// Customer Review. (One that is already waiting in one -- see below -- is not taken
// out of the customer's hands: the question it was given stands.)
//
// Two halves, because the data has two origins:
//
//   - Changes created here. The creation form's two boxes (customerApprovalRequired /
//     customerReviewRequired, change_request.customer_approval_required /
//     customer_review_required, migration 0189) cannot be set on an Emergency change:
//     a create that does, a PATCH that turns one on, and a PATCH that re-types a
//     change that has one ticked INTO Emergency are all refused with a 400
//     (ValidateCreateChangeRequestCustomerGates, checkEmergencyCustomerConsent). The
//     type is frozen once approval has been requested (checkChangeTypeEdit), so a
//     re-type is only ever possible in New.
//   - Changes that already exist -- one created before this rule with a box ticked, or a
//     migrated one (whose requirement flags are the previous system's, written by the
//     sync, and never ours to edit). Nothing is rewritten and reads show what is
//     stored, but the FLOW ignores the boxes for an Emergency change
//     (effectiveCustomerGates): CAB approval schedules it and Review closes it, so
//     nothing this flow does takes it into a customer state. A write of the
//     value a box already holds is a no-op and is accepted, like everywhere in the lock
//     (change_request_customer_lock.go): a client that sends the whole form back is not
//     punished for a legacy ticked box it did not touch.
//
// What the rule does NOT do is strand a change that is already in a customer state (a
// row from before the rule, or one the previous system itself sent to the customer): there the
// customer's question stands and every act of the loop works as on any other change --
// the customer's own answer moves it, a Re-schedule or a counter-proposal asks the
// project's contacts again, and one that cannot ask anybody is refused whole
// (provisionCustomerStage, legacyStageWouldBeProvisioned, requireSomebodyToAskForWindow).
// Cancelling a customer's request that nobody then replaces is never an outcome.

// changeModelEmergency is change_request.change_model's label for an Emergency change.
const changeModelEmergency = "EMERGENCY"

// emergencyNoCustomerConsentMsg is the reason every refusal of this rule gives.
const emergencyNoCustomerConsentMsg = "Emergency changes proceed without customer consent, so customer approval and customer review cannot be required"

// isEmergencyModel reports whether a change_model label (any case, "" for NULL) is Emergency.
func isEmergencyModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), changeModelEmergency)
}

// effectiveChangeModel is the change_model the change will have once a PATCH is written:
// the requested type's label when the request carries one that has a label, else the
// stored one (upper-case label, "" for NULL).
func effectiveChangeModel(stored string, requested *domain.ChangeRequestType) string {
	if requested != nil {
		if m, ok := changeRequestTypeToChangeModel[*requested]; ok {
			return m
		}
	}
	return stored
}

// effectiveCustomerGates is the customer requirement the approval flow acts on: the two
// boxes as stored, except that an Emergency change has neither -- it never asks the
// customer. Every decision the flow takes from a box (where CAB approval leads, whether
// Review leads to Customer Review, which next states are offered, whether a customer
// stage is asked for) goes through it; the stored value itself is left alone.
func effectiveCustomerGates(model string, approvalRequired, reviewRequired bool) (approval, review bool) {
	if isEmergencyModel(model) {
		return false, false
	}
	return approvalRequired, reviewRequired
}

// legalChangeRequestNextStatesForModel is legalChangeRequestNextStates for a change of the
// given (upper-case) change_model: the review box is judged as the flow reads it
// (effectiveCustomerGates), so an Emergency change's Review offers Closed whatever its
// stored box says.
func legalChangeRequestNextStatesForModel(state *string, model string, customerReviewRequired bool) []string {
	_, review := effectiveCustomerGates(model, false, customerReviewRequired)
	return legalChangeRequestNextStates(state, review)
}

// ValidateCreateChangeRequestCustomerGates is the create-time half of the rule: an
// Emergency change cannot be created with either customer box ticked. Exported, like
// ValidateCreateChangeRequestType, so every create path applies the identical rule and
// message -- in particular BEFORE the previous system is called on the dual-write path, where a
// refusal after the fact would strand a record there with no PostgreSQL row.
func ValidateCreateChangeRequestCustomerGates(t *domain.ChangeRequestType, approvalRequired, reviewRequired *bool) error {
	if t == nil || !isEmergencyModel(changeRequestTypeToChangeModel[*t]) {
		return nil
	}
	var fields []string
	if approvalRequired != nil && *approvalRequired {
		fields = append(fields, customerApprovalBox.field)
	}
	if reviewRequired != nil && *reviewRequired {
		fields = append(fields, customerReviewBox.field)
	}
	if len(fields) == 0 {
		return nil
	}
	return &apierror.ValidationError{Msg: emergencyNoCustomerConsentMsg + " (" + strings.Join(fields, " and ") + " must be false for an Emergency change)"}
}

// checkEmergencyCustomerConsent is the PATCH half of the rule (a creation-phase rule, run
// with the others in validateCreationPhaseEdits, from every state): on a change that is
// Emergency once the request is written, the request must not
//
//   - turn a box ON (a value of true where false is stored), and
//   - carry a change INTO Emergency (a type other than the stored one) while a box would
//     stay ticked -- the request's own value for the box, else the stored one.
//
// A write of the value a box already holds is a no-op and is accepted (see the file
// comment); turning a legacy ticked box off is the lock's business (add-only after New).
func checkEmergencyCustomerConsent(snap changeRequestGateSnapshot, req domain.PatchChangeRequestRequest) error {
	if !isEmergencyModel(effectiveChangeModel(snap.model, req.Type)) {
		return nil
	}
	retyped := req.Type != nil && !resendsStoredChangeType(snap.model, req.Type)
	var fields []string
	check := func(box customerRequirementBox, stored bool, requested *bool) {
		after := stored
		if requested != nil {
			after = *requested
		}
		if !after {
			return
		}
		if retyped || (requested != nil && !stored) {
			fields = append(fields, box.field)
		}
	}
	check(customerApprovalBox, snap.approvalRequired, req.CustomerApprovalRequired)
	check(customerReviewBox, snap.reviewRequired, req.CustomerReviewRequired)
	if len(fields) == 0 {
		return nil
	}
	if retyped {
		return &apierror.ValidationError{Msg: emergencyNoCustomerConsentMsg + ": turn off " + strings.Join(fields, " and ") + " before changing the type to emergency"}
	}
	return &apierror.ValidationError{Msg: emergencyNoCustomerConsentMsg + " (" + strings.Join(fields, " and ") + " must be false for an Emergency change)"}
}
