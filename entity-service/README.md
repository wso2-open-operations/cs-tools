# Entity Service

The entity service is the data and domain service of the CSM platform. The CSM portal backend, the
customer portal backend and the platform's integration services call it for everything that is stored
as a business entity: cases, projects, accounts, deployments, change requests, incidents, escalations,
time cards, team schedules and so on. It owns the PostgreSQL schema, enforces who may see which rows,
and, where a deployment still needs it, adapts an external ticketing system behind the same API.

For where this service sits in the platform, see the
[platform architecture](../apps/csm-portal/README.md#architecture).

## Contents

- [Architecture](#architecture)
  - [Tech stack](#tech-stack)
  - [Code layout](#code-layout)
  - [Request flow](#request-flow)
  - [Pluggable data sources](#pluggable-data-sources)
  - [Authentication and access scoping](#authentication-and-access-scoping)
  - [API contracts](#api-contracts)
  - [Health probes](#health-probes)
  - [Background jobs](#background-jobs)
  - [Database and migrations](#database-and-migrations)
- [Developer guide](#developer-guide)
  - [Prerequisites](#prerequisites)
  - [Quick start](#quick-start)
  - [Make targets](#make-targets)
  - [Running migrations](#running-migrations)
  - [Tests](#tests)
  - [Lint and security scans](#lint-and-security-scans)
  - [Adding an endpoint or entity](#adding-an-endpoint-or-entity)
  - [Adding a migration](#adding-a-migration)
  - [The scripts directory](#the-scripts-directory)
- [Configuration reference](#configuration-reference)
- [License](#license)

## Architecture

### Tech stack

| Layer          | Technology                                                                 |
| -------------- | -------------------------------------------------------------------------- |
| Language       | Go (see the `go` directive in [`go.mod`](go.mod) for the exact toolchain)  |
| HTTP           | Standard library `net/http` with Go 1.22+ method-and-path patterns (`mux.HandleFunc("POST /cases/search", ...)`) |
| Database       | PostgreSQL, accessed through `pgx` v5 (`pgxpool`) with hand-written SQL     |
| Messaging      | Kafka-compatible event bus through `kafka-go` (optional)                    |
| Cache          | Redis through `go-redis` (optional, user lookups only)                      |

There is no ORM and no code generator: repositories contain plain SQL.

### Code layout

```text
entity-service/
├── cmd/api/main.go          # Entry point: load config, open the pool, build the router, start
│                            #   the API and health listeners and the background workers
├── internal/
│   ├── config/              # Environment-driven Config: Load(), Validate(), DSN()
│   ├── server/              # NewRouter (routes.go), health listener, middleware chain, wiring
│   ├── middleware/          # Correlation id, panic recovery, access log, user-token capture, timeout
│   ├── auth/                # Token validation (JWKS), caller identity
│   ├── handler/             # HTTP layer: decode and validate input, call a service, write JSON
│   ├── service/             # Business rules, the data-source variants of each service, workers
│   ├── repository/          # PostgreSQL access (pgx), row-level-security scoping, transactions
│   ├── domain/              # Request, response and entity types shared across layers
│   ├── apierror/            # Sentinel errors and the JSON error responder
│   ├── db/                  # pgxpool construction
│   ├── cache/               # Optional Redis user cache and an in-process cache
│   ├── eventbus/, events/   # Event producer and the event envelope wire shape
│   ├── github/              # GitHub API client and routing (optional integration)
│   ├── salesentity/         # Client for an external sales-record service (optional)
│   ├── (client packages)    # Clients for the optional external ticketing adapter and sales-record service
│   └── validate/            # Small input validators
├── migrations/              # Plain SQL schema migrations
├── scripts/                 # Operator and test helper scripts (see below)
├── openapi.yaml             # Published contract of the main API
├── health-openapi.yaml      # Published contract of the health listener
├── queries/, sqlc.yaml      # Empty placeholders; sqlc is not used
└── Makefile
```

Dependencies point inwards: `handler` depends on service interfaces declared in `service`
(`interfaces.go` and the per-entity service files), `service` depends on repository types, and
`repository` depends on `pgx`. `server/routes.go` is the composition root. It builds every
repository, service and handler, picks the implementation for the configured data source, and
registers the routes.

### Request flow

```text
HTTP request
  └── CorrelationID → Recovery → Logger → UserIDToken → auth.Middleware (validate tokens)
        └── Timeout (REQUEST_TIMEOUT)
              └── caller identity middleware (resolve the caller's access scope)
                    └── ServeMux route → Handler (decode, validate)
                          └── Service (rules, pagination, data-source selection)
                                └── Repository (SQL under the caller's scope) → PostgreSQL
```

### Pluggable data sources

Each entity is served through a service interface, and `routes.go` chooses the implementation
from configuration:

| `DATA_SOURCE`                      | Behavior |
| ---------------------------------- | -------- |
| `postgres` (default)               | PostgreSQL is the system of record for reads and writes. |
| `servicenow`                       | Reads and writes for the migrated entities go to an external ticketing system through an integration service. PostgreSQL is optional and only backs the side tables that have no external equivalent. |
| `postgres-servicenow-dual-write`   | PostgreSQL stays authoritative for every read and write. Selected writes are mirrored afterwards to the external system on a best-effort basis, and failures are recorded rather than surfaced. |

`SLA_DATA_SOURCE` and `ATTACHMENT_DATA_SOURCE` (each `postgres` or `servicenow`) override the
choice for SLA reads and for case attachment and activity-feed reads independently, so a
deployment can move one resource at a time. The `servicenow` values are the literal configuration
spellings of "use the external ticketing system adapter"; the credentials for that adapter are
the `SERVICENOW_INTEGRATION_SERVICE_*` variables in the [configuration reference](#configuration-reference).

Features that exist only in the new platform (team schedules, the event outbox, scheduled-task
claim state, SLA status, and similar) are always stored in PostgreSQL, whatever the data source. They
are registered only when a database is configured.

### Authentication and access scoping

The service sits behind an API gateway and never handles credentials. It validates the tokens the
gateway forwards and then decides what the caller may see from data in PostgreSQL:

- `x-user-id-token` carries the end user's ID token. It is verified against the issuer's JWKS
  (`AUTH_ISSUER`, `AUTH_JWKS_URL`, `AUTH_USER_TOKEN_AUDIENCES`). Validation is always on. A token
  that is present but invalid is a 401 on every route.
- The calling application is identified by the client id in the `x-jwt-assertion` token (decoded,
  not re-verified here, because the gateway has already validated it).
- The caller's client id selects how much it is trusted. `M2M_CLIENT_IDS` lists pure
  machine-to-machine callers with full access. `CSM_PORTAL_BACKEND_CLIENT_ID` plus
  `CSM_PORTAL_USER_DOMAIN` identifies the internal portal backend, which is unrestricted only for
  users in that email domain. `CUSTOMER_PORTAL_BACKEND_CLIENT_ID` identifies the customer portal
  backend, which is always scoped to the forwarded user's own projects and is never trusted
  unconditionally.
- Row-level security in PostgreSQL enforces the resulting scope. Repositories that touch protected
  tables run their queries through `repository.Scoped`, which applies the caller's identity to the
  transaction. Background workers use an explicit system identity.

### API contracts

- [`openapi.yaml`](openapi.yaml) is the published contract of the main API (port `SERVER_PORT`).
- [`health-openapi.yaml`](health-openapi.yaml) is the contract of the health listener (port
  `HEALTH_PORT`).

`internal/server/routes_contract_test.go` fails the build when a route is registered in
`routes.go` but missing from `openapi.yaml`, because a gateway that routes only declared operations
would otherwise answer 404 for it in a deployed environment. Keep the two in sync in the same change.

### Health probes

The service listens on two ports. `SERVER_PORT` (default `8080`) carries the API. `HEALTH_PORT`
(default `8081`) carries nothing but the two probes, so it can be exposed to external monitoring
without exposing any business route. Which routes are reachable on the health port is decided by
the mux they are registered on (`internal/server/health.go`), not by a gateway rule. Do not register
business routes on the health mux.

| Probe                  | Port                | Answers |
| ---------------------- | ------------------- | ------- |
| `GET /health`          | API and health port | Always `200 {"status":"ok"}`. Liveness only; it makes no dependency calls. |
| `GET /health/database` | Health port only    | `200 {"status":"ok","database":"up"}` when a PostgreSQL round trip succeeds, `503 {"status":"unavailable","database":"down"}` when it fails. With no database configured it answers `200` with `"database":"not_configured"`. |

Failure bodies carry no error detail, and both probes send `Cache-Control: no-store`. `HEALTH_PORT`
must differ from `SERVER_PORT`; configuration validation rejects the same port. If you deploy with
a platform manifest that declares the ports statically (as `.choreo/component.yaml` does), leave
both at their defaults there and override them only locally.

### Background jobs

Workers start inside the API process (`cmd/api/main.go` and `internal/server/routes.go`) and stop
on shutdown. Most are off unless configured:

| Worker | Runs when | What it does |
| ------ | --------- | ------------ |
| Incident report drainer | A database is configured | Reads incident changes from the `event_outbox` table and creates incident report tasks. Interval: `INCIDENT_REPORT_POLL_INTERVAL`. |
| Change-request notice drainer | `CR_NOTICES_ENABLED=true`, a database, and event publishing configured | Publishes change-request notices to the event bus. Interval: `CR_NOTICE_POLL_INTERVAL`. |
| Outage notice drainer | A database (not `servicenow` data source), event publishing configured, and at least one of the outage recipient lists set | Publishes the two outage emails. Woken by a PostgreSQL `NOTIFY`, with `OUTAGE_NOTICE_POLL_INTERVAL` as the fallback poll. |
| Cloud status drainer | `CLOUD_STATUS_DRAINER_ENABLED=true`, `CLOUD_STATUS_SERVICE_IDS` set, not the `servicenow` data source | Records outage transitions and updates the cloud monitor status. Interval: `CLOUD_STATUS_POLL_INTERVAL`. |
| GitHub outbound worker | `GITHUB_INTEGRATION_ENABLED=true`, a token, `GITHUB_INTEGRATION_LOGIN`, and a database | Drains the outbound queue and pushes change-request activity to the linked issue. Interval: `GITHUB_OUTBOUND_INTERVAL`. |
| Sales-record ingest retry worker | The membership ingest is enabled with its prerequisites (see `routes.go`) and `SALESFORCE_INGEST_RETRY_INTERVAL` is above zero | Re-runs ingests that failed because a parent record was not yet present. Interval: `SALESFORCE_INGEST_RETRY_INTERVAL`; `0` disables it. |

Periodic work that is not tied to this process (scheduled sweeps, housekeeping) is driven by a
separate scheduled-task component that calls this service's `/scheduled-tasks/attempts` endpoints.

### Database and migrations

- The schema lives only in [`migrations/`](migrations). The service never applies migrations at
  startup; you apply them with `make migrate` (or your own tooling) before running it.
- Applied files are recorded in the `csm_migration_applied_migration` table. This is the same ledger
  convention another component of the platform uses against the same database, so keep it.
- There is no query-generation step. `queries/entity.sql` and `sqlc.yaml` exist but are empty.
  SQL is written by hand in `internal/repository`.
- Migration files come in three number spaces, applied in this order:
  1. `NNNN_<description>.sql`: four-digit, single-file, no separate up or down. The current convention.
  2. `1NNNNN_<description>.sql`: six-digit, 100000 and above. Row-level-security policy migrations,
     applied after every schema migration so the tables they protect already exist.
  3. `0NNNNN_<description>.up.sql` (with `.down.sql` rollbacks that are never auto-applied): legacy
     files numbered 000086 to 000105, applied last.
- Run the service as a non-superuser database role. PostgreSQL superusers bypass
  row-level security, so a superuser connection hides policy problems. The local compose stack
  migrates as an admin role and runs the service as a separate application role for this reason.

## Developer guide

### Prerequisites

- Go, at the version in the `go` directive of [`go.mod`](go.mod)
- PostgreSQL 15 or newer, with `psql` on your `PATH` (the Makefile migrates through `psql`)
- `make` and a POSIX shell (`bash`)
- Optional: Redis (user cache), a Kafka-compatible broker (event publishing), Docker (local database
  or the full local stack)
- Optional: [`gosec`](https://github.com/securego/gosec) and
  [`govulncheck`](https://golang.org/x/vuln/cmd/govulncheck)

### Quick start

1. Clone and fetch dependencies.

   ```bash
   git clone https://github.com/wso2-open-operations/cs-tools
   cd cs-tools/entity-service
   go mod download
   ```

2. Start a local PostgreSQL. Any instance works. For example, with Docker:

   ```bash
   docker run -d --name entity-pg -p 5432:5432 \
     -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=<admin-password> \
     -e POSTGRES_DB=entity_service postgres:18-alpine
   ```

   If you would rather bring up the whole platform (database, migrations, seed data, both
   backends, the portals and a mock identity provider), use the repository's compose stack
   described in the [portal README](../apps/csm-portal/README.md).

3. Create your environment file. It is gitignored and loaded automatically (a missing `.env` is
   ignored; a malformed one stops startup).

   ```bash
   cp .env.example .env
   ```

   Set at least the database variables and the three required `AUTH_*` variables. The comments in
   `.env.example` explain each group; every variable is documented below.

   ```env
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=<db-user>
   DB_PASSWORD=<db-password>
   DB_NAME=entity_service
   DB_SSLMODE=disable
   AUTH_ISSUER=https://idp.example.com/oauth2/token
   AUTH_JWKS_URL=https://idp.example.com/oauth2/jwks
   AUTH_USER_TOKEN_AUDIENCES=<webapp-client-id>
   ```

   Startup downloads the JWKS from `AUTH_JWKS_URL` and exits if it cannot. For local work without a
   real identity provider, point both at a mock OIDC server (the compose stack ships one).

4. Apply migrations (see [Running migrations](#running-migrations)).

   ```bash
   make migrate
   ```

5. Run the service.

   ```bash
   go run ./cmd/api
   ```

   The API listens on `http://localhost:8080` and the health listener on `http://localhost:8081`.

   ```bash
   curl -s http://localhost:8081/health/database
   ```

### Make targets

| Target                  | What it does |
| ----------------------- | ------------ |
| `make build`            | `go build ./...` |
| `make test`             | `go test ./...` |
| `make lint`             | `go vet ./...` |
| `make migrate`          | Applies pending files from `migrations/` through `psql` and records them (details below). |
| `make schema-bootstrap-sql` | Runs `scripts/generate_schema_bootstrap.sh`, which concatenates the migrations, in `make migrate` order, into standalone SQL files under `dist/` (gitignored) for someone to run by hand against an empty database. |

### Running migrations

`make migrate` reads `.env` if present, then connects with either `DATABASE_URL` (a full
connection string, takes priority) or `DB_HOST`, `DB_PORT` (default `5432`), `DB_USER`,
`DB_PASSWORD`, `DB_NAME` and `DB_SSLMODE` (default `require` for this target, so set
`DB_SSLMODE=disable` for a local database without TLS).

For each pending file, in the three-phase order described under
[Database and migrations](#database-and-migrations), it runs the file with `ON_ERROR_STOP`, then
inserts the filename into `csm_migration_applied_migration`. It is safe to re-run: recorded files
are skipped. Each run also writes a review bundle of exactly what it applied to
`dist/schema_migrations_<timestamp>.sql` and `dist/rls_migrations_<timestamp>.sql`.

Run migrations as a role that is allowed to create tables, types, functions and policies, then run
the service as a lower-privileged role.

### Tests

```bash
make test               # or: go test ./...
go test -race ./...     # recommended before sending a change
go test ./internal/service -run TestName
```

Most tests are unit tests and need nothing external. A subset are integration tests that exercise
real SQL and row-level-security policies. They skip themselves unless a DSN for a **migrated**
database is provided. Use a throwaway database, never one you care about:

| Variable | Used by |
| -------- | ------- |
| `ENTITY_TEST_DATABASE_URL` | The general integration tests |
| `*_TEST_DSN` (for example `CHANGE_REQUEST_TEST_DSN`, `CASE_STATS_TEST_DSN`, `PG_OUTAGE_TEST_DSN`) and `PLG_TEST_DATABASE_URL` | Per-feature integration tests; the exact name is in the `t.Skip` message of each test file |

### Lint and security scans

```bash
make lint                          # go vet ./...
gofmt -l .                         # must print nothing

go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec -fmt=text ./...              # should report 0 issues

go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...                  # should report no vulnerabilities
```

Fix the root cause of a gosec finding rather than suppressing it. Most `govulncheck` findings are
Go standard-library issues; bump the patch version in the `go` directive of `go.mod` and run
`go mod tidy`.

### Adding an endpoint or entity

Work from the database upwards, and prove each layer before building on it.

1. **Migration.** Add the table or column as described in [Adding a migration](#adding-a-migration).
   If the table holds data that callers must not all see, add a row-level-security migration in the
   `1NNNNN` space as well.
2. **Domain types.** Add request, response and entity structs to `internal/domain/`.
3. **Repository.** Add the SQL in `internal/repository/<entity>_repo.go`. Use parameter placeholders,
   never string concatenation. For protected tables, take a `*repository.Scoped` so the query runs
   under the caller's identity.
4. **Service.** Add an interface and implementation in `internal/service/`. Put business rules and
   pagination here. If the entity must also be served by another data source, add the second
   implementation next to the PostgreSQL one.
5. **Handler.** Add `internal/handler/<entity>_handler.go`: decode and validate the request, call the
   service, and map errors through `internal/apierror`.
6. **Route.** In `internal/server/routes.go`, construct the repository, service and handler and
   register the route with a method-and-path pattern, for example
   `mux.HandleFunc("GET /widgets/{id}", widgetHandler.GetWidget)`. Gate registration on `db != nil`
   if the feature is PostgreSQL-only.
7. **Contract.** Add the operation to [`openapi.yaml`](openapi.yaml). `TestEveryRegisteredRouteIsPublished`
   fails if you forget. Prefer adding over changing: existing consumers depend on current shapes.
8. **Tests.** Add unit tests beside each layer; add an integration test (gated on a test DSN, as
   above) for anything that depends on SQL or row-level-security behavior.
9. **Verify for real.** Run the service against a migrated database, make the request, and read the
   record back. A `200` alone proves nothing.

### Adding a migration

1. Pick the next free four-digit number in `migrations/` (or the next `1NNNNN` number for a
   row-level-security policy) and name the file `NNNN_<short_description>.sql`.
2. Start from the license header the existing files carry.
3. Write plain SQL with no transaction-control or ledger statements of its own. `make migrate`
   records the file after it succeeds.
4. Make it idempotent so a half-applied or re-applied file is harmless: `CREATE TABLE IF NOT EXISTS`,
   `ADD COLUMN IF NOT EXISTS`, `DROP POLICY IF EXISTS` before `CREATE POLICY`, and
   `DO $$ ... EXCEPTION WHEN duplicate_object THEN NULL; END $$;` around `CREATE TYPE`.
5. Do not edit a migration that has been merged; add a new one.
6. Apply it to a scratch database with `make migrate`, and apply it twice to confirm the second run
   is a no-op.

### The scripts directory

`scripts/` holds operator and test helpers. They are not part of the service build or runtime.

| File | Purpose |
| ---- | ------- |
| `generate_schema_bootstrap.sh` | Backs `make schema-bootstrap-sql`: builds standalone schema and RLS SQL files from `migrations/`. |
| `apply-team-schedule-migrations.sh` | Applies only the team-schedule migration chain, recording each in the ledger exactly as `make migrate` does. Dry run unless `APPLY=1`. |
| `renumber_rls_migrations.sh` | One-shot historical script that moved the row-level-security migrations into the `1NNNNN` number space. Not for routine use. |
| `record_manually_applied_rls_migrations.sql` | Records RLS migrations that were applied by hand, so `make migrate` does not re-run them. |
| `github-integration-e2e.sh`, `e2e.env.example` | End-to-end check of the optional GitHub integration against a real database and repository; it reverts what it writes. Copy `e2e.env.example` and fill in your own values. |
| `backfill_outage_communications.dev.sql` | Development-only data backfill. |
| `cutover/` | SQL used when cutting a data set over between sources. |
| `*.js` | Read-only inspection snippets for the external ticketing system, run inside that system's own script console during data-source work. Not needed for development against PostgreSQL. |

Read a script before running it against a database that matters.

## Configuration reference

Configuration is read from environment variables in `internal/config/config.go`, after loading
`.env` from the working directory if one exists. `Validate()` runs at startup and the process exits
on the first error. Placeholders such as `<...>` below are not real values.

Conventions: durations use Go syntax (`500ms`, `30s`, `5m`). "Required" means required in the
common configuration (PostgreSQL data source); the description says when it is conditional. A
group marked "together" must be all set or all unset.

### Server

| Variable | Required | Default | Description | Example |
| -------- | -------- | ------- | ----------- | ------- |
| `SERVER_PORT` | No | `8080` | API listener port. | `8080` |
| `HEALTH_PORT` | No | `8081` | Health listener port. Must resolve to a different port than `SERVER_PORT`. | `8081` |
| `SERVER_READ_TIMEOUT` | No | `60s` | HTTP server read timeout. Must be > 0; an unparsable value stops startup. | `60s` |
| `SERVER_WRITE_TIMEOUT` | No | `60s` | HTTP server write timeout. Must be > 0. | `60s` |
| `REQUEST_TIMEOUT` | No | `60s` | Per-request context deadline. Must be > 0. | `60s` |
| `UPSTREAM_CLIENT_TIMEOUT` | No | `60s` | Timeout of the HTTP client used for upstream calls. Must be > 0. | `60s` |
| `AVAILABILITY_TIMEZONE` | No | `Asia/Colombo` | IANA zone in which availability sweeps resolve period boundaries. | `UTC` |
| `WORKLOG_REPORT_TEAM_TYPES` | No | `cre-abt,cre` | `team.type` values the CS Worklogs reports cover. |
| `WORKLOG_REPORT_EXCLUDED_TEAM_KEYS` | No | empty | `team.key` values dropped from that scope — no section, no members in the totals, no lead on the derived audience. |

### Database

| Variable | Required | Default | Description | Example |
| -------- | -------- | ------- | ----------- | ------- |
| `DB_HOST` | No | `localhost` | PostgreSQL host. | `localhost` |
| `DB_PORT` | No | `5432` | PostgreSQL port. | `5432` |
| `DB_USER` | Yes (see note) | none | Database role. | `<db-user>` |
| `DB_PASSWORD` | Yes (see note) | none | Database password. | `<db-password>` |
| `DB_NAME` | Yes (see note) | none | Database name. | `entity_service` |
| `DB_SSLMODE` | No | none (empty is passed to the driver) | PostgreSQL `sslmode`. Set it explicitly: `disable` locally, `require` or stricter remotely. | `disable` |
| `DB_SCHEMA` | No | `<DB_USER>,public` | Sets the connection `search_path`. | `public` |
| `DB_POOL_MAX_CONNS` | No | `20` | Maximum pool connections. Positive integer. | `20` |
| `DB_POOL_MIN_CONNS` | No | `2` | Minimum pool connections. `0` allowed. | `2` |
| `DB_POOL_MAX_CONN_LIFETIME` | No | `30m` | Maximum connection lifetime. | `30m` |
| `DB_POOL_MAX_CONN_IDLE_TIME` | No | `5m` | Maximum connection idle time. | `5m` |

Note: `DB_USER`, `DB_PASSWORD` and `DB_NAME` are required when `DATA_SOURCE` is `postgres` or
`postgres-servicenow-dual-write`, and must be set together or not at all in every mode. With
`DATA_SOURCE=servicenow` they may all be omitted, in which case no pool is opened and PostgreSQL-only
features are not registered. `make migrate` also accepts `DATABASE_URL`, which the service itself
does not read.

### Data sources and the external ticketing adapter

| Variable | Required | Default | Description | Example |
| -------- | -------- | ------- | ----------- | ------- |
| `DATA_SOURCE` | No | `postgres` | `postgres`, `servicenow` or `postgres-servicenow-dual-write`. See [Pluggable data sources](#pluggable-data-sources). | `postgres` |
| `SLA_DATA_SOURCE` | No | `postgres` | `postgres` or `servicenow`; overrides the source for SLA reads. | `postgres` |
| `ATTACHMENT_DATA_SOURCE` | No | `postgres` | `postgres` or `servicenow`; overrides the source for attachment reads and the case activity feed. | `postgres` |
| `SERVICENOW_INTEGRATION_SERVICE_BASE_URL` | Conditional | none | Base URL of the adapter's integration service. Required when any of the three data-source variables above selects the external system (including dual-write). | `https://adapter.example.com` |
| `SERVICENOW_INTEGRATION_SERVICE_TOKEN_URL` | Conditional | none | OAuth2 token endpoint for the adapter. Required under the same condition. | `https://idp.example.com/oauth2/token` |
| `SERVICENOW_INTEGRATION_SERVICE_CLIENT_ID` | Conditional | none | OAuth2 client id. Required under the same condition. | `<adapter-client-id>` |
| `SERVICENOW_INTEGRATION_SERVICE_CLIENT_SECRET` | Conditional | none | OAuth2 client secret. Secret. Required under the same condition. | `<adapter-client-secret>` |
| `SERVICENOW_INTEGRATION_SERVICE_SCOPES` | No | none | Space-separated OAuth2 scopes. | `<scopes>` |

### Authentication and caller trust

| Variable | Required | Default | Description | Example |
| -------- | -------- | ------- | ----------- | ------- |
| `AUTH_ISSUER` | Yes | none | Issuer URL tokens must carry. | `https://idp.example.com/oauth2/token` |
| `AUTH_JWKS_URL` | Yes | none | JWKS endpoint for signature verification. It must load at startup or the process exits. | `https://idp.example.com/oauth2/jwks` |
| `AUTH_USER_TOKEN_AUDIENCES` | Yes | none | Comma-separated client ids an ID token's `aud` must contain to count as a user token. | `<webapp-client-id>` |
| `AUTH_CLOCK_SKEW` | No | `30s` | Leeway when checking token expiry. An invalid or non-positive value falls back to the default. | `30s` |
| `AUTH_ADMIN_ROLES` | No | none | Comma-separated token roles that make a caller a portal admin for the team-schedule admin actions and case-alert settings. Use the same value as the CSM portal backend. Unset means nobody is an admin. | `<admin-role>` |
| `M2M_CLIENT_IDS` | No | none | Comma-separated client ids of machine-to-machine callers granted unrestricted access to every project and case. | `<service-client-id-1>,<service-client-id-2>` |
| `CSM_PORTAL_BACKEND_CLIENT_ID` | No | none | Client id of the internal portal backend. Unrestricted only when the forwarded user's email is in `CSM_PORTAL_USER_DOMAIN`; otherwise refused (403). Set together with `CSM_PORTAL_USER_DOMAIN`. Must differ from `CUSTOMER_PORTAL_BACKEND_CLIENT_ID`. | `<portal-backend-client-id>` |
| `CSM_PORTAL_USER_DOMAIN` | No | none | Email domain for internal users (a leading `@` is stripped). Set together with `CSM_PORTAL_BACKEND_CLIENT_ID`. | `example.com` |
| `CUSTOMER_PORTAL_BACKEND_CLIENT_ID` | No | none | Client id of the customer portal backend. Always resolved from the forwarded user token and never trusted unconditionally. Do not also list it in `M2M_CLIENT_IDS`. | `<customer-portal-client-id>` |
| `CUSTOMER_ROLES` | No | none | Comma-separated role names whose holders' comments count as customer replies (external-system data source only). Unset disables that transition. | `<customer-role>` |
| `CS_ENGINEER_ROLE` | No | none | Role name marking a comment author as a support engineer, used to complete the response SLA. Unset skips that check. | `<engineer-role>` |

### Caching

| Variable | Required | Default | Description | Example |
| -------- | -------- | ------- | ----------- | ------- |
| `REDIS_URL` | No | none | `redis://` or `rediss://` connection string; takes priority over `REDIS_ADDR`. Contains the password, so treat it as a secret. The target must be non-clustered (or the enterprise clustering policy). | `rediss://:<access-key>@redis.example.com:6380` |
| `REDIS_ADDR` | No | none | Plain `host:port` for a local Redis. | `localhost:6379` |
| `REDIS_PASSWORD` | No | none | Password paired with `REDIS_ADDR`. | `<redis-password>` |
| `USER_CACHE_TTL` | No | `10m` | Lifetime of a cached user record. Writers also invalidate on change, so this is a backstop. | `10m` |
| `PRODUCT_VULNERABILITY_META_CACHE_TTL` | No | `24h` | Lifetime of the cached distinct product/version options of `GET /products/vulnerabilities/meta` (needs Redis). A successful `POST /products/vulnerabilities/sync` deletes the entry, so this is a backstop. | `24h` |

With neither `REDIS_URL` nor `REDIS_ADDR`, or when Redis is unreachable, user reads go straight to
PostgreSQL. Cache keys are namespaced `entity:v1:user:*`.

### Event publishing

Publishing needs `EVENT_HUB_BROKER` set and `EVENT_PUBLISHING_ENABLED=true`. Publishing is
synchronous with a bounded timeout, and failures are recorded in `event_publish_failures`.

| Variable | Required | Default | Description | Example |
| -------- | -------- | ------- | ----------- | ------- |
| `EVENT_HUB_BROKER` | No (together) | none | Kafka-compatible bootstrap address. Setting it is the feature gate. | `broker.example.com:9093` |
| `EVENT_HUB_CONNECTION_STRING` | No (together) | none | Namespace-scoped connection string used for SASL/PLAIN. Secret. | `<connection-string>` |
| `EVENT_HUB_TOPIC` | No (together) | none | Topic for case events. Must match the consumer's topic. | `case-events` |
| `EVENT_PUBLISHING_ENABLED` | No | `false` | Exact string `true` turns publishing on. | `true` |
| `CR_EVENT_HUB_TOPIC` | No | `cr-events` | Topic for change-request notices. | `cr-events` |
| `PROJECT_EVENT_HUB_TOPIC` | No | `project-events` | Topic for onboarding events. | `project-events` |
| `OUTAGE_EVENT_HUB_TOPIC` | No | `outage-events` | Topic for outage emails. | `outage-events` |
| `INCIDENT_EVENT_HUB_TOPIC` | No | unset (incident events use `EVENT_HUB_TOPIC`) | Dedicated topic for incident events. | `incident-events` |
| `SRE_EVENT_HUB_TOPIC` | No | none | When set, overrides both `CR_EVENT_HUB_TOPIC` and `OUTAGE_EVENT_HUB_TOPIC`; also enables special-operations incident alerts. | `sre-events` |
| `CASE_ESCALATION_NOTICES_ENABLED` | No | on (PostgreSQL data sources) | `false` turns off publishing the case escalation email event. Only `true`, `false` or unset are valid. | `true` |

### Change requests, outages, incidents and service requests

| Variable | Required | Default | Description | Example |
| -------- | -------- | ------- | ----------- | ------- |
| `CR_NOTICES_ENABLED` | No | `false` | Exact `true` starts the change-request notice drainer. Needs a database and event publishing. | `true` |
| `CR_NOTICE_POLL_INTERVAL` | No | `5s` | Idle poll interval of that drainer. Invalid or non-positive falls back to the default. | `5s` |
| `CR_STRICT_VISIBILITY_FROM` | No | unset (no cutover) | RFC 3339 instant with a zone. Change requests created at or after it are visible to customers only when designated to them. Do not move it after release. An invalid value stops startup. | `2026-11-01T00:00:00Z` |
| `OUTAGE_NOTICE_POLL_INTERVAL` | No | `60s` | Fallback poll of the outage notice drainer. | `60s` |
| `OUTAGE_NOTIFICATION_RECIPIENTS` | No | none | Comma-separated recipients of the internal-stakeholder outage notification. Empty disables that email. | `<recipient>` |
| `OUTAGE_COMMUNICATION_RECIPIENTS` | No | none | Comma-separated recipients of the outage communication. Empty disables that email. | `<recipient>` |
| `MIGRATION_CASE_REPORT_HOLD_TAG` | No | `s_mig_hold` | Tag name (matched ignoring case) that places a long-running migration in the weekly scan's "customer requested to hold" table (`GET /reports/migration-cases`). | `s_mig_hold` |
| `MIGRATION_CASE_REPORT_ACCOUNTS_TAG` | No | `s_acc` | Tag name (matched ignoring case) for the scan's "waiting on accounts team" table. | `s_acc` |
| `MIGRATION_CASE_REPORT_TEAM_KEYS` | No | none | Comma-separated team keys whose mail groups (`team.group_email`) the scan returns as its `recipients`. Empty = the scan derives no recipients and the scheduled task must configure its own. | `atlas,vega` |
| `CLOUD_STATUS_SERVICE_IDS` | No | none | Comma-separated service UUIDs whose outages feed the public cloud status data. Empty makes the sweep a no-op. | `00000000-0000-0000-0000-000000000000` |
| `CLOUD_STATUS_DRAINER_ENABLED` | No | `false` | Exact `true` starts the cloud status drainer (it also rewrites cloud monitor status). | `true` |
| `CLOUD_STATUS_POLL_INTERVAL` | No | `10s` | Poll interval of that drainer. | `10s` |
| `INCIDENT_REPORT_POLL_INTERVAL` | No | `5s` | Poll interval of the incident report drainer. | `5s` |
| `INCIDENT_DEFAULT_SERVICE_ID` | No | none | Service UUID whose support group is used when an incident's own service has none. Must be a UUID. | `00000000-0000-0000-0000-000000000000` |
| `INCIDENT_EXTRA_ASSIGNMENT_GROUP_IDS` | No | none | Comma-separated group UUIDs an incident may be created in even though they are the support group of no service (for example a security operations group). They are accepted by `POST /incidents` and offered by `POST /groups/search` with `supportGroupsOnly`; inactive groups still are not. Entries are trimmed, empty ones ignored; every entry must be a UUID or startup fails. Match is by id, never by name. | `791ae9fb-1b41-8d10-a30d-edb8b04bcbd4` |
| `SR_ALERT_SRE_TEAM_IDS` | No | none | Comma-separated team UUIDs whose new service requests are auto-assigned and acknowledged. All entries must be UUIDs. | `00000000-0000-0000-0000-000000000000` |
| `SPECIALIST_HANDOFF_CONFIG` | No | none | JSON routing for "escalate to specialist team" handoffs. Validated at startup; empty hands nothing off. | `<json>` |
| `SPECIALIST_HANDOFF_GITHUB_TOKENS` | No | none | One-line JSON object mapping credential names to GitHub tokens used to file handoff issues. Secret. | `{"<name>":"<token>"}` |
| `ESCALATION_EL1_AMERICAS_TL_EMAILS` | No | none | Comma-separated fixed EL1 escalation recipients. Each must look like an email address. | `<email>` |
| `ESCALATION_EL2_AMERICAS_TU_EMAILS` | No | none | Comma-separated fixed EL2 recipients. | `<email>` |
| `ESCALATION_EL2_PRODUCT_EMAIL_SERVICE` | No | none | Single EL2 recipient chosen for the matching product family. | `<email>` |
| `ESCALATION_EL2_PRODUCT_EMAIL_IDENTITY_SERVER` | No | none | Single EL2 recipient chosen for the matching product family. | `<email>` |
| `ESCALATION_EL2_PRODUCT_EMAIL_DEFAULT` | No | none | Single EL2 recipient for any other product. | `<email>` |
| `CASE_ALERTS_SECRET_KEY` | No | none | Base64 of 32 bytes. Only needed to open chat-room URLs saved sealed by an earlier migration. Secret. | `<base64-32-bytes>` |
| `CASE_ALERTS_SECRET_KEY_ID` | No | `1` | Key id stored beside each sealed URL. | `1` |
| `CASE_ALERTS_RUNTIME_CLIENT_IDS` | No | none | Comma-separated service client ids allowed to read case alert settings including webhook URLs. Unset means any internal service client may. | `<service-client-id>` |
| `CASE_ALERTS_ENVIRONMENT` | No | none | Label shown in the alert settings UI. | `staging` |

### GitHub integration (optional)

Enabled only when `GITHUB_INTEGRATION_ENABLED=true`, a token (`GITHUB_TOKEN` or entries in
`GITHUB_REPO_TOKENS`) and `GITHUB_INTEGRATION_LOGIN` are set, and a database is available. The
inbound webhook signature is checked by a separate component, so no webhook secret is read here.

| Variable | Required | Default | Description | Example |
| -------- | -------- | ------- | ----------- | ------- |
| `GITHUB_INTEGRATION_ENABLED` | No | `false` | Exact `true` turns the integration on. | `true` |
| `GITHUB_API_BASE_URL` | No | `https://api.github.com` | API root; set for GitHub Enterprise. | `https://api.github.com` |
| `GITHUB_TOKEN` | Conditional | none | Token for outbound calls. Secret. | `<github-token>` |
| `GITHUB_REPO_TOKENS` | Conditional | none | One-line JSON mapping `owner/repo` (or `owner`) to a token. Falls back to `GITHUB_TOKEN`. Malformed JSON stops startup. Secret. | `{"<owner>/<repo>":"<token>"}` |
| `GITHUB_INTEGRATION_LOGIN` | Conditional | none | The integration's own GitHub login; its events are ignored. | `<bot-login>` |
| `GITHUB_REPO_CONFIG_PATH` | No | built-in path | File in each repository that maps it to an account and project. | `<path-in-repo>` |
| `GITHUB_SR_SERVICENOW_FIRST` | No | `false` | `true` (dual-write data source only) creates service requests raised from issues in the external system first. | `false` |
| `GITHUB_OUTBOUND_INTERVAL` | No | `15s` | Outbound queue drain interval when the last pass came back short. | `15s` |
| `GITHUB_LABEL_TYPE_INCIDENT` | No | from `.github/labels.yml` | Override label. | `<label>` |
| `GITHUB_LABEL_TYPE_SERVICE_REQUEST` | No | from `.github/labels.yml` | Override label. | `<label>` |
| `GITHUB_LABELS_CLASS` | No | from `.github/labels.yml` | Override label vocabulary. | `<label>` |
| `GITHUB_LABEL_STATUS_ASSIGNED` | No | from `.github/labels.yml` | Override label. | `<label>` |
| `CSM_PORTAL_BASE_URL` | No | none | Base URL of the CSM portal, used to build links in GitHub comments and issues. Empty omits the link. | `https://portal.example.com` |

### Sales-record integration (optional)

The connection variables are all-or-nothing (`SALES_ENTITY_SCOPES` is optional). The
`CSM_MIGRATION_*` flags control migration-period ingest paths.

**Read this before relying on the flag defaults.** `internal/config/config.go` evaluates every
`CSM_MIGRATION_*` flag with `envFlagOn`: it is **on unless the value is `false`** (case-insensitive),
so an unset variable means enabled. The Go doc comments, `.env.example`, and `openapi.yaml`
describe these flags as defaulting to off. The code is the behavior; set `=false` explicitly to
disable a path. A path still needs its other prerequisites (a PostgreSQL-authoritative data source,
and for writes a complete connection) before its routes are registered.

| Variable | Required | Default | Description | Example |
| -------- | -------- | ------- | ----------- | ------- |
| `SALES_ENTITY_BASE_URL` | No (together) | none | Base URL of the sales-record service. | `https://sales.example.com` |
| `SALES_ENTITY_TOKEN_URL` | No (together) | none | OAuth2 token endpoint. | `https://idp.example.com/oauth2/token` |
| `SALES_ENTITY_CLIENT_ID` | No (together) | none | OAuth2 client id. | `<sales-client-id>` |
| `SALES_ENTITY_CLIENT_SECRET` | No (together) | none | OAuth2 client secret. Secret. | `<sales-client-secret>` |
| `SALES_ENTITY_SCOPES` | No | none | OAuth2 scopes. | `<scopes>` |
| `SALESFORCE_INGEST_RETRY_INTERVAL` | No | `5m` | Retry interval for failed ingests. `0` disables the worker; an invalid or negative value also disables it, with a warning. | `5m` |
| `CSM_MIGRATION_SALESFORCE_ACCOUNT_INGEST_ENABLED` | No | on (see above) | Accept account change events. | `false` |
| `CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED` | No | on | Accept project-contact and contact change events. | `false` |
| `CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED` | No | on | Accept opportunity change events. | `false` |
| `CSM_MIGRATION_SALESFORCE_PROJECT_INGEST_ENABLED` | No | on | Accept project change events (updates existing rows). | `false` |
| `CSM_MIGRATION_SALESFORCE_PROJECT_INSERT_ENABLED` | No | on | Let the project ingest insert projects that do not exist yet. | `false` |
| `CSM_MIGRATION_SALESFORCE_PARTNER_INGEST_ENABLED` | No | on | Refresh partner account relationships. | `false` |
| `CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED` | No | on | Register `POST /users/me/memberships/register`. | `false` |
| `CSM_MIGRATION_PORTAL_WRITES_ENABLED` | No | on | Register the portal membership write endpoints; also requires a PostgreSQL-authoritative data source and a complete sales-record connection. | `false` |
| `CSM_MIGRATION_CUSTOMER_ENGAGEMENT_INGEST_ENABLED` | No | on | Register `POST /customer-engagements/allocation-events`. | `false` |
| `CUSTOMER_ENGAGEMENT_FIREFIGHTING_TYPE_ID` | No | none | 32-character lowercase hex id of the firefighting engagement type; unset skips creating those engagements. | `<32-hex-chars>` |

## License

Apache License 2.0. See [LICENSE](LICENSE).
