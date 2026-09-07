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
	"os"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://gid:gid@localhost:5433/gid?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Skipf("skipping: postgres unreachable at %s: %v", url, err)
		return nil
	}
	t.Cleanup(pool.Close)
	return pool
}

var metricsTestConfig = &config.AppConfig{
	Taxonomy: config.Taxonomy{
		Statuses: []config.StatusEntry{
			{Name: "Open", Category: config.CategoryProductSide, AccruesSla: true},
			{Name: "In Progress", Category: config.CategoryProductSide, AccruesSla: true},
			{Name: "WOC", Category: config.CategoryCSSide, AccruesSla: false},
			{Name: "Resolved", Category: config.CategoryOther, AccruesSla: false, IsTerminal: true},
		},
	},
	Budgets: []config.BudgetEntry{
		{Priority: "Critical(P1)", BudgetHours: 24, Coverage: config.Coverage24x7, Rank: 1},
		{Priority: "High(P2)", BudgetHours: 24, Coverage: config.Coverage12x5Ist, Rank: 2},
		{Priority: "Medium(P3)", BudgetHours: 48, Coverage: config.Coverage12x5Ist, Rank: 3},
	},
}

const metricsFixtureRepo = "test-owner/test-metrics"

// seedMetricsFixture creates one project+repository with three issues (one
// per canonical priority tier, each in a distinct sla_state and current
// status) plus a matching sla_snapshots row dated today (UTC, via
// Postgres's own CURRENT_DATE to avoid any clock skew with the query's
// own now()), so overview/timeseries tests get deterministic, non-time-
// -flaky fixtures.
func seedMetricsFixture(t *testing.T, pool *pgxpool.Pool) (repositoryID int32) {
	t.Helper()
	ctx := context.Background()

	var projectID int32
	if err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1,$2,true) RETURNING id`,
		"PVT_metrics_test", "Metrics Test").Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1,$2,$3,$4,true) RETURNING id
	`, "test-owner", "test-metrics", `label:"Origin/CS"`, projectID).Scan(&repositoryID); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM sla_snapshots WHERE repository_id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM issue_sla WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repositoryID)
		pool.Exec(bg, `DELETE FROM issues WHERE repository_id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM repositories WHERE id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM projects WHERE id = $1`, projectID)
	})

	fixtures := []struct {
		number        int
		priority      string
		currentStatus string
		slaState      string
	}{
		{101, "Critical(P1)", "In Progress", "VIOLATED"},
		{102, "High(P2)", "WOC", "AT_RISK"},
		{103, "Medium(P3)", "Open", "OK"},
	}

	for _, f := range fixtures {
		var issueID int32
		if err := pool.QueryRow(ctx, `
			INSERT INTO issues (repository_id, github_number, state, priority, current_status, github_created_at, github_updated_at)
			VALUES ($1,$2,'OPEN',$3,$4,now(),now()) RETURNING id
		`, repositoryID, f.number, f.priority, f.currentStatus).Scan(&issueID); err != nil {
			t.Fatalf("insert issue %d: %v", f.number, err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO issue_sla (issue_id, priority, budget_hours, consumed_hours, sla_state, sla_running, computed_at, computed_through)
			VALUES ($1,$2,24,0,$3,false,now(),now())
		`, issueID, f.priority, f.slaState); err != nil {
			t.Fatalf("insert issue_sla %d: %v", f.number, err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO sla_snapshots (snapshot_date, issue_id, repository_id, priority, current_status, sla_state)
			VALUES (CURRENT_DATE, $1, $2, $3, $4, $5)
		`, issueID, repositoryID, f.priority, f.currentStatus, f.slaState); err != nil {
			t.Fatalf("insert sla_snapshot %d: %v", f.number, err)
		}
	}

	return repositoryID
}

func findProject(projects []Project, repo string) *Project {
	for i := range projects {
		if projects[i].Repo == repo {
			return &projects[i]
		}
	}
	return nil
}

