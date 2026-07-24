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

### Shared OAuth2 client credentials

Every upstream service client (customer entity, engineering entity, updates, SCIM, and future notification channels) authenticates as the same OAuth2 client-credentials app — only each service's base URL and scopes differ, so the credentials are configured once and reused.

| Variable | Description |
|---|---|
| `OAUTH2_CLIENT_ID` | OAuth2 client ID, shared by every upstream service client |
| `OAUTH2_CLIENT_SECRET` | OAuth2 client secret, shared by every upstream service client |
| `OAUTH2_TOKEN_URL` | OAuth2 token endpoint, shared by every upstream service client |

### Customer entity service

Backs `entity.CustomerEntityClient` (this repo's entity-service; cases, accounts, projects, products, etc.) — uses the shared OAuth2 credentials above.

| Variable | Description |
|---|---|
| `CUSTOMER_ENTITY_BASE_URL` | Base URL of the customer entity service |
| `CUSTOMER_ENTITY_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Engineering entity service (not yet wired in)

Backs `entity.EngineeringEntityClient.CreateGitIssue` (wso2-enterprise/digiops-engineering) but is not constructed in `cmd/server/main.go` — no handler calls it yet. These variables are not read by any code today. It uses the same shared OAuth2 credentials above (same `OAUTH2_CLIENT_ID`/`_CLIENT_SECRET`/`_TOKEN_URL`) — only its base URL and scopes are its own.

| Variable | Description |
|---|---|
| `ENGINEERING_ENTITY_BASE_URL` | Base URL of the engineering entity service |
| `ENGINEERING_ENTITY_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Updates service

| Variable | Description |
|---|---|
| `UPDATES_BASE_URL` | Base URL of the updates service |
| `UPDATES_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### SCIM operations service

| Variable | Description |
|---|---|
| `SCIM_BASE_URL` | Base URL of the SCIM operations service |
| `SCIM_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Notifications — email channel (not yet wired in)

`internal/notifications` (`EmailClient.SendEmail`) is ready to use but is not constructed in `cmd/server/main.go` — no handler calls it yet. These variables are not read by any code today; they're documented here for when the first caller is added, which should reuse the shared `OAUTH2_*` credentials above rather than adding its own. Each notification channel gets its own `NOTIFICATIONS_<CHANNEL>_*` prefix for its channel-specific settings — SMS/Twilio will follow this same convention once added.

| Variable | Description |
|---|---|
| `NOTIFICATIONS_EMAIL_BASE_URL` | Base URL of the email notification service |
| `NOTIFICATIONS_EMAIL_SCOPES` | Comma-separated OAuth2 scopes (optional) |
| `NOTIFICATIONS_EMAIL_FROM_ADDRESS` | Fixed "From" address used for every outgoing email |

### Notifications — Google Chat channel

`internal/notifications` (`GoogleChatClient.SendIncidentAlert`) posts a card message — title, short description, and an "Open in CSM Portal" button — to a Google Chat space via an incoming webhook. There's one space per product (each WSO2 product has its own space), so the client is configured with a list of `{product, webhookUrl}` pairs and routes each alert to the space matching the case's product (case- and whitespace-insensitive match; an unconfigured product returns an error rather than falling back). Unlike every other upstream client it does not use the shared `OAUTH2_*` credentials; a webhook URL is the only credential needed per space (Space settings > Apps & integrations > Webhooks). It's called from `POST /notifications/google-chat/alerts` (see [API Endpoints](#notifications) below), which today is triggered manually rather than from real case/incident creation.

| Variable | Description |
|---|---|
| `NOTIFICATIONS_GOOGLE_CHAT_SPACES` | JSON array of `{"product","webhookUrl"}` objects, one per Google Chat space — e.g. `[{"product":"api-manager","webhookUrl":"https://chat.googleapis.com/..."}]`. Malformed JSON is logged and treated as no spaces configured (does not fail startup) |
| `CSM_PORTAL_WEB_BASE_URL` | Base URL of the CSM portal webapp, used to build the "Open in CSM Portal" link at `/operations/incidents/{caseId}` (e.g. `http://localhost:3001` for local dev) |

### Auth

| Variable | Description |
|---|---|
| `AUTH_JWKS_ENDPOINT` | JWKS endpoint used to verify JWT signatures |
| `AUTH_ISSUER` | Expected `iss` claim value |
| `AUTH_AUDIENCE` | Comma-separated accepted `aud` values; token passes if any listed value is present in its `aud` claim |
| `AUTH_TOKEN_VALIDATOR_ENABLED` | Set to `false` for local development to skip signature verification (default `true`) |

### Server

| Variable | Description |
|---|---|
| `PORT` | Server listen port — a plain number, not an address (default `8080`) |

## Project Structure

```text
backend/
├── cmd/server/main.go          # Entry point — routes + server startup
├── internal/
│   ├── apierror/               # Typed upstream error types (4xx/5xx passthrough)
│   ├── entity/
│   │   ├── doc.go               # Package overview — one config/client pair per entity service
│   │   ├── customer_client.go   # OAuth2 HTTP client for the customer entity service (this repo's entity-service)
│   │   ├── customer.go          # CustomerEntityClient operations (cases, accounts, projects, ...)
│   │   └── engineering.go       # EngineeringEntityClient — CreateGitIssue (not yet wired into main.go — no caller)
│   ├── scim/
│   │   └── client.go           # OAuth2 HTTP client for the SCIM operations service
│   ├── updates/
│   │   ├── client.go           # OAuth2 HTTP client for the updates service
│   │   └── updates.go          # Updates service operations
│   ├── notifications/
│   │   ├── doc.go               # Package overview — one config/client pair per channel
│   │   ├── email.go             # EmailConfig/EmailClient/SendEmail (not yet wired into main.go — no caller)
│   │   └── googlechat.go        # GoogleChatConfig/GoogleChatClient/SendIncidentAlert (per-product webhook routing)
│   ├── middleware/
│   │   ├── auth.go             # JWT validation; injects UserInfo into context
│   │   ├── correlation.go      # X-CSM-Correlation-ID propagation + slog enrichment
│   │   ├── logger.go           # Per-request access log
│   │   └── security_headers.go # X-Content-Type-Options, CSP, HSTS on every response
│   └── handler/
│       ├── cases.go            # HTTP handlers for case endpoints
│       ├── state.go            # Case state machine (nextStates, isValidStateTransition, canCreateRelatedCase)
│       ├── catalogs.go                   # HTTP handlers for catalog endpoints (ServiceNow only)
│       ├── change_requests.go            # HTTP handlers for change-request endpoints
│       ├── product_vulnerabilities.go    # HTTP handlers for product vulnerability endpoints (ServiceNow only)
│       ├── accounts.go                   # HTTP handlers for account endpoints
│       ├── deployments.go                # HTTP handlers for deployment endpoints
│       ├── products.go                   # HTTP handlers for product endpoints
│       ├── projects.go                   # HTTP handlers for project endpoints
│       ├── incidents.go                  # HTTP handlers for incident endpoints (ServiceNow only)
│       ├── problems.go                   # HTTP handlers for problem endpoints (ServiceNow only)
│       ├── notifications.go              # HTTP handlers for notification channels (Google Chat alert endpoint)
│       ├── updates.go                    # HTTP handlers for updates endpoints
│       └── users.go                      # HTTP handlers for user endpoints
├── .env                        # Local config (git-ignored)
└── go.mod
```

## API Endpoints

### Cases

- `POST /cases` — Create a case (`type`: `case`; `service_request` and `security_report_analysis` are ServiceNow data source only)
- `GET /cases/{id}` — Get case by ID
- `PATCH /cases/{id}` — Update a case (state, severity, workState, watchList, or assigneeEmail); optional `resolutionCode`, `cause`, `closeNotes` accepted alongside `state: closed` or `state: solution_proposed`
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
- `POST /cases/{id}/github-issues` — Create a GitHub issue from a case; `reason` selects target repo (`default`/`migration`/`rd_ticket`; ServiceNow only)

### Users

- `GET /users/me` — Get current user profile (`id`, `email`, `firstName`, `lastName`, `timeZone`, `roles` from entity service; `phoneNumber` from SCIM)
- `PATCH /users/me` — Update current user profile (`phoneNumber` via SCIM, `timeZone` via entity service)
- `POST /users/search` — Search users; optional `filters` (`searchQuery`, `roles`, `userNames`, `emails`, `active`) and `sortBy` (`field`, `order`); response shape depends on data source (`User` for postgres, `SNUser` for ServiceNow)

### Accounts

- `GET /accounts/{id}` — Get account by ID; response takes one of two shapes: `Account`, or `AccountDetail` (`supportTier` as `{id, label}`, `owner`/`technicalOwner` as `{id, name}`)
- `POST /accounts/search` — Search accounts; optional `filters` (`searchQuery`, `active`, `pod`, `classification`); response takes one of two shapes: `AccountSearchResponse`, or `AccountViewSearchResponse` (`supportTier` as a label string, `owner`/`technicalOwner` as `{id, name}`)

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

- `POST /change-requests` — Create a change request (ServiceNow data source only)
- `GET /change-requests/{id}` — Get change request by ID (ServiceNow data source only)
- `PATCH /change-requests/{id}` — Update a change request; all fields optional but at least one required (`title`, `description`, `projectId`, `caseId`, `deploymentId`, `deployedProductId`, `assignedEngineerId`, `assignedTeamId`, `plannedStartOn`/`plannedEndOn` (format `YYYY-MM-DD HH:MM:SS`), `impact`, `state`, `type`, `justification`, `impactDescription`, `serviceOutage`, `communicationPlan`, `rollbackPlan`, `testPlan`); returns `{message, changeRequest}` with the full updated change request (ServiceNow data source only)
- `POST /change-requests/search` — Search change requests (ServiceNow data source only)

### CMDB

- `POST /services/search` — Search IT services (ServiceNow data source only)
- `POST /service-offerings/search` — Search service offerings (ServiceNow data source only)
- `POST /groups/search` — Search assignment groups (ServiceNow data source only)
- `POST /configuration-items/search` — Search configuration items (ServiceNow data source only)

### Time Cards

- `POST /time-cards/search` — Search time cards; optional `pagination` and `filters` (`projectIds`, `startDate`, `endDate`, `states`) (ServiceNow data source only)

### Catalogs

- `POST /catalogs/search` — Search service catalogs by deployed product (ServiceNow only)
- `GET /catalogs/{catalogId}/items/{catalogItemId}/variables` — Get catalog item variables (ServiceNow only)

### Product Vulnerabilities

- `POST /products/vulnerabilities/search` — Search product vulnerabilities; requires `pagination`, optional `filters` (`priority`, `searchQuery`, `productName`, `productVersion`) (ServiceNow data source only)
- `GET /products/vulnerabilities/{id}` — Get product vulnerability by ID (ServiceNow data source only)

### Conversations

- `GET /conversations/{id}/messages` — Get paginated messages for a conversation; optional query params `limit` (1–100, default 20) and `offset` (default 0) (ServiceNow data source only)
- `POST /conversations/search` — Search conversations; optional `filters` (`projectIds`, `states` (`ACTIVE`/`RESOLVED`), `searchQuery`, `createdByMe`) and `sortBy` (`field`: `createdOn`/`updatedOn`, `order`) (ServiceNow data source only)

### Updates

- `GET /updates/product-update-levels` — Get product update levels
- `POST /updates/levels/search` — Search updates between update levels

### Incidents

- `POST /incidents/search` — Search incidents; optional `filters` (`searchQuery`, `priorities`, `parentIds`) and `sortBy` (`field`: `createdOn`/`updatedOn`/`openedOn`, `order`) (ServiceNow data source only)
- `POST /incidents` — Create an incident (`callerId`, `category`, `serviceId`, `impact`, `urgency`, `subject` required; `subcategory`, `serviceOfferingId`, `configurationItemId`, `contactType`, `assignmentGroupId`, `assignedEngineerId`, `watchList`, `additionalComments`, `workNotes`, `parentId`, `parentIncidentId`, `changeRequestId`, `problemId`, `causedById` optional) (ServiceNow data source only)
- `GET /incidents/{id}` — Get full incident detail by ID (ServiceNow data source only)
- `PATCH /incidents/{id}` — Partially update an incident; all fields optional but at least one required (`subject`, `priority`, `state`, `category`/`subcategory`, `contactType`, `impact`/`urgency`, `resolutionCode`/`resolutionNotes`/`incidentReport`, `parentId`/`parentIncidentId`/`assignmentGroupId`/`assignedEngineerId`/`serviceId`/`serviceOfferingId`/`configurationItemId`/`changeRequestId`/`problemId`/`causedById`/`resolvedById`, `additionalComments`, `workNotes`, `watchList`); set a reference field to `null` to clear it (ServiceNow data source only)

### Problems

- `POST /problems/search` — Search problems; optional `filters` (`searchQuery`) (ServiceNow data source only)

### Notifications

- `POST /notifications/google-chat/alerts` — Send an incident alert card message to the Google Chat space configured for `product`; body requires `product`, `title`, `shortDescription`, `caseId`. Triggered manually today, pending integration into real case/incident creation.

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

# Search incidents
curl -X POST http://localhost:8080/incidents/search \
  -H "x-jwt-assertion: $JWT" \
  -H "Content-Type: application/json" \
  -d '{"filters":{"searchQuery":"outage","priorities":["CRITICAL"]},"pagination":{"limit":10,"offset":0}}'

# Create an incident
curl -X POST http://localhost:8080/incidents \
  -H "x-jwt-assertion: $JWT" \
  -H "Content-Type: application/json" \
  -d '{"callerId":"<caller-id>","category":"SECURITY","serviceId":"<service-id>","impact":"HIGH","urgency":"HIGH","subject":"Suspicious login activity"}'

# Get an incident by ID
curl -H "x-jwt-assertion: $JWT" http://localhost:8080/incidents/<incident-id>

# Partially update an incident
curl -X PATCH http://localhost:8080/incidents/<incident-id> \
  -H "x-jwt-assertion: $JWT" \
  -H "Content-Type: application/json" \
  -d '{"state":"RESOLVED","resolutionCode":"Solved (Work Around)"}'

# Search problems
curl -X POST http://localhost:8080/problems/search \
  -H "x-jwt-assertion: $JWT" \
  -H "Content-Type: application/json" \
  -d '{"filters":{"searchQuery":"database"},"pagination":{"limit":10,"offset":0}}'

# Search conversations
curl -X POST http://localhost:8080/conversations/search \
  -H "x-jwt-assertion: $JWT" \
  -H "Content-Type: application/json" \
  -d '{"filters":{"states":["ACTIVE"]},"pagination":{"limit":10,"offset":0}}'
```
