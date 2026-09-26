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

package metrics

import (
	"context"
	"testing"
)

// findSeries returns the series matching key, or nil if absent.
func findSeries(series []Series, key string) *Series {
	for i := range series {
		if series[i].Key == key {
			return &series[i]
		}
	}
	return nil
}

// TestBuildTimeseriesGroupByPriorityGapFillsAndPreseedsAllTiers verifies
// groupBy=priority pre-seeds all 3 canonical tiers (even ones with no
// matching data) and gap-fills missing dates with zero.
func TestBuildTimeseriesGroupByPriorityGapFillsAndPreseedsAllTiers(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	ts, err := BuildTimeseries(context.Background(), pool, metricsTestConfig, Filter{Repo: &repoFilter}, 7, "priority", "violated")
	if err != nil {
		t.Fatalf("BuildTimeseries: %v", err)
	}

	if ts.Window != 7 || ts.Metric != "violated" || ts.GroupBy != "priority" {
		t.Errorf("unexpected header fields: %+v", struct {
			Window  int
			Metric  string
			GroupBy string
		}{ts.Window, ts.Metric, ts.GroupBy})
	}
	if len(ts.Dates) != 7 {
		t.Fatalf("expected 7 dates, got %d", len(ts.Dates))
	}

	// All 3 canonical tiers must appear even though only P1 has VIOLATED data.
	if len(ts.Series) != 3 {
		t.Fatalf("expected 3 pre-seeded priority series, got %d: %+v", len(ts.Series), ts.Series)
	}
	// P1 -> P2 -> P3 ordering.
	wantOrder := []string{"P1", "P2", "P3"}
	for i, code := range wantOrder {
		if ts.Series[i].Label != code {
			t.Errorf("expected series[%d].label=%s, got %s", i, code, ts.Series[i].Label)
		}
	}

	p1 := findSeries(ts.Series, "Critical(P1)")
	if p1 == nil {
		t.Fatalf("expected a Critical(P1) series, got %+v", ts.Series)
	}
	if len(p1.Points) != 7 {
		t.Fatalf("expected 7 points, got %d", len(p1.Points))
	}
	if p1.Points[6] != 1 { // today = last point in the window
		t.Errorf("expected today's point=1 for Critical(P1) violated, got %d", p1.Points[6])
	}
	for i := 0; i < 6; i++ {
		if p1.Points[i] != 0 {
			t.Errorf("expected gap-filled zero at points[%d], got %d", i, p1.Points[i])
		}
	}

	p2 := findSeries(ts.Series, "High(P2)")
	if p2 == nil || p2.Points[6] != 0 {
		t.Errorf("expected High(P2) violated series to be all zero (it's AT_RISK, not VIOLATED), got %+v", p2)
	}
}

// TestBuildTimeseriesMetricAtRisk verifies metric=at_risk counts the
// fixture's AT_RISK issue on its own tier's series.
func TestBuildTimeseriesMetricAtRisk(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	ts, err := BuildTimeseries(context.Background(), pool, metricsTestConfig, Filter{Repo: &repoFilter}, 7, "priority", "at_risk")
	if err != nil {
		t.Fatalf("BuildTimeseries: %v", err)
	}
	p2 := findSeries(ts.Series, "High(P2)")
	if p2 == nil || p2.Points[6] != 1 {
		t.Fatalf("expected High(P2) at_risk today=1, got %+v", p2)
	}
}

// TestBuildTimeseriesGroupByNoneUsesSingleAllSeries verifies groupBy=none
// collapses every priority into one "all" series.
func TestBuildTimeseriesGroupByNoneUsesSingleAllSeries(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	ts, err := BuildTimeseries(context.Background(), pool, metricsTestConfig, Filter{Repo: &repoFilter}, 7, "none", "total")
	if err != nil {
		t.Fatalf("BuildTimeseries: %v", err)
	}
	if len(ts.Series) != 1 || ts.Series[0].Key != "all" {
		t.Fatalf("expected a single 'all' series, got %+v", ts.Series)
	}
	if ts.Series[0].Points[6] != 3 { // all 3 issues are tracked (priority IS NOT NULL)
		t.Errorf("expected today's total=3, got %d", ts.Series[0].Points[6])
	}
}

// TestBuildTimeseriesAbtTeamFilterNarrowsSeries verifies an abtTeam filter
// narrows the series to that team's issues, leaving the same query
// unfiltered as the baseline it must differ from.
func TestBuildTimeseriesAbtTeamFilterNarrowsSeries(t *testing.T) {
	pool := testPool(t)
	seedAbtTeamFixture(t, pool)
	ctx := context.Background()
	repoFilter := abtFixtureRepo

	// Unfiltered, the repo's P1 VIOLATED series counts both the Alpha issue
	// and the team-less one.
	all, err := BuildTimeseries(ctx, pool, metricsTestConfig, Filter{Repo: &repoFilter}, 7, "priority", "violated")
	if err != nil {
		t.Fatalf("BuildTimeseries: %v", err)
	}
	p1 := findSeries(all.Series, "Critical(P1)")
	if p1 == nil || p1.Points[6] != 2 {
		t.Fatalf("expected an unfiltered Critical(P1) violated today=2, got %+v", p1)
	}

	cases := []struct {
		name  string
		team  string
		key   string
		today int
	}{
		{name: "alpha owns one violated P1", team: abtTeamAlpha, key: "Critical(P1)", today: 1},
		{name: "beta owns none", team: abtTeamBeta, key: "Critical(P1)", today: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			team := tc.team
			ts, err := BuildTimeseries(ctx, pool, metricsTestConfig,
				Filter{Repo: &repoFilter, AbtTeam: &team}, 7, "priority", "violated")
			if err != nil {
				t.Fatalf("BuildTimeseries: %v", err)
			}
			s := findSeries(ts.Series, tc.key)
			if s == nil {
				t.Fatalf("expected a %s series, got %+v", tc.key, ts.Series)
			}
			if s.Points[6] != tc.today {
				t.Errorf("expected today's %s violated=%d for %s, got %d", tc.key, tc.today, tc.team, s.Points[6])
			}
		})
	}
}

// TestBuildTimeseriesAbtTeamFilterAppliesToGroupByNone verifies the abtTeam
// filter reaches the ungrouped "all" series too, where the metric counts
// every tracked issue rather than one sla_state.
func TestBuildTimeseriesAbtTeamFilterAppliesToGroupByNone(t *testing.T) {
	pool := testPool(t)
	seedAbtTeamFixture(t, pool)
	repoFilter := abtFixtureRepo
	team := abtTeamAlpha

	ts, err := BuildTimeseries(context.Background(), pool, metricsTestConfig,
		Filter{Repo: &repoFilter, AbtTeam: &team}, 7, "none", "total")
	if err != nil {
		t.Fatalf("BuildTimeseries: %v", err)
	}
	if len(ts.Series) != 1 || ts.Series[0].Key != "all" {
		t.Fatalf("expected a single 'all' series, got %+v", ts.Series)
	}
	if ts.Series[0].Points[6] != 2 { // Alpha's 2 tracked open issues
		t.Errorf("expected today's total=2 for %s, got %d", team, ts.Series[0].Points[6])
	}
}
