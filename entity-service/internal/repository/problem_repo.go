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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ProblemRepository defines the read operations for problem (migration
// 000059), a work_item type extension (id IS work_item.id) -- same
// shared-PK pattern as "case"/change_request. domain.ProblemState's values
// (NEW/ASSESS/ROOT_CAUSE_ANALYSIS/FIX_IN_PROGRESS/RESOLVED/CLOSED) match
// problem_state_enum's own labels by identity (unlike most enum pairs in
// this codebase, no case-fold or explicit map is needed). Category,
// Subcategory, and ResolutionCode are plain, unvalidated passthrough
// strings on ProblemDetail (per that type's own doc comment) -- so unlike
// Incident's category/resolution-code, there's no domain enum to reconcile
// against the real column values at all; they're rendered as-is.
//
// The assignment group lives on work_item.assignment_group_id (migration
// 0075), the generic column every work_item type shares. Writes set it
// (UpdateProblemFields/TransitionProblem); reads and the "assignmentGroupId"
// search filter do not use it yet, so ProblemDetail carries no group.
//
// TransitionProblem ports ServiceNow's ProblemUtils._applyProblemTransition;
// see its own doc comment.
//
// CreateProblemFromServiceNow (below) is the exception, same as
// CaseRepository.CreateCaseFromServiceNow/IncidentRepository.CreateIncidentFromServiceNow:
// it backs DATA_SOURCE=postgres-servicenow-dual-write's SN-first problem
// creation, where id/number come from ServiceNow rather than being generated
// here.
type ProblemRepository interface {
	// SearchProblems returns a filtered, paginated slice of problems
	// together with the total count of matching rows before pagination.
	SearchProblems(ctx context.Context, req domain.SearchProblemsRequest, states, assignedUserIDs []string) ([]domain.SearchProblemView, int, error)
	// AggregateProblems returns server-side aggregated counts of problems
	// per value of groupBy, capped to the top maxGroups buckets with the
	// remainder folded into the returned OthersCount.
	AggregateProblems(ctx context.Context, req domain.SearchProblemsRequest, states, assignedUserIDs []string, groupBy string, maxGroups int) (domain.AggregateResponse, error)
	// GetProblem returns the full detail of a single problem by its UUID,
	// or a NotFoundError if no matching row exists.
	GetProblem(ctx context.Context, id string) (domain.ProblemDetail, error)
	// CreateProblemFromServiceNow inserts a new problem row (both work_item
	// and "problem"), for DATA_SOURCE=postgres-servicenow-dual-write's
	// SN-first problem creation (see
	// problemService.createProblemSNFirst's own doc comment). Unlike
	// CaseRepository.CreateCaseFromServiceNow, no wso2ID parameter exists
	// here: work_item.wso2_id is only required (by the
	// work_item_wso2_id_required_by_type CHECK constraint, migration 0021)
	// for CASE/SERVICE_REQUEST/ANNOUNCEMENT/ENGAGEMENT/
	// SECURITY_REPORT_ANALYSIS -- PROBLEM is deliberately excluded from that
	// list, and ServiceNow's own problem-create response
	// (snCreateProblemResponse/snProblemDetailResponse) has no equivalent
	// field to supply one from anyway.
	//
	// Unlike CreateCaseFromServiceNow/CreateIncidentFromServiceNow, createdBy
	// is NOT taken from ServiceNow's response -- snProblemDetailResponse
	// (the shape ServiceNow's problem create endpoint actually returns) has
	// no createdBy/createdOn field at all, unlike case/incident/change
	// request. The caller (problemService.createProblemSNFirst) instead
	// resolves createdBy from the requesting user's own JWT email claim
	// (same middleware.UserIDTokenFromContext + emailFromJWT chain
	// caseService.CreateCase already uses when req.CreatedBy is empty) --
	// the calling user's identity is the only real signal for who actually
	// created the problem, since ServiceNow's own response gives none.
	//
	// state is the raw, already-confirmed ServiceNow state label
	// (snProblemDetailResponse.State) if ServiceNow returned one -- unlike
	// change_request's create response, problem's DOES return a state that
	// matches problem_state_enum's own labels by identity (see this file's
	// own package doc comment), so it is safe to cast straight through
	// rather than leaving the column NULL the way
	// CreateChangeRequestFromServiceNow does.
	//
	// id must be a canonical UUID (sysidToUUID(sn sys_id)). Returns a
	// ValidationError if id is not a valid UUID or state is not a valid
	// problem_state_enum label, or a ConflictError if a row already exists
	// for id/number (unique violation) -- the latter should not happen in
	// practice since ServiceNow only just generated these, but is reported
	// precisely rather than as an opaque infrastructure error if it ever
	// does.
	//
	// req.Category is normalized to uppercase and written to
	// problem.category. req.Subcategory is matched case-insensitively against
	// problem_subcategory.value within that category; unmatched values remain
	// NULL.
	CreateProblemFromServiceNow(ctx context.Context, req domain.CreateProblemRequest, id, number, createdBy string, state *string) (domain.ProblemDetail, error)
	// CreateProblem inserts a new problem row (both work_item and "problem")
	// for the plain-Postgres data source (no ServiceNow at all) --
	// createProblemPortalQuery's own doc comment has the full field-by-field
	// reasoning, which mirrors CreateProblemFromServiceNow's exactly except
	// identity (id/number) is generated here via
	// gen_random_uuid()/next_portal_work_item_number() instead of being
	// supplied by a prior ServiceNow response, createdBy is the calling
	// user's own resolved email, and state is hardcoded to NEW (there is no
	// ServiceNow response to confirm one from, and problem.state has no
	// column default of its own -- confirmed against the live schema --
	// unlike incident's).
	CreateProblem(ctx context.Context, req domain.CreateProblemRequest, createdBy string) (domain.ProblemDetail, error)

	// UpdateProblemFields writes any subset of the PATCH /problems/{id}
	// plain fields: req.CauseNotes/FixNotes/Workaround/TargetResolutionDate
	// (problem.cause_notes/fix_notes/workaround/due_on), req.AssignedToID
	// (work_item.assigned_to_id, the same generic column
	// CaseRepository.UpdateCaseFields writes for "case") and
	// req.AssignmentGroupID (work_item.assignment_group_id). req.Transition
	// is ignored here; TransitionProblem handles it.
	//
	// work_item.updated_on/updated_by are bumped unconditionally, matching
	// UpdateCaseFields' identical convention, using actorEmail (the caller's
	// own JWT email, resolved by problemService.resolveActorEmail) as
	// updated_by.
	//
	// Returns a NotFoundError if id does not name an existing PROBLEM work
	// item, or a ValidationError if assignedToId/assignmentGroupId does not
	// reference a real user/group row (FK violation) or targetResolutionDate
	// is not a valid RFC3339 timestamp.
	UpdateProblemFields(ctx context.Context, req domain.UpdateProblemRequest, actorEmail string) (ProblemWriteResult, error)
	// TransitionProblem applies transition t to the problem, together with
	// any plain fields req carries, in one transaction. See its
	// implementation's doc comment for the ServiceNow behaviour it ports.
	//
	// Returns a NotFoundError if id does not name an existing PROBLEM work
	// item, or a ValidationError if the problem is not in t.From or the
	// transition's own guard refuses it. A write the Assess rule moved back
	// to ASSESS is committed and reported via ProblemWriteResult.Reverted,
	// not as an error.
	TransitionProblem(ctx context.Context, req domain.UpdateProblemRequest, t domain.ProblemTransition, actorEmail string) (ProblemWriteResult, error)
}

// ProblemWriteResult is what a problem write reports back.
type ProblemWriteResult struct {
	UpdatedOn time.Time
	// AssignmentGroup is the row's work_item.assignment_group_id after the
	// write, with the group's name; nil when the problem has no group.
	AssignmentGroup *domain.EntityRef
	// Reverted is set by TransitionProblem when the Assess rule kept the
	// problem in ASSESS instead of the transition's target state.
	Reverted bool
}

type problemRepo struct {
	db *Scoped
}

// NewProblemRepository constructs a ProblemRepository backed by the given connection pool.
func NewProblemRepository(db *Scoped) ProblemRepository {
	return &problemRepo{db: db}
}

const problemFromJoins = `
	FROM work_item wi
	JOIN problem pr ON pr.id = wi.id
	LEFT JOIN problem_subcategory sc ON sc.id = pr.subcategory_id
	LEFT JOIN incident primary_inc ON primary_inc.id = pr.incident_id
	LEFT JOIN work_item primary_inc_wi ON primary_inc_wi.id = primary_inc.id
	LEFT JOIN change_request cr ON cr.id = pr.change_request_id
	LEFT JOIN work_item cr_wi ON cr_wi.id = cr.id
	LEFT JOIN work_item origin_case ON origin_case.id = wi.parent_id
	LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
	LEFT JOIN "user" rb ON rb.id = pr.resolved_by_id`

func problemWhereClause(f domain.SearchProblemsFilters, states, assignedUserIDs []string) (string, []any) {
	where := "WHERE wi.type = 'PROBLEM'"
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
	if len(states) > 0 {
		add("pr.state = ANY($%d::text[]::problem_state_enum[])", states)
	}
	if len(assignedUserIDs) > 0 {
		add("wi.assigned_to_id = ANY($%d::uuid[])", assignedUserIDs)
	}
	// assignmentGroupId has no backing column -- see this file's own package
	// doc comment; deliberately not applied here.

	return where, args
}

func scanSearchProblemView(row interface{ Scan(...any) error }) (domain.SearchProblemView, error) {
	var (
		id, number, subject string
		state               *string
		aeID, aeName        *string
	)
	if err := row.Scan(&id, &number, &subject, &state, &aeID, &aeName); err != nil {
		return domain.SearchProblemView{}, err
	}
	v := domain.SearchProblemView{ID: &id, Number: &number, Subject: &subject, State: state}
	if aeID != nil {
		v.AssignedTo = &domain.EntityRef{ID: *aeID, Name: stringOrEmpty(aeName)}
	}
	return v, nil
}

// SearchProblems implements ProblemRepository.
func (r *problemRepo) SearchProblems(ctx context.Context, req domain.SearchProblemsRequest, states, assignedUserIDs []string) ([]domain.SearchProblemView, int, error) {
	where, args := problemWhereClause(req.Filters, states, assignedUserIDs)

	countQuery := "SELECT COUNT(*) " + problemFromJoins + " " + where
	dataQuery := fmt.Sprintf(
		`SELECT wi.id, wi.number, wi.subject, pr.state::TEXT, ae.id,
		        COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), ''))
		 %s %s
		 ORDER BY wi.created_on DESC, wi.id
		 LIMIT $%d OFFSET $%d`,
		problemFromJoins, where, len(args)+1, len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var views []domain.SearchProblemView

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count problems: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query problems: %w", err)
		}
		defer rows.Close()

		result := make([]domain.SearchProblemView, 0, req.Pagination.Limit)
		for rows.Next() {
			v, err := scanSearchProblemView(rows)
			if err != nil {
				return fmt.Errorf("scan problem: %w", err)
			}
			result = append(result, v)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate problems: %w", err)
		}
		views = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return views, total, nil
}

// problemAggregateColumns maps a groupBy value to the real column/cast used
// to group by it. "assignmentGroup" (the only other value openapi.yaml's
// AggregateProblemsRequest.groupBy allows) is deliberately absent -- no
// backing column, see this file's own package doc comment.
var problemAggregateColumns = map[string]string{
	"state": "pr.state::TEXT",
}

// AggregateProblems implements ProblemRepository.
func (r *problemRepo) AggregateProblems(ctx context.Context, req domain.SearchProblemsRequest, states, assignedUserIDs []string, groupBy string, maxGroups int) (domain.AggregateResponse, error) {
	col, ok := problemAggregateColumns[groupBy]
	if !ok {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy=" + groupBy + " is not supported on the PostgreSQL data source"}
	}

	where, args := problemWhereClause(req.Filters, states, assignedUserIDs)

	query := fmt.Sprintf(`
		SELECT %s AS bucket, COUNT(*) AS bucket_count
		%s %s AND %s IS NOT NULL
		GROUP BY %s
		ORDER BY bucket_count DESC, bucket`, col, problemFromJoins, where, col, col)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("aggregate problems: %w", err)
	}
	defer rows.Close()

	var buckets []domain.AggregateBucket
	var totalRecords int
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return domain.AggregateResponse{}, fmt.Errorf("scan problem bucket: %w", err)
		}
		lowerKey := strings.ToLower(key)
		buckets = append(buckets, domain.AggregateBucket{Key: lowerKey, Label: lowerKey, Count: count})
		totalRecords += count
	}
	if err := rows.Err(); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("iterate problem buckets: %w", err)
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

