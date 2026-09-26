# Entity Service

Go HTTP server (`net/http`, standard library only) that owns all core CS-platform entities: users, accounts, projects, products, deployments, deployed products, cases, and case comments. It exposes a REST API consumed by portal BFFs and other internal services.

## Architecture

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
| `SERVER_PORT` | no       | `8080`  | Main API listen port       |
| `HEALTH_PORT` | no       | `8081`  | Health probe listen port; `Validate` rejects it being equal to `SERVER_PORT` (see "Health probes" below) |
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
| `CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED` | no | `false` | Must be `"true"` for `POST /salesforce/events` to act on `Project_Contact__c`/`Contact` envelopes (see "Salesforce membership ingest" below). The Account branch is unaffected |
| `CSM_MIGRATION_PORTAL_WRITES_ENABLED` | no | `false` | Must be `"true"` to register the four portal-driven membership write routes under `/projects/{id}/contacts` (see "Portal-driven membership writes" below). Also needs `DATA_SOURCE=postgres`, a pool, and the full `SALES_ENTITY_*` set (`Config.HasPortalMembershipWrites`). Off means the routes are **not registered at all**, not 403 |

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
Salesforce REST; the REST sales entity-service does. REST Customer has no AccountNumber,
Account_Vertical__c, or Technical_Owner_2 — `number` falls back to Salesforce
Id and those other columns stay null.
Non-Account entities return 204 and are ignored (do not 400 — ASB would
retry forever). DELETED soft-deletes by setting `deactivation_date`; never
`DELETE FROM account` (project → account is `ON DELETE CASCADE`).

## Salesforce membership ingest and onboarding steps

