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
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Per-project hours: the load and write halves around domain.ComputeProjectHours.
//
// recomputeProjectHours is the ONE implementation behind every trigger: the
// approval of a time card, the hourly recompute and the opportunity and line
// item ingest. It writes durations only. It never touches
// sf_opportunity.query_hour_state, which the hourly recompute alone compares
// against to detect a threshold crossing and notify.

// projectHoursLockKey serialises concurrent recomputes of one project, so an
// approval, the hourly job and an ingest cannot interleave their read and
// write. The lock is released when the surrounding transaction ends.
func projectHoursLockKey(projectID string) string { return "project-hours:" + projectID }

const projectHoursStateQuery = `
	SELECT COALESCE(onboarding_status::text, ''), onboarding_expiry_date, onboarding_go_live_date, CURRENT_DATE,
	       EXISTS (SELECT 1 FROM sf_opportunity_link k
	                 JOIN sf_opportunity_product op ON op.opportunity_id = k.opportunity_id
	                WHERE k.project_id = project.id)
	FROM project WHERE id = $1::uuid`

// Lines that can matter: query lines not yet ended, and every onboarding
// line, ended or not (an ended line still defines the onboarding pool that was
// consumed). Whether a query line has started is decided by the calculation.
// DISTINCT ON (op.id) because two links to one opportunity must not count its
// lines twice.
const projectHoursLinesQuery = `
	SELECT DISTINCT ON (op.id)
	       op.opportunity_id::text, COALESCE(op.product_name, ''), COALESCE(op.quantity, 0),
	       op.service_start_date, op.service_end_date
	FROM sf_opportunity_link k
	JOIN sf_opportunity_product op ON op.opportunity_id = k.opportunity_id
	WHERE k.project_id = $1::uuid
	  AND op.product_name = ANY($2::text[])
	  AND (op.product_name = '` + onboardingProductName + `' OR op.service_end_date >= CURRENT_DATE)
	ORDER BY op.id`

// projectHoursBillableQuery sums the approved, billable minutes over a
// work-date window, across the DISTINCT projects linked to the given
// opportunities (the allowance is shared by them, and a project linked through
// several is counted once). Attribution is by time_card.customer_project_id.
// NULL minute columns count as zero. Row-level security applies to time_card;
// the caller runs under the system identity.
const projectHoursBillableQuery = `
	SELECT COALESCE(SUM(
	           COALESCE(tc.analyzing_minutes, 0) + COALESCE(tc.setting_up_minutes, 0)
	         + COALESCE(tc.reproducing_debugging_minutes, 0)
	         + COALESCE(tc.providing_solution_minutes, 0)
	         + COALESCE(tc.patching_minutes, 0)), 0)::int
	FROM time_card tc
	WHERE tc.customer_project_id IN (
	        SELECT l.project_id FROM sf_opportunity_link l
	         WHERE l.opportunity_id = ANY($1::uuid[]) AND l.project_id IS NOT NULL)
	  AND tc.state = 'APPROVED'
	  AND tc.is_billable IS TRUE
	  AND tc.work_date >= $2::date
	  AND tc.work_date <= $3::date`

// projectHoursWriteQuery writes both pools in one statement and is a no-op
// when nothing changed. $5 says whether the onboarding columns are written at
// all: when false they keep whatever they hold.
const projectHoursWriteQuery = `
	UPDATE project SET
	  total_query_duration         = make_interval(mins => $2::int),
	  consumed_duration            = make_interval(mins => $3::int),
	  remaining_query_duration     = make_interval(mins => $4::int),
	  total_onboarding_duration    = CASE WHEN $5::boolean THEN make_interval(mins => $6::int) ELSE total_onboarding_duration END,
	  consumed_onboarding_duration = CASE WHEN $5::boolean THEN make_interval(mins => $7::int) ELSE consumed_onboarding_duration END,
	  remaining_onboarding_duration = CASE WHEN $5::boolean THEN make_interval(mins => $8::int) ELSE remaining_onboarding_duration END
	WHERE id = $1::uuid
	  AND (total_query_duration         IS DISTINCT FROM make_interval(mins => $2::int)
	    OR consumed_duration            IS DISTINCT FROM make_interval(mins => $3::int)
	    OR remaining_query_duration     IS DISTINCT FROM make_interval(mins => $4::int)
	    OR ($5::boolean AND (
	         total_onboarding_duration     IS DISTINCT FROM make_interval(mins => $6::int)
	      OR consumed_onboarding_duration  IS DISTINCT FROM make_interval(mins => $7::int)
	      OR remaining_onboarding_duration IS DISTINCT FROM make_interval(mins => $8::int))))`

