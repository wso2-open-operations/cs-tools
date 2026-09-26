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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StateCount is one state group of a stats aggregation. State is empty for a
// row whose extension table carries no state.
type StateCount struct {
	State string
	Count int
}

// ProjectSLAStatusInputs are the four independent conditions ServiceNow's
// projectSLAStatus checks. Every one must hold for a project to be "All
// Good"; any single failure makes it "Needs Attention".
type ProjectSLAStatusInputs struct {
	// HasOutstandingCase mirrors _hasAnyOutstandingIncident: a case that is
	// not closed and carries a severity. ServiceNow restricts to the
	// project's allowed severities, which has no Postgres equivalent, so
	// "has a severity at all" is the closest faithful reading.
	HasOutstandingCase bool
	// HasDeployedProduct mirrors _hasAnyDeployment, which despite its name
	// queries sn_install_base_item -- deployed products, not deployments.
	HasDeployedProduct bool
	// HasActiveEndDate mirrors _hasActiveProjectEndDate: the project has an
	// end date and it is not in the past.
	HasActiveEndDate bool
	// HasCustomerAdminContact mirrors _hasCustomerAdminContact: at least one
	// project contact resolves to a user holding the customer_admin role.
	HasCustomerAdminContact bool
}

// ProjectStatsRepository backs the Postgres implementation of the
// project-scoped stats endpoints other than case stats (see
// ProjectCaseStatsRepository for those). Each method is one aggregation of
// the corresponding ServiceNow ProjectStatsUtils helper.
type ProjectStatsRepository interface {
	// TimeLoggedMinutes returns approved billable and non-billable minutes
	// logged against a project's cases. ServiceNow sums time_card.total for
	// cases opened inside the project's start/end window; this schema has no
	// single total column, so the five per-activity minute columns
	// (migration 000039) are summed instead.
	//
	// The project comes from the time card's case (work_item.project_id),
	// not time_card.customer_project_id, matching SearchCaseTimeCards' own
	// choice -- one case can otherwise fragment across projects.
	TimeLoggedMinutes(ctx context.Context, projectID, startDate, endDate string) (billable, nonBillable int, err error)

	// DeploymentCount returns the project's active deployments.
	DeploymentCount(ctx context.Context, projectID string) (int, error)

	// DeployedProductCount returns the project's active deployed products.
	DeployedProductCount(ctx context.Context, projectID string) (int, error)

	// InstanceCount returns the deployment nodes reporting under the
	// project's key.
	InstanceCount(ctx context.Context, projectID string) (int, error)

	// LastDeploymentOn returns when the project's most recent deployment was
	// created, or nil when it has none.
	LastDeploymentOn(ctx context.Context, projectID string) (*time.Time, error)

	// OutstandingCounts returns the per-type counts of work items in an
	// outstanding state: caseStates for the five case-like types, crStates
	// for change requests (the two sets differ -- see the service's own
	// constants).
	OutstandingCounts(ctx context.Context, projectID string, caseStates, crStates []string) (map[string]int, error)

	// SLAStatusInputs evaluates the four projectSLAStatus conditions in one
	// round trip rather than four.
	SLAStatusInputs(ctx context.Context, projectID string) (ProjectSLAStatusInputs, error)

	// ConversationStateCounts returns the project's conversations grouped by
	// state. createdBy is an optional creator email filter.
	ConversationStateCounts(ctx context.Context, projectID, createdBy string) ([]StateCount, error)

	// ChangeRequestStateCounts returns the project's change requests grouped
	// by state.
	ChangeRequestStateCounts(ctx context.Context, projectID string) ([]StateCount, error)

	// ChangeRequestResolvedBuckets returns how many change requests in
	// closedState were closed since the start of the current UTC month, and
	// within the last 30 days.
	ChangeRequestResolvedBuckets(ctx context.Context, projectID, closedState string) (currentMonth, pastThirtyDays int, err error)
}

type projectStatsRepo struct {
	db *pgxpool.Pool
}

// NewProjectStatsRepository constructs a ProjectStatsRepository backed by the given connection pool.
func NewProjectStatsRepository(db *pgxpool.Pool) ProjectStatsRepository {
	return &projectStatsRepo{db: db}
}

// timeCardMinutesExpr sums the five per-activity minute columns. A NULL in
// any one contributes zero rather than nulling the whole row.
const timeCardMinutesExpr = `COALESCE(tc.analyzing_minutes,0) + COALESCE(tc.setting_up_minutes,0) +
	COALESCE(tc.reproducing_debugging_minutes,0) + COALESCE(tc.providing_solution_minutes,0) +
	COALESCE(tc.patching_minutes,0)`

