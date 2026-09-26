# CSM Integration Service

Go HTTP server (`net/http`, Go 1.26+) exposing Project/Account search and their
Contacts sub-resource, plus a subset of Case operations, incident creation and
search, and alert-incident mapping create/lookup, to third-party (M2M)
consumers. It forwards requests to the entity service and returns responses
as-is — it does not shape or authenticate on behalf of an end user.

## Why no `Auth` middleware

Unlike `apps/csm-portal/backend` (a BFF for the CSM Portal's own end users, which
validates an `x-jwt-assertion` JWT on every request), this service has no end-user
identity to check. It's consumed by third-party M2M clients through Choreo's API
Manager gateway, which owns the inbound trust boundary (subscription + client
credentials) before a request ever reaches this app. Do not add inbound JWT/Bearer
validation here without confirming that assumption no longer holds.

**This is not the same claim as "this service never returns 401/403."** `mapUpstreamError`
(`internal/handler/response.go`) maps an upstream 401/403 straight through to the
caller. "No app-level auth" means this service performs no authentication check of
its own; it does not mean 401/403 can't happen.

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
way `UpdateProject` and `CreateCaseComment` do.

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
exact path against `wso2sndev` on 2026-09-20 succeeded with no 401, creating a
real incident (`INC0096966`). So whether these two endpoints 401 depends on the
target ServiceNow environment's M2M credential configuration — it is not an
unconditional consequence of this service being M2M-only. Treat a 401 from
either endpoint as a possible, retryable outcome (see
`internal/csmclient/incidents.go`'s doc comment in `sre-alert-ingestion-service`
for the caller-side reasoning), not as proof the endpoint is permanently broken.

**`POST /services/search` (`SearchITServices`) uses this exact same
M2M-fallback mechanism** — it proxies a ServiceNow-backed entity-service CMDB
IT-service search operation, confirmed to go through the identical code path
as `CreateIncident`/`SearchIncidents` above. It works over M2M the same way
those two now do: a 401 is possible if the target environment's M2M
ServiceNow credential isn't configured, but that is not unconditional. This
endpoint backs `sre-alert-ingestion-service`'s live service-UUID resolution
fallback (see that service's own CLAUDE.md) — a static label-to-UUID map is
consulted first, synchronously, before an alert is ever buffered; this
endpoint is only called, at delivery-attempt time, for a label the static map
doesn't cover.

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
dependent on that data source."** Added for `sre-alert-ingestion-service`'s
group-attach work-note push (see that service's own CLAUDE.md): when it
attaches a new alert to an already-existing incident instead of creating one,
it pushes a work note summarizing the new alert via this endpoint —
best-effort, non-blocking, matching that service's existing failure-tolerance
conventions.

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

**`POST /cases/{id}/comments` (`CreateCaseComment`) has no such exception —
it is unconditionally "always 401" like `UpdateProject`, on both data
sources.** entity-service resolves the comment's author from the forwarded
`x-user-id-token` even on its Postgres-backed path, so there is no field
combination that succeeds through this M2M-only service today. Kept for the
same API-shape-completeness reason as `UpdateProject`.

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

## Middleware chain

`SecurityHeaders → CorrelationID → Logger → Mux`

- `SecurityHeaders` (`internal/middleware/security_headers.go`): sets
  `X-Content-Type-Options: nosniff`, `Content-Security-Policy:
  upgrade-insecure-requests`, and `Strict-Transport-Security:
  max-age=31536000; includeSubDomains` on every response
- `CorrelationID` (`internal/middleware/correlation.go`): reads
  `X-CSM-Correlation-ID` from the incoming request or generates a UUID v4; ensures
  the ID carries a `cis-` prefix (CSM Integration Service) either way, without
  double-prefixing an ID that already has it; stores the ID in context for the
  slog handler and for the entity client to forward; echoes the ID in the
  response header
- `Logger` (`internal/middleware/logger.go`): logs every completed request (method,
  path, status, elapsed) via slog

`middleware.ConfigureLogger()` must be called at startup — it wraps the default slog
handler so every `slog.*Context(r.Context(), …)` call automatically includes
`correlationID=<id>` when the context carries one.

## Upstream service modules

| Package | Upstream | Notes |
|---------|----------|-------|
| `entity` | Entity service | Account/Project + Contacts sub-resource, Case (patch + comment create), Opportunity/Invoice/ProjectOpportunityLink (read-only), incident creation/search/update, alert-incident mapping create/lookup; raw `[]byte` passthrough |

A new upstream service would get its own package under `internal/`, following the
same `Config`/`Client`/`NewClient`/`do()` pattern as `internal/entity`.

## Running locally

```bash
# from operations/csm-integration-service
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
   failure → write response. No auth check — see "Why no Auth middleware" above
4. **Route** (`cmd/server/main.go`) — register using Go 1.22 method-prefixed
   patterns: `"POST /accounts/{id}/contacts/search"`
5. **OpenAPI spec** (`openapi.yaml`) — add the path with 200/400/401/403/404/500
   responses — `mapUpstreamError` can map an upstream 401 or 403 straight
   through to the caller, so both belong alongside the others
6. **Tests** — add a handler test following `accounts_test.go`/`projects_test.go`'s
   shape (empty/invalid path param, body-too-large, invalid JSON, success
   passthrough, `upstreamErrors` table); extend the mock in `helpers_test.go` to
   satisfy the extended interface

## Handler conventions

- **Body size**: cap with `http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)`
  (1 MiB) before reading
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
- **No app-level inbound auth** — this is intentional (see above), not an
  oversight; don't "fix" it by bolting on JWT validation without confirming the
  Choreo gateway model has changed
- **Input validation** — validate and reject unexpected input at the boundary
  (path params, body size, JSON structure) before forwarding to the entity service
- **Error messages** — never leak upstream error details or stack traces to the
  caller; use the fixed `ErrMsg*` constants or a short fallback message
- **Security fixes in PRs** — describe security-related changes in neutral
  functional terms only, not called out as security fixes in the title/description