// GetProblem implements ProblemRepository.
func (r *problemRepo) GetProblem(ctx context.Context, id string) (domain.ProblemDetail, error) {
	query := `
		SELECT wi.id, wi.number, wi.subject, wi.description, pr.state::TEXT, pr.priority::TEXT,
		       pr.category::TEXT, sc.label,
		       origin_case.id, origin_case.number,
		       primary_inc.id, primary_inc_wi.number,
		       cr.id, cr_wi.number,
		       ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
		       pr.resolution_code::TEXT, pr.cause_notes, pr.fix_notes, pr.workaround,
		       pr.resolved_on, rb.id, COALESCE(rb.name, NULLIF(TRIM(CONCAT_WS(' ', rb.first_name, rb.last_name)), '')),
		       pr.opened_on, pr.closed_on
		` + problemFromJoins + `
		WHERE wi.id = $1 AND wi.type = 'PROBLEM'`

	var (
		id2, number, subject             string
		description                      *string
		state, priority                  *string
		category, subcategoryLabel       *string
		originCaseID, originCaseNumber   *string
		priIncID, priIncNumber           *string
		crID, crNumber                   *string
		aeID, aeName                     *string
		resolutionCode                   *string
		causeNotes, fixNotes, workaround *string
		resolvedOn                       *time.Time
		rbID, rbName                     *string
		openedOn, closedOn               *time.Time
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&id2, &number, &subject, &description, &state, &priority,
		&category, &subcategoryLabel,
		&originCaseID, &originCaseNumber,
		&priIncID, &priIncNumber,
		&crID, &crNumber,
		&aeID, &aeName,
		&resolutionCode, &causeNotes, &fixNotes, &workaround,
		&resolvedOn, &rbID, &rbName,
		&openedOn, &closedOn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProblemDetail{}, &apierror.NotFoundError{Msg: "problem not found"}
	}
	if err != nil {
		return domain.ProblemDetail{}, fmt.Errorf("get problem: %w", err)
	}

	d := domain.ProblemDetail{
		ID: &id2, Number: &number, Subject: &subject, Description: description, State: state, Priority: priority,
		Category: category, Subcategory: subcategoryLabel,
		ResolutionCode: resolutionCode, CauseNotes: causeNotes, FixNotes: fixNotes, Workaround: workaround,
	}
	if originCaseID != nil {
		d.OriginCase = &domain.CaseNumberRef{ID: *originCaseID, Number: stringOrEmpty(originCaseNumber)}
	}
	if priIncID != nil {
		d.PrimaryIncident = &domain.CaseNumberRef{ID: *priIncID, Number: stringOrEmpty(priIncNumber)}
	}
	if crID != nil {
		d.LinkedChangeRequest = &domain.CaseNumberRef{ID: *crID, Number: stringOrEmpty(crNumber)}
	}
	if aeID != nil {
		d.AssignedTo = &domain.EntityRef{ID: *aeID, Name: stringOrEmpty(aeName)}
	}
	if resolvedOn != nil {
		s := resolvedOn.UTC().Format(time.RFC3339)
		d.ResolvedOn = &s
	}
	if rbID != nil {
		d.ResolvedBy = &domain.EntityRef{ID: *rbID, Name: stringOrEmpty(rbName)}
	}
	if openedOn != nil {
		s := openedOn.UTC().Format(time.RFC3339)
		d.OpenedOn = &s
	}
	if closedOn != nil {
		s := closedOn.UTC().Format(time.RFC3339)
		d.ClosedOn = &s
	}

	// LinkedIncidents (the reverse of incident.problem_id) is a separate
	// query: problem's own row carries only its primary_incident (this
	// problem's originating incident), not the set of incidents that later
	// linked back to it.
	rows, err := r.db.Query(ctx, `
		SELECT i.id, wi2.number
		FROM incident i
		JOIN work_item wi2 ON wi2.id = i.id
		WHERE i.problem_id = $1
		ORDER BY wi2.created_on`, id2,
	)
	if err != nil {
		return domain.ProblemDetail{}, fmt.Errorf("query linked incidents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var incID, incNumber string
		if err := rows.Scan(&incID, &incNumber); err != nil {
			return domain.ProblemDetail{}, fmt.Errorf("scan linked incident: %w", err)
		}
		d.LinkedIncidents = append(d.LinkedIncidents, domain.CaseNumberRef{ID: incID, Number: incNumber})
	}
	if err := rows.Err(); err != nil {
		return domain.ProblemDetail{}, fmt.Errorf("iterate linked incidents: %w", err)
	}

	return d, nil
}

// createProblemPortalQuery is CreateProblem's (the plain-Postgres,
// caller-initiated path) query -- structurally identical to
// createProblemFromServiceNowQuery except id/number are generated here
// (gen_random_uuid()/next_portal_work_item_number(), migration 0140) instead
// of supplied by a prior ServiceNow response, and state is a fixed literal
// ('NEW') rather than a parameter -- there is no ServiceNow response to
// confirm one from on this path, and problem.state has no column default of
// its own (confirmed against the live schema, unlike incident's).
//
// Column/output order matches the trailing SELECT exactly.
const createProblemPortalQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, subject, description, type, parent_id
		)
		VALUES (
			gen_random_uuid(), NOW(), NOW(), $1, $1,
			next_portal_work_item_number(), $2, $6, 'PROBLEM'::work_item_type_enum, $3::uuid
		)
		RETURNING id, number, subject, description, created_on, updated_on, created_by
	),
	inserted_problem AS (
		INSERT INTO problem (
			id, state, incident_id, opened_on, category, subcategory_id
		)
		SELECT id, 'NEW'::problem_state_enum, $4::uuid, NOW(), $7::problem_category_enum,
		       -- subcategory is matched on problem_subcategory.value (lower-case
		       -- free text) within the chosen category; an unmatched value stays NULL.
		       (SELECT psc.id FROM problem_subcategory psc WHERE psc.category = $7::problem_category_enum AND psc.value = LOWER($5::text))
		FROM inserted_work_item
		RETURNING id
	)
	SELECT iwi.id, iwi.number, iwi.subject, iwi.description, iwi.created_on, iwi.updated_on, iwi.created_by
	FROM inserted_work_item iwi
	JOIN inserted_problem ip ON ip.id = iwi.id`

// CreateProblem implements ProblemRepository.
func (r *problemRepo) CreateProblem(ctx context.Context, req domain.CreateProblemRequest, createdBy string) (domain.ProblemDetail, error) {
	var category *string
	if req.Category != nil && strings.TrimSpace(*req.Category) != "" {
		v := strings.ToUpper(strings.TrimSpace(*req.Category))
		category = &v
	}
	var (
		outID, outNumber, outSubject, outCreatedBy string
		outDescription                             *string
		outCreatedOn, outUpdatedOn                 time.Time
	)
	err := r.db.QueryRow(ctx, createProblemPortalQuery,
		createdBy, req.Subject, req.OriginCaseID,
		req.PrimaryIncidentID, req.Subcategory, req.Description,
		category,
	).Scan(&outID, &outNumber, &outSubject, &outDescription, &outCreatedOn, &outUpdatedOn, &outCreatedBy)
	if err != nil {
		// problem_deny_all_insert (migration 0148) permits only an internal
		// caller -- problem has no project concept at all, same as incident.
		// POST /problems is already gated internalOnly at the route, so this
		// should not be reachable in practice, but map it defensively rather
		// than leaving a theoretical 42501 to surface as a raw 500.
		if IsRLSPolicyViolation(err) {
			return domain.ProblemDetail{}, &apierror.NotFoundError{Msg: "problem not found"}
		}
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "22001": // string_data_right_truncation -- e.g. subject over work_item.subject's VARCHAR(512)
				return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "a field value is too long: " + pgErr.Message}
			case "23503": // foreign_key_violation -- one of the referenced IDs does not exist
				return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			case "P0001": // raise_exception from integrity triggers
				return domain.ProblemDetail{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.ProblemDetail{}, fmt.Errorf("create problem: %w", err)
	}

	state := "NEW"
	return domain.ProblemDetail{
		ID:          &outID,
		Number:      &outNumber,
		Subject:     &outSubject,
		Description: outDescription,
		State:       &state,
	}, nil
}

// createProblemFromServiceNowQuery inserts both halves of a problem row
// (work_item + problem, the same shared-primary-key pattern
// createIncidentFromServiceNowQuery documents) in one round trip via a CTE,
// using caller-supplied identity (id/number/createdBy) rather than
// generating any of it -- see CreateProblemFromServiceNow's own doc comment
// for why, and for which req fields are deliberately left unwritten. type is
// hardcoded to 'PROBLEM'::work_item_type_enum. state is cast from
// ServiceNow's own confirmed response value when present (unlike
// change_request, whose create response carries no state at all -- see
// CreateChangeRequestFromServiceNow's own doc comment for that contrast);
// when ServiceNow returns no state, the column is left NULL rather than
// guessed.
//
// Column/output order matches the trailing SELECT exactly.
const createProblemFromServiceNowQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, subject, description, type, parent_id
		)
		VALUES (
			$1, NOW(), NOW(), $2, $2,
			$3, $4, $8, 'PROBLEM'::work_item_type_enum, $5::uuid
		)
		RETURNING id, number, subject, description, created_on, updated_on, created_by
	),
	inserted_problem AS (
		INSERT INTO problem (
			id, state, incident_id, opened_on, category, subcategory_id
		)
		VALUES (
			$1, $6::problem_state_enum, $7::uuid, NOW(), $9::problem_category_enum,
			-- subcategory is matched on problem_subcategory.value (lower-case
			-- free text) within the chosen category; an unmatched value stays NULL.
			(SELECT id FROM problem_subcategory WHERE category = $9::problem_category_enum AND value = LOWER($10::text))
		)
		RETURNING id
	)
	SELECT iwi.id, iwi.number, iwi.subject, iwi.description, iwi.created_on, iwi.updated_on, iwi.created_by
	FROM inserted_work_item iwi
	JOIN inserted_problem ip ON ip.id = iwi.id`

