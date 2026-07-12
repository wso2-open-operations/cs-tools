# MTTR Application — Design Document

**Version:** 1.0  
**Date:** March 28, 2026  
**Status:** Final

---

## 1. Overview

This document describes the design of a system to calculate **Mean Time to Resolve (MTTR)** for support cases using a **95th Percentile Truncated Average**. The system pushes resolved case data from ServiceNow (Washington DC release) to an external application hosted on WSO2 Choreo, which stores, aggregates, and serves MTTR metrics back to ServiceNow homepage widgets.

### 1.1 What is 95th Percentile Truncated Average?

The system identifies the top 5% of cases that took the longest to resolve and **excludes them** from the average calculation. This prevents extreme outliers from skewing team performance metrics.

For example, given 100 resolved cases sorted by business duration:
- The 95th percentile cutoff is at position 95
- Cases at positions 96–100 (top 5%) are excluded
- The MTTR is the average of cases at positions 1–95

---

## 2. Problem Statement

The support team needs accurate MTTR metrics across multiple dimensions (team, product, case type, priority, patched status). ServiceNow's native reporting lacks the ability to compute percentile-based truncated averages. Computing this within SN (e.g., via Performance Analytics) is limited and rigid.

### 2.1 Key Requirements

- Calculate MTTR as a 95th percentile truncated average
- Slice MTTR by: case type, team, priority, patched status, and monthly trends
- Rolling 12-month window
- Data volume: ~20,000 resolved cases per year
- Metrics consumed via ServiceNow homepage widgets
- Historical backfill of at least 1 year of data

---

## 3. Architecture

### 3.1 High-Level Architecture

```
ServiceNow (Washington DC)
│
├── Scheduled Job (every 10 min)
│       └── Queries sn_customerservice_case for recently closed or solution proposed cases
│       └── Uses sys_updated_on watermark (stored in System Property)
│       └── POST /api/v1/cases/batch → Choreo App
│
├── SN Widgets (Jelly UI Pages on Homepage)
│       └── GET /api/v1/mttr?type={dimension} → Choreo App
│       └── Renders data via Google Charts
│
└── REST Message (MTTR_Choreo_API)
        └── OAuth 2.0 Client Credentials authentication

WSO2 Choreo
│
├── Node.js Application
│   ├── POST /api/v1/cases/batch         ← ingest endpoint
│   ├── POST /api/v1/cases/bulk-import   ← one-time backfill
│   ├── GET  /api/v1/mttr?type=...       ← aggregated MTTR
│   ├── POST /api/v1/admin/cache/reset   ← manual recalculation
│   ├── GET  /api/v1/admin/ingestion-logs
│   └── GET  /api/v1/health
│
├── Aggregation Cron Job (daily)
│
└── Choreo Managed PostgreSQL
        ├── case_events       (raw data)
        ├── mttr_cache        (pre-computed aggregations)
        └── ingestion_log     (audit trail)
```

### 3.2 Data Flow

```
1. Case resolved/solution proposed in SN
2. SN Scheduled Job picks it up (every 10 min via sys_updated_on watermark)
3. Job POSTs batch to Choreo → UPSERT into case_events table
4. Daily aggregation job recalculates all MTTR metrics → writes to mttr_cache
5. SN Widget calls GET /api/v1/mttr → reads from mttr_cache → displays in portal
```

---

## 4. Design Decisions and Rationale

### 4.1 Why Scheduled Job Instead of Business Rule?

**Decision:** Use a ServiceNow Scheduled Job instead of an async Business Rule to push data.

**Rationale:**
- A Business Rule (even async) that makes an HTTP callout has **no built-in retry**. If Choreo is unreachable, the data is lost silently.
- A custom staging table was considered but ruled out since we cannot create custom tables in the instance.
- The Scheduled Job queries the case table directly using a watermark pattern (`sys_updated_on > last_sync_timestamp`). On success, the watermark advances. On failure, the same records are retried next run.
- This approach is inherently retry-safe without needing a staging table.

**Alternatives considered:**
- Async Business Rule → direct HTTP: No retry on failure → rejected
- Async BR → staging table → scheduled job: Requires custom table → rejected
- Flow Designer: Heavier, harder to version control → rejected

