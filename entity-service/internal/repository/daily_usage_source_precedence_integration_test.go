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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// seedOverlappingSources adds, on top of seedCoreNodes (which gives node B a live
// CORES row of 8 on 1 June):
//   - node B on 1 June -- a connected customer who also uploaded a report covering a
//     day the product already published: an uploaded CORES row of 12, TRANSACTION_COUNT
//     1000 live and 400 uploaded, and USERS 20 live beside a row of 99 with no
//     data_source (nullable since migration 0054). The values differ so picking the
//     wrong row shows;
//   - node C: only uploaded rows on 4 June (CORES 5, TRANSACTION_COUNT 300);
//   - node A: on 2 June a live USERS row with no value and an uploaded one of 7.
func seedOverlappingSources(t *testing.T, ctx context.Context, exec func(context.Context, string, ...any) error) {
	t.Helper()
	seedCoreNodes(t, ctx, exec)
	stmt := `INSERT INTO daily_usage_summary (id, created_on, updated_on, created_by, updated_by, deployment_node_id, deployment_number, summary_date, count_type, value, data_source)
		SELECT gen_random_uuid(), NOW(), NOW(), 't', 't', v.node::uuid, 'DEP-OLD', v.day::date, v.type, v.value, v.source::usage_data_source_enum
		FROM (VALUES
			('` + coresNodeB + `', '2026-06-01', 'CORES',             12,            'FILE_UPLOAD'),
			('` + coresNodeB + `', '2026-06-01', 'TRANSACTION_COUNT', 1000,          'API_CALL'),
			('` + coresNodeB + `', '2026-06-01', 'TRANSACTION_COUNT', 400,           'FILE_UPLOAD'),
			('` + coresNodeB + `', '2026-06-01', 'USERS',             20,            'API_CALL'),
			('` + coresNodeB + `', '2026-06-01', 'USERS',             99,            NULL),
			('` + coresNodeC + `', '2026-06-04', 'CORES',             5,             'FILE_UPLOAD'),
			('` + coresNodeC + `', '2026-06-04', 'TRANSACTION_COUNT', 300,           'FILE_UPLOAD'),
			('` + coresNodeA + `', '2026-06-02', 'USERS',             NULL::integer, 'API_CALL'),
			('` + coresNodeA + `', '2026-06-02', 'USERS',             7,             'FILE_UPLOAD')
		) AS v(node, day, type, value, source)`
	if err := exec(ctx, stmt); err != nil {
		t.Fatalf("seed overlapping sources: %v", err)
	}
}

func overlapPool(t *testing.T) (context.Context, *repository.Scoped) {
	t.Helper()
	pool := instanceIngestionTestPool(t)
	ctx := internalCtx()
	seedOverlappingSources(t, ctx, func(c context.Context, q string, a ...any) error {
		_, err := pool.Exec(c, q, a...)
		return err
	})
	return ctx, repository.NewScoped(pool)
}

// A day a node has both a live and an uploaded CORES row counts the node once, as
// the monthly rollup does, rather than adding the two. An upload alone still counts.
func TestSearchDeployedProductMetrics_LiveAndUploadedCoresCountOnce(t *testing.T) {
	ctx, db := overlapPool(t)
	got, err := repository.NewDeployedProductRepository(db).
		SearchDeployedProductMetrics(ctx, coresDeployedProduct, coresDeploymentID, "2026-06-01", "2026-06-30")
	if err != nil {
		t.Fatalf("SearchDeployedProductMetrics: %v", err)
	}
	want := map[string]struct{ total, count int }{
		"2026-06-01": {14, 2}, // A: 6 from its snapshot, B: its live 8 (not the uploaded 12, nor 20)
		"2026-06-04": {5, 1},  // C: uploaded only
	}
	seen := map[string]bool{}
	for _, e := range got.ChartData {
		w, ok := want[e.Date]
		if !ok {
			continue
		}
		seen[e.Date] = true
		if e.TotalCores != w.total || e.InstanceCount != w.count {
			t.Errorf("%s: totalCores=%d instanceCount=%d, want %d and %d", e.Date, e.TotalCores, e.InstanceCount, w.total, w.count)
		}
	}
	for d := range want {
		if !seen[d] {
			t.Errorf("chart has no %s: %+v", d, got.ChartData)
		}
	}
}

