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
// Response Formatting Helpers
// ----------------------------------------------------------------------------
// Small pure functions shared across route modules for shaping DB values
// into the API contract. Kept in one file so a formatting change (e.g.
// number of decimal places) can't drift between endpoints.
// ============================================================================

/**
 * Convert a millisecond duration to hours rounded to one decimal place.
 * Returns null for null/undefined input so the JSON serialiser preserves
 * the absence of a value instead of emitting `0`.
 *
 * @param {number|null|undefined} milliseconds
 * @returns {number|null}
 */
function millisecondsToHours(milliseconds) {
    if (milliseconds === null || milliseconds === undefined) return null;
    return Math.round((milliseconds / 3600000) * 10) / 10;
}

module.exports = { millisecondsToHours };
