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
// soon as possible"):
//
//	Normal:    New --(Request Approval)--> Assess [Peer Approval stage]
//	               --> Authorize [CAB Approval stage]
//	               --> Scheduled (automatically, on CAB approval)
//	               --> Implement --> Review --> Closed
//	Emergency: New --(Request Approval)--> Authorize [ECAB Approval stage only,
//	               no peer approval] --> Scheduled (automatically, on ECAB
//	               approval) --> Implement --> Review --> Closed
//	Standard:  New --(Request Approval)--> Scheduled (no approval stages at
//	               all) --> Implement --> Review --> Closed
//
// "Request Approval" is the one human action out of New and is always sent as
// {state: "assess"} -- the contract the webapp already has (legalNextStates of
// a New change request is ["assess", "canceled"] for every type). The
// resulting state is chosen here from the change's type, never by the caller.
// Scheduled is never a manual transition: it is reached only by the CAB/ECAB
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
//	    (CAB / ECAB approval, or Request Approval on a Standard change -- an
//	    assumption, Standard has no internal approvals to put the gate after) it
//	    moves it to Customer Approval instead when customer_approval_required.
//	    A human then records the customer's approval ({state: "scheduled"},
//	    legal ONLY from Customer Approval), which stamps is_customer_approval_required
//	    and schedules the change.
//	review gate: Review -> Customer Review -> Closed when
//	    customer_review_required, Review -> Closed otherwise. Closing from
//	    Customer Review records the customer's review (is_customer_review_required).
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
//	    stamped), rejecting it moves it to Rollback. The manual
//	    {state: scheduled} / {state: closed} is then refused: the answer comes
//	    from the approval.
//	without one (no project, or no eligible registered contact on it): no stage
//	    is provisioned and the manual path above stays the way out, so a change
//	    can never be stranded in a customer state with nobody able to answer.
//
// provisionCustomerStage keeps the stage in step with the change (state and
// project contacts) and is the one place that provisions, replaces or cancels it.

// A stage can only be decided while the change request is in the state it
// belongs to (approvalStageDecidableState): Peer in Assess, CAB / ECAB in
// Authorize, Review in Review, Customer Approval in Customer Approval, Customer
// Review in Customer Review. Three things keep an approver row from outliving
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
// A stage of unknown kind (a ServiceNow-synced stage with no recognisable
// label past the first two positions) and a change with no / unknown state are
// never guarded.

// Approver pools are INTERNAL-only. Every internal stage (Peer, CAB, ECAB,
// Review) is decided in the portal by WSO2 staff, who see every project; an
// external (customer) user sees only the projects they are a registered
// contact of, so an approver row for one could never be found, let alone
// decided. A pool is therefore filtered to users who are active
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
	approvalStageLabelECAB   = "ECAB Approval"
	approvalStageLabelReview = "Review"
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
	// CAB / ECAB groups).
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
		return changeRequestFlow{requestState: domain.ChangeRequestStateAuthorize, checkpoint: &changeRequestECABCheckpoint}
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
	stageKindECAB
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
		case approvalStageLabelCAB, approvalStageLabelLegacyAut:
			return stageKindCAB
		case approvalStageLabelECAB:
			return stageKindECAB
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

