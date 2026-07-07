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
// /api/v1/admin — Operator-Only Administration Endpoints
// ----------------------------------------------------------------------------
// Every route here is mounted behind both `authenticate` AND `requireAdmin`
// in src/index.js, so callers must present a JWT carrying the `mttr-admin`
// role/group claim.
//
// Routes:
//   POST /api/v1/admin/cache/reset        – force MTTR recomputation
//   GET  /api/v1/admin/ingestion-logs     – inspect recent batch history
//   POST /api/v1/admin/retention/run      – manually trigger retention
//
// These are intended for operators using `curl` or Postman — not for
// automated callers.
// ============================================================================

const express = require('express');
const { runFullAggregation, runAggregationForType, DIMENSIONS } = require('../services/aggregationService');
const { getIngestionLogs } = require('../services/ingestionService');
const { runRetention } = require('../services/retentionService');
const { withMaintenanceLock } = require('../utils/maintenanceLock');
const logger = require('../utils/logger');

// Response body returned when a heavy maintenance operation is refused
// because the advisory lock is held by another caller (cron tick, another
// admin request, or another pod). HTTP 409 Conflict; operator can retry.
const LOCK_HELD_RESPONSE = {
    status: 409,
    body: { error: 'Another maintenance operation is in progress. Please retry shortly.' },
};

const router = express.Router();

// Same whitelist used by /mttr. Sourced from DIMENSIONS so adding a new
// aggregation dimension automatically becomes resettable here too.
const VALID_DIMENSION_TYPES = Object.keys(DIMENSIONS);

/**
 * POST /api/v1/admin/cache/reset[?type=<dim>]
 *
 * Recomputes cached aggregations. Without `?type=…` rebuilds all
 * dimensions in one transaction; with it, rebuilds just that dimension
 * (much faster, useful when only one SN widget looks stale).
 */
router.post('/cache/reset', async (req, res) => {
    const requestedDimensionType = req.query.type;

    try {
        // Branch 1: targeted reset for a single dimension.
        // NOT lock-guarded — matches the cache-miss path in cacheService,
        // which invokes runAggregationForType lazily on user requests.
        // Concurrent computes for the same dimension UPSERT the same
        // cache_key rows (last write wins; no corruption).
        if (requestedDimensionType) {
            if (!VALID_DIMENSION_TYPES.includes(requestedDimensionType)) {
                return res.status(400).json({
                    error: `Invalid type. Must be one of: ${VALID_DIMENSION_TYPES.join(', ')}`,
                });
            }
            const result = await runAggregationForType(requestedDimensionType, req.correlationId);
            return res.json({
                status: 'ok',
                recalculated: [requestedDimensionType],
                ...result,
            });
        }

        // Branch 2: full rebuild of every dimension.
        // Lock-guarded — this is the same heavy path the nightly cron
        // runs. Overlapping calls would waste ~30s of compute each and
        // contend on every cache_key row in mttr_cache; the lock skips
        // duplicate work with a fast 409 to the second caller.
        const lockOutcome = await withMaintenanceLock(
            { correlationId: req.correlationId, source: 'admin_cache_reset_full' },
            () => runFullAggregation(req.correlationId)
        );
        if (!lockOutcome.acquired) {
            return res.status(LOCK_HELD_RESPONSE.status).json({
                ...LOCK_HELD_RESPONSE.body,
                correlation_id: req.correlationId,
            });
        }
        return res.json({
            status: 'ok',
            recalculated: Object.keys(DIMENSIONS),
            ...lockOutcome.result,
        });
    } catch (resetError) {
        logger.error('Cache reset error', {
            correlationId: req.correlationId,
            error: resetError.message,
        });
        return res.status(500).json({ error: 'Aggregation failed', correlation_id: req.correlationId });
    }
});

/**
 * GET /api/v1/admin/ingestion-logs?limit=<n>
 *
 * Returns the most recent ingestion-log rows (newest first), capped at
 * `limit` (1–200, default 50).
 */
router.get('/ingestion-logs', async (req, res) => {
    // Parse + validate `limit`:
    //   • Omitted        → default 50
    //   • 1..200         → use as-is
    //   • >200           → clamp to 200 (preserves prior behaviour for
    //                       clients that ask for "as many as possible")
    //   • ≤0, non-numeric, empty → HTTP 400 with an explanatory body
    //     rather than letting a bad `LIMIT -1` reach Postgres and fail
    //     as a generic 500.
    const DEFAULT_LIMIT = 50;
    const MAX_LIMIT = 200;
    let requestedLimit = DEFAULT_LIMIT;
    if (req.query.limit !== undefined) {
        const parsed = parseInt(req.query.limit, 10);
        if (!Number.isInteger(parsed) || parsed < 1) {
            return res.status(400).json({
                error: `limit must be a positive integer between 1 and ${MAX_LIMIT}`,
                correlation_id: req.correlationId,
            });
        }
        requestedLimit = Math.min(parsed, MAX_LIMIT);
    }

    try {
        const ingestionLogRows = await getIngestionLogs(requestedLimit);
        return res.json({ logs: ingestionLogRows });
    } catch (logsQueryError) {
        logger.error('Ingestion logs error', {
            correlationId: req.correlationId,
            error: logsQueryError.message,
        });
        return res.status(500).json({ error: 'Internal server error', correlation_id: req.correlationId });
    }
});

/**
 * POST /api/v1/admin/retention/run
 *
 * Manually triggers the same retention job that the daily cron runs.
 * Useful right after a large historical import, or when adjusting the
 * retention window and wanting to apply it immediately.
 */
router.post('/retention/run', async (req, res) => {
    try {
        logger.info('Admin: Manual retention job triggered', { correlationId: req.correlationId });
        // Lock-guarded — same heavy path the nightly cron runs. Refuses
        // with 409 if the cron or another admin call is already running.
        const lockOutcome = await withMaintenanceLock(
            { correlationId: req.correlationId, source: 'admin_retention_run' },
            () => runRetention(req.correlationId)
        );
        if (!lockOutcome.acquired) {
            return res.status(LOCK_HELD_RESPONSE.status).json({
                ...LOCK_HELD_RESPONSE.body,
                correlation_id: req.correlationId,
            });
        }
        return res.json({
            status: 'ok',
            message: 'Retention job completed',
            ...lockOutcome.result,
        });
    } catch (retentionError) {
        logger.error('Admin retention job error', {
            correlationId: req.correlationId,
            error: retentionError.message,
        });
        return res.status(500).json({ error: 'Retention job failed', correlation_id: req.correlationId });
    }
});

module.exports = router;
