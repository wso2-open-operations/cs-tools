// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// ============================================================================
// Aggregation Service — The MTTR Compute Engine
// ----------------------------------------------------------------------------
// This is the analytical core of the application. It:
//   1. Reads raw resolved-case rows from `case_events`.
//   2. Groups them along nine pre-defined dimensions (team, priority, etc).
//   3. Computes a 95th-percentile *truncated mean* duration per group.
//   4. UPSERTs the result into `mttr_cache` so the read APIs serve from a
//      pre-computed table in <100ms.
//
// Why P95 truncated mean instead of simple average / median?
//   A handful of multi-week cases (blocked on customer, awaiting vendor
//   patch, etc.) dominate any naïve average and give a misleading picture
//   of typical resolution time. Discarding the top 5% and averaging the
//   rest is the same approach SRE teams use for latency SLOs and produces
//   a metric that reflects *typical* customer experience.
//
//   The full algorithm is documented in docs/logic-and-architecture.md §2.
//
// When does this run?
//   • Daily at midnight from jobs/aggregationJob.js (full rebuild).
//   • On demand via POST /api/v1/admin/cache/reset.
//   • Lazily on cache-miss from cacheService.getCachedMTTR().
// ============================================================================

const db = require('../config/database');
const applicationConfig = require('../config');
const logger = require('../utils/logger');
const { computeTruncatedMTTR } = require('../utils/mttrMath');

// Cached config values — these never change at runtime.
const ROLLING_WINDOW_MONTHS = applicationConfig.aggregation.rollingMonths; // width of the analysis window

// ─── Dimension Definitions ─────────────────────────────────────────────────
//
// Each entry below describes ONE "slice" of the data set we want to compute
// MTTR for. The aggregation engine iterates over every entry, groups the
// raw rows by the listed `groupBy` columns (optionally filtered by `where`),
// computes the P95-truncated mean per group, and stores one cache row per
// group in `mttr_cache`.
//
//   groupBy  – columns the SELECT both projects and groups by
//   where    – optional SQL fragment AND-ed into the WHERE clause
//   monthly  – when true, the aggregation is also bucketed by month
//              (yielding a time-series suitable for line charts)
// ───────────────────────────────────────────────────────────────────────────

const DIMENSIONS = {
    // Overall MTTR for each case type (Incident, Query) — KPI cards
    overall_by_type: {
        groupBy: ['case_type'],
        where: null,
        monthly: false,
    },
    // Team-wise MTTR for Incidents — "which team is fastest?"
    team_incidents: {
        groupBy: ['cs_team'],
        where: "case_type = 'Incident'",
        monthly: false,
    },
    // Team × Priority breakdown — drives the stacked bar chart
    team_incidents_priority: {
        groupBy: ['cs_team', 'priority'],
        where: "case_type = 'Incident'",
        monthly: false,
    },
    // Team-wise MTTR for Queries
    team_queries: {
        groupBy: ['cs_team'],
        where: "case_type = 'Query'",
        monthly: false,
    },
    // MTTR for Incidents grouped by priority — drives the priority pie
    priority_incidents: {
        groupBy: ['priority'],
        where: "case_type = 'Incident'",
        monthly: false,
    },
    // Patched vs non-patched Incidents — "does shipping a fix speed us up?"
    patched_incidents: {
        groupBy: ['is_patched'],
        where: "case_type = 'Incident'",
        monthly: false,
    },
    // Patched vs non-patched Queries
    patched_queries: {
        groupBy: ['is_patched'],
        where: "case_type = 'Query'",
        monthly: false,
    },
    // Monthly MTTR trend per team — line chart
    monthly_trend_team: {
        groupBy: ['cs_team'],
        where: null,
        monthly: true,
    },
    // Monthly MTTR trend per case type — line chart
    monthly_trend_type: {
        groupBy: ['case_type'],
        where: null,
        monthly: true,
    },
    // Monthly MTTR trend per priority (Incidents only) — line chart
    monthly_trend_priority: {
        groupBy: ['priority'],
        where: "case_type = 'Incident'",
        monthly: true,
    },
};

