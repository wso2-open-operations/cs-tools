# Alert Core Service

Alert Core Service deduplicates incoming alerts into incidents and forwards them to
CSM via `csm-integration-service`, falling back to Google Chat when CSM doesn't
confirm. It reads alerts written by the separate `alert-ingestion` service from
PostgreSQL, folds them into incidents by fingerprint
(`source|service|metric_name|environment|unique_identifier`), and keeps retrying
notifications independently until they're actually delivered.

## Functionality

- **Alert discovery.** Every replica claims unprocessed alert rows with
  `SELECT ... FOR UPDATE SKIP LOCKED`, so no CDC, queue, shared cursor or leader
  election is needed. A claim seeds from the oldest ids and also pulls in other
  unclaimed alerts with the same fingerprint (the `alerts.fingerprint` column written
  by `alert-ingestion`), so one incident's alerts usually fold together on one
  replica. `alert-ingestion` posts to `POST /alertz` after each written batch to wake
  the poller; `poll.interval` is only the backstop.
- **Continuous pipeline.** Claimed alerts are grouped by fingerprint and sent to
  `poll.concurrency` fingerprint-sharded workers. The claimer tops the pipeline up to
  2 x `poll.max_batch` alerts as soon as space frees, so one slow fingerprint never
  idles the others. Finished alerts are marked processed (or released for retry) in
  batched statements every 20ms.
- **Deduplication by fingerprint.** Every alert is normalized (severity label,
  category, defaults from `CORE_ALERT_DEFAULTS`) and folded by
  `source|service|metric_name|environment|unique_identifier`. An alert inside
  `engine.dedup_window` (default 5m) of the incident's first alert, measured on each
  alert's `alerts.created_at`, becomes a Duplicate note; the first alert at or after
  that boundary, or after CSM closed the incident, opens a new incident. A resolving
  alert (`OK`/`Clear`) becomes an OK note and never creates an incident.
- **One transaction per fingerprint group.** A worker folds its whole group under a
  transaction-scoped advisory lock on the fingerprint: read the current incident and
  which alert ids are already recorded, decide in memory, write the notes and
  counters, commit. That is two pipelined round trips however many alerts the group
  holds, and replays after a crash are no-ops because `incident_notes.alert_id` is
  unique.
- **Incidents and notes.** `incidents_processed` has one row per incident, so an incident that
  is replaced before it was ever delivered still gets delivered. `incident_notes` is
  an append-only log of every alert folded into an incident, with flags for what is
  still owed to CSM (`csm_pending`) and Chat (`chat_pending`).
- **Delivery off the alert path.** Folding only touches Postgres. Folds mark the
  incident due (`incidents_processed.delivery_due_at`), and a separate delivery loop, run after
  every acknowledgement flush and at least every `notify.retry_sweep_interval`,
  delivers due incidents with `notify.delivery_concurrency` workers per replica. A
  per-incident advisory try-lock keeps workers and replicas from delivering the same
  incident twice. Each pass creates the incident in CSM if due (with exponential
  backoff: `csm_retry_base_delay`, `csm_retry_multiplier`, `csm_retry_max_delay`,
  up to `max_csm_attempts`), pushes owed work notes in order, posts the Chat
  fallback, and schedules its next due time.
- **Assignment group and contact type.** The create never sends an assignment group:
  entity-service assigns every incident to its service's support group, and rejects a
  create that names one (a permanent 400 here). The routing chain below still runs and
  is logged, but its answer is not sent until it is decided how it fits that rule. The
  group it would pick is the most specific signal the first alert carries, in this order:
  1. the group the alert names for itself (an AWS alarm's `AlarmDescription`
     `"assignment_group"`), as a group id or as a name mapped in
     `CSM_ASSIGNMENT_GROUP_ROUTES` (`"group:<name>"`);
  2. the matched CMDB service's support group;
  3. the topic it was sent from, an AWS SNS `TopicArn` (`"topic:<arn>"`);
  4. the account it was sent from, an AWS account id (`"account:<id>"`);
  5. `CSM_DEFAULT_ASSIGNMENT_GROUP_ID`.

  The log line `assignment group resolved, not sent` names the step that decided
  (`by=`). The contact type is sent when the alert's source has one in CSM's enum
  (Azure → `AZURE`, Site24x7 → `SITE_247`, Sentinel → `SENTINEL`); AWS and the rest
  have none, so their incidents reach the SRE ladder through the service's support group.
- **Duplicate-create protection.** Before creating an incident, and again before
  every retry, the service searches CSM by `correlationId` so a lost create response
  never causes a duplicate.
- **Chat fallback.** If CSM still hasn't confirmed an incident
  `notify.chat_fallback_delay` (25s) after it was created, it is posted once to
  `FALLBACK_CHAT_WEBHOOK_URLS`; CSM keeps retrying meanwhile. Without CSM configured,
  or after CSM permanently rejects the incident, the card is posted at once. With `chat_threading_enabled`, each incident gets its
  own thread, and Duplicate/OK alerts are collapsed into one digest reply per
  delivery pass ("12 duplicate alerts received.") instead of one Chat message per
  alert, which matters because Chat webhooks allow about one message per second.