The same `POST /salesforce/events` endpoint also ingests customer **memberships**
— Salesforce `Project_Contact__c` (a Contact's membership of a project) and
`Contact` — when `CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED=true`. Off by default:
`routes.go` then constructs the service with `NewSalesforceEventService`, which
acknowledges those entities with 204 and ignores them (the behaviour before this
branch existed). On, it uses `NewSalesforceEventServiceWithMembershipIngest`
with a `service.MembershipIngest` (membership repo, onboarding-step repo, the
same `salesentity.Client`, and the optional `eventPublisher`). This is the
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
| `Project_Contact__c` | DELETED | `project_contact.state = DEACTIVATED` for that `sf_id`; unknown id is a no-op (still 204). Never `DELETE FROM` |
| `Contact` | UPDATED | `GetContact`, then the CREATED/UPDATED path above for each of its `memberships` (name / email / `isCsAdmin` / `isCsIntegrationUser` changes propagate); every membership is attempted, the first error is returned |
| `Contact` | CREATED / DELETED | no-op |
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
   back to the project's own account → 404.
3. `"user"` by `sf_id`, else by `LOWER(email)` — exactly one (`"user".email` is
   not unique; two matches are a 409 rather than a guess) — else inserted with
   `user_name = lower(email)`, `is_active = true`, `is_system_user =
   isCsIntegrationUser`. An existing row gets name / email / `is_system_user`
   refreshed, **never `user_name`** (it is the join key to `account_contact`).
4. Global roles (`user_role`, `mapGlobalRoles`): always `external`; `partner`
   for a PARTNER CONTACT else `customer`. Nothing is revoked here. An
   integration user gets no global roles at all. A role name missing from the
   `role` table is a 503 naming it — the ServiceNow sync seeds those rows.
   The admin role is **not** decided at this step any more — see step 8.
5. `account_contact` by (`sf_id`, account), else (account, `LOWER(user_name)`),
   else inserted (`is_active = true`, `is_primary_contact = false`).
6. `project_contact` by `sf_id`, else (project, account_contact), else
   inserted; `email` and `state` (`INVITED` / `REGISTERED` / `RE-INVITED` /
   `DEACTIVATED`, `normalizeMembershipState`; anything else is a 400) updated.
7. Project groups (`project_contact_group`, `mapProjectGroups`, §6.4 of the
   onboarding design): `Portal user` + `Security Contact` → `Full Access`;
   `Portal user` → `General Access`; `Security Contact` → `Security Only`;
   `Lead` additionally → `Lead User Group`; `Admin` additionally → `Admin`
   (the group carrying the `ADMIN` project role, migration 000084 — `Admin`
   used to be global-only and recorded nothing per project); unknown roles are
   logged as `ignoredRoles` and never fail the ingest. The row set is
   replaced. A missing `project_group` row is a 503.
8. The derived account-level admin roles (`syncDerivedAdminRole`), run
   **after** step 7 so the membership just written counts: one aggregate over
   every membership this user holds, deciding `customer_admin` and
   `partner_admin` **separately** — each from the live ADMIN memberships that
   can support it — then granting each role it earned and revoking each one it
   did not. The contact's own `isCsAdmin` is an additional grant of the role
   *this* membership maps to. See "Admin is a project role" below for the two
   bugs this replaced.

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

**Schema prerequisite**: the `sf_id` columns on `"user"`, `account_contact`
and `project_contact` come from the csm-sync migration 0076, which is not in
this repo's `migrations/`; the ingest fails at the first `SELECT ... sf_id`
without it. `role` must contain `external`, `customer`, `partner`,
`customer_admin`, `partner_admin`; `project_group` must contain the four
groups above.

**Onboarding steps API** (`onboarding_step`, migration 000075; Postgres-only,
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
  (`AccessScope.Unrestricted`, i.e. `AUTH_INTERNAL_CLIENT_IDS`); anyone else
  gets 403. `created_by`/`updated_by` is `onboarding-step-api` —
  callers are internal services, no identity is derived from the request.
- `GET /onboarding-steps/{membershipSfId}` → `{steps: [...]}` in step order; an
  unknown membership is an empty list, not a 404.
- `POST /onboarding-steps/search` — `{filters: {projectId?, membershipSfIds?,
  statuses?}, pagination}` → `{steps, total, limit, offset}`, newest first,
  `normalizePagination` (limit 20, max 50).

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

Seven call sites publish today, all ServiceNow-data-source-only (`DATA_SOURCE=servicenow`;
there is no Postgres-backed equivalent for any of them). There is also one
Postgres-only, currently-inert exception: `caseService.UpdateCase`
(`case_service.go`) detects when a severity update crosses the LOW boundary
(entering it should make every time card on the case billable, leaving it
non-billable — LOW is WSO2's own support-policy "S4/Queries" tier, same
mapping `sla_policy.go` uses) and logs it, but its actual
`events.TypeCaseBillableStatusChanged` publish is commented out — see that
type's own doc comment in `internal/events/events.go` for why (no consumer
exists yet; Postgres has no `time_cards` table/repo/service at all today, a
prerequisite for the intended reaction). `caseService` gained a `publisher
EventPublisherService` field for this (nil the same way `snCaseService`'s
own `publisher` can be), wired from `routes.go`'s existing `eventPublisher`
var.

**Special case, detects and logs only — no behavior change yet**:
`caseService.AddCaseTag` calls `detectPatchTagBillableOverride`, which
*detects and logs* (nothing more) when a case tagged `"patch"`
(case/whitespace-insensitive) is currently at LOW severity — the eventual
intent is that WSO2 still covers a patch under support even for an
otherwise best-efforts S4 case, so such a case's time cards should one day
become non-billable regardless (one-directionally: removing the tag would
never reverse it), overriding the normal "entering S4 makes time cards
billable" rule. **Today this changes nothing**: no time card's billable
status is altered, no event is published, and no tag is ever persisted.
**TEMPORARY**: case tags have no real Postgres storage at all yet (no
`case_tags` table/repo — `AddCaseTag`/`RemoveCaseTag`/`SearchTags` are
ServiceNow-only, see `sn_case_service.go`'s own real implementations), so
`AddCaseTag` on this data source still always returns a 503 regardless of
this detection — added at explicit request, ahead of both real tag storage
and a real time-card reaction, so the rule's logic is demonstrable now and
easy to wire up for real once both exist.

- **`snCaseService.CreateCase`** publishes `case.created` via a private
  `publishCaseCreated` helper, called after the SN create call succeeds.
  Rather than building the payload from `req`/the create response (which
  carries only a few fields — see `snCreateCaseResponse`), it re-fetches the
  case via `GetCaseByID`, whose own SN response already resolves the
  reporter's display name, the project's name, and each watcher's email —
  exactly what `events.CaseCreatedPayload` needs. `Recipients` is the
  resolved watch list's emails only (an explicit, deliberate decision — this
  service has no other notion of who should be emailed for a case); a case
  created with no watchers is a normal state, not an error, so publishing is
  silently skipped rather than sending a payload
  `csm-notification-service`'s `events.Validate` would reject anyway for an
  empty `recipients` list. The same skip applies when the case has no
  severity: `CaseCreatedPayload.Priority` has no `omitempty` (a consumer
  always expects a real value) and `""` is not a real priority. Since
  severity is a required, validated field for `type: "case"`
  (`validateCreateCaseRequest`), this only actually triggers for the other
  four types `publishCaseCreatedEvent` also serves —
  `announcement`/`engagement`/`service_request`/`security_report_analysis`
  have no severity concept at all (a `"case"`-only column) — so none of
  those four ever publish `case.created`, by explicit request, not by
  oversight.
- **`snIncidentService.CreateIncident`** publishes `incident.created` via
  `publishIncidentCreated`, called the same way. No enrichment round trip is
  needed here: `req.Subject`/`req.AdditionalComments` already carry
  everything the payload needs (`Title`/`ShortDescription`, the latter
  falling back to `Subject` when `AdditionalComments` is absent). This
  service does not build or send an `IncidentLink` at all — this stays
  strictly a publisher of the fact that an incident was created, nothing
  more; `csm-notification-service` builds its own "Open in Portal" link
  from the event's `EntityID` (`recipientlinks.Resolver.IncidentLink`), the
  same way it already builds `case.created`'s portal link rather than
  trusting a caller-supplied one — see that service's own `CLAUDE.md`.
  Likewise, neither `Product` (which Google Chat space) nor `CallTo`
  (on-call number) is ever set from this service — per explicit decision,
  all notification-routing resolution belongs entirely in
  `csm-notification-service`, which substitutes its own configured defaults
  (`DEFAULT_CHAT_PRODUCT`/`INCIDENT_DEFAULT_CALL_TO`) when either is absent
  from the payload. Consuming events and sending emails/Chat alerts/calls is
  never this service's job — only publishing the raw fact that something
  happened is.
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
  not just receive an already-filtered recipient list.

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
payloads' `Product` field.
`CaseCreatedPayload.Product` was previously never populated at all ("this
service has no data source for it yet"); now it doubles as both a display
value in `csm-notification-service`'s redesigned `case.created` Chat card
and that service's own Chat-space routing key (`GoogleChatConfig.Spaces`
matches on it, falling back to `DEFAULT_CHAT_PRODUCT` when empty) — an
operator's `GOOGLE_CHAT_SPACES` config needs a `Product` entry matching
each deployed product's actual display name for per-product routing to
take effect; until then, every case routes to `DEFAULT_CHAT_PRODUCT`'s
space same as before this field was populated.

`caseTeamName(cv)` (same shared-helper pattern) resolves
`cv.AccountDetails.CreTeam.Name` (e.g. `"Team Nova"`, `""` when the case
has no account or the account has no CRE team) — used by the same three
publishers to populate their payloads' `Team` field, a purely-display
value in `csm-notification-service`'s Chat cards (unlike `Product`, it
plays no role in routing). `cv.AccountDetails` (and its `CreTeam`) is
resolved by `GetCaseByID` from the case's own embedded ServiceNow account
object at no extra request cost — but as of this field's introduction,
that embedded object's `creTeam`/`sreTeam` are documented in
`snCaseAccount`'s own doc comment as not yet guaranteed to be populated by
the ServiceNow integration, even though the standalone accounts endpoint
does return them. `Team` may therefore come back empty in practice until
that catches up — not a bug in this service if so.

