# CSM Integration Service

Go backend service exposing Account and Project reads and their Contacts
sub-resource, a subset of Case operations, opportunity/invoice reads, incident
create/search/update, IT-service search, alert-incident mappings, the
product-vulnerability sync, and the public cloud status dashboard reads, for
third-party (M2M) consumers.

## Quick Start

```bash
# from integrations/csm-integration-service
go run ./cmd/server/main.go
```

The server automatically loads `.env` from the working directory on startup (silently
ignored if absent).

Server starts at `http://localhost:8080`.

## Overview

- Default port: `8080`
- Runtime: Go `1.26+`
- Entry point: `cmd/server/main.go`
- Authentication and authorization:
  - Incoming requests: **authentication is the gateway's; authorization is per
    operation here.** This service is fronted by Choreo's API Manager gateway
    (subscription + M2M client-credentials app auth) — the app code performs no
    Bearer/JWT signature validation of its own. Unlike `apps/csm-portal/backend`
    (which authenticates its own end users), this service has no end-user identity
    to check. It does, however, require one OAuth2 scope per operation (for
    example `cases:write`, `contacts:read`, `vulnerabilities:sync`; the full list
    is in `openapi.yaml`'s security scheme and `cmd/server/routes.go`), read from
    the `scope` claim of the token the gateway forwards: no usable token is a
    401, a token without the operation's scope is a 403, and neither is forwarded
    upstream. `GET /health` is the only unscoped route. Set
    `REQUIRE_OPERATION_SCOPES=false` to switch the check off for local
    development only.
  - Outbound service calls: OAuth2 client credentials grant to the entity service
    (managed automatically) — always M2M, on every request, with no mechanism to
    carry an end-user identity. What that means per endpoint depends on the
    entity service's data source; `CLAUDE.md` is the authoritative breakdown.
    In short: `PATCH /projects/{id}` always 401s (kept for API-shape
    completeness); `PATCH /cases/{id}` succeeds for a state/severity/workState
    update on a Postgres data source and 401s on the other one; case comments
    and tags succeed on a Postgres data source using the configured trusted
    actor (`UMT_INTEGRATION_ACTOR_EMAIL`); the incident and IT-service
    operations succeed wherever the backing data source's machine credential
    is configured; the alert-incident mappings and the read-only searches work
    over M2M as long as this service's client id is allow-listed by the
    entity service.

## Prerequisites

- Go `1.26+` — [install](https://go.dev/doc/install)

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

Handler tests use a mock entity client (`internal/handler/helpers_test.go`) and a
shared `upstreamErrors` table covering every `mapUpstreamError` status-code mapping.
Entity client tests spin up real `httptest.Server`s to exercise the OAuth2
client-credentials flow, error-body truncation, and correlation ID forwarding.
Middleware tests cover header injection, ID generation/preservation, and the
scope guard's 401/403/pass decisions. `cmd/server/routes_test.go` drives every
route in the scope table through a real mux against a stub upstream, asserting
401 without a token, 403 with every other scope, and 2xx with the required one.
`internal/apierror` has a test pinning that `Error()` never includes the upstream body.

### Run tests before every push (recommended)

```bash
git config core.hooksPath .githooks
# or, from this directory:
make setup
```

## Security Scanning

```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec -fmt=text ./...
```

The scan should report **0 issues**. If a new finding appears, fix the root cause
before merging — do not suppress it without a code review.

## Configuration

Copy `.env.example` to `.env` and fill in the values:

### Entity service

| Variable | Description |
|---|---|
| `ENTITY_BASE_URL` | Base URL of the entity service |
| `ENTITY_TOKEN_URL` | OAuth2 token endpoint |
| `ENTITY_CLIENT_ID` | OAuth2 client ID |
| `ENTITY_CLIENT_SECRET` | OAuth2 client secret |
| `ENTITY_SCOPES` | Comma-separated OAuth2 scopes (optional) |
| `UMT_INTEGRATION_ACTOR_EMAIL` | **Required.** The trusted actor asserted on `POST /cases/{id}/comments` and `POST /cases/{id}/tags`. Must be a single bare e-mail address (the server exits at startup otherwise) and must be on the entity service's trusted-actor allowlist, or those writes are answered 403 |

### Server

| Variable | Description |
|---|---|
| `PORT` | Server listen port (default `8080`) |
| `REQUIRE_OPERATION_SCOPES` | Enforce the per-operation scope check on every route except `GET /health` (default `true`). Set to `false` only for local development against a client that forwards no token; the server logs a warning at startup when it is off |

## Project Structure

```text
csm-integration-service/
├── cmd/server/
│   ├── main.go                   # Entry point — config, middleware chain, server startup
│   └── routes.go                 # Route table: pattern → required scope → handler
├── internal/
│   ├── apierror/                 # Typed upstream error type (4xx/5xx passthrough)
│   ├── entity/
│   │   ├── client.go             # OAuth2 HTTP client for the entity service + health probe
│   │   └── entity.go             # Entity service operations
│   ├── middleware/
│   │   ├── correlation.go        # X-CSM-Correlation-ID validation/propagation + slog enrichment
│   │   ├── logger.go             # Per-request access log (health polls excluded)
│   │   ├── scopes.go             # Per-operation scope check on the forwarded token
│   │   └── security_headers.go   # Five security headers on every response
│   └── handler/
│       ├── response.go           # Shared writeError/writeJSON/mapUpstreamError + ErrMsg*
│       ├── body.go               # Shared request-body read/validate (size, JSON, empty-body policy)
│       ├── health.go             # GET /health — cached entity-service reachability
│       ├── accounts.go           # Account endpoints
│       ├── projects.go           # Project endpoints
│       ├── cases.go              # Case endpoints
│       ├── vulnerabilities.go    # Product-vulnerability sync endpoint
│       ├── opportunities.go      # Opportunity endpoints
│       ├── invoices.go           # Invoice endpoints
│       ├── project_opportunity_links.go  # Project-opportunity link search
│       ├── incidents.go          # Incident endpoints
│       ├── it_services.go        # IT-service search
│       ├── alert_incident_mappings.go    # Alert-incident mapping endpoints
│       └── cloud_status.go       # Public cloud status dashboard reads
├── .choreo/component.yaml
├── openapi.yaml
└── .env.example
```

## API Endpoints

The scope each route requires is in brackets.

- `GET /health` — entity-service reachability (200/503, cached 15 s); no token, no scope
- `GET /accounts/{id}` — get an account by ID [`accounts:read`]
- `POST /accounts/search` — search accounts [`accounts:read`]
- `POST /accounts/{id}/contacts/search` — search an account's contacts [`contacts:read`]
- `GET /projects/{id}` — get a project by ID [`projects:read`]
- `POST /projects/search` — search projects [`projects:read`]
- `POST /projects/{id}/contacts/search` — search a project's contacts [`contacts:read`]
- `PATCH /projects/{id}` — update project closure-state fields (ACP automation; currently always 401s, see Overview above) [`projects:write`]
- `POST /cases/search` — search cases [`cases:read`]
- `PATCH /cases/{id}` — update a case's state, severity, or workState (succeeds on a Postgres data source; other fields 400 there, and every field 401s on the other data source — see `CLAUDE.md`) [`cases:write`]
- `POST /cases/{id}/comments` — add a comment to a case as the configured trusted actor (succeeds on a Postgres data source; 401 on the other — see `CLAUDE.md`) [`cases:write`]
- `POST /cases/{id}/tags` — add a tag to a case as the configured trusted actor (same split as comments) [`cases:write`]
- `POST /incidents` — create an incident (401 possible where the backing data source's machine credential isn't configured — see `CLAUDE.md`) [`incidents:write`]
- `PATCH /incidents/{id}` — update an incident, e.g. push a work note (503 on a Postgres data source; otherwise the same conditional 401 as `POST /incidents`) [`incidents:write`]
- `POST /incidents/search` — search incidents (same conditional 401 as `POST /incidents`) [`incidents:read`]
- `POST /services/search` — search IT services (same conditional 401 as `POST /incidents`) [`services:read`]
- `POST /alert-incident-mappings` — create an alert-incident mapping [`alert-incident-mappings:write`]
- `POST /alert-incident-mappings/lookup` — look up alert-incident mappings [`alert-incident-mappings:read`]
- `POST /vulnerabilities/sync` — full-replace sync of product-vulnerability records (submit the complete current set on every call; an empty set is rejected) [`vulnerabilities:sync`]
- `GET /opportunities/{id}` — get an opportunity by ID [`opportunities:read`]
- `POST /opportunities/search` — search opportunities [`opportunities:read`]
- `GET /invoices/{id}` — get an invoice by ID [`invoices:read`]
- `POST /invoices/search` — search invoices [`invoices:read`]
- `POST /project-opportunity-links/search` — search project-opportunity links (no by-id fetch) [`opportunities:read`]
- `GET /cloud-status/monitors`, `/cloud-status/incidents`, `/cloud-status/incidents/{id}`, `/cloud-status/availabilities`, `/cloud-status/availability-history` — public status dashboard reads, each taking `?cloud=` [`cloud-status:read`]

All responses are raw JSON passthrough from the entity service — this service does not
reshape upstream response bodies, except the cloud status reads, which are wrapped in
the `{"result": {...}}` envelope the status dashboard expects.

## Run Locally

With `REQUIRE_OPERATION_SCOPES=false` (local only); otherwise each call also needs an
`x-jwt-assertion` header carrying a token with the route's scope.

```bash
curl -X POST http://localhost:8080/accounts/search -d '{}'
curl http://localhost:8080/accounts/<id>
curl -X POST http://localhost:8080/accounts/<id>/contacts/search -d '{}'
curl -X POST http://localhost:8080/projects/search -d '{}'
curl http://localhost:8080/projects/<id>
curl -X POST http://localhost:8080/projects/<id>/contacts/search -d '{}'
curl -X POST http://localhost:8080/vulnerabilities/sync -d '[{"wso2Id":"<id>"}]'   # an empty array is rejected: full-replace sync
curl -X PATCH http://localhost:8080/cases/<id> -d '{"state":"closed"}'
curl -X POST http://localhost:8080/cases/<id>/comments -d '{"type":"comment","content":"hi"}'
curl -X POST http://localhost:8080/opportunities/search -d '{}'
curl http://localhost:8080/opportunities/<id>
curl -X POST http://localhost:8080/invoices/search -d '{}'
curl http://localhost:8080/invoices/<id>
curl -X POST http://localhost:8080/project-opportunity-links/search -d '{}'
```
