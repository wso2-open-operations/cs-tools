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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// resolutionID is the id of the fixture row with that suffix.
func resolutionID(suffix string) string {
	return "98000000-0000-0000-0000-0000000000" + suffix
}

// The fixture's work items, all resolved or closed in or around March 2019
// (the tests' window is 1 to 31 March 2019, so every other row is outside it):
//
//	01 CASE S1 closed, resolved 5 March: RESOLUTION clock ACHIEVED 10h (an
//	   older ACHIEVED one of 99h, and a RESPONSE one, do not count)
//	02 CASE S1 solution proposed 10 March: no clock, a migrated 20h
//	03 CASE S2 closed 12 March with no resolved_on (migrated): a COMPLETED
//	   RESOLUTION clock of 6h wins over its migrated 50h (a later finished
//	   clock with no duration is passed over)
//	04 CASE S2 closed, resolved 15 March: only a BREACHED and a CANCELLED
//	   clock, no migrated value -- resolved, not measured
//	05 CASE S1 work in progress with a stale resolved_on of 8 March: not
//	   resolved, left out
//	06 CASE S2 closed, resolved 28 February: before the window
//	07 CASE S2 closed, resolved 31 March 23:00: in the window, a migrated 4h
//	08 CASE S2 closed, resolved 1 April 00:00: after the window
//	09 ENGAGEMENT closed, resolved 20 March: no severity, a migrated 8h
//	10 CASE S2 closed, resolved 1 March 00:00: the window's first instant, a
//	   migrated 2h
var resolutionItems = []struct {
	suffix, wiType, state, severity string
	resolvedOn, closedOn            any
	migrated                        any
}{
	{"01", "CASE", "CLOSED", "S1", rtAt(2019, 3, 5, 10), rtAt(2019, 3, 6, 10), nil},
	{"02", "CASE", "SOLUTION_PROPOSED", "S1", rtAt(2019, 3, 10, 9), nil, "20 hours"},
	{"03", "CASE", "CLOSED", "S2", nil, rtAt(2019, 3, 12, 9), "50 hours"},
	{"04", "CASE", "CLOSED", "S2", rtAt(2019, 3, 15, 9), rtAt(2019, 3, 16, 9), nil},
	{"05", "CASE", "WORK_IN_PROGRESS", "S1", rtAt(2019, 3, 8, 9), nil, "1 hour"},
	{"06", "CASE", "CLOSED", "S2", rtAt(2019, 2, 28, 9), rtAt(2019, 3, 2, 9), "1 hour"},
	{"07", "CASE", "CLOSED", "S2", rtAt(2019, 3, 31, 23), rtAt(2019, 4, 2, 9), "4 hours"},
	{"08", "CASE", "CLOSED", "S2", rtAt(2019, 4, 1, 0), rtAt(2019, 4, 2, 9), "1 hour"},
	{"09", "ENGAGEMENT", "CLOSED", "", rtAt(2019, 3, 20, 9), rtAt(2019, 3, 21, 9), "8 hours"},
	{"10", "CASE", "CLOSED", "S2", rtAt(2019, 3, 1, 0), rtAt(2019, 3, 1, 9), "2 hours"},
}

// resolutionClocks are the fixture's SLA clocks: work item, policy, stage,
// business duration and when the clock ended.
var resolutionClocks = []struct {
	suffix, item, policy, stage string
	duration                    any
	endOn                       time.Time
}{
	{"c1", "01", "e1", "ACHIEVED", "10 hours", rtAt(2019, 3, 5, 10)},
	{"c2", "01", "e1", "ACHIEVED", "99 hours", rtAt(2019, 3, 1, 10)},
	{"c3", "01", "e2", "ACHIEVED", "1 hour", rtAt(2019, 3, 1, 9)},
	{"c4", "03", "e1", "COMPLETED", "6 hours", rtAt(2019, 3, 12, 9)},
	{"c5", "04", "e1", "BREACHED", "30 hours", rtAt(2019, 3, 15, 9)},
	{"c6", "04", "e1", "CANCELLED", "40 hours", rtAt(2019, 3, 15, 9)},
	{"c7", "03", "e1", "COMPLETED", nil, rtAt(2019, 3, 13, 9)},
}

func rtAt(y int, m time.Month, d, h int) time.Time {
	return time.Date(y, m, d, h, 0, 0, 0, time.UTC)
}

