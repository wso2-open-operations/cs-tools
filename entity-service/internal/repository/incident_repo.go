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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// IncidentRepository defines the read operations for incident (migration
// 000058, extended by 000060), a work_item type extension (id IS
// work_item.id) -- same shared-PK pattern as "case"/change_request.
//
// IncidentView/SearchIncidentView render State/Priority/Category/Subcategory/
// ContactType/ResolutionCode as plain, unvalidated strings (per those types'
// own field comments), so reads need no enum reconciliation against
// domain.IncidentState/IncidentPriority/etc at all -- the real enum column
// text is simply passed through. Only the SEARCH FILTER path uses those
// strict domain enums (SearchIncidentsFilters.Priorities, the generic
// Filters array's "state"), and three of them have real, easy-to-miss
// mismatches against their Postgres enum's actual labels:
//   - incident_state_enum's "canceled" label is spelled with one L
//     ('CANCELED'), not domain.IncidentStateCancelled's two ("CANCELLED").
//   - incident_priority_enum has no 'PLANNING' label at all (only
//     CRITICAL/HIGH/MODERATE/LOW) -- domain.IncidentPriorityPlanning is
//     rejected with a ValidationError on this data source rather than
//     silently dropped or miscast.
//
// See incidentStateToEnum/incidentPriorityToEnum for both mappings.
//
// CreateIncident/UpdateIncident/HandOffIncidentToSpecialist have no
// Postgres implementation: CreateIncident needs work_item.number, which has
// no DB default or backing sequence anywhere in migrations/ (same blocker
// as CaseRepository.CreateCase); UpdateIncident touches several fields with
// no backing column at all (AssignmentGroupID, ConfigurationItemID,
// WatchList) alongside ones that do, and would need comment-table side
// effects for AdditionalComments/WorkNotes -- deferred as a unit rather than
// half-implemented; HandOffIncidentToSpecialist is an inherently
// ServiceNow-workflow-specific feature (moves the incident to a specialist
// group, opens a task, files a GitHub issue) with nothing in this schema to
// derive an equivalent from.
type IncidentRepository interface {
	// SearchIncidents returns a filtered, sorted, paginated slice of
	// incidents together with the total count of matching rows before
	// pagination. priorities/states are the already-mapped Postgres enum
	// label form of req.Filters.Priorities/the generic Filters array's
	// "state" entries -- mapping happens in the service layer since it can
	// return a ValidationError (e.g. for PLANNING), never here.
	SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest, priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time) ([]domain.SearchIncidentView, int, error)
	// AggregateIncidents returns server-side aggregated counts of incidents
	// per value of groupBy, capped to the top maxGroups buckets with the
	// remainder folded into the returned OthersCount.
	AggregateIncidents(ctx context.Context, req domain.SearchIncidentsRequest, priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time, groupBy string, maxGroups int) (domain.AggregateResponse, error)
	// GetIncidentByID returns the full detail of a single incident by its
	// UUID, or a NotFoundError if no matching row exists.
	GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error)
	// SearchIncidentActivities returns a paginated activity feed for an
	// incident (comments + field changes), newest first.
	SearchIncidentActivities(ctx context.Context, req domain.SearchIncidentActivitiesRequest) ([]domain.CaseActivity, int, error)
}

type incidentRepo struct {
	db *pgxpool.Pool
}

// NewIncidentRepository constructs an IncidentRepository backed by the given connection pool.
func NewIncidentRepository(db *pgxpool.Pool) IncidentRepository {
	return &incidentRepo{db: db}
}

const incidentFromJoins = `
	FROM work_item wi
	JOIN incident inc ON inc.id = wi.id
	LEFT JOIN incident_subcategory sc ON sc.id = inc.subcategory_id
	LEFT JOIN service svc ON svc.id = inc.service_id
	LEFT JOIN service_offering so ON so.id = inc.service_offering_id
	LEFT JOIN "user" caller ON caller.id = inc.caller_id
	LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
	LEFT JOIN work_item parent_wi ON parent_wi.id = wi.parent_id
	LEFT JOIN incident parent_inc ON parent_inc.id = inc.parent_incident_id
	LEFT JOIN work_item parent_inc_wi ON parent_inc_wi.id = parent_inc.id
	LEFT JOIN change_request cr ON cr.id = inc.change_request_id
	LEFT JOIN work_item cr_wi ON cr_wi.id = cr.id
	LEFT JOIN change_request caused_by_cr ON caused_by_cr.id = inc.caused_by_id
	LEFT JOIN work_item caused_by_wi ON caused_by_wi.id = caused_by_cr.id
	LEFT JOIN problem prob ON prob.id = inc.problem_id
	LEFT JOIN work_item prob_wi ON prob_wi.id = prob.id
	LEFT JOIN "user" rb ON rb.id = inc.resolved_by_id`

