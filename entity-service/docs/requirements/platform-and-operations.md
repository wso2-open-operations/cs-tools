# Platform and operations — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 12.

**Audience:** product owners, CS leads, QA, operators and engineers new to the service.

This document covers what every feature relies on: how callers are identified and what they
may see, the conventions every request and response follows, health checks, the settings an
operator must understand, database changes, and the security rules. Settings that belong to one
feature (reports, GitHub, escalation contacts) are described with that feature; the README holds
the full list.

Related: [access-control.md](access-control.md), [event-publishing.md](event-publishing.md),
[salesforce-ingest.md](salesforce-ingest.md),
[data-sources-and-scheduled-tasks.md](data-sources-and-scheduled-tasks.md).

---

## 1. Purpose

The entity service is the one place that owns CSM data (cases, projects, accounts, users and
more) and the rules around it. The CSM portal, the customer portal and other internal services
all go through it. It sits behind an API gateway and never handles anyone's password. This
document says what it guarantees to those callers and to the people who run it.

## 2. Start-up and shutdown

**PLT-010** The service shall read its settings from the environment (and from a local `.env`
file if one exists), check them all at start-up, and refuse to start, naming the problem, on the
first error. It shall never start half-configured. A malformed `.env` file also stops start-up.

**PLT-011** The checks (section 9) include: the two ports are valid and different; the data
source is one of the three allowed values; all-or-nothing groups are complete; required
authentication settings are present; and the main durations are positive.

**PLT-012** When a database is configured the service shall connect and confirm it answers
(within ten seconds) before starting, and refuse to start if it does not. Without database
settings it shall start with no database (legacy mode only, see
data-sources-and-scheduled-tasks.md).

**PLT-013** The service shall load the identity provider's signing keys at start-up and refuse to
start if none can be loaded. A wrong or unreachable key address would otherwise leave a
deployment up while quietly rejecting every token.

**PLT-014** The service shall **not** change the database at start-up. Database changes are
applied separately before a release (section 10).

**PLT-015** The service shall run two listeners: the full API on one port and the health checks
on another (section 8). Both are started together; if the health listener cannot start, the
service shall stop rather than run without it.

**PLT-016** On a stop request (interrupt or terminate) the service shall stop the health listener
first, so monitoring sees it leave rotation before in-flight requests are drained, then finish
in-flight API requests (up to ten seconds), then stop its background jobs, and then close its
connections to the message hub, so a notice in flight is not handed a connection that has gone.

## 3. Identifying the caller

Two tokens can arrive on a request, both set by the gateway or the calling portal.

**PLT-020** **The user's token.** When a portal acts for a person it forwards that person's ID
token. The service shall verify its signature, issuer, expiry (with a small allowed clock
difference, 30 seconds by default), that its audience is one of the accepted application ids, and
that it has an email. Only strong signature algorithms shall be accepted, so a token cannot be
"signed" using a public key as a secret. Verification is always on; there is no setting to turn
it off.
*Check:* send a token signed with the wrong key; it is refused as invalid or expired.

**PLT-021** A user token that is **present but invalid** shall always be refused as
unauthorised, on every route. It shall never be treated as "no token", which would turn a forged
token into an anonymous call.

**PLT-022** **The calling application's token.** Every service-to-service call carries the
application's own assertion, from which the service reads only the application's client id. The
assertion is decoded but its signature is **not** checked here, because the gateway has already
authenticated the caller. It shall be refused only when it cannot be decoded or carries no client
id. (**Confirm**, see 12.1.)

**PLT-023** The user token's roles claim shall be read as the person's portal roles, as one value
or a list; an unexpected shape reads as no roles and shall never make an otherwise valid token
fail.

**PLT-024** A request with **no** tokens shall pass this step. Whether that is acceptable is decided
by each endpoint when it works out what the caller may see (section 4). (**Confirm**, see 12.1.)

**PLT-025** The caller's id (the person's stable user id, else the application's client id, else a
dash) shall appear in the access log so a request can be traced across services. An id from a
token that failed to decode shall never be logged as if it were real.

## 4. What a caller may see

The decision is made from the caller's verified identity, never from anything the caller sends.

