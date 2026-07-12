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
// Cache Service — Read-Through Layer over mttr_cache
// ----------------------------------------------------------------------------
// Thin wrapper around the `mttr_cache` table used by the read APIs.
//
// Why a separate service?
//   • Hides the SQL behind a function name the routes can use.
//   • Implements a "compute-on-miss" fallback so the very first request
//     (or a request right after a manual reset) still returns data instead
//     of an empty array.
//
// The heavy lifting (computing P95 truncated means) lives in
// aggregationService.js — this module only reads, with one delegated
// write path on cache miss.
// ============================================================================

const db = require('../config/database');
const logger = require('../utils/logger');
const { runAggregationForType } = require('./aggregationService');

// Shared SELECT for the cache table. Extracted as a constant so the
// projection list stays in sync between the on-hit and on-miss paths.
const CACHE_SELECT_SQL = `
    SELECT cache_key, dimension_type, dimension_labels, period_start, period_end,
           total_cases, excluded_cases, p95_cutoff_ms, truncated_avg_ms, simple_avg_ms,
           min_sample_met, calculated_at
    FROM mttr_cache
    WHERE dimension_type = $1
    ORDER BY cache_key
`;

// In-flight aggregation single-flight: when the cache is cold for a
// dimension, the first request kicks off `runAggregationForType` and
// every concurrent request for the same dimension awaits the same
// promise instead of firing its own duplicate compute. Cleared on
// settle so the next legitimate cache miss can trigger a fresh run.
//
// Without this, a fan-out from 12 SN dashboard widgets right after a
// deploy would start 12 concurrent full aggregations against Postgres.
const inFlightAggregations = new Map();

/**
 * Read every cached row for one dimension type. Falls back to an
 * on-the-fly recompute if the cache is empty for that dimension
 * (e.g. fresh deployment, or right after `/admin/cache/reset`).
 *
 * Concurrent cache misses for the same dimension coalesce onto a
 * single aggregation run — see `inFlightAggregations` above.
 *
 * When several requests fan into a single in-flight aggregation, each
 * request logs its own "awaiting" line with its own correlation ID —
 * so a slow-request investigation can find both the request that
 * triggered the compute and every request that piggy-backed on it.
 *
 * @param {string} dimensionType         – one of the keys in DIMENSIONS
 * @param {string} [correlationId]       – request correlation ID; echoed
 *                                          into cache-miss + aggregation logs
 * @returns {Promise<Array<Object>>}     – raw cache rows (route layer reshapes)
 */
async function getCachedMTTR(dimensionType, correlationId) {
    let queryResult = await db.query(CACHE_SELECT_SQL, [dimensionType]);

    if (queryResult.rows.length === 0) {
        let pending = inFlightAggregations.get(dimensionType);
        if (pending) {
            logger.info('Cache miss, awaiting in-flight aggregation', { dimensionType, correlationId });
        } else {
            logger.info('Cache miss, computing aggregation on-the-fly', { dimensionType, correlationId });
            pending = runAggregationForType(dimensionType, correlationId)
                .finally(() => inFlightAggregations.delete(dimensionType));
            inFlightAggregations.set(dimensionType, pending);
        }
        await pending;
        queryResult = await db.query(CACHE_SELECT_SQL, [dimensionType]);
    }

    return queryResult.rows;
}

/**
 * Returns the most recent `calculated_at` across the entire cache. Used by
 * /health to surface "when was MTTR last refreshed" to API consumers.
 */
async function getLastCalculatedAt() {
    const queryResult = await db.query(
        'SELECT MAX(calculated_at) AS last_calculated_at FROM mttr_cache'
    );
    return queryResult.rows[0]?.last_calculated_at || null;
}

/**
 * Total number of cache rows across all dimensions — surfaced on /health
 * so operators can spot a sudden drop (typically a sign aggregation failed).
 */
async function getCacheCount() {
    const queryResult = await db.query('SELECT COUNT(*) AS count FROM mttr_cache');
    return parseInt(queryResult.rows[0].count, 10);
}

module.exports = { getCachedMTTR, getLastCalculatedAt, getCacheCount };
