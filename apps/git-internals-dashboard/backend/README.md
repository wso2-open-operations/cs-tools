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
| `GITHUB_TOKEN` | no | Fine-grained PAT (`Issues:Read` + `Projects:Read`). Used by incremental sync (`POST /sync/runs`), the real-GitHub seed (`cmd/seed`), and the metadata backfill (`cmd/backfill-meta`). Unset ⇒ `POST /sync/runs` returns `sync_token_missing`, seed falls back to synthetic fixtures, and backfill-meta fails immediately (it has no synthetic mode). |
| `CORS_ALLOWED_ORIGINS` | no | Comma-separated Origin allow-list. Empty ⇒ no cross-origin browser request allowed (fail closed). Local dev: `http://localhost:5173`. |
| `RECOMPUTE_ENABLED` | no (default on) | `0` disables the recompute scheduler (tests/CI). |
| `SLA_CONFIG_PATH` | no | Override config path (default `config/sla-config.yaml`). |
| `APP_CONFIG_PATH` | no | Override runtime config path (default `config/app-config.yaml`). |
| `LOG_LEVEL` | no | slog level, default `info`. |
| `SEED_STRICT_TAXONOMY` | no | `1` fails `make seed` on unknown board statuses instead of warning. |

The SLA taxonomy — which repos/projects to track, the status categories, and per-priority time
budgets — is configured in `config/sla-config.yaml`, loaded once at startup. Non-secret values
only; tokens and connection strings stay in environment variables.

**Known limitation — priority changes rewrite history.** There is no priority-change event log:
`internal/sla.ComputeSla` applies an issue's *current* priority's budget and coverage window to
its entire status-event timeline, not just to the time after the change. Escalating P3→P2 halves
the budget and re-masks all past accrual under the (same) 12x5 coverage window — it can flip an
issue straight to `VIOLATED`; downgrading can just as easily hide a real breach. Changing between
P1 and P2 additionally switches the coverage clock (24x7 vs. 12x5 IST) retroactively too. Fixing
this properly means persisting priority-change events and walking budget segments per interval;
until then, treat `pct_consumed`/`sla_state` as computed against an issue's *current* priority,
not a running record of the priority in effect when the time was actually spent.

**`settings.unknownStatusPolicy`** (`pause` default, or `accrue`) controls what the SLA clock
does with a board status absent from `taxonomy.statuses` — a renamed or newly added column. Either
way, the current set of unknown statuses is always surfaced on `GET /metrics/overview` (as
`unknownStatuses`, backed by the `unknown_statuses` table the recompute tick maintains) so they get
noticed and classified instead of silently mis-accruing forever. Like the priority-change caveat
above, changing this setting (or adding a status to `taxonomy.statuses`, which has the same effect)
retroactively re-walks every affected issue's full history on the next tick — expect a step in
`pct_consumed`/`consumed_hours`, not just a change going forward.

**`holidays`** (top-level list of `YYYY-MM-DD` dates, empty by default) excludes those IST calendar
days from the `12x5_ist` coverage window — e.g. Indian public holidays, if the SLA contract
excludes them; confirm before adding dates. `24x7` budgets are unaffected. Same retroactive-recompute
caveat as `unknownStatusPolicy`: adding or removing a date shifts `consumed_hours`/`pct_consumed`
for every issue whose history spans that date, on the next tick.

**Known limitation — GitHub closure stopping accrual is forward-only.** A GitHub issue closure now
caps its SLA clock and forces `TERMINAL` from the moment of closure onward (`sla.AdjustForClosure`),
fixing the case where a closed issue's board status was never moved to a terminal column and kept
accruing/polluting the `violated`/`at_risk` trend forever. This only changes what happens *going
forward*: the tick only ever re-upserts today's `sla_snapshots` row, so a long-closed issue's
already-written historical snapshot rows (e.g. old `VIOLATED` rows accrued before this fix shipped)
are not retroactively corrected — they stay as they were computed at the time. If cleaning up that
historical drift matters, it needs a one-off backfill job, not a code change here.

