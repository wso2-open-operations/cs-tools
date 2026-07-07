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
// /api/v1/health — Service Health Endpoint
// ----------------------------------------------------------------------------
// Returns a JSON snapshot of the service's runtime state. Used by:
//   • Choreo's load-balancer / readiness probes
//   • The Dockerfile HEALTHCHECK
//
// Intentionally not behind auth — health probes should never need a token.
// Returns:
//   200 + healthy snapshot when the DB responds.
//   503 + error message when the DB is unreachable.
// ============================================================================

const express = require('express');
const db = require('../config/database');
const { getLastCalculatedAt, getCacheCount } = require('../services/cacheService');
const logger = require('../utils/logger');

const router = express.Router();

// Layered protection against a stuck DB:
//   1. This Promise.race gives the caller a 503 in ≤ 3s regardless of
//      whether pg-node resolves — Choreo readiness probes fire every
//      few seconds and can't wait longer.
//   2. Pool-wide `statement_timeout` (configured on the pg pool — see
//      src/config/database.js) actually CANCELS the underlying query
//      server-side after DB_STATEMENT_TIMEOUT_MS (default 2 min), so
//      the pool client comes back instead of leaking for the full
//      idle window. Without (2), the Promise.race would return a
//      fast 503 but leave the orphaned query hogging a pool client.
const HEALTH_QUERY_TIMEOUT_MS = 3000;

function timeoutAfter(ms, label) {
    return new Promise((_, reject) => {
        setTimeout(() => reject(new Error(`${label} exceeded ${ms}ms`)), ms).unref();
    });
}

/**
 * GET /api/v1/health
 * Returns DB connectivity + ingestion / aggregation freshness indicators.
 */
router.get('/', async (req, res) => {
    try {
        // Wrap the whole read-side in a race against a hard timeout so
        // a stuck query can't hang the probe. If any single query in
        // the chain is slow, the race rejects, catch runs, 503 returns.
        const responseBody = await Promise.race([
            (async () => {
                // Liveness ping first — cheapest way to prove the pool
                // can hand out a working client. If this throws we skip
                // the diagnostic block entirely.
                const dbServerTime = await db.healthCheck();

                // Diagnostic queries in parallel — none depend on each other.
                //   • total_cases: approximate via pg_class.reltuples instead
                //     of SELECT COUNT(*) — the exact count would full-scan
                //     the case_events table on every probe. reltuples is
                //     updated by autovacuum/ANALYZE and is O(1). GREATEST(0,
                //     …) coerces the "-1" sentinel Postgres returns when a
                //     table has never been analysed (fresh install).
                //   • cache_entries / last_aggregation_at: mttr_cache is
                //     small (bounded by dimension × group) so an exact
                //     COUNT / MAX is cheap.
                //   • last_ingestion_at: MAX() uses the ingested_at index
                //     for an O(1) index-only scan.
                const [
                    totalCasesQuery,
                    lastCalculatedAt,
                    cacheRowCount,
                    lastIngestionQuery,
                ] = await Promise.all([
                    db.query(
                        `SELECT GREATEST(0, reltuples::bigint) AS estimate
                         FROM pg_class WHERE relname = 'case_events'`
                    ),
                    getLastCalculatedAt(),
                    getCacheCount(),
                    db.query('SELECT MAX(ingested_at) AS last_ingestion_at FROM ingestion_log'),
                ]);

                return {
                    status: 'healthy',
                    db_time: dbServerTime,
                    total_cases: parseInt(totalCasesQuery.rows[0]?.estimate ?? 0, 10),
                    last_ingestion_at: lastIngestionQuery.rows[0].last_ingestion_at || null,
                    last_aggregation_at: lastCalculatedAt,
                    cache_entries: cacheRowCount,
                };
            })(),
            timeoutAfter(HEALTH_QUERY_TIMEOUT_MS, 'Health probe'),
        ]);
        return res.json(responseBody);
    } catch (healthCheckError) {
        // Any failure here means the DB is down or unreachable — return
        // 503 so external probes can react (restart the pod, alert, etc.).
        //
        // The response body is intentionally generic: /health is
        // unauthenticated so leaking driver text, hostnames, or schema
        // names in the JSON would disclose internal detail to anyone
        // who can reach the endpoint. Full context lives in the log.
        logger.error('Health check failed', {
            error: healthCheckError.message,
            stack: healthCheckError.stack,
        });
        return res.status(503).json({
            status: 'unhealthy',
            error: 'Service temporarily unavailable',
        });
    }
});

module.exports = router;