**Known, accepted inconsistency**: `publishCaseAcknowledged` re-reads
`caseProductName(cv)` from a fresh `GetCaseByID` at acknowledge time,
rather than reusing whatever product `publishCaseCreated` read at create
time — so if a case's deployed product genuinely changes between creation
and acknowledgement, the two Chat alerts can route to different spaces.
This service has no persisted state for a case at all (ServiceNow is the
sole source of truth, no local DB row per case — the old `sla_clocks` table
used to be the one exception, removed; see "SLA status" below), so "preserving the
creation-time product" would mean adding new durable state purely to pin a
routing decision, not a same-service code change. It's also arguably not
even the more correct behavior: if a case's product association is
corrected after creation, routing its acknowledgement to the *current*
owning team's space is arguably more useful than a stale one. Left as
current-product routing; revisit only if the same-space guarantee turns
out to matter in practice.

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

**`work_item_activity` (migration 000056) now gets written to on the
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
ServiceNow or Event Hub round trip can't consume this service's own 30s
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

All four are **internal-caller-only** via `AccessService.ResolveScope`
(`AccessScope.Unrestricted`, i.e. `AUTH_INTERNAL_CLIENT_IDS`), exactly as the
onboarding-step endpoints are — anyone else gets 403 before anything
downstream is touched. A portal END USER must never call them directly:
whether this particular customer admin may invite this particular person into
this particular project is the portal backend's decision, made against the
account it has already scoped the session to. This service deliberately does
not re-derive that from a forwarded user token.

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
| `POST /projects/{id}/contacts` | `{email, firstName?, lastName?, roles: []}` | **201** + `ProjectMembership` | 400 bad address / unknown role / no roles, 403, 404 unknown project or no Salesforce account, 409 already an active contact, 503 |
| `PATCH /projects/{id}/contacts/{email}` | `{roles: []}` | **200** + `ProjectMembership` | 400, 403, 404, 503 |
| `DELETE /projects/{id}/contacts/{email}` | — | **204** | 400, 403, 404, 503 |
| `POST /projects/{id}/contacts/{email}/resend-invitation` | — | **204** | 400, 403, 404, 409 not INVITED, **429** inside the cooldown, 503 |

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
- **Change roles** replaces the Salesforce `Role__c` picklist and, with it,
  the membership's project groups. The state is untouched (the PATCH sends
  only `role`). An empty list is accepted and removes every group.
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

- `project_role_enum` **already carried `ADMIN`** (migration 000023 declared
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
  `customer`/`partner` exactly as before, and separately reports **which** of
  the two admin roles this contact would hold
  (`SalesforceMembershipUpsert.AdminRoleName`) — never whether they hold it.
- `syncDerivedAdminRole` (step 8 of the upsert, after the project groups are
  written) decides that, as **one query over the user's memberships** that
  answers for both managed roles at once:

  ```sql
  SELECT
    COALESCE(bool_or(ac.account_id  = p.account_id), FALSE),  -- earns customer_admin
    COALESCE(bool_or(ac.account_id <> p.account_id), FALSE)   -- earns partner_admin
  FROM "user" u
  JOIN account_contact ac ON LOWER(ac.user_name) = LOWER(u.user_name)
  JOIN project_contact pc ON pc.account_contact_id = ac.id
  JOIN project p ON p.id = pc.project_id
  JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
  JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
  JOIN project_role pr ON pr.id = pgr.project_role_id
  WHERE u.id = $1
    AND pr.role = 'ADMIN'::project_role_enum
    AND (pc.state IS NULL OR pc.state <> 'DEACTIVATED'::project_contact_state_enum)
  ```

  It binds only the user id — nothing about the membership in hand — which is
  precisely what makes the old failure impossible. Running it *after* step 7
  is what makes the membership being written count. A failed derivation aborts
  the write rather than quietly deciding "not an admin", which would revoke a
  real admin's role on a transient error.

  **Each role is decided on its own evidence**, and that split is the second
  bug fixed here. A membership is a partner one exactly when the contact's
  account is not the project's (`ac.account_id <> p.account_id`, the same test
  `membershipByEmail` applies), so only an ADMIN membership of that kind can
  support `partner_admin`, and only one of the other kind can support
  `customer_admin`. An earlier version asked a single "is this user an admin
  anywhere" question and then kept whichever role the membership in hand
  mapped to: processing a **non-admin partner membership** for someone who was
  a customer admin on their own account would grant them `partner_admin`, which
  no ADMIN membership supported, and revoke the `customer_admin` they had
  earned. The contact's own Salesforce `isCsAdmin` remains an additional grant
  of the role this membership maps to, never a revocation condition.

