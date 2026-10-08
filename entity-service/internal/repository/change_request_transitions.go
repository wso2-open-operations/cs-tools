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
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// This file owns the transition graph of PATCH {state}: which states a staff
// request may name while the change request is in a given state. It is ONE
// table -- changeRequestForwardNextStates and changeRequestRollbackFrom in
// change_request_repo.go, with Cancel from every non-final state -- read twice:
// legalChangeRequestNextStates renders it as legalNextStates (what the portal
// offers), and checkStaffStateRequest enforces it in patchChangeRequestTx (what
// the service accepts). They cannot drift apart because there is nothing else to
// read.
//
// THE GRAPH, for a staff request (the customer's own answers are not PATCHes of
// the state at all: answerCustomerStageViaPatch; the customer's proposed time is a
// start written to customer_updated_on that waits for WSO2, change_request_customer_proposal.go):
//
//	from              staff may request
//	new               assess (Request Approval), canceled
//	assess            canceled                      (waits for the peer approval)
//	authorize         canceled                      (waits for the CAB approval)
//	customer_approval authorize (Re-schedule), canceled   (waits for the customer)
//	                  "authorize" is the wire name of the Time Change loop, NOT a destination:
//	                  the change stays in customer_approval, the customers are asked again and
//	                  nothing goes through CAB again.
//	scheduled         implement, canceled
//	implement         review, canceled
//	review            closed | customer_review (by customer_review_required), rollback, canceled
//	customer_review   rollback, canceled            (waits for the customer)
//	closed, canceled, rollback: nothing -- final
//
// The states a change request reaches only through an approval are not edges:
// Assess -> Authorize (peer approval), Authorize -> Scheduled | Customer Approval
// (CAB approval), Customer Approval -> Scheduled (the customer's approval, OR WSO2's
// acceptance of the customer's own proposed time: "Accept proposed time" is its own
// request, confirmCustomerUpdatedDate, only while that proposal waits -- never a staff-named
// {state: "scheduled"}, which stays refused whatever is pending) and
// Customer Review -> Closed | Rollback (the customer's review). A request may
// also name the state the change is already in (a resend: no move).
//
// A request that names a state outside the graph is a 400 and changes nothing.
// Where a state has a refusal of its own that says more than "not an edge" --
// scheduled / customer_approval / authorize (reached by the approval flow),
// rollback (review states only), assess (Request Approval), anything out of a
// customer state (the customer's answer) -- patchChangeRequestTx's cases for it
// give that refusal; checkStaffStateRequest handles every other destination.

// normalizeRequestedChangeRequestState puts a requested state in the form every
// comparison here uses -- trimmed, lower case, which is how the domain names the
// states -- and refuses one that is not a state of the change request lifecycle.
func normalizeRequestedChangeRequestState(s domain.ChangeRequestState) (domain.ChangeRequestState, error) {
	n := domain.ChangeRequestState(strings.ToLower(strings.TrimSpace(string(s))))
	if !knownChangeRequestStates[strings.ToUpper(string(n))] {
		return "", &apierror.ValidationError{Msg: fmt.Sprintf("state %q is not a change request state", string(s))}
	}
	return n, nil
}

// changeRequestStaffTargets is every state a staff PATCH may name while the
// change is in the given state, whatever customer_review_required says (Review
// lists both Closed and Customer Review: the flag picks one, in
// patchChangeRequestTx's own cases and in legalChangeRequestNextStates). Final
// states have none.
func changeRequestStaffTargets(st domain.ChangeRequestState) []domain.ChangeRequestState {
	nexts, ok := changeRequestForwardNextStates[st]
	if !ok {
		return nil
	}
	out := append([]domain.ChangeRequestState{}, nexts...)
	if changeRequestRollbackFrom[st] {
		out = append(out, domain.ChangeRequestStateRollback)
	}
	return append(out, domain.ChangeRequestStateCanceled)
}

