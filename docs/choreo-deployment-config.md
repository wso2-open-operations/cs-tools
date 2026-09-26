# Choreo deployment configuration

Every configuration value the ServiceNow-replacement work needs, per Choreo component, and what
breaks if it is missing.

**Keep this current.** Add a row here in the same change that introduces the config it describes.
A value that only exists in a Go file and someone's memory is one redeploy away from an outage
nobody can diagnose — and these components fail *quietly* when misconfigured: a missing
`EVENT_HUB_TOPIC` publishes nothing, an unset `SUB_CRON_RECIPIENTS` sends no report, and both look
exactly like "no activity this week".

Last updated: 2026-09-24.

---

## Conventions that apply everywhere

- **Config is flat single-line strings.** Choreo will not hold multi-line values, which is why the
  per-task maps below are single-line JSON rather than YAML blocks.
- **`TZ=UTC` on every component.** Cron schedules are interpreted in the container's local zone
  (`internal/schedule.PeriodKey`). A non-UTC container shifts every schedule silently. This matters
  most for the weekly report — see its row below.
- **Secrets are Choreo secrets, never config values**, and never committed. The `OAUTH2_*` triples
  below are secrets.
- **One shared OAuth2 app per component**, scoped per-consumer via each client's own `*_SCOPES`.

---

## `entity-service`

The publisher and the system of record. Nothing below is new for the query-hour work except where
noted.

| Variable | Required | Purpose / failure mode if unset |
|---|---|---|
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` / `DB_SSLMODE` | Yes (Postgres) | All-or-nothing: `Config.Validate` rejects a partial set so a typo cannot silently disable the Postgres-only endpoints |
| `DATA_SOURCE` | Yes | `postgres` or `servicenow`. With `servicenow` and no DB, the query-hour endpoints are not registered and 404 |
| `SERVER_PORT` | No (8080) | HTTP listen port |
| `EVENT_HUB_BROKER` | No | Kafka bootstrap address. Feature-gates the publisher |
| `EVENT_HUB_CONNECTION_STRING` | With broker | Namespace SAS connection string |
| `EVENT_HUB_TOPIC` | With broker | Topic name. **All three must be set together** — `Config.Validate` enforces it, because a broker without a topic makes every publish fail silently while the deployment looks healthy |
| `AUTH_INTERNAL_CLIENT_IDS` | Yes, in practice | Comma-separated OAuth2 client ids treated as internal services. **The csm-scheduled-tasks client id must be listed here**, or `POST /query-hours/sweep` and `GET /query-hours/weekly-report` return 403 and both the hourly recompute and the weekly report fail every run — the report task reports the failure, but no report is ever sent |
| `EVENT_PUBLISHING_ENABLED` | No (`false`) | Must be exactly `"true"`. A second, independent kill switch on top of the broker config |
| `SUPPORT_ENGINEER_ROLE` | No | ServiceNow role that completes a case's response SLA clock |
| `CUSTOMER_ROLES` | No | Comma-separated roles that mark a customer reply |

**Query-hour specific:** none. The weekly report and the hourly recompute add no entity-service
configuration — they are endpoints on the existing deployment, and the Salesforce tables they read
are populated by csm-sync-service in digiops-cs, not by anything configured here.

---

## `csm-scheduled-tasks`

Runs one tick per Choreo invocation and fans out to sub-crons. **Set the Choreo trigger cadence and
`DRIVER_INTERVAL` to the same value** — the driver interval is also the default retry backoff, and
a mismatch makes retry timing wrong in a way nothing reports.

| Variable | Required | Purpose / failure mode if unset |
|---|---|---|
| `OAUTH2_CLIENT_ID` / `OAUTH2_CLIENT_SECRET` / `OAUTH2_TOKEN_URL` | Yes | Shared credentials for every client below |
| `CUSTOMER_ENTITY_SERVICE_BASE_URL` | Yes | entity-service base URL. Must be https (loopback exempt) |
| `CUSTOMER_ENTITY_SERVICE_SCOPES` | No | Comma-separated scopes for the entity-service clients |
| `EMAIL_BASE_URL` | Conditionally | Required at startup when alerts are on and any recipient is configured. Checked at boot so a misconfigured deployment fails loudly instead of at 18:30 on a Sunday |
| `EMAIL_SCOPES` | No | Scopes for the email client |
| `EMAIL_FROM_ADDRESS` | No | Fixed From address |
| `ALERTS_ENABLED` | No (`true`) | Global kill switch for **every** email, failure alerts and reports alike |
| `ALERT_RECIPIENTS` | No | Standing audience alerted on every failed attempt of every task |
| `DRIVER_INTERVAL` | No (`1h`) | Must match the Choreo trigger cadence |
| `SUB_CRON_SCHEDULES` | No | `{"<task>": "<cron>"}` — overrides any task's schedule by exact name |
| `SUB_CRON_RECIPIENTS` | No | `{"<task>": {"to": [...], "cc": [...]}}` — see the per-task table |
| `HOUSEKEEPING_RETENTION_DAYS` | No (`30`) | Days of resolved ledger history kept |
| `CSM_PORTAL_WEB_BASE_URL` | No | Portal linked from the allocation reminder |
| `SALESFORCE_BASE_URL` | No | **New.** Salesforce instance the weekly query-hour report links to, e.g. `https://wso2.lightning.force.com`. Unset renders account/opportunity/project cells as plain text — still a complete report, just without links |

### Registered sub-crons

`SUB_CRON_RECIPIENTS` means two different things depending on the task, and getting it backwards is
the easiest mistake here:

