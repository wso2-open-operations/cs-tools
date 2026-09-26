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
	"sort"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to the docker-composed Postgres, skipping the test
// (rather than failing) when it's unreachable.
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

// seedMetricsYesterdaySnapshots adds a second sla_snapshots row per fixture
// issue, dated yesterday (UTC, via Postgres's CURRENT_DATE - 1), with a
// deliberately different state/status mix from the fixture's today rows:
// yesterday counts violated=2 at_risk=0 productSide=1 against today's
// violated=1 at_risk=1 productSide=2. Every spark column therefore differs
// from every other column and from its own previous day, so an off-by-one
// or a mixed-up date in the spark aggregation cannot go unnoticed.
//
// It is deliberately separate from seedMetricsFixture: the timeseries tests
// assert gap-filled zeros on every day but today.
func seedMetricsYesterdaySnapshots(t *testing.T, pool *pgxpool.Pool, repositoryID int32) {
	t.Helper()
	ctx := context.Background()

	yesterday := []struct {
		number        int
		priority      string
		currentStatus string
		slaState      string
	}{
		{101, "Critical(P1)", "In Progress", "VIOLATED"},
		{102, "High(P2)", "WOC", "VIOLATED"},
		{103, "Medium(P3)", "WOC", "OK"},
	}
	for _, f := range yesterday {
		var issueID int32
		if err := pool.QueryRow(ctx, `SELECT id FROM issues WHERE repository_id = $1 AND github_number = $2`,
			repositoryID, f.number).Scan(&issueID); err != nil {
			t.Fatalf("look up issue %d: %v", f.number, err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO sla_snapshots (snapshot_date, issue_id, repository_id, priority, current_status, sla_state)
			VALUES (CURRENT_DATE - 1, $1, $2, $3, $4, $5)
		`, issueID, repositoryID, f.priority, f.currentStatus, f.slaState); err != nil {
			t.Fatalf("insert yesterday sla_snapshot %d: %v", f.number, err)
		}
	}
}

const metricsQuietFixtureRepo = "test-owner/test-metrics-quiet"

// seedQuietRepoFixture creates an enabled repository with no issues at all —
// the case where a repo has no current open non-terminal issue and must
// still show up in Projects/Volume, which cover every enabled repo.
func seedQuietRepoFixture(t *testing.T, pool *pgxpool.Pool) (repositoryID int32) {
	t.Helper()
	ctx := context.Background()

	var projectID int32
	if err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1,$2,true) RETURNING id`,
		"PVT_metrics_quiet_test", "Metrics Quiet Test").Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1,$2,$3,$4,true) RETURNING id
	`, "test-owner", "test-metrics-quiet", `label:"Origin/CS"`, projectID).Scan(&repositoryID); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM repositories WHERE id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM projects WHERE id = $1`, projectID)
	})
	return repositoryID
}

// findProject returns the project entry matching repo ("owner/name"), or
// nil if absent.
func findProject(projects []Project, repo string) *Project {
	for i := range projects {
		if projects[i].Repo == repo {
			return &projects[i]
		}
	}
	return nil
}

const (
	abtFixtureRepo      = "test-owner/test-metrics-abt"
	abtTeamAlpha        = "ABT-Test-Alpha"
	abtTeamBeta         = "ABT-Test-Beta"
	abtTeamClosedOnly   = "ABT-Test-Closed"
	abtTeamDisabledOnly = "ABT-Test-Disabled"
)

