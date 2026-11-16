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

// Integration tests for POST /time-cards/aggregate's repository method,
// against a real Postgres (time_card RLS, migration 0144, needs a real
// identity, and the minute sums and day series are SQL). Same DSN and skip
// pattern as time_card_repo_test.go:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run TimeCardAggregateIntegration

package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	tcAggProjectA = "47111111-0000-0000-0000-00000000000a" // named "Agg Alpha"
	tcAggProjectB = "47111111-0000-0000-0000-00000000000b" // no name, key TCAGGB
	tcAggUserAnn  = "47222222-0000-0000-0000-00000000000a" // named "Ann Agg"
	tcAggUserBob  = "47222222-0000-0000-0000-00000000000b" // no name, only an email
	tcAggCaseA    = "47555555-0000-0000-0000-00000000000a" // in project A
	tcAggCaseB    = "47555555-0000-0000-0000-00000000000b" // in project B
)

// tcAggCards is the fixture, chosen so that ranking by count and by minutes
// disagree (Ann logs more cards, Bob more minutes):
//
//	card  user  project  date        state      minutes
//	c1    Ann   A        2026-03-02  APPROVED   60
//	c2    Ann   A        2026-03-02  SUBMITTED  30
//	c3    Ann   A        2026-03-04  APPROVED   NULL in all five columns (0)
//	c4    Bob   B        2026-03-03  APPROVED   240
//	c5    Bob   none     2026-03-05  REJECTED   15
//	c6    Ann   A        2026-04-10  APPROVED   45 (outside the March window)
var tcAggCards = []struct {
	id, user, caseID, date, state string
	project                       *string
	minutes                       *int
}{
	{"47666666-0000-0000-0000-000000000001", tcAggUserAnn, tcAggCaseA, "2026-03-02", "APPROVED", ptrTo(tcAggProjectA), intPtr(60)},
	{"47666666-0000-0000-0000-000000000002", tcAggUserAnn, tcAggCaseA, "2026-03-02", "SUBMITTED", ptrTo(tcAggProjectA), intPtr(30)},
	{"47666666-0000-0000-0000-000000000003", tcAggUserAnn, tcAggCaseA, "2026-03-04", "APPROVED", ptrTo(tcAggProjectA), nil},
	{"47666666-0000-0000-0000-000000000004", tcAggUserBob, tcAggCaseB, "2026-03-03", "APPROVED", ptrTo(tcAggProjectB), intPtr(240)},
	{"47666666-0000-0000-0000-000000000005", tcAggUserBob, tcAggCaseB, "2026-03-05", "REJECTED", nil, intPtr(15)},
	{"47666666-0000-0000-0000-000000000006", tcAggUserAnn, tcAggCaseA, "2026-04-10", "APPROVED", ptrTo(tcAggProjectA), intPtr(45)},
}

// seedTimeCardAggregate adds the fixture above and removes it after the test.
func seedTimeCardAggregate(t *testing.T, pool *pgxpool.Pool) (context.Context, repository.TimeCardRepository) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM time_card WHERE created_by = 'time-card-agg-test'`)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE created_by = 'time-card-agg-test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = ANY($1::uuid[])`, []string{tcAggUserAnn, tcAggUserBob})
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = ANY($1::uuid[])`, []string{tcAggProjectA, tcAggProjectB})
	}
	cleanup()
	t.Cleanup(cleanup)
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, end_date)
	          VALUES ($1, now(), now(), 'time-card-agg-test', 'time-card-agg-test', 'TCAGGA', 'sf-tcagga', 'Agg Alpha', (now() + INTERVAL '30 days')::date),
	                 ($2, now(), now(), 'time-card-agg-test', 'time-card-agg-test', 'TCAGGB', 'sf-tcaggb', NULL, (now() + INTERVAL '30 days')::date)`,
		tcAggProjectA, tcAggProjectB)
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, name, is_active)
	          VALUES ($1, now(), now(), 'ann-agg@example.com', 'ann-agg@example.com', 'Ann Agg', true),
	                 ($2, now(), now(), 'bob-agg-login', 'bob-agg@example.com', NULL, true)`,
		tcAggUserAnn, tcAggUserBob)
	for _, c := range []struct{ id, project, number string }{{tcAggCaseA, tcAggProjectA, "TCAGG01"}, {tcAggCaseB, tcAggProjectB, "TCAGG02"}} {
		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		          VALUES ($1, now(), now(), 'time-card-agg-test', 'time-card-agg-test', $2, $2, 'a case', 'CASE', $3)`,
			c.id, c.number, c.project)
		mustExec(`INSERT INTO "case" (id, state, severity) VALUES ($1, 'OPEN', 'S2')`, c.id)
	}
	for _, c := range tcAggCards {
		mustExec(`INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, customer_project_id, user_id,
		                                 work_date, is_billable, state, analyzing_minutes)
		          VALUES ($1, now(), now(), 'time-card-agg-test', 'time-card-agg-test', $2, $3, $4, $5::date, true,
		                  $6::text::time_card_state_enum, $7)`,
			c.id, c.caseID, c.project, c.user, c.date, c.state, c.minutes)
	}
	return ctx, repository.NewTimeCardRepository(scoped)
}

