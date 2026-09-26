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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// SLAStatusRepository defines the read operation backing GET /sla-status —
// see domain.SLAStatus's own doc comment for what it replaced and why.
type SLAStatusRepository interface {
	// SearchActiveSLAStatuses returns every currently-active (sla.is_active =
	// true) clock across every case-like work item, one row per
	// (work_item, sla_policy.target), paginated.
	SearchActiveSLAStatuses(ctx context.Context, pagination domain.Pagination) ([]domain.SLAStatus, int, error)
}

type slaStatusRepo struct {
	db *pgxpool.Pool
}

// NewSLAStatusRepository constructs an SLAStatusRepository backed by the
// given connection pool.
func NewSLAStatusRepository(db *pgxpool.Pool) SLAStatusRepository {
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
const activeSLAStatusCTE = `
	WITH active_sla AS (
		SELECT DISTINCT ON (s.work_item_id, sp.target)
			s.work_item_id, sp.target, s.business_elapsed_percentage,
			s.has_breached, s.stage, s.start_on
		FROM sla s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.is_active AND sp.target IS NOT NULL
		ORDER BY s.work_item_id, sp.target, s.start_on DESC NULLS LAST, s.updated_on DESC
	)`

// activeSLAStatusFromJoins resolves each row's case-like display data --
// mirrors caseRepo.GetCaseByID's own product/severity joins exactly (see
// that query's own comments for why), restricted to case-like types only.
const activeSLAStatusFromJoins = `
	FROM active_sla als
	JOIN work_item wi ON wi.id = als.work_item_id
	LEFT JOIN "case" c ON c.id = wi.id
	` + caseLikeJoins + `
	LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
	LEFT JOIN product prod ON prod.id = dp.product_id
	LEFT JOIN product_version pv ON pv.id = dp.version_id
	WHERE wi.type = ANY(` + caseLikeWorkItemTypes + `)`

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
	var caseNumber, wso2CaseID, caseTitle, productName, state *string
	var stage string
	err := row.Scan(
		&s.CaseID, &target, &s.BusinessElapsedPercent, &s.HasBreached, &stage, &s.StartedOn,
		&caseNumber, &wso2CaseID, &caseTitle, &caseType,
		&productName, &severity, &state,
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
	if sev, ok := caseSeverityFromEnum[severity]; ok {
		s.Priority = strings.ToUpper(string(sev))
	}
	return s, nil
}

// SearchActiveSLAStatuses implements SLAStatusRepository.
func (r *slaStatusRepo) SearchActiveSLAStatuses(ctx context.Context, pagination domain.Pagination) ([]domain.SLAStatus, int, error) {
	countQuery := activeSLAStatusCTE + `
		SELECT COUNT(*)
		` + activeSLAStatusFromJoins

	dataQuery := activeSLAStatusCTE + `
		SELECT als.work_item_id::TEXT, als.target::TEXT, COALESCE(als.business_elapsed_percentage, 0), COALESCE(als.has_breached, FALSE), COALESCE(als.stage::TEXT, ''), als.start_on,
		       wi.number, wi.wso2_id, wi.subject, wi.type::TEXT,
		       prod.name || COALESCE(' ' || pv.version, ''), COALESCE(c.severity::TEXT, ''),
		       ` + caseLikeStateColumn + `
		` + activeSLAStatusFromJoins + `
		ORDER BY als.work_item_id, als.target
		LIMIT $1 OFFSET $2`

	var total int
	var statuses []domain.SLAStatus

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery).Scan(&total); err != nil {
			return fmt.Errorf("count active sla statuses: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, pagination.Limit, pagination.Offset)
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
