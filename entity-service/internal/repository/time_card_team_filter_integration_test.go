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

// Integration tests for the time card search's and aggregate's teamIds
// filter and the aggregate's "team" grouping, over the aggregate's own
// fixture (time_card_aggregate_integration_test.go):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run TimeCardTeam

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	tcTeamAnn   = "47777777-0000-0000-0000-00000000000a" // Ann, twice (as the Team Schedule can list a member)
	tcTeamBob   = "47777777-0000-0000-0000-00000000000b" // Bob
	tcTeamBoth  = "47777777-0000-0000-0000-00000000000c" // Ann and Bob
	tcTeamEmpty = "47777777-0000-0000-0000-00000000000d" // nobody
)

// seedTimeCardTeams adds the aggregate's fixture and four teams over its two
// engineers ("Time Card Team A" to "D", in the order above), removed after
// the test.
func seedTimeCardTeams(t *testing.T) (context.Context, repository.TimeCardRepository) {
	t.Helper()
	pool := caseStatsPool(t)
	ctx, repo := seedTimeCardAggregate(t, pool)
	teams := []string{tcTeamAnn, tcTeamBob, tcTeamBoth, tcTeamEmpty}
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM team WHERE id = ANY($1::uuid[])`, teams)
	}
	cleanup()
	t.Cleanup(cleanup)
	for i, id := range teams {
		if _, err := pool.Exec(ctx, `INSERT INTO team (id, created_on, updated_on, name, type) VALUES ($1, now(), now(), $2, 'cre-abt')`,
			id, "Time Card Team "+string(rune('A'+i))); err != nil {
			t.Fatalf("seed team: %v", err)
		}
	}
	for _, m := range [][2]string{{tcTeamAnn, tcAggUserAnn}, {tcTeamAnn, tcAggUserAnn}, {tcTeamBob, tcAggUserBob}, {tcTeamBoth, tcAggUserAnn}, {tcTeamBoth, tcAggUserBob}} {
		if _, err := pool.Exec(ctx, `INSERT INTO team_member (id, created_on, updated_on, team_id, user_id) VALUES (gen_random_uuid(), now(), now(), $1, $2)`,
			m[0], m[1]); err != nil {
			t.Fatalf("seed team member: %v", err)
		}
	}
	return ctx, repo
}

// TestTimeCardTeamFilterIntegration_MatchesTheTeamsMembers: a card matches
// when its engineer is a member of any of the teams, each card once however
// many of those teams (or rows of one team) list them; the search and every
// aggregate grouping agree.
func TestTimeCardTeamFilterIntegration_MatchesTheTeamsMembers(t *testing.T) {
	ctx, repo := seedTimeCardTeams(t)

	// March: Ann 3 cards / 90 min, Bob 2 cards / 255 min.
	for _, tc := range []struct {
		name           string
		teamIDs        []string
		userIDs        []string
		cards, minutes int
	}{
		{"one team, its member listed twice", []string{tcTeamAnn}, nil, 3, 90},
		{"two teams", []string{tcTeamAnn, tcTeamBob}, nil, 5, 345},
		{"one team with both", []string{tcTeamBoth}, nil, 5, 345},
		{"a member in two of the teams counts once", []string{tcTeamAnn, tcTeamBoth}, nil, 5, 345},
		{"a team with no members", []string{tcTeamEmpty}, nil, 0, 0},
		{"with the user filter: both must hold", []string{tcTeamAnn}, []string{tcAggUserBob}, 0, 0},
		{"with the user filter: both hold", []string{tcTeamBoth}, []string{tcAggUserBob}, 2, 255},
	} {
		f := tcAggFilters()
		if tc.userIDs != nil {
			f.UserIDs = tc.userIDs
		}
		f.TeamIDs = tc.teamIDs
		f.StartDate, f.EndDate = ptrTo("2026-03-01"), ptrTo("2026-03-31")

		views, total, err := repo.SearchTimeCards(ctx, domain.SearchTimeCardsRequest{Filters: f, Pagination: domain.Pagination{Limit: 50}}, "")
		if err != nil {
			t.Fatalf("%s: SearchTimeCards: %v", tc.name, err)
		}
		var minutes float64
		for _, v := range views {
			minutes += v.TotalTime
		}
		if total != tc.cards || len(views) != tc.cards || minutes != float64(tc.minutes) {
			t.Errorf("%s: search = %d cards (%d rows) / %v min, want %d / %d", tc.name, total, len(views), minutes, tc.cards, tc.minutes)
		}
		for _, groupBy := range []string{"user", "state", "project", "day"} {
			got, err := repo.AggregateTimeCards(ctx, domain.AggregateTimeCardsRequest{Filters: f, GroupBy: groupBy}, 10)
			if err != nil {
				t.Fatalf("%s: AggregateTimeCards by %s: %v", tc.name, groupBy, err)
			}
			if got.TotalRecords != tc.cards || got.TotalMinutes == nil || *got.TotalMinutes != tc.minutes {
				t.Errorf("%s by %s: aggregate = %d cards / %s min, want %d / %d",
					tc.name, groupBy, got.TotalRecords, tcAggMinutes(got.TotalMinutes), tc.cards, tc.minutes)
			}
		}
	}
}

// TestTimeCardTeamGroupingIntegration: one bucket per team asked for, its
// current members' cards, a team with none at zero; a card counts in every
// one of those teams its engineer is in, so the totals are the buckets' sums.
func TestTimeCardTeamGroupingIntegration(t *testing.T) {
	ctx, repo := seedTimeCardTeams(t)
	all := []string{tcTeamAnn, tcTeamBob, tcTeamBoth, tcTeamEmpty}
	ann := tcAggWant{tcTeamAnn, "Time Card Team A", 3, 90}
	bob := tcAggWant{tcTeamBob, "Time Card Team B", 2, 255}
	both := tcAggWant{tcTeamBoth, "Time Card Team C", 5, 345}
	empty := tcAggWant{tcTeamEmpty, "Time Card Team D", 0, 0}
	for _, tc := range []struct {
		name                                      string
		teamIDs, userIDs                          []string
		measure                                   string
		maxGroups                                 int
		want                                      []tcAggWant
		othersCount, othersMinutes, total, total2 int
	}{
		{"by count, an empty team at zero", all, nil, "count", 10, []tcAggWant{both, ann, bob, empty}, 0, 0, 10, 690},
		{"by minutes", all, nil, "minutes", 10, []tcAggWant{both, bob, ann, empty}, 0, 0, 10, 690},
		{"the rest in others", all, nil, "count", 2, []tcAggWant{both, ann}, 2, 255, 10, 690},
		{"only the teams asked for", []string{tcTeamBob, tcTeamAnn}, nil, "count", 10, []tcAggWant{ann, bob}, 0, 0, 5, 345},
		{"the search's filters apply", []string{tcTeamAnn, tcTeamBoth}, []string{tcAggUserBob}, "count", 10,
			[]tcAggWant{{tcTeamBoth, "Time Card Team C", 2, 255}, {tcTeamAnn, "Time Card Team A", 0, 0}}, 0, 0, 2, 255},
		{"a team id with no team", []string{tcTeamAnn, "47777777-0000-0000-0000-0000000000ff"}, nil, "count", 10, []tcAggWant{ann}, 0, 0, 3, 90},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tcAggFilters()
			if tc.userIDs != nil {
				f.UserIDs = tc.userIDs
			}
			f.TeamIDs = tc.teamIDs
			f.StartDate, f.EndDate = ptrTo("2026-03-01"), ptrTo("2026-03-31")
			got, err := repo.AggregateTimeCards(ctx, domain.AggregateTimeCardsRequest{Filters: f, GroupBy: "team", Measure: tc.measure}, tc.maxGroups)
			if err != nil {
				t.Fatalf("AggregateTimeCards: %v", err)
			}
			checkTimeCardAggregate(t, got, tc.want, tc.othersCount, tc.othersMinutes, tc.total, tc.total2)
		})
	}
}
