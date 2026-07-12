# MTTR Application — Implementation Guide

**Version:** 1.0  
**Date:** April 6, 2026

---

## 1. System Overview

The MTTR (Mean Time to Resolve) application calculates **P95 truncated-average** resolution times for support cases. It consists of three layers:

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Data Source** | ServiceNow (Washington DC) | Stores support cases; pushes resolved cases outbound |
| **Backend API** | Node.js / Express on WSO2 Choreo | Ingests, stores, aggregates, and serves MTTR metrics |
| **Presentation** | ServiceNow UI Pages (Jelly XML) | 12 homepage widgets rendered with Google Charts |

### Architecture Flow

```
ServiceNow Scheduled Job  ──(REST/OAuth 2.0)──►  Choreo Express API
        every 10 min                                    │
                                                 PostgreSQL DB
                                                        │
                                              Aggregation (cron daily)
                                                        │
ServiceNow Dashboard Widgets  ◄──(REST/OAuth 2.0)──  /api/v1/mttr
```

---

## 2. P95 Truncated-Average Algorithm

The core MTTR calculation excludes the top 5% of outliers:

1. **Collect** all `business_duration_ms` values for the given dimension (e.g., team + case type).
2. **Sort** ascending by duration.
3. **Find P95 cutoff**: `index = ceil(n × 0.95) - 1`.
4. **Exclude** all cases above the cutoff index (top 5%).
5. **Average** the remaining cases → this is the truncated mean.

If fewer than 20 cases (configurable via `MIN_SAMPLE_SIZE`), the simple average is used instead and a `min_sample_met: false` flag is set.

**Implementation:** `src/services/aggregationService.js` → `computeTruncatedMTTR(durations)`

---

## 3. Dimension Types

The system computes MTTR across 10 dimension types, each powering one or more SN widgets:

| Dimension Type | Group By | Filter | Monthly? | Widget |
|----------------|----------|--------|----------|--------|
| `overall_by_type` | case_type | — | No | KPI cards (overall, incidents, queries) |
| `team_incidents` | cs_team | Incident | No | Team × case type bar chart |
| `team_queries` | cs_team | Query | No | Team × case type bar chart |
| `team_incidents_priority` | cs_team, priority | Incident | No | (admin/future use) |
| `priority_incidents` | priority | Incident | No | (admin/future use) |
| `patched_incidents` | is_patched | Incident | No | Patched / Non-patched pie |
| `patched_queries` | is_patched | Query | No | Patched / Non-patched pie |
| `monthly_trend_team` | cs_team | — | Yes | Team trend line chart |
| `monthly_trend_type` | case_type | — | Yes | (admin/future use) |
| `monthly_trend_priority` | priority | Incident | Yes | Priority trend line chart |

**Implementation:** `DIMENSIONS` constant in `src/services/aggregationService.js`

---

## 4. Data Pipeline

### 4.1 ServiceNow Scheduled Job

**File:** `servicenow/scheduled-job-mttr-push.js`

**Schedule:** Every 10 minutes.

**Logic:**

1. Read watermark from system property `u_mttr_last_sync` (defaults to `2024-01-01 00:00:00` on first run).
2. Query `sn_customerservice_case` with filters:
   - State IN (3 = Closed, 6 = Resolved/Cancelled)
   - `sys_updated_on > watermark`
   - Account is not empty
   - Account → Integration CS Team is not empty
   - Project is not empty
   - Business Duration is not empty
   - Created on >= 2024-01-01
   - Case Type IN (Incident sys_id, Query sys_id)
3. For each case, extract fields and normalize:
   - `is_patched` → derived from `u_fix_eta_shared` (populated = patched)
   - `priority` → normalized to P1–P4 for Incidents; empty for Queries
   - `business_duration_ms` → from GlideDuration `dateNumericValue()`
4. POST batch to Choreo `/api/v1/cases/batch` via REST Message with OAuth 2.0.
5. On HTTP 200, advance watermark to latest `sys_updated_on` in batch.
6. On failure, do **not** advance watermark (next run retries).

### 4.2 Backend Ingestion

**File:** `src/services/ingestionService.js`

**Validation rules per case record:**

| Field | Rule |
|-------|------|
| `case_sys_id` | Required, string, max 32 chars |
| `business_duration_ms` | Required, positive integer |
| `created_date` | Required |
| `case_type` | Required, must be `Incident` or `Query` |
| `case_state` | Required |
| `product` | Required |
| `cs_team` | Required |
| `priority` | Required for Incidents only; null for Queries |

