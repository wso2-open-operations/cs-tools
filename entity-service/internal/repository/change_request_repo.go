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
// directly through this API. Writing it at create time
// (CreateChangeRequestRequest.GroupID) remains unwired -- see this file's
// own doc comment on CreateChangeRequestFromServiceNow. Filtering search
// results by it (the parsed filter array's assignmentGroupId) is also still
// unwired -- see changeRequestWhereClause's own comment.
//
// The remaining fields on the request/response contract have no
// established mapping and are always left unset rather than guessed at:
// ConfigurationItemID (no CMDB table exists at all in this schema); GroupID
// (the create-time field distinct from the above -- still unwired, for the
// same reason); ApprovedBy/ApprovedOn on domain.ChangeRequest (there is a
// summary change_request.approval enum but no approver/date columns);
// Environments/DeploymentProducts/Labels/Deployments (no M2M join table
// exists for any of the four).
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
	// Only fields with an unambiguous, already-established column/enum
	// mapping are written. Deliberately NOT applied, for the same
	// no-backing-column/no-confirmed-mapping reasons this file's own
	// package doc comment and changeRequestWhereClause's already give:
	// req.ConfigurationItemID (no CMDB table), req.GroupID (no
	// assignment-group mapping established for change_request -- see this
	// file's own package doc comment on AssignedTeamID), req.Category (four
	// of ChangeRequestCategory's thirteen values -- RegularReleaseCloud/
	// HotfixReleaseCloud/DevOps/CloudComputing -- have no
	// change_request_category_enum label, and PatchChangeRequest itself
	// does not attempt this mapping either), req.EnvironmentIDs/
	// req.DeploymentProductIDs (no M2M join tables exist for either), and
	// req.Comment/req.WorkNote (ServiceNow journal entries, no backing
	// column).
	CreateChangeRequestFromServiceNow(ctx context.Context, req domain.CreateChangeRequestRequest, id, number, createdBy string) (domain.CreateChangeRequestResponse, error)
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
	// status = 'requested' to decision ("approved"/"rejected", validated by
	// the caller before this is reached), stamping actorEmail as updated_by,
	// and returns that row's id. Returns a NotFoundError if no such row
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
}