func projectHoursProducts() []string {
	names := []string{domain.OnboardingProductName}
	for name := range domain.QueryHourProductHours {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// recomputeProjectHours recomputes and stores one project's hours on q, which
// must be a transaction (the advisory lock lasts until it ends) running under
// the system identity. A blank or unknown project id is a no-op.
func recomputeProjectHours(ctx context.Context, q querier, projectID string) error {
	if projectID == "" {
		return nil
	}
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, projectHoursLockKey(projectID)); err != nil {
		return fmt.Errorf("recompute project hours: lock: %w", err)
	}

	in := domain.ProjectHoursInput{}
	err := q.QueryRow(ctx, projectHoursStateQuery, projectID).Scan(&in.OnboardingStatus, &in.OnboardingExpiry, &in.GoLive, &in.Today, &in.HasLinkedLines)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("recompute project hours: read project: %w", err)
	}

	rows, err := q.Query(ctx, projectHoursLinesQuery, projectID, projectHoursProducts())
	if err != nil {
		return fmt.Errorf("recompute project hours: read lines: %w", err)
	}
	for rows.Next() {
		var l domain.ProjectHoursLine
		if err := rows.Scan(&l.OpportunityID, &l.Product, &l.Quantity, &l.Start, &l.End); err != nil {
			rows.Close()
			return fmt.Errorf("recompute project hours: scan line: %w", err)
		}
		in.Lines = append(in.Lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("recompute project hours: read lines: %w", err)
	}

	res, err := domain.ComputeProjectHours(in, func(oppIDs []string, from, to time.Time) (int, error) {
		var mins int
		if err := q.QueryRow(ctx, projectHoursBillableQuery, oppIDs, from, to).Scan(&mins); err != nil {
			return 0, fmt.Errorf("recompute project hours: read time cards: %w", err)
		}
		return mins, nil
	})
	if err != nil {
		return err
	}
	if !res.HasAllocation {
		return nil
	}
	if _, err := q.Exec(ctx, projectHoursWriteQuery, projectID,
		res.QueryTotal, res.QueryConsumed, res.QueryRemaining,
		res.OnboardingApplicable, res.OnboardingTotal, res.OnboardingConsumed, res.OnboardingRemaining); err != nil {
		return fmt.Errorf("recompute project hours: write: %w", err)
	}
	return nil
}

// recomputeOpportunityProjectHours recomputes every project linked to an
// opportunity, in id order. A blank id is a no-op. The ids are read to the end
// before any recompute runs, because a transaction serves one statement at a
// time.
func recomputeOpportunityProjectHours(ctx context.Context, q querier, opportunityID string) error {
	if opportunityID == "" {
		return nil
	}
	rows, err := q.Query(ctx, `SELECT DISTINCT project_id::text FROM sf_opportunity_link
		WHERE opportunity_id = $1::uuid AND project_id IS NOT NULL ORDER BY 1`, opportunityID)
	if err != nil {
		return fmt.Errorf("recompute project hours: list opportunity projects: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("recompute project hours: scan project: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("recompute project hours: list opportunity projects: %w", err)
	}
	for _, id := range ids {
		if err := recomputeProjectHours(ctx, q, id); err != nil {
			return err
		}
	}
	return nil
}

// recomputeMovedProjectHours and recomputeMovedOpportunityProjectHours are the
// counterparts of deriveMovedLinkProjects and deriveMovedLineItemOpportunities:
// the owner a row moved away from loses its allocation too.
func recomputeMovedProjectHours(ctx context.Context, q querier, previous []string, newProjectID string) error {
	for _, id := range previous {
		if id == newProjectID {
			continue
		}
		if err := recomputeProjectHours(ctx, q, id); err != nil {
			return err
		}
	}
	return nil
}

func recomputeMovedOpportunityProjectHours(ctx context.Context, q querier, previous []string, newOpportunityID string) error {
	for _, id := range previous {
		if id == newOpportunityID {
			continue
		}
		if err := recomputeOpportunityProjectHours(ctx, q, id); err != nil {
			return err
		}
	}
	return nil
}

// txBeginner is what a pgx.Tx offers to open a savepoint.
type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// bestEffortProjectHours runs fn, a project hours recompute, as a side effect
// of an ingest write that must not fail because of it. A failed statement
// aborts a Postgres transaction, so when q can open a nested transaction (a
// pgx.Tx can) fn runs inside a savepoint that is rolled back on error and the
// ingest transaction carries on. A q that cannot (a test double) runs fn
// directly. Either way the error is logged and swallowed: the hourly recompute
// heals the project.
func bestEffortProjectHours(ctx context.Context, q querier, scope string, fn func(q querier) error) {
	b, ok := q.(txBeginner)
	if !ok {
		if err := fn(q); err != nil {
			slog.WarnContext(ctx, "project hours recompute failed, ingest continues", "scope", scope, "error", err)
		}
		return
	}
	sp, err := b.Begin(ctx)
	if err != nil {
		slog.WarnContext(ctx, "project hours recompute skipped: savepoint failed", "scope", scope, "error", err)
		return
	}
	if err := fn(sp); err != nil {
		if rbErr := sp.Rollback(ctx); rbErr != nil {
			slog.WarnContext(ctx, "project hours recompute: savepoint rollback failed", "scope", scope, "error", rbErr)
		}
		slog.WarnContext(ctx, "project hours recompute failed, ingest continues", "scope", scope, "error", err)
		return
	}
	if err := sp.Commit(ctx); err != nil {
		slog.WarnContext(ctx, "project hours recompute: savepoint release failed", "scope", scope, "error", err)
	}
}

// siblingProjectsQuery lists the other projects linked to any opportunity the
// given project is linked to: they share its pools, so they change with it.
const siblingProjectsQuery = `
	SELECT DISTINCT l2.project_id::text
	FROM sf_opportunity_link l1
	JOIN sf_opportunity_link l2 ON l2.opportunity_id = l1.opportunity_id
	WHERE l1.project_id = $1::uuid
	  AND l2.project_id IS NOT NULL
	  AND l2.project_id <> l1.project_id
	ORDER BY 1`
