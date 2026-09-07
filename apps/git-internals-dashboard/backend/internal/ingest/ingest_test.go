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

// Port of v3's src/server/db/ingest.test.ts — DB-backed against the
// docker-composed Postgres (SPEC §15); skips cleanly with a log line when
// DATABASE_URL is unreachable so `go test` still runs without Docker.
package ingest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
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

var testAppConfig = &config.AppConfig{
	Repos: []config.RepoEntry{
		{Owner: "test-owner", Name: "test-repo-ingest", GithubProjectID: "PVT_test_ingest", ProjectTitle: "Test Project", IssueQuery: `label:"Origin/CS"`},
	},
	Taxonomy: config.Taxonomy{
		Statuses: []config.StatusEntry{
			{Name: "Open", Category: config.CategoryProductSide, AccruesSla: true, IsTerminal: false, SortOrder: 10},
			{Name: "In Progress", Category: config.CategoryProductSide, AccruesSla: true, IsTerminal: false, SortOrder: 20},
			{Name: "Resolved", Category: config.CategoryOther, AccruesSla: false, IsTerminal: true, SortOrder: 30},
		},
		Aliases: []config.AliasEntry{},
	},
	Budgets: []config.BudgetEntry{
		{Priority: "Critical(P1)", BudgetHours: 24, Coverage: config.Coverage24x7, Rank: 1},
	},
	Settings: config.Settings{
		PossibleThreshold:        0.75,
		RecomputeIntervalMinutes: 10,
		SyncOverlapMinutes:       15,
		SnapshotHourUtc:          0,
		SeedSnapshotDays:         90,
		SeedClosedLookbackDays:   90,
	},
}

func testPair(withLeadingEvent bool) Pair {
	events := []github.StatusEvent{}
	if withLeadingEvent {
		events = []github.StatusEvent{
			{CreatedAt: "2026-01-05T00:00:00.000Z", PreviousStatus: strp("Open"), Status: strp("In Progress")},
		}
	}
	return Pair{
		Node: github.IssueNode{
			Number:    42,
			State:     "OPEN",
			URL:       "https://github.com/test-owner/test-repo-ingest/issues/42",
			CreatedAt: "2026-01-01T00:00:00.000Z",
			UpdatedAt: "2026-01-10T12:00:00.000Z",
			ClosedAt:  nil,
			Labels:    []string{"Priority/Critical(P1)"},
		},
		Detail: github.IssueDetail{
			Number: 42,
			Events: events,
			ProjectStatuses: []github.ProjectStatus{
				{
					ProjectID:       "PVT_test_ingest",
					Status:          strp("In Progress"),
					StatusUpdatedAt: strp("2026-01-05T00:00:00.000Z"),
					ItemCreatedAt:   strp("2026-01-01T00:00:00.000Z"),
				},
			},
		},
	}
}

