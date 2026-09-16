# Customer Portal Backend (v2)

Rewrite of the existing backend at `apps/customer-portal/backend`. It is a backend-for-frontend
(BFF) for the customer portal: it authenticates callers, forwards requests to
[`entity-service`](../../../entity-service) (this repo's `cs-tools/entity-service`, not the
`digiops-cs/entity-service` the existing backend targets), and shapes the responses for the frontend.

This is a work in progress — only the 104 routes listed below are implemented so far, across
entity-service, the WSO2 Updates service, SCIM, the AI chat agent, the product-consumption
service, the registry (robot-account) service, and the project-contact onboarding service (six
more separate services — see [CLAUDE.md](./CLAUDE.md#the-ai-chat-agent),
[CLAUDE.md](./CLAUDE.md#the-product-consumption-service),
[CLAUDE.md](./CLAUDE.md#the-registry-service), and
[CLAUDE.md](./CLAUDE.md#the-project-contact-onboarding-service)). Everything else the existing
backend exposes still needs a Go handler; add them following the pattern described in
[CLAUDE.md](./CLAUDE.md#adding-a-new-endpoint).

## Quick Start

```bash
# from apps/customer-portal/backend-v2
go run ./cmd/server/main.go
```

The server automatically loads `.env` from the working directory on startup (silently ignored if absent).

Backend starts at `http://localhost:8080`.

## Overview

- Default port: `8080`
- Runtime: Go `1.26+`
- Entry point: `cmd/server/main.go`
- Authentication:
  - Incoming requests: JWT Bearer token; pass as `x-jwt-assertion` header when testing locally
  - Outbound calls to entity-service, the Updates service, SCIM, the AI chat agent, the
    product-consumption service, the registry service, and the project-contact onboarding service:
    OAuth2 client credentials grant, shared across all seven (optional — entity-service itself does
    not validate inbound credentials, see [CLAUDE.md](./CLAUDE.md))

## Prerequisites

- Go `1.26+` — [install](https://go.dev/doc/install)
- A running instance of `cs-tools/entity-service` (see its own README for `DATA_SOURCE` setup —
  `GET`/`PATCH /users/me` require `DATA_SOURCE=servicenow`)
- A running instance of the WSO2 Updates service and the SCIM operations service (for the
  `/updates/*` routes and phone-number fields on `/users/me`)
- A running instance of the AI chat agent (a separate Python service, not entity-service — for
  `/cases/classify`, `/conversations/*`, `/projects/*/conversations/*`, and `/ws`)
- A running instance of the product-consumption service (a separate service, not entity-service —
  for `/projects/*/deployments/*/license` and `/deployment-usages`)
- A running instance of the registry (robot-account) service (a separate service, not
  entity-service — for `/projects/*/registry-tokens*`, `/registry-tokens/*`, and
  `/projects/*/integration-users`)
- A running instance of the project-contact onboarding service (a separate service, not
  entity-service, not SCIM — for `/projects/*/contacts*`)

## Testing

```bash
go test ./...
go test -race ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

Or use `make`:

```bash
make test    # vet + test
make build   # vet + test + compile
```

### Run tests before every push (recommended)

```bash
git config core.hooksPath .githooks   # from the repo root, once
# or: make setup   # from this directory
```

## Security Scanning

```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec -fmt=text ./...
```

The scan must report **0 issues** before opening a PR touching this backend.

## Configuration

Copy `.env.example` to `.env` and fill in the values.

### Shared OAuth2 client credentials

Every upstream service client (entity-service, updates, SCIM, the AI chat agent, the
product-consumption service, the registry service, and the project-contact onboarding service)
authenticates as the same OAuth2 client-credentials app — only each service's base URL and scopes
differ.

| Variable | Description |
|---|---|
| `OAUTH2_CLIENT_ID` / `OAUTH2_CLIENT_SECRET` / `OAUTH2_TOKEN_URL` | Optional — only needed if a service sits behind a gateway requiring OAuth2 client-credentials auth |

### Entity service

| Variable | Description |
|---|---|
| `ENTITY_SERVICE_BASE_URL` | Base URL of `cs-tools/entity-service` |
| `ENTITY_SERVICE_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Updates service

| Variable | Description |
|---|---|
| `UPDATES_BASE_URL` | Base URL of the WSO2 Updates service |
| `UPDATES_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### SCIM operations service

| Variable | Description |
|---|---|
| `SCIM_BASE_URL` | Base URL of the SCIM operations service |
| `SCIM_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### AI chat agent

A separate Python service (not entity-service) — see [CLAUDE.md](./CLAUDE.md#the-ai-chat-agent).

| Variable | Description |
|---|---|
| `AI_CHAT_AGENT_BASE_URL` | Base URL of the AI chat agent's HTTP API |
| `AI_CHAT_AGENT_SCOPES` | Comma-separated OAuth2 scopes (optional) |
| `AI_CHAT_AGENT_WS_BASE_URL` | Base URL of the AI chat agent's WebSocket endpoint |
| `AI_CHAT_AGENT_WS_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Product-consumption service

Not entity-service — see [CLAUDE.md](./CLAUDE.md#the-product-consumption-service). The
subscription/license API and the usage-tracking API are two independently configurable base URLs,
so set both here too.

| Variable | Description |
|---|---|
| `PRODUCT_CONSUMPTION_SUBSCRIPTION_URL` | Base URL of the subscription/license API |
| `PRODUCT_CONSUMPTION_TRACKING_BASE_URL` | Base URL of the usage-tracking API (optional — falls back to `PRODUCT_CONSUMPTION_SUBSCRIPTION_URL` when unset) |
| `PRODUCT_CONSUMPTION_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Registry service

A separate service (not entity-service) — see [CLAUDE.md](./CLAUDE.md#the-registry-service).

| Variable | Description |
|---|---|
| `REGISTRY_BASE_URL` | Base URL of the registry (robot-account) service |
| `REGISTRY_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Project-contact onboarding service

A separate service (not entity-service, not SCIM) — see
[CLAUDE.md](./CLAUDE.md#the-project-contact-onboarding-service).

| Variable | Description |
|---|---|
| `USER_MANAGEMENT_BASE_URL` | Base URL of the project-contact onboarding service |
| `USER_MANAGEMENT_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Auth

| Variable | Description |
|---|---|
| `AUTH_JWKS_ENDPOINT` | JWKS endpoint used to verify JWT signatures |
| `AUTH_ISSUER` | Expected `iss` claim value |
| `AUTH_AUDIENCE` | Comma-separated accepted `aud` values |
| `AUTH_TOKEN_VALIDATOR_ENABLED` | `false` skips JWT signature verification — **local development only**; `.env.example` ships `false` for local convenience. Production **must** set this to `true` with a real `AUTH_JWKS_ENDPOINT`/`AUTH_ISSUER`/`AUTH_AUDIENCE` |
| `AUTH_ADMIN_ROLE` | The role string (from entity-service's `GET /users/me` `roles`) that grants admin privileges for registry-token and project-contact management |

### Server

| Variable | Description |
|---|---|
| `PORT` | REST server listen port — a plain number, not an address (default `8080`) |
| `WS_PORT` | WebSocket (`GET /ws`) listen port — a separate listener from `PORT`, must match the `customer-portal-websocket` endpoint in `.choreo/component.yaml` (default `8081`) |

## Project Structure

```text
backend-v2/
├── cmd/server/main.go           # Entry point — routes + server startup
├── internal/
│   ├── apierror/                # Typed upstream error type (4xx/5xx passthrough)
│   ├── entity/                  # OAuth2 HTTP client for cs-tools/entity-service
│   │   ├── client.go            # Config/Client/do()/getJSON()/postJSON()/patchJSON()
│   │   ├── types.go             # entity-service's wire-format structs (internal to this package)
│   │   ├── users.go             # GetMe, PatchMe
│   │   ├── accounts.go          # GetAccount
│   │   ├── projects.go          # SearchProjects, GetProject
│   │   ├── cases.go             # SearchCases, GetCase, CreateCase, UpdateCase, CreateCaseComment, SearchCaseActivities
│   │   ├── deployments.go       # SearchDeployments, CreateDeployment
│   │   ├── deployed_products.go # SearchDeployedProducts, CreateDeployedProduct, UpdateDeployedProduct
│   │   ├── attachments.go       # CreateAttachment, GetAttachmentContent, DeleteAttachment
│   │   ├── products.go          # SearchProducts, SearchProductVersions
│   │   ├── change_requests.go   # create/search/get/update, approvals get/decide
│   │   ├── call_requests.go     # CreateCallRequest, SearchCallRequests, UpdateCallRequest
│   │   ├── comments.go          # CreateComment (generic, any reference entity)
│   │   ├── conversations.go     # SearchConversations
│   │   ├── product_vulnerabilities.go # SearchProductVulnerabilities, GetProductVulnerability, GetVulnerabilityMeta
│   │   ├── catalogs.go          # SearchCatalogs, GetCatalogItemVariables
│   │   ├── time_cards.go        # SearchTimeCards, SearchCaseTimeCards
│   │   ├── global.go            # GetSystemMetadata, GlobalSearch
│   │   └── instances.go         # SearchInstances, SearchInstanceMetrics/Usage/MetricsStats/UsageStats
│   ├── registry/                # OAuth2 HTTP client for the registry (robot-account) service (not entity-service)
│   │   ├── client.go            # Config/Client/do()
│   │   ├── types.go
│   │   └── registry.go          # CreateToken, SearchTokens, GetTokenByID, DeleteToken, RegenerateToken, GetIntegrationUsersByProjectID, DeriveTokenInfoFromDescription
│   ├── usermanagement/           # OAuth2 HTTP client for the project-contact onboarding service (not entity-service, not SCIM)
│   │   ├── client.go            # Config/Client/do()/doJSON()
│   │   ├── types.go             # wire (semicolon-delimited role string) <-> portal (per-role booleans) translation
│   │   └── usermanagement.go    # GetProjectContacts, CreateProjectContact, RemoveProjectContact, UpdateMembershipRole, ValidateProjectContact
│   ├── updates/                 # OAuth2 HTTP client for the WSO2 Updates service
│   │   ├── client.go            # Config/Client/do()
│   │   ├── types.go             # upstream (snake_case) vs portal (camelCase) structs
│   │   ├── mapper.go            # snake_case <-> camelCase mapping
│   │   └── updates.go           # GetProductUpdateLevels, SearchUpdatesBetweenUpdateLevels
│   ├── scim/                    # OAuth2 HTTP client for the SCIM operations service
│   │   ├── client.go            # Config/Client/do()
│   │   ├── types.go
│   │   └── scim.go              # SearchUser, UpdateUserPhone
│   ├── aichatagent/              # OAuth2 HTTP + WebSocket client for the AI chat agent (not entity-service)
│   │   ├── client.go            # Config/Client/do()/getJSON()/postJSON()
│   │   ├── types.go             # AI chat agent's wire-format structs
│   │   └── ws.go                # WSConfig/WSClient/StreamChat — proxies the upstream WebSocket
│   ├── productconsumption/       # OAuth2 HTTP client for the product-consumption service (not entity-service)
│   │   ├── client.go            # Config/Client/do()/postJSON()/patchJSON()
│   │   ├── types.go             # product-consumption service's wire-format structs
│   │   ├── subscription.go      # ProcessLicenseDownload — the deployment-license state machine
│   │   └── tracking.go          # ImportDeploymentUsage
│   ├── dto/                     # Portal-facing response shapes + Map* functions from entity types
│   │   ├── user.go
│   │   ├── account.go
│   │   ├── project.go
│   │   ├── project_stats.go
│   │   ├── case.go
│   │   ├── deployment.go
│   │   ├── deployed_product.go
│   │   ├── attachment.go
│   │   ├── product.go
│   │   ├── product_vulnerability.go
│   │   ├── catalog.go
│   │   ├── time_card.go
│   │   ├── comment.go
│   │   ├── ai_chat.go
│   │   ├── product_consumption.go
│   │   ├── change_request.go
│   │   ├── call_request.go
│   │   ├── global.go            # metadata, global search, vulnerability meta
│   │   ├── conversation.go      # conversation details + status-update translation
│   │   ├── case_feedback.go
│   │   ├── deployed_product_metrics.go # + date-range validation helpers
│   │   ├── escalation.go
│   │   ├── case_time_cards.go
│   │   ├── instance.go          # shared by all 15 instances fan-out routes
│   │   ├── registry.go
│   │   └── contacts.go
│   ├── middleware/
│   │   ├── auth.go              # JWT validation; injects UserInfo into context
│   │   ├── correlation.go       # X-CSM-Correlation-ID propagation + slog enrichment
│   │   ├── logger.go            # Per-request access log
│   │   └── security_headers.go  # X-Content-Type-Options, CSP, HSTS on every response
│   └── handler/
│       ├── response.go          # writeJSON/writeError/mapUpstreamError shared helpers
│       ├── users.go             # GET/PATCH /users/me
│       ├── accounts.go          # GET /accounts/{id}
│       ├── projects.go          # POST /projects/search, GET /projects/{id}
│       ├── project_stats.go     # project filters/features/dashboard-stats (composite, some graceful-degradation), case-grouped time-cards
│       ├── cases.go             # cases search/get/create/update/comment/activities/feedback/escalations, case-scoped attachment update
│       ├── deployments.go       # POST /projects/{id}/deployments/search, POST /projects/{id}/deployments, PATCH /projects/{projectId}/deployments/{id}, deployment-scoped attachment update
│       ├── deployed_products.go # deployed-product search/create/update (scoped under /deployments/{deploymentId}/products) + per-deployed-product metrics/usage-counts
│       ├── attachments.go       # attachment create/download/get/delete
│       ├── products.go          # GET /products, POST /products/search, POST /products/{id}/versions/search
│       ├── product_vulnerabilities.go # vulnerability search/get
│       ├── catalogs.go          # catalog search (scoped under /deployments/products/{deployedProductId}), catalog item variables
│       ├── time_cards.go        # POST /projects/{id}/time-cards/search
│       ├── comments.go          # generic comment create
│       ├── ai_chat.go           # case classification, recommendations, conversation create/get/update/search/messages/summary
│       ├── websocket.go         # GET /ws — real-time AI chat proxy
│       ├── product_consumption.go # deployment license provisioning, deployment usage import
│       ├── change_requests.go   # change-request create/search/get/update/approvals
│       ├── call_requests.go     # call-request create/search/update
│       ├── updates.go           # GET /updates/product-update-levels, POST /updates/levels/search
│       ├── global.go            # GET /metadata, POST /search, GET /products/vulnerabilities/meta
│       ├── instances.go         # 15-route instances fan-out (project/deployment/deployed-product scoped)
│       ├── registry.go          # registry-token create/search/delete/regenerate, project integration-users
│       └── contacts.go          # project contact/membership add/remove/list/update-role/validate
├── .choreo/component.yaml
├── openapi.yaml
├── .env.example
└── go.mod
```

## API Endpoints

- `GET /health` — liveness check, no auth
- `GET /users/me` — current user's profile; name/timezone/roles from entity-service (requires
  `DATA_SOURCE=servicenow`), phone number from SCIM
- `PATCH /users/me` — update phone number (SCIM) and/or timezone (entity-service); at least one required
- `GET /accounts/{id}` — get account by ID (normalizes entity-service's Postgres/ServiceNow shapes into one)
- `POST /projects/search` — search projects
- `GET /projects/{id}` — get project by ID
- `GET /projects/{id}/filters` — get filter-dropdown options for a project (case states, severities, issue types, etc.)
- `GET /projects/{id}/features` — get a project's feature-access flags
- `GET /projects/{id}/stats` — get a project's dashboard statistics (combines case/conversation/deployment/activity stats; partial failures are tolerated)
- `GET /projects/{id}/stats/cases` — get a project's case statistics, optionally filtered by `caseTypes`/`createdBy` query params
- `GET /projects/{id}/stats/conversations` — get a project's conversation statistics, optionally filtered by `createdBy`
- `GET /projects/{id}/stats/support` — get a project's combined support statistics (case + conversation; partial failures are tolerated)
- `GET /projects/{id}/stats/time-cards` — get a project's time-card statistics, optionally filtered by `startDate`/`endDate`
- `GET /projects/{id}/stats/change-requests` — get a project's change-request statistics
- `GET /projects/{id}/stats/usage` — get a project's usage statistics (deployment, deployed-product and instance counts) for the Usage Metrics page
- `POST /projects/{id}/cases/search` — search a project's cases
- `GET /cases/{id}` — get case by ID
- `POST /cases` — create a case
- `PATCH /cases/{id}` — update a case (restricted, customer-safe field subset — see CLAUDE.md)
- `POST /cases/{id}/comments` — add a comment to a case (always a plain customer comment)
- `GET /cases/{id}/feedback` — get feedback previously submitted for a case (ServiceNow data source only)
- `POST /cases/{id}/feedback` — submit feedback for a case (ServiceNow data source only)
- `GET /cases/{id}/attachments` — search a case's attachments, paginated via `limit`/`offset` query params
- `POST /cases/{id}/attachments` — add an attachment to a case
- `PATCH /cases/{caseId}/attachments/{attachmentId}` — update a case attachment's name (description not supported on this route — see CLAUDE.md)
- `POST /cases/{caseId}/escalations` — escalate or de-escalate a case (ServiceNow data source only)
- `POST /cases/{caseId}/escalations/search` — search a case's escalations (ServiceNow data source only)
- `POST /projects/{id}/deployments/search` — search a project's deployments
- `POST /projects/{id}/deployments` — create a deployment (ServiceNow data source only)
- `PATCH /projects/{projectId}/deployments/{id}` — update a deployment's name/type/description, or deactivate it
- `GET /deployments/{deploymentId}/attachments` — list a deployment's attachments (paged via `limit`/`offset`)
- `POST /deployments/{deploymentId}/attachments` — upload an attachment to a deployment
- `PATCH /deployments/{deploymentId}/attachments/{attachmentId}` — update a deployment attachment's name/description
- `POST /deployments/{deploymentId}/products/search` — search a deployment's deployed products
- `POST /deployments/{deploymentId}/products` — create a deployed product (ServiceNow data source only)
- `PATCH /deployments/{deploymentId}/products/{id}` — update a deployed product's cores/tps/description, or deactivate it (ServiceNow data source only)
- `POST /deployments/{deploymentId}/products/{productId}/metrics/search` — get core-count metrics for a deployed product over a date range (`productId` is the deployed product's own ID; ServiceNow data source only)
- `POST /deployments/{deploymentId}/products/{productId}/metrics/usage-counts/search` — get usage-count metrics for a deployed product over a date range (ServiceNow data source only)
- `POST /attachments` — create an attachment
- `GET /attachments/{id}` — get an attachment's metadata plus base64-encoded content
- `GET /attachments/{id}/content` — download an attachment's raw file content
- `DELETE /attachments/{id}` — delete an attachment
- `POST /cases/{id}/activities/search` — search a case's activity feed (comments, attachments, field changes)
- `POST /change-requests` — create a change request (ServiceNow data source only)
- `POST /projects/{id}/change-requests/search` — search a project's change requests (ServiceNow data source only)
- `GET /change-requests/{id}` — get change request by ID (ServiceNow data source only)
- `PATCH /change-requests/{id}` — update a change request (restricted, customer-safe field subset — see CLAUDE.md; ServiceNow data source only)
- `GET /change-requests/{id}/approvals` — get a change request's approval stages (ServiceNow data source only)
- `POST /change-requests/{id}/approvals/decision` — approve/reject the caller's own pending approval (ServiceNow data source only)
- `POST /cases/{caseId}/call-requests` — create a call request for a case (ServiceNow data source only)
- `POST /cases/{caseId}/call-requests/search` — search a case's call requests (ServiceNow data source only)
- `PATCH /cases/{caseId}/call-requests/{id}` — update a call request (restricted, excludes agent-only fields — see CLAUDE.md; ServiceNow data source only)
- `GET /products` — browse products, paginated via `class`/`offset`/`limit` query params (`class` is accepted but not forwarded — entity-service has no class filter)
- `POST /products/search` — search products
- `POST /products/{id}/versions/search` — search a product's versions
- `POST /products/vulnerabilities/search` — search product vulnerabilities
- `GET /products/vulnerabilities/{id}` — get a product vulnerability by ID
- `GET /products/vulnerabilities/meta` — get valid vulnerability severity choices
- `POST /deployments/products/{deployedProductId}/catalogs/search` — search service catalogs available for a deployed product
- `GET /catalogs/{catalogId}/items/{itemId}` — get a catalog item's form variables
- `POST /projects/{id}/time-cards/search` — search a project's time cards (read-only; ServiceNow data source only)
- `POST /projects/{id}/cases/time-cards/search` — search time cards rolled up by case for a project (ServiceNow data source only)
- `POST /comments` — add a comment to any reference entity (case, conversation, change_request, deployment, incident) — always a plain customer comment
- `GET /metadata` — get system-wide reference data (time zones, project types, feedback emoji choices)
- `POST /search` — global search across projects and cases
- `GET /conversations/{id}` — get a conversation's details
- `PATCH /conversations/{id}` — update a conversation's status to `closed`/`abandoned`/`converted`
- `POST /projects/{id}/conversations` — start a new conversation and get the AI agent's first response (creates the conversation, calls the AI agent, optionally fetches KB recommendations, persists the reply as a comment, and auto-resolves if the agent reports the issue solved)
- `POST /cases/classify` — classify a chat transcript into a case type/severity via the AI chat agent
- `POST /conversations/recommendations/search` — get KB article recommendations via the AI chat agent
- `POST /projects/{id}/conversations/search` — search a project's AI chat conversations
- `GET /conversations/{id}/messages` — get a conversation's messages (backed by generic comment search)
- `POST /projects/{projectId}/conversations/{conversationId}/messages` — send a follow-up message on an existing conversation
- `GET /projects/{id}/conversations/{conversationId}/summary` — get a conversation's summary via the AI chat agent
- `GET /ws?sessionId={projectId}` — WebSocket: real-time AI chat proxy for an existing conversation (starting a brand-new conversation over this connection isn't supported — use `POST /projects/{id}/conversations` first, see CLAUDE.md). **Served on `WS_PORT` (8081), not `PORT`, and authenticated via the `Sec-WebSocket-Protocol` header rather than `x-jwt-assertion` — see CLAUDE.md for why.**
- `POST /projects/{projectId}/deployments/{deploymentId}/license` — provision (or resume provisioning) and return a deployment's license via the product-consumption service
- `POST /deployment-usages` — import a deployment-usage zip file (raw binary body, `Content-Type: application/zip`) via the product-consumption service
- `GET /updates/product-update-levels` — list product update levels
- `POST /updates/levels/search` — search update descriptions between two update levels
- Instances fan-out (15 routes — `searchInstances`/`searchInstanceMetrics`/`searchInstanceUsage`/`searchInstanceMetricsStats`/`searchInstanceUsageStats`, each exposed project-scoped, deployment-scoped, and deployed-product-scoped; ServiceNow data source only):
  - `POST /projects/{id}/instances/search`, `POST /deployments/{id}/instances/search`, `POST /deployments/products/{id}/instances/search`
  - `POST /projects/{id}/instances/metrics/search`, `POST /deployments/{id}/instances/metrics/search`, `POST /deployments/products/{id}/instances/metrics/search`
  - `POST /projects/{id}/instances/usages/search`, `POST /deployments/{id}/instances/usages/search`, `POST /deployments/products/{id}/instances/usages/search`
  - `POST /projects/{id}/instances/stats/metrics/search`, `POST /deployments/{id}/instances/stats/metrics/search`, `POST /deployments/products/{id}/instances/stats/metrics/search`
  - `POST /projects/{id}/instances/stats/usages/search`, `POST /deployments/{id}/instances/stats/usages/search`, `POST /deployments/products/{id}/instances/stats/usages/search`
- `POST /projects/{id}/registry-tokens` — create a registry token (robot account); service tokens require admin, non-admins can only create tokens for themselves
- `POST /projects/{id}/registry-tokens/search` — search a project's registry tokens (non-admins only see their own)
- `DELETE /registry-tokens/{id}` — delete a registry token (authorization is derived from the token's own project, not the URL)
- `POST /registry-tokens/{id}/regenerate` — regenerate a registry token's secret
- `GET /projects/{id}/integration-users` — list a project's integration (service-account) users
- `GET /projects/{id}/contacts` — list a project's contacts
- `POST /projects/{id}/contacts` — add a contact to a project (`adminEmail` is always the caller)
- `DELETE /projects/{id}/contacts/{email}` — remove a contact from a project
- `PATCH /projects/{id}/contacts/{email}` — update a contact's membership roles
- `POST /projects/{id}/contacts/validate` — validate whether a contact can be onboarded before adding them (may return a conflict, a reactivatable existing contact, or a green light)

Full request/response schemas are documented in [openapi.yaml](./openapi.yaml). All entity-service
and SCIM response shapes are portal-owned DTOs (see `internal/dto`) — the raw upstream response is
never returned verbatim; see [CLAUDE.md](./CLAUDE.md#response-shaping) for what is deliberately
excluded from each and why. The `updates` client is the one exception — its own types are already
portal-shaped camelCase (translated from the upstream snake_case in `internal/updates/mapper.go`),
so handlers write its return values directly with no further DTO layer.

## Run Locally

```bash
go run ./cmd/server/main.go
```

With `AUTH_TOKEN_VALIDATOR_ENABLED=false` (the `.env.example` local default), pass any valid JWT as
the `x-jwt-assertion` header — its signature is not verified.

### Examples

```bash
JWT="<your-jwt-token>"

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/users/me

curl -X PATCH http://localhost:8080/users/me \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"timeZone":"Asia/Colombo"}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/accounts/<account-id>

curl -X POST http://localhost:8080/projects/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"searchQuery":"acme"}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/filters

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/features

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/stats

curl -H "x-jwt-assertion: $JWT" "http://localhost:8080/projects/<project-id>/stats/cases?caseTypes=default_case&createdBy=<user-id>"

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/stats/conversations

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/stats/support

curl -H "x-jwt-assertion: $JWT" "http://localhost:8080/projects/<project-id>/stats/time-cards?startDate=2026-07-01&endDate=2026-07-31"

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/stats/change-requests

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/stats/usage

curl -X POST http://localhost:8080/projects/<project-id>/cases/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"filters":{"searchQuery":"login error","statusIds":[1,10],"severityIds":[11]}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/cases/<case-id>

curl -X POST http://localhost:8080/cases \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"type":"case","projectId":"<project-id>","deploymentId":"<deployment-id>","subject":"Login error","description":"...","severity":"high","issueType":"question"}'

curl -X PATCH http://localhost:8080/cases/<case-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"state":"closed","closeNotes":"Resolved on our end, thanks!"}'

curl -X POST http://localhost:8080/cases/<case-id>/comments \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"content":"Any update on this?"}'

curl -X POST http://localhost:8080/projects/<project-id>/deployments/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -X POST http://localhost:8080/projects/<project-id>/deployments \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"name":"Production","deploymentTypeKey":6,"description":"Primary production"}'

curl -X POST http://localhost:8080/deployments/<deployment-id>/products/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -X POST http://localhost:8080/deployments/<deployment-id>/products \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"projectId":"<project-id>","productId":"<product-id>","versionId":"<version-id>"}'

curl -X PATCH http://localhost:8080/deployments/<deployment-id>/products/<deployed-product-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"cores":4}'

curl -X POST http://localhost:8080/attachments \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"referenceId":"<case-id>","referenceType":"case","name":"log.txt","type":"text/plain","file":"<base64>"}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/attachments/<attachment-id>/content -o downloaded-file

curl -X DELETE -H "x-jwt-assertion: $JWT" http://localhost:8080/attachments/<attachment-id>

curl -H "x-jwt-assertion: $JWT" "http://localhost:8080/cases/<case-id>/attachments?limit=10&offset=0"

curl -X POST http://localhost:8080/cases/<case-id>/activities/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":20,"offset":0}}'

curl -H "x-jwt-assertion: $JWT" "http://localhost:8080/products?class=product_model&offset=0&limit=10"

curl -X POST http://localhost:8080/products/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"searchQuery":"wso2am"}'

curl -X POST http://localhost:8080/products/<product-id>/versions/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -X POST http://localhost:8080/change-requests \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"subject":"Upgrade WSO2 API Manager to 4.3.0"}'

curl -X POST http://localhost:8080/projects/<project-id>/change-requests/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/change-requests/<change-request-id>

curl -X PATCH http://localhost:8080/change-requests/<change-request-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"isCustomerApproved":true}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/change-requests/<change-request-id>/approvals

curl -X POST http://localhost:8080/change-requests/<change-request-id>/approvals/decision \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"decision":"approved"}'

curl -X POST http://localhost:8080/cases/<case-id>/call-requests \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"reason":"Discuss workaround","utcTimes":["2026-08-05T10:00:00Z"],"durationInMinutes":30}'

curl -X POST http://localhost:8080/cases/<case-id>/call-requests/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -X PATCH http://localhost:8080/cases/<case-id>/call-requests/<call-request-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"stateKey":6,"cancellationReason":"No longer needed"}'

curl -X PATCH http://localhost:8080/projects/<project-id>/deployments/<deployment-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"name":"Production (EU)"}'

curl -X POST http://localhost:8080/products/vulnerabilities/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"filters":{"productName":"wso2am"}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/products/vulnerabilities/<vulnerability-id>

curl -X POST http://localhost:8080/deployments/products/<deployed-product-id>/catalogs/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/catalogs/<catalog-id>/items/<item-id>

curl -X POST http://localhost:8080/projects/<project-id>/time-cards/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -X POST http://localhost:8080/comments \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"referenceId":"<change-request-id>","referenceType":"change_request","content":"Any update?"}'

curl -X POST http://localhost:8080/cases/classify \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"chatHistory":"user: my API gateway is down\n","envProducts":{},"region":"EU","tier":"gold","projectTypeId":"<project-type-id>"}'

curl -X POST http://localhost:8080/conversations/recommendations/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"chatHistory":[{"role":"user","content":"my API gateway is down","timestamp":"2026-08-01T10:00:00Z"}],"conversationData":{"chatHistory":"user: my API gateway is down","envProducts":{},"region":"EU","tier":"gold"}}'

curl -X POST http://localhost:8080/projects/<project-id>/conversations/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"filters":{},"sortBy":{},"pagination":{"limit":10,"offset":0}}'

curl -H "x-jwt-assertion: $JWT" "http://localhost:8080/conversations/<conversation-id>/messages?limit=20&offset=0"

curl -X POST http://localhost:8080/projects/<project-id>/conversations/<conversation-id>/messages \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"message":"It is still down","region":"EU","tier":"gold"}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/conversations/<conversation-id>/summary

# WebSocket (real-time chat proxy) — resumes an existing conversation only, see CLAUDE.md.
# NOTE: this runs on WS_PORT (8081), not 8080, and does NOT use x-jwt-assertion —
# a browser can't set headers on a WebSocket handshake, so the token travels as the
# last Sec-WebSocket-Protocol value. websocat sends that via --protocol:
websocat "ws://localhost:8081/ws?sessionId=<project-id>" --protocol "cs-customer-portal,$JWT"
# A non-browser client may instead send the header directly:
websocat "ws://localhost:8081/ws?sessionId=<project-id>" -H "x-user-id-token: $JWT"
# then send: {"message":"still seeing the error","conversationId":"<conversation-id>"}

curl -X POST http://localhost:8080/projects/<project-id>/deployments/<deployment-id>/license \
  -H "x-jwt-assertion: $JWT"

curl -X POST http://localhost:8080/deployment-usages \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/zip" \
  --data-binary @deployment-usage.zip

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/updates/product-update-levels

curl -X POST http://localhost:8080/updates/levels/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"productName":"wso2am","productVersion":"4.2.0","startingUpdateLevel":1,"endingUpdateLevel":10}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/metadata

curl -X POST http://localhost:8080/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"filters":{"searchQuery":"acme"}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/conversations/<conversation-id>

curl -X PATCH http://localhost:8080/conversations/<conversation-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"status":"closed"}'

curl -X POST http://localhost:8080/projects/<project-id>/conversations \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"message":"My API gateway is down","region":"EU","tier":"gold"}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/cases/<case-id>/feedback

curl -X POST http://localhost:8080/cases/<case-id>/feedback \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"emojiId":"<emoji-id>","chipIds":["<chip-id>"],"additionalComment":"Resolved quickly, thanks!"}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/attachments/<attachment-id>

curl -X PATCH http://localhost:8080/deployments/<deployment-id>/attachments/<attachment-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"name":"updated-log.txt","description":"Renamed for clarity"}'

curl -X PATCH http://localhost:8080/cases/<case-id>/attachments/<attachment-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"name":"updated-log.txt"}'

curl -X POST http://localhost:8080/deployments/<deployment-id>/products/<deployed-product-id>/metrics/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"startDate":"2026-07-01","endDate":"2026-07-31"}'

curl -X POST http://localhost:8080/deployments/<deployment-id>/products/<deployed-product-id>/metrics/usage-counts/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"startDate":"2026-07-01","endDate":"2026-07-31"}'

curl -X POST http://localhost:8080/cases/<case-id>/escalations \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"action":"ESCALATE","reason":"No response in 48 hours"}'

curl -X POST http://localhost:8080/cases/<case-id>/escalations/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -X POST http://localhost:8080/projects/<project-id>/cases/time-cards/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -X POST http://localhost:8080/projects/<project-id>/instances/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -X POST http://localhost:8080/projects/<project-id>/instances/metrics/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"startDate":"2026-07-01","endDate":"2026-07-31"}'

curl -X POST http://localhost:8080/projects/<project-id>/registry-tokens \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"robotName":"ci-pipeline","tokenType":"User"}'

curl -X POST http://localhost:8080/projects/<project-id>/registry-tokens/search \
  -H "x-jwt-assertion: $JWT"

curl -X DELETE -H "x-jwt-assertion: $JWT" http://localhost:8080/registry-tokens/<token-id>

curl -X POST -H "x-jwt-assertion: $JWT" http://localhost:8080/registry-tokens/<token-id>/regenerate

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/integration-users

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/contacts

curl -X POST http://localhost:8080/projects/<project-id>/contacts \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"contactEmail":"jane@example.com","contactFirstName":"Jane","contactLastName":"Doe","isCsIntegrationUser":false,"isPortalUser":true}'

curl -X DELETE -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/contacts/jane@example.com

curl -X PATCH http://localhost:8080/projects/<project-id>/contacts/jane@example.com \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"isLead":true}'

curl -X POST http://localhost:8080/projects/<project-id>/contacts/validate \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"contactEmail":"jane@example.com"}'
```
