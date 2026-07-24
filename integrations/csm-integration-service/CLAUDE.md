# CSM Integration Service

Go HTTP server (`net/http`, Go 1.26+) exposing Project/Account search and their
Contacts sub-resource to third-party (M2M) consumers. It forwards requests to the
entity service and returns responses as-is — it does not shape or authenticate on
behalf of an end user.

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
operation that can reach a ServiceNow-backed entity-service operation will
**always** get a mapped 401 from `mapUpstreamError` — not conditionally, always,
since there is no longer any path for a user token to reach entity-service.

**Known consequence: `UpdateProject` (`PATCH /projects/{id}`) cannot work.** It
was added for the Account Closure Process (ACP) automation, targets a
ServiceNow-data-source-only entity-service operation, and has no way to supply
the end-user identity that operation requires. This is a deliberate, accepted
gap (not something to silently "fix" by re-adding the pass-through) — its
handler/entity-client code and `openapi.yaml` entry are left in place as-is,
documenting the endpoint's existence and why it 401s unconditionally, rather than
being deleted. Confirm with whoever owns the ACP automation before changing this.

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
| `entity` | Entity service | Account/Project + Contacts sub-resource; raw `[]byte` passthrough |

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

1. **Upstream client** (`internal/entity/entity.go`) — add a method on `Client`
   that calls `c.do()`; use `url.PathEscape()` for every path parameter
2. **Handler interface** — extend the local interface in the relevant handler file
   (e.g. `entityAccountClient` in `accounts.go`); keep it minimal
3. **Handler func** — path/body guards → call client → `mapUpstreamError` on
   failure → write response. No auth check — see "Why no Auth middleware" above
4. **Route** (`cmd/server/main.go`) — register using Go 1.22 method-prefixed
   patterns: `"POST /accounts/{id}/contacts/search"`
5. **OpenAPI spec** (`openapi.yaml`) — add the path with 200/400/404/500 responses,
   plus 401 if the operation can reach a ServiceNow-backed entity-service
   operation (see below)
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
