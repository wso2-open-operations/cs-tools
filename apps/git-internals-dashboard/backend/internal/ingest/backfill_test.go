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

package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/jackc/pgx/v5/pgxpool"
)

// stubRepoIssueSearcher implements RepoIssueSearcher against a fixed table
// of per-repo responses, keyed by "owner/name".
type stubRepoIssueSearcher struct {
	nodesByRepo map[string][]github.IssueNode
	errByRepo   map[string]error
}

func (s *stubRepoIssueSearcher) FetchRepoIssues(_ context.Context, owner, name, _ string, _ int) ([]github.IssueNode, error) {
	key := owner + "/" + name
	if err, ok := s.errByRepo[key]; ok {
		return nil, err
	}
	return s.nodesByRepo[key], nil
}

// findBackfillResult returns the result for the given "owner/name" label, or
// fails the test if it's missing. The shared test database can contain
// enabled repositories beyond the ones this test creates, so results are
// looked up by label rather than by slice position or length.
func findBackfillResult(t *testing.T, results []BackfillRepoResult, repo string) BackfillRepoResult {
	t.Helper()
	for _, r := range results {
		if r.Repo == repo {
			return r
		}
	}
	t.Fatalf("expected a result for repo %q, got %+v", repo, results)
	return BackfillRepoResult{}
}

// TestBackfillIssueMetaUpdatesExistingRow verifies a pre-existing issues row
// gets its title/abt_team/opened_by populated from the stub's search result.
func TestBackfillIssueMetaUpdatesExistingRow(t *testing.T) {
	pool := testPool(t)
	ictx := setupIngestFixture(t, pool)
	ctx := context.Background()

	// Pre-existing row: ingested once with no title/meta in its body, same
	// as an issue that predates this feature.
	p := testPair(false)
	p.Node.Title = ""
	p.Node.Body = ""
	first, err := IngestIssue(ctx, pool, p, ictx)
	if err != nil {
		t.Fatalf("seed existing issue: %v", err)
	}

	body := strings.Join([]string{
		"ABT Team : Atlas",
		"Opened by : someone@wso2.com",
	}, "\n")
	stub := &stubRepoIssueSearcher{
		nodesByRepo: map[string][]github.IssueNode{
			"test-owner/test-repo-ingest": {
				{Number: 42, Title: "  A real title  ", Body: body},
			},
		},
	}

	results, err := BackfillIssueMeta(ctx, pool, stub, 90)
	if err != nil {
		t.Fatalf("BackfillIssueMeta: %v", err)
	}
	got := findBackfillResult(t, results, "test-owner/test-repo-ingest")
	if got.Err != nil {
		t.Errorf("expected no error, got %v", got.Err)
	}
	if got.Fetched != 1 {
		t.Errorf("expected Fetched=1, got %d", got.Fetched)
	}
	if got.Updated != 1 {
		t.Errorf("expected Updated=1, got %d", got.Updated)
	}

	var title, abtTeam, openedBy *string
	err = pool.QueryRow(ctx, `SELECT title, abt_team, opened_by FROM issues WHERE id = $1`, first.IssueID).
		Scan(&title, &abtTeam, &openedBy)
	if err != nil {
		t.Fatalf("read back issue: %v", err)
	}
	if title == nil || *title != "A real title" {
		t.Errorf("expected title %q, got %v", "A real title", title)
	}
	if abtTeam == nil || *abtTeam != "Atlas" {
		t.Errorf("expected abt_team %q, got %v", "Atlas", abtTeam)
	}
	if openedBy == nil || *openedBy != "someone@wso2.com" {
		t.Errorf("expected opened_by %q, got %v", "someone@wso2.com", openedBy)
	}
}

