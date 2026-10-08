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
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// This file owns the customer requirements lock: what may still be edited about
// WHO the customer is (the Customer Project, from which the read-only Customer
// Group is derived) and WHETHER the customer is asked (the two creation-form
// boxes, change_request.customer_approval_required / customer_review_required)
// once approval has been requested.
//
// The rule is a pure function of (the stored state, the stored value, the
// requested value, whether the change has a Customer Project) -- no marker, no
// column, no history table. "Creation phase" is the state New (or a NULL state,
// a pre-lifecycle legacy row, which counts as New): a change request can never
// return to New (PATCH {state: "new"} is refused once it has left it), so "it is
// in New" already says "approval has not been requested yet".
//
//   - In New everything is editable, in both directions: the Customer Project and
//     both boxes.
//   - The moment the change leaves New (Request Approval) the Customer Project is
//     FROZEN, for every caller, in every later state: a PATCH that changes
//     projectId is a 400, one that resends the stored value is an accepted no-op
//     (a client that sends the whole form back is not punished).
//   - The change TYPE is frozen with the project: it decides the whole approval flow, so once
//     Request Approval has chosen it a different type is a 400 and the stored one again an
//     accepted no-op (checkChangeTypeEdit). A Standard change has no approval stage to lock
//     it by, which is why the state, not the stage count, is the rule.
//   - After New a box is ADD-ONLY: false -> true is allowed until the gate the box
//     controls is passed (approvalRequirementEditable / reviewRequirementEditable),
//     and only on a change that has a Customer Project (none can be set any more);
//     true -> false is a 400 in every state. A change that reached a customer stage
//     necessarily has its box true, so add-only alone is what stops anything from
//     reopening it: a Re-schedule no longer leaves Customer Approval, and an old-flow
//     one still in flight (back in Authorize) cannot have the box unticked either, so
//     the CAB approval that follows still asks the same contacts.
//   - An EMERGENCY change takes no customer step at all, so neither box can be set on one,
//     in any state: a PATCH that turns one on, or that re-types a change that has one
//     ticked into Emergency, is a 400 (checkEmergencyCustomerConsent, change_request_emergency.go).
//     The rule judges what a request CHANGES: a write of the value a box already holds is
//     the no-op it is everywhere here, and the flow ignores the boxes of an Emergency change
//     whatever they say (effectiveCustomerGates).
//   - A new REQUESTER after Request Approval is never one more person to ask about a
//     customer gate still ahead (the requester never approves their own change): a PATCH
//     that changes requestedById is judged with the new requester in place of the stored
//     one, and refused with Request Approval's own message when nobody would be left to ask
//     (checkRequestedByLeavesSomebodyToAsk, rule 4c). The same overlay judges Request Approval
//     and a Re-schedule that carry a requestedById of their own.
//   - Request Approval itself ({state: assess}) is refused when a box is ticked and
//     there is no Customer Project, since the change would otherwise reach a
//     customer stage with nobody to ask and the project could not be set again.
//   - And it is refused when a box is ticked and the Customer Project has nobody
//     who can be asked (customerGroupCanBeAsked: no registered portal-user
//     contact other than the requester). With no staff action that answers for the
//     customer the change would otherwise reach Customer Approval / Customer Review
//     with nobody to answer, and could only be cancelled (or rolled back from
//     Review). The add-only tick of a box after Request Approval is refused for the
//     same reason. Only these two moments are judged: a change already beyond New
//     is never re-judged, so a contact who is deactivated AFTER Request Approval
//     (the residual edge) still leaves a change waiting for a contact to register,
//     and a legacy row seeded in a customer state keeps its Cancel / Roll back /
//     Re-schedule exits.
//   - Corrections after New are a cancel and a clone (Clone exists); there is no
//     administrator override.
//
// These are API-caller rules. csm-sync-service writes its tables directly (as an
// internal caller, never through PatchChangeRequest) and is not bound by them; see
// entity-service/CLAUDE.md, "Customer requirements lock".

// changeRequestCreationPhase reports whether the (upper-case) stored state is the
// creation phase: New, or NULL ("" -- a pre-lifecycle legacy row counts as New,
// like everywhere else).
func changeRequestCreationPhase(state string) bool {
	return state == "" || state == crStateNew
}

// lockStateName is the state as these messages name it: lower case, "new" for a
// NULL state.
func lockStateName(state string) string {
	if state == "" {
		return "new"
	}
	return strings.ToLower(state)
}

