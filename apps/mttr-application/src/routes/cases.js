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
// /api/v1/cases — Inbound Case Ingestion
// ----------------------------------------------------------------------------
// Two endpoints, both mounted behind authenticate middleware:
//
//   POST /api/v1/cases/batch       – called by the SN scheduled job every
//                                     10 minutes; capped at MAX_BATCH_SIZE
//                                     (500) records per request.
//
//   POST /api/v1/cases/bulk-import – one-off backfill endpoint; capped at
//                                     MAX_BULK_SIZE (5000) records so an
//                                     accidental flood doesn't take down
//                                     the regular ingestion path.
//
// Both endpoints share the same payload shape and delegate to the
// ingestion service for validation + UPSERT.
// ============================================================================

const express = require('express');
const { body, validationResult } = require('express-validator');
const { ingestBatch } = require('../services/ingestionService');
const { requireAdmin } = require('../middleware/auth');
const logger = require('../utils/logger');

const router = express.Router();

// Hard upper bounds on payload array size. These map to ServiceNow's
// REST message timeout (60s) and our DB transaction time budget.
const MAX_BATCH_SIZE = 500;     // SN scheduled job's per-tick batch limit
const MAX_BULK_SIZE = 5000;     // Backfill endpoint — manual operator use only

// Must match the ingestion_log.batch_id column (VARCHAR(100)) in
// sql/init.sql. An overlong value used to fail deep inside the INSERT
// with an opaque Postgres error and roll back the whole batch; we
// reject it at the validator now instead.
const MAX_BATCH_ID_LENGTH = 100;

/**
 * POST /api/v1/cases/batch
 *
 * Called by the SN scheduled job every 10 minutes. The job sends only
 * cases whose `sys_updated_on` is newer than its watermark, so this
 * endpoint is the hot ingestion path for production.
 */
router.post(
    '/batch',
    [
        // Schema-level validation. The deeper per-case validation
        // (durations, required fields, etc.) happens inside ingestBatch().
        body('batch_id')
            .isString().notEmpty().withMessage('batch_id is required')
            .isLength({ max: MAX_BATCH_ID_LENGTH }).withMessage(`batch_id must be ${MAX_BATCH_ID_LENGTH} characters or fewer`),
        body('cases').isArray({ min: 1, max: MAX_BATCH_SIZE }).withMessage(`cases must be an array of 1-${MAX_BATCH_SIZE} items`),
    ],
    async (req, res) => {
        const schemaValidationErrors = validationResult(req);
        if (!schemaValidationErrors.isEmpty()) {
            return res.status(400).json({ error: 'Validation failed', details: schemaValidationErrors.array() });
        }

        try {
            const ingestionResult = await ingestBatch(req.body.batch_id, req.body.cases, req.correlationId);
            return res.json(ingestionResult);
        } catch (ingestionError) {
            logger.error('Batch ingest endpoint error', {
                error: ingestionError.message,
                correlationId: req.correlationId,
            });
            return res.status(500).json({ error: 'Internal server error', correlation_id: req.correlationId });
        }
    }
);

/**
 * POST /api/v1/cases/bulk-import
 *
 * Same payload, larger size cap. Use this for one-time historical
 * imports — NOT for the regular SN sync (which should hit /batch).
 *
 * Admin-only: the 5000-record cap makes this a soft DoS surface, and
 * it also writes an audit row to ingestion_log on every call, so we
 * gate it behind the `mttr-admin` role.
 */
router.post(
    '/bulk-import',
    requireAdmin,
    [
        body('batch_id')
            .isString().notEmpty().withMessage('batch_id is required')
            .isLength({ max: MAX_BATCH_ID_LENGTH }).withMessage(`batch_id must be ${MAX_BATCH_ID_LENGTH} characters or fewer`),
        body('cases').isArray({ min: 1, max: MAX_BULK_SIZE }).withMessage(`cases must be an array of 1-${MAX_BULK_SIZE} items`),
    ],
    async (req, res) => {
        const schemaValidationErrors = validationResult(req);
        if (!schemaValidationErrors.isEmpty()) {
            return res.status(400).json({ error: 'Validation failed', details: schemaValidationErrors.array() });
        }

        try {
            const ingestionResult = await ingestBatch(req.body.batch_id, req.body.cases, req.correlationId);
            return res.json(ingestionResult);
        } catch (ingestionError) {
            logger.error('Bulk import endpoint error', {
                error: ingestionError.message,
                correlationId: req.correlationId,
            });
            return res.status(500).json({ error: 'Internal server error', correlation_id: req.correlationId });
        }
    }
);

module.exports = router;
