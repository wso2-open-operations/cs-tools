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

// Port of v3's src/server/lib/timeseries.ts, section by section.
package metrics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Series is one line on the timeseries chart.
type Series struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Points []int  `json:"points"`
}

// Timeseries is the wire shape GET /metrics/timeseries returns (SPEC §6.7).
type Timeseries struct {
	Window  int      `json:"window"`
	Metric  string   `json:"metric"`
	GroupBy string   `json:"groupBy"`
	Dates   []string `json:"dates"`
	Series  []Series `json:"series"`
}

type timeseriesRawRow struct {
	SnapshotDate time.Time
	Priority     *string
	N            int
}

// BuildTimeseries builds the /metrics/timeseries response for repo (optional
// "owner/name"), a days window, groupBy ("priority" | "none"), and metric
// ("violated" | "at_risk" | "total").
func BuildTimeseries(ctx context.Context, pool *pgxpool.Pool, cfg *config.AppConfig, repo *string, days int, groupBy, metric string) (Timeseries, error) {
	sql := `
		SELECT
			s.snapshot_date,
	`
	if groupBy == "priority" {
		sql += `s.priority,`
	} else {
		sql += `NULL::text AS priority,`
	}
	sql += `
			COUNT(*)::int AS n
		FROM sla_snapshots s
		JOIN repositories rep ON rep.id = s.repository_id
		WHERE s.snapshot_date >= (now() AT TIME ZONE 'UTC')::date - $1::int
		  AND rep.enabled = true
	`
	args := []any{days - 1}

	if repo != nil {
		owner, name, _ := strings.Cut(*repo, "/")
		args = append(args, owner, name)
		sql += fmt.Sprintf(" AND rep.owner = $%d AND rep.name = $%d", len(args)-1, len(args))
	}

	switch metric {
	case "violated":
		sql += " AND s.sla_state = 'VIOLATED'"
	case "at_risk":
		sql += " AND s.sla_state = 'AT_RISK'"
	default: // "total" = tracked only
		sql += " AND s.priority IS NOT NULL"
	}

	sql += " GROUP BY s.snapshot_date"
	if groupBy == "priority" {
		sql += ", s.priority"
	}
	sql += " ORDER BY s.snapshot_date"

	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return Timeseries{}, fmt.Errorf("metrics: timeseries query: %w", err)
	}
	defer rows.Close()
	var raw []timeseriesRawRow
	for rows.Next() {
		var r timeseriesRawRow
		if err := rows.Scan(&r.SnapshotDate, &r.Priority, &r.N); err != nil {
			return Timeseries{}, fmt.Errorf("metrics: scan timeseries row: %w", err)
		}
		raw = append(raw, r)
	}
	if err := rows.Err(); err != nil {
		return Timeseries{}, fmt.Errorf("metrics: iterate timeseries rows: %w", err)
	}

	var budgetPriorities []string
	if groupBy == "priority" {
		budgets := make([]config.BudgetEntry, len(cfg.Budgets))
		copy(budgets, cfg.Budgets)
		// Sorted by rank ascending, matching v3's [...budgets].sort((a,b) => a.rank-b.rank).
		sort.SliceStable(budgets, func(i, j int) bool { return budgets[i].Rank < budgets[j].Rank })
		for _, b := range budgets {
			budgetPriorities = append(budgetPriorities, b.Priority)
		}
	}

	// Complete date range for the window (no gaps for days with no data).
	dates := make([]string, days)
	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	for i := 0; i < days; i++ {
		dates[i] = today.AddDate(0, 0, -(days - 1 - i)).Format("2006-01-02")
	}
	dateIndex := make(map[string]int, len(dates))
	for i, d := range dates {
		dateIndex[d] = i
	}

	// Pre-seed all series keys so every priority appears even with zero data.
	seriesOrder := make([]string, 0)
	seriesMap := make(map[string][]int)
	ensureSeries := func(key string) []int {
		if pts, ok := seriesMap[key]; ok {
			return pts
		}
		pts := make([]int, len(dates))
		seriesMap[key] = pts
		seriesOrder = append(seriesOrder, key)
		return pts
	}
	if groupBy == "priority" {
		for _, p := range budgetPriorities {
			ensureSeries(p)
		}
	} else {
		ensureSeries("all")
	}
	for _, r := range raw {
		key := "all"
		if r.Priority != nil {
			key = *r.Priority
		}
		pts := ensureSeries(key)
		if idx, ok := dateIndex[r.SnapshotDate.UTC().Format("2006-01-02")]; ok {
			pts[idx] = r.N
		}
	}

	series := make([]Series, len(seriesOrder))
	for i, key := range seriesOrder {
		series[i] = Series{Key: key, Label: pCode(key), Points: seriesMap[key]}
	}
	sortSeriesByRank(series)

	return Timeseries{Window: days, Metric: metric, GroupBy: groupBy, Dates: dates, Series: series}, nil
}

func sortSeriesByRank(series []Series) {
	rank := func(label string) int {
		if r, ok := priorityRank[label]; ok {
			return r
		}
		return 5 // "all" and anything unrecognized
	}
	sort.SliceStable(series, func(i, j int) bool { return rank(series[i].Label) < rank(series[j].Label) })
}
