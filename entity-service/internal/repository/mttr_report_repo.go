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

// MTTRReportRepository reads the per-severity case counts and total
// resolution time behind the MTTR report.
type MTTRReportRepository interface {
	// SeverityStats returns, for each of S0/S1/S2/S3/S4 that has at least one
	// matching case, the number of cases whose LAST entry into Solution
	// Proposed falls within [windowStart, windowEnd), and the sum of each
	// such case's own resolution time (that entry's timestamp minus the
	// case's own created_on), in seconds. The last entry, not the first, is
	// used deliberately: a case can be proposed a solution, have it
	// rejected, and bounce back for more work before a later proposal
	// actually sticks -- the issue isn't resolved until that last one, so
	// using the first would understate how long the case really took.
	// creTeamIDs, when non-empty, restricts to cases whose account's CRE
	// team is one of them.
	SeverityStats(ctx context.Context, windowStart, windowEnd time.Time, creTeamIDs []string) ([]domain.MTTRSeverityStat, error)
}

type mttrReportRepository struct {
	db *Scoped
}

// NewMTTRReportRepository constructs the Postgres-backed MTTRReportRepository.
// It takes *Scoped, not the bare pool, so the read runs under the system
// identity the service sets.
func NewMTTRReportRepository(db *Scoped) MTTRReportRepository {
	return &mttrReportRepository{db: db}
}

// mttrSeverityStatsSQL finds each case's LAST entry into Solution Proposed
// (a case can leave and re-enter it -- a rejected proposal bounces it back
// for more work before a later one sticks -- so MAX(created_on) picks the
// entry that actually reflects when the case was resolved, not an earlier
// proposal that didn't hold), keeps only the cases that were created in the
// requested window AND whose last entry falls in it, and sums the resolution
// time per severity. A case opened before the window and resolved inside it is
// left out, so a month's figure is about the cases opened that month.
//
// The LOWER(new_value) IN (...) list matches both casings
// work_item_activity actually carries in practice (confirmed directly
// against prod: the raw enum label 'SOLUTION_PROPOSED' from one write path,
// the display label 'Solution Proposed' from another) -- the same
// case-insensitive match the case search's own stateEntered filter already
// uses. Served by the partial index idx_work_item_activity_state_entered
// (migration 0283): ON work_item_activity (LOWER(new_value), created_on)
// WHERE field_name = 'state'.
//
// Only a case that is resolved NOW is counted: its current state is
// Solution Proposed or Closed. A proposal the customer rejected sends the
// case back (e.g. to Work In Progress) while its earlier Solution Proposed
// entry stays in work_item_activity, so without this check such a case would
// be counted as resolved at that earlier time although it is not resolved.
// It is counted again once it is proposed again (or closed).
//
// Arguments: $1 window start (inclusive), $2 window end (exclusive), $3 the
// CRE team ids to restrict to -- an empty array means no restriction.
var mttrSeverityStatsSQL = `
WITH last_solution_proposed AS (
  SELECT sa.work_item_id, MAX(sa.created_on) AS entered_on
    FROM work_item_activity sa
   WHERE sa.field_name = 'state'
     AND LOWER(sa.new_value) IN ('solution_proposed', 'solution proposed')
   GROUP BY sa.work_item_id
)
SELECT c.severity::text,
       COUNT(*),
       COALESCE(SUM(EXTRACT(EPOCH FROM (fsp.entered_on - wi.created_on))), 0)
  FROM last_solution_proposed fsp
  JOIN work_item wi ON wi.id = fsp.work_item_id
  JOIN "case" c ON c.id = wi.id
  LEFT JOIN account a ON a.id = wi.account_id
 WHERE fsp.entered_on >= $1 AND fsp.entered_on < $2
   AND wi.created_on >= $1
   AND c.state IN ('SOLUTION_PROPOSED', 'CLOSED')
   AND c.severity IN ('S0', 'S1', 'S2', 'S3', 'S4')
   AND (cardinality($3::uuid[]) = 0 OR a.cre_team_id = ANY($3::uuid[]))
 GROUP BY c.severity`

func (r *mttrReportRepository) SeverityStats(ctx context.Context, windowStart, windowEnd time.Time, creTeamIDs []string) ([]domain.MTTRSeverityStat, error) {
	if creTeamIDs == nil {
		creTeamIDs = []string{}
	}
	rows, err := r.db.Query(ctx, mttrSeverityStatsSQL, windowStart, windowEnd, creTeamIDs)
	if err != nil {
		return nil, fmt.Errorf("mttr severity stats: %w", err)
	}
	defer rows.Close()

	out := make([]domain.MTTRSeverityStat, 0, 5)
	for rows.Next() {
		var stat domain.MTTRSeverityStat
		if err := rows.Scan(&stat.Severity, &stat.CaseCount, &stat.TotalResolutionSec); err != nil {
			return nil, fmt.Errorf("scan mttr severity stat: %w", err)
		}
		out = append(out, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mttr severity stats: %w", err)
	}
	return out, nil
}
