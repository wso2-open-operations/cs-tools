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

// Runs the real SQL of SearchEngagementAllocations against a migrated
// database; skipped unless ENTITY_TEST_DATABASE_URL is set (see
// schedule_repo_integration_test.go for how to build one).

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	seaAccount = "5ea10000-0000-4000-8000-000000000001"
	seaUserA   = "5ea10000-0000-4000-8000-000000000011"
	seaUserB   = "5ea10000-0000-4000-8000-000000000012"
	seaEngA    = "5ea10000-0000-4000-8000-000000000021"
	seaEngB    = "5ea10000-0000-4000-8000-000000000022"
)

func newEngagementAllocationIntegrationRepo(t *testing.T) (ScheduleRepository, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	clean := func() {
		mustExec(t, pool, `DELETE FROM customer_engagement WHERE id IN ($1, $2)`, seaEngA, seaEngB)
		mustExec(t, pool, `DELETE FROM account WHERE id = $1`, seaAccount)
		// Leave and shifts reference the user ON DELETE RESTRICT: they go first.
		mustExec(t, pool, `DELETE FROM team_schedule_absence WHERE user_id IN ($1, $2)`, seaUserA, seaUserB)
		mustExec(t, pool, `DELETE FROM team_schedule_assignment WHERE user_id IN ($1, $2)`, seaUserA, seaUserB)
		mustExec(t, pool, `DELETE FROM "user" WHERE id IN ($1, $2)`, seaUserA, seaUserB)
	}
	clean()
	t.Cleanup(clean)

	mustExec(t, pool, `INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number)
		VALUES ($1, now(), now(), 't', 't', 'Example Corp', 'ACC-SEA-IT-1')`, seaAccount)
	for _, u := range []struct{ id, email string }{{seaUserA, "jane.doe@example.com"}, {seaUserB, "john.roe@example.com"}} {
		mustExec(t, pool, `INSERT INTO "user" (id, created_on, updated_on, user_name, email)
			VALUES ($1, now(), now(), $2, $2)`, u.id, u.email)
	}
	// A is on an account with a type and a delivery mode; B has neither.
	mustExec(t, pool, `INSERT INTO customer_engagement (id, created_on, updated_on, name, engagement_id, state, account_id, engagement_type, delivery_mode)
		VALUES ($1, now(), now(), 'Rollout', 'EITSEA1', 'IN_PROGRESS', $2, 'CONSULTANCY', 'ONSITE')`, seaEngA, seaAccount)
	mustExec(t, pool, `INSERT INTO customer_engagement (id, created_on, updated_on, name, state)
		VALUES ($1, now(), now(), 'No account', 'IN_PROGRESS')`, seaEngB)

	alloc := func(allocID, eng, user, state, start, end string, util any) {
		mustExec(t, pool, `INSERT INTO customer_engagement_allocation_resource
			(id, created_on, updated_on, allocation_id, engagement_id, resource_id, state, start_date, end_date, utilization)
			VALUES (gen_random_uuid(), now(), now(), $1, $2, $3, $4::engagement_allocation_state_enum, $5::date, $6::date, $7)`,
			allocID, eng, user, state, start, end, util)
	}
	// In window, full detail.
	alloc("EALLOC-1", seaEngA, seaUserA, "CONFIRMED", "2026-09-21", "2026-09-25", 50)
	// Starts before and ends on the window's first day: overlaps (inclusive).
	alloc("EALLOC-2", seaEngA, seaUserA, "CONFIRMED", "2026-09-10", "2026-09-21", 25.5)
	// Ends the day before the window: out.
	alloc("EALLOC-3", seaEngA, seaUserA, "CONFIRMED", "2026-09-10", "2026-09-20", 100)
	// Starts the day after the window: out.
	alloc("EALLOC-4", seaEngA, seaUserA, "CONFIRMED", "2026-09-28", "2026-09-30", 100)
	// In window but not confirmed: out.
	alloc("EALLOC-5", seaEngA, seaUserA, "TENTATIVE", "2026-09-22", "2026-09-23", 100)
	alloc("EALLOC-6", seaEngA, seaUserA, "ALLOCATION_CANCELLED", "2026-09-22", "2026-09-23", 100)
	// Another person, engagement with no account, type, mode or utilization.
	alloc("EALLOC-7", seaEngB, seaUserB, "CONFIRMED", "2026-09-23", "2026-09-24", nil)
	return NewScheduleRepository(pool), pool
}

func TestScheduleIntegration_EngagementAllocationsConfirmedAndOverlapping(t *testing.T) {
	repo, _ := newEngagementAllocationIntegrationRepo(t)

	got, err := repo.SearchEngagementAllocations(context.Background(), domain.SearchScheduleEngagementAllocationsRequest{
		UserIDs: []string{seaUserA, seaUserB}, From: "2026-09-21", To: "2026-09-27"})
	if err != nil {
		t.Fatalf("SearchEngagementAllocations: %v", err)
	}
	ids := []string{}
	for _, a := range got {
		ids = append(ids, seaStr(a.AllocationID))
	}
	want := []string{"EALLOC-2", "EALLOC-1", "EALLOC-7"} // by person, then start date
	if len(ids) != len(want) {
		t.Fatalf("got allocations %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("got allocations %v, want %v", ids, want)
		}
	}

	full := got[1]
	if full.UserID != seaUserA || full.EngagementID != seaEngA {
		t.Fatalf("wrong ids: %+v", full)
	}
	if seaStr(full.EngagementDisplayID) != "EITSEA1" || seaStr(full.AccountName) != "Example Corp" ||
		seaStr(full.EngagementType) != "CONSULTANCY" || seaStr(full.ModeOfDelivery) != "ONSITE" {
		t.Fatalf("engagement/account fields wrong: %+v", full)
	}
	if full.Utilization == nil || *full.Utilization != 50 {
		t.Fatalf("utilization = %v, want 50", full.Utilization)
	}
	if full.StartDate != "2026-09-21" || full.EndDate != "2026-09-25" {
		t.Fatalf("dates = %s..%s", full.StartDate, full.EndDate)
	}
	if got[0].Utilization == nil || *got[0].Utilization != 25.5 {
		t.Fatalf("fractional utilization = %v, want 25.5", got[0].Utilization)
	}

	bare := got[2]
	if bare.AccountName != nil || bare.EngagementDisplayID != nil || bare.EngagementType != nil ||
		bare.ModeOfDelivery != nil || bare.Utilization != nil {
		t.Fatalf("missing data must be absent, not invented: %+v", bare)
	}
}

func TestScheduleIntegration_EngagementAllocationsOnlyTheRequestedPeople(t *testing.T) {
	repo, _ := newEngagementAllocationIntegrationRepo(t)

	got, err := repo.SearchEngagementAllocations(context.Background(), domain.SearchScheduleEngagementAllocationsRequest{
		UserIDs: []string{seaUserB}, From: "2026-09-21", To: "2026-09-27"})
	if err != nil {
		t.Fatalf("SearchEngagementAllocations: %v", err)
	}
	if len(got) != 1 || got[0].UserID != seaUserB {
		t.Fatalf("got %+v, want only user B's allocation", got)
	}
}

func seaStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
