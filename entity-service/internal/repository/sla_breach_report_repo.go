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
	"sort"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/businessdays"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SLABreachReportRepository lists SLA clocks that breached inside a window.
//
// Internal-caller-only, like SLAStatusRepository: the report is mailed to
// an internal distribution list and has no per-caller scoping of its own.
type SLABreachReportRepository interface {
	ListBreaches(ctx context.Context, from, to time.Time, definitions, stages, onboardingStatuses, excludeOnboardingStatuses []string) ([]domain.SLABreachRow, error)
}

type slaBreachReportRepo struct {
	db *Scoped
}

// NewSLABreachReportRepository constructs a SLABreachReportRepository.
func NewSLABreachReportRepository(db *Scoped) SLABreachReportRepository {
	return &slaBreachReportRepo{db: db}
}

// slaBreachReportQuery lists every clock that has breached and could
// fall in the reported week. The week itself is decided in Go, by
// ListBreaches -- see slaBreachedOn below for why.
//
// It reads `sla_live`, not `sla`, for three columns a retrospective
// report cannot get from the stored row:
//
//   - live_has_breached. The engine only writes has_breached when a clock
//     STOPS (ACHIEVED/CANCELLED/COMPLETED, computed as elapsed >=
//     duration at that moment). A clock still running past its due time
//     has has_breached = FALSE, forever, even though it has plainly
//     breached -- 126,683 of staging's 129,172 breached clocks sit at
//     IN_PROGRESS. live_has_breached is that same comparison made at
//     query time, and falls back to the stored flag for every stopped
//     row, so it is a superset of the old predicate, never a different
//     one.
//   - live_stage, which reads BREACHED for such a clock. That is the
//     stage a caller filtering on stages means by it.
//   - live_elapsed_percentage. business_elapsed_percentage is written at
//     registration (0) and only recomputed when the clock stops, so a
//     running clock's stored percentage reports 0% in a report whose
//     whole subject is how far past its target it ran.
//
// is_active is deliberately NOT filtered: a clock on a closed case is
// exactly what "what breached last week" exists to show.
//
// A PAUSED clock is admitted separately. live_has_breached is only
// recomputed for raw stage IN_PROGRESS; for every other stage the view
// returns the stored has_breached, and SetPaused freezes a clock's
// elapsed values without ever writing that flag. An engine clock that
// ran past its target and was then paused therefore reads
// has_breached = FALSE forever, and the CRE variant -- which lists
// PAUSED among its stages -- would never see it. Its frozen
// business_elapsed_percentage, which live_elapsed_percentage falls back
// to for exactly these rows, is the breach test the flag should have
// been: >= 100 means it had already breached when it paused. The open
// pause is not counted, since the frozen value stopped advancing at
// paused_on, and a clock stopped on time still reads under 100 and
// stays out.
//
// The window predicate is in two halves. A row with a recorded instant
// is filtered on that instant directly. A row without one is admitted on
// start_on -- a cheap, index-friendly bound, since a clock cannot breach
// before it starts and a business-day one cannot take more wall-clock
// time than twice its duration plus a week of weekends. Pause is part of
// that bound because it is part of the derived instant below: a clock
// that spent a month paused breaches a month further from its start_on
// than its duration alone allows for, and a bound that ignored that
// would drop it from the candidate set before Go ever saw it. Those
// candidates are narrowed to the exact week in Go.
//
// The join set mirrors activeSLAStatusFromJoins' own account/product/
// project joins, minus the ones only live status needs. caseLikeJoins is
// deliberately not pulled in: its only unique contribution here would be
// caseLikeStateColumn, which this report doesn't show, and it LEFT JOINs
// the RLS-protected `announcement` table for no benefit.
//
// `cre` is the account's CRE group -- ServiceNow's
// account.u_integration_cs_team, the field the source report grouped on.
//
// The product comes from the case's DEPLOYED product first, falling back
// to work_item.product_id. That is the path the portal itself resolves a
// case's product by (case_repo.go joins deployed_products -> products and
// shows nothing else), so the two now agree; keying on work_item.product_id
// alone left the column empty for every case that carries only a deployed
// product -- 1,195 of staging's 13,535 cases, and every row of the first
// production run.
const slaBreachReportQuery = `
	SELECT
		COALESCE(cre.name, ''),
		COALESCE(wi.wso2_id, ''),
		COALESCE(wi.number, ''),
		COALESCE(prod.name, ''),
		COALESCE(c.issue_type::TEXT, ''),
		COALESCE(c.severity::TEXT, ''),
		COALESCE(sp.target::TEXT, ''),
		COALESCE(s.live_elapsed_percentage, 0),
		sp.name,
		s.original_breached_on,
		COALESCE(p.onboarding_status::TEXT, ''),
		s.start_on,
		COALESCE(EXTRACT(EPOCH FROM sp.duration), 0),
		COALESCE(sp.schedule, ''),
		COALESCE(EXTRACT(EPOCH FROM s.pause_duration), 0),
		COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0)
	FROM sla_live s
	JOIN sla_policy sp ON sp.id = s.sla_policy_id
	JOIN work_item wi ON wi.id = s.work_item_id
	LEFT JOIN "case" c ON c.id = wi.id
	LEFT JOIN deployed_product dprod ON dprod.id = wi.deployed_product_id
	LEFT JOIN product prod ON prod.id = COALESCE(dprod.product_id, wi.product_id)
	LEFT JOIN account a ON a.id = wi.account_id
	LEFT JOIN "group" cre ON cre.id = a.cre_team_id
	LEFT JOIN project p ON p.id = wi.project_id
	WHERE wi.type = ANY(` + caseLikeWorkItemTypes + `)
	  AND (s.live_has_breached IS TRUE
	       OR (s.live_stage::TEXT = 'PAUSED' AND COALESCE(s.live_elapsed_percentage, 0) >= 100))
	  AND ((s.original_breached_on >= $1 AND s.original_breached_on < $2)
	       OR (s.original_breached_on IS NULL
	           AND s.start_on IS NOT NULL
	           AND s.start_on < $2
	           AND s.start_on >= $1::timestamptz
	               - ((sp.duration
	                   + COALESCE(s.pause_duration, INTERVAL '0')
	                   + COALESCE(s.business_pause_duration, INTERVAL '0')) * 2
	                  + INTERVAL '7 days')))
	  AND sp.name = ANY($3)
	  AND ($4::TEXT[] IS NULL OR cardinality($4::TEXT[]) = 0 OR s.live_stage::TEXT = ANY($4))
	  AND ($5::TEXT[] IS NULL OR cardinality($5::TEXT[]) = 0 OR p.onboarding_status::TEXT = ANY($5))
	  -- "is not one of" keeps a case with no project, and so no status:
	  -- ServiceNow's own operator does not discard an empty value, and
	  -- dropping those rows here would silently shrink the report.
	  AND ($6::TEXT[] IS NULL OR cardinality($6::TEXT[]) = 0
	       OR p.onboarding_status IS NULL OR NOT (p.onboarding_status::TEXT = ANY($6)))`