// customerRequirementBox describes one of the two boxes for the lock rules.
type customerRequirementBox struct {
	// field is the API field name, as the messages name it.
	field string
	// editable says whether the gate the box controls is still ahead in the
	// (upper-case) state.
	editable func(state string) bool
	// gatePassed is the 400 for an add after the gate.
	gatePassed func(state string) string
}

var (
	customerApprovalBox = customerRequirementBox{
		field:    "customerApprovalRequired",
		editable: approvalRequirementEditable,
		gatePassed: func(state string) string {
			return fmt.Sprintf("customerApprovalRequired can no longer be changed: the change request has already passed the approval stage (current state: %s)", lockStateName(state))
		},
	}
	customerReviewBox = customerRequirementBox{
		field:    "customerReviewRequired",
		editable: reviewRequirementEditable,
		gatePassed: func(state string) string {
			return fmt.Sprintf("customerReviewRequired can no longer be changed: the change request has already left the review stage (current state: %s)", lockStateName(state))
		},
	}
)

// Messages of the lock. They are the API contract (openapi.yaml, the CSM Edit
// dialog's own wording mirrors them), so they are spelled out in one place.
const (
	changeRequestCannotReturnToNewMsg = `state "new" cannot be set: a change request that has left New cannot return to it. Cancel it and clone it instead.`
	changeRequestApprovalNeedsProject = "approval cannot be requested: the customer's approval and/or review is required but no Customer Project is set, so there is nobody to ask. Select a Customer Project first (or clear the requirement)."
)

// nobodyToAskMsg is the 400 for a customer box that is ticked (at Request Approval,
// or turned on after it) on a Customer Project that has nobody who can be asked.
// It names the box (or both) and says what to do. approval / review say which
// boxes the refusal is about; at least one is true.
func nobodyToAskMsg(approval, review bool) string {
	what := "customer approval is"
	switch {
	case approval && review:
		what = "customer approval and customer review are"
	case review:
		what = "customer review is"
	}
	return what + " required but nobody on this project can be asked (no registered contact other than the requester): register a contact for the project first"
}

func customerProjectFrozenMsg(state string) string {
	return fmt.Sprintf("projectId can no longer be changed: the Customer Project is fixed once approval has been requested (current state: %s). Cancel this change request and clone it to use another project.", lockStateName(state))
}

func (b customerRequirementBox) cannotTurnOffMsg(state string) string {
	return fmt.Sprintf("%s can no longer be turned off: once approval has been requested a customer requirement can be added but never removed (current state: %s). Cancel and clone to correct it.", b.field, lockStateName(state))
}

func (b customerRequirementBox) needsProjectMsg() string {
	return fmt.Sprintf("%s cannot be turned on: this change request has no Customer Project, and one can no longer be set after approval was requested. Cancel and clone it with a project.", b.field)
}

// checkReturnToNew is rule 1: {state: "new"} on a change request that has left
// New. (A resend on a change that is still in New is a no-op and is accepted.)
func checkReturnToNew(state string, requested *domain.ChangeRequestState) error {
	if requested == nil || !strings.EqualFold(string(*requested), string(domain.ChangeRequestStateNew)) {
		return nil
	}
	if changeRequestCreationPhase(state) {
		return nil
	}
	// A final change request is refused as every other request to move it is
	// (changeRequestFinalRefusal), which says more than "cannot return to New".
	if terminalChangeRequestState(state) {
		return changeRequestFinalRefusal(state, *requested)
	}
	return &apierror.ValidationError{Msg: changeRequestCannotReturnToNewMsg}
}

// checkCustomerProjectEdit is rule 2: the Customer Project is editable only in
// the creation phase. storedProject nil means none is stored (NULL -> X counts as
// a change); requested nil means the request does not carry projectId. Writing the
// value already stored is an accepted no-op.
func checkCustomerProjectEdit(state string, storedProject *string, requested *string) error {
	if requested == nil || changeRequestCreationPhase(state) {
		return nil
	}
	if storedProject != nil && strings.EqualFold(strings.TrimSpace(*storedProject), strings.TrimSpace(*requested)) {
		return nil
	}
	return &apierror.ValidationError{Msg: customerProjectFrozenMsg(state)}
}