// TestBackfillIssueMetaSkipsUnknownIssueNumberWithoutInserting verifies a
// search result for an issue number with no existing row is silently
// skipped rather than inserted as a new row.
func TestBackfillIssueMetaSkipsUnknownIssueNumberWithoutInserting(t *testing.T) {
	pool := testPool(t)
	ictx := setupIngestFixture(t, pool)
	ctx := context.Background()

	before := countRows(t, pool, `SELECT count(*) FROM issues WHERE repository_id = $1`, ictx.RepositoryID)

	stub := &stubRepoIssueSearcher{
		nodesByRepo: map[string][]github.IssueNode{
			"test-owner/test-repo-ingest": {
				{Number: 9999, Title: "Never ingested", Body: "ABT Team : Ghost"},
			},
		},
	}

	results, err := BackfillIssueMeta(ctx, pool, stub, 90)
	if err != nil {
		t.Fatalf("BackfillIssueMeta: %v", err)
	}
	got := findBackfillResult(t, results, "test-owner/test-repo-ingest")
	if got.Err != nil {
		t.Errorf("expected no error, got %v", got.Err)
	}
	if got.Updated != 0 {
		t.Errorf("expected Updated=0 for an issue number with no existing row, got %d", got.Updated)
	}

	after := countRows(t, pool, `SELECT count(*) FROM issues WHERE repository_id = $1`, ictx.RepositoryID)
	if after != before {
		t.Errorf("expected issues row count to stay at %d, got %d", before, after)
	}
}

// TestBackfillIssueMetaIsolatesPerRepoFailures verifies a search failure on
// one repo is recorded on that repo's result without stopping the other
// enabled repo from being processed.
func TestBackfillIssueMetaIsolatesPerRepoFailures(t *testing.T) {
	pool := testPool(t)
	ictx := setupIngestFixture(t, pool)
	ctx := context.Background()

	createSecondEnabledRepo(t, pool)

	p := testPair(false)
	p.Node.Title = ""
	p.Node.Body = ""
	if _, err := IngestIssue(ctx, pool, p, ictx); err != nil {
		t.Fatalf("seed existing issue: %v", err)
	}

	wantErr := errors.New("github search failed")
	stub := &stubRepoIssueSearcher{
		nodesByRepo: map[string][]github.IssueNode{
			"test-owner/test-repo-ingest": {
				{Number: 42, Title: "Recovered title", Body: "ABT Team : Atlas"},
			},
		},
		errByRepo: map[string]error{
			"test-owner/test-repo-ingest-2": wantErr,
		},
	}

	results, err := BackfillIssueMeta(ctx, pool, stub, 90)
	if err != nil {
		t.Fatalf("BackfillIssueMeta: %v", err)
	}
	ok := findBackfillResult(t, results, "test-owner/test-repo-ingest")
	failed := findBackfillResult(t, results, "test-owner/test-repo-ingest-2")
	if ok.Err != nil {
		t.Errorf("expected the working repo to succeed, got %v", ok.Err)
	}
	if ok.Updated != 1 {
		t.Errorf("expected the working repo to update 1 row, got %d", ok.Updated)
	}
	if failed.Err == nil {
		t.Error("expected the failing repo to report an error")
	}
}

// createSecondEnabledRepo inserts a second enabled project/repository pair
// ("test-owner/test-repo-ingest-2") alongside the one setupIngestFixture
// creates, so a test can exercise more than one enabled repo at once.
// Registers its own cleanup.
func createSecondEnabledRepo(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	var projectID, repositoryID int32
	err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1, $2, true) RETURNING id`,
		"PVT_test_ingest_2", "Test Project 2").Scan(&projectID)
	if err != nil {
		t.Fatalf("create second project: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1, $2, $3, $4, true) RETURNING id
	`, "test-owner", "test-repo-ingest-2", `label:"Origin/CS"`, projectID).Scan(&repositoryID)
	if err != nil {
		t.Fatalf("create second repository: %v", err)
	}

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM issues WHERE repository_id = $1`, repositoryID)
		pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repositoryID)
		pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, projectID)
	})
}
