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
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ChangeRequestRepository defines the persistence operations for the change
// request entity, split across work_item (migration 0021, fields common
// to every work_item type) and change_request (migration 0043, a
// shared-PK extension -- change_request.id IS work_item.id, same pattern
// as "case").
//
// ServiceID/ServiceOfferingID are backed by change_request.service_id/
// service_offering_id (migration 0046), FKs into service/service_offering
// (migrations 0044/0045).
//
// Type (domain.ChangeRequestType) is backed by change_request.change_model
// (migration 0056) -- NOT change_request.change_request_type, whose real
// enum values are INFRA/GENERAL, a completely different, unrelated
// classification. See changeRequestChangeModelToType/changeRequestTypeToChangeModel
// for the mapping, including the four ChangeRequestType values added
// alongside this that have no ServiceNow-data-source equivalent.
//
// CustomerGroupID is backed by change_request.customer_group_id (migration
// 000074, a FK into "group") -- written by CreateChangeRequestFromServiceNow
// and read back by GetChangeRequestByID as domain.ChangeRequest.CustomerGroup
// (see changeRequestDetailJoins/changeRequestDetailColumns).
//
// AssignedTeamID (distinct from CustomerGroupID) is backed by
// work_item.assignment_group_id (migration 0075, a FK into "group") and read
// back as domain.ChangeRequest.AssignedTeam via changeRequestFromJoins' own
// "group" ag join -- see changeRequestSelectColumns' own doc comment for the
// real bug this fixes (Assess could never be requested for any change
// request, since the frontend requires it set first). Writing it via
// PatchChangeRequestRequest.AssignedTeamID is now wired too (addWI sets
// assignment_group_id, same shape as AssignedEngineerID immediately above
// it; a 23503 on the FK is mapped to the friendly field name "assignedTeamId"
// via changeRequestPatchFKField, the same convention every other FK column
// on this PATCH already uses) -- csm-sync-service still separately populates
// it from ServiceNow's own Assignment group field for every work_item type
// the same way it does assigned_to_id, but a caller can now also set it
// directly through this API. It is also written at create time, from
// CreateChangeRequestRequest.GroupID (the create form's "Assignment group"
// picker -- the same /groups/search picker, labelled the same, that the edit
// dialog sends as PatchChangeRequestRequest.AssignedTeamID), by both
// CreateChangeRequest and CreateChangeRequestFromServiceNow. An earlier
// revision left GroupID unwritten on both Postgres create paths -- a real
// reported bug: the assignment group picked on the create form was silently
// dropped, so the new change request read back with no AssignedTeam (and
// could not be moved to Assess until someone set it again). Filtering search
// results by it (the parsed filter array's assignmentGroupId) is also still
// unwired -- see changeRequestWhereClause's own comment.
//
// OnHold/OnHoldReason are backed by change_request.is_on_hold/
// on_hold_reason (migration 0178) -- a concept this
// schema had no representation of at all before, despite a live
// investigation of the real ServiceNow "Change Request - Normal" workflow
// finding "on hold" threaded through nearly every stage transition. See
// PatchChangeRequestRequest.OnHold's own doc comment for the full write
// semantics and the state-change gate it drives (patchChangeRequestTx's
// own on-hold gate, right before the New->Assess team gate), and
// entity-service's own CLAUDE.md "Change requests" -> "On hold" for the
// SN-provenance writeup.
//
// The remaining fields on the request/response contract have no
// established mapping and are always left unset rather than guessed at:
// ConfigurationItemID (no CMDB table exists at all in this schema);
// ApprovedBy/ApprovedOn on domain.ChangeRequest (there is a
// summary change_request.approval enum but no approver/date columns);
// Labels (no join table). Deployments / Environments / DeploymentProducts ARE
// backed (migration 0191's change_request_deployment / _environment /
// _deployed_product, rules in change_request_links.go), as is Project
// (work_item.project_id), now written at create time too.
//
// LegalNextStates is populated -- see legalChangeRequestNextStates's own
// doc comment for how, and for the one branch it deliberately does not
// attempt (Authorize/Review's conditional detour through Customer
// Approval/Customer Review).
//
// CreateChangeRequest has no Postgres implementation at all: work_item.number
// has no DB default and no backing sequence anywhere in migrations/, the
// same blocker CaseRepository.CreateCase has -- see that method's own doc
// comment.
//
// GetChangeRequestApprovals/DecideChangeRequestApproval ARE implemented
// against approval_stage/approval_stage_approver (migration 0089), which
// mirror ServiceNow's generic sysapproval_group/sysapproval_approver tables
// -- see that migration's own comment. Stage label/approverType have no
// backing column (ServiceNow derives them from two hardcoded group sys_ids
// that were never synced into this schema as a lookup) and are instead
// derived positionally in buildChangeRequestApprovals; see that function's
// own doc comment for exactly what is and isn't replicated from
// ChangeRequestUtils.getChangeRequestApprovals.
//
// CreateChangeRequestFromServiceNow (below) is the exception, same as
// CaseRepository.CreateCaseFromServiceNow/IncidentRepository.CreateIncidentFromServiceNow:
// it backs DATA_SOURCE=postgres-servicenow-dual-write's SN-first change
// request creation, where identity comes from ServiceNow rather than being
// generated here.
type ChangeRequestRepository interface {
	// SearchChangeRequests returns a filtered, sorted, paginated slice of
	// change requests together with the total count of matching rows
	// before pagination. createdStartDate/createdEndDate/approval are the
	// already-parsed form of req.Filters.Filters (the generic field/op/values
	// array) -- parsing happens in the service layer
	// (service.ParseChangeRequestFieldFilters), never here, since this
	// layer must not import the service package. assignmentGroupIDs is
	// accepted for interface symmetry with that parsed result but still never
	// applied here: the column it would filter on (work_item.assignment_group_id)
	// does have a real join now (see this file's own package doc comment,
	// and changeRequestWhereClause's own comment), but wiring this specific
	// filter remains out of scope.
	SearchChangeRequests(ctx context.Context, req domain.SearchChangeRequestsRequest, createdStartDate, createdEndDate *time.Time, approval *string, assignmentGroupIDs []string) ([]domain.SearchChangeRequestView, int, error)
	// AggregateChangeRequests returns server-side aggregated counts of
	// change requests per value of groupBy, capped to the top maxGroups
	// buckets with the remainder folded into the returned OthersCount. The
	// parsed-filter parameters are the same as SearchChangeRequests'.
	AggregateChangeRequests(ctx context.Context, req domain.AggregateChangeRequestsRequest, groupBy string, maxGroups int, createdStartDate, createdEndDate *time.Time, approval *string) (domain.AggregateResponse, error)
	// GetChangeRequestByID returns the full detail of a single change
	// request by its UUID, or a NotFoundError if no matching row exists.
	GetChangeRequestByID(ctx context.Context, id string) (domain.ChangeRequest, error)
	// PatchChangeRequest applies req's non-nil fields to the change request
	// identified by id, using actorEmail as work_item.updated_by. Returns a
	// NotFoundError if id does not exist.
	PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest, actorEmail string) (domain.ChangeRequest, error)
	// CreateChangeRequest inserts a new change request row (both work_item
	// and change_request) for the plain-Postgres data source (no ServiceNow
	// at all) -- createChangeRequestPortalQuery's own doc comment has the
	// full field-by-field reasoning, which mirrors CreateChangeRequestFromServiceNow's
	// exactly except identity (id/number) is generated here via
	// gen_random_uuid()/next_portal_work_item_number() instead of being
	// supplied by a prior ServiceNow response, and createdBy is the calling
	// user's own resolved email rather than ServiceNow's echoed value.
	CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest, createdBy string) (domain.CreateChangeRequestResponse, error)
	// CreateChangeRequestFromServiceNow inserts a new change request row
	// (both work_item and change_request), for
	// DATA_SOURCE=postgres-servicenow-dual-write's SN-first change request
	// creation (see changeRequestService.createChangeRequestSNFirst's own doc
	// comment). Unlike CaseRepository.CreateCaseFromServiceNow, no wso2ID
	// parameter exists here: work_item.wso2_id is only required (by the
	// work_item_wso2_id_required_by_type CHECK constraint, migration 0021)
	// for CASE/SERVICE_REQUEST/ANNOUNCEMENT/ENGAGEMENT/
	// SECURITY_REPORT_ANALYSIS -- CHANGE_REQUEST is deliberately excluded
	// from that list (the same table's own inline comment: "change_request
	// work items have no wso2_id data"), and ServiceNow's own change-request
	// create response (snCreateChangeRequestResponse) has no equivalent
	// field to supply one from anyway. id/number/createdBy are exactly what
	// ServiceNow already returned for the change request it just created.
	// id must be a canonical UUID (sysidToUUID(sn sys_id)). Returns a
	// ValidationError if id is not a valid UUID, if req.Type has no
	// change_model equivalent, or if a row already exists for id/number
	// (unique violation) -- the latter should not happen in practice since
	// ServiceNow only just generated these, but is reported precisely
	// rather than as an opaque infrastructure error if it ever does.
	//
	// change_request.state is hardcoded to NEW, never req.State -- an
	// earlier revision of this method left it NULL entirely (see git
	// history), reasoned as: snCreateChangeRequestResponse carries no state
	// field at all, so unlike req.Category/Priority/Risk/Impact (plain
	// request-supplied values ServiceNow's create payload already forwards
	// verbatim and this method can echo back with equal confidence), the
	// state ServiceNow's workflow engine actually assigned was never
	// confirmed by the response -- writing req.State straight through would
	// risk recording a value ServiceNow silently overrode. That caution was
	// real but led to a worse bug: with no state at all, a freshly created
	// change request offered no promote action whatsoever (see
	// legalChangeRequestNextStates's nil case), not even the one every
	// change request always starts with. The org's own Change Management
	// process flow resolves the original uncertainty directly: creation
	// always begins at New unconditionally, with no branch or caller input
	// that changes that -- so New is not a guess at what ServiceNow decided,
	// it is the one value ServiceNow's real workflow always assigns on
	// create, confirmed independent of the response's own silence on the
	// question. req.State is accepted on this request type only because
	// PatchChangeRequestRequest shares its fields with CreateChangeRequestRequest;
	// it has no legal effect at creation and is intentionally ignored here.
	// See CreateProblemFromServiceNow's own doc comment for the contrasting
	// case, where the response DOES return a confirmed, identity-matching
	// state.
	//
	// Written beyond the core columns: the Customer Project (work_item.project_id),
	// deployments / environments / deployment products (the join tables, validated
	// and derived per change_request_links.go), category (all 13 values have an
	// enum label since migration 0191) and the Comment / WorkNote journal entries
	// (comment rows). Everything runs in one transaction. Deliberately NOT
	// applied: req.ConfigurationItemID (no CMDB table).
	CreateChangeRequestFromServiceNow(ctx context.Context, req domain.CreateChangeRequestRequest, id, number, createdBy string) (domain.CreateChangeRequestResponse, error)
	// ValidateChangeRequestLinks validates a customer-scope selection (project,
	// deployments, environments, deployment products) and returns it with
	// everything derived, without writing anything -- the pre-flight the
	// ServiceNow-first create runs before it calls ServiceNow. A
	// ValidationError names the offending field. See change_request_links.go.
	ValidateChangeRequestLinks(ctx context.Context, sel domain.ChangeRequestLinkSelection) (domain.ChangeRequestLinkSet, error)
	// GetChangeRequestLinkOptions backs POST /change-requests/link-options: the
	// project's active deployments and, for the deployments chosen so far, the
	// environments and deployment products that follow from them.
	GetChangeRequestLinkOptions(ctx context.Context, req domain.ChangeRequestLinkOptionsRequest) (domain.ChangeRequestLinkOptionsResponse, error)
	// GetChangeRequestApprovals returns every approval stage for the change
	// request identified by id (approval_stage rows with work_item_id = id,
	// ordered by created_on ascending) together with each stage's approvers
	// (approval_stage_approver, matched by stage_id). Stage label/approverType
	// are derived positionally in Go from this ordering -- see
	// buildChangeRequestApprovals' own doc comment. Returns an empty
	// domain.ChangeRequestApprovals{} (not a NotFoundError) if id has no
	// approval_stage rows: a change request legitimately has zero stages
	// before ServiceNow's workflow creates its first one, and this method
	// does not separately check work_item existence -- same "no rows is not
	// an error" convention as SearchChangeRequests.
	GetChangeRequestApprovals(ctx context.Context, id string) (domain.ChangeRequestApprovals, error)
	// DecideChangeRequestApproval flips the ONE approval_stage_approver row
	// matching work_item_id = id AND approver_user_id = approverUserID AND
	// state = 'REQUESTED' to the decision (the request-level "approved" /
	// "rejected", validated by the caller before this is reached, is stored
	// UPPER_SNAKE_CASE as "APPROVED" / "REJECTED"), stamping actorEmail as
	// updated_by, and returns that row's id. Returns a NotFoundError if no such row
	// exists -- covers id not existing, the caller having no approval on
	// this change request, and the caller's approval already being decided,
	// all in the one WHERE clause (mirrors ServiceNow's decideApproval
	// restriction that only the caller's own PENDING approval can be acted
	// on -- see sn_change_request_service.go's DecideChangeRequestApproval
	// doc comment). Also cascades change_request.state from Assess to
	// Authorize when this decision is the one that resolves that stage's own
	// approval (first-responder-wins, same quorum rule as
	// buildChangeRequestApprovals) -- see this method's own doc comment for
	// the full reasoning and its deliberately narrow scope.
	DecideChangeRequestApproval(ctx context.Context, id, approverUserID, decision, actorEmail string) (string, error)
}

type changeRequestRepo struct {
	db *Scoped
	// vis is the customer-visibility policy (change_request_visibility.go): who
	// of the customers may see which change request. The zero value is "no
	// cutover": every change request is legacy.
	vis CRVisibility
}

// NewChangeRequestRepository constructs a ChangeRequestRepository backed by the
// given connection pool. The optional CRVisibility is the customer-visibility
// policy (the strict-visibility cutover instant); omitted, every change request
// is legacy, which is what customers saw before the strict rule existed.
func NewChangeRequestRepository(db *Scoped, vis ...CRVisibility) ChangeRequestRepository {
	return &changeRequestRepo{db: db, vis: firstCRVisibility(vis)}
}

// changeRequestFromJoins is shared by every read method. LEFT joins
// throughout: a change request may exist with no project/deployment/
// deployed-product/assignee/originating-case link at all (those are only
// ever set later, via PatchChangeRequest -- CreateChangeRequestRequest has
// no project/case field whatsoever).
const changeRequestFromJoins = `
	FROM work_item wi
	JOIN change_request cr ON cr.id = wi.id
	LEFT JOIN project p ON p.id = wi.project_id
	LEFT JOIN deployment d ON d.id = wi.deployment_id
	LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
	LEFT JOIN product prod ON prod.id = wi.product_id
	LEFT JOIN service svc ON svc.id = cr.service_id
	LEFT JOIN service_offering so ON so.id = cr.service_offering_id
	LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
	LEFT JOIN work_item origin_case ON origin_case.id = wi.parent_id
	LEFT JOIN "group" ag ON ag.id = wi.assignment_group_id`

// ag (work_item.assignment_group_id, migration 0075) backs
// domain.ChangeRequest's AssignedTeam -- the same column incident_repo.go's
// own GetIncidentByID already joins and reads (as AssignmentGroup there; a
// different domain field name for the same underlying column, matching what
// the CSM Portal calls it for each entity type). This file used to leave it
// entirely unread ("no confirmed mapping" -- see this file's own package doc
// comment history): a real, reported bug -- Assess could never be requested
// for *any* change request through the portal, since the frontend's own
// TARGET_BLOCKED_REASON requires assignedTeam to be set first, and it could
// never come back non-null. Confirmed live against a real change request
// with a genuine ServiceNow Assignment group ("Devops"): its Assigned
// engineer synced and displayed correctly (a column read the same way), but
// Assigned team always showed empty -- proving csm-sync-service already
// populates work_item.assignment_group_id for change requests the same way
// it does for every other work_item type; this was purely a read-side gap,
// not a missing-data one, so no create/patch write-path changes are needed
// alongside this join.
//
// start_on / end_on are read through isfinite(): a row holding Postgres'
// 'infinity' / '-infinity' (nothing the API writes any more, see
// change_request_window.go, but a row can get there by other doors) reads as no
// planned time at all, instead of failing the scan into a time.Time and with it
// the whole detail read or list the row is part of.
const changeRequestSelectColumns = `
	wi.id, wi.number, wi.subject, wi.description,
	p.id, p.name,
	origin_case.id, origin_case.number,
	d.id, d.name,
	dp.id, dp.name,
	prod.id, prod.name,
	svc.id, svc.name,
	so.id, so.name,
	ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
	CASE WHEN isfinite(cr.start_on) THEN cr.start_on END, CASE WHEN isfinite(cr.end_on) THEN cr.end_on END,
	cr.impact::TEXT, cr.state::TEXT, cr.change_model::TEXT,
	wi.created_on, wi.updated_on,
	ag.id, ag.name,
	cr.is_on_hold, cr.on_hold_reason`

// changeRequestChangeModelToType/changeRequestTypeToChangeModel map between
// change_request.change_model's real enum labels (migration 0056) and
// domain.ChangeRequestType. Unlike change_request.change_request_type
// (INFRA/GENERAL -- a genuinely different, unrelated classification, see
// this file's own package doc comment), change_model's vocabulary overlaps
// domain.ChangeRequestType's existing values enough (AZURE/EMERGENCY/
// NORMAL/STANDARD case-fold directly) that this is its real backing column
// -- the four that don't already exist as domain values
// (CHANGE_REGISTRATION/CLOUD_INFRASTRUCTURE/INFRA/UNAUTHORIZED_CHANGE) were
// added as new ChangeRequestType constants rather than dropped, since they
// are genuine ServiceNow change-model choices, not noise.
var changeRequestChangeModelToType = map[string]domain.ChangeRequestType{
	"AZURE":                domain.ChangeRequestTypeAzure,
	"CHANGE_REGISTRATION":  domain.ChangeRequestTypeChangeRegistration,
	"CLOUD_INFRASTRUCTURE": domain.ChangeRequestTypeCloudInfrastructure,
	"EMERGENCY":            domain.ChangeRequestTypeEmergency,
	"INFRA":                domain.ChangeRequestTypeInfra,
	"NORMAL":               domain.ChangeRequestTypeNormal,
	"STANDARD":             domain.ChangeRequestTypeStandard,
	"UNAUTHORIZED_CHANGE":  domain.ChangeRequestTypeUnauthorizedChange,
}

var changeRequestTypeToChangeModel = func() map[domain.ChangeRequestType]string {
	m := make(map[domain.ChangeRequestType]string, len(changeRequestChangeModelToType))
	for enumValue, t := range changeRequestChangeModelToType {
		m[t] = enumValue
	}
	return m
}()

// ValidateCreateChangeRequestType enforces that a change request is created
// with a type, and that it is one of the three creatable ones: standard,
// normal or emergency. The type decides the whole approval flow (Standard
// needs none, Normal needs peer then CAB approval, Emergency needs CAB
// approval only), so a change without one cannot be routed. Exported so every
// create path -- both service layers and both repository creates -- applies the
// identical rule and message.
func ValidateCreateChangeRequestType(t *domain.ChangeRequestType) error {
	if t == nil || *t == "" {
		return &apierror.ValidationError{Msg: "type is required: a change request must be one of standard, normal or emergency"}
	}
	if !domain.IsCreatableChangeRequestType(*t) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("type %q is not allowed: a change request must be one of standard, normal or emergency", *t)}
	}
	return nil
}

func ptrString(s string) *string { return &s }

// ChangeRequestTypeSupported reports whether t has a change_model label,
// i.e. whether CreateChangeRequestFromServiceNow can persist it. Exported so
// the service layer can reject an unsupported type before, not after, the
// ServiceNow-first create -- see createChangeRequestSNFirst's own comment.
func ChangeRequestTypeSupported(t domain.ChangeRequestType) bool {
	_, ok := changeRequestTypeToChangeModel[t]
	return ok
}

