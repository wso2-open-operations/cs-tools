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

package jobs

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
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

var tickTestAppConfig = &config.AppConfig{
	Taxonomy: config.Taxonomy{
		Statuses: []config.StatusEntry{
			{Name: "Open", Category: config.CategoryProductSide, AccruesSla: true},
			{Name: "WOC", Category: config.CategoryCSSide, AccruesSla: false},
		},
		Aliases: []config.AliasEntry{},
	},
	Budgets: []config.BudgetEntry{
		{Priority: "Critical(P1)", BudgetHours: 24, Coverage: config.Coverage24x7, Rank: 1},
	},
	Settings: config.Settings{AtRiskThreshold: 0.75, RecomputeIntervalMinutes: 10, SyncOverlapMinutes: 15, SeedSnapshotDays: 90, SeedClosedLookbackDays: 90},
}

// strp returns a pointer to s, for building literal *string fixture fields.
func strp(s string) *string { return &s }

// seedIngestedIssue ingests one fixture issue via internal/ingest so the
// tick test exercises RunTickOnce against a realistic issue_sla row rather
// than a hand-inserted one.
func seedIngestedIssue(t *testing.T, pool *pgxpool.Pool) (issueID int32, repositoryID int32) {
	t.Helper()
	ctx := context.Background()

	var projectID int32
	if err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1,$2,true) RETURNING id`,
		"PVT_tick_test", "Tick Test").Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1,$2,$3,$4,true) RETURNING id
	`, "test-owner", "test-repo-tick", `label:"Origin/CS"`, projectID).Scan(&repositoryID); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM sla_snapshots WHERE repository_id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM issue_sla WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repositoryID)
		pool.Exec(bg, `DELETE FROM issue_status_events WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repositoryID)
		pool.Exec(bg, `DELETE FROM issues WHERE repository_id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM repositories WHERE id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM projects WHERE id = $1`, projectID)
	})

	runtime := ingest.BuildRuntimeConfig(tickTestAppConfig)
	now, _ := time.Parse(time.RFC3339, "2026-01-10T12:00:00Z")

	result, err := ingest.IngestIssue(ctx, pool, ingest.Pair{
		Node: github.IssueNode{
			Number: 1, State: "OPEN", URL: "https://github.com/test-owner/test-repo-tick/issues/1",
			CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
			Labels: []string{"Priority/Critical(P1)"},
		},
		Detail: github.IssueDetail{
			Number: 1,
			ProjectStatuses: []github.ProjectStatus{
				{ProjectID: "PVT_tick_test", Status: strp("Open"), StatusUpdatedAt: strp("2026-01-01T00:00:00Z"), ItemCreatedAt: strp("2026-01-01T00:00:00Z")},
			},
		},
	}, ingest.Context{
		RepositoryID: repositoryID, SlaProjectID: projectID,
		Repo:    ingest.RepoRef{Owner: "test-owner", Name: "test-repo-tick", GithubProjectID: "PVT_tick_test"},
		Runtime: runtime, Source: "synthetic", Now: now,
	})
	if err != nil {
		t.Fatalf("seed ingest: %v", err)
	}
	return result.IssueID, repositoryID
}

// seedIngestedIssues ingests n fixture issues (distinct github numbers) into
// one fresh repo/project, for tests that need more rows than
// seedIngestedIssue's single issue.
func seedIngestedIssues(t *testing.T, pool *pgxpool.Pool, n int) (issueIDs []int32, repositoryID int32) {
	t.Helper()
	ctx := context.Background()

	var projectID int32
	if err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1,$2,true) RETURNING id`,
		"PVT_page_test", "Page Test").Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1,$2,$3,$4,true) RETURNING id
	`, "test-owner", "test-repo-page", `label:"Origin/CS"`, projectID).Scan(&repositoryID); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM sla_snapshots WHERE repository_id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM issue_sla WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repositoryID)
		pool.Exec(bg, `DELETE FROM issue_status_events WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repositoryID)
		pool.Exec(bg, `DELETE FROM issues WHERE repository_id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM repositories WHERE id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM projects WHERE id = $1`, projectID)
	})

	runtime := ingest.BuildRuntimeConfig(tickTestAppConfig)
	now, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")

	for i := 1; i <= n; i++ {
		result, err := ingest.IngestIssue(ctx, pool, ingest.Pair{
			Node: github.IssueNode{
				Number: i, State: "OPEN", URL: fmt.Sprintf("https://github.com/test-owner/test-repo-page/issues/%d", i),
				CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				Labels: []string{"Priority/Critical(P1)"},
			},
			Detail: github.IssueDetail{
				Number: i,
				ProjectStatuses: []github.ProjectStatus{
					{ProjectID: "PVT_page_test", Status: strp("Open"), StatusUpdatedAt: strp("2026-01-01T00:00:00Z"), ItemCreatedAt: strp("2026-01-01T00:00:00Z")},
				},
			},
		}, ingest.Context{
			RepositoryID: repositoryID, SlaProjectID: projectID,
			Repo:    ingest.RepoRef{Owner: "test-owner", Name: "test-repo-page", GithubProjectID: "PVT_page_test"},
			Runtime: runtime, Source: "synthetic", Now: now,
		})
		if err != nil {
			t.Fatalf("seed ingest issue %d: %v", i, err)
		}
		issueIDs = append(issueIDs, result.IssueID)
	}
	return issueIDs, repositoryID
}

