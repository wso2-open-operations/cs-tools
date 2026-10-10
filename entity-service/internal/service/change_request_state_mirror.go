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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The operations a change request's mirror writes are recorded under in
// sn_writeback_failures (and replayed by).
const (
	// snWritebackOpPatch: the fields of a PATCH, without its state.
	snWritebackOpPatch = "patch"
	// snWritebackOpState: one state move PostgreSQL made that the mirror
	// writes to the previous system itself (payload {"state": <ours>}):
	// Request Approval, a staff move, Accept proposed time, the GitHub sync.
	snWritebackOpState = "state"
	// snWritebackOpStateCheck: one state move PostgreSQL made that the
	// previous system's own engine is expected to make from the mirrored
	// decision or answer (payload {"state": <ours>, "after": <cause>}); the
	// job reads the record back and records a divergence.
	snWritebackOpStateCheck = "state_check"
	// snWritebackOpDecision: an approval decision ("approved" / "rejected").
	snWritebackOpDecision = "approval_decision"
	// snWritebackEntityChangeRequest is the entity type of all of them.
	snWritebackEntityChangeRequest = "change_request"
)

// stateMirrorMode says how a state move is kept in step with the previous
// system.
type stateMirrorMode int

const (
	// stateMirrorWrite: this service writes the state (or Request Approval)
	// itself, after reading the record and checking its change model
	// (writeState).
	stateMirrorWrite stateMirrorMode = iota
	// stateMirrorCheck: the previous system's own engine makes the move from
	// the decision / answer mirrored just before; this service only reads the
	// record back and records a divergence (checkState).
	stateMirrorCheck
)

// snStatePayload is what a "state" / "state_check" failure row holds.
type snStatePayload struct {
	State string `json:"state"`
	After string `json:"after,omitempty"`
}

// changeRequestStateMirror is the ONE rule that keeps the previous system's
// record in step with change_request.state under
// DATA_SOURCE=postgres-servicenow-dual-write: after a repository transaction
// commits, if the state it committed differs from the state it found (both
// read under the row lock inside that transaction: repository.ChangeRequestStates),
// one job for that move is dispatched after whatever fields the same call
// mirrored, so the writes reach the previous system in that order (the
// dispatcher serializes the writes of one record). Nothing is dispatched when
// the state did not move -- a resend of the state the change is in, a field
// edit, the Re-schedule / counter-proposal / decline wire (`{state:
// "authorize"}` out of Customer Approval, which does not move the state), a
// customer's proposed window, a refused request -- so the rule is idempotent:
// a state the sync from the previous system has already written back equal
// to ours never passes through a service call and is never re-sent, a move
// already mirrored is not mirrored twice, and a job that finds the previous
// system already in our state writes nothing.
//
// The previous system enforces its change model on every write (verified; see
// snChangeModelMoves), so HOW a move is mirrored depends on its cause:
//
//   - Request Approval (New -> Assess): `requestApproval: true`, its own
//     action, which moves its record to Assess and lets its policy provision
//     its approval stage. Never a state write of Assess.
//   - A staff move (Cancel, Implement, Review, Closed), Request Approval on
//     a type whose first state is written directly (Authorize, Scheduled),
//     Accept proposed time (-> Scheduled) and the GitHub sync's SetState:
//     a direct state write, but only after reading the record and finding a
//     move its model allows from ITS current state (snDirectStateMoveAllowed);
//     otherwise, and for the states its model has no direct move into at all
//     (Customer Approval, Customer Review, Rollback: snStateInModel), the move
//     is NOT sent and is recorded as a visible write-back failure that names
//     the state, the reason and the state the record is in. A write it still
//     refuses (a bare 500 through the pass-through: its model, or its
//     "Mandatory Assignment Group" rule on a record with no group) is recorded
//     the same way, with the state that was sent in the reason.
//   - The approval cascade (Assess -> Authorize on the peer approval,
//     Authorize -> Scheduled or Customer Approval on the CAB approval, a
//     customer stage's outcome through the decision route) and the customer's
//     own answer through the PATCH (isCustomerApproved / isCustomerReviewed:
//     Customer Approval -> Scheduled, Customer Review -> Closed, a rejection ->
//     Canceled / Rollback): NO state write. The decision / the answer is
//     mirrored (its own paths) and its engine moves its record; the job that
//     follows reads the record back and, when its state is not ours, records
//     a "state divergence" failure (ours / theirs) for a person to resolve.
//
// A refused or diverged move leaves PostgreSQL committed; the row in
// sn_writeback_failures is listed on the change request's detail
// (domain.ChangeRequest.MirrorFailures) and replayed on request, which
// re-runs the same job (read, check, write) and clears the row only when the
// record is in step.
type changeRequestStateMirror struct {
	snMirror    ChangeRequestService
	snWriteback *SNWritebackDispatcher
}

