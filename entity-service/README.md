# Customer Entity Service

## Tech Stack

| Layer     | Technology               |
| --------- | ------------------------ |
| Language  | Go 1.26.3                |
| Framework | Gin                      |
| Database  | PostgreSQL 15+           |
| Driver    | pgx v5 (connection pool) |

## Project Structure

```text
entity-service/
├── cmd/api/main.go              # Entry point — wires all layers and starts the server
├── internal/
│   ├── config/config.go         # Env-based config, builds PostgreSQL DSN
│   ├── db/
│   │   ├── postgres.go          # pgxpool setup and connection
│   │   └── migrate.go           # Schema migration runner
│   ├── domain/entity.go         # Shared domain types (Case, Page, inputs)
│   ├── events/events.go         # Envelope{Type, EntityID, Payload} — the case-events wire shape, kept in sync by hand with apps/csm-portal/backend and csm-notification-service's own copies
│   ├── eventbus/
│   │   ├── config.go            # Config + SASL/PLAIN setup for Azure Event Hub's Kafka-compatible endpoint
│   │   ├── producer.go          # Producer — publish a record, wait for ack
│   │   └── logger.go            # Bridges kafka-go's Logger/ErrorLogger to slog
│   ├── service/
│   │   ├── interfaces.go        # CaseRepository and CaseService interfaces
│   │   ├── entity_service.go    # Business logic — pagination, validation
│   │   ├── event_publisher_service.go # EventPublisherService.Publish — builds the envelope, publishes it, records a failure if Event Hub doesn't ack (wired in via routes.go; called from snCaseService.CreateCase and snIncidentService.CreateIncident)
│   │   └── sla_status_service.go # SLAStatusService — lists currently-active SLA clocks, read live from the "sla" table
│   ├── repository/
│   │   ├── entity_repo.go       # SQL queries against the "case" table
│   │   └── tx.go                # Transaction helper
│   ├── handler/
│   │   ├── entity_handler.go    # HTTP handler — bind JSON, call service, respond
│   │   └── health_handler.go    # /healthz and /readyz probes
│   ├── server/
│   │   ├── server.go            # Gin engine setup, middleware registration
│   │   └── routes.go            # URL → handler mapping
│   ├── middleware/
│   │   ├── logger.go            # Request logging
│   │   ├── recovery.go          # Panic recovery → 500
│   │   └── timeout.go           # Per-request context deadline
│   └── apierror/errors.go       # Sentinel errors and JSON error responder
├── migrations/                  # SQL migration files (up/down)
├── queries/                     # Raw SQL queries (sqlc source)
├── deploy/                      # Dockerfile and docker-compose
├── sqlc.yaml                    # sqlc code generation config
├── .env.example                 # Environment variable template
└── Makefile                     # Common dev targets
```

## Prerequisites

