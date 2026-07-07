# MTTR Application

> **MTTR (Mean Time to Resolve)** analytics platform for WSO2 Customer Support.
> Pulls resolved customer-service cases from ServiceNow, computes a statistically robust
> **95th-percentile truncated mean** MTTR across nine dimensions, caches the results in
> PostgreSQL, and serves them to ServiceNow homepage widgets.

---

## Quickstart

```bash
# 1. Install dependencies (Node 18+)
npm install

# 2. Copy the env template and point DB_* at a local Postgres
cp .env.example .env
$EDITOR .env

# 3. Create the schema (idempotent)
npm run db:init

# 4. Run in dev mode with auto-reload
npm run dev

# 5. Smoke-test
curl -s http://localhost:3000/api/v1/health | jq
curl -s http://localhost:3000/api/v1/mttr/types | jq
```

That's it — the service is up on port 3000 with `AUTH_ENABLED=false` (see [Security Model](#security-model) for why that's the default). For the full picture — architecture, data model, aggregation dimensions, deployment to Choreo, ServiceNow wiring — keep reading.

---

## Table of Contents

1. [What This Project Does](#what-this-project-does)
2. [Why P95 Truncated Mean Instead of Average?](#why-p95-truncated-mean-instead-of-average)
3. [High-Level Architecture](#high-level-architecture)
4. [Repository Layout](#repository-layout)
5. [End-to-End Data Flow](#end-to-end-data-flow)
6. [Aggregation Dimensions](#aggregation-dimensions)
7. [Local Development Setup](#local-development-setup)
8. [Database Schema](#database-schema)
9. [REST API Reference](#rest-api-reference)
10. [Deployment to Choreo](#deployment-to-choreo)
11. [Security Model](#security-model)
12. [ServiceNow Integration Setup](#servicenow-integration-setup)
13. [Operations](#operations)
14. [Troubleshooting](#troubleshooting)
15. [Further Reading](#further-reading)

---

## What This Project Does

WSO2's Customer Support team needs to know **how long it actually takes to resolve a case** — broken down by team, priority, product, case type, patched-vs-non-patched, and over time. ServiceNow's built-in reporting only offers simple averages (which a single 90-day outlier can ruin) and doesn't scale across multiple years of data.

This project is an external **analytics service** that:

1. **Gets** Closed/Resolved cases from ServiceNow every 10 minutes (incremental, watermark-based).
2. **Computes** a 95th-percentile truncated-mean MTTR per dimension (excludes the slowest 5% of cases as outliers).
3. **Caches** results in PostgreSQL so widgets load quickly regardless of data volume.
4. **Serves** the cached MTTR via a REST API.
5. **Renders** the metrics in **12 homepage widgets in ServiceNow** (Google Charts via Jelly UI Pages in SN).
6. **Archives** quarterly summaries so historical trend charts survive long after raw data is deleted.

---

## Why P95 Truncated Mean Instead of Average?

Imagine a team that resolved 100 cases:

- 95 cases resolved in 2 – 10 hours
- 5 cases took 500+ hours each (blocked on customer, awaiting vendor patch, holiday delay)

A **simple average** would be ~30 hours. That's misleading — it makes the team look slow even though 95% of customers got fast service.

The **95th-percentile truncated mean** algorithm:

1. Sort all durations ascending.
2. Drop the top 5%.
3. Average what remains.

When a group has fewer than `MIN_SAMPLE_SIZE` (default 20) cases the system falls back to a simple average and sets `min_sample_met: false`. 

> Full algorithm walkthrough: see [`docs/logic-and-architecture.md` §2](docs/logic-and-architecture.md#2-why-p95-truncated-average).

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│ LAYER 1 — DATA SOURCE                                               │
│ ServiceNow Instance                                                 │
│                                                                     │
│   Scheduled Job (every 10 min) ─POST───►  /api/v1/cases/batch       │
│   Script Include  ◄─GET────────────────  /api/v1/mttr?type=…        │
│   Widgets (Jelly)  ◄─via Script Include   /api/v1/summary/historical│
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│ LAYER 2 — COMPUTE & API                                             │
│ WSO2 Choreo (Node.js + Express)                                     │
│                                                                     │
│   • Ingestion pipeline     – validates + UPSERTs case_events        │
│   • Aggregation engine     – nightly P95 truncated-mean computation │
│   • Query layer            – serves cached MTTR + historical data   │
│   • Retention engine       – purges old raw data, archives summaries│
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│ LAYER 3 — STORAGE                                                   │
│ PostgreSQL (Choreo-managed)                                         │
│                                                                     │
│   • case_events          – raw resolved cases (rolling 24 months)   │
│   • mttr_cache           – pre-computed MTTR per dimension          │
│   • case_events_summary  – quarterly archived MTTR (forever)        │
│   • ingestion_log        – batch audit trail (rolling 14 days)      │
└─────────────────────────────────────────────────────────────────────┘
```

### Why split it like this?

| Concern | Reason |
|---|---|
| ServiceNow should not compute P95 | SN's server-side scripting hits execution-time limits on large arrays. |
| Choreo (Node.js) is stateless | All state lives in PostgreSQL — the container can scale or restart with no data loss. |
| Pre-computed cache, not live recalculation | MTTR doesn't change second-by-second. A daily rebuild gives every widget load with less latency. |

---

## Repository Layout

```
.
├── .choreo/
│   └── component.yaml              ← Choreo component spec (port, basePath, OpenAPI ref)
├── docs/                           ← Detailed design and setup docs
│   ├── design-document.md          ← High-level design summary
│   ├── implementation-guide.md     ← Step-by-step build instructions
│   ├── logic-and-architecture.md   ← The "why" behind every decision
│   └── setup-guide.md              ← End-to-end setup (local → Choreo → SN)
├── servicenow/                     ← Code that lives INSIDE ServiceNow
│   ├── rest-message-setup.js       ← Step-by-step config guide (not executed)
│   ├── scheduled-job-mttr-push.js  ← Runs every 10 min in SN; pushes cases to Choreo
│   ├── script-include-mttr-dashboard.js ← Server-side adapter for widgets
│   └── widget-renderer-mttr.js     ← Homepage Widget Renderer entry point
├── servicenow-diagnostic-full.js   ← Background script for troubleshooting, eg why cases are/aren't syncing.
├── sql/
│   └── init.sql                    ← Table + index DDL (idempotent)
├── src/
│   ├── index.js                    ← Express bootstrap + middleware + route mounting
│   ├── config/
│   │   ├── index.js                ← Reads env vars into one config object
│   │   └── database.js             ← pg connection pool + helpers
│   ├── middleware/
│   │   └── auth.js                 ← JWT verification + role check
│   ├── services/
│   │   ├── ingestionService.js     ← Validate + UPSERT batches into case_events
│   │   ├── aggregationService.js   ← P95 truncated-mean compute engine
│   │   ├── cacheService.js         ← Read-through layer over mttr_cache
│   │   └── retentionService.js     ← Quarterly summarisation + raw-data deletion
│   ├── routes/
│   │   ├── health.js               ← /api/v1/health (unauthenticated)
│   │   ├── cases.js                ← /api/v1/cases/batch + /bulk-import
│   │   ├── mttr.js                 ← /api/v1/mttr?type=…
│   │   ├── admin.js                ← /api/v1/admin/* (admin role required)
│   │   └── summary.js              ← /api/v1/summary/historical, /periods
│   ├── jobs/
│   │   └── aggregationJob.js       ← node-cron: aggregation + retention nightly
│   ├── scripts/
│   │   └── initDb.js               ← `npm run db:init` — runs sql/init.sql
│   └── utils/
│       └── logger.js               ← Winston logger
├── Dockerfile                      ← node:20-alpine, non-root, HEALTHCHECK
├── openapi.yaml                    ← OpenAPI 3 spec (Choreo uses this)
├── package.json
└── .env.example                    ← All supported env variables documented
```

---

## End-to-End Data Flow

Tracing one case from ServiceNow all the way into the SN widgets:

```
1. Case CS-00012345 is closed in ServiceNow
   product="WSO2 API Manager", priority="2 - High",
   business_duration=14h 30m, account.u_integration_cs_team="Cloud",
   u_fix_eta_shared="2026-03-15"

2. Next scheduled-job tick (within 10 min) picks it up because
   sys_updated_on > u_mttr_last_sync watermark.

3. SN validates & normalises:
     cs_team = "Cloud"        (account default — no IAM/Choreo override)
     priority = "P2"          (extracted from "2 - High")
     is_patched = true        (u_fix_eta_shared is populated)

4. SN POSTs to /api/v1/cases/batch:
   { "batch_id": "batch_1712505600000",
     "cases": [{
       "case_sys_id":"abc123…", "product":"WSO2 API Manager",
       "cs_team":"Cloud", "business_duration_ms":52200000,
       "case_type":"Incident", "priority":"P2",
       "is_patched":true, "case_state":"Closed", …
     }] }

5. Choreo ingestion service:
   • Validates every field
   • UPSERTs into case_events (INSERT or UPDATE by case_sys_id)
   • Writes 1 row to ingestion_log
   • Returns { received:1, inserted:1, updated:0, rejected:0 }

6. SN advances watermark to the case's sys_updated_on.

7. At midnight, the aggregation cron runs:
   • Recomputes all 9 dimensions over the rolling 12-month window
   • This case contributes to overall_by_type, team_incidents,
     team_incidents_priority, priority_incidents, patched_incidents,
     monthly_trend_team, monthly_trend_type, monthly_trend_priority
   • Results UPSERTed into mttr_cache
   • Stale cache rows + old ingestion logs purged
   • Quarters older than 24 months summarised into case_events_summary
     and their raw rows deleted

8. SN homepage widget refreshes:
   • Jelly template calls MttrDashboardAPI.getTeamCaseTypeMTTR()
   • That calls GET /api/v1/mttr?type=team_incidents (cached!)
   • Returns "Cloud" bar including this case's duration
   • Google Charts renders the bar chart

9. ~24 months later, this case's quarter ages out:
   • Retention job summarises Q1 2026 cases by
     (case_type, priority, cs_team, product, is_patched)
   • Writes one summary row per group to case_events_summary
   • Deletes the raw case_events rows
   • The historical /summary/historical endpoint still surfaces this
     case's contribution via the quarterly aggregate.
```

---

## Aggregation Dimensions

The system computes MTTR across nine "dimensions" — each answers a different question. All are powered by entries in `DIMENSIONS` in [`src/services/aggregationService.js`](src/services/aggregationService.js).

| # | Dimension (`?type=…`) | GROUP BY | Filter | Question Answered |
|---|---|---|---|---|
| 1 | `overall_by_type` | `case_type` | — | Overall Incident vs Query MTTR |
| 2 | `team_incidents` | `cs_team` | `case_type='Incident'` | Which team resolves Incidents fastest/slowest? |
| 3 | `team_queries` | `cs_team` | `case_type='Query'` | Which team resolves Queries fastest/slowest? |
| 4 | `team_incidents_priority` | `cs_team, priority` | `case_type='Incident'` | How does each team perform per priority? |
| 5 | `priority_incidents` | `priority` | `case_type='Incident'` | Are P1s really faster than P4s? |
| 6 | `patched_incidents` | `is_patched` | `case_type='Incident'` | Do patched Incidents resolve faster? |
| 7 | `patched_queries` | `is_patched` | `case_type='Query'` | Do patched Queries resolve faster? |
| 8 | `monthly_trend_team` | `cs_team, month` | — | Is each team trending up or down? |
| 9 | `monthly_trend_type` | `case_type, month` | — | Is overall MTTR trending up or down? |
| 10| `monthly_trend_priority` | `priority, month` | `case_type='Incident'` | Are P1 incidents getting faster over time? |

---

## Local Development Setup

### Prerequisites

- Node.js ≥ 18
- PostgreSQL 14+ (locally, in Docker, or Choreo-managed)

### Steps

```bash
# 1. Clone and install dependencies
git clone <repo-url>
cd MTTR_Application
npm install

# 2. Create a local .env from the template
cp .env.example .env
# Edit .env: set DB_HOST/DB_USER/DB_PASSWORD to point at your Postgres instance.

# 3. Create the database schema (idempotent)
npm run db:init

# 4. Run in dev mode (nodemon, auto-reload)
npm run dev
# OR run once:
npm start
```

The server listens on `PORT` (default 3000). Confirm it's up with `curl http://localhost:3000/api/v1/health`.

### Bypassing auth locally

`AUTH_ENABLED=false` (the default) makes the auth middleware inject a mock admin user, so every endpoint — including `/admin/*` — works without a JWT.

### Seeding test data

Push a batch via `curl`:

```bash
curl -X POST http://localhost:3000/api/v1/cases/batch \
  -H 'Content-Type: application/json' \
  -d '{
    "batch_id": "manual-test-1",
    "cases": [{
      "case_sys_id": "test-001",
      "product": "WSO2 API Manager",
      "cs_team": "Cloud",
      "business_duration_ms": 7200000,
      "created_date": "2026-01-01T10:00:00Z",
      "closed_date":  "2026-01-01T12:00:00Z",
      "case_type": "Incident",
      "priority":  "P2",
      "is_patched": true,
      "case_state": "Closed"
    }]
  }'
```

Then force a recompute and view the result:

```bash
curl -X POST http://localhost:3000/api/v1/admin/cache/reset
curl http://localhost:3000/api/v1/mttr?type=team_incidents
```

---

## Database Schema

All schema lives in [`sql/init.sql`](sql/init.sql) (idempotent — safe to re-run).

### `case_events` — raw resolved cases

| Column | Type | Notes |
|---|---|---|
| `id` | SERIAL PK | |
| `case_sys_id` | VARCHAR(32) UNIQUE | ServiceNow `sys_id`; natural UPSERT key |
| `product` | VARCHAR(100) | Display name from `u_wso2_product` |
| `cs_team` | VARCHAR(100) | IAM/Choreo override OR account default |
| `business_duration_ms` | BIGINT | SLA-aware work time in ms |
| `created_date` | TIMESTAMPTZ | `sys_created_on` |
| `closed_date` | TIMESTAMPTZ | `closed_at` (state 3) or `resolved_at` (state 6) — stable close/resolve time; NOT `sys_updated_on` |
| `case_type` | VARCHAR(50) | Incident or Query |
| `priority` | VARCHAR(30) | P1–P4 (NULL for Queries) |
| `is_patched` | BOOLEAN | True when `u_fix_eta_shared` is populated |
| `case_state` | VARCHAR(30) | "Closed", "Solution Proposed", etc. |

Retained for `RETENTION_CASE_EVENTS_MONTHS` (default 24). Older rows are summarised and deleted.

### `mttr_cache` — pre-computed aggregations

| Column | Type | Notes |
|---|---|---|
| `cache_key` | VARCHAR(255) UNIQUE | `{dimension}|{label_values}|{rolling|YYYY-MM}` |
| `dimension_type` | VARCHAR(50) | e.g. "team_incidents" |
| `dimension_labels` | JSONB | Echoed back in API responses |
| `period_start` / `period_end` | DATE | The rolling 12-month window bounds |
| `total_cases` / `excluded_cases` | INTEGER | Sample size + P95 exclusion count |
| `p95_cutoff_ms` | BIGINT | "anything above this was excluded" |
| `truncated_avg_ms` | BIGINT | **THE MTTR VALUE** |
| `simple_avg_ms` | BIGINT | For comparison |
| `min_sample_met` | BOOLEAN | False → UI shows a warning |

### `case_events_summary` — quarterly archive

One row per `(period_label, case_type, priority, cs_team, product, is_patched)`. Survives forever; powers `/summary/historical`.

### `ingestion_log` — batch audit trail

One row per inbound batch. Retained for `RETENTION_INGESTION_LOG_DAYS` (default 14).

---

## REST API Reference

Mounted under `/api/v1/`. The full OpenAPI spec is in [`openapi.yaml`](openapi.yaml).

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET  | `/health` | None | DB liveness + ingestion / cache freshness |
| POST | `/cases/batch` | User | Incremental ingest (≤ 500 cases) |
| POST | `/cases/bulk-import` | User | One-off backfill (≤ 5000 cases) |
| GET  | `/mttr?type=<dim>` | User | Cached MTTR for one dimension |
| GET  | `/mttr/types` | User | Discover valid `type` values |
| GET  | `/summary/historical?case_type=…&group_by=…` | User | Quarterly archived MTTR |
| GET  | `/summary/periods` | User | List of quarters that have summaries |
| POST | `/admin/cache/reset[?type=<dim>]` | Admin | Recompute all or one dimension |
| GET  | `/admin/ingestion-logs?limit=<n>` | Admin | Recent batch history |
| POST | `/admin/retention/run` | Admin | Manually trigger retention |

### Example: `GET /api/v1/mttr?type=team_incidents`

```json
{
  "dimension_type": "team_incidents",
  "period": { "start": "2025-05-18", "end": "2026-05-18" },
  "calculated_at": "2026-05-18T00:00:01Z",
  "data": [
    {
      "labels": { "cs_team": "Cloud" },
      "total_cases": 150,
      "excluded_cases": 8,
      "p95_cutoff_hours": 72.5,
      "mttr_hours": 18.3,
      "simple_avg_hours": 25.1,
      "min_sample_met": true
    },
    { "labels": { "cs_team": "IAM" }, ... }
  ]
}
```

---

## Deployment to Choreo

1. **Push the repo** to GitHub (or wherever Choreo is connected).
2. **Create a new Service component** in Choreo pointing at the repo root.
   - The `.choreo/component.yaml` declares the service: REST endpoint, port 3000, base path `/api/v1`, OpenAPI at `openapi.yaml`.
3. **Provision Choreo-managed PostgreSQL** and link it; Choreo injects the connection settings as environment variables.
4. **Set the required env vars** in the Choreo deployment console — see [`.env.example`](.env.example).
5. **Build & deploy** — the bundled `Dockerfile` produces a `node:20-alpine` image running as non-root with a built-in HEALTHCHECK against `/api/v1/health`.
6. **Configure CORS** if any browser-based clients outside Choreo need to call the API: set `CORS_ALLOWED_ORIGINS` to a comma-separated allowlist.
7. **Run `npm run db:init`** once against the managed Postgres (Choreo job, or `psql` from a bastion host).

> Detailed deployment recipe: see Phase 2 of [`docs/setup-guide.md`](docs/setup-guide.md).

---

## Security Model

### Trust boundary

```
   ┌────────────┐   HTTPS + OAuth 2.0    ┌───────────────┐   internal HTTP   ┌────────────────┐
   │ ServiceNow │  ────────────────────▶ │ Choreo gateway│ ────────────────▶ │ MTTR container │
   └────────────┘  (Client Credentials)  └───────────────┘  (already-authed) └────────────────┘
     external              │                    │                                    │
                           ▼                    ▼                                    ▼
                     TLS termination      Validates JWT           App code runs with AUTH_ENABLED=false
                     Rate-limit at edge   against Choreo STS      Trusts req has already been authenticated
```

**The Choreo gateway is the only ingress path to the container.** Port 3000 is exposed to the Choreo internal network only — there is no NodePort, LoadBalancer, or ClusterIP that a caller outside Choreo can reach directly. Every request that arrives at `src/index.js` has already been authenticated by the gateway against the OpenAPI-declared `BearerAuth` security scheme (see [`openapi.yaml`](openapi.yaml)).

### Why `AUTH_ENABLED=false` in production is safe

The gateway is the authentication enforcement point. Double-validating the JWT inside the container would add a second JWKS fetch and a second signature check for no additional security benefit — the container never sees an unauthenticated request under the network model above.

`AUTH_ENABLED=true` remains supported for non-Choreo deployments where the app is exposed without a gateway in front (development containers, bare-metal deploys, etc.). Set the flag plus `AUTH_JWKS_URI` / `AUTH_ISSUER` / `AUTH_AUDIENCE` for that case; the middleware will then validate JWTs directly.

### Layered controls

| Layer | Where enforced | What it stops |
|---|---|---|
| TLS | Choreo edge | Man-in-the-middle |
| OAuth 2.0 | Choreo gateway (per `openapi.yaml` `security: [BearerAuth]`) | Unauthenticated calls to any `/api/*` route except `/health` |
| Admin role | Container (`requireAdmin` middleware) | Non-admin tokens hitting `/admin/*` or `/cases/bulk-import` |
| Rate limit | Both edges — Choreo gateway (per-customer) + container (`RATE_LIMIT_MAX`, per real client IP thanks to `trust proxy: 1`) | Runaway loops, accidental DoS |
| SQL injection | Container (parameterised queries throughout + `GROUP_BY_COLUMN_MAP.has()` guard on the one identifier interpolation) | Malicious inputs bypassing the whitelists |
| Log injection | Container (`X-Correlation-Id` header validated against `^[A-Za-z0-9._:-]{1,128}$`, malformed values dropped + logged) | Forged log lines via header |

### Health endpoint carve-out

`/api/v1/health` is deliberately unauthenticated (`security: []` in the OpenAPI spec) so Choreo readiness / liveness probes and the Dockerfile `HEALTHCHECK` can hit it. It returns a generic body only — no driver text, hostnames, or schema names — so exposing it publicly is safe.

---

## ServiceNow Integration Setup

The ServiceNow side has **three** moving parts:

### 1. Outbound REST plumbing

Follow [`servicenow/rest-message-setup.js`](servicenow/rest-message-setup.js) or Phase 3 of [`docs/setup-guide.md`](docs/setup-guide.md) to:

- Create a REST Message record `MTTR_Choreo_API` pointing at your Choreo URL.
- Attach an OAuth 2.0 client-credentials profile.
- Add six HTTP Methods: `POST_Batch`, `POST_BulkImport`, `GET_MTTR`, `GET_Health`, `POST_CacheReset`, `GET_Summary`.
- Create the system property `u_mttr_last_sync` (used as the sync watermark).

### 2. Scheduled job (continuous sync)

Paste [`servicenow/scheduled-job-mttr-push.js`](servicenow/scheduled-job-mttr-push.js) into a Scheduled Script Execution:

- **Name**: `MTTR - Push Resolved Cases`
- **Run**: every 10 minutes
- **Active**: true

The job is **incremental** (uses the watermark) and **at-least-once** (the watermark only advances after HTTP 200, so a Choreo outage simply means retry on the next tick).

### 3. Visualisation (12 homepage widgets)

For each entry in `sections()` of [`servicenow/widget-renderer-mttr.js`](servicenow/widget-renderer-mttr.js):

- Create a UI Page record (`sys_ui_page`) with the matching `name`.
- Inside its Jelly template, call `new MttrDashboardAPI().<method>()` from [`servicenow/script-include-mttr-dashboard.js`](servicenow/script-include-mttr-dashboard.js) to fetch data.
- Render with Google Visualization API (already CSP-whitelisted in most SN instances).

Drop the renderer onto a Homepage and pick a section.

---

## Operations

### Cron schedule

The daily aggregation cron is configured by `AGGREGATION_CRON` (default `0 0 * * *` — midnight UTC). On every tick it runs:

1. `runFullAggregation()` — recompute all 9 dimensions
2. `purgeStaleCache()` — drop cache rows from prior runs
3. `runRetention()` — purge old ingestion logs, summarise + delete aged cases

### Manual cache reset

```bash
# Recompute everything
curl -X POST $API/admin/cache/reset

# Recompute one dimension
curl -X POST "$API/admin/cache/reset?type=team_incidents"
```

### Tuning retention

Two env vars control retention:

| Env var | Default | Effect |
|---|---|---|
| `RETENTION_INGESTION_LOG_DAYS` | 14 | How long batch audit rows survive |
| `RETENTION_CASE_EVENTS_MONTHS` | 24 | Age past which raw cases are summarised & deleted |

The 24-month default leaves a 12-month safety buffer beyond the 12-month rolling aggregation window. Quarters within the cutoff are re-summarised on every run so reopened-and-re-closed cases stay accurate.

### Logs

All structured logs go to stdout via Winston. In production they emit as JSON for Choreo's log collector; locally they are colourised plaintext.

---

## Troubleshooting

### Cases aren't appearing in the SN widgets

1. Run [`servicenow-diagnostic-full.js`](servicenow-diagnostic-full.js) as a Background Script inside ServiceNow. It mirrors the scheduled job's query and reports exactly why cases were skipped (wrong case type, no product, no team, invalid duration).
2. Check `/api/v1/admin/ingestion-logs` — every batch is logged with `records_received / inserted / updated / rejected` + reasons for any rejections.
3. Check `/api/v1/health` — confirms the DB is up, shows `last_ingestion_at` and `last_aggregation_at`.

### Response shows `"min_sample_met": false`

A dimension group had fewer than `MIN_SAMPLE_SIZE` (default 20) cases — the system intentionally fell back to a simple average. Either wait for more data or lower `MIN_SAMPLE_SIZE` if appropriate.

### MTTR looks stale

- Check `last_aggregation_at` in `/health`. If it's >24 h old, the cron is failing — look at logs.
- Force an immediate recompute with `POST /admin/cache/reset`.

### Authentication 401 errors

- Verify `AUTH_ENABLED`, `AUTH_JWKS_URI`, `AUTH_ISSUER`, `AUTH_AUDIENCE` env vars match your IdP.
- For Choreo-protected deployments you can usually leave `AUTH_ENABLED=false` inside the container — the API gateway handles auth before the request reaches Express.

### Scheduled job logs "Push failed with HTTP 401"

The OAuth profile attached to the REST Message can't get a fresh access token. Re-check the `Client ID`, `Client Secret`, and `Token URL` on the OAuth profile record in SN.

---

## Further Reading

The `docs/` directory contains the project's design and operations corpus:

| File | Audience |
|---|---|
| [`setup-guide.md`](docs/setup-guide.md) | **First-time deployers — end-to-end setup (laptop → Choreo → ServiceNow)** |
| [`logic-and-architecture.md`](docs/logic-and-architecture.md) | Engineers — the "why" behind every decision |
| [`design-document.md`](docs/design-document.md) | Reviewers — high-level design summary |
| [`implementation-guide.md`](docs/implementation-guide.md) | Implementers — step-by-step build instructions |

