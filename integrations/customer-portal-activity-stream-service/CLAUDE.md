# Customer Portal Activity Stream Service

Go HTTP server (`net/http`, Go 1.26+) serving the case-activity SSE endpoint (`GET /cases/{id}/activities/stream`) on a dedicated listener (default `:9092`). Consumes the `case-events` Azure Event Hub topic in its own per-replica consumer group (`LatestOffset`) and fans `case.comment_added` (customer-visible comments only) / `case.status_changed` events to connected SSE clients via an in-process `BroadcastHub`.

Cloned from `integrations/csm-portal-activity-stream-service`, adapted for `apps/customer-portal`. The underlying "case" concept, ID space, and entity-service contract are identical between the two portals, so almost nothing besides naming needed to change — see that service's own `CLAUDE.md` for the original design rationale, most of which applies here unchanged.

## Why a separate service

- A REST API listener's `WriteTimeout`/`IdleTimeout` would kill long-lived SSE connections. The stream needs its own listener with both timeouts disabled.
- Horizontal scale: each replica must see 100% of events (per-replica consumer group + `LatestOffset`), and the BroadcastHub only knows its own process's subscribers. Separating the stream lets it scale independently of the BFF.
- Choreo deployment: the stream has its own endpoint declaration (`.choreo/component.yaml`) and OpenAPI spec (`openapi-stream.yaml`), distinct from `apps/customer-portal/backend-v2`'s REST API.

## Event flow

```
apps/csm-portal/backend ──┐
apps/customer-portal/backend-v2 ──┤──▶ entity-service (EventPublisherService) ──▶ Event Hub "case-events" ──▶ this service (own consumer group) ──▶ BroadcastHub ──▶ browser EventSource
```

Both portals' backends call the same entity-service HTTP endpoints (`POST /comments`, `PATCH /cases/{id}`); the publish happens inside entity-service's `snCaseService.CreateCaseComment`/`UpdateCase`, unconditionally, regardless of which portal's backend made the call. This service, `csm-portal-activity-stream-service`, and `csm-notification-service` all consume the same topic independently, each in its own consumer group.

Only `case.comment_added` (skipped when the comment is an internal note) and `case.status_changed` are broadcast; other types (e.g. `case.created`, `case.assigned`, `case.severity_changed`, `case.acknowledged`, `incident.created`) are not. The payload is minimal: `{caseId, type, timestamp}` — no comment text, field values, names or emails.

## Auth

The SSE endpoint validates `x-jwt-assertion` (and optional `x-user-id-token`) on every request via `middleware.Auth`. There is no separate ticket/token-exchange step. The browser connects with these headers directly via an EventSource polyfill (`@sanity/eventsource`) — native `EventSource` cannot set custom headers.

The incoming `x-user-id-token` is forwarded to the entity-service `GetCase` call (upstream ACL check) so a caller can only subscribe to a case they're authorized to read.

