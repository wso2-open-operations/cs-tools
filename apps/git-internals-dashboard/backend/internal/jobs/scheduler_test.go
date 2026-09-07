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
	"os"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
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
	Settings: config.Settings{PossibleThreshold: 0.75, RecomputeIntervalMinutes: 10, SyncOverlapMinutes: 15, SeedSnapshotDays: 90, SeedClosedLookbackDays: 90},
}

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
				Number: i, State: "OPEN", URL: "https://github.com/test-owner/test-repo-page/issues/1",
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

// TestRunTickOnceWalksAllPagesViaKeysetPagination (AUDIT-FINDINGS B1): with
// a page size smaller than the fixture set, RunTickOnce must still process
// every issue exactly once — proving the i.id > lastID walk correctly
// advances across page boundaries rather than looping or skipping rows.
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
