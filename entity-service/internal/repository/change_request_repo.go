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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ChangeRequestRepository defines the persistence operations for the change
// request entity, split across work_item (migration 000016, fields common
// to every work_item type) and change_request (migration 000047, a
// shared-PK extension -- change_request.id IS work_item.id, same pattern
// as "case").
//
// ServiceID/ServiceOfferingID are backed by change_request.service_id/
// service_offering_id (migration 000050), FKs into service/service_offering
// (migrations 000048/000049).
//
// Type (domain.ChangeRequestType) is backed by change_request.change_model
// (migration 000055) -- NOT change_request.change_request_type, whose real
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
// The remaining fields on the request/response contract have no
// established mapping and are always left unset rather than guessed at:
// ConfigurationItemID (no CMDB table exists at all in this schema); GroupID
// and AssignedTeamID (distinct from CustomerGroupID -- these would need
// work_item.assignment_group_id, migration 000074, which nothing in this
// file joins or reads yet); ApprovedBy/ApprovedOn/LegalNextStates on
// domain.ChangeRequest (there is a summary change_request.approval enum
// but no approver/date columns, and LegalNextStates is a ServiceNow
// workflow-engine computation with nothing to derive it from here);
// Environments/DeploymentProducts/Labels/Deployments (no M2M join table
// exists for any of the four).
//
// CreateChangeRequest has no Postgres implementation at all: work_item.number
// has no DB default and no backing sequence anywhere in migrations/, the
// same blocker CaseRepository.CreateCase has -- see that method's own doc
// comment.
//
// GetChangeRequestApprovals/DecideChangeRequestApproval ARE implemented
// against approval_stage/approval_stage_approver (migration 000087), which
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
	// accepted for interface symmetry with that parsed result but never
	// applied: there is no column anywhere in this schema for it (see this
	// file's own package doc comment).
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
	// CreateChangeRequestFromServiceNow inserts a new change request row
	// (both work_item and change_request), for
	// DATA_SOURCE=postgres-servicenow-dual-write's SN-first change request
	// creation (see changeRequestService.createChangeRequestSNFirst's own doc
	// comment). Unlike CaseRepository.CreateCaseFromServiceNow, no wso2ID
	// parameter exists here: work_item.wso2_id is only required (by the
	// work_item_wso2_id_required_by_type CHECK constraint, migration 000016)
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
	// change_request.state is deliberately left NULL (the column has no
	// NOT NULL/DEFAULT, unlike incident_state_enum's NOT NULL DEFAULT
	// 'NEW'): snCreateChangeRequestResponse carries no state field at all,
	// so unlike req.Category/Priority/Risk/Impact (plain request-supplied
	// values ServiceNow's create payload already forwards verbatim and this
	// method can echo back with equal confidence), the state ServiceNow's
	// workflow engine actually assigned after evaluating req.State (if any)
	// is never confirmed by the response -- writing req.State straight
	// through would risk recording a value ServiceNow silently overrode.
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
	// doc comment).
	DecideChangeRequestApproval(ctx context.Context, id, approverUserID, decision, actorEmail string) (string, error)
}

type changeRequestRepo struct {
	db *pgxpool.Pool
}

// NewChangeRequestRepository constructs a ChangeRequestRepository backed by the given connection pool.
func NewChangeRequestRepository(db *pgxpool.Pool) ChangeRequestRepository {
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
	LEFT JOIN work_item origin_case ON origin_case.id = wi.parent_id`

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
	wi.created_on, wi.updated_on`

// changeRequestChangeModelToType/changeRequestTypeToChangeModel map between
// change_request.change_model's real enum labels (migration 000055) and
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
	// The parsed filter array's assignmentGroupId has no corresponding
	// column anywhere in this schema (see this file's own package doc
	// comment) and is deliberately not applied here.

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
}

// changeRequestPatchCRFKField mirrors changeRequestPatchFKField for the
// change_request table's own FK columns (migration 000050).
var changeRequestPatchCRFKField = map[string]string{
	"change_request_service_id_fkey":          "serviceId",
	"change_request_service_offering_id_fkey": "serviceOfferingId",
}

