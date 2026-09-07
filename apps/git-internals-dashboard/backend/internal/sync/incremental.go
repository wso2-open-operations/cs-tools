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

// Package sync is the manual incremental sync (SPEC §8.3, port of v3's
// src/server/db/sync/incremental.ts): one GitHub fetch pass per enabled
// repository, using the same shared ingest as the seed. No scheduling
// concerns here — that's internal/jobs.
//
// Known, documented limitation: an issue that stops matching a repo's
// issueQuery (e.g. a label removed) no longer appears in incremental results
// and its row goes stale until the next full reseed reconciles it.
package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sanitizeSyncErrorMaxLen bounds the classification persisted to
// sync_runs.error and returned via the API (AUDIT-FINDINGS A2) — well over
// any classification this package produces, kept only as a hard cap.
const sanitizeSyncErrorMaxLen = 200

// Overridable only by tests, so the courtesy pacing between issues doesn't
// make tests take real wall-clock seconds.
var interIssueDelay = 150 * time.Millisecond

// GithubClient is the minimal GitHub surface incremental sync needs, kept
// narrow (search + detail only, no FetchRepoIssues) so tests can stub it
// without a network call.
type GithubClient interface {
	SearchAll(ctx context.Context, q string) ([]github.IssueNode, error)
	FetchIssueDetail(ctx context.Context, owner, name string, number int) (*github.IssueDetail, error)
}

// RepoResult is one repository's outcome from a sync run.
type RepoResult struct {
	Repo            string `json:"repo"`   // "owner/name"
	Status          string `json:"status"` // "success" | "error"
	IssuesProcessed int    `json:"issuesProcessed"`
	EventsInserted  int    `json:"eventsInserted"`
	Error           string `json:"error,omitempty"`
}

// Summary is the wire shape POST /sync/runs returns (SPEC §6.8).
type Summary struct {
	StartedAt  time.Time    `json:"startedAt"`
	FinishedAt time.Time    `json:"finishedAt"`
	Repos      []RepoResult `json:"repos"`
}

type repoRow struct {
	ID              int32
	Owner           string
	Name            string
	IssueQuery      string
	SlaProjectID    *int32
	GithubProjectID *string
	LastSyncedAt    *time.Time
}

// Run performs one incremental sync pass over every enabled repository.
// Per-repo failures are isolated: a failing repo's watermark is left
// untouched and an error sync_runs row is recorded, but the loop continues
// with the remaining repos (SPEC §8.3). Callers (the POST /sync/runs
// handler) are responsible for the GITHUB_TOKEN presence check and for
// constructing client — this package has no env-var concerns of its own, so
// it stays testable against a stubbed GithubClient with no network at all.
func Run(ctx context.Context, pool *pgxpool.Pool, client GithubClient, cfg *config.AppConfig, runtime *ingest.RuntimeConfig) (Summary, error) {
	overlap := time.Duration(cfg.Settings.SyncOverlapMinutes) * time.Minute
	lookback := time.Duration(cfg.Settings.SeedClosedLookbackDays) * 24 * time.Hour

	startedAt := time.Now().UTC()

	repos, err := fetchEnabledRepos(ctx, pool)
	if err != nil {
		return Summary{}, fmt.Errorf("sync: list enabled repos: %w", err)
	}

	results := make([]RepoResult, 0, len(repos))
	for _, repo := range repos {
		repoLabel := repo.Owner + "/" + repo.Name
		// Captured before any fetch — advances the watermark to "now" on
		// success, so anything updated mid-run is re-covered by the next
		// sync's overlap.
		syncStartedAt := time.Now().UTC()
		var since time.Time
		if repo.LastSyncedAt != nil {
			since = repo.LastSyncedAt.Add(-overlap)
		} else {
			since = syncStartedAt.Add(-lookback).Add(-overlap)
		}

		issuesProcessed, eventsInserted, syncErr := syncOneRepo(ctx, pool, client, runtime, repo, repoLabel, since, syncStartedAt)
		if syncErr == nil {
			if err := advanceWatermark(ctx, pool, repo.ID, syncStartedAt); err != nil {
				return Summary{}, err
			}
			if err := insertSyncRun(ctx, pool, repo.ID, since, syncStartedAt, "success", issuesProcessed, ""); err != nil {
				return Summary{}, err
			}
			results = append(results, RepoResult{Repo: repoLabel, Status: "success", IssuesProcessed: issuesProcessed, EventsInserted: eventsInserted})
			continue
		}

		// Full detail (possibly including raw GitHub response body, GraphQL
		// error text, or a wrapped network/DB error chain) is logged here and
		// only here — sync_runs.error and the API response carry the
		// sanitized classification (AUDIT-FINDINGS A2).
		slog.ErrorContext(ctx, "sync: repo failed", "repo", repoLabel, "err", syncErr)
		message := sanitizeSyncError(syncErr)
		// Do not advance this repo's watermark — continue with the rest.
		if err := insertSyncRun(ctx, pool, repo.ID, since, syncStartedAt, "error", 0, message); err != nil {
			return Summary{}, err
		}
		results = append(results, RepoResult{Repo: repoLabel, Status: "error", Error: message})
	}

	return Summary{StartedAt: startedAt, FinishedAt: time.Now().UTC(), Repos: results}, nil
}

