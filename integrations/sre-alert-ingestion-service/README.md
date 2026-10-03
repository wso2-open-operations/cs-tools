# Alert Ingestion Service

Alert Ingestion Service receives monitoring webhooks from ten sources (AWS, Azure, Datadog,
Elasticsearch, GCP, Icinga, OpenObserve, OpenSearch, Prometheus, Site24x7), normalizes each alert
to a common shape, gives it a gap-free `ALT#########` id, and writes it to the `alerts` table in
PostgreSQL. The separate `alert-core-service` reads those rows in id order and turns them into
incidents; this service never creates incidents and never touches alert-core's own tables
(`incidents_*`, `processor_lease`).

## Functionality

- **Per-source transforms.** Each source has its own mapping under `internal/sources/<source>/`,
  following the ServiceNow Edge API mappings for that source. A payload a transform rejects is
  answered `400` and never claims an id. A temporary `servicenow` route accepts already-canonical
  alerts during the parallel run with alert-core's legacy integration.
- **Gap-free id allocation.** Every replica claims ranges of ids from `alert_seq`, a native
  Postgres sequence, with a single `nextval()` call, so concurrent replicas can never collide or
  need a retry loop. A claim covers everything queued at that moment, up to a configurable batch
  size, so a burst costs only a handful of round trips.
- **Batched, resilient writes.** Every alert in a claimed batch is written in one pipelined round
  trip instead of one round trip per alert. A row that fails is retried individually with
  exponential backoff; if it still fails before the write deadline, a filler row is written under
  the same id so alert-core skips it immediately instead of waiting out its gap timeout.
- **AWS SNS subscription handling.** When an SNS topic subscribes the AWS webhook URL, the service
  confirms the subscription (fetching its `SubscribeURL`, only from `sns.<region>.amazonaws.com`)
  and answers `200` without storing an alert.
- **Backpressure.** Accepted-but-unfinished work is capped by both a queue size and a memory
  budget; past either limit, new webhooks get `503` immediately rather than queuing indefinitely.
- **Wake-up call.** After writing a batch, the service posts once to alert-core's `POST /alertz`
  (calls are coalesced so at most one is in flight) so alert-core doesn't have to wait for its own
  poll interval. If the call fails, that poll still picks the rows up.
- **Health and liveness endpoints.** `/healthz` never checks Postgres, so a database outage
  doesn't pull every replica out of rotation; `/livez` always answers `200` while the process runs.

## Package layout

- `internal/sources`: the per-source transforms (`aws`, `azure`, `datadog`, `elasticsearch`,
  `gcp`, `icinga`, `openobserve`, `opensearch`, `prometheus`, `site24x7`, `servicenow`) and the
  registry that dispatches a webhook to the right one.
- `internal/allocator`: claims id ranges from `alert_seq` and batches writes, with per-row retry
  and filler-row fallback.
- `internal/postgres`: connection setup and the `alert_seq`/`alerts` helpers the allocator builds
  on.
- `internal/outbound/corewake`: the coalesced `POST /alertz` call to alert-core.
- `internal/outbound/snsconfirm`: confirms AWS SNS topic subscriptions.
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

Requires Go 1.25.5 and a non-production PostgreSQL database (never the production one) via the
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
   are accepted), plus `/healthz` and `/livez` as the readiness and liveness probes.
3. Set the environment variables documented in `.env.example`, marking `PGPASSWORD` as a secret.
4. Mount a customized `config.toml` under **Manage > Configs and Secrets > File Mount** if any
   default needs changing; without it the service runs on `config.toml.example`'s values.
5. Connect this component to `sre-alert-core-service`'s endpoint and point `ALERT_CORE_WAKE_URL`
   at its `/alertz` path. Both services must share the same `PG*` values.
6. Any number of replicas is safe: ids stay unique across replicas because every claim pulls from
   `alert_seq`, which can never hand out the same value twice.
