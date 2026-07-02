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

// ISO-week helpers (weeks run Monday–Sunday). Dates are handled as plain
// YYYY-MM-DD strings in UTC to avoid timezone drift in week boundaries.

function toUtcDate(iso: string): Date {
  return new Date(`${iso.slice(0, 10)}T00:00:00Z`);
}

function isoOf(d: Date): string {
  return d.toISOString().slice(0, 10);
}

/** The Monday (YYYY-MM-DD) of the ISO week containing `iso`. */
export function weekStartOf(iso: string): string {
  const d = toUtcDate(iso);
  // getUTCDay: 0=Sun..6=Sat. Days since Monday = (day + 6) % 7.
  const sinceMonday = (d.getUTCDay() + 6) % 7;
  d.setUTCDate(d.getUTCDate() - sinceMonday);
  return isoOf(d);
}

/** The Sunday (YYYY-MM-DD) ending the week that starts on `weekStart`. */
export function weekEndOf(weekStart: string): string {
  const d = toUtcDate(weekStart);
  d.setUTCDate(d.getUTCDate() + 6);
  return isoOf(d);
}

/** Stable key for a week — the Monday date. */
export function weekKey(iso: string): string {
  return weekStartOf(iso);
}

/**
 * Today's date as YYYY-MM-DD in the user's **local** timezone. Use this for the
 * default log date and "current week" so an engineer logging at 1 AM local time
 * gets the local calendar day, not the UTC one (which can be the day before).
 */
export function localTodayIso(): string {
  const now = new Date();
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 10);
}

/** Human label, e.g. "Jun 23 – 29, 2026" (or spanning months/years). */
export function weekLabel(weekStart: string): string {
  const start = toUtcDate(weekStart);
  const end = toUtcDate(weekEndOf(weekStart));
  const mon = (d: Date): string =>
    d.toLocaleDateString("en-US", { month: "short", timeZone: "UTC" });
  const day = (d: Date): number => d.getUTCDate();
  const year = end.getUTCFullYear();
  if (mon(start) === mon(end)) {
    return `${mon(start)} ${day(start)} – ${day(end)}, ${year}`;
  }
  return `${mon(start)} ${day(start)} – ${mon(end)} ${day(end)}, ${year}`;
}
