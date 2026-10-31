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

// Runs the real SQL of the engagement allocation view against a migrated
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
	eavAccount = "5eb10000-0000-4000-8000-000000000001"
	eavUserA   = "5eb10000-0000-4000-8000-000000000011"
	eavUserB   = "5eb10000-0000-4000-8000-000000000012"
	eavEngA    = "5eb10000-0000-4000-8000-000000000021"
	eavEngB    = "5eb10000-0000-4000-8000-000000000022"
	eavAlloc1  = "5eb10000-0000-4000-8000-000000000031"
	eavAlloc2  = "5eb10000-0000-4000-8000-000000000032"
	eavAlloc3  = "5eb10000-0000-4000-8000-000000000033"
)

func newEngagementAllocationViewIntegrationRepo(t *testing.T) EngagementAllocationViewRepository {
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
		mustExec(t, pool, `DELETE FROM customer_engagement WHERE id IN ($1, $2)`, eavEngA, eavEngB)
		mustExec(t, pool, `DELETE FROM account WHERE id = $1`, eavAccount)
		mustExec(t, pool, `DELETE FROM "user" WHERE id IN ($1, $2)`, eavUserA, eavUserB)
	}
	clean()
	t.Cleanup(clean)

	mustExec(t, pool, `INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number)
		VALUES ($1, now(), now(), 't', 't', 'Example Corp', 'ACC-EAV-IT-1')`, eavAccount)
	mustExec(t, pool, `INSERT INTO "user" (id, created_on, updated_on, user_name, name, email)
		VALUES ($1, now(), now(), 'jane.doe', 'Jane Doe', 'Jane.Doe@example.com')`, eavUserA)
	mustExec(t, pool, `INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name, email)
		VALUES ($1, now(), now(), 'john.roe', 'John', 'Roe', 'john.roe@example.com')`, eavUserB)
	mustExec(t, pool, `INSERT INTO customer_engagement (id, created_on, updated_on, name, engagement_id, state, account_id, engagement_type, delivery_mode)
		VALUES ($1, now(), now(), 'Rollout', 'EITEAV1', 'IN_PROGRESS', $2, 'CONSULTANCY', 'ONSITE')`, eavEngA, eavAccount)
	mustExec(t, pool, `INSERT INTO customer_engagement (id, created_on, updated_on, name, engagement_id, state)
		VALUES ($1, now(), now(), 'No account', 'EITEAV2', 'NEW')`, eavEngB)

	alloc := func(id, ref, eng, user, state, start, end string, util any) {
		mustExec(t, pool, `INSERT INTO customer_engagement_allocation_resource
			(id, created_on, updated_on, allocation_id, engagement_id, resource_id, state, start_date, end_date, start_time, end_time, timezone, utilization)
			VALUES ($1, now(), now(), $2, $3, $4, $5::engagement_allocation_state_enum, $6::date, $7::date, '09:00:00', '17:00:00', 'UTC', $8)`,
			id, ref, eng, user, state, start, end, util)
	}
	alloc(eavAlloc1, "EALLOC-EAV1", eavEngA, eavUserA, "CONFIRMED", "2026-09-21", "2026-09-25", 50)
	alloc(eavAlloc2, "EALLOC-EAV2", eavEngA, eavUserB, "TENTATIVE", "2026-10-05", "2026-10-09", 25.5)
	alloc(eavAlloc3, "EALLOC-EAV3", eavEngB, eavUserB, "CONFIRMED", "2026-08-03", "2026-08-07", nil)
	return NewEngagementAllocationViewRepository(pool)
}

