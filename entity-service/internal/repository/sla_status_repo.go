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

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// SLAStatusRepository defines the read operations backing GET /sla-status —
// see domain.SLAStatus's own doc comment for what it replaced and why.
type SLAStatusRepository interface {
	// SearchActiveSLAStatuses returns every currently-active (sla.is_active =
	// true) clock across every case-like work item, one row per
	// (work_item, sla_policy.target), paginated. sourceFilter, when
	// non-empty, must be a real sla_source_enum label ("CSM"/"SERVICENOW")
	// and narrows the result to just that source -- added for
	// csm-notification-service's own Redis-recovery reconciliation pass
	// (source=CSM), which only ever needs this engine's own rows and would
	// otherwise pay the cost of scanning the full, much larger
	// previous-system-synced row set for nothing. Empty means no filter, the
	// original, unscoped behavior.
	SearchActiveSLAStatuses(ctx context.Context, pagination domain.Pagination, sourceFilter string) ([]domain.SLAStatus, int, error)

	// GetClockState returns the single most recently-started "sla" row for
	// (workItemID, target) -- regardless of is_active -- so a caller can
	// tell a genuinely resolved clock (a real row, in a terminal stage)
	// apart from one that was simply never registered at all (no row
	// found), which SearchActiveSLAStatuses's own is_active-filtered list
	// can never distinguish between (both look like "not in the active
	// list"). Added for csm-notification-service's own pre-alert
	// verification -- see that repo's own CLAUDE.md, "SLA breach-alerting
	// engine" -- which needs exactly that distinction to avoid either
	// suppressing a genuine alert over an unrelated registration failure,
	// or confirming the wrong clock incarnation after a severity revision.
	// target must be a real sla_policy_target_enum label ("RESPONSE"/
	// "WORKAROUND"/"RESOLUTION"); sourceFilter follows the same convention
	// as SearchActiveSLAStatuses's own (empty/"CSM"/"SERVICENOW").
	// domain.SLAClockState.Found is false, every other field at its zero
	// value, when no row exists for this (workItemID, target, source) at
	// all.
	GetClockState(ctx context.Context, workItemID, target, sourceFilter string) (domain.SLAClockState, error)
}

type slaStatusRepo struct {
	db *Scoped
}

// NewSLAStatusRepository constructs an SLAStatusRepository backed by the
// given connection pool.
func NewSLAStatusRepository(db *Scoped) SLAStatusRepository {
	return &slaStatusRepo{db: db}
}

// activeSLAStatusCTE is shared by the count and page queries below. It picks
// the one relevant "sla" row per (work_item, sla_policy.target): "sla" can
// carry more than one row per pair (a policy reset re-applies the SLA, see
// the live count in CLAUDE.md), so this takes the most recently started one,
// falling back to most recently updated for the rare row with no start_on.
// sp.target IS NOT NULL excludes the handful of "sla" rows (3, checked live)
// whose policy has no target set at all -- nothing this endpoint could label
// as a clock type.
//
// Also carries schedule/business_pause_duration so the data query's own
// scanSLAStatus can apply the same business-day correction GetClockState
// already applies to a single clock -- see scanSLAStatus's own comment.
//
// sourceArgIndex, when > 0, adds "AND s.source = $<sourceArgIndex>" -- the
// placeholder's position differs between the count query (which otherwise
// binds nothing) and the data query (which already binds limit/offset/the
// evaluation-subscription project type name), so the caller passes whichever
// index is next free in its own query rather than this function assuming one.
// 0 means no source filter at all, the original unscoped behavior.
// Known, deliberate limitation (see entity-service/CLAUDE.md's own note):
// `DISTINCT ON (s.work_item_id, sp.target)` picks exactly one row per
// target, but the per-policy registration guard allows two policies to both
// have a genuinely open (is_active = TRUE) clock on the same target at
// once -- `s.is_active` already excludes every terminal row (CANCELLED/
// ACHIEVED/COMPLETED all set it FALSE), so the remaining ambiguity is
// narrower than GetClockState's own (which has none of that filter, by
// design -- see that method's own doc comment): only between two
// simultaneously-open rows, where `ORDER BY ... start_on DESC NULLS LAST,
// updated_on DESC` below picks the more recently started/updated one.
// Keying this read by clock id or policy instead of target would be the
// real fix, deliberately out of scope here.
func activeSLAStatusCTE(sourceArgIndex int) string {
	sourceFilter := ""
	if sourceArgIndex > 0 {
		sourceFilter = fmt.Sprintf(" AND s.source = $%d::sla_source_enum", sourceArgIndex)
	}
	return `
	WITH active_sla AS (
		SELECT DISTINCT ON (s.work_item_id, sp.target)
			s.work_item_id, sp.target, s.live_elapsed_percentage AS business_elapsed_percentage,
			s.live_has_breached AS has_breached, s.live_stage AS stage, s.start_on,
			s.paused_on, s.pause_duration, s.duration,
			COALESCE(sp.schedule, '') AS schedule, s.business_pause_duration
		FROM sla_live s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.is_active AND sp.target IS NOT NULL` + sourceFilter + `
		ORDER BY s.work_item_id, sp.target, s.start_on DESC NULLS LAST, s.updated_on DESC
	)`
}