// checkRequirementEdit is rules 3 and 4 for one box: stored and requested are
// the stored value and the one in the request (nil = not in the request).
//
//   - an unchanged value is always accepted;
//   - in the creation phase any change is accepted (Request Approval is what
//     checks it makes sense, see checkRequestApprovalHasProject);
//   - after New true -> false is refused whatever the state;
//   - after New false -> true is refused once the box's gate is passed, with the
//     box's existing message, and then refused when the change has no Customer
//     Project (none can be set any more); whether the project has anybody to ask
//     is a database question, so checkTickedBoxCanBeAsked judges it next.
func checkRequirementEdit(box customerRequirementBox, state string, stored bool, requested *bool, hasProject bool) error {
	if requested == nil || *requested == stored || changeRequestCreationPhase(state) {
		return nil
	}
	if stored {
		return &apierror.ValidationError{Msg: box.cannotTurnOffMsg(state)}
	}
	if !box.editable(state) {
		return &apierror.ValidationError{Msg: box.gatePassed(state)}
	}
	if !hasProject {
		return &apierror.ValidationError{Msg: box.needsProjectMsg()}
	}
	return nil
}

// checkRequestApprovalHasProject is rule 6: Request Approval ({state: assess}) on
// a change still in the creation phase is refused when a box is set (the request's
// value, else the stored one) and there is no Customer Project (the request's, else
// the stored one). A resend on a change that has already left New is not Request
// Approval and is never refused here.
func checkRequestApprovalHasProject(state string, approvalRequired, reviewRequired, hasProject bool) error {
	if !changeRequestCreationPhase(state) || hasProject || !(approvalRequired || reviewRequired) {
		return nil
	}
	return &apierror.ValidationError{Msg: changeRequestApprovalNeedsProject}
}

// requireSomebodyToAsk is the refusal shared by Request Approval and the add-only
// tick of a box: the boxes named (approval / review) are required, so a customer
// stage will be provisioned for the Customer Project, and the project must have
// somebody who can be asked -- the very test provisionCustomerStage applies
// (customerGroupCanBeAsked). A change with no project is not this rule's case
// (checkRequestApprovalHasProject, needsProjectMsg own it), nor is one that needs
// no customer step.
func requireSomebodyToAsk(ctx context.Context, q crQuerier, workItemID string, project *string, approval, review bool) error {
	return requireSomebodyToAskWith(ctx, q, workItemID, project, approval, review, nil)
}

// requireSomebodyToAskWith is requireSomebodyToAsk judging the change's creators with the
// request's own requestedById in place of the stored one (nil: the stored one): a PATCH
// that sets the requester AND needs somebody to ask is judged as it will stand once
// written (rule 4c, Request Approval, a Re-schedule).
func requireSomebodyToAskWith(ctx context.Context, q crQuerier, workItemID string, project *string, approval, review bool, requestedBy **string) error {
	if !(approval || review) || !hasProjectID(project) {
		return nil
	}
	ok, err := customerGroupCanBeAskedWith(ctx, q, workItemID, strings.ToLower(strings.TrimSpace(*project)), requestedBy)
	if err != nil {
		return fmt.Errorf("patch change request: check who can be asked: %w", err)
	}
	if ok {
		return nil
	}
	return &apierror.ValidationError{Msg: nobodyToAskMsg(approval, review)}
}

// checkRequestApprovalCanAsk is rule 7: Request Approval ({state: assess}) on a
// change still in the creation phase is refused when a box is set (the request's
// value, else the stored one) and the Customer Project in effect (the request's,
// else the stored one) has nobody who can be asked. It runs after
// checkRequestApprovalHasProject, which keeps its own message and precedence for a
// change with no project. A resend on a change that has already left New is not
// Request Approval and is never judged here.
func checkRequestApprovalCanAsk(ctx context.Context, q crQuerier, workItemID, state string, approvalRequired, reviewRequired bool, project *string, requestedBy **string) error {
	if !changeRequestCreationPhase(state) {
		return nil
	}
	return requireSomebodyToAskWith(ctx, q, workItemID, project, approvalRequired, reviewRequired, requestedBy)
}

