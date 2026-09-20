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
// The remaining fields on the request/response contract have no real
// column anywhere in the migrations and are always left unset rather than
// guessed at: ConfigurationItemID and GroupID (no CMDB/group tables exist
// at all in this schema); AssignedTeamID (work_item has no team FK either
// -- see the "Fixing case enum-casing..." section's own AssignedTeam note);
// ApprovedBy/ApprovedOn/LegalNextStates on domain.ChangeRequest (there is a
// summary change_request.approval enum but no approver/date columns, and
// LegalNextStates is a ServiceNow workflow-engine computation with nothing
// to derive it from here).
//
// CreateChangeRequest has no Postgres implementation at all: work_item.number
// has no DB default and no backing sequence anywhere in migrations/, the
// same blocker CaseRepository.CreateCase has -- see that method's own doc
// comment. GetChangeRequestApprovals/DecideChangeRequestApproval also have
// none: they need per-stage, per-approver approval records, and this schema
// only has one summary change_request.approval column.
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
const changeRequestDetailColumns = `
	wi.created_by, cr.justification, cr.impact_description, cr.service_outage_downtime,
	cr.communication_plan, cr.rollback_process, cr.test_plan,
	cr.is_customer_approved, cr.is_customer_reviewed`

// GetChangeRequestByID implements ChangeRequestRepository.
func (r *changeRequestRepo) GetChangeRequestByID(ctx context.Context, id string) (domain.ChangeRequest, error) {
	query := "SELECT " + changeRequestSelectColumns + ", " + changeRequestDetailColumns + " " + changeRequestFromJoins + " WHERE wi.id = $1 AND wi.type = 'CHANGE_REQUEST'"

	var cr domain.ChangeRequest
	var (
		isCustomerApproved, isCustomerReviewed *bool
	)
	row := r.db.QueryRow(ctx, query, id)
	// scanChangeRequestView expects exactly its own column list; the detail
	// columns are scanned separately via a small wrapper so the two column
	// lists stay independently maintainable.
	view, err := scanChangeRequestViewAndDetail(row, &cr.CreatedBy, &cr.Justification, &cr.ImpactDescription,
		&cr.ServiceOutage, &cr.CommunicationPlan, &cr.RollbackPlan, &cr.TestPlan,
		&isCustomerApproved, &isCustomerReviewed)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ChangeRequest{}, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return domain.ChangeRequest{}, fmt.Errorf("get change request by id: %w", err)
	}
	cr.SearchChangeRequestView = view
	cr.HasCustomerApproved = isCustomerApproved != nil && *isCustomerApproved
	cr.HasCustomerReviewed = isCustomerReviewed != nil && *isCustomerReviewed
	// ApprovedBy/ApprovedOn/LegalNextStates/Type have no real column -- see
	// this file's own package doc comment.
	return cr, nil
}

// scanChangeRequestViewAndDetail scans changeRequestSelectColumns followed
// by changeRequestDetailColumns's targets in the same Scan call (a single
// row's columns must be scanned together), returning the parsed view part
// separately from the detail-only fields the caller already holds pointers
// to.
func scanChangeRequestViewAndDetail(row pgx.Row, createdBy *string, justification, impactDescription, serviceOutage, communicationPlan, rollbackPlan, testPlan **string, isCustomerApproved, isCustomerReviewed **bool) (domain.SearchChangeRequestView, error) {
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
		createdBy, justification, impactDescription, serviceOutage, communicationPlan, rollbackPlan, testPlan,
		isCustomerApproved, isCustomerReviewed,
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
	// AssignedTeamID has no real column -- see this file's own package doc comment.

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