func syncOneRepo(ctx context.Context, pool *pgxpool.Pool, client GithubClient, runtime *ingest.RuntimeConfig, repo repoRow, repoLabel string, since, now time.Time) (issuesProcessed, eventsInserted int, err error) {
	if repo.SlaProjectID == nil || repo.GithubProjectID == nil {
		return 0, 0, fmt.Errorf("repository %s has no linked project — config sync should have set this", repoLabel)
	}

	q := fmt.Sprintf("repo:%s/%s %s updated:>=%s", repo.Owner, repo.Name, repo.IssueQuery, formatISO(since))
	nodes, err := client.SearchAll(ctx, q)
	if err != nil {
		return 0, 0, fmt.Errorf("github search: %w", err)
	}

	for _, node := range nodes {
		detail, err := client.FetchIssueDetail(ctx, repo.Owner, repo.Name, node.Number)
		if err != nil {
			return issuesProcessed, eventsInserted, fmt.Errorf("github issue detail: %w", err)
		}
		if detail != nil {
			result, err := ingest.IngestIssue(ctx, pool, ingest.Pair{Node: node, Detail: *detail}, ingest.Context{
				RepositoryID: repo.ID,
				SlaProjectID: *repo.SlaProjectID,
				Repo:         ingest.RepoRef{Owner: repo.Owner, Name: repo.Name, GithubProjectID: *repo.GithubProjectID},
				Runtime:      runtime,
				Source:       "github",
				Now:          now,
			})
			if err != nil {
				return issuesProcessed, eventsInserted, fmt.Errorf("ingest: %w", err)
			}
			issuesProcessed++
			eventsInserted += result.EventsInserted
		}
		if err := github.SleepOrDone(ctx, interIssueDelay); err != nil { // courtesy gap for the secondary rate limiter
			return issuesProcessed, eventsInserted, err
		}
	}
	return issuesProcessed, eventsInserted, nil
}

// sanitizeSyncError reduces a per-repo sync failure to a short, stable
// classification safe to persist in sync_runs.error and return via
// GET /sync/status / POST /sync/runs (AUDIT-FINDINGS A2). The raw err —
// which may carry up to 500 bytes of GitHub response body, a GraphQL error
// message, or a wrapped network/DB error chain — must be logged separately
// by the caller and is never included here.
func sanitizeSyncError(err error) string {
	var apiErr *github.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Public()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "sync canceled"
	}

	msg := err.Error()
	for _, stage := range []string{"github search", "github issue detail", "ingest"} {
		if strings.HasPrefix(msg, stage+": ") {
			return stage + " failed"
		}
	}

	// Anything else (e.g. the "repository has no linked project" config
	// error) is entirely internally-derived — no external response body or
	// error chain — so it's safe to pass through, just length-capped.
	if len(msg) > sanitizeSyncErrorMaxLen {
		msg = msg[:sanitizeSyncErrorMaxLen]
	}
	return msg
}

// formatISO matches JavaScript's Date.prototype.toISOString() format
// exactly (millisecond precision, always UTC "Z"), since the search query
// string is a plain string comparison against what a human/GitHub expects.
func formatISO(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func fetchEnabledRepos(ctx context.Context, pool *pgxpool.Pool) ([]repoRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT r.id, r.owner, r.name, r.issue_query, r.sla_project_id, p.github_project_id, r.last_synced_at
		FROM repositories r
		LEFT JOIN projects p ON p.id = r.sla_project_id
		WHERE r.enabled = true
		ORDER BY r.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []repoRow
	for rows.Next() {
		var r repoRow
		if err := rows.Scan(&r.ID, &r.Owner, &r.Name, &r.IssueQuery, &r.SlaProjectID, &r.GithubProjectID, &r.LastSyncedAt); err != nil {
			return nil, err
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

func advanceWatermark(ctx context.Context, pool *pgxpool.Pool, repositoryID int32, syncStartedAt time.Time) error {
	_, err := pool.Exec(ctx, `UPDATE repositories SET last_synced_at = $2, updated_at = now() WHERE id = $1`, repositoryID, syncStartedAt)
	return err
}

func insertSyncRun(ctx context.Context, pool *pgxpool.Pool, repositoryID int32, since, startedAt time.Time, status string, issuesProcessed int, errMessage string) error {
	var errArg *string
	if errMessage != "" {
		errArg = &errMessage
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO sync_runs (repository_id, kind, since_ts, started_at, finished_at, status, issues_processed, error)
		VALUES ($1, 'manual', $2, $3, now(), $4, $5, $6)
	`, repositoryID, since, startedAt, status, issuesProcessed, errArg)
	return err
}