// seedAbtTeamFixture creates an enabled repository holding four open issues
// across two ABT teams plus one team-less issue, and two issues that must
// never reach the overview: one on a CLOSED issue and one in a disabled
// repository. Every open issue in the enabled repo also gets a matching
// sla_snapshots row dated today, so the spark and timeseries sections have
// data to narrow.
//
// The ABT teams are deliberately named so they sort alphabetically in the
// order Alpha, Beta, Closed, Disabled, and distinctively enough not to
// collide with anything else sharing the test database.
func seedAbtTeamFixture(t *testing.T, pool *pgxpool.Pool) (repositoryID int32) {
	t.Helper()
	ctx := context.Background()

	var projectID int32
	if err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1,$2,true) RETURNING id`,
		"PVT_metrics_abt_test", "Metrics ABT Test").Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1,$2,$3,$4,true) RETURNING id
	`, "test-owner", "test-metrics-abt", `label:"Origin/CS"`, projectID).Scan(&repositoryID); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	var disabledRepoID int32
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1,$2,$3,$4,false) RETURNING id
	`, "test-owner", "test-metrics-abt-disabled", `label:"Origin/CS"`, projectID).Scan(&disabledRepoID); err != nil {
		t.Fatalf("create disabled repository: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		for _, id := range []int32{repositoryID, disabledRepoID} {
			pool.Exec(bg, `DELETE FROM sla_snapshots WHERE repository_id = $1`, id)
			pool.Exec(bg, `DELETE FROM issue_sla WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, id)
			pool.Exec(bg, `DELETE FROM issues WHERE repository_id = $1`, id)
			pool.Exec(bg, `DELETE FROM repositories WHERE id = $1`, id)
		}
		pool.Exec(bg, `DELETE FROM projects WHERE id = $1`, projectID)
	})

	fixtures := []struct {
		repositoryID  int32
		number        int
		state         string
		priority      string
		currentStatus string
		slaState      string
		abtTeam       *string
	}{
		{repositoryID, 201, "OPEN", "Critical(P1)", "In Progress", "VIOLATED", ptr(abtTeamAlpha)},
		{repositoryID, 202, "OPEN", "High(P2)", "WOC", "AT_RISK", ptr(abtTeamAlpha)},
		{repositoryID, 203, "OPEN", "Medium(P3)", "Open", "OK", ptr(abtTeamBeta)},
		{repositoryID, 204, "OPEN", "Critical(P1)", "Open", "VIOLATED", nil},
		{repositoryID, 205, "CLOSED", "Critical(P1)", "Open", "VIOLATED", ptr(abtTeamClosedOnly)},
		{disabledRepoID, 206, "OPEN", "Critical(P1)", "Open", "VIOLATED", ptr(abtTeamDisabledOnly)},
	}

	for _, f := range fixtures {
		var issueID int32
		if err := pool.QueryRow(ctx, `
			INSERT INTO issues (repository_id, github_number, state, priority, current_status, abt_team, github_created_at, github_updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,now(),now()) RETURNING id
		`, f.repositoryID, f.number, f.state, f.priority, f.currentStatus, f.abtTeam).Scan(&issueID); err != nil {
			t.Fatalf("insert issue %d: %v", f.number, err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO issue_sla (issue_id, priority, budget_hours, consumed_hours, sla_state, sla_running, computed_at, computed_through)
			VALUES ($1,$2,24,0,$3,false,now(),now())
		`, issueID, f.priority, f.slaState); err != nil {
			t.Fatalf("insert issue_sla %d: %v", f.number, err)
		}
		if f.state != "OPEN" || f.repositoryID != repositoryID {
			continue
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO sla_snapshots (snapshot_date, issue_id, repository_id, priority, current_status, sla_state)
			VALUES (CURRENT_DATE, $1, $2, $3, $4, $5)
		`, issueID, f.repositoryID, f.priority, f.currentStatus, f.slaState); err != nil {
			t.Fatalf("insert sla_snapshot %d: %v", f.number, err)
		}
	}

	return repositoryID
}

// ptr returns a pointer to v — the filter fields and nullable fixture
// columns are all pointer-typed.
func ptr[T any](v T) *T { return &v }

// indexOf returns the position of want in values, or -1 if absent.
func indexOf(values []string, want string) int {
	for i, v := range values {
		if v == want {
			return i
		}
	}
	return -1
}