// seedResolutionFixture inserts the fixture's policies, work items and
// clocks, removing any left over from an earlier run first and again when the
// test ends.
func seedResolutionFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM sla WHERE created_by = 'resolution-time-test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM sla_policy WHERE created_by = 'resolution-time-test'`)
		for _, it := range resolutionItems {
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, resolutionID(it.suffix))
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%s): %v", sql, err)
		}
	}
	for suffix, target := range map[string]string{"e1": "RESOLUTION", "e2": "RESPONSE"} {
		mustExec(`INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, target)
			VALUES ($1, now(), now(), 'resolution-time-test', 'resolution-time-test', $2, $3::sla_policy_target_enum)`,
			resolutionID(suffix), "Resolution time test "+target, target)
	}
	created := rtAt(2019, 2, 1, 9)
	for _, it := range resolutionItems {
		id := resolutionID(it.suffix)
		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, business_duration)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, 'resolution time integration test fixture', $5::work_item_type_enum, $6::interval)`,
			id, created, "RT-"+it.suffix, "RT-WSO2-"+it.suffix, it.wiType, it.migrated)
		switch it.wiType {
		case "CASE":
			mustExec(`INSERT INTO "case" (id, state, severity, resolved_on, closed_on)
				VALUES ($1, $2::case_state_enum, $3::case_severity_enum, $4, $5)`, id, it.state, it.severity, it.resolvedOn, it.closedOn)
		case "ENGAGEMENT":
			mustExec(`INSERT INTO engagement (id, state, type, resolved_on, closed_on)
				VALUES ($1, $2::engagement_state_enum, 'MIGRATION'::engagement_type_enum, $3, $4)`, id, it.state, it.resolvedOn, it.closedOn)
		}
	}
	for _, c := range resolutionClocks {
		mustExec(`INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id, sla_policy_id, stage, business_duration, end_on)
			VALUES ($1, now(), now(), 'resolution-time-test', 'resolution-time-test', $2, $3, $4::sla_stage_enum, $5::interval, $6)`,
			resolutionID(c.suffix), resolutionID(c.item), resolutionID(c.policy), c.stage, c.duration, c.endOn)
	}
}

// resolutionRequest is a case search with the given parsed filters.
func resolutionRequest(p domain.ParsedCaseFilters) domain.SearchCasesRequest {
	return domain.SearchCasesRequest{Parsed: p, Pagination: domain.Pagination{Limit: 50},
		SortBy: domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc}}
}

// marchStart and aprilStart are the tests' window, [1 March, 1 April) 2019.
var marchStart, aprilStart = rtAt(2019, 3, 1, 0), rtAt(2019, 4, 1, 0)

// TestResolutionTimeIntegration_Total: against a real database, the window
// keeps the solution proposed and closed cases resolved (else closed) in it,
// and each case's business duration is its latest finished RESOLUTION clock's,
// else its migrated one, else none.
func TestResolutionTimeIntegration_Total(t *testing.T) {
	pool := caseStatsPool(t)
	seedResolutionFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}

	got, err := repo.CaseResolutionTime(ctx, resolutionRequest(domain.ParsedCaseFilters{}), "", marchStart, aprilStart, scope)
	if err != nil {
		t.Fatalf("CaseResolutionTime: %v", err)
	}
	// 01 (10h), 02 (20h), 03 (6h), 04 (none), 07 (4h), 09 (8h), 10 (2h).
	if len(got) != 1 || got[0].Key != nil || got[0].Count != 7 || got[0].Measured != 6 || got[0].Seconds != 50*3600 {
		t.Fatalf("total = %+v, want one row: 7 resolved, 6 measured, 50 hours", got)
	}

	// A filter narrows the same cases: S1 only is 01 and 02.
	got, err = repo.CaseResolutionTime(ctx, resolutionRequest(domain.ParsedCaseFilters{Severities: []domain.CaseSeverity{"critical"}}), "", marchStart, aprilStart, scope)
	if err != nil {
		t.Fatalf("CaseResolutionTime: %v", err)
	}
	if got[0].Count != 2 || got[0].Measured != 2 || got[0].Seconds != 30*3600 {
		t.Errorf("S1 = %+v, want 2 resolved, 30 hours", got[0])
	}

	// An empty window is one row of nothing.
	got, err = repo.CaseResolutionTime(ctx, resolutionRequest(domain.ParsedCaseFilters{}), "", rtAt(1990, 1, 1, 0), rtAt(1990, 2, 1, 0), scope)
	if err != nil {
		t.Fatalf("CaseResolutionTime: %v", err)
	}
	if len(got) != 1 || got[0].Count != 0 || got[0].Measured != 0 || got[0].Seconds != 0 {
		t.Errorf("empty window = %+v, want one row of zeros", got)
	}
}

// TestResolutionTimeIntegration_Grouped: against a real database, a grouping
// sums each group's cases, labels it as the aggregate does, and returns the
// cases with no value for it (the engagement has no severity) as a nil-keyed
// row.
func TestResolutionTimeIntegration_Grouped(t *testing.T) {
	pool := caseStatsPool(t)
	seedResolutionFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true}

	got, err := repo.CaseResolutionTime(ctx, resolutionRequest(domain.ParsedCaseFilters{}), "severity", marchStart, aprilStart, scope)
	if err != nil {
		t.Fatalf("CaseResolutionTime: %v", err)
	}
	byKey := map[string]domain.CaseResolutionTimeSum{}
	for _, s := range got {
		k := "<none>"
		if s.Key != nil {
			k = *s.Key
		}
		byKey[k] = s
	}
	want := map[string]struct {
		count, measured int
		hours           float64
	}{
		"high":     {4, 3, 12},
		"critical": {2, 2, 30},
		"<none>":   {1, 1, 8},
	}
	if len(byKey) != len(want) {
		t.Fatalf("groups = %+v, want high, critical and the engagement's own row", got)
	}
	for k, w := range want {
		s := byKey[k]
		if s.Count != w.count || s.Measured != w.measured || s.Seconds != w.hours*3600 {
			t.Errorf("%s = %+v, want %d resolved, %d measured, %v hours", k, s, w.count, w.measured, w.hours)
		}
	}
	if got[0].Key == nil || *got[0].Key != "high" || got[0].Label != "high" {
		t.Errorf("first row = %+v, want high (most cases), labelled by its key", got[0])
	}

	got, err = repo.CaseResolutionTime(ctx, resolutionRequest(domain.ParsedCaseFilters{}), "type", marchStart, aprilStart, scope)
	if err != nil {
		t.Fatalf("CaseResolutionTime: %v", err)
	}
	if len(got) != 2 || *got[0].Key != "case" || got[0].Count != 6 || *got[1].Key != "engagement" || got[1].Count != 1 {
		t.Errorf("by type = %+v, want case 6 and engagement 1", got)
	}
}

// TestResolutionTimeIntegration_RefusesGroupings: the state groupings and
// productVersion, and an unknown field, are refused.
func TestResolutionTimeIntegration_RefusesGroupings(t *testing.T) {
	pool := caseStatsPool(t)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	for _, g := range []string{"state", "workState", "productVersion", "nope"} {
		if _, err := repo.CaseResolutionTime(ctx, resolutionRequest(domain.ParsedCaseFilters{}), g, marchStart, aprilStart, repository.SearchScope{Unrestricted: true}); err == nil {
			t.Errorf("groupBy %q: want a validation error", g)
		}
	}
}

const (
	rtrAccountID = "98100000-0000-0000-0000-000000000001"
	rtrProjectID = "98100000-0000-0000-0000-000000000002"
	rtrOtherProj = "98100000-0000-0000-0000-000000000003"
	rtrContactID = "98100000-0000-0000-0000-000000000004"
	rtrOwnCase   = "98100000-0000-0000-0000-000000000005"
	rtrOtherCase = "98100000-0000-0000-0000-000000000006"
	rtrPolicyID  = "98100000-0000-0000-0000-000000000007"
	rtrCustomer  = "rtr-customer@test.local"
)

// resolutionRLSPool connects as CASE_CREATE_RLS_TEST_DSN's role, which does
// not bypass row-level security, and seeds through the system identity a
// customer who is a contact of one of two projects, each with one case
// resolved in March 2019 and its RESOLUTION clock (3 hours in theirs, 5 in
// the other), removing it all again when the test ends.
func resolutionRLSPool(t *testing.T) *pgxpool.Pool {
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
		_, _ = scoped.Exec(sys, `DELETE FROM sla WHERE created_by = 'resolution-rls-test'`)
		_, _ = scoped.Exec(sys, `DELETE FROM sla_policy WHERE id = $1`, rtrPolicyID)
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE id IN ($1, $2)`, rtrOwnCase, rtrOtherCase)
		_, _ = scoped.Exec(sys, `DELETE FROM project_contact WHERE email = $1`, rtrCustomer)
		_, _ = scoped.Exec(sys, `DELETE FROM project WHERE id IN ($1, $2)`, rtrProjectID, rtrOtherProj)
		_, _ = scoped.Exec(sys, `DELETE FROM account_contact WHERE id = $1`, rtrContactID)
		_, _ = scoped.Exec(sys, `DELETE FROM account WHERE id = $1`, rtrAccountID)
	}
	cleanup()
	t.Cleanup(cleanup)

	created := rtAt(2019, 3, 1, 9)
	for _, st := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number) VALUES ($1, $2, $2, 't', 't', 'RTR Account', 'RTR-ACC')`, []any{rtrAccountID, created}},
		{`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, account_id) VALUES ($1, $2, $2, 't', 't', 'RTRPROJ', 'RTR', $3), ($4, $2, $2, 't', 't', 'RTROTHER', 'RTR other', $3)`, []any{rtrProjectID, created, rtrAccountID, rtrOtherProj}},
		{`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id) VALUES ($1, $2, $2, 't', 't', 'RTR Customer', $3)`, []any{rtrContactID, created, rtrAccountID}},
		{`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state) VALUES (gen_random_uuid(), $1, $1, 't', 't', $2, $3, $4, 'REGISTERED')`, []any{created, rtrCustomer, rtrContactID, rtrProjectID}},
		{`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id) VALUES ($1, $3, $3, 'test', 'test', 'RTR-1', 'RTR-WSO2-1', 'resolution RLS fixture', 'CASE', $4), ($2, $3, $3, 'test', 'test', 'RTR-2', 'RTR-WSO2-2', 'resolution RLS fixture', 'CASE', $5)`, []any{rtrOwnCase, rtrOtherCase, created, rtrProjectID, rtrOtherProj}},
		{`INSERT INTO "case" (id, state, severity, resolved_on, closed_on) VALUES ($1, 'CLOSED', 'S2', $3, $3), ($2, 'CLOSED', 'S2', $3, $3)`, []any{rtrOwnCase, rtrOtherCase, rtAt(2019, 3, 10, 9)}},
		{`INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, target) VALUES ($1, now(), now(), 'resolution-rls-test', 'resolution-rls-test', 'Resolution RLS test', 'RESOLUTION')`, []any{rtrPolicyID}},
		{`INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id, sla_policy_id, stage, business_duration, end_on) VALUES (gen_random_uuid(), now(), now(), 'resolution-rls-test', 'resolution-rls-test', $1, $3, 'ACHIEVED', '3 hours', $4), (gen_random_uuid(), now(), now(), 'resolution-rls-test', 'resolution-rls-test', $2, $3, 'ACHIEVED', '5 hours', $4)`, []any{rtrOwnCase, rtrOtherCase, rtrPolicyID, rtAt(2019, 3, 10, 9)}},
	} {
		if _, err := scoped.Exec(sys, st.sql, st.args...); err != nil {
			t.Fatalf("seed (%.60s): %v", st.sql, err)
		}
	}
	return pool
}

// TestResolutionTimeIntegration_ACustomerSeesTheirOwnProjects: with
// row-level security in force, a customer caller counts and measures only
// their own project's resolved case, with its own clock's duration, and an
// internal caller both. The context carries no identity: the repository
// stamps the scope's own, as it does for every caller.
func TestResolutionTimeIntegration_ACustomerSeesTheirOwnProjects(t *testing.T) {
	pool := resolutionRLSPool(t)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	for _, tc := range []struct {
		name            string
		scope           repository.SearchScope
		count, measured int
		hours           float64
	}{
		{"a contact of one project", repository.SearchScope{ProjectIDs: []string{rtrProjectID}, ViewerEmail: rtrCustomer}, 1, 1, 3},
		{"both projects asked for, a contact of one", repository.SearchScope{ProjectIDs: []string{rtrProjectID, rtrOtherProj}, ViewerEmail: rtrCustomer}, 1, 1, 3},
		{"an internal caller", repository.SearchScope{Unrestricted: true}, 2, 2, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.CaseResolutionTime(context.Background(), resolutionRequest(domain.ParsedCaseFilters{}), "", marchStart, aprilStart, tc.scope)
			if err != nil {
				t.Fatalf("CaseResolutionTime: %v", err)
			}
			if len(got) != 1 || got[0].Count != tc.count || got[0].Measured != tc.measured || got[0].Seconds != tc.hours*3600 {
				t.Errorf("got %+v, want %d resolved, %d measured, %v hours", got, tc.count, tc.measured, tc.hours)
			}
		})
	}
}
