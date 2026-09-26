# CSM Scheduled Tasks

A single Choreo Scheduled Task (Go 1.26+) that runs several independently-scheduled sub-crons —
"flows and actions" that need to happen on a recurring basis — behind one shared trigger, with
failure tracking and automatic retry backed by entity-service.

Each invocation runs exactly one tick and exits — there is no long-running process here, unlike
every other Go component in this repo. Choreo's own Scheduled Task trigger supplies the cadence.

## Why this shape

Choreo bills/schedules per component trigger, not per job, so registering ten Choreo Scheduled
Tasks for ten sub-crons is both operationally heavier and harder to reason about than one component
that fans out internally. The trade-off this design makes to support that: every sub-cron's
schedule is evaluated against a single shared "driver" cadence (this component's own Choreo
trigger), which must be at least as frequent as the tightest sub-cron registered.

Failure handling doesn't use a fixed retry-count cap. A failed sub-cron keeps retrying on every
eligible tick until it either succeeds, or its own next scheduled period comes due — at which point
the old, still-unresolved attempt is abandoned ("superseded") rather than resurrected, and only the
newest period is chased from then on. See `CLAUDE.md` for the full design ("period keys",
"supersede") and why an earlier two-pass version of the tick algorithm collapsed into one pass.

## Project structure

```text
csm-scheduled-tasks/
├── cmd/server/main.go       # Entry point — builds the registry, runs one Engine.Tick, exits
├── internal/
│   ├── schedule/period.go   # PeriodKey(cronExpr, now) — the "most recent scheduled firing" concept
│   ├── registry/registry.go # Task{Name, Schedule, Handler, RetryBackoff, To, Cc}
│   ├── engine/engine.go     # Tick: claim → run → report back, once per task per invocation
│   ├── ledger/client.go     # entity-service client for this component's own durable state (Attempt/Complete/Fail/DeleteResolvedBefore)
│   ├── entitycases/client.go # Separate, read-only entity-service client for case search — used by report-style sub-crons (see CLAUDE.md, "Stale cases report")
│   ├── notify/              # Email sending — same internal email service csm-notification-service uses; alert.html (failure alerts) and stale_cases_report.html (see CLAUDE.md, "Per-task report emails")
│   ├── httpsec/httpsec.go   # Shared HTTPS/redirect guards for ledger, entitycases, and notify's OAuth2 clients
│   ├── housekeeping/        # Sub-cron: deletes old resolved scheduled_task_run rows (see CLAUDE.md, "Housekeeping")
│   ├── stalecases/          # Sub-cron: reports cases open too long (see CLAUDE.md, "Stale cases report")
│   └── apierror/errors.go   # Typed upstream-error wrapper, shared by ledger, entitycases, and notify
```

No `.choreo/component.yaml` here — this component is created as a Choreo "Scheduled Task" (not
"Service") directly in Choreo Console, where its cron trigger is also configured; there's no
endpoint declaration this component needs to check in.

## Running locally

```bash
# from operations/csm-scheduled-tasks
cp .env.example .env   # fill in CUSTOMER_ENTITY_SERVICE_* at minimum
go run ./cmd/server
```

Each run is one tick against whichever entity-service `.env` points at, then the process exits.
Two sub-crons are registered today: `housekeeping_cleanup` (see `CLAUDE.md`, "Housekeeping") and
`stale_cases_report` (see `CLAUDE.md`, "Stale cases report"); see `CLAUDE.md` ("Adding a sub-cron")
for how to register another.

## Environment variables

See `CLAUDE.md` for the full table. At minimum, `CUSTOMER_ENTITY_SERVICE_BASE_URL` and the shared
`OAUTH2_CLIENT_ID`/`OAUTH2_CLIENT_SECRET`/`OAUTH2_TOKEN_URL` are required — this component cannot
claim or report anything without entity-service. The same `OAUTH2_*` credentials also back the
email client, and any future service client — see `CLAUDE.md`'s own note on that convention.

## Commands

```bash
go vet ./...              # vet
go test -race ./...       # race-detector tests
go build -o server ./cmd/server
```

## Entity-service dependency

This component has no database of its own. All claim/retry/succeed/fail state lives in
entity-service's `scheduled_task_run` table, added alongside this component (see that repo's
`migrations/000014_create_scheduled_task_run.up.sql`, `CLAUDE.md` — "Scheduled task runs" — and
`openapi.yaml`). Deploy that migration before this component's first real invocation.
