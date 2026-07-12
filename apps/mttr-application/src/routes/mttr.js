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
// /api/v1/mttr — MTTR Read API
// ----------------------------------------------------------------------------
// Two endpoints, both mounted behind authenticate middleware:
//
//   GET /api/v1/mttr?type=<dimension>  – returns the cached aggregation
//                                         result for ONE dimension type.
//                                         The valid types are derived from
//                                         the DIMENSIONS object in
//                                         aggregationService.js, so adding
//                                         a new dimension automatically
//                                         exposes it here.
//
//   GET /api/v1/mttr/types             – returns the list of valid
//                                         dimension types. Used by clients
//                                         that want to discover them.
//
// Output shape (per row in `data`):
//   {
//     labels:          {cs_team: "Cloud", priority: "P2", ...},
//     total_cases:     150,
//     excluded_cases:  8,
//     p95_cutoff_hours:72.5,
//     mttr_hours:      18.3,    ← THE headline number
//     simple_avg_hours:25.1,
//     min_sample_met:  true
//   }
// ============================================================================

const express = require('express');
const { query, validationResult } = require('express-validator');
const { getCachedMTTR } = require('../services/cacheService');
const { DIMENSIONS, getRollingPeriod } = require('../services/aggregationService');
const { millisecondsToHours } = require('../utils/format');
const logger = require('../utils/logger');

const router = express.Router();

// Whitelist of dimension types. Sourced from DIMENSIONS so the route stays
// in sync with the compute layer — add a key there and it's accepted here.
const VALID_DIMENSION_TYPES = Object.keys(DIMENSIONS);

/**
 * GET /api/v1/mttr?type=<dimension>
 * Returns cached MTTR data for the requested dimension. If the cache is
 * cold for that dimension, cacheService computes it on the fly first.
 */
router.get(
    '/',
    [
        query('type')
            .isString()
            .isIn(VALID_DIMENSION_TYPES)
            .withMessage(`type must be one of: ${VALID_DIMENSION_TYPES.join(', ')}`),
    ],
    async (req, res) => {
        const schemaValidationErrors = validationResult(req);
        if (!schemaValidationErrors.isEmpty()) {
            return res.status(400).json({ error: 'Validation failed', details: schemaValidationErrors.array() });
        }

        const requestedDimensionType = req.query.type;

        try {
            const cacheRows = await getCachedMTTR(requestedDimensionType, req.correlationId);

            // Empty result: explicitly tell the client "no data yet" instead
            // of returning an ambiguous empty array.
            if (cacheRows.length === 0) {
                return res.json({
                    dimension_type: requestedDimensionType,
                    period: getRollingPeriod(),
                    calculated_at: null,
                    message: 'No data available for this dimension.',
                    data: [],
                });
            }

            // All cache rows for one dimension share the same period and
            // calculated_at — grab them from the first row.
            //
            // pg-node returns Postgres DATE columns as JS Date objects
            // (midnight UTC). JSON.stringify(Date) would emit a full ISO
            // datetime like "2026-01-01T00:00:00.000Z"; the OpenAPI
            // contract for this endpoint declares period as `format: date`
            // ("YYYY-MM-DD"). Format explicitly so this branch and the
            // empty-data branch above (which uses getRollingPeriod)
            // return the same shape. calculated_at is a timestamptz and
            // ISO datetime is the correct shape for it — left alone.
            const period = {
                start: cacheRows[0].period_start.toISOString().split('T')[0],
                end: cacheRows[0].period_end.toISOString().split('T')[0],
            };
            const calculatedAt = cacheRows[0].calculated_at;

            // Reshape DB rows into the public API contract: millisecond
            // columns become rounded `_hours` fields more useful to clients.
            const responseData = cacheRows.map((cacheRow) => ({
                labels: cacheRow.dimension_labels,
                total_cases: cacheRow.total_cases,
                excluded_cases: cacheRow.excluded_cases,
                p95_cutoff_hours: millisecondsToHours(cacheRow.p95_cutoff_ms),
                mttr_hours: millisecondsToHours(cacheRow.truncated_avg_ms),
                simple_avg_hours: millisecondsToHours(cacheRow.simple_avg_ms),
                min_sample_met: cacheRow.min_sample_met,
            }));

            return res.json({
                dimension_type: requestedDimensionType,
                period,
                calculated_at: calculatedAt,
                data: responseData,
            });
        } catch (queryError) {
            logger.error('MTTR query error', {
                dimensionType: requestedDimensionType,
                correlationId: req.correlationId,
                error: queryError.message,
            });
            return res.status(500).json({ error: 'Internal server error', correlation_id: req.correlationId });
        }
    }
);

/**
 * GET /api/v1/mttr/types
 * Returns the whitelist of valid dimension types so clients can dynamically
 * build UI selectors instead of hard-coding the list.
 */
router.get('/types', (req, res) => {
    res.json({ types: VALID_DIMENSION_TYPES });
});

module.exports = router;
