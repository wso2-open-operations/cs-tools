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
// Winston Logger
// ----------------------------------------------------------------------------
// Centralised structured logger used across the service.
//
//   • In PRODUCTION  → emits JSON (each line is one log record). This is the
//                      format Choreo's log collector ingests into the
//                      observability dashboard.
//   • In DEVELOPMENT → emits colourised, human-readable text for the console.
//
// All other modules import this logger so log format and metadata
// (defaultMeta.service) stay consistent everywhere.
// ============================================================================

const winston = require('winston');
const applicationConfig = require('../config');

const logger = winston.createLogger({
    level: applicationConfig.logLevel,
    format: winston.format.combine(
        // Always include an ISO timestamp on every record
        winston.format.timestamp(),
        // If an Error object is logged, include its stack trace
        winston.format.errors({ stack: true }),
        // Format switches between JSON (prod) and colorised text (dev)
        applicationConfig.env === 'production'
            ? winston.format.json()
            : winston.format.combine(winston.format.colorize(), winston.format.simple())
    ),
    // Tag every log line with the service name so multi-service log
    // aggregators can filter MTTR-app records easily.
    defaultMeta: { service: 'mttr-app' },
    transports: [new winston.transports.Console()],
});

module.exports = logger;
