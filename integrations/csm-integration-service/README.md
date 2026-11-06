# CSM Integration Service

A small Go HTTP service that exposes a curated, machine-to-machine (M2M) API over
CSM platform data (accounts, projects, contacts, cases, incidents, opportunities,
invoices, vulnerabilities, cloud status) for third-party consumers. It forwards each
request to the entity service and returns the upstream JSON unchanged.

- [Architecture](#architecture)
- [Developer guide](#developer-guide)
- [Configuration](#configuration)
- [API endpoints](#api-endpoints)

## Architecture

### Purpose and position

The CSM platform keeps its system of record behind the **entity service**, which is an
internal API and is not published to outside parties. This service is the single
published front door for external, non-interactive consumers (automation, sync jobs,
public status pages). It adds no business logic: it validates the shape of a request,
forwards it, and maps upstream failures to a stable error vocabulary.

```text
 third-party M2M client
          |  (API gateway: subscription + client-credentials app auth)
          v
 csm-integration-service  --- OAuth2 client credentials --->  entity service  --->  data source
```

For how this service fits into the wider platform (portal, backends, entity service,
event backbone), see the
[platform architecture](../../apps/csm-portal/README.md#architecture).

### Interactions

| Direction | Peer | Detail |
|---|---|---|
| Inbound | API gateway | Plain HTTP on `PORT`. The gateway owns inbound trust (subscription plus M2M client-credentials auth). |
| Outbound | Entity service | HTTPS JSON, authenticated with an OAuth2 client-credentials token that is fetched and refreshed automatically. The request's correlation ID is forwarded. |

This service stores nothing itself; all data lives behind the entity service.

### Authentication model

- **Inbound: no authentication in the application.** The gateway is the trust boundary,
  and the app performs no Bearer/JWT validation. Unlike the portal backend
  (`apps/csm-portal/backend`), there is no end-user identity here. Do not add inbound
  token validation without confirming that assumption has changed. Note this does not
  mean 401/403 cannot happen: an upstream 401/403 is passed through to the caller.
- **Outbound: always M2M.** Every request to the entity service uses the service's own
  client-credentials identity. There is no mechanism to forward an end-user token.
  Consequently, entity-service operations that strictly require an end-user identity
  (those that exist only for the legacy external case-management data source) are
  rejected with 401. Such operations are kept in the API for shape completeness:
  - `PATCH /projects/{id}` always returns 401.
  - `PATCH /cases/{id}`: a `state`/`severity`/`workState` update succeeds when the
    entity service runs on the PostgreSQL data source; any other field returns 400
    there. On the legacy data source every field returns 401.
  - `PATCH /incidents/{id}`: returns 503 on the PostgreSQL data source (no PostgreSQL
    path exists); on the legacy data source it succeeds only if the target environment
    has an M2M fallback credential configured, otherwise 401.
  - `POST /incidents`, `POST /incidents/search` and `POST /services/search` behave the
    same way as `PATCH /incidents/{id}` on the legacy data source (a 401 is possible,
    not guaranteed).
- **Case writes act as a configured actor.** `POST /cases/{id}/comments` and
  `POST /cases/{id}/tags` accept only `type`/`content` and `label` from the caller. The
  service injects the author identity itself from `UMT_INTEGRATION_ACTOR_EMAIL`, which
  must appear in the entity service's trusted-actor allowlist, otherwise those calls
  return 403. The caller can never supply the actor.

### Request pipeline

`cmd/server/main.go` wires a `net/http` `ServeMux` (Go 1.22+ method-and-wildcard
patterns) behind three middlewares, outermost first:

1. `SecurityHeaders` sets `X-Content-Type-Options`, `Content-Security-Policy`,
   `Strict-Transport-Security`, `X-Frame-Options` and `Referrer-Policy` on every response.
2. `CorrelationID` reads `X-CSM-Correlation-ID` (or generates a UUID v4), normalises it to
   carry exactly one `cis-` prefix, echoes it in the response, adds it to every log
   record, and forwards it on every outgoing entity-service request.
3. `Logger` writes one structured access-log line per request (method, path, status,
   elapsed).

Handlers cap request bodies at 1 MiB (413 beyond that), require valid JSON (400
otherwise), and require path ids to be UUIDs (400 otherwise; applies to the
account/project/case/incident/opportunity/invoice `{id}` routes). Search and sync
bodies are forwarded verbatim and the entity service owns field validation.

Upstream errors are mapped by `mapUpstreamError`:

| Upstream status | Returned to caller |
|---|---|
| 400 | 400, with the upstream message if it is short and free of database internals; otherwise a generic "Invalid request payload." |
| 401 / 403 / 404 | same status, fixed message |
| 409 / 422 | same status, endpoint-specific message |
| 502 / 503 / 504 | 503 |
| anything else, or a transport error | 500 |

Errors are `{"message": "..."}`. Successful responses are the upstream body as-is,
with one exception: the `/cloud-status/*` endpoints wrap the data in a compatibility
envelope `{"result": {"code": 0, "message": "success", "cloud": "<cloud>", "data": ...}}`
that an existing public status dashboard consumes.

The server uses `ReadHeaderTimeout` 10 s, `ReadTimeout`/`WriteTimeout` 30 s and
`IdleTimeout` 60 s, an outbound request timeout of 25 s (10 s for token fetches), and
shuts down gracefully (15 s) on `SIGINT`/`SIGTERM`.

### Project structure

```text
csm-integration-service/
├── cmd/server/main.go          # Entry point: config, routes, server lifecycle
├── internal/
│   ├── apierror/               # Typed upstream error (status code + body excerpt)
│   ├── entity/
│   │   ├── client.go           # OAuth2 client-credentials HTTP client for the entity service
│   │   └── entity.go           # One method per upstream operation
│   ├── middleware/             # correlation.go, logger.go, security_headers.go
│   └── handler/                # One file per resource (accounts, projects, cases, incidents,
│                               #   it_services, alert_incident_mappings, opportunities,
│                               #   invoices, project_opportunity_links, vulnerabilities,
│                               #   cloud_status) plus response.go (shared helpers)
├── .choreo/component.yaml      # Deployment descriptor (port 8080, serves openapi.yaml)
├── openapi.yaml                # Published API contract
├── Makefile
└── .env.example
```

## Developer guide

### Prerequisites

- Go `1.26.0` or newer (see `go.mod`) - [install](https://go.dev/doc/install)
- A reachable entity service and an OAuth2 client for it (see [Configuration](#configuration))
- Optional: `gosec` for the security scan

### Build

```bash
# from integrations/csm-integration-service
go build -o server ./cmd/server
# or: make build   (vet + race tests + compile)
```

### Run locally

```bash
cp .env.example .env     # then fill in the values
go run ./cmd/server
```

`.env` is read from the working directory at startup; it is ignored if absent, and
variables already set in the process environment win over the file. The server listens on
`http://localhost:8080` unless `PORT` is set. Missing required variables cause an
immediate exit with an error log. Never commit a real `.env`.

Smoke test (the gateway is not present locally, so no credentials are needed on these
calls, but the entity service must be reachable):

```bash
curl http://localhost:8080/health
curl -X POST http://localhost:8080/accounts/search -d '{}'
curl http://localhost:8080/accounts/<account-uuid>
curl -X POST http://localhost:8080/projects/search -d '{}'
curl -X POST http://localhost:8080/cases/search -d '{}'
curl -X POST http://localhost:8080/vulnerabilities/sync -d '[]'   # full replace: sends an empty set
curl "http://localhost:8080/cloud-status/monitors?cloud=<cloud>"
```

Search bodies are passed through to the entity service, so consult `openapi.yaml` for
the accepted filter and pagination shapes. `/vulnerabilities/sync` replaces the whole
stored set, so do not point a smoke test at a shared environment.

### Test

```bash
go vet ./...
go test ./...
go test -race ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
make test      # vet + go test -race
```

Handler tests use a mock entity client (`internal/handler/helpers_test.go`) and a shared
table covering every `mapUpstreamError` mapping. Entity client tests start real
`httptest.Server`s to exercise the client-credentials flow, error-body truncation and
correlation-ID forwarding. Middleware tests cover header injection and ID
generation/preservation. `cmd/server` (wiring only) and `internal/apierror` have no
dedicated tests.

### Lint and security scan

```bash
go vet ./...
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec -fmt=text ./...
```

The scan should report 0 issues. Fix the root cause of any new finding rather than
suppressing it. `#nosec` annotations in the code each carry a justification.

### Adding or changing an endpoint

1. Add the upstream call to `internal/entity/entity.go` (a thin wrapper over `Client.do`;
   escape path segments with `url.PathEscape`).
2. Add a handler in `internal/handler/` with a small interface for the entity methods it
   needs, so tests can mock it. Reuse `writeJSON`, `writeError`, `mapUpstreamError`,
   `uuidRe` and `maxRequestBodyBytes` from `response.go`.
3. Register the route in `cmd/server/main.go`.
4. Document it in `openapi.yaml` (this file is the published contract; keep it in step).
5. Add handler tests, including an upstream-error case.
6. Before exposing an operation that needs an end-user identity upstream, remember this
   service cannot supply one (see [Authentication model](#authentication-model)).

Keep the service a pass-through: do not reshape upstream bodies, and keep
data-source-specific vocabulary out of the contract.

## Configuration

Configuration is by environment variables only, optionally loaded from a `.env` file in
the working directory (process environment takes precedence). Copy `.env.example` as a
starting point.

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `ENTITY_BASE_URL` | Yes | none | Base URL of the entity service; a trailing `/` is trimmed. | `https://entity.example.com/api` |
| `ENTITY_TOKEN_URL` | Yes | none | OAuth2 token endpoint used for the client-credentials grant. | `https://auth.example.com/oauth2/token` |
| `ENTITY_CLIENT_ID` | Yes | none | OAuth2 client id of this service. | `<client-id>` |
| `ENTITY_CLIENT_SECRET` | Yes | none | OAuth2 client secret. Treat as a secret. | `<client-secret>` |
| `ENTITY_SCOPES` | No | none (no scopes requested) | Comma-separated OAuth2 scopes; blanks are ignored. | `scope-a,scope-b` |
| `UMT_INTEGRATION_ACTOR_EMAIL` | No at startup; needed for case writes | empty | Email of this service's trusted M2M actor, asserted as the author on `POST /cases/{id}/comments` and `POST /cases/{id}/tags`. Must be in the entity service's trusted-actor allowlist, otherwise those calls return 403. | `integration-bot@example.com` |
| `MARK_FIX_ISSUED_ACTOR_EMAILS` | No | empty (no attribution) | Maps a third-party consumer's own client id (decoded from the `x-jwt-assertion` header the gateway forwards) to its display email, so a `markFixIssued` `PATCH /cases/{id}` call is attributed to the real caller in entity-service's activity feed instead of this service's own shared client id. A JSON object of client id to email; an unmapped or missing client id, or an unparseable value, just skips attribution (logged at WARN), never blocks the call. | `{"client-a":"team-a@example.com","client-b":"team-b@example.com"}` |
| `PORT` | No | `8080` | TCP listen port. | `8080` |

Notes:

- There are no log-level or other tuning variables; logging is structured text to stderr
  at the default `slog` level.
- Timeouts and the 1 MiB body limit are constants in code, not configuration.
- `UMT_INTEGRATION_ACTOR_EMAIL` is not yet listed in `.env.example`; add it to your `.env`
  if you use the case comment or tag endpoints.

## API endpoints

The authoritative contract is `openapi.yaml`. Routes registered in `cmd/server/main.go`:

| Area | Routes |
|---|---|
| Health | `GET /health` (200, no body, no upstream call) |
| Accounts | `GET /accounts/{id}`, `POST /accounts/search`, `POST /accounts/{id}/contacts/search` |
| Projects | `GET /projects/{id}`, `POST /projects/search`, `POST /projects/{id}/contacts/search`, `PATCH /projects/{id}` (always 401, see above) |
| Cases | `POST /cases/search`, `PATCH /cases/{id}`, `POST /cases/{id}/comments`, `POST /cases/{id}/tags` |
| Incidents | `POST /incidents`, `PATCH /incidents/{id}`, `POST /incidents/search`, `POST /services/search` (IT service search) |
| Alert mappings | `POST /alert-incident-mappings`, `POST /alert-incident-mappings/lookup` |
| Commercial | `GET /opportunities/{id}`, `POST /opportunities/search`, `GET /invoices/{id}`, `POST /invoices/search`, `POST /project-opportunity-links/search` (no by-id fetch upstream) |
| Vulnerabilities | `POST /vulnerabilities/sync` (full replace: submit the complete current set every call, not a delta) |
| Cloud status | `GET /cloud-status/monitors`, `/incidents`, `/availabilities`, `/availability-history`, `/incidents/{id}`, each with a `cloud` query parameter |

Alert-mapping endpoints are PostgreSQL-backed upstream and are expected to work over M2M.