// activeSLAStatusFromJoins resolves each row's case-like display data --
// mirrors caseRepo.GetCaseByID's own product/severity joins exactly (see
// that query's own comments for why), restricted to case-like types only.
// The account/project/project_type joins exist purely to feed
// csm-notification-service's own Chat-audience routing (Team, onboarding
// status, evaluation-account flag) — see domain.SLAStatus's own doc
// comment for each field.
const activeSLAStatusFromJoins = `
	FROM active_sla als
	JOIN work_item wi ON wi.id = als.work_item_id
	LEFT JOIN "case" c ON c.id = wi.id
	` + caseLikeJoins + `
	LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
	LEFT JOIN product prod ON prod.id = dp.product_id
	LEFT JOIN product_version pv ON pv.id = dp.version_id
	LEFT JOIN account a ON a.id = wi.account_id
	LEFT JOIN "group" cre ON cre.id = a.cre_team_id
	LEFT JOIN "user" teamlead ON teamlead.id = cre.manager_id
	LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
	LEFT JOIN project p ON p.id = wi.project_id
	LEFT JOIN project_type pt ON pt.id = p.project_type_id
	WHERE wi.type = ANY(` + caseLikeWorkItemTypes + `)`

// evaluationSubscriptionProjectTypeName is project_type.name's exact value
// for the "Evaluation Subscription" type. Matched by name, not a hardcoded
// id: project_type isn't seeded by this repo's own migrations (it's
// populated by an external sync), so nothing guarantees a given row's id
// is the same across environments -- name has a UNIQUE constraint and is
// the stable, portable key here.
const evaluationSubscriptionProjectTypeName = "Evaluation Subscription"

