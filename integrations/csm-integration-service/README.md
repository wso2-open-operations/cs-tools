# CSM Integration Service

Go backend service exposing Project search, Account search, and their Contacts
sub-resource, for third-party (M2M) consumers.

## Quick Start

```bash
# from operations/csm-integration-service
go run ./cmd/server/main.go
```

The server automatically loads `.env` from the working directory on startup (silently
ignored if absent).

Server starts at `http://localhost:8080`.

## Overview

- Default port: `8080`
- Runtime: Go `1.26+`
- Entry point: `cmd/server/main.go`
- Authentication:
  - Incoming requests: **none at the app layer.** This service is fronted by Choreo's
    API Manager gateway (subscription + M2M client-credentials app auth) — the app
    code performs no Bearer/JWT validation of its own. Unlike `apps/csm-portal/backend`
    (which authenticates its own end users), this service has no end-user identity to
    check.
  - Outbound service calls: OAuth2 client credentials grant to the entity service
    (managed automatically) — always M2M, on every request, with no mechanism to
    carry an end-user identity. entity-service's ServiceNow-backed operations
    require a forwarded end-user identity token and will always reject a request
    from this service with 401 — this service can only ever serve entity-service
    data that doesn't require one (Postgres-backed operations). `PATCH
    /projects/{id}` is a known, deliberate exception: it's kept for the Account
    Closure Process (ACP) automation's API shape, but currently always 401s —
    see `CLAUDE.md` before adding any other endpoint that targets a
    ServiceNow-backed operation.

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
Middleware tests cover header injection and ID generation/preservation.
`cmd/server` (wiring only) and `internal/apierror` (a two-line `Error()` method)
have no dedicated tests, matching the same judgment call `apps/csm-portal/backend`
makes for its own equivalents.

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

### Server

| Variable | Description |
|---|---|
| `PORT` | Server listen port (default `8080`) |

## Project Structure

```text
csm-integration-service/
├── cmd/server/main.go          # Entry point — routes + server startup
├── internal/
│   ├── apierror/                 # Typed upstream error type (4xx/5xx passthrough)
│   ├── entity/
│   │   ├── client.go             # OAuth2 HTTP client for the entity service
│   │   └── entity.go             # Entity service operations (accounts, projects, contacts)
│   ├── middleware/
│   │   ├── correlation.go        # X-CSM-Correlation-ID propagation + slog enrichment
│   │   ├── logger.go             # Per-request access log
│   │   └── security_headers.go   # X-Content-Type-Options, CSP, HSTS on every response
│   └── handler/
│       ├── response.go           # Shared writeError/writeJSON/mapUpstreamError + ErrMsg*
│       ├── accounts.go           # HTTP handlers for account endpoints
│       └── projects.go           # HTTP handlers for project endpoints
├── .choreo/component.yaml
├── openapi.yaml
└── .env.example
```

## API Endpoints

- `GET /health` — health check, no auth
- `GET /accounts/{id}` — get an account by ID
- `POST /accounts/search` — search accounts
- `POST /accounts/{id}/contacts/search` — search an account's contacts
- `GET /projects/{id}` — get a project by ID
- `POST /projects/search` — search projects
- `POST /projects/{id}/contacts/search` — search a project's contacts
- `PATCH /projects/{id}` — update project closure-state fields (ACP automation; currently always 401s, see Overview above)

All responses are raw JSON passthrough from the entity service — this service does not
reshape upstream response bodies.

## Run Locally

```bash
curl -X POST http://localhost:8080/accounts/search -d '{}'
curl http://localhost:8080/accounts/<id>
curl -X POST http://localhost:8080/accounts/<id>/contacts/search -d '{}'
curl -X POST http://localhost:8080/projects/search -d '{}'
curl http://localhost:8080/projects/<id>
curl -X POST http://localhost:8080/projects/<id>/contacts/search -d '{}'
```