// CreateProblemFromServiceNow implements ProblemRepository.
func (r *problemRepo) CreateProblemFromServiceNow(ctx context.Context, req domain.CreateProblemRequest, id, number, createdBy string, state *string) (domain.ProblemDetail, error) {
	// WithSystemIdentity: this insert never sets a project_id on the new
	// work_item row at all (problems have no project concept, same as
	// incidents -- see this file's own package doc comment), so work_item's
	// INSERT policy (migration 0147) can only be satisfied by is_internal.
	// Same reasoning as IncidentRepository.CreateIncidentFromServiceNow's
	// own identical stamp.
	ctx = WithSystemIdentity(ctx)
	var category *string
	if req.Category != nil && strings.TrimSpace(*req.Category) != "" {
		v := strings.ToUpper(strings.TrimSpace(*req.Category))
		category = &v
	}
	var (
		outID, outNumber, outSubject, outCreatedBy string
		outDescription                             *string
		outCreatedOn, outUpdatedOn                 time.Time
	)
	err := r.db.QueryRow(ctx, createProblemFromServiceNowQuery,
		id, createdBy,
		number, req.Subject, req.OriginCaseID,
		state, req.PrimaryIncidentID, req.Description,
		category, req.Subcategory,
	).Scan(&outID, &outNumber, &outSubject, &outDescription, &outCreatedOn, &outUpdatedOn, &outCreatedBy)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation on id/number -- see this method's own doc comment for why this "shouldn't" happen
				return domain.ProblemDetail{}, &apierror.ConflictError{Msg: "a problem already exists for this ServiceNow id/number: " + pgErr.Detail}
			case "22P02": // invalid_text_representation -- id (or state) was not a valid UUID/enum label
				return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "id is not a valid UUID, or state is not a valid problem state: " + id}
			case "22001": // string_data_right_truncation -- e.g. subject over work_item.subject's VARCHAR(512)
				return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "a field value is too long: " + pgErr.Message}
			case "23503": // foreign_key_violation -- one of the referenced IDs does not exist
				return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			case "P0001": // raise_exception from integrity triggers
				return domain.ProblemDetail{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.ProblemDetail{}, fmt.Errorf("create problem from servicenow: %w", err)
	}

	return domain.ProblemDetail{
		ID:          &outID,
		Number:      &outNumber,
		Subject:     &outSubject,
		Description: outDescription,
		State:       state,
	}, nil
}

