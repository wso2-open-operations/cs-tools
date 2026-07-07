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
// Maintenance Lock — Postgres advisory-lock guard
// ----------------------------------------------------------------------------
// Wraps the heavy maintenance paths (full aggregation, retention) so at most
// ONE caller executes them at a time, whether that caller is the nightly
// cron tick, a manual `/admin/cache/reset`, or a manual
// `/admin/retention/run`, and whether the pod is single- or multi-instance.
//
// Uses `pg_try_advisory_lock` — a session-scoped, non-blocking advisory lock
// held on a single dedicated pool client for the duration of the work. If
// the lock is held elsewhere the helper returns immediately without
// executing `work`.
//
// Why session-scoped and not transaction-scoped?
//   The maintenance work opens and closes its own transactions internally
//   (aggregation runs one BEGIN/COMMIT per dimension). A transaction-scoped
//   advisory lock would be released on the first COMMIT.
//
// Why a dedicated client?
//   Advisory locks live on the Postgres session, which for pg-node is one
//   pool client. If the lock were acquired on a client that got returned
//   to the pool mid-work, another caller could check out the same client
//   and unwittingly hold the lock.
// ============================================================================

const db = require('../config/database');
const logger = require('./logger');

// Arbitrary bigint identifying the MTTR maintenance lock within this
// Postgres database. If we ever add a second advisory-lock use case in
// this app, pick a fresh unique number so the two don't collide.
const MAINTENANCE_LOCK_KEY = 942_177_382_026;

/**
 * Try to acquire the maintenance advisory lock; if acquired, run `work`
 * and release the lock. If the lock is already held elsewhere, log at
 * INFO and return without invoking `work`.
 *
 * @param {Object} logContext        – merged into every log line (correlationId, source, etc.)
 * @param {Function} work            – async function to execute under the lock
 * @returns {Promise<{acquired: boolean, result?: any}>}
 */
async function withMaintenanceLock(logContext, work) {
    const lockClient = await db.getClient();
    try {
        const acquireResult = await lockClient.query(
            'SELECT pg_try_advisory_lock($1) AS acquired',
            [MAINTENANCE_LOCK_KEY]
        );
        if (!acquireResult.rows[0].acquired) {
            logger.info('Maintenance lock held by another session — skipping', logContext);
            return { acquired: false };
        }
        try {
            const result = await work();
            return { acquired: true, result };
        } finally {
            // Release even if `work` threw so the lock never deadlocks a
            // subsequent caller. The client is released back to the pool
            // in the outer finally.
            await lockClient.query('SELECT pg_advisory_unlock($1)', [MAINTENANCE_LOCK_KEY]);
        }
    } finally {
        lockClient.release();
    }
}

module.exports = { withMaintenanceLock };