// tcAggFilters limits a query to the fixture's own two users, so other tests'
// time cards in the same database never count.
func tcAggFilters() *domain.SearchTimeCardsFilters {
	return &domain.SearchTimeCardsFilters{UserIDs: []string{tcAggUserAnn, tcAggUserBob}}
}

type tcAggWant struct {
	key, label     string
	count, minutes int
}

// checkTimeCardAggregate compares an aggregate with the groups wanted, in
// order, and with its "others" and total counts and minutes; every minute
// field must be present (minutes were asked for).
func checkTimeCardAggregate(t *testing.T, got domain.AggregateResponse, want []tcAggWant, othersCount, othersMinutes, total, totalMinutes int) {
	t.Helper()
	if len(got.Groups) != len(want) {
		t.Fatalf("groups = %+v, want %d groups %+v", got.Groups, len(want), want)
	}
	for i, w := range want {
		g := got.Groups[i]
		if g.Key != w.key || g.Label != w.label || g.Count != w.count || g.TotalMinutes == nil || *g.TotalMinutes != w.minutes {
			t.Errorf("group %d = {%s %q %d %s min}, want {%s %q %d %d min}",
				i, g.Key, g.Label, g.Count, tcAggMinutes(g.TotalMinutes), w.key, w.label, w.count, w.minutes)
		}
	}
	if got.OthersCount != othersCount || got.OthersMinutes == nil || *got.OthersMinutes != othersMinutes {
		t.Errorf("others = %d cards / %s min, want %d / %d", got.OthersCount, tcAggMinutes(got.OthersMinutes), othersCount, othersMinutes)
	}
	if got.TotalRecords != total || got.TotalMinutes == nil || *got.TotalMinutes != totalMinutes {
		t.Errorf("totals = %d cards / %s min, want %d / %d", got.TotalRecords, tcAggMinutes(got.TotalMinutes), total, totalMinutes)
	}
}

// tcAggMinutes prints a minute field, nil as "nil".
func tcAggMinutes(p *int) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprint(*p)
}

// TestTimeCardAggregateIntegration_ByState: keys lower-cased as the states
// filter takes them; a card with all five minute columns NULL counts as a
// card and adds 0 minutes; a count tie is broken by minutes.
func TestTimeCardAggregateIntegration_ByState(t *testing.T) {
	ctx, repo := seedTimeCardAggregate(t, caseStatsPool(t))
	got, err := repo.AggregateTimeCards(ctx, domain.AggregateTimeCardsRequest{Filters: tcAggFilters(), GroupBy: "state"}, 10)
	if err != nil {
		t.Fatalf("AggregateTimeCards: %v", err)
	}
	checkTimeCardAggregate(t, got, []tcAggWant{
		{"approved", "approved", 4, 345},
		{"submitted", "submitted", 1, 30},
		{"rejected", "rejected", 1, 15},
	}, 0, 0, 6, 390)
}

// TestTimeCardAggregateIntegration_ByUserRanksByMeasure: with a cap of one,
// ranking by count keeps Ann (4 cards) and ranking by minutes keeps Bob (255
// minutes); the other is folded into others, and the totals stay whole.
// Labels: Ann's name; Bob has none, so his email.
func TestTimeCardAggregateIntegration_ByUserRanksByMeasure(t *testing.T) {
	ctx, repo := seedTimeCardAggregate(t, caseStatsPool(t))
	ann := tcAggWant{tcAggUserAnn, "Ann Agg", 4, 135}
	bob := tcAggWant{tcAggUserBob, "bob-agg@example.com", 2, 255}

	all, err := repo.AggregateTimeCards(ctx, domain.AggregateTimeCardsRequest{Filters: tcAggFilters(), GroupBy: "user"}, 10)
	if err != nil {
		t.Fatalf("AggregateTimeCards: %v", err)
	}
	checkTimeCardAggregate(t, all, []tcAggWant{ann, bob}, 0, 0, 6, 390)

	byCount, err := repo.AggregateTimeCards(ctx, domain.AggregateTimeCardsRequest{Filters: tcAggFilters(), GroupBy: "user", Measure: "count"}, 1)
	if err != nil {
		t.Fatalf("AggregateTimeCards by count: %v", err)
	}
	checkTimeCardAggregate(t, byCount, []tcAggWant{ann}, 2, 255, 6, 390)

	byMinutes, err := repo.AggregateTimeCards(ctx, domain.AggregateTimeCardsRequest{Filters: tcAggFilters(), GroupBy: "user", Measure: "minutes"}, 1)
	if err != nil {
		t.Fatalf("AggregateTimeCards by minutes: %v", err)
	}
	checkTimeCardAggregate(t, byMinutes, []tcAggWant{bob}, 4, 135, 6, 390)
}

