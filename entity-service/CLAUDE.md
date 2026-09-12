# Entity Service

Go HTTP server (`net/http`, standard library only) that owns all core CS-platform entities: users, accounts, projects, products, deployments, deployed products, cases, and case comments. It exposes a REST API consumed by portal BFFs and other internal services.

## Architecture

Strict four-layer stack — no shortcuts across layers:

```
Handler → Service → Repository → PostgreSQL (pgx/v5)
```

All wiring happens explicitly in `internal/server/routes.go` (no DI framework). The full dependency graph is built there: `NewRepository(db) → NewService(repo) → NewHandler(svc)`, then registered on a `net/http.ServeMux`.

Middleware chain wraps the mux: **CorrelationID → Recovery → Logger → UserIDToken → Timeout** (10 s per request).

`CorrelationID` reads the `X-CSM-Correlation-ID` request header forwarded by the portal BFF, or generates a UUID v4 if absent. The ID is stored in the request context and echoed in the response header. All access log lines and panic logs include the correlation ID for end-to-end request tracing.

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

\* `DB_USER`/`DB_PASSWORD`/`DB_NAME` are required when `DATA_SOURCE=postgres`
and **optional** when `DATA_SOURCE=servicenow`, where entity reads and writes
go to the SN integration service instead. They are all-or-nothing in both
modes — `Config.Validate` rejects a partial set, so a typo can't silently
disable the Postgres-only endpoints. With `DATA_SOURCE=servicenow` and no
database, `Config.HasDatabase` is false, `cmd/api/main.go` opens no pool, and
`NewRouter` skips registering the two Postgres-only feature sets
(`/event-publish-failures*`, `/cases/{caseId}/sla-clocks*`), which then 404.
A failed Event Hub publish is logged instead of recorded — see
`EventPublisherService.Publish`'s nil-`failures` branch.
| `SERVER_PORT` | no       | `8080`  | HTTP listen port           |
| `EVENT_HUB_BROKER` | no | — | Kafka-compatible bootstrap address; feature-gates `EventPublisherService` (see "Event Hub publishing" below) |
| `EVENT_HUB_CONNECTION_STRING` | no* | — | Event Hub namespace Shared Access Policy connection string. *Required once `EVENT_HUB_BROKER` is set |
| `EVENT_HUB_TOPIC` | no* | — | Event Hub (Kafka topic) name. *Required once `EVENT_HUB_BROKER` is set |
| `EVENT_PUBLISHING_ENABLED` | no | `false` | Must be `"true"` for `EventPublisherService` to actually get constructed, even with `EVENT_HUB_BROKER` fully configured — a separate safe-by-default kill switch |
| `SUPPORT_ENGINEER_ROLE` | no | — | ServiceNow role name whose presence on a case comment's resolved author completes the case's "response" SLA clock — see "SLA clocks" below |
| `CUSTOMER_ROLES` | no | — | Comma-separated ServiceNow role names whose presence on a case comment's resolved author marks a customer reply — see `applyCustomerReplyStateTransition` in "SLA clocks" below |

`CSM_TEAM_REGISTRY` and `CSM_USER_ROLES` are **not read here**. The team registry
and the assignable-role allow-list are organisation vocabulary and live in the CSM
portal backend (`apps/csm-portal/backend`), resolved once at startup. This service
holds no organisation vocabulary at all — do not reintroduce it.

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
  empty `recipients` list.
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
sole source of truth, no local DB row per case — see this file's own
"SLA clocks" section for the one deliberate exception), so "preserving the
creation-time product" would mean adding new durable state purely to pin a
routing decision, not a same-service code change. It's also arguably not
even the more correct behavior: if a case's product association is
corrected after creation, routing its acknowledgement to the *current*
owning team's space is arguably more useful than a stale one. Left as
current-product routing; revisit only if the same-space guarantee turns
out to matter in practice.

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

## SLA clocks

`sla_clocks` (migration `000011`, `internal/domain/entity.go`'s `SLAClock`,
`internal/repository/sla_clock_repo.go`, `internal/service/sla_clock_service.go`)
is durable per-case SLA timer state — `caseId`/`clockType`, `startedAt`/`dueAt`,
and up to three tier-crossing timestamps. Like `event_publish_failures`, it has
no ServiceNow equivalent and is always backed by Postgres regardless of
`DATA_SOURCE`, so `caseId` is a plain string, not a foreign key — a
ServiceNow-backed case has no local `cases` row to reference.