// stringOrEmpty returns "" for a nil column value rather than propagating a
// nil *string into a domain type's plain string fields. Shared across this
// package's repositories (was previously defined alongside the now-removed
// sla_clocks repository).
func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// nullIfEmpty returns nil for an empty string so it's stored as SQL NULL
// rather than an empty-string value — matches stringOrEmpty's read-back
// convention above.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// scanSLAStatus reads one row of SearchActiveSLAStatuses' data query.
//
// BusinessElapsedPercent is corrected the same way GetClockState's own doc
// comment explains: als.business_elapsed_percentage (sla_live's view column)
// is a flat wall-clock figure with no business-day calendar of its own, so
// for a still-running (IN_PROGRESS/BREACHED) clock bound to a 'WSO2 Business
// days' policy it is wrong -- a Friday-started clock 's "live" percentage
// drifts over the weekend when nothing should be accumulating at all. Before
// this fix, GetClockState (the single-clock detail view) silently
// overwrote that same wrong figure while this bulk path (the CSM Portal
// table view) did not, so the same clock showed two different percentages
// in the two views. Recomputed here via businessDayElapsedSnapshot -- the
// exact same formula GetClockState/CompleteClock/SetPaused/RecomputeActive
// all share -- rather than duplicated. A PAUSED or terminal row's stored
// figure was already frozen correctly, business-day-aware, at the moment it
// was written, so only the live stages are recomputed.
func scanSLAStatus(row interface{ Scan(...any) error }) (domain.SLAStatus, error) {
	var s domain.SLAStatus
	var target, severity, caseType string
	var caseNumber, wso2CaseID, caseTitle, productName, state, teamName, onboardingStatus *string
	var teamEmail, teamLeadName, assigneeName, assigneeEmail *string
	var stage, schedule string
	var isEvaluation bool
	var pauseDurationSeconds *int64
	var durationSeconds int64
	var businessPauseSeconds float64
	err := row.Scan(
		&s.CaseID, &target, &s.BusinessElapsedPercent, &s.HasBreached, &stage, &s.StartedOn,
		&s.PausedOn, &pauseDurationSeconds, &durationSeconds, &schedule, &businessPauseSeconds,
		&caseNumber, &wso2CaseID, &caseTitle, &caseType,
		&productName, &severity, &state, &teamName, &onboardingStatus, &isEvaluation,
		&teamEmail, &teamLeadName, &assigneeName, &assigneeEmail,
	)
	if err != nil {
		return domain.SLAStatus{}, err
	}
	s.ClockType = strings.ToLower(target)
	s.IsPaused = stage == "PAUSED"
	if pauseDurationSeconds != nil {
		s.PauseDurationSeconds = *pauseDurationSeconds
	}
	s.DurationSeconds = durationSeconds
	s.Schedule = schedule
	if _, breached, pct, ok := businessDayCorrectedLiveState(schedule, stage, s.StartedOn, businessPauseSeconds, float64(durationSeconds)); ok {
		// Both BusinessElapsedPercent AND HasBreached are corrected together
		// here -- an earlier version of this method corrected only the
		// percentage, leaving HasBreached (and, in GetClockState's own
		// equivalent correction, Stage) to still come from sla_live's own
		// flat-time computation. That let a business-day clock report
		// HasBreached=true right next to a corrected percentage well under
		// 100 -- one response disagreeing with itself. See
		// businessDayCorrectedLiveState's own doc comment for the confirmed
		// example.
		s.BusinessElapsedPercent = pct
		s.HasBreached = breached
	}
	s.CaseNumber = stringOrEmpty(caseNumber)
	s.WSO2CaseID = stringOrEmpty(wso2CaseID)
	s.CaseTitle = stringOrEmpty(caseTitle)
	s.CaseType = caseType
	s.Product = stringOrEmpty(productName)
	s.State = stringOrEmpty(state)
	s.Team = stringOrEmpty(teamName)
	s.ProjectOnboardingStatus = stringOrEmpty(onboardingStatus)
	s.IsEvaluationAccount = isEvaluation
	s.TeamEmail = stringOrEmpty(teamEmail)
	s.TeamLeadName = stringOrEmpty(teamLeadName)
	s.AssigneeName = stringOrEmpty(assigneeName)
	s.AssigneeEmail = stringOrEmpty(assigneeEmail)
	if sev, ok := caseSeverityFromEnum[severity]; ok {
		s.Priority = strings.ToUpper(string(sev))
	}
	return s, nil
}