// boxesTurnedOnAfterNew says which boxes a PATCH turns ON (false -> true) on a
// change that has left New: the add-only edit rule 4 lets through. Unticked boxes,
// boxes already ticked, boxes the request does not carry, and everything in the
// creation phase are not turned on.
func boxesTurnedOnAfterNew(snap changeRequestGateSnapshot, approvalRequired, reviewRequired *bool) (approval, review bool) {
	if changeRequestCreationPhase(snap.state) {
		return false, false
	}
	approval = approvalRequired != nil && *approvalRequired && !snap.approvalRequired
	review = reviewRequired != nil && *reviewRequired && !snap.reviewRequired
	return approval, review
}

// checkTickedBoxCanBeAsked is rule 4b: a box turned on after Request Approval
// (which rule 4 let through: its gate is still ahead and the change has a
// Customer Project) is refused when the project has nobody who can be asked, with
// the message of Request Approval's refusal naming the box(es) turned on -- the
// change would otherwise reach a gate nobody can answer. Only the turning-on is
// judged: a request that leaves the boxes as they are is never re-judged, whatever
// the project's contacts have become since.
func checkTickedBoxCanBeAsked(ctx context.Context, q crQuerier, workItemID string, snap changeRequestGateSnapshot, approvalRequired, reviewRequired *bool) error {
	approval, review := boxesTurnedOnAfterNew(snap, approvalRequired, reviewRequired)
	return requireSomebodyToAsk(ctx, q, workItemID, snap.projectID, approval, review)
}

// hasProjectID reports whether a project id value is present.
func hasProjectID(project *string) bool {
	return project != nil && strings.TrimSpace(*project) != ""
}

// changeRequestPatchNeedsGate reports whether the request carries anything the
// creation-phase gate judges: the state, the project, the type, the requester, either
// box, or any deployment field.
func changeRequestPatchNeedsGate(req domain.PatchChangeRequestRequest) bool {
	return req.State != nil || req.ProjectID != nil || req.Type != nil || req.RequestedByID != nil ||
		req.CustomerApprovalRequired != nil || req.CustomerReviewRequired != nil ||
		req.DeploymentIDs != nil || req.DeploymentProductIDs != nil ||
		req.DeploymentID != nil || req.DeployedProductID != nil
}

// validateCreationPhaseEdits applies rules 1 to 4 in order (the first failing one
// wins, every refusal a 400) to a PATCH against the snapshot read under the row
// lock. Rule 5 (deployments follow the frozen project) is
// planChangeRequestLinks' and checkSingularDeploymentFields'; rule 6 is the
// assess case of patchChangeRequestTx's.
func validateCreationPhaseEdits(snap changeRequestGateSnapshot, req domain.PatchChangeRequestRequest) error {
	if err := checkReturnToNew(snap.state, req.State); err != nil {
		return err
	}
	if err := checkCustomerProjectEdit(snap.state, snap.projectID, req.ProjectID); err != nil {
		return err
	}
	if err := checkChangeTypeEdit(snap, req.Type); err != nil {
		return err
	}
	// Rule 2c: an Emergency change takes no customer step, so neither box can be set on
	// one. Before the box rules below, so a tick on an Emergency change is refused for
	// that reason and not for one the lock gives about a gate it never has.
	if err := checkEmergencyCustomerConsent(snap, req); err != nil {
		return err
	}
	return validateCustomerGateEdits(snap, req.CustomerApprovalRequired, req.CustomerReviewRequired)
}

// customerGatesAhead says which customer gates are still ahead of -- or are being
// waited on by -- a change in the given (upper-case) state with the boxes as stated: the
// approval gate until the customer's approval is given (New, Assess, Authorize, Customer
// Approval), the review gate until the customer's review is (every state up to and
// including Customer Review; a final state has none).
func customerGatesAhead(state string, approvalRequired, reviewRequired bool) (approval, review bool) {
	if terminalChangeRequestState(state) {
		return false, false
	}
	approval = approvalRequired && (approvalRequirementEditable(state) || state == crStateCustomerApproval)
	review = reviewRequired && (reviewRequirementEditable(state) || state == crStateCustomerReview)
	return approval, review
}

