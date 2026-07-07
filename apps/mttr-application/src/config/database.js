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
// PostgreSQL Connection Pool
// ----------------------------------------------------------------------------
// Creates a single shared `pg` connection pool that every other module
// borrows clients from. The pool size, host, credentials, and SSL setting
// are all read from the application configuration.
//
// Exported helpers:
//   • query()        – run a parameterised query and return the result rows
//   • getClient()    – check out a client for a multi-statement transaction
//                      (the caller MUST release it back to the pool)
//   • healthCheck()  – cheap round-trip query used by /api/v1/health
//   • pool           – the raw pg.Pool instance (used by initDb.js)
// ============================================================================

const { Pool } = require('pg');
const applicationConfig = require('./index');
const logger = require('../utils/logger');

// Single connection pool shared by every module that needs database access.
// Reusing connections avoids the cost of repeatedly opening TCP/TLS sessions.
const connectionPool = new Pool({
    host: applicationConfig.db.host,
    port: applicationConfig.db.port,
    database: applicationConfig.db.database,
    user: applicationConfig.db.user,
    password: applicationConfig.db.password,
    ssl: applicationConfig.db.ssl,
    min: applicationConfig.db.min,
    max: applicationConfig.db.max,
    idleTimeoutMillis: applicationConfig.db.idleTimeoutMillis,
    connectionTimeoutMillis: applicationConfig.db.connectionTimeoutMillis,
    // Sets `statement_timeout` on every session pg-node opens against
    // Postgres. Any query that runs past this deadline is cancelled
    // server-side and the client returns to the pool — prevents a
    // pathologically slow query (or a stuck one) from leaking a client
    // for the full idle window. Applies to every path (aggregation,
    // retention, ingestion, health), so the value must sit above any
    // legitimate query duration. See src/config/index.js for the
    // rationale on the default (2 min).
    statement_timeout: applicationConfig.db.statementTimeoutMillis,
});

// `error` is fired for idle clients that disconnect unexpectedly (e.g. the
// DB restarted). We log it; pg will reconnect on the next checkout.
connectionPool.on('error', (poolError) => {
    logger.error('Unexpected database pool error', { error: poolError.message });
});

/**
 * Execute a parameterised SQL statement against the shared pool.
 * Always prefer this over string-interpolating values — `pg` substitutes
 * `$1, $2, …` placeholders safely and prevents SQL injection.
 *
 * @param {string} sqlText  – SQL with `$1`-style placeholders
 * @param {Array}  params   – values to bind to those placeholders
 * @returns {Promise<pg.QueryResult>}
 */
async function query(sqlText, params) {
    const queryStartedAt = Date.now();
    const queryResult = await connectionPool.query(sqlText, params);
    const queryDurationMs = Date.now() - queryStartedAt;

    // Log at debug level so we can profile slow queries without flooding
    // production logs (which run at "info" by default).
    logger.debug('Executed query', {
        text: sqlText.substring(0, 80),
        duration: queryDurationMs,
        rows: queryResult.rowCount,
    });
    return queryResult;
}

/**
 * Check out a dedicated client from the pool. Required for multi-statement
 * transactions (BEGIN/COMMIT/ROLLBACK) since each statement must travel
 * over the same connection.
 *
 * ⚠ The caller is responsible for calling `client.release()` (typically in
 * a `finally` block) — otherwise the pool will leak connections.
 */
async function getClient() {
    return connectionPool.connect();
}

/**
 * Health check used by /api/v1/health and the Docker HEALTHCHECK.
 * Returns the DB server's current timestamp on success; throws on failure
 * (caller is expected to turn the exception into an HTTP 503).
 */
async function healthCheck() {
    const result = await connectionPool.query('SELECT NOW() AS now');
    return result.rows[0].now;
}

module.exports = { query, getClient, healthCheck, pool: connectionPool };
