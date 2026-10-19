# Customer Portal Activity Stream Service

Go HTTP server (`net/http`, Go 1.26+) that serves the case-activity SSE endpoint (`GET /cases/{id}/activities/stream`). It consumes the `case-events` Azure Event Hub topic in its own consumer group and fans `case.comment_added` (customer-visible comments only) / `case.status_changed` events to any connected SSE clients via an in-process BroadcastHub.

Cloned from `integrations/csm-portal-activity-stream-service`, adapted for `apps/customer-portal`. See that repo's `CLAUDE.md` for the original design rationale, and this repo's own `CLAUDE.md` for customer-portal-specific notes.

## Why a separate service

- The REST API's :8080 listener has `WriteTimeout`/`IdleTimeout` that would kill long-lived SSE connections. The stream needs its own listener with both timeouts disabled.
- Horizontal scale: each replica must see 100% of events (per-replica consumer group + `LatestOffset`), and the BroadcastHub only knows its own process's subscribers. Separating the stream lets it scale independently of the BFF.
- Choreo deployment: the stream has its own endpoint declaration (`.choreo/component.yaml`) and OpenAPI spec (`openapi-stream.yaml`), distinct from `apps/customer-portal/backend-v2`'s REST API.

## Event flow

```
apps/csm-portal/backend ──┐
apps/customer-portal/backend-v2 ──┤──▶ entity-service ──▶ Event Hub "case-events" ──▶ this service (own consumer group) ──▶ BroadcastHub ──▶ browser EventSource
```

Both portals' backends call the same entity-service endpoints; the publish is a side effect of entity-service's own `CreateCaseComment`/`UpdateCase`, unconditional on which portal called it. This service, `csm-portal-activity-stream-service`, and `csm-notification-service` each consume the same topic independently, in their own consumer groups.

Only `case.comment_added` (skipped when the comment is an internal note) and `case.status_changed` are broadcast; other types (e.g. `case.created`, `case.assigned`, `case.severity_changed`, `case.acknowledged`, `incident.created`) are not. The payload is minimal: `{caseId, type, timestamp}` — no comment text, field values, names or emails.

## Auth

The SSE endpoint validates `x-jwt-assertion` (and optional `x-user-id-token`) on every request via `middleware.Auth`. There is no separate ticket/token-exchange step. The browser connects with these headers directly via an EventSource polyfill (`@sanity/eventsource`) — native `EventSource` cannot set custom headers.

The incoming `x-user-id-token` is forwarded to the entity-service `GetCase` call (upstream ACL check) so a caller can only subscribe to a case they're authorized to read.

## Middleware chain

`SecurityHeaders → CORS → CorrelationID → Auth → Logger → Mux`

- `SecurityHeaders`: `X-Content-Type-Options: nosniff`, `Content-Security-Policy: upgrade-insecure-requests`, `Strict-Transport-Security: max-age=31536000; includeSubDomains`
- `CORS`: fail-closed (comma-separated `STREAM_CORS_ALLOWED_ORIGINS`; unset denies all). Must wrap Auth (preflight carries no x-jwt-assertion). In Choreo prod the gateway supplies CORS itself; this matters for local dev.
- `CorrelationID`: reads `X-Customer-Portal-Correlation-ID` or generates UUID v4; stores in context (for slog + entity client); echoes in response.
- `Auth`: validates `x-jwt-assertion` JWT (JWKS/Issuer/Audiences/ClockSkew). Stores `UserInfo` in context; also extracts `x-user-id-token` (falls back to `x-jwt-assertion`) and puts it in the entity client context via `entity.WithUserIDToken`.
- `Logger`: logs method/path/status/elapsed via slog; correlation ID + userID auto-injected via context.

`ConfigureLogger()` must be called at startup.

## Config (environment variables)

| Variable | Required | Description |
|---|---|---|
| `EVENT_HUB_BROKER` | No — optional feature gate | Kafka-compatible bootstrap address, e.g. `<namespace>.servicebus.windows.net:9093`. Left unset, the service starts and serves only the health check; the SSE endpoint returns 503 |
| `EVENT_HUB_CONNECTION_STRING` | Required once `EVENT_HUB_BROKER` is set | Shared Access Policy connection string (namespace-scoped, no EntityPath) |
| `EVENT_HUB_TOPIC` | Required once `EVENT_HUB_BROKER` is set | Event Hub name = Kafka topic (`case-events`) |
| `EVENT_HUB_CONSUMER_GROUP` | No (default `customer-portal-activity-stream-service`) | Base consumer group name (suffixed per-replica with `-replica-<hostname>`) |
| `STREAM_PORT` | No (default 9092) | Port the SSE listener binds to |
| `STREAM_CORS_ALLOWED_ORIGINS` | No | Comma-separated browser Origins for the SSE endpoint; fail-closed |
| `CORS_ALLOWED_ORIGINS` | No | Comma-separated browser Origins for the health listener (:8080); fail-closed |
| `AUTH_JWKS_ENDPOINT` | Yes | JWKS URL for JWT validation — verify against customer-portal's own Asgardeo tenant, see `CLAUDE.md`'s "Auth" section |
| `AUTH_ISSUER` | Yes | Expected `iss` claim |
| `AUTH_AUDIENCE` | Yes | Comma-separated accepted `aud` values |
| `AUTH_TOKEN_VALIDATOR_ENABLED` | No (default true) | Set `false` to decode without signature (local dev) |
| `OAUTH2_CLIENT_ID` | Yes | Client ID for entity-service calls (client-credentials) |
| `OAUTH2_CLIENT_SECRET` | Yes | Client secret for entity-service calls |
| `OAUTH2_TOKEN_URL` | Yes | Token endpoint URL |
| `CUSTOMER_ENTITY_BASE_URL` | Yes | Base URL of the entity service (e.g. `https://entity-service/...`) |
| `CUSTOMER_ENTITY_SCOPES` | No | Comma-separated scopes for entity-service |

## Running locally

```bash
# from integrations/customer-portal-activity-stream-service
go run ./cmd/server/main.go
```

The server auto-loads `.env` from the working directory at startup.

## Testing

```bash
go vet ./...
go test -race ./...
```

## Deployment (Choreo)

See `.choreo/component.yaml` (two endpoints: health on :8080, SSE on :9092 with `openapi-stream.yaml`). The stream endpoint is Public network visibility. See `CLAUDE.md`'s "Deployment (Choreo)" section for known gateway config gotchas to apply up front.
