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

import type { BeLineBucket } from "@api/backend/types";

/** The trailing window a trend reads when the code names none, and its cap (the backends'). */
const TREND_DEFAULT_DAYS = 30;
const TREND_MAX_DAYS = 90;
const TREND_BUCKETS: readonly BeLineBucket[] = ["day", "week", "month"];

/** A volume trend's window: the last `days` days, or a fixed one grouped by `bucket`. */
export type TrendWindow = { days: number } | { startDate: string; endDate: string; bucket?: BeLineBucket };

/**
 * The window of a code dashboard's trend call (`caseTrend`, `plgTrend`), or
 * throws (the code sees the message, prefixed by `fn`) when the args are not
 * `{days?}` or `{startDate, endDate, bucket?}`. Both volume trend endpoints
 * take the same windows, dates or date tokens; the cases' refuses a window
 * over its bucket's limit with no reason, so the shape is checked here and
 * the limits are the catalogue's to teach.
 */
export function buildTrendWindow(
  fn: string,
  { days, startDate, endDate, bucket }: { days?: unknown; startDate?: unknown; endDate?: unknown; bucket?: unknown },
): TrendWindow {
  if (startDate === undefined && endDate === undefined) {
    if (bucket !== undefined) throw new Error(`${fn}: bucket needs startDate and endDate`);
    if (days !== undefined && !(Number.isInteger(days) && (days as number) >= 1 && (days as number) <= TREND_MAX_DAYS)) {
      throw new Error(`${fn}: days must be an integer from 1 to ${TREND_MAX_DAYS}`);
    }
    return { days: (days as number | undefined) ?? TREND_DEFAULT_DAYS };
  }
  if (days !== undefined) throw new Error(`${fn}: use days or startDate and endDate, not both`);
  if (typeof startDate !== "string" || typeof endDate !== "string" || !startDate || !endDate) {
    throw new Error(`${fn}: startDate and endDate are both needed, as dates or date tokens`);
  }
  if (bucket !== undefined && !TREND_BUCKETS.includes(bucket as BeLineBucket)) {
    throw new Error(`${fn}: bucket must be one of ${TREND_BUCKETS.join(", ")}`);
  }
  return { startDate, endDate, ...(bucket !== undefined ? { bucket: bucket as BeLineBucket } : {}) };
}
