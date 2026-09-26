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
	"time"

	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// QueryHourRepository reads the csm-sync-service-owned `project` and
// `time_card` tables and maintains entity-service's own
// `project_query_hours` row per project.
//
// It never writes `project`: those columns mirror ServiceNow and the next
// sync run would overwrite anything Go put there. See migration 000084.
type QueryHourRepository interface {
	// Consumption aggregates one project's approved time cards and reads its
	// entitlement. Returns a NotFoundError if no project row has that id.
	Consumption(ctx context.Context, projectID string) (domain.QueryHourConsumption, error)
	// Upsert writes the computed position and returns it as stored, together
	// with the state it replaced — nil when the project had no row, i.e. this
	// is its first computation. The previous state is reported from inside
	// the same statement so a concurrent recompute cannot also observe it and
	// publish a duplicate threshold notice; see upsertSQL.
	// lastPushedState/lastPushedAt are preserved, never overwritten here —
	// MarkPushed owns those.
	Upsert(ctx context.Context, c domain.QueryHourConsumption, state int) (domain.ProjectQueryHours, *int, error)
	// MarkPushed records that Choreo accepted `state` for this project.
	MarkPushed(ctx context.Context, projectID string, state int, at time.Time) error
	// Get returns the stored position without recomputing it.
	Get(ctx context.Context, projectID string) (domain.ProjectQueryHours, error)
	// StaleProjectIDs returns up to limit project ids whose stored position is
	// older than `olderThan`, oldest first, plus any project that has approved
	// time cards but no project_query_hours row at all. The scheduled sweep
	// uses this so it never has to walk every project every run.
	StaleProjectIDs(ctx context.Context, olderThan time.Time, limit int) ([]string, error)
	// ProjectIDForTimeCard resolves a time card to the project it belongs to.
	// Used by the targeted recompute path.
	ProjectIDForTimeCard(ctx context.Context, timeCardID string) (string, error)
	// NotificationContext returns the names and internal addresses the
	// threshold email needs. Any field may be empty when the row carries none.
	NotificationContext(ctx context.Context, projectID string) (domain.QueryHourNotificationContext, error)
}

type queryHourRepo struct {
	db *pgxpool.Pool
}

// NewQueryHourRepository constructs a QueryHourRepository over the given pool.
func NewQueryHourRepository(db *pgxpool.Pool) QueryHourRepository {
	return &queryHourRepo{db: db}
}

// consumptionSQL aggregates a project's approved time cards.
//
// Three deliberate differences from the ServiceNow original
// (QueryHourUtils.getConsumed):
//
//  1. SN summed `time_card.total`. The Postgres time_card has no such column —
//     it carries the five per-activity minute columns instead — so the total
//     is their sum. patching_minutes IS included: SN summed u_patching
//     separately alongside total, which only makes sense if patching is part
//     of total.
//  2. SN filtered on `week_starts_on` BETWEEN the earliest service start date
//     AND today, so a week straddling the boundary counted wholly on one side.
//     Postgres has the real `work_date`, so the filter is exact. No window is
//     applied at all here — every approved card on the project counts, which
//     matches what the entitlement is actually measuring.
//  3. SN issued one aggregate grouped by task and then TWO more aggregates per
//     task to split billable from non-billable — an N+1 over tasks. This is a
//     single grouped scan.
//
// entitlementHoursSQL is the port of ServiceNow's entitlement rule, from
// QueryHourUtils.updateProjectQueryDetails.
//
// The rule: for every opportunity product line funding this project whose
// service window covers today, add quantity x hours-per-pack. ServiceNow
// derives hours-per-pack by string-matching the product name, and this
// reproduces that exactly — including the fact that "Query support limit
// (included in subscription)" is a x1 pack while the Development Support
// packs are x10/25/50/100/200.
//
// Two things deliberately NOT changed from the original:
//
//  1. A product name outside the six contributes ZERO, silently. That is
//     ServiceNow's behaviour and changing it here would alter entitlements
//     without anyone deciding to. It is also the rule's weakest point — a
//     renamed or new pack is invisible — so `unmatched_line_count` is
//     reported alongside, making the silence observable.
//  2. `development_support_hours` is mirrored (migration 0080) and holds the
//     same number without any string matching, but ServiceNow ignores it and
//     it is unverified in production, so it is only REPORTED here, not used.
//     Switch to it once its population is confirmed.
const entitlementHoursSQL = `
    SELECT
        COALESCE(SUM(
            pr.quantity * CASE pr.product_name
                WHEN 'Development Support - 200 hours' THEN 200
                WHEN 'Development Support - 100 hours' THEN 100
                WHEN 'Development Support - 50 hours'  THEN 50
                WHEN 'Development Support - 25 hours'  THEN 25
                WHEN 'Development Support - 10 hours'  THEN 10
                WHEN 'Query support limit (included in subscription)' THEN 1
                ELSE 0
            END
        ), 0) AS entitlement_hours,
        COUNT(*) FILTER (WHERE pr.product_name NOT IN (
            'Development Support - 200 hours',
            'Development Support - 100 hours',
            'Development Support - 50 hours',
            'Development Support - 25 hours',
            'Development Support - 10 hours',
            'Query support limit (included in subscription)'
        )) AS unmatched_line_count,
        COUNT(*) AS active_line_count,
        COALESCE(SUM(pr.development_support_hours), 0) AS dev_support_hours
      FROM sf_opportunity_link l
      JOIN sf_opportunity_product pr ON pr.opportunity_id = l.opportunity_id
     WHERE l.project_id = $1
       AND pr.service_start_date <= CURRENT_DATE
       AND pr.service_end_date   >= CURRENT_DATE`

