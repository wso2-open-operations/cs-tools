# CSM Integration Service

Go HTTP server (`net/http`, Go 1.26+) exposing Project/Account search and their
Contacts sub-resource, plus a subset of Case operations, incident creation and
search, and alert-incident mapping create/lookup, to third-party (M2M)
consumers. It forwards requests to the entity service and returns responses
as-is — it does not shape or authenticate on behalf of an end user.

## Authentication is the gateway's; authorization is per-operation scopes here

Unlike `apps/csm-portal/backend` (a BFF for the CSM Portal's own end users, which
validates an `x-jwt-assertion` JWT on every request), this service has no end-user
identity to check. It's consumed by third-party M2M clients through Choreo's API
Manager gateway, which owns the inbound trust boundary (subscription + client
credentials) before a request ever reaches this app. Do not add inbound JWT/Bearer
signature validation here without confirming that assumption no longer holds.

What this service does do is **authorize per operation**. Every route except
`GET /health` requires one OAuth2 scope (`cases:write`, `contacts:read`,
`vulnerabilities:sync`, …); the full route-to-scope table is
`cmd/server/routes.go` and the same names are published in `openapi.yaml`'s
`oauth2ClientCredentials` scheme and on each operation. `middleware.ScopeGuard`
(`internal/middleware/scopes.go`) enforces it by decoding — never
signature-verifying — the `x-jwt-assertion` token the gateway forwards and
reading its `scope` claim (space-separated string or array; `scp` as a
fallback). It fails closed: no usable token is 401, a token without the
operation's scope — including a token with no scope claim at all — is 403,
and neither reaches the entity service. The check is on by default;
`REQUIRE_OPERATION_SCOPES=false` disables it for local development only, and
the server logs a warning at startup when it is off.

The point of the split: the gateway proves *who* is calling; the scope table
decides *what* that caller may do, so a subscriber that only needs the status
dashboard reads never inherits the ability to patch cases or run the
product-vulnerability sync, even though this service's own credential to the
entity service is unrestricted.

**"No authentication here" is not the same claim as "this service never returns
401/403 on its own."** The scope guard produces both, and `mapUpstreamError`
(`internal/handler/response.go`) additionally maps an upstream 401/403 straight
through to the caller.

## This service is M2M-only — no end-user identity is ever forwarded

Earlier revisions of this service optionally forwarded a caller-supplied
`x-user-id-token` header to entity-service, for entity-service's ServiceNow-backed
operations (which require a forwarded end-user identity and reject M2M-only
requests — confirmed directly against `cs_entity_service`, which returns 401
`Missing or invalid user ID token header.` on every such resource with no
exceptions). That pass-through has been removed: this service's identity to
entity-service is now unconditionally M2M, with no mechanism anywhere to carry an
end-user token. Do not re-add one without confirming this decision no longer
holds — see git history for the removal if context is needed.

Practical implication: this service can only ever serve entity-service data that
doesn't require a forwarded user identity (Postgres-backed operations). Any
operation that can reach a ServiceNow-backed entity-service operation that
*strictly requires* a forwarded user identity will get a mapped 401 from
`mapUpstreamError` unconditionally, since there is no longer any path for a
user token to reach entity-service. **`POST /incidents`/`POST /incidents/search`
are a documented exception to this** — see their own paragraph below — because
their underlying ServiceNow operation has a separately-configured M2M
credential fallback, so it doesn't strictly require a forwarded user token the
way `UpdateProject` does. (`CreateCaseComment` used to be in the same
unconditional bucket as `UpdateProject` too, but no longer is on
`DATA_SOURCE=postgres` -- see its own paragraph below.)

**`PATCH /projects/{id}` (`UpdateProject`) is kept despite this — deliberately, not
by oversight.** It was added for the Account Closure Process (ACP) automation, but
it targets a ServiceNow-data-source-only entity-service operation that requires a
forwarded end-user identity — something this service structurally cannot provide
under an M2M-only model. **Every call to this endpoint currently receives a mapped
401 from `mapUpstreamError`, unconditionally.** It's kept for API-shape
completeness (a real caller has somewhere to point at, and the shape of the
request/response is documented and stable), not because it works today.