func eavIDs(rows []domain.EngagementAllocationView) []string {
	ids := []string{}
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestEngagementAllocationViewIntegration_SearchFiltersSortAndPage(t *testing.T) {
	repo := newEngagementAllocationViewIntegrationRepo(t)
	ctx := context.Background()
	// The shared test database may hold other rows; scope every search to ours.
	scope := func(f domain.EngagementAllocationFilters) domain.SearchEngagementAllocationsRequest {
		return domain.SearchEngagementAllocationsRequest{Pagination: domain.Pagination{Limit: 50}, Filters: f}
	}
	ours := map[string]bool{eavAlloc1: true, eavAlloc2: true, eavAlloc3: true}
	search := func(req domain.SearchEngagementAllocationsRequest) ([]string, int) {
		t.Helper()
		rows, total, err := repo.SearchAllocations(ctx, req)
		if err != nil {
			t.Fatalf("SearchAllocations: %v", err)
		}
		got := []string{}
		for _, id := range eavIDs(rows) {
			if ours[id] {
				got = append(got, id)
			}
		}
		return got, total
	}
	eq := func(name string, got, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v, want %v", name, got, want)
			}
		}
	}

	got, _ := search(scope(domain.EngagementAllocationFilters{SearchQuery: "EALLOC-EAV"}))
	eq("default order is newest start date first", got, []string{eavAlloc2, eavAlloc1, eavAlloc3})

	got, _ = search(scope(domain.EngagementAllocationFilters{SearchQuery: "eav_"}))
	eq("underscore is literal, not a wildcard", got, []string{})
	got, _ = search(scope(domain.EngagementAllocationFilters{SearchQuery: "rollout"}))
	eq("engagement name, case-insensitive", got, []string{eavAlloc2, eavAlloc1})
	got, _ = search(scope(domain.EngagementAllocationFilters{SearchQuery: "eiteav2"}))
	eq("engagement display id", got, []string{eavAlloc3})
	got, _ = search(scope(domain.EngagementAllocationFilters{AccountID: eavAccount}))
	eq("account id", got, []string{eavAlloc2, eavAlloc1})
	got, _ = search(scope(domain.EngagementAllocationFilters{AccountName: "example"}))
	eq("account name", got, []string{eavAlloc2, eavAlloc1})
	got, _ = search(scope(domain.EngagementAllocationFilters{EngineerID: eavUserB}))
	eq("engineer", got, []string{eavAlloc2, eavAlloc3})
	got, _ = search(scope(domain.EngagementAllocationFilters{EngagementID: eavEngB}))
	eq("engagement", got, []string{eavAlloc3})
	got, _ = search(scope(domain.EngagementAllocationFilters{States: []string{"CONFIRMED"}, SearchQuery: "EALLOC-EAV"}))
	eq("state", got, []string{eavAlloc1, eavAlloc3})
	got, _ = search(scope(domain.EngagementAllocationFilters{From: "2026-09-25", To: "2026-10-05", SearchQuery: "EALLOC-EAV"}))
	eq("overlap, ends inclusive", got, []string{eavAlloc2, eavAlloc1})
	got, _ = search(scope(domain.EngagementAllocationFilters{From: "2026-09-26", To: "2026-10-04", SearchQuery: "EALLOC-EAV"}))
	eq("gap between spans", got, []string{})
	got, _ = search(scope(domain.EngagementAllocationFilters{From: "2026-10-01", SearchQuery: "EALLOC-EAV"}))
	eq("from only", got, []string{eavAlloc2})

	req := scope(domain.EngagementAllocationFilters{SearchQuery: "EALLOC-EAV"})
	req.SortBy = domain.EngagementAllocationSortBy{Field: "engineerName", Order: "asc"}
	got, _ = search(req)
	// Jane Doe < John Roe (name from first and last when there is no name column).
	if got[0] == eavAlloc2 || got[0] == eavAlloc3 {
		t.Fatalf("engineerName asc: Jane Doe's allocation should come first, got %v", got)
	}
	req.SortBy = domain.EngagementAllocationSortBy{Field: "accountName", Order: "asc"}
	got, _ = search(req)
	if got[len(got)-1] != eavAlloc3 {
		t.Fatalf("accountName asc puts the allocation with no account last, got %v", got)
	}

	// Paging: total counts every match, the page is cut.
	req = scope(domain.EngagementAllocationFilters{SearchQuery: "EALLOC-EAV"})
	req.Pagination = domain.Pagination{Limit: 2, Offset: 1}
	rows, total, err := repo.SearchAllocations(ctx, req)
	if err != nil || total != 3 || len(rows) != 2 || rows[0].ID != eavAlloc1 {
		t.Fatalf("page = %v total %d err %v", eavIDs(rows), total, err)
	}
}