**Verify before deploying**: confirm which Asgardeo tenant/app registration `apps/customer-portal/backend-v2` actually authenticates against (its own `config.toml`/JWT interceptor setup) and use those same `AUTH_JWKS_ENDPOINT`/`AUTH_ISSUER`/`AUTH_AUDIENCE`/`OAUTH2_*` values here — don't assume they're identical to `csm-portal-activity-stream-service`'s just because the variable names match.

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
| `EVENT_HUB_CONNECTION_STRING` | Required once `EVENT_HUB_BROKER` is set | Shared Access Policy connection string (namespace-scoped, no EntityPath) — same Event Hub namespace as `csm-portal-activity-stream-service`, since it's the same shared `case-events` topic |
| `EVENT_HUB_TOPIC` | Required once `EVENT_HUB_BROKER` is set | Event Hub name = Kafka topic (`case-events`) |
| `EVENT_HUB_CONSUMER_GROUP` | No (default `customer-portal-activity-stream-service`) | Base consumer group name (suffixed per-replica with `-replica-<hostname>`) — deliberately distinct from `csm-portal-activity-stream-service`'s own group so each service gets its own full copy of every event |
| `STREAM_PORT` | No (default 9092) | Port the SSE listener binds to |
| `STREAM_CORS_ALLOWED_ORIGINS` | No | Comma-separated browser Origins for the SSE endpoint; fail-closed |
| `CORS_ALLOWED_ORIGINS` | No | Comma-separated browser Origins for the health listener (:8080); fail-closed |
| `AUTH_JWKS_ENDPOINT` | Yes | JWKS URL for JWT validation — verify against customer-portal's own tenant, see "Auth" above |
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
go test -race ./...
```

## Deployment (Choreo)

See `.choreo/component.yaml` (two endpoints: health on :8080, SSE on :9092 with `openapi-stream.yaml`). The stream endpoint is Public network visibility.

Apply these proactively at creation time — all five were found and fixed one at a time while deploying the CSM sibling service (see that repo's memory of the saga, or ask for the full history):

1. **JWKS x5c fix** is already included (`internal/middleware`, copied verbatim from the CSM sibling after PR #1557 fixed it there) — Asgardeo's JWKS cert has a negative X.509 serial number that Go's `crypto/x509` has rejected since Go 1.23.
2. **Choreo gateway CORS header allowlist**: add `x-jwt-assertion`/`x-user-id-token` to the API's `Access-Control-Allow-Headers` panel — Choreo Connect answers preflights itself with its own default list otherwise.
3. **Choreo gateway Security Scheme**: set **Operation Level Security → Disabled** for `GET /cases/{id}/activities/stream` — the app's own `middleware.Auth` is the real auth layer; the gateway's default OAuth2/`Authorization`-header scheme doesn't match what this endpoint receives (`x-jwt-assertion`, not `Authorization`).
4. **Choreo endpoint timeout**: set Resiliency → Endpoint timeout to the max (`299999`ms) — the 60s default force-disconnects every SSE connection.
5. **Producer wiring**: already confirmed live for customer-portal, no action needed — see "Event flow" above.

## Packages

- `internal/apierror` — typed upstream error
- `internal/events` — `Envelope` + event types (a 4th hand-synced copy alongside entity-service, csm-notification-service, and csm-portal-activity-stream-service — see that package's doc comment)
- `internal/eventbus` — `Consumer` (simple: no retry/DLQ; commit after Handle; `LatestOffset`; per-replica group suffix)
- `internal/stream` — `BroadcastHub` (in-process pub-sub per case ID; `subscriberBuffer=4`; non-blocking publish)
- `internal/caseevents` — `Handler` (consumes events, fans to BroadcastHub for the two SSE types)
- `internal/entity` — minimal `CustomerEntityClient` (only `GetCase`, OAuth2 client-credentials, forwards `x-user-id-token` + correlation ID)
- `internal/middleware` — `Auth`, `CORS`, `CorrelationID`, `Logger`, `SecurityHeaders`
- `internal/handler` — `StreamCaseActivities` SSE handler + `response.go` helpers (`writeError`, `mapUpstreamErrorGeneric`, error constants)

## Known limitations

- No per-user/per-replica cap on concurrent SSE connections. A hostile client could exhaust goroutines/FDs.
- `Envelope` is hand-synced across four Go modules — changes must be propagated manually to entity-service, csm-notification-service, and csm-portal-activity-stream-service.
- A redeploy (new pod/hostname) causes one-time replay of retained events (accepted tradeoff).
- Consumer groups accumulate forever on the broker (no API to delete).
- No frontend integration yet — `apps/customer-portal/webapp` and `apps/customer-portal/microapp` have no equivalent of the CSM webapp's `useCaseActivityStream` hook. This service is backend-only until that follow-up lands.

## Adding a new event type to SSE

1. Add the type to `internal/events/events.go` (mirror entity-service's copy).
2. Add it to the switch in `internal/caseevents/handler.go` (broadcast to hub).
3. Update `openapi-stream.yaml` if the payload shape changes.
4. Propagate the same change to the sibling `csm-portal-activity-stream-service` if both portals should see it.

## Security

- Never commit secrets — API keys, tokens, passwords, service URLs with credentials must not appear in source or config.
- No sensitive data in logs — do not log request bodies, JWT payloads, or user PII; log only IDs and error summaries.
- JWT is the only auth mechanism — all endpoints validate the caller via `middleware.UserInfoFromContext`; no public endpoints.
- Error messages — never leak upstream error details or stack traces to the caller; use the fixed error constants or a short fallback message (see `mapUpstreamErrorGeneric`).
- Run `go vet` + `go test -race` before opening a PR.
