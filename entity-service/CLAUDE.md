# Entity Service

Go HTTP server (`net/http`, standard library only) that owns all core CS-platform entities: users, accounts, projects, products, deployments, deployed products, cases, and case comments. It exposes a REST API consumed by portal BFFs and other internal services.

## Architecture

Strict four-layer stack — no shortcuts across layers:

```
Handler → Service → Repository → PostgreSQL (pgx/v5)
```

All wiring happens explicitly in `internal/server/routes.go` (no DI framework). The full dependency graph is built there: `NewRepository(db) → NewService(repo) → NewHandler(svc)`, then registered on a `net/http.ServeMux`.

Middleware chain wraps the mux: **CorrelationID → Recovery → Logger → UserIDToken → Timeout** (10 s per request).

`CorrelationID` reads the `X-CSM-Correlation-ID` request header forwarded by the portal BFF, or generates a UUID v4 if absent. The ID is stored in the request context and echoed in the response header. All access log lines and panic logs include the correlation ID for end-to-end request tracing.

## Running locally

```bash
cp .env.example .env   # fill in DB_* vars
go run ./cmd/api/main.go
```

The server loads `.env` automatically on startup (silently ignored if absent). Port defaults to `8080`; override with `SERVER_PORT`.

## Environment variables

| Variable      | Required | Default | Purpose                   |
|---------------|----------|---------|---------------------------|
| `DB_HOST`     | yes      | —       | PostgreSQL hostname        |
| `DB_PORT`     | yes      | —       | PostgreSQL port            |
| `DB_USER`     | yes      | —       | Database user              |
| `DB_PASSWORD` | yes      | —       | Database password          |
| `DB_NAME`     | yes      | —       | Database name              |
| `DB_SSLMODE`  | no       | —       | `disable` or `require`    |
| `SERVER_PORT` | no       | `8080`  | HTTP listen port           |

## Adding a new entity

Follow these steps in order:

1. **Domain types** (`internal/domain/entity.go`) — add request/response structs and any enums; keep all types in this one file
2. **Repository** (`internal/repository/<entity>_repo.go`) — define the `<Entity>Repository` interface in the same file, then implement it against pgx; use parameterized queries only, never string-interpolate user-supplied values
3. **Service** (`internal/service/<entity>_service.go`) — implement the business logic (validation, pagination normalization); register the interface in `internal/service/interfaces.go`
4. **Handler** (`internal/handler/<entity>_handler.go`) — follow the handler pattern below
5. **Route** (`internal/server/routes.go`) — wire repo → svc → handler, then register routes using Go 1.22 method-prefixed patterns (e.g. `"POST /widgets/{id}/search"`)
6. **OpenAPI spec** (`openapi.yaml`) — document every new path; declare 400/404/500 responses on every endpoint

## Adding a new endpoint to an existing entity

1. Add the method to the repository interface and implement it
2. Add the method to the service interface (`interfaces.go`) and implement it in the service
3. Add the handler func
4. Register the route in `routes.go`
5. Document in `openapi.yaml`

## Handler conventions

Every handler follows the same skeleton:

```go
func (h *WidgetHandler) CreateWidget(w http.ResponseWriter, r *http.Request) {
    var req domain.CreateWidgetRequest
    if !decodeRequest(w, r, &req) {   // enforces 1 MiB cap + unknown-field rejection
        return
    }
    result, err := h.svc.CreateWidget(r.Context(), req)
    if err != nil {
        writeServiceError(w, r, err)  // maps service errors to HTTP status codes
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(result)
}
```

- `decodeRequest` (in `internal/handler/decode.go`) enforces a 1 MiB body cap, rejects unknown fields, and rejects trailing data after the JSON object
- `writeServiceError` (same file) maps `ValidationError` → 400, `NotFoundError` → 404, `ServiceUnavailableError` → 503, `context.DeadlineExceeded` → 408; everything else → 500
- Never write custom status mappings inline in a handler

