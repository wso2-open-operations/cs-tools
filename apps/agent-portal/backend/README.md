# Agent Portal Backend

Go backend service for the Agent Portal application.

## Quick Start

```bash
# from apps/agent-portal/backend
export $(cat .env | xargs) && go run ./cmd/server
```

Backend starts at `http://localhost:8080`.

## Overview

- Default port: `8080`
- Runtime: Go `1.22+`
- Entry point: `cmd/server/main.go`
- Authentication: OAuth2 client credentials grant (token managed automatically)

## Prerequisites

- Go `1.22+` — [install](https://go.dev/doc/install)

## Configuration

Copy `.env` and fill in the values:

| Variable | Description |
|---|---|
| `ENTITY_BASE_URL` | Base URL of the entity service |
| `ENTITY_TOKEN_URL` | OAuth2 token endpoint |
| `ENTITY_CLIENT_ID` | OAuth2 client ID |
| `ENTITY_CLIENT_SECRET` | OAuth2 client secret |
| `ENTITY_SCOPES` | Comma-separated OAuth2 scopes (optional) |
| `PORT` | Server listen address (default `:8080`) |

## Project Structure

```text
backend/
├── cmd/server/main.go          # Entry point — routes + server startup
├── internal/
│   ├── entity/
│   │   ├── client.go           # OAuth2 HTTP client for the entity service
│   │   └── entity.go           # Entity service operations (cases, ...)
│   └── handler/
│       └── cases.go            # HTTP handlers (equivalent to service.bal)
├── .env                        # Local config (git-ignored)
└── go.mod
```

## API Endpoints

### Cases

- `POST /cases` — Create a case
- `POST /cases/search` — Search cases
- `GET /cases/{id}` — Get case by ID

## Run Locally

```bash
export $(cat .env | xargs) && go run ./cmd/server
```

### Examples

```bash
# Create a case
curl -X POST http://localhost:8080/cases \
  -H "Content-Type: application/json" \
  -d '{"type":"DEFAULT_CASE","projectId":"<project-id>","deploymentId":"<deployment-id>"}'

# Search cases
curl -X POST http://localhost:8080/cases/search \
  -H "Content-Type: application/json" \
  -d '{"filters":{"searchQuery":"login error"},"pagination":{"limit":10,"offset":0}}'

# Get a case
curl http://localhost:8080/cases/<case-id>
```
