# Entity Service

Go HTTP server (`net/http`, standard library only) that owns all core CS-platform entities: users, accounts, projects, products, deployments, deployed products, cases, and case comments. It exposes a REST API consumed by portal BFFs and other internal services.

## Architecture

> **`cmd/` holds exactly one directory: `cmd/api`.** Never add a second
> `package main` there — Choreo's build picks this service's main package from
> `cmd/`, and a second directory fails the pipeline. It also drags dev-only
> code into everything `go build ./...`, `go vet` and gosec walk. Put one-off
> verification tooling in an env-var-guarded `TestServe*Harness` in
> `internal/server/`, or in `scripts/`, or keep it out of the repo.

Strict four-layer stack — no shortcuts across layers:

```
Handler → Service → Repository → PostgreSQL (pgx/v5)
```

All wiring happens explicitly in `internal/server/routes.go` (no DI framework). The full dependency graph is built there: `NewRepository(db) → NewService(repo) → NewHandler(svc)`, then registered on a `net/http.ServeMux`.

Middleware chain wraps the mux: **CorrelationID → Recovery → Logger → UserIDToken → auth.Middleware → Timeout** (10 s per request; `auth.Middleware` was missing from this list before — see "Token validation and caller-scoped access" below for what it does).

`CorrelationID` reads the `X-CSM-Correlation-ID` request header forwarded by the portal BFF, or generates a UUID v4 if absent. The ID is stored in the request context and echoed in the response header. All access log lines and panic logs include the correlation ID for end-to-end request tracing.

`Logger`'s access log line also carries `callerId` — the same Asgardeo user UUID `apps/csm-portal/backend`/`apps/customer-portal/backend-v2` already log for the request that reached them (their `UserInfo.UserID`, the `userid` claim), decoded here from the `x-user-id-token` those BFFs forward — so one request can be traced across services by that one value, not just the correlation ID. Falls back to the client id from a pure machine-to-machine caller's own `x-jwt-assertion` token when there's no end user in the loop, or `-` when neither validated (no tokens presented, or a token that failed validation — its claims are never trusted or logged). This needs its own plumbing (`auth.IdentityHolder`, a mutable pointer `Logger` installs into the request context before `auth.Middleware` runs) rather than the simpler `auth.WithIdentity`/`IdentityFromContext` pair every handler/service already uses to read the caller's identity: `auth.Middleware` returns early with a 401 without ever calling `next.ServeHTTP` on an invalid token, so a value it only ever handed *forward* down the chain (the normal way `context.WithValue` works) would never reach `Logger`, which wraps it — and a rejected request must still show up in this access log. See `auth.IdentityHolder`'s own doc comment for the full reasoning.

## Running locally

```bash
cp .env.example .env   # fill in DB_* vars
go run ./cmd/api/main.go
```

The server loads `.env` automatically on startup (silently ignored if absent). Port defaults to `8080`; override with `SERVER_PORT`.

## Environment variables

| Variable      | Required | Default | Purpose                   |
|---------------|----------|---------|---------------------------|
| `DB_HOST`     | no       | `localhost` | PostgreSQL hostname    |
| `DB_PORT`     | no       | `5432`  | PostgreSQL port            |
| `DB_USER`     | yes*     | —       | Database user              |
| `DB_PASSWORD` | yes*     | —       | Database password          |
| `DB_NAME`     | yes*     | —       | Database name              |
| `DB_SSLMODE`  | no       | —       | `disable` or `require`    |
| `DB_SCHEMA`   | no       | `DB_USER,public` | Pins the connection's `search_path` (`DSN`'s `options=-c search_path=...`), same purpose as `operations/csm-sync-service`'s own `DB_SCHEMA` — see that config's `withSchema`. The fallback makes explicit what Postgres' own default `search_path` (`"$user", public`) would already do implicitly — `public` must survive it, since every deployment's tables live there today (unqualified migrations). An explicit value is used verbatim, with no `public` appended |
| `DB_POOL_MAX_CONNS` | no | `20` | pgxpool max open connections (see "Connection pool settings" below) |
| `DB_POOL_MIN_CONNS` | no | `2` | pgxpool connections kept warm when idle; `0` is a valid, accepted value |
| `DB_POOL_MAX_CONN_LIFETIME` | no | `30m` | pgxpool connection rotation interval |
| `DB_POOL_MAX_CONN_IDLE_TIME` | no | `5m` | pgxpool idle-connection release interval |
| `SERVER_PORT` | no       | `8080`  | Main API listen port       |
| `HEALTH_PORT` | no       | `8081`  | Health probe listen port; `Validate` rejects it being equal to `SERVER_PORT` (see "Health probes" below) |
| `SERVER_READ_TIMEOUT` | no | `60s` | Main API server read timeout (Go duration, e.g. `60s`); must be > 0 |
| `SERVER_WRITE_TIMEOUT` | no | `60s` | Main API server write timeout; must be > 0 |
| `REQUEST_TIMEOUT` | no | `60s` | Per-request context timeout; must be > 0 |
| `UPSTREAM_CLIENT_TIMEOUT` | no | `60s` | Data-source HTTP client timeout; must be > 0 |
| `EVENT_HUB_BROKER` | no | — | Kafka-compatible bootstrap address; feature-gates `EventPublisherService` (see "Event Hub publishing" below) |
| `EVENT_HUB_CONNECTION_STRING` | no* | — | Event Hub namespace Shared Access Policy connection string. *Required once `EVENT_HUB_BROKER` is set |
| `EVENT_HUB_TOPIC` | no* | — | Event Hub (Kafka topic) name. *Required once `EVENT_HUB_BROKER` is set |
| `EVENT_PUBLISHING_ENABLED` | no | `false` | Must be `"true"` for `EventPublisherService` to actually get constructed, even with `EVENT_HUB_BROKER` fully configured — a separate safe-by-default kill switch |
| `CUSTOMER_ROLES` | no | — | Comma-separated ServiceNow role names whose presence on a case comment's resolved author marks a customer reply — see "Customer-reply state transition" below |
| `SALES_ENTITY_BASE_URL` | no* | — | REST `sales/sales-entity-service` base URL (not GraphQL `sales/entity-graphql-service`). *Required once any `SALES_ENTITY_*` var is set |
| `SALES_ENTITY_TOKEN_URL` | no* | — | OAuth2 token endpoint (client_credentials grant) |
| `SALES_ENTITY_CLIENT_ID` | no* | — | Choreo connection client id |
| `SALES_ENTITY_CLIENT_SECRET` | no* | — | Choreo connection client secret |
| `SALES_ENTITY_SCOPES` | no | — | Optional space-separated OAuth2 scopes for REST `sales/sales-entity-service` |
| `CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED` | no | `false` | Must be `"true"` for `POST /users/me/memberships/register` to be registered at all (see "Membership registration" below). Off = the route 404s and nothing on that path can write to Salesforce |
| `CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED` | no | `false` | Must be `"true"` for `POST /salesforce/events` to act on `Project_Contact__c`/`Contact` envelopes, the Contact writer included (see "Salesforce membership ingest" and "The Contact writer" below). The Account branch is unaffected |
| `CSM_MIGRATION_SALESFORCE_ACCOUNT_INGEST_ENABLED` | no | `false` | Must be `"true"` for `POST /salesforce/events` to act on `Account` envelopes; off, they are acknowledged and ignored. Keep it off while the ServiceNow sync still writes `account`. The upsert resolves the row by `sf_id` (not unique since migration 0095), then links a same-`number` row with no `sf_id`, then inserts; the SE-1 columns are kept when Salesforce sends none; see "Salesforce Account ingest" for the full column set |
| `CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED` | no | `false` | Must be `"true"` for `POST /salesforce/events` to act on `Opportunity` envelopes (`sf_opportunity` plus its `sf_opportunity_product` line items, see "Salesforce Opportunity ingest" below) and `Linked_Opportunity__c`, `Invoice__c` and standalone `OpportunityLineItem` envelopes (`sf_opportunity_link`, `sf_invoice`, `sf_opportunity_product`); off, they are acknowledged and ignored. Keep it off while csm-sync-service still copies these tables from ServiceNow: the two writers use different row ids and `sf_id` is not unique, so both on means duplicate rows |
| `CSM_MIGRATION_SALESFORCE_PROJECT_INGEST_ENABLED` | no | `false` | Must be `"true"` for `POST /salesforce/events` to act on `Project__c` envelopes (see "Salesforce Project ingest" below); off, they are acknowledged and ignored. Update-only unless the insert switch is on too |
| `CSM_MIGRATION_SALESFORCE_PROJECT_INSERT_ENABLED` | no | `false` | With the project flag on, lets the Project ingest and `EnsureProject` insert projects CSM does not have. Keep it off while csm-sync-service still inserts `project` rows (different id schemes and a UNIQUE `key`: its insert would fail forever); turn it on at cutover |
| `CSM_MIGRATION_SALESFORCE_PARTNER_INGEST_ENABLED` | no | `false` | Must be `"true"` for the partner-link refresh (`account_relationship` "Is Partner Of" / "Is Customer Of") to run after Account events and partner-contact membership events, and for `POST /salesforce/accounts/{sfId}/refresh-partners` to be registered (see "Salesforce partner relationships" below). Keep it off while csm-sync-service still copies `account_relationship` from ServiceNow |
| `SALESFORCE_INGEST_RETRY_INTERVAL` | no | `5m` | How often the Salesforce ingest retry worker re-runs memberships whose DATABASE step FAILED because their project or account was not in CSM yet, and how old such a failure must be before it is re-run (see "Salesforce ingest ledger and the delayed-retry job" below). `0` disables the job, and so does an unparseable or negative value (logged as a warning; it fails closed rather than falling back to `5m`). Only runs when `CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED=true` |
| `CSM_MIGRATION_PORTAL_WRITES_ENABLED` | no | `false` | Must be `"true"` to register the four portal-driven membership write routes under `/projects/{id}/contacts` (see "Portal-driven membership writes" below). Also needs `DATA_SOURCE=postgres`, a pool, and the full `SALES_ENTITY_*` set (`Config.HasPortalMembershipWrites`). Off means the routes are **not registered at all**, not 403 |
| `CSM_MIGRATION_CUSTOMER_ENGAGEMENT_INGEST_ENABLED` | no | `false` | Registers `POST /customer-engagements/allocation-events` (Postgres-authoritative only); see "Allocation events" below |
| `CUSTOMER_ENGAGEMENT_FIREFIGHTING_TYPE_ID` | no | — | ServiceNow sys_id written as `engagement_type_id` on firefighting engagements created by allocation events. Unset skips creating them |
| `REDIS_URL` | no | — | `rediss://:<key>@<host>:<port>` (TLS, Azure Managed Redis); wins over `REDIS_ADDR`. Turns on the user cache (see "User cache (Redis)" below). `Validate` requires a `redis`/`rediss` scheme and a host, and never echoes the URL |
| `REDIS_ADDR` / `REDIS_PASSWORD` | no | — | Plain, non-TLS Redis for local runs. Either this or `REDIS_URL` makes `Config.HasRedis` true |
| `USER_CACHE_TTL` | no | `10m` | Backstop lifetime of a cached user; an unparseable or non-positive value falls back to `10m` |
| `CR_STRICT_VISIBILITY_FROM` | no | — | RFC 3339 instant **with a zone**: change requests created at or after it are visible to a customer only when designated to them; earlier ones are "legacy" and keep today's visibility. Unset/empty = no cutover, every change request is legacy (the safe default and the rollback; a WARN is logged at startup). An unparsable value refuses to start. Set once per environment at release and never move it — see "Customer visibility and the cutover" |

\* `DB_USER`/`DB_PASSWORD`/`DB_NAME` are required when `DATA_SOURCE=postgres`
and **optional** when `DATA_SOURCE=servicenow`, where entity reads and writes
go to the SN integration service instead. They are all-or-nothing in both
modes — `Config.Validate` rejects a partial set, so a typo can't silently
disable the Postgres-only endpoints. With `DATA_SOURCE=servicenow` and no
database, `Config.HasDatabase` is false, `cmd/api/main.go` opens no pool, and
`NewRouter` skips registering the two Postgres-only feature sets
(`/event-publish-failures*`, `/sla-status`), which then 404.
A failed Event Hub publish is logged instead of recorded — see
`EventPublisherService.Publish`'s nil-`failures` branch.

`CSM_TEAM_REGISTRY` and `CSM_USER_ROLES` are **not read here**. The team registry
and the assignable-role allow-list are organisation vocabulary and live in the CSM
portal backend (`apps/csm-portal/backend`), resolved once at startup. This service
holds no organisation vocabulary at all — do not reintroduce it.

## Health probes

The process runs **two** HTTP listeners, and the split is a security boundary, not a
convenience:

- `SERVER_PORT` (8080) — the full API (`internal/server/routes.go`), published at
  **Organization** visibility.
- `HEALTH_PORT` (8081) — `internal/server/health.go`, a minimal mux carrying only the two
  probes below, published at **Public** visibility so external alerting can poll it with no
  credentials.

Both are declared as separate Choreo endpoints in `.choreo/component.yaml`, the public one
against its own `health-openapi.yaml`.

**`.choreo/component.yaml` hardcodes both ports and nothing reconciles them with the env vars at
deploy time.** Overriding `HEALTH_PORT` in a Choreo deployment routes public health traffic to a
port with no listener, and the symptom — a health endpoint that never answers — is
indistinguishable from the outage it exists to report. Leave `HEALTH_PORT` unset there; override
it locally only, and if the port ever has to change, change `component.yaml` in the same commit.
`SERVER_PORT` has carried this same coupling since before the health endpoint existed.

What is publicly reachable is decided by *which mux a handler is registered on* — true in this
process, visible in one file — rather than by a gateway basePath rule that lives in another
system and fails open if it is ever wrong. **Never register a business route on the health mux,
and never point the public Choreo endpoint at 8080.** That is the whole reason this is a second
listener rather than a second basePath.

| Probe | Where | Behaviour |
|---|---|---|
| `GET /health` | both listeners | Always `200 {"status":"ok"}`. Dependency-free by design: a liveness probe that fails on a database outage would have the orchestrator restart or drain an instance that is working fine. |
| `GET /health/database` | health listener only | `200 {"status":"ok","database":"up"}` after a successful `Ping`, `503 {"status":"unavailable","database":"down"}` when it fails. |

Conventions to preserve when touching these:

- **Only a deployment that has a pool can fail `DatabaseCheck`.** This probe alerts on a
  *PostgreSQL* outage; a no-pool deployment (`DATA_SOURCE=servicenow`) has no PostgreSQL to be
  out, so it answers `200` with `database: "not_configured"`. A 503 there would alert
  continuously against a database that is not supposed to exist. The distinct `database` value
  is what keeps the case visible to anyone reading the body.
- **Failure bodies carry no detail.** No driver message, host, or port — pgx errors routinely
  embed all three, and this endpoint is unauthenticated and public. Report only whether the
  dependency is up. There is a test asserting this specifically.
- **Both probes send `Cache-Control: no-store`.** A cached 200 keeps reporting healthy straight
  through the outage the probe exists to catch.
- **Pass an untyped nil, not a nil `*pgxpool.Pool`,** to `handler.NewHealthHandler`. A nil
  pointer stored in an interface makes the interface non-nil, so the handler's own `db != nil`
  guard would pass and `Ping` would be called on a nil pool. `server.NewHealthServer` does this
  conversion explicitly; keep it that way.
- **No `Logger` middleware on the health listener** — alerting polls continuously and would
  otherwise fill the logs. `Recovery` stays, since a panic there would take down the main API
  with it.

## Cases and incidents are different entities

A **case** is `POST /cases`, `domain.CaseView`, the `case.*` events. An
**incident** is `POST /incidents`, `domain.IncidentView`, the `incident.*`
events. Separate endpoints, separate domain types, separate handlers and
separate service files (`sn_case_service.go` against `sn_incident_service.go`).
A "comment added" on one is not a "comment added" on the other, which is why
both `case.comment_added` and `incident.comment_added` exist and carry
different payloads. Don't collapse the vocabulary: the two event families
cannot be merged without two different payloads sharing one name.

**"SRE incident" is not a third thing.** `integrations/sre-alert-ingestion-service`
turns a vendor alert (Azure, Grafana, Site24x7, OpenSearch) into a platform
incident by calling the same `POST /incidents` through csm-integration-service,
so an alert-born incident is exactly the entity the `incident.*` events
describe. csm-notification-service's call-escalation ladder escalates it like
any other.

## Event Hub publishing

`internal/eventbus` (a minimal Kafka producer for Azure Event Hub's
Kafka-compatible endpoint, `EVENT_HUB_BROKER`/`EVENT_HUB_CONNECTION_STRING`/
`EVENT_HUB_TOPIC`) and `internal/events` (`Envelope{Type, EntityID, Payload}`,
the wire shape `csm-notification-service` consumes) are ported from
`apps/csm-portal/backend`'s own copies of the same packages — that backend's
`internal/eventbus`/`internal/events` predate these and remain in place; the
two are kept in sync by hand, same as `csm-notification-service`'s own copy.

`service.EventPublisherService` (`internal/service/event_publisher_service.go`)
wraps a `kafkaProducer` (satisfied by `*eventbus.Producer`) and publishes a
domain event via `Publish(ctx, eventType, entityID, payload)`, keyed by
`entityID` so every event about the same case/incident stays ordered on the
same partition. If Event Hub doesn't acknowledge the publish, it durably
records the failure via `EventPublishFailureService.CreateEventPublishFailure`
— called directly, in-process, unlike `apps/csm-portal/backend`'s own
`eventpublisher.Publisher`, which has to reach this same table over HTTP
(`POST /event-publish-failures`) since it lives in a different service.

**Wired in**: `NewEventPublisherService` is constructed in
`internal/server/routes.go` (not `cmd/api/main.go` — `NewRouter` owns the
whole dependency graph; see "Adding a new entity" below), gated on
`cfg.EventHubBroker != "" && cfg.EventPublishingEnabled` — the same
optional-wiring convention `apps/csm-portal/backend/cmd/server/main.go`
used to use for its own now-removed Event Hub pipeline, but keyed on Event
Hub config specifically, not `cfg.DataSource`: publishing is a distinct
concern from which backend serves reads. `EventPublishingEnabled` is a
second, independent kill switch on top of `EventHubBroker` — it defaults to
`false` (`EVENT_PUBLISHING_ENABLED` must be exactly `"true"`), so an
environment can have Event Hub fully configured and still publish nothing
until this is explicitly turned on; every publisher call site already
handles `eventPublisher == nil` as a no-op, so this required no changes
anywhere except `config.go`/`routes.go` themselves. `Config.Validate`
rejects a partial Event Hub configuration (e.g. `EVENT_HUB_BROKER` set but
`EVENT_HUB_CONNECTION_STRING`/`EVENT_HUB_TOPIC` empty) at startup — all
three must be set together or not at all, since `NewRouter`'s gate only
checks `EventHubBroker`, and constructing `EventPublisherService` with a
missing connection string or topic would make every publish attempt fail
silently while the deployment otherwise looks healthy;
`EventPublishingEnabled` isn't part of that all-or-nothing group — it's
just a bool, either `"true"` or not. `NewRouter` returns the constructed
`EventPublisherService` (nil if unconfigured) alongside the `http.Handler`,
threaded through `server.New` to `cmd/api/main.go`, which calls `Close()` on
it during shutdown, after `srv.Shutdown`.

## Service request events (`sr.*`)

`SRNoticeService` (`internal/service/sr_notice_service.go`) ports ServiceNow's
"SR New Request - Acknowledge & Chat Alert" flow (discovery scripts 73/74) and
publishes three events to the operations topic (`SRE_EVENT_HUB_TOPIC`,
sre-events), not the case topic: SRs belong to SRE. Types and payloads are in
`internal/events/service_request.go`; csm-notification-service turns them into
Chat cards in the SR's SRE-team space.

- `sr.created`: every SR created on the plain-Postgres path
  (`caseService.CreateCase`). Under dual-write the SR is created in ServiceNow
  first and its own flow still runs there, so nothing happens here.
- `sr.acknowledged`: when the SR's account SRE team is in
  `SR_ALERT_SRE_TEAM_IDS`, the SR is first assigned to that team, then gets
  ServiceNow's acknowledgement comment (word for word) and is moved to OPEN --
  a no-op for a native SR, which is created OPEN. The comment is written
  straight to the table, so no `case.comment_added` fires: the flow saves with
  `setWorkflow(false)`. ServiceNow acknowledges only when its card was sent;
  here the gate is the team being listed (product decision, 2026-10-07).
- `sr.comment_added`: every comment or work note on an SR, from
  `createCaseCommentAs` (plain and dual-write), independent of
  `case.comment_added`'s recipient gate. Carries the author and the SR's tags;
  the consumer decides the devops-sm customer-comment alert.

Wired in `routes.go` only when there is an SRE topic and a publisher: the
acknowledgement must never be posted with no card announcing the SR.

## User cache (Redis)

`GET /users/{id}` and `GET /users/me` are served cache-aside from Redis when
`Config.HasRedis()` and there is a pool. `NewRouter` wraps `userSvc` in
`service.NewCachedUserService(inner, cache)` (`internal/service/cached_user_service.go`),
a decorator over `UserService` that overrides `GetUser`, `GetMe`, `PatchMe`
and `CreateUser` and passes everything else straight through. The Redis side
lives in `internal/cache` (`NewRedisClient`, `UserCache`); the service layer
depends only on the `service.UserCache`/`service.UserCacheInvalidator`
interfaces in `interfaces.go`. `rdb.Close()` runs from `closePublishers` at
shutdown.

Keys (all under `entity:v1:user:`; bump `v1` when a cached shape changes):

| Key | Value |
|---|---|
| `detail:{id}` | `domain.UserDetail` for `GET /users/{id}` |
| `me:{id}` | `domain.GetUserMeResponse` for `GET /users/me` |
| `id-by-email:{sha256(lower(email))}` | user id, so `GetMe` (keyed by the caller's email) and email-only invalidations can find the id |

Conventions to preserve:

- **Invalidate after commit, by deleting.** Every writer of user, contact or
  membership rows calls `InvalidateUser(ctx, userID, email)` once its write has
  succeeded: `PatchMe`/`CreateUser` in the decorator, `writeContact`/
  `deactivateContact` (Contact writer), `ingestMembership` and the DELETED
  branch (membership ingest), and `Invite`/`UpdateRoles`/`Deactivate`
  (`project_membership_write_service.go`, via `MembershipWriteDeps.UserCache`).
  The `DeactivateBySfID` repos return the affected `[]domain.AffectedUser` for
  this. A new writer of `user`, `account_contact` or `project_contact` must do
  the same, or its change is invisible for up to `USER_CACHE_TTL`. Never write
  the new value into the cache from a writer; the next read repopulates it.
- **Fail open.** Every Redis call has a short timeout and a failure is a miss,
  never an error to the caller. Warnings are rate-limited (`warn`, once per
  30s); a failed invalidation is logged at ERROR with the user id only.
- **Never cache errors or not-found.** Only a successful inner result is
  stored. `GetUser` validates the id before touching the cache.
- **No PII in keys or logs.** Emails are hashed in keys and never logged;
  `REDIS_URL` holds the access key, so `Validate` and `NewRedisClient` return
  generic errors that never quote it.
- **Keep the invalidator a nil interface when the cache is off.**
  `userCacheInvalidator` in `routes.go` is declared as
  `service.UserCacheInvalidator` and assigned only when the cache is built;
  assigning a nil `*cache.UserCache` would make it non-nil (the same pitfall as
  the health handler's pool). `invalidateUser` treats a nil invalidator as a
  no-op.
- **`GetMe` checks the cached email.** A `me:{id}` entry whose `Email` does not
  match the caller is treated as a miss, which guards against a stale
  `id-by-email` entry after an email change.

Known limits: a read that races an invalidation can re-cache the old value
until the TTL; a degraded `GetMe` (e.g. groups unavailable) is cached like any
other success; `SearchUsers`, `GetUsersByIDs` and `DATA_SOURCE=servicenow` are
not cached. The client is a plain `redis.NewClient`, so the target must not use
the "OSS Cluster" clustering policy.

## Salesforce Account ingest

`POST /salesforce/events` accepts the ASB envelope `{eventType, entity, referenceId}`
from `sales-apex-trigger-subscriber`, which dual-forwards every envelope to
ServiceNow and to this endpoint. The subscriber has no `dataSource` switch.
Wired in `internal/server/routes.go` only when entity-service
`DATA_SOURCE=postgres`, a pool is available, and all four `SALES_ENTITY_*`
vars are set — the same optional all-or-nothing style as Event Hub.
`Config.Validate` rejects a partial REST sales-entity-service set at startup.

`internal/salesentity` uses stdlib `net/http` and an OAuth2 `client_credentials`
grant, then REST `POST /customer-search` with `{ids, isRealTime: true, limit: 1}`
against Choreo id `sales/sales-entity-service` (not GraphQL
`sales/entity-graphql-service`). Token is refreshed on 401. An empty
search result on CREATED/UPDATED/RESTORED is a 503 so the caller can retry
(the event can arrive before Salesforce commits). This service does not call
Salesforce REST; the REST sales entity-service does.
Non-Account entities return 204 and are ignored (do not 400 — ASB would
retry forever).

**What the Account branch writes** (`upsertAccount` + `mapSalesEntityCustomer` in
`salesforce_event_service.go`, `UpsertFromSalesforce` in `account_repo.go`). The
UPDATE lists only Salesforce-owned columns; `account_repo_salesforce_test.go` pins
the list.

- Plain assignment (a value cleared in Salesforce clears here): `name`, `sf_id`,
  `industry`, `region`, `global_pod`, `phone` (a value over 64 characters keeps the
  stored phone), `sales_region`, `sub_region`, `life_cycle` (`status`),
  `naics_industry`, `sub_industry`, `classification`, `technical_owner_id`, the
  billing address (`street`, `city`, `state_province`, `postal_code`, `country`),
  `account_manager_id` (`owner.email`), `activation_date`, `lost_date`, `lost_reason`.
- `COALESCE(new, stored)`: `customer_success_manager_id` (`csmEmail`),
  `secondary_technical_owner_id`, `renewal_account_manager_id`
  (`renewalManager.email`), `account_vertical`, `lost_reason_category`,
  `deactivation_date`. Sales Entity sends these only after its "customer fields"
  change (SE-1 in `docs/customer-onboarding/SALESFORCE_SYNC_PLAN.md`); until then they
  keep the ServiceNow sync's value. Switch them to plain assignment once SE-1 ships.
- Never written: `number` on an existing row (a new row gets the Salesforce Id,
  decision D9), `cre_team_id`, `sre_team_id`, `support_tier`, `support_timezone`,
  `suspension_process_state`, the two AI flags, `drive_location`.
- Person references resolve by email against `"user"` (`LookupUserIDByEmail`); an
  email with no user writes NULL and logs a WARN with the role and email.
- A value longer than its VARCHAR column (`accountColumnLimits`) or a date that does
  not parse (`2006-01-02`) is written as NULL with a WARN, so one bad field cannot
  fail the event.
- A customer with no name is acknowledged (204, no retry, no dead letter), logged,
  and recorded FAILED in `salesforce_ingest_state`.

**Duplicate guard and ledger.** CREATED/UPDATED/RESTORED run `shouldSkipIngest`
with entity `account` and the customer's `lastModifiedDate`, then write the account
row and a SUCCEEDED ledger row in one transaction; a failed write records FAILED
(best effort) and returns the error so Service Bus redelivers. `EnsureAccount`
writes without the guard (the row is known to be missing).

**DELETED** sets `account.deleted_on` (migration 0171, decision D7) and writes a
DELETED ledger row in one transaction, stamped with the current time or the recorded
version when that is later (Salesforce's clock can run ahead of ours), so the row
really becomes DELETED and the RESTORED that follows is not skipped. No Sales Entity
read. `deactivation_date` is not touched: it is Salesforce's contract end date. Any
later CREATED/UPDATED/RESTORED clears `deleted_on`. Never `DELETE FROM account`
(project → account is `ON DELETE CASCADE`). `SearchAccounts` excludes
`deleted_on IS NOT NULL`; `GetAccountByID`, `LookupAccountIDBySfID` (so
`EnsureAccount`), the partner lookup and the joins from projects/cases/global search
still see the row.

### One row per sf_id

`sf_id` is not unique, so every ingest write and parent lookup updates ONE row by `id`;
the tie-break lives in `internal/repository/sf_id_resolve.go`. Deletes still hit every copy.

## Salesforce membership ingest and onboarding steps

The same `POST /salesforce/events` endpoint also ingests customer **memberships**
— Salesforce `Project_Contact__c` (a Contact's membership of a project) and
`Contact` — when `CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED=true`. Off by default:
`routes.go` then constructs the service with `NewSalesforceEventService`, which
acknowledges those entities with 204 and ignores them (the behaviour before this
branch existed). On, it uses `NewSalesforceEventServiceWithMembershipIngest`
with a `service.MembershipIngest` (membership repo, onboarding-step repo, the
same `salesentity.Client`, the Contact writer's `SalesforceContactRepository`,
and the optional `eventPublisher`). This is the
**only** writer of customer memberships into Postgres: the customer portal and
the hourly reconcile job never write these tables themselves, they replay the
envelope (`{eventType, entity: "Project_Contact__c", referenceId}`) to this
endpoint and let the ingest re-read Salesforce.

**Entity dispatch** (`salesforce_event_service.go` → `salesforce_membership_ingest.go`;
entity names are matched case-insensitively, `Project_Contact` is accepted as
an alias of `Project_Contact__c`, and the raw value is logged):

| Entity | Event | Action |
|---|---|---|
| `Project_Contact__c` | CREATED / UPDATED / RESTORED | `GetProjectContact` (REST `POST /project-contacts/search`) then `GetContact` (`POST /contacts/search`) → `ProjectMembershipRepository.Upsert` in one transaction → DATABASE step → publish `project_contact.invited` if the state is INVITED / RE-INVITED. It is published to **`PROJECT_EVENT_HUB_TOPIC`** (default `project-events`), not the shared `EVENT_HUB_TOPIC`: onboarding gets its own topic so a case-event backlog cannot delay an invitation, and its dead-letter queue can be watched on its own. Same broker and credentials, same failure recording — only the topic differs, and csm-notification-service consumes it with its own consumer group. |
| `Project_Contact__c` | DELETED | `project_contact.state = DEACTIVATED` for that `sf_id`, and in the same transaction the user's account-level admin role is re-derived (`syncDerivedAdminRole`; the contact is read for its classification and `isCsAdmin`, and an unreadable contact leaves the roles alone); unknown id is a no-op (still 204). Never `DELETE FROM` |
| `Contact` | CREATED / UPDATED / RESTORED | The Contact writer (below): `GetContact` once, write `"user"` / `account_contact` / contact-derived roles + ledger, then the CREATED/UPDATED path above for each of its `memberships` **with the fetched contact** (no second read); every membership is attempted, the first error is returned |
| `Contact` | DELETED | No fetch: `account_contact.is_active` and `"user".is_active` = FALSE by `sf_id`, ledger stamped DELETED. Never a hard delete |
| anything else | any | 204, ignored |

An empty sales-entity-service result is a 503 (the ASB event can arrive before
Salesforce commits), the same posture as the Account branch.

**Duplicate-event guard.** Salesforce emits several UPDATED events per save and
the portal replays the envelope after its own write, so the same version
arrives more than once. `ingestMembership` parses the record's
`lastModifiedDate` (`2026-09-18T06:37:07.000+0000`, `parseSalesforceLastModified`)
and skips the upsert (204) when the membership's DATABASE step is `SUCCEEDED`
with an `eventModifiedOn` that is not older. A `FAILED` step never blocks a
retry. An unparseable date logs a warning and just runs the (idempotent)
upsert with `now()`.

**The upsert** (`repository/project_membership_repo.go`, actor
`domain.SalesforceSyncActor` = `salesforce-sync`) resolves every row by natural
key first and stamps `sf_id` on the way, so the same envelope can be replayed
any number of times and old rows that pre-date the flow get their `sf_id`
back-filled instead of duplicated:

1. `project` by `key` (the Salesforce subscription key), then by `sf_id` → 404.
2. `account` by `sf_id = contact.customerId`; a non-PARTNER membership falls
   back to the project's own account → 404. (The Contact writer, which runs
   first on a Contact event, ensures the contact's account with
   `EnsureAccount`; this step does not.)
3. `"user"` by `sf_id`, else by `LOWER(email)` — exactly one (`"user".email` is
   not unique; two matches are a 409 rather than a guess) — else inserted with
   `user_name = lower(email)`, `is_active = true`, `is_system_user =
   isCsIntegrationUser`. An existing row gets name / email / `is_system_user`
   refreshed, **never `user_name`** (it is the join key to `account_contact`).
4. Global roles (`user_role`, `mapGlobalRoles`): always `external`, plus
   `partner` when the **contact's account classification** is `Partner`
   (case-insensitive) and `customer` otherwise — decision D1, the ServiceNow
   script's basis; it used to follow the membership type (`PARTNER CONTACT`).
   `{customer, partner}` is a managed pair (`ManagedGlobalRoles`): the other
   half is revoked, so a reclassified account flips the role. When the
   classification is unknown (a portal write that has just created the
   contact) the contact is a customer and nothing is revoked. An integration
   user gets no global roles at all and nothing is revoked. A role name
   missing from the `role` table is a 503 naming it — the ServiceNow sync
   seeds those rows. The admin role is **not** decided at this step — see
   step 8.
5. `account_contact` by (`sf_id`, account), else (account, `LOWER(user_name)`),
   else inserted (`is_active = true`). `is_primary_contact` is Salesforce's
   `isPrimaryContact` on insert and update; when Sales Entity sends none it is
   FALSE on insert and left alone on update (the portal writes send none).
6. `project_contact` by `sf_id`, else (project, account_contact), else
   inserted; `email` and `state` (`INVITED` / `REGISTERED` / `RE-INVITED` /
   `DEACTIVATED`, `normalizeMembershipState`; anything else is a 400) updated.
7. Project groups (`project_contact_group`, `mapProjectGroups`, §6.4 of the
   onboarding design): `Portal user` + `Security Contact` → `Full Access`;
   `Portal user` → `General Access`; `Security Contact` → `Security Only`;
   `Lead` additionally → `Lead User Group`; `Admin` additionally → `Admin`
   (the group carrying the `ADMIN` project role, migration 0128 — `Admin`
   used to be global-only and recorded nothing per project); `Business
   Contact` additionally → `Business Contact  Group` (**two spaces**, the name
   ServiceNow gave it and the one stored in `project_group`, matched exactly;
   it carries the `BUSINESS_CONTACT` project role), so a contact can be, say,
   Portal user + Business Contact. The six labels with no CSM group (Business
   Owner / Promoter / Detractor, Technical Owner / Champion / Detractor) are
   ignored by decision D2; they and any unknown label are logged once per
   membership (`ignoredRoles` at info level, `unknownRoles` as a warning) and
   never fail the ingest. The row set is replaced. A missing `project_group`
   row is a 503. `salesforceRolesForGroups` maps the group back to `Business
   Contact`, so a portal deactivation that writes the stored roles back to
   Salesforce keeps it; the portal writes do not accept it as input.
8. The derived account-level admin role (`syncDerivedAdminRole`), run
   **after** step 7 so the membership just written counts: the user is an
   admin when any of their live memberships carries the ADMIN project role or
   the contact's `isCsAdmin` is set; the role is `partner_admin` for a
   Partner-classified account and `customer_admin` otherwise (the same basis
   as step 4), and the other one is always revoked. See "Admin is a project
   role" below for the bugs this replaced.

The DATABASE `onboarding_step` is written **inside the same transaction**
(`upsertOnboardingStep` takes a `querier`, satisfied by both the pool and a
`pgx.Tx`), so it can never disagree with the rows. On failure the ingest writes
`DATABASE = FAILED` with `lastError` best-effort and returns the original error.
`project_contact.invited` (`events.ProjectContactInvitedPayload`: membership /
contact Salesforce ids, email, given / family name, project name and key, the
raw Salesforce roles, `isIntegrationUser`, `type`, `eventModifiedOn` = the
membership's Salesforce LastModifiedDate, and an optional `resend` marker) is
published only after the transaction committed, only for INVITED / RE-INVITED,
**and only when this event actually moved the membership into that state** —
the ingest created the `project_contact` row
(`SalesforceMembershipUpsertResult.CreatedProjectContact`), or the row's
stored state before the upsert
(`SalesforceMembershipUpsertResult.PreviousState`) was something else; a nil publisher
skips it, a publish failure is logged (and recorded by
`EventPublisherService`), never returned. csm-notification-service consumes it,
provisions the Asgardeo user via the SCIM service and sends the invitation,
then records IDENTITY and EMAIL through the endpoints below (SKIPPED for an
integration user).

**Echo suppression — the STATE TRANSITION is the signal, not the row insert.**
Every portal membership write (below) also writes Salesforce, and every
Salesforce write comes back here through the Service Bus subscriber as an
ordinary CREATED/UPDATED envelope. By the time that echo lands the row already
exists **and already carries the new state**, because the portal write wrote
both first — so `PreviousState` equals the state the echo carries and nothing
is published. Publishing for it would have csm-notification-service send a
**second invitation e-mail for the one invitation the customer admin sent** —
one click, two mails. An echo updates the row silently instead. A genuinely
Salesforce-originated invitation (someone invited in Salesforce itself, or the
historical backfill) still creates the row here and still publishes.

This used to gate on `CreatedProjectContact` alone, which silently dropped
**re-invitations made in Salesforce**: those move an existing DEACTIVATED row
to RE-INVITED, so no row is created and the person was never told. Comparing
the previous state catches that case (DEACTIVATED → RE-INVITED differs, so it
publishes) while still suppressing the portal's own echo (RE-INVITED →
RE-INVITED is unchanged, so it does not). Do not put the insert-only condition
back.

**Contact who already signed in:** Salesforce saves their new membership as REGISTERED. It still gets `project_contact.invited` (existing-account email) when the row is new (created < 24 h ago in Salesforce) or comes back from DEACTIVATED, but not on RESTORED. The portal invite stores the REGISTERED state Salesforce returns and publishes.

`project_contact.registered` (`events.ProjectContactRegisteredPayload`) is published the same way, only on an
existing row moving INVITED / RE-INVITED → REGISTERED; csm-notification-service sends the Welcome email (step `WELCOME_EMAIL`).

**Schema prerequisite**: the `sf_id` columns on `"user"`, `account_contact`
and `project_contact` come from the csm-sync migration 0076, which is not in
this repo's `migrations/`; the ingest fails at the first `SELECT ... sf_id`
without it. `role` must contain `external`, `customer`, `partner`,
`customer_admin`, `partner_admin`; `project_group` must contain the groups
above (`Business Contact  Group` included, for a membership carrying that
label).

### The Contact writer

Before it, a Contact event only refreshed the contact's existing memberships,
so a contact with no membership (how a commercial or billing contact is
onboarded) wrote nothing to CSM. The Contact writer
(`internal/service/salesforce_contact_ingest.go`,
`repository.SalesforceContactRepository`) owns a contact's `"user"` and
`account_contact` rows and the contact-derived part of `user_role`, under the
same `CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED` flag; the membership
upsert stays the only writer of `project_contact`.

CREATED / UPDATED / RESTORED (`ingestContact`):

1. `GetContact` once. Sales Entity fields read beyond the membership path's:
   `isPrimaryContact`, `accountId` (else `account.id`), `account.classification`.
   `userActive` is decoded ahead of the Sales Entity release that exposes it
   and is not written yet.
2. Duplicate guard: `shouldSkipIngest` on the ledger (entity
   `domain.SalesforceIngestEntityContact` = `contact`) with the contact's
   `lastModifiedDate`. A duplicate skips the contact write but **not** the
   membership fan-out, so a membership that failed on the first delivery is
   retried by the redelivery (each membership has its own guard).
3. No account on the contact: logged, nothing written (account_contact needs
   one), fan-out still runs. Otherwise `EnsureAccount` on the contact's
   account; a failure (`NotFoundError` when the account is not in CSM and the
   Account ingest is off) records the ledger FAILED and fails the event, so
   Service Bus redelivers it, and the delayed-retry job re-runs it through
   `RetryContactIngest` once the account lands. A contact with no email (400)
   or a failed write also fails the event (not retried by the job).
4. One transaction, under a transaction-scoped advisory lock on the contact
   `sf_id`: `"user"` via `upsertMembershipUser` (by `sf_id`, then unique email;
   `user_name` never renamed; a user this writer soft-deleted — ledger
   DELETED — is reactivated, nobody else's `is_active` is touched); global
   roles via `syncGlobalRoles` with the managed `{customer, partner}` pair;
   `account_contact` via `upsertAccountContact` with `is_primary_contact`;
   the account move — the contact's active `account_contact` rows on other
   accounts go `is_active = FALSE`, **unless** the contact still holds a live
   (non-DEACTIVATED) membership on a project of that account (a partner
   contact legitimately keeps a row there); `syncDerivedAdminRole`; the ledger
   row SUCCEEDED.
5. Fan out to `contact.memberships` through `ingestMembership`, passing the
   fetched contact (re-read only if it is not the membership's own contact).
   The fan-out re-points each membership at the new account's
   `account_contact`.

Integration users keep the ingest's behaviour: a `"user"` with
`is_system_user = TRUE` and an `account_contact`, no global roles, no admin
decision.

DELETED (`deactivateContact`): no fetch (Salesforce hides deleted records).
`account_contact.is_active` and `"user".is_active` go FALSE by `sf_id`, and the
ledger row is stamped DELETED, keeping its recorded version (or now, for a
contact never ingested), so a later RESTORED is not skipped as a duplicate.
Nothing is hard-deleted: `project_contact` and `onboarding_step` reference these
rows. Roles are left alone (Salesforce deletes the memberships with their own
events).

**Onboarding steps API** (`onboarding_step`, migration 0118; Postgres-only,
404 without a pool, like `scheduled_task_run`): one row per
(`membershipSfId`, `step`), `step` ∈ IDENTITY / DATABASE / EMAIL /
REGISTRATION, `status` ∈ SUCCEEDED / FAILED / SKIPPED, `attemptCount`
incremented on every rewrite, `eventModifiedOn` = the Salesforce version the
write was based on.

- `PUT /onboarding-steps/{membershipSfId}/{step}` — body `{status, lastError?,
  eventType, eventModifiedOn, email, contactSfId?, projectId?,
  projectContactId?}` → 200 with the row. `lastError` is dropped unless
  `status` is FAILED (a stale error must not outlive a success) and truncated
  to 1000 characters (runes). Every method requires an internal caller
  (`AccessScope.Unrestricted`, i.e. `M2MClientIDs`); anyone else
  gets 403. `created_by`/`updated_by` is `onboarding-step-api` —
  callers are internal services, no identity is derived from the request.
- `GET /onboarding-steps/{membershipSfId}` → `{steps: [...]}` in step order; an
  unknown membership is an empty list, not a 404.
- `POST /onboarding-steps/search` — `{filters: {projectId?, membershipSfIds?,
  statuses?}, pagination}` → `{steps, total, limit, offset}`, newest first,
  `normalizePagination` (limit 20, max 50).

## Salesforce ingest ledger and the delayed-retry job

`salesforce_ingest_state` (migration 0170) is the per-record ledger of the
Salesforce ingest for every object that is not a membership (memberships keep
using `onboarding_step`). One row per (`entity`, `sf_id`), where `entity` is the
CSM table the record lands in (`domain.SalesforceIngestEntityAccount` = `account`,
`domain.SalesforceIngestEntityContact` = `contact` for the Contact writer, which
writes two tables; more to come per family); `status` ∈ SUCCEEDED / FAILED (CHECK constraint),
`event_modified_on` = the Salesforce `LastModifiedDate` the last write was based on,
`attempt_count` = consecutive failures (up while the row stays FAILED, back to 1 on
a success or on the first failure after one, so a record Salesforce saves often
never reaches the retry cap by succeeding). `repository.SalesforceIngestStateRepository`
(`Get`, `Upsert`, `ListMissingParentFailures`, `RecordRetryAttempt`) is generic over `entity`; a repository that writes
its own rows in a transaction records the ledger in that same transaction through
`upsertSalesforceIngestState(ctx, q querier, ...)`, like `upsertOnboardingStep`. The
upsert only moves the outcome columns when the incoming version is at least as new,
or the recorded row was stamped DELETED.

`shouldSkipIngest` (`internal/service/salesforce_ingest_guard.go`) is the duplicate
guard on that ledger, with `ingestMembership`'s three rules: skip when a SUCCEEDED
row's `event_modified_on` is not before the incoming version; a missing or
unparseable `LastModifiedDate` skips the guard (warning logged) and uses
`time.Now().UTC()`; a row last written by DELETED never blocks. FAILED rows never block.

`EnsureAccount(ctx, sfID)` on the Salesforce event service returns a parent
account's CSM id for a child ingest: it reads `account.id` by `sf_id`
(`SalesforceIngestSupport.Accounts`, wired regardless of flags); when absent and the
Account ingest is on it runs the ordinary `upsertAccount` and reads again; when
absent and the Account ingest is off it returns `NotFoundError`
`account not found for sfId "<sfId>"` — the prefix `repository.IsMissingParentError`
matches, so every child family's FAILED row is picked up by the delayed-retry job.

**The delayed-retry job** (`SalesforceIngestRetryWorker`,
`internal/service/salesforce_ingest_retry_worker.go`). Service Bus redelivers a
failed envelope five times within seconds, but a new project reaches CSM through
the ServiceNow sync up to five minutes later, so a membership on a brand-new project
dead-letters before its project exists. Started from `routes.go` (it needs the
membership-ingest-enabled service, publisher included) only when
`CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED=true` and
`SALESFORCE_INGEST_RETRY_INTERVAL` > 0; stopped by the `closePublishers` func
`cmd/api/main.go` calls on shutdown, before the producers close. Every tick (the
first one after one interval) it reads DATABASE steps with status FAILED, a
`last_error` starting "project not found" / "account not found", `updated_on`
older than the interval and `retry_count` < 12
(`OnboardingStepRepository.ListMissingParentFailures`), and re-runs
`ingestMembership` for each as UPDATED (`RetryMembershipIngest`), 30s timeout each,
at most 100 per tick. Each failed re-run counts one retry (`RecordRetryAttempt`:
`retry_count` + 1, `updated_on = now()`, `last_error` kept, while still FAILED);
redeliveries and new events never add to `retry_count` (migration 0174), and a
successful project/account ingest resets it on the rows whose error names that parent
(`RequeueMissingParentFailures`). So a parent that never arrives stops being retried after about an
hour at the default. FAILED ledger rows are read the same way
(`SalesforceIngestStateRepository.ListMissingParentFailures`, which applies the
registered-retrier, missing-parent and attempt-cap filters in SQL before the batch
limit, so a backlog of rows the job would skip cannot starve eligible ones) and handed to
`EntityRetriers[entity]`; `opportunity`, `linked_opportunity`, `invoice` and `opportunity_line_item` register one each
(`RetryOpportunityIngest`, `RetryLinkedOpportunityIngest`, `RetryInvoiceIngest`, `RetryOpportunityLineItemIngest`) when `CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED=true`, `project` one
(`RetryProjectIngest`) when `CSM_MIGRATION_SALESFORCE_PROJECT_INGEST_ENABLED=true`, and `contact` always
registers one (`RetryContactIngest`: the whole Contact writer as UPDATED, fan-out
included; it runs under the membership flag the job already requires), and
`account_partners` registers one (`RetryPartnerRefresh`) when
`CSM_MIGRATION_SALESFORCE_PARTNER_INGEST_ENABLED=true`. Other
entities are only counted (the Account ingest records FAILED rows but has no parent to wait for, so it
registers none).

## Salesforce Opportunity ingest

`POST /salesforce/events` acts on `Opportunity` envelopes when
`CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED=true` (off by default: they are
acknowledged and ignored). Code: `internal/service/salesforce_opportunity_ingest.go`
(attached to the event service with `WithOpportunityIngest` in `routes.go`),
`internal/repository/sf_opportunity_repo.go`, `GetOpportunity` in
`internal/salesentity/opportunity.go`. Plan: `docs/customer-onboarding/SALESFORCE_SYNC_PLAN.md` §7.

- **CREATED / UPDATED / RESTORED:** `POST /opportunities/search {id, limit: 1}` on the
  REST Sales Entity (empty result = `ServiceUnavailableError`, so Service Bus
  retries), then `shouldSkipIngest` on the ledger with entity `opportunity`, then
  `EnsureAccount(customerId)`, then one transaction that writes `sf_opportunity`, its
  line items and the SUCCEEDED ledger row.
- **`sf_opportunity`** by `sf_id` (not unique): advisory lock `"sf-opportunity:"+sfId`,
  update every row with the `sf_id`, else insert with `gen_random_uuid()`;
  `created_by`/`updated_by` = `salesforce-sync`, `sync_time_stamp = now()`. Written:
  `name`, `account_id`, `stage`, `is_won`, `close_date` (= `supportAccountEndDateRollUp`,
  not Salesforce `CloseDate`: what the ServiceNow script stored, decision D6),
  `eula_version`, `eula_version_decimal` (first `\d+\.\d+` in the version; `3.4` when
  the version is empty; NULL when it has no decimal). When Sales Entity does not send
  the `eulaVersion` key at all (a build without the field) both EULA columns are kept.
  **Never written:** `type`, `owner`, `engagement_code`, `query_hour_state`
  (ServiceNow-derived; `TestSfOpportunitySQL_NeverTouchesServiceNowColumns` pins the
  column lists). Values longer than their VARCHAR (migration 0080) are truncated and logged.
- **Line items (derived path):** the embedded `subscriptionLineItems` replace the
  opportunity's `sf_opportunity_product` rows as a set, in the same transaction:
  update by `line_item_sf_id` (else insert), then delete the rows under this
  `opportunity_id` the list does not mention. Written: `name`, `product_name`,
  `quantity`, `service_start_date`, `service_end_date`, `product_code`,
  `product_description`, `product_family`, `product_unit`, `eng_product_code`,
  `product_sf_id`, `classification`, `environment`, `total_price`. **Never written:**
  `development_support_hours`, `engagement_code`. With duplicate `sf_opportunity` rows
  for one `sf_id`, the row "One row per sf_id" picks owns the line items. Standalone `OpportunityLineItem`
  events are handled too (see "Standalone OpportunityLineItem" below).
- **DELETED:** hard delete `sf_opportunity WHERE sf_id = $1` (FKs cascade line items
  and project links, null invoices) plus a DELETED ledger row, in one transaction;
  a never-ingested opportunity is acknowledged and logged.
- **Failures:** an error after the fetch writes a FAILED ledger row (best effort,
  outside the rolled-back transaction). A missing account (Account ingest off) fails
  with `NotFoundError` "account not found for sfId ..." so the delayed-retry job
  re-runs it; with the Account ingest on, `EnsureAccount` ingests the account first.
  The retry job itself only runs while the membership ingest is on.
- **Duplicate guard:** the REST Sales Entity does not return `lastModifiedDate` for
  opportunities yet, so today the guard is skipped with a warning and the idempotent
  upsert runs on every event; it starts working once Sales Entity sends the field.
- **`Linked_Opportunity__c`** (also `Linked_Opportunity`), same flag:
  `internal/service/salesforce_linked_opportunity_ingest.go` (`WithLinkedOpportunityIngest`),
  `internal/repository/sf_opportunity_link_repo.go`, `GetLinkedOpportunity` on
  `POST /linked-opportunities/search`. CREATED/UPDATED/RESTORED: guard with entity
  `linked_opportunity`, resolve the opportunity through the shared `ensureOpportunity`
  (see "Invoice__c" below; the script's `_initOpportunity`), the project through
  `EnsureProject` (missing project with inserts off = `NotFoundError`, re-run by the retry
  job, which registers `RetryLinkedOpportunityIngest`), then upsert `sf_opportunity_link` by
  `link_sf_id` under lock `"sf-opportunity-link:"+id`: `number` (= Name), `opportunity_id`,
  `project_id`, `sync_time_stamp`. A link without a project or opportunity id is recorded
  FAILED and acknowledged. DELETED: hard delete by the 18-character `link_sf_id` plus a
  DELETED ledger row; a never-ingested link is acknowledged.

## Salesforce Project ingest

`POST /salesforce/events` acts on `Project__c` envelopes when
`CSM_MIGRATION_SALESFORCE_PROJECT_INGEST_ENABLED=true` (off by default: acknowledged and
ignored). Code: `internal/service/salesforce_project_ingest.go` (`WithProjectIngest` in
`routes.go`), `internal/repository/salesforce_project_repo.go`, `GetProject` in
`internal/salesentity/project.go`. Plan: `docs/customer-onboarding/SALESFORCE_SYNC_PLAN.md` §6.

- **CREATED / UPDATED / RESTORED:** `POST /projects/search {id, limit: 1}` (empty result =
  `ServiceUnavailableError`), `shouldSkipIngest` with entity `project` (Sales Entity sends
  `lastModifiedDate`, so the guard works), `EnsureAccount(customerId)`, `project_type_id` by
  exact `project_type.name` (unknown label = NULL + warning; types are never created), then
  one transaction: advisory lock `"project-sf:"+sfId`, resolve the row by `sf_id`, else by
  `key` (the `sf_id` is stamped on a key match), update, ledger row.
- **Written (only):** `sf_id`, `key`, `name`, `account_id` (kept when Salesforce names no
  account), `start_date`, `end_date`, `description`, `project_type_id`,
  `compliance_violation_date`, `onboarding_go_live_date` (`Go_Live_Date__c`: CSM owns it and
  writes it to Salesforce, decision D4, so this mirrors it back), plus `updated_on`/`updated_by`.
  Closure states, `wso2_closure_state`, hour counters, onboarding fields, credentials,
  `number`, `is_active` are never in the UPDATE (`TestSalesforceProjectSQL_ColumnList`).
- **Update-only by default:** with `CSM_MIGRATION_SALESFORCE_PROJECT_INSERT_ENABLED` off a
  project CSM does not have is `NotFoundError` "project not found for sfId ..." plus a FAILED
  ledger row, which the delayed-retry job re-runs. On, it is inserted with
  `gen_random_uuid()` and `is_active = TRUE`.
- **Empty `Project_Key__c` (decision D5):** refused: FAILED ledger row with the reason,
  event acknowledged (no retry can fix it).
- **DELETED:** `is_active = FALSE` on every row with the (18-character) `sf_id` plus a DELETED
  ledger row. RESTORED, or any upsert whose previous ledger row is a successful DELETED (or a
  failed RESTORED), sets `is_active = TRUE` again, in a separate statement touching only
  rows where it is FALSE.
- **`EnsureProject(sfId)`:** lookup by `sf_id` (`SalesforceIngestSupport.Projects`,
  flag-independent); when missing and the project flag and insert switch are both on, the
  project is fetched and upserted first. Otherwise `NotFoundError`. The membership ingest
  calls it when its upsert fails with "project not found" and retries the membership once;
  with inserts off it keeps the original error for the retry job.

### Invoice__c (same flag)

`Invoice__c` (also accepted as `Invoice`) envelopes are handled under
`CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED` too. Code:
`internal/service/salesforce_invoice_ingest.go` (`WithInvoiceIngest`),
`internal/repository/sf_invoice_repo.go`, `GetInvoice` in `internal/salesentity/invoice.go`,
and `ensureOpportunity` in `internal/service/salesforce_opportunity_ensure.go` (shared by
the linked-opportunity, invoice and line-item families).

- **CREATED / UPDATED / RESTORED:** `POST /invoices/search {id, limit: 1}` (empty =
  retryable), guard on entity `invoice` (Sales Entity sends `lastModifiedDate`), then
  `ensureOpportunity(opportunityId)` — look up `sf_opportunity` by `sf_id`, and when
  missing run the Opportunity branch inline (which ensures the account) **without the
  duplicate guard**, as `EnsureAccount` and `EnsureProject` do: a missing row means the
  ledger's "already written" is stale (a cascade or an out-of-band delete), so the
  opportunity is re-ingested at its current version — then one
  transaction under `"sf-invoice:"+sfId`: update every `sf_invoice` row with the `sf_id`,
  else insert, plus the SUCCEEDED ledger row. Every data column is written (`name`,
  `description`, `classification` — VARCHAR(40), truncated with a warning —
  `opportunity_id`, `invoiced_amount`, `invoice_date`, `invoiced_due_date`,
  `original_invoice_due_date` = `originalInvoiceDueDate ?? dueDate`, `invoiced_paid_date`,
  `service_start_date`, `service_end_date`, `sync_time_stamp`). An invoice with no
  `opportunityId` is written with a NULL `opportunity_id` and a warning. Auto-created
  invoices are included (the standalone search does not filter them).
- **DELETED:** hard delete by `sf_id` (widened to 18 characters) plus a DELETED ledger
  row; a never-ingested invoice is acknowledged.
- **Never from the Opportunity event:** the invoices embedded in an Opportunity lack the
  invoice date, the parent id and the original due date, and leave out auto-created
  invoices, so they are ignored (plan §7).
- **Retry:** `invoice` registers `RetryInvoiceIngest` with the delayed-retry job when the
  flag is on; a missing account under the inline Opportunity ingest fails with the
  "account not found ..." prefix it matches.

### Standalone OpportunityLineItem (same flag)

`OpportunityLineItem` envelopes are handled under
`CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED` too. Code:
`internal/service/salesforce_opportunity_line_item_ingest.go` (`WithOpportunityLineItemIngest`),
`internal/repository/sf_opportunity_line_item_repo.go`, `GetOpportunityLineItem` in
`internal/salesentity/opportunity_line_item.go`.

- **CREATED / UPDATED / RESTORED:** `POST /opportunity-line-items/search {id, limit: 1}`
  (empty = retryable; no `opportunityId` = retryable), guard on entity
  `opportunity_line_item`, `ensureOpportunity(opportunityId)`, then one transaction under
  the **parent opportunity's** lock `"sf-opportunity:"+opportunitySfId` (the lock the
  derived set replace holds, so the two paths cannot both insert one line item) that runs
  the derived path's own UPDATE-by-`line_item_sf_id`-else-INSERT with the same mapping
  (`mapSalesEntityLineItem`) plus the ledger row. `development_support_hours` and
  `engagement_code` are never written.
- **DELETED:** hard delete by `line_item_sf_id` (18 characters), under the owning
  opportunity's lock when the row is stored; a never-ingested line item is acknowledged.
- **Retry:** `opportunity_line_item` registers `RetryOpportunityLineItemIngest` when the
  flag is on.

## Allocation events (`POST /customer-engagements/allocation-events`)

Ports ServiceNow `processAllocationEvent` for allocation-app events, in one transaction, exactly as ServiceNow does: allocation types 76/83 find or create the Firefighting engagement by engagement id; every other type finds by line item only, or skips. Internal clients only. A skip answers 200 with a `reason`; only 5xx is retryable. Migration 0187's unique indexes back the `ON CONFLICT` upserts that make repeats safe. Stages and tasks are not ported. Stop csm-sync's `u_customer_engagement*` jobs before turning the flag on: their `delete_sync` removes rows written here.

## Salesforce partner relationships

The partner links in `account_relationship` ("partner **Is Partner Of** customer",
read by the invitation validator through `AccountPartnerRepository`) are refreshed
from Salesforce when `CSM_MIGRATION_SALESFORCE_PARTNER_INGEST_ENABLED=true` (off by
default). Code: `RefreshPartners` in `internal/service/salesforce_partner_ingest.go`
(attached with `WithPartnerIngest` in `routes.go`),
`internal/repository/account_partner_write_repo.go`, `GetCustomerPartners` in
`internal/salesentity/partners.go`. Plan: `docs/customer-onboarding/SALESFORCE_SYNC_PLAN.md`
§4 and decision D8 (store a copy).

- **Read:** `POST /customer-search {ids: [id], isRealTime: true, includePartners: true}`.
  A response without the `partners` key (or with `null`) is a `ServiceUnavailableError`,
  never "no partners": an older Sales Entity build must not wipe the stored links.
- **Write:** `EnsureAccount` for the customer and every partner first (a missing one is
  ingested when the Account ingest is on, else the refresh fails with "account not
  found ..." before writing anything), then one transaction under the advisory lock
  `"account-partners:"+customerSfId`: insert each missing forward row (`from` = partner,
  `to` = customer, `Is Partner Of` / `Is Customer Of`, `is_reverse_relationship = false`)
  and its mirror (`from` = customer, `to` = partner, labels swapped, `true`), delete the
  forward and reverse partner rows whose partner left the set, and record a SUCCEEDED
  ledger row with entity `account_partners` (sf_id = the customer). Rows of any other
  label are never touched; `relationship_type_id` stays NULL (the reader matches labels).
- **Callers, all behind the one flag:** every Account CREATED/UPDATED/RESTORED after the
  upsert, including a replay the duplicate guard skipped (a partner change does not move
  the account's `LastModifiedDate`; a failure fails the event so Service Bus redelivers
  it); every membership event whose contact's account differs from the project's account
  (best effort: logged and recorded FAILED, never failing the membership); the internal
  route `POST /salesforce/accounts/{sfId}/refresh-partners` (internal callers only,
  registered only with the flag on; answers the stored set); and the delayed-retry job
  (`RetryPartnerRefresh` under `account_partners`).
- **Not built yet:** the nightly sweep over every account (TODO in `RefreshPartners`); it
  is the only way to catch a partner added in Salesforce with no related save.

## Membership registration (`POST /users/me/memberships/register`)

H-0 of the customer onboarding flow. A customer invited in the Customer Portal
gets a Salesforce Contact and a `Project_Contact__c` membership in state
INVITED; something has to mark that membership REGISTERED once the person
actually signs in. **ServiceNow owns that today** — its verification page
clears the contact's lockout flag on first sign-in. After cutover the Customer
Portal owns it, and the work lives here rather than in the portal, so the
portal never needs Salesforce write access of its own.

`POST /users/me/memberships/register` → **204, no body**, no request body either. The
caller is the Customer Portal acting on behalf of the signed-in user, so it
carries an end-user token and the caller is resolved exactly the way
`GET /users/me` resolves it: the `email` claim of the already-validated
`x-user-id-token` (`middleware.UserIDTokenFromContext` → `emailFromJWT`).
A missing header is a 401, an undecodable token a 400 — no new convention.

**Postgres-only and off by default.** `CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED` must
be exactly `"true"` (same parse as every other flag here); while it is off
`routes.go` does not register the route at all, so it 404s and nothing on this
path can reach Salesforce. It also needs what it depends on — a pool, the four
`SALES_ENTITY_*` vars, and `CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED=true` for the
re-ingest — so with the ingest off this 404s too, rather than flipping
Salesforce with no matching database write.

**The no-op fast path is the point.** The portal calls this on *every* profile
load, and a membership is only ever INVITED once, so the overwhelmingly common
outcome is: one indexed read of `project_contact`, no rows, return. Nothing is
logged and Salesforce is never touched. Keep it that way — anything added to
this path runs on every profile load of every user.

`MembershipRegistrationRepository.InvitedMembershipsByEmail` (`membership_registration_repo.go`) is
that read: `project_contact` joined to `account_contact`, matched on
`LOWER(pc.email)` (the same join `access_repo.go`'s `RegisteredProjectIDs`
uses for the mirror-image state), state ∈ INVITED / RE-INVITED, returning
`project_contact.sf_id` (the membership) and `account_contact.sf_id` (the
Contact). A row missing either id is left out — it could not be flipped in
Salesforce anyway. Same `sf_id` schema prerequisite as the membership ingest
(csm-sync migration 0076).

**THE ORDER OF THE TWO SALESFORCE WRITES IS LOAD-BEARING — do not reorder
them.** Salesforce's `SN_T_Project_Contact` trigger recomputes every
membership's `State__c` from the Contact's "Locked Out [ Service Now ]" boolean
(`Contact.State__c`) on **every** save: locked out → INVITED, not locked out →
REGISTERED, unless the membership is DEACTIVATED. So per membership, in this
order:

1. `UpdateContactLockout(contactSfId, false)` — `PATCH /contacts/{id}`
   `{lockoutStatus: false}`, 200 with no body.
2. `UpdateProjectContactState(membershipSfId, "REGISTERED")` —
   `PATCH /project-contacts/{id}` `{state}`, 200 with the record
   sales-entity-service re-read from Salesforce (a failed re-read there is
   still a 200 with an **empty body**, which the client returns as a zero
   record and no error).

Doing it the other way round has the trigger overwrite REGISTERED back to
INVITED on the state write's own save — verified by hand. If step 1 fails,
step 2 is **skipped** for that membership: with the flag still set the state
write would be a no-op that looked like a success.

3. **Re-ingest**, so Postgres matches Salesforce before the request returns
   instead of whenever the ASB envelope for the same save arrives. This calls
   `SalesforceEventService.HandleEvent` — the very same entry point
   `POST /salesforce/events` is backed by — with the envelope Salesforce itself
   would have emitted (`{eventType: UPDATED, entity: "Project_Contact__c",
   referenceId: membershipSfId}`). That seam is deliberate: the mapping, the
   `ProjectMembershipRepository.Upsert`, the DATABASE step and the duplicate
   guard are then literally the same code, none of it reimplemented. The flip
   changed the record's `LastModifiedDate`, so the duplicate guard does not
   skip it. `routes.go` hands the membership-registration service the *same*
   membership-ingest-enabled `SalesforceEventService` value the Salesforce
   event handler holds.
4. **`REGISTRATION` = SUCCEEDED / FAILED** per membership via the existing
   `OnboardingStepRepository.Upsert`, actor `domain.SalesforceSyncActor`
   (`salesforce-sync`, the same actor the ingest records steps under),
   `eventType` UPDATED and `eventModifiedOn` = the persisted record's own
   `LastModifiedDate` when Salesforce returned one, else `now()`. The whole
   per-membership attempt is guarded as one unit: any of the three writes
   failing records FAILED with the (1000-rune-truncated) error, since the
   membership is only really registered once Salesforce is flipped *and*
   Postgres has caught up. The step write itself is best-effort and bounded by
   its own 3s timeout — the Salesforce side is already committed by then, so it
   is logged, never returned.

**Error posture**: one membership failing never stops the others; each failure
is logged with the membership and contact ids. An error is returned only when
**every** membership failed (the first one, so its own status mapping
survives) — a partial success is a 204, and the memberships that did not flip
are still INVITED, so the caller's next profile load retries them.

**PII**: no log line on this path carries the user's email address. Membership
and contact Salesforce ids identify the record. (The `onboarding_step` row does
store `email` — that column is part of the existing ledger and the ingest fills
it the same way; the rule is about logs.)

The two write methods live on the same `salesentity.Client` as the reads
(`UpdateContactLockout`/`UpdateProjectContactState`, `patch`/`patchWithRetry`
mirroring `search`/`searchWithRetry` — same token handling, same
refresh-once-on-401). One status mapping differs on purpose: a **404 on a
PATCH is a `NotFoundError`**, not the `ServiceUnavailableError` the read path
returns, because on a write against a record this service has already ingested
a 404 means the record is genuinely gone, not "Salesforce has not committed it
yet, retry". A 400 (Salesforce rejected the write) stays a `DownstreamError`.
`service.SalesEntityMembershipWriteClient` is a separate interface from
`SalesEntityMembershipClient` so the ingest cannot accidentally gain write
access to Salesforce; `*salesentity.Client` satisfies both.

Ten call sites publish today, all ServiceNow-data-source-only (`DATA_SOURCE=servicenow`;
there is no Postgres-backed equivalent for any of them). There is also one
Postgres-only exception, but it is **not** an Event Hub publish at all, and
it lives entirely in `case_repo.go`, not the service layer:
`CaseRepository.UpdateCase`'s severity branch and `AddCaseTag`'s "patch"
branch both call `recomputeTimeCardsBillable`, which sets every time_card
row under a case to `isLow && !hasPatchTag` — entering LOW/S4 severity
(WSO2's own support-policy tier, same mapping `sla_policy.go` uses) makes a
case's time cards billable, leaving it makes them non-billable, *unless* the
case carries a `"patch"` tag (case/whitespace-insensitive), in which case
they stay non-billable regardless — WSO2 still covers a patch under support
even for an otherwise best-efforts S4 case.

This used to be designed as an `events.TypeCaseBillableStatusChanged`
publish for csm-notification-service to react to (committed that way,
commented out, for a while), but a same-database write entity-service
already has transactional access to has nothing to gain from an event-hub
round trip through a separate service with no database of its own — that
type and its consumer (`csm-notification-service`'s own
`internal/timecardengine`) have both been removed entirely. An intermediate
revision then called this directly from the **service** layer
(`caseService.detectBillableStatusChange`/`detectPatchTagBillableOverride`,
via a plain `CaseRepository.SetTimeCardsBillableForCase(ctx, caseID,
isBillable)`), which a CodeRabbit review on the PR caught two real bugs in:

1. **The patch override wasn't checked live.** `detectBillableStatusChange`
   computed `isBillable` from the severity transition alone, with no idea
   whether a `"patch"` tag already existed — so a case tagged `"patch"`
   *before* it ever crossed into LOW still got marked billable on that
   crossing, and leaving-then-re-entering LOW after an earlier override had
   the same effect. Fixed by `caseHasPatchTag` checking `work_item_tag`/`tag`
   fresh, every time a LOW-boundary crossing happens or a `"patch"` tag is
   added — never relying on a value computed at some earlier, possibly
   stale, point in time.
2. **No ordering guarantee between the severity write and the time-card
   write.** The service-layer version ran `SetTimeCardsBillableForCase` as
   a separate call *after* `UpdateCase`/`AddCaseTag` had already committed —
   two concurrent severity-changing requests on the same case could
   interleave such that the slower one's (now-stale) time-card write landed
   *after* the faster one's, leaving the final committed severity
   disagreeing with the final time-card billable state. Fixed by moving the
   recompute **inside** `UpdateCase`'s existing transaction (which already
   takes `SELECT severity::TEXT FROM "case" WHERE id = $1 FOR UPDATE` before
   writing — this reuses, not adds, that lock) and `AddCaseTag`'s own
   transaction (which now takes the identical lock on its `"patch"` branch
   before checking/writing) — the same case row's lock serializes two
   otherwise-racing writers against each other exactly like
   `AcknowledgeCase`'s own "first write wins" pattern already does, so
   whichever transaction commits last is also the one whose
   fresh-within-that-transaction read determines the final state. Both
   writes are still best-effort within their own transaction (a failure is
   logged, never allowed to roll back the severity/tag change that already
   succeeded) — the fix is ordering and freshness, not changing that
   posture.

`time_card` (migration 0041) already has a real `is_billable` column and
full Postgres CRUD (`time_card_repo.go`/`time_card_service.go`) — the
"Postgres has no time_cards table/repo/service" premise the original,
commented-out event design rested on was stale by the time any of this was
revisited.

- **`snCaseService.CreateCase`** publishes `case.created` via a private
  `publishCaseCreated` helper, called after the SN create call succeeds.
  Rather than building the payload from `req`/the create response (which
  carries only a few fields — see `snCreateCaseResponse`), it re-fetches the
  case via `GetCaseByID`, whose own SN response already resolves the
  reporter's display name, the project's name, and each watcher's email —
  exactly what `events.CaseCreatedPayload` needs. A case created with no
  recipients (either way, see below) is a normal state, not an error, so
  publishing is silently skipped rather than sending a payload
  `csm-notification-service`'s `events.Validate` would reject anyway for an
  empty `recipients` list.

  **Only `type: "case"` requires a severity to publish at all.**
  `CaseCreatedPayload.Priority` has no `omitempty` (a consumer always
  expects a real value) and `""` is not a real priority, so a nil severity
  used to skip the whole publish — but severity is a `"case"`-only column
  (`validateCreateCaseRequest`), so that gate previously meant the other
  four types `publishCaseCreatedEvent` also serves —
  `engagement`/`service_request`/`security_report_analysis`/`announcement`
  — never published `case.created` at all. **Fixed at explicit request**:
  the severity gate now only applies when `req.Type == "case"`; the other
  four publish regardless, with `Priority` simply left `""`
  (`csm-notification-service` already renders that gracefully — see its own
  `CLAUDE.md`).

  **`Recipients` depends on `req.Type`.** For `case`/`engagement`/
  `service_request`/`security_report_analysis` it's the case's own resolved
  watch list emails, unioned with the account's four default-watcher
  stakeholders resolved fresh via `CaseRepository.AccountDefaultWatcherEmails`
  — see "Case watch list" below for why those four are resolved at publish
  time rather than read from the persisted watch list. For `announcement`,
  `publishCaseCreatedEvent` instead resolves the audience via
  `CaseService.ProjectContactEmailsByRole` — every `project_contact`
  currently holding the `SECURITY_CONTACT` project role when
  `req.IsSecurityAnnouncement` is true, else every contact holding
  `PORTAL_USER` — bypassing the watch-list mechanism entirely, since a
  project contact often has no matching `"user"` row to add as a
  `work_item_watcher` (`work_item_watcher.user_id` is `NOT NULL`). Falls
  back to the case's own watch-list emails (still unioned with the account's
  default watchers) when no contact holds the requested role for that
  project — a project with nobody in the requested role must still notify
  someone, not silently notify no one. `ProjectContactEmailsByRole` is Postgres-only
  (`project_contact`/`project_role` have no ServiceNow equivalent); on
  `snCaseService` it delegates to `pgFallback` when configured, else
  returns empty (no error) — same "can't resolve, skip" posture as every
  other Postgres-only gap in this file.

  `csm-notification-service`'s own `handleCaseCreated` mirrors this split
  on the Chat side: its Google Chat alert is skipped entirely for these
  same four non-`"case"` types (an exclude-list keyed on
  `CaseCreatedPayload.CaseType`) — those types notify by email only, per
  the same explicit request. See that service's own `CLAUDE.md`.
- **`snIncidentService.CreateIncident`** publishes `incident.created` via
  `publishIncidentCreated`, called the same way. `Title`/`ShortDescription`
  come straight from `req.Subject`/`req.AdditionalComments` (the latter
  falling back to `Subject` when absent), and `Number`/`ReportedAt` from the
  create response. The **escalation fields** need one best-effort
  `GetIncidentByID` read: `csm-notification-service`'s call-escalation ladder
  (its `internal/paging`) is keyed on the incident's *priority*, which
  ServiceNow derives from impact and urgency and which neither `req` nor the
  create response carries, and on the assigned team's display name, where
  `req` has only a sys_id. That read is deliberately not fatal and not even
  required: if it fails, the event goes out with exactly the fields it
  carried before the ladder existed (every escalation field is `omitempty`
  on both sides), the direct call still happens, and the ladder simply
  doesn't start — losing the page would be strictly worse than losing the
  ladder. `Account`/`ABTEligible` are declared on the payload but **never
  populated** here: incidents have no account field in this domain model,
  and this service has no product→BU mapping to derive ABT eligibility from.
  `ABTEligible` is a `*bool` for exactly that reason — an absent value must
  stay absent rather than decoding as an explicit `false`, which would claim
  an answer nobody gave — see that service's own `CLAUDE.md` for what the
  missing flag does to the USA_WEEKEND routing rule.
  On `DATA_SOURCE=postgres` (`NewIncidentServiceWithPublisher`, with no
  ServiceNow behind it) later work notes also go through `PATCH /incidents/{id}`
  -- an alert-born SRE incident's follow-up alerts from `sre-alert-core-service`
  -- written as comments in one transaction (`CreateIncidentNotes`: a work note
  and a comment commit together or not at all), with no ServiceNow mirror. That
  create path publishes the same enriched `incident.created` (the read-back is a
  Postgres `GetIncidentByID`), so an alert-born incident reaches the ladder with
  its priority and team. The same update sends `incident.acknowledged`/
  `incident.assigned` when it moves the incident out of NEW or sets an assignee
  (`publishIncidentStopSignals`).
  `incident.created` has exactly one reaction on the receiving side now — a
  Twilio voice call — not a Google Chat alert: `csm-notification-service`
  removed that reaction entirely, per explicit product direction (an
  incident pages on-call directly; a separate Chat post was redundant with
  that). The escalation ladder's own `chat` channel is a different thing and
  is unaffected: it posts a card per *rung* of a climbing escalation, not one
  on creation. This service does not build or send an `IncidentLink` at all —
  it stays strictly a publisher of the fact that an incident was created;
  `csm-notification-service` builds its own portal link from the event's
  `EntityID` (`recipientlinks.Resolver.IncidentLink`), the same way it
  already builds `case.created`'s. `CallTo` (on-call number) is never set
  from this service either — per explicit decision, all notification-routing
  resolution belongs entirely in `csm-notification-service`, which
  substitutes its own configured `INCIDENT_DEFAULT_CALL_TO` when it is absent
  from the payload. `Product` is still accepted on the wire (decode
  compatibility) but no longer read by `csm-notification-service` at all.
  Consuming events and sending emails/Chat alerts/calls is never this
  service's job — only publishing the raw fact that something happened is.
- **`incidentService.createIncidentPortal`** (plain `DATA_SOURCE=postgres`)
  publishes the same `incident.created`, through the same
  `publishIncidentCreatedEvent` the dual-write path uses, once the insert
  has committed. That create reproduces ServiceNow's
  `IncidentUtils.createIncident`: state New, priority from impact × urgency
  (`incidentPriorityFor`), the optional fields it sets (subcategory,
  assigned engineer, `cmdb_ci_id`, watch list), and `additionalComments` /
  `workNotes` as COMMENT / WORK_NOTE rows, all in one transaction.
- **`snCaseService.CreateCaseComment`** publishes `case.comment_added` via
  `publishCommentAdded`, called after the SN comment-create call succeeds.
  Enriches via `GetCaseByID` for `ProjectID`/`CaseTitle`/`Recipients`, the
  same as `publishCaseCreated`. `events.CommentAddedPayload.Name` (the
  comment author's resolved display name) is the one field that call can't
  supply: ServiceNow's create-comment response (`snCreateCommentResponse`)
  carries only a raw, unresolved `CreatedBy` string, and every other
  resolved-author-name lookup in this file goes through a GET/search
  response, never a bare create-acknowledgment one. `publishCommentAdded`
  resolves it via a second call, `resolveCommentAuthorName`
  (`SearchCaseComments`, matching the just-created comment by id in a
  bounded first page — `resolveCommentAuthorNameSearchLimit`, currently 20;
  the new comment is essentially certain to be within that many of the
  case's most recent regardless of `SearchCaseComments`' own sort order,
  which this service doesn't control). If that lookup doesn't find it (an
  unlikely ordering edge case), publishing is skipped rather than sending an
  event with an empty or fabricated author name — same "skip rather than
  send something `events.Validate` would reject" precedent as an empty
  `Recipients` list. When `req.Type` is `domain.CommentTypeWorkNote` (an
  internal note — never meant for a customer to see), `Recipients` is
  filtered down to `wso2EmailDomain` (`@wso2.com`) addresses only via
  `filterWso2Emails`, regardless of who else is on the case's watch list —
  a case's watch list can include customer watchers, and an internal note
  must never notify them just because they happen to be watching the case.
  `wso2EmailDomain` mirrors `apps/csm-portal/backend`'s own constant of the
  same name. `events.CommentAddedPayload.IsInternalNote` is set to
  `req.Type == domain.CommentTypeWorkNote` on every publish — `csm-notification-service`
  renders a distinct email layout for it (`RenderInternalNoteEmail`, see
  that service's own `CLAUDE.md`), so it needs to know the comment's type,
  not just receive an already-filtered recipient list. `CommentAddedPayload`
  also carries `AuthorEmail` (the same resolved author identity as `Name`,
  just the address rather than the display name — `author.Email`/`actorEmail`
  at each of this payload's two call sites), `Product` (`caseProductName(cv)`),
  and `Team`/`IsEvaluationAccount`/`ProjectOnboardingStatus` — purely for
  `csm-notification-service`'s own consumption: `AuthorEmail` lets that
  service classify whether a new comment is customer-authored before running
  it through its frustration-detection step (see that service's own
  `CLAUDE.md`, "Frustration detection"), and `Team`/`IsEvaluationAccount`/
  `ProjectOnboardingStatus` let a resulting Chat alert route through
  `chataudience.Resolve` the same team-first way an SLA breach alert does,
  rather than always the fixed `Incident Monitor` audience. The latter two are
  resolved via the new `CaseRepository.ProjectOnboardingInfo(ctx, projectID)`
  (a small, on-demand `project`/`project_type` join — the same two facts
  `sla_status_repo.go`'s own bulk join already resolves for `GET
  /sla-status`, just read here per-comment instead of in bulk), reached from
  `snCaseService` via the same `pgFallback`-delegation pattern as
  `AccountDefaultWatcherEmails` just above (empty/`false` with no error on a
  pure-ServiceNow deployment with no Postgres pool). A lookup failure here is
  logged and the fields are simply left at their zero value — the email
  reaction `publishCommentAddedEvent` exists to drive must never be blocked
  by this enrichment failing.

- **`snIncidentService.UpdateIncident`** publishes the two signals that
  start and stop a call escalation, via `publishEscalationSignals`:
  `incident.acknowledged` when the incident genuinely **leaves NEW** (the
  specification's acknowledgement gesture for a newly reported incident), and
  `incident.priority_elevated` when its priority **strictly increases in
  urgency** (the second trigger, keyed on the new priority). A change to
  `Impact` or `Urgency` counts as a priority change for both purposes:
  ServiceNow derives priority from them — `CreateIncident` requires both and
  accepts no priority at all — so a PATCH raising urgency raises the priority
  just as surely as one naming it. Both are guarded
  against a no-op re-PATCH the same way `publishSeverityChanged` is, which
  needs the incident as it was *before* the PATCH — so `UpdateIncident`
  fetches a baseline first, but only when the request touches `State` or
  `Priority`, so every other PATCH pays no extra round trip. A failed
  baseline fetch publishes nothing rather than guessing at a transition. The
  elevated payload carries the same optional escalation fields as
  `incident.created`, from the post-PATCH view, plus `ElevatedAt` (`now`,
  the instant the ladder's offsets run from). Neither carries an actor:
  `UpdateIncidentRequest` has none and this service cannot resolve who
  performed an update. A third signal, **`incident.assigned`**
  (`publishIncidentAssigned`), goes out when `AssignedEngineerID` genuinely
  changes the assignee to someone (`incidentAssignment`: not on a re-send of
  the same assignee, not when it is cleared), carrying the assignee's id and
  display name from the post-PATCH view. It is the SRE escalation ladder's
  stop signal ("assignee set on incident"); the CRE ladder ignores it. It
  shares the pre-PATCH baseline fetch, which an `AssignedEngineerID` PATCH now
  also triggers.
- **`snCommentSearchService.CreateComment`** (the reference-generic comment
  service, ServiceNow branch only) publishes `incident.comment_added` via
  `publishIncidentCommentAdded` whenever a comment lands on an
  `incident`-type reference. This is the **only** stop signal an
  elevation-triggered ladder has: the specification's acknowledgement gesture
  for a priority elevation is a *public comment*, and an elevated incident
  has normally already left NEW, so `incident.acknowledged` can never fire
  for it again. Work notes are published too, with `IsPublic: false` — the
  consumer decides that only a public comment acknowledges, keeping the event
  a statement of fact rather than baking a notification policy into the
  service that owns the data. The payload carries **no author**, a recorded
  known gap: incidents have no customer-portal surface today, so a public
  comment on one is written by internal staff in practice; if that ever
  changes, an author must be added and checked (see
  `snCaseService.resolveCommentAuthor` for the lookup it would need). The
  comment service gained an optional `publisher EventPublisherService` for
  this, wired from `routes.go`'s existing `eventPublisher`; the Postgres
  comment service publishes nothing, same as every other publisher here.

`publishCaseCreated`, `publishCommentAdded`, `publishStatusChanged`, and
`publishCaseAssigned` — every `case.*` publisher above, not
`snIncidentService.CreateIncident` — also set `CaseNumber`
(`cv.Number`/`before.Number`, the case's human-readable ServiceNow
reference, e.g. `"CS0023001"`) and `WSO2CaseID` (`cv.InternalID`/
`before.InternalID`, ServiceNow's `u_wso2_case_id` custom field — the CSM
portal's own case identifier, e.g. `"WSO2-1000"`, distinct from
`CaseNumber`) alongside `CaseID` (the UUID) — `csm-notification-service`
displays `WSO2CaseID`/`CaseNumber` in every subject line and template slot
instead of the UUID, which is meaningless to an end user (a real, reported
bug before these fields existed at all); `CaseID` is unchanged for anything
link-related. `publishStatusChanged`/`publishCaseAssigned` additionally set
`CaseTitle` (`before.Subject`) — neither `case.status_changed` nor
`case.assigned` originally carried one at all, needed once
`csm-notification-service` started requiring every
`case.*` email's subject to follow one explicit standard format,
`"[WSO2 Support] (<wso2 case id>/<case number>) <title>"` (see that
service's own `CLAUDE.md`, `dispatch.subjectLine`).
- **`snCaseService.UpdateCase`** publishes `case.status_changed` via
  `publishStatusChanged`, called only when the PATCH's own `req.State` was
  set (a `nil` `State` — e.g. an `assigneeEmail`-only PATCH — never
  triggers this; `State`/`Severity`/`WorkState`/`WatchList`/`AssigneeEmail`/
  `ParentID`/`Acknowledge` are already mutually exclusive per request, so a
  single `UpdateCase` call can never be both a status change and something
  else). `NewStatus` is the raw ServiceNow state label from the update
  response (`snResp.Case.State.Label`, e.g. `"Work In Progress"`) rather
  than `domain.CaseState`'s own enum conversion
  (`snCaseStateLabelToEnum`) — the enum conversion silently leaves the
  domain value unset on an unrecognized label, while the raw label is
  always present whenever `snResp.Case.State` is non-nil. `Recipients`/
  `ProjectID` need a fresh `GetCaseByID` call regardless:
  `snUpdateCaseResponse`'s own `WatchList` has emails but no project
  reference at all.

- **`snCaseService.UpdateCase`** also publishes `case.assigned` via
  `publishCaseAssigned`, called only when `req.AssigneeEmail` was set (the
  mirror image of the `case.status_changed` path above — `State`/
  `AssigneeEmail` are mutually exclusive per request, so a single
  `UpdateCase` call is never both). This was blocked for a while on
  identity: `csm-notification-service`'s `CaseAssignedPayload` used to
  require a non-empty `AssignerName`/`AssignerEmail` — the person who
  *performed* the assignment — and this service has no inbound-auth/
  identity layer able to resolve that (the `x-user-id-token` header
  `middleware.UserIDTokenFromContext` forwards is opaque, just a
  pass-through to ServiceNow, not a decodable identity). The actual
  unblock was realizing that's the wrong question: `req.AssigneeEmail` (the
  new assignee, not the assigner) is directly on the update request with
  no resolution needed at all, and `csm-notification-service`'s payload was
  renamed `AssigneeName`/`AssigneeEmail` to match — see that service's own
  `CLAUDE.md`. `publishCaseAssigned`'s `AssigneeName` comes from
  `snResp.Case.AssignedTo.Name` (ServiceNow's own resolved display name
  from the PATCH response), falling back to the email if that's empty;
  `AssigneeEmail` is `*req.AssigneeEmail` verbatim — guaranteed correct
  since it's exactly what the caller requested. Same pre-PATCH
  `GetCaseByID` no-op guard as `case.status_changed`: a caller re-PATCHing
  the case's own current assignee must not send every watcher a false
  "case assigned" email — compares `cv.AssignedEngineer.Email` against
  `*req.AssigneeEmail` before the PATCH, same as `cv.State` there.

- **`snCaseService.UpdateCase`** also publishes `case.acknowledged` via
  `publishCaseAcknowledged`, called only when `req.Acknowledge` was true and
  the acknowledge genuinely claimed the case for the first time —
  `resp.Case.AlreadyAcknowledged` distinguishes that from a repeat
  `Acknowledge:true` call that succeeded without changing anything (see
  `UpdateCaseRequest.Acknowledge`'s own doc comment); only the former is a
  real event worth a Chat alert. Chat-only, like `case.assigned` used to be
  blocked and now isn't — but `case.acknowledged` has no email reaction at
  all, ever, so its own `events.CaseAcknowledgedPayload` has no
  `Recipients`/watch-list concept whatsoever, unlike every other `case.*`
  payload. Re-fetches via `GetCaseByID` rather than trusting the PATCH
  response, same "re-fetch rather than trust a narrow response" precedent
  as `publishCaseCreated`: `snUpdateCaseResponse`'s acknowledge path only
  ever echoes `Number`/`AlreadyAcknowledged`/`AcknowledgedBy`, none of which
  cover `CaseNumber`/`WSO2CaseID`/`Severity`/`Product` — everything
  `csm-notification-service`'s Chat alert needs to display (see that
  service's own `CLAUDE.md` for the card's exact shape).

- **`snCaseService.UpdateCase`** also publishes `case.severity_changed` via
  `publishSeverityChanged`, called only when `req.Severity` was set AND
  actually differs from the case's prior severity — the same pre-PATCH
  `GetCaseByID` no-op guard `case.status_changed`/`case.assigned` use
  (`req.State`/`req.Severity`/`req.AssigneeEmail` are already mutually
  exclusive per request, so this and the status/assignee blocks never both
  fire for the same call). Unlike `case.acknowledged`, this has both an
  email reaction (`Recipients`, the same watch-list-emails audience as
  `case.status_changed`/`case.assigned`) and a Chat alert (`Product`, same
  `caseProductName(before)` reasoning as `publishCaseCreated`/
  `publishCaseAcknowledged`) — `csm-notification-service`'s `dispatch`
  package fans this one payload out to both channels. `OldSeverity` comes
  from the pre-PATCH `GetCaseByID` enrichment (`before.Severity`);
  `NewSeverity` from the PATCH response's own echoed severity
  (`resp.Case.Severity`, only set when `snResp.Case.Severity != nil`) — no
  second `GetCaseByID` needed the way `publishCaseAcknowledged` needs one,
  since `UpdateCase`'s existing pre-PATCH enrichment already supplies
  everything this payload needs (`CaseNumber`/`WSO2CaseID`/`CaseTitle`/
  `Product`/`Recipients` all come from that same `before` `CaseView`). Same
  "empty `Recipients` list skips the whole publish" precedent as
  `publishCaseCreated` — including the Chat alert, since this event has no
  Chat-only path the way `case.acknowledged` does; a severity change with
  no watchers has nobody to notify by design.

`caseProductName(cv)` (a small shared helper) resolves
`cv.DeployedProductDetails.Product.Name` (e.g. `"WSO2 API Manager"`, `""`
when the case has no deployed product) — used by `publishCaseCreated`,
`publishCaseAcknowledged`, and `publishSeverityChanged` to populate their
payloads' `Product` field, a purely-display value in
`csm-notification-service`'s Chat cards. `CaseCreatedPayload.Product` was
previously never populated at all ("this service has no data source for it
yet"); it plays no role in Chat routing on the receiving side —
`csm-notification-service` removed its earlier per-product Chat-space
routing (`GoogleChatConfig.Spaces`, `DEFAULT_CHAT_PRODUCT`) once it became
clear this deployment has no real per-product Chat space need:
`case.created`/`case.acknowledged`/`case.severity_changed` now always route
to a single fixed Chat audience regardless of `Product` — see that
service's own `CLAUDE.md`.

`caseTeamName(cv)` (same shared-helper pattern) resolves
`cv.AccountDetails.CreTeam.Name` (e.g. `"Team Nova"`, `""` when the case
has no account or the account has no CRE team) — used by the same three
publishers to populate their payloads' `Team` field, also purely display in
`csm-notification-service`'s Chat cards for these three event types (SLA
breach alerts are the one place a case's team genuinely drives Chat
routing — see `csm-notification-service`'s own `CLAUDE.md`, "SLA
breach-alerting engine"). `cv.AccountDetails` (and its `CreTeam`) is
resolved by `GetCaseByID` from the case's own embedded ServiceNow account
object at no extra request cost — but as of this field's introduction,
that embedded object's `creTeam`/`sreTeam` are documented in
`snCaseAccount`'s own doc comment as not yet guaranteed to be populated by
the ServiceNow integration, even though the standalone accounts endpoint
does return them. `Team` may therefore come back empty in practice until
that catches up — not a bug in this service if so.

**`caseService.UpdateCase` (the Postgres data source) supports
`Acknowledge`/`AssigneeEmail` too** — `caseService.acknowledgeCase`/
`updateCaseAssignee` (own branches in `UpdateCase`, alongside
`updateCaseWatchList`/`updateCaseParent`/`updateCaseFields`) write
`work_item.acknowledged_by_user_id`/`assigned_to_id` via
`CaseRepository.AcknowledgeCase`/`UpdateCaseAssignee`.
`CaseRepository.AcknowledgeCase` claims a case first-write-wins inside a
transaction that row-locks `work_item` (`SELECT ... FOR UPDATE`) before
reading whether it's already claimed, so two concurrent Acknowledge calls on
the same case can't both believe they were first; it returns
`alreadyAcknowledged` plus whoever now holds the claim, read back from the
same "user" join regardless of which branch actually ran. `UpdateCaseAssignee`
has no such guard at all — it unconditionally writes `assigned_to_id` every
call, no no-op detection in the repository layer.
- **This service's own contribution on top of that**: neither
  `acknowledgeCase` nor `updateCaseAssignee` originally published a
  `case.acknowledged`/`case.assigned` event on the Postgres data source at
  all — Acknowledge/AssigneeEmail worked (the write itself succeeded,
  ServiceNow-mirror dispatch fired under dual-write mode), but
  `csm-notification-service` never heard about either one unless
  `DATA_SOURCE=servicenow`. Closing that gap added:
  - **`updateCaseAssignee`'s own no-op detection**, done at the service
    layer since the repository doesn't do it: a `GetCaseByID` fetch right
    before the write (gated on `s.publisher != nil`, so a deployment with no
    Event Hub configured pays nothing extra) compares
    `cv.AssignedEngineer.Email` against the requested `AssigneeEmail`
    case-insensitively — the same guard `snCaseService.UpdateCase`'s own
    AssigneeEmail path applies, just done here instead of in SQL.
  - **`publishCaseAcknowledged`/`publishCaseAssigned`**, reusing the exact
    same shared helpers/payload shapes ServiceNow's own versions do
    (`caseProductName`/`caseTeamName`/`watchListUserEmails`,
    `events.CaseAcknowledgedPayload`/`CaseAssignedPayload`) — both live in
    the same `service` package, so nothing needed duplicating.
    `publishCaseAcknowledged` only fires when `!alreadyAcknowledged` (a
    repeat `Acknowledge:true` against an already-claimed case changed
    nothing, so nothing to publish — the same distinction
    `snCaseService.publishCaseAcknowledged`'s own call site makes) and takes
    `acknowledgerName` straight from `AcknowledgeCase`'s own return value, no
    second lookup. `publishCaseAssigned` re-fetches the case via
    `GetCaseByID` *after* the write (the pre-write fetch above exists only
    to detect the no-op, not to reuse as a payload source) and guards
    `cv.ProjectDetails` as nilable — unlike ServiceNow's `CaseView`, which
    always has one, this data source's does not (see `GetCaseByID`'s own
    comment on why project/deployment joins are `LEFT JOIN`s here).
  - **`GetCaseByID`'s query now also joins `acknowledged_by_user_id`**
    (`LEFT JOIN "user" ack ON ack.id = wi.acknowledged_by_user_id`, same
    pattern as its pre-existing `assigned_to_id`/`ae` join) to populate
    `CaseView.AcknowledgedBy` — previously always nil on this data source
    even after a successful Acknowledge, since nothing read the column back
    for display outside `AcknowledgeCase`'s own one-off query.
- **Not carried over from the ServiceNow path**: there is no elevated-role
  check on `Acknowledge` here — `acknowledgeCase`'s own doc comment notes
  this is deliberate, not an oversight: no Postgres-side permission model
  exists yet, so this data source only requires a known authenticated
  caller, same as every other Postgres case mutation. `SearchCaseView` still
  has no `AcknowledgedBy` field either (matching ServiceNow's own
  `SearchCaseView`-equivalent, which doesn't surface it there either — only
  `GetCaseByID` does).

**`work_item_activity` (migration 0055) now gets written to on the
Postgres data source too.** `SearchCaseActivities`' own `field_change` branch
already rendered any `field_name` generically (`caseActivityFieldChangeLabel`
title-cases it, e.g. `"assigned_to_id"` → `"Assigned To Id"`) — the table was
fully wired up on the read side, but **nothing in this codebase ever wrote to
it** on this data source (the ServiceNow data source's own case activity
comes from a live upstream call instead, not this table; this table appears
to exist for the ServiceNow data source's own sync process to populate,
which this data source has no equivalent of). The practical symptom: a
Postgres-native state/severity/workState/assign/acknowledge/parent change
produced no entry in the case's own activity feed at all — a real, reported
gap ("with servicenow data source all are shown").
- **`CaseRepository.RecordCaseFieldChangeActivity(ctx, caseID, fieldName,
  oldValue, newValue, actorEmail)`** is the missing write half — a plain
  `INSERT INTO work_item_activity`. `caseService.recordFieldChangeActivity`
  wraps it best-effort (log and ignore on failure, same posture as every
  `publishXxx` helper in this file): the mutation itself has already
  succeeded by the time this runs, so a failure here must never undo it or
  report the request as failed. A blank `actorEmail` skips the write
  entirely (no anonymous rows) rather than inserting one with an empty
  `user_email`.
- **Old/new value convention, since there's no ServiceNow sync to match
  against**: `state`/`severity`/`work_state` store the raw lowercase domain
  enum value (e.g. `"work_in_progress"`), the same representation the JSON
  API already uses for these fields — no separate display-label map
  invented purely for this. `assigned_to_id`/`acknowledged_by_user_id`
  store a human display name instead (e.g. `"Jane Doe"`, or `"Unassigned"`
  for "no prior assignee") — a raw UUID would be useless to a human reading
  the activity feed, and `caseActivityFieldChangeLabel` already title-cases
  `field_name` itself to say *which* field, so the value only needs to say
  *who*. `parent_id` stores the parent case's own number (e.g.
  `"CS0001"`), fetched via an extra best-effort `GetCaseByID` before/after
  the write in `updateCaseParent` (rare operation, so the extra round trips
  are an acceptable cost) — an empty string if that lookup fails.
- **Call sites**: `updateCaseAssignee`/`acknowledgeCase`/`updateCaseParent`
  already resolve an `actor` for other reasons (ServiceNow mirror
  attribution, first-write-wins claiming) and reuse `actor.Email` directly.
  The main `UpdateCase` body's state/severity/workState branch is the one
  exception — it has never required an authenticated caller before (no
  permission model exists for Postgres-side case mutations at all yet), so
  it resolves the actor **best-effort**: a missing/invalid `x-user-id-token`
  just means this update's activity entry is skipped, not a newly-rejected
  request. That branch's own `before` `*domain.CaseView` fetch (previously
  gated on `s.publisher != nil && req.State != nil`, only for
  `case.status_changed`'s sake) is now unconditional whenever `req.State` or
  `req.WorkState` is set, since the activity write needs the prior value
  regardless of whether Event Hub is configured at all.
- **Deliberately out of scope**: `updateCaseFields`'s combinable "plain
  field" bundle (Subject/Description/DeploymentID/DeployedProductID/fix-ETAs/
  RelatedCaseID/WorkaroundProvided) does not write to `work_item_activity`
  yet — up to nine fields in one call, several without a cheaply-available
  "old" value, is a larger and more speculative addition than the six
  branches above; left for a future change if it turns out to matter in
  practice the same way assign/state did.

Every helper above runs **synchronously** (not detached/async the way
`apps/csm-portal/backend`'s own `internal/handler/cases.go` `publishAsync`
is), each bounded by its own 5s `context.WithTimeout`
(`publishCaseCreatedTimeout`/`publishIncidentCreatedTimeout`/
`publishCommentAddedTimeout`/`publishStatusChangedTimeout`/
`publishSeverityChangedTimeout`) so a slow
ServiceNow or Event Hub round trip can't consume this service's own
request timeout — a deliberate simplicity trade-off over the async+
`WaitGroup`-drain pattern, made because this service (unlike that backend)
has no existing per-handler struct to hold a drain hook, and adding one
purely for this would be a larger change than the added latency (typically
well under a second) justifies. Revisit if that latency turns out to matter
in practice. Every helper's failure — enrichment or the publish call itself
— is logged (`slog.Error`/`slog.Warn`) and does **not** fail
`CreateCase`/`CreateCaseComment`/`UpdateCase`/`CreateIncident`'s own
response: the case/comment/incident already exists in ServiceNow by that
point, so a notification-side hiccup must not be reported to the caller as a
failed
create.

## Portal-driven membership writes

`CSM_MIGRATION_PORTAL_WRITES_ENABLED=true` (exactly `"true"`, off by default)
registers four write endpoints under the existing `/projects/{id}/contacts`
namespace. They are how **both** portals change who is a contact on a project:
the Customer Portal when a customer admin manages their own users, and the CSM
Portal when an account manager does it for them. Same endpoints, same
semantics, one implementation — `service.ProjectMembershipWriteService`
(`project_membership_write_service.go`), `handler.ProjectMembershipHandler`.

The CSM database is the source of truth. Salesforce is kept in step, not read
from as an authority. The Service Bus subscriber still feeds
Salesforce-originated changes into the ingest above, so a portal write and a
Salesforce edit converge on the same rows.

### The write ordering, and why the database commits last

Every one of the four follows the same shape:

1. Open the Postgres transaction. **Take a transaction-scoped advisory lock
   on the (project, address) pair**, then read the project and its account,
   and the membership that may already be there.
2. Write **Salesforce inside that transaction**, searching before every create.
3. Write the rows through the existing membership `Upsert`.
4. Commit.

The property this buys, agreed explicitly: **the database and Salesforce are
updated together or neither is, and the caller gets an error.** The reasoning
is one asymmetry — Salesforce has no transaction, so once its write returns it
is final; PostgreSQL does, and rolling it back costs nothing. So the only
participant that can be undone goes **last**. Anything that fails before the
commit leaves both systems exactly as they were.

`repository.ProjectMembershipRepository.UpsertWithin` is what holds the
transaction open across the Salesforce half: it takes a
`MembershipWritePlan`, a callback given the resolved
`MembershipWriteContext{Target, Existing}` and returning the
`SalesforceMembershipUpsert` to write. This deliberately **reuses
`upsertMembershipTx`**, the same seven-step resolve-and-write the Salesforce
ingest uses — there is exactly one writer of customer memberships into
Postgres, and it did not get a second copy.

Two details worth knowing:

- **The Salesforce half runs before the row write, not after**, even though
  the design reads "write the rows, then Salesforce". The rows need the
  Salesforce ids: `project_contact.sf_id`, `account_contact.sf_id` and
  `"user".sf_id` are stamped from the records this step creates or finds.
  What matters for correctness is unchanged — the commit is still last, and
  it is still the only thing that can be rolled back.
- **Every Salesforce create is preceded by a search.** The Sales Entity
  create endpoints are deliberately not idempotent, so searching first is
  what makes a retry, a hand edit made directly in Salesforce, and an
  orphaned record from a previous failure all get *adopted* rather than
  duplicated.
- **A contact the membership already links to is read by ID, not by
  address.** `project_contact.email` (the address the person was invited
  under) and the Salesforce Contact's own `Email` are different fields and do
  drift apart — `domain.UserContactAccess` compares them for exactly that
  reason. Resolving a known membership by address could therefore miss its
  Contact, and the search-first rule would then do the wrong thing very
  confidently: create a second Contact, create a second `Project_Contact__c`,
  and leave the real membership untouched while the role change or
  deactivation reported success. `writeSalesforce` calls `GetContact` with
  `MembershipWriteContext.Existing.ContactSfID` whenever it has one, and only
  falls back to the address search when that id does not resolve (so every
  self-healing path still works).
- **Concurrent writes for the same (project, address) are serialized**, by
  `pg_advisory_xact_lock(hashtextextended(projectId || '|' || email, 0))`
  taken as the transaction's first statement. Under READ COMMITTED the reads
  in step 1 see nothing of an uncommitted sibling, and those reads are what
  decide between "invite" and "change" — so a double-submitted invitation, or
  two admins inviting the same person at once, would both find no membership,
  both search Salesforce (both searches finishing before either create), and
  both create. Two Contacts, two `Project_Contact__c` records, two e-mails.
  The lock holds for the whole write *including* the Salesforce calls, which
  is the point: that is exactly the window. It is keyed per (project,
  address), so unrelated writes never queue behind each other — and the same
  address being invited to two different projects at once is still two
  Salesforce contact searches, which the search-first rule handles only if
  the first has committed. That narrow case is unchanged.

**The one residue.** A commit that fails *after* Salesforce succeeded leaves a
Salesforce record with no row behind it. The search-first rule makes that
self-healing — the next write for the same person adopts it — but it is also
recorded, so it can be retried rather than lost:
`repository.ErrMembershipCommitFailed` marks that specific case, and
`recordSalesforceOrphan` writes it to **`event_publish_failures`** with
`eventType = "salesforce.membership_write"`, the membership Salesforce id as
`entityId`, and a JSON payload of what should have happened (operation,
project, email, the ids, state, roles, whether the contact/membership was
created). That table was reused rather than a new one added: its shape already
fits exactly (a type, the id it is about, the JSON, the error, a `resolved_on`),
and it already has a repository, a service, a search endpoint and a resolve
endpoint — so the backlog is visible the day this ships instead of needing its
own API first. A dedicated `salesforce_write_failures` table would have been
the same five columns with a second copy of all of that around them. **Nothing
drains it automatically**; recording it is the whole of the commitment here.
The caller gets a 503 ("the change could not be saved; please try again"),
which is accurate: retrying is both safe and the right thing to do.

### The four endpoints

Authorized by `authorizeMembershipWrite`: trusted callers pass; a customer must be
REGISTERED on the project (else 404) and hold `customer_admin`/`partner_admin` (else 403), and their
own email replaces `inviterEmail`. No `M2MClientIDs` entry is needed for the portal.

`{id}` is the CSM project UUID, so these sit beside the search and get already
in that namespace. `{email}` keys the membership — the way the Customer Portal
addresses a contact today, and the only identifier a caller has before the
person exists in either system. `created_by`/`updated_by` is
`portal-membership-write` (`domain.PortalMembershipWriteActor`), distinct from
the ingest's `salesforce-sync`, and the DATABASE onboarding step records
`eventType = PORTAL_WRITE` stamped with the Salesforce record's own
`LastModifiedDate` so the echo of that very write is recognised as not newer
by the ingest's duplicate guard.

| Endpoint | Body | Success | Errors |
|---|---|---|---|
| `POST /projects/{id}/contacts` | `{email, firstName?, lastName?, roles: []}` | **201** + `ProjectMembership` | 400 bad address / unknown role / no roles, 403, 404 unknown project, 409 already an active contact or a missing Salesforce id, 503 |
| `POST /projects/{id}/contacts/validate` | `{email, inviterEmail?}` | **200** + `ProjectMembershipValidation` (`valid:false` + `reason` CONFLICT/FORBIDDEN/INVALID + `message` for a refusal; INVALID with the generic support message when a Salesforce id is missing) | 400 bad address, 403 not internal, 404, 503 |
| `PATCH /projects/{id}/contacts/{email}` | `{roles: []}` | **200** + `ProjectMembership` | 400, 403, 404, 409 missing Salesforce id, 503 |
| `DELETE /projects/{id}/contacts/{email}` | — | **204** | 400, 403, 404, 409 missing Salesforce id, 503 |
| `POST /projects/{id}/contacts/{email}/resend-invitation` | — | **204** | 400, 403, 404, 409 not INVITED or missing Salesforce id, **429** inside the cooldown, 503 |

- **Invite** resolves the project and its account, finds the Salesforce
  contact by address and creates it only if absent, finds the membership for
  (project, contact) and creates it only if absent (otherwise PATCHes its
  state and roles), writes the rows, commits, then publishes
  `project_contact.invited`. State is `INVITED`, or `RE-INVITED` when a
  previously DEACTIVATED membership is being brought back. An address that is
  already an **active** contact on the project is a 409 ("change their roles
  instead") — the adopt-don't-duplicate rule is about the *Salesforce* record,
  not about re-inviting somebody who is already there. At least one role is
  required: an invitation granting nothing would provision an identity that
  sees an empty portal.
- **Validate** is Invite's dry run: the same caller gate, the same "already
  an active contact" rule and the same `InvitationValidator`, run against the
  project and membership read outside any transaction
  (`ResolveWriteContext`, no advisory lock). Nothing is written and nothing
  is published. A refusal is a 200 with `valid:false` so the caller can tell
  it apart from a failed check; a valid answer carries the Salesforce contact
  the invitation would adopt, if any. The Customer Portal calls it before
  showing the invite form.
- **Change roles** replaces the portal-managed labels of the Salesforce
  `Role__c` picklist (Portal user, Security Contact, Lead, Admin) and, with
  them, the membership's project groups. `Role__c` is PATCHed as a whole list,
  so `mergePortalManagedRoles` keeps every other label already on the fetched
  Salesforce membership (Business Contact, the six D2 labels, anything
  unknown) after the requested ones; the groups are derived from that merged
  list, so a Business Contact keeps `Business Contact  Group`. When the fetched
  membership carries neither `roles` nor `role`, the write fails (503) rather
  than overwrite blind. A re-invitation PATCHes roles the same way. The state is
  untouched (the PATCH sends only `role`). An empty list is accepted and removes
  every portal-managed role.
- **Deactivate** sets `DEACTIVATED` in both systems. Never a delete:
  DEACTIVATED is a real value of both the Salesforce picklist and
  `project_contact_state_enum`, and it is what the portal does today. The
  roles are left exactly as they are — the derived admin role ignores
  deactivated memberships, so nothing has to be erased to drop it.
- **Resend invitation** writes nothing at all. It re-publishes
  `project_contact.invited` with `resend: true`, which is the one thing that
  makes csm-notification-service bypass its own already-sent guard (that guard
  is what stops a duplicate Salesforce event turning into a duplicate email,
  so a deliberate resend has to say so). Valid only while an invitation is
  outstanding — `INVITED` or `RE-INVITED`. REGISTERED means they already
  accepted and DEACTIVATED means they should not get one; RE-INVITED is not
  the product of a resend but the state **Invite** writes when it brings a
  deactivated contact back, so a person left in it by a failed notification
  has the same right to a retry as an `INVITED` one. A
  **five-minute cooldown per membership** is enforced from the EMAIL step's
  `updatedOn` in the onboarding ledger (the record of when an invitation was
  actually sent, written by csm-notification-service itself rather than
  guessed at here); inside it the call is a 429. No EMAIL step yet means none
  has been sent, so there is nothing to wait for. With no publisher
  configured this is a 503 rather than a silent success: unlike an
  invitation, whose database and Salesforce writes are the substance of the
  call, a resend **is** the event.

**A missing Salesforce id fails the write cleanly.** Every write needs the
project's `sf_id` and its account's `sf_id`; a write on a membership that is
already there (role change, deactivate, re-invite, resend) also needs that
membership's `project_contact.sf_id` and its contact's Salesforce id. If any
is NULL or blank, `requireSalesforceLinks` (resend: an inline check) refuses
the call with a **409** carrying a generic, per-operation message ("This
contact can't be updated right now. Please contact WSO2 support.") **before any
Salesforce call**, and logs the operation, the project and membership ids and
which ids were missing. The alternative was worse on every path: a NULL
`project.sf_id` failed the target read with a raw driver error (bare 500); a
blank one reached Salesforce and was refused there with an internal
validation message, sometimes after a Contact had been created; and a
membership with no `sf_id` was looked up by (project, contact) and, on a miss,
a second `Project_Contact__c` was created — for a deactivation, a new
DEACTIVATED record beside the real one. 409 rather than 400/404/503: the
request is well-formed, the rows exist, and retrying will not help until the
data is fixed; what blocks it is the rows' current state, and a
`ConflictError` message reaches the caller verbatim.

`apierror.TooManyRequestsError` was added for the cooldown (429 in
`writeServiceError`) — the first rate-limit this service applies, and a
deliberate caller-pacing decision rather than a downstream limit passed
through, so its message is returned to the caller.

`roles` on the wire are the raw Salesforce `Role__c` labels
(`Portal user` / `Security Contact` / `Lead` / `Admin`), matched
case-insensitively and normalised to Salesforce's own spelling before being
sent. A label Salesforce would reject is a **400 naming it**, not an
`ignoredRoles` log line: the ingest is right to tolerate an unknown role on a
record Salesforce already holds (refusing would strip a real membership over a
vocabulary gap), and wrong to tolerate one a caller is asking us to write.

### The Sales Entity write client (`internal/salesentity/membership_writes.go`)

`SearchContactByEmail` (`POST /contacts/search {email}`), `CreateContact`
(`POST /contacts`, 201), `SearchProjectContact`
(`POST /project-contacts/search {projectId, contactId}`),
`CreateProjectContact` (`POST /project-contacts`, 201),
`UpdateProjectContact`/`UpdateProjectContactState`/`UpdateProjectContactRoles`
(`PATCH /project-contacts/{id}` with `state` and/or `role`), and
`UpdateContactLockout` (`PATCH /contacts/{id}` `{lockoutStatus}`).

Two contract details that are easy to get wrong:

- **Absence is an answer, not an error.** The by-Id `GetContact`/
  `GetProjectContact` above map an empty result to a 503 (the Salesforce event
  can arrive before the record is visible). The two *search* methods here do
  the opposite and return `found = false` with no error — "no such contact" is
  exactly what tells the caller to create one. A row that comes back not
  actually carrying the address asked for is also `found = false`: a duplicate
  contact is recoverable, a membership written for the wrong person is not.
- **`PATCH /project-contacts/{id}` answers 200 with an EMPTY body** when its
  own re-read fails, even though the write itself succeeded. That is decoded
  as a zero `ProjectContact` and **no error** — treating it as a parse failure
  would roll back a transaction over a write that actually landed. The caller
  keeps the id it already held.

**Dependency**: the two create endpoints are being added to
`digiops-sales/sales-entity-service` in a parallel change (branch
`sales-entity-contact-create`) to exactly this contract. Until they are
deployed, `CSM_MIGRATION_PORTAL_WRITES_ENABLED` must stay off — which is why
the flag not registering the routes at all is the right default: a portal
built against them fails loudly with a 404 instead of writing one system and
not the other.

### Admin is a project role now, and the account-level role is derived

**The bug this fixes.** Salesforce's `Admin` role used to map straight to the
**global** `customer_admin`/`partner_admin` role on the user, with nothing
recorded per project — and `syncGlobalRoles` computed its revoke list as
`managedAdminRoles - wanted` from *the single membership being processed*. So
processing any one non-admin membership **revoked that user's admin
everywhere**. A customer admin on project A who was also an ordinary portal
user on project B lost their admin the moment B's membership was re-ingested.

**The agreed model.** Admin is stored per project, and the account-level role
is derived from it: admin on any project under an account means admin on every
project under that account, and nothing outside it.

- `project_role_enum` **already carried `ADMIN`** (migration 0028 declared
  all five values up front), so there was no enum to widen — the task brief
  expected one, and the schema had already done it. What was missing is the
  vocabulary a membership can attach to: a membership reaches its roles
  through `project_contact_group → project_group → project_group_role →
  project_role`, never `project_role` directly. **Migration 000084** seeds the
  `ADMIN` `project_role` row, an `Admin` `project_group`, and the link between
  them, idempotently (those rows are normally seeded by the ServiceNow sync,
  so this has to be safe against a database that already has them).
  `mapProjectGroups` maps Salesforce `Admin` → that group, alongside the
  existing PORTAL_USER / LEAD_USER / SECURITY_CONTACT mappings.
- `mapGlobalRoles` no longer decides admin at all. It returns `external` plus
  `customer`/`partner` (from the contact's account classification, decision
  D1), and separately reports **which** of the two admin roles this contact
  would hold (`SalesforceMembershipUpsert.AdminRoleName`: `partner_admin` for
  a Partner-classified account, else `customer_admin`) — never whether they
  hold it.
- `syncDerivedAdminRole` (step 8 of the upsert, after the project groups are
  written; also the Contact writer, and the membership DELETED path) decides
  that, as **one query over the user's memberships**:

  ```sql
  SELECT EXISTS (
    SELECT 1
    FROM "user" u
    JOIN account_contact ac ON LOWER(ac.user_name) = LOWER(u.user_name)
    JOIN project_contact pc ON pc.account_contact_id = ac.id
    JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
    JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
    JOIN project_role pr ON pr.id = pgr.project_role_id
    WHERE u.id = $1
      AND pr.role = 'ADMIN'::project_role_enum
      AND (pc.state IS NULL OR pc.state <> 'DEACTIVATED'::project_contact_state_enum))
  ```

  It binds only the user id — nothing about the membership in hand — which is
  precisely what makes the old failure impossible. Running it *after* step 7
  is what makes the membership being written count. A failed derivation aborts
  the write rather than quietly deciding "not an admin", which would revoke a
  real admin's role on a transient error.

  **Which role is the contact's, not the membership's.** The admin role (like
  `customer`/`partner`) follows the contact's account classification, so it
  no longer matters which membership is in hand: every writer passes the same
  `AdminRoleName` for the same contact, the user holds that one role when
  they are an admin, and the other is revoked. An earlier version split the
  answer by `ac.account_id <> p.account_id` per membership, which disagreed
  with ServiceNow for a partner's employee on the partner's own project and
  gave `partner_admin` to a related contact on another account's project;
  before that, a single "admin anywhere" answer combined with the role of the
  membership in hand let a non-admin partner membership trade one role for
  the other. The contact's own Salesforce `isCsAdmin` is an additional grant,
  never a revocation condition.

A deactivated membership's `ADMIN` role does not count, which is how
deactivating someone's last admin project drops their account-level role
without erasing anything. A membership DELETED in Salesforce re-derives the
role in the same transaction as the deactivation.

### Account roles on the contacts search

`POST /projects/{id}/contacts/search` lists only live memberships: DEACTIVATED
rows (the ingest's soft delete; ServiceNow hard-deleted them, so no portal ever
listed one) are left out of both the page and `total`. The change-request notice
recipients (`cr_notice_repo.go` `ProjectContactEmails`) and the SLA-status
customer-admin check (`project_stats_repo.go` `hasCustomerAdminContactExists`)
leave them out the same way. It returns each contact's
**`accountRoles`** alongside their project `roles` — a separate list, never
merged: `roles` is what they may do on *this* project, `accountRoles` what they
are across the account. It is read from `user_role`/`role` (where
`syncDerivedAdminRole` materialises the derived admin role) restricted to the
five names this write path owns — `external`, `customer`/`partner`,
`customer_admin`/`partner_admin` — so an internal role a staff account happens
to hold can never leak into a customer-facing contact list. Empty for a row
with no linked `"user"`, and always empty on the ServiceNow data source, which
has no notion of this role set. The point of it is the admin entry: both
portals can render an Admin badge on a contact list without a second call per
row.

## SLA status

**Replaces the old `sla_clocks` table entirely** (removed in migration
`000079`) — see that migration's own comment, and the history below, for
why. `GET /sla-status` (`internal/domain/entity.go`'s `SLAStatus`,
`internal/repository/sla_status_repo.go`, `internal/service/sla_status_service.go`)
now reads SLA state **live from the `sla` table** (migration `0048`), which
ServiceNow's own SLA engine populates via sync — real
`businessElapsedPercent`/`hasBreached`/`stage` per `(work_item, sla_policy)`,
not a value this service computes, schedules, or approximates itself. There
is no registration step, no duration policy to guess, and no
pause/resume/completion to track in-process any more: the synced row already
reflects all of that, pauses included.

**History, for context on the removal.** `sla_clocks` was a stand-in built
*before* the `sla` table existed in Postgres: it hand-registered a clock per
case at creation time (`sn_case_service.go`'s old `publishSLAClockRegister`),
using a hardcoded severity->duration guess
(`internal/service/sla_policy.go`'s old `slaDurations`, approximating WSO2's
own [support policy](https://wso2.com/licenses/support-policy/6.0)) rather
than ServiceNow's real SLA computation, and needed a matching amount of
in-process bookkeeping to stay roughly correct (`applyResponseSLAOnComment`
completing the `response` clock early on a qualifying comment,
`applyCaseStateSLAEffects` pausing/resuming/completing `workaround`/
`resolution` on state changes) — all of that is now redundant: the `sla`
table already reflects a support-engineer response, a case being on hold, or
a case closing, because ServiceNow's own SLA engine reacted to those same
events on its own side and the sync carried the result in. Removing this
also fixed a real, if minor, side effect: the old `TestSNCaseService_CreateCase_PublishesCaseCreated`
test only passed some of the time because `publishSLAClockRegister` was a
second, unrelated `Publish` call folded into `CreateCase`'s response path.

**`GET /sla-status` returns every currently-active clock across every
case-like work item in one paginated list** (`sla.is_active = TRUE`,
joined through `sla_policy.target` for `clockType` — `response`/
`workaround`/`resolution`, lower-cased from `RESPONSE`/`WORKAROUND`/
`RESOLUTION`), not one clock for one case. **Historical note: an earlier
design had `integrations/csm-notification-service` poll this endpoint
continuously and diff `businessElapsedPercent` against what it already
alerted on itself — that poll was abandoned** (that repo's own
`internal/slaengine/client.go` doc comment: a single page measured
6-34+ seconds against real production data, reliably tripping the gateway
timeout) **in favor of a Redis-based engine that tracks and alerts entirely
on its own**, reacting to `case.*` events rather than polling this endpoint
at all. This is a genuinely different shape from every other paginated
endpoint in this file regardless: it returns every active case's SLA data
in one bulk list with no per-project/per-case filtering, so it has its own
pagination cap (`normalizeSLAStatusPagination` — default `500`, max `2000`)
well above the generic `20`/`50` `normalizePagination` uses everywhere else.

**`source` (query param, `csm`/`servicenow`) narrows the result to one
`sla.source` value** — added so `csm-notification-service`'s Redis engine
could call this endpoint again for exactly one purpose: rebuilding its own
tracking state from Postgres if its Redis instance is ever wiped (a
one-shot reconciliation pass at process startup, not a recurring poll — see
that repo's own `CLAUDE.md`). `source=csm` scopes the query to just this
engine's own, much smaller row set (`WHERE s.source = 'CSM'`, injected into
`activeSLAStatusCTE`), so that reconciliation read never pays the cost of
scanning the full ServiceNow-synced table the old, abandoned poll design
choked on. Omitted (the default, and every other caller's behavior)
means no filter, identical to this endpoint's original, unscoped shape.

**`GET /sla-status` is internal-caller-only** (`slaStatusService.
requireInternalCaller`, mirroring `onboarding_step_service.go`'s own helper
of the same name/reasoning) — `AccessService.ResolveScope`'s scope must be
`Unrestricted` (an `M2MClientIDs` client, or `CSMPortalBackendClientID` with a
matching-domain caller), refused with
`ForbiddenError` otherwise. This is the one Postgres-backed read in this
file that genuinely has no narrower scope to fall back to instead: it
returns every active case's clock — case number, title, product, severity —
in one bulk list with no per-project/per-case filtering of its own, unlike
every other endpoint `AccessService` scopes by project membership. `auth.
Middleware` itself lets an unauthenticated request through by design (see
its own doc comment — enforcement is each endpoint's own job), so without
this check this endpoint would have handed out every active case's SLA data
to any caller able to reach the service at all, token or not — a real gap
this closed, not a hypothetical one.

**`sla` can carry more than one row per `(work_item, target)`** (a policy
reset re-applies the SLA — 662 of ~124,600 pairs, checked live), so the
repository picks the most recently started one per pair (`DISTINCT ON`,
falling back to most recently updated for the rare row with no `start_on`)
rather than an arbitrary one. `businessElapsedPercent` and `hasBreached` are
read straight off the row — `hasBreached` is not re-derived from the
percentage even though the two agree in every row checked so far
(`>= 100%` exactly where `hasBreached` is true): ServiceNow's own verdict is
what should be trusted if that ever changes. The eight display fields
(case number/WSO2 case id/title/type/product/team/priority/state) mirror
`GetCaseByID`'s own product/severity joins exactly, and — unlike the old
design's point-in-time registration snapshot — are read live alongside the
SLA data on every call, so they can't go stale between registration and a
breach firing days later. `team` now resolves via `account.cre_team_id`
joined to `"group"` (added alongside `ProjectOnboardingStatus`/
`IsEvaluationAccount` below — was previously always empty on this data
source, since nothing in this schema resolved a case to a team before this).

**`ProjectOnboardingStatus`/`IsEvaluationAccount` exist purely for
`csm-notification-service`'s own SLA breach-alert Chat-audience routing** —
the same team/onboarding/evaluation facts that service's own
`internal/chataudience.Resolve` uses to route its Chat alerts, per explicit
product direction: only SLA breach and (eventually) a
customer-frustration-detector alert route by real per-team/audience
resolution this way — `case.created`/`case.acknowledged`/
`case.severity_changed` all route to a single fixed
`chataudience.IncidentMonitor` audience instead, with no team detection at
all, and `incident.created` has no Chat reaction. `ProjectOnboardingStatus`
is `project.onboarding_status`'s raw enum label (e.g. `"IN_PROGRESS"`), `""`
when the work item has no project or the column is unset.
`IsEvaluationAccount` is true when the project's `project_type` matches the
fixed `evaluationSubscriptionProjectTypeName` ("Evaluation Subscription",
matched by name — see that constant's own doc comment for why not a
hardcoded id). Both are resolved via `LEFT JOIN`s added to
`activeSLAStatusFromJoins` (`account`/`"group"` for `team`,
`project`/`project_type` for the other two) — best-effort display/routing
enrichment, not part of the SLA clock itself; a work item with no
project/account simply reports the zero value for each.

**`AssigneeName`/`AssigneeEmail`/`TeamEmail`/`TeamLeadName` exist purely for
`csm-notification-service`'s own SLA breach-alert EMAIL reaction** (the
assignee/team-group emails sent alongside the existing Chat alert — see that
repo's own `CLAUDE.md`, "SLA breach-alerting engine") — not used by anything
in this service itself. `AssigneeEmail`/`AssigneeName` resolve via a
`LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id`; `TeamEmail` is
`"group".group_email` (the same `"group"` row `team` already joins through)
and `TeamLeadName` resolves via a second `LEFT JOIN "user" teamlead ON
teamlead.id = cre.manager_id` — the group's manager, not a dedicated "team
lead" column, since none exists on this schema today. All four are
best-effort, same posture as `ProjectOnboardingStatus`/`IsEvaluationAccount`
above: a work item with no assignee/group simply reports empty strings. A
dedicated team table may replace the `"group"` lookup for `TeamEmail`/
`TeamLeadName` later — noted, not yet needed.

### `GET /sla-duration-policy` — a separate, static duration table for `csm-notification-service`'s own native SLA tracking

`sla_duration_policy` (migration `0192`) is a small, static reference table —
13 rows, WSO2's own published [Enterprise Support
Policy](https://wso2.com/licenses/support-policy/6.0) durations, seeded
directly in the migration, never touched by any sync — deliberately
independent of both `sla`/`sla_policy` above (ServiceNow-shaped, no plain
severity column, empty for a case that never went through that sync) and
the CSM-native SLA clock engine's own `sla_policy_resolver.go` (reads the
real, synced `sla_policy` table). `ReferenceDataRepository.
ListSLADurationPolicy`/`GET /sla-duration-policy` (internal-caller-only,
same `requireInternalCaller` gate as `GET /sla-status`) exposes it —
`severity` already translated from the raw `case_severity_enum` label
("S0") to the same uppercase English word a `case.*` event's own `Priority`
field carries ("CATASTROPHIC"), via this package's own
`caseSeverityFromEnum` — so a consumer can match this response directly
against a `case.created` payload's `Priority` with no translation of its
own. `integrations/csm-notification-service` fetches the full set once at
startup to compute each case's own SLA due dates itself (see that repo's
own `CLAUDE.md`, "SLA breach-alerting engine") — entity-service's role here
is purely to trigger (publish the facts it already publishes, below) and
to hand over this one piece of static policy data; that service owns all
the actual duration bookkeeping, due-date arithmetic and alerting.

**`events.CommentAddedPayload.IsSupportEngineerResponse`** is the other
half of enabling that: true when a comment is a public comment
(`req.Type == domain.CommentTypeComment`) authored by a user holding
`CSEngineerRole` — computed once per comment and shared by both the
CSM-native SLA engine's own response-clock completion and this new payload
flag, via `case_service.go`'s `isSupportEngineerAuthor` (Postgres,
`UserRepository.GetUserRoles`) and `sn_case_service.go`'s
`isSupportEngineerAuthorSN` (ServiceNow, `SNUserService.SearchUsers`) —
see `CS_ENGINEER_ROLE`'s own `.env.example` doc comment. This lets
`csm-notification-service` complete a case's own response clock the moment
a qualifying reply lands, with no role/identity resolution of its own —
the one signal entity-service is uniquely positioned to compute, since it
owns the role data. `case.created`/`case.status_changed` already carry
everything else that service's own tracking needs (`Priority`/`CreatedAt`,
`NewStatus`), so neither payload needed any change for this.

## CSM-native SLA clock engine

`internal/service/sla_engine_service.go` (`SLAEngineService`) is what actually
keeps the `sla` table populated for a case-like work item this deployment
creates itself — real, durable `source='CSM'` rows, not a value ServiceNow's
own sync computes. This exists specifically for the dual-write pilot (and,
in principle, any future pure-Postgres mode): a case created there has no
ServiceNow-synced `sla` row of its own to read `GET /sla-status` from, so
without this engine it would simply never get SLA tracking at all,
regardless of severity.

- **Durations come from deterministic, severity-keyed `sla_policy` rows this
  engine seeds itself** (`internal/service/sla_policy_resolver.go`,
  migration `0203_csm_sla_policy_by_severity.sql`) — not the real,
  ServiceNow-synced `sla_policy` rows, and not a hardcoded map (the
  now-deleted `sla_clocks` design's old approach — see "SLA status" above for
  that history). An earlier version of this resolver looked up the real
  synced rows by a guessed exact name (`"<P0-P3|Query> - <Response|
  Workaround|Resolution> (<Managed Services|Open Source>)"`, with the "plan"
  half itself guessed from a case's project subscription type), falling
  back to a loose pattern match when that guess missed — **confirmed, against
  real staging data, to silently find nothing for every LOW-severity case**:
  the real synced rows for "Query" (LOW) are named things like `QuerySLA` and
  `Onboarding Case Customer Query Response`, which never matched that assumed
  naming convention under either plan label or the pattern fallback, so
  `RegisterCaseClocks` registered nothing at all for any LOW-severity case —
  this is why a real SLA breach alert (fired correctly by
  `csm-notification-service`'s own Redis engine) never showed up in the CSM
  Portal UI (`GET /sla-status` reads this table) for a LOW/S4 case. Migration
  0203 seeds one `source='CSM'` row per `(severity, clock_type)` pair
  `sla_duration_policy` (migration 0192) already defines, named
  deterministically (`"<severity> - <target> (CSM)"`, e.g.
  `"S4 - RESPONSE (CSM)"`) and with durations copied straight from that same
  table — `resolve` now does a single exact-name lookup keyed on severity
  alone, which can never miss, and the two SLA engines (this one, and
  `csm-notification-service`'s own Redis-based tracker, which has always read
  `sla_duration_policy` directly) can never disagree on a duration. This
  supersedes migration `0136_csm_p0_sla_policies.sql`'s narrower precedent
  (CSM rows for P0/Catastrophic only, under the old naming convention) —
  0136's own rows are left in place (harmless, unreferenced) rather than
  dropped.
- **`internal/repository/sla_engine_repo.go`'s `RecomputeActive`** computes
  the same flat wall-clock formula it always has (`(NOW() - start_on) /
  duration * 100`, no business-hours calendar, same crudeness the old
  deleted design had) — but it is no longer run by a periodic background
  worker. `SLAEngineRecomputeWorker` (which used to call it every 45s over
  every active `source='CSM'` row — a continuous Postgres write with no
  bearing on alerting, since `csm-notification-service`'s Redis engine fires
  breach alerts independently of this table) has been deleted. The `sla_live`
  view (migration `0204_sla_live_view.sql`) reproduces the identical formula
  live, at read time, instead: every reader of
  `business_elapsed_percentage`/`has_breached`/`business_duration`/
  `remaining_business_duration`/`stage` (`GET /sla-status`,
  `POST /task-slas/search`, `GET /task-slas/{id}`, the case/incident
  SLA-breach search filters) now reads `sla_live`'s `live_*` columns instead
  of `sla`'s own stored ones for an `IN_PROGRESS`/`BREACHED` row; a
  `PAUSED`/`COMPLETED`/`ACHIEVED`/`CANCELLED` row is already frozen at
  whatever `SetPaused`/`CompleteClock` last wrote and the view simply passes
  those columns through unchanged. `RecomputeActive` itself is untouched and
  still exported on `SLAEngineRepository` — currently unreferenced by any
  production code path, kept rather than deleted in case a future admin
  "force recompute" tool ever wants it.
- **A BREACHED clock is not terminal — it keeps being recomputed, and stays
  completable, until its own genuine finishing event.** A real, reported bug
  had `RecomputeActive` stop touching a row the instant it first flipped to
  `BREACHED` (its `WHERE` clause only ever matched `IN_PROGRESS`), freezing
  `business_elapsed_percentage`/`business_duration` forever at whatever the
  breaching tick happened to compute — e.g. a response SLA observed stuck at
  "59m" elapsed long after real time had moved well past that, because
  nothing ever recomputed it again. `CompleteClock`/`SetPaused` had the
  matching half of the same bug: both excluded `BREACHED` via
  `slaEngineActiveStageFilter`, so a RESPONSE clock that breached before a
  support engineer ever replied silently ignored that reply's own
  `CompleteClock` call — matching zero rows instead of finally finalizing
  it. Fixed by giving `CompleteClock`/`SetPaused` their own, narrower
  `slaEngineOpenStageFilter` (excludes only `ACHIEVED`/`CANCELLED`/
  `COMPLETED` — the stages a clock genuinely never leaves — not `BREACHED`
  too), and widening `RecomputeActive`'s own `WHERE` to `stage IN
  ('IN_PROGRESS', 'BREACHED')` (still excluding `PAUSED`, for the same
  "pause must actually stop accumulation" reason it always did).
  `business_elapsed_percentage` is no longer capped at 100 either, in both
  `RecomputeActive` and `CompleteClock` — a clock now shows its real,
  uncapped overrun (e.g. 134%) for as long as it stays unanswered/BREACHED,
  and still shows that same true number once it's finally completed,
  instead of an identical-looking 100% regardless of how late the real
  completion actually was. The webapp's `CaseSlaTable` already handles a
  percentage above 100 correctly (clamps the progress-bar fill, shows the
  real number as text), confirmed before uncapping this.
- **A case closing now finalizes all three clock types, not just
  resolution.** `ApplyCaseStateEffects`'s `CaseStateClosed` branch used to
  only resume+complete `resolution` and merely pause `workaround` forever
  (a documented, carried-forward gap from the old deleted design) — and
  never touched `response` at all, leaving a case closed before anyone ever
  replied with its response clock permanently `IN_PROGRESS`/`BREACHED`.
  Closing now resumes+completes `workaround` the same way `resolution`
  already was, and completes `response` directly (it's never paused at any
  state) — all three unconditionally, every close, since `CompleteClock`'s
  own stage filter is a no-op for whichever clock(s) already reached a
  genuine completion (an engineer's reply, an earlier close) before this
  ran.
- **`RegisterCaseClocks`/`ReviseCaseClocks`/`CompleteResponseClock`/
  `ApplyCaseStateEffects`** are all called directly, in-process, from
  `snCaseService`'s own case-lifecycle hooks (create/severity-change/
  qualifying-comment/state-change) — best-effort, same "must never fail the
  actual mutation" reasoning `publishCaseCreatedEvent` follows.
- **A real, fixed bug: SLA clock registration must run only after this
  case's Postgres `work_item` row actually exists.** `sla.work_item_id` has
  a hard, non-deferrable foreign key on `work_item(id)` (migration `0048`).
  `snCaseService.CreateCase` calls `registerCaseSLAClocks` right after its
  own ServiceNow POST succeeds — fine for plain `DATA_SOURCE=servicenow`,
  since that path has no Postgres insert of its own to wait for at all. But
  in dual-write mode, `caseService.createCaseSNFirst` reaches
  `snCaseService.CreateCase` *as its mirror*, calling it **before**
  `caseService`'s own `CreateCaseFromServiceNow` insert — so every
  dual-write case creation tried to insert an `sla` row referencing a
  `work_item_id` that didn't exist yet, failing with a foreign-key violation
  (`SQLSTATE 23503`), logged (`"sla engine: register clock failed"`) and
  silently dropped (best-effort, never retried, never surfaced) — meaning no
  SLA breach alert ever fired for any dual-write-created case. Fixed the
  same way this exact problem was already solved for the `case.created`
  publish (see `publishCaseCreatedEvent`'s own doc comment): `routes.go`
  constructs `snCaseMirrorSvc` with a **nil `slaEngine`**, so its own
  embedded registration call is a no-op when reached via the mirror path
  (mirroring its existing nil `publisher`), and `caseService` registers the
  clocks itself — `registerCaseSLAClocksEvent` (`sn_case_service.go`,
  factored out the same way `publishCaseCreatedEvent` was) — right after its
  own insert succeeds. Any future change that gives `snCaseMirrorSvc` a
  non-nil `slaEngine` again, or that adds another consumer of
  `snCaseService.CreateCase` as a mirror/pass-through, needs the same
  "does the referenced Postgres row exist yet at this call site"
  check — see `TestCaseService_CreateCase_RegistersSLAClocksOnlyAfterPostgresSucceeds`
  for the regression test.
- **The same "wired into `snCaseService` only" gap existed for every other
  `SLAEngineService` hook too, not just registration — all now fixed in
  `caseService` directly.** `snCaseService.applyResponseSLAOnComment`/
  `applyCaseStateSLAEffects`/`reviseCaseSLAClocks` are the plain-ServiceNow-
  mode's own hooks; `caseService.createCaseCommentAs`/`UpdateCase` (the
  active, Postgres-primary path in dual-write mode) had no equivalents at
  all — a support engineer's reply never completed the response clock, no
  state transition ever paused/resumed/completed workaround or resolution,
  and no severity change ever revised a case's clocks, for any dual-write
  case, full stop. Each is now its own hook on `caseService`:
  - **`completeResponseSLAOnComment`** (`createCaseCommentAs`) — gated on
    the same `CSEngineerRole` (`CS_ENGINEER_ROLE`) config
    `snCaseService.applyResponseSLAOnComment` already uses, shared rather
    than duplicated: "CS engineer" and "support engineer" are the same
    real-world role, just checked against a different role vocabulary here
    (`repository.UserRepository.GetUserRoles`, Postgres' own `user_role`
    table) than `snCaseService`'s own lookup. Resolves the
    comment author via `userRepo.GetUserByEmail` then `GetUserRoles` —
    tolerates both failing (the M2M `CreateCaseCommentAs` path has no
    guaranteed user row, per that method's own doc comment) by skipping,
    the same "can't confirm, skip" posture `CSEngineerRole` itself uses
    when unconfigured.
  - **`ApplyCaseStateEffects`** (`UpdateCase`) — fires unconditionally
    whenever `req.State != nil`, not gated on a genuine change, matching
    `snCaseService`'s own call site exactly: every effect it applies is
    idempotent, so a no-op re-PATCH just harmlessly re-applies the same
    effect.
  - **`ReviseCaseClocks`** (`UpdateCase`) — gated on a genuine severity
    change (unlike the state hook above), reusing the same `GetCaseByID`
    fetch the `case.severity_changed` publish already does, rather than a
    second round trip.
- **The workaround clock could never actually complete, on either code
  path, until now — a separate, previously-accepted gap this also
  closes.** `ApplyCaseStateEffects` only ever pauses/resumes the workaround
  clock (even on close, per its own doc comment — there was no "workaround
  provided" signal wired into it). `WorkaroundProvided` (a real field on
  `UpdateCaseRequest`/`CaseView`, the "Provide Workaround" action) is the
  one genuine such signal that exists anywhere in the domain model, and
  it simply wasn't connected to the SLA engine at all. `SLAEngineService`
  now has its own `CompleteWorkaroundClock`, called from both `caseService.
  updateCaseFields` and `snCaseService.UpdateCase` whenever
  `WorkaroundProvided` is set to `true` — `false` (a recall) deliberately
  does **not** reopen a completed clock; `SLAEngineRepository` has no
  "uncomplete" operation, and a recall is rare enough that this stays a
  known, accepted gap rather than something built speculatively.
- **Sharing a fix ETA with the customer completes BOTH the workaround and
  resolution clocks, not just one.** The webapp's "Share fix ETA with
  customer" action (`SetFixEtaDialog.tsx`, ServiceNow-only on the wire —
  `req.AddPublicComment` alongside a fix-ETA date) once WSO2 has committed a
  fix timeline to the customer, neither clock has anything further to
  track. **The trigger is `work_item.eta_shared_on` becoming non-null, not
  `req.AddPublicComment` itself** — that request flag has no guaranteed
  connection to *when* the ETA-share actually lands in Postgres (whatever
  external process populates `eta_shared_on` does so on its own schedule,
  not synchronously with this one PATCH), so `CaseService.GetCaseEtaSharedOn`
  (`domain.CaseView.EtaSharedOn`, sourced from Postgres on **every** data
  source — see that field's own doc comment for why ServiceNow has no
  equivalent column at all) is checked on every `UpdateCase` call instead,
  on both `caseService` (Postgres) and `snCaseService` (ServiceNow, via
  `pgFallback` when configured, else always nil/no-op). Cheap and
  idempotent, same as every other completion check here: a case's clocks
  get completed the next time anything about it changes, once the shared
  fact is persisted, not strictly on the PATCH that shared it.
  `SLAEngineService.CompleteFixEtaSharedClocks` calls `CompleteClock` for
  both targets, same real, uncapped elapsed-time-at-this-moment semantics
  every other completion path uses (see `SLAEngineRepository.CompleteClock`'s
  own doc comment) — not an unconditional 100%. Independent of
  `WorkaroundProvided`'s own hook just above: a caller can set both signals
  at once, in which case `CompleteWorkaroundClock` simply becomes a no-op
  for whichever of the two runs second. Postgres's own `GetCaseByID` now
  also selects `best_case_eta`/`most_likely_eta`/`worst_case_eta`/
  `eta_shared_on` for the first time — previously write-only columns on
  this data source, never read back into a `CaseView` at all.

## Customer-reply state transition

Unrelated to SLA tracking above, but lives in the same file and used to be
documented alongside it: `sn_case_service.go`'s `CreateCaseComment` calls
`applyCustomerReplyStateTransition` for every comment. When a
customer-visible comment arrives while the case is `Awaiting Info`/
`Solution Proposed`, from an author holding one of the configurable
`CUSTOMER_ROLES` env var (comma-separated ServiceNow role names,
deliberately no committed default — organisation-specific vocabulary, same
reasoning `apps/csm-portal/backend`'s own `CSM_TEAM_REGISTRY` uses; resolved
the same way as every other author-role check in this file, since this
service has no auth/identity layer of its own and the `x-user-id-token` it
forwards is opaque), this calls `s.UpdateCase` with `State: WaitingOnWSO2`
**in-process**, not a second, separate ServiceNow PATCH — reusing
`UpdateCase`'s own `publishStatusChanged` call rather than duplicating it.
Requires its own `GetCaseByID` call to read the case's current state —
nothing else in `CreateCaseComment`'s flow surfaces it (`publishCommentAdded`
fetches one for its own purpose but never shares it, and is itself skipped
when `s.publisher` is nil).

**KNOWN GAP**: the read (this function's own `GetCaseByID`) and the write
(`UpdateCase`'s PATCH) are not atomic — a case moved to some other state
(e.g. closed) in that window still gets unconditionally set back to
`Waiting on WSO2`. Not unique to this function: every `UpdateCase` caller
that sets `State`/`Severity`/`AssigneeEmail` has the same read-then-PATCH
race, since ServiceNow is the sole source of truth (no local row/version)
and the Choreo integration's PATCH has no conditional-update mechanism
(ETag/version/`sys_mod_count`) to close it with. Fixing this needs that
integration to expose one first — a cross-team dependency, not addressed
here.

## Scheduled task runs

`scheduled_task_run` (migration `0108`, `internal/domain/entity.go`'s
`ScheduledTaskRun`, `internal/repository/scheduled_task_run_repo.go`,
`internal/service/scheduled_task_run_service.go`) is durable claim/retry
state for `operations/csm-scheduled-tasks` — a single Choreo Scheduled Task
that internally fans out to any number of independently-scheduled sub-crons
on one shared driver cadence. Like `event_publish_failures`, it has no
ServiceNow equivalent and is always backed by Postgres regardless of
`DATA_SOURCE`.

`taskName` is a caller-defined registry key, not a fixed enum: which
sub-crons exist, and on what schedule, is a policy decision made entirely by
`operations/csm-scheduled-tasks`' own registry, not something this service
tracks.

There is no stored status column: status is always derivable from which
timestamp is set, and each is independently useful on its own —
`succeededOn` (done, forever, for this period), `supersededOn` (abandoned:
the next period came due before this one ever succeeded), or `nextRetryOn`
(eligible for another attempt once it's in the past). See
`operations/csm-scheduled-tasks`'s own `CLAUDE.md` for the full design
behind "period keys" and "supersede" — this service only stores the result
of that design, it does not compute period keys or decide backoff itself.

Exposed at:

- `POST /scheduled-tasks/attempts` — the only endpoint with real decision
  logic. Named as a collection-create (like GitHub's `.../dispatches` or
  `.../deployments`), not a verb-suffixed action path — POST creates a new
  "attempt" resource in the `attempts` collection. Atomically claims
  `taskName`/`periodKey` if it's allowed to run right now: a period this
  task hasn't seen before first supersedes any other still-open row for the
  same `taskName` (there is at most one by construction), then inserts and
  claims fresh; an existing row whose `nextRetryOn` has arrived (or that
  looks like an orphaned claim — see `staleClaimAfterSeconds`) is bumped
  and claimed; anything else (already succeeded, already superseded, not
  yet due, genuinely still claimed by a live attempt) is denied. Concurrent
  callers racing for the same `taskName` — whether the exact same
  `periodKey` or two different ones — are serialized by a
  transaction-scoped Postgres advisory lock keyed on `taskName`
  (`pg_advisory_xact_lock(hashtext(taskName))`), not just the table's own
  `UNIQUE(task_name, period_key)` constraint: that constraint alone only
  stops two claims from colliding on the *same* period, not two concurrent
  claims for two different *new* periods of the same task, which would
  otherwise both find no existing row and both insert successfully —
  leaving two open rows for one task at once. The lock closes that window;
  at most one caller can ever see `allowed: true` for a given `taskName` at
  a time, regardless of which period it's for.
- `PATCH /scheduled-tasks/attempts/{id}` — reports an attempt's outcome,
  `{attemptCount, status: "succeeded"|"failed", error?, nextRetryOn?}` (the
  latter two required only when `status` is `"failed"`). One endpoint, not
  two separate action-style ones (an earlier version had `POST .../complete`
  and `POST .../fail`) — PATCH is the correct verb for a partial update to
  an existing resource's state, and "which outcome" is naturally the
  request body's job, not the URL's. Rejects the update (404) unless the
  caller's `attemptCount` still matches the active claim (the value
  `Attempt` returned) — a worker that stalls past `staleClaimAfterSeconds`
  and gets reclaimed by a different caller later finds its own stale report
  rejected instead of silently overwriting whatever the reclaiming caller's
  own attempt has since done. On `"failed"`, deliberately does not mark the
  row succeeded or superseded, so it stays eligible for another attempt, or
  for being superseded once the next period's own `Attempt` call comes in.
- `GET /scheduled-tasks/attempts?status=<failed|succeeded|superseded>` —
  monitoring only, not called by the engine's own claim/retry logic. Plain
  unpaginated list. `status=failed` stays small by construction (at most
  one open row per `taskName`), and `status=succeeded`/`superseded` now
  stays bounded too, as long as `operations/csm-scheduled-tasks`' own
  `housekeeping_cleanup` sub-cron (below) keeps running — that result set
  has no cap of its own, it's only ever kept small by that cleanup actually
  happening; don't assume it's small in a deployment where it isn't.
- `DELETE /scheduled-tasks/attempts?resolvedBefore=<RFC3339 timestamp>` — deletes
  every row that succeeded or was superseded before the cutoff, by its own
  `succeededOn`/`supersededOn` (not `createdOn` — a row open for 89 days
  before finally resolving on day 90 gets the same retention window as one
  resolved on day one, not an immediate deletion because it happens to look
  old by creation time). A row still `failed` is never deleted regardless
  of age — it represents a genuinely unresolved problem, not history to
  archive. Called daily by `operations/csm-scheduled-tasks`' own
  self-hosted `housekeeping_cleanup` sub-cron (`internal/housekeeping`
  there) — that endpoint existed from the start, but this is the first
  thing that actually calls it.

## Comment, product vulnerability, and time-card Postgres support

`comment` (migration 0040), `product_vulnerability` (migration 0038),
and `time_card`/`time_card_approver` (migration 0041) had tables from the
start but no repository/service ever queried them — every route backed by
these entities (`/comments*`, `/products/vulnerabilities/*`, `/time-cards/*`,
`/cases/time-cards/search`) was ServiceNow-only regardless of
`cfg.DataSource`. `comment_repo.go`/`comment_service.go`,
`product_vulnerability_repo.go`/`product_vulnerability_service.go`, and
`time_card_repo.go`/`time_card_service.go` wire up a Postgres-backed
implementation for each, following the same `routes.go` "SN branch vs.
Postgres branch, same service interface" pattern `caseRepo`/`projectRepo`
already use — no route path, request, or response shape changed.

- **Comments**: `comment.work_item_id` is a foreign key into `work_item(id)`,
  so only reference types that are themselves work_item subtypes can be
  commented on through Postgres — see
  `repository.ReferenceTypeToWorkItemType`. **`"case"` maps to all five
  case-like work_item types** (`CASE`/`ENGAGEMENT`/`SERVICE_REQUEST`/
  `SECURITY_REPORT_ANALYSIS`/`ANNOUNCEMENT`), not just literal `CASE` — found
  live as a real bug via a HAR comparison against the ServiceNow data
  source: a `CS`-numbered work_item whose real type was `SERVICE_REQUEST`
  returned zero comments through `POST /comments/search` (which
  `csm-portal-backend`'s case-scoped comment endpoint forwards to, injecting
  `referenceType:"case"`) even though it had real comment rows, because this
  map used to bind a single `"CASE"` value where `case_repo.go`'s own
  `GetCaseByID`/`SearchCases` have matched all five case-like types since
  "Case-like work_item types" landed — this file was simply never updated to
  match. `"deployment"` has no entry:
  `deployment` (migration 0018) is its own standalone table with its own
  primary key space, not a work_item subtype, so `CreateComment`/
  `SearchComments` reject it with a `ValidationError` before any query runs.
  `CreateComment` also refuses to write `CommentTypeActivity`
  (`comment_type_enum`'s `APPROVAL_HISTORY` label is reserved for
  ServiceNow's own audit trail, never a caller-authored comment) but still
  accepts it as a search filter, for reading rows a future SN-sourced ETL
  might load. `comment.created_by` is a free-text `VARCHAR`, not a foreign
  key into `"user"` (it mirrors ServiceNow's `sys_journal_field` author
  string, which can be a non-user integration account) — the Postgres path
  writes the caller's resolved email into it, the same identity mechanism
  `caseService.CreateCaseComment` uses (`x-user-id-token` → `emailFromJWT` →
  `UserRepository.GetUserByEmail`). **`SearchComments` now also resolves a
  display name for that email**, via the same `LEFT JOIN "user" ON
  LOWER(email) = LOWER(created_by)` (wrapped in its own `DISTINCT ON`
  subquery — email has no unique constraint) that `SearchCaseActivities`'s
  own comment branch already used — found live as a real, visible bug: a
  case's comment bubbles showed the commenter's raw email while that same
  case's Lifecycle/Attachment entries, on the sibling `/activities/search`
  endpoint, already showed a resolved name for the identical author, because
  only that second endpoint ever did the join. `CommentRow.CreatedByName`
  (`comment_repo.go`) is `""` for an address with no matching `"user"` row
  (an integration/automation account like `github_pipeline` — a real,
  legitimate case, not an error) — `commentRowToDomain` passes it through as
  the `UserReference.Name`, and the webapp's own `authorDisplayName` already
  falls back to the email whenever `Name` is empty, so this needed no
  webapp change at all, only entity-service. `CreateComment`'s own
  echoed-back response (`CaseCommentDetail.CreatedBy`) is a plain email
  string with no name field on its wire contract at all — deliberately left
  as-is; the webapp only reads a comment's display name from `SearchComments`
  once the list is (re)fetched, never from the create response.
- **Product vulnerabilities**: `SearchProductVulnerabilities`/
  `GetProductVulnerability`/`GetVulnerabilityMeta` are read-only queries
  against `product_vulnerability`, which mirrors ServiceNow's own
  vulnerability record 1:1 and is deliberately standalone (no FK into
  `product`/`product_version` — see that migration's own doc comment).
  `GetVulnerabilityMeta` reads `product_vulnerability_severity_enum`'s
  labels straight from Postgres's own enum catalog
  (`pg_enum`/`ListSeverities`) rather than hardcoding them, so it can never
  drift from the migration that defines the type.
  `SyncProductVulnerabilities` has **no Postgres equivalent** and always
  returns a `ServiceUnavailableError` on that data source: its full-replace
  semantics (delete anything absent from the submitted set, upsert
  everything present) need a stable external join key with a
  database-enforced uniqueness guarantee, and `product_vulnerability` has no
  `UNIQUE` constraint on any column other than its own generated `id` —
  adding one is a schema change, out of scope for wiring up the existing
  table's read queries.
- **Time cards**: like comments, the Postgres path has no inbound-auth
  layer to forward a caller's identity through, so `CreateTimeCard`/
  `UpdateTimeCard`/`DeleteTimeCard` resolve the caller's user id from
  `x-user-id-token` the same way `caseService.CreateCaseComment` does,
  rather than trusting a submitter id in the request body. `UpdateTimeCard`
  enforces "only editable while `submitted`" and `DeleteTimeCard` enforces
  "only the submitter, only while `submitted`" itself, in the repository's
  `WHERE` clause (`state = 'submitted'` / `user_id = $2 AND state =
  'submitted'`) — the ServiceNow-backed implementation instead trusts SN to
  enforce both, since it just forwards the caller's token.
  `TransitionTimeCardState` (approve/reject) similarly requires the actor to
  be an eligible approver (a `time_card_approver` row for this specific
  card) **or** a holder of the global `admin` role (`role.name = 'admin'`,
  the same role `recompute_user_type` — migration 0011 — already treats as
  a distinct global grant), and in either case not the card's own submitter,
  AND the card to currently be `submitted` — all checked under one
  `SELECT ... FOR UPDATE` so a concurrent approver-list/role edit or a
  second transition attempt can't slip through between the check and the
  write. The `admin` branch is a deliberate "approve by exception" escape
  hatch, added at explicit product request: unlike every other eligibility
  check in this file, it is not scoped to any particular card at all —
  holding `admin` lets a caller decide *any* submitted card, regardless of
  whether `time_card_approver` lists them for it. Self-approval is still
  blocked unconditionally, admin included. The CSM Portal webapp's own
  `useTimecardRole` hook mirrors this exactly (`isApprover || isAdmin`) for
  which cards it shows Approve/Reject controls on — see that repo's own
  `CLAUDE.md`. That guard only fires at decide-time, though — until now nothing
  stopped the same submitter/approver pairing from being written in the
  first place. `validateApproverIDsExcludeSubmitter` (`time_card_service.go`)
  closes that at create/edit time instead: `CreateTimeCard`/`UpdateTimeCard`
  both reject a request naming the submitting user among `ApproverIDs`,
  compared against `submitterID` directly rather than re-reading the card's
  own stored `user_id` (cheap and correct for both call sites — see that
  function's own doc comment). The webapp's own approver picker
  (`LogTimeCardDialog.tsx`) already filters itself out of both the live
  search and the "Recently selected" list for the same reason, but that was
  always a UI convenience only, never an enforced rule — a direct API call
  (or a stale client) could still persist it before this. `CreateTimeCard`
  validates a supplied `projectId` against the
  case's own `work_item.project_id` (`case.id` and `work_item.id` are the
  same value) rather than trusting an unrelated existing project id;
  omitting it leaves `customer_project_id` `NULL`, unchanged from before
  this check existed. Approvers (`time_card_approver`) are replaced
  wholesale, never diffed, whenever `ApproverIDs` is provided on an edit.
  `SearchCaseTimeCards`' rollup (`CaseTimeCardSummary`) is computed with
  `GROUP BY`/`SUM`/`COUNT FILTER` in one query per page, not aggregated in
  Go — its returned project comes from the case's own
  `work_item.project_id`, not any individual time card's
  `customer_project_id`, so one case can never fragment into multiple
  summary rows.
  `SearchTimeCards`/`SearchCaseTimeCards` require a valid `x-user-id-token`
  (the same minimum bar as every write here) but do not yet scope results
  to what the caller specifically owns, approves, or manages — there is no
  authorization model to build that against today. `callerEmail` is
  threaded to the repository layer for that future decision, unused for
  filtering, the same deliberate posture as `AccountContactRepository`/
  `ProjectContactRepository`'s own `callerEmail` parameter below.

## Case tags, case watch list, account/project contacts, and user roles

A second round of wiring previously-ServiceNow-only routes up to Postgres,
following the same "SN branch vs. Postgres branch, same service interface"
pattern as the section above — no route path, request, or response shape
changed.

- **Case tags** (`tag`/`work_item_tag`, migration 0026): `CaseService.
  AddCaseTag`/`RemoveCaseTag`/`SearchTags` in `case_service.go` were a
  detection-only stub that always returned 503 — now actually persist.
  `AddCaseTag` finds-or-creates a tag by name (case-insensitively;
  `tag.name` has no `UNIQUE` constraint, so a race between two first-uses
  of the same never-before-seen label can produce a cosmetic duplicate row,
  not a correctness bug) and attaches it to the case's underlying
  `work_item`, idempotently. A `"patch"` label on a case currently at
  LOW/S4 severity also flips the case's time cards non-billable, inside the
  same transaction as the attach — see "Event Hub publishing" above
  (`recomputeTimeCardsBillable`) for the full design and the race it fixes.
- **Case watch list** (`work_item_watcher`, migration 0042):
  `UpdateCase`'s `WatchList` field, previously rejected outright on this
  data source, now has its own branch (`updateCaseWatchList`) — split out
  with an early return specifically so it can't disturb the pre-existing
  `state`/`severity`/`workState` branch (including its billable-status side
  effect). Mutually exclusive with `State`/`Severity`/`WorkState` per
  request, same as ServiceNow — and, same as ServiceNow, with every other
  `UpdateCaseRequest` field that's ServiceNow-only regardless of `WatchList`
  (`AssigneeEmail`, `EngagementPaymentType`, `IssueType`, `ResolutionCode`,
  `Cause`, `CloseNotes`, `AddPublicComment`, `Product`, `PublicTicket`,
  `Acknowledge`, `WorkaroundProvided`, and the rest of the existing
  unconditional rejection list) — a caller can no longer combine, say,
  `resolutionCode` with a Postgres `UpdateCase` call and have it silently
  ignored. `GetCaseByID` also now populates `WatchList` via the same
  `fetchCaseWatchers` helper `SetCaseWatchList` uses to read back its own
  result; both build each `WatchListUser.User` with an empty id
  (`domain.NewUserReference("", ...)`), never the watcher's own resolved id
  — `WatchListUser.User`'s own doc comment requires that field to stay null
  regardless of whether this data source happens to know it.

  **A new case always gets no watchers at all on the SN-first create
  path.** `CreateCaseFromServiceNow`'s insert only ever writes `work_item`/
  the type-specific extension table — never `work_item_watcher` — so every
  Postgres-sourced read of a freshly created case (`GetCaseByID`,
  `SearchCases`) showed no watchers, and `publishCaseCreatedEvent`'s own
  `Recipients` (built from a `GetCaseByID` call) went out to nobody. An
  earlier revision of this fix mirrored `req.WatchList` (whatever the
  caller sent, forwarded to ServiceNow via `s.snMirror.CreateCase`) into
  `work_item_watcher` — deliberately replaced: that design still routed the
  default watch list through ServiceNow-shaped concepts (email vs. UUID
  resolution, `userRepo.GetUserByEmail` lookups) for something this schema
  can answer directly. `createCaseSNFirst` calls `addRequestedWatchers`
  right after `CreateCaseFromServiceNow` succeeds and before
  `publishCaseCreatedEvent`: it persists `req.WatchList` (whatever the
  caller explicitly asked for) via `CaseRepository.SetCaseWatchList`,
  nothing more — `req.WatchList` itself is unaffected by any of this; it's
  still forwarded to ServiceNow as part of the create request the normal
  way, this addition is purely about what the Postgres mirror also
  guarantees.

  **The account's four named stakeholders are never persisted into
  `work_item_watcher` at all, on either the create or the update path —
  they're resolved fresh, straight from the account row, every time a
  `case.*` event is about to be emailed.** An earlier version of this
  auto-added `account.technical_owner_id`/`secondary_technical_owner_id`/
  `account_manager_id`/`renewal_account_manager_id` (migration 0012,
  `customer_success_manager_id` deliberately excluded — unlike the other
  four, the CSM is not meant to receive these default case notifications)
  as real watch-list rows on every create and merged them back in,
  unremovable, on every update (`addAccountDefaultWatchers`/
  `updateCaseWatchList`'s own "mandatory stakeholder floor"). Replaced at
  explicit product request: a stakeholder reassignment on the account (the
  account's own `technical_owner_id` etc. changing) had no effect on a
  case's already-persisted watch list, so every case created before the
  reassignment kept emailing the OLD stakeholder indefinitely — and a
  case's "Watchers" list in both portals showed four people who were never
  really watching *that* case specifically, just standing in for "whoever
  holds this account role right now." `resolveCaseDefaultWatcherEmails`
  (`sn_case_service.go`, shared by every `case.*` publisher — see
  "Recipients depends on req.Type" above) calls
  `CaseRepository.AccountDefaultWatcherEmails` (a `project JOIN account`
  straight to `"user".email`, no id-to-email round trip) and unions the
  result into that event's `Recipients`, fresh, every single send — so a
  reassignment is reflected on the very next notification with no case
  edit required, and a departed stakeholder stops being emailed the moment
  the account itself is updated. `addRequestedWatchers` (create) and
  `updateCaseWatchList` (update) now persist only what the caller
  explicitly asked for — no merge, no floor, no exemption from
  `validateWatchListProjectMembership` for a submitted id (nothing exempt
  to submit any more).

  **Superseded below: `fetchCaseWatchers` now also synthesizes the
  account's named stakeholders directly into every read, not just into the
  email audience.** The paragraph above (and `AccountDefaultWatcherEmails`)
  is still exactly how `Recipients` is resolved for a `case.*` email — that
  hasn't changed, `customer_success_manager_id` included: the CSM still
  isn't unioned into a case.* email's `Recipients`. What changed, by later
  explicit product request, is the *display* side: a case's "Watchers" list
  in both portals previously showed only real `work_item_watcher` rows, so
  the stakeholders being emailed by default were invisible on that list
  entirely — a customer or engineer looking at "who's watching this case"
  had no way to see them. `fetchCaseWatchers` (`case_repo.go`) now builds
  its result as a `WITH` CTE rather than a single `SELECT` off
  `work_item_watcher`:

  - `persisted` reads real `work_item_watcher` rows as before, but an
    `EXTERNAL` (customer) one is only included when they are currently a
    live (non-`DEACTIVATED`) `project_contact` on the case's own project —
    a customer who has since left the project stops showing up as a
    watcher. An `INTERNAL`/`SYSTEM`/`NOT_AVAILABLE` watcher is never
    subject to that check at all, which is also what keeps an engineer's
    own Follow/Unfollow self-subscribe working regardless of
    `project_contact` membership.
  - `stakeholders` resolves **five** account roles — the same four
    `AccountDefaultWatcherEmails` resolves, plus
    `customer_success_manager_id` — and synthesizes one row per resolved
    stakeholder, every time, with `locked = true`. This is a strictly
    larger set than the email audience by deliberate product decision: the
    CSM is a real stakeholder worth *showing* on the case, even though
    `AccountDefaultWatcherEmails` still deliberately excludes them from the
    default email audience (see that function's own doc comment — a
    decision about who gets emailed, not about who the account's
    stakeholders are). None of these five are auto-persisted into
    `work_item_watcher` (see above) and still aren't; this is a read-time
    join, not a write.
  - A user who is both a real persisted watcher and one of the five
    stakeholders appears exactly once, as the locked (stakeholder) copy —
    `DISTINCT ON (id)` ordered `locked DESC` after `UNION ALL`-ing the two
    CTEs together, then re-sorted by `user_name` for a stable response.

  `domain.WatchListUser.Locked` therefore does carry a real enforcement
  meaning again, just not the old "mandatory floor" one: `SetCaseWatchList`
  has no way for a caller to submit one of these five as an explicit
  watcher (there's no `work_item_watcher` row to add or remove), so a
  Locked entry can only ever come or go via the account's own stakeholder
  columns changing, never via an add/remove request. See that field's own
  doc comment in `entity.go`.

  **A case's watch list now also gains whoever comments on it, scoped to
  that one case.** `caseService.createCaseCommentAs` (the Postgres comment
  path — `work_item_watcher` has no ServiceNow equivalent, so this doesn't
  apply to the pure-ServiceNow data source) calls
  `subscribeCommenterToWatchList` right after the comment itself is
  successfully written: it resolves the commenter's own `"user"` row via
  `userRepo.GetUserByEmail` and, if that resolves, calls the new
  `CaseRepository.AddCaseWatcherIfAbsent(ctx, caseID, userID)` — a single
  `INSERT ... WHERE NOT EXISTS`, not `SetCaseWatchList`'s destructive
  delete-and-replace, so it can run on every comment without disturbing
  whatever else is already on the list. Best-effort and silent on failure,
  same posture as every other comment-creation side effect in this file
  (`completeResponseSLAOnComment`, `publishCommentAddedEvent`) — the
  comment has already been written by the time this runs, so a lookup or
  write failure here must never undo that. An `actorEmail` that doesn't
  resolve to a real user (the M2M `CreateCaseCommentAs` path deliberately
  has none — see that method's own doc comment) is skipped the same way
  `isSupportEngineerAuthor` already treats it: can't confirm, not an error.
  This needed no new eligibility check of its own:
  `validateWatchListProjectMembership`/`filterActiveWatchListUsers` already
  apply to whatever ends up in `work_item_watcher` regardless of how it got
  there, so a commenter who later leaves the project is dropped from future
  emails by that existing mechanism exactly like any other persisted
  watcher.

  **Duplicates across all of the above are impossible by construction, not
  by a separate filter.** `work_item_watcher`'s own `UNIQUE (work_item_id,
  user_id)` makes `AddCaseWatcherIfAbsent` a no-op for an existing
  watcher; `fetchCaseWatchers`' `DISTINCT ON (id)` is what collapses a user
  who is simultaneously a persisted watcher and a synthesized stakeholder
  into the one locked entry described above.

  **A persisted (explicitly-added) watcher is re-checked for live project
  membership immediately before each `case.*` email goes out, for
  everything except case creation.** `validateWatchListProjectMembership`
  only ever ran once, when a watcher was first added — someone who later
  left the project (deactivated, or never finished registering) kept being
  emailed indefinitely, since nothing re-checked. `filterActiveWatchListUsers`/
  `isActiveProjectWatcher` (`case_service.go`) re-run that same two-part
  check (INTERNAL staff, or a REGISTERED `project_contact` on the case's
  project) against `cv.WatchList`/`before.WatchList` right before each of
  `publishCommentAddedEvent`/`publishStatusChangedEvent`/
  `publishSeverityChangedEvent`/`publishCaseAssigned`'s own Postgres-path
  call sites, silently dropping (logged at INFO, not an error) anyone no
  longer eligible. Deliberately NOT applied to case creation — a watcher
  requested in the same `CreateCase` call couldn't possibly have gone stale
  within that same request — and deliberately NOT applied on the
  plain-ServiceNow data source, which has no Postgres `project_contact`
  table to check a ServiceNow-sourced watch list's (non-Postgres-UUID)
  ids against in the first place. A repository error while checking a
  given watcher keeps that watcher rather than risk silently dropping a
  real recipient over a transient failure.
- **Account contacts** (`account_contact`, migration 0026) and **project
  contacts** (`project_contact` + `project_contact_group`/`project_group`/
  `project_group_role`/`project_role`, migrations 000022-000025): new
  `AccountContactService`/`ProjectContactService` Postgres implementations.
  Neither table has its own name/email column — `account_contact.user_name`
  and, for project contacts, `account_contact` joined through
  `project_contact.account_contact_id` are matched against `"user".user_name`
  (case-insensitively) to resolve a display name/email; a row with no
  matching `"user"` row falls back to the raw `user_name` (account contacts)
  or the invited `email` (project contacts, matching
  `domain.ProjectContact.Email`'s own documented fallback). A project
  contact's `Roles` is the union of `project_role.role` across every
  `project_group` it belongs to via `project_contact_group` — which now
  includes `ADMIN`, since admin is a project role (see "Portal-driven
  membership writes" above); `AccountRoles` is the separate account-level list
  described in that same section.
  `NotificationsEnabled` has no backing column anywhere in this schema and
  is hardcoded `true` (see `projectContactRowToDomain`'s own comment) —
  flagged as a known gap, not fabricated data pretending to be real.
- **User roles** (`role`/`user_role`, migrations 0008/0010):
  `SearchUsersFilters.RoleIDs` (holds role **names**, e.g. `"admin"`,
  despite the field's name — see `domain.UserRole`'s own doc comment) was
  previously rejected outright on Postgres; `user_repo.go`'s `SearchUsers`
  now joins through `user_role`/`role` with OR semantics (matches if the
  user holds *any* of the given roles). `GetMe`'s `Roles` is still always
  empty — nothing has asked for it on that path, this only wires up the
  search filter.
- **Case activities** (`CaseRepository.SearchCaseActivities`): merges
  `comment` and complete `case_attachment` rows into one newest-first feed
  via a `UNION ALL` CTE — was previously an unconditional
  `ServiceUnavailableError` stub. There is no field-change audit table in
  this schema, so `req.IncludeFieldChanges` has no effect on this data
  source; an absent field-change history is a valid state per
  `SearchCaseActivitiesRequest`'s own doc comment, not an error.
  `CaseActivity.DownloadURL` is left empty for attachment entries — this
  service builds no portal links or absolute URLs to itself (same posture
  as the Event Hub section above); a caller resolves the actual bytes via
  `GET /attachments/{id}/content`. The comment branch's `"user"` join is by
  email (`comment.created_by` is a free-text VARCHAR, not a FK), and
  `"user".email` has no unique constraint (migration 0002 only makes
  `user_name` UNIQUE) — so that join is wrapped in its own `DISTINCT ON
  (cm.id)` subquery to guarantee one activity row per comment even if two
  user rows share an address. Without it, a shared address would fan one
  comment out into multiple feed rows while the sibling `COUNT` query (which
  never joins `"user"`) still counted it once, so the page and its `total`
  would disagree.

**Pre-existing bug fixed as a side effect, not scope creep**: `user_repo.go`
queried a `users` table with `created_at`/`updated_at`/`phone`/`timezone`
columns that do not exist anywhere in `migrations/` — the real table is
`"user"` (migration 0002) with `created_on`/`updated_on` and no
`phone`/`timezone` column at all. Every identity-resolution call this
service makes (`GetUserByEmail`, used by `CreateCaseComment`, `AddCaseTag`/
`RemoveCaseTag`/`SearchTags`, `SetCaseWatchList`, `CreateTimeCard`/
`UpdateTimeCard`/`DeleteTimeCard`/`TransitionTimeCardState`, `resolveActor`)
depended on this, so it had to be fixed here rather than deferred — see
"Fixing the plural/singular table-name mismatch" below for the five sibling
repos that had the same problem and are now fixed too.

**Threading the caller's identity to the repository layer**: several of the
methods above (`SearchAccountContacts`, `SearchProjectContacts`,
`GetProjectContact`) accept a `callerEmail string` parameter that reaches
the repository layer but is **not yet used to restrict any query** — added
at explicit request, so a future authorization decision (e.g. restricting
an `EXTERNAL` `user_type` caller to only the accounts/projects they are
themselves a contact on) has the caller's identity already available at the
SQL-query-writing layer without needing to re-plumb it through every layer
again. `resolveCallerEmail` (`account_contact_service.go`) is the shared
helper: decodes `x-user-id-token`'s `email` claim without a `"user"` table
lookup, since nothing on these paths needs the caller's platform id today,
only their claimed email.

## PATCH /cases/{id}: assignee, acknowledge, parent, and the combinable field bundle

Found live: assigning a case ("Assign to me") and acknowledging one both
400'd on this data source with a generic "Invalid request payload." (the
CSM/customer portal backend's own catch-all for any upstream 400) --
`AssigneeEmail` and `Acknowledge` were on `UpdateCase`'s unconditional
"only supported for the ServiceNow data source" rejection list even though
neither actually needs anything ServiceNow-specific: `work_item.
assigned_to_id` (migration 0039) and `work_item.acknowledged_by_user_id`
(migration 0021) are both real, direct columns, already read elsewhere
(`assignedUserId` search filter, `GetCaseByID`'s own `AssignedEngineer`).
Prompted by that bug report, this pass re-derived `UpdateCase`'s *entire*
field-combination contract from `sn_case_service.go`'s own UpdateCase --
the actual, currently-enforced source of truth for which fields may be
combined -- rather than re-guessing it, since the Postgres and ServiceNow
data sources must accept the same PATCH shapes.

**The exclusive/combinable split now mirrors ServiceNow's exactly**, down to
the variable names (`exclusiveCount`/`combinableCount` in both files' own
`UpdateCase`):
- **Exclusive** (at most one per request, and none may be combined with
  anything else, including each other): `state`/`severity`/`workState` (one
  of the three), `watchList`, `assigneeEmail`, `parentId`, `acknowledge`.
  `parentId` joins this group for the first time here -- it was previously
  rejected outright; `work_item.parent_id` (migration 0039) is the same
  self-reference `GetCaseByID`'s own `ParentCase` already reads the other
  direction, so `updateCaseParent`/`CaseRepository.UpdateCaseParent` wire it
  up the same way `updateCaseAssignee` does.
- **Combinable** (any subset, freely combined with each other, never with
  the exclusive group): `subject`, `description`, `deploymentId`,
  `deployedProductId`, `bestCaseFixEta`/`mostLikelyFixEta`/`worstCaseFixEta`,
  `relatedCaseId`, `autocloseHoldUntil`, `workaroundProvided` -- all newly wired up via
  `updateCaseFields`/`CaseRepository.UpdateCaseFields`, one dynamic
  `UPDATE ... SET` per table (`work_item` for most of these,
  `"case"` for `relatedCaseId` alone) built from exactly the non-nil pointers
  `req` carries. `UpdatedCase` only has an echo slot for the fix-ETA trio
  (see each field's own doc comment, "Present only when the update set X");
  every other field in this bundle follows ServiceNow's own "a plain field
  write only returns `{id, updatedOn, updatedBy}`" contract -- the caller
  re-reads via `GetCaseByID` to see the new value.
- **`resolutionCode`/`cause`/`closeNotes` are deliberately NOT in either
  group above.** `sn_case_service.go`'s own UpdateCase only allows them
  alongside a `state` transition, and only to `closed` or
  `solution_proposed` (`snResolutionStates`) -- so they ride inside the
  existing `state`/`severity`/`workState` branch's own `"case"` `UPDATE`
  (`updateCaseQuery`'s new `$5`/`$6`/`$7`), gated by the identical
  restriction, rather than living in the free-standing combinable bundle.
  `closeNotes` uses `COALESCE($7, close_notes)` rather than the other five
  columns' `''`-sentinel trick, since `""` is itself a meaningful value to
  write there (clearing existing notes), unlike an enum column where `''` is
  never valid anyway.
- **`issueType`/`engagementType`/`engagementPaymentType`/`catalogId`/
  `catalogItemId`/`variables` stay rejected**, for the mirror-image reason:
  `sn_case_service.go` only accepts them when `type` is also provided (a
  full type transfer) -- `"engagementType, engagementPaymentType, issueType,
  catalogId, catalogItemId, and variables are only allowed when type is also
  provided"`. `type` itself has no Postgres implementation (a real type
  transfer would mean moving a row between `"case"`/`engagement`/
  `service_request`/etc, each a physically separate extension table --
  genuinely larger, separate work, not attempted here), so none of its five
  companions have anywhere to go either. `addPublicComment`/`product`/
  `publicTicket` (the "Share Fix ETA" comment-posting side effect) remain
  rejected too. `autocloseHoldUntil` is **not** in this list: it was, on the
  belief that no column backed it, but every case-like extension table has
  `autoclosure_step`/`autoclosure_state_on` -- see "Auto-closure hold" below.

**A real pre-existing read-side bug found while building the write side**:
`GetCaseByID` cast `"case".resolution_code` straight into
`domain.CaseResolutionCode` with no translation at all
(`domain.CaseResolutionCode(*resolutionCode)`), but three of the sixteen
`case_resolution_code_enum` labels don't match their domain constant by
identity -- `CONSIDERED_FOR_ROADMAP_ALT`/`SOLVED_WORKAROUND_PROVIDED_ALT`
are ServiceNow's own duplicate picklist entries for a concept the Postgres
enum only has one canonical label for, and
`AbruptlyClosedDueToNonResponsiveness` is missing the enum's own
`_THROUGH_AUTO_CLOSURE` suffix. Every case resolved with that last code
rendered a `resolutionCode` value no `domain.CaseResolutionCode` constant
declares. `caseResolutionCodeToEnum`/`caseResolutionCodeFromEnum`
(`case_repo.go`, next to `caseSeverityToEnum`'s own identical-shaped fix)
hold the mapping both directions now, same "flag the mismatch explicitly
rather than guess" precedent as severity's own S0..S4 mapping.

**The SN-mirror-writeback pattern was extended to match**, so
`DATA_SOURCE=postgres-servicenow-dual-write` doesn't drift on these fields
either: `patchCaseAssignee`/`patchCaseAcknowledge`/`patchCaseParent`/
`patchCaseFieldsBundle` (`sn_case_service.go`) are bare ServiceNow PATCHes
with none of `UpdateCase`'s own enrichment reads or no-op detection --
exactly `patchCaseFields`/`patchCaseWatchList`'s own established shape,
reached through four new narrow interfaces
(`snAssigneePatcher`/`snAcknowledgePatcher`/`snParentPatcher`/
`snFieldsBundlePatcher`). The acknowledge mirror only fires when this call's
own claim actually succeeded (`!alreadyAcknowledged`) -- a repeat
`Acknowledge:true` against an already-acknowledged case changed nothing in
Postgres, so there's nothing new to mirror.

**`patchCaseFieldsBundle` mirrors only four of the nine combinable
fields.** `snUpdateCasePayload`'s own field comments say
`Title`/`Description`/`DeploymentID`/`DeployedProductID`/`RelatedCaseID`
are each "not yet available in the backing service" -- a first version of
this mirror sent them anyway (Subject onto the payload's `Title`,
`DeploymentID`/`DeployedProductID`/`RelatedCaseID` converted to sysids),
which a CodeRabbit review on PR #1986 caught: sending a field the backing
service doesn't implement either gets silently ignored or fails the whole
PATCH, neither of which leaves ServiceNow any better synced than not
mirroring it. Only `BestCaseFixEta`/`MostLikelyFixEta`/`WorstCaseFixEta`/
`WorkaroundProvided` are confirmed available (their own doc comments say
so) and actually forwarded; the function returns `nil` without a PATCH
call at all when a request sets none of those four, rather than sending an
empty no-op. All nine fields still write to Postgres via
`CaseRepository.UpdateCaseFields` regardless -- this only narrows what the
*ServiceNow mirror* attempts. The `sn_writeback_failures` payload for this
branch (`updateCaseFields`'s own `Dispatch` call) records the actual values
of those same four fields, not a fixed field-name placeholder, so a failed
mirror can actually be replayed by hand.

**`resolutionCode`/`cause`/`closeNotes`'s state-gating check runs before
the branch dispatch, not after.** The same CodeRabbit review caught that
neither `exclusiveCount` nor `combinableCount` counts these three fields at
all, so a request like `{assigneeEmail, resolutionCode}` or `{subject,
closeNotes}` used to sail past the mutual-exclusion check, get dispatched
to `updateCaseAssignee`/`updateCaseFields`, and return 200 with the
resolution fields silently ignored -- never validated, never written. The
check now runs immediately after the `exclusiveCount`/`combinableCount`
validation and before any branch (`WatchList`/`AssigneeEmail`/`ParentID`/
`Acknowledge`/the combinable bundle) gets a chance to return early.

## Auto-closure hold (`autocloseHoldUntil`) on the Postgres data sources

`PATCH /cases/{id} {autocloseHoldUntil}` is the CSM portal's "Hold auto-closure"
action. It used to 400 on `postgres` and `postgres-servicenow-dual-write` ("only
supported for the ServiceNow data source", the portal showing only "Could not
hold auto-closure."), because the Postgres service listed it among fields with
"no backing column". That was wrong: `"case"`, `service_request`, `engagement`
and `security_report_analysis` all carry `autoclosure_step VARCHAR(50)` and
`autoclosure_state_on TIMESTAMPTZ` (migrations 0023/0024), the very columns
csm-sync-service fills from ServiceNow's `u_autoclosure_step` /
`u_autoclosure_state_time` (staging holds `DEFAULT`, `FIRST_COMMENT`, `ON_HOLD`,
`SECOND_COMMENT`; the `ON_HOLD` rows carry midnight-UTC dates). `announcement`
has neither column, so a hold on one is a 400, never a silent no-op.

- **Write** (`setAutocloseHoldTx`, called from `updateCaseFieldsTx`, same
  transaction as the other plain fields): `autoclosure_step = 'ON_HOLD'` and
  `autoclosure_state_on` = the UTC calendar day at midnight, on whichever
  extension table owns the row. The hold has day granularity, and midnight UTC is
  what the ServiceNow mirror sends (`formatSNDateOnly`) and what the sync reads
  back, so the two stores never disagree about the day.
- **The date contract: the day is the UTC date of `autocloseHoldUntil`, so a
  client sends the chosen day at 00:00 UTC.** The CSM portal used to send the end
  of the picked local day converted to UTC, which is the *next* UTC day for anyone
  west of UTC: 23:59 on 22 Oct in New York is 03:59 on 23 Oct UTC, so the hold
  landed on 23 Oct (a day late), on the ServiceNow data source too, since July.
  East of UTC it was harmless (23:59 on 22 Oct in Colombo is 18:29 on 22 Oct UTC).
  The portal now sends `2026-10-22T00:00:00.000Z`, and reads the stored day back
  with the UTC date. This service keeps taking the UTC date, so a deployed older
  portal behaves exactly as before: nothing here changes with deploy order.
- **It is a plain combinable field**, counted in `combinableCount`, so it obeys
  the same "never with an exclusive field" rule as `subject`.
- **Mirror (dual-write)**: `patchCaseFieldsBundle` forwards it as
  `autocloseHoldUntil` (date only) and the `sn_writeback_failures` replay payload
  records it. This is the write that decides anything: ServiceNow's own flow is
  what closes -- or doesn't close -- the case, so a hold stored only in Postgres
  would stop nothing. The next sync brings ServiceNow's own step/time back.
- **Read**: `GetCaseByID` returns `autoclosureStep` / `autoclosureStateTime` from
  `caseLikeAutoclosureStepColumn` / `caseLikeAutoclosureStateOnColumn`, as the
  ServiceNow data source always did. A case with no step omits both. **This puts
  `FIRST_COMMENT` / `SECOND_COMMENT` on about 5,800 staging cases that showed
  nothing before**; only `ON_HOLD` is a hold, and the CSM webapp's chip and dialog
  prefill key on `ON_HOLD` for that reason.
- Not done: the case *search* views carry no auto-closure fields on any data
  source, and nothing here releases a hold early or moves a case between the
  other steps (the raw step stays unsettable by design).
- Tests: `case_repo_autoclose_hold_integration_test.go` (real Postgres, run as a
  non-superuser so row-level security is in force; `CASE_STATS_TEST_DSN`),
  `TestCaseService_UpdateCase_AcceptsAutocloseHold` / `_AutocloseHoldIsMirroredToServiceNow`,
  `TestSNCaseService_PatchCaseFieldsBundle_*`, and
  `TestCaseService_UpdateCase_AutocloseHoldReachesServiceNowOverHTTP`, which runs
  the real production chain (case service, writeback dispatcher, a real
  `snCaseService` mirror) against a fake ServiceNow server and asserts the PATCH
  it receives: the case's sysid, only `autocloseHoldUntil` as a date, the caller's
  token forwarded.

## Change requests

`change_request` (migration 0043) is a shared-PK extension of `work_item`,
same pattern as `"case"` (`change_request.id` IS `work_item.id`). `SearchChangeRequests`,
`AggregateChangeRequests`, `GetChangeRequest`, and `PatchChangeRequest` are
wired up to it (`change_request_repo.go`/`change_request_service.go`).
`changeRequestService.SearchChangeRequests` validates `req.SortBy` against
the same `validChangeRequestSortField`/`validChangeRequestSortOrder` maps
`sn_change_request_service.go` already used, so an unrecognized `sortBy`
value is a 400 on both data sources instead of silently falling back to
`created_on DESC` only on Postgres.

### The approval tables are the sync layer's (state, not status)

`approval_stage` and `approval_stage_approver` mirror ServiceNow's
`sysapproval_group` / `sysapproval_approver` and are defined by csm-sync-service
(migrations 0089 and 0138, mirrored into this folder). **This service adds nothing
further to them and renames nothing.** The one column it did add to the sync-owned
`approval_stage` (`checkpoint_label`, migration 0179: the stage's name, NULL on a
synced stage, which then falls back to the stage's position) predates this rule; it
is kept as it stands until the ServiceNow table that defines approval stages /
checkpoints is reconciled with it, and nothing further is added. The approver's
standing is the column **`approval_stage_approver.state`** (renamed from `status`
by 0138), stored UPPER_SNAKE_CASE: `REQUESTED`, `APPROVED`, `REJECTED`, `NOT_REQUESTED`,
`NOT_REQUIRED`, `CANCELLED`, `NOT_ENTITLED`. Every statement below that says an
approver row is "requested" / "approved" / "rejected" / "cancelled" means that value
of `state` (the request-level decision a client sends stays lowercase,
`"approved"` / `"rejected"`; the repository writes it uppercased). The read model
(`normalizeChangeRequestApprovalStatus`) passes the stored value through and only
upper-cases a lowercase raw ServiceNow value, so it never double-converts; the BFFs
and webapps compare it case-insensitively against the same UPPER_SNAKE set.
`approval_stage.raw_status` is the raw ServiceNow `approval` passthrough and stays
lowercase.

### Approval flow by change type (current behaviour)

> **This section is the current contract and supersedes the Assess/Authorize/
> "Move to Assess" history further down wherever they differ** (that history
> is kept for the reasoning behind individual mechanisms). Code:
> `change_request_approval_flow.go` plus `patchChangeRequestTx` /
> `DecideChangeRequestApproval` in `change_request_repo.go`; migration
> `0188_change_request_approval_groups.sql`.

**A change request is created with exactly one of three types** — `standard`,
`normal`, `emergency` (ServiceNow's own "What type of change is required?":
Normal "requires one or more approvals", Standard "does not require approval",
Emergency "must be implemented as soon as possible"). `type` is **required** on
create: `repository.ValidateCreateChangeRequestType` is applied by both
Postgres creates, the ServiceNow-first (dual-write) path *before* ServiceNow is
called, the pure ServiceNow service, and the csm-portal BFF. `azure`/`infra`/…
still exist on synced legacy rows and read back fine but cannot be chosen at
create. The type cannot be changed by PATCH once an approval stage exists.

| Type | Flow |
|---|---|
| Normal | New →(**Request Approval**)→ Assess `[Peer Approval]` → Authorize `[CAB Approval]` → **Scheduled automatically on CAB approval** → Implement → Review → Closed |
| Emergency | New →(**Request Approval**)→ Authorize `[ECAB Approval only]` → **Scheduled automatically on ECAB approval** → Implement → Review → Closed |
| Standard | New →(**Request Approval**)→ **Scheduled** (no approval stages at all) → Implement → Review → Closed |

Two off-ramps/loops sit outside the table: **Roll back** (from Review / Customer
Review, final) and **Re-schedule** (from Customer Approval back to Authorize,
below).

The table is the flow with both creation-form checkboxes **unticked**. With
**Customer Approval** ticked, every "→ Scheduled" above becomes "→ **Customer
Approval** → (the customer's own approval) → Scheduled"; with **Customer Review**
ticked, "Review → Closed" becomes "Review → **Customer Review** → (the customer's
own review) → Closed". See "Customer Approval / Customer Review checkboxes" below.

**Compliance rule: no staff action records the customer's approval or review on
the customer's behalf.** The customer's answer is the customer's decision and
ServiceNow's record of it is audited; a decision made for the customer and stored
as theirs would be a compliance problem. A change in Customer Approval / Customer
Review therefore moves on **only through the customer's own answer**, given in the
Customer Portal (`PATCH {isCustomerApproved|isCustomerReviewed}` from a registered
contact, or the decision route on their own approver row -- "Customer answers
through PATCH" below). There is no "Bypass customer approval" / "Bypass customer
review" (they were a manual `{state: scheduled}` / `{state: closed}` that stamped
the flag): the refusal is the same whatever the project, the contacts or the
stage (see the first bullet after the table). What staff keep: **Cancel** (any
non-final state), **Re-schedule** out of Customer Approval (the customer is asked
again) and **Roll back** out of Customer Review (while nobody is being asked).
Emergency changes are acted on without the customer's consent: they do not tick
the customer boxes. Same rule for every door: the PATCH, the decision route (a
caller decides only their own `REQUESTED` row), create and clone (always New), the
GitHub sync's `SetState` (refuses to leave a customer state), the dual-write mirror
(a refused PATCH is never mirrored) and the pure ServiceNow data source (its
offered states are filtered the same way).

* **The transition graph of `PATCH {state}`** (`change_request_transitions.go`; Postgres and
  dual-write -- a pure ServiceNow source forwards the PATCH and ServiceNow is the authority).
  One table, `changeRequestForwardNextStates` + `changeRequestRollbackFrom` + Cancel from every
  non-final state, is read twice: `legalChangeRequestNextStates` renders it as `legalNextStates`
  and `checkStaffStateRequest` enforces it in `patchChangeRequestTx` (right after the row lock),
  so what the portal offers is exactly what the service accepts. A staff request may name the
  state the change is already in (a resend: no move) or a move out of it:

  | from | staff may request by PATCH |
  |---|---|
  | new (or NULL) | `assess` (Request Approval), `canceled` |
  | assess | `canceled` only: the **peer approval** moves it on (Assess -> Authorize is the cascade's) |
  | authorize | `canceled` only: the **CAB / ECAB approval** moves it on (to scheduled, or customer_approval) |
  | customer_approval | `authorize` (Re-schedule), `canceled`: the **customer's answer** moves it on |
  | scheduled | `implement`, `canceled` |
  | implement | `review`, `canceled` |
  | review | `closed` (or `customer_review` when `customerReviewRequired`), `rollback`, `canceled` |
  | customer_review | `rollback` (while nobody is asked), `canceled`: the **customer's review** moves it on |
  | closed, canceled, rollback | **nothing: final** |

  Every other request is a 400 and writes nothing: from a **final** state
  `state "implement" cannot be set manually from canceled: a change request that is canceled
  cannot be moved` (likewise `closed`, and `rolled back` -- this replaces the rollback-only
  guard: a change canceled in Customer Approval used to be revivable to Implement / Review /
  Closed / Customer Review with no customer answer, and `{state: implement}` from New, Assess or
  Authorize used to skip Peer and CAB approval and the customer); a **jump** `state "implement"
  cannot be set manually from assess: it is waiting for its peer approval, which moves it on by
  itself; the moves open to staff from assess are: only canceled` (what the change waits for,
  then the open moves); out of a customer state the customer-answer refusals below
  (`refuseStaffExitFromCustomerState`); `scheduled` / `customer_approval` / `authorize` (outside
  Re-schedule) / `rollback` (outside the review states) / `assess` (not New) / `new` keep the
  refusal of their own -- **also when the change is already in that state** (a resent
  `{state: scheduled}` is still a 400: staff never name it; the other resends -- `new`, `assess`,
  `implement`, `review`, `customer_review`, `closed`, `canceled` -- are accepted no-ops). The
  requested state is **trimmed and read case-insensitively** (`" IMPLEMENT "` is `implement`);
  a value that is not a state is a 400 `state "X" is not a change request state`.
  **`legalNextStates` of Assess no longer lists `authorize`**: the PATCH always refused
  Assess -> Authorize (only the peer approval takes it), so the portal was offered an edge the
  service would not accept; `assess` offers `["canceled"]`. A NULL state is New for the PATCH
  but has no `legalNextStates` (as it never had). Tests: `TestChangeRequestStaffTargets`,
  `TestCheckStaffStateRequest_*`, `TestTransitionRefusalMessages` (unit) and
  `TestChangeRequestTransitionsIntegration_*` (every state by every request, with every box
  combination, native and migrated-shaped rows; Cancel-then-revive from every customer-state
  exit and from Closed / Rollback; no step skipped; spelling), run as a superuser and as `csm_app`.
* **"Request Approval" is the one human action out of New** and is always sent
  as `{state: "assess"}` (legalNextStates of New is `["assess","canceled"]` for
  every type — the webapp contract). The state written is chosen from the type,
  never by the caller: Assess / Authorize / Scheduled. It is only legal from New
  (a resend that matches the resulting state is an idempotent no-op; anything
  else is a 400). It still requires an assigned team.
* **There is no "Schedule" action.** `scheduled` is never in `legalNextStates`
  and a manual `{state: "scheduled"}` (or `"authorize"` / `"customer_approval"`)
  PATCH is rejected (400) **from every state, `customer_approval` included**:
  Scheduled is reached only by the CAB/ECAB approval cascade, by Request Approval
  on a Standard change, or -- out of `customer_approval` -- by the customer's own
  approval. Out of `customer_approval` the 400 is `state "scheduled" cannot be set
  manually from customer_approval: the customer's approval can only be given by
  the customer in the Customer Portal; cancel the change or re-schedule it
  instead`, whatever the project (none, no contacts, only the creator), whether
  anybody was asked, whether a stage is live or already decided, and with or
  without `isCustomerApproved` in the body (`customerOutcomeRefusal`). The
  same holds for `{state: "closed"}` out of `customer_review` (`... from
  customer_review: the customer's review can only be given by the customer in the
  Customer Portal; roll the change back or cancel it instead`, or `... cancel the
  change instead` while the customer group is being asked, because a failed
  review is then theirs to give too), and for **every other destination** out of a
  customer state but Cancel, Re-schedule (`customer_approval`) and Roll back
  (`customer_review`) -- `{state: "implement"}` would skip the customer exactly as
  `scheduled` would (`refuseStaffExitFromCustomerState`, which keeps the customer's wording
  for these two states; staying where it is, e.g. a resent Request Approval on a Standard
  change, is not an exit). A refused PATCH writes nothing
  (state, flags, approver rows and `updated_on` stay). Nor can staff send the
  customer's flags: `isCustomerApproved` / `isCustomerReviewed` from anyone but
  the customer (an internal caller, staff who also hold an external record, an
  internal client credential), true or false, alone or with a state, is a 400
  `isCustomerApproved cannot be set on the customer's behalf: the customer's
  approval can only be given by the customer in the Customer Portal` (likewise
  `isCustomerReviewed`; `refuseStaffCustomerOutcomeFlags`) -- refused, not
  ignored. The CSM portal has no "Bypass customer approval" / "Bypass customer
  review" entries in its "Change state" menu any more. The ServiceNow data
  source's own offered states are filtered the same way
  (`withoutCustomerOutcomeStates`: `scheduled` from no state, `closed` not from
  `customer_review`; what ServiceNow itself returns is unchanged, and a PATCH on
  that source is forwarded to ServiceNow, which is the authority there).
  `legalNextStates` per state (the single source of truth the webapp renders): new
  `[assess, canceled]`, assess `[canceled]` (the peer approval moves it on), authorize
  `[canceled]` (the CAB / ECAB approval does),
  customer_approval `[authorize, canceled]` (`authorize` = Re-schedule), scheduled
  `[implement, canceled]`, implement `[review, canceled]`, review
  `[closed, rollback, canceled]` -- or `[customer_review, rollback, canceled]` when
  `customerReviewRequired` --, customer_review `[rollback, canceled]`, terminal
  states none. **While a live Customer Review stage exists (the project's contacts
  are being asked, see "Customer Group" below) `customer_review` offers only
  `[canceled]`**: the manual `rollback` is withdrawn and refused too (a member's
  rejection rolls the change back). `customer_approval` is `[authorize, canceled]`
  live or not (Re-schedule stays: an internal user may re-plan, which supersedes
  the pending request).
* **Re-schedule** (the process diagram's "Time Change" loop). In
  `customer_approval`, `PATCH {state: "authorize", plannedStartOn?,
  plannedEndOn?}` sends the change back through internal approval because the
  planned time changed. It is the one manual way into `authorize`; from any
  other state the PATCH is a 400 `state "authorize" cannot be set manually: it
  is reached automatically through the approval flow (Request Approval, then
  peer approval); it can only be set by hand to re-schedule a change from
  customer_approval`. **"Time Change = Yes" is enforced**: the request must
  carry a start and/or end that differs from the stored instant, else 400
  `re-scheduling requires a changed planned start or end: ...` (a bound that is
  not a date-time: 400 `plannedStartOn must be a valid date-time, either RFC 3339
  ... or YYYY-MM-DD HH:MM:SS in UTC ...`, see "The planned window is parsed
  here"; an end before the start: 400 `the planned start must not be after the
  planned end`; an end equal to the start: 400 `... must not be the same as the
  planned end: the window must have a duration`). The on-hold
  gate applies; the whole PATCH is one transaction, so a re-schedule that cannot
  be satisfied (e.g. the CAB group has nobody eligible) changes nothing.
  Effects: the new window is applied; the customer's pending stage is cancelled
  (it stays as a record, `provisionCustomerStage` as for any state exit); the
  state becomes `authorize` and a **fresh internal stage** is provisioned --
  Normal: a new "CAB Approval" stage (the peer approval stands), Emergency: a
  new "ECAB Approval" stage -- from the same group with the creator listed
  cancelled (`provisionReauthorizationStage`; no ordinal-position test, but
  `provisionApprovalStage` now counts only the FIRST stage of each label, so
  the Review checkpoint is still provisioned for a re-scheduled change). Stage
  order after one loop: Peer, CAB, Customer Approval (cancelled), CAB (new).
  When the new CAB / ECAB stage is approved the ordinary cascade sends the
  change to `customer_approval` and provisions a fresh Customer Approval stage
  for the customer group. Rejecting the new stage behaves as a CAB / ECAB
  rejection always has (siblings cancelled, state unchanged). **Re-schedule re-asks the
  customer on every row**: it writes our own `customer_approval_required = true` in the same
  UPDATE as the new window (never the sync-owned `is_customer_approval_required`, ServiceNow's
  record of the customer's answer), because the cascade that ends the loop reads that column
  (`approvalGateTarget`) and migration 0189 defaulted it to false for every existing and synced
  row -- including the ones already waiting in Customer Approval, where a Re-schedule used to go
  CAB -> **Scheduled** with the customer never asked about the new plan. A row with nobody to
  ask then waits in Customer Approval, as every such row does; a `customerApprovalRequired:
  false` resent in the same request is not a way round (the stored false is accepted by the
  lock, and the Re-schedule writes true).
  (`TestChangeRequestRescheduleLegacyIntegration_*`: native and migrated-shaped rows, Normal and
  Standard, with a project with contacts, with none and with no project, and the customer's own
  proposal.) **Standard** has
  no internal approval to repeat: the dates are applied, the change **stays in
  `customer_approval`** and the customer is asked again (pending stage
  cancelled, a fresh one provisioned when the group has an eligible member;
  the manual `scheduled` path stays otherwise). The loop can be repeated.
  `customerApprovalRequired` remains editable in `authorize` (existing rule),
  so it can still be unticked there. No new notifications.
* **Roll back** (`rollback`) is the failed-review off-ramp of the process
  diagram and is offered from exactly two states, `review` (internal review
  failed) and `customer_review` (customer review failed), whether or not
  `customerReviewRequired` is set (`changeRequestRollbackFrom`). A manual
  `{state: "rollback"}` from any other state is a 400 `state "rollback" can only
  be set from review or customer_review`; from `customer_review` with a live
  customer stage it is refused like a manual `closed` (the group's members
  decide; their rejection already yields `rollback`). The on-hold gate applies.
  Rolling back stamps no `is_customer_review_required` (`isCustomerReviewed`
  alongside it is a 400, as it is from staff in any request), provisions no stage, and **cancels every still-
  `REQUESTED` approver row** of the change (all stages stay as a record; the
  customer-group rejection cascade does the same). **`rollback` is final**:
  `legalNextStates` is none and any other state PATCH out of it is a 400
  (`state "implement" cannot be set manually from rollback: a change request that is rolled
  back cannot be moved`, the same refusal as for closed and canceled; a repeated
  `{state: rollback}` keeps `state "rollback" can only be set from review or customer_review`). Cancel and
  Close do the same since "An approval is only actionable in its stage's state"
  (below): every `closed` / `canceled` / `rollback` change has no `REQUESTED` row
  left, internal stages included (this supersedes the earlier "Cancel does not
  cancel the internal stages' pending approvers"). The ServiceNow data source
  replays `stateKey` 2 like any other state and `withoutCustomerOutcomeStates` does not
  strip `rollback`. Project stats "outstanding" counting is unchanged by this.
* **An approval is only actionable in its stage's state** (bug: an internal
  reviewer kept Approve / Reject on the *Review* stage of a change that was
  already `closed`, and while it waited at `customer_review` for the customer).
  Deciding Review changes no state -- a human moves the change on -- and nothing
  used to cancel the Review stage's other approvers when it left Review, so their
  rows stayed `REQUESTED` for ever. Now every stage is tied to the one state in
  which it can be decided (`approvalStageDecidableState`,
  `change_request_approval_flow.go`; the kind comes from `classifyApprovalStage`:
  explicit `checkpoint_label` first, the legacy positional fallback second):

  | Stage kind (label) | Decidable only while the change is in |
  |---|---|
  | Peer Approval (`Assess`) | `assess` |
  | CAB Approval (`Authorize`), ECAB Approval | `authorize` |
  | Review | `review` |
  | Customer Approval | `customer_approval` |
  | Customer Review | `customer_review` |

  A stage of unknown kind (`stageKindOther`: a ServiceNow-synced stage past the
  first two positions, or with an unrecognised label) and a change with a NULL or
  unknown state are **never guarded** -- ServiceNow-synced data behaves as before.
  It is enforced four ways:
  * **Auto-cancel** -- `reconcileStaleApprovers`, run in the same transaction at
    the end of every path that writes `change_request.state`: `patchChangeRequestTx`
    (whenever the PATCH carries a state: forward moves, Re-schedule, Roll back,
    Cancel, Close, the customer outcomes, on-hold-off-and-advance) and
    `DecideChangeRequestApproval` (after its cascades: Peer -> Authorize, CAB /
    ECAB -> Scheduled / Customer Approval, the customer stages' outcomes) and the
    GitHub sync's state writer (`githubMutationRepository.SetState`, a closed issue
    closing the change). It sets
    to `CANCELLED` (stamping `updated_on` / `updated_by`) every still-`REQUESTED`
    row of every stage whose decidable state is not the change's *current* state --
    and **every** still-`REQUESTED` row once the change is `closed`, `canceled` or
    `rollback`. It runs after the stage the new state needs was provisioned, so
    that stage (the fresh CAB / ECAB stage of a Re-schedule, the Review stage on
    entering Review, a customer stage) is kept; the superseded customer stage of a
    Re-schedule stays as a cancelled record. Examples: Review -> Customer Review /
    Closed / Rollback / Canceled cancels the Review approvers; leaving Customer
    Approval cancels the customer's. Request Approval (New -> Assess provisions
    Peer for `assess`; an Emergency's New -> Authorize provisions ECAB for
    `authorize`; Standard has no stage) is unaffected.
  * **Decision guard** -- `DecideChangeRequestApproval` resolves the caller's
    pending stage (their oldest `REQUESTED` row on a stage decidable in the
    current state, else their oldest one) and, when that stage's kind has a
    decidable state and the change is in another *known* state, refuses with a
    **409** `ConflictError` and changes nothing: `this approval is no longer
    pending: the change request is in <State>, but the <Stage> stage can only be
    decided while it is in <State>` (e.g. `... is in Closed, but the Review stage
    can only be decided while it is in Review`). The who-may-decide checks
    (creator, internal-only) come first. This covers rows the reconcile never saw
    (written before it existed, or by a path that does not run it -- e.g. a
    direct database write or the ServiceNow sync). The decision's UPDATE is narrowed to the
    resolved stage, so a caller holding a stale row and a live one decides only the
    live one. The BFF passes the 409 message through on the decision endpoint
    (`mapApprovalDecisionError`), as it does a 403's.
  * **`canDecide`** is `false` for a `REQUESTED` row whose stage's decidable state
    is not the change's current state (`markCanDecide`), so the webapp (which
    renders Approve / Reject from `canDecide`, never from the state) disables them.
  * **Migration 0193** (`0193_change_request_cancel_stale_approvals.sql`) is the
    data fix for rows written before this: idempotent, it cancels every `REQUESTED`
    row (a) of any change that is `CLOSED` / `CANCELED` / `ROLLBACK`, and (b) of
    a stage with an explicit `checkpoint_label` (the map above, in a SQL `CASE`,
    legacy `Assess` / `Authorize` included) whose state differs from the change's
    current state. It never touches rows of a stage with a NULL / unrecognised
    label unless (a), a change with a NULL state, rows that are not `REQUESTED`, or
    the change request itself; `updated_by` is
    `migration:0193_change_request_cancel_stale_approvals`. It flags the session
    internal (`set_config('app.is_internal', 'true', false)`, cleared at the end)
    because `approval_stage_approver` is FORCE row-level secured. Safe to re-run.

  Tests: `change_request_stale_approvals_integration_test.go`
  (`TestChangeRequestFlowIntegration_StaleApprovals_*`: the Review -> Customer
  Review -> Closed lifecycle with stages / row statuses / `canDecide` after every
  step, Review -> Closed, Roll back, Cancel from every state, the Re-schedule loop,
  Emergency / Standard unaffected, the guard on crafted legacy rows, unguarded
  ServiceNow-style stages, the GitHub state write, the migration) and the unit tests
  `TestApprovalStageDecidableState*` / `TestApprovalStageOutOfState` /
  `TestStaleApprovalRefusal` in `change_request_repo_test.go`.
* **Approver pools are INTERNAL-only.** Every internal stage (Peer, CAB, ECAB,
  Review) is decided by WSO2 staff, who see every project; an external
  (customer) user sees only the projects they are a registered contact of, so an
  approver row for one could never be found, let alone decided. A pool is
  therefore filtered, when it is resolved, to **active** (`"user".is_active`,
  NULL counting as active) users whose **`"user".user_type = 'INTERNAL'`** (the
  type `recompute_user_type()` derives from the `internal`/`admin` roles;
  `EXTERNAL`, `SYSTEM` and `NOT_AVAILABLE` users are never eligible) —
  `internalApproverIDs` / `onlyInternalApprovers` in
  `change_request_approval_flow.go`, applied to the peer pool (assigned group and
  the `Devops Approval` fallback), the CAB / ECAB groups and the Review pool. The
  same test is applied again at decision time (`approverDecisionBlock`, also what
  drives `canDecide`): a non-internal user holding an internal-stage row is
  refused with a 403 (`only active internal (WSO2) users can approve or reject
  the <stage> stage ...`) and gets `canDecide=false`. The customer stages
  (below) are the exception: their approvers are the project's registered
  customer contacts, external by nature. An internal stage whose group has
  members but no eligible internal one is a 400 that says so (`... has no active
  internal (WSO2) members to provision as <stage> approvers: external/customer
  users and inactive users cannot approve an internal stage`).
* **Approver pools.**
  * *Peer Approval* — Normal only. **Every active internal member of the change's
    assigned group** (`team_member.group_id`), whatever team type that group is.
    The creator is still listed, as a `CANCELLED` row, and never counts towards
    the pool. Who is experienced enough to peer-approve is decided when people
    are added to the group (membership management), **not** when the stage is
    provisioned. The pool is the **`Devops Approval`** group
    (`domain.PeerApprovalFallbackGroupName`, the ServiceNow flow's peer approval
    group, same rules) only when there is no assigned group or the assigned group
    yields nobody eligible (no active internal member other than the creator).
    Neither → 400 "no eligible peer approvers".
  * *CAB Approval* — Normal only, **right after** peer approval, its own
    group (`CAB Approval`). Provisioned inside the peer-approval decision's
    transaction; if it cannot be (nobody eligible) the peer decision is **rolled
    back** with a 400 rather than stranding the change in Authorize. Request
    Approval also pre-validates the CAB pool so the failure is early.
  * *ECAB Approval* — Emergency only, **its own group** (`ECAB Approval`), the
    only stage (no peer approval, no CAB).
  * Standard: no stage.
  * *Review* — assigned team, provisioned on a `{state: "review"}` PATCH once
    exactly two stages exist, i.e. Normal only. Its approvers can only decide
    while the change is in `review`: moving on (Customer Review / Closed /
    Rollback / Canceled) cancels the rows nobody answered.
* **Local seed personas** (`scripts/csm-compose/seed-entity-service.sql`) — a
  separate set of people for exercising the approval and customer-approval flows
  locally, so the fixtures do not hang on jane.doe / john.smith (whose rows stay:
  cases, time cards, the customer portal and other seed data use them). All on
  `example.com`; `user_type` is *derived* from the role by
  `recompute_user_type()`, not set by hand.

  | Persona | Email | Role → `user_type` | Seats |
  |---|---|---|---|
  | Alice Perera | `alice.perera@example.com` | `internal` → INTERNAL | group "Example Corp ABT" (901, the assigned group of every fixture), "CAB Approval", "ECAB Approval", "Devops Approval"; peer approver on CHG-FIXED-003 (requested) / -004 (approved) |
  | Bob Fernando | `bob.fernando@example.com` | `internal` → INTERNAL | same groups; peer approver on -003 (requested) / -004 (cancelled) |
  | Carol Silva | `carol.silva@example.com` | `internal` → INTERNAL | same groups; peer approver on -003 (requested) / -004 (cancelled) |
  | Dave Mendis | `dave.mendis@example.com` | `customer` → EXTERNAL | registered `PORTAL_USER` contact of project 401 "Example Corp Production"; Customer Approval approver on CHG-FIXED-007, Customer Review on -008 (requested) |
  | Erin Jayawardena | `erin.jayawardena@example.com` | `customer` → EXTERNAL | same as Dave |
  | Mira Santos | `mira.santos@lumenworks.example` | `customer` → EXTERNAL | registered `PORTAL_USER` contact of the generated project **"Lumen Works Platform"** (found by name — its id is random per database; a no-op where no such project exists, picked up by the next seed run after the seed-generator created it): its Customer Group, the people asked at Customer Approval / Customer Review |
  | Noel Prasad | `noel.prasad@lumenworks.example` | `customer` → EXTERNAL | same as Mira |

  * **Who can give the customer's answer, locally**: only the customer personas -- dave and
    erin (project 401), mira and noel (Lumen Works Platform) -- in the customer portal, on
    the change requests of their own project. alice, bob and carol (internal) are the CAB /
    peer approvers and the staff in the CSM portal: on CHG-FIXED-007 / -008 (or any change in a
    customer state) their `{state: "scheduled"}` / `{state: "closed"}` and their
    `isCustomerApproved` / `isCustomerReviewed` are 400s, and `POST .../approvals/decision` is a
    403 while the stage is live (a 404 when none is); there is no bypass. They can Cancel,
    Re-schedule (`customer_approval`) and Roll back (`customer_review`, when nobody is asked).
  * jane.doe (internal) is the requester persona: still a *team* member of Example
    Corp ABT (so `/users/me` and `GET /teams/{id}/members` keep working) but
    deliberately out of the *group* (`team_member.group_id` NULL), and in no CAB /
    ECAB / Devops group. john.smith (a customer) is still in the assigned group on
    purpose — the standing probe of the INTERNAL-only pools: Request Approval on
    CHG-FIXED-002 provisions alice, bob and carol, never john. Neither is a
    registered contact of project 401 any more (Other Corp's sam.other is
    unchanged).
  * The seed also removes the **old group-based Customer Group** (groups 911 / 912,
    "Example Corp Customer Approvers", with their memberships) from a database seeded
    by an earlier version: the Customer Group is now the project's registered contacts
    and nothing references those groups. Re-run the seed with
    `docker-compose up -d migrate`.
  * The **`Devops Approval`** group (the peer fallback) is seeded with the three
    internal personas — it is created only when no group of that name exists.
  * **The local stack connects as `csm_app`, not `postgres`.** A superuser skips every
    row-level-security policy, `FORCE ROW LEVEL SECURITY` or not, so with entity-service on
    `postgres` the `work_item` project-membership policies never applied: a customer could read
    another project's change request (and its approver list) by id. `migrate-and-seed.sh` creates
    `csm_app` (`NOSUPERUSER NOBYPASSRLS`) and re-grants it after every migration pass;
    `docker-compose.yml` points entity-service's `DB_USER` at it. Migrations and seeds still run as
    `postgres`. Verified as `csm_app`: dave asking for a Lumen Works change request gets 404 on the
    detail, the PATCH and the decision (200 with an empty list on `/approvals`); mira likewise for
    project 401; each customer reads and answers their own. Integration tests that use
    `CHANGE_REQUEST_TEST_DSN` still connect as `postgres` (a superuser), so they see the
    superuser behaviour.
  * **Local fixtures are dev-only, and the sync owns the schema.** Files under
    `scripts/csm-compose/fixtures/` are named for the migration they follow and run straight
    after it (recorded as `fixture:<version>` in the stack's own `schema_migrations`; safe to
    re-run). They load rows ServiceNow would supply (`0031_project_type_table.sql`, the
    `0121_timezone_table.sql` stand-in time zones) or converge an OLD local volume to the
    sync's table (`0136_change_request_deployment_table.sql`: a volume built before the sync's
    0136 was mirrored has that junction without its `id`, and the sync's `CREATE TABLE IF NOT
    EXISTS` leaves it so). They are not migrations and nothing outside the compose stack runs
    them; they never declare a table, column or type for the repo, because csm-sync-service's
    ServiceNow schema is the authority and `entity-service/migrations/` only mirrors it.
  * **Traps** (both from `seed-team-schedule.sql`): never grant the `internal`
    role to a customer (`recompute_user_type()` checks internal before external,
    so that customer becomes INTERNAL and gets unrestricted scope), and the
    personas' `user_role` rows carry **no** `created_by` — that file deletes every
    `created_by = 'seed'` internal grant of anyone outside its own roster.
  * **Self-healing.** Most seed rows are `ON CONFLICT DO NOTHING`, which would
    leave a database seeded before the personas on jane/john for ever. The
    personas and the eight `CHG-FIXED-*` fixtures are therefore upserted /
    deleted-and-reinserted: re-running the seed (`docker-compose -p <project> up
    -d migrate`, which runs it every time) removes jane/john's contact, CAB/ECAB and
    approver rows, installs the personas' and **resets the fixtures to their
    starting state** (state, stamps, stages, approvers), so no volume wipe is
    needed. The Playwright suite re-runs the seed before it starts.
  * **Who the customer portal's real-stack specs play**
    (`apps/customer-portal/webapp/tests/e2e/specs/local/`, "State-changing local specs" in its
    README). dave and erin answer, reject, review and propose on **CHG-FIXED-007 / -008**, and on
    **CHG-FIXED-005** (the Standard change: `PATCH {state: assess}` by alice through the CSM BFF
    puts it in Customer Approval with both contacts asked). **alice** is the CAB approver who
    approves a customer's proposed window (`POST /change-requests/{id}/approvals/decision`) so the
    change returns to Customer Approval; **mira** (Lumen Works Platform) is "a customer of another
    project" and must get 404 on project 401's change requests. The specs re-seed in the stack's
    Postgres (`E2E_POSTGRES_CONTAINER`, never a guessed name) before every test and after their
    file, and read raw rows back as `postgres`, which is why the fixtures' approver rows are asserted
    exactly: after one proposal loop the history is the cancelled Customer Approval rows, the CAB
    stage, then fresh Customer Approval rows for dave and erin. Their planned dates are NULL in the
    seed, so a proposal there is a whole new window. entity-service reads the zone-less
    `YYYY-MM-DD HH:MM:SS` both portals send as UTC itself (`change_request_window.go`), not in the
    database session's `TimeZone`, so the specs hold on a database set to any zone.
  * **What the customers can see in the customer portal.** The portal's Operations
    menu (Service requests, Change requests) comes from `GET /projects/{id}/features`,
    i.e. the project's `project_type` flags `has_service_request_read_access` /
    `has_change_request_read_access` (migration 0130). The local fixture row
    "Subscription" (`…a3`, the type of projects 401 / 402) is created with every flag
    FALSE, so the seed **sets those two flags** (an `UPDATE` of that one
    `created_by = 'local-fixture'` row, so it also corrects a database seeded earlier;
    no other flag or type is touched), and moves **"Lumen Works Platform"** (random
    project type, per database) onto "Subscription" when its type does not grant change
    request read access. (`"user".timezone`, which `GET /users/me` selects, is the
    sync's: `0122_user_add_timezone.sql` adds it, so the seed no longer does.)
  * **How each caller is scoped in the compose stack** (`AccessService.ResolveScope`;
    `docker-compose.yml`'s entity-service block). A caller that is in none of
    `M2M_CLIENT_IDS` / `CSM_PORTAL_BACKEND_CLIENT_ID` / `CUSTOMER_PORTAL_BACKEND_CLIENT_ID`
    is resolved from its forwarded `x-user-id-token` alone (INTERNAL → everything,
    EXTERNAL → the projects they are a registered contact of), which is how both portal
    backends already behave here; `CUSTOMER_PORTAL_BACKEND_CLIENT_ID` makes that a
    guarantee for the customer backend (it stays scoped even if also listed in
    `M2M_CLIENT_IDS`). `CSM_PORTAL_BACKEND_CLIENT_ID` / `CSM_PORTAL_USER_DOMAIN` stay
    unset: the staff and customer personas share `example.com`. `M2M_CLIENT_IDS` lists only
    the pure machine-to-machine services (`csm-integration-service`,
    `csm-notification-service`); the old `AUTH_INTERNAL_CLIENT_IDS` is read by nothing.
  * Tests: `TestChangeRequestSeedIntegration_*` (personas, fixture approvers, the
    seeded assigned group / CAB / ECAB end to end, the Devops fallback, and the
    seed's self-healing from the old shape inside a rolled-back transaction),
    `_SeedCustomerGroupFixtures`, `_CustomerPortalEntitlements` (the project type
    flags and Lumen's type, from the old shape, twice, with nothing else touched).
* **`CAB Approval` and `ECAB Approval` groups** are created by migration 0188
  (fixed ids `00000000-0000-4000-8000-00000000ca01` / `…eca1`) only when no group
  of that name exists, idempotently; membership is NOT seeded (synced from
  ServiceNow or set by an operator; the local compose seed adds the three internal
  personas — see "Local seed personas").
  Groups are resolved **by name**; members are `team_member.group_id` (and
  `team_member.team_id` of a team with that name, which is how the CR-notice
  flow addresses them).
* **The creator may not approve at any stage** (peer, CAB, ECAB) — they may
  still cancel. The creator is the user whose email is `work_item.created_by`
  or who is `change_request.requested_by_user_id`. They are provisioned
  `CANCELLED` where they are in a pool, and `DecideChangeRequestApproval`
  refuses them with a 403 even if a `REQUESTED` row exists.
* **Only active internal users may decide an internal stage** — not
  provisioned, and refused (403) at decision time even if a stale row exists
  (e.g. a customer who was a member of the team before the pools were
  INTERNAL-only). There is no endpoint in this repo that edits group membership
  (it comes from the ServiceNow sync), so provisioning + decision time are the
  enforcement points.
* **`canDecide`** on each approver in `GET /change-requests/{id}/approvals` is
  true only on the calling user's own `REQUESTED` row when they may actually
  decide it (not creator; an active internal user on an internal stage; and the
  change is in the state the row's stage belongs to -- see "An approval is only
  actionable in its stage's state"). Additive, advisory; the
  decision endpoint re-checks. Postgres data source only.
* **Rejections** of the internal stages keep the existing behaviour: siblings
  cancelled, no state change in either direction. (A *customer contact's*
  rejection does move the change — see "Customer Group" below.)
* Stage labels (`approval_stage.checkpoint_label`) are now `Peer Approval`,
  `CAB Approval`, `ECAB Approval`, `Review`, plus the customer group's (project contacts') `Customer
  Approval` / `Customer Review`; pre-existing `Assess`/`Authorize`
  labels (and unlabeled positional stages) are still recognised as peer/CAB.

### Approval stages mirrored from ServiceNow (no `checkpoint_label`), and "nobody is eligible"

A stage this service provisions always has its `checkpoint_label`. A stage with **none** is
one csm-sync-service mirrored from ServiceNow (`sysapproval_group` / `sysapproval_approver`),
i.e. migrated data: no label, an approver whose `"user"` row may be missing, an
`assignment_group_id` that is the CAB's, some other group, or NULL (an unsynced ServiceNow
group also yields NULL, so NULL is **not** a marker of anything). `checkpoint_label` is left
exactly as it is (keeping or dropping it is decided once the ServiceNow table is reconciled);
this section is about how such a stage is *read* at runtime, using only existing columns.

It used to be read by its zero-based position alone (`classifyApprovalStage`: 0 = Peer, 1 =
CAB, anything else unknown). A position is only a guess, and a wrong guess acted on data
nobody could explain: an Emergency change in Authorize whose single synced stage sits at
position 0 had a **Peer** stage in Authorize, so its approver's decision was refused as stale
(409) and `canDecide` was false; a stale position-0 stage of a Normal change that had moved
on was **cancelled** by the next state change.

`runtimeApprovalStageKind(label, position, groupName, model, state)` is the runtime reading,
used by `reconcileStaleApprovers`, `approvalStageInfo` (so also `callerPendingApprovalStage`
and the decision itself) and `markCanDecide`. A stage with a label is read by it, as before.
A stage with none is read as the first of these that applies, **and the result counts only in
the state it is decided in** (`approvalStageDecidableState`: Peer in Assess, CAB / ECAB in
Authorize), otherwise it is `stageKindOther`:

1. its own assignment group: the group named `ECAB Approval` -> ECAB, `CAB Approval` -> CAB;
2. an Emergency change in Authorize has no peer stage and no CAB stage -> ECAB;
3. the positional guess (0 Peer, 1 CAB).

`stageKindOther` is not tied to any state: it is **never cancelled** by a state move
(`reconcileStaleApprovers`; a finished change -- Closed / Canceled / Rollback -- still cancels
every requested row, unchanged), **never refused as out of state**, shown decidable to its
REQUESTED approver (the creator rule still applies, and so does the internal-only rule for the
kinds that need it: nothing is loosened) and a decision on it is recorded with no state
cascade. **Customer kinds are never inferred for an unlabeled stage** (fail closed): the
customer's answer is only ever accepted on a stage this service wrote with its label, a synced
stage that names a project contact stays an ordinary stage, `customerCanAnswer` stays false for
it, and the customer portal's approvals read cuts *every* unlabeled stage down to its label and
status (`redactInternalApprovalStages` recognises the customer's stages by label alone).

Deviation from the design note this was built from, on purpose and stricter in the safe
direction: the note read the group-name rule (1) as unconditional; here it also requires the
state to match, so a CAB-group row on a change that has already been scheduled is not
cancelled by the move on. In Authorize the two readings are identical.

*Display is unchanged* (the approvals read still labels an unlabeled stage positionally --
"Assess", "Authorize", then "Customer Approval" / `DYNAMIC_CONTACT` -- which is how a synced
Review stage at position 2 reads to staff; `changeRequestApprovalStagePosition` is the display
fallback and is not the runtime reading). Left alone on purpose: the labels are what the
portals have always shown for synced data.

**Diagnosable "nobody eligible".** Approver pools stay INTERNAL-only (an active user whose
`user_type` is `INTERNAL`; `internalApproverIDs` is untouched). What changed is the refusal's
message: when Request Approval is refused because the peer pool, the CAB / ECAB group or the
assigned team of the Review stage yields nobody, it now ends with counts of that group's
members and why none counted -- by `user_type` (`NOT_AVAILABLE`, `EXTERNAL`, `SYSTEM`, none),
inactive, no user record, the creator -- never names
(`describeExcludedMembers`/`noInternalMembersError`), e.g. `the "CAB Approval" group has no
active internal (WSO2) members to provision as CAB Approval approvers: external/customer users
and inactive users cannot approve an internal stage (the "CAB Approval" group: 14 members, none
eligible: 9 user_type NOT_AVAILABLE, 3 inactive, 1 external, 1 creator)`. The peer pool lists
each group it tried (the assigned group, then `Devops Approval`).

**Audit (read-only, no migration).** Run against a synced environment to see what the reading
above changes there: stages with no label, by the change's type and state, the position guess,
what they are read as now, how many have a REQUESTED approver, how many the position-only reading
would have refused and cancelled (`was_guarded_before`), and how many have an approver with no user:

```sql
WITH staged AS (
  SELECT ast.id, ast.work_item_id, g.name AS group_name,
         (SELECT COUNT(*) FROM approval_stage earlier
           WHERE earlier.work_item_id = ast.work_item_id
             AND (earlier.created_on, earlier.id) < (ast.created_on, ast.id)) AS pos,
         COALESCE(cr.change_model::text, '') AS model, COALESCE(cr.state::text, '') AS state,
         EXISTS (SELECT 1 FROM approval_stage_approver a WHERE a.stage_id = ast.id AND a.state = 'REQUESTED') AS has_requested,
         EXISTS (SELECT 1 FROM approval_stage_approver a WHERE a.stage_id = ast.id AND a.approver_user_id IS NULL) AS has_userless
  FROM approval_stage ast
  JOIN change_request cr ON cr.id = ast.work_item_id
  LEFT JOIN "group" g ON g.id = ast.assignment_group_id
  WHERE COALESCE(ast.checkpoint_label, '') = ''
), classified AS (
  SELECT s.*,
         CASE s.pos WHEN 0 THEN 'PEER' WHEN 1 THEN 'CAB' ELSE 'OTHER' END AS positional,
         CASE WHEN s.group_name = 'ECAB Approval' THEN 'ECAB'
              WHEN s.group_name = 'CAB Approval' THEN 'CAB'
              WHEN s.model = 'EMERGENCY' AND s.state = 'AUTHORIZE' THEN 'ECAB'
              WHEN s.pos = 0 THEN 'PEER' WHEN s.pos = 1 THEN 'CAB' ELSE 'OTHER' END AS candidate
  FROM staged s
)
SELECT c.model, c.state, c.positional,
       CASE WHEN (c.candidate = 'PEER' AND c.state = 'ASSESS')
              OR (c.candidate IN ('CAB', 'ECAB') AND c.state = 'AUTHORIZE') THEN c.candidate
            ELSE 'OTHER' END AS kind_now,
       COUNT(*) AS stages, COUNT(DISTINCT c.work_item_id) AS changes,
       COUNT(*) FILTER (WHERE c.has_requested) AS with_requested_approver,
       COUNT(*) FILTER (WHERE c.has_requested AND c.positional <> 'OTHER'
                          AND NOT ((c.positional = 'PEER' AND c.state = 'ASSESS') OR (c.positional = 'CAB' AND c.state = 'AUTHORIZE'))
                          AND c.state NOT IN ('CLOSED', 'CANCELED', 'ROLLBACK', '')) AS was_guarded_before,
       COUNT(*) FILTER (WHERE c.has_userless) AS with_userless_approver,
       COUNT(*) FILTER (WHERE c.group_name IS NULL) AS no_group
FROM classified c
GROUP BY 1, 2, 3, 4
ORDER BY 1, 2, 3, 4;
```

On the local stack's database it returns no rows (nothing there is synced); it has not been run
against a synced environment from here.

Tests: `TestRuntimeApprovalStageKind` / `_NeverOutOfState` (the table, and that an unlabeled stage
can never read as out of state or as a customer stage), `TestExcludedMembersSummary`, and against
Postgres `TestChangeRequestSyncedStagesIntegration_*` with SN-shaped rows (an Emergency change in
Authorize with one stage at position 0; a CAB-group stage with no label after two others; a stale
position-0 stage on a change that moved on; a position-2 stage naming the customer's contact; an
approver with no user; the counts). Two older tests asserted the position-only reading and were
adapted: `TestChangeRequestIntegration_DecideApprovalDoesNotCascadeOutsideAssess` (the decision is
now recorded with no cascade instead of a 409) and the "unlabelled stage past the first two
positions" case of `StaleApprovals_UnknownStagesAreNotGuarded`.

### Opening an approval stage's assignment group (`GET /groups/{id}`)

The Approval tab's *Assignment group* is a link: it opens the group the stage was
provisioned from and lists who is in it, like ServiceNow's group form and its
"Group Members" tab. Code: `group_detail_repo.go` (`GetGroupDetail`),
`service/group_detail_service.go`, `handler/group_detail_handler.go`; route in
`routes.go`. PostgreSQL data source only (the route is not registered without a
pool, like `GET /teams/{id}/members`).

* **`GET /change-requests/{id}/approvals` gained `assignmentGroup`** on each stage:
  `{id, name}` of `approval_stage.assignment_group_id`, **`null` for the Customer
  Approval / Customer Review stages** (recorded against no group: their approvers are
  the project's registered contacts) and for any stage with no group. Additive:
  `approverName` is unchanged, and the ServiceNow data source always returns `null`.
* **`GET /groups/{id}`** (`id` is a `"group"` id, **not** a `team` id -- `POST
  /groups/search` lists the `team` registry) returns `{id, name, description, email,
  manager: {id, name}|null, members: [{id, name, email, userType, role}], total}`;
  absent parts are `null`, `members` is `[]` for a group nobody is in, `total` =
  `len(members)`. Unknown id is a 404, a malformed one a 400.
* **Who is listed is who the approval pools provision from**, so the page and the
  stage agree (apart from per-change exclusions such as the creator, who is
  provisioned cancelled but is still *in* the group). There are two shapes of pool and
  the page follows each: an **assigned group** (the Peer and Review stages) is
  `team_member.group_id = <the group's id>` and nothing else (`groupMemberIDs`) -- a
  `team` that merely shares the group's name adds nobody, because its members are not in
  the peer pool either (the seed's Jane Doe sits in the *team* "Example Corp ABT" and in no
  approval group); the **CAB Approval / ECAB Approval / Devops Approval** groups
  (`namedPoolGroups`) are resolved by name (`namedGroup`): any `team_member` whose
  `group_id` is a `"group"` of that name or whose `team_id` is a `team` of that name. A
  group row with no name matches on its own `group_id` only. **Only active INTERNAL users
  are listed** (`"user".user_type = 'INTERNAL'` and `is_active` not false, NULL counting as
  active) -- the very rule every pool applies (`internalApproverIDs`), so a customer who
  sits in a staff group, a deactivated user or a user with no derivable type is neither
  provisioned nor shown. (The group's own membership is untouched; the page just does not
  promise an approver the stage cannot have.) One row per user (the `lead` role of any of
  their rows wins), name order (case-insensitive; the name falls back to first + last
  name, then the email, so nobody is listed blank). If the pools change -- a new named pool,
  or a different eligibility rule -- change `namedPoolGroups` / `groupDetailMembersSQL`
  with them: the integration tests hold the page against `namedGroup` / `groupMemberIDs`
  narrowed by `onlyInternalApprovers`.
* **Internal callers only** (`RequireInternalCaller`, checked before the id is parsed):
  an external (customer) caller gets 403, so a customer's contacts are never
  enumerable here. The route is not RLS-scoped because `group`, `team_member` and
  `user` carry no row-level security.
* Tests: `group_detail_repo_integration_test.go` (real Postgres, `CHANGE_REQUEST_TEST_DSN`:
  members, order, inactive / customer / typeless users left out, duplicates, same-name
  team/group, unknown id, empty group, and the list held against `namedGroup` /
  `groupMemberIDs` narrowed by `onlyInternalApprovers`),
  `change_request_approvals_group_integration_test.go` (approvals carry the group;
  customer stage `null`; the CAB group page equals the stage's approvers; a customer in
  the assigned group is neither a peer approver nor on its page),
  `TestBuildChangeRequestApprovals_AssignmentGroup`, the service and handler tests.

### Customer project, deployments and deployment products

The change request form's **Customer Project**, **Deployments** (multi-select) and
**Deployment products** (read-only), plus **Category**, the read-only **Customer
Group** (see the next section), **Additional comments** (customer visible) and
**Work notes**. Code: `change_request_links.go` (all rules), `change_request_repo.go`
(create / `patchChangeRequestTx` / `GetChangeRequestByID`); migrations
`0191_change_request_project_links.sql` and
`0192_change_request_drop_environments.sql`. PostgreSQL data source only.

**There is no Environments field.** A deployment already *is* an environment
instance of a project (its role, Primary production / Staging / QA …, is
`deployment.type`, still returned as `type` by the lookup), so a separate
Environments selection only repeated the Deployments one. Migration 0191 first
added an `environment` catalogue and a `change_request_environment` join table
for it; migration **0192 drops both** (`DROP TABLE IF EXISTS`, idempotent) and
0191 itself is untouched (it may already be applied somewhere). `environmentIds`
is refused on every write path (create, PATCH, both services) with the 400
`environmentIds is no longer supported: deployments carry the environment`, and
`environments` is gone from the detail response and the lookup.

*Data model.*

| Field | Storage |
| --- | --- |
| Customer Project | `work_item.project_id` (already existed; create now writes it) |
| Deployments | `change_request_deployment (change_request_id, deployment_id)` |
| Deployment products | `change_request_deployed_product (change_request_id, deployed_product_id)` (the same rows the case form's Product picker lists), stored as a snapshot |
| Category | `change_request.category`; the enum gained `REGULAR_RELEASE_CLOUD`, `HOTFIX_RELEASE_CLOUD`, `DEVOPS`, `CLOUD_COMPUTING` so all 13 API values persist |
| Customer Group | **not stored**: derived live from the project's registered contacts (next section). `change_request.customer_group_id` (migration 0075) is kept but no longer written or read |
| Additional comments / Work notes | `comment` rows of type `COMMENT` / `WORK_NOTE`, `created_by` = the caller's email |

The join tables have FKs with `ON DELETE CASCADE`, a lookup index each and `FORCE ROW
LEVEL SECURITY` (project membership through `work_item.project_id`, like
`work_item_tag`; listed in `rlsProtectedTables`). Neither carried an UPDATE policy
(entity-service only deletes and re-inserts), but `change_request_deployment` is
also csm-sync-service's own junction (its migration 0136, the surrogate-id shape),
and the sync writes it with `INSERT … ON CONFLICT … DO UPDATE`, whose conflict
branch an RLS table with no UPDATE policy refuses for everyone. Migration
**0205** adds the internal-only UPDATE policy on it (the 0190 shape: no
project-member branch, so a customer session is refused as before; no column,
table or type changes). `change_request_deployed_product` is ours alone and stays
without one: `rlsCommandsDeniedOnPurpose` records why. The first deployment (name order)
and its first deployed product are mirrored into `work_item.deployment_id` /
`deployed_product_id` so list views keep working; `deploymentId`/`deployedProductId`
on PATCH cannot be combined with `deploymentIds`.

*Rules (identical on create, PATCH and the lookup).* Violations are 400
`ValidationError`s naming the field and id:

1. `projectId` must exist. `deploymentIds` require `projectId`; each deployment must
   exist, be active and belong to the project.
2. **Deployment products are read-only and derived**: always the active
   (`active IS NULL OR TRUE`) deployed products of the chosen deployments. A caller
   may state `deploymentProductIds`, but only as exactly that set (on PATCH also
   exactly the stored snapshot, so a re-sent value never fails because the
   deployment gained a product since); anything else is "deploymentProductIds is
   read-only: …". Without `deploymentIds` they are refused.
3. Each list holds at most 100 ids; duplicates are collapsed.
4. `customerGroupId` and `environmentIds` are refused (see above and below).

*PATCH.* Arrays replace. `deploymentIds` changed → deployments and products
(re-derived) are rewritten; `[]` clears them. Changing `projectId` while deployments
are stored requires `deploymentIds` in the same request. **Edit window:** deployments and
deployment products can change only while the change has not reached `implement` (states
new … scheduled); from implement/review/customer_review/rollback/closed/canceled a
*change* is a 400 ("<field> can no longer be changed: the change request is in state
…") while re-sending the stored value is accepted. They are resolved against the change's
project, which after New is the **frozen** stored one (the single-valued
`deploymentId` / `deployedProductId`, which skip the list rules, are held to the same
window and the same project: `checkSingularDeploymentFields`). **The Customer Project
itself moves in New only**: see "Customer requirements lock" below. Category and the
journal entries are not windowed. A refused
PATCH writes nothing (all in the PATCH transaction). `comment` / `workNote` append a
row each; blank is refused on PATCH and ignored on create. `durationInput` is still
unsupported on this data source.

*Create* runs validation, the insert, the two join tables and the two journal rows in
one transaction (all-or-nothing). The ServiceNow-first path refuses the removed
fields and validates the selection
(`ChangeRequestRepository.ValidateChangeRequestLinks`) **before** calling ServiceNow.

*Lookup.* `POST /change-requests/link-options {projectId, deploymentIds?}` →
`{deployments:[{id,name,type}], deploymentProducts:[{id,name,deployment}], customerContacts:[{id,name,email?}]}`
(`GetChangeRequestLinkOptions`): the project's active deployments, for the chosen ones
the products that follow — computed by the same derivation the writes validate against,
so what it offers is exactly what create accepts — and the project's registered
contacts (the read-only Customer Group, name order, `[]` when none). The route is **internal callers only**
(`internalOnly`, pinned by `TestChangeRequestLinkOptionsIsInternalOnly`): it takes a
project id and returns that project's deployments and customer contacts, so an
external caller must not be able to enumerate other projects through it.

*ServiceNow mirror (dual-write).* `customerGroupId` and `environmentIds` are **no
longer forwarded** (they are no longer accepted, so there is nothing to forward; the
ServiceNow-only service refuses them with the same 400 instead of sending them, and
no longer maps `customerGroup` / `environments` from a ServiceNow read). The ServiceNow
client types still carry `categoryKey`, `comment`, `workNote` (create) and `projectId`,
`comment`, `workNote` (PATCH), which keep being forwarded. They do **not** carry a
project or a deployment *list* (create has neither; PATCH has the single
`deploymentId`/`deployedProductId`), and the product ids PostgreSQL derives are not
ServiceNow records, whose field names and reference tables are not discoverable from
this repository. So `projectId` (create), `deploymentIds` and `deploymentProductIds`
are **stripped from the mirror** like the customer gate flags
(`changeRequestService.createChangeRequestSNFirst` / `PatchChangeRequest`); the
ServiceNow-only service refuses `projectId`/`deploymentIds` instead of dropping them.
Wire them once the ServiceNow field names are known. A change request created in
dual-write mode also gets its comment rows in PostgreSQL; if csm-sync-service syncs
the ServiceNow journal back it may add its own copies.

### Customer Approval / Customer Review checkboxes

Real ServiceNow's change request form has two checkboxes on creation, **Customer
Approval** and **Customer Review**. They are implemented as
`change_request.customer_approval_required` / `customer_review_required`
(migration `0189_change_request_customer_gates.sql`, `BOOLEAN NOT NULL DEFAULT
false`, idempotent) and the API fields **`customerApprovalRequired`** /
**`customerReviewRequired`** — accepted on `POST /change-requests` and
`PATCH /change-requests/{id}`, returned on the detail response (and the PATCH
receipt). Postgres data source only.

* **They are NOT `is_customer_approval_required` / `is_customer_review_required`.** Those two
  record the customer's *outcome* ("the customer has confirmed"): stamped only by
  the customer's own answer (`applyCustomerStageOutcome`, true only: never by staff,
  see "The customer's outcome flags are the customer's alone" below), and in the ServiceNow scripted API
  only writable while the change is in the matching state (`isCustomerApproved:
  false` there moves it to Cancelled, `isCustomerReviewed: false` to Rollback —
  see `EditChangeRequestDialog.tsx`'s doc comment). The new columns are the
  *requirement*. No code or comment in this repo ties the form's checkboxes to
  the existing columns (the older note that two records with both booleans
  `false` offered different branches points the other way), so they got columns
  of their own. The ServiceNow field names of the two checkboxes are **not
  discoverable from this repo** (the SN client talks to a Choreo API whose
  source is not here), so they are not mirrored: the dual-write mirror strips
  them from the PATCH it replays (and skips the mirror entirely when nothing
  else is in the PATCH); the pure ServiceNow data source ignores them.
* **Approval gate.** Wherever the flow moves a change to Scheduled — CAB / ECAB
  approval in `DecideChangeRequestApproval` (`approvalGateTarget`), or Request
  Approval on a Standard change (`requestApprovalDestination`; Standard has no
  internal approval to put the gate after, so the gate sits right after Request
  Approval — an assumption) — a change with `customerApprovalRequired` goes to
  **`customer_approval`** instead. There `legalNextStates` is `[authorize,
  canceled]`: only the **customer's own approval** (Customer Portal) schedules the
  change and stamps `is_customer_approval_required = true`; the human PATCH
  `{state: "scheduled"}` is refused (400, from every state, with or without
  `isCustomerApproved`, with or without anybody asked). Cancel is staff's side of
  the customer declining; the customer's own rejection also cancels. The
  `isCustomerApproved: false → Cancelled` behaviour belongs to the ServiceNow
  scripted API and, on the Postgres path, to the customer's own answer.
* **Review gate.** `review` offers `[customer_review, rollback, canceled]` when
  `customerReviewRequired`, else `[closed, rollback, canceled]` (**a behaviour change for
  rows that predate the checkbox: they default to false and Review now offers
  Closed directly instead of both**). A manual `{state: "customer_review"}` is
  refused when not required ("customer review is not required …"), and
  `{state: "closed"}` from `review` is refused when required ("customer review
  is required …; move it to customer_review first"). `customer_review` offers
  `[rollback, canceled]` (`[canceled]` while the contacts are being asked): only the
  customer's own review closes the change and stamps
  `is_customer_review_required = true`; a manual `{state: "closed"}` out of it is a 400.
  No other transition is graph-checked — as before, the PATCH does not enforce a full
  transition graph -- except out of the two customer states (see "There is no
  "Schedule" action" above).
* **Free in New, add-only afterwards** (`validateCustomerGateEdits`, checked under the
  row locks; the whole rule, with its table, is "Customer requirements lock" below). In
  New either box can be ticked and unticked. From Request Approval on a ticked box
  can never be unticked (400 `customerApprovalRequired can no longer be turned off: ...`),
  and an unticked one can still be ticked until the gate it controls has been passed --
  `customerApprovalRequired` while the state is New / Assess / Authorize,
  `customerReviewRequired` up to and including Review -- and only on a change that has
  a Customer Project **with somebody who can be asked** (below: 400 `customer approval is
  required but nobody on this project can be asked ...`). Past the gate a tick is the old 400 (`customerApprovalRequired
  can no longer be changed: the change request has already passed the approval stage
  (current state: X)` / `customerReviewRequired can no longer be changed: the change
  request has already left the review stage (current state: X)`); resending the stored
  value is always accepted. The flag in the same PATCH wins over the stored one for the
  state routing (`{state: "assess", customerApprovalRequired: true}` on a Standard
  change lands in `customer_approval`; closing a required Review in the same PATCH that
  unticks it is no longer possible: the way on is Customer Review).
* **Customer Group: who gives the customer's answer.** See the next section.
* Tests: `TestChangeRequestFlowIntegration_*CustomerGate*` /
  `*CustomerApproval*` / `*CustomerReview*` / `ManualScheduledOnlyFromCustomerApproval`
  (real Postgres, `CHANGE_REQUEST_TEST_DSN`), `TestLegalChangeRequestNextStates`,
  `TestCustomerGateHelpers`, the service tests
  `TestChangeRequestService_*CustomerGate*`, and the csm-portal BFF handler tests.

### Customer requirements lock (what can still be edited once approval is requested)

Decision (the product owner's, 2026-10-06): *who the customer is* (the Customer
Project, from which the read-only Customer Group is derived) and *whether the
customer is asked* (the two creation-form boxes `customer_approval_required` /
`customer_review_required`, the **requirements** -- not the `is_customer_*_required`
outcome stamps) are fixed when the change is created; Request Approval freezes the
project and makes the boxes add-only. Code: `change_request_customer_lock.go` (the
rules, pure functions), the creation-phase gate in `patchChangeRequestTx`,
`validateCustomerGateEdits`, `lockChangeRequestForPatch`, `planChangeRequestLinks`,
`checkSingularDeploymentFields`. **No column, no table, no enum, no migration, no
"reached" marker, no backfill.**

*Creation phase = the stored state is `NEW` (or NULL, a pre-lifecycle row, which counts
as New).* A change can never return to New, so "it is in New" already says "approval has
not been requested". The rule is a pure function of (stored state, stored value,
requested value, whether the change has a Customer Project).

| State | Project change | Project resend | Box off -> on (project stored) | Box off -> on (no project) | Box on -> off | `{state: "new"}` |
|---|---|---|---|---|---|---|
| New / NULL | ok | ok | ok | ok | ok | ok (no-op) |
| Assess, Authorize | **400 frozen** | ok | ok | **400 needs a project** | **400 cannot turn off** | **400** |
| Customer Approval, Scheduled, Implement, Review | **400 frozen** | ok | approval box **400 gate passed**; review box ok | approval **400 gate passed**; review **400 needs a project** | **400** | **400** |
| Customer Review, Rollback, Closed, Canceled | **400 frozen** | ok | **400 gate passed** | **400 gate passed** | **400** | **400** (Rollback, Closed, Canceled: "... cannot be moved") |

(`approvalRequirementEditable` = New / Assess / Authorize, `reviewRequirementEditable` =
everything up to and including Review: the cut-offs are the ones that existed before
the lock. "Project stored" is the **stored** project: none can be set after New. An "ok" in
the box off -> on column of a state after New is "ok **if somebody can be asked on that
project**": otherwise **400 nobody to ask**, rule 4b below, which needs the database and so
is not a column of the pure truth table.)
The same table is `TestCustomerRequirementsLock_TruthTable` (Go, unit, row ids
`<STATE>/<column>`, outcome codes `ok / frozen / cannot-turn-off / gate-passed /
needs-project / return-to-new / final`); the CSM Edit dialog computes the same
rule client-side from the stored (state, flag, hasProject) and keeps a Vitest table with
**the same row ids and outcome codes** (no file is shared between the modules). The server
stays the authority and answers 400.

The rules, in the order they are applied (**the first failing one wins, every refusal is a
400, a refused PATCH writes nothing -- not even the rest of the same request**):

1. `{state: "new"}` on a change that has left New -> `state "new" cannot be set: a change
   request that has left New cannot return to it. Cancel it and clone it instead.` (On a
   closed, canceled or rolled-back change the refusal is the final-state one, `state "new" cannot
   be set manually from closed: a change request that is closed cannot be moved`.)
2. `projectId` that differs from the stored one (NULL -> X included) after New ->
   `projectId can no longer be changed: the Customer Project is fixed once approval has
   been requested (current state: X). Cancel this change request and clone it to use
   another project.` The stored value resent is an accepted no-op (a client that sends the
   whole form back is not punished).
3. A box ticked -> unticked after New -> `customerApprovalRequired can no longer be turned
   off: once approval has been requested a customer requirement can be added but never
   removed (current state: X). Cancel and clone to correct it.` (`customerReviewRequired`
   likewise.)
4. A box unticked -> ticked after New: the gate's own cut-off first (`customerApprovalRequired
   can no longer be changed: the change request has already passed the approval stage ...`
   / `customerReviewRequired can no longer be changed: ... already left the review stage
   ...`), then, when the change has no Customer Project, `customerApprovalRequired cannot be
   turned on: this change request has no Customer Project, and one can no longer be set
   after approval was requested. Cancel and clone it with a project.`
4b. A box turned ON after New (rule 4 let it through: its gate is ahead and the change has a
   project) when the project has **nobody who can be asked** -> `customer approval is required
   but nobody on this project can be asked (no registered contact other than the requester):
   register a contact for the project first` (`customer review is ...` for the review box,
   `customer approval and customer review are ...` when both are turned on in the one PATCH --
   only the boxes being turned on are named). The change would otherwise reach a gate nobody can
   answer. Only the turning-on is judged (`checkTickedBoxCanBeAsked`): a box already ticked, a
   resend, an unticked box, and every other edit are never re-judged, whatever the project's
   contacts have become since.
5. Deployments / deployment products keep the until-implement window and must belong to
   the (frozen) project -- also `deploymentId` / `deployedProductId`, which used to skip
   the list rules: another project's -> 400 `deploymentId does not belong to the change
   request's project: <id>`; a change with no project takes none (`deploymentId requires a
   Customer Project`); a changed value past `implement` -> `deploymentId can no longer be
   changed ...`; the stored value resent is accepted in every state.
6. **Request Approval guard.** `{state: "assess"}` (the Request Approval action) on a change
   still in New is refused when the effective approval box or review box (the request's
   value, else the stored one) is set and the effective project (the request's `projectId`,
   else the stored one) is empty -> `approval cannot be requested: the customer's approval
   and/or review is required but no Customer Project is set, so there is nobody to ask.
   Select a Customer Project first (or clear the requirement).` For every type (a Standard
   change would otherwise land in Customer Approval with nobody to ask), in the same PATCH
   that clears the box or chooses the project it is accepted, and a RESEND of
   `{state: "assess"}` on a change that already left New is the idempotent no-op it always
   was, even for a legacy change that ticked a box with no project. (The CSM webapp mirrors
   the guard by disabling Request Approval with the reason "Select a Customer Project before
   requesting approval" through `TARGET_BLOCKED_REASON`; that is the CSM webapp's change, the
   server above is the authority.)
7. **Request Approval needs somebody to ask** (the product owner's decision, "Refuse Request
   Approval"). `{state: "assess"}` on a change still in New is refused when the effective approval
   box or review box (the request's value, else the stored one) is set, the effective project is
   set, and that project has **nobody who can be asked** -> `customer approval is required but
   nobody on this project can be asked (no registered contact other than the requester): register a
   contact for the project first` (`customer review is required but ...` for the review box,
   `customer approval and customer review are required but ...` when both are set). It runs AFTER
   rule 6, which keeps its own message and precedence for a change with no project. For every type
   that goes through Request Approval (Normal waits in Assess, Standard lands in Customer Approval
   or Scheduled); a refused request writes nothing, not even the rest of its fields. **Why:** with no
   staff action that answers for the customer (the Bypass is gone), a change that reaches Customer
   Approval / Customer Review with nobody to answer can only be cancelled (or rolled back from
   Review), an invitation to cancel and clone just to get past a gate.
   - **"Somebody who can be asked" is not a second definition: it is the one the stage
     provisioning uses.** `customerGroupCanBeAsked` = `customerContactUserIDs(project)` -- the
     project's `REGISTERED` portal-user contacts (`PORTAL_USER` project role) whose `"user"` row
     exists and is active (never an `INVITED` / `RE-INVITED` / `DEACTIVATED` contact, never one
     with only the `SECURITY_CONTACT` role, never one whose user is deactivated or missing) --
     and `changeRequestCreatorUserIDs` (the requester never approves their own change), judged by
     `anyContactToAsk`, which `provisionCustomerStage` and the read-only
     `legacyStageWouldBeProvisioned` call too. The refusal therefore predicts exactly the
     "nobody asked" outcome (`..._TheRefusalPredictsTheProvisioningOutcome` proves it, per
     situation, against the provisioning). The stored legacy `customer_group_id` is ignored, as it
     is by the provisioning.
   - **Reads our boxes only.** The requirement is `customer_approval_required` /
     `customer_review_required` (migration 0189), the very read rule 6 uses; the sync-owned
     `is_customer_*_required` and `customer_group_id` are neither read nor written (a migrated
     change in New -- `CHG` number, ServiceNow flags set, our columns false, an unlabeled synced stage --
     is unaffected: `..._AMigratedShapedChangeInNewIsUnaffected`). No new column, table or
     migration.
   - **What is NOT judged.** Only Request Approval (New -> Assess) and the turning-on of a box
     (rule 4b). A change already beyond New is never re-judged: a legacy row seeded in a customer
     state keeps Cancel / Roll back and Re-schedule exactly as before
     (`..._ALegacyDeadEndRowIsNotRejudged`). **Residual edge, documented and not built for:** every
     registered contact deactivated AFTER Request Approval. The change then reaches the gate with
     nobody asked, which is the dead end it always was (Cancel; Re-schedule from Customer Approval;
     Roll back from Customer Review; a contact who registers is asked when the stored `projectId` is
     restated) -- `..._ResidualEdgeContactsLeaveAfterRequestApproval`. Emergency changes are out of
     scope here (their boxes are forced off by their own rule); the refusal is type-blind and simply
     never has a box to judge on one.
   - Tests: `change_request_nobody_to_ask_integration_test.go` (the matrix box x situation x action,
     as the superuser, as `csm_app` and on a copy whose approval tables have the sync's enum
     columns), `TestNobodyToAskMsg` / `TestAnyContactToAsk` / `TestBoxesTurnedOnAfterNew` (pure),
     the service row in `change_request_service_lock_test.go`.

**The Re-schedule hole, closed.** A change that reached Customer Approval necessarily has
`customer_approval_required = true`; Re-schedule (a WSO2 user's `{state: "authorize"}` or a
customer's own proposal of a new time) sends a Normal / Emergency change back to Authorize
and keeps a Standard one in Customer Approval. Before the lock the box could be unticked in
Authorize (it was editable until the gate), so the CAB / ECAB approval that followed went
straight to Scheduled and the customer was never asked about the new plan. Now the box
cannot be unticked in Authorize (or anywhere after New), the project cannot be swapped,
and the change cannot be sent back to New, so the approval that follows asks the same
contacts again in a fresh stage (the superseded one stays as a record).
`TestChangeRequestLockIntegration_RescheduleCannotReopenTheCustomersApproval` proves it for
Normal, Emergency and Standard; `..._CustomerProposalCannotReopenTheApproval` for the
customer's own proposal; `..._CustomerReviewCannotBeReopened` for the review (a required
review cannot be skipped by unticking it to close from Review). Corrections after New are a
**cancel and a clone** (Clone is a create); there is no administrator override.

**Concurrency.** Request Approval and a project edit must not both read "New". The PATCH
that carries a state, a project, a box or a deployment field first locks the `work_item`
row (`SELECT ... FOR NO KEY UPDATE`) and **only then** reads the `change_request` side (state,
model, both boxes, project) in a statement of its own, then locks the `change_request` row:
the order (work_item, then change_request) every other PATCH takes. The strength matters:
a decision locks `change_request` first and then INSERTs `approval_stage` /
`approval_stage_approver` rows whose foreign keys take `FOR KEY SHARE` on the same
`work_item` row, which `FOR UPDATE` refuses -- the PATCH would wait for the decision and the
decision for the PATCH (`deadlock detected`, SQLSTATE 40P01; the lock the old
`planChangeRequestLinks` took, `FOR UPDATE OF wi`, had that hazard for the PATCHes that
carried a project or deployments). `FOR NO KEY UPDATE` is what the PATCH's own UPDATE of
`work_item` takes, excludes another PATCH and lets the decision's INSERT through
(`TestChangeRequestLockIntegration_APatchAndADecisionDoNotDeadlock`, which fails with 40P01
against `FOR UPDATE`). Reading the state in
the same statement as the lock does not work: under READ COMMITTED a join read is answered
from the statement's own snapshot, which is older than the lock's grant (the old
`planChangeRequestLinks` did exactly that). `TestChangeRequestLockIntegration_RequestApprovalRacingAProjectEdit`
stands a transaction in for the Request Approval in progress (it holds the lock and has
moved the state to Assess, uncommitted), starts the project edit, waits until it is blocked
on the lock and only then commits the first one: the edit must be refused, and the test
**fails against the one-statement lock-and-read** (verified by mutation).

**Every write path of these fields** (re-grepped for `UPDATE work_item ... project_id`,
`customer_approval_required =`, `customer_review_required =` and the state writers):

| Writer | What the lock does |
|---|---|
| `PATCH /change-requests/{id}` (`patchChangeRequestTx`) | the only code that updates `work_item.project_id` / the two requirement columns of an existing change: all the rules above (1-7, including 4b) |
| `POST /change-requests` and `CreateChangeRequestFromServiceNow` (SN-first create) | create in New: free (a ticked box with no project, or on a project nobody can be asked on, is accepted; Request Approval is what refuses it). **Clone** is a create. |
| GitHub `CreateFromIssue` | creates in New with the project the issue maps to |
| GitHub `SetState` (`github_mutation_repo.go`) | writes `state` by SQL (it could write NEW); **no caller exists** in the repository; it refuses to move a change out of Customer Approval / Customer Review (a label or an issue event must not answer for the customer), and is otherwise not guarded and not a lock concern until a caller exists |
| `DecideChangeRequestApproval` / `applyCustomerStageOutcome` | write the `is_customer_*_required` **outcome stamps** (true only) and the state moves; never the requirement columns or the project |
| `scripts/csm-compose/seed-entity-service.sql` | the dev fixtures CHG-FIXED-005..008 are upserted (`ON CONFLICT DO UPDATE`) as a reset; every ticked fixture past New has a project (`..._CreatesAreFreeInNew` asserts it) |
| `seed-generator/generate_workitems.go` | random rows: writes only the outcome stamps |
| csm-sync-service (the ServiceNow sync) | see below: not an API caller |

**The outcome flags are the customer's alone, and stay one-way** (verified, no hole):
no staff request can write them any more (`refuseStaffCustomerOutcomeFlags` refuses
`isCustomerApproved` / `isCustomerReviewed` from anyone but the customer, true or
false), the customer's own answer refuses a rejection of a flag already true, and
`applyCustomerStageOutcome` only ever sets true. No create path writes them; the only
writers of `is_customer_(approval|review)_required` are `applyCustomerStageOutcome` (the
customer's answer) and the two dev seed scripts (fixtures and random rows, not a runtime
path).
`TestChangeRequestLockIntegration_OutcomeFlagsStayOneWay`,
`TestChangeRequestNoBypassIntegration_*`.

**ServiceNow dual-write and the sync -- how these writes happen, and the decision.**

* *PATCH is PostgreSQL-first.* `changeRequestService.PatchChangeRequest` calls the repository,
  all rules above run inside its transaction, and only **after commit** is the asynchronous
  mirror dispatched (`snWriteback.Dispatch` -> `snMirror.PatchChangeRequest`) with the request
  minus the PostgreSQL-only fields (both boxes, `deploymentIds`, `deploymentProductIds`, the
  expected window). A refusal returns before the dispatch: **nothing is mirrored and no writeback
  failure is recorded** (`TestChangeRequestService_PatchChangeRequest_RefusalIsNeverMirrored`).
  `projectId` is forwarded as before; it can only arrive as a resend after New. The mirror now
  also gets `plannedStartOn` / `plannedEndOn` in ServiceNow's `YYYY-MM-DD HH:MM:SS` (UTC) layout
  (`repository.PlannedTimestampForServiceNow`): PostgreSQL accepts RFC 3339 too, which
  ServiceNow's service refused, so an RFC 3339 PATCH used to commit and then fail every mirror write.
  **The ServiceNow service takes RFC 3339 itself now** (`snPlannedTimestamp`, which calls that same
  function): on a PATCH (`plannedStartOn` / `plannedEndOn`) and on a create (`plannedStartDate` /
  `plannedEndDate`, and the `durationInput` cross-check) a value with a zone designator is converted to
  the zoneless UTC layout before it is forwarded, a `YYYY-MM-DD HH:MM:SS` value is forwarded as sent
  (whatever its year: that service never bounded it), and everything else (`infinity`, `now`,
  `tomorrow`, a date alone, a zone name, an RFC 3339 value outside the years 2000 to 2100) is the same
  400 as before (`<field> must follow the format: YYYY-MM-DD HH:mm:ss`) with no downstream call. So the
  pure ServiceNow data source and the ServiceNow-first create (which hands ServiceNow the text the
  caller sent, validated, not converted) accept both layouts, as the PostgreSQL data source does
  (`TestSNPlannedTimestamp`, `TestSNChangeRequestService_PatchChangeRequest_PlannedWindowLayouts`,
  `TestSNChangeRequestService_CreateChangeRequest_PlannedWindowLayouts`). The expected window of a
  customer's answer is still PostgreSQL's alone: the ServiceNow service ignores the two fields.
* *Create is ServiceNow-first* (`createChangeRequestSNFirst`): type, scope and (new) the planned
  window are validated before ServiceNow is called; ServiceNow gets no project / deployments / boxes;
  the PostgreSQL insert (`CreateChangeRequestFromServiceNow`, now normalising the window like the
  portal create) is in New, where nothing is locked.
* *csm-sync-service is a separate service* (its repository is not in this checkout; what is known
  is from the 0190 / 0205 migration headers and entity-service's own notes): it writes its tables
  with plain SQL as an internal caller (`app.is_internal = true`) and **never goes through
  `PatchChangeRequest`, `validateCustomerGateEdits` or any validator here**, so none of the
  refusals can break it. It can move `project_id` or `state` of a change after New (an edit made
  in ServiceNow); the lock does not and cannot stop that, and everything here that depends on the
  project reads the change's *current* one (the Customer Group is derived live; a stale
  designation fails closed). **Decision: the refusals bind API callers only, and no database
  trigger is added** (it would also fire for the sync, whose writes are the source of truth for
  migrated rows, and the user ruled out schema changes). Not verified here (the sync's source is
  not in the checkout): whether it overwrites `customer_approval_required` /
  `customer_review_required` (our 0189 columns, which it has no reason to know) and whether its
  `delete_sync` covers `approval_stage_approver`.
* *Existing rows* are not migrated: a change that is already past New keeps whatever its boxes and
  project are; the rule applies to the edits made from now on, by state. A legacy change that ticked
  a box with no project cannot be given one any more -- cancel and clone.

Tests: `change_request_customer_lock_test.go` (the truth table, rule order, case-insensitive
project resend, the Request Approval guard), `change_request_customer_lock_integration_test.go`
(everything above against Postgres, as the superuser and as `csm_app`),
`change_request_service_lock_test.go` (a refusal is never mirrored). Old tests that encoded the
earlier rule were changed, not deleted: the two "editable until the gate" tests (an untick after New
is now a 400), `CloseFromReviewHonoursTheFlagInTheSamePatch` (unticking in the closing PATCH is
refused; the way on is Customer Review), `PatchEditWindow` (the project moves in New only),
`CustomerGroupFollowsTheProject` (rewritten: the stage follows the contacts of the frozen
project), and every test that ticked a box on a change with no project (`createGated` puts it on a
project with no registered contacts: nobody to ask, and nobody who may answer for the customer --
and Request Approval now refuses that, so a test that needs the dead end uses
`requestApprovalThenContactsLeave` (a stand-in contact that leaves right after Request Approval:
the residual edge) or `legacyInCustomerState` (a row written with SQL, as an older build left it);
the rest moved to a project with contacts, `createAnswerable`).

### Customer Group: approving / rejecting Customer Approval and Customer Review

A change request's **Customer Group** is **not a group you pick**: it is
derived, live and read-only, from the change request's **Customer Project** — the
project's **registered contacts** (`customerContacts` on the detail response and on
`POST /change-requests/link-options`; the UI label stays "Customer Group"). It is
never stored, so it can never point at another customer's people: a contact belongs
to exactly the project it was registered on. When the change reaches
`customer_approval` / `customer_review`, **those contacts are asked**: a "Customer
Approval" / "Customer Review" stage with one `REQUESTED` row per contact appears in the
change request's approvals, and they answer it **in the customer portal** (customers do
not sign in to the CSM portal, whose Approvals tab only *shows* the stage and its
outcome). Code: `change_request_links.go`
(`customerContactsSQL`, `loadProjectCustomerContacts`, `customerContactRefs`),
`change_request_approval_flow.go` (`provisionCustomerStage`,
`applyCustomerStageOutcome`, `customerStageSpec*`).

* **Who is a customer contact.** A `project_contact` of the change request's
  `work_item.project_id` in state **`REGISTERED`** holding the **`PORTAL_USER`**
  project role (through `project_contact_group` → `project_group_role` →
  `project_role` — the very chain `callerIsRegisteredPortalContact` uses
  for "a registered contact with role X on project Y"), whose `"user"` is active.
  The name/email/user come from `account_contact.user_name` matched
  case-insensitively to `"user".user_name`, as in `ProjectContactRepository`
  (a contact with no `"user"` row is listed with its contact email but cannot
  hold an approval). `INVITED` / `RE-INVITED` / `DEACTIVATED` contacts, contacts
  holding only `SECURITY_CONTACT`, and deactivated users are not in the group.
  Empty (`[]`, never null) when the change request has no project or the project
  has no such contact. `change_request.customer_group_id` (migration 0075) is no
  longer written or read; a stored legacy value is ignored (also for approvals).
* **API.** `customerGroupId` is **no longer accepted** on create or PATCH (any
  value, `null` included): 400 `customerGroupId is no longer accepted: the customer
  group is derived from the customer project's registered contacts`
  (`RejectRemovedCreateFields` / `RejectRemovedPatchFields`: in the service before
  anything else — so before ServiceNow is called — and again in the repository).
  The detail response drops `customerGroup` for `customerContacts:
  [{id (project_contact.id), name, email?}]` (name order).
* **Eligible approvers** = the contacts' active users minus the CR's creator (listed
  `CANCELLED` like on every other stage; they can never decide). The
  INTERNAL-only rule does **not** apply to customer stages (the contacts are
  external).
* **Stages.** Entering `customer_approval` writes an `approval_stage`
  `checkpoint_label = "Customer Approval"`, **`assignment_group_id` NULL** (the
  group is not a `"group"` row), one `REQUESTED` `approval_stage_approver` per
  eligible contact; entering `customer_review` the same with `"Customer Review"`.
  Entry points (all call `provisionCustomerStage`): CAB / ECAB approval cascade,
  Request Approval on a Standard change, the `{state: "customer_review"}` PATCH,
  and any PATCH that carries `state` **or `projectId`** (after New an equal `projectId`
  is the accepted no-op write that still re-derives the contacts, see below). The stage kind rides on
  `checkpoint_label` (`stageKindCustomerApproval` / `stageKindCustomerReview` in
  `classifyApprovalStage`) — **no migration**. The approvals read response shows
  them under those labels, `approverName` = `"Customer Group"` (the fixed
  `customerGroupDisplayName`), `approverType` `STATIC_GROUP`. The Customer stages
  are excluded from the internal checkpoint ordinal count in
  `provisionApprovalStage`, so the Review stage of a Normal change is still
  created after a Customer Approval stage exists.
* **Decisions** go through `DecideChangeRequestApproval` (first responder wins,
  siblings cancelled, `canDecide` true only on a member's own `REQUESTED` row).
  Outcomes, applied in the same transaction:

  | Stage | Approved | Rejected |
  |---|---|---|
  | Customer Approval | `scheduled`, `is_customer_approval_required = true` | `canceled` |
  | Customer Review | `closed`, `is_customer_review_required = true` | `rollback` |

  Rejected review -> `rollback` (which also cancels the change's still-requested
  approver rows): the same state a human reaches with the manual Roll back
  action from `review` / `customer_review` (see "Roll back" above), and
  `canceled` for a declined approval matches the ServiceNow `isCustomerApproved:
  false` semantics. **`rollback` is terminal** (`legalNextStates` none). The decision comes from the approval, so the flag is stamped by the outcome itself
  (`applyCustomerStageOutcome`; the decider is a project contact).
* **A non-contact** (or anyone without a `REQUESTED` row) deciding on a change
  waiting on its live customer stage gets a **403** `only members of the
  customer group (the registered contacts of this change request's project) can
  approve or reject the customer's approval|review of this change request` (the
  creator keeps "the creator of a change request cannot approve it"); elsewhere
  the old 404 "no pending approval found" stays. **Customer A's contacts can
  neither be asked about, nor decide, customer B's change request.**
* **Who answers: the customer, in the customer portal — never in the CSM portal.**
  `DecideChangeRequestApproval` decides as the caller's own `"user"` (resolved from the
  `x-user-id-token` email) and only on their own `REQUESTED` row, so a registered
  contact's token is accepted by the entity service. Customers do **not** sign in to the
  CSM portal (`apps/csm-portal`; its BFF routes `POST /change-requests/{id}/approvals/decision`
  as `PermWrite`, the `cs_engineer` / `admin` roles, for internal approvers), so a CSM user
  is never an approver of a customer stage and the CSM Approvals tab only displays the
  stage's rows and the outcome. The customer answers from the customer portal
  (`apps/customer-portal`: its change request page's Approve / Reject, and its backend-v2,
  which routes the same `GET /change-requests/{id}/approvals` and `POST .../approvals/decision`
  and forwards the customer's `x-user-id-token`). Do not write a test that signs a customer
  in to the CSM portal. The CSM Playwright specs apply a customer's answer server-side
  instead (`apps/csm-portal/webapp/tests/e2e/utils/customerPortalDecision.ts`: a mock-oidc
  ID token for the contact, sent to this endpoint; or `customerDecides` on the fake API) and
  assert what the CSM page then shows — the deciding contact's row Approved / Rejected, the
  others' Cancelled, the change Scheduled / Closed / Canceled / Rollback. With a live stage
  `legalNextStates` offers `authorize` (Re-schedule, `customer_approval` only) and `canceled`
  (the manual `scheduled` / `closed` is refused, with or without a live stage).
  The ServiceNow workflow is the same shape (customer-side approvers answer in ServiceNow).
* **The customer's two ways to answer are one decision.** The customer portal's
  backend-v2 reaches entity-service with the customer's own `x-user-id-token` through
  either `POST /change-requests/{id}/approvals/decision` or `PATCH
  /change-requests/{id}` (`{isCustomerApproved}` / `{isCustomerReviewed}`: the portal's
  Approve / Reject buttons -- the contract it was built against ServiceNow with). Both
  end in the same code and leave the same state, flags and rows; see "Customer answers
  through PATCH" below. backend-v2 lets customer-side roles reach both routes through a
  narrow `decide` permission (`apps/customer-portal/backend-v2/CLAUDE.md`); who may
  answer *which* change is decided here, not there.
* **Nobody to ask: no stage, and no way for staff to answer for the customer.** A project with no
  eligible contact (a ticked box needs a project since the lock: Request Approval is refused without
  one, and **refused when the project has nobody who can be asked** since "Refuse Request Approval"
  -- rule 7 of the lock; a change with no project can still be met on rows that predate the lock; a
  project whose only contact is the creator; a legacy change with contacts but no stage yet; every
  contact deactivated after Request Approval, the residual edge): **no stage**.
  `customer_approval` is `[authorize, canceled]` and `customer_review`
  `[rollback, canceled]`, and a manual `{state: "scheduled"}` / `{state: "closed"}` is a **400**
  exactly as with a live stage (see "There is no "Schedule" action"): the customer's approval /
  review can only be given by the customer in the Customer Portal. Such a change waits: staff can
  Cancel it, Re-schedule it (`customer_approval`), Roll it back (`customer_review`, nobody is being
  asked), or have a contact registered and restate the stored `projectId` in a PATCH, which asks
  them (`TestChangeRequestLockIntegration_NoContactsReachedTheStage`,
  `TestChangeRequestNoBypassIntegration_ANobodyToAskChangeIsAskedOnceAContactRegisters`). A legacy
  change that already waits in the state with contacts but no stage gets its stage from the
  customer's own first act ("An in-flight legacy change request" below). **Decided: Request Approval
  refuses a change whose project has no eligible contact (and so does turning a box on after it), so
  a change created through the portal cannot reach this dead end except by the residual edge or as a
  legacy row.** With a live
  stage, `legalNextStates` is `[authorize, canceled]` at `customer_approval` and `[canceled]` at
  `customer_review` (the manual `rollback` is withdrawn: `state "rollback" cannot be set manually:
  the customer's review has been requested from the customer group (the registered contacts of the
  change request's project) and is given by one of them approving or rejecting it in the change
  request's approvals (POST /change-requests/{id}/approvals/decision)`). Cancel stays available and
  cancels the pending rows.
* **Contacts changed later** (`provisionCustomerStage`, idempotent, under the
  `change_request` row lock; the stage is compared with the project's *current*
  eligible contact set). The Customer Project itself can no longer change after New
  (the lock), so what follows is about the contacts of the frozen project: a PATCH
  that carries `state` or `projectId` -- after New an equal `projectId` is an
  accepted no-op write that still re-derives the group, the way to tell a change in
  Customer Approval / Customer Review about a contact who registered or left since
  -- re-reads them. A contact registered / deregistered while a stage is live ->
  the old stage's `REQUESTED` rows are `CANCELLED` and a new stage is provisioned for
  who is registered now (never two live stages; the old stage stays as a record);
  nobody eligible -> pending rows cancelled, nobody asked; resent / unrelated
  PATCH -> nothing. A stage already approved/rejected is never re-provisioned.
  (`TestChangeRequestFlowIntegration_CustomerGroupFollowsTheProject`; it used to move
  the project around while a stage was live, which the lock refuses.)
* **ServiceNow.** Not mirrored beyond the existing decision replay
  (`approval_decision` writeback); no ServiceNow field names for customer-group
  approvals are guessed. The pure ServiceNow data source's offered states are filtered
  like PostgreSQL's (`withoutCustomerOutcomeStates` offers no `scheduled`, and no `closed`
  from `customer_review`); the rest of that source is unchanged.
* **ServiceNow impact.** The dual-write mirror used to forward `customerGroupId`;
  it no longer does (nothing to send). Pure-ServiceNow reads no longer map
  `customerGroup`.
* Seed: `scripts/csm-compose/seed-entity-service.sql` seeds two customers — Example
  Corp (project 401, registered contacts dave.mendis and erin.jayawardena) and Other Corp
  (project 402, registered contact sam.other) with the `PORTAL_USER` role /
  "General Access" project group they hold — and CHG-FIXED-007
  (`customer_approval`) / CHG-FIXED-008 (`customer_review`) on project 401 with
  their stages (no assignment group). See "Local seed personas" below.
* Tests: `TestChangeRequestFlowIntegration_CustomerGroup*`,
  `_StoredCustomerGroupIsNoLongerUsedForApprovals`, `_SeedCustomerGroupFixtures`,
  `TestChangeRequestScopeIntegration_CustomerContactsAreDerivedFromTheProject`,
  `_CreateRefusesRemovedFields`, `_PatchRefusesRemovedFields`,
  `_LinkOptions` (real Postgres), `TestRejectRemovedChangeRequestFields`,
  `TestCustomerStageSpecs`, `TestWithoutStaffRollbackWhileCustomerReviewPending`,
  `TestCustomerStageManualRefusal`, `TestRefuseStaffCustomerOutcomeFlags`,
  `TestRefuseStaffExitFromCustomerState`, `TestClassifyApprovalStage`,
  `TestChangeRequestNoBypassIntegration_*` (every situation that used to allow the
  bypass, the exact `legalNextStates` table, the exits that stay).

### Customer visibility and the cutover

**A customer sees a change request only when it was designated to them.** The rule
(`change_request_visibility.go`, the one place it is spelled) for every caller that is
not `Unrestricted` -- customers, and staff who also hold a customer record
(`HasInternalAccess` without `Unrestricted`) -- is: they are a **registered contact of
the change request's CURRENT project** AND either

1. **it was designated to them**: they hold, or ever held, an `approval_stage_approver`
   row (`REQUESTED`, `APPROVED`, `REJECTED` or `CANCELLED`; the sync's `NOT_REQUESTED` /
   `NOT_REQUIRED` / `NOT_ENTITLED` mean "not asked") on a **"Customer Approval" /
   "Customer Review"** stage of it. Designation is per person and permanent: it is made
   when the stage is provisioned (`provisionCustomerStage`), nothing in this codebase
   deletes an approver row (`reconcileStaleApprovers`, `cancelLiveCustomerStages`,
   migration 0193 only move `REQUESTED` to `CANCELLED`), so the change request stays
   visible in **every** later state -- Authorize after a proposed new time, Scheduled,
   Implement, Review, Customer Review, Closed, Rollback, Canceled -- to the contact who
   answered *and* to the siblings whose row the answer Cancelled; a contact who
   registers **after** the stage was provisioned was never asked and does not see it;
   moving the change request to another project takes it from the old project's
   contacts (and gives it to nobody in the new one: nobody there was asked), moving it
   back gives it back; or
2. **it is legacy** (below): visible to the project's registered contacts in the states
   customers have always seen -- everything past Authorize (Scheduled, Customer
   Approval, Implement, Review, Customer Review, Rollback, Closed, Canceled), **never**
   New / Assess / Authorize or a NULL state.

Not visible means *absent* -- from the list, its total, the aggregate, the stat cards,
the calendar, the exports -- and **404 `change request not found`** on every by-id
read and write (detail, approvals, decision, PATCH of any field, comments, attachments),
the same answer for a change request that does not exist, so an id cannot be probed.
This supersedes the earlier interim state narrowing (backend-v2's
`restrictToCustomerVisibleStates`, the webapp's `EXCLUDED_ALLOWED`) **on the Postgres
data source**, where it must be removed with it -- left in, it would keep hiding a
designated change request in Authorize: with this rule a designated change request in
Authorize is shown and a non-designated one in Customer Approval is not. The ServiceNow
data source has no designation, so the state list stays there, applied by this service
(see "The ServiceNow data source" below), not by a BFF or a webapp.

**Where it is enforced: Go SQL, not a row-level-security policy.** The legacy test needs
the state, the creation time and a configured instant; a policy would need a new session
setting on every statement (`queueIdentity`, `setCallerIdentity`) and `CREATE POLICY`
takes `ACCESS EXCLUSIVE` on `work_item` (migration 0190's header records 0147 deadlocking
the running sync); and the integration suite connects as a superuser, so an RLS-only
guarantee could not be proven in the normal run. The fragment is **self-contained**: it
binds the viewer's email from the Go context identity (never the `app.viewer_email`
setting, which six mid-transaction `setCallerIdentity(Unrestricted)` calls blank) and
checks project membership itself, so it holds for a superuser connection and a restricted
role alike; RLS stays what it was, a second coarser line underneath. A restricted caller
with no email matches nothing (`FALSE`). `a, b` below are the `work_item` / `change_request`
aliases, `$e` the lower-cased email, `$c` the cutover instant (NULL when unset):

```sql
a.project_id IN (SELECT pc.project_id FROM project_contact pc
                  WHERE LOWER(pc.email) = LOWER($e) AND pc.state = 'REGISTERED')
AND (
  a.id IN (SELECT ast.work_item_id
             FROM approval_stage_approver asa JOIN approval_stage ast ON ast.id = asa.stage_id
            WHERE ast.checkpoint_label IN ('Customer Approval', 'Customer Review')
              AND asa.state IN ('REQUESTED', 'APPROVED', 'REJECTED', 'CANCELLED')
              AND asa.approver_user_id IN (SELECT u.id FROM "user" u WHERE LOWER(u.email) = LOWER($e)))
  OR (b.state::text IN ('SCHEDULED','CUSTOMER_APPROVAL','IMPLEMENT','REVIEW','CUSTOMER_REVIEW',
                        'ROLLBACK','CLOSED','CANCELED')
      AND ($c IS NULL OR a.created_on < $c))
)
```

Every subquery is uncorrelated, so Postgres evaluates each once (a hashed subplan);
`idx_user_email_lower` (migration 0192_user_email_lower_index) and
`idx_approval_stage_approver_approver_user_id` serve the designation lookup. The labels
and the state lists come from the Go constants and a test pins them
(`TestCRVisibility_*`).

**Per path, as implemented** (`TestChangeRequestVisibilityIntegration_*` asserts each, in
every state, for each persona):

| Path a customer can reach | Enforcement |
|---|---|
| `POST /change-requests/search` (list, total, calendar, CSV / PDF export, header search, stat-card filters) | the fragment in `SearchChangeRequests` (count and page) |
| `POST /change-requests/aggregate` (not routed by backend-v2) | the fragment in `AggregateChangeRequests` |
| `GET /change-requests/{id}`, and the PATCH receipt (the same read) | the fragment in `GetChangeRequestByID`'s single SELECT: 404 |
| `GET /change-requests/{id}/approvals` | `requireVisibleChangeRequest` first: 404 (it used to answer an empty 200, which confirmed the id existed) |
| `POST /change-requests/{id}/approvals/decision` | the guard is the first statement of `DecideChangeRequestApproval`'s transaction, before any lock |
| `PATCH /change-requests/{id}` (an answer, a proposed time, any other field) | the guard is the first statement of `PatchChangeRequest`'s transaction, **before** `classifyExternalPatch`: a hidden change request is a 404 even for a field no customer may set (it would otherwise be the 403 that confirms it exists) |
| `GET /projects/{id}/stats/change-requests`, `GET /projects/{id}/stats` (outstanding count) | the fragment in `ChangeRequestStateCounts`, `ChangeRequestResolvedBuckets` and the change request part of `OutstandingCounts`; the service counts Authorize as outstanding **for a customer only** (`crOutstandingStatesFor`), so the card matches their list |
| `/comments`, `/comments/search` with `referenceType: change_request`, and the by-comment-id operations (get, edit, delete, edit history) | `requireVisibleReference` (create, search); a `NOT EXISTS (hidden change request)` condition on the by-id statements |
| `POST /cases/{id}/comments` and the other case endpoints that act on **whatever work item an id names**: comments search, tags, watch list, `POST /attachments/search` with `referenceType: change_request`, `PATCH` parent | `rejectHiddenChangeRequest` / `requireVisibleChangeRequest` in `caseRepo` (customer portal's backend-v2 forwards `POST /cases/{id}/comments` for any id, so a change request's id could be passed); `UpdateCaseParent` is now restricted to case-like types |
| change request attachments | not reachable through the customer portal (`authorizeAttachmentAccess` forces `case` / `deployment`); the entity-level search is guarded as above |
| `POST /change-requests`, `/link-options` | `internalOnly` (route) |
| incidents, problems, SLAs | `internalOnly`; their joins only name a linked change request |
| global search, case activities | case-like types only |
| customer notice mail (approval requested, plan-date answer) | `CustomerNoticeEmails`: the **designated contacts** (registered, with a customer-stage approver row), not every contact of the project; a legacy change request nobody was asked about keeps the project audience |

`TestChangeRequestVisibilityLint_*` (an AST lint in the style of the row-level-security
bypass lint) fails the build when an exported repository function touching a change
request or its approvals neither applies the fragment / guard nor carries a
`// crvis:<reason>` comment, and when a method of the customer-facing interfaces is
unclassified.

**The ServiceNow data source (`DATA_SOURCE=servicenow`, the pure one).** Nothing is
designated to a customer there (ServiceNow runs the approvals; this service asks nobody),
so every change request is legacy by definition and the rule is only its state list:
everything past Authorize, **never New / Assess / Authorize**. ServiceNow's own search
does not apply that list: it returns every state a caller names and, for a caller that
names none, every state there is. The customer portals used to hide the three by never
asking for them (the webapp's allowed-state list), which no caller that asks differently
(the microapp's "all" tab sends no state at all, a direct API call) was held to, so
`sn_change_request_customer_view.go` applies the rule here, for every caller that is not
staff:

* a search or aggregate is sent only the visible states (`customerChangeRequestStates`):
  the requested ones that are visible, **every visible state when none is named** (never
  "no state filter"), and a request that names only hidden states is answered with an
  empty page without a request to ServiceNow; validation (a malformed state is still a
  400) runs first;
* what ServiceNow answers is checked again: a change request in a state that is not in the
  visible list (also one this service has no word for) is dropped and not counted, and the
  number dropped is logged (`dropHiddenChangeRequests`; nothing is expected to be dropped);
* `GET /projects/{id}/metadata` leaves New, Assess and Authorize out of
  `changeRequestStates` (by ServiceNow key or by label), so the state filter a BFF builds
  from it cannot name them, and `GET /projects/{id}/change-requests/stats` leaves their
  rows out of `stateCount`.

"Staff" is a request whose scope is `Unrestricted`: `callerIdentityMiddleware` already
resolves every request once (`AccessService.ResolveScope`: the CSM portal's backend for a
user of `CSM_PORTAL_USER_DOMAIN`, the `M2M_CLIENT_IDS` clients, an INTERNAL user where a
database is there to look them up) and attaches the scope to the context, and
`customerViewApplies` reads it. **Every other request, including one for which no scope
could be resolved (no identity, an unknown client, the customer portal's client with no
database to look the user up in), gets the customer view: fail closed**; the customer
portal's own client is never unrestricted, even if it is also listed as machine-to-machine
(`ResolveScope` checks it first). This is the line the Postgres rule draws (`crViewer`), so
a ServiceNow deployment shows its staff New / Assess / Authorize exactly when they resolve
as unrestricted for every other scoped endpoint; `TestServiceNowRouter_*` drive the real
router and fail if that stops being true.

Not changed on this data source, by design or for want of a source: the by-id reads and
writes (detail, approvals, decision, PATCH, comments) stay ServiceNow's own decision (a
customer needs the id, which only a list gives them; ids are not guessable); the counters
of the change request stats (`totalCount`, `outstandingCount`, `activeCount`,
`actionRequiredCount`) are computed by ServiceNow over every state and are passed through
(backend-v2 leaves the same three *rows* of `stateCount` out too, by ServiceNow key, as a
second line). Tests:
`sn_change_request_customer_view_test.go` and `server/sn_change_request_customer_view_route_test.go`
(a fake ServiceNow that answers every state whatever it was asked; the second drives the
real router), and `TestChangeRequestVisibilityIntegration_NamingAStateNeverWidens*` for the
Postgres data source (naming a state only narrows).

**Legacy and `CR_STRICT_VISIBILITY_FROM`.** Change requests migrated or synced from
ServiceNow, and the ones raised before this rule, were never asked through our flow:
they carry no customer-stage rows (and `customer_approval_required` /
`customer_review_required` are false on migrated rows), so "designated" alone would hide
every one of them from the customers who see them today. No existing column says where a
row came from (`created_by` is an email on both, `sn_sys_id` columns exist on other
tables only, number series and id shapes are unsafe) and no column may be added, so the
one deterministic marker is **time**: `work_item.created_on` against the instant in
**`CR_STRICT_VISIBILITY_FROM`** (RFC 3339 with a zone). Created **before** it = legacy;
at or after = strict. **Unset or empty = no cutover: every change request is legacy**
(today's visibility; the safe default and the rollback; a WARN is logged at startup,
because it is also how a deployment that means to be strict silently is not). The local
compose stack sets `2000-01-01T00:00:00Z`, so the seed and the specs are strict. An
unparsable value refuses to start (`Config.Validate`); `CRVisibilityFromConfig` fails
**closed** (strict from the epoch) should it ever be reached without `Validate`.

Release procedure: set it **once per environment** to that release's own instant, never
move it, and leave it unset to roll back. Known failure modes, deliberately accepted:

* A change request **raised directly in ServiceNow after the instant** counts as new but
  never passed our flow: it has no customer rows, so strict mode **hides it from
  customers** until it reaches a customer state *through us* (`provisionCustomerStage`
  designates it then). Decide per environment whether to leave the variable unset until
  ServiceNow raising stops.
* **Native change requests on dev older than the instant** count as legacy.
* **Each environment needs its own value**; **moving the instant flips existing rows
  retroactively** (earlier = more strict, later = more legacy).
* The sync rewrites `created_on` from ServiceNow: skew matters only for rows created
  within the sync's skew of the instant.
* A contact who is a **registered** contact of a legacy change request's project but was
  not asked sees it past Authorize (as before); in Authorize they do not, unless the
  customer-stage rows designate them.

**An in-flight legacy change request with no live customer stage** (the user's
`CS-PORTAL-000026 "Demo Test 1"` on Lumen Works Platform: in Customer Approval since an
older build) used to answer **409 "nobody asked"** to the customer's Approve. Now the
customer's first act -- an answer (`answerCustomerStageViaPatch`), a proposed time
(`prepareCustomerProposal`) or a decision (`DecideChangeRequestApproval`) -- calls
`ensureCustomerStageForLegacy` in **its own transaction**, which provisions the stage
through the same `provisionCustomerStage` the normal path uses (every registered
`PORTAL_USER` contact of the project, one `REQUESTED` row each, the creator listed
`CANCELLED`), which also **designates** them: the change request stays visible after the
act (the answer's `SCHEDULED`; the proposal's `AUTHORIZE`, a state a legacy change
request is otherwise hidden in). It acts only when the caller is a customer (never
staff), the change request is legacy, it is in Customer Approval / Customer Review right
now, **no live stage** exists for that state (an existing live stage is never touched),
and the caller is a registered `PORTAL_USER` contact of its own project;
`provisionCustomerStage` re-checks under `FOR UPDATE OF cr`, so concurrent customers
provision exactly once (`..._LegacyInFlightGetsItsStage/two_customers_answering_at_once`),
never reopens a stage that was decided, and a refused act rolls its provisioning back with
it. `customerCanAnswer` is **true** for the contacts such a stage would ask, computed
read-only (`legacyStageWouldBeProvisioned`; `GET` never writes). Nothing is backfilled and
the pure ServiceNow data source never reaches this code. **Unverified:** whether
csm-sync-service's `delete_sync` covers `approval_stage_approver` (its repository is not
in this checkout): if it removes our rows, the change request reverts to legacy / hidden
(fail closed) and the next customer act provisions the stage again. All registered
contacts of the project are designated at the first touch (accepted).

**ServiceNow dual-write and the sync.** The visibility rule is a **read** filter: it adds
no write refusals, so the PostgreSQL-first PATCH and its asynchronous ServiceNow mirror
are unaffected, a 404 returns before the mirror is dispatched, and csm-sync-service
(which writes SQL as `app.is_internal = true`, never through these validators) is not
touched. The fragment reads the change request's *current* `project_id`, so a project the
sync moves a change request to strands stale designations (fail closed).

**Cost** (`EXPLAIN (ANALYZE, BUFFERS)` as the non-superuser role on a synthetic copy of
the schema: 400,000 work items, 200,000 change requests, 3,000 projects, 60,000 users,
97,000 approver rows; the viewer a registered contact of 5 projects, project 0 holding
20,086 change requests of which 516 are visible to them). The dominant cost is the
row-level security that already ran per row; the fragment adds almost nothing and, being
selective, often cuts the work the policy does:

| Count query for project 0 | Execution | Buffers |
|---|---|---|
| before this change (RLS + the project hint) | 213 ms | 141,110 |
| strict (cutover long ago: 516 visible) | 90 ms | 16,564 |
| unset (every change request legacy: 14,530 visible) | 221 ms | 167,397 |
| strict, the page query (`ORDER BY created_on DESC LIMIT 25`) | 66 ms | 16,570 |
| the by-id guard | < 1 ms | 55 |

The designation lookup is one hashed subplan over the viewer's own approver rows
(`idx_approval_stage_approver_approver_user_id`, `idx_user_email_lower`). The membership
subquery scans `project_contact` once more per statement (`setViewerProjectIDsSQL` already
scans it once per request; 432 buffers at 25,000 contacts): there is no index on
`lower(project_contact.email)`, and none is added here (no migration).

**Deliberately not done / follow-ups.** No column, table, type or policy was added; if
database-level enforcement is wanted later it needs a policy-only migration numbered after
`git pull` (0206 or later; upstream and this branch use up to 0205), a new session setting
`app.cr_strict_from` on every statement, and the same predicate as a function. The detail
still returns `createdBy` (a staff email) to a customer. Mixed-identity staff on the
customer portal see designated change requests only. The unlabeled-stage classification
hardening for migrated approval stages and the diagnosable "nobody eligible" counts are
the change request lock / dates work's (`change_request_approval_flow.go`), not this
section's.

Tests: `TestChangeRequestVisibilityIntegration_*` (real Postgres,
`CHANGE_REQUEST_TEST_DSN`, run twice -- as a superuser and as the non-superuser
`csm_app`: designated stays visible through the whole lifecycle and in Rollback /
Canceled, per person and permanent, never required the customer, the legacy rule state by
state and instant by instant incl. the cutover boundary, a legacy change request a
customer acted on, an in-flight legacy change request gets its stage (once, concurrently),
internal / BFF / mixed / no-email callers, comments by id, the notice audience, the stats,
the case endpoints), `TestChangeRequestVisibilityLint_*`, `TestCRVisibility*`,
`TestConfig_CRStrictVisibilityFrom`, `TestCRVisibilityFromConfig`,
`TestProjectChangeRequestStats_AuthorizeIsOutstandingForCustomersOnly`.

### Customer answers through PATCH (customer portal)

The customer portal was built against ServiceNow, where the customer's answer is
`PATCH /change-requests/{id}` with `{isCustomerApproved: true|false}` in Customer
Approval, `{isCustomerReviewed: true|false}` in Customer Review, or
`{plannedStartOn}` to propose a new implementation time. On this data source those
requests and the Approvals-tab decisions are **one mechanism with two doors**.
Code: `change_request_customer_outcome.go` (`classifyExternalPatch`,
`answerCustomerStageViaPatch`, `prepareCustomerProposal`), hooked at the top of
`patchChangeRequestTx`; the shared implementation is
`decideChangeRequestApprovalTx` (what `DecideChangeRequestApproval` runs).

**Who is "a customer" here: an external caller** -- a resolved identity with
`SearchScope.Unrestricted == false` and `HasInternalAccess == false`
(`isExternalCaller`). Internal staff (and the system identity, and staff who also
hold an external record) are NOT the customer: their PATCH keeps the whole contract
except the customer's answer -- `isCustomerApproved` / `isCustomerReviewed` from them
is a 400 (it used to be a bookkeeping stamp of the flag that moved nothing), and so is
the manual `{state: scheduled|closed}` out of the customer states (see "There is no
"Schedule" action"). A staff user who is also a registered contact answers through the
decision route on their own `REQUESTED` row, like anyone.

* **Whitelist.** An external caller's PATCH may carry exactly one of the customer's
  answer (`isCustomerApproved` **or** `isCustomerReviewed`, optionally with
  `expectedPlannedStartOn` / `expectedPlannedEndOn`, see "The answer is bound to the
  window the customer saw") or a proposed window
  (`plannedStartOn` and/or `plannedEndOn`), and **nothing else**: any other field
  -- title, state, project, assignee, `requestApproval`, `onHold`, comment,
  `customerApprovalRequired`, a field added later -- is a **403** `customers can
  only record the customer's approval or review ... or propose a new
  implementation time ...`, checked by clearing those fields and requiring the
  rest of the request to be empty. Row-level security lets *any* member of the
  project update the row and has no notion of fields, so this is the layer that says
  what a customer may change; the portal in front of it is not relied on. Both
  outcomes in one request, or an answer together with a window, is a 400.
* **The answer is a decision.** `{isCustomerApproved}` is the caller deciding their
  own pending approval on the Customer Approval stage, `{isCustomerReviewed}` on the
  Customer Review stage -- `answerCustomerStageViaPatch` ends in
  `decideChangeRequestApprovalTx`, so the calling contact's row becomes
  `APPROVED` / `REJECTED`, the siblings `CANCELLED`, the state moves and the flag is
  stamped by the approving outcome, once:

  | PATCH from a registered contact | State | Flag | Rows |
  |---|---|---|---|
  | `{isCustomerApproved: true}` in `customer_approval` | `scheduled` | `is_customer_approval_required = true` | caller `APPROVED`, siblings `CANCELLED` |
  | `{isCustomerApproved: false}` | `canceled` | not stamped | caller `REJECTED`, siblings `CANCELLED` |
  | `{isCustomerReviewed: true}` in `customer_review` | `closed` | `is_customer_review_required = true` | caller `APPROVED`, siblings `CANCELLED` |
  | `{isCustomerReviewed: false}` | `rollback` | not stamped | caller `REJECTED`, siblings `CANCELLED` |

  (`TestChangeRequestCustomerOutcomeIntegration_PatchEqualsDecisionRoute` drives
  each row through both doors and requires identical state, flags and rows.) What
  the old code did instead -- measured, not assumed: with a live stage a contact's
  `{isCustomerApproved: true}` stamped the flag and left the change in Customer
  Approval with every approver row still `REQUESTED`; a second contact's approve
  was a silent no-op and their reject a 400 "locked"; without a live stage it was
  the same stamp-and-stay.
* **Refusals, in order** (the first to fail wins; nothing is written until all
  pass, the whole PATCH is one transaction): change request not visible to the caller
  (not designated to them and not legacy, see above) -> 404, before the request is
  even classified; caller not a **registered `PORTAL_USER` contact of the change
  request's own project** (another project's contact, a contact with no
  `PORTAL_USER` role, an invited one, a non-contact, an unknown user) -> **403**;
  change request not in the answer's state (Customer Approval for
  `isCustomerApproved`, Customer Review for `isCustomerReviewed`) -> **409**, the
  stale-approval message (`this approval is no longer pending: the change request
  is in Scheduled, but the Customer Approval stage can only be decided while it is in
  Customer Approval`) -- which is also what the second contact gets after the first
  answered; a rejection of a flag already `true` -> 400 `locked once set to true`;
  an answer that names the window it was given for (`expectedPlannedStartOn` /
  `expectedPlannedEndOn`) while the stored window is another -> **409** `the planned
  implementation time of this change request changed after you opened it (it is now
  ... to ...)`;
  no live customer request and none could be created -> **409** `no customer approval
  is pending on this change request: it has not been requested from the project's
  registered contacts, so there is nothing to answer here` (WSO2 does not answer in the
  customer's place: the manual `{state: scheduled|closed}` is a 400 too): for a **legacy** change request waiting in
  the state the first customer act *creates* the stage instead (see "An in-flight legacy
  change request"), so the 409 is left for a project with nobody to ask but the creator,
  or a stage already decided; a strict change request nobody was asked about is not
  visible at all (404); then the decision's own rules: the
  creator -> 403 `the creator of a change request cannot approve it`, a contact who
  was not asked (registered after the request went out, inactive user) -> 403 `only
  members of the customer group ...`.
* **`customerCanAnswer` -- what the portal offers.** The change request detail
  (`GET /change-requests/{id}`, and the PATCH receipt, which is the same read:
  `GetChangeRequestByID`) carries `customerCanAnswer` for an external caller: the
  per-viewer "may I answer this now", so the customer portal shows Approve / Reject /
  Propose from the server's answer instead of guessing. `hasCustomerApproved` /
  `hasCustomerReviewed` cannot be that guess: they are the recorded OUTCOME
  (`is_customer_approval_required` / `is_customer_review_required`, NULL until the
  customer has approved), false for exactly as long as the change waits for the
  customer. `customerCanAnswer` is computed by `customerCanAnswer` (read-only, no
  locks) and is **true exactly when the answer would be accepted**: the change is in
  Customer Approval / Customer Review; the viewer is a registered `PORTAL_USER` contact
  of its project; the customer's request is live (a stage of that state with a
  `REQUESTED` row) -- or, for a **legacy** change request waiting in that state with
  no stage at all, it would be created by the viewer's own act and they would be one
  of the people asked (`legacyStageWouldBeProvisioned`, read-only); the viewer holds a `REQUESTED` row on that stage (so not a contact
  registered afterwards, not one a sibling's answer or a Re-schedule cancelled); and
  `approverDecisionBlock` lets them (the creator never). It reuses the answer path's
  helpers (`liveCustomerStageForState`, `customerApproverUserID`,
  `changeRequestCreatorsForApprover`, `callerIsRegisteredPortalContact`), the
  same ones `markCanDecide` is built on for the Approvals tab, so the three cannot
  drift. A `*bool` with `omitempty`: **present (true/false) only for an external
  caller on the PostgreSQL data source**; **absent** for staff / internal callers /
  the system identity, for the ServiceNow data source and when the check itself
  failed (logged) -- absent means unknown, which is not false, and a client then
  keeps what it did before the field existed. Never on search rows, and nothing else
  about the approvals (approver identities, stages) is added to the customer's
  detail. It flips to false in the very PATCH receipt of the caller's answer or
  proposal, and back to true for the contacts once a Re-schedule has asked them
  again (a Standard change at once, Normal / Emergency after CAB / ECAB approves).
  It does not look at `onHold`: a held change refuses a *proposal* (409), not an
  answer, so a client offers Propose New Time when `customerCanAnswer && state ==
  customer_approval && !onHold`; entity-service's detail carries `onHold`, and the
  customer portal's backend-v2 passes it on as the boolean `isOnHold` (never the
  reason). `customerCanAnswer` is also the *propose* signal: the
  proposal is gated on the same pending-row test (`customerHasRequestedRow`), so a
  contact told false can neither answer nor propose. Tests:
  `TestChangeRequestCustomerCanAnswerIntegration_*` (lifecycle, after any answer
  through either door, who may answer with the PATCH as the oracle, nobody asked,
  Re-schedule flips for Normal / Standard, not on search rows),
  `TestCustomerCanAnswer_NeedsNoQueryOutsideTheCustomerStates`,
  `TestMarkCustomerCanAnswer_WhoIsToldWhat`,
  `TestChangeRequest_CustomerCanAnswerJSONContract`.
* **The answer is bound to the window the customer saw.** `{isCustomerApproved |
  isCustomerReviewed, expectedPlannedStartOn?, expectedPlannedEndOn?}`: each bound
  named must still equal the stored one when the answer is recorded
  (`checkExpectedSchedule`, under the change request's row lock, right after the
  stale-state check), else 409 and nothing changes. After a Re-schedule loop the
  change returns to Customer Approval and the contacts are asked again with fresh
  rows, so the stale-state check alone passes for a page opened before it -- without
  this a stale tab approved a window its reader never saw. Omitted: no check (an answer
  sent as before is recorded as before, and so is every integrator's). The fields
  belong to a customer's answer only: alone or beside a proposal is a 400, and from a
  WSO2 user's PATCH too. They are compared as instants (any of the accepted layouts;
  not held to the year range, since they are never written). The customer portal sends
  the `startDate` / `endDate` it read.
* **Propose new implementation time = Re-schedule.** `{plannedStartOn,
  plannedEndOn?}` from a registered contact in `customer_approval` is the process
  diagram's "Time Change" loop started by the customer: `prepareCustomerProposal`
  checks it (registered contact, not the creator, state `customer_approval`, **asked**
  -- a live customer stage on which the caller holds a `REQUESTED` row, else 409 when
  nobody was asked and 403 `only members of the customer group ... who have been asked`
  when the caller was not, as proposing cancels the asked contacts' pending approvals --
  the window still to come, not on hold) and turns the request into the very `{state: authorize, plannedStartOn?,
  plannedEndOn?}` a WSO2 user sends, so the existing Re-schedule applies unchanged:
  "Time Change = Yes" enforced (`re-scheduling requires a changed planned start or
  end`), the new window applied, the customer's pending request cancelled (kept as a
  record), a **fresh CAB / ECAB approval** (Normal: CAB, the peer approval stands;
  Emergency: ECAB), and when it is approved the cascade returns the change to
  Customer Approval with a fresh customer stage. A Standard change has no internal
  approval to repeat: dates applied, stays in Customer Approval, the customer asked
  again. Anywhere but Customer Approval -> 409, on hold -> 409, creator /
  non-contact -> 403. A start alone keeps the STORED end (it is not shifted for the
  customer), so a proposed start after the stored end is a 400 `the planned start
  must not be after the planned end`: a customer moving a change to a later date
  proposes the whole window -- the new start and the new end, the same length -- as
  the CSM portal's Re-schedule dialog does. A window with no length (end == start) is a
  400 for every Re-schedule, and a customer's proposal must be still to come (400 `... is
  in the past`; a WSO2 user's Re-schedule may re-plan a window that has gone by).
* **The planned window is parsed here, not by Postgres** (`change_request_window.go`,
  applied at the top of `patchChangeRequestTx` for every caller, and on create). It used
  to be bound as `$n::text::timestamptz`, which accepts `infinity`, `-infinity`, `now`,
  `tomorrow`, `epoch`, a bare date, a zone name, years up to 294276, and reads a value
  with no zone in the database *session's* `TimeZone`. Measured on the local stack, a
  customer's `{plannedEndOn: "infinity"}` was stored and the PATCH answered 500 (the
  read-back failed after the commit), after which `GET /change-requests/{id}` was a 500
  for everybody and, once the change was in Customer Approval, so was the project's
  whole change request search. Now exactly two layouts are accepted: RFC 3339 with a
  zone designator, and `YYYY-MM-DD HH:MM:SS` as **UTC**, in the years 2000 to 2100 --
  anything else is a 400 `plannedStartOn must be a valid date-time, either RFC 3339
  (2030-03-01T09:00:00Z) or YYYY-MM-DD HH:MM:SS in UTC, in the years 2000 to 2100` --
  and what reaches SQL is the parsed instant re-written as RFC 3339 UTC, so the stored
  value no longer depends on the session. `changeRequestSelectColumns` also reads
  `start_on` / `end_on` through `isfinite(...)`, so a row that holds an infinity by some
  other door reads as having no planned time instead of failing the scan (and with it
  the detail or the whole list). `TestChangeRequestCustomerProposalIntegration_*` pins the
  whole-window proposal for Normal / Emergency / Standard, the messages for a bad window
  (unchanged, ends before it starts, empty, in the past, not a date), the hostile values,
  UTC under a Colombo / Los Angeles session and a non-finite row that still reads.
* **Lock order.** The answer and the proposal take the `work_item` row first (a
  `PATCH`'s own `updated_on` / `updated_by` bump, `lockCustomerAnswerRow`), then
  `change_request` -- the order every other PATCH takes -- so a customer's answer and
  a concurrent edit cannot deadlock. Two contacts answering at once serialise on the
  `change_request` lock: one answers, the other gets the 409
  (`TestChangeRequestCustomerOutcomeIntegration_ConcurrentAnswers`).
* **Exposure of internal states: closed.** A customer's reads and writes of a change
  request are decided by the visibility rule ("Customer visibility and the cutover"
  above): New / Assess / Authorize change requests that were never designated to them are
  absent and 404 by id, here and through backend-v2 (whose interim state narrowing is
  superseded by it and must go). `GET /change-requests/{id}/approvals` does not name WSO2's internal
  approvers to a customer -- the Customer Approval / Customer Review stages are given
  whole, every other stage (Peer, CAB, ECAB, Review) as its label and status only, no
  approver names, ids or group (`redactInternalApprovalStages`,
  `TestChangeRequestCustomerPrivacyIntegration_ApprovalsHideWhoApprovesInternally`).
* Tests: `TestChangeRequestCustomerOutcomeIntegration_*` (real Postgres,
  `CHANGE_REQUEST_TEST_DSN`: lifecycle, rejections, PATCH == decision route, out of
  state, who may answer, nobody asked, the flag lock, the whitelist, concurrency,
  Re-schedule from a proposal), `TestChangeRequestCustomerPrivacyIntegration_*` (the
  answer bound to the window seen, the approvals a customer reads),
  `TestChangeRequestIntegration_PatchCustomerFlag*`
  (the original flag authorisation, rewritten: staff can no longer send the flags),
  `TestClassifyExternalPatch`,
  `TestIsExternalCaller`, `TestStateForMessage`, `TestNormalizePlannedTimestamp_*`,
  `TestRequireFutureWindow`, `TestRedactInternalApprovalStages`. The integration DSN connects as a
  Postgres superuser, which bypasses row-level security: what they assert is this
  repository's own checks, never RLS.

**`scanChangeRequestView`/`scanChangeRequestViewAndDetail` had a
scan-destination bug** found in production logs: `wi.created_on`/
`wi.updated_on` (`TIMESTAMPTZ`) were scanned directly into
`&v.CreatedOn`/`&v.UpdatedOn`, both `string` fields on
`SearchChangeRequestView` (RFC3339-formatted, like `PlannedStartOn`) — pgx
v5 can't scan a binary-format timestamptz into a `*string`
(`SearchChangeRequests` failed on every call with "can't scan into dest\[20\]
... cannot scan timestamptz ... in binary format"). Fixed the same way
`PlannedStartOn`/`PlannedEndOn` already were: scan into an intermediate
`time.Time`, then `.UTC().Format(time.RFC3339)` into the string field.
Both approval methods (`GetChangeRequestApprovals`, `DecideChangeRequestApproval`)
are not -- see below for why. (`CreateChangeRequest` used to be on this list
too, blocked on the same `work_item.number` gap `CaseRepository.CreateCase`
had; both are now implemented, via `next_portal_work_item_number()` -- see
"CreateCase and case numbers" above and `ChangeRequestRepository.CreateChangeRequest`'s
own doc comment. Unlike case, `change_request` is excluded from
`work_item_wso2_id_required_by_type`, so no `wso2_id` generation is needed
here at all.)

- **`GetChangeRequestApprovals`/`DecideChangeRequestApproval`**: these
  model multiple approval *stages*, each with multiple *approvers* and
  per-approver status (`domain.ChangeRequestApproval`/`ChangeRequestApprover`).
  This schema has only one summary `change_request.approval` column
  (`REQUESTED`/`APPROVED`/`REJECTED`/`NOT_REQUESTED`) — no approval-stage or
  approver table at all. There's nothing to serve either method from
  without a schema change, so both always return a `ServiceUnavailableError`
  on Postgres.

**`ServiceID`/`ServiceOfferingID` are now wired up** (migration 0046 added
`change_request.service_id`/`service_offering_id`, FKs into `service`/
`service_offering`, migrations 0044/0045): readable via
`SearchChangeRequestView.Service`/`ServiceOffering` and writable via
`PatchChangeRequestRequest.ServiceID`/`ServiceOfferingID`. `service`/
`service_offering` also got their own Postgres implementations
(`it_service_repo.go`/`service_offering_repo.go`) backing `POST /services/
search` and `POST /service-offerings/search`, previously ServiceNow-only.

**`AssignedTeamID` is now read** — `work_item.assignment_group_id` (migration
0075, a FK into `group`), via `changeRequestFromJoins`' own `"group" ag`
join, back as `domain.ChangeRequest.AssignedTeam`. A real, reported bug: the
CSM Portal's own action bar at the time required `assignedTeam` to be set
before it would let a change request advance to Assess at all, and since
this was never read, *no* change request could ever be promoted past New
through the portal on this data source — confirmed live against a real
change request with a genuine ServiceNow Assignment group ("Devops"), whose
`AssignedEngineer` synced and displayed correctly while `AssignedTeam`
always showed empty. This proved `csm-sync-service` already populates
`work_item.assignment_group_id` for change requests the same way it does
for every other `work_item` type, so the fix is read-only — no create/patch
write-path changes were needed alongside it.

**That frontend gate was briefly believed stale, removed, then confirmed
real and reinstated — with real teeth this time.** It was carried over
unchanged from when New→Assess sent a ServiceNow "Request Approval" action
(which genuinely needed a team), and since that transition became a plain,
ungated `{state: "assess"}` PATCH (see "New→Assess is a plain, ungated state
change" below) it looked like a stale leftover with nothing left to gate —
removed for exactly that reason. **This was then confirmed wrong by explicit
product decision**: an assigned team is still compulsory before Assess, for
a different and still-current reason — see the approver auto-provisioning
paragraph below, which needs a team to have anyone to provision at all.
`ChangeRequestActionBar.tsx`'s `TARGET_BLOCKED_REASON` has an `assess` entry
again. This time the requirement is **also enforced server-side**, in
`PatchChangeRequest` itself — not just the frontend courtesy check — so no
direct API caller can bypass it: a `{state: "assess"}` PATCH with no
`assignedTeamId` in the same request, and none already on the record, is
rejected with a `ValidationError` before anything is written.

**Writing `AssignedTeamID` is now wired too.** `PatchChangeRequestRequest.
AssignedTeamID` sets `work_item.assignment_group_id` the same way
`AssignedEngineerID` sets `assigned_to_id` immediately above it in
`PatchChangeRequest`; a `23503` FK violation maps to the friendly field name
`assignedTeamId` via `changeRequestPatchFKField`, same convention as every
other FK column on this PATCH. Verified live against the local compose
stack: setting it to a real seeded `"group"` row round-trips correctly and
survives a reload; setting it to a well-formed but unknown id produces a
clean `assignedTeamId does not refer to an existing record` validation error
instead of a raw Postgres error. Writing it at **create** time
(`CreateChangeRequestRequest.GroupID`) remains unwired — a separate,
different field with no confirmed equivalence to this one (see this file's
own comment on `CreateChangeRequestFromServiceNow`). Filtering search
results by it (the parsed filter array's `assignmentGroupId`) is also still
unwired — see `changeRequestWhereClause`'s own comment.

**The moment a change request actually enters Assess, the assigned team's
own members are auto-provisioned as that stage's approvers.** By explicit
product decision: a real change request was found live sitting in Assess
with its own Approvals tab completely empty — "no approval stages recorded
for this change request" — with no way for anyone to ever approve it into
Authorize, because nothing anywhere writes `approval_stage`/
`approval_stage_approver` rows on this data source; those are normally only
ever populated by `csm-sync-service` mirroring ServiceNow's own
`sysapproval_group`/`sysapproval_approver` tables, and this particular
record evidently had none synced (or none in ServiceNow at all — the two
cases can't be told apart from here). `PatchChangeRequest` now closes that
gap itself: whenever a `{state: "assess"}` patch succeeds and the work item
has no `approval_stage` row yet, it creates one (`assignment_group_id` = the
effective assigned team from this same request or already on the record),
then inserts one `REQUESTED` `approval_stage_approver` row for every
`team_member` whose **`group_id`** (not `team_id` — see below) matches that
team, all inside the same transaction as the state write. `GetChangeRequestApprovals`
needed no changes at all — it already renders whatever `approval_stage`/
`approval_stage_approver` rows exist, regardless of who wrote them.

**The change request's own requester is provisioned `CANCELLED`, not
`REQUESTED`, when they are also a member of the assigned team** — mirroring
ServiceNow's own real self-approval-prevention behavior, confirmed live
against a real ServiceNow record (wso2sndev.service-now.com, CHG0039122,
inspected directly) rather than guessed: that record's own requester was
also a member of the group an approval stage was generated for, and their
`sysapproval_approver` row came back already in `Cancelled` state — the
record's own activity log shows this as the very first "Field changes"
entry at creation, not a later transition away from `Requested`. Every
other team member's row is `Requested` exactly as before. The row still
gets created, same as real ServiceNow — it's just born cancelled rather
than omitted. `requested_by_user_id` is read fresh from `change_request`
inside the same transaction (not from the request's own
`RequestedByID`, which is only set when this particular PATCH is the one
changing it) so this reflects the change request's post-PATCH value, since
the `crSets` UPDATE earlier in this same transaction may have just set it.
**Reintroduces the identical dead-end risk the empty-group check above
already guards against, in a new shape**: if excluding the requester would
leave zero `REQUESTED` approvers — the requester is the assigned team's
only member, or every member happens to be the requester via some data
anomaly — the whole `{state: "assess"}` PATCH is rejected with a
`ValidationError` ("the assigned team has no members other than the
requester to provision as Assess approvers"), checked before the
`approval_stage` row is created, same ordering discipline as the
empty-group check.

Scoped to "no `approval_stage` exists yet" so a resent `{state: "assess"}`
(a retry, or an unrelated field edit while already in Assess) can never
duplicate the stage or re-seed approvers over whatever
`DecideChangeRequestApproval` has since done to it — that method owns
everything about an existing stage from the moment this provisioning step
creates it.

**Two correctness issues caught on CodeRabbit review of this same addition,
both fixed here:**

1. **An assigned team with no members used to still create an empty stage.**
   The original ordering created `approval_stage` first, then queried
   `team_member` — if that query came back empty, the transaction still
   committed a stage with zero approvers, and since "no `approval_stage`
   exists yet" is exactly the condition this whole block gates on, a later
   `{state: "assess"}` PATCH would never retry provisioning either: the
   change request was left stuck in Assess with an approval nobody could
   ever decide. Fixed by querying and validating `team_member` **before**
   creating the stage: an empty result now rejects the whole PATCH with a
   `ValidationError` ("the assigned team has no members to provision as
   Assess approvers") and leaves no `approval_stage` row behind at all,
   rather than committing a dead-end one.
2. **`team_member` has no unique constraint on `(user_id, group_id)`.** A
   duplicated membership row would have queued one `approval_stage_approver`
   INSERT per duplicate, seeding two `REQUESTED` rows for the same person.
   The query is now `SELECT DISTINCT user_id`, not `SELECT user_id`.

**`team_member.group_id` is the real column for this, and it is distinct
from `team_member.team_id`.** `team_member` carries both: `team_id`
(`NOT NULL`) is the hand-curated internal team registry's own FK (`team`,
migration 0033 — what `POST /groups/search`/`GetUserGroups` read), while
`group_id` (nullable) is a separate FK into the same `"group"` table
`work_item.assignment_group_id`/`approval_stage.assignment_group_id`
reference. These are two distinct tables with two distinct id spaces in
general — checked directly: `team_member.group_id` was unpopulated (0 of
160 rows) in the local compose stack's own seed data, and only one `team`
row happens to share an id with a `"group"` row at all (the local seed
script's own "Example Corp ABT" fixture, deliberately given matching ids
purely for that one fixture's convenience, not a general guarantee). Do not
substitute `team_id` for this lookup even though the one local test fixture
would appear to work either way — `group_id` is the column whose FK
actually points at the same `"group"` row the rest of this feature uses
everywhere else, and is presumably populated in real synced environments by
`csm-sync-service` mirroring ServiceNow's own `sys_user_grmember`, the same
way `assignment_group_id` itself is populated from `sys_user_group`.

**(Superseded — see "Approval flow by change type" above: the second stage is now the separate `CAB Approval` group, not the assigned team, and is only provisioned by the peer-approval cascade, never by a direct `{state: "authorize"}` PATCH.) The second approval checkpoint, Authorize ("Risk approvals" in real
ServiceNow), got the identical auto-provisioning treatment as Assess**
— the same gap, one lifecycle step later: a change request that reaches
Authorize with nothing in `approval_stage`/`approval_stage_approver` for it
is just as stuck as the original Assess-empty-Approvals-tab bug this whole
feature exists to fix. By explicit product decision this reuses the SAME
assigned team (`work_item.assignment_group_id`) Assess already uses — there
is no confirmed evidence ServiceNow uses a separate CAB-specific group for
this gate, so none is invented here — with the identical self-approval-
exclusion and dead-end-guard rules, just parameterized by which checkpoint
is being gated ("Assess"/"Authorize" in the `ValidationError` messages).

**Refactored first, rather than duplicating the ~140-line Assess block.**
`provisionApprovalStage(ctx, tx, workItemID, assignedTeamID, actorEmail,
checkpoint)` (`change_request_repo.go`) is the single implementation both
checkpoints now call: query-and-validate `team_member` before creating
anything (the CodeRabbit-fixed empty-group ordering), the self-approval
exclusion and its own dead-end guard, `DISTINCT` deduplication, one
`approval_stage` row plus one `approval_stage_approver` row per member. The
two Assess-only `ValidationError` messages ("the assigned team has no
members ... as Assess approvers", "... no members other than the requester
... as Assess approvers") are unchanged in wording for Assess itself —
`checkpoint.Label` is simply `"Assess"` there, `"Authorize"` for the new
caller.

**The design question this needed answering: how does a second checkpoint's
provisioning tell "no stage exists yet for ME" apart from "a stage already
exists for an EARLIER checkpoint"?** Migration 0089's `approval_stage` has
no column recording which lifecycle transition a given row belongs to, and
adding one was considered and rejected: every stage this repository has
ever created (Assess's, going back to the original addition) and every
stage `csm-sync-service` has ever mirrored in from ServiceNow's own
`sysapproval_group` would need a backfill to populate it, and this schema
already has an established, working answer to "which checkpoint is this"
that needs no new column and no backfill at all — a stage's zero-based
**ordinal position** among `approval_stage` rows for the same
`work_item_id`, ordered by `created_on`. `changeRequestApprovalStagePosition`
(the read path, `GetChangeRequestApprovals`) already derives a stage's
label — "Assess"/"Authorize"/"Customer Approval" — purely from this
ordinal, and `DecideChangeRequestApproval`'s own `isAssessStage` check (see
below) already gates its state cascade on the identical ordinal rather than
on `change_request.state` alone. Reusing that same convention for
provisioning, instead of inventing a second, parallel way to answer the
same question, keeps every consumer of "which checkpoint is this stage" —
old and new — in agreement with no migration required.

Concretely: `changeRequestApprovalCheckpoint{Position, Label}` carries the
expected ordinal (`changeRequestAssessCheckpoint` = 0,
`changeRequestAuthorizeCheckpoint` = 1), and `provisionApprovalStage` only
creates a stage when `COUNT(*) FROM approval_stage WHERE work_item_id = $1`
**exactly equals** `checkpoint.Position` — not merely "less than or equal",
and not "not yet exists at this checkpoint's label". Fewer existing stages
than `Position` means an earlier checkpoint's own stage hasn't been created
yet, and provisioning this one anyway would land it at the wrong ordinal
and be silently mislabeled the next time `GetChangeRequestApprovals` reads
it back (e.g. an Authorize stage created with zero prior stages would read
back as "Assess"); more existing stages means this checkpoint (or a later
one) already has its stage — the exact generalization of the original
Assess-only "no `approval_stage` exists yet" guard
(`TestChangeRequestIntegration_PatchAssessDoesNotReprovisionWhenStageExists`'s
own invariant), which this change keeps enforcing unchanged for Assess
(`Position: 0`, i.e. still exactly "no stage exists at all yet") while
extending the identical mechanism to Authorize. An Assess stage already
existing does not block Authorize's own provisioning (it's precisely the
precondition Authorize's `Position: 1` expects), and vice versa — confirmed
by `TestChangeRequestIntegration_AssessAndAuthorizeStagesCoexist`, which
runs the real Assess→decide→Authorize flow end to end and checks both
stages' approvers are independently correct and neither clobbers the other.

**Wired into both of Authorize's real entry points, with deliberately
different failure handling at each.** A change request reaches Authorize
two ways on this data source, mirroring the two ways Assess provisioning
already triggers (a direct `{state: "assess"}` PATCH, generically, and
nothing else — Assess has no cascade of its own):

1. **A direct `{state: "authorize"}` PATCH** (`patchChangeRequestTx`) — the
   generic `if req.State != nil { ... }` trigger, exactly mirroring Assess's
   own. Resolves the effective assigned team the identical way Assess does
   (`req.AssignedTeamID` from this same request, else whatever is already on
   `work_item.assignment_group_id`), then calls `provisionApprovalStage`.
   Unlike Assess, there is **no** new compulsory "assignedTeamId is
   required" gate added for this transition — by the time a change request
   reaches Authorize through its one real, normal path (Assess's own
   compulsory gate, then approval), a team is already guaranteed to be on
   the record, so inventing a second hard gate here would be redundant
   product surface for a case that shouldn't occur. A direct PATCH that
   skips Assess entirely (this data source enforces no legal-transition
   order — see this file's own "LegalNextStates" history above) with no team
   ever assigned is still caught: `provisionApprovalStage`'s own "no
   members" check treats a nil/empty team the same as an assigned-but-empty
   group, so the whole PATCH is rejected with the same `ValidationError`
   shape rather than silently leaving Authorize stageless. **This path fails
   loudly** — a `ValidationError` from `provisionApprovalStage` rolls back
   the whole PATCH, same as Assess, since a direct PATCH caller can see and
   immediately correct it.
2. **`DecideChangeRequestApproval`'s own Assess→Authorize cascade** — the
   `isAssessStage`-gated branch that already flips `change_request.state` to
   `AUTHORIZE` on a resolving approval (see that method's own doc comment
   above). This is the path a change request actually reaches Authorize
   through in production, not the direct PATCH above (there is no "Change
   state → Authorize" button; the Approvers section is the only route).
   Immediately after the state write, this reads the assigned team fresh off
   `work_item` (this method carries no `PatchChangeRequestRequest` of its
   own) and calls the identical `provisionApprovalStage`. **This path is
   deliberately best-effort** — a provisioning failure (no team, an empty
   group, a requester-only group) is logged (`slog.WarnContext`) and
   swallowed, never returned from `DecideChangeRequestApproval` itself. This
   is a considered asymmetry, not an oversight: by the time provisioning
   runs here, the approver's own decision has already been recorded and
   `change_request.state` has already genuinely advanced — rolling that
   whole transaction back because some OTHER, future checkpoint's team
   configuration has a problem would turn a real, valid approval into a
   confusing failure for the person who just approved it, over something
   entirely outside their action. This matches the same "a downstream side
   effect must never fail the primary mutation" convention this file's own
   `publishXxx` helpers already follow elsewhere in this service. The
   practical effect of this is the known, accepted gap it creates: a change
   request whose assigned team has no members (or only the requester) by
   the time the Assess→Authorize cascade fires lands in Authorize with no
   approval stage of its own — exactly the dead-end this whole feature
   exists to prevent, just for this one specific, narrow precondition
   failure on this one specific entry point, logged rather than silent.

   **Caught on review, fixed before merge**: "logged and swallowed" only
   actually held for `provisionApprovalStage`'s own `ValidationError`
   returns (empty group, requester-only group), which happen before any SQL
   write runs. A failure at the *database* level inside it instead — a
   constraint violation, a bad cast — poisons the whole surrounding
   Postgres transaction: every later statement, including this method's own
   eventual `COMMIT`, would then fail with "current transaction is
   aborted," silently rolling back the very approval decision this
   best-effort block exists to protect — defeating its entire stated
   purpose for exactly the class of failure it was least prepared for. The
   call is now wrapped in its own `SAVEPOINT` (`tx.Begin(ctx)` on an
   already-open pgx `Tx` issues one): a failure rolls back only that
   savepoint — undoing just provisioning's own half-written statements —
   and the outer transaction, decision and all, commits normally; only a
   genuine failure to open or release the savepoint itself (vanishingly
   rare — e.g. the connection dying) propagates as a real error.

**The third approval checkpoint, Review ("Internal Review" in real
ServiceNow's own workflow), gets the identical auto-provisioning treatment as
Assess and Authorize** — the same gap, two lifecycle steps later, at the
Review state of `changeRequestForwardNextStates`
(...→Implement→Review→{Closed, CustomerReview}→Closed). Same product decision
as Authorize: this reuses the SAME assigned team
(`work_item.assignment_group_id`), the identical self-approval-exclusion and
dead-end-guard rules, via `changeRequestReviewCheckpoint =
changeRequestApprovalCheckpoint{Position: 2, Label: "Review"}` — the next
ordinal after Authorize, created via the exact same `provisionApprovalStage`
every earlier checkpoint already calls; nothing about those rules is
checkpoint-specific, only the ordinal/label differs.

**Review has only ONE real entry point, not two — confirmed before writing
any code, not assumed.** Authorize's own write-up above documents two real
entry points precisely because `DecideChangeRequestApproval`'s state cascade
exists at all for Assess→Authorize. Reading that method in full (including
its own doc comment's explicit scoping) confirms it stops there: "The state
cascade is deliberately scoped to Assess→Authorize only — Authorize's own
outgoing approval gate (into Scheduled or Customer Approval) is a separate,
deferred piece of work." No cascading approval-decision mechanism exists
anywhere in this repository past Authorize, so a change request reaches
Review exclusively through a direct `{state: "review"}` PATCH, via
Authorize→Scheduled→Implement→Review (or the Customer Approval detour) —
the same generic `if req.State != nil { ... }` trigger in
`patchChangeRequestTx` every earlier checkpoint already uses, with no second
call site to wire into `DecideChangeRequestApproval` the way Authorize's own
cascade path required. Unlike Authorize, there is therefore no best-effort/
fails-loudly asymmetry to document for Review: its one entry point fails
loudly, exactly like Authorize's own direct-PATCH entry point does, for the
same reason — a direct PATCH caller can see and immediately correct a
`ValidationError`. Same "no compulsory assignedTeamId gate" reasoning as
Authorize applies too: by the time a change request reaches Review through
its only normal path, a team is already guaranteed to be on the record, so a
third hard gate would be redundant product surface for a case that shouldn't
occur; a direct PATCH that skips straight to Review with no team ever
assigned is still caught by `provisionApprovalStage`'s own "no members"
check.

**The labeling collision above is now fixed, via migration `0179`.** Adding
the Review checkpoint at position 2 collided with `changeRequestApprovalStagePosition`'s
pre-existing hardcoded "position ≥ 2 → Customer Approval" default (written
when position 2 was still purely theoretical) — a real change request that
actually took the Authorize→Customer Approval branch would land ITS stage at
the same ordinal, and `approval_stage` had no column recording which
lifecycle transition a given row was actually for, so position alone could
never tell the two apart once both were possible at the same ordinal.

Resolved with `approval_stage.checkpoint_label` (migration `0179`, nullable
`VARCHAR(50)`): `provisionApprovalStage` now writes its own `checkpoint.Label`
("Assess"/"Authorize"/"Review") directly onto every stage it creates, rather
than leaving it to be inferred later from sibling count.
`changeRequestApprovalStageLabel` (the new wrapper `buildChangeRequestApprovals`
actually calls) prefers this explicit label when present, and only falls
back to the original `changeRequestApprovalStagePosition` ordinal heuristic
when it's NULL — true for every ServiceNow-synced stage (that system has no
equivalent concept to sync) and any stage provisioned before this column
existed, so neither needs a backfill and nothing synced is reinterpreted.
Every checkpoint this codebase provisions going forward (Customer Approval,
Review again, Customer Review) must keep writing its own explicit label the
same way — the ordinal heuristic is now purely legacy-fallback plumbing, not
something new checkpoints should ever rely on again.

**Fields still with no real column anywhere, left unset rather than
guessed at** (see `ChangeRequestRepository`'s own doc comment for the full
list): `ConfigurationItemID` (no CMDB table exists in this schema at all);
`Type` (`domain.ChangeRequestType` — standard/normal/emergency/... — has
**no** relationship to `change_request.change_request_type`, whose real enum
values are `INFRA`/`GENERAL`, a completely different classification, not a
subset of the domain enum); `ApprovedBy`/`ApprovedOn` on
`domain.ChangeRequest` (no approver/date columns exist). `Duration`
(`cr.calendar_duration`, an `INTERVAL`) is also left unset — no confirmed
display format to render it in.

**`LegalNextStates` used to be on the list above too ("a ServiceNow
workflow-engine computation with nothing to derive it from here") — it no
longer is.** Its absence on this data source was reported live: a change
request could be created (`DATA_SOURCE=postgres-servicenow-dual-write` is
ServiceNow-first on create), but the CSM Portal's own lifecycle action bar
(`ChangeRequestActionBar.tsx`) renders nothing at all when
`legalNextStates` is empty — the reported symptom was "create works, but no
way to promote it," for every change request on this data source, not just
one. `changeRequestForwardNextStates`/`legalChangeRequestNextStates`
(`change_request_repo.go`) now compute it: a forward-only graph
(New→Assess→Authorize→{Scheduled, Customer Approval}→Implement→
Review→{Closed, Customer Review}→Closed). Every edge except
`CustomerApproval`'s and `CustomerReview`'s own outgoing move (see below)
was read directly off a real change request sitting in that exact state
on the live ServiceNow instance (its own `state` field's dropdown, which
ServiceNow itself only ever populates with the choices it currently
considers legal) — confirmed, not guessed. `"canceled"` is additionally
offered alongside the forward move(s) from every non-terminal state, since
the Cancel Change action was observed available on every reachable state.
`Rollback`/`Closed`/`Canceled`
return `nil` (terminal, no legal forward move), matching ServiceNow's own
answer for a record with none.

> **Historical — superseded by "Approval flow by change type" and "Customer
> Approval / Customer Review checkboxes" above.** Authorize no longer offers
> Scheduled / Customer Approval to a human, and Review's branch is now decided by
> `customerReviewRequired` instead of offering both. Kept for the reasoning.

**Authorize and Review each have two confirmed forward moves, not one —
found the hard way.** A first revision of this map picked a single "common
case" edge for each (Authorize→Scheduled, Review→Closed), reasoning that
`domain.ChangeRequest.HasCustomerApproved`/`HasCustomerReviewed`
(`change_request.customer_approval`/`customer_review`) record whether the
customer **has already** signed off, not whether a given change request
**requires** that gate, so they can't be used to decide the branch — true,
but it was resting on an unverified assumption that one branch was simply
the common case. Checking several more real records directly disproved
that: two Authorize-state records with no other visible difference in the
fields this schema exposes (same type, both approval/review booleans
false) had dropdowns offering `Scheduled` on one and `Customer Approval` on
the other — and the identical split was found for Review (`Closed` on one
record, `Customer Review` on another, again with no discriminating field
found). Whatever ServiceNow actually keys this decision on is not visible
anywhere in this schema, so both confirmed branches are now offered for
each of these two states rather than guessing which one applies to a given
record. This means an engineer can be offered an action ServiceNow's own
workflow would consider illegal for that specific record — an accepted
risk here, matching `PatchChangeRequest`'s own pre-existing lack of a
legal-transition check on this data source (any enum value is accepted and
written directly; this map doesn't change that) — the offered action still
gets ServiceNow's own real rejection reason back on the attempt
(`mapUpstreamError` surfaces it) rather than silently succeeding wrong.
Revisit if the real gating field is ever identified.

**This risk only actually reaches an engineer for the Review branch.** The
webapp's own `ChangeRequestActionBar.tsx` hardcodes `"customer_approval"`
into its `NEVER_OFFERED_TARGETS` list — reached only by ServiceNow's own
approval process, never human-enterable there, per that list's own doc
comment — and filters it out unconditionally regardless of what
`legalNextStates` returns, so Authorize's `Customer Approval` entry is
accurate data that never becomes a clickable button. `"customer_review"`
carries no such exclusion, so Review's `Customer Review` entry does render
as a real, selectable action.

`CustomerApproval`/`CustomerReview`'s own **outgoing** edges (what a change
request already sitting in one of those two states advances to) are a
separate, smaller gap: no real change request was found sitting in either
state despite specifically checking, so both are inferred by sequence
position (`CustomerApproval` precedes `Scheduled`; `CustomerReview`
precedes `Closed`) rather than confirmed live.

**Change request creation now always sets `state = 'NEW'` explicitly** —
`createChangeRequestFromServiceNowQuery` previously left `change_request.state`
unset entirely (the column has no `NOT NULL`/`DEFAULT`), reasoned at the
time as: ServiceNow's own create response carries no state field to
confirm what its workflow engine actually assigned, so writing
`req.State` straight through risked recording a value ServiceNow silently
overrode. That reasoning was sound but produced a worse bug, reported
live: a freshly created change request had `state = NULL`, and
`legalChangeRequestNextStates(nil)` returns `nil` — so a brand new change
request offered no promote action whatsoever, not even the one every
change request always starts with. The org's own Change Management
process flow resolves the original uncertainty directly: every change
request begins at New unconditionally, with no branch or caller input that
changes that — so `'NEW'` is not a guess at what ServiceNow decided, it is
the one value ServiceNow's real workflow always assigns on create.
`CreateChangeRequestRequest.State` is still accepted on the wire (it's
shared with `PatchChangeRequestRequest`) but has no effect at creation and
is intentionally ignored by this insert.

**New→Assess is a plain, ungated state change — `RequestApproval` has
nothing to do with it.** An earlier revision of this section documented the
New→Assess promote action as sending `{requestApproval: true}` (see
`ChangeRequestActionBar.tsx`/`buildTransitionPatch` on the frontend) rather
than `{state: "assess"}`, and had `PatchChangeRequest` locally force
`state = 'ASSESS'` whenever `RequestApproval` was true and `req.State`
wasn't itself separately provided — modeling New→Assess as a special
"request approval" ceremony, gated on the record actually being in New
(rejecting with a `ConflictError` otherwise). **Checked against the real
ServiceNow instance and confirmed wrong**: New→Assess is a plain, direct,
ungated state change — like picking a new value from a dropdown — with no
relationship to approval at all. The frontend now sends a plain
`{state: "assess"}` for this transition, exactly like every other one, and
`PatchChangeRequest` handles it generically via its existing
`if req.State != nil { ... }` branch, with no special casing for New→Assess.

`RequestApproval` is now a pure bookkeeping flag: `{requestApproval: true}`
still sets `change_request.approval = 'REQUESTED'` (other code/displays may
still care about that field), but has **no state-transition side effect at
all**, and is no longer gated on the caller's current state — it can be sent
against a change request in any state and only ever touches `approval`.

The one real approval-gated transition remains **Assess→Authorize**, handled
entirely by the separate `POST /change-requests/{id}/approvals/decision`
endpoint (`DecideChangeRequestApproval`) — unrelated to `RequestApproval` and
unchanged by any of this.

**That cascade did not actually exist when this section was first written.**
An earlier revision of this same fix claimed `DecideChangeRequestApproval`
"already cascades `change_request.state` forward on approval" — that claim
was false, based on a misread of an unrelated earlier test, and was never
actually verified. Live testing (after the direct "Change state -> Authorize"
button was removed, leaving the Approvers section the only path to Authorize)
showed approving did nothing at all to `change_request.state`. Fixed
properly: `DecideChangeRequestApproval` now applies the same
first-responder-wins quorum rule `buildChangeRequestApprovals` uses at read
time — a single approval, provided nobody on the same stage has rejected,
both (1) advances `change_request.state` from Assess to Authorize and (2)
cancels every other still-`REQUESTED` approver on that same stage, matching
real ServiceNow's own observed behavior on a genuine multi-approver group
(confirmed live: only the 1-2 who actually responded were left
Approved/Rejected, every other pending approver on the same group was moved
to Cancelled, not left sitting at Requested indefinitely). **A rejection
used to do neither** — see the dedicated writeup just below, which closes
that gap. The state cascade itself is still deliberately scoped to
Assess→Authorize only — a decision on an Authorize-stage approver still
cancels its own siblings, but has no state-cascade effect yet.

**A rejection now cancels its stage's other pending siblings too — exactly
like an approval does, at every checkpoint — but still never touches
`change_request.state`, in either direction.** This was a real, confirmed
gap, not a deliberate asymmetry: `decideChangeRequestApprovalQuery` only
ever flipped the acting approver's own row and returned, so a rejected
stage's other `REQUESTED` approvers were left sitting there forever, with
no way to tell "this stage was rejected" apart from "nobody has looked at
it yet" short of reading every row — exactly the same dead-end the original
cancellation fix (above) already closed for approvals, just left open on
the rejection side. `DecideChangeRequestApproval` now runs the identical
`UPDATE approval_stage_approver SET state = 'CANCELLED' ... WHERE stage_id
= $1 AND state = 'REQUESTED'` on a rejection too (factored into a shared
`cancelSiblingApprovalStageApprovers` helper both branches now call), at
Assess, Authorize, and Review alike — sibling-cancellation was never
Assess-specific to begin with, only the state cascade is.

`change_request.state` is deliberately left completely untouched by a
rejection, both before and after this fix — no forward advance (obviously:
nothing was approved) and, just as deliberately, **no backward rollback
either**. A live investigation of the real ServiceNow "Change Request -
Normal" workflow (wso2sndev.service-now.com) found a genuinely complex
reject/rollback pattern threaded through it — "Set Values — `cancelled when
reject`", "Set Values — `Rollback when reviews rejected`", and a dedicated
"Rollback To — `Rollback to Customer Approval Process`" activity that moves
`change_request.state` BACKWARD to an earlier stage — but that investigation
was ACL-blocked on the actual condition scripts before it could confirm
either which earlier state a given rejection rolls back to, or under what
precise conditions it does so. Implementing a guess at that targeted
rollback would be inventing product semantics with no confirmed basis
(the same discipline this file's own "On hold" section above and its
`CustomerApproval`/`CustomerReview` outgoing-edges writeup already apply to
similarly unconfirmed SN behavior), so this is left as a known, explicitly
flagged, accepted gap rather than a guess: a rejected change request simply
stays exactly where it already was.

**The "already-resolved stage" edge case this needed a decision on**: can a
rejection ever land on a stage some OTHER decision already resolved, and if
so, what should happen? Two sub-cases, handled differently and on purpose:

- A sibling of the SAME stage-resolving decision (the normal case this fix
  itself creates) can never reach this branch at all — every decision in
  this method runs inside a transaction that first locks the owning
  `change_request` row (`SELECT ... FOR UPDATE`), serializing every
  decision against every other one for the same change request, so the
  moment any decision (approval or rejection) cancels a stage's other
  `REQUESTED` siblings, a later decision attempt on one of those siblings
  fails at `decideChangeRequestApprovalQuery`'s own
  `state = 'REQUESTED'` WHERE clause first (`NotFoundError`, "no pending
  approval found") — it never even gets a `stageID` to act on, let alone
  reaches the cancellation code.
- Data this method did NOT itself create or resolve — a ServiceNow-synced
  stage, or one seeded before this fix shipped — has no such guarantee: an
  `APPROVED` row can legitimately coexist with other still-`REQUESTED` rows
  that were never cancelled, because whatever created them predates (or is
  outside) this method's own cancellation discipline. For exactly this
  case, the rejection branch checks `hasApproval` (mirroring the existing
  approval branch's own `hasRejection` check) before cancelling anything,
  and skips cancellation entirely when the stage already has an `APPROVED`
  row — a late/duplicate rejection on an already-resolved stage is a no-op
  on its siblings, not a destructive retroactive cancellation of approvers
  an earlier approval had every right to leave alone. Covered by
  `TestChangeRequestIntegration_DecideRejectionDoesNotDisturbAlreadyApprovedStage`,
  which seeds exactly this shape directly (bypassing
  `DecideChangeRequestApproval` for the approval, the way a sync would) and
  confirms the untouched sibling survives.

`TestChangeRequestIntegration_DecideRejectionCancelsSiblingApprovers` and
`...CancelsSiblingApproversAtEveryCheckpoint` (table-driven over Authorize
and Review, using a synthetic earlier-`approval_stage` history to push the
real stage to each ordinal — the same mechanism
`changeRequestApprovalStagePosition`/`isAssessStage` already read) are the
regression guards for the cancellation itself; the existing
`TestChangeRequestIntegration_DecideApprovalRejectionDoesNotCascade` already
covered (and still covers) the single-approver no-state-change case this
fix does not alter.

`domain.ChangeRequestApprover` also gained `CreatedOn`/`Comments` (both
`*string`, both read from `approval_stage_approver.created_on`/`.comments`
via `changeRequestApprovalApproversQuery`) to support a full UI redesign:
the CSM Portal's own Approvers list used to nest approvers under a
collapsible per-stage accordion card — reported live as confusing (an
approver looking for their own pending decision gained nothing from first
finding "their" stage card and expanding it) — and now renders as one flat
table (State/Approver/Assignment group/Comments/Created/Approved on),
matching real ServiceNow's own Approvers list layout exactly, with every
approver from every stage shown together rather than grouped. Both new
fields are always null on the ServiceNow-backed data source: the Choreo
`GET /change-requests/{id}/approvals` response has no equivalent fields to
populate them from.

**Linking** — `CaseID` (`work_item.parent_id`, the generic self-reference, migration
0039, not case-specific) and `AssignedEngineerID` are still PATCH-only. The
Customer Project / Deployments / Deployment products fields can now be set at
creation too (migration 0191, see "Customer project, deployments and deployment
products" below); `SearchChangeRequestView.Project`/`Case`
can still be empty (`EntityRef{}`)/`nil` for a change request that was created
without them and never linked — a real, valid state for this schema, not a bug.

**"On hold" is now a real, enforced concept — it had no representation
anywhere in this schema at all before.** A live investigation of the real
ServiceNow "Change Request - Normal" workflow (wso2sndev.service-now.com, all
56 activities mapped end to end) found "on hold" threaded through nearly
every stage transition: activities named "Assess and On hold" / "Authorize
and On hold" / "Internal Review and On hold", each immediately followed by
an `If — "Check if Change is \"On hold\""` branch that, when true, runs a
`Wait for condition — "Wait for On hold to be false"` before that stage's
own approval/transition logic is allowed to proceed at all. Nothing in
entity-service modeled any of this — no column, no gate, nothing — so a
change request here never stopped advancing through its lifecycle
regardless of any ServiceNow-side hold, a confirmed, real gap between this
mirror and the system it models.

- **Schema** (migration 0178): `change_request.is_on_hold BOOLEAN`,
  `on_hold_reason TEXT`. (`on_hold_started_on` was added here originally and dropped
  again in migration 0190 — ServiceNow has no equivalent field and nothing consumed it.) Shape follows two
  existing precedents in this same table rather than inventing a third: the
  boolean naming matches `is_customer_approval_required`/`is_customer_review_required`/
  `is_planning_visible_to_customers` (migration 0043), and the
  flag-plus-"since" pairing mirrors `work_item.workaround_provided_on`/
  `workaround_provided_by_user_id` (migration 0021) — a nullable TIMESTAMPTZ
  recording *when* a state began, not a second boolean. `on_hold_reason` is
  free TEXT, matching every other free-text change_request column here
  (`justification`/`impact_description`/...): there is no fixed, closed
  vocabulary of hold reasons anywhere to draw an ENUM from. All three are
  nullable with no DEFAULT, so a record that predates this migration simply
  reads as "never on hold" (`OnHold` nil on the wire) rather than `false` —
  the same "a record predating a column has no opinion on it" posture this
  file already documents for e.g. `account.deleted_on`.
- **Domain** (`internal/domain/entity.go`): `SearchChangeRequestView` (and
  therefore `ChangeRequest`, which embeds it) gained `OnHold *bool`/
  `OnHoldReason *string` on the read side.
  `PatchChangeRequestRequest` gained `OnHold *bool`/`OnHoldReason *string` on
  the write side — plain optional pointers, not the tri-state
  pointer-to-pointer convention the Group C1/C2 "field-parity additions"
  use: `OnHoldReason` has no standalone "explicit clear" wire shape of its
  own (clearing it always goes through `OnHold: false` instead — see below),
  so a second level of nil-ness would have nothing to express.
- **Combinable, not exclusive — a deliberate, documented choice.** The task
  that added this asked for a reasoned choice between the two, matching
  whichever existing pattern this endpoint already follows most
  consistently. `UpdateCaseRequest` (case's own PATCH) has a real exclusive/
  combinable split (`state`/`watchList`/`assigneeEmail`/`parentId`/
  `acknowledge` mutually exclusive; `subject`/`description`/... freely
  combinable — see "PATCH /cases/{id}" above) — but `PatchChangeRequestRequest`
  has **no such grouping at all**: every existing field on this PATCH (state,
  impact, assignedTeamId, justification, requestApproval, ...) is already
  independently settable and freely combinable with every other field, with
  only one blanket "at least one field must be provided" check. Introducing
  a new exclusive group just for `onHold` would be inventing a new pattern
  for this one endpoint rather than following its own established one, so
  `OnHold`/`OnHoldReason` are fully combinable — including with `State`
  itself, which is exactly what the simultaneous-clear-and-advance behavior
  below depends on.
- **The gate** (`patchChangeRequestTx`, immediately after `work_item`'s own
  `UPDATE ... RETURNING id` succeeds — **not** before any write runs, see
  below for why): a PATCH that sets `state` is rejected with a
  `ValidationError` when `change_request.is_on_hold` is **currently**
  `true` — read fresh inside the same transaction, locked `FOR UPDATE`,
  never from whatever this same PATCH's own `crSets` might also be setting.

  **Caught on review, fixed before merge**: an earlier revision ran this
  check first, as a plain unlocked `SELECT`, before `work_item` was ever
  touched. That left a real race — a concurrent `{onHold: true}`-only PATCH
  could commit in the window between this read and this transaction's own
  later writes, letting a state-changing PATCH land against a record that
  was actually on hold by the time it committed. The fix isn't simply
  adding `FOR UPDATE` at that same early spot, though: every PATCH,
  state-changing or not, always writes `work_item` first (`wiSets` above
  always includes at least `updated_on`/`updated_by`) and `change_request`
  second (`crSets`, whenever it's non-empty) — so locking `change_request`
  at the old, earlier position would make this one code path take the
  *opposite* lock order from every other PATCH, and two transactions taking
  the same pair of locks in opposite orders is exactly how Postgres
  deadlocks. Moving the gate to run after `work_item` is already locked
  keeps the order consistently `work_item` → `change_request` everywhere.
  **The one deliberate
  exception**: `{state: X, onHold: false}` in the same request is allowed
  straight through — "take it off hold and advance in one call" (an
  approver clearing a hold and immediately promoting the record) is a
  legitimate, common single action, not two separate PATCHes, so a request
  that is *also* turning `OnHold` off is excluded from the gate rather than
  rejected by it. Taking a record off hold with no state change at all
  (`{onHold: false}` alone) is **never** blocked by anything, regardless of
  the record's current lifecycle state, terminal states included. A PATCH
  that never touches `state` at all (editing `description`, say) is
  completely unaffected by this gate either way, on-hold or not —
  deliberately: being on hold only ever blocks *advancing the lifecycle*,
  never any other field.
- **The write semantics** (same function, in the `change_request` `UPDATE`'s
  own field-by-field block): `OnHold: true` sets `is_on_hold = true`
  and sets `on_hold_reason` to `OnHoldReason` if provided in the same
  request, else clears it to `NULL` (a fresh hold event does not inherit a
  stale reason text from whatever hold period preceded it). `OnHold: false`
  always clears `on_hold_reason` to `NULL` regardless of whether `OnHoldReason` also accompanies the same request —
  taking a record off hold wins over setting a reason in the same call.
  `OnHoldReason` sent alone (`OnHold` omitted) only updates the reason text,
  letting a caller correct or add a reason on an existing hold without
  resending `OnHold` itself; it has no effect on `is_on_hold`
  and is not validated against the record's current
  on-hold status (a reason sent while not on hold is written but harmless —
  not cross-validated, matching this PATCH's existing "don't over-engineer a
  rarely-meaningful combination" posture elsewhere in this same field set).
- **Not done here, deliberately**: no search filter on `isOnHold` and no
  `AggregateChangeRequests` grouping by it — out of scope for this pass,
  following the same "accepted, not wired" posture `changeRequestWhereClause`
  already documents for `assignmentGroupId`. The webapp's own on-hold toggle
  and a blocked-reason display on the action bar are a deliberate follow-up
  cycle once this API contract exists, not part of this change.

**The customer's outcome flags are the customer's alone.**
`PatchChangeRequestRequest.IsCustomerApproved` / `IsCustomerReviewed`
(`domain.ChangeRequest.HasCustomerApproved` / `HasCustomerReviewed` on the read side;
columns `is_customer_approval_required` / `is_customer_review_required`, migration 0043,
plain booleans, no `approval_stage` involvement of their own) used to be written
straight through in `patchChangeRequestTx` from any caller, then (a first fix) from an
internal caller or a registered `PORTAL_USER` contact, one-way-locked
(`authorizeChangeRequestCustomerFlagWrite`). **Compliance rule (the user's decision):
no staff action records the customer's approval or review on the customer's behalf**
-- the answer is the customer's decision and ServiceNow's record of it is audited --
so that function, and the internal caller's right to stamp the flags, are gone:

- **A request that carries either flag from anyone but the customer is a 400** -- the
  caller is not an external customer answering (`isExternalCaller`, which already
  returned through `answerCustomerStageViaPatch`): internal staff, staff who also hold
  an external record, an internal client credential, a context with no identity. True
  or false, alone or with a state or other fields, whatever is stored (a no-op
  `true` -> `true` is refused too): `isCustomerApproved cannot be set on the customer's
  behalf: the customer's approval can only be given by the customer in the Customer
  Portal` (likewise `isCustomerReviewed` / "review"; `refuseStaffCustomerOutcomeFlags`,
  decided before anything is looked at or written). Refused, never silently ignored,
  so a client that still sends them (the CSM microapp's edit dialog did) learns it.
- **The flags are stamped by exactly one thing**: the customer's own answer
  (`applyCustomerStageOutcome`, true only), through `PATCH {isCustomerApproved|
  isCustomerReviewed}` from a registered contact or the decision route -- see "Customer
  answers through PATCH". The one-way lock stays there: a rejection of a flag already
  `true` is a 400 `locked once set to true`. There is no override path for anyone.
- **Who is "a registered contact"**: `callerIsRegisteredPortalContact` -- a
  `project_contact` of the change request's OWN project (`work_item.project_id`), in
  state `REGISTERED`, holding the `PORTAL_USER` project role via `project_contact_group`
  -> `project_group_role` -> `project_role` (the join chain
  `CaseRepository.ProjectContactEmailsByRole` / `ProjectContactRepository` use). It has
  no shortcut for an internal caller any more, so nothing built on it can let staff answer
  for the customer.
- **A project with no qualifying contact** simply has nobody who can answer: the change
  request waits in the customer state (cancel, re-schedule or, for a review, roll back; a
  contact registered later is asked when the stored `projectId` is restated). Request Approval
  refuses to send a change there (rule 7 of the customer requirements lock, same test as the
  provisioning), so this is a legacy row or the residual edge (every contact deactivated after
  Request Approval).
- **The ServiceNow-backed data source** is a different mechanism (its scripted API's
  dedicated `patchCustomerApproved` / `patchCustomerReviewed` handlers gate flipping either
  field on the change sitting in the matching state, and the "off" direction there drives a
  real state transition: `isCustomerApproved: false` -> Cancelled, `isCustomerReviewed: false`
  -> Rollback; see `EditChangeRequestDialog.tsx`'s doc comment). `sn_change_request_service.go`
  forwards the PATCH to ServiceNow, which is the authority there (this service cannot tell
  staff from the customer on that source without an identity lookup); the CSM BFF refuses
  the two flags from its staff callers for every data source. Only the offered states are
  filtered on that source (`withoutCustomerOutcomeStates`).
- **Tests**: `change_request_no_bypass_integration_test.go` (`TestChangeRequestNoBypassIntegration_*`),
  `change_request_repo_integration_test.go` (`TestChangeRequestIntegration_PatchCustomerFlag*`:
  staff refused whatever is stored; the registered contact, a contact of another project, a
  contact with no `PORTAL_USER` role and an invited one against the customer's own answer; the
  lock on a flag already true), `change_request_repo_test.go` (`TestRefuseStaffCustomerOutcomeFlags`,
  `TestRefuseStaffExitFromCustomerState`). **Environment quirk**: the local docker-compose
  stack's `CHANGE_REQUEST_TEST_DSN` connects as the `postgres` role, a Postgres superuser, which
  bypasses every RLS policy, so a "different project" / "invited contact" PATCH is refused by this
  code's own `ForbiddenError` instead of `work_item`'s RLS `NotFoundError`; the tests' doc comments
  say so, and the suite is also run as `csm_app` (RLS enforced).

## Fixing case enum-casing/mapping bugs and GetCaseByID's false 404s

Found in production logs after the plural/singular fix shipped: every
`case_state_enum`/`case_issue_type_enum`/`case_work_state_enum`/
`engagement_type_enum` filter and write in `case_repo.go` cast a
`domain.CaseState`/`CaseIssueType`/`CaseWorkState`/`EngagementType` value
(all lowercase, e.g. `"work_in_progress"`) straight into its Postgres enum
column (all `UPPER_SNAKE_CASE`, e.g. `'WORK_IN_PROGRESS'`), so every
`SearchCases` state/severity/issueType/workState/engagementType filter and
every `UpdateCase` state/severity/workState write failed with `invalid input
value for enum ... (SQLSTATE 22P02)`. Fixed with `strings.ToUpper(...)` at
every write/filter site and `strings.ToLower(...)` at every read site
(`GetCaseByID`, `SearchCases`, `scanUpdatedCase`) — for state, issue type,
work state, and engagement type, whose domain and real-column values match
1:1 once case-folded.

**Severity is the one exception**: `case_severity_enum`'s real labels are
`'S0'`..`'S4'`, completely unrelated to `domain.CaseSeverity`'s
catastrophic/critical/high/medium/low — case-folding alone can't bridge
that. `caseSeverityToEnum`/`caseSeverityFromEnum` (`case_repo.go`) map
between them using the standard S0=most-severe/S4=least-severe ITSM
convention, since no migration comment or other table states the intended
correspondence. Flagged in the maps' own doc comment in case that
assumption is ever wrong — but without some mapping, severity can't be
written or filtered on Postgres at all.

**`GetCaseByID` also had a separate, unrelated bug**: it inner-joined
`deployment`/`deployed_product`/`product` (all nullable FKs on `work_item`,
same as `SearchCases` already documented for the same three tables), so any
case missing one of those links came back zero rows — misreported as 404
"case not found" — while still appearing correctly in `SearchCases`'s
result list, since that query already used `LEFT JOIN` for these three.
Fixed by matching `SearchCases`'s join type; `CaseView.DeploymentDetails`/
`DeployedProductDetails` are already pointer fields, so this needed no
domain/contract change, only nil-checks in the scan.

## Fixing user_repo.go's NULL-scan crash and user_type-casing bug

`POST /users/search` failed on every call whose results included a user with
no `first_name` set: `scanUser` scanned `"user".first_name`/`last_name`
(both nullable, migration 0002) directly into `domain.User`'s required
(non-pointer) `FirstName`/`LastName` string fields — pgx v5 can't scan `NULL`
into a plain `*string` destination. `email` (also nullable on `"user"`) had
the same latent bug, not yet hit in production but certain to fail the same
way. Fixed by scanning all three into intermediate `*string` vars and
`stringOrEmpty(...)`-defaulting them, same pattern as every other nullable
column fix in this file.

**`user_type` had a casing/mapping bug on top of the same NULL-scan risk**:
`user_type_enum`'s real labels (migration 0011) are `SYSTEM`/`INTERNAL`/
`EXTERNAL`/`NOT_AVAILABLE`, scanned directly into `domain.UserType` (whose
values are lowercase `internal`/`customer`/`system`/`external`) with no
translation at all — never exercised before because `user_type` was
previously always `NULL` in practice or never appeared in a search result
that got fully inspected. `userTypeFromEnum` now maps `EXTERNAL` to
`UserTypeCustomer` specifically, not `UserTypeExternal` — see
`UserTypeExternal`'s own doc comment: "the postgres source emits customer,
ServiceNow emits external" for the same underlying concept (confirmed
against `recompute_user_type`'s trigger logic, migration 0011: `EXTERNAL`
is derived from `external`/`partner`/`customer`/... roles). `NOT_AVAILABLE`
(the trigger's fallback for a user with no matching role at all) has no
domain equivalent and is left `""` — same as a `NULL` `user_type` — rather
than inventing a fifth `UserType` value nothing else expects.

## Fixing the plural/singular table-name mismatch

`case_repo.go`, `project_repo.go`, `product_repo.go`, `product_version_repo.go`,
`deployment_repo.go`, and `deployed_product_repo.go` used to query plural,
unquoted table names (`cases`, `projects`, `products`, `accounts`,
`deployments`, `deployed_products`, `case_comments`) that never existed in
`migrations/`, which instead define singular/quoted `"case"`, project,
product, account, deployment, deployed_product, split across
`work_item`+`"case"`. All six are now fixed **except one method** —
`case_repo.go`'s `CreateCase`, see below.

- **`product_version_repo.go`**: pure rename (`product_versions` →
  `product_version`, `created_at`/`updated_at` → `created_on`/`updated_on`).
  No other column was wrong.
- **`deployment_repo.go`**: same rename, plus one semantic bug beyond
  naming: `deployment.created_by` is a plain `VARCHAR` audit string (an
  email, this codebase's own convention — see e.g. `commentService` writing
  the caller's email into `comment.created_by`), never a UUID FK, so
  `JOIN "user" u ON d.created_by = u.id` would either fail to type-check or
  silently match nothing even after the table rename. Fixed by resolving
  the creator via `LEFT JOIN "user" u ON LOWER(u.email) = LOWER(d.created_by)`
  — `CreatedBy` comes back `nil` (not a fabricated `EntityRef` with an empty
  id) when the email doesn't resolve to a known user.
- **`deployed_product_repo.go`**: rename, plus `dp.product_version_id` →
  the real column `dp.version_id`. Also newly populates `Cores`/`TPS`/
  `Category` from `core_count`/`tps_count`/`product_category` — real columns
  that existed but were never selected at all (a distinct, adjacent gap,
  fixed in the same pass since it was a one-line addition once the query
  was being rewritten anyway). `update_level_info` (JSONB) → `Updates` is
  still not populated: its actual JSON shape isn't confirmed against any
  real payload, so it's deliberately left nil rather than guessed at.
- **`product_repo.go`**: `class`/`product_class_enum` don't exist anywhere
  in the migrations. The real, unambiguous equivalent is
  `product.category` (`product_category_enum`: `SOFTWARE`/`SERVICE`) —
  `domain.Product.Class`'s own values (`"software"`/`"service"`) match it
  1:1 once case-folded; `manufacturer`/`business_unit`/`unit` are different
  classification axes on the same table, not substitutes for this one.
- **`project_repo.go`**: rename, plus two fields with **no real column at
  all** (`subscriptionType`, `closureStatus`/`account.tier` — ServiceNow
  vocabulary with values like `"managed_cloud_subscription"`/`"read_only"`
  that don't match any of `project`'s several different closure-state
  columns, and `account` has no tier-like column whatsoever) — left as
  their zero value rather than mapped to a guessed-at column, with a doc
  comment explaining why. `AgentEnabled`/`KbReferencesEnabled` *do* have a
  clear real-column match despite the name difference
  (`account.ai_gen_response_enabled`/`smart_knowledge_base_suggestions_enabled`)
  and are populated from them (both nullable `BOOLEAN`s, treated as `false`
  when `NULL`).
- **`case_repo.go`**: the largest of the six — `work_item`+`"case"` is a
  genuine two-table split (not a single mis-named table), so every method
  needed a real rewrite, not just a rename:
  - `GetCaseByID`/`SearchCases`: case-specific fields
    (severity/issue_type/state/work_state/closed_on) come from `"case"`;
    everything else (number, subject, description, created_on/updated_on,
    created_by, the project/deployment/deployed-product/account ids,
    assignee, parent) comes from `work_item`, since those are common to
    every work_item type, not case-only. `SearchCases` LEFT JOINs `"case"`
    (it can return non-case types too — `service_request`, `engagement`,
    `security_report_analysis` — and `"announcement"` rows have no
    deployment/deployed-product at all), so applying a state/severity/
    issue-type/work-state filter implicitly narrows results to case-type
    rows, since a non-case row's joined `"case"` columns are always `NULL`.
    `EngagementTypes` filters/selects from the separate `engagement` table
    (`eng.type`, migration 0024) the same way, LEFT joined. `ParentCase`
    now resolves its `Type` from the parent's own real `work_item.type`
    (via `work_item.parent_id`, migration 0039 — a generic self-reference
    across every work_item type, not case-specific) instead of always
    hardcoding `"case"`; `RelatedCase` (`"case".related_case_id`, migration
    000038) is genuinely case-specific, so hardcoding `"case"` there is
    still correct. `account_id` is read directly off `work_item.account_id`
    (a real, direct column — migration 0021) rather than derived
    transitively through the project, since work_item has its own.

    **`work_item.account_id` was never populated at create time, on any of
    the five SN-first create paths.** `CreateCaseFromServiceNow`'s five
    `create*FromServiceNowQuery` inserts (case/announcement/service_request/
    engagement/security_report_analysis) all wrote `project_id`/
    `deployment_id`/`deployed_product_id` but never `account_id` — so a case
    created on `DATA_SOURCE=postgres-servicenow-dual-write` always showed a
    blank Account on its overview card, even though its project has one.
    `CreateCaseRequest` has no `accountId` field for a caller to supply
    (ServiceNow's own create response doesn't return one either — there was
    genuinely nothing to write), so every insert now derives it with
    `(SELECT account_id FROM project WHERE id = $7)`, `$7` being the
    already-bound `project_id` parameter — no new bind parameter needed. The
    plain-Postgres-only `CreateCase` (still 503s on `work_item.number` — see
    "CreateCase and case numbers" below) got the same fix via a `LEFT JOIN
    project` for consistency, even though it can't be exercised yet.
  - `CreateCaseComment`/`SearchCaseComments`: now target the real
    generic `comment` table (migration 0040, keyed by `work_item_id`, not
    `case_id`) instead of the nonexistent `case_comments` — sharing the
    same `comment_type_enum` mapping `commentTypeToEnum` in
    `comment_service.go` uses (`caseCommentTypeEnum`/`caseCommentEnumType`
    in `case_repo.go`, kept local rather than importing the service package
    per this repo's own layering rule). `CreateCaseComment` refuses
    `CommentTypeActivity` for the same reason `commentService.CreateComment`
    does (`APPROVAL_HISTORY` is ServiceNow-audit-trail-only). This also
    required a one-line, tightly-coupled fix in `case_service.go`:
    `CreateCaseComment` used to pass the resolved user's **UUID** as
    `req.CreatedBy` (matching the old, nonexistent `case_comments` table's
    assumed UUID FK); it now passes the user's **email**, matching
    `comment.created_by`'s real `VARCHAR` shape — this was a necessary,
    coupled fix, not scope creep, since the two bugs are the same
    underlying wrong-schema assumption surfacing in two layers.
  - `UpdateCase`: now a single `WITH` CTE updating both `"case"`
    (state/severity/work_state/closed_on) and `work_item` (updated_on) in
    one round trip — the `work_item` CTE's `AND EXISTS (SELECT 1 FROM
    updated_case)` guard means a nonexistent id updates nothing in either
    table, not a partial update.
  - **`CreateCase` is still broken, deliberately** — this is the one
    method that can't be fixed with a rename. `work_item.number` and
    `work_item.wso2_id` are both `UNIQUE` with no DB default and **no
    backing sequence anywhere in `migrations/`** — despite this file's own
    "Database migrations" section documenting the intended design
    ("generated from dedicated sequences via column defaults"), no
    `CREATE SEQUENCE` for either one was ever actually added, and the
    intended number *format* isn't specified anywhere either (ServiceNow's
    own case numbers look like `"CS0023001"`, but that's not proven to be
    the intended Postgres-native format). Explicitly deferred per product
    decision rather than guessed at. Whoever picks this up next needs to
    decide: a new migration adding sequences + column defaults (fulfilling
    the already-stated design), or Go-side generation with a retry-on-
    conflict loop — either way, the exact prefix/padding/format needs a
    real answer, not an invented one.

## A freshly created case silently omitted escalationLevel/isEscalated entirely

Found the same way as the project-fields gaps above (HAR diff, this time against case-creation
traffic): `GET /cases/{id}` genuinely has real Postgres backing and working code for both fields
(`GetCaseByID` already selects `current_escalation_level`/`is_escalated` and `SearchCases`'
own escalation filter already treats a NULL row as "not escalated" -- see this file's own
"escalation (isEmpty / isNotEmpty)" note), but a brand-new case has NULL for both columns
(case creation sets neither), and the read path passed that NULL straight through as `nil` --
which `omitempty` then drops from the response entirely, rather than rendering the same
"never escalated" default ServiceNow's own case response always includes (`escalationLevel:
{id: "0", label: "EL0"}`, `isEscalated: false`) from the moment a case exists. Fixed by
defaulting NULL to that same state in `GetCaseByID`, matching the semantic the filter side
already gives NULL rather than inventing a new one.

## POST /deployments/{id}/products/search dropped product.abbreviation on Postgres

`ProductRef.Abbreviation`'s own doc comment claimed "absent on the Postgres data source, whose
products table has no equivalent column" -- checked directly against real data and this is
wrong: `product.code` (migration 0015) holds exactly this value (`"wso2am"` for `"WSO2 API
Manager"`, `"wso2is"` for `"WSO2 Identity Server"`), the same vocabulary this field's own doc
comment already describes the product-updates catalogue keying on. `SearchDeployedProducts`
simply never selected it. Fixed by adding `p.code` to the query and scanning it straight into
`Product.Abbreviation` (already the correct `*string` type for a nullable column). Doc comment
corrected to match.

## GET /projects/{id} and POST /projects/search were missing most of a project's own fields

Found by diffing the Postgres and ServiceNow customer-portal responses field-for-field
(HAR capture comparison) against the real customer-portal-backend-v2 traffic: `GetProjectByID`
never selected most of `ProjectDetailsView`'s own fields, even though every one of them has a
real Postgres column (migration 0014) -- `account.ownerEmail`/`technicalOwnerEmail`,
`closureState`, `onboardingStatus`, `goLivePlanDate`, `onboardingExpiryDate`, and all six
query/onboarding-hours balances (`totalQueryHours`, `consumedQueryHours`, `remainingQueryHours`,
`totalOnboardingHours`, `consumedOnboardingHours`, `remainingOnboardingHours`) all came back as
their zero value regardless of what was actually stored. Fixed by extending `GetProjectByID`'s
query and scan:
- `mgr`/`tow` are two new `LEFT JOIN "user"` aliases resolving `account.account_manager_id`/
  `technical_owner_id` to `.email` -- the same two FKs `account_repo.go`'s own
  `accountSelectColumns` already resolves for `GET /accounts/{id}`, reused here under the same
  alias names. `TechnicalOwnerEmail`'s mapping is exact (the column is literally named
  `technical_owner_id`); `OwnerEmail` is inferred as `account_manager_id` -- the account's other
  named "owner" role, and the one ServiceNow's own project payload pairs with
  `technicalOwnerEmail` the same way. Revisit if that pairing turns out to be wrong.
- `wso2_closure_state`/`onboarding_status`/`onboarding_go_live_date`/`onboarding_go_live_plan_date`/
  `onboarding_expiry_date` scan straight into `ProjectDetailsView`'s already-pointer fields --
  no local var needed, same NULL-tolerance as every other optional column here.
- The six query/onboarding-hours balances are stored as `INTERVAL`, not a plain number;
  `EXTRACT(EPOCH FROM ...) / 3600.0` converts to hours in SQL (matching
  `project_case_stats_repo.go`'s existing `EXTRACT(EPOCH ...)` convention for a duration column)
  and scans directly into the matching `*float64` field -- a NULL interval extracts to a NULL
  numeric, preserving "not tracked" instead of becoming a fabricated `0`.

`ProjectClosureFields.ClosureState`'s own doc comment used to say "(ServiceNow data source only)"
-- stale even before this fix, since `project.wso2_closure_state` was always a real column; only
the query never read it. Corrected to say it's populated on both data sources now.

`POST /projects/search` had the same gap for `closureState` specifically, but through a second,
independent bug on top of the first: `project_repo.go`'s `SearchProjects` query never selected
`wso2_closure_state` at all (so `domain.Project` had nowhere to put it), **and**
`project_service.go`'s `domain.Project` -> `domain.ProjectView` mapping didn't copy the field
across even after it was added to the repo type -- the same shape of bug this file's own
`StartDate` fix (see "SearchProjects crashed..." below) already hit once for a different field on
this exact mapping. Both had to be fixed together: `domain.Project` gained a `ClosureState *string`
field, the repo query now selects `p.wso2_closure_state::TEXT`, and the service layer's
`ProjectView` construction now sets `ProjectClosureFields: domain.ProjectClosureFields{ClosureState: p.ClosureState}`.

Two things intentionally left untouched by this same audit, not code bugs:
- `GET /projects/{id}/features`' `acceptedSeverityValues`/`has*Access` flags being empty on some
  environments is a **migration data-backfill gap, not a code bug** -- confirmed live: migration
  000085's `ADD COLUMN`s exist, but its `UPDATE ... WHERE name = '<project type>'` backfill never
  ran, because that environment's schema is owned by a separate sync tool (its own
  `csm_migration_*` tracking tables, an entirely different numbering/naming scheme) that mirrors
  column shape but has no way to replicate entity-service's own custom seed-data logic embedded in
  a migration file. Redeploying entity-service will not fix this on its own; the backfill has to be
  run directly against that environment.
- `GET /projects/{id}/filters`' `severityBasedAllocationTime` has no real Postgres source and was
  deliberately not derived from `sla_policy` (migration 0136 and its ServiceNow-synced rows) as a
  substitute -- checked directly against real data, and none of that table's `RESPONSE`-target
  durations match the actual per-severity minutes a real project's filters response returns, so
  synthesizing a value from it would risk returning a plausible-looking but wrong number. This
  stays an explicit TODO (see `project_metadata_service.go`'s own comment) rather than a fix.

## GetProjectByID 404'd on any project with no linked account

Found in the same audit pass as the SearchProjects fix below, by explicitly
testing a project confirmed to have `account_id IS NULL` (not just spot
checking one that had an account, which the first pass over this endpoint
missed). `GetProjectByID`'s `JOIN account a ON p.account_id = a.id` was an
INNER JOIN, so a project with no linked account produced zero rows and came
back as `&apierror.NotFoundError{Msg: "project not found"}` even though the
project genuinely exists -- confirmed live against one of the 14 (of 1956)
such projects. Same class of false-404 `GetCaseByID` already had for its own
optional joins (see "Case-like work_item types" history above) before that
was fixed. Fixed the same way: `JOIN` -> `LEFT JOIN`, with `a.id`/`a.name`
scanned into nullable locals and left as `ProjectAccountRef`'s zero value
(`""`, not fabricated) when no account matched -- `Account` stays a required,
always-present object on the wire (`ProjectDetailsView.Account` has no
`omitempty` and no doc comment claiming otherwise), just with empty fields,
rather than changing the JSON contract to nullable. `ActivationDate`/`Region`
already tolerated a LEFT JOIN's NULLs without any change, since
`ProjectAccountRef` already types them as pointers.

## SearchProjects crashed on any page containing a NULL start_date/end_date/account_id

Found while auditing whether `GetProjectByID`/`SearchProjects` still work
correctly. `project.start_date`/`end_date`/`account_id` (migration 0014)
are all nullable, but `domain.Project` (the internal repository<->service
handoff type `SearchProjects` uses -- never serialized directly; `ProjectView`
is what actually reaches a caller) had non-pointer `time.Time`/`string`
fields for them, so `project_repo.go`'s scan failed with `cannot scan NULL
into *time.Time` the moment any of the 13-14 (of 1956) rows with a NULL date
reached the query -- confirmed live, and this wasn't a rare edge case: the
very first unfiltered page (most-recently-created projects first) already
contained one. Fixed by making `Project.AccountID`/`StartDate`/`EndDate`
pointers, matching `ProjectDetailsView`'s own already-pointer `StartDate`/
`EndDate` (which already carries a doc comment for exactly this "ServiceNow
may legitimately leave either unset" reality). Also fixed a second, smaller
gap found in the same pass: `project_service.go`'s mapping into `ProjectView`
never set `StartDate` at all (only `EndDate`), even though `ProjectView.
StartDate` is a real, already-documented field -- both now pass through
directly rather than being re-boxed through a local copy. `ProjectView.
Account` staying `nil` for Postgres-sourced results is unrelated and
unchanged -- that one's already documented as "(ServiceNow data source
only)", a deliberate scope boundary, not a bug. Verified by paging through
all 1956 projects (every NULL row included) with no error afterward.

## CreateCase and case numbers (Postgres data source)

**Table names are fixed.** `caseRepo.CreateCase` used to `INSERT INTO cases`, a
table that does not exist (staging has no plural entity tables: it is `work_item`
plus the `"case"` extension). It now writes a `work_item` row (type `CASE`) and a
`"case"` row in one transaction; a failure on the second insert leaves no
`work_item` row. Following the synced data, `work_item.created_by` holds the
creator's **email** (6,995 of 8,066 staging cases), so it is taken from the
`"user"` row of `req.CreatedBy` (a user id, also stored as `opened_by_user_id`);
an unknown creator is a validation error. Verified against the real schema with
`PREPARE` on staging and end to end on a local database built from all
migrations.

**Resolved by migration `0140_portal_created_work_item_numbering.sql`**, which
this section used to say was still an open product decision. `number` for
every work_item type comes from `next_portal_work_item_number()`
(`'CS-PORTAL-' || a zero-padded sequence value`, `portal_work_item_number_seq`)
-- a visually distinct prefix rules out any collision with ServiceNow's own
still-running `CS` + 7-digit sync, the same reasoning migrations `0113`/`0115`
already used for GitHub-sourced records (`CHG-GH-...`/`SR-GH-...`).
**Exception: `INCIDENT_TASK`** (migration `0201`) takes ServiceNow's format
from 0180's TASK series, `next_work_item_number('INCIDENT_TASK')`, started at
`TASK1000000` -- far above ServiceNow's range (TASK0084630 on staging,
2026-10-07), as outages did with `OUT0010000`. The cutover seed script only
moves sequences forward, so it is unaffected. `wso2_id`
(required, by `work_item_wso2_id_required_by_type`, only for the five
case-like types -- CASE/SERVICE_REQUEST/ENGAGEMENT/SECURITY_REPORT_ANALYSIS/
ANNOUNCEMENT) comes from `next_portal_wso2_id(project_id)`
(`'<project.key>-PORTAL-' || a per-project counter column`,
`project.portal_wso2_id_counter`) -- the real prefix stays recognizable as
belonging to the project, with a distinct marker inside the id rather than a
wholesale distinct prefix, since the counter (unlike `number`) is per-project,
not global. Both functions `RAISE EXCEPTION` on a bad input (an unknown
`project_id` for the latter), mapped by `mapCreateCaseError`'s existing
`P0001` branch to a `ValidationError`.

`CaseRepository.CreateCase` (`case_repo.go`'s `createCaseTx`) now dispatches
on `req.Type` to one of five `*PortalQuery` consts (`createCasePortalQuery`/
`createAnnouncementPortalQuery`/`createServiceRequestPortalQuery`/
`createEngagementPortalQuery`/`createSecurityReportAnalysisPortalQuery`),
each a near-identical CTE to its already-existing `*FromServiceNowQuery`
sibling (used by the dual-write mirror path, `CreateCaseFromServiceNow`) --
the only real difference is identity: `gen_random_uuid()`/
`next_portal_work_item_number()`/`next_portal_wso2_id($N)` generate it here,
rather than taking it from a prior ServiceNow response. Every initial state
literal (`'OPEN'`) was confirmed against the live enum catalog for each of
the five state enums, not assumed from case's own convention.
`caseService.CreateCase`'s own type switch no longer treats
announcement/service_request/engagement/security_report_analysis as
dual-write-only -- all five types work identically on the plain `postgres`
data source and `postgres-servicenow-dual-write` alike now.

What the data still says, for context on the format choice:
- Real synced `number` is `CS` + 7 digits in **one series shared by every
  work-item type** (cases, service requests, engagements, security reports,
  announcements), max `CS0442200` when checked. ServiceNow allocated them and
  the sync is still running, so a locally generated number in that same
  series could collide with a synced one -- the whole reason for a visually
  distinct prefix instead. The leftover `cases_number_seq`/`cases_wso2_id_seq`
  sequences (both 63) were never attached to any column and were dropped by
  migration `0140` itself (`DROP SEQUENCE IF EXISTS`), not reused.
- Real synced `wso2_id` is `<project key>-<per-project counter>` (prefix
  equals `project.key` for 1,101 of 1,233 linked cases; the rest are renamed
  or malformed keys) and the counters have gaps.
- The migrations define a `work_item_wso2_id_required_by_type` CHECK (a case-like
  type needs a `wso2_id`) that **staging does not have** -- staging's schema is
  built by the sync service's own migration list, which differs from this
  directory (see "Staging schema drift" below).

## Staging schema drift

Staging's schema is not built from this directory. The sync service records its
own list in `csm_sync_applied_migration` (`0001_control_plane.sql` ..
`0076_add_sf_id_columns.sql`), numbered differently from `migrations/`. Diffed
column by column (a local database built from every migration here vs staging,
68 shared tables) when checked:

- **Tables only in `migrations/`, absent from staging:** `alert_incident_mapping`
  (0103), `case_attachment` (0106/0107), `announcement_requests` (0120),
  `onboarding_step` (000075). Queries on them fail in staging with "relation does
  not exist"; none of it is a naming problem, the tables were simply never created.
- **Columns renamed in staging** (the code used the old names and failed with
  "column does not exist"): `deployment_node.subscription_key` -> `project_key`,
  `deployment_node.deployment_ref` -> `deployment_number`,
  `deployment_information.number_of_cores` -> `core_count`,
  `deployment_information.reported_created_on/reported_updated_on` ->
  `payload_created_on/payload_updated_on`, `daily_usage_summary.deployment_ref` ->
  `deployment_number`. The `deployment_*` rename is a change of meaning, not just
  of spelling: the value is a deployment **number** (`DEP000002442`), never a UUID.
- **Constraints:** the `work_item_wso2_id_required_by_type` CHECK exists in the
  migrations but not in staging.

Check the live schema, not just the migrations, before assuming a table, column or
constraint exists. `sf_id` on `user`/`account_contact`/`project_contact` and
`project.number`/`license_secrets`/`primary_secret_key`/`secondary_secret_key`
used to be on the "only in staging" list above; migrations 000075-000077 added
them here too (schema only -- see the next section for why no Go code changed).

## project.number, the three sf_id columns, and project's secret fields have no Go code yet, deliberately

Migrations 000075-000077 add `project.primary_secret_key`/`secondary_secret_key`/
`license_secrets`, `sf_id` on `account_contact`/`project_contact`/`"user"`, and
`project.number` -- schema only, no repository/service/handler/route wiring, and
that gap was checked deliberately rather than left as an oversight:

- **`project.number` mirrors `account.number`'s own precedent, including the
  "never read back" part.** `account.number` is written by `UpsertFromSalesforce`
  (`account_repo.go`) but not selected by any query, not on `AccountRow`, and not
  on `AccountView`/`AccountDetail` -- it exists purely so the Salesforce upsert has
  somewhere to put the value. `project.number`'s own migration comment says it
  follows that exact precedent, so it stays unexposed the same way until something
  needs it.
- **No code in this repo writes `project`, `account_contact`, `project_contact` or
  `"user"` rows at all** (confirmed: no `INSERT`/`UPDATE` against any of the four
  outside `UpsertFromSalesforce`, which only touches `account`). The real-time
  Salesforce sync only handles `Customer`/`account` events
  (`salesforce_event_service.go`, `internal/salesentity`) -- there is no
  project/contact/user event handler to extend, so the three new `sf_id` columns
  have no producer yet. The migration's own comment says as much for `"user"`: "has
  no mapping populating it yet."
- **`AccountContact`/`ProjectContact` expose no row-level identifier at all today**
  (`ProjectContact.ID` is the linked *user's* id, not `project_contact.id`) --
  contacts are always nested search results, never fetched by their own id, so
  there is no existing shape to add `sfId` to without inventing one.
- **The three secret columns hold credential material.** Nothing in this API
  exposes a secret today, and adding one without being asked would be a real
  security decision, not a schema follow-up -- left alone entirely.
- **The `work_item_activity.updated_on`/`updated_by` columns these migrations also
  drop are not selected anywhere** (`case_repo.go`/`incident_repo.go` only read
  `id`/`created_on`/`user_email`/`field_name`/`old_value`/`new_value`), so removing
  them needed no code change either.

## GetCaseByID's CloseNotes was silently swapped with ResolutionNotes

Found via a HAR comparison against the ServiceNow data source for the same
case: `closeNotes` was always `null` in every Postgres `GetCaseByID`
response, while `resolutionNotes` held what was actually close-notes data.
`caseLikeCloseNotesColumn` (`COALESCE(c.close_notes, eng.close_notes,
sr.close_notes, sra.close_notes, ann.close_notes)`) was scanned into a
local variable named `resolutionNotes` and assigned to
`cv.ResolutionNotes` -- but this schema has no separate `resolution_notes`
column anywhere (only `close_notes`, on all five case-like tables), and the
ServiceNow-backed path (`sn_case_service.go`) treats `CloseNotes`/
`ResolutionNotes` as genuinely distinct fields from two different upstream
values. Fixed by scanning into `closeNotes` and assigning it to
`cv.CloseNotes`; `cv.ResolutionNotes` now correctly stays `nil` on this data
source (no confirmed column for it), rather than being double-filled from
`close_notes`.

## CaseView.ProjectDetails / SearchCaseView.Project are now optional

Both were required (non-pointer) `EntityRef` fields, but `work_item.project_id`
has no `NOT NULL` constraint and a meaningful fraction of real cases have no
project linked. `project` was still an `INNER JOIN` in both `GetCaseByID` and
`SearchCases`, which silently dropped/404'd those cases entirely -- the same
class of bug the deployment/deployed-product/product joins had (see the
enum-casing/false-404s section above), just for a required rather than
optional field, so fixing it required a response contract change: both
fields are now `*EntityRef`, `null` when absent, and `project` is a
`LEFT JOIN` in both queries. The ServiceNow-backed paths
(`sn_case_service.go`) always populate a value, so they only needed the
pointer wrap, not a nil-check.

## Case-like work_item types, GetMe roles/groups, and groups

**GetCaseByID/SearchCases now serve all five case-like work_item types**
(`validCaseType` in `case_service.go`: case/engagement/service_request/
security_report_analysis/announcement), not just `CASE`. Previously
`GetCaseByID` hard-filtered `wi.type = 'CASE'`, so the other four 404'd on
detail lookup even though `SearchCases` already returned them; `SearchCases`
itself defaulted to *no* type restriction when the caller passed no `types`
filter, which meant every work_item type (including change requests,
incidents...) leaked into unfiltered case search results. Both are fixed via
`caseLikeWorkItemTypes`/`caseLikeJoins`/`caseLike*Column` (`case_repo.go`):
`state`/`cause`/`close_notes`/`resolved_on`/`closed_on` are `COALESCE`d
across whichever of the five extension tables actually matches (exactly one
ever does, since each is a shared-PK extension keyed to a specific
`wi.type`) — `announcement_state_enum`'s `CLOSE` (not `CLOSED`) is
normalized to match the other four's vocabulary. `severity`/`issue_type`/
`current_escalation_level`/`is_escalated` remain `"case"`-only, since no
other extension table has those columns. `work_state`/`resolution_code` are
not: see "Work state and resolution code on non-case types" below.
`GetCaseByID` also now populates `Cause`/`ResolutionCode`/`ResolutionNotes`/
`ResolvedOn`/`EscalationLevel`/`IsEscalated` for the first time — real
columns that were simply never selected before, not previously believed
unavailable. `EscalationLevel` strips `case_escalation_level_enum`'s `EL`
prefix (`'EL2'` -> `"2"`) per `CaseView.EscalationLevel`'s own doc comment.

**`GetMe.Roles`/`GetMe.Groups`** were hardcoded to empty slices even though
the tables to back them already existed and were queried elsewhere:
`UserRepository.GetUserRoles`/`GetUserGroups` (`user_repo.go`) join
`user_role`/`role` and `team_member`/`team` respectively for the caller's
own id.

**`POST /groups/search`** is now Postgres-backed too (`group_repo.go`),
against `team` (migration 0033) — "mirror[s] a hand-curated allow-list of
ServiceNow's OOB sys_user_group / sys_user_grmember tables" per that
migration's own comment, the same concept `GroupService` searches.
`domain.Group.Active` has no backing column and is hardcoded `true`;
`Parent` has no hierarchy column on `team` and is always `nil`.

**Not wired up**: `project_type` has no corresponding field anywhere on
`domain.Project`/`ProjectDetail` today, so there is nothing to populate
without first adding a new response field — left alone pending that
decision, not overlooked.

## IT services (CMDB services)

`service` (migration 0044) is the CMDB service catalogue: `incident`,
`incident_task`, `change_request`, `outage` and `cloud_monitor` reference it
via `service_id`, `service_offering` via `parent_id`, and its five group
columns (migration 0075) reference `"group"`. `ITServiceRepository.SearchITServices`
(`it_service_repo.go`) wires `POST /services/search` up to it on Postgres;
previously this route only existed on the ServiceNow data source.
`domain.ITService.Class` is mapped from `service.category` (a free-text
`VARCHAR`) — the same choice already made for `product.category` ->
`domain.Product.Class` in `product_repo.go`, since there's no column
literally named "class". `BusinessCriticality` maps 1:1 (case-folded) via
`itServiceBusinessCriticalityFromEnum`. `ServiceClassification`
(business_service/technology_management_service/application_service on the
ServiceNow data source) has no corresponding column on `service` at all —
`category`/`subcategory` are free text, not drawn from that three-value set
— so it is always left `nil` on Postgres rather than guessed at.

**`SupportGroup` comes from `service.support_group_id`** (LEFT JOIN `"group"`).
It was never selected before, so the CSM portal's Create Incident page — which
defaults the incident's assignment group to the service's support group —
always showed it blank. Use `support_group_id`, not `service.assignment_group_id`:
support group is the ServiceNow/CSDM "team that handles this service's
incidents" field (in synced data, 98% of incidents with both a service and an
assignment group carry that service's support group), while a CI's own
`assignment_group` is a different, generic field that no synced service sets.
The incident's own group belongs on `work_item.assignment_group_id`.

## time_card.state/issue_complexity became real enums; case_id now targets work_item

A later migration revision changed `time_card`: `state`/`issue_complexity`
went from plain `VARCHAR` to real enums (`time_card_state_enum`:
`PENDING`/`SUBMITTED`/`APPROVED`/`REJECTED`/`RECALLED`/`PROCESSED`/`UNKNOWN`;
`time_card_issue_complexity_enum`: `NOT_APPLICABLE`/`LOW`/`MEDIUM`/`HIGH`),
and `case_id`'s FK retargeted from `"case"(id)` to `work_item(id)` -- a time
card can now be logged against any case-like work_item type, not just
`CASE`. Both changes broke `time_card_repo.go` in the same ways this
codebase has hit repeatedly:

- Every write (`CreateTimeCard`'s `'submitted'` literal, `UpdateTimeCardFields`/
  `TransitionTimeCardState`/`DeleteTimeCard`'s `state = 'submitted'` checks,
  `issue_complexity` writes) used lowercase values against columns that are
  now `UPPER_SNAKE_CASE` enums -- fixed with `strings.ToUpper(...)` at every
  write site, plus `::text::time_card_issue_complexity_enum`/
  `::text::time_card_state_enum` casts on `$`-bound parameters (not literal
  SQL text, which resolves its own type from context and only needed the
  casing fix) to avoid the same pgx v5 codec issue this file's date fields
  already work around -- see `TransitionTimeCardState`'s own comment, which
  mirrors `case_repo.go`'s `updateCaseQuery` pattern exactly (one `::enum`-cast
  usage of a placeholder, one bare-text-comparison usage of the same
  placeholder, in the same statement).
- Every read (`scanTimeCardView`) needed a `::TEXT` cast plus
  `strings.ToLower(...)` back to the domain's lowercase convention, and a
  nullable-safe scan (`state`/`issue_complexity` have no `NOT NULL`
  constraint) -- both were previously scanned straight into `*string`
  response fields with no case-folding.
- `timeCardFromJoins`/`SearchCaseTimeCards`'s `JOIN "case" c ON c.id = tc.case_id`
  would now silently exclude any time card logged against a non-`CASE`
  work_item type (same "false exclusion via the wrong join" bug class as
  `GetCaseByID`'s project/deployment joins). Fixed by joining `work_item`
  directly on `tc.case_id` -- `"case"` was only ever needed for `wi.number`/
  `wi.subject`, both already on `work_item` itself, so this also simplifies
  the query.

## case_escalation/case_escalation_notification_list back EscalationService's SearchEscalations

The same migration batch added `case_escalation`/
`case_escalation_notification_list`, finally giving `EscalationService`
(and the case-scoped `CaseEscalationService` wrapper over it) something to
read on Postgres -- previously entirely ServiceNow-only.
`escalation_repo.go`/`escalation_service.go` implement `SearchEscalations`
only; `CreateEscalation` stays a `ServiceUnavailableError` on Postgres,
since neither an escalation-level-transition rule (does `ESCALATE` always
mean "current level + 1", capped at `EL5`? is there a per-case-type
override?) nor a notification-recipient rule (watchers? the assigned
engineer? an account's own escalation contacts?) exists anywhere in this
schema to derive from -- guessing either would be inventing business logic,
not reading it off a table. `domain.ChoiceListItem.Label` for an escalation
level is set to the same plain `"0".."5"` id as `ID` (`case_escalation_level_enum`'s
`EL` prefix stripped) rather than a fabricated display string this data
source has no real source for -- unlike the ServiceNow data source, which
gets both `id` and `label` directly from ServiceNow's own choice-list
payload.

`routes.go`'s `caseEscalationHandler`/`escalationHandler` are now
constructed unconditionally (Postgres or ServiceNow), since
`CaseEscalationService` is already a thin, fully generic wrapper over
whichever `EscalationService` it's given -- no changes needed there at all.

**Not yet verifiable against real data**: `case_escalation`/
`case_escalation_notification_list`'s migration hasn't actually been
applied to the staging database this was checked against (same gap as
`case_attachment`/`alert_incident_mapping`/`work_item_tag` -- see the
"Fixing wso2_id" section's own note on checking directly against the
database rather than trusting a migration file's presence in this repo).
The code matches the migration's schema definition exactly; it just
couldn't be exercised against live rows yet.

## CaseView/SearchCaseView/Case.InternalID stays a required string (fixed the panic without changing the wire type)

Found via a direct query against `work_item` grouped by `type`: `wso2_id`
(`InternalID`) is `NULL` for a handful of real `CASE`/`ENGAGEMENT`/
`SERVICE_REQUEST` rows, even though the `work_item_wso2_id_required_by_type`
`CHECK` constraint (migration 0021) requires it `NOT NULL` for those
types -- **the constraint is evidently not actually enforced against this
data** (added after these rows already existed, and never backfilled/
revalidated). Don't trust a `CHECK` constraint's claim over what a direct
query of the actual data shows.

**First attempt made `InternalID` `*string`** (rendering `null` for those
rows) but still scanned into a non-pointer `string` local, so it kept
crashing in production with `cannot scan NULL into *string` -- fixing the
wrong half of the problem. **Second attempt** made the scan itself
`*string`-safe but kept the `*string` response type -- CodeRabbit caught
that this breaks compatibility: `openapi.yaml` declares `internalId` as a
required, non-nullable `string` in every `Case`/`CaseView`/`SearchCaseView`/
`GlobalSearchCase` schema, and the customer-portal Ballerina client and
backend-v2 both declare it as plain `string` too -- a Ballerina client
deserializing `{"internalId": null}` into a non-nilable `string` field
throws at runtime (unlike Go, which silently zero-values it). Changing the
wire type to fix an internal scan panic isn't worth risking every other
consumer of this response.

**Final fix**: `InternalID` stays `string` on `Case`/`CaseView`/
`SearchCaseView` (unchanged wire contract, `""` when absent, matching the
declared OpenAPI schema and every other consumer's expectations). The panic
is fixed entirely on the scan side: `GetCaseByID`/`SearchCases`/
`scanUpdatedCase` (`case_repo.go`) scan `wso2_id` into a `*string` local,
then `stringOrEmpty(...)` converts it to `""` for the response -- crash-safe
internally, contract-identical externally. No changes needed on the
ServiceNow-backed path (`sn_case_service.go`), since its raw case struct
already carries `InternalID` as a plain string with no equivalent nil risk.

**`CaseView`/`Case`.`Severity`/`IssueType`/`State` are now optional too --
a much bigger version of the same problem.** A direct query against
`"case"` (7,681 real `CASE` rows) found `severity IS NULL` for **86%**
and `issue_type IS NULL` for **99%** of them -- not an edge case, the
common case (`state IS NULL` for only 5 rows, but still non-zero).
`Severity`/`IssueType`/`State` were required (non-pointer) fields on both
`domain.Case` and `CaseView`, so the overwhelming majority of real case
responses were rendering `"severity": ""`/`"issueType": ""` -- values that
aren't even valid `domain.CaseSeverity`/`CaseIssueType` labels, let alone
real ones. `SearchCaseView.Severity`/`IssueType` were already `*string`
(so already correct); only its `State` needed the same fix. Fixed by
making all five (`Case.Severity/IssueType/State`, `CaseView.Severity/
IssueType/State`, `SearchCaseView.State`) pointers, and
`CaseRepository.UpdateCase`'s `previousSeverity` return value too (used for
its own internal LOW-severity-boundary check — see "Event Hub publishing"
above, `recomputeTimeCardsBillable` — which treats a nil severity as "not
LOW" on either side of the comparison rather than crashing or silently
comparing against `""`).

The ServiceNow-backed path (`sn_case_service.go`) always supplies a real
value for these three, so its many read sites (map lookups keyed by
severity/state, string conversions, equality checks against
`domain.CaseSeverityLow` and friends) needed dereferencing rather than a
contract change of their own -- `derefSeverity`/`derefState`/
`ptrOfCaseSeverity`/`ptrOfCaseIssueType` (`user_service.go`) bridge that
without introducing a second parallel set of nil-handling logic on the SN
side. `domain.UpdatedCase.State`/`Severity` (the `PATCH /cases/{id}`
response) became pointers too, matching the sibling `WorkState` field's
existing pointer convention there.

## Incident, Problem, IncidentTask, and Conversation (migrations 000057-000060, 000066)

A large schema addition (10 migrations: `conversation`, `incident`, `problem`,
`change_task`, `communication_plan`, `communication_task_definition`,
`incident_alert`, `incident_alert_task`, `incident_task`, plus incident/
problem subcategory lookup tables) landed on `dev-app-csm-portal` in one
batch. None of these 11 tables exist on the staging database this was
developed against yet (checked directly) -- same recurring gap as several
other recently-merged migrations. `change_task`/`communication_plan`/
`communication_task_definition`/`incident_alert`/`incident_alert_task` have
**no existing endpoint on any data source** to wire up at all (no
`sn_*_service.go` for any of them) and are left entirely unimplemented --
nothing to back. `Incident`/`Problem`/`IncidentTask`/`Conversation` do have
existing ServiceNow-only endpoints; new `incident_repo.go`/`problem_repo.go`/
`incident_task_repo.go`/`conversation_repo.go` (+ matching `*_service.go`)
wire up the read side of all four, following the standard "SN branch vs.
Postgres branch, same service interface" pattern.

**`IncidentView`/`SearchIncidentView`/`ProblemDetail`/`SearchProblemView`/
`IncidentTaskDetail` render State/Priority/Category/Subcategory/ContactType/
ResolutionCode as plain, unvalidated strings** (per those fields' own doc
comments) -- so reads need no enum reconciliation against
`domain.IncidentState`/`IncidentPriority`/etc at all; the real Postgres enum
text is simply passed through as-is. Only the **search filter path** uses
the strict domain enums (`SearchIncidentsFilters.Priorities`, the generic
`Filters` array's `"state"` on both Incident and Problem), and reading the
migration SQL side by side with `domain.go`'s Go constants (a static,
line-by-line comparison, not something that needed live data) turned up
three real, easy-to-miss mismatches:

- `incident_state_enum`'s cancelled label is `'CANCELED'` (one L), not
  `domain.IncidentStateCancelled`'s `"CANCELLED"` (two Ls).
- `incident_priority_enum` has no `'PLANNING'` label at all (only
  `CRITICAL`/`HIGH`/`MODERATE`/`LOW`) -- `domain.IncidentPriorityPlanning`
  is rejected with a `ValidationError` on this data source rather than
  silently dropped or bound into an invalid enum cast.
- `conversation_state_enum`'s closed label is `'CLOSE'` (no D), not
  `domain.ConversationStateClosed`'s `"CLOSED"`.

`incidentStateToEnum`/`incidentPriorityToEnum` (`incident_service.go`) and
`conversationStateToEnum`/`conversationStateFromEnum` (`conversation_repo.go`)
hold these mappings, used consistently on every read/write/filter path so
none of them can drift from the others -- each has a unit test locking in
the exact mismatch. `domain.ProblemState`'s values match `problem_state_enum`
by identity (a rarer case in this codebase where no mapping was needed at
all).

**`ParseIncidentFieldFilters`/`ParseProblemFieldFilters`/
`ParseIncidentTaskFieldFilters` (the existing `incident_filters.go`/
`problem_filters.go`/`incident_task_filters.go`) are NOT reused for the
Postgres data source's "state" filter.** All three translate a caller's
`domain.IncidentState`/`ProblemState` value into ServiceNow's own raw
numeric state keys (`parsedIncidentFilters.StateKeys` and siblings) --
correct for that data source, meaningless for Postgres's own clean enum
columns. Each new `*_service.go` has its own
`parse*FieldFiltersPostgres` that reuses those files' field/op allow-lists,
`requireFilterValues`/`badFilterCombo` helpers, and (for Incident)
`parseIncidentFilterDate`/`parseIncidentFilterBool` (both fully
data-source-agnostic), but maps `"state"` through the Postgres-specific enum
functions above instead. `incident_task`'s SN state choice list has no
confirmed-complete, unambiguous enum at all (see
`parsedIncidentTaskFilters.StateKeys`'s own doc comment) -- Postgres's own
`incident_task_state_enum` has no such ambiguity, so that data source
accepts the enum's own label strings (case-insensitive) directly in the
`"state"` filter, a deliberate, documented divergence from SN's
raw-integer convention for the same field.

**Filters with no backing column are accepted, validated, and silently not
applied** (never rejected outright) -- matching `changeRequestWhereClause`'s
own established precedent for the identical class of gap (e.g.
`change_request`'s own `assignmentGroupId`): `assignmentGroupId` on all
three of Incident/Problem/IncidentTask, `configurationItemId`, and
`productName` on Incident. `businessServiceId` on Incident IS applied,
mapped to `incident.service_id` -- "business service" is ServiceNow's own
name for what this schema calls `service`, the same identification
`service_offering_repo.go` already makes. `assignedUserId` on
Problem/Incident IS applied too, mapped to `work_item.assigned_to_id`, a
real, direct column. `madeSla`/`slaViolated` on Incident map to
`incident.is_sla_met` and an `EXISTS`/`NOT EXISTS` check against `sla.has_breached`
(migration 0048) respectively.

**`SearchIncidentActivities` reuses `scanCaseActivity`'s exact query shape**
(`case_repo.go`) -- an activity feed entry (comment or field change) is not
inherently case-specific, and `comment`/`work_item_activity` are both keyed
by the generic `work_item_id`. Unlike `SearchCaseActivities`, there is no
`case_attachment`-equivalent table for incidents, so this feed can never
have an `"attachment"` kind entry.

**`UpdateConversation` and `CreateConversation` are both implemented.**
`CreateConversation` takes its number from `next_portal_work_item_number()`
(migration 0140), starts the conversation `ACTIVE`, and stores the first
message the way csm-sync-service lands ServiceNow's `u_initial_message`:
`work_item.subject` (first 100 runes) and `work_item.description` (in full).
`InitialMessage` on reads is that description, falling back to the earliest
comment. Under dual-write it is ServiceNow-first and synchronous, like
`createProblemSNFirst`, so the row carries ServiceNow's id and later comment
mirrors target a conversation ServiceNow knows. Migration 0190 replaced
0146's internal-only INSERT policy on `conversation` with `conversation_write`
(internal or project member, same as `case_write`); the work_item and
conversation rows are inserted as two statements in one transaction because
that policy's work_item lookup cannot see a sibling CTE's insert. Without
this, every Novera chat on Postgres failed at create and nothing was
persisted.

**`SearchConversations` picks the page first, and its COUNT carries no display joins.**
`conversationSearchQueries` (`conversation_repo.go`) counts over `work_item`/`conversation`
only (every filter reads just those two), and selects the page's `wi.id`s in an inner query
(WHERE, the caller's sort with the `wi.id` tie-break, LIMIT/OFFSET) before joining the
project, linked case and creator onto those rows, as `SearchCases` does. The creator is a
`LEFT JOIN LATERAL ... ORDER BY id LIMIT 1` on `LOWER(email)`, not a plain join: `"user".email`
is not unique (111 duplicated addresses on staging), so the plain join fanned a conversation out
into one row per user while the COUNT disagreed, and for some free-text terms the planner chose a
nested loop that rescanned `"user"` once per matching conversation (hundreds of milliseconds of
database time per query, two queries per request). Do not put the display joins back into the
COUNT or the inner page query. `conversation_repo_search_integration_test.go` replays the
previous SQL as an oracle against a real RLS-forced database (internal caller, project member and
stranger, every filter, both sorts, page by page) and `TestConversationSearchQueries` pins the shape.

**`CreateProblem`/`CreateIncident` are now implemented on the plain-Postgres
data source too**, via `next_portal_work_item_number()` (migration 0140 --
see "CreateCase and case numbers" above): `ProblemRepository.CreateProblem`/
`IncidentRepository.CreateIncident`, called from `problemService`/
`incidentService`'s own `CreateProblem`/`CreateIncident` when `s.snMirror ==
nil`, alongside the pre-existing `createProblemSNFirst`/`createIncidentSNFirst`
dual-write paths (which already worked this whole time under
`DATA_SOURCE=postgres-servicenow-dual-write`, taking id/number from
ServiceNow's own response instead of generating them -- the plain-Postgres
gap this closes was specific to a deployment with no ServiceNow mirror at
all). Neither needs `wso2_id`: both are excluded from
`work_item_wso2_id_required_by_type`. `problem.state` has no column default
of its own (unlike `incident.state`, which defaults to `'NEW'`), so
`CreateProblem`'s portal path hardcodes it to `'NEW'::problem_state_enum`
explicitly. `CreateConversation` followed later -- see above.

**`HandOffIncidentToSpecialist` is implemented on Postgres** and writes
Postgres only -- no ServiceNow call, in dual-write mode too.
`incident_handoff_service.go` ports `IncidentHandoffUtils.handOff` (the
"Escalate to Special Ops" UI action): eligibility as 409s, and one
transaction that moves `work_item.assignment_group_id`, clears the assignee,
opens a TASK-numbered `[Runbook Task]` (in the same Special Ops group --
WSO2 SRE Team no longer exists) and writes the reason JSON as a work note.
Routing is configuration, not code or tables: `SPECIALIST_HANDOFF_CONFIG`
(one line of JSON, `specialist_handoff_config.go`, validated at startup --
a bad value refuses to start) lists products, each with its service ids,
its Special Ops teams (`key` = the handoff's `escalationTeam`, `label`,
`groupId`) and an optional GitHub repo. A product with several teams
(Choreo) requires `escalationTeam`; one with a single team (Asgardeo) takes
it and records no team, as SN does. `GET /specialist-handoff-teams?serviceId=`
feeds the dialog, and `IncidentView.CanHandOffToSpecialist` (SN's
`canEscalateToSpecialOps`; false when the incident is already with any of
its product's groups) is computed in the service from the config.
The GitHub issue and the "Escalated to Special Ops team." note follow,
best effort. Each product's repo picks a token by `github.credential`
(default: its owner) from the secret `SPECIALIST_HANDOFF_GITHUB_TOKENS`
(`{"<credential>":"<token>"}`, falling back to `GITHUB_TOKEN`), one client
per credential (`WithHandoffIssueCreators`), independent of the
change-request GitHub sync. No token: the handoff still succeeds and reports
`githubIssueError`. No webhook -- SN never reads anything back from the
issue. `IncidentView.SpecialistHandoff` is derived at
read time from those notes and the task, as SN's `getHandoffSummary` does.

**`UpdateProblem`/`UpdateIncident` are also not
implemented**: `UpdateProblem.Transition` is validated
server-side by ServiceNow's own workflow engine with no fixed, confirmed
transition rule set to reimplement (see that field's own doc comment --
deliberately not a closed enum for exactly this reason);
`UpdateIncident` touches several fields with no backing column at all
(`AssignmentGroupID`, `ConfigurationItemID`, `WatchList`) alongside ones
that do, and would need `comment`-table side effects for
`AdditionalComments`/`WorkNotes` mirroring `caseService.UpdateCase`'s own
comment-on-update behavior -- deferred as a unit rather than
half-implemented.

`IncidentView.WatchList`/`LinkedServiceRequests` are always empty slices on
this data source (never populated) -- `work_item_watcher` could back the
former (same table `SetCaseWatchList`/`fetchCaseWatchers` already use for
cases) and `work_item.parent_id` reverse lookups could back the latter,
matching the pattern `ProblemRepository.GetProblem`'s `LinkedIncidents`
already uses for `incident.problem_id`'s own reverse lookup -- left as a
known, flagged gap rather than built out further given the size of this
change, not because either is infeasible.

**`SearchIncidentActivities`/`SearchCaseActivities` verify the id is
actually an incident/case-like work item before reading its activity
feed** (`EXISTS (SELECT 1 FROM incident WHERE id = $1)` and the
`caseLikeWorkItemTypes`-filtered equivalent respectively) -- found as a
real IDOR during review: `comment`/`case_attachment`/`work_item_activity`
are all keyed by the generic `work_item_id` with no type filter of their
own, so without this check a caller could pass any other work item's UUID
(a change request, a different case, ...) through either endpoint and read
that record's comments/attachments/field changes instead of a 404.
`SearchCaseActivities`'s copy of this gap pre-dated this change (inherited
from the original comment/attachment UNION ALL implementation) and was
fixed alongside the new incident one rather than left for later, since it's
the identical bug.

## change_request.change_model and work_item_activity (migrations 0056/0055)

Two small, unrelated migrations, both unverified against real data (neither
table/column exists on the staging database this was developed against
yet -- same recurring gap as several other recently-added tables in this
codebase).

**`change_model`** turned out to be `domain.ChangeRequestType`'s real
backing column -- previously undiscovered because the *other*
change-request-type-shaped column, `change_request.change_request_type`
(INFRA/GENERAL), is a completely different, unrelated classification (see
this file's own "Fixing the plural/singular table-name mismatch" section).
`change_model`'s real enum labels (`AZURE`/`CHANGE_REGISTRATION`/
`CLOUD_INFRASTRUCTURE`/`EMERGENCY`/`INFRA`/`NORMAL`/`STANDARD`/
`UNAUTHORIZED_CHANGE`) only partially overlap `domain.ChangeRequestType`'s
existing values (`standard`/`normal`/`emergency`/`azure` case-fold
directly; `model`/`site_reliability_ops` have no equivalent on this data
source, rejected with a `ValidationError` on `PatchChangeRequest` rather
than silently dropped) -- the four with no existing domain constant
(`change_registration`/`cloud_infrastructure`/`infra`/`unauthorized_change`)
were added as new values rather than dropped, since they're genuine
ServiceNow change-model choices, not noise.
`changeRequestChangeModelToType`/`changeRequestTypeToChangeModel`
(`change_request_repo.go`) hold the mapping both directions; `Type` is now
read on `SearchChangeRequestView` and writable via `PatchChangeRequestRequest.Type`.

**`work_item_activity`** is the field-change audit table this schema
previously had none of (`CaseRepository.SearchCaseActivities`'s own doc
comment used to say exactly that). `SearchCaseActivities` now adds a third
`UNION ALL` branch over it, gated on `req.IncludeFieldChanges`, following
the same "SN branch vs. Postgres branch, same service interface" pattern --
no route/request/response shape changed, this just makes an existing,
previously-inert request field actually work. Each `work_item_activity` row
is one single field mutation with no confirmed grouping key (e.g. a shared
timestamp) to bundle several simultaneous changes into one activity entry
the way a ServiceNow journal entry might, so each row becomes its own
`CaseActivity` with a single-element `Changes` slice rather than guessing
at a bundling rule. `FieldChange.FieldLabel` is a humanized rendering of
the raw `field_name` column (`caseActivityFieldChangeLabel`, the same
space-separated-title-case convention `taskSlaStageDisplay` already uses
for a raw enum label) -- there's no field-name-to-display-label mapping
anywhere else in this schema to defer to instead.

## Instances and usage tracking (deployment_node, hourly_usage_summary, daily_usage_summary, deployment_information)

Migration 000054 added a 6-table cluster mirroring ServiceNow's product usage
tracking (`deployment_node`, `deployment_information`, `hourly_usage_summary`,
`daily_usage_summary`, `monthly_usage_summary`,
`product_usage_map`) -- see that migration's own doc comment for the full
shape. `project_daily_summary` was dropped from this cluster (it never had a
consuming endpoint -- see the "not wired up" note below) to match the
identically-named table's removal from `operations/csm-sync-service`'s own
copy of this schema. This finally gives the previously ServiceNow-only "instance" concept
(`InstanceService`, `POST /instances/*`) and the two
`/deployed-products/{id}/metrics*` endpoints something to read on Postgres.
`instance_repo.go`/`instance_service.go` are new; `deployed_product_repo.go`/
`deployed_product_service.go` gained the two metrics methods (previously
unconditional `ServiceUnavailableError` stubs).

**"Instance" is `deployment_node`.** `Instance.Key` is `node_id`;
`Instance.Metadata` comes from that node's latest `deployment_information`
row (by `payload_updated_on`). `CoreCount` is `deployment_information.core_count`,
a real integer column (an earlier revision parsed a free-text `number_of_cores`).
`Updates` has no backing column on `deployment_information` at all
(`deployed_product.update_level_info` is a different, per-deployed-product
concept, not per-node) and is always `nil`.

**Column names follow staging, not what an older revision of this file's own
migration used to create.** Staging's schema is built by the sync service,
which renamed columns this code was written against:
`deployment_node.subscription_key` -> `project_key`, `deployment_node.deployment_ref`
-> `deployment_number`, `deployment_information.number_of_cores` -> `core_count`
and `reported_created_on/reported_updated_on` -> `payload_created_on/
payload_updated_on`, `daily_usage_summary.deployment_ref` -> `deployment_number`.
Migration `0054_usage_tracking_tables.sql` used the old names until this was
caught (checked directly against `operations/csm-sync-service`'s own copy of
the same migration and this repo's own `instance_repo.go`, which already
queried the *new* names) and fixed to match both — with the old names,
`SearchInstances`, `SearchInstanceMetrics` and `SearchInstanceUsage` failed on
staging with "column does not exist". Still worth checking the live schema
before trusting any migration file, here or elsewhere.

**Project/Deployment/DeployedProduct references, verified against staging.**
`deployment_node.product_version_id` is a real foreign key, so `Product` is
always reliable. `deployment_node` has no foreign key to project, deployment or
deployed_product, only two free-text columns copied from the reported payload,
and `instanceRefJoins` (`instance_repo.go`) uses them like this:
- `project_key` -> `project.key` (unique, present on every node), so a node's
  Project never depends on its deployment resolving.
- `deployment_number` -> `deployment.number` (unique) **only if that deployment
  belongs to the node's own project.** The reported value is not always a
  deployment number: staging has a sys_id-like hex string and a bare `"320"` that
  equals the number of a deployment in a *different* project, so matching the
  number alone would attach those nodes to the wrong project. Requiring the
  project to agree leaves them unresolved (Deployment/DeployedProduct nil, Project
  still set). The old code cast `deployment_ref` to a UUID, but the value is a
  deployment number, never a UUID, so that join could not match anything.
- `DeployedProduct` additionally requires `deployed_product.version_id` to match
  the node's product_version, since `deployment_id` alone doesn't uniquely
  identify one.

Checked against staging's 16 nodes: 14 resolve to a project, 11 to a deployment,
none to a deployment of another project, and filtering by project or deployment
matches independent SQL counts. The same resolution is reused by
`deployed_product_repo.go`'s `resolveDeployedProductNodes` for the two
`/deployed-products/{id}/metrics*` endpoints. **Data gap:** no
`deployment_information.node_id` matches any `deployment_node.node_id` in staging
(the former are sys_ids and `TEST2`/`TEST3`), so no instance has `Metadata` there.

**Metrics vs. usage vs. usage-stats read three different tables, not one,
because only one of them carries what each endpoint needs:**

- `SearchInstanceMetrics`/`InstanceDataPoint` (CoreCount, JDKVersion, raw
  `DeploymentMetadata`) reads `deployment_information` -- the only table
  with JDK version or the raw deployment-info JSON at all.
- `SearchInstanceUsage`/`InstanceSummary` (an open `map[string]int` of count
  types per day) reads `hourly_usage_summary` -- per-node, per-day, per-count-type
  facts (`count_type` in practice holds `CORES`/`TPS`/`MTX`/`MAU`, but
  nothing enforces that set; it stays a free string, same reasoning as the
  migration's own comment on that column).
- `SearchInstanceUsageStats` reads `daily_usage_summary` instead of
  `hourly_usage_summary`, specifically because `daily_usage_summary` is the only one
  of the two with a `data_source` column (`usage_data_source_enum`:
  `API_CALL`/`FILE_UPLOAD`) -- `InstanceStatsFilters.DataSource` (an int, 1
  or 2) only has something to filter against there.
  `instanceDataSourceEnum` (`instance_service.go`) maps 1/2 to the enum
  labels; an unrecognized value is a `ValidationError`, not a silent no-op.
- `SearchInstanceMetricsStats` has the same `DataSource` field on its
  request type (`InstanceStatsFilters` is shared), but `deployment_information`
  has no data-source column at all -- there's nothing to filter on Postgres.
  Rather than silently ignore a caller-supplied `dataSource`, a non-nil value
  is rejected with a `ValidationError` before the repository is ever called
  (same "reject explicitly rather than silently ignore an unsupported
  filter" convention as `SearchDeployedProducts`' `ProductCategories`
  rejection). It aggregates the `CORES` reading across every matching
  instance into one total per day (the only numeric metric
  `deployment_information` carries); `Summary.Current` is the most recent
  day's total in range, `Min`/`Max`/`Avg` are computed across those daily
  totals.

**A known, accepted ambiguity**: `deployment_information.node_id` is a plain
string, not a foreign key to `deployment_node.id` -- so if the same `node_id`
text were ever reused by two different `deployment_node` rows (the
migration's own "identity not consistent upstream" warning suggests this is
possible upstream), `SearchInstances`' metadata lookup could attach the same
latest snapshot to both. Not fixable within this schema: `deployment_information`
has no other way to identify which specific node row it belongs to.

**`monthly_usage_summary` is not wired up.** No existing endpoint's response
shape has a monthly-granularity rollup concept to serve from it;
`product_usage_map` (a product-code -> display-unit lookup) has no consuming
field either. Left unused rather than exposed speculatively, same as other
tables with no current caller elsewhere in this file.

## Service offerings and task SLAs

`service_offering` (migration 0045) is now Postgres-backed
(`service_offering_repo.go`): `POST /service-offerings/search`, previously
ServiceNow-only. `parent_id` (FK into `service`, migration 0044) maps to
`ServiceOffering.Service`; `SearchServiceOfferingsFilters.ServiceIDs` filters
on it.

`sla`/`sla_policy` (migrations 0047/0048) back `TaskSlaService`
(`task_sla_repo.go`) -- previously ServiceNow-only `POST /task-slas/search`/
`GET /task-slas/{id}`. Both tables are real and populated in the staging
database (66 `sla_policy` rows, 128k+ `sla` rows at the time this was
checked) -- unlike several other recently-added tables in this codebase,
this one could be verified against live data. `sla.stage`/`sla_policy`'s
various enum columns are rendered as space-separated title case
(`"IN_PROGRESS"` -> `"In Progress"`) to match the ServiceNow-backed
implementation's own display convention (`view.Stage = t.Stage.Label`, a
human SN label, not a raw enum).

**`BusinessTimeLeft`/`BusinessElapsedTime`/`TaskSlaDefinitionDetail.Duration`
are now populated**, via `formatDurationSeconds` (`task_sla_repo.go`),
which renders an `EXTRACT(EPOCH FROM ...)` duration as a compact
human-readable string (e.g. `"9 Days 22 Hours 11 Minutes"`), dropping any
zero-value leading/trailing unit. This was previously left `nil` on the
belief that no rendering format could be confirmed -- but checking the
actual consumer (`apps/csm-portal/webapp`'s `caseSlaMapping.ts`/
`CaseSlaTable.tsx`, `apps/csm-portal/microapp`'s `SlaTab.tsx`) showed it
renders this string as a completely opaque label with no parsing at all
(`` `${value} left}` ``/`` `${value} elapsed}` ``), so any clear
human-readable rendering is safe -- unlike, say, `change_request_repo.go`'s
`calendar_duration`, which stays `nil` because nothing confirms its
consumer treats it the same way. `BusinessElapsedTime`/`BusinessTimeLeft`
map to `sla.business_duration`/`sla.remaining_business_duration`
respectively (confirmed against real rows: `business_duration` tracks
elapsed *business* time so far, matching `business_elapsed_percentage`'s
own existing semantics, and is a real, distinct value from the wall-clock
`sla.duration`/`remaining_actual_duration` columns whenever the policy's
schedule isn't 24x7); `TaskSlaDefinitionDetail.Duration` maps to
`sla_policy.duration` (the SLA policy's own target duration, e.g. `"4
Hours"`, `"15 Minutes"`) -- simply never selected before, not previously
believed unavailable.

**Still left `nil`, now confirmed rather than assumed**:
`ScheduleSource`/`Flow`/`Workflow`/`IsEnableLogging`/`DurationType` on the
definition detail have no backing column anywhere on `sla_policy` (checked
directly against the live schema's full column list, not just the
migration file); `ResetCondition` still has no matching column --
`reset_action` (already mapped to `ResetAction`) is a different concept
from a "reset condition" this field's name implies. `sla_policy` does have
its own `resume_condition` column (distinct from `pause_condition`, which
is mapped to `PauseCondition`), but `TaskSlaDefinitionDetail` has no field
for it at all -- a real, minor gap, left unselected rather than adding a
new response field speculatively.

## GET /metadata and GET /projects/{id}/metadata now have Postgres support

Both were entirely ServiceNow-only (their handlers were only wired when
`DATA_SOURCE=servicenow`), which broke `apps/customer-portal/backend-v2`'s
`/filters` and `/features` endpoints in Postgres mode -- its own CLAUDE.md
says both are built purely from `GetProjectMetadata`, with no fallback.

**`GetProjectMetadata` lives on `ProjectStatsService`, a 7-method interface
bundled with all the `/projects/{id}/stats/*` endpoints -- and only this one
method got a Postgres implementation.** Rather than stub the other 6
(`GetProjectStats`, `GetProjectCaseStats`, `GetProjectConversationStats`,
`GetProjectDeploymentStats`, `GetProjectTimeCardStats`,
`GetProjectChangeRequestStats`) with fake "not implemented" errors -- which
would have turned their routes from a clean 404 in Postgres mode into a
misleading 4xx, breaking the `TestPostgresOnlyRoutesAreAbsentWithoutAPool`-style
convention this codebase already relies on for signaling "this route doesn't
exist on this data source" -- `GetProjectMetadata` was split out into its own
narrower interface, `service.ProjectMetadataService`, and its own handler,
`ProjectMetadataHandler` (`internal/handler/project_metadata_handler.go`).
`GET /projects/{id}/metadata` is now registered unconditionally in
`routes.go`, backed by `projectMetadataService` (Postgres) or reusing the
already-constructed `snProjectStatsSvc` value (ServiceNow) -- Go interfaces
being structural, `snProjectStatsService` satisfies `ProjectMetadataService`
without any change. `ProjectStatsHandler`/`ProjectStatsService` are
unchanged and still ServiceNow-only for the remaining 6 stats methods. If a
future entity-service method needs the same treatment (real Postgres support
for one method of an otherwise-ServiceNow-only bundled interface), follow
this same split-interface-and-handler pattern rather than stubbing the rest.

**`ReferenceDataRepository`** (`internal/repository/reference_data_repo.go`)
backs both endpoints:
- `ListProjectTypes`/`GetProjectByID` read the `project_type` table
  (migration 0031) and `project.project_type_id` (migration 0032) --
  confirmed live: 1952 of 1956 `project` rows have a `project_type_id` set.
- `EnumLabels` queries `pg_catalog.pg_enum`/`pg_type` directly (`WHERE
  t.typname = ANY($1::text[])`) rather than hardcoding each enum's label
  list, so `ProjectMetadataResponse`'s choice lists (case states/severities/
  issue types, deployment types, engagement types/payment types,
  change-request states/impacts, time-card states, conversation states)
  always match whatever the migrations currently define. Each label becomes
  a `ChoiceListItem{ID: label, Label: label}` -- Postgres enums have no
  separate numeric-id/display-label pair the way ServiceNow's `sys_choice`
  records do, so the raw enum label is used as both.
- `ProjectMetadataResponse.CaseTypes` is NOT queried -- it's
  `case_service.go`'s own `validCaseType` vocabulary (`case`, `engagement`,
  `security_report_analysis`, `service_request`, `announcement`), listed
  directly as `caseTypeRefItems` in `project_metadata_service.go` since it's
  a fixed filter vocabulary, not a database table.

**Left empty with a TODO comment, not fabricated** (per this codebase's
existing convention of flagging genuine data-source gaps rather than
inventing data): `SystemMetadataResponse.FeedbackEmojis` (static
ServiceNow-side config, not project/case data); `SeverityBasedAllocationTime`
(no SLA-allocation-time table exists);
`ProjectFeatures.AcceptedSeverityValues` and every `Has*Access`/product-
category field (no per-project feature-entitlement or severity-restriction
columns exist anywhere in the Postgres schema -- checked directly against
the `project` table's full column list, not just assumed). (`CallRequestStates`
used to be on this list; `customer_call` -- migration 0073 -- has since
landed, so it's now read live from `customer_call_state_enum` like every other
choice list. See "Call requests and the service-request catalog" below.
`TimeZones` used to be on this list too; see below.)

**`SystemMetadataResponse.TimeZones` is now read from a real `timezone`
table** (`value`, `label`, `utc_offset`, `dst`; 39 rows at the time this was
wired up) via `ReferenceDataRepository.ListTimeZones`, mapped `value -> id`/
`label -> label` into the same `{id, label}` `domain.ChoiceListItem` shape
the ServiceNow-backed response already used -- no wire-contract change.
**The table is the sync's**: `0121_timezone_table.sql` (mirrored from
csm-sync-service) creates it, with the four columns above and no rows -- the rows
are ServiceNow's, so a local database gets stand-ins from
`scripts/csm-compose/fixtures/0121_timezone_table.sql`. Its column names/types
were first confirmed by querying the live staging database directly
(`information_schema.columns`), before that migration was mirrored. Deliberately
not reconciled against the ServiceNow choice list's own 54-entry version
(confirmed, by hand, against a live HAR capture of the ServiceNow-backed
`GET /metadata` response) -- the
two lists disagree in both size and some labels (e.g. ServiceNow's separate
`Asia/Shanghai`="China" and `Asia/Singapore`="Singapore / Malaysia /
Philippines" entries are one consolidated `Asia/Singapore` row here), which
is this table's own deliberate, independent curation, not a migration gap to
fix. `utc_offset`/`dst` exist on the table but have no slot in
`ChoiceListItem` -- left unread rather than widening that contract for data
nothing consumes yet.

**`GlobalService.GlobalSearch` (`POST /search`) still has no Postgres
implementation** -- cross-entity project+case search is a materially larger
feature (its own query/ranking design across two tables) than the
reference-data reads `GetSystemMetadata` serves, so it returns a
`ValidationError` explaining the gap in Postgres mode rather than 404 (the
route itself is now registered in both modes, since `GetSystemMetadata`
needed to be) or a silently-empty result.

## Token validation and caller-scoped access

entity-service used to read `x-user-id-token` without verifying it and had no
notion of "what may this caller see" -- in Phase 1 ServiceNow applied that via
the forwarded token, so moving to Postgres removed the only enforcement. This
adds it back, in entity-service (next to the data), not in each caller.

**Token validation (`internal/auth`)** mirrors `apps/csm-portal/backend`'s
validator (`golang-jwt/jwt/v5` + `keyfunc/v3`, same versions), against
**Asgardeo** (not Choreo). Two tokens can arrive on the same request:
- `x-user-id-token`: the end user's ID token. Checked for signature, issuer,
  expiry, an `aud` among `AUTH_USER_TOKEN_AUDIENCES`, and an `email` claim.
- `x-jwt-assertion`: the calling application's client-credentials assertion
  (every backend, including csm-integration-service, sends one -- it's the
  only token a pure machine-to-machine caller ever sends). **Decoded only,
  never signature/issuer/expiry-verified** (`Validator.ExtractClientID`) --
  its `client_id` (else `azp`) claim is trusted at face value as the client
  id. This mirrors `apps/csm-portal/backend`'s own `x-jwt-assertion` handling,
  which runs with signature verification off in every Choreo deployment, not
  just locally: this token is minted by the gateway in front of the service
  after it already authenticated the caller by its own means, over a path
  this service already trusts. Re-verifying it against Asgardeo's JWKS was
  tried first and caused a real outage -- a JWKS refresh rate-limit/lookup
  failure rejected every internal caller -- and added no real security either,
  since the client id is only ever checked against the deployment-controlled
  deployment-controlled `M2MClientIDs`/`CSMPortalBackendClientID`/
  `CustomerPortalBackendClientID` configs, never used as a capability grant
  derived from an unproven claim. No audience check either way.

**Always on -- there is no config flag to disable it.** `AUTH_ISSUER`/
`AUTH_JWKS_URL`/`AUTH_USER_TOKEN_AUDIENCES` are required (`config.Validate`
rejects startup without them) and govern `x-user-id-token` validation; they
play no part in reading `x-jwt-assertion`, which is never checked against
them. Only asymmetric algorithms are accepted for `x-user-id-token` (an
HS256 token "signed" with the public key is rejected -- there is a test). A
`x-user-id-token` that is **present but invalid is always a 401 on every
route**, never downgraded to "no token": that would turn a forged user token
into an anonymous request. `x-jwt-assertion` is rejected only when it can't
even be decoded, or carries neither a `client_id` nor an `azp` claim. A
request with no tokens at all passes through the middleware; whether that's
acceptable is decided per endpoint (see below).

Two things learned the hard way, both mirrored from/corrected against the CSM
backend: Asgardeo publishes JWKS `x5c` certs Go 1.23+ refuses to parse, so the
JWKS transport strips `x5c` (`x5c_transport.go`); and `keyfunc` does **not**
fail when the JWKS URL is unreachable at construction -- it logs and retries in
the background, which would leave a misconfigured deployment up rejecting every
token. `NewValidator` therefore fails unless at least one key actually loaded,
and `NewRouter` panics on that error at startup.

**Who may see what (`AccessService.ResolveScope`)**. The decision comes from
the *validated* identity, never from a list the caller sends, and applies the
same way everywhere it's wired (see "Where this is actually enforced" below):

| Request carries | Result |
|---|---|
| no verified identity (only possible if the auth middleware was left out of the chain -- a bug) | 503 -- never scope from an unverified token |
| `x-jwt-assertion` client id is `CustomerPortalBackendClientID` | **always** resolved from `x-user-id-token` (row below) -- checked first, never unconditionally trusted, no matter what else this id is also (mis)configured into |
| `x-jwt-assertion` client id is `CSMPortalBackendClientID` AND `x-user-id-token`'s email ends in `CSMPortalUserDomain` | **everything, unconditionally** |
| `x-jwt-assertion` client id is `CSMPortalBackendClientID` but the email does NOT match the domain (or there's no user token at all) | 403 -- refused outright, not resolved some other way |
| `x-jwt-assertion` client id is in `M2MClientIDs` | **everything, unconditionally** -- regardless of any `x-user-id-token` the same request also carries |
| none of the above, user token, `user_type` INTERNAL (all active rows for the email) | everything |
| none of the above, user token, EXTERNAL (customer) | only projects where their email is a `REGISTERED` `project_contact`, and the cases in them; none registered = an empty result, never "no filter" |
| none of the above, user token, inactive / SYSTEM / NOT_AVAILABLE / unknown email | 403 |
| none of the above, no user token | 401 -- no legitimate caller to resolve |

**`M2MClientIDs`/`CSMPortalBackendClientID` win outright once matched -- there
is no comparison with the user token's own scope.** A user token an M2M
caller forwards (if any) is used only for attribution elsewhere
(`created_by`/`updated_by`), never for scoping -- not even to widen or
narrow anything; `CSMPortalBackendClientID`'s path does carry the forwarded
email into `AccessScope.ViewerEmail` for the same attribution purpose, since
that path always has one (the domain check requires it). This is simpler
than an earlier revision of this design (a "rescue" that only kicked in for
an *unknown* forwarded email, deferring to the user's own scope otherwise):
once real deployments settled on which callers are genuinely internal, there
was no longer a case where an internal client legitimately forwards a real
customer's token, so the extra nuance was removed. `AccessRepository` is
never even queried on the `M2MClientIDs`/`CSMPortalBackendClientID` paths
(there is a test asserting zero DB calls).

`user.email` is **not unique** (staging shares emails across rows), so on the
non-internal-client path rows are combined conservatively: internal access
needs every active row to be INTERNAL; an email that is also an EXTERNAL
customer is scoped as a customer. `user.is_active` NULL counts as active.
`project_contact` states other than `REGISTERED` (INVITED, RE-INVITED,
DEACTIVATED) grant nothing -- **staging data caveat**: at the time of writing
only 97 REGISTERED contact rows covered 68 of ~1956 projects (260 INVITED), so
customer results are limited by how much has been synced; flip the state in
`access_repo.go` if INVITED contacts should count.

**A related, separate gap surfaced while building this, not yet fixed**:
`recompute_user_type()`'s trigger (migration 0011) classifies only the
`admin` and `internal` Asgardeo/SN roles as `user_type = INTERNAL` -- a person
whose only role is `agent` ends up `NOT_AVAILABLE` and is denied here even
though they *do* have a `user` row. Whether `agent` should count as internal
is a product decision, not something to guess at here.

**Three separate configs classify a client-credentials caller**, deliberately
not one shared allow-list -- see `AccessClientConfig`'s own doc comment:

- **`M2MClientIDs`** (`M2M_CLIENT_IDS`, config.go's `ParseInternalClientIDs`)
  is a plain comma-separated set of client ids for pure machine-to-machine
  callers -- no human in the loop at all (the GitHub webhook
  delivery/service-request handlers, the Salesforce partner ingest, and
  similar). No `clientId=role` grammar, no "delegate" role: those existed in
  an earlier revision, when a caller that always forwards a user token
  needed a role distinct from one that sometimes doesn't -- superseded by
  the two singular configs below once it became clear those callers
  (apps/csm-portal/backend, apps/customer-portal/backend-v2) needed
  fundamentally different treatment, not just a different list entry. Which
  real client ids belong in `M2MClientIDs` is a deployment decision this
  file doesn't prescribe.
- **`CSMPortalBackendClientID` + `CSMPortalUserDomain`** (`CSM_PORTAL_BACKEND_CLIENT_ID` /
  `CSM_PORTAL_USER_DOMAIN`, singular): `apps/csm-portal/backend`'s client id,
  unrestricted only with a matching-domain forwarded user email -- see the
  decision table above. Must be set together or not at all
  (`config.Validate`).
- **`CustomerPortalBackendClientID`** (`CUSTOMER_PORTAL_BACKEND_CLIENT_ID`,
  singular): `apps/customer-portal/backend-v2`'s client id, checked FIRST and
  always resolved from the forwarded user token -- this is the structural fix
  for the deployment mistake the three-config split exists to prevent: a
  customer-facing BFF's client id ending up with unconditional,
  RLS-bypassing access to every project and case for every customer. Because
  `CustomerPortalBackendClientID` is checked before `M2MClientIDs` or
  `CSMPortalBackendClientID`, even pasting this same id into `M2MClientIDs`
  by mistake has no effect -- there is no shared list it could land in that
  grants it anything. `config.Load` logs a `slog.Warn` if it finds this id
  (or `CSMPortalBackendClientID`) also present in `M2MClientIDs` anyway, as a
  hygiene signal, and `config.Validate` rejects `CSMPortalBackendClientID ==
  CustomerPortalBackendClientID` outright at startup (an unambiguous
  copy-paste mistake no ordering can resolve).

### Where this is actually enforced

`AccessService` is wired into, and enforced by:
- `POST /search` (global search) -- projects match name/key, cases match
  number/subject/WSO2 id/description (case-insensitive, LIKE metacharacters
  escaped so `%` and `_` are literal); results are limited to `project`/
  case-like work items; `sortBy` accepts `name`/`createdOn`/`updatedOn` only
  (mapped to fixed columns, never interpolated). Case `state`/`severity` use
  the raw enum labels as id and label (same vocabulary as project metadata).
  `activeChatsCount`/`actionRequiredCount`/`outstandingCount` are 0 -- their
  definition lives in ServiceNow-side logic with no Postgres equivalent yet
  (TODO).
- `GET /projects/{id}` / `GET /cases/{id}` -- a project or case outside scope
  is a 404, indistinguishable from one that doesn't exist at all (never a 403
  that would reveal it exists). The scope filter is folded straight into the
  `WHERE` clause (`ProjectRepository.GetProjectByID`/`CaseRepository.
  GetCaseByID`'s new `scope SearchScope` parameter) rather than fetched-then-
  checked, so this is one query either way.
- `POST /projects/search` / `POST /cases/search` -- the scope's project list
  is ANDed into the query **independently** of whatever project filter the
  request itself carries (`SearchCasesRequest.Filters.Filters`'
  `projectId`/`in`, if present): a scoped caller explicitly asking for a
  project outside their own scope gets zero rows, never someone else's data,
  and gets the same narrowing even with no project filter of their own.

**This applies to the Postgres data source only.** In ServiceNow mode
`GetProjectByID`/`GetCaseByID` (and the search endpoints) go to ServiceNow
itself with the forwarded `x-user-id-token`, so what a caller may see there is
ServiceNow's own decision, and `AccessService` is not consulted. Note that
`snProjectService`/`snCaseService` *hold* a `pgFallback` (their constructor
doc comments say by-id reads use it), but their `GetProjectByID`/`GetCaseByID`
bodies don't call it -- do not assume this scoping reaches ServiceNow mode
because of that field. Tokens are still validated on every request in both
modes; only the per-caller project/case scoping is Postgres-only. Bringing
ServiceNow mode under the same scoping would mean routing those two reads
through the Postgres services, which changes where their data comes from --
a separate decision, not made here.

**Deploy prerequisite: machine-to-machine callers.** Any service that calls a
scoped endpoint directly with only a client-credentials token (no
`x-user-id-token`) gets a 401 unless its client id is in `M2MClientIDs`.
Before rolling this out, list every direct
service-to-service caller of `GET /projects/{id}`, `GET /cases/{id}`,
`POST /projects/search`, `POST /cases/search` and `POST /search` and add the
ones that should have unconditional access. A caller that reaches entity-service
*through* another service is identified by that other service's client id, not
its own.

The three product-consumption routes (`GET`/`PATCH
/projects/{id}/consumption`, `POST
/projects/{id}/deployments/{deploymentId}/license`) are scoped too, but check
membership in `service.authorizeProject` rather than pushing the scope into a
query. Two of the three have no query to push it into: a write and an upstream
call that leaves the service entirely. Refused as a 404 for the same reason as
the by-id reads. **These are scoped on both data sources**, unlike the five
operations above -- they are registered in ServiceNow mode deliberately (see
"Product-consumption provisioning state" in `README.md`), so ServiceNow is not
there to scope them.

**Not yet wired**: every other project/case-adjacent read (comments,
time cards, attachments, conversations, change requests,
call requests, catalogs, instances, etc.) still does no per-caller scoping --
the auth middleware validates tokens on every route, but only the operations
above actually call `AccessService`. Extending it further is follow-up work,
not done in this pass.

**Exception, added later**: `POST /escalations` / `POST /cases/{id}/escalations`
(`EscalationService.CreateEscalation`, Postgres data source) DOES call
`AccessService.ResolveScope` and authorizes `caseId` through
`CaseRepository.GetCaseByID` before mutating anything -- an out-of-scope
case is a `NotFoundError`, same convention as the by-id reads above. This was
wired in specifically because CreateEscalation MUTATES a case (escalate/
de-escalate) and returns its details, unlike the read endpoints still listed
above as not-yet-wired. Every OTHER case mutation (`UpdateCase`, `AddCaseTag`,
`AcknowledgeCase`, `CreateCaseComment`, ...) remains unscoped -- this is a
narrow, deliberately inconsistent fix for one endpoint under active review,
not a decision that case mutations are scoped now.

## Call requests and the service-request catalog (migrations 000067-000072)

Six tables landed together (`customer_call`, `sr_category`, `catalog_item`,
`catalog_item_category`, `catalog_variable`, `sr_category_routing_rule`) and
each backs a previously ServiceNow-only feature. Both feature groups' routes
are now registered for **both** data sources (`callRequestRepo`/`catalogRepo`
in `routes.go`, same wiring shape as every other dual-source entity).

**Verification status**: the six tables exist in staging with exactly the
migrations' columns/types/enum labels, but are all **empty (0 rows)** -- so
the assumptions marked ASSUMPTION below could not be checked against real
synced rows. What *was* verified against staging: every read path executes
without error on real data (real cases, projects, deployed products), both
write statements `PREPARE` cleanly against the real schema (no write was made),
and the catalog matching logic was run with the real repository code on real
deployed products using session-local `TEMP` tables shadowing the empty ones.
The rest (create/update semantics, edge cases) was proven on a throwaway local
Postgres built from all 72 migrations.

**A data finding that changed the design**: `product.unit` is NULL for
**every** product in staging (17/17) and 214 deployed products have no
`product_id`, so a strict `rule.product_unit = product.unit` could never match
any rule that names a unit -- the catalog would always be empty. Unit matching
is therefore fail-open: it is only compared when both sides are known. Classification
(`deployed_product.product_category`, populated for ~88% of rows) stays strict:
a deployed product with no category only matches rules with no classification
requirement. TODO: make unit strict again once `product.unit` is populated.

### Call requests (`customer_call`) -- `call_request_repo.go`/`call_request_service.go`

**Per-caller scoping is enforced by Postgres row-level security, not by Go.**
Migration 0143 puts `FORCE ROW LEVEL SECURITY` on `customer_call`: a row is
visible to an internal caller, or to a caller who is a registered member of its
parent case's project (`is_project_member(work_item.project_id)`), so a call with
no parent case is visible to internal callers only. Both searches therefore apply
no project filter of their own, and any filter below is ANDed on top of what RLS
allows -- a filter can narrow a caller's view, never widen it. Call requests are
still not one of the operations `AccessService` is wired into (see that
section's "Not yet wired" list), and the `x-user-id-token` is read only to
attribute writes (`created_by`/`updated_by`, `opened_by_id`).

All four `CallRequestService` methods are implemented. `state` maps to
`customer_call_state_enum` by upper/lower-casing (all eight labels match
`domain.CallRequestStateType` exactly -- `CANCELED` both sides, no spelling
drift; `TestCallRequestStatesMatchMigration` diffs them against the real
migration file). Timestamps are RFC3339 UTC like the rest of the Postgres code.

- `case` ref <- `work_item` (LEFT JOIN: `customer_call.work_item_id` is
  nullable); `assignee` <- the `"user"` display name (falling back to email);
  `notes` <- `all_notes`; `meetingLink` <- `call_link`;
  `scheduleTime` <- `scheduled_on`; `durationMin` <- `duration` (INTERVAL).
- `preferredTimes` <- `final_times` (JSONB). Checked against 393 synced staging
  rows: the shape is an array of **objects**, `{"time": "...", "index": 0}`
  (sometimes with extra `state`/`datetime`/`user` keys), not strings, and the
  times come in two spellings (`MM/DD/YYYY HH:MM:SS` 324, `YYYY-MM-DD
  HH:MM:SS` 168), both UTC (`scheduled_on` equals the first time as a UTC
  instant). `decodeFinalTimes` reads objects ordered by `index` and, for rows
  this service wrote, plain strings; it returns RFC3339 UTC and **skips** any
  element that is not a time -- six synced "times" are actually ServiceNow
  script error text (`Error: Missing parameters (localTime or timezone).`).
  Note this service *writes* a plain string array, so one column holds two
  shapes; the reader handles both, but whether the sync reads written rows back
  is unknown.
- **ASSUMPTION**: `actualDurationMin` <- `actual_call_duration` (free VARCHAR),
  parsed as a whole number of minutes (what this service writes); any other
  format reads as `nil`.
- **ASSUMPTION**: create -> state `pending_on_wso2` (the customer raised it,
  WSO2 must schedule). `PATCH`'s `assignee` is interpreted as an **email**
  (resolved via `GetUserByEmail`).
- `callRequestStates` in project metadata uses the lowercase domain ids
  (`pending_on_wso2`) with display labels -- the vocabulary these endpoints
  accept -- unlike the other metadata choice lists, which still use the raw
  UPPER_SNAKE enum labels (see the metadata section above; not yet aligned
  with each list's own API vocabulary).
- Search-all's `assignedUserIds` and `assignmentTeamIds` both describe the
  **parent case**, as the csm-portal contract says, and the dashboards'
  "My Call Requests" / "Calls To Attend" widgets depend on that:
  `assignedUserIds` matches `work_item.assigned_to_id` (OR'd with the call's
  own `customer_call.assigned_to_id`, which is only set once an engineer
  schedules the call -- matching only that column left every
  `pending_on_wso2` call out of "My Call Requests", digiops-cs#3314);
  `assignmentTeamIds` matches the case's account CRE team
  (`account.cre_team_id`, the same path the case search's `creTeam` filter and
  `BeTeam.creGroupId` use). It used to be rejected with a 400 on the belief
  that nothing held a case's team, which made "Calls To Attend" fail to load
  on every dashboard. `work_item.assignment_group_id` is deliberately not
  used: it is unpopulated on synced cases (`assignedTeam` reads back null). A
  call with no parent case, or whose account has no CRE team, never matches a
  team filter.
  `call_request_search_all_integration_test.go` (real Postgres, skipped without
  `CALL_REQUEST_TEST_DSN`; needs a non-superuser, non-BYPASSRLS login, which it
  checks) pins both filters for an internal caller and for a customer
  registered on one project.
  `caseStates`/`excludeCaseStates` reuse `caseLikeStateColumn`/`caseLikeJoins`
  so they work for every case-like type.
- **Not done, deliberately (same "don't guess" rule as `CreateCase`)**:
  `number` is left NULL on create (no default, no sequence, no confirmed
  format -- returned as `""`); `cancellationReason` is **rejected with a 400**
  rather than accepted-and-dropped (no column: `reason` is the request's own
  reason -- note the customer portal passes it through, so cancelling *with* a
  reason fails on this data source until a column exists); `closed_on`/`closed_by_id`
  are never set (which states count as "closed" is unspecified); state
  transitions aren't validated against the current state.

### Service-request catalog -- `catalog_repo.go`/`catalog_service.go`

- **ASSUMPTION**: a "catalog" is an `sr_category` row (it's the only
  catalog-level entity with a UUID and a name; ServiceNow's `sc_catalog` level
  was collapsed into the plain `sr_category.catalog` enum), its items linked
  through `catalog_item_category`.
- Availability for a deployed product: a `sr_category_routing_rule` matches when
  `rule.product_unit = product.unit` and `rule.classification =
  deployed_product.product_category` (same label sets but distinct enum types,
  hence `::TEXT` casts). **ASSUMPTION**: a NULL on the rule side is a wildcard
  ("any"). A NULL unit on the deployed-product side does not exclude a rule
  (see the data finding above); a NULL classification only matches rules that
  don't specify one. Only active categories with at least one available item are returned;
  an unknown deployed product is a 404.
- `GetCatalogItemVariables` 404s unless the item is linked to that catalog.
  `catalog_variable`'s `read_only`/`hidden`/`reference_table`/`max_length`/
  `validation_name`/`validation_regex`/`validation_message` columns and the
  sibling `catalog_variable_choice` table (migration 0125) back
  `readOnly`/`hidden`/`maxLength`/`referenceTable`/`validation`/`choices` on
  this data source now -- a NULL `read_only`/`hidden` reads as `false`, and
  `Choices` is only set when the variable has at least one `is_inactive IS NOT
  TRUE` choice row (an inactive choice is excluded entirely, not flagged). This
  data is kept current by a separate sync service, not written here. A NULL
  `is_active` counts as active. Under `postgres-servicenow-dual-write`, this
  one endpoint reads Postgres directly (unlike `SearchCatalogs`, which still
  falls back to ServiceNow -- see `catalogService.snMirror`'s own doc comment).

## Case search filters on the Postgres data source

`caseRepo.SearchCases` implements `tag`, `projectOnboardingStatus` (in/notIn),
`taskSLABusinessElapsedPercent` (gte/lte), `escalationLevel`, `escalation`
(isEmpty/isNotEmpty), `parentId` (eq), `product`, `creTeam`/`sreTeam` (in), and
`anyOf`. The rest of the ServiceNow-shaped filters are still rejected with a 400
by `caseService.SearchCases` (`projectType`, `slaBreached`,
`accountEscalationActive`, ...) because dropping one would silently widen the
result set.

- **`product` (in)** was rejected outright even though `SearchCases`'s own
  joins already carry `prod` (the deployed product's catalog row, used for
  every result's `ProductName`) -- found alongside `creTeam`/`sreTeam` below
  by proactively auditing `caseService.SearchCases`'s remaining rejections
  for real backing columns rather than waiting for another live report.
  Matches on `prod.name = ANY(...)`, an exact match against the same value
  already selected into each row.
- **`creTeam`/`sreTeam` (in)** were rejected the same way, but `SearchCases`
  had no `account`/`"group"` join to filter on at all -- only `GetCaseByID`
  had it (`account a` -> `"group" cre`/`"group" sre` via
  `a.cre_team_id`/`a.sre_team_id`). Added the identical joins to
  `SearchCases` and matched on `cre.id`/`sre.id = ANY(...)`. **Still blocked
  on data, not schema, the same caveat as before this fix**: staging's
  `group` table was empty as of the investigation that first found this (the
  sync has no job for the full group source), so every group FK is NULL --
  confirm `group` is actually populated in the target environment before
  expecting this filter to return anything.

- **`parentId eq`** was accepted by `ParseCaseFieldFilters` (the customer/CSM
  portals' "Linked Items" tab sends it to find a case's child cases) but
  unconditionally rejected by `caseService.SearchCases` with "not supported
  by this data source", even though nothing about it is actually
  ServiceNow-specific: `work_item.parent_id` (migration 0039) is the exact
  same generic self-reference `GetCaseByID`'s own `ParentCase` already reads
  in the other direction. Fixed with a plain `wi.parent_id = $N::uuid`
  predicate in `SearchCases`'s `WHERE` clause — not routed through the
  shared `caseFieldPredicates` (top-level + `anyOf` branches), since
  `rejectUnsupportedOrGroupFields` already refuses `parentId` inside an
  `anyOf` branch unconditionally (a pre-existing, unrelated rule, left as
  is), so it only ever needs to apply at the top level.

- **One builder for top-level fields and `anyOf` branches.** `caseFieldPredicates`
  (`case_field_predicates.go`) turns a `caseFieldSet` into SQL for type, project,
  deployment, assignee, state, severity, issue type, engagement type, work state,
  escalation level and tags. The top-level search and each `anyOf` branch both use
  it, so a fix in one cannot miss the other (the state bug below is what happens
  otherwise). Only the ServiceNow adapter used to parse `anyOf` into
  `Parsed.OrGroups`; `caseService.SearchCases` now does too and runs the same value
  validation (`validateCaseFieldValues`) on each branch. A branch is the AND of its
  fields, branches are OR'd, and the whole is ANDed with the top-level filters.
- **escalationLevel** matches `"case".current_escalation_level` ("0".."5" ->
  `EL0`..`EL5`; anything else is a 400), the value the case detail already shows.
  **escalation** matches `"case".is_escalated`. The `case_escalation` table is an
  event *history* (several rows per case) and often disagrees with the case's
  current level, so it is deliberately not used.
- **projectId notIn** filtered on `c.project_id`, but `"case"` has no such column
  (it is `work_item.project_id`), so every such search failed with "column
  c.project_id does not exist". Fixed, and a case with no project now satisfies
  notIn.
- **tag**: `EXISTS`/`NOT EXISTS` over `work_item_tag` joined to `tag`, names
  compared case-insensitively (as `AddCaseTag` looks tags up). `in` = carries any
  of the names; `notIn` = carries none (an untagged case satisfies it).

- **onboarding status**: matched against `project.onboarding_status` through the
  existing LEFT JOIN. The wire vocabulary is ServiceNow's ("Not-Applicable",
  "OnHold"), the enum's is `NOT_APPLICABLE`/`ON_HOLD`, so `onboardingStatusEnumLabels`
  normalizes (case, `-`, `_`, space ignored) and rejects unknown values -- an
  unknown value in a `notIn` would otherwise widen the result. A NULL status
  (or no project) never matches `in` but does match `notIn`.
- **SLA percent**: one `EXISTS` over `sla.business_elapsed_percentage`, so both
  bounds apply to the *same* SLA row. Any SLA row counts regardless of `stage`
  (the `domain.TaskSLAFilter` contract). Checked against staging: restricting to
  in-progress SLAs changed a 1,652-case result to 1,634, so the choice barely
  matters on real data.
- **Data caveats (staging, when checked)**: one case has `current_escalation_level`
  EL4 but `is_escalated` false, so "escalated at level 4" returns 0 although a
  level-4 case exists. Also 6,833 of 8,055 `CASE` work items
  have a NULL `project_id` (mostly 2023-2024 cases; `deployment` doesn't carry
  the project either), so project-based filters only ever see the remaining
  ~15% -- a sync gap, not a query bug. `work_item_tag` now exists in staging but
  held only 107 links (30 on cases) against 2,624 tags, so tag results are sparse
  until the label sync catches up.
- **`work_item.type` disagrees with the extension row** for some staging rows:
  41 `CASE` work items carry an `announcement` row (all `OPEN`) and 1 carries a
  `service_request` row; 156 `CASE` and 39 `SERVICE_REQUEST` work items have no
  extension row at all, so they have no state and never match a state filter.
  Search selects by `work_item.type` and reads the state from whichever extension
  row exists, so those 41 count as `case` + `open`.

### The state filter is a targeted id lookup, not the five-table COALESCE, for `in`/`notIn`

`caseLikeStateColumn`'s five-way `COALESCE` across every case-like extension
table can never be served by an index -- it's a runtime expression, not a
column -- and real production-volume testing confirmed it costs the most
database time of any query this service runs, by far: every row
`caseSearchJoins` admits has to be joined to all five extension tables before
the `COALESCE` can even be evaluated, for every `state` filter on every case
search. `caseLikeStateLookupClause` (`case_field_predicates.go`) replaces the
`WHERE` predicate for both the `state` (`in`) and `ExcludeStates` (`notIn`)
filters with a `wi.id = ANY(ARRAY(SELECT id FROM <type-table> WHERE state ...
UNION ALL ...))` lookup (`<> ALL(ARRAY(...))` for `notIn`), scoped to exactly
the type(s) the request's own `type` filter already named -- every dashboard
widget's case search is, in practice, a `{type}` filter alongside a `{state}`
filter, so this lets the planner use that type's own existing state index
(e.g. `idx_case_state`) and skip the other four extension-table joins
entirely, instead of joining everything and filtering after. Falls back to
checking every case-like type (same coverage as the `COALESCE` it replaces)
whenever the request names no type of its own -- the `DefaultTypes` case, and
any `anyOf` branch that doesn't narrow `type` itself. The `SELECT` list's own
display column still reads `caseLikeStateColumn` as before; only the
`WHERE`-clause matching changed.

**Wrapped in `ARRAY(...)` rather than used as a bare `wi.id [NOT ]IN (...)`
subquery.** Confirmed against real production-volume data that the `ARRAY`
form evaluates the `UNION ALL` exactly once (an upfront, independent
computation) and then probes `work_item`'s own primary key per element,
while a bare `IN`/`NOT IN` subquery here is prone to being planned as a join
against the subquery's result set instead -- which, combined with this
schema's row-level-security predicates layered onto every table, measured
meaningfully slower than the `ARRAY` form for the identical result. ids are
always non-`NULL` (each branch selects a primary key), so `<> ALL` is an
exact negation of `= ANY` here, with no three-valued-logic subtlety to
account for.

**`projectOnboardingStatus`/its `notIn` sibling deliberately keep filtering
through the `LEFT JOIN` to `p`, despite the superficial similarity to the
state-filter rewrite above.** The same "match against the table directly via
an `ARRAY`-wrapped id lookup instead of filtering through a join" technique
was tried here too and measured, directly against production-volume data, to
bring no benefit and in some cases be slower. The state lookup wins because a
case search's state filter is typically highly selective (e.g. "open" is a
small fraction of all cases); `projectOnboardingStatus` filters in practice
tend to be the opposite -- a widget excluding only a couple of terminal
statuses matches nearly every project -- so building an array of almost
every project id and checking per-row membership against it costs more than
the indexed nested-loop join this already was. Revisit only with a
measurement showing otherwise for a specific, genuinely selective
onboarding-status filter shape.

**Known, accepted divergence**: a work_item row whose own `type` disagrees
with which extension table actually holds its data (see the bullet above --
confirmed live, and rare) is found by the old `COALESCE` regardless of its
declared type, since that approach blindly checks all five tables for every
row. This lookup trusts `wi.type` and only checks that type's own table, so
it diverges from the `COALESCE` both ways for such a row: an `in` filter
misses it whenever the request narrows `type` to something other than the
table the row's data actually lives in (the `COALESCE` would have matched
it there), and a `notIn` filter wrongly keeps it for the mirror-image reason
-- its declared type's own table has no row to find, so the lookup can never
see the state that should have excluded it, and the row passes the `notIn`
check when the old `COALESCE` would have excluded it. Deliberately not fixed
by always checking every table regardless of the request's own type filter
-- that would reproduce the exact cost this rewrite exists to avoid, to
compensate for a handful of rows a separate sync-side data-quality issue
produced, not something every case search should pay for indefinitely.

## Announcement requests

`announcement_requests` (migration `0042`, `internal/domain/entity.go`'s
`AnnouncementRequest`, `internal/repository/announcement_request_repo.go`,
`internal/service/announcement_request_service.go`) is durable state for an
announcement that hasn't been published yet — `draft -> pending_approval ->
approved -> published`. Like `event_publish_failures`/`scheduled_task_run`, it has no
ServiceNow equivalent and is always backed by Postgres regardless of
`DATA_SOURCE`. It exists purely to let the CSM portal remember an
announcement is mid-flight while the real approval decision happens over
email, entirely outside this service — there is no approver role, no email
sending, and no preview-rendering component here. The dry-run case the
caller's own mechanism already creates (a real case in a test project) *is*
the preview the approver reviews; this service just records that one
happened (`dryRunCaseId`/`dryRunAt`/`dryRunBy`) and refuses `Submit` until it
has (see `AnnouncementRequestService.Submit`'s own doc comment) — submitting
for approval without one would send an approval request for content nobody
has actually seen rendered.

`audienceDefinition` is an opaque JSON blob (whichever shape the caller's own
create form builds) that this service never interprets, so a draft can be
re-opened and re-edited without re-deriving anything. `resolvedProjectIds` is
a *different* field: the actual resolved project id list, frozen by the
caller (with whatever exclusions it applies — see `AnnouncementHandler`'s
`injectExcludeProjectKeys` in `csm-portal-backend`) at the moment of
`Submit`, never re-resolved later. That's a deliberate choice: the frozen
snapshot is what the dry-run link and the approval email actually describe,
so `Publish` must send to exactly that list, not to whatever "all customer
projects" happens to resolve to by the time someone gets around to
publishing.

**Editing behaves differently depending on the row's current state — this is
the one piece of real business logic here, not just CRUD.** `Update` (backing
`PATCH /announcement-requests/{id}`) branches on the row's current state,
fetched fresh immediately before deciding what to do:
- `draft` → a plain field update, no state change.
- `pending_approval` → the same field update, but reverts to `draft` as one
  atomic side effect, clearing the frozen `resolvedProjectIds` snapshot and
  the dry-run record. The content is out for real review over email at this
  point, so a silent change under the reviewer isn't safe, and the old dry
  run no longer describes whatever's about to be re-submitted.
- `approved` → subject/description/`isSecurityAnnouncement` may still be
  updated in place with **no** state change and **no** audience change (an
  audience-change attempt here is rejected outright) — a human has already
  said yes over email, so this is a deliberate, explicitly-accepted
  trade-off: a post-approval edit is not re-verified against a fresh dry run
  before `Publish` sends whatever's currently there.
- `published` → rejected outright; nothing about a published request is
  editable through this entity again.

**Every state-transition write is atomically conditioned on the state (and,
for `Submit`, the dry-run precondition) it requires, inside the `UPDATE`'s
own `WHERE` clause — not just checked beforehand in the service layer.** A
service-layer "fetch current state, validate, then write" sequence is not
atomic against two concurrent transitions on the same row (e.g. one caller's
`Submit` racing another's `RecordDryRun`-clearing `RevertToDraft`); without
the state repeated in the `WHERE` clause itself, the slower writer would
silently apply its own stale-precondition write after the faster one already
moved the row on. Every mutating repository method (`Update`, `RecordDryRun`,
`Submit`, `Approve`, `RevertToDraft`, `MarkPublished`) follows this shape;
`announcementRequestRepo.onConflictOrNotFound` is what a 0-row `UPDATE ...
RETURNING` turns into — a `ConflictError` if the row still exists (the
precondition changed underneath the caller, a real race) or the propagated
`NotFoundError` if it doesn't (checked via one extra `Get`, only on this rare
path, so the common case stays a single round trip).

**`POST /announcement-requests/search` filters by state two ways, and they are
mutually exclusive.** `state` (one value) is the original field; `states` (a
list) matches a request in *any* of the listed states and returns them as one
merged list — ordered newest-first and paginated as a whole, so `total`/`hasMore`
describe the merged result rather than one state. The repository query is
`state = ANY($4::text[])` against a nil-when-empty `text[]`, so an omitted or
empty `states` means "no state filter," never "match nothing." `Search` rejects
`state` + `states` together (judged by the field being *sent*, so an explicit
`"states": []` alongside `state` is rejected too), any state outside the four
lifecycle values, and `readyForScheduledPublish` combined with either (that flag is the
`csm-scheduled-tasks` cron's own "approved and due" query and ignores state
filters by design). `state` is deliberately kept rather than folded into
`states`: the registry's published-requests lookup and the cron both send it,
and the handler's `decodeRequest` rejects unknown JSON fields, so a client that
sends only `state` (as the CSM portal's Requests tab does for a one-state
selection) keeps working against a build that predates `states`.

This service never creates the real per-project cases itself — `MarkPublished`
only records that publishing happened, by whom, and when. The actual fan-out
(`POST /cases` per project) is, and remains, the caller's own job, unchanged
from before this entity existed; there is deliberately no persisted
per-project delivery ledger here either — that belongs to a future batch
entity, not this one.

## POST /users/search sortBy on the Postgres data source

`userService.SearchUsers` used to reject any `sortBy` on Postgres ("only supported
for the ServiceNow data source") even though the OpenAPI contract advertises
`name`/`createdOn`/`updatedOn` and the CSM users page always sends `name`/`asc`,
so that page's search 400'd. It now validates the field/order the same way the
ServiceNow adapter does (`validUserSortField`/`validUserSortOrder`) and
`userOrderBy` maps them to fixed SQL expressions (never request text). `name`
orders on `LOWER(COALESCE(NULLIF(name,''), first + last, user_name))` because
`"user".name` is empty for a few synced rows (5 of 2,937 in staging); `u.id` is
always the last tie-break so pages are stable. No `sortBy` keeps newest-first.

## POST /users/search active filter on the Postgres data source

`userService.SearchUsers` used to reject any `active` filter on Postgres
("only supported for the ServiceNow data source") even though `"user".
is_active` is a real, already-read column — found live via the case
detail page's Time Tracking tab, whose approver search sends
`{roleIds: ["timecard_approver"], active: true}` to only offer active
approvers, and 400'd outright. `userRepo.SearchUsers` now filters on it:
`active: true` matches `is_active IS NULL OR is_active = TRUE` (a NULL row
counts as active, the same convention `AccessService.ResolveScope` already
uses for this exact column), `active: false` matches `is_active = FALSE`
strictly — a row with no `is_active` recorded at all is not known to be
inactive, so it must not satisfy that filter.

## POST /users/search userIds/groupIds/groupNames filters on the Postgres data source

Same shape of gap as the `active` filter above, found by proactively auditing
`SearchUsersFilters` for other fields still rejected outright on Postgres
rather than waiting for another endpoint to hit one: `userIds`, `groupIds`
and `groupNames` were all bundled into one blanket rejection, even though
each has a real backing column/table already read elsewhere. `userRepo.
SearchUsers` now filters on them:
- `userIds` — `u.id = ANY($n::uuid[])`.
- `groupIds` — `EXISTS (SELECT 1 FROM team_member tm WHERE tm.user_id = u.id
  AND tm.team_id = ANY($n::uuid[]))` (migration 0033, the same table
  `GetUserGroups` reads).
- `groupNames` — the same `EXISTS` joined to `team` on `t.id = tm.team_id`,
  matching `t.name = ANY($n::text[])` instead of the id; kept alongside
  `groupIds` because callers' team registries are keyed by name (ids differ
  per environment and not every configured team has one).

`userService.SearchUsers` still validates `userIds`/`groupIds` as UUIDs
(`validateUUIDs`) before they reach the repository — only the "unsupported on
Postgres" rejection was removed, not the format check.

## POST /users/search roleIds silently matched nothing for a namespaced role name

Reported live: the Time Tracking tab's approver search
(`{roleIds: ["timecard_approver"], active: true}`) came back empty (not a 400
— the `active` fix above was already live) once the caller's list was scoped
to that one role. `userRepo.SearchUsers`'s `roleIds` predicate did an exact
`r.name = ANY(...)` match, but the synced `role.name` value carries a
namespace prefix for at least some roles (`sn_customerservice.
timecard_approver`, not the bare `timecard_approver` a caller sends — see the
CSM webapp's own `ResponsiveRoleChips.tsx`'s `ROLE_CATALOGUE_ALIASES`, which
exists purely to strip this same prefix back off for *display*; there was no
equivalent normalization for *searching*). Fixed by also matching on the
suffix after the last `.` (`regexp_replace(r.name, '^.*\.', '')`), so a filter
value matches whether the stored name is bare or namespaced — purely
additive: it can never match less than a plain `r.name = ANY(...)` did before.

## POST /users/search returns each user's roles (Postgres data source)

The Postgres `User` had no roles, so the CSM users page showed none even though
`user_role` holds them (2,803 of 2,937 staging users have at least one).
`userRepo.SearchUsers` now calls `attachRoles`, which reads the roles for the
whole page in **one** query (`user_role` joined to `role`), not one per user, and
`domain.User.Roles` is always non-nil (`[]` when none) in a search result. Other
lookups (`GetUserByEmail`) leave it nil. Two things worth knowing:
- `user_role` has **no unique constraint** on `(user_id, role_id)` and staging holds
  113 duplicated pairs (111 users), so the queries use `DISTINCT`; without it a
  user would show `["admin", "admin"]`. `GetUserRoles` (used by `GET /users/me`)
  got the same `DISTINCT`.
- The CSM webapp used to decide "ServiceNow user or not" by whether `roles` was
  present, so adding it here flips a postgres user into the ServiceNow branch and
  blanks the name unless the webapp is updated. Ship the webapp change first or
  together (`csmUsers.ts`'s `isSnUser` no longer looks at `roles`).
- `GET /users/{id}` (the profile page) is registered only for the ServiceNow data
  source; it is not available on Postgres at all.

## POST /users/by-ids failed the whole batch on one non-UUID id

Reported live: the staff portal's Knowledge page called `POST /users/by-ids` to
resolve author names and got a 500 ("Failed to look up users."), so no row showed
an author. The page builds the id list from each article's `authorId` **and**
`updatedBy`, and `knowledge_article.updated_by` (migration 0044) is a free-text
`VARCHAR(255)`, not a user reference: on one page of 20 articles, 6 values were sent,
4 shaped like UUIDs, 1 like an email address and 1 a short plain string. `"user".id`
is a UUID column, so `WHERE id = ANY($1)` makes Postgres reject the whole statement
(`invalid input syntax for type uuid`, SQLSTATE 22P02) on the first value that is not
a UUID, which also drops the lookup for the valid ids in the same batch.

Fixed in `userService.GetUsersByIDs`: values that are not well-formed UUIDs
(`validate.IsUUID`, case-insensitive) are skipped before the repository is called,
and a batch with none left answers `{"users": []}` without querying. Skipping
rather than rejecting is deliberate: a value that is not a UUID can never match a
row, so the answer is the one a query would have given for an unknown id, and a 400
(what `validateUUIDs` returns for a single path or body id) would fail the page just
as the 500 did. The wire contract is unchanged (`openapi.yaml` only gained a
description), the repository is untouched, and no webapp change is needed: the
lookup tolerates whatever a caller builds its list from, and every caller of
`useUsersByIds` (the KB All, List, Admin and Review Queue pages and the article
timeline) benefits at once.

Because ids are filtered here, a free-text `updated_by` never resolves to a name; a
caller that wants to show one has to fall back to the raw value itself.

`user_service_by_ids_test.go` pins what reaches the repository (valid ids only, in
order; no query when none are valid; errors still propagate).
`user_by_ids_integration_test.go` (skipped without `CASE_STATS_TEST_DSN`) runs the
service over the real repository on a real `"user"` table; against the unfiltered
code it fails with Postgres's own `invalid input syntax for type uuid`.

## GET /users/{id} on the Postgres data source

The route was registered only for ServiceNow, so opening a user in the CSM portal
on Postgres said "The requested resource was not found." It now returns
`domain.UserDetail`: the user (display name, `active`, type), `roles` (DISTINCT, see
above), `groups` (the teams from `team_member`, from which the BFF derives the
profile's team block) and, for customers only (`user_type` EXTERNAL, emitted as
`customer`), `projectAccess`.

- **It is a dedicated type, not `SNUserDetail`.** That type always sends `lockedOut`
  and per-project `notificationsEnabled`, neither of which this schema stores, and the
  page shows a "Locked out: No" chip whenever `lockedOut` is present, so reusing it
  would assert something unknowable. Those two fields are omitted. `timeZone` is a
  separate case — `"user".timezone` is a real column (see "GET/PATCH /users/me and
  the timezone column" below) — but `UserDetail` doesn't carry it today either, since
  nothing has asked for a user's timezone on this specific (by-id, not-self) profile
  read; only `GetMe`/`PatchMe` expose it so far.
- **`projectAccess`** is one row per `project_contact` invited under the user's email:
  `contactEmail` is the row's email, `contactRecordPresent` is `account_contact_id IS NOT
  NULL`, `contactRecordEmail` is the linked `account_contact.user_name` (it differs from
  the invited email on 20 of 357 staging rows), `registrationState` is the row's state, and
  `roles` come through `project_contact_group -> project_group_role -> project_role`
  (PORTAL_USER, SECURITY_CONTACT, LEAD_USER, BUSINESS_CONTACT).
- **`grantsCaseAccess` is exactly the rule `AccessService` enforces**: the contact is
  `REGISTERED` (`registeredContactState` in `access_repo.go`, shared by both). The
  ServiceNow version also required the linked contact's email to match; this data source
  does not, so reporting that here would describe a rule that is not applied.
- Enrichment failures are errors, not silently partial profiles (the ServiceNow adapter
  degrades to empty blocks; a database error here is a real fault).
- Like the other user routes this does no per-caller scoping; the BFF gates it.

## GET/PATCH /users/me and the timezone column

`"user".timezone` (`VARCHAR(64) REFERENCES timezone(value)`) is a real column,
added by the sync-mirrored `0122_user_add_timezone.sql` — it was first confirmed
directly against the live database, before that migration was mirrored. The
`timezone` reference table it points at is the sync's too (see "GET /metadata and
GET /projects/{id}/metadata" above).
`GetMe` was already wiring `domain.User.Timezone` through to its own response
(`GetUserMeResponse.TimeZone`) before this was fixed — it just always came
back `nil`, since `userColumns`/`prefixUserColumns`/`scanUser` never selected
the column at all. Both now do.

**`PATCH /users/me` didn't exist on this data source until now.**
`UserService` (the Postgres interface) had no `PatchMe` method whatsoever —
unlike `GetMe`, which has always had a real Postgres implementation
alongside the ServiceNow one, this route was registered only inside the
`snUserHandler != nil` branch in `routes.go`, so a Postgres deployment 404'd
on it outright. `UserService.PatchMe`/`UserRepository.UpdateUserTimeZone`
now exist, resolving the caller the exact same way `GetMe` does
(`x-user-id-token`'s email claim → `GetUserByEmail`, never a caller-supplied
id) and writing `"user".timezone` for that row alone — a user can only ever
update their own timezone through this endpoint, same as the ServiceNow
path's own scoping. This service does not check the value against the
`timezone` reference table, so any non-empty value passes the service; on a database
with the sync's `0122` shape (the column REFERENCES `timezone(value)`) a value that is
not in that table is refused by the foreign key (`user_timezone_fkey`, SQLSTATE 23503)
and surfaces as a 500. Only a blank value is rejected up front (`"timeZone is required"`, mirroring
`snUserService.PatchMe`'s own validation).

## POST /users creates a new "user" row (Postgres-only)

Before this, no code anywhere in this service wrote a `"user"` row at all — `UpsertFromSalesforce`
(`account_repo.go`) only ever touches `account`, and `project_membership_repo.go`'s own user upsert
only fires as a side effect of ingesting a Salesforce membership. `UserRepository.CreateUser`
(`user_repo.go`) is the first direct write path: `user_name` is always `lower(email)` (matching the
membership ingest's own convention), `is_active` is always `TRUE`, and `id`/`created_on`/
`updated_on` are supplied inline (`gen_random_uuid(), NOW(), NOW()`) since the column has no DB-side
default. `user_type` is never set directly — it's derived by a trigger (migration 0011) from
`is_system_user` (left unset here, so NULL/false) and role membership, the same as every other write
path in this codebase that touches `"user"`.

`roles` is optional; when supplied, `grantRoles` resolves every name against `role` (migration
000004) **before** inserting any `user_role` row — a partially-granted set on one unseeded name would
be a confusing half-success — and fails the whole request with a `ServiceUnavailableError` naming
the first bad one, the same posture `syncGlobalRoles` uses for the Salesforce membership ingest
(a role name is deployment config, not something this service validates against a fixed enum — see
`domain.UserRole`'s own doc comment). Everything happens in one transaction: a duplicate email (the
`user_name` `UNIQUE` constraint) or a missing role rolls back the insert too, never leaving an
orphaned `"user"` row with no roles.

The acting caller is resolved from `x-user-id-token` (`emailFromJWT`, the same helper `GetMe` uses)
and stamped as `created_by`/`updated_by` — this service still has no notion of "admin" itself;
restricting who may call this is `apps/csm-portal/backend`'s job (see that repo's own `CLAUDE.md`).

**`userService.CreateUser` also rejects granting an internal-resolving role to a non-`@wso2.com`
email.** `internalUserTypeRoles` (`user_service.go`, next to `emailRE`) is `["admin", "internal"]` —
the same two names `recompute_user_type`'s trigger maps to `user_type = INTERNAL` — and
`requestsInternalUserType` checks `req.Roles` against it case-insensitively before `repo.CreateUser`
runs; a match with `req.Email` not ending in `wso2EmailDomain` (`@wso2.com`, the same constant
`sn_case_service.go`'s `filterWso2Emails` already uses) is a `*apierror.ValidationError`. This is the
real enforcement boundary for that rule: `apps/csm-portal/backend`'s own `UsersHandler.CreateUser` runs
an identical, independently-maintained check for a fast 400 before ever reaching this service, but
this one is what actually protects the database — `POST /users` is this service's own route, and
nothing about role-name validation here should assume a single, trusted caller (see this file's own
note on `domain.UserRole` not being validated against a fixed enum for the same reasoning). Found live:
the CSM portal's Add User form sent no `roles` at all until it gained a type selector, so every user it
created resolved to `user_type = NOT_AVAILABLE` — checked directly against staging before this shipped
(128 such users). The type selector fixes that by granting `internal`/`external`; this check is what
stops it from being pointed at the wrong email.

**Creating an EXTERNAL-type user via `POST /users` is temporarily disabled.** `externalUserTypeRoles`
(next to `internalUserTypeRoles`) is `["external", "partner", "customer", "partner_admin",
"customer_admin"]` — every role name `recompute_user_type`'s trigger maps to `user_type = EXTERNAL` —
and `requestsExternalUserType` rejects any of them with a `*apierror.ValidationError` regardless of
email. This only affects `POST /users`: the Salesforce membership/contact ingest (its own, separate
`upsertMembershipUser`/`SalesforceContactRepository` write path, not `UserRepository.CreateUser`) is
unaffected and keeps creating external users exactly as before. `apps/csm-portal/backend`'s
`UsersHandler.CreateUser` mirrors the same check for a fast 400, and the webapp's Add User form
disables the "External" option in its type selector rather than offering a choice the backend will
reject — all three are temporary, meant to come out together once external-type creation is ready.

## SearchDeployments crashed on any page containing a NULL deployment.type

Reported live: `POST /deployments/search` failing with `cannot scan NULL into
*string`. `deployment.type` (migration 0018) has no `NOT NULL` constraint —
38 of 2859 rows are NULL on staging, checked live — but `DeploymentView.Type`
is a required (non-pointer) `DeploymentType` field on the wire, and
`deployment_repo.go`'s `SearchDeployments` scanned the column straight into
it. Fixed the same way as `CaseView.InternalID` (see that section above):
the wire contract stays a required string (every consumer already expects
that), only the scan side changes — `d.type::TEXT` now scans into a `*string`
local, and `stringOrEmpty(...)` (already used elsewhere in this file for the
identical class of fix) converts a NULL to `""` instead of crashing the whole
page. Verified directly against a real project with a NULL-type deployment on
staging: the search now returns all of that project's deployments, the
NULL-type ones as `"type": ""`.

**`/cases/{id}/tasks/search` (and every other `TaskService` method) is not a
bug — it's `unavailableTaskService`'s documented, deliberate 503** ("tasks
are only supported for the ServiceNow data source"). Checked directly
against staging: **no table matching `%task%` exists anywhere in the public
schema** — there is no Postgres-backed task storage at all to have a data bug
in. Implementing this would be a genuinely new feature (a migration + a real
`task_repo.go`), not a fix to something already wired up incorrectly — same
class of gap as `GlobalService.GlobalSearch`'s own "no Postgres
implementation" note elsewhere in this file.

## SearchProjects crashed on any page containing a NULL project.sf_id

Reported live: `POST /projects/search` failing with `cannot scan NULL into
*string`. `project.sf_id` is declared `NOT NULL` (migration 0014), but that
constraint turned out not to be actually enforced against real data — the
same class of gap `CaseView.InternalID`'s own doc comment describes for
`wso2_id`, and `SearchDeployments`'s for `deployment.type` (see that section
above) — and `project_repo.go`'s `SearchProjects` scanned the column straight
into `domain.Project.SfID` (a required, non-pointer field). Fixed the same
way as those: the wire contract stays a required string, only the scan side
changes — `p.sf_id` now scans into a `*string` local, defaulted to `""` when
NULL, rather than widening `Project.SfID` to `*string` and touching every
other reader of it. `GetProjectDetails`'s own `sf_id` scan (a separate query,
a separate endpoint) was not touched -- not reported broken, so left alone
rather than fixed speculatively.

## SearchKBArticles failed on any page containing a row with a NULL body/state/author_id

Reported live: `POST /kb-articles/search` returning 500 (`cannot scan NULL into
*string`), which the staff portal's Knowledge page surfaced as "Failed to search
KB articles". `knowledge_article` (migration 0044) allows NULL in `body`, `state`,
`knowledge_base_id`, `author_id` and `latest`, and on staging 2,386 of the 7,794
`latest = true` rows have a NULL `body`, 2,403 a NULL `author_id` and 1 a NULL
`state` (`knowledge_base_id` had none). `domain.KBArticle` declares all five as
required, non-pointer fields, and `scanKBArticle` scanned the columns straight
into them, so a single such row failed the whole page.

Fixed the same way as `CaseView.InternalID` and `DeploymentView.Type` (see those
sections above): the wire contract is unchanged (`KnowledgeBaseID`/`Body`/`State`/
`AuthorID` stay required strings, `Latest` a plain bool, `""`/`false` when the
column is NULL, so `openapi.yaml` and the portal clients are unaffected), and only
the scan side changes. `scanKBArticle` scans those five columns into pointer
locals and converts them with `stringOrEmpty(...)` (`latest != nil && *latest`
for the bool). It is the only place that scans `knowledge_article` columns, and
`CreateKBArticle`, `GetKBArticleByID`, `SearchKBArticles`, `UpdateKBArticleState`
and `UpdateKBArticleContent` all go through it, so every read path and every
`RETURNING` scan is covered by the one change.

`kb_article_repo_test.go` scans rows with NULL columns through a fake row that,
like pgx, rejects a NULL into a non-pointer destination, so it fails if the scan
reverts to plain destinations. `kb_article_repo_integration_test.go` runs search,
get-by-id and edit against a real `knowledge_article` table (skipped without
`CASE_STATS_TEST_DSN`).

An article whose `state` is NULL reads as `state: ""`. The service's
`isLegalKBArticleTransition` has no transition out of an unrecognised state, so
`PATCH` on such an article returns a 400 ("invalid state transition") rather than
a 500 — it is listed and viewable, but needs its state set before it can move
through review.

### `knowledge_article_history` has its own migration (0203) and creates the table only if it is missing

Two paths use the table — `ListKBArticleHistory`, and the history insert inside
`UpdateKBArticleState`'s transaction (so a state transition rolls back entirely
where the table is absent). `UpdateKBArticleContent` does not touch it, and search
never does. The table used to be defined only by the old-style pair
`000030_knowledge_article_history.up.sql` / `.down.sql`, which this change removes.
It was never part of the numbered series, so a database built from `migrations/`
got no table, and staging had it only because someone created it by hand (before
22:26 IST on 6 October 2026; the seven rows in it are test transitions).

**`0203_knowledge_article_history.sql` is `CREATE TABLE IF NOT EXISTS` plus
`CREATE INDEX IF NOT EXISTS`, and nothing else.** It never drops, truncates,
alters or rewrites, so it is safe on a database that already has the table (hand
made, or from an earlier run), safe to re-run, and safe on a database that is
refilled from ServiceNow: an existing table and its rows are left exactly as they
are. Keep it that way. If the table ever needs another column, add a new numbered
`ALTER TABLE ... ADD COLUMN IF NOT EXISTS` migration; do not edit 0203 into a
drop-and-recreate.

**Why the legacy pair had to go, not just be joined by a new file.** `make migrate`
globs `migrations/*.sql` and runs every file not yet recorded in
`csm_migration_applied_migration`, `.down.sql` files included, in name order. The
`000030` pair was never recorded on staging, and `.down.sql` sorts before `.up.sql`,
so the next `make migrate` against it would have run the `.down.sql` first
(`DROP TABLE knowledge_article_history`, rows gone) and then the `.up.sql`
(`CREATE TABLE IF NOT EXISTS`, an empty table again). That is the repo-wide trap
described in the top-level `CLAUDE.md` ("Only `NNNN_*.sql` belongs in that
folder"), and here it would have silently emptied the history.
`kb_article_repo_integration_test.go` runs 0203 repeatedly over a table that holds
rows and asserts they survive.

The other old-style KB pairs (`000020`–`000027`) are the same trap and are **not**
touched here; `000027_replace_kb_tables_with_real_schema.down.sql` drops
`knowledge_article` and `knowledge_base`. They need their own change.

## Case feedback silently 404'd on the Postgres data source instead of a documented 503

Reported live: a case's Activity timeline always showed "Could not load Case
Feedback" — on every case, every time. Unlike tasks (previous section) and
every other ServiceNow-only entity in this codebase, `routes.go` only
constructed `feedbackHandler` when `cfg.DataSource ==
config.DataSourceServiceNow`, leaving it `nil` (and, with the surrounding
`if feedbackHandler != nil` guard, both `POST /cases/feedback/search` and
`/aggregate` entirely **unregistered**) on Postgres — a silent 404, even
though `openapi.yaml` already documents a `503` `ErrorResponse` for both
paths. No feedback table exists anywhere in `migrations/` either, so this is
genuinely ServiceNow-only, same as tasks — the bug was purely in *how* that
was expressed. Fixed by adding `unavailableFeedbackService`
(`feedback_service.go`), an exact mirror of `unavailableTaskService`: every
method returns the documented `*apierror.ServiceUnavailableError`. `routes.go`
now always constructs `feedbackHandler` (Postgres gets the unavailable
stand-in, same `if cfg.DataSource == ... else ...` shape as `activeTaskSvc`
above) and always registers both routes unconditionally — a real, documented
503 instead of an undocumented 404 callers can't distinguish from a
genuinely missing resource.

**Update:** the "no feedback table" premise above is stale. Migration `0102`
added `work_item_feedback`, and `pgFeedbackService`
(`pg_feedback_service.go`, over `feedback_repo.go`) now serves `POST
/cases/feedback/search` and `/aggregate` for both the `postgres` and
`postgres-servicenow-dual-write` data sources (dual write reads Postgres, never
the backing system). `unavailableFeedbackService` is now only the fallback for
a data source with no feedback store. Known gap: the table stores no
per-rating reason chips, so every `reasons_*` bucket returns an empty result.
`GET`/`POST /cases/{id}/feedback` (the emoji submission contract) now has a
real Postgres implementation too — see the dedicated section below.

## GET/POST /cases/{id}/feedback and GET /metadata's feedbackEmojies on Postgres

`GET`/`POST /cases/{id}/feedback` used to be a hardcoded 503 on this data
source ("case feedback is only supported for the [synced data source]").
Migration `0127`/`0128` (`work_item_feedback_metric`/
`work_item_feedback_metric_option`/`work_item_feedback_reason`, mirrored
from the same upstream sync that already populates `work_item_feedback`
itself) give this a real implementation:
`internal/repository/case_feedback_repo.go` (`CaseRepository.GetCaseFeedback`/
`CreateCaseFeedback`), wired into `case_service.go`'s own `GetCaseFeedback`/
`SubmitCaseFeedback`.

**The emoji catalog is five rows, resolved by name, not by a stored FK.**
`work_item_feedback` (migration `0102`) has no column saying which emoji/
metric a submission picked — only a plain `rating` (1-5) and `rating_label`
(e.g. `"Very Satisfied"`), the exact shape the upstream sync already writes.
Every `"<rating> - Reasons"` row of `work_item_feedback_metric` is, by
construction, its clean rating label plus the fixed `" - Reasons"` suffix,
so `rating_label || ' - Reasons'` always resolves back to the one metric row
a submission's `emojiId` pointed at — `caseFeedbackRatingByLabel`/
`resolveCaseFeedbackRating` hold this fixed, closed 5-value correspondence,
reused by both the per-case endpoints and
`ReferenceDataRepository.ListFeedbackEmojis` (`GET /metadata`'s
`feedbackEmojies` field) so the two can never disagree on what a "rating"
means. A reason's own `option_value`/`reason` are kept as recorded with no
FK to `work_item_feedback_metric_option` (migration `0128`'s own design), so
a since-renamed-or-removed option has no current id to report — that chip
is left out of `GetCaseFeedback`'s result entirely rather than guessed at.

**Two guards, both enforced inside `CreateCaseFeedback`'s own transaction,
in this order:**

1. **The case must already be closed.** This form is a post-closure
   satisfaction survey — the portal only ever offers it once a case has
   closed — so a submission against a case that's still open is rejected
   with a `409 ConflictError` ("feedback can only be submitted once the
   case is closed"), checked via the same `caseLikeStateColumn`/
   `caseLikeJoins` resolution `GetCaseByID`/`SearchCases` already use for
   every case-like type, so "closed" can never drift between this check and
   what the case detail page itself shows.
2. **One submission per case, ever.** `work_item_feedback.work_item_id` is
   `UNIQUE`; the insert is `ON CONFLICT (work_item_id) DO NOTHING`, and zero
   rows returned is a second `409 ConflictError` ("feedback has already
   been submitted for this case").

Every chip submitted must belong to the submitted `emojiId`'s own option
set — a chip from a different emoji's question is rejected with a
`ValidationError`, not silently accepted. `CreateCaseFeedback` validates
`emojiId` against both `work_item_feedback_metric.is_active` and
`selected_image IS NOT NULL` — the exact same definition `ListFeedbackEmojis`
uses for "a real catalog emoji", so a submission can never be accepted for
an id `GET /metadata` would never have offered as a choice in the first
place.

**`GetCaseFeedback` (the read side) is internal-caller-only — an
external/customer caller gets `403 Forbidden` before the repository is even
reached, by explicit product decision.** A case's submitted feedback (the
customer's own satisfaction rating/comment) is a one-way signal meant for
WSO2 staff, never shown back to the customer who submitted it — not even
for a case they are themselves a registered contact on. `caseService.
requireInternalCaller` delegates to the shared `RequireInternalCaller`
(`require_internal.go`), the identical "no scope short of internal is safe
to hand this out under" gate `slaStatusService`'s own `requireInternalCaller`
already uses for the same reasoning.

**`CreateCaseFeedback` (the write side) needs no equivalent explicit
check** — a caller may only submit feedback for a case they actually have
access to, but this is enforced entirely by RLS on the existence/state query
above, not by a second access check in the service layer. Every request's
identity is already stamped onto its context once, by
`callerIdentityMiddleware` (`internal/server/identity_middleware.go`),
before any handler runs; `CreateCaseFeedback` runs inside a transaction that
reads that same identity and sets it as session GUCs, so `work_item`'s own
`FORCE ROW LEVEL SECURITY` already makes a case outside the caller's scope
return zero rows on that one query — the same "exists, just not yours ->
NotFoundError" posture every by-id case read already has. An earlier
revision added an explicit `GetCaseByID` call here (mirroring
`EscalationService.CreateEscalation`'s own check-then-mutate shape) before
realizing it was pure duplication: `GetCaseByID` is the single most
expensive read in this file (~15 joins plus two extra round trips for
tags/watchers), re-proving something the one lightweight query
`CreateCaseFeedback` already runs provides for free. Removed; see
`TestCaseFeedbackIntegration_RejectsSubmissionForAnOutOfScopeCase`
(`case_feedback_repo_integration_test.go`) for the real, RLS-level
regression guard — a service-layer test with a stub repository cannot
exercise this at all, since RLS only exists in real Postgres.

**Identity, not invention.** `AssessmentID` (`CaseEmojiFeedback`/
`CaseFeedbackResult`'s own wire field) is left at its Go zero value on this
data source — there is no assessment-instance concept anywhere in this
schema to populate it from, unlike the synced path's own real id for it.
`SubmittedByUserID` comes from `resolveActor`, the same
`x-user-id-token`-derived lookup every other Postgres-native write in this
file already uses.

## closed_by_user_id was never written or read on the Postgres data source

Reported live: an externally-closed case showed "Case closed by system"
regardless of who actually closed it. `closed_by_user_id` is a real column
on all five case-like extension tables (migrations `0023`/`0024`), and the
field it backs (`CaseView.ClosedBy`) was already wired up on the synced
read path — but `case_repo.go` never selected it in `GetCaseByID`, and
`UpdateCase`'s own state-transition write never set it either. Confirmed
directly against a real case: `work_item_activity` already had the correct
closer's email recorded for its `state` field-change entry (the identity
was available at close time, it just never reached this column).

Fixed on both sides:

- **Write**: `CaseRepository.UpdateCase` gained an `actorID *string`
  parameter — the resolved caller's own `"user"` id, threaded through
  `updateCaseQuery` and the four `caseLikeExtensionUpdate` queries. It is
  stamped onto `closed_by_user_id` only on a transition **to** closed, and
  cleared back to `NULL` on a transition **away** from closed — the
  identical transition-gated shape `closed_on` itself already has.
  `caseService.UpdateCase` resolves `actorID` via the same `resolveActor`
  call its `recordFieldChangeActivity` already uses, from the caller's
  `x-user-id-token` **only** — never from the request body, and never
  guessed at for a pure machine-to-machine caller with no end-user token
  (that caller's close simply leaves `closed_by_user_id` unset, the same
  best-effort posture `actorEmail` already has there).
- **Read**: `GetCaseByID` now joins `"user" closer ON closer.id =` the new
  `caseLikeClosedByUserIDColumn` (a `COALESCE` across all five extension
  tables' own `closed_by_user_id`, mirroring `caseLikeClosedOnColumn`'s
  existing shape) and populates `CaseView.ClosedBy`.

**Forward-only, deliberately.** A case closed before this change keeps
`closed_by_user_id = NULL` forever unless backfilled separately — nothing
here retroactively derives it (e.g. from `work_item_activity`'s own
recorded email), since that would be a data migration decision, not a code
fix.

## CreateCase enforces a project type's product-category allow-list for case/SR

`project_type.default_case_product_categories`/`sr_product_categories`
(`deployed_product_category_enum[]`, migration
`0130_project_type_feature_entitlement.sql`, transcribed from ServiceNow's
own `ProjectTypeFeatureManager.FEATURE_MATRIX`) were, until now, purely
advisory: `ReferenceDataRepository.GetProjectByID` already surfaced them as
`ProjectFeatures.DefaultCaseProductCategories`/`SrProductCategories` via
`GET /projects/{id}/features`, read-only, for the frontend's own product
dropdown to filter against (and `SearchDeployedProducts`' fail-open
NULL-category handling — see that query's own doc comment — exists
specifically so an uncategorized product isn't hidden from that dropdown).
Nothing ever stopped a caller from creating a `case`/`service_request`
against a deployed product whose category didn't match the project type's
own configured requirement at all — the matrix was real configuration with
no enforcement behind it.

`caseService.validateDeployedProductCategoryForType` (`case_service.go`)
closes this at `CreateCase` time, for `type: "case"` (checked against
`DefaultCaseProductCategories`) and `type: "service_request"` (checked
against `SrProductCategories`) only — the two types the matrix actually
names; every other type is unaffected, and a project type with no entry for
the request's own type ("N/A" in the matrix, an empty/nil slice) stays
unrestricted exactly as before this check existed.

**Fail-closed on an uncategorized deployed product, by deliberate product
decision — the opposite of `SearchDeployedProducts`' own read-side
posture.** A deployed product with no `product_category` set (the majority
of real rows today) now FAILS this check once a project type restricts the
request's type, rather than being treated as a wildcard match. The whole
point of this gate is to make categorizing a deployed product matter; the
CSM Portal's own Create/Edit Deployed Product dialogs are what let staff set
one (`apps/csm-portal/webapp`'s `CreateDeployedProductDialog.tsx`/
`EditDeployedProductDialog.tsx`), closing the loop this check opens.

**One call site, nil-safe, covers both the plain-Postgres and dual-write
data sources.** `validateDeployedProductCategoryForType` is called from
`CreateCase` right after the existing `deploymentId`/`deployedProductId`
UUID validation and before the `s.snMirror != nil` branch — so it runs
identically whether `s.snMirror` is set (`DATA_SOURCE=postgres-servicenow-dual-write`,
`createCaseSNFirst`) or nil (plain `DATA_SOURCE=postgres`), with no
duplicated logic. It depends on two new, optional `caseService` fields
(`referenceDataRepo`/`deployedProductRepo`), wired via
`WithProductCategoryEnforcement(svc, referenceDataRepo, deployedProductRepo)`
— a post-construction step, not a new constructor parameter, specifically so
every existing `NewCaseService`/`NewCaseServiceWithSNWriteback` call site
(every test, and `DataSourceServiceNow`'s own `pgCaseFallbackSvc` in
`routes.go`) keeps compiling and behaving unchanged; the two constructors'
own doc comments already established this precedent for exactly this
reason. Both fields nil (the default) skips the check entirely, the same
posture as every other optional `caseService` dependency
(`publisher`/`snMirror`/...).

**`DATA_SOURCE=servicenow` does not get this check, by explicit product
decision** — `routes.go` only calls `WithProductCategoryEnforcement` for the
`DataSourcePostgresServiceNowDualWrite` and default (plain-Postgres)
branches. `snCaseService.CreateCase` never reaches `caseService`'s code at
all (it validates and builds its own ServiceNow payload directly), and its
`pgFallback` field — already used for three other Postgres-only reads — is
not wired to either new repository. Staging/production both run dual-write,
where this data is already available; a plain-ServiceNow deployment is left
as a documented, known gap, same posture as every other Postgres-only
feature in this file.

`DeployedProductRepository.GetDeployedProductCategory(ctx, id)` is the one
new repository method this needed — a single-row lookup
(`SELECT product_category::TEXT FROM deployed_product WHERE id = $1`,
lower-cased before returning), deliberately not reusing
`SearchDeployedProducts`' list/filter machinery for a one-row check.

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
- **Caller-supplied aliases for an enum field** (e.g. `caseTypeAliases` in `case_service.go`, resolving `"default_case"` to the canonical `"case"`) exist because a real, currently-in-production caller was built against a different value than this service's own canonical one — usually the raw upstream (ServiceNow) wire value, from before this service introduced its own domain-level enum. Normalize via the alias map as the FIRST thing that happens to the value, before it reaches any `validXxx` map, data-source-specific translation (e.g. `snCaseTypeMap`), or the Postgres repository/DB enum cast — every one of those must only ever see the canonical value, never the alias. Add a new alias here rather than either (a) teaching every downstream consumer about a second valid spelling, or (b) asking the caller to change, since the caller is an already-deployed frontend, not something this change can update in lockstep.

## Repository conventions

- Each entity gets one file; the `<Entity>Repository` interface lives at the top of the same file
- Use `pgx.ErrNoRows` to detect missing rows and return `*apierror.NotFoundError`
- Wrap unexpected errors with `fmt.Errorf("operation name: %w", err)` for traceability
- PostgreSQL enum casts are required for enum columns (e.g. `$1::case_state_enum`)
- For queries that need both a COUNT and a SELECT, run them concurrently with `errgroup` (see `SearchCases` and `SearchCaseComments` in `case_repo.go`)
- A search whose caller never shows a total sets `skipTotal` on its request (`domain.SearchCasesRequest`, `SearchIncidentsRequest`, `SearchChangeRequestsRequest`, `SearchProblemsRequest`, `SearchConversationsRequest`): the repository then skips the COUNT entirely and the response's `total` is `domain.TotalNotComputed` (-1), not a lower bound. Three paths ignore the flag and keep reporting a total: the ServiceNow data source, a case search with `groupBy` (bucket counts) and `POST /announcements/registry/cases` (the number returned), so a consumer treats -1 as "no total" and must not assume the reverse. Global search (the CSM portal's quick-nav palette) sends it; the COUNT costs as much as the page query and holds a second pool connection. The default is unchanged. Request bodies are decoded with `DisallowUnknownFields`, so a caller must not send `skipTotal` to a build that predates it (the portal webapp falls back to the plain search on a 400). `search_skip_total_integration_test.go` records the statements each repository sends to prove the COUNT is not run and the page is identical.

## Domain types

All shared types live in `internal/domain/entity.go`. Conventions:

- JSON field names use camelCase (`json:"fieldName"`)
- Request structs include only the fields a caller can supply; ID fields injected from path params use `json:"-"`
- Optional fields in request structs use pointer types (`*CasePriority`) so absent fields are distinguishable from zero values
- Response structs return the full entity row
- **Date/time field naming:** all timestamp fields in response structs must use the `On` suffix: `createdOn`, `updatedOn`, `closedOn`. Never use `At` (`createdAt`, `updatedAt`, `closedAt`). Domain-specific date fields that carry a business meaning (e.g. `startDate`, `endDate`, `activationDate`) keep the `Date` suffix. This applies to both Go struct field names and JSON tags.
- **Empty strings must never appear in responses where the value is absent.** Use pointer types (`*string`, `*EntityRef`, `*DeployedProductRef`, etc.) for any response field that may be absent, and leave them `nil` so they serialise as JSON `null`. Never assign an empty-string value to a non-pointer field as a stand-in for "not present". For optional sub-fields within a required struct (e.g. `UserRef.ID` when only the email is known), add `omitempty` to the JSON tag so they are omitted rather than serialised as `""`.
- **Request enum field naming:** enum fields in request structs use plain field names with no suffix — both in the Go struct field name and the JSON tag (e.g. `State \`json:"state"\``, `Priority \`json:"priority"\``, `Type \`json:"type"\``; arrays: `States \`json:"states"\``, `Priorities \`json:"priorities"\``). UUID ID fields use the `ID` / `IDs` suffix: `ProjectID \`json:"projectId"\`` / `ProjectIDs \`json:"projectIds"\`` (no `Key`). Response structs follow the same plain naming. When mapping to ServiceNow SN payload structs internally, field names in those private structs may use `Key` suffix where required by the Choreo API contract (e.g. `riskKey`, `stateKey`).
- **Enum fields in responses (search and detail):** always render enum-valued fields as plain nullable strings using `UPPER_SNAKE_CASE` domain enum values (e.g. `"priority": "HIGH"`, `"state": "IN_PROGRESS"`, `"category": "SECURITY"`). Never return raw SN labels (e.g. `"1 - High"`, `"In Progress"`) or `{id, label}` objects. Map the SN id (integer or string key) through the domain label map in the service layer. If the SN id is not present in the map, leave the field `nil` rather than falling back to the raw label.

## Error types (`internal/apierror`)

| Type                    | HTTP status | When to use                              |
|-------------------------|-------------|------------------------------------------|
| `*ValidationError`      | 400         | Invalid input supplied by the caller     |
| `*NotFoundError`        | 404         | Requested resource does not exist        |
| `*ServiceUnavailableError` | 503      | Downstream dependency temporarily down   |

`apierror.WriteJSON(w, status, msg)` writes `{"code": <status>, "message": "<msg>"}`.

**Machine-readable `errorCode`.** The `message` is wording for people and changes as it is improved, so a client must never branch on it. A refusal a client has to tell apart from the others of its status carries a stable `errorCode` string beside the message: `{"code": 409, "message": "...", "errorCode": "change_request_on_hold"}` (`apierror.ErrorResponse.ErrorCode`, written by `apierror.WriteJSONWithCode`; omitted when there is none, so an unnamed refusal's body is exactly what it always was). Only `*ConflictError` and `*ForbiddenError` have a `Code` field today (set where the refusal is raised, `Code: apierror.CodeChangeRequestOnHold`; `writeServiceError` writes it). Rules: **no database change** (it is attached in code, nothing is stored); the status and the message of the refusal are untouched by naming it; the constants live in `internal/apierror/codes.go`, are lower `snake_case`, are **only ever added to, never renamed or reused for another meaning** (`TestCodes_AreStableLowerSnakeCase` pins them), and the OpenAPI `ErrorResponse.errorCode` description lists them (deliberately not a closed enum, so a client treats a value it does not know as "no more specific than the status"). The customer portal's PATCH `/change-requests/{id}` (a customer's answer or proposed implementation time) and the approvals decision route name these:

| Refusal | `errorCode` | Status |
|---|---|---|
| A proposed implementation time on a change request WSO2 has on hold (the answer itself is still possible) | `change_request_on_hold` | 409 |
| An answer that names the planned window the customer was shown (`expectedPlannedStartOn` / `expectedPlannedEndOn`) when it is no longer the change request's window; nothing recorded | `change_request_schedule_changed` | 409 |
| An answer to an approval that is no longer pending: already answered (by the caller or a sibling contact), the change request has left the state the answer belongs to (`staleApprovalRefusal`), or no request for it is open | `change_request_approval_not_pending` | 409 |
| A proposed time on a change request that is not in Customer Approval, or that nobody has been asked to approve | `change_request_not_proposable` | 409 |
| A registered contact the customer's request was never sent to (or whose request a sibling's answer or a re-schedule withdrew): proposing, or answering on a live customer stage | `change_request_not_asked` | 403 |
| Not a registered PORTAL_USER contact of the change request's project, the change request's own creator, a user who may not decide an internal stage, a field a customer may not set, a caller with no user record | `change_request_forbidden` | 403 |

`TestWriteServiceError_CarriesTheMachineReadableCode` (handler) and `TestChangeRequestErrorCodesIntegration_*` (repository, against a real database) pin each code to its refusal. customer-portal `backend-v2` and the CSM portal BFF pass the code through with the status they give it; the customer webapp classifies a refusal by it (`describeChangeRequestActionError`), and a 409 with a code it does not know, or none (an older entity-service), is "something went wrong, refresh", never "already answered".

**Never put `pgErr.Detail` verbatim in a `ValidationError.Msg`.** `writeServiceError`'s own comment states a `ValidationError`'s message is always safe to return to the caller as-is, but a Postgres foreign-key violation's `Detail` field quotes the real table and column name (e.g. `` Key (assigned_to_id)=(...) is not present in table "user". ``) — handing an API caller schema internals. When a `23503` can be attributed to a specific request field (e.g. via `pgErr.ConstraintName`, since none of this schema's inline `REFERENCES` get an explicit `CONSTRAINT` name, so Postgres's default `<table>_<column>_fkey` naming applies), name that field instead. See `change_request_repo.go`'s `changeRequestPatchFKField` map for the pattern. Several older `23503` handlers elsewhere in `internal/repository/` (`case_repo.go`, `time_card_repo.go`) still return `pgErr.Detail` this way — a known pre-existing gap, not newly introduced, and not yet fixed.

## Database migrations

Migrations live in `migrations/` as plain SQL files, numbered `NNNN_<description>.sql` (4-digit, single file, no separate `.up`/`.down`) — matching `operations/csm-sync-service`'s own convention exactly, since that service and this one migrate against the same shared Postgres database. This replaces the older `000NNN_<description>.up.sql`/`.down.sql` convention (6-digit, up/down pairs) this file used to document; every migration under the old convention was renumbered/consolidated into the new one, not left running side by side with it.

- **Each file is a complete, forward-only migration** — there is no scripted rollback. A change that needs undoing is a new forward migration, not a `.down.sql`. `IF NOT EXISTS`/`IF EXISTS` guards (already this repo's convention) make every file safe to re-run.
- **A migration file itself carries no tracking statement.** `make migrate` (Makefile) creates `csm_migration_applied_migration` (`filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now()`) if absent, then for each `migrations/*.sql` file, in ascending order: skips it if its name is already in that table, otherwise applies it (`psql -f`) and only then records it with a separate `INSERT INTO csm_migration_applied_migration (filename) VALUES (...)` — this exactly mirrors `operations/csm-sync-service`'s own `make migrate` loop, since both services must track migrations against the same shared database the same way. `scripts/generate_schema_bootstrap.sh` (a combined-file generator for a from-scratch DB, also ported from that service, supporting `--since`/`--from`/`--to` for a delta) is the one thing that *does* append the tracking insert per migration — necessary there because a single concatenated file has no per-statement loop to do it externally.
- **Numbers `0001`–`0102` are a byte-for-byte, contiguous mirror of `operations/csm-sync-service`'s own `migrations/0001`–`0102`**, including its control-plane tables (`migration_job`/`migration_run`/`sync_checkpoint`/`schema_version`, renamed to the `csm_migration_` prefix at `0091`) — entity-service's own Go code never queries those tables, but the file is kept here anyway so `make migrate` produces the *identical* resulting schema whichever repo it's run from, not just an overlapping subset. A handful of these (e.g. `0025`/`0033`/`0098`) are no-ops against this repo's own already-correct `CREATE TABLE` statements (guarded by `IF EXISTS`/`IF NOT EXISTS`/an already-true condition) — kept anyway, for the same reason. Beyond `0102`, this repo has its own entity-specific migrations that only exist on this side (`0103`+ covers GitHub integration, announcement requests, onboarding steps, the Team Schedule tables, and more — none of it sync-service's concern) — **but a shared-table migration from sync-service is still pulled in verbatim under its own real filename whenever one lands, even at a number this repo has already used for something else of its own.** `0090_outage_affected_ci_table.sql` and `0103`–`0108` (`product_name_unit_unique`, `project_add_onboarding_owner`, `product_version_deployment_profile_unique`, `incident_category_add_missing_values`, `incident_resolution_code_add_resolved_by_caller`, `case_cause_add_user_mistake`) are exactly this: sync-service migrations mirrored in unchanged, coexisting at the same leading number as this repo's own unrelated files there — normal per "Duplicate migration numbers are normal here" in the top-level `cs-tools/CLAUDE.md`, since the tracking key is the full basename. **Never rename a mirrored file to avoid the collision or to fit this repo's own sequence** — the filename is what `csm_migration_applied_migration` tracks it by, so a rename makes `make migrate` treat an already-applied sync-service migration as brand new and re-run it from scratch against a database where the real, differently-named version already ran.
- **Mirroring a file means copying its SQL, never its license header.** `operations/csm-sync-service` is a proprietary repo and every file there opens with WSO2's proprietary "All Rights Reserved" copyright block; this repo is the open-source mirror (`fork-repos/OpenSource/cs-tools`) and every file here — no exception, migrations included — opens with the Apache 2.0 header instead (see any existing migration for the exact text). A plain `cp` of a new file from sync-service carries the wrong header over; this was missed across ~30 files in one pass before being caught and fixed in bulk (`592fcb844`). When adding a migration that's missing here entirely, replace the copyright block (everything up to the first blank line) with this repo's own Apache header before saving it — never copy the file as-is. A sync-service file with no header at all (some genuinely have none) still gets the Apache header added, matching every other file in this directory.
- **Whenever a new migration touches a shared table (not something entity-service-only), check `operations/csm-sync-service/migrations/` directly for the next real number before picking one here** — its migrations are the authoritative record of what actually runs against the shared database, and it has continued past whatever this file's own highest number was at any given time. Picking a number here that sync-service has already used for something else creates two same-numbered-but-different migrations across the two repos; `make migrate` from either repo would then apply both under different filenames with no conflict *detected*, silently leaving whichever repo didn't get involved missing the other's columns/tables. When sync-service adds a migration for a table entity-service also cares about (or its own control-plane numbering advances), mirror the file here at the same number, the same way `0101`/`0102` (`account_support_fields`/`work_item_feedback_table`) were pulled in.
- **The identical collision can happen entirely within this repo, with no other service involved.** Two branches cut from the same base each see the same "current highest number," each add their own next-numbered file, and both PRs merge cleanly — git sees two different filenames, so there's no merge conflict to catch it. The result is the same silent, undetected collision as the cross-repo case above: two unrelated migrations sharing one number, `make migrate` applies both under their own filenames without complaint, and the numbering no longer identifies one unambiguous point in the sequence. Rebase onto the target branch's actual latest `migrations/` state before opening a migration PR, and check for a same-number collision as part of reviewing one — this repo has no CI check enforcing unique leading numbers today.
- **`ALTER TYPE ... ADD VALUE` migrations stay the only statement in their file** — it cannot run in the same transaction as a later statement that uses the new value, and every file here is expected to be applied with plain autocommit (never wrapped in `BEGIN`/`COMMIT`, never run with `psql -1`/`--single-transaction`).

Each migration creates its PostgreSQL enums, sequences, and tables in a single transaction (aside from the `ALTER TYPE ... ADD VALUE` exception above). Apply them in ascending order before starting the service — `make migrate` does this.

Key conventions enforced at the DB level:
- Primary keys are `UUID DEFAULT gen_random_uuid()`
- Human-readable IDs (e.g. `CASE-001`, `WSO2-001`) are generated from dedicated sequences via column defaults
- Enum types (e.g. `case_state_enum`, `case_priority_enum`) enforce valid values at the DB level; Go enum validation in the service layer is an additional guard
- Triggers enforce relational constraints that foreign keys alone cannot express (e.g. deployment must belong to the same project as the case)
- **Table names are always singular** (`case`, `user`, `comment`, `product_vulnerability`, `case_attachment`, ...), never plural (`cases`, `users`, `case_attachments`). A plural name (`case_attachments`) has been introduced by mistake before and had to be renamed later — check this before adding a new `CREATE TABLE`.
- **Timestamp columns always use the `_on` suffix** (`created_on`, `updated_on`, `resolved_on`, `started_on`, `due_on`, ...), never `_at` (`created_at`, `updated_at`). This mirrors the JSON `On`-suffix convention under "Domain types" below — the DB column and the wire field should read the same way. Several migrations (`alert_incident_mapping`, `event_publish_failures`, the now-removed `sla_clocks`, `case_attachment`, `scheduled_task_run`, `sn_writeback_failures`, `announcement_requests`) used `_at` before being fixed — check this before adding a new `TIMESTAMPTZ` column.

## OpenAPI spec

`openapi.yaml` is the source of truth for the API contract.

- Error responses reference `$ref: '#/components/schemas/ErrorResponse'`
- Path parameters that accept UUIDs must declare `format: uuid`
- Every writable endpoint (POST, PATCH) needs 400 and 404 responses in addition to the success response
- Schema names should match the Go domain type names (e.g. `CreateCaseRequest`, `Case`)

## Connection pool settings

Tuned via `config.Config`, applied by `internal/db.NewPool`. Each is env-configurable (`internal/config/config.go`); the values below are what an unset deployment gets — identical to what this file used to hardcode before these existed:

| Setting             | Env var                       | Default |
|---------------------|--------------------------------|---------|
| Max connections     | `DB_POOL_MAX_CONNS`            | 20      |
| Min connections     | `DB_POOL_MIN_CONNS`            | 2       |
| Max conn lifetime   | `DB_POOL_MAX_CONN_LIFETIME`    | 30 min  |
| Max idle time       | `DB_POOL_MAX_CONN_IDLE_TIME`   | 5 min   |

`DB_POOL_MAX_CONNS` falls back to its default on an unset, non-numeric, or non-positive value (a pool that may open no connections at all can never serve a single query). `DB_POOL_MIN_CONNS` falls back the same way **except zero is accepted** — pgxpool genuinely permits a minimum of 0 (a deployment that doesn't want to retain any idle connections) — same fail-safe-to-default posture `getDurationOrDefault` already uses for every duration-shaped env var here, now shared by `getInt32OrDefault`. An invalid value for any of the four surfaces through `Config.Validate()` at startup (`loadErr`), the same mechanism `SERVER_READ_TIMEOUT`/etc. already use.

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
- **Running gosec** — this module's `go.mod` floor is newer than the Go bundled in
  `securego/gosec:latest`, and that image sets `GOTOOLCHAIN=local`, so the scan
  silently loads **zero files** and reports `Issues: 0` — a pass that examined
  nothing. Pass `GOTOOLCHAIN=auto` and check the `Files:` count is non-zero:

  ```bash
  docker run --rm -v "$PWD":/src -v gomod:/go/pkg/mod -w /src \
    -e GOTOOLCHAIN=auto securego/gosec:latest -fmt=text ./...
  ```

- **Security fixes in PRs** — when a change is made to fix a security issue (gosec findings, input sanitization, etc.), do not mention it in the PR title or description; describe the change in neutral functional terms only
- **Run govulncheck on every change** — `govulncheck ./...` (install once: `go install golang.org/x/vuln/cmd/govulncheck@latest`) must report no vulnerabilities before opening a PR. Most findings here are Go standard-library CVEs tied to the toolchain patch version pinned in `go.mod`'s `go` directive — bump it to the latest `1.26.x` patch (and run `go mod tidy` so the toolchain download matches) rather than working around the symptom. A finding in a third-party module (e.g. `golang.org/x/text`, pulled in transitively via `pgx`) is fixed with `go get <module>@<fixed-version>`

## Work state and resolution code on non-case types (migration 0184)

PR #2289 made `UpdateCase` write the right extension table for each case-like
type, and rejected `severity`/`workState`/`resolutionCode` for every type but
`case`. ServiceNow disagrees for two of the three: every case-like record lives
in `sn_customerservice_case`, and on wso2sndev in-progress service requests,
engagements and security report analyses carry `u_work_state` (1 = Ongoing,
2 = Paused), and closed ones carry `resolution_code`. Announcements carry
neither. Severity (`priority` 9–14) is effectively case-only; service requests
use `priority` 1–4, a different scale, which this does not model.

With `workState` rejected, the portal's Start progress left a service request
in Work in Progress with an empty work state, shown as "Paused", and public
replies stayed locked, since both the BFF and the webapp require `ongoing`.

- **Migration 0184** adds `work_state case_work_state_enum` and
  `resolution_code case_resolution_code_enum` (the "case" enum types) to
  `service_request`, `engagement` and `security_report_analysis`. Applied by
  hand like 0179-0181; the sync service does not create or fill them yet.
- **`validateUpdateCaseFieldsForType`**: `severity` stays case-only;
  `workState`/`resolutionCode` are rejected only for announcements.
- **Update queries** for the three types write both columns (`$5`/`$6`) and
  return `work_state`. `caseLikeExtensionUpdate` builds the statement and
  arguments for every non-case type, shared by `UpdateCase` and the
  one-Ongoing path.
- **One Ongoing per engineer** now spans case, service request, engagement and
  security report analysis (`workStateWorkItemTypes`), matching the webapp's
  own conflict lookup, which searches every case-like type.
- **Reads** (`GetCaseByID`, `SearchCases`, the `workState` filter and
  aggregate) use `caseLikeWorkStateColumn`/`caseLikeResolutionCodeColumn`.
- **Dual write** needed no change: the ServiceNow mirror for state, work state
  and resolution fields already PATCHes the shared case record, whatever its type.

## Incident report flows (migration 0181)

Ports two ServiceNow flows, both "Incident Updated where State changes to X", one step each:

| SN flow | Trigger | Postgres effect |
|---|---|---|
| Create Incident Report Task | state → `IN_PROGRESS` | inserts `work_item` (type `INCIDENT_TASK`, number from `next_portal_work_item_number()`) + `incident_task`: subject `[Incident Report] Create the incident report for <number>`, service / assignment group / assignee copied from the incident, priority `CRITICAL`, type `INCIDENT_REPORT`, state `OPEN` |
| Incident Report Generator | state → `RESOLVED` | overwrites `incident.incident_report` with SN's HTML template: number, priority label, created time (UTC) filled; Timeline … Next Steps left as `-` |

**Postgres only, by design.** In dual-write mode ServiceNow's own flows keep writing ServiceNow;
this writes the side the portal reads. It never calls ServiceNow (the drainer has no user token,
and SN has no incident-task create endpoint).

**Mechanism.** 0181 attaches 0051's `trg_event_outbox` to `incident` (AFTER UPDATE only — like
SN, an incident *inserted* already In Progress creates no task). `IncidentReportDrainer`
(`internal/service/incident_report_service.go`) reads `entity_type = 'incident'` rows.

**Unlike the CR / cloud-status drainers, nothing is marked done at claim time.** Each row is
locked (`FOR UPDATE SKIP LOCKED`), applied, and marked published in ONE transaction; a crash or
failed write rolls all of it back and the row is retried. 0181 adds `attempts`, `last_error`,
`last_attempt_on` to `event_outbox` for this: backoff 30s doubling to a 1h cap, parked after
`IncidentReportMaxAttempts` (10, ≈3h). Re-drive a parked row with
`UPDATE event_outbox SET published_on = NULL, attempts = 0 WHERE id = …`.

The generator's own write is an incident UPDATE too, so it lands in the outbox — with only
`incident_report` in its diff, which the drainer acknowledges as a no-op. Do not "fix" that by
filtering in the trigger.

**No on/off switch, like the ServiceNow flows.** The drainer starts whenever there is a database
pool (`INCIDENT_REPORT_POLL_INTERVAL`, default 5s, is the only setting) — even with
`DATA_SOURCE=servicenow`, because a drainer that is off lets the trigger's rows pile up and replays
them as stale tasks when it comes on. Always running means there is never such a backlog, so there
is no start cutoff and no age limit: a change is applied however late, and an outage only delays.

Tests: `incident_report_service_test.go` (unit), `incident_report_integration_test.go`
(`INCIDENT_REPORT_TEST_DSN`, real DB with all migrations: both flows, rollback, backoff, retry).

### [WSO2 Cloud Ops] Post resolution tasks (migration 0188)

Runs in the same Resolved handler, after the report, in the same transaction. SN condition:
service Choreo or Asgardeo, state changes to Resolved. **Deliberate divergence:** the alert tasks
keep that service limit, but the workaround problem is created for an incident on **any** service
(product decision, 2026-10-06). Every block is an independent If on the incident as it is now:

| Condition (`resolution_code`) | Effect |
|---|---|
| `FALSE_ALARM` | incident_task `[Alert Task][Falser Alarm] <number> alert is a false alarm` (SN's spelling), `CRITICAL`, group WSO2 SRE Team |
| `DUPLICATE` or `DUPLICATE_ALERT` | `[Alert Task][Duplicate Alert] <number> alert is a duplicate`, `CRITICAL`, WSO2 SRE Team. Both spellings are SN's one "Duplicate" choice: the sync writes `DUPLICATE_ALERT`, the portal `DUPLICATE` |
| `NOT_ACTIONABLE_ALERT` | `[Alert Task][Not Actionable Alert] <number> is not an actionable alert`, `HIGH`, WSO2 SRE Team |
| `SOLVED_WORK_AROUND` and no `problem_id` | problem `Fix the root cause of <number>` with the incident's service, impact, urgency and priority (0188 adds `problem.service_id/impact/urgency`), `incident_id` = the incident, group Choreo Special Ops (Choreo), Asgardeo Operations Team (Asgardeo), otherwise the incident's own assignment group (none if it has none); then `incident.problem_id` = it |

**Dual-write (`postgres-servicenow-dual-write`): the workaround problem is written to both
stores, by the resolve request, not this flow.** A problem that exists only in Postgres gets a
CS-PORTAL number and cannot be moved through its states (every problem transition is
ServiceNow-first, by id: the PATCH 404s). The flow has no user, and the CSM API needs the caller's
`x-user-id-token`, so `UpdateIncident` creates it (`workaround_problem.go`): when the request moves
the incident to Resolved as Solved (Workaround) and it has no problem, `createProblemSNFirst`
(subject + primary incident → ServiceNow's id, PRB number and priority, stored as-is), then
`ProblemRepository.LinkWorkaroundProblem` sets the group and `incident.problem_id` in Postgres; the
stored group is mirrored to the problem and `problemId` rides the incident's own mirror (ServiceNow's
`createProblem` sets `u_incident` but never `incident.problem_id`). **Both stores hold the same
values**: the CSM API takes no service, impact or urgency (discovery script 72; `ProblemUtils`
reads only subject/description/category/subcategory/priority/originCaseId/primaryIncidentId, and the
Priority Problem Lookup overwrites priority), so neither store gets the incident's -- that needs a
`ProblemUtils` change first. A failure is logged and never undoes the resolve.
`NewDualWriteIncidentReportService` (main.go) runs this flow without the problem block. Sending
`problemId` with the Resolved state in one ServiceNow update also keeps ServiceNow's own active
copy of this flow from creating a second problem (its block 8 needs an empty `problem_id`). Reads
stay on Postgres.

The services and groups are SN sys_ids as Postgres UUIDs, constants in
`incident_report_service.go`. A group missing from the database leaves the record unassigned
rather than failing the change (the insert looks the id up). **Not ported:** the runbook block
(`u_runbook_solve_the_issue = 2` and not a workaround → `[Runbook Task] Modify the runbook`):
the field has no column and no portal input. `MissingSchema` also checks 0188's columns.

Tests: `post_resolution_tasks_test.go` (unit), `post_resolution_tasks_integration_test.go`.
