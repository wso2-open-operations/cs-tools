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
// Ingestion Service
// ----------------------------------------------------------------------------
// Responsible for the WRITE half of the system: validating and persisting
// batches of resolved-case records pushed in from ServiceNow's scheduled
// job (or from a one-off bulk import).
//
// Public API:
//   • validateCase(record)        – returns null when valid, otherwise an
//                                    error reason string
//   • ingestBatch(batchId, cases) – UPSERTs the whole batch in a single
//                                    transaction and writes an audit row
//   • getIngestionLogs(limit)     – read recent batch-ingest history
//
// Why UPSERT? ServiceNow can re-send the same case multiple times if it
// gets reopened/re-closed. `case_sys_id` is the natural key, so we INSERT
// new rows and UPDATE existing ones in a single statement — guaranteeing
// no duplicates regardless of how many times a case appears in a batch.
// ============================================================================

const db = require('../config/database');
const logger = require('../utils/logger');
const { v4: uuidv4 } = require('uuid');

// Whitelist of case types we accept. Anything else is rejected at the
// validation gate — the SN scheduled job also filters by this, but we
// re-check here as defence in depth (e.g. for direct API or bulk imports).
const ALLOWED_CASE_TYPES = ['Incident', 'Query'];

/**
 * Validate a single case record from the inbound batch payload.
 *
 * Returns `null` when the record is valid; otherwise returns a short
 * human-readable reason string that gets logged in `ingestion_log`.
 * Invalid records are skipped (the rest of the batch still ingests).
 *
 * @param {Object} caseRecord  – raw case object from the SN batch payload
 * @returns {string|null}      – null when valid, else the rejection reason
 */
function validateCase(caseRecord) {
    // case_sys_id is the natural key for UPSERT — must be present and fit
    // the column width (32 chars, matches ServiceNow sys_id format).
    if (!caseRecord.case_sys_id || typeof caseRecord.case_sys_id !== 'string') {
        return 'missing or invalid case_sys_id';
    }
    if (caseRecord.case_sys_id.length > 32) {
        return 'case_sys_id exceeds 32 characters';
    }

    // business_duration_ms is the value MTTR is computed from — a zero,
    // negative, or non-numeric value would corrupt aggregation OR abort
    // the whole batch at INSERT time. Reject upfront: only finite
    // numbers > 0 pass. Numeric strings ("5000") and floats (1.5) are
    // rejected too — the SN scheduled job always sends parsed integers
    // via GlideDuration.dateNumericValue(), so anything else is a bug
    // on the caller's side that we want visible in the rejection log.
    if (caseRecord.business_duration_ms === null || caseRecord.business_duration_ms === undefined) {
        return 'business_duration_ms is null or missing';
    }
    if (typeof caseRecord.business_duration_ms !== 'number' || !Number.isFinite(caseRecord.business_duration_ms)) {
        return 'business_duration_ms must be a finite number';
    }
    // Integer required — the DB column is BIGINT and would reject a
    // decimal at INSERT time (rolling back the whole batch). SN always
    // sends parsed integers via GlideDuration.dateNumericValue.
    if (!Number.isInteger(caseRecord.business_duration_ms)) {
        return 'business_duration_ms must be an integer (milliseconds)';
    }
    if (caseRecord.business_duration_ms <= 0) {
        return 'business_duration_ms is zero or negative';
    }

    // Date / type / state / product / team are all NOT NULL columns in the
    // database — reject early instead of producing a SQL constraint error.
    if (!caseRecord.created_date) {
        return 'created_date is missing';
    }
    // Only accept string timestamps (matches the OpenAPI contract's
    // `format: date-time`; SN sends "YYYY-MM-DD HH:MM:SS" strings).
    // A numeric like 12345 would otherwise be parsed by Postgres as
    // "year 12345 AD" — silent corruption of the historical dataset.
    if (typeof caseRecord.created_date !== 'string') {
        return 'created_date must be an ISO-format string';
    }
    // Reject values that Postgres would fail to parse as a timestamp
    // ("not a date", "2024-13-45", etc.). Date.parse returns NaN for
    // unparseable input; a finite result means the value is at least
    // date-shaped and won't rollback the transaction.
    if (!Number.isFinite(Date.parse(caseRecord.created_date))) {
        return 'created_date is not a valid timestamp';
    }
    if (!caseRecord.case_type) {
        return 'case_type is missing';
    }
    if (!ALLOWED_CASE_TYPES.includes(caseRecord.case_type)) {
        // Truncate + String-coerce the echoed value — it's caller-supplied
        // and lands in ingestion_log.rejected_details (JSONB) and the
        // admin /ingestion-logs response. Prevents an attacker (or a
        // buggy caller) from stuffing arbitrarily large strings into
        // the audit log via a single bad batch record.
        return 'unsupported case_type: ' + String(caseRecord.case_type).substring(0, 50);
    }
    if (!caseRecord.case_state) {
        return 'case_state is missing';
    }
    // Length guards below match the VARCHAR widths in sql/init.sql.
    // Without them an overlong value would slip past validation, hit
    // the UPSERT, and abort the whole batch with a generic Postgres
    // "value too long for type character varying(N)" — rolling back
    // up to MAX_BATCH_SIZE valid records with it.
    if (caseRecord.case_state.length > 30) {
        return 'case_state exceeds 30 characters';
    }
    if (!caseRecord.product) {
        return 'product is missing';
    }
    if (caseRecord.product.length > 100) {
        return 'product exceeds 100 characters';
    }
    if (!caseRecord.cs_team) {
        return 'cs_team is missing';
    }
    if (caseRecord.cs_team.length > 100) {
        return 'cs_team exceeds 100 characters';
    }

    // Priority only applies to Incidents (P1–P4). Queries legitimately
    // arrive with an empty priority — we don't reject those.
    if (caseRecord.case_type === 'Incident' && !caseRecord.priority) {
        return 'priority is missing for Incident';
    }
    if (caseRecord.priority && caseRecord.priority.length > 30) {
        return 'priority exceeds 30 characters';
    }

    // Normalise case_state to lowercase in-place so the composite index
    // idx_case_events_state_closed(case_state, closed_date) can serve
    // the aggregation queries. Postgres cannot use a B-tree index on
    // case_state to satisfy a `LOWER(case_state) = ...` predicate, so
    // the previous query shape forced a full seq scan of case_events
    // on every nightly aggregation tick. Normalising here (rather than
    // wrapping every query) puts the transform once, on the write side.
    caseRecord.case_state = caseRecord.case_state.toLowerCase();

    return null;
}