`clockType` is deliberately **not** a fixed enum, but only three values are
actually used: `response`, `workaround`, `resolution` — see
`internal/service/sla_policy.go`'s `slaDurations`, which maps a case's raw
severity to each applicable clock's duration per WSO2's own
[support policy](https://wso2.com/licenses/support-policy/6.0) (Enterprise
plan). `LOW` severity's entry has only `response` — the policy defines no
fixed Workaround/Resolution SLA at that tier ("best efforts"), so those two
clocks are never registered for a `LOW`-severity case at all. `slaDurations`
also has a small note-worthy exception: `MEDIUM`'s `resolution` duration
approximates the policy's "1 Business Week" as a flat 7 days, then
`csm-notification-service`'s slaengine (which computes the actual due
timestamp, not this service) rolls that forward off a weekend if it would
otherwise land on one — see `slaAvoidWeekendClockTypes` and
`events.SLAClockRegisterPayload.AvoidWeekendDueDate`. Registering a clock
that already exists for a `(caseId, clockType)` pair resets it from scratch
(`RegisterSLAClock`) rather than adjusting it in place — including its
eight display-only fields (case number/WSO2 case id/title/type/product/
team/priority/state, a point-in-time snapshot from registration, added in
migration `000014`), populated so `csm-notification-service`'s slaengine
can build a Google Chat breach card from one `GetClock` call with no second
lookup — this service is the only thing with case data to give it.

Exposed at `POST /cases/{caseId}/sla-clocks` (register/reset),
`GET /cases/{caseId}/sla-clocks/{clockType}` (read one), and
`PATCH /cases/{caseId}/sla-clocks/{clockType}/tiers/{tier}` with body
`{"status": "reached"}` (`domain.SLATierStatus` — the only valid value today,
modeled as an enum rather than a bare boolean so a future status doesn't
need a breaking change) to mark a `50`/`75`/`100` tier reached, idempotently
— a second call for an already-reached tier returns the original timestamp,
not an error, plus `alreadyReached: true` so the caller can tell the two
cases apart (the underlying `UPDATE ... WHERE ... IS NULL` already decides
atomically which caller "really" set it; `alreadyReached` is just that
outcome surfaced instead of discarded). `SLAClockService.Pause`/`.Resume`
(set/clear `pausedAt`) have **no HTTP route at all** — `sn_case_service.go`
is their only caller, in-process (see below), so an HTTP surface for them
would be pure speculative API surface nothing outside this service needs.

**`alreadyReached` reflects the database claim only, not whether any
caller's own reaction (e.g. publishing a notification) to winning that
claim ever actually succeeded.** Gating a reaction on it being false is a
real, valid choice when duplicate-free behavior matters more than
guaranteed delivery — `csm-notification-service`'s
`internal/slaengine.Engine` does exactly this (see that repo's `CLAUDE.md`
for its full reasoning: it accepts the rare risk of a lost notification on
an Event Hub publish failure in exchange for not duplicate-publishing every
time a stale wake entry gets rediscovered, e.g. after a Redis outage) — but
it is a trade-off, not a free win: a caller whose reaction failed after it
won the claim will see `alreadyReached=true` on retry and skip the
reaction forever, having never completed it once. Only gate a reaction on
this field if that risk is acceptable for the use case, or if the
reaction's own completion is tracked separately and durably instead. The
sole caller today is
`csm-notification-service`'s SLA timer engine
(`internal/slaengine`), which owns the actual scheduling (a Redis wake index
and ticker) — this service only stores the result of that scheduling, it does
not compute or track wake times itself.

`sn_case_service.go` is the only caller today (Postgres-backed cases have no
SLA tracking — this is ServiceNow-only, same as most of this file):

- **`CreateCase`** publishes `sla.clock.register` (`publishSLAClockRegister`,
  folded directly into the existing `publishCaseCreated` — sharing its
  `GetCaseByID` fetch and its `s.publisher == nil` guard, since registration
  is inherently Kafka-based too) for every clock `slaDurations` has an entry
  for at the case's severity. Deliberately runs **before**
  `publishCaseCreated`'s own watch-list check: that check only gates the
  `case.created` *email*, and SLA tracking must happen regardless of
  whether the case has watchers.