| The caller is | They see |
|---|---|
| The customer portal's application | Always only what the **forwarded user** may see; a user token is required. This application is never trusted outright, even if its id is also listed as trusted. |
| The CSM portal's application, with a forwarded user whose email is in the internal domain | Everything. |
| The CSM portal's application, with no user or one outside the internal domain | Refused (403). |
| A listed machine-to-machine application | Everything, whatever user token it also forwards (used only to record who). |
| A person with an internal user type (all their active records internal) | Everything. |
| A person with an external (customer) type | Only projects where they are a **registered** project contact, and the cases in them; none registered means an empty result, never "no filter". |
| Any other person: inactive, system, unknown, or mixed in a way that is not clearly internal | Refused (403). |
| No user token and not a listed application | Refused (401). |

**PLT-030** The service shall apply the table above in this order of precedence: customer-portal
application first; then CSM-portal application; then listed machine applications; then the
person's own user type.

**PLT-031** When one email has several user records, the service shall combine them
conservatively: internal access needs **every** active record to be internal; an email that is
also an external customer is treated as a customer. Less access, never more, when the data is
ambiguous. A record that is not marked inactive counts as active.

**PLT-032** A project or case outside the caller's scope shall be reported as **not found**,
indistinguishable from one that does not exist, never as forbidden, so nobody can learn that
another customer's record exists. Where a write is refused by the database on a protected
record, the refusal is reported the same way.

**PLT-033** An action reserved for internal callers shall be refused as forbidden to anyone else
(an allow-listed internal application, or a person whose active records are all internal). An
action reserved for a **service** shall be refused to any person, even an internal one.

**PLT-034** The customer portal's application id, the CSM portal's application id and the
listed machine-application ids are deployment settings. The CSM portal id and its internal email
domain shall be set together or not at all, and the two portal ids shall never be the same value.
A portal id that is also listed as a machine application only logs a warning, because the
precedence above already prevents it gaining anything.

**PLT-035** An identity that cannot be resolved shall be reported as the identity layer reports it
(unauthorised or unavailable), never downgraded to "forbidden".

## 5. Request handling

**PLT-040** Every request body shall be limited to 1 MiB. A larger body is refused as a bad
request, naming the limit. Endpoints that legitimately carry files (attachments) have their own
larger, stated limits.

**PLT-041** A request body shall be exactly one JSON object. Unknown fields shall be refused, as
shall a second object after the first, malformed JSON (with the position), and a value of the
wrong type (named in plain words, without internal type names).
*Check:* send an extra field; the request is refused rather than the field silently ignored.

**PLT-042** Every request shall carry a **correlation id**. The service shall use the one the
caller sends (the CSM correlation header) or generate one, record it in the logs, and return it in
the response header so a person can quote it in a support request.

**PLT-043** Every request shall have a time limit (60 seconds by default). When it runs out, work
in progress is cancelled, and where that surfaces as an error the caller is told the request
timed out (408). The server's own read and write limits are 60 seconds by default, its idle limit
60 seconds, and calls to upstream services 60 seconds.

**PLT-044** A crash (panic) while handling a request shall be caught, logged with the correlation
id, and answered as a generic server error, so one bad request cannot stop the service. If the
reply had already started, no second error body is written.

**PLT-045** The access log shall record, for each request: method, path (with line breaks
removed), correlation id, caller id, status and time taken. It shall not record query strings,
request bodies or tokens.

## 6. Responses and errors

**PLT-050** Every search shall be paged. A request with no limit, or a zero or negative one,
shall use the default of **20** (users: **10**). A limit above **50** shall be refused with the
message "limit cannot exceed 50". A negative offset is treated as zero.

**PLT-051** Every search response shall report its total as `total` (never another name), with the
limit and offset used, and, for some searches, whether there are more results.

**PLT-052** Every error shall be JSON with a numeric `code` and a human `message`. Some refusals
that a program must tell apart also carry a stable `errorCode`; programs shall branch on that,
never on the wording.

**PLT-053** Failures shall map to status codes as follows:

