# CSM Portal Backend

Go HTTP server (`net/http`, Go 1.22+) that acts as a backend-for-frontend (BFF) for the CSM portal. It authenticates callers, forwards requests to upstream services, and shapes responses for the frontend.

## Middleware chain

`CorrelationID → Auth → Logger → Mux`

- `CorrelationID` (`internal/middleware/correlation.go`): reads `X-CSM-Correlation-ID` from the incoming request or generates a UUID v4; stores the ID in context for the slog handler and for the entity client to forward; echoes the ID in the response header
- `Auth` (`internal/middleware/auth.go`): validates the `x-jwt-assertion` JWT and sets `UserInfo` in context
- `Logger` (`internal/middleware/logger.go`): logs every completed request (method, path, status, elapsed) via slog; runs after Auth so both `correlationID` and `userID` are present in every record

`middleware.ConfigureLogger()` must be called at startup — it wraps the default slog handler so that every `slog.*Context(r.Context(), …)` call anywhere in the codebase automatically includes `correlationID=<id>` when the context carries one.

## Upstream service modules

Each upstream service has its own client package under `internal/`:

| Package | Upstream | Notes |
|---------|----------|-------|
| `entity` | Entity service | Most case/account/project endpoints; raw `[]byte` passthrough |
| `scim` | SCIM service | User/group lookups |
| `updates` | Updates service | Product update levels; returns typed structs (not raw passthrough) |

New upstream services get their own package under `internal/` following the same `Client` + `do()` pattern.

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
3. **Handler func** — auth check → path/body guards → call client → `mapUpstreamError` on failure → write response
4. **Route** (`cmd/server/main.go`) — register using Go 1.22 method-prefixed patterns: `"POST /cases/{id}/comments"`
5. **OpenAPI spec** (`openapi.yaml`) — add the path with 200/400/401/403/404/500 responses; `403` is always required because `mapUpstreamError` can return it
6. **Tests** — add handler tests; update the mock in `helpers_test.go` to satisfy the extended interface

## Handler conventions

- **Auth**: always check `middleware.UserInfoFromContext(r.Context()) == nil` first → 401
- **Body size**: cap with `http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)` (1 MiB) before reading
- **Path params**: guard against empty string after `r.PathValue("id")`; if the param is a UUID, also validate format using the package-level `uuidRe` compiled regex and return 400 on mismatch — fail fast before calling the upstream
- **Upstream errors**: always use `mapUpstreamError(w, err, "<fallback message>")` — never write custom status mappings inline
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
- **Input validation** — validate and reject unexpected input at the boundary (path params, body size, JSON structure) before forwarding to upstream services
- **Error messages** — never leak upstream error details or stack traces to the caller; use the fixed `ErrMsg*` constants or a short fallback message

## Testing

- Mocks live in `internal/handler/helpers_test.go` — when you extend a handler interface, add the new field and method to the mock there
- `upstreamErrors(fallback)` returns the standard upstream error table used across all handler tests
- `withUser()` injects a test user into the request context
- `decodeJSON[T]()` decodes response bodies in assertions
- Use real UUIDs (e.g. `"11111111-1111-1111-1111-111111111111"`) for UUID path param test values — not fake slugs like `"case-1"`