// Startup allowlist assertion — every DIMENSIONS entry's groupBy column
// is interpolated directly into SQL string in fetchDurations below. All
// current values are hardcoded plain column names and safe, but the
// pattern is structurally identical to an injection vulnerability: if
// someone later loads DIMENSIONS from a config table or env var, an
// attacker-controlled column name would be silently exploitable. This
// loop turns that future mistake into an immediate boot-time crash.
//
// Note: dimensionDef.where fragments are also interpolated but aren't
// amenable to structural validation (they're SQL predicates, not
// identifiers). Their safety guarantee is that they're all authored
// here in this file — same-file review is the enforcement.
const SAFE_SQL_IDENTIFIER = /^[a-z_][a-z0-9_]*$/i;
for (const [dimensionName, dimensionDef] of Object.entries(DIMENSIONS)) {
    for (const column of dimensionDef.groupBy) {
        if (!SAFE_SQL_IDENTIFIER.test(column)) {
            throw new Error(
                `Unsafe groupBy column in dimension '${dimensionName}': ${column}`
            );
        }
    }
}

// The P95 truncated-mean itself lives in src/utils/mttrMath.js so the
// aggregation and retention services share one implementation. See
// docs/logic-and-architecture.md §2 for the algorithm description.

// ─── Rolling Period Boundaries ─────────────────────────────────────────────
//
// Returns the date window the daily aggregation operates over. The window
// is "today minus ROLLING_WINDOW_MONTHS" to "today" — so every day the
// window slides forward by one day.

function getRollingPeriod() {
    const windowEnd = new Date();
    const windowStart = new Date();
    windowStart.setMonth(windowStart.getMonth() - ROLLING_WINDOW_MONTHS);
    return {
        start: windowStart.toISOString().split('T')[0],
        end: windowEnd.toISOString().split('T')[0],
    };
}

// ─── Fetch raw durations for a single dimension ────────────────────────────
//
// Builds a SQL query that:
//   • restricts to Closed/Resolved cases in the rolling window
//   • applies any dimension-specific WHERE clause
//   • optionally filters by a specific month (for monthly trend dimensions)
//   • orders by the group keys (so groupByLabels() can rely on order)

async function fetchDurations(dimensionDef, periodWindow, specificMonth) {
    // case_state is normalised to lowercase at ingest (see
    // ingestionService.validateCase) so this predicate can use the
    // composite index idx_case_events_state_closed(case_state,
    // closed_date). Do NOT wrap with LOWER() — Postgres cannot use a
    // B-tree index on the raw column to satisfy a function predicate.
    let sqlText = `
        SELECT ${dimensionDef.groupBy.join(', ')}, business_duration_ms
        FROM case_events
        WHERE case_state IN ('closed', 'resolved')
          AND closed_date >= $1
          AND closed_date <= ($2::date + interval '1 day')
    `;
    const sqlParams = [periodWindow.start, periodWindow.end];

    // Dimension-specific filter (e.g. case_type = 'Incident').
    if (dimensionDef.where) {
        sqlText += ` AND ${dimensionDef.where}`;
    }

    // For monthly-trend dimensions, narrow further to a single calendar month.
    if (specificMonth) {
        sqlText += ` AND to_char(closed_date, 'YYYY-MM') = $${sqlParams.length + 1}`;
        sqlParams.push(specificMonth);
    }

    sqlText += ` ORDER BY ${dimensionDef.groupBy.join(', ')}, business_duration_ms`;

    const queryResult = await db.query(sqlText, sqlParams);
    return queryResult.rows;
}

// ─── Get the distinct months present in the rolling window ─────────────────
//
// Used by monthly-trend dimensions to know which months to iterate over.
// (Picking distinct months from the DATA, not generating them, means empty
// months are skipped — preventing zero-value points in the line chart.)