Records that fail validation are rejected (not inserted) but do not fail the batch. The response includes `rejected_details` with the reason for each rejected record.

**Upsert strategy:** `INSERT ... ON CONFLICT (case_sys_id) DO UPDATE` — this allows the scheduled job to re-push updated cases without duplicates.

### 4.3 Aggregation

**File:** `src/services/aggregationService.js`

**Trigger:** Daily cron job (default: midnight) + on-demand via admin API + auto-compute on cache miss.

**Steps per dimension type:**

1. Determine rolling period (last 12 months from today).
2. For monthly dimensions: get distinct months, then for each month, fetch durations grouped by the dimension's `groupBy` fields.
3. For non-monthly dimensions: fetch all durations in the period, grouped by `groupBy` fields.
4. For each group: run `computeTruncatedMTTR()`.
5. Upsert result into `mttr_cache` table keyed by `cache_key` (format: `dimensionType|groupValues|month_or_rolling`).

**Cache miss auto-compute:** If the `/api/v1/mttr` endpoint finds no cached data for a dimension type, it triggers `runAggregationForType()` on-the-fly before returning results. This is handled in `src/services/cacheService.js`.

---

## 5. Database Schema

**File:** `sql/init.sql`

### Tables

**`case_events`** — Raw case data ingested from ServiceNow.

| Column | Type | Notes |
|--------|------|-------|
| `id` | SERIAL PK | |
| `case_sys_id` | VARCHAR(32) UNIQUE | Upsert key |
| `product` | VARCHAR(100) | WSO2 product name |
| `cs_team` | VARCHAR(100) | Integration CS team |
| `business_duration_ms` | BIGINT | Resolution time in milliseconds |
| `created_date` | TIMESTAMPTZ | Case creation date |
| `closed_date` | TIMESTAMPTZ | Case closure/resolution date |
| `case_type` | VARCHAR(50) | `Incident` or `Query` |
| `priority` | VARCHAR(30) | P1–P4 for Incidents; NULL for Queries |
| `is_patched` | BOOLEAN | Derived from fix ETA field |
| `case_state` | VARCHAR(30) | Closed, Resolved, etc. |
| `ingested_at` | TIMESTAMPTZ | When record was first ingested |
| `updated_at` | TIMESTAMPTZ | When record was last updated |

**`mttr_cache`** — Pre-computed P95 truncated averages.

| Column | Type | Notes |
|--------|------|-------|
| `cache_key` | VARCHAR(255) UNIQUE | `dimensionType\|groupValues\|month_or_rolling` |
| `dimension_type` | VARCHAR(50) | One of the 10 dimension types |
| `dimension_labels` | JSONB | Group-by field values |
| `period_start` / `period_end` | DATE | Rolling window boundaries |
| `total_cases` | INTEGER | Cases in the group |
| `excluded_cases` | INTEGER | Cases excluded by P95 cutoff |
| `p95_cutoff_ms` | BIGINT | Duration at the 95th percentile |
| `truncated_avg_ms` | BIGINT | P95 truncated average (the MTTR) |
| `simple_avg_ms` | BIGINT | Simple average for comparison |
| `min_sample_met` | BOOLEAN | False if below minimum sample size |

**`ingestion_log`** — Audit trail for batch imports.

---

## 6. API Endpoints

**Base URL:** `/api/v1` (Choreo gateway prepends the component path)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | None | Health check + data freshness |
| POST | `/cases/batch` | Bearer JWT | Ingest up to 500 cases |
| POST | `/cases/bulk-import` | Bearer JWT | Backfill up to 5000 cases |
| GET | `/mttr?type={dim}` | Bearer JWT | Get cached MTTR for a dimension |
| GET | `/mttr/types` | Bearer JWT | List available dimension types |
| POST | `/admin/cache/reset[?type=]` | Bearer JWT + Admin | Force recalculation |
| GET | `/admin/ingestion-logs` | Bearer JWT + Admin | View recent ingestion logs |

**Full OpenAPI spec:** `openapi.yaml`

---

## 7. Authentication

- **Choreo STS** issues OAuth 2.0 Client Credentials tokens.
- The Express middleware verifies JWT via JWKS (`RS256`).
- When `AUTH_ENABLED=false` (development), all requests are passed through with a synthetic dev user.
- ServiceNow authenticates to Choreo via an OAuth 2.0 profile on the REST Message.

**Implementation:** `src/middleware/auth.js`

---

## 8. ServiceNow Dashboard Widgets

### 8.1 Widget Architecture

