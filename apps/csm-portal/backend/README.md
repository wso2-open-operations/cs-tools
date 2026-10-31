# CSM Portal Backend

The Go backend-for-frontend (BFF) of the CSM Portal, the internal webapp that customer-success
engineers use to handle cases, projects, deployments, incidents, change requests, time cards and
more. It authenticates and authorises the caller, validates and shapes requests, and fans out to
the services that own the data.

This service owns no database. All business data lives behind the **entity-service** (see
[Role of this service](#role-of-this-service)).

## Table of contents

- [Quick start](#quick-start)
- [Architecture](#architecture)
  - [Role of this service](#role-of-this-service)
  - [Code layout](#code-layout)
  - [Request lifecycle](#request-lifecycle)
  - [Authentication and authorization](#authentication-and-authorization)
  - [Error handling](#error-handling)
  - [Adding a new endpoint](#adding-a-new-endpoint)
- [Developer guide](#developer-guide)
  - [Prerequisites](#prerequisites)
  - [Running locally](#running-locally)
  - [Make targets](#make-targets)
  - [Tests](#tests)
  - [Linting and security scanning](#linting-and-security-scanning)
  - [Git hooks](#git-hooks)
  - [OpenAPI spec](#openapi-spec)
  - [Debugging tips](#debugging-tips)
- [Configuration reference](#configuration-reference)
  - [Required settings](#required-settings)
  - [Server and timeouts](#server-and-timeouts)
  - [Authentication](#authentication)
  - [Access control](#access-control)
  - [Upstream services](#upstream-services)
  - [Feature flags](#feature-flags)
  - [Directory vocabularies](#directory-vocabularies)
  - [Dashboards](#dashboards)
  - [Notifications](#notifications)
  - [Attachment storage](#attachment-storage)
  - [Embedded PLG module](#embedded-plg-module)
  - [Settings read from the .env file](#settings-read-from-the-env-file)
- [API surface](#api-surface)

## Quick start

```bash
# from apps/csm-portal/backend
cp .env.example .env     # then fill in the required values, see "Required settings"
go run ./cmd/server
```

The server loads `.env` from the working directory on startup (silently ignored if absent) and
listens on `http://localhost:8080` by default. Every route except the two health probes needs a
token in the `x-jwt-assertion` header; see [Running locally](#running-locally).

To run the whole platform (both portals, entity-service, PostgreSQL, a mock identity provider)
instead of this service alone, use the Docker Compose stack described in
[../README.md](../README.md#run-everything-with-docker-compose).

## Architecture

For the platform-level picture (portals, gateway, backends, entity-service, event backbone) see
[the root architecture diagram](../README.md#architecture). This section only covers the inside of
this one service.

### Role of this service

The browser never talks to the entity-service directly. This BFF sits between them:

| Concern | Handled here (BFF) | Handled by the entity-service |
|---|---|---|
| Token validation, roles to permissions | Yes | No |
| Route-level authorization | Yes (`AUTH_*_ROLES` mapping) | No |
| Per-record rules that need the record (for example "only the assigned engineer may post a public comment", case state machine) | Yes | Stores the data the rule reads |
| Request shape guards (body size cap, UUID path params, JSON validity) | Yes | Full domain validation |
| Persistence, queries, business data, the data source (relational database or an external system) | No | Yes |
| Aggregation and reshaping for the webapp (dashboards, `nextStates`, role vocabulary, team registry) | Yes | Raw data |
| Credentials and URLs of other upstream services (user directory, updates, notifications) | Yes | No |

Most handlers are thin: they authorize, guard the request, forward the JSON body to the
entity-service client, and return the response untouched. Only responses the portal must reshape
are decoded into typed structs.

Upstream services this process calls, all with the same OAuth2 client-credentials application
(only base URL and scopes differ per service):

| Upstream | Package | Required |
|---|---|---|
| Customer entity-service (cases, accounts, projects, products, users, time cards, ...) | `internal/entity` | Yes |
| User-directory (SCIM) operations service | `internal/scim` | Yes |
| Product updates service | `internal/updates` | Yes |
| Engineering entity service (files GitHub issues) | `internal/entity` | No |
| Notification and integration services (health probe only today) | `internal/csmnotification`, `internal/csmintegration` | No |
| Sales entity GraphQL service, Google Drive | `internal/entity`, `internal/googledrive` | No |
| File-transfer service for direct attachment upload | `internal/sftpgo` | No |
| Google Chat incoming webhooks | `internal/notifications` | No |

### Code layout

```text
backend/
├── cmd/server/
│   ├── main.go              # Composition root: config, clients, handlers, route table, middleware chain, graceful shutdown
│   └── timeouts.go          # REST_READ_TIMEOUT / REST_WRITE_TIMEOUT / ENTITY_SERVICE_TIMEOUT parsing
├── internal/
│   ├── middleware/          # Auth (JWT), CORS, CorrelationID, Logger, SecurityHeaders
│   ├── handler/             # HTTP handlers, one file per resource; access.go holds the permission model
│   ├── entity/              # Entity-service clients (customer, engineering, sales GraphQL); customer*.go = raw passthrough methods
│   ├── apierror/            # Typed non-2xx upstream error (status + body excerpt)
│   ├── scim/                # User-directory (SCIM) operations client
│   ├── updates/             # Product updates client
│   ├── csmnotification/     # Notification-service client (health probe)
│   ├── csmintegration/      # Integration-service client (health probe)
│   ├── notifications/       # Google Chat and email notification channels
│   ├── googledrive/         # Google Drive folder browsing
│   ├── sftpgo/              # Direct-upload attachment storage client
│   ├── risk/                # Customer-health risk tracking, over the entity client
│   ├── dashboard/           # Dashboard definition registry (directory loader, presets, sections)
│   ├── directory/           # Team registry, role allow-list, role-id mapping parsed from config
│   ├── viewer/              # Viewer-audience helpers
│   └── plg/                 # Embedded PLG module, mounted onto the same mux (see below)
├── dashboards.example/      # Committed sample dashboard definitions
├── openapi.yaml             # Published API contract
├── .env.example             # Configuration template
├── .choreo/component.yaml   # Deployment descriptor (port 8080, schema = openapi.yaml)
└── Makefile
```

Dependency direction is `cmd/server` to `handler` to `entity` / `scim` / `updates` / ... to
`apierror`. Handlers depend on small local interfaces (for example `entityGroupClient`), not on
the concrete client, so tests substitute a mock. `internal/middleware` and `internal/handler`
share the `UserInfo` carried in the request context.

### Request lifecycle

The handler chain is assembled in `cmd/server/main.go`, outermost first:

1. **SecurityHeaders** sets `X-Content-Type-Options`, `Content-Security-Policy` and
   `Strict-Transport-Security` on every response, including CORS preflights.
2. **CORS** answers preflights and sets `Access-Control-Allow-Origin` only for origins listed in
   `CORS_ALLOWED_ORIGINS`. It runs before Auth because a preflight carries no token.
3. **CorrelationID** reads `X-CSM-Correlation-ID` or generates one, echoes it on the response, adds
   it to every log line, and stores it for forwarding to upstreams.
4. **Auth** validates the `x-jwt-assertion` token (signature through the JWKS endpoint, issuer,
   expiry, audience), requires `email` and `userid` claims, and puts a `UserInfo` (email, user id,
   names, roles) in the context. `GET /health` and `GET /health/dependencies` skip it. It also
   stores the `x-user-id-token` header (falling back to the assertion itself) for forwarding.
5. **Logger** writes one `request completed` line per request (method, path, status, elapsed).
6. **ServeMux** (Go 1.22 method-prefixed patterns such as `GET /cases/{id}`) dispatches to the
   route, which `route(pattern, permission, handler)` has wrapped in **AccessGuard.Require**: it
   returns `401` with no user, `403` when the token roles do not satisfy the permission.
7. **The handler** guards the request, calls the upstream client, and writes JSON.
8. **The upstream client** (`CustomerEntityClient.do`) obtains and refreshes an OAuth2 token
   automatically, forwards `x-user-id-token` and `X-CSM-Correlation-ID`, and returns the raw
   response body or an `*apierror.Error` for any non-2xx.

Shutdown is graceful (15 s) on `SIGINT`/`SIGTERM`.

### Authentication and authorization

- The identity provider is any OIDC provider. In a real deployment the API gateway validates the
  bearer token and passes it to this service as `x-jwt-assertion`; this service validates it again
  against `AUTH_JWKS_ENDPOINT`. It never stores credentials.
- **Route-level authorization** is a set lookup of the token's `roles` claim against the role names
  configured in the `AUTH_*_ROLES` variables (see [Access control](#access-control)). No upstream
  call is made. The role names are deployment configuration and have no defaults. Every route is
  registered through `route(...)` with a required permission, so a route cannot be added without
  choosing one.
- **Record-level authorization** is resolved from platform data, not from the token: for example,
  posting a public case comment requires being the case's assigned engineer, and de-escalating
  requires being one of the case's team leads (and nothing more than a portal role). Handlers read the record from the entity-service for
  these checks. A caller whose token grants a limited role but has no platform user record is
  provisioned one on first use from their own token.
- `GET /users/me` reports the caller's portal roles from the same guard that enforces the routes, so
  what the webapp shows and what the backend enforces cannot drift. The webapp must treat the
  backend's `403` as the real gate and never add authorization gates of its own.

### Error handling

- Every error body is `{"message": "..."}`, plus an optional stable `errorCode` for a few refusals a
  client may branch on. Clients must never branch on `message`.
- Auth failures are `401` with a fixed message and are logged server-side with the real reason.
- Handlers translate an upstream `*apierror.Error` with `mapUpstreamErrorGeneric` by default: `401`,
  `403`, `404` get fixed messages, `400/409/422` get the handler's fallback message, `502/503/504`
  become `503`, anything else `500`. The upstream reason is **not** echoed to the caller.
- `mapUpstreamError` (which does surface the upstream reason for `400/409/422`) is reserved for
  PATCH/update handlers, where a 4xx is a rejection of what the caller just sent.
- Log the full upstream error with `slog.ErrorContext(ctx, ..., "err", err)` before mapping. Do not
  log request bodies, token contents or personal data; log ids and error summaries only.
- Request bodies are capped at 1 MiB (`maxRequestBodyBytes`) with `http.MaxBytesReader`; larger
  bodies get `413`.

### Adding a new endpoint

Follow these steps in order, lowest layer first. Do not add a route for a field the entity-service
does not accept yet.

1. **Entity-service first.** The data or behavior must exist upstream (a separate component in this
   repository). Verify it works against the real service before building on it.
2. **Upstream client method** in `internal/entity/customer.go` (or the relevant `internal/<module>/`
   package): a method that calls `c.do(ctx, method, path, body)`. Escape every path parameter with
   `url.PathEscape`.
3. **Handler interface.** Extend the small local interface in the handler file (for example
   `entityGroupClient` in `internal/handler/groups.go`), listing only methods that handler calls.
4. **Handler function.** In order: read the user with `middleware.UserInfoFromContext` (nil is
   `401`), validate path params (empty check; `uuidRe` for UUIDs, `400` on mismatch), cap and read
   the body, check `json.Valid`, call the client, log and map the error with
   `mapUpstreamErrorGeneric`, then `writeJSON`. `internal/handler/groups.go` is a compact example.
5. **Register the route** in `cmd/server/main.go` with
   `route("POST /things/search", handler.PermView, thingHandler.SearchThings)`. Pick the narrowest
   existing permission (see [Access control](#access-control)); a new permission belongs in
   `internal/handler/access.go` with its config variable.
6. **Document it in `openapi.yaml`**: the path, request and response schemas, and `200/400/401/403/404/500`
   responses (`403` is always required). A test fails if a registered route is missing from the
   spec (see [OpenAPI spec](#openapi-spec)).
7. **Tests.** Add handler tests next to the handler, extending the mock in
   `internal/handler/helpers_test.go` if you extended an interface.
8. **Verify for real.** Run the stack, call the endpoint, and read the record back. A `200` alone
   does not prove state changed.
9. If the endpoint adds, moves or removes a user-visible flow, update the matching topic of the
   in-app user guide in the webapp.

## Developer guide

### Prerequisites

- Go matching the `go` directive in [`go.mod`](go.mod) (currently `1.26.x`). Install from
  <https://go.dev/doc/install>.
- `make` (optional, for the targets below).
- Docker (optional): only for the full local stack and for the pre-push hook.

### Running locally

1. `cp .env.example .env` and set the [required settings](#required-settings). For a local run
   without a real identity provider:
   - set `AUTH_TOKEN_VALIDATOR_ENABLED=false` so tokens are decoded but their signature is not
     verified (never in a shared or production environment);
   - `AUTH_JWKS_ENDPOINT`, `AUTH_ISSUER` and `AUTH_AUDIENCE` are still required to be non-empty;
     placeholder values are fine in this mode (for example `https://idp.example.com/jwks`,
     `https://idp.example.com`, `csm-portal`);
   - set `CORS_ALLOWED_ORIGINS` to your webapp origin (for example `http://localhost:3001`) if a
     browser will call this service directly;
   - set the `AUTH_*_ROLES` variables to the role names you will put in your test token's `roles`
     claim, or every route except `/users/me` returns `403`.
2. The service needs a reachable entity-service (`CUSTOMER_ENTITY_BASE_URL`) and an OAuth2 token
   endpoint (`OAUTH2_TOKEN_URL`) to serve data. The Docker Compose stack in the root README provides
   both, plus a mock identity provider that issues tokens for any user name.
3. `go run ./cmd/server` (or `make build && ./server`).
4. Call it with a token:

```bash
JWT="<a-jwt-with-email-userid-and-roles-claims>"

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/users/me

curl -X POST http://localhost:8080/cases/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"filters":{"searchQuery":"login error"},"pagination":{"limit":10,"offset":0}}'
```

The token must carry `email` and `userid` claims (and `roles` for anything beyond `/users/me`).
`given_name` and `family_name` are optional. `roles` may be a string or an array.

### Make targets

| Target | What it does |
|---|---|
| `make setup` | Points the repository's git hooks path at `.githooks` (once per clone) |
| `make vet` | `go vet ./...` |
| `make test` | `vet`, then `go test -race ./...` |
| `make build` | `test`, then compiles `./cmd/server` to `./server` |
| `make clean` | Removes the `./server` binary |
| `make` / `make all` | Same as `make build` |

### Tests

Tests are pure unit tests: no running services, network or environment variables are needed.

```bash
go test ./...                                   # everything
go test -race ./...                             # with the race detector (what `make test` runs)
go test ./internal/handler/...                  # one package
go test -v -run TestSearchAccounts ./internal/handler/...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

`cmd/server` has startup tests that exercise the config loaders, and
`TestEveryRegisteredRouteIsInOpenAPI`, which fails when a route registered in `main.go` has no
operation in `openapi.yaml`.

### Linting and security scanning

Run these before opening a pull request; both should report nothing.

```bash
go vet ./...                                                    # also run by `make test`
go install github.com/securego/gosec/v2/cmd/gosec@latest        # once
gosec -fmt=text ./...
go install golang.org/x/vuln/cmd/govulncheck@latest             # once
govulncheck ./...
```

Fix the root cause of a gosec finding rather than suppressing it. Most `govulncheck` findings are
standard-library issues fixed by bumping the `go` directive in `go.mod` to the latest patch of the
pinned minor version and running `go mod tidy`.

There is nothing to regenerate: the service has no code generation. The only generated-looking
artifact is the compiled `./server` binary, which is git-ignored.

### Git hooks

Run once per clone:

```bash
make setup      # equivalent to: git config core.hooksPath .githooks   (from the repo root)
```

The repository's `pre-push` hook then runs the tests of the components touched by the push inside a
`golang` container (so Docker must be running) and aborts the push on failure. Skip it in an
exceptional case with `git push --no-verify`.

### OpenAPI spec

[`openapi.yaml`](openapi.yaml) is the published contract used by the webapp, other teams and the
API gateway, which only forwards operations the spec documents. Update it in the same change as
any API change: new, removed or changed endpoints, request/response shapes and error codes.

- Error responses use `$ref: '#/components/schemas/ErrorPayload'`.
- Every operation declares a `403` response.
- UUID path parameters declare `format: uuid`.
- Binary download operations document the `Content-Disposition: attachment` and
  `X-Content-Type-Options: nosniff` headers.
- Edit it by hand; it is not generated. `.choreo/component.yaml` points the deployment at it.

### Debugging tips

- **Startup exits immediately.** The log names the offending variable (`required environment
  variable is not set`, `invalid CSM_TEAM_REGISTRY`, `invalid dashboard definitions`, and so on).
  Malformed configuration is fatal on purpose.
- **`403` on everything.** The token's `roles` claim matches none of the `AUTH_*_ROLES` values.
  Startup logs a warning listing every unset role variable; the access log line says
  `access denied: token carries no role granting this permission`.
- **`401`.** The token is missing, expired, has the wrong issuer or audience, or lacks the `email` or
  `userid` claim. The reason is in the server log (`auth: token validation failed`), not the response.
- **Browser says "blocked by CORS".** `CORS_ALLOWED_ORIGINS` is empty (default: no cross-origin
  request is allowed) or lacks the exact origin.
- **Following one request.** Send `X-CSM-Correlation-ID: my-trace-1`; it appears on every log line for
  that request and is forwarded to the entity-service, so you can follow it across both logs.
- **Upstream `4xx` shown as a generic message.** By design; the full upstream status and body are in
  the server log line for the failed call.
- **Slow uploads cut off.** Raise `REST_WRITE_TIMEOUT` and `ENTITY_SERVICE_TIMEOUT` (keep the second
  shorter).
- **Re-reading dashboards without restarting.** Set `DASHBOARDS_HOT_RELOAD=true` (local only).
- Logs are text on stderr via `log/slog`.

## Configuration reference

All configuration is environment variables (the Choreo deployment supplies them from its own
configuration). A `.env` file in the working directory is loaded at startup for local use; see
[Settings read from the .env file](#settings-read-from-the-env-file). `.env.example` is the template.
Never commit a real `.env`.

Conventions used below:

- **Required** means the process exits at startup if unset or empty.
- "Comma list" values are split on `,` with whitespace trimmed and empty items dropped.
- Go durations look like `45s` or `2m`.
- Example values are placeholders.

### Required settings

| Variable | Description | Example |
|---|---|---|
| `OAUTH2_CLIENT_ID` | OAuth2 client id, shared by every upstream service client | `your-client-id` |
| `OAUTH2_CLIENT_SECRET` | OAuth2 client secret, shared by every upstream service client | `your-client-secret` |
| `OAUTH2_TOKEN_URL` | OAuth2 token endpoint, shared by every upstream service client | `https://idp.example.com/oauth2/token` |
| `CUSTOMER_ENTITY_BASE_URL` | Base URL of the customer entity-service | `https://entity.example.com/v1` |
| `UPDATES_BASE_URL` | Base URL of the updates service | `https://updates.example.com` |
| `SCIM_BASE_URL` | Base URL of the user-directory (SCIM) operations service | `https://scim.example.com` |
| `AUTH_JWKS_ENDPOINT` | JWKS endpoint used to verify token signatures (required even when the validator is disabled) | `https://idp.example.com/oauth2/jwks` |
| `AUTH_ISSUER` | Expected `iss` claim | `https://idp.example.com` |
| `AUTH_AUDIENCE` | Comma list of accepted `aud` values; a token passes if any is present | `csm-portal` |

These nine are the only variables the process refuses to start without. Without the `AUTH_*_ROLES`
variables below it starts, but nobody can use it.

### Server and timeouts

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `PORT` | No | `8080` | Listen port; a plain number 1-65535, not an address | `8080` |
| `REST_READ_TIMEOUT` | No | `60s` | HTTP server read timeout | `60s` |
| `REST_WRITE_TIMEOUT` | No | `60s` | HTTP server write timeout | `60s` |
| `ENTITY_SERVICE_TIMEOUT` | No | `60s` | Per-request timeout of the entity-service client | `45s` |
| `CORS_ALLOWED_ORIGINS` | No | empty (no cross-origin request allowed) | Comma list of browser origins allowed to call this service directly. Behind a gateway that adds CORS itself, leave unset. Not listed in `.env.example` | `http://localhost:3001` |

The three timeouts must be greater than 0 or startup fails; no ordering is enforced, but keeping
`ENTITY_SERVICE_TIMEOUT` below `REST_WRITE_TIMEOUT` lets a handler return a clean error. The 60 s
defaults exist so large inline-attachment uploads are not cut off.

### Authentication

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `AUTH_TOKEN_VALIDATOR_ENABLED` | No | enabled | Any value other than the literal `false` keeps signature verification on. `false` only decodes the token: **local development only** | `false` |

`AUTH_JWKS_ENDPOINT`, `AUTH_ISSUER` and `AUTH_AUDIENCE` are in [Required settings](#required-settings).
Clock skew tolerance is fixed at 5 s.

### Access control

Each variable is a comma list of role names as they appear in the token's `roles` claim; holding
**any one** grants the portal role. Matching is exact and case-sensitive. There is **no default**:
a role whose variable is unset or empty is held by nobody, and startup logs a warning naming each
unset one (except the three marked optional).

| Variable | Warns if unset | Grants |
|---|---|---|
| `AUTH_VIEWER_ROLES` | Yes | `view`, `view_shared_entity`, read access to security-report cases, and the viewer-audience routes (customer health, files, `/user-info`, ...) |
| `AUTH_ESCALATOR_ROLES` | Yes | `view`, `escalate` |
| `AUTH_ATTACHMENT_DOWNLOADER_ROLES` | Yes | `view`, `download_attachment` |
| `AUTH_SUPPORT_ENGINEER_ROLES` | Yes | The `cs_engineer` portal role: `view`, `view_operations`, `time_cards_and_updates`, `download_attachment`, `write`, `security_center`, `escalate`, work notes, comment edit/delete, PLG use. (The variable keeps its old name; the role is `cs_engineer`.) Not `approve_time_card`, `admin` or `create_announcement` |
| `AUTH_ADMIN_ROLES` | Yes | Everything, including `admin`-only routes |
| `AUTH_USAGE_METRICS_VIEWER_ROLES` | Yes | `view`; the `/usage-metrics/*` routes |
| `AUTH_TIMECARD_APPROVER_ROLES` | Yes | `view`, `time_cards_and_updates`, `approve_time_card` |
| `AUTH_DASHBOARD_DESIGNER_ROLES` | Yes | `view` |
| `AUTH_ANNOUNCEMENT_CREATOR_ROLES` | Yes | Creating and sending customer announcements. Required **in addition to** `write`; `admin` always can. Unset means only `admin` can |
| `AUTH_SECURITY_TEAM_ROLES` | Yes | Read-only `view`, `view_operations`, `security_center`; never any state-changing permission |
| `AUTH_WORKNOTE_CREATOR_ROLES` | No (optional) | Posting a `work_note` comment on a case only; none of `write`'s other actions |
| `AUTH_COMMENT_UPDATER_ROLES` | No (optional) | Reaching comment edit/delete, and editing or deleting **any** comment (a plain `cs_engineer` only succeeds on their own); also `security_center` |
| `AUTH_ONBOARDING_LEAD_ROLES` | No (optional) | Setting a project's onboarding status, owner and dates (`PATCH /projects/{id}`) only; no read access and no other write. `admin` always can, `cs_engineer` cannot. Unset means only `admin` can |
| `AUTH_SALES_SOLUTIONS_ROLES` | No (optional) | Only `view_shared_entity` (the shared case, account, project and team-member reads); also reported by `GET /users/me` so the webapp can pick a navigation |

Example: `AUTH_ESCALATOR_ROLES=example-escalators-role,example-leads-role`.

Permissions and the routes they guard (`internal/handler/access.go` is the source of truth):

| Permission | Routes |
|---|---|
| authenticated | `/users/me` and its saved-filter-views sub-routes: any valid token, so a caller with no portal role can still load their profile and see a "no access" screen |
| `view` | Reads, `*/search` and `*/aggregate` routes not listed below |
| `view_shared_entity` | The subset of reads shared with the sales audience (`GET /cases/{id}`, `POST /cases/search`, accounts, projects, team members): everyone with `view` plus the sales-solutions role |
| `view_operations` | Operations reads: incidents, change requests, problems, incident tasks, outages, alerts. `cs_engineer`, `admin`, `security_team` |
| `time_cards_and_updates` | Time-card routes and update-level lookups. `cs_engineer`, `admin`, time-card approver |
| `approve_time_card` | `PATCH /time-cards/{id}` when the body sets `state`; checked inside the handler. Time-card approver and `admin` |
| `escalate` | Escalating via `POST /cases/{id}/escalations` (the route itself only needs `view`; the handler checks this for `ESCALATE`). De-escalating does not use it: it requires being one of the case's team leads, whatever the caller's other roles, and holding `escalate` does not substitute |
| `download_attachment` | `GET /attachments/{id}/content`, `POST /attachments/{id}/share` |
| `write` | Every other `POST`/`PATCH`/`DELETE` |
| `admin` | Admin-only routes such as `POST /users`, `POST /users/{id}/roles`, `GET /roles/grantable`, knowledge-base administration |
| `security_center` | `/products/vulnerabilities` routes. Security-report cases are gated inside the case handler |
| `create_announcement` | Writes on `/announcement-requests` (together with `write`), and `POST /cases` with `type: announcement` |
| `update_delete_comment` | `PATCH`/`DELETE /comments/{id}`; which comment may be touched is decided per comment (see `AUTH_COMMENT_UPDATER_ROLES`) |
| `manage_onboarding` | `PATCH /projects/{id}` when the body sets `onboardingStatus`, `onboardingOwner` or any of the four onboarding dates (`onboardingStartDate`, `onboardingGoLivePlanDate`, `onboardingGoLiveDate`, `onboardingExpiryDate`); checked inside the handler. `onboarding_lead` and `admin`. The route itself (`update_project`) admits `cs_engineer`, `admin` and `onboarding_lead`; any other field in the body still needs `write` |
| `use_plg`, `manage_playbooks` | The `/plg/*` routes: `cs_engineer` and `admin` use them; authoring playbook templates is `admin` only |

A caller with none of the required roles gets `403`. Some actions need more than the role: posting a
public case comment also requires being the case's assigned engineer (the role is necessary, not
sufficient).

### Upstream services

Every client below authenticates with the shared `OAUTH2_*` credentials; only its URL and scopes
differ.

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `CUSTOMER_ENTITY_SCOPES` | No | none | Comma list of OAuth2 scopes for the entity-service (also reused by the embedded PLG module) | `entity.read,entity.write` |
| `CUSTOMER_ENTITY_DATA_SOURCE` | No | the legacy external-system value (see notes) | Which data source the paired entity-service runs (its own `DATA_SOURCE`): `postgres` or the legacy external-system value that `.env.example` shows. Case-insensitive; any other value is fatal | `postgres` |
| `UPDATES_SCOPES` | No | none | Comma list of OAuth2 scopes for the updates service | `updates.read` |
| `SCIM_SCOPES` | No | none | Comma list of OAuth2 scopes for the SCIM service | `scim.read` |
| `ENGINEERING_ENTITY_BASE_URL` | No | unset | Base URL of the engineering entity service. When set, `POST /cases/{id}/github-issues` files issues through it instead of the entity-service, and `GET /health/dependencies` checks it. Must be `https` (a path is allowed; userinfo, query and fragment are not), else startup fails | `https://engineering.example.com/v1` |
| `ENGINEERING_ENTITY_SCOPES` | No | none | Comma list of OAuth2 scopes for it | `engineering.write` |
| `CSM_NOTIFICATION_SERVICE_BASE_URL` | No | unset | Notification service, used only by `GET /health/dependencies` today. `https` base URL; unset reports `not_configured` | `https://notify.example.com` |
| `CSM_NOTIFICATION_SERVICE_SCOPES` | No | none | Comma list of scopes | `notify.read` |
| `CSM_INTEGRATION_SERVICE_BASE_URL` | No | unset | Integration service, used only by `GET /health/dependencies` today. `https` base URL | `https://integration.example.com` |
| `CSM_INTEGRATION_SERVICE_SCOPES` | No | none | Comma list of scopes | `integration.read` |
| `GOOGLE_DRIVE_CLIENT_ID` | No | unset | OAuth2 client id for the Drive folder-browsing routes (`/files`) | `your-drive-client-id` |
| `GOOGLE_DRIVE_CLIENT_SECRET` | No | unset | Matching client secret | `your-drive-client-secret` |
| `GOOGLE_DRIVE_REFRESH_TOKEN` | No | unset | Refresh token for the OAuth2 refresh-token grant | `your-refresh-token` |
| `PAGING_PHONE_SCIM_FALLBACK` | No | on | Case-paging reads phone numbers from the entity-service and asks SCIM for people with none. Any value except `false` keeps the fallback; set `false` once numbers are filled | `false` |

Notes:

- `CUSTOMER_ENTITY_DATA_SOURCE` defaults to the legacy external-system value (the data source
  deployments used before PostgreSQL). It only matters for two things today: team search merges the
  entity-service's `team` table only for `postgres`, and the combination of `postgres` with a
  non-empty `CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS` is rejected at startup (the Postgres project
  search cannot honor the exclusion). **When running against PostgreSQL, set it to `postgres`.**
- `UPDATES_BASE_URL`, `SCIM_BASE_URL` and `CUSTOMER_ENTITY_BASE_URL` are not scheme-checked; the
  optional services above are, and must be `https`.
- When `ENGINEERING_ENTITY_BASE_URL` is set, the issue's repository comes from the case's product via
  the entity-service's product-to-repository mapping (`GET /products/github-repo`), a request
  `repoOverride` is ignored, an unmapped product gets `400`, and a work note with the issue URL is
  added to the case best-effort.

### Feature flags

| Variable | Default | Description |
|---|---|---|
| `CSM_MIGRATION_ONBOARDING_STATUS_ENABLED` | **on** | Registers `GET /projects/{id}/onboarding-steps` (per-contact onboarding status). Unlike the other flags, anything except `false` (case-insensitive, trimmed) leaves it **on**, including unset; set `false` to disable, in which case the path 404s. See the note below |
| `SFTPGO_ATTACHMENT_STORAGE_ENABLED` | off | Enables direct-upload attachment storage; see [Attachment storage](#attachment-storage). Parsed with `strconv.ParseBool`; an unparseable non-empty value logs a warning and is treated as off |

> The inline comments in `cmd/server/main.go` and `.env.example` describe
> `CSM_MIGRATION_ONBOARDING_STATUS_ENABLED` as "off unless exactly `true`", but the implementation
> (`onboardingStatusEnabled`) and its unit test make it on unless `false`. This README documents the
> implemented behavior.

### Directory vocabularies

Curated lists supplied as configuration, so adding a team or role is a restart, not a release.
Both are parsed at startup and a malformed value is fatal.

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `CSM_TEAM_REGISTRY` | No | none (empty registry, startup warns) | Team catalogue: rows separated by `,`, fields by `\|`: `teamKey\|Display Name[\|FAMILY[\|creGroupId[\|sreGroupId]]]`. `FAMILY` is one of `CRE-ABT`, `CRE`, `SRE-ABT`, `SRE`, `SME` (case-insensitive) or empty; group ids are 32 hex characters. Duplicate keys or names fail startup naming the row | `alpha\|Alpha Team\|CRE-ABT,beta\|Beta Team\|SRE-ABT` |
| `CSM_USER_ROLES` | No | `agent,admin,commenter,customer,customer_admin,partner,partner_admin,internal,external,timecard_approver` | Assignable-role allow-list, comma list; validates the `roleIds` user filter and is served by `POST /roles/search`. Duplicates are fatal | `agent,admin,internal,external` |
| `ASGARDEO_ROLE_IDS` | No | none | JSON object mapping a real role name (exactly as listed in `AUTH_<ROLE>_ROLES`) to its identity-provider role id. Enables SCIM-backed features: `GET /users/time-card-approvers`, granting roles through `POST /users` (`grantRoles`) and `POST /users/{id}/roles` (`GET /roles/grantable` reports the same resolved set both read), and `csmPlatformRoles` on `GET /users/{id}`. A name without an entry just has those features off. The variable name is historical | `{"example-approver-role":"00000000-0000-0000-0000-000000000000"}` |
| `ASGARDEO_ROLE_GROUP_NAMES` | No | none | JSON object mapping a **portal role key** (not a real Asgardeo role name -- see `ASGARDEO_ROLE_IDS` above) to a list of Asgardeo **group display names** (not ids -- matched case-sensitively against the name shown in the Asgardeo console). A second, independent way `csmPlatformRoles` on `GET /users/{id}` can grant a portal role: by Asgardeo group membership, instead of (or alongside) a role assignment. A portal role key without an entry simply has no group that grants it, and leaving this variable unset entirely changes nothing -- group matching is purely additive on top of `ASGARDEO_ROLE_IDS` | `{"cs_engineer":["CSM Engineers"]}` |
| `CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS` | No | none | Comma list of project keys that an "all customer projects" announcement audience search always excludes (the caller cannot opt out). Requires `CUSTOMER_ENTITY_DATA_SOURCE` not to be `postgres` | `PROJA,PROJB` |

`CSM_TEAM_REGISTRY` has no default on purpose: team names are organisation vocabulary. `Display Name`
is matched verbatim against the data source's group name when resolving members, so a wrong one
resolves zero members silently (blank names are rejected for that reason). Fields are positional:
supply `key|Name||<creGroupId>` to give an id without a family. A `family` filter on
`POST /teams/search` is answered from the registry alone.

### Dashboards

Dashboard definitions are JSON files, **one file per dashboard**, read once at startup and held in
memory. The filename is irrelevant: `id`, `displayName` and `type` (`cs`, `cre` or `sre`) come from
the file. A malformed, unreadable or duplicate-`id` file **fails startup naming the file**. Files
starting with `_` and non-`.json` files are ignored by the directory loader.

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `DASHBOARDS_DIR` | No | unset (no dashboards unless `DASHBOARDS_CONFIG` is set) | Directory with one `*.json` per dashboard. A set but missing directory is fatal | `./dashboards.example` |
| `DASHBOARDS_HOT_RELOAD` | No | `false` | Re-read `DASHBOARDS_DIR` on every request instead of serving the startup snapshot. `strconv.ParseBool` values; an unparseable non-empty value warns and is `false`. A failed re-read keeps serving the last good definitions. **Local development only** | `true` |
| `DASHBOARD_PRESETS_FILE` | No | none | JSON file of `presetKey` to literal filter fragment, shared by every dashboard and referenced as `{"preset": "key"}`. A dashboard's own `filterPresets` shadows it. Only read when `DASHBOARDS_DIR` is set | `./dashboards.example/_presets.json` |
| `DASHBOARD_SECTIONS_FILE` | No | none | JSON file of reusable widget sections (`sectionKey` to `{displayName, widgets}`) that a dashboard pulls in through `includeSections`. Only read when `DASHBOARDS_DIR` is set | `./dashboards.example/_sections.json` |
| `DASHBOARDS_CONFIG` | No | none | **Deprecated.** The whole registry as one JSON array. Used only when `DASHBOARDS_DIR` is unset; malformed content is fatal | `[...]` |

A definition may set `"restricted": true`: only callers holding `cs_engineer` or `admin` see it in
`GET /dashboards`, and `GET /dashboards/{id}` returns `403` to everyone else.

#### `dashboards.example/`

A committed, sanitised sample set that shows the schema; `.env.example` points `DASHBOARDS_DIR` at
it so a fresh clone starts.

| File | Purpose |
|---|---|
| `sample-dashboard.json` | An unrestricted, default (`isDefault`) dashboard of `type: cs`: mostly `count` widgets over case states, each with `id`, `displayName`, `resourceType`, `shape`, `gridWidth`, `section` and a `query.filters` array of `{field, op, values}` |
| `sample-team-dashboard.json` | A team-scoped dashboard (`type: cre`, `isTeamBased: true`, `targetTeam`) marked `"restricted": true` |
| `_sections.json` | Shared widget sections (for example "My Work", using the `__current_user__` placeholder). Skipped by the directory loader because of the leading underscore; used by pointing `DASHBOARD_SECTIONS_FILE` at it and referencing it from `includeSections` |

For a real deployment: `cp -r dashboards.example dashboards` (the `./dashboards` directory is
git-ignored because widget criteria are environment-specific) and set `DASHBOARDS_DIR=./dashboards`.
A very large dashboard can be split over several files by giving the extra files `"partOf": "<id>"`
and only `widgets`; any other field in a part file is a startup error.

### Notifications

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `NOTIFICATIONS_GOOGLE_CHAT_SPACES` | No | none | JSON array of `{"product","webhookUrl"}`, one Google Chat space per product, used by `POST /notifications/google-chat/alerts`. The product match is case- and whitespace-insensitive; an unconfigured product returns an error. Malformed JSON is logged and alerts are unavailable; startup continues | `[{"product":"example-product","webhookUrl":"https://chat.example.com/webhook/your-key"}]` |
| `CSM_PORTAL_WEB_BASE_URL` | No | none | Base URL of the webapp, used for the "Open in CSM Portal" link in alert cards | `http://localhost:3001` |

`internal/notifications` also contains an email client, but it is not constructed in
`cmd/server/main.go` and no `NOTIFICATIONS_EMAIL_*` variable is read today.

### Attachment storage

Off by default. When `SFTPGO_ATTACHMENT_STORAGE_ENABLED` is true, these are read and the
direct-upload routes (`POST /cases/{id}/attachments/upload-token`,
`POST /attachments/{id}/share`, `POST /cases/{caseId}/attachments/{attachmentId}/confirm`) and
inline-image extraction on case comments are active. When off none of them are read, the three
routes are not registered (404), and the streaming attachment routes are unaffected either way.

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `SFTPGO_BASE_URL` | When enabled | none | The file-transfer service's REST API base URL. `https`, a host, no userinfo, no path, no query or fragment | `https://files.example.com` |
| `SFTPGO_PUBLIC_BASE_URL` | No | `SFTPGO_BASE_URL` | Public host used to build share links, if its share pages are fronted separately. Same URL rules | `https://share.example.com` |

### Embedded PLG module

`internal/plg` is an additional set of handlers mounted on the same mux under the `/plg` prefix. It
reuses this service's middleware chain, the same permission guard and, by default, the same
entity-service address and OAuth2 application, so nothing needs configuring. These overrides exist
for unusual setups:

| Variable | Default | Description |
|---|---|---|
| `PLG_CONFIG_FILE` | `config.json` | Optional JSON config file; a missing file is fine, a malformed one is fatal |
| `PLG_ENTITY_BASE_URL` | `CUSTOMER_ENTITY_BASE_URL` | Entity-service base URL (must be absolute `http(s)`) |
| `PLG_ENTITY_OAUTH_TOKEN_URL` | `OAUTH2_TOKEN_URL` | Token endpoint. Plaintext `http` is rejected unless the host is loopback |
| `PLG_ENTITY_OAUTH_CLIENT_ID` | `OAUTH2_CLIENT_ID` | OAuth2 client id |
| `PLG_ENTITY_OAUTH_CLIENT_SECRET` | `OAUTH2_CLIENT_SECRET` | OAuth2 client secret |
| `PLG_ENTITY_SCOPES` | `CUSTOMER_ENTITY_SCOPES` | Scope string passed through verbatim |
| `PLG_ENTITY_TIMEOUT_SECONDS` | `20` | Per-request timeout in seconds |
| `PLG_SERVER_PORT`, `PLG_SERVER_REQUEST_TIMEOUT_SECONDS`, `PLG_ALLOWED_ORIGINS`, `PLG_LOG_LEVEL`, `PLG_LOG_FORMAT` | | Parsed and validated (an invalid `PLG_LOG_LEVEL` other than `DEBUG/INFO/WARN/ERROR` or `PLG_LOG_FORMAT` other than `text/json` fails startup) but not used when embedded; the main server's port, timeouts, CORS and logging apply |

A plaintext `http` token URL on a non-loopback host (for example a container hostname in a local
stack) makes the PLG guard fail startup; give `PLG_ENTITY_OAUTH_TOKEN_URL` a loopback URL there.

### Settings read from the .env file

The `.env` loader is deliberately simple:

- `KEY=value` lines; blank lines and lines starting with `#` are skipped.
- Surrounding single or double quotes on the value are stripped.
- A variable already set in the real environment is **never overridden** by `.env`.
- **There are no inline comments.** `AUTH_AUDIENCE=  # comma list` sets the value to `# comma list`.
  Put comments on their own line, and clear any trailing comment that `.env.example` carries on a
  value line (for example after `AUTH_AUDIENCE=`) when you copy it.

## API surface

The authoritative, complete list of operations with request and response schemas is
[`openapi.yaml`](openapi.yaml). Routes are registered in `cmd/server/main.go` (about 250 on the
main service plus the `/plg/*` routes). Main areas:

| Area | Example routes |
|---|---|
| Health | `GET /health` (liveness, always `200`, no dependency calls: use for restart probes), `GET /health/dependencies` (aggregates SCIM, updates, and the optional notification, integration and engineering services; `200`, or `503` if any checked dependency is down; do not use as a liveness probe). Neither requires a token |
| Cases | `POST /cases`, `GET`/`PATCH /cases/{id}`, `POST /cases/search`, `/cases/aggregate`, `/cases/volume-trend`, comments, activities, escalations, call requests, tags, GitHub issues |
| Attachments | `POST /attachments`, `POST /attachments/search`, `GET /attachments/{id}/content`, `DELETE /attachments/{id}` |
| Accounts, projects, products, deployments | `/accounts/*`, `/projects/*` (contacts, metadata), `/products/*`, `/deployments/*` and deployed products |
| Operations | `/incidents`, `/change-requests` (including approvals), `/problems`, `/incident-tasks`, `/outages`, `/alerts`, `/smart-alerts` |
| Time cards and updates | `/time-cards/*`, `/updates/product-update-levels`, `/updates/levels/search` |
| Users and roles | `/users/me`, `/users/search`, `/users/{id}`, `POST /users`, `POST /users/{id}/roles`, `/roles/search`, `/roles/grantable`, `/teams/search` |
| Team schedule | `/team-schedule/*` (assignments, absences, rotas, paging chain) |
| Knowledge base | `/kb-articles/*`, `/knowledge-bases/*`, `/kb-manager-*` |
| Announcements | `/announcements/*`, `/announcement-requests/*` |
| Dashboards | `/dashboards`, `/dashboards/{id}`, `/dashboards/filter-presets`, `/dashboards/sections` |
| Viewer audience | `/customer-health/*`, `/usage-metrics/*`, `/files`, `/user-info` |

Many routes forward to entity-service operations that only exist for one data source; when the
paired entity-service does not support an operation for its data source it returns an error that
this service passes through as a mapped status. Do not hide actions in the webapp by data source;
let the backend reject and surface the error.