// NewChangeRequestRepository constructs a ChangeRequestRepository backed by the given connection pool.
func NewChangeRequestRepository(db *Scoped) ChangeRequestRepository {
	return &changeRequestRepo{db: db}
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
	cr.start_on, cr.end_on, cr.impact::TEXT, cr.state::TEXT, cr.change_model::TEXT,
	wi.created_on, wi.updated_on,
	ag.id, ag.name`

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

// ChangeRequestTypeSupported reports whether t has a change_model label,
// i.e. whether CreateChangeRequestFromServiceNow can persist it. Exported so
// the service layer can reject an unsupported type before, not after, the
// ServiceNow-first create -- see createChangeRequestSNFirst's own comment.
func ChangeRequestTypeSupported(t domain.ChangeRequestType) bool {
	_, ok := changeRequestTypeToChangeModel[t]
	return ok
}

// changeRequestForwardNextStates is the confirmed forward move(s) out of each
// non-terminal change_request state. Values, not just keys, are
// domain.ChangeRequestState so a typo here is a compile error, not a typo
// that silently offers a nonexistent state.
//
// This is not a guess for most of these edges: each was read directly off a
// real change_request in that exact state on the live production
// backing-system instance, via its own "state" field's dropdown (which ServiceNow itself
// populates with only the choices it currently considers legal for that
// record) -- Assess only ever offered "Authorize", Scheduled only ever
// offered "Implement", and so on. CustomerApproval's and CustomerReview's
// own outgoing rows are the confirmed exception -- see the paragraph below.
//
// Authorize and Review are the two states with more than one confirmed
// forward move, and this was found the hard way: an initial version of this
// map picked a single "common case" edge for each (Authorize->Scheduled,
// Review->Closed), reasoning that domain.ChangeRequest.HasCustomerApproved/
// HasCustomerReviewed record whether the customer HAS already signed off,
// not whether a given change request REQUIRES that gate, so they can't be
// used to decide the branch. That reasoning about the two booleans still
// holds, but checking several more real records directly disproved the
// "there's one common case" assumption it was resting on: two Authorize-state
// records with no other visible difference in the fields this schema exposes
// (same type, similar customer/no-customer project, both approval/review
// booleans false) had dropdowns offering Scheduled on one and Customer
// Approval on the other -- and the identical split was found for Review
// (Closed on one record, Customer Review on another, again with no
// discriminating field found). Whatever ServiceNow actually keys this
// decision on is not visible anywhere in this schema. Given that, both
// confirmed branches are listed for each of these two states rather than
// guessing which single one applies to a given record -- offering an option
// ServiceNow's own workflow would consider illegal for that specific record
// is a real, accepted risk here, matching PatchChangeRequest's own already-
// existing lack of a legal-transition check on this data source (any enum
// value is accepted and written directly); this map does not change that.
//
// In practice this risk only actually reaches an engineer for the Review
// branch: the CSM Portal's own ChangeRequestActionBar.tsx hardcodes
// "customer_approval" into its NEVER_OFFERED_TARGETS list (reached only by
// ServiceNow's own approval process, never human-enterable there, per that
// list's own doc comment) and filters it out unconditionally regardless of
// what this function returns, so Authorize's Customer Approval entry here
// is accurate data that never becomes a clickable button. "customer_review"
// carries no such exclusion, so Review's Customer Review entry does render
// as a real, selectable action.
//
// CustomerApproval/CustomerReview's own OUTGOING edges (what happens once a
// change request now sitting in one of those two states itself advances) are
// still not directly confirmed -- no change request was found sitting in
// either state despite checking specifically. Both are inferred by sequence
// position (CustomerApproval precedes Scheduled; CustomerReview precedes
// Closed) rather than guessed at randomly, but this is a real, distinct gap
// from the fully-confirmed edges above -- revisit if a real example of
// either surfaces.
var changeRequestForwardNextStates = map[domain.ChangeRequestState][]domain.ChangeRequestState{
	domain.ChangeRequestStateNew:              {domain.ChangeRequestStateAssess},
	domain.ChangeRequestStateAssess:           {domain.ChangeRequestStateAuthorize},
	domain.ChangeRequestStateAuthorize:        {domain.ChangeRequestStateScheduled, domain.ChangeRequestStateCustomerApproval},
	domain.ChangeRequestStateCustomerApproval: {domain.ChangeRequestStateScheduled},
	domain.ChangeRequestStateScheduled:        {domain.ChangeRequestStateImplement},
	domain.ChangeRequestStateImplement:        {domain.ChangeRequestStateReview},
	domain.ChangeRequestStateReview:           {domain.ChangeRequestStateClosed, domain.ChangeRequestStateCustomerReview},
	domain.ChangeRequestStateCustomerReview:   {domain.ChangeRequestStateClosed},
}

// legalChangeRequestNextStates computes domain.ChangeRequest.LegalNextStates
// for the Postgres data source, which (unlike ServiceNow) has no workflow
// engine of its own to compute this dynamically -- see
// changeRequestForwardNextStates' own doc comment for how this graph was
// derived, including the two states (Authorize, Review) with more than one
// confirmed forward move.
//
// "canceled" is offered alongside the forward move(s) from every
// non-terminal state: the Cancel Change action was available on every
// reachable state checked live, with no exception found. Rollback/Closed/
// Canceled are terminal -- nil, matching ServiceNow's own "no
// legalNextStates at all" answer for a record with no legal forward move.
func legalChangeRequestNextStates(state *string) []string {
	if state == nil {
		return nil
	}
	nexts, ok := changeRequestForwardNextStates[domain.ChangeRequestState(*state)]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(nexts)+1)
	for _, next := range nexts {
		result = append(result, string(next))
	}
	return append(result, string(domain.ChangeRequestStateCanceled))
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
		s := startOn.UTC().Format(time.RFC3339)
		v.PlannedStartOn = &s
	}
	if endOn != nil {
		s := endOn.UTC().Format(time.RFC3339)
		v.PlannedEndOn = &s
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
	// RLS remains the authorization boundary.
	where += viewerProjectHintFor(ctx, "wi")

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

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count change requests: %w", err)
		}
		return nil
	})

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
// fields ChangeRequest carries beyond SearchChangeRequestView.
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
	cr.is_customer_approved, cr.is_customer_reviewed,
	cr.implementation_plan, cr.priority::TEXT, cr.category::TEXT,
	rb.id, COALESCE(rb.name, NULLIF(TRIM(CONCAT_WS(' ', rb.first_name, rb.last_name)), '')),
	cr.affected_services, cr.affected_component, cr.rollback_duration,
	cg.id, cg.name,
	cr.change_request_type::TEXT, cr.likelihood::TEXT, cr.is_planning_visible_to_customers,
	cr.customer_updated_date_confirmation::TEXT, cr.customer_updated_on,
	cr.work_start_on, cr.work_end_on, cr.git_reference`

// changeRequestDetailJoins adds the two FK joins changeRequestDetailColumns
// needs beyond changeRequestFromJoins -- kept separate from (not folded
// into) changeRequestFromJoins since RequestedBy/CustomerGroup are detail
// -only fields (domain.ChangeRequest, not SearchChangeRequestView): folding
// these into the shared joins would cost every SearchChangeRequests/
// AggregateChangeRequests row two extra joins neither ever selects from.
const changeRequestDetailJoins = `
	LEFT JOIN "user" rb ON rb.id = cr.requested_by_user_id
	LEFT JOIN "group" cg ON cg.id = cr.customer_group_id`

// GetChangeRequestByID implements ChangeRequestRepository.
func (r *changeRequestRepo) GetChangeRequestByID(ctx context.Context, id string) (domain.ChangeRequest, error) {
	query := "SELECT " + changeRequestSelectColumns + ", " + changeRequestDetailColumns + " " +
		changeRequestFromJoins + " " + changeRequestDetailJoins + " WHERE wi.id = $1 AND wi.type = 'CHANGE_REQUEST'"

	var cr domain.ChangeRequest
	row := r.db.QueryRow(ctx, query, id)
	err := scanChangeRequestViewAndDetail(row, &cr)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ChangeRequest{}, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return domain.ChangeRequest{}, fmt.Errorf("get change request by id: %w", err)
	}
	// ApprovedBy/ApprovedOn/LegalNextStates/Environments/DeploymentProducts/
	// Labels/Deployments have no real column -- see this file's own package
	// doc comment.
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

		createdBy                                                          string
		justification, impactDescription, serviceOutage                    *string
		communicationPlan, rollbackPlan, testPlan                          *string
		isCustomerApproved, isCustomerReviewed                             *bool
		implementationPlan, priority, category                             *string
		rbID, rbName                                                       *string
		affectedServicesText, affectedComponentsText, rollbackDurationText *string
		cgID, cgName                                                       *string
		changeRequestType, likelihood                                      *string
		isPlanningVisibleToCustomers                                       *bool
		confirmCustomerUpdatedDate                                         *string
		customerUpdatedOn, workStart, workEnd                              *time.Time
		gitReference                                                       *string
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
		&createdBy, &justification, &impactDescription, &serviceOutage, &communicationPlan, &rollbackPlan, &testPlan,
		&isCustomerApproved, &isCustomerReviewed,
		&implementationPlan, &priority, &category,
		&rbID, &rbName,
		&affectedServicesText, &affectedComponentsText, &rollbackDurationText,
		&cgID, &cgName,
		&changeRequestType, &likelihood, &isPlanningVisibleToCustomers,
		&confirmCustomerUpdatedDate, &customerUpdatedOn,
		&workStart, &workEnd, &gitReference,
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
		s := startOn.UTC().Format(time.RFC3339)
		v.PlannedStartOn = &s
	}
	if endOn != nil {
		s := endOn.UTC().Format(time.RFC3339)
		v.PlannedEndOn = &s
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
	v.CreatedOn = createdOn.UTC().Format(time.RFC3339)
	v.UpdatedOn = updatedOn.UTC().Format(time.RFC3339)
	cr.SearchChangeRequestView = v
	cr.LegalNextStates = legalChangeRequestNextStates(v.State)

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
	if cgID != nil {
		cr.CustomerGroup = &domain.EntityRef{ID: *cgID, Name: stringOrEmpty(cgName)}
	}
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
	if customerUpdatedOn != nil {
		s := customerUpdatedOn.UTC().Format(time.RFC3339)
		cr.CustomerUpdatedOn = &s
	}
	if workStart != nil {
		s := workStart.UTC().Format(time.RFC3339)
		cr.WorkStart = &s
	}
	if workEnd != nil {
		s := workEnd.UTC().Format(time.RFC3339)
		cr.WorkEnd = &s
	}
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
	wiID, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) (string, error) {
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
	// New->Assess is compulsorily gated on an assigned team -- a real,
	// reported bug: this used to be a frontend-only courtesy check
	// (ChangeRequestActionBar.tsx's TARGET_BLOCKED_REASON), easily bypassed
	// by any direct API caller, and it was carried over stale from an
	// earlier, since-disproven model of this transition. Confirmed by
	// explicit product decision that assignedTeamId IS genuinely required to
	// move to Assess -- enforced here, not just in the UI, so this can never
	// be skipped. Accepts either a team supplied in this same request or one
	// already on the record (e.g. set via a prior PATCH through the Edit
	// dialog, then "Move to Assess" sent as its own separate request). The
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
			return "", &apierror.ValidationError{Msg: "assignedTeamId is required before moving a change request to Assess"}
		}
	}

	wiSets := []string{"updated_on = NOW()", "updated_by = $1"}
	wiArgs := []any{actorEmail}
	wiIdx := 2
	addWI := func(assignment string, val any) {
		wiSets = append(wiSets, fmt.Sprintf(assignment, wiIdx))
		wiArgs = append(wiArgs, val)
		wiIdx++
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
	if req.State != nil {
		addCR("state = $%d::change_request_state_enum", strings.ToUpper(string(*req.State)))
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
	if req.IsCustomerApproved != nil {
		addCR("is_customer_approved = $%d", *req.IsCustomerApproved)
	}
	if req.IsCustomerReviewed != nil {
		addCR("is_customer_reviewed = $%d", *req.IsCustomerReviewed)
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
	if req.CustomerGroupID != nil {
		if *req.CustomerGroupID == nil {
			crSets = append(crSets, "customer_group_id = NULL")
		} else {
			addCR("customer_group_id = $%d::uuid", **req.CustomerGroupID)
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

	// The moment a change request actually enters Assess, the assigned
	// team's own members are provisioned as its Assess-stage approvers --
	// by explicit product decision, so there is always someone to approve
	// once a request reaches Assess, instead of landing in the Approvals
	// tab with nobody listed. Scoped to "no approval_stage exists for this
	// work item yet" so a later no-op {state: "assess"} PATCH (or any other
	// field edit while already in Assess) can never duplicate the stage or
	// re-seed approvers for a group that may since have changed; the
	// Assess->Authorize cascade (DecideChangeRequestApproval) owns
	// everything about this stage from here on.
	if req.State != nil && strings.EqualFold(string(*req.State), "assess") {
		// approval_stage_visibility's SELECT policy (migration 0145) already
		// allows any member of the change request's own project to see this
		// count, so -- like the assigned-team check above -- this still runs
		// under the caller's own identity, no escalation needed yet.
		var existingStages int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, id).Scan(&existingStages); err != nil {
			return "", fmt.Errorf("patch change request: check existing approval stages: %w", err)
		}
		if existingStages == 0 {
			// approval_stage/approval_stage_approver's own INSERT policies
			// (migration 0145) are internal-only -- a real project member's
			// own identity cannot write either table directly. Escalating
			// here is safe: the caller's project membership for THIS change
			// request has already been confirmed by the work_item/
			// change_request writes above, under their own, unescalated
			// identity, before this point is ever reached. setCallerIdentity
			// re-sets the same session-local GUCs Scoped.InTx set at the
			// start of this transaction -- its own doc comment's "LOCAL
			// scoping" note means this takes effect for the remainder of
			// THIS transaction only, same as every other statement here.
			if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
				return "", fmt.Errorf("patch change request: escalate identity for approver provisioning: %w", err)
			}

			// team_member.group_id (distinct from its own team_id, the
			// hand-curated internal registry's own FK) is exactly what
			// identifies membership of a "group" row -- the same table
			// work_item.assignment_group_id/approval_stage.assignment_group_id
			// both reference. Populated by csm-sync-service mirroring
			// ServiceNow's own sys_user_grmember, same as every other
			// group-derived column in this schema. team_member carries no
			// RLS of its own (confirmed against rlsProtectedTables), so the
			// identity escalation above makes no difference to this read --
			// it's here only for the stage/approver INSERTs below.
			//
			// Queried -- and validated as non-empty -- BEFORE the stage is
			// created (CodeRabbit catch): creating an empty stage first and
			// finding no members after would still commit the empty stage,
			// since "no approval_stage exists yet" is exactly what gates
			// provisioning -- leaving the change request stuck in Assess
			// with an approval_stage nobody can ever decide, since a later
			// PATCH would skip provisioning entirely. DISTINCT guards
			// against team_member having no unique constraint on
			// (user_id, group_id); a duplicate row must not seed two
			// requested approver rows for the same person.
			memberRows, err := tx.Query(ctx, `SELECT DISTINCT user_id FROM team_member WHERE group_id = $1::uuid`, *effectiveAssignedTeamID)
			if err != nil {
				return "", fmt.Errorf("patch change request: list assignment group members: %w", err)
			}
			var memberIDs []string
			for memberRows.Next() {
				var uid string
				if err := memberRows.Scan(&uid); err != nil {
					memberRows.Close()
					return "", fmt.Errorf("patch change request: scan assignment group member: %w", err)
				}
				memberIDs = append(memberIDs, uid)
			}
			memberRows.Close()
			if err := memberRows.Err(); err != nil {
				return "", fmt.Errorf("patch change request: assignment group members: %w", err)
			}
			if len(memberIDs) == 0 {
				return "", &apierror.ValidationError{Msg: "the assigned team has no members to provision as Assess approvers"}
			}

			var stageID string
			if err := tx.QueryRow(ctx,
				`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id)
				 VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3::uuid)
				 RETURNING id`,
				actorEmail, id, *effectiveAssignedTeamID).Scan(&stageID); err != nil {
				return "", fmt.Errorf("patch change request: create approval stage: %w", err)
			}

			for _, uid := range memberIDs {
				if _, err := tx.Exec(ctx,
					`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, work_item_id, stage_id, approver_user_id, status)
					 VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3, $4::uuid, 'requested')`,
					actorEmail, id, stageID, uid); err != nil {
					return "", fmt.Errorf("patch change request: seed approval stage approver: %w", err)
				}
			}
		}
	}

	return wiID, nil
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
			number, subject, description, type, assigned_to_id
		)
		VALUES (
			gen_random_uuid(), NOW(), NOW(), $1, $1,
			next_portal_work_item_number(), $2, $3, 'CHANGE_REQUEST'::work_item_type_enum, $4::uuid
		)
		RETURNING id, number, subject, created_on, updated_on, created_by
	),
	inserted_change_request AS (
		INSERT INTO change_request (
			id, state, service_id, service_offering_id, impact, risk, priority, change_model,
			justification, implementation_plan, risk_impact_analysis, backout_plan, test_plan,
			start_on, end_on, requested_by_user_id, customer_group_id,
			is_planning_visible_to_customers, affected_services, affected_component, rollback_duration
		)
		SELECT id, 'NEW'::change_request_state_enum, $5::uuid, $6::uuid, $7::change_request_impact_enum, $8::change_request_risk_enum,
		       $9::change_request_priority_enum, $10::change_request_change_model_enum,
		       $11, $12, $13, $14, $15,
		       $16::text::timestamptz, $17::text::timestamptz, $18::uuid, $19::uuid,
		       $20, $21, $22, $23
		FROM inserted_work_item
		RETURNING id
	)
	SELECT iwi.id, iwi.number, iwi.subject, iwi.created_on, iwi.updated_on, iwi.created_by
	FROM inserted_work_item iwi
	JOIN inserted_change_request icr ON icr.id = iwi.id`

// CreateChangeRequest implements ChangeRequestRepository.
func (r *changeRequestRepo) CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest, createdBy string) (domain.CreateChangeRequestResponse, error) {
	var changeModel *string
	if req.Type != nil {
		v, ok := changeRequestTypeToChangeModel[*req.Type]
		if !ok {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("type %q is not supported on the PostgreSQL data source", *req.Type)}
		}
		changeModel = &v
	}

	var impact, risk, priority *string
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

	var (
		outID, outNumber, outSubject, outCreatedBy string
		outCreatedOn, outUpdatedOn                 time.Time
	)
	err := r.db.QueryRow(ctx, createChangeRequestPortalQuery,
		createdBy, req.Subject, req.Description, req.AssignedEngineerID,
		req.ServiceID, req.ServiceOfferingID, impact, risk, priority, changeModel,
		req.Justification, req.ImplementationPlan, req.RiskImpactAnalysis, req.BackoutPlan, req.TestPlan,
		req.PlannedStartDate, req.PlannedEndDate, req.RequestedByID, req.CustomerGroupID,
		req.IsPlanningVisibleToCustomers, req.AffectedServicesText, req.AffectedComponentsText, req.RollbackDurationText,
	).Scan(&outID, &outNumber, &outSubject, &outCreatedOn, &outUpdatedOn, &outCreatedBy)
	if err != nil {
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

	resp := domain.CreateChangeRequestResponse{Message: "Change request created successfully."}
	resp.ChangeRequest.ID = outID
	resp.ChangeRequest.Number = outNumber
	resp.ChangeRequest.CreatedOn = outCreatedOn.UTC().Format(time.RFC3339)
	resp.ChangeRequest.CreatedBy = outCreatedBy
	return resp, nil
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
			number, subject, description, type, assigned_to_id
		)
		VALUES (
			$1, NOW(), NOW(), $2, $2,
			$3, $4, $5, 'CHANGE_REQUEST'::work_item_type_enum, $6::uuid
		)
		RETURNING id, number, subject, created_on, updated_on, created_by
	),
	inserted_change_request AS (
		INSERT INTO change_request (
			id, state, service_id, service_offering_id, impact, risk, priority, change_model,
			justification, implementation_plan, risk_impact_analysis, backout_plan, test_plan,
			start_on, end_on, requested_by_user_id, customer_group_id,
			is_planning_visible_to_customers, affected_services, affected_component, rollback_duration
		)
		VALUES (
			$1, 'NEW'::change_request_state_enum, $7::uuid, $8::uuid, $9::change_request_impact_enum, $10::change_request_risk_enum,
			$11::change_request_priority_enum, $12::change_request_change_model_enum,
			$13, $14, $15, $16, $17,
			$18::text::timestamptz, $19::text::timestamptz, $20::uuid, $21::uuid,
			$22, $23, $24, $25
		)
		RETURNING id
	)
	SELECT iwi.id, iwi.number, iwi.subject, iwi.created_on, iwi.updated_on, iwi.created_by
	FROM inserted_work_item iwi
	JOIN inserted_change_request icr ON icr.id = iwi.id`

// CreateChangeRequestFromServiceNow implements ChangeRequestRepository.
func (r *changeRequestRepo) CreateChangeRequestFromServiceNow(ctx context.Context, req domain.CreateChangeRequestRequest, id, number, createdBy string) (domain.CreateChangeRequestResponse, error) {
	var changeModel *string
	if req.Type != nil {
		v, ok := changeRequestTypeToChangeModel[*req.Type]
		if !ok {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("type %q is not supported on the PostgreSQL data source", *req.Type)}
		}
		changeModel = &v
	}

	var impact, risk, priority *string
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

	var (
		outID, outNumber, outSubject, outCreatedBy string
		outCreatedOn, outUpdatedOn                 time.Time
	)
	// WithSystemIdentity: change_request's INSERT policy (migration 0145)
	// is internal-only -- this insert never sets a project_id (see this
	// file's own package doc comment on CreateChangeRequestFromServiceNow),
	// so there is nothing to check project membership against regardless of
	// who issued the original HTTP request, and the insert only ever runs
	// after ServiceNow's own workflow has already accepted the create --
	// treat it as the trusted, already-authorized system operation it is
	// rather than inheriting whatever identity happened to be on ctx.
	ctx = WithSystemIdentity(ctx)
	err := r.db.QueryRow(ctx, createChangeRequestFromServiceNowQuery,
		id, createdBy,
		number, req.Subject, req.Description, req.AssignedEngineerID,
		req.ServiceID, req.ServiceOfferingID, impact, risk, priority, changeModel,
		req.Justification, req.ImplementationPlan, req.RiskImpactAnalysis, req.BackoutPlan, req.TestPlan,
		req.PlannedStartDate, req.PlannedEndDate, req.RequestedByID, req.CustomerGroupID,
		req.IsPlanningVisibleToCustomers, req.AffectedServicesText, req.AffectedComponentsText, req.RollbackDurationText,
	).Scan(&outID, &outNumber, &outSubject, &outCreatedOn, &outUpdatedOn, &outCreatedBy)
	if err != nil {
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

	resp := domain.CreateChangeRequestResponse{Message: "Change request created successfully."}
	resp.ChangeRequest.ID = outID
	resp.ChangeRequest.Number = outNumber
	resp.ChangeRequest.CreatedOn = outCreatedOn.UTC().Format(time.RFC3339)
	resp.ChangeRequest.CreatedBy = outCreatedBy
	return resp, nil
}

// changeRequestApprovalStagesQuery backs GetChangeRequestApprovals' first of
// two flat queries -- see that method's own doc comment for why this isn't
// one three-way join. Ordered by created_on (then id as a stable tie-break
// for rows inserted in the same instant, e.g. a backfill) since
// buildChangeRequestApprovals' positional stage-label derivation depends
// entirely on this ordering.
const changeRequestApprovalStagesQuery = `
	SELECT ast.id, g.name
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
	       asa.status, asa.created_on, asa.updated_on, asa.comments
	FROM approval_stage_approver asa
	LEFT JOIN "user" u ON u.id = asa.approver_user_id
	WHERE asa.work_item_id = $1
	ORDER BY asa.created_on ASC, asa.id ASC`

// changeRequestApprovalStageRow is one row of changeRequestApprovalStagesQuery.
type changeRequestApprovalStageRow struct {
	id                  string
	assignmentGroupName *string
}

// changeRequestApprovalApproverRow is one row of
// changeRequestApprovalApproversQuery. rawStatus/stageID are nullable
// pointers because both approval_stage_approver.status and .stage_id are
// (migration 0089's own comment on nullable FKs throughout, plus status
// having no NOT NULL/DEFAULT). approverUserID is nullable because the LEFT
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
}