// checkRequestedByLeavesSomebodyToAsk is rule 4c: a PATCH that changes the REQUESTER
// of a change that has left New is refused when, with the new requester, a customer gate
// still ahead (customerGatesAhead) would have nobody to ask -- the requester never
// approves their own change, so naming the project's only registered contact as the
// requester would strand the change at its customer gate. The refusal is the one Request
// Approval gives (nobodyToAskMsg, naming the gate(s)). Only a CHANGE of the requester is
// judged: a request that leaves it as it is (a whole-form resend included) is never
// re-judged, whatever the project's contacts have become since, and the creation phase is
// left to Request Approval, which judges the requester then in force
// (checkRequestApprovalCanAsk). The boxes in effect are the request's, else the stored
// ones.
func checkRequestedByLeavesSomebodyToAsk(ctx context.Context, q crQuerier, workItemID string, snap changeRequestGateSnapshot, req domain.PatchChangeRequestRequest) error {
	if req.RequestedByID == nil || changeRequestCreationPhase(snap.state) {
		return nil
	}
	var stored *string
	if err := q.QueryRow(ctx, `SELECT requested_by_user_id::text FROM change_request WHERE id = $1`, workItemID).Scan(&stored); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("patch change request: read the stored requester: %w", err)
	}
	requested := *req.RequestedByID
	if (stored == nil && requested == nil) || (stored != nil && requested != nil && strings.EqualFold(strings.TrimSpace(*stored), strings.TrimSpace(*requested))) {
		return nil
	}
	approvalRequired, reviewRequired := snap.approvalRequired, snap.reviewRequired
	if req.CustomerApprovalRequired != nil {
		approvalRequired = *req.CustomerApprovalRequired
	}
	if req.CustomerReviewRequired != nil {
		reviewRequired = *req.CustomerReviewRequired
	}
	// An Emergency change has no customer gate ahead whatever its boxes say.
	approvalRequired, reviewRequired = effectiveCustomerGates(effectiveChangeModel(snap.model, req.Type), approvalRequired, reviewRequired)
	approval, review := customerGatesAhead(snap.state, approvalRequired, reviewRequired)
	return requireSomebodyToAskWith(ctx, q, workItemID, snap.projectID, approval, review, req.RequestedByID)
}

// changeTypeFrozenMsg is the 400 for a change of the change TYPE after Request
// Approval (rule 2b).
func changeTypeFrozenMsg(state string) string {
	return fmt.Sprintf("type can no longer be changed: the change type decides the approval flow, which is fixed once approval has been requested (current state: %s). Cancel this change request and clone it to use another type.", lockStateName(state))
}

// checkChangeTypeEdit is rule 2b: the change TYPE is editable only in the creation
// phase, like the Customer Project. The type decides the whole approval flow (Standard:
// none; Normal: peer then CAB; Emergency: CAB only), so once the change has left New --
// Request Approval has chosen its flow -- a type other than the stored one is a 400; the
// stored type again (a client that resends the whole form) is an accepted no-op. Until
// now only an approval-stage COUNT locked the type (patchChangeRequestTx still checks it,
// as a second line), which left a Standard change -- it has no stage at all -- free to be
// re-typed after Request Approval. A type with no change_model label is not the stored one
// either.
func checkChangeTypeEdit(snap changeRequestGateSnapshot, requested *domain.ChangeRequestType) error {
	if requested == nil || changeRequestCreationPhase(snap.state) {
		return nil
	}
	if resendsStoredChangeType(snap.model, requested) {
		return nil
	}
	return &apierror.ValidationError{Msg: changeTypeFrozenMsg(snap.state)}
}

// resendsStoredChangeType reports whether a requested type IS the stored one (storedModel is
// change_model's upper-case label, "" when NULL): a client that sends the whole form back sends the
// type it already has, which changes nothing. A type with no change_model label, or a change with no
// stored model, is not the stored one.
func resendsStoredChangeType(storedModel string, requested *domain.ChangeRequestType) bool {
	if requested == nil || storedModel == "" {
		return false
	}
	model, ok := changeRequestTypeToChangeModel[*requested]
	return ok && strings.EqualFold(model, storedModel)
}