// changeRequestForwardNextStates is the forward move(s) a HUMAN is offered out
// of each non-terminal change_request state (domain.ChangeRequest.
// LegalNextStates, which the webapp renders as-is) -- and the forward moves a
// staff PATCH {state} is accepted for: the same table drives both
// (legalChangeRequestNextStates renders it, checkStaffStateRequest enforces it;
// change_request_transitions.go has the whole graph). Values are
// domain.ChangeRequestState so a typo here is a compile error.
//
// The graph was originally read off real change_requests on the live
// wso2.service-now.com instance. It has since been reshaped for the Request
// Approval / CAB flow (change_request_approval_flow.go has the per-type flows)
// and the two creation-form checkboxes, Customer Approval and Customer Review
// (change_request.customer_approval_required / customer_review_required):
//
//   - New -> Assess is the "Request Approval" action, offered for every type.
//   - Assess and Authorize are approval waits with NO move for staff but Cancel:
//     a change leaves Assess through the peer approval and Authorize through the
//     CAB approval, i.e. through DecideChangeRequestApproval's cascade.
//     Assess -> Authorize used to be listed here although patchChangeRequestTx has
//     always refused it ("cannot be set manually"): listing it offered an edge
//     the service would not take, so it is gone and the table is exactly what
//     the PATCH accepts.
//   - Scheduled is never offered as a target, from any state. There is no
//     "Schedule" action: a change reaches Scheduled automatically (CAB
//     approval, or Request Approval on a Standard change) unless Customer
//     Approval is required, in which case those same events move it to
//     Customer Approval instead, and from there only the CUSTOMER's approval
//     (given in the Customer Portal) schedules it. patchChangeRequestTx rejects
//     a manual {state: "scheduled"} from every other state.
//   - Review offers Closed -- or, when customer_review_required is set,
//     Customer Review instead (legalChangeRequestNextStates applies that
//     branch; the map holds both, the PATCH picks one by the same flag). Customer
//     Review offers no forward move: only the CUSTOMER's review (given in the
//     Customer Portal) closes it, so staff are left with Rollback and Cancel there.
//   - Customer Approval and Customer Review are customer states: the change
//     leaves them through the customer's own answer and nothing else, so no
//     staff action may record that answer (patchChangeRequestTx and
//     refuseStaffExitFromCustomerState). Staff keep Cancel everywhere, Rollback
//     out of Customer Review, and, out of Customer Approval, Authorize, which
//     means Re-schedule / Propose a different time: the wire name of the Time
//     Change loop, which does NOT move the state (the planned time changed, so the
//     customer is asked again; nothing goes through internal approval again --
//     patchChangeRequestTx documents the contract). It is the one state from which
//     a manual {state: "authorize"} is accepted. WSO2's acceptance of a time the
//     customer proposed is not a state either: it is its own request
//     (confirmCustomerUpdatedDate).
//   - Rollback is the failed-review off-ramp and is offered from exactly two
//     states, Review (the internal review failed) and Customer Review (the
//     customer's review failed): changeRequestRollbackFrom. It is not a
//     forward move, so it is not in this map.
//   - Closed, Canceled and Rollback have no entry: they are final, nothing moves
//     a change out of them by PATCH (checkStaffStateRequest).
var changeRequestForwardNextStates = map[domain.ChangeRequestState][]domain.ChangeRequestState{
	// New's one human action is Request Approval, always sent as
	// {state: "assess"}; where it actually lands depends on the change's type
	// and on customer_approval_required (see change_request_approval_flow.go).
	domain.ChangeRequestStateNew: {domain.ChangeRequestStateAssess},
	// Assess and Authorize are the two approval waits, with an empty (non-nil)
	// entry each, not a missing one: legalChangeRequestNextStates still offers
	// Cancel for a state that has an entry, and nothing for one that does not.
	// Assess leaves through the peer approval cascade (to Authorize), Authorize
	// through CAB approval (to Scheduled, or Customer Approval): never by a
	// human PATCH.
	domain.ChangeRequestStateAssess:    {},
	domain.ChangeRequestStateAuthorize: {},
	// Customer Approval is the customer's step: the customer's own approval (the
	// Customer Portal) schedules the change, their rejection cancels it, and no
	// staff action stands in for either, so "scheduled" is NOT offered here.
	// "authorize" is Re-schedule / Propose a different time (the process diagram's
	// Time Change loop): the wire name, not a destination -- the state does not move
	// (planStaffTimeResponse). WSO2's acceptance of a time the customer proposed is a
	// request of its own (confirmCustomerUpdatedDate), not a state. Cancel is added by
	// legalChangeRequestNextStates.
	domain.ChangeRequestStateCustomerApproval: {domain.ChangeRequestStateAuthorize},
	domain.ChangeRequestStateScheduled:        {domain.ChangeRequestStateImplement},
	domain.ChangeRequestStateImplement:        {domain.ChangeRequestStateReview},
	// Review is Closed directly unless the customer's review is required, in
	// which case it is Customer Review instead: the entry lists both (what a PATCH
	// may name from Review), legalChangeRequestNextStates keeps the one the flag
	// picks.
	domain.ChangeRequestStateReview: {domain.ChangeRequestStateClosed, domain.ChangeRequestStateCustomerReview},
	// Customer Review has an empty (non-nil) entry on purpose: the customer's own
	// review closes the change (or rolls it back), so "closed" is NOT offered to
	// staff. Rollback and Cancel are added by legalChangeRequestNextStates.
	domain.ChangeRequestStateCustomerReview: {},
}

// legalChangeRequestNextStates computes domain.ChangeRequest.LegalNextStates
// for the Postgres data source, which (unlike ServiceNow) has no workflow
// engine of its own to compute this dynamically -- see
// changeRequestForwardNextStates' own doc comment for how this graph was
// derived.
//
// customerReviewRequired (change_request.customer_review_required) is the one
// input beyond the state: a Review that requires the customer's review offers
// Customer Review INSTEAD of Closed; one that does not offers Closed directly.
// Customer Review itself offers no forward move at all (only the customer's
// review closes it).
//
// "rollback" is offered, right after the forward move and before "canceled",
// from exactly two states -- Review and Customer Review (the review failed;
// changeRequestRollbackFrom). It is the failed-review branch of the process
// diagram, so it is offered whether or not customer review is required.
//
// The result for the two customer states is therefore, exactly:
// customer_approval [authorize, canceled] and customer_review [rollback,
// canceled] -- never "scheduled" or "closed", which only the customer's own
// answer can reach. (GetChangeRequestByID then takes "rollback" off
// customer_review while the customer group's review request is pending.)
//
// "canceled" is offered alongside the forward move(s) from every
// non-terminal state: the Cancel Change action was available on every
// reachable state checked live, with no exception found. Rollback/Closed/
// Canceled are terminal -- nil, matching ServiceNow's own "no
// legalNextStates at all" answer for a record with no legal forward move.
func legalChangeRequestNextStates(state *string, customerReviewRequired bool) []string {
	if state == nil {
		return nil
	}
	st := domain.ChangeRequestState(*state)
	targets := changeRequestStaffTargets(st)
	if targets == nil {
		return nil
	}
	result := make([]string, 0, len(targets))
	for _, next := range targets {
		// Review lists Closed and Customer Review: the flag picks one.
		if st == domain.ChangeRequestStateReview &&
			((next == domain.ChangeRequestStateCustomerReview && !customerReviewRequired) ||
				(next == domain.ChangeRequestStateClosed && customerReviewRequired)) {
			continue
		}
		result = append(result, string(next))
	}
	return result
}

// changeRequestRollbackFrom is the set of states a change can be rolled back
// from by hand: the two review states. A rejected Customer Review also lands
// in Rollback, through the customer group's approval (customerReviewStageSpec),
// which is a different path and not governed by this set.
var changeRequestRollbackFrom = map[domain.ChangeRequestState]bool{
	domain.ChangeRequestStateReview:         true,
	domain.ChangeRequestStateCustomerReview: true,
}

// scanChangeRequestView scans changeRequestSelectColumns into a
// SearchChangeRequestView. Duration is never set here -- see this file's
// own package doc comment for why (no confirmed rendering format).
func scanChangeRequestView(row interface{ Scan(...any) error }) (domain.SearchChangeRequestView, error) {
	var v domain.SearchChangeRequestView
	var (
		projectID, projectName *string
		caseID, caseNumber     *string
		depID, depName         *string
		dpID, dpName           *string
		prodID, prodName       *string
		svcID, svcName         *string
		soID, soName           *string
		aeID, aeName           *string
		agID, agName           *string
		startOn, endOn         *time.Time
		impact, state          *string
		changeModel            *string
		createdOn, updatedOn   time.Time
		isOnHold               *bool
		onHoldReason           *string
	)
	err := row.Scan(
		&v.ID, &v.Number, &v.Subject, &v.Description,
		&projectID, &projectName,
		&caseID, &caseNumber,
		&depID, &depName,
		&dpID, &dpName,
		&prodID, &prodName,
		&svcID, &svcName,
		&soID, &soName,
		&aeID, &aeName,
		&startOn, &endOn, &impact, &state, &changeModel,
		&createdOn, &updatedOn,
		&agID, &agName,
		&isOnHold, &onHoldReason,
	)
	if err != nil {
		return domain.SearchChangeRequestView{}, err
	}
	if projectID != nil {
		v.Project = domain.EntityRef{ID: *projectID, Name: stringOrEmpty(projectName)}
	}
	if caseID != nil {
		v.Case = &domain.EntityRef{ID: *caseID, Name: stringOrEmpty(caseNumber)}
	}
	if depID != nil {
		v.Deployment = &domain.EntityRef{ID: *depID, Name: stringOrEmpty(depName)}
	}
	if dpID != nil {
		v.DeployedProduct = &domain.EntityRef{ID: *dpID, Name: stringOrEmpty(dpName)}
	}
	if prodID != nil {
		v.Product = &domain.EntityRef{ID: *prodID, Name: stringOrEmpty(prodName)}
	}
	if svcID != nil {
		v.Service = &domain.EntityRef{ID: *svcID, Name: stringOrEmpty(svcName)}
	}
	if soID != nil {
		v.ServiceOffering = &domain.EntityRef{ID: *soID, Name: stringOrEmpty(soName)}
	}
	if aeID != nil {
		v.AssignedEngineer = &domain.EntityRef{ID: *aeID, Name: stringOrEmpty(aeName)}
	}
	if agID != nil {
		v.AssignedTeam = &domain.EntityRef{ID: *agID, Name: stringOrEmpty(agName)}
	}
	if startOn != nil {
		v.PlannedStartOn = fmtInstantPtr(startOn)
	}
	if endOn != nil {
		v.PlannedEndOn = fmtInstantPtr(endOn)
	}
	if impact != nil {
		lower := strings.ToLower(*impact)
		v.Impact = &lower
	}
	if state != nil {
		lower := strings.ToLower(*state)
		v.State = &lower
	}
	if changeModel != nil {
		if t, ok := changeRequestChangeModelToType[*changeModel]; ok {
			s := string(t)
			v.Type = &s
		}
	}
	v.OnHold = isOnHold
	v.OnHoldReason = onHoldReason
	v.CreatedOn = createdOn.UTC().Format(time.RFC3339)
	v.UpdatedOn = updatedOn.UTC().Format(time.RFC3339)
	return v, nil
}

// changeRequestWhereClause builds the shared WHERE clause + args for
// SearchChangeRequests and AggregateChangeRequests, so the two can't drift
// out of sync on which rows a given filter set matches.
func changeRequestWhereClause(f domain.SearchChangeRequestsFilters, createdStartDate, createdEndDate *time.Time, approval *string) (string, []any) {
	where := "WHERE wi.type = 'CHANGE_REQUEST'"
	args := []any{}
	argIdx := 1

	add := func(clause string, val any) {
		where += fmt.Sprintf(" AND "+clause, argIdx)
		args = append(args, val)
		argIdx++
	}

	if len(f.ProjectIDs) > 0 {
		add("wi.project_id = ANY($%d::uuid[])", f.ProjectIDs)
	}
	if len(f.States) > 0 {
		states := make([]string, len(f.States))
		for i, s := range f.States {
			states[i] = strings.ToUpper(string(s))
		}
		add("cr.state = ANY($%d::change_request_state_enum[])", states)
	}
	if len(f.Impacts) > 0 {
		impacts := make([]string, len(f.Impacts))
		for i, imp := range f.Impacts {
			impacts[i] = strings.ToUpper(string(imp))
		}
		add("cr.impact = ANY($%d::change_request_impact_enum[])", impacts)
	}
	if f.ClosedStartDate != nil {
		add("cr.closed_on >= $%d", *f.ClosedStartDate)
	}
	if f.ClosedEndDate != nil {
		add("cr.closed_on <= $%d", *f.ClosedEndDate)
	}
	if f.Number != nil && *f.Number != "" {
		add("wi.number = $%d", *f.Number)
	}
	if f.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (wi.subject ILIKE $%d ESCAPE '\\' OR wi.number ILIKE $%d ESCAPE '\\')", argIdx, argIdx)
		args = append(args, pattern)
		argIdx++
	}
	if createdStartDate != nil {
		add("wi.created_on >= $%d", *createdStartDate)
	}
	if createdEndDate != nil {
		add("wi.created_on <= $%d", *createdEndDate)
	}
	if approval != nil {
		add("cr.approval = $%d::change_request_approval_enum", changeRequestApprovalEnum(*approval))
	}
	// The parsed filter array's assignmentGroupId has a real backing column
	// now (work_item.assignment_group_id, read as AssignedTeam -- see this
	// file's own package doc comment), but filtering search results by it is
	// still not wired here; out of scope for the read-side fix that added
	// the column's own join.

	return where, args
}

// changeRequestApprovalEnum maps ServiceNow's raw task.approval value
// ("not requested"/"requested"/"approved"/"rejected") to the real
// change_request_approval_enum label.
func changeRequestApprovalEnum(snApproval string) string {
	return strings.ToUpper(strings.ReplaceAll(snApproval, " ", "_"))
}

// SearchChangeRequests implements ChangeRequestRepository.
func (r *changeRequestRepo) SearchChangeRequests(ctx context.Context, req domain.SearchChangeRequestsRequest, createdStartDate, createdEndDate *time.Time, approval *string, _ []string) ([]domain.SearchChangeRequestView, int, error) {
	where, args := changeRequestWhereClause(req.Filters, createdStartDate, createdEndDate, approval)
	// Planner hint for external callers only (see viewerProjectHint): without
	// it, making the policy helpers parallel safe lets Postgres pick a parallel
	// scan of all of work_item for a customer with a handful of change requests.
	// RLS remains the authorization boundary for project membership.
	where += viewerProjectHintFor(ctx, "wi")
	// What a customer may see of those is decided here, in SQL, by the one
	// visibility rule (change_request_visibility.go): designated to them, or
	// legacy. The same fragment narrows the count, the page and every other
	// customer-reachable read, so they cannot disagree.
	visSQL, args := r.vis.andClause(ctx, "wi", "cr", args)
	where += visSQL

	sortCol := "wi.created_on"
	if req.SortBy.Field == domain.ChangeRequestSortFieldUpdatedOn {
		sortCol = "wi.updated_on"
	}
	sortDir := "DESC"
	if req.SortBy.Order == domain.ChangeRequestSortOrderAsc {
		sortDir = "ASC"
	}

	countQuery := "SELECT COUNT(*) " + changeRequestFromJoins + " " + where
	dataQuery := fmt.Sprintf("SELECT %s %s %s ORDER BY %s %s, wi.id LIMIT $%d OFFSET $%d",
		changeRequestSelectColumns, changeRequestFromJoins, where, sortCol, sortDir, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var views []domain.SearchChangeRequestView

	eg, egCtx := errgroup.WithContext(ctx)

	// SkipTotal: the caller does not show a total (global search shows a handful
	// of hits), so the COUNT is not run at all -- it is as costly as the page
	// query and holds a second pool connection while it runs.
	if req.SkipTotal {
		total = domain.TotalNotComputed
	} else {
		eg.Go(func() error {
			if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
				return fmt.Errorf("count change requests: %w", err)
			}
			return nil
		})
	}

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query change requests: %w", err)
		}
		defer rows.Close()

		out := make([]domain.SearchChangeRequestView, 0, req.Pagination.Limit)
		for rows.Next() {
			v, err := scanChangeRequestView(rows)
			if err != nil {
				return fmt.Errorf("scan change request: %w", err)
			}
			out = append(out, v)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate change requests: %w", err)
		}
		views = out
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return views, total, nil
}

// changeRequestAggregateColumns maps an AggregateChangeRequestsRequest.GroupBy
// value to the real column/cast used to group by it.
var changeRequestAggregateColumns = map[string]string{
	"state":    "cr.state::TEXT",
	"impact":   "cr.impact::TEXT",
	"category": "cr.category::TEXT",
	"risk":     "cr.risk::TEXT",
}

// AggregateChangeRequests implements ChangeRequestRepository.
func (r *changeRequestRepo) AggregateChangeRequests(ctx context.Context, req domain.AggregateChangeRequestsRequest, groupBy string, maxGroups int, createdStartDate, createdEndDate *time.Time, approval *string) (domain.AggregateResponse, error) {
	col, ok := changeRequestAggregateColumns[groupBy]
	if !ok {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + groupBy}
	}

	where, args := changeRequestWhereClause(req.Filters, createdStartDate, createdEndDate, approval)
	where += viewerProjectHintFor(ctx, "wi") // planner hint, external callers only; see SearchChangeRequests
	visSQL, args := r.vis.andClause(ctx, "wi", "cr", args)
	where += visSQL // the one visibility rule; see SearchChangeRequests

	query := fmt.Sprintf(`
		SELECT %s AS bucket, COUNT(*) AS bucket_count
		%s %s AND %s IS NOT NULL
		GROUP BY %s
		ORDER BY bucket_count DESC, bucket`, col, changeRequestFromJoins, where, col, col)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("aggregate change requests: %w", err)
	}
	defer rows.Close()

	var buckets []domain.AggregateBucket
	var totalRecords int
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return domain.AggregateResponse{}, fmt.Errorf("scan change request bucket: %w", err)
		}
		lowerKey := strings.ToLower(key)
		buckets = append(buckets, domain.AggregateBucket{Key: lowerKey, Label: lowerKey, Count: count})
		totalRecords += count
	}
	if err := rows.Err(); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("iterate change request buckets: %w", err)
	}

	if maxGroups <= 0 || maxGroups >= len(buckets) {
		return domain.AggregateResponse{Groups: buckets, TotalRecords: totalRecords}, nil
	}

	othersCount := 0
	for _, b := range buckets[maxGroups:] {
		othersCount += b.Count
	}
	return domain.AggregateResponse{
		Groups:       buckets[:maxGroups],
		OthersCount:  othersCount,
		TotalRecords: totalRecords,
	}, nil
}

