# CSM Notification Service

A Go background worker that turns CSM platform domain events into outbound
notifications: email, Google Chat posts, and voice calls. It consumes events from an
event bus (Azure Event Hub through its Kafka-compatible endpoint), renders and sends the
matching notifications, and runs an optional SLA breach-alerting engine and an optional
incident call-escalation ladder. Its only HTTP route is `GET /health`.

- [Architecture](#architecture)
- [Developer guide](#developer-guide)
- [Configuration](#configuration)
- [Security notes](#security-notes)

## Architecture

### Purpose and position

The entity service (and, for change-request notices, the flow service) publishes events (a case was created, a comment
was added, an incident was raised, a change request needs approval, a contact was
invited...). This service is a **pure consumer**: it does not own a database, does not
publish the events it reacts to, and has no inbound API besides the health probe. For how
it fits into the wider platform (portals, backends, entity service, event backbone), see
the [platform architecture](../../apps/csm-portal/README.md#architecture).

```text
 entity service, flow service (publishers)
        |  publish JSON envelopes {type, entityId, payload}
        v
 Event Hub topics (Kafka protocol)
   case topic | change-request topic | outage topic | onboarding topic | shared ops topic | DLQ topics
        |
        v   one consumer group per topic, N consumers per group
 internal/eventbus.Consumer  --(3 attempts, 2 s apart)-->  dispatch.Dispatcher.Handle
        |                                                      |
        | retries exhausted                                    +--> email service      (OAuth2 client credentials)
        v                                                      +--> Google Chat webhooks
 dead-letter topic --> DLQ consumer --> same handler           +--> voice-call provider (Twilio REST)
   (a second failure is logged and dropped)                    +--> entity service     (link/role lookup, onboarding ledger)
                                                               +--> identity-admin (SCIM) service
                                                               +--> frustration-detector service
                                                               +--> public status-page webhooks
                          Redis <-- SLA engine, call-escalation ladder
```

### Interactions

| Direction | Peer | Purpose |
|---|---|---|
| Inbound | Event Hub topics | Events to react to (see [Event types](#event-types)). |
| Inbound | Health probe | `GET /health` returns 200 with no body. |
| Outbound | Event Hub | Dead-letter topics; `sla.tier_reached` on the case topic. |
| Outbound | Email service | `POST` of rendered HTML mail; OAuth2 client credentials. |
| Outbound | Google Chat | Incoming-webhook posts, one webhook per audience (space). |
| Outbound | Twilio | Voice calls (and an SMS client that nothing calls today). |
| Outbound | Entity service | Per-recipient role lookup (`POST /users/search`), onboarding-step ledger, full comment/case content, SLA duration policy and clock state, paging contacts, execution-summary notes, cloud-status delivery reports. |
| Outbound | Identity-admin (SCIM) service | Create the invitee's user for `project_contact.invited`; fallback phone lookup for paging. |
| Outbound | Frustration-detector service | Analyse customer comments (optional). |
| Outbound | Status-page webhooks | Post outage declared/ended notices (optional). |
| Outbound | Redis | Clock and ladder state for the SLA engine and call-escalation ladder (optional). |

The service holds no durable state of its own. Deduplicating a publish and recovering a
publish that was never acknowledged are the publisher's job.

### Event types

`internal/events` defines the envelope and one payload struct per type;
`events.Validate` (called first by `Dispatcher.Handle`) is the service's only input
validation boundary. Types and their reaction, from `dispatch.Dispatcher.Handle`:

| Type | Reaction |
|---|---|
| `case.created`, `case.comment_added`, `case.status_changed`, `case.assigned`, `case.severity_changed` | One email per distinct portal link among the recipients (recipients are supplied in the payload). `case.created` and `case.severity_changed` also post a Chat alert to the fixed "Incident Monitor" audience; the two steps are independent. `case.comment_added` also feeds frustration detection and the SLA engine. |
| `case.acknowledged` | Chat alert only. |
| `case.workaround_provided` | Completes the workaround SLA clock (no message). |
| `case.escalated` | One email to the escalation's notification list. |
| `incident.created` | Voice call to the payload's `callTo`, or `INCIDENT_DEFAULT_CALL_TO`; incident Chat card where `INCIDENT_CHAT_SPACES` has an entry. |
| `incident.assigned` | "Incident Acknowledged." reply in the incident's Chat thread. |
| `incident.acknowledged`, `incident.priority_elevated`, `incident.comment_added` | Ignored here; owned by the call-escalation ladder. |
| `incident.special_ops_alert` | Pages the subject-matter expert through the SRE ladder engine, if enabled. |
| `change_request.approval_requested`, `change_request.plan_date_notice` | One email; the payload's audience (internal or customer) picks the portal link. |
| `outage.notification_due`, `outage.communication_due` | Outage email. |
| `outage.status_page_due` | One status-page webhook attempt, reported back to the entity service. |
| `project_contact.invited` | Onboarding: IDENTITY step (create user via SCIM) then EMAIL step (invitation), each recorded on the entity service's ledger as SUCCEEDED, FAILED or SKIPPED. A resend sends a short reminder and bypasses the duplicate-invitation check. |
| `project_contact.registered` | Welcome email after first sign-in, once per membership. |
| `sr.created`, `sr.acknowledged`, `sr.comment_added` | Chat alert to the owning SRE team's audience, threaded per service request. |
| `paging.test_call_requested` | Places a test call to a paging number (needs Redis and the entity service). |
| `sla.tier_reached` | Published by the SLA engine; ignored by the dispatcher. |

On shared topics (`Dispatcher.HandleShared`) an unknown type is skipped; on topics this
service owns, an unknown type is an error and ends in the DLQ.

Recipient links: the dispatcher asks the entity service for each recipient's roles and
builds a customer-portal or CSM-portal case link per recipient. Roles in `CUSTOMER_ROLES`
mean customer, then roles in `CSM_ROLES` mean CSM, otherwise the email domain decides (the
operator's own domain, hard-coded in `internal/recipientlinks`, gets the CSM link).

### Delivery semantics

- Each consumer retries a failing record 3 times with a 2 s delay, then hands it to the
  dead-letter publisher (or, on a DLQ consumer, logs and drops it), then commits.
- Each topic has its own DLQ; a new consumer group starts at the first offset.
- The call-escalation ladder has its own DLQ so a failed ladder record is never replayed
  through the dispatcher (which would place a duplicate call).
- Consumer counts should not exceed a topic's partition count; a startup check warns
  (extra consumers sit idle).

### Optional engines (need Redis)

- **SLA breach-alerting engine** (`internal/slaengine`). Active when `REDIS_URL` or
  `REDIS_ADDR` is set. Clocks are registered, paused and completed by three dispatcher
  handlers (`case.created`, `case.status_changed`, `case.comment_added`); a ticker scans a
  Redis wake-index, and on a new 50/75/100 percent crossing publishes `sla.tier_reached`
  and posts a Chat breach alert. Durations come from the entity service's
  duration-policy endpoint, fetched once at startup; a failure there disables SLA tracking
  for the run (logged) without stopping the service. State is rebuilt from the entity
  service by a one-shot reconciliation at startup.
- **Call-escalation ladder** (`internal/paging`). Active when Redis is set, escalation is
  enabled, and a resolver is available (a roster or the Team Schedule). Two ladders (CRE
  and SRE) climb a priority-driven plan of calls and/or Chat posts, each with its own
  consumer group and store namespace. Behaviour is governed by an optional YAML file
  (`INCIDENT_ESCALATION_CONFIG`; schema in `internal/paging/config.go`) plus the database
  settings described under `PAGING_CONFIG_SOURCE`. With no resolver the engine
  deliberately does not start, because a ladder that can call nobody looks like coverage.

The Redis client is a plain non-clustered client. Managed Redis must use a non-clustered
policy; a cluster-aware ("OSS cluster") policy is not supported.

### Middleware and runtime

`SecurityHeaders` (nosniff, upgrade-insecure-requests, HSTS), then `CorrelationID`
(reads or generates `X-CSM-Correlation-ID`, ensures a `cns-` prefix, echoes it, enriches
logs), then `Logger` (one access-log line per request). Server timeouts: header 10 s,
read/write 30 s, idle 60 s; graceful shutdown on `SIGINT`/`SIGTERM`. There is no
application-level auth because the only route is the health probe.

### Project structure

```text
csm-notification-service/
├── cmd/server/
│   ├── main.go                  # Wiring: config, clients, consumers, engines, health server
│   ├── sre_topic.go             # Shared operations-topic (sre-events) consumer planning
│   └── case_alert_settings.go   # PAGING_CONFIG_SOURCE: load ladder settings from the entity service
├── internal/
│   ├── apierror/                # Typed upstream error
│   ├── chataudience/            # Which Chat audiences an SLA breach resolves to
│   ├── dispatch/                # Dispatcher.Handle: validate, route by type, send
│   ├── entity/                  # Entity-service client (users search, onboarding ledger, content, cloud status)
│   ├── escalation/              # Frustration-detector client
│   ├── eventbus/                # kafka-go wrapper: Config, Producer, Consumer (retry, DLQ hook, commit)
│   ├── events/                  # Envelope, per-type payloads, Validate
│   ├── middleware/              # security_headers, correlation, logger
│   ├── notifications/           # Email, Google Chat, Twilio clients; HTML templates/
│   ├── oauthhttp/               # Shared OAuth2 client-credentials HTTP client
│   ├── paging/                  # Call-escalation ladder engine, resolvers, store, config
│   ├── recipientlinks/          # Per-recipient customer/CSM portal link
│   ├── scim/                    # Identity-admin (SCIM) client
│   ├── slaengine/               # SLA clocks in Redis + ticker
│   └── statuspage/              # Status-page webhook poster
├── .choreo/component.yaml       # Deployment descriptor (port 8080, Organization visibility)
├── openapi.yaml                 # Contract: GET /health only
└── .env.example
```

Channels are one client per file in `internal/notifications`, each with its own config
(upstream auth schemes differ). All clients are built once in `cmd/server/main.go`.

## Developer guide

### Prerequisites

- Go `1.26.6` or newer (see `go.mod`) - [install](https://go.dev/doc/install)
- An Event Hub namespace (or any Kafka endpoint reachable with TLS and SASL/PLAIN) with
  the topics provisioned. The service does not create topics. Local brokers without TLS
  are not supported: `internal/eventbus` always dials with TLS and SASL/PLAIN.
- Optional: Redis for the SLA engine and call-escalation ladder; `gosec` for scanning.

### Build and run

```bash
# from integrations/csm-notification-service
go build -o server ./cmd/server
cp .env.example .env        # fill in at least the required event bus variables
go run ./cmd/server
```

`.env` is read from the working directory if present; variables already set in the
process environment win. Missing required variables cause an immediate exit with an error
log. The health probe is `curl http://localhost:8080/health`. To exercise the whole
pipeline, publish a valid envelope to your case topic and watch the logs; use
`EMAIL_DEBUG_MODE=true` with `EMAIL_DEBUG_RECIPIENTS` so email goes to a safe test list,
and `EMAIL_SENDING_ENABLED=false` or `CALL_SENDING_ENABLED=false` to log instead of send.

### Test and lint

```bash
go vet ./...
go test -race ./...
```

Tests are self-contained, with two optional exceptions: `internal/paging/store_test.go`
runs against a real Redis (it tries `REDIS_URL`, then `REDIS_ADDR`, then
`localhost:6379`) and skips when none is reachable:

```bash
docker run --rm -p 6379:6379 redis
go test ./internal/paging/ -v
```

For security scanning: `go install github.com/securego/gosec/v2/cmd/gosec@latest`
then `gosec -fmt=text ./...`.

### Exercising the call-escalation ladder locally

The repository's local stack (see the platform README) includes helper scripts under
`scripts/csm-compose/`, for example `trigger-escalation.sh` and
`trigger-sre-escalation.sh`, that publish incidents against the local stack's rota.
Run them with `-h` for options. They default to the `log` channel, which prints who each
rung would reach and contacts nobody. To cover the Kafka hop and `cmd/server` itself,
point the service at a real namespace with `REDIS_ADDR` and an escalation roster or Team
Schedule resolver, then create and re-prioritise an incident through the entity service.
Unset `INCIDENT_DEFAULT_CALL_TO` while testing, otherwise each incident also gets the
dispatcher's single immediate call (the service warns at startup when both are live).

### Extending

- **A new event type**: add the constant, payload struct and `Validate` rule in
  `internal/events` (and `KnownTypes`), add a case to `Dispatcher.Handle`, and a handler
  in `internal/dispatch`. Return an error only for failures a retry could fix. Add tests
  next to the handler.
- **A new channel**: add a `<Name>Config`/`<Name>Client` in `internal/notifications`, build
  it in `main.go`, pass it to the dispatcher, and call it from the handler that should
  trigger it.
- **A new email**: add an HTML template under `internal/notifications/templates` and a
  render function in `templates.go`.
- **A new topic**: add a config, a consumer group and a dead-letter producer in
  `main.go`, and consider whether it should ride the shared operations topic instead.
- Keep logs free of recipient addresses, phone numbers and upstream response bodies.

## Configuration

Environment variables only, optionally loaded from `.env` (process environment wins).
Conventions: booleans accept `true`/`false` case-insensitively and fall back to the
default for anything else, except the three marked "exact", which compare to the
lowercase string `true`; counts must be positive integers or the default is used;
durations use Go syntax (`30s`, `5m`) and must be positive or the default is used.
"Optional" variables left unset disable the feature rather than failing startup unless a
note says otherwise. Examples are placeholders.

### Event bus (required)

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `EVENT_HUB_BROKER` | Yes | none | Kafka bootstrap address of the namespace. | `<namespace>.servicebus.windows.net:9093` |
| `EVENT_HUB_CONNECTION_STRING` | Yes | none | Shared-access connection string, used as the SASL/PLAIN password. Secret. | `<connection-string>` |
| `EVENT_HUB_TOPIC` | Yes | none | Case topic (case.*, incident.*, paging.*, sla.*). | `case-events` |
| `EVENT_HUB_DLQ_TOPIC` | Yes | none | Dead-letter topic for the case topic. Provision separately. | `case-events-dlq` |
| `EVENT_HUB_CONSUMER_GROUP` | No | `csm-notification-service` | Group for the case topic. | |
| `EVENT_HUB_DLQ_CONSUMER_GROUP` | No | `csm-notification-service-dlq` | Group for the case DLQ. | |
| `MAIN_CONSUMER_COUNT` | No | `1` | Consumer instances for the case topic. | `2` |
| `DLQ_CONSUMER_COUNT` | No | `1` | Consumer instances for the case DLQ. | `1` |

### Additional topics (all optional)

Each topic has a topic variable, a DLQ topic, consumer groups and counts. Topics must
exist before deployment and their names must match the publisher's configuration.

| Variables | Defaults |
|---|---|
| `CR_EVENT_HUB_TOPIC`, `CR_EVENT_HUB_DLQ_TOPIC` | `cr-events`, `cr-events-dlq` |
| `CR_CONSUMER_GROUP`, `CR_DLQ_CONSUMER_GROUP` | `csm-notification-service-cr`, `csm-notification-service-cr-dlq` |
| `CR_CONSUMER_COUNT`, `CR_DLQ_CONSUMER_COUNT` | `1`, `1` |
| `OUTAGE_EVENT_HUB_TOPIC`, `OUTAGE_EVENT_HUB_DLQ_TOPIC` | `outage-events`, `outage-events-dlq` |
| `OUTAGE_CONSUMER_GROUP`, `OUTAGE_DLQ_CONSUMER_GROUP` | `csm-notification-service-outage`, `csm-notification-service-outage-dlq` |
| `OUTAGE_CONSUMER_COUNT`, `OUTAGE_DLQ_CONSUMER_COUNT` | `1`, `1` |
| `PROJECT_EVENT_HUB_TOPIC`, `PROJECT_EVENT_HUB_DLQ_TOPIC` | `project-events`, `project-events-dlq` |
| `PROJECT_CONSUMER_GROUP`, `PROJECT_DLQ_CONSUMER_GROUP` | `csm-notification-service-project`, `csm-notification-service-project-dlq` |
| `PROJECT_CONSUMER_COUNT`, `PROJECT_DLQ_CONSUMER_COUNT` | `1`, `1` |
| `INCIDENT_ESCALATION_DLQ_TOPIC` | `escalation-events-dlq` (the ladder's own DLQ) |

The change-request, outage and project consumers are always started (unless replaced by
the shared operations topic below).

**Shared operations topic.** One topic can carry change-request notices, outage emails
and service-request alerts, routed by event type. Off unless `SRE_EVENT_HUB_TOPIC` is set.

| Variable | Default | Description |
|---|---|---|
| `SRE_EVENT_HUB_TOPIC` | unset (feature off) | Shared topic, e.g. `sre-events`. Must not equal the case or onboarding topic. When it equals the CR or outage topic, that consumer is not started (the shared consumer replaces it). |
| `SRE_EVENT_HUB_DLQ_TOPIC` | `<SRE topic>-dlq` | Its DLQ. Must exist and differ from the topic. |
| `SRE_CONSUMER_GROUP`, `SRE_DLQ_CONSUMER_GROUP` | takes over the CR/outage consumer's group when that consumer read the same topic (to keep its committed offset); otherwise `csm-notification-service-sre` and `csm-notification-service-sre-dlq` | An explicit value always wins. |
| `SRE_CONSUMER_COUNT`, `SRE_DLQ_CONSUMER_COUNT` | `1`, `1` | Instances. |

**Incidents on their own topic.**

| Variable | Default | Description |
|---|---|---|
| `INCIDENT_EVENT_HUB_TOPIC` | unset | Topic where the entity service publishes `incident.*`. Set it here before setting it on the publisher. Empty or equal to the case topic adds nothing. |
| `INCIDENT_CONSUMER_GROUP` | `<EVENT_HUB_CONSUMER_GROUP>-incidents` | Dispatcher consumer for that topic (skipped when the shared operations consumer already reads it). Dead-letters to the case DLQ. |
| `INCIDENT_CONSUMER_COUNT` | `1` | Instances. |
| `PAGING_SRE_INCIDENT_CONSUMER_GROUP` | `csm-notification-service-paging-sre-incidents` | Group the SRE ladder uses to read that topic. |

### Email

| Variable | Required | Default | Description | Example |
|---|---|---|---|---|
| `EMAIL_BASE_URL` | Yes, to send email | none | Base URL of the email service. | `https://email.example.com` |
| `EMAIL_SCOPES` | No | none | Comma-separated OAuth2 scopes. | `scope-a,scope-b` |
| `EMAIL_FROM_ADDRESS` | Yes unless `EMAIL_SENDING_ENABLED=false` | none | Fixed From address; startup exits if missing while sending is on. | `no-reply@example.com` |
| `EMAIL_REPLY_TO` | No | built-in support address if unset; none if set to empty | Reply-To for onboarding emails. Comma-separated. | `support@example.com` |
| `EMAIL_SENDING_ENABLED` | No | `true` | Killswitch for case.* email. `false` logs instead of sending, regardless of debug mode. Does not affect Chat or calls. | `false` |
| `EMAIL_DEBUG_MODE` | No | off (exact `true` to enable) | Redirect case.* email to `EMAIL_DEBUG_RECIPIENTS`. Links are still resolved from the real recipients. | `true` |
| `EMAIL_DEBUG_RECIPIENTS` | No | none | Comma-separated test addresses. With debug on and this empty, the email is skipped. | `qa@example.com` |
| `DEFAULT_CSM_EMAIL_CC` | No | none | Comma-separated CC added to the CSM-portal-link group of case.* email only, never to the customer group and never in debug mode. | `team@example.com` |
| `ACP_REACTIVATION_CC` | No | none | Comma-separated CC on the "Back to Open state" email (`project.reactivated`); never in debug mode. | `lifecycle@example.com,billing@example.com` |

### Google Chat

| Variable | Default | Description | Example |
|---|---|---|---|
| `GOOGLE_CHAT_SPACES` | unset (Chat alerts unavailable) | JSON array of `{"audience","webhookUrl"}`, one per space. Audience matching ignores whitespace but is case-sensitive. Meant now only for the standing, non-team audiences: `Incident Monitor`, `Onboarding`, `Americas`, `Evaluation` — a real team's own space should come from entity-service's `team.chat_webhook_url` instead (see `TEAM_CHAT_SPACES_REFRESH_INTERVAL` below); a team still listed here during migration still works, and wins over a same-named entity-service entry. A malformed value (or unknown JSON keys, such as the legacy product-keyed shape) logs an error and disables Chat. An audience with no entry is skipped. | `[{"audience":"Incident Monitor","webhookUrl":"<webhook-url>"}]` |
| `INCIDENT_CHAT_SPACES` | unset (no incident cards) | JSON object, assignment group (matched case-insensitively) to webhook URL. | `{"Example SRE Group":"<webhook-url>"}` |
| `INCIDENT_CHAT_FANOUT` | unset | JSON object, assignment group to the list of groups whose `INCIDENT_CHAT_SPACES` spaces get its cards. Names only. The call is placed once; a card that fails in one space is not retried. | `{"Default Group":["Group A","Group B"]}` |
| `TEAM_CHAT_SPACES_REFRESH_INTERVAL` | `5m` | How often a team's own Chat webhook URL (entity-service's `GET /teams/chat-webhooks`) is re-fetched and merged into the routing table. Only read when `CUSTOMER_ENTITY_BASE_URL` is set; a failed refresh logs and keeps the previous set. | `1m` |
| `GOOGLE_CHAT_AUDIENCE_SPACES` | n/a | Removed; if set, a warning is logged and it is ignored. Use `GOOGLE_CHAT_SPACES`. | |

### Voice calls and SMS (Twilio)

| Variable | Default | Description | Example |
|---|---|---|---|
| `TWILIO_ACCOUNT_SID` | none | Account SID. A missing config only errors when a call is requested. | `<account-sid>` |
| `TWILIO_AUTH_TOKEN` | none | Auth token. Secret. | `<auth-token>` |
| `TWILIO_FROM_NUMBER` | none | E.164 sending number. Always required for calls; SMS fallback sender. | `+15550100000` |
| `TWILIO_MESSAGING_SERVICE_SID` | none | Preferred SMS sender (SMS only; nothing sends SMS today). | `<messaging-service-sid>` |
| `TWILIO_VOICE` | provider default | TTS voice for calls. | `Polly.Raveena` |
| `TWILIO_LANGUAGE` | provider default | TTS language for calls. | `en-IN` |
| `TWILIO_API_BASE_URL` | `https://api.twilio.com/2010-04-01` | Override for a regional API edge. | |
| `TWILIO_RING_TIMEOUT_SECONDS` | `0` (provider default, 60 s) | How long a call rings before giving up. Applies when positive. | `30` |
| `CALL_SENDING_ENABLED` | `true` | Killswitch for calls (single incident call and the ladder). `false` logs instead of dialling. | `false` |
| `INCIDENT_DEFAULT_CALL_TO` | none | Fallback E.164 number for `incident.created` when the payload omits `callTo`. | `+15550100001` |

### Shared OAuth2 client (used by email, entity, SCIM, detector and paging clients)

| Variable | Required | Description | Example |
|---|---|---|---|
| `OAUTH2_TOKEN_URL` | Yes if any of those clients or Redis is used | Token endpoint. | `https://auth.example.com/oauth2/token` |
| `OAUTH2_CLIENT_ID` | Same | Client id. | `<client-id>` |
| `OAUTH2_CLIENT_SECRET` | Same | Client secret. Secret. | `<client-secret>` |

All three are read with plain lookups (a missing value fails at first use), except that
setting `REDIS_URL`/`REDIS_ADDR` makes them, with `CUSTOMER_ENTITY_BASE_URL`, mandatory at
startup.

### Entity service and recipient links

| Variable | Default | Description | Example |
|---|---|---|---|
| `CUSTOMER_ENTITY_BASE_URL` | none | Entity service base URL. Unset makes every case.* email fail (startup warning) and disables paging test calls, notes and the Team Schedule resolver. Mandatory when Redis is set. | `https://entity.example.com` |
| `CUSTOMER_ENTITY_SCOPES` | none | Comma-separated OAuth2 scopes. | `scope-a` |
| `CSM_PORTAL_WEB_BASE_URL` | none | Builds `<url>/cases/{caseId}` for CSM recipients, `/operations/outages/{id}` for outages and incident links. Unset yields relative links (warning). | `https://csm.example.com` |
| `CUSTOMER_PORTAL_WEB_BASE_URL` | none | Builds `<url>/projects/{projectId}/support/cases/{caseId}` for customer recipients. | `https://portal.example.com` |
| `CUSTOMER_ROLES` | none | Comma-separated role names classified customer. | `role-a,role-b` |
| `CSM_ROLES` | none | Comma-separated role names classified CSM. | `role-c` |

The entity service must allow this service's OAuth2 client id for the onboarding-ledger,
paging and note endpoints (it returns 403 otherwise).

### Customer onboarding

| Variable | Default | Description | Example |
|---|---|---|---|
| `CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED` | `true` (code default; see note) | Run the IDENTITY step (create the invitee's user via SCIM). `false` records it SKIPPED. | `false` |
| `CSM_MIGRATION_ONBOARD_EMAIL_ENABLED` | `true` (code default; see note) | Run the EMAIL step. `false` records it SKIPPED. Still subject to `EMAIL_SENDING_ENABLED` and debug mode. | `false` |
| `SCIM_BASE_URL` | none | Identity-admin service base URL (`POST /organizations/external/users`). Needed for the IDENTITY step; also enables the phone fallback for paging. | `https://scim.example.com` |
| `SCIM_SCOPES` | none | Comma-separated OAuth2 scopes. | `scope-a` |
| `ONBOARD_EMAIL_FROM` | `EMAIL_FROM_ADDRESS` | Sender of invitation email. | `welcome@example.com` |
| `ONBOARD_PORTAL_URL` | built-in public support-portal URL | Sign-in link in the invitation. | `https://portal.example.com` |

Note: `.env.example` and the comments in `cmd/server/main.go` describe these two flags as
opt-in (off unless exactly `true`), but the code reads them with a default of `true`
(`envBool(..., true)`). Set them to `false` explicitly to keep a deployment from running
the steps until the code or documentation is reconciled.

### Frustration detection

| Variable | Default | Description | Example |
|---|---|---|---|
| `ESCALATION_DETECTOR_BASE_URL` | unset (feature off) | Base URL of the frustration-detector service (`POST {url}/escalations`). A result above its threshold posts a Chat alert through `GOOGLE_CHAT_SPACES` (team first, falling back to `Incident Monitor`). | `https://detector.example.com` |
| `ESCALATION_DETECTOR_SCOPES` | none | Comma-separated OAuth2 scopes. | `scope-a` |

### Status-page webhooks

| Variable | Default | Description | Example |
|---|---|---|---|
| `CLOUD_STATUS_WEBHOOK_URLS` | unset (events reported undelivered for a scheduler to retry) | JSON object of cloud slug to base URL (`https` only; posts to `<url>/api/v1/webhook`). An unparseable value stops startup. | `{"cloud-a":"https://status.example.com"}` |
| `CLOUD_STATUS_WEBHOOK_SECRETS` | none | JSON object of secrets; the whole `X-Webhook-Signature` header value, with a `default` entry as fallback. Required to parse when URLs are set. | `{"default":"Secret <token>"}` |

### SLA engine and Redis

| Variable | Default | Description | Example |
|---|---|---|---|
| `REDIS_URL` | unset | `rediss://` (TLS) connection string; percent-encode the password. Takes priority over `REDIS_ADDR`. A malformed value stops startup. | `rediss://:<password>@<host>:6380` |
| `REDIS_ADDR` | unset | Plain non-TLS address. Setting either `REDIS_URL` or `REDIS_ADDR` enables the SLA engine and makes the ladder and paging test calls possible. | `localhost:6379` |
| `REDIS_PASSWORD` | none | Password for `REDIS_ADDR`; ignored with `REDIS_URL`. | |
| `SLA_TICK_INTERVAL` | `5m` | How often the wake-index is scanned. | `1m` |
| `SLA_EMAIL_SENDING_ENABLED` | `true` | Killswitch for all SLA messages (breach Chat card). Events still publish and tiers still advance. | `false` |

### Call-escalation ladder

Requires Redis. The ladder starts only if escalation is enabled, a resolver can answer
(roster, or Team Schedule with the entity service configured) and the configuration parses.

| Variable | Default | Description | Example |
|---|---|---|---|
| `INCIDENT_ESCALATION_CONFIG` | unset | Path to the escalation YAML (schema: `internal/paging/config.go`). Unset keeps the env-derived behaviour: CRE ladder enabled, SRE ladder enabled only with the Team Schedule resolver. A bad file disables both ladders. | `/etc/csm/escalation.yaml` |
| `INCIDENT_ESCALATION_ENABLED` | unset | If non-empty, overrides the file's master switch (`true` on, anything else off). | `false` |
| `INCIDENT_ESCALATION_CHANNEL` | `call` | `call`, `chat`, `both` or `log` (log only prints who would be reached). The file's per-ladder channel wins when present. An invalid value disables the ladder. | `log` |
| `INCIDENT_ESCALATION_RESOLVER` | roster | `team-schedule` resolves rungs from the entity service's Team Schedule (needs `CUSTOMER_ENTITY_BASE_URL`); anything else uses the roster. | `team-schedule` |
| `INCIDENT_ESCALATION_ROSTER` | none | JSON roster of level to recipients, used by the roster resolver. Contains phone numbers (secret). Invalid JSON stops the ladder. | `{"default":{"LEVEL_1":[<recipient>]}}` |
| `INCIDENT_ESCALATION_PHONES` | none | JSON object, lower-case email to E.164 number, for the Team Schedule resolver. A fallback: a valid CSM profile number is used first. | `{"jane.doe@example.com":"+15550100002"}` |
| `INCIDENT_ESCALATION_TEST_CALL_TO` | none | If set, every call goes to this number instead (safe rehearsal); nothing replaces it, not even a CSM profile number. | `+15550100003` |
| `INCIDENT_ESCALATION_SRE_L4` | off (exact `true`) | Include level 4 in the SRE ladder when no file is used. | `true` |
| `INCIDENT_ESCALATION_SSML` | off (exact `true`) | Use SSML for call speech. | `true` |
| `INCIDENT_ESCALATION_CHAT_AUDIENCE` | none | Chat audience a rung posts to when the incident names no product. | `Incident Monitor` |
| `INCIDENT_ESCALATION_NOTE_ACTOR` | built-in system actor | Identity under which a customer case's execution summary is written; must be allowed by the entity service. | `bot@example.com` |
| `INCIDENT_ESCALATION_CONSUMER_GROUP` | `csm-notification-service-escalation` | CRE ladder group. | |
| `INCIDENT_ESCALATION_SRE_CONSUMER_GROUP` | `csm-notification-service-escalation-sre` | SRE ladder group. | |
| `INCIDENT_ESCALATION_DLQ_CONSUMER_GROUP` | `<CRE group>-dlq` | CRE ladder DLQ group (the SRE one is `<SRE group>-dlq`). | |
| `INCIDENT_ESCALATION_CONSUMER_COUNT`, `INCIDENT_ESCALATION_DLQ_CONSUMER_COUNT` | `1`, `1` | Instances per ladder. | |
| `INCIDENT_ESCALATION_TICK_INTERVAL` | `5s` | Ladder scheduler tick. | `5s` |
| `PAGING_CONFIG_SOURCE` | `db` | `db` uses ladder settings saved in the entity service (Team Schedule alert settings) over the file's; `off` uses file and env only; `shadow` reads them, logs differences and keeps the file's. Unknown values act as `off`. Read once at start. Needs `CUSTOMER_ENTITY_BASE_URL`. | `off` |
| `PAGING_CONFIG_WAIT` | `2m` | How long a `db` start retries the entity service (`0` tries once). On failure it uses the last settings stored in Redis, else the file's. | `30s` |
| `PAGING_PHONE_SCIM_FALLBACK` | on | `false` stops asking the identity-admin service for numbers the entity service lacks. | `false` |
| `ALERTING_CALL_OUTCOME_CHECK` | on | Log how each placed paging call ended (`escalation: call outcome`, `result=completed\|no-answer\|busy\|failed\|unresolved`), read from Twilio after the ring timeout. Logging only; `false` turns it off. | `false` |

### Server

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Listen port; a bare number between 1 and 65535, not an address. Anything else exits. |

## Security notes

- Never commit `.env`, client secrets, webhook URLs, rosters or phone numbers.
- Logs carry IDs and error summaries only: no recipient emails, phone numbers or upstream
  response bodies. Keep it that way when changing handlers.
- `events.Validate` is the only input boundary; keep rejecting unexpected input there
  before it reaches a notification client.
- Describe security-related fixes in PRs in neutral functional terms.