A deactivated membership's `ADMIN` role does not count, which is how
deactivating someone's last admin project drops their account-level role
without erasing anything.

### Account roles on the contacts search

`POST /projects/{id}/contacts/search` now returns each contact's
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
now reads SLA state **live from the `sla` table** (migration `000052`), which
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
`RESOLUTION`), not one clock for one case — `integrations/csm-notification-service`
polls this periodically and diffs `businessElapsedPercent` against what it
already alerted on itself (see that repo's own `internal/slaengine`), rather
than this service pushing individual tier-crossing notifications the way the
old design's Redis wake index did. This is a genuinely different shape from
every other paginated endpoint in this file: its one real caller is a
periodic bulk poll (~5,500 rows checked live), not a UI list a human scrolls
through, so it has its own pagination cap
(`normalizeSLAStatusPagination` — default `500`, max `2000`) well above the
generic `20`/`50` `normalizePagination` uses everywhere else; a low cap here
would only turn one intended round trip into over a hundred for no one's
benefit.

**`GET /sla-status` is internal-caller-only** (`slaStatusService.
requireInternalCaller`, mirroring `onboarding_step_service.go`'s own helper
of the same name/reasoning) — `AccessService.ResolveScope`'s scope must be
`Unrestricted` (an `AUTH_INTERNAL_CLIENT_IDS` client), refused with
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
breach firing days later. `team` is always empty on this data source: unlike
ServiceNow, nothing in this schema resolves a case to a team today.

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

`scheduled_task_run` (migration `000045`, `internal/domain/entity.go`'s
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

`comment` (migration 000037), `product_vulnerability` (migration 000034),
and `time_card`/`time_card_approver` (migration 000039) had tables from the
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
  `deployment` (migration 000013) is its own standalone table with its own
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
  be an eligible approver (a `time_card_approver` row, and not the card's
  own submitter) AND the card to currently be `submitted` — both checked
  under one `SELECT ... FOR UPDATE` so a concurrent approver-list edit or a
  second transition attempt can't slip through between the check and the
  write. `CreateTimeCard` validates a supplied `projectId` against the
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

- **Case tags** (`tag`/`work_item_tag`, migration 000021): `CaseService.
  AddCaseTag`/`RemoveCaseTag`/`SearchTags` in `case_service.go` were a
  detection-only stub that always returned 503 — see
  `detectPatchTagBillableOverride`'s own doc comment for that history — and
  now actually persist. `AddCaseTag` finds-or-creates a tag by name
  (case-insensitively; `tag.name` has no `UNIQUE` constraint, so a race
  between two first-uses of the same never-before-seen label can produce a
  cosmetic duplicate row, not a correctness bug) and attaches it to the
  case's underlying `work_item`, idempotently. The `detectPatchTagBillableOverride`
  "patch tag on a LOW-severity case" detection still only logs — condition
  (a) it was blocked on (case tags having real storage) is now true, but
  condition (b) (a consumer for `events.TypeCaseBillableStatusChanged`)
  still doesn't exist.
- **Case watch list** (`work_item_watcher`, migration 000040):
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
  can answer directly.

  **Every case now gets its account's four named stakeholders as watchers,
  unconditionally, from a pure Postgres lookup — no ServiceNow involved.**
  `account.customer_success_manager_id`/`technical_owner_id`/
  `secondary_technical_owner_id`/`account_manager_id` (migration 000008)
  are already `"user"` ids, so there's no email/UUID ambiguity to resolve
  at all. `createCaseSNFirst` calls `addAccountDefaultWatchers` right after
  `CreateCaseFromServiceNow` succeeds and before `publishCaseCreatedEvent`;
  it resolves those four ids for the case's project via
  `CaseRepository.AccountDefaultWatcherIDs` (a `project JOIN account`,
  whichever of the four are set, deduplicated) and writes them with the
  same `CaseRepository.SetCaseWatchList` the `UpdateCase` branch above
  already uses. A project with no linked account, or an account with none
  of the four roles set, is a normal state (an empty slice, `SetCaseWatchList`
  never called) — not an error. A repository failure here is logged, not
  returned: ServiceNow already has the case by this point, so a missing
  default watch list must not be reported as a failed create, same posture
  as every other post-ServiceNow-success step in this file (event
  publishing included). `req.WatchList` itself is unaffected by any of
  this — it's still forwarded to ServiceNow as part of the create request
  the normal way; this addition is purely about what the Postgres mirror
  also guarantees.
- **Account contacts** (`account_contact`, migration 000020) and **project
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
- **User roles** (`role`/`user_role`, migrations 000004/000006):
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
  `"user".email` has no unique constraint (migration 000001 only makes
  `user_name` UNIQUE) — so that join is wrapped in its own `DISTINCT ON
  (cm.id)` subquery to guarantee one activity row per comment even if two
  user rows share an address. Without it, a shared address would fan one
  comment out into multiple feed rows while the sibling `COUNT` query (which
  never joins `"user"`) still counted it once, so the page and its `total`
  would disagree.

**Pre-existing bug fixed as a side effect, not scope creep**: `user_repo.go`
queried a `users` table with `created_at`/`updated_at`/`phone`/`timezone`
columns that do not exist anywhere in `migrations/` — the real table is
`"user"` (migration 000001) with `created_on`/`updated_on` and no
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
assigned_to_id` (migration 000036) and `work_item.acknowledged_by_user_id`
(migration 000016) are both real, direct columns, already read elsewhere
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
  rejected outright; `work_item.parent_id` (migration 000036) is the same
  self-reference `GetCaseByID`'s own `ParentCase` already reads the other
  direction, so `updateCaseParent`/`CaseRepository.UpdateCaseParent` wire it
  up the same way `updateCaseAssignee` does.
- **Combinable** (any subset, freely combined with each other, never with
  the exclusive group): `subject`, `description`, `deploymentId`,
  `deployedProductId`, `bestCaseFixEta`/`mostLikelyFixEta`/`worstCaseFixEta`,
  `relatedCaseId`, `workaroundProvided` -- all newly wired up via
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
  `publicTicket` (the "Share Fix ETA" comment-posting side effect) and
  `autocloseHoldUntil` (no backing column anywhere in this schema) remain
  rejected too.

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

## Change requests

`change_request` (migration 000047) is a shared-PK extension of `work_item`,
same pattern as `"case"` (`change_request.id` IS `work_item.id`). `SearchChangeRequests`,
`AggregateChangeRequests`, `GetChangeRequest`, and `PatchChangeRequest` are
wired up to it (`change_request_repo.go`/`change_request_service.go`).
`changeRequestService.SearchChangeRequests` validates `req.SortBy` against
the same `validChangeRequestSortField`/`validChangeRequestSortOrder` maps
`sn_change_request_service.go` already used, so an unrecognized `sortBy`
value is a 400 on both data sources instead of silently falling back to
`created_on DESC` only on Postgres.

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
`CreateChangeRequest` and both approval methods (`GetChangeRequestApprovals`,
`DecideChangeRequestApproval`) are not, for two different reasons:

- **`CreateChangeRequest`**: `work_item.number` has no DB default and no
  backing sequence anywhere in `migrations/` — the exact same blocker
  `CaseRepository.CreateCase` has (see "Fixing the plural/singular
  table-name mismatch" below). Deferred for the same reason: generating it
  needs a product decision (sequence + migration vs. Go-side generation,
  and the exact number format) this change doesn't make unilaterally.
- **`GetChangeRequestApprovals`/`DecideChangeRequestApproval`**: these
  model multiple approval *stages*, each with multiple *approvers* and
  per-approver status (`domain.ChangeRequestApproval`/`ChangeRequestApprover`).
  This schema has only one summary `change_request.approval` column
  (`REQUESTED`/`APPROVED`/`REJECTED`/`NOT_REQUESTED`) — no approval-stage or
  approver table at all. There's nothing to serve either method from
  without a schema change, so both always return a `ServiceUnavailableError`
  on Postgres.

**`ServiceID`/`ServiceOfferingID` are now wired up** (migration 000050 added
`change_request.service_id`/`service_offering_id`, FKs into `service`/
`service_offering`, migrations 000048/000049): readable via
`SearchChangeRequestView.Service`/`ServiceOffering` and writable via
`PatchChangeRequestRequest.ServiceID`/`ServiceOfferingID`. `service`/
`service_offering` also got their own Postgres implementations
(`it_service_repo.go`/`service_offering_repo.go`) backing `POST /services/
search` and `POST /service-offerings/search`, previously ServiceNow-only.

**Fields still with no real column anywhere, left unset rather than
guessed at** (see `ChangeRequestRepository`'s own doc comment for the full
list): `ConfigurationItemID`, `GroupID`, and `AssignedTeamID` (no CMDB/group
tables exist in this schema at all); `Type`
(`domain.ChangeRequestType` — standard/normal/emergency/... — has **no**
relationship to `change_request.change_request_type`, whose real enum
values are `INFRA`/`GENERAL`, a completely different classification, not a
subset of the domain enum); `ApprovedBy`/`ApprovedOn`/`LegalNextStates` on
`domain.ChangeRequest` (no approver/date columns for the first two;
`LegalNextStates` is a ServiceNow workflow-engine computation with nothing
to derive it from here). `Duration` (`cr.calendar_duration`, an `INTERVAL`)
is also left unset — no confirmed display format to render it in.

**Linking happens entirely through `PATCH`, never at creation** —
`CreateChangeRequestRequest` has no project/case field at all;
`PatchChangeRequestRequest.ProjectID`/`DeploymentID`/`DeployedProductID`/
`AssignedEngineerID` map directly to their `work_item` columns, and
`CaseID` maps to `work_item.parent_id` (`domain.LinkedChangeRequestRef`'s
own doc comment already describes this as "the reverse of
`PatchChangeRequestRequest.CaseID`" — confirmed here as the generic
`work_item.parent_id` self-reference, migration 000036, not case-specific).
Because of this, `SearchChangeRequestView.Project`/`Case` can be empty
(`EntityRef{}`)/`nil` for a change request that exists but hasn't been
linked yet — a real, valid state for this schema, not a bug.

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
(both nullable, migration 000001) directly into `domain.User`'s required
(non-pointer) `FirstName`/`LastName` string fields — pgx v5 can't scan `NULL`
into a plain `*string` destination. `email` (also nullable on `"user"`) had
the same latent bug, not yet hit in production but certain to fail the same
way. Fixed by scanning all three into intermediate `*string` vars and
`stringOrEmpty(...)`-defaulting them, same pattern as every other nullable
column fix in this file.

**`user_type` had a casing/mapping bug on top of the same NULL-scan risk**:
`user_type_enum`'s real labels (migration 000007) are `SYSTEM`/`INTERNAL`/
`EXTERNAL`/`NOT_AVAILABLE`, scanned directly into `domain.UserType` (whose
values are lowercase `internal`/`customer`/`system`/`external`) with no
translation at all — never exercised before because `user_type` was
previously always `NULL` in practice or never appeared in a search result
that got fully inspected. `userTypeFromEnum` now maps `EXTERNAL` to
`UserTypeCustomer` specifically, not `UserTypeExternal` — see
`UserTypeExternal`'s own doc comment: "the postgres source emits customer,
ServiceNow emits external" for the same underlying concept (confirmed
against `recompute_user_type`'s trigger logic, migration 000007: `EXTERNAL`
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
    (`eng.type`, migration 000019) the same way, LEFT joined. `ParentCase`
    now resolves its `Type` from the parent's own real `work_item.type`
    (via `work_item.parent_id`, migration 000036 — a generic self-reference
    across every work_item type, not case-specific) instead of always
    hardcoding `"case"`; `RelatedCase` (`"case".related_case_id`, migration
    000038) is genuinely case-specific, so hardcoding `"case"` there is
    still correct. `account_id` is read directly off `work_item.account_id`
    (a real, direct column — migration 000016) rather than derived
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
    generic `comment` table (migration 000037, keyed by `work_item_id`, not
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
correctly. `project.start_date`/`end_date`/`account_id` (migration 000009)
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

**Still not usable, deliberately:** nothing generates `work_item.number`
(NOT NULL, unique) or `wso2_id`, so the insert is refused and the repository
returns `ServiceUnavailableError` ("case numbers are not generated") rather than
an opaque 500. Do not guess these -- the decision (a DB sequence + default vs
Go-side generation) is still open. What the data says, for whoever decides:
- `number` is `CS` + 7 digits in **one series shared by every work-item type**
  (cases, service requests, engagements, security reports, announcements), max
  `CS0442200` when checked. ServiceNow allocated them and the sync is still
  running, so a locally generated number can collide with a synced one. The
  leftover `cases_number_seq`/`cases_wso2_id_seq` sequences (both 63) are not
  attached to any column.
- `wso2_id` is `<project key>-<per-project counter>` (prefix equals
  `project.key` for 1,101 of 1,233 linked cases; the rest are renamed or
  malformed keys) and the counters have gaps.
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
  (000014), `case_attachment` (000043/000044), `announcement_requests` (000077),
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
`work_state`/`resolution_code`/`current_escalation_level`/`is_escalated`
remain `"case"`-only, since no other extension table has those columns.
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
against `team` (migration 000028) — "mirror[s] a hand-curated allow-list of
ServiceNow's OOB sys_user_group / sys_user_grmember tables" per that
migration's own comment, the same concept `GroupService` searches.
`domain.Group.Active` has no backing column and is hardcoded `true`;
`Parent` has no hierarchy column on `team` and is always `nil`.

**Not wired up**: `project_type` has no corresponding field anywhere on
`domain.Project`/`ProjectDetail` today, so there is nothing to populate
without first adding a new response field — left alone pending that
decision, not overlooked.

## IT services (CMDB services)

`service` (migration 000048) is a standalone table — no FK to or from any
other table in this schema. `ITServiceRepository.SearchITServices`
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
`CHECK` constraint (migration 000016) requires it `NOT NULL` for those
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
`CaseRepository.UpdateCase`'s `previousSeverity` return value too (used by
`caseService.detectBillableStatusChange` for the LOW-severity-boundary
check, which now treats a nil severity as "not LOW" on either side of the
comparison rather than crashing or silently comparing against `""`).

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
(migration 000052) respectively.

**`SearchIncidentActivities` reuses `scanCaseActivity`'s exact query shape**
(`case_repo.go`) -- an activity feed entry (comment or field change) is not
inherently case-specific, and `comment`/`work_item_activity` are both keyed
by the generic `work_item_id`. Unlike `SearchCaseActivities`, there is no
`case_attachment`-equivalent table for incidents, so this feed can never
have an `"attachment"` kind entry.

**`UpdateConversation` is implemented** (a plain `conversation.state` enum
write, no `work_item.number` generation needed for an update) but
**`CreateConversation`/`CreateProblem`/`CreateIncident` are not**: all three
need `work_item.number`, which has no DB default or backing sequence
anywhere in `migrations/` -- the same blocker `CaseRepository.CreateCase`
already has. **`UpdateProblem`/`UpdateIncident`/`HandOffIncidentToSpecialist`
are also not implemented**: `UpdateProblem.Transition` is validated
server-side by ServiceNow's own workflow engine with no fixed, confirmed
transition rule set to reimplement (see that field's own doc comment --
deliberately not a closed enum for exactly this reason);
`UpdateIncident` touches several fields with no backing column at all
(`AssignmentGroupID`, `ConfigurationItemID`, `WatchList`) alongside ones
that do, and would need `comment`-table side effects for
`AdditionalComments`/`WorkNotes` mirroring `caseService.UpdateCase`'s own
comment-on-update behavior -- deferred as a unit rather than
half-implemented; `HandOffIncidentToSpecialist` is an inherently
ServiceNow-workflow-specific feature (moves the incident to a specialist
group, opens a runbook-gap task, files a GitHub issue) with no
assignment-group or handoff-tracking concept anywhere in this schema to
derive an equivalent from. `IncidentView.SpecialistHandoff` is always `nil`
on this data source for the same reason -- the correct "never handed off"
representation per that field's own doc comment, not a gap.

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

## change_request.change_model and work_item_activity (migrations 000055/000056)

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

**Column names follow staging, not `migrations/`.** Staging's schema is built by
the sync service, and it renamed columns this code was written against:
`deployment_node.subscription_key` -> `project_key`, `deployment_node.deployment_ref`
-> `deployment_number`, `deployment_information.number_of_cores` -> `core_count`
and `reported_created_on/reported_updated_on` -> `payload_created_on/
payload_updated_on`, `daily_usage_summary.deployment_ref` -> `deployment_number`.
Migration 000054 still uses the old names. With the old names `SearchInstances`,
`SearchInstanceMetrics` and `SearchInstanceUsage` failed on staging with "column
does not exist". Check the live schema before trusting the migrations.

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

`service_offering` (migration 000049) is now Postgres-backed
(`service_offering_repo.go`): `POST /service-offerings/search`, previously
ServiceNow-only. `parent_id` (FK into `service`, migration 000048) maps to
`ServiceOffering.Service`; `SearchServiceOfferingsFilters.ServiceIDs` filters
on it.

`sla`/`sla_policy` (migrations 000051/000052) back `TaskSlaService`
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
  (migration 000026) and `project.project_type_id` (migration 000027) --
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
inventing data): `SystemMetadataResponse.TimeZones`/`FeedbackEmojis` (static
ServiceNow-side config, not project/case data); `SeverityBasedAllocationTime`
(no SLA-allocation-time table exists);
`ProjectFeatures.AcceptedSeverityValues` and every `Has*Access`/product-
category field (no per-project feature-entitlement or severity-restriction
columns exist anywhere in the Postgres schema -- checked directly against
the `project` table's full column list, not just assumed). (`CallRequestStates`
used to be on this list; `customer_call` -- migration 000072 -- has since
landed, so it's now read live from `customer_call_state_enum` like every other
choice list. See "Call requests and the service-request catalog" below.)

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
  `AUTH_INTERNAL_CLIENT_IDS` allow-list, never used as a capability grant
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
| `x-jwt-assertion` client id is in `AUTH_INTERNAL_CLIENT_IDS` | **everything, unconditionally** -- regardless of any `x-user-id-token` the same request also carries |
| not an internal client, user token, `user_type` INTERNAL (all active rows for the email) | everything |
| not an internal client, user token, EXTERNAL (customer) | only projects where their email is a `REGISTERED` `project_contact`, and the cases in them; none registered = an empty result, never "no filter" |
| not an internal client, user token, inactive / SYSTEM / NOT_AVAILABLE / unknown email | 403 |
| not an internal client, no user token | 401 -- no legitimate caller to resolve |

**An internal client id wins outright -- there is no comparison with the user
token's own scope.** Every client id configured here is itself an
already-trusted internal service (see `AUTH_INTERNAL_CLIENT_IDS` config
below), so a user token it forwards (if any) is used only for attribution
elsewhere (`created_by`/`updated_by`), never for scoping -- not even to widen
or narrow anything. This is simpler than an earlier revision of this design
(a "rescue" that only kicked in for an *unknown* forwarded email, deferring to
the user's own scope otherwise): once real deployments settled on which
callers are genuinely internal, there was no longer a case where an internal
client legitimately forwards a real customer's token, so the extra nuance was
removed. `AccessRepository` is never even queried on the internal-client path
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
`recompute_user_type()`'s trigger (migration 000007) classifies only the
`admin` and `internal` Asgardeo/SN roles as `user_type = INTERNAL` -- a person
whose only role is `agent` ends up `NOT_AVAILABLE` and is denied here even
though they *do* have a `user` row. Whether `agent` should count as internal
is a product decision, not something to guess at here.

**`AUTH_INTERNAL_CLIENT_IDS`** (config.go's `ParseInternalClientIDs`) is a
plain comma-separated set of client ids -- no `clientId=role` grammar, no
"delegate" role: those existed in an earlier revision, when a caller that
always forwards a user token needed a role distinct from one that sometimes
doesn't. In practice every caller either (a) is itself trusted with
unconditional access (an internal client id), or (b) is resolved purely from
whatever user token it forwards -- there's no third case, so a plain
allow-list is all `ResolveScope` needs. Which real client ids belong in it is
a deployment decision this file doesn't prescribe.

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
`x-user-id-token`) gets a 401 unless its client id is in
`AUTH_INTERNAL_CLIENT_IDS`. Before rolling this out, list every direct
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
escalations, time cards, attachments, conversations, change requests,
call requests, catalogs, instances, etc.) still does no per-caller scoping --
the auth middleware validates tokens on every route, but only the operations
above actually call `AccessService`. Extending it further is follow-up work,
not done in this pass.

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

**Per-caller scoping is not enforced here yet.** Every route's tokens are now
validated (see "Token validation and caller-scoped access" above), but call
requests aren't one of the operations `AccessService` is wired into (see that
section's "Not yet wired" list) -- a validated caller can read/write any
call request regardless of project access. The `x-user-id-token` is read
only to attribute writes (`created_by`/`updated_by`, `opened_by_id`), not to
authorize them. Extending `AccessService` here is the same follow-up work
called out there, not done in this pass.

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
- Search-all's `assignmentTeamIds` is rejected with a 400 rather than
  ignored: nothing on this schema holds a case's assignment team
  (`customer_call.assignment_group` was deliberately skipped in the
  migration), and a silently-ignored filter would widen the result set.
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
  `catalog_variable` has no columns for `readOnly`/`hidden`/`maxLength`/
  `referenceTable`/`validation`/`choices`, so those stay at their zero value
  (TODO: choice-based variables render as free text until a choices table
  exists). A NULL `is_active` counts as active.

## Case search filters on the Postgres data source

`caseRepo.SearchCases` implements `tag`, `projectOnboardingStatus` (in/notIn),
`taskSLABusinessElapsedPercent` (gte/lte), `escalationLevel`, `escalation`
(isEmpty/isNotEmpty), `parentId` (eq), and `anyOf`. The rest of the
ServiceNow-shaped filters are
still rejected with a 400 by `caseService.SearchCases` (`product`, `projectType`,
`creTeam`/`sreTeam`, `slaBreached`, `accountEscalationActive`, ...) because
dropping one would silently widen the result set. `creTeam`/`sreTeam` and
call-request `assignmentTeamIds` are blocked on data, not schema: the group
columns exist but staging's `group` table was empty (the sync has no job for the
full group source) so every group FK is NULL.

- **`parentId eq`** was accepted by `ParseCaseFieldFilters` (the customer/CSM
  portals' "Linked Items" tab sends it to find a case's child cases) but
  unconditionally rejected by `caseService.SearchCases` with "not supported
  by this data source", even though nothing about it is actually
  ServiceNow-specific: `work_item.parent_id` (migration 000036) is the exact
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

## Announcement requests

`announcement_requests` (migration `000040`, `internal/domain/entity.go`'s
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
  AND tm.team_id = ANY($n::uuid[]))` (migration 000028, the same table
  `GetUserGroups` reads).
- `groupNames` — the same `EXISTS` joined to `team` on `t.id = tm.team_id`,
  matching `t.name = ANY($n::text[])` instead of the id; kept alongside
  `groupIds` because callers' team registries are keyed by name (ids differ
  per environment and not every configured team has one).

`userService.SearchUsers` still validates `userIds`/`groupIds` as UUIDs
(`validateUUIDs`) before they reach the repository — only the "unsupported on
Postgres" rejection was removed, not the format check.

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

## GET /users/{id} on the Postgres data source

The route was registered only for ServiceNow, so opening a user in the CSM portal
on Postgres said "The requested resource was not found." It now returns
`domain.UserDetail`: the user (display name, `active`, type), `roles` (DISTINCT, see
above), `groups` (the teams from `team_member`, from which the BFF derives the
profile's team block) and, for customers only (`user_type` EXTERNAL, emitted as
`customer`), `projectAccess`.

- **It is a dedicated type, not `SNUserDetail`.** That type always sends `lockedOut`,
  `timeZone` and per-project `notificationsEnabled`, none of which this schema stores,
  and the page shows a "Locked out: No" chip whenever `lockedOut` is present, so
  reusing it would assert something unknowable. Those fields are omitted.
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

## POST /users creates a new "user" row (Postgres-only)

Before this, no code anywhere in this service wrote a `"user"` row at all — `UpsertFromSalesforce`
(`account_repo.go`) only ever touches `account`, and `project_membership_repo.go`'s own user upsert
only fires as a side effect of ingesting a Salesforce membership. `UserRepository.CreateUser`
(`user_repo.go`) is the first direct write path: `user_name` is always `lower(email)` (matching the
membership ingest's own convention), `is_active` is always `TRUE`, and `id`/`created_on`/
`updated_on` are supplied inline (`gen_random_uuid(), NOW(), NOW()`) since the column has no DB-side
default. `user_type` is never set directly — it's derived by a trigger (migration 000007) from
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

## SearchDeployments crashed on any page containing a NULL deployment.type

Reported live: `POST /deployments/search` failing with `cannot scan NULL into
*string`. `deployment.type` (migration 000013) has no `NOT NULL` constraint —
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

**Never put `pgErr.Detail` verbatim in a `ValidationError.Msg`.** `writeServiceError`'s own comment states a `ValidationError`'s message is always safe to return to the caller as-is, but a Postgres foreign-key violation's `Detail` field quotes the real table and column name (e.g. `` Key (assigned_to_id)=(...) is not present in table "user". ``) — handing an API caller schema internals. When a `23503` can be attributed to a specific request field (e.g. via `pgErr.ConstraintName`, since none of this schema's inline `REFERENCES` get an explicit `CONSTRAINT` name, so Postgres's default `<table>_<column>_fkey` naming applies), name that field instead. See `change_request_repo.go`'s `changeRequestPatchFKField` map for the pattern. Several older `23503` handlers elsewhere in `internal/repository/` (`case_repo.go`, `time_card_repo.go`) still return `pgErr.Detail` this way — a known pre-existing gap, not newly introduced, and not yet fixed.

## Database migrations

Migrations live in `migrations/` as plain SQL files, numbered `000NNN_<description>.up.sql` / `.down.sql`. Each migration creates its PostgreSQL enums, sequences, and tables in a single transaction. Apply them in ascending order before starting the service.

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

Configured in `internal/db/postgres.go`:

| Setting             | Value   |
|---------------------|---------|
| Max connections     | 20      |
| Min connections     | 2       |
| Max conn lifetime   | 30 min  |
| Max idle time       | 5 min   |

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