// UpdateProblemFields implements ProblemRepository. "problem" is updated
// first (if it has any columns to touch), so a nonexistent id is caught
// before work_item's own row is touched at all -- if req names no "problem"
// column (i.e. only AssignedToID/AssignmentGroupID was set), work_item's own
// UPDATE ... WHERE id = $1 AND type = 'PROBLEM' alone still correctly
// reports not-found, and the explicit type check keeps this from silently
// bumping updated_on/updated_by on a work_item row of some other type that
// happens to share the id (same shared-primary-key space every work_item
// extension table uses). Same overall shape as
// CaseRepository.UpdateCaseFields.
func (r *problemRepo) UpdateProblemFields(ctx context.Context, req domain.UpdateProblemRequest, actorEmail string) (ProblemWriteResult, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (ProblemWriteResult, error) {
		return updateProblemFieldsTx(ctx, tx, req, actorEmail)
	})
}

// updateProblemFieldsTx is UpdateProblemFields' body, extracted so it can
// run inside r.db.InTx's closure and inside TransitionProblem's transaction.
func updateProblemFieldsTx(ctx context.Context, tx pgx.Tx, req domain.UpdateProblemRequest, actorEmail string) (ProblemWriteResult, error) {
	var problemSets []string
	problemArgs := []any{req.ID}
	idx := 2
	if req.CauseNotes != nil {
		problemSets = append(problemSets, fmt.Sprintf("cause_notes = $%d", idx))
		problemArgs = append(problemArgs, *req.CauseNotes)
		idx++
	}
	if req.FixNotes != nil {
		problemSets = append(problemSets, fmt.Sprintf("fix_notes = $%d", idx))
		problemArgs = append(problemArgs, *req.FixNotes)
		idx++
	}
	if req.Workaround != nil {
		problemSets = append(problemSets, fmt.Sprintf("workaround = $%d", idx))
		problemArgs = append(problemArgs, *req.Workaround)
		idx++
	}
	if req.TargetResolutionDate != nil {
		t, err := time.Parse(time.RFC3339, *req.TargetResolutionDate)
		if err != nil {
			return ProblemWriteResult{}, &apierror.ValidationError{Msg: "targetResolutionDate must be a valid RFC3339 timestamp"}
		}
		problemSets = append(problemSets, fmt.Sprintf("due_on = $%d", idx))
		problemArgs = append(problemArgs, t)
		idx++
	}
	if len(problemSets) > 0 {
		tag, err := tx.Exec(ctx, `UPDATE problem SET `+strings.Join(problemSets, ", ")+` WHERE id = $1`, problemArgs...)
		if err != nil {
			return ProblemWriteResult{}, fmt.Errorf("update problem fields: problem: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ProblemWriteResult{}, &apierror.NotFoundError{Msg: "problem not found"}
		}
	}

	// work_item.updated_on/updated_by are bumped unconditionally, matching
	// UpdateCaseFields' identical convention, even when only "problem"
	// columns above changed.
	wiSets := []string{"updated_on = NOW()", "updated_by = $2"}
	wiArgs := []any{req.ID, actorEmail}
	widx := 3
	if req.AssignedToID != nil {
		wiSets = append(wiSets, fmt.Sprintf("assigned_to_id = $%d::uuid", widx))
		wiArgs = append(wiArgs, *req.AssignedToID)
		widx++
	}
	if req.AssignmentGroupID != nil {
		wiSets = append(wiSets, fmt.Sprintf("assignment_group_id = $%d::uuid", widx))
		wiArgs = append(wiArgs, *req.AssignmentGroupID)
		widx++
	}

	var (
		res                ProblemWriteResult
		groupID, groupName *string
	)
	err := tx.QueryRow(ctx, `
		WITH updated AS (
			UPDATE work_item SET `+strings.Join(wiSets, ", ")+`
			WHERE id = $1 AND type = 'PROBLEM'
			RETURNING updated_on, assignment_group_id
		)
		SELECT u.updated_on, g.id::TEXT, g.name
		FROM updated u
		LEFT JOIN "group" g ON g.id = u.assignment_group_id`, wiArgs...).Scan(&res.UpdatedOn, &groupID, &groupName)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProblemWriteResult{}, &apierror.NotFoundError{Msg: "problem not found"}
	}
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			field := "assignedToId"
			if strings.Contains(pgErr.ConstraintName, "assignment_group") || strings.Contains(pgErr.Detail, "assignment_group_id") {
				field = "assignmentGroupId"
			}
			return ProblemWriteResult{}, &apierror.ValidationError{Msg: field + " does not exist: " + pgErr.Detail}
		}
		return ProblemWriteResult{}, fmt.Errorf("update problem fields: work_item: %w", err)
	}
	if groupID != nil {
		res.AssignmentGroup = &domain.EntityRef{ID: *groupID, Name: stringOrEmpty(groupName)}
	}

	return res, nil
}