// TimeLoggedMinutes implements ProjectStatsRepository.
func (r *projectStatsRepo) TimeLoggedMinutes(ctx context.Context, projectID, startDate, endDate string) (int, int, error) {
	args := []any{projectID}
	dateClause := ""
	if startDate != "" {
		args = append(args, startDate)
		dateClause += fmt.Sprintf(" AND tc.work_date >= $%d::date", len(args))
	}
	if endDate != "" {
		args = append(args, endDate)
		dateClause += fmt.Sprintf(" AND tc.work_date <= $%d::date", len(args))
	}

	var billable, nonBillable int
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(`+timeCardMinutesExpr+`) FILTER (WHERE tc.is_billable IS TRUE), 0),
		       COALESCE(SUM(`+timeCardMinutesExpr+`) FILTER (WHERE tc.is_billable IS NOT TRUE), 0)
		  FROM time_card tc
		  JOIN work_item wi ON wi.id = tc.case_id
		 WHERE wi.project_id = $1::uuid
		   AND tc.state = 'APPROVED'`+dateClause, args...).
		Scan(&billable, &nonBillable)
	if err != nil {
		return 0, 0, fmt.Errorf("project stats: time logged: %w", err)
	}
	return billable, nonBillable, nil
}

// DeploymentCount implements ProjectStatsRepository.
func (r *projectStatsRepo) DeploymentCount(ctx context.Context, projectID string) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM deployment WHERE project_id = $1::uuid AND is_active IS TRUE`,
		projectID).Scan(&n); err != nil {
		return 0, fmt.Errorf("project stats: deployment count: %w", err)
	}
	return n, nil
}

// DeployedProductCount implements ProjectStatsRepository.
func (r *projectStatsRepo) DeployedProductCount(ctx context.Context, projectID string) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM deployed_product WHERE project_id = $1::uuid AND active IS TRUE`,
		projectID).Scan(&n); err != nil {
		return 0, fmt.Errorf("project stats: deployed product count: %w", err)
	}
	return n, nil
}

// InstanceCount implements ProjectStatsRepository.
//
// deployment_node has no foreign key to project: it carries a free-text key
// copied from the reported payload, matched to project.key exactly as
// instance_repo.go's own instanceRefJoins does.
//
// The column is spelled project_key, following the live (sync-built) schema
// rather than migration 000054's subscription_key -- the same deliberate
// choice instance_repo.go makes, and the reason a database built purely from
// migrations/ cannot run this query. See CLAUDE.md's "Staging schema drift".
func (r *projectStatsRepo) InstanceCount(ctx context.Context, projectID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		  FROM deployment_node dn
		  JOIN project p ON p.key = dn.project_key
		 WHERE p.id = $1::uuid`, projectID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("project stats: instance count: %w", err)
	}
	return n, nil
}

// LastDeploymentOn implements ProjectStatsRepository.
func (r *projectStatsRepo) LastDeploymentOn(ctx context.Context, projectID string) (*time.Time, error) {
	var last *time.Time
	err := r.db.QueryRow(ctx,
		`SELECT MAX(created_on) FROM deployment WHERE project_id = $1::uuid`,
		projectID).Scan(&last)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("project stats: last deployment: %w", err)
	}
	return last, nil
}

