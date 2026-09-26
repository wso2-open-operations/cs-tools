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
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProjectCaseStatsFilter narrows the aggregations below to one project, and
// optionally to a set of case types and a creator.
//
// Types holds validCaseType's (case_service.go) lowercase domain values; they
// are upper-cased here to the work_item_type_enum labels the column actually
// stores. An empty slice means "every case-like type", matching the
// ServiceNow implementation's noFilter branch (ProjectStatsUtils
// _resolveCaseTypeIds), which applies no type restriction at all rather than
// matching nothing.
//
// CreatedBy is an email address, not a username: work_item.created_by is a
// free-text VARCHAR holding an email (see case_repo.go's own note on it), so
// the ServiceNow implementation's email -> sys_user.user_name lookup
// (_resolveUsername) has no analogue here -- the value is compared directly,
// case-insensitively, the same way case_repo.go joins creators by email.
type ProjectCaseStatsFilter struct {
	ProjectID string
	Types     []string
	CreatedBy string

	// Scope is the caller's resolved AccessScope. StateSeverityCounts,
	// StateEngagementTypeCounts, ResolvedBuckets and ClosedByCreatedWindow
	// all read caseLikeStateColumn/caseLikeClosedOnColumn, which fold in
	// announcement.state/closed_on through caseLikeJoins's LEFT JOIN to the
	// RLS-protected `announcement` table (migration 000085) -- without the
	// caller's identity set in the same transaction, a restricted
	// announcement's columns come back NULL, which caseLikeStateColumn's
	// COALESCE then reads as "no state", undercounting or misclassifying it.
	// AverageResponseSeconds and CaseTypeCounts don't select any
	// announcement column, so they're unaffected and don't consult Scope.
	Scope SearchScope
}

// StateSeverityCount is one (state, severity) group of the main aggregation.
// Severity is empty for a row whose type carries no severity -- see
// ProjectCaseStatsRepository.StateSeverityCounts.
type StateSeverityCount struct {
	State    string
	Severity string
	Count    int
}

// StateEngagementTypeCount is one (state, engagement type) group. Only
// engagement rows have a type, so every returned row is an engagement.
type StateEngagementTypeCount struct {
	State          string
	EngagementType string
	Count          int
}

// ProjectCaseStatsRepository backs the Postgres implementation of
// GET /projects/{id}/cases/stats. Each method is one aggregation of the
// ServiceNow ProjectStatsUtils.getProjectScopeCaseStats equivalent, kept
// separate because that implementation deliberately applies different
// filters to each (see the per-method notes) -- collapsing them into one
// query would silently change the numbers.
type ProjectCaseStatsRepository interface {
	// StateSeverityCounts returns counts grouped by state and severity,
	// mirroring the ServiceNow main aggregate's groupBy('state') +
	// groupBy('priority'). Severity is read from the "case" extension table
	// only: it is the sole case-like table with a severity column
	// (migration 000018), so rows of every other type return an empty
	// Severity and are counted in the totals but in no severity bucket.
	StateSeverityCounts(ctx context.Context, f ProjectCaseStatsFilter) ([]StateSeverityCount, error)

	// StateEngagementTypeCounts returns counts grouped by state and
	// engagement type. The ServiceNow equivalent (engAgg) does NOT apply its
	// createdBy filter -- only the main aggregate does -- so neither does
	// this, deliberately.
	StateEngagementTypeCounts(ctx context.Context, f ProjectCaseStatsFilter) ([]StateEngagementTypeCount, error)

	// ResolvedBuckets returns the count of resolved cases closed since the
	// start of the current UTC month, and within the last 30 days.
	//
	// resolvedStates are the states that count as resolved. Bucketing is on
	// closed_on, NOT resolved_on: the ServiceNow implementation reads
	// closed_at and requires it to be non-empty, so a row resolved but never
	// closed is excluded from both buckets while still counting toward
	// resolvedCount.total.
	ResolvedBuckets(ctx context.Context, f ProjectCaseStatsFilter, resolvedStates []string) (currentMonth, pastThirtyDays int, err error)

	// ClosedByCreatedWindow returns the count of cases in state closedState
	// created in the last 30 days, and in the 30 days before that -- the two
	// windows behind changeRate.resolvedEngagements.
	//
	// Two deliberate mirrors of the ServiceNow implementation
	// (_getProjectResolvedDetails): the windows filter on CREATION date
	// while counting closures, and f.Types is ignored (that method never
	// applies the case-type filter its caller resolved).
	ClosedByCreatedWindow(ctx context.Context, f ProjectCaseStatsFilter, closedState string) (current, previous int, err error)

	// AverageResponseSeconds returns the mean business duration, in seconds,
	// of completed RESPONSE-target SLAs attached to this project's cases
	// created in the last 30 days, together with how many contributed.
	//
	// Mirrors _getSLADetails, including that it applies neither the
	// case-type nor the createdBy filter, and that a row with no business
	// duration contributes zero rather than being skipped (_durationToSeconds
	// returns 0 for an unparseable value but the loop still counts it).
	//
	// The ServiceNow version additionally restricts to the project's allowed
	// severities; Postgres has no per-project severity-restriction table
	// (see project_metadata_service.go's note on AcceptedSeverityValues), so
	// that narrowing cannot be reproduced and is omitted.
	AverageResponseSeconds(ctx context.Context, projectID string) (avgSeconds float64, count int, err error)

	// CaseTypeCounts returns counts grouped by work_item type, keyed by the
	// lowercase domain type (CaseTypeRefs' ids). Mirrors the ServiceNow
	// caseTypeAgg, which scopes to the project's allowed case types and
	// applies createdBy but ignores the requested caseTypes filter -- so
	// f.Types is ignored here too. The allowed-case-type narrowing has no
	// Postgres equivalent (no feature-entitlement table) and is omitted.
	CaseTypeCounts(ctx context.Context, f ProjectCaseStatsFilter) (map[string]int, error)
}

