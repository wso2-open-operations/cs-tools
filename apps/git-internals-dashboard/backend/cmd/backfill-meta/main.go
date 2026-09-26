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

// One-time backfill of title/abt_team/opened_by on issues rows that
// predate this feature. Requires GITHUB_TOKEN — unlike cmd/seed, there is
// no synthetic-fixtures fallback: a backfill is only meaningful against
// real GitHub data. Run via `make backfill-meta`.
//
// PRIVACY: this command only ever writes issues.title/abt_team/opened_by
// on rows that already exist. It never inserts rows and never touches
// issue_sla, issue_status_events, or sla_snapshots.
package main

import (
	"context"
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
)

// main connects to the configured GitHub repos and Postgres, then runs one
// backfill pass over every enabled repository's existing issues rows.
func main() {
	cliutil.LoadDotEnv(".env")

	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		fmt.Fprintln(os.Stderr, "[backfill-meta] FAILED: backfill-meta requires GITHUB_TOKEN; there is no synthetic mode for this command")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	appCfg, err := appconfig.Load()
	if err != nil {
		fatal("invalid app-config.yaml", err)
	}
	// Boot-only: must run before any github.Client is constructed below.
	github.Apply(appCfg.GitHub)

	app, err := config.Load()
	if err != nil {
		fatal("invalid sla-config.yaml", err)
	}

	pool, err := db.NewPoolWithConfig(ctx, cliutil.MustEnv("DATABASE_URL"), appCfg.Database)
	if err != nil {
		fatal("failed to connect to postgres", err)
	}
	defer pool.Close()

	client := github.NewClient(token)

	results, err := ingest.BackfillIssueMeta(ctx, pool, client, app.Settings.SeedClosedLookbackDays)
	if err != nil {
		fatal("backfill failed", err)
	}

	failed := false
	for _, r := range results {
		if r.Err != nil {
			failed = true
			fmt.Printf("[backfill-meta] %s: FAILED: %v%s\n", r.Repo, r.Err, rateLimitSuffix(client))
			continue
		}
		fmt.Printf("[backfill-meta] %s: fetched %d, updated %d%s\n", r.Repo, r.Fetched, r.Updated, rateLimitSuffix(client))
	}

	if failed {
		os.Exit(1)
	}
}

// rateLimitSuffix returns ", quota: N remaining (resets HH:MM:SS)" for a
// result log line, or "" when no response has reported a quota yet.
// cmd/seed has its own unexported copy of this helper; a helper in another
// command's package main isn't reachable from here.
func rateLimitSuffix(client github.Client) string {
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

// fatal prints msg and err to stderr and exits the process with status 1.
func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "[backfill-meta] FAILED: %s: %v\n", msg, err)
	os.Exit(1)
}
