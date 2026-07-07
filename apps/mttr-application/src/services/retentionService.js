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
// Retention Service — Keeps the Database Bounded
// ----------------------------------------------------------------------------
// Runs once per day (as the last step of the aggregation cron) and is
// responsible for three things:
//
//   1. purgeOldIngestionLogs()      – delete batch-audit rows older than N
//                                     days (default 14). Operational noise
//                                     that loses value quickly.
//
//   2. purgeStaleCache(client)      – delete mttr_cache rows that aren't
//                                     part of the most-recent computation.
//                                     Handles dimensions whose labels
//                                     changed (renamed team, deleted
//                                     priority, etc.) so stale rows don't
//                                     linger in API responses.
//
//   3. summariseAndPurgeOldCases()  – the headline retention behaviour:
//                                     summarise raw `case_events` rows
//                                     into quarterly `case_events_summary`
//                                     rows, then delete the raw rows once
//                                     they've aged past the retention cutoff.
//
// Why summarise then delete?
//   The active MTTR computation only needs the last 12 months of raw data.
//   But the /summary/historical endpoint (powering quarterly SN widgets)
//   needs older data — just not row-by-row detail. Pre-computing one MTTR
//   figure per quarter × team × priority × … gives us unlimited historical
//   trend coverage at a fraction of the storage cost (~100× compression).
//
// Why 24 months as the deletion cutoff?
//   12-month rolling window + 12-month safety margin. Even if someone bumps
//   AGGREGATION_ROLLING_MONTHS to 18 there's still headroom.
// ============================================================================

const db = require('../config/database');
const applicationConfig = require('../config');
const logger = require('../utils/logger');
const { computeTruncatedMTTR } = require('../utils/mttrMath');

const INGESTION_LOG_RETENTION_DAYS = applicationConfig.retention.ingestionLogDays;
const CASE_EVENTS_RETENTION_MONTHS = applicationConfig.retention.caseEventsMonths;

// ─── Ingestion-log cleanup ────────────────────────────────────────────────
//
// Pure tactical DELETE — no transaction needed because each ingestion_log
// row is independent of the others.

async function purgeOldIngestionLogs(correlationId) {
    const deleteResult = await db.query(
        `DELETE FROM ingestion_log
         WHERE ingested_at < NOW() - ($1 * INTERVAL '1 day')
         RETURNING id`,
        [INGESTION_LOG_RETENTION_DAYS]
    );
    const deletedRowCount = deleteResult.rowCount;
    if (deletedRowCount > 0) {
        logger.info(
            `Retention: purged ${deletedRowCount} ingestion_log rows older than ${INGESTION_LOG_RETENTION_DAYS} days`,
            { correlationId }
        );
    }
    return deletedRowCount;
}

// ─── mttr_cache: keep only the most recent computation per dimension ──────
//
// After a full recompute the cache will briefly contain BOTH the previous
// run's rows and the current run's rows (because UPSERT only overwrites
// rows whose cache_key matches). For dimensions whose labels changed
// (renamed team, deleted priority, etc.) the old rows would otherwise
// stick around forever — this DELETE removes anything whose calculated_at
// isn't the latest for its dimension_type.
//
// Owns its own pool client — a previous version took the client as an
// argument with a docstring claiming it ran inside the caller's
// transaction, but the caller never actually opened one, so the
// invariant was misleading. The DELETE is a single statement (atomic
// on its own); wrapping it in an explicit BEGIN/COMMIT would not
// change observable behaviour.
//
// KNOWN RACE — deliberately not fixed here:
//   runFullAggregation opens+commits its own transaction, THEN this
//   function runs. Between those two commits the cache holds both the
//   new rows AND stale rows for dimensions whose labels changed.
//   A `/mttr?type=X` read in that millisecond-scale window can see
//   both. Shrinking the window to zero would require sharing a single
//   transaction across the whole cron tick — a bigger refactor. In
//   practice the affected surface (rows for renamed labels only) is
//   small and self-heals on the next request after purge completes.