// TestRunTickOnceWalksAllPagesViaKeysetPagination verifies that with a page
// size smaller than the fixture set, RunTickOnce still processes every
// issue exactly once — proving the i.id > lastID walk correctly advances
// across page boundaries rather than looping or skipping rows.
func TestRunTickOnceWalksAllPagesViaKeysetPagination(t *testing.T) {
	pool := testPool(t)
	const pageSize, issueCount = 2, 5
	origPageSize := recomputePageSize
	recomputePageSize = pageSize
	t.Cleanup(func() { recomputePageSize = origPageSize })

	issueIDs, _ := seedIngestedIssues(t, pool, issueCount)
	runtime := ingest.BuildRuntimeConfig(tickTestAppConfig)

	now, _ := time.Parse(time.RFC3339, "2026-01-02T00:00:00Z")
	summary, err := RunTickOnce(context.Background(), pool, runtime, now)
	if err != nil {
		t.Fatalf("RunTickOnce: %v", err)
	}
	if summary.Processed < issueCount {
		t.Fatalf("expected at least %d issues processed across %d pages of size %d, got %d",
			issueCount, (issueCount+pageSize-1)/pageSize, pageSize, summary.Processed)
	}

	for _, id := range issueIDs {
		var slaState string
		if err := pool.QueryRow(context.Background(), `SELECT sla_state FROM issue_sla WHERE issue_id = $1`, id).Scan(&slaState); err != nil {
			t.Fatalf("query issue_sla for issue %d: %v", id, err)
		}
		if slaState == "" {
			t.Errorf("expected issue %d to have been recomputed, got empty sla_state", id)
		}
		var snapshotCount int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM sla_snapshots WHERE issue_id = $1`, id).Scan(&snapshotCount); err != nil {
			t.Fatalf("query sla_snapshots for issue %d: %v", id, err)
		}
		if snapshotCount != 1 {
			t.Errorf("expected exactly 1 snapshot row for issue %d, got %d", id, snapshotCount)
		}
	}
}

// TestRunTickOnceUpdatesIssueSlaAndSnapshot verifies a tick recomputes
// issue_sla and upserts exactly one sla_snapshots row per day, idempotently
// across repeated ticks on the same day.
func TestRunTickOnceUpdatesIssueSlaAndSnapshot(t *testing.T) {
	pool := testPool(t)
	issueID, repositoryID := seedIngestedIssue(t, pool)
	runtime := ingest.BuildRuntimeConfig(tickTestAppConfig)

	// A day later: 24h of Open (accrues, 24x7) against a 24h budget => VIOLATED.
	now, _ := time.Parse(time.RFC3339, "2026-01-02T00:00:00Z")
	summary, err := RunTickOnce(context.Background(), pool, runtime, now)
	if err != nil {
		t.Fatalf("RunTickOnce: %v", err)
	}
	if summary.Processed < 1 {
		t.Fatalf("expected at least 1 issue processed, got %d", summary.Processed)
	}

	var slaState string
	var pctConsumed float64
	if err := pool.QueryRow(context.Background(), `SELECT sla_state, pct_consumed FROM issue_sla WHERE issue_id = $1`, issueID).Scan(&slaState, &pctConsumed); err != nil {
		t.Fatalf("query issue_sla: %v", err)
	}
	if slaState != "VIOLATED" {
		t.Errorf("expected VIOLATED, got %s (pctConsumed=%v)", slaState, pctConsumed)
	}

	var snapshotCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM sla_snapshots WHERE issue_id = $1`, issueID).Scan(&snapshotCount); err != nil {
		t.Fatalf("query sla_snapshots: %v", err)
	}
	if snapshotCount != 1 {
		t.Fatalf("expected 1 snapshot row, got %d", snapshotCount)
	}

	// Second tick, same day: idempotent — still exactly one snapshot row,
	// historical rows never touched, today's row just updates in place.
	if _, err := RunTickOnce(context.Background(), pool, runtime, now.Add(time.Hour)); err != nil {
		t.Fatalf("second RunTickOnce: %v", err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM sla_snapshots WHERE issue_id = $1`, issueID).Scan(&snapshotCount); err != nil {
		t.Fatalf("query sla_snapshots after second tick: %v", err)
	}
	if snapshotCount != 1 {
		t.Errorf("expected snapshot upsert to stay idempotent (1 row), got %d", snapshotCount)
	}

	_ = repositoryID
}

// TestRunTickOnceFreezesConsumptionOnceIssueIsClosedOnGithub is a
// scheduler-level regression: a GitHub-closed issue whose board status is
// still an accruing one must stop gaining consumed_hours between ticks and
// report TERMINAL, instead of accruing forever because the board was never
// moved to a terminal status.
func TestRunTickOnceFreezesConsumptionOnceIssueIsClosedOnGithub(t *testing.T) {
	pool := testPool(t)
	issueID, _ := seedIngestedIssue(t, pool)
	runtime := ingest.BuildRuntimeConfig(tickTestAppConfig)
	ctx := context.Background()

	// Close the issue on GitHub 6h after it was opened, while its board
	// status ("Open") still accrues — the scenario a missing "closed ->
	// Done" board automation leaves behind.
	closedAt, _ := time.Parse(time.RFC3339, "2026-01-01T06:00:00Z")
	if _, err := pool.Exec(ctx, `UPDATE issues SET state = 'CLOSED', github_closed_at = $2 WHERE id = $1`, issueID, closedAt); err != nil {
		t.Fatalf("close issue: %v", err)
	}

	firstTick, _ := time.Parse(time.RFC3339, "2026-01-02T00:00:00Z")
	if _, err := RunTickOnce(ctx, pool, runtime, firstTick); err != nil {
		t.Fatalf("first RunTickOnce: %v", err)
	}
	var slaState string
	var consumedHours float64
	if err := pool.QueryRow(ctx, `SELECT sla_state, consumed_hours FROM issue_sla WHERE issue_id = $1`, issueID).Scan(&slaState, &consumedHours); err != nil {
		t.Fatalf("query issue_sla after first tick: %v", err)
	}
	if slaState != "TERMINAL" {
		t.Errorf("expected TERMINAL once closed, got %s", slaState)
	}
	closeTo(t, consumedHours, 6)

	// A later tick must not accrue any further: consumed_hours stays frozen
	// at closure instead of growing with wall-clock time.
	secondTick, _ := time.Parse(time.RFC3339, "2026-01-05T00:00:00Z")
	if _, err := RunTickOnce(ctx, pool, runtime, secondTick); err != nil {
		t.Fatalf("second RunTickOnce: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT sla_state, consumed_hours FROM issue_sla WHERE issue_id = $1`, issueID).Scan(&slaState, &consumedHours); err != nil {
		t.Fatalf("query issue_sla after second tick: %v", err)
	}
	if slaState != "TERMINAL" {
		t.Errorf("expected TERMINAL to persist, got %s", slaState)
	}
	closeTo(t, consumedHours, 6)
}

// closeTo fails the test with label unless got is within 1e-5 of want.
func closeTo(t *testing.T, got, want float64) {
	t.Helper()
	if got < want-1e-5 || got > want+1e-5 {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestRunTickOnceSurfacesAndReclassifiesUnknownStatuses is a
// scheduler-level regression: a status absent from taxonomy.statuses must
// be surfaced in unknown_statuses (for GET /metrics/overview to warn on),
// and must disappear again once the taxonomy is updated to recognize it —
// the table tracks "unknown right now", not "ever seen".
func TestRunTickOnceSurfacesAndReclassifiesUnknownStatuses(t *testing.T) {
	pool := testPool(t)
	issueID, _ := seedIngestedIssue(t, pool)
	runtime := ingest.BuildRuntimeConfig(tickTestAppConfig)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `UPDATE issues SET current_status = 'Mystery Column' WHERE id = $1`, issueID); err != nil {
		t.Fatalf("set unknown status: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM unknown_statuses WHERE status = 'Mystery Column'`)
	})

	now, _ := time.Parse(time.RFC3339, "2026-01-02T00:00:00Z")
	if _, err := RunTickOnce(ctx, pool, runtime, now); err != nil {
		t.Fatalf("RunTickOnce: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT occurrence_count FROM unknown_statuses WHERE status = 'Mystery Column'`).Scan(&count); err != nil {
		t.Fatalf("query unknown_statuses: %v", err)
	}
	if count < 1 {
		t.Errorf("expected occurrence_count >= 1, got %d", count)
	}

	// Reclassify: taxonomy now recognizes the status -> it must be dropped
	// from unknown_statuses on the very next tick.
	reclassifiedCfg := *tickTestAppConfig
	reclassifiedCfg.Taxonomy.Statuses = append(append([]config.StatusEntry{}, tickTestAppConfig.Taxonomy.Statuses...),
		config.StatusEntry{Name: "Mystery Column", Category: config.CategoryOther, AccruesSla: false})
	reclassifiedRuntime := ingest.BuildRuntimeConfig(&reclassifiedCfg)
	if _, err := RunTickOnce(ctx, pool, reclassifiedRuntime, now.Add(time.Hour)); err != nil {
		t.Fatalf("second RunTickOnce: %v", err)
	}

	var stillThere int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM unknown_statuses WHERE status = 'Mystery Column'`).Scan(&stillThere); err != nil {
		t.Fatalf("query unknown_statuses after reclassify: %v", err)
	}
	if stillThere != 0 {
		t.Errorf("expected 'Mystery Column' dropped after reclassification, got %d rows", stillThere)
	}
}
