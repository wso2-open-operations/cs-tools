# CSM Portal Backend

Go HTTP server (`net/http`, Go 1.26+) that acts as a backend-for-frontend (BFF) for the CSM portal. It authenticates callers, forwards requests to upstream services, and shapes responses for the frontend.

## Middleware chain

`SecurityHeaders → CorrelationID → Auth → Logger → Mux`

- `SecurityHeaders` (`internal/middleware/security_headers.go`): sets `X-Content-Type-Options: nosniff`, `Content-Security-Policy: upgrade-insecure-requests`, and `Strict-Transport-Security: max-age=31536000; includeSubDomains` on every response; outermost so headers are present even on auth failures
- `CorrelationID` (`internal/middleware/correlation.go`): reads `X-CSM-Correlation-ID` from the incoming request or generates a UUID v4; stores the ID in context for the slog handler and for the entity client to forward; echoes the ID in the response header
- `Auth` (`internal/middleware/auth.go`): validates the `x-jwt-assertion` JWT and sets `UserInfo` in context. When `TokenValidatorEnabled` is true, the JWKS fetch runs through `x5cStrippingTransport`, which strips the `x5c` field from every key before `MicahParks/jwkset` parses the response — some IdPs (Asgardeo included) publish JWKS certs with a negative serial number, which Go's `crypto/x509` rejects since Go 1.23 and would otherwise make the whole JWK Set fail to load, even though verification only needs `n`/`e`. In Choreo deployments `TokenValidatorEnabled` is false (the gateway validates the JWT upstream), so this path only runs in local dev.
- `Logger` (`internal/middleware/logger.go`): logs every completed request (method, path, status, elapsed) via slog; runs after Auth so both `correlationID` and `userID` are present in every record

`middleware.ConfigureLogger()` must be called at startup — it wraps the default slog handler so that every `slog.*Context(r.Context(), …)` call anywhere in the codebase automatically includes `correlationID=<id>` when the context carries one.

## Access control (roles)

`Auth` only proves identity. Authorisation is `handler.AccessGuard` (`internal/handler/access.go`): it checks the caller's `roles` claim — decoded into `middleware.UserInfo.Roles` — against the role names configured per role. It makes **no upstream call**; a check is a set lookup. (Do not switch it to the entity service `GET /users/me` role data: that adds an upstream call (and a cache to offset it) per request for no gain.)