## Service conventions

- Validate all input **before** hitting the repository; return `*apierror.ValidationError` for bad input
- UUID fields must be validated with `validateUUIDs()` (defined in the service package)
- Pagination: call `normalizePagination()` — it caps `limit` at 100 and sets defaults
- Use `validXxx` maps (e.g. `validCaseState`, `validCasePriority`) to validate enum fields; add a map entry whenever you add an enum constant
- Service methods must not import the `handler` or `repository` packages

## Repository conventions

- Each entity gets one file; the `<Entity>Repository` interface lives at the top of the same file
- Use `pgx.ErrNoRows` to detect missing rows and return `*apierror.NotFoundError`
- Wrap unexpected errors with `fmt.Errorf("operation name: %w", err)` for traceability
- PostgreSQL enum casts are required for enum columns (e.g. `$1::case_state_enum`)
- For queries that need both a COUNT and a SELECT, run them concurrently with `errgroup` (see `SearchCases` and `SearchCaseComments` in `case_repo.go`)

## Domain types

All shared types live in `internal/domain/entity.go`. Conventions:

- JSON field names use camelCase (`json:"fieldName"`)
- Request structs include only the fields a caller can supply; ID fields injected from path params use `json:"-"`
- Optional fields in request structs use pointer types (`*CasePriority`) so absent fields are distinguishable from zero values
- Response structs return the full entity row
- **Date/time field naming:** all timestamp fields in response structs must use the `On` suffix: `createdOn`, `updatedOn`, `closedOn`. Never use `At` (`createdAt`, `updatedAt`, `closedAt`). Domain-specific date fields that carry a business meaning (e.g. `startDate`, `endDate`, `activationDate`) keep the `Date` suffix. This applies to both Go struct field names and JSON tags.
- **Empty strings must never appear in responses where the value is absent.** Use pointer types (`*string`, `*EntityRef`, `*DeployedProductRef`, etc.) for any response field that may be absent, and leave them `nil` so they serialise as JSON `null`. Never assign an empty-string value to a non-pointer field as a stand-in for "not present". For optional sub-fields within a required struct (e.g. `UserRef.ID` when only the email is known), add `omitempty` to the JSON tag so they are omitted rather than serialised as `""`.
- **Request enum field naming:** enum fields in **request** structs must use the `Key` / `Keys` suffix in **both the Go struct field name and the JSON tag / openapi spec** — singular for a single value, plural for an array. Examples: Go field `StateKey \`json:"stateKey"\``, `PriorityKey \`json:"priorityKey"\``, `IssueTypeKey \`json:"issueTypeKey"\``, `WorkStateKey \`json:"workStateKey"\``, `TypeKey \`json:"typeKey"\``; arrays: `StateKeys \`json:"stateKeys"\``, `PriorityKeys \`json:"priorityKeys"\``, `IssueTypeKeys \`json:"issueTypeKeys"\``, `DeploymentTypeKeys \`json:"deploymentTypeKeys"\``. UUID ID fields follow a separate convention: `ProjectID \`json:"projectId"\`` / `ProjectIDs \`json:"projectIds"\`` (no `Key` suffix). Response structs are unaffected — they use the plain field name (e.g. `State`, `Priority` with `json:"state"`, `json:"priority"`).

## Error types (`internal/apierror`)

| Type                    | HTTP status | When to use                              |
|-------------------------|-------------|------------------------------------------|
| `*ValidationError`      | 400         | Invalid input supplied by the caller     |
| `*NotFoundError`        | 404         | Requested resource does not exist        |
| `*ServiceUnavailableError` | 503      | Downstream dependency temporarily down   |

`apierror.WriteJSON(w, status, msg)` writes `{"code": <status>, "message": "<msg>"}`.

## Database migrations