// TestBuildOverviewHeroCountsAndSpark verifies the hero section's
// violated/atRisk/cs/productSide counts match the seeded fixture when
// scoped to one repo, and pins all three spark columns across two
// snapshot dates: each column's today (index 15) and yesterday (index 14)
// value, plus the delta derived from them.
func TestBuildOverviewHeroCountsAndSpark(t *testing.T) {
	pool := testPool(t)
	repositoryID := seedMetricsFixture(t, pool)
	seedMetricsYesterdaySnapshots(t, pool, repositoryID)
	repoFilter := metricsFixtureRepo

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig, Filter{Repo: &repoFilter})
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

	// Spark columns, pinned per day: index 15 is today, index 14 yesterday.
	sparkCases := []struct {
		name          string
		spark         []int
		delta         int
		today         int
		yesterday     int
		expectedDelta int
	}{
		{name: "violated", spark: overview.Hero.Violated.Spark, delta: overview.Hero.Violated.Delta, today: 1, yesterday: 2, expectedDelta: -1},
		{name: "atRisk", spark: overview.Hero.AtRisk.Spark, delta: overview.Hero.AtRisk.Delta, today: 1, yesterday: 0, expectedDelta: 1},
		{name: "productSide", spark: overview.Hero.ProductSide.Spark, delta: overview.Hero.ProductSide.Delta, today: 2, yesterday: 1, expectedDelta: 1},
	}
	for _, tc := range sparkCases {
		if len(tc.spark) != 16 {
			t.Fatalf("%s: expected a 16-element spark array, got %d", tc.name, len(tc.spark))
		}
		if tc.spark[15] != tc.today {
			t.Errorf("%s: expected today's (index 15) spark count=%d, got %d", tc.name, tc.today, tc.spark[15])
		}
		if tc.spark[14] != tc.yesterday {
			t.Errorf("%s: expected yesterday's (index 14) spark count=%d, got %d", tc.name, tc.yesterday, tc.spark[14])
		}
		if tc.delta != tc.expectedDelta {
			t.Errorf("%s: expected delta=%d, got %d", tc.name, tc.expectedDelta, tc.delta)
		}
		for i := 0; i < 14; i++ {
			if tc.spark[i] != 0 {
				t.Errorf("%s: expected gap-filled zero at spark[%d], got %d", tc.name, i, tc.spark[i])
			}
		}
	}
}

// TestBuildOverviewProjectsCard verifies the fixture repo's Projects card
// reports the expected violated/atRisk/cs/onTrack/tracked counts.
func TestBuildOverviewProjectsCard(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig, Filter{Repo: &repoFilter})
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}

	// Projects always reflects ALL enabled repos, so we look up our
	// fixture's row by name rather than asserting the whole list.
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

// TestBuildOverviewProjectsIncludesRepoWithNoOpenIssues guards against
// Projects/Volume silently dropping an enabled repo just because it has no
// current open non-terminal issue: allIssues only contains repos with at
// least one such issue, so building Projects/Volume from it would drop a
// quiet repo entirely instead of showing it with zero counts.
func TestBuildOverviewProjectsIncludesRepoWithNoOpenIssues(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	seedQuietRepoFixture(t, pool)

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig, Filter{})
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}

	p := findProject(overview.Projects, metricsQuietFixtureRepo)
	if p == nil {
		t.Fatalf("expected a project entry for %s (zero open issues), got %+v", metricsQuietFixtureRepo, overview.Projects)
	}
	if p.Violated != 0 || p.AtRisk != 0 || p.Cs != 0 || p.OnTrack != 0 || p.OpenTracked != 0 || p.Untracked != 0 {
		t.Errorf("expected an all-zero project entry, got %+v", p)
	}
	if !p.AllClear {
		t.Error("expected allClear=true for a repo with no issues")
	}

	found := false
	for _, v := range overview.Volume {
		if v.RepoID == p.RepoID {
			found = true
			if v.Total != 0 {
				t.Errorf("expected zero volume for the quiet repo, got %+v", v)
			}
		}
	}
	if !found {
		t.Errorf("expected a volume entry for %s, got %+v", metricsQuietFixtureRepo, overview.Volume)
	}
}