### 4.2 Why UPSERT Instead of INSERT?

**Decision:** All ingestions use UPSERT on `case_sys_id`.

**Rationale:**
- Cases can transition through Solution Proposed → Rejected → Solution Proposed → Closed. Each transition may update `business_duration`. UPSERT ensures the latest value always wins.
- If the Scheduled Job's HTTP response times out but Choreo actually processed the batch, the next run re-sends the same records. UPSERT makes this safe (no duplicates).

### 4.3 Why Pre-aggregation Cannot Use Traditional Rollups?

**Decision:** Compute P95 truncated average from raw data each time, not from pre-aggregated values.

**Rationale:**
- Percentiles are **not additive**. You cannot merge P95 from set A and P95 from set B to get P95 of A∪B. The raw sorted data is required for accurate computation.
- At 20K records/year (~1,650/month), a full-table sort is trivially fast (<1 second).
- The aggregation runs daily via a cron job and stores results in the `mttr_cache` DB table.
- API responses read from the cache table, not raw data.

### 4.4 Why DB-Based Cache Instead of In-Memory?

**Decision:** Use the `mttr_cache` PostgreSQL table as the primary cache.

**Rationale:**
- Choreo can restart pods (scaling, deployments, health checks). In-memory cache is lost on restart.
- The cache table has ~50-100 rows (one per dimension combination). Reads are sub-millisecond.
- The daily aggregation job writes to this table. Pod restarts don't affect data freshness.
- An optional in-memory layer can be added on top if needed.

### 4.5 Cache Refresh Strategy

**Decision:** Daily recalculation + manual reset API.

**Rationale:**
- MTTR is a rolling 12-month metric. A 24-hour-old number is perfectly acceptable.
- Per-event recalculation (every time a case is ingested) was considered but adds unnecessary computation for a long-term metric.
- A `POST /api/v1/admin/cache/reset` endpoint allows on-demand recalculation when stakeholders need the latest data.

### 4.6 Why Node.js?

**Decision:** Node.js/Express over Ballerina, Python, or Java.

**Rationale:**
- The team's familiarity with JavaScript reduces development and maintenance friction.
- Choreo has good Node.js support with Docker-based deployment.
- The business logic (sorting, percentile math) is simple enough that language-specific stats libraries aren't needed.
- Largest ecosystem for web middleware (helmet, cors, rate-limit, JWT validation).

### 4.7 Patched vs Non-Patched Derivation

**Decision:** Derive `is_patched` from whether the SN `u_fix_eta_shared` date field is populated.

**Rationale:**
- The source field is a date, not a boolean.
- If the date is non-empty → the case had a fix ETA shared → considered "patched."
- Conversion happens at extraction time in the SN Scheduled Job, not in Choreo.

### 4.8 Why the Scheduled Job Also Handles Backfill?

**Decision:** The first run of the Scheduled Job performs the historical backfill automatically.

**Rationale:**
- When `u_mttr_last_sync` property is empty, the job defaults to 1 year ago and starts pulling historical data in batches of 500.
- No separate backfill mechanism needed.
- A bulk-import API endpoint exists for direct large imports if the batch approach is too slow.

### 4.9 Minimum Sample Size

**Decision:** If a dimension grouping has fewer than 20 cases, fall back to simple average (no 95th percentile truncation).

**Rationale:**
- P95 of 10 cases means excluding 0.5 cases — mathematically ambiguous.
- The API response includes a `min_sample_met: false` flag so widgets can display a warning.

### 4.10 API Authentication (Choreo STS — Client Credentials)

**Decision:** Authenticate all API consumers via OAuth 2.0 Client Credentials grant through Choreo STS.

**Rationale:**
- The sole consumer is ServiceNow (scheduled jobs and scripts) — machine-to-machine only.
- Choreo gateway validates JWT tokens from Choreo STS and strips the Authorization header before forwarding to the backend.
- `AUTH_ENABLED=false` on Choreo deployments — the gateway is the auth layer, no double validation needed.
- `AUTH_ENABLED=true` is available for non-Choreo deployments where the app must validate JWTs directly.

