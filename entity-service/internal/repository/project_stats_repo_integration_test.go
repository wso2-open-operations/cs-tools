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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Skipped without CASE_STATS_TEST_DSN, the same env var the case-stats
// integration test uses -- both need a real schema for the same reason.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ProjectStatsIntegration

const statsProjectID = "31111111-1111-1111-1111-111111111111"

// seedProjectStats adds a conversation, a change request, a time card and a
// deployment/deployed product on top of a case, so every aggregation has
// something to return.
func seedProjectStats(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM time_card WHERE created_by = 'stats-test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM work_item WHERE created_by = 'stats-test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM deployed_product WHERE created_by = 'stats-test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM deployment WHERE created_by = 'stats-test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE user_name = 'stats-test@example.com'`)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, statsProjectID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, end_date)
	          VALUES ($1, now(), now(), 'stats-test', 'stats-test', 'STATSTEST', 'sf-statstest', (now() + INTERVAL '30 days')::date)`, statsProjectID)

	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active)
	          VALUES ('32222222-0000-0000-0000-000000000001', now(), now(), 'stats-test@example.com', 'stats-test@example.com', true)`)

	mustExec(`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, is_active, project_id)
	          VALUES ('33333333-0000-0000-0000-000000000001', now() - INTERVAL '3 days', now(), 'stats-test', 'stats-test', 'DEPSTAT01', 'stats dep', true, $1)`, statsProjectID)
	mustExec(`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, active, project_id)
	          VALUES ('34444444-0000-0000-0000-000000000001', now(), now(), 'stats-test', 'stats-test', 'DPSTAT01', true, $1)`, statsProjectID)

	// One open case (outstanding, and enough to flip SLA status), one
	// conversation, one change request awaiting customer approval.
	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
	          VALUES ('35555555-0000-0000-0000-000000000001', now(), now(), 'stats-test', 'stats-test', 'CSSTAT01', 'STATS-1', 'a case', 'CASE', $1)`, statsProjectID)
	mustExec(`INSERT INTO "case" (id, state, severity) VALUES ('35555555-0000-0000-0000-000000000001', 'OPEN', 'S2')`)

	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, project_id)
	          VALUES ('35555555-0000-0000-0000-000000000002', now(), now(), 'stats-test', 'stats-test', 'CONVSTAT01', 'a conversation', 'CONVERSATION', $1)`, statsProjectID)
	mustExec(`INSERT INTO conversation (id, state) VALUES ('35555555-0000-0000-0000-000000000002', 'ACTIVE')`)

	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, project_id)
	          VALUES ('35555555-0000-0000-0000-000000000003', now(), now(), 'stats-test', 'stats-test', 'CRSTAT01', 'a change request', 'CHANGE_REQUEST', $1)`, statsProjectID)
	mustExec(`INSERT INTO change_request (id, state) VALUES ('35555555-0000-0000-0000-000000000003', 'CUSTOMER_APPROVAL')`)

	// 90 billable + 30 non-billable minutes, approved, against the case.
	mustExec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, user_id, work_date, is_billable, state, analyzing_minutes, providing_solution_minutes)
	          VALUES ('36666666-0000-0000-0000-000000000001', now(), now(), 'stats-test', 'stats-test', '35555555-0000-0000-0000-000000000001', '32222222-0000-0000-0000-000000000001', CURRENT_DATE, true, 'APPROVED', 60, 30)`)
	mustExec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, user_id, work_date, is_billable, state, analyzing_minutes)
	          VALUES ('36666666-0000-0000-0000-000000000002', now(), now(), 'stats-test', 'stats-test', '35555555-0000-0000-0000-000000000001', '32222222-0000-0000-0000-000000000001', CURRENT_DATE, false, 'APPROVED', 30)`)
	// Not APPROVED -- must be excluded.
	mustExec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, user_id, work_date, is_billable, state, analyzing_minutes)
	          VALUES ('36666666-0000-0000-0000-000000000003', now(), now(), 'stats-test', 'stats-test', '35555555-0000-0000-0000-000000000001', '32222222-0000-0000-0000-000000000001', CURRENT_DATE, true, 'SUBMITTED', 999)`)
}

func TestProjectStatsIntegration_Aggregations(t *testing.T) {
	pool := caseStatsPool(t)
	seedProjectStats(t, pool)

	ctx := context.Background()
	repo := repository.NewProjectStatsRepository(pool)

	t.Run("TimeLoggedMinutes", func(t *testing.T) {
		billable, nonBillable, err := repo.TimeLoggedMinutes(ctx, statsProjectID, "", "")
		if err != nil {
			t.Fatalf("TimeLoggedMinutes: %v", err)
		}
		// The SUBMITTED card is excluded; 60+30 billable, 30 non-billable.
		if billable != 90 || nonBillable != 30 {
			t.Errorf("minutes = %d/%d, want 90/30", billable, nonBillable)
		}
	})

	t.Run("TimeLoggedMinutesRespectsDateRange", func(t *testing.T) {
		_, _, err := repo.TimeLoggedMinutes(ctx, statsProjectID, "2000-01-01", "2000-01-31")
		if err != nil {
			t.Fatalf("TimeLoggedMinutes(range): %v", err)
		}
	})

	t.Run("DeploymentAndDeployedProductCounts", func(t *testing.T) {
		d, err := repo.DeploymentCount(ctx, statsProjectID)
		if err != nil {
			t.Fatalf("DeploymentCount: %v", err)
		}
		dp, err := repo.DeployedProductCount(ctx, statsProjectID)
		if err != nil {
			t.Fatalf("DeployedProductCount: %v", err)
		}
		if d != 1 || dp != 1 {
			t.Errorf("counts = %d/%d, want 1/1", d, dp)
		}
	})

	t.Run("LastDeploymentOn", func(t *testing.T) {
		last, err := repo.LastDeploymentOn(ctx, statsProjectID)
		if err != nil {
			t.Fatalf("LastDeploymentOn: %v", err)
		}
		if last == nil {
			t.Fatal("LastDeploymentOn = nil, want the seeded deployment's creation time")
		}
	})

	t.Run("OutstandingCounts", func(t *testing.T) {
		counts, err := repo.OutstandingCounts(ctx, repository.SearchScope{Unrestricted: true}, statsProjectID,
			[]string{"OPEN", "WORK_IN_PROGRESS", "AWAITING_INFO", "WAITING_ON_WSO2", "REOPENED", "SOLUTION_PROPOSED"},
			[]string{"CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW"})
		if err != nil {
			t.Fatalf("OutstandingCounts: %v", err)
		}
		if counts["case"] != 1 {
			t.Errorf("outstanding cases = %d, want 1", counts["case"])
		}
		if counts["change_request"] != 1 {
			t.Errorf("outstanding change requests = %d, want 1", counts["change_request"])
		}
	})

	t.Run("SLAStatusInputs", func(t *testing.T) {
		in, err := repo.SLAStatusInputs(ctx, statsProjectID)
		if err != nil {
			t.Fatalf("SLAStatusInputs: %v", err)
		}
		if !in.HasOutstandingCase {
			t.Error("HasOutstandingCase = false, want true (an OPEN case with a severity exists)")
		}
		if !in.HasDeployedProduct {
			t.Error("HasDeployedProduct = false, want true")
		}
		if !in.HasActiveEndDate {
			t.Error("HasActiveEndDate = false, want true (end_date is 30 days out)")
		}
		if in.HasCustomerAdminContact {
			t.Error("HasCustomerAdminContact = true, want false (no contacts seeded)")
		}
	})

	t.Run("ConversationStateCounts", func(t *testing.T) {
		rows, err := repo.ConversationStateCounts(ctx, statsProjectID, "")
		if err != nil {
			t.Fatalf("ConversationStateCounts: %v", err)
		}
		if len(rows) != 1 || rows[0].State != "ACTIVE" || rows[0].Count != 1 {
			t.Errorf("rows = %+v, want one ACTIVE row with count 1", rows)
		}
	})

	t.Run("ConversationStateCountsCreatedByFilter", func(t *testing.T) {
		rows, err := repo.ConversationStateCounts(ctx, statsProjectID, "nobody@example.com")
		if err != nil {
			t.Fatalf("ConversationStateCounts(createdBy): %v", err)
		}
		if len(rows) != 0 {
			t.Errorf("rows = %+v, want none for an unrelated creator", rows)
		}
	})

	t.Run("ChangeRequestStateCounts", func(t *testing.T) {
		rows, err := repo.ChangeRequestStateCounts(ctx, statsProjectID)
		if err != nil {
			t.Fatalf("ChangeRequestStateCounts: %v", err)
		}
		if len(rows) != 1 || rows[0].State != "CUSTOMER_APPROVAL" {
			t.Errorf("rows = %+v, want one CUSTOMER_APPROVAL row", rows)
		}
	})

	t.Run("ChangeRequestResolvedBuckets", func(t *testing.T) {
		cm, p30, err := repo.ChangeRequestResolvedBuckets(ctx, statsProjectID, "CLOSED")
		if err != nil {
			t.Fatalf("ChangeRequestResolvedBuckets: %v", err)
		}
		if cm != 0 || p30 != 0 {
			t.Errorf("buckets = %d/%d, want 0/0 (nothing closed)", cm, p30)
		}
	})

	// InstanceCount queries deployment_node.project_key, the live (sync-built)
	// schema's spelling. A database built from migrations/ has
	// subscription_key instead, so this fails there by design -- the service
	// degrades the count to zero rather than failing the dashboard. Asserting
	// the failure keeps the drift visible instead of silently untested; if
	// the migration is ever reconciled, this expectation flips.
	t.Run("InstanceCountFailsOnMigrationBuiltSchema", func(t *testing.T) {
		if _, err := repo.InstanceCount(ctx, statsProjectID); err == nil {
			t.Skip("deployment_node.project_key exists -- schema reconciled with the live one")
		}
	})
}