// slaBreachedOn is WHEN a clock breached.
//
// original_breached_on is the recorded instant -- but nothing in this
// platform writes that column. It was only ever populated by the
// previous system's own sync, so every clock the CSM SLA engine breaches
// has it NULL, and a report keyed on it alone reports nothing at all.
// That is not hypothetical: in production a portal filter showed ~130
// currently-breached cases while this report returned zero rows for the
// same week, because every one of those clocks was engine-written.
//
// So the instant is derived when it was never recorded: the moment the
// clock's elapsed time reached its policy's duration, which is start_on
// advanced by (duration + the pause already taken). A '24 x 7' policy is
// plain arithmetic; a 'WSO2 Business days' one goes through
// businessdays.Add, the exact inverse of the businessdays.Elapsed this
// service already measures business time with, so a derived instant and
// a live percentage can never disagree about where a weekend begins.
// Deriving it here rather than in SQL keeps the query a plain filter and
// needs nothing added to the database.
//
// A recorded instant always wins, so the previous system's own synced
// history reports exactly as it did before this derivation existed.
//
// Caveat, deliberate, and it costs more than precision: a clock paused
// AFTER it breached has that later pause folded into the budget, which
// pushes the derived instant later than the real one -- far enough to
// change which week the clock is reported in. A 48-hour clock started on
// the 16th breaches on the 18th and is mailed in that week's report; if
// it then pauses from the 24th to the 30th, pause_duration grows on
// resume, the derived instant moves to the 24th, and the NEXT week's
// report mails the same breach again. Two sends, one breach.
//
// It is not fixable from here. Nothing in the row distinguishes pause
// taken before the breach from pause taken after it -- only the
// aggregate survives -- so recovering the first instant means writing it
// when the engine detects the breach, which is a write to `sla` from the
// SLA engine and not something a report may do.
//
// What bounds it: a recorded instant always wins, so none of the synced
// history this report has been mailing is affected; a clock that is
// still paused has pause_duration excluding the open pause, so its
// derived instant is right; and the duplicate is across two weekly
// sends, never within one report.
func slaBreachedOn(recorded *time.Time, startOn *time.Time, durationSecs, pauseSecs, businessPauseSecs float64, schedule string) (time.Time, bool) {
	if recorded != nil {
		return *recorded, true
	}
	if startOn == nil || durationSecs <= 0 {
		return time.Time{}, false
	}
	if schedule == slaScheduleWSO2BusinessDays {
		budget := time.Duration((durationSecs + businessPauseSecs) * float64(time.Second))
		return businessdays.Add(*startOn, budget), true
	}
	return startOn.Add(time.Duration((durationSecs + pauseSecs) * float64(time.Second))), true
}