// PatchChangeRequest implements ChangeRequestRepository.
func (r *changeRequestRepo) PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest, actorEmail string) (domain.ChangeRequest, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.ChangeRequest{}, fmt.Errorf("patch change request: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

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
	// AssignedTeamID has no wired mapping here -- see this file's own package doc comment.

	wiArgs = append(wiArgs, id)
	wiQuery := fmt.Sprintf(`UPDATE work_item SET %s WHERE id = $%d AND type = 'CHANGE_REQUEST' RETURNING id`, strings.Join(wiSets, ", "), wiIdx)
	var wiID string
	if err := tx.QueryRow(ctx, wiQuery, wiArgs...).Scan(&wiID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ChangeRequest{}, &apierror.NotFoundError{Msg: "change request not found"}
		}
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			field := changeRequestPatchFKField[pgErr.ConstraintName]
			if field == "" {
				field = "one or more referenced fields"
			}
			return domain.ChangeRequest{}, &apierror.ValidationError{Msg: field + " does not refer to an existing record"}
		}
		return domain.ChangeRequest{}, fmt.Errorf("patch change request work_item: %w", err)
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
			// (migration 000055) and have no real enum label there --
			// see changeRequestChangeModelToType's own doc comment.
			return domain.ChangeRequest{}, &apierror.ValidationError{Msg: fmt.Sprintf("type %q is not supported on the PostgreSQL data source", *req.Type)}
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
	if req.RequestApproval != nil && *req.RequestApproval {
		addCR("approval = $%d::change_request_approval_enum", "REQUESTED")
	}
	// Type has no real mapping -- see this file's own package doc comment.

	if len(crSets) > 0 {
		crArgs = append(crArgs, id)
		crQuery := fmt.Sprintf(`UPDATE change_request SET %s WHERE id = $%d`, strings.Join(crSets, ", "), crIdx)
		if _, err := tx.Exec(ctx, crQuery, crArgs...); err != nil {
			if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
				field := changeRequestPatchCRFKField[pgErr.ConstraintName]
				if field == "" {
					field = "one or more referenced fields"
				}
				return domain.ChangeRequest{}, &apierror.ValidationError{Msg: field + " does not refer to an existing record"}
			}
			return domain.ChangeRequest{}, fmt.Errorf("patch change request: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ChangeRequest{}, fmt.Errorf("patch change request: commit tx: %w", err)
	}

	return r.GetChangeRequestByID(ctx, wiID)
}

// createChangeRequestFromServiceNowQuery inserts both halves of a change
// request row (work_item + change_request, the same shared-primary-key
// pattern createCaseFromServiceNowQuery/createIncidentFromServiceNowQuery
// document) in one round trip via a CTE, using caller-supplied identity
// (id/number/createdBy) rather than generating any of it -- see
// CreateChangeRequestFromServiceNow's own doc comment for why, and for which
// req fields are deliberately left unwritten. type is hardcoded to
// 'CHANGE_REQUEST'::work_item_type_enum. change_request.state is left NULL
// -- see CreateChangeRequestFromServiceNow's own doc comment for why, unlike
// incident's reliance on a NOT NULL DEFAULT column.
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
			id, service_id, service_offering_id, impact, risk, priority, change_model,
			justification, implementation_plan, risk_impact_analysis, backout_plan, test_plan,
			start_on, end_on, requested_by_user_id, customer_group_id,
			is_planning_visible_to_customers, affected_services, affected_component, rollback_duration
		)
		VALUES (
			$1, $7::uuid, $8::uuid, $9::change_request_impact_enum, $10::change_request_risk_enum,
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
// onto approval_stage_approver, migration 000087's own comment on why)
// rather than joining through approval_stage, same reasoning as that
// column's own comment.
const changeRequestApprovalApproversQuery = `
	SELECT asa.id, asa.stage_id,
	       COALESCE(NULLIF(TRIM(u.name), ''), NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), '') AS approver_name,
	       asa.status, asa.updated_on
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
// (migration 000087's own comment on nullable FKs throughout, plus status
// having no NOT NULL/DEFAULT).
type changeRequestApprovalApproverRow struct {
	id           string
	stageID      *string
	approverName string
	rawStatus    *string
	updatedOn    time.Time
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
		if err := approverRows.Scan(&ap.id, &ap.stageID, &ap.approverName, &ap.rawStatus, &ap.updatedOn); err != nil {
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
// (a ServiceNow sysapproval_approver.state passthrough -- migration 000087's
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

			domainApprovers = append(domainApprovers, domain.ChangeRequestApprover{
				ID:          ap.id,
				Name:        ap.approverName,
				Status:      status,
				RespondedOn: respondedOn,
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
// doc comment.
const decideChangeRequestApprovalQuery = `
	UPDATE approval_stage_approver
	SET status = $3, updated_on = NOW(), updated_by = $4
	WHERE work_item_id = $1 AND approver_user_id = $2 AND status = 'requested'
	RETURNING id`

// DecideChangeRequestApproval implements ChangeRequestRepository.
func (r *changeRequestRepo) DecideChangeRequestApproval(ctx context.Context, id, approverUserID, decision, actorEmail string) (string, error) {
	var approvalID string
	err := r.db.QueryRow(ctx, decideChangeRequestApprovalQuery, id, approverUserID, decision, actorEmail).Scan(&approvalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &apierror.NotFoundError{Msg: "no pending approval found for this change request and caller"}
	}
	if err != nil {
		return "", fmt.Errorf("decide change request approval: %w", err)
	}
	return approvalID, nil
}
