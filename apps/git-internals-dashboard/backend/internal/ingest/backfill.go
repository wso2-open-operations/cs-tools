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
	"fmt"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepoIssueSearcher is the narrow GitHub surface the backfill needs.
type RepoIssueSearcher interface {
	FetchRepoIssues(ctx context.Context, owner, name, issueQuery string, closedLookbackDays int) ([]github.IssueNode, error)
}

// BackfillRepoResult is one repository's outcome from a backfill run.
type BackfillRepoResult struct {
	Repo    string // "owner/name"
	Fetched int    // issues returned by search
	Updated int    // existing rows updated (RowsAffected summed across the batch)
	Err     error
}

// backfillRepoRow is the subset of a repositories row BackfillIssueMeta
// needs to run one repo's search and scope its updates.
type backfillRepoRow struct {
	ID         int32
	Owner      string
	Name       string
	IssueQuery string
}

// BackfillIssueMeta updates only issues.title/abt_team/opened_by for rows
// that already exist, deriving each value from a GitHub search pass exactly
// like ingest does — no per-issue detail calls, no inserts, and no changes
// to issue_sla/issue_status_events/sla_snapshots. Safe to re-run: an issue
// whose body no longer carries a value gets that column set back to NULL,
// same overwrite behavior as a normal ingest. A failure fetching or
// updating one repository is recorded on that repository's result and does
// not stop the remaining repositories.
func BackfillIssueMeta(ctx context.Context, pool *pgxpool.Pool, gh RepoIssueSearcher, closedLookbackDays int) ([]BackfillRepoResult, error) {
	repos, err := fetchEnabledRepoRows(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("backfill: list enabled repos: %w", err)
	}

	results := make([]BackfillRepoResult, 0, len(repos))
	for _, repo := range repos {
		repoLabel := repo.Owner + "/" + repo.Name

		nodes, err := gh.FetchRepoIssues(ctx, repo.Owner, repo.Name, repo.IssueQuery, closedLookbackDays)
		if err != nil {
			results = append(results, BackfillRepoResult{Repo: repoLabel, Err: err})
			continue
		}

		updated, err := backfillRepoIssues(ctx, pool, repo.ID, nodes)
		if err != nil {
			results = append(results, BackfillRepoResult{Repo: repoLabel, Fetched: len(nodes), Err: err})
			continue
		}

		results = append(results, BackfillRepoResult{Repo: repoLabel, Fetched: len(nodes), Updated: updated})
	}

	return results, nil
}

// fetchEnabledRepoRows returns every enabled repository's id/owner/name/
// issue_query, ordered by id for deterministic output across runs.
func fetchEnabledRepoRows(ctx context.Context, pool *pgxpool.Pool) ([]backfillRepoRow, error) {
	rows, err := pool.Query(ctx, `SELECT id, owner, name, issue_query FROM repositories WHERE enabled = true ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []backfillRepoRow
	for rows.Next() {
		var r backfillRepoRow
		if err := rows.Scan(&r.ID, &r.Owner, &r.Name, &r.IssueQuery); err != nil {
			return nil, err
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

// backfillRepoIssues derives title/abt_team/opened_by for each returned
// search node and updates only the issues rows that already exist for
// repositoryID, batched into one round trip. A node whose github_number has
// no matching row (not yet ingested by any prior seed/sync) matches zero
// rows and is silently skipped — this never inserts.
func backfillRepoIssues(ctx context.Context, pool *pgxpool.Pool, repositoryID int32, nodes []github.IssueNode) (int, error) {
	if len(nodes) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, node := range nodes {
		title := normalizeTitle(node.Title)
		meta := ExtractIssueMeta(node.Body) // node.Body is discarded after this line — never logged, persisted elsewhere, or returned
		batch.Queue(`
			UPDATE issues SET title = $3, abt_team = $4, opened_by = $5
			WHERE repository_id = $1 AND github_number = $2
		`, repositoryID, node.Number, title, meta.ABTTeam, meta.OpenedBy)
	}

	br := pool.SendBatch(ctx, batch)
	var updated int
	for range nodes {
		tag, err := br.Exec()
		if err != nil {
			_ = br.Close()
			return updated, err
		}
		updated += int(tag.RowsAffected())
	}
	if err := br.Close(); err != nil {
		return updated, err
	}
	return updated, nil
}