func incidentWhereClause(f domain.SearchIncidentsFilters, priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time) (string, []any) {
	where := "WHERE wi.type = 'INCIDENT'"
	args := []any{}
	argIdx := 1

	add := func(clause string, val any) {
		where += fmt.Sprintf(" AND "+clause, argIdx)
		args = append(args, val)
		argIdx++
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
	if len(priorities) > 0 {
		add("inc.priority = ANY($%d::text[]::incident_priority_enum[])", priorities)
	}
	if len(f.ParentIDs) > 0 {
		add("wi.parent_id = ANY($%d::uuid[])", f.ParentIDs)
	}
	if len(states) > 0 {
		add("inc.state = ANY($%d::text[]::incident_state_enum[])", states)
	}
	if len(serviceIDs) > 0 {
		add("inc.service_id = ANY($%d::uuid[])", serviceIDs)
	}
	if len(assignedUserIDs) > 0 {
		add("wi.assigned_to_id = ANY($%d::uuid[])", assignedUserIDs)
	}
	if madeSla != nil {
		add("inc.is_sla_met = $%d", *madeSla)
	}
	if slaViolated != nil {
		if *slaViolated {
			where += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM sla WHERE sla.work_item_id = wi.id AND sla.has_breached = true)")
		} else {
			where += fmt.Sprintf(" AND NOT EXISTS (SELECT 1 FROM sla WHERE sla.work_item_id = wi.id AND sla.has_breached = true)")
		}
	}
	if createdStartDate != nil {
		add("wi.created_on >= $%d", *createdStartDate)
	}
	if createdEndDate != nil {
		add("wi.created_on <= $%d", *createdEndDate)
	}
	// assignmentGroupId/productName have no backing column or confirmed
	// mapping -- deliberately not applied here, matching
	// changeRequestWhereClause's own precedent for the identical class of
	// gap. businessServiceId itself IS applied above, mapped to
	// inc.service_id -- "business service" is ServiceNow's own name for
	// what this schema calls "service" (service_offering.parent_id in
	// service_offering_repo.go already makes the same identification).

	return where, args
}

const incidentSelectColumns = `
	wi.id, wi.number, wi.subject, inc.opened_on,
	caller.id, COALESCE(caller.name, NULLIF(TRIM(CONCAT_WS(' ', caller.first_name, caller.last_name)), '')),
	inc.priority::TEXT, inc.state::TEXT, inc.category::TEXT,
	parent_wi.id, parent_wi.number,
	parent_inc.id, parent_inc_wi.number,
	ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
	wi.created_on, wi.created_by, wi.updated_on, wi.updated_by`

func scanSearchIncidentView(row interface{ Scan(...any) error }) (domain.SearchIncidentView, error) {
	var (
		id, number, subject          string
		openedOn                     *time.Time
		callerID, callerName         *string
		priority, state, category    *string
		parentID, parentNumber       *string
		parentIncID, parentIncNumber *string
		aeID, aeName                 *string
		createdOn, updatedOn         time.Time
		createdBy, updatedBy         string
	)
	if err := row.Scan(
		&id, &number, &subject, &openedOn,
		&callerID, &callerName,
		&priority, &state, &category,
		&parentID, &parentNumber,
		&parentIncID, &parentIncNumber,
		&aeID, &aeName,
		&createdOn, &createdBy, &updatedOn, &updatedBy,
	); err != nil {
		return domain.SearchIncidentView{}, err
	}
	v := domain.SearchIncidentView{
		ID: &id, Number: &number, Subject: &subject,
		Priority: priority, State: state, Category: category,
		CreatedOn: createdOn.UTC().Format(time.RFC3339), CreatedBy: createdBy,
		UpdatedOn: updatedOn.UTC().Format(time.RFC3339), UpdatedBy: updatedBy,
	}
	if openedOn != nil {
		s := openedOn.UTC().Format(time.RFC3339)
		v.OpenedOn = &s
	}
	if callerID != nil {
		v.Caller = &domain.EntityRef{ID: *callerID, Name: stringOrEmpty(callerName)}
	}
	if parentID != nil {
		v.Parent = &domain.EntityRef{ID: *parentID, Name: stringOrEmpty(parentNumber)}
	}
	if parentIncID != nil {
		v.ParentIncident = &domain.EntityRef{ID: *parentIncID, Name: stringOrEmpty(parentIncNumber)}
	}
	if aeID != nil {
		v.AssignedTo = &domain.EntityRef{ID: *aeID, Name: stringOrEmpty(aeName)}
	}
	return v, nil
}