// SearchActiveSLAStatuses implements SLAStatusRepository.
func (r *slaStatusRepo) SearchActiveSLAStatuses(ctx context.Context, pagination domain.Pagination, sourceFilter string) ([]domain.SLAStatus, int, error) {
	// Placeholder numbering: the count query binds only the source filter
	// (if present, as $1); the data query already binds limit/offset/the
	// evaluation-subscription project type name as $1-$3, so the source
	// filter there is $4. activeSLAStatusCTE takes 0 to omit the filter
	// entirely, keeping both queries' SQL text byte-identical to before this
	// parameter existed when sourceFilter is "".
	countArgs := []any{}
	countSourceArg := 0
	if sourceFilter != "" {
		countSourceArg = 1
		countArgs = append(countArgs, sourceFilter)
	}
	dataArgs := []any{pagination.Limit, pagination.Offset, evaluationSubscriptionProjectTypeName}
	dataSourceArg := 0
	if sourceFilter != "" {
		dataSourceArg = 4
		dataArgs = append(dataArgs, sourceFilter)
	}

	countQuery := activeSLAStatusCTE(countSourceArg) + `
		SELECT COUNT(*)
		` + activeSLAStatusFromJoins

	dataQuery := activeSLAStatusCTE(dataSourceArg) + `
		SELECT als.work_item_id::TEXT, als.target::TEXT, COALESCE(als.business_elapsed_percentage, 0), COALESCE(als.has_breached, FALSE), COALESCE(als.stage::TEXT, ''), als.start_on,
		       als.paused_on, EXTRACT(EPOCH FROM als.pause_duration)::BIGINT,
		       COALESCE(EXTRACT(EPOCH FROM als.duration), 0)::BIGINT,
		       als.schedule, COALESCE(EXTRACT(EPOCH FROM als.business_pause_duration), 0),
		       wi.number, wi.wso2_id, wi.subject, wi.type::TEXT,
		       prod.name || COALESCE(' ' || pv.version, ''), COALESCE(c.severity::TEXT, ''),
		       ` + caseLikeStateColumn + `,
		       cre.name, p.onboarding_status::TEXT, COALESCE(pt.name = $3, FALSE),
		       cre.group_email, COALESCE(teamlead.name, NULLIF(TRIM(CONCAT_WS(' ', teamlead.first_name, teamlead.last_name)), '')),
		       COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')), ae.email
		` + activeSLAStatusFromJoins + `
		ORDER BY als.work_item_id, als.target
		LIMIT $1 OFFSET $2`

	var total int
	var statuses []domain.SLAStatus

	// Both queries join caseLikeStateColumn/caseLikeJoins, which LEFT JOINs
	// the RLS-protected `announcement` table (migration 000085). This
	// endpoint has no caller-scoped filtering of its own -- it's an
	// internal-caller-only read (see SLAStatusRepository's doc comment) -- so
	// Unrestricted is the correct scope here, not a resolved user scope: it
	// still must be set explicitly, in the same transaction as each query,
	// or a restricted announcement's state/severity columns come back NULL
	// instead of their real values. Stamped onto ctx once, then both Scoped
	// calls below pick it up automatically -- same convention as
	// case_repo.go's GetCaseByID/SearchCases and global_search_repo.go's
	// runSearch, all of which take an explicit scope rather than relying on
	// whatever identity ctx already carries.
	ctx = WithCallerIdentity(ctx, SearchScope{Unrestricted: true})

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, countArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count active sla statuses: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query active sla statuses: %w", err)
		}
		defer rows.Close()
		result := make([]domain.SLAStatus, 0, pagination.Limit)
		for rows.Next() {
			st, err := scanSLAStatus(rows)
			if err != nil {
				return fmt.Errorf("scan sla status: %w", err)
			}
			result = append(result, st)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate active sla statuses: %w", err)
		}
		statuses = result
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return statuses, total, nil
}