// TestTimeCardAggregateIntegration_ByProject: the card with no project has
// no bucket but is counted in others and the totals; project B has no name,
// so its key.
func TestTimeCardAggregateIntegration_ByProject(t *testing.T) {
	ctx, repo := seedTimeCardAggregate(t, caseStatsPool(t))
	got, err := repo.AggregateTimeCards(ctx, domain.AggregateTimeCardsRequest{Filters: tcAggFilters(), GroupBy: "project"}, 10)
	if err != nil {
		t.Fatalf("AggregateTimeCards: %v", err)
	}
	checkTimeCardAggregate(t, got, []tcAggWant{
		{tcAggProjectA, "Agg Alpha", 4, 135},
		{tcAggProjectB, "TCAGGB", 1, 240},
	}, 1, 15, 6, 390)
}

// TestTimeCardAggregateIntegration_ByDay: every day of the window in date
// order, days without cards at 0, the April card outside it left out.
func TestTimeCardAggregateIntegration_ByDay(t *testing.T) {
	ctx, repo := seedTimeCardAggregate(t, caseStatsPool(t))
	f := tcAggFilters()
	f.StartDate, f.EndDate = ptrTo("2026-03-01"), ptrTo("2026-03-07")
	got, err := repo.AggregateTimeCards(ctx, domain.AggregateTimeCardsRequest{Filters: f, GroupBy: "day"}, 2)
	if err != nil {
		t.Fatalf("AggregateTimeCards: %v", err)
	}
	day := func(d string, count, minutes int) tcAggWant { return tcAggWant{d, d, count, minutes} }
	checkTimeCardAggregate(t, got, []tcAggWant{
		day("2026-03-01", 0, 0),
		day("2026-03-02", 2, 90),
		day("2026-03-03", 1, 240),
		day("2026-03-04", 1, 0),
		day("2026-03-05", 1, 15),
		day("2026-03-06", 0, 0),
		day("2026-03-07", 0, 0),
	}, 0, 0, 5, 345)

	// The filters apply inside the window too: Bob alone leaves Ann's days at 0.
	f.UserIDs = []string{tcAggUserBob}
	bob, err := repo.AggregateTimeCards(ctx, domain.AggregateTimeCardsRequest{Filters: f, GroupBy: "day"}, 10)
	if err != nil {
		t.Fatalf("AggregateTimeCards for Bob: %v", err)
	}
	checkTimeCardAggregate(t, bob, []tcAggWant{
		day("2026-03-01", 0, 0),
		day("2026-03-02", 0, 0),
		day("2026-03-03", 1, 240),
		day("2026-03-04", 0, 0),
		day("2026-03-05", 1, 15),
		day("2026-03-06", 0, 0),
		day("2026-03-07", 0, 0),
	}, 0, 0, 2, 255)
}

// TestTimeCardAggregateIntegration_MatchesTheSearch: for the same filters
// every grouping counts exactly the cards the search returns, and sums
// exactly their totalTime (the card with no project included).
func TestTimeCardAggregateIntegration_MatchesTheSearch(t *testing.T) {
	ctx, repo := seedTimeCardAggregate(t, caseStatsPool(t))
	for name, tc := range map[string]struct {
		states       []domain.TimeCardState
		cards, total int
	}{
		"approved":  {[]domain.TimeCardState{"approved"}, 3, 300},
		"any state": {nil, 5, 345},
	} {
		f := tcAggFilters()
		f.States = tc.states
		f.StartDate, f.EndDate = ptrTo("2026-03-01"), ptrTo("2026-03-31")

		views, total, err := repo.SearchTimeCards(ctx, domain.SearchTimeCardsRequest{Filters: f, Pagination: domain.Pagination{Limit: 50}}, "")
		if err != nil {
			t.Fatalf("%s: SearchTimeCards: %v", name, err)
		}
		var searchMinutes float64
		for _, v := range views {
			searchMinutes += v.TotalTime
		}
		if total != tc.cards || searchMinutes != float64(tc.total) {
			t.Fatalf("%s: search = %d cards / %v min, want %d / %d", name, total, searchMinutes, tc.cards, tc.total)
		}
		for _, groupBy := range []string{"user", "state", "project", "day"} {
			got, err := repo.AggregateTimeCards(ctx, domain.AggregateTimeCardsRequest{Filters: f, GroupBy: groupBy}, 1)
			if err != nil {
				t.Fatalf("%s: AggregateTimeCards by %s: %v", name, groupBy, err)
			}
			if got.TotalRecords != total || got.TotalMinutes == nil || float64(*got.TotalMinutes) != searchMinutes {
				t.Errorf("%s by %s: aggregate = %d cards / %s min, want the search's %d / %v",
					name, groupBy, got.TotalRecords, tcAggMinutes(got.TotalMinutes), total, searchMinutes)
			}
		}
	}
}