// consumptionSQL aggregates a project's approved time cards.
//
// Three deliberate differences from the ServiceNow original
// (QueryHourUtils.getConsumed):
//
//  1. SN summed `time_card.total`. The Postgres time_card has no such column —
//     it carries the five per-activity minute columns instead — so the total
//     is their sum. patching_minutes IS included: SN summed u_patching
//     separately alongside total, which only makes sense if patching is part
//     of total.
//  2. SN filtered on `week_starts_on` BETWEEN the earliest service start date
//     AND today, so a week straddling the boundary counted wholly on one side.
//     Postgres has the real `work_date`, so the filter is exact. No window is
//     applied at all here — every approved card on the project counts, which
//     matches what the entitlement is actually measuring.
//  3. SN issued one aggregate grouped by task and then TWO more aggregates per
//     task to split billable from non-billable — an N+1 over tasks. This is a
//     single grouped scan.
//
// `synced_entitlement_minutes` is ServiceNow's own figure, mirrored into
// project.total_query_duration. It is the fallback for a project with no
// opportunity lines yet, and during the parallel run it is what the derived
// figure gets compared against.
const consumptionSQL = `
SELECT
    p.id::text,
    COALESCE(p.key, ''),
    COALESCE(p.sf_id, ''),
    COALESCE(
        FLOOR(EXTRACT(EPOCH FROM p.total_query_duration) / 60)::int, 0
    ) AS synced_entitlement_minutes,
    COALESCE(SUM(
        CASE WHEN tc.is_billable IS TRUE THEN
            COALESCE(tc.analyzing_minutes, 0)
          + COALESCE(tc.setting_up_minutes, 0)
          + COALESCE(tc.reproducing_debugging_minutes, 0)
          + COALESCE(tc.providing_solution_minutes, 0)
          + COALESCE(tc.patching_minutes, 0)
        ELSE 0 END
    ), 0)::int AS billable_minutes,
    COALESCE(SUM(
        CASE WHEN tc.is_billable IS NOT TRUE THEN
            COALESCE(tc.analyzing_minutes, 0)
          + COALESCE(tc.setting_up_minutes, 0)
          + COALESCE(tc.reproducing_debugging_minutes, 0)
          + COALESCE(tc.providing_solution_minutes, 0)
          + COALESCE(tc.patching_minutes, 0)
        ELSE 0 END
    ), 0)::int AS non_billable_minutes
FROM project p
LEFT JOIN time_card tc
       ON tc.customer_project_id = p.id
      AND tc.state = 'APPROVED'
WHERE p.id = $1
GROUP BY p.id, p.key, p.sf_id, p.total_query_duration`

