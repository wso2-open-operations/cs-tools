# Alert Ingestion Service

Alert Ingestion Service receives monitoring webhooks from ten sources (AWS, Azure, Datadog,
Elasticsearch, GCP, Icinga, OpenObserve, OpenSearch, Prometheus, Site24x7), normalizes each alert
to a common shape, gives it a unique `ALT#########` id, and writes it to the `alerts` table in
PostgreSQL. The separate `alert-core-service` claims those rows and turns them into incidents; this
service never creates incidents and never touches alert-core's own tables.

## Functionality

- **Per-source transforms.** Each source has its own mapping under `internal/sources/<source>/`,
  following the ServiceNow Edge API mappings for that source. A payload a transform rejects is
  answered `400` and never claims an id. A temporary `servicenow` route accepts already-canonical
  alerts during the parallel run with alert-core's legacy integration.
- **Unique id allocation.** Each batch draws its ids from `alert_seq`, a native Postgres sequence,
  in one round trip and uses exactly the values returned. Ids are unique across replicas but not
  consecutive, since replicas draw from the same sequence concurrently; alert-core claims rows with
  `SKIP LOCKED` and never waits on gaps. A batch covers everything queued when a writer frees up,
  up to `allocator.max_batch`.
- **Batched, idempotent writes.** Every batch is written in one `INSERT ... SELECT FROM unnest(...)
  ON CONFLICT (id) DO NOTHING` statement, so retrying a batch after a lost commit acknowledgement
  is a no-op. A failed batch is retried with exponential backoff until `store.insert_attempts` or
  `store.write_deadline`; if it still fails its senders get `503` and resend. `created_at` comes
  from the database clock, and each row carries the alert's `fingerprint` so alert-core can claim
  one incident's alerts together.
- **AWS SNS subscription handling.** When an SNS topic subscribes the AWS webhook URL, the service
  verifies the message's SNS signature, confirms the subscription (fetching its `SubscribeURL`, only
  from `sns.<region>.amazonaws.com`) and answers `200` without storing an alert. An
  `UnsubscribeConfirmation` is logged and answered `200`, never stored; any other SNS message type
  except `Notification` is rejected with `400`.
- **Backpressure.** Accepted-but-unfinished work is capped by both a queue size and a memory
  budget; past either limit, new webhooks get `503` immediately rather than queuing indefinitely.
- **Wake-up call.** After writing a batch, the service posts once to alert-core's `POST /alertz`
  (calls are coalesced so at most one is in flight) so alert-core doesn't have to wait for its own
  poll interval. If the call fails, that poll still picks the rows up.
- **Health and liveness endpoints.** `/healthz` never checks Postgres, so a database outage
  doesn't pull every replica out of rotation; `/livez` always answers `200` while the process runs.
  `/dbz` reports only database reachability (`200` or `503`) for monitoring; it reuses one ping per
  second, so frequent checks hold at most one connection.
- **Routing signals.** Alongside the canonical fields, an alert can carry what alert-core uses to
  choose its CSM assignment group: `assignment_group` (an AWS alarm's `AlarmDescription` JSON may
  name one, e.g. `{"service":"...","assignment_group":"SRE - Apollo"}`) and, for AWS,
  `source_topic` (the SNS `TopicArn`) and `source_account` (the AWS account id).

## Package layout

- `internal/sources`: the per-source transforms (`aws`, `azure`, `datadog`, `elasticsearch`,
  `gcp`, `icinga`, `openobserve`, `opensearch`, `prometheus`, `site24x7`, `servicenow`) and the
  registry that dispatches a webhook to the right one.
- `internal/allocator`: batches submissions, claims ids from `alert_seq`, and writes each batch
  with whole-batch retry.
- `internal/postgres`: connection setup and the `alert_seq`/`alerts` helpers the allocator builds
  on.
- `internal/outbound/corewake`: the coalesced `POST /alertz` call to alert-core.
- `internal/outbound/snsconfirm`: confirms AWS SNS topic subscriptions.
- `internal/outbound/dbfallback`: on a database outage, posts a DATABASE CONNECTION FAILURE card to
  one Google Chat space, then each alert that fails its last write as a reply in that card's
  thread. The next stored batch ends the outage, so a later one opens a new thread.
- `internal/transport/server`: the HTTP handlers for the source webhooks and health/liveness
  endpoints.
- `internal/transport/auth`: optional per-source credential checking against alert-core's
  `integration_users` table.
- `internal/model`: the canonical alert shape shared across transforms and storage.
- `internal/config`: loads and validates `config.toml`.
- `cmd/server`: wires everything together and manages startup/shutdown.

## Running it

```bash
go run ./cmd/server
go build ./... && go vet ./... && go test ./...
```

Requires Go 1.26 and a non-production PostgreSQL database (never the production one) via the
`PG*` environment variables; see `.env.example`. Azure Flexible Server requires
`PGSSLMODE=require` (the default); a local Postgres for development can set
`PGSSLMODE=disable`.

Configuration lives in `config.toml` (queue sizes, batch limits, write/retry timeouts); see
`config.toml.example` for every key with its default. `config.toml` itself is gitignored, since
it's treated as deployment config rather than source.

```bash
cp .env.example .env
```

## Choreo Deployment

1. Create a *Service* component from this repo with build context
   `integrations/sre-alert-ingestion-service` and the Dockerfile build preset.
2. Endpoints come from `.choreo/component.yaml`: the source webhooks under
   `/api/wso2/v1/sre_alert_api` (defined per-source in `openapi.yaml`, so only known source paths
   are accepted), plus `/healthz` and `/livez` as the readiness and liveness probes, and `/dbz`
   for database health.
3. Set the environment variables documented in `.env.example`, marking `PGPASSWORD` as a secret.
4. Mount a customized `config.toml` under **Manage > Configs and Secrets > File Mount** if any
   default needs changing; without it the service runs on `config.toml.example`'s values.
5. Connect this component to `sre-alert-core-service`'s endpoint and point `ALERT_CORE_WAKE_URL`
   at its `/alertz` path. Set `ALERT_CORE_WAKE_TOKEN` (a secret, from `openssl rand -hex 32`) to the
   same value on both components. Both services must share the same `PG*` values.
   Set `DB_FALLBACK_CHAT_WEBHOOK_URL` (a secret) to the webhook of the Chat space that should get
   alerts the database could not store.
6. Any number of replicas is safe: ids stay unique across replicas because every claim pulls from
   `alert_seq`, which can never hand out the same value twice.