type projectCaseStatsRepo struct {
	db *pgxpool.Pool
}

// NewProjectCaseStatsRepository constructs a ProjectCaseStatsRepository backed by the given connection pool.
func NewProjectCaseStatsRepository(db *pgxpool.Pool) ProjectCaseStatsRepository {
	return &projectCaseStatsRepo{db: db}
}

// caseStatsFrom is the FROM/JOIN clause every aggregation here shares: the
// case-like work_item rows with each shared-PK extension table joined under
// the aliases caseLikeStateColumn and friends expect.
const caseStatsFrom = `
	FROM work_item wi
	LEFT JOIN "case" c ON c.id = wi.id` + caseLikeJoins

// caseStatsWhere builds the shared WHERE clause and its arguments. next is
// the 1-based index of the first placeholder to use, so callers that prepend
// their own arguments stay in sync. includeCreatedBy and includeTypes let a
// caller opt out of a filter the ServiceNow implementation does not apply to
// that particular aggregation.
func caseStatsWhere(f ProjectCaseStatsFilter, next int, includeTypes, includeCreatedBy bool) (string, []any) {
	clauses := []string{
		fmt.Sprintf("wi.project_id = $%d::uuid", next),
		"wi.type = ANY(" + caseLikeWorkItemTypes + ")",
	}
	args := []any{f.ProjectID}
	next++

	if includeTypes && len(f.Types) > 0 {
		enumTypes := make([]string, len(f.Types))
		for i, t := range f.Types {
			enumTypes[i] = strings.ToUpper(t)
		}
		clauses = append(clauses, fmt.Sprintf("wi.type = ANY($%d::work_item_type_enum[])", next))
		args = append(args, enumTypes)
		next++
	}

	if includeCreatedBy && f.CreatedBy != "" {
		clauses = append(clauses, fmt.Sprintf("LOWER(wi.created_by) = LOWER($%d)", next))
		args = append(args, f.CreatedBy)
	}

	return " WHERE " + strings.Join(clauses, " AND "), args
}

// StateSeverityCounts implements ProjectCaseStatsRepository.
func (r *projectCaseStatsRepo) StateSeverityCounts(ctx context.Context, f ProjectCaseStatsFilter) ([]StateSeverityCount, error) {
	where, args := caseStatsWhere(f, 1, true, true)
	var out []StateSeverityCount
	err := runWithCallerIdentity(ctx, r.db, f.Scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+caseLikeStateColumn+` AS state,
			       COALESCE(c.severity::TEXT, '') AS severity,
			       COUNT(*)`+caseStatsFrom+where+`
			 GROUP BY 1, 2`, args...)
		if err != nil {
			return fmt.Errorf("project case stats: state/severity counts: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var sc StateSeverityCount
			// state is NULL when a case-like row has no extension row at all;
			// such a row still counts toward totalCount, so it is kept with an
			// empty State rather than dropped.
			var state *string
			if err := rows.Scan(&state, &sc.Severity, &sc.Count); err != nil {
				return fmt.Errorf("project case stats: scan state/severity count: %w", err)
			}
			if state != nil {
				sc.State = *state
			}
			out = append(out, sc)
		}
		return rows.Err()
	})
	return out, err
}

// StateEngagementTypeCounts implements ProjectCaseStatsRepository.
func (r *projectCaseStatsRepo) StateEngagementTypeCounts(ctx context.Context, f ProjectCaseStatsFilter) ([]StateEngagementTypeCount, error) {
	where, args := caseStatsWhere(f, 1, true, false)
	var out []StateEngagementTypeCount
	err := runWithCallerIdentity(ctx, r.db, f.Scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+caseLikeStateColumn+` AS state,
			       eng.type::TEXT,
			       COUNT(*)`+caseStatsFrom+where+`
			   AND eng.type IS NOT NULL
			 GROUP BY 1, 2`, args...)
		if err != nil {
			return fmt.Errorf("project case stats: engagement type counts: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var ec StateEngagementTypeCount
			var state *string
			if err := rows.Scan(&state, &ec.EngagementType, &ec.Count); err != nil {
				return fmt.Errorf("project case stats: scan engagement type count: %w", err)
			}
			if state != nil {
				ec.State = *state
			}
			out = append(out, ec)
		}
		return rows.Err()
	})
	return out, err
}

