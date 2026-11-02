# CSM Portal

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../../LICENSE)
[![GitHub last commit](https://img.shields.io/github/last-commit/wso2-open-operations/cs-tools/main?path=apps%2Fcsm-portal)](https://github.com/wso2-open-operations/cs-tools/commits/main/?path=apps/csm-portal)
[![GitHub issues](https://img.shields.io/github/issues/wso2-open-operations/cs-tools.svg)](https://github.com/wso2-open-operations/cs-tools/issues)

CSM Portal is the internal, customer-success-engineer-facing part of the **CSM platform**: an open-source
solution for customer success operations, built from a Go backend-for-frontend (BFF) and a React web
application. It lets engineers handle cases, projects, accounts, deployments, time tracking, operations
(incidents, change and service requests, problems) and security information from one workspace.

This README is the entry point for the whole platform: it explains the [architecture](#architecture), how to
[develop and run](#developer-guide) each piece, and how the pieces are [configured](#configuration).

## Contents

- [Why CSM Portal?](#why-csm-portal)
- [Features](#features)
- [Architecture](#architecture)
  - [Components](#components)
  - [Request flow](#request-flow)
  - [Authentication and authorization](#authentication-and-authorization)
  - [Data stores and events](#data-stores-and-events)
- [Developer guide](#developer-guide)
  - [Prerequisites](#prerequisites)
  - [Repository layout](#repository-layout)
  - [Run everything with Docker Compose](#run-everything-with-docker-compose)
  - [Run a single component](#run-a-single-component)
  - [Build, test and lint](#build-test-and-lint)
  - [Contributing](#contributing)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Reporting issues](#reporting-issues)
- [License](#license)

## Why CSM Portal?

Customer success teams need one workspace to track support cases, customer projects, deployments, product
updates and security information across accounts. Managing these in disparate systems creates disconnected
workflows and duplicated effort. CSM Portal combines:

- a Go BFF that validates the caller's identity and fronts the platform's upstream services, and
- a React single-page application for the engineer workflows.

## Features

- **Case management**: create, search, patch and comment on cases; attachments; call requests; escalations.
- **Projects and deployments**: customer projects, deployments and deployed products across accounts.
- **Operations**: incidents, change requests, service requests and problems.
- **Time cards**: time spent per case, with an approval flow.
- **Security center**: product vulnerability disclosures per account.
- **Product updates**: update levels and upgrade paths between product versions.
- **Role-based access**: portal roles map to the permissions of each route (see
  [Authentication and authorization](#authentication-and-authorization)).
- **In-app user guide**: the webapp serves a `/help` route with one topic per navigation section.

## Architecture

The CSM platform replaces a legacy, external ticketing-system-backed stack with a purpose-built one.
Internal engineers work customer cases through **this portal**; customers use a separate, existing
**customer portal** (`apps/customer-portal`). Each portal has its own BFF, and both read and write the same
PostgreSQL-backed **entity service**, which is the platform's system of record.

```text
        Engineers                                          Customers
            |                                                  |
   +--------v---------+                              +---------v----------+
   |  CSM webapp      |                              | Customer webapp    |
   |  (React SPA)     |                              | (React SPA)        |
   +--------+---------+                              +---------+----------+
            | HTTPS, Bearer token                              | HTTPS, Bearer token
   +--------v-------------------------------------------------v----------+
   |  API gateway: validates the token, forwards the identity as          |
   |  the x-jwt-assertion header (OIDC identity provider supplies keys)   |
   +--------+-------------------------------------------------+----------+
            |                                                  |
   +--------v---------+                              +---------v----------+
   | csm-portal       |                              | customer-portal    |
   | backend (Go BFF) |                              | backend-v2 (Go BFF)|
   +--------+---------+                              +---------+----------+
            |   REST + OAuth2 client credentials               |
            +--------------------+-----------------------------+
                                 |
                       +---------v----------+      REST (machine-to-machine)
                       |   entity-service   |<-----------------------------+
                       |   (Go, REST)       |                              |
                       +----+----------+----+                  +----------+-----------+
                            |          |                       | csm-integration-     |
                  SQL       |          | publishes events      | service (M2M API)    |
                  +---------v--+   +---v------------------+    +----------^-----------+
                  | PostgreSQL |   | Event bus (Kafka API)|               |
                  | (system of |   | topic: case-events   |    sre-alert-ingestion / core
                  |  record)   |   +--+---------+------+--+      (alert -> incident)
                  +------------+      |         |      |
                            consumed  |         |      |
                      +---------------v+  +-----v------v---------------+
                      | csm-notification|  | activity-stream services   |
                      | -service        |  | (one per portal) -> SSE    |
                      | email/chat/voice|  +----------------------------+
                      +----------------+

   operations/csm-scheduled-tasks  --REST-->  entity-service   (recurring jobs)
   operations/csm-webhooks/github  --REST-->  entity-service   (inbound webhooks)
```

### Components

| Component | Path | Language | Role |
|---|---|---|---|
| CSM webapp | [`apps/csm-portal/webapp`](./webapp/README.md) | React + TypeScript | The engineer UI. Talks only to the CSM backend (and, optionally, the activity-stream service), never to the entity service. |
| CSM backend (BFF) | [`apps/csm-portal/backend`](./backend/README.md) | Go | Validates the caller's token, enforces portal roles per route, and fronts the entity service plus the updates and directory (SCIM) services. Contract: [`backend/openapi.yaml`](./backend/openapi.yaml). |
| CSM microapp | [`apps/csm-portal/microapp`](./microapp) | React + TypeScript | Mobile micro-app variant of the portal UI. |
| Customer portal | [`apps/customer-portal`](../customer-portal/README.md) | Go BFF (`backend-v2`) + React | The customer-facing portal. `backend-v2` is the PostgreSQL-era backend; `apps/customer-portal/backend` is the older, deprecated stack. |
| Entity service | [`entity-service`](../../entity-service/README.md) | Go | System of record API over PostgreSQL. Publishes domain events (for example `case.created`, `comment.added`) to the event bus. Can also be configured with an external ticketing/case system adapter (`DATA_SOURCE`); the default and the one used here is `postgres`. |
| Integration service | [`integrations/csm-integration-service`](../../integrations/csm-integration-service/README.md) | Go | Machine-to-machine REST API (accounts, projects, contacts, a subset of cases) for third-party callers. Inbound trust is delegated to the API gateway. |
| AI integration service | [`integrations/csm-ai-integration-service`](../../integrations/csm-ai-integration-service/README.md) | Go | Every model call of the platform (today: dashboard generation for this portal's backend), with the catalogues the model reads. Organization-visible; the backend is its only caller. |
| Notification service | [`integrations/csm-notification-service`](../../integrations/csm-notification-service/README.md) | Go | Event consumer (health endpoint only) that sends email, chat and voice-call notifications. |
| Activity-stream services | [`integrations/csm-portal-activity-stream-service`](../../integrations/csm-portal-activity-stream-service/README.md), [`integrations/customer-portal-activity-stream-service`](../../integrations/customer-portal-activity-stream-service/README.md) | Go | Consume the event stream and expose a per-case Server-Sent-Events feed to each portal's activity tab. |
| Alert services | [`integrations/sre-alert-ingestion-service`](../../integrations/sre-alert-ingestion-service/README.md), [`integrations/sre-alert-core-service`](../../integrations/sre-alert-core-service/README.md) | Go | Ingest monitoring alerts, deduplicate them into incidents and forward them to the platform through the integration service. |
| Other integrations | [`integrations/acp-closure-service`](../../integrations/acp-closure-service/README.md), [`integrations/sftpgo-authentication-service`](../../integrations/sftpgo-authentication-service/README.md) | Go | Account-closure job and file-transfer authentication hook. |
| Scheduled tasks | [`operations/csm-scheduled-tasks`](../../operations/csm-scheduled-tasks/README.md) | Go | One run-to-completion component that runs several cron-style sub-tasks per invocation, with retry state kept in the entity service. |
| Webhooks | [`operations/csm-webhooks`](../../operations/csm-webhooks/README.md) | Go | One folder per inbound webhook source (currently `github`). Verifies the signature and forwards to the entity service. |
| PostgreSQL | n/a | n/a | System of record (owned by the entity service). |
| Event bus | n/a | n/a | Kafka-compatible endpoint. Local dev uses Apache Kafka; deployed environments use a managed Kafka-compatible service. |

### Request flow

A typical read from the CSM portal, end to end:

1. The webapp signs the engineer in with an OIDC authorization-code flow (PKCE) against the configured
   identity provider and holds the resulting access token.
2. The webapp calls the CSM backend with `Authorization: Bearer <token>`.
3. The API gateway validates the token and forwards the request to the backend with the token in the
   `x-jwt-assertion` header. (Locally, a small gateway shim does this header rewrite; see
   [Run everything with Docker Compose](#run-everything-with-docker-compose).)
4. The backend's auth middleware verifies the `x-jwt-assertion` JWT again (signature via the provider's JWKS
   endpoint, issuer, audience, expiry) and extracts the caller's identity and roles. Each route is registered
   with a required permission, so a request without the needed role gets `403`.
5. The backend calls the entity service (and, for some routes, the updates or directory services) using its own
   OAuth2 client-credentials token, forwarding the end user's identity where the upstream scopes data by user.
6. The entity service reads or writes PostgreSQL and, for domain changes, publishes an event.
7. Event consumers (notification, activity-stream services) react asynchronously; the activity-stream services
   push updates to open case pages over SSE.

### Authentication and authorization

- **Authentication** is delegated to an OIDC identity provider. The platform never stores credentials. The
  gateway validates the token at the edge, and services behind it trust the gateway-validated *identity*
  (`x-jwt-assertion`). The CSM backend additionally verifies the JWT signature against the provider's JWKS
  endpoint unless `AUTH_TOKEN_VALIDATOR_ENABLED=false` (local development only).
- **Authorization** is decided by the platform, not the gateway. In the CSM backend, the token's `roles` claim
  is matched against role names configured through `AUTH_*_ROLES` variables, producing portal roles such as
  viewer, `cs_engineer`, escalator and admin; each route requires one permission. The backend returns the
  caller's portal roles from `GET /users/me`, and the backend's `403` is the real gate. User, team and role
  records live in PostgreSQL behind the entity service. See
  [`backend/README.md`](./backend/README.md#access-control) for the full role and permission table.
- **Entity service** validates the end user's ID token (`x-user-id-token`) when a portal backend acts for a
  user, and treats `x-jwt-assertion` client-credentials callers as service-to-service callers. Machine-to-machine
  client ids are listed in its `M2M_CLIENT_IDS` setting. See
  [`entity-service/README.md`](../../entity-service/README.md).
- **Machine-to-machine** services (`csm-integration-service`, webhooks, scheduled tasks) authenticate to the
  entity service as OAuth2 clients and rely on the API gateway for inbound trust.

### Data stores and events

- **PostgreSQL** is the system of record, accessed only by the entity service (and the alert services, which
  use their own tables). Migrations live in [`entity-service/migrations`](../../entity-service/migrations).
- **Redis** is optional: the entity service can use it as a user cache (`REDIS_URL` or `REDIS_ADDR`).
  Caches are read-performance only; PostgreSQL stays authoritative.
- **Event bus**: the entity service publishes to the `case-events` topic (plus `sre-events` and optionally
  per-domain topics), and consumers use consumer groups. Failed consumer records go to a dead-letter topic
  (`case-events-dlq`). All Go services use `github.com/segmentio/kafka-go` with SASL/PLAIN over TLS, which is
  the transport of a managed Kafka-compatible endpoint. The local Compose stack therefore runs a real Apache
  Kafka broker (KRaft mode) with a throwaway CA and SASL/PLAIN username `$ConnectionString`, so no application
  code differs between local and deployed environments. The CA and broker certificate are generated once into
  the persistent `csm-dev-certs` volume and reused until that volume is removed.

## Developer guide

### Prerequisites

| Tool | Version | Needed for |
|---|---|---|
| Go | 1.26+ (`go.mod` declares `go 1.26.9`) | backend, entity-service, integrations, operations |
| Node.js | 20+ (the Compose image builds with Node 20) | webapp |
| pnpm | 9+ | webapp (`pnpm-lock.yaml`) |
| Docker with Compose v2 | recent | the full local stack |
| PostgreSQL | 15+ | running the entity service outside Docker |

### Repository layout

```text
cs-tools/
├── apps/
│   ├── csm-portal/
│   │   ├── backend/         # Go BFF (cmd/server, internal/, openapi.yaml)
│   │   ├── webapp/          # React + TypeScript SPA
│   │   ├── microapp/        # mobile micro-app UI
│   │   └── README.md        # this file
│   └── customer-portal/     # customer-facing portal (backend-v2 is the Go BFF)
├── entity-service/          # Go system-of-record API, migrations/, openapi.yaml
├── integrations/            # event consumers, M2M API, alert services, other integrations
├── operations/              # csm-scheduled-tasks, csm-webhooks
├── docker/local/            # Dockerfiles used only by the local Compose stack
├── scripts/csm-compose/     # local-dev tooling: mock identity provider, gateway shim, seed data
├── docker-compose.yml       # the full local platform
└── .githooks/pre-push       # optional pre-push test hook
```

### Run everything with Docker Compose

The root [`docker-compose.yml`](../../docker-compose.yml) brings up the whole platform against a seeded
PostgreSQL database, with a local mock identity provider so no access to a real one is needed. It is for local
development only.

Prerequisites: Docker with Compose v2 and roughly 4 GB of free RAM. These host ports must be free (change only
the host side, left of the `:`, in `docker-compose.yml` if they collide): `3000`, `3001`, `5433`, `6379`,
`8081`–`8086`, `8090`, `8092`, `9095`, `9096`, `9100`, `19094`.

```sh
docker compose up -d      # from the repository root; the first run builds every image (a few minutes)
docker compose ps
docker compose logs -f <service>
```

On startup the stack:

1. starts PostgreSQL, Redis and Kafka, and generates the throwaway TLS CA (`certgen`);
2. applies the entity service's SQL migrations and loads seed data (`migrate`, using
   `scripts/csm-compose/seed-entity-service.sql`, then `seed-generator`, which adds a broader randomized
   data set and is idempotent);
3. creates the `case-events`, `case-events-dlq`, `sre-events` and `sre-events-dlq` topics;
4. starts the backends, gateway shims and webapps.

| What | URL |
|---|---|
| CSM webapp | http://localhost:3001 |
| Customer portal webapp | http://localhost:3000 |
| CSM backend (via gateway shim) | http://localhost:8082 |
| Customer portal backend-v2 | http://localhost:8090 (REST), `:8092` (WebSocket) |
| entity-service | http://localhost:8081 |
| csm-integration-service | http://localhost:8084 |
| csm-ai-integration-service | http://localhost:8087 |
| csm-notification-service (health only) | http://localhost:8083/health |
| csm-portal-activity-stream-service | health `:8085`, SSE `:9095` |
| customer-portal-activity-stream-service | health `:8086`, SSE `:9096` |
| Mock identity provider | http://localhost:9100 |
| PostgreSQL | `localhost:5433` (user `postgres`, password `devpassword`, database `csm_platform`; local dev only) |
| Redis | `localhost:6379` |
| Kafka (external listener, SASL_SSL) | `localhost:19094` |

Stop the stack with `docker compose down` (keeps data) or `docker compose down -v` (also wipes the Postgres,
Kafka and certificate volumes, so the next `up` reseeds from scratch).

**Signing in.** [`scripts/csm-compose/mock-oidc`](../../scripts/csm-compose/mock-oidc) is a local-only OIDC
provider (discovery, authorization-code flow with PKCE, client-credentials grant, JWKS, userinfo). It performs
**no credential check**: it signs a real RS256 JWT for whatever email and comma-separated group list you enter.
Never point anything except this local stack at it. Open http://localhost:3001, enter any email and a group such
as `cs_engineer` (the Compose file maps `cs_engineer` to the engineer role through `AUTH_SUPPORT_ENGINEER_ROLES`;
other roles use `cs_viewer`, `cs_admin`, and so on, see `docker-compose.yml`).

**Gateway shim.** Browsers send `Authorization: Bearer <token>`, but the backends read `x-jwt-assertion`. In
production the API gateway performs that rewrite; locally
[`scripts/csm-compose/gateway-shim`](../../scripts/csm-compose/gateway-shim) does, and publishes the host ports
`8082` (CSM) and `8090` (customer portal) in front of the backends.

**Calling an API from the command line:**

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

curl http://localhost:8082/users/me -H "Authorization: Bearer $TOKEN"
```

Both BFFs run with real JWT signature validation enabled (`AUTH_TOKEN_VALIDATOR_ENABLED=true`) against the mock
provider's JWKS endpoint, so this exercises the production validation path with a throwaway signing key.

**Checks that prove each piece does real work:**

```sh
# Postgres has seed data
docker exec $(docker ps -qf name=csm-platform-postgres-1) \
  psql -U postgres -d csm_platform -c "SELECT number, subject FROM work_item LIMIT 5;"

# entity-service reads it back (GET /health never touches the database, so it proves nothing about data)
curl http://localhost:8081/accounts/00000000-0000-0000-0000-000000000301

# csm-integration-service returns the same account over the M2M API
curl http://localhost:8084/accounts/00000000-0000-0000-0000-000000000301
```

To exercise the event pipeline, publish an event to `case-events` with any Kafka client speaking TLS and
SASL/PLAIN (username `$ConnectionString`, password = the `EVENT_HUB_CONNECTION_STRING` set in
`docker-compose.yml`), then watch
`docker compose logs -f csm-notification-service csm-portal-activity-stream-service customer-portal-activity-stream-service`.

### Run a single component

Use this when you already have the upstream services available (for example the Compose stack for the entity
service, with only the component you are editing running on the host).

| Component | Command (from the component directory) | Details |
|---|---|---|
| CSM backend | `cp .env.example .env`, fill it in, then `go run ./cmd/server/main.go` (listens on `:8080`) | [`backend/README.md`](./backend/README.md) |
| CSM webapp | `pnpm install`, `cp public/config.js.example public/config.js`, edit it, then `pnpm run dev` (serves on `:3001`) | [`webapp/README.md`](./webapp/README.md) |
| CSM webapp, fully local | `pnpm run dev:local`: the Vite dev server acts as the gateway, pointing at a mock identity provider on `:9100` and a backend on `:8080` | [`webapp/README.md`](./webapp/README.md#local-full-stack-mode-sandboxed-dev) |
| entity-service | `cp .env.example .env`, fill in the database settings, then `go run cmd/api/main.go` (listens on `:8080`, health on `:8081`) | [`entity-service/README.md`](../../entity-service/README.md) |
| csm-integration-service | `go run ./cmd/server/main.go` | [README](../../integrations/csm-integration-service/README.md) |
| csm-ai-integration-service | `go run ./cmd/server` | [README](../../integrations/csm-ai-integration-service/README.md) |
| Other integrations and operations | see each component's README | [`integrations/`](../../integrations), [`operations/`](../../operations) |

The backend and the entity service load a `.env` file from the working directory automatically (a missing file is
ignored). Never commit `.env` or `public/config.js`; only the `*.example` templates are tracked.

### Build, test and lint

| Component | Command | Notes |
|---|---|---|
| CSM backend | `make test` (runs `go vet ./...` then `go test -race ./...`) | `make build` runs the tests then builds `./server`; `make setup` installs the shared git hooks |
| CSM backend | `go test ./...` | faster, without the race detector |
| CSM webapp | `pnpm run lint` (ESLint), `pnpm run test` (Vitest), `pnpm run build` (`tsc -b && vite build`) | `pnpm run test:e2e` runs Playwright |
| entity-service | `make build`, `make test`, `make lint` (`go vet ./...`) | `make migrate` applies the SQL migrations to a database |
| Other Go components | `go build ./...` and `go test ./...` in the component directory | some have their own `Makefile` |

Security scanning commands (`gosec`, `govulncheck`) are documented in the backend and entity-service READMEs.

### Contributing

Contributions are welcome. Create a feature branch, keep changes focused, and open a pull request with a clear
description and verification steps. Use the repository's [issue template](../../issue_template.md) and
[pull request template](../../pull_request_template.md). Run
`git config core.hooksPath .githooks` (or `make setup` in the backend) to enable the optional
[`.githooks/pre-push`](../../.githooks/pre-push) hook, which runs the tests of the components a push touches.
If you change a user-facing flow in the webapp, also update the matching topic under
`webapp/src/features/help/content/`.

## Configuration

Each component is configured through environment variables, except the webapp, which reads a runtime
`window.config` object from `public/config.js`. Names below are the ones the code reads; see each component's
README (and its `.env.example`) for the full list, defaults and validation rules.

| Component | Mechanism | Key settings | Full reference |
|---|---|---|---|
| CSM webapp | `public/config.js` (from `config.js.example`) | `CSM_PORTAL_AUTH_BASE_URL`, `CSM_PORTAL_AUTH_CLIENT_ID`, `CSM_PORTAL_BACKEND_BASE_URL`; optional sign-in/sign-out redirect URLs (default to the page origin), stream, theme, feature-override and banner keys | [`webapp/README.md`](./webapp/README.md#configuration) |
| CSM backend | env / `.env` (from `.env.example`) | `OAUTH2_CLIENT_ID`, `OAUTH2_CLIENT_SECRET`, `OAUTH2_TOKEN_URL`; `CUSTOMER_ENTITY_BASE_URL`, `UPDATES_BASE_URL`, `SCIM_BASE_URL` (+ `*_SCOPES`); `AUTH_JWKS_ENDPOINT`, `AUTH_ISSUER`, `AUTH_AUDIENCE`, `AUTH_TOKEN_VALIDATOR_ENABLED`; `AUTH_*_ROLES`; `CORS_ALLOWED_ORIGINS`, `PORT` | [`backend/README.md`](./backend/README.md#configuration-reference) |
| entity-service | env / `.env` (from `.env.example`) | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`; `SERVER_PORT`, `HEALTH_PORT`; `DATA_SOURCE` (default `postgres`); `AUTH_ISSUER`, `AUTH_JWKS_URL`, `AUTH_USER_TOKEN_AUDIENCES`; `M2M_CLIENT_IDS`; `EVENT_PUBLISHING_ENABLED`, `EVENT_HUB_BROKER`, `EVENT_HUB_CONNECTION_STRING`, `EVENT_HUB_TOPIC`; optional `REDIS_URL` / `REDIS_ADDR` | [`entity-service/README.md`](../../entity-service/README.md#configuration-reference) |
| csm-integration-service | env / `.env` | `ENTITY_BASE_URL`, `ENTITY_TOKEN_URL`, `ENTITY_CLIENT_ID`, `ENTITY_CLIENT_SECRET`, `ENTITY_SCOPES`, `PORT` | [README](../../integrations/csm-integration-service/README.md) |
| csm-ai-integration-service | env / `.env` | `ANTHROPIC_API_KEY` (secret), `CLAUDE_BASE_URL`, `CLAUDE_MODEL`, `CLAUDE_TIMEOUT`, `CATALOGUE_DIR`, `PORT` | [README](../../integrations/csm-ai-integration-service/README.md) |
| csm-notification-service | env / `.env` | `EVENT_HUB_*` consumer settings, channel credentials, `PORT` | [README](../../integrations/csm-notification-service/README.md) |
| Activity-stream services | env / `.env` | `EVENT_HUB_*`, `OAUTH2_*`, `CUSTOMER_ENTITY_BASE_URL`, `AUTH_JWKS_ENDPOINT`, `AUTH_ISSUER`, `AUTH_AUDIENCE`, `HEALTH_PORT`, `STREAM_PORT` | [CSM](../../integrations/csm-portal-activity-stream-service/README.md), [customer](../../integrations/customer-portal-activity-stream-service/README.md) |
| Scheduled tasks, webhooks, alert services | env or `config.toml` | see each README | [`operations/`](../../operations), [`integrations/`](../../integrations) |

How the settings line up across components (the same value must agree in both places):

| Concern | Set on | Must match |
|---|---|---|
| Token issuer and keys | `AUTH_ISSUER` + `AUTH_JWKS_ENDPOINT` (backend, activity-stream), `AUTH_ISSUER` + `AUTH_JWKS_URL` (entity-service) | The identity provider's issuer and JWKS URL |
| Token audience | `AUTH_AUDIENCE` (backend, activity-stream), `AUTH_USER_TOKEN_AUDIENCES` (entity-service) | The webapp's OIDC client id (`CSM_PORTAL_AUTH_CLIENT_ID`) |
| Backend to entity-service | `CUSTOMER_ENTITY_BASE_URL`, `OAUTH2_*` (backend) | The entity service's address; the backend's client id must be known to the entity service |
| M2M callers | `M2M_CLIENT_IDS` (entity-service) | The OAuth2 client ids of the integration, notification, webhook and scheduled-task components |
| Browser to backend | `CSM_PORTAL_BACKEND_BASE_URL` (webapp), `CORS_ALLOWED_ORIGINS` (backend) | The gateway URL in front of the backend; the webapp's origin |
| Event bus | `EVENT_HUB_BROKER`, `EVENT_HUB_CONNECTION_STRING`, `EVENT_HUB_TOPIC` (entity-service, consumers) | The same broker and topic everywhere; consumers also need their own consumer group |
| Portal roles | `AUTH_*_ROLES` (backend) | The role names the identity provider puts in the token's `roles` claim |

The Compose file sets working local values for all of this; it is the most complete worked example.

## Troubleshooting

- **A container keeps restarting after `up`**: run `docker compose logs <service>`. It is almost always a
  missing or placeholder environment variable; the backends exit at startup naming the variable instead of
  failing silently.
- **`Group Coordinator Not Available` in a Kafka consumer**: only happens on a from-scratch Kafka data
  directory when a single broker cannot meet the default internal-topic replication factor. The Compose file
  already sets the replication factors to 1; if you hit it anyway, run `docker compose down -v` and start clean.
- **A Kafka client fails TLS verification against the local broker**: the broker certificate is signed by the
  throwaway CA in the `csm-dev-certs` volume. Go services need `SSL_CERT_FILE=/certs/ca.crt` with that volume
  mounted (already wired in Compose); a script on the host must trust the same CA.
- **`pnpm build` fails with `packages field missing or empty`** when building the customer-portal webapp outside
  its Dockerfile: that webapp's `pnpm-workspace.yaml` has no `packages:` key, which pnpm 9.15 rejects. The
  Dockerfile works around it by removing that file before building.
- **Port collisions**: check a service's real default port in code, not only its `.env.example` (for example
  `customer-portal-backend-v2`'s `WS_PORT` defaults to `8081`, the same as the entity service's health port, so
  Compose sets it to `8092`).
- **401 from a backend you call directly**: send the token as `x-jwt-assertion`, not `Authorization`, or go
  through the gateway shim on `:8082` / `:8090`.

## Reporting issues

1. **Opening an issue**: use this repository's issue tracker and include reproduction steps, expected behavior,
   actual behavior and relevant logs.
2. **Security issues**: do not report them through public issues. Follow the
   [WSO2 Security Vulnerability Reporting Guidelines](https://security.docs.wso2.com/en/latest/security-reporting/vulnerability-reporting-guidelines/).

## License

CSM Portal is licensed under Apache 2.0. See the [LICENSE](../../LICENSE) file for details.