// lockChangeRequestForPatch is the first thing a PATCH that carries a state, a
// project, a box or a deployment field does: it locks the work_item row (FOR NO KEY
// UPDATE, see below), and only
// THEN reads the change_request side (state, model, both boxes, project) in a new
// statement. Under READ COMMITTED a read that shares a statement with the lock can
// be answered from a snapshot older than the lock's grant: Request Approval racing
// a project edit would then judge the edit against a stored state of New that was
// already left. The order (work_item, then change_request) is the one every other
// PATCH takes.
func lockChangeRequestForPatch(ctx context.Context, tx pgx.Tx, id string) (changeRequestGateSnapshot, error) {
	var locked string
	var updatedBy *string
	// FOR NO KEY UPDATE, the strength the PATCH's own UPDATE of work_item takes, not
	// FOR UPDATE: a decision (DecideChangeRequestApproval) locks change_request first
	// and then INSERTs approval_stage / approval_stage_approver rows, whose foreign
	// keys take FOR KEY SHARE on this very work_item row, which FOR UPDATE would
	// refuse -- a deadlock with a PATCH that holds this lock and waits for the
	// change_request one. Two PATCHes still exclude each other (NO KEY UPDATE
	// conflicts with itself), which is all this lock is for.
	//
	// updated_by comes back with the lock: it is the last writer BEFORE this PATCH's own
	// UPDATE of the row (which always stamps the caller). A staff answer to a time the
	// customer proposed has to know who proposed it, and the only place that is still
	// intact is here (changeRequestGateSnapshot.priorWriter).
	err := tx.QueryRow(ctx,
		`SELECT id::text, updated_by FROM work_item WHERE id = $1::uuid AND type = 'CHANGE_REQUEST' FOR NO KEY UPDATE`, id).Scan(&locked, &updatedBy)
	if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
		return changeRequestGateSnapshot{}, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return changeRequestGateSnapshot{}, fmt.Errorf("patch change request: lock work item: %w", err)
	}
	snap, err := lockChangeRequestGateSnapshot(ctx, tx, id)
	if err != nil {
		return changeRequestGateSnapshot{}, err
	}
	snap.priorWriter = newLastWriter(updatedBy)
	return snap, nil
}

// checkSingularDeploymentFields is rule 5 for the single-valued PATCH fields
// deploymentId / deployedProductId, which skip resolveChangeRequestLinks: a value
// other than the stored one is subject to the same edit window as the lists
// (changeRequestLinksLockedStates) and must belong to the project the change will
// have, which after New is the frozen stored one. effectiveProject is the request's
// projectId else the stored one; a change request with no project cannot take a
// deployment, as on create. A value that does not exist is left to the foreign key,
// which answers it as the field "does not refer to an existing record".
func checkSingularDeploymentFields(ctx context.Context, tx pgx.Tx, id string, req domain.PatchChangeRequestRequest, snap changeRequestGateSnapshot, effectiveProject *string) error {
	if req.DeploymentID == nil && req.DeployedProductID == nil {
		return nil
	}
	var storedDeployment, storedProduct *string
	if err := tx.QueryRow(ctx,
		`SELECT deployment_id::text, deployed_product_id::text FROM work_item WHERE id = $1::uuid`, id).Scan(&storedDeployment, &storedProduct); err != nil {
		return fmt.Errorf("patch change request: read stored deployment fields: %w", err)
	}
	same := func(stored, requested *string) bool {
		return stored != nil && strings.EqualFold(strings.TrimSpace(*stored), strings.TrimSpace(*requested))
	}
	check := func(field string, requested, stored *string, projectOf string) error {
		if requested == nil || same(stored, requested) {
			return nil
		}
		if snap.state != "" && changeRequestLinksLockedStates[snap.state] {
			return linkValidationf("%s can no longer be changed: the change request is in state %q (project, deployments and deployment products are editable only before implementation starts)",
				field, strings.ToLower(snap.state))
		}
		if !hasProjectID(effectiveProject) {
			return linkValidationf("%s requires a Customer Project: a deployment must belong to the change request's project", field)
		}
		var project *string
		err := tx.QueryRow(ctx, projectOf, strings.TrimSpace(*requested)).Scan(&project)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // unknown id: the foreign key answers it
		}
		if err != nil {
			return fmt.Errorf("patch change request: check %s against the project: %w", field, err)
		}
		if project == nil || !strings.EqualFold(*project, strings.TrimSpace(*effectiveProject)) {
			return linkValidationf("%s does not belong to the change request's project: %s", field, strings.TrimSpace(*requested))
		}
		return nil
	}
	if err := check("deploymentId", req.DeploymentID, storedDeployment,
		`SELECT d.project_id::text FROM deployment d WHERE d.id = $1::uuid`); err != nil {
		return err
	}
	return check("deployedProductId", req.DeployedProductID, storedProduct,
		`SELECT d.project_id::text FROM deployed_product dp JOIN deployment d ON d.id = dp.deployment_id WHERE dp.id = $1::uuid`)
}
