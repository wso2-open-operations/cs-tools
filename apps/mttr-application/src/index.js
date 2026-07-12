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
// MTTR Application — Express Bootstrap
// ----------------------------------------------------------------------------
// Wires together the middleware stack, mounts every route module, and starts
// the HTTP server + the daily aggregation cron job.
//
// Boot sequence:
//   1. Apply security middleware (helmet, CORS, rate-limit, body parser).
//   2. Install a per-request access log.
//   3. Mount routes (health → cases → mttr → admin → summary).
//   4. Install 404 + global error handlers.
//   5. Start listening, then start the cron job that recomputes MTTR daily.
// ============================================================================

const express = require('express');
const helmet = require('helmet');
const cors = require('cors');
const rateLimit = require('express-rate-limit');
const applicationConfig = require('./config');
const logger = require('./utils/logger');
const { authenticate, requireAdmin } = require('./middleware/auth');
const { correlationId } = require('./middleware/correlationId');
const { startAggregationJob } = require('./jobs/aggregationJob');
const { pool } = require('./config/database');

// ── Route modules ─────────────────────────────────────────────────────────
const casesRoutes = require('./routes/cases');
const mttrRoutes = require('./routes/mttr');
const adminRoutes = require('./routes/admin');
const healthRoutes = require('./routes/health');
const summaryRoutes = require('./routes/summary');

const expressApp = express();

// Trust the Choreo gateway that sits one hop in front of this container.
// Without this, req.ip resolves to the gateway's internal IP for every
// request — meaning the rate limiter counts against one global bucket
// instead of per real-client IP (from the gateway-set X-Forwarded-For
// header). `1` = trust exactly one proxy in the chain.
expressApp.set('trust proxy', 1);

// ─── Security & body parsing ──────────────────────────────────────────────

// helmet sets a sensible set of HTTP security headers with strict defaults.
expressApp.use(helmet());

// CORS: deny-by-default. Only the two well-known local labels get the
// wildcard origin; every other value of NODE_ENV (production, staging,
// perf, uat, preprod, or a typo) uses the CORS_ALLOWED_ORIGINS
// allowlist. If that env-var is unset in a restricted environment the
// filter yields an empty array — browsers are then CORS-blocked, which
// is the intended safe default.
expressApp.use(cors({
    origin: (applicationConfig.env === 'development' || applicationConfig.env === 'test')
        ? '*'
        : (process.env.CORS_ALLOWED_ORIGINS || '').split(',').filter(Boolean),
}));

// 10MB body limit accommodates a worst-case 5000-case bulk-import payload.
expressApp.use(express.json({ limit: '10mb' }));

// Correlation-ID: reuse the caller's X-Correlation-Id header if present
// (that's how the SN scheduled job's batch_id flows into our logs) or
// mint a fresh UUID. Installed before the access log so every line
// emitted below can quote req.correlationId.
expressApp.use(correlationId);

// ─── Rate limiting (applied to /api/* only) ───────────────────────────────

const apiRateLimiter = rateLimit({
    windowMs: applicationConfig.rateLimit.windowMs,
    max: applicationConfig.rateLimit.max,
    standardHeaders: true,    // emit RateLimit-* headers
    legacyHeaders: false,     // suppress X-RateLimit-* (deprecated)
    message: { error: 'Too many requests, please try again later.' },
});
expressApp.use('/api/', apiRateLimiter);

// ─── Per-request access log ───────────────────────────────────────────────

expressApp.use((req, res, next) => {
    const requestStartedAt = Date.now();
    res.on('finish', () => {
        logger.info(`${req.method} ${req.originalUrl} ${res.statusCode} ${Date.now() - requestStartedAt}ms`, {
            correlationId: req.correlationId,
        });
    });
    next();
});

// ─── Route mounting ───────────────────────────────────────────────────────
//
// Health is intentionally first and unauthenticated — load balancers,
// readiness probes, and the Docker HEALTHCHECK all need to hit it without
// credentials.

// Health (no auth — used by load balancers / Choreo probes)
expressApp.use('/api/v1/health', healthRoutes);

// Cases ingestion (authenticated — SN scheduled job)
expressApp.use('/api/v1/cases', authenticate, casesRoutes);