// SearchIncidents implements IncidentRepository.
func (r *incidentRepo) SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest, priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time) ([]domain.SearchIncidentView, int, error) {
	where, args := incidentWhereClause(req.Filters, priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStartDate, createdEndDate)

	sortCol := "wi.created_on"
	switch req.SortBy.Field {
	case domain.IncidentSortFieldUpdatedOn:
		sortCol = "wi.updated_on"
	case domain.IncidentSortFieldOpenedOn:
		sortCol = "inc.opened_on"
	}
	sortDir := "DESC"
	if req.SortBy.Order == domain.IncidentSortOrderAsc {
		sortDir = "ASC"
	}

	countQuery := "SELECT COUNT(*) " + incidentFromJoins + " " + where
	dataQuery := fmt.Sprintf(
		`SELECT %s %s %s ORDER BY %s %s, wi.id LIMIT $%d OFFSET $%d`,
		incidentSelectColumns, incidentFromJoins, where, sortCol, sortDir, len(args)+1, len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var views []domain.SearchIncidentView

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count incidents: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query incidents: %w", err)
		}
		defer rows.Close()

		result := make([]domain.SearchIncidentView, 0, req.Pagination.Limit)
		for rows.Next() {
			v, err := scanSearchIncidentView(rows)
			if err != nil {
				return fmt.Errorf("scan incident: %w", err)
			}
			result = append(result, v)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate incidents: %w", err)
		}
		views = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return views, total, nil
}

// incidentAggregateColumns maps a groupBy value to the real column/cast
// used to group by it -- the three values openapi.yaml's
// AggregateIncidentsRequest.groupBy allows are "state"/"assignmentGroup"/
// "businessService". "assignmentGroup" is deliberately absent (no backing
// column); "businessService" maps to inc.service_id, the same
// businessServiceId/service_id identification incidentWhereClause's own
// doc comment makes for the filter of the same name.
var incidentAggregateColumns = map[string]string{
	"state":           "inc.state::TEXT",
	"businessService": "inc.service_id::TEXT",
}

// AggregateIncidents implements IncidentRepository.
func (r *incidentRepo) AggregateIncidents(ctx context.Context, req domain.SearchIncidentsRequest, priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time, groupBy string, maxGroups int) (domain.AggregateResponse, error) {
	col, ok := incidentAggregateColumns[groupBy]
	if !ok {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy=" + groupBy + " is not supported on the PostgreSQL data source"}
	}

	where, args := incidentWhereClause(req.Filters, priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStartDate, createdEndDate)

	query := fmt.Sprintf(`
		SELECT %s AS bucket, COUNT(*) AS bucket_count
		%s %s AND %s IS NOT NULL
		GROUP BY %s
		ORDER BY bucket_count DESC, bucket`, col, incidentFromJoins, where, col, col)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("aggregate incidents: %w", err)
	}
	defer rows.Close()

	var buckets []domain.AggregateBucket
	var totalRecords int
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return domain.AggregateResponse{}, fmt.Errorf("scan incident bucket: %w", err)
		}
		lowerKey := strings.ToLower(key)
		buckets = append(buckets, domain.AggregateBucket{Key: lowerKey, Label: lowerKey, Count: count})
		totalRecords += count
	}
	if err := rows.Err(); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("iterate incident buckets: %w", err)
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