// ListBreaches runs slaBreachReportQuery and narrows its candidates to
// the clocks whose breach instant falls in [from, to), ordered the way
// the report renders them.
func (r *slaBreachReportRepo) ListBreaches(ctx context.Context, from, to time.Time, definitions, stages, onboardingStatuses, excludeOnboardingStatuses []string) ([]domain.SLABreachRow, error) {
	// work_item is RLS-protected (see migration 100003 and the repo's own
	// RLS notes): without an explicit scope every row is filtered out and
	// the report looks like a quiet week rather than a broken query. This
	// endpoint is internal-caller-only, so Unrestricted is correct here --
	// the same reasoning SearchActiveSLAStatuses' own call documents.
	ctx = WithCallerIdentity(ctx, SearchScope{Unrestricted: true})

	rows, err := r.db.Query(ctx, slaBreachReportQuery, from, to, definitions, stages, onboardingStatuses, excludeOnboardingStatuses)
	if err != nil {
		return nil, fmt.Errorf("query sla breaches: %w", err)
	}
	defer rows.Close()

	// Non-nil even with zero rows, so the response serializes as [] rather
	// than null.
	out := make([]domain.SLABreachRow, 0)
	for rows.Next() {
		var (
			row               domain.SLABreachRow
			recorded          *time.Time
			startOn           *time.Time
			durationSecs      float64
			schedule          string
			pauseSecs         float64
			businessPauseSecs float64
		)
		if err := rows.Scan(
			&row.Team, &row.WSO2CaseID, &row.CaseNumber, &row.Product,
			&row.CaseType, &row.Priority, &row.Target, &row.BusinessElapsedPercent,
			&row.Definition, &recorded, &row.OnboardingStatus,
			&startOn, &durationSecs, &schedule, &pauseSecs, &businessPauseSecs,
		); err != nil {
			return nil, fmt.Errorf("scan sla breach row: %w", err)
		}
		breachedOn, ok := slaBreachedOn(recorded, startOn, durationSecs, pauseSecs, businessPauseSecs, schedule)
		if !ok || breachedOn.Before(from) || !breachedOn.Before(to) {
			continue
		}
		row.BreachedOn = breachedOn
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sla breaches: %w", err)
	}

	// Ordered here rather than in SQL: a derived instant does not exist
	// until the loop above has computed it, and the report groups on the
	// CRE team, so the ordering has to hold across the whole result.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Team != out[j].Team {
			return out[i].Team < out[j].Team
		}
		if out[i].Definition != out[j].Definition {
			return out[i].Definition < out[j].Definition
		}
		return out[i].BreachedOn.Before(out[j].BreachedOn)
	})
	return out, nil
}
