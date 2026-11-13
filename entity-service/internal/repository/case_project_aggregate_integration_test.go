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
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// projectAggID is the id of the fixture row with that suffix: cases and
// projects share the prefix, apart by suffix.
func projectAggID(suffix string) string {
	return "96000000-0000-0000-0000-0000000000" + suffix
}

// The fixture's projects: one with a name, one without (NULL) and one with
// an empty name; the last two are labelled with their keys. Its cases, all
// created in April 2019 (the tests' createdOn window keeps every other case
// out):
//
//	PJ-1  PJX Alpha,   open,   high
//	PJ-2  PJX Alpha,   closed, high
//	PJ-3  PJX Alpha,   open,   low
//	PJ-4  PJX-BETA,    open,   high
//	PJ-5  PJX-BETA,    open,   low
//	PJ-6  PJX-GAMMA,   open,   low
//	PJ-7  no project,  open,   high
var projectAggProjects = []struct {
	suffix, key string
	name        any
}{
	{"a1", "PJX-ALPHA", "PJX Alpha"},
	{"b1", "PJX-BETA", nil},
	{"c1", "PJX-GAMMA", ""},
}

var projectAggCases = []struct{ suffix, project, state, severity string }{
	{"01", "a1", "OPEN", "S2"},
	{"02", "a1", "CLOSED", "S2"},
	{"03", "a1", "OPEN", "S4"},
	{"04", "b1", "OPEN", "S2"},
	{"05", "b1", "OPEN", "S4"},
	{"06", "c1", "OPEN", "S4"},
	{"07", "", "OPEN", "S2"},
}

// seedProjectAggFixture inserts the fixture's projects and cases, removing
// any left over from an earlier run first and again when the test ends.
func seedProjectAggFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		for _, c := range projectAggCases {
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, projectAggID(c.suffix))
		}
		for _, p := range projectAggProjects {
			_, _ = scoped.Exec(ctx, `DELETE FROM project WHERE id = $1`, projectAggID(p.suffix))
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Date(2019, time.April, 10, 12, 0, 0, 0, time.UTC)
	for _, p := range projectAggProjects {
		if _, err := scoped.Exec(ctx, `INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, $5)`, projectAggID(p.suffix), now, p.key, "PJX-SF-"+p.suffix, p.name); err != nil {
			t.Fatalf("seed project %s: %v", p.suffix, err)
		}
	}
	for _, c := range projectAggCases {
		var project any
		if c.project != "" {
			project = projectAggID(c.project)
		}
		id := projectAggID(c.suffix)
		if _, err := scoped.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, 'project aggregate integration test fixture', 'CASE', $5)`,
			id, now, "PJ-"+c.suffix, "PJ-WSO2-"+c.suffix, project); err != nil {
			t.Fatalf("seed work_item %s: %v", c.suffix, err)
		}
		if _, err := scoped.Exec(ctx, `INSERT INTO "case" (id, state, severity) VALUES ($1, $2::case_state_enum, $3::case_severity_enum)`,
			id, c.state, c.severity); err != nil {
			t.Fatalf("seed case %s: %v", c.suffix, err)
		}
	}
}

// projectAggRequest is a case search in the fixture's createdOn window with
// the given parsed filters added, sorted as the service defaults it.
func projectAggRequest(p domain.ParsedCaseFilters) domain.SearchCasesRequest {
	from, to := time.Date(2019, time.April, 1, 0, 0, 0, 0, time.UTC), time.Date(2019, time.April, 30, 0, 0, 0, 0, time.UTC)
	p.StartCreatedDate, p.EndCreatedDate = &from, &to
	return domain.SearchCasesRequest{Parsed: p, Pagination: domain.Pagination{Limit: 50},
		SortBy: domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc}}
}

// TestProjectAggregateIntegration_GroupsByProject: against a real database,
// grouping by project gives one bucket per project, keyed by its id (as the
// projectId filter takes it) and labelled with its name, else its key (a
// NULL or empty name), largest first; a case without a project is in no
// bucket; the search's other filters apply; and each bucket's count is the
// projectId filter's total for its key.
func TestProjectAggregateIntegration_GroupsByProject(t *testing.T) {
	pool := caseStatsPool(t)
	seedProjectAggFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}
	alpha, beta, gamma := projectAggID("a1"), projectAggID("b1"), projectAggID("c1")

	for _, tc := range []struct {
		name   string
		parsed domain.ParsedCaseFilters
		want   []domain.AggregateBucket
	}{
		{"every case", domain.ParsedCaseFilters{}, []domain.AggregateBucket{
			{Key: alpha, Label: "PJX Alpha", Count: 3},
			{Key: beta, Label: "PJX-BETA", Count: 2},
			{Key: gamma, Label: "PJX-GAMMA", Count: 1},
		}},
		{"open high-severity cases", domain.ParsedCaseFilters{States: []domain.CaseState{"open"}, Severities: []domain.CaseSeverity{"high"}}, []domain.AggregateBucket{
			{Key: alpha, Label: "PJX Alpha", Count: 1},
			{Key: beta, Label: "PJX-BETA", Count: 1},
		}},
		{"one project's cases", domain.ParsedCaseFilters{ProjectIDs: []string{beta}}, []domain.AggregateBucket{
			{Key: beta, Label: "PJX-BETA", Count: 2},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.AggregateCases(ctx, projectAggRequest(tc.parsed), "project", scope)
			if err != nil {
				t.Fatalf("AggregateCases: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("buckets = %+v, want %+v", got, tc.want)
			}
			for _, b := range got {
				p := tc.parsed
				p.ProjectIDs = []string{b.Key}
				_, total, err := repo.SearchCases(ctx, projectAggRequest(p), scope)
				if err != nil {
					t.Fatalf("SearchCases: %v", err)
				}
				if total != b.Count {
					t.Errorf("bucket %q counts %d, the projectId filter finds %d", b.Key, b.Count, total)
				}
			}
		})
	}
}
