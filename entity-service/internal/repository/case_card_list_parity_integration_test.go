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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The dashboard and Support cards (Outstanding, Closed (Last 30d)) each link to
// a list, and a customer compares the two. Both sides are built from different
// code: the card from the stats repositories, the list from SearchCases with the
// filters the portal sends. This pins that, for the same project, the two agree
// for every case-like type -- including the three kinds of row that used to make
// them disagree on real synced data:
//
//   - a work item with no extension row at all (no state anywhere);
//   - a work item typed CASE whose only extension row is an engagement's (a state,
//     but not one a list filtered by state on type CASE can see);
//   - a service request / engagement that was closed in the window (the closed-date
//     filter used to read "case".closed_on only, so those never matched).
//
// Skipped without CASE_STATS_TEST_DSN, like the other stats integration tests.
const cardListProject = "55555555-5555-5555-5555-555555555555"

// cardListOpenStates is every non-CLOSED state of the case-like state enums, the
// list the customer portal builds for an "outstanding" list from the project's
// filter metadata.
var cardListOpenStates = []domain.CaseState{
	"open", "work_in_progress", "awaiting_info", "waiting_on_wso2", "reopened", "solution_proposed",
}

func seedCardListParity(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE created_by = 'card-list-parity@wso2.com'`)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, cardListProject)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id)
	          VALUES ($1, now(), now(), 'seed', 'seed', 'CARDLIST', 'sf-cardlist')`, cardListProject)

	// ext is which extension table gets the row: "" = none at all.
	items := []struct {
		wiType, ext, state, closedOn string
	}{
		{"CASE", "case", "OPEN", "NULL"},
		{"CASE", "case", "WORK_IN_PROGRESS", "NULL"},
		{"CASE", "case", "CLOSED", "now() - INTERVAL '2 days'"},
		{"CASE", "", "", "NULL"}, // no extension row at all
		// typed CASE, an engagement's row, with a closure time inside the window: it must not
		// match a CASE closed-date search on the engagement's date
		{"CASE", "engagement", "OPEN", "now() - INTERVAL '5 days'"},
		{"SERVICE_REQUEST", "service_request", "OPEN", "NULL"},
		{"SERVICE_REQUEST", "service_request", "CLOSED", "now() - INTERVAL '3 days'"},
		{"SERVICE_REQUEST", "service_request", "CLOSED", "now() - INTERVAL '45 days'"},
		{"ENGAGEMENT", "engagement", "CLOSED", "now() - INTERVAL '10 days'"},
		{"ENGAGEMENT", "engagement", "OPEN", "NULL"},
	}
	for i, it := range items {
		id := plcID(900 + i)
		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		          VALUES ($1, now(), now(), 'card-list-parity@wso2.com', 'card-list-parity@wso2.com', $2, $3, 'card list parity', $4::work_item_type_enum, $5)`,
			id, "CL-"+id[24:], "CL-W-"+id[24:], it.wiType, cardListProject)
		switch it.ext {
		case "case":
			mustExec(`INSERT INTO "case" (id, state, severity, closed_on)
			          VALUES ($1, $2::case_state_enum, 'S3'::case_severity_enum, `+it.closedOn+`)`, id, it.state)
		case "engagement":
			mustExec(`INSERT INTO engagement (id, state, type, closed_on)
			          VALUES ($1, $2::engagement_state_enum, 'MIGRATION'::engagement_type_enum, `+it.closedOn+`)`, id, it.state)
		case "service_request":
			mustExec(`INSERT INTO service_request (id, state, closed_on)
			          VALUES ($1, $2::service_request_state_enum, `+it.closedOn+`)`, id, it.state)
		}
	}
}

func TestCardsAgreeWithTheirListsIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	seedCardListParity(t, pool)

	ctx := repository.WithSystemIdentity(context.Background())
	scope := repository.SearchScope{Unrestricted: true, ViewerEmail: "card-list-parity@wso2.com"}
	scopedCtx := repository.WithCallerIdentity(context.Background(), scope)
	stats := repository.NewProjectCaseStatsRepository(repository.NewScoped(pool))
	cases := repository.NewCaseRepository(repository.NewScoped(pool))

	// listTotal is the total the portal's list shows: the same search the cards
	// link to, counted.
	listTotal := func(t *testing.T, parsed domain.ParsedCaseFilters) int {
		t.Helper()
		parsed.ProjectIDs = []string{cardListProject}
		_, total, err := cases.SearchCases(scopedCtx, domain.SearchCasesRequest{
			SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc},
			Pagination: domain.Pagination{Limit: 5},
			Parsed:     parsed,
			CountOnly:  true,
		}, scope)
		if err != nil {
			t.Fatalf("SearchCases: %v", err)
		}
		return total
	}

	// Outstanding: the card counts every state but CLOSED, from the stats repository.
	cardOutstanding := func(t *testing.T, typ string) int {
		t.Helper()
		rows, err := stats.StateSeverityCounts(ctx, repository.ProjectCaseStatsFilter{
			ProjectID: cardListProject, Types: []string{typ},
		})
		if err != nil {
			t.Fatalf("StateSeverityCounts(%s): %v", typ, err)
		}
		n := 0
		for _, r := range rows {
			// The same rule projectCaseStatsService applies: a row with no state of
			// its own type is neither outstanding nor closed.
			if r.State != "" && r.State != "CLOSED" {
				n += r.Count
			}
		}
		return n
	}

	// Closed (Last 30d): the card's own resolved buckets, against the list's
	// closed-date filter over the same states.
	now := time.Now()
	windowStart := now.Add(-30 * 24 * time.Hour)
	cardClosed := func(t *testing.T, typ string) int {
		t.Helper()
		_, past30, err := stats.ResolvedBuckets(ctx, repository.ProjectCaseStatsFilter{
			ProjectID: cardListProject, Types: []string{typ},
		}, []string{"CLOSED", "SOLUTION_PROPOSED"})
		if err != nil {
			t.Fatalf("ResolvedBuckets(%s): %v", typ, err)
		}
		return past30
	}

	for _, tc := range []struct {
		typ             string
		wantOutstanding int
		wantClosed      int
	}{
		// 2 open cases; the stateless one and the engagement-row one are in no state list.
		{"case", 2, 1},
		{"service_request", 1, 1}, // the 45-day-old closure is outside the window
		{"engagement", 1, 1},
	} {
		t.Run(tc.typ, func(t *testing.T) {
			if got := cardOutstanding(t, tc.typ); got != tc.wantOutstanding {
				t.Errorf("card outstanding = %d, want %d", got, tc.wantOutstanding)
			}
			if got := listTotal(t, domain.ParsedCaseFilters{Types: []string{tc.typ}, States: cardListOpenStates}); got != tc.wantOutstanding {
				t.Errorf("outstanding list = %d, want %d", got, tc.wantOutstanding)
			}

			if got := cardClosed(t, tc.typ); got != tc.wantClosed {
				t.Errorf("card closed (30d) = %d, want %d", got, tc.wantClosed)
			}
			closedList := listTotal(t, domain.ParsedCaseFilters{
				Types:           []string{tc.typ},
				States:          []domain.CaseState{"closed", "solution_proposed"},
				ClosedStartDate: &windowStart,
				ClosedEndDate:   &now,
			})
			if closedList != tc.wantClosed {
				t.Errorf("closed (30d) list = %d, want %d", closedList, tc.wantClosed)
			}
			// A closed-date search with no state filter at all reads the date of the item's own
			// type only, so the CASE-typed item with an engagement's closure time is not found.
			dateOnly := listTotal(t, domain.ParsedCaseFilters{
				Types: []string{tc.typ}, ClosedStartDate: &windowStart, ClosedEndDate: &now,
			})
			if dateOnly != tc.wantClosed {
				t.Errorf("closed-date-only list = %d, want %d", dateOnly, tc.wantClosed)
			}
		})
	}

	// The per-type outstanding counts of GET /projects/{id}/stats read the same state.
	t.Run("OutstandingCounts per type", func(t *testing.T) {
		counts, err := repository.NewProjectStatsRepository(repository.NewScoped(pool)).OutstandingCounts(ctx, cardListProject,
			[]string{"OPEN", "WORK_IN_PROGRESS", "AWAITING_INFO", "WAITING_ON_WSO2", "REOPENED", "SOLUTION_PROPOSED"}, nil)
		if err != nil {
			t.Fatalf("OutstandingCounts: %v", err)
		}
		if counts["case"] != 2 || counts["service_request"] != 1 || counts["engagement"] != 1 {
			t.Errorf("OutstandingCounts = %v, want case 2 / service_request 1 / engagement 1", counts)
		}
	})
}