| Situation | Status |
|---|---|
| The caller's input is invalid | 400 |
| The caller is not identified | 401 |
| Identified but not allowed | 403 |
| No such record (or not visible to the caller) | 404 |
| The request timed out | 408 |
| Conflicts with the record's current state | 409 |
| Removed on purpose | 410 |
| Repeating an action inside its waiting period | 429 |
| A dependency refused with a reason the service can safely pass on | 500, with that reason |
| A dependency is temporarily unavailable | 503, with a generic "try again later" |
| Anything unexpected | 500, with a generic message |

**PLT-054** A 5xx reply shall never carry internal detail (database messages, hosts, ports,
upstream bodies). The detail is logged, with the correlation id, and the caller gets only a generic
message. A bad-input message is safe by design and shall never quote database internals.

**PLT-055** A caller that disconnects mid-request shall be logged and sent no reply.

**PLT-056** The API shall be published as a description (OpenAPI) listing every route, and the
build shall fail if a route is registered but missing from it, because a gateway that routes only
declared operations would answer "not found" for it in a deployed environment.

## 7. Caching

**PLT-060** When a cache (Redis) is configured, reading a user's own profile or another user's
detail shall be served from it for a limited time (10 minutes by default). Every writer of a user,
contact or membership shall drop the affected entry **after** its write commits, so a change shows
at once; the time limit is only a backstop.

**PLT-061** The cache shall **fail open**: if it is slow or down, reads go to the database and no
error reaches the caller. Errors and "not found" shall never be cached. Cache keys shall hold no
email in clear and logs shall never quote the connection string.

**PLT-062** Without a cache configured, every read goes to the database.

## 8. Health checks

**PLT-070** The service shall answer `GET /health` with "ok" on both listeners, with no checks of
any dependency, so a database outage never gets a working instance restarted.

**PLT-071** `GET /health/database` shall be available only on the health listener. It shall answer
"ok / up" when a database round trip succeeds and 503 "unavailable / down" when it fails. A
deployment with no database shall answer 200 "not configured", because there is no database to
be down.

**PLT-072** Health answers shall carry no detail about the failure (no driver message, host or
port), because the listener is public so outside monitoring needs no credentials. They shall
carry a "do not cache" header so a stored success cannot hide a real outage.

**PLT-073** Only the two probes shall be reachable on the health port. Business routes shall be
registered only on the main listener. The health port shall differ from the API port. A
deployment platform that fixes the ports in its own manifest should leave both at their defaults.

**PLT-074** The health listener shall have short limits (5 seconds to read or write, 30 idle) so
a slow client cannot tie it up, and shall not write an access-log line for each poll.

## 9. Settings an operator must understand

Durations use Go style (`30s`, `5m`). "Together" means all or none. Placeholders are not values.

### Server and database

| Setting | Default | Purpose |
|---|---|---|
| Server port / health port | 8080 / 8081 | The two listeners; must differ. |
| Read, write, request and upstream timeouts | 60s each | Section 5. Must be greater than zero; an unreadable value stops start-up. |
| Database host / port | localhost / 5432 | Where the data lives. |
| Database user, password and name | none | Required in database and dual-write modes; all three or none in every mode. |
| Database SSL mode | none (driver default) | Set explicitly: `disable` locally, `require` or stricter remotely. |
| Database schema | user name, then public | The search path. |
| Pool: maximum / minimum connections | 20 / 2 | Only an **unset** value uses the default. Maximum must be a positive whole number; minimum may be zero. A value that is not a number, or is out of range, stops start-up. |
| Pool: maximum lifetime / idle time | 30m / 5m | Connection recycling. |

### Data source and legacy connection

| Setting | Default | Purpose |
|---|---|---|
| Data source (`DATA_SOURCE`) | `postgres` | One of `postgres` (database mode), `servicenow` (legacy mode) or `postgres-servicenow-dual-write` (dual-write mode); any other value stops start-up. See data-sources-and-scheduled-tasks.md. |
| SLA data source (`SLA_DATA_SOURCE`) | `postgres` | `postgres` or `servicenow`; overrides the source for SLA reads. |
| Product-usage read mode | `postgres` | `legacy` restores the legacy routing and only matters in dual-write mode. |
| Legacy integration address, token address, client id, client secret | none | Needed when the data source or SLA source is legacy or dual-write; the secret is a secret. |

