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

// Idempotent seed (SPEC §9, port of v3's seed/seed.ts). Resets the tables it
// owns, then rebuilds from real GitHub data (if GITHUB_TOKEN is set) or
// synthetic fixtures (if not). Run via `make seed`.
//
// PRIVACY: persists no titles, assignees, openers, labels, or actors.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/sla"
	"github.com/jackc/pgx/v5/pgxpool"
)

const dayMs = 24 * time.Hour

var interIssueDelay = 150 * time.Millisecond // courtesy gap for the secondary rate limiter; overridable by tests

func main() {
	loadDotEnv(".env")

	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	strictTaxonomy := strings.TrimSpace(os.Getenv("SEED_STRICT_TAXONOMY")) == "1"

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := config.Load()
	if err != nil {
		fatal("invalid sla-config.yaml", err)
	}
	runtime := ingest.BuildRuntimeConfig(app)

	pool, err := db.NewPool(ctx, mustEnv("DATABASE_URL"))
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
	syncSummary, err := db.SyncConfigToDB(ctx, pool, app)
	if err != nil {
		fatal("config sync failed", err)
	}
	fmt.Printf("[seed] config sync: %d repos active, %d disabled\n", syncSummary.ActiveRepos, syncSummary.DisabledRepos)

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

		pairs, err := gatherRepoIssues(ctx, client, now, r, repoIndex, app.Settings.SeedClosedLookbackDays)
		if err != nil {
			fatal(fmt.Sprintf("failed to gather issues for %s/%s", r.Owner, r.Name), err)
		}
		fmt.Printf("[seed]   %s/%s: %d issues\n", r.Owner, r.Name, len(pairs))

		source := "synthetic"
		if token != "" {
			source = "github"
		}

		for _, pair := range pairs {
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
	out := make([]ingest.Pair, 0, len(nodes))
	for _, node := range nodes {
		detail, err := client.FetchIssueDetail(ctx, r.Owner, r.Name, node.Number)
		if err != nil {
			return nil, err
		}
		if detail != nil {
			out = append(out, ingest.Pair{Node: node, Detail: *detail})
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interIssueDelay):
		}
	}
	return out, nil
}

func startOfUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func endOfUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 23, 59, 59, 999_000_000, time.UTC)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fatal("required environment variable is not set", fmt.Errorf("%s", key))
	}
	return v
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "[seed] FAILED: %s: %v\n", msg, err)
	os.Exit(1)
}

// loadDotEnv reads a .env file and sets any unset environment variables from
// it (csm-portal's loadDotEnv pattern). Silently ignored if the file does
// not exist.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

// writeSnapshots reconstructs one issue's daily sla_snapshots rows by
// replaying its reconciled event log (result.SlaEvents) as of each past
// day's end, from max(the issue's first event, the snapshotDays window
// start) through today. Never touched again after the seed writes it — the
// recompute scheduler only ever upserts *today's* row (SPEC §8.2).
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

	for !day.After(today) {
		through := endOfUTCDay(day)
		statusThatDay := sla.StatusAsOf(result.SlaEvents, through)
		r := sla.ComputeSla(result.Priority, result.SlaEvents, statusThatDay, runtime.Cfg, through)
		_, err := pool.Exec(ctx, `
			INSERT INTO sla_snapshots (
				snapshot_date, issue_id, repository_id, priority, current_status,
				budget_hours, consumed_hours, remaining_hours, pct_consumed, sla_state, sla_running
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, day, result.IssueID, repositoryID, result.Priority, statusThatDay,
			r.BudgetHours, r.ConsumedHours, r.RemainingHours, r.PctConsumed, string(r.SlaState), r.SlaRunning)
		if err != nil {
			return fmt.Errorf("insert sla_snapshot for %s: %w", day.Format("2006-01-02"), err)
		}
		day = day.Add(dayMs)
	}
	return nil
}