// MTTR query (authenticated — SN widgets)
expressApp.use('/api/v1/mttr', authenticate, mttrRoutes);

// Admin (authenticated + admin role required)
expressApp.use('/api/v1/admin', authenticate, requireAdmin, adminRoutes);

// Historical summaries (authenticated — quarterly trend charts)
expressApp.use('/api/v1/summary', authenticate, summaryRoutes);

// ─── 404 + global error handlers ──────────────────────────────────────────

expressApp.use((req, res) => {
    res.status(404).json({ error: 'Not found', correlation_id: req.correlationId });
});

// Any error thrown synchronously or passed to next(err) lands here.
// Returns a generic 500 to the client and logs the full stack for ops.
// The correlation ID is echoed in the response so a caller can quote it
// back when reporting an issue — no need to correlate by timestamp.
expressApp.use((err, req, res, next) => {
    logger.error('Unhandled error', {
        error: err.message,
        stack: err.stack,
        correlationId: req.correlationId,
    });
    res.status(500).json({ error: 'Internal server error', correlation_id: req.correlationId });
});

// ─── Process-level error handlers ─────────────────────────────────────────
//
// Node 18+ terminates on unhandled rejections and uncaught exceptions
// with a fatal message to stderr and no structured log line — meaning
// pool `error` events, cron callback slips, or startup races can crash
// the pod silently. Install top-level handlers that route both classes
// through our winston logger so ops sees them in the log aggregator
// with the same shape as every other error line.
//
// Policy split:
//   • unhandledRejection → log and CONTINUE. The most likely sources
//     are pg-node pool events and cron async slips; letting a single
//     background hiccup take down the API would create outages from
//     transient issues. If the same rejection keeps recurring, ops
//     sees it in the log stream and investigates.
//   • uncaughtException → log and EXIT. State may be corrupted after
//     a synchronous throw escapes to the top level; Node's official
//     guidance is exit-fast and let the process supervisor restart.
process.on('unhandledRejection', (reason) => {
    logger.error('Unhandled promise rejection', { reason: String(reason) });
});
process.on('uncaughtException', (err) => {
    logger.error('Uncaught exception — shutting down', { error: err.message, stack: err.stack });
    process.exit(1);
});

// ─── Start server + cron ──────────────────────────────────────────────────

const server = expressApp.listen(applicationConfig.port, () => {
    logger.info(`MTTR App started on port ${applicationConfig.port} (${applicationConfig.env})`);

    // Kick off the daily aggregation cron once the HTTP server is up
    // so we don't risk computing while the DB pool is still warming.
    startAggregationJob();
});

// ─── Graceful shutdown ────────────────────────────────────────────────────
//
// Choreo (Kubernetes) sends SIGTERM before terminating a pod and waits
// `terminationGracePeriodSeconds` (default 30s) before SIGKILL. Without
// a handler, in-flight requests get dropped mid-response and DB pool
// connections aren't closed cleanly. We:
//   1. Stop accepting new HTTP connections (`server.close`)
//   2. Wait for in-flight requests to finish
//   3. Drain the DB pool (`pool.end`)
//   4. Exit(0)
// A hard 25s timer short-circuits any hung shutdown so we exit under
// our own control before SIGKILL arrives — otherwise a stuck request
// or a wedged pool.end could leak the whole grace window.
// SIGINT is handled the same way so dev-loop Ctrl+C is also clean.
function gracefulShutdown(signal) {
    logger.info(`${signal} received — closing HTTP server`);
    setTimeout(() => {
        logger.error('Graceful shutdown timed out — forcing exit');
        process.exit(1);
    }, 25000).unref();

    server.close((serverCloseErr) => {
        if (serverCloseErr) {
            logger.error('HTTP server close error', { error: serverCloseErr.message });
        }
        pool.end()
            .then(() => {
                logger.info('DB pool drained — exiting');
                process.exit(0);
            })
            .catch((poolEndErr) => {
                logger.error('DB pool drain error', { error: poolEndErr.message });
                process.exit(1);
            });
    });
}
process.on('SIGTERM', () => gracefulShutdown('SIGTERM'));
process.on('SIGINT', () => gracefulShutdown('SIGINT'));

module.exports = expressApp;
