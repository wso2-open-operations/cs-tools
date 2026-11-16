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
// change into a state in a window. Runs against a real Postgres, locally:
// no CI workflow runs the entity-service Go tests. Skipped without
// CASE_STATS_TEST_DSN; the customer caller's test, which needs a role that
// does NOT bypass row-level security (a superuser skips every policy), is
// skipped without CASE_CREATE_RLS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... CASE_CREATE_RLS_TEST_DSN=postgres://csm_app... \
//	  go test ./internal/repository/ -run StateEntered

package repository_test

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// stateEnteredCaseID is the work item id of the fixture case with that
// suffix (SE-<suffix>).
func stateEnteredCaseID(suffix string) string {
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
//	SE-F  now Waiting on WSO2, created in it: no state row at all
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
	{"6", "WAITING_ON_WSO2", nil},
}

// seedStateEnteredFixture inserts the fixture's cases and their activity
// rows, removing any left over from an earlier run first and again when the
// test ends.
func seedStateEnteredFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		for _, c := range stateEnteredFixture {
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, stateEnteredCaseID(c.suffix))
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	created := time.Date(2019, time.March, 10, 12, 0, 0, 0, time.UTC)
	for _, c := range stateEnteredFixture {
		id := stateEnteredCaseID(c.suffix)
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

// stateEnteredAt parses an RFC3339 instant for a test window, panicking on
// a typo.
func stateEnteredAt(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

// TestStateEnteredIntegration_MatchesTheLoggedChangesIntoTheState: against
// a real database, a case matches when its state history has a change into
// one of the states inside the window, whether it is still there, once
// however many times it entered, and only from state rows.
func TestStateEnteredIntegration_MatchesTheLoggedChangesIntoTheState(t *testing.T) {
	pool := caseStatsPool(t)
	seedStateEnteredFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}

	wow, awaiting := []string{"waiting on wso2"}, []string{"awaiting info"}
	today, todayEnd := stateEnteredAt("2026-08-15T00:00:00Z"), stateEnteredAt("2026-08-15T23:59:59.999999999Z")
	yesterdayEnd := stateEnteredAt("2026-08-14T23:59:59.999999999Z")
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
		{"between 09:30 and 10:30", domain.ParsedCaseFilters{EnteredStateLabels: wow, EnteredStartDate: stateEnteredAt("2026-08-15T09:30:00Z"), EnteredEndDate: stateEnteredAt("2026-08-15T10:30:00Z")}, []string{"SE-1"}},
		{"a window starting at the change", domain.ParsedCaseFilters{EnteredStateLabels: wow, EnteredStartDate: stateEnteredAt("2026-08-15T10:00:00Z"), EnteredEndDate: stateEnteredAt("2026-08-15T10:30:00Z")}, []string{"SE-1"}},
		{"a window ending at the change", domain.ParsedCaseFilters{EnteredStateLabels: wow, EnteredStartDate: stateEnteredAt("2026-08-15T09:30:00Z"), EnteredEndDate: stateEnteredAt("2026-08-15T10:00:00Z")}, []string{"SE-1"}},
		{"into a state no case entered", domain.ParsedCaseFilters{EnteredStateLabels: []string{"closed"}}, nil},
		{"into Open: only a move back, never a creation", domain.ParsedCaseFilters{EnteredStateLabels: []string{"open"}}, []string{"SE-5"}},
		{"in WOW now, created there or not", domain.ParsedCaseFilters{States: []domain.CaseState{"waiting_on_wso2"}}, []string{"SE-2", "SE-3", "SE-6"}},
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
		EnteredStateLabels: []string{"waiting on wso2"}, EnteredStartDate: stateEnteredAt("2026-08-15T00:00:00Z"),
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

// The customer caller's fixture: a project the customer is a registered
// contact of and one they are not, each with a case that moved to Waiting on
// WSO2 on 15 August 2026.
const (
	serAccountID  = "93100000-0000-0000-0000-000000000001"
	serProjectID  = "93100000-0000-0000-0000-000000000002"
	serOtherProj  = "93100000-0000-0000-0000-000000000003"
	serContactID  = "93100000-0000-0000-0000-000000000004"
	serOwnCase    = "93100000-0000-0000-0000-000000000005"
	serOtherCase  = "93100000-0000-0000-0000-000000000006"
	serCustomer   = "ser-customer@test.local"
	serMovedToWOW = "2026-08-15T10:00:00Z"
)

// stateEnteredRLSPool connects as CASE_CREATE_RLS_TEST_DSN's role, which
// does not bypass row-level security, and seeds the customer caller's
// fixture through the system identity, removing it again when the test ends.
func stateEnteredRLSPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CASE_CREATE_RLS_TEST_DSN")
	if dsn == "" {
		t.Skip("CASE_CREATE_RLS_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE id IN ($1, $2)`, serOwnCase, serOtherCase)
		_, _ = scoped.Exec(sys, `DELETE FROM project_contact WHERE email = $1`, serCustomer)
		_, _ = scoped.Exec(sys, `DELETE FROM project WHERE id IN ($1, $2)`, serProjectID, serOtherProj)
		_, _ = scoped.Exec(sys, `DELETE FROM account_contact WHERE id = $1`, serContactID)
		_, _ = scoped.Exec(sys, `DELETE FROM account WHERE id = $1`, serAccountID)
	}
	cleanup()
	t.Cleanup(cleanup)

	created := time.Date(2019, time.March, 10, 12, 0, 0, 0, time.UTC)
	for _, st := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number) VALUES ($1, $2, $2, 't', 't', 'SER Account', 'SER-ACC')`, []any{serAccountID, created}},
		{`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, account_id) VALUES ($1, $2, $2, 't', 't', 'SERPROJ', 'SER', $3), ($4, $2, $2, 't', 't', 'SEROTHER', 'SER other', $3)`, []any{serProjectID, created, serAccountID, serOtherProj}},
		{`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id) VALUES ($1, $2, $2, 't', 't', 'SER Customer', $3)`, []any{serContactID, created, serAccountID}},
		{`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state) VALUES (gen_random_uuid(), $1, $1, 't', 't', $2, $3, $4, 'REGISTERED')`, []any{created, serCustomer, serContactID, serProjectID}},
		{`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id) VALUES ($1, $3, $3, 'test', 'test', 'SER-1', 'SER-WSO2-1', 'state-entered RLS fixture', 'CASE', $4), ($2, $3, $3, 'test', 'test', 'SER-2', 'SER-WSO2-2', 'state-entered RLS fixture', 'CASE', $5)`, []any{serOwnCase, serOtherCase, created, serProjectID, serOtherProj}},
		{`INSERT INTO "case" (id, state) VALUES ($1, 'WAITING_ON_WSO2'), ($2, 'WAITING_ON_WSO2')`, []any{serOwnCase, serOtherCase}},
		{`INSERT INTO work_item_activity (id, created_on, created_by, work_item_id, field_name, old_value, new_value) VALUES (gen_random_uuid(), $1::timestamptz, 'test', $2, 'state', 'Open', 'Waiting on WSO2'), (gen_random_uuid(), $1::timestamptz, 'test', $3, 'state', 'Open', 'Waiting on WSO2')`, []any{serMovedToWOW, serOwnCase, serOtherCase}},
	} {
		if _, err := scoped.Exec(sys, st.sql, st.args...); err != nil {
			t.Fatalf("seed (%.60s): %v", st.sql, err)
		}
	}
	return pool
}

// TestStateEnteredIntegration_ACustomerSeesTheirOwnProjectsMoves: with
// row-level security in force (work_item_activity's policy shows a customer
// only their projects' rows), a customer caller matches the move in their
// own project and not the one in a project they are not a contact of.
func TestStateEnteredIntegration_ACustomerSeesTheirOwnProjectsMoves(t *testing.T) {
	pool := stateEnteredRLSPool(t)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	parsed := domain.ParsedCaseFilters{EnteredStateLabels: []string{"waiting on wso2"}, EnteredStartDate: stateEnteredAt("2026-08-15T00:00:00Z")}

	for _, tc := range []struct {
		name  string
		scope repository.SearchScope
		want  []string
	}{
		{"a contact of one project", repository.SearchScope{ProjectIDs: []string{serProjectID}, ViewerEmail: serCustomer}, []string{"SER-1"}},
		{"both projects asked for, a contact of one", repository.SearchScope{ProjectIDs: []string{serProjectID, serOtherProj}, ViewerEmail: serCustomer}, []string{"SER-1"}},
		{"an internal caller", repository.SearchScope{Unrestricted: true}, []string{"SER-1", "SER-2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := repository.WithCallerIdentity(context.Background(), tc.scope)
			if tc.scope.Unrestricted {
				ctx = repository.WithSystemIdentity(context.Background())
			}
			views, total, err := repo.SearchCases(ctx, stateEnteredRequest(parsed), tc.scope)
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
