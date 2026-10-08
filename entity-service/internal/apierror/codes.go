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

package apierror

// Machine-readable names of the refusals a client has to tell apart from the
// other refusals of the same HTTP status, returned as the error body's
// errorCode (ErrorResponse.ErrorCode) beside the human-readable message.
//
// They exist because a client must never branch on the wording of a message:
// the words are for people and change as they are improved, a code does not.
// A code is a contract, so it is only ever added to, never renamed or reused
// for another meaning, and it is a plain lower-case snake_case string. It never
// changes the HTTP status or the message of the refusal it names, and nothing
// is stored: it is attached where the refusal is raised.
//
// These are the refusals of a customer's answer to a change request and of a
// proposed implementation time (PATCH /change-requests/{id} from an external
// caller), and of the approvals/decision route they share a rule set with. A
// client that does not know a code (a newer server than the client) handles
// the refusal as it would any other of its status; a client that gets none (an
// older server) does the same.
const (
	// CodeChangeRequestOnHold is the 409 for a proposed implementation time on a
	// change request that WSO2 has on hold. The customer is still being asked, so
	// the answer itself is still possible; only the proposal is refused.
	CodeChangeRequestOnHold = "change_request_on_hold"

	// CodeChangeRequestScheduleChanged is the 409 for an answer that names the
	// planned window the customer was shown, when the change request's window
	// is no longer that one (it was re-scheduled behind an open page). Nothing
	// was recorded; reading the change request again shows the new time. WSO2's
	// own response to a customer's proposed time (Accept proposed time, propose
	// a different time) names the window it was shown the same way and is
	// refused with the same code.
	CodeChangeRequestScheduleChanged = "change_request_schedule_changed"

	// CodeChangeRequestApprovalNotPending is the 409 for an answer to an approval
	// that is no longer waiting for one: it was already answered (by the caller or
	// by a sibling contact), the change request has left the state the answer
	// belongs to, or no request for the answer is open at all.
	CodeChangeRequestApprovalNotPending = "change_request_approval_not_pending"

	// CodeChangeRequestNotProposable is the 409 for a proposed implementation
	// time on a change request that is not open to one: it is not in Customer
	// Approval, or nobody has been asked for the customer's approval.
	CodeChangeRequestNotProposable = "change_request_not_proposable"

	// CodeChangeRequestProposalNotNow is the 409 for a proposed implementation
	// time that cannot be taken right now although the change request is in
	// Customer Approval and the customer is being asked: another approval (not
	// the customer's) is being asked at the same time. The customer's own answer
	// is still possible; only the proposal is refused, as with
	// CodeChangeRequestOnHold.
	CodeChangeRequestProposalNotNow = "change_request_proposal_not_now"

	// CodeChangeRequestNoPlannedWindow is the 409 for a proposed implementation
	// time on a change request that has no planned window to move (a proposal
	// is a new start, and the planned length is kept). The customer is still
	// being asked; only the proposal is refused, as with CodeChangeRequestOnHold.
	CodeChangeRequestNoPlannedWindow = "change_request_no_planned_window"

	// CodeChangeRequestProposerNotRecorded is the 409 for WSO2's acceptance of a
	// stored time that nobody is recorded as having proposed: the date may have been
	// written by someone at WSO2 or left over from an earlier cycle, and accepting it
	// would schedule the change for a time no customer ever consented to. No staff
	// action stands in for the customer's own answer, so the acceptance is refused;
	// proposing a different time (which asks the customers to approve it) still works.
	CodeChangeRequestProposerNotRecorded = "change_request_proposer_not_recorded"

	// CodeChangeRequestNotAsked is the 403 for a contact of the project who holds
	// no REQUESTED row on the customer stage that is LIVE (the customer is being
	// asked, and this contact is not among those asked): registered after the
	// request went out, or a row of theirs cancelled directly. Only the contacts
	// asked may answer or propose. A request that was withdrawn -- a sibling's
	// answer settled the stage, or the change left the state -- is not this refusal:
	// no stage is live then, and an answer is a 409
	// CodeChangeRequestApprovalNotPending (a proposal, a 409
	// CodeChangeRequestNotProposable).
	CodeChangeRequestNotAsked = "change_request_not_asked"

	// CodeChangeRequestForbidden is the 403 for every other refusal of who may
	// act: not a registered contact of the change request's project, the change
	// request's own creator, or a field a customer may not set. The caller may
	// not do this here, now or later.
	CodeChangeRequestForbidden = "change_request_forbidden"
)
