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

// UpdateCaseFields stamps work_item.eta_shared_on server-side when a fix ETA
// date is written or changed, and leaves it alone when the same date is
// re-sent. Runs against a real Postgres. Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run EtaSharedOnIntegration

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestEtaSharedOnIntegration_StampedOnlyWhenADateChanges(t *testing.T) {
	pool := caseStatsPool(t)
	seedClosedByFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	// Fixture starts with no stamp (nil), no actor id needed for ETA writes.
	if got, err := repo.GetCaseEtaSharedOn(ctx, cbCaseID); err != nil || got != nil {
		t.Fatalf("before any ETA: got %v, err %v, want nil, nil", got, err)
	}

	eta := "2026-10-20"
	if _, err := repo.UpdateCaseFields(ctx, domain.UpdateCaseRequest{ID: cbCaseID, BestCaseFixEta: &eta}, "", "integration-client"); err != nil {
		t.Fatalf("UpdateCaseFields(first): %v", err)
	}
	first, err := repo.GetCaseEtaSharedOn(ctx, cbCaseID)
	if err != nil || first == nil {
		t.Fatalf("after first ETA: got %v, err %v, want a timestamp", first, err)
	}

	if _, err := repo.UpdateCaseFields(ctx, domain.UpdateCaseRequest{ID: cbCaseID, BestCaseFixEta: &eta}, "", "integration-client"); err != nil {
		t.Fatalf("UpdateCaseFields(same): %v", err)
	}
	same, _ := repo.GetCaseEtaSharedOn(ctx, cbCaseID)
	if same == nil || !same.Equal(*first) {
		t.Fatalf("same date re-sent: stamp = %v, want unchanged %v", same, first)
	}

	changed := "2026-10-27"
	if _, err := repo.UpdateCaseFields(ctx, domain.UpdateCaseRequest{ID: cbCaseID, BestCaseFixEta: &changed}, "", "integration-client"); err != nil {
		t.Fatalf("UpdateCaseFields(changed): %v", err)
	}
	after, _ := repo.GetCaseEtaSharedOn(ctx, cbCaseID)
	if after == nil || !after.After(*first) {
		t.Fatalf("changed date: stamp = %v, want after %v", after, first)
	}
}