// approvalStageInfo reads stageID's label and ordinal position among the work
// item's stages (ordered by created_on, id -- the same ordering
// changeRequestApprovalStagesQuery uses).
func approvalStageInfo(ctx context.Context, q crQuerier, workItemID, stageID string) (approvalStageKind, error) {
	var label *string
	var pos int
	err := q.QueryRow(ctx, `
		SELECT ast.checkpoint_label,
		       (SELECT COUNT(*) FROM approval_stage earlier
		         WHERE earlier.work_item_id = $1
		           AND (earlier.created_on, earlier.id) < (ast.created_on, ast.id))
		FROM approval_stage ast WHERE ast.id = $2`, workItemID, stageID).Scan(&label, &pos)
	if err != nil {
		return stageKindOther, fmt.Errorf("read approval stage: %w", err)
	}
	return classifyApprovalStage(label, pos), nil
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
// INTERNAL one (Peer, CAB, ECAB, Review) whose approvers must be internal
// users. The customer stages and unclassified (ServiceNow-synced) stages are
// not: customer contacts are external by nature.
func stageKindNeedsInternalApprover(kind approvalStageKind) bool {
	switch kind {
	case stageKindPeer, stageKindCAB, stageKindECAB, stageKindReview:
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
	case stageKindECAB:
		return approvalStageLabelECAB
	case stageKindReview:
		return approvalStageLabelReview
	case stageKindCustomerApproval:
		return approvalStageLabelCustomerApproval
	case stageKindCustomerReview:
		return approvalStageLabelCustomerReview
	}
	return "this"
}

// noInternalMembersMessage is the ValidationError for a CAB / ECAB / Review
// pool that has members but none who is an active internal user, so the
// operator can see that the people are there and why they do not count.
func noInternalMembersMessage(poolDescription, label string) string {
	return fmt.Sprintf(
		"%s has no active internal (WSO2) members to provision as %s approvers: external/customer users and inactive users cannot approve an internal stage",
		poolDescription, label)
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
	eligible := func(members []string) ([]string, bool, error) {
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
		return kept, requestable, nil
	}

	if assignedTeamID != nil && *assignedTeamID != "" {
		members, err := groupMemberIDs(ctx, q, *assignedTeamID)
		if err != nil {
			return approvalPool{}, err
		}
		kept, ok, err := eligible(members)
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
		kept, ok, err := eligible(members)
		if err != nil {
			return approvalPool{}, err
		}
		if ok {
			return approvalPool{groupID: gid, members: kept}, nil
		}
	}
	return approvalPool{}, &apierror.ValidationError{Msg: fmt.Sprintf(
		"no eligible peer approvers: the assigned group has no active internal members other than the change's creator (external/customer users cannot approve), and the %q group has none either",
		domain.PeerApprovalFallbackGroupName)}
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
			return approvalPool{}, &apierror.ValidationError{Msg: fmt.Sprintf("the %q group does not exist, so a %s stage cannot be provisioned", cp.GroupName, cp.Label)}
		}
		if len(members) == 0 {
			return approvalPool{}, &apierror.ValidationError{Msg: fmt.Sprintf("the %q group has no members to provision as %s approvers", cp.GroupName, cp.Label)}
		}
		if members, err = onlyInternalApprovers(ctx, q, members); err != nil {
			return approvalPool{}, err
		}
		if len(members) == 0 {
			return approvalPool{}, &apierror.ValidationError{Msg: noInternalMembersMessage(fmt.Sprintf("the %q group", cp.GroupName), cp.Label)}
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
		if members, err = onlyInternalApprovers(ctx, q, members); err != nil {
			return approvalPool{}, err
		}
		if len(members) == 0 {
			return approvalPool{}, &apierror.ValidationError{Msg: noInternalMembersMessage("the assigned team", cp.Label)}
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
//     ECAB, Review), even if a row for them exists (a row can predate the
//     rule, or the user's type can change after provisioning). The customer
//     stages are not subject to it.
//
// stageKind is the kind of the stage the caller's pending row belongs to; pass
// stageKindOther when it is not known.
func approverDecisionBlock(ctx context.Context, q crQuerier, userID string, creatorIDs map[string]bool, kind approvalStageKind) error {
	if creatorIDs[strings.ToLower(userID)] {
		return &apierror.ForbiddenError{Msg: "the creator of a change request cannot approve it"}
	}
	if stageKindNeedsInternalApprover(kind) {
		internal, err := internalApproverIDs(ctx, q, []string{userID})
		if err != nil {
			return err
		}
		if !internal[strings.ToLower(userID)] {
			return &apierror.ForbiddenError{Msg: fmt.Sprintf(
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
}

// lockChangeRequestGateSnapshot reads (and locks, FOR UPDATE) the fields the
// customer gates depend on. Locking keeps a concurrent approval decision
// (which takes the same lock) from moving the change past a gate between this
// read and the write that depends on it.
func lockChangeRequestGateSnapshot(ctx context.Context, tx pgx.Tx, id string) (changeRequestGateSnapshot, error) {
	var state, model *string
	var snap changeRequestGateSnapshot
	err := tx.QueryRow(ctx,
		`SELECT state::text, change_model::text, customer_approval_required, customer_review_required
		 FROM change_request WHERE id = $1 FOR UPDATE`, id,
	).Scan(&state, &model, &snap.approvalRequired, &snap.reviewRequired)
	if errors.Is(err, pgx.ErrNoRows) {
		return snap, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return snap, fmt.Errorf("patch change request: read approval gates: %w", err)
	}
	snap.state = strings.ToUpper(stringOrEmpty(state))
	snap.model = strings.ToUpper(stringOrEmpty(model))
	return snap, nil
}

// requestApprovalDestination is the state Request Approval writes: the flow's
// own (Assess / Authorize / Scheduled), except that a flow with no internal
// approval to wait for (Standard) goes to Customer Approval instead of
// Scheduled when the customer's approval is required. Normal and Emergency
// reach the customer gate later, when CAB / ECAB approves
// (approvalGateTarget).
func requestApprovalDestination(flow changeRequestFlow, customerApprovalRequired bool) domain.ChangeRequestState {
	if flow.checkpoint == nil && flow.requestState == domain.ChangeRequestStateScheduled && customerApprovalRequired {
		return domain.ChangeRequestStateCustomerApproval
	}
	return flow.requestState
}

// approvalGateTarget is the upper-case state a CAB / ECAB approval moves the
// change to: Customer Approval when the customer's approval is required,
// Scheduled otherwise.
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

// validateCustomerGateEdits refuses an edit of customer_approval_required /
// customer_review_required once the gate it controls has been passed. A write
// of the value already stored is a no-op and is always accepted, so a client
// that resends the whole form is not punished for fields it did not touch.
func validateCustomerGateEdits(snap changeRequestGateSnapshot, approvalRequired, reviewRequired *bool) error {
	state := strings.ToLower(snap.state)
	if state == "" {
		state = "new"
	}
	if approvalRequired != nil && *approvalRequired != snap.approvalRequired && !approvalRequirementEditable(snap.state) {
		return &apierror.ValidationError{Msg: fmt.Sprintf(
			"customerApprovalRequired can no longer be changed: the change request has already passed the approval stage (current state: %s)", state)}
	}
	if reviewRequired != nil && *reviewRequired != snap.reviewRequired && !reviewRequirementEditable(snap.state) {
		return &apierror.ValidationError{Msg: fmt.Sprintf(
			"customerReviewRequired can no longer be changed: the change request has already left the review stage (current state: %s)", state)}
	}
	return nil
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

// withoutManualCustomerOutcome drops the manual way out of a customer state
// from legalNextStates while a customer stage is live for it: "scheduled"
// (Record customer approval) from Customer Approval, "closed" (Close) and
// "rollback" (the failed review) from Customer Review. Only Cancel is left;
// the decision comes from the approval (a member rejecting the review rolls
// the change back).
func withoutManualCustomerOutcome(state *string, nexts []string, liveStage bool) []string {
	if !liveStage || state == nil || nexts == nil {
		return nexts
	}
	spec := customerStageSpecForState(strings.ToUpper(*state))
	if spec == nil {
		return nexts
	}
	manual := strings.ToLower(spec.approvedState)
	rejected := strings.ToLower(spec.rejectedState)
	out := make([]string, 0, len(nexts))
	for _, n := range nexts {
		// Cancel is the one manual way out that stays: rejectedState for
		// Customer Approval IS canceled, hence the explicit guard.
		if n != manual && (n != rejected || n == string(domain.ChangeRequestStateCanceled)) {
			out = append(out, n)
		}
	}
	return out
}

// customerStageManualRefusal is the 400 for a manual PATCH of the customer
// state's outcome ({state: scheduled} / {state: closed}) while the customer
// group's approval request is pending.
func customerStageManualRefusal(target string, spec *customerStageSpec, live *liveCustomerStage) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf(
		"state %q cannot be set manually: the customer's %s has been requested from the customer group (the registered contacts of the change request's project) and is given by one of them approving or rejecting it in the change request's approvals (POST /change-requests/{id}/approvals/decision)",
		target, spec.what)}
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
// resulting window must not end before it starts.
func checkRescheduleWindow(ctx context.Context, tx pgx.Tx, id string, start, end *string) error {
	var startChanged, endChanged, inverted bool
	err := tx.QueryRow(ctx, `
		SELECT COALESCE($1::text::timestamptz IS DISTINCT FROM start_on, false) AND $1::text IS NOT NULL,
		       COALESCE($2::text::timestamptz IS DISTINCT FROM end_on, false) AND $2::text IS NOT NULL,
		       COALESCE(COALESCE($1::text::timestamptz, start_on) > COALESCE($2::text::timestamptz, end_on), false)
		FROM change_request WHERE id = $3`, start, end, id).Scan(&startChanged, &endChanged, &inverted)
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
	return nil
}

// provisionReauthorizationStage opens a FRESH stage for a checkpoint that has
// already run (CAB / ECAB after a Re-schedule): the same group, the same
// creator exclusion, a new approval_stage row (the earlier stage stays as a
// record). Idempotent: a stage of this label that still has a requested
// approver is left alone. Unlike provisionApprovalStage it does not test the
// checkpoint's ordinal position -- repeating a checkpoint is the point. A group
// with nobody eligible is a ValidationError, so a change is never re-scheduled
// into an approval nobody can give.
func provisionReauthorizationStage(ctx context.Context, tx pgx.Tx, workItemID, actorEmail string, cp changeRequestApprovalCheckpoint) error {
	if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
		return fmt.Errorf("re-schedule: escalate identity: %w", err)
	}
	var live bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM approval_stage ast
		                 WHERE ast.work_item_id = $1 AND ast.checkpoint_label = $2
		                   AND EXISTS (SELECT 1 FROM approval_stage_approver asa WHERE asa.stage_id = ast.id AND asa.state = 'REQUESTED'))`,
		workItemID, cp.Label).Scan(&live); err != nil {
		return fmt.Errorf("re-schedule: check live %s stage: %w", cp.Label, err)
	}
	if live {
		return nil
	}
	creatorIDs, err := changeRequestCreatorUserIDs(ctx, tx, workItemID)
	if err != nil {
		return fmt.Errorf("re-schedule: %w", err)
	}
	pool, err := resolveApprovalPool(ctx, tx, cp, nil, creatorIDs)
	if err != nil {
		return err
	}
	return insertApprovalStage(ctx, tx, workItemID, actorEmail, cp.Label, pool, creatorIDs)
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
// of the given kind can be decided: Peer in Assess, CAB / ECAB in Authorize,
// Review in Review, Customer Approval in Customer Approval, Customer Review in
// Customer Review. "" for a stage of unknown kind (stageKindOther -- a
// ServiceNow-synced stage with no recognisable label): it is not tied to any
// state and is never guarded.
func approvalStageDecidableState(kind approvalStageKind) string {
	switch kind {
	case stageKindPeer:
		return crStateAssess
	case stageKindCAB, stageKindECAB:
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
	return &apierror.ConflictError{Msg: fmt.Sprintf(
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
//     Review, the customer's once it was re-scheduled back to Authorize.
//
// The stages stay as a record; only the approver rows move to `cancelled`
// (updated_by = actorEmail, like every other cancel helper). A stage is
// classified exactly as classifyApprovalStage does (checkpoint_label first,
// the historical positional fallback second); a stage of unknown kind and a
// NULL / unknown change request state are left alone.
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
	var state *string
	if err := tx.QueryRow(ctx, `SELECT state::text FROM change_request WHERE id = $1`, workItemID).Scan(&state); err != nil {
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
		SELECT ast.id::text, ast.checkpoint_label,
		       (SELECT COUNT(*) FROM approval_stage earlier
		         WHERE earlier.work_item_id = ast.work_item_id
		           AND (earlier.created_on, earlier.id) < (ast.created_on, ast.id))
		FROM approval_stage ast
		WHERE ast.work_item_id = $1
		  AND EXISTS (SELECT 1 FROM approval_stage_approver asa WHERE asa.stage_id = ast.id AND asa.state = 'REQUESTED')`,
		workItemID)
	if err != nil {
		return fmt.Errorf("reconcile approvers: list live stages: %w", err)
	}
	var stale []string
	for rows.Next() {
		var stageID string
		var label *string
		var pos int
		if err := rows.Scan(&stageID, &label, &pos); err != nil {
			rows.Close()
			return fmt.Errorf("reconcile approvers: scan stage: %w", err)
		}
		if approvalStageOutOfState(classifyApprovalStage(label, pos), current) {
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
//   - no project, or no eligible contact: no stage; the manual "record the
//     customer's approval" / close path stays available.
//
// The stage's assignment group is NULL (the Customer Group is not a "group"
// row); the approvals read response names it "Customer Group".
//
// Returns whether a stage was provisioned. Callers: the CAB / ECAB approval
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
	eligible := false
	for _, m := range members {
		if !creatorIDs[strings.ToLower(m)] {
			eligible = true
			break
		}
	}
	if !eligible {
		// The manual path stays open (see the doc comment): nothing is
		// stranded, but say why no stage appeared.
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
	return &apierror.ForbiddenError{Msg: fmt.Sprintf(
		"only members of the customer group (the registered contacts of this change request's project) can approve or reject the customer's %s of this change request", spec.what)}, nil
}
