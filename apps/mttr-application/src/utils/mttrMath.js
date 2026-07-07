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
// MTTR Math — P95 Truncated-Mean Calculation
// ----------------------------------------------------------------------------
// Shared pure-function implementation used by both the aggregation service
// (which computes rolling-window MTTR into `mttr_cache`) and the retention
// service (which computes quarterly MTTR into `case_events_summary`).
//
// Previously duplicated in both services — extracted here so a fix to the
// algorithm can't silently diverge between the two callers. See
// docs/logic-and-architecture.md §2 for the full algorithm description.
// ============================================================================

const applicationConfig = require('../config');

// Below this sample size a P95 cut is meaningless (would exclude 0–1 rows),
// so we return the simple average and flag min_sample_met=false so the UI
// can render a "not enough data" warning.
const MIN_SAMPLE_FOR_P95 = applicationConfig.aggregation.minSampleSize;

/**
 * Compute the P95-truncated-mean MTTR for a bucket of business-duration
 * values (all in milliseconds).
 *
 * Returns `null` for empty/missing input so callers can distinguish
 * "no data" from "zero duration".
 *
 * Small-sample path (n < MIN_SAMPLE_FOR_P95): returns the simple average
 * with `min_sample_met: false` — no rows excluded.
 *
 * Normal path: sorts, discards the top ~5% as outliers, averages the rest.
 *
 * @param {Array<number>} durations
 * @returns {{
 *   total_cases: number,
 *   excluded_cases: number,
 *   p95_cutoff_ms: number|null,
 *   truncated_avg_ms: number,
 *   simple_avg_ms: number,
 *   min_sample_met: boolean
 * } | null}
 */
function computeTruncatedMTTR(durations) {
    if (!durations || durations.length === 0) return null;

    const sortedDurations = [...durations].sort((a, b) => a - b);
    const sampleSize = sortedDurations.length;
    const simpleAverage = sortedDurations.reduce((sum, v) => sum + v, 0) / sampleSize;

    if (sampleSize < MIN_SAMPLE_FOR_P95) {
        return {
            total_cases: sampleSize,
            excluded_cases: 0,
            p95_cutoff_ms: null,
            truncated_avg_ms: Math.round(simpleAverage),
            simple_avg_ms: Math.round(simpleAverage),
            min_sample_met: false,
        };
    }

    // P95 index using ceiling-rounded boundary, then -1 because arrays are
    // zero-indexed. e.g. n=30 → ceil(28.5)=29 → index 28 (29 items kept).
    const p95Index = Math.ceil(sampleSize * 0.95) - 1;
    const p95CutoffValue = sortedDurations[p95Index];
    const includedDurations = sortedDurations.slice(0, p95Index + 1);
    const truncatedAverage =
        includedDurations.reduce((sum, v) => sum + v, 0) / includedDurations.length;

    return {
        total_cases: sampleSize,
        excluded_cases: sampleSize - includedDurations.length,
        p95_cutoff_ms: p95CutoffValue,
        truncated_avg_ms: Math.round(truncatedAverage),
        simple_avg_ms: Math.round(simpleAverage),
        min_sample_met: true,
    };
}

module.exports = { computeTruncatedMTTR };
