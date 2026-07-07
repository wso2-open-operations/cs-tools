# MTTR Application — Setup Guide

> **Audience**: Engineering team members setting up the MTTR system end-to-end (local dev → Choreo → ServiceNow).

---

## Table of Contents

1. [Overview](#1-overview)
2. [Prerequisites](#2-prerequisites)
3. [Architecture Summary](#3-architecture-summary)
4. [Phase 1 — Local Development Setup](#4-phase-1--local-development-setup)
5. [Phase 2 — Choreo Deployment (Production)](#5-phase-2--choreo-deployment-production)
6. [Phase 3 — ServiceNow Configuration](#6-phase-3--servicenow-configuration)
7. [Phase 4 — Verification & Smoke Tests](#7-phase-4--verification--smoke-tests)
8. [Environment Variable Reference](#8-environment-variable-reference)
9. [API Endpoint Reference](#9-api-endpoint-reference)
10. [Troubleshooting](#10-troubleshooting)
11. [Maintenance & Operations](#11-maintenance--operations)
12. [Verification Checklist](#12-verification-checklist)

---

## 1. Overview

The MTTR (Mean Time to Resolve) application calculates P95 truncated-average resolution times for Customer Service cases. The system has three layers:

| Layer | Technology | Purpose |
|---|---|---|
| **Data Source** | ServiceNow (Scheduled Job) | Pushes resolved/closed case data every 10 minutes |
| **Backend API** | Node.js/Express on WSO2 Choreo | Ingests data, computes MTTR metrics, exposes REST API |
| **Presentation** | ServiceNow Homepage Widgets | Visualises MTTR across teams, priorities, trends |

---

## 2. Prerequisites

### Tools Required

| Tool | Version | Purpose |
|---|---|---|
| **Node.js** | ≥ 18.x | Runtime |
| **npm** | ≥ 9.x | Package manager |
| **PostgreSQL** | ≥ 14.x | Database |
| **Git** | Any recent | Source control |
| **Docker** (optional) | ≥ 20.x | Container build/test |

### Access Required

| System | Required Access |
|---|---|
| **GitHub** | Read/write to the MTTR_Application repository |
| **WSO2 Choreo** | Project admin to create services, provision DB, configure env vars |
| **ServiceNow** | Admin access to create REST Messages, Scheduled Jobs, System Properties, Script Includes, UI Pages, and Homepage widgets |

---

## 3. Architecture Summary

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         ServiceNow Instance                             │
│                                                                         │
│  ┌─────────────────-─┐   ┌──────────────────────┐   ┌────────────────┐  │
│  │ Scheduled Job     │   │ MttrDashboardAPI     │   │ 12 Homepage    │  │
│  │ (every 10 min)    │   │ (Script Include)     │   │ Widgets        │  │
│  │                   │   │                      │   │ (Jelly/Google  │  │
│  │ Queries cases     │   │ Fetches MTTR data    │   │  Charts)       │  │
│  │ POSTs to Choreo   │   │ from Choreo API      │   │                │  │
│  └────────┬──────────┘   └──────────┬───────────┘   └───────┬────────┘  │
│           │                         │                       │           │
└───────────┼─────────────────────────┼───────────────────────┼───────────┘
            │                         │                       │
            ▼                         ▼                       │
┌─────────────────────────────────────────────────────────────┼───────────┐
│                     WSO2 Choreo (Cloud)                     │           │
│                                                             │           │
│  ┌─────────────────────────────────────────────────────┐    │           │
│  │           Choreo API Gateway (OAuth 2.0)            │◄───┘           │
│  └──────────────────────┬──────────────────────────────┘                │
│                         │                                               │
│  ┌──────────────────────▼──────────────────────────────┐                │
│  │              Node.js Express API                    │                │
│  │                                                     │                │
│  │  /api/v1/cases/batch      → Ingestion               │                │
│  │  /api/v1/cases/bulk-import → Backfill               │                │
│  │  /api/v1/mttr             → Query cached MTTR       │                │
│  │  /api/v1/mttr/types       → List dimensions         │                │
│  │  /api/v1/admin/*          → Cache reset, retention  │                │
│  │  /api/v1/summary/*        → Historical quarterly    │                │
│  │  /api/v1/health           → Health check            │                │
│  │                                                     │                │
│  │  ┌──────────────────────────────────────────────┐   │                │
│  │  │  Cron Job (daily midnight)                   │   │                │
│  │  │  → Recompute all MTTR dimensions             │   │                │
│  │  │  → Purge stale cache                         │   │                │
│  │  │  → Quarterly archival + retention cleanup    │   │                │
│  │  └──────────────────────────────────────────────┘   │                │
│  └──────────────────────┬──────────────────────────────┘                │
│                         │                                               │
│  ┌──────────────────────▼──────────────────────────────┐                │
│  │              PostgreSQL Database                    │                │
│  │                                                     │                │
│  │  case_events          (raw ingested data)           │                │
│  │  mttr_cache           (pre-computed aggregations)   │                │
│  │  ingestion_log        (audit trail)                 │                │
│  │  case_events_summary  (archived quarterly data)     │                │
│  └─────────────────────────────────────────────────────┘                │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Phase 1 — Local Development Setup

### Step 1: Clone the Repository

```bash
git clone <repository-url> MTTR_Application
cd MTTR_Application
```

### Step 2: Install Dependencies

```bash
npm install
```

This installs all production and dev dependencies defined in `package.json`:

| Package | Purpose |
|---|---|
| `express` | HTTP server |
| `pg` | PostgreSQL client |
| `node-cron` | Scheduled aggregation job |
| `helmet` | HTTP security headers |
| `cors` | Cross-origin resource sharing |
| `express-rate-limit` | API rate limiting |
| `jsonwebtoken` | JWT verification |
| `jwks-rsa` | JWKS key retrieval |
| `dotenv` | Environment variable loading |
| `winston` | Structured logging |
| `express-validator` | Request body validation |
| `uuid` | Unique ID generation |

### Step 3: Set Up PostgreSQL

Create the database and user:

```bash
# Connect to PostgreSQL as superuser
psql -U postgres
```

```sql
CREATE USER mttr_user WITH PASSWORD 'your_secure_password';
CREATE DATABASE mttr_db OWNER mttr_user;
GRANT ALL PRIVILEGES ON DATABASE mttr_db TO mttr_user;
\q
```

### Step 4: Configure Environment Variables

Create a `.env` file in the project root:

```bash
cp .env.example .env   # or create manually
```

```env
# ─── Application ──────────────────────────────────────
NODE_ENV=development
PORT=3000
LOG_LEVEL=debug

# ─── PostgreSQL ───────────────────────────────────────
DB_HOST=localhost
DB_PORT=5432
DB_NAME=mttr_db
DB_USER=mttr_user
DB_PASSWORD=your_secure_password
DB_SSL=false
DB_POOL_MIN=2
DB_POOL_MAX=10

# ─── Authentication ──────────────────────────────────
# Set to false for local dev (bypasses JWT verification, injects mock admin user)
AUTH_ENABLED=false

# ─── Aggregation ──────────────────────────────────────
AGGREGATION_CRON=0 0 * * *
AGGREGATION_ROLLING_MONTHS=12
MIN_SAMPLE_SIZE=20

# ─── Retention ────────────────────────────────────────
RETENTION_INGESTION_LOG_DAYS=14
RETENTION_CASE_EVENTS_MONTHS=24

# ─── Rate Limiting ───────────────────────────────────
RATE_LIMIT_WINDOW_MS=900000
RATE_LIMIT_MAX=100
```

> **Note**: Never commit the `.env` file. It should already be in `.gitignore`.

### Step 5: Initialize the Database Schema

```bash
npm run db:init
```

This runs `sql/init.sql` against your configured database and creates:

- **`case_events`** — Raw case data ingested from ServiceNow
- **`mttr_cache`** — Pre-computed MTTR aggregations
- **`ingestion_log`** — Audit trail for batch ingestion operations
- **`case_events_summary`** — Archived quarterly MTTR summaries

All indexes and unique constraints are created automatically.

### Step 6: Start the Application

```bash
# Development mode (auto-restart on file changes)
npm run dev

# Or production mode
npm start
```

The application starts on `http://localhost:3000`.

### Step 7: Verify the Setup

```bash
# Health check
curl http://localhost:3000/api/v1/health

# Expected response:
# {
#   "status": "healthy",
#   "db_time": "2026-04-07T...",
#   "total_cases": 0,
#   "last_ingestion_at": null,
#   "last_aggregation_at": null,
#   "cache_entries": 0
# }
```

### Step 8: Test Ingestion with Sample Data

```bash
curl -X POST http://localhost:3000/api/v1/cases/batch \
  -H "Content-Type: application/json" \
  -d '{
    "batch_id": "test_001",
    "cases": [
      {
        "case_sys_id": "abc123def456",
        "product": "WSO2 API Manager",
        "cs_team": "Cloud",
        "business_duration_ms": 3600000,
        "created_date": "2026-03-01T10:00:00Z",
        "closed_date": "2026-03-02T10:00:00Z",
        "case_type": "Incident",
        "priority": "P2",
        "is_patched": false,
        "case_state": "Closed"
      }
    ]
  }'
```

### Step 9: Verify the API is up

```bash
curl http://localhost:3000/api/v1/health
```

---

## 5. Phase 2 — Choreo Deployment (Production)

### Step 1: Push Code to GitHub

Ensure all code is committed and pushed to a GitHub repository accessible by Choreo.

```bash
git add .
git commit -m "MTTR App ready for Choreo deployment"
git push origin main
```

### Step 2: Create a Choreo Project

1. Log in to [Choreo Console](https://console.choreo.dev/)
2. Create a new **Project** (e.g., `mttr`)
3. Under the project, create a new **Service** component:
   - **Name**: `mttr-app` (or similar)
   - **Type**: Service
   - **Source**: Connect your GitHub repository
   - **Branch**: `main`
   - **Build Pack**: Dockerfile
   - **Dockerfile Path**: `./Dockerfile`
   - **Port**: `3000`

### Step 3: Provision PostgreSQL

You have three options:

#### Option A: Choreo Managed PostgreSQL (Recommended)
1. In the Choreo project, go to **Dependencies** → **Databases**
2. Create a new PostgreSQL instance
3. Choreo will inject `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD` as environment variables

#### Option B: External Managed PostgreSQL
Use a managed PostgreSQL provider (Neon, Supabase, Aiven, AWS RDS, etc.):
1. Create a PostgreSQL database on your provider
2. Note the connection credentials
3. Configure them as environment variables in Choreo (Step 4)

#### Option C: Self-Hosted PostgreSQL
If you manage your own PostgreSQL server, ensure it's accessible from Choreo's network.

### Step 4: Configure Environment Variables in Choreo

In the Choreo component settings, add the following environment variables:

| Variable | Value | Notes |
|---|---|---|
| `NODE_ENV` | `production` | |
| `PORT` | `3000` | Must match Dockerfile EXPOSE |
| `DB_HOST` | `<your-db-host>` | From Step 3 |
| `DB_PORT` | `5432` | Default PostgreSQL port |
| `DB_NAME` | `mttr_db` | Your database name |
| `DB_USER` | `mttr_user` | Your database user |
| `DB_PASSWORD` | `<password>` | Mark as secret |
| `DB_SSL` | `true` | Required for most cloud DBs |
| `DB_POOL_MIN` | `2` | |
| `DB_POOL_MAX` | `10` | |
| `AUTH_ENABLED` | `false` | Choreo gateway handles auth |
| `AGGREGATION_CRON` | `0 0 * * *` | Daily at midnight |
| `AGGREGATION_ROLLING_MONTHS` | `12` | |
| `MIN_SAMPLE_SIZE` | `20` | |
| `RETENTION_INGESTION_LOG_DAYS` | `14` | |
| `RETENTION_CASE_EVENTS_MONTHS` | `24` | |
| `LOG_LEVEL` | `info` | |

> **Why `AUTH_ENABLED=false`?** Choreo's API Gateway validates the OAuth 2.0 token before the request reaches the Express app. The app doesn't need to re-verify the JWT. In local dev, setting this to `false` also injects a mock admin user for convenience.

### Step 5: Initialize the Database Schema on Choreo

After the first deployment, you need to run the database initialization once. Options:

**Option A**: Connect to the database directly and run `sql/init.sql`:
```bash
psql -h <db-host> -U <db-user> -d <db-name> -f sql/init.sql
```

**Option B**: If shell access is available via Choreo:
```bash
npm run db:init
```

**Option C**: Copy-paste the contents of `sql/init.sql` into your database admin tool (pgAdmin, DBeaver, Choreo DB console, etc.).

### Step 6: Build & Deploy

1. In Choreo, trigger a **Build** for the service component
2. Choreo builds the Docker image using the `Dockerfile`
3. Once built, **Deploy** to the target environment (Development → Production)
4. Choreo provisions the container with the configured environment variables

### Step 7: Verify Deployment

```bash
# Replace with your actual Choreo endpoint
CHOREO_URL="https://<your-choreo-endpoint>"

# Health check (no auth needed)
curl "$CHOREO_URL/api/v1/health"
```

### Step 8: Publish the API & Create an Application

1. In Choreo, go to **API Management** → **Lifecycle**
2. **Publish** the API to the Developer Portal
3. In the **Developer Portal**, create a new **Application**
4. Subscribe the application to the MTTR API
5. Under the application, go to **Production Keys** → **OAuth 2.0**
6. Generate **Client ID** and **Client Secret**
7. Note the **Token Endpoint** (typically `https://sts.choreo.dev/oauth2/token`)

These credentials will be used by ServiceNow to authenticate API calls.

### Step 9: Note the Deployed Endpoints

Record these URLs — they're needed for ServiceNow configuration:

| Purpose | URL |
|---|---|
| Health Check | `GET {CHOREO_URL}/api/v1/health` |
| Batch Ingestion | `POST {CHOREO_URL}/api/v1/cases/batch` |
| Bulk Import | `POST {CHOREO_URL}/api/v1/cases/bulk-import` |
| MTTR Query | `GET {CHOREO_URL}/api/v1/mttr?type={dimensionType}` |
| MTTR Types | `GET {CHOREO_URL}/api/v1/mttr/types` |
| Cache Reset | `POST {CHOREO_URL}/api/v1/admin/cache/reset` |
| Ingestion Logs | `GET {CHOREO_URL}/api/v1/admin/ingestion-logs` |
| Retention Run | `POST {CHOREO_URL}/api/v1/admin/retention/run` |
| Historical Summary | `GET {CHOREO_URL}/api/v1/summary/historical` |
| Summary Periods | `GET {CHOREO_URL}/api/v1/summary/periods` |

---

## 6. Phase 3 — ServiceNow Configuration

### Overview

ServiceNow needs four components configured:

1. **OAuth 2.0 profile** — Authentication against Choreo STS
2. **REST Message** — HTTP methods for all API calls
3. **System Property** — Watermark for incremental sync
4. **Scheduled Job** — Pushes resolved cases every 10 minutes
5. **Script Include** — Server-side API wrapper for widgets
6. **12 UI Page widgets** — Visualizations
7. **Homepage configuration** — Arrange widgets on a homepage

### Step 1: Create the OAuth 2.0 Application Registry

1. Navigate to **System OAuth** → **Application Registry**
2. Click **New** → **Connect to a third party OAuth Provider**
3. Configure:

| Field | Value |
|---|---|
| Name | `Choreo MTTR OAuth` |
| Client ID | *(from Choreo Step 8)* |
| Client Secret | *(from Choreo Step 8)* |
| Token URL | `https://sts.choreo.dev/oauth2/token` |
| Default Grant Type | `Client Credentials` |

4. Save

### Step 2: Create the REST Message

1. Navigate to **System Web Services** → **Outbound** → **REST Message**
2. Click **New**
3. Configure:

| Field | Value |
|---|---|
| Name | `MTTR_Choreo_API` |
| Endpoint | `https://<your-choreo-endpoint>` *(base URL only)* |
| Authentication | `OAuth 2.0` |
| OAuth profile | *(select the profile from Step 1)* |

4. Save

### Step 3: Create HTTP Methods on the REST Message

Create the following 6 HTTP Methods under the REST Message:

#### Method 1: POST_Batch

| Field | Value |
|---|---|
| Name | `POST_Batch` |
| HTTP Method | `POST` |
| Endpoint | `https://<choreo-url>/api/v1/cases/batch` |
| Content-Type | `application/json` |
| HTTP Request Header | `Content-Type: application/json` |

#### Method 2: POST_BulkImport

| Field | Value |
|---|---|
| Name | `POST_BulkImport` |
| HTTP Method | `POST` |
| Endpoint | `https://<choreo-url>/api/v1/cases/bulk-import` |
| Content-Type | `application/json` |
| HTTP Request Header | `Content-Type: application/json` |

#### Method 3: GET_MTTR

| Field | Value |
|---|---|
| Name | `GET_MTTR` |
| HTTP Method | `GET` |
| Endpoint | `https://<choreo-url>/api/v1/mttr?type=${type}` |

Add a **Variable Substitution**:

| Name | Test Value |
|---|---|
| `type` | `overall_by_type` |

#### Method 4: GET_Summary

| Field | Value |
|---|---|
| Name | `GET_Summary` |
| HTTP Method | `GET` |
| Endpoint | `https://<choreo-url>/api/v1/summary/historical?case_type=${case_type}&group_by=${group_by}` |

Add **Variable Substitutions**:

| Name | Test Value |
|---|---|
| `case_type` | `Incident` |
| `group_by` | `team` |

#### Method 5: GET_Health

| Field | Value |
|---|---|
| Name | `GET_Health` |
| HTTP Method | `GET` |
| Endpoint | `https://<choreo-url>/api/v1/health` |

#### Method 6: POST_CacheReset

| Field | Value |
|---|---|
| Name | `POST_CacheReset` |
| HTTP Method | `POST` |
| Endpoint | `https://<choreo-url>/api/v1/admin/cache/reset` |
| Content-Type | `application/json` |

### Step 4: Create the System Property

1. Navigate to **System Properties** → **All**
2. Click **New**
3. Configure:

| Field | Value |
|---|---|
| Name | `u_mttr_last_sync` |
| Value | *(leave empty — the scheduled job sets it automatically on first run)* |
| Description | `Watermark timestamp for MTTR data sync to Choreo. Updated automatically by the MTTR scheduled job. Do not modify manually unless troubleshooting.` |
| Type | `string` |

### Step 5: Create the Scheduled Job

1. Navigate to **System Definition** → **Scheduled Jobs**
2. Click **New**
3. Configure:

| Field | Value |
|---|---|
| Name | `MTTR - Push Resolved Cases` |
| Active | `true` |
| Run | `Periodically` |
| Repeat Interval | `10 minutes` |
| Run this script | *(paste the contents of `servicenow/scheduled-job-mttr-push.js`)* |

4. Verify the `ENDPOINT` and `HTTP_METHOD` variables in the script match your REST Message and Method names:
   ```javascript
   var ENDPOINT = 'MTTR_Choreo_API';      // Must match REST Message name
   var HTTP_METHOD = 'POST_Batch';         // Must match HTTP Method name
   ```

5. Save

### Step 6: Create the Script Include

1. Navigate to **System Definition** → **Script Includes**
2. Click **New**
3. Configure:

| Field | Value |
|---|---|
| Name | `MttrDashboardAPI` |
| API Name | `global.MttrDashboardAPI` |
| Client callable | `false` |
| Active | `true` |
| Script | *(paste the contents of `servicenow/script-include-mttr-dashboard.js`)* |

4. Save

### Step 7: Import the 12 Widget UI Pages

For each XML file in `servicenow/widgets/`, create a UI Page in ServiceNow:

1. Navigate to **System UI** → **UI Pages**
2. Click **New**
3. For each widget file:

| XML File | UI Page Name | Description |
|---|---|---|
| `mttr_widget_overall.xml` | `mttr_widget_overall` | Overall CS MTTR (KPI card) |
| `mttr_widget_incidents.xml` | `mttr_widget_incidents` | Incident MTTR (KPI card) |
| `mttr_widget_queries.xml` | `mttr_widget_queries` | Query MTTR (KPI card) |
| `mttr_widget_patched.xml` | `mttr_widget_patched` | Patched Issues (Donut pie) |
| `mttr_widget_non_patched.xml` | `mttr_widget_non_patched` | Non-Patched Issues (Donut pie) |
| `mttr_widget_priority_trend.xml` | `mttr_widget_priority_trend` | Monthly Priority Trend (Line) |
| `mttr_widget_team_casetype.xml` | `mttr_widget_team_casetype` | Team × Case Type (Bar) |
| `mttr_widget_team_trend.xml` | `mttr_widget_team_trend` | Monthly Team Trend (Line) |
| `mttr_widget_hist_overall.xml` | `mttr_widget_hist_overall` | Historical Overall (Quarterly) |
| `mttr_widget_hist_inc_priority.xml` | `mttr_widget_hist_inc_priority` | Historical Incidents by Priority |
| `mttr_widget_hist_inc_team.xml` | `mttr_widget_hist_inc_team` | Historical Incidents by Team |
| `mttr_widget_hist_qry_team.xml` | `mttr_widget_hist_qry_team` | Historical Queries by Team |

For each UI Page:
- Set the **Name** exactly as shown above
- Paste the XML content into the **HTML** field
- Set **Direct** to `true`
- Save

### Step 8: Create the Homepage Dashboard

1. Navigate to **Self-Service** → **Homepage Admin** (or **System UI** → **Homepages**)
2. Create a new **Homepage** or edit an existing one
3. Add **Renderer** widgets using the renderer script from `servicenow/widget-renderer-mttr.js`:

**Recommended layout** (3-column grid):

| Row | Column 1 | Column 2 | Column 3 |
|---|---|---|---|
| 1 | CS Overall MTTR | Overall Incident MTTR | Overall Query MTTR |
| 2 | Team × Case Type MTTR (colspan=3) | | |
| 3 | Patched Issues | Non-Patched Issues | Priority Trend |
| 4 | Monthly Team Trend (colspan=2) | | Monthly Priority Trend |
| 5 | Historical - Overall (colspan=3) | | |
| 6 | Historical - Incidents by Team | Historical - Incidents by Priority | Historical - Queries by Team |

4. Set the homepage **Active** and assign to the target group/role

### Step 9: Test the Scheduled Job

1. Navigate to the Scheduled Job created in Step 5
2. Click **Execute Now**
3. Check **System Logs** → **All** for entries starting with `MTTR Sync:`
4. Verify:
   - Cases are being queried
   - Batch is sent to Choreo
   - Watermark is advancing (`u_mttr_last_sync` property updates)
   - No HTTP errors in the logs

### Step 10: Verify Widgets

1. Navigate to the Homepage dashboard
2. Verify all 12 widgets render correctly
3. If widgets show "No data", this is expected if the first aggregation hasn't run yet
4. Either wait for the daily midnight cron or manually trigger aggregation:

```bash
curl -X POST "$CHOREO_URL/api/v1/admin/cache/reset" \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json"
```

---

## 7. Phase 4 — Verification & Smoke Tests

### Backend Smoke Tests

Run these after deployment to verify all endpoints:

```bash
BASE_URL="https://<your-choreo-endpoint>"

# 1. Health check
curl -s "$BASE_URL/api/v1/health" | jq .

# 2. List available dimension types
curl -s "$BASE_URL/api/v1/mttr/types" | jq .

# 3. Query MTTR by dimension
curl -s "$BASE_URL/api/v1/mttr?type=overall_by_type" | jq .

# 4. Check ingestion logs
curl -s "$BASE_URL/api/v1/admin/ingestion-logs?limit=5" | jq .

# 5. Check summary periods
curl -s "$BASE_URL/api/v1/summary/periods" | jq .
```

### ServiceNow Smoke Tests

1. **Scheduled Job**: Execute the job manually and check logs for `MTTR Sync:` entries
2. **REST Message**: Test each HTTP Method using the **Test** button on the REST Message form
3. **Script Include**: Run in a background script:
   ```javascript
   var api = new MttrDashboardAPI();
   var result = api.getOverallByType();
   gs.info(JSON.stringify(result));
   ```
4. **Widgets**: Navigate to the SN homepage and verify all charts render

### Diagnostic Script

If cases aren't syncing as expected, run the diagnostic script in a ServiceNow **Background Script** (System Definition → Scripts - Background):

Paste the contents of `servicenow-diagnostic-full.js` and execute. It will print:
- Total cases that match the query
- How many are valid for sync
- How many are skipped (with reasons and examples)

---

## 8. Environment Variable Reference

| Variable | Default | Description |
|---|---|---|
| `NODE_ENV` | `development` | `development` or `production` |
| `PORT` | `3000` | HTTP listen port |
| `LOG_LEVEL` | `info` | Winston log level (`debug`, `info`, `warn`, `error`) |
| `DB_HOST` | `localhost` | PostgreSQL hostname |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `mttr_db` | Database name |
| `DB_USER` | `mttr_user` | Database username |
| `DB_PASSWORD` | *(empty)* | Database password |
| `DB_SSL` | `false` | Set `true` for cloud-hosted DBs |
| `DB_POOL_MIN` | `2` | Minimum connection pool size |
| `DB_POOL_MAX` | `10` | Maximum connection pool size |
| `AUTH_ENABLED` | `false` | Enable JWT verification (`true`/`false`) |
| `AUTH_JWKS_URI` | *(empty)* | JWKS endpoint for public key retrieval |
| `AUTH_ISSUER` | *(empty)* | Expected JWT issuer |
| `AUTH_AUDIENCE` | *(empty)* | Expected JWT audience |
| `AGGREGATION_CRON` | `0 0 * * *` | Cron expression for aggregation job (default: daily midnight) |
| `AGGREGATION_ROLLING_MONTHS` | `12` | Rolling window for MTTR calculation |
| `MIN_SAMPLE_SIZE` | `20` | Minimum cases for P95 to be applied |
| `RETENTION_INGESTION_LOG_DAYS` | `14` | Days to keep ingestion logs |
| `RETENTION_CASE_EVENTS_MONTHS` | `24` | Months before raw cases are archived to quarterly summaries |
| `RATE_LIMIT_WINDOW_MS` | `900000` | Rate limit window (15 min default) |
| `RATE_LIMIT_MAX` | `100` | Max requests per window |
| `CORS_ALLOWED_ORIGINS` | *(empty)* | Comma-separated allowed origins (production only) |

---

## 9. API Endpoint Reference

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/health` | None | Health check with DB stats |
| `POST` | `/api/v1/cases/batch` | Required | Ingest 1–500 cases |
| `POST` | `/api/v1/cases/bulk-import` | Required | Ingest 1–5000 cases (backfill) |
| `GET` | `/api/v1/mttr?type={dim}` | Required | Query cached MTTR for dimension |
| `GET` | `/api/v1/mttr/types` | Required | List all valid dimension types |
| `POST` | `/api/v1/admin/cache/reset` | Admin | Recalculate MTTR (all or specific type) |
| `GET` | `/api/v1/admin/ingestion-logs` | Admin | View recent ingestion history |
| `POST` | `/api/v1/admin/retention/run` | Admin | Manually trigger retention job |
| `GET` | `/api/v1/summary/historical` | Required | Query archived quarterly summaries |
| `GET` | `/api/v1/summary/periods` | Required | List available quarterly periods |

### Valid Dimension Types

| Dimension Type | Description |
|---|---|
| `overall_by_type` | Overall MTTR by case type (Incident vs Query) |
| `team_incidents` | Team-level MTTR for Incidents |
| `team_queries` | Team-level MTTR for Queries |
| `team_incidents_priority` | Team + Priority breakdown for Incidents |
| `priority_incidents` | Priority-level MTTR for Incidents |
| `patched_incidents` | Patched vs Non-patched for Incidents |
| `patched_queries` | Patched vs Non-patched for Queries |
| `monthly_trend_team` | Monthly trend by team |
| `monthly_trend_type` | Monthly trend by case type |

---

## 10. Troubleshooting

### Health Check Returns "unhealthy"

- **Cause**: Database connection failed
- **Fix**: Verify `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD` are correct. Check `DB_SSL=true` for cloud DBs.

### Scheduled Job Logs "Push failed with HTTP 401"

- **Cause**: OAuth token expired or credentials invalid
- **Fix**: Verify OAuth profile (Client ID, Client Secret, Token URL) in the REST Message authentication settings. Test the token endpoint manually.

### Scheduled Job Logs "No new cases to push"

- **Cause**: No cases updated since the watermark
- **Fix**: Check `u_mttr_last_sync` system property. If it's ahead of all data, manually reset it to an earlier date.

### Widgets Show "No data available"

- **Cause**: MTTR cache is empty (aggregation hasn't run yet)
- **Fix**: Either wait for the midnight cron or manually reset the cache via the admin endpoint.

### Cases Not Appearing in Ingestion

- **Cause**: Cases are being skipped by the scheduled job's validation logic
- **Fix**: Run `servicenow-diagnostic-full.js` in a background script to see which cases are skipped and why.

### Common Skip Reasons

| Skip Reason | Meaning | Resolution |
|---|---|---|
| Wrong Case Type | `u_case_type` is not Incident or Query | Expected — these case types are out of scope |
| Invalid Duration | `business_duration` is 0 or null | Case has no SLA data — check SLA configuration |
| No Product | `u_wso2_product` is empty | Fill in the product field on the case |
| No Team | No IAM/Choreo product match and `account.u_integration_cs_team` is empty | Assign the account to an Integration CS Team |

### Database Schema Migration

If you modify `sql/init.sql`, the `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` statements are safe to re-run. They won't drop existing data.

---

## 11. Maintenance & Operations

### Daily Automated Tasks (Cron Job)

The aggregation job runs daily at midnight (configurable via `AGGREGATION_CRON`) and performs:

1. **Recompute all MTTR dimensions** — Recalculates P95 truncated average for all 9 dimension types across the 12-month rolling window
2. **Purge stale cache** — Removes old cache entries, keeping only the latest computation per dimension
3. **Run retention** — Summarises quarters older than 24 months into `case_events_summary`, deletes the raw rows, and purges ingestion logs older than 14 days

### Manual Cache Reset

If MTTR data appears stale after a large data import, trigger a manual recalculation:

```bash
# Recalculate all dimensions
curl -X POST "$CHOREO_URL/api/v1/admin/cache/reset" \
  -H "Authorization: Bearer <token>"

# Recalculate a specific dimension
curl -X POST "$CHOREO_URL/api/v1/admin/cache/reset?type=team_incidents" \
  -H "Authorization: Bearer <token>"
```

### Manual Retention Run

```bash
curl -X POST "$CHOREO_URL/api/v1/admin/retention/run" \
  -H "Authorization: Bearer <token>"
```

### Bulk Import (One-Time Backfill)

For initial data migration or backfilling historical data, use the bulk import endpoint (up to 5000 cases per request):

```bash
curl -X POST "$CHOREO_URL/api/v1/cases/bulk-import" \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"batch_id": "backfill_001", "cases": [...]}'
```

### Monitoring

- **Health endpoint**: `GET /api/v1/health` — Used by Choreo health probes (every 30s) and can be monitored externally
- **Ingestion logs**: `GET /api/v1/admin/ingestion-logs` — Check for rejected records
- **Application logs**: Available in Choreo's log viewer (structured JSON in production)

### Scaling Considerations

| Scenario | Action |
|---|---|
| High ingestion volume | Increase `DB_POOL_MAX`, consider read replicas |
| Slow aggregation | Verify indexes exist, consider reducing `AGGREGATION_ROLLING_MONTHS` |
| Rate limiting too aggressive | Increase `RATE_LIMIT_MAX` or `RATE_LIMIT_WINDOW_MS` |
| Widgets slow | Cache is pre-computed, so this shouldn't happen. Check DB connectivity. |

---

## 12. Verification Checklist

A condensed checklist to confirm each phase is complete.

**Phase 1 — Local Development**
- [ ] `node --version` ≥ 18
- [ ] PostgreSQL reachable and `npm run db:init` succeeded
- [ ] `npm start` runs without errors
- [ ] `GET /api/v1/health` returns `"status": "healthy"`
- [ ] Sample batch ingest returns `inserted: 1` (or more)
- [ ] `GET /api/v1/mttr?type=team_incidents` returns the expected MTTR

**Phase 2 — Choreo Deployment**
- [ ] Service component builds and deploys without errors
- [ ] PostgreSQL is provisioned and `init.sql` has been run against it
- [ ] All required env vars are set in the Choreo console
- [ ] Unauthenticated requests are rejected by the Choreo gateway (401)
- [ ] Client Credentials token from Choreo STS works with Bearer auth
- [ ] `AUTH_ENABLED=false` on the Choreo deployment (gateway handles auth)

**Phase 3 — ServiceNow**
- [ ] OAuth Application Registry created and tested
- [ ] REST Message `MTTR_Choreo_API` authenticates via Client Credentials
- [ ] All 6 HTTP Methods (`POST_Batch`, `POST_BulkImport`, `GET_MTTR`, `GET_Summary`, `GET_Health`, `POST_CacheReset`) tested
- [ ] System Property `u_mttr_last_sync` exists (initially empty)
- [ ] Scheduled Job `MTTR - Push Resolved Cases` is Active and runs every 10 min
- [ ] `u_mttr_last_sync` advances after each run
- [ ] `total_cases` in `/health` grows with each batch
- [ ] Script Include `MttrDashboardAPI` is Active
- [ ] All 12 UI Page widgets created and load without Jelly errors
- [ ] Homepage configured with the widgets and visible to the target group/role

---

## Appendix A — Docker Build & Run (Local)

```bash
# Build the image
docker build -t mttr-app .

# Run with environment variables
docker run -d \
  --name mttr-app \
  -p 3000:3000 \
  -e NODE_ENV=production \
  -e DB_HOST=host.docker.internal \
  -e DB_PORT=5432 \
  -e DB_NAME=mttr_db \
  -e DB_USER=mttr_user \
  -e DB_PASSWORD=your_password \
  -e AUTH_ENABLED=false \
  mttr-app
```

## Appendix B — File Inventory

```
MTTR_Application/
├── Dockerfile                              # Container definition
├── openapi.yaml                            # OpenAPI 3.0.3 spec
├── package.json                            # Dependencies & scripts
├── servicenow-diagnostic-full.js           # Debug tool for SN
├── docs/
│   ├── design-document.md                  # Architecture & design decisions
│   ├── implementation-guide.md             # Technical implementation details
│   ├── logic-and-architecture.md           # Logic, reasoning & execution layers
│   └── setup-guide.md                      # This document (end-to-end setup)
├── servicenow/
│   ├── rest-message-setup.js               # REST Message config reference
│   ├── scheduled-job-mttr-push.js          # SN Scheduled Job script
│   ├── script-include-mttr-dashboard.js    # SN Script Include
│   ├── widget-renderer-mttr.js             # Homepage renderer script
│   └── widgets/                            # 12 Jelly/Google Charts UI Pages
├── sql/
│   └── init.sql                            # Database schema
└── src/
    ├── index.js                            # Express app entry point
    ├── config/
    │   ├── database.js                     # PostgreSQL pool & helpers
    │   └── index.js                        # Configuration from env vars
    ├── jobs/
    │   └── aggregationJob.js               # Cron job orchestrator
    ├── middleware/
    │   └── auth.js                         # JWT auth + admin check
    ├── routes/
    │   ├── admin.js                        # Admin endpoints
    │   ├── cases.js                        # Ingestion endpoints
    │   ├── health.js                       # Health check
    │   ├── mttr.js                         # MTTR query endpoints
    │   └── summary.js                      # Historical summary endpoints
    ├── scripts/
    │   └── initDb.js                       # DB schema initialization
    ├── services/
    │   ├── aggregationService.js           # MTTR computation engine
    │   ├── cacheService.js                 # Cache read/write
    │   ├── ingestionService.js             # Case validation & UPSERT
    │   └── retentionService.js             # Data retention & archival
    └── utils/
        └── logger.js                       # Winston logging setup
```