async function getDistinctMonths(periodWindow) {
    // case_state matched by raw equality (no LOWER wrapper) so the
    // composite index can serve this — see fetchDurations comment.
    const queryResult = await db.query(
        `SELECT DISTINCT to_char(closed_date, 'YYYY-MM') AS month
         FROM case_events
         WHERE case_state IN ('closed', 'resolved') AND closed_date >= $1 AND closed_date <= ($2::date + interval '1 day')
         ORDER BY month`,
        [periodWindow.start, periodWindow.end]
    );
    return queryResult.rows.map((row) => row.month);
}

// ─── Group rows by their dimension labels ──────────────────────────────────
//
// Takes the flat row set from fetchDurations() and groups durations into
// buckets keyed by a pipe-joined string of the label values. Each bucket
// carries its labels (echoed back into the cache for the UI) and a flat
// array of durations ready for computeTruncatedMTTR().

function groupByLabels(rows, groupByFields) {
    const groupedBuckets = {};
    for (const row of rows) {
        // Compose a deterministic key by concatenating label values.
        const bucketKey = groupByFields.map((fieldName) => String(row[fieldName])).join('|');

        if (!groupedBuckets[bucketKey]) {
            const labels = {};
            for (const fieldName of groupByFields) {
                labels[fieldName] = row[fieldName];
            }
            groupedBuckets[bucketKey] = { labels, durations: [] };
        }
        groupedBuckets[bucketKey].durations.push(Number(row.business_duration_ms));
    }
    return groupedBuckets;
}

// ─── Write one aggregation result to the cache ─────────────────────────────
//
// UPSERTs by `cache_key` so re-running aggregation simply overwrites the
// existing row rather than creating duplicates. Stale rows from removed
// labels are cleaned up separately by retentionService.purgeStaleCache().

async function writeCacheEntry(dbClient, cacheKey, dimensionType, dimensionLabels, periodWindow, mttrStats) {
    await dbClient.query(
        `INSERT INTO mttr_cache
            (cache_key, dimension_type, dimension_labels, period_start, period_end,
             total_cases, excluded_cases, p95_cutoff_ms, truncated_avg_ms, simple_avg_ms,
             min_sample_met, calculated_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
         ON CONFLICT (cache_key) DO UPDATE SET
            dimension_labels = EXCLUDED.dimension_labels,
            period_start = EXCLUDED.period_start,
            period_end = EXCLUDED.period_end,
            total_cases = EXCLUDED.total_cases,
            excluded_cases = EXCLUDED.excluded_cases,
            p95_cutoff_ms = EXCLUDED.p95_cutoff_ms,
            truncated_avg_ms = EXCLUDED.truncated_avg_ms,
            simple_avg_ms = EXCLUDED.simple_avg_ms,
            min_sample_met = EXCLUDED.min_sample_met,
            calculated_at = NOW()`,
        [
            cacheKey,
            dimensionType,
            JSON.stringify(dimensionLabels),
            periodWindow.start,
            periodWindow.end,
            mttrStats.total_cases,
            mttrStats.excluded_cases,
            mttrStats.p95_cutoff_ms,
            mttrStats.truncated_avg_ms,
            mttrStats.simple_avg_ms,
            mttrStats.min_sample_met,
        ]
    );
}

// ─── Run aggregation for a single dimension ────────────────────────────────
//
// Branches on whether the dimension is monthly (loop over months × groups)
// or rolling (single loop over groups). Returns the number of cache rows
// written, useful for the cron job's summary log.