func (r *queryHourRepo) Consumption(ctx context.Context, projectID string) (domain.QueryHourConsumption, error) {
	var c domain.QueryHourConsumption
	err := r.db.QueryRow(ctx, consumptionSQL, projectID).Scan(
		&c.ProjectID, &c.ProjectKey, &c.ProjectSFID,
		&c.SyncedEntitlementMinutes, &c.BillableMinutes, &c.NonBillableMinutes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QueryHourConsumption{}, &apierror.NotFoundError{
			Msg: fmt.Sprintf("project %s not found", projectID)}
	}
	if err != nil {
		return domain.QueryHourConsumption{}, fmt.Errorf("aggregating query hours for project %s: %w", projectID, err)
	}

	// Entitlement is a separate query rather than a join: the consumption
	// aggregate is already grouped over time cards, and folding a second
	// one-to-many join into it would multiply the minute sums.
	var hours, devSupportHours float64
	var unmatched, activeLines int
	err = r.db.QueryRow(ctx, entitlementHoursSQL, projectID).Scan(
		&hours, &unmatched, &activeLines, &devSupportHours)
	if err != nil {
		// ONLY "relation does not exist" may be degraded. The fallback exists
		// for a database where csm-sync-service migration 0080 has not been
		// applied yet — a permanent, structural condition.
		//
		// Swallowing every error here would be far worse than it looks: a
		// cancelled context, a dropped connection or a statement timeout would
		// silently substitute ServiceNow's figure, which can differ from the
		// derived one. That changes the computed state, pushes a different
		// totalQueryTime to Choreo, and can publish a threshold email — and
		// the next sweep, succeeding, flips it all back. One transient blip
		// would cost a false push and two emails.
		// Literal SQLSTATE with a comment, matching problem_repo.go and
		// case_repo.go rather than pulling in pgerrcode for one constant.
		pgErr := (*pgconn.PgError)(nil)
		if !errors.As(err, &pgErr) || pgErr.Code != "42P01" { // undefined_table
			return domain.QueryHourConsumption{}, fmt.Errorf(
				"deriving query-hour entitlement for project %s: %w", projectID, err)
		}
		slog.Warn("query hours: opportunity tables absent (migration 0080 not applied), falling back to the synced ServiceNow figure",
			"projectId", projectID, "error", err)
		c.EntitlementMinutes = c.SyncedEntitlementMinutes
		c.EntitlementSource = domain.EntitlementSourceServiceNow
		return c, nil
	}

	c.ActiveLineCount = activeLines
	c.UnmatchedLineCount = unmatched
	c.DevSupportHours = devSupportHours

	if activeLines == 0 {
		// No opportunity line funds this project today. That is a real answer
		// for an expired or unlinked project, but it is indistinguishable from
		// "the link table has not synced yet", so ServiceNow's figure wins
		// until the derived one has something to say.
		c.EntitlementMinutes = c.SyncedEntitlementMinutes
		c.EntitlementSource = domain.EntitlementSourceServiceNow
		return c, nil
	}

	c.EntitlementMinutes = int(hours * 60)
	c.EntitlementSource = domain.EntitlementSourceOpportunityLines
	return c, nil
}

