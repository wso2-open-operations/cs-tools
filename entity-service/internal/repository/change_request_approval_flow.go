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
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// This file owns the type-dependent change request approval flow of the
// PostgreSQL data source. The three change types behave as follows
// (ServiceNow's own semantics: Normal "requires one or more approvals",
// Standard "does not require approval", Emergency "must be implemented as
// soon as possible").
//
// The previous system has no Emergency CAB. An Emergency change is approved by the same
// "CAB Approval" group a Normal change's second stage is, and an Emergency change
// migrated from the previous system has exactly that one stage (no Peer stage). An earlier
// build wrote a stage labelled "ECAB Approval" for Emergency changes; that label
// is no longer written, but a stage that still carries it is shown under it and is
// decided as the CAB stage (approvalStageLabelHistoricECAB):
//
//	Normal:    New --(Request Approval)--> Assess [Peer Approval stage]
//	               --> Authorize [CAB Approval stage]
//	               --> Scheduled (automatically, on CAB approval)
//	               --> Implement --> Review --> Closed
//	Emergency: New --(Request Approval)--> Authorize [ONE stage, in the existing
//	               "CAB Approval" group: no Peer stage, no Assess stage]
//	               --> Scheduled (automatically, on that CAB approval)
//	               --> Implement --> Review --> Closed
//	Standard:  New --(Request Approval)--> Scheduled (no approval stages at
//	               all) --> Implement --> Review --> Closed
//
// "Request Approval" is the one human action out of New and is always sent as
// {state: "assess"} -- the contract the webapp already has (legalNextStates of
// a New change request is ["assess", "canceled"] for every type). The
// resulting state is chosen here from the change's type, never by the caller.
// Scheduled is never a manual transition: it is reached only by the CAB
// approval cascade in DecideChangeRequestApproval, or by Request Approval on
// a Standard change, which has nothing to wait for.
//
// A change request whose type is NULL or not one of the three (a legacy or
// ServiceNow-synced row: azure, infra, ...) follows the Normal flow.
//
// The creation form's two checkboxes add an optional customer step on each
// side of the implementation (change_request.customer_approval_required /
// customer_review_required, migration 0189):
//
//	approval gate: wherever the flow above would move the change to Scheduled
//	    (CAB approval, or Request Approval on a Standard change -- an
//	    assumption, Standard has no internal approvals to put the gate after) it
//	    moves it to Customer Approval instead when customer_approval_required.
//	    Only the CUSTOMER's own approval, given in the Customer Portal, then
//	    schedules the change (and stamps is_customer_approval_required).
//	review gate: Review -> Customer Review -> Closed when
//	    customer_review_required, Review -> Closed otherwise. Only the
//	    CUSTOMER's own review, given in the Customer Portal, closes the change
//	    from Customer Review (and stamps is_customer_review_required).
//
// COMPLIANCE RULE: no staff action records the customer's approval or review on
// the customer's behalf. The customer's answer is the customer's decision and
// ServiceNow's record of it is audited, so a decision made for the customer and
// stored as theirs would be a compliance problem. A change in a customer state
// moves on only through the customer's own answer; staff keep Cancel (any
// state), Re-schedule (out of Customer Approval: the customer is asked again)
// and Rollback (out of Customer Review, while nobody is being asked). See
// refuseStaffExitFromCustomerState, refuseStaffCustomerOutcomeFlags and
// changeRequestForwardNextStates.
//
// EMERGENCY changes are acted on without the customer's consent: they never ENTER
// Customer Approval or Customer Review. The two boxes cannot be set on one (a create
// or a PATCH that would is a 400: checkEmergencyCustomerConsent,
// ValidateCreateChangeRequestCustomerGates), and the flow ignores whatever the stored
// boxes say for a change whose type is Emergency (effectiveCustomerGates), so a row
// that already carries one -- an Emergency change from before the rule, or a
// migrated one -- still goes CAB approval -> Scheduled and Review ->
// Closed. Reads of the stored values are untouched. An Emergency change that is
// ALREADY waiting in a customer state (the same two kinds of row) is not taken out of
// the customer's hands: the customer stage logic below does not look at the type, so
// its contacts are asked, can answer, and are asked again by a Re-schedule exactly as
// on any other change.
//
// Who gives the customer's answer depends on the change's Customer Project. The
// Customer Group is not stored or picked: it is the project's registered
// PORTAL_USER contacts, derived live by customerContactUserIDs(projectID).
// change_request.customer_group_id is legacy and ignored when provisioning:
//
//	with at least one eligible contact (an active registered contact who is not
//	    the creator): entering Customer Approval / Customer Review provisions
//	    an approval stage -- "Customer Approval" / "Customer Review", with NO
//	    assignment group (the approvals response shows "Customer Group") -- and
//	    one REQUESTED approver per eligible contact. The contacts decide it in
//	    the Approvals tab, through DecideChangeRequestApproval like every other
//	    stage (first responder wins). Approving Customer Approval schedules the
//	    change (is_customer_approval_required stamped), rejecting it cancels it;
//	    approving Customer Review closes it (is_customer_review_required
//	    stamped), rejecting it moves it to Rollback.
//	without one (no project, or no eligible registered contact on it): no stage
//	    is provisioned and nobody is asked. Staff do NOT answer for the
//	    customer: the change can be cancelled or re-scheduled, or it waits until
//	    a contact is registered and the project is restated (provisionCustomerStage
//	    then asks them). A legacy change that reached the state with nobody asked
//	    gets its stage when a contact first acts (ensureCustomerStageForLegacy).
//
// That dead end is not produced any more by Request Approval: it is REFUSED when
// a customer box is ticked and the project has nobody who can be asked, and so is
// turning a box on after it (checkRequestApprovalCanAsk, checkTickedBoxCanBeAsked in
// change_request_customer_lock.go), by asking the very test the stage provisioning
// asks (customerGroupCanBeAsked / anyContactToAsk). What is left are the rows that
// predate the refusal (legacy rows, seeded in a customer state) and the residual
// edge: every registered contact deactivated AFTER Request Approval, which is not
// built for -- a change that is beyond New is never re-judged.
//
// provisionCustomerStage keeps the stage in step with the change (state and
// project contacts) and is the one place that provisions, replaces or cancels it.

// A stage can only be decided while the change request is in the state it
// belongs to (approvalStageDecidableState): Peer in Assess, CAB in Authorize,
// Review in Review, Customer Approval in Customer Approval, Customer Review in
// Customer Review. Three things keep an approver row from outliving
// that state, and a fourth repairs the ones that already did:
//
//   - reconcileStaleApprovers runs at the end of every transaction that can
//     change change_request.state (patchChangeRequestTx, DecideChangeRequestApproval,
//     the GitHub sync's SetState) and cancels every still-requested approver row of a stage whose state the
//     change is no longer in -- and ALL of them once it is Closed / Canceled /
//     Rollback. Review is the case that matters most: deciding it changes no
//     state (the change is moved on by hand), so without this its other
//     approvers stayed actionable for ever, also while the change waited at
//     Customer Review for the customer;
//   - DecideChangeRequestApproval refuses a decision on such a row (409,
//     staleApprovalRefusal), which also covers rows that predate the reconcile;
//   - canDecide is false for such a row (markCanDecide);
//   - migration 0193 cancels the rows already in the database.
//
// A stage of unknown kind and a change with no / unknown state are never guarded.
// A stage with no checkpoint_label (a ServiceNow-synced one) is of unknown kind
// unless its assignment group, the change's type and state, or its position
// within the state the change is in say what it is (runtimeApprovalStageKind):
// its position alone is a guess, and a guess never cancels, hides or refuses an
// approval.

// Approver pools are INTERNAL-only. Every internal stage (Peer, CAB, Review) is
// decided in the portal by WSO2 staff, who see every project; an external
// (customer) user sees only the projects they are a registered contact of, so
// an approver row for one could never be found, let alone decided. A pool is
// therefore filtered to users who are active
// ("user".is_active, NULL counting as active) and "user".user_type = 'INTERNAL'
// -- the type recompute_user_type() derives from the internal/admin roles --
// when it is resolved (internalApproverIDs), and the same test is applied
// again at decision time (approverDecisionBlock, which also drives canDecide).
// The customer stages are the exception on purpose: their approvers are the
// project's registered customer contacts, external by nature.

// Stage labels written to approval_stage.checkpoint_label by this file's
// provisioning. LegacyAssessLabel/LegacyAuthorizeLabel are what stages created
// before the CAB flow were written with; they are still recognised when a
// stage is classified (see classifyApprovalStage).
const (
	approvalStageLabelPeer   = "Peer Approval"
	approvalStageLabelCAB    = "CAB Approval"
	approvalStageLabelReview = "Review"
	// approvalStageLabelHistoricECAB is the label an earlier build wrote on an
	// Emergency change's only stage ("ECAB Approval", in a group of its own).
	// The previous system has no Emergency CAB, so nothing writes it any more: it is only
	// RECOGNISED, so an in-flight Emergency change that already holds such a stage
	// keeps showing it under that name and its approvers (the REQUESTED rows that
	// were written for it) can still decide it, as the CAB stage -- classifyApprovalStage.
	approvalStageLabelHistoricECAB = "ECAB Approval"
	// The customer's own stages (see provisionCustomerStage): not part of the
	// internal checkpoint ordinals, so they are written and recognised by label.
	approvalStageLabelCustomerApproval = "Customer Approval"
	approvalStageLabelCustomerReview   = "Customer Review"
	approvalStageLabelLegacyAss        = "Assess"
	approvalStageLabelLegacyAut        = "Authorize"
)

// approvalPoolKind selects where a checkpoint's approvers come from.
type approvalPoolKind int