// mirrorStateMove applies the rule to what a repository transaction reported.
// cause names the call, for the log line and the failure row.
func (m changeRequestStateMirror) mirrorStateMove(ctx context.Context, id string, states repository.ChangeRequestStates, cause string, mode stateMirrorMode) {
	if m.snWriteback == nil || !states.Moved() {
		return
	}
	committed := ""
	if states.Committed != nil {
		committed = *states.Committed
	}
	m.dispatchState(ctx, id, committed, cause, mode)
}

// dispatchState queues the job for a move PostgreSQL committed.
func (m changeRequestStateMirror) dispatchState(ctx context.Context, id, committed, cause string, mode stateMirrorMode) {
	if m.snWriteback == nil {
		return
	}
	state := strings.ToLower(strings.TrimSpace(committed))
	mirrorID := id
	switch mode {
	case stateMirrorCheck:
		slog.InfoContext(ctx, "change request mirror: state move left to the previous system's engine; read-back queued",
			"changeRequestId", id, "state", state, "cause", cause)
		m.snWriteback.Dispatch(ctx, snWritebackEntityChangeRequest, id, snWritebackOpStateCheck, snStatePayload{State: state, After: cause},
			func(writeCtx context.Context) error { return m.checkState(writeCtx, mirrorID, state, cause) })
	default:
		slog.InfoContext(ctx, "change request mirror: state move dispatched to the previous system",
			"changeRequestId", id, "state", state, "cause", cause)
		m.snWriteback.Dispatch(ctx, snWritebackEntityChangeRequest, id, snWritebackOpState, snStatePayload{State: state, After: cause},
			func(writeCtx context.Context) error { return m.writeState(writeCtx, mirrorID, state) })
	}
}

// theirState reads the previous system's record and returns its state as
// this service's enum string ("" when it has none or an unknown label).
func (m changeRequestStateMirror) theirState(ctx context.Context, id string) (domain.ChangeRequestState, error) {
	cur, err := m.snMirror.GetChangeRequest(ctx, id)
	if err != nil {
		return "", fmt.Errorf("the previous system's record could not be read: %w", err)
	}
	if cur.State == nil {
		return "", nil
	}
	return domain.ChangeRequestState(strings.ToLower(strings.TrimSpace(*cur.State))), nil
}

// writeState is the "state" job: read the record, and write our state only
// as the previous system's model allows it. A refusal is a ConflictError (a
// 409 on the replay route) naming the state and the reason; it is recorded
// like any failed write.
func (m changeRequestStateMirror) writeState(ctx context.Context, id, state string) error {
	ours, ok := snMirrorableState(state)
	if !ok {
		return &apierror.ConflictError{Msg: fmt.Sprintf("state %q is not mirrorable: the previous system's change request API has no key for it; not sent", state)}
	}
	theirs, err := m.theirState(ctx, id)
	if err != nil {
		return fmt.Errorf("state %q not sent: %w", state, err)
	}
	if theirs == ours {
		// Already in step (its engine got there, an earlier write landed, or
		// the sync brought it back): nothing to write.
		return nil
	}
	yes := true
	switch {
	case ours == domain.ChangeRequestStateAssess && theirs == domain.ChangeRequestStateNew:
		// Request Approval: the previous system's own action, which moves its
		// record to Assess and provisions its approval stage.
		if _, err := m.snMirror.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{RequestApproval: &yes}); err != nil {
			return fmt.Errorf("Request Approval (requestApproval: true, for state %q; the previous system was in %q) failed: %w", state, theirs, err)
		}
		return nil
	case !snStateInModel(ours):
		return &apierror.ConflictError{Msg: fmt.Sprintf(
			"state %q is not mirrorable: the previous system's change model has no direct move into it (it reaches it only through its own paths); its record is in %q; not sent", state, string(theirs))}
	case !snDirectStateMoveAllowed(theirs, ours):
		return &apierror.ConflictError{Msg: fmt.Sprintf(
			"transition to %q is not allowed by the previous system's change model from its current state %q; not sent", state, string(theirs))}
	}
	if _, err := m.snMirror.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{State: &ours}); err != nil {
		return fmt.Errorf("state write {state: %q} (the previous system was in %q) failed: %w", state, string(theirs), err)
	}
	return nil
}