async function purgeStaleCache(correlationId) {
    const dbClient = await db.getClient();
    try {
        const deleteResult = await dbClient.query(
            `DELETE FROM mttr_cache mc
             USING (
                 SELECT dimension_type, MAX(calculated_at) AS latest
                 FROM mttr_cache
                 GROUP BY dimension_type
             ) keep
             WHERE mc.dimension_type = keep.dimension_type
               AND mc.calculated_at < keep.latest
             RETURNING mc.id`
        );
        const deletedRowCount = deleteResult.rowCount;
        if (deletedRowCount > 0) {
            logger.info(`Retention: purged ${deletedRowCount} stale mttr_cache entries`, { correlationId });
        }
        return deletedRowCount;
    } finally {
        dbClient.release();
    }
}

// P95 truncated-mean lives in src/utils/mttrMath.js — same implementation
// used by aggregationService, so a fix to the algorithm applies to both
// the rolling window and the quarterly summariser at once.

// ─── Quarter helpers ──────────────────────────────────────────────────────
// Quarters are the granularity at which historical summaries are stored.

/** Format a quarter as "YYYY-QN", e.g. quarterLabel(2025, 2) → "2025-Q2". */
function quarterLabel(year, quarter) {
    return `${year}-Q${quarter}`;
}

/** Return the inclusive date range for a given quarter. */
function quarterBounds(year, quarter) {
    const quarterDateRanges = {
        1: { start: `${year}-01-01`, end: `${year}-03-31` },
        2: { start: `${year}-04-01`, end: `${year}-06-30` },
        3: { start: `${year}-07-01`, end: `${year}-09-30` },
        4: { start: `${year}-10-01`, end: `${year}-12-31` }
    };
    return quarterDateRanges[quarter];
}

// ─── Summarise & purge aged case_events ───────────────────────────────────
//
// This is the operationally important retention step.
//
// Algorithm:
//   cutoff = NOW() - CASE_EVENTS_RETENTION_MONTHS
//   for each completed quarter from oldest data through the previous quarter:
//       if quarter already summarised AND quarter older than cutoff:
//           → delete any leftover raw rows for that quarter
//             (catches cases re-closed AFTER the original summary ran)
//       else:
//           → re-summarise the quarter (groups by case_type, priority,
//             cs_team, product, is_patched), writing one row per group
//             via UPSERT to case_events_summary
//           → if the quarter is older than cutoff, also delete the raw rows
//
// Why re-summarise quarters that are within the cutoff?
//   Cases can be re-opened and re-closed weeks later. Re-summarising recent
//   quarters keeps the historical view accurate as that churn happens; we
//   only stop re-summarising once the quarter is permanently frozen.
//
// Everything runs inside ONE transaction so a failure mid-way through any
// quarter leaves both the raw table and the summary table untouched.

