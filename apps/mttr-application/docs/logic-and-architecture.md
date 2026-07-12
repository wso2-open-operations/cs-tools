# MTTR Application — Logic, Architecture & Process Execution Layers

> **Version**: 1.0.0  
> **Last updated**: April 2026  
> **Audience**: Engineering team members who need to understand the "why" behind every decision

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Why P95 Truncated Average](#2-why-p95-truncated-average)
3. [System Architecture Layers](#3-system-architecture-layers)
4. [Process Execution Layers](#4-process-execution-layers)
5. [Data Ingestion Logic](#5-data-ingestion-logic)
6. [MTTR Computation Logic](#6-mttr-computation-logic)
7. [Aggregation Dimensions — What & Why](#7-aggregation-dimensions--what--why)
8. [Caching Strategy](#8-caching-strategy)
9. [Data Retention & Archival Logic](#9-data-retention--archival-logic)
10. [Security Model](#10-security-model)
11. [ServiceNow Integration Logic](#11-servicenow-integration-logic)
12. [Visualization Logic](#12-visualization-logic)
13. [Design Decision Log](#13-design-decision-log)

---

## 1. Problem Statement

### The Need

Customer Support teams need to measure how long it takes to resolve cases — the **Mean Time to Resolve (MTTR)**. This metric is critical for:

- **SLA management**: Ensuring cases are resolved within contractual timelines
- **Team performance tracking**: Comparing resolution times across teams
- **Trend monitoring**: Detecting degradation or improvement over time
- **Priority analysis**: Understanding how severity impacts resolution speed
- **Patched vs non-patched analysis**: Measuring the impact of fix availability on resolution

### Why Not Use ServiceNow Reporting Directly?

| Limitation | Impact |
|---|---|
| SN Performance Analytics is expensive and limited to basic aggregations | Cannot compute P95 truncated averages |
| SN reporting calculates simple averages, which are easily skewed by outliers | A single 90-day case can make the average misleading |
| Cross-team comparisons require complex report configurations | No single dashboard view |
| Historical trend analysis is limited | Cannot track quarterly trends over multiple years |
| Real-time recalculation on every page load doesn't scale | Slow dashboard for large case volumes |

### The Solution

Build an external analytics service that:
1. **Pulls** resolved case data from ServiceNow incrementally
2. **Computes** statistically robust MTTR metrics (P95 truncated average)
3. **Pre-caches** results for fast retrieval
4. **Pushes** visualizations back into ServiceNow dashboards
5. **Archives** historical data with quarterly rollups

---

## 2. Why P95 Truncated Average

### The Problem with Simple Averages

Consider a team that resolved 100 cases:
- 95 cases resolved in 2–10 hours
- 5 cases took 500+ hours due to customer delays, complex bugs, or awaiting vendor patches

A **simple average** would be ~30 hours — misrepresenting the team's actual performance on typical cases.

### Why Not Median?

The median (P50) ignores too much data. It tells you the midpoint but doesn't reflect the experience of customers in the slower half.

### Why P95 Specifically?

The **95th percentile truncated average** (P95-TA) gives a balanced view:

1. **Sort** all durations ascending
2. **Cut off** the top 5% (outliers — extreme cases, customer-caused delays, blocked-on-vendor)
3. **Average** the remaining 95%

This produces a value that:
- Represents what "most" customers experience
- Isn't distorted by a handful of extreme cases
- Is more stable than median across small sample sizes
- Is an industry-standard approach used by SRE/operations teams (similar to how AWS, Google, and other cloud providers report latency metrics)

### The Algorithm (Code Reference)

```
function computeTruncatedMTTR(durations):
    if durations is empty → return null
    
    sort durations ascending
    n = length of durations
    simpleAvg = sum(durations) / n
    
    if n < MIN_SAMPLE (default 20):
        → return simpleAvg with min_sample_met = false
        → Reason: With < 20 cases, P95 exclusion removes 0–1 cases,
          which doesn't meaningfully filter outliers. The simple average
          is more honest for small samples.
    
    p95Index = ceil(n × 0.95) - 1
    p95Cutoff = durations[p95Index]
    included = durations[0 .. p95Index]    // first 95%
    truncatedAvg = sum(included) / length(included)
    
    return {
        total_cases:     n,
        excluded_cases:  n - length(included),
        p95_cutoff_ms:   p95Cutoff,          // "anything above this was excluded"
        truncated_avg_ms: truncatedAvg,      // THE MTTR VALUE
        simple_avg_ms:    simpleAvg,          // for comparison
        min_sample_met:   true
    }
```

### Why `min_sample_met` Flag?

When a team or dimension has fewer than 20 cases (configurable via `MIN_SAMPLE_SIZE`), the P95 calculation would exclude 0 or 1 case — not enough to meaningfully remove outliers. Instead of silently reporting a potentially misleading P95 value, the system:

1. Falls back to a **simple average**
2. Sets `min_sample_met = false`
3. The UI displays a warning indicator so viewers know the metric isn't statistically robust

This prevents small teams or new dimensions from showing artificially precise numbers.

---

## 3. System Architecture Layers

The system is composed of three distinct layers, each with a specific responsibility:

```
┌────────────────────────────────────────────────────────────────────┐
│                     LAYER 1: DATA SOURCE                           │
│                     (ServiceNow Instance)                          │
│                                                                    │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │ Scheduled Job (every 10 minutes)                           │    │
│  │                                                            │    │
│  │ 1. Read watermark (u_mttr_last_sync property)              │    │
│  │ 2. Query sn_customerservice_case WHERE:                    │    │
│  │    - state IN (Closed, Resolved)                           │    │
│  │    - sys_updated_on > watermark                            │    │
│  │    - has account, product, business_duration               │    │
│  │    - created_on >= 2024-01-01                              │    │
│  │    - case_type IN (Incident, Query)                        │    │
│  │ 3. Validate & normalize each case                          │    │
│  │ 4. POST batch to Choreo                                    │    │
│  │ 5. Advance watermark on success                            │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                    │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │ Script Include (MttrDashboardAPI)                          │    │
│  │ 12 Homepage Widgets (UI Pages with Jelly + Google Charts)  │    │
│  │                                                            │    │
│  │ Fetches pre-computed MTTR from Choreo API, transforms      │    │
│  │ data for Google Visualization API rendering                │    │
│  └────────────────────────────────────────────────────────────┘    │
└────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────────┐
│                     LAYER 2: COMPUTE & API                         │
│                     (WSO2 Choreo — Node.js/Express)                │
│                                                                    │
│  ┌─── Ingestion Pipeline ─────────────────────────────────────┐    │
│  │ /cases/batch         → Validate → UPSERT case_events       │    │
│  │ /cases/bulk-import   → Validate → UPSERT case_events       │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                    │
│  ┌─── Aggregation Engine (daily cron) ────────────────────────┐    │
│  │ For each dimension type:                                   │    │
│  │   Query case_events → Group → P95 Truncated Mean           │    │
│  │   → Write to mttr_cache                                    │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                    │
│  ┌─── Query Layer ────────────────────────────────────────────┐    │
│  │ /mttr?type={dim}     → Read from mttr_cache                │    │
│  │ /mttr/types          → List all dimension types            │    │
│  │ /summary/historical  → Read from case_events_summary       │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                    │
│  ┌─── Retention Engine (daily cron) ──────────────────────────┐    │
│  │ Summarise → Archive → Delete aged raw data                 │    │
│  │ Purge stale cache entries                                  │    │
│  │ Purge old ingestion logs                                   │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                    │
│  ┌─── Administration ───────────────────────────────────────-─┐    │
│  │ /admin/cache/reset    → Trigger on-demand recalculation    │    │
│  │ /admin/ingestion-logs → View batch history                 │    │
│  │ /admin/retention/run  → Manually trigger retention         │    │
│  └────────────────────────────────────────────────────────────┘    │
└────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────────┐
│                     LAYER 3: STORAGE                               │
│                     (PostgreSQL)                                   │
│                                                                    │
│  ┌─── case_events ────────────────────────────────────────────┐    │
│  │ Raw case data. One row per resolved/closed case.           │    │
│  │ UPSERTed by case_sys_id (idempotent).                      │    │
│  │ Retained for RETENTION_CASE_EVENTS_MONTHS (24 months).     │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                    │
│  ┌─── mttr_cache ─────────────────────────────────────────────┐    │
│  │ Pre-computed MTTR for each dimension.                      │    │
│  │ Refreshed daily by aggregation cron.                       │    │
│  │ Stale entries purged after each recalculation.             │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                    │
│  ┌─── case_events_summary ────────────────────────────────────┐    │
│  │ Archived quarterly MTTR summaries.                         │    │
│  │ Survives after raw case_events are deleted.                │    │
│  │ Powers the /summary/historical endpoint.                   │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                    │
│  ┌─── ingestion_log ──────────────────────────────────────────┐    │
│  │ Audit trail of every batch ingestion.                      │    │
│  │ Purged after RETENTION_INGESTION_LOG_DAYS (14 days).       │    │
│  └────────────────────────────────────────────────────────────┘    │
└────────────────────────────────────────────────────────────────────┘
```

### Why This Separation?

| Concern | Reasoning |
|---|---|
| **ServiceNow shouldn't compute P95** | SN's server-side scripting (Rhino/ES5) lacks efficient array operations. Processing 10,000+ durations per dimension would hit execution time limits. |
| **External compute (Choreo) is stateless** | The Node.js app has no local state. It reads/writes to PostgreSQL. If the container restarts or scales, nothing is lost. |
| **PostgreSQL handles the heavy lifting** | Indexed queries across 100K+ rows with GROUP BY operations are PostgreSQL's strength. The app delegates grouping and filtering to SQL where possible. |
| **Pre-computed cache avoids real-time recalculation** | MTTR doesn't change every second. Computing once daily and serving from cache makes every dashboard load instant (<100ms) regardless of data volume. |

---

## 4. Process Execution Layers

This section describes every automated and manual process in the system, in the order they execute.

### 4.1 Process Layer 1 — Data Extraction (ServiceNow → Choreo)

```
    ┌─────────────────────────────────────────────────────────┐
    │              SCHEDULED JOB (every 10 minutes)           │
    │                                                         │
    │  START                                                  │
    │    │                                                    │
    │    ▼                                                    │
    │  Read watermark from system property                    │
    │  (u_mttr_last_sync)                                     │
    │    │                                                    │
    │    ▼                                                    │
    │  Query sn_customerservice_case                          │
    │  WHERE state IN (3,6)                                   │
    │    AND sys_updated_on > watermark                       │
    │    AND account IS NOT NULL                              │
    │    AND project IS NOT NULL                              │
    │    AND business_duration IS NOT NULL                    │
    │    AND sys_created_on >= '2024-01-01'                   │
    │    AND u_case_type IN (Incident, Query)                 │
    │  ORDER BY sys_updated_on ASC                            │
    │  LIMIT 500                                              │
    │    │                                                    │
    │    ▼                                                    │
    │  For each case record:                                  │
    │    │                                                    │
    │    ├── Track max sys_updated_on (new watermark)         │
    │    │                                                    │
    │    ├── Validate case_type is Incident or Query          │
    │    │   └── Skip if not ────────────────────┐            │
    │    │                                       │            │
    │    ├── Validate product is not empty       │            │
    │    │   └── Skip if empty ──────────────────┤            │
    │    │                                       │            │
    │    ├── Determine cs_team:                  │            │
    │    │   ├── If product contains "identity"  │            │
    │    │   │   or "asgardeo" → "IAM"           ▼            │
    │    │   ├── If product contains "choreo"   SKIPPED       │
    │    │   │   → "Choreo"                    CASES          │
    │    │   └── Else → account.u_integration    (logged      │
    │    │       _cs_team                        with         │
    │    │   └── Skip if no team ────────────── reason)       │
    │    │                                                    │
    │    ├── Validate business_duration > 0                   │
    │    │   └── Skip if invalid ────────────────┘            │
    │    │                                                    │
    │    ├── Derive is_patched:                               │
    │    │   u_fix_eta_shared is populated → true             │
    │    │   u_fix_eta_shared is empty → false                │
    │    │                                                    │
    │    ├── Normalize priority:                              │
    │    │   Incident → extract P1/P2/P3/P4                   │
    │    │   Query → empty string (no priority)               │
    │    │                                                    │
    │    └── Add to cases[] batch array                       │
    │                                                         │
    │  If cases[] is empty:                                   │
    │    Log skip statistics, advance watermark, EXIT         │
    │                                                         │
    │  POST /api/v1/cases/batch with:                         │
    │    { batch_id, cases: [...] }                           │
    │    │                                                    │
    │    ├── HTTP 200 → Advance watermark, log success        │
    │    └── HTTP !200 → Do NOT advance watermark             │
    │                    (next run retries same cases)        │
    │                                                         │
    │  END                                                    │
    └─────────────────────────────────────────────────────────┘
```

**Key Design Decisions:**

| Decision | Reasoning |
|---|---|
| **Watermark-based incremental sync** | Only processes cases updated since the last successful sync. Avoids re-processing the entire dataset every 10 minutes. |
| **500 case batch limit** | Stays within ServiceNow's scripting execution time limits and REST message payload constraints. |
| **Watermark advances even when 0 valid cases** | Prevents re-querying the same invalid cases forever. The watermark is based on `sys_updated_on`, not case validity. |
| **Watermark does NOT advance on HTTP failure** | Guarantees at-least-once delivery. If Choreo is down, the same batch is retried on the next run. |
| **Product-based team override (IAM, Choreo)** | Some products should always belong to a specific team regardless of how the account is classified. This provides accurate team attribution. |
| **is_patched derived from u_fix_eta_shared** | If a fix ETA date exists, the case is considered patched. This is a proxy indicator — if a release date is set, a fix was delivered. |
| **Priority empty for Queries** | Query-type cases don't have a meaningful severity classification. Sending an empty string avoids mixing priority analysis with non-priority data. |

### 4.2 Process Layer 2 — Data Ingestion (Choreo receives data)

```
    ┌─────────────────────────────────────────────────────────┐
    │        INGESTION ENDPOINT (/cases/batch)                │
    │                                                         │
    │  Receive POST request                                   │
    │    │                                                    │
    │    ▼                                                    │
    │  Express-validator checks:                              │
    │    - batch_id: non-empty string                         │
    │    - cases: array with 1–500 elements                   │
    │    │                                                    │
    │    ▼                                                    │
    │  For each case in the array:                            │
    │    │                                                    │
    │    ▼                                                    │
    │  validateCase(caseRecord):                              │
    │    - case_sys_id: not empty, ≤ 32 chars                 │
    │    - business_duration_ms: > 0 (number)                 │
    │    - created_date: valid date string                    │
    │    - case_type: "Incident" or "Query"                   │
    │    - case_state: not empty                              │
    │    - product: not empty                                 │
    │    - cs_team: not empty                                 │
    │    - priority: required if case_type = "Incident"       │
    │    │                                                    │
    │    ├── Valid → proceed to UPSERT                        │
    │    └── Invalid → add to rejected_details                │
    │                                                         │
    │  BEGIN TRANSACTION                                      │
    │    │                                                    │
    │    ▼                                                    │
    │  For each valid case:                                   │
    │    INSERT INTO case_events (...)                        │
    │    ON CONFLICT (case_sys_id)                            │
    │    DO UPDATE SET                                        │
    │      product = EXCLUDED.product,                        │
    │      cs_team = EXCLUDED.cs_team,                        │
    │      business_duration_ms = EXCLUDED.business_duration, │
    │      ... (all fields),                                  │
    │      updated_at = NOW()                                 │
    │    │                                                    │
    │    ├── xmax = 0 → INSERT (new case)                     │
    │    └── xmax ≠ 0 → UPDATE (case re-sent with changes)    │
    │                                                         │
    │  INSERT INTO ingestion_log:                             │
    │    batch_id, received, inserted, updated, rejected      │
    │                                                         │
    │  COMMIT TRANSACTION                                     │
    │    │                                                    │
    │    ▼                                                    │
    │  Return response:                                       │
    │  {                                                      │
    │    batch_id, received, inserted,                        │
    │    updated, rejected, rejected_details                  │
    │  }                                                      │
    └─────────────────────────────────────────────────────────┘
```

**Key Design Decisions:**

| Decision | Reasoning |
|---|---|
| **UPSERT (INSERT ON CONFLICT UPDATE)** | Cases can be re-sent when updated in SN (e.g., business_duration changes after closure). UPSERT ensures the latest data always wins without duplicates. |
| **Transaction wrapping** | If any UPSERT fails, the entire batch rolls back. This prevents partial ingestion where some cases are saved but the log doesn't reflect the full picture. |
| **Dual validation (SN + API)** | The scheduled job validates in SN, and the API validates again. This defense-in-depth catches any data issues from direct API calls, bulk imports, or future integrations. |
| **Separate `/bulk-import` (5000 limit)** | Backfill operations need larger batches. A separate endpoint with a higher limit prevents accidental abuse of the regular batch endpoint. |
| **`case_sys_id` as the natural key** | ServiceNow `sys_id` is globally unique and immutable. Using it as the UPSERT key guarantees deduplication regardless of how many times a case is sent. |

### 4.3 Process Layer 3 — Aggregation (Daily MTTR Computation)

```
    ┌─────────────────────────────────────────────────────────┐
    │          AGGREGATION CRON JOB (daily, midnight)         │
    │                                                         │
    │  Triggered by node-cron (AGGREGATION_CRON)              │
    │    │                                                    │
    │    ▼                                                    │
    │  ┌──────────────────────────────────────────────────┐   │
    │  │         runFullAggregation()                     │   │
    │  │                                                  │   │
    │  │  rolling_start = NOW() - 12 months               │   │
    │  │  rolling_end = NOW()                             │   │
    │  │                                                  │   │
    │  │  For each of 9 DIMENSION TYPES:                  │   │
    │  │    │                                             │   │
    │  │    ▼                                             │   │
    │  │  SELECT business_duration_ms, {groupBy columns}  │   │
    │  │  FROM case_events                                │   │
    │  │  WHERE closed_date BETWEEN rolling_start         │   │
    │  │        AND rolling_end                           │   │
    │  │    AND case_state IN ('Closed', 'Resolved')      │   │
    │  │    AND {dimension-specific filters}              │   │
    │  │    │                                             │   │
    │  │    ▼                                             │   │
    │  │  Group rows by dimension labels                  │   │
    │  │  (e.g., by cs_team for team_incidents)           │   │
    │  │    │                                             │   │
    │  │    ▼                                             │   │
    │  │  For each group:                                 │   │
    │  │    durations[] = all business_duration_ms values │   │
    │  │    mttr = computeTruncatedMTTR(durations)        │   │
    │  │    │                                             │   │
    │  │    ▼                                             │   │
    │  │  UPSERT INTO mttr_cache:                         │   │
    │  │    cache_key = dimension_type + labels (unique)  │   │
    │  │    dimension_type, dimension_labels (JSONB),     │   │
    │  │    period_start, period_end,                     │   │
    │  │    total_cases, excluded_cases, p95_cutoff_ms,   │   │
    │  │    truncated_avg_ms, simple_avg_ms,              │   │
    │  │    min_sample_met, calculated_at = NOW()         │   │
    │  │                                                  │   │
    │  └──────────────────────────────────────────────────┘   │
    │    │                                                    │
    │    ▼                                                    │
    │  ┌──────────────────────────────────────────────────┐   │
    │  │         purgeStaleCache()                        │   │
    │  │                                                  │   │
    │  │  DELETE FROM mttr_cache                          │   │
    │  │  WHERE calculated_at < MAX(calculated_at)        │   │
    │  │        for each dimension_type                   │   │
    │  │                                                  │   │
    │  │  (keeps only the latest computation)             │   │
    │  └──────────────────────────────────────────────────┘   │
    │    │                                                    │
    │    ▼                                                    │
    │  ┌──────────────────────────────────────────────────┐   │
    │  │         runRetention()                           │   │
    │  │         (see Process Layer 4)                    │   │
    │  └──────────────────────────────────────────────────┘   │
    │                                                         │
    │  END                                                    │
    └─────────────────────────────────────────────────────────┘
```

**Key Design Decisions:**

| Decision | Reasoning |
|---|---|
| **Daily computation, not real-time** | MTTR doesn't need second-level freshness. Cases close over hours/days. Daily recalculation is sufficient and avoids expensive queries on every API call. |
| **12-month rolling window** | A year of data provides enough volume for P95 to be meaningful while staying current. Configurable via `AGGREGATION_ROLLING_MONTHS`. |
| **Pre-computed cache (not materialised views)** | PostgreSQL materialised views require `REFRESH MATERIALIZED VIEW` which locks the view. Separate cache table allows concurrent reads during writes. |
| **UPSERT into cache by `cache_key`** | Each dimension + label combination has a unique key. On recomputation, the row is updated in-place rather than creating duplicates. |
| **Stale cache purge after recomputation** | If a dimension's labels change (e.g., a team is renamed or deleted), old cache rows for that label would persist. The purge removes any entry not from the latest calculation. |

### 4.4 Process Layer 4 — Data Retention & Archival

```
    ┌─────────────────────────────────────────────────────────┐
    │              RETENTION (part of daily cron)             │
    │                                                         │
    │  ┌─── Step 1: Purge Ingestion Logs ──────────────────┐  │
    │  │                                                   │  │
    │  │  DELETE FROM ingestion_log                        │  │
    │  │  WHERE ingested_at < NOW() - 14 days              │  │
    │  │                                                   │  │
    │  │  Reasoning: Ingestion logs are operational audit. │  │
    │  │  After 2 weeks, their diagnostic value drops.     │  │
    │  └───────────────────────────────────────────────────┘  │
    │                                                         │
    │  ┌─── Step 2: Quarterly Archival ────────────────────┐  │
    │  │                                                   │  │
    │  │  cutoff = NOW() - 24 months                       │  │
    │  │                                                   │  │
    │  │  For each completed quarter (Q1 → Q4):            │  │
    │  │    │                                              │  │
    │  │    ├── If quarter_end < cutoff (old enough):      │  │
    │  │    │   AND not yet summarised:                    │  │
    │  │    │     1. Query raw case_events for this quarter│  │
    │  │    │     2. Group by:                             │  │
    │  │    │        (case_type, priority, cs_team,        │  │
    │  │    │         product, is_patched)                 │  │
    │  │    │     3. Compute P95 truncated mean per group  │  │
    │  │    │     4. INSERT into case_events_summary       │  │
    │  │    │        (ON CONFLICT → UPDATE)                │  │
    │  │    │     5. DELETE raw case_events for this quarte│  │
    │  │    │                                              │  │
    │  │    ├── If quarter_end < cutoff (old enough):      │  │
    │  │    │   AND already summarised:                    │  │
    │  │    │     1. DELETE leftover raw case_events only  │  │
    │  │    │        (catches cases updated after summary) │  │
    │  │    │                                              │  │
    │  │    └── If quarter_end >= cutoff (too recent):     │  │
    │  │        1. Re-summarise (update summary with latest│  │
    │  │           data) but do NOT delete raw rows        │  │
    │  │        2. This keeps summaries up-to-date for     │  │
    │  │           historical reporting while raw data     │  │
    │  │           remains for active MTTR computation     │  │
    │  │                                                   │  │
    │  └───────────────────────────────────────────────────┘  │
    │                                                         │
    │  Result: { ingestion_log_purged, summarised, deleted }  │
    └─────────────────────────────────────────────────────────┘
```

**Key Design Decisions:**

| Decision | Reasoning |
|---|---|
| **24-month retention before archival** | Ensures at least 2 full years of raw data for the 12-month rolling aggregation plus a safety margin. |
| **Quarterly granularity for summaries** | Monthly would create too many summary rows. Yearly would lose seasonal patterns. Quarterly is the sweet spot for historical trend analysis. |
| **Summarise THEN delete (not vice versa)** | If deletion fails after summarisation, the raw data still exists. If summarisation fails, the transaction rolls back and nothing is deleted. |
| **Re-summarise recent quarters without deletion** | Cases can be updated after initial resolution (e.g., reopened and re-closed). Re-summarisation captures these changes before the quarter ages out. |
| **`ON CONFLICT DO UPDATE` for summaries** | Prevents duplicate summary rows and ensures re-summarisation updates existing data correctly. |
| **All retention in a single transaction** | Either everything succeeds (summaries written + raw data deleted) or everything rolls back. No orphaned state. |

### 4.5 Process Layer 5 — Query & Serving

```
    ┌─────────────────────────────────────────────────────────┐
    │           QUERY FLOW (API call → Response)              │
    │                                                         │
    │  GET /api/v1/mttr?type=team_incidents                   │
    │    │                                                    │
    │    ▼                                                    │
    │  Route validation (mttrRoutes):                         │
    │    - Validate 'type' against whitelist of 9 types       │
    │    │                                                    │
    │    ▼                                                    │
    │  cacheService.getCachedMTTR('team_incidents')           │
    │    │                                                    │
    │    ▼                                                    │
    │  SELECT * FROM mttr_cache                               │
    │  WHERE dimension_type = 'team_incidents'                │
    │    │                                                    │
    │    ├── Cache HIT (rows found):                          │
    │    │   Return rows with period, calculated_at, data     │
    │    │                                                    │
    │    └── Cache MISS (no rows):                            │
    │        → Call runAggregationForType('team_incidents')   │
    │        → Re-query mttr_cache                            │
    │        → Return newly computed data                     │
    │                                                         │
    │  Response format:                                       │
    │  {                                                      │
    │    "period": { "start", "end" },                        │
    │    "calculated_at": "2026-04-07T00:00:00Z",             │
    │    "data": [                                            │
    │      {                                                  │
    │        "labels": { "cs_team": "Cloud" },                │
    │        "total_cases": 150,                              │
    │        "excluded_cases": 8,                             │
    │        "p95_cutoff_hours": 72.5,                        │
    │        "mttr_hours": 18.3,                              │
    │        "simple_avg_hours": 25.1,                        │
    │        "min_sample_met": true                           │
    │      },                                                 │
    │      ...                                                │
    │    ]                                                    │
    │  }                                                      │
    └─────────────────────────────────────────────────────────┘
```

```
    ┌─────────────────────────────────────────────────────────┐
    │           HISTORICAL QUERY FLOW                         │
    │                                                         │
    │  GET /api/v1/summary/historical                         │
    │      ?case_type=Incident&group_by=team                  │
    │    │                                                    │
    │    ▼                                                    │
    │  Query case_events_summary table                        │
    │  (archived quarterly data)                              │
    │    │                                                    │
    │    ▼                                                    │
    │  Pivot data into a time-series format:                  │
    │  {                                                      │
    │    "case_type": "Incident",                             │
    │    "group_by": "team",                                  │
    │    "periods": ["2024-Q1", "2024-Q2", ...],              │
    │    "series": {                                          │
    │      "IAM": {                                           │
    │        "2024-Q1": {                                     │
    │          "mttr_hours": 15.2,                            │
    │          "total_cases": 42,                             │
    │          "min_sample_met": true                         │
    │        },                                               │
    │        ...                                              │
    │      },                                                 │
    │      "Cloud": { ... }                                   │
    │    }                                                    │
    │  }                                                      │
    └─────────────────────────────────────────────────────────┘
```

**Key Design Decisions:**

| Decision | Reasoning |
|---|---|
| **Cache-first with fallback computation** | If the cache is empty (fresh deployment, cache reset), the API computes on-demand rather than returning empty data. |
| **Milliseconds stored, hours returned** | Internal storage in milliseconds preserves precision. The API converts to hours (1 decimal) for human-readable display. |
| **Dimension type whitelist** | Prevents arbitrary SQL injection through the type parameter. Only 9 known types are accepted. |
| **Pivoted historical response** | Time-series format (periods × series) is what chart libraries expect. Pivoting server-side avoids complex client-side transformations. |

---

## 5. Data Ingestion Logic

### Case Fields & Transformations

| SN Field | DB Column | Transformation | Reasoning |
|---|---|---|---|
| `sys_id` | `case_sys_id` | Direct copy | Immutable unique ID |
| `u_wso2_product` (display) | `product` | Direct copy | Product name as text |
| Derived from product + account | `cs_team` | IAM/Choreo override or account team | Accurate team attribution |
| `business_duration.dateNumericValue()` | `business_duration_ms` | Milliseconds | SLA-aware duration, excludes non-business hours |
| `sys_created_on` | `created_date` | Direct copy | Case creation timestamp |
| `sys_updated_on` | `closed_date` | Direct copy | Latest update (used for watermark) |
| `u_case_type` (display) | `case_type` | "Incident" or "Query" | Case classification |
| `priority` (display) | `priority` | Normalized to P1–P4 (Incidents only) | Consistent short form |
| `u_fix_eta_shared` | `is_patched` | NOT NULL → true, NULL → false | Proxy for whether a fix was delivered |
| `state` (display) | `case_state` | "Closed" or "Solution Proposed" etc. | Resolution state |

### Why `business_duration` (not calendar duration)?

Calendar duration (`opened_at → closed_at`) includes weekends, holidays, and non-business hours. A case opened Friday 5pm and closed Monday 9am would show 64 hours — misleading when the actual work time was 0 hours.

ServiceNow's `business_duration` respects the SLA schedule:
- Only counts hours within the defined business schedule
- Pauses when the case is "Awaiting Customer" or "On Hold"
- Gives a true measure of support team effort

### Team Assignment Logic

```
Product Name → lowercase
  │
  ├── Contains "identity" or "asgardeo"?
  │     → Team = "IAM"
  │
  ├── Contains "choreo"?
  │     → Team = "Choreo"
  │
  └── Neither?
        → Team = account.u_integration_cs_team
          │
          ├── Has value → Use it
          └── Empty → SKIP this case (no team)
```

**Why product-based overrides?**

Some accounts work with multiple WSO2 products. If Account X has `u_integration_cs_team = "Cloud"` but raises an Identity Server case, that case should count toward the IAM team's metrics, not Cloud's. Product-based overrides ensure accurate attribution.

---

## 6. MTTR Computation Logic

### Full Algorithm Walkthrough

Given a set of business durations for a specific group (e.g., all Incidents for the Cloud team in the past 12 months):

```
Input: [2h, 5h, 8h, 12h, 15h, 20h, 25h, 30h, 48h, 72h, 100h, 120h,
        150h, 200h, 500h, 1000h, 2h, 3h, 7h, 9h, 11h, 14h, 18h, 22h,
        28h, 35h, 45h, 60h, 80h, 90h]

Step 1: Sort ascending
[2, 2, 3, 5, 7, 8, 9, 11, 12, 14, 15, 18, 20, 22, 25, 28, 30, 35, 45,
 48, 60, 72, 80, 90, 100, 120, 150, 200, 500, 1000]

n = 30

Step 2: Check min sample
30 >= 20 (MIN_SAMPLE) → proceed with P95

Step 3: Calculate P95 index
p95Index = ceil(30 × 0.95) - 1 = ceil(28.5) - 1 = 29 - 1 = 28
p95Cutoff = sorted[28] = 500h

Step 4: Include first 29 values (index 0..28)
Excluded: [1000h] (1 case)
Included: first 29 values

Step 5: Calculate truncated average
truncatedAvg = sum(first 29) / 29 = 1679 / 29 ≈ 57.9h

Step 6: Calculate simple average (for comparison)
simpleAvg = sum(all 30) / 30 = 2679 / 30 ≈ 89.3h

Result:
  total_cases: 30
  excluded_cases: 1
  p95_cutoff_ms: 500h (in ms)
  truncated_avg_ms: 57.9h (THE MTTR)
  simple_avg_ms: 89.3h
  min_sample_met: true
```

The P95 truncated average (57.9h) is **34% lower** than the simple average (89.3h) because the 1000h outlier was excluded. This gives a more accurate picture of typical resolution time.

### Small Sample Behaviour

When a group has fewer than `MIN_SAMPLE_SIZE` (default 20) cases:

```
Input: [5h, 10h, 15h, 20h, 500h]  (only 5 cases)

n = 5 < 20 → use simple average

simpleAvg = 550 / 5 = 110h

Result:
  total_cases: 5
  excluded_cases: 0
  p95_cutoff_ms: null
  truncated_avg_ms: 110h (simple average used)
  simple_avg_ms: 110h
  min_sample_met: false  ← UI shows warning
```

---

## 7. Aggregation Dimensions — What & Why

The system computes MTTR across 9 different "dimensions" (ways of slicing the data). Each dimension answers a different question:

| # | Dimension Type | GROUP BY | Filters | Question It Answers |
|---|---|---|---|---|
| 1 | `overall_by_type` | `case_type` | None | What is our overall Incident MTTR vs Query MTTR? |
| 2 | `team_incidents` | `cs_team` | `case_type = 'Incident'` | Which team resolves Incidents fastest/slowest? |
| 3 | `team_queries` | `cs_team` | `case_type = 'Query'` | Which team resolves Queries fastest/slowest? |
| 4 | `team_incidents_priority` | `cs_team`, `priority` | `case_type = 'Incident'` | How does each team perform by priority? |
| 5 | `priority_incidents` | `priority` | `case_type = 'Incident'` | How does priority affect resolution time? |
| 6 | `patched_incidents` | `is_patched` | `case_type = 'Incident'` | Do patched Incidents resolve faster? |
| 7 | `patched_queries` | `is_patched` | `case_type = 'Query'` | Do patched Queries resolve faster? |
| 8 | `monthly_trend_team` | `cs_team`, `month` | None | Is each team's MTTR improving or degrading? |
| 9 | `monthly_trend_type` | `case_type`, `month` | None | Is overall MTTR trending up or down? |

### Why These Specific Dimensions?

- **Overall (1)**: Executive summary. "How are we doing?"
- **Team (2, 3)**: Performance management. "Which teams need help?"
- **Team × Priority (4)**: Deep dive. "Is Team X slow only on P1s, or across the board?"
- **Priority (5)**: SLA analysis. "Are P1s really faster than P4s?"
- **Patched (6, 7)**: Fix delivery impact. "Does having a patch reduce resolution time?"
- **Monthly Trends (8, 9)**: Direction of travel. "Are things getting better or worse?"

---

## 8. Caching Strategy

### Why Cache?

```
Without cache:
  Dashboard load → 9 SQL queries × each GROUP BY → 9 P95 computations
  = ~2-5 seconds per dashboard view with 50K+ rows

With cache:
  Dashboard load → 9 simple SELECT FROM mttr_cache
  = ~50ms per dashboard view regardless of data volume
```

### Cache Lifecycle

```
1. Daily midnight cron:
   → Compute all 9 dimensions
   → UPSERT into mttr_cache (updates existing or inserts new)
   → Purge stale cache (delete non-latest entries per dimension)

2. On API query:
   → Read from mttr_cache
   → If empty (first deployment, post-reset): compute on-demand

3. Manual cache reset (/admin/cache/reset):
   → Recompute all or specific dimension
   → Used after large data imports or when immediate refresh is needed
```

### Cache Key Structure

```
cache_key = "{dimension_type}:{label1}={value1}:{label2}={value2}:..."

Examples:
  "overall_by_type:case_type=Incident"
  "team_incidents:cs_team=Cloud"
  "team_incidents_priority:cs_team=IAM:priority=P1"
  "monthly_trend_team:cs_team=Choreo:month=2026-03"
```

### Why Not Use Redis?

PostgreSQL is already the system of record. Adding Redis would:
- Introduce another infrastructure dependency
- Create cache coherency problems (two stores to keep in sync)
- Add operational complexity for a low-read-rate system (< 1000 requests/day)

The `mttr_cache` table with indexed `dimension_type` is fast enough for this workload.

---

## 9. Data Retention & Archival Logic

### The Problem

Without retention, the `case_events` table grows indefinitely:
- ~500 cases/month × 12 teams = ~6,000 rows/month
- After 5 years = ~360,000 rows
- Aggregation queries scan the full table on each run

### The Solution: Quarterly Summarisation

```
case_events (raw data)
    │
    │  After 24 months:
    │  Summarise by quarter into case_events_summary
    │  Then delete the raw rows
    │
    ▼
case_events_summary (compressed)
    - One row per unique (quarter, case_type, priority, team, product, is_patched)
    - Contains pre-computed P95 truncated mean
    - ~100x fewer rows than the original
    - Powers the /summary/historical endpoint
```

### Retention Timeline Example

```
Today: 2027-01-15
Retention cutoff: 2027-01-15 - 24 months = 2025-01-15

Quarter 2024-Q1 (Jan-Mar 2024) → ends 2024-03-31 < cutoff → SUMMARISE + DELETE
Quarter 2024-Q2 (Apr-Jun 2024) → ends 2024-06-30 < cutoff → SUMMARISE + DELETE
Quarter 2024-Q3 (Jul-Sep 2024) → ends 2024-09-30 < cutoff → SUMMARISE + DELETE
Quarter 2024-Q4 (Oct-Dec 2024) → ends 2024-12-31 < cutoff → SUMMARISE + DELETE
Quarter 2025-Q1 (Jan-Mar 2025) → ends 2025-03-31 > cutoff → SUMMARISE only (keep raw)
Quarter 2025-Q2 (Apr-Jun 2025) → ends 2025-06-30 > cutoff → SUMMARISE only (keep raw)
... and so on
```

### Why 24 Months?

- The rolling aggregation window is 12 months
- 24-month retention provides a 12-month safety buffer
- Even if someone changes `AGGREGATION_ROLLING_MONTHS` to 18, there's still 6 months of headroom
- Quarters within the 24-month window are summarised (for historical reporting) but raw data is kept (for active aggregation)

---

## 10. Security Model

### Defence in Depth

```
Layer 1: Network Level
├── Choreo API Gateway terminates TLS
├── Only HTTPS traffic accepted
└── No direct access to backend service

Layer 2: Authentication
├── Choreo validates OAuth 2.0 JWT before forwarding
├── Express app (optional): Re-verifies JWT via JWKS
├── ServiceNow authenticates using Client Credentials grant
└── /health endpoint: No auth (for health probes)

Layer 3: Authorization
├── Regular endpoints: Any authenticated user
├── /admin/* endpoints: Requires 'mttr-admin' role in JWT
└── Role checked via middleware (requireAdmin)

Layer 4: Input Validation
├── express-validator: Schema validation on ingestion payloads
├── Service-level: validateCase() checks each field
├── SQL: Parameterized queries (prevents SQL injection)
└── Payload size: 10MB limit via express.json()

Layer 5: Rate Limiting
├── express-rate-limit: 100 requests per 15-minute window (configurable)
├── Returns 429 with retry-after header
└── Applies to /api/* routes only

Layer 6: HTTP Security Headers
├── helmet: Sets CSP, X-Frame-Options, X-Content-Type-Options, etc.
└── CORS: Restricted origins in production

Layer 7: Container Security
├── Non-root user (UID 10001)
├── npm ci --omit=dev (no dev dependencies in production)
└── node:20-alpine (minimal base image)
```

### Why `AUTH_ENABLED=false` on Choreo?

Choreo's API Gateway already validates the OAuth 2.0 token before the request reaches the Express app. Having the app re-validate the same token is redundant processing. Setting `AUTH_ENABLED=false`:
- Eliminates unnecessary JWKS calls
- Reduces latency (no JWT verification per request)
- Still safe because the gateway is the security boundary
- In development, this also injects a mock admin user for convenience

---

## 11. ServiceNow Integration Logic

### Script Include API (MttrDashboardAPI)

The Script Include acts as a server-side adapter between the Choreo REST API and ServiceNow's widgets:

```
MttrDashboardAPI
├── _fetchDimension(type)         → GET /mttr?type={type}
├── _fetchSummary(caseType, grp)  → GET /summary/historical
├── _hoursToDisplay(hours)        → "X Days Y Hours Z Minutes"
│
├── getOverallByType()            → { overall, incident, query }
├── getPatchedMTTR()              → { patched, non_patched }
├── getMonthlyTrendPriority()     → [ {month, P1, P2, P3, P4} ]
├── getTeamCaseTypeMTTR()         → [ {team, Incident, Query} ]
├── getMonthlyTrendTeam()         → { months, teams: {name: []} }
│
├── getHistoricalOverall()        → { periods, Incident:[], Query:[] }
├── getHistoricalIncidentsByTeam()
├── getHistoricalIncidentsByPriority()
└── getHistoricalQueriesByTeam()
```

### Why a Script Include (Not Direct REST Calls from Widgets)?

1. **Centralised authentication**: One place manages the REST Message and OAuth profile
2. **Data transformation**: The raw API response needs reshaping for Google Charts
3. **Error handling**: Consistent error handling across all widgets
4. **Reusability**: Multiple widgets can call the same method
5. **Testability**: Can be tested in background scripts independently of widgets

### Widget Architecture

```
UI Page (Jelly XML)
  │
  ├── Server-side processing block (<g2:evaluate>):
  │   └── Calls MttrDashboardAPI methods
  │       └── Returns data as Jelly variables
  │
  ├── Client-side JavaScript block:
  │   └── Uses Google Visualization API to render charts
  │       ├── Reads data from Jelly-injected variables
  │       ├── Converts to DataTable format
  │       └── Renders appropriate chart type
  │
  └── HTML/CSS:
      └── Container divs, responsive sizing
```

### Why Google Charts in SN?

- ServiceNow's Content Security Policy blocks most external scripts by default
- Google Visualization API is whitelisted on most ServiceNow instances
- Google Charts supports the specific chart types needed (pie, line, column)
- Jelly templates can inject data directly into JavaScript without AJAX calls

### 12 Widget Summary

| Widget | Chart Type | Data Source Method | Key Metric |
|---|---|---|---|
| Overall MTTR | KPI number | `getOverallByType()` | Combined MTTR |
| Incident MTTR | KPI number | `getOverallByType()` | Incident-only MTTR |
| Query MTTR | KPI number | `getOverallByType()` | Query-only MTTR |
| Patched Issues | Donut pie | `getPatchedMTTR()` | Patched Incident/Query split |
| Non-Patched Issues | Donut pie | `getPatchedMTTR()` | Non-patched Incident/Query split |
| Priority Trend | Line chart | `getMonthlyTrendPriority()` | P1–P4 monthly MTTR |
| Team × Case Type | Column chart | `getTeamCaseTypeMTTR()` | Team Incident vs Query bars |
| Monthly Team Trend | Line chart | `getMonthlyTrendTeam()` | Per-team monthly trend |
| Historical Overall | Line chart | `getHistoricalOverall()` | Quarterly Incident vs Query |
| Historical Inc/Priority | Line chart | `getHistoricalIncidentsByPriority()` | Quarterly P1–P4 trend |
| Historical Inc/Team | Line chart | `getHistoricalIncidentsByTeam()` | Quarterly team trend |
| Historical Qry/Team | Line chart | `getHistoricalQueriesByTeam()` | Quarterly query team trend |

---

## 12. Visualization Logic

### Unit Conversions

| Source Unit | Display Context | Conversion |
|---|---|---|
| Milliseconds (DB) | API response | `ms / 3,600,000` → hours (1 decimal) |
| Hours (API) | KPI cards | Direct display with "hours" suffix |
| Hours (API) | Chart Y-axis | `hours / 24` → days (where appropriate) |
| Hours (API) | SN widgets | `_hoursToDisplay()` → "X Days Y Hours Z Minutes" |

---

## 13. Design Decision Log

This section documents every significant design decision, the alternatives considered, and why the chosen approach was selected.

### D1: Scheduled Push vs Webhook vs Pull

| Approach | Pros | Cons | Verdict |
|---|---|---|---|
| **Scheduled push (chosen)** | Simple, reliable, incremental, no SN plugin needed | 10-min delay | ✅ Best for this use case |
| Webhook (Business Rule) | Real-time | Fires on every update (noisy), hard to batch, SN performance impact | ❌ |
| Pull from external | Decouples SN from API | Requires SN API auth, complex pagination, no standard filter for "closed since X" | ❌ |

### D2: P95 Truncated Average vs Other Statistical Methods

| Method | Pros | Cons | Verdict |
|---|---|---|---|
| Simple average | Easy to understand | Skewed by outliers | ❌ |
| Median (P50) | Outlier-resistant | Ignores half the data | ❌ |
| **P95 truncated mean (chosen)** | Outlier-resistant, uses 95% of data, industry standard | Slightly more complex | ✅ |
| P90 truncated mean | More aggressive filtering | Excludes too many valid cases | ❌ |

### D3: Pre-Computed Cache vs On-Demand Computation

| Approach | Pros | Cons | Verdict |
|---|---|---|---|
| **Pre-computed daily (chosen)** | Fast reads (<100ms), scales to many dashboard viewers | Data up to 24h stale | ✅ |
| Real-time computation | Always fresh | Slow (2-5s per view), doesn't scale | ❌ |
| Materialized views | Built-in PostgreSQL feature | Locks during refresh, limited transformation | ❌ |

### D4: Single Service vs Microservices

| Approach | Pros | Cons | Verdict |
|---|---|---|---|
| **Single Express app (chosen)** | Simple deployment, shared DB pool, easy debugging | All-or-nothing scaling | ✅ |
| Separate ingestion/aggregation/query services | Independent scaling | Operational complexity for low-volume system | ❌ |

### D5: PostgreSQL vs Time-Series DB

| Database | Pros | Cons | Verdict |
|---|---|---|---|
| **PostgreSQL (chosen)** | Choreo native integration, JSONB support, mature, team expertise | Not optimized for time-series | ✅ |
| TimescaleDB | Time-series optimized | Extra dependency, limited Choreo support | ❌ |
| InfluxDB | Purpose-built | Separate infrastructure, no SQL joins | ❌ |

### D6: UPSERT vs Insert-Only + Deduplication

| Approach | Pros | Cons | Verdict |
|---|---|---|---|
| **UPSERT by case_sys_id (chosen)** | Always latest data, simple, no duplicates | Loses version history | ✅ |
| Insert-only + process latest | Version history preserved | Complex queries, growing table | ❌ |

### D7: Quarterly Archival vs Rolling Window Only

| Approach | Pros | Cons | Verdict |
|---|---|---|---|
| **Quarterly archival (chosen)** | Unlimited history for trend analysis, bounded raw table | More complex retention logic | ✅ |
| Rolling window only | Simpler | Loses all data older than 12 months | ❌ |
| No retention | Simplest | Unbounded table growth | ❌ |

### D8: SN UI Pages on Homepage vs Performance Analytics

| Approach | Pros | Cons | Verdict |
|---|---|---|---|
| **UI Pages on Homepage (chosen)** | Works in standard UI, simpler Jelly templates, admin-configurable layout | Not mobile-optimized | ✅ |
| Performance Analytics | Native SN feature | Cannot compute P95, expensive license | ❌ |

### D9: Google Charts vs D3.js vs Highcharts (in SN)

| Library | Pros | Cons | Verdict |
|---|---|---|---|
| **Google Charts (chosen)** | CSP-whitelisted in SN, server-rendered data injection, no AJAX needed | Basic styling | ✅ |
| D3.js | Most flexible | CSP blocked in many SN instances, complex code | ❌ |
| Highcharts | Rich charts | License required, CSP issues | ❌ |

---

## Appendix A — Data Flow End-to-End Example

Here's how a single case flows through the entire system:

```
1. Case CS-00012345 is closed in ServiceNow
   - Product: "WSO2 API Manager"
   - Account: "Acme Corp" (u_integration_cs_team = "Cloud")
   - Business Duration: 14h 30m (52,200,000 ms)
   - u_case_type: Incident
   - Priority: "2 - High"
   - u_fix_eta_shared: "2026-03-15"

2. Scheduled Job runs (next 10-min cycle)
   - Query finds CS-00012345 (sys_updated_on > watermark)
   - Product "wso2 api manager" doesn't contain "identity", "asgardeo", or "choreo"
   - cs_team = "Cloud" (from account)
   - is_patched = true (u_fix_eta_shared is populated)
   - priority = "P2" (extracted from "2 - High")
   - Added to batch array

3. POST /api/v1/cases/batch
   {
     "batch_id": "batch_1712505600000",
     "cases": [{
       "case_sys_id": "abc123...",
       "product": "WSO2 API Manager",
       "cs_team": "Cloud",
       "business_duration_ms": 52200000,
       "created_date": "2026-03-01T10:00:00",
       "closed_date": "2026-03-15T12:00:00",
       "case_type": "Incident",
       "priority": "P2",
       "is_patched": true,
       "case_state": "Closed"
     }]
   }

4. Ingestion Service
   - Validates all fields ✓
   - INSERT INTO case_events ... ON CONFLICT (case_sys_id) DO UPDATE
   - Logs to ingestion_log: 1 received, 1 inserted, 0 updated, 0 rejected
   - Returns 200 with summary

5. Watermark Advanced
   - u_mttr_last_sync = "2026-03-15 12:05:00" (sys_updated_on of this case)

6. Daily Aggregation (midnight)
   - Case appears in multiple dimensions:
     • overall_by_type: grouped under "Incident"
     • team_incidents: grouped under "Cloud"
     • team_incidents_priority: grouped under "Cloud" + "P2"
     • priority_incidents: grouped under "P2"
     • patched_incidents: grouped under "true"
     • monthly_trend_team: grouped under "Cloud" + "2026-03"
     • monthly_trend_type: grouped under "Incident" + "2026-03"
   - Each group's P95 truncated mean is recalculated
   - Results cached in mttr_cache

7. Widget Query
   - GET /mttr?type=team_incidents
   - Returns Cloud team MTTR including this case's duration
   - Widget renders bar chart showing Cloud team's updated MTTR

8. 24 Months Later (2028-Q1)
   - Retention job finds 2026-Q1 is eligible for archival
   - Summarises all Q1 2026 cases by (case_type, priority, team, product, is_patched)
   - Writes summary to case_events_summary
   - Deletes raw case_events rows for Q1 2026
   - CS-00012345's duration is now part of a quarterly aggregate
   - Historical widgets still show Q1 2026 data via /summary/historical
```

---

## Appendix B — Glossary

| Term | Definition |
|---|---|
| **MTTR** | Mean Time to Resolve — average time to close a case |
| **P95** | 95th percentile — the value below which 95% of observations fall |
| **Truncated Average** | Average computed after excluding top N% of values |
| **Dimension** | A way of slicing MTTR data (by team, priority, case type, etc.) |
| **Rolling Window** | Latest N months of data, sliding forward daily |
| **Watermark** | Timestamp of the last successfully synced record |
| **UPSERT** | INSERT a new row, or UPDATE if a row with the same key exists |
| **Business Duration** | SLA-aware time, excludes non-business hours and hold time |
| **is_patched** | Whether a software fix was delivered for this case |
| **Retention Cutoff** | Date beyond which raw data is archived and deleted |
| **Cache Key** | Unique identifier for a cached MTTR computation |
| **Jelly** | ServiceNow's XML-based template language for UI Pages |