func TestSearchDeployedProductUsageCounts_LiveWinsOverUploadForTheSameDay(t *testing.T) {
	ctx, db := overlapPool(t)
	got, err := repository.NewDeployedProductRepository(db).
		SearchDeployedProductUsageCounts(ctx, coresDeployedProduct, coresDeploymentID, "2026-06-01", "2026-06-30")
	if err != nil {
		t.Fatalf("SearchDeployedProductUsageCounts: %v", err)
	}
	want := map[string]map[string]float64{
		"2026-06-01": {"CORES": 8, "TRANSACTION_COUNT": 1000, "USERS": 20}, // live, over uploaded and no-source rows
		"2026-06-02": {"USERS": 7},                                         // the uploaded value, not the live row that has none
		"2026-06-04": {"CORES": 5, "TRANSACTION_COUNT": 300},
	}
	byDate := map[string]map[string]float64{}
	for _, e := range got.ChartData {
		byDate[e.Date] = map[string]float64{}
		for ct, c := range e.Counts {
			byDate[e.Date][ct] = c.Value
			if len(c.Instances) != 1 {
				t.Errorf("%s %s: %d instances, want 1 (one node, counted once)", e.Date, ct, len(c.Instances))
			}
		}
	}
	for d, types := range want {
		for ct, v := range types {
			if byDate[d][ct] != v {
				t.Errorf("%s %s = %v, want %v", d, ct, byDate[d][ct], v)
			}
		}
	}
	if agg := got.Summary.CountTypes["TRANSACTION_COUNT"]; agg.Max != 1000 {
		t.Errorf("TRANSACTION_COUNT summary max = %v, want 1000", agg.Max)
	}
}

func TestSearchInstanceUsageStats_LiveWinsOverUploadForTheSameDay(t *testing.T) {
	ctx, db := overlapPool(t)
	repo := repository.NewInstanceRepository(db)
	filters := domain.InstanceDateRangeFilters{
		StartDate: "2026-06-01", EndDate: "2026-06-30", DeployedProductIDs: []string{coresDeployedProduct},
	}

	got, err := repo.SearchInstanceUsageStats(ctx, filters, nil)
	if err != nil {
		t.Fatalf("SearchInstanceUsageStats: %v", err)
	}
	for _, c := range []struct {
		day, countType string
		want           int
	}{
		{"2026-06-01", "TRANSACTION_COUNT", 1000},
		{"2026-06-01", "CORES", 8},
		{"2026-06-01", "USERS", 20},
		{"2026-06-02", "USERS", 7},
		{"2026-06-04", "TRANSACTION_COUNT", 300},
	} {
		if v := got.Stats[c.day][c.countType]; v != c.want {
			t.Errorf("all sources: %s %s = %d, want %d", c.day, c.countType, v, c.want)
		}
	}

	// Asking for one source still returns that source's rows, whatever the other holds.
	upload := "FILE_UPLOAD"
	uploaded, err := repo.SearchInstanceUsageStats(ctx, filters, &upload)
	if err != nil {
		t.Fatalf("SearchInstanceUsageStats(FILE_UPLOAD): %v", err)
	}
	if v := uploaded.Stats["2026-06-01"]["TRANSACTION_COUNT"]; v != 400 {
		t.Errorf("FILE_UPLOAD: 2026-06-01 TRANSACTION_COUNT = %d, want the uploaded 400", v)
	}
	if v := uploaded.Stats["2026-06-01"]["CORES"]; v != 12 {
		t.Errorf("FILE_UPLOAD: 2026-06-01 CORES = %d, want the uploaded 12", v)
	}
	live := "API_CALL"
	lived, err := repo.SearchInstanceUsageStats(ctx, filters, &live)
	if err != nil {
		t.Fatalf("SearchInstanceUsageStats(API_CALL): %v", err)
	}
	if _, ok := lived.Stats["2026-06-04"]; ok {
		t.Errorf("API_CALL: 2026-06-04 has only uploaded rows, got %v", lived.Stats["2026-06-04"])
	}
}

// The monthly rollup makes the same choice as the daily reads: the live row, over an
// uploaded one and over one with no data_source (migration 0250).
func TestRollupMonthlyUsage_LiveWinsOverUploadAndNoSourceRows(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	ctx := internalCtx()
	seedOverlappingSources(t, ctx, func(c context.Context, q string, a ...any) error {
		_, err := pool.Exec(c, q, a...)
		return err
	})
	if _, err := pool.Exec(ctx, `SELECT rollup_monthly_usage(2026, 6)`); err != nil {
		t.Fatalf("rollup_monthly_usage: %v", err)
	}
	for _, c := range []struct {
		node, countType string
		want            int
	}{
		{"core-node-b", "TRANSACTION_COUNT", 1000}, // not the uploaded 400
		{"core-node-b", "USERS", 20},               // not the no-source 99
		{"core-node-b", "CORES", 8},                // not the uploaded 12
		{"core-node-c", "TRANSACTION_COUNT", 300},  // uploaded only
		{"core-node-a", "USERS", 7},                // the live row has no value
	} {
		var got int
		if err := pool.QueryRow(ctx, `SELECT count FROM monthly_usage_summary
			WHERE node_id = $1 AND count_type = $2 AND year = 2026 AND month = 6`, c.node, c.countType).Scan(&got); err != nil {
			t.Errorf("%s %s: %v", c.node, c.countType, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s %s = %d, want %d", c.node, c.countType, got, c.want)
		}
	}
}
