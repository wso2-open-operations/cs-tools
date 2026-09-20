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
// AssignmentGroup is always nil: problem has no assignment-group column
// anywhere (same gap as change_request's own AssignedTeamID), and a
// caller's "assignmentGroupId" filter is silently not applied, matching
// changeRequestWhereClause's own precedent for the identical gap.
//
// CreateProblem/UpdateProblem have no Postgres implementation: CreateProblem
// needs work_item.number, which has no DB default or backing sequence
// anywhere in migrations/ (same blocker as CaseRepository.CreateCase);
// UpdateProblem's Transition field is validated server-side by the data
// source itself, with no fixed, confirmed transition rule set to reimplement
// here (see domain.UpdateProblemRequest's own doc comment -- deliberately
// not a closed enum for exactly this reason).
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
}

type problemRepo struct {
	db *pgxpool.Pool
}

// NewProblemRepository constructs a ProblemRepository backed by the given connection pool.
func NewProblemRepository(db *pgxpool.Pool) ProblemRepository {
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
		SELECT wi.id, wi.number, wi.subject, pr.state::TEXT, pr.priority::TEXT,
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
		&id2, &number, &subject, &state, &priority,
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
		ID: &id2, Number: &number, Subject: &subject, State: state, Priority: priority,
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