func TestBuildOverviewHeroCountsAndSpark(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig, &repoFilter, nil)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}

	if overview.Hero.Violated.N != 1 {
		t.Errorf("expected hero.violated.n=1, got %d", overview.Hero.Violated.N)
	}
	if overview.Hero.AtRisk.N != 1 {
		t.Errorf("expected hero.atRisk.n=1, got %d", overview.Hero.AtRisk.N)
	}
	if overview.Hero.Cs.N != 1 { // WOC is CS_SIDE
		t.Errorf("expected hero.cs.n=1, got %d", overview.Hero.Cs.N)
	}
	if overview.Hero.ProductSide.N != 2 { // "In Progress" + "Open" are PRODUCT_SIDE
		t.Errorf("expected hero.productSide.n=2, got %d", overview.Hero.ProductSide.N)
	}

	if len(overview.Hero.Violated.Spark) != 16 {
		t.Fatalf("expected a 16-element spark array, got %d", len(overview.Hero.Violated.Spark))
	}
	if overview.Hero.Violated.Spark[15] != 1 {
		t.Errorf("expected today's (index 15) violated spark count=1, got %d", overview.Hero.Violated.Spark[15])
	}
	if overview.Hero.AtRisk.Spark[15] != 1 {
		t.Errorf("expected today's at_risk spark count=1, got %d", overview.Hero.AtRisk.Spark[15])
	}
}

func TestBuildOverviewProjectsCard(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig, &repoFilter, nil)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}

	// Projects always reflects ALL enabled repos (SPEC §6.6), so we look up
	// our fixture's row by name rather than asserting the whole list.
	p := findProject(overview.Projects, metricsFixtureRepo)
	if p == nil {
		t.Fatalf("expected a project entry for %s, got %+v", metricsFixtureRepo, overview.Projects)
	}
	if p.Violated != 1 || p.AtRisk != 1 || p.Cs != 1 || p.OnTrack != 1 {
		t.Errorf("expected violated=1 atRisk=1 cs=1 onTrack=1, got %+v", p)
	}
	if p.OpenTracked != 3 || p.Untracked != 0 {
		t.Errorf("expected openTracked=3 untracked=0, got %+v", p)
	}
	if p.AllClear {
		t.Error("expected allClear=false (has violated/atRisk/cs issues)")
	}
}

func TestBuildOverviewPrioritiesAndMatrix(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig, &repoFilter, nil)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}

	if len(overview.Priorities) != 3 {
		t.Fatalf("expected all 3 canonical tiers present, got %d", len(overview.Priorities))
	}
	// Sorted P1 -> P2 -> P3.
	wantCodes := []string{"P1", "P2", "P3"}
	for i, code := range wantCodes {
		if overview.Priorities[i].Code != code {
			t.Errorf("expected priorities[%d].code=%s, got %s", i, code, overview.Priorities[i].Code)
		}
	}
	if overview.Priorities[0].Violated != 1 || overview.Priorities[0].Total != 1 {
		t.Errorf("expected P1 violated=1 total=1, got %+v", overview.Priorities[0])
	}
	if overview.Priorities[1].AtRisk != 1 {
		t.Errorf("expected P2 atRisk=1, got %+v", overview.Priorities[1])
	}
	if overview.Priorities[2].OnTrack != 1 {
		t.Errorf("expected P3 onTrack=1, got %+v", overview.Priorities[2])
	}

	if overview.Matrix.GrandTotal != 3 {
		t.Errorf("expected matrix grandTotal=3, got %d", overview.Matrix.GrandTotal)
	}
	if overview.Matrix.Totals.Violated != 1 || overview.Matrix.Totals.AtRisk != 1 || overview.Matrix.Totals.Cs != 1 || overview.Matrix.Totals.OnTrack != 1 {
		t.Errorf("expected matrix totals violated=1 atRisk=1 cs=1 onTrack=1, got %+v", overview.Matrix.Totals)
	}
}

func TestBuildOverviewPriorityFilterNarrowsHero(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo
	priorityFilter := "Critical(P1)"

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig, &repoFilter, &priorityFilter)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	if overview.Hero.Violated.N != 1 {
		t.Errorf("expected hero.violated.n=1 for Critical(P1) only, got %d", overview.Hero.Violated.N)
	}
	if overview.Hero.AtRisk.N != 0 {
		t.Errorf("expected hero.atRisk.n=0 when filtered to Critical(P1), got %d", overview.Hero.AtRisk.N)
	}
}
