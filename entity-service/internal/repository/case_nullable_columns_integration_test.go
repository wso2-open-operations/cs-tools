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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// A case with no description and no project, carrying a comment with no
// content and no type, must still read back from an update, the comment list
// and the activity feed. All four columns are nullable. Skipped without
// CASE_STATS_TEST_DSN.
func TestCaseReads_TolerateNullableColumns(t *testing.T) {
	pool := caseStatsPool(t)
	scoped := repository.NewScoped(pool)
	ctx := rsInternal()
	const caseID = "7a000000-0000-4000-8000-0000000000c9"
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM comment WHERE work_item_id = $1`, caseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM "case" WHERE id = $1`, caseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, caseID)
	}
	cleanup()
	t.Cleanup(cleanup)
	for _, stmt := range []string{
		`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		 VALUES ($1, now(), now(), 't', 't', 'NULLCOL0001', 'NULLCOL-1', 'no description, no project', 'CASE')`,
		`INSERT INTO "case" (id, state) VALUES ($1, 'OPEN')`,
		`INSERT INTO comment (id, created_on, created_by, work_item_id) VALUES (gen_random_uuid(), now(), 'jane.doe@example.com', $1)`,
	} {
		if _, err := scoped.Exec(ctx, stmt, caseID); err != nil {
			t.Fatalf("seed (%.60s): %v", stmt, err)
		}
	}
	repo := repository.NewCaseRepository(scoped)

	st := domain.CaseStateWorkInProgress
	c, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: caseID, State: &st})
	if err != nil {
		t.Fatalf("UpdateCase on a case with NULL description/project: %v", err)
	}
	if c.Description != "" || c.ProjectID != "" {
		t.Errorf("Description=%q ProjectID=%q, want both empty", c.Description, c.ProjectID)
	}

	comments, total, err := repo.SearchCaseComments(ctx, domain.SearchCaseCommentsRequest{CaseID: caseID, Pagination: domain.Pagination{Limit: 10}})
	if err != nil {
		t.Fatalf("SearchCaseComments with a NULL-content comment: %v", err)
	}
	if total != 1 || len(comments) != 1 || comments[0].Content != "" {
		t.Errorf("comments = %+v (total %d), want one with empty content", comments, total)
	}

	acts, total, err := repo.SearchCaseActivities(ctx, domain.SearchCaseActivitiesRequest{CaseID: caseID, Pagination: domain.Pagination{Limit: 10}})
	if err != nil {
		t.Fatalf("SearchCaseActivities with a NULL-content comment: %v", err)
	}
	if total != 1 || len(acts) != 1 || acts[0].Content != "" {
		t.Errorf("activities = %+v (total %d), want one with empty content", acts, total)
	}
}