// GetClockState implements SLAStatusRepository. Deliberately reads
// sla_live (not the raw "sla" table), matching SearchActiveSLAStatuses's
// own convention, so a still-live clock's stage/has_breached reflect the
// current, recomputed-at-read-time truth rather than whatever
// RecomputeActive's last periodic pass happened to write. Reads directly
// against "sla"/"sla_policy" with no work_item/case/announcement join at
// all -- unlike SearchActiveSLAStatuses, this never needs to resolve any
// case-like display data, so there's nothing here that depends on
// row-level security (confirmed: the "sla" table itself carries none).
//
// BusinessElapsedPercent is NOT simply read off sla_live's own
// live_elapsed_percentage for every row: that view has no business-day
// calendar of its own (it only knows the flat wall-clock formula --
// see migration 0204/0245's own doc comments), so for a still-running
// (IN_PROGRESS/BREACHED) clock bound to a 'WSO2 Business days' policy, this
// method instead computes the live figure itself, through
// businessDayElapsedSnapshot (sla_engine_repo.go -- the exact same formula
// CompleteClock/SetPaused/RecomputeActive use, reused rather than
// duplicated here). Every other case -- 24x7, or PAUSED/terminal regardless
// of schedule -- reads the view's own value directly, which is already
// correct for those (a PAUSED/terminal row's figure was already frozen
// correctly, business-day-aware, at the moment it was written).
func (r *slaStatusRepo) GetClockState(ctx context.Context, workItemID, target, sourceFilter string) (domain.SLAClockState, error) {
	args := []any{workItemID, target}
	sourceFilterSQL := ""
	if sourceFilter != "" {
		sourceFilterSQL = " AND s.source = $3::sla_source_enum"
		args = append(args, sourceFilter)
	}

	// Known, deliberate limitation (see entity-service/CLAUDE.md's own note
	// on this): the per-policy registration guard lets two policies
	// register clocks against the same (work_item, target) at once, and
	// this read still returns only one row, picked by ORDER BY/LIMIT 1
	// below. Deliberately NOT given a stage filter to resolve that: this
	// method's whole reason for existing (see its own doc comment) is
	// returning the most recently-started row REGARDLESS of stage/
	// is_active, so a caller can distinguish "cleanly resolved" from
	// "never registered" from "superseded by a later incarnation" -- a
	// CodeRabbit review already caught and fixed the bug an active-only
	// filter causes here once. Excluding terminal stages now would
	// reintroduce exactly that bug for the sake of a narrower, much rarer
	// problem (two policies simultaneously open on one target); keying the
	// read -- and csm-notification-service's own Redis clock -- by clock id
	// or policy instead of target is the real fix, deliberately out of
	// scope here.
	query := `
		SELECT s.is_active, s.live_stage::TEXT, s.live_has_breached, s.start_on,
		       s.paused_on, EXTRACT(EPOCH FROM s.pause_duration)::BIGINT,
		       COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0),
		       COALESCE(EXTRACT(EPOCH FROM s.duration), 0),
		       COALESCE(s.live_elapsed_percentage, 0),
		       COALESCE(sp.schedule, '')
		FROM sla_live s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.work_item_id = $1::uuid AND sp.target::TEXT = $2` + sourceFilterSQL + `
		ORDER BY s.start_on DESC NULLS LAST, s.updated_on DESC
		LIMIT 1`

	// Scoped requires an identity on ctx even for a table with no RLS of its
	// own (see NewSLAEngineRepository's own doc comment for the identical
	// requirement) -- Unrestricted is correct here for the same reason
	// SearchActiveSLAStatuses stamps it: this is an internal-caller-only
	// read (SLAStatusService.requireInternalCaller) with no caller-scoped
	// filtering of its own to fall back to instead.
	ctx = WithCallerIdentity(ctx, SearchScope{Unrestricted: true})

	var state domain.SLAClockState
	var pauseDurationSeconds *int64
	var businessPauseSeconds, durationSeconds, viewPercent float64
	var schedule string
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&state.IsActive, &state.Stage, &state.HasBreached, &state.StartedOn,
		&state.PausedOn, &pauseDurationSeconds,
		&businessPauseSeconds, &durationSeconds, &viewPercent, &schedule,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SLAClockState{}, nil
	}
	if err != nil {
		return domain.SLAClockState{}, fmt.Errorf("get sla clock state: %w", err)
	}
	if pauseDurationSeconds != nil {
		state.PauseDurationSeconds = *pauseDurationSeconds
	}
	state.Found = true
	state.IsPaused = state.Stage == "PAUSED"
	state.DurationSeconds = int64(durationSeconds)
	state.BusinessElapsedPercent = viewPercent
	state.Schedule = schedule
	state.BusinessPauseDurationSeconds = int64(businessPauseSeconds)

	if correctedStage, breached, pct, ok := businessDayCorrectedLiveState(schedule, state.Stage, state.StartedOn, businessPauseSeconds, durationSeconds); ok {
		// Stage AND HasBreached are now corrected together with the
		// percentage -- see businessDayCorrectedLiveState's own doc comment
		// for the confirmed, live-observed example this fixes (a clock
		// reporting BREACHED while its own corrected percentage read 58%).
		// csm-notification-service reads this exact response before sending
		// a breach alert, so an uncorrected Stage/HasBreached here could
		// previously trigger a false alert for a business-day clock that
		// was, in reality, nowhere near its deadline.
		state.Stage = correctedStage
		state.HasBreached = breached
		state.BusinessElapsedPercent = pct
	}
	return state, nil
}