// TestBuildOverviewPrioritiesAndMatrix verifies all 3 canonical priority
// tiers are present, correctly ranked, and the matrix totals match.
func TestBuildOverviewPrioritiesAndMatrix(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig, Filter{Repo: &repoFilter})
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

	// Fixture 102 (High(P2), WOC — a CS-side status) is AT_RISK: the matrix
	// row's own cells must land it in Cs only, never also in AtRisk, and
	// every row's four cells must sum to that row's own total.
	if overview.Matrix.Rows[1].Cells.AtRisk != 0 || overview.Matrix.Rows[1].Cells.Cs != 1 {
		t.Errorf("expected P2 matrix row cells atRisk=0 cs=1, got %+v", overview.Matrix.Rows[1].Cells)
	}
	for _, row := range overview.Matrix.Rows {
		sum := row.Cells.Violated + row.Cells.AtRisk + row.Cells.OnTrack + row.Cells.Cs
		if sum != row.Total {
			t.Errorf("expected row %s cells to sum to its own total %d, got cells=%+v (sum=%d)", row.Code, row.Total, row.Cells, sum)
		}
	}

	if overview.Matrix.GrandTotal != 3 {
		t.Errorf("expected matrix grandTotal=3, got %d", overview.Matrix.GrandTotal)
	}
	// Fixture 102 (High(P2), WOC — a CS-side status) is AT_RISK: the matrix's
	// four cells are mutually exclusive, so a CS-side issue lands only in Cs,
	// never also in AtRisk, regardless of its own SLA state.
	if overview.Matrix.Totals.Violated != 1 || overview.Matrix.Totals.AtRisk != 0 || overview.Matrix.Totals.Cs != 1 || overview.Matrix.Totals.OnTrack != 1 {
		t.Errorf("expected matrix totals violated=1 atRisk=0 cs=1 onTrack=1, got %+v", overview.Matrix.Totals)
	}
	if overview.Matrix.Totals.Violated+overview.Matrix.Totals.AtRisk+overview.Matrix.Totals.OnTrack+overview.Matrix.Totals.Cs != overview.Matrix.GrandTotal {
		t.Errorf("expected matrix cells to sum to grandTotal, got totals=%+v grandTotal=%d", overview.Matrix.Totals, overview.Matrix.GrandTotal)
	}
}

// TestBuildOverviewPriorityFilterNarrowsHero verifies a priority filter
// narrows the hero counts to just that priority tier.
func TestBuildOverviewPriorityFilterNarrowsHero(t *testing.T) {
	pool := testPool(t)
	seedMetricsFixture(t, pool)
	repoFilter := metricsFixtureRepo
	priorityFilter := "Critical(P1)"

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig, Filter{Repo: &repoFilter, Priority: &priorityFilter})
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

