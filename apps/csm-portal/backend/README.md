# CSM Portal Backend

Go backend service for the CSM Portal application.

## Quick Start

```bash
# from apps/csm-portal/backend
go run ./cmd/server/main.go
```

The server automatically loads `.env` from the working directory on startup (silently ignored if absent).

Backend starts at `http://localhost:8080`.

## Overview

- Default port: `8080`
- Runtime: Go `1.26+`
- Entry point: `cmd/server/main.go`
- Authentication:
  - Incoming requests: JWT Bearer token (validated by Choreo gateway + JWKS endpoint); pass as `x-jwt-assertion` header when testing locally
  - Outbound service calls: OAuth2 client credentials grant (managed automatically)

## Prerequisites

- Go `1.26+` — [install](https://go.dev/doc/install)

## Testing

Tests are pure unit tests — no running services or environment variables needed.

```bash
# Run all tests (from apps/csm-portal/backend)
go test ./...

# Run with verbose output (shows every subtest)
go test -v ./...

# Run with the race detector
go test -race ./...

# Run a specific package
go test ./internal/handler/...
go test ./internal/middleware/...

# Run a specific test or subtest
go test -v -run TestSearchAccounts ./internal/handler/...
go test -v -run "TestSearchAccounts/upstream_errors" ./internal/handler/...

# Check test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Or use `make`:

```bash
make test    # vet + test
make build   # vet + test + compile
```

### Run tests before every push (recommended)

Set up the shared git hook once from the **repo root**:

```bash
git config core.hooksPath .githooks
```

Or from the backend directory:

```bash
make setup
```

After this, `git push` automatically runs `go test ./...` whenever backend files are in the push. If any test fails, the push is aborted.

To skip the hook in exceptional cases:

```bash
git push --no-verify
```

## Security Scanning

Run [gosec](https://github.com/securego/gosec) to check for common security issues:

```bash
# Install gosec (once)
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run from apps/csm-portal/backend
gosec -fmt=text ./...
```

The scan should report **0 issues**. If a new finding appears, fix the root cause before merging — do not suppress it without a code review.

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

### SCIM operations service

| Variable | Description |
|---|---|
| `SCIM_BASE_URL` | Base URL of the SCIM operations service |
| `SCIM_TOKEN_URL` | OAuth2 token endpoint |
| `SCIM_CLIENT_ID` | OAuth2 client ID |
| `SCIM_CLIENT_SECRET` | OAuth2 client secret |
| `SCIM_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Server

| Variable | Description |
|---|---|
| `PORT` | Server listen address (default `:8080`) |

## Project Structure

```text
backend/
├── cmd/server/main.go          # Entry point — routes + server startup
├── internal/
│   ├── apierror/               # Typed upstream error types (4xx/5xx passthrough)
│   ├── entity/
│   │   ├── client.go           # OAuth2 HTTP client for the entity service
│   │   └── entity.go           # Entity service operations (cases, accounts, projects, ...)
│   ├── scim/
│   │   └── client.go           # OAuth2 HTTP client for the SCIM operations service
│   ├── updates/
│   │   ├── client.go           # OAuth2 HTTP client for the updates service
│   │   └── updates.go          # Updates service operations
│   ├── middleware/
│   │   ├── auth.go             # JWT validation; injects UserInfo into context
│   │   ├── correlation.go      # X-CSM-Correlation-ID propagation + slog enrichment
│   │   ├── logger.go           # Per-request access log
│   │   └── security_headers.go # X-Content-Type-Options, CSP, HSTS on every response
│   └── handler/
│       ├── cases.go            # HTTP handlers for case endpoints
│       ├── state.go            # Case state machine (nextStates, isValidStateTransition)
│       ├── catalogs.go                   # HTTP handlers for catalog endpoints (ServiceNow only)
│       ├── change_requests.go            # HTTP handlers for change-request endpoints
│       ├── product_vulnerabilities.go    # HTTP handlers for product vulnerability endpoints (ServiceNow only)
│       ├── accounts.go                   # HTTP handlers for account endpoints
│       ├── deployments.go                # HTTP handlers for deployment endpoints
│       ├── products.go                   # HTTP handlers for product endpoints
│       ├── projects.go                   # HTTP handlers for project endpoints
│       ├── updates.go                    # HTTP handlers for updates endpoints
│       └── users.go                      # HTTP handlers for user endpoints
├── .env                        # Local config (git-ignored)
└── go.mod
```

## API Endpoints

### Cases

- `POST /cases` — Create a case (`type`: `case`; `service_request` and `security_report_analysis` are ServiceNow data source only)
- `GET /cases/{id}` — Get case by ID
- `PATCH /cases/{id}` — Update a case (state, severity, workState, watchList, or assigneeEmail)
- `POST /cases/search` — Search cases; filters include `searchQuery`, `types`, `states`, `severities`, `workStates` (`ongoing`/`paused`), `assignedUserIds`, `projectIds`, `deploymentIds`, `engagementTypes`, `issueTypes`, date ranges, `createdBy`, `createdByMe`
- `POST /cases/{id}/comments` — Create a comment on a case
- `POST /cases/{id}/comments/search` — Search comments on a case
- `POST /attachments` — Upload an attachment (`referenceId`, `referenceType`, `name`, `type`, `file` in body)
- `POST /attachments/search` — Search attachments (`referenceId`, `referenceType` in body)
- `GET /attachments/{id}/content` — Download an attachment
- `DELETE /attachments/{id}` — Delete an attachment (ServiceNow only)
- `POST /cases/{id}/call-requests` — Create a call request for a case (ServiceNow only)
- `POST /cases/{id}/call-requests/search` — Search call requests for a case (ServiceNow only)
- `PATCH /cases/{id}/call-requests/{callRequestId}` — Update a call request (ServiceNow only)

### Users

- `GET /users/me` — Get current user profile (`id`, `email`, `firstName`, `lastName`, `timeZone`, `roles` from entity service; `phoneNumber` from SCIM)
- `PATCH /users/me` — Update current user profile (`phoneNumber` via SCIM, `timeZone` via entity service)
- `POST /users/search` — Search users; optional `filters` (`searchQuery`, `roles`, `userNames`, `emails`, `active`) and `sortBy` (`field`, `order`); response shape depends on data source (`User` for postgres, `SNUser` for ServiceNow)

### Accounts

- `GET /accounts/{id}` — Get account by ID
- `POST /accounts/search` — Search accounts

### Projects

- `GET /projects/{id}` — Get project by ID
- `POST /projects/search` — Search projects

### Products

- `POST /products/search` — Search products
- `POST /products/{id}/versions/search` — Search product versions

### Deployments

- `POST /deployments` — Create a deployment (ServiceNow data source only)
- `PATCH /deployments/{id}` — Update a deployment (name, type, description, or deactivate; ServiceNow data source only)
- `POST /deployments/search` — Search deployments
- `POST /deployments/{id}/products` — Create a deployed product under a deployment (ServiceNow data source only)
- `PATCH /deployments/{deploymentId}/products/{productId}` — Update a deployed product (cores, tps, description, or deactivate; ServiceNow data source only)
- `POST /deployments/{id}/products/search` — Search deployed products

### Change Requests

- `GET /change-requests/{id}` — Get change request by ID (ServiceNow data source only)
- `PATCH /change-requests/{id}` — Update a change request (`plannedStartOn`, `isCustomerApproved`, `isCustomerReviewed`; ServiceNow data source only)
- `POST /change-requests/search` — Search change requests (ServiceNow data source only)

### Time Cards

- `POST /time-cards/search` — Search time cards; optional `pagination` and `filters` (`projectIds`, `startDate`, `endDate`, `states`) (ServiceNow data source only)

### Catalogs

- `POST /catalogs/search` — Search service catalogs by deployed product (ServiceNow only)
- `GET /catalogs/{catalogId}/items/{catalogItemId}/variables` — Get catalog item variables (ServiceNow only)

### Product Vulnerabilities

- `POST /products/vulnerabilities/search` — Search product vulnerabilities; requires `pagination`, optional `filters` (`priority`, `searchQuery`, `productName`, `productVersion`) (ServiceNow data source only)
- `GET /products/vulnerabilities/{id}` — Get product vulnerability by ID (ServiceNow data source only)

### Updates

- `GET /updates/product-update-levels` — Get product update levels
- `POST /updates/levels/search` — Search updates between update levels

## Run Locally

```bash
# from apps/csm-portal/backend
go run ./cmd/server/main.go
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

# Get current user profile
curl -H "x-jwt-assertion: $JWT" http://localhost:8080/users/me

# Update current user's phone number
curl -X PATCH http://localhost:8080/users/me \
  -H "x-jwt-assertion: $JWT" \
  -H "Content-Type: application/json" \
  -d '{"phoneNumber":"+94771234567"}'

# Get product update levels
curl -H "x-jwt-assertion: $JWT" http://localhost:8080/updates/product-update-levels

# Search updates between update levels
curl -X POST http://localhost:8080/updates/levels/search \
  -H "x-jwt-assertion: $JWT" \
  -H "Content-Type: application/json" \
  -d '{"productName":"wso2am","productVersion":"4.2.0","startingUpdateLevel":1,"endingUpdateLevel":10}'
```