| Task name | Default schedule | What `SUB_CRON_RECIPIENTS` means | Unset means |
|---|---|---|---|
| `housekeeping_cleanup` | `0 3 * * *` | Failure alerts only | Silent failures (beyond `ALERT_RECIPIENTS`) |
| `stale_cases_report` | `0 7 * * *` | **The report's actual audience** | No report sent, and the query is skipped |
| `open_cases_report` | `0 8 * * *` | **The report's actual audience** | No report sent, and the query is skipped |
| `allocation_status_update_reminder` | `0 0 * * 1` | Failure alerts only — its real audience comes from the data | Reminders still go out; nobody is told if it fails |
| `query_hour_recompute` | `0 * * * *` | Failure alerts only | Recompute still runs; nobody is told if it fails |
| `query_hours_weekly_report` | `30 18 * * 0` | **The report's actual audience** | No report sent, and the query is skipped |

Both `query_hour_recompute` and `query_hours_weekly_report` call **internal-only** entity-service
endpoints. Their OAuth2 client id must appear in entity-service's `AUTH_INTERNAL_CLIENT_IDS` or
every run fails with a 403 — see that variable's row above. This is the single most likely reason
for a deployment where the tasks run, the ledger records failures, and nobody receives anything.

**`query_hours_weekly_report`'s schedule is Sunday 18:30 UTC and that is not a typo for Monday.**
ServiceNow fires it 00:00:05 Monday in `Asia/Colombo`, which *is* 18:30 UTC Sunday. `0 0 * * 1`
would look like the obvious translation and would move the mail 5½ hours later, and change its date
stamp — the stamp is a UTC date, which is why the Monday mail is headed with Sunday's.

---

## `csm-notification-service`

Consumes events and sends. Relevant to query hours because it owns the per-project **threshold**
email — a different email from the weekly report, on a different rule. See
`operations/csm-scheduled-tasks/CLAUDE.md` for why they must stay separate.

| Variable | Required | Purpose |
|---|---|---|
| `EVENT_HUB_BROKER` / `EVENT_HUB_CONNECTION_STRING` | Yes | Bus connection |
| `EVENT_HUB_TOPIC` | Yes | Topic to consume |
| `EVENT_HUB_CONSUMER_GROUP` | Yes | This consumer's group |
| `CR_EVENT_HUB_TOPIC` | Yes (dev has it) | The change-request topic is managed separately on purpose, not only for testing |
| `OAUTH2_*` | Yes | Shared credentials |
| `EMAIL_BASE_URL` / `EMAIL_FROM_ADDRESS` | Yes | Email delivery |
| `DEFAULT_CHAT_PRODUCT` | No | Fallback Google Chat space when a case has no deployed product |
| `INCIDENT_DEFAULT_CALL_TO` | No | Fallback on-call number |

---

## `csm-flow-service`

| Variable | Required | Purpose |
|---|---|---|
| `EVENT_HUB_BROKER` / `EVENT_HUB_CONNECTION_STRING` / `EVENT_HUB_TOPIC` | Yes | `mustEnv` — the process will not start without them |
| `EVENT_HUB_CONSUMER_GROUP` | Yes | Its own group, separate from csm-notification-service's |
| `CUSTOMER_ENTITY_SERVICE_BASE_URL` | No | Optional; fails on first use rather than at boot |
| DLQ settings | No | Optional; fails on first use |

Correlation prefix is `cfs-`.

---

## Azure Event Hub

A **topic** isolates volume; a **consumer group** isolates processing. That distinction decides
every entry here — the house rule from commit `ef22c6575`: *"A separate consumer group isolates
processing; only a separate topic isolates volume."*

| Needed | Why |
|---|---|
| `cs-events` topic | The main entity-service publisher stream |
| `cr-events` topic | Change requests, managed separately — a deliberate decision, not a test artefact |
| One consumer group per consuming component | csm-notification-service and csm-flow-service must not share one, or they steal each other's partitions |

**The query-hour work needs no new topic and no new consumer group.** The threshold event is a new
event *type* routed inside the existing dispatcher's switch, and the weekly report never touches the
bus at all — it is an HTTP read plus an email.

---

## Deployment order

1. **digiops-cs `csm-sync-service` first.** Migrations 0080/0081 create the Salesforce mirror tables
   (`sf_opportunity`, `sf_opportunity_product`, `sf_opportunity_link`, `sf_invoice`,
   `account_relationship`) and the mapping jobs populate them. Until they hold data the weekly
   report is correct but empty, and the hourly recompute falls back to ServiceNow's synced figure.
2. **entity-service.** Apply its own migrations, then deploy.
3. **csm-scheduled-tasks**, with the sub-cron config above.
4. **csm-notification-service**, if the threshold email is going live in the same window.

---

## Before turning anything on

- **The double-fire rule.** Registering a port is a paired change with deactivating its ServiceNow
  counterpart in the same change. Two systems sending the same email is worse than neither.
- **`query_hours_weekly_report` is NOT yet a paired deactivation.** The ServiceNow flow it ports
  also *writes* `sf_opportunity.query_hour_state` and caches rendered HTML onto the account; only
  the read half is ported. Turning that flow off stops those writes too, and nothing has yet
  established what still reads that column. Until it is settled, both systems send — so keep this
  task's recipient list narrow.
- **The consume path has never been exercised end to end.** Every email sent during development
  went straight to the email service, bypassing the bus. Before cutover, publish to a throwaway
  topic and confirm the consumer reacts.