// changeRequestDetailColumns extends changeRequestSelectColumns with the
// fields ChangeRequest carries beyond SearchChangeRequestView. The three timestamps in
// the second block (customer_updated_on, work_start_on, work_end_on) are read through
// isfinite(), like the planned window: a row holding Postgres' infinity (a sync or a hand
// edit can leave one) reads as none instead of failing the scan, and with it the whole
// detail read.
//
// The second block (implementation_plan through git_reference) is domain.
// ChangeRequest's own "field-parity additions" (see that struct's doc
// comment, Groups B/C1/C2/D) -- real change_request columns that
// CreateChangeRequestFromServiceNow (Group B's four) already writes, or
// that exist for a future write path (Groups C2/D, "read-through only"),
// but that nothing read back here before this. requested_by_user_id and
// customer_group_id are FKs (to "user"/"group" respectively), so they need
// their own joins -- see changeRequestDetailJoins. Environments/
// DeploymentProducts/Labels/Deployments (the four []EntityRef/[]string
// fields in those same groups) are deliberately excluded: no M2M join
// table for any of them exists anywhere in migrations/, so there is
// nothing to select -- same "no real column" posture as ApprovedBy/
// ApprovedOn/LegalNextStates already have (see this file's own package
// doc comment).
const changeRequestDetailColumns = `
	wi.created_by, cr.justification, cr.impact_description, cr.service_outage_downtime,
	cr.communication_plan, cr.rollback_process, cr.test_plan,
	cr.is_customer_approval_required, cr.is_customer_review_required,
	cr.implementation_plan, cr.priority::TEXT, cr.category::TEXT,
	rb.id, COALESCE(rb.name, NULLIF(TRIM(CONCAT_WS(' ', rb.first_name, rb.last_name)), '')),
	cr.affected_services, cr.affected_component, cr.rollback_duration,
	cr.change_request_type::TEXT, cr.likelihood::TEXT, cr.is_planning_visible_to_customers,
	cr.customer_updated_date_confirmation::TEXT,
	CASE WHEN isfinite(cr.customer_updated_on) THEN cr.customer_updated_on END,
	CASE WHEN isfinite(cr.work_start_on) THEN cr.work_start_on END, CASE WHEN isfinite(cr.work_end_on) THEN cr.work_end_on END, cr.git_reference,
	cr.customer_approval_required, cr.customer_review_required`

// changeRequestDetailJoins adds the two FK joins changeRequestDetailColumns
// needs beyond changeRequestFromJoins -- kept separate from (not folded
// into) changeRequestFromJoins since RequestedBy is a detail
// -only fields (domain.ChangeRequest, not SearchChangeRequestView): folding
// these into the shared joins would cost every SearchChangeRequests/
// AggregateChangeRequests row two extra joins neither ever selects from.
const changeRequestDetailJoins = `
	LEFT JOIN "user" rb ON rb.id = cr.requested_by_user_id`

// GetChangeRequestByID implements ChangeRequestRepository.
func (r *changeRequestRepo) GetChangeRequestByID(ctx context.Context, id string) (domain.ChangeRequest, error) {
	ctx = withCRVisibility(ctx, r.vis)
	// The visibility rule is part of the one SELECT, so a change request a
	// customer may not see is exactly as absent here as it is from the list.
	visSQL, args := r.vis.andClause(ctx, "wi", "cr", []any{id})
	query := "SELECT " + changeRequestSelectColumns + ", " + changeRequestDetailColumns + " " +
		changeRequestFromJoins + " " + changeRequestDetailJoins + " WHERE wi.id = $1 AND wi.type = 'CHANGE_REQUEST'" + visSQL

	var cr domain.ChangeRequest
	row := r.db.QueryRow(ctx, query, args...)
	err := scanChangeRequestViewAndDetail(row, &cr)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ChangeRequest{}, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return domain.ChangeRequest{}, fmt.Errorf("get change request by id: %w", err)
	}
	// Deployments / DeploymentProducts come from the join tables (never nil).
	// ApprovedBy/ApprovedOn/Labels have no real column -- see this file's own
	// package doc comment.
	cr.Deployments, cr.DeploymentProducts, err = loadChangeRequestLinks(ctx, r.db, id)
	if err != nil {
		return domain.ChangeRequest{}, fmt.Errorf("get change request by id: %w", err)
	}
	// The Customer Group is derived live from the project, never stored.
	if cr.CustomerContacts, err = customerContactRefs(ctx, r.db, cr.Project.ID); err != nil {
		return domain.ChangeRequest{}, fmt.Errorf("get change request by id: %w", err)
	}

	// While the customer group's review request is pending (a live "Customer
	// Review" stage), staff are not offered Rollback either: a failed review is
	// the customer's to give (their rejection rolls the change back). The
	// customer's own approval / review is never offered to staff at all
	// (changeRequestForwardNextStates), live stage or not.
	if cr.State != nil {
		live, err := liveCustomerStageForState(ctx, r.db, id, strings.ToUpper(*cr.State))
		if err != nil {
			return domain.ChangeRequest{}, fmt.Errorf("get change request by id: %w", err)
		}
		cr.LegalNextStates = withoutStaffRollbackWhileCustomerReviewPending(cr.State, cr.LegalNextStates, live != nil)
	}

	// The conversation about a time the customer proposed, when there is one.
	r.fillCustomerProposal(ctx, &cr)

	// For a customer reading the detail (or the PATCH receipt, which is this
	// same read): whether they may answer it right now. Computed last, from the
	// state just read, and only for external callers.
	r.markCustomerCanAnswer(ctx, &cr)
	return cr, nil
}

// scanChangeRequestViewAndDetail scans changeRequestSelectColumns followed
// by changeRequestDetailColumns's targets in the same Scan call (a single
// row's columns must be scanned together), populating cr directly rather
// than returning a long list of out-params.
func scanChangeRequestViewAndDetail(row pgx.Row, cr *domain.ChangeRequest) error {
	var v domain.SearchChangeRequestView
	var (
		projectID, projectName *string
		caseID, caseNumber     *string
		depID, depName         *string
		dpID, dpName           *string
		prodID, prodName       *string
		svcID, svcName         *string
		soID, soName           *string
		aeID, aeName           *string
		agID, agName           *string
		startOn, endOn         *time.Time
		impact, state          *string
		changeModel            *string
		createdOn, updatedOn   time.Time
		isOnHold               *bool
		onHoldReason           *string

		createdBy                                                          string
		justification, impactDescription, serviceOutage                    *string
		communicationPlan, rollbackPlan, testPlan                          *string
		isCustomerApproved, isCustomerReviewed                             *bool
		implementationPlan, priority, category                             *string
		rbID, rbName                                                       *string
		affectedServicesText, affectedComponentsText, rollbackDurationText *string
		changeRequestType, likelihood                                      *string
		isPlanningVisibleToCustomers                                       *bool
		confirmCustomerUpdatedDate                                         *string
		customerUpdatedOn, workStart, workEnd                              *time.Time
		gitReference                                                       *string
		customerApprovalRequired, customerReviewRequired                   bool
	)
	err := row.Scan(
		&v.ID, &v.Number, &v.Subject, &v.Description,
		&projectID, &projectName,
		&caseID, &caseNumber,
		&depID, &depName,
		&dpID, &dpName,
		&prodID, &prodName,
		&svcID, &svcName,
		&soID, &soName,
		&aeID, &aeName,
		&startOn, &endOn, &impact, &state, &changeModel,
		&createdOn, &updatedOn,
		&agID, &agName,
		&isOnHold, &onHoldReason,
		&createdBy, &justification, &impactDescription, &serviceOutage, &communicationPlan, &rollbackPlan, &testPlan,
		&isCustomerApproved, &isCustomerReviewed,
		&implementationPlan, &priority, &category,
		&rbID, &rbName,
		&affectedServicesText, &affectedComponentsText, &rollbackDurationText,
		&changeRequestType, &likelihood, &isPlanningVisibleToCustomers,
		&confirmCustomerUpdatedDate, &customerUpdatedOn,
		&workStart, &workEnd, &gitReference,
		&customerApprovalRequired, &customerReviewRequired,
	)
	if err != nil {
		return err
	}
	if projectID != nil {
		v.Project = domain.EntityRef{ID: *projectID, Name: stringOrEmpty(projectName)}
	}
	if caseID != nil {
		v.Case = &domain.EntityRef{ID: *caseID, Name: stringOrEmpty(caseNumber)}
	}
	if depID != nil {
		v.Deployment = &domain.EntityRef{ID: *depID, Name: stringOrEmpty(depName)}
	}
	if dpID != nil {
		v.DeployedProduct = &domain.EntityRef{ID: *dpID, Name: stringOrEmpty(dpName)}
	}
	if prodID != nil {
		v.Product = &domain.EntityRef{ID: *prodID, Name: stringOrEmpty(prodName)}
	}
	if svcID != nil {
		v.Service = &domain.EntityRef{ID: *svcID, Name: stringOrEmpty(svcName)}
	}
	if soID != nil {
		v.ServiceOffering = &domain.EntityRef{ID: *soID, Name: stringOrEmpty(soName)}
	}
	if aeID != nil {
		v.AssignedEngineer = &domain.EntityRef{ID: *aeID, Name: stringOrEmpty(aeName)}
	}
	if agID != nil {
		v.AssignedTeam = &domain.EntityRef{ID: *agID, Name: stringOrEmpty(agName)}
	}
	if startOn != nil {
		v.PlannedStartOn = fmtInstantPtr(startOn)
	}
	if endOn != nil {
		v.PlannedEndOn = fmtInstantPtr(endOn)
	}
	if impact != nil {
		lower := strings.ToLower(*impact)
		v.Impact = &lower
	}
	if state != nil {
		lower := strings.ToLower(*state)
		v.State = &lower
	}
	if changeModel != nil {
		if t, ok := changeRequestChangeModelToType[*changeModel]; ok {
			s := string(t)
			v.Type = &s
		}
	}
	v.OnHold = isOnHold
	v.OnHoldReason = onHoldReason
	v.CreatedOn = createdOn.UTC().Format(time.RFC3339)
	v.UpdatedOn = updatedOn.UTC().Format(time.RFC3339)
	cr.SearchChangeRequestView = v
	// The next states offered follow the review gate in effect: an Emergency change
	// is acted on without the customer's consent, so its Review goes to Closed
	// whatever its stored box says (the stored value itself is returned as is below).
	cr.LegalNextStates = legalChangeRequestNextStatesForModel(v.State, stringOrEmpty(changeModel), customerReviewRequired)
	cr.CustomerApprovalRequired = customerApprovalRequired
	cr.CustomerReviewRequired = customerReviewRequired

	cr.CreatedBy = createdBy
	cr.Justification = justification
	cr.ImpactDescription = impactDescription
	cr.ServiceOutage = serviceOutage
	cr.CommunicationPlan = communicationPlan
	cr.RollbackPlan = rollbackPlan
	cr.TestPlan = testPlan
	cr.HasCustomerApproved = isCustomerApproved != nil && *isCustomerApproved
	cr.HasCustomerReviewed = isCustomerReviewed != nil && *isCustomerReviewed

	cr.ImplementationPlan = implementationPlan
	if priority != nil {
		lower := strings.ToLower(*priority)
		cr.Priority = &lower
	}
	if category != nil {
		lower := strings.ToLower(*category)
		cr.Category = &lower
	}
	if rbID != nil {
		cr.RequestedBy = &domain.EntityRef{ID: *rbID, Name: stringOrEmpty(rbName)}
	}
	cr.AffectedServicesText = affectedServicesText
	cr.AffectedComponentsText = affectedComponentsText
	cr.RollbackDurationText = rollbackDurationText
	if changeRequestType != nil {
		lower := strings.ToLower(*changeRequestType)
		cr.ChangeRequestType = &lower
	}
	if likelihood != nil {
		lower := strings.ToLower(*likelihood)
		cr.Likelihood = &lower
	}
	cr.IsPlanningVisibleToCustomers = isPlanningVisibleToCustomers != nil && *isPlanningVisibleToCustomers
	if confirmCustomerUpdatedDate != nil {
		lower := strings.ToLower(*confirmCustomerUpdatedDate)
		cr.ConfirmCustomerUpdatedDate = &lower
	}
	cr.CustomerUpdatedOn = fmtInstantPtr(customerUpdatedOn)
	cr.WorkStart = fmtInstantPtr(workStart)
	cr.WorkEnd = fmtInstantPtr(workEnd)
	cr.GitReference = gitReference
	return nil
}

// changeRequestPatchFKField maps work_item's FK constraints touched by
// PatchChangeRequest back to the request field that set them. None of these
// FKs are given an explicit CONSTRAINT name in the migrations, so Postgres's
// default "<table>_<column>_fkey" naming applies. Naming the field here
// keeps a 23503 violation's client-facing message useful without echoing
// pgErr.Detail, which quotes the real table/column name.
var changeRequestPatchFKField = map[string]string{
	"work_item_project_id_fkey":          "projectId",
	"work_item_parent_id_fkey":           "caseId",
	"work_item_deployment_id_fkey":       "deploymentId",
	"work_item_deployed_product_id_fkey": "deployedProductId",
	"work_item_assigned_to_id_fkey":      "assignedEngineerId",
	"work_item_assignment_group_id_fkey": "assignedTeamId",
}

// changeRequestPatchCRFKField mirrors changeRequestPatchFKField for the
// change_request table's own FK columns (migration 0046).
var changeRequestPatchCRFKField = map[string]string{
	"change_request_service_id_fkey":          "serviceId",
	"change_request_service_offering_id_fkey": "serviceOfferingId",
}

// PatchChangeRequest implements ChangeRequestRepository.
func (r *changeRequestRepo) PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest, actorEmail string) (domain.ChangeRequest, error) {
	ctx = withCRVisibility(ctx, r.vis)
	wiID, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) (string, error) {
		// FIRST statement of the transaction, before the request is classified
		// or anything is locked or written: a customer who may not see this
		// change request gets the same 404 whatever they send, even a field
		// they could never set (which would otherwise be a 403 that confirms
		// the change request exists).
		if err := r.vis.requireVisibleChangeRequest(ctx, tx, id); err != nil {
			return "", err
		}
		return patchChangeRequestTx(ctx, tx, id, req, actorEmail)
	})
	if err != nil {
		return domain.ChangeRequest{}, err
	}
	return r.GetChangeRequestByID(ctx, wiID)
}