- **CSM status sync.** Open/closed state is read back from CSM, never inferred, and
  only for incidents that are still inside their window and have received alerts
  since the last check, at most once per `state_check_interval`.
- **Asynchronous commit.** Core connections run with `synchronous_commit=off`.
  Every core write is replayed after a crash (claims expire and folds are
  idempotent), so a lost tail of commits costs only reprocessing, never an alert.
- **Retention.** One replica at a time deletes processed alerts older than
  `retention.alerts` and incidents with nothing left to deliver older than
  `retention.incidents`, in chunks.
- **Health and liveness endpoints.** `/healthz` checks PostgreSQL connectivity;
  `/livez` doesn't, so a transient DB blip triggers a readiness dip rather than a
  pod restart. `/dbz` reports only database reachability (`200` or `503`) for monitoring; it
  reuses one ping per second, so frequent checks hold at most one connection.

## Multi-container support

Every replica runs its own poller with no leader election. `SKIP LOCKED` guarantees
two replicas never claim the same alert row, and a crashed replica's claims become
reclaimable after `poll.claim_ttl`. If two replicas do hold alerts for the same
fingerprint, the transaction-scoped fold lock serializes them, so the outcome is
the same as one replica processing both. Delivery is serialized per incident by a
session advisory try-lock (`internal/pglock`), and CSM's dedup-by-tag search covers
the remaining gap where a replica crashes between a CSM create succeeding and its
confirmation being stored.

## Package layout

- `schema.sql` / `schema.go`: the database schema, embedded and applied at startup.
- `internal/poll`: the claim, fold and acknowledge pipeline, and the delivery loop.
- `internal/engine`: the fold decision (a pure function over the current incident
  and the group) and the per-incident delivery pass.
- `internal/store`: pgx repositories for the alerts claim queue, incidents and
  notes, and the retention job.
- `internal/pglock`: session advisory try-locks for delivery and retention.
- `internal/csm`: OAuth2 client for `csm-integration-service` (create incident,
  update work notes, search by correlation id, resolve service id).
- `internal/notify`: wraps the CSM client and Chat webhooks with retry/backoff.
- `internal/model`: the `Alert`/`Incident`/`Note` shapes, severity/fingerprint
  normalization, and HTML note formatting.
- `internal/hub`: the `/alertz` HTTP handler that wakes the poller early.
- `internal/postgres`: connection setup (`pgxpool`) and schema migration.
- `internal/config`: loads and validates `config.toml`.
- `internal/auth`: PBKDF2 hashing/verification and the `integration_users`
  repository (used by `cmd/user` and by sre-alert-ingestion-service's webhook auth),
  plus `RequireWakeToken`, which guards `/alertz` with the shared
  `ALERT_CORE_WAKE_TOKEN`.
- `cmd/server`: wires everything together and manages startup/shutdown.
- `cmd/user`: CLI to create/rotate, list, enable, and disable `integration_users` rows.

## Running it

```bash
go run ./cmd/server
go build ./... && go vet ./... && go test ./...
```

Postgres integration tests (store queries and an end-to-end load test) create and
drop a throwaway database using the `PG*` environment; `LOAD_ALERTS` sizes the load
test (default 5000):

```bash
set -a && source .env && set +a
go test -tags integration ./internal/store ./internal/poll
```

Only `PGHOST`, `PGDATABASE`, `PGUSER` and `PGPASSWORD` are required. The service
applies `schema.sql` itself on every startup (idempotent, under an advisory lock), so
no manual migration step is needed. `schema.sql` only creates what is missing (every
statement is `CREATE ... IF NOT EXISTS`), so a change to an existing table needs its own
migration.

CSM delivery turns on only when all of `CSM_INTEGRATION_BASE_URL`,
`CSM_INTEGRATION_TOKEN_URL`, `CSM_INTEGRATION_CLIENT_ID`,
`CSM_INTEGRATION_CLIENT_SECRET`, `CSM_CALLER_ID` and `CSM_UNKNOWN_SERVICE_ID` are set.
Without them incidents are still created and deduplicated in Postgres and surfaced via
`FALLBACK_CHAT_WEBHOOK_URLS` (if set). See `.env.example`.

`config.toml` is optional and gitignored. Every tunable has a built-in default
(`internal/config.defaults`); a present file overrides field by field, and every value
is validated at startup. `config.toml.example` documents the available fields.

```bash
cp config.toml.example config.toml
```

Internal API users (`integration_users`) are documented separately in
[`PROVISION.md`](PROVISION.md).

## Choreo Deployment

`config.toml` is not baked into the image because it's supplied at runtime via
Choreo's **Manage > Configs and Secrets > File Mount**:

1. In the component's Choreo console, go to **Manage Configs and Secrets >
   File Mount** and add a new file mount.
2. Set the file **name** to `config.toml` and paste the contents of
   `config.toml.example` (customized as needed) as the file content.
3. Mount it at the path the service reads from: the working directory root
   (so it resolves as `config.toml`), or any path if you also set the
   `CONFIG_PATH` environment variable to that path.
4. Redeploy. The poller, PostgreSQL, notify, engine, retention and server
   tunables all load from this mounted file on startup.
