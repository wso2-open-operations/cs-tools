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

// Integration test for sorting case search results by assignee
// (digiops-cs#2998): SearchCases's pgSortColMap now has an "assignee" entry
// reusing the same assigned-engineer display-name expression the SELECT
// list already computes, with no new join. Same DSN and skip-when-unset
// pattern as time_card_daily_cap_integration_test.go (caseStatsPool, this
// same package) -- sorting on a COALESCE/CONCAT_WS expression can only be
// verified against a real Postgres.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseAssigneeSort

package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const assigneeSortProjectID = "41111111-1111-1111-1111-111111111113"

// Cases deliberately seeded out of alphabetical assignee order (Charlie,
// Alice, Bob, then one left unassigned) so a passing test can only mean the
// query actually sorted them, not that they happened to come back in
// insertion order.
var assigneeSortCases = []struct {
	caseID, caseNumber, userID, userEmail, firstName, lastName string
}{
	{"48888888-0000-0000-0000-000000000001", "CASRT01", "49999999-0000-0000-0000-000000000001", "case-assignee-sort-test-charlie@example.com", "Charlie", "Brown"},
	{"48888888-0000-0000-0000-000000000002", "CASRT02", "49999999-0000-0000-0000-000000000002", "case-assignee-sort-test-alice@example.com", "Alice", "Smith"},
	{"48888888-0000-0000-0000-000000000003", "CASRT03", "49999999-0000-0000-0000-000000000003", "case-assignee-sort-test-bob@example.com", "Bob", "Jones"},
}

const assigneeSortUnassignedCaseID = "48888888-0000-0000-0000-000000000004"

// seedAssigneeSortFixture creates one project, three users with distinct
// names, three cases each assigned to one of them, and a fourth case left
// unassigned (assigned_to_id NULL) to prove NULLS LAST groups it at the end
// regardless of sort direction.
func seedAssigneeSortFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM "case" WHERE id IN (SELECT id FROM work_item WHERE created_by = 'case-assignee-sort-test')`)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE created_by = 'case-assignee-sort-test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE user_name LIKE 'case-assignee-sort-test-%@example.com'`)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, assigneeSortProjectID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, end_date)
	          VALUES ($1, now(), now(), 'case-assignee-sort-test', 'case-assignee-sort-test', 'CASEASSORT', 'sf-caseassort', (now() + INTERVAL '30 days')::date)`,
		assigneeSortProjectID)

	for _, tc := range assigneeSortCases {
		mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, first_name, last_name, is_active)
		          VALUES ($1, now(), now(), $2, $2, $3, $4, true)`,
			tc.userID, tc.userEmail, tc.firstName, tc.lastName)

		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id, assigned_to_id)
		          VALUES ($1, now(), now(), 'case-assignee-sort-test', 'case-assignee-sort-test', $2, $2, 'a case', 'CASE', $3, $4)`,
			tc.caseID, tc.caseNumber, assigneeSortProjectID, tc.userID)
		mustExec(`INSERT INTO "case" (id, state, severity) VALUES ($1, 'OPEN', 'S2')`, tc.caseID)
	}

	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id, assigned_to_id)
	          VALUES ($1, now(), now(), 'case-assignee-sort-test', 'case-assignee-sort-test', 'CASRT04', 'CASRT04', 'a case', 'CASE', $2, NULL)`,
		assigneeSortUnassignedCaseID, assigneeSortProjectID)
	mustExec(`INSERT INTO "case" (id, state, severity) VALUES ($1, 'OPEN', 'S2')`, assigneeSortUnassignedCaseID)
}

// searchAssigneeSortNames runs SearchCases sorted by assignee and returns
// the assignee display name per result row (empty string for unassigned),
// in result order.
func searchAssigneeSortNames(t *testing.T, pool *pgxpool.Pool, order domain.CaseSortOrder) []string {
	t.Helper()
	caseRepo := repository.NewCaseRepository(repository.NewScoped(pool))
	// buildCaseSearchWhere reads req.Parsed, not the raw Filters.Filters --
	// that translation normally happens in the service layer
	// (service.ParseCaseFieldFilters), which this repository-level test
	// bypasses entirely, so Parsed.ProjectIDs is set directly here instead.
	req := domain.SearchCasesRequest{
		Parsed:     domain.ParsedCaseFilters{ProjectIDs: []string{assigneeSortProjectID}},
		SortBy:     domain.CaseSort{Field: domain.CaseSortFieldAssignee, Order: order},
		Pagination: domain.Pagination{Limit: 10, Offset: 0},
	}
	views, total, err := caseRepo.SearchCases(context.Background(), req, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("SearchCases() error = %v", err)
	}
	if total != 4 {
		t.Fatalf("SearchCases() total = %d, want 4", total)
	}
	names := make([]string, len(views))
	for i, v := range views {
		if v.AssignedEngineer != nil {
			names[i] = v.AssignedEngineer.Name
		}
	}
	return names
}

func TestCaseAssigneeSort_Ascending(t *testing.T) {
	pool := caseStatsPool(t)
	seedAssigneeSortFixture(t, pool)

	got := searchAssigneeSortNames(t, pool, domain.CaseSortOrderAsc)
	want := []string{"Alice Smith", "Bob Jones", "Charlie Brown", ""}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("position %d = %q, want %q (full order: %v)", i, got[i], name, got)
		}
	}
}

func TestCaseAssigneeSort_DescendingStillPutsUnassignedLast(t *testing.T) {
	pool := caseStatsPool(t)
	seedAssigneeSortFixture(t, pool)

	got := searchAssigneeSortNames(t, pool, domain.CaseSortOrderDesc)
	want := []string{"Charlie Brown", "Bob Jones", "Alice Smith", ""}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("position %d = %q, want %q (full order: %v)", i, got[i], name, got)
		}
	}
}
