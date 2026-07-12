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
// Correlation-ID Middleware
// ----------------------------------------------------------------------------
// Ensures every request has a stable identifier that:
//   • is echoed back to the caller in an `X-Correlation-Id` response header
//   • is attached to `req.correlationId` for downstream handlers
//   • is stamped on every access-log and error-log line for that request
//
// If the caller already sent an `X-Correlation-Id` header we reuse theirs
// (this is how the ServiceNow scheduled job's `batch_id` traces end-to-end
// into Choreo logs); otherwise we mint a UUID so unattributed traffic
// still gets a searchable ID.
// ============================================================================

const { v4: uuidv4 } = require('uuid');
const logger = require('../utils/logger');

const HEADER_NAME = 'x-correlation-id';   // Express lowercases request headers
const RESPONSE_HEADER_NAME = 'X-Correlation-Id';

function correlationId(req, res, next) {
    const incoming = req.headers[HEADER_NAME];
    // Reject anything that isn't a plain, short, printable string — stops
    // a caller from injecting log-forging payloads or newline characters
    // into our own log stream via the header.
    const safe = typeof incoming === 'string' && /^[A-Za-z0-9._:-]{1,128}$/.test(incoming);
    if (incoming !== undefined && !safe) {
        // Log-injection attempts are cheap to raise and expensive to detect
        // post-hoc; surface each one at WARN so ops sees them in the log
        // aggregator. Only the source IP is logged — the malformed value
        // itself is intentionally NOT echoed, since that's what the attacker
        // was trying to inject into our log stream.
        logger.warn('Rejected malformed X-Correlation-Id header', { ip: req.ip });
    }
    req.correlationId = safe ? incoming : uuidv4();
    res.setHeader(RESPONSE_HEADER_NAME, req.correlationId);
    next();
}

module.exports = { correlationId };