async function aggregateDimension(dimensionType, dbClient) {
    const dimensionDef = DIMENSIONS[dimensionType];
    if (!dimensionDef) throw new Error(`Unknown dimension type: ${dimensionType}`);

    const periodWindow = getRollingPeriod();
    let cacheRowsWritten = 0;

    if (dimensionDef.monthly) {
        // Monthly trend: one cache row per (group × month) combination.
        const monthsWithData = await getDistinctMonths(periodWindow);
        for (const monthLabel of monthsWithData) {
            const rawRows = await fetchDurations(dimensionDef, periodWindow, monthLabel);
            const groupedBuckets = groupByLabels(rawRows, dimensionDef.groupBy);

            for (const [bucketKey, bucket] of Object.entries(groupedBuckets)) {
                const mttrStats = computeTruncatedMTTR(bucket.durations);
                if (!mttrStats) continue;

                // Add the month into the labels echoed back by the API.
                const labelsWithMonth = { ...bucket.labels, month: monthLabel };
                const cacheKey = `${dimensionType}|${bucketKey}|${monthLabel}`;
                await writeCacheEntry(dbClient, cacheKey, dimensionType, labelsWithMonth, periodWindow, mttrStats);
                cacheRowsWritten++;
            }
        }
    } else {
        // Rolling-window aggregate: one cache row per group.
        const rawRows = await fetchDurations(dimensionDef, periodWindow, null);
        const groupedBuckets = groupByLabels(rawRows, dimensionDef.groupBy);

        for (const [bucketKey, bucket] of Object.entries(groupedBuckets)) {
            const mttrStats = computeTruncatedMTTR(bucket.durations);
            if (!mttrStats) continue;

            const cacheKey = `${dimensionType}|${bucketKey}|rolling`;
            await writeCacheEntry(dbClient, cacheKey, dimensionType, bucket.labels, periodWindow, mttrStats);
            cacheRowsWritten++;
        }
    }

    return cacheRowsWritten;
}

// ─── Run aggregation for ALL dimensions in one transaction ────────────────
//
// Called by the nightly cron and by the manual cache reset endpoint.
// Wrapping everything in a single transaction means a failure mid-way
// leaves the cache untouched (no half-rebuilt state).

async function runFullAggregation(correlationId) {
    const aggregationStartedAt = Date.now();
    logger.info('Starting full aggregation', { correlationId });

    const dbClient = await db.getClient();
    const rowsWrittenByDimension = {};

    try {
        await dbClient.query('BEGIN');

        for (const dimensionType of Object.keys(DIMENSIONS)) {
            const cacheRowsWritten = await aggregateDimension(dimensionType, dbClient);
            rowsWrittenByDimension[dimensionType] = cacheRowsWritten;
            logger.info(`Aggregated ${dimensionType}: ${cacheRowsWritten} entries`, { correlationId });
        }

        await dbClient.query('COMMIT');
    } catch (aggregationError) {
        await dbClient.query('ROLLBACK');
        logger.error('Full aggregation failed', { correlationId, error: aggregationError.message });
        throw aggregationError;
    } finally {
        dbClient.release();
    }

    const totalDurationMs = Date.now() - aggregationStartedAt;
    logger.info('Full aggregation completed', {
        correlationId,
        durationMs: totalDurationMs,
        results: rowsWrittenByDimension,
    });
    return { duration_ms: totalDurationMs, results: rowsWrittenByDimension };
}

// ─── Run aggregation for ONE specific dimension type ───────────────────────
//
// Used by:
//   • cacheService on a cache miss (lazy compute)
//   • POST /admin/cache/reset?type=…  (targeted refresh)

async function runAggregationForType(dimensionType, correlationId) {
    const aggregationStartedAt = Date.now();
    logger.info(`Starting aggregation for ${dimensionType}`, { correlationId });

    const dbClient = await db.getClient();
    let cacheRowsWritten;

    try {
        await dbClient.query('BEGIN');
        cacheRowsWritten = await aggregateDimension(dimensionType, dbClient);
        await dbClient.query('COMMIT');
    } catch (aggregationError) {
        await dbClient.query('ROLLBACK');
        logger.error(`Aggregation for ${dimensionType} failed`, {
            correlationId,
            error: aggregationError.message,
        });
        throw aggregationError;
    } finally {
        dbClient.release();
    }

    const totalDurationMs = Date.now() - aggregationStartedAt;
    logger.info(`Aggregation for ${dimensionType} completed`, {
        correlationId,
        durationMs: totalDurationMs,
        entries: cacheRowsWritten,
    });
    return { dimension_type: dimensionType, entries: cacheRowsWritten, duration_ms: totalDurationMs };
}

module.exports = {
    computeTruncatedMTTR,
    runFullAggregation,
    runAggregationForType,
    DIMENSIONS,
    getRollingPeriod,
};
