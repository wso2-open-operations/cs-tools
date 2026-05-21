# CSM Portal Backend

Go backend service for the CSM Portal application.

## Quick Start

```bash
# from apps/csm-portal/backend
set -a && source .env && set +a && go run ./cmd/server
```

Backend starts at `http://localhost:8080`.

## Overview

- Default port: `8080`
- Runtime: Go `1.22+`
- Entry point: `cmd/server/main.go`
- Authentication:
  - Incoming requests: JWT Bearer token (validated by Choreo gateway + JWKS endpoint); pass as `x-jwt-assertion` header when testing locally
  - Outbound service calls: OAuth2 client credentials grant (managed automatically)

## Prerequisites

- Go `1.22+` — [install](https://go.dev/doc/install)

## Configuration

Copy `.env` and fill in the values:

### Entity service

| Variable | Description |
|---|---|
| `ENTITY_BASE_URL` | Base URL of the entity service |
| `ENTITY_TOKEN_URL` | OAuth2 token endpoint |
| `ENTITY_CLIENT_ID` | OAuth2 client ID |
| `ENTITY_CLIENT_SECRET` | OAuth2 client secret |
| `ENTITY_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Updates service

| Variable | Description |
|---|---|
| `UPDATES_BASE_URL` | Base URL of the updates service |
| `UPDATES_TOKEN_URL` | OAuth2 token endpoint |
| `UPDATES_CLIENT_ID` | OAuth2 client ID |
| `UPDATES_CLIENT_SECRET` | OAuth2 client secret |
| `UPDATES_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Server

| Variable | Description |
|---|---|
| `PORT` | Server listen address (default `:8080`) |

## Project Structure

```text
backend/
├── cmd/server/main.go          # Entry point — routes + server startup
├── internal/
│   ├── entity/
│   │   ├── client.go           # OAuth2 HTTP client for the entity service
│   │   └── entity.go           # Entity service operations (cases, ...)
│   ├── updates/
│   │   ├── client.go           # OAuth2 HTTP client for the updates service
│   │   └── updates.go          # Updates service operations
│   └── handler/
│       ├── cases.go            # HTTP handlers for case endpoints
│       └── updates.go          # HTTP handlers for updates endpoints
├── .env                        # Local config (git-ignored)
└── go.mod
```

## API Endpoints

### Cases

- `POST /cases` — Create a case
- `POST /cases/search` — Search cases
- `GET /cases/{id}` — Get case by ID

### Updates

- `GET /updates/recommended-update-levels` — Get recommended update levels (user email sourced from JWT)
- `GET /updates/product-update-levels` — Get product update levels
- `POST /updates/levels/search` — Search updates between update levels

## Run Locally

```bash
# from apps/csm-portal/backend
set -a && source .env && set +a && go run ./cmd/server
```

When `AUTH_TOKEN_VALIDATOR_ENABLED=false` (default for local), pass any valid JWT as the `x-jwt-assertion` header. In production, the Choreo gateway validates the Bearer token and injects this header automatically.

### Examples

```bash
JWT="<your-jwt-token>"

# Create a case
curl -X POST http://localhost:8080/cases \
  -H "x-jwt-assertion: $JWT" \
  -H "Content-Type: application/json" \
  -d '{"type":"DEFAULT_CASE","projectId":"<project-id>","deploymentId":"<deployment-id>"}'

# Search cases
curl -X POST http://localhost:8080/cases/search \
  -H "x-jwt-assertion: $JWT" \
  -H "Content-Type: application/json" \
  -d '{"filters":{"searchQuery":"login error"},"pagination":{"limit":10,"offset":0}}'

# Get a case
curl -H "x-jwt-assertion: $JWT" http://localhost:8080/cases/<case-id>

# Get recommended update levels (user email is read from the JWT)
curl -H "x-jwt-assertion: $JWT" http://localhost:8080/updates/recommended-update-levels

# Get product update levels
curl -H "x-jwt-assertion: $JWT" http://localhost:8080/updates/product-update-levels

# Search updates between update levels
curl -X POST http://localhost:8080/updates/levels/search \
  -H "x-jwt-assertion: $JWT" \
  -H "Content-Type: application/json" \
  -d '{"productName":"wso2am","productVersion":"4.2.0","startingUpdateLevel":1,"endingUpdateLevel":10}'
```
