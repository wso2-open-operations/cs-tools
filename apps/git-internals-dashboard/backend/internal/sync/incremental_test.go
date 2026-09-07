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

// Port of v3's src/server/db/sync/incremental.test.ts behaviors — watermark
// computation, per-repo error isolation, and idempotency — against a stubbed
// GithubClient rather than a mocked module, since Go has no module-mocking
// equivalent. The token-presence check (v3's SyncTokenMissingError) moved to
// the POST /sync/runs handler (see this package's doc comment); it has no
// counterpart here. Likewise, sync_runs carries no triggeredBy column in
// this schema (SPEC §4) — this backend is authless (D7), so there is no
// caller identity to record; the corresponding v3 tests have no counterpart.
package sync

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
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

func strp(s string) *string { return &s }

var testAppConfig = &config.AppConfig{
	Taxonomy: config.Taxonomy{
		Statuses: []config.StatusEntry{
			{Name: "Open", Category: config.CategoryProductSide, AccruesSla: true},
			{Name: "In Progress", Category: config.CategoryProductSide, AccruesSla: true},
		},
		Aliases: []config.AliasEntry{},
	},
	Budgets:  []config.BudgetEntry{{Priority: "Critical(P1)", BudgetHours: 24, Coverage: config.Coverage24x7, Rank: 1}},
	Settings: config.Settings{PossibleThreshold: 0.75, RecomputeIntervalMinutes: 10, SyncOverlapMinutes: 15, SeedSnapshotDays: 90, SeedClosedLookbackDays: 90},
}

// stubClient is a hand-rolled GithubClient stub — search/detail behavior is
// driven by test-supplied functions so each test controls exactly what the
// "GitHub API" returns without any network call.
type stubClient struct {
	search func(ctx context.Context, q string) ([]github.IssueNode, error)
	detail func(ctx context.Context, owner, name string, number int) (*github.IssueDetail, error)
}

func (s *stubClient) SearchAll(ctx context.Context, q string) ([]github.IssueNode, error) {
	return s.search(ctx, q)
}

func (s *stubClient) FetchIssueDetail(ctx context.Context, owner, name string, number int) (*github.IssueDetail, error) {
	return s.detail(ctx, owner, name, number)
}

func fixtureNode(number int) github.IssueNode {
	return github.IssueNode{
		Number: number, State: "OPEN",
		URL:       "https://github.com/test-sync/repo/issues/1",
		CreatedAt: "2026-01-01T00:00:00.000Z", UpdatedAt: "2026-01-05T00:00:00.000Z",
		Labels: []string{"Priority/Critical(P1)"},
	}
}

func fixtureDetail(number int, projectID string) *github.IssueDetail {
	return &github.IssueDetail{
		Number: number,
		Events: []github.StatusEvent{
			{CreatedAt: "2026-01-02T00:00:00.000Z", PreviousStatus: strp("Open"), Status: strp("In Progress")},
		},
		ProjectStatuses: []github.ProjectStatus{
			{ProjectID: projectID, Status: strp("In Progress"), StatusUpdatedAt: strp("2026-01-02T00:00:00.000Z"), ItemCreatedAt: strp("2026-01-01T00:00:00.000Z")},
		},
	}
}

