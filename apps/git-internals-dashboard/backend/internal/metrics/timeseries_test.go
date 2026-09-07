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

func findSeries(series []Series, key string) *Series {
	for i := range series {
		if series[i].Key == key {
			return &series[i]
		}
	}
	return nil
}

func TestBuildTimeseriesGroupByPriorityGapFillsAndPreseedsAllTiers(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	ts, err := BuildTimeseries(context.Background(), pool, metricsTestConfig, &repoFilter, 7, "priority", "violated")
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

func TestBuildTimeseriesMetricAtRisk(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	ts, err := BuildTimeseries(context.Background(), pool, metricsTestConfig, &repoFilter, 7, "priority", "at_risk")
	if err != nil {
		t.Fatalf("BuildTimeseries: %v", err)
	}
	p2 := findSeries(ts.Series, "High(P2)")
	if p2 == nil || p2.Points[6] != 1 {
		t.Fatalf("expected High(P2) at_risk today=1, got %+v", p2)
	}
}

func TestBuildTimeseriesGroupByNoneUsesSingleAllSeries(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	ts, err := BuildTimeseries(context.Background(), pool, metricsTestConfig, &repoFilter, 7, "none", "total")
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
