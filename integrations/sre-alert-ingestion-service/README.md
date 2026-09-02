# SRE Alert Ingestion Service

Go backend service that ingests normalized alerts from external
monitoring/alerting tools (Azure, Site24x7, Grafana, Datadog, Prometheus,
etc.) and turns each into a platform incident, by calling the sibling M2M
gateway `csm-integration-service`. If that call fails, the alert is durably
buffered in this service's own dedicated Postgres database, retried with
exponential backoff, and — if it keeps failing — escalated via a Twilio
voice call to SRE, through a channel independent of the platform.

**The entire point of this component is to not be a single point of failure
on the platform's own availability.** Buffering survives this service's own
restarts and a regional failover, not just an in-process retry loop.

## Quick Start

```bash
# from integrations/sre-alert-ingestion-service
psql "$SRE_ALERT_DATABASE_URL" -f migrations/0001_create_alert_buffer.up.sql
go run ./cmd/server/main.go
```

The server automatically loads `.env` from the working directory on startup
(silently ignored if absent).

Server starts at `http://localhost:8080`.

## Overview

- Default port: `8080`
- Runtime: Go `1.26+`
- Entry point: `cmd/server/main.go`
- Authentication:
  - Incoming requests (`POST /alerts`): **none at the app layer.** This
    service is fronted by Choreo's API Manager gateway (subscription + M2M
    app auth), matching `csm-integration-service`'s own convention — see
    that service's `CLAUDE.md` for the full rationale, which applies
    identically here.
  - Outbound calls to `csm-integration-service`: OAuth2 client credentials
    grant (managed automatically), always M2M.
  - Outbound calls to Twilio: HTTP Basic Auth (Account SID / Auth Token) —
    Twilio has no OAuth2 flow.

## Architecture

```
SRE monitoring tool
       |  POST /alerts
       v
sre-alert-ingestion-service
       |  1. validate + map to a CreateIncidentRequest
       |  2. persist to alert_buffer (status=pending) -- BEFORE any delivery attempt
       |  3. respond 202 {id}
       |
       |  (background worker, polling alert_buffer on a timer)
       |  4. attempt POST /incidents on csm-integration-service
       |     success -> status=delivered
       |     retryable failure, budget remaining -> status stays pending, retry_count++
       |     retryable failure, budget exhausted -> Twilio voice call, status=escalated
       |     non-retryable failure (e.g. 400) -> status=failed, no escalation
       v
csm-integration-service  ---->  entity-service  ---->  platform incident store
```

Persist-then-attempt, not attempt-then-persist: nothing is lost even if this
process crashes between accepting a request and its first delivery attempt.

## Prerequisites

- Go `1.26+` — [install](https://go.dev/doc/install)
- PostgreSQL — a **dedicated** database for this service's buffer (never
  CSM's own database — see "Why a dedicated database" below)

## Configuration

Copy `.env.example` to `.env` and fill in the values:

| Variable | Description |
|---|---|
| `SRE_ALERT_DATABASE_URL` | Buffer database connection string (`postgres://...`) |
| `CSM_INTEGRATION_BASE_URL` | Base URL of `csm-integration-service` |
| `CSM_INTEGRATION_TOKEN_URL` | OAuth2 token endpoint for `csm-integration-service` |
| `CSM_INTEGRATION_CLIENT_ID` | OAuth2 client ID |
| `CSM_INTEGRATION_CLIENT_SECRET` | OAuth2 client secret |
| `CSM_INTEGRATION_SCOPES` | Comma-separated OAuth2 scopes |
| `SRE_ALERT_CALLER_ID` | A real, provisioned platform user id — see "Known limitations" |
| `SRE_ALERT_MAX_RETRIES` | Retryable-failure count before escalation (default `3`) |
| `SRE_ALERT_POLL_INTERVAL_SECONDS` | How often the worker scans the buffer (default `15`) |
| `TWILIO_ACCOUNT_SID` | Twilio account SID |
| `TWILIO_AUTH_TOKEN` | Twilio auth token |
| `TWILIO_FROM_NUMBER` | Twilio-provisioned caller-ID number (E.164) |
| `SRE_ALERT_ONCALL_NUMBER` | Static on-call number every escalation call rings (E.164) — see "Known limitations" |
| `TWILIO_VOICE` | Optional: TTS voice for the escalation call |
| `TWILIO_LANGUAGE` | Optional: TTS language/locale |
| `TWILIO_API_BASE_URL` | Optional: override Twilio's API base (tests / regional edge) |
| `PORT` | Server listen port (default `8080`) |

## Database / migrations

This service owns a **dedicated** Postgres database — never CSM's own
database. Sharing a database would reintroduce exactly the coupling this
service exists to remove: if CSM's database has a problem, this service's
ability to buffer alerts must not degrade with it.

Migrations follow this repo's `up`/`down` SQL-pair convention (matching
`entity-service/migrations` and
`integrations/sftpgo-authentication-service/db/migrations`), applied via
`psql`, not from application code:

```bash
psql "$SRE_ALERT_DATABASE_URL" -f migrations/0001_create_alert_buffer.up.sql
```

