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
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// A problem's assignment group (work_item.assignment_group_id) is read back
// by GetProblem and SearchProblems, and the assignmentGroupId filter applies
// to it. Run with ENTITY_TEST_DATABASE_URL; seeded rows are deleted after.
const (
	pgGroupID    = "47777777-0000-0000-0000-0000000000e1"
	pgOtherGroup = "47777777-0000-0000-0000-0000000000e2"
	pgProblemID  = "47777777-0000-0000-0000-0000000000e3"
	pgOtherID    = "47777777-0000-0000-0000-0000000000e4"
)

func TestProblemAssignmentGroupIntegration(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	ctx := WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	cleanup := func() {
		for _, q := range []string{
			`DELETE FROM work_item WHERE id IN ('` + pgProblemID + `', '` + pgOtherID + `')`,
			`DELETE FROM "group" WHERE id IN ('` + pgGroupID + `', '` + pgOtherGroup + `')`,
		} {
			if _, err := pool.Exec(context.Background(), q); err != nil {
				t.Errorf("CLEANUP FAILED (%s): %v", q, err)
			}
		}
	}
	cleanup()
	t.Cleanup(func() { cleanup(); pool.Close() })
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	exec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, NOW(), NOW(), 't', 't', 'Choreo Special Ops (test)'), ($2, NOW(), NOW(), 't', 't', 'Other (test)')`, pgGroupID, pgOtherGroup)
	for id, num := range map[string]string{pgProblemID: "PRB-PG-0001", pgOtherID: "PRB-PG-0002"} {
		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		      VALUES ($1, NOW(), NOW(), 't', 't', $2, 'group read-back test', 'PROBLEM')`, id, num)
		exec(`INSERT INTO problem (id, state, is_active, opened_on) VALUES ($1, 'NEW', TRUE, NOW())`, id)
	}
	repo := NewProblemRepository(NewScoped(pool))

	// Set through the update path, as the Edit dialog does.
	g := pgGroupID
	if _, err := repo.UpdateProblemFields(ctx, domain.UpdateProblemRequest{ID: pgProblemID, AssignmentGroupID: &g}, "t@example.com"); err != nil {
		t.Fatalf("set group: %v", err)
	}
	other := pgOtherGroup
	if _, err := repo.UpdateProblemFields(ctx, domain.UpdateProblemRequest{ID: pgOtherID, AssignmentGroupID: &other}, "t@example.com"); err != nil {
		t.Fatalf("set other group: %v", err)
	}

	d, err := repo.GetProblem(ctx, pgProblemID)
	if err != nil {
		t.Fatalf("GetProblem: %v", err)
	}
	if d.AssignmentGroup == nil || d.AssignmentGroup.ID != pgGroupID || d.AssignmentGroup.Name != "Choreo Special Ops (test)" {
		t.Errorf("GetProblem assignmentGroup = %+v, want the group set", d.AssignmentGroup)
	}

	req := domain.SearchProblemsRequest{Filters: domain.SearchProblemsFilters{SearchQuery: "PRB-PG-"}, Pagination: domain.Pagination{Limit: 10}}
	views, total, err := repo.SearchProblems(ctx, req, nil, nil, []string{pgGroupID})
	if err != nil {
		t.Fatalf("SearchProblems: %v", err)
	}
	if total != 1 || len(views) != 1 || views[0].ID == nil || *views[0].ID != pgProblemID {
		t.Fatalf("filter by group: total %d views %+v, want just %s", total, views, pgProblemID)
	}
	if views[0].AssignmentGroup == nil || views[0].AssignmentGroup.Name != "Choreo Special Ops (test)" {
		t.Errorf("list assignmentGroup = %+v", views[0].AssignmentGroup)
	}
	if _, total, err := repo.SearchProblems(ctx, req, nil, nil, nil); err != nil || total != 2 {
		t.Errorf("no group filter: total %d err %v, want 2", total, err)
	}
}
