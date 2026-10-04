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
	"sync"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Concurrent registrations and revisions for one work item must leave exactly
// one open clock per target. Skipped without CASE_STATS_TEST_DSN.
func TestSLAEngineIntegration_ConcurrentRegisterLeavesOneOpenClock(t *testing.T) {
	pool := caseStatsPool(t)
	seedSLAEngineWorkItem(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	repo := repository.NewSLAEngineRepository(scoped)

	resp, err := repo.FindPolicyByName(ctx, "P0 - Response (Managed Services)", "RESPONSE")
	if err != nil {
		t.Fatalf("FindPolicyByName(response): %v", err)
	}

	for round := 0; round < 5; round++ {
		if _, err := scoped.Exec(ctx, `DELETE FROM sla WHERE work_item_id = $1::uuid`, slaEngineIntegrationWorkItemID); err != nil {
			t.Fatalf("reset: %v", err)
		}
		var wg sync.WaitGroup
		errs := make(chan error, 24)
		for i := 0; i < 24; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				var err error
				if i%4 == 0 {
					_, err = repo.ReviseClocks(ctx, slaEngineIntegrationWorkItemID, []repository.SLAPolicyRef{resp})
				} else {
					_, err = repo.RegisterClock(ctx, slaEngineIntegrationWorkItemID, resp)
				}
				errs <- err
			}(i)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
		}
		var open int
		if err := scoped.QueryRow(ctx, `SELECT COUNT(*) FROM sla s JOIN sla_policy sp ON sp.id = s.sla_policy_id
			WHERE s.work_item_id = $1::uuid AND s.source = 'CSM' AND sp.target::TEXT = 'RESPONSE'
			  AND s.stage::TEXT IN ('IN_PROGRESS', 'PAUSED')`, slaEngineIntegrationWorkItemID).Scan(&open); err != nil {
			t.Fatalf("count: %v", err)
		}
		if open != 1 {
			t.Fatalf("round %d: %d open RESPONSE clocks, want exactly 1", round, open)
		}
	}
}