**`POST /incidents` (`CreateIncident`) and `POST /incidents/search`
(`SearchIncidents`) are NOT in the same "always 401" state as `UpdateProject`,
despite proxying ServiceNow-backed entity-service incident operations.** Their
underlying ServiceNow layer has a deliberate fallback: when no end-user identity
token is forwarded, it uses a separately-configured M2M ServiceNow credential
instead of erroring, and only 401s if that fallback credential is itself
unconfigured in the target environment. A live end-to-end call through this
exact path against a development environment (`<dev-tenant>`) succeeded with no
401, creating an incident (`<incident-number>`). So whether these two endpoints 401 depends on the
target ServiceNow environment's M2M credential configuration — it is not an
unconditional consequence of this service being M2M-only. Treat a 401 from
either endpoint as a possible outcome that depends on the environment's M2M
ServiceNow credential: check that credential before retrying, and don't treat
the 401 as proof the endpoint is permanently broken.

**`POST /services/search` (`SearchITServices`) uses this exact same
M2M-fallback mechanism** — it proxies a ServiceNow-backed entity-service CMDB
IT-service search operation, confirmed to go through the identical code path
as `CreateIncident`/`SearchIncidents` above. It works over M2M the same way
those two now do: a 401 is possible if the target environment's M2M
ServiceNow credential isn't configured, but that is not unconditional.

**`PATCH /incidents/{id}` (`PatchIncident`) uses this exact same M2M-fallback
mechanism as `CreateIncident`/`SearchIncidents`/`SearchITServices` above — but
unlike `PATCH /cases/{id}` below, it has no Postgres-data-source path at
all.** On `DATA_SOURCE=postgres`, entity-service's `incidentService.UpdateIncident`
unconditionally returns a 503 (not supported on this data source yet — several
fields have no backing Postgres column, and others would need comment-table
side effects not implemented there); there is no field combination that
succeeds. On `DATA_SOURCE=servicenow`, it goes through the identical
M2M-credential-fallback code path as the other three endpoints above: a 401
is possible if the target environment's M2M ServiceNow credential isn't
configured, but not unconditional. **Do not describe this endpoint as "always
401" (that's `UpdateProject`'s situation) or as a field-dependent partial
exception like `PATCH /cases/{id}` (that endpoint's Postgres path narrows to
specific fields instead of failing outright) — its actual behavior is
"unconditionally ServiceNow-backed, no Postgres fallback, M2M-credential-
dependent on that data source."**