// GetChangeRequestApprovals implements ChangeRequestRepository.
func (r *changeRequestRepo) GetChangeRequestApprovals(ctx context.Context, id string) (domain.ChangeRequestApprovals, error) {
	stageRows, err := r.db.Query(ctx, changeRequestApprovalStagesQuery, id)
	if err != nil {
		return domain.ChangeRequestApprovals{}, fmt.Errorf("get change request approvals: query stages: %w", err)
	}
	var stages []changeRequestApprovalStageRow
	for stageRows.Next() {
		var st changeRequestApprovalStageRow
		if err := stageRows.Scan(&st.id, &st.assignmentGroupName); err != nil {
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
		if err := approverRows.Scan(&ap.id, &ap.stageID, &ap.approverUserID, &ap.approverName, &ap.rawStatus, &ap.createdOn, &ap.updatedOn, &ap.comments); err != nil {
			approverRows.Close()
			return domain.ChangeRequestApprovals{}, fmt.Errorf("get change request approvals: scan approver: %w", err)
		}
		approvers = append(approvers, ap)
	}
	approverRows.Close()
	if err := approverRows.Err(); err != nil {
		return domain.ChangeRequestApprovals{}, fmt.Errorf("get change request approvals: approvers: %w", err)
	}

	return buildChangeRequestApprovals(stages, approvers), nil
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

// changeRequestApprovalStatusByRaw normalizes approval_stage_approver.status
// (a ServiceNow sysapproval_approver.state passthrough -- migration 0089's
// own comment) to the UPPER_SNAKE_CASE values domain.ChangeRequestApprover.
// Status already carries for the ServiceNow data source (see
// snChangeRequestService.GetChangeRequestApprovals, which passes ServiceNow's
// own already-uppercase values straight through) -- this is the Postgres
// equivalent of that pass-through, applied to SN's raw lowercase state
// strings instead.
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
func buildChangeRequestApprovals(stages []changeRequestApprovalStageRow, approvers []changeRequestApprovalApproverRow) domain.ChangeRequestApprovals {
	approversByStage := make(map[string][]changeRequestApprovalApproverRow, len(stages))
	for _, ap := range approvers {
		if ap.stageID == nil {
			continue
		}
		approversByStage[*ap.stageID] = append(approversByStage[*ap.stageID], ap)
	}

	result := make([]domain.ChangeRequestApproval, 0, len(stages))
	for pos, st := range stages {
		label, approverType := changeRequestApprovalStagePosition(pos)

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
			// or csm-sync-service's own mapper moves status away from
			// "requested", so it doubles as the response timestamp once a
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

		result = append(result, domain.ChangeRequestApproval{
			Stage:        label,
			ApproverType: approverType,
			ApproverName: stringOrEmpty(st.assignmentGroupName),
			Status:       stageStatus,
			Approvers:    domainApprovers,
		})
	}

	return domain.ChangeRequestApprovals{Approvals: result}
}

// decideChangeRequestApprovalQuery backs DecideChangeRequestApproval. The
// WHERE clause's status = 'requested' is the entire enforcement of "only the
// caller's own PENDING approval can be decided" -- see that method's own
// doc comment. stage_id is also returned (nullable, same as everywhere else
// in this file) so the caller can re-check that stage's own overall outcome
// for the Assess->Authorize cascade below.
const decideChangeRequestApprovalQuery = `
	UPDATE approval_stage_approver
	SET status = $3, updated_on = NOW(), updated_by = $4
	WHERE work_item_id = $1 AND approver_user_id = $2 AND status = 'requested'
	RETURNING id, stage_id`

// DecideChangeRequestApproval implements ChangeRequestRepository.
//
// A real, reported gap: this used to only flip the one approval_stage_approver
// row and stop there -- nothing else on the stage was touched, and
// change_request.state never moved, which left Authorize permanently
// unreachable once the direct "Change state -> Authorize" button was removed
// (see ChangeRequestActionBar.tsx's NEVER_OFFERED_TARGETS): approving
// correctly recorded the decision, but nothing in the system ever advanced
// the record past Assess. Confirmed live against a real approval on this
// data source before this fix.
//
// An approval that resolves the stage (first-responder-wins: a single
// approval is enough once nobody on it has rejected -- the same quorum rule
// buildChangeRequestApprovals already uses at read time) now does two
// things, matching real ServiceNow's own observed behavior on a genuine
// multi-approver group:
//
//  1. Every other still-Requested approver on the same stage is moved to
//     Cancelled, not left sitting at Requested indefinitely.
//  2. If the change request is currently sitting in Assess, state advances
//     to Authorize.
//
// A rejection never does either (there is no confirmed target for a
// rejected Assess stage in this schema -- see changeRequestForwardNextStates'
// own doc comment on what is and isn't modeled), and neither does an
// approval that arrives after the record has already moved on (the
// currentState check simply no longer matches).
//
// The state cascade is deliberately scoped to Assess->Authorize only --
// Authorize's own outgoing approval gate (into Scheduled or Customer
// Approval) is a separate, deferred piece of work, so a decision on an
// Authorize-stage approver still cancels its own siblings but has no state
// cascade effect at all yet.
//
// Three correctness issues caught on CodeRabbit review of this method, all
// fixed here:
//
//  1. Concurrency: two decisions on the same change request (a concurrent
//     approval/rejection race, or two approvals racing each other's sibling
//     cancellation) were not serialized at all, so one could read a stale
//     "no rejection yet" snapshot or deadlock against the other. Every
//     decision now locks the change_request row (SELECT ... FOR UPDATE)
//     before touching any approver row, for both approvals and rejections.
//  2. change_request.state is nullable (see this file's own CLAUDE.md on
//     pre-existing NULL-state records) and used to be scanned into a plain
//     string, which would crash on such a record instead of simply leaving
//     the decision recorded with no cascade.
//  3. The cascade used to key off change_request.state == "ASSESS" alone,
//     with no check on which stage was actually being decided -- approving
//     a pending Authorize-stage approver while the record happened to still
//     read ASSESS would incorrectly advance it too. It now also confirms
//     stageID is the Assess-position stage (the earliest by created_on/id,
//     the same ordinal changeRequestApprovalStagePosition uses at read
//     time) before advancing; an Authorize-stage decision still cancels its
//     own siblings regardless.
func (r *changeRequestRepo) DecideChangeRequestApproval(ctx context.Context, id, approverUserID, decision, actorEmail string) (string, error) {
	// InTxReturning: Scoped stamps the caller identity on the transaction's
	// own session (the base branch's r.db.Begin is not available on Scoped).
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (string, error) {
		// Locks the change_request row before any approver row is touched,
		// serializing every decision against it -- approvals and rejections
		// alike -- so the hasRejection/isAssessStage checks below always see a
		// consistent snapshot and two concurrent approvals can't deadlock
		// cancelling each other's sibling rows. A change request that doesn't
		// exist yields no row here; the approver UPDATE just below still
		// produces the real pgx.ErrNoRows for that case, so this lock query's
		// own ErrNoRows is swallowed rather than treated as a fault.
		var lockedID string
		if err := tx.QueryRow(ctx, `SELECT id FROM change_request WHERE id = $1 FOR UPDATE`, id).Scan(&lockedID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("decide change request approval: lock change request: %w", err)
		}

		var approvalID string
		var stageID *string
		err := tx.QueryRow(ctx, decideChangeRequestApprovalQuery, id, approverUserID, decision, actorEmail).Scan(&approvalID, &stageID)
		if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
			return "", &apierror.NotFoundError{Msg: "no pending approval found for this change request and caller"}
		}
		if err != nil {
			return "", fmt.Errorf("decide change request approval: %w", err)
		}

		if decision == "approved" && stageID != nil {
			var hasRejection bool
			if err := tx.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM approval_stage_approver WHERE stage_id = $1 AND status = 'rejected')`,
				*stageID).Scan(&hasRejection); err != nil {
				return "", fmt.Errorf("decide change request approval: check stage rejections: %w", err)
			}
			if !hasRejection {
				// This decision resolves the stage (first-responder-wins: a
				// single approval is enough once nobody on it has rejected) --
				// cancel every other still-pending approver on the same stage,
				// matching real ServiceNow's own observed behavior: confirmed
				// live against a real 119-approver group, only the 1-2 who
				// actually responded first were left Approved, every other
				// still-Requested approver on that same group was moved to
				// Cancelled, not left sitting at Requested indefinitely.
				if _, err := tx.Exec(ctx,
					`UPDATE approval_stage_approver SET status = 'cancelled', updated_on = NOW(), updated_by = $2
				 WHERE stage_id = $1 AND status = 'requested'`,
					*stageID, actorEmail); err != nil {
					return "", fmt.Errorf("decide change request approval: cancel sibling approvers: %w", err)
				}

				// The one real change_request.state cascade this repository
				// attempts: Assess -> Authorize. Deliberately scoped this
				// narrow -- see this method's own doc comment for why
				// Authorize's own outgoing gate isn't attempted here. Gated on
				// stageID actually being the Assess-position stage (position 0,
				// same ordinal changeRequestApprovalStagePosition uses), not
				// merely on change_request.state reading ASSESS -- see this
				// method's own doc comment, point 3.
				var isAssessStage bool
				if err := tx.QueryRow(ctx, `
					SELECT NOT EXISTS (
						SELECT 1 FROM approval_stage earlier
						WHERE earlier.work_item_id = $1
						  AND (earlier.created_on, earlier.id) < (SELECT created_on, id FROM approval_stage WHERE id = $2)
					)`, id, *stageID).Scan(&isAssessStage); err != nil {
					return "", fmt.Errorf("decide change request approval: check stage position: %w", err)
				}
				if isAssessStage {
					// Nullable (see this method's own doc comment, point 2) --
					// a NULL state simply has nothing to cascade from, not a
					// scan failure.
					var currentState sql.NullString
					if err := tx.QueryRow(ctx, `SELECT state FROM change_request WHERE id = $1`, id).Scan(&currentState); err != nil {
						return "", fmt.Errorf("decide change request approval: check current state: %w", err)
					}
					if currentState.Valid && currentState.String == "ASSESS" {
						if _, err := tx.Exec(ctx, `UPDATE change_request SET state = 'AUTHORIZE' WHERE id = $1`, id); err != nil {
							return "", fmt.Errorf("decide change request approval: advance state: %w", err)
						}
					}
				}
			}
		}

		return approvalID, nil
	})
}

// changeRequestCategoryPGLabels is change_request_category_enum's label set
// (migration 0043). The domain enum carries four more values (regular/hotfix
// release cloud, devops, cloud computing) with no label here.
var changeRequestCategoryPGLabels = map[string]bool{
	"SOFTWARE": true, "NETWORK": true, "SERVICE": true, "TELECOM": true, "HARDWARE": true,
	"SYSTEM_SOFTWARE": true, "DOCUMENTATION": true, "APPLICATIONS_SOFTWARE": true, "OTHER": true,
}
