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

// Idempotent seed. Resets the tables it owns, then rebuilds from real
// GitHub data (if GITHUB_TOKEN is set) or synthetic fixtures (if not). Run
// via `make seed`.
//
// PRIVACY: persists the issue title, ABT team, and a @wso2.com opened-by
// address. Assignees, labels, and other actors are not persisted.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/appconfig"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/cliutil"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/sla"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const dayMs = 24 * time.Hour

// progressInterval controls how often the slow per-issue loops (GitHub
// detail fetch, DB ingest + snapshot backfill) print a heartbeat. Neither
// loop otherwise prints anything between the per-repo issue count and its
// completion, which reads as "stuck" for large repos.
const progressInterval = 25

var interIssueDelay = 150 * time.Millisecond // courtesy gap for the secondary rate limiter; overridable by tests

// rateLimitSuffix returns ", quota: N remaining (resets HH:MM:SS)" for a
// progress log line, or "" when client is nil (synthetic fixtures — no
// GitHub calls happen) or no response has reported a quota yet. GitHub's
// resetAt is trimmed to time-of-day since it's always today or minutes away.
func rateLimitSuffix(client github.Client) string {
	if client == nil {
		return ""
	}
	remaining, resetAt, ok := client.RateLimitRemaining()
	if !ok {
		return ""
	}
	resetLabel := resetAt
	if t, err := time.Parse(time.RFC3339, resetAt); err == nil {
		resetLabel = t.Local().Format("15:04:05")
	}
	return fmt.Sprintf(", quota: %d remaining (resets %s)", remaining, resetLabel)
}