// patchChangeRequestTx is PatchChangeRequest's body, extracted so it can run
// inside r.db.InTx's closure (Scoped.InTx pulls caller identity from ctx and
// sets it once for the whole transaction, same shape as
// timeCardRepo.createTimeCardTx). Returns the change request's id (== the
// work_item id) on success.
func patchChangeRequestTx(ctx context.Context, tx pgx.Tx, id string, req domain.PatchChangeRequestRequest, actorEmail string) (string, error) {
	// A customer (an external caller) may only answer what is waiting on the
	// customer or propose a new implementation time; see
	// change_request_customer_outcome.go. Decided before anything else is looked
	// at or written: the answer is the caller deciding their own pending approval
	// (the same code the decision route runs), the proposal is a start written to
	// customer_updated_on for WSO2 to answer (change_request_customer_proposal.go:
	// nothing else moves), and anything else is refused.
	if IsExternalCaller(ctx) {
		cp, err := classifyExternalPatch(req)
		if err != nil {
			return "", err
		}
		switch cp.kind {
		case customerPatchAnswer:
			return answerCustomerStageViaPatch(ctx, tx, id, cp, actorEmail)
		case customerPatchProposal:
			return proposeCustomerTime(ctx, tx, id, req, actorEmail)
		}
	}

	// Whatever is left is NOT a customer answering (that returned above): staff,
	// an internal client credential, a context with no identity. None of them may
	// record the customer's approval or review for the customer, so a request that
	// carries either flag is refused outright -- not applied, not ignored, and not
	// only when it would change the stored value (see refuseStaffCustomerOutcomeFlags).
	if err := refuseStaffCustomerOutcomeFlags(req); err != nil {
		return "", err
	}
	// WSO2's acceptance of the time a customer proposed ("Accept proposed time"):
	// a request of its own, answered by acceptCustomerProposal, which writes AGREE,
	// the proposal as the planned start and the state Scheduled in one UPDATE (see
	// change_request_customer_proposal.go). It carries no state: {state: "scheduled"}
	// stays refused for every staff caller.
	if req.ConfirmCustomerUpdatedDate != nil {
		return acceptCustomerProposal(ctx, tx, id, req, actorEmail)
	}
	// The requested state is read the way the table of moves is written: trimmed
	// and lower case, whatever the caller sent. A value that is not a state of the
	// lifecycle is refused here, before it reaches a comparison or the enum cast.
	if req.State != nil {
		normalized, err := normalizeRequestedChangeRequestState(*req.State)
		if err != nil {
			return "", err
		}
		req.State = &normalized
	}

	// The planned window is parsed before it is used for anything, by whoever
	// sends it (see change_request_window.go): nothing but an RFC 3339 /
	// "YYYY-MM-DD HH:MM:SS" date-time in a sane range, as UTC, reaches SQL.
	req, err := normalizePatchPlannedWindow(req)
	if err != nil {
		return "", err
	}
	// The window a customer's answer was given for goes with that answer
	// (classifyExternalPatch took it above), WSO2's acceptance of a proposed time
	// (acceptCustomerProposal took that) and a staff re-schedule or counter-proposal
	// (state "authorize", which checks them under the row lock); nobody else has a use
	// for them, nor for the proposal they name.
	staffTimeResponse := req.State != nil && strings.EqualFold(string(*req.State), "authorize")
	if (req.ExpectedPlannedStartOn != nil || req.ExpectedPlannedEndOn != nil) && !staffTimeResponse {
		return "", &apierror.ValidationError{Msg: "expectedPlannedStartOn / expectedPlannedEndOn can only accompany a customer's approval or review (isCustomerApproved / isCustomerReviewed), WSO2's acceptance of a customer's proposed time (confirmCustomerUpdatedDate) or a re-schedule (state authorize)"}
	}
	if req.ExpectedCustomerUpdatedOn != nil && !staffTimeResponse {
		return "", &apierror.ValidationError{Msg: "expectedCustomerUpdatedOn can only accompany WSO2's acceptance of a customer's proposed time (confirmCustomerUpdatedDate) or a re-schedule (state authorize)"}
	}

	// New->Assess is compulsorily gated on an assigned team -- a real,
	// reported bug: this used to be a frontend-only courtesy check
	// (ChangeRequestActionBar.tsx's TARGET_BLOCKED_REASON), easily bypassed
	// by any direct API caller, and it was carried over stale from an
	// earlier, since-disproven model of this transition. Confirmed by
	// explicit product decision that assignedTeamId IS genuinely required to
	// move to Assess -- enforced here, not just in the UI, so this can never
	// be skipped. Accepts either a team supplied in this same request or one
	// already on the record (e.g. set via a prior PATCH through the Edit
	// dialog, then "Request Approval" sent as its own separate request). The
	// SELECT below runs under the caller's own (not yet escalated) identity
	// -- work_item_visibility's USING clause (migration 0147) already allows
	// any project member to read their own row, so this needs no escalation
	// of its own.
	var effectiveAssignedTeamID *string
	if req.State != nil && strings.EqualFold(string(*req.State), "assess") {
		if req.AssignedTeamID != nil {
			effectiveAssignedTeamID = req.AssignedTeamID
		} else {
			var existing *string
			if err := tx.QueryRow(ctx, `SELECT assignment_group_id::text FROM work_item WHERE id = $1`, id).Scan(&existing); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return "", &apierror.NotFoundError{Msg: "change request not found"}
				}
				return "", fmt.Errorf("patch change request: check existing assigned team: %w", err)
			}
			effectiveAssignedTeamID = existing
		}
		if effectiveAssignedTeamID == nil || *effectiveAssignedTeamID == "" {
			return "", &apierror.ValidationError{Msg: "assignedTeamId is required before requesting approval for a change request"}
		}
	}

	// Journal entries ("Additional comments" / "Work notes") are append-only
	// and cannot be blank.
	if req.Comment != nil && strings.TrimSpace(*req.Comment) == "" {
		return "", &apierror.ValidationError{Msg: "comment must not be empty"}
	}
	if req.WorkNote != nil && strings.TrimSpace(*req.WorkNote) == "" {
		return "", &apierror.ValidationError{Msg: "workNote must not be empty"}
	}

	// Customer-scope fields (project, deployments, environments, deployment
	// products): validated and planned here, before anything is written, so a
	// refused combination leaves the change request untouched. See
	// change_request_links.go for the rules and the edit window.
	if err := RejectRemovedPatchFields(req); err != nil {
		return "", err
	}

	// The creation-phase gate (change_request_customer_lock.go): what may still
	// be edited about the Customer Project and the two customer boxes is a
	// function of the state the change is in -- all of it in New, from Request
	// Approval on the project is frozen and the boxes can only be ticked, never
	// unticked. The state must be the CURRENT one, so the work_item row is locked
	// first and the change_request side is read afterwards, in a statement of its
	// own (lockChangeRequestForPatch): a Request Approval racing a project edit
	// then serialises on that lock instead of both reading "New". The snapshot is
	// reused by everything below that needs the gates (it stays valid: the
	// change_request row is locked from here to the end of the transaction).
	var gates changeRequestGateSnapshot
	if changeRequestPatchNeedsGate(req) {
		var err error
		if gates, err = lockChangeRequestForPatch(ctx, tx, id); err != nil {
			return "", err
		}
		if err := validateCreationPhaseEdits(gates, req); err != nil {
			return "", err
		}
		// A box turned on after Request Approval needs somebody who can be asked
		// (rule 4b, after the rules that need no query).
		if err := checkTickedBoxCanBeAsked(ctx, tx, id, gates, req.CustomerApprovalRequired, req.CustomerReviewRequired); err != nil {
			return "", err
		}
		// A new requester after Request Approval is never one more person to ask
		// about a customer gate still ahead: judged with the request's own
		// requestedById in place of the stored one (rule 4c).
		if err := checkRequestedByLeavesSomebodyToAsk(ctx, tx, id, gates, req); err != nil {
			return "", err
		}
	}
	linkPlan, err := planChangeRequestLinks(ctx, tx, id, req, gates)
	if err != nil {
		return "", err
	}
	// The project the change will have once this PATCH is written: the request's,
	// else the stored one (the only one there can be after New).
	effectiveProject := gates.projectID
	if req.ProjectID != nil {
		effectiveProject = req.ProjectID
	}
	if err := checkSingularDeploymentFields(ctx, tx, id, req, gates, effectiveProject); err != nil {
		return "", err
	}

	wiSets := []string{"updated_on = NOW()", "updated_by = $1"}
	wiArgs := []any{actorEmail}
	wiIdx := 2
	addWI := func(assignment string, val any) {
		wiSets = append(wiSets, fmt.Sprintf(assignment, wiIdx))
		wiArgs = append(wiArgs, val)
		wiIdx++
	}
	if linkPlan != nil && linkPlan.setSingulars {
		// The single-valued deployment / deployed product columns follow the
		// first chosen deployment / product (NULL when none), so the list views
		// that still read them stay in step.
		addWI("deployment_id = $%d::uuid", linkPlan.deploymentID)
		addWI("deployed_product_id = $%d::uuid", linkPlan.deployedProductID)
	}
	if req.Title != nil {
		addWI("subject = $%d", *req.Title)
	}
	if req.Description != nil {
		addWI("description = $%d", *req.Description)
	}
	if req.ProjectID != nil {
		addWI("project_id = $%d::uuid", *req.ProjectID)
	}
	if req.CaseID != nil {
		addWI("parent_id = $%d::uuid", *req.CaseID)
	}
	if req.DeploymentID != nil {
		addWI("deployment_id = $%d::uuid", *req.DeploymentID)
	}
	if req.DeployedProductID != nil {
		addWI("deployed_product_id = $%d::uuid", *req.DeployedProductID)
	}
	if req.AssignedEngineerID != nil {
		addWI("assigned_to_id = $%d::uuid", *req.AssignedEngineerID)
	}
	if req.AssignedTeamID != nil {
		addWI("assignment_group_id = $%d::uuid", *req.AssignedTeamID)
	}

	wiArgs = append(wiArgs, id)
	wiQuery := fmt.Sprintf(`UPDATE work_item SET %s WHERE id = $%d AND type = 'CHANGE_REQUEST' RETURNING id`, strings.Join(wiSets, ", "), wiIdx)
	var wiID string
	if err := tx.QueryRow(ctx, wiQuery, wiArgs...).Scan(&wiID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
			// A member moving the change request to a project they do not
			// belong to fails work_item's WITH CHECK (SQLSTATE 42501):
			// refused on purpose, so not-found, like the change_request
			// UPDATE below, never a 500.
			return "", &apierror.NotFoundError{Msg: "change request not found"}
		}
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			field := changeRequestPatchFKField[pgErr.ConstraintName]
			if field == "" {
				field = "one or more referenced fields"
			}
			return "", &apierror.ValidationError{Msg: field + " does not refer to an existing record"}
		}
		return "", fmt.Errorf("patch change request work_item: %w", err)
	}

	// On-hold gate: a change request currently on hold must not advance via
	// a state-changing PATCH. See entity-service's own CLAUDE.md "Change
	// requests" -> "On hold" for the ServiceNow provenance (the real
	// "Change Request - Normal" workflow gates nearly every stage transition
	// behind an "Is this on hold?" check, something this schema had no
	// concept of at all before change_request.is_on_hold/migration 0178).
	//
	// Locked with FOR UPDATE and checked here, AFTER work_item's own UPDATE
	// above, rather than at the top of this function before anything is
	// written -- a plain, unlocked SELECT run before any write leaves a
	// window for a concurrent onHold:true-only PATCH to commit in between
	// this read and this transaction's own later writes, letting a
	// state-changing PATCH land against a record that is actually on hold
	// by the time it commits (CodeRabbit catch). FOR UPDATE alone at the
	// OLD, earlier position would have fixed that race but introduced a
	// worse one: every PATCH -- state-changing or not -- always writes
	// work_item first (wiSets above always includes at least updated_on/
	// updated_by) and change_request second (crSets below, whenever it's
	// non-empty), so locking change_request before work_item here would
	// make this one code path take the OPPOSITE lock order from every other
	// PATCH, and two transactions taking a shared pair of locks in opposite
	// orders is exactly how Postgres deadlocks. Running the gate here, after
	// work_item is already locked, keeps the order the same
	// (work_item -> change_request) as every other PATCH unconditionally
	// takes.
	//
	// Checked against the CURRENTLY STORED value -- never against whatever
	// OnHold this very request's own crSets may be about to set below. That
	// is deliberate: {state: X, onHold: false} in the SAME PATCH is
	// explicitly allowed ("take it off hold and advance in one call", e.g.
	// an approver clearing a hold and immediately promoting the record in
	// one action), so a request that is ALSO turning OnHold off is excluded
	// from this gate rather than rejected by it. Only a state change against
	// a record that is on hold and NOT simultaneously being taken off hold
	// in this same request is refused. The gate only ever fires when
	// req.State is non-nil -- a PATCH that doesn't touch State (e.g. editing
	// Description) is never affected by it regardless of the record's
	// on-hold status, and taking a record OFF hold (OnHold: false) with no
	// state change at all is never blocked by anything here either.
	if req.State != nil && (req.OnHold == nil || *req.OnHold) {
		var currentlyOnHold *bool
		if err := tx.QueryRow(ctx, `SELECT is_on_hold FROM change_request WHERE id = $1 FOR UPDATE`, id).Scan(&currentlyOnHold); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", &apierror.NotFoundError{Msg: "change request not found"}
			}
			return "", fmt.Errorf("patch change request: check on-hold state: %w", err)
		}
		if currentlyOnHold != nil && *currentlyOnHold {
			return "", &apierror.ValidationError{Msg: "change request is on hold; take it off hold (onHold: false) before changing its state"}
		}
	}

	// Approval-flow routing (change_request_approval_flow.go has the per-type
	// flows). State changes that are not a free human choice are decided here,
	// before anything is written to change_request. checkStaffStateRequest (the
	// transition graph, change_request_transitions.go) has already refused every
	// request that is not a resend or an edge of the table -- a final change moves
	// nowhere, no step or gate is skipped -- except the targets whose refusal is
	// one of the cases below (they say more than "not an edge"):
	//
	//   - {state: "authorize"|"customer_approval"} is rejected, except {state:
	//     "authorize"} out of Customer Approval, which is Re-schedule / Propose a
	//     different time (the state does not move; planStaffTimeResponse). Authorize
	//     is reached only by peer approval (or Request Approval on an Emergency
	//     change); Customer Approval only by the approval flow when
	//     customerApprovalRequired is set. Accepting them here would let any
	//     caller skip an approval the flow requires.
	//   - {state: "scheduled"} is rejected from EVERY state. Scheduled is reached
	//     only by the CAB cascade, by Request Approval on a Standard change,
	//     or -- from Customer Approval -- by the CUSTOMER's own approval, which no
	//     staff action can give on their behalf (customerOutcomeRefusal).
	//   - {state: "customer_review"} is rejected unless customerReviewRequired
	//     is set, and {state: "closed"} from Review is rejected when it is: the
	//     customer's review is a required step in between. {state: "closed"}
	//     from Customer Review is rejected too: only the customer's own review
	//     closes the change from there.
	//   - A change in Customer Approval or Customer Review leaves it by the
	//     customer's own answer, or by one of the few staff exits that do not
	//     answer for the customer (refuseStaffExitFromCustomerState): Cancel, and
	//     Re-schedule from Customer Approval / Rollback from Customer Review.
	//     Every other destination is refused, whichever way it is spelled.
	//   - {state: "rollback"} is the failed-review off-ramp: accepted only from
	//     Review and Customer Review (and from Customer Review only while no
	//     customer-group review request is pending -- its members' rejection
	//     is what rolls the change back then). It stamps no customer flag,
	//     provisions no stage, and cancels the still-requested approvers
	//     (as does reaching Closed or Canceled -- reconcileStaleApprovers).
	//     Rollback, Closed and Canceled are final: no state change is accepted
	//     out of them (checkStaffStateRequest).
	//   - {state: "assess"} is the Request Approval action. It is only legal
	//     from New, and the state actually written is chosen from the change's
	//     type: Assess (Normal), Authorize (Emergency), Scheduled (Standard) --
	//     Customer Approval instead of Scheduled for a Standard change that
	//     requires the customer's approval.
	//
	// The gate flags in effect are the ones in this very request when it carries
	// them, else the stored ones; an edit of a flag that the customer requirements
	// lock refuses (validateCustomerGateEdits) never gets here. The row was read
	// under FOR UPDATE by the creation-phase gate above.
	effectiveState := req.State
	var requestApprovalFlow *changeRequestFlow
	// Re-schedule / Propose a different time (see the "authorize" case below): what
	// the staff request does to the customer's proposal and to the customers' request.
	var timeResp timeResponse
	approvalRequired, reviewRequired := gates.approvalRequired, gates.reviewRequired
	if req.CustomerApprovalRequired != nil {
		approvalRequired = *req.CustomerApprovalRequired
	}
	if req.CustomerReviewRequired != nil {
		reviewRequired = *req.CustomerReviewRequired
	}
	// An Emergency change is acted on without the customer's consent: whatever its
	// boxes say (a legacy or migrated row can carry one), neither gate
	// applies to it -- the type in effect being the request's, else the stored one.
	approvalRequired, reviewRequired = effectiveCustomerGates(effectiveChangeModel(gates.model, req.Type), approvalRequired, reviewRequired)
	if req.State != nil {
		// The graph (change_request_transitions.go): a request must name the state
		// the change is in (a resend) or a move staff may make from it. Closed,
		// Canceled and Rollback are final -- nothing moves a change out of them -- and
		// no request skips a gate: Assess and Authorize leave through their approvals,
		// Customer Approval / Customer Review through the customer's answer, and a
		// state is never jumped over (New to Implement, Scheduled to Review, ...). The
		// targets that have a refusal of their own below (scheduled, authorize,
		// customer_approval, rollback, assess, new) are left to it.
		if err := checkStaffStateRequest(gates.state, *req.State, reviewRequired); err != nil {
			return "", err
		}
		switch strings.ToLower(string(*req.State)) {
		case "authorize":
			// "authorize" is the wire name of the Time Change loop out of Customer
			// Approval, not a destination: the state does not move. It is WSO2's
			// answer to a time the customer proposed -- Propose a different time (the
			// window it names, DISAGREE written) or a decline (the window kept) --
			// or, with no proposal waiting, a Re-schedule (the diagram's "Time Change =
			// Yes"). Either way the customers are asked again when the window changes,
			// and NOTHING goes through CAB again: the change itself has not changed
			// (planStaffTimeResponse).
			if gates.state != "CUSTOMER_APPROVAL" {
				return "", &apierror.ValidationError{Msg: fmt.Sprintf(
					"state %q cannot be set manually: it is reached automatically through the approval flow (Request Approval, then peer approval); it can only be set by hand to re-schedule a change from customer_approval", *req.State)}
			}
			resp, err := planStaffTimeResponse(ctx, tx, id, req, gates)
			if err != nil {
				return "", err
			}
			timeResp = resp
			if resp.declineOnly() {
				// A decline that keeps the window writes the answer and nothing else: no
				// state, no stage, no approver row (the customers' live request stands).
				effectiveState = nil
			} else {
				stay := domain.ChangeRequestStateCustomerApproval
				effectiveState = &stay
			}
		case "customer_approval":
			return "", &apierror.ValidationError{Msg: fmt.Sprintf(
				"state %q cannot be set manually: it is reached automatically through the approval flow when customerApprovalRequired is set", *req.State)}
		case "scheduled":
			// Never a manual choice, from any state. Out of Customer Approval it
			// would be the customer's approval given by someone else (the
			// customer's own answer, in the Customer Portal, is the only way).
			if gates.state == crStateCustomerApproval {
				return "", customerOutcomeRefusal(ctx, tx, id, &customerApprovalStageSpec, *req.State)
			}
			return "", &apierror.ValidationError{Msg: fmt.Sprintf(
				"state %q cannot be set manually: it is reached automatically through the approval flow (Request Approval, then peer/CAB approval), or by the customer's own approval from customer_approval", *req.State)}
		case "rollback":
			// The failed-review off-ramp: only from the two review states.
			// Terminal, no stamp of is_customer_review_required, no new stage.
			if gates.state != "REVIEW" && gates.state != "CUSTOMER_REVIEW" {
				return "", &apierror.ValidationError{Msg: `state "rollback" can only be set from review or customer_review`}
			}
			if gates.state == "CUSTOMER_REVIEW" {
				// With the customer group's review request pending, a failed
				// review is the members' decision (rejecting it rolls the
				// change back), as for closing.
				if live, err := liveCustomerStageForState(ctx, tx, id, gates.state); err != nil {
					return "", fmt.Errorf("patch change request: %w", err)
				} else if live != nil {
					return "", customerStageManualRefusal("rollback", &customerReviewStageSpec, live)
				}
			}
		case "customer_review":
			if isEmergencyModel(effectiveChangeModel(gates.model, req.Type)) {
				return "", &apierror.ValidationError{Msg: "state \"customer_review\" cannot be set: " + emergencyNoCustomerConsentMsg + "; close it from review instead"}
			}
			if !reviewRequired {
				return "", &apierror.ValidationError{Msg: "state \"customer_review\" cannot be set: customer review is not required for this change request (customerReviewRequired is false); close it from review instead"}
			}
		case "closed":
			if gates.state == "REVIEW" && reviewRequired {
				return "", &apierror.ValidationError{Msg: "state \"closed\" cannot be set from review: customer review is required for this change request (customerReviewRequired is true); move it to customer_review first"}
			}
			if gates.state == crStateCustomerReview {
				// As for scheduled above: the customer's own review (in the
				// Customer Portal) is the only way to close from here.
				return "", customerOutcomeRefusal(ctx, tx, id, &customerReviewStageSpec, *req.State)
			}
		case "assess":
			flow := changeRequestFlowForModel(effectiveChangeModel(gates.model, req.Type))
			dest := requestApprovalDestination(flow, approvalRequired)
			if gates.state != "" && gates.state != "NEW" && !strings.EqualFold(gates.state, string(dest)) {
				return "", &apierror.ValidationError{Msg: "approval can only be requested for a change request in the New state"}
			}
			// Request Approval is the moment the Customer Project is frozen: a
			// change that needs the customer must have one, or it would reach a
			// customer stage with nobody to ask and the project could not be set
			// any more. The boxes and the project in effect are the request's
			// own, else the stored ones.
			if err := checkRequestApprovalHasProject(gates.state, approvalRequired, reviewRequired, hasProjectID(effectiveProject)); err != nil {
				return "", err
			}
			// ... and with a project, it needs somebody who can be asked: the
			// registered contacts the customer stage would ask (the same test
			// provisionCustomerStage applies), else the change would reach Customer
			// Approval / Customer Review with nobody to answer.
			if err := checkRequestApprovalCanAsk(ctx, tx, id, gates.state, approvalRequired, reviewRequired, effectiveProject, req.RequestedByID); err != nil {
				return "", err
			}
			effectiveState = &dest
			requestApprovalFlow = &flow
		}
		// The destination that will actually be written (Request Approval and a
		// Standard re-schedule resolve to a state of their own above): out of a
		// customer state only the exits that answer nothing for the customer.
		if effectiveState != nil {
			if err := refuseStaffExitFromCustomerState(ctx, tx, id, gates.state, *effectiveState); err != nil {
				return "", err
			}
		}
	}
	if req.Type != nil && !resendsStoredChangeType(gates.model, req.Type) {
		// The approval stages already provisioned belong to the type they were
		// provisioned for; changing the type afterwards would leave a stage
		// structure that does not match it. (The stored type again is no change:
		// checkChangeTypeEdit, the state rule, accepts it as a no-op, and the stage
		// count is its second line for a real change only, so a whole-form resend of
		// a Normal change after Request Approval is not refused for its stages.)
		var stages int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, id).Scan(&stages); err != nil {
			return "", fmt.Errorf("patch change request: check approval stages before type change: %w", err)
		}
		if stages > 0 {
			return "", &apierror.ValidationError{Msg: "the change type cannot be changed once approval has been requested"}
		}
	}

	crSets := []string{}
	crArgs := []any{}
	crIdx := 1
	addCR := func(assignment string, val any) {
		crSets = append(crSets, fmt.Sprintf(assignment, crIdx))
		crArgs = append(crArgs, val)
		crIdx++
	}
	if req.PlannedStartOn != nil {
		// ::text::timestamptz, not left uncast: assigning a bare Go string
		// parameter directly to a TIMESTAMPTZ column makes Postgres infer
		// that parameter's OID as timestamptz, and pgx v5's timestamptz
		// codec has no encode plan for a raw string once that happens (it
		// expects time.Time/pgtype.Timestamptz). Casting through text first
		// keeps the parameter bound as text -- matching a Go string's own
		// default codec -- with the timestamptz conversion then happening
		// server-side.
		addCR("start_on = $%d::text::timestamptz", *req.PlannedStartOn)
	}
	if req.PlannedEndOn != nil {
		addCR("end_on = $%d::text::timestamptz", *req.PlannedEndOn)
	}
	if req.ServiceID != nil {
		addCR("service_id = $%d::uuid", *req.ServiceID)
	}
	if req.ServiceOfferingID != nil {
		addCR("service_offering_id = $%d::uuid", *req.ServiceOfferingID)
	}
	if req.Impact != nil {
		addCR("impact = $%d::change_request_impact_enum", strings.ToUpper(string(*req.Impact)))
	}
	if effectiveState != nil {
		addCR("state = $%d::change_request_state_enum", strings.ToUpper(string(*effectiveState)))
	}
	if req.Type != nil {
		enumValue, ok := changeRequestTypeToChangeModel[*req.Type]
		if !ok {
			// "model"/"site_reliability_ops" predate change_model
			// (migration 0056) and have no real enum label there --
			// see changeRequestChangeModelToType's own doc comment.
			return "", &apierror.ValidationError{Msg: fmt.Sprintf("type %q is not supported on the PostgreSQL data source", *req.Type)}
		}
		addCR("change_model = $%d::change_request_change_model_enum", enumValue)
	}
	if req.Justification != nil {
		addCR("justification = $%d", *req.Justification)
	}
	if req.ImpactDescription != nil {
		addCR("impact_description = $%d", *req.ImpactDescription)
	}
	if req.ServiceOutage != nil {
		addCR("service_outage_downtime = $%d", *req.ServiceOutage)
	}
	if req.CommunicationPlan != nil {
		addCR("communication_plan = $%d", *req.CommunicationPlan)
	}
	if req.RollbackPlan != nil {
		addCR("rollback_process = $%d", *req.RollbackPlan)
	}
	if req.TestPlan != nil {
		addCR("test_plan = $%d", *req.TestPlan)
	}
	// IsCustomerApproved / IsCustomerReviewed (the customer's OUTCOME columns,
	// is_customer_approval_required / is_customer_review_required) are never
	// written from this path: a staff request that carries either was refused at
	// the top (refuseStaffCustomerOutcomeFlags), and the customer's own answer
	// never gets here (answerCustomerStageViaPatch returned earlier, and
	// applyCustomerStageOutcome stamps the flag with the state it moves to).
	// The creation form's checkboxes: the requirement, not the outcome. Any
	// edit past the gate was refused above (validateCustomerGateEdits).
	if req.CustomerApprovalRequired != nil {
		addCR("customer_approval_required = $%d", *req.CustomerApprovalRequired)
	}
	// WSO2's answer to a time the customer proposed: the previous system's Disagree, written
	// with the window WSO2 proposes instead (or the plan it keeps). A literal, not a
	// bound value, so it reads the same on every shape of the enum column. Never the
	// sync-owned requirement flags: a Re-schedule writes no flag at all (the change
	// does not re-enter the CAB cascade, so nothing needs one).
	if timeResp.disagree {
		crSets = append(crSets, "customer_updated_date_confirmation = 'DISAGREE'")
	}
	if req.CustomerReviewRequired != nil {
		addCR("customer_review_required = $%d", *req.CustomerReviewRequired)
	}
	// RequestApproval is a pure bookkeeping flag: it records that approval
	// has been requested (change_request.approval = 'REQUESTED') and has no
	// state-transition side effect of its own. This used to also force
	// state to ASSESS when req.State wasn't separately provided, modeling
	// New->Assess as an approval-gated ceremony -- confirmed against the
	// real ServiceNow instance to be wrong: New->Assess (like every other
	// non-approval-gated transition) is a plain, ungated state change, no
	// different from picking a new value from a dropdown, and has nothing
	// to do with approval at all. The frontend now sends a plain
	// {state: "assess"} for that transition, handled generically by the
	// req.State branch above -- no special casing needed here for
	// New->Assess specifically. The one real approval-gated transition is
	// Assess->Authorize, which is unrelated to this flag entirely and is
	// handled by DecideChangeRequestApproval, which already cascades
	// change_request.state forward on its own.
	if req.RequestApproval != nil && *req.RequestApproval {
		addCR("approval = $%d::change_request_approval_enum", "REQUESTED")
	}
	// **T fields: nil outer = omitted, non-nil outer with nil inner = explicit
	// null (clear the column), otherwise set it.
	addNullableText := func(col string, v **string) {
		if v == nil {
			return
		}
		if *v == nil {
			crSets = append(crSets, col+" = NULL")
			return
		}
		addCR(col+" = $%d", **v)
	}
	addNullableText("implementation_plan", req.ImplementationPlan)
	addNullableText("affected_services", req.AffectedServicesText)
	addNullableText("affected_component", req.AffectedComponentsText)
	addNullableText("rollback_duration", req.RollbackDurationText)
	if req.RequestedByID != nil {
		if *req.RequestedByID == nil {
			crSets = append(crSets, "requested_by_user_id = NULL")
		} else {
			addCR("requested_by_user_id = $%d::uuid", **req.RequestedByID)
		}
	}
	if req.Priority != nil {
		if *req.Priority == nil {
			crSets = append(crSets, "priority = NULL")
		} else {
			addCR("priority = $%d::change_request_priority_enum", strings.ToUpper(string(**req.Priority)))
		}
	}
	if req.Category != nil {
		if *req.Category == nil {
			crSets = append(crSets, "category = NULL")
		} else {
			label := strings.ToUpper(string(**req.Category))
			if !changeRequestCategoryPGLabels[label] {
				return "", &apierror.ValidationError{Msg: fmt.Sprintf("category %q is not supported on the PostgreSQL data source", **req.Category)}
			}
			addCR("category = $%d::change_request_category_enum", label)
		}
	}
	if req.IsPlanningVisibleToCustomers != nil {
		addCR("is_planning_visible_to_customers = $%d", *req.IsPlanningVisibleToCustomers)
	}
	// OnHold/OnHoldReason -- see PatchChangeRequestRequest.OnHold's own doc
	// comment for the full tri-state write behavior this implements:
	//   - OnHold: true  -> is_on_hold = true, on_hold_reason = OnHoldReason if
	//     provided in this same request, else NULL (a fresh hold event does
	//     not inherit a stale reason from a previous one).
	//   - OnHold: false -> is_on_hold = false, and on_hold_reason is cleared
	//     to NULL regardless of whether OnHoldReason also accompanies this
	//     same request -- taking a record off hold always wins over setting a
	//     reason text in the same call.
	//   - OnHold omitted, OnHoldReason provided -> only on_hold_reason is
	//     written, letting a caller edit the reason text of an existing hold
	//     (or set one belatedly) without resending OnHold itself.
	// The two branches are mutually exclusive by construction (each either
	// appends exactly one "on_hold_reason = ..." assignment or none), so
	// there is never a double assignment to the same column in one UPDATE.
	if req.OnHold != nil {
		addCR("is_on_hold = $%d", *req.OnHold)
		if *req.OnHold {
			if req.OnHoldReason != nil {
				addCR("on_hold_reason = $%d", *req.OnHoldReason)
			} else {
				crSets = append(crSets, "on_hold_reason = NULL")
			}
		} else {
			crSets = append(crSets, "on_hold_reason = NULL")
		}
	} else if req.OnHoldReason != nil {
		addCR("on_hold_reason = $%d", *req.OnHoldReason)
	}
	// Type has no real mapping -- see this file's own package doc comment.

	if len(crSets) > 0 {
		crArgs = append(crArgs, id)
		crQuery := fmt.Sprintf(`UPDATE change_request SET %s WHERE id = $%d`, strings.Join(crSets, ", "), crIdx)
		ct, err := tx.Exec(ctx, crQuery, crArgs...)
		if err != nil {
			if IsRLSPolicyViolation(err) {
				return "", &apierror.NotFoundError{Msg: "change request not found"}
			}
			if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
				field := changeRequestPatchCRFKField[pgErr.ConstraintName]
				if field == "" {
					field = "one or more referenced fields"
				}
				return "", &apierror.ValidationError{Msg: field + " does not refer to an existing record"}
			}
			return "", fmt.Errorf("patch change request: %w", err)
		}
		// change_request's RLS USING clause (migration 0145) silently
		// excludes a row the caller isn't a project member of -- a plain
		// Exec with no RETURNING never surfaces that as pgx.ErrNoRows the
		// way the work_item UPDATE above does, so it must be checked
		// explicitly here or a non-member caller would see a false
		// "success" with nothing actually changed.
		if ct.RowsAffected() == 0 {
			return "", &apierror.NotFoundError{Msg: "change request not found"}
		}
	}

	// Join rows for the scope fields, then the journal entries.
	if err := applyChangeRequestLinkPlan(ctx, tx, id, linkPlan); err != nil {
		return "", fmt.Errorf("patch change request: %w", err)
	}
	if req.Comment != nil {
		if err := insertChangeRequestJournalEntry(ctx, tx, id, "COMMENT", actorEmail, *req.Comment); err != nil {
			return "", fmt.Errorf("patch change request: %w", err)
		}
	}
	if req.WorkNote != nil {
		if err := insertChangeRequestJournalEntry(ctx, tx, id, "WORK_NOTE", actorEmail, *req.WorkNote); err != nil {
			return "", fmt.Errorf("patch change request: %w", err)
		}
	}

	// Request Approval provisions the first approval stage for the change's type
	// (see change_request_approval_flow.go): the peer approval stage for a
	// Normal change, the one CAB stage for an Emergency change, nothing for a
	// Standard change (it is already Scheduled). provisionApprovalStage owns
	// "no stage exists yet for this checkpoint" so a resent {state: "assess"}
	// never duplicates a stage or re-seeds approvers; from here on
	// DecideChangeRequestApproval owns the stage.
	//
	// For a Normal change the CAB stage is provisioned later, by the peer
	// approval cascade -- but its approver pool is validated NOW, in the same
	// transaction, so a change cannot be sent for peer approval into a flow
	// that has nobody to give the CAB approval (the request is refused with a
	// clear message instead of being stranded in Authorize later).
	if requestApprovalFlow != nil && requestApprovalFlow.checkpoint != nil {
		created, err := provisionApprovalStage(ctx, tx, id, effectiveAssignedTeamID, actorEmail, *requestApprovalFlow.checkpoint)
		if err != nil {
			return "", err
		}
		if created && requestApprovalFlow.checkpoint.Pool == poolPeer {
			creatorIDs, err := changeRequestCreatorUserIDs(ctx, tx, id)
			if err != nil {
				return "", fmt.Errorf("patch change request: %w", err)
			}
			if _, err := resolveApprovalPool(ctx, tx, changeRequestCABCheckpoint, nil, creatorIDs); err != nil {
				return "", err
			}
		}
	}

	// The third approval checkpoint, Review ("Internal Review" in real
	// ServiceNow): the identical auto-provisioning gap the Assess and
	// Authorize paragraphs above close, at the next lifecycle step a change
	// request can reach it -- ...->Implement->Review->{Closed,
	// CustomerReview}->Closed (changeRequestForwardNextStates). Same product
	// decision as Authorize: reuses the SAME assigned team
	// (work_item.assignment_group_id), the identical self-approval-exclusion
	// and dead-end-guard rules, via provisionApprovalStage itself -- nothing
	// about those rules is checkpoint-specific, only the ordinal/label
	// (changeRequestReviewCheckpoint) differs.
	//
	// Unlike Authorize, Review has no cascading entry point at all to wire a
	// second call site for: DecideChangeRequestApproval's own state cascade
	// is deliberately scoped to Assess->Authorize only (see that method's own
	// doc comment -- "Authorize's own outgoing approval gate ... is a
	// separate, deferred piece of work"), so nothing in this repository ever
	// advances change_request.state to Review as a side effect of an approval
	// decision. The only way a change request reaches Review at all on this
	// data source is a direct {state: "review"} PATCH -- via Authorize's own
	// Scheduled->Implement->Review path, since there is no "Change state ->
	// Review" cascade to piggyback on the way Authorize piggybacks on
	// DecideChangeRequestApproval's Assess->Authorize cascade. This branch is
	// therefore the ONE real entry point Review's provisioning needs, not an
	// incomplete first half of a pair.
	//
	// Same "no compulsory assignedTeamId gate" reasoning as Authorize: by the
	// time a change request reaches Review through its only normal path
	// (Assess's own compulsory gate, then Authorize, then Scheduled/Customer
	// Approval, then Implement), a team is already guaranteed to be on the
	// record, so a third hard gate here would be redundant product surface
	// for a case that shouldn't occur. A direct PATCH that skips straight to
	// {state: "review"} with no team ever assigned is still caught by
	// provisionApprovalStage's own "no members" check, exactly like Authorize.
	var effectiveAssignedTeamIDForReview *string
	if req.State != nil && strings.EqualFold(string(*req.State), "review") {
		if req.AssignedTeamID != nil {
			effectiveAssignedTeamIDForReview = req.AssignedTeamID
		} else {
			var existing *string
			if err := tx.QueryRow(ctx, `SELECT assignment_group_id::text FROM work_item WHERE id = $1`, id).Scan(&existing); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return "", &apierror.NotFoundError{Msg: "change request not found"}
				}
				return "", fmt.Errorf("patch change request: check existing assigned team: %w", err)
			}
			effectiveAssignedTeamIDForReview = existing
		}
		if _, err := provisionApprovalStage(ctx, tx, id, effectiveAssignedTeamIDForReview, actorEmail, changeRequestReviewCheckpoint); err != nil {
			return "", err
		}
	}

	// Re-schedule / Propose a different time with a new window: the customers'
	// pending request is replaced by a fresh one (the call below provisions it once
	// the old one is cancelled). Nothing internal is repeated.
	if timeResp.asksAgain {
		if err := cancelLiveCustomerStages(ctx, tx, id, actorEmail); err != nil {
			return "", err
		}
	}
	// The customer group (the project's registered contacts) answers Customer
	// Approval / Customer Review through an approval stage of its own
	// (provisionCustomerStage). Whatever this PATCH changed about the state, or
	// restated about the project, bring that stage in step with the change as it
	// now stands: provision it on entering the state (Request Approval on a
	// Standard change, {state: customer_review}), replace it when the contacts
	// changed, cancel it when the change leaves the state. Idempotent, and a no-op
	// for every other state.
	//
	// The project can only CHANGE in New (change_request_customer_lock.go), where
	// there is no customer stage to follow it. After New the trigger on projectId
	// is for the resend: an equal projectId is an accepted no-op write that still
	// re-derives the contacts, which is the way a change in a customer state
	// learns of a contact who registered after it was asked (the Customer Group
	// is derived live and nothing else notices).
	if (req.State != nil && !timeResp.declineOnly()) || req.ProjectID != nil {
		if _, err := provisionCustomerStage(ctx, tx, id, actorEmail); err != nil {
			return "", err
		}
	}

	// The change has a new state: an approval that belongs to a state it is no
	// longer in is not actionable any more. This cancels the still-requested
	// approvers of every stage the change has left (Review's once it moves on
	// to Customer Review / Closed / Rollback, the customer's once it is
	// re-scheduled, ...) and of ALL stages once it is Closed, Canceled or
	// Rolled back. Last on purpose: the stage the NEW state needs (Peer, CAB,
	// Review, a customer stage) has just been provisioned above and
	// belongs to that state, so it is kept. See reconcileStaleApprovers.
	if effectiveState != nil {
		if err := reconcileStaleApprovers(ctx, tx, id, actorEmail); err != nil {
			return "", err
		}
	}

	return wiID, nil
}

