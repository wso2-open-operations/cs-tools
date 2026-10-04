# CSM Notification Service

Go background worker (`net/http`, Go 1.26+) with no inbound API beyond a health check. `csm-portal-backend` and `customer-portal-backend` publish domain events (case created, comment added, status changed, case assigned, incident created) directly to Azure Event Hub; this service's own consumers read them back and react asynchronously by sending email, Google Chat alerts, and voice calls. See [Event-driven notifications](#event-driven-notifications).

This service was extracted from `apps/csm-portal/backend/internal/notifications`, which previously hosted the same email/Google Chat clients. That backend no longer constructs or calls any notification client directly. This service itself used to expose `POST /events` (backends called it, and it published to Event Hub on their behalf) — that producer-side hop was removed once the backends took over publishing directly; this service is a pure consumer now.

## Why no `Auth` middleware

The only route this service exposes is `GET /health` (Choreo's liveness probe) — there's no end-user or caller identity to check, since everything else is Kafka consumption, not an HTTP request.

## Current scope — TODO

- **SMS and direct call channels are unused.** `TwilioClient.SendSMS` has no caller — `MakeCall` is only invoked by `incident.created`.

Retries are two-tiered with exponential backoff and jitter (`eventbus.RetryPolicy`): the main consumer retries a failing record (default 5 attempts over roughly 15–30 s), then publishes it to the dead-letter topic stamped with an `x-retry-not-before` header (`DLQ_RETRY_DELAY`, default 5 min); the DLQ consumer waits until then and retries again (default 5 attempts over roughly 4–7.5 min). A record that exhausts that tier too — or whose dead-letter publish fails — is **parked**: published whole to `EVENT_HUB_PARKING_TOPIC` with its failure reason for a manual replay (see [Parked records](#parked-records)), and logged at ERROR. Without a parking topic it is logged and dropped; never silently acknowledged.

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
| `notifications` | Hosts `EmailClient`/`SendEmail` (`email.go`, OAuth2 client-credentials auth), `GoogleChatClient`/`Send*Alert` (`googlechat.go`, per-audience incoming-webhook auth — see `GOOGLE_CHAT_SPACES` below), and `TwilioClient`/`SendSMS`+`MakeCall` (`twilio.go`, HTTP Basic Auth — sms and call are two methods on one client, since both are the same Twilio account/auth) |

Each channel gets its own config/client pair in its own file, since channels differ in upstream auth scheme. All three clients are constructed once in `cmd/server/main.go` and handed to `dispatch.NewDispatcher` — a new channel follows the same client pattern, then gets wired into `Dispatcher` for whichever event type should trigger it (see "Adding a new event type" in CLAUDE.md).

## Event-driven notifications

```text
csm-portal-backend ──┐
                      ├──▶ Event Hub topic ──▶ main consumer group ──▶ dispatch.Dispatcher.Handle
customer-portal-backend ┘                           │                  (render email or send incident alerts)
                                                      │ (retries exhausted)
                                                      ▼
                                              Event Hub DLQ topic ──▶ DLQ consumer group ──▶ dispatch.Dispatcher.Handle
                                                                       (waits for x-retry-not-before, then a
                                                                        slower retry pass, same Handle)
                                                                                │ (retries exhausted)
                                                                                ▼
                                                                  Event Hub parking topic (manual replay)
```

Both backends publish directly to the main topic — there is no HTTP hop through this service. Two packages implement the bus→consumer side; `internal/notifications` (templates + channel clients) is the last-mile sender they call.

- **`internal/events`** — the event schema and its only remaining validation boundary. `Envelope{Type, EntityID, Payload}` plus one payload struct per `Type` (`case.created`, `case.comment_added`, `case.status_changed`, `case.assigned`, `case.acknowledged`, `case.severity_changed`, `incident.created`, the two `change_request.*` notices, and `project_contact.invited` — see [Customer onboarding](#customer-onboarding-project_contactinvited)), each carrying every value its matching reaction needs. `case.acknowledged` is Chat-only (no email/`recipients`); `case.severity_changed` has both an email and a Chat reaction, same as `case.created`. `EntityID` is a case ID for the `case.*` types or an incident ID for `incident.created` — whatever this event is about; for the `case.*` types, it must match the payload's own `caseId`. `Validate(entityID, type, payload)` decodes strictly and checks required fields — moved here from a since-removed HTTP handler, since there's no request boundary to validate at anymore; `dispatch.Dispatcher.Handle` calls it before rendering/sending anything. Payloads still carry denormalized display values (names/titles) this service has no other way to obtain, but no longer carry pre-built case/comment links — five of the six `case.*` payloads (every one except `case.acknowledged`, which is Chat-only) carry `projectId`/`caseId` (and `commentId`, for `case.comment_added`) instead, and `internal/dispatch` resolves each recipient's own portal-appropriate link itself via `internal/recipientlinks`. `incident.created` has exactly one reaction — a voice call — not an email, per explicit product direction (an incident pages on-call directly; a separate Chat post was redundant with that). Its call destination (`callTo`) is caller-supplied or falls back to `INCIDENT_DEFAULT_CALL_TO`. `case.created`/`case.acknowledged`/`case.severity_changed` each post their own Google Chat alert (via their own `Send*Alert` method), always to the fixed `"Incident Monitor"` audience — no team detection, no per-product routing — and, for `case.created`/`case.severity_changed`, alongside their email (a failure in one doesn't block the other); `case.acknowledged` is Chat-only. `case.created`'s/`case.acknowledged`'s/`case.severity_changed`'s Chat alerts always target the CSM portal's case link (`recipientlinks.Resolver.CSMLink`), not a per-recipient link, since a Chat post has no per-recipient audience the way an email does.
- **`internal/entity`** — a minimal customer-entity-service client implementing exactly two endpoints: `POST /users/search` (backs `internal/recipientlinks`'s per-recipient role lookup) and `PUT /onboarding-steps/{membershipSfId}/{step}` (`RecordOnboardingStep`, the one write — `project_contact.invited`'s step outcomes), unlike `apps/csm-portal/backend`'s own entity client, a ~60-method passthrough surface. Not a notification channel — doesn't follow the `<Name>Config`/`<Name>Client`-in-`internal/notifications` pattern below, since it's an upstream data client, not something that sends a notification itself.
- **`internal/recipientlinks`** — `Resolver.ResolveLinks(ctx, emails, projectID, caseID)` looks up each email's role via `internal/entity` and returns the case link appropriate to their portal (customer vs CSM), with a customer-role → CSM-role → email-domain fallback chain (a recipient whose roles match neither list, or who has no entity-service record, gets the CSM link for a wso2.com address and the customer link otherwise). A per-*recipient* decision, not per-event: the same `case.comment_added` notification can go to both a customer watcher and an internal CSM watcher at once, each needing a different link.
- **`internal/eventbus`** — a thin wrapper around [`github.com/segmentio/kafka-go`](https://github.com/segmentio/kafka-go) (a pure-Go Kafka client, no cgo — keeps this service on Choreo's buildpack deploy, MIT licensed) for Azure Event Hub's Kafka-compatible endpoint. `Producer.Publish` does a synchronous produce — this service's own use of it today is only for publishing to the dead-letter topic, since the main topic's producer side now lives in the backends. `Consumer.Run(ctx, handle, onExhausted)` polls a consumer group, retries a failing record on its `RetryPolicy` (exponential backoff with jitter), then calls `onExhausted` (the dead-letter publish) or, on a last tier (`onExhausted` nil), parks the record (`WithParking`) — committing either way. On shutdown it stops fetching but lets the in-flight record finish and commit within `SHUTDOWN_DRAIN_TIMEOUT`. `Consumer.Status()` feeds `/health`. `PartitionCount(ctx, cfg)` reports a topic's real partition count, used at startup to sanity-check a configured consumer count. See CLAUDE.md for the franz-go → kafka-go swap rationale and its two known trade-offs.
- **`internal/dispatch`** — `Dispatcher.Handle` implements `eventbus.Handle`: decode the record as an `events.Envelope`, validate it (`events.Validate`), then for the four `case.*` types, resolve each recipient's own case link (`groupByLink`, via `internal/recipientlinks`), bucket recipients by the link they resolved to, and render+send one email per distinct link (`sendPerGroup`) — recipients sharing a link still batch into one `SendEmail` call. `case.created`/`case.severity_changed` additionally post a Google Chat alert to the CSM portal's case link (fixed `"Incident Monitor"` audience), independent of the email step (a failure in one doesn't block the other); `case.acknowledged` is Chat-only. `incident.created` skips the email/link-resolution path entirely and places a voice call directly from the payload's own `callTo` field (falling back to `INCIDENT_DEFAULT_CALL_TO` when the payload omits it) — no Google Chat reaction.
- **`internal/scim`** — a minimal SCIM operations service client (`EnsureExternalUser` → `POST /organizations/external/users`, 201 created / 200 already existed) used only by the `project_contact.invited` handler to create an invitee's Asgardeo user. Not a notification channel, so it lives beside `internal/entity` rather than in `internal/notifications`.
- **`project_contact.invited`** — the customer onboarding flow's consumer side, published by entity-service once a Salesforce project-contact membership in state INVITED / RE-INVITED is in its database. Two sequential steps, each behind its own default-off flag and each recorded on entity-service's onboarding-step ledger as SUCCEEDED / FAILED / SKIPPED: **IDENTITY** (`CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED`) creates the Asgardeo user via `internal/scim`; **EMAIL** (`CSM_MIGRATION_ONBOARD_EMAIL_ENABLED`) sends the invitation — the "welcome, your account was created" template for a new user, the "project added to your account" template when SCIM said the user already existed — to the invitee alone, linking to `ONBOARD_PORTAL_URL`. An integration user records both steps SKIPPED and gets nothing. A failed step records FAILED with the error and returns it (normal retry/DLQ path); a retry after an email failure reuses the first attempt's identity answer rather than asking SCIM again. Step recording is best-effort: a ledger write failing is logged, never turned into a retry. Arrives on its own topic (`PROJECT_EVENT_HUB_TOPIC`, default `project-events`, with `PROJECT_EVENT_HUB_DLQ_TOPIC` as its dead-letter queue), not the main case-events one. Before sending, the handler checks entity-service's ledger and skips a membership whose EMAIL step already succeeded — the guard that stops a portal invitation and the Salesforce event it produces from both inviting the same contact. A **resend** (`isResend` on the payload, set when an admin presses "Resend invitation") deliberately bypasses that check and sends a third, short reminder template instead: the guard is for an accidental second invitation, not a requested one, and by then the Asgardeo account exists, so neither the "welcome" nor the "you already have an account" wording fits someone who may never have seen the first email. The EMAIL step is still recorded, so the ledger's attempt count keeps counting. See CLAUDE.md for the full reasoning.
- **`project_contact.registered`** — sends the Welcome email after a contact's first sign-in, once per membership (`WELCOME_EMAIL` step).
- **Dead-letter queue** — when the main consumer's `Handle` call fails on every attempt of its retry policy, the record is published to `EVENT_HUB_DLQ_TOPIC` (unchanged bytes, plus an `x-retry-not-before` header) instead of being dropped. A second, independent consumer group runs against that topic, using the same `Dispatcher.Handle`; it waits until the not-before time, then retries on the slower dead-letter policy. Exhausting that tier parks the record (see [Parked records](#parked-records)). The time-card consumer has no dead-letter tier of its own and parks directly. Provision `EVENT_HUB_DLQ_TOPIC` and `EVENT_HUB_PARKING_TOPIC` as their own Event Hubs in Azure before deploying; this service doesn't create topics itself.
- **Configurable consumer counts** — `MAIN_CONSUMER_COUNT`/`DLQ_CONSUMER_COUNT` each start that many independent `eventbus.Consumer` instances, all joining the same consumer group; Kafka's own rebalancing splits a topic's partitions across however many are actually running. Keep each count at or below its topic's real partition count — a startup check logs a warning (not a hard failure) if it isn't, since excess consumers just sit idle rather than causing an error.

## Configuration

Copy `.env.example` to `.env` and fill in the values:

### Email notification channel

| Variable | Description |
|---|---|
| `EMAIL_BASE_URL` | Base URL of the email notification service (optional) |
| `EMAIL_SCOPES` | Comma-separated OAuth2 scopes for the email service (optional) — authenticates with the shared `OAUTH2_CLIENT_ID`/`OAUTH2_CLIENT_SECRET`/`OAUTH2_TOKEN_URL` below, not its own credentials |
| `EMAIL_FROM_ADDRESS` | Fixed "From" address used for every outgoing email (optional) |
| `EMAIL_REPLY_TO` | Reply-To for onboarding emails only. Unset means `support@wso2.com`; empty means none |
| `EMAIL_SENDING_ENABLED` | Temporary killswitch for `case.*` email delivery — unset/anything but `false` means real sending. Checked before `EMAIL_DEBUG_MODE` below, so it silences email regardless of debug mode. Doesn't affect Google Chat or Twilio |
| `EMAIL_DEBUG_MODE` | Set to `true` to redirect every `case.*` email to `EMAIL_DEBUG_RECIPIENTS` instead of the event's real recipients (recipient links are still resolved against the real recipients either way) — the email still actually sends, just to a safe test list; unset/anything else means real sending to real recipients. Doesn't affect Google Chat or Twilio |
| `EMAIL_DEBUG_RECIPIENTS` | Comma-separated email addresses used instead of the real recipients when `EMAIL_DEBUG_MODE=true`. Empty + debug mode on skips that email entirely rather than sending to nobody |

### Google Chat notification channel

| Variable | Description |
|---|---|
| `GOOGLE_CHAT_SPACES` | JSON array of `{"audience","webhookUrl"}` objects, one per Google Chat space — the only Google Chat routing mechanism this service has. `audience` is either a real CRE team name (exact, case-sensitive match — see entity-service's `"group".name`) or one of the standing audiences `"Incident Monitor"`/`"Onboarding"`/`"Americas"`/`"Evaluation"`. `case.created`/`case.acknowledged`/`case.severity_changed` always route to the fixed `"Incident Monitor"` audience; SLA breach alerts resolve a real per-team/onboarding/time-of-day audience (see `internal/chataudience`). Optional — left unset or malformed, Google Chat alerts are unavailable but startup and every other endpoint work normally. An audience with no configured space is skipped (logged), not an error |

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
| `CALL_SENDING_ENABLED` | Temporary killswitch — set to `false` to log instead of actually placing `incident.created`'s Twilio call; unset/anything else means real calling. `incident.created` has no Google Chat reaction to affect — voice call only |
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
| `EVENT_HUB_PARKING_TOPIC` | Event Hub that receives records no retry tier will take any more (see [Parked records](#parked-records)). Optional only because it must be provisioned first — unset, such a record is logged at ERROR and dropped. Set it in every real deployment |
| `HANDLE_MAX_ATTEMPTS` | First-tier (main, change-request, onboarding, time-card) attempts per record. Optional — defaults to `5` |
| `HANDLE_RETRY_BASE_DELAY` / `HANDLE_RETRY_MAX_DELAY` | First-tier backoff: the pause doubles from the base up to the cap, each jittered to between half and all of it. Optional — default `2s` / `1m` |
| `DLQ_RETRY_DELAY` | How long after being dead-lettered a record waits before the dead-letter consumer first tries it. Optional — defaults to `5m` |
| `DLQ_HANDLE_MAX_ATTEMPTS` | Dead-letter-tier attempts per record. Optional — defaults to `5` |
| `DLQ_HANDLE_RETRY_BASE_DELAY` / `DLQ_HANDLE_RETRY_MAX_DELAY` | Dead-letter-tier backoff. Optional — default `30s` / `5m` |
| `CONSUMER_STALL_TIMEOUT` | `/health` reports 503 once a consumer has shown no progress (poll, handle, commit) for this long. Optional — defaults to `5m`; keep it well above one poll (30s) plus the slowest handler attempt |
| `SHUTDOWN_DRAIN_TIMEOUT` | On SIGTERM, how long an in-flight record may still run and commit before its context is cancelled. Optional — defaults to `20s`; keep it, plus the reader close, inside the platform's termination grace period |

### Change-request topics

The change-request notices (`change_request.*`) ride their own topic pair.

| Variable | Description |
|---|---|
| `CR_EVENT_HUB_TOPIC` | Event Hub carrying the change-request notices. Optional — defaults to `cr-events` |
| `CR_EVENT_HUB_DLQ_TOPIC` | Its dead-letter Event Hub. Optional — defaults to `cr-events-dlq` |
| `CR_CONSUMER_GROUP` | Consumer group ID for `CR_EVENT_HUB_TOPIC`. Optional — defaults to `csm-notification-service-cr` |
| `CR_DLQ_CONSUMER_GROUP` | Consumer group ID for `CR_EVENT_HUB_DLQ_TOPIC`. Optional — defaults to `csm-notification-service-cr-dlq` |
| `CR_CONSUMER_COUNT` | How many concurrent consumer instances to run for `CR_EVENT_HUB_TOPIC`. Optional — defaults to `1` |
| `CR_DLQ_CONSUMER_COUNT` | Same, for `CR_EVENT_HUB_DLQ_TOPIC`. Optional — defaults to `1` |

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

Optional, gated on `REDIS_URL` or `REDIS_ADDR` — unset (both) means `internal/slaengine` never polls. Not a Kafka consumer: on a plain ticker, it polls entity-service's `GET /sla-status` (backed by the real, data-source-synced `sla` table, not a value this service computes itself), diffs each clock's live elapsed percentage against the last tier it alerted for (a small cursor per `(caseId, clockType)` kept in Redis), and — on a genuinely new 50%/75%/100% crossing since its last poll — publishes `sla.tier_reached` and sends a Google Chat breach alert directly (not routed through `internal/dispatch`). The first time this engine ever sees a given clock, it seeds the cursor at that clock's *current* tier without alerting — avoiding an alert flood from every SLA clock already in progress the moment this engine starts polling; only a tier crossed on a later poll is a genuine new crossing. Replaces an earlier design that registered a durable clock per case on a now-removed entity-service `sla_clocks` table (a stand-in built before the real `sla` table existed) and scheduled Redis wake-ups off a locally-computed due date — see entity-service's own `CLAUDE.md` ("SLA status") for the full history. Pausing/resuming a clock never needs a signal from this service either: the backing data source's own SLA engine freezes `businessElapsedPercent` while paused, so a paused clock's tier simply doesn't advance until it resumes.

`REDIS_URL` (a `rediss://:<password>@<host>:<port>` connection string, parsed with `redis.ParseURL`) is how a managed, TLS-only Redis is configured — Azure Managed Redis, Azure Cache for Redis — since the `rediss` scheme makes go-redis dial with TLS automatically; takes priority over `REDIS_ADDR`/`REDIS_PASSWORD` when set. `REDIS_ADDR`/`REDIS_PASSWORD` remain the plain, non-TLS pair for a local Redis.

The client is a plain `redis.NewClient` — it only supports a non-clustered Redis (a real standalone instance, or a managed Redis under a non-clustered/"Enterprise" clustering policy, where the provider's own proxy hides the sharding). It does **not** support "OSS Cluster" policy, which needs a cluster-aware client to follow `MOVED`/`ASK` redirects. Confirm the target resource's clustering policy before pointing `REDIS_URL` at it.

This engine's own narrow entity-service client talks to the same entity-service as `CUSTOMER_ENTITY_BASE_URL`/`CUSTOMER_ENTITY_SCOPES` (see [Customer entity service](#customer-entity-service) above) — not a different backend — so it reuses those same two variables, plus the shared `OAUTH2_*` credentials (all required once `REDIS_URL` or `REDIS_ADDR` is set), rather than a redundant `SLA_ENTITY_*` pair.

| Variable | Description |
|---|---|
| `REDIS_URL` | `rediss://:<url-encoded-password>@<host>:<port>` connection string for a TLS Redis (Azure Managed Redis/Azure Cache for Redis). Percent-encode the password if it contains `+`, `/`, or `=`. Takes priority over `REDIS_ADDR`/`REDIS_PASSWORD` |
| `REDIS_ADDR` | Redis address for a plain, non-TLS Redis, e.g. `localhost:6379`. Ignored when `REDIS_URL` is set. Unset (with `REDIS_URL` also unset) disables this whole engine |
| `REDIS_PASSWORD` | Optional — empty for a local Redis with no auth. Ignored when `REDIS_URL` is set |
| `SLA_TICK_INTERVAL` | How often this engine polls `GET /sla-status` and diffs tiers. Optional — defaults to `5m`. Most active SLA clocks don't change more than a few times a day, so a short interval mostly just adds load without meaningfully lowering alert latency |

### Server

| Variable | Description |
|---|---|
| `PORT` | Server listen port — a plain number, not an address (default `8080`) |

## Parked records

A record is parked once no retry tier remains for it: the dead-letter consumer exhausted its attempts, the dead-letter publish itself failed, or the time-card consumer exhausted its attempts. Parking publishes one JSON message to `EVENT_HUB_PARKING_TOPIC`, keyed by the original record's key, shaped as `eventbus.ParkedRecord`:

```json
{"parkedAt": "…", "consumer": "case-events-dlq", "sourceTopic": "case-events-dlq", "sourcePartition": 0, "sourceOffset": 123,
 "key": "<entity id>", "attempts": 5, "failure": "dispatch: …: upstream returned 503", "record": { …the original envelope… }}
```

`record` is the original value byte for byte (`recordBase64` instead, when it was not valid JSON). `failure` is a summary with any upstream response body removed. Every park is also logged at ERROR (`eventbus: record exhausted every retry tier and was parked; manual replay required`) and counted in `/health`'s `parked` counter — alert on either.

**Replay**, once the cause is fixed: read the parking topic with any Kafka client using the namespace's connection string (SASL/PLAIN, username `$ConnectionString`), and for each message to replay, publish its `record` (or decoded `recordBase64`) **unchanged** to the main topic of the same family (`EVENT_HUB_TOPIC`, `CR_EVENT_HUB_TOPIC` or `PROJECT_EVENT_HUB_TOPIC`), using the parked message's key. It then gets the full retry schedule again. Replaying is safe to repeat for the onboarding emails (the ledger guards them); for case/incident/change-request notices a channel that had already succeeded before the record was parked will be sent again, so check the logs for what already went out. Give the parking topic the longest retention the namespace tier allows; nothing in this service consumes it.

## Project Structure

```text
csm-notification-service/
├── cmd/
│   └── server/
│       ├── main.go              # Entry point — config, consumer groups, retry/park wiring, drain on shutdown
│       └── health.go            # GET /health from consumer liveness; consumer registry + drain
├── internal/
│   ├── apierror/               # Typed upstream error type + Summary (body-free form for logs)
│   ├── middleware/
│   │   ├── correlation.go      # X-CSM-Correlation-ID propagation + slog enrichment
│   │   ├── logger.go           # Per-request access log
│   │   └── security_headers.go # X-Content-Type-Options, CSP, HSTS on every response
│   ├── notifications/
│   │   ├── doc.go              # Package overview — one config/client pair per channel
│   │   ├── email.go            # EmailConfig/EmailClient/SendEmail
│   │   ├── googlechat.go       # GoogleChatConfig/GoogleChatClient/Send*Alert (audience-keyed)
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
│   │   ├── producer.go          # Producer — publish a record (optionally with headers), wait for ack
│   │   ├── consumer.go          # Consumer — poll loop, retry, OnExhausted, park, commit, status, drain
│   │   ├── retry.go             # RetryPolicy — exponential backoff with jitter
│   │   └── parking.go           # ParkFunc/ParkedRecord — the parking-topic envelope
│   ├── dispatch/
│   │   ├── dispatch.go          # Dispatcher, Deps/Config, NewDispatcher, Handle (envelope → validate → route)
│   │   ├── case_handlers.go     # case.* handlers, groupByLink, sendPerGroup
│   │   ├── incident.go          # incident.created (voice call)
│   │   ├── change_request.go    # change_request.* notices
│   │   ├── onboarding.go        # project_contact.invited/registered (SCIM → invitation → step ledger)
│   │   ├── idempotency.go       # content-keyed per-record claims
│   │   └── display.go           # case references, subject line, severity/case-type labels
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

- `GET /health` — Health check (Choreo's liveness probe). 200 while every consumer is running and has made progress within `CONSUMER_STALL_TIMEOUT`; 503 once any consumer's poll loop has exited or stalled, so the platform restarts the instance. The JSON body lists each consumer's state, idle time, last fetch error and counters (handled, failed attempts, dead-lettered, parked, dropped). This is the only inbound HTTP route this service has — everything else happens via the Kafka consumer groups described in [Event-driven notifications](#event-driven-notifications).

## Security

- **Never commit secrets** — client IDs/secrets, webhook URLs, and service URLs with credentials must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — log only IDs and error summaries
- **No app-level inbound auth** — this is intentional (see above), not an oversight
- **Input validation** — `events.Validate`, called from `dispatch.Dispatcher.Handle`, is the only validation boundary this service has left; keep rejecting unexpected input there rather than letting it reach a notification client
- **No recipient emails in logs** — `internal/recipientlinks`'s role-lookup warnings log `caseID`/`found`/`isCustomer`, never the recipient's email address (PII); keep it that way if this code changes. Upstream response bodies are kept out of failure logs too (`apierror.Summary`)
- **Security fixes in PRs** — describe security-related changes in neutral functional terms only, not called out as security fixes in the title/description
