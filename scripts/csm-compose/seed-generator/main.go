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
//
// seed-generator populates entity-service's Postgres database with a
// broader, randomized set of dummy CSM data for the docker-compose local dev
// stack (see apps/csm-portal/README.md) -- accounts, projects, deployments,
// products, users, and a spread of work items (cases, service requests,
// change requests, incidents, engagements, problems) with comments, time
// cards, escalations, watchers, and tags, so the webapps' dashboards and
// list views show believable variety instead of the single fixed case
// scripts/csm-compose/seed-entity-service.sql seeds.
//
// LOCAL DEVELOPMENT ONLY. Every name, email, and company generated here is
// synthesized at runtime from the generic word lists in words.go -- never
// copied from any real list -- and nothing this program generates is ever
// committed; only this source is.
//
// Runs as its own one-shot compose service, gated on the "migrate" service's
// completion (see docker-compose.yml): migrate's own image is
// postgres:16-alpine, with no Go toolchain to build or run this binary
// inside that container, so this can't be a literal extra step appended to
// migrate-and-seed.sh the way the static SQL seed is -- it needs its own
// image, built the same way mock-oidc and gateway-shim already are.
//
// Idempotent across repeated `docker compose up` runs via a marker table
// (seed_generator_run): if that table already has a row, this program logs
// that generation already ran and exits 0 without touching any other table.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed-generator: failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	dsn := buildDSN()

	conn, err := connectWithRetry(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer conn.Close(ctx)

	already, err := alreadyGenerated(ctx, conn)
	if err != nil {
		return fmt.Errorf("checking marker table: %w", err)
	}
	if already {
		slog.Info("seed-generator: bulk seed data already generated, skipping")
		return nil
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	summary, err := generateAll(ctx, tx)
	if err != nil {
		return fmt.Errorf("generating seed data: %w", err)
	}

	if err := markGenerated(ctx, tx, summary); err != nil {
		return fmt.Errorf("recording marker row: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	slog.Info("seed-generator: done", "summary", summary.String())
	return nil
}

// buildDSN follows sre-alert-ingestion-service's precedent (see that
// service's cmd/server/main.go and this repo's docker-compose.yml comment on
// it): discrete DB_* environment variables, not a single connection-string
// var, matching entity-service's own DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/
// DB_NAME/DB_SSLMODE naming since this tool populates that same database.
func buildDSN() string {
	host := envOrDefault("DB_HOST", "postgres")
	port := envOrDefault("DB_PORT", "5432")
	user := envOrDefault("DB_USER", "postgres")
	password := os.Getenv("DB_PASSWORD")
	name := envOrDefault("DB_NAME", "csm_platform")
	sslmode := envOrDefault("DB_SSLMODE", "disable")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, name, sslmode)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// connectWithRetry waits for Postgres the same way migrate-and-seed.sh does
// (a plain retry loop) -- this program can start before the "migrate"
// service's own database-readiness wait has settled in edge cases (compose
// only guarantees migrate's entrypoint exited 0, not that every downstream
// connection is instantly accepted), so it tolerates a handful of early
// connection failures before giving up.
func connectWithRetry(ctx context.Context, dsn string) (*pgx.Conn, error) {
	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		conn, err := pgx.Connect(ctx, dsn)
		if err == nil {
			if pingErr := conn.Ping(ctx); pingErr == nil {
				return conn, nil
			} else {
				lastErr = pingErr
				_ = conn.Close(ctx)
			}
		} else {
			lastErr = err
		}
		slog.Info("seed-generator: waiting for postgres...", "attempt", attempt+1)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return nil, fmt.Errorf("giving up after retries: %w", lastErr)
}

// alreadyGenerated creates the marker table if needed and reports whether a
// previous run already populated it.
func alreadyGenerated(ctx context.Context, conn *pgx.Conn) (bool, error) {
	_, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS seed_generator_run (
			id SMALLINT PRIMARY KEY DEFAULT 1,
			generated_on TIMESTAMPTZ NOT NULL,
			summary TEXT NOT NULL,
			CONSTRAINT seed_generator_run_singleton CHECK (id = 1)
		)`)
	if err != nil {
		return false, err
	}

	var exists bool
	err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM seed_generator_run WHERE id = 1)`).Scan(&exists)
	return exists, err
}

func markGenerated(ctx context.Context, tx pgx.Tx, summary genSummary) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO seed_generator_run (id, generated_on, summary) VALUES (1, $1, $2)
		 ON CONFLICT (id) DO NOTHING`,
		time.Now(), summary.String())
	return err
}