### Authentication and callers

| Setting | Default | Purpose |
|---|---|---|
| Token issuer, signing-key address, accepted audiences | none | **Required.** Always-on verification of the user token. |
| Clock difference allowed | 30s | Leeway on expiry. Bad or non-positive values fall back to the default. |
| Admin roles | none | Token roles that make a caller a portal admin for team-schedule admin actions and case-alert settings. Unset means nobody is. |
| Machine-application client ids | none | Applications with full access. |
| CSM portal client id and internal email domain | none | Together; the domain's leading `@` is dropped. |
| Customer portal client id | none | Always scoped to the forwarded user. |

### Message hub and background jobs

See event-publishing.md. The hub address, connection secret and shared topic are all-or-nothing;
publishing needs an explicit `true`. Topics default as listed in event-publishing.md.

### Caching

| Setting | Default | Purpose |
|---|---|---|
| Cache connection string, or host and password | none | A connection string takes priority. Either one turns the cache on. |
| User cache lifetime | 10m | Backstop for invalidation. |
| Product-vulnerability options lifetime | 24h | Needs the cache. |

### Integration connections (each all-or-nothing)

The sales-record service, the directory service, the holiday calendar and the HR service each have
an address, token address, client id and client secret that must be set together or not at all.
The features that need them (see salesforce-ingest.md and team-schedule.md) are simply not offered
when their group is absent.

### Migration switches

The Salesforce and portal-write switches (`CSM_MIGRATION_*`) are on unless set to `false`, and
need their other prerequisites before their routes exist (see salesforce-ingest.md).
(**Confirm**, see 12.2.)

**PLT-080** Each all-or-nothing group shall stop start-up when only partly set: database
credentials, the message hub trio, the sales-record service, the directory service, the holiday
calendar, the HR service, and the CSM portal client id with its domain.

**PLT-081** Other settings shall be checked for shape at start-up: a list of escalation contact
emails must look like email addresses; the escalation-email switch must be `true`, `false` or
unset; ids that must be UUIDs (the incident default service, extra assignment groups, service
request teams) must be UUIDs; the firefighting type id must be 32 hex
characters; the cache address must start with `redis://` or `rediss://` and have a host;
internal email domains must look like `@example.com`; the query-hour domain must start with `@`.

**PLT-082** These switches are **off** until set to exactly `true`: event publishing,
change-request notices, the cloud status job, usage roll-up, the external-issue integration, and
project reopening. Project reopening also starts in dry run until its dry-run setting is
`false`. The escalation-email switch is on unless set to `false`. The migration switches in
section 9 are on unless set to `false` (**Confirm**, see 12.2).

**PLT-083** A server timeout, a pool duration or a pool size that is set but cannot be read (or is out of range) shall stop start-up; only an unset value uses its default. A polling
interval or similar optional duration that cannot be read, or is not positive, shall fall back to
its default without stopping the service. An unreadable Salesforce retry interval shall turn that
job off with a warning.

## 10. Database changes

**PLT-090** The schema shall live only in the migration files. They shall be applied by a
separate command before a release, which records each applied file by **its full file name** in a
ledger table, skips files already recorded, and stops at the first failure. Running it twice
applies nothing the second time.

**PLT-091** A migration shall be a single forward-only file; there is no scripted rollback. A change
that needs undoing is a new migration. Files shall be safe to run again.

**PLT-092** Files shall be applied in three groups, always in this order: the four-digit schema
migrations in ascending order; then the six-digit security-policy migrations (100000 and above);
then the old numbered files below 100000. This guarantees the tables a policy protects exist
before it is applied.

**PLT-093** The file name is the identity: a number used twice is not a conflict (both apply), and
**renaming an applied file makes it look new and runs it again**, so a rename needs a reason and
a plan for servers that already have it. Only files of the three accepted shapes belong in the
folder.

**PLT-094** Each run shall also write review bundles of exactly what it applied: one of the
security-policy files, one of the schema files, so a deploy has a standalone record of the
security SQL that shipped.

**PLT-095** The first block of numbers is a mirror of the sync component's own files, because both
use the same database and ledger. A migration that touches a table the sync component also uses
shall have its number checked against that component's files first.


