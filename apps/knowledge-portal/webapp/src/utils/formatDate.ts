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

const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;
const WEEK_MS = 7 * DAY_MS;

/** "Just now", "5 minutes ago", "3 hours ago", "yesterday", "4 days ago". */
export function formatRelativeTime(date: Date, now: Date = new Date()): string {
  const age = now.getTime() - date.getTime();
  if (age < MINUTE_MS) return "Just now";
  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
  if (age < HOUR_MS) return rtf.format(-Math.floor(age / MINUTE_MS), "minute");
  if (age < DAY_MS) return rtf.format(-Math.floor(age / HOUR_MS), "hour");
  return rtf.format(-Math.floor(age / DAY_MS), "day");
}

/** Locale date for an ISO timestamp, or undefined when it is missing or invalid. */
export function formatDate(iso: string | null | undefined): string | undefined {
  if (!iso) return undefined;
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return undefined;
  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export interface RelativeDate {
  iso: string;
  /** "3 days ago" for the last week, the locale date before that. */
  label: string;
  /** Always the locale date, for a tooltip or `title`. */
  absolute: string;
}

/**
 * Recent timestamps read better as "2 days ago" on a card; anything older is
 * the plain date, which is what a reader scanning for freshness wants.
 */
export function formatRelativeDate(iso: string | null | undefined, now: Date = new Date()): RelativeDate | undefined {
  const absolute = formatDate(iso);
  if (!iso || !absolute) return undefined;
  const date = new Date(iso);
  const age = now.getTime() - date.getTime();
  const label = age >= 0 && age < WEEK_MS ? formatRelativeTime(date, now) : absolute;
  return { iso, label, absolute };
}