const (
	// poolAssignedGroup: members of the change request's own assigned group
	// (work_item.assignment_group_id). Used by the Review checkpoint.
	poolAssignedGroup approvalPoolKind = iota
	// poolPeer: the peer approval pool -- see resolvePeerPool.
	poolPeer
	// poolNamedGroup: members of the group named checkpoint.GroupName (the
	// CAB group).
	poolNamedGroup
)

// changeRequestFlow is what Request Approval does for one change type.
type changeRequestFlow struct {
	// requestState is the state Request Approval moves a New change to.
	requestState domain.ChangeRequestState
	// checkpoint is the approval stage provisioned on entry, nil when the
	// type has no approval at all (Standard).
	checkpoint *changeRequestApprovalCheckpoint
}

// changeRequestFlowForModel resolves the flow from change_request.change_model
// (the upper-case enum label; "" or unknown follows the Normal flow).
func changeRequestFlowForModel(model string) changeRequestFlow {
	switch strings.ToUpper(model) {
	case "EMERGENCY":
		return changeRequestFlow{requestState: domain.ChangeRequestStateAuthorize, checkpoint: &changeRequestEmergencyCABCheckpoint}
	case "STANDARD":
		return changeRequestFlow{requestState: domain.ChangeRequestStateScheduled}
	default:
		return changeRequestFlow{requestState: domain.ChangeRequestStateAssess, checkpoint: &changeRequestPeerCheckpoint}
	}
}

// crQuerier is the sliver of *Scoped and pgx.Tx the approval-flow helpers
// share, so they read the same inside a transaction and standalone.
type crQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// approvalStageKind is the role of an approval stage in the flow.
type approvalStageKind int

const (
	stageKindOther approvalStageKind = iota
	stageKindPeer
	stageKindCAB
	stageKindReview
	// stageKindCustomerApproval / stageKindCustomerReview are the customer
	// group's stages, entered with the Customer Approval / Customer Review
	// states.
	stageKindCustomerApproval
	stageKindCustomerReview
)

// classifyApprovalStage resolves a stage's role from its explicit
// checkpoint_label, falling back to its zero-based ordinal position for a
// stage with no label (a ServiceNow-synced stage, or one written before
// migration 0179) -- the historical Assess (0) / Authorize (1) convention.
func classifyApprovalStage(label *string, position int) approvalStageKind {
	if label != nil && *label != "" {
		switch *label {
		case approvalStageLabelPeer, approvalStageLabelLegacyAss:
			return stageKindPeer
		case approvalStageLabelCAB, approvalStageLabelLegacyAut, approvalStageLabelHistoricECAB:
			return stageKindCAB
		case approvalStageLabelReview:
			return stageKindReview
		case approvalStageLabelCustomerApproval:
			return stageKindCustomerApproval
		case approvalStageLabelCustomerReview:
			return stageKindCustomerReview
		default:
			return stageKindOther
		}
	}
	switch position {
	case 0:
		return stageKindPeer
	case 1:
		return stageKindCAB
	default:
		return stageKindOther
	}
}

// runtimeApprovalStageKind is the role the flow treats a stage as when it decides,
// guards or lists what can be decided on a change request of the given
// (upper-case) model in the given (upper-case) state. A stage with an explicit
// checkpoint_label is classified by it (classifyApprovalStage). A stage with NONE
// -- one csm-sync-service mirrored from ServiceNow, so migrated data -- is
// classified only as far as the data it does carry makes provable, and never as a
// customer stage:
//
//  1. the stage's own assignment group names it: the "CAB Approval" group makes
//     it a CAB stage -- whatever the change's type and whatever its position, which
//     is how a migrated Emergency change's one stage (no Peer stage, in
//     the CAB group, at position 0) reads;
//  2. failing that, an Emergency change in Authorize has no peer stage, so a stage
//     on it can only be the CAB's;
//  3. failing that, the historical positional guess (0 = Peer, 1 = CAB; see
//     classifyApprovalStage);
//
// and whichever of the three produced a kind, the kind COUNTS only when the state
// it is decided in is the state the change is in (approvalStageDecidableState).
// Otherwise the stage is stageKindOther: not tied to any state, so it is never
// cancelled by reconcileStaleApprovers (a finished change's own terminal cancel
// excepted), never refused as out of state, and decidable by its REQUESTED
// approver (the creator rule still applies). That is the safe reading of a stage
// whose position is only a guess: an Emergency change in Authorize whose single
// synced stage sits at position 0 used to read as a PEER stage, so its approver's
// decision was refused as stale (409) and canDecide was false; a stage at
// position 2 can be anything; a stale position-0 stage on a change that has moved
// on to Authorize is not a Peer approval that has gone out of date, it is a row
// nobody can say anything about. Customer kinds are never inferred for an
// unlabeled stage: it fails closed (the customer stages are written by this
// service with their label, and the customer's answer is only ever accepted on
// one of those).
func runtimeApprovalStageKind(label *string, position int, groupName *string, model, state string) approvalStageKind {
	if label != nil && *label != "" {
		return classifyApprovalStage(label, position)
	}
	candidate := stageKindOther
	switch strings.TrimSpace(stringOrEmpty(groupName)) {
	case domain.CABApprovalGroupName:
		candidate = stageKindCAB
	default:
		if isEmergencyModel(model) && strings.EqualFold(strings.TrimSpace(state), crStateAuthorize) {
			candidate = stageKindCAB
		} else {
			candidate = classifyApprovalStage(nil, position)
		}
	}
	want := approvalStageDecidableState(candidate)
	if want == "" || !strings.EqualFold(strings.TrimSpace(state), want) {
		return stageKindOther
	}
	return candidate
}

// approvalStageInfo reads stageID's label, assignment group and ordinal position
// among the work item's stages (ordered by created_on, id -- the same ordering
// changeRequestApprovalStagesQuery uses), and the model and state of the change
// request, and resolves the stage's kind with runtimeApprovalStageKind.
func approvalStageInfo(ctx context.Context, q crQuerier, workItemID, stageID string) (approvalStageKind, error) {
	var label, groupName, model, state *string
	var pos int
	err := q.QueryRow(ctx, `
		SELECT ast.checkpoint_label, g.name,
		       (SELECT COUNT(*) FROM approval_stage earlier
		         WHERE earlier.work_item_id = $1
		           AND (earlier.created_on, earlier.id) < (ast.created_on, ast.id)),
		       cr.change_model::text, cr.state::text
		FROM approval_stage ast
		LEFT JOIN "group" g ON g.id = ast.assignment_group_id
		LEFT JOIN change_request cr ON cr.id = ast.work_item_id
		WHERE ast.id = $2`, workItemID, stageID).Scan(&label, &groupName, &pos, &model, &state)
	if err != nil {
		return stageKindOther, fmt.Errorf("read approval stage: %w", err)
	}
	return runtimeApprovalStageKind(label, pos, groupName, stringOrEmpty(model), stringOrEmpty(state)), nil
}

// changeRequestCreatorUserIDs returns the (lower-cased) ids of every user who
// counts as the creator of the change request and therefore may never approve
// it at any stage: the user whose email is work_item.created_by, and
// change_request.requested_by_user_id (the existing self-approval-exclusion
// identity). work_item.opened_by_user_id is deliberately not part of this:
// it is a ServiceNow "opened by" passthrough that can name someone other than
// who raised the change here. An unreadable/missing change request yields an
// empty set, not an error.
func changeRequestCreatorUserIDs(ctx context.Context, q crQuerier, workItemID string) (map[string]bool, error) {
	return changeRequestCreatorUserIDsWith(ctx, q, workItemID, nil)
}

// changeRequestCreatorUserIDsWith is changeRequestCreatorUserIDs judged AS IF a
// PATCH's own requestedById had already been written: a non-nil requestedBy (the
// request's field, a pointer to pointer exactly like domain.PatchChangeRequestRequest's)
// replaces the stored requested_by_user_id -- with nobody when it clears the field --
// while the creator named by work_item.created_by stays. It is how the nobody-to-ask
// refusal judges a request that changes the requester in the same PATCH.
func changeRequestCreatorUserIDsWith(ctx context.Context, q crQuerier, workItemID string, requestedByOverride **string) (map[string]bool, error) {
	ids := map[string]bool{}
	var requestedBy, createdBy *string
	err := q.QueryRow(ctx, `
		SELECT cr.requested_by_user_id::text, wi.created_by
		FROM work_item wi
		LEFT JOIN change_request cr ON cr.id = wi.id
		WHERE wi.id = $1`, workItemID).Scan(&requestedBy, &createdBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ids, nil
		}
		return nil, fmt.Errorf("read change request creator: %w", err)
	}
	if requestedByOverride != nil {
		requestedBy = *requestedByOverride
	}
	if requestedBy != nil && *requestedBy != "" {
		ids[strings.ToLower(*requestedBy)] = true
	}
	if createdBy != nil && strings.TrimSpace(*createdBy) != "" {
		rows, err := q.Query(ctx, `SELECT id::text FROM "user" WHERE LOWER(email) = LOWER($1)`, strings.TrimSpace(*createdBy))
		if err != nil {
			return nil, fmt.Errorf("resolve change request creator: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, fmt.Errorf("scan change request creator: %w", err)
			}
			ids[strings.ToLower(id)] = true
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("change request creator rows: %w", err)
		}
	}
	return ids, nil
}