**See §11 Trust Boundary below** for the network model that makes disabling in-container auth safe.

---

## 5. Data Model

### 5.1 case_events (Raw Case Data)

| Column | Type | Description |
|---|---|---|
| id | SERIAL PK | Auto-increment |
| case_sys_id | VARCHAR(32) UNIQUE | SN case sys_id (UPSERT key) |
| product | VARCHAR(100) | WSO2 product name |
| cs_team | VARCHAR(100) | Integration CS team |
| business_duration_ms | BIGINT | Business duration in milliseconds |
| created_date | TIMESTAMPTZ | Case opened_at |
| closed_date | TIMESTAMPTZ | Case closed_at (nullable for Solution Proposed) |
| case_type | VARCHAR(50) | Incident, Query, Security Report |
| priority | VARCHAR(20) | P1, P2, P3, P4 |
| is_patched | BOOLEAN | Derived from u_fix_eta_shared date presence |
| case_state | VARCHAR(30) | solution_proposed or closed |
| ingested_at | TIMESTAMPTZ | When record was first ingested |
| updated_at | TIMESTAMPTZ | When record was last updated |

### 5.2 mttr_cache (Pre-computed Aggregations)

| Column | Type | Description |
|---|---|---|
| id | SERIAL PK | Auto-increment |
| cache_key | VARCHAR(255) UNIQUE | Composite key for UPSERT |
| dimension_type | VARCHAR(50) | e.g., team_incidents, patched_queries |
| dimension_labels | JSONB | e.g., {"cs_team": "Cloud", "priority": "P1"} |
| period_start | DATE | Rolling period start |
| period_end | DATE | Rolling period end |
| total_cases | INTEGER | Total cases in group |
| excluded_cases | INTEGER | Top 5% excluded |
| p95_cutoff_ms | BIGINT | The 95th percentile threshold |
| truncated_avg_ms | BIGINT | The MTTR value (P95 truncated mean) |
| simple_avg_ms | BIGINT | Untruncated mean for comparison |
| min_sample_met | BOOLEAN | False if below 20 cases |
| calculated_at | TIMESTAMPTZ | When this row was last computed |

### 5.3 ingestion_log (Audit Trail)

| Column | Type | Description |
|---|---|---|
| id | SERIAL PK | Auto-increment |
| batch_id | VARCHAR(100) | Batch identifier from SN job |
| records_received | INTEGER | Total in batch |
| records_inserted | INTEGER | New records |
| records_updated | INTEGER | Updated via UPSERT |
| records_rejected | INTEGER | Failed validation |
| rejected_details | JSONB | Array of {sys_id, reason} |
| ingested_at | TIMESTAMPTZ | Timestamp |

---

## 6. API Specification

### 6.1 Ingestion

**POST /api/v1/cases/batch** — Ingest up to 500 cases  
**POST /api/v1/cases/bulk-import** — Ingest up to 5000 cases (backfill)

Request:
```json
{
  "batch_id": "batch_20260328_143000",
  "cases": [
    {
      "case_sys_id": "abc123",
      "product": "WSO2 API Manager",
      "cs_team": "Cloud Team",
      "business_duration_ms": 86400000,
      "created_date": "2026-03-01T08:00:00Z",
      "closed_date": "2026-03-02T08:00:00Z",
      "case_type": "Incident",
      "priority": "P2",
      "is_patched": true,
      "case_state": "closed"
    }
  ]
}
```

Response (200):
```json
{
  "batch_id": "batch_20260328_143000",
  "received": 50,
  "inserted": 45,
  "updated": 3,
  "rejected": 2,
  "rejected_details": [
    {"case_sys_id": "xyz789", "reason": "business_duration_ms is null"}
  ]
}
```

### 6.2 MTTR Query

**GET /api/v1/mttr?type={dimension_type}**

Valid types: `overall_by_type`, `team_incidents`, `team_incidents_priority`, `team_queries`, `priority_incidents`, `patched_incidents`, `patched_queries`, `monthly_trend_team`, `monthly_trend_type`

