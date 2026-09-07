# Git Internals Dashboard Backend

Go service (stdlib `net/http`, `pgx/v5`, Postgres) for the Git Internals Dashboard. Ingests
GitHub issues, computes SLA compliance against a configurable taxonomy, and serves the metrics
API consumed by the webapp.

## Quick Start

```bash
# from apps/git-internals-dashboard/backend
docker compose up -d          # Postgres on localhost:5433 (user/pass/db: gid/gid/gid)
cp .env.example .env
set -a && source .env && set +a   # make migrate needs DATABASE_URL exported (see note below)
make migrate                  # golang-migrate against DATABASE_URL
make seed                     # synthetic fixtures unless GITHUB_TOKEN is set
make run                      # :8080
```

## Overview

- Default port: `8080`
- Runtime: Go `1.26+`
- Entry point: `cmd/server/main.go`
- Authentication: none — this service is designed to sit behind a gateway that validates the
  caller's identity, enforces rate limiting, and handles CORS in production. `CORS_ALLOWED_ORIGINS`
  exists only so a local frontend dev server can reach the backend directly without a gateway in
  front of it.

## Prerequisites

- Go `1.26+` — [install](https://go.dev/doc/install)
- Docker (for local Postgres via `docker-compose.yml`)
- The [golang-migrate CLI](https://github.com/golang-migrate/migrate):
  `go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

> **`make migrate` needs `DATABASE_URL` in your actual shell environment**, not just in `.env`.
> `make run` and `make seed` load `.env` themselves (`cmd/server`/`cmd/seed` call `loadDotEnv`
> internally), but `make migrate` shells out straight to the external `migrate` binary, which never
> reads `.env`. If you skip the `set -a && source .env && set +a` step (or otherwise don't have
> `DATABASE_URL` exported), you'll hit `error: failed to parse scheme from database URL: URL cannot
> be empty`.

## Testing

```bash
# Run all tests (from apps/git-internals-dashboard/backend)
go test ./...

# Run with the race detector, serialized (DB-backed tests share the compose Postgres)
go test -race -p 1 ./...

# Run a specific package
go test ./internal/handler/...

# Check test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Or use `make`:

```bash
make test    # vet + race-detector test
make build   # vet + test + compile
```

## Configuration

Copy `.env.example` to `.env` and fill in the values.

| Variable | Required | Purpose |
|---|---|---|
| `DATABASE_URL` | yes | Postgres DSN. Local default `postgres://gid:gid@localhost:5433/gid?sslmode=disable`. |
| `PORT` | no (default `8080`) | HTTP listen port. |
| `GITHUB_TOKEN` | no | Fine-grained PAT (`Issues:Read` + `Projects:Read`). Used by on-demand titles, incremental sync, and the real-GitHub seed. Unset ⇒ titles resolve to `null`, `POST /sync/runs` returns `sync_token_missing`, seed falls back to synthetic fixtures. |
| `CORS_ALLOWED_ORIGINS` | no | Comma-separated Origin allow-list. Empty ⇒ no cross-origin browser request allowed (fail closed). Local dev: `http://localhost:5173`. |
| `RECOMPUTE_ENABLED` | no (default on) | `0` disables the recompute scheduler (tests/CI). |
| `SLA_CONFIG_PATH` | no | Override config path (default `config/sla-config.yaml`). |
| `LOG_LEVEL` | no | slog level, default `info`. |
| `SEED_STRICT_TAXONOMY` | no | `1` fails `make seed` on unknown board statuses instead of warning. |

The SLA taxonomy — which repos/projects to track, the status categories, and per-priority time
budgets — is configured in `config/sla-config.yaml`, loaded once at startup. Non-secret values
only; tokens and connection strings stay in environment variables.

## Project Structure

```text
backend/
├── cmd/
│   ├── server/main.go       # Entry point — routes + server startup
│   └── seed/main.go         # Synthetic or real-GitHub seed data
├── internal/
│   ├── apierror/            # {"error":{"code","message"}} envelope + write helpers
│   ├── middleware/           # logger.go, recovery.go, cors.go
│   ├── config/                # sla-config.yaml load + validate
│   ├── db/                     # pgxpool init, config-sync
│   ├── sla/                     # Pure SLA engine — no I/O
│   ├── github/                   # GraphQL client: search, issue detail, titles
│   ├── ingest/                     # normalize.go, ingest.go — the single write path for GitHub-derived data
│   ├── sync/                        # Incremental GitHub fetch + ingest, per-repo watermarks
│   ├── jobs/                          # lock.go (advisory lock), scheduler.go (recompute tick)
│   ├── metrics/                        # ttlcache.go, overview.go, timeseries.go
│   ├── taxonomy/                        # Config-driven status taxonomy helpers
│   └── handler/                          # issues.go, taxonomy.go, metrics.go, sync.go, titles.go
├── migrations/                # golang-migrate SQL migrations
└── openapi.yaml                # API contract
```

## API Endpoints

- `GET /healthz` — Liveness probe
- `GET /taxonomy` — Get the configured status taxonomy
- `GET /issues` — List issues
- `GET /issues/{id}` — Get issue by ID
- `POST /issues/titles` — Resolve issue titles live from GitHub (never persisted)
- `GET /metrics/overview` — Aggregate SLA/volume overview
- `GET /metrics/timeseries` — SLA/volume trend over time
- `POST /sync/runs` — Trigger an incremental GitHub sync
- `GET /sync/status` — Get the status of the most recent sync

See `openapi.yaml` for the full request/response contract.

## Adding an endpoint

1. Add/extend a domain package function (`internal/handler` calls into `internal/db`,
   `internal/metrics`, `internal/sync`, etc. — never issues raw SQL inline beyond simple lookups).
2. Add the handler method, using `internal/apierror` for every non-2xx response and `writeJSON`
   for 2xx.
3. Register the route in `cmd/server/main.go` with a method-prefixed pattern
   (`"GET /issues/{id}"`).
4. Update `openapi.yaml`.
5. Add handler tests (validation, DB-backed behavior, error envelope shape).

## Privacy

No issue titles, labels, assignees, openers, or event actors are ever persisted to the database
or returned by any endpoint except `POST /issues/titles` (fetched live from GitHub, cached in
memory only). Labels are read transiently during ingest solely to derive an issue's priority,
then discarded.

## Deploying

Buildpack: Go, project path `backend/`. `.choreo/component.yaml` declares one REST endpoint
(basePath `/`, port 8080, `schemaFilePath: openapi.yaml`). Migrations run per-release, not at
server startup:

```bash
migrate -path migrations -database "$DATABASE_URL" up
```