// ResolvedBuckets implements ProjectCaseStatsRepository.
func (r *projectCaseStatsRepo) ResolvedBuckets(ctx context.Context, f ProjectCaseStatsFilter, resolvedStates []string) (int, int, error) {
	if len(resolvedStates) == 0 {
		return 0, 0, nil
	}
	where, args := caseStatsWhere(f, 1, true, true)
	args = append(args, resolvedStates)
	statePlaceholder := fmt.Sprintf("$%d", len(args))

	// date_trunc is applied to a UTC-converted now() and converted back, so
	// the month boundary is the UTC one the ServiceNow implementation uses
	// (getYearUTC/getMonthUTC) rather than the database server's timezone.
	var currentMonth, pastThirtyDays int
	err := runWithCallerIdentity(ctx, r.db, f.Scope, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FILTER (WHERE closed_on >= date_trunc('month', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),
			       COUNT(*) FILTER (WHERE closed_on >= now() - INTERVAL '30 days')
			  FROM (
			        SELECT `+caseLikeStateColumn+` AS state,
			               `+caseLikeClosedOnColumn+` AS closed_on`+caseStatsFrom+where+`
			       ) resolved
			 WHERE state = ANY(`+statePlaceholder+`) AND closed_on IS NOT NULL`, args...).
			Scan(&currentMonth, &pastThirtyDays)
	})
	if err != nil {
		return 0, 0, fmt.Errorf("project case stats: resolved buckets: %w", err)
	}
	return currentMonth, pastThirtyDays, nil
}

// ClosedByCreatedWindow implements ProjectCaseStatsRepository.
func (r *projectCaseStatsRepo) ClosedByCreatedWindow(ctx context.Context, f ProjectCaseStatsFilter, closedState string) (int, int, error) {
	where, args := caseStatsWhere(f, 1, false, true)
	args = append(args, closedState)
	statePlaceholder := fmt.Sprintf("$%d", len(args))

	var current, previous int
	err := runWithCallerIdentity(ctx, r.db, f.Scope, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FILTER (WHERE created_on >= now() - INTERVAL '30 days'),
			       COUNT(*) FILTER (WHERE created_on >= now() - INTERVAL '60 days'
			                          AND created_on <  now() - INTERVAL '30 days')
			  FROM (
			        SELECT `+caseLikeStateColumn+` AS state,
			               wi.created_on`+caseStatsFrom+where+`
			       ) windowed
			 WHERE state = `+statePlaceholder, args...).
			Scan(&current, &previous)
	})
	if err != nil {
		return 0, 0, fmt.Errorf("project case stats: change-rate windows: %w", err)
	}
	return current, previous, nil
}

// AverageResponseSeconds implements ProjectCaseStatsRepository.
func (r *projectCaseStatsRepo) AverageResponseSeconds(ctx context.Context, projectID string) (float64, int, error) {
	var count int
	var avg float64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*),
		       COALESCE(AVG(COALESCE(EXTRACT(EPOCH FROM s.business_duration), 0)), 0)
		  FROM sla s
		  JOIN sla_policy p ON p.id = s.sla_policy_id
		  JOIN work_item wi ON wi.id = s.work_item_id
		 WHERE wi.project_id = $1::uuid
		   AND wi.type = ANY(`+caseLikeWorkItemTypes+`)
		   AND p.target = 'RESPONSE'
		   AND s.stage = 'COMPLETED'
		   AND wi.created_on >= now() - INTERVAL '30 days'`, projectID).
		Scan(&count, &avg)
	if err != nil {
		return 0, 0, fmt.Errorf("project case stats: average response time: %w", err)
	}
	return avg, count, nil
}

// CaseTypeCounts implements ProjectCaseStatsRepository.
func (r *projectCaseStatsRepo) CaseTypeCounts(ctx context.Context, f ProjectCaseStatsFilter) (map[string]int, error) {
	where, args := caseStatsWhere(f, 1, false, true)
	rows, err := r.db.Query(ctx, `
		SELECT wi.type::TEXT, COUNT(*)`+caseStatsFrom+where+`
		 GROUP BY 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("project case stats: case type counts: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var workItemType string
		var count int
		if err := rows.Scan(&workItemType, &count); err != nil {
			return nil, fmt.Errorf("project case stats: scan case type count: %w", err)
		}
		out[strings.ToLower(workItemType)] = count
	}
	return out, rows.Err()
}