The "deferred pending a captured end-user token" history below (from the owning
team's internal issue, written by the engineer who built the ACP path) describes
`UpdateProject`'s situation specifically — that endpoint's ServiceNow operation
has no equivalent M2M fallback, so it remains unconditionally 401 as described.

**`PATCH /cases/{id}` (`PatchCase`) is a partial exception to "always 401" —
know the difference before assuming every writable endpoint here behaves like
`UpdateProject`, and know that its behavior depends on entity-service's own
data source, not just on which fields are sent.**

- On `DATA_SOURCE=postgres`, a state/severity/workState update (optionally
  combined with resolutionCode/cause/closeNotes when state is closed or
  solution_proposed) **succeeds** through this M2M-only service — that path in
  entity-service's `case_service.go` never checks a forwarded identity token.
  Every other field this request shape accepts (watchList, assigneeEmail,
  type and its companions, parentId, relatedCaseId, autocloseHoldUntil,
  subject, description, deploymentId, deployedProductId, the fix-ETA group,
  acknowledge, workaroundProvided) is rejected there with a **400**, not a
  401 — entity-service's Postgres path explicitly refuses them as
  ServiceNow-only, without ever reaching a token check.
- On `DATA_SOURCE=servicenow`, entity-service's `sn_case_service.go` requires
  a forwarded identity token for every field this operation accepts,
  including a bare state/severity/workState update — so on that data source,
  every call through this service gets a mapped **401**, the same as
  `UpdateProject`, with no field combination that succeeds.

Don't assume a 401 here means the endpoint is broken the way `UpdateProject`
is, and don't assume a 400 here means bad input from the caller — check both
which fields were sent and which data source entity-service is running.

**`POST /cases/{id}/comments` (`CreateCaseComment`) is now a partial
exception to "always 401" too, mirroring `POST /cases/{id}/tags`'s M2M
`actorEmail` path. Know the difference before assuming it's still stuck in
the always-401 state described in earlier revisions of this doc.**

- On `DATA_SOURCE=postgres`, this handler injects this service's own
  configured `UMT_INTEGRATION_ACTOR_EMAIL` into the request body as
  `actorEmail`, never taken from the caller, the same way `AddCaseTag`
  injects it. entity-service checks it against its own
  `M2M_TRUSTED_ACTOR_EMAILS` allowlist and, when it matches, creates the
  comment with no forwarded token required. **Succeeds** today when
  `UMT_INTEGRATION_ACTOR_EMAIL` is allowlisted; **403** if it is not. The
  variable is required: the server refuses to start when it is unset or not
  a single bare e-mail address (`actorEmail` in `cmd/server/main.go`).
- On `DATA_SOURCE=servicenow`, entity-service's `sn_case_service.go` never
  hard-required a token locally to begin with, it just forwards whatever
  `x-user-id-token` is on the request (possibly empty) straight to
  ServiceNow, and a caller-supplied `actorEmail` is accepted but silently
  ignored there (a plain passthrough, mirroring `AddCaseTagAs`'s own SN-mode
  counterpart). Since this service never forwards a token, every call on
  this data source still gets a mapped **401** from ServiceNow itself, same
  as before this fix.

`POST /cases/{id}/tags` (`AddCaseTag`, see `cases.go`) injects
`UMT_INTEGRATION_ACTOR_EMAIL` the same way, for the same
Postgres-succeeds/ServiceNow-still-401 split described above.

## `POST /alert-incident-mappings` and `POST /alert-incident-mappings/lookup` are functional today

**Unlike `PATCH /projects/{id}` above, these two endpoints are NOT stuck in an
always-401 state.** They proxy a Postgres-only entity-service operation with no
ServiceNow dependency, so no forwarded end-user identity is required — this
service's M2M-only identity to entity-service is sufficient on its own. A
caller through Choreo's gateway can expect a real `201`/`200` from these today,
not a guaranteed `401`. (`POST /incidents` and `POST /incidents/search` are
also not guaranteed-401 — see their own paragraph above — but unlike these two,
their success still depends on the target ServiceNow environment's M2M
credential being configured.) Don't assume every endpoint in this service is in
the "kept for API-shape completeness, doesn't work yet" state described above —
check whether the underlying entity-service operation is ServiceNow-backed
(needs either a forwarded identity or a configured M2M fallback) or
Postgres-only (works fine over M2M unconditionally) before documenting a new
endpoint one way or the other.

## Contacts and opportunity reads on `DATA_SOURCE=postgres`

Contacts search and opportunity/invoice/link reads work over M2M only if this service's client id is in entity-service's `AUTH_INTERNAL_CLIENT_IDS`; otherwise 401/403.

## Middleware chain

`SecurityHeaders → CorrelationID → Logger → Mux → (ScopeGuard per route) → handler`

- `SecurityHeaders` (`internal/middleware/security_headers.go`): sets five
  headers on every response — `X-Content-Type-Options: nosniff`,
  `Content-Security-Policy: upgrade-insecure-requests`,
  `Strict-Transport-Security: max-age=31536000; includeSubDomains`,
  `X-Frame-Options: DENY` and `Referrer-Policy: no-referrer`
- `CorrelationID` (`internal/middleware/correlation.go`): reads
  `X-CSM-Correlation-ID` from the incoming request and keeps it only if it is at
  most 128 bytes of `[A-Za-z0-9._-]` (otherwise generates a UUID v4); ensures
  the ID carries a `cis-` prefix (CSM Integration Service) either way, without
  double-prefixing an ID that already has it; stores the ID in context for the
  slog handler and for the entity client to forward; echoes the ID in the
  response header
- `Logger` (`internal/middleware/logger.go`): logs every completed request (method,
  path, status, elapsed) via slog, recording the first status actually sent;
  `GET /health` polls are served but not logged

`GET /health` (`internal/handler/health.go`) is a real dependency check: 200
`{"status":"ok"}` while the entity service's own public `/health` answers 2xx,
503 `{"status":"unavailable"}` otherwise, with the probe result cached for 15 s
and one probe in flight at a time. `apierror.Error.Error()` reports the
upstream status only — never the body — so a wrapped upstream error is safe to
log; read `Body` explicitly when it is needed.
- `ScopeGuard` (`internal/middleware/scopes.go`): not in the outer chain but
  wrapped around every route except `GET /health` by `newMux`
  (`cmd/server/routes.go`); rejects a request whose forwarded token lacks the
  route's scope (401 no token / 403 wrong scope) before the handler runs

`middleware.ConfigureLogger()` must be called at startup — it wraps the default slog
handler so every `slog.*Context(r.Context(), …)` call automatically includes
`correlationID=<id>` when the context carries one.

## Upstream service modules

| Package | Upstream | Notes |
|---------|----------|-------|
| `entity` | Entity service | Account/Project + Contacts sub-resource, Case (search + patch + comment create + tag create), Opportunity/Invoice/ProjectOpportunityLink (read-only), incident creation/search/update, alert-incident mapping create/lookup; raw `[]byte` passthrough |

A new upstream service would get its own package under `internal/`, following the
same `Config`/`Client`/`NewClient`/`do()` pattern as `internal/entity`.

## Running locally

```bash
# from integrations/csm-integration-service
go run ./cmd/server/main.go
```

The server auto-loads `.env` from the working directory at startup (silently ignored
if absent).

## Commands

```bash
make setup   # wire up git hooks (once after clone)
make test    # vet + race-detector tests
make build   # runs tests then compiles ./cmd/server
```

## Adding a new endpoint

0. **Confirm the upstream operation doesn't require a forwarded end-user
   identity first.** This service is M2M-only with no mechanism to carry one —
   if the entity-service operation you want to expose is ServiceNow-backed and
   requires `x-user-id-token`, calls will always 401 here (see `UpdateProject`
   above, kept deliberately in that state). Know this going in rather than being
   surprised by it later — a new endpoint in this situation should document the
   same "always 401 today" reality rather than implying it works.
1. **Upstream client** (`internal/entity/entity.go`) — add a method on `Client`
   that calls `c.do()`; use `url.PathEscape()` for every path parameter
2. **Handler interface** — extend the local interface in the relevant handler file
   (e.g. `entityAccountClient` in `accounts.go`); keep it minimal
3. **Handler func** — path/body guards → call client → `mapUpstreamError` on
   failure → write response. No auth check inside the handler — the scope
   guard runs in front of it (see the authorization section above)
4. **Route and scope** (`cmd/server/routes.go`) — add a row to `routes()` using
   a Go 1.22 method-prefixed pattern (`"POST /accounts/{id}/contacts/search"`)
   and the scope it requires; reuse an existing `scope*` constant for the same
   resource and verb, or add one. `routes_test.go` then covers the new route's
   401/403/2xx behaviour automatically
5. **OpenAPI spec** (`openapi.yaml`) — add the path with a `security:` entry
   naming that same scope (and, for a new scope, a line under
   `securitySchemes.oauth2ClientCredentials.flows.clientCredentials.scopes`)
   plus 200/400/401/403/404/500 responses — the scope guard produces 401/403
   itself, and `mapUpstreamError` can also map an upstream 401 or 403 straight
   through, so both belong alongside the others
6. **Tests** — add a handler test following `accounts_test.go`/`projects_test.go`'s
   shape (empty/invalid path param, body-too-large, invalid JSON, success
   passthrough, `upstreamErrors` table); extend the mock in `helpers_test.go` to
   satisfy the extended interface

## Handler conventions

- **Request body**: always `body, ok := readJSONBody(w, r, bodyRequired)`
  (`internal/handler/body.go`) — never re-implement the read. It applies the
  1 MiB cap (413), rejects unreadable or invalid JSON (400), and enforces the
  empty-body policy: `bodyRequired` 400s on an empty body; `bodyOptional`
  forwards it as "no filters" and is only for searches whose `openapi.yaml`
  `requestBody` says `required: false`. Keep the two in step
- **Path params**: guard against empty string after `r.PathValue("id")`; if the
  param is a UUID, also validate format using the package-level `uuidRe` compiled
  regex and return 400 on mismatch — fail fast before calling the upstream
- **Upstream errors**: always use `mapUpstreamError(w, err, "<fallback message>")`
  — never write custom status mappings inline
- **Response**: raw `[]byte` passthrough via `writeJSON` — this service does not
  reshape upstream response bodies

## OpenAPI spec

**`openapi.yaml` must be updated whenever the API changes.** It is the contract
published to third-party consumers via Choreo's Developer Portal.

- Error responses use `$ref: '#/components/schemas/ErrorResponse'`
- Path parameters that expect UUIDs must declare `format: uuid` on the schema
- The `oauth2ClientCredentials` security scheme documents how external consumers
  authenticate through Choreo — its `tokenUrl` is a placeholder until the real
  Choreo-managed API is provisioned; don't fill it with a guessed URL

## Security

- **Never commit secrets** — client IDs/secrets and service URLs with credentials
  must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — log only IDs and error summaries
- **No app-level inbound authentication, but per-operation authorization** —
  the gateway authenticates; this service only reads the forwarded token's
  scope claim (see the authorization section above). Don't bolt on JWT
  signature validation without confirming the Choreo gateway model has
  changed, and never register a route outside the scope table
- **Input validation** — validate and reject unexpected input at the boundary
  (path params, body size, JSON structure) before forwarding to the entity service
- **Error messages** — never leak upstream error details or stack traces to the
  caller; use the fixed `ErrMsg*` constants or a short fallback message
- **Security fixes in PRs** — describe security-related changes in neutral
  functional terms only, not called out as security fixes in the title/description