- **`CreateCaseComment`** calls `applyResponseSLAOnComment` for every
  customer-visible comment (`req.Type == domain.CommentTypeComment` — work
  notes/activity entries don't count). This service has no auth/identity
  layer of its own (the `x-user-id-token` it forwards is opaque), so
  "is this comment's author a support engineer" is answered by resolving
  the comment's author (`resolveCommentAuthor`, the same lookup
  `publishCommentAdded` already needs for its own display name) and
  checking their ServiceNow role via `SNUserService.SearchUsers` filtered
  by email, against the **configurable** `SUPPORT_ENGINEER_ROLE` env var
  (deliberately no committed default — organisation-specific vocabulary,
  same reasoning `apps/csm-portal/backend`'s own `CSM_TEAM_REGISTRY` uses).
  A match claims all three tiers (`50`/`75`/`100`) of the `response` clock
  at once via `SetSLAClockTierReached` — claiming all three, not just
  `100`, is what suppresses a later spurious breach alert: when
  `csm-notification-service`'s slaengine eventually reaches the wake-index
  entries this clock's registration created, its own `SetTierReachedIfUnset`
  call sees each already claimed and quietly drops the wake entry instead
  of publishing a breach.
- **`UpdateCase`** calls `applyCaseStateSLAEffects` after every genuine
  state-changing PATCH, using the new state alone (no "old state" needed —
  see that function's own doc comment for why). `Awaiting Info`/
  `Solution Proposed` pauses `workaround`+`resolution`; any other state
  resumes both; `Closed` completes `resolution` the same
  claim-all-three-tiers way as `response` above, and pauses (not
  completes) `workaround` — **`workaround` has no completion trigger wired
  up at all yet** (see the `// TODO` in `applyCaseStateSLAEffects`; it
  needs a "workaround provided" signal this domain model doesn't have).

All three are deliberately **independent of `s.publisher`** except
registration itself (inherently Kafka-based) — pause/resume/completion are
pure in-process DB writes via `SLAClockService`, so a deployment without
Event Hub configured must not lose them as a side effect of that.

**`CreateCaseComment`** also calls `applyCustomerReplyStateTransition` —
not itself an SLA-clock write, but it triggers one indirectly. When a
customer-visible comment arrives while the case is `Awaiting Info`/
`Solution Proposed`, from an author holding one of the configurable
`CUSTOMER_ROLES` (same role-lookup mechanism as `applyResponseSLAOnComment`,
just checked against a list instead of a single role — an organisation can
have more than one customer-facing role), this calls `s.UpdateCase` with
`State: WaitingOnWSO2` **in-process**, not a second, separate ServiceNow
PATCH — reusing `UpdateCase`'s own `publishStatusChanged` and
`applyCaseStateSLAEffects` calls entirely rather than duplicating either.
`applyCaseStateSLAEffects`'s `default` case (any state other than
`AwaitingInfo`/`SolutionProposed`/`Closed`) is exactly the resume behavior
this needs, so no new SLA-specific code was needed for that part at all.
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

`scheduled_task_run` (migration `000013`, `internal/domain/entity.go`'s
`ScheduledTaskRun`, `internal/repository/scheduled_task_run_repo.go`,
`internal/service/scheduled_task_run_service.go`) is durable claim/retry
state for `operations/csm-scheduled-tasks` — a single Choreo Scheduled Task
that internally fans out to any number of independently-scheduled sub-crons
on one shared driver cadence. Like `sla_clocks`/`event_publish_failures`, it
has no ServiceNow equivalent and is always backed by Postgres regardless of
`DATA_SOURCE`. It is also the one intentionally **singular** table name in
this schema — every other table here is plural; don't "fix" it to match.

`taskName` is a caller-defined registry key, not a fixed enum — same
reasoning as `sla_clocks.clockType`: which sub-crons exist, and on what
schedule, is a policy decision made entirely by `operations/csm-scheduled-tasks`'
own registry, not something this service tracks.

There is no stored status column, the same choice `sla_clocks` makes for the
same reason: status is always derivable from which timestamp is set, and
each is independently useful on its own — `succeededOn` (done, forever, for
this period), `supersededOn` (abandoned: the next period came due before
this one ever succeeded), or `nextRetryOn` (eligible for another attempt
once it's in the past). See `operations/csm-scheduled-tasks`'s own
`CLAUDE.md` for the full design behind "period keys" and "supersede" — this
service only stores the result of that design, it does not compute period
keys or decide backoff itself, the same division of labor as `sla_clocks`.

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

## Database migrations

Migrations live in `migrations/` as plain SQL files, numbered `000NNN_<description>.up.sql` / `.down.sql`. Each migration creates its PostgreSQL enums, sequences, and tables in a single transaction. Apply them in ascending order before starting the service.

Key conventions enforced at the DB level:
- Primary keys are `UUID DEFAULT gen_random_uuid()`
- Human-readable IDs (e.g. `CASE-001`, `WSO2-001`) are generated from dedicated sequences via column defaults
- Enum types (e.g. `case_state_enum`, `case_priority_enum`) enforce valid values at the DB level; Go enum validation in the service layer is an additional guard
- Triggers enforce relational constraints that foreign keys alone cannot express (e.g. deployment must belong to the same project as the case)

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