// upsertSQL writes the computed position AND reports the state it replaced, in
// one statement.
//
// The previous state has to come from here rather than a separate read. The
// caller publishes a threshold notice when the state crosses upward, and a
// read-then-write pair lets two concurrent recomputes — the hourly sweep and a
// time-card-triggered one for the same project — both observe the old state,
// both conclude they caused the crossing, and both publish. The owners and the
// standing cc groups would receive the same notice twice, which is precisely
// the double send this cutover exists to prevent.
//
// `prev` is safe to read alongside the insert: PostgreSQL guarantees that
// sub-statements in a WITH clause cannot see one another's effects on the
// target table, so it always reports the row as it was before this statement,
// regardless of the order the planner runs them in. FOR UPDATE then serialises
// concurrent callers on an existing row, so the second one reads the first's
// committed state rather than the stale one.
//
// When no row exists, `prev` is empty and previous_state comes back NULL. Both
// racers on a genuinely new project therefore see "first computation", and the
// service stays silent for it by design — so that case cannot double-send
// either.
const upsertSQL = `
WITH prev AS (
    SELECT query_hour_state
      FROM project_query_hours
     WHERE project_id = $1
       FOR UPDATE
),
up AS (
    INSERT INTO project_query_hours (
        project_id, entitlement_minutes, consumed_minutes,
        billable_minutes, non_billable_minutes, query_hour_state,
        computed_at, updated_at
    ) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
    ON CONFLICT (project_id) DO UPDATE SET
        entitlement_minutes  = EXCLUDED.entitlement_minutes,
        consumed_minutes     = EXCLUDED.consumed_minutes,
        billable_minutes     = EXCLUDED.billable_minutes,
        non_billable_minutes = EXCLUDED.non_billable_minutes,
        query_hour_state     = EXCLUDED.query_hour_state,
        computed_at          = NOW(),
        updated_at           = NOW()
    RETURNING project_id::text, entitlement_minutes, consumed_minutes,
              billable_minutes, non_billable_minutes, query_hour_state,
              last_pushed_state, last_pushed_at, computed_at
)
SELECT up.*, (SELECT query_hour_state FROM prev) AS previous_state
  FROM up`

func (r *queryHourRepo) Upsert(ctx context.Context, c domain.QueryHourConsumption, state int) (domain.ProjectQueryHours, *int, error) {
	var q domain.ProjectQueryHours
	var previousState *int
	err := r.db.QueryRow(ctx, upsertSQL,
		c.ProjectID, c.EntitlementMinutes, c.ConsumedMinutes(),
		c.BillableMinutes, c.NonBillableMinutes, state,
	).Scan(
		&q.ProjectID, &q.EntitlementMinutes, &q.ConsumedMinutes,
		&q.BillableMinutes, &q.NonBillableMinutes, &q.QueryHourState,
		&q.LastPushedState, &q.LastPushedAt, &q.ComputedAt,
		&previousState,
	)
	if err != nil {
		return domain.ProjectQueryHours{}, nil, fmt.Errorf("upserting query hours for project %s: %w", c.ProjectID, err)
	}
	q.ProjectKey = c.ProjectKey
	q.ProjectSFID = c.ProjectSFID
	return q, previousState, nil
}