func TestEngagementAllocationViewIntegration_DetailAndUpdates(t *testing.T) {
	repo := newEngagementAllocationViewIntegrationRepo(t)
	ctx := context.Background()

	v, err := repo.AllocationByID(ctx, eavAlloc1)
	if err != nil {
		t.Fatalf("AllocationByID: %v", err)
	}
	if v.Engagement.ID != eavEngA || *v.Engagement.DisplayID != "EITEAV1" || *v.Engagement.Type != "CONSULTANCY" ||
		*v.Engagement.ModeOfDelivery != "ONSITE" || *v.Engagement.State != "IN_PROGRESS" {
		t.Errorf("engagement = %+v", v.Engagement)
	}
	if v.Account == nil || v.Account.Name != "Example Corp" || v.Engineer == nil || v.Engineer.ID != eavUserA ||
		v.Engineer.Name != "Jane Doe" || v.Engineer.Email == nil {
		t.Errorf("account/engineer = %+v / %+v", v.Account, v.Engineer)
	}
	if *v.StartDate != "2026-09-21" || *v.EndDate != "2026-09-25" || *v.StartTime != "09:00:00" || *v.Timezone != "UTC" ||
		v.Utilization == nil || *v.Utilization != 50 || *v.State != "CONFIRMED" || v.LatestUpdateDate != nil {
		t.Errorf("allocation = %+v", v)
	}
	if bare, _ := repo.AllocationByID(ctx, eavAlloc3); bare.Account != nil || bare.Utilization != nil {
		t.Errorf("missing data must be absent: %+v", bare)
	}
	if _, err := repo.AllocationByID(ctx, "5eb10000-0000-4000-8000-0000000000ff"); err == nil {
		t.Error("a missing allocation must be an error")
	}

	// UserByEmail is case-insensitive and nil for a stranger.
	if p, err := repo.UserByEmail(ctx, "jane.doe@example.com"); err != nil || p == nil || p.ID != eavUserA {
		t.Errorf("UserByEmail = %+v, %v", p, err)
	}
	if p, err := repo.UserByEmail(ctx, "nobody@example.com"); err != nil || p != nil {
		t.Errorf("stranger = %+v, %v", p, err)
	}

	// Two updates, the second newer; the list is newest first, with the author.
	w := EngagementAllocationUpdateWrite{
		AllocationID: eavAlloc1, EngagementID: eavEngA, AuthorID: eavUserA, Subject: "First", Content: "one",
		State: "PUBLISHED", Frequency: "WEEKLY", Scope: "INDIVIDUAL", Visibility: "INTERNAL",
		CycleStartDate: "2026-09-21", ActorEmail: "jane.doe@example.com",
	}
	first, err := repo.CreateUpdate(ctx, w)
	if err != nil || first.ID == "" || first.PublishedDate == nil {
		t.Fatalf("CreateUpdate: %+v, %v", first, err)
	}
	w.Subject, w.Content = "Second", "two"
	if _, err := repo.CreateUpdate(ctx, w); err != nil {
		t.Fatalf("CreateUpdate 2: %v", err)
	}
	ups, total, err := repo.ListUpdates(ctx, eavAlloc1, 10, 0)
	if err != nil || total != 2 || len(ups) != 2 || *ups[0].Subject != "Second" || *ups[1].Subject != "First" {
		t.Fatalf("ListUpdates = %+v total %d err %v", ups, total, err)
	}
	u := ups[0]
	if *u.State != "PUBLISHED" || *u.Visibility != "INTERNAL" || *u.CycleStartDate != "2026-09-21" ||
		u.Author == nil || u.Author.ID != eavUserA || u.Author.Name != "Jane Doe" {
		t.Errorf("update = %+v", u)
	}
	page, total, _ := repo.ListUpdates(ctx, eavAlloc1, 1, 1)
	if total != 2 || len(page) != 1 || *page[0].Subject != "First" {
		t.Errorf("page 2 = %+v total %d", page, total)
	}
	if after, _ := repo.AllocationByID(ctx, eavAlloc1); after.LatestUpdateDate == nil {
		t.Error("latestUpdateDate must be set once an update is published")
	}
}