// checkState is the "state_check" job: the previous system's engine was
// expected to make this move from the decision / answer mirrored before it;
// read the record back and record a divergence when it did not. Nothing is
// written: what to do about a divergence is a person's decision.
func (m changeRequestStateMirror) checkState(ctx context.Context, id, state, cause string) error {
	theirs, err := m.theirState(ctx, id)
	if err != nil {
		return fmt.Errorf("state %q after %s not checked: %w", state, cause, err)
	}
	if string(theirs) == state {
		return nil
	}
	return &apierror.ConflictError{Msg: fmt.Sprintf("state divergence after %s: ours=%q theirs=%q", cause, state, string(theirs))}
}

// registerReplays tells the dispatcher how each of the change request's
// recorded failures is re-sent (SNWritebackDispatcher.ReplaySNWritebackFailure):
// a "patch" row holds a PatchChangeRequestRequest, a "state" / "state_check"
// row the state (the job runs again: read, check, write / compare), an
// "approval_decision" row the decision string, exactly as Dispatch marshaled
// them.
func (m changeRequestStateMirror) registerReplays() {
	if m.snWriteback == nil {
		return
	}
	m.snWriteback.RegisterReplay(snWritebackEntityChangeRequest, snWritebackOpPatch, func(ctx context.Context, entityID string, payload json.RawMessage) error {
		var req domain.PatchChangeRequestRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return fmt.Errorf("replay change request patch: the recorded payload is not a PATCH body: %w", err)
		}
		_, err := m.snMirror.PatchChangeRequest(ctx, entityID, req)
		return err
	})
	m.snWriteback.RegisterReplay(snWritebackEntityChangeRequest, snWritebackOpState, func(ctx context.Context, entityID string, payload json.RawMessage) error {
		var p snStatePayload
		if err := json.Unmarshal(payload, &p); err != nil || p.State == "" {
			return fmt.Errorf("replay change request state: the recorded payload names no state: %v", err)
		}
		return m.writeState(ctx, entityID, p.State)
	})
	m.snWriteback.RegisterReplay(snWritebackEntityChangeRequest, snWritebackOpStateCheck, func(ctx context.Context, entityID string, payload json.RawMessage) error {
		var p snStatePayload
		if err := json.Unmarshal(payload, &p); err != nil || p.State == "" {
			return fmt.Errorf("replay change request state check: the recorded payload names no state: %v", err)
		}
		return m.checkState(ctx, entityID, p.State, p.After)
	})
	m.snWriteback.RegisterReplay(snWritebackEntityChangeRequest, snWritebackOpDecision, func(ctx context.Context, entityID string, payload json.RawMessage) error {
		var decision string
		if err := json.Unmarshal(payload, &decision); err != nil {
			return fmt.Errorf("replay change request approval decision: the recorded payload is not a decision: %w", err)
		}
		_, err := m.snMirror.DecideChangeRequestApproval(ctx, entityID, decision)
		return err
	})
}

// mirrorFailuresOf lists what of the change request the previous system is
// still missing (sn_writeback_failures by its id, oldest first), for the
// detail response. nil when the read failed -- logged, never an error for the
// detail -- so a caller can tell "nothing outstanding" ([]) from "unknown".
func (m changeRequestStateMirror) mirrorFailuresOf(ctx context.Context, id string) []domain.ChangeRequestMirrorFailure {
	rows, err := m.snWriteback.ListFailures(ctx, snWritebackEntityChangeRequest, id)
	if err != nil {
		slog.ErrorContext(ctx, "change request mirror: the write-back failures of the change request could not be read",
			"changeRequestId", id, "error", err)
		return nil
	}
	out := make([]domain.ChangeRequestMirrorFailure, 0, len(rows))
	for _, f := range rows {
		out = append(out, domain.ChangeRequestMirrorFailure{ID: f.ID, Operation: f.Operation, Payload: f.Payload, Error: f.Error, CreatedOn: f.CreatedOn})
	}
	return out
}