// setupIngestFixture creates the project/repository rows IngestIssue needs
// and returns a ready-to-use Context plus a cleanup that removes everything
// it created (including any issues/events/sla rows a test ingests).
func setupIngestFixture(t *testing.T, pool *pgxpool.Pool) Context {
	t.Helper()
	ctx := context.Background()

	var projectID, repositoryID int32
	err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1, $2, true) RETURNING id`,
		"PVT_test_ingest", "Test Project").Scan(&projectID)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1, $2, $3, $4, true) RETURNING id
	`, "test-owner", "test-repo-ingest", `label:"Origin/CS"`, projectID).Scan(&repositoryID)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM issue_sla WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repositoryID)
		pool.Exec(context.Background(), `DELETE FROM issue_status_events WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repositoryID)
		pool.Exec(context.Background(), `DELETE FROM issues WHERE repository_id = $1`, repositoryID)
		pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repositoryID)
		pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, projectID)
	})

	runtime := BuildRuntimeConfig(testAppConfig)
	now, _ := time.Parse(time.RFC3339, "2026-01-10T12:00:00.000Z")

	return Context{
		RepositoryID: repositoryID,
		SlaProjectID: projectID,
		Repo:         RepoRef{Owner: "test-owner", Name: "test-repo-ingest", GithubProjectID: "PVT_test_ingest"},
		Runtime:      runtime,
		Source:       "github",
		Now:          now,
	}
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	return n
}

func TestIngestIssueIsIdempotentAcrossRepeatedRuns(t *testing.T) {
	pool := testPool(t)
	ictx := setupIngestFixture(t, pool)
	ctx := context.Background()
	p := testPair(true)

	first, err := IngestIssue(ctx, pool, p, ictx)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if !first.Created {
		t.Error("expected first ingest to report created=true")
	}
	if first.EventsInserted != 2 { // leading "derived" + the one real event
		t.Errorf("expected 2 events inserted, got %d", first.EventsInserted)
	}

	second, err := IngestIssue(ctx, pool, p, ictx)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if second.Created {
		t.Error("expected second ingest to report created=false")
	}
	if second.IssueID != first.IssueID {
		t.Errorf("expected same issue id, got %d vs %d", second.IssueID, first.IssueID)
	}
	if second.EventsInserted != 0 {
		t.Errorf("expected 0 new events on re-ingest (dedupe), got %d", second.EventsInserted)
	}

	if got := countRows(t, pool, `SELECT count(*) FROM issues WHERE repository_id = $1`, ictx.RepositoryID); got != 1 {
		t.Errorf("expected 1 issue row, got %d", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM issue_sla WHERE issue_id = $1`, first.IssueID); got != 1 {
		t.Errorf("expected 1 issue_sla row, got %d", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM issue_status_events WHERE issue_id = $1`, first.IssueID); got != 2 {
		t.Errorf("expected 2 issue_status_events rows, got %d", got)
	}
}

func TestIngestIssueCreatesLeadingDerivedEventWithStableDedupeKey(t *testing.T) {
	pool := testPool(t)
	ictx := setupIngestFixture(t, pool)
	ctx := context.Background()
	p := testPair(true)

	result, err := IngestIssue(ctx, pool, p, ictx)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	rows, err := pool.Query(ctx, `SELECT source, previous_status, status, dedupe_key FROM issue_status_events WHERE issue_id = $1 ORDER BY occurred_at ASC`, result.IssueID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	type row struct {
		Source, DedupeKey      string
		PreviousStatus, Status *string
	}
	var events []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.Source, &r.PreviousStatus, &r.Status, &r.DedupeKey); err != nil {
			t.Fatalf("scan: %v", err)
		}
		events = append(events, r)
	}
	rows.Close()

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Source != "derived" {
		t.Errorf("expected first event source=derived, got %s", events[0].Source)
	}
	if events[0].PreviousStatus != nil {
		t.Errorf("expected first event previousStatus=nil, got %v", *events[0].PreviousStatus)
	}
	if events[0].Status == nil || *events[0].Status != "Open" {
		t.Errorf("expected first event status=Open (firstEvent.previousStatus), got %v", events[0].Status)
	}
	if events[1].Source != "github" {
		t.Errorf("expected second event source=github, got %s", events[1].Source)
	}

	firstDedupeKey := events[0].DedupeKey

	// Re-ingesting must reproduce the identical dedupeKey, not a new row.
	if _, err := IngestIssue(ctx, pool, p, ictx); err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM issue_status_events WHERE issue_id = $1`, result.IssueID); got != 2 {
		t.Fatalf("expected still 2 events after re-ingest, got %d", got)
	}
	var dedupeAfter string
	if err := pool.QueryRow(ctx, `SELECT dedupe_key FROM issue_status_events WHERE issue_id = $1 ORDER BY occurred_at ASC LIMIT 1`, result.IssueID).Scan(&dedupeAfter); err != nil {
		t.Fatalf("query dedupe key: %v", err)
	}
	if dedupeAfter != firstDedupeKey {
		t.Errorf("expected stable dedupe key %q, got %q", firstDedupeKey, dedupeAfter)
	}
}

func TestIngestIssueDoesNotSynthesizeLeadingEventWhenTimelineEmpty(t *testing.T) {
	pool := testPool(t)
	ictx := setupIngestFixture(t, pool)
	ctx := context.Background()

	result, err := IngestIssue(ctx, pool, testPair(false), ictx)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if result.EventsInserted != 0 {
		t.Errorf("expected 0 events inserted, got %d", result.EventsInserted)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM issue_status_events WHERE issue_id = $1`, result.IssueID); got != 0 {
		t.Errorf("expected 0 issue_status_events rows, got %d", got)
	}
}