Response (200):
```json
{
  "dimension_type": "team_incidents",
  "period": {"start": "2025-03-28", "end": "2026-03-28"},
  "calculated_at": "2026-03-28T00:00:00Z",
  "data": [
    {
      "labels": {"cs_team": "Cloud Team"},
      "total_cases": 150,
      "excluded_cases": 8,
      "p95_cutoff_hours": 120.5,
      "mttr_hours": 45.2,
      "simple_avg_hours": 52.1,
      "min_sample_met": true
    }
  ]
}
```

### 6.3 Admin

**POST /api/v1/admin/cache/reset** — Recalculate all aggregations  
**POST /api/v1/admin/cache/reset?type=team_incidents** — Recalculate specific dimension  
**GET /api/v1/admin/ingestion-logs?limit=50** — View recent ingestion logs

### 6.4 Health

**GET /api/v1/health**

```json
{
  "status": "healthy",
  "total_cases": 19847,
  "last_ingestion_at": "2026-03-28T14:00:00Z",
  "last_aggregation_at": "2026-03-28T00:00:00Z",
  "cache_entries": 47
}
```

---

## 7. ServiceNow Components

| Component | Type | Purpose |
|---|---|---|
| MTTR - Push Resolved Cases | Scheduled Job (10 min) | Queries case table, pushes batches to Choreo |
| u_mttr_last_sync | System Property | Stores watermark timestamp |
| MTTR_Choreo_API | REST Message | HTTP configuration for Choreo API calls |
| MttrDashboardAPI | Script Include | Fetches MTTR data from Choreo for the widgets |
| 12 MTTR UI Pages | Jelly UI Pages | Render the charts on the homepage |

---

## 8. Aggregation Dimensions

| Widget | Dimension Type | Grouping | Chart |
|---|---|---|---|
| Overall MTTR per type | overall_by_type | case_type | KPI cards |
| Team MTTR (Incidents) | team_incidents | cs_team | Bar |
| Team MTTR (Incidents + Priority) | team_incidents_priority | cs_team × priority | Stacked bar |
| Team MTTR (Queries) | team_queries | cs_team | Bar |
| Incident MTTR by Priority | priority_incidents | priority | Pie |
| Patched Incidents | patched_incidents | is_patched | Pie |
| Patched Queries | patched_queries | is_patched | Pie |
| Monthly trend by team | monthly_trend_team | cs_team × month | Line |
| Monthly trend by type | monthly_trend_type | case_type × month | Line |

---

## 9. Data Validation Rules

Records are **rejected** at ingestion if:
- `case_sys_id` is missing or exceeds 32 characters
- `business_duration_ms` is null, zero, or negative
- `created_date` is missing
- `case_type`, `case_state`, `product`, `cs_team`, or `priority` is missing

Rejected records are logged in `ingestion_log.rejected_details` for review.

---

## 10. Security

| Layer | Mechanism |
|---|---|
| SN → Choreo API | OAuth 2.0 Client Credentials (Choreo STS) |
| Admin Endpoints | JWT role check (`mttr-admin`) — enforced at the gateway when `AUTH_ENABLED=false`, otherwise in-container by `requireAdmin` |
| Transport | HTTPS (TLS 1.2+) terminated at Choreo edge |
| Input Validation | express-validator + `GROUP_BY_COLUMN_MAP.has()` guard on the one identifier interpolation in `/summary/historical` |
| Rate Limiting | express-rate-limit — default 1000 req / 15 min per real client IP; env-tunable via `RATE_LIMIT_MAX` / `RATE_LIMIT_WINDOW_MS`; runaway-loop safety net (Choreo gateway is the customer-facing quota) |
| Trusted Proxy | `expressApp.set('trust proxy', 1)` — `req.ip` derived from `X-Forwarded-For` set by Choreo's one-hop gateway |
| Log Injection | `X-Correlation-Id` header validated against `^[A-Za-z0-9._:-]{1,128}$`; malformed values silently dropped and WARN-logged (IP only, not the payload) |
| HTTP Headers | helmet.js (CSP, HSTS, X-Frame-Options, etc.) |
| Error Body | Generic error strings only; internal detail (stacks, driver text, hostnames) stays in the log |
| SQL | Every query parameterised (`$1, $2, …`); the one column-identifier interpolation is `.has()`-guarded against a Map allow-list |

---

