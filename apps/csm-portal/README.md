# CSM Portal

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../../LICENSE)
[![GitHub last commit](https://img.shields.io/github/last-commit/wso2-open-operations/cs-tools/main?path=apps%2Fcsm-portal)](https://github.com/wso2-open-operations/cs-tools/commits/main/?path=apps/csm-portal)
[![GitHub issues](https://img.shields.io/github/issues/wso2-open-operations/cs-tools.svg)](https://github.com/wso2-open-operations/cs-tools/issues)

CSM Portal is an open-source solution for customer success management operations, built with a modular architecture that combines a Go backend-for-frontend (BFF) and a React web application. It enables customer success managers to handle cases, projects, accounts, deployments, time tracking, and security operations through a unified platform.

## Why CSM Portal?

Customer success teams need a single workspace to track support cases, customer projects, deployments, product updates, and security vulnerabilities across accounts. Managing these in disparate systems creates disconnected workflows and duplicated effort.

CSM Portal addresses this by combining:

- A centralized Go BFF that validates identity and proxies authenticated requests to upstream services,
- A React SPA for CSM workflows including case management, project tracking, time cards, and more.

This setup lets teams deliver consistent, efficient customer success operations without rebuilding shared infrastructure.

## CSM Portal Features

- **Modular Application Structure**
  Backend and webapp are developed as independent units while remaining part of one cohesive product.
- **Centralized BFF Layer**
  The Go backend validates JWTs, applies security headers, and proxies requests to entity, SCIM, and updates upstream services.
- **Rich Case Management**
  Create, search, patch, and comment on support cases, with attachment uploads and call request support.
- **Project & Deployment Tracking**
  View and manage customer projects, deployments, and deployed products across accounts.
- **Time Cards & Operations**
  Track time spent per case and manage customer success operations including change requests.
- **Security Center**
  Search and view product vulnerability disclosures per account.
- **Product Updates**
  Browse product update levels and upgrade paths between versions.
- **Identity Integration**
  Authentication is integrated with OIDC-compatible identity providers for secure access control.

## Project Structure

```bash
.
├── backend                  # Go BFF REST API service
│   └── README.md            # Detailed backend documentation
├── webapp                   # React + TypeScript SPA for CSMs
│   └── README.md            # Detailed webapp documentation
└── README.md                # You're here
```

CSM Portal is one part of the broader CSM platform. The platform's other components —
`entity-service`, the customer-facing portal (`apps/customer-portal`), and several
supporting integrations — live elsewhere in this repository. See
[Platform architecture](#platform-architecture) below for how they all fit together, and
[Running the full platform locally](#running-the-full-platform-locally) for a
one-command way to bring all of them up together.

## Technologies Used

### Backend

- **Language**: [Go](https://go.dev/) 1.26+
- **Authentication**: JWT validation via JWKS endpoint (OIDC-compatible)
- **Upstream Clients**: OAuth2 client credentials for entity, SCIM, and updates services

### Frontend

- **Framework**: React 19 + TypeScript (Vite)
- **UI**: WSO2 Oxygen UI
- **Data Layer**: TanStack Query
- **Authentication**: OIDC React SDK

## Getting Started

### Prerequisites

- Go 1.26+
- Node.js 20+ (LTS recommended)
- `pnpm` 9+

### Backend Setup Guide

- [Backend](./backend/README.md)

### Frontend Setup Guide

- [Webapp](./webapp/README.md)

The two guides above run just this portal's backend and webapp, against whatever
upstream services (entity-service, etc.) you already have available. If you'd rather
bring up the **entire CSM platform** — both portals, entity-service, Postgres, the event
bus, and every supporting integration — in one shot, see the next two sections instead.

## Platform architecture

The CSM platform is replacing a legacy, vendor ticketing-system-backed stack with a
purpose-built one. Internal CS engineers work customer cases through **this portal** (the
CSM portal); customers interact through a separate, existing **customer portal**
(`apps/customer-portal`). Both are served by their own Go BFF, and both ultimately read
and write the same PostgreSQL-backed **entity service**, which is the platform's system of
record.

```
                          ┌─────────────────────┐        ┌──────────────────────────┐
                          │   CSM webapp (FE)    │        │  Customer webapp (FE)     │
                          └──────────┬───────────┘        └────────────┬─────────────┘
                                     │ REST (x-jwt-assertion)           │ REST (x-jwt-assertion)
                          ┌──────────▼───────────┐        ┌────────────▼─────────────┐
                          │  csm-portal backend   │        │ customer-portal backend-v2│
                          │  (Go BFF, this dir)   │        │ (Go BFF)                  │
                          └──────────┬───────────┘        └────────────┬─────────────┘
                                     │ REST                              │ REST
                                     └──────────────┬────────────────────┘
                                                     ▼
                                          ┌─────────────────────┐
                                          │   entity-service     │  <- system of record
                                          │   (Go, Postgres)      │
                                          └──────────┬───────────┘
                                                     │
                                    ┌────────────────┼─────────────────────┐
                                    ▼                ▼                     ▼
                          ┌──────────────────┐  ┌───────────┐   ┌───────────────────────┐
                          │ csm-integration-  │  │ Postgres  │   │  Kafka (case-events)   │
                          │ service (M2M API) │  │           │   │  (stands in for Azure  │
                          └────────▲──────────┘  └───────────┘   │  Event Hub locally)    │
                                   │                              └────────┬──────┬───────┘
                    ┌──────────────┴───────────┐                          │      │
                    │ sre-alert-ingestion-      │                consumed by      consumed by
                    │ service (external alerts) │                          │      │
                    └───────────────────────────┘             notification- activity-stream
                                                                service     services (x2, one
                                                                            per webapp) -> SSE
```

Auth for every backend is a JWT presented in the `x-jwt-assertion` header, validated
against an OIDC-compatible identity provider (issuer + JWKS). Nothing in this repo names a
specific identity provider — the platform is IdP-agnostic by design.

### Components

| Component | Language | Role |
|---|---|---|
| `apps/csm-portal/webapp` | React | This portal's UI. Calls csm-portal backend only, never entity-service directly. |
| `apps/csm-portal/backend` | Go | This portal's BFF. Validates the caller's JWT, calls entity-service for all case/project/account data. |
| `apps/customer-portal/webapp` | React | Customer-facing portal UI. Calls customer-portal backend-v2 only. |
| `apps/customer-portal/backend-v2` | Go | Customer portal's BFF (the current, Postgres-era backend — **not** the legacy Ballerina `apps/customer-portal/backend`, which is a separate, older stack). |
| `entity-service` | Go | Shared entity service both BFFs talk to. PostgreSQL-backed (`DATA_SOURCE=postgres`) — the platform's system of record. Publishes domain events (`case.created`, `comment.added`, ...) to the event bus. Does not itself validate JWTs; it trusts whatever caller invoked it (the BFFs sit in front of it). |
| `integrations/csm-integration-service` | Go | M2M-only REST API (accounts/projects/contacts/incidents) for machine callers, e.g. `sre-alert-ingestion-service`. No inbound JWT validation — trust is delegated to the API gateway in every real environment. |
| `integrations/csm-notification-service` | Go | Consumes case/incident events from Kafka and turns them into email / Google Chat / voice-call notifications. |
| `integrations/csm-portal-activity-stream-service` | Go | Consumes the same event stream and re-exposes it as a per-case Server-Sent-Events (SSE) feed for this portal's live activity tab. |
| `integrations/customer-portal-activity-stream-service` | Go | Same idea, for the customer portal webapp. |
| `integrations/sre-alert-ingestion-service` | Go | Accepts inbound alerts from external monitoring tools (Site24x7, Grafana, etc.), buffers them in its **own** dedicated Postgres database, and forwards them to `csm-integration-service` to create platform incidents. |
| PostgreSQL | — | System of record for entity-service, and separately for `sre-alert-ingestion-service`'s alert buffer. |
| Kafka | — | Local stand-in for Azure Event Hub's Kafka-compatible endpoint (production). See below. |

### Event backbone: Kafka standing in for Azure Event Hub

In every deployed environment, `entity-service`, `csm-notification-service`, and both
activity-stream services talk to **Azure Event Hub's Kafka-compatible endpoint** using
`github.com/segmentio/kafka-go`, authenticating with SASL/PLAIN over TLS (Event Hub's
required transport). That client configuration is **hardcoded** — there is no
plaintext/no-TLS code path anywhere in these services.

The local compose stack described below therefore runs a **real, local Apache Kafka
broker** (the official `apache/kafka` image, KRaft mode, Apache 2.0 licensed) configured
to accept the same protocol: TLS (via a throwaway CA) and SASL/PLAIN with the literal
username `$ConnectionString` that the Go client hardcodes. The CA and broker cert are
generated once into the persistent `csm-dev-certs` volume; a normal `docker compose up` /
`docker compose down` cycle reuses them, and they are only regenerated when that volume is
empty or has been removed with `docker compose down -v`. Production is Azure Event Hub;
local dev is real Kafka speaking the same wire protocol — no application code differs
between the two.

The topic used everywhere is `case-events`, plus a `case-events-dlq` dead-letter topic for
`csm-notification-service`.

## Running the full platform locally

A `docker-compose.yml` at the repo root brings up the entire CSM platform — both portals,
entity-service, Postgres, Kafka, and every integration listed above — against a real,
seeded PostgreSQL database, with a local mock identity provider so no access to a real IdP
is needed.

### Prerequisites

- Docker and Docker Compose v2 (`docker compose version`).
- About 4 GB of free RAM for the stack (Postgres, Kafka, 9 Go services, 2 webapps).
- Ports free on the host: `3000`, `3001`, `5433`, `8081`–`8087`, `8090`, `8092`, `9095`,
  `9096`, `9100`, `19094`. If any of these collide with something else you're already
  running, edit the `ports:` mappings in `docker-compose.yml` (only the **host** side,
  left of the `:`, needs to change).

### Bring the stack up

From the repo root:

```sh
docker compose up -d
```

First run builds every image (a few minutes — mostly Go module downloads and two
`pnpm install`s) and:

1. Starts PostgreSQL and Kafka (KRaft mode).
2. Generates a throwaway TLS CA + broker certificate (`certgen`).
3. Applies `entity-service`'s and `sre-alert-ingestion-service`'s raw SQL migrations, and
   loads dummy seed data — one account, project, deployment, case, comment, and time
   card — into a fresh `csm_platform` database (`migrate` service; see
   `scripts/csm-compose/seed-entity-service.sql`).
4. Creates the `case-events` / `case-events-dlq` Kafka topics.
5. Starts every backend, then both webapps.

Watch progress with `docker compose logs -f`, and check everything is up with
`docker compose ps`.

### URLs

| What | URL |
|---|---|
| CSM webapp (this portal) | http://localhost:3001 |
| Customer portal webapp | http://localhost:3000 |
| csm-portal backend (this portal's BFF) | http://localhost:8082 |
| customer-portal backend-v2 | http://localhost:8090 (REST), `:8092` (WebSocket) |
| entity-service | http://localhost:8081 |
| csm-integration-service | http://localhost:8084 |
| csm-notification-service (health only, no UI) | http://localhost:8083/health |
| csm-portal-activity-stream-service | health `:8085`, SSE `:9095` |
| customer-portal-activity-stream-service | health `:8086`, SSE `:9096` |
| sre-alert-ingestion-service | http://localhost:8087 |
| mock-oidc (discovery, JWKS, token, login) | http://localhost:9100 |
| Postgres | `localhost:5433` (`postgres` / `devpassword`, databases `csm_platform` and `sre_alerts`) |
| Kafka (external listener, SASL_SSL) | `localhost:19094` |

### Bring it down

```sh
docker compose down       # stop, keep data
docker compose down -v    # stop and wipe Postgres/Kafka/cert volumes -- next `up` reseeds from scratch
```

### Logging in via the mock identity provider

`scripts/csm-compose/mock-oidc` is a small, local-dev-only OpenID Connect provider. **It
performs no credential check at all** — it signs a real RS256 JWT for whatever email and
group list you type into its login form. Never point anything other than this local
compose stack at it.

It implements enough of the OIDC spec (discovery document, authorization-code flow with
PKCE, client-credentials grant, JWKS, userinfo) that both webapps' normal OIDC client code
and both BFFs' normal JWT-validation code run unmodified against it.

**Through a webapp:** open http://localhost:3001 (or :3000), let it redirect you to
`http://localhost:9100/oauth2/authorize`, type in any email address and a comma-separated
group list (e.g. `cs_engineer` for this portal), and submit. You'll be redirected back
signed in.

**From the command line**, to get a bearer token directly (useful for `curl`):

```sh
CODE=$(curl -s -i -X POST http://localhost:9100/oauth2/authorize \
  --data-urlencode "client_id=csm-portal-webapp" \
  --data-urlencode "redirect_uri=http://localhost:3001" \
  --data-urlencode "email=jane.doe@example.com" \
  --data-urlencode "groups=cs_engineer" \
  | grep -i '^location:' | sed -E 's/.*code=([^&]*)&?.*/\1/' | tr -d '\r\n')

TOKEN=$(curl -s -X POST http://localhost:9100/oauth2/token \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=$CODE" \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['access_token'])")

curl http://localhost:8082/users/me -H "x-jwt-assertion: $TOKEN"
```

Both BFFs run with real JWT signature validation **enabled**
(`AUTH_TOKEN_VALIDATOR_ENABLED=true`) against the mock provider's JWKS endpoint, so this
exercises the real validation code path, just against a throwaway signing key instead of a
real identity provider.

### Verifying the stack

A few checks that prove each piece is doing real work, not just running:

**Postgres has real seed data:**

```sh
docker exec $(docker ps -qf name=csm-platform-postgres-1) \
  psql -U postgres -d csm_platform -c "SELECT number, subject FROM work_item;"
```

**entity-service reads it back:**

```sh
curl http://localhost:8081/accounts/00000000-0000-0000-0000-000000000301
```
(`GET /health` always returns `{"status":"ok"}` without touching the database, so it proves
nothing about the seeded data; this instead does a plain by-id get against the account row
the seed data above creates. The `search` endpoints, including `/accounts/search`, have a
known bug on this branch — see [Known issues](#known-issues) below — which is why this uses
the by-id get rather than a search.)

**A BFF round-trips to entity-service and Postgres** (get a token as shown above, then):

```sh
curl http://localhost:8082/users/me -H "x-jwt-assertion: $TOKEN"
# -> real seeded user, e.g. {"id":"...","email":"jane.doe@example.com","firstName":"Jane","lastName":"Doe",...}
```

**csm-integration-service returns real M2M data:**

```sh
curl http://localhost:8084/accounts/00000000-0000-0000-0000-000000000301
# -> real seeded account "Example Corp"
```

**Kafka + notification-service + both activity-stream services**: publish a synthetic
event onto `case-events` (any Kafka client speaking TLS + SASL/PLAIN username
`$ConnectionString`, password = `EVENT_HUB_CONNECTION_STRING`), then watch:

```sh
docker compose logs -f csm-notification-service csm-portal-activity-stream-service customer-portal-activity-stream-service
```

You should see `dispatch: email sending disabled ... not sending` (notification-service
logs a real dispatch decision without needing real email credentials) and `caseevents:
received case event` from both activity-stream services.

**sre-alert-ingestion-service creates a real row:**

This service authenticates `POST /alerts` itself via HTTP Basic Auth (no gateway in front
of it locally, matching its real AKS deployment) — the `-u` flag below is required, not
optional. The dev-only credential (`devuser` / `devpassword`) is set via
`SRE_ALERT_AUTH_USERS` in `docker-compose.yml`.

```sh
curl -X POST http://localhost:8087/alerts -H 'Content-Type: application/json' \
  -u devuser:devpassword -d '{
  "source": "prometheus", "severity": "critical",
  "service": "smoke-test", "metricName": "test_metric",
  "description": "verification"
}'
# -> 202 {"alertNumber":"ALT0000001","id":"..."}

docker exec $(docker ps -qf name=csm-platform-postgres-1) \
  psql -U postgres -d sre_alerts -c "SELECT alert_number, status FROM alert_buffer;"
```

### Troubleshooting

- **A container keeps restarting right after `up`**: `docker compose logs <service>`.
  Almost always a missing/placeholder env var — every backend fails loudly (`os.Exit(1)`
  with a named variable) rather than silently, by design.
- **`Group Coordinator Not Available` in any Kafka-consuming service's logs**: only
  happens on a from-scratch Kafka data dir. This compose file already sets
  `KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1` (and the transaction-log equivalents) because
  a single-broker cluster can't satisfy Kafka's default replication factor of 3 for its
  own internal topics. If you hit this anyway (e.g. after hand-editing the Kafka service),
  it means `__consumer_offsets` failed to create — `docker compose down -v` and start
  clean.
- **A Kafka client (in a service, or your own test script) fails TLS verification against
  the local broker**: the broker's certificate is signed by a throwaway CA generated fresh
  by the `certgen` service into the `csm-dev-certs` volume on first boot. Any Go service
  that talks to Kafka needs `SSL_CERT_FILE=/certs/ca.crt` and the `csm-dev-certs` volume
  mounted (already wired up for `entity-service`, `csm-notification-service`, and both
  activity-stream services) — a plain `curl`/local script needs the same CA trusted.
- **`pnpm build` fails with `packages field missing or empty`** if you're building the
  customer-portal webapp outside its compose Dockerfile: that webapp's
  `pnpm-workspace.yaml` carries only `allowBuilds`/`minimumReleaseAgeExclude` settings, no
  `packages:` key, which the pinned pnpm version (9.15.x) refuses to build under. The
  Dockerfile works around it by deleting that file before `pnpm build` (it's not needed
  for a single-package build); this is a repo/tooling gap, not something to fix by
  hand-editing the file.
- **`WS_PORT` / port collisions**: `customer-portal-backend-v2`'s `WS_PORT` defaults to
  `8081`, the same as `entity-service`'s default port — this compose file sets it
  explicitly to `8092`. If you add a new service, check its actual default port in code,
  not just its `.env.example` (several vars are marked optional there but are enforced
  as non-empty, or even scheme-restricted, at startup).

### Known issues

These are **pre-existing application bugs on this branch**, not something introduced or
fixed by the compose stack. Reported here for visibility; do not treat their presence as a
defect in the Docker/compose setup itself.

- **`entity-service`'s repository layer queries table names that don't match its own
  migrations, for six of its core entities.** Every migration creates singular table names
  (`case`, `project`, `deployment`, `product`, `product_version`, `deployed_product`), but
  the corresponding repositories query plural names that don't exist:
  - `internal/repository/case_repo.go` — `cases` (e.g. lines 145, 209, 218-219, 444,
    814-817, 831)
  - `internal/repository/project_repo.go` — `projects` (lines 72, 77, 136)
  - `internal/repository/deployment_repo.go` — `deployments` (lines 80, 87)
  - `internal/repository/product_repo.go` — `products` (lines 67, 71)
  - `internal/repository/product_version_repo.go` — `product_versions` (lines 67, 73)
  - `internal/repository/deployed_product_repo.go` — `deployed_products` (lines 59, 66)

  Every code path that touches one of these (`POST /cases/search`, `GET /cases/{id}`,
  `POST /projects/search`, `POST /deployed-products/search`, case creation, case comments
  — which authorizes via `GetCase` first — and both activity-stream services' per-case SSE
  authorization, which also calls `GetCase`) fails with `relation "..." does not exist`
  against a freshly-migrated Postgres database. Other repositories (`account`, `comment`,
  `work_item`, `time_card`, `sla_clocks`, `event_publish_failures`,
  `scheduled_task_run`, `project_contact`, `product_vulnerability`, `user_role`) correctly
  match their migrated table names and work as expected — the seed data and verification
  steps above route around the six broken ones deliberately (seeding rows directly in
  Postgres, and verifying Kafka/SSE delivery with a directly-published synthetic event
  instead of a real case-creation call).
- **`sre-alert-ingestion-service`'s delivery to `csm-integration-service` requires an
  HTTPS endpoint** (`internal/csmclient` refuses non-HTTPS URLs outright). The compose
  stack runs `csm-integration-service` over plain HTTP, so a buffered alert is created and
  persisted correctly, but the worker's subsequent delivery attempt to create the
  downstream incident will retry and fail locally. This is a deliberate security control
  in that client, not a bug — fully exercising it locally would require fronting
  `csm-integration-service` with TLS, which the compose stack does not currently set up.

## Reporting Issues

### 1. Opening an issue

Please use this repository's issue tracker and include reproduction steps, expected behavior, actual behavior, and relevant logs.

### 2. Reporting security issues

Please do not report security issues through public issues. Follow the [WSO2 Security Vulnerability Reporting Guidelines](https://security.docs.wso2.com/en/latest/security-reporting/vulnerability-reporting-guidelines/).

## Contributing

Contributions are welcome. Create a feature branch, keep changes focused, and submit a pull request with a clear description and verification steps.

## License

CSM Portal is licensed under Apache 2.0. See the [LICENSE](../../LICENSE) file for details.