**`breachedEver`** (on `issue_sla`, exposed as `sla.breachedEver` in the issues API) is a sticky
"was this ever violated" signal, independent of `sla_state` — `sla_state` reports `TERMINAL` once
an issue resolves, which would otherwise mask a real past `VIOLATED`. It's OR'd against its prior
stored value on every write, so it can only go `false` → `true`, and is backfilled at migration time
from `sla_snapshots` history. That backfill is bounded by the snapshot window: an issue whose only
breach predates its earliest retained snapshot reads `breachedEver: false` — correct given available
history, but worth knowing before treating it as an absolute "never breached" claim.

### `config/app-config.yaml`

Operational tuning — server networking, DB pool, in-process caches, GitHub client pacing,
background jobs, and API request-validation limits — lives in `config/app-config.yaml`, loaded
once at startup by `internal/appconfig`. It is a separate file from `sla-config.yaml`: the SLA
config is domain data synced into the database at boot, while this file never touches the
database and changes on its own cadence (per environment, per load profile).

- **Precedence:** an environment variable (e.g. `PORT`) overrides the file, which overrides the
  built-in default. Every value in the committed file equals its built-in default, so the file can
  be edited, trimmed, or removed without changing behavior unless a value is actually changed.
- **Missing vs. broken:** if `APP_CONFIG_PATH` is unset and the default path is absent, the backend
  logs a warning and boots with built-in defaults. If `APP_CONFIG_PATH` is set and unreadable, or
  the file is present but fails to parse or validate, boot fails — an explicit path is treated as
  explicit intent.
- `settings.recomputeIntervalMinutes` is **not** in this file — it stays in `sla-config.yaml`
  alongside the other SLA-math knobs, so there is one source of truth per knob.
- The `api.*` limits mirror the ranges documented in `openapi.yaml`; change both together or the
  published contract will desync from the running service.
- `securityHeaders` sets response headers (CSP, `X-Frame-Options`,
  `Strict-Transport-Security`, etc.) on every response via
  `middleware.SecurityHeaders`. Ships with documented defaults commented out
  in the file, same as `database:` — a key added here overrides that one
  default's value or adds a new header, with no code change required.
- `readiness` tunes `GET /readyz`: the DB ping deadline (`timeoutSeconds`), how long a
  result is cached (`cacheTTLSeconds`), and whether pool saturation alone fails the probe
  (`failOnPoolSaturation` / `poolSaturationThresholdPercent`, off by default — see
  "Probes" below for why).
- Non-secret by rule, same as `sla-config.yaml`: tokens, DB URLs, and other credentials stay in
  environment variables, never in this file.
- **Choreo deployments** that need non-default values: mount the file and point `APP_CONFIG_PATH`
  at the mount path — `<<FILL IN>>` the component's config-mount configuration once that's decided.

## Probes

- **Liveness (`GET /healthz`)** never touches the database — it only reports the process
  is up and serving. If it depended on Postgres, a DB blip would make the kubelet kill and
  restart every replica at once: a restart storm on top of an outage, with no recovery path.
- **Readiness (`GET /readyz`)** answers "should traffic come here right now": a bounded
  `pool.Ping` plus a `pool.Stat()` snapshot, briefly cached (`readiness.cacheTTLSeconds`)
  to collapse concurrent probes into one DB round trip. 200 when ready, 503 otherwise, with
  a `checks.database.status` of `unreachable`, `timeout`, or `pool_saturated`. Pool
  saturation is always reported but only fails the probe when `failOnPoolSaturation` is
  explicitly enabled — a load spike saturates every replica at roughly the same moment, so
  making saturation alone fail the probe by default would turn backpressure into a total
  outage.