// main runs one idempotent seed pass: reset, config sync, then ingest every
// configured repo's issues (real GitHub data if GITHUB_TOKEN is set,
// synthetic fixtures otherwise), backfilling daily sla_snapshots as it goes.
func main() {
	cliutil.LoadDotEnv(".env")

	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	strictTaxonomy := strings.TrimSpace(os.Getenv("SEED_STRICT_TAXONOMY")) == "1"

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	appCfg, err := appconfig.Load()
	if err != nil {
		fatal("invalid app-config.yaml", err)
	}
	// Boot-only: must run before any github.Client is constructed below.
	github.Apply(appCfg.GitHub)
	interIssueDelay = time.Duration(appCfg.Seed.InterIssueDelayMs) * time.Millisecond

	app, err := config.Load()
	if err != nil {
		fatal("invalid sla-config.yaml", err)
	}
	runtime := ingest.BuildRuntimeConfig(app)

	pool, err := db.NewPoolWithConfig(ctx, cliutil.MustEnv("DATABASE_URL"), appCfg.Database)
	if err != nil {
		fatal("failed to connect to postgres", err)
	}
	defer pool.Close()

	var client github.Client
	mode := "SYNTHETIC fixtures (no GITHUB_TOKEN)"
	if token != "" {
		client = github.NewClient(token)
		mode = "REAL GitHub data"
	}

	repoNames := make([]string, len(app.Repos))
	for i, r := range app.Repos {
		repoNames[i] = r.Owner + "/" + r.Name
	}
	fmt.Printf("\n[seed] mode = %s\n", mode)
	fmt.Printf("[seed] repos: %s\n", strings.Join(repoNames, ", "))

	fmt.Println("[seed] resetting seeded tables...")
	if err := resetSeededTables(ctx, pool); err != nil {
		fatal("failed to reset seeded tables", err)
	}

	fmt.Println("[seed] syncing config to db...")
	syncStart := time.Now()
	syncSummary, err := db.SyncConfigToDB(ctx, pool, app)
	if err != nil {
		fatal("config sync failed", err)
	}
	fmt.Printf("[seed] config sync: %d repos active, %d disabled (%s)\n", syncSummary.ActiveRepos, syncSummary.DisabledRepos, time.Since(syncStart).Round(time.Millisecond))

	now := time.Now().UTC()
	stateCounts := map[string]int{}
	unknownStatuses := map[string]int{}
	totalIssues := 0

	for repoIndex, r := range app.Repos {
		var repoID, slaProjectID int32
		err := pool.QueryRow(ctx, `SELECT id, sla_project_id FROM repositories WHERE owner = $1 AND name = $2`, r.Owner, r.Name).
			Scan(&repoID, &slaProjectID)
		if err != nil {
			fatal(fmt.Sprintf("repository %s/%s not found — config sync should have created it", r.Owner, r.Name), err)
		}

		gatherStart := time.Now()
		pairs, err := gatherRepoIssues(ctx, client, now, r, repoIndex, app.Settings.SeedClosedLookbackDays)
		if err != nil {
			fatal(fmt.Sprintf("failed to gather issues for %s/%s", r.Owner, r.Name), err)
		}
		fmt.Printf("[seed]   %s/%s: %d issues (gathered in %s%s)\n", r.Owner, r.Name, len(pairs), time.Since(gatherStart).Round(time.Second), rateLimitSuffix(client))

		source := "synthetic"
		if token != "" {
			source = "github"
		}

		fmt.Printf("[seed]     ingesting %d issues + backfilling %d days of snapshots each...\n", len(pairs), app.Settings.SeedSnapshotDays)
		ingestStart := time.Now()
		for i, pair := range pairs {
			result, err := ingest.IngestIssue(ctx, pool, pair, ingest.Context{
				RepositoryID: repoID,
				SlaProjectID: slaProjectID,
				Repo:         ingest.RepoRef{Owner: r.Owner, Name: r.Name, GithubProjectID: r.GithubProjectID},
				Runtime:      runtime,
				Source:       source,
				Now:          now,
			})
			if err != nil {
				fatal(fmt.Sprintf("failed to ingest %s/%s#%d", r.Owner, r.Name, pair.Node.Number), err)
			}
			totalIssues++
			stateCounts[string(result.SlaState)]++
			for _, s := range result.UnknownStatuses {
				unknownStatuses[s]++
			}

			// Reconstruct daily snapshots by replaying the event log as of
			// each past day. Seed-only backfill: the sync never writes
			// historical snapshots, so this stays here rather than in the
			// shared ingest.
			if err := writeSnapshots(ctx, pool, pair, result, repoID, now, app.Settings.SeedSnapshotDays, runtime); err != nil {
				fatal(fmt.Sprintf("failed to backfill snapshots for %s/%s#%d", r.Owner, r.Name, pair.Node.Number), err)
			}

			if (i+1)%progressInterval == 0 || i+1 == len(pairs) {
				fmt.Printf("[seed]     ingested %d/%d issues (%s elapsed%s)\n", i+1, len(pairs), time.Since(ingestStart).Round(time.Second), rateLimitSuffix(client))
			}
		}

		if _, err := pool.Exec(ctx, `UPDATE repositories SET last_synced_at = $2, updated_at = now() WHERE id = $1`, repoID, now); err != nil {
			fatal(fmt.Sprintf("failed to update last_synced_at for %s/%s", r.Owner, r.Name), err)
		}
	}

	var snapshotCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sla_snapshots`).Scan(&snapshotCount); err != nil {
		fatal("failed to count snapshots", err)
	}

	stateCountsJSON, _ := json.Marshal(stateCounts)
	fmt.Printf("\n[seed] done.\n  issues:        %d\n  by SLA state:  %s\n  snapshots:     %d\n\n", totalIssues, stateCountsJSON, snapshotCount)

	if len(unknownStatuses) > 0 {
		fmt.Println("\n[seed] ⚠⚠⚠ UNKNOWN STATUSES — not present in the config taxonomy ⚠⚠⚠")
		fmt.Println("  status → issue occurrence count")
		for status, count := range unknownStatuses {
			fmt.Printf("  %q → %d\n", status, count)
		}
		fmt.Println("  unknown statuses default to pause + non-terminal")
		if strictTaxonomy {
			fmt.Println("[seed] SEED_STRICT_TAXONOMY=1 — failing due to unknown statuses above.")
			os.Exit(1)
		}
	}
}

// resetSeededTables clears the tables the seed owns, FK-safe order (children
// before parents). Repositories/projects are file-synced derived tables —
// db.SyncConfigToDB owns their lifecycle, so they are not reset here.
func resetSeededTables(ctx context.Context, pool *pgxpool.Pool) error {
	for _, table := range []string{"sla_snapshots", "issue_sla", "issue_status_events", "issues", "sync_runs"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("delete from %s: %w", table, err)
		}
	}
	return nil
}

// gatherRepoIssues returns real GitHub data when client is non-nil, or
// deterministic synthetic fixtures otherwise.
func gatherRepoIssues(ctx context.Context, client github.Client, now time.Time, r config.RepoEntry, repoIndex, closedLookbackDays int) ([]ingest.Pair, error) {
	if client == nil {
		return syntheticRepoIssues(now, r, repoIndex), nil
	}

	nodes, err := client.FetchRepoIssues(ctx, r.Owner, r.Name, r.IssueQuery, closedLookbackDays)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[seed]     %s/%s: fetching details for %d issues from GitHub...\n", r.Owner, r.Name, len(nodes))
	fetchStart := time.Now()
	out := make([]ingest.Pair, 0, len(nodes))
	for i, node := range nodes {
		detail, err := client.FetchIssueDetail(ctx, r.Owner, r.Name, node.Number)
		if err != nil {
			return nil, err
		}
		if detail != nil {
			out = append(out, ingest.Pair{Node: node, Detail: *detail})
		}
		if (i+1)%progressInterval == 0 || i+1 == len(nodes) {
			fmt.Printf("[seed]     fetched %d/%d issue details (%s elapsed%s)\n", i+1, len(nodes), time.Since(fetchStart).Round(time.Second), rateLimitSuffix(client))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interIssueDelay):
		}
	}
	return out, nil
}

// startOfUTCDay returns t truncated to 00:00:00.000 UTC on its own day.
func startOfUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// endOfUTCDay returns t's day at 23:59:59.999 UTC.
func endOfUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 23, 59, 59, 999_000_000, time.UTC)
}

// fatal logs msg and err to stderr and exits the process with status 1.
func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "[seed] FAILED: %s: %v\n", msg, err)
	os.Exit(1)
}

// writeSnapshots reconstructs one issue's daily sla_snapshots rows by
// replaying its reconciled event log (result.SlaEvents) as of each past
// day's end, from max(the issue's first event, the snapshotDays window
// start) through today. Never touched again after the seed writes it — the
// recompute scheduler only ever upserts *today's* row.
func writeSnapshots(ctx context.Context, pool *pgxpool.Pool, pair ingest.Pair, result ingest.Result, repositoryID int32, now time.Time, snapshotDays int, runtime *ingest.RuntimeConfig) error {
	firstMs := now
	if len(result.SlaEvents) > 0 {
		firstMs = result.SlaEvents[0].OccurredAt
	} else if t, err := time.Parse(time.RFC3339, pair.Node.CreatedAt); err == nil {
		firstMs = t
	}
	windowStart := now.Add(-time.Duration(snapshotDays-1) * dayMs)
	start := firstMs
	if windowStart.After(start) {
		start = windowStart
	}
	day := startOfUTCDay(start)
	today := startOfUTCDay(now)

	closed := pair.Node.State == "CLOSED"
	var closedAt *time.Time
	if pair.Node.ClosedAt != nil {
		if t, err := time.Parse(time.RFC3339, *pair.Node.ClosedAt); err == nil {
			closedAt = &t
		}
	}

	// Batched: one round trip for the whole snapshot window instead of one
	// Exec per day (up to snapshotDays, default 90) — same win as
	// ingest.IngestIssue's event-log batching, and the dominant cost when the
	// seed targets a remote DB (each unbatched round trip pays full network
	// latency).
	batch := &pgx.Batch{}
	days := make([]time.Time, 0, snapshotDays)
	for d := day; !d.After(today); d = d.Add(dayMs) {
		through := endOfUTCDay(d)
		// Days on/after closure replay with the closure-capped clock, same
		// as the live projection: consumption freezes and the state reports
		// TERMINAL from the day of closure onward.
		cfg, effectiveThrough := sla.AdjustForClosure(runtime.Cfg, through, closed, closedAt)
		statusThatDay := sla.StatusAsOf(result.SlaEvents, effectiveThrough)
		r := sla.ComputeSla(result.Priority, result.SlaEvents, statusThatDay, cfg, effectiveThrough)
		days = append(days, d)
		batch.Queue(`
			INSERT INTO sla_snapshots (
				snapshot_date, issue_id, repository_id, priority, current_status,
				budget_hours, consumed_hours, remaining_hours, pct_consumed, sla_state, sla_running
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, d, result.IssueID, repositoryID, result.Priority, statusThatDay,
			r.BudgetHours, r.ConsumedHours, r.RemainingHours, r.PctConsumed, string(r.SlaState), r.SlaRunning)
	}

	br := pool.SendBatch(ctx, batch)
	for _, d := range days {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return fmt.Errorf("insert sla_snapshot for %s: %w", d.Format("2006-01-02"), err)
		}
	}
	return br.Close()
}
