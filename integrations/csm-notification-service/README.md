# CSM Notification Service

Go background worker (`net/http`, Go 1.26+) with no inbound API beyond a health check. `csm-portal-backend` and `customer-portal-backend` publish domain events (case created, comment added, status changed, case assigned, incident created) directly to Azure Event Hub; this service's own consumers read them back and react asynchronously by sending email, Google Chat alerts, and voice calls. See [Event-driven notifications](#event-driven-notifications).

This service was extracted from `apps/csm-portal/backend/internal/notifications`, which previously hosted the same email/Google Chat clients. That backend no longer constructs or calls any notification client directly. This service itself used to expose `POST /events` (backends called it, and it published to Event Hub on their behalf) — that producer-side hop was removed once the backends took over publishing directly; this service is a pure consumer now.

## Why no `Auth` middleware

The only route this service exposes is `GET /health` (Choreo's liveness probe) — there's no end-user or caller identity to check, since everything else is Kafka consumption, not an HTTP request.

## Current scope — TODO

- **SMS and direct call channels are unused.** `TwilioClient.SendSMS` has no caller — `MakeCall` is only invoked by `incident.created`.

A dead-letter queue exists (see [Event-driven notifications](#event-driven-notifications)) — a record that exhausts the main consumer's retries is published there and gets a fresh retry pass from a separate DLQ consumer, rather than being dropped immediately. There is still no third tier past that.

This service deliberately has no database connection and never talks to one directly. Deduplicating a publish, or recovering one Event Hub never acknowledged, is now entirely the publishing backend's job — `entity-service`'s `event_publish_failures` table exists for that, written to only when Event Hub doesn't ack a publish, and this service never reads or writes it.

Recipient resolution for `case.*` events is **not** a TODO in the same sense: the caller (e.g. `csm-portal-backend`) supplies a `recipients` array in the event payload itself, since this service has no way to resolve watchers/assignee/reporter on its own. That's still short of "real" resolution in the sense that the caller has to already know the audience, but it's not a fixed/hardcoded stand-in either — every event picks its own recipients. This service *does* now resolve which portal link each of those recipients gets (`internal/recipientlinks`, backed by `internal/entity`'s customer-entity-service client) — a distinct, smaller kind of resolution from audience resolution; see [Event-driven notifications](#event-driven-notifications).

## Middleware chain

`SecurityHeaders → CorrelationID → Logger → Mux`

- `SecurityHeaders` (`internal/middleware/security_headers.go`): sets `X-Content-Type-Options: nosniff`, `Content-Security-Policy: upgrade-insecure-requests`, and `Strict-Transport-Security: max-age=31536000; includeSubDomains` on every response
- `CorrelationID` (`internal/middleware/correlation.go`): reads `X-CSM-Correlation-ID` from the incoming request or generates a UUID v4; ensures the ID carries a `cns-` prefix (CSM Notification Service) either way; stores the ID in context for the slog handler; echoes the ID in the response header
- `Logger` (`internal/middleware/logger.go`): logs every completed request (method, path, status, elapsed) via slog

`middleware.ConfigureLogger()` must be called at startup — it wraps the default slog handler so every `slog.*Context(r.Context(), …)` call automatically includes `correlationID=<id>` when the context carries one.

## Notification channels

| Package | Notes |
|---------|-------|
| `notifications` | Hosts `EmailClient`/`SendEmail` (`email.go`, OAuth2 client-credentials auth), `GoogleChatClient`/`SendIncidentAlert` (`googlechat.go`, per-product incoming-webhook auth), and `TwilioClient`/`SendSMS`+`MakeCall` (`twilio.go`, HTTP Basic Auth — sms and call are two methods on one client, since both are the same Twilio account/auth) |

Each channel gets its own config/client pair in its own file, since channels differ in upstream auth scheme. All three clients are constructed once in `cmd/server/main.go` and handed to `dispatch.NewDispatcher` — a new channel follows the same client pattern, then gets wired into `Dispatcher` for whichever event type should trigger it (see "Adding a new event type" in CLAUDE.md).

## Event-driven notifications

```text
csm-portal-backend ──┐
                      ├──▶ Event Hub topic ──▶ main consumer group ──▶ dispatch.Dispatcher.Handle
customer-portal-backend ┘                           │                  (render email or send incident alerts)
                                                      │ (retries exhausted)
                                                      ▼
                                              Event Hub DLQ topic ──▶ DLQ consumer group ──▶ dispatch.Dispatcher.Handle
                                                                       (fresh retry pass, same Handle;
                                                                        exhausting it here just logs+drops)
```

Both backends publish directly to the main topic — there is no HTTP hop through this service. Two packages implement the bus→consumer side; `internal/notifications` (templates + channel clients) is the last-mile sender they call.

- **`internal/events`** — the event schema and its only remaining validation boundary. `Envelope{Type, EntityID, Payload}` plus one payload struct per `Type` (`case.created`, `case.comment_added`, `case.status_changed`, `case.assigned`, `case.acknowledged`, `case.severity_changed`, `incident.created`), each carrying every value its matching reaction needs. `case.acknowledged` is Chat-only (no email/`recipients`); `case.severity_changed` has both an email and a Chat reaction, same as `case.created`. `EntityID` is a case ID for the `case.*` types or an incident ID for `incident.created` — whatever this event is about; for the `case.*` types, it must match the payload's own `caseId`. `Validate(entityID, type, payload)` decodes strictly and checks required fields — moved here from a since-removed HTTP handler, since there's no request boundary to validate at anymore; `dispatch.Dispatcher.Handle` calls it before rendering/sending anything. Payloads still carry denormalized display values (names/titles) this service has no other way to obtain, but no longer carry pre-built case/comment links — five of the six `case.*` payloads (every one except `case.acknowledged`, which is Chat-only) carry `projectId`/`caseId` (and `commentId`, for `case.comment_added`) instead, and `internal/dispatch` resolves each recipient's own portal-appropriate link itself via `internal/recipientlinks`. `incident.created` is the one type with two independent reactions (a Google Chat alert *and* a voice call) rather than an email. Its call destination (`callTo`) is caller-supplied or falls back to `INCIDENT_DEFAULT_CALL_TO`; its Chat alert's portal link is *not* caller-supplied — like `case.created`'s, it's built by this service itself (`recipientlinks.Resolver.IncidentLink`), not trusted from the payload — and its `product` falls back to `DEFAULT_CHAT_PRODUCT` (see below) the same way `case.created`'s does. `case.created` also posts a Google Chat alert alongside its email — the same `SendIncidentAlert` call incident.created uses — but always to the CSM portal's case link (`recipientlinks.Resolver.CSMLink`), not a per-recipient link, since a Chat post has no per-recipient audience the way an email does.
- **`internal/entity`** — a minimal, read-only customer-entity-service client, implementing only `POST /users/search` (unlike `apps/csm-portal/backend`'s own entity client, a ~60-method passthrough surface). Backs `internal/recipientlinks`'s per-recipient role lookup. Not a notification channel — doesn't follow the `<Name>Config`/`<Name>Client`-in-`internal/notifications` pattern below, since it's an upstream data client, not something that sends a notification itself.
- **`internal/recipientlinks`** — `Resolver.ResolveLinks(ctx, emails, projectID, caseID)` looks up each email's role via `internal/entity` and returns the case link appropriate to their portal (customer vs CSM), with a role → role → userType → CSM-default fallback chain. A per-*recipient* decision, not per-event: the same `case.comment_added` notification can go to both a customer watcher and an internal CSM watcher at once, each needing a different link.
- **`internal/eventbus`** — a thin wrapper around [`github.com/segmentio/kafka-go`](https://github.com/segmentio/kafka-go) (a pure-Go Kafka client, no cgo — keeps this service on Choreo's buildpack deploy, MIT licensed) for Azure Event Hub's Kafka-compatible endpoint. `Producer.Publish` does a synchronous produce — this service's own use of it today is only for publishing to the dead-letter topic, since the main topic's producer side now lives in the backends. `Consumer.Run(ctx, handle, onExhausted)` polls a consumer group, retries a failing record `handleAttempts` (3) times with a fixed delay, then calls `onExhausted` (or logs at ERROR and drops, if `onExhausted` is nil) before committing either way. `PartitionCount(ctx, cfg)` reports a topic's real partition count, used at startup to sanity-check a configured consumer count. See CLAUDE.md for the franz-go → kafka-go swap rationale and its two known trade-offs.
- **`internal/dispatch`** — `Dispatcher.Handle` implements `eventbus.Handle`: decode the record as an `events.Envelope`, validate it (`events.Validate`), then for the four `case.*` types, resolve each recipient's own case link (`groupByLink`, via `internal/recipientlinks`), bucket recipients by the link they resolved to, and render+send one email per distinct link (`sendPerGroup`) — recipients sharing a link still batch into one `SendEmail` call. `case.created` additionally posts a Google Chat alert to the CSM portal's case link, independent of the email step (a failure in one doesn't block the other). `incident.created` skips the email/link-resolution path entirely and sends a Google Chat alert plus places a voice call directly from the payload's own fields (falling back to `DEFAULT_CHAT_PRODUCT`/`INCIDENT_DEFAULT_CALL_TO` when the payload omits `product`/`callTo`).
- **Dead-letter queue** — when the main consumer's `Handle` call fails on all `handleAttempts` attempts, the record is published to `EVENT_HUB_DLQ_TOPIC` instead of being dropped. A second, independent consumer group runs against that topic, using the same `Dispatcher.Handle` — so a dead-lettered record gets its own fresh retry pass — but with nowhere further to escalate to: exhausting retries there just logs and drops. Provision `EVENT_HUB_DLQ_TOPIC` as its own Event Hub in Azure before deploying; this service doesn't create topics itself.
- **Configurable consumer counts** — `MAIN_CONSUMER_COUNT`/`DLQ_CONSUMER_COUNT` each start that many independent `eventbus.Consumer` instances, all joining the same consumer group; Kafka's own rebalancing splits a topic's partitions across however many are actually running. Keep each count at or below its topic's real partition count — a startup check logs a warning (not a hard failure) if it isn't, since excess consumers just sit idle rather than causing an error.

## Configuration

Copy `.env.example` to `.env` and fill in the values:

### Email notification channel

| Variable | Description |
|---|---|
| `EMAIL_BASE_URL` | Base URL of the email notification service (optional) |
| `EMAIL_SCOPES` | Comma-separated OAuth2 scopes for the email service (optional) — authenticates with the shared `OAUTH2_CLIENT_ID`/`OAUTH2_CLIENT_SECRET`/`OAUTH2_TOKEN_URL` below, not its own credentials |
| `EMAIL_FROM_ADDRESS` | Fixed "From" address used for every outgoing email (optional) |
| `EMAIL_SENDING_ENABLED` | Temporary killswitch for `case.*` email delivery — unset/anything but `false` means real sending. Checked before `EMAIL_DEBUG_MODE` below, so it silences email regardless of debug mode. Doesn't affect Google Chat or Twilio |
| `EMAIL_DEBUG_MODE` | Set to `true` to redirect every `case.*` email to `EMAIL_DEBUG_RECIPIENTS` instead of the event's real recipients (recipient links are still resolved against the real recipients either way) — the email still actually sends, just to a safe test list; unset/anything else means real sending to real recipients. Doesn't affect Google Chat or Twilio |
| `EMAIL_DEBUG_RECIPIENTS` | Comma-separated email addresses used instead of the real recipients when `EMAIL_DEBUG_MODE=true`. Empty + debug mode on skips that email entirely rather than sending to nobody |

### Google Chat notification channel

| Variable | Description |
|---|---|
| `GOOGLE_CHAT_SPACES` | JSON array of `{"product","webhookUrl"}` objects, one per Google Chat space. Optional — left unset or malformed, Google Chat alerts are unavailable but startup and every other endpoint work normally. An entry with `"product":"default"` is itself optional and opts in a fallback: an alert whose resolved product matches no other entry routes there instead of erroring |

### SMS and call notification channels (Twilio)

| Variable | Description |
|---|---|
| `TWILIO_ACCOUNT_SID` | Twilio Account SID (optional) |
| `TWILIO_AUTH_TOKEN` | Twilio Auth Token (optional) |
| `TWILIO_MESSAGING_SERVICE_SID` | Twilio Messaging Service SID — preferred for sms, since this is how our account actually sends (optional) |
| `TWILIO_FROM_NUMBER` | Fixed Twilio-provisioned sending number, E.164 format. Used for sms only if `TWILIO_MESSAGING_SERVICE_SID` is unset; **always required for the call channel** — Voice has no Messaging Service equivalent (optional overall, but the call channel won't work without it) |
| `TWILIO_VOICE` | Call channel only: TTS voice for `<Say>` (e.g. `Polly.Raveena`). Optional — empty uses Twilio's account default voice |
| `TWILIO_LANGUAGE` | Call channel only: TTS language/locale for `<Say>` (e.g. `en-IN`), affects pronunciation. Optional — empty uses Twilio's default for the selected voice |
| `TWILIO_API_BASE_URL` | Overrides Twilio's REST API base (default `https://api.twilio.com/2010-04-01`). Optional — only for a regional Twilio edge/API endpoint |
| `CALL_SENDING_ENABLED` | Temporary killswitch — set to `false` to log instead of actually placing `incident.created`'s Twilio call; unset/anything else means real calling. Doesn't affect the Google Chat alert |
| `DEFAULT_CHAT_PRODUCT` | Fallback Google Chat space (`product`) used when a `case.created` or `incident.created` payload omits it — e.g. a publisher like entity-service that can't determine the right space itself (optional) |
| `INCIDENT_DEFAULT_CALL_TO` | Fallback on-call phone number (E.164) used when an `incident.created` payload omits `callTo` (optional) |

### Customer entity service

Backs `internal/recipientlinks`'s per-recipient role lookup (`POST /users/search` only). Optional per deployment like the channels above, but an unset `CUSTOMER_ENTITY_BASE_URL` makes every `case.*` email fail rather than just disabling one channel — a startup warning is logged when this happens. Like the email channel above, this client authenticates with the **shared** `OAUTH2_CLIENT_ID`/`OAUTH2_CLIENT_SECRET`/`OAUTH2_TOKEN_URL` credentials (see below), the same OAuth2 app `apps/csm-portal/backend`'s own entity client uses — only `BaseURL`/`Scopes` are specific to this client.

| Variable | Description |
|---|---|
| `OAUTH2_CLIENT_ID` | Shared OAuth2 client ID, used by the email channel and the customer entity service client (optional) |
| `OAUTH2_CLIENT_SECRET` | Shared OAuth2 client secret (optional) |
| `OAUTH2_TOKEN_URL` | Shared OAuth2 token endpoint (optional) |
| `CUSTOMER_ENTITY_BASE_URL` | Base URL of this repo's entity-service (optional, see above) |
| `CUSTOMER_ENTITY_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Recipient portal links

| Variable | Description |
|---|---|
| `CSM_PORTAL_WEB_BASE_URL` | CSM portal webapp base URL — `<CSM_PORTAL_WEB_BASE_URL>/cases/{caseId}` for recipients classified CSM. Technically optional (empty just yields a relative, non-clickable link), but should be set for any deployment that actually sends `case.*` emails — a startup warning is logged if it's unset |
| `CUSTOMER_PORTAL_WEB_BASE_URL` | Customer portal webapp base URL — `<CUSTOMER_PORTAL_WEB_BASE_URL>/projects/{projectId}/support/cases/{caseId}` for recipients classified customer. Same caveat as above — logged as a startup warning if unset |
| `CUSTOMER_ROLES` | Comma-separated role names classified customer (optional) |
| `CSM_ROLES` | Comma-separated role names classified CSM (optional) |

Classification isn't just "role in `CUSTOMER_ROLES`" — it's a fallback chain, since neither role list needs to be exhaustive: a role in `CUSTOMER_ROLES` → customer; else a role in `CSM_ROLES` → CSM; else — including when entity-service has no record for the recipient at all — the recipient's email domain (`@wso2.com` → CSM, anything else → customer). See `Resolver.linkFor`'s doc comment for the full reasoning.

### Event bus (Azure Event Hub)

Required — unlike the channels above, this is this service's core purpose, so a missing value fails startup loudly. This service is a pure consumer; the backends publish directly to `EVENT_HUB_TOPIC` themselves.

| Variable | Description |
|---|---|
| `EVENT_HUB_BROKER` | Kafka bootstrap address: `<namespace>.servicebus.windows.net:9093` |
| `EVENT_HUB_CONNECTION_STRING` | The namespace's Shared Access Policy connection string (Namespace > Shared access policies > a policy's Primary Connection String); used as the SASL/PLAIN password. Never commit a real value |
| `EVENT_HUB_TOPIC` | Event Hub (Kafka topic) name, e.g. `case-events` |
| `EVENT_HUB_CONSUMER_GROUP` | Consumer group ID the main consumer's instances join. Optional — defaults to `csm-notification-service` |
| `MAIN_CONSUMER_COUNT` | How many concurrent consumer instances to run for `EVENT_HUB_TOPIC`. Optional — defaults to `1`; keep at or below the topic's real partition count |

### Dead-letter queue

Required — a record that exhausts the main consumer's retries is published here rather than dropped; without a valid topic here, that would fail loudly at publish time instead.

| Variable | Description |
|---|---|
| `EVENT_HUB_DLQ_TOPIC` | A second Event Hub in the same namespace, provisioned separately in Azure |
| `EVENT_HUB_DLQ_CONSUMER_GROUP` | Consumer group ID the DLQ consumer's instances join. Optional — defaults to `csm-notification-service-dlq` |
| `DLQ_CONSUMER_COUNT` | How many concurrent consumer instances to run for `EVENT_HUB_DLQ_TOPIC`. Optional — defaults to `1`; same partition-count guidance as `MAIN_CONSUMER_COUNT` |

### SLA timer engine

Optional, gated on `REDIS_URL` or `REDIS_ADDR` — unset (both) means `internal/slaengine` neither consumes `sla.clock.register` nor ticks. Ported from a standalone POC: registers a per-case SLA clock (durable state on entity-service's `sla_clocks` table, with durations entity-service computes from case severity per WSO2's own [support policy](https://wso2.com/licenses/support-policy/6.0)) when it sees a `sla.clock.register` event, tracks 50%/75%/100% elapsed via a Redis wake index, publishes `sla.tier_reached`, and sends a Google Chat breach alert directly (not routed through `internal/dispatch`) when a ticker finds a due, still-unresolved entry. Pausing, resuming, and completing a clock early (e.g. a support engineer's first response) never touch this service at all — those are direct, in-process writes from entity-service's own case-handling code straight to its `sla_clocks` table; see that repo's `CLAUDE.md`.

`REDIS_URL` (a `rediss://:<password>@<host>:<port>` connection string, parsed with `redis.ParseURL`) is how a managed, TLS-only Redis is configured — Azure Managed Redis, Azure Cache for Redis — since the `rediss` scheme makes go-redis dial with TLS automatically; takes priority over `REDIS_ADDR`/`REDIS_PASSWORD` when set. `REDIS_ADDR`/`REDIS_PASSWORD` remain the plain, non-TLS pair for a local Redis.

The client is a plain `redis.NewClient` — it only supports a non-clustered Redis (a real standalone instance, or a managed Redis under a non-clustered/"Enterprise" clustering policy, where the provider's own proxy hides the sharding). It does **not** support "OSS Cluster" policy, which needs a cluster-aware client to follow `MOVED`/`ASK` redirects. Confirm the target resource's clustering policy before pointing `REDIS_URL` at it.

This engine's own narrow `sla_clocks` client talks to the same entity-service as `CUSTOMER_ENTITY_BASE_URL`/`CUSTOMER_ENTITY_SCOPES` (see [Customer entity service](#customer-entity-service) above) — not a different backend — so it reuses those same two variables, plus the shared `OAUTH2_*` credentials (all required once `REDIS_URL` or `REDIS_ADDR` is set), rather than a redundant `SLA_ENTITY_*` pair.

| Variable | Description |
|---|---|
| `REDIS_URL` | `rediss://:<url-encoded-password>@<host>:<port>` connection string for a TLS Redis (Azure Managed Redis/Azure Cache for Redis). Percent-encode the password if it contains `+`, `/`, or `=`. Takes priority over `REDIS_ADDR`/`REDIS_PASSWORD` |
| `REDIS_ADDR` | Redis address for a plain, non-TLS Redis, e.g. `localhost:6379`. Ignored when `REDIS_URL` is set. Unset (with `REDIS_URL` also unset) disables this whole engine |
| `REDIS_PASSWORD` | Optional — empty for a local Redis with no auth. Ignored when `REDIS_URL` is set |
| `SLA_CONSUMER_GROUP` | Consumer group ID this engine's own consumer instances join — independent from `EVENT_HUB_CONSUMER_GROUP`/`EVENT_HUB_DLQ_CONSUMER_GROUP`. Optional — defaults to `csm-notification-service-sla` |
| `SLA_CONSUMER_COUNT` | How many concurrent consumer instances to run. Optional — defaults to `1` |
| `SLA_TICK_INTERVAL` | How often the ticker scans the Redis wake index for due tiers. Optional — defaults to `15s` |

### Billable status engine

Always started (no Redis/state dependency, unlike the SLA timer engine above). `internal/timecardengine.Engine` consumes `case.billable_status_changed` — published by entity-service's Postgres data source when a case's severity crosses into or out of `LOW` — on its own dedicated consumer group, for the same reason the SLA timer engine has one: `eventbus.Consumer` processes one record at a time, fully sequentially, so a future bulk time-card update must not delay unrelated email/Chat delivery on `dispatch.Dispatcher`'s own consumer group.

**Currently log-only.** Entity-service has no `time_cards` table on its Postgres data source yet (time cards are ServiceNow-only there), so there's no bulk-update reaction to perform — and entity-service's own `Publish` call for this event is itself still commented out. This consumer group exists ahead of need: the plumbing (topic wiring, retry/DLQ behavior, schema validation) is in place and ready for when that reaction is built.

| Variable | Description |
|---|---|
| `TIME_CARD_CONSUMER_GROUP` | Consumer group ID this engine's own consumer instances join. Optional — defaults to `csm-notification-service-time-card` |
| `TIME_CARD_CONSUMER_COUNT` | How many concurrent consumer instances to run. Optional — defaults to `1` |

### Server

| Variable | Description |
|---|---|
| `PORT` | Server listen port — a plain number, not an address (default `8080`) |

## Project Structure

```text
csm-notification-service/
├── cmd/
│   └── server/main.go           # Entry point — starts the HTTP health server + both consumer groups
├── internal/
│   ├── apierror/               # Typed upstream error type (4xx/5xx passthrough)
│   ├── middleware/
│   │   ├── correlation.go      # X-CSM-Correlation-ID propagation + slog enrichment
│   │   ├── logger.go           # Per-request access log
│   │   └── security_headers.go # X-Content-Type-Options, CSP, HSTS on every response
│   ├── notifications/
│   │   ├── doc.go              # Package overview — one config/client pair per channel
│   │   ├── email.go            # EmailConfig/EmailClient/SendEmail
│   │   ├── googlechat.go       # GoogleChatConfig/GoogleChatClient/SendIncidentAlert
│   │   ├── twilio.go           # TwilioConfig/TwilioClient/SendSMS+MakeCall
│   │   └── templates/          # HTML email templates + templates.go's Render* functions
│   ├── events/
│   │   ├── events.go           # Envelope + per-Type payload structs (the event schema)
│   │   └── validate.go         # Validate — the only remaining validation boundary
│   ├── entity/
│   │   └── customer.go         # Minimal read-only entity-service client (POST /users/search only)
│   ├── recipientlinks/
│   │   └── resolver.go         # Resolver.ResolveLinks — per-recipient customer/CSM portal link
│   ├── eventbus/
│   │   ├── config.go            # Config + SASL/PLAIN setup + PartitionCount, shared by producer/consumer
│   │   ├── producer.go          # Producer — publish a record, wait for ack
│   │   └── consumer.go          # Consumer — consumer-group poll loop, retry, OnExhausted, commit
│   ├── dispatch/
│   │   └── dispatch.go          # Dispatcher.Handle — envelope → validate → resolve links → group → template → EmailClient
│   └── slaengine/
│       ├── client.go            # EntityClient — narrow HTTP client for entity-service's sla_clocks endpoints
│       ├── redis.go             # WakeIndex — the Redis ZSET scheduling index
│       └── engine.go            # Engine.Handle (register clocks) + Engine.Tick/RunTicker (fire due tiers)
├── .env                         # Local config (git-ignored)
└── go.mod
```

## Running locally

```bash
# from integrations/csm-notification-service
go run ./cmd/server/main.go
```

The server auto-loads `.env` from the working directory at startup (silently ignored if absent).

## Commands

```bash
go vet ./...              # vet
go test -race ./...       # vet + race-detector tests
go build -o server ./cmd/server   # compile
```

## API Endpoints

- `GET /health` — Health check (Choreo's liveness probe). This is the only inbound HTTP route this service has — everything else happens via the two Kafka consumer groups described in [Event-driven notifications](#event-driven-notifications).

## Security

- **Never commit secrets** — client IDs/secrets, webhook URLs, and service URLs with credentials must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — log only IDs and error summaries
- **No app-level inbound auth** — this is intentional (see above), not an oversight
- **Input validation** — `events.Validate`, called from `dispatch.Dispatcher.Handle`, is the only validation boundary this service has left; keep rejecting unexpected input there rather than letting it reach a notification client
- **No recipient emails in logs** — `internal/recipientlinks`'s role-lookup warnings log `caseID`/`roles`/`userType`, never the recipient's email address (PII); keep it that way if this code changes
- **Security fixes in PRs** — describe security-related changes in neutral functional terms only, not called out as security fixes in the title/description