## 11. Trust Boundary

### 11.1 Network model

```
   ┌────────────┐   HTTPS + OAuth 2.0    ┌───────────────┐   internal HTTP   ┌────────────────┐
   │ ServiceNow │  ────────────────────▶ │ Choreo gateway│ ────────────────▶ │ MTTR container │
   └────────────┘  (Client Credentials)  └───────────────┘  (already-authed) └────────────────┘
     external              │                    │                                    │
                           ▼                    ▼                                    ▼
                     TLS termination      Validates JWT           App code runs with AUTH_ENABLED=false
                     Rate-limit at edge   against Choreo STS      Trusts req has already been authenticated
```

### 11.2 Enforcement

The Choreo managed gateway is the **only** ingress path to the container. The pod exposes port 3000 on the Choreo internal network only — there is no NodePort, no LoadBalancer, no ClusterIP path that a caller outside Choreo can reach. Every request that arrives at `src/index.js` has already transited:

1. **Choreo edge** — TLS termination, edge rate-limits, DDoS protection.
2. **Choreo gateway** — OAuth 2.0 validation against Choreo STS per the `security: - BearerAuth: []` declaration in `openapi.yaml`. The gateway strips the `Authorization` header before forwarding to the container.

The enforcement is declared in three places:

| Where | What it declares |
|---|---|
| `.choreo/component.yaml` | `networkVisibilities: [Public]` — the *gateway* is publicly reachable (SN is an external caller). The container is not. Comment block at top of file spells this out. |
| `openapi.yaml` global `security` | Bearer JWT required on every route except `/health` (which carries `security: []`). Choreo synthesises the gateway config from this. |
| `src/middleware/auth.js` | `requireAdmin` short-circuits when `AUTH_ENABLED=false` — the container trusts the gateway has already validated + role-checked. `requireAdmin` at the route level is present for defence-in-depth when running outside Choreo (where the flag is on). |

### 11.3 Consequence for `AUTH_ENABLED=false` in production

Double-validating the JWT inside the container would add a second JWKS fetch and a second signature check for zero incremental security benefit — the trust boundary above guarantees the container never sees an unauthenticated request. `AUTH_ENABLED=true` remains supported for non-Choreo deployments (dev containers, bare-metal) where the app is exposed without a gateway.

### 11.4 What happens if the trust boundary is violated

If someone reaches the container directly (e.g. a hypothetical mis-routed ingress, an in-cluster debug session with port-forward, a Choreo network-policy regression), every `/admin/*` and `/cases/bulk-import` route becomes open — `requireAdmin` short-circuits to a mock admin when `auth.enabled=false`. **Choreo's network policy is therefore a load-bearing security control**, not an optimisation. Any change to `.choreo/component.yaml`'s `networkVisibilities` or to the OpenAPI `security` block must be treated as a security change and reviewed accordingly.

### 11.5 Observability of that boundary

- Correlation IDs are minted at the container edge and echoed on every response header + error body, so a caller reporting an issue can quote one ID that reappears in every log line for that request.
- Correlation-ID rejections are logged at WARN (with client IP, deliberately not the malformed value) so log-injection attempts are visible in the log aggregator.
- Cache-miss aggregations run on the request thread carry the requester's correlation ID; the nightly cron mints its own `cron_<uuid>` prefix so unattended runs remain traceable.

---

## 12. Monitoring

- **Health endpoint** (`/api/v1/health`): Exposes total cases, last ingestion time, last aggregation time, cache entries.
- **SN-side monitoring**: The Scheduled Job logs warnings to the SN system log if push fails.
- **Data freshness**: All MTTR responses include `calculated_at` so consumers can verify freshness.
- **Ingestion audit**: All batches logged with insert/update/reject counts (`GET /api/v1/admin/ingestion-logs`).
- **Correlation IDs**: Every log line carries a `correlationId` field; grep by that field to reconstruct a single request or a single cron run.

---

## 13. Deployment

- **Platform**: WSO2 Choreo (Docker-based Node.js component)
- **Database**: PostgreSQL
- **CI/CD**: Git push to Choreo-linked repository triggers build and deploy
- **Configuration**: Environment variables injected via Choreo console