// OutstandingCounts implements ProjectStatsRepository. The returned map is
// keyed by the lowercase domain type ("case", "change_request", ...).
func (r *projectStatsRepo) OutstandingCounts(ctx context.Context, projectID string, caseStates, crStates []string) (map[string]int, error) {
	out := make(map[string]int)

	rows, err := r.db.Query(ctx, `
		SELECT wi.type::TEXT, COUNT(*)
		  FROM work_item wi
		  LEFT JOIN "case" c ON c.id = wi.id`+caseLikeJoins+`
		 WHERE wi.project_id = $1::uuid
		   AND wi.type = ANY(`+caseLikeWorkItemTypes+`)
		   AND `+caseLikeStateColumn+` = ANY($2)
		 GROUP BY 1`, projectID, caseStates)
	if err != nil {
		return nil, fmt.Errorf("project stats: outstanding case counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, fmt.Errorf("project stats: scan outstanding count: %w", err)
		}
		out[strings.ToLower(t)] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var crCount int
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		  FROM work_item wi
		  JOIN change_request cr ON cr.id = wi.id
		 WHERE wi.project_id = $1::uuid AND cr.state::TEXT = ANY($2)`, projectID, crStates).Scan(&crCount)
	if err != nil {
		return nil, fmt.Errorf("project stats: outstanding change request count: %w", err)
	}
	out["change_request"] = crCount

	return out, nil
}

// SLAStatusInputs implements ProjectStatsRepository.
func (r *projectStatsRepo) SLAStatusInputs(ctx context.Context, projectID string) (ProjectSLAStatusInputs, error) {
	var in ProjectSLAStatusInputs
	err := r.db.QueryRow(ctx, `
		SELECT
		  EXISTS (
		    SELECT 1 FROM work_item wi
		      LEFT JOIN "case" c ON c.id = wi.id`+caseLikeJoins+`
		     WHERE wi.project_id = $1::uuid
		       AND wi.type = ANY(`+caseLikeWorkItemTypes+`)
		       AND c.severity IS NOT NULL
		       AND `+caseLikeStateColumn+` IS DISTINCT FROM 'CLOSED'
		  ),
		  EXISTS (SELECT 1 FROM deployed_product WHERE project_id = $1::uuid AND active IS TRUE),
		  EXISTS (SELECT 1 FROM project WHERE id = $1::uuid AND end_date IS NOT NULL AND end_date >= CURRENT_DATE),
		  EXISTS (
		    SELECT 1
		      FROM project_contact pc
		      JOIN account_contact ac ON ac.id = pc.account_contact_id
		      JOIN "user" u ON LOWER(u.user_name) = LOWER(ac.user_name)
		      JOIN user_role ur ON ur.user_id = u.id
		      JOIN role rl ON rl.id = ur.role_id
		     WHERE pc.project_id = $1::uuid AND rl.name = 'customer_admin'
		  )`, projectID).
		Scan(&in.HasOutstandingCase, &in.HasDeployedProduct, &in.HasActiveEndDate, &in.HasCustomerAdminContact)
	if err != nil {
		return ProjectSLAStatusInputs{}, fmt.Errorf("project stats: sla status: %w", err)
	}
	return in, nil
}

// ConversationStateCounts implements ProjectStatsRepository.
func (r *projectStatsRepo) ConversationStateCounts(ctx context.Context, projectID, createdBy string) ([]StateCount, error) {
	args := []any{projectID}
	createdByClause := ""
	if createdBy != "" {
		args = append(args, createdBy)
		createdByClause = fmt.Sprintf(" AND LOWER(wi.created_by) = LOWER($%d)", len(args))
	}

	rows, err := r.db.Query(ctx, `
		SELECT conv.state::TEXT, COUNT(*)
		  FROM work_item wi
		  JOIN conversation conv ON conv.id = wi.id
		 WHERE wi.project_id = $1::uuid`+createdByClause+`
		 GROUP BY 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("project stats: conversation state counts: %w", err)
	}
	defer rows.Close()
	return scanStateCounts(rows, "conversation")
}

// ChangeRequestStateCounts implements ProjectStatsRepository.
func (r *projectStatsRepo) ChangeRequestStateCounts(ctx context.Context, projectID string) ([]StateCount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cr.state::TEXT, COUNT(*)
		  FROM work_item wi
		  JOIN change_request cr ON cr.id = wi.id
		 WHERE wi.project_id = $1::uuid
		 GROUP BY 1`, projectID)
	if err != nil {
		return nil, fmt.Errorf("project stats: change request state counts: %w", err)
	}
	defer rows.Close()
	return scanStateCounts(rows, "change request")
}

// ChangeRequestResolvedBuckets implements ProjectStatsRepository. Bucketing
// is on closed_on and requires it to be set, matching the ServiceNow
// implementation's closed_at != ” guard.
func (r *projectStatsRepo) ChangeRequestResolvedBuckets(ctx context.Context, projectID, closedState string) (int, int, error) {
	var currentMonth, pastThirtyDays int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE cr.closed_on >= date_trunc('month', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),
		       COUNT(*) FILTER (WHERE cr.closed_on >= now() - INTERVAL '30 days')
		  FROM work_item wi
		  JOIN change_request cr ON cr.id = wi.id
		 WHERE wi.project_id = $1::uuid
		   AND cr.state::TEXT = $2
		   AND cr.closed_on IS NOT NULL`, projectID, closedState).
		Scan(&currentMonth, &pastThirtyDays)
	if err != nil {
		return 0, 0, fmt.Errorf("project stats: change request resolved buckets: %w", err)
	}
	return currentMonth, pastThirtyDays, nil
}

// scanStateCounts collects (state, count) rows, tolerating a NULL state.
func scanStateCounts(rows pgx.Rows, label string) ([]StateCount, error) {
	var out []StateCount
	for rows.Next() {
		var state *string
		var n int
		if err := rows.Scan(&state, &n); err != nil {
			return nil, fmt.Errorf("project stats: scan %s state count: %w", label, err)
		}
		sc := StateCount{Count: n}
		if state != nil {
			sc.State = *state
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}