// groupMemberIDs lists the distinct user ids in the "group" identified by
// groupID (team_member.group_id -- see CLAUDE.md on why group_id, not team_id).
func groupMemberIDs(ctx context.Context, q crQuerier, groupID string) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT DISTINCT user_id::text FROM team_member WHERE group_id = $1::uuid`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan group member: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// internalApproverIDs returns which of userIDs may be provisioned as, or act
// as, an approver of an INTERNAL stage: an active ("user".is_active, NULL
// counting as active, like everywhere else) user whose user_type is INTERNAL.
// External (customer/partner) users, system users and users with no derivable
// type are never eligible; neither is an id with no "user" row.
func internalApproverIDs(ctx context.Context, q crQuerier, userIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `
		SELECT u.id::text FROM "user" u
		WHERE u.id = ANY($1::uuid[])
		  AND u.user_type = 'INTERNAL'::user_type_enum
		  AND COALESCE(u.is_active, true)`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("check internal approvers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan internal approver: %w", err)
		}
		out[strings.ToLower(id)] = true
	}
	return out, rows.Err()
}

// onlyInternalApprovers keeps the members that internalApproverIDs accepts,
// in their original order.
func onlyInternalApprovers(ctx context.Context, q crQuerier, members []string) ([]string, error) {
	internal, err := internalApproverIDs(ctx, q, members)
	if err != nil {
		return nil, err
	}
	var kept []string
	for _, m := range members {
		if internal[strings.ToLower(m)] {
			kept = append(kept, m)
		}
	}
	return kept, nil
}

// stageKindNeedsInternalApprover reports whether the stage kind is an
// INTERNAL one (Peer, CAB, Review) whose approvers must be internal
// users. The customer stages and unclassified (ServiceNow-synced) stages are
// not: customer contacts are external by nature.
func stageKindNeedsInternalApprover(kind approvalStageKind) bool {
	switch kind {
	case stageKindPeer, stageKindCAB, stageKindReview:
		return true
	}
	return false
}

// stageKindName is the stage kind's label in messages.
func stageKindName(kind approvalStageKind) string {
	switch kind {
	case stageKindPeer:
		return approvalStageLabelPeer
	case stageKindCAB:
		return approvalStageLabelCAB
	case stageKindReview:
		return approvalStageLabelReview
	case stageKindCustomerApproval:
		return approvalStageLabelCustomerApproval
	case stageKindCustomerReview:
		return approvalStageLabelCustomerReview
	}
	return "this"
}

// noInternalMembersMessage is the ValidationError for a CAB / Review
// pool that has members but none who is an active internal user, so the
// operator can see that the people are there and why they do not count.
func noInternalMembersMessage(poolDescription, label string) string {
	return fmt.Sprintf(
		"%s has no active internal (WSO2) members to provision as %s approvers: external/customer users and inactive users cannot approve an internal stage",
		poolDescription, label)
}

// noInternalMembersError is the ValidationError of a pool whose members include
// nobody who counts as an internal approver: noInternalMembersMessage plus, in
// brackets, how many members there were and why each did not count
// (describeExcludedMembers: counts by user_type / inactive / no user record, never
// names), so the operator can tell "the group is empty of staff" from "the group's
// members have not had their user type resolved yet". The rule itself is not
// loosened: only the message changes. A failure to count leaves the plain message.
func noInternalMembersError(ctx context.Context, q crQuerier, poolDescription, label string, members []string, creatorIDs map[string]bool) error {
	msg := noInternalMembersMessage(poolDescription, label)
	if summary, err := describeExcludedMembers(ctx, q, members, creatorIDs); err == nil {
		msg += " (" + poolDescription + ": " + summary + ")"
	} else {
		slog.WarnContext(ctx, "could not describe the excluded group members", "pool", poolDescription, "error", err)
	}
	return &apierror.ValidationError{Msg: msg}
}

// notMirroredGroupNote is what a refusal about an approver group that has no members adds,
// for the groups the approval flow resolves by name (CAB Approval, Devops Approval): a group
// may have no members, which is an operations matter -- their membership is maintained in the
// portal database itself (no admin screen, no schema of its own), and until somebody has done
// that a stage that needs the group cannot be created, so the refusal names the group.
func notMirroredGroupNote(groupName string) string {
	return fmt.Sprintf("the sync from the previous system does not mirror the membership of the %q group: it is maintained in the portal database (one team_member row per approver, with group_id set to that group)", groupName)
}

// namedGroup resolves a group by name: its id (preferring, when the mirror
// produced several same-named rows, the one that actually has members) and
// its distinct members. A member is anyone with team_member.group_id pointing
// at a group of that name, or team_member.team_id pointing at a team of that
// name (the CR-notice flow addresses these audiences by team name). exists is
// false when no such group row exists at all.
func namedGroup(ctx context.Context, q crQuerier, name string) (groupID string, members []string, exists bool, err error) {
	err = q.QueryRow(ctx, `
		SELECT g.id::text FROM "group" g WHERE g.name = $1
		ORDER BY (SELECT COUNT(*) FROM team_member tm WHERE tm.group_id = g.id) DESC, g.created_on ASC, g.id ASC
		LIMIT 1`, name).Scan(&groupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, false, nil
		}
		return "", nil, false, fmt.Errorf("resolve group %q: %w", name, err)
	}
	rows, err := q.Query(ctx, `
		SELECT DISTINCT tm.user_id::text FROM team_member tm
		WHERE tm.group_id IN (SELECT id FROM "group" WHERE name = $1)
		   OR tm.team_id  IN (SELECT id FROM team WHERE name = $1)`, name)
	if err != nil {
		return "", nil, true, fmt.Errorf("list members of group %q: %w", name, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", nil, true, fmt.Errorf("scan member of group %q: %w", name, err)
		}
		members = append(members, id)
	}
	return groupID, members, true, rows.Err()
}

// approvalPool is a resolved approver pool: the group the stage is recorded
// against and the users to seed as approvers.
type approvalPool struct {
	groupID string
	members []string
}

// resolvePeerPool resolves the peer approval pool of a Normal change: every
// member of the change's assigned group (team_member.group_id) who is an
// active INTERNAL user (internalApproverIDs: a customer who happens to be a
// member is never placed in the pool). Whether a member is experienced enough
// to peer-approve is decided when people are added to the group, not here.
//
// The creator is never counted as a way to satisfy the pool (the caller still
// lists them, as cancelled). When there is no assigned group, or the assigned
// group yields nobody eligible -- no active internal member other than the
// creator -- the PeerApprovalFallbackGroupName group ("Devops Approval") is
// used instead, subject to the same rules.
func resolvePeerPool(ctx context.Context, q crQuerier, assignedTeamID *string, creatorIDs map[string]bool) (approvalPool, error) {
	// why is what each group that was tried yielded, for the refusal below: when
	// nobody is eligible the caller is told how many people were looked at and why
	// each did not count (counts only -- no names).
	var why []string
	eligible := func(groupLabel string, members []string) ([]string, bool, error) {
		kept, err := onlyInternalApprovers(ctx, q, members)
		if err != nil {
			return nil, false, err
		}
		requestable := false
		for _, m := range kept {
			if !creatorIDs[strings.ToLower(m)] {
				requestable = true
				break
			}
		}
		if !requestable && len(members) > 0 {
			summary, err := describeExcludedMembers(ctx, q, members, creatorIDs)
			if err != nil {
				return nil, false, err
			}
			why = append(why, groupLabel+": "+summary)
		}
		return kept, requestable, nil
	}

	if assignedTeamID != nil && *assignedTeamID != "" {
		members, err := groupMemberIDs(ctx, q, *assignedTeamID)
		if err != nil {
			return approvalPool{}, err
		}
		kept, ok, err := eligible("the assigned group", members)
		if err != nil {
			return approvalPool{}, err
		}
		if ok {
			return approvalPool{groupID: *assignedTeamID, members: kept}, nil
		}
	}

	gid, members, exists, err := namedGroup(ctx, q, domain.PeerApprovalFallbackGroupName)
	if err != nil {
		return approvalPool{}, err
	}
	if exists {
		kept, ok, err := eligible(fmt.Sprintf("the %q group", domain.PeerApprovalFallbackGroupName), members)
		if err != nil {
			return approvalPool{}, err
		}
		if ok {
			return approvalPool{groupID: gid, members: kept}, nil
		}
	}
	msg := fmt.Sprintf(
		"no eligible peer approvers: the assigned group has no active internal members other than the change's creator (external/customer users cannot approve), and the %q group has none either",
		domain.PeerApprovalFallbackGroupName)
	if len(why) > 0 {
		msg += " (" + strings.Join(why, "; ") + ")"
	}
	// A fallback group nobody has put members in is the usual reason on a synced
	// environment: say whose job that is.
	if !exists || len(members) == 0 {
		msg += ": " + notMirroredGroupNote(domain.PeerApprovalFallbackGroupName)
	}
	return approvalPool{}, &apierror.ValidationError{Msg: msg}
}

// excludedMembers is what describeExcludedMembers counts about a group's members.
type excludedMembers struct {
	total int
	// noUser: no "user" row for the member id. inactive: "user".is_active false.
	noUser, inactive int
	// byType: the active members whose user_type is not INTERNAL, by label ("" is
	// reported as "no user_type").
	byType map[string]int
	// creator: active internal members left out only because they created / requested
	// the change.
	creator int
	// eligible: active internal members who are not the creator.
	eligible int
}

// summary renders the counts, most numerous reason first: "14 members, none
// eligible: 9 user_type NOT_AVAILABLE, 3 inactive, 1 external, 1 creator". With an
// eligible member left it says how many are ("14 members, 2 eligible").
func (e excludedMembers) summary() string {
	type reason struct {
		label string
		n     int
	}
	var reasons []reason
	add := func(label string, n int) {
		if n > 0 {
			reasons = append(reasons, reason{label, n})
		}
	}
	add("no user record", e.noUser)
	add("inactive", e.inactive)
	for t, n := range e.byType {
		switch t {
		case "EXTERNAL":
			add("external", n)
		case "SYSTEM":
			add("system", n)
		case "":
			add("no user_type", n)
		default:
			add("user_type "+t, n)
		}
	}
	add("creator", e.creator)
	sort.SliceStable(reasons, func(i, j int) bool {
		if reasons[i].n != reasons[j].n {
			return reasons[i].n > reasons[j].n
		}
		return reasons[i].label < reasons[j].label
	})
	parts := make([]string, len(reasons))
	for i, r := range reasons {
		parts[i] = fmt.Sprintf("%d %s", r.n, r.label)
	}
	noun := "members"
	if e.total == 1 {
		noun = "member"
	}
	if e.eligible > 0 {
		return fmt.Sprintf("%d %s, %d eligible", e.total, noun, e.eligible)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%d %s, none eligible", e.total, noun)
	}
	return fmt.Sprintf("%d %s, none eligible: %s", e.total, noun, strings.Join(parts, ", "))
}

// countExcludedMembers classifies each distinct member id by why it does or does
// not count as an approver of an internal stage (see internalApproverIDs, whose
// rule it mirrors exactly: an active user whose user_type is INTERNAL; and the
// creator, who is listed but never asked). Counts only: the point is to make a
// "nobody is eligible" refusal diagnosable -- a migrated group's members may all be
// users whose type could not be derived -- without naming anyone and without
// loosening the rule.
func countExcludedMembers(ctx context.Context, q crQuerier, members []string, creatorIDs map[string]bool) (excludedMembers, error) {
	out := excludedMembers{byType: map[string]int{}}
	seen := map[string]bool{}
	var ids []string
	for _, m := range members {
		key := strings.ToLower(strings.TrimSpace(m))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		ids = append(ids, key)
	}
	out.total = len(ids)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `
		SELECT u.id::text, COALESCE(u.is_active, true), COALESCE(u.user_type::text, '')
		FROM "user" u WHERE u.id = ANY($1::uuid[])`, ids)
	if err != nil {
		return out, fmt.Errorf("describe group members: %w", err)
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var id, userType string
		var active bool
		if err := rows.Scan(&id, &active, &userType); err != nil {
			return out, fmt.Errorf("describe group members: scan: %w", err)
		}
		found[strings.ToLower(id)] = true
		switch {
		case !active:
			out.inactive++
		case userType != "INTERNAL":
			out.byType[userType]++
		case creatorIDs[strings.ToLower(id)]:
			out.creator++
		default:
			out.eligible++
		}
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("describe group members: %w", err)
	}
	out.noUser = len(ids) - len(found)
	return out, nil
}

// describeExcludedMembers is countExcludedMembers rendered for a message.
func describeExcludedMembers(ctx context.Context, q crQuerier, members []string, creatorIDs map[string]bool) (string, error) {
	counts, err := countExcludedMembers(ctx, q, members, creatorIDs)
	if err != nil {
		return "", err
	}
	return counts.summary(), nil
}

// resolveApprovalPool resolves checkpoint's approver pool for the change
// request. Its errors are caller-actionable ValidationErrors with no stage
// created yet.
func resolveApprovalPool(ctx context.Context, q crQuerier, cp changeRequestApprovalCheckpoint, assignedTeamID *string, creatorIDs map[string]bool) (approvalPool, error) {
	switch cp.Pool {
	case poolPeer:
		return resolvePeerPool(ctx, q, assignedTeamID, creatorIDs)
	case poolNamedGroup:
		gid, members, exists, err := namedGroup(ctx, q, cp.GroupName)
		if err != nil {
			return approvalPool{}, err
		}
		if !exists {
			return approvalPool{}, &apierror.ValidationError{Msg: fmt.Sprintf("the %q group does not exist, so a %s stage cannot be provisioned: %s", cp.GroupName, cp.Label, notMirroredGroupNote(cp.GroupName))}
		}
		if len(members) == 0 {
			return approvalPool{}, &apierror.ValidationError{Msg: fmt.Sprintf("the %q group has no members to provision as %s approvers: %s", cp.GroupName, cp.Label, notMirroredGroupNote(cp.GroupName))}
		}
		all := members
		if members, err = onlyInternalApprovers(ctx, q, members); err != nil {
			return approvalPool{}, err
		}
		if len(members) == 0 {
			return approvalPool{}, noInternalMembersError(ctx, q, fmt.Sprintf("the %q group", cp.GroupName), cp.Label, all, creatorIDs)
		}
		requestable := false
		for _, m := range members {
			if !creatorIDs[strings.ToLower(m)] {
				requestable = true
				break
			}
		}
		if !requestable {
			return approvalPool{}, &apierror.ValidationError{Msg: fmt.Sprintf("the %q group has no members other than the creator to provision as %s approvers", cp.GroupName, cp.Label)}
		}
		return approvalPool{groupID: gid, members: members}, nil
	default: // poolAssignedGroup
		if assignedTeamID == nil || *assignedTeamID == "" {
			return approvalPool{}, &apierror.ValidationError{Msg: fmt.Sprintf("the assigned team has no members to provision as %s approvers", cp.Label)}
		}
		members, err := groupMemberIDs(ctx, q, *assignedTeamID)
		if err != nil {
			return approvalPool{}, err
		}
		if len(members) == 0 {
			return approvalPool{}, &apierror.ValidationError{Msg: fmt.Sprintf("the assigned team has no members to provision as %s approvers", cp.Label)}
		}
		all := members
		if members, err = onlyInternalApprovers(ctx, q, members); err != nil {
			return approvalPool{}, err
		}
		if len(members) == 0 {
			return approvalPool{}, noInternalMembersError(ctx, q, "the assigned team", cp.Label, all, creatorIDs)
		}
		requestable := false
		for _, m := range members {
			if !creatorIDs[strings.ToLower(m)] {
				requestable = true
				break
			}
		}
		if !requestable {
			return approvalPool{}, &apierror.ValidationError{Msg: fmt.Sprintf("the assigned team has no members other than the requester to provision as %s approvers", cp.Label)}
		}
		return approvalPool{groupID: *assignedTeamID, members: members}, nil
	}
}

// approverDecisionBlock reports why userID may not decide an approval on the
// change request right now, or nil when nothing blocks them. Only the rules
// that depend on WHO the person is are checked here (the creator rule and the
// internal-only rule); that they hold a REQUESTED row is decided by the
// caller's own UPDATE.
//
//   - The creator of a change request (see changeRequestCreatorUserIDs) may
//     not approve it at any stage. They may still cancel it.
//   - Only an active INTERNAL user may decide an internal stage (Peer, CAB,
//     Review), even if a row for them exists (a row can predate the
//     rule, or the user's type can change after provisioning). The customer
//     stages are not subject to it.
//
// stageKind is the kind of the stage the caller's pending row belongs to; pass
// stageKindOther when it is not known.
func approverDecisionBlock(ctx context.Context, q crQuerier, userID string, creatorIDs map[string]bool, kind approvalStageKind) error {
	if creatorIDs[strings.ToLower(userID)] {
		return &apierror.ForbiddenError{Code: apierror.CodeChangeRequestForbidden, Msg: "the creator of a change request cannot approve it"}
	}
	if stageKindNeedsInternalApprover(kind) {
		internal, err := internalApproverIDs(ctx, q, []string{userID})
		if err != nil {
			return err
		}
		if !internal[strings.ToLower(userID)] {
			return &apierror.ForbiddenError{Code: apierror.CodeChangeRequestForbidden, Msg: fmt.Sprintf(
				"only active internal (WSO2) users can approve or reject the %s stage of a change request; external/customer users cannot", stageKindName(kind))}
		}
	}
	return nil
}

// changeRequestGateSnapshot is what the customer gates and the approval
// routing need to know about a change request, read once under a row lock.
type changeRequestGateSnapshot struct {
	// state is the upper-case change_request_state_enum label, "" when NULL.
	state string
	// model is the upper-case change_model label, "" when NULL.
	model            string
	approvalRequired bool
	reviewRequired   bool
	// projectID is the stored Customer Project (work_item.project_id), nil when
	// the change has none.
	projectID *string
	// priorWriter is work_item.updated_by as the PATCH found it, before its own write
	// replaced it (set by lockChangeRequestForPatch only; read=false from every other reader
	// of the snapshot, which run after a write of their own). A staff time response reads
	// the proposer of a waiting time from it, never from the column afterwards.
	priorWriter lastWriter
}

// lockChangeRequestGateSnapshot reads (and locks, FOR UPDATE) the fields the
// customer gates depend on. Locking keeps a concurrent approval decision
// (which takes the same lock) from moving the change past a gate between this
// read and the write that depends on it.
//
// The lock is on the change_request row only. A caller that must not act on a
// stale snapshot of the work_item side (the Customer Project) locks that row
// first and reads this afterwards, in a separate statement:
// lockChangeRequestForPatch.
func lockChangeRequestGateSnapshot(ctx context.Context, tx pgx.Tx, id string) (changeRequestGateSnapshot, error) {
	var state, model, project *string
	var snap changeRequestGateSnapshot
	err := tx.QueryRow(ctx,
		`SELECT cr.state::text, cr.change_model::text, cr.customer_approval_required, cr.customer_review_required, wi.project_id::text
		 FROM change_request cr LEFT JOIN work_item wi ON wi.id = cr.id
		 WHERE cr.id = $1 FOR UPDATE OF cr`, id,
	).Scan(&state, &model, &snap.approvalRequired, &snap.reviewRequired, &project)
	if errors.Is(err, pgx.ErrNoRows) {
		return snap, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return snap, fmt.Errorf("patch change request: read approval gates: %w", err)
	}
	snap.state = strings.ToUpper(stringOrEmpty(state))
	snap.model = strings.ToUpper(stringOrEmpty(model))
	if hasProjectID(project) {
		p := strings.ToLower(strings.TrimSpace(*project))
		snap.projectID = &p
	}
	return snap, nil
}

// requestApprovalDestination is the state Request Approval writes: the flow's
// own (Assess / Authorize / Scheduled), except that a flow with no internal
// approval to wait for (Standard) goes to Customer Approval instead of
// Scheduled when the customer's approval is required. Normal reaches the customer
// gate later, when CAB approves (approvalGateTarget); Emergency never reaches it.
func requestApprovalDestination(flow changeRequestFlow, customerApprovalRequired bool) domain.ChangeRequestState {
	if flow.checkpoint == nil && flow.requestState == domain.ChangeRequestStateScheduled && customerApprovalRequired {
		return domain.ChangeRequestStateCustomerApproval
	}
	return flow.requestState
}

// approvalGateTarget is the upper-case state a CAB approval moves the change to:
// Customer Approval when the customer's approval is required, Scheduled otherwise.
// The caller passes the requirement in effect (effectiveCustomerGates): false for
// an Emergency change, whatever its stored box says.
func approvalGateTarget(customerApprovalRequired bool) string {
	if customerApprovalRequired {
		return "CUSTOMER_APPROVAL"
	}
	return "SCHEDULED"
}

// approvalRequirementEditable reports whether customer_approval_required may
// still be changed in the given (upper-case) state: until the approval gate it
// controls has been passed, i.e. while the change is New, Assess or Authorize
// (a NULL state is a pre-lifecycle legacy row and counts as New).
func approvalRequirementEditable(state string) bool {
	switch state {
	case "", "NEW", "ASSESS", "AUTHORIZE":
		return true
	}
	return false
}

// reviewRequirementEditable reports whether customer_review_required may still
// be changed in the given (upper-case) state: until the change leaves Review,
// the step whose next move it decides. Customer Review and every terminal
// state are past it.
func reviewRequirementEditable(state string) bool {
	switch state {
	case "CUSTOMER_REVIEW", "ROLLBACK", "CLOSED", "CANCELED":
		return false
	}
	return true
}

// validateCustomerGateEdits judges an edit of customer_approval_required /
// customer_review_required against the customer requirements lock (see
// change_request_customer_lock.go): free in New, ADD-ONLY afterwards. A write of
// the value already stored is a no-op and is always accepted, so a client that
// resends the whole form is not punished for fields it did not touch.
//
// After New, in every state: true -> false is refused. false -> true is accepted
// only while the gate the box controls is still ahead (approvalRequirementEditable
// / reviewRequirementEditable, the cut-offs kept from before the lock) and only on
// a change that has a Customer Project, which can no longer be set.
func validateCustomerGateEdits(snap changeRequestGateSnapshot, approvalRequired, reviewRequired *bool) error {
	hasProject := hasProjectID(snap.projectID)
	if err := checkRequirementEdit(customerApprovalBox, snap.state, snap.approvalRequired, approvalRequired, hasProject); err != nil {
		return err
	}
	return checkRequirementEdit(customerReviewBox, snap.state, snap.reviewRequired, reviewRequired, hasProject)
}

// ---------------------------------------------------------------------------
// Customer Approval / Customer Review stages
// ---------------------------------------------------------------------------

// customerStageSpec describes one of the two customer stages: the state it is
// entered with, its label (approval_stage.checkpoint_label), its kind, and
// what its outcome does to the change.
type customerStageSpec struct {
	// state is the upper-case change_request_state_enum label the stage
	// belongs to.
	state string
	label string
	kind  approvalStageKind
	// approvedState / rejectedState are where the stage's outcome moves the
	// change.
	approvedState, rejectedState string
	// approvedFlagColumn is the change_request column the approval stamps.
	approvedFlagColumn string
	// what is the customer's answer in words, for messages.
	what string
}

var (
	customerApprovalStageSpec = customerStageSpec{
		state: "CUSTOMER_APPROVAL", label: approvalStageLabelCustomerApproval, kind: stageKindCustomerApproval,
		approvedState: "SCHEDULED", rejectedState: "CANCELED", approvedFlagColumn: "is_customer_approval_required",
		what: "approval",
	}
	// A rejected customer review moves the change to Rollback: the state the
	// ServiceNow workflow that handles a rejected review writes (see
	// ChangeRequestActionBar.tsx in the webapp -- "rollback is written by the
	// workflow that handles a rejected review"). Rollback is terminal here
	// exactly as it is everywhere else in this data source.
	customerReviewStageSpec = customerStageSpec{
		state: "CUSTOMER_REVIEW", label: approvalStageLabelCustomerReview, kind: stageKindCustomerReview,
		approvedState: "CLOSED", rejectedState: "ROLLBACK", approvedFlagColumn: "is_customer_review_required",
		what: "review",
	}
)

// customerStageSpecForState returns the customer stage a change in the given
// (upper-case) state waits on, nil for every other state.
func customerStageSpecForState(state string) *customerStageSpec {
	switch state {
	case customerApprovalStageSpec.state:
		return &customerApprovalStageSpec
	case customerReviewStageSpec.state:
		return &customerReviewStageSpec
	}
	return nil
}

// customerStageSpecForLabel is customerStageSpecForState keyed by the stage's
// checkpoint label.
func customerStageSpecForLabel(label string) *customerStageSpec {
	switch label {
	case customerApprovalStageSpec.label:
		return &customerApprovalStageSpec
	case customerReviewStageSpec.label:
		return &customerReviewStageSpec
	}
	return nil
}

// customerStageSpecForKind is customerStageSpecForState keyed by stage kind.
func customerStageSpecForKind(kind approvalStageKind) *customerStageSpec {
	switch kind {
	case stageKindCustomerApproval:
		return &customerApprovalStageSpec
	case stageKindCustomerReview:
		return &customerReviewStageSpec
	}
	return nil
}

// customerGroupDisplayName names the customer stages' approver pool in the
// approvals read response: the Customer Group is not a stored group any more,
// it is the change request's project's registered contacts.
const customerGroupDisplayName = "Customer Group"

// customerContactUserIDs lists the distinct "user" ids of the project's
// registered portal-user contacts (the Customer Group -- see
// loadProjectCustomerContacts). A contact with no "user" row cannot hold an
// approval and is skipped.
func customerContactUserIDs(ctx context.Context, q crQuerier, projectID string) ([]string, error) {
	contacts, err := loadProjectCustomerContacts(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var ids []string
	for _, c := range contacts {
		key := strings.ToLower(c.userID)
		if c.userID == "" || seen[key] {
			continue
		}
		seen[key] = true
		ids = append(ids, c.userID)
	}
	return ids, nil
}

// anyContactToAsk is THE definition of "somebody can be asked" for a customer
// stage: at least one of the Customer Group's members (customerContactUserIDs:
// the project's REGISTERED portal-user contacts whose user is active -- never an
// invited-only or a deactivated contact) is not one of the change request's
// creators (changeRequestCreatorUserIDs / changeRequestCreatorsForApprover: the
// requester never approves their own change). provisionCustomerStage, the
// read-only twin legacyStageWouldBeProvisioned and Request Approval's refusal
// (customerGroupCanBeAsked) all ask this one function, so the refusal predicts
// exactly the "nobody asked" outcome of the provisioning and cannot drift from it.
func anyContactToAsk(members []string, creatorIDs map[string]bool) bool {
	for _, m := range members {
		if !creatorIDs[strings.ToLower(m)] {
			return true
		}
	}
	return false
}

// customerGroupCanBeAsked reports whether a customer stage of the change request
// workItemID, provisioned now for the Customer Project projectID, would ask
// somebody: the members the stage asks (customerContactUserIDs) and the change's
// creators (changeRequestCreatorUserIDs), judged by anyContactToAsk -- exactly
// what provisionCustomerStage reads. Nothing is written. A blank project has
// nobody (the caller says that in its own words).
func customerGroupCanBeAsked(ctx context.Context, q crQuerier, workItemID, projectID string) (bool, error) {
	return customerGroupCanBeAskedWith(ctx, q, workItemID, projectID, nil)
}

// customerGroupCanBeAskedWith is customerGroupCanBeAsked with the creators judged as
// if a PATCH's own requestedById were already written (changeRequestCreatorUserIDsWith):
// the refusal of a request that changes the requester must predict the provisioning
// that follows the write, not the one before it.
func customerGroupCanBeAskedWith(ctx context.Context, q crQuerier, workItemID, projectID string, requestedBy **string) (bool, error) {
	if strings.TrimSpace(projectID) == "" {
		return false, nil
	}
	members, err := customerContactUserIDs(ctx, q, projectID)
	if err != nil {
		return false, err
	}
	if len(members) == 0 {
		return false, nil
	}
	creatorIDs, err := changeRequestCreatorUserIDsWith(ctx, q, workItemID, requestedBy)
	if err != nil {
		return false, err
	}
	return anyContactToAsk(members, creatorIDs), nil
}

// stageApproverUserIDs lists every approver (whatever their status) of a stage.
func stageApproverUserIDs(ctx context.Context, q crQuerier, stageID string) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT approver_user_id::text FROM approval_stage_approver WHERE stage_id = $1`, stageID)
	if err != nil {
		return nil, fmt.Errorf("list stage approvers: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan stage approver: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// liveCustomerStage is a customer stage that still has a REQUESTED approver.
type liveCustomerStage struct {
	stageID   string
	label     string
	groupID   string
	groupName string
}

// liveCustomerStages lists the change's customer stages with at least one
// REQUESTED approver, oldest first.
func liveCustomerStages(ctx context.Context, q crQuerier, workItemID string) ([]liveCustomerStage, error) {
	rows, err := q.Query(ctx, `
		SELECT ast.id::text, ast.checkpoint_label, COALESCE(ast.assignment_group_id::text, ''), COALESCE(g.name, '')
		FROM approval_stage ast
		LEFT JOIN "group" g ON g.id = ast.assignment_group_id
		WHERE ast.work_item_id = $1
		  AND ast.checkpoint_label IN ($2, $3)
		  AND EXISTS (SELECT 1 FROM approval_stage_approver asa WHERE asa.stage_id = ast.id AND asa.state = 'REQUESTED')
		ORDER BY ast.created_on ASC, ast.id ASC`,
		workItemID, approvalStageLabelCustomerApproval, approvalStageLabelCustomerReview)
	if err != nil {
		return nil, fmt.Errorf("list live customer stages: %w", err)
	}
	defer rows.Close()
	var out []liveCustomerStage
	for rows.Next() {
		var st liveCustomerStage
		if err := rows.Scan(&st.stageID, &st.label, &st.groupID, &st.groupName); err != nil {
			return nil, fmt.Errorf("scan live customer stage: %w", err)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// liveCustomerStageForState returns the live customer stage of the kind the
// given (upper-case) state waits on, nil when the state has none or none is
// live.
func liveCustomerStageForState(ctx context.Context, q crQuerier, workItemID, state string) (*liveCustomerStage, error) {
	spec := customerStageSpecForState(state)
	if spec == nil {
		return nil, nil
	}
	live, err := liveCustomerStages(ctx, q, workItemID)
	if err != nil {
		return nil, err
	}
	for i := range live {
		if live[i].label == spec.label {
			return &live[i], nil
		}
	}
	return nil, nil
}

// withoutStaffRollbackWhileCustomerReviewPending takes "rollback" out of the
// next states of a change in Customer Review while the customer group's review
// request is live: a failed review is then the customer's to give (a member
// rejecting the review rolls the change back), not a staff action. Cancel stays.
//
// This is the only thing left to filter. The customer's own approval and review
// ("scheduled" out of Customer Approval, "closed" out of Customer Review) are
// never offered to staff at all, live stage or not: see
// changeRequestForwardNextStates.
func withoutStaffRollbackWhileCustomerReviewPending(state *string, nexts []string, liveStage bool) []string {
	if !liveStage || state == nil || nexts == nil || !strings.EqualFold(*state, string(domain.ChangeRequestStateCustomerReview)) {
		return nexts
	}
	out := make([]string, 0, len(nexts))
	for _, n := range nexts {
		if n != string(domain.ChangeRequestStateRollback) {
			out = append(out, n)
		}
	}
	return out
}

// customerStageManualRefusal is the 400 for a manual {state: rollback} out of
// Customer Review while the customer group's review request is pending: the
// failed review is given by one of them rejecting it, not by staff.
func customerStageManualRefusal(target string, spec *customerStageSpec, live *liveCustomerStage) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf(
		"state %q cannot be set manually: the customer's %s has been requested from the customer group (the registered contacts of the change request's project) and is given by one of them approving or rejecting it in the change request's approvals (POST /change-requests/{id}/approvals/decision)",
		target, spec.what)}
}

// ---------------------------------------------------------------------------
// The customer's own answer is the only way out of a customer state
// ---------------------------------------------------------------------------

// A change waiting in Customer Approval / Customer Review moves on only through
// the customer's own answer, given by a registered contact of the change
// request's project in the Customer Portal (answerCustomerStageViaPatch /
// decideChangeRequestApprovalTx). No WSO2 staff action records that answer, for
// any change, with or without anybody having been asked, because the answer is
// the customer's decision and ServiceNow's record of it is audited: a decision
// made for the customer and stored as theirs would be a compliance problem.
// What staff keep: Cancel (any state), Re-schedule from Customer Approval (the
// customer is asked again), Rollback from Customer Review (while nobody is
// being asked). Emergency changes are acted on without the customer's consent and
// never get here (effectiveCustomerGates).

// staffCustomerOutcomeFlagRefusal is the 400 a request that carries
// isCustomerApproved / isCustomerReviewed from anyone but the customer is
// refused with.
func staffCustomerOutcomeFlagRefusal(flag string, spec *customerStageSpec) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf(
		"%s cannot be set on the customer's behalf: the customer's %s can only be given by the customer in the Customer Portal", flag, spec.what)}
}

// refuseStaffCustomerOutcomeFlags refuses a PATCH from a caller that is not the
// customer when it carries isCustomerApproved or isCustomerReviewed, true or
// false, alone or with a state: those two fields ARE the customer's answer, and
// nobody else may give it. They used to be accepted from staff (stamping the
// flag, with or without moving the state); they are refused outright now, not
// ignored, so a client that still sends them learns it.
func refuseStaffCustomerOutcomeFlags(req domain.PatchChangeRequestRequest) error {
	if req.IsCustomerApproved != nil {
		return staffCustomerOutcomeFlagRefusal("isCustomerApproved", &customerApprovalStageSpec)
	}
	if req.IsCustomerReviewed != nil {
		return staffCustomerOutcomeFlagRefusal("isCustomerReviewed", &customerReviewStageSpec)
	}
	return nil
}

// customerOutcomeRefusal is the 400 for a staff PATCH that would take a change
// out of Customer Approval / Customer Review through a door only the customer's
// own answer opens ({state: scheduled} out of Customer Approval, {state: closed}
// out of Customer Review, or any other destination): refused whatever the
// project's contacts are, whether anybody was asked, and with or without a
// stage, and it changes nothing. The message says why and what staff can do
// instead, which depends on the state: Re-schedule (Customer Approval),
// Rollback (Customer Review, while nobody is being asked), and Cancel.
func customerOutcomeRefusal(ctx context.Context, q crQuerier, workItemID string, spec *customerStageSpec, target domain.ChangeRequestState) error {
	instead := "cancel the change or re-schedule it"
	if spec.state == crStateCustomerReview {
		instead = "roll the change back or cancel it"
		live, err := liveCustomerStageForState(ctx, q, workItemID, spec.state)
		if err != nil {
			return fmt.Errorf("patch change request: %w", err)
		}
		if live != nil {
			// A failed review is the customer's to give while they are being asked.
			instead = "cancel the change"
		}
	}
	return &apierror.ValidationError{Msg: fmt.Sprintf(
		"state %q cannot be set manually from %s: the customer's %s can only be given by the customer in the Customer Portal; %s instead",
		strings.ToLower(string(target)), strings.ToLower(spec.state), spec.what, instead)}
}

// refuseStaffExitFromCustomerState is the one guard behind "the change moves on
// only through the customer's own answer": for a change in Customer Approval or
// Customer Review it refuses every destination the PATCH would write except the
// exits that answer nothing for the customer -- Cancel; Re-schedule (authorize)
// out of Customer Approval; Rollback out of Customer Review (patchChangeRequestTx
// has refused that one already while the customer group's review is pending) --
// and staying where it is (a resent Request Approval / customer_review, no move).
// current is the change's upper-case state; target is the state about to be
// written. The transition graph (checkStaffStateRequest) accepts exactly these
// exits too; it leaves the customer states to this guard so that the refusal
// names the customer's answer -- {state: implement} out of Customer Approval
// would skip the customer as surely as {state: scheduled} would.
func refuseStaffExitFromCustomerState(ctx context.Context, q crQuerier, workItemID, current string, target domain.ChangeRequestState) error {
	spec := customerStageSpecForState(current)
	if spec == nil {
		return nil
	}
	t := strings.ToLower(string(target))
	if strings.EqualFold(t, current) || t == string(domain.ChangeRequestStateCanceled) {
		return nil
	}
	switch spec.state {
	case crStateCustomerApproval:
		if t == string(domain.ChangeRequestStateAuthorize) {
			return nil
		}
	case crStateCustomerReview:
		if t == string(domain.ChangeRequestStateRollback) {
			return nil
		}
	}
	return customerOutcomeRefusal(ctx, q, workItemID, spec, target)
}

// cancelLiveStageApprovers cancels the REQUESTED approvers of a stage (the
// stage itself stays, as a record: all its rows read CANCELLED).
func cancelLiveStageApprovers(ctx context.Context, tx pgx.Tx, stageID, actorEmail string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE approval_stage_approver SET state = 'CANCELLED', updated_on = NOW(), updated_by = $2
		 WHERE stage_id = $1 AND state = 'REQUESTED'`, stageID, actorEmail); err != nil {
		return fmt.Errorf("cancel customer stage approvers: %w", err)
	}
	return nil
}

// checkRescheduleWindow is the "Time Change = Yes" test of a Re-schedule:
// the request must carry a planned start and/or end that differs from what is
// stored (a value equal to the stored instant is not a change), and the
// resulting window must start before it ends (a zero-length window is no
// window). start and end are the normalised values of
// normalizePatchPlannedWindow, or nil.
func checkRescheduleWindow(ctx context.Context, tx pgx.Tx, id string, start, end *string) error {
	var startChanged, endChanged, inverted, empty bool
	err := tx.QueryRow(ctx, `
		SELECT COALESCE($1::text::timestamptz IS DISTINCT FROM start_on, false) AND $1::text IS NOT NULL,
		       COALESCE($2::text::timestamptz IS DISTINCT FROM end_on, false) AND $2::text IS NOT NULL,
		       COALESCE(COALESCE($1::text::timestamptz, start_on) > COALESCE($2::text::timestamptz, end_on), false),
		       COALESCE(COALESCE($1::text::timestamptz, start_on) = COALESCE($2::text::timestamptz, end_on), false)
		FROM change_request WHERE id = $3`, start, end, id).Scan(&startChanged, &endChanged, &inverted, &empty)
	if errors.Is(err, pgx.ErrNoRows) {
		return &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && strings.HasPrefix(pgErr.Code, "22") {
			return &apierror.ValidationError{Msg: "plannedStartOn and plannedEndOn must be valid date-times (RFC 3339)"}
		}
		return fmt.Errorf("patch change request: check re-schedule window: %w", err)
	}
	if !startChanged && !endChanged {
		return &apierror.ValidationError{Msg: "re-scheduling requires a changed planned start or end: send plannedStartOn and/or plannedEndOn with a value different from the stored one"}
	}
	if inverted {
		return &apierror.ValidationError{Msg: "the planned start must not be after the planned end"}
	}
	if empty {
		return &apierror.ValidationError{Msg: "the planned start must not be the same as the planned end: the window must have a duration"}
	}
	return nil
}

// cancelLiveCustomerStages cancels the requested approvers of every live
// customer stage (the stages stay, as a record).
func cancelLiveCustomerStages(ctx context.Context, tx pgx.Tx, workItemID, actorEmail string) error {
	live, err := liveCustomerStages(ctx, tx, workItemID)
	if err != nil {
		return err
	}
	if len(live) == 0 {
		return nil
	}
	if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
		return fmt.Errorf("cancel customer stages: escalate identity: %w", err)
	}
	for _, st := range live {
		if err := cancelLiveStageApprovers(ctx, tx, st.stageID, actorEmail); err != nil {
			return err
		}
	}
	return nil
}

// cancelPendingApprovers cancels every still-REQUESTED approver row of the
// change, on whatever stage (the stages stay, as a record). Used when the
// change reaches a state nothing can be approved in any more by hand (Closed,
// Canceled, Rollback -- see reconcileStaleApprovers, its only caller).
func cancelPendingApprovers(ctx context.Context, tx pgx.Tx, workItemID, actorEmail string) error {
	// approval_stage_approver writes are internal-only (see
	// provisionApprovalStage); the caller has proven their access to the
	// change by writing to it in this transaction.
	if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
		return fmt.Errorf("cancel pending approvers: escalate identity: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE approval_stage_approver SET state = 'CANCELLED', updated_on = NOW(), updated_by = $2
		 WHERE work_item_id = $1 AND state = 'REQUESTED'`, workItemID, actorEmail); err != nil {
		return fmt.Errorf("cancel pending approvers: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Stage <-> state: an approval is only actionable while the change is in the
// state its stage belongs to
// ---------------------------------------------------------------------------

// Upper-case change_request_state_enum labels.
const (
	crStateNew              = "NEW"
	crStateAssess           = "ASSESS"
	crStateAuthorize        = "AUTHORIZE"
	crStateCustomerApproval = "CUSTOMER_APPROVAL"
	crStateScheduled        = "SCHEDULED"
	crStateImplement        = "IMPLEMENT"
	crStateReview           = "REVIEW"
	crStateCustomerReview   = "CUSTOMER_REVIEW"
	crStateRollback         = "ROLLBACK"
	crStateClosed           = "CLOSED"
	crStateCanceled         = "CANCELED"
)

// knownChangeRequestStates is every label of change_request_state_enum. A state
// outside it (or a NULL one) is "unknown": nothing is guarded on such a change.
var knownChangeRequestStates = map[string]bool{
	crStateNew: true, crStateAssess: true, crStateAuthorize: true, crStateCustomerApproval: true,
	crStateScheduled: true, crStateImplement: true, crStateReview: true, crStateCustomerReview: true,
	crStateRollback: true, crStateClosed: true, crStateCanceled: true,
}

// terminalChangeRequestState reports whether the (upper-case) state is final:
// Closed, Canceled or Rollback. Nothing can be approved on such a change.
func terminalChangeRequestState(state string) bool {
	switch state {
	case crStateClosed, crStateCanceled, crStateRollback:
		return true
	}
	return false
}

// approvalStageDecidableState is the one change request state in which a stage
// of the given kind can be decided: Peer in Assess, CAB in Authorize,
// Review in Review, Customer Approval in Customer Approval, Customer Review in
// Customer Review. "" for a stage of unknown kind (stageKindOther -- a
// ServiceNow-synced stage with no recognisable label): it is not tied to any
// state and is never guarded.
func approvalStageDecidableState(kind approvalStageKind) string {
	switch kind {
	case stageKindPeer:
		return crStateAssess
	case stageKindCAB:
		return crStateAuthorize
	case stageKindReview:
		return crStateReview
	case stageKindCustomerApproval:
		return crStateCustomerApproval
	case stageKindCustomerReview:
		return crStateCustomerReview
	}
	return ""
}

// approvalStageOutOfState reports whether a stage of the given kind can no
// longer be decided because the change is in another state: the kind has a
// decidable state, the change's (upper-case) state is a known one, and they
// differ. False for an unknown kind and for a NULL / unknown state, so
// ServiceNow-synced data behaves exactly as before.
func approvalStageOutOfState(kind approvalStageKind, currentState string) bool {
	want := approvalStageDecidableState(kind)
	if want == "" {
		return false
	}
	current := strings.ToUpper(strings.TrimSpace(currentState))
	return knownChangeRequestStates[current] && current != want
}

// changeRequestStateDisplayName renders an upper-case state label for a
// message: CUSTOMER_REVIEW -> "Customer Review".
func changeRequestStateDisplayName(state string) string {
	words := strings.Fields(strings.ReplaceAll(strings.ToLower(state), "_", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// staleApprovalRefusal is the 409 for a decision on a stage whose state the
// change request has left (or never reached): the approval is no longer
// pending, and the message says where the change is and where the stage can be
// decided. Not a 403 -- the caller is allowed to decide, just not now.
func staleApprovalRefusal(kind approvalStageKind, currentState string) error {
	return &apierror.ConflictError{Code: apierror.CodeChangeRequestApprovalNotPending, Msg: fmt.Sprintf(
		"this approval is no longer pending: the change request is in %s, but the %s stage can only be decided while it is in %s",
		changeRequestStateDisplayName(currentState), stageKindName(kind),
		changeRequestStateDisplayName(approvalStageDecidableState(kind)))}
}

// reconcileStaleApprovers cancels the REQUESTED approver rows that are no
// longer actionable because of the state the change request is in NOW (read
// inside the transaction, so it sees the state this very transaction wrote):
//
//   - Closed, Canceled, Rollback: every still-requested row of the change, on
//     every stage (nothing can be approved on a finished change -- this
//     supersedes the earlier "Cancel leaves the internal stages' pending
//     approvers" behaviour);
//   - any other known state: the requested rows of every stage whose decidable
//     state (approvalStageDecidableState) is not the current one -- e.g. the
//     Review stage's approvers once the change has left Review for Customer
//     Review, the customer's once an old-flow Re-schedule sent it back to Authorize.
//
// The stages stay as a record; only the approver rows move to `CANCELLED`
// (updated_by = actorEmail, like every other cancel helper). A stage is
// classified by runtimeApprovalStageKind: its checkpoint_label first and, for a
// stage with none (a ServiceNow-synced one), only what its group, the change's
// type and state, and its position PROVE -- so an unlabeled stage is never
// cancelled here because of a guess about its position: a stage of unknown kind
// and a NULL / unknown change request state are left alone (a finished change's
// terminal cancel, above, is the one thing that takes every row).
//
// It must run AFTER the transaction has written the new state and provisioned
// the stage that state needs (a stage provisioned for the current state is
// never cancelled by it, so the order only matters the other way round).
// Idempotent. Callers: patchChangeRequestTx and DecideChangeRequestApproval,
// the two paths of the approval flow that write change_request.state, and the
// GitHub sync's githubMutationRepository.SetState.
func reconcileStaleApprovers(ctx context.Context, tx pgx.Tx, workItemID, actorEmail string) error {
	// approval_stage / approval_stage_approver writes are internal-only (see
	// provisionApprovalStage); the caller has proven their access to the change
	// by writing to it in this transaction. The reads need it too: a project
	// member sees its stages, an unrelated reader would not.
	if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
		return fmt.Errorf("reconcile approvers: escalate identity: %w", err)
	}
	var state, model *string
	if err := tx.QueryRow(ctx, `SELECT state::text, change_model::text FROM change_request WHERE id = $1`, workItemID).Scan(&state, &model); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("reconcile approvers: read state: %w", err)
	}
	current := strings.ToUpper(stringOrEmpty(state))
	if !knownChangeRequestStates[current] {
		return nil
	}
	if terminalChangeRequestState(current) {
		return cancelPendingApprovers(ctx, tx, workItemID, actorEmail)
	}

	// The stages that still hold a requested approver, with the ordinal
	// position the classifier's fallback needs (same ordering as
	// approvalStageInfo / changeRequestApprovalStagesQuery).
	rows, err := tx.Query(ctx, `
		SELECT ast.id::text, ast.checkpoint_label, g.name,
		       (SELECT COUNT(*) FROM approval_stage earlier
		         WHERE earlier.work_item_id = ast.work_item_id
		           AND (earlier.created_on, earlier.id) < (ast.created_on, ast.id))
		FROM approval_stage ast
		LEFT JOIN "group" g ON g.id = ast.assignment_group_id
		WHERE ast.work_item_id = $1
		  AND EXISTS (SELECT 1 FROM approval_stage_approver asa WHERE asa.stage_id = ast.id AND asa.state = 'REQUESTED')`,
		workItemID)
	if err != nil {
		return fmt.Errorf("reconcile approvers: list live stages: %w", err)
	}
	var stale []string
	for rows.Next() {
		var stageID string
		var label, groupName *string
		var pos int
		if err := rows.Scan(&stageID, &label, &groupName, &pos); err != nil {
			rows.Close()
			return fmt.Errorf("reconcile approvers: scan stage: %w", err)
		}
		if approvalStageOutOfState(runtimeApprovalStageKind(label, pos, groupName, stringOrEmpty(model), current), current) {
			stale = append(stale, stageID)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reconcile approvers: live stages: %w", err)
	}
	if len(stale) == 0 {
		return nil
	}
	tag, err := tx.Exec(ctx,
		`UPDATE approval_stage_approver SET state = 'CANCELLED', updated_on = NOW(), updated_by = $3
		 WHERE work_item_id = $1 AND stage_id = ANY($2::uuid[]) AND state = 'REQUESTED'`,
		workItemID, stale, actorEmail)
	if err != nil {
		return fmt.Errorf("reconcile approvers: cancel stale approvers: %w", err)
	}
	slog.InfoContext(ctx, "cancelled approver rows of stages the change request has left",
		"changeRequestId", workItemID, "state", current, "stages", len(stale), "approvers", tag.RowsAffected())
	return nil
}

// provisionCustomerStage brings the change's customer stage in step with the
// change as it now stands (read under FOR UPDATE), and is idempotent. The
// customer's approvers are the Customer Group of the change request: the
// REGISTERED portal-user contacts of its Customer Project, derived live
// (loadProjectCustomerContacts) -- nothing is stored, and the contacts of one
// project are never the approvers of another project's change request.
//
//   - state Customer Approval / Customer Review and a project with at least one
//     eligible contact (a registered contact whose user is active and is not
//     the creator): provisions the "Customer Approval" / "Customer Review"
//     stage -- one REQUESTED approver per eligible contact, the creator (if a
//     contact) listed CANCELLED like on every other stage -- unless a live stage
//     for exactly that contact set already exists (nothing to do) or the stage
//     has already been decided (approved or rejected: nothing left to ask);
//   - a live customer stage that no longer matches -- the project was changed,
//     its contacts changed, or the change left that state (e.g. was cancelled)
//     -- has its REQUESTED approvers cancelled, so there are never two live
//     customer stages and nobody is asked a question that no longer applies. A
//     changed project gets a fresh stage for its own contacts (first bullet);
//   - an Emergency change is treated like any other. The Emergency rule keeps it from
//     ENTERING a customer state (effectiveCustomerGates: CAB approval schedules it,
//     Review closes it), so a state that has a customer stage is only ever one it is
//     already waiting in -- a row from before the rule, or a migrated one --
//     and the question the customer was given there stands: it is asked, replaced
//     on a Re-schedule and answered like the same question on any other change.
//     Nothing here may leave such a change waiting in a customer state with nobody to
//     answer, which is all a guard on the model would do (it would cancel the live
//     request on a Re-schedule and put no fresh one in its place);
//   - no project, or no eligible contact: no stage and nobody is asked. There is
//     no staff path that answers for the customer: the change can be cancelled
//     or re-scheduled, or wait for a contact to register (a PATCH that restates
//     the project then asks them). Request Approval refuses to get a change here
//     in the first place (customerGroupCanBeAsked asks the same question); what
//     reaches it anyway is a legacy row or a project whose contacts all left after
//     approval was requested.
//
// The stage's assignment group is NULL (the Customer Group is not a "group"
// row); the approvals read response names it "Customer Group".
//
// Returns whether a stage was provisioned. Callers: the CAB approval
// cascade and Request Approval on a Standard change (entering Customer
// Approval), a {state: customer_review} PATCH, and any PATCH that sets or
// changes the state or the project.
func provisionCustomerStage(ctx context.Context, tx pgx.Tx, workItemID, actorEmail string) (bool, error) {
	var state, projectID *string
	if err := tx.QueryRow(ctx,
		`SELECT cr.state::text, wi.project_id::text
		 FROM change_request cr JOIN work_item wi ON wi.id = cr.id
		 WHERE cr.id = $1 FOR UPDATE OF cr`, workItemID).Scan(&state, &projectID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("provision customer stage: read change request: %w", err)
	}
	// No check of the change's type here, on purpose: an Emergency change is kept from
	// ENTERING a customer state elsewhere (effectiveCustomerGates), and the only way
	// it is in one is that it was already waiting there. Its customer's question
	// is then answered, replaced and cancelled like anybody's.
	spec := customerStageSpecForState(strings.ToUpper(stringOrEmpty(state)))
	live, err := liveCustomerStages(ctx, tx, workItemID)
	if err != nil {
		return false, err
	}
	if spec == nil && len(live) == 0 {
		return false, nil
	}

	// approval_stage / approval_stage_approver writes are internal-only (see
	// provisionApprovalStage). The caller has already proven their access to
	// this change request by writing to it in this transaction.
	if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
		return false, fmt.Errorf("provision customer stage: escalate identity: %w", err)
	}

	// The Customer Group, as it is now: the project's registered contacts.
	var members []string
	if spec != nil && projectID != nil {
		if members, err = customerContactUserIDs(ctx, tx, *projectID); err != nil {
			return false, fmt.Errorf("provision customer stage: %w", err)
		}
	}

	keep := false
	for _, st := range live {
		if spec != nil && st.label == spec.label && !keep {
			have, err := stageApproverUserIDs(ctx, tx, st.stageID)
			if err != nil {
				return false, err
			}
			if len(members) > 0 && sameIDSet(have, members) {
				keep = true
				continue
			}
		}
		if err := cancelLiveStageApprovers(ctx, tx, st.stageID, actorEmail); err != nil {
			return false, err
		}
	}
	if spec == nil || len(members) == 0 || keep {
		return false, nil
	}

	var decided bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM approval_stage_approver asa JOIN approval_stage ast ON ast.id = asa.stage_id
		                 WHERE ast.work_item_id = $1 AND ast.checkpoint_label = $2 AND asa.state IN ('APPROVED', 'REJECTED'))`,
		workItemID, spec.label).Scan(&decided); err != nil {
		return false, fmt.Errorf("provision customer stage: check decided stage: %w", err)
	}
	if decided {
		return false, nil
	}

	creatorIDs, err := changeRequestCreatorUserIDs(ctx, tx, workItemID)
	if err != nil {
		return false, fmt.Errorf("provision customer stage: %w", err)
	}
	if !anyContactToAsk(members, creatorIDs) {
		// Nobody can be asked (see the doc comment): staff can cancel or
		// re-schedule the change but cannot answer for the customer. Say why
		// no stage appeared. Request Approval and the add-only tick of a box
		// refuse this very case up front (customerGroupCanBeAsked), so a change
		// that has a box ticked only gets here when its contacts went away after
		// that, or when it is a legacy row.
		slog.InfoContext(ctx, "customer group has no eligible approvers, customer stage not provisioned",
			"changeRequestId", workItemID, "stage", spec.label)
		return false, nil
	}
	if err := insertApprovalStage(ctx, tx, workItemID, actorEmail, spec.label, approvalPool{members: members}, creatorIDs); err != nil {
		return false, err
	}
	return true, nil
}

// applyCustomerStageOutcome moves the change on after a customer stage was
// resolved by a decision: Customer Approval approved -> Scheduled (stamping
// is_customer_approval_required), rejected -> Canceled; Customer Review approved ->
// Closed (stamping is_customer_review_required), rejected -> Rollback. The change
// must still be in the stage's state (a stage whose change has moved on has
// had its approvers cancelled, so this is a defence, not a path). Returns
// whether the state moved.
func applyCustomerStageOutcome(ctx context.Context, tx pgx.Tx, workItemID string, spec *customerStageSpec, currentState string, approved bool) (bool, error) {
	if !strings.EqualFold(currentState, spec.state) {
		return false, nil
	}
	var ct pgconn.CommandTag
	var err error
	if approved {
		ct, err = tx.Exec(ctx,
			fmt.Sprintf(`UPDATE change_request SET state = $2::change_request_state_enum, %s = true WHERE id = $1`, spec.approvedFlagColumn),
			workItemID, spec.approvedState)
	} else {
		ct, err = tx.Exec(ctx, `UPDATE change_request SET state = $2::change_request_state_enum WHERE id = $1`, workItemID, spec.rejectedState)
	}
	if err != nil {
		return false, fmt.Errorf("decide change request approval: apply customer %s outcome: %w", spec.what, err)
	}
	if ct.RowsAffected() == 0 {
		return false, &apierror.NotFoundError{Msg: "change request not found"}
	}
	// Whatever the outcome leaves requested (a rolled-back / closed change is
	// final) is cancelled by DecideChangeRequestApproval's closing
	// reconcileStaleApprovers.
	return true, nil
}

// customerStageDecisionRefusal is the 403 for a caller with no pending approval
// of their own on a change that is waiting on the customer group: only a
// member of that group may answer, and the caller is not one (or has already
// been superseded by a sibling's answer, in which case the stage is no longer
// live and this returns nil). nil when the change is not waiting on a live
// customer stage; the caller falls back to its generic "no pending approval".
func customerStageDecisionRefusal(ctx context.Context, tx pgx.Tx, workItemID string) (*apierror.ForbiddenError, error) {
	var state *string
	if err := tx.QueryRow(ctx, `SELECT state::text FROM change_request WHERE id = $1`, workItemID).Scan(&state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("decide change request approval: read state: %w", err)
	}
	live, err := liveCustomerStageForState(ctx, tx, workItemID, strings.ToUpper(stringOrEmpty(state)))
	if err != nil {
		return nil, fmt.Errorf("decide change request approval: %w", err)
	}
	if live == nil {
		return nil, nil
	}
	spec := customerStageSpecForState(strings.ToUpper(stringOrEmpty(state)))
	return &apierror.ForbiddenError{Code: apierror.CodeChangeRequestNotAsked, Msg: fmt.Sprintf(
		"only members of the customer group (the registered contacts of this change request's project) can approve or reject the customer's %s of this change request", spec.what)}, nil
}
