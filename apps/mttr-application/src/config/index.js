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
// Application Configuration
// ----------------------------------------------------------------------------
// Centralises every tunable parameter the app reads from environment
// variables. Each section maps to a discrete concern (database, auth,
// aggregation, retention, rate-limiting, logging).
//
// Defaults are chosen so the service runs locally without any .env file,
// but every production deployment is expected to override them via Choreo
// environment configuration or a deployment .env file.
// ============================================================================

require('dotenv').config();

const applicationConfig = {
    // Runtime environment ("development" | "production" | "test")
    env: process.env.NODE_ENV || 'development',

    // HTTP port the Express server listens on
    port: parseInt(process.env.PORT, 10) || 3000,

    // ── PostgreSQL connection settings ──────────────────────────────────
    // The pool size (min/max) is tuned for Choreo's managed Postgres tier;
    // increase the max if the service is scaled to many concurrent users.
    db: {
        host: process.env.DB_HOST || 'localhost',
        port: parseInt(process.env.DB_PORT, 10) || 5432,
        database: process.env.DB_NAME || 'mttr_db',
        user: process.env.DB_USER || 'mttr_user',
        password: process.env.DB_PASSWORD || '',
        // Choreo-managed Postgres uses TLS with a self-signed CA, so we
        // accept it via rejectUnauthorized=false when DB_SSL=true.
        ssl: process.env.DB_SSL === 'true' ? { rejectUnauthorized: false } : false,
        min: parseInt(process.env.DB_POOL_MIN, 10) || 2,
        max: parseInt(process.env.DB_POOL_MAX, 10) || 10,
        // How long an idle client can sit in the pool before pg closes it.
        // Choreo's managed Postgres tier reaps idle server-side connections
        // after ~60s; our client-side reap must be shorter, otherwise the
        // pool hands out a client whose TCP session the server has already
        // torn down and the next query fails with a cryptic ECONNRESET.
        idleTimeoutMillis: parseInt(process.env.DB_POOL_IDLE_MS, 10) || 30000,
        // How long to wait for a free client from the pool before failing
        // the checkout. Guards against requests hanging indefinitely when
        // the DB is unreachable — instead they fast-fail and surface a 5xx.
        connectionTimeoutMillis: parseInt(process.env.DB_POOL_CONNECT_MS, 10) || 10000,
        // Postgres-side per-query deadline (SET statement_timeout). Any
        // query that runs longer than this is cancelled server-side and
        // the client returns to the pool. Prevents a genuinely stuck
        // query (Postgres pathology, lock wait, network stall) from
        // leaking a pool client for the full server-side idle window.
        // The health handler additionally races on the JS side with a
        // 3s ceiling for a fast 503, but that alone doesn't cancel the
        // orphaned pg promise — this server-side deadline does.
        //
        // 2 min is generous vs. every legitimate MTTR query (aggregation
        // ~10s, retention ~30s per full run under the maintenance lock).
        // Raise if the workload ever grows past that; env-driven.
        statementTimeoutMillis: parseInt(process.env.DB_STATEMENT_TIMEOUT_MS, 10) || 120000,
    },

    // ── OAuth 2.0 / JWT authentication settings ─────────────────────────
    // When `enabled` is false the auth middleware injects a mock admin
    // user — useful for local development. In production Choreo's API
    // gateway already validates the token, so we typically leave this
    // disabled inside the container too (see logic-and-architecture.md §10).
    auth: {
        enabled: process.env.AUTH_ENABLED === 'true',
        jwksUri: process.env.AUTH_JWKS_URI || '',
        issuer: process.env.AUTH_ISSUER || '',
        audience: process.env.AUTH_AUDIENCE || '',
    },

    // ── Aggregation (MTTR calculation) settings ─────────────────────────
    // `cron`           – when to recompute all dimensions (default: nightly)
    // `rollingMonths`  – width of the data window used by the daily compute
    // `minSampleSize`  – below this case-count, fall back to simple average
    //                    instead of P95-truncated mean (avoids misleading
    //                    metrics on tiny samples).
    aggregation: {
        cron: process.env.AGGREGATION_CRON || '0 0 * * *',
        rollingMonths: parseInt(process.env.AGGREGATION_ROLLING_MONTHS, 10) || 12,
        minSampleSize: parseInt(process.env.MIN_SAMPLE_SIZE, 10) || 20,
    },

    // ── Data retention settings ─────────────────────────────────────────
    // The retention job runs as the last step of the daily aggregation cron.
    //   ingestionLogDays  – how long batch-ingest audit rows are kept
    //   caseEventsMonths  – age past which raw cases are summarised &
    //                       deleted (their P95 summary survives forever
    //                       in case_events_summary).
    retention: {
        ingestionLogDays: parseInt(process.env.RETENTION_INGESTION_LOG_DAYS, 10) || 14,
        caseEventsMonths: parseInt(process.env.RETENTION_CASE_EVENTS_MONTHS, 10) || 24,
    },

    // ── Express rate-limiting (applied to all /api/* routes) ────────────
    // Purpose: runaway-loop / accidental-DoS safety net — NOT a customer
    // quota. Choreo's gateway already rate-limits per client at the edge;
    // this is the second line of defence inside the container.
    //
    // Sizing rationale — the MTTR API has one upstream caller (the WSO2
    // ServiceNow instance):
    //   • Scheduled job:  1 call every 10 min → 6/hour, negligible
    //   • Dashboard read: ~12 API calls per homepage load × N viewers
    // A 15-min budget of 1000 fits ~83 dashboard loads (or ~40 viewers
    // refreshing twice each) with lots of headroom, while still catching
    // a runaway `while(true)` from the SN script — which would emit
    // thousands of req/sec and trip the limiter in seconds.
    //
    // Tune per environment via RATE_LIMIT_MAX / RATE_LIMIT_WINDOW_MS.
    rateLimit: {
        windowMs: parseInt(process.env.RATE_LIMIT_WINDOW_MS, 10) || 900000,  // 15 minutes
        max: parseInt(process.env.RATE_LIMIT_MAX, 10) || 1000,
    },

    // Winston log level: "error" | "warn" | "info" | "debug"
    logLevel: process.env.LOG_LEVEL || 'info',
};

module.exports = applicationConfig;