Migrations live in `migrations/` as plain SQL files, numbered `000NNN_<description>.up.sql` / `.down.sql`. Each migration creates its PostgreSQL enums, sequences, and tables in a single transaction. Apply them in ascending order before starting the service.

Key conventions enforced at the DB level:
- Primary keys are `UUID DEFAULT gen_random_uuid()`
- Human-readable IDs (e.g. `CASE-001`, `WSO2-001`) are generated from dedicated sequences via column defaults
- Enum types (e.g. `case_state_enum`, `case_priority_enum`) enforce valid values at the DB level; Go enum validation in the service layer is an additional guard
- Triggers enforce relational constraints that foreign keys alone cannot express (e.g. deployment must belong to the same project as the case)

## OpenAPI spec

`openapi.yaml` is the source of truth for the API contract.

- Error responses reference `$ref: '#/components/schemas/ErrorResponse'`
- Path parameters that accept UUIDs must declare `format: uuid`
- Every writable endpoint (POST, PATCH) needs 400 and 404 responses in addition to the success response
- Schema names should match the Go domain type names (e.g. `CreateCaseRequest`, `Case`)

## Connection pool settings

Configured in `internal/db/postgres.go`:

| Setting             | Value   |
|---------------------|---------|
| Max connections     | 20      |
| Min connections     | 2       |
| Max conn lifetime   | 30 min  |
| Max idle time       | 5 min   |

## Pagination response conventions

All search responses — regardless of data source — must use `total` (not `totalRecords`) as the JSON field name for the count of matched records. This applies to every `SearchXxxResponse` struct in `internal/domain/entity.go`.

ServiceNow integration responses from Choreo use `totalRecords` internally (in the private `snXxxResponse` structs inside the `sn_*` service files). Always map that value to the `Total` field of the domain response before returning:

```go
return domain.SearchFooResponse{
    Foos:   views,
    Total:  snResp.TotalRecords, // map SN field → domain field
    Limit:  req.Pagination.Limit,
    Offset: req.Pagination.Offset,
}, nil
```

## ServiceNow data source (`sn_*` services)

ServiceNow uses 32-character hex sysids (e.g. `abc123...`) while the rest of the platform uses standard UUIDs (`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`). Conversion helpers live in `internal/service/sn_id.go`.

**Rules — apply without exception:**

- **Outbound (request to SN):** convert every UUID to a sysid with `uuidToSysid()` / `uuidsToSysids()` before including it in the SN payload.
- **Inbound (response from SN):** convert every ID field back to a UUID with `sysidToUUID()` before populating the domain response struct. This includes every ID in every response type — cases, comments, projects, deployments, deployed products, etc.

Missing a `sysidToUUID()` call on a response ID means callers receive a bare sysid they cannot use to call back into the entity service.

**SN payload field types must match what the Choreo Ballerina integration service expects.** The public domain API and the `sn_*` payload structs are separate layers with different representations:

- **String enum → integer key:** ServiceNow choice-list fields use integer keys (`typeKey`, `stateKey`, etc.) in the Choreo API even when the domain exposes string enums (e.g. `"primary_production"`). Add a `xxxToKey map[domain.XxxType]int` in the SN service file (see `deploymentTypeToKey` in `sn_deployment_service.go`) and look up the integer before populating the SN payload. Never pass a string directly into a field the Choreo API defines as an integer — it will fail at runtime with a Ballerina data-binding error.
- **Before adding a new writable SN endpoint**, read the existing `sn_*` payload structs for that entity (or a similar one) to confirm which fields Choreo expects as integers vs strings. Cross-reference the Choreo API contract to identify which choice-list fields require integer keys.

## Security

- Never commit secrets — use environment variables; `.env` is git-ignored
- Never log request bodies, passwords, or tokens; log only IDs and sanitised error summaries
- All SQL uses parameterized queries; never interpolate user input into query strings
- Validate and reject unexpected input at the handler boundary before it reaches the service or repository
