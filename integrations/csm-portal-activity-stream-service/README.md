# CSM Activity Stream Service

Go HTTP server (`net/http`, Go 1.26+) that serves the case-activity SSE endpoint (`GET /cases/{id}/activities/stream`). It consumes the `case-events` Azure Event Hub topic in its own consumer group and fans `case.comment_added` / `case.status_changed` events to any connected SSE clients via an in-process BroadcastHub.

This service was extracted from `apps/csm-portal/backend/internal/stream` + `internal/caseevents` + the dedicated :9092 listener in `apps/csm-portal/backend/cmd/server/main.go`. That backend no longer serves the SSE stream and no longer publishes to the topic; `entity-service` is the sole publisher of `case-events`.

## Why a separate service

- The REST API's :8080 listener has `WriteTimeout`/`IdleTimeout` that would kill long-lived SSE connections. The stream needs its own listener with both timeouts disabled.
- Horizontal scale: each replica must see 100% of events (per-replica consumer group + `LatestOffset`), and the BroadcastHub only knows its own process's subscribers. Separating the stream lets it scale independently of the BFF.
- Choreo deployment: the stream has its own endpoint declaration (`.choreo/component.yaml`) and OpenAPI spec (`openapi-stream.yaml`), distinct from the main `csm-portal-backend` REST API.

## Event flow

```
entity-service ──▶ Event Hub "case-events" ──▶ this service (consumer group per replica, LatestOffset) ──▶ BroadcastHub ──▶ browser EventSource
```

Only `case.comment_added` and `case.status_changed` are broadcast. The payload is minimal: `{caseId, type, timestamp}` (`timestamp` = the record's time on the topic) — no comment text or field values.

Each stream starts with `retry: 3000`, and each `case_updated` event carries an `id:`. A client reconnecting with `Last-Event-ID` is replayed the events it missed for that case, best-effort: the window is short (last 256 events, at most 2 minutes), in memory, and local to one replica, so clients must still refresh on every (re)connect.

## Auth

The SSE endpoint validates `x-jwt-assertion` (and optional `x-user-id-token`) on every request via the same `middleware.Auth` chain as `csm-portal-backend`. There is no separate ticket/token-exchange step. The browser connects with these headers directly via an EventSource polyfill (`@sanity/eventsource`) — native `EventSource` cannot set custom headers.

The incoming `x-user-id-token` is forwarded to the entity-service `GetCase` call (upstream ACL check) so a caller can only subscribe to a case they're authorized to read.

A stream is bounded by the credential that opened it: it is closed at `min(token exp, connect + STREAM_MAX_LIFETIME)` and the case-access check is repeated every `STREAM_REAUTH_INTERVAL`. A server-side close is announced with a terminal `event: stream_closed` (`data: {"reason": "token_expired" | "max_lifetime" | "access_revoked"}`) so the client can reconnect with a fresh token.

## Middleware chain

`SecurityHeaders → CORS → CorrelationID → Auth → Logger → Mux`

- `SecurityHeaders`: `X-Content-Type-Options: nosniff`, `Content-Security-Policy: upgrade-insecure-requests`, `Strict-Transport-Security: max-age=31536000; includeSubDomains`
- `CORS`: fail-closed (comma-separated `STREAM_CORS_ALLOWED_ORIGINS`; unset denies all). Must wrap Auth (preflight carries no x-jwt-assertion). In Choreo prod the gateway supplies CORS itself; this matters for local dev.
- `CorrelationID`: reads `X-CSM-Correlation-ID` or generates UUID v4; stores in context (for slog + entity client); echoes in response.
- `Auth`: validates `x-jwt-assertion` JWT (JWKS/Issuer/Audiences/ClockSkew). Stores `UserInfo` in context; also extracts `x-user-id-token` (falls back to `x-jwt-assertion`) and puts it in the entity client context via `entity.WithUserIDToken`.
- `Logger`: logs method/path/status/elapsed via slog; correlation ID + userID auto-injected via context.

`ConfigureLogger()` must be called at startup.

## Config (environment variables)

| Variable | Required | Description |
|---|---|---|
| `EVENT_HUB_BROKER` | No — optional feature gate | Kafka-compatible bootstrap address, e.g. `<namespace>.servicebus.windows.net:9093`. Left unset, the service starts and serves only the health check; the SSE endpoint returns 503 |
| `EVENT_HUB_CONNECTION_STRING` | Required once `EVENT_HUB_BROKER` is set | Shared Access Policy connection string (namespace-scoped, no EntityPath) |
| `EVENT_HUB_TOPIC` | Required once `EVENT_HUB_BROKER` is set | Event Hub name = Kafka topic (`case-events`) |
| `EVENT_HUB_CONSUMER_GROUP` | No (default `csm-portal-activity-stream-service`) | Base consumer group name (suffixed per-replica with `-replica-<hostname>`) |
| `STREAM_PORT` | No (default 9092) | Port the SSE listener binds to |
| `STREAM_MAX_LIFETIME` | No (default `1h`) | Maximum lifetime of one SSE connection. A stream is closed at `min(token exp, connect + STREAM_MAX_LIFETIME)` with a terminal `stream_closed` event; the client reconnects with a fresh token |
| `STREAM_MAX_CONNECTIONS_PER_USER` | No (default `8`) | Maximum concurrently open streams per authenticated user on one replica; excess connections get `429` with `Retry-After`. `0` disables the cap |
| `STREAM_MAX_CONNECTIONS` | No (default `2000`) | Maximum concurrently open streams per replica; excess connections get `503` with `Retry-After`. `0` disables the cap |
| `STREAM_REAUTH_INTERVAL` | No (default `10m`) | How often an open stream repeats the connect-time case-access check (entity-service `GetCase`). A definitive 401/403/404 closes the stream (`stream_closed`, reason `access_revoked`); transient upstream errors are logged and the stream kept open |
| `STREAM_CORS_ALLOWED_ORIGINS` | No | Comma-separated browser Origins for the SSE endpoint; fail-closed |
| `CORS_ALLOWED_ORIGINS` | No | Comma-separated browser Origins for the health listener (:8080); fail-closed |
| `AUTH_JWKS_ENDPOINT` | Yes | JWKS URL for JWT validation |
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
# from integrations/csm-portal-activity-stream-service
go run ./cmd/server/main.go
```

The server auto-loads `.env` from the working directory at startup.

## Testing

```bash
go vet ./...
go test -race ./...
```

## Deployment (Choreo)

See `.choreo/component.yaml` (two endpoints: health on :8080, SSE on :9092 with `openapi-stream.yaml`). The stream endpoint is Public network visibility.