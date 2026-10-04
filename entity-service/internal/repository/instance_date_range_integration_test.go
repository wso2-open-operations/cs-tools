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

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The usage date range includes both end days in full and nothing either
// side of them. Skipped without CASE_STATS_TEST_DSN.
func TestInstanceUsage_DateRangeIncludesWholeEndDays(t *testing.T) {
	pool := caseStatsPool(t)
	seedRLSScopedRemaining(t, pool)
	const node = "7a000000-0000-4000-8000-0000000000d9"
	ctx := context.Background()
	cleanup := func() { _, _ = pool.Exec(ctx, `DELETE FROM deployment_node WHERE id = $1`, node) }
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `INSERT INTO deployment_node (id, created_on, updated_on, created_by, updated_by, node_id, project_key)
		VALUES ($1, now(), now(), 't', 't', 'dr-node', 'RSSCOPED-A')`, node); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	// Day boundaries in the session time zone, which is what the filter uses.
	for _, ts := range []string{
		"2026-03-09 23:59:59", // the day before: out
		"2026-03-10 00:00:00", // start day, first instant: in
		"2026-03-11 23:59:59", // end day, last second: in
		"2026-03-12 00:00:00", // the day after: out
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO hourly_usage_summary (id, created_on, updated_on, created_by, updated_by, deployment_node_id, count_type, count, counted_on)
			VALUES (gen_random_uuid(), now(), now(), 't', 't', $1, 'CORES', 1, $2::timestamp::timestamptz)`, node, ts); err != nil {
			t.Fatalf("seed usage %s: %v", ts, err)
		}
	}

	repo := repository.NewInstanceRepository(repository.NewScoped(pool))
	got, _, err := repo.SearchInstanceUsage(rsInternal(), domain.InstanceDateRangeFilters{
		StartDate: "2026-03-10", EndDate: "2026-03-11", ProjectIDs: []string{rsProjectA},
	})
	if err != nil {
		t.Fatalf("SearchInstanceUsage: %v", err)
	}
	total := 0
	for _, e := range got {
		if e.InstanceID == node {
			for _, p := range e.PeriodSummaries {
				for _, c := range p.Counts {
					total += c
				}
			}
		}
	}
	if total != 2 {
		t.Errorf("usage counted in range = %d, want 2 (the two in-range rows only); entries: %+v", total, got)
	}
}