- Go 1.21+
- PostgreSQL 15+ (local via Docker or Azure)
- (Optional) [sqlc](https://sqlc.dev/) for query code generation

## Quick Start

### 1. Clone and install dependencies

```bash
git clone https://github.com/wso2-open-operations/cs-tools
cd cs-tools/entity-service
go mod download
```

### 2. Configure environment

```bash
cp .env.example .env
```

Edit `.env` with your database credentials:

```env
DB_HOST=localhost
DB_PORT=5434
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=your_db
DB_SSLMODE=disable       # use "require" for Azure PostgreSQL
```

### 3. Run

```bash
go run cmd/api/main.go
```

Server starts at `http://localhost:8080`.

## Request Flow

```text
HTTP Request
  └── Gin Router
        └── Middleware (logger, recovery, timeout)
              └── Handler          — bind JSON, validate
                    └── Service    — business logic, pagination
                          └── Repository  — SQL query
                                └── PostgreSQL
```

## Environment Variables

| Variable    | Required | Default   | Description       |
| ----------- | -------- | --------- | ----------------- |
| DB_HOST     | Yes      | localhost | PostgreSQL host   |
| DB_PORT     | Yes      | 5432      | PostgreSQL port   |
| DB_USER     | Yes      | postgres  | Database user     |
| DB_PASSWORD | Yes      | —         | Database password |
| DB_NAME     | Yes      | postgres  | Database name     |
| DB_SSLMODE  | No       | require   | SSL mode          |
| SERVER_PORT | No       | 8080      | Main API listener port |
| HEALTH_PORT | No       | 8081      | Health probe listener port; must differ from `SERVER_PORT`, and must be left at its default in Choreo deployments (see below) |

> `.env` file is loaded automatically if present. Absent `.env` is silently ignored; a malformed one causes a fatal startup error.

## Health probes

The service listens on **two** ports. `SERVER_PORT` (8080) carries the API and is published at
**Organization** visibility. `HEALTH_PORT` (8081) carries nothing but the health probes and is
published at **Public** visibility, so external alerting can poll it without credentials — see
`.choreo/component.yaml`, which declares one Choreo endpoint per port.

`.choreo/component.yaml` declares both ports statically and nothing reconciles them with the
environment at deploy time, so **overriding `SERVER_PORT` or `HEALTH_PORT` in a Choreo deployment
routes traffic to a port with no listener.** For the health endpoint that is particularly
unhelpful: a probe that never answers looks exactly like the outage it exists to report. Override
these locally only.

The split is deliberate and is the security boundary itself: what is publicly reachable is decided
by which mux a handler is registered on (`internal/server/health.go`), not by a gateway path rule
in another system that fails open if it is ever wrong. Nothing but the two probes below is
reachable on the public port, whatever happens to that config. **Do not point the public Choreo
endpoint at port 8080, and do not register business routes on the health mux.**

| Probe | Port | Answers |
| ----- | ---- | ------- |
| `GET /health` | 8080 and 8081 | Always `200 {"status":"ok"}`. Pure liveness — makes no dependency calls, so a database outage never gets the instance restarted or pulled from rotation. |
| `GET /health/database` | 8081 only | `200 {"status":"ok","database":"up"}` when a round trip to PostgreSQL succeeds, `503 {"status":"unavailable","database":"down"}` when it fails. |

Two probes rather than one combined check, so alerting can tell "the component is down" apart from
"the component is up but its database is not".

The database probe alerts on a **PostgreSQL** outage specifically. A deployment running without a
connection pool (`DATA_SOURCE=servicenow`, where reads go through the ServiceNow integration
service) has no PostgreSQL to be out, so it answers `200` with `database: "not_configured"` rather
than a 503 that would fire continuously against a database that is not supposed to exist.

Failure bodies deliberately carry no error detail — no driver message, host, or port. The
endpoint is public, so it reports only whether the dependency is up, never anything about the
infrastructure behind it. Both probes send `Cache-Control: no-store`, since a cached 200 would
keep reporting healthy straight through an outage.

### Directory vocabularies — moved

`CSM_TEAM_REGISTRY` and `CSM_USER_ROLES` are **no longer read by this service**. The team registry
and the assignable-role allow-list are organisation vocabulary; they now live in the CSM portal
backend, which resolves them once at startup and serves `POST /teams/search` and
`POST /roles/search` from memory. This service holds no organisation vocabulary at all.

Configure them in `apps/csm-portal/backend/.env` — see that module's
[README](../apps/csm-portal/backend/README.md#directory-vocabularies). Setting them here has no
effect.

### Event Hub publishing

`internal/service.EventPublisherService` publishes domain events to Event Hub's Kafka-compatible
endpoint for `csm-notification-service` to consume. Constructed in `internal/server/routes.go`,
gated on **both** `EVENT_HUB_BROKER` being set (not `DATA_SOURCE`) **and** `EVENT_PUBLISHING_ENABLED`
being `"true"` — either left unset/false, nothing changes; `CreateCase`/`CreateIncident`/etc.
behave exactly as before this was wired in. `EVENT_PUBLISHING_ENABLED` defaults to `false`, so a
fully-configured Event Hub connection still publishes nothing until it's explicitly turned on.

Seven ServiceNow-data-source-only call sites publish today: `case.created`, `case.comment_added`,
`case.status_changed`, `case.assigned`, `case.acknowledged`, `case.severity_changed`, and
`incident.created`. See entity-service's `CLAUDE.md` ("Event Hub publishing") for the full
reasoning behind each, including why all seven publish synchronously with a bounded timeout
rather than async.

| Variable | Description |
|---|---|
| `EVENT_HUB_BROKER` | Kafka bootstrap address: `<namespace>.servicebus.windows.net:9093` — the feature gate (optional) |
| `EVENT_HUB_CONNECTION_STRING` | The namespace's Shared Access Policy connection string — must be namespace-scoped (no `EntityPath`), not scoped to a single Event Hub (required once `EVENT_HUB_BROKER` is set) |
| `EVENT_HUB_TOPIC` | Event Hub (Kafka topic) name, e.g. `case-events` — must match `csm-notification-service`'s own `EVENT_HUB_TOPIC` (required once `EVENT_HUB_BROKER` is set) |
| `EVENT_PUBLISHING_ENABLED` | Set to `true` to actually publish. Defaults to `false` — safe by default even with Event Hub fully configured (optional) |
| `AUTH_ISSUER` / `AUTH_JWKS_URL` | Asgardeo issuer and JWKS URL for validating the `x-user-id-token` user ID token -- always on, there is no flag to disable it. Required; the JWKS must load at startup or the process exits. A present-but-invalid `x-user-id-token` is a 401 on every route. `x-jwt-assertion` (the client-credentials assertion) is decoded only, never verified against these -- see entity-service's `CLAUDE.md` ("Token validation and caller-scoped access") for why |
| `AUTH_USER_TOKEN_AUDIENCES` | Comma-separated client ids an ID token's `aud` must contain to count as a user token; required |
| `AUTH_CLOCK_SKEW` | Leeway for `exp` (default `30s`) |
| `AUTH_INTERNAL_CLIENT_IDS` | Comma-separated Asgardeo application client ids trusted with unconditional full access to every project and case (checked against a client-credentials `x-jwt-assertion` token), regardless of any `x-user-id-token` the same request also carries. A caller not in this list is resolved purely from its `x-user-id-token` instead. Which real client ids go here is a deployment decision, but a service that calls the scoped endpoints directly with only a client-credentials token gets a 401 unless it is listed (optional) |
| `CUSTOMER_ROLES` | Comma-separated ServiceNow role names whose presence on a case comment's author marks it a customer reply — see "Customer reply state transition" below. No default; unset means that path never fires (optional) |

### SLA status

`GET /sla-status` reads SLA state live from the `sla` table (migration `000052`), which
ServiceNow's own SLA engine populates via sync — real `businessElapsedPercent`/`hasBreached`/
`stage` per `(work_item, sla_policy)`. Has no ServiceNow equivalent of its own — always backed
by Postgres regardless of `DATA_SOURCE`, same as `event_publish_failures`. `clockType` is
`response`/`workaround`/`resolution`, lower-cased from `sla_policy.target`.

Returns every currently-active clock across every case-like work item in one paginated list
(default limit `500`, max `2000` — much higher than this service's other paginated endpoints,
since the one real caller is `csm-notification-service` polling periodically, not a UI list).
There is no registration step and nothing for this service to schedule or track in-process any
more: the synced `sla` row already reflects pauses, completions, and breaches, because
ServiceNow's own SLA engine reacted to those events on its own side. This replaces an earlier
`sla_clocks` design (a hand-registered clock per case, using a hardcoded severity->duration
guess) that existed before the `sla` table did — see `CLAUDE.md`'s "SLA status" section for
the full history.

`csm-notification-service`'s SLA engine polls `GET /sla-status` periodically and diffs
`businessElapsedPercent` against what it already alerted on itself (its own Redis state, not
anything this service tracks), sending a Google Chat card directly on a newly-crossed tier —
not routed through this service.

### Customer reply state transition

When a customer-visible comment (not a work note) from a user holding one of the `CUSTOMER_ROLES`
roles (looked up via `SNUserService.SearchUsers`, filtered by the comment author's email) arrives
while the case is `Awaiting Info`/`Solution Proposed`, `sn_case_service.go`'s
`applyCustomerReplyStateTransition` moves it back to `Waiting on WSO2` — a customer reply means
it's WSO2's turn to act again. Implemented as a plain in-process call to this service's own
`UpdateCase`, not a separate ServiceNow PATCH — so it gets `case.status_changed` publishing for
free, with no duplicated logic.

### Scheduled task runs

`scheduled_task_run` (migration `000045`) is durable claim/retry state for `operations/csm-scheduled-tasks`, a single Choreo
Scheduled Task that fans out to many independently-scheduled sub-crons on one shared driver
cadence. Has no ServiceNow equivalent — always backed by Postgres. No stored status column: a row's
state is always derivable from which timestamp is set (`succeededOn`, `supersededOn`,
`nextRetryOn`) — see entity-service's own `CLAUDE.md` ("Scheduled task runs") for the full design
and `operations/csm-scheduled-tasks`'s `CLAUDE.md` for the "period keys"/"supersede" reasoning
behind it.

Consumed by that component's engine, which claims a period via
`POST /scheduled-tasks/attempts`, then reports back via
`PATCH /scheduled-tasks/attempts/{id}` (`{attemptCount, status: "succeeded"|"failed", ...}`).
`GET /scheduled-tasks/attempts?status=<filter>` is monitoring-only, and
`DELETE /scheduled-tasks/attempts?resolvedBefore=<ts>` backs that same component's own self-hosted
`housekeeping_cleanup` sub-cron (`internal/housekeeping`), which calls it daily.

## Security Scanning

Run [gosec](https://github.com/securego/gosec) to check for common security issues:

```bash
# Install gosec (once)
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run from entity-service
gosec -fmt=text ./...
```

The scan should report **0 issues**. If a new finding appears, fix the root cause before merging — do not suppress it without a code review.

Run [govulncheck](https://golang.org/x/vuln/cmd/govulncheck) to check for known vulnerabilities:

```bash
# Install govulncheck (once)
go install golang.org/x/vuln/cmd/govulncheck@latest

# Run from entity-service
govulncheck ./...
```

The scan should report **no vulnerabilities**. Most findings are Go standard-library CVEs tied to the toolchain patch pinned in `go.mod`'s `go` directive — bump it to the latest `1.26.x` patch and run `go mod tidy` to resolve them.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
