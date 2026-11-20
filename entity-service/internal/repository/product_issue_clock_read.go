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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// productIssueClockRank orders the sla rows of one work item: open stages first,
// then newest created_on, then id (the rule of piQueries.LatestClockRow).
const productIssueClockRank = `(s.stage::TEXT NOT IN ('ACHIEVED', 'CANCELLED', 'COMPLETED')) DESC, s.created_on DESC, s.id DESC`

// productIssueClockWhere picks the rows a clock read and the search sort consider:
// a started, not cancelled resolution clock of a product issue policy. Both reads
// use it with productIssueClockRank so they choose the same row.
const productIssueClockWhere = `sp.applies_to = 'PRODUCT_ISSUE' AND sp.target = 'RESOLUTION'
		   AND s.start_on IS NOT NULL AND s.stage::TEXT <> 'CANCELLED'`

// attachProductIssueClocks loads the newest RESOLUTION clock row of every summary in
// one query and sets Clock. Summaries with no clock keep Clock nil.
func attachProductIssueClocks(ctx context.Context, db piDB, items []domain.ProductIssueSummary) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	rows, err := db.Query(ctx, `
		SELECT DISTINCT ON (s.work_item_id) s.work_item_id::TEXT, sp.name, COALESCE(s.schedule, sp.schedule, ''), s.stage::TEXT,
		       s.start_on, s.paused_on, s.end_on,
		       COALESCE(EXTRACT(EPOCH FROM COALESCE(sp.duration, s.duration)), 0)::float8,
		       COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0)::float8,
		       COALESCE(s.has_breached, FALSE), COALESCE(s.business_elapsed_percentage, 0)::float8
		  FROM sla s
		  JOIN sla_policy sp ON sp.id = s.sla_policy_id
		 WHERE s.work_item_id = ANY($1::text[]::uuid[]) AND `+productIssueClockWhere+`
		 ORDER BY s.work_item_id, `+productIssueClockRank, ids)
	if err != nil {
		return fmt.Errorf("load product issue clocks: %w", err)
	}
	defer rows.Close()
	byID := map[string]*domain.ProductIssueClock{}
	for rows.Next() {
		var id string
		var c domain.ProductIssueClock
		var stage *string
		if err := rows.Scan(&id, &c.PolicyName, &c.Schedule, &stage, &c.StartOn, &c.PausedOn, &c.EndOn,
			&c.BudgetSeconds, &c.BusinessPauseSeconds, &c.HasBreached, &c.StoredPercentage); err != nil {
			return fmt.Errorf("scan product issue clock: %w", err)
		}
		if stage != nil {
			c.Stage = *stage
		}
		byID[id] = &c
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		items[i].Clock = byID[items[i].ID]
	}
	return nil
}
