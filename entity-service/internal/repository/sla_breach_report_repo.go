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
	"time"

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

// slaBreachReportQuery lists every breached clock in the window.
//
// It reads `sla` directly rather than `sla_live`, and deliberately does
// NOT reuse activeSLAStatusCTE: that CTE filters `s.is_active`, which a
// clock on a closed or completed case no longer is -- exactly the rows a
// retrospective "what breached last week" report exists to show. The live
// view's business-day correction is irrelevant here for the same reason:
// this report asks when a clock breached (a recorded instant,
// original_breached_on), never how far along a running clock is.
//
// The join set mirrors activeSLAStatusFromJoins' own account/product/
// project joins, minus the ones only live status needs. caseLikeJoins is
// deliberately not pulled in: its only unique contribution here would be
// caseLikeStateColumn, which this report doesn't show, and it LEFT JOINs
// the RLS-protected `announcement` table for no benefit.
//
// `cre` is the account's CRE group -- ServiceNow's
// account.u_integration_cs_team, the field the source report grouped on.
const slaBreachReportQuery = `
	SELECT
		COALESCE(cre.name, ''),
		COALESCE(wi.number, ''),
		COALESCE(wi.wso2_id, ''),
		wi.id::TEXT,
		COALESCE(wi.subject, ''),
		COALESCE(a.name, ''),
		COALESCE(c.severity::TEXT, ''),
		COALESCE(prod.name || COALESCE(' ' || pv.version, ''), ''),
		sp.name,
		s.original_breached_on,
		COALESCE(s.stage::TEXT, ''),
		COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), ''), ''),
		COALESCE(p.onboarding_status::TEXT, '')
	FROM sla s
	JOIN sla_policy sp ON sp.id = s.sla_policy_id
	JOIN work_item wi ON wi.id = s.work_item_id
	LEFT JOIN "case" c ON c.id = wi.id
	LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
	LEFT JOIN product prod ON prod.id = dp.product_id
	LEFT JOIN product_version pv ON pv.id = dp.version_id
	LEFT JOIN account a ON a.id = wi.account_id
	LEFT JOIN "group" cre ON cre.id = a.cre_team_id
	LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
	LEFT JOIN project p ON p.id = wi.project_id
	WHERE wi.type = ANY(` + caseLikeWorkItemTypes + `)
	  AND s.has_breached IS TRUE
	  AND s.original_breached_on >= $1
	  AND s.original_breached_on <  $2
	  AND sp.name = ANY($3)
	  AND ($4::TEXT[] IS NULL OR cardinality($4::TEXT[]) = 0 OR s.stage::TEXT = ANY($4))
	  AND ($5::TEXT[] IS NULL OR cardinality($5::TEXT[]) = 0 OR p.onboarding_status::TEXT = ANY($5))
	  -- "is not one of" keeps a case with no project, and so no status:
	  -- ServiceNow's own operator does not discard an empty value, and
	  -- dropping those rows here would silently shrink the report.
	  AND ($6::TEXT[] IS NULL OR cardinality($6::TEXT[]) = 0
	       OR p.onboarding_status IS NULL OR NOT (p.onboarding_status::TEXT = ANY($6)))
	ORDER BY COALESCE(cre.name, ''), sp.name, s.original_breached_on`

// ListBreaches runs slaBreachReportQuery over [from, to).
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
		var row domain.SLABreachRow
		if err := rows.Scan(
			&row.Team, &row.CaseNumber, &row.WSO2CaseID, &row.CaseID, &row.Subject,
			&row.Account, &row.Severity, &row.Product, &row.Definition,
			&row.BreachedOn, &row.Stage, &row.AssigneeName, &row.OnboardingStatus,
		); err != nil {
			return nil, fmt.Errorf("scan sla breach row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sla breaches: %w", err)
	}
	return out, nil
}