/**
 * Ingest a batch of cases into `case_events` via UPSERT and write an
 * audit row to `ingestion_log` — all inside one transaction so partial
 * failures roll back cleanly.
 *
 * Returns counts of inserted / updated / rejected records plus the
 * rejection reasons (returned to the caller for debugging).
 *
 * @param {string} batchId          – caller-supplied batch identifier
 * @param {Array}  cases            – array of case objects (already array-validated by the route)
 * @param {string} [correlationId]  – optional request correlation ID, echoed into every log line
 */
async function ingestBatch(batchId, cases, correlationId) {
    let insertedCount = 0;
    let updatedCount = 0;
    let rejectedCount = 0;
    const rejectedDetails = [];

    // Borrow a dedicated client because we need BEGIN/COMMIT — the
    // pool-level `query()` helper can't span statements.
    const dbClient = await db.getClient();
    try {
        await dbClient.query('BEGIN');

        for (const caseRecord of cases) {
            const validationError = validateCase(caseRecord);
            if (validationError) {
                rejectedCount++;
                rejectedDetails.push({
                    case_sys_id: caseRecord.case_sys_id || 'unknown',
                    reason: validationError,
                });
                continue;
            }

            // is_patched may arrive as a real boolean (from the SN script)
            // or as the string "true" (from manual API calls / curl). Normalise
            // both to a proper boolean for the BOOLEAN column.
            const isPatchedBoolean = caseRecord.is_patched === true
                || String(caseRecord.is_patched).toLowerCase() === 'true';

            // UPSERT by case_sys_id. The `xmax = 0` trick on the RETURNING
            // clause distinguishes inserts from updates:
            //   xmax == 0 → row was just inserted
            //   xmax != 0 → row already existed and got updated
            const upsertResult = await dbClient.query(
                `INSERT INTO case_events
                    (case_sys_id, product, cs_team, business_duration_ms, created_date, closed_date,
                     case_type, priority, is_patched, case_state, ingested_at, updated_at)
                 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
                 ON CONFLICT (case_sys_id) DO UPDATE SET
                    product = EXCLUDED.product,
                    cs_team = EXCLUDED.cs_team,
                    business_duration_ms = EXCLUDED.business_duration_ms,
                    created_date = EXCLUDED.created_date,
                    closed_date = EXCLUDED.closed_date,
                    case_type = EXCLUDED.case_type,
                    priority = EXCLUDED.priority,
                    is_patched = EXCLUDED.is_patched,
                    case_state = EXCLUDED.case_state,
                    updated_at = NOW()
                 RETURNING (xmax = 0) AS is_insert`,
                [
                    caseRecord.case_sys_id,
                    caseRecord.product,
                    caseRecord.cs_team,
                    caseRecord.business_duration_ms,
                    caseRecord.created_date,
                    caseRecord.closed_date || null,
                    caseRecord.case_type,
                    caseRecord.priority || null,
                    isPatchedBoolean,
                    caseRecord.case_state,
                ]
            );

            if (upsertResult.rows[0].is_insert) {
                insertedCount++;
            } else {
                updatedCount++;
            }
        }

        // Audit row — surfaced via /admin/ingestion-logs so operators
        // can see batch history without DB access.
        await dbClient.query(
            `INSERT INTO ingestion_log
                (batch_id, records_received, records_inserted, records_updated, records_rejected, rejected_details)
             VALUES ($1, $2, $3, $4, $5, $6)`,
            [batchId, cases.length, insertedCount, updatedCount, rejectedCount, JSON.stringify(rejectedDetails)]
        );

        await dbClient.query('COMMIT');
    } catch (transactionError) {
        // Roll back the entire batch on any failure — we never want a
        // partial ingest where some rows landed but the audit row didn't.
        await dbClient.query('ROLLBACK');
        logger.error('Batch ingestion failed', {
            batchId,
            correlationId,
            error: transactionError.message,
        });
        throw transactionError;
    } finally {
        // Always return the client to the pool, even on error, or we
        // leak connections.
        dbClient.release();
    }

    logger.info('Batch ingested', {
        batchId,
        correlationId,
        received: cases.length,
        inserted: insertedCount,
        updated: updatedCount,
        rejected: rejectedCount,
    });

    return {
        batch_id: batchId,
        received: cases.length,
        inserted: insertedCount,
        updated: updatedCount,
        rejected: rejectedCount,
        rejected_details: rejectedDetails,
    };
}

/**
 * Fetch the most recent N ingestion-log rows, newest first.
 * Used by /admin/ingestion-logs.
 */
async function getIngestionLogs(limit = 50) {
    const queryResult = await db.query(
        'SELECT * FROM ingestion_log ORDER BY ingested_at DESC LIMIT $1',
        [limit]
    );
    return queryResult.rows;
}

module.exports = { ingestBatch, getIngestionLogs, validateCase };
