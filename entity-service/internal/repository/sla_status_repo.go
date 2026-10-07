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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// SLAStatusRepository defines the read operation backing GET /sla-status —
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
	// ServiceNow-synced row set for nothing. Empty means no filter, the
	// original, unscoped behavior.
	SearchActiveSLAStatuses(ctx context.Context, pagination domain.Pagination, sourceFilter string) ([]domain.SLAStatus, int, error)
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
// sourceArgIndex, when > 0, adds "AND s.source = $<sourceArgIndex>" -- the
// placeholder's position differs between the count query (which otherwise
// binds nothing) and the data query (which already binds limit/offset/the
// evaluation-subscription project type name), so the caller passes whichever
// index is next free in its own query rather than this function assuming one.
// 0 means no source filter at all, the original unscoped behavior.
func activeSLAStatusCTE(sourceArgIndex int) string {
	sourceFilter := ""
	if sourceArgIndex > 0 {
		sourceFilter = fmt.Sprintf(" AND s.source = $%d::sla_source_enum", sourceArgIndex)
	}
	return `
	WITH active_sla AS (
		SELECT DISTINCT ON (s.work_item_id, sp.target)
			s.work_item_id, sp.target, s.live_elapsed_percentage AS business_elapsed_percentage,
			s.live_has_breached AS has_breached, s.live_stage AS stage, s.start_on
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

func scanSLAStatus(row interface{ Scan(...any) error }) (domain.SLAStatus, error) {
	var s domain.SLAStatus
	var target, severity, caseType string
	var caseNumber, wso2CaseID, caseTitle, productName, state, teamName, onboardingStatus *string
	var teamEmail, teamLeadName, assigneeName, assigneeEmail *string
	var stage string
	var isEvaluation bool
	err := row.Scan(
		&s.CaseID, &target, &s.BusinessElapsedPercent, &s.HasBreached, &stage, &s.StartedOn,
		&caseNumber, &wso2CaseID, &caseTitle, &caseType,
		&productName, &severity, &state, &teamName, &onboardingStatus, &isEvaluation,
		&teamEmail, &teamLeadName, &assigneeName, &assigneeEmail,
	)
	if err != nil {
		return domain.SLAStatus{}, err
	}
	s.ClockType = strings.ToLower(target)
	s.IsPaused = stage == "PAUSED"
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