## 11. Data protection and security

**PLT-100** The database shall enforce what a caller may see itself, with row-level security, not
only the Go code. For each request that touches protected data, the service shall tell the
database who is calling (internal or not, their email, and their project list) for the duration of
that one transaction only. The database then lets an internal caller see everything and an external
caller only records of projects they are a registered member of.

**PLT-101** If code reaches a protected table with **no** caller identity, the service shall fail
closed (forbidden), and shall never assume "unrestricted". Background jobs shall declare
themselves as the system, explicitly.

**PLT-102** The service shall run as a database role that is **not** a superuser, because a
superuser bypasses row-level security and hides policy mistakes.

**PLT-103** Some resources have no per-project concept (task SLAs, incidents and the other
incident-family records) and are therefore refused outright to callers who are not internal, as
are the reports, runbooks and customer-health actions that are for staff only.

**PLT-104** Every query shall use parameters; user input shall never be built into SQL text. Search
and sort fields shall be mapped from fixed lists.

**PLT-105** Secrets (database password, hub connection string, client secrets, cache password,
signing material) shall come from the environment only and shall never appear in logs, errors,
failure records or health answers. Token values are never logged; only the header name and a
reason are, when a token is rejected.

**PLT-106** Log lines that include user-supplied text (paths, messages) shall have line breaks
and control characters removed, so one request cannot forge another log line.

**PLT-107** A customer-visible response shall not carry internal-only fields. (**Confirm**, see
12.3: the service returns its own shape; trimming for customers is expected in the portal
backends.)

**PLT-108** The documented release practice requires a clean static security scan and a clean
known-vulnerability scan before every release. This is a practice of the team, not something the
running service enforces.

## 12. Open questions for the product owner

1. **Machine-application identity is trusted, not verified.** The application assertion is only
   decoded here, and a request with no tokens passes the identity step; the gateway and the
   per-endpoint checks are what protect the routes. Several actions (change-notice intake,
   failure records, scheduled-task runs) have no check of their own. Is gateway protection alone
   enough?
2. **Switches default on, not off.** The migration switches turn **on** unless set to `false`,
   which is the opposite of the comments in the code and in the example settings file. A
   deployment that sets nothing writes accounts, memberships and projects from Salesforce and
   accepts allocation events.
3. **Customer-safe shaping lives elsewhere.** The service returns full records (including who
   was notified of an escalation) to anyone who may see the case, and relies on the portals to
   trim.
4. **Admin roles are checked in two places.** A person's admin role is checked in the portal
   backend and, for a few actions, again here against a separate list that can silently differ.
   Some actions (banners, SLA recalculation, escalation access) already trust the portal
   backend's identity instead; others still read the role list.
5. **No limit on the number of requests.** The service has no rate limit of its own; it relies on
   the gateway.
6. **Retention of logs and failure records** is not defined here.

## 13. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Start-up and shutdown | `cmd/api/main.go`, `internal/db/postgres.go`, `internal/config/config.go` |
| Token checks | `internal/auth/validator.go`, `internal/auth/middleware.go`, `internal/auth/x5c_transport.go` |
| Caller scope | `internal/service/access_service.go`, `require_internal.go`, `internal/server/identity_middleware.go`, `internal_only.go` |
| Request handling | `internal/middleware/*`, `internal/handler/decode.go`, `internal/server/server.go` |
| Pagination | `internal/service/user_service.go` (`normalizePagination`) |
| Errors | `internal/apierror`, `internal/handler/decode.go` |
| Health | `internal/handler/health_handler.go`, `internal/server/health.go`, `health-openapi.yaml` |
| Cache | `internal/cache`, `internal/service/cached_user_service.go` |
| Database changes | `Makefile` (`migrate`), `migrations/` |
| Row-level security | `internal/repository/scoped.go`, migrations in the `1NNNNN` series |
| API description | `openapi.yaml`, `internal/server/routes_contract_test.go` |
| Settings | `internal/config/config.go`, `README.md` (configuration reference), `.env.example` |
| Tests | `internal/auth/auth_test.go`, `internal/config/config_test.go`, `internal/handler/health_handler_test.go`, `internal/service/access_service_test.go` |