// makeRepo creates a project+repository fixture and registers cleanup.
func makeRepo(t *testing.T, pool *pgxpool.Pool, name, githubProjectID string, lastSyncedAt *time.Time) (repoID, projectID int32) {
	t.Helper()
	ctx := context.Background()
	if err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1,$2,true) RETURNING id`,
		githubProjectID, name).Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled, last_synced_at)
		VALUES ($1,$2,$3,$4,true,$5) RETURNING id
	`, "test-sync", name, `label:"Origin/CS"`, projectID, lastSyncedAt).Scan(&repoID); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM sync_runs WHERE repository_id = $1`, repoID)
		pool.Exec(bg, `DELETE FROM sla_snapshots WHERE repository_id = $1`, repoID)
		pool.Exec(bg, `DELETE FROM issue_sla WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repoID)
		pool.Exec(bg, `DELETE FROM issue_status_events WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repoID)
		pool.Exec(bg, `DELETE FROM issues WHERE repository_id = $1`, repoID)
		pool.Exec(bg, `DELETE FROM repositories WHERE id = $1`, repoID)
		pool.Exec(bg, `DELETE FROM projects WHERE id = $1`, projectID)
	})
	return repoID, projectID
}

var sinceRe = regexp.MustCompile(`updated:>=(.+)$`)

func TestRunComputesWatermarkFromLastSyncedAtMinusOverlap(t *testing.T) {
	pool := testPool(t)
	lastSyncedAt := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	makeRepo(t, pool, "watermark-repo", "PVT_watermark", &lastSyncedAt)

	var capturedQuery string
	client := &stubClient{
		search: func(ctx context.Context, q string) ([]github.IssueNode, error) {
			if strings.Contains(q, "watermark-repo") {
				capturedQuery = q
			}
			return nil, nil
		},
		detail: func(ctx context.Context, owner, name string, number int) (*github.IssueDetail, error) {
			return nil, nil
		},
	}

	if _, err := Run(context.Background(), pool, client, testAppConfig, ingest.BuildRuntimeConfig(testAppConfig)); err != nil {
		t.Fatalf("Run: %v", err)
	}

	m := sinceRe.FindStringSubmatch(capturedQuery)
	if m == nil {
		t.Fatalf("expected an updated:>= clause in query %q", capturedQuery)
	}
	since, err := time.Parse("2006-01-02T15:04:05.000Z", m[1])
	if err != nil {
		t.Fatalf("parse since: %v", err)
	}
	want := lastSyncedAt.Add(-15 * time.Minute) // default syncOverlapMinutes: 15
	if !since.Equal(want) {
		t.Errorf("expected since=%v, got %v", want, since)
	}
}

func TestRunFallsBackToLookbackWindowWhenLastSyncedAtIsNil(t *testing.T) {
	pool := testPool(t)
	makeRepo(t, pool, "lookback-repo", "PVT_lookback", nil)

	var capturedQuery string
	client := &stubClient{
		search: func(ctx context.Context, q string) ([]github.IssueNode, error) {
			if strings.Contains(q, "lookback-repo") {
				capturedQuery = q
			}
			return nil, nil
		},
		detail: func(ctx context.Context, owner, name string, number int) (*github.IssueDetail, error) {
			return nil, nil
		},
	}

	before := time.Now().UTC()
	if _, err := Run(context.Background(), pool, client, testAppConfig, ingest.BuildRuntimeConfig(testAppConfig)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	after := time.Now().UTC()

	m := sinceRe.FindStringSubmatch(capturedQuery)
	if m == nil {
		t.Fatalf("expected an updated:>= clause in query %q", capturedQuery)
	}
	since, err := time.Parse("2006-01-02T15:04:05.000Z", m[1])
	if err != nil {
		t.Fatalf("parse since: %v", err)
	}
	// default settings: seedClosedLookbackDays=90, syncOverlapMinutes=15
	lowerBound := before.Add(-90*24*time.Hour - 15*time.Minute - time.Second)
	upperBound := after.Add(-90*24*time.Hour - 15*time.Minute + time.Second)
	if since.Before(lowerBound) || since.After(upperBound) {
		t.Errorf("expected since between %v and %v, got %v", lowerBound, upperBound, since)
	}
}

func TestRunIsolatesPerRepoFailures(t *testing.T) {
	pool := testPool(t)
	repoAID, _ := makeRepo(t, pool, "fail-repo", "PVT_fail", nil)
	repoBID, _ := makeRepo(t, pool, "ok-repo", "PVT_ok", nil)

	client := &stubClient{
		search: func(ctx context.Context, q string) ([]github.IssueNode, error) {
			if strings.Contains(q, "fail-repo") {
				return nil, errBoom
			}
			if strings.Contains(q, "ok-repo") {
				return []github.IssueNode{fixtureNode(1)}, nil
			}
			return nil, nil
		},
		detail: func(ctx context.Context, owner, name string, number int) (*github.IssueDetail, error) {
			return fixtureDetail(number, "PVT_ok"), nil
		},
	}

	summary, err := Run(context.Background(), pool, client, testAppConfig, ingest.BuildRuntimeConfig(testAppConfig))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var resultA, resultB *RepoResult
	for i := range summary.Repos {
		switch summary.Repos[i].Repo {
		case "test-sync/fail-repo":
			resultA = &summary.Repos[i]
		case "test-sync/ok-repo":
			resultB = &summary.Repos[i]
		}
	}
	if resultA == nil || resultA.Status != "error" {
		t.Fatalf("expected fail-repo status=error, got %+v", resultA)
	}
	if resultB == nil || resultB.Status != "success" || resultB.IssuesProcessed != 1 {
		t.Fatalf("expected ok-repo status=success issuesProcessed=1, got %+v", resultB)
	}

	var lastSyncedA, lastSyncedB *time.Time
	if err := pool.QueryRow(context.Background(), `SELECT last_synced_at FROM repositories WHERE id = $1`, repoAID).Scan(&lastSyncedA); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT last_synced_at FROM repositories WHERE id = $1`, repoBID).Scan(&lastSyncedB); err != nil {
		t.Fatal(err)
	}
	if lastSyncedA != nil {
		t.Error("expected fail-repo's watermark to remain unchanged (nil)")
	}
	if lastSyncedB == nil {
		t.Error("expected ok-repo's watermark to have advanced")
	}

	var errStatus, errMessage string
	if err := pool.QueryRow(context.Background(), `SELECT status, error FROM sync_runs WHERE repository_id = $1`, repoAID).Scan(&errStatus, &errMessage); err != nil {
		t.Fatal(err)
	}
	// AUDIT-FINDINGS A2: sync_runs.error carries a sanitized classification,
	// never the raw underlying error detail ("boom" must not leak through).
	if errStatus != "error" || errMessage != "github search failed" {
		t.Errorf("expected an error sync_runs row with sanitized error=%q, got status=%s error=%s", "github search failed", errStatus, errMessage)
	}
	if strings.Contains(errMessage, "boom") {
		t.Errorf("expected sync_runs.error not to leak raw error detail, got %q", errMessage)
	}

	var okStatus string
	var okIssuesProcessed int
	if err := pool.QueryRow(context.Background(), `SELECT status, issues_processed FROM sync_runs WHERE repository_id = $1`, repoBID).Scan(&okStatus, &okIssuesProcessed); err != nil {
		t.Fatal(err)
	}
	if okStatus != "success" || okIssuesProcessed != 1 {
		t.Errorf("expected a success sync_runs row with issuesProcessed=1, got status=%s issuesProcessed=%d", okStatus, okIssuesProcessed)
	}
}

