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

Run [govulncheck](https://golang.org/x/vuln/cmd/govulncheck) to check for known vulnerabilities:

```bash
# Install govulncheck (once)
go install golang.org/x/vuln/cmd/govulncheck@latest

# Run from apps/csm-portal/backend
govulncheck ./...
```

The scan should report **no vulnerabilities**. Most findings are Go standard-library CVEs tied to the toolchain patch pinned in `go.mod`'s `go` directive — bump it to the latest `1.26.x` patch and run `go mod tidy` to resolve them.

## Configuration

Copy `.env` and fill in the values:

### Shared OAuth2 client credentials

Every upstream service client (customer entity, engineering entity, updates, SCIM) authenticates as the same OAuth2 client-credentials app — only each service's base URL and scopes differ, so the credentials are configured once and reused.

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

### Request timeouts

Go duration strings (e.g. `45s`, `2m`); unset or empty uses the default. All must be greater than 0; the service exits at startup otherwise. No ordering between them is enforced, though keeping `ENTITY_SERVICE_TIMEOUT` shorter than `REST_WRITE_TIMEOUT` lets the handler return a clean error. The defaults deliberately raise the previous values (server 30s, entity client 25s) so large inline-attachment uploads are not cut off, matching the customer-portal backend.

| Variable | Default | Description |
|---|---|---|
| `REST_READ_TIMEOUT` | `60s` | Main REST server read timeout |
| `REST_WRITE_TIMEOUT` | `60s` | Main REST server write timeout |
| `ENTITY_SERVICE_TIMEOUT` | `60s` | Per-request timeout of the customer entity service client |

### Engineering entity service (optional)

Backs `entity.EngineeringEntityClient.CreateGitIssue` (a separate internal engineering entity service). When `ENGINEERING_ENTITY_BASE_URL` is set, `POST /cases/{id}/github-issues` files the issue through it instead of forwarding to the entity service; unset, that endpoint behaves exactly as before. It uses the same shared OAuth2 credentials above (`OAUTH2_CLIENT_ID`/`_CLIENT_SECRET`/`_TOKEN_URL`) — only its base URL and scopes are its own. The same configuration also backs its `GET /health/dependencies` check (see [Health](#health) above); unset, that dependency reports `not_configured` there too.

| Variable | Description |
|---|---|
| `ENGINEERING_ENTITY_BASE_URL` | Base URL of the engineering entity service. Optional — setting it switches "Open Git issue" over to it. Must be `https` (a path is allowed, but no userinfo, query or fragment); anything else fails startup |
| `ENGINEERING_ENTITY_SCOPES` | Comma-separated OAuth2 scopes (optional) |

On this path the target comes from the case's product, not from the request: the backend reads the case, takes its product name, and looks it up with the entity service's `GET /products/github-repo` (the `product_repo_mapping` table). `repoOverride` is ignored, and a case whose product has no mapping gets a 400 (`No GitHub repository is mapped for this product.`). The mapping's `owner` is passed as both the GitHub organisation and owner (the engineering service selects its GitHub access token by that organisation name, so it must be one it is configured with). The service's response has no issue URL, so the URL returned to the web app is built as `https://github.com/<owner>/<repo>/issues/<number>`. The title (max 256 characters) and description are sent, with `updateLevel`, `publicIssueUrl` and `hotFixRequired` appended to the body. The labels are `Origin/CS`, the update level, the mapping's `githubLabel`, `Type/Patch` and `patch` for a patch, the priority for `Type/Discussion`, and `Require/Hotfix`, `regression`, `Affected/Migration` (`reason` of `migration`) and `Onboarding/affected` when they apply. After the issue is created, a work note with its URL is written on the case; that write is best-effort, so a failure is logged and the create still succeeds. Case tags are left to the web app.

### Customer-onboarding status (optional, off by default)

Backs `GET /projects/{id}/onboarding-steps` — the per-contact onboarding status the CSM Portal's project Contacts tab shows (which of IDENTITY / DATABASE / EMAIL / REGISTRATION succeeded, failed or was skipped for each invited email, with the attempt count and last error). It reads the entity service's onboarding ledger (`POST /onboarding-steps/search`) through the existing `CustomerEntityClient`; no extra URL or credential is needed.

| Variable | Description |
|---|---|
| `CSM_MIGRATION_ONBOARDING_STATUS_ENABLED` | Exactly `true` registers the route. Unset, empty or any other value (including `1`, `TRUE`, `yes`) keeps it off: the route is not registered (the path 404s) and nothing else in the backend changes. Stricter than the `strconv.ParseBool` parsing `SFTPGO_*` uses on purpose — every `CSM_MIGRATION_*` flag is a cutover switch |

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

### csm-notification-service / csm-integration-service (health check only)

Both optional — used only to back `GET /health/dependencies` today (see "Health" under [API Endpoints](#health) above). Unset leaves that dependency reported as `not_configured` rather than failing startup. Uses the shared `OAUTH2_*` credentials above.

| Variable | Description |
|---|---|
| `CSM_NOTIFICATION_SERVICE_BASE_URL` | Base URL of `integrations/csm-notification-service`. Optional |
| `CSM_NOTIFICATION_SERVICE_SCOPES` | Comma-separated OAuth2 scopes (optional) |
| `CSM_INTEGRATION_SERVICE_BASE_URL` | Base URL of `integrations/csm-integration-service`. Optional |
| `CSM_INTEGRATION_SERVICE_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Notifications — email channel (not yet wired in)

`internal/notifications` (`EmailClient.SendEmail`) is ready to use but is not constructed in `cmd/server/main.go` — no handler calls it yet. These variables are not read by any code today; they're documented here for when the first caller is added, which should reuse the shared `OAUTH2_*` credentials above rather than adding its own. Each notification channel gets its own `NOTIFICATIONS_<CHANNEL>_*` prefix for its channel-specific settings — SMS/Twilio will follow this same convention once added.

| Variable | Description |
|---|---|
| `NOTIFICATIONS_EMAIL_BASE_URL` | Base URL of the email notification service (optional) |
| `NOTIFICATIONS_EMAIL_SCOPES` | Comma-separated OAuth2 scopes (optional) |
| `NOTIFICATIONS_EMAIL_FROM_ADDRESS` | Fixed "From" address used for every outgoing email (optional) |

### Notifications — Google Chat channel

`internal/notifications` (`GoogleChatClient.SendIncidentAlert`) posts a card message — title, short description, and an "Open in CSM Portal" button — to a Google Chat space via an incoming webhook. There's one space per product (each WSO2 product has its own space), so the client is configured with a list of `{product, webhookUrl}` pairs and routes each alert to the space matching the case's product (case- and whitespace-insensitive match; an unconfigured product returns an error rather than falling back). Unlike every other upstream client it does not use the shared `OAUTH2_*` credentials; a webhook URL is the only credential needed per space (Space settings > Apps & integrations > Webhooks). It's called from `POST /notifications/google-chat/alerts` (see [API Endpoints](#notifications) below), which today is triggered manually rather than from real case/incident creation.

| Variable | Description |
|---|---|
| `NOTIFICATIONS_GOOGLE_CHAT_SPACES` | JSON array of `{"product","webhookUrl"}` objects, one per Google Chat space — e.g. `[{"product":"api-manager","webhookUrl":"https://chat.googleapis.com/..."}]`. Optional — left unset, malformed, Google Chat alerts are unavailable but startup and every other endpoint work normally |
| `CSM_PORTAL_WEB_BASE_URL` | Base URL of the CSM portal webapp, used to build the "Open in CSM Portal" link at `/operations/incidents/{caseId}` (e.g. `http://localhost:3001` for local dev). Optional — only needed alongside `NOTIFICATIONS_GOOGLE_CHAT_SPACES` above |

### Dashboards

Dashboard definitions are files, one JSON file per dashboard, read once at startup and held in
memory. The filename is irrelevant — `id`, `displayName` and `type` come from the file's own
content. A malformed, unreadable or duplicate-`id` file **fails startup naming the file** rather
than being skipped: a silently dropped dashboard is invisible. See `dashboards.example/` for the
schema.

| Variable | Description |
|---|---|
| `DASHBOARDS_DIR` | Directory holding one `*.json` file per dashboard. `.env.example` ships `./dashboards.example` so a fresh clone starts; for a real set, `cp -r dashboards.example dashboards` (`./dashboards` is gitignored) and point this at it. A missing directory is fatal |
| `DASHBOARDS_HOT_RELOAD` | Re-read `DASHBOARDS_DIR` on every request instead of serving the startup snapshot. Parsed with `strconv.ParseBool`, so `1`/`t`/`true`/`yes`-style values are not interchangeable — `1`, `t`, `T`, `TRUE`, `true`, `True` are true, and an unparseable non-empty value logs a warning and is treated as false. **Local development only**; default false |
| `DASHBOARDS_CONFIG` | **Deprecated.** The whole registry crammed into one JSON array variable. Honoured only when `DASHBOARDS_DIR` is unset, and warns when used. Malformed content is fatal |

A dashboard definition may set `"restricted": true` — then only a caller holding the `cs_engineer`
or `admin` role can see it: `GET /dashboards` leaves it out of the list for everyone else, and
`GET /dashboards/{id}` returns `403` for a direct request to its id. Every other role sees only the
unrestricted dashboards. Unset (the default, `false`) means every portal role can see it, same as
before this field existed. This is enforced by `handler.DashboardHandler` itself, not by the route's
own permission (`GET /dashboards`/`GET /dashboards/{id}` both stay `PermView` — the list route must
still run for every viewer and only filter its result, not reject the whole request).

### Directory vocabularies

Two curated lists are supplied as configuration rather than code, so adding a team or a role is a
config change and a restart, not a release. Both are parsed at startup: **a malformed value is
fatal**, so a typo stops a deploy instead of silently emptying a page. They previously lived in
`entity-service`; that service no longer reads them.

| Variable | Description |
|---|---|
| `CSM_TEAM_REGISTRY` | Team catalogue. `teamKey\|Display Name\|FAMILY\|groupId` rows separated by `,`; `FAMILY` and `groupId` are optional. Optional overall — unset means no teams (startup warns) |
| `CSM_USER_ROLES` | Assignable-role allow-list, comma-separated. Optional; unset uses the built-in list |
| `ASGARDEO_ROLE_IDS` | Real role name → identity-provider role id mapping, a JSON object string (e.g. `{"example-timecard-approver-role":"11111111-1111-1111-1111-111111111111"}`). Keyed by the role name exactly as it appears in `AUTH_<ROLE>_ROLES`, not a portal-role key. Optional overall and per-name — a real name with no entry here just means that role has no SCIM-backed feature wired up (e.g. `GET /users/time-card-approvers` 404s, and that portal role is absent from `GET /roles/grantable`) |

```bash
# FAMILY is one of CRE-ABT, CRE, SRE-ABT, SRE (case insensitive). Any other
# family value — or a duplicate team key or display name — fails startup
# naming the offending row.
# The names below are placeholders — supply the real ones per environment.
CSM_TEAM_REGISTRY="alpha|Alpha Team|CRE-ABT,beta|Beta Team|SRE-ABT,gamma|Gamma Team"

CSM_USER_ROLES="agent,admin,commenter,customer,customer_admin,partner,partner_admin,internal,external,timecard_approver"
```

`CSM_TEAM_REGISTRY` has **no default, by design**. Team names are organisation vocabulary and are
deliberately not committed to this repository — only placeholders appear here and in
`.env.example`. Unset, the registry is empty and team lookups return nothing, with a warning
logged. `Display Name` is matched verbatim against the backing data source's group name when
resolving members, so a wrong or blank one resolves **zero members silently**; that is why an empty
field is rejected outright. The registry is resolved into an in-memory index at startup. `POST
/teams/search` lists the entity service's `team` table (`POST /teams/search` there) and enriches
each team whose name matches a registry row with that row's key, family and group ids; a request
with a `family` filter is answered from the registry alone, since family is not stored on the
`team` table.

`CSM_USER_ROLES` does have a default, because role names are generic platform vocabulary rather
than organisation-specific. It drives both the `roleIds` filter validation and the catalogue that
`POST /roles/search` serves, so the picker and the filter cannot disagree.

### Auth

| Variable | Description |
|---|---|
| `AUTH_JWKS_ENDPOINT` | JWKS endpoint used to verify JWT signatures |
| `AUTH_ISSUER` | Expected `iss` claim value |
| `AUTH_AUDIENCE` | Comma-separated accepted `aud` values; token passes if any listed value is present in its `aud` claim |
| `AUTH_TOKEN_VALIDATOR_ENABLED` | Set to `false` for local development to skip signature verification (default `true`) |

### Access control

A valid token proves who the caller is; the **roles on the token** decide what they may do. The
`roles` claim of the validated `x-jwt-assertion` is checked against the role names configured for
each role — no upstream call is made. Every route is registered in `cmd/server/main.go` through
`route(pattern, permission, handler)`, which takes the permission as a required argument, so a new
route cannot be added without choosing one.

Each portal role's token role names are configuration. The variable holds a comma-separated list; holding **any
one** of the listed roles grants the role. Matching is exact and case-sensitive. There is
**no default**: the names are organisation vocabulary and are not committed here, so a role whose
variable is unset or empty is held by nobody (startup logs a warning naming each one). With none
configured at all, nobody can use the portal.

| Variable | Grants |
|---|---|
| `AUTH_VIEWER_ROLES` | view |
| `AUTH_ESCALATOR_ROLES` | view, escalate (`cs_engineer` and `admin` also escalate; de-escalating additionally requires being one of the case's ABT team leads) |
| `AUTH_ATTACHMENT_DOWNLOADER_ROLES` | view, download_attachment |
| `AUTH_SUPPORT_ENGINEER_ROLES` | view, view_operations, time_cards_and_updates, download_attachment, write (which includes posting comments), security_center — everything except `admin`-only routes and approving a time card (a dedicated responsibility held only by its own role plus admin). Includes escalating a case, as in ServiceNow, where any internal engineer may escalate. Grants the `cs_engineer` portal role (renamed from `support_engineer`; the env var name was deliberately left as-is to avoid a coordinated deployment config change) |
| `AUTH_ADMIN_ROLES` | everything, including `admin`-only routes no other role holds |
| `AUTH_USAGE_METRICS_VIEWER_ROLES` | view |
| `AUTH_TIMECARD_APPROVER_ROLES` | view, time_cards_and_updates, and approving/rejecting a time card (`PATCH /time-cards/{id}` with `state` set — `cs_engineer` does NOT grant this) |
| `AUTH_DASHBOARD_DESIGNER_ROLES` | view |
| `AUTH_WORKNOTE_CREATOR_ROLES` | posting a `work_note`-type comment on a case only (`POST /cases/{id}/comments`) — not a customer-visible reply, and none of `write`'s other actions. `cs_engineer`/`admin` already grant this via `write`; a plain `viewer` does NOT (it is read-only), so give this role to anyone who should add work notes without full write; unset is a normal, supported state (like `AUTH_SALES_SOLUTIONS_ROLES`), not a misconfiguration — startup does not warn about it |

A worknote-creator- or escalator-only caller is provisioned a platform `"user"` record
on first use rather than needing to go through the admin "Add User" flow first: before
posting a work note or creating/removing a case escalation, this backend checks whether
the caller already has one (`GET /users/me`) and, if not, creates it from the caller's
own token (`given_name`/`family_name`/`email`), granted the `internal` role. A
`cs_engineer`/`admin` caller is assumed already provisioned and skips this check.

```bash
# Several token roles can grant one portal role; any one is enough.
AUTH_ESCALATOR_ROLES=example-escalators-role,example-leads-role
```

| Permission | Routes |
|---|---|
| authenticated | `GET`/`PATCH /users/me` — any valid token, no role needed, so a user holding no portal role can still load their profile and be shown a "no access" screen |
| `view` | every other `GET`, `*/search` and `*/aggregate` |
| `view_operations` | the same reads under `/incidents`, `/change-requests`, `/problems`, `/incident-tasks`, `/outages`, `/alerts` and `/smart-alerts` — CS engineer and admin only, so a view-only role sees cases and customers but not Operations |
| `time_cards_and_updates` | every time-card route (`POST /time-cards/search`, `POST /time-cards`, `PATCH`/`DELETE /time-cards/{id}`) and the update-level lookups (`GET /updates/product-update-levels`, `POST /updates/levels/search`) — CS engineer, admin and time-card approver only, so a view-only role sees neither area. Approving/rejecting a time card (a `state`-carrying `PATCH /time-cards/{id}`) additionally requires the separate `approve_time_card` permission below, held only by time-card approver and admin |
| `approve_time_card` | `PATCH /time-cards/{id}` when the body sets `state` (approve/reject) — time-card approver and admin only, checked by inspecting the body inside the shared handler, not a route permission of its own |
| `escalate` | `POST /cases/{id}/escalations` — cs_engineer, escalator and admin; de-escalating also requires being one of the case's ABT team leads |
| `download_attachment` | `GET /attachments/{id}/content`, `POST /attachments/{id}/share` |
| `write` | every other `POST`/`PATCH`/`DELETE`, including case, incident and change-request comments — except the `admin`-only routes below |
| `admin` | `POST /users` (create a new platform user), `GET /roles/grantable` (which portal roles that endpoint can grant) — held by the `admin` role alone; `cs_engineer` does not grant it |
| `security_center` | `POST /products/vulnerabilities/search`, `GET /products/vulnerabilities/{id}`, plus security-report cases (a `POST /cases/search`/`GET /cases/{id}` request naming case type `security_report_analysis`) — `cs_engineer` and `admin` only, even though every other role also holding `view` can otherwise read cases and products freely. See `CaseHandler.WithAccessGuard`'s own doc comment for why `GET /cases/{id}` cannot enforce this per-case (the response's `type` field is null on the Postgres data source) |

A caller whose token holds none of the required roles gets `403`. Escalation and
attachment-download are separate from `cs_engineer` so other staff can be granted just that one
ability. Posting a public case comment still additionally requires being the case's assigned
engineer (see `CreateCaseComment`); the role is necessary, not sufficient.

`GET /users/me` returns `roles` — which portal roles the caller holds, as stable keys (`viewer`,
`escalator`, `attachment_downloader`, `cs_engineer`, `usage_metrics_viewer`,
`timecard_approver`, `dashboard_designer`, `admin`). A caller can hold several; it is `[]` for a caller
holding no portal role. It comes from the same guard that authorises the routes, so what the frontend
is told and what the backend enforces cannot disagree. This is the portal roles only: the entity
service's own role data is no longer returned. The frontend decides what to show or hide from these
roles; the backend's `403` is the real gate. `GET /users/{id}` reports the same vocabulary for an
internal target (see that endpoint's own entry below) — sourced from SCIM instead of a JWT, since
this endpoint is looking at someone *other* than the caller.

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
│   │   ├── onboarding.go        # CustomerEntityClient.SearchOnboardingSteps — typed onboarding-ledger search
│   │   └── engineering.go       # EngineeringEntityClient — CreateGitIssue (wired when ENGINEERING_ENTITY_BASE_URL is set)
│   ├── scim/
│   │   └── client.go           # OAuth2 HTTP client for the SCIM operations service
│   ├── updates/
│   │   ├── client.go           # OAuth2 HTTP client for the updates service
│   │   └── updates.go          # Updates service operations
│   ├── csmnotification/
│   │   └── client.go           # OAuth2 HTTP client for csm-notification-service (health check only)
│   ├── csmintegration/
│   │   └── client.go           # OAuth2 HTTP client for csm-integration-service (health check only)
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
│       ├── onboarding_steps.go           # GET /projects/{id}/onboarding-steps (behind CSM_MIGRATION_ONBOARDING_STATUS_ENABLED)
│       ├── incidents.go                  # HTTP handlers for incident endpoints (ServiceNow only)
│       ├── problems.go                   # HTTP handlers for problem endpoints (ServiceNow only)
│       ├── updates.go                    # HTTP handlers for updates endpoints
│       ├── health.go                     # GET /health/dependencies — aggregating dependency health check
│       └── users.go                      # HTTP handlers for user endpoints
├── .env                        # Local config (git-ignored)
└── go.mod
```

## API Endpoints

### Health

- `GET /health` — Liveness probe; always `200`, no dependency calls. Wire this up as the restart/drain-triggering probe
- `GET /health/dependencies` — Aggregating dependency check: SCIM Service, Updates Service, CSM Notification Service, CSM Integration Service and Engineering Entity Service (each independently optional except SCIM/Updates). This backend's own core entity-service is not checked here. `200` when every checked dependency is `ok`, `503` if any is `down`. Do **not** wire this one up as a liveness/restart probe — see [Configuration](#configuration) and `internal/handler/health.go`'s own doc comment for why the two are kept separate

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
- `POST /cases/{id}/github-issues` — Create a GitHub issue from a case. By default forwarded to the entity service (`reason` selects the target repo — `default`/`migration`/`rd_ticket`; ServiceNow only); with `ENGINEERING_ENTITY_BASE_URL` set it is filed through the engineering entity service instead (see above)
- `GET /products/github-repo?name=` — The GitHub repository mapped to a product, proxied to the entity service's `product_repo_mapping` lookup. The "Open Git issue" dialog shows it; 404 when no repository is mapped

### Users

- `GET /users/me` — Get current user profile (`id`, `email`, `firstName`, `lastName`, `timeZone` from entity service; `roles` are the portal roles granted by the caller's token; `phoneNumber` from SCIM)
- `PATCH /users/me` — Update current user profile (`phoneNumber` via SCIM, `timeZone` via entity service)
- `POST /users/search` — Search users; optional `filters` (`searchQuery`, `roles`, `userNames`, `emails`, `active`) and `sortBy` (`field`, `order`); response shape depends on data source (`User` for postgres, `SNUser` for ServiceNow)
- `GET /users/{id}` — Get one user's full profile (both data sources); adds `teams` (derived from `groups`) and, for external contacts only, `externalAccount` (`exists`/`locked`, from SCIM's "external" org search). For a wso2.com-email target (regardless of its recorded `userType`), also adds `csmPlatformRoles` — the same portal-role vocabulary `GET /users/me` reports (`viewer`/`escalator`/.../`admin`), sourced from that user's own SCIM role assignment (filtered to this portal's `app-csm-*` roles) — alongside, not replacing, the response's own `roles` field (entity-service's own role vocabulary, a genuinely different thing). All enrichments (teams, externalAccount, csmPlatformRoles) are best-effort — absent/unchanged rather than failing the request if their lookup fails
- `POST /users` — Create a new user (`firstName`, `lastName`, `email` required to have at least one of firstName/lastName; optional `roles`, validated against the configured role allow-list). **Admin-only** (`admin` permission — see "Access control" above); Postgres data source only. `roles` is also how a caller sets the new user's type (entity-service derives `userType` from role membership) — the Add User form sends exactly one of `internal`/`external`. Granting `internal`/`admin` for a non-`@wso2.com` email is rejected with 400. A separate, unrelated optional field, `grantRoles` (portal role keys, e.g. `["cs_engineer"]`), grants each via SCIM once the user exists — an unknown key is a 400, a SCIM-side failure is logged but does not fail the create (see "Granting portal roles on user creation" in `CLAUDE.md`)
- `GET /users/time-card-approvers` — Lists the real Asgardeo membership (`{id, email}` per member, merged and deduplicated across every real role name `AUTH_TIMECARD_APPROVER_ROLES` configures an id for) of the `timecard_approver` role, via the SCIM operations service — authoritative over, and potentially different from, entity-service's own Postgres `role`/`user_role` tables. Returns 404 unless `ASGARDEO_ROLE_IDS` configures at least one `timecard_approver` real role name; a SCIM 401/403 (e.g. a missing roles-read scope on this backend's own SCIM credentials) is reported as 502, never passed through as the caller's own 401/403 (see "Listing time card approvers via SCIM" in `CLAUDE.md`)
- `GET /roles/grantable` — Lists the portal role keys `POST /users`' own `grantRoles` field can grant in this deployment (just the key, e.g. `cs_engineer` — never the real role name/id behind it). **Admin-only**, same gate as `POST /users` itself

### Accounts

- `GET /accounts/{id}` — Get account by ID; response takes one of two shapes: `Account`, or `AccountDetail` (`supportTier` as `{id, label}`, `owner`/`technicalOwner` as `{id, name}`)
- `POST /accounts/search` — Search accounts; optional `filters` (`searchQuery`, `active`, `pod`, `classification`); response takes one of two shapes: `AccountSearchResponse`, or `AccountViewSearchResponse` (`supportTier` as a label string, `owner`/`technicalOwner` as `{id, name}`)

### Projects

- `GET /projects/{id}` — Get project by ID
- `POST /projects/search` — Search projects
- `GET /projects/{id}/onboarding-steps` — Customer-onboarding steps of the project's contacts, grouped per membership (invited email) in flow order; only registered when `CSM_MIGRATION_ONBOARDING_STATUS_ENABLED=true` (see Configuration)

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
- `POST /conversations/search` — Search conversations; optional `filters` (`projectIds`, `states` (`ACTIVE`/`RESOLVED`/`CONVERTED`/`ABANDONED`/`CLOSED`), `searchQuery`, `number` (exact match), `createdByMe`, `createdBy` (list of creator emails)) and `sortBy` (`field`: `createdOn`/`updatedOn`, `order`) (ServiceNow data source only)

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