// GetIncidentByID implements IncidentRepository.
func (r *incidentRepo) GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error) {
	query := `
		SELECT wi.id, wi.number, wi.subject, inc.opened_on,
		       caller.id, COALESCE(caller.name, NULLIF(TRIM(CONCAT_WS(' ', caller.first_name, caller.last_name)), '')),
		       inc.priority::TEXT, inc.state::TEXT, inc.category::TEXT, sc.label,
		       parent_wi.id, parent_wi.number,
		       parent_inc.id, parent_inc_wi.number,
		       ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
		       svc.id, svc.name, so.id, so.name,
		       inc.contact_type::TEXT,
		       inc.impact::TEXT, inc.urgency::TEXT,
		       cr.id, cr_wi.number, prob.id, prob_wi.number,
		       caused_by_cr.id, caused_by_wi.number,
		       inc.resolution_code::TEXT, inc.close_notes,
		       rb.id, COALESCE(rb.name, NULLIF(TRIM(CONCAT_WS(' ', rb.first_name, rb.last_name)), '')),
		       inc.resolved_on, inc.incident_report,
		       wi.created_on, wi.created_by, wi.updated_on, wi.updated_by
		` + incidentFromJoins + `
		WHERE wi.id = $1 AND wi.type = 'INCIDENT'`

	var (
		id2, number, subject               string
		openedOn                           *time.Time
		callerID, callerName               *string
		priority, state, category, subcatL *string
		parentID, parentNumber             *string
		parentIncID, parentIncNumber       *string
		aeID, aeName                       *string
		svcID, svcName, soID, soName       *string
		contactType                        *string
		impact, urgency                    *string
		crID, crNumber, probID, probNumber *string
		causedByID, causedByNumber         *string
		resolutionCode, closeNotes         *string
		rbID, rbName                       *string
		resolvedOn                         *time.Time
		incidentReport                     *string
		createdOn, updatedOn               time.Time
		createdBy, updatedBy               string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&id2, &number, &subject, &openedOn,
		&callerID, &callerName,
		&priority, &state, &category, &subcatL,
		&parentID, &parentNumber,
		&parentIncID, &parentIncNumber,
		&aeID, &aeName,
		&svcID, &svcName, &soID, &soName,
		&contactType,
		&impact, &urgency,
		&crID, &crNumber, &probID, &probNumber,
		&causedByID, &causedByNumber,
		&resolutionCode, &closeNotes,
		&rbID, &rbName,
		&resolvedOn, &incidentReport,
		&createdOn, &createdBy, &updatedOn, &updatedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IncidentView{}, &apierror.NotFoundError{Msg: "incident not found"}
	}
	if err != nil {
		return domain.IncidentView{}, fmt.Errorf("get incident: %w", err)
	}

	v := domain.IncidentView{
		ID: &id2, Number: &number, Subject: &subject,
		Priority: priority, State: state, Category: category, Subcategory: subcatL,
		ContactType: contactType, Impact: impact, Urgency: urgency,
		ResolutionCode: resolutionCode, ResolutionNotes: closeNotes, IncidentReport: incidentReport,
		WatchList:             []domain.IncidentWatchListItem{},
		LinkedServiceRequests: []domain.LinkedServiceRequestRef{},
		CreatedOn:             createdOn.UTC().Format(time.RFC3339), CreatedBy: createdBy,
		UpdatedOn: updatedOn.UTC().Format(time.RFC3339), UpdatedBy: updatedBy,
		// SpecialistHandoff has no backing state anywhere in this schema --
		// no assignment-group/handoff-tracking table exists -- so it's
		// always nil here, the correct "never handed off" representation
		// per its own doc comment.
	}
	if openedOn != nil {
		s := openedOn.UTC().Format(time.RFC3339)
		v.OpenedOn = &s
	}
	if callerID != nil {
		v.Caller = &domain.EntityRef{ID: *callerID, Name: stringOrEmpty(callerName)}
	}
	if parentID != nil {
		v.Parent = &domain.EntityRef{ID: *parentID, Name: stringOrEmpty(parentNumber)}
	}
	if parentIncID != nil {
		v.ParentIncident = &domain.EntityRef{ID: *parentIncID, Name: stringOrEmpty(parentIncNumber)}
	}
	if aeID != nil {
		v.AssignedTo = &domain.EntityRef{ID: *aeID, Name: stringOrEmpty(aeName)}
	}
	if svcID != nil {
		v.Service = &domain.EntityRef{ID: *svcID, Name: stringOrEmpty(svcName)}
	}
	if soID != nil {
		v.ServiceOffering = &domain.EntityRef{ID: *soID, Name: stringOrEmpty(soName)}
	}
	if crID != nil {
		v.ChangeRequest = &domain.EntityRef{ID: *crID, Name: stringOrEmpty(crNumber)}
	}
	if probID != nil {
		v.Problem = &domain.EntityRef{ID: *probID, Name: stringOrEmpty(probNumber)}
	}
	if causedByID != nil {
		v.CausedBy = &domain.EntityRef{ID: *causedByID, Name: stringOrEmpty(causedByNumber)}
	}
	if resolvedOn != nil {
		s := resolvedOn.UTC().Format(time.RFC3339)
		v.ResolvedOn = &s
	}
	if rbID != nil {
		name := stringOrEmpty(rbName)
		v.ResolvedBy = &name
	}
	return v, nil
}