func TestRunIsIdempotentAcrossRepeatedRuns(t *testing.T) {
	pool := testPool(t)
	makeRepo(t, pool, "idem-repo", "PVT_idem", nil)

	client := &stubClient{
		search: func(ctx context.Context, q string) ([]github.IssueNode, error) {
			if strings.Contains(q, "idem-repo") {
				return []github.IssueNode{fixtureNode(42)}, nil
			}
			return nil, nil
		},
		detail: func(ctx context.Context, owner, name string, number int) (*github.IssueDetail, error) {
			return fixtureDetail(number, "PVT_idem"), nil
		},
	}

	runtime := ingest.BuildRuntimeConfig(testAppConfig)
	first, err := Run(context.Background(), pool, client, testAppConfig, runtime)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	second, err := Run(context.Background(), pool, client, testAppConfig, runtime)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}

	firstEvents := findRepoResult(first, "test-sync/idem-repo").EventsInserted
	secondEvents := findRepoResult(second, "test-sync/idem-repo").EventsInserted
	if firstEvents <= 0 {
		t.Errorf("expected first run to insert events, got %d", firstEvents)
	}
	if secondEvents != 0 {
		t.Errorf("expected second run to insert zero new events (dedupe), got %d", secondEvents)
	}
}

func findRepoResult(s Summary, repo string) RepoResult {
	for _, r := range s.Repos {
		if r.Repo == repo {
			return r
		}
	}
	return RepoResult{}
}

var errBoom = errors.New("boom")
