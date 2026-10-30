# CSM Scheduled Tasks

A single scheduled-task component (Go) that runs several independently-scheduled recurring jobs
("sub-crons") behind one shared platform trigger, with failure tracking and automatic retry backed
by the entity-service.

Each invocation runs exactly one tick and exits. There is no long-running process and no HTTP
surface, unlike most other Go components in this repository. The hosting platform's own scheduled
trigger supplies the cadence.

## Contents

- [Architecture](#architecture)
- [Developer guide](#developer-guide)
- [Configuration](#configuration)

## Architecture

### Purpose and place in the platform

This component is the platform's recurring-job runner: housekeeping, scheduled reports, scheduled
publishing, and similar work that must happen on a clock rather than in response to a user action.
It sits beside the other backend services, holds no database of its own, and talks only to
entity-service (and an email service) over HTTPS using OAuth2 client credentials. For how it fits
into the wider CSM platform, see the
[platform architecture](../../apps/csm-portal/README.md#architecture).

```text
 platform scheduled trigger (every DRIVER_INTERVAL)
            |
            v
   csm-scheduled-tasks  (one Engine.Tick per invocation, then exit)
      |             |
      |             +--> email service      (failure alerts, report emails)
      v
   entity-service  (scheduled_task_run ledger, case search, announcements, ...)
```

### Trigger model

- **Driver cadence.** The component is created as a "Scheduled Task" (not a "Service") in the
  hosting platform, whose cron trigger fires it. `DRIVER_INTERVAL` must be set to match that
  trigger. It must be at least as frequent as the tightest sub-cron registered, because every
  sub-cron's schedule is only evaluated when the process runs.
- **Sub-cron schedules.** Each registered task has its own 5-field cron expression (a 6-field form
  with a leading seconds field is also accepted). Schedules are evaluated in the process's local
  time zone; set `TZ=UTC` in the deployment so the documented times mean UTC.
- **Why one component.** Scheduling is per component trigger, not per job, so fanning out
  internally is operationally simpler than one component per job.

### Period keys, retry and supersede

For a task's cron expression `E` and the current time, `internal/schedule.PeriodKey` returns the
most recent firing of `E` at or before now. Every tick between two firings derives the same key, so
retries land on the same ledger row.

On each tick, for each registered task, `internal/engine` does one pass:

1. Compute the current period key.
2. Ask entity-service to claim an attempt (`POST /scheduled-tasks/attempts`). It atomically allows
   or denies: a new period supersedes any still-open older period for the same task and is claimed;
   an existing failed row whose retry time has arrived is claimed again; anything else is denied.
3. If allowed, run the task's handler, then report the outcome
   (`PATCH /scheduled-tasks/attempts/{id}`) as success or failure. A failure sets a next-retry time;
   there is no retry cap.

A failing task therefore retries on every eligible tick until it succeeds or its own next period
comes due, at which point the old attempt is abandoned ("superseded"). Handlers must be idempotent
per period. Every failed attempt sends a failure alert email (see [Alerting](#alerting)).

### Registered tasks

| Task name (ledger key) | Default schedule | Enabled | What it does |
|---|---|---|---|
| `housekeeping_cleanup` | `0 3 * * *` | Always | Deletes resolved (succeeded or superseded) ledger rows older than `HOUSEKEEPING_RETENTION_DAYS`. Open or failed rows are never deleted. |
| `stale_cases_report` | `0 7 * * *` | Always | Emails a report of cases not closed and created at least 30 days ago (fixed threshold). Does nothing and still succeeds if the task has no `to` recipients. |
| `open_cases_report` | `0 8 * * *` | Always | Emails a report of cases still in the `open` state that were created before the start of yesterday (UTC calendar-day boundary). Does nothing and still succeeds if the task has no `to` recipients. |
| `migration_case_report` | `30 22 * * 0` | Always | Emails the weekly migration case scan: migration engagements still open after `MIGRATION_CASE_REPORT_OLDER_THAN_DAYS`, in three tables (customer asked to hold, waiting on the accounts team, neither). Sent to the mail groups of the teams entity-service is configured with (`MIGRATION_CASE_REPORT_TEAM_KEYS` there) plus this task's own `to`/`cc`; fails the run when that audience is empty. Always sent, even with empty tables. |
| `cs_worklogs_daily_report` | `55 3 * * *` | Always | Emails the CS Worklogs daily report for yesterday. 03:55 UTC is 09:25 Asia/Colombo, when the ServiceNow original sends. The reported day is derived in Asia/Colombo, not UTC. Does nothing and still succeeds if the task has no `to` recipients. |
| `cs_worklogs_weekly_report` | `0 16 * * 1` | Always | Emails the CS Worklogs weekly report for the Monday-to-Sunday just gone. 16:00 UTC Monday is 21:30 Asia/Colombo, matching `WeeklyReportAutomation V2`. Does nothing and still succeeds if the task has no `to` recipients. |
| `publish_scheduled_announcements` | `*/15 * * * *` | Always | Finds scheduled announcements whose publish time has arrived and asks entity-service to publish each one. Every due item is attempted independently; failures are joined into one error. |
| `cloud_status_webhooks` | `*/5 * * * *` | Only if `CLOUD_STATUS_ENABLED=true` | Asks entity-service which service-status transitions owe an outbound webhook, posts each to its configured status-dashboard base URL, and records delivery. |
| `availability_recalculation` | `0 3 * * *` | Only if `AVAILABILITY_RECALC_ENABLED=true` | Triggers entity-service to recompute service availability figures; fails the run on a partial sweep or an empty subject list. The arithmetic runs in entity-service. |
| `usage_rollup_daily` | `30 0 * * *` | Only if `USAGE_ROLLUP_ENABLED=true` | Triggers entity-service to rebuild the daily usage rows of the last `USAGE_ROLLUP_DAILY_LOOKBACK_DAYS` days (default 2) before today from the hourly rows. The arithmetic runs in the database. |
| `usage_rollup_monthly` | `30 1 * * *` | Only if `USAGE_ROLLUP_ENABLED=true` | Triggers entity-service to rebuild last month's and the current month's monthly usage rows from the daily rows. |
| `unresolved_products_digest` | `0 6 * * *` | Only does anything once `SUB_CRON_RECIPIENTS` names its recipients | Emails the deployment nodes seen in the last 24 hours whose reported product or version is not in the product catalogue (from entity-service's `GET /instances/unresolved-products`). Sends nothing on a day with nothing to report. |
| `acp_reopen_check` | `30 3 * * *` | Only if `ACP_REOPEN_CHECK_ENABLED=true` | Drives entity-service's daily ACP reopen check (`POST /acp/reopen-check`, page by page): reopens projects ACP suspended whose renewal or payment an event missed, and finishes ones a person set back to Open. Needs entity-service with `CSM_ACP_REOPEN_ENABLED=true`. See CLAUDE.md, "ACP reopen check". |
| `acp_closure` | `0 4 * * 1-5` | Only if `ACP_ENABLED=true` | The Account Closure Process: sends the end-date and invoice notices that are due (90/60/30/15/7 days) and suspends projects at day 0 in CSM and Salesforce. Dry run by default (`ACP_DRY_RUN`). Skips weekends in Colombo time. Retried after 4 hours, not every tick. See CLAUDE.md, "ACP closure". |

Effective promptness of any task is bounded by the shared driver cadence, not by its schedule
string alone. Task names are API contracts: renaming one orphans its history and stops matching any
`SUB_CRON_SCHEDULES` or `SUB_CRON_RECIPIENTS` entry.

### Debug mail mode and tryouts

Two settings exist for checking a report in a real deployment, production included, before its
audience sees it:

- A `debug` list on a report task's `SUB_CRON_RECIPIENTS` entry puts **that one report** in debug
  mail mode: it goes to the listed addresses instead of its real audience, with the real To/Cc named
  in a notice at the top of the body and `[Test copy]` prefixed to the subject. Everything else is
  exactly what the audience would get, built from the deployment's real data. Other reports and
  every failure alert are unaffected.
- `TRYOUT_TASK=<task name>` runs that one task's handler once, right now, and exits. The schedule
  is ignored, the ledger is not touched (the run is neither recorded nor retried), and the engine
  never ticks. It refuses to start unless that task has a `debug` list: a tryout is for checking,
  and checking must never reach the real audience. The task names are the ones in the table above.

The intended routine in Choreo: add the task's `debug` list, set `TRYOUT_TASK`, run the scheduled
task once by hand, confirm the mail in the debug inbox, then remove both. Leaving a `debug` list in
place silently keeps that report away from its audience, which is why startup names every report
on trial in a warning.

### Alerting

- `ALERT_RECIPIENTS` is a standing audience emailed on every failed attempt of every task.
- A task's own `to`/`cc` in `SUB_CRON_RECIPIENTS` is added to the failure alert for that task (its
  `to` is merged into the same To line). For the two report tasks, that `to`/`cc` instead is the
  actual report audience.
- A task's failures send no email only when both lists are empty. The failure is still recorded
  and retried.
- `ALERTS_ENABLED=false` is a global kill switch for every email the component sends, including
  reports; the report tasks then skip their work entirely for that tick.

### Code structure

```text
csm-scheduled-tasks/
├── cmd/server/main.go            # Entry point: reads config, builds the task list, runs one Engine.Tick, exits
├── internal/
│   ├── schedule/                 # PeriodKey(cronExpr, now): most recent scheduled firing
│   ├── registry/                 # Task{Name, Schedule, Handler, RetryBackoff, To, Cc}
│   ├── engine/                   # Tick: claim -> run -> report back, once per task per invocation
│   ├── ledger/                   # entity-service client for run state (Attempt/Complete/Fail/DeleteResolvedBefore)
│   ├── entitycases/              # entity-service client for case search (used by report tasks)
│   ├── announcementpublish/      # Client + handler for publish_scheduled_announcements
│   ├── cloudstatus/              # Client, outbound webhook poster and handler for cloud_status_webhooks
│   ├── availability/             # Client + handler for availability_recalculation
│   ├── housekeeping/             # Handler for housekeeping_cleanup
│   ├── stalecases/               # Handler for stale_cases_report
│   ├── opencases/                # Handler for open_cases_report
│   ├── migrationscan/            # Handler for migration_case_report
│   ├── migrationreport/          # Client + types for entity-service's migration case scan
│   ├── worklogreport/            # Client, wire types and audience resolver for the CS Worklogs reports
│   ├── worklogsreport/           # Handlers for cs_worklogs_daily_report and cs_worklogs_weekly_report
│   ├── acp/                      # Handler for acp_closure, and the ACP rules and clients (closure, sweep, recipients, notify, entity, salesentity, ...)
│   ├── notify/                   # Email client and HTML templates (alert, stale/open case reports, CS Worklogs, migration case scan)
│   ├── httpsec/                  # Shared https-only URL guard and redirect guard for the HTTP clients
│   └── apierror/                 # Typed upstream-error wrapper shared by the clients
├── .env.example                  # Configuration template
└── go.mod
```

There is no `.choreo/component.yaml`: the component is created as a Scheduled Task directly in the
hosting console, where its cron trigger is configured, and it declares no endpoints.

**Single `package main` rule.** `cmd/server` must remain the only `package main` in this module. The
Google Go buildpack used for deployment picks the package to build by listing `main` packages; with
two it cannot choose and fails with `no Go files in /workspace`, a message that names neither
offending directory.

### Entity-service dependency

All claim, retry, success and failure state lives in entity-service's `scheduled_task_run` table
and its `/scheduled-tasks/attempts` endpoints. Deploy an entity-service version that includes them
before this component's first real invocation.

## Developer guide

### Prerequisites

- Go 1.26.6 or newer (per `go.mod`).
- An entity-service instance this component can reach over HTTPS (plain `http` is accepted only
  for `localhost`/loopback addresses), and an OAuth2 client-credentials client that entity-service
  accepts.
- Optional: an email service endpoint, needed only if alert or report recipients are configured.

### Build, run, test

```bash
# from operations/csm-scheduled-tasks
cp .env.example .env      # then fill in the required values (see Configuration)
go run ./cmd/server       # runs exactly one tick against the configured entity-service, then exits

go build -o server ./cmd/server
go vet ./...
go test -race ./...
```

`.env` in the working directory is loaded automatically at startup if present; variables already
set in the real environment take precedence. `.env` is git-ignored; never commit it. Logs are JSON
on stdout.

Because a run is a single tick, re-run the command to simulate further ticks. A task is only
claimed if its period key is new or its retry time has arrived, so a second immediate run usually
does nothing for tasks that already succeeded. For quick local testing of a handler, a task
schedule can be overridden through `SUB_CRON_SCHEDULES`.

### Adding a new task

1. **Write the handler** in its own package under `internal/`: a
   `func(ctx context.Context) error`. Return `nil` on success and a descriptive error on failure.
   It must be idempotent per period and needs no knowledge of retries, periods or the ledger. If it
   needs a new entity-service endpoint, add a small dedicated client in the same package (follow
   `announcementpublish` or `entitycases`) and build it with `httpsec`-guarded https URLs.
2. **Register it** in the `tasks` slice in `cmd/server/main.go` as a `registry.Task`:
   - `Name`: a stable ledger key.
   - `Schedule`: `scheduleFor(scheduleOverrides, "<name>", "<default cron>")`, so the default lives
     in code and can be overridden by `SUB_CRON_SCHEDULES`.
   - `Handler`: your handler.
   - `To`/`Cc`: from `recipientsFor(recipientOverrides, "<name>")`. Do not hardcode addresses.
   - Optionally `RetryBackoff`; it defaults to `DRIVER_INTERVAL` when zero.
3. **Gate risky tasks** behind an explicit `*_ENABLED` flag (default false) and register them only
   when enabled, as `cloud_status_webhooks` and `availability_recalculation` do, so a switched-off
   task leaves no ledger rows.
4. **Report-style tasks** own their email template (`internal/notify/templates/`) and recipients
   inside the handler; the engine never sends success emails.
5. **Add tests** next to the handler with a fake client (handlers depend on small interfaces for
   this reason), and add any new variable to `.env.example` and the table below.

The schedule of every registered task is validated at startup; an invalid cron expression stops the
component from starting.

## Configuration

Variables are read from the process environment, with `.env` in the working directory used as a
fallback. Booleans use Go `strconv.ParseBool` syntax (`true`/`false`/`1`/`0`); a malformed value
falls back to the default with a warning. JSON-valued variables that fail to parse are logged and
treated as unset, except where noted. All example values are placeholders.

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `OAUTH2_TOKEN_URL` | Yes | none | OAuth2 token endpoint for the client-credentials grant. One shared credential set backs the entity-service and email clients. Startup fails if unset. | `https://auth.example.com/oauth2/token` |
| `OAUTH2_CLIENT_ID` | Yes | none | OAuth2 client id. Startup fails if unset. | `<client-id>` |
| `OAUTH2_CLIENT_SECRET` | Yes | none | OAuth2 client secret. Startup fails if unset. | `<client-secret>` |
| `CUSTOMER_ENTITY_SERVICE_BASE_URL` | Yes | none | Base URL of entity-service. Must be `https` (loopback hosts may use `http`). | `https://entity.example.com` |
| `CUSTOMER_ENTITY_SERVICE_SCOPES` | No | empty | Comma-separated OAuth2 scopes requested for every entity-service client. | `scope-a,scope-b` |
| `EMAIL_BASE_URL` | Conditional | empty | Email service base URL (`/send-email` is appended). Required at startup when `ALERTS_ENABLED` is true and `ALERT_RECIPIENTS` or any task's `to` list is non-empty. Must be `https` if set. | `https://email.example.com` |
| `EMAIL_SCOPES` | No | empty | Comma-separated OAuth2 scopes for the email client. | `scope-a` |
| `EMAIL_FROM_ADDRESS` | No | empty | Fixed From address on every email sent. | `noreply@example.com` |
| `WORKLOG_RECIPIENTS_SOURCE` | No | `configured` | Where the CS Worklogs audience comes from. `configured` uses `SUB_CRON_RECIPIENTS`; `derived` uses the audience entity-service builds from the team registry (team leads and each team's `group_email`). Switching changes who receives the report, so it is deliberate — the drift line logged on every send shows how much it would change first. An unrecognised value stops startup. | `derived` |
| `CSM_PORTAL_WEB_BASE_URL` | No | empty | CSM portal base URL each issue key in a CS Worklogs report links to (`<base>/cases/<id>`), matching csm-notification-service. The reports go to engineers, so this is the CSM portal, not the customer one. Empty renders the keys as plain text. Must be https when set; an invalid value stops startup. | `https://csm.example.com` |
| `ALERTS_ENABLED` | No | `true` | Global kill switch for all emails (failure alerts and report emails). When false, failures are still recorded and retried, and report tasks skip their work. | `false` |
| `ALERT_RECIPIENTS` | No | empty | Comma-separated standing audience emailed on every failed attempt of every task. | `oncall@example.com` |
| `TRYOUT_TASK` | No | empty | Runs the named task once, immediately, outside its schedule and the ledger, then exits. Refuses to start unless that task has a `debug` list in `SUB_CRON_RECIPIENTS`. See "Debug mail mode and tryouts". | `migration_case_report` |
| `SUB_CRON_RECIPIENTS` | No | empty | JSON object of per-task recipients keyed by task name: `{"<task>": {"to": [...], "cc": [...], "debug": [...]}}`. Extra failure-alert audience for most tasks; the actual report audience for `stale_cases_report` and `open_cases_report`. `debug`, on a report task, puts that one report in debug mail mode (see "Debug mail mode and tryouts"). | `{"stale_cases_report":{"to":["team@example.com"],"cc":[],"debug":["you@example.com"]}}` |
| `MIGRATION_CASE_REPORT_OLDER_THAN_DAYS` | No | `180` | The age, in days, a migration must reach before `migration_case_report` lists it. | `180` |
| `MIGRATION_CASE_REPORT_LINK_BASE_URL` | No | empty | The support portal's case page prefix; the migration case scan links each issue to `<prefix>/<case number>`. Unset leaves the keys as plain text. Must be https. | `https://support.example.com/cases` |
| `SUB_CRON_SCHEDULES` | No | empty | JSON object overriding task schedules by name: `{"<task>": "<cron>"}`. Unlisted tasks keep their built-in default. An invalid cron expression stops startup. | `{"housekeeping_cleanup":"0 4 * * *"}` |
| `DRIVER_INTERVAL` | No | `1h` | Expected invocation cadence; must match the platform trigger. Go duration syntax, must be positive. Used as the default retry backoff and to size the orphaned-claim safety margin. | `15m` |
| `HOUSEKEEPING_RETENTION_DAYS` | No | `30` | Days of resolved ledger history kept by `housekeeping_cleanup`. Plain positive integer, not a duration string. | `30` |
| `AVAILABILITY_RECALC_ENABLED` | No | `false` | Registers `availability_recalculation`. Enable only once no other process writes the same availability data, otherwise rows can be duplicated. | `true` |
| `USAGE_ROLLUP_ENABLED` | No | `false` | Registers `usage_rollup_daily` and `usage_rollup_monthly`. Enable only once ServiceNow's usage aggregation jobs and csm-sync-service's mirroring of the same tables are off, otherwise the rows are fought over. | `true` |
| `USAGE_ROLLUP_DAILY_LOOKBACK_DAYS` | No | `2` | How many days before today the daily rollup rebuilds each night. | `3` |
| `ACP_ENABLED` | No | `false` | Registers `acp_closure`. No other `ACP_*` setting is read while false. This component's client id must be in entity-service's `M2M_CLIENT_IDS`. | `true` |
| `ACP_DRY_RUN` | No | `true` | Decide and log, write nothing to CSM or Salesforce. A malformed value is true. It does not stop emails: with `ACP_EMAIL_SEND_ENABLED=true` the notices are sent, and since nothing is recorded they are sent again on every run. A `TRYOUT_TASK=acp_closure` without `ACP_TEST_PROJECT_ID` is always a dry run. | `false` |
| `ACP_EMAIL_SEND_ENABLED` | No | `false` | Sends the notices for real (also needs `ALERTS_ENABLED` and `EMAIL_BASE_URL`); false logs them. A `debug` list for `acp_closure` in `SUB_CRON_RECIPIENTS` redirects every notice. | `true` |
| `ACP_ALLOW_NON_WSO2_RECIPIENTS` | No | `false` | Lets notices reach addresses outside `@wso2.com`. Production only. | `true` |
| `ACP_STANDING_CC_RECIPIENTS` | No | empty | Comma-separated distribution lists added to every notice. Leave empty outside production. | `lifecycle@example.com,billing@example.com` |
| `ACP_TEST_PROJECT_ID` | No | empty | Scopes a run to one CSM project id. | `<csm-project-id>` |
| `ACP_EXCLUDED_PROJECT_IDS` | No | empty | Comma-separated CSM project ids a run skips. | `<id>,<id>` |
| `ACP_SALES_ENTITY_BASE_URL` / `ACP_SALES_ENTITY_TOKEN_URL` / `ACP_SALES_ENTITY_CLIENT_ID` / `ACP_SALES_ENTITY_CLIENT_SECRET` | Yes, when `ACP_ENABLED` is true | none | sales-entity-service (Salesforce) and its own client credentials. Must be `https`. | `https://sales-entity.example.com` |
| `ACP_SALES_ENTITY_SCOPES` | No | empty | Space-separated scopes for the sales-entity-service client. | `scope-a scope-b` |
| `ACP_REOPEN_CHECK_ENABLED` | No | `false` | Registers `acp_reopen_check`. Needs entity-service with `CSM_ACP_REOPEN_ENABLED=true`; whether anything is written is entity-service's `CSM_ACP_REOPEN_DRY_RUN`. | `true` |
| `CLOUD_STATUS_ENABLED` | No | `false` | Registers `cloud_status_webhooks`. Enable only once no other system posts the same status events, otherwise each event is published twice. | `true` |
| `CLOUD_STATUS_WEBHOOK_URLS` | Yes, when `CLOUD_STATUS_ENABLED` is true | none | JSON object mapping a cloud slug to the status dashboard's base URL (`/api/v1/webhook` is appended). Empty or unparseable stops startup. | `{"cloud-a":"https://status-a.example.com"}` |
| `CLOUD_STATUS_WEBHOOK_SECRETS` | Yes, when `CLOUD_STATUS_ENABLED` is true | none | JSON object of the full `X-Webhook-Signature` header value, per cloud slug, with a `default` key as fallback. The value is the whole header, including any scheme prefix the receiver expects. Empty or unparseable stops startup. | `{"default":"Secret <token>"}` |

Notes:

- Not configurable by environment: the stale-cases age threshold (fixed at 30 days) and the
  per-request HTTP timeouts.
- The component sets no execution timeout of its own; the platform's scheduled-task time limit
  bounds a run. On `SIGTERM` it cancels in-flight requests and exits.
- Obsolete: `SUB_CRON_RECIPIENTS` entries for the former outage-email tasks are ignored; those
  emails are now sent by other services.