Each widget is a standalone **UI Page** (Jelly XML) that can be embedded into any ServiceNow dashboard via a Homepage Widget Renderer.

| Widget | UI Page Name | Chart Type | Data Source (Script Include method) |
|--------|-------------|------------|-------------------------------------|
| CS Overall MTTR | `mttr_widget_overall` | KPI card | `getOverallByType()` |
| Incident MTTR | `mttr_widget_incidents` | KPI card | `getOverallByType()` |
| Query MTTR | `mttr_widget_queries` | KPI card | `getOverallByType()` |
| Priority Trend | `mttr_widget_priority_trend` | Line chart | `getMonthlyTrendPriority()` |
| Team Trend | `mttr_widget_team_trend` | Line chart | `getMonthlyTrendTeam()` |
| Team × Case Type | `mttr_widget_team_casetype` | Column chart | `getTeamCaseTypeMTTR()` |
| Patched MTTR | `mttr_widget_patched` | Donut pie | `getPatchedMTTR()` |
| Non-Patched MTTR | `mttr_widget_non_patched` | Donut pie | `getPatchedMTTR()` |

### 8.2 Script Include

**File:** `servicenow/script-include-mttr-dashboard.js`  
**Class:** `MttrDashboardAPI`

The Script Include fetches data from the Choreo API via `sn_ws.RESTMessageV2` and transforms the raw API response into structures optimized for each widget type. Key transformations:

- **Hours → Display:** `_hoursToDisplay()` converts decimal hours to `X Days Y Hours Z Minutes` format for tooltips.
- **Dimension merging:** `getTeamCaseTypeMTTR()` fetches `team_incidents` and `team_queries` separately, then merges them into a single team → {Incident, Query} structure.
- **Patched bucketing:** `getPatchedMTTR()` splits data into `patched` and `non_patched` buckets.

### 8.3 Jelly XML Constraints

ServiceNow's Jelly XML parser imposes restrictions on `<script>` blocks:

| Constraint | Solution |
|-----------|----------|
| Raw `<` breaks XML parsing | Reverse comparisons: `array.length > i` instead of `i < array.length` |
| Raw `&&` is invalid XML | Use arithmetic checks: `a + b === 0` instead of `a === 0 && b === 0` |
| `<![CDATA[...]]>` inside `<script>` renders a blank page | Do NOT use CDATA in script blocks; only in `<g:evaluate>` |
| `${var}` inside `<script>` is evaluated by Jelly | Use `<textarea style="display:none">` to pass data, read via `document.getElementById().value` |

### 8.4 Responsiveness

**Chart widgets** (5 widgets): Dynamic height via aspect ratio calculated from container width at draw time, debounced `resize` listener, and `ResizeObserver` for container-level changes. Initial draw delayed by 100ms to allow SN layout to finalize.

**KPI widgets** (3 widgets): CSS `@media` queries scale font size and padding at 768px and 480px breakpoints.

### 8.5 Y-Axis Unit Conversion

The API returns MTTR in hours. Chart widgets convert to **Days** at render time (`hours / 24`, rounded to 1 decimal). Tooltips show the original `X Days Y Hours Z Minutes` format via `setFormattedValue()`.

### 8.6 Homepage Widget Renderer

**File:** `servicenow/widget-renderer-mttr.js`

Maps section names to UI Page names. Allows users to add MTTR widgets to any SN Homepage or Dashboard by selecting a section from the renderer's list.

---

## 9. Deployment

### 9.1 Choreo (Backend)

- **Source:** GitHub repo for `MTTR_Application`, branch `main`.
- **Auto-deploy:** Choreo redeploys on every push to `main`.
- **Dockerfile:** `node:20-alpine`, non-root user (UID 10001), built-in health check.
- **Environment variables:** `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_SSL`, `AUTH_ENABLED`, `AUTH_JWKS_URI`, `AUTH_ISSUER`, `AUTH_AUDIENCE`.

### 9.2 ServiceNow

All ServiceNow artifacts are created manually in the instance:

1. **REST Message:** `MTTR_Choreo_API` with OAuth 2.0 Client Credentials profile (see `servicenow/rest-message-setup.js`).
2. **Script Include:** `MttrDashboardAPI` — paste from `servicenow/script-include-mttr-dashboard.js`.
3. **Scheduled Job:** `MTTR - Push Resolved Cases` (every 10 min) — paste from `servicenow/scheduled-job-mttr-push.js`.
4. **UI Pages:** Create 8 UI Pages (one per widget XML file in `servicenow/widgets/`). Name must match the filename without extension.
5. **Homepage Widget Renderer:** Paste from `servicenow/widget-renderer-mttr.js`.
6. **System Property:** `u_mttr_last_sync` (string, initially empty).