// callerIsRegisteredPortalContact reports whether actorEmail is a REGISTERED
// PORTAL_USER contact of the project projectID (the change request's own
// project, work_item.project_id): the one test of "this person is one of the
// customer", behind the customer's own answer (requireRegisteredContact), the
// read-only twin of it (customerCanAnswer) and the legacy customer stage
// (ensureCustomerStageForLegacy). Nobody else is the customer: there is
// deliberately NO shortcut for an internal caller (it used to say yes to one,
// for the staff write of the customer flag that no longer exists), so nothing
// built on this can let staff answer for the customer. projectID nil/empty (an
// unlinked change request) or an empty email means no project_contact row could
// match, so this returns false without querying.
//
// The same "registered contact with role X on project Y" join chain
// CaseRepository.ProjectContactEmailsByRole and customerContactsSQL use.
func callerIsRegisteredPortalContact(ctx context.Context, tx crQuerier, projectID *string, actorEmail string) (bool, error) {
	if projectID == nil || *projectID == "" || actorEmail == "" {
		return false, nil
	}
	var qualifies bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM project_contact pc
			JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
			JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
			JOIN project_role pr ON pr.id = pgr.project_role_id
			WHERE pc.project_id = $1::uuid
			  AND LOWER(pc.email) = LOWER($2)
			  AND pc.state = 'REGISTERED'::project_contact_state_enum
			  AND pr.role = 'PORTAL_USER'::project_role_enum
		)`, *projectID, actorEmail,
	).Scan(&qualifies)
	if err != nil {
		return false, fmt.Errorf("check registered project contact: %w", err)
	}
	return qualifies, nil
}

// changeRequestApprovalCheckpoint names one approval checkpoint along this
// change request's forward lifecycle (New -> Assess -> Authorize -> ...) for
// provisionApprovalStage's own purposes. approval_stage carries no column of
// its own saying which lifecycle transition a given row belongs to
// (migration 0089's full column list has none), so "which checkpoint is
// this" is answered the same way it already is everywhere else in this
// file: by a stage's zero-based ordinal position among approval_stage rows
// for the same work_item_id, ordered by created_on --
// changeRequestApprovalStagePosition (the read path, GetChangeRequestApprovals)
// and DecideChangeRequestApproval's own isAssessStage check (the decision
// path) both already rely on this exact convention. Position is deliberately
// NOT a new "stage_type" column: adding one would leave those two existing,
// already-correct call sites un-migrated for any stage created before the
// column existed (including every stage csm-sync-service has ever mirrored
// from ServiceNow's own sysapproval_group), whereas reusing the ordinal
// keeps every consumer of "which checkpoint is this" -- old and new -- in
// agreement with no backfill required.
type changeRequestApprovalCheckpoint struct {
	// Position is this checkpoint's zero-based ordinal among the change
	// request's approval stages. provisionApprovalStage only ever creates a
	// stage when precisely this many approval_stage rows already exist: fewer
	// means an earlier checkpoint's own stage has not been created yet, and
	// provisioning this one now would land it at the wrong ordinal; more means
	// this checkpoint (or a later one) already has a stage -- the "no stage
	// exists yet" guard, generalized to any position.
	Position int
	// Label names this checkpoint: written to approval_stage.checkpoint_label
	// (what GetChangeRequestApprovals returns as the stage's name) and used in
	// ValidationError messages.
	Label string
	// Pool says where the approvers come from; GroupName is the group's name
	// for poolNamedGroup.
	Pool      approvalPoolKind
	GroupName string
}

var (
	// changeRequestPeerCheckpoint is a Normal change's first stage: peer
	// approval by the active internal members of the assigned group (the
	// "Devops Approval" group when there is none), entered by Request Approval
	// (state Assess).
	changeRequestPeerCheckpoint = changeRequestApprovalCheckpoint{Position: 0, Label: approvalStageLabelPeer, Pool: poolPeer}
	// changeRequestCABCheckpoint is a Normal change's second stage, right
	// after peer approval: the "CAB Approval" group, its own approver group.
	// Provisioned by the peer-approval cascade (state Authorize); approving it
	// moves the change to Scheduled.
	changeRequestCABCheckpoint = changeRequestApprovalCheckpoint{Position: 1, Label: approvalStageLabelCAB, Pool: poolNamedGroup, GroupName: domain.CABApprovalGroupName}
	// changeRequestEmergencyCABCheckpoint is an Emergency change's ONLY stage (no
	// peer approval, so it sits at position 0): the same "CAB Approval" stage, in the
	// same group, as a Normal change's second one -- the previous system has no Emergency CAB,
	// and an Emergency change migrated from it has exactly this one stage. Entered by
	// Request Approval (state Authorize); approving it moves the change to Scheduled.
	changeRequestEmergencyCABCheckpoint = changeRequestApprovalCheckpoint{Position: 0, Label: approvalStageLabelCAB, Pool: poolNamedGroup, GroupName: domain.CABApprovalGroupName}
	// changeRequestReviewCheckpoint is the third stage of a Normal change
	// ("Internal Review"), entered by a {state: "review"} PATCH, drawn from
	// the change's assigned team. Only provisions once exactly two stages
	// already exist (peer, CAB); Standard and Emergency changes never reach
	// two and so never get one. Its label is explicit (checkpoint_label,
	// migration 0179), so it no longer collides with the positional
	// "Customer Approval" fallback label.
	changeRequestReviewCheckpoint = changeRequestApprovalCheckpoint{Position: 2, Label: approvalStageLabelReview, Pool: poolAssignedGroup}
)

// provisionApprovalStage is the auto-provisioning step shared by every
// approval checkpoint: it resolves the checkpoint's approver pool
// (resolveApprovalPool -- assigned group, peer pool, or the named CAB
// group), then creates one approval_stage row plus one approval_stage_approver
// row per member, all validated BEFORE anything is created (an empty or
// creator-only pool must never commit a stage nobody can decide).
//
// The change request's creator/requester (changeRequestCreatorUserIDs) is
// provisioned `CANCELLED`, not `REQUESTED`, when they are a member of the
// pool: nobody approves their own change request, at any stage. This
// mirrors ServiceNow's own self-approval prevention (confirmed live on
// CHG0039122: the requester's sysapproval_approver row is born Cancelled).
//
// assignedTeamID may be nil/empty (only the assigned-group and peer pools use
// it). Returns created=false, nil (a deliberate no-op) when
// checkpoint.Position does not match the number of approval_stage rows
// already on this work item.
func provisionApprovalStage(ctx context.Context, tx pgx.Tx, workItemID string, assignedTeamID *string, actorEmail string, checkpoint changeRequestApprovalCheckpoint) (bool, error) {
	// approval_stage_visibility's SELECT policy (migration 0145) already
	// allows any member of the change request's own project to see this
	// count, so this still runs under the caller's own identity.
	// The Customer Approval / Customer Review stages (provisionCustomerStage)
	// sit outside the internal checkpoint ordinals: counting them would put
	// the Review checkpoint of a Normal change that went through Customer
	// Approval at position 3 instead of 2, and it would silently never be
	// provisioned.
	// A repeated CAB stage (a change re-scheduled before a Re-schedule stopped
	// going back through CAB; such a change may still be in flight) is the same
	// checkpoint again, not a further one: only the FIRST stage of each label counts,
	// or the Review checkpoint would never be provisioned for a change that was
	// re-scheduled.
	var existingStages int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM approval_stage s WHERE s.work_item_id = $1
		   AND COALESCE(s.checkpoint_label, '') NOT IN ($2, $3)
		   AND NOT EXISTS (SELECT 1 FROM approval_stage e
		                    WHERE e.work_item_id = s.work_item_id AND e.checkpoint_label = s.checkpoint_label
		                      AND (e.created_on, e.id) < (s.created_on, s.id))`,
		workItemID, approvalStageLabelCustomerApproval, approvalStageLabelCustomerReview).Scan(&existingStages); err != nil {
		return false, fmt.Errorf("patch change request: check existing approval stages: %w", err)
	}
	if existingStages != checkpoint.Position {
		return false, nil
	}

	// approval_stage/approval_stage_approver's own INSERT policies (migration
	// 0145) are internal-only. Escalating here is safe: the caller's project
	// membership for THIS change request has already been confirmed by the
	// work_item/change_request writes earlier in the same transaction, under
	// their own, unescalated identity. setCallerIdentity re-sets the same
	// session-local GUCs Scoped.InTx set at the start of this transaction
	// (LOCAL scoping: this lasts for the remainder of THIS transaction only).
	if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
		return false, fmt.Errorf("patch change request: escalate identity for approver provisioning: %w", err)
	}

	creatorIDs, err := changeRequestCreatorUserIDs(ctx, tx, workItemID)
	if err != nil {
		return false, fmt.Errorf("patch change request: %w", err)
	}
	pool, err := resolveApprovalPool(ctx, tx, checkpoint, assignedTeamID, creatorIDs)
	if err != nil {
		return false, err
	}
	if err := insertApprovalStage(ctx, tx, workItemID, actorEmail, checkpoint.Label, pool, creatorIDs); err != nil {
		return false, err
	}
	return true, nil
}