func (r *queryHourRepo) MarkPushed(ctx context.Context, projectID string, state int, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE project_query_hours
		   SET last_pushed_state = $2, last_pushed_at = $3, updated_at = NOW()
		 WHERE project_id = $1`, projectID, state, at)
	if err != nil {
		return fmt.Errorf("marking query-hour push for project %s: %w", projectID, err)
	}
	return nil
}

const getSQL = `
SELECT q.project_id::text,
       COALESCE(p.key, ''), COALESCE(p.sf_id, ''),
       q.entitlement_minutes, q.consumed_minutes,
       q.billable_minutes, q.non_billable_minutes, q.query_hour_state,
       q.last_pushed_state, q.last_pushed_at, q.computed_at
  FROM project_query_hours q
  LEFT JOIN project p ON p.id = q.project_id
 WHERE q.project_id = $1`

func (r *queryHourRepo) Get(ctx context.Context, projectID string) (domain.ProjectQueryHours, error) {
	var q domain.ProjectQueryHours
	err := r.db.QueryRow(ctx, getSQL, projectID).Scan(
		&q.ProjectID, &q.ProjectKey, &q.ProjectSFID,
		&q.EntitlementMinutes, &q.ConsumedMinutes,
		&q.BillableMinutes, &q.NonBillableMinutes, &q.QueryHourState,
		&q.LastPushedState, &q.LastPushedAt, &q.ComputedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProjectQueryHours{}, &apierror.NotFoundError{
			Msg: fmt.Sprintf("no query-hour position recorded for project %s", projectID)}
	}
	if err != nil {
		return domain.ProjectQueryHours{}, fmt.Errorf("reading query hours for project %s: %w", projectID, err)
	}
	return q, nil
}

// staleSQL returns projects needing a recompute: those whose stored row is
// older than the cutoff, and those that have approved time cards but no row
// at all. Ordered oldest-first (NULLS FIRST puts never-computed projects at
// the front) so a limited sweep makes progress rather than re-doing the same
// projects every run.
const staleSQL = `
SELECT p.id::text
  FROM project p
  LEFT JOIN project_query_hours q ON q.project_id = p.id
 WHERE (q.computed_at IS NULL OR q.computed_at < $1)
   AND (
        q.project_id IS NOT NULL
     OR EXISTS (SELECT 1 FROM time_card tc
                 WHERE tc.customer_project_id = p.id AND tc.state = 'APPROVED')
   )
 ORDER BY q.computed_at ASC NULLS FIRST, p.id
 LIMIT $2`

func (r *queryHourRepo) StaleProjectIDs(ctx context.Context, olderThan time.Time, limit int) ([]string, error) {
	rows, err := r.db.Query(ctx, staleSQL, olderThan, limit)
	if err != nil {
		return nil, fmt.Errorf("listing stale query-hour projects: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning stale query-hour project: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stale query-hour projects: %w", err)
	}
	return ids, nil
}

func (r *queryHourRepo) ProjectIDForTimeCard(ctx context.Context, timeCardID string) (string, error) {
	var projectID *string
	err := r.db.QueryRow(ctx,
		`SELECT customer_project_id::text FROM time_card WHERE id = $1`, timeCardID).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &apierror.NotFoundError{Msg: fmt.Sprintf("time card %s not found", timeCardID)}
	}
	if err != nil {
		return "", fmt.Errorf("resolving project for time card %s: %w", timeCardID, err)
	}
	// customer_project_id is nullable (ON DELETE SET NULL). A card with no
	// project has nothing to recompute; that is a client error, not a crash.
	if projectID == nil || *projectID == "" {
		return "", &apierror.NotFoundError{
			Msg: fmt.Sprintf("time card %s is not linked to a project", timeCardID)}
	}
	return *projectID, nil
}

// notificationContextSQL resolves the threshold email's audience.
//
// ServiceNow's flow walked account.u_owner and account.u_technical_owner as
// GlideRecord references and read `email` off each. The synced equivalents are
// account.account_manager_id and account.technical_owner_id, both FKs to
// "user" — see csm-sync-service's customer_account mapping, which maps u_owner
// onto account_manager_id.
//
// The @wso2.com filter is NOT applied here. It is a business rule about who
// may read an internal usage notice, so it lives in the service alongside the
// rest of the recipient assembly, the same way the engagement port put its own
// filter in front of the publish rather than in SQL.
const notificationContextSQL = `
SELECT COALESCE(a.name, ''),
       COALESCE(p.name, ''),
       COALESCE(am.email, ''),
       COALESCE(towner.email, ''),
       COALESCE(NULLIF(TRIM(CONCAT_WS(' ', am.first_name, am.last_name)), ''), '')
  FROM project p
  LEFT JOIN account a      ON a.id = p.account_id
  LEFT JOIN "user" am      ON am.id = a.account_manager_id
  LEFT JOIN "user" towner  ON towner.id = a.technical_owner_id
 WHERE p.id = $1`

func (r *queryHourRepo) NotificationContext(ctx context.Context, projectID string) (domain.QueryHourNotificationContext, error) {
	var c domain.QueryHourNotificationContext
	err := r.db.QueryRow(ctx, notificationContextSQL, projectID).Scan(
		&c.AccountName, &c.ProjectName, &c.AccountManagerEmail, &c.TechnicalOwnerEmail,
		&c.AccountManagerName)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QueryHourNotificationContext{}, &apierror.NotFoundError{
			Msg: fmt.Sprintf("project %s not found", projectID)}
	}
	if err != nil {
		return domain.QueryHourNotificationContext{}, fmt.Errorf(
			"resolving query-hour notification context for project %s: %w", projectID, err)
	}
	return c, nil
}