- Neither probe checks GitHub reachability or queries application tables — an unset
  `GITHUB_TOKEN` is a degraded feature, not unreadiness, and a missing table is a migration
  failure, not a readiness signal.
- **Choreo configuration:** point liveness at `GET /healthz` and readiness at
  `GET /readyz`. `<<FILL IN>>` the component's actual probe configuration once decided —
  `.choreo/component.yaml`'s `schemaVersion: 1.2` endpoint schema has no `probes:`/
  `healthCheck:` field to encode this in directly.
- On SIGTERM, this service marks itself draining (`/readyz` → 503 `draining`, `/healthz`
  unaffected) and waits `readiness.drainGracePeriodSeconds` (default `0`, i.e. no change in
  behavior) before the normal graceful-shutdown sequence begins. Set this in Choreo to
  roughly two readiness probe periods so the platform reliably stops routing traffic here
  before the process exits.

## Project Structure

```text
backend/
├── cmd/
│   ├── server/main.go       # Entry point — routes + server startup
│   ├── seed/main.go         # Synthetic or real-GitHub seed data
│   └── backfill-meta/main.go  # One-time backfill of title/abtTeam/openedBy for existing rows
├── internal/
│   ├── apierror/            # {"error":{"code","message"}} envelope + write helpers
│   ├── middleware/           # logger.go, recovery.go, cors.go, headers.go
│   ├── config/                # sla-config.yaml load + validate
│   ├── appconfig/              # app-config.yaml load + validate (operational tuning)
│   ├── db/                     # pgxpool init, config-sync
│   ├── sla/                     # Pure SLA engine — no I/O
│   ├── github/                   # GraphQL client: search, issue detail
│   ├── ingest/                     # normalize.go, ingest.go — the single write path for GitHub-derived data
│   ├── sync/                        # Incremental GitHub fetch + ingest, per-repo watermarks
│   ├── jobs/                          # lock.go (advisory lock), scheduler.go (recompute tick)
│   ├── metrics/                        # ttlcache.go, overview.go, timeseries.go
│   ├── taxonomy/                        # Config-driven status taxonomy helpers
│   └── handler/                          # issues.go, taxonomy.go, metrics.go, sync.go
├── migrations/                # golang-migrate SQL migrations
├── config/
│   ├── sla-config.yaml         # SLA domain config: repos, taxonomy, budgets
│   └── app-config.yaml         # Operational runtime config (see Configuration above)
└── openapi.yaml                # API contract
```

## API Endpoints

- `GET /healthz` — Liveness probe (no dependencies checked)
- `GET /readyz` — Readiness probe (bounded DB ping + pool stats; see "Probes" below)
- `GET /taxonomy` — Get the configured status taxonomy
- `GET /issues` — List issues
- `GET /issues/{id}` — Get issue by ID
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

Persisted: issue title, ABT team, and opened-by (only a `@wso2.com` address), as approved by
Security. The issue body is read transiently during ingest solely to derive ABT team and
opened-by, then discarded. Labels are read transiently to derive priority, then discarded.
Assignees and status-event actors are never fetched or stored.

## Deploying

Buildpack: Go, project path `backend/`. `.choreo/component.yaml` declares one REST endpoint
(basePath `/`, port 8080, `schemaFilePath: openapi.yaml`). Migrations run per-release, not at
server startup:

```bash
migrate -path migrations -database "$DATABASE_URL" up
```

### Backfilling issue metadata

After deploying a release that adds new issue metadata columns, run the backfill once so
existing rows pick up the new values:

```bash
GITHUB_TOKEN=... DATABASE_URL=... make backfill-meta
```

It only updates existing rows (never inserts, never touches SLA history) and is safe to
re-run.

The backfill only reaches issues within the search lookback window
(`settings.seedClosedLookbackDays`); issues closed longer ago than that keep `NULL`
title/abt_team/opened_by permanently unless the lookback setting is widened, but since
they're closed, the default open-only views never show them anyway.