// TransitionProblem implements ProblemRepository. It ports ServiceNow's
// ProblemUtils._applyProblemTransition, including what ServiceNow does
// around it on save:
//
//   - The problem must be in t.From, else a ValidationError naming the
//     current state. The row is locked (FOR UPDATE) for the check, so two
//     concurrent transitions cannot both pass it. The check reads
//     problem.state, the column every Postgres write path fills;
//     problem_state is written alongside it below but is NULL on problems
//     created in Postgres.
//   - "close" is refused when resolution_code is RISK_ACCEPTED (the native
//     "Complete" UI action's canComplete() guard).
//   - cause_notes, fix_notes, workaround, assigned_to and assignment_group
//     ride along in the same save. targetResolutionDate does not:
//     ProblemUtils' transition path never applies due_date.
//   - state and problem_state are both set to the target.
//   - "resolve" sets resolution_code = FIX_APPLIED (the native "Resolve"
//     UI action's hardcoded value); "close" sets is_active = false.
//
// ServiceNow's "Update Problem State to Assess" before business rule forces
// the problem back to Assess on any save where assigned_to is non-empty,
// regardless of the state just written (live-verified 2026-08-23 per
// ProblemUtils' own comment). This reproduces it: with an assignee after the
// save, state and problem_state land on ASSESS, the other writes stand, and
// Reverted is set so the caller can answer 409 the way ProblemUtils' re-read
// does. The whole write is committed either way, as it is in ServiceNow.
//
// resolved_on/resolved_by_id and closed_on are set when the problem lands
// on RESOLVED/CLOSED, the job ServiceNow's own task/problem business rules
// do for resolved_at/resolved_by/closed_at. resolved_by_id is the "user"
// row matching actorEmail, or NULL when there is none.
func (r *problemRepo) TransitionProblem(ctx context.Context, req domain.UpdateProblemRequest, t domain.ProblemTransition, actorEmail string) (ProblemWriteResult, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (ProblemWriteResult, error) {
		var state, resolutionCode, assignedToID *string
		err := tx.QueryRow(ctx, `
			SELECT pr.state::TEXT, pr.resolution_code::TEXT, wi.assigned_to_id::TEXT
			FROM problem pr
			JOIN work_item wi ON wi.id = pr.id
			WHERE pr.id = $1 AND wi.type = 'PROBLEM'
			FOR UPDATE OF pr, wi`, req.ID).Scan(&state, &resolutionCode, &assignedToID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ProblemWriteResult{}, &apierror.NotFoundError{Msg: "problem not found"}
		}
		if err != nil {
			return ProblemWriteResult{}, fmt.Errorf("transition problem: read state: %w", err)
		}

		current := stringOrEmpty(state)
		if current != string(t.From) {
			return ProblemWriteResult{}, &apierror.ValidationError{Msg: fmt.Sprintf(
				"Invalid state transition: '%s' can only be used when the problem is in state %s. Current state: %s",
				t.Name, t.From, current)}
		}
		if t.Name == "close" && stringOrEmpty(resolutionCode) == "RISK_ACCEPTED" {
			return ProblemWriteResult{}, &apierror.ValidationError{
				Msg: "Invalid state transition: 'close' is not available when resolution_code is 'RISK_ACCEPTED'"}
		}

		fields := domain.UpdateProblemRequest{
			ID:                req.ID,
			CauseNotes:        req.CauseNotes,
			FixNotes:          req.FixNotes,
			Workaround:        req.Workaround,
			AssignedToID:      req.AssignedToID,
			AssignmentGroupID: req.AssignmentGroupID,
		}
		res, err := updateProblemFieldsTx(ctx, tx, fields, actorEmail)
		if err != nil {
			return ProblemWriteResult{}, err
		}

		assignee := stringOrEmpty(assignedToID)
		if req.AssignedToID != nil {
			assignee = *req.AssignedToID
		}
		landed := t.To
		if assignee != "" && landed != domain.ProblemStateAssess {
			landed = domain.ProblemStateAssess
			res.Reverted = true
		}

		sets := []string{
			"state = $2::TEXT::problem_state_enum",
			"problem_state = $2::TEXT::problem_problem_state_enum",
		}
		args := []any{req.ID, string(landed)}
		switch t.Name {
		case "resolve":
			sets = append(sets, "resolution_code = 'FIX_APPLIED'")
		case "close":
			sets = append(sets, "is_active = FALSE")
		}
		switch landed {
		case domain.ProblemStateResolved:
			args = append(args, actorEmail)
			sets = append(sets, "resolved_on = NOW()",
				fmt.Sprintf(`resolved_by_id = (SELECT id FROM "user" WHERE LOWER(email) = LOWER($%d) LIMIT 1)`, len(args)))
		case domain.ProblemStateClosed:
			sets = append(sets, "closed_on = NOW()")
		}
		if _, err := tx.Exec(ctx, `UPDATE problem SET `+strings.Join(sets, ", ")+` WHERE id = $1`, args...); err != nil {
			return ProblemWriteResult{}, fmt.Errorf("transition problem: %w", err)
		}

		return res, nil
	})
}
