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
// Authentication & Authorisation Middleware
// ----------------------------------------------------------------------------
// Two Express middlewares used by the route layer:
//
//   • authenticate  – validates an OAuth 2.0 / JWT Bearer token using the
//                     configured JWKS endpoint, then attaches the decoded
//                     payload to req.user.
//   • requireAdmin  – ensures req.user carries the `mttr-admin` role.
//
// When AUTH_ENABLED=false (the default for both local dev and Choreo
// production — see logic-and-architecture.md §10 for why) both middlewares
// turn into pass-throughs and inject a mock admin user so route handlers
// always have a consistent `req.user` shape.
// ============================================================================

const jwt = require('jsonwebtoken');
const jwksClient = require('jwks-rsa');
const applicationConfig = require('../config');
const logger = require('../utils/logger');

// JWKS (JSON Web Key Set) client used to fetch the IdP's public signing
// keys. Created once at module load so the in-memory cache survives across
// requests. If auth is disabled this stays undefined.
let jwksKeyClient;
if (applicationConfig.auth.enabled && applicationConfig.auth.jwksUri) {
    jwksKeyClient = jwksClient({
        jwksUri: applicationConfig.auth.jwksUri,
        cache: true,            // cache keys in memory to avoid refetching
        rateLimit: true,        // throttle outbound JWKS requests
        jwksRequestsPerMinute: 5,
    });
}

/**
 * Callback used by jsonwebtoken.verify() to obtain the public key that
 * matches the JWT header's `kid`. Wraps the async JWKS lookup in the
 * node-style (err, key) callback the jwt library expects.
 */
function resolveSigningKey(jwtHeader, doneCallback) {
    if (!jwksKeyClient) return doneCallback(new Error('JWKS client not configured'));
    jwksKeyClient.getSigningKey(jwtHeader.kid, (lookupError, signingKey) => {
        if (lookupError) return doneCallback(lookupError);
        doneCallback(null, signingKey.getPublicKey());
    });
}

/**
 * Express middleware: verify a Bearer JWT and attach the decoded payload
 * to `req.user`. Returns 401 if the header is missing/malformed or the
 * signature/claims fail validation.
 *
 * When AUTH_ENABLED=false, skips verification entirely and injects a mock
 * admin user — keeps local development friction-free.
 */
function authenticate(req, res, next) {
    // Bypass mode — used for local dev and inside Choreo where the API
    // gateway already validated the token.
    if (!applicationConfig.auth.enabled) {
        req.user = { sub: 'dev-user', roles: ['mttr-admin'] };
        return next();
    }

    const authorizationHeader = req.headers.authorization;
    if (!authorizationHeader || !authorizationHeader.startsWith('Bearer ')) {
        return res.status(401).json({ error: 'Missing or invalid Authorization header' });
    }

    // Strip the literal "Bearer " prefix (7 chars) to isolate the token.
    const bearerToken = authorizationHeader.substring(7);

    jwt.verify(
        bearerToken,
        resolveSigningKey,
        {
            audience: applicationConfig.auth.audience,
            issuer: applicationConfig.auth.issuer,
            algorithms: ['RS256'],   // Asgardeo / standard OIDC providers use RS256
        },
        (verificationError, decodedPayload) => {
            if (verificationError) {
                logger.warn('JWT verification failed', { error: verificationError.message });
                return res.status(401).json({ error: 'Invalid or expired token' });
            }
            req.user = decodedPayload;
            next();
        }
    );
}

/**
 * Express middleware: only allow requests whose JWT carries the
 * `mttr-admin` role/group. Mounted in front of every /admin/* route.
 *
 * Different IdPs put role claims under different keys; we accept either
 * `roles` or `groups` to stay portable.
 */
function requireAdmin(req, res, next) {
    // In bypass mode the mock user is always admin — no check needed.
    if (!applicationConfig.auth.enabled) return next();

    const userRoles = req.user?.roles || req.user?.groups || [];
    if (!userRoles.includes('mttr-admin')) {
        return res.status(403).json({ error: 'Insufficient permissions. Admin role required.' });
    }
    next();
}

module.exports = { authenticate, requireAdmin };
