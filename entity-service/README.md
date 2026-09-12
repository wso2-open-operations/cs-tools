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
│   │   └── sla_clock_service.go # SLAClockService — register/get/mark-tier-reached for a case's SLA clocks
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

> `.env` file is loaded automatically if present. Absent `.env` is silently ignored; a malformed one causes a fatal startup error.

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
| `SUPPORT_ENGINEER_ROLE` | ServiceNow role name whose presence on a case comment's author completes the case's "response" SLA clock — see "SLA clocks" below. No default; unset means that specific completion path never fires (optional) |
| `CUSTOMER_ROLES` | Comma-separated ServiceNow role names whose presence on a case comment's author marks it a customer reply — see "Customer reply state transition" below. No default; unset means that path never fires (optional) |

### SLA clocks

`sla_clocks` (migration `000011`, display columns added in `000014`) durably tracks per-case SLA
timers — `caseId`/`clockType`, `startedAt`/`dueAt`, up to three tier-crossing timestamps
(`reached50At`/`reached75At`/`reached100At`), `pausedAt`, and eight display-only fields (case
number/WSO2 case id/title/type/product/team/priority/state, a point-in-time snapshot from
registration). Has no ServiceNow equivalent — always backed by Postgres regardless of
`DATA_SOURCE`, same as `event_publish_failures`. `clockType` is a caller-defined string, not a
fixed enum, but only three are actually used: `response`, `workaround`, `resolution`.

Durations come from WSO2's own [support policy](https://wso2.com/licenses/support-policy/6.0)
(Enterprise plan), keyed by the case's severity — see `internal/service/sla_policy.go`. Every
case gets a `response` clock; `LOW` severity gets only that one (no fixed
Workaround/Resolution SLA, "best efforts"). `sn_case_service.go`'s `CreateCase` publishes
`sla.clock.register` (consumed by `csm-notification-service`'s SLA timer engine,
`internal/slaengine`, which registers the clock via `POST /cases/{caseId}/sla-clocks` and starts
tracking 50/75/100% elapsed) unconditionally — independent of whether the case has any watchers
to email.

Pause/resume/completion are direct, in-process calls from `sn_case_service.go` to
`SLAClockService` — no Event Hub round trip:

- A case state change to `Awaiting Info`/`Solution Proposed` pauses `workaround`+`resolution`;
  any other state resumes both.
- The case closing completes `resolution` (claims all three tiers via
  `SetSLAClockTierReached`) and pauses `workaround` — `workaround` has no completion trigger
  wired up yet (see the `// TODO` in `applyCaseStateSLAEffects`).
- A customer-visible comment (not a work note) from a user holding the `SUPPORT_ENGINEER_ROLE`
  role (looked up via `SNUserService.SearchUsers`, filtered by the comment author's email)
  completes `response`.

`csm-notification-service`'s SLA timer engine reads a clock back via
`GET /cases/{caseId}/sla-clocks/{clockType}` to check `pausedOn` before firing a tier, and
records a crossed tier idempotently via `PATCH /cases/{caseId}/sla-clocks/{clockType}/tiers/{tier}`
with `{"status": "reached"}` — the same endpoint "complete early" reuses to pre-claim all three
tiers at once, which is what suppresses a later spurious breach alert for an already-satisfied
clock. On a genuine breach it sends a Google Chat card directly (not routed through this
service).

### Customer reply state transition

When a customer-visible comment (not a work note) from a user holding one of the `CUSTOMER_ROLES`
roles (looked up the same way as `SUPPORT_ENGINEER_ROLE`, via `SNUserService.SearchUsers` filtered
by the comment author's email) arrives while the case is `Awaiting Info`/`Solution Proposed`,
`sn_case_service.go`'s `applyCustomerReplyStateTransition` moves it back to `Waiting on WSO2` — a
customer reply means it's WSO2's turn to act again. Implemented as a plain in-process call to this
service's own `UpdateCase`, not a separate ServiceNow PATCH — so it gets `case.status_changed`
publishing and the SLA pause/resume side effects above for free, with no duplicated logic.

### Scheduled task runs

`scheduled_task_run` (migration `000013` — the one intentionally singular table name in this
schema) is durable claim/retry state for `operations/csm-scheduled-tasks`, a single Choreo
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