// insertApprovalStage writes one approval_stage (checkpoint_label = label,
// assignment_group_id = the pool's group) and one approval_stage_approver per
// distinct pool member: `REQUESTED`, except the change's creator(s), who are
// born `CANCELLED` (nobody approves their own change). The caller has already
// escalated the transaction's identity.
//
// checkpoint_label (migration 0179) records which checkpoint this is
// explicitly; GetChangeRequestApprovals prefers it over the ordinal heuristic.
func insertApprovalStage(ctx context.Context, tx pgx.Tx, workItemID, actorEmail, label string, pool approvalPool, creatorIDs map[string]bool) error {
	var stageID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, checkpoint_label)
		 VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, NULLIF($3::text, '')::uuid, $4)
		 RETURNING id`,
		actorEmail, workItemID, pool.groupID, label).Scan(&stageID); err != nil {
		return fmt.Errorf("patch change request: create approval stage: %w", err)
	}

	seen := map[string]bool{}
	for _, uid := range pool.members {
		key := strings.ToLower(uid)
		if seen[key] {
			continue
		}
		seen[key] = true
		status := "REQUESTED"
		if creatorIDs[key] {
			status = "CANCELLED"
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, work_item_id, stage_id, approver_user_id, state)
			 VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3, $4::uuid, $5)`,
			actorEmail, workItemID, stageID, uid, status); err != nil {
			return fmt.Errorf("patch change request: seed approval stage approver: %w", err)
		}
	}
	return nil
}

// createChangeRequestPortalQuery is CreateChangeRequest's (the plain-Postgres,
// caller-initiated path) query -- structurally identical to
// createChangeRequestFromServiceNowQuery except id/number are generated here
// (gen_random_uuid()/next_portal_work_item_number(), migration 0140) instead
// of supplied by a prior ServiceNow response, and there is no wso2_id column
// at all either way (change_request is excluded from
// work_item_wso2_id_required_by_type, per this file's own package doc
// comment on CreateChangeRequestFromServiceNow). state is hardcoded to NEW
// for the identical reason given there: it is the one value the org's
// Change Management process always assigns on creation, not a guess.
//
// Column/output order matches the trailing SELECT exactly.
const createChangeRequestPortalQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, subject, description, type, assigned_to_id, assignment_group_id,
			project_id, deployment_id, deployed_product_id
		)
		VALUES (
			gen_random_uuid(), NOW(), NOW(), $1, $1,
			next_portal_work_item_number(), $2, $3, 'CHANGE_REQUEST'::work_item_type_enum, $4::uuid, $24::uuid,
			$27::uuid, $28::uuid, $29::uuid
		)
		RETURNING id, number, subject, created_on, updated_on, created_by
	),
	inserted_change_request AS (
		INSERT INTO change_request (
			id, state, service_id, service_offering_id, impact, risk, priority, change_model,
			justification, implementation_plan, risk_impact_analysis, backout_plan, test_plan,
			start_on, end_on, requested_by_user_id, customer_group_id,
			is_planning_visible_to_customers, affected_services, affected_component, rollback_duration,
			customer_approval_required, customer_review_required, category
		)
		SELECT id, 'NEW'::change_request_state_enum, $5::uuid, $6::uuid, $7::change_request_impact_enum, $8::change_request_risk_enum,
		       $9::change_request_priority_enum, $10::change_request_change_model_enum,
		       $11, $12, $13, $14, $15,
		       $16::text::timestamptz, $17::text::timestamptz, $18::uuid, $19::uuid,
		       $20, $21, $22, $23, COALESCE($25::boolean, false), COALESCE($26::boolean, false),
		       $30::change_request_category_enum
		FROM inserted_work_item
		RETURNING id
	)
	SELECT iwi.id, iwi.number, iwi.subject, iwi.created_on, iwi.updated_on, iwi.created_by
	FROM inserted_work_item iwi
	JOIN inserted_change_request icr ON icr.id = iwi.id`

// createdChangeRequestRow is what both create queries return.
type createdChangeRequestRow struct {
	id, number, subject, createdBy string
	createdOn, updatedOn           time.Time
}

func (c createdChangeRequestRow) response() domain.CreateChangeRequestResponse {
	resp := domain.CreateChangeRequestResponse{Message: "Change request created successfully."}
	resp.ChangeRequest.ID = c.id
	resp.ChangeRequest.Number = c.number
	resp.ChangeRequest.CreatedOn = c.createdOn.UTC().Format(time.RFC3339)
	resp.ChangeRequest.CreatedBy = c.createdBy
	return resp
}

// changeRequestCreateCategory maps the request's category to the enum label
// the insert writes, or a ValidationError when it has none.
func changeRequestCreateCategory(req domain.CreateChangeRequestRequest) (*string, error) {
	if req.Category == nil {
		return nil, nil
	}
	label := strings.ToUpper(string(*req.Category))
	if !changeRequestCategoryPGLabels[label] {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("category %q is not supported on the PostgreSQL data source", *req.Category)}
	}
	return &label, nil
}

// createChangeRequestWithScope is the part of both create paths that is not
// the insert itself: inside one transaction it validates and derives the
// customer-scope selection (project, deployments, environments, deployment
// products -- change_request_links.go), runs insert (which writes the
// work_item/change_request rows, project, the first deployment/deployed
// product and the category), writes the three join tables, and appends the
// "Additional comments" / "Work notes" journal entries as comment rows. All or
// nothing: a refused combination leaves no change request behind.
func (r *changeRequestRepo) createChangeRequestWithScope(
	ctx context.Context,
	req domain.CreateChangeRequestRequest,
	createdBy string,
	insert func(ctx context.Context, tx pgx.Tx, projectID, deploymentID, deployedProductID, category *string) (createdChangeRequestRow, error),
) (createdChangeRequestRow, error) {
	if err := RejectRemovedCreateFields(req); err != nil {
		return createdChangeRequestRow{}, err
	}
	category, err := changeRequestCreateCategory(req)
	if err != nil {
		return createdChangeRequestRow{}, err
	}
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (createdChangeRequestRow, error) {
		links, err := resolveChangeRequestLinks(ctx, tx, domain.ChangeRequestLinkSelection{
			ProjectID:            req.ProjectID,
			DeploymentIDs:        req.DeploymentIDs,
			DeploymentProductIDs: req.DeploymentProductIDs,
		}, resolveLinkOpts{})
		if err != nil {
			return createdChangeRequestRow{}, err
		}
		var projectID *string
		if links.projectID != "" {
			projectID = &links.projectID
		}
		row, err := insert(ctx, tx, projectID, firstOrNil(links.deploymentIDs()), firstOrNil(links.productIDs()), category)
		if err != nil {
			return createdChangeRequestRow{}, err
		}
		if err := writeChangeRequestDeployments(ctx, tx, row.id, links.deploymentIDs()); err != nil {
			return createdChangeRequestRow{}, err
		}
		if err := writeChangeRequestProducts(ctx, tx, row.id, links.productIDs()); err != nil {
			return createdChangeRequestRow{}, err
		}
		if req.Comment != nil && strings.TrimSpace(*req.Comment) != "" {
			if err := insertChangeRequestJournalEntry(ctx, tx, row.id, "COMMENT", createdBy, *req.Comment); err != nil {
				return createdChangeRequestRow{}, err
			}
		}
		if req.WorkNote != nil && strings.TrimSpace(*req.WorkNote) != "" {
			if err := insertChangeRequestJournalEntry(ctx, tx, row.id, "WORK_NOTE", createdBy, *req.WorkNote); err != nil {
				return createdChangeRequestRow{}, err
			}
		}
		return row, nil
	})
}

func changeRequestEnumArgs(req domain.CreateChangeRequestRequest) (impact, risk, priority *string) {
	if req.Impact != nil {
		v := strings.ToUpper(string(*req.Impact))
		impact = &v
	}
	if req.Risk != nil {
		v := strings.ToUpper(string(*req.Risk))
		risk = &v
	}
	if req.Priority != nil {
		v := strings.ToUpper(string(*req.Priority))
		priority = &v
	}
	return impact, risk, priority
}

// CreateChangeRequest implements ChangeRequestRepository.
//
// crvis: internal callers only: POST /change-requests is wrapped by internalOnly (server/routes.go), so no customer identity reaches it
func (r *changeRequestRepo) CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest, createdBy string) (domain.CreateChangeRequestResponse, error) {
	if err := ValidateCreateChangeRequestType(req.Type); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	if err := ValidateCreateChangeRequestCustomerGates(req.Type, req.CustomerApprovalRequired, req.CustomerReviewRequired); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	// The planned window is validated and read as UTC here, as on a PATCH (see
	// change_request_window.go), not left to the database's own date parser.
	req, err := normalizeCreatePlannedWindow(req)
	if err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	changeModel := ptrString(changeRequestTypeToChangeModel[*req.Type])
	impact, risk, priority := changeRequestEnumArgs(req)

	row, err := r.createChangeRequestWithScope(ctx, req, createdBy, func(ctx context.Context, tx pgx.Tx, projectID, deploymentID, deployedProductID, category *string) (createdChangeRequestRow, error) {
		var out createdChangeRequestRow
		err := tx.QueryRow(ctx, createChangeRequestPortalQuery,
			createdBy, req.Subject, req.Description, req.AssignedEngineerID,
			req.ServiceID, req.ServiceOfferingID, impact, risk, priority, changeModel,
			req.Justification, req.ImplementationPlan, req.RiskImpactAnalysis, req.BackoutPlan, req.TestPlan,
			req.PlannedStartDate, req.PlannedEndDate, req.RequestedByID, nil, // customer_group_id: no longer written (derived from the project)
			req.IsPlanningVisibleToCustomers, req.AffectedServicesText, req.AffectedComponentsText, req.RollbackDurationText,
			req.GroupID, req.CustomerApprovalRequired, req.CustomerReviewRequired,
			projectID, deploymentID, deployedProductID, category,
		).Scan(&out.id, &out.number, &out.subject, &out.createdOn, &out.updatedOn, &out.createdBy)
		return out, err
	})
	if err != nil {
		var ve *apierror.ValidationError
		if errors.As(err, &ve) {
			return domain.CreateChangeRequestResponse{}, err
		}
		// change_request_write_internal_only (migration 0145) permits only an
		// internal caller to INSERT -- POST /change-requests is gated
		// internalOnly at the route (routes.go), so this should not be
		// reachable in practice, but map it the same defensive way
		// PatchChangeRequest already does rather than leaving a theoretical
		// 42501 to surface as a raw 500.
		if IsRLSPolicyViolation(err) {
			return domain.CreateChangeRequestResponse{}, &apierror.NotFoundError{Msg: "change request not found"}
		}
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation -- one of the referenced IDs does not exist
				return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			case "P0001": // raise_exception from integrity triggers
				return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.CreateChangeRequestResponse{}, fmt.Errorf("create change request: %w", err)
	}
	return row.response(), nil
}

// createChangeRequestFromServiceNowQuery inserts both halves of a change
// request row (work_item + change_request, the same shared-primary-key
// pattern createCaseFromServiceNowQuery/createIncidentFromServiceNowQuery
// document) in one round trip via a CTE, using caller-supplied identity
// (id/number/createdBy) rather than generating any of it -- see
// CreateChangeRequestFromServiceNow's own doc comment for why, and for which
// req fields are deliberately left unwritten. type is hardcoded to
// 'CHANGE_REQUEST'::work_item_type_enum.
//
// state is likewise hardcoded to 'NEW'::change_request_state_enum, not left
// NULL as an earlier revision of this query did (see git history) -- a real
// reported bug: with no state at all, legalChangeRequestNextStates(nil)
// (this file) returns nil, so a freshly created change request offered no
// promote action whatsoever, not even the one every change request always
// starts with. The org's own Change Management process flow confirms every
// created change request begins at New unconditionally (no branch at
// creation decides otherwise), so this is a fixed value, not a field this
// query needs to accept from the caller -- ServiceNow's own create response
// carries no state field to pull one from anyway (see this method's own doc
// comment).
//
// Column/output order matches the trailing SELECT exactly.
const createChangeRequestFromServiceNowQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, subject, description, type, assigned_to_id, assignment_group_id,
			project_id, deployment_id, deployed_product_id
		)
		VALUES (
			$1, NOW(), NOW(), $2, $2,
			$3, $4, $5, 'CHANGE_REQUEST'::work_item_type_enum, $6::uuid, $26::uuid,
			$29::uuid, $30::uuid, $31::uuid
		)
		RETURNING id, number, subject, created_on, updated_on, created_by
	),
	inserted_change_request AS (
		INSERT INTO change_request (
			id, state, service_id, service_offering_id, impact, risk, priority, change_model,
			justification, implementation_plan, risk_impact_analysis, backout_plan, test_plan,
			start_on, end_on, requested_by_user_id, customer_group_id,
			is_planning_visible_to_customers, affected_services, affected_component, rollback_duration,
			customer_approval_required, customer_review_required, category
		)
		VALUES (
			$1, 'NEW'::change_request_state_enum, $7::uuid, $8::uuid, $9::change_request_impact_enum, $10::change_request_risk_enum,
			$11::change_request_priority_enum, $12::change_request_change_model_enum,
			$13, $14, $15, $16, $17,
			$18::text::timestamptz, $19::text::timestamptz, $20::uuid, $21::uuid,
			$22, $23, $24, $25, COALESCE($27::boolean, false), COALESCE($28::boolean, false),
			$32::change_request_category_enum
		)
		RETURNING id
	)
	SELECT iwi.id, iwi.number, iwi.subject, iwi.created_on, iwi.updated_on, iwi.created_by
	FROM inserted_work_item iwi
	JOIN inserted_change_request icr ON icr.id = iwi.id`

// CreateChangeRequestFromServiceNow implements ChangeRequestRepository.
//
// crvis: internal callers only: the ServiceNow-first create runs behind POST /change-requests, wrapped by internalOnly (server/routes.go)
func (r *changeRequestRepo) CreateChangeRequestFromServiceNow(ctx context.Context, req domain.CreateChangeRequestRequest, id, number, createdBy string) (domain.CreateChangeRequestResponse, error) {
	if err := ValidateCreateChangeRequestType(req.Type); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	if err := ValidateCreateChangeRequestCustomerGates(req.Type, req.CustomerApprovalRequired, req.CustomerReviewRequired); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	// The planned window is validated and read as UTC here, as on the portal
	// create and on a PATCH (see change_request_window.go), not left to the
	// database's own date parser: ServiceNow has accepted the change by now, but
	// what reaches PostgreSQL is still only ever a parsed instant. (The service
	// validates the same window before ServiceNow is called, so for a caller that
	// goes through it this cannot fail here.)
	req, err := normalizeCreatePlannedWindow(req)
	if err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	changeModel := ptrString(changeRequestTypeToChangeModel[*req.Type])
	impact, risk, priority := changeRequestEnumArgs(req)

	// WithSystemIdentity: change_request's INSERT policy (migration 0145)
	// is internal-only, and the insert only ever runs after ServiceNow's own
	// workflow has already accepted the create -- treat it as the trusted,
	// already-authorized system operation it is rather than inheriting
	// whatever identity happened to be on ctx. The project, if any, is the one
	// the caller selected on the form (validated in the same transaction).
	ctx = WithSystemIdentity(ctx)
	row, err := r.createChangeRequestWithScope(ctx, req, createdBy, func(ctx context.Context, tx pgx.Tx, projectID, deploymentID, deployedProductID, category *string) (createdChangeRequestRow, error) {
		var out createdChangeRequestRow
		err := tx.QueryRow(ctx, createChangeRequestFromServiceNowQuery,
			id, createdBy,
			number, req.Subject, req.Description, req.AssignedEngineerID,
			req.ServiceID, req.ServiceOfferingID, impact, risk, priority, changeModel,
			req.Justification, req.ImplementationPlan, req.RiskImpactAnalysis, req.BackoutPlan, req.TestPlan,
			req.PlannedStartDate, req.PlannedEndDate, req.RequestedByID, nil, // customer_group_id: no longer written (derived from the project)
			req.IsPlanningVisibleToCustomers, req.AffectedServicesText, req.AffectedComponentsText, req.RollbackDurationText,
			req.GroupID, req.CustomerApprovalRequired, req.CustomerReviewRequired,
			projectID, deploymentID, deployedProductID, category,
		).Scan(&out.id, &out.number, &out.subject, &out.createdOn, &out.updatedOn, &out.createdBy)
		return out, err
	})
	if err != nil {
		var ve *apierror.ValidationError
		if errors.As(err, &ve) {
			return domain.CreateChangeRequestResponse{}, err
		}
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation on id/number -- see this method's own doc comment for why this "shouldn't" happen
				return domain.CreateChangeRequestResponse{}, &apierror.ConflictError{Msg: "a change request already exists for this ServiceNow id/number: " + pgErr.Detail}
			case "22P02": // invalid_text_representation -- id (or another uuid/enum-typed field) was not valid
				return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: "id is not a valid UUID: " + id}
			case "23503": // foreign_key_violation -- one of the referenced IDs does not exist
				return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			case "P0001": // raise_exception from integrity triggers
				return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.CreateChangeRequestResponse{}, fmt.Errorf("create change request from servicenow: %w", err)
	}
	return row.response(), nil
}

