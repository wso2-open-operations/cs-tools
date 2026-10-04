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

/** A Date as a local-calendar "YYYY-MM-DD" (never the UTC date). */
export function localDateStr(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

/** Today's date in the viewer's timezone. Call per render, never at module load. */
export function todayLocalStr(now: Date = new Date()): string {
  return localDateStr(now);
}

/** The local date `n` calendar days before today. */
export function daysAgoLocalStr(n: number, now: Date = new Date()): string {
  return localDateStr(new Date(now.getFullYear(), now.getMonth(), now.getDate() - n));
}

/**
 * Clamps a typed "YYYY-MM-DD" into `[min, max]`. An empty or partial value is
 * returned unchanged so the user can keep typing; ISO dates compare correctly
 * as strings once they are complete.
 */
export function clampDateInput(value: string, min: string, max: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return value;
  if (value > max) return max;
  if (value < min) return min;
  return value;
}