// TestBuildOverviewSurfacesUnknownStatuses verifies the unrecognized-status
// dashboard signal: rows in unknown_statuses (as the recompute tick left
// them) come back on Overview, most-frequent first, independent of any
// repo/priority filter.
func TestBuildOverviewSurfacesUnknownStatuses(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM unknown_statuses WHERE status IN ('Overview Test Mystery A', 'Overview Test Mystery B')`)
	})
	if _, err := pool.Exec(ctx, `
		INSERT INTO unknown_statuses (status, occurrence_count, first_seen_at, last_seen_at) VALUES
			('Overview Test Mystery A', 3, $1, $1),
			('Overview Test Mystery B', 7, $1, $1)
	`, now); err != nil {
		t.Fatalf("seed unknown_statuses: %v", err)
	}

	overview, err := BuildOverview(ctx, pool, metricsTestConfig, Filter{})
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}

	byStatus := make(map[string]UnknownStatus, len(overview.UnknownStatuses))
	for _, u := range overview.UnknownStatuses {
		byStatus[u.Status] = u
	}
	a, ok := byStatus["Overview Test Mystery A"]
	if !ok {
		t.Fatalf("expected 'Overview Test Mystery A' in overview.unknownStatuses, got %+v", overview.UnknownStatuses)
	}
	if a.OccurrenceCount != 3 {
		t.Errorf("expected occurrenceCount=3, got %d", a.OccurrenceCount)
	}
	if _, ok := byStatus["Overview Test Mystery B"]; !ok {
		t.Fatalf("expected 'Overview Test Mystery B' in overview.unknownStatuses, got %+v", overview.UnknownStatuses)
	}

	// Most-frequent first: B (7) must be ordered before A (3).
	var idxA, idxB int
	for i, u := range overview.UnknownStatuses {
		if u.Status == "Overview Test Mystery A" {
			idxA = i
		}
		if u.Status == "Overview Test Mystery B" {
			idxB = i
		}
	}
	if idxB > idxA {
		t.Errorf("expected higher-occurrence status B before A, got order %+v", overview.UnknownStatuses)
	}
}

// TestBuildOverviewAbtTeamsListsOpenEnabledTeams verifies the ABT Team
// dropdown's option list: sorted, distinct, drawn only from open issues in
// enabled repositories, and unchanged by whichever filters are active.
func TestBuildOverviewAbtTeamsListsOpenEnabledTeams(t *testing.T) {
	pool := testPool(t)
	seedAbtTeamFixture(t, pool)
	ctx := context.Background()

	unfiltered, err := BuildOverview(ctx, pool, metricsTestConfig, Filter{})
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}

	// Other repositories share this database, so assert on the fixture's own
	// distinctive team names and on the whole list's ordering, not on length.
	alphaIdx := indexOf(unfiltered.AbtTeams, abtTeamAlpha)
	betaIdx := indexOf(unfiltered.AbtTeams, abtTeamBeta)
	if alphaIdx < 0 || betaIdx < 0 {
		t.Fatalf("expected %s and %s in abtTeams, got %+v", abtTeamAlpha, abtTeamBeta, unfiltered.AbtTeams)
	}
	if alphaIdx >= betaIdx {
		t.Errorf("expected %s before %s, got %+v", abtTeamAlpha, abtTeamBeta, unfiltered.AbtTeams)
	}
	if indexOf(unfiltered.AbtTeams, abtTeamClosedOnly) >= 0 {
		t.Errorf("expected a CLOSED issue's team to be excluded, got %+v", unfiltered.AbtTeams)
	}
	if indexOf(unfiltered.AbtTeams, abtTeamDisabledOnly) >= 0 {
		t.Errorf("expected a disabled repo's team to be excluded, got %+v", unfiltered.AbtTeams)
	}
	if !sort.StringsAreSorted(unfiltered.AbtTeams) {
		t.Errorf("expected abtTeams sorted, got %+v", unfiltered.AbtTeams)
	}
	for i := 1; i < len(unfiltered.AbtTeams); i++ {
		if unfiltered.AbtTeams[i] == unfiltered.AbtTeams[i-1] {
			t.Errorf("expected distinct abtTeams, got a repeat of %q", unfiltered.AbtTeams[i])
		}
	}

	// The dropdown must keep offering every team whichever filters narrow
	// the rest of the response — including the team filter itself.
	narrowing := []struct {
		name   string
		filter Filter
	}{
		{name: "priority", filter: Filter{Priority: ptr("Critical(P1)")}},
		{name: "repo", filter: Filter{Repo: ptr(abtFixtureRepo)}},
		{name: "abtTeam", filter: Filter{AbtTeam: ptr(abtTeamBeta)}},
	}
	for _, tc := range narrowing {
		t.Run(tc.name, func(t *testing.T) {
			filtered, err := BuildOverview(ctx, pool, metricsTestConfig, tc.filter)
			if err != nil {
				t.Fatalf("BuildOverview: %v", err)
			}
			a, b := indexOf(filtered.AbtTeams, abtTeamAlpha), indexOf(filtered.AbtTeams, abtTeamBeta)
			if a < 0 || b < 0 || a >= b {
				t.Errorf("expected %s before %s regardless of the %s filter, got %+v",
					abtTeamAlpha, abtTeamBeta, tc.name, filtered.AbtTeams)
			}
			if indexOf(filtered.AbtTeams, abtTeamClosedOnly) >= 0 || indexOf(filtered.AbtTeams, abtTeamDisabledOnly) >= 0 {
				t.Errorf("expected closed/disabled teams excluded under the %s filter, got %+v", tc.name, filtered.AbtTeams)
			}
		})
	}
}

// TestBuildOverviewAbtTeamFilterNarrowsEverySection verifies an abtTeam
// filter narrows the hero, spark, projects, priorities, matrix and volume
// sections to that team's issues, and is echoed back on filters.
func TestBuildOverviewAbtTeamFilterNarrowsEverySection(t *testing.T) {
	pool := testPool(t)
	seedAbtTeamFixture(t, pool)
	repoFilter := abtFixtureRepo
	team := abtTeamAlpha

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig,
		Filter{Repo: &repoFilter, AbtTeam: &team})
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}

	if overview.Filters.AbtTeam == nil || *overview.Filters.AbtTeam != team {
		t.Errorf("expected filters.abtTeam=%q, got %+v", team, overview.Filters.AbtTeam)
	}

	// Alpha owns the VIOLATED P1 ("In Progress") and the AT_RISK P2 ("WOC")
	// issue; the team-less VIOLATED P1 issue must not be counted.
	if overview.Hero.Violated.N != 1 {
		t.Errorf("expected hero.violated.n=1, got %d", overview.Hero.Violated.N)
	}
	if overview.Hero.AtRisk.N != 1 {
		t.Errorf("expected hero.atRisk.n=1, got %d", overview.Hero.AtRisk.N)
	}
	if overview.Hero.Cs.N != 1 {
		t.Errorf("expected hero.cs.n=1, got %d", overview.Hero.Cs.N)
	}
	if overview.Hero.ProductSide.N != 1 {
		t.Errorf("expected hero.productSide.n=1, got %d", overview.Hero.ProductSide.N)
	}

	if overview.Hero.Violated.Spark[15] != 1 {
		t.Errorf("expected today's violated spark=1, got %d", overview.Hero.Violated.Spark[15])
	}
	if overview.Hero.AtRisk.Spark[15] != 1 {
		t.Errorf("expected today's at_risk spark=1, got %d", overview.Hero.AtRisk.Spark[15])
	}
	if overview.Hero.ProductSide.Spark[15] != 1 {
		t.Errorf("expected today's product-side spark=1, got %d", overview.Hero.ProductSide.Spark[15])
	}

	p := findProject(overview.Projects, abtFixtureRepo)
	if p == nil {
		t.Fatalf("expected a project entry for %s, got %+v", abtFixtureRepo, overview.Projects)
	}
	if p.Violated != 1 || p.AtRisk != 1 || p.Cs != 1 || p.OnTrack != 0 {
		t.Errorf("expected project violated=1 atRisk=1 cs=1 onTrack=0, got %+v", p)
	}
	if p.OpenTracked != 2 || p.Untracked != 0 {
		t.Errorf("expected project openTracked=2 untracked=0, got %+v", p)
	}

	if overview.Priorities[0].Violated != 1 || overview.Priorities[0].Total != 1 {
		t.Errorf("expected P1 violated=1 total=1, got %+v", overview.Priorities[0])
	}
	if overview.Priorities[1].AtRisk != 1 || overview.Priorities[1].Total != 1 {
		t.Errorf("expected P2 atRisk=1 total=1, got %+v", overview.Priorities[1])
	}
	if overview.Priorities[2].Total != 0 {
		t.Errorf("expected P3 total=0 (it belongs to another team), got %+v", overview.Priorities[2])
	}

	if overview.Matrix.GrandTotal != 2 {
		t.Errorf("expected matrix grandTotal=2, got %d", overview.Matrix.GrandTotal)
	}
	// The AT_RISK P2 issue is on a CS-side status ("WOC"), so it lands only
	// in Cs, never also in AtRisk.
	if overview.Matrix.Totals.Violated != 1 || overview.Matrix.Totals.AtRisk != 0 || overview.Matrix.Totals.Cs != 1 || overview.Matrix.Totals.OnTrack != 0 {
		t.Errorf("expected matrix totals violated=1 atRisk=0 cs=1 onTrack=0, got %+v", overview.Matrix.Totals)
	}

	// Volume counts issue creation, so the fixture's issues all land in the
	// current (last) week: 2 for Alpha, against 4 tracked issues in the repo.
	var volumeTotal int
	for _, v := range overview.Volume {
		if v.RepoID == p.RepoID {
			volumeTotal = v.Total
			if last := v.Weeks[len(v.Weeks)-1]; last.ByPriority.P1 != 1 || last.ByPriority.P2 != 1 {
				t.Errorf("expected the current week to hold P1=1 P2=1, got %+v", last)
			}
		}
	}
	if volumeTotal != 2 {
		t.Errorf("expected volume total=2 for %s, got %d", team, volumeTotal)
	}
}

// TestBuildOverviewAllFiltersTogether verifies repo, priority and abtTeam
// applied simultaneously, so the argument ordering every filtered query
// builds is exercised with all three placeholders bound at once.
func TestBuildOverviewAllFiltersTogether(t *testing.T) {
	pool := testPool(t)
	seedAbtTeamFixture(t, pool)
	f := Filter{Repo: ptr(abtFixtureRepo), Priority: ptr("Critical(P1)"), AbtTeam: ptr(abtTeamAlpha)}

	overview, err := BuildOverview(context.Background(), pool, metricsTestConfig, f)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}

	// Only issue 201 is in this repo, at P1, on Alpha.
	if overview.Hero.Violated.N != 1 {
		t.Errorf("expected hero.violated.n=1, got %d", overview.Hero.Violated.N)
	}
	if overview.Hero.AtRisk.N != 0 {
		t.Errorf("expected hero.atRisk.n=0 (its issue is P2), got %d", overview.Hero.AtRisk.N)
	}
	if overview.Hero.Violated.Spark[15] != 1 {
		t.Errorf("expected today's violated spark=1, got %d", overview.Hero.Violated.Spark[15])
	}
	if overview.Hero.AtRisk.Spark[15] != 0 {
		t.Errorf("expected today's at_risk spark=0, got %d", overview.Hero.AtRisk.Spark[15])
	}
	if overview.Hero.ProductSide.Spark[15] != 1 {
		t.Errorf("expected today's product-side spark=1, got %d", overview.Hero.ProductSide.Spark[15])
	}

	if overview.Filters.Repo == nil || overview.Filters.Priority == nil || overview.Filters.AbtTeam == nil {
		t.Errorf("expected all three filters echoed back, got %+v", overview.Filters)
	}
}

// TestBuildOverviewEmptyProductSideCategory verifies a taxonomy with no
// PRODUCT_SIDE status — a valid configuration — yields zero product-side
// counts across the whole spark window rather than an error.
func TestBuildOverviewEmptyProductSideCategory(t *testing.T) {
	pool := testPool(t)
	repositoryID := seedMetricsFixture(t, pool)
	seedMetricsYesterdaySnapshots(t, pool, repositoryID)
	repoFilter := metricsFixtureRepo

	cfg := &config.AppConfig{
		Taxonomy: config.Taxonomy{
			Statuses: []config.StatusEntry{
				{Name: "WOC", Category: config.CategoryCSSide, AccruesSla: false},
				{Name: "Resolved", Category: config.CategoryOther, AccruesSla: false, IsTerminal: true},
			},
		},
		Budgets: metricsTestConfig.Budgets,
	}

	overview, err := BuildOverview(context.Background(), pool, cfg, Filter{Repo: &repoFilter})
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	if overview.Hero.ProductSide.N != 0 {
		t.Errorf("expected hero.productSide.n=0, got %d", overview.Hero.ProductSide.N)
	}
	if overview.Hero.ProductSide.Delta != 0 {
		t.Errorf("expected hero.productSide.delta=0, got %d", overview.Hero.ProductSide.Delta)
	}
	for i, n := range overview.Hero.ProductSide.Spark {
		if n != 0 {
			t.Errorf("expected an all-zero product-side spark, got %d at index %d", n, i)
		}
	}
	// The other spark columns still report the fixture's counts.
	if overview.Hero.Violated.Spark[15] != 1 || overview.Hero.Violated.Spark[14] != 2 {
		t.Errorf("expected the violated spark unaffected (today=1 yesterday=2), got %+v", overview.Hero.Violated.Spark)
	}
}