// changeRequestApprovalStagesQuery backs GetChangeRequestApprovals' first of
// two flat queries -- see that method's own doc comment for why this isn't
// one three-way join. Ordered by created_on (then id as a stable tie-break
// for rows inserted in the same instant, e.g. a backfill) since
// buildChangeRequestApprovals' positional stage-label derivation depends
// entirely on this ordering.
const changeRequestApprovalStagesQuery = `
	SELECT ast.id, g.name, ast.checkpoint_label, g.id::text
	FROM approval_stage ast
	LEFT JOIN "group" g ON g.id = ast.assignment_group_id
	WHERE ast.work_item_id = $1
	ORDER BY ast.created_on ASC, ast.id ASC`

// changeRequestApprovalApproversQuery backs GetChangeRequestApprovals'
// second flat query. approver_name reuses comment_repo.go's
// display-name COALESCE convention (resolved_name), not
// user_repo.go's userSortColumns one, since there's no user_name fallback
// need here -- an approver with no resolvable name still reads as "" rather
// than falling back to a login handle. Filtered by work_item_id (denormalized
// onto approval_stage_approver, migration 0089's own comment on why)
// rather than joining through approval_stage, same reasoning as that
// column's own comment.
//
// u.id is selected alongside asa.id because domain.ChangeRequestApprover.ID
// must be the approver's own user id, not this junction row's id -- the
// ServiceNow-backed GetChangeRequestApprovals (sn_change_request_service.go)
// already returns sysidToUUID(the approver's own sys_id) there, and the CSM
// webapp's isMyPendingApproval compares this field against the signed-in
// caller's own /users/me id to decide whether to render Approve/Reject at
// all. A real, reported bug: this query used to select only asa.id, so
// every approver here carried the junction row's own id instead -- nobody
// could ever approve/reject their own pending approval through the portal
// on this data source, since that id could never equal any real user's id.
const changeRequestApprovalApproversQuery = `
	SELECT asa.id, asa.stage_id, u.id,
	       COALESCE(NULLIF(TRIM(u.name), ''), NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), '') AS approver_name,
	       asa.state, asa.created_on, asa.updated_on, asa.comments, u.email
	FROM approval_stage_approver asa
	LEFT JOIN "user" u ON u.id = asa.approver_user_id
	WHERE asa.work_item_id = $1
	ORDER BY asa.created_on ASC, asa.id ASC`

// changeRequestApprovalStageRow is one row of changeRequestApprovalStagesQuery.
type changeRequestApprovalStageRow struct {
	id                  string
	assignmentGroupName *string
	// checkpointLabel is migration 0179's own column -- nil for any stage
	// created before it existed, or synced from ServiceNow (which has no
	// equivalent concept); buildChangeRequestApprovals falls back to the
	// ordinal-position heuristic in that case.
	checkpointLabel *string
	// assignmentGroupID is the "group" row's id (nil for a stage recorded
	// against no group -- the customer stages -- or one whose group row is
	// gone); it becomes domain.ChangeRequestApproval.AssignmentGroup.
	assignmentGroupID *string
}

// changeRequestApprovalApproverRow is one row of
// changeRequestApprovalApproversQuery. rawStatus/stageID are nullable
// pointers because both approval_stage_approver.state (renamed from status,
// migration 0138) and .stage_id are (migration 0089's own comment on
// nullable FKs throughout, plus this column having no NOT NULL/DEFAULT
// either). approverUserID is nullable because the LEFT
// JOIN to "user" leaves it null whenever approver_user_id itself is null or
// points to a since-deleted user row.
type changeRequestApprovalApproverRow struct {
	id             string
	stageID        *string
	approverUserID *string
	approverName   string
	rawStatus      *string
	createdOn      time.Time
	updatedOn      time.Time
	comments       *string
	// approverEmail is "user".email, used only to recognise the calling
	// viewer's own row for domain.ChangeRequestApprover.CanDecide.
	approverEmail *string
}

// GetChangeRequestApprovals implements ChangeRequestRepository.
func (r *changeRequestRepo) GetChangeRequestApprovals(ctx context.Context, id string) (domain.ChangeRequestApprovals, error) {
	ctx = withCRVisibility(ctx, r.vis)
	// A change request the caller may not see has no approvals to show them:
	// 404, not the empty 200 a stage-less change request answers (that answer
	// would confirm the change request exists).
	if err := r.vis.requireVisibleChangeRequest(ctx, r.db, id); err != nil {
		return domain.ChangeRequestApprovals{}, err
	}
	stageRows, err := r.db.Query(ctx, changeRequestApprovalStagesQuery, id)
	if err != nil {
		return domain.ChangeRequestApprovals{}, fmt.Errorf("get change request approvals: query stages: %w", err)
	}
	var stages []changeRequestApprovalStageRow
	for stageRows.Next() {
		var st changeRequestApprovalStageRow
		if err := stageRows.Scan(&st.id, &st.assignmentGroupName, &st.checkpointLabel, &st.assignmentGroupID); err != nil {
			stageRows.Close()
			return domain.ChangeRequestApprovals{}, fmt.Errorf("get change request approvals: scan stage: %w", err)
		}
		stages = append(stages, st)
	}
	stageRows.Close()
	if err := stageRows.Err(); err != nil {
		return domain.ChangeRequestApprovals{}, fmt.Errorf("get change request approvals: stages: %w", err)
	}

	approverRows, err := r.db.Query(ctx, changeRequestApprovalApproversQuery, id)
	if err != nil {
		return domain.ChangeRequestApprovals{}, fmt.Errorf("get change request approvals: query approvers: %w", err)
	}
	var approvers []changeRequestApprovalApproverRow
	for approverRows.Next() {
		var ap changeRequestApprovalApproverRow
		if err := approverRows.Scan(&ap.id, &ap.stageID, &ap.approverUserID, &ap.approverName, &ap.rawStatus, &ap.createdOn, &ap.updatedOn, &ap.comments, &ap.approverEmail); err != nil {
			approverRows.Close()
			return domain.ChangeRequestApprovals{}, fmt.Errorf("get change request approvals: scan approver: %w", err)
		}
		approvers = append(approvers, ap)
	}
	approverRows.Close()
	if err := approverRows.Err(); err != nil {
		return domain.ChangeRequestApprovals{}, fmt.Errorf("get change request approvals: approvers: %w", err)
	}

	// The change's own state and type: the stage names of a synced Emergency
	// change (buildChangeRequestApprovals) and who may decide (markCanDecide) are read
	// from them. Failing to read them is not fatal to the read: the stages are named by
	// position, as before, and nobody is marked as able to decide.
	var crState, crModel *string
	metaErr := r.db.QueryRow(ctx, `SELECT state::text, change_model::text FROM change_request WHERE id = $1`, id).Scan(&crState, &crModel)
	if metaErr != nil {
		slog.WarnContext(ctx, "get change request approvals: state lookup failed, canDecide left false", "changeRequestId", id, "error", metaErr)
	}
	result := buildChangeRequestApprovals(stages, approvers, stringOrEmpty(crModel))
	if metaErr == nil {
		r.markCanDecide(ctx, id, stages, approvers, &result, crState, crModel)
	}
	if IsExternalCaller(ctx) {
		redactInternalApprovalStages(stages, &result)
	}
	return result, nil
}

// redactInternalApprovalStages cuts what a CUSTOMER may read of the approvals
// down to what is theirs. The Customer Approval / Customer Review stages are the
// customer's own colleagues' answers and stay whole; every other stage (Peer,
// CAB, Review) is WSO2's internal approval, of which the customer is told
// the stage, that it is a stage of this change and where it stands -- not who sits
// on it: no approver names, no internal user ids, no group.
func redactInternalApprovalStages(stages []changeRequestApprovalStageRow, result *domain.ChangeRequestApprovals) {
	for i := range result.Approvals {
		if i < len(stages) {
			// Customer kinds come from the label alone (a stage with none is
			// never taken for the customer's), so the positional classifier is
			// all this needs; everything else is internal and is cut down.
			switch classifyApprovalStage(stages[i].checkpointLabel, i) {
			case stageKindCustomerApproval, stageKindCustomerReview:
				continue
			}
		}
		a := &result.Approvals[i]
		a.ApproverName = ""
		a.AssignmentGroup = nil
		a.Approvers = []domain.ChangeRequestApprover{}
	}
}

// markCanDecide sets domain.ChangeRequestApprover.CanDecide on the calling
// viewer's own REQUESTED approver rows, applying the same who-may-decide rules
// DecideChangeRequestApproval enforces (creator may never approve; only an
// active internal user may decide an internal stage) and its stage/state rule:
// a row of a stage the change request is no longer in the state of
// (approvalStageOutOfState -- Review's once the change moved on, ...) is not
// decidable, whatever its status still says. It is purely advisory for the UI --
// DecideChangeRequestApproval re-checks everything -- so any failure here
// (no viewer identity, an unreadable creator row) leaves CanDecide false
// rather than failing the read.
func (r *changeRequestRepo) markCanDecide(ctx context.Context, id string, stages []changeRequestApprovalStageRow, approvers []changeRequestApprovalApproverRow, result *domain.ChangeRequestApprovals, crState, crModel *string) {
	identity, ok := CallerIdentityFromContext(ctx)
	if !ok || strings.TrimSpace(identity.ViewerEmail) == "" {
		return
	}
	viewer := strings.ToLower(strings.TrimSpace(identity.ViewerEmail))
	viewerIDs := map[string]bool{}
	for _, ap := range approvers {
		if ap.approverUserID != nil && ap.approverEmail != nil && strings.ToLower(strings.TrimSpace(*ap.approverEmail)) == viewer {
			viewerIDs[strings.ToLower(*ap.approverUserID)] = true
		}
	}
	if len(viewerIDs) == 0 {
		return
	}
	currentState := strings.ToUpper(stringOrEmpty(crState))
	currentModel := strings.ToUpper(stringOrEmpty(crModel))
	creatorIDs, err := changeRequestCreatorUserIDs(ctx, r.db, id)
	if err != nil {
		slog.WarnContext(ctx, "get change request approvals: creator lookup failed, canDecide left false", "changeRequestId", id, "error", err)
		return
	}
	// The creator is also recognised by the viewer's email matching
	// work_item.created_by (see DecideChangeRequestApproval).
	var emailMatchesCreator bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM work_item WHERE id = $1 AND LOWER(created_by) = $2)`, id, viewer).Scan(&emailMatchesCreator); err != nil {
		slog.WarnContext(ctx, "get change request approvals: creator email check failed, canDecide left false", "changeRequestId", id, "error", err)
		return
	}
	for i := range result.Approvals {
		if i >= len(stages) {
			break
		}
		kind := runtimeApprovalStageKind(stages[i].checkpointLabel, i, stages[i].assignmentGroupName, currentModel, currentState)
		outOfState := approvalStageOutOfState(kind, currentState)
		for j := range result.Approvals[i].Approvers {
			ap := &result.Approvals[i].Approvers[j]
			uid := strings.ToLower(ap.ID)
			if ap.Status != "REQUESTED" || !viewerIDs[uid] || outOfState {
				continue
			}
			ids := creatorIDs
			if emailMatchesCreator {
				ids = map[string]bool{uid: true}
				for k := range creatorIDs {
					ids[k] = true
				}
			}
			if err := approverDecisionBlock(ctx, r.db, ap.ID, ids, kind); err != nil {
				var forbidden *apierror.ForbiddenError
				if errors.As(err, &forbidden) {
					continue
				}
				slog.WarnContext(ctx, "get change request approvals: decision check failed, canDecide left false", "changeRequestId", id, "error", err)
				continue
			}
			ap.CanDecide = true
		}
	}
}

// changeRequestApprovalStageLabel resolves a stage's label/approver type,
// preferring its own explicit checkpoint_label (migration 0179, written by
// provisionApprovalStage for every checkpoint this codebase provisions)
// over the positional heuristic below. Every locally-provisioned checkpoint
// is a static-group approval (Assess/Authorize/Review today), so an
// explicit label always pairs with domain.ChangeRequestApproverTypeStaticGroup;
// a nil checkpointLabel (a ServiceNow-synced stage, or one provisioned
// before this column existed) falls back to changeRequestApprovalStagePosition
// unchanged.
//
// One exception to the position fallback, for a stage with no label on an EMERGENCY
// change (model is its upper-case change_model label): the previous system gives an Emergency
// change no peer stage and routes it to the CAB group alone, so such a stage in the
// "CAB Approval" group IS the CAB stage and is named so, wherever it sits -- not
// "Assess" because it happens to be first. (The same group-first reading decides it,
// runtimeApprovalStageKind.) Any other unlabeled stage keeps its positional name.
func changeRequestApprovalStageLabel(st changeRequestApprovalStageRow, pos int, model string) (string, domain.ChangeRequestApproverType) {
	if st.checkpointLabel != nil && *st.checkpointLabel != "" {
		return *st.checkpointLabel, domain.ChangeRequestApproverTypeStaticGroup
	}
	if isEmergencyModel(model) && strings.TrimSpace(stringOrEmpty(st.assignmentGroupName)) == domain.CABApprovalGroupName {
		return approvalStageLabelCAB, domain.ChangeRequestApproverTypeStaticGroup
	}
	return changeRequestApprovalStagePosition(pos)
}

// changeRequestApprovalStagePosition maps a stage's zero-based position
// (ordered by approval_stage.created_on) to its label and approver type.
// This is the POSITIONAL-ONLY subset of ChangeRequestUtils.
// getChangeRequestApprovals' real ServiceNow logic: the real script include
// primarily keys stage label/approverType off two hardcoded ServiceNow
// group sys_ids (falling back to this same ordinal scheme only when a
// stage's group matches neither), but those sys_ids are ServiceNow-internal
// values that were never synced into this schema as a lookup anywhere --
// there is no group.sn_sys_id-shaped column, or equivalent, to match
// against. Replicating the fallback ordinal scheme unconditionally (0 =
// Assess, 1 = Authorize, 2+ = Customer Approval) is therefore the closest
// available approximation, not a full reimplementation.
//
// This is now purely the FALLBACK for a stage with no explicit
// checkpoint_label (see changeRequestApprovalStageLabel, its only caller) --
// a ServiceNow-synced stage, or one provisioned before that column existed.
// Every checkpoint this codebase provisions writes its own label explicitly
// and never reaches this heuristic at all.
func changeRequestApprovalStagePosition(pos int) (string, domain.ChangeRequestApproverType) {
	switch pos {
	case 0:
		return "Assess", domain.ChangeRequestApproverTypeStaticGroup
	case 1:
		return "Authorize", domain.ChangeRequestApproverTypeStaticGroup
	default:
		return "Customer Approval", domain.ChangeRequestApproverTypeDynamicContact
	}
}

// changeRequestApprovalStatusByRaw normalizes approval_stage_approver.state
// (renamed from status by migration 0138; a ServiceNow sysapproval_approver.state
// passthrough -- migration 0089's own comment) to the UPPER_SNAKE_CASE values
// domain.ChangeRequestApprover.Status already carries for the ServiceNow data
// source (see snChangeRequestService.GetChangeRequestApprovals, which passes
// ServiceNow's own already-uppercase values straight through) -- this is the
// Postgres equivalent of that pass-through, applied to SN's raw lowercase
// state strings instead. Harmless once 0138's own sync-layer normalization
// lands new rows already uppercase: the map only matches lowercase keys, so
// an already-uppercase value falls through to the ToUpper no-op below.
var changeRequestApprovalStatusByRaw = map[string]string{
	"requested":    "REQUESTED",
	"approved":     "APPROVED",
	"rejected":     "REJECTED",
	"not_required": "NOT_REQUIRED",
	"cancelled":    "CANCELLED",
	"no_consensus": "NO_CONSENSUS",
}

// normalizeChangeRequestApprovalStatus applies changeRequestApprovalStatusByRaw,
// falling back to an uppercased passthrough for any value outside that set
// (so an as-yet-unseen ServiceNow state string still reads sensibly instead
// of silently vanishing -- domain.ChangeRequestApprover.Status is
// deliberately an open string, not a closed enum, for exactly this reason)
// and "UNKNOWN" only for a nil/empty raw value.
func normalizeChangeRequestApprovalStatus(raw *string) string {
	if raw == nil || *raw == "" {
		return "UNKNOWN"
	}
	if v, ok := changeRequestApprovalStatusByRaw[*raw]; ok {
		return v
	}
	return strings.ToUpper(*raw)
}

// buildChangeRequestApprovals assembles the nested domain.ChangeRequestApprovals
// shape from the two flat result sets GetChangeRequestApprovals queries
// separately (a single three-way join fanned out across stage and approver
// would need de-duplicating stage columns per approver row in Go anyway, so
// two flat queries scan more simply for no real cost -- this table is
// per-change-request, never more than a handful of rows).
//
// Approvers whose stage_id is NULL (the schema allows it -- migration
// 000087's own comment on nullable FKs throughout) are dropped: they have
// no stage to attach to, and ChangeRequestApprovals' response shape has no
// stage-less bucket to put them in.
func buildChangeRequestApprovals(stages []changeRequestApprovalStageRow, approvers []changeRequestApprovalApproverRow, model string) domain.ChangeRequestApprovals {
	approversByStage := make(map[string][]changeRequestApprovalApproverRow, len(stages))
	for _, ap := range approvers {
		if ap.stageID == nil {
			continue
		}
		approversByStage[*ap.stageID] = append(approversByStage[*ap.stageID], ap)
	}

	result := make([]domain.ChangeRequestApproval, 0, len(stages))
	for pos, st := range stages {
		label, approverType := changeRequestApprovalStageLabel(st, pos, model)

		stageApprovers := approversByStage[st.id]
		domainApprovers := make([]domain.ChangeRequestApprover, 0, len(stageApprovers))
		sawApproved, sawRejected := false, false
		for _, ap := range stageApprovers {
			status := normalizeChangeRequestApprovalStatus(ap.rawStatus)
			switch status {
			case "APPROVED":
				sawApproved = true
			case "REJECTED":
				sawRejected = true
			}

			// RespondedOn has no dedicated column. approval_stage_approver.
			// updated_on changes whenever DecideChangeRequestApproval (below)
			// or csm-sync-service's own mapper moves state away from
			// REQUESTED, so it doubles as the response timestamp once a
			// decision exists -- left nil while still REQUESTED (updated_on
			// is just the row's sync/insert watermark then) or UNKNOWN
			// (nothing meaningful to date).
			var respondedOn *string
			if status != "REQUESTED" && status != "UNKNOWN" {
				s := ap.updatedOn.UTC().Format(time.RFC3339)
				respondedOn = &s
			}

			// Falls back to the junction row's own id only when the
			// approver's user can't be resolved (approver_user_id null, or
			// pointing at a since-deleted user) -- purely so this approver
			// still has a stable, non-empty id to key a list on; it can
			// never equal a real caller's own id, so isMyPendingApproval
			// (webapp) correctly never offers Approve/Reject for it either.
			approverID := ap.id
			if ap.approverUserID != nil {
				approverID = *ap.approverUserID
			}

			createdOn := ap.createdOn.UTC().Format(time.RFC3339)

			domainApprovers = append(domainApprovers, domain.ChangeRequestApprover{
				ID:          approverID,
				Name:        ap.approverName,
				Status:      status,
				CreatedOn:   &createdOn,
				RespondedOn: respondedOn,
				Comments:    ap.comments,
			})
		}

		// First-responder-wins over the stage's approvers, mirroring
		// ChangeRequestUtils._deriveStageStatus (see approval_stage.raw_status'
		// own migration comment) -- a single REJECTED beats any number of
		// APPROVED, and a single APPROVED (once nobody has rejected) is
		// enough to resolve the stage; anything else leaves it PENDING.
		stageStatus := domain.ChangeRequestApprovalStatusPending
		if sawRejected {
			stageStatus = domain.ChangeRequestApprovalStatusRejected
		} else if sawApproved {
			stageStatus = domain.ChangeRequestApprovalStatusApproved
		}

		// The Customer Group is derived from the project's contacts, so its
		// stages are recorded against no "group" row.
		approverName := stringOrEmpty(st.assignmentGroupName)
		if approverName == "" && st.checkpointLabel != nil && customerStageSpecForLabel(*st.checkpointLabel) != nil {
			approverName = customerGroupDisplayName
		}
		// The group reference lets a client open the group and list its
		// members (GET /groups/{id}). Customer stages are recorded against no
		// group, so it stays nil for them.
		var assignmentGroup *domain.ChangeRequestApprovalGroup
		if st.assignmentGroupID != nil && *st.assignmentGroupID != "" {
			assignmentGroup = &domain.ChangeRequestApprovalGroup{ID: *st.assignmentGroupID, Name: stringOrEmpty(st.assignmentGroupName)}
		}
		result = append(result, domain.ChangeRequestApproval{
			Stage:           label,
			ApproverType:    approverType,
			ApproverName:    approverName,
			Status:          stageStatus,
			AssignmentGroup: assignmentGroup,
			Approvers:       domainApprovers,
		})
	}

	return domain.ChangeRequestApprovals{Approvals: result}
}

// decideChangeRequestApprovalQuery backs DecideChangeRequestApproval. The
// WHERE clause's state = 'REQUESTED' is the entire enforcement of "only the
// caller's own PENDING approval can be decided" -- see that method's own
// doc comment. stage_id is also returned (nullable, same as everywhere else
// in this file) so the caller can re-check that stage's own overall outcome
// for the Assess->Authorize cascade below.
//
// $5 is the stage the caller's decision is for (NULL: any -- the caller has no
// pending row with a stage, so the UPDATE matches nothing or the one stage-less
// row). It narrows the UPDATE to that one stage when the caller holds pending
// rows on several (a stale one left over from before the state reconcile and a
// live one): deciding must never resolve a row of another stage along with it.
const decideChangeRequestApprovalQuery = `
	UPDATE approval_stage_approver
	SET state = $3, updated_on = NOW(), updated_by = $4
	WHERE work_item_id = $1 AND approver_user_id = $2 AND state = 'REQUESTED'
	  AND ($5::uuid IS NULL OR stage_id = $5::uuid)
	RETURNING id, stage_id`

// cancelSiblingApprovalStageApprovers moves every other still-Requested
// approver on the given stage to Cancelled. Shared, byte-for-byte identical
// mechanics behind DecideChangeRequestApproval's first-responder-wins
// quorum rule, used the same way whether the decision that resolved the
// stage was an approval or a rejection -- see that method's own doc
// comment for why both now resolve a stage identically.
func cancelSiblingApprovalStageApprovers(ctx context.Context, tx pgx.Tx, stageID, actorEmail string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE approval_stage_approver SET state = 'CANCELLED', updated_on = NOW(), updated_by = $2
		 WHERE stage_id = $1 AND state = 'REQUESTED'`,
		stageID, actorEmail); err != nil {
		return fmt.Errorf("decide change request approval: cancel sibling approvers: %w", err)
	}
	return nil
}