- **Per-role config.** Each role has an `AUTH_<ROLE>_ROLES` env var (comma-separated role names, any one grants the role), read once at startup by `loadAccessConfig` in `cmd/server/main.go`. There is deliberately **no default** (role names are organisation vocabulary and must not be committed, same as `CSM_TEAM_REGISTRY`): an unset/empty variable means nobody holds that role, and startup warns naming each one. Tests use dummy names via `testAccessConfig()` in `access_test.go`, never real ones. Matching is exact and case-sensitive. Adding a role means a new `AccessConfig` field, its env var, and its place in `NewAccessGuard`.
- **The `roles` claim is a string for one role and an array for several** (Asgardeo does this). `middleware.stringList` accepts both; a plain `[]string` would reject a single-role user's whole token with a 401. Any other shape fails the token.
- **Every route goes through `route(pattern, perm, handler)` in `cmd/server/main.go`.** `perm` is a required argument with no default — pick `PermView` for reads/searches/aggregates, `PermViewOperations` for reads under the Operations area (incidents, change requests, problems, incident tasks, outages, alerts), `PermTimeCardsAndUpdates` for every time-card route and the update-level lookups (CS engineer, admin and time-card approver), `PermWrite` for other state changes, `PermAdmin` for the handful of actions reserved for admin alone (currently just `POST /users`, creating a new platform user), `PermViewSecurityCenter` for Security Center (both `/products/vulnerabilities/*` routes; see "Security Center access" below for how `/cases/*` is handled), or one of the narrower ones (`PermEscalate`, `PermDownloadAttachment`). Case, incident and change-request comments are `PermWrite`. `PermAuthenticated` (no role needed) is only for the caller's own `/users/me`. `/health` is the only route registered directly on the mux.
- **The policy is `NewAccessGuard`.** `admin` satisfies every permission, including `PermAdmin`; `cs_engineer` satisfies every other route permission (view, escalate, download, write, including comments, security center) but **not** `PermAdmin` — that is the one permission admin does not share with cs_engineer; `escalator`/`attachment-downloader` grant only their one ability plus view (so a view-only role cannot read Operations — `PermViewOperations` is CS engineer and admin only, matching the frontend's `canUseOperations`); `timecard-approver` grants view plus `PermTimeCardsAndUpdates`; `usage-metrics-viewer`/`dashboard-designer` grant only view here (no backend route for those features); the frontend gates them. Every role implies view, **except** `PermViewSecurityCenter` — a plain viewer/escalator/attachment_downloader/usage_metrics_viewer/timecard_approver/dashboard_designer holds `PermView` but not this.
- **`GET /users/me` reports `roles`** (from `AccessGuard.RolesFor`): the stable keys of the portal roles the token roles grant — several possible, fixed order — and is **not** the entity service's role data, which the response no longer carries. The frontend decides what to show or hide from these roles (there is deliberately no derived `permissions` list); the backend's `403` is the real gate. Dashboard-designer access is only the `AUTH_DASHBOARD_DESIGNER_ROLES` role (the old `DASHBOARD_DESIGNER_EMAILS` email allow-list is gone).
- The role check is separate from resource-level guards inside handlers (e.g. the assigned-engineer check on public case comments) — both must pass.
- **`PermViewAllDashboards` (CS engineer and admin) is the one permission with no route.** A `dashboard.Dashboard` may set `"restricted": true`; `DashboardHandler` calls the newly-exported `AccessGuard.Permits(perm, roles)` itself to filter it out of `GET /dashboards` and 403 a direct `GET /dashboards/{id}` for anyone else, while both routes stay `PermView` so the list route still runs for every viewer. `Permits` exists specifically for this — a handler that must gate one specific resource against a caller's roles from inside its own logic, not a whole route via `Require`.
- Handler tests call handlers directly and bypass the guard; test the guard in `access_test.go`. `dashboards_test.go`'s `withRestrictedDashboard` temporarily swaps the package-level `dashboard.Active()` registry for one that adds a `Restricted` dashboard, restored via `t.Cleanup` — every other test in that file depends on the shared two-dashboard fixture's exact count, so never add a permanent third dashboard to it.

## Creating a user (POST /users)

Admin-only (`PermAdmin`, see "Access control" above) — the CSM portal's Add User UI is the one
caller. `UsersHandler.CreateUser` (`internal/handler/users.go`) validates `roles` against
`Directory.IsValidRole` (the same startup-resolved `CSM_USER_ROLES` allow-list `POST /roles/search`
serves) before forwarding the request body unchanged to the entity service's own `POST /users` —
entity-service deliberately does not validate role names itself (see that repo's own `domain.UserRole`
doc comment), so this is the one place that does. `roles` is optional and currently unused by the
frontend (no role-picker UI yet, since there is no Asgardeo-backed way to browse/assign roles at
account-creation time today) — the field exists end-to-end and works if sent, it's just not wired
into the Add User form yet.

## Security Center access (PermViewSecurityCenter)

Security Center (the webapp's Security reports + Vulnerabilities tabs) is restricted to `cs_engineer`
and `admin` only — every other role, even though it holds `PermView`, is denied. This needed two
different mechanisms, because the feature isn't backed by its own exclusive routes:

- **`/products/vulnerabilities/search` and `/products/vulnerabilities/{id}`** are genuinely
  Security-Center-exclusive, so they're gated the ordinary way: `route(..., handler.PermViewSecurityCenter, ...)`
  in `cmd/server/main.go`.
- **`POST /cases/search`** is the shared, generic case-search endpoint every case-type tab uses
  (Support, Operations sub-tabs, Engagements, Security reports) — it stays registered at `PermView`,
  since narrowing that route-level permission would lock out every other tab too. Instead,
  `CaseHandler.SearchCases` inspects the request body itself: `caseSearchTargetsSecurityReports`
  (`internal/handler/cases.go`) reads the generic filter expression (`filters.filters[]`, and each
  `filters.anyOf[]` branch) for a `{field: "type", op: "in", values: [...]}` predicate naming
  `security_report_analysis`, and if one is found, additionally requires `PermViewSecurityCenter` via
  `CaseHandler.access` (wired with `WithAccessGuard`, the same pattern `UsersHandler` uses) —
  a plain `PermView` caller gets 403 instead of the search running. This only catches an *explicit*
  request for that type, the same way Security Center's own `caseTypes`-locked search
  (`CsmIssuesView`, webapp) always sends one; a hypothetical unfiltered "every case type" search that
  happens to also return security-report rows is a known, narrower gap, not handled here.
- **`GET /cases/{id}` has no equivalent check, deliberately.** `CaseView.type` (entity-service's own
  `openapi.yaml`) is only populated for ServiceNow cases — null on Postgres — so there is no reliable
  way for this handler to tell a security-report case apart from any other by inspecting the response
  alone, and a check that silently does nothing on one data source would be worse than no check at
  all (it would look like protection without being any). See `CaseHandler.WithAccessGuard`'s own doc
  comment for the full reasoning. Practically: since `SearchCases` is now locked down, a non-`cs_engineer`/
  `admin` caller can no longer *discover* a security-report case's id through the portal at all — the
  residual gap is a caller who already has one (a pre-existing bookmark, or a guess) fetching it
  directly by id. Closing that fully needs entity-service itself to resolve and enforce it (it has
  reliable type data on either data source), not this BFF layer.

## Upstream service modules

Each upstream service has its own client package under `internal/`:

| Package | Upstream | Notes |
|---------|----------|-------|
| `entity` | Multiple entity services (see below) | Hosts `CustomerEntityClient` (this repo's entity-service; most case/account/project endpoints, raw `[]byte` passthrough) and `EngineeringEntityClient` (a separate internal engineering entity service; `CreateGitIssue`, typed request/response). `EngineeringEntityClient` is constructed in `cmd/server/main.go` only when `ENGINEERING_ENTITY_BASE_URL` is set, and then `CaseHandler.CreateCaseGithubIssue` uses it (via `WithEngineeringClient`) instead of the entity service: the target must be a `GITHUB_ISSUE_REPO_OPTIONS` entry, and unlike the entity service's version it does not write the issue URL back to the case or tag a regression |
| `scim` | SCIM service | User/group lookups. Two orgs: `SearchUser` queries the "internal" org (WSO2 staff — phone number, last password update). `SearchExternalUser` queries the "external" org (customer/partner contacts — existence + lock status, mirroring `infra-operations/operations/asgardeo-user-check`'s `{exists, locked}` contract). `GetUser` calls the latter only when the entity response's `userType` isn't `internal`, and treats a lookup failure as best-effort — logged, response returned unchanged, never a failed request |
| `updates` | Updates service | Product update levels; returns typed structs (not raw passthrough) |

New upstream services get their own package under `internal/` following the same `Client` + `do()` pattern. `entity` is the exception: because it hosts multiple, separately-deployed/differently-authenticated services, it uses a `<Name>Config`/`<Name>Client` pair per service/file instead of one shared `Client` for the whole package — `CustomerEntityClient`/`EngineeringEntityClient`.

**Notifications (email, Google Chat, and future channels like SMS/voice-Twilio) have moved out of this backend** into `integrations/csm-notification-service`, a standalone Go service invoked over HTTP. This backend no longer constructs or calls any notification client directly, and no longer publishes domain events to Azure Event Hub either — that responsibility (case.created/case.acknowledged/case.severity_changed/incident.created, and the like) now belongs entirely to `entity-service`, the one publisher of the `case-events` topic. This backend previously had its own producer-side pipeline (`internal/eventbus`/`internal/events`/`internal/eventpublisher`, publishing `case.comment_added`/`case.status_changed` from `CaseHandler`) — it was removed entirely to avoid two independent services writing to the same topic. Per-recipient portal-link resolution (which of the customer/CSM portal a given recipient's email lands on) **does not live in this backend** either — it's in `csm-notification-service`'s own `internal/recipientlinks`/`internal/entity`, since that's the service actually composing and sending the email.

**Shared OAuth2 credentials**: `CustomerEntityClient` and `EngineeringEntityClient` (both in `entity`), `updates`, `scim`, and PLG's `internal/plg/entityclient` all authenticate as the same OAuth2 client-credentials app — `cmd/server/main.go` reads `OAUTH2_CLIENT_ID`/`OAUTH2_CLIENT_SECRET`/`OAUTH2_TOKEN_URL` once and passes them into every service's `Config`; only `<SERVICE>_BASE_URL`/`<SERVICE>_SCOPES` are per-service. `EngineeringEntityConfig` still has its own `ClientID`/`ClientSecret`/`TokenURL` fields (matching every other service's `Config` shape), but when it's wired into `main.go` those should be filled with the same shared `oauth2ClientID`/`oauth2ClientSecret`/`oauth2TokenURL` values, not new `ENGINEERING_ENTITY_*` env vars. Follow this pattern for any new upstream service client unless it genuinely uses a different OAuth2 app. PLG is the one client that **inherits** rather than being passed a `Config` per field: `main.go` hands `plg.Mount` a `plgconfig.EntityDefaults` carrying `CUSTOMER_ENTITY_BASE_URL` and the three shared `OAUTH2_*` values, because PLG reaches the same entity-service as `CustomerEntityClient` and a second copy of those settings is one that goes stale on the next secret rotation. `PLG_ENTITY_BASE_URL`/`PLG_ENTITY_OAUTH_*` exist as overrides and are normally unset.

## Running locally

```bash
# from apps/csm-portal/backend
go run ./cmd/server/main.go
```

The server auto-loads `.env` from the working directory at startup (silently ignored if absent). No need to `source .env` manually.

`cp .env.example .env` is enough to start: `DASHBOARDS_DIR` points at the committed `dashboards.example/` (a missing directory is fatal). For a real dashboard set, `cp -r dashboards.example dashboards` and repoint it — `./dashboards` is gitignored.

`CSM_TEAM_REGISTRY` (team vocabulary: `teamKey|Display Name|FAMILY|groupId` rows, comma separated) and `CSM_USER_ROLES` (assignable-role allow-list) are read **here**, not in `entity-service` — they moved. Both are resolved once at startup into `internal/directory`, so `POST /teams/search` and `POST /roles/search` make no upstream call. A malformed row, an unknown family, or a duplicate team key/display name is fatal at startup, naming the row. Both are flat single-line strings on purpose: the deployment platform's configuration UI is one-dimensional and stringifies nested collections, so a structured registry cannot be deployed at all. Do not introduce nested-collection configuration.

## Commands

```bash
make setup   # wire up git hooks (once after clone)
make test    # vet + race-detector tests
make build   # runs tests then compiles ./cmd/server
```

Tests run automatically on `git push` via the pre-push hook.

## Adding a new endpoint

Follow these steps in order:

1. **Upstream client** (`internal/<module>/`) — add a method on `Client` that calls `c.do()`; use `url.PathEscape()` for every path parameter
2. **Handler interface** — extend the local interface in the relevant handler file (e.g. `entityCaseClient` in `cases.go`); keep it minimal — only methods that handler actually calls
3. **Handler func** — auth check → path/body guards → call client → `mapUpstreamErrorGeneric` on failure (see Handler conventions below for the one PATCH-handler exception) → write response
4. **Route** (`cmd/server/main.go`) — register with `route(...)` using Go 1.22 method-prefixed patterns and the permission the caller needs (see Access control): `route("POST /cases/{id}/comments", handler.PermComment, caseHandler.CreateCaseComment)`
5. **OpenAPI spec** (`openapi.yaml`) — add the path with 200/400/401/403/404/500 responses; `403` is always required because `mapUpstreamError`/`mapUpstreamErrorGeneric` can return it
6. **Tests** — add handler tests; update the mock in `helpers_test.go` to satisfy the extended interface
7. **gosec** — run `gosec -fmt=text ./...` (see README's Security Scanning section) before opening the PR; it must report 0 issues

## Handler conventions

- **Auth**: always check `middleware.UserInfoFromContext(r.Context()) == nil` first → 401
- **Body size**: cap with `http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)` (1 MiB) before reading
- **Path params**: guard against empty string after `r.PathValue("id")`; if the param is a UUID, also validate format using the package-level `uuidRe` compiled regex and return 400 on mismatch — fail fast before calling the upstream
- **Field naming**: case create/patch use bare names without `Key`/`Keys` suffix — `state`, `severity`, `workState`, `type`, `engagementType`, `catalogId`, `catalogItemId`, `variables` (PATCH — the latter four only meaningful transferring into `engagement`/`service_request` respectively), `type`, `severity`, `issueType` (POST); search filters use `states`, `severities`, `types`, `issueTypes`, `engagementTypes`; deployment search uses `deploymentTypes`; case comments use `type` (not `typeKey`); case create accepts `type: "case"`, `"service_request"`, or `"security_report_analysis"` (ServiceNow only for the latter two); case-type transfer via PATCH accepts `type: "case"`, `"engagement"`, `"security_report_analysis"`, or `"service_request"` (ServiceNow only)
- **Deployment ID injection**: two helpers exist in `deployments.go` — `injectDeploymentID` (injects `deploymentIds: [id]` array, used by search) and `injectDeploymentIDField` (injects `deploymentId: id` string, used by create/update). Use the correct one for the endpoint's upstream contract.
- **Upstream errors**: use `mapUpstreamErrorGeneric(w, err, "<fallback message>")` for every endpoint by default — never write custom status mappings inline. Only the ten PATCH/update handlers (`PatchCase`, `PatchCallRequest`, `PatchMe`, `UpdateProject`, `PatchDeployment`, `PatchDeployedProduct`, `PatchChangeRequest`, `UpdateTimeCard`, `UpdateTask`, `PatchIncident`) use `mapUpstreamError` instead, which surfaces the upstream 400/409/422 reason (e.g. "Invalid state transition") — appropriate there because the request body just submitted is what's being rejected. Every other endpoint (search/create/get/delete) forwards a payload that's only partially validated at this layer, so a 4xx from upstream isn't reliably something the caller could have avoided; `mapUpstreamErrorGeneric` returns the fixed fallback message for those instead of echoing upstream detail. Both log the full reason via the caller's `slog.ErrorContext(ctx, ..., "err", err)` regardless of which is used.
- **Response**: return raw `[]byte` with `writeJSON` for simple passthroughs; unmarshal into typed structs only when the response shape needs to change

## OpenAPI spec

**`openapi.yaml` must be updated whenever the API changes** — new endpoints, removed endpoints, changed request/response shapes, new error codes. It is the contract consumed by the frontend and other teams; an out-of-date spec is worse than no spec.

- Error responses use `$ref: '#/components/schemas/ErrorPayload'`
- Every endpoint must declare a `403` response
- Path parameters that expect UUIDs must declare `format: uuid` on the schema
- The `Case` schema includes a computed `nextStates` read-only field populated server-side from `state`
- Binary-download endpoints (e.g. attachment content) must document the `Content-Disposition: attachment` and `X-Content-Type-Options: nosniff` security headers in their description

## Response shape

- For **portal-owned/transformed** responses (typed structs constructed by the portal), all JSON fields must use **camelCase** (e.g. `createdAt`, `projectId`, `issueType`); use `json:"fieldName"` struct tags to enforce this
- **Raw passthrough** responses may retain upstream field naming as-is — do not reshape them unless there is an explicit requirement to do so
- The `nextStates` field is portal-constructed and follows camelCase like all other portal-owned fields

## Security

- **Never commit secrets** — API keys, tokens, passwords, and service URLs with credentials must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — do not log request bodies, JWT payloads, or user PII; log only IDs and error summaries
- **JWT is the only auth mechanism** — all endpoints must validate the caller via `middleware.UserInfoFromContext`; there are no public endpoints
- **Audience** — `Config.Audiences` is `[]string`; a token is accepted if its `aud` claim contains **any** of the configured values (OR logic). Set via `AUTH_AUDIENCE` as a comma-separated string
- **Input validation** — validate and reject unexpected input at the boundary (path params, body size, JSON structure) before forwarding to upstream services
- **Error messages** — never leak upstream error details or stack traces to the caller; use the fixed `ErrMsg*` constants or a short fallback message. This is what `mapUpstreamErrorGeneric` enforces by default; see the Handler conventions section for the narrow PATCH-handler exception that uses `mapUpstreamError` instead
- **Security fixes in PRs** — when a change is made to fix a security issue (gosec findings, input sanitization, etc.), do not mention it in the PR title or description; describe the change in neutral functional terms only
- **Run gosec on every backend change** — `gosec -fmt=text ./...` (install once: `go install github.com/securego/gosec/v2/cmd/gosec@latest`) must report 0 issues before opening a PR touching this backend; fix the root cause of any finding rather than suppressing it, unless a `#nosec` annotation with a justification comment already covers that exact case
- **Run govulncheck on every backend change** — `govulncheck ./...` (install once: `go install golang.org/x/vuln/cmd/govulncheck@latest`) must report no vulnerabilities before opening a PR touching this backend. Most findings here are Go standard-library CVEs tied to the toolchain patch version pinned in `go.mod`'s `go` directive — bump it to the latest `1.26.x` patch (and run `go mod tidy` so the toolchain download matches) rather than working around the symptom. A finding in a third-party module is fixed with `go get <module>@<fixed-version>`

## Testing

- Mocks live in `internal/handler/helpers_test.go` — when you extend a handler interface, add the new field and method to the mock there
- `upstreamErrors(fallback)` is the error table for the ten `mapUpstreamError` PATCH-handler tests (surfaces the upstream 400/409/422 reason); `upstreamErrorsGeneric(fallback)` is its counterpart for every other handler's tests, which call `mapUpstreamErrorGeneric` and always expect `fallback` for those statuses instead
- `withUser()` injects a test user into the request context
- `decodeJSON[T]()` decodes response bodies in assertions
- Use real UUIDs (e.g. `"11111111-1111-1111-1111-111111111111"`) for UUID path param test values — not fake slugs like `"case-1"`
