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

// Instance reads on a live database: a project member only ever gets their
// own projects' nodes, whatever the request filters by, and all five instance
// queries run with the scope predicate. Skipped without CASE_STATS_TEST_DSN.

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	isNodeA = "7a000000-0000-4000-8000-0000000000d1"
	isNodeB = "7a000000-0000-4000-8000-0000000000d2"
)

func TestInstanceScope_MemberSeesOnlyOwnProjects(t *testing.T) {
	pool := caseStatsPool(t)
	seedRLSScopedRemaining(t, pool)
	ctx := context.Background()
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM deployment_node WHERE id IN ($1, $2)`, isNodeA, isNodeB)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `INSERT INTO deployment_node (id, created_on, updated_on, created_by, updated_by, node_id, project_key) VALUES
		($1, now(), now(), 't', 't', 'is-node-a', 'RSSCOPED-A'),
		($2, now(), now(), 't', 't', 'is-node-b', 'RSSCOPED-B')`, isNodeA, isNodeB); err != nil {
		t.Fatalf("seed nodes: %v", err)
	}

	repo := repository.NewInstanceRepository(repository.NewScoped(pool))
	nodes := func(t *testing.T, ctx context.Context, f *domain.InstanceSearchFilters) map[string]bool {
		t.Helper()
		got, _, err := repo.SearchInstances(ctx, domain.SearchInstancesRequest{Filters: f, Pagination: domain.Pagination{Limit: 500}})
		if err != nil {
			t.Fatalf("SearchInstances: %v", err)
		}
		out := map[string]bool{}
		for _, in := range got {
			if in.ID == isNodeA || in.ID == isNodeB {
				out[in.ID] = true
			}
		}
		return out
	}
	member, _ := rsCustomer(rsMemberA, rsProjectA)

	if got := nodes(t, rsInternal(), nil); !got[isNodeA] || !got[isNodeB] {
		t.Errorf("internal, no filter: %v, want both nodes", got)
	}
	if got := nodes(t, member, nil); !got[isNodeA] || got[isNodeB] {
		t.Errorf("member, no filter: %v, want only project A's node", got)
	}
	if got := nodes(t, member, &domain.InstanceSearchFilters{ProjectIDs: []string{rsProjectB}}); len(got) != 0 {
		t.Errorf("member asking for project B: %v, want nothing", got)
	}

	// The four date-range queries must run with the scope predicate in place.
	today := time.Now().UTC().Format("2006-01-02")
	f := domain.InstanceDateRangeFilters{StartDate: today, EndDate: today}
	for name, c := range map[string]context.Context{"internal": rsInternal(), "member": member} {
		if _, _, err := repo.SearchInstanceMetrics(c, f); err != nil {
			t.Errorf("%s SearchInstanceMetrics: %v", name, err)
		}
		if _, _, err := repo.SearchInstanceUsage(c, f); err != nil {
			t.Errorf("%s SearchInstanceUsage: %v", name, err)
		}
		if _, err := repo.SearchInstanceMetricsStats(c, f); err != nil {
			t.Errorf("%s SearchInstanceMetricsStats: %v", name, err)
		}
		if _, err := repo.SearchInstanceUsageStats(c, f, nil); err != nil {
			t.Errorf("%s SearchInstanceUsageStats: %v", name, err)
		}
	}
}
