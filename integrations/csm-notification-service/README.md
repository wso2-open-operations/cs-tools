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

- **`internal/events`** — the event schema and its only remaining validation boundary. `Envelope{Type, EntityID, Payload}` plus one payload struct per `Type` (`case.created`, `case.comment_added`, `case.status_changed`, `case.assigned`, `case.acknowledged`, `case.severity_changed`, `incident.created`, the two `change_request.*` notices, and `project_contact.invited` — see [Customer onboarding](#customer-onboarding-project_contactinvited)), each carrying every value its matching reaction needs. `case.acknowledged` is Chat-only (no email/`recipients`); `case.severity_changed` has both an email and a Chat reaction, same as `case.created`. `EntityID` is a case ID for the `case.*` types or an incident ID for `incident.created` — whatever this event is about; for the `case.*` types, it must match the payload's own `caseId`. `Validate(entityID, type, payload)` decodes strictly and checks required fields — moved here from a since-removed HTTP handler, since there's no request boundary to validate at anymore; `dispatch.Dispatcher.Handle` calls it before rendering/sending anything. Payloads still carry denormalized display values (names/titles) this service has no other way to obtain, but no longer carry pre-built case/comment links — five of the six `case.*` payloads (every one except `case.acknowledged`, which is Chat-only) carry `projectId`/`caseId` (and `commentId`, for `case.comment_added`) instead, and `internal/dispatch` resolves each recipient's own portal-appropriate link itself via `internal/recipientlinks`. `incident.created` is the one type with two independent reactions (a Google Chat alert *and* a voice call) rather than an email. Its call destination (`callTo`) is caller-supplied or falls back to `INCIDENT_DEFAULT_CALL_TO`; its Chat alert's portal link is *not* caller-supplied — like `case.created`'s, it's built by this service itself (`recipientlinks.Resolver.IncidentLink`), not trusted from the payload — and its `product` falls back to `DEFAULT_CHAT_PRODUCT` (see below) the same way `case.created`'s does. `case.created` also posts a Google Chat alert alongside its email — the same `SendIncidentAlert` call incident.created uses — but always to the CSM portal's case link (`recipientlinks.Resolver.CSMLink`), not a per-recipient link, since a Chat post has no per-recipient audience the way an email does.
- **`internal/entity`** — a minimal customer-entity-service client implementing exactly two endpoints: `POST /users/search` (backs `internal/recipientlinks`'s per-recipient role lookup) and `PUT /onboarding-steps/{membershipSfId}/{step}` (`RecordOnboardingStep`, the one write — `project_contact.invited`'s step outcomes), unlike `apps/csm-portal/backend`'s own entity client, a ~60-method passthrough surface. Not a notification channel — doesn't follow the `<Name>Config`/`<Name>Client`-in-`internal/notifications` pattern below, since it's an upstream data client, not something that sends a notification itself.
- **`internal/recipientlinks`** — `Resolver.ResolveLinks(ctx, emails, projectID, caseID)` looks up each email's role via `internal/entity` and returns the case link appropriate to their portal (customer vs CSM), with a role → role → userType → CSM-default fallback chain. A per-*recipient* decision, not per-event: the same `case.comment_added` notification can go to both a customer watcher and an internal CSM watcher at once, each needing a different link.
- **`internal/eventbus`** — a thin wrapper around [`github.com/segmentio/kafka-go`](https://github.com/segmentio/kafka-go) (a pure-Go Kafka client, no cgo — keeps this service on Choreo's buildpack deploy, MIT licensed) for Azure Event Hub's Kafka-compatible endpoint. `Producer.Publish` does a synchronous produce — this service's own use of it today is only for publishing to the dead-letter topic, since the main topic's producer side now lives in the backends. `Consumer.Run(ctx, handle, onExhausted)` polls a consumer group, retries a failing record `handleAttempts` (3) times with a fixed delay, then calls `onExhausted` (or logs at ERROR and drops, if `onExhausted` is nil) before committing either way. `PartitionCount(ctx, cfg)` reports a topic's real partition count, used at startup to sanity-check a configured consumer count. See CLAUDE.md for the franz-go → kafka-go swap rationale and its two known trade-offs.
- **`internal/dispatch`** — `Dispatcher.Handle` implements `eventbus.Handle`: decode the record as an `events.Envelope`, validate it (`events.Validate`), then for the four `case.*` types, resolve each recipient's own case link (`groupByLink`, via `internal/recipientlinks`), bucket recipients by the link they resolved to, and render+send one email per distinct link (`sendPerGroup`) — recipients sharing a link still batch into one `SendEmail` call. `case.created` additionally posts a Google Chat alert to the CSM portal's case link, independent of the email step (a failure in one doesn't block the other). `incident.created` skips the email/link-resolution path entirely and sends a Google Chat alert plus places a voice call directly from the payload's own fields (falling back to `DEFAULT_CHAT_PRODUCT`/`INCIDENT_DEFAULT_CALL_TO` when the payload omits `product`/`callTo`).
- **`internal/scim`** — a minimal SCIM operations service client (`EnsureExternalUser` → `POST /organizations/external/users`, 201 created / 200 already existed) used only by the `project_contact.invited` handler to create an invitee's Asgardeo user. Not a notification channel, so it lives beside `internal/entity` rather than in `internal/notifications`.
- **`project_contact.invited`** — the customer onboarding flow's consumer side, published by entity-service once a Salesforce project-contact membership in state INVITED / RE-INVITED is in its database. Two sequential steps, each behind its own default-off flag and each recorded on entity-service's onboarding-step ledger as SUCCEEDED / FAILED / SKIPPED: **IDENTITY** (`CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED`) creates the Asgardeo user via `internal/scim`; **EMAIL** (`CSM_MIGRATION_ONBOARD_EMAIL_ENABLED`) sends the invitation — the "welcome, your account was created" template for a new user, the "project added to your account" template when SCIM said the user already existed — to the invitee alone, linking to `ONBOARD_PORTAL_URL`. An integration user records both steps SKIPPED and gets nothing. A failed step records FAILED with the error and returns it (normal retry/DLQ path); a retry after an email failure reuses the first attempt's identity answer rather than asking SCIM again. Step recording is best-effort: a ledger write failing is logged, never turned into a retry. Arrives on its own topic (`PROJECT_EVENT_HUB_TOPIC`, default `project-events`, with `PROJECT_EVENT_HUB_DLQ_TOPIC` as its dead-letter queue), not the main case-events one. Before sending, the handler checks entity-service's ledger and skips a membership whose EMAIL step already succeeded — the guard that stops a portal invitation and the Salesforce event it produces from both inviting the same contact. A **resend** (`isResend` on the payload, set when an admin presses "Resend invitation") deliberately bypasses that check and sends a third, short reminder template instead: the guard is for an accidental second invitation, not a requested one, and by then the Asgardeo account exists, so neither the "welcome" nor the "you already have an account" wording fits someone who may never have seen the first email. The EMAIL step is still recorded, so the ledger's attempt count keeps counting. See CLAUDE.md for the full reasoning.
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

### Customer onboarding (`project_contact.invited`)

Both flags default off, so a deployment without them records both steps as SKIPPED and sends nothing. The SCIM client authenticates with the shared `OAUTH2_*` app above, same as the email and entity clients; step recording reuses the entity client, and entity-service requires this service's OAuth2 client id to be in its `AUTH_INTERNAL_CLIENT_IDS` for the `/onboarding-steps` endpoints.

| Variable | Description |
|---|---|
| `CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED` | Set to `true` to create the invitee's Asgardeo user via the SCIM operations service; unset/anything else records IDENTITY as SKIPPED (default off) |
| `CSM_MIGRATION_ONBOARD_EMAIL_ENABLED` | Set to `true` to send the invitation email; unset/anything else records EMAIL as SKIPPED (default off). Still subject to `EMAIL_SENDING_ENABLED` and `EMAIL_DEBUG_MODE` above |
| `SCIM_BASE_URL` | Base URL of the SCIM operations service (`POST /organizations/external/users`). Optional — required in practice once `CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED=true` (a startup warning is logged if missing) |
| `SCIM_SCOPES` | Comma-separated OAuth2 scopes for the SCIM operations service (optional) — shared `OAUTH2_*` credentials, not its own |
| `ONBOARD_EMAIL_FROM` | Sender address for the invitation email, sent through the same email client as everything else. Optional — defaults to `EMAIL_FROM_ADDRESS` |
| `ONBOARD_PORTAL_URL` | Sign-in link the invitation points at. Optional — defaults to `https://support.wso2.com` |

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

### Customer-onboarding topics

`project_contact.invited` rides its own topic pair, not `EVENT_HUB_TOPIC` — a separate consumer group isolates processing, only a separate topic isolates volume, and an invitation backlog must never sit behind a flood of case events. Its dead-letter queue is likewise its own, so the onboarding DLQ can be watched on its own. Both are consumed by the same `Dispatcher.Handle` as every other topic. All optional (the defaults are what entity-service publishes to), but both Event Hubs must be provisioned in Azure before deploying — this service doesn't create topics itself.

| Variable | Description |
|---|---|
| `PROJECT_EVENT_HUB_TOPIC` | Event Hub carrying the onboarding events. Optional — defaults to `project-events` |
| `PROJECT_EVENT_HUB_DLQ_TOPIC` | Its dead-letter Event Hub. Optional — defaults to `project-events-dlq` |
| `PROJECT_CONSUMER_GROUP` | Consumer group ID for `PROJECT_EVENT_HUB_TOPIC`. Optional — defaults to `csm-notification-service-project` |
| `PROJECT_DLQ_CONSUMER_GROUP` | Consumer group ID for `PROJECT_EVENT_HUB_DLQ_TOPIC`. Optional — defaults to `csm-notification-service-project-dlq` |
| `PROJECT_CONSUMER_COUNT` | How many concurrent consumer instances to run for `PROJECT_EVENT_HUB_TOPIC`. Optional — defaults to `1`; same partition-count guidance as `MAIN_CONSUMER_COUNT` |
| `PROJECT_DLQ_CONSUMER_COUNT` | Same, for `PROJECT_EVENT_HUB_DLQ_TOPIC`. Optional — defaults to `1` |

### SLA breach-alerting engine

Optional, gated on `REDIS_URL` or `REDIS_ADDR` — unset (both) means `internal/slaengine` never polls. Not a Kafka consumer: on a plain ticker, it polls entity-service's `GET /sla-status` (backed by the real, ServiceNow-synced `sla` table, not a value this service computes itself), diffs each clock's live elapsed percentage against the last tier it alerted for (a small cursor per `(caseId, clockType)` kept in Redis), and — on a genuinely new 50%/75%/100% crossing since its last poll — publishes `sla.tier_reached` and sends a Google Chat breach alert directly (not routed through `internal/dispatch`). The first time this engine ever sees a given clock, it seeds the cursor at that clock's *current* tier without alerting — avoiding an alert flood from every SLA clock already in progress the moment this engine starts polling; only a tier crossed on a later poll is a genuine new crossing. Replaces an earlier design that registered a durable clock per case on a now-removed entity-service `sla_clocks` table (a stand-in built before the real `sla` table existed) and scheduled Redis wake-ups off a locally-computed due date — see entity-service's own `CLAUDE.md` ("SLA status") for the full history. Pausing/resuming a clock never needs a signal from this service either: ServiceNow's own SLA engine freezes `businessElapsedPercent` while paused, so a paused clock's tier simply doesn't advance until it resumes.

`REDIS_URL` (a `rediss://:<password>@<host>:<port>` connection string, parsed with `redis.ParseURL`) is how a managed, TLS-only Redis is configured — Azure Managed Redis, Azure Cache for Redis — since the `rediss` scheme makes go-redis dial with TLS automatically; takes priority over `REDIS_ADDR`/`REDIS_PASSWORD` when set. `REDIS_ADDR`/`REDIS_PASSWORD` remain the plain, non-TLS pair for a local Redis.

The client is a plain `redis.NewClient` — it only supports a non-clustered Redis (a real standalone instance, or a managed Redis under a non-clustered/"Enterprise" clustering policy, where the provider's own proxy hides the sharding). It does **not** support "OSS Cluster" policy, which needs a cluster-aware client to follow `MOVED`/`ASK` redirects. Confirm the target resource's clustering policy before pointing `REDIS_URL` at it.

This engine's own narrow entity-service client talks to the same entity-service as `CUSTOMER_ENTITY_BASE_URL`/`CUSTOMER_ENTITY_SCOPES` (see [Customer entity service](#customer-entity-service) above) — not a different backend — so it reuses those same two variables, plus the shared `OAUTH2_*` credentials (all required once `REDIS_URL` or `REDIS_ADDR` is set), rather than a redundant `SLA_ENTITY_*` pair.

| Variable | Description |
|---|---|
| `REDIS_URL` | `rediss://:<url-encoded-password>@<host>:<port>` connection string for a TLS Redis (Azure Managed Redis/Azure Cache for Redis). Percent-encode the password if it contains `+`, `/`, or `=`. Takes priority over `REDIS_ADDR`/`REDIS_PASSWORD` |
| `REDIS_ADDR` | Redis address for a plain, non-TLS Redis, e.g. `localhost:6379`. Ignored when `REDIS_URL` is set. Unset (with `REDIS_URL` also unset) disables this whole engine |
| `REDIS_PASSWORD` | Optional — empty for a local Redis with no auth. Ignored when `REDIS_URL` is set |
| `SLA_TICK_INTERVAL` | How often this engine polls `GET /sla-status` and diffs tiers. Optional — defaults to `5m`. Most active SLA clocks don't change more than a few times a day, so a short interval mostly just adds load without meaningfully lowering alert latency |

### Billable status engine

Always started (no Redis/state dependency, unlike the SLA engine above — it's a plain Kafka consumer). `internal/timecardengine.Engine` consumes `case.billable_status_changed` — published by entity-service's Postgres data source when a case's severity crosses into or out of `LOW` — on its own dedicated consumer group: `eventbus.Consumer` processes one record at a time, fully sequentially, so a future bulk time-card update must not delay unrelated email/Chat delivery on `dispatch.Dispatcher`'s own consumer group.

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
│   │   └── templates/          # HTML email templates (case.*, change_request.*, project_contact_invited_*) + templates.go's Render* functions
│   ├── events/
│   │   ├── events.go           # Envelope + per-Type payload structs (the event schema)
│   │   └── validate.go         # Validate — the only remaining validation boundary
│   ├── entity/
│   │   ├── customer.go         # Minimal entity-service client (POST /users/search)
│   │   └── onboarding.go       # RecordOnboardingStep (PUT /onboarding-steps/{membershipSfId}/{step})
│   ├── scim/
│   │   └── client.go           # SCIM operations service client (EnsureExternalUser) for project_contact.invited
│   ├── recipientlinks/
│   │   └── resolver.go         # Resolver.ResolveLinks — per-recipient customer/CSM portal link
│   ├── eventbus/
│   │   ├── config.go            # Config + SASL/PLAIN setup + PartitionCount, shared by producer/consumer
│   │   ├── producer.go          # Producer — publish a record, wait for ack
│   │   └── consumer.go          # Consumer — consumer-group poll loop, retry, OnExhausted, commit
│   ├── dispatch/
│   │   └── dispatch.go          # Dispatcher.Handle — envelope → validate → resolve links → group → template → EmailClient; handleProjectContactInvited (SCIM → invitation → step ledger)
│   └── slaengine/
│       ├── client.go            # EntityClient — narrow HTTP client for entity-service's GET /sla-status
│       ├── redis.go             # TierStore — last-alerted-tier cursor per (caseId, clockType)
│       └── engine.go            # Engine.Tick/RunTicker — poll, diff tiers, alert on new crossings
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