// SearchIncidentActivities implements IncidentRepository. Reuses
// scanCaseActivity's exact query/column shape (case_repo.go) -- an activity
// feed entry (comment or field change) is not inherently case-specific, and
// work_item_activity/comment are both keyed by the generic work_item_id.
// There are no incident attachments table equivalent to case_attachments
// (that table is case-specific by name and FK), so this feed never has an
// "attachment" kind entry, unlike SearchCaseActivities.
func (r *incidentRepo) SearchIncidentActivities(ctx context.Context, req domain.SearchIncidentActivitiesRequest) ([]domain.CaseActivity, int, error) {
	// Confirm req.IncidentID is actually an incident before reading its
	// activity feed -- comment/work_item_activity are both keyed by the
	// generic work_item_id with no type filter of their own, so without
	// this check a caller could pass any other work_item's UUID (a case,
	// change request, ...) through this endpoint and read that record's
	// comments/field changes instead.
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM incident WHERE id = $1)`, req.IncidentID).Scan(&exists); err != nil {
		return nil, 0, fmt.Errorf("check incident exists: %w", err)
	}
	if !exists {
		return nil, 0, &apierror.NotFoundError{Msg: "incident not found"}
	}

	includeFieldChanges := req.IncludeFieldChanges != nil && *req.IncludeFieldChanges

	countQuery := `SELECT (SELECT COUNT(*) FROM comment WHERE work_item_id = $1)`
	if includeFieldChanges {
		countQuery += ` + (SELECT COUNT(*) FROM work_item_activity WHERE work_item_id = $1)`
	}

	dataQuery := `
		WITH activity AS (
			SELECT
				c.id, 'comment' AS kind, c.content, c.created_on AS created_on,
				c.email, c.first_name, c.last_name, c.name, c.type::text AS comment_type,
				NULL::text AS file_name, NULL::text AS content_type, NULL::bigint AS size_bytes,
				NULL::text AS field_name, NULL::text AS old_value, NULL::text AS new_value
			FROM (
				SELECT DISTINCT ON (cm.id)
					cm.id, cm.content, cm.created_on, cm.created_by AS email,
					u1.first_name, u1.last_name,
					COALESCE(u1.name, NULLIF(TRIM(CONCAT_WS(' ', u1.first_name, u1.last_name)), '')) AS name,
					cm.type
				FROM comment cm
				LEFT JOIN "user" u1 ON LOWER(u1.email) = LOWER(cm.created_by)
				WHERE cm.work_item_id = $1
				ORDER BY cm.id, u1.id
			) c`
	if includeFieldChanges {
		dataQuery += `

			UNION ALL

			SELECT
				fc.id, 'field_change' AS kind, '' AS content, fc.created_on AS created_on,
				fc.email, fc.first_name, fc.last_name, fc.name,
				NULL::text AS comment_type,
				NULL::text AS file_name, NULL::text AS content_type, NULL::bigint AS size_bytes,
				fc.field_name, fc.old_value, fc.new_value
			FROM (
				SELECT DISTINCT ON (wa.id)
					wa.id, wa.created_on, wa.user_email AS email,
					u3.first_name, u3.last_name,
					COALESCE(u3.name, NULLIF(TRIM(CONCAT_WS(' ', u3.first_name, u3.last_name)), '')) AS name,
					wa.field_name, wa.old_value, wa.new_value
				FROM work_item_activity wa
				LEFT JOIN "user" u3 ON LOWER(u3.email) = LOWER(wa.user_email)
				WHERE wa.work_item_id = $1
				ORDER BY wa.id, u3.id
			) fc`
	}
	dataQuery += `
		)
		SELECT id, kind, content, created_on, email, first_name, last_name, name, comment_type, file_name, content_type, size_bytes, field_name, old_value, new_value
		FROM activity
		ORDER BY created_on DESC, id
		LIMIT $2 OFFSET $3`

	var total int
	var activity []domain.CaseActivity

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, req.IncidentID).Scan(&total); err != nil {
			return fmt.Errorf("count incident activities: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, req.IncidentID, req.Pagination.Limit, req.Pagination.Offset)
		if err != nil {
			return fmt.Errorf("query incident activities: %w", err)
		}
		defer rows.Close()

		result := make([]domain.CaseActivity, 0, req.Pagination.Limit)
		for rows.Next() {
			a, err := scanCaseActivity(rows)
			if err != nil {
				return fmt.Errorf("scan incident activity: %w", err)
			}
			result = append(result, a)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate incident activities: %w", err)
		}
		activity = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return activity, total, nil
}