// changeRequestFinalPhrase is how a final state reads in "a change request that
// is ... cannot be moved".
func changeRequestFinalPhrase(state string) string {
	if state == crStateRollback {
		return "rolled back"
	}
	return strings.ToLower(state)
}

// changeRequestFinalRefusal is the 400 for every attempt to move a closed,
// canceled or rolled-back change request: those states have no exit by PATCH,
// whatever is asked, and revive nothing (a canceled change in Customer Approval
// cannot be sent to Implement, a closed one cannot be re-opened).
func changeRequestFinalRefusal(current string, requested domain.ChangeRequestState) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf(
		"state %q cannot be set manually from %s: a change request that is %s cannot be moved",
		string(requested), strings.ToLower(current), changeRequestFinalPhrase(current))}
}

// changeRequestWaitingOn is what the change is waiting for in a state staff
// cannot move it out of, for the refusal message of a jump.
var changeRequestWaitingOn = map[string]string{
	crStateNew:       `approval has not been requested yet (Request Approval is state "assess")`,
	crStateAssess:    "it is waiting for its peer approval, which moves it on by itself",
	crStateAuthorize: "it is waiting for its CAB approval, which moves it on by itself (to Customer Approval first when the customer's approval is required)",
	crStateScheduled: "a change request goes through implement and review in order, one step at a time",
	crStateImplement: "a change request goes through implement and review in order, one step at a time",
	crStateReview:    "a change request cannot go back to an earlier step",
}

// changeRequestJumpRefusal is the 400 for a request that names a state which is
// not an edge of the graph out of the current one: nothing is skipped (the
// approvals, the customer's gate) and nothing goes back.
func changeRequestJumpRefusal(current string, requested domain.ChangeRequestState, reviewRequired bool) error {
	why := changeRequestWaitingOn[current]
	if why == "" {
		why = "that is not a step of the change request's lifecycle from here"
	}
	cur := strings.ToLower(current)
	open := legalChangeRequestNextStates(&cur, reviewRequired)
	opens := "none"
	switch len(open) {
	case 0:
	case 1:
		opens = "only " + open[0]
	default:
		opens = strings.Join(open, ", ")
	}
	return &apierror.ValidationError{Msg: fmt.Sprintf(
		"state %q cannot be set manually from %s: %s; the moves open to staff from %s are: %s",
		string(requested), cur, why, cur, opens)}
}

// checkStaffStateRequest enforces the graph for a staff PATCH {state}:
// current is the change's upper-case state ("" for NULL, which counts as New),
// requested the normalized one. It returns nil for a resend (the state the
// change is already in), for every edge of the table, and for the requests whose
// refusal patchChangeRequestTx's cases own (see the file comment); it refuses a
// final state's every other request and every jump.
//
// A customer state is not judged here: refuseStaffExitFromCustomerState and the
// cases for closed / rollback give the refusals that name the customer's answer,
// and they accept exactly the table's moves out of it (Cancel, Re-schedule,
// Roll back).
func checkStaffStateRequest(current string, requested domain.ChangeRequestState, reviewRequired bool) error {
	if current == "" {
		current = crStateNew
	}
	cur := domain.ChangeRequestState(strings.ToLower(current))
	if requested == cur {
		return nil
	}
	if terminalChangeRequestState(current) {
		return changeRequestFinalRefusal(current, requested)
	}
	if customerStageSpecForState(current) != nil {
		return nil
	}
	switch requested {
	case domain.ChangeRequestStateNew, domain.ChangeRequestStateAssess,
		domain.ChangeRequestStateAuthorize, domain.ChangeRequestStateCustomerApproval,
		domain.ChangeRequestStateScheduled, domain.ChangeRequestStateRollback:
		// Each has a refusal of its own (new: the lock's return-to-New rule;
		// assess: Request Approval; authorize / customer_approval / scheduled:
		// the approval flow; rollback: the review states).
		return nil
	}
	for _, next := range changeRequestStaffTargets(cur) {
		if next == requested {
			return nil
		}
	}
	return changeRequestJumpRefusal(current, requested, reviewRequired)
}