// mirrorablePatchFields is the field coverage of the PATCH mirror: it returns
// req with every field the previous system's change request API accepts kept
// exactly as sent, and every other field removed, so none of them is ever
// sent. KEPT (each has a field on that API's PATCH): title, description,
// projectId, caseId, deploymentId, deployedProductId, assignedEngineerId,
// assignedTeamId, plannedStartOn, plannedEndOn (converted to the layout that
// API takes, by the caller), impact, type, justification, impactDescription,
// serviceOutage, communicationPlan, rollbackPlan, testPlan,
// isCustomerApproved, isCustomerReviewed, requestApproval,
// isPlanningVisibleToCustomers, implementationPlan, priority, category,
// requestedById, affectedServicesText, affectedComponentsText,
// rollbackDurationText, comment, workNote (durationInput is refused by this
// data source before anything is written, so it never reaches here).
//
// DELIBERATELY NOT MIRRORED, because that API has no field for them (they
// are PostgreSQL-only and stay so; the list is the whole list):
//   - the proposal conversation: confirmCustomerUpdatedDate (Accept proposed
//     time), expectedCustomerUpdatedOn, expectedPlannedStartOn /
//     expectedPlannedEndOn (the window an answer was given for) and a
//     customer's proposed window itself (plannedStartOn / plannedEndOn sent
//     by an external caller: mirrorOfTheTimeConversation);
//   - the two requirement checkboxes customerApprovalRequired /
//     customerReviewRequired (the creation form's requirement, not the
//     customer's outcome, which isCustomerApproved / isCustomerReviewed are);
//   - deploymentIds and deploymentProductIds (that API carries one
//     deployment and one deployed product, and its deployment products are
//     its own records, not the ids PostgreSQL derives);
//   - customerGroupId and environmentIds (refused before anything else: the
//     Customer Group is derived from the project's registered contacts);
//   - onHold / onHoldReason and serviceId / serviceOfferingId (no field on
//     that API's PATCH);
//   - state: never from the request. The state the previous system gets is
//     the one PostgreSQL COMMITTED, handled by changeRequestStateMirror after
//     the fields, and only when it moved;
//   - our approval stage rows (approval_stage / approval_stage_approver,
//     which the approval flow provisions and cancels) are not fields of the
//     request at all and are never written to the previous system: its own
//     approvals are driven by the mirrored decision (decideApproval).
func mirrorablePatchFields(req domain.PatchChangeRequestRequest) domain.PatchChangeRequestRequest {
	m := req
	m.State = nil
	m.ConfirmCustomerUpdatedDate, m.ExpectedCustomerUpdatedOn = nil, nil
	m.ExpectedPlannedStartOn, m.ExpectedPlannedEndOn = nil, nil
	m.CustomerApprovalRequired, m.CustomerReviewRequired = nil, nil
	m.DeploymentIDs, m.DeploymentProductIDs = nil, nil
	m.CustomerGroupID, m.EnvironmentIDs = nil, nil
	m.OnHold, m.OnHoldReason = nil, nil
	m.ServiceID, m.ServiceOfferingID = nil, nil
	return m
}

// githubMutationsWithStateMirror is repository.GithubMutationRepository with
// the state rule applied to SetState: an issue event that moves a change
// request (Closed, Canceled, Implement, Rollback -- the only moves the sync
// may make) is mirrored to the previous system like a staff PATCH's move
// would be (a direct write where its model allows it; Rollback is not in
// that model and is recorded as not mirrorable). SetState's `changed` is the
// prior-vs-new comparison made under the change_request row lock (IS
// DISTINCT FROM in the UPDATE), so nothing is sent for a write of the state
// the change already holds or for a refused move.
type githubMutationsWithStateMirror struct {
	repository.GithubMutationRepository
	mirror changeRequestStateMirror
}

// MirrorGithubStateChanges wraps m so every state SetState commits is mirrored
// to the previous system (DATA_SOURCE=postgres-servicenow-dual-write; wired
// in routes.go). Every other method is m's own.
func MirrorGithubStateChanges(m repository.GithubMutationRepository, mirror ChangeRequestService, dispatcher *SNWritebackDispatcher) repository.GithubMutationRepository {
	return &githubMutationsWithStateMirror{GithubMutationRepository: m, mirror: changeRequestStateMirror{snMirror: mirror, snWriteback: dispatcher}}
}

// SetState implements repository.GithubMutationRepository.
func (g *githubMutationsWithStateMirror) SetState(ctx context.Context, id, state string) (bool, error) {
	changed, err := g.GithubMutationRepository.SetState(ctx, id, state)
	if err != nil || !changed {
		return changed, err
	}
	g.mirror.dispatchState(ctx, id, strings.ToUpper(strings.TrimSpace(state)), "github sync SetState", stateMirrorWrite)
	return changed, nil
}
