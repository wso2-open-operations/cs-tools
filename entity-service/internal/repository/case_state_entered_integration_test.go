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

// Integration tests for the case search's stateEntered / stateEnteredOn
// filters: the cases whose logged state history (work_item_activity) has a
// change into a state in a window. Runs against a real Postgres. Skipped
// without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run StateEntered

package repository_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func stateEnteredID(suffix string) string {
	return "93000000-0000-0000-0000-00000000000" + suffix
}

// The state history of the fixture's cases, all created in March 2019 (the
// tests' createdOn window keeps every other case out). "Today" is 15 August
// 2026.
//
//	SE-A  now Work In Progress: into Waiting on WSO2 today 10:00, out again 11:00
//	SE-B  now Waiting on WSO2: into it twice today (09:00, 14:00)
//	SE-C  now Waiting on WSO2: into it yesterday
//	SE-D  now Awaiting Info: into it today, logged in upper case (as a sync may)
//	SE-E  now Open: only a severity change today, and a state change out of
//	      Waiting on WSO2 today
var stateEnteredFixture = []struct {
	suffix, state string
	history       [][4]string // field, old, new, logged at (RFC3339)
}{
	{"1", "WORK_IN_PROGRESS", [][4]string{
		{"state", "Open", "Waiting on WSO2", "2026-08-15T10:00:00Z"},
		{"state", "Waiting on WSO2", "Work In Progress", "2026-08-15T11:00:00Z"},
	}},
	{"2", "WAITING_ON_WSO2", [][4]string{
		{"state", "Awaiting Info", "Waiting on WSO2", "2026-08-15T09:00:00Z"},
		{"state", "Waiting on WSO2", "Awaiting Info", "2026-08-15T12:00:00Z"},
		{"state", "Awaiting Info", "Waiting on WSO2", "2026-08-15T14:00:00Z"},
	}},
	{"3", "WAITING_ON_WSO2", [][4]string{
		{"state", "Open", "Waiting on WSO2", "2026-08-14T16:00:00Z"},
	}},
	{"4", "AWAITING_INFO", [][4]string{
		{"state", "Work In Progress", "AWAITING INFO", "2026-08-15T08:00:00Z"},
	}},
	{"5", "OPEN", [][4]string{
		{"severity", "S3", "Waiting on WSO2", "2026-08-15T08:00:00Z"},
		{"state", "Waiting on WSO2", "Open", "2026-08-15T08:30:00Z"},
	}},
}

func seedStateEnteredFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		for _, c := range stateEnteredFixture {
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, stateEnteredID(c.suffix))
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	created := time.Date(2019, time.March, 10, 12, 0, 0, 0, time.UTC)
	for _, c := range stateEnteredFixture {
		id := stateEnteredID(c.suffix)
		if _, err := scoped.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, 'state-entered integration test fixture', 'CASE')`,
			id, created, "SE-"+c.suffix, "SE-WSO2-"+c.suffix); err != nil {
			t.Fatalf("seed work_item %s: %v", c.suffix, err)
		}
		if _, err := scoped.Exec(ctx, `INSERT INTO "case" (id, state) VALUES ($1, $2::case_state_enum)`, id, c.state); err != nil {
			t.Fatalf("seed case %s: %v", c.suffix, err)
		}
		for _, h := range c.history {
			if _, err := scoped.Exec(ctx, `INSERT INTO work_item_activity (id, created_on, created_by, work_item_id, field_name, old_value, new_value)
				VALUES (gen_random_uuid(), $1::timestamptz, 'test', $2, $3, $4, $5)`, h[3], id, h[0], h[1], h[2]); err != nil {
				t.Fatalf("seed activity %s: %v", c.suffix, err)
			}
		}
	}
}

// stateEnteredRequest is a case search in the fixture's createdOn window with
// the given parsed filters added, sorted as the service defaults it.
func stateEnteredRequest(p domain.ParsedCaseFilters) domain.SearchCasesRequest {
	from, to := time.Date(2019, time.March, 1, 0, 0, 0, 0, time.UTC), time.Date(2019, time.March, 31, 0, 0, 0, 0, time.UTC)
	p.StartCreatedDate, p.EndCreatedDate = &from, &to
	return domain.SearchCasesRequest{Parsed: p, Pagination: domain.Pagination{Limit: 50},
		SortBy: domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc}}
}

func at(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestStateEnteredIntegration_MatchesTheLoggedChangesIntoTheState(t *testing.T) {
	pool := caseStatsPool(t)
	seedStateEnteredFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}

	wow, awaiting := []string{"waiting on wso2"}, []string{"awaiting info"}
	today, todayEnd := at("2026-08-15T00:00:00Z"), at("2026-08-15T23:59:59.999999999Z")
	yesterdayEnd := at("2026-08-14T23:59:59.999999999Z")
	for _, tc := range []struct {
		name   string
		parsed domain.ParsedCaseFilters
		want   []string
	}{
		{"into WOW today, left again or not, twice once", domain.ParsedCaseFilters{EnteredStateLabels: wow, EnteredStartDate: today}, []string{"SE-1", "SE-2"}},
		{"into WOW today, a closed window", domain.ParsedCaseFilters{EnteredStateLabels: wow, EnteredStartDate: today, EnteredEndDate: todayEnd}, []string{"SE-1", "SE-2"}},
		{"into WOW today and in WOW now", domain.ParsedCaseFilters{EnteredStateLabels: wow, EnteredStartDate: today, States: []domain.CaseState{"waiting_on_wso2"}}, []string{"SE-2"}},
		{"into WOW up to yesterday", domain.ParsedCaseFilters{EnteredStateLabels: wow, EnteredEndDate: yesterdayEnd}, []string{"SE-3"}},
		{"into WOW at any time", domain.ParsedCaseFilters{EnteredStateLabels: wow}, []string{"SE-1", "SE-2", "SE-3"}},
		{"into Awaiting Info today, any letter case", domain.ParsedCaseFilters{EnteredStateLabels: awaiting, EnteredStartDate: today}, []string{"SE-2", "SE-4"}},
		{"into either today", domain.ParsedCaseFilters{EnteredStateLabels: append(append([]string{}, wow...), awaiting...), EnteredStartDate: today}, []string{"SE-1", "SE-2", "SE-4"}},
		{"between 09:30 and 10:30", domain.ParsedCaseFilters{EnteredStateLabels: wow, EnteredStartDate: at("2026-08-15T09:30:00Z"), EnteredEndDate: at("2026-08-15T10:30:00Z")}, []string{"SE-1"}},
		{"a window starting at the change", domain.ParsedCaseFilters{EnteredStateLabels: wow, EnteredStartDate: at("2026-08-15T10:00:00Z"), EnteredEndDate: at("2026-08-15T10:30:00Z")}, []string{"SE-1"}},
		{"a window ending at the change", domain.ParsedCaseFilters{EnteredStateLabels: wow, EnteredStartDate: at("2026-08-15T09:30:00Z"), EnteredEndDate: at("2026-08-15T10:00:00Z")}, []string{"SE-1"}},
		{"into a state no case entered", domain.ParsedCaseFilters{EnteredStateLabels: []string{"closed"}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			views, total, err := repo.SearchCases(ctx, stateEnteredRequest(tc.parsed), scope)
			if err != nil {
				t.Fatalf("SearchCases: %v", err)
			}
			var got []string
			for _, v := range views {
				got = append(got, v.Number)
			}
			sort.Strings(got)
			if total != len(tc.want) || len(got) != len(tc.want) {
				t.Fatalf("got %v (total %d), want %v", got, total, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestStateEnteredIntegration_TheAggregateAgrees: the aggregate shares the
// search's WHERE, so the same cases are grouped (by their state now).
func TestStateEnteredIntegration_TheAggregateAgrees(t *testing.T) {
	pool := caseStatsPool(t)
	seedStateEnteredFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	buckets, err := repo.AggregateCases(ctx, stateEnteredRequest(domain.ParsedCaseFilters{
		EnteredStateLabels: []string{"waiting on wso2"}, EnteredStartDate: at("2026-08-15T00:00:00Z"),
	}), "state", repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("AggregateCases: %v", err)
	}
	got := map[string]int{}
	for _, b := range buckets {
		got[b.Key] += b.Count
	}
	if len(got) != 2 || got["work_in_progress"]+got["WORK_IN_PROGRESS"] != 1 || got["waiting_on_wso2"]+got["WAITING_ON_WSO2"] != 1 {
		t.Errorf("buckets = %+v, want one Work In Progress (SE-1) and one Waiting on WSO2 (SE-2)", buckets)
	}
}
