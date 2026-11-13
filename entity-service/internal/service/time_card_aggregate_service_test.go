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

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// aggregateTimeCardsService returns a timeCardService whose repository
// records the request and cap it was called with; called reports whether it
// was called at all.
func aggregateTimeCardsService() (svc TimeCardService, got *domain.AggregateTimeCardsRequest, gotMax *int, called *bool) {
	got, gotMax, called = &domain.AggregateTimeCardsRequest{}, new(int), new(bool)
	repo := &stubTimeCardRepo{
		aggregateTimeCards: func(_ context.Context, req domain.AggregateTimeCardsRequest, maxGroups int) (domain.AggregateResponse, error) {
			*got, *gotMax, *called = req, maxGroups, true
			return domain.AggregateResponse{}, nil
		},
	}
	return NewTimeCardService(repo, stubUserRepo{}), got, gotMax, called
}

func aggregateDates(start, end string) *domain.SearchTimeCardsFilters {
	return &domain.SearchTimeCardsFilters{StartDate: &start, EndDate: &end}
}

// TestTimeCardService_AggregateTimeCards_RefusesBadRequests: every refusal
// is a 400 with its own message, and none reaches the repository.
func TestTimeCardService_AggregateTimeCards_RefusesBadRequests(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	bad := "not-a-uuid"
	for _, tc := range []struct {
		name string
		req  domain.AggregateTimeCardsRequest
		want string
	}{
		{"no groupBy", domain.AggregateTimeCardsRequest{}, "groupBy is required"},
		{"unknown groupBy", domain.AggregateTimeCardsRequest{GroupBy: "approver"}, "groupBy contains invalid value: approver"},
		{"unknown measure", domain.AggregateTimeCardsRequest{GroupBy: "user", Measure: "hours"}, "measure contains invalid value: hours"},
		{"negative maxGroups", domain.AggregateTimeCardsRequest{GroupBy: "user", MaxGroups: -1}, "maxGroups must not be negative"},
		{"a search filter the search refuses", domain.AggregateTimeCardsRequest{GroupBy: "user",
			Filters: &domain.SearchTimeCardsFilters{UserID: &bad}}, "filters"},
		{"a team id that is not a UUID", domain.AggregateTimeCardsRequest{GroupBy: "user",
			Filters: &domain.SearchTimeCardsFilters{TeamIDs: []string{"cc6da6e6-e58b-6bd3-271d-d5e5e6e23a5c", bad}}}, "filters contains invalid UUID"},
		{"team without teams", domain.AggregateTimeCardsRequest{GroupBy: "team"}, "groupBy team requires filters.teamIds"},
		{"team with an empty team list", domain.AggregateTimeCardsRequest{GroupBy: "team",
			Filters: &domain.SearchTimeCardsFilters{TeamIDs: []string{}}}, "groupBy team requires filters.teamIds"},
		{"more than 100 teams", domain.AggregateTimeCardsRequest{GroupBy: "team",
			Filters: &domain.SearchTimeCardsFilters{TeamIDs: teamIDs(101)}}, "teamIds must not contain more than 100 values"},
		{"day without dates", domain.AggregateTimeCardsRequest{GroupBy: "day"}, "groupBy day requires filters.startDate and filters.endDate"},
		{"day with only a start", domain.AggregateTimeCardsRequest{GroupBy: "day",
			Filters: &domain.SearchTimeCardsFilters{StartDate: ptr("2026-10-01")}}, "groupBy day requires filters.startDate and filters.endDate"},
		{"day ending before it starts", domain.AggregateTimeCardsRequest{GroupBy: "day",
			Filters: aggregateDates("2026-10-10", "2026-10-09")}, "endDate must not be before startDate"},
		{"day over 31 days", domain.AggregateTimeCardsRequest{GroupBy: "day",
			Filters: aggregateDates("2026-10-01", "2026-11-01")}, "groupBy day covers at most 31 days"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, called := aggregateTimeCardsService()
			_, err := svc.AggregateTimeCards(ctx, tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one containing %q", err, tc.want)
			}
			if *called {
				t.Error("the repository was called for a refused request")
			}
		})
	}
}

