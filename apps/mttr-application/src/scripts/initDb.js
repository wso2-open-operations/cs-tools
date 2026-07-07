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
// Database Initialisation Script
// ----------------------------------------------------------------------------
// One-off helper that reads sql/init.sql (which contains CREATE TABLE +
// CREATE INDEX statements for every table the app needs) and executes it
// against the configured PostgreSQL instance.
//
// Usage:
//   npm run db:init
//
// The SQL is written to be idempotent (CREATE TABLE IF NOT EXISTS, etc.)
// so it is safe to run multiple times — useful when iterating on the
// schema during development.
// ============================================================================

const fs = require('fs');
const path = require('path');
const { pool: connectionPool } = require('../config/database');
const logger = require('../utils/logger');

async function initDb() {
    const schemaSqlPath = path.join(__dirname, '..', '..', 'sql', 'init.sql');
    const schemaSqlText = fs.readFileSync(schemaSqlPath, 'utf8');

    logger.info('Initializing database schema...');

    try {
        await connectionPool.query(schemaSqlText);
        logger.info('Database schema initialized successfully.');
    } catch (schemaInitError) {
        logger.error('Database initialization failed', { error: schemaInitError.message });
        process.exit(1);
    } finally {
        // Important: end the pool so the Node process can exit cleanly
        // (otherwise the open connections keep the event loop alive).
        await connectionPool.end();
    }
}

initDb();
