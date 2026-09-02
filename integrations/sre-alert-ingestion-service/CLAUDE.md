# SRE Alert Ingestion Service

Go HTTP server (`net/http`, Go 1.26+) that ingests normalized alerts from
external monitoring tools and turns each into a platform incident via
`csm-integration-service`, buffering durably and escalating via Twilio when
CSM can't be reached. Read `README.md` first for the architecture diagram,
config table, and severity-mapping table — this file is the "why", not a
restatement of the "what".

## The one thing this service exists for

**Never be a single point of failure on the platform's own availability.**
Every design choice here traces back to that:

- `POST /alerts` persists to Postgres **before** any delivery attempt, and
  responds `202` based on that persistence succeeding — not on delivery
  succeeding. See `internal/handler.AlertHandler.CreateAlert`'s doc comment.
- The buffer is a **dedicated** Postgres database, never CSM's own. Sharing
  one would reintroduce the exact coupling this service removes.
- The escalation channel (Twilio voice call) is **independent of CSM** —
  reachable even when CSM itself is fully down.
- Retry state (`retry_count`, `last_attempt_at`) lives in Postgres, not
  in-process memory, so it survives this service's own restart or a
  regional failover — an in-process retry loop would not.

If a change makes any of these four properties weaker (e.g. "just cache the
retry state in memory for speed", "share the CSM database, it's simpler"),
it is undoing the reason this service exists. Don't.

## Why a 401 is retryable — read this before touching `isRetryable`

`csm-integration-service`'s `POST /incidents` currently returns 401 on
**every single call**, unconditionally — not intermittently, not under some
conditions. Reason: it's M2M-only, and the entity-service operation it
proxies is ServiceNow-backed, requiring a forwarded end-user identity token
this stack has no mechanism to supply yet. This is a confirmed, deliberate,
already-decided state (see `csm-integration-service`'s own `CLAUDE.md`,
"This service is M2M-only") — not something to "fix" from this side.

`internal/worker.isRetryable` (`internal/worker/worker.go`) therefore treats
401 as retryable — the opposite of how a 401 normally reads ("don't retry,
the caller isn't authorized"). Here it means "CSM cannot currently accept
this incident through this path", which is exactly the
CSM-unavailability condition this whole service buffers and retries
through. The only status this service treats as a **terminal**,
non-retryable failure is **400** (`apierror.Error.StatusCode ==
http.StatusBadRequest`): the payload itself is invalid, and retrying the
same payload can never succeed. Do not add a special case for 401 that
skips retry — every buffered alert currently ends up escalated via Twilio
specifically because this classification is correct; changing it silently
drops every alert this service ever ingests instead.

When the missing end-user-identity infrastructure lands upstream and
`POST /incidents` starts succeeding, this service's behavior should need
**zero changes** — alerts that used to ride out the full retry window
before escalating should now just succeed sooner. If you find yourself
wanting to change `isRetryable` once that infrastructure exists, that's a
sign the classification was already correct and nothing needs to move.

## Dedup: a failed create does not mean nothing was created

A failed `POST /incidents` proves nothing about whether the incident exists
upstream — the request can succeed on the far side while the response is
lost to a timeout or connection reset. `internal/worker.attempt` therefore
never blindly retries: for any row with `RetryCount > 0` (never the first
attempt — nothing could exist yet on attempt 1) it first calls
`csmclient.Client.SearchIncidentByTag` looking for
`csmclient.DedupTag(row.ID)` (`"[alert:<row-id>]"`), which
`internal/handler.buildSubject` already stamped as the leading text of this
row's own `CreateIncidentRequest.Subject` back when the row was first
buffered — `row.ID` is stable for the row's lifetime, so it's always the
right tag to search for, no re-parsing needed. A match short-circuits
straight to `MarkDelivered` against the found incident; `POST /incidents`
is not called again.

The id itself is generated client-side by `internal/idgen`, in
`internal/handler.CreateAlert`, *before* `store.Enqueue` — not left to
`alert_buffer.id`'s `gen_random_uuid()` column default — because the tag has
to already be inside the JSON payload being persisted, not bolted on after
the fact. Don't move id generation back into the store: that reintroduces
the exact chicken-and-egg problem this was built to avoid.

The search call fails open: no match, or the search call itself erroring
(including the 401 it also currently always gets, for the identical
missing-end-user-identity reason as `CreateIncident` — see the section
above), both fall through to attempting `POST /incidents` as normal. Do not
change this to fail closed ("couldn't confirm, so don't retry") — that
would turn an availability safeguard into a new way to silently drop a
buffered alert, which is the one thing this entire service exists to
prevent. In today's state (search also 401s), this makes the dedup check
**not yet actually effective in production** — every retry hits "search
401'd, proceed to create anyway" — but it is structurally correct and
requires zero changes once the identity gap closes.

## Persist-first is not an optimization

`internal/handler.AlertHandler.CreateAlert` does the full alert→incident
mapping (`MapToIncident`) and marshals the result **before** calling
`store.Enqueue`, then returns `202` purely on `Enqueue` succeeding — it
never calls `csm-integration-service` inline. This ordering is what makes
"a buffered alert survives this service's own crash" actually true
regardless of when in the request lifecycle the crash happens. Do not add
an inline delivery attempt to `CreateAlert` "for lower latency to first
attempt" — that's what `SRE_ALERT_POLL_INTERVAL_SECONDS` is for; a crash
between an inline attempt and persisting would silently lose the alert.

## Backoff-due filtering happens in Go, not SQL

`internal/store.PostgresStore.PendingBatch` returns *every* `pending` row
(bounded by `limit`), not just the ones currently due for retry — the
due-or-not decision (`internal/backoff.Due`) happens afterward, in
`internal/worker.RunOnce`, purely in Go against each row's
`RetryCount`/`LastAttemptAt`. This is deliberate: it keeps `internal/backoff`
pure and unit-testable with zero database dependency (see
`internal/backoff/backoff_test.go`), and keeps `internal/worker`'s tests
(`internal/worker/worker_test.go`) fully mockable — no query-shape coupling
between the backoff math and the SQL. If the buffer ever grows large enough
that scanning every pending row per tick becomes a real cost, push the
due-ness filter into the `PendingBatch` SQL query — but that's a
performance change to make deliberately, with a reason, not a default.

## Middleware chain

`SecurityHeaders → CorrelationID → Logger → Mux` — identical shape to
`csm-integration-service`, with one difference: the correlation-ID prefix
is `sais-` (SRE Alert Ingestion Service), not `cis-`. `internal/worker`
also uses the correlation-ID machinery outside any HTTP request: each
delivery attempt gets its own fresh `sais-`-prefixed ID via
`middleware.WithCorrelationID(ctx, middleware.NewCorrelationID())` before
calling `csmclient`/logging, since a retry happening minutes or hours after
the original `POST /alerts` request completed is a distinct traceable
event, not a continuation of that request's own ID.

## Why this service has its own copies of apierror/middleware, not shared packages

Go modules in this repo don't share `internal/` packages across services —
confirmed by `csm-integration-service` and `acp-closure-service` each having
their own `internal/apierror`, `internal/entity` (their respective upstream
clients), etc., despite calling the same upstream in `acp-closure-service`'s
case. This service follows the same convention: `internal/apierror` and
`internal/middleware` here are this service's own copies (adapted, not
byte-identical — e.g. the correlation prefix, and `middleware.WithCorrelationID`/
`NewCorrelationID` which the reference services don't need since they have
no background worker), not an attempt to import another service's
`internal/` package (which Go's `internal/` visibility rule would block
across module boundaries anyway, even if this repo's structure allowed it).

## `internal/csmclient` calls csm-integration-service, never entity-service directly

Same choice `acp-closure-service` already made for the same reason (see its
own `CLAUDE.md`, "Calls csm-integration-service, not entity-service
directly"): `csm-integration-service` is the service actually fronted for
M2M/third-party consumers, and its `POST /incidents` endpoint is the
documented, stable contract point for exactly this use case. Do not add a
direct entity-service client here without revisiting that decision.

## Twilio client: plain `net/http`, not the official SDK

This service's `internal/notifications/twilio.go` is a **self-contained copy**, not a shared
package or a runtime dependency: no code in this service imports, calls, or is deployed
alongside `csm-notification-service`, and this service's `go.mod` has no reference to it (see
"Deployment isolation" below). The implementation style (`MakeCall`/`Escalate`, Basic Auth,
form-encoded POST, TwiML built via `encoding/xml` for safe escaping, plain `net/http` instead
of Twilio's official Go SDK) was originally written for
`integrations/csm-notification-service/internal/notifications/twilio.go` and copied here,
trimmed to `MakeCall`/`Escalate` only (no SMS need in this service) — credited for provenance,
not wired as a dependency. Diverge from it freely if this service's needs change; there is no
coupling to keep in sync.

## Deployment isolation

This service is deployed and scaled independently of every other CSM component, by design —
its entire purpose is to keep working when the rest of the CSM platform doesn't. Its only
runtime dependency on anything CSM-owned is an HTTP call to `csm-integration-service`'s
`POST /incidents` / `POST /incidents/search` (see "Why `csm-integration-service`, not
`entity-service`, directly" above). It does not import any other service's Go module, is not
deployed alongside any other component, and its buffer database is its own dedicated Postgres
instance, never shared with the CSM platform's database. Do not add an import of, or a
deploy-time dependency on, any other `cs-tools` component without revisiting this decision.

## Postgres driver: `jackc/pgx/v5` via `database/sql`, matching repo convention

Confirmed by checking `entity-service/go.mod` and
`integrations/sftpgo-authentication-service/go.mod` (both `jackc/pgx/v5`)
before choosing — `lib/pq` was not independently evaluated, since matching
an existing repo-wide convention was the higher-value call here.
`sftpgo-authentication-service/internal/service/database.go` is the
reference for the exact wiring: `sql.Open("pgx", dsn)` via the
`github.com/jackc/pgx/v5/stdlib` blank-imported driver, not `pgxpool`
directly.

## Migrations: plain SQL files, applied via `psql`, no migration-runner dependency

Matches `entity-service/migrations` and
`integrations/sftpgo-authentication-service/db/migrations`: numbered
`NNNN_description.up.sql`/`.down.sql` pairs, no `golang-migrate` or similar
framework dependency, applied by the operator (or deploy tooling) via
`psql -f`, not from `cmd/server/main.go` at startup. `NewPostgresStore`
deliberately does not run migrations itself — see its doc comment.

## Go toolchain note

`go.mod` declares `go 1.26.0`, matching this repo's other services
(`entity-service`, `csm-integration-service`, `acp-closure-service`). If
your local `go` binary is older, `GOTOOLCHAIN=auto` (Go's own built-in
toolchain-management feature) transparently downloads and uses 1.26 — no
need to lower the `go.mod` version to match an older local install.

## Vendor neutrality

This service's own contract (`openapi.yaml`, `AlertRequest`,
`internal/severity`) must stay vendor-agnostic — no ServiceNow vocabulary,
per this repo's `cs-tools`-wide rule. `AlertRequest.Source` values
(`azure`, `site24x7`, `sentinel`, ...) name the *external alerting tools*
this service ingests from, which is a different thing from CSM's own
ServiceNow-vs-Postgres data-source distinction — this file's mapping tables
never mention ServiceNow, and neither should any future addition to them.