---

## 10. Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 3000 | Express server port |
| `NODE_ENV` | development | Environment mode |
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 5432 | PostgreSQL port |
| `DB_NAME` | mttr_db | Database name |
| `DB_USER` | mttr_user | Database user |
| `DB_PASSWORD` | — | Database password |
| `DB_SSL` | false | Enable SSL (`true`/`false`) |
| `DB_POOL_MIN` | 2 | Min pool connections |
| `DB_POOL_MAX` | 10 | Max pool connections |
| `AUTH_ENABLED` | false | Enable JWT auth |
| `AUTH_JWKS_URI` | — | JWKS endpoint URL |
| `AUTH_ISSUER` | — | Expected JWT issuer |
| `AUTH_AUDIENCE` | — | Expected JWT audience |
| `AGGREGATION_CRON` | `0 0 * * *` | Aggregation cron expression |
| `AGGREGATION_ROLLING_MONTHS` | 12 | Rolling window size |
| `MIN_SAMPLE_SIZE` | 20 | Minimum cases for P95 calculation |
| `RATE_LIMIT_WINDOW_MS` | 900000 | Rate limit window (15 min) |
| `RATE_LIMIT_MAX` | 100 | Max requests per window |
| `LOG_LEVEL` | info | Winston log level |

### ServiceNow Constants (in Scheduled Job)

| Constant | Value | Description |
|----------|-------|-------------|
| `BATCH_SIZE` | 500 | Cases per API call |
| `PROPERTY_NAME` | `u_mttr_last_sync` | Watermark system property |
| `INCIDENT_CASE_SYS_ID` | `8d4b87bd1b18f010cb6898aebd4bcb59` | sys_id of Incident case type |
| `QUERY_CASE_SYS_ID` | `0d5b8fbd1b18f010cb6898aebd4bcba5` | sys_id of Query case type |

---

## 11. File Structure

```
MTTR_Application/
├── Dockerfile                              # Container image definition
├── openapi.yaml                            # OpenAPI 3.0 specification
├── package.json                            # Node.js dependencies
├── sql/
│   └── init.sql                            # Database schema (3 tables)
├── src/
│   ├── index.js                            # Express app entry point
│   ├── config/
│   │   ├── index.js                        # Environment config loader
│   │   └── database.js                     # PostgreSQL connection pool
│   ├── middleware/
│   │   └── auth.js                         # JWT authentication middleware
│   ├── routes/
│   │   ├── health.js                       # GET /health
│   │   ├── cases.js                        # POST /cases/batch, /cases/bulk-import
│   │   ├── mttr.js                         # GET /mttr, /mttr/types
│   │   └── admin.js                        # POST /admin/cache/reset, GET /admin/ingestion-logs
│   ├── services/
│   │   ├── ingestionService.js             # Validation + upsert logic
│   │   ├── aggregationService.js           # P95 truncated mean + dimension aggregation
│   │   └── cacheService.js                 # Cache read with auto-compute on miss
│   ├── jobs/
│   │   └── aggregationJob.js               # Daily cron wrapper
│   ├── scripts/
│   │   └── initDb.js                       # One-time schema setup
│   └── utils/
│       └── logger.js                       # Winston logger
├── servicenow/
│   ├── scheduled-job-mttr-push.js          # SN Scheduled Job script
│   ├── script-include-mttr-dashboard.js    # MttrDashboardAPI Script Include
│   ├── rest-message-setup.js               # REST Message configuration guide
│   ├── widget-renderer-mttr.js             # Homepage Widget Renderer
│   └── widgets/
│       ├── mttr_widget_overall.xml         # KPI: Overall MTTR
│       ├── mttr_widget_incidents.xml       # KPI: Incident MTTR
│       ├── mttr_widget_queries.xml         # KPI: Query MTTR
│       ├── mttr_widget_priority_trend.xml  # Line: Priority trend
│       ├── mttr_widget_team_trend.xml      # Line: Team trend
│       ├── mttr_widget_team_casetype.xml   # Column: Team × Case Type
│       ├── mttr_widget_patched.xml         # Pie: Patched MTTR
│       └── mttr_widget_non_patched.xml     # Pie: Non-patched MTTR
└── docs/
    ├── design-document.md                  # High-level design
    ├── setup-guide.md                      # Setup instructions
    └── implementation-guide.md             # This file
```
