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
// /api/v1/summary — Historical (Archived) Quarterly Summaries
// ----------------------------------------------------------------------------
// The aggregation/cache layer powers the rolling 12-month window. Once
// raw `case_events` rows are summarised and deleted by the retention job
// (see retentionService.js), the only place historical MTTR survives is
// `case_events_summary`.
//
// These routes expose that table in a format ready for time-series charts.
//
//   GET /api/v1/summary/historical?case_type=…&group_by=…
//      Returns a {periods, series} structure where:
//        • periods is a sorted list of quarter labels ("2024-Q1",…)
//        • series is keyed by the chosen group (team/priority/product)
//          with each entry containing per-period MTTR statistics.
//
//   GET /api/v1/summary/periods
//      Returns the list of quarters that have at least one summary row.
// ============================================================================

const express = require('express');
const { query, validationResult } = require('express-validator');
const db = require('../config/database');
const { millisecondsToHours } = require('../utils/format');
const logger = require('../utils/logger');

const router = express.Router();

// Allow-list of accepted `group_by` values, mapped to the actual DB
// column each one resolves to. Held as a Map so the handler can
// `.has()` before interpolating anything into SQL. If the
// express-validator `.isIn()` whitelist above ever widens without a
// matching update here, `.has()` returns false and the request 400s
// — rather than silently interpolating an unvalidated identifier.
const GROUP_BY_COLUMN_MAP = new Map([
    ['team', 'cs_team'],
    ['priority', 'priority'],
    ['product', 'product'],
]);

// Startup allowlist assertion on the VALUES of the map. The `.has()`
// guard below blocks unknown request inputs, but a future maintainer
// adding a poisoned entry like ['team', 'cs_team; DROP TABLE …'] would
// silently become live SQLi. Reject anything that isn't a plain SQL
// identifier at module load — same pattern used in
// aggregationService.js for DIMENSIONS[].groupBy.
const SAFE_SQL_IDENTIFIER = /^[a-z_][a-z0-9_]*$/i;
for (const [key, column] of GROUP_BY_COLUMN_MAP) {
    if (!SAFE_SQL_IDENTIFIER.test(column)) {
        throw new Error(`Unsafe SQL column in GROUP_BY_COLUMN_MAP['${key}']: ${column}`);
    }
}

/**
 * GET /api/v1/summary/historical
 *
 * Pivots the long-format `case_events_summary` rows into a time-series
 * structure ready for charting libraries:
 *
 *   {
 *     case_type: "Incident",
 *     group_by:  "team",
 *     periods:   ["2024-Q1","2024-Q2",…],
 *     series: {
 *       "IAM":   { "2024-Q1": {mttr_hours, total_cases, …}, … },
 *       "Cloud": { … }
 *     }
 *   }
 *
 * Query params:
 *   case_type  – required: "Incident" or "Query"
 *   group_by   – optional: "priority" | "team" | "product" (default: team)
 */
router.get(
    '/historical',
    [
        query('case_type')
            .isString()
            .isIn(['Incident', 'Query'])
            .withMessage('case_type must be "Incident" or "Query"'),
        query('group_by')
            .optional()
            .isIn(['priority', 'team', 'product'])
            .withMessage('group_by must be one of: priority, team, product'),
    ],
    async (req, res) => {
        const schemaValidationErrors = validationResult(req);
        if (!schemaValidationErrors.isEmpty()) {
            return res.status(400).json({ error: 'Validation failed', details: schemaValidationErrors.array() });
        }

        const requestedCaseType = req.query.case_type;
        const requestedGroupBy = req.query.group_by || 'team';

        // Belt-and-braces: even though express-validator already rejected
        // out-of-range values above, we refuse anything not present in
        // GROUP_BY_COLUMN_MAP before touching SQL. If both checks ever
        // drift out of sync, this one still blocks injection.
        if (!GROUP_BY_COLUMN_MAP.has(requestedGroupBy)) {
            return res.status(400).json({
                error: `group_by must be one of: ${Array.from(GROUP_BY_COLUMN_MAP.keys()).join(', ')}`,
            });
        }
        const groupByColumn = GROUP_BY_COLUMN_MAP.get(requestedGroupBy);

        try {
            const summaryRows = await db.query(
                `SELECT period_label, period_start, period_end,
                        ${groupByColumn} AS group_label,
                        total_cases, excluded_cases,
                        p95_cutoff_ms, truncated_avg_ms, simple_avg_ms,
                        min_sample_met, summarised_at
                 FROM case_events_summary
                 WHERE case_type = $1
                 ORDER BY period_start, ${groupByColumn}`,
                [requestedCaseType]
            );

            // Pivot rows into { series[group][period] = stats } so the
            // client doesn't need to do this transformation client-side.
            const uniquePeriods = new Set();
            const seriesByGroupLabel = {};

            for (const summaryRow of summaryRows.rows) {
                uniquePeriods.add(summaryRow.period_label);
                const groupLabel = summaryRow.group_label || 'Unknown';

                if (!seriesByGroupLabel[groupLabel]) seriesByGroupLabel[groupLabel] = {};
                seriesByGroupLabel[groupLabel][summaryRow.period_label] = {
                    mttr_hours: millisecondsToHours(summaryRow.truncated_avg_ms),
                    simple_avg_hours: millisecondsToHours(summaryRow.simple_avg_ms),
                    total_cases: summaryRow.total_cases,
                    min_sample_met: summaryRow.min_sample_met,
                };
            }

            // Periods sort lexicographically — "2024-Q1" < "2024-Q2" < … —
            // which conveniently matches chronological order.
            const sortedPeriods = Array.from(uniquePeriods).sort();

            return res.json({
                case_type: requestedCaseType,
                group_by: requestedGroupBy,
                periods: sortedPeriods,
                series: seriesByGroupLabel,
            });
        } catch (historicalQueryError) {
            logger.error('Historical summary query error', {
                correlationId: req.correlationId,
                error: historicalQueryError.message,
            });
            return res.status(500).json({ error: 'Internal server error', correlation_id: req.correlationId });
        }
    }
);

/**
 * GET /api/v1/summary/periods
 *
 * Returns the distinct quarter labels (and their date bounds) that have at
 * least one summary row. Useful for UI selectors that want to show only
 * the quarters with actual data.
 */
router.get('/periods', async (req, res) => {
    try {
        const periodRows = await db.query(
            `SELECT DISTINCT period_label, period_start, period_end
             FROM case_events_summary
             ORDER BY period_start`
        );
        return res.json({ periods: periodRows.rows });
    } catch (periodsQueryError) {
        logger.error('Summary periods query error', {
            correlationId: req.correlationId,
            error: periodsQueryError.message,
        });
        return res.status(500).json({ error: 'Internal server error', correlation_id: req.correlationId });
    }
});

module.exports = router;