// TestTimeCardService_AggregateTimeCards_PassesGoodRequests covers the edges
// that must still pass (a 31-day window, a one-day window) and the default
// cap.
func TestTimeCardService_AggregateTimeCards_PassesGoodRequests(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	for _, tc := range []struct {
		name    string
		req     domain.AggregateTimeCardsRequest
		wantMax int
	}{
		{"user, default cap", domain.AggregateTimeCardsRequest{GroupBy: "user"}, 10},
		{"project ranked by minutes, own cap", domain.AggregateTimeCardsRequest{GroupBy: "project", Measure: "minutes", MaxGroups: 3}, 3},
		{"state ranked by count", domain.AggregateTimeCardsRequest{GroupBy: "state", Measure: "count"}, 10},
		{"day, 31 days", domain.AggregateTimeCardsRequest{GroupBy: "day", Filters: aggregateDates("2026-10-01", "2026-10-31")}, 10},
		{"day, one day", domain.AggregateTimeCardsRequest{GroupBy: "day", Filters: aggregateDates("2026-10-05", "2026-10-05")}, 10},
		{"team, ranked by minutes, own cap", domain.AggregateTimeCardsRequest{GroupBy: "team", Measure: "minutes", MaxGroups: 4,
			Filters: &domain.SearchTimeCardsFilters{TeamIDs: []string{"cc6da6e6-e58b-6bd3-271d-d5e5e6e23a5c"}}}, 4},
		{"team, default: every team named, past the usual cap of 10", domain.AggregateTimeCardsRequest{GroupBy: "team",
			Filters: &domain.SearchTimeCardsFilters{TeamIDs: teamIDs(12)}}, 12},
		{"team, 100 teams", domain.AggregateTimeCardsRequest{GroupBy: "team",
			Filters: &domain.SearchTimeCardsFilters{TeamIDs: teamIDs(100)}}, 100},
		{"user, in two teams", domain.AggregateTimeCardsRequest{GroupBy: "user",
			Filters: &domain.SearchTimeCardsFilters{TeamIDs: []string{"cc6da6e6-e58b-6bd3-271d-d5e5e6e23a5c", "ecd25d38-81f4-e11a-9d7b-86ab8976c426"}}}, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, got, gotMax, called := aggregateTimeCardsService()
			if _, err := svc.AggregateTimeCards(ctx, tc.req); err != nil {
				t.Fatalf("error = %v, want none", err)
			}
			if !*called || got.GroupBy != tc.req.GroupBy || *gotMax != tc.wantMax {
				t.Errorf("repository called = %v with groupBy %q, cap %d; want groupBy %q, cap %d",
					*called, got.GroupBy, *gotMax, tc.req.GroupBy, tc.wantMax)
			}
		})
	}
}

// TestTimeCardService_AggregateTimeCards_NeedsACaller: the same minimum bar
// as the search, no token, no query.
func TestTimeCardService_AggregateTimeCards_NeedsACaller(t *testing.T) {
	svc, _, _, called := aggregateTimeCardsService()
	_, err := svc.AggregateTimeCards(context.Background(), domain.AggregateTimeCardsRequest{GroupBy: "user"})
	if err == nil || !strings.Contains(err.Error(), "x-user-id-token") {
		t.Fatalf("error = %v, want the missing-token refusal", err)
	}
	if *called {
		t.Error("the repository was called without a caller")
	}
}

// TestSNTimeCardService_SearchTimeCards_RefusesTeamIDs: ServiceNow has no
// team filter, and dropping it would return every engineer's cards.
func TestSNTimeCardService_SearchTimeCards_RefusesTeamIDs(t *testing.T) {
	_, err := buildTimeCardSearchPayload(domain.SearchTimeCardsRequest{Filters: &domain.SearchTimeCardsFilters{
		TeamIDs: []string{"cc6da6e6-e58b-6bd3-271d-d5e5e6e23a5c"}}})
	if err == nil || !strings.Contains(err.Error(), "filters.teamIds is not supported when DATA_SOURCE=servicenow") {
		t.Fatalf("error = %v, want the servicenow refusal", err)
	}
	if _, err := buildTimeCardSearchPayload(domain.SearchTimeCardsRequest{Filters: &domain.SearchTimeCardsFilters{}}); err != nil {
		t.Fatalf("no team filter: error = %v, want none", err)
	}
}

// TestSNTimeCardService_AggregateTimeCards_IsRefused: DATA_SOURCE=servicenow
// has no time card aggregate.
func TestSNTimeCardService_AggregateTimeCards_IsRefused(t *testing.T) {
	_, err := (&snTimeCardService{}).AggregateTimeCards(context.Background(), domain.AggregateTimeCardsRequest{GroupBy: "user"})
	if err == nil || !strings.Contains(err.Error(), "not supported when DATA_SOURCE=servicenow") {
		t.Fatalf("error = %v, want the servicenow refusal", err)
	}
}

// teamIDs is n distinct well-formed team ids.
func teamIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1)
	}
	return ids
}
