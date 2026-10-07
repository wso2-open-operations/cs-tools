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
	// was recorded; reading the change request again shows the new time.
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

	// CodeChangeRequestNotAsked is the 403 for a contact of the project whom the
	// customer's request was never sent to (or whose request a sibling's answer or
	// a re-schedule withdrew): only the contacts asked may answer or propose.
	CodeChangeRequestNotAsked = "change_request_not_asked"

	// CodeChangeRequestForbidden is the 403 for every other refusal of who may
	// act: not a registered contact of the change request's project, the change
	// request's own creator, or a field a customer may not set. The caller may
	// not do this here, now or later.
	CodeChangeRequestForbidden = "change_request_forbidden"
)