async function summariseAndPurgeOldCases(correlationId) {
    const now = new Date();
    const retentionCutoffDate = new Date();
    retentionCutoffDate.setMonth(retentionCutoffDate.getMonth() - CASE_EVENTS_RETENTION_MONTHS);

    // Determine the most recently completed quarter (the current quarter
    // is still in progress and not eligible for summarisation yet).
    const currentQuarterNumber = Math.ceil((now.getMonth() + 1) / 3);
    let previousQuarter = currentQuarterNumber - 1;
    let previousQuarterYear = now.getFullYear();
    if (previousQuarter === 0) { previousQuarter = 4; previousQuarterYear--; }

    // Build the list of quarter candidates. We start from the oldest known
    // year (across raw cases AND existing summaries) so newly-arrived
    // historical data also gets summarised on first run.
    const quarterCandidates = [];
    const oldestRawCaseRow = await db.query(
        `SELECT EXTRACT(YEAR FROM MIN(closed_date))::int AS yr FROM case_events`
    );
    const oldestSummaryRow = await db.query(
        `SELECT EXTRACT(YEAR FROM MIN(period_start))::int AS yr FROM case_events_summary`
    );
    const earliestYearWithData = Math.min(
        (oldestRawCaseRow.rows[0] && oldestRawCaseRow.rows[0].yr) || previousQuarterYear,
        (oldestSummaryRow.rows[0] && oldestSummaryRow.rows[0].yr) || previousQuarterYear
    );

    for (let yearIter = earliestYearWithData; yearIter <= previousQuarterYear; yearIter++) {
        // In the current year we only iterate up through the previous (completed) quarter.
        const lastQuarterThisYear = (yearIter === previousQuarterYear) ? previousQuarter : 4;
        for (let quarterIter = 1; quarterIter <= lastQuarterThisYear; quarterIter++) {
            const bounds = quarterBounds(yearIter, quarterIter);
            quarterCandidates.push({
                label: quarterLabel(yearIter, quarterIter),
                ...bounds,
                // Quarters whose end-date is older than the cutoff are
                // safe to delete raw rows for; newer ones are summary-only.
                canDelete: new Date(bounds.end) < retentionCutoffDate,
            });
        }
    }

    if (quarterCandidates.length === 0) {
        logger.debug('Retention: no completed quarter periods found', { correlationId });
        return { summarised: 0, deleted: 0 };
    }

    let totalSummariesWritten = 0;
    let totalRawRowsDeleted = 0;
    const dbClient = await db.getClient();

    try {
        await dbClient.query('BEGIN');

        for (const quarterPeriod of quarterCandidates) {
            // Every quarter — even ones that already have a summary AND
            // are past the retention cutoff — falls through to the
            // summarise-then-(optionally-)delete path below. Do NOT add
            // a fast-path that skips summarisation.
            //
            // Why: a case can be re-opened + re-closed weeks after its
            // original quarter was already summarised and its raw row
            // deleted. Re-ingestion writes a fresh raw row into the
            // still-open case_events table; if we then skipped
            // re-summarising and just deleted, that late row would be
            // erased without being folded into case_events_summary
            // (silent data loss on the historical dashboard).
            //
            // The UPSERT below (ON CONFLICT DO UPDATE) safely re-writes
            // any existing summary rows, and if no raw rows exist for
            // this quarter (the common case after the first successful
            // retention run) we short-circuit at the
            // `rawCaseRows.rows.length === 0` check a few lines down.

            // Pull the raw durations for the quarter grouped by the
            // summary-table's natural key. We do NOT filter by case_state
            // here because cases are already filtered at ingestion (the SN
            // job only sends Closed/Resolved cases).
            const rawCaseRows = await dbClient.query(
                `SELECT case_type, priority, cs_team, product, is_patched,
                        business_duration_ms
                 FROM case_events
                 WHERE closed_date >= $1
                   AND closed_date <= ($2::date + interval '1 day')
                 ORDER BY case_type, priority, cs_team, product, is_patched, business_duration_ms`,
                [quarterPeriod.start, quarterPeriod.end]
            );

            if (rawCaseRows.rows.length === 0) {
                continue; // No rows in this quarter — nothing to summarise.
            }

            // Group rows by (case_type, priority, cs_team, product, is_patched)
            // so we compute one summary row per unique combination.
            const groupedByDimensions = {};
            for (const rawRow of rawCaseRows.rows) {
                const groupKey = [
                    rawRow.case_type,
                    rawRow.priority || '',
                    rawRow.cs_team,
                    rawRow.product || '',
                    String(rawRow.is_patched),
                ].join('|');

                if (!groupedByDimensions[groupKey]) {
                    groupedByDimensions[groupKey] = {
                        case_type: rawRow.case_type,
                        priority: rawRow.priority || null,
                        cs_team: rawRow.cs_team,
                        product: rawRow.product || null,
                        is_patched: rawRow.is_patched,
                        durations: [],
                    };
                }
                groupedByDimensions[groupKey].durations.push(Number(rawRow.business_duration_ms));
            }

            // For each group compute the P95-truncated mean and UPSERT it.
            for (const groupBucket of Object.values(groupedByDimensions)) {
                const mttrStats = computeTruncatedMTTR(groupBucket.durations);
                if (!mttrStats) continue;

                await dbClient.query(
                    `INSERT INTO case_events_summary
                        (period_label, period_start, period_end, case_type, priority,
                         cs_team, product, is_patched,
                         total_cases, excluded_cases, p95_cutoff_ms,
                         truncated_avg_ms, simple_avg_ms, min_sample_met)
                     VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
                     ON CONFLICT (period_label, case_type, COALESCE(priority, ''),
                                  cs_team, COALESCE(product, ''), COALESCE(is_patched::text, ''))
                     DO UPDATE SET
                        total_cases = EXCLUDED.total_cases,
                        excluded_cases = EXCLUDED.excluded_cases,
                        p95_cutoff_ms = EXCLUDED.p95_cutoff_ms,
                        truncated_avg_ms = EXCLUDED.truncated_avg_ms,
                        simple_avg_ms = EXCLUDED.simple_avg_ms,
                        min_sample_met = EXCLUDED.min_sample_met,
                        summarised_at = NOW()`,
                    [
                        quarterPeriod.label,
                        quarterPeriod.start,
                        quarterPeriod.end,
                        groupBucket.case_type,
                        groupBucket.priority,
                        groupBucket.cs_team,
                        groupBucket.product,
                        groupBucket.is_patched,
                        mttrStats.total_cases,
                        mttrStats.excluded_cases,
                        mttrStats.p95_cutoff_ms,
                        mttrStats.truncated_avg_ms,
                        mttrStats.simple_avg_ms,
                        mttrStats.min_sample_met,
                    ]
                );
                totalSummariesWritten++;
            }

            // Only delete raw rows if the quarter is past the retention cutoff.
            if (quarterPeriod.canDelete) {
                const deleteResult = await dbClient.query(
                    `DELETE FROM case_events
                     WHERE closed_date >= $1 AND closed_date <= ($2::date + interval '1 day')
                     RETURNING id`,
                    [quarterPeriod.start, quarterPeriod.end]
                );
                totalRawRowsDeleted += deleteResult.rowCount;
                logger.info(
                    `Retention: summarised + deleted period ${quarterPeriod.label} — ${Object.keys(groupedByDimensions).length} groups, ${deleteResult.rowCount} raw rows deleted`,
                    { correlationId }
                );
            } else {
                logger.info(
                    `Retention: summarised period ${quarterPeriod.label} — ${Object.keys(groupedByDimensions).length} groups (raw data retained)`,
                    { correlationId }
                );
            }
        }

        await dbClient.query('COMMIT');
    } catch (retentionError) {
        await dbClient.query('ROLLBACK');
        logger.error('Retention: summarise/purge failed', {
            correlationId,
            error: retentionError.message,
        });
        throw retentionError;
    } finally {
        dbClient.release();
    }

    logger.info(
        `Retention: total ${totalSummariesWritten} summaries written, ${totalRawRowsDeleted} raw rows deleted`,
        { correlationId }
    );
    return { summarised: totalSummariesWritten, deleted: totalRawRowsDeleted };
}

// ─── Public entry point: run every retention step ────────────────────────
//
// Called once per day from the aggregation cron. Also exposed via
// POST /admin/retention/run for manual triggering.

async function runRetention(correlationId) {
    logger.info('Retention: starting', { correlationId });
    const ingestionLogsPurged = await purgeOldIngestionLogs(correlationId);
    const caseRetentionResult = await summariseAndPurgeOldCases(correlationId);
    logger.info('Retention: completed', {
        correlationId,
        ingestionPurged: ingestionLogsPurged,
        ...caseRetentionResult,
    });
    return { ingestion_log_purged: ingestionLogsPurged, ...caseRetentionResult };
}

module.exports = {
    purgeOldIngestionLogs,
    purgeStaleCache,
    summariseAndPurgeOldCases,
    runRetention,
};