// DecideChangeRequestApproval implements ChangeRequestRepository.
//
// It flips the caller's own REQUESTED approval_stage_approver row to decision,
// resolves the stage (first-responder-wins, the quorum rule
// buildChangeRequestApprovals also uses at read time), and cascades
// change_request.state:
//
//   - peer approval resolved while the change is in Assess: state -> Authorize,
//     and the CAB Approval stage is provisioned. A failure to provision it (the
//     CAB group has nobody eligible) fails the WHOLE decision, rolling it back:
//     approving into a state with nobody to approve next would strand the
//     change in Authorize, and the peer approver can neither fix nor see why.
//   - CAB approval (a Normal change's second stage, or an Emergency change's only
//     one) resolved while the change is in Authorize: state -> Scheduled,
//     automatically -- or Customer Approval when customer_approval_required is set
//     and the change is not an Emergency one (approvalGateTarget,
//     effectiveCustomerGates: an Emergency change never asks the customer). There
//     is no manual Schedule action; out of Customer Approval only the CUSTOMER's own
//     approval (the customer stage below) schedules the change.
//   - the customer group's stage while the change waits in the matching state
//     (provisionCustomerStage): "Customer Approval" approved -> Scheduled and
//     is_customer_approval_required = true, rejected -> Canceled; "Customer Review"
//     approved -> Closed and is_customer_review_required = true, rejected ->
//     Rollback. CAB approval into Customer Approval provisions the
//     "Customer Approval" stage in the same transaction.
//   - any other stage (Review, or a stage that is neither): the decision is
//     recorded and siblings cancelled, no state change.
//
// A stage can only be decided while the change request is in the state it
// belongs to (approvalStageDecidableState): a decision on a stage whose state
// the change has left is refused with a 409 and changes nothing
// (staleApprovalRefusal) -- defence in depth for rows the state reconcile has
// not cancelled (it runs on every state change, and migration 0193 repaired the
// ones that already existed). Every decision ends with reconcileStaleApprovers,
// after the cascades above, so a state the decision moved the change to leaves
// no approver of a state it left actionable (Customer Review approved -> Closed
// cancels everything still requested).
//
// A resolving approval or rejection cancels every other still-Requested
// approver on the stage (matching real ServiceNow: confirmed live on a
// 119-approver group). A rejection of an INTERNAL stage never changes
// change_request.state, in either direction: real ServiceNow rolls back to a
// state this schema cannot confirm, so inventing one would be guessing --
// existing behaviour, kept. The customer stages are the exception, above:
// their rejection has one obvious meaning (the customer declined / the review
// failed).
//
// Who may decide (checked before the row is touched, ForbiddenError otherwise):
//
//   - the change request's creator/requester may never approve it, at any
//     stage (they may still cancel it);
//   - only an active INTERNAL user may decide an internal stage (Peer, CAB,
//     Review); the customer stages are decided by the project's
//     (external) contacts.
//
// Concurrency: the change_request row is locked (SELECT ... FOR UPDATE) before
// any approver row is touched, so two decisions are serialized and cannot
// deadlock cancelling each other's siblings. change_request.state is nullable
// (pre-existing NULL-state records): a NULL state simply has nothing to cascade
// from. The cascade is keyed on both the state AND the kind of the decided
// stage, so approving a CAB stage never advances a change that is somehow still
// reading Assess.
func (r *changeRequestRepo) DecideChangeRequestApproval(ctx context.Context, id, approverUserID, decision, actorEmail string) (string, error) {
	ctx = withCRVisibility(ctx, r.vis)
	// InTxReturning: Scoped stamps the caller identity on the transaction's
	// own session (the base branch's r.db.Begin is not available on Scoped).
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (string, error) {
		// First statement: a change request the caller may not see has nothing
		// for them to decide (404), before any lock is taken.
		if err := r.vis.requireVisibleChangeRequest(ctx, tx, id); err != nil {
			return "", err
		}
		// A legacy change request already sitting in Customer Approval / Review
		// with nobody asked (it got there under an older build) gets its live
		// customer stage here, in the customer's own transaction, so their
		// answer is recorded exactly like any other. See
		// ensureCustomerStageForLegacy.
		if err := ensureCustomerStageForLegacy(ctx, tx, id, actorEmail); err != nil {
			return "", err
		}
		return decideChangeRequestApprovalTx(ctx, tx, id, approverUserID, decision, actorEmail)
	})
}

// decideChangeRequestApprovalTx is DecideChangeRequestApproval's body, run in a
// transaction the caller owns. It is the ONE implementation of "a user decides
// their own pending approval": the approvals/decision route calls it directly, and
// so does the PATCH {isCustomerApproved / isCustomerReviewed} path a customer
// uses (answerCustomerStageViaPatch), so both mechanisms record the same stage
// rows, move the state the same way and stamp the same flags.
func decideChangeRequestApprovalTx(ctx context.Context, tx pgx.Tx, id, approverUserID, decision, actorEmail string) (string, error) {
	var lockedID string
	var lockedState *string
	if err := tx.QueryRow(ctx, `SELECT id, state::text FROM change_request WHERE id = $1 FOR UPDATE`, id).Scan(&lockedID, &lockedState); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("decide change request approval: lock change request: %w", err)
	}
	// The state the change is in, fixed for this transaction by the lock
	// ("" when NULL or not visible: nothing is guarded then).
	crState := strings.ToUpper(stringOrEmpty(lockedState))

	// Who-may-decide rules, before the approver row is touched. The
	// creator rule needs no row; the internal-only rule needs the kind of the
	// stage the caller's pending row belongs to (none found: fall through to the
	// UPDATE below, whose zero rows produce the usual NotFoundError).
	creatorIDs, err := changeRequestCreatorsForApprover(ctx, tx, id, approverUserID, actorEmail)
	if err != nil {
		return "", fmt.Errorf("decide change request approval: %w", err)
	}
	// The stage the caller is deciding: their oldest pending row on a stage
	// that is decidable in the state the change is in; failing that (every
	// pending row is on a stage the change has left) their oldest one, which
	// is then refused below. None found: fall through to the UPDATE, whose
	// zero rows produce the usual NotFoundError.
	pendingStageID, kind, err := callerPendingApprovalStage(ctx, tx, id, approverUserID, crState)
	if err != nil {
		return "", fmt.Errorf("decide change request approval: %w", err)
	}
	if err := approverDecisionBlock(ctx, tx, approverUserID, creatorIDs, kind); err != nil {
		return "", err
	}
	// An approval whose stage belongs to a state the change is no longer in
	// is no longer pending. Refused before anything is written.
	if pendingStageID != nil && approvalStageOutOfState(kind, crState) {
		return "", staleApprovalRefusal(kind, crState)
	}

	var approvalID string
	var stageID *string
	err = tx.QueryRow(ctx, decideChangeRequestApprovalQuery, id, approverUserID, strings.ToUpper(decision), actorEmail, pendingStageID).Scan(&approvalID, &stageID)
	if errors.Is(err, pgx.ErrNoRows) {
		// A change waiting on the customer group's answer: say who may give
		// it, rather than a bare "no pending approval" for someone who is
		// simply not in that group.
		if refusal, rerr := customerStageDecisionRefusal(ctx, tx, id); rerr != nil {
			return "", rerr
		} else if refusal != nil {
			return "", refusal
		}
		return "", &apierror.NotFoundError{Msg: "no pending approval found for this change request and caller"}
	}
	if IsRLSPolicyViolation(err) {
		return "", &apierror.NotFoundError{Msg: "no pending approval found for this change request and caller"}
	}
	if err != nil {
		return "", fmt.Errorf("decide change request approval: %w", err)
	}

	if decision == "approved" && stageID != nil {
		var hasRejection bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM approval_stage_approver WHERE stage_id = $1 AND state = 'REJECTED')`,
			*stageID).Scan(&hasRejection); err != nil {
			return "", fmt.Errorf("decide change request approval: check stage rejections: %w", err)
		}
		if !hasRejection {
			// This decision resolves the stage: cancel every other
			// still-pending approver on it.
			if err := cancelSiblingApprovalStageApprovers(ctx, tx, *stageID, actorEmail); err != nil {
				return "", err
			}

			stageKind, err := approvalStageInfo(ctx, tx, id, *stageID)
			if err != nil {
				return "", fmt.Errorf("decide change request approval: %w", err)
			}
			var currentState, changeModel sql.NullString
			var customerApprovalRequired bool
			if err := tx.QueryRow(ctx, `SELECT state, customer_approval_required, change_model::text FROM change_request WHERE id = $1`, id).Scan(&currentState, &customerApprovalRequired, &changeModel); err != nil {
				return "", fmt.Errorf("decide change request approval: check current state: %w", err)
			}
			// An Emergency change never asks the customer, whatever its box says.
			customerApprovalRequired, _ = effectiveCustomerGates(changeModel.String, customerApprovalRequired, false)
			switch {
			case stageKind == stageKindPeer && currentState.Valid && currentState.String == "ASSESS":
				if _, err := tx.Exec(ctx, `UPDATE change_request SET state = 'AUTHORIZE' WHERE id = $1`, id); err != nil {
					return "", fmt.Errorf("decide change request approval: advance state: %w", err)
				}
				// CAB approval is right after peer approval, in its own
				// group. Provisioned inside this transaction and NOT
				// best-effort: see this method's doc comment.
				if _, err := provisionApprovalStage(ctx, tx, id, nil, actorEmail, changeRequestCABCheckpoint); err != nil {
					return "", err
				}
			case stageKind == stageKindCAB && currentState.Valid && currentState.String == "AUTHORIZE":
				// CAB approval (a Normal change's second stage, an Emergency
				// change's only one) schedules the change automatically -- there
				// is no manual Schedule -- unless the customer's approval is
				// required (never so for an Emergency change), in which case the
				// change waits in Customer Approval for the customer's own
				// approval (no staff action gives it for them).
				target := approvalGateTarget(customerApprovalRequired)
				if _, err := tx.Exec(ctx, `UPDATE change_request SET state = $2::change_request_state_enum WHERE id = $1`, id, target); err != nil {
					return "", fmt.Errorf("decide change request approval: advance state: %w", err)
				}
				// Entering Customer Approval asks the customer group, when
				// the change has one with someone eligible; otherwise nobody
				// is asked and the change can only be cancelled or re-scheduled
				// (see provisionCustomerStage).
				if target == customerApprovalStageSpec.state {
					if _, err := provisionCustomerStage(ctx, tx, id, actorEmail); err != nil {
						return "", err
					}
				}
			case customerStageSpecForKind(stageKind) != nil && currentState.Valid:
				// The customer group's answer: Customer Approval ->
				// Scheduled (customer approval recorded), Customer Review
				// -> Closed (customer review recorded).
				if _, err := applyCustomerStageOutcome(ctx, tx, id, customerStageSpecForKind(stageKind), currentState.String, true); err != nil {
					return "", err
				}
			}
		}
	} else if decision == "rejected" && stageID != nil {
		// A rejection resolves the stage exactly as decisively as an
		// approval does, so siblings are cancelled here too -- unless the
		// stage was already resolved by an approval (a ServiceNow-synced
		// or pre-fix stage can hold an Approved row next to still-Requested
		// ones; a late rejection must not retroactively cancel them).
		// change_request.state is deliberately left untouched -- see this
		// method's doc comment.
		var hasApproval bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM approval_stage_approver WHERE stage_id = $1 AND state = 'APPROVED')`,
			*stageID).Scan(&hasApproval); err != nil {
			return "", fmt.Errorf("decide change request approval: check stage approvals: %w", err)
		}
		if !hasApproval {
			if err := cancelSiblingApprovalStageApprovers(ctx, tx, *stageID, actorEmail); err != nil {
				return "", err
			}
			// A customer group's rejection is an outcome, unlike every
			// internal stage's: Customer Approval rejected -> Canceled,
			// Customer Review rejected -> Rollback.
			stageKind, err := approvalStageInfo(ctx, tx, id, *stageID)
			if err != nil {
				return "", fmt.Errorf("decide change request approval: %w", err)
			}
			if spec := customerStageSpecForKind(stageKind); spec != nil {
				var currentState sql.NullString
				if err := tx.QueryRow(ctx, `SELECT state FROM change_request WHERE id = $1`, id).Scan(&currentState); err != nil {
					return "", fmt.Errorf("decide change request approval: check current state: %w", err)
				}
				if currentState.Valid {
					if _, err := applyCustomerStageOutcome(ctx, tx, id, spec, currentState.String, false); err != nil {
						return "", err
					}
				}
			}
		}
	}

	// The decision may have moved the change (Peer -> Authorize, CAB ->
	// Scheduled / Customer Approval, a customer outcome -> Scheduled /
	// Closed / Canceled / Rollback): cancel what that left actionable.
	if err := reconcileStaleApprovers(ctx, tx, id, actorEmail); err != nil {
		return "", err
	}

	return approvalID, nil
}

// changeRequestCreatorsForApprover is changeRequestCreatorUserIDs plus the
// caller: the creator is recognised by email too (work_item.created_by), which
// covers a creator whose user row the id-based lookup missed, so when
// actorEmail is the creator's, approverUserID is added to the set. The one place
// the "nobody approves their own change" rule is assembled, for the decision
// itself and for the customer's other answers (a proposed implementation time).
func changeRequestCreatorsForApprover(ctx context.Context, tx crQuerier, id, approverUserID, actorEmail string) (map[string]bool, error) {
	creatorIDs, err := changeRequestCreatorUserIDs(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if actorEmail != "" {
		var emailMatchesCreator bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM work_item WHERE id = $1 AND LOWER(created_by) = LOWER($2))`,
			id, actorEmail).Scan(&emailMatchesCreator); err != nil {
			return nil, fmt.Errorf("check creator: %w", err)
		}
		if emailMatchesCreator {
			creatorIDs[strings.ToLower(approverUserID)] = true
		}
	}
	return creatorIDs, nil
}

// callerPendingApprovalStage resolves which stage the caller's decision is for:
// among the stages they hold a REQUESTED approver row on (oldest first), the first
// one of KNOWN kind decidable in crState (the change's upper-case state); failing
// that the first one of unknown kind (stageKindOther: a ServiceNow-synced stage
// that nothing proves anything about, decidable in any state); failing that, the
// oldest, which is then refused as out of state. Returns that stage's id and kind;
// (nil, stageKindOther) when the caller has no pending row, or only a stage-less
// one.
//
// A stage of known kind in its state wins over one of unknown kind whichever is
// older: a contact who also holds a row on a synced stage (position two, say) is
// answering the customer's stage this service wrote for them, not that row, and a
// decision recorded on the wrong stage moves nothing.
func callerPendingApprovalStage(ctx context.Context, tx pgx.Tx, workItemID, approverUserID, crState string) (*string, approvalStageKind, error) {
	rows, err := tx.Query(ctx,
		`SELECT stage_id::text FROM approval_stage_approver
		 WHERE work_item_id = $1 AND approver_user_id = $2 AND state = 'REQUESTED'
		 ORDER BY created_on ASC, id ASC`, workItemID, approverUserID)
	if err != nil {
		return nil, stageKindOther, fmt.Errorf("find pending approval: %w", err)
	}
	var stageIDs []*string
	for rows.Next() {
		var stageID *string
		if err := rows.Scan(&stageID); err != nil {
			rows.Close()
			return nil, stageKindOther, fmt.Errorf("find pending approval: %w", err)
		}
		stageIDs = append(stageIDs, stageID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, stageKindOther, fmt.Errorf("find pending approval: %w", err)
	}

	var first, unknown *string
	firstKind, unknownKind := stageKindOther, stageKindOther
	for _, stageID := range stageIDs {
		if stageID == nil {
			continue
		}
		kind, err := approvalStageInfo(ctx, tx, workItemID, *stageID)
		if err != nil {
			return nil, stageKindOther, err
		}
		switch {
		case approvalStageDecidableState(kind) == "":
			// Unknown kind: decidable anywhere, but only if nothing better turns up.
			if unknown == nil {
				unknown, unknownKind = stageID, kind
			}
		case !approvalStageOutOfState(kind, crState):
			return stageID, kind, nil
		}
		if first == nil {
			first, firstKind = stageID, kind
		}
	}
	if unknown != nil {
		return unknown, unknownKind, nil
	}
	return first, firstKind, nil
}

// changeRequestCategoryPGLabels is change_request_category_enum's label set
// (migration 0043, plus the four cloud/devops values added by migration 0191)
// -- every value of the domain's category enum.
var changeRequestCategoryPGLabels = map[string]bool{
	"SOFTWARE": true, "NETWORK": true, "SERVICE": true, "TELECOM": true, "HARDWARE": true,
	"SYSTEM_SOFTWARE": true, "DOCUMENTATION": true, "APPLICATIONS_SOFTWARE": true, "OTHER": true,
	"REGULAR_RELEASE_CLOUD": true, "HOTFIX_RELEASE_CLOUD": true, "DEVOPS": true, "CLOUD_COMPUTING": true,
}
