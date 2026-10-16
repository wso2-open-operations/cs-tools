# CSM Activity Stream Service

Go HTTP server (`net/http`, Go 1.26+) serving the case-activity SSE endpoint (`GET /cases/{id}/activities/stream`) on a dedicated listener (default `:9092`). Consumes the `case-events` Azure Event Hub topic in its own per-replica consumer group (`LatestOffset`) and fans `case.comment_added` / `case.status_changed` events to connected SSE clients via an in-process `BroadcastHub`.

Extracted from `apps/csm-portal/backend` (internal/stream, internal/caseevents, internal/eventbus consumer, and the :9092 listener in cmd/server/main.go). That backend no longer serves the SSE stream, nor does it publish to the topic anymore — its own `internal/eventbus`/`internal/eventpublisher`/`internal/events` were removed entirely; `entity-service` is now the sole publisher of `case-events`.

## Why a separate service

- The REST API's :8080 listener has `WriteTimeout`/`IdleTimeout` that would kill long-lived SSE connections. The stream needs its own listener with both timeouts disabled.
- Horizontal scale: each replica must see 100% of events (per-replica consumer group + `LatestOffset`), and the BroadcastHub only knows its own process's subscribers. Separating the stream lets it scale independently of the BFF.
- Choreo deployment: the stream has its own endpoint declaration (`.choreo/component.yaml`) and OpenAPI spec (`openapi-stream.yaml`), distinct from the main `csm-portal-backend` REST API.

## Event flow

```
entity-service ──▶ Event Hub "case-events" ──▶ this service (consumer group per replica) ──▶ BroadcastHub ──▶ browser EventSource
```

Only `case.comment_added` and `case.status_changed` are broadcast. The payload is minimal: `{caseId, type, timestamp}` (`timestamp` = the record's time on the topic, RFC 3339 UTC, omitted if unknown) — no comment text or field values.

SSE framing: each stream starts with `retry: 3000`; each `case_updated` carries `id: <hub-epoch>-<seq>`. On reconnect with `Last-Event-ID` the handler replays this case's missed events from the hub's in-memory ring (`stream.ReplayCapacity` = 256 events across all cases, `stream.ReplayWindow` = 2 min). Best-effort only: IDs are local to one process, so a reconnect to another replica, after a restart, or past the window gets no replay — clients still refresh on (re)connect. Consumer offsets are committed in 1 s batches (`CommitInterval`), not one synchronous commit per record.

## Auth

The SSE endpoint validates `x-jwt-assertion` (and optional `x-user-id-token`) on every request via the same `middleware.Auth` chain as `csm-portal-backend`. There is no separate ticket/token-exchange step. The browser connects with these headers directly via an EventSource polyfill (`@sanity/eventsource`) — native `EventSource` cannot set custom headers.

The incoming `x-user-id-token` is forwarded to the entity-service `GetCase` call (upstream ACL check) so a caller can only subscribe to a case they're authorized to read.

**Stream lifetime.** A stream never outlives the credential that opened it: the handler derives a context deadline at `min(token exp, connect + STREAM_MAX_LIFETIME)` and repeats the `GetCase` access check every `STREAM_REAUTH_INTERVAL`. When either bound ends the stream the server writes a terminal `event: stream_closed` with `data: {"reason": "token_expired" | "max_lifetime" | "access_revoked"}` and closes; the browser polyfill reconnects with fresh headers (and gets a 401/403 if access really is gone). A transient upstream failure during re-authorization does not close the stream.

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
go test -race ./...
```

## Deployment (Choreo)

See `.choreo/component.yaml` (two endpoints: health on :8080, SSE on :9092 with `openapi-stream.yaml`). The stream endpoint is Public network visibility.

## Packages

- `internal/apierror` — typed upstream error (mirrors csm-portal-backend/internal/apierror)
- `internal/events` — `Envelope` + event type constants only (hand-synced copy; keep in sync with csm-notification-service's and entity-service's own copies). Payload structs are deliberately not copied — this service never reads payloads
- `internal/eventbus` — `Consumer` (simple: no retry/DLQ; commit after Handle; `LatestOffset`; per-replica group suffix; `Run` returns an error, logged at Error, when its reader stops — `io.EOF` included) and `Supervisor` (replaces an exited consumer with exponential backoff, 1s doubling to 30s; `Running()` drives `/health`, which answers 503 `{"status":"unavailable"}` while the configured consumer is not running and stays 200 when Event Hub is not configured)
- `internal/stream` — `BroadcastHub` (in-process pub-sub per case ID; `subscriberBuffer=4`; non-blocking publish; `CloseAll` is registered as the stream server's `RegisterOnShutdown` hook so SIGTERM ends every stream with a terminal `event: shutdown` and `Server.Shutdown` completes within its grace period)
- `internal/caseevents` — `Handler` (consumes events, fans to BroadcastHub for the two SSE types)
- `internal/entity` — minimal `CustomerEntityClient` (only `GetCase`, OAuth2 client-credentials, forwards `x-user-id-token` + correlation ID)
- `internal/middleware` — `Auth`, `CORS`, `CorrelationID`, `Logger`, `SecurityHeaders` (mirrors csm-portal-backend)
- `internal/handler` — `StreamCaseActivities` SSE handler + `response.go` helpers (`writeError`, `mapUpstreamErrorGeneric`, error constants)

## Known limitations

- The concurrent-stream caps (`STREAM_MAX_CONNECTIONS_PER_USER`, `STREAM_MAX_CONNECTIONS`) are per replica and in-memory; there is no cluster-wide count, so a user's effective cap is the per-user cap multiplied by the replica count.
- `Envelope` is hand-synced across three Go modules — changes must be propagated manually to csm-notification-service's and entity-service's own `internal/events`.
- A redeploy (new pod/hostname) creates a new consumer group that starts at `LatestOffset`, so events published while no consumer was running are not broadcast (no replay of retained history); clients refresh on reconnect.
- Consumer groups accumulate forever on the broker (no API to delete).

## Adding a new event type to SSE

1. Add the type to `internal/events/events.go` (mirror csm-notification-service).
2. Add it to the switch in `internal/caseevents/handler.go` (broadcast to hub).
3. Update `openapi-stream.yaml` if the payload shape changes.
4. Frontend hooks (`useCaseActivityStream` in webapp + microapp) already invalidate on any `case_updated` event — no frontend change needed unless the event type needs different handling.

## Security

- Never commit secrets — API keys, tokens, passwords, service URLs with credentials must not appear in source or config.
- No sensitive data in logs — do not log request bodies, JWT payloads, or user PII; log only IDs and error summaries.
- JWT is the only auth mechanism — all endpoints validate the caller via `middleware.UserInfoFromContext`; no public endpoints.
- Error messages — never leak upstream error details or stack traces to the caller; use the fixed error constants or a short fallback message (see `mapUpstreamErrorGeneric`).
- Run `go vet` + `go test -race` before opening a PR.