Driver: `github.com/jackc/pgx/v5` via `database/sql` (the `pgx/v5/stdlib`
adapter) — the same choice already established in this repo by
`entity-service` and `integrations/sftpgo-authentication-service`, used here
for consistency rather than re-evaluated independently.

### `alert_buffer` schema

| Column | Purpose |
|---|---|
| `id` | UUID primary key, generated **client-side** by `internal/idgen` before the row is persisted (not by the column's `gen_random_uuid()` default — see "Duplicate-incident dedup" below for why) — returned to the caller as the buffered alert's id |
| `received_at` | When the alert was accepted |
| `payload` | The already-mapped `CreateIncidentRequest` JSON (not the raw inbound alert) — see `internal/handler.MapToIncident`. Its `Subject` is tagged with this row's own `id` (`internal/csmclient.DedupTag`) |
| `status` | `pending` \| `delivered` \| `escalated` \| `failed` |
| `retry_count` | Number of failed, retryable delivery attempts so far |
| `last_attempt_at` | Timestamp of the most recent delivery attempt |
| `last_error` | The most recent attempt's error, if any |
| `incident_id` | Set once delivery succeeds |
| `escalated_at` | Set once the Twilio escalation call is placed |

## Severity mapping

`AlertRequest.Severity` maps to `CreateIncidentRequest`'s `Impact`/`Urgency`
(`internal/severity.MapImpactUrgency`) — this service's own choice; no
upstream contract dictates it:

| Severity | Impact | Urgency |
|---|---|---|
| `critical` | HIGH | HIGH |
| `major` | HIGH | MEDIUM |
| `minor` | MEDIUM | MEDIUM |
| `warning` | LOW | MEDIUM |
| `ok` | LOW | LOW |
| *(anything else)* | LOW | LOW — fails safe, not open |

`AlertRequest.Source` maps to `ContactType` (`internal/severity.MapContactType`)
only where an existing enum value fits: `azure`→`AZURE`, `site24x7`/`site247`→`SITE_247`,
`sentinel`/`microsoft-sentinel`→`SENTINEL`. Any other source omits `ContactType`
entirely rather than guessing.

`AlertRequest.Category` passes through uppercased if it's one of `INQUIRY` /
`SERVICE_INTERRUPTION` / `SECURITY` (case-insensitive); otherwise it defaults
to `SERVICE_INTERRUPTION` (`internal/severity.MapCategory`).

## API Endpoints

- `GET /health` — liveness/readiness; reports `503` if the buffer database
  is unreachable
- `POST /alerts` — accepts a normalized alert, persists it to the buffer,
  responds `202` with `{"id": "<buffered-alert-id>"}`. Never attempts
  delivery inline — see "Architecture" above.

See `openapi.yaml` for the full request/response schema.

## Retry / escalation behavior

A background worker (`internal/worker`) polls `alert_buffer` every
`SRE_ALERT_POLL_INTERVAL_SECONDS` for `pending` rows, and attempts
delivery for whichever ones are due per exponential backoff
(`internal/backoff`: 30s base delay, doubling, capped at 30 minutes).

Every `POST /incidents` failure is classified (`internal/worker.isRetryable`):

- **HTTP 400** — the buffered payload itself is invalid. Retrying it can
  never succeed, so the row is marked `failed` immediately: no retry, no
  Twilio escalation (that channel exists for "CSM won't accept this right
  now", not "this request is malformed").
- **Everything else — including HTTP 401** — is retryable. See "Known
  limitations" below for exactly why a 401 is treated as retryable here,
  which is the opposite of how a 401 is normally read.

Once a row accumulates `SRE_ALERT_MAX_RETRIES` retryable failures, the
worker places a Twilio voice call (`internal/notifications.TwilioClient.Escalate`)
and marks the row `escalated` — terminal; this service does not resume
retrying an escalated row automatically.

## Duplicate-incident dedup

A failed `POST /incidents` call does not prove the incident wasn't actually
created — the request can succeed on the far side while the response is
lost (timeout, connection reset, etc.). Retrying blindly in that situation
risks creating a second, duplicate incident for the same alert. This
service guards against that in two parts:

1. **Every incident's `Subject` is tagged with its buffer row's own id.**
   `internal/idgen` generates `alert_buffer.id` client-side, *before* the
   row is persisted (not via the column's `gen_random_uuid()` default), so
   `internal/handler.MapToIncident` can embed it as a dedup tag —
   `internal/csmclient.DedupTag(id)`, format `"[alert:<row-id>]"` — as the
   leading text of `CreateIncidentRequest.Subject`. This is fully within
   this service's own control, unlike `AlertRequest.UniqueIdentifier`
   (vendor-supplied and optional), and guaranteed unique per buffered
   alert.
2. **Before any *retry* (never the first attempt — nothing could exist yet
   on attempt 1), the worker searches for that tag first.**
   `internal/worker.attempt` calls
   `csmclient.Client.SearchIncidentByTag` (`POST /incidents/search` on
   `csm-integration-service`, `searchQuery` = the row's dedup tag) whenever
   `row.RetryCount > 0`. A match means an earlier attempt's incident
   already exists: the row is marked `delivered` against that incident's
   id/number and `POST /incidents` is **not** called again. No match, or
   the search call itself failing, both **fail open toward attempting
   delivery** — the search existing at all must never become a new way to
   silently drop a buffered alert.

**Known limitation:** `POST /incidents/search`, like `POST /incidents`
itself, is ServiceNow-backed and currently also always 401s for the same
missing-end-user-identity reason (see "Known limitations" below). Until
that infrastructure gap is closed, every retry today hits "search 401'd,
proceeding to create anyway" — the dedup check is **structurally correct
and ready to work**, but **not yet actually effective in production**. It
does not itself resolve the identity gap.

## Known limitations

These are deliberate, already-decided states this service does not attempt
to work around — documented here rather than as scattered code comments.

- **`POST /incidents` on `csm-integration-service` currently always returns
  401.** `csm-integration-service` is M2M-only, and the upstream
  entity-service incident-creation operation is ServiceNow-backed,
  requiring a forwarded end-user identity token this stack cannot currently
  supply. This service's retry/buffer/escalate logic treats that 401 as a
  retryable CSM-unavailability signal (see `internal/worker.isRetryable`'s
  doc comment for the full reasoning), so every alert submitted today will
  eventually reach Twilio escalation rather than ever being delivered. This
  is expected until the missing end-user-identity infrastructure exists
  upstream — not a bug in this service.
- **`SRE_ALERT_CALLER_ID` must be a real, provisioned platform user id.**
  The platform has no "system"/machine-caller concept for machine-created
  incidents today. This service does not guess or hardcode a value — the
  operator must provision a real user and configure its id before this
  service can create incidents (moot today anyway, given the point above,
  but the contract is sound for when it isn't). The service refuses to
  start if this is unset.
- **Escalation calls a single static on-call number, not a live rotation.**
  `SRE_ALERT_ONCALL_NUMBER` is fixed config, not looked up against any
  on-call schedule. A future iteration integrating a real rotation (e.g.
  a PagerDuty/Opsgenie lookup before placing the call) would change
  `internal/worker`'s escalation step and `internal/notifications.TwilioClient`
  accordingly.
- **No further fallback if the Twilio escalation call itself fails.** If
  CSM is unreachable and Twilio is *also* unreachable (or misconfigured),
  the row is still marked `escalated` and the failure is logged — there is
  no second notification channel. This is the worst case this service can
  be in by design; a wider on-call/paging integration was out of scope for
  this iteration.

## Testing

```bash
go build ./...
go vet ./...
go test ./...
go test -race ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

Or use `make`:

```bash
make test    # vet + race-detector tests
make build   # vet + test + compile
```

- **Request validation, severity mapping, backoff math, retry/escalation
  branching, and the Twilio client are all unit-tested with hand-rolled
  mocks** (no mocking library, matching this repo's convention) — no real
  Postgres or Twilio required for `go test ./...` to fully exercise this
  service's decision logic.
- **`internal/store`'s Postgres implementation additionally has a real-DB
  integration test** (`internal/store/postgres_test.go`), guarded by
  `SRE_ALERT_TEST_DATABASE_URL` — it skips cleanly when that's unset, so it
  never blocks `go test ./...` in an environment without Postgres. Run it
  against a real (disposable) database:

  ```bash
  docker run -d --rm --name sais-test-pg -e POSTGRES_PASSWORD=testpass \
    -e POSTGRES_DB=sais_test -p 55499:5432 postgres:16-alpine
  SRE_ALERT_TEST_DATABASE_URL="postgres://postgres:testpass@127.0.0.1:55499/sais_test?sslmode=disable" \
    go test ./internal/store/... -v
  docker stop sais-test-pg
  ```

  This test applies the migration itself before each test and exercises the
  actual SQL (JSONB round-trip, NULL handling, the partial `pending` index)
  — coverage an interface mock can't provide.
- `cmd/server` (wiring only) has no dedicated tests, matching this repo's
  convention for wiring-only code (e.g. `csm-integration-service`'s own
  `cmd/server`).

## Security Scanning

```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec -fmt=text ./...
```

## Project Structure

```text
sre-alert-ingestion-service/
├── cmd/server/main.go            # Entry point — routes, worker startup, graceful shutdown
├── internal/
│   ├── apierror/                 # Typed upstream error type (4xx/5xx passthrough)
│   ├── backoff/                  # Pure exponential-backoff math (no I/O)
│   ├── csmclient/                # OAuth2 client credentials HTTP client for csm-integration-service
│   ├── handler/                  # POST /alerts, GET /health, alert-to-incident mapping
│   ├── middleware/                # X-CSM-Correlation-ID, access log, security headers
│   ├── notifications/            # Twilio voice-call escalation channel
│   ├── severity/                 # Severity/source/category mapping tables
│   ├── store/                    # Durable buffer: Store interface + Postgres implementation
│   └── worker/                   # Background retry/escalation loop
├── migrations/
│   ├── 0001_create_alert_buffer.up.sql
│   └── 0001_create_alert_buffer.down.sql
├── .choreo/component.yaml
├── openapi.yaml
└── .env.example
```
